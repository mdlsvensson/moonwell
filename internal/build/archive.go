package build

import (
	"errors"
	"io/fs"
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

const mapSuffix = ".w3x"

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

func lowerASCII(text string) string {
	lowered := []byte(text)
	for at, char := range lowered {
		if char >= 'A' && char <= 'Z' {
			lowered[at] = char + 'a' - 'A'
		}
	}
	return string(lowered)
}

func removeArchive(at place) error {
	err := fsx.RemoveFile(at.file)
	if err == nil {
		return nil
	}
	var held *diag.Error
	if errors.As(err, &held) && held.Cause != nil {
		err = held.Cause
	}
	return errNotRemoved(at.label, err)
}

const unfinished = ".tmp"

func writeArchive(at place, archive []byte) error {
	if err := os.MkdirAll(filepath.Dir(at.file), 0o777); err != nil {
		return errArchiveNotWritten(at.label, err)
	}
	beside := at.file + unfinished
	err := writeAnew(beside, archive)
	if err == nil {
		err = os.Rename(beside, at.file)
	}
	if err != nil {
		_ = os.Remove(beside)
		return errArchiveNotWritten(at.label, err)
	}
	return nil
}

func writeAnew(file string, data []byte) error {
	if err := os.Remove(file); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	made, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		return err
	}
	_, err = made.Write(data)
	if closed := made.Close(); err == nil {
		err = closed
	}
	return err
}

const (
	insideHint     = "Set build.folder to a folder inside the project, such as dist/bin."
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
