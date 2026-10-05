package build

import (
	"os"
	"path/filepath"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
)

// This file holds the places Moonwell writes what it builds: real folders of the project.

const (
	// distDir is the folder of a project that Moonwell writes what it builds into, from the project folder.
	distDir = "dist"
	// stageDir is the folder the maps are staged in, from the project folder.
	stageDir = distDir + "/stage"
)

// outputAt is the way to every place Moonwell writes what it builds: relative is the place from the project
// folder, with "/", such as dist/.lock. Nothing need be at the place.
//
// The place is reached with fsx.Inside, so every folder on the way to it is a real folder of the project, the
// first too: what a build writes, and what it removes to write it, stays in the project. A link on the way, or
// at the place, is refused in words of this package's own, since what fsx.Inside says of a link is said of files
// the user keeps, and these are Moonwell's to make. fsx.Inside's other refusals are passed on: a path that leaves
// the project, a name Windows cannot hold, and a way the system cannot look at.
func outputAt(root, relative string) (string, error) {
	place, err := fsx.Inside(root, relative)
	if err == nil {
		return place, nil
	}
	if link, found := linkOnTheWay(root, relative); found {
		return "", errLinkedOutput(link, relative)
	}
	return "", err
}

// linkOnTheWay is the first step of the way to relative that is a link, as a path from the project folder with
// "/". It is found by a look at each step, the first step first, and not by what a refusal says: fsx names no
// kind of refusal. A path that fsx.RelPath does not take has no steps to look at.
func linkOnTheWay(root, relative string) (link string, found bool) {
	slashed, portable := fsx.RelPath(relative)
	if !portable {
		return "", false
	}
	for end := 1; end <= len(slashed); end++ {
		if end < len(slashed) && slashed[end] != '/' {
			continue
		}
		info, err := fsx.Lstat(filepath.Join(root, filepath.FromSlash(slashed[:end])))
		if err != nil || info == nil {
			return "", false // nothing is below a step that is not there, or that cannot be looked at
		}
		if fsx.IsLink(info) {
			return slashed[:end], true
		}
	}
	return "", false
}

// place is a file or a folder that Moonwell writes.
type place struct {
	file  string // where it is on disk
	label string // its path from the project folder, with "/": how messages name it
}

// placeOf is a place Moonwell writes, with the way to it looked at; label is its path from the project folder,
// with "/". The refusals are those of outputAt, and that of a file where a folder on the way belongs.
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

// fileOnTheWay is the first folder on the way to label that is a file; label is a path from the project folder,
// with "/". What stands at label itself is not looked at.
//
// outputAt takes a file on the way for a place that nothing is at, and what a system then says of a write or a
// removal below the file differs from system to system. So the file is refused by its name, before either.
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

// labelOf is a file at or below the place as its path from the project folder; file is where it is on disk. A
// file that is not below the place is named as the place.
func (at place) labelOf(file string) string {
	below, err := filepath.Rel(at.file, file)
	if err != nil || below == "." || !filepath.IsLocal(below) {
		return at.label
	}
	return at.label + "/" + filepath.ToSlash(below)
}

// ---- errors ----

func errFileForFolder(file, wanted string) error {
	return &diag.Error{
		Msg:  file + " is a file, not a folder.",
		File: file,
		Hint: "Moonwell writes " + wanted + " below it: remove or rename the file, then try again.",
	}
}

// errLinkedOutput is the refusal of a link on the way to a place Moonwell writes: link is the step that is one,
// and file the place, both from the project folder.
func errLinkedOutput(link, file string) error {
	return &diag.Error{
		Msg:  link + " is a link: Moonwell writes what it builds into real files and folders.",
		File: file,
		Hint: "Remove the link (or Windows junction) at " + link + ", then try again: Moonwell makes what it needs " +
			"there.",
	}
}
