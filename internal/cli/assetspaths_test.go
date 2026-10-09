package cli

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/assets"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func paths(e *env.Env, log *testkit.LogRecorder, file, gameList string) ([]string, error) {
	err := reportAssetPaths(background, e, file, assets.ParseGamePaths(gameList))
	return log.Lines(), err
}

func knight() []byte {
	return testkit.MDX(
		testkit.Chunk("TEXS", testkit.Concat(testkit.Texture("Textures/Knight.blp", 0), testkit.Texture("", 1))),
		testkit.Chunk("PREM", testkit.Emitter(`Abilities\Heal.mdx`, 0)),
	)
}

func TestOutsideAProjectAssetsPathsTellsInGamePathsFromCustomOnesShownWithBackslashes(t *testing.T) {
	root := t.TempDir()
	testkit.WriteFile(t, root, "knight.mdx", knight())
	e, log := testkit.Env(t, root)
	lines, err := paths(e, log, "knight.mdx", "# test\ntextures/knight.dds\n")
	want := []string{
		"knight.mdx",
		`  texture         Textures\Knight.blp   in-game path`,
		`  texture         team colour (slot 1)`,
		`  particle model  Abilities\Heal.mdx    custom path`,
		"1 model, 3 paths: 1 in-game, 1 custom.",
	}
	if err != nil || !slices.Equal(lines, want) {
		t.Errorf("assets:paths = %v; log =\n%s", err, strings.Join(lines, "\n"))
	}
	if exists(root, "dist") {
		t.Error("assets:paths outside a project made dist/")
	}
}

func TestAReforgedTifReferenceMatchesTheGamesDds(t *testing.T) {
	root := t.TempDir()
	testkit.WriteFile(t, root, "grass.mdx",
		testkit.MDX(testkit.Chunk("TEXS", testkit.Texture("Doodads/Corn/plant1_Normal.tif", 0))))
	e, log := testkit.Env(t, root)
	lines, err := paths(e, log, "grass.mdx", "doodads/corn/plant1_normal.dds\n")
	if err != nil || len(lines) != 3 || lines[1] != `  texture  Doodads\Corn\plant1_Normal.tif  in-game path` {
		t.Errorf("assets:paths = %v; log = %q", err, lines)
	}
}

func TestAnEmptyInGamePathListIsAnnounced(t *testing.T) {
	root := t.TempDir()
	testkit.WriteFile(t, root, "a.mdx", testkit.MDX(testkit.Chunk("TEXS", testkit.Texture(`Textures\A.blp`, 0))))
	e, log := testkit.Env(t, root)
	lines, err := paths(e, log, "a.mdx", "")
	if err != nil || len(lines) != 4 ||
		lines[0] != "warning: Moonwell's in-game path list is empty, so every path shows as custom." ||
		lines[1] != "a.mdx" || lines[3] != "1 model, 1 path: 0 in-game, 1 custom." {
		t.Errorf("assets:paths = %v; log = %q", err, lines)
	}
}

func TestOutsideAProjectAssetsPathsNeedsAFileThatExistsAndIsNotAFolder(t *testing.T) {
	root := t.TempDir()
	testkit.WriteFile(t, root, "notes.mdx", []byte("Model {\n}\nBroken {\n"))
	for _, c := range []struct {
		what, file string
		words      string
		named      string
	}{
		{what: "no file", file: "", words: "needs a model file outside a Moonwell project"},
		{what: "a file that is not there", file: "missing.mdx", words: "does not exist", named: "missing.mdx"},
		{what: "a folder", file: ".", words: "is a folder", named: "."},
		{what: "a file that is no model", file: "notes.mdx", words: "Not a readable model: ", named: "notes.mdx"},
	} {
		e, log := testkit.Env(t, root)
		lines, err := paths(e, log, c.file, "textures/knight.dds\n")
		failure := asError(t, err, c.what)
		if !strings.Contains(failure.Msg, c.words) || failure.Hint == "" || failure.File != c.named ||
			failure.Cause != nil {
			t.Errorf("%s: error = %+v", c.what, failure)
		}
		if len(lines) != 0 {
			t.Errorf("%s: assets:paths logged %q before it failed", c.what, lines)
		}
	}
}

func TestOutsideAProjectAssetsPathsNamesAFileItCannotRead(t *testing.T) {
	root := t.TempDir()
	testkit.MakeUnreadable(t, testkit.WriteFile(t, root, "held.mdx", knight()))
	e, log := testkit.Env(t, root)
	lines, err := paths(e, log, "held.mdx", "textures/knight.dds\n")
	failure := asError(t, err, "a file that cannot be read")
	if !strings.Contains(failure.Msg, "Reading the model failed: ") || failure.Hint == "" || failure.File != "held.mdx" ||
		failure.Cause == nil {
		t.Errorf("error = %+v", failure)
	}
	if len(lines) != 0 {
		t.Errorf("assets:paths logged %q before it failed", lines)
	}
}

func TestAssetsPathsNamesAModelFromTheFolderItRunsIn(t *testing.T) {
	root, elsewhere := t.TempDir(), t.TempDir()
	testkit.WriteFile(t, root, "units/knight.mdx", knight())
	testkit.WriteFile(t, root, "..knight.mdx", knight())
	outside := testkit.WriteFile(t, elsewhere, "knight.mdx", knight())
	for _, c := range []struct{ file, heading string }{
		{"units/knight.mdx", "units/knight.mdx"},
		{"..knight.mdx", "..knight.mdx"},
		{filepath.Join("units", "knight.mdx"), "units/knight.mdx"},
		{filepath.Join(root, "units", "knight.mdx"), "units/knight.mdx"},
		{outside, fsx.ToSlash(outside)},
	} {
		e, log := testkit.Env(t, root)
		lines, err := paths(e, log, c.file, "textures/knight.dds\n")
		if err != nil || len(lines) != 5 || lines[0] != c.heading {
			t.Errorf("%s: assets:paths = %v; log = %q, want the heading %q", c.file, err, lines, c.heading)
		}
	}
}

func TestTheAssetsPathsLineReportsOnTheFileItIsGiven(t *testing.T) {
	root := t.TempDir()
	testkit.WriteFile(t, root, "knight.mdx", knight())
	for _, args := range [][]string{{"assets:paths", "knight.mdx"}, {"assets:paths", "--", "knight.mdx"}} {
		r := ok(t, root, args...)
		if r.stdout != "" || !strings.HasPrefix(r.output, "knight.mdx\n  texture ") ||
			!strings.Contains(r.output, "\n1 model, 3 paths: ") || strings.Contains(r.output, "warning: ") {
			t.Errorf("%q: %+v", args, r)
		}
	}
	fails(t, root, []string{"error: assets:paths needs a model file outside a Moonwell project.",
		"\nhint: moonwell assets:paths assets/Models/Knight.mdx"}, "assets:paths", "")
	fails(t, root, []string{"error: ", "\nhint: "}, "assets:paths", "knight.mdx", "b.mdx")
	if entries := testkit.Snapshot(t, root); len(entries) != 1 {
		t.Errorf("assets:paths outside a project left %d entries there, want the model alone", len(entries))
	}
}

func TestPklLibraryAssetsPaths(t *testing.T) {
	root, _ := projectWithAssetLibrary(t)
	testkit.WriteFile(t, root, "assets/Models/Own.mdx",
		testkit.MDX(testkit.Chunk("TEXS", testkit.Texture(`textures\golem.BLP`, 0))))
	e, log, ran := pklOnly(t, root)
	lines, err := paths(e, log, "", "# test\n")
	want := []string{
		"warning: Moonwell's in-game path list is empty, so every path shows as custom.",
		"library golems: Models/Golem.mdx",
		`  texture  Textures\Golem.blp  custom path, imported`,
		"assets/Models/Own.mdx",
		`  texture  textures\golem.BLP  custom path, imported`,
		"2 models, 2 paths: 0 in-game, 2 custom imported, 0 custom not imported.",
	}
	if err != nil || !slices.Equal(lines, want) {
		t.Fatalf("assets:paths = %v; log =\n%s", err, strings.Join(lines, "\n"))
	}
	onlyPkl(t, ran)
	if !exists(root, ".moonwell/library-assets/golems/Models/Golem.mdx") || exists(root, "dist/.lock") {
		t.Error("assets:paths did not sync the library, or left the build lock behind")
	}
}

func TestPklAssetsPathsClassifiesAllReferences(t *testing.T) {
	root := newProject(t, "my-map")
	textures := testkit.Concat(testkit.Texture(`Textures\Knight.blp`, 0), testkit.Texture(`Textures\Cape.blp`, 0),
		testkit.Texture(`Textures\Missing.blp`, 0), testkit.Texture("", 1))
	emitters := testkit.Concat(testkit.Emitter(`Models\Glow.mdl`, 0), testkit.Emitter(`Models\Only.mdx`, 0))
	testkit.WriteFile(t, root, "assets/Models/Knight.mdx",
		testkit.MDX(testkit.Chunk("TEXS", textures), testkit.Chunk("PREM", emitters)))
	testkit.WriteFile(t, root, "assets/Textures/knight.BLP", []byte{1})
	testkit.WriteFile(t, root, "assets/art/cape.blp", []byte{2})
	testkit.WriteFile(t, root, "assets/Models/Glow.mdx", testkit.MDX())
	write(t, root, "assets/Models/Only.mdl", "Version {\n FormatVersion 800,\n}\n")
	edit(t, root, "moonwell.pkl", "paths {}", `paths { ["art/cape.blp"] = #"Textures\Cape.blp"# }`)
	const gameList = "# test\ntextures/knight.dds\ntextures/missing.blp\n"

	e, log, _ := pklOnly(t, root)
	lines, err := paths(e, log, "assets/Models/Knight.mdx", gameList)
	want := []string{
		"assets/Models/Knight.mdx",
		`  texture         Textures\Knight.blp   in-game path, replaced`,
		`  texture         Textures\Cape.blp     custom path, imported`,
		`  texture         Textures\Missing.blp  in-game path`,
		`  texture         team colour (slot 1)`,
		`  particle model  Models\Glow.mdl       custom path, imported`,
		`  particle model  Models\Only.mdx       custom path, not imported`,
		"1 model, 6 paths: 2 in-game, 2 custom imported, 1 custom not imported.",
	}
	if err != nil || !slices.Equal(lines, want) {
		t.Fatalf("assets:paths = %v; log =\n%s", err, strings.Join(lines, "\n"))
	}

	e, log, _ = pklOnly(t, root)
	lines, err = paths(e, log, "", gameList)
	all := slices.Concat([]string{"assets/Models/Glow.mdx", "  (no referenced files)"}, want[:7],
		[]string{"assets/Models/Only.mdl", "  (no referenced files)",
			"3 models, 6 paths: 2 in-game, 2 custom imported, 1 custom not imported."})
	if err != nil || !slices.Equal(lines, all) {
		t.Fatalf("assets:paths = %v; log =\n%s", err, strings.Join(lines, "\n"))
	}
}

func TestPklAssetsPathsReportsReadableModelsBeforeFailure(t *testing.T) {
	root := newProject(t, "my-map")
	write(t, root, "assets/Models/A.mdl", "Model {\n}\nBroken {\n")
	testkit.WriteFile(t, root, "assets/Models/B.mdx",
		testkit.MDX(testkit.Chunk("TEXS", testkit.Texture(`Textures\B.blp`, 0))))
	e, log, _ := pklOnly(t, root)
	lines, err := paths(e, log, "", "textures/b.blp\n")
	failure := asError(t, err, "a model that cannot be read")
	if !strings.Contains(failure.Msg, "1 model could not be read") || failure.File != "" ||
		!strings.Contains(failure.Hint, "assets/Models/A.mdl") {
		t.Errorf("error = %+v", failure)
	}
	want := []string{
		"assets/Models/A.mdl",
		"  (unreadable: Not a readable model: the Broken block is never closed.)",
		"assets/Models/B.mdx",
		`  texture  Textures\B.blp  in-game path`,
		"2 models, 1 path: 1 in-game, 0 custom imported, 0 custom not imported, 1 model unreadable.",
	}
	if !slices.Equal(lines, want) {
		t.Fatalf("log =\n%s", strings.Join(lines, "\n"))
	}
	r := failsWithPklAlone(t, root,
		[]string{"assets/Models/A.mdl\n  (unreadable: ", "\nerror: 1 model could not be read.\nhint: "}, "assets:paths")
	if !strings.HasSuffix(r.output, "the report above lists why each one is unreadable.") || r.stdout != "" {
		t.Errorf("%+v", r)
	}
}

func TestAReportWithModelsThatCouldNotBeReadFailsAndNamesEach(t *testing.T) {
	reports := []assets.ModelReport{
		{Heading: "assets/Models/A.mdl", Unreadable: "the Broken block is never closed"},
		{Heading: "assets/Models/B.mdx"},
		{Heading: "library golems: Models/C.mdx", Unreadable: "it is cut short"},
	}
	failure := asError(t, checkModelsReadable(reports), "two models that cannot be read")
	if !strings.Contains(failure.Msg, "2 models could not be read") ||
		!strings.Contains(failure.Hint, "assets/Models/A.mdl, library golems: Models/C.mdx;") {
		t.Errorf("error = %+v", failure)
	}
	if err := checkModelsReadable(reports[1:2]); err != nil {
		t.Errorf("a report whose models were all read fails: %v", err)
	}
}

func TestPklAssetsPathsOfAProjectWithoutModels(t *testing.T) {
	root := newProject(t, "my-map")
	write(t, root, "assets/icons/a.blp", "icon")
	e, log, _ := pklOnly(t, root)
	if lines, err := paths(e, log, "", ""); err != nil || !slices.Equal(lines, []string{"No models under assets/."}) {
		t.Errorf("assets:paths = %v; log = %q", err, lines)
	}
	if exists(root, "dist/.lock") {
		t.Error("assets:paths left the build lock behind")
	}
}

func TestPklAssetsPathsOfAFileInAProjectThatImportsNothing(t *testing.T) {
	root := newProject(t, "my-map")
	testkit.WriteFile(t, root, "drafts/knight.mdx", knight())
	e, log, _ := pklOnly(t, root)
	lines, err := paths(e, log, "drafts/knight.mdx", "textures/knight.dds\n")
	want := []string{
		"drafts/knight.mdx",
		`  texture         Textures\Knight.blp   in-game path`,
		`  texture         team colour (slot 1)`,
		`  particle model  Abilities\Heal.mdx    custom path, not imported`,
		"1 model, 3 paths: 1 in-game, 0 custom imported, 1 custom not imported.",
	}
	if err != nil || !slices.Equal(lines, want) {
		t.Errorf("assets:paths = %v; log =\n%s", err, strings.Join(lines, "\n"))
	}
}

func TestPklAssetsPathsIsRefusedBesideARunningBuild(t *testing.T) {
	root, _ := projectWithAssetLibrary(t)
	testkit.WriteFile(t, root, "knight.mdx", knight())
	holdBuildLock(t, root)
	for _, file := range []string{"", "knight.mdx"} {
		e, log, _ := pklOnly(t, root)
		lines, err := paths(e, log, file, "textures/knight.dds\n")
		failure := asError(t, err, "assets:paths beside a build")
		if failure.File != "dist/.lock" || failure.Hint == "" || len(lines) != 0 ||
			!strings.Contains(failure.Msg, "Another Moonwell build is running") {
			t.Errorf("%q: error = %+v; log = %q", file, failure, lines)
		}
	}
	if exists(root, ".moonwell") || !exists(root, "dist/.lock") {
		t.Error("a refused assets:paths synced the libraries, or removed the lock of the build")
	}
}
