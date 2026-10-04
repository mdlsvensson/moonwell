package next

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const (
	newTree = "github.com/mdlsvensson/moonwell/next/internal/"
	oldTree = "github.com/mdlsvensson/moonwell/internal/"
)

var (
	foundations = []string{"manifest", "mapdir", "env", "diag", "fsx", "binio"}
	areas       = []string{"objects", "settings", "assets", "script", "library", "editor", "toolchain"}
	// belowFormats is what a war3/… package may import besides other war3/… packages.
	belowFormats = []string{"diag", "fsx", "binio"}
	testOnly     = []string{"testkit", "oracle"}
)

func isFormat(pkg string) bool { return strings.HasPrefix(pkg, "war3/") }

func onAShelf(pkg string) bool {
	return isFormat(pkg) || pkg == "build" || pkg == "cli" || slices.Contains(foundations, pkg) ||
		slices.Contains(areas, pkg) || slices.Contains(testOnly, pkg)
}

// allowed reports whether a non-test file of package from may import package to. Both are paths below
// next/internal.
func allowed(from, to string) bool {
	below := isFormat(to) || slices.Contains(foundations, to)
	switch {
	case slices.Contains(testOnly, to):
		return slices.Contains(testOnly, from)
	case slices.Contains(testOnly, from):
		return true
	case isFormat(from):
		return isFormat(to) || slices.Contains(belowFormats, to)
	case slices.Contains(foundations, from):
		return below
	case from == "editor" && (to == "objects" || to == "script"):
		return true
	case slices.Contains(areas, from):
		return below
	case from == "build":
		return below || slices.Contains(areas, to)
	case from == "cli":
		return to != "cli"
	}
	return false
}

func TestImportsOnlyGoDownTheShelves(t *testing.T) {
	err := filepath.WalkDir("internal", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		from := filepath.ToSlash(strings.TrimPrefix(filepath.Dir(path), "internal"+string(filepath.Separator)))
		if !onAShelf(from) {
			t.Errorf("%s: package %s is on no shelf; add it to this test and to the spec's layout", path, from)
		}
		isTest := strings.HasSuffix(path, "_test.go")
		for _, spec := range file.Imports {
			target, _ := strconv.Unquote(spec.Path.Value)
			if strings.HasPrefix(target, oldTree) && filepath.Base(path) != "oracle_test.go" && from != "oracle" {
				t.Errorf("%s imports the old tree (%s); only oracle_test.go files may", path, target)
			}
			if to, inNewTree := strings.CutPrefix(target, newTree); inNewTree && !isTest && !allowed(from, to) {
				t.Errorf("%s: package %s must not import %s", path, from, to)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
