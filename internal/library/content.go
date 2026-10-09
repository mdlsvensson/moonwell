package library

import (
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

type libraryContent struct {
	modules     []archiveFile
	assets      []archiveFile
	shipsAssets bool
	isLocal     bool
}

func (c libraryContent) checkUsable(key, manifestName string) error {
	if err := c.checkUsableNames(key, "module", c.modules, manifestName); err != nil {
		return err
	}
	return c.checkUsableNames(key, "assets", c.assets, manifestName)
}

func (c libraryContent) checkUsableNames(key, kind string, files []archiveFile, manifestName string) error {
	names := make([]string, len(files))
	for i, file := range files {
		names[i] = file.name
	}
	slices.Sort(names)
	seen := map[string]string{}
	for _, name := range names {
		if _, ok := fsx.CleanRelPath(name); !ok || !isInsideLibrary(name) {
			return errUnusableName(key, kind, name, manifestName, c.isLocal)
		}
		if existing, taken := seen[strings.ToLower(name)]; taken {
			return errCaseConflict(key, kind, existing, name, manifestName, c.isLocal)
		}
		seen[strings.ToLower(name)] = name
	}
	if first, second, found := findFileDirCaseConflict(names, seen); found {
		return errCaseConflict(key, kind, first, second, manifestName, c.isLocal)
	}
	if first, second, found := findDirCaseConflict(names); found {
		return errDirCaseConflict(key, kind, first, second, manifestName, c.isLocal)
	}
	return nil
}

func findFileDirCaseConflict(paths []string, filesByLower map[string]string) (first, second string, found bool) {
	for _, path := range paths {
		for i, c := range path {
			if c != '/' {
				continue
			}
			dir := path[:i]
			if file, ok := filesByLower[strings.ToLower(dir)]; ok && file != dir {
				return min(file, dir), max(file, dir), true
			}
		}
	}
	return "", "", false
}

func findDirCaseConflict(paths []string) (first, second string, found bool) {
	type dirUse struct{ dir, path string }
	dirsByLower := map[string]dirUse{}
	for _, path := range paths {
		for i, c := range path {
			if c != '/' {
				continue
			}
			dir := path[:i]
			switch existing, taken := dirsByLower[strings.ToLower(dir)]; {
			case !taken:
				dirsByLower[strings.ToLower(dir)] = dirUse{dir, path}
			case existing.dir != dir:
				return existing.path, path, true
			}
		}
	}
	return "", "", false
}

func isInsideLibrary(path string) bool {
	if strings.ContainsAny(path, `\:`) {
		return false
	}
	for segment := range strings.SplitSeq(path, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

func hasHiddenSegment(path string) bool {
	return strings.HasPrefix(path, ".") || strings.Contains(path, "/.")
}

func unusableHint(isLocal bool, kind, what, them string) string {
	switch {
	case !isLocal:
		return reportHint
	case kind == "assets":
		return "Rename " + what + " in the library, or set assets in the library's " + File + " to a folder without " + them + "."
	}
	return "Rename " + what + " in the library, or set the library's dir to a folder without " + them + "."
}

func errUnusableName(key, kind, name, manifestName string, isLocal bool) error {
	return &diag.Error{
		Msg:  "Library " + key + ": " + name + " in its " + kind + " folder has a name that Windows cannot hold.",
		File: manifestName,
		Hint: unusableHint(isLocal, kind, "the file", "it"),
	}
}

func errCaseConflict(key, kind, first, second, manifestName string, isLocal bool) error {
	return &diag.Error{
		Msg:  "Library " + key + ": " + first + " and " + second + " in its " + kind + " folder differ only in letter case.",
		File: manifestName,
		Hint: unusableHint(isLocal, kind, "one of them", "them"),
	}
}

func errDirCaseConflict(key, kind, first, second, manifestName string, isLocal bool) error {
	return &diag.Error{
		Msg: "Library " + key + ": " + first + " and " + second + " in its " + kind +
			" folder lie in folders that differ only in letter case.",
		File: manifestName,
		Hint: unusableHint(isLocal, kind, "one of the two folders", "them"),
	}
}
