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

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
)

const (
	ModulesDir = ".moonwell/libraries"
	AssetsDir  = ".moonwell/library-assets"
)

const (
	stampFile   = ".moonwell-library.json"
	stampLayout = 2
)

type Synced struct {
	Key     string
	Modules string
	Assets  string
}

func Sync(ctx context.Context, e *env.Env, libraries map[string]manifest.Library, manifestName string) ([]Synced, error) {
	keys := slices.Sorted(maps.Keys(libraries))
	if err := checkKeys(keys, manifestName); err != nil {
		return nil, err
	}
	if _, err := lockPath(e.Root); err != nil {
		return nil, err
	}
	if err := removeStale(e.Root, keys); err != nil {
		return nil, err
	}
	lock, err := readLock(e.Root)
	if err != nil {
		return nil, err
	}
	synced, entries, err := syncEach(ctx, e, keys, libraries, lock, manifestName)
	if err != nil {
		return nil, err
	}
	if err := writeLock(e.Root, entries); err != nil {
		return nil, err
	}
	return synced, nil
}

func checkKeys(keys []string, manifestName string) error {
	spelled := map[string]string{}
	for _, key := range keys {
		if !isValidKey(key) {
			return errNotAKey(key)
		}
		if _, portable := fsx.CleanRelPath(key); !portable {
			return errUnusableKey(key, manifestName)
		}
		if other, taken := spelled[strings.ToLower(key)]; taken {
			return errKeysDifferByCase(other, key, manifestName)
		}
		spelled[strings.ToLower(key)] = key
	}
	return nil
}

const keyCharacters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-"

func isValidKey(key string) bool {
	return key != "" && !strings.ContainsFunc(key, func(r rune) bool { return !strings.ContainsRune(keyCharacters, r) })
}

func removeStale(root string, keys []string) error {
	modules, err := fsx.SafeJoinNoSymlinks(root, ModulesDir)
	if err != nil {
		return err
	}
	assets, err := fsx.SafeJoinNoSymlinks(root, AssetsDir)
	if err != nil {
		return err
	}
	if err := removeUnlisted(modules, ModulesDir, keys); err != nil {
		return err
	}
	return removeUnlisted(assets, AssetsDir, keys)
}

func removeUnlisted(folder, dir string, keys []string) error {
	entries, err := os.ReadDir(folder)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return errUnwritable(dir, err)
	}
	for _, entry := range entries {
		if slices.Contains(keys, entry.Name()) {
			continue
		}
		if err := fsx.RemoveAll(filepath.Join(folder, entry.Name())); err != nil {
			return errUnremovable(dir+"/"+entry.Name(), err)
		}
	}
	return nil
}

func syncEach(
	ctx context.Context, e *env.Env, keys []string, libraries map[string]manifest.Library, lock map[string]lockEntry,
	manifestName string,
) ([]Synced, map[string]lockEntry, error) {
	synced, entries := make([]Synced, 0, len(keys)), map[string]lockEntry{}
	for _, key := range keys {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		var locked *lockEntry
		if entry, isLocked := lock[key]; isLocked {
			locked = &entry
		}
		lies, entry, err := syncOne(ctx, e, key, libraries[key], locked, manifestName)
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

func syncOne(
	ctx context.Context, e *env.Env, key string, library manifest.Library, locked *lockEntry, manifestName string,
) (Synced, *lockEntry, error) {
	at, err := libraryDirsOf(e.Root, key)
	if err != nil {
		return Synced{}, nil, err
	}
	switch {
	case library.Path != nil:
		shipsAssets, err := syncLocal(e.Root, at, *library.Path, library.Dir, manifestName)
		if err != nil {
			return Synced{}, nil, err
		}
		return at.toSynced(shipsAssets), locked, nil
	case library.GitHub == nil || library.Tag == nil:
		return Synced{}, nil, errNeitherLocalNorOfGitHub(key)
	}
	entry, err := syncGitHub(ctx, e, at, library, locked, manifestName)
	if err != nil {
		return Synced{}, nil, err
	}
	return at.toSynced(entry.Assets != nil), &entry, nil
}

type libraryDirs struct {
	key     string
	modules string
	assets  string
}

func modulesDirName(key string) string { return ModulesDir + "/" + key }
func assetsDirName(key string) string  { return AssetsDir + "/" + key }

func libraryDirsOf(root, key string) (libraryDirs, error) {
	modules, err := fsx.SafeJoinNoSymlinks(root, modulesDirName(key))
	if err != nil {
		return libraryDirs{}, err
	}
	assets, err := fsx.SafeJoinNoSymlinks(root, assetsDirName(key))
	if err != nil {
		return libraryDirs{}, err
	}
	return libraryDirs{key, modules, assets}, nil
}

func (f libraryDirs) toSynced(shipsAssets bool) Synced {
	lies := Synced{Key: f.key, Modules: modulesDirName(f.key)}
	if shipsAssets {
		lies.Assets = assetsDirName(f.key)
	}
	return lies
}

func removeAssets(at libraryDirs) error {
	if err := fsx.RemoveAll(at.assets); err != nil {
		return errUnremovable(assetsDirName(at.key), err)
	}
	return nil
}

type libraryContent struct {
	modules     []archiveFile
	assets      []archiveFile
	shipsAssets bool
	local       bool
}

func (s libraryContent) checkUsable(key, manifestName string) error {
	if err := s.checkUsableNames(key, "module", s.modules, manifestName); err != nil {
		return err
	}
	return s.checkUsableNames(key, "assets", s.assets, manifestName)
}

func (s libraryContent) checkUsableNames(key, kind string, files []archiveFile, manifestName string) error {
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = f.name
	}
	slices.Sort(names)
	spelled := map[string]string{}
	for _, name := range names {
		if _, portable := fsx.CleanRelPath(name); !portable || !isInsideLibrary(name) {
			return errUnusableName(key, kind, name, manifestName, s.local)
		}
		if other, taken := spelled[strings.ToLower(name)]; taken {
			return errTwoSpellings(key, kind, other, name, manifestName, s.local)
		}
		spelled[strings.ToLower(name)] = name
	}
	if first, second, found := findFileDirCaseConflict(names, spelled); found {
		return errTwoSpellings(key, kind, first, second, manifestName, s.local)
	}
	if first, second, found := findDirCaseConflict(names); found {
		return errFoldersOfTwoSpellings(key, kind, first, second, manifestName, s.local)
	}
	return nil
}

func findFileDirCaseConflict(paths []string, spelled map[string]string) (first, second string, found bool) {
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

func findDirCaseConflict(paths []string) (first, second string, found bool) {
	type spelling struct{ folder, path string }
	spelled := map[string]spelling{}
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

func stampText(entry lockEntry) string {
	return formatObject(append(lockEntryMembers(entry), member{"layout", strconv.Itoa(stampLayout)}), "") + "\n"
}

func localStampText(source string) string {
	return formatObject([]member{{"path", fsx.QuoteJSON(source)}}, "") + "\n"
}

func errNotAKey(key string) error {
	return fmt.Errorf("library: %q is not a library key: a key is made of ASCII letters, digits, _ and -", key)
}

func errNeitherLocalNorOfGitHub(key string) error {
	return fmt.Errorf("library: the library %s has no path, and not both a repository and a tag", key)
}

func errUnusableKey(key, manifestName string) error {
	return &diag.Error{
		Msg:  "Library " + key + ": its key is a name that Windows keeps for a device.",
		File: manifestName,
		Hint: "Give the library another key: each library gets a folder of its key's name in " + ModulesDir +
			"/, and Windows cannot make a folder named " + key + ".",
	}
}

func errKeysDifferByCase(first, second, manifestName string) error {
	return &diag.Error{
		Msg:  "Libraries " + first + " and " + second + " differ only by case.",
		File: manifestName,
		Hint: "Rename one of them: each library gets a folder in " + ModulesDir + "/.",
	}
}

const folderHint = "Close programs that have files in .moonwell/ open, then retry."

func errUnwritable(path string, cause error) error {
	return &diag.Error{Msg: "Writing " + path + " failed: " + describeFetchError(cause), File: path, Hint: folderHint, Cause: cause}
}

func errUnremovable(path string, cause error) error {
	return &diag.Error{Msg: "Removing " + path + " failed: " + describeFetchError(cause), File: path, Hint: folderHint, Cause: cause}
}

func unusableHint(local bool, kind, what, them string) string {
	switch {
	case !local:
		return reportHint
	case kind == "assets":
		return "Rename " + what + " in the library, or set assets in the library's " + File + " to a folder without " + them + "."
	}
	return "Rename " + what + " in the library, or set the library's dir to a folder without " + them + "."
}

func errUnusableName(key, kind, name, manifestName string, local bool) error {
	return &diag.Error{
		Msg:  "Library " + key + ": " + name + " in its " + kind + " folder has a name that Windows cannot hold.",
		File: manifestName,
		Hint: unusableHint(local, kind, "the file", "it"),
	}
}

func errTwoSpellings(key, kind, first, second, manifestName string, local bool) error {
	return &diag.Error{
		Msg:  "Library " + key + ": " + first + " and " + second + " in its " + kind + " folder differ only in letter case.",
		File: manifestName,
		Hint: unusableHint(local, kind, "one of them", "them"),
	}
}

func errFoldersOfTwoSpellings(key, kind, first, second, manifestName string, local bool) error {
	return &diag.Error{
		Msg: "Library " + key + ": " + first + " and " + second + " in its " + kind +
			" folder lie in folders that differ only in letter case.",
		File: manifestName,
		Hint: unusableHint(local, kind, "one of the two folders", "them"),
	}
}
