package fsx

import (
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/mdlsvensson/moonwell/internal/diag"
)

var deviceName = regexp.MustCompile(`(?i)^(con|prn|aux|nul|com[1-9]|lpt[1-9])(?:\.|$)`)

func RelPath(value string) (path string, ok bool) {
	normalized := strings.ReplaceAll(value, `\`, "/")
	for segment := range strings.SplitSeq(normalized, "/") {
		if unsafeSegment(segment) {
			return "", false
		}
	}
	return normalized, true
}

func unsafeSegment(segment string) bool {
	if segment == "" || segment == "." || segment == ".." {
		return true
	}
	return strings.ContainsFunc(segment, isControl) || strings.ContainsAny(segment, `<>:"|?*`) ||
		strings.HasSuffix(segment, ".") || strings.HasSuffix(segment, " ") || deviceName.MatchString(segment)
}

func isControl(r rune) bool { return r < 32 }

func SafeJoin(root, relative string) (string, error) {
	return joinBelow(root, relative, refuseLink)
}

func joinBelow(root, relative string, look func(path string) error) (string, error) {
	current, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	rel, ok := RelPath(relative)
	if !ok {
		return "", errInvalidPath(relative)
	}
	for segment := range strings.SplitSeq(rel, "/") {
		current = filepath.Join(current, segment)
		if err := look(current); err != nil {
			return "", err
		}
	}
	return current, nil
}

func Inside(root, relative string) (string, error) {
	place, err := joinBelow(root, relative, stepInside)
	var refused *diag.Error
	switch {
	case err == nil:
		return place, nil
	case errors.As(err, &refused):
		return "", errRefused(relative, refused)
	}
	return "", errUnreachable(relative, err)
}

func stepInside(path string) error {
	err := refuseLink(path)
	if errors.Is(err, syscall.ENOTDIR) {
		return nil
	}
	return err
}

func refuseLink(path string) error {
	info, err := Lstat(path)
	if err != nil {
		return err
	}
	if info != nil && IsLink(info) {
		return LinkError(path)
	}
	return nil
}

func LinkError(path string) error {
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

func errRefused(relative string, refused *diag.Error) error {
	named := *refused
	named.File = relative
	return &named
}

func errUnreachable(relative string, cause error) error {
	return &diag.Error{
		Msg:   relative + " cannot be reached: " + Reason(cause),
		File:  relative,
		Hint:  "Make sure that every folder on the way to it can be read, then try again.",
		Cause: cause,
	}
}
