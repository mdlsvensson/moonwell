package library

import (
	"cmp"
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
	seen := map[string]string{}
	for _, key := range keys {
		if !isValidKey(key) {
			return errNotAKey(key)
		}
		if _, ok := fsx.CleanRelPath(key); !ok {
			return errUnusableKey(key, manifestName)
		}
		if existing, taken := seen[strings.ToLower(key)]; taken {
			return errKeysDifferByCase(existing, key, manifestName)
		}
		seen[strings.ToLower(key)] = key
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

func removeUnlisted(fullPath, dir string, keys []string) error {
	entries, err := os.ReadDir(fullPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return errUnwritable(dir, err)
	}
	for _, entry := range entries {
		if slices.Contains(keys, entry.Name()) {
			continue
		}
		if err := fsx.RemoveAll(filepath.Join(fullPath, entry.Name())); err != nil {
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
		result, entry, err := syncOne(ctx, e, key, libraries[key], locked, manifestName)
		if err != nil {
			return nil, nil, err
		}
		synced = append(synced, result)
		if entry != nil {
			entries[key] = *entry
		}
	}
	return synced, entries, nil
}

func syncOne(
	ctx context.Context, e *env.Env, key string, library manifest.Library, locked *lockEntry, manifestName string,
) (Synced, *lockEntry, error) {
	dirs, err := libraryDirsOf(e.Root, key)
	if err != nil {
		return Synced{}, nil, err
	}
	switch {
	case library.Path != nil:
		if library.OverriddenIn != "" {
			e.Log.Info("Library " + key + ": the local folder " + *library.Path + " (" + library.OverriddenIn + ").")
		}
		shipsAssets, err := syncLocal(e.Root, dirs, *library.Path, library.Dir, cmp.Or(library.OverriddenIn, manifestName))
		if err != nil {
			return Synced{}, nil, err
		}
		return dirs.toSynced(shipsAssets), locked, nil
	case library.GitHub == nil || library.Tag == nil:
		return Synced{}, nil, errNoSource(key)
	}
	entry, err := syncGitHub(ctx, e, dirs, library, locked, manifestName)
	if err != nil {
		return Synced{}, nil, err
	}
	return dirs.toSynced(entry.Assets != nil), &entry, nil
}

type libraryDirs struct {
	key     string
	modules string
	assets  string
}

func modulesDirName(key string) string { return ModulesDir + "/" + key }

func assetsDirName(key string) string { return AssetsDir + "/" + key }

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

func (d libraryDirs) toSynced(shipsAssets bool) Synced {
	synced := Synced{Key: d.key, Modules: modulesDirName(d.key)}
	if shipsAssets {
		synced.Assets = assetsDirName(d.key)
	}
	return synced
}

func removeAssets(dirs libraryDirs) error {
	if err := fsx.RemoveAll(dirs.assets); err != nil {
		return errUnremovable(assetsDirName(dirs.key), err)
	}
	return nil
}

func stampText(entry lockEntry) string {
	return formatObject(append(lockEntryMembers(entry), jsonMember{"layout", strconv.Itoa(stampLayout)}), "") + "\n"
}

func localStampText(source string) string {
	return formatObject([]jsonMember{{"path", fsx.QuoteJSON(source)}}, "") + "\n"
}

func errNotAKey(key string) error {
	return fmt.Errorf("library: %q is not a library key: a key is made of ASCII letters, digits, _ and -", key)
}

func errNoSource(key string) error {
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
	return &diag.Error{Msg: "Writing " + path + " failed: " + describeError(cause), File: path, Hint: folderHint, Cause: cause}
}

func errUnremovable(path string, cause error) error {
	return &diag.Error{Msg: "Removing " + path + " failed: " + describeError(cause), File: path, Hint: folderHint, Cause: cause}
}
