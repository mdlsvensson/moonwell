package library

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
)

// syncGitHub downloads a tag of a GitHub library into its two folders, unless they hold the entry the lock has
// for it, and returns the library's lock entry. locked is that entry, nil when the lock has none.
func syncGitHub(
	ctx context.Context, e *env.Env, at folders, library manifest.Library, locked *LockEntry, manifestFile string,
) (LockEntry, error) {
	github, tag, dir := *library.GitHub, *library.Tag, library.Dir
	if err := refuseNames(at.key, github, tag, manifestFile); err != nil {
		return LockEntry{}, err
	}
	sameTag := locked != nil && locked.GitHub == github && locked.Tag == tag && locked.Dir == dir
	if sameTag && holds(at, *locked) {
		return *locked, nil
	}
	commit, files, err := downloadTag(ctx, e.Fetch, at.key, github, tag, manifestFile)
	if err != nil {
		return LockEntry{}, err
	}
	kept, err := keep(at.key, github, tag, dir, files, manifestFile)
	if err != nil {
		return LockEntry{}, err
	}
	entry := entryFor(github, tag, dir, commit, kept)
	if sameTag && moved(*locked, entry) {
		return LockEntry{}, errMoved(at.key, github, tag, locked.Commit, entry.Commit)
	}
	if err := writeDownloaded(e.Root, at, kept, entry); err != nil {
		return LockEntry{}, err
	}
	e.Log.Info("Fetched library " + at.key + ": " + github + " " + tag + " (" + short(entry.Commit, 7) + ").")
	return entry, nil
}

// refuseNames refuses a tag and a repository whose name would lead a download address somewhere else: a part of
// the tag between two "/", or the repository's name, that is "." or "..".
func refuseNames(key, github, tag, manifestFile string) error {
	for segment := range strings.SplitSeq(tag, "/") {
		if segment == "." || segment == ".." {
			return errNotATag(key, github, tag, manifestFile)
		}
	}
	if parts := strings.Split(github, "/"); len(parts) > 1 && (parts[1] == "." || parts[1] == "..") {
		return errNotARepository(key, github, manifestFile)
	}
	return nil
}

// holds reports whether the library's folders hold entry: the stamp in its module folder is the entry with the
// layout of the folders, and the folder of its files for the map is there when it ships any.
func holds(at folders, entry LockEntry) bool {
	stamp, err := os.ReadFile(filepath.Join(at.modules, stampFile))
	if err != nil {
		return false
	}
	stamped, isEntry := entryOf(stamp)
	if !isEntry || !hasStampLayout(stamp) || !sameLocked(stamped, entry) {
		return false
	}
	return entry.Assets == nil || fsx.IsDir(at.assets)
}

// hasStampLayout reports whether a stamp, which is a JSON object, names the layout of the folders as a number.
func hasStampLayout(stamp []byte) bool {
	members, _ := objectOf(stamp)
	var layout float64
	return json.Unmarshal(members["layout"], &layout) == nil && layout == stampLayout
}

// sameLocked reports whether two lock entries are the same in every member.
func sameLocked(a, b LockEntry) bool {
	if (a.Assets == nil) != (b.Assets == nil) || (a.Assets != nil && *a.Assets != *b.Assets) {
		return false
	}
	return a.GitHub == b.GitHub && a.Tag == b.Tag && a.Dir == b.Dir && a.Commit == b.Commit && a.Files == b.Files
}

// ---- what is kept of a tag ----

// keep is what is kept of the files of a tag: the modules, which are the files below the module folder, and the
// files for the map when the library's own file names a folder of them. The module folder is the manifest's dir,
// else the one the library's file names, else the library's root. A folder that is named and has no files is
// refused.
func keep(key, github, tag, dir string, files []file, manifestFile string) (shipped, error) {
	libraryFile := "https://github.com/" + github + "/blob/" + tag + "/" + File
	content, present := contentIn(files, File)
	described, err := ParseFile(key, content, present, libraryFile)
	if err != nil {
		return shipped{}, err
	}
	moduleDir := dir
	if moduleDir == "" && described.Dir != nil {
		moduleDir = *described.Dir
	}
	kept := shipped{modules: keepBelow(files, moduleDir, described.Assets), shipsAssets: described.Assets != nil}
	switch {
	case len(kept.modules) == 0 && dir != "":
		return shipped{}, errNoFolderOfTheManifest(key, moduleDir, tag, manifestFile)
	case len(kept.modules) == 0:
		return shipped{}, errNoFolderOfTheLibrary(key, moduleDir, tag, libraryFile)
	}
	if kept.shipsAssets {
		if kept.assets = keepBelow(files, *described.Assets, nil); len(kept.assets) == 0 {
			return shipped{}, errNoAssetsOfTheLibrary(key, *described.Assets, tag, libraryFile)
		}
	}
	return kept, kept.refuseUnusable(key, manifestFile)
}

// contentIn is the bytes of the file of a name among files; false when there is none.
func contentIn(files []file, name string) (content []byte, present bool) {
	for _, f := range files {
		if f.name == name {
			return f.data, true
		}
	}
	return nil, false
}

// keepBelow is the files below the folder dir of a library, all of them when dir names none, each under its
// path from dir. A file in a folder or with a name that starts with "." is not kept (.github/, .gitignore), nor
// is a file below the folder except, when one is given.
func keepBelow(files []file, dir string, except *string) []file {
	prefix := ""
	for _, segment := range strings.FieldsFunc(dir, func(r rune) bool { return r == '/' || r == '\\' }) {
		if segment != "." {
			prefix += segment + "/"
		}
	}
	var kept []file
	for _, f := range files {
		below, isBelow := strings.CutPrefix(f.name, prefix)
		if !isBelow || (except != nil && strings.HasPrefix(f.name, *except+"/")) || hasDotName(below) {
			continue
		}
		kept = append(kept, file{below, f.data})
	}
	return kept
}

// hasDotName reports whether a name on the path starts with ".".
func hasDotName(path string) bool {
	return strings.HasPrefix(path, ".") || strings.Contains(path, "/.")
}

// entryFor is the lock entry of a tag: what the manifest says of the library, the tag's commit, and the hashes of
// what is kept of it.
func entryFor(github, tag, dir, commit string, kept shipped) LockEntry {
	entry := LockEntry{GitHub: github, Tag: tag, Dir: dir, Commit: commit, Files: filesHash(kept.modules)}
	if kept.shipsAssets {
		hash := filesHash(kept.assets)
		entry.Assets = &hash
	}
	return entry
}

// moved reports whether a tag is something else than the lock recorded of it: another commit, or other files
// under the same commit.
//
// The files are compared only when both entries, or neither, have a hash of files for the map. An entry
// without one, for a library that ships such files, comes from a Moonwell that knows no files for the map: its
// hash of the modules may count files that are shipped for the map, so only the commit says whether the tag
// moved.
func moved(locked, entry LockEntry) bool {
	switch {
	case entry.Commit != locked.Commit:
		return true
	case (locked.Assets == nil) != (entry.Assets == nil):
		return false
	}
	return entry.Files != locked.Files || (entry.Assets != nil && *entry.Assets != *locked.Assets)
}

// short is the first bytes of a commit, at most length of them.
func short(commit string, length int) string { return commit[:min(length, len(commit))] }

// ---- writing what is kept ----

// writeDownloaded makes the library's two folders hold what is kept of a tag. From its first step to its last
// the folders hold no entry, so a sync that is interrupted anywhere between them is downloaded again: the stamp
// the module folder has is removed first, then the files for the map are replaced, and the module folder, which
// comes with the stamp of the tag, is replaced last.
func writeDownloaded(root string, at folders, kept shipped, entry LockEntry) error {
	err := dropStamp(at)
	switch {
	case err != nil:
		return err
	case kept.shipsAssets:
		err = replace(root, AssetsDir, at.key, kept.assets, "")
	default:
		err = removeAssets(at)
	}
	if err != nil {
		return err
	}
	return replace(root, ModulesDir, at.key, kept.modules, stampOf(entry))
}

// dropStamp removes the stamp of the library's module folder. A stamp that is no file is left: no entry is read
// from it.
func dropStamp(at folders) error {
	stamp := filepath.Join(at.modules, stampFile)
	if info, err := fsx.Lstat(stamp); err != nil || info == nil || info.IsDir() {
		return nil
	}
	if err := fsx.RemoveFile(stamp); err != nil {
		return errUnwritable(modulesOf(at.key), err)
	}
	return nil
}

// replace writes the files, and the stamp when one is given, into the folder .<key>.tmp of dir, which then takes
// the place of the folder <key>. dir is one of the two folders, from the project folder at root.
func replace(root, dir, key string, files []file, stamp string) error {
	label := dir + "/" + key
	temp, err := inProject(root, dir+"/."+key+".tmp", label)
	if err != nil {
		return err
	}
	target, err := inProject(root, label, label)
	if err != nil {
		return err
	}
	if stamp != "" {
		files = append(slices.Clone(files), file{stampFile, []byte(stamp)})
	}
	if err := writeAnew(temp, files); err != nil {
		return errUnwritable(label, err)
	}
	if err := fsx.RemoveAll(target); err != nil {
		return errUnwritable(label, err)
	}
	if err := os.Rename(temp, target); err != nil {
		return errUnwritable(label, err)
	}
	return nil
}

// writeAnew makes folder, which holds nothing afterwards but the files, each below it under its name. Its
// failures are the system's.
func writeAnew(folder string, files []file) error {
	if err := fsx.RemoveAll(folder); err != nil {
		return err
	}
	for _, f := range files {
		path, err := fsx.SafeJoin(folder, f.name)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
			return err
		}
		if err := os.WriteFile(path, f.data, 0o666); err != nil {
			return err
		}
	}
	return nil
}

// ---- errors ----

func errNotATag(key, github, tag, manifestFile string) error {
	return &diag.Error{
		Msg:  "Library " + key + ": " + tag + " is not a tag name.",
		File: manifestFile,
		Hint: "Use the tag's name as it appears at https://github.com/" + github + "/tags.",
	}
}

func errNotARepository(key, github, manifestFile string) error {
	return &diag.Error{
		Msg:  "Library " + key + ": " + github + " is not a GitHub repository.",
		File: manifestFile,
		Hint: `Write it as "owner/repo".`,
	}
}

func errNoFolderOfTheManifest(key, dir, tag, manifestFile string) error {
	return &diag.Error{
		Msg:  "Library " + key + " has no folder " + dir + " at " + tag + ".",
		File: manifestFile,
		Hint: "Fix the library's dir.",
	}
}

func errNoFolderOfTheLibrary(key, dir, tag, libraryFile string) error {
	return &diag.Error{
		Msg:  "Library " + key + " has no folder " + dir + " at " + tag + ".",
		File: libraryFile,
		Hint: "Its " + File + " names a dir that has no files.",
	}
}

func errNoAssetsOfTheLibrary(key, assets, tag, libraryFile string) error {
	return &diag.Error{
		Msg:  "Library " + key + " has no folder " + assets + " at " + tag + ".",
		File: libraryFile,
		Hint: "Its " + File + " names an assets folder that has no files; report it to the library's author.",
	}
}

// errMoved refuses a tag that is something else than the lock recorded of it: from is the commit the lock has,
// and to the commit the tag is at. Where the two are one commit, it is the files kept of the tag that are others,
// and the refusal names the commit once and says so.
func errMoved(key, github, tag, from, to string) error {
	what := "tag " + tag + " of " + github + " moved from " + short(from, 12) + " to " + short(to, 12) +
		" since " + LockFile + " recorded it."
	if from == to {
		what = "the files of tag " + tag + " of " + github + " are not those " + LockFile + " recorded for commit " +
			short(to, 12) + "."
	}
	return &diag.Error{
		Msg:  "Library " + key + ": " + what,
		File: LockFile,
		Hint: "If the move was intended, delete the library's entry from " + LockFile + " and run the command again.",
	}
}
