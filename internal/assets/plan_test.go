package assets

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/war3/imp"
)

func sha256Of(text string) string { return fsx.SHA256Hex([]byte(text)) }

func TestWithNothingToImportAndNothingOwnedPlanChangesNothingAndReadsNothing(t *testing.T) {
	s := newAssetProject(t)
	writeFile(t, s.mapDir, "war3map.imp", "\x09\x09")
	writeFile(t, s.mapDir, "Textures/kept.blp")
	folder := s.openMap()
	for _, name := range []string{"war3map.imp", "Textures/kept.blp"} {
		file := filepath.Join(s.mapDir, filepath.FromSlash(name))
		if err := errors.Join(os.Remove(file), os.Mkdir(file, 0o777)); err != nil {
			t.Fatal(err)
		}
	}
	result, err := Plan(background, folder, nil, State{})
	if err != nil {
		t.Fatalf("Plan: %v", diag.Format(err))
	}
	if len(result.Assets) != 0 || len(result.Changes) != 0 || len(result.State.Files) != 0 {
		t.Errorf("Plan = %+v, want no asset, no change and nothing owned", result)
	}
}

func TestPlanWritesEachAssetAndTheIndexAndKeepsTheEditorsOwnImports(t *testing.T) {
	s := newAssetProject(t)
	const icon = "ReplaceableTextures/CommandButtons/BTNSword.blp"
	const disabled = "ReplaceableTextures/CommandButtonsDisabled/DISBTNSword.blp"
	writeFile(t, s.root, "assets/"+icon)
	writeFile(t, s.root, "assets/icons/disabled.blp", "disabled")
	writeFile(t, s.mapDir, "war3mapImported/existing.wav", "editor")
	s.writeImports(imp.Entry{Flag: 5, Path: "existing.wav"})
	before := testkit.Snapshot(t, s.root)

	_, result := s.mustPlan(`{"paths":{"icons/disabled.blp":"` + disabled + `"},"exclude":[]}`)
	if got, want := changePaths(result.Changes), []string{icon, disabled, "war3map.imp"}; !slices.Equal(got, want) {
		t.Fatalf("the changes are %q, want %q", got, want)
	}
	if got := string(result.Changes[1].Data); got != "disabled" {
		t.Errorf("the mapped asset is written as %q", got)
	}
	entries, err := imp.Read(result.Changes[2].Data, "war3map.imp")
	want := []imp.Entry{
		{Flag: 5, Path: "existing.wav"},
		{Flag: imp.CustomPath, Path: strings.ReplaceAll(icon, "/", `\`)},
		{Flag: imp.CustomPath, Path: strings.ReplaceAll(disabled, "/", `\`)},
	}
	if err != nil || !slices.Equal(entries, want) {
		t.Errorf("the index lists %+v, %v, want %+v", entries, err, want)
	}
	owned := []Owned{{icon, sha256Of("asset")}, {disabled, sha256Of("disabled")}}
	if !slices.Equal(result.State.Files, owned) {
		t.Errorf("the state is %+v, want %+v", result.State.Files, owned)
	}
	if got := rows(result.Assets); !slices.Equal(got, []row{{"", icon, icon}, {"", "icons/disabled.blp", disabled}}) {
		t.Errorf("the assets are %+v", got)
	}
	s.checkUnchanged(before, "planning")
}

func TestPlanChangesOnlyWhatDiffersAndRemovesOnlyOwnedFilesNoAssetWants(t *testing.T) {
	s := newAssetProject(t)
	for name, content := range map[string]string{
		"same.blp": "same", "edited.blp": "first", "dropped.blp": "dropped", "Sounds/dropped.wav": "dropped too",
		"unmanaged.txt": "keep",
	} {
		writeFile(t, s.mapDir, name, content)
	}
	s.writeOwnedState("same.blp", "edited.blp", "Sounds/dropped.wav", "dropped.blp", "gone.blp")
	s.writeImports(
		imp.Entry{Flag: 13, Path: "same.blp"}, imp.Entry{Flag: 13, Path: "edited.blp"},
		imp.Entry{Flag: 13, Path: `Sounds\dropped.wav`}, imp.Entry{Flag: 13, Path: "dropped.blp"},
		imp.Entry{Flag: 13, Path: "gone.blp"})
	for name, content := range map[string]string{"same.blp": "same", "edited.blp": "second", "gone.blp": "back", "new.blp": "new"} {
		writeFile(t, s.root, "assets/"+name, content)
	}

	_, result := s.mustPlan(noBlock)
	want := []string{"edited.blp", "gone.blp", "new.blp", "-Sounds/dropped.wav", "-dropped.blp", "war3map.imp"}
	if got := changePaths(result.Changes); !slices.Equal(got, want) {
		t.Errorf("the changes are %q, want %q", got, want)
	}
	owned := []Owned{{"edited.blp", sha256Of("second")}, {"gone.blp", sha256Of("back")}, {"new.blp", sha256Of("new")},
		{"same.blp", sha256Of("same")}}
	if !slices.Equal(result.State.Files, owned) {
		t.Errorf("the state is %+v, want %+v", result.State.Files, owned)
	}
}

func TestPlanKeepsAnImportOfTheMapsOwnByteForByteWithAMarkAtItsStart(t *testing.T) {
	const marked = "\xEF\xBB\xBFa.blp"
	s := newAssetProject(t)
	writeFile(t, s.mapDir, marked, "the map's own")
	s.writeImports(imp.Entry{Flag: 13, Path: marked})
	writeFile(t, s.root, "assets/b.blp")

	_, result := s.mustPlan(noBlock)
	if got, want := changePaths(result.Changes), []string{"b.blp", "war3map.imp"}; !slices.Equal(got, want) {
		t.Fatalf("the changes are %q, want %q", got, want)
	}
	want := imp.Write([]imp.Entry{{Flag: 13, Path: marked}, {Flag: imp.CustomPath, Path: "b.blp"}})
	if got := result.Changes[1].Data; !bytes.Equal(got, want) {
		t.Errorf("the index is %q, want %q", got, want)
	}
}

func TestPlanChangesNothingInAMapThatHoldsEveryAssetAndListsIt(t *testing.T) {
	s := newAssetProject(t)
	writeFile(t, s.root, "assets/Textures/a.blp")
	writeFile(t, s.mapDir, "Textures/a.blp")
	s.writeOwnedState("Textures/a.blp")
	s.writeImports(imp.Entry{Flag: 13, Path: `Textures\a.blp`})
	if _, result := s.mustPlan(noBlock); len(result.Changes) != 0 || len(result.State.Files) != 1 {
		t.Errorf("the changes are %q and the state %+v, want no change and one owned file",
			changePaths(result.Changes), result.State.Files)
	}
}

func TestAnOwnedImportKeepsTheFlagWorldEditorSavedItWith(t *testing.T) {
	s := newAssetProject(t)
	for _, name := range []string{"Textures/a.blp", "Textures/b.blp"} {
		writeFile(t, s.root, "assets/"+name)
		writeFile(t, s.mapDir, name)
	}
	s.writeOwnedState("Textures/a.blp", "Textures/b.blp")
	saved := []imp.Entry{{Flag: 29, Path: `Textures\a.blp`}, {Flag: 29, Path: `Textures\b.blp`}}
	s.writeImports(saved...)
	if _, result := s.mustPlan(noBlock); len(result.Changes) != 0 {
		t.Errorf("a map World Editor saved is changed: %q", changePaths(result.Changes))
	}
	writeFile(t, s.root, "assets/Textures/c.blp")
	_, result := s.mustPlan(noBlock)
	if got := changePaths(result.Changes); !slices.Equal(got, []string{"Textures/c.blp", "war3map.imp"}) {
		t.Fatalf("the changes are %q", got)
	}
	entries, err := imp.Read(result.Changes[1].Data, "war3map.imp")
	if want := append(saved, imp.Entry{Flag: imp.CustomPath, Path: `Textures\c.blp`}); err != nil || !slices.Equal(entries, want) {
		t.Errorf("the index lists %+v, %v, want %+v", entries, err, want)
	}
}

func TestAnOwnedImportThatWorldEditorSavedWithoutACustomPathKeepsThatFlag(t *testing.T) {
	s := newAssetProject(t)
	writeFile(t, s.root, "assets/war3mapImported/a.wav")
	writeFile(t, s.mapDir, "war3mapImported/a.wav")
	s.writeOwnedState("war3mapImported/a.wav")
	s.writeImports(imp.Entry{Flag: 5, Path: "a.wav"})
	_, result := s.mustPlan(noBlock)
	if got := changePaths(result.Changes); !slices.Equal(got, []string{"war3map.imp"}) {
		t.Fatalf("the changes are %q", got)
	}
	entries, err := imp.Read(result.Changes[0].Data, "war3map.imp")
	if want := []imp.Entry{{Flag: 5, Path: `war3mapImported\a.wav`}}; err != nil || !slices.Equal(entries, want) {
		t.Errorf("the index lists %+v, %v, want %+v", entries, err, want)
	}
}

func TestPlanSpellsFoldersAsTheMapDoesAndNewOnesAsTheFirstAssetToNameThem(t *testing.T) {
	s := newAssetProject(t)
	writeFile(t, s.mapDir, "Textures/existing.blp")
	for _, name := range []string{"textures/new.blp", "a.blp", "b.blp", "c.blp"} {
		writeFile(t, s.root, "assets/"+name)
	}
	block := `{"paths":{"a.blp":"Sound/Music/a.blp","b.blp":"sound/music/b.blp","c.blp":"SOUND/Effects/c.blp"},"exclude":[]}`
	_, result := s.mustPlan(block)
	spelled := []string{"SOUND/Effects/c.blp", "SOUND/Music/a.blp", "SOUND/Music/b.blp", "Textures/new.blp"}
	var targets, owned []string
	for _, asset := range result.Assets {
		targets = append(targets, asset.Target)
	}
	for _, file := range result.State.Files {
		owned = append(owned, file.Path)
	}
	if !slices.Equal(targets, spelled) || !slices.Equal(owned, spelled) {
		t.Errorf("the assets are written as %q and owned as %q, want %q", targets, owned, spelled)
	}
	if got := changePaths(result.Changes); !slices.Equal(got, append(spelled, "war3map.imp")) {
		t.Fatalf("the changes are %q", got)
	}
	entries, err := imp.Read(result.Changes[4].Data, "war3map.imp")
	if err != nil || len(entries) != 4 || entries[0].Path != `SOUND\Effects\c.blp` || entries[3].Path != `Textures\new.blp` {
		t.Errorf("the index lists %+v, %v", entries, err)
	}
}

func TestAnOwnedFileIsFoundAndWrittenUnderTheSpellingTheMapHas(t *testing.T) {
	s := newAssetProject(t)
	writeFile(t, s.mapDir, "models/UNIT.mdx", "first")
	writeFile(t, s.mapDir, "WAR3MAP.IMP", string(imp.Write([]imp.Entry{{Flag: 13, Path: `Models\Unit.mdx`}})))
	writeFile(t, s.root, ".asset-state/map.w3x.json", string(State{Files: []Owned{{`Models\Unit.mdx`, sha256Of("first")}}}.Encode()))
	writeFile(t, s.root, "assets/Models/Unit.mdx", "second")
	_, result := s.mustPlan(noBlock)
	if got := changePaths(result.Changes); !slices.Equal(got, []string{"models/UNIT.mdx", "WAR3MAP.IMP"}) {
		t.Errorf("the changes are %q", got)
	}
	if want := []Owned{{"models/UNIT.mdx", sha256Of("second")}}; !slices.Equal(result.State.Files, want) {
		t.Errorf("the state is %+v, want %+v", result.State.Files, want)
	}
}

func TestAnAssetAtAFileOrAnImportTheMapHasAndDoesNotOwnIsRefused(t *testing.T) {
	const own = "Import it under another path with assets.paths, or remove the map's own copy."
	tests := []struct {
		name       string
		asset      string
		library    string
		file       string
		entry      *imp.Entry
		msg, where string
		hint       string
	}{
		{"a file of the map", "a.blp", "", "a.blp", nil,
			"Asset a.blp conflicts with a file or import already in the map.", "a.blp", own},
		{"a file in another letter case", "textures/A.blp", "", "Textures/a.BLP", nil,
			"Asset textures/A.blp conflicts with a file or import already in the map.", "Textures/a.BLP", own},
		{"an import with a custom path and no file", "Textures/a.blp", "", "", &imp.Entry{Flag: 29, Path: `textures\A.blp`},
			"Asset Textures/a.blp conflicts with a file or import already in the map.", "Textures/a.blp", own},
		{"an import in the folder World Editor imports into", "war3mapImported/a.wav", "", "", &imp.Entry{Flag: 8, Path: "A.wav"},
			"Asset war3mapImported/a.wav conflicts with a file or import already in the map.", "war3mapImported/a.wav", own},
		{"a library's file", "Models/Golem.mdx", "ui", "Models/Golem.mdx", nil,
			"Asset Models/Golem.mdx of library ui conflicts with a file or import already in the map.", "Models/Golem.mdx",
			"Remove the map's own copy in World Editor's Import Manager."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newAssetProject(t)
			var libraries []string
			if tt.library == "" {
				writeFile(t, s.root, "assets/"+tt.asset)
			} else {
				writeFile(t, s.root, "libraries/"+tt.library+"/"+tt.asset)
				libraries = []string{tt.library}
			}
			if tt.file != "" {
				writeFile(t, s.mapDir, tt.file, "the editor's")
			}
			if tt.entry != nil {
				s.writeImports(*tt.entry)
			}
			before := testkit.Snapshot(t, s.root)
			e := s.mustFailPlan(noBlock, libraries...)
			if e.Msg != tt.msg || e.File != mapLabel+"/"+tt.where || e.Hint != tt.hint {
				t.Errorf("error = %+v, want %q at %s", e, tt.msg, tt.where)
			}
			s.checkUnchanged(before, "a refused plan")
		})
	}
}

func TestAnOwnedFileEditedInTheMapIsRefusedAlsoWhenNoAssetWantsIt(t *testing.T) {
	s := newAssetProject(t)
	writeFile(t, s.root, ".asset-state/map.w3x.json", string(State{Files: []Owned{{"textures/A.BLP", sha256Of("asset")}}}.Encode()))
	writeFile(t, s.mapDir, "Textures/a.blp", "manual edit")
	for _, asset := range []string{"assets/Textures/a.blp", ""} {
		if asset != "" {
			writeFile(t, s.root, asset, "a newer asset")
		}
		e := s.mustFailPlan(noBlock)
		if e.Msg != "Textures/a.blp was modified in the map after assets:sync wrote it." ||
			e.File != mapLabel+"/Textures/a.blp" || !strings.Contains(e.Hint, "source map") {
			t.Errorf("with the asset %q: error = %+v", asset, e)
		}
		if err := os.RemoveAll(filepath.Join(s.root, "assets", "Textures")); err != nil {
			t.Fatal(err)
		}
	}
	if got := s.readMapFile("Textures/a.blp"); got != "manual edit" {
		t.Errorf("a refused plan left %q in the edited file", got)
	}
}

func TestAnAssetBelowAFileOfTheMapIsRefusedAlsoWhenTheFileIsOwned(t *testing.T) {
	const own = "Import it under another path with assets.paths, or remove "
	tests := []struct {
		name                 string
		files                []string
		owned                []string
		asset, target        string
		library              bool
		inTheWay, wantsToBe  string
		hintStart, hintWords string
	}{
		{"a file of the map", []string{"Textures"}, nil, "a.blp", "textures/a.blp", false, "Textures", "textures/a.blp", own, "Textures"},
		{"a file in a folder", []string{"Units/Hero"}, nil, "a.blp", "units/hero/skins/a.blp", false,
			"Units/Hero", "units/hero/skins/a.blp", own, "Units/Hero"},
		{"an owned file that no asset wants", []string{"data"}, []string{"data"}, "data/inner.txt", "", false,
			"data", "data/inner.txt", own, "data"},
		{"a library's file", []string{"UI"}, nil, "ui/frame.fdf", "", true, "UI", "ui/frame.fdf", "Remove UI", "source map"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newAssetProject(t)
			for _, file := range tt.files {
				writeFile(t, s.mapDir, file)
			}
			s.writeOwnedState(tt.owned...)
			block, libraries := noBlock, []string(nil)
			switch {
			case tt.library:
				writeFile(t, s.root, "libraries/lib/"+tt.asset)
				libraries = []string{"lib"}
			case tt.target != "":
				writeFile(t, s.root, "assets/"+tt.asset)
				block = `{"paths":{"` + tt.asset + `":"` + tt.target + `"},"exclude":[]}`
			default:
				writeFile(t, s.root, "assets/"+tt.asset)
			}
			e := s.mustFailPlan(block, libraries...)
			msg := tt.inTheWay + " in the map is a file, not a directory, so " + tt.wantsToBe + " cannot go there."
			if e.Msg != msg || e.File != mapLabel+"/"+tt.inTheWay || !strings.HasPrefix(e.Hint, tt.hintStart) ||
				!strings.Contains(e.Hint, tt.hintWords) {
				t.Errorf("error = %+v, want %q at %s", e, msg, tt.inTheWay)
			}
		})
	}
}

func TestAnAssetNamedAsAFolderOfTheMapIsRefused(t *testing.T) {
	empty := func(s *assetProject) {
		if err := os.Mkdir(filepath.Join(s.mapDir, "Textures"), 0o777); err != nil {
			s.t.Fatal(err)
		}
	}
	tests := []struct {
		name  string
		setup func(s *assetProject)
	}{
		{"a folder with a file in it", func(s *assetProject) { writeFile(s.t, s.mapDir, "Textures/a.blp") }},
		{"an empty folder", empty},
		{"a folder where the state lists a file", func(s *assetProject) {
			empty(s)
			writeFile(s.t, s.root, ".asset-state/map.w3x.json", string(State{Files: []Owned{{"textures", zeros}}}.Encode()))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newAssetProject(t)
			tt.setup(s)
			writeFile(t, s.root, "assets/textures")
			e := s.mustFailPlan(noBlock)
			if e.Msg != "Asset textures would replace a folder in the map." || e.File != mapLabel+"/Textures" ||
				!strings.Contains(e.Hint, "remove that folder") {
				t.Errorf("error = %+v", e)
			}
		})
	}
}

func TestTheFirstAssetWithoutRoomIsTheOneRefused(t *testing.T) {
	s := newAssetProject(t)
	writeFile(t, s.mapDir, "a")
	writeFile(t, s.mapDir, "b.blp")
	writeFile(t, s.root, "assets/a/inner.blp")
	writeFile(t, s.root, "assets/b.blp")
	if e := s.mustFailPlan(noBlock); !strings.Contains(e.Msg, "a in the map is a file") {
		t.Errorf("error = %+v, want it about a/inner.blp, the first asset", e)
	}
	block := `{"paths":{"b.blp":"0.blp"},"exclude":[]}`
	writeFile(t, s.mapDir, "0.blp")
	if e := s.mustFailPlan(block); !strings.Contains(e.Msg, "Asset 0.blp conflicts") {
		t.Errorf("error = %+v, want it about 0.blp, the first asset", e)
	}
}

func TestPlanRefusesAnIndexOfImportsItCannotUse(t *testing.T) {
	two := imp.Write([]imp.Entry{{Flag: 13, Path: `Textures\x.blp`}, {Flag: 5, Path: "y.wav"}, {Flag: 29, Path: `textures/X.BLP`}})
	tests := []struct {
		name  string
		index []byte
		words string
	}{
		{"a file that is no index", []byte{9, 9}, "war3map.imp is unreadable: it is truncated."},
		{"a path listed twice", two, `war3map.imp lists textures/X.BLP twice.`},
		{"a path that leaves the map", imp.Write([]imp.Entry{{Flag: 13, Path: `..\x.blp`}}), `Invalid asset path: ..\x.blp`},
		{"a path no file can have, in the folder World Editor imports into", imp.Write([]imp.Entry{{Flag: 8, Path: `a?.wav`}}),
			`Invalid asset path: war3mapImported\a?.wav`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newAssetProject(t)
			writeFile(t, s.root, "assets/a.blp")
			testkit.WriteFile(t, s.mapDir, "War3map.imp", tt.index)
			e := s.mustFailPlan(noBlock)
			if e.Msg != tt.words || e.File != mapLabel+"/War3map.imp" || e.Hint == "" {
				t.Errorf("error = %+v, want %q at the map's War3map.imp", e, tt.words)
			}
		})
	}
}

func TestAFolderNamedAsTheIndexOfImportsIsRefusedBeforeAnythingIsPlanned(t *testing.T) {
	s := newAssetProject(t)
	writeFile(t, s.root, "assets/a.blp")
	writeFile(t, s.mapDir, "War3Map.imp/stray.txt")
	before := testkit.Snapshot(t, s.root)
	e := s.mustFailPlan(noBlock)
	if !strings.Contains(e.Msg, "War3Map.imp in the map is a folder") || e.File != mapLabel+"/War3Map.imp" ||
		!strings.Contains(e.Hint, "Remove that folder") || strings.Contains(e.Hint, "another path") {
		t.Errorf("error = %+v, want the folder named as the map spells it, and a hint that fits the index", e)
	}
	s.checkUnchanged(before, "a refused plan")
}

func TestPlanStopsAtAnInterruptBetweenFilesAndWritesNothing(t *testing.T) {
	s := newAssetProject(t)
	writeFile(t, s.root, "assets/a.blp")
	writeFile(t, s.root, "assets/b.blp")
	writeFile(t, s.mapDir, "owned.blp")
	s.writeOwnedState("owned.blp")
	before := testkit.Snapshot(t, s.root)
	const asks = 4
	for limit := range asks {
		ctx := &cancelAfterCtx{Context: background, limit: limit}
		_, result, err := s.tryPlan(ctx, noBlock)
		if e := asDiagError(t, err, "an interrupted plan"); e.Msg != "Interrupted; nothing was written." || result != nil {
			t.Errorf("cancelled at ask %d: Plan = %+v, %+v", limit+1, result, e)
		}
		if ctx.calls != limit+1 {
			t.Errorf("cancelled at ask %d: Plan asked %d times, want it to stop at the ask that was refused", limit+1, ctx.calls)
		}
	}
	ctx := &cancelAfterCtx{Context: background, limit: asks}
	if _, _, err := s.tryPlan(ctx, noBlock); err != nil || ctx.calls != asks {
		t.Errorf("Plan = %v after %d asks, want a plan after %d", err, ctx.calls, asks)
	}
	s.checkUnchanged(before, "planning")
}

func TestPlanRefusesAPathNoAssetMayHaveAsACallersBug(t *testing.T) {
	script := Asset{Source: "x.lua", Target: "war3map.lua", Data: []byte("x"), Hash: sha256Of("x")}
	tests := []struct {
		name   string
		assets []Asset
		owned  State
		words  string
	}{
		{"an asset as the map's script", []Asset{script}, State{}, `an asset has the in-map path "war3map.lua"`},
		{"an asset outside the map", []Asset{{Target: "../x.blp"}}, State{}, `an asset has the in-map path "../x.blp"`},
		{"two assets at one path", []Asset{{Target: "Textures/a.blp"}, {Target: `textures\A.blp`}}, State{},
			`two assets have the in-map path "textures\\A.blp"`},
		{"the map's script as an owned file", nil, State{Files: []Owned{{"war3map.lua", sha256Of("the script")}}},
			`an owned file has the in-map path "war3map.lua"`},
		{"the map's script in its folder, as an owned file", nil, State{Files: []Owned{{`Scripts\war3map.j`, zeros}}},
			`an owned file has the in-map path "Scripts\\war3map.j"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newAssetProject(t)
			writeFile(t, s.mapDir, "war3map.lua", "the script")
			result, err := Plan(background, s.openMap(), tt.assets, tt.owned)
			var expected *diag.Error
			if err == nil || errors.As(err, &expected) || !strings.Contains(err.Error(), tt.words) || result != nil {
				t.Errorf("Plan = %+v, %v, want a plain error that says %q", result, err, tt.words)
			}
		})
	}
}

func TestAMapFileThePlanNeedsAndCannotReadIsRefusedByItsName(t *testing.T) {
	for _, held := range []string{"Textures/owned.blp", "war3map.imp"} {
		t.Run(held, func(t *testing.T) {
			s := newAssetProject(t)
			writeFile(t, s.mapDir, "Textures/owned.blp")
			s.writeOwnedState("Textures/owned.blp")
			s.writeImports(imp.Entry{Flag: 13, Path: `Textures\owned.blp`})
			testkit.MakeUnreadable(t, filepath.Join(s.mapDir, filepath.FromSlash(held)))
			e := s.mustFailPlan(noBlock)
			if !strings.HasPrefix(e.Msg, "Reading a map file failed") || e.File != mapLabel+"/"+held || e.Cause == nil {
				t.Errorf("error = %+v", e)
			}
		})
	}
}

func TestThePlanLaidOverAViewIsStagedAndTheSourceMapIsLeftAlone(t *testing.T) {
	s := newAssetProject(t)
	writeFile(t, s.mapDir, "Models/unit.mdx", "first")
	writeFile(t, s.mapDir, "Models/dropped.mdx", "dropped")
	writeFile(t, s.mapDir, "unmanaged.txt", "keep")
	writeFile(t, s.mapDir, "war3map.w3i", "info")
	s.writeOwnedState("Models/unit.mdx", "Models/dropped.mdx")
	s.writeImports(imp.Entry{Flag: 13, Path: `Models\unit.mdx`}, imp.Entry{Flag: 13, Path: `Models\dropped.mdx`})
	writeFile(t, s.root, "assets/models/unit.mdx", "second")
	writeFile(t, s.root, "assets/Sound/theme.mp3", "theme")
	before := testkit.Snapshot(t, s.root)

	folder, result := s.mustPlan(noBlock)
	view := folder.WithChanges([]mapdir.Change{{Path: "war3map.w3i", Data: []byte("patched")}}).WithChanges(result.Changes)
	stage := filepath.Join(t.TempDir(), "stage", "map.w3x")
	if err := view.StageTo(stage); err != nil {
		t.Fatalf("StageTo: %v", diag.Format(err))
	}
	s.checkUnchanged(before, "a build")

	staged := map[string]string{}
	for name, data := range testkit.Snapshot(t, stage) {
		if data != nil {
			staged[name] = string(data)
		}
	}
	index := string(imp.Write([]imp.Entry{{Flag: 13, Path: `Models\unit.mdx`}, {Flag: 13, Path: `Sound\theme.mp3`}}))
	want := map[string]string{"Models/unit.mdx": "second", "Sound/theme.mp3": "theme", "unmanaged.txt": "keep",
		"war3map.w3i": "patched", "war3map.imp": index}
	for name, content := range want {
		if staged[name] != content {
			t.Errorf("the stage holds %q under %s, want %q", staged[name], name, content)
		}
	}
	if len(staged) != len(want) {
		t.Errorf("the stage holds %d files, want %d: %v", len(staged), len(want), staged)
	}
}
