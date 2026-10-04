package manifest_test

import (
	"context"
	"encoding/json"
	"iter"
	"strings"
	"testing"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/ordered"
	oldproc "github.com/mdlsvensson/moonwell/internal/proc"
	oldproject "github.com/mdlsvensson/moonwell/internal/project"
	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/oracle"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

// What this file compares, and what it leaves out. Both trees are given the JSON pkl prints, and what they read of
// the plain blocks (map, build, launch, yue, lint, assets, libraries) is compared; the settings and the objects are
// compared by the oracles of their areas. What Load refuses before it has a project is compared message by
// message, and the files this package renders byte by byte.
//
// Left out, each for a difference that is meant:
//
//   - A document without an assets, a lint or a libraries block. The other tree gives such a block a default, for
//     packages older than itself; this tree reads what a package of its own minor version prints, which always has
//     the three (TestDecodeIgnoresAFieldItDoesNotKnowAndSuppliesNoDefault). Every document of the other tree's
//     tests lacks one, so each is compared with the blocks it lacks added as pkl prints them, and is counted.
//   - Output that is JSON and not shaped like a project. The other tree names the first wrong field in words of
//     its own, and this tree gives the decoder's reason (TestDecodeRefusesWhatIsNotShapedLikeAProjectWithOneError).
//   - The order of the library keys, which this tree sorts (TestDecodeReadsLibrariesOfBothKindsAndListsTheirKeysSorted),
//     and of asset paths whose names are whole numbers, which the other tree moves to the front
//     (TestOrderedDecodesAMappingInTheOrderItWasWritten). No document here has two libraries out of order or such
//     a name.

// everyPlainBlock sets every field of the plain blocks, the nullable ones too.
const everyPlainBlock = `{"map":{"folder":"hero.w3x","entry":"src/game/init.yue"},
	"build":{"folder":"out","minify":true},
	"launch":{"gameExecutable":"C:\\Games\\Warcraft III.exe","args":["-launch","-windowmode","fullscreen"]},
	"yue":{"version":"0.34.2","path":"tools/yue"},
	"assets":{"paths":{"icons/b.blp":"ReplaceableTextures\\CommandButtons\\BTNB.blp","a.mdx":"a.mdx"},
		"exclude":["credits/","notes.txt"]},
	"lint":{"unknownGlobals":"warning","globals":["MyLibrary","_G2"]},
	"libraries":{"example":{"github":"mdlsvensson/moonwell-example-lib","tag":"v0.1.0","dir":"src"},
		"mine":{"path":"../mine","dir":""}},
	"settings":{},"objects":{}}`

// The documents of the other tree's tests of project.Parse that it accepts, as those tests build them.
const (
	carriedBlocks = `"map":{"folder":"map.w3x","entry":"src/main.yue"},"build":{"folder":"dist/bin","minify":false},
	"launch":{"args":["-launch"]},"yue":{"version":"0.34.2"}`
	carriedAssets = `,"assets":{"paths":{},"exclude":[]}`
)

func carried(more string) string { return "{" + carriedBlocks + carriedAssets + more + "}" }

var carriedDocuments = []struct{ name, document string }{
	{"the nullable fields left out", carried("")},
	{"libraries", carried(`,"libraries":{
		"example":{"github":"mdlsvensson/moonwell-example-lib","tag":"v0.1.0","dir":"src"},
		"mine":{"path":"../mine","dir":""}}`)},
	{"lint", carried(`,"lint":{"unknownGlobals":"warning","globals":["MyLibrary"]}`)},
	{"objects", carried(`,"objects":{"units":{
		"captain":{"id":"h000","base":"hfoo","source":"objects/units.pkl","properties":{}},
		"local":{"id":"h001","base":"hfoo","properties":{}}}}`)},
	{"settings", carried(`,"settings":{"gameplay":{"foodLimit":200},"gameplayConstants":{"misc":{"foodceiling":"200"}}}`)},
	{"assets", "{" + carriedBlocks + `,"assets":{"paths":{"a.blp":"Textures\\a.blp"},"exclude":["credits/"]}}`},
	{"no assets block", "{" + carriedBlocks + "}"},
}

// standardBlocks is what pkl prints for each block the other tree has a default for, when a manifest leaves the
// block alone.
var standardBlocks = map[string]string{
	"assets":    `{"paths":{},"exclude":[]}`,
	"lint":      `{"unknownGlobals":"error","globals":[]}`,
	"libraries": `{}`,
}

// completed is the document with each standard block it lacks, and whether it lacked one.
func completed(t *testing.T, document string) (string, bool) {
	t.Helper()
	var blocks map[string]json.RawMessage
	if err := json.Unmarshal([]byte(document), &blocks); err != nil {
		t.Fatalf("%s: %v", document, err)
	}
	lacked := false
	for name, block := range standardBlocks {
		if _, has := blocks[name]; !has {
			blocks[name], lacked = json.RawMessage(block), true
		}
	}
	whole, err := json.Marshal(blocks)
	if err != nil {
		t.Fatal(err)
	}
	return string(whole), lacked
}

// plain is what a tree read of a manifest's plain blocks, in one shape for both.
type plain struct {
	Root, File string
	Map        manifest.Map
	Build      manifest.Build
	Launch     manifest.Launch
	Yue        manifest.Yue
	Lint       manifest.Lint
	AssetPaths [][2]string // a name and its target, in the order the tree holds them
	Exclude    []string
	Libraries  map[string]manifest.Library
}

func pairs(entries iter.Seq2[string, string]) [][2]string {
	var list [][2]string
	for name, target := range entries {
		list = append(list, [2]string{name, target})
	}
	return list
}

func plainOfOld(p *oldproject.Project) plain {
	libraries := map[string]manifest.Library{}
	for key, library := range p.Libraries.All() {
		libraries[key] = manifest.Library(library)
	}
	return plain{
		Root: p.Root, File: p.Manifest,
		Map: manifest.Map(p.Map), Build: manifest.Build(p.Build), Launch: manifest.Launch(p.Launch),
		Yue: manifest.Yue(p.Yue), Lint: manifest.Lint(p.Lint),
		AssetPaths: pairs(p.Assets.Paths.All()), Exclude: p.Assets.Exclude, Libraries: libraries,
	}
}

func plainOfNew(p *manifest.Project) plain {
	return plain{
		Root: p.Root, File: p.File, Map: p.Map, Build: p.Build, Launch: p.Launch, Yue: p.Yue, Lint: p.Lint,
		AssetPaths: pairs(p.Assets.Paths.All()), Exclude: p.Assets.Exclude, Libraries: p.Libraries,
	}
}

// read gives a document to both trees, which must both accept it, and returns what each read.
func read(t *testing.T, what, document string) (want, got plain) {
	t.Helper()
	tree, err := ordered.Decode([]byte(document))
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
	old, err := oldproject.Parse("/p", tree, "moonwell.local.pkl")
	if err != nil {
		t.Fatalf("%s: the other tree refuses the document: %v", what, err)
	}
	decoded, err := manifest.Decode("/p", "moonwell.local.pkl", []byte(document))
	if err != nil {
		t.Fatalf("%s: this tree refuses the document: %v", what, err)
	}
	return plainOfOld(old), plainOfNew(decoded)
}

func TestOracleOnThePlainBlocks(t *testing.T) {
	compared, leftOut := 0, 0
	compare := func(what, document string) {
		t.Helper()
		want, got := read(t, what, document)
		oracle.Values(t, what, want, got)
		compared++
	}
	compare("every plain block", everyPlainBlock)
	for _, c := range carriedDocuments {
		whole, lacked := completed(t, c.document)
		compare(c.name+", with every block", whole)
		if !lacked {
			compare(c.name, c.document)
			continue
		}
		leftOut++
		want, got := read(t, c.name, c.document)
		if sameJSON(t, want, got) {
			t.Errorf("%s: both trees read the same of it, so it need not be left out", c.name)
		}
	}
	if leftOut != len(carriedDocuments) {
		t.Errorf("%d documents left out for a block they lack, want %d", leftOut, len(carriedDocuments))
	}
	if compared != 1+len(carriedDocuments) {
		t.Errorf("%d documents compared, want %d", compared, 1+len(carriedDocuments))
	}
}

func sameJSON(t *testing.T, a, b any) bool {
	t.Helper()
	first, errFirst := json.Marshal(a)
	second, errSecond := json.Marshal(b)
	if errFirst != nil || errSecond != nil {
		t.Fatal(errFirst, errSecond)
	}
	return string(first) == string(second)
}

func deps(version string) string {
	return `{"schemaVersion":1,"resolvedDependencies":{
		"package://pkg.pkl-lang.org/github.com/mdlsvensson/moonwell/moonwell@0":{"type":"local",
		"uri":"projectpackage://pkg.pkl-lang.org/github.com/mdlsvensson/moonwell/moonwell@` + version + `","path":"../schema"}}}`
}

// printing stands in for running pkl in both trees: every program prints the result.
func printing(result env.RunResult) (oldproc.RunFunc, env.RunFunc) {
	old := func(context.Context, string, []string, oldproc.Options) (oldproc.Result, error) {
		return oldproc.Result{Code: result.Code, Stdout: result.Stdout, Stderr: result.Stderr}, nil
	}
	return old, func(context.Context, string, []string, env.RunOptions) (env.RunResult, error) { return result, nil }
}

// load loads the project of a new folder with the files in both trees.
func load(t *testing.T, files map[string]string, result env.RunResult) (old *oldproject.Project, oldErr error, loaded *manifest.Project, err error) {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		testkit.WriteFile(t, root, name, []byte(content))
	}
	oldRun, run := printing(result)
	e, _ := testkit.Env(t, root)
	e.Run = run
	old, oldErr = oldproject.Load(context.Background(), root, "pkl", oldRun)
	loaded, err = manifest.Load(context.Background(), e, "pkl")
	return old, oldErr, loaded, err
}

func TestOracleOnWhatLoadRefuses(t *testing.T) {
	current := deps(moonwell.Version)
	project := map[string]string{"moonwell.pkl": "", "PklProject.deps.json": current}
	tests := []struct {
		name   string
		files  map[string]string
		result env.RunResult
	}{
		{"no manifest", map[string]string{"PklProject.deps.json": current}, env.RunResult{}},
		{"no dependencies file", map[string]string{"moonwell.pkl": ""}, env.RunResult{}},
		{"dependencies that are not JSON", map[string]string{"moonwell.local.pkl": "", "PklProject.deps.json": "{ not json"}, env.RunResult{}},
		{"dependencies without the package", map[string]string{"moonwell.pkl": "", "PklProject.deps.json": `{"resolvedDependencies":{}}`}, env.RunResult{}},
		{"dependencies that are a list", map[string]string{"moonwell.pkl": "", "PklProject.deps.json": `[]`}, env.RunResult{}},
		{"a package before the first install script", map[string]string{"moonwell.pkl": "", "PklProject.deps.json": deps("0.7.0")}, env.RunResult{}},
		{"a package with an install script", map[string]string{"moonwell.pkl": "", "PklProject.deps.json": deps("0.8.2")}, env.RunResult{}},
		{"a package of a later major version", map[string]string{"moonwell.pkl": "", "PklProject.deps.json": deps("1.0.0")}, env.RunResult{}},
		{"an evaluation that fails", project, env.RunResult{Code: 1, Stdout: "ignored", Stderr: "\n-- Pkl Error --\nType constraint violated\n\n"}},
		{"an evaluation that fails on the other stream", project, env.RunResult{Code: 1, Stdout: "  No such module \n"}},
		{"an evaluation that fails in silence", map[string]string{"moonwell.pkl": "", "moonwell.local.pkl": "", "PklProject.deps.json": current}, env.RunResult{Code: 3}},
		{"output that is not JSON", project, env.RunResult{Stdout: "map { }\n"}},
		{"no output", project, env.RunResult{}},
		{"long output that is not JSON", project, env.RunResult{Stdout: " " + strings.Repeat("0123456789", 60) + "\n"}},
	}
	refused := 0
	for _, tt := range tests {
		_, want, _, got := load(t, tt.files, tt.result)
		if oracle.Refusals(t, tt.name, want, got) {
			refused++
		}
	}
	if refused != len(tests) {
		t.Errorf("%d of %d loads were refused by both trees", refused, len(tests))
	}
}

func TestOracleOnWhatLoadReads(t *testing.T) {
	compared := 0
	for _, files := range []map[string]string{
		{"moonwell.pkl": "", "PklProject.deps.json": deps(moonwell.Version)},
		{"moonwell.pkl": "", "moonwell.local.pkl": "", "PklProject.deps.json": deps(moonwell.Version)},
	} {
		old, oldErr, loaded, err := load(t, files, env.RunResult{Stdout: everyPlainBlock, Stderr: "a warning"})
		if oldErr != nil || err != nil {
			t.Fatalf("a project was refused: the other tree says %v, this tree %v", oldErr, err)
		}
		oracle.Values(t, "a loaded project", plainOfOld(old), plainOfNew(loaded))
		compared++
	}
	if compared != 2 {
		t.Errorf("%d projects compared, want 2", compared)
	}
}

func TestOracleOnThePackageVersionAndTheRenderedFiles(t *testing.T) {
	compared := 0
	for _, document := range []string{
		deps("0.1.3"), deps("0.9.1-rc.1"), `{"resolvedDependencies":{}}`, `{}`, `[]`, `null`, `{ not json`, ``,
		`{"resolvedDependencies":{"package://x/moonwell@0":"1.0.0","package://y/moonwell@1":{"uri":"p://y/moonwell@1.2.3"}}}`,
		`{"resolvedDependencies":{"package://x/moonwell@0":{"uri":3},"package://x/other@0":{"uri":"p://x/other@0.9.1"}}}`,
	} {
		want, wantErr := oldproject.ReadPackageVersion(document)
		got, gotErr := manifest.ReadPackageVersion([]byte(document))
		oracle.Refusals(t, "the version in "+document, wantErr, gotErr)
		oracle.Values(t, "the version in "+document, want, got)
		compared++
	}
	for _, versions := range [][2]string{
		{"0.1.9", "0.1.0"}, {"0.7.0", "0.8.0"}, {"0.8.2", "0.9.0"}, {"0.9.1", "0.10.0"}, {"1.0.0", "0.9.1"},
		{"latest", "0.9.1"}, {"0.9", "0.9.1"}, {"", "0.9.1"}, {"0.x.1", "1.0.0"},
	} {
		what := "package " + versions[0] + " and program " + versions[1]
		oracle.Refusals(t, what, oldproject.CheckPackageVersion(versions[0], versions[1]), manifest.CheckPackageVersion(versions[0], versions[1]))
		compared++
	}
	files := []struct{ name, want, got string }{
		{"PklProject of the published package", oldproject.PklProject("0.9.1", ""), manifest.PklProject("0.9.1", "")},
		{"PklProject of a local package", oldproject.PklProject("", "../schema"), manifest.PklProject("", "../schema")},
		{"moonwell.local.pkl", oldproject.LocalPkl(), manifest.LocalPkl()},
		{"the install line", oldproject.InstallLine("0.8.2"), manifest.InstallLine("0.8.2")},
		{"the package address", oldproject.PackageBaseURI, manifest.PackageBaseURI},
		{"the game", oldproject.DefaultGameExecutable, manifest.DefaultGameExecutable},
	}
	for _, file := range files {
		oracle.Bytes(t, file.name, []byte(file.want), []byte(file.got))
		compared++
	}
	if compared != 10+9+len(files) {
		t.Errorf("%d comparisons, want %d", compared, 10+9+len(files))
	}
}
