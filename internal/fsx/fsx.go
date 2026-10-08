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

func ToSlash(path string) string { return filepath.ToSlash(path) }

func IsWithin(path, dir string) bool {
	path, errPath := filepath.Abs(path)
	dir, errDir := filepath.Abs(dir)
	if errPath != nil || errDir != nil {
		return false
	}
	if onWindows {
		path, dir = strings.ToLower(path), strings.ToLower(dir)
	}
	rel, err := filepath.Rel(dir, path)
	return err == nil && isInsideRel(rel)
}

func isInsideRel(rel string) bool {
	escapes := rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
	return !filepath.IsAbs(rel) && !escapes
}

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
		files = append(files, ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(files)
	return files, nil
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func IsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func Lstat(path string) (fs.FileInfo, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return info, err
}

func ReadFileIfExists(path string) (data []byte, found bool, err error) {
	data, err = os.ReadFile(path)
	switch {
	case err == nil:
		return data, true, nil
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, syscall.ENOTDIR):
		return nil, false, nil
	}
	return nil, false, err
}

func IsSymlink(info fs.FileInfo) bool {
	return info.Mode()&fs.ModeSymlink != 0 || (onWindows && info.Mode()&fs.ModeIrregular != 0)
}

func RemoveAll(path string) error {
	return wrapIfInUse(os.RemoveAll(path), path)
}

func RemoveFile(path string) error {
	err := os.Remove(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return wrapIfInUse(err, path)
}

const sharingViolation = syscall.Errno(32)

func wrapIfInUse(err error, path string) error {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return err
	}
	if errno == syscall.EBUSY || (onWindows && errno == sharingViolation) {
		return errInUse(path, err)
	}
	return err
}

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

func ReplaceDir(source, destination string) error {
	if err := RemoveAll(destination); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o777); err != nil {
		return err
	}
	return CopyTree(source, destination)
}

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

func CopyProgram(source, destination string) (copied bool, err error) {
	program, err := os.ReadFile(source)
	if err != nil {
		return false, err
	}
	copied, err = writeIfChanged(destination, program, 0o777)
	if err != nil || !copied || onWindows {
		return copied, err
	}
	if err := os.Chmod(destination, 0o755); err != nil {
		return false, err
	}
	return true, nil
}

func WriteIfChanged(path, content string) (wrote bool, err error) {
	return writeIfChanged(path, []byte(content), 0o666)
}

func writeIfChanged(path string, content []byte, mode fs.FileMode) (wrote bool, err error) {
	existing, found, err := ReadFileIfExists(path)
	if err != nil || (found && bytes.Equal(existing, content)) {
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

func ReadSource(path, displayPath string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", errUnreadable(displayPath, err)
	}
	return blankShebang(TrimBOM(string(data))), nil
}

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

func SHA256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func ResolvePath(base, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(base, path)
}

func errInUse(path string, cause error) error {
	return &diag.Error{
		Msg:   path + " is in use by another program.",
		Hint:  "Close Warcraft III or World Editor if it has this map open, then try again.",
		Cause: cause,
	}
}

func errUnreadable(displayPath string, cause error) error {
	return &diag.Error{
		Msg:   "Reading " + displayPath + " failed: " + Reason(cause),
		File:  displayPath,
		Hint:  "Close any program that has the file open and check that it is a readable file, then try again.",
		Cause: cause,
	}
}
