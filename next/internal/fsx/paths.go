package fsx

import (
	"errors"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
)

// deviceName matches a name Windows reserves for a device, with or without an extension: "con", "NUL.txt".
var deviceName = regexp.MustCompile(`(?i)^(con|prn|aux|nul|com[1-9]|lpt[1-9])(?:\.|$)`)

// RelPath returns value as a portable relative path with "/" separators. It reports false for a path that could
// leave its folder or fail on Windows.
func RelPath(value string) (path string, ok bool) {
	normalized := strings.ReplaceAll(value, `\`, "/")
	for segment := range strings.SplitSeq(normalized, "/") {
		if unsafeSegment(segment) {
			return "", false
		}
	}
	return normalized, true
}

// unsafeSegment reports whether one name of a path is empty, points at a folder (".", ".."), or is a name Windows
// cannot hold: a control or reserved character, a dot or a space at the end, or a device.
func unsafeSegment(segment string) bool {
	if segment == "" || segment == "." || segment == ".." {
		return true
	}
	return strings.ContainsFunc(segment, isControl) || strings.ContainsAny(segment, `<>:"|?*`) ||
		strings.HasSuffix(segment, ".") || strings.HasSuffix(segment, " ") || deviceName.MatchString(segment)
}

// isControl reports whether r is one of the control characters below the space.
func isControl(r rune) bool { return r < 32 }

// SafeJoin joins relative under root, refusing a link at every step below root so that data cannot leave its
// folder. root itself is trusted: the caller chose it (a project opened through a junction, say), so it is no way
// out.
func SafeJoin(root, relative string) (string, error) {
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
		if err := refuseLink(current); err != nil {
			return "", err
		}
	}
	return current, nil
}

// Inside returns the place of relative below root, for a file or folder of a project. relative is written with
// "/". A path that leaves root, and a link on the way to it, is refused with a *diag.Error whose File is relative;
// so is a way the system cannot look at, with its Cause.
//
// It is SafeJoin with every failure worded for a user: a caller passes its error on as it is. Nothing need be at
// the place, and root itself is trusted, as SafeJoin says.
func Inside(root, relative string) (string, error) {
	place, err := SafeJoin(root, relative)
	var refused *diag.Error
	switch {
	case err == nil:
		return place, nil
	case errors.As(err, &refused):
		return "", errRefused(relative, refused)
	}
	return "", errUnreachable(relative, err)
}

// refuseLink fails when path is a link. A path that does not exist is no link.
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

// ---- errors ----

// LinkError is the failure for a link where Moonwell needs real files.
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

// errRefused is a refusal of SafeJoin, in its words, with the path as it was given as its file.
func errRefused(relative string, refused *diag.Error) error {
	named := *refused
	named.File = relative
	return &named
}

// errUnreachable is the failure of the system to look at a step of the way to relative: a folder that may not be
// read, or a name the system cannot hold.
func errUnreachable(relative string, cause error) error {
	return &diag.Error{
		Msg:   relative + " cannot be reached: " + Reason(cause),
		File:  relative,
		Hint:  "Make sure that every folder on the way to it can be read, then try again.",
		Cause: cause,
	}
}
