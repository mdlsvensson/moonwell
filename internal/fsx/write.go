package fsx

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"github.com/mdlsvensson/moonwell/internal/diag"
)

func WriteIfChanged(path, content string) (wrote bool, err error) {
	return writeIfChanged(path, []byte(content), 0o666)
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
		symlink, err := os.Readlink(path)
		if err != nil {
			return err
		}
		return os.Symlink(symlink, target)
	}
	return CopyFile(path, target)
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

func errInUse(path string, cause error) error {
	return &diag.Error{
		Msg:   path + " is in use by another program.",
		Hint:  "Close Warcraft III or World Editor if it has this map open, then try again.",
		Cause: cause,
	}
}
