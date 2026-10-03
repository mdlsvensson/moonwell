// Package fsx holds the file helpers the rest of Moonwell shares.
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
	"strings"
	"syscall"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/text"
)

// ToPosix writes path with "/" separators.
func ToPosix(path string) string { return filepath.ToSlash(path) }

// IsWithin reports whether path is folder or inside it; ignoring case on Windows, whose file systems usually do.
func IsWithin(path, folder string) bool {
	path, errPath := filepath.Abs(path)
	folder, errFolder := filepath.Abs(folder)
	if errPath != nil || errFolder != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		path, folder = text.Lower(path), text.Lower(folder)
	}
	between, err := filepath.Rel(folder, path)
	if err != nil {
		return false
	}
	return between == "." ||
		(!filepath.IsAbs(between) && between != ".." && !strings.HasPrefix(between, ".."+string(filepath.Separator)))
}

// ListFiles returns all files below dir, as sorted POSIX paths relative to dir.
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
	text.Sort(files)
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

// IsLink reports whether info is a symlink or, on Windows, a junction.
func IsLink(info fs.FileInfo) bool {
	return info.Mode()&os.ModeSymlink != 0 || (runtime.GOOS == "windows" && info.Mode()&os.ModeIrregular != 0)
}

// RemoveAll removes path and everything below it. A missing path is not an error.
func RemoveAll(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return inUse(err, path)
	}
	return nil
}

// RemoveFile removes a file. It is never recursive, so a folder with contents fails instead of being deleted. A
// missing path is not an error.
func RemoveFile(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return inUse(err, path)
	}
	return nil
}

const errSharingViolation = syscall.Errno(32) // Windows: another process has the file open

// inUse returns a clear error when another program holds path open, else err unchanged.
func inUse(err error, path string) error {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return err
	}
	if errno != syscall.EBUSY && !(runtime.GOOS == "windows" && errno == errSharingViolation) {
		return err
	}
	return &diag.Error{
		Msg:   path + " is in use by another program.",
		Hint:  "Close Warcraft III or World Editor if it has this map open, then try again.",
		Cause: err,
	}
}

// Reason is the operating system's reason for a failure, without the operation and path Go puts before it.
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

// ReplaceDir replaces destination with a copy of source.
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
		target := filepath.Join(destination, rel)
		switch {
		case entry.IsDir():
			return os.MkdirAll(target, 0o777)
		case entry.Type()&os.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		default:
			return CopyFile(path, target)
		}
	})
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
func CopyProgram(source, destination string) (bool, error) {
	wanted, err := os.ReadFile(source)
	if err != nil {
		return false, err
	}
	existing, err := os.ReadFile(destination)
	if err == nil && bytes.Equal(existing, wanted) {
		return false, nil
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o777); err != nil {
		return false, err
	}
	if err := os.WriteFile(destination, wanted, 0o777); err != nil {
		return false, err
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(destination, 0o755); err != nil {
			return false, err
		}
	}
	return true, nil
}

// WriteIfChanged writes content unless the file already has exactly that content. It reports whether it wrote.
func WriteIfChanged(path, content string) (bool, error) {
	existing, err := os.ReadFile(path)
	if err == nil && string(existing) == content {
		return false, nil
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, []byte(content), 0o666)
}

// ReadSource reads a source file as standalone Lua's loadfile sees it: without a leading byte order mark, and with a
// first line that starts with `#` (a shebang) blanked, keeping its line break so line numbers stay. label is the
// file's POSIX path relative to the project, for error messages.
func ReadSource(path, label string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", &diag.Error{
			Msg:   "Reading " + label + " failed: " + Reason(err),
			File:  label,
			Cause: err,
			Hint:  "Close any program that has the file open and check that it is a readable file, then try again.",
		}
	}
	source := strings.TrimPrefix(text.Lossy(data), "\xEF\xBB\xBF")
	if strings.HasPrefix(source, "#") {
		end := strings.IndexAny(source, "\r\n")
		if end < 0 {
			end = len(source)
		}
		source = source[end:]
	}
	return source, nil
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
