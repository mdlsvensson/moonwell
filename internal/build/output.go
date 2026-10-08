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

func outputAt(root, relative string) (string, error) {
	place, err := fsx.SafeJoinNoSymlinks(root, relative)
	if err == nil {
		return place, nil
	}
	if link, found := linkOnTheWay(root, relative); found {
		return "", errLinkedOutput(link, relative)
	}
	return "", err
}

func linkOnTheWay(root, relative string) (link string, found bool) {
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

type place struct {
	file  string
	label string
}

func placeOf(root, label string) (place, error) {
	file, err := outputAt(root, label)
	if err != nil {
		return place{}, err
	}
	if blocking, found := fileOnTheWay(root, label); found {
		return place{}, errFileForFolder(blocking, label)
	}
	return place{file: file, label: label}, nil
}

func fileOnTheWay(root, label string) (file string, found bool) {
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

func (at place) labelOf(file string) string {
	below, err := filepath.Rel(at.file, file)
	if err != nil || below == "." || !filepath.IsLocal(below) {
		return at.label
	}
	return at.label + "/" + filepath.ToSlash(below)
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
