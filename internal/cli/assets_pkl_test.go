package cli_test

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/assets"
	"github.com/mdlsvensson/moonwell/internal/cli"
	"github.com/mdlsvensson/moonwell/internal/models"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func TestPklAssetsCheckAndSync(t *testing.T) {
	root := newProject(t, "my-map")
	write(t, root, "assets/icons/a.blp", "icon")
	env, log := newEnv(root)
	plan, err := cli.Assets(background, env, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Assets) != 1 || plan.Assets[0].Target != "icons/a.blp" {
		t.Fatal(plan.Assets)
	}
	var changes []string
	for _, line := range log.Lines {
		if strings.HasPrefix(line, "write ") || strings.HasPrefix(line, "delete ") {
			changes = append(changes, line)
		}
	}
	if !reflect.DeepEqual(changes, []string{"write maps/map.w3x/icons/a.blp", "write maps/map.w3x/war3map.imp"}) {
		t.Fatal(changes)
	}
	if exists(root, "maps/map.w3x/icons/a.blp") || exists(root, ".asset-state") {
		t.Fatal("check wrote into source map")
	}
	if _, err = cli.Assets(background, env, true); err != nil {
		t.Fatal(err)
	}
	if read(t, root, "maps/map.w3x/icons/a.blp") != "icon" {
		t.Fatal("asset bytes")
	}
	imports, err := assets.ReadImports([]byte(read(t, root, "maps/map.w3x/war3map.imp")), "war3map.imp")
	if err != nil || len(imports) != 1 || imports[0].Path != `icons\a.blp` {
		t.Fatalf("%v %+v", err, imports)
	}
	contains(t, read(t, root, ".asset-state/map.w3x.json"), "icons/a.blp")
	plan, err = cli.Assets(background, env, false)
	if err != nil || len(plan.Changes) != 0 {
		t.Fatalf("%v %+v", err, plan)
	}
}

func TestPklAssetsInterruptedSyncWritesNothing(t *testing.T) {
	root := newProject(t, "my-map")
	write(t, root, "assets/icons/a.blp", "icon")
	before := testkit.Snapshot(t, filepath.Join(root, "maps"))
	ctx, cancel := context.WithCancel(background)
	cancel()
	env, _ := newEnv(root)
	_, err := cli.Assets(ctx, env, true)
	if err == nil {
		t.Fatal("cancelled sync succeeded")
	}
	sameFiles(t, before, testkit.Snapshot(t, filepath.Join(root, "maps")), "interrupted sync")
	if exists(root, ".asset-state") {
		t.Fatal("ownership written")
	}
}

func projectWithAssetLibrary(t *testing.T) (root, library, plain string) {
	t.Helper()
	root = newProject(t, "my-map")
	library = filepath.Join(filepath.Dir(root), "golems")
	write(t, library, "moonwell-library.json", `{"dir":"src","assets":"assets"}`)
	write(t, library, "src/golems/spawn.lua", "return {}\n")
	testkit.WriteFile(t, library, "assets/Models/Golem.mdx", testkit.MDX(testkit.Chunk("TEXS", testkit.Texture(`Textures\Golem.blp`, 0))))
	testkit.WriteFile(t, library, "assets/Textures/Golem.blp", []byte{1})
	write(t, library, "assets/war3mapImported/golems/frames.toc", "war3mapImported\\golems\\frames.fdf\n")
	plain = read(t, root, "moonwell.pkl")
	edit(t, root, "moonwell.pkl", "libraries {\n", "libraries {\n  [\"golems\"] { path = \""+filepath.ToSlash(library)+"\" }\n")
	return
}

func TestPklLibraryAssetsSyncAndRemoval(t *testing.T) {
	root, _, plain := projectWithAssetLibrary(t)
	testkit.WriteFile(t, root, "assets/Textures/golem.blp", []byte{2})
	env, log := newEnv(root)
	plan, err := cli.Assets(background, env, false)
	if err != nil {
		t.Fatal(err)
	}
	var targets, owners []string
	for _, a := range plan.Assets {
		targets = append(targets, a.Target)
		owners = append(owners, a.Library)
	}
	if !reflect.DeepEqual(targets, []string{"Models/Golem.mdx", "Textures/golem.blp", "war3mapImported/golems/frames.toc"}) || !reflect.DeepEqual(owners, []string{"golems", "", "golems"}) {
		t.Fatalf("%v %v", targets, owners)
	}
	contains(t, strings.Join(log.Lines, "\n"), "library golems: Models/Golem.mdx -> Models\\Golem.mdx", "assets/Textures/golem.blp replaces library golems's Textures/Golem.blp")
	if exists(root, "maps/map.w3x/Models") || !exists(root, ".moonwell/library-assets/golems/Models/Golem.mdx") {
		t.Fatal("check staging")
	}
	if _, err = cli.Assets(background, env, true); err != nil {
		t.Fatal(err)
	}
	if read(t, root, "maps/map.w3x/Textures/golem.blp") != string([]byte{2}) || read(t, root, "maps/map.w3x/war3mapImported/golems/frames.toc") != "war3mapImported\\golems\\frames.fdf\n" {
		t.Fatal("library bytes")
	}
	write(t, root, "moonwell.pkl", plain)
	if _, err = cli.Assets(background, env, true); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"maps/map.w3x/Models/Golem.mdx", "maps/map.w3x/war3mapImported/golems/frames.toc", ".moonwell/library-assets/golems"} {
		if exists(root, name) {
			t.Error(name)
		}
	}
	imports, err := assets.ReadImports([]byte(read(t, root, "maps/map.w3x/war3map.imp")), "war3map.imp")
	if err != nil || len(imports) != 1 || imports[0].Path != `Textures\golem.blp` {
		t.Fatalf("%v %+v", err, imports)
	}
}

func TestPklLibraryAssetsPaths(t *testing.T) {
	root, _, _ := projectWithAssetLibrary(t)
	testkit.WriteFile(t, root, "assets/Models/Own.mdx", testkit.MDX(testkit.Chunk("TEXS", testkit.Texture(`textures\golem.BLP`, 0))))
	env, _ := newEnv(root)
	reports, err := cli.AssetsPaths(background, env, "", models.ParseGamePaths("# test\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 2 || reports[0].Heading != "library golems: Models/Golem.mdx" || reports[1].Heading != "assets/Models/Own.mdx" {
		t.Fatal(reports)
	}
	for _, report := range reports {
		if len(report.Refs) != 1 || report.Refs[0].Status != "custom path, imported" {
			t.Fatal(report)
		}
	}
}

func TestPklAssetsPathsClassifiesAllReferences(t *testing.T) {
	root := newProject(t, "my-map")
	tex := testkit.Concat(testkit.Texture(`Textures\Knight.blp`, 0), testkit.Texture(`Textures\Cape.blp`, 0), testkit.Texture(`Textures\Missing.blp`, 0), testkit.Texture("", 1))
	testkit.WriteFile(t, root, "assets/Models/Knight.mdx", testkit.MDX(testkit.Chunk("TEXS", tex), testkit.Chunk("PREM", testkit.Concat(testkit.Emitter(`Models\Glow.mdl`, 0), testkit.Emitter(`Models\Only.mdx`, 0)))))
	testkit.WriteFile(t, root, "assets/Textures/knight.BLP", []byte{1})
	testkit.WriteFile(t, root, "assets/art/cape.blp", []byte{2})
	testkit.WriteFile(t, root, "assets/Models/Glow.mdx", testkit.MDX())
	write(t, root, "assets/Models/Only.mdl", "Version {\n FormatVersion 800,\n}\n")
	edit(t, root, "moonwell.pkl", "paths {}", `paths { ["art/cape.blp"] = #"Textures\Cape.blp"# }`)
	env, log := newEnv(root)
	options := models.ParseGamePaths("# test\ntextures/knight.dds\ntextures/missing.blp\n")
	reports, err := cli.AssetsPaths(background, env, "assets/Models/Knight.mdx", options)
	if err != nil {
		t.Fatal(err)
	}
	wanted := []string{"in-game path, replaced", "custom path, imported", "in-game path", "", "custom path, imported", "custom path, not imported"}
	if len(reports) != 1 || len(reports[0].Refs) != len(wanted) {
		t.Fatal(reports)
	}
	for i, ref := range reports[0].Refs {
		if string(ref.Status) != wanted[i] {
			t.Fatalf("%d: %+v", i, ref)
		}
	}
	if log.Lines[len(log.Lines)-1] != "1 model, 6 paths: 2 in-game, 2 custom imported, 1 custom not imported." {
		t.Fatal(log.Lines)
	}
	reports, err = cli.AssetsPaths(background, env, "", options)
	if err != nil || len(reports) != 3 || reports[0].Heading != "assets/Models/Glow.mdx" || reports[2].Heading != "assets/Models/Only.mdl" || len(reports[0].Refs) != 0 || len(reports[2].Refs) != 0 {
		t.Fatalf("%v %+v", err, reports)
	}
	contains(t, strings.Join(log.Lines, "\n"), "  (no referenced files)")
}

func TestPklAssetsPathsReportsReadableModelsBeforeFailure(t *testing.T) {
	root := newProject(t, "my-map")
	write(t, root, "assets/Models/A.mdl", "Model {\n}\nBroken {\n")
	testkit.WriteFile(t, root, "assets/Models/B.mdx", testkit.MDX(testkit.Chunk("TEXS", testkit.Texture(`Textures\B.blp`, 0))))
	env, log := newEnv(root)
	_, err := cli.AssetsPaths(background, env, "", models.ParseGamePaths("textures/b.blp\n"))
	e := asError(t, err, "models")
	contains(t, e.Msg, "1 model could not be read")
	contains(t, e.Hint, "assets/Models/A.mdl")
	wanted := []string{"assets/Models/A.mdl", "  (unreadable: Not a readable model: the Broken block is never closed.)", "assets/Models/B.mdx", `  texture  Textures\B.blp  in-game path`, "2 models, 1 path: 1 in-game, 0 custom imported, 0 custom not imported, 1 model unreadable."}
	if !reflect.DeepEqual(log.Lines, wanted) {
		t.Fatalf("%q", log.Lines)
	}
}
