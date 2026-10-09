package build

import (
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/fsx"
)

type dirFault int

const (
	dirOK dirFault = iota
	dirEscapes
	dirEmpty
	dirNotPortable
)

func parseDir(value string) (parts []string, fault dirFault) {
	parts = splitPath(value)
	switch {
	case isRooted(value) || slices.Contains(parts, ".."):
		return nil, dirEscapes
	case len(parts) == 0:
		return nil, dirEmpty
	case !isPortablePath(parts):
		return nil, dirNotPortable
	}
	return parts, dirOK
}

func isPortablePath(parts []string) bool {
	_, ok := fsx.CleanRelPath(strings.Join(parts, "/"))
	return ok
}

func splitPath(value string) []string {
	var parts []string
	for part := range strings.SplitSeq(strings.ReplaceAll(value, `\`, "/"), "/") {
		if part != "" && part != "." {
			parts = append(parts, part)
		}
	}
	return parts
}

func isRooted(path string) bool {
	if strings.HasPrefix(path, "/") || strings.HasPrefix(path, `\`) {
		return true
	}
	if len(path) < 2 || path[1] != ':' {
		return false
	}
	letter := path[0] | 0x20
	return letter >= 'a' && letter <= 'z'
}

const unusableNames = `A name cannot hold a control character or any of < > : " | ? *, end with a dot or a ` +
	"space, or be a device name such as CON or NUL."
