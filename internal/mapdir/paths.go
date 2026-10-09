package mapdir

import (
	"strings"
)

func Key(path string) string {
	return strings.ToLower(toSlash(path))
}

func toSlash(path string) string { return strings.ReplaceAll(path, `\`, "/") }

func joinPath(dir, name string) string {
	if dir == "" {
		return name
	}
	return dir + "/" + name
}

func parentDirs(path string) []string {
	var dirs []string
	for i := range len(path) {
		if path[i] == '/' {
			dirs = append(dirs, path[:i])
		}
	}
	return dirs
}
