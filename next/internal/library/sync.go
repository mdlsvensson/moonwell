// Package library brings a project's libraries of modules up to date: GitHub tag archives and local folders are
// copied into .moonwell/libraries/ and .moonwell/library-assets/, and moonwell.lock records what the tags resolved
// to.
//
// It takes the manifest's libraries and the outside world (the project folder, the network and the log), and
// returns where each library lies in the project, or the first failure. Beside Sync it reads a library's own file
// (ParseFile), reads and writes the lock (ReadLock, WriteLock), and lists the folders that a sync reads the local
// libraries from (Locals), for what watches them.
//
// It knows nothing of modules, of maps or of what is done with the files. In a project it writes and removes
// below its two folders and the lock file only; a local library's own folder is read and never written.
//
// Of Moonwell it imports manifest, env, diag and fsx.
package library

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
)

const (
	ModulesDir = ".moonwell/libraries"      // <key>/ holds a library's modules
	AssetsDir  = ".moonwell/library-assets" // <key>/ holds the files a library ships for the map
)

const (
	// stampFile is the file in a library's module folder that says what its two folders hold: the lock entry of a
	// downloaded library, or the folder a local library is copied from.
	stampFile = ".moonwell-library.json"
	// stampLayout is the layout of the two folders that a stamp vouches for. A library whose stamp names another
	// layout, or none, is downloaded again.
	stampLayout = 2
)

// Synced is a library as it lies in the project after a sync. Both folders are paths from the project folder,
// with "/".
type Synced struct {
	Key     string
	Modules string
	Assets  string // "" when the library ships no files
}

// Sync brings both folders up to date for every library, and moonwell.lock with the GitHub ones. A GitHub library
// whose folders hold its lock entry is not downloaded again; a local library keeps the lock entry it had. It
// returns the libraries sorted by key. manifestFile names the manifest in errors.
func Sync(ctx context.Context, e *env.Env, libraries map[string]manifest.Library, manifestFile string) ([]Synced, error) {
	keys := slices.Sorted(maps.Keys(libraries))
	if err := refuseKeys(keys, manifestFile); err != nil {
		return nil, err
	}
	// Before anything is removed: the lock is read after the folders of libraries that left are gone.
	if err := refuseLinkedLock(e.Root); err != nil {
		return nil, err
	}
	// Before any library is synced: where letter case is ignored, a folder lib that is left of another library
	// would take the files of the library Lib, and be removed afterwards.
	if err := removeStale(e.Root, keys); err != nil {
		return nil, err
	}
	lock, err := ReadLock(e.Root)
	if err != nil {
		return nil, err
	}
	synced, entries, err := syncEach(ctx, e, keys, libraries, lock, manifestFile)
	if err != nil {
		return nil, err
	}
	if err := WriteLock(e.Root, entries); err != nil {
		return nil, err
	}
	return synced, nil
}

// refuseKeys refuses keys that cannot each name a folder of its own on every system: a key that Windows cannot
// hold as a folder's name, and two keys that differ only in letter case, which would share one folder there.
func refuseKeys(keys []string, manifestFile string) error {
	spelled := map[string]string{} // each key by its spelling in lower case
	for _, key := range keys {
		if !isKey(key) {
			return errNotAKey(key)
		}
		if _, portable := fsx.RelPath(key); !portable {
			return errUnusableKey(key, manifestFile)
		}
		if other, taken := spelled[strings.ToLower(key)]; taken {
			return errKeysDifferByCase(other, key, manifestFile)
		}
		spelled[strings.ToLower(key)] = key
	}
	return nil
}

// keyCharacters is what a library's key is made of.
const keyCharacters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-"

// isKey reports whether key is one the manifest lets through: one or more ASCII letters, digits, "_" and "-". Such
// a key is one name of a folder, and so is the "." before it and the ".tmp" after it.
func isKey(key string) bool {
	return key != "" && !strings.ContainsFunc(key, func(r rune) bool { return !strings.ContainsRune(keyCharacters, r) })
}

// removeStale removes, from each of the two folders, every entry whose name is not the key of a library: the
// folders of libraries that left the manifest, and the .<key>.tmp folders that a sync which was interrupted
// left. The two folders are the program's own: a project keeps nothing else in them. An entry that is a link is
// removed as the link it is, and what it points to stays.
func removeStale(root string, keys []string) error {
	// Both folders are reached before anything is removed from either: a link at the second is refused with the
	// first as it was.
	modules, err := inProject(root, ModulesDir, ModulesDir)
	if err != nil {
		return err
	}
	assets, err := inProject(root, AssetsDir, AssetsDir)
	if err != nil {
		return err
	}
	if err := removeOthers(modules, ModulesDir, keys); err != nil {
		return err
	}
	return removeOthers(assets, AssetsDir, keys)
}

// removeOthers removes every entry of folder whose name is not among the keys. dir names the folder in errors.
func removeOthers(folder, dir string, keys []string) error {
	entries, err := os.ReadDir(folder)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return errUnwritable(dir, err)
	}
	for _, entry := range entries {
		if slices.Contains(keys, entry.Name()) {
			continue
		}
		if err := fsx.RemoveAll(filepath.Join(folder, entry.Name())); err != nil {
			return errUnwritable(dir+"/"+entry.Name(), err)
		}
	}
	return nil
}

// syncEach syncs the libraries in the order of the keys, and returns where each lies and the entries of the lock.
func syncEach(
	ctx context.Context, e *env.Env, keys []string, libraries map[string]manifest.Library, lock map[string]LockEntry,
	manifestFile string,
) ([]Synced, map[string]LockEntry, error) {
	synced, entries := make([]Synced, 0, len(keys)), map[string]LockEntry{}
	for _, key := range keys {
		// A sync that is stopped ends between two libraries, with the context's own error: a local library is
		// synced without a download that would see it.
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		var locked *LockEntry
		if entry, isLocked := lock[key]; isLocked {
			locked = &entry
		}
		lies, entry, err := syncOne(ctx, e, key, libraries[key], locked, manifestFile)
		if err != nil {
			return nil, nil, err
		}
		synced = append(synced, lies)
		if entry != nil {
			entries[key] = *entry
		}
	}
	return synced, entries, nil
}

// syncOne syncs one library, local or of GitHub. locked is the entry the lock has for it, nil when it has none.
// It returns where the library lies and the entry the lock gets for it, nil for none.
func syncOne(
	ctx context.Context, e *env.Env, key string, library manifest.Library, locked *LockEntry, manifestFile string,
) (Synced, *LockEntry, error) {
	at, err := foldersOf(e.Root, key)
	if err != nil {
		return Synced{}, nil, err
	}
	switch {
	case library.Path != nil:
		shipsAssets, err := syncLocal(e.Root, at, *library.Path, library.Dir, manifestFile)
		if err != nil {
			return Synced{}, nil, err
		}
		// A path usually comes from moonwell.local.pkl, which is not committed. The entry of the committed lock
		// stays, so that the tag is checked against it when the library comes from GitHub again.
		return at.synced(shipsAssets), locked, nil
	case library.GitHub == nil || library.Tag == nil:
		return Synced{}, nil, errNeitherLocalNorOfGitHub(key)
	}
	entry, err := syncGitHub(ctx, e, at, library, locked, manifestFile)
	if err != nil {
		return Synced{}, nil, err
	}
	return at.synced(entry.Assets != nil), &entry, nil
}

// ---- the folders of a library ----

// folders is where one library lies in a project.
type folders struct {
	key     string
	modules string // ModulesDir/<key>, as a path on disk
	assets  string // AssetsDir/<key>, as a path on disk
}

// modulesOf and assetsOf are the two folders of a library as paths from the project folder.
func modulesOf(key string) string { return ModulesDir + "/" + key }
func assetsOf(key string) string  { return AssetsDir + "/" + key }

// foldersOf is the two folders of the library key in the project at root. A link at either, or on the way to
// one, is refused.
func foldersOf(root, key string) (folders, error) {
	modules, err := inProject(root, modulesOf(key), modulesOf(key))
	if err != nil {
		return folders{}, err
	}
	assets, err := inProject(root, assetsOf(key), assetsOf(key))
	if err != nil {
		return folders{}, err
	}
	return folders{key, modules, assets}, nil
}

// synced is what Sync returns for the library.
func (f folders) synced(shipsAssets bool) Synced {
	lies := Synced{Key: f.key, Modules: modulesOf(f.key)}
	if shipsAssets {
		lies.Assets = assetsOf(f.key)
	}
	return lies
}

// inProject is the path on disk of a file or folder that Sync writes or removes: path, written from the project
// folder with "/". A link at it or on the way to it is refused, since what is written or removed through a link
// lies somewhere else. A failure of the system is one of writing the folder label.
func inProject(root, path, label string) (string, error) {
	onDisk, err := fsx.SafeJoin(root, path)
	var refused *diag.Error
	switch {
	case err == nil:
		return onDisk, nil
	case errors.As(err, &refused):
		return "", errRefusedPath(path, refused)
	}
	return "", errUnwritable(label, err)
}

// removeAssets removes the folder of the files a library ships for the map: the library ships none.
func removeAssets(at folders) error {
	if err := fsx.RemoveAll(at.assets); err != nil {
		return errUnwritable(assetsOf(at.key), err)
	}
	return nil
}

// ---- what a library ships ----

// shipped is the files that are kept of a library: its modules, and the files for the map when it names a folder
// of them. Each file's name is its path below the folder it is kept from.
type shipped struct {
	modules     []file
	assets      []file
	shipsAssets bool // the library names a folder of files for the map
	local       bool // the library is a folder of the user's own, whose files the user can rename
}

// refuseUnusable refuses a library with a file that not every system can hold where it is written.
func (s shipped) refuseUnusable(key, manifestFile string) error {
	if err := s.refuseUnusableNames(key, "module", s.modules, manifestFile); err != nil {
		return err
	}
	return s.refuseUnusableNames(key, "assets", s.assets, manifestFile)
}

// refuseUnusableNames refuses files of one folder of a library that a folder cannot hold on every system: a file
// with a name Windows cannot hold, two files whose paths differ only in letter case, which are one file there, a
// file and a folder whose paths do, which are one name there, and two files in folders that differ only in
// letter case, which are one folder there. kind names the folder: "module" or "assets".
func (s shipped) refuseUnusableNames(key, kind string, files []file, manifestFile string) error {
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = f.name
	}
	slices.Sort(names)
	spelled := map[string]string{} // each path by its spelling in lower case
	for _, name := range names {
		if _, portable := fsx.RelPath(name); !portable || !insideLibrary(name) {
			return errUnusableName(key, kind, name, manifestFile, s.local)
		}
		if other, taken := spelled[strings.ToLower(name)]; taken {
			return errTwoSpellings(key, kind, other, name, manifestFile, s.local)
		}
		spelled[strings.ToLower(name)] = name
	}
	if first, second, found := fileAndFolderOfTwoSpellings(names, spelled); found {
		return errTwoSpellings(key, kind, first, second, manifestFile, s.local)
	}
	if first, second, found := inFoldersOfTwoSpellings(names); found {
		return errFoldersOfTwoSpellings(key, kind, first, second, manifestFile, s.local)
	}
	return nil
}

// fileAndFolderOfTwoSpellings finds, among the paths, which are in byte order, a file and a folder that one of
// them lies in whose paths differ only in letter case: the two as they are spelled, the first by bytes first.
// spelled is each of the paths by its spelling in lower case.
//
// A file and a folder of one spelling are not found. They are one name on every system, which a library's
// folder cannot hold twice anywhere: the write of the second fails by itself.
func fileAndFolderOfTwoSpellings(paths []string, spelled map[string]string) (first, second string, found bool) {
	for _, path := range paths {
		for i, c := range path {
			if c != '/' {
				continue
			}
			folder := path[:i]
			if file, taken := spelled[strings.ToLower(folder)]; taken && file != folder {
				return min(file, folder), max(file, folder), true
			}
		}
	}
	return "", "", false
}

// inFoldersOfTwoSpellings finds two of the paths, which are in byte order, that lie in folders whose paths differ
// only in letter case: for the first folder that is spelled in two ways, the first path of each spelling.
func inFoldersOfTwoSpellings(paths []string) (first, second string, found bool) {
	type spelling struct{ folder, path string } // a folder as it is spelled, and the first path that lies in it
	spelled := map[string]spelling{}            // each folder by its spelling in lower case
	for _, path := range paths {
		for i, c := range path {
			if c != '/' {
				continue
			}
			folder := path[:i]
			switch other, taken := spelled[strings.ToLower(folder)]; {
			case !taken:
				spelled[strings.ToLower(folder)] = spelling{folder, path}
			case other.folder != folder:
				return other.path, path, true
			}
		}
	}
	return "", "", false
}

// ---- the stamp ----

// stampOf is the stamp of a downloaded library: its lock entry, and the layout of its folders.
func stampOf(entry LockEntry) string {
	return objectText(append(entryMembers(entry), member{"layout", strconv.Itoa(stampLayout)}), "") + "\n"
}

// stampOfFolder is the stamp of a local library: the folder its modules are copied from.
func stampOfFolder(source string) string {
	return objectText([]member{{"path", fsx.Quoted(source)}}, "") + "\n"
}

// ---- errors ----

// errNotAKey is a plain error: the manifest's schema lets no such key through, so only a caller that did not
// read the libraries from a manifest can hand one over. It is refused all the same, since a key becomes the name
// of a folder that is written and removed.
func errNotAKey(key string) error {
	return fmt.Errorf("library: %q is not a library key: a key is made of ASCII letters, digits, _ and -", key)
}

// errNeitherLocalNorOfGitHub is a plain error: the manifest's schema gives every library a path, or a
// repository and a tag.
func errNeitherLocalNorOfGitHub(key string) error {
	return fmt.Errorf("library: the library %s has no path, and not both a repository and a tag", key)
}

func errUnusableKey(key, manifestFile string) error {
	return &diag.Error{
		Msg:  "Library " + key + ": its key is a name that Windows keeps for a device.",
		File: manifestFile,
		Hint: "Give the library another key: each library gets a folder of its key's name in " + ModulesDir +
			"/, and Windows cannot make a folder named " + key + ".",
	}
}

func errKeysDifferByCase(first, second, manifestFile string) error {
	return &diag.Error{
		Msg:  "Libraries " + first + " and " + second + " differ only by case.",
		File: manifestFile,
		Hint: "Rename one of them: each library gets a folder in " + ModulesDir + "/.",
	}
}

// errRefusedPath is the refusal of a path as fsx words it, with the path from the project folder as its file.
func errRefusedPath(path string, refused *diag.Error) error {
	named := *refused
	named.File = path
	return &named
}

// errUnwritable is a failure to write or remove at path, a folder below .moonwell/ written from the project
// folder.
func errUnwritable(path string, cause error) error {
	return &diag.Error{
		Msg:   "Writing " + path + " failed: " + reasonOf(cause),
		File:  path,
		Hint:  "Close programs that have files in .moonwell/ open, then retry.",
		Cause: cause,
	}
}

// unusableHint is the hint of a refusal of files that cannot be used, for the one who can act on it: the author
// of a downloaded library, or the user, whose own folder a local library is. The user can rename what, or have
// the library's folder of that kind be one without them: the module folder is the dir of the manifest or of the
// library's file, and the folder of the files for the map is the assets of the library's file.
func unusableHint(local bool, kind, what, them string) string {
	switch {
	case !local:
		return reportHint
	case kind == "assets":
		return "Rename " + what + " in the library, or set assets in the library's " + File + " to a folder without " + them + "."
	}
	return "Rename " + what + " in the library, or set the library's dir to a folder without " + them + "."
}

func errUnusableName(key, kind, name, manifestFile string, local bool) error {
	return &diag.Error{
		Msg:  "Library " + key + ": " + name + " in its " + kind + " folder has a name that Windows cannot hold.",
		File: manifestFile,
		Hint: unusableHint(local, kind, "the file", "it"),
	}
}

func errTwoSpellings(key, kind, first, second, manifestFile string, local bool) error {
	return &diag.Error{
		Msg:  "Library " + key + ": " + first + " and " + second + " in its " + kind + " folder differ only in letter case.",
		File: manifestFile,
		Hint: unusableHint(local, kind, "one of them", "them"),
	}
}

func errFoldersOfTwoSpellings(key, kind, first, second, manifestFile string, local bool) error {
	return &diag.Error{
		Msg: "Library " + key + ": " + first + " and " + second + " in its " + kind +
			" folder lie in folders that differ only in letter case.",
		File: manifestFile,
		Hint: unusableHint(local, kind, "one of the two folders", "them"),
	}
}
