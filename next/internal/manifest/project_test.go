package manifest

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
)

// The blocks pkl prints for every project, as a manifest that sets little prints them. plainBlocks leaves the
// nullable fields out, as pkl leaves out a null.
const (
	plainBlocks = `"map":{"folder":"map.w3x","entry":"src/main.yue"},"build":{"folder":"dist/bin","minify":false},
		"launch":{"args":["-launch"]},"yue":{"version":"0.34.3"}`
	assetsBlock    = `"assets":{"paths":{},"exclude":[]}`
	lintBlock      = `"lint":{"unknownGlobals":"error","globals":[]}`
	librariesBlock = `"libraries":{}`
)

// printed is what pkl prints for a manifest: the plain blocks, and the blocks given, each of which replaces the
// block of its name.
func printed(blocks ...string) string {
	all := []string{plainBlocks}
	for _, standard := range []string{assetsBlock, lintBlock, librariesBlock} {
		name, _, _ := strings.Cut(standard, ":")
		if !slices.ContainsFunc(blocks, func(block string) bool { return strings.HasPrefix(block, name+":") }) {
			all = append(all, standard)
		}
	}
	return "{" + strings.Join(append(all, blocks...), ",") + "}"
}

// decoded is the project of a document that must decode.
func decoded(t *testing.T, document, file string) *Project {
	t.Helper()
	project, err := Decode("/p", file, []byte(document))
	if err != nil {
		t.Fatalf("Decode(%s): %v", document, diag.Format(err))
	}
	return project
}

// asError is err as the expected failure it must be.
func asError(t *testing.T, err error, what string) *diag.Error {
	t.Helper()
	var failure *diag.Error
	if !errors.As(err, &failure) {
		t.Fatalf("%s: got %v, want a *diag.Error", what, err)
	}
	return failure
}

// text is what a pointer to a text holds, or "(nil)".
func text(value *string) string {
	if value == nil {
		return "(nil)"
	}
	return *value
}

func TestDecodeLeavesANullableFieldThatPklOmittedNil(t *testing.T) {
	p := decoded(t, printed(), "moonwell.pkl")
	if p.Root != "/p" || p.File != "moonwell.pkl" {
		t.Errorf("Root = %q, File = %q", p.Root, p.File)
	}
	if p.Map != (Map{Folder: "map.w3x", Entry: "src/main.yue"}) || p.Build != (Build{Folder: "dist/bin"}) {
		t.Errorf("map = %+v, build = %+v", p.Map, p.Build)
	}
	if p.Launch.GameExecutable != nil || !slices.Equal(p.Launch.Args, []string{"-launch"}) {
		t.Errorf("launch = %+v", p.Launch)
	}
	if p.Yue.Version != "0.34.3" || p.Yue.Path != nil {
		t.Errorf("yue = %+v", p.Yue)
	}
	if p.Assets.Paths.Len() != 0 || len(p.Assets.Exclude) != 0 || len(p.Libraries) != 0 || !p.Objects.Empty() {
		t.Errorf("assets = %+v, libraries = %+v, objects = %+v", p.Assets, p.Libraries, p.Objects)
	}
	if p.Lint.UnknownGlobals != "error" || len(p.Lint.Globals) != 0 {
		t.Errorf("lint = %+v", p.Lint)
	}
	if p.Settings.Info.Name != nil || len(p.Settings.Players) != 0 || p.Settings.GameplayConstants.Len() != 0 {
		t.Errorf("settings = %+v", p.Settings)
	}
}

func TestDecodeReadsEveryPlainBlock(t *testing.T) {
	p := decoded(t, `{"map":{"folder":"hero.w3x","entry":"src/game/init.yue"},
		"build":{"folder":"out","minify":true},
		"launch":{"gameExecutable":"C:/wc3.exe","args":["-launch","-windowmode","windowed"]},
		"yue":{"version":"0.34.2","path":"tools/yue"},
		"lint":{"unknownGlobals":"warning","globals":["MyLibrary"]}}`, "moonwell.local.pkl")
	if p.File != "moonwell.local.pkl" || p.Map != (Map{Folder: "hero.w3x", Entry: "src/game/init.yue"}) {
		t.Errorf("File = %q, map = %+v", p.File, p.Map)
	}
	if p.Build != (Build{Folder: "out", Minify: true}) {
		t.Errorf("build = %+v", p.Build)
	}
	if text(p.Launch.GameExecutable) != "C:/wc3.exe" || !slices.Equal(p.Launch.Args, []string{"-launch", "-windowmode", "windowed"}) {
		t.Errorf("launch = %q, %q", text(p.Launch.GameExecutable), p.Launch.Args)
	}
	if p.Yue.Version != "0.34.2" || text(p.Yue.Path) != "tools/yue" {
		t.Errorf("yue = %q, %q", p.Yue.Version, text(p.Yue.Path))
	}
	if p.Lint.UnknownGlobals != "warning" || !slices.Equal(p.Lint.Globals, []string{"MyLibrary"}) {
		t.Errorf("lint = %+v", p.Lint)
	}
}

func TestDecodeReadsLibrariesOfBothKindsAndListsTheirKeysSorted(t *testing.T) {
	p := decoded(t, printed(`"libraries":{
		"mine":{"path":"../mine","dir":""},
		"example":{"github":"mdlsvensson/moonwell-example-lib","tag":"v0.1.0","dir":"src"},
		"Zeta":{"path":"z","dir":""}}`), "moonwell.pkl")
	example, mine := p.Libraries["example"], p.Libraries["mine"]
	if text(example.GitHub) != "mdlsvensson/moonwell-example-lib" || text(example.Tag) != "v0.1.0" ||
		example.Path != nil || example.Dir != "src" {
		t.Errorf("example = %q, %q, %q, %q", text(example.GitHub), text(example.Tag), text(example.Path), example.Dir)
	}
	if mine.GitHub != nil || mine.Tag != nil || text(mine.Path) != "../mine" || mine.Dir != "" {
		t.Errorf("mine = %q, %q, %q, %q", text(mine.GitHub), text(mine.Tag), text(mine.Path), mine.Dir)
	}
	if got := p.LibraryKeys(); !slices.Equal(got, []string{"Zeta", "example", "mine"}) {
		t.Errorf("LibraryKeys = %q, want them sorted", got)
	}
	if got := decoded(t, printed(), "moonwell.pkl").LibraryKeys(); len(got) != 0 {
		t.Errorf("LibraryKeys of no libraries = %q", got)
	}
}

func TestDecodeReadsTheAssetsBlockInTheOrderItWasWritten(t *testing.T) {
	p := decoded(t, printed(`"assets":{"paths":{"b.blp":"Textures\\b.blp","a.blp":"Textures\\a.blp"},
		"exclude":["credits/"]}`), "moonwell.pkl")
	if got := entries(t, p.Assets.Paths); got != `b.blp=Textures\b.blp a.blp=Textures\a.blp` {
		t.Errorf("paths = %s", got)
	}
	if target, _ := p.Assets.Paths.Get("a.blp"); target != `Textures\a.blp` || !slices.Equal(p.Assets.Exclude, []string{"credits/"}) {
		t.Errorf("a.blp = %q, exclude = %q", target, p.Assets.Exclude)
	}
}

// A program reads the project of a package of its own minor version, which may print a field the program does not
// know, and always prints every block: a block that is missing is not given a default here.
func TestDecodeIgnoresAFieldItDoesNotKnowAndSuppliesNoDefault(t *testing.T) {
	p := decoded(t, `{"later":{"x":1},"map":{"folder":"map.w3x","later":true,"entry":"src/main.yue"},
		"build":{"folder":"dist/bin","minify":false,"later":[1]},"yue":{"version":"0.34.3"}}`, "moonwell.pkl")
	if p.Map != (Map{Folder: "map.w3x", Entry: "src/main.yue"}) || p.Build.Folder != "dist/bin" {
		t.Errorf("map = %+v, build = %+v", p.Map, p.Build)
	}
	if p.Lint.UnknownGlobals != "" || p.Lint.Globals != nil || p.Assets.Exclude != nil || p.Libraries != nil {
		t.Errorf("a missing block got a default: lint = %+v, assets = %+v, libraries = %+v", p.Lint, p.Assets, p.Libraries)
	}
}

func TestDecodeRefusesWhatIsNotShapedLikeAProjectWithOneError(t *testing.T) {
	const mapBlock, buildBlock = `"map":{"folder":"map.w3x","entry":"src/main.yue"}`, `"build":{"folder":"dist/bin","minify":false}`
	tests := []struct {
		name, document string
		words          []string // of the message
		without        []string // not in the message
	}{
		{"not an object", `[]`, []string{"array"}, nil},
		{"a block of another type", `{"map":3}`, []string{"number", "map"}, nil},
		{"a field of another type", `{"map":{"folder":3}}`, []string{"number", "map.folder"}, nil},
		{"a list of another type", `{"launch":{"args":[1]}}`, []string{"number", "launch.args"}, nil},
		{"a library of another type", `{"libraries":{"a":{"path":3}}}`, []string{"number", "libraries.a.path"}, nil},
		{"a player of another type", `{"settings":{"players":{"7":{"x":"1"}}}}`, []string{"string", "settings.players.7.x"}, nil},
		{"asset paths as a list", `{"assets":{"paths":[]}}`, []string{"array", "a mapping"}, nil},
		{"an asset path of another type", `{"assets":{"paths":{"a.blp":3}}}`, []string{"number", "a.blp: "}, nil},
		{"an object without a text for its id", `{"objects":{"units":{"a":{"id":3}}}}`, []string{"number", "a: id: "}, nil},
		{"an object that is a list", `{"objects":{"units":{"a":[]}}}`, []string{"array", "a: ", "a mapping"}, nil},
		{"a section that is not a mapping", `{"settings":{"gameInterface":{"Frame":"x"}}}`, []string{"string", "Frame: ", "a mapping"}, nil},
		{"not JSON", `map { }`, []string{"invalid character"}, nil},
		{"an empty document", `{}`, []string{"map.folder", "map.entry", "build.folder", "yue.version"}, nil},
		{"null", `null`, []string{"map.folder", "map.entry", "build.folder", "yue.version"}, nil},
		{"an empty map block", `{"map":{}}`, []string{"map.folder", "map.entry", "build.folder", "yue.version"}, nil},
		{"no yue block", `{` + mapBlock + `,` + buildBlock + `}`, []string{"yue.version"}, []string{"map.", "build."}},
		{"no entry", `{"map":{"folder":"map.w3x"},` + buildBlock + `,"yue":{"version":"0.34.3"}}`,
			[]string{"map.entry"}, []string{"map.folder", "build.", "yue."}},
		{"an empty build folder", `{` + mapBlock + `,"build":{"folder":""},"yue":{"version":"0.34.3"}}`,
			[]string{"build.folder"}, []string{"map.", "yue."}},
		{"another module's output", `{"heroes":{},"units":{"a":{"id":"h000","base":"hfoo"}}}`,
			[]string{"map.folder", "yue.version"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project, err := Decode("/p", "moonwell.local.pkl", []byte(tt.document))
			failure := asError(t, err, tt.document)
			if project != nil || failure.File != "moonwell.local.pkl" || !strings.Contains(failure.Msg, "moonwell.local.pkl") {
				t.Errorf("project = %v, error = %+v", project, failure)
			}
			for _, word := range tt.words {
				if !strings.Contains(failure.Msg, word) {
					t.Errorf("the message %q lacks %q", failure.Msg, word)
				}
			}
			// The reason is the decoder's, without the name of its package and without a type of this one.
			for _, word := range append([]string{"json:", "Ordered", "Object"}, tt.without...) {
				if strings.Contains(failure.Msg, word) {
					t.Errorf("the message %q has %q", failure.Msg, word)
				}
			}
			if !strings.Contains(failure.Hint, "@moonwell/Project.pkl") || !strings.Contains(failure.Hint, "PklProject") {
				t.Errorf("hint = %q", failure.Hint)
			}
		})
	}
}
