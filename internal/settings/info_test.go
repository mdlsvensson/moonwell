package settings

import (
	"bytes"
	"encoding/binary"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/war3/w3i"
)

const infoFile = "maps/map.w3x/war3map.w3i"

var (
	everyVersion    = []int32{18, 25, 28, 31, 32, 33, 39}
	detailsVersions = []int32{28, 31, 32, 33, 39}
)

func mustPatchInfo(t *testing.T, source []byte, document string) []byte {
	t.Helper()
	result, err := patchInfo(source, mustDecodeSettings(t, document), infoFile)
	if err != nil {
		t.Fatalf("patchInfo(%s): %v", document, err)
	}
	return result
}

func mustFailPatchInfo(t *testing.T, source []byte, document string) *diag.Error {
	t.Helper()
	_, err := patchInfo(source, mustDecodeSettings(t, document), infoFile)
	diagErr := asDiagError(t, err, document)
	if diagErr.File != infoFile || diagErr.Hint == "" {
		t.Errorf("%s: the error names %q and hints %q", document, diagErr.File, diagErr.Hint)
	}
	return diagErr
}

func mustReadInfo(t *testing.T, data []byte, depth w3i.Depth) *w3i.Info {
	t.Helper()
	info, err := w3i.Read(data, infoFile, depth)
	if err != nil {
		t.Fatal(err)
	}
	return info
}

func withoutFlags(t *testing.T) []byte {
	t.Helper()
	source := testkit.SyntheticMapInfo(39)
	binary.LittleEndian.PutUint32(source[mustReadInfo(t, source, w3i.Basic).Flags.Start:], 0)
	return source
}

func TestALoadingScreenPatchOfVersion39KeepsTheUnknownFieldBeforeItAndEveryByteAfterIt(t *testing.T) {
	source := testkit.Fixture(t, "map-settings-v39/war3map.w3i")
	if !bytes.Equal(source[141:145], []byte{64, 0, 0, 0}) {
		t.Fatalf("the fixture's bytes 141 to 145 are % X", source[141:145])
	}
	replacement := []byte("Loading.mdx\x00Text\x00Title\x00Subtitle\x00")
	expected := slices.Concat(source[:145], replacement, source[149:])
	document := `{"loadingScreen":{"model":"Loading.mdx","text":"Text","title":"Title","subtitle":"Subtitle"}}`
	if got := mustPatchInfo(t, source, document); !bytes.Equal(got, expected) {
		t.Error("the loading screen patch changed other bytes")
	}
	padded := slices.Concat(make([]byte, 3), source, make([]byte, 4))
	if got := mustPatchInfo(t, padded[3:3+len(source)], document); !bytes.Equal(got, expected) {
		t.Error("a slice of a larger buffer patches differently")
	}
}

func TestColoursWaterTintAndSoundEnvironmentMatchAWorldEditorSave(t *testing.T) {
	source := testkit.Fixture(t, "map-settings-v39/war3map.w3i")
	editor := testkit.Fixture(t, "map-settings-v39/war3map-colors.w3i")
	got := mustPatchInfo(t, source,
		`{"environment":{"soundEnvironment":"Dungeon","waterColor":[255,0,0,255],"fog":{"enabled":true,"color":[255,0,0,255]}}}`)
	if len(got) != len(editor) {
		t.Fatalf("the patched file has %d bytes, World Editor's save %d", len(got), len(editor))
	}
	editorOnly := []int{4, 141, 230, 231, 234, 235, 238, 239}
	for i := range editor {
		if !slices.Contains(editorOnly, i) && got[i] != editor[i] {
			t.Errorf("byte %d is %#x, World Editor wrote %#x", i, got[i], editor[i])
		}
	}
}

func TestEveryVersionKeepsEveryOtherByteAndTakesAnEmptyText(t *testing.T) {
	const name = "M\xc3\xb8\xc3\xb8nwell"
	document := `{"info":{"name":"` + name + `","author":"","description":"TRIGSTR_001"},"loadingScreen":{"title":"Changed","background":7}}`
	restore := `{"info":{"name":"TRIGSTR_001","author":"Author","description":"Description"},"loadingScreen":{"title":"Title","background":0}}`
	for _, version := range everyVersion {
		source := testkit.SyntheticMapInfo(version)
		changed := mustPatchInfo(t, source, document)
		if again := mustPatchInfo(t, source, document); !bytes.Equal(again, changed) {
			t.Errorf("version %d: the patch is not repeatable", version)
		}
		info := mustReadInfo(t, changed, w3i.Basic)
		if info.Name.Value != name || info.Author.Value != "" || info.Description.Value != "TRIGSTR_001" ||
			info.Loading.Title.Value != "Changed" || info.Loading.Background.Value != 7 {
			t.Errorf("version %d: %+v", version, info)
		}
		if restored := mustPatchInfo(t, changed, restore); !bytes.Equal(restored, source) {
			t.Errorf("version %d: setting the fields back did not give the bytes back", version)
		}
	}
}

func TestPlayerForceAndEnvironmentEditsKeepEveryOtherByteOfEveryVersion(t *testing.T) {
	document := `{
		"players":{"0":{"x":256,"controller":"computer"}},
		"forces":{"0":{"name":"Blue","sharedVision":true}},
		"environment":{"soundEnvironment":"Dungeon","waterColor":[2,3,4,255],"fog":{"start":2000}}}`
	restore := `{
		"players":{"0":{"x":128,"controller":"user"}},
		"forces":{"0":{"name":"Force 1","sharedVision":false}},
		"environment":{"soundEnvironment":"Default","waterColor":[255,255,255,255],"fog":{"start":1000}}}`
	for _, version := range detailsVersions {
		source := testkit.SyntheticMapInfo(version)
		changed := mustPatchInfo(t, source, document)
		details := mustReadInfo(t, changed, w3i.Extended).Details
		player, force := details.Players[0], details.Forces[0]
		if player.X.Value != 256 || player.Controller.Value != 2 || force.Name.Value != "Blue" ||
			force.Flags.Value != 11 || details.Fog.Start.Value != 2000 || details.SoundEnvironment.Value != "Dungeon" {
			t.Errorf("version %d: %+v", version, details)
		}
		restored := mustPatchInfo(t, changed, restore)
		flags := mustReadInfo(t, source, w3i.Basic).Flags
		copy(restored[mustReadInfo(t, restored, w3i.Basic).Flags.Start:], source[flags.Start:flags.End])
		if !bytes.Equal(restored, source) {
			t.Errorf("version %d: setting the fields back did not give the bytes back", version)
		}
	}
}

func TestEveryFieldOfAPlayerAForceAndTheFogIsWritten(t *testing.T) {
	source := testkit.SyntheticMapInfo(39)
	changed := mustPatchInfo(t, source, `{
		"info":{"recommendedPlayers":"Any"},
		"loadingScreen":{"model":"L.mdx","text":"T","subtitle":"S"},
		"players":{"0":{"name":"Hero","race":"undead","fixedStart":false,"y":64.5}},
		"forces":{"0":{"allied":false,"alliedVictory":false,"sharedVision":true,"sharedControl":true,"sharedAdvancedControl":true}},
		"environment":{"fog":{"enabled":true,"style":2,"end":9000,"density":0.25,"color":[10,20,30,40]}}}`)
	info := mustReadInfo(t, changed, w3i.Extended)
	if info.RecommendedPlayers.Value != "Any" || info.Loading.Model.Value != "L.mdx" || info.Loading.Text.Value != "T" ||
		info.Loading.Subtitle.Value != "S" {
		t.Errorf("the description and the loading screen: %+v", info)
	}
	player := info.Details.Players[0]
	if player.Name.Value != "Hero" || player.Race.Value != 3 || player.FixedStart.Value != 0 || player.Y.Value != 64.5 ||
		player.Controller.Value != 1 || player.X.Value != 128 {
		t.Errorf("player 0 = %+v", player)
	}
	if force := info.Details.Forces[0]; force.Flags.Value != 8|16|32 || force.Name.Value != "Force 1" {
		t.Errorf("force 0 = %+v", force)
	}
	fog := info.Details.Fog
	color := [4]uint8{fog.Color[0].Value, fog.Color[1].Value, fog.Color[2].Value, fog.Color[3].Value}
	if fog.Style.Value != 2 || fog.Start.Value != 1000 || fog.End.Value != 9000 || fog.Density.Value != 0.25 ||
		color != [4]uint8{10, 20, 30, 40} {
		t.Errorf("fog = %+v", fog)
	}
	if info.Flags.Value != 0x40|0x2000 {
		t.Errorf("the map flags are %#x, want custom forces and fog", info.Flags.Value)
	}
	off := mustReadInfo(t, mustPatchInfo(t, changed, `{"environment":{"fog":{"enabled":false}}}`), w3i.Basic)
	if off.Flags.Value != 0x40 {
		t.Errorf("with the fog turned off the map flags are %#x", off.Flags.Value)
	}
}

func TestWhatAMapInfoOfItsVersionCannotHoldIsRefusedByTheMapInfo(t *testing.T) {
	for _, version := range []int32{18, 25} {
		mustFailPatchInfo(t, testkit.SyntheticMapInfo(version), `{"players":{"0":{"name":"P"}}}`)
	}
	mustFailPatchInfo(t, testkit.SyntheticMapInfo(18), `{"loadingScreen":{"model":""}}`)
	source := testkit.SyntheticMapInfo(39)
	mustFailPatchInfo(t, source, `{"players":{"2":{"name":"P"}}}`)
	mustFailPatchInfo(t, source, `{"forces":{"1":{"name":"F"}}}`)
	mustFailPatchInfo(t, source, `{"environment":{"fog":{"start":6000}}}`)
}

func TestOnlyFogEditsLookAtTheFogTheMapHas(t *testing.T) {
	source := testkit.SyntheticMapInfo(39)
	fog := mustReadInfo(t, source, w3i.Extended).Details.Fog
	binary.LittleEndian.PutUint32(source[fog.End.Start:], math.Float32bits(float32(math.Inf(1))))
	mustPatchInfo(t, source, `{"info":{"name":"Safe"}}`)
	mustPatchInfo(t, source, `{"environment":{"soundEnvironment":"Safe"}}`)
	mustFailPatchInfo(t, source, `{"environment":{"fog":{"enabled":true}}}`)
	if got := mustPatchInfo(t, source, `{}`); !bytes.Equal(got, source) {
		t.Error("no settings changed the file")
	}
}

func TestForceEditsNeedTheCustomForcesOfTheMap(t *testing.T) {
	diagErr := mustFailPatchInfo(t, withoutFlags(t), `{"forces":{"0":{"name":"X"}}}`)
	if !strings.Contains(diagErr.Msg, "custom forces") {
		t.Errorf("error = %+v", diagErr)
	}
}

func TestARefusalThatNeedsTheMapNamesTheSetting(t *testing.T) {
	source := testkit.SyntheticMapInfo(39)
	tests := []struct {
		name            string
		source          []byte
		document, words string
	}{
		{"a player the map lacks", source, `{"players":{"7":{"name":"P"}}}`,
			`settings.players["7"]: player 7 does not exist in the source map.`},
		{"a force the map lacks", source, `{"forces":{"1":{"name":"F"}}}`,
			`settings.forces["1"]: force 1 does not exist in the source map.`},
		{"a force without custom forces", withoutFlags(t), `{"forces":{"0":{"name":"F"}}}`,
			`settings.forces["0"]: force overrides require custom forces`},
		{"a force the map lacks, without custom forces", withoutFlags(t), `{"forces":{"1":{"name":"F"}}}`,
			`settings.forces["1"]: force 1 does not exist`},
		{"a fog that starts after its end", source, `{"environment":{"fog":{"start":6000}}}`,
			"settings.environment.fog: start, end and density must be"},
		{"a fog that ends before its start", source, `{"environment":{"fog":{"end":999}}}`,
			"settings.environment.fog: start, end and density must be"},
		{"a loading-screen model before version 25", testkit.SyntheticMapInfo(18), `{"loadingScreen":{"model":""}}`,
			"settings.loadingScreen.model: custom loading-screen"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diagErr := mustFailPatchInfo(t, tt.source, tt.document); !strings.Contains(diagErr.Msg, tt.words) {
				t.Errorf("%s: %q, want %q", tt.document, diagErr.Msg, tt.words)
			}
		})
	}
}

func TestPlayersAndForcesGoInSlotOrder(t *testing.T) {
	source := testkit.SyntheticMapInfo(39)
	tests := []struct{ name, document, words string }{
		{"players", `{"players":{"10":{"name":"k"},"2":{"name":"c"}}}`, "player 2 does not exist"},
		{"forces", `{"forces":{"11":{"name":"k"},"3":{"name":"c"}}}`, "force 3 does not exist"},
		{"a player before a force", `{"forces":{"3":{"name":"c"}},"players":{"10":{"name":"k"}}}`, "player 10 does not exist"},
		{"a force before the fog", `{"environment":{"fog":{"start":6000}},"forces":{"3":{"name":"c"}}}`, "force 3 does not exist"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diagErr := mustFailPatchInfo(t, source, tt.document); !strings.Contains(diagErr.Msg, tt.words) {
				t.Errorf("the first refusal is %q, want %q", diagErr.Msg, tt.words)
			}
		})
	}
}

func TestAnOverrideWithNothingSetIsSkippedSoItsSlotNeedNotExist(t *testing.T) {
	empty := `{"info":{"name":null},"players":{"7":{"name":null}},"forces":{"5":{}},"environment":{"fog":{}}}`
	for _, source := range [][]byte{testkit.SyntheticMapInfo(18), []byte("not a map info")} {
		if got := mustPatchInfo(t, source, empty); !bytes.Equal(got, source) {
			t.Error("overrides with nothing set changed the file")
		}
	}
	source := testkit.SyntheticMapInfo(39)
	changed := mustPatchInfo(t, source, `{"players":{"7":{},"0":{"name":"Hero"}},"forces":{"5":{},"0":{"name":"Blue"}}}`)
	details := mustReadInfo(t, changed, w3i.Extended).Details
	if details.Players[0].Name.Value != "Hero" || details.Forces[0].Name.Value != "Blue" {
		t.Errorf("the overrides beside the empty ones: %+v", details)
	}
	if got := mustPatchInfo(t, withoutFlags(t), `{"forces":{"0":{}}}`); !bytes.Equal(got, withoutFlags(t)) {
		t.Error("a force override with nothing set needed custom forces")
	}
}

func TestThePreviewIsNotStoredInTheMapInfo(t *testing.T) {
	source := []byte("not a map info")
	if got := mustPatchInfo(t, source, `{"info":{"preview":"preview.tga"}}`); !bytes.Equal(got, source) {
		t.Error("a preview alone read or changed the map info")
	}
}

func TestAControllerOrARaceWithoutANumberIsNotTheUsersMistake(t *testing.T) {
	source := testkit.SyntheticMapInfo(39)
	for _, name := range []string{"elf", ""} {
		for _, player := range []manifest.Player{{Race: &name}, {Controller: &name}} {
			_, err := patchInfo(source, manifest.Settings{Players: map[int]manifest.Player{0: player}}, infoFile)
			if _, expected := diag.FirstProblem(err); err == nil || expected || !strings.Contains(err.Error(), "has no number") {
				t.Errorf("the name %q: error = %v, want one that is not a diag error", name, err)
			}
		}
	}
}

func TestSettingsStoredInTheMapInfoNeedOneThatReads(t *testing.T) {
	diagErr := mustFailPatchInfo(t, []byte{39, 0, 0}, `{"info":{"name":"N"}}`)
	if !strings.Contains(diagErr.Msg, "Cannot read map settings") {
		t.Errorf("error = %+v", diagErr)
	}
}
