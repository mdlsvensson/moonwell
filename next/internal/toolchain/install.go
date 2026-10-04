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

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
)

// install is one download of a tool on its way into the cache: the steps of Ensure.
type install struct {
	ctx     context.Context
	e       *env.Env
	tool    Tool
	version string
	asset   Asset
	target  string // the folder in the cache that holds this version of the tool
}

// programIn is the program's file in a folder that holds the tool: the target, or a staging folder.
func (in install) programIn(folder string) string {
	return filepath.Join(folder, filepath.FromSlash(in.asset.Binary))
}

// download fetches the asset. A context that was cancelled is passed on as it is.
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

// verify refuses a download whose SHA-256 is not the pinned one. It comes before anything is written or run.
func (in install) verify(download []byte) error {
	if actual := fsx.SHA256Hex(download); actual != in.asset.SHA256 {
		return errChecksum(in.tool, in.asset.SHA256, actual)
	}
	return nil
}

// stage makes a staging folder beside the target, so that moving it into place is one rename on one disk.
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

// discard removes the staging folder; after a move into place there is none. One that cannot be removed (a
// program that still has a file of it open) is named in a warning: it is no place a program is taken from, and
// the failure of the install, when there is one, is what the command reports.
func (in install) discard(staging string) {
	if err := fsx.RemoveAll(staging); err != nil {
		// Some of the reasons end with a full stop and some do not.
		reason := strings.TrimSuffix(fsx.Reason(err), ".")
		in.e.Log.Warn("Moonwell could not remove its staging folder " + staging + " (" + reason +
			"). Nothing in it is used; remove the folder yourself.")
	}
}

// unpack puts the program of the download into staging, where every user may run it.
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
	// Windows keeps no permission to run a file; every other system needs it set.
	if runtime.GOOS == "windows" {
		return nil
	}
	if err := os.Chmod(staged, 0o755); err != nil {
		return errNotInstalled(in.tool, staged, err)
	}
	return nil
}

// unpackAs unpacks the download by the kind of its archive.
func (in install) unpackAs(download []byte, staging, staged string) error {
	switch in.asset.Archive {
	case "":
		return in.write(staged, download)
	case "zip":
		// Only the program is taken out, so no name in the archive can write outside the staging folder.
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

// write writes a file of the staging folder.
func (in install) write(file string, data []byte) error {
	if err := os.WriteFile(file, data, 0o777); err != nil {
		return errNotInstalled(in.tool, file, err)
	}
	return nil
}

// zipEntry reads one file of a zip archive.
func zipEntry(tool Tool, archive []byte, name string) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	// An archive with a name that is not safe to unpack is reported with its reader: it is read all the same,
	// since no entry is written under its own name.
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

// readEntry is what an entry of a zip archive holds.
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

// untar unpacks a 7z archive into staging. Go's library reads no 7z; the tar.exe that Windows ships does. Only
// Windows has that program, and only the downloads for Windows are 7z archives: on another system this step
// fails as a program that cannot be started.
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
	if result.Code != 0 {
		return errNotExtracted(in.tool, result.Stderr)
	}
	if err := os.Remove(archive); err != nil {
		return errNotInstalled(in.tool, archive, err)
	}
	return nil
}

// windowsTar is the tar.exe of Windows itself, by its full path: a tar on PATH may be another program.
func windowsTar() string {
	systemRoot := os.Getenv("SystemRoot")
	if systemRoot == "" {
		systemRoot = `C:\Windows`
	}
	return filepath.Join(systemRoot, "System32", "tar.exe")
}

// askVersion runs the staged program and refuses one that does not report the version asked for.
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

// moveIntoPlace renames the staging folder to the target and returns the program there.
func (in install) moveIntoPlace(staging string) (string, error) {
	program := in.programIn(in.target)
	// A failed rename with the program in place means another process finished the same install first: theirs is
	// kept, and passed the same checks.
	if err := os.Rename(staging, in.target); err != nil && !fsx.Exists(program) {
		return "", errInTheWay(in.tool, in.target, err)
	}
	return program, nil
}

// ---- errors ----

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
	return &diag.Error{Msg: "Extracting " + tool.Title + " failed:\n" + trimmed(stderr)}
}

func errOtherVersion(tool Tool, found, version string) error {
	return &diag.Error{
		Msg: "Downloaded " + tool.Title + " reports version " + orUnknown(found) + ", expected " + version + ".",
	}
}

// errNotInstalled is a file or a folder of the cache that could not be written.
func errNotInstalled(tool Tool, path string, cause error) error {
	return &diag.Error{
		Msg:   "Installing " + tool.Title + " failed: " + fsx.Reason(cause),
		File:  path,
		Hint:  "Make sure Moonwell can write to its cache folder and the disk is not full, then retry.",
		Cause: cause,
	}
}

// errInTheWay is a target that could not be made and holds no program: something else is in its place.
func errInTheWay(tool Tool, target string, cause error) error {
	return &diag.Error{
		Msg:   "Installing " + tool.Title + " failed: " + fsx.Reason(cause),
		File:  target,
		Hint:  "Remove " + target + " and retry: the folder is in the way and holds no " + tool.Title + ".",
		Cause: cause,
	}
}

// errArchiveKind is not a diag error: the kinds of archive are in Moonwell's own list of downloads, so one that
// it cannot unpack is its own bug.
func errArchiveKind(kind string) error {
	return errors.New("Cannot unpack a download of the kind " + strconv.Quote(kind) + ".")
}
