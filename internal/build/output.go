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
)

func outputPath(root, relative string) (string, error) {
	place, err := fsx.SafeJoinNoSymlinks(root, relative)
	if err == nil {
		return place, nil
	}
	if link, found := findSymlinkOnPath(root, relative); found {
		return "", errLinkedOutput(link, relative)
	}
	return "", err
}

func findSymlinkOnPath(root, relative string) (link string, found bool) {
	slashed, portable := fsx.CleanRelPath(relative)
	if !portable {
		return "", false
	}
	for end := 1; end <= len(slashed); end++ {
		if end < len(slashed) && slashed[end] != '/' {
			continue
		}
		info, err := fsx.Lstat(filepath.Join(root, filepath.FromSlash(slashed[:end])))
		if err != nil || info == nil {
			return "", false
		}
		if fsx.IsSymlink(info) {
			return slashed[:end], true
		}
	}
	return "", false
}

type outputFile struct {
	fullPath    string
	displayPath string
}

func newOutputFile(root, label string) (outputFile, error) {
	file, err := outputPath(root, label)
	if err != nil {
		return outputFile{}, err
	}
	if blocking, found := findBlockingFile(root, label); found {
		return outputFile{}, errFileForFolder(blocking, label)
	}
	return outputFile{fullPath: file, displayPath: label}, nil
}

func findBlockingFile(root, label string) (file string, found bool) {
	for at, char := range label {
		if char != '/' {
			continue
		}
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(label[:at])))
		if err == nil && !info.IsDir() {
			return label[:at], true
		}
	}
	return "", false
}

func (f outputFile) displayPathOf(file string) string {
	below, err := filepath.Rel(f.fullPath, file)
	if err != nil || below == "." || !filepath.IsLocal(below) {
		return f.displayPath
	}
	return f.displayPath + "/" + filepath.ToSlash(below)
}

func errFileForFolder(file, wanted string) error {
	return &diag.Error{
		Msg:  file + " is a file, not a folder.",
		File: file,
		Hint: "Moonwell writes " + wanted + " below it: remove or rename the file, then try again.",
	}
}

func errLinkedOutput(link, file string) error {
	return &diag.Error{
		Msg:  link + " is a link: Moonwell writes what it builds into real files and folders.",
		File: file,
		Hint: "Remove the link (or Windows junction) at " + link + ", then try again: Moonwell makes what it needs " +
			"there.",
	}
}
