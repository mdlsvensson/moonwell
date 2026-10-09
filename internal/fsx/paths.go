package fsx

import (
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/mdlsvensson/moonwell/internal/diag"
)

func ToSlash(path string) string { return filepath.ToSlash(path) }

func ResolvePath(base, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(base, path)
}

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

var deviceName = regexp.MustCompile(`(?i)^(con|prn|aux|nul|com[1-9]|lpt[1-9])(?:\.|$)`)

func CleanRelPath(value string) (path string, ok bool) {
	normalized := strings.ReplaceAll(value, `\`, "/")
	for segment := range strings.SplitSeq(normalized, "/") {
		if isUnsafeSegment(segment) {
			return "", false
		}
	}
	return normalized, true
}

func isUnsafeSegment(segment string) bool {
	if segment == "" || segment == "." || segment == ".." {
		return true
	}
	return strings.ContainsFunc(segment, isControlRune) || strings.ContainsAny(segment, `<>:"|?*`) ||
		strings.HasSuffix(segment, ".") || strings.HasSuffix(segment, " ") || deviceName.MatchString(segment)
}

func isControlRune(r rune) bool { return r < 32 }

func SafeJoin(root, relative string) (string, error) {
	return joinChecked(root, relative, checkNotSymlink)
}

func SafeJoinNoSymlinks(root, relative string) (string, error) {
	fullPath, err := joinChecked(root, relative, checkNotSymlinkPastFile)
	var diagErr *diag.Error
	switch {
	case err == nil:
		return fullPath, nil
	case errors.As(err, &diagErr):
		return "", errPathRefused(relative, diagErr)
	}
	return "", errPathUnreachable(relative, err)
}

func joinChecked(root, relative string, check func(path string) error) (string, error) {
	fullPath, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	rel, ok := CleanRelPath(relative)
	if !ok {
		return "", errInvalidPath(relative)
	}
	for segment := range strings.SplitSeq(rel, "/") {
		fullPath = filepath.Join(fullPath, segment)
		if err := check(fullPath); err != nil {
			return "", err
		}
	}
	return fullPath, nil
}

func checkNotSymlinkPastFile(path string) error {
	err := checkNotSymlink(path)
	if errors.Is(err, syscall.ENOTDIR) {
		return nil
	}
	return err
}

func checkNotSymlink(path string) error {
	info, err := Lstat(path)
	if err != nil {
		return err
	}
	if info != nil && IsSymlink(info) {
		return NewSymlinkError(path)
	}
	return nil
}

func NewSymlinkError(path string) error {
	return &diag.Error{
		Msg:  "Symlinks are not supported: " + path,
		Hint: "Replace the link (or Windows junction) with the real files.",
	}
}

func errInvalidPath(value string) error {
	return &diag.Error{
		Msg: "Invalid path: " + value,
		Hint: "Use a relative path such as icons/BTNSword.blp, without .., drive letters or characters Windows " +
			"forbids.",
	}
}

func errPathRefused(relative string, diagErr *diag.Error) error {
	withFile := *diagErr
	withFile.File = relative
	return &withFile
}

func errPathUnreachable(relative string, cause error) error {
	return &diag.Error{
		Msg:   relative + " cannot be reached: " + Reason(cause),
		File:  relative,
		Hint:  "Make sure that every folder on the way to it can be read, then try again.",
		Cause: cause,
	}
}
