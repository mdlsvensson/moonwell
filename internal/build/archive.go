package build

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
)

// This file holds the archive as a file of the project: where a build puts it, what a build removes there, and
// what it writes there.

// mapSuffix ends the name of a map: of its folder, and of its archive.
const mapSuffix = ".w3x"

// clearedArchive is where the project's archive goes, with the archive of the build before removed.
func clearedArchive(p *manifest.Project) (place, error) {
	out, err := archiveOf(p)
	if err != nil {
		return place{}, err
	}
	if err := removeArchive(out); err != nil {
		return place{}, err
	}
	return out, nil
}

// packInto packs the planned map and writes the archive to its place, and says so before and after: packing is
// the slow part of a build. The map's name, for the archive's header, is the name of its folder without .w3x.
func packInto(e *env.Env, plan *Result, out place) error {
	e.Log.Info("Packing archive...")
	archive, err := pack(plan.Map, strings.TrimSuffix(path.Base(out.label), mapSuffix))
	if err != nil {
		return err
	}
	if err := writeArchive(out, archive); err != nil {
		return err
	}
	e.Log.Info("Built " + out.label + " (" + strconv.Itoa(len(plan.Program.Modules)) + " module(s)).")
	return nil
}

// ---- the archive's place ----

// archiveOf is where the project's archive goes: <build.folder>/<map.folder> from the project folder. Nothing
// need be at the place.
//
// A build removes what is at the place and writes a file there, so a folder at the place is refused, with the
// manifest as its file.
func archiveOf(p *manifest.Project) (place, error) {
	folder, err := mapFolder(p)
	if err != nil {
		return place{}, err
	}
	into, err := buildFolder(p, folder)
	if err != nil {
		return place{}, err
	}
	out, err := placeOf(p.Root, into+"/"+folder)
	if err != nil {
		return place{}, err
	}
	if fsx.IsDir(out.file) {
		return place{}, errOutputIsAFolder(p.File, out.label)
	}
	return out, nil
}

// buildFolder is the project's build.folder as a path from the project folder, with "/"; folder is the map's,
// which names the archive in a refusal.
//
// The value is read as readFolder reads a folder of the manifest, which is how mapFolder reads map.folder: every
// way the schema lets a folder be written names the folder, such as "dist/bin/" and "./out". The refusals have
// the manifest as their file: a value that leaves the project, one that names no folder, one with a name that
// Windows cannot hold, since the archive goes where every system can make it, and, as the schema has it, a folder
// Moonwell keeps for itself.
func buildFolder(p *manifest.Project, folder string) (string, error) {
	written := p.Build.Folder
	parts, fault := readFolder(written)
	switch fault {
	case leavesItsFolder:
		asWritten := strings.TrimRight(strings.ReplaceAll(written, `\`, "/"), "/")
		return "", errOutputOutside(p.File, asWritten+"/"+folder)
	case namesNoFolder:
		return "", errNoBuildFolder(p.File, written)
	case unusableName:
		return "", errUnusableBuildFolder(p.File, written)
	}
	into := strings.Join(parts, "/")
	if kept, isKept := keptFolder(parts); isKept {
		return "", errOutputInKeptFolder(p.File, into+"/"+folder, kept)
	}
	return into, nil
}

// keptFolder is the folder Moonwell reads from or stages into that a path of these parts is, or is in: maps,
// src or dist/stage, as isReservedFolder of schema/Project.pkl has them. The schema compares in lower case, which
// for these names is the lower case of ASCII: no other letter becomes one of theirs.
//
// An archive in one of them would stand among the source maps, among the gameplay, or in the stage of a map.
func keptFolder(parts []string) (kept string, found bool) {
	first := lowerASCII(parts[0])
	switch {
	case first == mapsDir, first == sourcesDir:
		return first, true
	case len(parts) > 1 && first+"/"+lowerASCII(parts[1]) == stageDir:
		return stageDir, true
	}
	return "", false
}

// lowerASCII is text with its ASCII letters in lower case, and every other byte as it is.
func lowerASCII(text string) string {
	lowered := []byte(text)
	for at, char := range lowered {
		if char >= 'A' && char <= 'Z' {
			lowered[at] = char + 'a' - 'A'
		}
	}
	return string(lowered)
}

// ---- what a build removes, and what it writes ----

// removeArchive removes the archive at a place: one file, and never a folder with what is in it. An archive that
// is not there is no failure.
func removeArchive(at place) error {
	err := fsx.RemoveFile(at.file)
	if err == nil {
		return nil
	}
	// fsx words a file that another program holds by its place on disk: the system's failure is its cause.
	var held *diag.Error
	if errors.As(err, &held) && held.Cause != nil {
		err = held.Cause
	}
	return errNotRemoved(at.label, err)
}

// writeArchive writes the archive to its place, with the folders on the way. An archive that could not be
// written whole is removed: a failed build leaves no archive. A failure of that removal is passed over, since
// the failure to write is the one to report.
func writeArchive(at place, archive []byte) error {
	if err := os.MkdirAll(filepath.Dir(at.file), 0o777); err != nil {
		return errArchiveNotWritten(at.label, err)
	}
	if err := os.WriteFile(at.file, archive, 0o666); err != nil {
		_ = fsx.RemoveFile(at.file)
		return errArchiveNotWritten(at.label, err)
	}
	return nil
}

// ---- errors ----

const (
	// insideHint ends a refusal of a build.folder that names no folder of the project.
	insideHint = "Set build.folder to a folder inside the project, such as dist/bin."
	// outputOnlyHint ends a refusal of a build.folder whose folder holds more than what a build writes.
	outputOnlyHint = "Set build.folder to a folder that only holds build output, such as dist/bin."
)

func errOutputOutside(manifestFile, output string) error {
	return &diag.Error{
		Msg:  "The build output " + output + " is outside the project.",
		File: manifestFile,
		Hint: insideHint,
	}
}

func errNoBuildFolder(manifestFile, written string) error {
	return &diag.Error{Msg: `build.folder names no folder: "` + written + `".`, File: manifestFile, Hint: insideHint}
}

func errUnusableBuildFolder(manifestFile, written string) error {
	return &diag.Error{
		Msg:  `build.folder has a name that Windows cannot hold: "` + written + `".`,
		File: manifestFile,
		Hint: insideHint + " " + unusableNames,
	}
}

func errOutputInKeptFolder(manifestFile, output, kept string) error {
	return &diag.Error{
		Msg:  "The build output " + output + " is in " + kept + "/, a folder Moonwell reads from or stages into.",
		File: manifestFile,
		Hint: outputOnlyHint,
	}
}

func errOutputIsAFolder(manifestFile, output string) error {
	return &diag.Error{
		Msg:  "The build output " + output + " is a directory; refusing to replace it.",
		File: manifestFile,
		Hint: outputOnlyHint,
	}
}

// archiveHint ends a failure to remove or to write the archive.
const archiveHint = "Close Warcraft III or World Editor if they have the archive open, and make sure that its " +
	"folder can be written, then retry."

func errNotRemoved(archive string, cause error) error {
	return &diag.Error{
		Msg:   "Removing " + archive + " failed: " + fsx.Reason(cause),
		File:  archive,
		Hint:  archiveHint,
		Cause: cause,
	}
}

func errArchiveNotWritten(archive string, cause error) error {
	return &diag.Error{
		Msg:   "Writing " + archive + " failed: " + fsx.Reason(cause),
		File:  archive,
		Hint:  archiveHint,
		Cause: cause,
	}
}
