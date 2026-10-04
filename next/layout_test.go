package next

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path"
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
	// foundations is each foundation with the packages of the new tree it may import: what its package comment
	// says it imports, and no more. No foundation imports a war3/… package.
	foundations = map[string][]string{
		"diag":     nil,
		"binio":    nil,
		"fsx":      {"diag"},
		"env":      {"diag", "fsx"},
		"mapdir":   {"diag", "fsx"},
		"manifest": {"env", "diag", "fsx"},
	}
	areas = []string{"objects", "settings", "assets", "script", "library", "editor", "toolchain"}
	// belowFormats is what a war3/… package may import besides other war3/… packages.
	belowFormats = []string{"diag", "fsx", "binio"}
	testOnly     = []string{"testkit", "oracle"}
)

func isFormat(pkg string) bool { return strings.HasPrefix(pkg, "war3/") }

func isFoundation(pkg string) bool {
	_, is := foundations[pkg]
	return is
}

func onAShelf(pkg string) bool {
	return isFormat(pkg) || pkg == "build" || pkg == "cli" || isFoundation(pkg) ||
		slices.Contains(areas, pkg) || slices.Contains(testOnly, pkg)
}

// allowed reports whether a non-test file of package from may import package to. Both are paths below
// next/internal.
func allowed(from, to string) bool {
	below := isFormat(to) || isFoundation(to)
	switch {
	case slices.Contains(testOnly, to):
		return slices.Contains(testOnly, from)
	case slices.Contains(testOnly, from):
		return true
	case isFormat(from):
		return isFormat(to) || slices.Contains(belowFormats, to)
	case isFoundation(from):
		return slices.Contains(foundations[from], to)
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

// shelved is the package of a file as a path below internal, and whether the file is below internal at all. The
// file is a path below this folder with "/" separators.
func shelved(file string) (pkg string, is bool) {
	pkg, is = strings.CutPrefix(path.Dir(file)+"/", "internal/")
	return strings.TrimSuffix(pkg, "/"), is
}

// Every Go file below this folder keeps off the old tree, unless it is an oracle_test.go or in the package oracle.
// The files below internal are also on the shelves: each package is on one, and imports only what its shelf may.
func TestImportsOnlyGoDownTheShelves(t *testing.T) {
	err := filepath.WalkDir(".", func(file string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(file, ".go") {
			return err
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		file = filepath.ToSlash(file)
		from, onShelves := shelved(file)
		if onShelves && !onAShelf(from) {
			t.Errorf("%s: package %s is on no shelf; add it to this test and to the spec's layout", file, from)
		}
		isTest := strings.HasSuffix(file, "_test.go")
		for _, spec := range parsed.Imports {
			target, _ := strconv.Unquote(spec.Path.Value)
			if strings.HasPrefix(target, oldTree) && path.Base(file) != "oracle_test.go" && from != "oracle" {
				t.Errorf("%s imports the old tree (%s); only oracle_test.go files may", file, target)
			}
			to, inNewTree := strings.CutPrefix(target, newTree)
			if inNewTree && onShelves && !isTest && !allowed(from, to) {
				t.Errorf("%s: package %s must not import %s", file, from, to)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
