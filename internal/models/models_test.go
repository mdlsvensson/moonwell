package models

import (
	"errors"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

const knight = `// Exported by a modelling tool
Version {
	FormatVersion 800,
}
Model "Knight" {
	NumGeosets 1,
}
Textures 3 {
	Bitmap {
		Image "Textures\Knight.blp",
	}
	Bitmap {
		Image "",
		ReplaceableId 1,
	}
	Bitmap {
		Image "",
		ReplaceableId 2,
	}
}
ParticleEmitter "Heal" {
	ObjectId 3,
	EmitterUsesMDL,
	static EmissionRate 1,
	Visibility 2 {
		DontInterp,
		0: 1,
		100: 0,
	}
	Translation 1 {
		Linear,
		0: { 1, 2, 3 },
	}
	Particle {
		static LifeSpan 1,
		static InitVelocity 0,
		Path "Abilities\Spells\Human\Heal.mdx",
	}
}
ParticleEmitter "Spark" {
	EmitterUsesTGA,
	Path "Textures\Spark.blp",
}
ParticleEmitter "Empty" {
	Path "",
}
Attachment "Hand" {
	AttachmentID 0,
	Path "Models\Sword.mdx",
}
ParticleEmitterPopcorn "Fire" {
	Path "Effects\Fire.pkfx",
}
FaceFX "Head" {
	Path "FaceFX\Knight.facefx",
}
`

var knightPaths = []Path{
	{Kind: Texture, Path: `Textures\Knight.blp`},
	{Kind: Texture, ReplaceableID: 1},
	{Kind: Texture, ReplaceableID: 2},
	{Kind: ParticleModel, Path: `Abilities\Spells\Human\Heal.mdx`},
	{Kind: ParticleTexture, Path: `Textures\Spark.blp`},
	{Kind: Attachment, Path: `Models\Sword.mdx`},
	{Kind: Popcorn, Path: `Effects\Fire.pkfx`},
	{Kind: FaceEffect, Path: `FaceFX\Knight.facefx`},
}

// notReadable checks that err is "Not a readable model" naming the file once, through File.
func notReadable(t *testing.T, err error, file string) *diag.Error {
	t.Helper()
	var e *diag.Error
	if !errors.As(err, &e) || !strings.HasPrefix(e.Msg, "Not a readable model: ") || e.File != file {
		t.Fatalf("got %v, want a model error naming %s", err, file)
	}
	if strings.Contains(e.Msg, "Knight.md") {
		t.Errorf("the file is printed once, from File: %q", e.Msg)
	}
	return e
}

func TestReadMDLReadsEveryPathBearingBlockAndIgnoresTheRest(t *testing.T) {
	got, err := ReadMDL(knight, "knight.mdl")
	if err != nil || !slices.Equal(got, knightPaths) {
		t.Errorf("ReadMDL = %+v, %v", got, err)
	}
}

func TestReadMDLRejectsBrokenText(t *testing.T) {
	for source, problem := range map[string]string{
		`Bitmap { Image "Textures\A.blp`:         "a string is never closed",
		"}":                                      "a } has no matching {",
		`Textures 1 { Bitmap { Image "a.blp", }`: "the Textures block is never closed",
		`Version { } { Path "x" `:                "the unnamed block is never closed",
	} {
		_, err := ReadMDL(source, "assets/Knight.mdl")
		if e := notReadable(t, err, "assets/Knight.mdl"); e.Msg != "Not a readable model: "+problem+"." {
			t.Errorf("ReadMDL(%q): %q", source, e.Msg)
		}
	}
}

func TestPathsPicksTheReaderFromTheContent(t *testing.T) {
	got, err := Paths([]byte(knight), "knight.mdl")
	if err != nil || !slices.Equal(got, knightPaths) {
		t.Errorf("Paths of text = %+v, %v", got, err)
	}
	got, err = Paths(testkit.MDX(testkit.Chunk("TEXS", testkit.Texture(`Textures\A.blp`, 0))), "a.mdx")
	if err != nil || !slices.Equal(got, []Path{{Kind: Texture, Path: `Textures\A.blp`}}) {
		t.Errorf("Paths of binary = %+v, %v", got, err)
	}
	blp := []byte{0x42, 0x4c, 0x50, 0x31, 0, 0, 0, 0} // a BLP texture, not a model
	_, err = Paths(blp, "icon.blp")
	if e := notReadable(t, err, "icon.blp"); !strings.Contains(e.Msg, "neither a binary MDX nor a text MDL") {
		t.Errorf("a texture: %q", e.Msg)
	}
}

func TestPathsRejectsTextThatIsNotAModel(t *testing.T) {
	for _, source := range []string{
		"",
		"hello world",
		"version https://git-lfs.github.com/spec/v1\noid sha256:abc\nsize 123\n", // a Git LFS pointer
		"Textures 1 {\n\tBitmap {\n\t\tImage \"a.blp\",\n\t}\n}\n",               // a block, but no Version or Model
		"Textures 1 {\n\tVersion {\n\t}\n}\n",                                    // a nested Version block does not count
	} {
		_, err := Paths([]byte(source), "assets/Knight.mdx")
		e := notReadable(t, err, "assets/Knight.mdx")
		if e.Msg != "Not a readable model: it has no Version or Model block." {
			t.Errorf("Paths(%q): %q", source, e.Msg)
		}
	}
	_, err := Paths([]byte{0x4d, 0x44}, "assets/Knight.mdx") // "MD": too short to be an MDX
	notReadable(t, err, "assets/Knight.mdx")
}

func TestReadMDLAcceptsAModelWithOnlyAVersionBlockOrOnlyAModelBlock(t *testing.T) {
	for _, source := range []string{"Version {\n\tFormatVersion 800,\n}\n", "Model \"A\" {\n\tNumGeosets 0,\n}\n"} {
		if got, err := ReadMDL(source, "a.mdl"); err != nil || len(got) != 0 {
			t.Errorf("ReadMDL(%q) = %+v, %v", source, got, err)
		}
	}
}

func TestIsMDXRecognizesTheMDLXMagic(t *testing.T) {
	if !IsMDX(testkit.MDX()) || IsMDX([]byte("Version {")) || IsMDX([]byte{0x4d, 0x44}) {
		t.Error("IsMDX is wrong")
	}
}

func TestReadMDXReadsTexturesIncludingReplaceableSlots(t *testing.T) {
	data := testkit.MDX(
		testkit.Chunk("VERS", testkit.U32(800)),
		testkit.Chunk("TEXS", testkit.Concat(
			testkit.Texture(`Textures\Knight.blp`, 0), testkit.Texture("", 1), testkit.Texture("", 2),
			testkit.Texture("", 11),
		)),
	)
	want := []Path{
		{Kind: Texture, Path: `Textures\Knight.blp`},
		{Kind: Texture, ReplaceableID: 1},
		{Kind: Texture, ReplaceableID: 2},
		{Kind: Texture, ReplaceableID: 11},
	}
	if got, err := ReadMDX(data, "knight.mdx"); err != nil || !slices.Equal(got, want) {
		t.Errorf("ReadMDX = %+v, %v", got, err)
	}
}

func TestReadMDXReadsEmittersAttachmentsPopcornAndFaceEffectsInFileOrder(t *testing.T) {
	data := testkit.MDX(
		testkit.Chunk("ZZZZ", make([]byte, 13)), // an unknown chunk is skipped by its size
		testkit.Chunk("PREM", testkit.Concat(
			testkit.Emitter(`Abilities\Spells\Human\Heal.mdx`, testkit.EmitterUsesMDL),
			testkit.Emitter(`Textures\Spark.blp`, testkit.EmitterUsesTGA),
			testkit.Emitter(`Models\Default.mdx`, 0),
			testkit.Emitter("", testkit.EmitterUsesMDL),
		)),
		testkit.Chunk("ATCH", testkit.Concat(testkit.Attachment(`Models\Sword.mdx`), testkit.Attachment(""))),
		testkit.Chunk("CORN", testkit.Popcorn(`Effects\Fire.pkfx`)),
		testkit.Chunk("FAFX", testkit.FaceEffect("Head", `FaceFX\Knight.facefx`)),
	)
	want := []Path{
		{Kind: ParticleModel, Path: `Abilities\Spells\Human\Heal.mdx`},
		{Kind: ParticleTexture, Path: `Textures\Spark.blp`},
		{Kind: ParticleModel, Path: `Models\Default.mdx`},
		{Kind: Attachment, Path: `Models\Sword.mdx`},
		{Kind: Popcorn, Path: `Effects\Fire.pkfx`},
		{Kind: FaceEffect, Path: `FaceFX\Knight.facefx`},
	}
	if got, err := ReadMDX(data, "knight.mdx"); err != nil || !slices.Equal(got, want) {
		t.Errorf("ReadMDX = %+v, %v", got, err)
	}
}

func TestReadMDXRejectsDamagedFiles(t *testing.T) {
	whole := testkit.MDX(testkit.Chunk("TEXS", testkit.Texture("a.blp", 0)))
	record := testkit.Attachment(`Models\Sword.mdx`)
	readsNaturally := regexp.MustCompile(`\ba [A-Z]{4}\b`)
	for problem, data := range map[string][]byte{
		"a chunk header is cut off":                            testkit.Concat(testkit.MDX(), []byte{1, 2, 3}),
		"the TEXS chunk runs past the end of the file":         whole[:100],
		"the TEXS chunk is not a whole number of textures":     testkit.MDX(testkit.Chunk("TEXS", make([]byte, 100))),
		"the FAFX chunk is not a whole number of face effects": testkit.MDX(testkit.Chunk("FAFX", make([]byte, 100))),
		"the ATCH chunk has a record with an invalid size":     testkit.MDX(testkit.Chunk("ATCH", testkit.SetU32(record, 0, 9999))),
		"the ATCH chunk has a node with an invalid size":       testkit.MDX(testkit.Chunk("ATCH", testkit.SetU32(record, 4, 10))),
		"the ATCH chunk has a record too small for its path":   testkit.MDX(testkit.Chunk("ATCH", testkit.SetU32(record, 0, 4+96))),
	} {
		_, err := ReadMDX(data, "assets/Knight.mdx")
		e := notReadable(t, err, "assets/Knight.mdx")
		if e.Msg != "Not a readable model: "+problem+"." {
			t.Errorf("got %q, want %q", e.Msg, problem)
		}
		if readsNaturally.MatchString(e.Msg) {
			t.Errorf("reads unnaturally: %s", e.Msg)
		}
	}
}

func TestDescribeLabelsReplaceableTextures(t *testing.T) {
	for want, path := range map[string]Path{
		`Textures\A.blp`:                {Kind: Texture, Path: `Textures\A.blp`},
		"team colour (slot 1)":          {Kind: Texture, ReplaceableID: 1},
		"team glow (slot 2)":            {Kind: Texture, ReplaceableID: 2},
		"replaceable texture (slot 11)": {Kind: Texture, ReplaceableID: 11},
	} {
		if got := Describe(path); got != want {
			t.Errorf("Describe(%+v) = %q", path, got)
		}
	}
}

func TestNormalizeGamePathStripsStoragePrefixesAndKeepsOnlyModelReferencedFileTypes(t *testing.T) {
	for line, want := range map[string]string{
		`war3.w3mod:Units\Human\Footman\Footman.mdx`:                                 "units/human/footman/footman.mdx",
		"war3.w3mod:_hd.w3mod:Doodads/LordaeronSummer/Plants/Corn/plant1_Normal.dds": "doodads/lordaeronsummer/plants/corn/plant1_normal.dds",
		"war3.w3mod:_locales/enus.w3mod:Textures/Black32.blp":                        "textures/black32.blp",
		"_hd.w3mod/_locales/dede.w3mod/Textures/Black32.blp":                         "textures/black32.blp",
		`war3.mpq:Abilities\Spells\Human\Heal\Heal.mdl`:                              "abilities/spells/human/heal/heal.mdl",
		"  Effects/Fire.pkfx  ":                                                      "effects/fire.pkfx",
		// Reforged stores its particle effects baked, as .pkb (the real CASC export has no .pkfx).
		`war3.w3mod:_de.w3mod:abilities\ribbon\chainlightning.pkb`: "abilities/ribbon/chainlightning.pkb",
	} {
		if got, ok := NormalizeGamePath(line); !ok || got != want {
			t.Errorf("NormalizeGamePath(%q) = %q, %v", line, got, ok)
		}
	}
	for _, line := range []string{
		"war3.w3mod:Sound/Music/mp3Music/ArthasTheme.mp3", "war3.w3mod:Units/UnitData.slk", "", "   ",
	} {
		if got, ok := NormalizeGamePath(line); ok {
			t.Errorf("NormalizeGamePath(%q) kept %q", line, got)
		}
	}
}

func TestRenderGamePathsWritesAHeaderAndSortedUniquePaths(t *testing.T) {
	list := strings.Join([]string{
		"war3.w3mod:Textures/Black32.blp",
		"war3.w3mod:_hd.w3mod:Textures/Black32.blp",
		"war3.w3mod:Units/Human/Footman/Footman.mdx",
		"war3.w3mod:Sound/Hit.wav",
		"war3.w3mod:Abilities/Spells/Human/Heal/Heal.mdx",
	}, "\r\n")
	want := strings.Join([]string{
		"# Warcraft III 3.0.0.24268",
		"abilities/spells/human/heal/heal.mdx",
		"textures/black32.blp",
		"units/human/footman/footman.mdx",
		"",
	}, "\n")
	if got := RenderGamePaths(list, "3.0.0.24268"); got != want {
		t.Errorf("RenderGamePaths = %q", got)
	}
}

func TestGamePathKeyIgnoresTextureExtensionsLetterCaseAndSeparatorsAndReadsMdlAsMdx(t *testing.T) {
	same := [][2]string{
		{`Doodads\LordaeronSummer\Plants\Corn\plant1_Normal.tif`, "doodads/lordaeronsummer/plants/corn/plant1_normal.dds"},
		{`Textures\Black32.BLP`, "textures/black32.dds"},
		{`Models\Glow.mdl`, "models/glow.mdx"},
	}
	for _, pair := range same {
		if GamePathKey(pair[0]) != GamePathKey(pair[1]) {
			t.Errorf("%q and %q have different keys", pair[0], pair[1])
		}
	}
	different := [][2]string{
		{`Models\Glow.mdx`, "models/glow.blp"},
		{`Textures\Other\Black32.blp`, "textures/black32.blp"},
	}
	for _, pair := range different {
		if GamePathKey(pair[0]) == GamePathKey(pair[1]) {
			t.Errorf("%q and %q have one key", pair[0], pair[1])
		}
	}
}

func TestParseGamePathsSkipsCommentsAndBlankLines(t *testing.T) {
	keys := ParseGamePaths("# Warcraft III 3.0\n\ntextures/black32.blp\nunits/human/footman/footman.mdx\n")
	if len(keys) != 2 || !keys[GamePathKey(`Textures\Black32.dds`)] || !keys[GamePathKey(`Units\Human\Footman\Footman.mdl`)] {
		t.Errorf("keys = %v", keys)
	}
	if got := ParseGamePaths("# not generated yet\n"); len(got) != 0 {
		t.Errorf("a list with only a comment has keys %v", got)
	}
}

func TestTheEmbeddedPathListIsTheGames(t *testing.T) {
	keys := LoadGamePaths()
	if len(keys) < 30000 || !keys[GamePathKey(`Units\Human\Footman\Footman.mdx`)] || !keys[GamePathKey(`Textures\Black32.blp`)] {
		t.Errorf("the embedded list has %d keys", len(keys))
	}
}
