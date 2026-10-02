package cli_test

import (
	"bytes"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/assets"
	"github.com/mdlsvensson/moonwell/internal/cli"
	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/pipeline"
	"github.com/mdlsvensson/moonwell/internal/project"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func builtScript(t *testing.T, root string) string {
	t.Helper()
	ok(t, root, "build")
	data, found := packed(t, archive(t, root), "war3map.lua")
	if !found {
		t.Fatal("no Lua in archive")
	}
	return string(data)
}

func TestE2EInitBuildInjectedArchive(t *testing.T) {
	root := compiling(t)
	r := ok(t, root, "build")
	contains(t, r.output, "Built dist/bin/map.w3x")
	opened := archive(t, root)
	lua, found := packed(t, opened, "war3map.lua")
	if !found {
		t.Fatal("Lua absent")
	}
	contains(t, string(lua), "function main()", `__mw.define("main", function(...)`, `__mw.boot("main")`, "1751543663")
	if _, found = packed(t, opened, "war3map.w3i"); !found {
		t.Fatal("map info absent")
	}
	list, found := packed(t, opened, "(listfile)")
	if !found {
		t.Fatal("listfile absent")
	}
	contains(t, string(list), "war3map.lua")
}

func TestE2ECheckWritesEditorDeclarations(t *testing.T) {
	root := compiling(t)
	ok(t, root, "check")
	contains(t, read(t, root, ".moonwell/types/natives.d.lua"), "function CreateUnit(")
	contains(t, read(t, root, ".moonwell/types/objects.d.lua"), "---@field captain integer h000")
	contains(t, read(t, root, ".moonwell/types/map.d.lua"), "---@type unit\ngg_unit_Hblm_0003 = nil")
	contains(t, read(t, root, ".moonwell/types/moonwell.d.lua"), "function moonwell.on_main(fn) end", "function require(name) end")
	contains(t, read(t, root, ".moonwell/yue/moonwell/macros.yue"), "export macro FourCC")
}

func TestE2ECheckFourCCSourcePosition(t *testing.T) {
	root := compiling(t)
	edit(t, root, "src/main.yue", `$FourCC("hfoo")`, `$FourCC("hfo")`)
	fails(t, root, []string{`error: src/main.yue:13 › $FourCC needs a string literal of exactly 4 characters, such as "hfoo".`}, "check")
}

func TestE2EBuildImportsAssets(t *testing.T) {
	root := compiling(t)
	data := []byte{0, 1, 2, 250, 255}
	testkit.WriteFile(t, root, "assets/Models/unit.mdx", data)
	ok(t, root, "build")
	opened := archive(t, root)
	got, found := packed(t, opened, `Models\unit.mdx`)
	if !found || !bytes.Equal(got, data) {
		t.Fatal(got)
	}
	imp, _ := packed(t, opened, "war3map.imp")
	imports, err := assets.ReadImports(imp, "war3map.imp")
	if err != nil || len(imports) != 1 || imports[0].Flag != 13 || imports[0].Path != `Models\unit.mdx` {
		t.Fatalf("%v %+v", err, imports)
	}
	if exists(root, ".asset-state") || exists(root, "maps/map.w3x/Models") {
		t.Fatal("build touched source")
	}
}

func TestE2ECheckReportsAssetProblem(t *testing.T) {
	root := compiling(t)
	edit(t, root, "moonwell.pkl", "paths {}", `paths { ["missing.blp"] = "x.blp" }`)
	fails(t, root, []string{"does not exist"}, "check")
}

func TestE2EFailedBuildDeletesPreviousArchive(t *testing.T) {
	root := compiling(t)
	ok(t, root, "build")
	if !exists(root, "dist/bin/map.w3x") {
		t.Fatal("archive absent")
	}
	write(t, root, "src/main.yue", "x = \n  if then\n")
	fails(t, root, []string{"error: src/main.yue:"}, "build")
	if exists(root, "dist/bin/map.w3x") {
		t.Fatal("stale archive survived")
	}
}

func stageAndLaunch(t *testing.T, root, body string) string {
	t.Helper()
	exe := filepath.Join(root, "Warcraft III.exe")
	write(t, root, "Warcraft III.exe", "")
	writeLocal(t, root, "launch { gameExecutable = #\""+exe+"\"# }\n"+body)
	env, _ := newEnv(root)
	var calls []struct {
		exe  string
		args []string
	}
	env.Spawn = func(command string, args []string) error {
		calls = append(calls, struct {
			exe  string
			args []string
		}{command, args})
		return nil
	}
	if err := cli.Test(background, env, pipeline.StageOptions{}); err != nil {
		t.Fatal(err)
	}
	staged := filepath.Join(root, "dist", "stage", "map.w3x")
	if len(calls) != 1 || calls[0].exe != exe || !reflect.DeepEqual(calls[0].args, []string{"-launch", "-windowmode", "windowed", "-loadfile", staged}) {
		t.Fatal(calls)
	}
	contains(t, read(t, staged, "war3map.lua"), `__mw.boot("main")`)
	return staged
}

func TestE2ETestStagesAndLaunches(t *testing.T) { stageAndLaunch(t, compiling(t), "") }

func TestE2ESetupLocalManifestCreatesAndKeeps(t *testing.T) {
	root := compiling(t)
	remove(t, root, "moonwell.local.pkl")
	ok(t, root, "setup")
	if read(t, root, "moonwell.local.pkl") != project.LocalPkl() {
		t.Fatal("wrong local manifest")
	}
	mine := "amends \"moonwell.pkl\"\nlaunch { gameExecutable = \"/games/wc3.exe\" }\n"
	write(t, root, "moonwell.local.pkl", mine)
	ok(t, root, "setup")
	if read(t, root, "moonwell.local.pkl") != mine {
		t.Fatal("overwritten")
	}
}

func TestE2ESetupUpgradesEditorFilesAndKeepsExisting(t *testing.T) {
	root := compiling(t)
	for _, path := range []string{"yueconfig.yue", ".luarc.json", ".vscode"} {
		remove(t, root, path)
	}
	write(t, root, ".gitignore", "dist/\nmoonwell.local.pkl\n.pkl-lsp/\n")
	r := ok(t, root, "setup")
	for _, path := range []string{"yueconfig.yue", ".luarc.json", ".vscode/extensions.json", ".moonwell/types/natives.d.lua"} {
		if !exists(root, path) {
			t.Error(path)
		}
	}
	contains(t, read(t, root, ".gitignore"), ".moonwell/\nsrc/**/*.lua\n")
	contains(t, r.output, "Added yueconfig.yue for the editor.")
	mine := "return { build: false }\n"
	write(t, root, "yueconfig.yue", mine)
	ok(t, root, "setup")
	if read(t, root, "yueconfig.yue") != mine {
		t.Fatal("overwritten")
	}
}

func TestE2EBuildRefusesSourceAsOutput(t *testing.T) {
	root := compiling(t)
	edit(t, root, "moonwell.pkl", `folder = "dist/bin"`, `folder = "maps"`)
	fails(t, root, []string{"isReservedFolder"}, "build")
	if !exists(root, "maps/map.w3x/war3map.lua") {
		t.Fatal("source deleted")
	}
}

func TestE2ECheckReportsSyntaxPosition(t *testing.T) {
	root := compiling(t)
	write(t, root, "src/main.yue", "import \"moonwell\" as mw\nx = \n  if then\n")
	fails(t, root, []string{"error: src/main.yue:"}, "check")
}

func TestE2EDevRechecksSourceChanges(t *testing.T) {
	root := compiling(t)
	wait, stop := startDev(t, root, false)
	defer stop()
	wait("Watching src/")
	write(t, root, "src/main.yue", "x = \n  if then\n")
	wait("error: src/main.yue:")
}

const typo = "error: src/main.yue:9:10 › Unknown global CreatUnit.\nhint: Did you mean CreateUnit? Declare your own globals with `global`, or add them to lint.globals in moonwell.pkl."

func TestE2ELintMisspeltNativePositionAndHint(t *testing.T) {
	root := compiling(t)
	edit(t, root, "src/main.yue", "CreateUnit Player(0)", "CreatUnit Player(0)")
	fails(t, root, []string{typo}, "check")
}

func TestE2ELintWarningBuildSucceeds(t *testing.T) {
	root := compiling(t)
	edit(t, root, "src/main.yue", "CreateUnit Player(0)", "CreatUnit Player(0)")
	edit(t, root, "moonwell.pkl", `unknownGlobals = "error"`, `unknownGlobals = "warning"`)
	r := ok(t, root, "build")
	contains(t, r.output, strings.Replace(typo, "error: ", "warning: ", 1), "Built dist/bin/map.w3x")
}

func TestE2ELintDeclaredConfiguredAndMapGlobals(t *testing.T) {
	root := compiling(t)
	write(t, root, "src/state.yue", "global Round = 1\n")
	appendTo(t, root, "src/main.yue", "\nimport \"state\"\nglobal Score = 0\nprint Score, Round, MyLibrary, gg_unit_Hblm_0003\n")
	edit(t, root, "moonwell.pkl", "globals = List()", `globals = List("MyLibrary")`)
	ok(t, root, "check")
}

func TestE2ELintOnlyRequiredFilesAndTheirGlobals(t *testing.T) {
	root := compiling(t)
	write(t, root, "src/extra.yue", "global Extra = 1\nprint Extra\n")
	ok(t, root, "check")
	appendTo(t, root, "src/main.yue", "\nprint Extra\n")
	fails(t, root, []string{"Unknown global Extra."}, "check")
}

func TestE2ELuaModulesRequiredAndUnused(t *testing.T) {
	root := compiling(t)
	if !exists(root, "lua/.gitkeep") {
		t.Fatal("lua folder absent")
	}
	write(t, root, "lua/tools/init.lua", "local M = {}\nfunction M.greet(name)\n return \"Hello, \" .. name\nend\nreturn M\n")
	write(t, root, "lua/counter.lua", "Count = 0\nfunction CountUp()\n Count = Count + 1\nend\n")
	write(t, root, "lua/unused.lua", "Unused = true\n")
	appendTo(t, root, "src/main.yue", "\nimport \"tools\"\nrequire \"counter\"\nCountUp!\nprint tools.greet \"Moonwell\"\n")
	lua := builtScript(t, root)
	contains(t, lua, `__mw.define("tools", function(...)`, `__mw.define("counter", function(...)`, `"tools", "lua/tools/init.lua"}`)
	if strings.Contains(lua, "Unused = true") {
		t.Fatal("unused bundled")
	}
}

func TestE2ELuaAndYueModuleCollision(t *testing.T) {
	root := compiling(t)
	write(t, root, "lua/main.lua", "return {}\n")
	fails(t, root, []string{"error: lua/main.lua › Module main is defined by src/main.yue and lua/main.lua."}, "check")
}

func TestE2ELuaLocalFunctionIsNotGlobal(t *testing.T) {
	root := compiling(t)
	write(t, root, "lua/counter.lua", "local function hidden() end\n")
	appendTo(t, root, "src/main.yue", "\nrequire \"counter\"\nhidden!\n")
	fails(t, root, []string{"Unknown global hidden."}, "check")
}

func TestE2ELuaGlobalsKnownOnlyWhenRequired(t *testing.T) {
	root := compiling(t)
	write(t, root, "lua/counter.lua", "function CountUp() end\n")
	base := read(t, root, "src/main.yue")
	write(t, root, "src/main.yue", base+"\nCountUp!\n")
	fails(t, root, []string{"Unknown global CountUp."}, "check")
	write(t, root, "src/main.yue", base+"\nrequire \"counter\"\nCountUp!\n")
	ok(t, root, "check")
}

func exampleLibrary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write(t, dir, "src/example/greet.lua", "local M = {}\nfunction M.hello(name)\n return \"Hello, \" .. name\nend\nreturn M\n")
	write(t, dir, "src/example/loud.yue", "import \"example.greet\"\n\nexport shout = (name) -> greet.hello(name)\\upper!\n")
	write(t, dir, "src/example/loud.lua", "return { shout = function() return \"stale\" end }\n")
	write(t, dir, "src/example/globals.lua", "function ExampleAdd(a, b)\n return a + b\nend\n")
	return dir
}

func useLibrary(t *testing.T, root, library string) {
	t.Helper()
	appendTo(t, root, "moonwell.local.pkl", "\nlibraries { [\"ex\"] { path = \""+filepath.ToSlash(library)+"\"; dir = \"src\" } }\n")
	appendTo(t, root, "src/main.yue", "\nimport \"example.loud\"\nrequire \"example.globals\"\nprint loud.shout \"Moonwell\"\nprint ExampleAdd 1, 2\n")
}

func TestE2ELocalLibraryBuildAndEditorView(t *testing.T) {
	root := compiling(t)
	useLibrary(t, root, exampleLibrary(t))
	lua := builtScript(t, root)
	contains(t, lua, `__mw.define("example.loud", function(...)`, `"example.greet", ".moonwell/libraries/ex/example/greet.lua"}`, `".moonwell/libraries/ex/example/loud.yue"`)
	if strings.Contains(lua, `"stale"`) || exists(root, "moonwell.lock") {
		t.Fatal("stale module or local lock")
	}
	for _, path := range []string{".moonwell/libraries/ex/example/loud.yue", ".moonwell/lua/example/greet.lua"} {
		if !exists(root, path) {
			t.Error(path)
		}
	}
	contains(t, read(t, root, ".moonwell/lua/example/loud.lua"), "shout")
	ok(t, root, "setup")
	if !exists(root, ".moonwell/lua/example/loud.lua") {
		t.Fatal("setup removed compiled view")
	}
}

func TestE2ELocalLibraryCollisionNamesBoth(t *testing.T) {
	root := compiling(t)
	useLibrary(t, root, exampleLibrary(t))
	write(t, root, "lua/example/greet.lua", "return {}\n")
	fails(t, root, []string{"Module example.greet is defined by lua/example/greet.lua and .moonwell/libraries/ex/example/greet.lua."}, "check")
}

func TestE2ESetupLibraryViewDespiteCollision(t *testing.T) {
	root := compiling(t)
	useLibrary(t, root, exampleLibrary(t))
	write(t, root, "lua/example/greet.lua", "return {}\n")
	ok(t, root, "setup")
	contains(t, read(t, root, ".moonwell/lua/example/greet.lua"), "Hello, ")
	if !exists(root, ".moonwell/types/natives.d.lua") {
		t.Fatal("no declarations")
	}
}

func TestE2ESetupEditorFilesBeforeFailedLibrarySync(t *testing.T) {
	root := compiling(t)
	useLibrary(t, root, filepath.Join(t.TempDir(), "missing"))
	fails(t, root, []string{"is not a folder"}, "setup")
	for _, path := range []string{".moonwell/types/natives.d.lua", ".moonwell/yue/moonwell/macros.yue"} {
		if !exists(root, path) {
			t.Error(path)
		}
	}
}

func TestE2ELibraryAssetsBuildAndMapReplacement(t *testing.T) {
	root := compiling(t)
	library := t.TempDir()
	write(t, library, "moonwell-library.json", `{"dir":"src","assets":"assets"}`)
	write(t, library, "src/golems/names.lua", "return { first = \"Granite\" }\n")
	write(t, library, "assets/war3mapImported/golems/frames.toc", "toc from the library")
	write(t, library, "assets/Textures/Golem.blp", "texture from the library")
	appendTo(t, root, "moonwell.local.pkl", "\nlibraries { [\"golems\"] { path = \""+filepath.ToSlash(library)+"\" } }\n")
	appendTo(t, root, "src/main.yue", "\nimport \"golems.names\"\nprint names.first\n")
	r := ok(t, root, "check")
	contains(t, r.output, "2 asset(s).")
	write(t, root, "assets/textures/golem.blp", "texture from the map")
	r = ok(t, root, "build")
	contains(t, r.output, "assets/textures/golem.blp replaces library golems's Textures/Golem.blp", "Imported 2 asset(s).")
	staged := filepath.Join(root, "dist", "stage", "map.w3x")
	if read(t, staged, "war3mapImported/golems/frames.toc") != "toc from the library" || read(t, staged, "textures/golem.blp") != "texture from the map" {
		t.Fatal("staged assets")
	}
	imports, err := assets.ReadImports([]byte(read(t, staged, "war3map.imp")), "war3map.imp")
	if err != nil || len(imports) != 2 {
		t.Fatalf("%v %+v", err, imports)
	}
	opened := archive(t, root)
	toc, _ := packed(t, opened, `war3mapImported\golems\frames.toc`)
	if string(toc) != "toc from the library" {
		t.Fatal(toc)
	}
	lua, _ := packed(t, opened, "war3map.lua")
	contains(t, string(lua), `__mw.define("golems.names"`)
	if exists(root, "maps/map.w3x/war3mapImported") {
		t.Fatal("source map written")
	}
}

func TestE2ECaptainObjectsRepeatableAndSourceUnchanged(t *testing.T) {
	root := compiling(t)
	before := testkit.Snapshot(t, filepath.Join(root, "maps", "map.w3x"))
	ids := read(t, root, "src/generated/objects.yue")
	var previous string
	for range 2 {
		r := ok(t, root, "build")
		contains(t, r.output, "Added 1 custom object(s) to 2 file(s).")
		data := read(t, root, "dist/bin/map.w3x")
		if previous != "" && data != previous {
			t.Fatal("archive changed")
		}
		previous = data
	}
	opened := archive(t, root)
	for _, name := range []string{"war3map.w3u", "war3mapSkin.w3u"} {
		data, found := packed(t, opened, name)
		if !found {
			t.Fatal(name)
		}
		file, err := objects.ReadModFile(data, objects.Simple, name)
		if err != nil {
			t.Fatal(err)
		}
		if len(file.Custom.Objects) != 1 || file.Custom.Objects[0].Base != "hfoo" || file.Custom.Objects[0].ID != "h000" {
			t.Fatal(file)
		}
		if name == "war3mapSkin.w3u" {
			values := map[string]string{}
			for _, set := range file.Custom.Objects[0].Sets {
				for _, mod := range set.Mods {
					if mod.Value.Type == "string" {
						values[mod.Field] = mod.Value.Text
					}
				}
			}
			if !reflect.DeepEqual(values, map[string]string{"unam": "Captain", "umdl": `units\human\TheCaptain\TheCaptain`, "uico": `ReplaceableTextures\CommandButtons\BTNTheCaptain.blp`}) {
				t.Fatal(values)
			}
		}
	}
	lua, _ := packed(t, opened, "war3map.lua")
	contains(t, string(lua), `__mw.define("generated.objects", function(...)`)
	sameFiles(t, before, testkit.Snapshot(t, filepath.Join(root, "maps", "map.w3x")), "build source")
	if read(t, root, "src/generated/objects.yue") != ids {
		t.Fatal("IDs rewritten")
	}
	contains(t, ok(t, root, "objects:check").output, "src/generated/objects.yue: current")
}
