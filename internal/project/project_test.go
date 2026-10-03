package project_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/ordered"
	"github.com/mdlsvensson/moonwell/internal/proc"
	"github.com/mdlsvensson/moonwell/internal/project"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

var background = context.Background()

const full = `"map":{"folder":"map.w3x","entry":"src/main.yue"},"build":{"folder":"dist/bin","minify":false},
	"launch":{"args":["-launch"]},"yue":{"version":"0.34.2"}`

const fullAssets = `,"assets":{"paths":{},"exclude":[]}`

// manifest is the FULL manifest of the TypeScript tests, with more keys added.
func manifest(more string) string {
	if more != "" {
		more = "," + more
	}
	return "{" + full + fullAssets + more + "}"
}

func parse(t *testing.T, document, file string) (*project.Project, error) {
	t.Helper()
	tree, err := ordered.Decode([]byte(document))
	if err != nil {
		t.Fatalf("test JSON %s: %v", document, err)
	}
	return project.Parse("/p", tree, file)
}

func parsed(t *testing.T, document, file string) *project.Project {
	t.Helper()
	p, err := parse(t, document, file)
	if err != nil {
		t.Fatalf("Parse(%s): %v", document, diag.Format(err))
	}
	return p
}

func asError(t *testing.T, err error, what string) *diag.Error {
	t.Helper()
	var e *diag.Error
	if !errors.As(err, &e) {
		t.Fatalf("%s: got %v, want a *diag.Error", what, err)
	}
	return e
}

func refused(t *testing.T, document, file, message string) *diag.Error {
	t.Helper()
	_, err := parse(t, document, file)
	e := asError(t, err, document)
	if !strings.Contains(e.Msg, message) || e.File != file {
		t.Errorf("Parse(%s): %+v, want %q naming %s", document, e, message, file)
	}
	return e
}

func TestParseMapsOmittedNullableFieldsToNil(t *testing.T) {
	p := parsed(t, manifest(""), "moonwell.pkl")
	if p.Root != "/p" || p.Manifest != "moonwell.pkl" || p.Map != (project.Map{Folder: "map.w3x", Entry: "src/main.yue"}) ||
		p.Build != (project.Build{Folder: "dist/bin"}) || p.Launch.GameExecutable != nil ||
		!slices.Equal(p.Launch.Args, []string{"-launch"}) || p.Yue.Version != "0.34.2" || p.Yue.Path != nil ||
		p.Assets.Paths.Len() != 0 || len(p.Assets.Exclude) != 0 || p.Settings.Has() || !p.Objects.Empty() ||
		p.Lint.UnknownGlobals != "error" || len(p.Lint.Globals) != 0 || p.Libraries.Len() != 0 {
		t.Errorf("project = %+v", p)
	}
}

func TestParseReadsLibrariesAndRequiresGithubWithTagUnlessPathIsSet(t *testing.T) {
	p := parsed(t, manifest(`"libraries":{
		"example":{"github":"mdlsvensson/moonwell-example-lib","tag":"v0.1.0","dir":"src"},
		"mine":{"path":"../mine","dir":""}}`), "m.pkl")
	example, _ := p.Libraries.Get("example")
	mine, _ := p.Libraries.Get("mine")
	if *example.GitHub != "mdlsvensson/moonwell-example-lib" || *example.Tag != "v0.1.0" || example.Path != nil ||
		example.Dir != "src" || mine.GitHub != nil || mine.Tag != nil || *mine.Path != "../mine" || mine.Dir != "" ||
		!slices.Equal(p.LibraryKeys(), []string{"example", "mine"}) {
		t.Errorf("libraries = %+v, %+v", example, mine)
	}
	e := refused(t, manifest(`"libraries":{"half":{"github":"a/b","dir":""}}`), "m.pkl",
		`libraries["half"] needs both github and tag, or a path.`)
	if !strings.Contains(e.Hint, `path = "../my-library"`) {
		t.Errorf("hint = %q", e.Hint)
	}
}

func TestParseReadsTheLintBlock(t *testing.T) {
	p := parsed(t, manifest(`"lint":{"unknownGlobals":"warning","globals":["MyLibrary"]}`), "m.pkl")
	if p.Lint.UnknownGlobals != "warning" || !slices.Equal(p.Lint.Globals, []string{"MyLibrary"}) {
		t.Errorf("lint = %+v", p.Lint)
	}
	refused(t, manifest(`"lint":{"unknownGlobals":"off","globals":[]}`), "m.pkl",
		`lint.unknownGlobals must be "error" or "warning".`)
}

func TestParseReadsObjectsDefaultingSourcesToTheEvaluatedManifest(t *testing.T) {
	p := parsed(t, manifest(`"objects":{"units":{
		"captain":{"id":"h000","base":"hfoo","source":"objects/units.pkl","properties":{}},
		"local":{"id":"h001","base":"hfoo","properties":{}}}}`), "moonwell.local.pkl")
	captain, _ := p.Objects["units"].Get("captain")
	local, _ := p.Objects["units"].Get("local")
	if captain.Source != "objects/units.pkl" || local.Source != "moonwell.local.pkl" {
		t.Errorf("sources = %q, %q", captain.Source, local.Source)
	}
	e := refused(t, manifest(`"objects":{"units":{"a":{"base":"hfoo"}}}`), "moonwell.pkl",
		`objects.units["a"].id must be a string.`)
	if e.Hint != objects.SchemaHint {
		t.Errorf("hint = %q", e.Hint)
	}
}

func TestParseDefaultsAbsentSettingsAndValidatesMalformedSettingsWithTheManifestPath(t *testing.T) {
	if p := parsed(t, manifest(""), "moonwell.pkl"); p.Settings == nil || p.Settings.Has() {
		t.Errorf("settings = %+v", p.Settings)
	}
	refused(t, manifest(`"settings":{"players":{"24":{}}}`), "moonwell.local.pkl", "players")
}

func TestParseRejectsConflictingTypedAndRawGameplayConstantsAndKeepsSettingsUnmerged(t *testing.T) {
	refused(t, manifest(`"settings":{"gameplay":{"foodLimit":200},"gameplayConstants":{"misc":{"foodceiling":"100"}}}`),
		"moonwell.local.pkl", "FoodCeiling")
	p := parsed(t, manifest(`"settings":{"gameplay":{"foodLimit":200},"gameplayConstants":{"misc":{"foodceiling":"200"}}}`),
		"moonwell.pkl")
	if got := ordered.Stringify(&p.Settings.GameplayConstants, 0); p.Manifest != "moonwell.pkl" || got != `{"misc":{"foodceiling":"200"}}` {
		t.Errorf("gameplay constants = %s", got)
	}
}

func TestParseReadsAssetsAndRejectsAWrongShape(t *testing.T) {
	base := "{" + full
	p := parsed(t, base+`,"assets":{"paths":{"a.blp":"Textures\\a.blp"},"exclude":["credits/"]}}`, "moonwell.pkl")
	if target, _ := p.Assets.Paths.Get("a.blp"); target != `Textures\a.blp` || !slices.Equal(p.Assets.Exclude, []string{"credits/"}) {
		t.Errorf("assets = %+v", p.Assets)
	}
	for bad, message := range map[string]string{
		`null`:                           "assets must be an object",
		`{"paths":[],"exclude":[]}`:      "assets.paths must be",
		`{"paths":{"a":1},"exclude":[]}`: "assets.paths must be",
		`{"paths":{},"exclude":"x"}`:     "assets.exclude must be",
	} {
		refused(t, base+`,"assets":`+bad+`}`, "moonwell.pkl", message)
	}
}

func TestParseDefaultsAMissingAssetsBlock(t *testing.T) {
	// Moonwell 0.1.0 schema packages have none.
	p := parsed(t, "{"+full+"}", "moonwell.pkl")
	if p.Assets.Paths == nil || p.Assets.Paths.Len() != 0 || len(p.Assets.Exclude) != 0 {
		t.Errorf("assets = %+v", p.Assets)
	}
}

func TestParseRejectsASchemaMismatch(t *testing.T) {
	e := refused(t, strings.Replace(manifest(""), `"folder":"map.w3x","entry":"src/main.yue"`, `"folder":3`, 1),
		"moonwell.pkl", "map.folder must be a string.")
	if e.Hint != objects.SchemaHint {
		t.Errorf("hint = %q", e.Hint)
	}
	for document, message := range map[string]string{
		`[]`:                       "the manifest must be an object.",
		`{"map":{}}`:               "build must be an object.",
		"{" + full + `,"build":3}`: "build must be an object.",
		strings.Replace(manifest(""), `"minify":false`, `"minify":"no"`, 1): "build.minify must be a boolean.",
		strings.Replace(manifest(""), `["-launch"]`, `[1]`, 1):              "launch.args must be a list of strings.",
		manifest(`"libraries":{"a":{"path":3}}`):                            `libraries["a"].path must be a string.`,
	} {
		refused(t, document, "moonwell.pkl", message)
	}
}

func localDeps(version string) string {
	return `{"schemaVersion":1,"resolvedDependencies":{
		"package://pkg.pkl-lang.org/github.com/mdlsvensson/moonwell/moonwell@0":{"type":"local",
		"uri":"projectpackage://pkg.pkl-lang.org/github.com/mdlsvensson/moonwell/moonwell@` + version + `","path":"../schema"}}}`
}

func TestReadPackageVersionFindsTheResolvedMoonwellVersion(t *testing.T) {
	if got, err := project.ReadPackageVersion(localDeps("0.1.3")); err != nil || got != "0.1.3" {
		t.Errorf("ReadPackageVersion = %q, %v", got, err)
	}
	for _, document := range []string{`{"resolvedDependencies":{}}`, `{}`, `[]`, `null`} {
		_, err := project.ReadPackageVersion(document)
		if e := asError(t, err, document); !strings.Contains(e.Msg, "not a resolved") {
			t.Errorf("%s: %+v", document, e)
		}
	}
}

func TestCheckPackageVersionComparesMajorMinorOnly(t *testing.T) {
	if err := project.CheckPackageVersion("0.1.9", "0.1.0"); err != nil {
		t.Error(err)
	}
	old := asError(t, project.CheckPackageVersion("0.7.0", "0.8.0"), "a project from the Deno CLI")
	if old.Msg != "Pkl package moonwell@0.7.0 does not match Moonwell CLI 0.8.0." || old.File != "PklProject" ||
		old.Hint != "Use moonwell@0.8.x in PklProject and run `pkl project resolve`." {
		t.Errorf("error = %+v", old)
	}
	// A version released as an executable can be installed; the hint names both ways out.
	newer := asError(t, project.CheckPackageVersion("0.8.2", "0.9.0"), "a project for another executable")
	line := "curl -fsSL https://github.com/mdlsvensson/moonwell/releases/download/moonwell@0.8.2/install.sh | sh"
	if runtime.GOOS == "windows" {
		line = "irm https://github.com/mdlsvensson/moonwell/releases/download/moonwell@0.8.2/install.ps1 | iex"
	}
	want := "Install Moonwell 0.8.2 (" + line + "), or use moonwell@0.9.x in PklProject and run `pkl project resolve`."
	if newer.Hint != want || project.InstallLine("0.8.2") != line {
		t.Errorf("hint = %q, want %q", newer.Hint, want)
	}
}

type output struct {
	code           int
	stdout, stderr string
}

// fakeRunner answers commands by the longest matching prefix of their command line.
func fakeRunner(outputs map[string]output) proc.RunFunc {
	return func(_ context.Context, command string, args []string, _ proc.Options) (proc.Result, error) {
		key := strings.Join(append([]string{command}, args...), " ")
		best := ""
		for prefix := range outputs {
			if strings.HasPrefix(key, prefix) && len(prefix) > len(best) {
				best = prefix
			}
		}
		if best == "" {
			return proc.Result{}, fmt.Errorf("unexpected command: %s", key)
		}
		answer := outputs[best]
		return proc.Result{Code: answer.code, Stdout: answer.stdout, Stderr: answer.stderr}, nil
	}
}

func projectFolder(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		testkit.WriteFile(t, root, name, []byte(content))
	}
	return root
}

func TestLoadPrefersMoonwellLocalPklAndParsesPklOutputOfTheProgramItIsGiven(t *testing.T) {
	root := projectFolder(t, map[string]string{
		"moonwell.pkl": "", "moonwell.local.pkl": "", "PklProject.deps.json": localDeps(moonwell.Version),
	})
	run := fakeRunner(map[string]output{
		"/cache/pkl/0.32.1/pkl eval --format json --project-dir . moonwell.local.pkl": {stdout: manifest("")},
	})
	p, err := project.Load(background, root, "/cache/pkl/0.32.1/pkl", run)
	if err != nil || p.Map.Folder != "map.w3x" || p.Manifest != "moonwell.local.pkl" || p.Root != root {
		t.Errorf("Load = %+v, %v", p, err)
	}
}

func TestLoadReportsPklEvaluationErrors(t *testing.T) {
	root := projectFolder(t, map[string]string{"moonwell.pkl": "", "PklProject.deps.json": localDeps(moonwell.Version)})
	run := fakeRunner(map[string]output{
		"pkl --version": {stdout: "Pkl 0.32.1"},
		"pkl eval":      {code: 1, stderr: "–– Pkl Error ––\nType constraint violated\n"},
	})
	_, err := project.Load(background, root, "pkl", run)
	e := asError(t, err, "an evaluation error")
	if e.Msg != "Evaluating moonwell.pkl failed:\n–– Pkl Error ––\nType constraint violated" || e.File != "moonwell.pkl" {
		t.Errorf("error = %+v", e)
	}
}

func TestLoadExplainsAMissingManifestAndMissingOrCorruptDependencies(t *testing.T) {
	version := fakeRunner(map[string]output{"pkl --version": {stdout: "Pkl 0.32.1"}})
	empty := t.TempDir()
	_, err := project.Load(background, empty, "pkl", version)
	if e := asError(t, err, "no manifest"); e.Msg != "No moonwell.pkl found in this directory." || e.File != empty ||
		!strings.Contains(e.Hint, "moonwell init") {
		t.Errorf("error = %+v", e)
	}
	_, err = project.Load(background, projectFolder(t, map[string]string{"moonwell.pkl": ""}), "pkl", version)
	if e := asError(t, err, "no deps"); e.Msg != "PklProject.deps.json is missing." || e.File != "PklProject" {
		t.Errorf("error = %+v", e)
	}
	root := projectFolder(t, map[string]string{"moonwell.pkl": "", "PklProject.deps.json": "{ not json"})
	_, err = project.Load(background, root, "pkl", version)
	if e := asError(t, err, "corrupt deps"); !strings.Contains(e.Msg, "PklProject.deps.json") || !strings.Contains(e.Hint, "pkl project resolve") {
		t.Errorf("error = %+v", e)
	}
	stale := projectFolder(t, map[string]string{"moonwell.pkl": "", "PklProject.deps.json": localDeps("0.1.0")})
	_, err = project.Load(background, stale, "pkl", version)
	if e := asError(t, err, "another version"); !strings.Contains(e.Msg, "does not match Moonwell CLI "+moonwell.Version) {
		t.Errorf("error = %+v", e)
	}
}

func TestLoadExplainsPklOutputThatIsNotJSON(t *testing.T) {
	root := projectFolder(t, map[string]string{"moonwell.pkl": "", "PklProject.deps.json": localDeps(moonwell.Version)})
	run := fakeRunner(map[string]output{"pkl --version": {stdout: "Pkl 0.32.1"}, "pkl eval": {stdout: "map { }\n"}})
	_, err := project.Load(background, root, "pkl", run)
	if e := asError(t, err, "not JSON"); e.Msg != "pkl eval printed output that is not valid JSON:\nmap { }" || e.Hint == "" {
		t.Errorf("error = %+v", e)
	}
}

func TestEnsureLocalManifestCreatesMoonwellLocalPklOnceAndNeverOverwritesIt(t *testing.T) {
	root := t.TempDir()
	local := filepath.Join(root, "moonwell.local.pkl")
	if created, err := project.EnsureLocalManifest(root); err != nil || !created {
		t.Fatalf("the first call: %v, %v", created, err)
	}
	if content, _ := os.ReadFile(local); string(content) != project.LocalPkl() {
		t.Errorf("the file holds %q", content)
	}
	os.WriteFile(local, []byte("mine"), 0o666)
	if created, err := project.EnsureLocalManifest(root); err != nil || created {
		t.Errorf("the second call: %v, %v", created, err)
	}
	if content, _ := os.ReadFile(local); string(content) != "mine" {
		t.Errorf("the file was overwritten: %q", content)
	}
}

func TestPklProjectDeclaresARemoteOrLocalMoonwellDependency(t *testing.T) {
	remote := project.PklProject("0.1.0", "")
	if !strings.Contains(remote, `["moonwell"] { uri = "package://pkg.pkl-lang.org/github.com/mdlsvensson/moonwell/moonwell@0.1.0" }`) {
		t.Errorf("remote = %q", remote)
	}
	local := project.PklProject("", "../schema")
	if !strings.Contains(local, `["moonwell"] = import("../schema/PklProject")`) {
		t.Errorf("local = %q", local)
	}
	// The committed template uses the local-link form.
	template, err := os.ReadFile(filepath.Join(testkit.RepoRoot(t), "template", "PklProject"))
	if err != nil || string(template) != local {
		t.Errorf("template/PklProject = %q, %v", template, err)
	}
}

func TestLocalPklAmendsMoonwellPklAndSetsTheEscapedDefaultGamePath(t *testing.T) {
	local := project.LocalPkl()
	for _, want := range []string{
		`amends "moonwell.pkl"`,
		`gameExecutable = "C:\\Program Files (x86)\\Warcraft III\\_retail_\\x86_64\\Warcraft III.exe"`,
		"`moonwell setup` recreates it.",
	} {
		if !strings.Contains(local, want) {
			t.Errorf("LocalPkl lacks %q:\n%s", want, local)
		}
	}
}

func TestLoadEvaluatesARealProjectAgainstTheLocalPackage(t *testing.T) {
	testkit.NeedPkl(t)
	root := t.TempDir()
	schema := filepath.Join(root, "pkl")
	if err := fsx.CopyTree(filepath.Join(testkit.RepoRoot(t), "schema"), schema); err != nil {
		t.Fatal(err)
	}
	os.RemoveAll(filepath.Join(schema, "tests"))
	testkit.WriteFile(t, root, "PklProject", []byte(project.PklProject("", "pkl")))
	testkit.WriteFile(t, root, "moonwell.pkl", []byte("amends \"@moonwell/Project.pkl\"\n\nmap { folder = \"hero.w3x\" }\n"))
	testkit.WriteFile(t, root, "moonwell.local.pkl", []byte("amends \"moonwell.pkl\"\n\nlaunch { gameExecutable = \"C:/wc3.exe\" }\n"))
	resolved, err := proc.Run(background, "pkl", []string{"project", "resolve"}, proc.Options{Dir: root})
	if err != nil || resolved.Code != 0 {
		t.Fatalf("pkl project resolve: %v\n%s", err, resolved.Stderr)
	}
	p, err := project.Load(background, root, "pkl", proc.Run)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	if p.Map.Folder != "hero.w3x" || p.Launch.GameExecutable == nil || *p.Launch.GameExecutable != "C:/wc3.exe" ||
		p.Yue.Version != "0.34.3" || p.Yue.Path != nil || p.Assets.Paths.Len() != 0 || len(p.Assets.Exclude) != 0 ||
		p.Settings.Has() || p.Lint.UnknownGlobals != "error" || len(p.Lint.Globals) != 0 || p.Libraries.Len() != 0 {
		t.Errorf("project = %+v", p)
	}
}
