package fsx

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
)

var deviceName = regexp.MustCompile(`(?i)^(con|prn|aux|nul|com[1-9]|lpt[1-9])(?:\.|$)`)

func unsafeSegment(segment string) bool {
	if segment == "" || segment == "." || segment == ".." {
		return true
	}
	for _, r := range segment {
		if r < 32 {
			return true
		}
	}
	return strings.ContainsAny(segment, `<>:"|?*`) || strings.HasSuffix(segment, ".") ||
		strings.HasSuffix(segment, " ") || deviceName.MatchString(segment)
}

// RelPath returns value as a portable relative path with "/" separators. It rejects anything that could escape its
// folder or fail on Windows.
func RelPath(value string) (string, error) {
	normalized := strings.ReplaceAll(value, `\`, "/")
	invalid := normalized == ""
	for _, segment := range strings.Split(normalized, "/") {
		invalid = invalid || unsafeSegment(segment)
	}
	if invalid {
		return "", &diag.Error{
			Msg:  "Invalid asset path: " + value,
			Hint: "Use a relative path such as icons/BTNSword.blp, without .., drive letters or characters Windows forbids.",
		}
	}
	return normalized, nil
}

// LinkError is the failure for a symlink where Moonwell needs real files.
func LinkError(path string) error {
	return &diag.Error{
		Msg:  "Symlinks are not supported: " + path,
		Hint: "Replace the link (or Windows junction) with the real files.",
	}
}

// SafeJoin joins relative under root, refusing a symlink at every step below root so that data cannot leave its
// folder. root itself is trusted: it is chosen by the caller (such as a project opened through a junction), not an
// escape.
func SafeJoin(root, relative string) (string, error) {
	current, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	rel, err := RelPath(relative)
	if err != nil {
		return "", err
	}
	for _, segment := range strings.Split(rel, "/") {
		current = filepath.Join(current, segment)
		info, err := Lstat(current)
		if err != nil {
			return "", err
		}
		if info != nil && IsLink(info) {
			return "", LinkError(current)
		}
	}
	return current, nil
}
