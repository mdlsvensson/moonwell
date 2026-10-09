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

func prepareArchivePath(project *manifest.Project) (outputFile, error) {
	output, err := archivePath(project)
	if err != nil {
		return outputFile{}, err
	}
	if err := removeArchive(output); err != nil {
		return outputFile{}, err
	}
	return output, nil
}

func prepareTestArchivePath(project *manifest.Project) (outputFile, error) {
	mapDir, err := sourceMapDir(project)
	if err != nil {
		return outputFile{}, err
	}
	output, err := newOutputFile(project.Root, testDir+"/"+mapDir)
	if err != nil {
		return outputFile{}, err
	}
	if fsx.IsDir(output.fullPath) {
		return outputFile{}, errTestArchiveIsAFolder(output.displayPath)
	}
	if err := removeArchive(output); err != nil {
		return outputFile{}, err
	}
	return output, nil
}

func packArchive(e *env.Env, plan *Result, output outputFile) error {
	e.Log.Info("Packing archive...")
	archive, err := packMap(plan.Map, strings.TrimSuffix(path.Base(output.displayPath), mapSuffix))
	if err != nil {
		return err
	}
	return writeArchive(output, archive)
}

func logBuilt(log *env.Logger, plan *Result, output outputFile) {
	log.Info("Built " + output.displayPath + " (" + strconv.Itoa(len(plan.Program.Modules)) + " module(s)).")
}

func archivePath(project *manifest.Project) (outputFile, error) {
	mapDir, err := sourceMapDir(project)
	if err != nil {
		return outputFile{}, err
	}
	buildDir, err := resolveBuildDir(project, mapDir)
	if err != nil {
		return outputFile{}, err
	}
	output, err := newOutputFile(project.Root, buildDir+"/"+mapDir)
	if err != nil {
		return outputFile{}, err
	}
	if fsx.IsDir(output.fullPath) {
		return outputFile{}, errOutputIsAFolder(project.ManifestName, output.displayPath)
	}
	return output, nil
}

func resolveBuildDir(project *manifest.Project, mapDir string) (string, error) {
	value := project.Build.Folder
	parts, fault := parseDir(value)
	switch fault {
	case dirEscapes:
		normalized := strings.TrimRight(strings.ReplaceAll(value, `\`, "/"), "/")
		return "", errOutputOutside(project.ManifestName, normalized+"/"+mapDir)
	case dirEmpty:
		return "", errNoBuildFolder(project.ManifestName, value)
	case dirNotPortable:
		return "", errUnusableBuildFolder(project.ManifestName, value)
	}
	buildDir := strings.Join(parts, "/")
	if reserved, isReserved := reservedDir(parts); isReserved {
		return "", errOutputInReservedDir(project.ManifestName, buildDir+"/"+mapDir, reserved)
	}
	return buildDir, nil
}

func reservedDir(parts []string) (reserved string, found bool) {
	first := toLowerASCII(parts[0])
	if first == mapsDir || first == sourcesDir {
		return first, true
	}
	if len(parts) == 1 {
		return "", false
	}
	switch firstTwo := first + "/" + toLowerASCII(parts[1]); firstTwo {
	case stageDir, testDir:
		return firstTwo, true
	}
	return "", false
}

func toLowerASCII(text string) string {
	lower := []byte(text)
	for index, char := range lower {
		if char >= 'A' && char <= 'Z' {
			lower[index] = char + 'a' - 'A'
		}
	}
	return string(lower)
}

func removeArchive(output outputFile) error {
	err := fsx.RemoveFile(output.fullPath)
	if err == nil {
		return nil
	}
	var diagErr *diag.Error
	if errors.As(err, &diagErr) && diagErr.Cause != nil {
		err = diagErr.Cause
	}
	return errNotRemoved(output.displayPath, err)
}

const tempSuffix = ".tmp"

func writeArchive(output outputFile, archive []byte) error {
	if err := os.MkdirAll(filepath.Dir(output.fullPath), 0o777); err != nil {
		return errArchiveNotWritten(output.displayPath, err)
	}
	tempPath := output.fullPath + tempSuffix
	err := writeNewFile(tempPath, archive)
	if err == nil {
		err = os.Rename(tempPath, output.fullPath)
	}
	if err != nil {
		_ = os.Remove(tempPath)
		return errArchiveNotWritten(output.displayPath, err)
	}
	return nil
}

func writeNewFile(fullPath string, data []byte) error {
	if err := os.Remove(fullPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	created, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		return err
	}
	_, err = created.Write(data)
	if closeErr := created.Close(); err == nil {
		err = closeErr
	}
	return err
}

const (
	insideHint     = "Set build.folder to a folder inside the project, such as dist/bin."
	outputOnlyHint = "Set build.folder to a folder that only holds build output, such as dist/bin."
)

func errOutputOutside(manifestName, output string) error {
	return &diag.Error{
		Msg:  "The build output " + output + " is outside the project.",
		File: manifestName,
		Hint: insideHint,
	}
}

func errNoBuildFolder(manifestName, value string) error {
	return &diag.Error{Msg: `build.folder names no folder: "` + value + `".`, File: manifestName, Hint: insideHint}
}

func errUnusableBuildFolder(manifestName, value string) error {
	return &diag.Error{
		Msg:  `build.folder has a name that Windows cannot hold: "` + value + `".`,
		File: manifestName,
		Hint: insideHint + " " + unusableNames,
	}
}

func errOutputInReservedDir(manifestName, output, reserved string) error {
	return &diag.Error{
		Msg:  "The build output " + output + " is in " + reserved + "/, a folder Moonwell reads from or writes for itself.",
		File: manifestName,
		Hint: outputOnlyHint,
	}
}

func errTestArchiveIsAFolder(displayPath string) error {
	return &diag.Error{
		Msg:  displayPath + " is a directory; refusing to replace it with the archive that test.archive asks for.",
		File: displayPath,
		Hint: "Remove or rename that directory, then retry.",
	}
}

func errOutputIsAFolder(manifestName, output string) error {
	return &diag.Error{
		Msg:  "The build output " + output + " is a directory; refusing to replace it.",
		File: manifestName,
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
