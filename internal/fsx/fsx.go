// Package fsx holds the file helpers the rest of Moonwell shares: listing, copying, removing, reading and writing
// files, checking that a path stays inside its folder, and a journal of writes that can be undone.
//
// It takes paths, text and bytes, and returns paths, text, bytes and errors. A failure a user can act on (a file
// another program holds open, a source file that cannot be read, a path that is not safe to join, a link where real
// files are needed) is a *diag.Error with a hint; every other failure is the operating system's error as it came,
// for the caller to word with Reason. Inside is the one door that words the system's failure itself, by the path
// it was given.
//
// The package must not know what the files are for: it reads no manifest and no map format, decides no layout of a
// project, prints nothing, and imports no package of Moonwell but diag.
package fsx

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"

	"github.com/mdlsvensson/moonwell/internal/diag"
)

const onWindows = runtime.GOOS == "windows"

// ToPosix writes path with "/" separators.
func ToPosix(path string) string { return filepath.ToSlash(path) }

// IsWithin reports whether path is folder or inside it. Letter case is ignored on Windows, whose file systems
// usually ignore it too.
func IsWithin(path, folder string) bool {
	path, errPath := filepath.Abs(path)
	folder, errFolder := filepath.Abs(folder)
	if errPath != nil || errFolder != nil {
		return false
	}
	if onWindows {
		path, folder = strings.ToLower(path), strings.ToLower(folder)
	}
	between, err := filepath.Rel(folder, path)
	return err == nil && staysInside(between)
}

// staysInside reports whether between, the way from a folder to a path, never leaves the folder. A name that only
// starts with two dots ("..map") stays inside.
func staysInside(between string) bool {
	leaves := between == ".." || strings.HasPrefix(between, ".."+string(filepath.Separator))
	return !filepath.IsAbs(between) && !leaves
}

// ListFiles returns every file below dir as a path relative to dir with "/" separators, sorted by bytes.
func ListFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, ToPosix(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(files)
	return files, nil
}

// Exists reports whether path exists.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// IsDir reports whether path is an existing folder.
func IsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// Lstat is os.Lstat, with nil and no error for a path that does not exist.
func Lstat(path string) (fs.FileInfo, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return info, err
}

// IsLink reports whether info is a symlink or, on Windows, a junction, which Go reports as an irregular file.
func IsLink(info fs.FileInfo) bool {
	return info.Mode()&fs.ModeSymlink != 0 || (onWindows && info.Mode()&fs.ModeIrregular != 0)
}

// RemoveAll removes path and everything below it. A missing path is not an error.
func RemoveAll(path string) error {
	return inUse(os.RemoveAll(path), path)
}

// RemoveFile removes a file. It is never recursive, so a folder with contents fails instead of being deleted. A
// missing path is not an error.
func RemoveFile(path string) error {
	err := os.Remove(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return inUse(err, path)
}

// sharingViolation is the Windows error for a file another process has open. The same number means something else
// on other systems.
const sharingViolation = syscall.Errno(32)

// inUse returns the failure a user reads when err says another program holds path open, and err itself otherwise.
func inUse(err error, path string) error {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return err
	}
	if errno == syscall.EBUSY || (onWindows && errno == sharingViolation) {
		return errInUse(path, err)
	}
	return err
}

// Reason is the operating system's reason for a failure, without the operation and the path Go puts before it.
func Reason(err error) string {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err.Error()
	}
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		return linkErr.Err.Error()
	}
	var execErr *exec.Error
	if errors.As(err, &execErr) {
		return execErr.Err.Error()
	}
	return err.Error()
}

// ReplaceDir replaces destination with a copy of source, creating the folder destination is in.
func ReplaceDir(source, destination string) error {
	if err := RemoveAll(destination); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o777); err != nil {
		return err
	}
	return CopyTree(source, destination)
}

// CopyTree copies the folder source to destination, which must not exist. Symlinks are copied as links.
func CopyTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		return copyEntry(path, filepath.Join(destination, rel), entry)
	})
}

// copyEntry makes target what entry is at path: a folder, a link to the same place, or a file with the same bytes.
func copyEntry(path, target string, entry fs.DirEntry) error {
	switch {
	case entry.IsDir():
		return os.MkdirAll(target, 0o777)
	case entry.Type()&fs.ModeSymlink != 0:
		link, err := os.Readlink(path)
		if err != nil {
			return err
		}
		return os.Symlink(link, target)
	}
	return CopyFile(path, target)
}

// CopyFile copies the file source to destination with its permissions.
func CopyFile(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// CopyProgram copies the executable source to destination, creating its folder, unless destination already holds the
// same bytes. It reports whether it copied.
func CopyProgram(source, destination string) (copied bool, err error) {
	program, err := os.ReadFile(source)
	if err != nil {
		return false, err
	}
	copied, err = writeIfChanged(destination, program, 0o777)
	if err != nil || !copied || onWindows {
		return copied, err
	}
	// A file that was already there keeps the mode it had, so the mode is set after the write.
	if err := os.Chmod(destination, 0o755); err != nil {
		return false, err
	}
	return true, nil
}

// WriteIfChanged writes content to path, creating its folder, unless the file already holds exactly that content.
// It reports whether it wrote.
func WriteIfChanged(path, content string) (wrote bool, err error) {
	return writeIfChanged(path, []byte(content), 0o666)
}

// writeIfChanged writes content to path with mode, creating its folder, unless the file already holds content.
func writeIfChanged(path string, content []byte, mode fs.FileMode) (wrote bool, err error) {
	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, content) {
		return false, nil
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		return false, err
	}
	if err := os.WriteFile(path, content, mode); err != nil {
		return false, err
	}
	return true, nil
}

// ReadSource reads a source file as standalone Lua's loadfile sees it, which is as bytes: a leading byte order
// mark is dropped and a first line that starts with `#` (a shebang) is blanked, and every other byte is returned
// as the file has it. Nothing is decoded, so the string may hold bytes that are not UTF-8. label is the file's
// path relative to the project with "/" separators, for the error.
func ReadSource(path, label string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", errUnreadable(label, err)
	}
	return blankShebang(strings.TrimPrefix(string(data), byteOrderMark)), nil
}

// blankShebang empties a first line that starts with `#` and keeps its line break, so line numbers stay.
func blankShebang(source string) string {
	if !strings.HasPrefix(source, "#") {
		return source
	}
	end := strings.IndexAny(source, "\r\n")
	if end < 0 {
		return ""
	}
	return source[end:]
}

// SHA256Hex is the SHA-256 of b in lower-case hexadecimal.
func SHA256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// Resolve returns path when it is absolute, and otherwise path under base: how a path written in a manifest is
// read against the folder it is relative to.
func Resolve(base, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(base, path)
}

// ---- errors ----

func errInUse(path string, cause error) error {
	return &diag.Error{
		Msg:   path + " is in use by another program.",
		Hint:  "Close Warcraft III or World Editor if it has this map open, then try again.",
		Cause: cause,
	}
}

func errUnreadable(label string, cause error) error {
	return &diag.Error{
		Msg:   "Reading " + label + " failed: " + Reason(cause),
		File:  label,
		Hint:  "Close any program that has the file open and check that it is a readable file, then try again.",
		Cause: cause,
	}
}
