package settings_test

import (
	"bytes"
	"encoding/binary"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/settings"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/w3i"
)

const mapInfoFile = "map/war3map.w3i"

func patchInfo(t *testing.T, source []byte, document string) []byte {
	t.Helper()
	patched, err := settings.PatchMapInfo(source, validated(t, document), mapInfoFile)
	if err != nil {
		t.Fatalf("PatchMapInfo(%s): %v", document, err)
	}
	return patched
}

// refusedInfo checks that the patch fails with an error naming the map info file.
func refusedInfo(t *testing.T, source []byte, document string) string {
	t.Helper()
	_, err := settings.PatchMapInfo(source, validated(t, document), mapInfoFile)
	e := asError(t, err, document)
	if e.File != mapInfoFile {
		t.Errorf("%s: the error names %q", document, e.File)
	}
	return e.Msg
}

func readInfo(t *testing.T, source []byte, extended bool) *w3i.Info {
	t.Helper()
	info, err := w3i.Read(source, extended, mapInfoFile)
	if err != nil {
		t.Fatal(err)
	}
	return info
}

func TestV39LoadingPatchPreservesIndependentlyRecordedExtensionAndTail(t *testing.T) {
	source := testkit.Fixture(t, "map-settings-v39/war3map.w3i")
	if !bytes.Equal(source[141:145], []byte{64, 0, 0, 0}) {
		t.Fatalf("the fixture's bytes 141 to 145 are % X", source[141:145])
	}
	replacement := []byte("Loading.mdx\x00Text\x00Title\x00Subtitle\x00")
	expected := slices.Concat(source[:145], replacement, source[149:])
	document := `{"loadingScreen":{"model":"Loading.mdx","text":"Text","title":"Title","subtitle":"Subtitle"}}`
	if got := patchInfo(t, source, document); !bytes.Equal(got, expected) {
		t.Error("the loading screen patch changed other bytes")
	}
	padded := slices.Concat(make([]byte, 3), source, make([]byte, 4))
	if got := patchInfo(t, padded[3:3+len(source)], document); !bytes.Equal(got, expected) {
		t.Error("a slice of a larger buffer patches differently")
	}
}

func TestColoursWaterTintAndSoundEnvironmentMatchAWorldEditorSave(t *testing.T) {
	source := testkit.Fixture(t, "map-settings-v39/war3map.w3i")
	editor := testkit.Fixture(t, "map-settings-v39/war3map-colors.w3i")
	patched := patchInfo(t, source,
		`{"environment":{"soundEnvironment":"Dungeon","waterColor":[255,0,0,255],"fog":{"enabled":true,"color":[255,0,0,255]}}}`)
	if len(patched) != len(editor) {
		t.Fatalf("patched has %d bytes, World Editor's save %d", len(patched), len(editor))
	}
	// World Editor also rewrote its save counter, an unknown field and the three camera zoom values.
	editorOnly := []int{4, 141, 230, 231, 234, 235, 238, 239}
	for i := range editor {
		if !slices.Contains(editorOnly, i) && patched[i] != editor[i] {
			t.Errorf("byte %d is %#x, World Editor wrote %#x", i, patched[i], editor[i])
		}
	}
}

func TestAllSupportedVersionsPreserveEveryUnrelatedByteAndAllowExplicitClears(t *testing.T) {
	for _, version := range []int32{18, 25, 28, 31, 32, 33, 39} {
		source := testkit.SyntheticMapInfo(version)
		document := `{"info":{"name":"Møønwell","author":"","description":"TRIGSTR_001"},"loadingScreen":{"title":"Changed","background":7}}`
		changed := patchInfo(t, source, document)
		if again := patchInfo(t, source, document); !bytes.Equal(again, changed) {
			t.Errorf("version %d: the patch is not repeatable", version)
		}
		parsed := readInfo(t, changed, false)
		if parsed.Name.Value != "Møønwell" || parsed.Author.Value != "" || parsed.Loading.Background.Value != 7 {
			t.Errorf("version %d: %+v", version, parsed)
		}
		restored := patchInfo(t, changed,
			`{"info":{"name":"TRIGSTR_001","author":"Author","description":"Description"},"loadingScreen":{"title":"Title","background":0}}`)
		if !bytes.Equal(restored, source) {
			t.Errorf("version %d: restoring the fields did not restore the bytes", version)
		}
	}
}

func TestExtendedEditsPreserveOldVersionUnknownBytes(t *testing.T) {
	for _, version := range []int32{28, 31, 32, 33, 39} {
		source := testkit.SyntheticMapInfo(version)
		changed := patchInfo(t, source, `{
			"players":{"0":{"x":256,"controller":"computer"}},
			"forces":{"0":{"name":"Blue","sharedVision":true}},
			"environment":{"soundEnvironment":"Dungeon","waterColor":[2,3,4,255],"fog":{"start":2000}}}`)
		d := readInfo(t, changed, true).Details
		if d.Players[0].X.Value != 256 || d.Players[0].Controller.Value != 2 || d.Forces[0].Name.Value != "Blue" ||
			d.Forces[0].Flags.Value != 11 || d.Fog.Start.Value != 2000 {
			t.Errorf("version %d: %+v", version, d)
		}
		restored := patchInfo(t, changed, `{
			"players":{"0":{"x":128,"controller":"user"}},
			"forces":{"0":{"name":"Force 1","sharedVision":false}},
			"environment":{"soundEnvironment":"Default","waterColor":[255,255,255,255],"fog":{"start":1000}}}`)
		// Water-color customization is a monotonic map flag; undo that flag for byte comparison.
		flags := readInfo(t, source, false).Flags
		copy(restored[readInfo(t, restored, false).Flags.Start:], source[flags.Start:flags.End])
		if !bytes.Equal(restored, source) {
			t.Errorf("version %d: restoring the fields did not restore the bytes", version)
		}
	}
}

func TestUnsupportedAndAbsentStructuresFailAsMapFileErrors(t *testing.T) {
	for _, version := range []int32{18, 25} {
		refusedInfo(t, testkit.SyntheticMapInfo(version), `{"players":{"0":{"name":"P"}}}`)
	}
	refusedInfo(t, testkit.SyntheticMapInfo(18), `{"loadingScreen":{"model":""}}`)
	source := testkit.SyntheticMapInfo(39)
	refusedInfo(t, source, `{"players":{"2":{"name":"P"}}}`)
	refusedInfo(t, source, `{"forces":{"1":{"name":"F"}}}`)
	refusedInfo(t, source, `{"environment":{"fog":{"start":6000}}}`)
}

func TestNonFogEditsIgnoreUnusedInheritedFogAndFogEditsRejectNonfiniteInheritance(t *testing.T) {
	source := testkit.SyntheticMapInfo(39)
	fog := readInfo(t, source, true).Details.Fog
	binary.LittleEndian.PutUint32(source[fog.End.Start:], math.Float32bits(float32(math.Inf(1))))
	patchInfo(t, source, `{"info":{"name":"Safe"}}`)
	patchInfo(t, source, `{"environment":{"soundEnvironment":"Safe"}}`)
	refusedInfo(t, source, `{"environment":{"fog":{"enabled":true}}}`)
	if got := patchInfo(t, source, `{}`); !bytes.Equal(got, source) {
		t.Error("no settings changed the file")
	}
}

func TestForceEditsRequireTheEditorsCustomForcesFlag(t *testing.T) {
	source := testkit.SyntheticMapInfo(39)
	binary.LittleEndian.PutUint32(source[readInfo(t, source, false).Flags.Start:], 0)
	refusedInfo(t, source, `{"forces":{"0":{"name":"X"}}}`)
}

func TestMapDependentErrorsNameTheSettingsPathThatNeedsTheMap(t *testing.T) {
	source := testkit.SyntheticMapInfo(39)
	noForces := testkit.SyntheticMapInfo(39)
	binary.LittleEndian.PutUint32(noForces[readInfo(t, noForces, false).Flags.Start:], 0)
	for _, c := range []struct {
		source            []byte
		document, message string
	}{
		{source, `{"players":{"7":{"name":"P"}}}`, `settings.players["7"]: player 7 does not exist in the source map.`},
		{source, `{"forces":{"1":{"name":"F"}}}`, `settings.forces["1"]: force 1 does not exist in the source map.`},
		{noForces, `{"forces":{"0":{"name":"F"}}}`, `settings.forces["0"]: force overrides require custom forces`},
		{source, `{"environment":{"fog":{"start":6000}}}`, "settings.environment.fog: start, end and density must be"},
		{testkit.SyntheticMapInfo(18), `{"loadingScreen":{"model":""}}`, "settings.loadingScreen.model: custom loading-screen"},
	} {
		if message := refusedInfo(t, c.source, c.document); !strings.Contains(message, c.message) {
			t.Errorf("%s: %q, want %q", c.document, message, c.message)
		}
	}
}
