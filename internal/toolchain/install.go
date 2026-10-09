package toolchain

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

type installer struct {
	ctx     context.Context
	e       *env.Env
	tool    Tool
	version string
	asset   Asset
	target  string
}

func (inst installer) programPath(dir string) string {
	return filepath.Join(dir, filepath.FromSlash(inst.asset.Binary))
}

func (inst installer) download() ([]byte, error) {
	inst.e.Log.Info("Downloading " + inst.tool.Title + " " + inst.version + "...")
	status, body, err := inst.e.Fetch(inst.ctx, inst.asset.URL)
	switch {
	case err != nil && inst.ctx.Err() != nil:
		return nil, inst.ctx.Err()
	case err != nil:
		return nil, errDownloadFailed(inst.tool, inst.asset.URL, err)
	case status < 200 || status > 299:
		return nil, errDownloadStatus(inst.tool, inst.asset.URL, status)
	}
	return body, nil
}

func (inst installer) verify(download []byte) error {
	if actual := fsx.SHA256Hex(download); actual != inst.asset.SHA256 {
		return errChecksum(inst.tool, inst.asset.SHA256, actual)
	}
	return nil
}

func (inst installer) makeStagingDir() (string, error) {
	beside := filepath.Dir(inst.target)
	if err := os.MkdirAll(beside, 0o777); err != nil {
		return "", errNotInstalled(inst.tool, beside, err)
	}
	staging, err := os.MkdirTemp(beside, ".install-")
	if err != nil {
		return "", errNotInstalled(inst.tool, beside, err)
	}
	return staging, nil
}

func (inst installer) removeStagingDir(staging string) {
	if err := fsx.RemoveAll(staging); err != nil {
		reason := strings.TrimSuffix(fsx.Reason(err), ".")
		inst.e.Log.Warn("Moonwell could not remove its staging folder " + staging + " (" + reason +
			"). Nothing in it is used; remove the folder yourself.")
	}
}

func (inst installer) unpack(download []byte, staging string) error {
	staged := inst.programPath(staging)
	if err := os.MkdirAll(filepath.Dir(staged), 0o777); err != nil {
		return errNotInstalled(inst.tool, filepath.Dir(staged), err)
	}
	if err := inst.unpackArchive(download, staging, staged); err != nil {
		return err
	}
	if !fsx.Exists(staged) {
		return errProgramMissing(inst.tool, inst.asset.Binary)
	}
	if runtime.GOOS == "windows" {
		return nil
	}
	if err := os.Chmod(staged, 0o755); err != nil {
		return errNotInstalled(inst.tool, staged, err)
	}
	return nil
}

func (inst installer) unpackArchive(download []byte, staging, staged string) error {
	switch inst.asset.Archive {
	case "":
		return inst.writeFile(staged, download)
	case "zip":
		program, err := readZipEntry(inst.tool, download, inst.asset.Binary)
		if err != nil {
			return err
		}
		return inst.writeFile(staged, program)
	case "7z":
		return inst.extractWithTar(download, staging)
	}
	return errUnknownArchiveKind(inst.asset.Archive)
}

func (inst installer) writeFile(file string, data []byte) error {
	if err := os.WriteFile(file, data, 0o777); err != nil {
		return errNotInstalled(inst.tool, file, err)
	}
	return nil
}

func readZipEntry(tool Tool, archive []byte, name string) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil && reader == nil {
		return nil, errInvalidZip(strings.TrimPrefix(err.Error(), "zip: "))
	}
	for _, entry := range reader.File {
		if entry.Name == name {
			return readZipFile(entry)
		}
	}
	return nil, errProgramMissing(tool, name)
}

func readZipFile(entry *zip.File) ([]byte, error) {
	file, err := entry.Open()
	if err != nil {
		return nil, errInvalidZip(entry.Name + " cannot be read: " + err.Error())
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, errInvalidZip(entry.Name + " cannot be read: " + err.Error())
	}
	return data, nil
}

func (inst installer) extractWithTar(download []byte, staging string) error {
	archive := filepath.Join(staging, "archive.7z")
	if err := inst.writeFile(archive, download); err != nil {
		return err
	}
	options := env.RunOptions{Hint: capitalize(inst.tool.ManualInstallHint)}
	result, err := inst.e.Run(inst.ctx, windowsTarPath(), []string{"-xf", archive, "-C", staging}, options)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return errNotExtracted(inst.tool, result.Stderr)
	}
	if err := os.Remove(archive); err != nil {
		return errNotInstalled(inst.tool, archive, err)
	}
	return nil
}

func windowsTarPath() string {
	systemRoot := os.Getenv("SystemRoot")
	if systemRoot == "" {
		systemRoot = `C:\Windows`
	}
	return filepath.Join(systemRoot, "System32", "tar.exe")
}

func (inst installer) verifyVersion(staging string) error {
	output, err := inst.tool.runVersionCommand(inst.ctx, inst.e, inst.programPath(staging), capitalize(inst.tool.ManualInstallHint))
	if err != nil {
		return err
	}
	if found := inst.tool.parseVersion(output); found != inst.version {
		return errVersionMismatch(inst.tool, found, inst.version)
	}
	return nil
}

func (inst installer) moveIntoPlace(staging string) (string, error) {
	program := inst.programPath(inst.target)
	err := os.Rename(staging, inst.target)
	switch {
	case err == nil:
		return program, nil
	case fsx.Exists(program):
		return program, nil
	case fsx.Exists(inst.target):
		return "", errTargetBlocked(inst.tool, inst.target, err)
	}
	return "", errNotInstalled(inst.tool, inst.target, err)
}

func errDownloadFailed(tool Tool, url string, cause error) error {
	return &diag.Error{
		Msg:   "Downloading " + url + " failed.",
		Hint:  "Check your connection and retry, or " + tool.ManualInstallHint,
		Cause: cause,
	}
}

func errDownloadStatus(tool Tool, url string, status int) error {
	return &diag.Error{
		Msg:  "Downloading " + url + " failed with HTTP " + strconv.Itoa(status) + ".",
		Hint: "Retry later, or " + tool.ManualInstallHint,
	}
}

func errChecksum(tool Tool, expected, actual string) error {
	return &diag.Error{
		Msg: tool.Title + " download checksum mismatch (expected " + expected + ", got " + actual + ").",
		Hint: "Retry the download, and do not bypass the check. If it keeps failing, report it, or " +
			tool.ManualInstallHint,
	}
}

func errInvalidZip(reason string) error {
	return &diag.Error{Msg: "Invalid zip archive: " + reason + "."}
}

func errProgramMissing(tool Tool, binary string) error {
	return &diag.Error{Msg: "The " + tool.Title + " archive has no " + binary + "."}
}

func errNotExtracted(tool Tool, stderr string) error {
	return &diag.Error{Msg: "Extracting " + tool.Title + " failed:\n" + fsx.TrimASCIISpace(stderr)}
}

func errVersionMismatch(tool Tool, found, version string) error {
	return &diag.Error{
		Msg: "Downloaded " + tool.Title + " reports version " + versionOrUnknown(found) + ", expected " + version + ".",
	}
}

func errNotInstalled(tool Tool, path string, cause error) error {
	return &diag.Error{
		Msg:   "Installing " + tool.Title + " failed: " + fsx.Reason(cause),
		File:  path,
		Hint:  "Make sure Moonwell can write to its cache folder and the disk is not full, then retry.",
		Cause: cause,
	}
}

func errTargetBlocked(tool Tool, target string, cause error) error {
	return &diag.Error{
		Msg:   "Installing " + tool.Title + " failed: " + fsx.Reason(cause),
		File:  target,
		Hint:  "Remove " + target + " and retry: the folder is in the way and holds no " + tool.Title + ".",
		Cause: cause,
	}
}

func errUnknownArchiveKind(kind string) error {
	return errors.New("Cannot unpack a download of the kind " + strconv.Quote(kind) + ".")
}
