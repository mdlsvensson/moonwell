package assets

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
)

func (c *collector) addLibraryAssets(root string, library Library) error {
	displayPath := displayPathOf(root, library.Dir)
	if err := checkNotSymlink(library.Dir, displayPath); err != nil {
		return err
	}
	folder, err := openDir(library.Dir, displayPath)
	if err != nil {
		return blameLibrary(err, library.Key, displayPath)
	}
	for _, source := range sortedFiles(folder) {
		if hasHiddenSegment(source) {
			continue
		}
		if err := c.addLibraryFile(folder, library.Key, displayPath, source); err != nil {
			return err
		}
	}
	return nil
}

func displayPathOf(root, dir string) string {
	if rel, err := filepath.Rel(root, dir); err == nil && filepath.IsLocal(rel) {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(dir)
}

func checkNotSymlink(dir, displayPath string) error {
	if info, err := fsx.Lstat(dir); err == nil && info != nil && fsx.IsSymlink(info) {
		return errLinkedFolder(dir, displayPath)
	}
	return nil
}

func (c *collector) addLibraryFile(folder *mapdir.Folder, library, displayPath, source string) error {
	target, err := parseTargetPath(source)
	if err != nil {
		return blameLibrary(err, library, displayPath)
	}
	if existing, taken := c.byTarget[mapdir.Key(target)]; taken {
		return c.replaceAsset(existing, library, source, target)
	}
	return blameLibrary(c.add(folder, Asset{Source: source, Library: library, Target: target}), library, displayPath)
}

func (c *collector) replaceAsset(existing Asset, library, source, target string) error {
	if existing.Library != "" {
		return errTwoLibraries(c.manifestName, existing.Library, library, target)
	}
	c.replaced = append(c.replaced, "assets/"+existing.Source+" replaces library "+library+"'s "+source)
	return nil
}

func blameLibrary(err error, library, displayPath string) error {
	var diagErr *diag.Error
	if !errors.As(err, &diagErr) || diagErr.Cause != nil {
		return err
	}
	return errInLibrary(library, displayPath, diagErr)
}

func errTwoLibraries(manifestName, first, second, target string) error {
	return &diag.Error{
		Msg:  "Libraries " + first + " and " + second + " both import " + strings.ReplaceAll(target, "/", `\`) + ".",
		File: manifestName,
		Hint: "Drop one of the libraries, or put your own file at that path under assets/ to replace both.",
	}
}

func errLinkedFolder(dir, displayPath string) error {
	err := fsx.NewSymlinkError(dir)
	var diagErr *diag.Error
	if errors.As(err, &diagErr) {
		diagErr.File = displayPath
	}
	return err
}

func errInLibrary(library, displayPath string, diagErr *diag.Error) error {
	return &diag.Error{
		Msg:   "Library " + library + ": " + diagErr.Msg,
		File:  displayPath,
		Hint:  "Report it to the library's author, or use another version of the library.",
		Cause: diagErr,
	}
}
