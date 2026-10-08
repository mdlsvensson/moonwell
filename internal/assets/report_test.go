package assets

import (
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/war3/model"
)

func textured(paths ...string) []byte {
	var entries []byte
	for _, path := range paths {
		entries = append(entries, testkit.Texture(path, 0)...)
	}
	return testkit.MDX(testkit.Chunk("TEXS", entries))
}

func statuses(report ModelReport) []PathStatus {
	list := []PathStatus{}
	for _, ref := range report.Refs {
		list = append(list, ref.Status)
	}
	return list
}

func headings(reports []ModelReport) []string {
	list := []string{}
	for _, report := range reports {
		list = append(list, report.Heading)
	}
	return list
}

func sameLines(t *testing.T, got, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("the report is\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestModelsAreTheAssetsImportedAsAModelNamedByWhereTheyComeFrom(t *testing.T) {
	assets := []Asset{
		{Source: "Models/Knight.mdx", Target: "Models/Knight.mdx", Bytes: []byte("knight")},
		{Source: "art/tree.bin", Target: "Doodads/Tree.MDL", Bytes: []byte("tree")},
		{Source: "Models/Golem.mdx", Library: "golems", Target: "Models/Golem.MdX", Bytes: []byte("golem")},
		{Source: "Models/Spare.mdx", Target: "spare/model.bin", Bytes: []byte("spare")},
		{Source: "Textures/a.blp", Target: "Textures/a.blp", Bytes: []byte("texture")},
		{Source: "notes.mdx.txt", Target: "notes.mdx.txt", Bytes: []byte("notes")},
		{Source: "mdx", Target: "mdx", Bytes: []byte("no extension")},
	}
	want := []Model{
		{"assets/Models/Knight.mdx", []byte("knight")},
		{"assets/art/tree.bin", []byte("tree")},
		{"library golems: Models/Golem.mdx", []byte("golem")},
	}
	got := Models(assets)
	if !slices.EqualFunc(got, want, func(a, b Model) bool { return a.Heading == b.Heading && string(a.Data) == string(b.Data) }) {
		t.Errorf("Models = %q", got)
	}
	if found := Models(assets[3:]); len(found) != 0 {
		t.Errorf("assets without a model have the models %q", found)
	}
}

func TestOutsideAProjectAReportTellsInGamePathsFromCustomOnesShownWithBackslashes(t *testing.T) {
	knight := Model{"knight.mdx", testkit.MDX(
		testkit.Chunk("TEXS", testkit.Concat(testkit.Texture("Textures/Knight.blp", 0), testkit.Texture("", 1))),
		testkit.Chunk("PREM", testkit.Emitter(`Abilities\Heal.mdx`, 0)),
	)}
	reports := ReportModels([]Model{knight}, ParseGamePaths("# test\ntextures/knight.dds\n"), nil)
	if len(reports) != 1 || reports[0].Heading != "knight.mdx" || reports[0].Unreadable != "" {
		t.Fatalf("ReportModels = %+v", reports)
	}
	if got := statuses(reports[0]); !slices.Equal(got, []PathStatus{InGame, "", Custom}) {
		t.Errorf("statuses = %q", got)
	}
	if first := reports[0].Refs[0]; first.Kind != model.Texture || first.Path.Path != "Textures/Knight.blp" {
		t.Errorf("the first reference is %+v, want the path as the model holds it", first)
	}
	sameLines(t, RenderReports(reports, false), []string{
		"knight.mdx",
		`  texture         Textures\Knight.blp   in-game path`,
		`  texture         team colour (slot 1)`,
		`  particle model  Abilities\Heal.mdx    custom path`,
		"1 model, 3 paths: 1 in-game, 1 custom.",
	})
}

func TestAReferenceIsClassifiedByTheGamesPathsAndInAProjectByWhatABuildImports(t *testing.T) {
	gamePaths := ParseGamePaths("doodads/corn/plant1_normal.dds\nunits/footman.mdx\n")
	none := map[string]bool{}
	tests := []struct {
		name      string
		reference string
		targets   map[string]bool
		want      PathStatus
	}{
		{"a Reforged .tif that the game stores as .dds", "Doodads/Corn/plant1_Normal.tif", nil, InGame},
		{"a path the game does not ship", `Textures\Mine.blp`, nil, Custom},
		{"a requested .mdl the game ships as .mdx", `Units\Footman.mdl`, nil, InGame},
		{"a game path in a project", `Units\Footman.mdx`, none, InGame},
		{"a game path that a build imports", `Units\Footman.mdx`, map[string]bool{"units/footman.mdx": true}, InGameReplaced},
		{"a custom path that a build imports", `Textures\Mine.BLP`, map[string]bool{"textures/mine.blp": true}, CustomImported},
		{"a custom path that no build imports", `Textures\Mine.blp`, map[string]bool{"textures/other.blp": true}, CustomNotImported},
		{"a project that imports nothing", `Textures\Mine.blp`, none, CustomNotImported},
		{"a requested .mdl imported as .mdx", `Models\Glow.mdl`, map[string]bool{"models/glow.mdx": true}, CustomImported},
		{"a requested .mdx imported as .mdl, which the game never loads", `Models\Only.mdx`, map[string]bool{"models/only.mdl": true}, CustomNotImported},
		{"a requested .mdl imported as .mdl", `Models\Only.mdl`, map[string]bool{"models/only.mdl": true}, CustomNotImported},
		{"a texture imported under another texture type", `Textures\Mine.blp`, map[string]bool{"textures/mine.dds": true}, CustomNotImported},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reports := ReportModels([]Model{{"a.mdx", textured(tt.reference)}}, gamePaths, tt.targets)
			if got := statuses(reports[0]); !slices.Equal(got, []PathStatus{tt.want}) {
				t.Errorf("statuses = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWithoutAnyGamePathsEveryPathIsCustom(t *testing.T) {
	reports := ReportModels([]Model{{"a.mdx", textured(`Textures\A.blp`)}}, map[string]bool{}, nil)
	if got := statuses(reports[0]); !slices.Equal(got, []PathStatus{Custom}) {
		t.Errorf("statuses = %q", got)
	}
}

func TestTheModelsALibraryShipsAreReportedAndItsFilesCountAsImported(t *testing.T) {
	root := t.TempDir()
	testkit.WriteFile(t, root, "libraries/golems/Models/Golem.mdx", textured(`Textures\Golem.blp`))
	put(t, root, "libraries/golems/Textures/Golem.blp")
	put(t, root, "libraries/golems/war3mapImported/golems/frames.toc")
	testkit.WriteFile(t, root, "assets/Models/Own.mdx", textured(`textures\golem.BLP`))
	assets, _ := collect(t, root, noBlock, "golems")

	reports := ReportModels(Models(assets), ParseGamePaths("# test\n"), Targets(assets))
	if got, want := headings(reports), []string{"library golems: Models/Golem.mdx", "assets/Models/Own.mdx"}; !slices.Equal(got, want) {
		t.Fatalf("the reports are of %q, want %q", got, want)
	}
	for _, report := range reports {
		if got := statuses(report); !slices.Equal(got, []PathStatus{CustomImported}) {
			t.Errorf("%s: statuses = %q", report.Heading, got)
		}
	}
	sameLines(t, RenderReports(reports, true), []string{
		"library golems: Models/Golem.mdx",
		`  texture  Textures\Golem.blp  custom path, imported`,
		"assets/Models/Own.mdx",
		`  texture  textures\golem.BLP  custom path, imported`,
		"2 models, 2 paths: 0 in-game, 2 custom imported, 0 custom not imported.",
	})
}

func TestInAProjectAReportClassifiesEveryReferenceOfEveryModel(t *testing.T) {
	root := t.TempDir()
	textures := testkit.Concat(testkit.Texture(`Textures\Knight.blp`, 0), testkit.Texture(`Textures\Cape.blp`, 0),
		testkit.Texture(`Textures\Missing.blp`, 0), testkit.Texture("", 1))
	emitters := testkit.Concat(testkit.Emitter(`Models\Glow.mdl`, 0), testkit.Emitter(`Models\Only.mdx`, 0))
	testkit.WriteFile(t, root, "assets/Models/Knight.mdx", testkit.MDX(testkit.Chunk("TEXS", textures), testkit.Chunk("PREM", emitters)))
	put(t, root, "assets/Textures/knight.BLP")
	put(t, root, "assets/art/cape.blp")
	testkit.WriteFile(t, root, "assets/Models/Glow.mdx", testkit.MDX())
	put(t, root, "assets/Models/Only.mdl", "Version {\n FormatVersion 800,\n}\n")
	assets, _ := collect(t, root, `{"paths":{"art/cape.blp":"Textures\\Cape.blp"},"exclude":[]}`)
	gamePaths := ParseGamePaths("# test\ntextures/knight.dds\ntextures/missing.blp\n")
	models := Models(assets)
	if len(models) != 3 || models[1].Heading != "assets/Models/Knight.mdx" {
		t.Fatalf("Models = %+v", models)
	}

	reports := ReportModels(models[1:2], gamePaths, Targets(assets))
	wanted := []PathStatus{InGameReplaced, CustomImported, InGame, "", CustomImported, CustomNotImported}
	if len(reports) != 1 || !slices.Equal(statuses(reports[0]), wanted) {
		t.Fatalf("ReportModels = %+v", reports)
	}
	lines := RenderReports(reports, true)
	if last := lines[len(lines)-1]; last != "1 model, 6 paths: 2 in-game, 2 custom imported, 1 custom not imported." {
		t.Errorf("the summary is %q", last)
	}

	reports = ReportModels(models, gamePaths, Targets(assets))
	if got, want := headings(reports), []string{"assets/Models/Glow.mdx", "assets/Models/Knight.mdx", "assets/Models/Only.mdl"}; !slices.Equal(got, want) {
		t.Fatalf("the reports are of %q, want %q", got, want)
	}
	if len(reports[0].Refs) != 0 || len(reports[2].Refs) != 0 || reports[0].Unreadable != "" || reports[2].Unreadable != "" {
		t.Errorf("the models without references are reported as %+v and %+v", reports[0], reports[2])
	}
	sameLines(t, RenderReports(reports, true), []string{
		"assets/Models/Glow.mdx",
		"  (no referenced files)",
		"assets/Models/Knight.mdx",
		`  texture         Textures\Knight.blp   in-game path, replaced`,
		`  texture         Textures\Cape.blp     custom path, imported`,
		`  texture         Textures\Missing.blp  in-game path`,
		`  texture         team colour (slot 1)`,
		`  particle model  Models\Glow.mdl       custom path, imported`,
		`  particle model  Models\Only.mdx       custom path, not imported`,
		"assets/Models/Only.mdl",
		"  (no referenced files)",
		"3 models, 6 paths: 2 in-game, 2 custom imported, 1 custom not imported.",
	})
}

func TestAModelThatCannotBeReadIsReportedInItsPlaceBesideTheOthers(t *testing.T) {
	root := t.TempDir()
	put(t, root, "assets/Models/A.mdl", "Model {\n}\nBroken {\n")
	testkit.WriteFile(t, root, "assets/Models/B.mdx", textured(`Textures\B.blp`))
	assets, _ := collect(t, root, noBlock)

	reports := ReportModels(Models(assets), ParseGamePaths("textures/b.blp\n"), Targets(assets))
	if got, want := headings(reports), []string{"assets/Models/A.mdl", "assets/Models/B.mdx"}; !slices.Equal(got, want) {
		t.Fatalf("the reports are of %q, want %q", got, want)
	}
	if broken := reports[0]; !strings.Contains(broken.Unreadable, "the Broken block is never closed") || len(broken.Refs) != 0 {
		t.Errorf("the model that cannot be read is reported as %+v", broken)
	}
	if reports[1].Unreadable != "" {
		t.Errorf("the model that can be read is reported as %+v", reports[1])
	}
	sameLines(t, RenderReports(reports, true), []string{
		"assets/Models/A.mdl",
		"  (unreadable: Not a readable model: the Broken block is never closed.)",
		"assets/Models/B.mdx",
		`  texture  Textures\B.blp  in-game path`,
		"2 models, 1 path: 1 in-game, 0 custom imported, 0 custom not imported, 1 model unreadable.",
	})
}

func TestTheSummaryCountsModelsPathsAndEachStatusAndSaysHowManyModelsAreUnreadable(t *testing.T) {
	ref := func(status PathStatus) ModelRef {
		return ModelRef{Path: model.Path{Kind: model.Texture, Path: "a.blp"}, Status: status}
	}
	slot := ModelRef{Path: model.Path{Kind: model.Texture, ReplaceableID: 2}}
	read := ModelReport{Heading: "a.mdx", Refs: []ModelRef{
		ref(InGame), ref(InGameReplaced), ref(CustomImported), ref(CustomNotImported), ref(CustomNotImported), ref(Custom), slot,
	}}
	broken := ModelReport{Heading: "b.mdx", Unreadable: "Not a readable model."}
	tests := []struct {
		name      string
		reports   []ModelReport
		inProject bool
		want      string
	}{
		{"no model", nil, false, "0 models, 0 paths: 0 in-game, 0 custom."},
		{"outside a project", []ModelReport{read}, false, "1 model, 7 paths: 2 in-game, 1 custom."},
		{"in a project", []ModelReport{read, read}, true, "2 models, 14 paths: 4 in-game, 2 custom imported, 4 custom not imported."},
		{"one path", []ModelReport{{Heading: "c.mdx", Refs: []ModelRef{ref(Custom)}}}, false, "1 model, 1 path: 0 in-game, 1 custom."},
		{"one unreadable outside a project", []ModelReport{broken}, false, "1 model, 0 paths: 0 in-game, 0 custom, 1 model unreadable."},
		{"two unreadable in a project", []ModelReport{broken, read, broken}, true,
			"3 models, 7 paths: 2 in-game, 1 custom imported, 2 custom not imported, 2 models unreadable."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := RenderReports(tt.reports, tt.inProject)
			if got := lines[len(lines)-1]; got != tt.want {
				t.Errorf("the summary is %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTheColumnsOfAReportArePaddedByCharacters(t *testing.T) {
	wide := Model{"wide.mdx", textured("Textures\\\xf0\x9f\x98\x80.blp", `Textures\Longer.blp`, "Textures\\\xc3\xa9.blp")}
	reports := ReportModels([]Model{wide}, map[string]bool{}, nil)
	sameLines(t, RenderReports(reports, false), []string{
		"wide.mdx",
		"  texture  Textures\\\xf0\x9f\x98\x80.blp       custom path",
		`  texture  Textures\Longer.blp  custom path`,
		"  texture  Textures\\\xc3\xa9.blp       custom path",
		"1 model, 3 paths: 0 in-game, 3 custom.",
	})
}
