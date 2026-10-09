package fsx

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/mdlsvensson/moonwell/internal/diag"
)

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

func IsSymlink(info fs.FileInfo) bool {
	return info.Mode()&fs.ModeSymlink != 0 || (onWindows && info.Mode()&fs.ModeIrregular != 0)
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

func errUnreadable(displayPath string, cause error) error {
	return &diag.Error{
		Msg:   "Reading " + displayPath + " failed: " + Reason(cause),
		File:  displayPath,
		Hint:  "Close any program that has the file open and check that it is a readable file, then try again.",
		Cause: cause,
	}
}
