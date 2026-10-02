package library

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/layout"
	"github.com/mdlsvensson/moonwell/internal/logging"
	"github.com/mdlsvensson/moonwell/internal/ordered"
	"github.com/mdlsvensson/moonwell/internal/project"
	"github.com/mdlsvensson/moonwell/internal/text"
)

const (
	// stamp is the file that says what a library folder holds: its lock entry, or the local path it was copied
	// from.
	stamp = ".moonwell-library.json"
	// stampLayout is the stamp's layout: 2 since a library can ship assets. A folder with another layout is fetched
	// again.
	stampLayout = 2
)

// Deps is what Sync needs from the outside world.
type Deps struct {
	Fetch Fetch
	Log   *logging.Logger
}

// folders are the two folders under .moonwell/ that hold every library's modules and assets.
type folders struct{ modules, assets string }

// Sync brings .moonwell/libraries/<key>/ and .moonwell/library-assets/<key>/ up to date for every library of the
// manifest, and moonwell.lock with the GitHub ones. A GitHub library whose folders already hold its lock entry is
// not downloaded again; a local library keeps the lock entry it had.
func Sync(ctx context.Context, root string, libraries *ordered.Map[project.Library], manifest string, deps Deps) error {
	keys := slices.Clone(libraries.Keys())
	text.Sort(keys)
	if err := refuseCaseClashes(keys, manifest); err != nil {
		return err
	}
	in := folders{
		modules: filepath.Join(root, filepath.FromSlash(layout.LibrariesDir)),
		assets:  filepath.Join(root, filepath.FromSlash(layout.LibraryAssetsDir)),
	}
	// Before syncing: on a case-insensitive file system, a stale folder `lib` would otherwise take key `Lib`'s files
	// and then be removed as stale.
	if err := removeStale(in.modules, layout.LibrariesDir, keys); err != nil {
		return err
	}
	if err := removeStale(in.assets, layout.LibraryAssetsDir, keys); err != nil {
		return err
	}
	lock, err := ReadLock(root)
	if err != nil {
		return err
	}
	next := map[string]LockEntry{}
	for _, key := range keys {
		library, _ := libraries.Get(key)
		locked, isLocked := lock[key]
		if library.Path != nil {
			if err := syncLocal(root, in, key, *library.Path, library.Dir, manifest); err != nil {
				return err
			}
			// A path usually comes from moonwell.local.pkl, which is not committed: keep the committed lock entry, so
			// switching back still checks the tag.
			if isLocked {
				next[key] = locked
			}
			continue
		}
		var previous *LockEntry
		if isLocked {
			previous = &locked
		}
		entry, err := syncGitHub(ctx, in, key, library, previous, manifest, deps)
		if err != nil {
			return err
		}
		next[key] = entry
	}
	if err := WriteLock(root, next); err != nil {
		return &diag.Error{
			Msg:   "Writing " + LockFile + " failed: " + reasonOf(err),
			File:  LockFile,
			Cause: err,
			Hint:  "Close programs that have moonwell.lock open, and check it is not read-only.",
		}
	}
	return nil
}

// refuseCaseClashes fails for keys such as `Lib` and `lib`, which would share one folder on Windows.
func refuseCaseClashes(keys []string, manifest string) error {
	seen := map[string]string{}
	for _, key := range keys {
		if other, ok := seen[text.Lower(key)]; ok {
			return &diag.Error{
				Msg:  "Libraries " + other + " and " + key + " differ only by case.",
				File: manifest,
				Hint: "Rename one of them: each library gets a folder in .moonwell/libraries/.",
			}
		}
		seen[text.Lower(key)] = key
	}
	return nil
}

// writing reports a failure of body as one writing <label>/<name> (the folder label itself for "").
func writing(label, name string, body func() error) error {
	err := body()
	if err == nil {
		return nil
	}
	path := label
	if name != "" {
		path += "/" + name
	}
	return &diag.Error{
		Msg:   "Writing " + path + " failed: " + reasonOf(err),
		File:  path,
		Cause: err,
		Hint:  "Close programs that have files in .moonwell/ open, then retry.",
	}
}

// removeStale removes folders of libraries no longer in the manifest, and leftover `.<key>.tmp` folders.
func removeStale(folder, label string, keys []string) error {
	var names []string
	err := writing(label, "", func() error {
		entries, err := os.ReadDir(folder)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, name := range names {
		if slices.Contains(keys, name) {
			continue
		}
		if err := writing(label, name, func() error { return fsx.RemoveAll(filepath.Join(folder, name)) }); err != nil {
			return err
		}
	}
	return nil
}

var (
	moduleFile = regexp.MustCompile(`\.(yue|lua)$`)
	anyFile    = regexp.MustCompile(`^`)
)

// syncLocal copies a local library from <path>: its modules (.yue and .lua files) under the module folder and, when
// its moonwell-library.json names one, every file under its assets folder; both outside dot-names such as .git/.
// Only changed files are written, and every other file is removed.
func syncLocal(root string, in folders, key, path, dir, manifest string) error {
	base, err := filepath.Abs(fsx.Resolve(root, path))
	if err != nil {
		return err
	}
	file := filepath.Join(base, File)
	// A file that cannot be read counts as none: a library without the file is the usual case.
	content, readErr := os.ReadFile(file)
	described, err := ParseFile(key, content, readErr == nil, file)
	if err != nil {
		return err
	}
	moduleDir := dir
	if moduleDir == "" && described.Dir != nil {
		moduleDir = *described.Dir
	}
	source := fsx.Resolve(base, moduleDir)
	if !fsx.IsDir(source) {
		return &diag.Error{
			Msg:  "Library " + key + ": " + source + " is not a folder.",
			File: manifest,
			Hint: "Set the library's path (and dir) to a folder that holds its modules.",
		}
	}
	if fsx.IsWithin(in.modules, source) {
		return &diag.Error{
			Msg:  "Library " + key + ": " + source + " contains this project's " + layout.LibrariesDir + ".",
			File: manifest,
			Hint: "Point the library's path (and dir) at the folder that holds its modules, not at the project.",
		}
	}
	assetsSource := ""
	if described.Assets != nil {
		assetsSource = fsx.Resolve(base, *described.Assets)
		if !fsx.IsDir(assetsSource) {
			return &diag.Error{
				Msg:  "Library " + key + ": " + assetsSource + " is not a folder.",
				File: file,
				Hint: "Create the folder, or fix assets in the library's " + File + ".",
			}
		}
	}
	files, assets := NewFiles(), NewFiles()
	read := func(into *Files, folder string, names *regexp.Regexp, skip string) error {
		listed, err := listLocalFiles(folder, names, skip, "")
		if err != nil {
			return err
		}
		for _, name := range listed {
			data, err := os.ReadFile(filepath.Join(folder, filepath.FromSlash(name)))
			if err != nil {
				return err
			}
			into.Set(name, data)
		}
		return nil
	}
	err = read(files, source, moduleFile, assetsSource)
	if err == nil && assetsSource != "" {
		err = read(assets, assetsSource, anyFile, "")
	}
	if err != nil {
		return &diag.Error{
			Msg:   "Reading library " + key + " from " + source + " failed: " + reasonOf(err),
			File:  manifest,
			Cause: err,
			Hint:  "Check the library's path and that its files can be read.",
		}
	}
	err = writing(layout.LibraryAssetsDir, key, func() error {
		if assetsSource == "" {
			return fsx.RemoveAll(filepath.Join(in.assets, key))
		}
		return mirror(filepath.Join(in.assets, key), assets, "")
	})
	if err != nil {
		return err
	}
	target := filepath.Join(in.modules, key)
	return writing(layout.LibrariesDir, key, func() error {
		if err := mirror(target, files, stamp); err != nil {
			return err
		}
		from := &ordered.Map[any]{}
		from.Set("path", source)
		_, err := fsx.WriteIfChanged(filepath.Join(target, stamp), ordered.Stringify(from, 2)+"\n")
		return err
	})
}

// mirror makes target hold exactly files (and keep, when named), writing only the files that changed.
func mirror(target string, files *Files, keep string) error {
	for _, name := range files.Names() {
		data, _ := files.Get(name)
		if err := writeBytesIfChanged(filepath.Join(target, filepath.FromSlash(name)), data); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(target, 0o777); err != nil {
		return err
	}
	existing, err := fsx.ListFiles(target)
	if err != nil {
		return err
	}
	for _, name := range existing {
		if name != keep && !files.Has(name) {
			if err := os.Remove(filepath.Join(target, filepath.FromSlash(name))); err != nil {
				return err
			}
		}
	}
	return nil
}

// listLocalFiles lists the files under dir whose name matches, as POSIX paths. It never takes a file or looks
// inside a folder whose name starts with ".", and never looks inside skip (the assets folder, when it lies inside
// the module folder).
func listLocalFiles(dir string, name *regexp.Regexp, skip, prefix string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path := prefix + entry.Name()
		full := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			if skip != "" && fsx.IsWithin(full, skip) {
				continue
			}
			inner, err := listLocalFiles(full, name, skip, path+"/")
			if err != nil {
				return nil, err
			}
			files = append(files, inner...)
		} else if name.MatchString(entry.Name()) {
			files = append(files, path)
		}
	}
	return files, nil
}

// syncGitHub downloads a GitHub library unless its folders hold its lock entry, and returns its lock entry.
func syncGitHub(ctx context.Context, in folders, key string, library project.Library, locked *LockEntry, manifest string, deps Deps) (LockEntry, error) {
	github, tag, dir := *library.GitHub, *library.Tag, library.Dir
	none := LockEntry{}
	for _, segment := range strings.Split(tag, "/") {
		if segment == "." || segment == ".." {
			return none, &diag.Error{
				Msg:  "Library " + key + ": " + tag + " is not a tag name.",
				File: manifest,
				Hint: "Use the tag's name as it appears at https://github.com/" + github + "/tags.",
			}
		}
	}
	if parts := strings.Split(github, "/"); len(parts) > 1 && (parts[1] == "." || parts[1] == "..") {
		return none, &diag.Error{
			Msg: "Library " + key + ": " + github + " is not a GitHub repository.", File: manifest, Hint: `Write it as "owner/repo".`,
		}
	}
	sameTag := locked != nil && locked.GitHub == github && locked.Tag == tag && locked.Dir == dir
	if sameTag && holds(in, key, *locked) {
		return *locked, nil
	}
	commit, files, err := DownloadTag(ctx, key, github, tag, manifest, deps.Fetch)
	if err != nil {
		return none, err
	}
	where := "https://github.com/" + github + "/blob/" + tag + "/" + File
	content, hasFile := files.Get(File)
	described, err := ParseFile(key, content, hasFile, where)
	if err != nil {
		return none, err
	}
	moduleDir, except := dir, ""
	if moduleDir == "" && described.Dir != nil {
		moduleDir = *described.Dir
	}
	if described.Assets != nil {
		except = *described.Assets
	}
	kept := keepDir(files, moduleDir, except, described.Assets != nil)
	if kept.Len() == 0 {
		failure := &diag.Error{
			Msg:  "Library " + key + " has no folder " + moduleDir + " at " + tag + ".",
			File: where,
			Hint: "Its " + File + " names a dir that has no files.",
		}
		if dir != "" {
			failure.File, failure.Hint = manifest, "Fix the library's dir."
		}
		return none, failure
	}
	var assets *Files
	if described.Assets != nil {
		assets = keepDir(files, *described.Assets, "", false)
		if assets.Len() == 0 {
			return none, &diag.Error{
				Msg:  "Library " + key + " has no folder " + *described.Assets + " at " + tag + ".",
				File: where,
				Hint: "Its " + File + " names an assets folder that has no files; report it to the library's author.",
			}
		}
	}
	entry := LockEntry{GitHub: github, Tag: tag, Dir: dir, Commit: commit, Files: FilesHash(kept)}
	if assets != nil {
		hash := FilesHash(assets)
		entry.Assets = &hash
	}
	// A lock from before libraries shipped assets has no assets hash, and its module hash may count files that are
	// assets now: then only the commit is compared.
	if sameTag {
		comparable := (locked.Assets == nil) == (entry.Assets == nil)
		sameAssets := locked.Assets == nil || entry.Assets == nil || *locked.Assets == *entry.Assets
		if entry.Commit != locked.Commit || (comparable && (entry.Files != locked.Files || !sameAssets)) {
			return none, &diag.Error{
				Msg: "Library " + key + ": tag " + tag + " of " + github + " moved from " + short(locked.Commit, 12) + " to " +
					short(entry.Commit, 12) + " since moonwell.lock recorded it.",
				File: LockFile,
				Hint: "If the move was intended, delete the library's entry from moonwell.lock and run the command again.",
			}
		}
	}
	// The assets first and the stamp last: an interrupted sync leaves folders that are fetched again.
	err = writing(layout.LibraryAssetsDir, key, func() error {
		if assets == nil {
			return fsx.RemoveAll(filepath.Join(in.assets, key))
		}
		return replaceFolder(in.assets, key, assets, nil)
	})
	if err != nil {
		return none, err
	}
	err = writing(layout.LibrariesDir, key, func() error { return replaceFolder(in.modules, key, kept, &entry) })
	if err != nil {
		return none, err
	}
	deps.Log.Info("Fetched library " + key + ": " + github + " " + tag + " (" + short(entry.Commit, 7) + ").")
	return entry, nil
}

func short(commit string, length int) string { return commit[:min(length, len(commit))] }

// holds reports whether the library's folders hold entry: the stamp says so, and its assets folder is there when
// it has assets.
func holds(in folders, key string, entry LockEntry) bool {
	data, err := os.ReadFile(filepath.Join(in.modules, key, stamp))
	if err != nil {
		return false
	}
	tree, err := ordered.Decode(data)
	if err != nil {
		return false
	}
	fields, ok := tree.(*ordered.Object)
	if !ok {
		return false
	}
	if layoutField, _ := fields.Get("layout"); layoutField != float64(stampLayout) {
		return false
	}
	stamped, ok := entryOf(fields)
	if !ok || stamped.GitHub != entry.GitHub || stamped.Tag != entry.Tag || stamped.Dir != entry.Dir ||
		stamped.Commit != entry.Commit || stamped.Files != entry.Files || (stamped.Assets == nil) != (entry.Assets == nil) ||
		(entry.Assets != nil && *stamped.Assets != *entry.Assets) {
		return false
	}
	return entry.Assets == nil || fsx.IsDir(filepath.Join(in.assets, key))
}

var separators = regexp.MustCompile(`[\\/]`)

// keepDir returns the files under dir (all of them when it is empty), relative to it, except those in a folder or
// with a name that starts with "." (such as .github/, and the stamp), and, when hasExcept is set, except those
// under the folder except.
func keepDir(files *Files, dir, except string, hasExcept bool) *Files {
	var segments []string
	for _, segment := range separators.Split(dir, -1) {
		if segment != "" && segment != "." {
			segments = append(segments, segment)
		}
	}
	prefix := strings.Join(segments, "/")
	kept := NewFiles()
	for _, path := range files.Names() {
		if hasExcept && strings.HasPrefix(path, except+"/") {
			continue
		}
		relative := path
		if prefix != "" {
			var under bool
			if relative, under = strings.CutPrefix(path, prefix+"/"); !under {
				continue
			}
		}
		if slices.ContainsFunc(strings.Split(relative, "/"), func(segment string) bool { return strings.HasPrefix(segment, ".") }) {
			continue
		}
		data, _ := files.Get(path)
		kept.Set(relative, data)
	}
	return kept
}

// replaceFolder writes the files, and the stamp of entry when given, into .<key>.tmp/, which then replaces
// <folder>/<key>.
func replaceFolder(folder, key string, files *Files, entry *LockEntry) error {
	temp := filepath.Join(folder, "."+key+".tmp")
	if err := fsx.RemoveAll(temp); err != nil {
		return err
	}
	for _, path := range files.Names() {
		file := filepath.Join(temp, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(file), 0o777); err != nil {
			return err
		}
		data, _ := files.Get(path)
		if err := os.WriteFile(file, data, 0o666); err != nil {
			return err
		}
	}
	if entry != nil {
		fields := stampFields(*entry)
		fields.Set("layout", stampLayout)
		if _, err := fsx.WriteIfChanged(filepath.Join(temp, stamp), ordered.Stringify(fields, 2)+"\n"); err != nil {
			return err
		}
	}
	target := filepath.Join(folder, key)
	if err := fsx.RemoveAll(target); err != nil {
		return err
	}
	return os.Rename(temp, target)
}

func writeBytesIfChanged(path string, data []byte) error {
	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, data) {
		return nil
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o666)
}
