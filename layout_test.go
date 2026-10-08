package moonwell

import (
	"errors"
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
	module    = "github.com/mdlsvensson/moonwell"
	shelves   = module + "/internal/"
	toolsTree = module + "/tools/"
)

var (
	scanned     = []string{"internal", "cmd", "tools"}
	foundations = map[string][]string{
		"diag":     nil,
		"binio":    nil,
		"fsx":      {"diag"},
		"env":      {"diag", "fsx"},
		"mapdir":   {"diag", "fsx"},
		"manifest": {"env", "diag", "fsx"},
	}
	areas        = []string{"objects", "settings", "assets", "script", "library", "editor", "toolchain"}
	belowFormats = []string{"diag", "fsx", "binio"}
	testOnly     = []string{"testkit", "tooltest"}
	outsideWorld = []string{"os/exec", "net/http"}
	excused      = map[string][]string{
		"fsx": {"os/exec"},
	}
	commandLine = []string{
		"github.com/spf13/cobra", "github.com/spf13/pflag", "github.com/inconshreveable/mousetrap",
	}
	generatorMay   = []string{"objects", "script", "assets", "manifest", "fsx"}
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

func allowed(from, to string) bool {
	below := isFormat(to) || isFoundation(to)
	switch {
	case from == "testkit" && (slices.Contains(areas, to) || to == "tooltest"):
		return false
	case slices.Contains(testOnly, from) && (to == "build" || to == "cli"):
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

func allowedInATest(from, to string) bool {
	return from == to || slices.Contains(testOnly, to) || allowed(from, to)
}

type goFile struct {
	path    string
	pkg     string
	shelved bool
	tool    string
	tooled  bool
	isTest  bool
}

func fileAt(file string) goFile {
	pkg, shelved := packageBelow(file, "internal")
	tool, tooled := packageBelow(file, "tools")
	return goFile{file, pkg, shelved, tool, tooled, strings.HasSuffix(file, "_test.go")}
}

func packageBelow(file, top string) (string, bool) {
	pkg, is := strings.CutPrefix(path.Dir(file)+"/", top+"/")
	if !is {
		return "", false
	}
	return strings.TrimSuffix(pkg, "/"), true
}

func isParser(tool string) bool { return path.Dir(tool) == "gen" }

var rules = []func(f goFile, target string) string{
	insideTheProgram, lineReadByCli, cliAlone, downTheShelves, offTheTools, amongTheTools,
}

func lineReadByCli(f goFile, target string) string {
	reads := slices.ContainsFunc(commandLine, func(m string) bool { return target == m || strings.HasPrefix(target, m+"/") })
	if !reads || f.shelved && f.pkg == "cli" {
		return ""
	}
	return fmt.Sprintf("%s imports %s; only cli reads the command line", f.path, target)
}

func insideTheProgram(f goFile, target string) string {
	let := f.isTest || f.pkg == "env" || slices.Contains(testOnly, f.pkg) || slices.Contains(excused[f.pkg], target)
	if let || !slices.Contains(outsideWorld, target) {
		return ""
	}
	return fmt.Sprintf("%s imports %s; only env and test files may", f.path, target)
}

func cliAlone(f goFile, target string) string {
	inModule := target == module || strings.HasPrefix(target, module+"/")
	to, onShelves := strings.CutPrefix(target, shelves)
	switch {
	case !strings.HasPrefix(f.path, "cmd/") || !inModule || strings.HasPrefix(target, toolsTree):
	case onShelves && to == "cli":
	case onShelves && f.isTest && slices.Contains(testOnly, to):
	default:
		return fmt.Sprintf("%s: a command imports cli and nothing else of the module, not %s", f.path, target)
	}
	return ""
}

func downTheShelves(f goFile, target string) string {
	to, onShelves := strings.CutPrefix(target, shelves)
	switch {
	case !onShelves || !f.shelved:
	case f.isTest && !allowedInATest(f.pkg, to):
		return fmt.Sprintf("%s: a test of package %s must not import %s", f.path, f.pkg, to)
	case !f.isTest && !allowed(f.pkg, to):
		return fmt.Sprintf("%s: package %s must not import %s", f.path, f.pkg, to)
	}
	return ""
}

func offTheTools(f goFile, target string) string {
	if f.tooled || !strings.HasPrefix(target, toolsTree) {
		return ""
	}
	return fmt.Sprintf("%s imports %s; only the generator and its parsers import a package below tools", f.path, target)
}

func amongTheTools(f goFile, target string) string {
	inModule := target == module || strings.HasPrefix(target, module+"/")
	switch {
	case !f.tooled || !inModule || toolMay(f, target):
		return ""
	case f.isTest:
		return fmt.Sprintf("%s: a test of package %s must not import %s", f.path, path.Join("tools", f.tool), target)
	}
	return fmt.Sprintf("%s: package %s must not import %s", f.path, path.Join("tools", f.tool), target)
}

func toolMay(f goFile, target string) bool {
	to, onShelves := strings.CutPrefix(target, shelves)
	below := func(packages []string) bool { return onShelves && slices.Contains(packages, to) }
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

func (f goFile) broken(imports []string) []string {
	var reports []string
	const onNoShelf = "%s: package %s is on no shelf; add it to this test and to ARCHITECTURE.md"
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

func walkShelves(root string, report func(format string, args ...any)) error {
	for _, folder := range scanned {
		top := filepath.Join(root, folder)
		if _, err := os.Stat(top); errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err := walkFolder(root, top, report); err != nil {
			return err
		}
	}
	return nil
}

func walkFolder(root, top string, report func(format string, args ...any)) error {
	return filepath.WalkDir(top, func(file string, entry fs.DirEntry, err error) error {
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

func TestImportsOnlyGoDownTheShelves(t *testing.T) {
	for _, folder := range scanned {
		if _, err := os.Stat(folder); err != nil {
			t.Fatalf("the folder %s of the module is not read: %v", folder, err)
		}
	}
	if err := walkShelves(".", t.Errorf); err != nil {
		t.Fatal(err)
	}
}

func importing(pkg string, targets ...string) string {
	source := "package " + pkg + "\n"
	for _, target := range targets {
		source += "\nimport _ " + strconv.Quote(target)
	}
	return source + "\n"
}

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

func TestTheWalkLetsOnlyCliReadTheCommandLine(t *testing.T) {
	const cobra, pflag = "github.com/spf13/cobra", "github.com/spf13/pflag"
	walked(t, map[string]string{
		"internal/cli/cli.go":        importing("cli", cobra, pflag),
		"internal/cli/cli_test.go":   importing("cli", cobra),
		"internal/build/build.go":    importing("build", pflag),
		"internal/env/env_test.go":   importing("env", cobra+"/doc"),
		"cmd/moonwell/main.go":       importing("main", cobra),
		"tools/gen/main.go":          importing("main", pflag),
		"internal/script/cobras.go":  importing("script", "github.com/spf13/cobrasnake"),
		"internal/testkit/lines.go":  importing("testkit", cobra),
		"internal/war3/mpq/flags.go": importing("mpq", pflag),
		"internal/fsx/explorer.go":   importing("fsx", "github.com/inconshreveable/mousetrap"),
	},
		"internal/build/build.go imports "+pflag+"; only cli reads the command line",
		"internal/env/env_test.go imports "+cobra+"/doc; only cli reads the command line",
		"cmd/moonwell/main.go imports "+cobra+"; only cli reads the command line",
		"tools/gen/main.go imports "+pflag+"; only cli reads the command line",
		"internal/testkit/lines.go imports "+cobra+"; only cli reads the command line",
		"internal/war3/mpq/flags.go imports "+pflag+"; only cli reads the command line",
		"internal/fsx/explorer.go imports github.com/inconshreveable/mousetrap; only cli reads the command line",
	)
}

func TestTheWalkReportsAFileThatBreaksARuleATestFileToo(t *testing.T) {
	walked(t, map[string]string{
		"internal/assets/breaks_test.go": importing("assets", shelves+"mapdir", shelves+"settings"),
		"internal/mapdir/breaks.go":      importing("mapdir", shelves+"fsx", shelves+"assets"),
		"internal/assets/keeps_test.go":  importing("assets", shelves+"assets", shelves+"testkit", shelves+"war3/imp"),
		"internal/editor/keeps.go":       importing("editor", shelves+"objects", shelves+"script", shelves+"mapdir"),
		"internal/mapdir/keeps_test.go":  importing("mapdir", shelves+"testkit", shelves+"diag"),
	},
		"internal/assets/breaks_test.go: a test of package assets must not import settings",
		"internal/mapdir/breaks.go: package mapdir must not import assets",
	)
}

func TestTheWalkHoldsBuildAndCliToTheirShelves(t *testing.T) {
	in := func(packages ...string) []string {
		targets := []string{module}
		for _, pkg := range packages {
			targets = append(targets, shelves+pkg)
		}
		return targets
	}
	walked(t, map[string]string{
		"internal/build/keeps.go":       importing("build", in("script", "editor", "mapdir", "env", "war3/mpq")...),
		"internal/build/keeps_test.go":  importing("build", in("build", "testkit", "tooltest", "script")...),
		"internal/build/breaks.go":      importing("build", in("cli", "testkit", "watch")...),
		"internal/build/breaks_test.go": importing("build", in("cli")...),
		"internal/cli/keeps.go":         importing("cli", in("build", "toolchain", "manifest", "diag", "war3/lua")...),
		"internal/cli/keeps_test.go":    importing("cli", in("cli", "build", "testkit", "tooltest")...),
		"internal/cli/breaks.go":        importing("cli", in("tooltest", "watch")...),
		"internal/cli/breaks_test.go":   importing("cli", in("watch")...),
		"internal/script/breaks.go":     importing("script", in("build", "cli")...),
		"internal/env/breaks_test.go":   importing("env", in("build")...),
	},
		"internal/build/breaks.go: package build must not import cli",
		"internal/build/breaks.go: package build must not import testkit",
		"internal/build/breaks.go: package build must not import watch",
		"internal/build/breaks_test.go: a test of package build must not import cli",
		"internal/cli/breaks.go: package cli must not import tooltest",
		"internal/cli/breaks.go: package cli must not import watch",
		"internal/cli/breaks_test.go: a test of package cli must not import watch",
		"internal/script/breaks.go: package script must not import build",
		"internal/script/breaks.go: package script must not import cli",
		"internal/env/breaks_test.go: a test of package env must not import build",
	)
}

func TestTheWalkHoldsTheTestOnlyPackagesOffBuildAndCli(t *testing.T) {
	walked(t, map[string]string{
		"internal/testkit/keeps.go":        importing("testkit", shelves+"env", shelves+"war3/mpq"),
		"internal/tooltest/keeps.go":       importing("tooltest", shelves+"toolchain", shelves+"testkit"),
		"internal/testkit/breaks.go":       importing("testkit", shelves+"build", shelves+"cli"),
		"internal/tooltest/breaks.go":      importing("tooltest", shelves+"build", shelves+"cli"),
		"internal/tooltest/breaks_test.go": importing("tooltest", shelves+"build", shelves+"tooltest"),
	},
		"internal/testkit/breaks.go: package testkit must not import build",
		"internal/testkit/breaks.go: package testkit must not import cli",
		"internal/tooltest/breaks_test.go: a test of package tooltest must not import build",
		"internal/tooltest/breaks.go: package tooltest must not import build",
		"internal/tooltest/breaks.go: package tooltest must not import cli",
	)
}

func TestTheWalkLetsACommandImportCliAlone(t *testing.T) {
	walked(t, map[string]string{
		"cmd/moonwell/main.go":        importing("main", "os", shelves+"cli"),
		"cmd/moonwell/main_test.go":   importing("main", "os", shelves+"cli", shelves+"testkit", shelves+"tooltest"),
		"cmd/moonwell/breaks.go":      importing("main", shelves+"build", shelves+"env", module, module+"/cmd/other"),
		"cmd/moonwell/breaks_test.go": importing("main", shelves+"script", shelves+"clitest"),
		"cmd/other/breaks.go":         importing("main", shelves+"testkit"),
	},
		"cmd/moonwell/breaks.go: a command imports cli and nothing else of the module, not "+shelves+"build",
		"cmd/moonwell/breaks.go: a command imports cli and nothing else of the module, not "+shelves+"env",
		"cmd/moonwell/breaks.go: a command imports cli and nothing else of the module, not "+module,
		"cmd/moonwell/breaks.go: a command imports cli and nothing else of the module, not "+module+"/cmd/other",
		"cmd/moonwell/breaks_test.go: a command imports cli and nothing else of the module, not "+shelves+"script",
		"cmd/moonwell/breaks_test.go: a command imports cli and nothing else of the module, not "+shelves+"clitest",
		"cmd/other/breaks.go: a command imports cli and nothing else of the module, not "+shelves+"testkit",
	)
}

func TestTheWalkReadsInternalCmdAndToolsAndNothingBesideThem(t *testing.T) {
	walked(t, map[string]string{
		"internal/script/breaks.go": importing("script", "os/exec"),
		"cmd/moonwell/breaks.go":    importing("main", "net/http"),
		"tools/gen/breaks.go":       importing("main", "os/exec"),
		"unread.go":                 importing("moonwell", "net/http", shelves+"cli"),
		"template/unread.go":        importing("template", "os/exec", toolsTree+"gen"),
	},
		"internal/script/breaks.go imports os/exec; only env and test files may",
		"cmd/moonwell/breaks.go imports net/http; only env and test files may",
		"tools/gen/breaks.go imports os/exec; only env and test files may",
	)
}

func TestTheWalkHoldsTheGeneratorAndItsParsersToTheirShelf(t *testing.T) {
	gen, slk, ini, jass := toolsTree+"gen", toolsTree+"gen/slk", toolsTree+"gen/ini", toolsTree+"gen/jass"
	in := func(packages ...string) []string {
		var targets []string
		for _, pkg := range packages {
			targets = append(targets, shelves+pkg)
		}
		return targets
	}
	deeper, command := toolsTree+"gen/slk/deeper", module+"/cmd/moonwell"
	const addIt = "; add it to this test and to ARCHITECTURE.md"
	generator := append(in("objects", "script", "assets", "manifest", "fsx"), "os", module, slk, ini, jass)
	generatorTest := append(in("testkit", "tooltest", "objects"), slk, "os/exec")
	notForIt := append(in("settings", "env", "diag", "war3/lua", "build", "testkit"), command, "os/exec")
	walked(t, map[string]string{
		"tools/gen/keeps.go":             importing("main", generator...),
		"tools/gen/keeps_test.go":        importing("main", generatorTest...),
		"tools/gen/slk/keeps.go":         importing("slk", "strings", "encoding/json"),
		"tools/gen/slk/keeps_test.go":    importing("slk_test", slk, shelves+"testkit", shelves+"fsx"),
		"tools/gen/breaks.go":            importing("main", notForIt...),
		"tools/gen/breaks_test.go":       importing("main", append(in("library", "cli"), deeper)...),
		"tools/gen/slk/breaks.go":        importing("slk", module, shelves+"fsx", shelves+"diag", ini, gen),
		"tools/gen/slk/breaks_test.go":   importing("slk", shelves+"objects", shelves+"diag", jass, module),
		"tools/gen/slk/deeper/breaks.go": importing("deeper", "os"),
		"tools/other/breaks.go":          importing("other", shelves+"fsx"),
		"tools/breaks_test.go":           importing("tools", shelves+"testkit", shelves+"fsx"),
	},
		"tools/gen/breaks.go: package tools/gen must not import "+shelves+"settings",
		"tools/gen/breaks.go: package tools/gen must not import "+shelves+"env",
		"tools/gen/breaks.go: package tools/gen must not import "+shelves+"diag",
		"tools/gen/breaks.go: package tools/gen must not import "+shelves+"war3/lua",
		"tools/gen/breaks.go: package tools/gen must not import "+shelves+"build",
		"tools/gen/breaks.go: package tools/gen must not import "+shelves+"testkit",
		"tools/gen/breaks.go: package tools/gen must not import "+command,
		"tools/gen/breaks.go imports os/exec; only env and test files may",
		"tools/gen/breaks_test.go: a test of package tools/gen must not import "+shelves+"library",
		"tools/gen/breaks_test.go: a test of package tools/gen must not import "+shelves+"cli",
		"tools/gen/breaks_test.go: a test of package tools/gen must not import "+deeper,
		"tools/gen/slk/breaks.go: package tools/gen/slk must not import "+module,
		"tools/gen/slk/breaks.go: package tools/gen/slk must not import "+shelves+"fsx",
		"tools/gen/slk/breaks.go: package tools/gen/slk must not import "+shelves+"diag",
		"tools/gen/slk/breaks.go: package tools/gen/slk must not import "+ini,
		"tools/gen/slk/breaks.go: package tools/gen/slk must not import "+gen,
		"tools/gen/slk/breaks_test.go: a test of package tools/gen/slk must not import "+shelves+"objects",
		"tools/gen/slk/breaks_test.go: a test of package tools/gen/slk must not import "+shelves+"diag",
		"tools/gen/slk/breaks_test.go: a test of package tools/gen/slk must not import "+jass,
		"tools/gen/slk/breaks_test.go: a test of package tools/gen/slk must not import "+module,
		"tools/gen/slk/deeper/breaks.go: package tools/gen/slk/deeper is on no shelf"+addIt,
		"tools/other/breaks.go: package tools/other is on no shelf"+addIt,
		"tools/other/breaks.go: package tools/other must not import "+shelves+"fsx",
		"tools/breaks_test.go: package tools is on no shelf"+addIt,
		"tools/breaks_test.go: a test of package tools must not import "+shelves+"fsx",
	)
}

func TestTheWalkKeepsEveryFileThatIsNotBelowToolsOffTheGenerator(t *testing.T) {
	gen, slk := toolsTree+"gen", toolsTree+"gen/slk"
	const onlyThey = "; only the generator and its parsers import a package below tools"
	walked(t, map[string]string{
		"tools/gen/keeps.go":              importing("main", slk),
		"internal/objects/breaks.go":      importing("objects", slk, shelves+"manifest"),
		"internal/objects/breaks_test.go": importing("objects", gen),
		"internal/testkit/breaks.go":      importing("testkit", slk),
		"internal/cli/breaks.go":          importing("cli", gen, shelves+"build"),
		"cmd/moonwell/breaks.go":          importing("main", gen, shelves+"cli"),
	},
		"internal/objects/breaks.go imports "+slk+onlyThey,
		"internal/objects/breaks_test.go imports "+gen+onlyThey,
		"internal/testkit/breaks.go imports "+slk+onlyThey,
		"internal/cli/breaks.go imports "+gen+onlyThey,
		"cmd/moonwell/breaks.go imports "+gen+onlyThey,
	)
}

func TestTheWalkLetsOnlyEnvAndTestsReachOutsideTheProgram(t *testing.T) {
	walked(t, map[string]string{
		"internal/env/keeps.go":          importing("env", "os/exec", "net/http"),
		"internal/library/keeps_test.go": importing("library", "net/http", "os/exec"),
		"internal/testkit/keeps.go":      importing("testkit", "os/exec"),
		"internal/tooltest/keeps.go":     importing("tooltest", "net/http"),
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
	},
		"internal/script/breaks.go imports os/exec; only env and test files may",
		"internal/library/breaks.go imports net/http; only env and test files may",
		"internal/fsx/breaks.go imports net/http; only env and test files may",
		"internal/build/breaks.go imports os/exec; only env and test files may",
		"internal/build/breaks.go imports net/http; only env and test files may",
		"internal/cli/breaks.go imports os/exec; only env and test files may",
		"internal/war3/mpq/breaks.go imports net/http; only env and test files may",
		"cmd/moonwell/breaks.go imports os/exec; only env and test files may",
	)
}

func TestTheRulesOfTheShelvesForTestOnlyPackagesAndForTestFiles(t *testing.T) {
	cases := []struct {
		from, to string
		inATest  bool
		want     bool
	}{
		{"tooltest", "toolchain", false, true},
		{"tooltest", "testkit", false, true},
		{"toolchain", "tooltest", false, false},
		{"script", "tooltest", false, false},
		{"testkit", "tooltest", false, false},
		{"testkit", "toolchain", false, false},
		{"testkit", "env", false, true},
		{"script", "toolchain", false, false},
		{"script", "toolchain", true, false},
		{"assets", "settings", true, false},
		{"editor", "objects", false, true},
		{"editor", "objects", true, true},
		{"editor", "script", true, true},
		{"editor", "assets", true, false},
		{"assets", "build", true, false},
		{"assets", "cli", true, false},
		{"mapdir", "assets", true, false},
		{"mapdir", "env", true, false},
		{"war3/imp", "assets", true, false},
		{"war3/imp", "mapdir", true, false},
		{"war3/imp", "war3/lua", true, true},
		{"script", "tooltest", true, true},
		{"script", "testkit", true, true},
		{"assets", "assets", true, true},
		{"assets", "mapdir", true, true},
		{"assets", "war3/imp", true, true},
		{"mapdir", "testkit", true, true},
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
		{"cli", "tooltest", false, false},
		{"cli", "tooltest", true, true},
		{"cli", "cli", true, true},
		{"cli", "watch", false, false},
		{"cli", "watch", true, false},
		{"script", "build", false, false},
		{"toolchain", "cli", false, false},
		{"manifest", "build", true, false},
		{"testkit", "build", false, false},
		{"testkit", "cli", false, false},
		{"tooltest", "build", false, false},
		{"tooltest", "build", true, false},
		{"tooltest", "cli", false, false},
		{"tooltest", "cli", true, false},
		{"tooltest", "script", false, true},
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
