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

func Sync(ctx context.Context, e *env.Env, libraries map[string]manifest.Library, manifestFile string) ([]Synced, error) {
	keys := slices.Sorted(maps.Keys(libraries))
	if err := refuseKeys(keys, manifestFile); err != nil {
		return nil, err
	}
	if _, err := lockAt(e.Root); err != nil {
		return nil, err
	}
	if err := removeStale(e.Root, keys); err != nil {
		return nil, err
	}
	lock, err := readLock(e.Root)
	if err != nil {
		return nil, err
	}
	synced, entries, err := syncEach(ctx, e, keys, libraries, lock, manifestFile)
	if err != nil {
		return nil, err
	}
	if err := writeLock(e.Root, entries); err != nil {
		return nil, err
	}
	return synced, nil
}

func refuseKeys(keys []string, manifestFile string) error {
	spelled := map[string]string{}
	for _, key := range keys {
		if !isKey(key) {
			return errNotAKey(key)
		}
		if _, portable := fsx.CleanRelPath(key); !portable {
			return errUnusableKey(key, manifestFile)
		}
		if other, taken := spelled[strings.ToLower(key)]; taken {
			return errKeysDifferByCase(other, key, manifestFile)
		}
		spelled[strings.ToLower(key)] = key
	}
	return nil
}

const keyCharacters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-"

func isKey(key string) bool {
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
	if err := removeOthers(modules, ModulesDir, keys); err != nil {
		return err
	}
	return removeOthers(assets, AssetsDir, keys)
}

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
			return errUnremovable(dir+"/"+entry.Name(), err)
		}
	}
	return nil
}

func syncEach(
	ctx context.Context, e *env.Env, keys []string, libraries map[string]manifest.Library, lock map[string]lockEntry,
	manifestFile string,
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

func syncOne(
	ctx context.Context, e *env.Env, key string, library manifest.Library, locked *lockEntry, manifestFile string,
) (Synced, *lockEntry, error) {
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

type folders struct {
	key     string
	modules string
	assets  string
}

func modulesOf(key string) string { return ModulesDir + "/" + key }
func assetsOf(key string) string  { return AssetsDir + "/" + key }

func foldersOf(root, key string) (folders, error) {
	modules, err := fsx.SafeJoinNoSymlinks(root, modulesOf(key))
	if err != nil {
		return folders{}, err
	}
	assets, err := fsx.SafeJoinNoSymlinks(root, assetsOf(key))
	if err != nil {
		return folders{}, err
	}
	return folders{key, modules, assets}, nil
}

func (f folders) synced(shipsAssets bool) Synced {
	lies := Synced{Key: f.key, Modules: modulesOf(f.key)}
	if shipsAssets {
		lies.Assets = assetsOf(f.key)
	}
	return lies
}

func removeAssets(at folders) error {
	if err := fsx.RemoveAll(at.assets); err != nil {
		return errUnremovable(assetsOf(at.key), err)
	}
	return nil
}

type shipped struct {
	modules     []file
	assets      []file
	shipsAssets bool
	local       bool
}

func (s shipped) refuseUnusable(key, manifestFile string) error {
	if err := s.refuseUnusableNames(key, "module", s.modules, manifestFile); err != nil {
		return err
	}
	return s.refuseUnusableNames(key, "assets", s.assets, manifestFile)
}

func (s shipped) refuseUnusableNames(key, kind string, files []file, manifestFile string) error {
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = f.name
	}
	slices.Sort(names)
	spelled := map[string]string{}
	for _, name := range names {
		if _, portable := fsx.CleanRelPath(name); !portable || !insideLibrary(name) {
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

func inFoldersOfTwoSpellings(paths []string) (first, second string, found bool) {
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

func stampOf(entry lockEntry) string {
	return objectText(append(entryMembers(entry), member{"layout", strconv.Itoa(stampLayout)}), "") + "\n"
}

func stampOfFolder(source string) string {
	return objectText([]member{{"path", fsx.QuoteJSON(source)}}, "") + "\n"
}

func errNotAKey(key string) error {
	return fmt.Errorf("library: %q is not a library key: a key is made of ASCII letters, digits, _ and -", key)
}

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

const folderHint = "Close programs that have files in .moonwell/ open, then retry."

func errUnwritable(path string, cause error) error {
	return &diag.Error{Msg: "Writing " + path + " failed: " + reasonOf(cause), File: path, Hint: folderHint, Cause: cause}
}

func errUnremovable(path string, cause error) error {
	return &diag.Error{Msg: "Removing " + path + " failed: " + reasonOf(cause), File: path, Hint: folderHint, Cause: cause}
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
