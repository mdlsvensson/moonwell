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

// hashed is the hash of a file that holds text.
func hashed(text string) string { return fsx.SHA256Hex([]byte(text)) }

func TestWithNothingToImportAndNothingOwnedPlanChangesNothingAndReadsNothing(t *testing.T) {
	s := newSite(t)
	put(t, s.mapDir, "war3map.imp", "\x09\x09")
	put(t, s.mapDir, "Textures/kept.blp")
	folder := s.open()
	// A folder where the scan found a file: a read of either file fails.
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
	s := newSite(t)
	const icon = "ReplaceableTextures/CommandButtons/BTNSword.blp"
	const disabled = "ReplaceableTextures/CommandButtonsDisabled/DISBTNSword.blp"
	put(t, s.root, "assets/"+icon)
	put(t, s.root, "assets/icons/disabled.blp", "disabled")
	put(t, s.mapDir, "war3mapImported/existing.wav", "editor")
	s.setImports(imp.Entry{Flag: 5, Path: "existing.wav"})
	before := testkit.Snapshot(t, s.root)

	_, result := s.planned(`{"paths":{"icons/disabled.blp":"` + disabled + `"},"exclude":[]}`)
	if got, want := names(result.Changes), []string{icon, disabled, "war3map.imp"}; !slices.Equal(got, want) {
		t.Fatalf("the changes are %q, want %q", got, want)
	}
	if got := string(result.Changes[1].Bytes); got != "disabled" {
		t.Errorf("the mapped asset is written as %q", got)
	}
	entries, err := imp.Read(result.Changes[2].Bytes, "war3map.imp")
	want := []imp.Entry{
		{Flag: 5, Path: "existing.wav"},
		{Flag: imp.CustomPath, Path: strings.ReplaceAll(icon, "/", `\`)},
		{Flag: imp.CustomPath, Path: strings.ReplaceAll(disabled, "/", `\`)},
	}
	if err != nil || !slices.Equal(entries, want) {
		t.Errorf("the index lists %+v, %v, want %+v", entries, err, want)
	}
	owned := []Owned{{icon, hashed("asset")}, {disabled, hashed("disabled")}}
	if !slices.Equal(result.State.Files, owned) {
		t.Errorf("the state is %+v, want %+v", result.State.Files, owned)
	}
	if got := rows(result.Assets); !slices.Equal(got, []row{{"", icon, icon}, {"", "icons/disabled.blp", disabled}}) {
		t.Errorf("the assets are %+v", got)
	}
	s.unchanged(before, "planning")
}

func TestPlanChangesOnlyWhatDiffersAndRemovesOnlyOwnedFilesNoAssetWants(t *testing.T) {
	s := newSite(t)
	for name, content := range map[string]string{
		"same.blp": "same", "edited.blp": "first", "dropped.blp": "dropped", "Sounds/dropped.wav": "dropped too",
		"unmanaged.txt": "keep",
	} {
		put(t, s.mapDir, name, content)
	}
	// The state lists a file the map does not have: its asset is written again.
	s.owns("same.blp", "edited.blp", "Sounds/dropped.wav", "dropped.blp", "gone.blp")
	s.setImports(
		imp.Entry{Flag: 13, Path: "same.blp"}, imp.Entry{Flag: 13, Path: "edited.blp"},
		imp.Entry{Flag: 13, Path: `Sounds\dropped.wav`}, imp.Entry{Flag: 13, Path: "dropped.blp"},
		imp.Entry{Flag: 13, Path: "gone.blp"})
	for name, content := range map[string]string{"same.blp": "same", "edited.blp": "second", "gone.blp": "back", "new.blp": "new"} {
		put(t, s.root, "assets/"+name, content)
	}

	_, result := s.planned(noBlock)
	// The writes in the order of the assets, the removals in the order of the state, then the index.
	want := []string{"edited.blp", "gone.blp", "new.blp", "-Sounds/dropped.wav", "-dropped.blp", "war3map.imp"}
	if got := names(result.Changes); !slices.Equal(got, want) {
		t.Errorf("the changes are %q, want %q", got, want)
	}
	owned := []Owned{{"edited.blp", hashed("second")}, {"gone.blp", hashed("back")}, {"new.blp", hashed("new")},
		{"same.blp", hashed("same")}}
	if !slices.Equal(result.State.Files, owned) {
		t.Errorf("the state is %+v, want %+v", result.State.Files, owned)
	}
}

// The index is read and written back. An import of the map's own whose name starts with a byte order mark names
// a file by those bytes, and is listed by them still when another asset is added.
func TestPlanKeepsAnImportOfTheMapsOwnByteForByteWithAMarkAtItsStart(t *testing.T) {
	const marked = "\xEF\xBB\xBFa.blp"
	s := newSite(t)
	put(t, s.mapDir, marked, "the map's own")
	s.setImports(imp.Entry{Flag: 13, Path: marked})
	put(t, s.root, "assets/b.blp")

	_, result := s.planned(noBlock)
	if got, want := names(result.Changes), []string{"b.blp", "war3map.imp"}; !slices.Equal(got, want) {
		t.Fatalf("the changes are %q, want %q", got, want)
	}
	want := imp.Write([]imp.Entry{{Flag: 13, Path: marked}, {Flag: imp.CustomPath, Path: "b.blp"}})
	if got := result.Changes[1].Bytes; !bytes.Equal(got, want) {
		t.Errorf("the index is %q, want %q", got, want)
	}
}

func TestPlanChangesNothingInAMapThatHoldsEveryAssetAndListsIt(t *testing.T) {
	s := newSite(t)
	put(t, s.root, "assets/Textures/a.blp")
	put(t, s.mapDir, "Textures/a.blp")
	s.owns("Textures/a.blp")
	s.setImports(imp.Entry{Flag: 13, Path: `Textures\a.blp`})
	if _, result := s.planned(noBlock); len(result.Changes) != 0 || len(result.State.Files) != 1 {
		t.Errorf("the changes are %q and the state %+v, want no change and one owned file",
			names(result.Changes), result.State.Files)
	}
}

func TestAnOwnedImportKeepsTheFlagWorldEditorSavedItWith(t *testing.T) {
	s := newSite(t)
	for _, name := range []string{"Textures/a.blp", "Textures/b.blp"} {
		put(t, s.root, "assets/"+name)
		put(t, s.mapDir, name)
	}
	s.owns("Textures/a.blp", "Textures/b.blp")
	// World Editor 3.00 saves the flag 13 as 29.
	saved := []imp.Entry{{Flag: 29, Path: `Textures\a.blp`}, {Flag: 29, Path: `Textures\b.blp`}}
	s.setImports(saved...)
	if _, result := s.planned(noBlock); len(result.Changes) != 0 {
		t.Errorf("a map World Editor saved is changed: %q", names(result.Changes))
	}
	put(t, s.root, "assets/Textures/c.blp")
	_, result := s.planned(noBlock)
	if got := names(result.Changes); !slices.Equal(got, []string{"Textures/c.blp", "war3map.imp"}) {
		t.Fatalf("the changes are %q", got)
	}
	entries, err := imp.Read(result.Changes[1].Bytes, "war3map.imp")
	if want := append(saved, imp.Entry{Flag: imp.CustomPath, Path: `Textures\c.blp`}); err != nil || !slices.Equal(entries, want) {
		t.Errorf("the index lists %+v, %v, want %+v", entries, err, want)
	}
}

// An entry without a custom path keeps its flag too, and is then written with the whole in-map path: the index
// names another file than the asset's.
func TestAnOwnedImportThatWorldEditorSavedWithoutACustomPathKeepsThatFlag(t *testing.T) {
	s := newSite(t)
	put(t, s.root, "assets/war3mapImported/a.wav")
	put(t, s.mapDir, "war3mapImported/a.wav")
	s.owns("war3mapImported/a.wav")
	s.setImports(imp.Entry{Flag: 5, Path: "a.wav"})
	_, result := s.planned(noBlock)
	if got := names(result.Changes); !slices.Equal(got, []string{"war3map.imp"}) {
		t.Fatalf("the changes are %q", got)
	}
	entries, err := imp.Read(result.Changes[0].Bytes, "war3map.imp")
	if want := []imp.Entry{{Flag: 5, Path: `war3mapImported\a.wav`}}; err != nil || !slices.Equal(entries, want) {
		t.Errorf("the index lists %+v, %v, want %+v", entries, err, want)
	}
}

func TestPlanSpellsFoldersAsTheMapDoesAndNewOnesAsTheFirstAssetToNameThem(t *testing.T) {
	s := newSite(t)
	put(t, s.mapDir, "Textures/existing.blp")
	for _, name := range []string{"textures/new.blp", "a.blp", "b.blp", "c.blp"} {
		put(t, s.root, "assets/"+name)
	}
	block := `{"paths":{"a.blp":"Sound/Music/a.blp","b.blp":"sound/music/b.blp","c.blp":"SOUND/Effects/c.blp"},"exclude":[]}`
	_, result := s.planned(block)
	// The assets come in the order of their paths, so c.blp is the first to name the folder Sound.
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
	if got := names(result.Changes); !slices.Equal(got, append(spelled, "war3map.imp")) {
		t.Fatalf("the changes are %q", got)
	}
	entries, err := imp.Read(result.Changes[4].Bytes, "war3map.imp")
	if err != nil || len(entries) != 4 || entries[0].Path != `SOUND\Effects\c.blp` || entries[3].Path != `Textures\new.blp` {
		t.Errorf("the index lists %+v, %v", entries, err)
	}
}

func TestAnOwnedFileIsFoundAndWrittenUnderTheSpellingTheMapHas(t *testing.T) {
	s := newSite(t)
	put(t, s.mapDir, "models/UNIT.mdx", "first")
	put(t, s.mapDir, "WAR3MAP.IMP", string(imp.Write([]imp.Entry{{Flag: 13, Path: `Models\Unit.mdx`}})))
	put(t, s.root, ".asset-state/map.w3x.json", string(State{Files: []Owned{{`Models\Unit.mdx`, hashed("first")}}}.Bytes()))
	put(t, s.root, "assets/Models/Unit.mdx", "second")
	_, result := s.planned(noBlock)
	if got := names(result.Changes); !slices.Equal(got, []string{"models/UNIT.mdx", "WAR3MAP.IMP"}) {
		t.Errorf("the changes are %q", got)
	}
	if want := []Owned{{"models/UNIT.mdx", hashed("second")}}; !slices.Equal(result.State.Files, want) {
		t.Errorf("the state is %+v, want %+v", result.State.Files, want)
	}
}

func TestAnAssetAtAFileOrAnImportTheMapHasAndDoesNotOwnIsRefused(t *testing.T) {
	const own = "Import it under another path with assets.paths, or remove the map's own copy."
	tests := []struct {
		name       string
		asset      string // under assets/, or under the library's folder
		library    string
		file       string // a file the map has
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
			s := newSite(t)
			var libraries []string
			if tt.library == "" {
				put(t, s.root, "assets/"+tt.asset)
			} else {
				put(t, s.root, "libraries/"+tt.library+"/"+tt.asset)
				libraries = []string{tt.library}
			}
			if tt.file != "" {
				put(t, s.mapDir, tt.file, "the editor's")
			}
			if tt.entry != nil {
				s.setImports(*tt.entry)
			}
			before := testkit.Snapshot(t, s.root)
			e := s.refusedPlan(noBlock, libraries...)
			if e.Msg != tt.msg || e.File != mapLabel+"/"+tt.where || e.Hint != tt.hint {
				t.Errorf("error = %+v, want %q at %s", e, tt.msg, tt.where)
			}
			s.unchanged(before, "a refused plan")
		})
	}
}

func TestAnOwnedFileEditedInTheMapIsRefusedAlsoWhenNoAssetWantsIt(t *testing.T) {
	s := newSite(t)
	// The state spells the path in its own way: a file of the map is found in any letter case.
	put(t, s.root, ".asset-state/map.w3x.json", string(State{Files: []Owned{{"textures/A.BLP", hashed("asset")}}}.Bytes()))
	put(t, s.mapDir, "Textures/a.blp", "manual edit")
	for _, asset := range []string{"assets/Textures/a.blp", ""} {
		if asset != "" {
			put(t, s.root, asset, "a newer asset")
		}
		e := s.refusedPlan(noBlock)
		if e.Msg != "Textures/a.blp was modified in the map after assets:sync wrote it." ||
			e.File != mapLabel+"/Textures/a.blp" || !strings.Contains(e.Hint, "source map") {
			t.Errorf("with the asset %q: error = %+v", asset, e)
		}
		if err := os.RemoveAll(filepath.Join(s.root, "assets", "Textures")); err != nil {
			t.Fatal(err)
		}
	}
	if got := s.inMap("Textures/a.blp"); got != "manual edit" {
		t.Errorf("a refused plan left %q in the edited file", got)
	}
}

func TestAnAssetBelowAFileOfTheMapIsRefusedAlsoWhenTheFileIsOwned(t *testing.T) {
	const own = "Import it under another path with assets.paths, or remove "
	tests := []struct {
		name                 string
		files                []string // of the map
		owned                []string
		asset, target        string // under assets/, and the path it is mapped to
		library              bool
		inTheWay, wantsToBe  string
		hintStart, hintWords string
	}{
		{"a file of the map", []string{"Textures"}, nil, "a.blp", "textures/a.blp", false, "Textures", "textures/a.blp", own, "Textures"},
		{"a file in a folder", []string{"Units/Hero"}, nil, "a.blp", "units/hero/skins/a.blp", false,
			"Units/Hero", "units/hero/skins/a.blp", own, "Units/Hero"},
		// The plan would remove the owned file. No file of the map becomes a folder, an owned one neither.
		{"an owned file that no asset wants", []string{"data"}, []string{"data"}, "data/inner.txt", "", false,
			"data", "data/inner.txt", own, "data"},
		{"a library's file", []string{"UI"}, nil, "ui/frame.fdf", "", true, "UI", "ui/frame.fdf", "Remove UI", "source map"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newSite(t)
			for _, file := range tt.files {
				put(t, s.mapDir, file)
			}
			s.owns(tt.owned...)
			block, libraries := noBlock, []string(nil)
			switch {
			case tt.library:
				put(t, s.root, "libraries/lib/"+tt.asset)
				libraries = []string{"lib"}
			case tt.target != "":
				put(t, s.root, "assets/"+tt.asset)
				block = `{"paths":{"` + tt.asset + `":"` + tt.target + `"},"exclude":[]}`
			default:
				put(t, s.root, "assets/"+tt.asset)
			}
			e := s.refusedPlan(block, libraries...)
			msg := tt.inTheWay + " in the map is a file, not a directory, so " + tt.wantsToBe + " cannot go there."
			if e.Msg != msg || e.File != mapLabel+"/"+tt.inTheWay || !strings.HasPrefix(e.Hint, tt.hintStart) ||
				!strings.Contains(e.Hint, tt.hintWords) {
				t.Errorf("error = %+v, want %q at %s", e, msg, tt.inTheWay)
			}
		})
	}
}

func TestAnAssetNamedAsAFolderOfTheMapIsRefused(t *testing.T) {
	empty := func(s *site) {
		if err := os.Mkdir(filepath.Join(s.mapDir, "Textures"), 0o777); err != nil {
			s.t.Fatal(err)
		}
	}
	tests := []struct {
		name  string
		setup func(s *site)
	}{
		{"a folder with a file in it", func(s *site) { put(s.t, s.mapDir, "Textures/a.blp") }},
		{"an empty folder", empty},
		// The file the state lists is a folder in the map: it is not checked as an owned file, and not replaced.
		{"a folder where the state lists a file", func(s *site) {
			empty(s)
			put(s.t, s.root, ".asset-state/map.w3x.json", string(State{Files: []Owned{{"textures", zeros}}}.Bytes()))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newSite(t)
			tt.setup(s)
			put(t, s.root, "assets/textures")
			e := s.refusedPlan(noBlock)
			if e.Msg != "Asset textures would replace a folder in the map." || e.File != mapLabel+"/Textures" ||
				!strings.Contains(e.Hint, "remove that folder") {
				t.Errorf("error = %+v", e)
			}
		})
	}
}

func TestTheFirstAssetWithoutRoomIsTheOneRefused(t *testing.T) {
	s := newSite(t)
	put(t, s.mapDir, "a")
	put(t, s.mapDir, "b.blp")
	put(t, s.root, "assets/a/inner.blp")
	put(t, s.root, "assets/b.blp")
	if e := s.refusedPlan(noBlock); !strings.Contains(e.Msg, "a in the map is a file") {
		t.Errorf("error = %+v, want it about a/inner.blp, the first asset", e)
	}
	// The same two assets, the other one first.
	block := `{"paths":{"b.blp":"0.blp"},"exclude":[]}`
	put(t, s.mapDir, "0.blp")
	if e := s.refusedPlan(block); !strings.Contains(e.Msg, "Asset 0.blp conflicts") {
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
			s := newSite(t)
			put(t, s.root, "assets/a.blp")
			testkit.WriteFile(t, s.mapDir, "War3map.imp", tt.index)
			e := s.refusedPlan(noBlock)
			if e.Msg != tt.words || e.File != mapLabel+"/War3map.imp" || e.Hint == "" {
				t.Errorf("error = %+v, want %q at the map's War3map.imp", e, tt.words)
			}
		})
	}
}

func TestAFolderNamedAsTheIndexOfImportsIsRefusedBeforeAnythingIsPlanned(t *testing.T) {
	s := newSite(t)
	put(t, s.root, "assets/a.blp")
	put(t, s.mapDir, "War3Map.imp/stray.txt")
	before := testkit.Snapshot(t, s.root)
	e := s.refusedPlan(noBlock)
	if !strings.Contains(e.Msg, "War3Map.imp in the map is a folder") || e.File != mapLabel+"/War3Map.imp" ||
		!strings.Contains(e.Hint, "Remove that folder") || strings.Contains(e.Hint, "another path") {
		t.Errorf("error = %+v, want the folder named as the map spells it, and a hint that fits the index", e)
	}
	s.unchanged(before, "a refused plan")
}

func TestPlanStopsAtAnInterruptBetweenFilesAndWritesNothing(t *testing.T) {
	s := newSite(t)
	put(t, s.root, "assets/a.blp")
	put(t, s.root, "assets/b.blp")
	put(t, s.mapDir, "owned.blp")
	s.owns("owned.blp")
	before := testkit.Snapshot(t, s.root)
	// One ask before anything is read, one before each owned file and one before each asset.
	const asks = 4
	for limit := range asks {
		ctx := &countdown{Context: background, limit: limit}
		_, result, err := s.plan(ctx, noBlock)
		if e := asError(t, err, "an interrupted plan"); e.Msg != "Interrupted; nothing was written." || result != nil {
			t.Errorf("cancelled at ask %d: Plan = %+v, %+v", limit+1, result, e)
		}
		if ctx.asks != limit+1 {
			t.Errorf("cancelled at ask %d: Plan asked %d times, want it to stop at the ask that was refused", limit+1, ctx.asks)
		}
	}
	ctx := &countdown{Context: background, limit: asks}
	if _, _, err := s.plan(ctx, noBlock); err != nil || ctx.asks != asks {
		t.Errorf("Plan = %v after %d asks, want a plan after %d", err, ctx.asks, asks)
	}
	s.unchanged(before, "planning")
}

// A path that Collect and ReadState refuse cannot reach a plan through them. Given by another caller, it is that
// caller's bug: the plan would write, or remove, one of the map's own files.
func TestPlanRefusesAPathNoAssetMayHaveAsACallersBug(t *testing.T) {
	script := Asset{Source: "x.lua", Target: "war3map.lua", Bytes: []byte("x"), Hash: hashed("x")}
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
		{"the map's script as an owned file", nil, State{Files: []Owned{{"war3map.lua", hashed("the script")}}},
			`an owned file has the in-map path "war3map.lua"`},
		{"the map's script in its folder, as an owned file", nil, State{Files: []Owned{{`Scripts\war3map.j`, zeros}}},
			`an owned file has the in-map path "Scripts\\war3map.j"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newSite(t)
			put(t, s.mapDir, "war3map.lua", "the script")
			result, err := Plan(background, s.open(), tt.assets, tt.owned)
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
			s := newSite(t)
			put(t, s.mapDir, "Textures/owned.blp")
			s.owns("Textures/owned.blp")
			s.setImports(imp.Entry{Flag: 13, Path: `Textures\owned.blp`})
			testkit.MakeUnreadable(t, filepath.Join(s.mapDir, filepath.FromSlash(held)))
			e := s.refusedPlan(noBlock)
			if !strings.HasPrefix(e.Msg, "Reading a map file failed") || e.File != mapLabel+"/"+held || e.Cause == nil {
				t.Errorf("error = %+v", e)
			}
		})
	}
}

// A build lays the changes over its view of the map and stages the view: nothing is written into the source map,
// and the state file is neither read nor written.
func TestThePlanLaidOverAViewIsStagedAndTheSourceMapIsLeftAlone(t *testing.T) {
	s := newSite(t)
	put(t, s.mapDir, "Models/unit.mdx", "first")
	put(t, s.mapDir, "Models/dropped.mdx", "dropped")
	put(t, s.mapDir, "unmanaged.txt", "keep")
	put(t, s.mapDir, "war3map.w3i", "info")
	s.owns("Models/unit.mdx", "Models/dropped.mdx")
	s.setImports(imp.Entry{Flag: 13, Path: `Models\unit.mdx`}, imp.Entry{Flag: 13, Path: `Models\dropped.mdx`})
	put(t, s.root, "assets/models/unit.mdx", "second")
	put(t, s.root, "assets/Sound/theme.mp3", "theme")
	before := testkit.Snapshot(t, s.root)

	folder, result := s.planned(noBlock)
	// The view a build has by then holds the changes of the other areas.
	view := folder.With([]mapdir.Change{{Name: "war3map.w3i", Bytes: []byte("patched")}}).With(result.Changes)
	stage := filepath.Join(t.TempDir(), "stage", "map.w3x")
	if err := view.StageTo(stage); err != nil {
		t.Fatalf("StageTo: %v", diag.Format(err))
	}
	s.unchanged(before, "a build")

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
