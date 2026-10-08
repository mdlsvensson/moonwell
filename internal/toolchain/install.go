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

type install struct {
	ctx     context.Context
	e       *env.Env
	tool    Tool
	version string
	asset   Asset
	target  string
}

func (in install) programIn(folder string) string {
	return filepath.Join(folder, filepath.FromSlash(in.asset.Binary))
}

func (in install) download() ([]byte, error) {
	in.e.Log.Info("Downloading " + in.tool.Title + " " + in.version + "...")
	status, body, err := in.e.Fetch(in.ctx, in.asset.URL)
	switch {
	case err != nil && in.ctx.Err() != nil:
		return nil, in.ctx.Err()
	case err != nil:
		return nil, errDownloadFailed(in.tool, in.asset.URL, err)
	case status < 200 || status > 299:
		return nil, errDownloadStatus(in.tool, in.asset.URL, status)
	}
	return body, nil
}

func (in install) verify(download []byte) error {
	if actual := fsx.SHA256Hex(download); actual != in.asset.SHA256 {
		return errChecksum(in.tool, in.asset.SHA256, actual)
	}
	return nil
}

func (in install) stage() (string, error) {
	beside := filepath.Dir(in.target)
	if err := os.MkdirAll(beside, 0o777); err != nil {
		return "", errNotInstalled(in.tool, beside, err)
	}
	staging, err := os.MkdirTemp(beside, ".install-")
	if err != nil {
		return "", errNotInstalled(in.tool, beside, err)
	}
	return staging, nil
}

func (in install) discard(staging string) {
	if err := fsx.RemoveAll(staging); err != nil {
		reason := strings.TrimSuffix(fsx.Reason(err), ".")
		in.e.Log.Warn("Moonwell could not remove its staging folder " + staging + " (" + reason +
			"). Nothing in it is used; remove the folder yourself.")
	}
}

func (in install) unpack(download []byte, staging string) error {
	staged := in.programIn(staging)
	if err := os.MkdirAll(filepath.Dir(staged), 0o777); err != nil {
		return errNotInstalled(in.tool, filepath.Dir(staged), err)
	}
	if err := in.unpackAs(download, staging, staged); err != nil {
		return err
	}
	if !fsx.Exists(staged) {
		return errNoProgram(in.tool, in.asset.Binary)
	}
	if runtime.GOOS == "windows" {
		return nil
	}
	if err := os.Chmod(staged, 0o755); err != nil {
		return errNotInstalled(in.tool, staged, err)
	}
	return nil
}

func (in install) unpackAs(download []byte, staging, staged string) error {
	switch in.asset.Archive {
	case "":
		return in.write(staged, download)
	case "zip":
		program, err := zipEntry(in.tool, download, in.asset.Binary)
		if err != nil {
			return err
		}
		return in.write(staged, program)
	case "7z":
		return in.untar(download, staging)
	}
	return errArchiveKind(in.asset.Archive)
}

func (in install) write(file string, data []byte) error {
	if err := os.WriteFile(file, data, 0o777); err != nil {
		return errNotInstalled(in.tool, file, err)
	}
	return nil
}

func zipEntry(tool Tool, archive []byte, name string) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil && reader == nil {
		return nil, errInvalidZip(strings.TrimPrefix(err.Error(), "zip: "))
	}
	for _, entry := range reader.File {
		if entry.Name == name {
			return readEntry(entry)
		}
	}
	return nil, errNoProgram(tool, name)
}

func readEntry(entry *zip.File) ([]byte, error) {
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

func (in install) untar(download []byte, staging string) error {
	archive := filepath.Join(staging, "archive.7z")
	if err := in.write(archive, download); err != nil {
		return err
	}
	options := env.RunOptions{Hint: sentence(in.tool.Otherwise)}
	result, err := in.e.Run(in.ctx, windowsTar(), []string{"-xf", archive, "-C", staging}, options)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return errNotExtracted(in.tool, result.Stderr)
	}
	if err := os.Remove(archive); err != nil {
		return errNotInstalled(in.tool, archive, err)
	}
	return nil
}

func windowsTar() string {
	systemRoot := os.Getenv("SystemRoot")
	if systemRoot == "" {
		systemRoot = `C:\Windows`
	}
	return filepath.Join(systemRoot, "System32", "tar.exe")
}

func (in install) askVersion(staging string) error {
	printed, err := in.tool.ask(in.ctx, in.e, in.programIn(staging), sentence(in.tool.Otherwise))
	if err != nil {
		return err
	}
	if found := in.tool.versionIn(printed); found != in.version {
		return errOtherVersion(in.tool, found, in.version)
	}
	return nil
}

func (in install) moveIntoPlace(staging string) (string, error) {
	program := in.programIn(in.target)
	err := os.Rename(staging, in.target)
	switch {
	case err == nil:
		return program, nil
	case fsx.Exists(program):
		return program, nil
	case fsx.Exists(in.target):
		return "", errInTheWay(in.tool, in.target, err)
	}
	return "", errNotInstalled(in.tool, in.target, err)
}

func errDownloadFailed(tool Tool, url string, cause error) error {
	return &diag.Error{
		Msg:   "Downloading " + url + " failed.",
		Hint:  "Check your connection and retry, or " + tool.Otherwise,
		Cause: cause,
	}
}

func errDownloadStatus(tool Tool, url string, status int) error {
	return &diag.Error{
		Msg:  "Downloading " + url + " failed with HTTP " + strconv.Itoa(status) + ".",
		Hint: "Retry later, or " + tool.Otherwise,
	}
}

func errChecksum(tool Tool, expected, actual string) error {
	return &diag.Error{
		Msg: tool.Title + " download checksum mismatch (expected " + expected + ", got " + actual + ").",
		Hint: "Retry the download, and do not bypass the check. If it keeps failing, report it, or " +
			tool.Otherwise,
	}
}

func errInvalidZip(reason string) error {
	return &diag.Error{Msg: "Invalid zip archive: " + reason + "."}
}

func errNoProgram(tool Tool, binary string) error {
	return &diag.Error{Msg: "The " + tool.Title + " archive has no " + binary + "."}
}

func errNotExtracted(tool Tool, stderr string) error {
	return &diag.Error{Msg: "Extracting " + tool.Title + " failed:\n" + fsx.TrimASCIISpace(stderr)}
}

func errOtherVersion(tool Tool, found, version string) error {
	return &diag.Error{
		Msg: "Downloaded " + tool.Title + " reports version " + orUnknown(found) + ", expected " + version + ".",
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

func errInTheWay(tool Tool, target string, cause error) error {
	return &diag.Error{
		Msg:   "Installing " + tool.Title + " failed: " + fsx.Reason(cause),
		File:  target,
		Hint:  "Remove " + target + " and retry: the folder is in the way and holds no " + tool.Title + ".",
		Cause: cause,
	}
}

func errArchiveKind(kind string) error {
	return errors.New("Cannot unpack a download of the kind " + strconv.Quote(kind) + ".")
}
