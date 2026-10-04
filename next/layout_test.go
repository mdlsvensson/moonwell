package next

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
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
	// testOnly is the packages that only test files import. tooltest gives a test the tools that toolchain
	// fetches, so it imports that area.
	testOnly = []string{"testkit", "oracle", "tooltest"}
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
	case from == "testkit" && (slices.Contains(areas, to) || to == "tooltest"):
		// The tests of the foundations import testkit, and every area imports a foundation: an area in testkit,
		// or tooltest, which imports one, would be an import cycle in those tests.
		return false
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

// allowedInATest reports whether a test file of package from may import package to: what a file of the package
// that is not a test may, and also the package itself and the test-only packages. So the tests of an area keep
// off the other areas, off build and off cli as the area does, and those of a foundation or a format keep off
// the areas.
func allowedInATest(from, to string) bool {
	return from == to || slices.Contains(testOnly, to) || allowed(from, to)
}

// shelved is the package of a file as a path below internal, and whether the file is below internal at all. The
// file is a path below this folder with "/" separators.
func shelved(file string) (pkg string, is bool) {
	pkg, is = strings.CutPrefix(path.Dir(file)+"/", "internal/")
	return strings.TrimSuffix(pkg, "/"), is
}

// walkShelves reads the imports of every Go file below root, a folder laid out as this one, and reports each rule
// a file breaks, naming the file by its path below root with "/".
func walkShelves(root string, report func(format string, args ...any)) error {
	return filepath.WalkDir(root, func(file string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(file, ".go") {
			return err
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		if file, err = filepath.Rel(root, file); err != nil {
			return err
		}
		file = filepath.ToSlash(file)
		from, onShelves := shelved(file)
		if onShelves && !onAShelf(from) {
			report("%s: package %s is on no shelf; add it to this test and to the spec's layout", file, from)
		}
		isTest := strings.HasSuffix(file, "_test.go")
		for _, spec := range parsed.Imports {
			target, _ := strconv.Unquote(spec.Path.Value)
			if strings.HasPrefix(target, oldTree) && path.Base(file) != "oracle_test.go" && from != "oracle" {
				report("%s imports the old tree (%s); only oracle_test.go files may", file, target)
			}
			to, inNewTree := strings.CutPrefix(target, newTree)
			switch {
			case !inNewTree || !onShelves:
			case isTest && !allowedInATest(from, to):
				report("%s: a test of package %s must not import %s", file, from, to)
			case !isTest && !allowed(from, to):
				report("%s: package %s must not import %s", file, from, to)
			}
		}
		return nil
	})
}

// What is checked, for every Go file below this folder:
//
//   - It keeps off the old tree, unless it is named oracle_test.go or is in the package oracle.
//   - A file below internal is in a package that is on a shelf. A shelf that has no package yet is no failure.
//   - A file below internal that is not a test imports only what its shelf may (allowed): a foundation what its
//     package comment names, a format the formats and what is below them, an area the foundations and the
//     formats and never another area, but for editor, which may import objects and script. A test-only package
//     (testkit, oracle, tooltest) is imported by no such file but one of a test-only package, and testkit imports
//     no area and not tooltest.
//   - A test file below internal follows the rule of its package, and may also import its own package and the
//     test-only packages (allowedInATest). So the tests of an area import no other area, with editor's exception,
//     and neither build nor cli; the tests of a foundation or a format import no area.
func TestImportsOnlyGoDownTheShelves(t *testing.T) {
	if err := walkShelves(".", t.Errorf); err != nil {
		t.Fatal(err)
	}
}

// The walk is given a small tree with a file of each kind that breaks a rule, beside files that keep them, and
// must report those two and nothing else.
func TestTheWalkReportsAFileThatBreaksARuleATestFileToo(t *testing.T) {
	importing := func(pkg string, targets ...string) string {
		source := "package " + pkg + "\n"
		for _, target := range targets {
			source += "\nimport _ " + strconv.Quote(target)
		}
		return source + "\n"
	}
	files := map[string]string{
		"internal/assets/breaks_test.go": importing("assets", newTree+"mapdir", newTree+"settings"),
		"internal/mapdir/breaks.go":      importing("mapdir", newTree+"fsx", newTree+"assets"),
		"internal/assets/keeps_test.go":  importing("assets", newTree+"assets", newTree+"testkit", newTree+"war3/imp"),
		"internal/assets/oracle_test.go": importing("assets", oldTree+"assets", newTree+"oracle"),
		"internal/editor/keeps.go":       importing("editor", newTree+"objects", newTree+"script", newTree+"mapdir"),
		"internal/mapdir/keeps_test.go":  importing("mapdir", newTree+"testkit", newTree+"diag"),
	}
	root := t.TempDir()
	for name, source := range files {
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o777); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(source), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	var reported []string
	report := func(format string, args ...any) { reported = append(reported, fmt.Sprintf(format, args...)) }
	if err := walkShelves(root, report); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"internal/assets/breaks_test.go: a test of package assets must not import settings",
		"internal/mapdir/breaks.go: package mapdir must not import assets",
	}
	if !slices.Equal(reported, want) {
		t.Errorf("the walk reported %q, want %q", reported, want)
	}
}

func TestTheRulesOfTheShelvesForTestOnlyPackagesAndForTestFiles(t *testing.T) {
	cases := []struct {
		from, to string
		inATest  bool
		want     bool
	}{
		// tooltest is test-only: it may import an area, no file that is not a test imports it, and testkit keeps off
		// it as it keeps off the areas.
		{"tooltest", "toolchain", false, true},
		{"tooltest", "testkit", false, true},
		{"toolchain", "tooltest", false, false},
		{"script", "tooltest", false, false},
		{"testkit", "tooltest", false, false},
		{"testkit", "toolchain", false, false},
		{"testkit", "env", false, true},
		// No area imports another, in its tests neither; editor may import objects and script.
		{"script", "toolchain", false, false},
		{"script", "toolchain", true, false},
		{"assets", "settings", true, false},
		{"editor", "objects", false, true},
		{"editor", "objects", true, true},
		{"editor", "script", true, true},
		{"editor", "assets", true, false},
		// A test follows the rule of its package: an area's keeps off build and cli, and a foundation's and a
		// format's keep off the areas and off what their package may not import.
		{"assets", "build", true, false},
		{"assets", "cli", true, false},
		{"mapdir", "assets", true, false},
		{"mapdir", "env", true, false},
		{"war3/imp", "assets", true, false},
		{"war3/imp", "mapdir", true, false},
		{"war3/imp", "war3/lua", true, true},
		// A test of an area may import the test-only packages, its own package, and what is below it.
		{"script", "tooltest", true, true},
		{"script", "testkit", true, true},
		{"assets", "oracle", true, true},
		{"assets", "assets", true, true},
		{"assets", "mapdir", true, true},
		{"assets", "war3/imp", true, true},
		{"mapdir", "testkit", true, true},
	}
	for _, c := range cases {
		got := allowed(c.from, c.to)
		if c.inATest {
			got = allowedInATest(c.from, c.to)
		}
		if got != c.want {
			t.Errorf("%s importing %s, in a test %v: allowed = %v, want %v", c.from, c.to, c.inATest, got, c.want)
		}
	}
	if !onAShelf("tooltest") {
		t.Error("tooltest is on no shelf")
	}
}
