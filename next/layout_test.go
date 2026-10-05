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
	module  = "github.com/mdlsvensson/moonwell"
	newTree = module + "/next/internal/"
	oldTree = module + "/internal/"
	// toolsTree is what a contributor runs and the program is not built from: the generator, tools/gen, and its
	// parsers, each a package directly below it.
	toolsTree = module + "/next/tools/"
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
	// oldPrograms is the old tree's command and its generators, which lie beside its internal folder. The root
	// package of the module is no part of the old tree: it holds the embedded files and Version.
	oldPrograms = []string{module + "/cmd/", module + "/tools/"}
	// outsideWorld is the packages of the standard library that start a program and that reach the network.
	// Only env imports them, and the files that only tests are built from: every other package reaches the
	// outside through an env.Env, which a test replaces.
	outsideWorld = []string{"os/exec", "net/http"}
	// excused is the packages beside env with a file, not a test, that imports one of outsideWorld, each with
	// the imports it is let off for.
	excused = map[string][]string{
		// fsx.Reason takes the system's reason out of an *exec.Error, the failure to find or to start a program:
		// it names the type, and starts nothing.
		"fsx": {"os/exec"},
	}
	// generatorMay is the packages below internal that the generator may import: the three areas whose data it
	// writes, and the two foundations it takes the names of a manifest and the text of a file from.
	generatorMay = []string{"objects", "script", "assets", "manifest", "fsx"}
	// parserTestsMay is the packages below internal, beside the test-only ones, that a test of a parser may
	// import: fsx decodes the text that a parser is given.
	parserTestsMay = []string{"fsx"}
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
	case slices.Contains(testOnly, from) && (to == "build" || to == "cli"):
		// The tests of the areas import the test-only packages, and build and cli import the areas: either of the
		// two in a test-only package would be an import cycle in those tests.
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
		return below || slices.Contains(areas, to) || to == "build"
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

// goFile is a Go file as the rules see it.
type goFile struct {
	path    string // below the folder that is walked, with "/"
	pkg     string // its package as a path below internal; "" for a file that is not below internal
	shelved bool   // it is below internal
	tool    string // its package as a path below tools; "" for a file that is not below tools, or right in it
	tooled  bool   // it is below tools
	isTest  bool   // it is named _test.go
}

// fileAt is the file at a path below the folder that is walked, with "/".
func fileAt(file string) goFile {
	pkg, shelved := packageBelow(file, "internal")
	tool, tooled := packageBelow(file, "tools")
	return goFile{file, pkg, shelved, tool, tooled, strings.HasSuffix(file, "_test.go")}
}

// packageBelow is the folder of the file as a path below top, and whether the file is below top.
func packageBelow(file, top string) (string, bool) {
	pkg, is := strings.CutPrefix(path.Dir(file)+"/", top+"/")
	if !is {
		return "", false
	}
	return strings.TrimSuffix(pkg, "/"), true
}

// isParser reports whether a package below tools is a parser of the generator: one directly below tools/gen.
func isParser(tool string) bool { return path.Dir(tool) == "gen" }

// rules is the rules on one import of a file, in the order their reports are made. Each returns what the import
// breaks; "" for an import that keeps the rule.
var rules = []func(f goFile, target string) string{
	offTheOldTree, insideTheProgram, cliAlone, downTheShelves, offTheTools, amongTheTools,
}

// offTheOldTree holds a file off the old tree: its internal folder, its command and its generators. A file
// named oracle_test.go compares the two trees, and may import any of it. The package oracle compares their
// errors, and may import what is below the internal folder.
func offTheOldTree(f goFile, target string) string {
	switch {
	case path.Base(f.path) == "oracle_test.go":
	case isOldProgram(target), strings.HasPrefix(target, oldTree) && f.pkg != "oracle":
		return fmt.Sprintf("%s imports the old tree (%s); only oracle_test.go files may", f.path, target)
	}
	return ""
}

// isOldProgram reports whether target is a package of the old tree's command or of its generators.
func isOldProgram(target string) bool {
	return slices.ContainsFunc(oldPrograms, func(prefix string) bool { return strings.HasPrefix(target, prefix) })
}

// insideTheProgram holds a file off outsideWorld, unless it is of env, of a test-only package, a test, or
// excused.
func insideTheProgram(f goFile, target string) string {
	let := f.isTest || f.pkg == "env" || slices.Contains(testOnly, f.pkg) || slices.Contains(excused[f.pkg], target)
	if let || !slices.Contains(outsideWorld, target) {
		return ""
	}
	return fmt.Sprintf("%s imports %s; only env and test files may", f.path, target)
}

// cliAlone holds a file below cmd/ to cli, of all the module: a command is a call of cli. Its tests may import
// the test-only packages too. An import of the old tree is offTheOldTree's to judge, and one of a package below
// tools offTheTools'.
func cliAlone(f goFile, target string) string {
	inModule := target == module || strings.HasPrefix(target, module+"/")
	inOldTree := strings.HasPrefix(target, oldTree) || isOldProgram(target)
	to, inNewTree := strings.CutPrefix(target, newTree)
	switch {
	case !strings.HasPrefix(f.path, "cmd/") || !inModule || inOldTree || strings.HasPrefix(target, toolsTree):
	case inNewTree && to == "cli":
	case inNewTree && f.isTest && slices.Contains(testOnly, to):
	default:
		return fmt.Sprintf("%s: a command imports cli and nothing else of the module, not %s", f.path, target)
	}
	return ""
}

// downTheShelves holds a file below internal to what its shelf may import of the new tree.
func downTheShelves(f goFile, target string) string {
	to, inNewTree := strings.CutPrefix(target, newTree)
	switch {
	case !inNewTree || !f.shelved:
	case f.isTest && !allowedInATest(f.pkg, to):
		return fmt.Sprintf("%s: a test of package %s must not import %s", f.path, f.pkg, to)
	case !f.isTest && !allowed(f.pkg, to):
		return fmt.Sprintf("%s: package %s must not import %s", f.path, f.pkg, to)
	}
	return ""
}

// offTheTools holds a file that is not below tools off the generator and its parsers: the program is not built
// from them, and its tests do not lean on them.
func offTheTools(f goFile, target string) string {
	if f.tooled || !strings.HasPrefix(target, toolsTree) {
		return ""
	}
	return fmt.Sprintf("%s imports %s; only the generator and its parsers import a package below tools", f.path, target)
}

// amongTheTools holds a file below tools to what its package may import of the module (toolMay). An import of
// the old tree is offTheOldTree's to judge.
func amongTheTools(f goFile, target string) string {
	inModule := target == module || strings.HasPrefix(target, module+"/")
	inOldTree := strings.HasPrefix(target, oldTree) || isOldProgram(target)
	switch {
	case !f.tooled || !inModule || inOldTree || toolMay(f, target):
		return ""
	case f.isTest:
		return fmt.Sprintf("%s: a test of package %s must not import %s", f.path, path.Join("tools", f.tool), target)
	}
	return fmt.Sprintf("%s: package %s must not import %s", f.path, path.Join("tools", f.tool), target)
}

// toolMay reports whether a file below tools may import a package of the module. The generator may import the
// root package, the packages of generatorMay, and its parsers. A parser may import nothing. A test may also
// import its own package and the test-only packages, and a test of a parser the packages of parserTestsMay.
func toolMay(f goFile, target string) bool {
	to, inNewTree := strings.CutPrefix(target, newTree)
	below := func(packages []string) bool { return inNewTree && slices.Contains(packages, to) }
	switch {
	case f.isTest && (target == toolsTree+f.tool || below(testOnly)):
		return true
	case f.tool == "gen":
		return target == module || below(generatorMay) || isParser(strings.TrimPrefix(target, toolsTree))
	case isParser(f.tool):
		return f.isTest && below(parserTestsMay)
	}
	return false
}

// broken is every rule the file breaks with its imports: that of its package first, then those of each import
// in the order of the rules.
func (f goFile) broken(imports []string) []string {
	var reports []string
	const onNoShelf = "%s: package %s is on no shelf; add it to this test and to the spec's layout"
	if f.shelved && !onAShelf(f.pkg) {
		reports = append(reports, fmt.Sprintf(onNoShelf, f.path, f.pkg))
	}
	if f.tooled && f.tool != "gen" && !isParser(f.tool) {
		reports = append(reports, fmt.Sprintf(onNoShelf, f.path, path.Join("tools", f.tool)))
	}
	for _, target := range imports {
		for _, rule := range rules {
			if report := rule(f, target); report != "" {
				reports = append(reports, report)
			}
		}
	}
	return reports
}

// importsOf is the paths that the Go file on disk imports, in the order it names them.
func importsOf(file string) ([]string, error) {
	parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	var imports []string
	for _, spec := range parsed.Imports {
		target, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, err
		}
		imports = append(imports, target)
	}
	return imports, nil
}

// walkShelves reads the imports of every Go file below root, a folder laid out as this one, and reports each rule
// a file breaks, naming the file by its path below root with "/".
func walkShelves(root string, report func(format string, args ...any)) error {
	return filepath.WalkDir(root, func(file string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(file, ".go") {
			return err
		}
		imports, err := importsOf(file)
		if err != nil {
			return err
		}
		below, err := filepath.Rel(root, file)
		if err != nil {
			return err
		}
		for _, broken := range fileAt(filepath.ToSlash(below)).broken(imports) {
			report("%s", broken)
		}
		return nil
	})
}

// What is checked, for every Go file below this folder:
//
//   - It keeps off the old tree, which is the old module's internal folder, its command (cmd/) and its
//     generators (tools/), unless it is named oracle_test.go. The package oracle may import what is below the
//     internal folder. The module's root package is no part of the old tree (offTheOldTree).
//   - It imports neither os/exec nor net/http, unless it is of env, of a test-only package, or a test. fsx is
//     excused for os/exec, and excused says why (insideTheProgram).
//   - A file below cmd/ imports cli and nothing else of the module, and a test there the test-only packages too
//     (cliAlone).
//   - A file below internal is in a package that is on a shelf. A shelf that has no package yet is no failure.
//   - A file below internal that is not a test imports only what its shelf may (allowed): a foundation what its
//     package comment names, a format the formats and what is below them, an area the foundations and the
//     formats and never another area, but for editor, which may import objects and script; build the areas and
//     what is below them; cli build and what build may. A test-only package (testkit, oracle, tooltest) is
//     imported by no such file but one of a test-only package; it imports neither build nor cli, and testkit
//     imports no area and not tooltest.
//   - A test file below internal follows the rule of its package, and may also import its own package and the
//     test-only packages (allowedInATest). So the tests of an area import no other area, with editor's exception,
//     and neither build nor cli; the tests of a foundation or a format import no area.
//   - A file below tools is of the generator, tools/gen, or of one of its parsers, the packages directly below
//     it: any other folder there is on no shelf. Of the module, the generator imports the root package, the
//     areas objects, script and assets, the foundations manifest and fsx, and its parsers; a parser imports
//     nothing. A test file there follows the rule of its package, and may also import its own package and the
//     test-only packages; a test of a parser may import fsx too, which decodes the text a parser is given
//     (amongTheTools).
//   - A file that is not below tools imports no package below tools (offTheTools).
func TestImportsOnlyGoDownTheShelves(t *testing.T) {
	if err := walkShelves(".", t.Errorf); err != nil {
		t.Fatal(err)
	}
}

// importing is the source of a file of the package that imports the targets.
func importing(pkg string, targets ...string) string {
	source := "package " + pkg + "\n"
	for _, target := range targets {
		source += "\nimport _ " + strconv.Quote(target)
	}
	return source + "\n"
}

// walked plants a tree of the files, each a path below the tree's folder with "/" and the file's source, walks
// it, and fails the test unless the walk reports what is wanted and nothing else. The order of the reports does
// not count.
func walked(t *testing.T, files map[string]string, want ...string) {
	t.Helper()
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
	slices.Sort(reported)
	if want = slices.Sorted(slices.Values(want)); !slices.Equal(reported, want) {
		t.Errorf("the walk reported:\n%s\nwant:\n%s", strings.Join(reported, "\n"), strings.Join(want, "\n"))
	}
}

// The walk is given a small tree with a file of each kind that breaks a rule, beside files that keep them, and
// must report those two and nothing else.
func TestTheWalkReportsAFileThatBreaksARuleATestFileToo(t *testing.T) {
	walked(t, map[string]string{
		"internal/assets/breaks_test.go": importing("assets", newTree+"mapdir", newTree+"settings"),
		"internal/mapdir/breaks.go":      importing("mapdir", newTree+"fsx", newTree+"assets"),
		"internal/assets/keeps_test.go":  importing("assets", newTree+"assets", newTree+"testkit", newTree+"war3/imp"),
		"internal/assets/oracle_test.go": importing("assets", oldTree+"assets", newTree+"oracle"),
		"internal/editor/keeps.go":       importing("editor", newTree+"objects", newTree+"script", newTree+"mapdir"),
		"internal/mapdir/keeps_test.go":  importing("mapdir", newTree+"testkit", newTree+"diag"),
	},
		"internal/assets/breaks_test.go: a test of package assets must not import settings",
		"internal/mapdir/breaks.go: package mapdir must not import assets",
	)
}

// build stands above the areas and below cli, and cli above them all; a package that is on no shelf is no
// package that either may import.
func TestTheWalkHoldsBuildAndCliToTheirShelves(t *testing.T) {
	// in is the root package of the module, which every package may import, and the packages of the new tree.
	in := func(packages ...string) []string {
		targets := []string{module}
		for _, pkg := range packages {
			targets = append(targets, newTree+pkg)
		}
		return targets
	}
	walked(t, map[string]string{
		"internal/build/keeps.go":       importing("build", in("script", "editor", "mapdir", "env", "war3/mpq")...),
		"internal/build/keeps_test.go":  importing("build", in("build", "testkit", "oracle", "script")...),
		"internal/build/breaks.go":      importing("build", in("cli", "testkit", "watch")...),
		"internal/build/breaks_test.go": importing("build", in("cli")...),
		"internal/cli/keeps.go":         importing("cli", in("build", "toolchain", "manifest", "diag", "war3/lua")...),
		"internal/cli/keeps_test.go":    importing("cli", in("cli", "build", "testkit", "tooltest")...),
		"internal/cli/breaks.go":        importing("cli", in("oracle", "watch")...),
		"internal/cli/breaks_test.go":   importing("cli", in("watch")...),
		"internal/script/breaks.go":     importing("script", in("build", "cli")...),
		"internal/env/breaks_test.go":   importing("env", in("build")...),
	},
		"internal/build/breaks.go: package build must not import cli",
		"internal/build/breaks.go: package build must not import testkit",
		"internal/build/breaks.go: package build must not import watch",
		"internal/build/breaks_test.go: a test of package build must not import cli",
		"internal/cli/breaks.go: package cli must not import oracle",
		"internal/cli/breaks.go: package cli must not import watch",
		"internal/cli/breaks_test.go: a test of package cli must not import watch",
		"internal/script/breaks.go: package script must not import build",
		"internal/script/breaks.go: package script must not import cli",
		"internal/env/breaks_test.go: a test of package env must not import build",
	)
}

// The tests of the areas import the test-only packages, and build and cli import the areas: so a test-only
// package imports neither of the two, in a test of its own neither. It may import an area, but for testkit.
func TestTheWalkHoldsTheTestOnlyPackagesOffBuildAndCli(t *testing.T) {
	walked(t, map[string]string{
		"internal/testkit/keeps.go":      importing("testkit", newTree+"env", newTree+"war3/mpq"),
		"internal/oracle/keeps.go":       importing("oracle", newTree+"script", newTree+"testkit", newTree+"diag"),
		"internal/tooltest/keeps.go":     importing("tooltest", newTree+"toolchain", newTree+"env"),
		"internal/testkit/breaks.go":     importing("testkit", newTree+"build", newTree+"cli"),
		"internal/oracle/breaks.go":      importing("oracle", newTree+"cli"),
		"internal/oracle/breaks_test.go": importing("oracle", newTree+"build", newTree+"oracle"),
		"internal/tooltest/breaks.go":    importing("tooltest", newTree+"build", newTree+"cli"),
	},
		"internal/testkit/breaks.go: package testkit must not import build",
		"internal/testkit/breaks.go: package testkit must not import cli",
		"internal/oracle/breaks.go: package oracle must not import cli",
		"internal/oracle/breaks_test.go: a test of package oracle must not import build",
		"internal/tooltest/breaks.go: package tooltest must not import build",
		"internal/tooltest/breaks.go: package tooltest must not import cli",
	)
}

// A command is main and a call of cli: what it imports of the module is cli, and in a test the test-only
// packages too. The root package and build are for cli to import.
func TestTheWalkLetsACommandImportCliAlone(t *testing.T) {
	walked(t, map[string]string{
		"cmd/moonwell/main.go":        importing("main", "os", newTree+"cli"),
		"cmd/moonwell/main_test.go":   importing("main", "testing", newTree+"cli", newTree+"testkit", newTree+"oracle"),
		"cmd/moonwell/oracle_test.go": importing("main", newTree+"cli", module+"/cmd/moonwell", oldTree+"cli"),
		"cmd/moonwell/breaks.go":      importing("main", newTree+"build", newTree+"env", module, module+"/next"),
		"cmd/moonwell/breaks_test.go": importing("main", newTree+"script", newTree+"clitest"),
		"cmd/other/breaks.go":         importing("main", newTree+"testkit"),
	},
		"cmd/moonwell/breaks.go: a command imports cli and nothing else of the module, not "+newTree+"build",
		"cmd/moonwell/breaks.go: a command imports cli and nothing else of the module, not "+newTree+"env",
		"cmd/moonwell/breaks.go: a command imports cli and nothing else of the module, not "+module,
		"cmd/moonwell/breaks.go: a command imports cli and nothing else of the module, not "+module+"/next",
		"cmd/moonwell/breaks_test.go: a command imports cli and nothing else of the module, not "+newTree+"script",
		"cmd/moonwell/breaks_test.go: a command imports cli and nothing else of the module, not "+newTree+"clitest",
		"cmd/other/breaks.go: a command imports cli and nothing else of the module, not "+newTree+"testkit",
	)
}

// The old tree is its internal folder, its command and its generators. A file named oracle_test.go may import
// any of it; the package oracle what is below the internal folder; no other file any of it. The root package
// of the module is no part of the old tree.
func TestTheWalkKeepsOffTheOldCommandAndTheOldGeneratorsToo(t *testing.T) {
	command, generators, tables := module+"/cmd/moonwell", module+"/tools/gen", module+"/tools/gen/slk"
	walked(t, map[string]string{
		"internal/assets/oracle_test.go": importing("assets", oldTree+"assets", command, generators, tables),
		"internal/oracle/keeps.go":       importing("oracle", oldTree+"diag", newTree+"diag"),
		"internal/cli/keeps.go":          importing("cli", module, newTree+"build"),
		"keeps.go":                       importing("next", module),
		"internal/assets/breaks.go":      importing("assets", generators),
		"internal/assets/breaks_test.go": importing("assets", command, tables),
		"internal/oracle/breaks.go":      importing("oracle", generators),
		"internal/testkit/breaks.go":     importing("testkit", oldTree+"testkit"),
		"cmd/moonwell/breaks.go":         importing("main", command),
		"breaks_test.go":                 importing("next", generators, oldTree+"diag"),
	},
		"internal/assets/breaks.go imports the old tree ("+generators+"); only oracle_test.go files may",
		"internal/assets/breaks_test.go imports the old tree ("+command+"); only oracle_test.go files may",
		"internal/assets/breaks_test.go imports the old tree ("+tables+"); only oracle_test.go files may",
		"internal/oracle/breaks.go imports the old tree ("+generators+"); only oracle_test.go files may",
		"internal/testkit/breaks.go imports the old tree ("+oldTree+"testkit); only oracle_test.go files may",
		"cmd/moonwell/breaks.go imports the old tree ("+command+"); only oracle_test.go files may",
		"breaks_test.go imports the old tree ("+generators+"); only oracle_test.go files may",
		"breaks_test.go imports the old tree ("+oldTree+"diag); only oracle_test.go files may",
	)
}

// The generator writes the data of three areas, so it may import those, the two foundations it reads names and
// text with, the root package and its parsers, and nothing else of the module. A parser imports nothing of it. A
// test there may also import its own package and the test-only packages, and a test of a parser fsx. A folder
// below tools that is neither is on no shelf, and the rule on os/exec holds below tools too.
func TestTheWalkHoldsTheGeneratorAndItsParsersToTheirShelf(t *testing.T) {
	gen, slk, ini, jass := toolsTree+"gen", toolsTree+"gen/slk", toolsTree+"gen/ini", toolsTree+"gen/jass"
	in := func(packages ...string) []string {
		var targets []string
		for _, pkg := range packages {
			targets = append(targets, newTree+pkg)
		}
		return targets
	}
	deeper, old := toolsTree+"gen/slk/deeper", module+"/tools/gen"
	const addIt = "; add it to this test and to the spec's layout"
	generator := append(in("objects", "script", "assets", "manifest", "fsx"), "os", module, slk, ini, jass)
	generatorTest := append(in("testkit", "oracle", "tooltest", "objects"), slk, "os/exec")
	notForIt := append(in("settings", "env", "diag", "war3/lua", "build", "testkit"), module+"/next", "os/exec")
	walked(t, map[string]string{
		"tools/gen/keeps.go":             importing("main", generator...),
		"tools/gen/keeps_test.go":        importing("main", generatorTest...),
		"tools/gen/oracle_test.go":       importing("main", old+"/slk", oldTree+"objects", newTree+"oracle"),
		"tools/gen/slk/keeps.go":         importing("slk", "strings", "encoding/json"),
		"tools/gen/slk/keeps_test.go":    importing("slk_test", slk, newTree+"testkit", newTree+"fsx"),
		"tools/gen/slk/oracle_test.go":   importing("slk_test", slk, old+"/slk", oldTree+"text", newTree+"oracle"),
		"tools/gen/breaks.go":            importing("main", notForIt...),
		"tools/gen/breaks_test.go":       importing("main", append(in("library", "cli"), deeper, old)...),
		"tools/gen/slk/breaks.go":        importing("slk", module, newTree+"fsx", newTree+"diag", ini, gen),
		"tools/gen/slk/breaks_test.go":   importing("slk", newTree+"objects", newTree+"diag", jass, module),
		"tools/gen/slk/deeper/breaks.go": importing("deeper", "os"),
		"tools/other/breaks.go":          importing("other", newTree+"fsx"),
		"tools/breaks_test.go":           importing("tools", newTree+"testkit", newTree+"fsx"),
	},
		"tools/gen/breaks.go: package tools/gen must not import "+newTree+"settings",
		"tools/gen/breaks.go: package tools/gen must not import "+newTree+"env",
		"tools/gen/breaks.go: package tools/gen must not import "+newTree+"diag",
		"tools/gen/breaks.go: package tools/gen must not import "+newTree+"war3/lua",
		"tools/gen/breaks.go: package tools/gen must not import "+newTree+"build",
		"tools/gen/breaks.go: package tools/gen must not import "+newTree+"testkit",
		"tools/gen/breaks.go: package tools/gen must not import "+module+"/next",
		"tools/gen/breaks.go imports os/exec; only env and test files may",
		"tools/gen/breaks_test.go: a test of package tools/gen must not import "+newTree+"library",
		"tools/gen/breaks_test.go: a test of package tools/gen must not import "+newTree+"cli",
		"tools/gen/breaks_test.go: a test of package tools/gen must not import "+deeper,
		"tools/gen/breaks_test.go imports the old tree ("+old+"); only oracle_test.go files may",
		"tools/gen/slk/breaks.go: package tools/gen/slk must not import "+module,
		"tools/gen/slk/breaks.go: package tools/gen/slk must not import "+newTree+"fsx",
		"tools/gen/slk/breaks.go: package tools/gen/slk must not import "+newTree+"diag",
		"tools/gen/slk/breaks.go: package tools/gen/slk must not import "+ini,
		"tools/gen/slk/breaks.go: package tools/gen/slk must not import "+gen,
		"tools/gen/slk/breaks_test.go: a test of package tools/gen/slk must not import "+newTree+"objects",
		"tools/gen/slk/breaks_test.go: a test of package tools/gen/slk must not import "+newTree+"diag",
		"tools/gen/slk/breaks_test.go: a test of package tools/gen/slk must not import "+jass,
		"tools/gen/slk/breaks_test.go: a test of package tools/gen/slk must not import "+module,
		"tools/gen/slk/deeper/breaks.go: package tools/gen/slk/deeper is on no shelf"+addIt,
		"tools/other/breaks.go: package tools/other is on no shelf"+addIt,
		"tools/other/breaks.go: package tools/other must not import "+newTree+"fsx",
		"tools/breaks_test.go: package tools is on no shelf"+addIt,
		"tools/breaks_test.go: a test of package tools must not import "+newTree+"fsx",
	)
}

// The program is not built from the generator: no file of it imports a package below tools, a test and a file
// that compares the two trees neither, and no test-only package does.
func TestTheWalkKeepsEveryFileThatIsNotBelowToolsOffTheGenerator(t *testing.T) {
	gen, slk := toolsTree+"gen", toolsTree+"gen/slk"
	const onlyThey = "; only the generator and its parsers import a package below tools"
	walked(t, map[string]string{
		"tools/gen/keeps.go":              importing("main", slk),
		"internal/objects/breaks.go":      importing("objects", slk, newTree+"manifest"),
		"internal/objects/breaks_test.go": importing("objects", gen),
		"internal/objects/oracle_test.go": importing("objects", slk, module+"/tools/gen/slk"),
		"internal/testkit/breaks.go":      importing("testkit", slk),
		"internal/cli/breaks.go":          importing("cli", gen, newTree+"build"),
		"cmd/moonwell/breaks.go":          importing("main", gen, newTree+"cli"),
		"breaks_test.go":                  importing("next", slk),
	},
		"internal/objects/breaks.go imports "+slk+onlyThey,
		"internal/objects/breaks_test.go imports "+gen+onlyThey,
		"internal/objects/oracle_test.go imports "+slk+onlyThey,
		"internal/testkit/breaks.go imports "+slk+onlyThey,
		"internal/cli/breaks.go imports "+gen+onlyThey,
		"cmd/moonwell/breaks.go imports "+gen+onlyThey,
		"breaks_test.go imports "+slk+onlyThey,
	)
}

// A program is started and the network is reached through an env.Env, which a test replaces: so only env
// imports the two packages that do it, and the files that only tests are built from. fsx is excused for the one
// it imports (excused says why), and not for the other.
func TestTheWalkLetsOnlyEnvAndTestsReachOutsideTheProgram(t *testing.T) {
	walked(t, map[string]string{
		"internal/env/keeps.go":          importing("env", "os/exec", "net/http"),
		"internal/library/keeps_test.go": importing("library", "net/http", "os/exec"),
		"internal/testkit/keeps.go":      importing("testkit", "os/exec"),
		"internal/tooltest/keeps.go":     importing("tooltest", "net/http"),
		"internal/oracle/keeps.go":       importing("oracle", "os/exec"),
		"internal/fsx/keeps.go":          importing("fsx", "os/exec", "os"),
		"internal/script/keeps.go":       importing("script", "os", "net/url", "net/http/httptest"),
		"cmd/moonwell/main_test.go":      importing("main", "os/exec"),
		"internal/script/breaks.go":      importing("script", "os/exec"),
		"internal/library/breaks.go":     importing("library", "net/http"),
		"internal/fsx/breaks.go":         importing("fsx", "net/http"),
		"internal/build/breaks.go":       importing("build", "os/exec", "net/http"),
		"internal/cli/breaks.go":         importing("cli", "os/exec"),
		"internal/war3/mpq/breaks.go":    importing("mpq", "net/http"),
		"cmd/moonwell/breaks.go":         importing("main", "os/exec"),
		"breaks.go":                      importing("next", "net/http"),
	},
		"internal/script/breaks.go imports os/exec; only env and test files may",
		"internal/library/breaks.go imports net/http; only env and test files may",
		"internal/fsx/breaks.go imports net/http; only env and test files may",
		"internal/build/breaks.go imports os/exec; only env and test files may",
		"internal/build/breaks.go imports net/http; only env and test files may",
		"internal/cli/breaks.go imports os/exec; only env and test files may",
		"internal/war3/mpq/breaks.go imports net/http; only env and test files may",
		"cmd/moonwell/breaks.go imports os/exec; only env and test files may",
		"breaks.go imports net/http; only env and test files may",
	)
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
		// build imports the areas and what is below them, and cli imports build too. Neither imports the other
		// way, a test-only package outside a test, or a package that is on no shelf; and nothing below imports
		// either.
		{"build", "script", false, true},
		{"build", "editor", false, true},
		{"build", "mapdir", false, true},
		{"build", "war3/mpq", false, true},
		{"build", "cli", false, false},
		{"build", "cli", true, false},
		{"build", "testkit", false, false},
		{"build", "testkit", true, true},
		{"build", "build", true, true},
		{"build", "watch", false, false},
		{"cli", "build", false, true},
		{"cli", "library", false, true},
		{"cli", "env", false, true},
		{"cli", "war3/lua", false, true},
		{"cli", "oracle", false, false},
		{"cli", "oracle", true, true},
		{"cli", "cli", true, true},
		{"cli", "watch", false, false},
		{"cli", "watch", true, false},
		{"script", "build", false, false},
		{"toolchain", "cli", false, false},
		{"manifest", "build", true, false},
		// A test-only package imports neither, in its tests neither; it may still import an area.
		{"testkit", "build", false, false},
		{"testkit", "cli", false, false},
		{"oracle", "build", false, false},
		{"oracle", "cli", true, false},
		{"tooltest", "build", false, false},
		{"tooltest", "cli", false, false},
		{"oracle", "script", false, true},
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
