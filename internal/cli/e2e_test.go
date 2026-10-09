package cli

import (
	"bytes"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/war3/imp"
	"github.com/mdlsvensson/moonwell/internal/war3/objmod"
)

func openBuiltArchive(t *testing.T, root string) *testkit.MPQ {
	t.Helper()
	opened, err := testkit.OpenMPQ([]byte(readFile(t, root, "dist/bin/map.w3x")))
	if err != nil {
		t.Fatal(err)
	}
	return opened
}

func readArchiveFile(t *testing.T, opened *testkit.MPQ, name string) (data []byte, found bool) {
	t.Helper()
	data, found, err := opened.Read(name)
	if err != nil {
		t.Fatal(err)
	}
	return data, found
}

func readBuiltScript(t *testing.T, root string) string {
	t.Helper()
	mustSucceed(t, root, "build")
	data, found := readArchiveFile(t, openBuiltArchive(t, root), "war3map.lua")
	if !found {
		t.Fatal("the archive has no script")
	}
	return string(data)
}

func stageAndLaunch(t *testing.T, root, body string) string {
	t.Helper()
	staged := launched(t, root, body, "test")
	checkContains(t, readFile(t, staged, "war3map.lua"), `__mw.boot("main")`)
	return staged
}

func launched(t *testing.T, root, body string, line ...string) string {
	t.Helper()
	game := filepath.Join(root, "Warcraft III.exe")
	writeFile(t, root, "Warcraft III.exe", "")
	writeLocalManifest(t, root, "launch { gameExecutable = #\""+game+"\"# }\n"+body)
	var started [][]string
	withTheGame := func(root string, log *env.Logger) *env.Env {
		e := env.New(root, log)
		e.Spawn = func(program string, args []string) error {
			started = append(started, append([]string{program}, args...))
			return nil
		}
		return e
	}
	r := checkExitCode(t, runCLIIn(background, withTheGame, root, line...), 0, line)
	checkContains(t, r.output, "Launched Warcraft III with dist/stage/map.w3x.")
	staged := filepath.Join(root, "dist", "stage", "map.w3x")
	want := []string{game, "-launch", "-windowmode", "windowed", "-loadfile", staged}
	if len(started) != 1 || !slices.Equal(started[0], want) {
		t.Fatalf("the line started %q, want %q once", started, want)
	}
	return staged
}

func startDev(t *testing.T, root string) (wait func(wanted string)) {
	t.Helper()
	e, log := newRealEnv(root)
	until := startDevIn(t, e, log)
	return func(wanted string) {
		t.Helper()
		until("a line with "+strconv.Quote(wanted), func() bool {
			return slices.ContainsFunc(log.Lines(), func(line string) bool { return strings.Contains(line, wanted) })
		})
	}
}

func TestE2EInitBuildInjectedArchive(t *testing.T) {
	root := compiling(t)
	r := mustSucceed(t, root, "build")
	checkContains(t, r.output, "Built dist/bin/map.w3x")
	opened := openBuiltArchive(t, root)
	lua, found := readArchiveFile(t, opened, "war3map.lua")
	if !found {
		t.Fatal("the archive has no script")
	}
	checkContains(t, string(lua), "function main()", `__mw.define("main", function(...)`, `__mw.boot("main")`, "1751543663")
	if _, found = readArchiveFile(t, opened, "war3map.w3i"); !found {
		t.Fatal("the archive has no map info")
	}
	list, found := readArchiveFile(t, opened, "(listfile)")
	if !found {
		t.Fatal("the archive has no list of its files")
	}
	checkContains(t, string(list), "war3map.lua")
}

func TestE2ECheckWritesEditorDeclarations(t *testing.T) {
	root := compiling(t)
	mustSucceed(t, root, "check")
	checkContains(t, readFile(t, root, ".moonwell/types/natives.d.lua"), "function CreateUnit(")
	checkContains(t, readFile(t, root, ".moonwell/types/objects.d.lua"), "---@field captain integer h000")
	checkContains(t, readFile(t, root, ".moonwell/types/map.d.lua"), "---@type unit\ngg_unit_Hblm_0003 = nil")
	checkContains(t, readFile(t, root, ".moonwell/types/moonwell.d.lua"),
		"function moonwell.on_main(fn) end", "function require(name) end")
	checkContains(t, readFile(t, root, ".moonwell/yue/moonwell/macros.yue"), "export macro FourCC")
}

func TestE2ECheckFourCCSourcePosition(t *testing.T) {
	root := compiling(t)
	replaceInFile(t, root, "src/main.yue", `$FourCC("hfoo")`, `$FourCC("hfo")`)
	mustFail(t, root, []string{"error: src/main.yue:13 " + mark +
		` $FourCC needs a string literal of exactly 4 characters, such as "hfoo".`}, "check")
}

func TestE2EBuildImportsAssets(t *testing.T) {
	root := compiling(t)
	data := []byte{0, 1, 2, 250, 255}
	testkit.WriteFile(t, root, "assets/Models/unit.mdx", data)
	mustSucceed(t, root, "build")
	opened := openBuiltArchive(t, root)
	got, found := readArchiveFile(t, opened, `Models\unit.mdx`)
	if !found || !bytes.Equal(got, data) {
		t.Fatalf("the archive holds %v for the asset", got)
	}
	index, _ := readArchiveFile(t, opened, "war3map.imp")
	imports, err := imp.Read(index, "war3map.imp")
	if err != nil || len(imports) != 1 || imports[0].Flag != imp.CustomPath || imports[0].Path != `Models\unit.mdx` {
		t.Fatalf("the index of imports: %v %+v", err, imports)
	}
	if exists(root, ".asset-state") || exists(root, "maps/map.w3x/Models") {
		t.Fatal("a build wrote into the source map, or the ownership state")
	}
}

func TestE2ECheckReportsAssetProblem(t *testing.T) {
	root := compiling(t)
	replaceInFile(t, root, "moonwell.pkl", "paths {}", `paths { ["missing.blp"] = "x.blp" }`)
	mustFail(t, root, []string{"does not exist"}, "check")
}

func TestE2EFailedBuildDeletesPreviousArchive(t *testing.T) {
	root := compiling(t)
	mustSucceed(t, root, "build")
	if !exists(root, "dist/bin/map.w3x") {
		t.Fatal("a build left no archive")
	}
	writeFile(t, root, "src/main.yue", "x = \n  if then\n")
	mustFail(t, root, []string{"error: src/main.yue:"}, "build")
	if exists(root, "dist/bin/map.w3x") {
		t.Fatal("the archive of the build before is there after a build that failed")
	}
}

func TestE2ETestStagesAndLaunches(t *testing.T) { stageAndLaunch(t, compiling(t), "") }

func TestE2ETestStagesTheEntryAndTheFormThatItsLineNames(t *testing.T) {
	root := compiling(t)
	writeFile(t, root, "src/other.yue", "import \"moonwell\" as mw\n\nmw.on_main ->\n  print \"Another entry.\"\n")
	const otherEntry, minifiedOther = `__mw.boot("other")`, `"other", "src/other.yue", true},`
	plain := readFile(t, stageAndLaunch(t, root, ""), "war3map.lua")
	if strings.Contains(plain, otherEntry) || strings.Contains(plain, `.yue", true},`) {
		t.Fatal("a test without flags staged another entry than the manifest's, or a minified module")
	}
	staged := readFile(t, launched(t, root, "", "test", "--entry", "src/other.yue", "--minify"), "war3map.lua")
	for _, part := range []string{otherEntry, minifiedOther, `"Another entry."`} {
		if !strings.Contains(staged, part) {
			t.Errorf("a test with --entry src/other.yue --minify staged a script without %s", part)
		}
	}
	if strings.Contains(staged, `__mw.boot("main")`) || strings.Contains(staged, `__mw.define("main"`) {
		t.Fatal("a test with --entry staged the manifest's entry")
	}
}

func TestE2ESetupLocalManifestCreatesAndKeeps(t *testing.T) {
	world, root := newFakeWorld(t), newProject(t, "my-map")
	removeFile(t, root, "moonwell.local.pkl")
	world.mustSucceed(t, root, "setup")
	if readFile(t, root, "moonwell.local.pkl") != manifest.LocalManifestText() {
		t.Fatal("wrong local manifest")
	}
	mine := "amends \"moonwell.pkl\"\nlaunch { gameExecutable = \"/games/wc3.exe\" }\n"
	writeFile(t, root, "moonwell.local.pkl", mine)
	world.mustSucceed(t, root, "setup")
	if readFile(t, root, "moonwell.local.pkl") != mine {
		t.Fatal("overwritten")
	}
}

func TestE2ESetupUpgradesEditorFilesAndKeepsExisting(t *testing.T) {
	world, root := newFakeWorld(t), newProject(t, "my-map")
	for _, path := range []string{"yueconfig.yue", ".luarc.json", ".vscode"} {
		removeFile(t, root, path)
	}
	writeFile(t, root, ".gitignore", "dist/\nmoonwell.local.pkl\n.pkl-lsp/\n")
	r := world.mustSucceed(t, root, "setup")
	for _, path := range []string{
		"yueconfig.yue", ".luarc.json", ".vscode/extensions.json", ".moonwell/types/natives.d.lua",
	} {
		if !exists(root, path) {
			t.Error(path)
		}
	}
	checkContains(t, readFile(t, root, ".gitignore"), ".moonwell/\nsrc/**/*.lua\n")
	checkContains(t, r.output, "Added yueconfig.yue for the editor.")
	mine := "return { build: false }\n"
	writeFile(t, root, "yueconfig.yue", mine)
	world.mustSucceed(t, root, "setup")
	if readFile(t, root, "yueconfig.yue") != mine {
		t.Fatal("overwritten")
	}
}

func TestE2EBuildRefusesSourceAsOutput(t *testing.T) {
	root := compiling(t)
	replaceInFile(t, root, "moonwell.pkl", `folder = "dist/bin"`, `folder = "maps"`)
	mustFail(t, root, []string{"isReservedFolder"}, "build")
	if !exists(root, "maps/map.w3x/war3map.lua") {
		t.Fatal("a refused build removed the source map's script")
	}
}

func TestE2ECheckReportsSyntaxPosition(t *testing.T) {
	root := compiling(t)
	writeFile(t, root, "src/main.yue", "import \"moonwell\" as mw\nx = \n  if then\n")
	mustFail(t, root, []string{"error: src/main.yue:"}, "check")
}

func TestE2EDevRechecksSourceChanges(t *testing.T) {
	root := compiling(t)
	wait := startDev(t, root)
	wait("Watching src/")
	writeFile(t, root, "src/main.yue", "x = \n  if then\n")
	wait("error: src/main.yue:")
}

const typo = "error: src/main.yue:9:10 " + mark + " Unknown global CreatUnit.\nhint: Did you mean CreateUnit? " +
	"Declare your own globals with `global`, or add them to lint.globals in moonwell.pkl."

func TestE2ELintMisspeltNativePositionAndHint(t *testing.T) {
	root := compiling(t)
	replaceInFile(t, root, "src/main.yue", "CreateUnit Player(0)", "CreatUnit Player(0)")
	mustFail(t, root, []string{typo}, "check")
}

func TestE2ELintWarningBuildSucceeds(t *testing.T) {
	root := compiling(t)
	replaceInFile(t, root, "src/main.yue", "CreateUnit Player(0)", "CreatUnit Player(0)")
	replaceInFile(t, root, "moonwell.pkl", `unknownGlobals = "error"`, `unknownGlobals = "warning"`)
	r := mustSucceed(t, root, "build")
	checkContains(t, r.output, strings.Replace(typo, "error: ", "warning: ", 1), "Built dist/bin/map.w3x")
}

func TestE2ELintDeclaredConfiguredAndMapGlobals(t *testing.T) {
	root := compiling(t)
	writeFile(t, root, "src/state.yue", "global Round = 1\n")
	appendToFile(t, root, "src/main.yue",
		"\nimport \"state\"\nglobal Score = 0\nprint Score, Round, MyLibrary, gg_unit_Hblm_0003\n")
	replaceInFile(t, root, "moonwell.pkl", "globals = List()", `globals = List("MyLibrary")`)
	mustSucceed(t, root, "check")
}

func TestE2ELintOnlyRequiredFilesAndTheirGlobals(t *testing.T) {
	root := compiling(t)
	writeFile(t, root, "src/extra.yue", "global Extra = 1\nprint Extra\n")
	mustSucceed(t, root, "check")
	appendToFile(t, root, "src/main.yue", "\nprint Extra\n")
	mustFail(t, root, []string{"Unknown global Extra."}, "check")
}

func TestE2ELuaModulesRequiredAndUnused(t *testing.T) {
	root := compiling(t)
	if !exists(root, "lua/.gitkeep") {
		t.Fatal("a new project has no lua folder")
	}
	writeFile(t, root, "lua/tools/init.lua",
		"local M = {}\nfunction M.greet(name)\n return \"Hello, \" .. name\nend\nreturn M\n")
	writeFile(t, root, "lua/counter.lua", "Count = 0\nfunction CountUp()\n Count = Count + 1\nend\n")
	writeFile(t, root, "lua/unused.lua", "Unused = true\n")
	appendToFile(t, root, "src/main.yue",
		"\nimport \"tools\"\nrequire \"counter\"\nCountUp!\nprint tools.greet \"Moonwell\"\n")
	lua := readBuiltScript(t, root)
	checkContains(t, lua, `__mw.define("tools", function(...)`, `__mw.define("counter", function(...)`,
		`"tools", "lua/tools/init.lua"}`)
	if strings.Contains(lua, "Unused = true") {
		t.Fatal("a module that nothing requires is in the bundle")
	}
}

func TestE2ELuaAndYueModuleCollision(t *testing.T) {
	root := compiling(t)
	writeFile(t, root, "lua/main.lua", "return {}\n")
	mustFail(t, root, []string{
		"error: lua/main.lua " + mark + " Module main is defined by src/main.yue and lua/main.lua.",
	}, "check")
}

func TestE2ELuaLocalFunctionIsNotGlobal(t *testing.T) {
	root := compiling(t)
	writeFile(t, root, "lua/counter.lua", "local function hidden() end\n")
	appendToFile(t, root, "src/main.yue", "\nrequire \"counter\"\nhidden!\n")
	mustFail(t, root, []string{"Unknown global hidden."}, "check")
}

func TestE2ELuaGlobalsKnownOnlyWhenRequired(t *testing.T) {
	root := compiling(t)
	writeFile(t, root, "lua/counter.lua", "function CountUp() end\n")
	base := readFile(t, root, "src/main.yue")
	writeFile(t, root, "src/main.yue", base+"\nCountUp!\n")
	mustFail(t, root, []string{"Unknown global CountUp."}, "check")
	writeFile(t, root, "src/main.yue", base+"\nrequire \"counter\"\nCountUp!\n")
	mustSucceed(t, root, "check")
}

func TestE2ELocalLibraryBuildAndEditorView(t *testing.T) {
	root := compiling(t)
	useLibrary(t, root, exampleLibrary(t))
	lua := readBuiltScript(t, root)
	checkContains(t, lua, `__mw.define("example.loud", function(...)`,
		`"example.greet", ".moonwell/libraries/ex/example/greet.lua"}`, `".moonwell/libraries/ex/example/loud.yue"`)
	if strings.Contains(lua, `"stale"`) || exists(root, "moonwell.lock") {
		t.Fatal("the bundle holds the stale Lua of a YueScript module, or a local library got a lock")
	}
	for _, path := range []string{".moonwell/libraries/ex/example/loud.yue", ".moonwell/lua/example/greet.lua"} {
		if !exists(root, path) {
			t.Error(path)
		}
	}
	checkContains(t, readFile(t, root, ".moonwell/lua/example/loud.lua"), "shout")
	newFakeWorld(t).mustSucceed(t, root, "setup")
	if !exists(root, ".moonwell/lua/example/loud.lua") {
		t.Fatal("setup removed the editor's view of a compiled module")
	}
}

func TestE2ELocalLibraryCollisionNamesBoth(t *testing.T) {
	root := compiling(t)
	useLibrary(t, root, exampleLibrary(t))
	writeFile(t, root, "lua/example/greet.lua", "return {}\n")
	mustFail(t, root, []string{"Module example.greet is defined by lua/example/greet.lua and " +
		".moonwell/libraries/ex/example/greet.lua."}, "check")
}

func TestE2ESetupLibraryViewDespiteCollision(t *testing.T) {
	world, root := newFakeWorld(t), newProject(t, "my-map")
	useLibrary(t, root, exampleLibrary(t))
	writeFile(t, root, "lua/example/greet.lua", "return {}\n")
	world.mustSucceed(t, root, "setup")
	checkContains(t, readFile(t, root, ".moonwell/lua/example/greet.lua"), "Hello, ")
	if !exists(root, ".moonwell/types/natives.d.lua") {
		t.Fatal("no declarations")
	}
}

func TestE2ESetupEditorFilesBeforeFailedLibrarySync(t *testing.T) {
	world, root := newFakeWorld(t), newProject(t, "my-map")
	useLibrary(t, root, filepath.Join(t.TempDir(), "missing"))
	world.mustFail(t, root, []string{"is not a folder"}, "setup")
	for _, path := range []string{".moonwell/types/natives.d.lua", ".moonwell/yue/moonwell/macros.yue"} {
		if !exists(root, path) {
			t.Error(path)
		}
	}
}

func TestE2ELibraryAssetsBuildAndMapReplacement(t *testing.T) {
	root := compiling(t)
	library := t.TempDir()
	writeFile(t, library, "moonwell-library.json", `{"dir":"src","assets":"assets"}`)
	writeFile(t, library, "src/golems/names.lua", "return { first = \"Granite\" }\n")
	writeFile(t, library, "assets/war3mapImported/golems/frames.toc", "toc from the library")
	writeFile(t, library, "assets/Textures/Golem.blp", "texture from the library")
	appendToFile(t, root, "moonwell.local.pkl",
		"\nlibraries { [\"golems\"] { path = \""+filepath.ToSlash(library)+"\" } }\n")
	appendToFile(t, root, "src/main.yue", "\nimport \"golems.names\"\nprint names.first\n")
	r := mustSucceed(t, root, "check")
	checkContains(t, r.output, "2 asset(s).")
	writeFile(t, root, "assets/textures/golem.blp", "texture from the map")
	r = mustSucceed(t, root, "build")
	checkContains(t, r.output, "assets/textures/golem.blp replaces library golems's Textures/Golem.blp",
		"Imported 2 asset(s).")
	staged := filepath.Join(root, "dist", "stage", "map.w3x")
	if readFile(t, staged, "war3mapImported/golems/frames.toc") != "toc from the library" ||
		readFile(t, staged, "textures/golem.blp") != "texture from the map" {
		t.Fatal("the stage holds other bytes than the assets")
	}
	imports, err := imp.Read([]byte(readFile(t, staged, "war3map.imp")), "war3map.imp")
	if err != nil || len(imports) != 2 {
		t.Fatalf("the index of imports: %v %+v", err, imports)
	}
	opened := openBuiltArchive(t, root)
	toc, _ := readArchiveFile(t, opened, `war3mapImported\golems\frames.toc`)
	if string(toc) != "toc from the library" {
		t.Fatalf("the archive holds %q for the library's file", toc)
	}
	lua, _ := readArchiveFile(t, opened, "war3map.lua")
	checkContains(t, string(lua), `__mw.define("golems.names"`)
	if exists(root, "maps/map.w3x/war3mapImported") {
		t.Fatal("a build wrote into the source map")
	}
}

func TestE2ECaptainObjectsRepeatableAndSourceUnchanged(t *testing.T) {
	root := compiling(t)
	before := testkit.Snapshot(t, filepath.Join(root, "maps", "map.w3x"))
	ids := readFile(t, root, "src/generated/objects.yue")
	var previous string
	for range 2 {
		r := mustSucceed(t, root, "build")
		checkContains(t, r.output, "Added 1 custom object(s) to 2 file(s).")
		data := readFile(t, root, "dist/bin/map.w3x")
		if previous != "" && data != previous {
			t.Fatal("a second build packed another archive")
		}
		previous = data
	}
	opened := openBuiltArchive(t, root)
	for _, name := range []string{"war3map.w3u", "war3mapSkin.w3u"} {
		data, found := readArchiveFile(t, opened, name)
		if !found {
			t.Fatal(name)
		}
		file, err := objmod.Read(data, objmod.Simple, name)
		if err != nil {
			t.Fatal(err)
		}
		custom := file.Custom.Objects
		if len(custom) != 1 || custom[0].Base.String() != "hfoo" || custom[0].ID.String() != "h000" {
			t.Fatalf("%s: the custom objects are %+v", name, custom)
		}
		if name == "war3mapSkin.w3u" {
			captainsTexts(t, custom[0])
		}
	}
	lua, _ := readArchiveFile(t, opened, "war3map.lua")
	checkContains(t, string(lua), `__mw.define("generated.objects", function(...)`)
	checkSameFiles(t, before, testkit.Snapshot(t, filepath.Join(root, "maps", "map.w3x")), "build source")
	if readFile(t, root, "src/generated/objects.yue") != ids {
		t.Fatal("a build wrote another ids module than init's")
	}
	checkContains(t, mustSucceed(t, root, "objects:check").output, "src/generated/objects.yue: current")
}

func captainsTexts(t *testing.T, captain objmod.Object) {
	t.Helper()
	texts := map[string]string{}
	for _, set := range captain.Sets {
		for _, mod := range set.Mods {
			if mod.Value.Type == objmod.String {
				texts[mod.Field.String()] = mod.Value.Text
			}
		}
	}
	want := map[string]string{
		"unam": "Captain", "umdl": `units\human\TheCaptain\TheCaptain`,
		"uico": `ReplaceableTextures\CommandButtons\BTNTheCaptain.blp`,
	}
	if len(texts) != len(want) {
		t.Fatalf("the captain's texts are %q, want %q", texts, want)
	}
	for field, text := range want {
		if texts[field] != text {
			t.Fatalf("the captain's texts are %q, want %q", texts, want)
		}
	}
}
