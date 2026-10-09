package build

import (
	"os"
	"path/filepath"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

const (
	distDir  = "dist"
	stageDir = distDir + "/stage"
	testDir  = distDir + "/test"
)

type outputFile struct {
	fullPath    string
	displayPath string
}

func newOutputFile(root, displayPath string) (outputFile, error) {
	fullPath, err := outputPath(root, displayPath)
	if err != nil {
		return outputFile{}, err
	}
	if blocking, found := findBlockingFile(root, displayPath); found {
		return outputFile{}, errFileForFolder(blocking, displayPath)
	}
	return outputFile{fullPath: fullPath, displayPath: displayPath}, nil
}

func outputPath(root, relative string) (string, error) {
	fullPath, err := fsx.SafeJoinNoSymlinks(root, relative)
	if err == nil {
		return fullPath, nil
	}
	if symlink, found := findSymlinkOnPath(root, relative); found {
		return "", errLinkedOutput(symlink, relative)
	}
	return "", err
}

func findSymlinkOnPath(root, relative string) (symlink string, found bool) {
	path, ok := fsx.CleanRelPath(relative)
	if !ok {
		return "", false
	}
	for end := 1; end <= len(path); end++ {
		if end < len(path) && path[end] != '/' {
			continue
		}
		info, err := fsx.Lstat(filepath.Join(root, filepath.FromSlash(path[:end])))
		if err != nil || info == nil {
			return "", false
		}
		if fsx.IsSymlink(info) {
			return path[:end], true
		}
	}
	return "", false
}

func findBlockingFile(root, displayPath string) (file string, found bool) {
	for index, char := range displayPath {
		if char != '/' {
			continue
		}
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(displayPath[:index])))
		if err == nil && !info.IsDir() {
			return displayPath[:index], true
		}
	}
	return "", false
}

func (f outputFile) displayPathOf(fullPath string) string {
	rel, err := filepath.Rel(f.fullPath, fullPath)
	if err != nil || rel == "." || !filepath.IsLocal(rel) {
		return f.displayPath
	}
	return f.displayPath + "/" + filepath.ToSlash(rel)
}

func errFileForFolder(path, wantedPath string) error {
	return &diag.Error{
		Msg:  path + " is a file, not a folder.",
		File: path,
		Hint: "Moonwell writes " + wantedPath + " below it: remove or rename the file, then try again.",
	}
}

func errLinkedOutput(symlink, path string) error {
	return &diag.Error{
		Msg:  symlink + " is a link: Moonwell writes what it builds into real files and folders.",
		File: path,
		Hint: "Remove the link (or Windows junction) at " + symlink + ", then try again: Moonwell makes what it needs " +
			"there.",
	}
}
