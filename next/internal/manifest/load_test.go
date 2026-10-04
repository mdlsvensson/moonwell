package manifest

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

var background = context.Background()

// world is a test world for a new project folder that holds the files.
func world(t *testing.T, files map[string]string) *env.Env {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		testkit.WriteFile(t, root, name, []byte(content))
	}
	e, _ := testkit.Env(t, root)
	return e
}

// ran is one program a stand-in was asked to run: its command line and the folder it was to run in.
type ran struct{ line, dir string }

// answering stands in for env.Run. It answers a command line with the result of the longest prefix it knows, keeps
// what it was asked in calls, and returns an error for a command line it knows no prefix of.
func answering(calls *[]ran, results map[string]env.RunResult) env.RunFunc {
	return func(_ context.Context, program string, args []string, options env.RunOptions) (env.RunResult, error) {
		line := strings.Join(append([]string{program}, args...), " ")
		*calls = append(*calls, ran{line, options.Dir})
		best, known := "", false
		for prefix := range results {
			if strings.HasPrefix(line, prefix) && len(prefix) >= len(best) {
				best, known = prefix, true
			}
		}
		if !known {
			return env.RunResult{}, errors.New("unexpected command: " + line)
		}
		return results[best], nil
	}
}

func TestLoadEvaluatesTheLocalManifestWhenThereIsOneWithTheProgramItIsGiven(t *testing.T) {
	deps := resolvedDeps(moonwell.Version)
	tests := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"both manifests", map[string]string{"moonwell.pkl": "", "moonwell.local.pkl": "", "PklProject.deps.json": deps}, "moonwell.local.pkl"},
		{"the shared manifest alone", map[string]string{"moonwell.pkl": "", "PklProject.deps.json": deps}, "moonwell.pkl"},
		{"the local manifest alone", map[string]string{"moonwell.local.pkl": "", "PklProject.deps.json": deps}, "moonwell.local.pkl"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := world(t, tt.files)
			var calls []ran
			line := "/cache/pkl/0.32.1/pkl eval --format json --project-dir . " + tt.want
			e.Run = answering(&calls, map[string]env.RunResult{line: {Stdout: printed()}})
			p, err := Load(background, e, "/cache/pkl/0.32.1/pkl")
			if err != nil {
				t.Fatal(diag.Format(err))
			}
			if p.Map.Folder != "map.w3x" || p.File != tt.want || p.Root != e.Root {
				t.Errorf("project = %+v", p)
			}
			if !slices.Equal(calls, []ran{{line, e.Root}}) {
				t.Errorf("ran %+v, want %q in the project folder", calls, line)
			}
		})
	}
}

func TestLoadRefusesInOrderWhatItCannotRead(t *testing.T) {
	deps := resolvedDeps(moonwell.Version)
	project := map[string]string{"moonwell.pkl": "", "PklProject.deps.json": deps}
	evaluation := func(result env.RunResult) map[string]env.RunResult {
		return map[string]env.RunResult{"pkl eval": result}
	}
	tests := []struct {
		name  string
		files map[string]string
		runs  map[string]env.RunResult // nil when the failure comes before pkl may run
		file  string                   // "<root>" stands for the project folder
		words []string                 // of the message
		hint  string
	}{
		{"no manifest", map[string]string{"PklProject.deps.json": "{ not json"}, nil,
			"<root>", []string{"No moonwell.pkl found"}, "moonwell init"},
		{"no dependencies file", map[string]string{"moonwell.pkl": ""}, nil,
			"PklProject", []string{"PklProject.deps.json is missing"}, "pkl project resolve"},
		{"a dependencies file that is a folder", map[string]string{"moonwell.pkl": "", "PklProject.deps.json/x": ""}, nil,
			"PklProject.deps.json", []string{"Reading PklProject.deps.json failed"}, "pkl project resolve"},
		{"a dependencies file that is not JSON", map[string]string{"moonwell.pkl": "", "PklProject.deps.json": "{ not json"}, nil,
			"PklProject.deps.json", []string{"not valid JSON"}, "pkl project resolve"},
		{"a package that is not resolved", map[string]string{"moonwell.pkl": "", "PklProject.deps.json": "{}"}, nil,
			"PklProject.deps.json", []string{"not a resolved dependency"}, "pkl project resolve"},
		{"a package of another version", map[string]string{"moonwell.pkl": "", "PklProject.deps.json": resolvedDeps("0.1.0")}, nil,
			"PklProject", []string{"moonwell@0.1.0", "does not match", moonwell.Version}, "pkl project resolve"},
		{"an evaluation that fails", project,
			evaluation(env.RunResult{Code: 1, Stdout: "ignored", Stderr: "\n-- Pkl Error --\nType constraint violated\n"}),
			"moonwell.pkl", []string{"Evaluating moonwell.pkl failed:\n-- Pkl Error --\nType constraint violated"}, ""},
		{"an evaluation that fails and says so on the other stream", project,
			evaluation(env.RunResult{Code: 2, Stdout: "No such module\n"}),
			"moonwell.pkl", []string{"Evaluating moonwell.pkl failed:\nNo such module"}, ""},
		{"output that is not JSON", project, evaluation(env.RunResult{Stdout: "map { }\n"}),
			"moonwell.pkl", []string{"not valid JSON:\nmap { }"}, "Pkl 0.32"},
		{"no output", project, evaluation(env.RunResult{}),
			"moonwell.pkl", []string{"not valid JSON"}, "Pkl 0.32"},
		{"output that is not a project", project, evaluation(env.RunResult{Stdout: `{"map":[]}`}),
			"moonwell.pkl", []string{"moonwell.pkl", "array", "map"}, "@moonwell/Project.pkl"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := world(t, tt.files)
			var calls []ran
			if tt.runs != nil {
				e.Run = answering(&calls, tt.runs)
			}
			p, err := Load(background, e, "pkl")
			failure := asError(t, err, tt.name)
			if file := strings.ReplaceAll(failure.File, e.Root, "<root>"); p != nil || file != tt.file {
				t.Errorf("project = %v, File = %q, want %q", p, file, tt.file)
			}
			for _, word := range tt.words {
				if !strings.Contains(failure.Msg, word) {
					t.Errorf("the message %q lacks %q", failure.Msg, word)
				}
			}
			if !strings.Contains(failure.Hint, tt.hint) || (tt.hint == "") != (failure.Hint == "") {
				t.Errorf("hint = %q, want %q in it", failure.Hint, tt.hint)
			}
		})
	}
}

func TestLoadShowsTheStartOfLongOutputThatIsNotJSON(t *testing.T) {
	e := world(t, map[string]string{"moonwell.pkl": "", "PklProject.deps.json": resolvedDeps(moonwell.Version)})
	var calls []ran
	// Each "\xC3\xA9" is one character of two bytes.
	e.Run = answering(&calls, map[string]env.RunResult{"pkl eval": {Stdout: strings.Repeat("\xC3\xA9", 600)}})
	_, err := Load(background, e, "pkl")
	failure := asError(t, err, "long output")
	if shown := strings.Count(failure.Msg, "\xC3\xA9"); shown != 500 {
		t.Errorf("the message shows %d characters of the output, want 500", shown)
	}
	if failure.Cause == nil {
		t.Error("the error does not keep the decoder's reason as its cause")
	}
}

func TestLoadPassesOnAProgramThatCannotBeStarted(t *testing.T) {
	e := world(t, map[string]string{"moonwell.pkl": "", "PklProject.deps.json": resolvedDeps(moonwell.Version)})
	failed := env.SpawnError("pkl", errors.New("no such program"), "", "")
	e.Run = func(context.Context, string, []string, env.RunOptions) (env.RunResult, error) {
		return env.RunResult{}, failed
	}
	if p, err := Load(background, e, "pkl"); p != nil || err != error(failed) {
		t.Errorf("Load = %v, %v, want the failure as Run gave it", p, err)
	}
}

// linked is a test world for a new project that depends on the schema/ of this checkout, with the dependency
// resolved by pkl, and the pkl program. Pkl loads a local dependency only from the project's own drive, so the
// temporary folder must be on the drive of the checkout.
func linked(t *testing.T, files map[string]string) (*env.Env, string) {
	t.Helper()
	pkl := testkit.NeedPkl(t)
	e := world(t, files)
	e.Run = env.Run
	schema, err := filepath.Rel(e.Root, filepath.Join(testkit.RepoRoot(t), "schema"))
	if err != nil {
		t.Fatalf("the temporary folder %s must be on the drive of the checkout: %v", e.Root, err)
	}
	testkit.WriteFile(t, e.Root, "PklProject", []byte(PklProject(moonwell.Version, filepath.ToSlash(schema))))
	resolved, err := e.Run(background, pkl, []string{"project", "resolve"}, env.RunOptions{Dir: e.Root})
	if err != nil || resolved.Code != 0 {
		t.Fatalf("pkl project resolve: %v\n%s", err, resolved.Stderr)
	}
	return e, pkl
}

func TestLoadReadsTheTemplatesManifestWithRealPkl(t *testing.T) {
	template, err := moonwell.TemplateFiles()
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, file := range template {
		if strings.HasSuffix(file.Path, ".pkl") {
			files[file.Path] = string(file.Data)
		}
	}
	e, pkl := linked(t, files)
	shared, err := Load(background, e, pkl)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	if shared.File != "moonwell.pkl" || shared.Launch.GameExecutable != nil {
		t.Errorf("File = %q, game = %q", shared.File, text(shared.Launch.GameExecutable))
	}
	if created, err := EnsureLocalManifest(e.Root); err != nil || !created {
		t.Fatalf("EnsureLocalManifest = %v, %v", created, err)
	}
	p, err := Load(background, e, pkl)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	if p.Root != e.Root || p.File != "moonwell.local.pkl" || text(p.Launch.GameExecutable) != DefaultGameExecutable {
		t.Errorf("Root = %q, File = %q, game = %q", p.Root, p.File, text(p.Launch.GameExecutable))
	}
	if p.Map != (Map{Folder: "map.w3x", Entry: "src/main.yue"}) || p.Build != (Build{Folder: "dist/bin"}) ||
		!slices.Equal(p.Launch.Args, []string{"-launch", "-windowmode", "windowed"}) || p.Yue.Version == "" || p.Yue.Path != nil {
		t.Errorf("map = %+v, build = %+v, launch = %q, yue = %+v", p.Map, p.Build, p.Launch.Args, p.Yue)
	}
	if p.Assets.Paths.Len() != 0 || p.Assets.Exclude == nil || len(p.Assets.Exclude) != 0 ||
		p.Lint.UnknownGlobals != "error" || p.Lint.Globals == nil || p.Libraries == nil || len(p.Libraries) != 0 {
		t.Errorf("assets = %+v, lint = %+v, libraries = %+v", p.Assets, p.Lint, p.Libraries)
	}
	// The template writes player 0 with every field null: the override is there, and sets nothing.
	if player, written := p.Settings.Players["0"]; !written || player != (Player{}) || p.Settings.Info != (Info{}) {
		t.Errorf("player 0 = %+v, written %v; info = %+v", player, written, p.Settings.Info)
	}
	captain, _ := p.Objects.Units.Get("captain")
	typed := slices.Sorted(slices.Values(captain.Typed.Keys()))
	if captain.ID != "h000" || captain.Base != "hfoo" || captain.Source != "objects/units.pkl" ||
		!slices.Equal(typed, []string{"iconGameInterface", "modelFile", "name"}) {
		t.Errorf("captain = %+v, with the typed properties %q", captain, typed)
	}
	if name, _ := captain.Typed.Get("name"); name != "Captain" || captain.Properties.Len() != 0 || p.Objects.Units.Len() != 1 {
		t.Errorf("captain's name = %v, properties = %q, units = %q", name, captain.Properties.Keys(), p.Objects.Units.Keys())
	}
}

// everything is a manifest that sets every setting, a library of each kind, an asset path, lint globals and an
// object of every category. Where the schema allows it, a value is false, zero or empty, which is set all the same.
const everything = `amends "@moonwell/Project.pkl"

map { folder = "hero.w3x"; entry = "src/game/init.yue" }
build { folder = "out"; minify = true }
launch { gameExecutable = "C:/Games/Warcraft III.exe"; args = List("-launch") }
yue { version = "0.34.2"; path = "tools/yue" }
assets {
  paths { ["icons/BTNSword.blp"] = #"ReplaceableTextures\CommandButtons\BTNSword.blp"# }
  exclude = List("credits/")
}
lint { unknownGlobals = "warning"; globals = List("MyLibrary") }
libraries {
  ["example"] { github = "mdlsvensson/moonwell-example-lib"; tag = "v0.2.0"; dir = "src" }
  ["mine"] { path = "../mine" }
}
settings {
  info { name = ""; author = "A"; description = "D"; recommendedPlayers = "R"; preview = "preview.png" }
  loadingScreen { background = 0; model = "M.mdx"; text = "T"; title = "Ti"; subtitle = "S" }
  players {
    ["0"] { name = "P"; controller = "user"; race = "human"; fixedStart = false; x = 0; y = -896.5 }
  }
  forces {
    ["0"] {
      name = "F"; allied = false; alliedVictory = false; sharedVision = false; sharedControl = false
      sharedAdvancedControl = false
    }
  }
  environment {
    soundEnvironment = "Dungeon"
    waterColor = List(0, 0, 0, 0)
    fog { enabled = false; style = 0; start = 0; end = 0; density = 0; color = List(0, 0, 0, 0) }
  }
  gameplay { heroMaxLevel = 20; foodLimit = 0 }
  gameplayConstants { ["Misc"] { ["DefenseArmor"] = "0.05" } }
  gameInterface { ["FrameDef"] { ["UPKEEP_NONE"] = "No Upkeep" } }
}
objects {
  heroes { ["paladin"] { id = "H000"; base = "Hpal"; name = "Paladin"; properties { ["raw"] = 1 } } }
  units { ["captain"] { id = "h000"; base = "hfoo"; name = "Captain"; properties { ["raw"] = true } } }
  buildings { ["tower"] { id = "h001"; base = "hwtw"; name = "Tower"; properties { ["raw"] = "x" } } }
  items { ["sword"] { id = "I000"; base = "ratc"; name = "Sword"; properties { ["raw"] = List(1, 2) } } }
  abilities { ["bolt"] { id = "A000"; base = "AHtb"; name = "Bolt"; properties { ["raw"] = List(List("a")) } } }
  buffs { ["stun"] { id = "B000"; base = "BHtb"; tooltip = "Stunned"; properties { ["raw"] = 0.5 } } }
  upgrades { ["armor"] { id = "R000"; base = "Rhar"; name = List("One", "Two"); properties { ["raw"] = "y" } } }
}
`

func TestLoadSetsEveryFieldOfAManifestThatSetsEverythingWithRealPkl(t *testing.T) {
	e, pkl := linked(t, map[string]string{"moonwell.pkl": everything})
	p, err := Load(background, e, pkl)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	for _, path := range unset(p) {
		t.Errorf("%s is not set: the schema and its field in this package have different names, or the manifest of "+
			"this test does not set it", path)
	}
	// A few values, to see that each went where it belongs.
	if target, _ := p.Assets.Paths.Get("icons/BTNSword.blp"); target != `ReplaceableTextures\CommandButtons\BTNSword.blp` {
		t.Errorf("the asset path = %q", target)
	}
	player, force, fog := p.Settings.Players["0"], p.Settings.Forces["0"], p.Settings.Environment.Fog
	if *player.Y != -896.5 || *player.X != 0 || *player.FixedStart || *force.Allied || *fog.Enabled || *fog.Density != 0 {
		t.Errorf("player = %+v, force = %+v, fog = %+v", player, force, fog)
	}
	if !slices.Equal(p.LibraryKeys(), []string{"example", "mine"}) || text(p.Libraries["mine"].Path) != "../mine" ||
		text(p.Libraries["example"].Tag) != "v0.2.0" {
		t.Errorf("libraries = %+v", p.Libraries)
	}
	for _, category := range Categories {
		objects := p.Objects.Of(category)
		for key, object := range objects.All() {
			if objects.Len() != 1 || object.Source != "moonwell.pkl" || object.Typed.Len() != 1 || object.Properties.Len() != 1 {
				t.Errorf("%s: %d objects; %s = %+v", category, objects.Len(), key, object)
			}
		}
	}
}

// unset walks a value and returns the path of every field that is zero wherever it is found. The elements of a
// map or a list share one path, so a field of theirs is set when one element sets it: a library has a repository
// or a folder, and never both. A pointer that is not nil is set, whatever it points at.
func unset(value any) []string {
	set := map[string]bool{}
	var paths []string
	note := func(path string, isSet bool) {
		if _, known := set[path]; !known {
			paths = append(paths, path)
		}
		set[path] = set[path] || isSet
	}
	var walk func(path string, v reflect.Value)
	walk = func(path string, v reflect.Value) {
		switch v.Kind() {
		case reflect.Struct:
			for i := range v.NumField() {
				walk(path+"."+v.Type().Field(i).Name, v.Field(i))
			}
		case reflect.Map:
			note(path, v.Len() > 0)
			for _, key := range v.MapKeys() {
				walk(path+"[]", v.MapIndex(key))
			}
		case reflect.Slice:
			note(path, v.Len() > 0)
			for i := range v.Len() {
				walk(path+"[]", v.Index(i))
			}
		case reflect.Pointer, reflect.Interface:
			note(path, !v.IsNil())
		default:
			note(path, !v.IsZero())
		}
	}
	walk("Project", reflect.Indirect(reflect.ValueOf(value)))
	return slices.DeleteFunc(paths, func(path string) bool { return set[path] })
}

func TestUnsetNamesAFieldThatNoValueOfItsKindSets(t *testing.T) {
	bare := unset(decoded(t, printed(), "moonwell.pkl"))
	for _, path := range []string{
		"Project.Build.Minify", "Project.Launch.GameExecutable", "Project.Yue.Path", "Project.Assets.Exclude",
		"Project.Lint.Globals", "Project.Libraries", "Project.Settings.Info.Name", "Project.Settings.Players",
		"Project.Settings.Environment.WaterColor", "Project.Settings.Environment.Fog.Enabled",
		"Project.Settings.Gameplay.FoodLimit", "Project.Objects.Upgrades.keys",
	} {
		if !slices.Contains(bare, path) {
			t.Errorf("a manifest that sets little: %s is not named in %q", path, bare)
		}
	}
	for _, path := range []string{"Project.Root", "Project.File", "Project.Map.Folder", "Project.Launch.Args", "Project.Yue.Version"} {
		if slices.Contains(bare, path) {
			t.Errorf("a manifest that sets little: %s is named, and it is set", path)
		}
	}
	kinds := func(libraries string) []string {
		return unset(decoded(t, printed(`"libraries":`+libraries, `"settings":{"info":{"name":""}}`), "moonwell.pkl"))
	}
	one := kinds(`{"mine":{"path":"../mine","dir":""}}`)
	if !slices.Contains(one, "Project.Libraries[].GitHub") || !slices.Contains(one, "Project.Libraries[].Dir") ||
		slices.Contains(one, "Project.Libraries[].Path") || slices.Contains(one, "Project.Settings.Info.Name") {
		t.Errorf("a library of one kind, and an empty name: %q", one)
	}
	both := kinds(`{"mine":{"path":"../mine","dir":""},"example":{"github":"a/b","tag":"v1","dir":"src"}}`)
	if slices.ContainsFunc(both, func(path string) bool { return strings.HasPrefix(path, "Project.Libraries") }) {
		t.Errorf("a library of each kind: %q", both)
	}
}
