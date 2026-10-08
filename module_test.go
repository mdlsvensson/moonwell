package moonwell

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

var dependencies = []string{
	"github.com/inconshreveable/mousetrap",
	"github.com/spf13/cobra",
	"github.com/spf13/pflag",
}

func TestTheModuleDependsOnCobraAloneAndUsesNoCgo(t *testing.T) {
	mod, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	licences, err := os.ReadFile("THIRD_PARTY_LICENSES")
	if err != nil {
		t.Fatal(err)
	}
	var required []string
	for _, line := range strings.Split(string(mod), "\n") {
		words := strings.Fields(line)
		if len(words) > 0 && words[0] == "require" {
			words = words[1:]
		}
		switch {
		case len(words) == 0 || words[0] == "(" || words[0] == ")" || words[0] == "module" || words[0] == "go":
		case len(words) >= 2 && strings.HasPrefix(words[1], "v"):
			required = append(required, words[0])
			if named := words[0] + " " + words[1]; !strings.Contains(string(licences), "\n"+named+"\n") {
				t.Errorf("THIRD_PARTY_LICENSES has no line %q: it names each module at the version go.mod requires", named)
			}
		default:
			t.Errorf("go.mod has %q: Moonwell's go.mod names the module, the Go version and what it requires",
				strings.TrimSpace(line))
		}
	}
	slices.Sort(required)
	if !slices.Equal(required, dependencies) {
		t.Errorf("go.mod requires %q, want %q and no other module", required, dependencies)
	}
	err = filepath.WalkDir(".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && (strings.HasPrefix(entry.Name(), ".") && path != "." || entry.Name() == "testdata") {
			return filepath.SkipDir
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range file.Imports {
			name := strings.Trim(spec.Path.Value, `"`)
			first, _, _ := strings.Cut(name, "/")
			ofADependency := slices.ContainsFunc(dependencies, func(dependency string) bool {
				return name == dependency || strings.HasPrefix(name, dependency+"/")
			})
			if name == "C" {
				t.Errorf("%s imports \"C\": Moonwell uses no cgo", path)
			} else if strings.Contains(first, ".") && !strings.HasPrefix(name, "github.com/mdlsvensson/moonwell") &&
				!ofADependency {
				t.Errorf("%s imports %s: Moonwell depends on %q and on no other module", path, name, dependencies)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestVersionEqualsThePklPackageVersion(t *testing.T) {
	project, err := os.ReadFile("schema/PklProject")
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`(?m)^\s*version = "([^"]+)"`).FindSubmatch(project)
	if match == nil {
		t.Fatal("schema/PklProject has no version")
	}
	if string(match[1]) != Version {
		t.Errorf("schema/PklProject has version %s, version.go has %s", match[1], Version)
	}
}

func TestTemplateFilesAreTheProjectInitCopies(t *testing.T) {
	files, err := TemplateFiles()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, file := range files {
		got = append(got, file.Path)
	}
	want := []string{
		".gitattributes",
		".gitignore",
		".luarc.json",
		".vscode/extensions.json",
		"assets/ReplaceableTextures/CommandButtons/.gitkeep",
		"assets/ReplaceableTextures/CommandButtonsDisabled/.gitkeep",
		"assets/ReplaceableTextures/PassiveButtons/.gitkeep",
		"lua/.gitkeep",
		"maps/map.w3x/conversation.json",
		"maps/map.w3x/war3map.doo",
		"maps/map.w3x/war3map.lua",
		"maps/map.w3x/war3map.mmp",
		"maps/map.w3x/war3map.shd",
		"maps/map.w3x/war3map.w3c",
		"maps/map.w3x/war3map.w3e",
		"maps/map.w3x/war3map.w3grp",
		"maps/map.w3x/war3map.w3i",
		"maps/map.w3x/war3map.w3l",
		"maps/map.w3x/war3map.w3r",
		"maps/map.w3x/war3map.wct",
		"maps/map.w3x/war3map.wpm",
		"maps/map.w3x/war3map.wtg",
		"maps/map.w3x/war3map.wts",
		"maps/map.w3x/war3mapMap.blp",
		"maps/map.w3x/war3mapUnits.doo",
		"moonwell.pkl",
		"objects/units.pkl",
		"src/generated/objects.yue",
		"src/main.yue",
		"yueconfig.yue",
	}
	if !slices.Equal(got, want) {
		t.Errorf("the template's files changed.\ngot:  %q\nwant: %q\n"+
			"Update this list if the change belongs in the template; otherwise remove the stray files.", got, want)
	}
	for _, file := range files {
		if len(file.Data) == 0 && !strings.HasSuffix(file.Path, ".gitkeep") {
			t.Errorf("template/%s is empty", file.Path)
		}
	}
}

func TestTemplateFilesSkipWhatAProjectGenerates(t *testing.T) {
	for path, want := range map[string]bool{
		".moonwell/types/x.d.lua": true,
		"src/main.lua":            true,
		"dist/a":                  true,
		"src/main.yue":            false,
		"lua/greeter.lua":         false,
	} {
		if got := isGenerated(path); got != want {
			t.Errorf("isGenerated(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestTheTemplatesGitattributesNeverConvertsMapsOrAssets(t *testing.T) {
	files, err := TemplateFiles()
	if err != nil {
		t.Fatal(err)
	}
	index := slices.IndexFunc(files, func(file TemplateFile) bool { return file.Path == ".gitattributes" })
	if index < 0 {
		t.Fatal("template/.gitattributes is missing")
	}
	lines := strings.Split(string(files[index].Data), "\n")
	for _, line := range []string{"maps/** binary", "assets/** -text"} {
		if !slices.Contains(lines, line) {
			t.Errorf("template/.gitattributes lacks %q", line)
		}
	}
}

func TestEmbeddedDataIsPresent(t *testing.T) {
	if !strings.HasPrefix(GamePaths, "# Warcraft III ") {
		t.Error("data/game-paths.txt does not start with its version header")
	}
	if !strings.Contains(RuntimeLua, "__mw") || !strings.Contains(MacrosYue, "FourCC") {
		t.Error("the runtime or the macro module is not embedded")
	}
	if len(Metadata) == 0 || len(Natives) == 0 {
		t.Error("the metadata or the natives are not embedded")
	}
}

func TestTheREADMENamesThisVersion(t *testing.T) {
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{
		"releases/download/moonwell@" + Version + "/install.ps1",
		"becomes `moonwell@" + Version + "`",
	} {
		if !strings.Contains(string(readme), text) {
			t.Errorf("README.md lacks %q: update the version it shows", text)
		}
	}
}
