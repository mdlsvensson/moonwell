package library

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
)

func syncGitHub(
	ctx context.Context, e *env.Env, at folders, library manifest.Library, locked *lockEntry, manifestFile string,
) (lockEntry, error) {
	github, tag, dir := *library.GitHub, *library.Tag, library.Dir
	if err := refuseNames(at.key, github, tag, manifestFile); err != nil {
		return lockEntry{}, err
	}
	sameTag := locked != nil && locked.GitHub == github && locked.Tag == tag && locked.Dir == dir
	if sameTag && holds(at, *locked) {
		return *locked, nil
	}
	commit, files, err := downloadTag(ctx, e.Fetch, at.key, github, tag, manifestFile)
	if err != nil {
		return lockEntry{}, err
	}
	kept, err := keep(at.key, github, tag, dir, files, manifestFile)
	if err != nil {
		return lockEntry{}, err
	}
	entry := entryFor(github, tag, dir, commit, kept)
	if sameTag && moved(*locked, entry) {
		return lockEntry{}, errMoved(at.key, github, tag, locked.Commit, entry.Commit)
	}
	if err := writeDownloaded(e.Root, at, kept, entry); err != nil {
		return lockEntry{}, err
	}
	e.Log.Info("Fetched library " + at.key + ": " + github + " " + tag + " (" + short(entry.Commit, 7) + ").")
	return entry, nil
}

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

func holds(at folders, entry lockEntry) bool {
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

func hasStampLayout(stamp []byte) bool {
	members, _ := objectOf(stamp)
	var layout float64
	return json.Unmarshal(members["layout"], &layout) == nil && layout == stampLayout
}

func sameLocked(a, b lockEntry) bool {
	if (a.Assets == nil) != (b.Assets == nil) || (a.Assets != nil && *a.Assets != *b.Assets) {
		return false
	}
	return a.GitHub == b.GitHub && a.Tag == b.Tag && a.Dir == b.Dir && a.Commit == b.Commit && a.Files == b.Files
}

func keep(key, github, tag, dir string, files []file, manifestFile string) (shipped, error) {
	libraryFile := "https://github.com/" + github + "/blob/" + tag + "/" + File
	content, present := contentIn(files, File)
	described, err := parseFile(key, content, present, libraryFile)
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

func contentIn(files []file, name string) (content []byte, present bool) {
	for _, f := range files {
		if f.name == name {
			return f.data, true
		}
	}
	return nil, false
}

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

func hasDotName(path string) bool {
	return strings.HasPrefix(path, ".") || strings.Contains(path, "/.")
}

func entryFor(github, tag, dir, commit string, kept shipped) lockEntry {
	entry := lockEntry{GitHub: github, Tag: tag, Dir: dir, Commit: commit, Files: filesHash(kept.modules)}
	if kept.shipsAssets {
		hash := filesHash(kept.assets)
		entry.Assets = &hash
	}
	return entry
}

func moved(locked, entry lockEntry) bool {
	switch {
	case entry.Commit != locked.Commit:
		return true
	case (locked.Assets == nil) != (entry.Assets == nil):
		return false
	}
	return entry.Files != locked.Files || (entry.Assets != nil && *entry.Assets != *locked.Assets)
}

func short(commit string, length int) string { return commit[:min(length, len(commit))] }

func writeDownloaded(root string, at folders, kept shipped, entry lockEntry) error {
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

func dropStamp(at folders) error {
	stamp := filepath.Join(at.modules, stampFile)
	if info, err := fsx.Lstat(stamp); err != nil || info == nil || info.IsDir() {
		return nil
	}
	if err := fsx.RemoveFile(stamp); err != nil {
		return errUnremovable(modulesOf(at.key)+"/"+stampFile, err)
	}
	return nil
}

func replace(root, dir, key string, files []file, stamp string) error {
	label := dir + "/" + key
	temp, err := fsx.SafeJoinNoSymlinks(root, dir+"/."+key+".tmp")
	if err != nil {
		return err
	}
	target, err := fsx.SafeJoinNoSymlinks(root, label)
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
		return errUnremovable(label, err)
	}
	if err := os.Rename(temp, target); err != nil {
		return errUnwritable(label, err)
	}
	return nil
}

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

func errMoved(key, github, tag, from, to string) error {
	what := "tag " + tag + " of " + github + " moved from " + short(from, 12) + " to " + short(to, 12) +
		" since " + lockFile + " recorded it."
	if from == to {
		what = "the files of tag " + tag + " of " + github + " are not those " + lockFile + " recorded for commit " +
			short(to, 12) + "."
	}
	return &diag.Error{
		Msg:  "Library " + key + ": " + what,
		File: lockFile,
		Hint: "If the move was intended, delete the library's entry from " + lockFile + " and run the command again.",
	}
}
