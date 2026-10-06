package model_test

import (
	"errors"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/war3/model"
)

// modelFile is the name the tests give a reader for its errors.
const modelFile = "assets/Knight.mdx"

// knight is a text model with every block that bears a path, and blocks that bear none.
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

var knightPaths = []model.Path{
	{Kind: model.Texture, Path: `Textures\Knight.blp`},
	{Kind: model.Texture, ReplaceableID: 1},
	{Kind: model.Texture, ReplaceableID: 2},
	{Kind: model.ParticleModel, Path: `Abilities\Spells\Human\Heal.mdx`},
	{Kind: model.ParticleTexture, Path: `Textures\Spark.blp`},
	{Kind: model.Attachment, Path: `Models\Sword.mdx`},
	{Kind: model.Popcorn, Path: `Effects\Fire.pkfx`},
	{Kind: model.FaceEffect, Path: `FaceFX\Knight.facefx`},
}

// header is the start of a text model: without a Version or a Model block a text is not one.
const header = "Version {\n\tFormatVersion 800,\n}\n"

// binaryModel is a binary model the tests build, with what it references.
type binaryModel struct {
	name string
	data []byte
	want []model.Path
}

// binaryModels are the binary models that read.
func binaryModels() []binaryModel {
	return []binaryModel{
		{"no chunks", testkit.MDX(), nil},
		{"one texture", testkit.MDX(testkit.Chunk("TEXS", testkit.Texture(`Textures\A.blp`, 0))), []model.Path{
			{Kind: model.Texture, Path: `Textures\A.blp`},
		}},
		{"textures and replaceable slots", testkit.MDX(
			testkit.Chunk("VERS", testkit.U32(800)),
			testkit.Chunk("TEXS", testkit.Concat(
				testkit.Texture(`Textures\Knight.blp`, 0), testkit.Texture("", 1), testkit.Texture("", 2),
				testkit.Texture("", 11),
			)),
		), []model.Path{
			{Kind: model.Texture, Path: `Textures\Knight.blp`},
			{Kind: model.Texture, ReplaceableID: 1},
			{Kind: model.Texture, ReplaceableID: 2},
			{Kind: model.Texture, ReplaceableID: 11},
		}},
		{"emitters, attachments, popcorn and face effects", testkit.MDX(
			testkit.Chunk("ZZZZ", make([]byte, 13)), // a chunk the reader does not know is passed over by its size
			testkit.Chunk("PREM", testkit.Concat(
				testkit.Emitter(`Abilities\Spells\Human\Heal.mdx`, testkit.EmitterUsesMDL),
				testkit.Emitter(`Textures\Spark.blp`, testkit.EmitterUsesTGA),
				testkit.Emitter(`Models\Default.mdx`, 0),
				testkit.Emitter("", testkit.EmitterUsesMDL),
			)),
			testkit.Chunk("ATCH", testkit.Concat(testkit.Attachment(`Models\Sword.mdx`), testkit.Attachment(""))),
			testkit.Chunk("CORN", testkit.Popcorn(`Effects\Fire.pkfx`)),
			testkit.Chunk("FAFX", testkit.FaceEffect("Head", `FaceFX\Knight.facefx`)),
		), []model.Path{
			{Kind: model.ParticleModel, Path: `Abilities\Spells\Human\Heal.mdx`},
			{Kind: model.ParticleTexture, Path: `Textures\Spark.blp`},
			{Kind: model.ParticleModel, Path: `Models\Default.mdx`},
			{Kind: model.Attachment, Path: `Models\Sword.mdx`},
			{Kind: model.Popcorn, Path: `Effects\Fire.pkfx`},
			{Kind: model.FaceEffect, Path: `FaceFX\Knight.facefx`},
		}},
		{"an emitter with both flags, an empty popcorn and an empty face effect", testkit.MDX(
			testkit.Chunk("TEXS", nil),
			testkit.Chunk("PREM", testkit.Emitter(`Models\Both.mdx`, testkit.EmitterUsesMDL|testkit.EmitterUsesTGA)),
			testkit.Chunk("CORN", testkit.Concat(testkit.Popcorn(""), testkit.Popcorn(`Effects\Smoke.pkb`))),
			testkit.Chunk("FAFX", testkit.Concat(testkit.FaceEffect("None", ""), testkit.FaceEffect("Head", "a.facefx"))),
			testkit.Chunk("ATCH", nil),
		), []model.Path{
			{Kind: model.ParticleModel, Path: `Models\Both.mdx`},
			{Kind: model.Popcorn, Path: `Effects\Smoke.pkb`},
			{Kind: model.FaceEffect, Path: "a.facefx"},
		}},
		// The reader leaves the magic to IsMDX and reads what follows the first four bytes.
		{"bytes shorter than the magic", []byte("MD"), nil},
	}
}

// damagedModel is a binary model that does not read, with the words that tell its refusal from the others.
type damagedModel struct {
	name  string
	data  []byte
	words string
}

// damagedModels are binary models with one thing wrong in each.
func damagedModels() []damagedModel {
	whole := testkit.MDX(testkit.Chunk("TEXS", testkit.Texture("a.blp", 0)))
	attachment := testkit.Attachment(`Models\Sword.mdx`)
	emitter := testkit.Emitter(`Models\Default.mdx`, 0)
	popcorn := testkit.Popcorn(`Effects\Fire.pkfx`)
	chunk := func(tag string, body []byte) []byte { return testkit.MDX(testkit.Chunk(tag, body)) }
	return []damagedModel{
		{"bytes after the last chunk", testkit.Concat(testkit.MDX(), []byte{1, 2, 3}), "a chunk header is cut off"},
		{"one byte after the last chunk", testkit.Concat(whole, []byte{1}), "a chunk header is cut off"},
		{"one byte after the magic", testkit.Concat(testkit.MDX(), []byte{1}), "a chunk header is cut off"},
		{"a chunk cut short", whole[:100], "the TEXS chunk runs past the end of the file"},
		{"textures of a wrong size", chunk("TEXS", make([]byte, 100)), "the TEXS chunk is not a whole number of textures"},
		{"face effects of a wrong size", chunk("FAFX", make([]byte, 100)),
			"the FAFX chunk is not a whole number of face effects"},
		{"a record larger than its chunk", chunk("ATCH", testkit.SetU32(attachment, 0, 9999)),
			"the ATCH chunk has a record with an invalid size"},
		{"a node smaller than a node", chunk("ATCH", testkit.SetU32(attachment, 4, 10)),
			"the ATCH chunk has a node with an invalid size"},
		{"a record that ends at its node", chunk("ATCH", testkit.SetU32(attachment, 0, 4+96)),
			"the ATCH chunk has a record too small for its path"},
		{"bytes after the last record", chunk("ATCH", testkit.Concat(attachment, []byte{1, 2, 3})),
			"the ATCH chunk has a record that is cut off"},
		// Four bytes are a size, so the record is not cut off: it is smaller than a node.
		{"a size after the last record", chunk("ATCH", testkit.Concat(attachment, testkit.U32(4))),
			"the ATCH chunk has a record with an invalid size"},
		{"a record smaller than a node", chunk("PREM", testkit.SetU32(emitter, 0, 99)),
			"the PREM chunk has a record with an invalid size"},
		{"a node larger than its record", chunk("PREM", testkit.SetU32(emitter, 4, uint32(len(emitter)-3))),
			"the PREM chunk has a node with an invalid size"},
		{"a node that leaves no room for the path", chunk("CORN", testkit.SetU32(popcorn, 4, 96+261)),
			"the CORN chunk has a record too small for its path"},
		// The bytes of a tag are printed as the characters of Latin-1.
		{"a chunk with a tag that is not ASCII, cut short", testkit.Concat(testkit.MDX(), []byte("T\xC9XS"), testkit.U32(9)),
			"the T\xC3\x89XS chunk runs past the end of the file"},
	}
}

// brokenText is a text that does not read as a model, with the words that tell its refusal from the others.
type brokenText struct {
	source string
	words  string
}

// brokenTexts are texts whose strings or blocks do not close.
func brokenTexts() []brokenText {
	return []brokenText{
		{`Bitmap { Image "Textures\A.blp`, "a string is never closed"},
		{"}", "a } has no matching {"},
		{header + "Bitmap {\n}\n}\nBitmap {", "a } has no matching {"},
		{`Textures 1 { Bitmap { Image "a.blp", }`, "the Textures block is never closed"},
		{`Version { } { Path "x" `, "the unnamed block is never closed"},
		{`Version { } "Knight" { `, "the unnamed block is never closed"},
	}
}

// textsThatAreNoModel are texts in which every string and block closes, without a Version or a Model block.
func textsThatAreNoModel() []string {
	return []string{
		"",
		"hello world",
		"version https://git-lfs.github.com/spec/v1\noid sha256:abc\nsize 123\n", // a Git LFS pointer
		"Textures 1 {\n\tBitmap {\n\t\tImage \"a.blp\",\n\t}\n}\n",               // a block, but no Version or Model
		"Textures 1 {\n\tVersion {\n\t}\n}\n",                                    // a nested Version block does not count
		"\"Version\" {\n}\n",                                                     // a string does not name a block
	}
}

// headerOnlyTexts are text models of one block.
func headerOnlyTexts() []string {
	return []string{"Version {\n\tFormatVersion 800,\n}\n", "Model \"A\" {\n\tNumGeosets 0,\n}\n"}
}

// readsUnnaturally matches a message that says "a TEXS chunk" where it means the one chunk the file has.
var readsUnnaturally = regexp.MustCompile(`\ba [A-Z]{4}\b`)

// refused checks that err says the model is not readable, names the file through File and nowhere else, has a
// hint, and has the words.
func refused(t *testing.T, what string, err error, file, words string) {
	t.Helper()
	var failure *diag.Error
	if !errors.As(err, &failure) || failure.File != file {
		t.Errorf("%s: got %v, want an error naming %s", what, err, file)
		return
	}
	if failure.Msg != "Not a readable model: "+words+"." {
		t.Errorf("%s: message %q, want a model that is not readable and %q", what, failure.Msg, words)
	}
	const hint = "Re-export it from your modelling tool, or open it in a model viewer to check it."
	if strings.Contains(failure.Msg, "Knight.md") || readsUnnaturally.MatchString(failure.Msg) || failure.Hint != hint {
		t.Errorf("%s: message %q, hint %q: the file is named once, through File, and there is a hint", what,
			failure.Msg, failure.Hint)
	}
}

func TestReadMDLReadsEveryPathBearingBlockAndIgnoresTheRest(t *testing.T) {
	got, err := model.ReadMDL(knight, "knight.mdl")
	if err != nil || !slices.Equal(got, knightPaths) {
		t.Errorf("ReadMDL = %+v, %v", got, err)
	}
}

func TestReadMDLRefusesBrokenText(t *testing.T) {
	for _, broken := range brokenTexts() {
		got, err := model.ReadMDL(broken.source, "assets/Knight.mdl")
		refused(t, broken.source, err, "assets/Knight.mdl", broken.words)
		if got != nil {
			t.Errorf("%q: a refused text returned %+v", broken.source, got)
		}
	}
}

func TestPathsPicksTheReaderFromTheContent(t *testing.T) {
	got, err := model.Paths([]byte(knight), "knight.mdl")
	if err != nil || !slices.Equal(got, knightPaths) {
		t.Errorf("Paths of text = %+v, %v", got, err)
	}
	for _, binary := range binaryModels() {
		if !model.IsMDX(binary.data) {
			continue
		}
		got, err := model.Paths(binary.data, "a.mdx")
		if err != nil || !slices.Equal(got, binary.want) {
			t.Errorf("Paths of binary, %s = %+v, %v", binary.name, got, err)
		}
	}
	blp := []byte{0x42, 0x4c, 0x50, 0x31, 0, 0, 0, 0} // a BLP texture, not a model
	_, err = model.Paths(blp, "icon.blp")
	refused(t, "a texture", err, "icon.blp", "it is neither a binary MDX nor a text MDL file")
}

func TestPathsRefusesTextThatIsNotAModel(t *testing.T) {
	for _, source := range textsThatAreNoModel() {
		_, err := model.Paths([]byte(source), modelFile)
		refused(t, source, err, modelFile, "it has no Version or Model block")
	}
	_, err := model.Paths([]byte{0x4d, 0x44}, modelFile) // "MD": too short to be an MDX
	refused(t, "MD", err, modelFile, "it has no Version or Model block")
}

func TestReadMDLAcceptsAModelWithOnlyAVersionBlockOrOnlyAModelBlock(t *testing.T) {
	for _, source := range headerOnlyTexts() {
		if got, err := model.ReadMDL(source, "a.mdl"); err != nil || len(got) != 0 {
			t.Errorf("ReadMDL(%q) = %+v, %v", source, got, err)
		}
	}
}

func TestIsMDXRecognizesTheMDLXMagic(t *testing.T) {
	for _, c := range []struct {
		data []byte
		want bool
	}{
		{testkit.MDX(), true},
		{testkit.MDX(testkit.Chunk("VERS", testkit.U32(800))), true},
		{[]byte("Version {"), false},
		{[]byte{0x4d, 0x44}, false},
		{[]byte("mdlx"), false},
		{nil, false},
	} {
		if got := model.IsMDX(c.data); got != c.want {
			t.Errorf("IsMDX(%q) = %v", c.data, got)
		}
	}
}

func TestReadMDXReadsTexturesEmittersAttachmentsPopcornAndFaceEffectsInFileOrder(t *testing.T) {
	for _, binary := range binaryModels() {
		if got, err := model.ReadMDX(binary.data, "knight.mdx"); err != nil || !slices.Equal(got, binary.want) {
			t.Errorf("%s: ReadMDX = %+v, %v", binary.name, got, err)
		}
	}
}

func TestReadMDXRefusesDamagedFiles(t *testing.T) {
	for _, damaged := range damagedModels() {
		got, err := model.ReadMDX(damaged.data, modelFile)
		refused(t, damaged.name, err, modelFile, damaged.words)
		if got != nil {
			t.Errorf("%s: a refused file returned %+v", damaged.name, got)
		}
	}
}

func TestReadMDXReturnsNoPathsOfAFileItRefuses(t *testing.T) {
	data := testkit.MDX(
		testkit.Chunk("TEXS", testkit.Texture("a.blp", 0)),
		testkit.Chunk("ATCH", testkit.Concat(testkit.Attachment("a.mdx"), []byte{1})),
	)
	got, err := model.ReadMDX(data, modelFile)
	refused(t, "a second chunk that is damaged", err, modelFile, "the ATCH chunk has a record that is cut off")
	if got != nil {
		t.Errorf("a refused file returned %+v", got)
	}
}

func TestDescribeLabelsReplaceableTextures(t *testing.T) {
	for _, c := range []struct {
		path model.Path
		want string
	}{
		{model.Path{Kind: model.Texture, Path: `Textures\A.blp`}, `Textures\A.blp`},
		{model.Path{Kind: model.Texture, Path: `Textures\A.blp`, ReplaceableID: 1}, `Textures\A.blp`},
		{model.Path{Kind: model.Texture, ReplaceableID: 1}, "team colour (slot 1)"},
		{model.Path{Kind: model.Texture, ReplaceableID: 2}, "team glow (slot 2)"},
		{model.Path{Kind: model.Texture, ReplaceableID: 11}, "replaceable texture (slot 11)"},
		{model.Path{Kind: model.Texture}, "replaceable texture (slot 0)"},
	} {
		if got := model.Describe(c.path); got != c.want {
			t.Errorf("Describe(%+v) = %q, want %q", c.path, got, c.want)
		}
	}
}

// textModel is the blocks of a text model after its header, with what they reference.
type textModel struct {
	name   string
	blocks string
	want   []model.Path
}

// emitterTexts are particle emitters whose path is in a Particle block, in the emitter, in both or in neither.
func emitterTexts() []textModel {
	heal := []model.Path{{Kind: model.ParticleModel, Path: "heal.mdx"}}
	return []textModel{
		{"the path in the Particle block", `ParticleEmitter "A" { Particle { Path "heal.mdx", } }`, heal},
		{"a path of its own before the Particle block", `ParticleEmitter "A" { Path "heal.mdx", Particle { Path "b.mdx", } }`, heal},
		{"an empty path of its own before the Particle block", `ParticleEmitter "A" { Path "", Particle { Path "b.mdx", } }`, nil},
		{"a path of its own after the Particle block", `ParticleEmitter "A" { Particle { Path "b.mdx", } Path "heal.mdx", }`, heal},
		{"two Particle blocks", `ParticleEmitter "A" { Particle { Path "heal.mdx", } Particle { Path "b.mdx", } }`, heal},
		{"the flags of the emitter, not of the Particle block",
			`ParticleEmitter "A" { EmitterUsesTGA, Particle { EmitterUsesMDL, Path "spark.blp", } }`,
			[]model.Path{{Kind: model.ParticleTexture, Path: "spark.blp"}}},
		{"a Particle block deeper than the emitter's own", `ParticleEmitter "A" { Other { Particle { Path "b.mdx", } } }`, nil},
		{"a Particle block of another block", `Attachment "A" { Particle { Path "b.mdx", } }`, nil},
		{"a Particle block of no block", `Particle { Path "b.mdx", }`, nil},
	}
}

// statementTexts are blocks whose statements have each shape a statement may have, and each way of writing the
// tokens between them.
func statementTexts() []textModel {
	texture := func(path string, slot int64) []model.Path {
		return []model.Path{{Kind: model.Texture, Path: path, ReplaceableID: slot}}
	}
	return []textModel{
		{"a block without statements", "Bitmap { }", texture("", 0)},
		{"a last statement without a comma", `Bitmap { Image "a.blp" }`, texture("a.blp", 0)},
		{"the last of two values", `Bitmap { Image "a.blp", Image "b.blp", ReplaceableId 1, ReplaceableId 2, }`, texture("b.blp", 2)},
		{"a negative slot", "Bitmap { ReplaceableId -3, }", texture("", -3)},
		{"a slot too large for 64 bits", "Bitmap { ReplaceableId 99999999999999999999, }", texture("", 1<<63-1)},
		{"a slot that is not a whole number", "Bitmap { ReplaceableId 1.5, ReplaceableId +2, ReplaceableId 0x3, ReplaceableId -, }",
			texture("", 0)},
		{"a slot written as a string", `Bitmap { ReplaceableId "1", }`, texture("", 0)},
		{"an image written as a word", "Bitmap { Image a.blp, }", texture("", 0)},
		{"a statement of three words", `Bitmap { static Image "a.blp", Image "b.blp" 1, }`, texture("", 0)},
		{"a statement that starts with a string", `Bitmap { "Image" "a.blp", }`, texture("", 0)},
		{"a path written as a flag", "Attachment { Path, }", nil},
		{"a path written as a word", "Attachment { Path a.mdx, }", nil},
		{"an empty path in each block that takes its file from one",
			`Attachment { Path "", } ParticleEmitter { Path "", } ParticleEmitterPopcorn { Path "", } FaceFX { Path "", }`, nil},
		{"a string after a word that is not Path", `Attachment { Image "a.mdx", Name "b.mdx", }`, nil},
		{"a string after a word that is not Image", `Bitmap { Path "a.blp", Name "b.blp", }`, texture("", 0)},
		{"a flag written with a value", `ParticleEmitter { EmitterUsesTGA 1, Path "a.blp", }`,
			[]model.Path{{Kind: model.ParticleModel, Path: "a.blp"}}},
		{"comments", "Bitmap { // Image \"no.blp\",\n\tImage \"a//b.blp\", // the slot\r\n\tReplaceableId 1, }// end", texture("a//b.blp", 1)},
		{"a word with a slash", "Bitmap { Image/ \"a.blp\", }", texture("", 0)},
		{"two slashes inside a word", "Bitmap { Image//x \"a.blp\", }", texture("", 0)},
		{"a word that starts with one slash", "Bitmap { /Image \"a.blp\", ReplaceableId /1, }", texture("", 0)},
		{"no white space at all", `Bitmap{Image"a.blp",ReplaceableId 1}Attachment"A"{Path"b.mdx"}`,
			[]model.Path{{Kind: model.Texture, Path: "a.blp", ReplaceableID: 1}, {Kind: model.Attachment, Path: "b.mdx"}}},
		{"every white space of ASCII", "Bitmap\v{\fImage\t\"a.blp\"\r,\nReplaceableId\v1\f}", texture("a.blp", 1)},
		{"a string over several lines", "Bitmap { Image \"a\n{b},\", }", texture("a\n{b},", 0)},
		{"a path-bearing block inside another", `Geoset { Bitmap { Image "a.blp", } }`, texture("a.blp", 0)},
		{"a block named by the first word before it", `Image "a.blp" Bitmap { }`, nil},
		{"a block named after a comma", `Image "a.blp", Bitmap { }`, texture("", 0)},
	}
}

func TestReadMDLTakesAnEmittersPathFromItsParticleBlockAndReadsAStatementByItsShape(t *testing.T) {
	for _, c := range slices.Concat(emitterTexts(), statementTexts()) {
		got, err := model.ReadMDL(header+c.blocks, "a.mdl")
		if err != nil || !slices.Equal(got, c.want) {
			t.Errorf("%s: ReadMDL = %+v, %v, want %+v", c.name, got, err, c.want)
		}
	}
}

// spacedText is a text model whose texture has a character outside ASCII between Image and its string.
func spacedText(space string) string {
	return header + "Bitmap {\n\tImage" + space + "\"a.blp\",\n}\n"
}

// spacesOutsideASCII are the characters outside ASCII that are white space to some readers and text to ReadMDL.
func spacesOutsideASCII() map[string]string {
	return map[string]string{
		"a no-break space":              "\xC2\xA0",
		"an ogham space mark":           "\xE1\x9A\x80",
		"an en quad":                    "\xE2\x80\x80",
		"a hair space":                  "\xE2\x80\x8A",
		"a line separator":              "\xE2\x80\xA8",
		"a paragraph separator":         "\xE2\x80\xA9",
		"a narrow no-break space":       "\xE2\x80\xAF",
		"a medium mathematical space":   "\xE2\x81\x9F",
		"an ideographic space":          "\xE3\x80\x80",
		"a byte order mark in the text": "\xEF\xBB\xBF",
	}
}

func TestOnlyASCIIWhiteSpaceSeparatesTheTokensOfATextModel(t *testing.T) {
	noImage := []model.Path{{Kind: model.Texture}}
	for name, space := range spacesOutsideASCII() {
		// The character is part of the word Image, so the texture has no image.
		got, err := model.ReadMDL(spacedText(space), "a.mdl")
		if err != nil || !slices.Equal(got, noImage) {
			t.Errorf("%s: ReadMDL = %+v, %v, want %+v", name, got, err, noImage)
		}
	}
}

func TestPathsDropsALeadingByteOrderMarkAndReadMDLTakesItsTextAsItIs(t *testing.T) {
	marked := "\xEF\xBB\xBF" + header
	if got, err := model.Paths([]byte(marked), modelFile); err != nil || len(got) != 0 {
		t.Errorf("Paths = %+v, %v", got, err)
	}
	_, err := model.ReadMDL(marked, modelFile)
	refused(t, "a text that starts with a byte order mark", err, modelFile, "it has no Version or Model block")
}

// invalidBytes is a model with a path that is not UTF-8, and the path Paths reads from it.
type invalidBytes struct {
	name string
	data []byte
	want string
}

// replacement is U+FFFD, which stands for bytes that are not UTF-8.
const replacement = "\xEF\xBF\xBD"

// runsOfInvalidBytes are models with a path that holds several invalid sequences in a row. Paths reads each run as
// one U+FFFD.
func runsOfInvalidBytes() []invalidBytes {
	binary := func(path string) []byte { return testkit.MDX(testkit.Chunk("TEXS", testkit.Texture(path, 0))) }
	text := func(path string) []byte { return []byte(header + "Bitmap {\n\tImage \"" + path + "\",\n}\n") }
	return []invalidBytes{
		{"binary, two bytes", binary("a\xFF\xFE.blp"), "a" + replacement + ".blp"},
		{"binary, a cut sequence and a byte", binary("a\xE6\x9C\xFF.blp"), "a" + replacement + ".blp"},
		{"binary, two runs", binary("\xC0\x80a\xF8\x88.blp"), replacement + "a" + replacement + ".blp"},
		{"text, two bytes", text("a\xFF\xFE.blp"), "a" + replacement + ".blp"},
		{"text, a cut sequence and a byte", text("a\xE6\x9C\xFF.blp"), "a" + replacement + ".blp"},
		{"text, a surrogate", text("\xED\xA0\x80.blp"), replacement + ".blp"},
	}
}

// singleInvalidSequences are models with a path that holds invalid sequences one at a time, and a byte order mark.
func singleInvalidSequences() []invalidBytes {
	binary := func(path string) []byte { return testkit.MDX(testkit.Chunk("TEXS", testkit.Texture(path, 0))) }
	text := func(path string) []byte { return []byte(header + "Bitmap {\n\tImage \"" + path + "\",\n}\n") }
	return []invalidBytes{
		{"binary, one byte", binary("a\xFF.blp"), "a" + replacement + ".blp"},
		{"binary, a cut sequence", binary("a\xE6\x9C.blp"), "a" + replacement + ".blp"},
		{"binary, bytes apart", binary("\xFFa\xFE"), replacement + "a" + replacement},
		{"binary, a byte order mark in front", binary("\xEF\xBB\xBFa.blp"), "a.blp"},
		{"binary, a byte order mark inside", binary("a\xEF\xBB\xBF.blp"), "a\xEF\xBB\xBF.blp"},
		{"binary, not ASCII", binary("M\xC3\xA5ne\xE6\x9C\x88.blp"), "M\xC3\xA5ne\xE6\x9C\x88.blp"},
		{"text, one byte", text("a\xFF.blp"), "a" + replacement + ".blp"},
		{"text, a cut sequence", text("a\xE6\x9C.blp"), "a" + replacement + ".blp"},
		{"text, not ASCII", text("M\xC3\xA5ne\xE6\x9C\x88.blp"), "M\xC3\xA5ne\xE6\x9C\x88.blp"},
	}
}

func TestPathsReadsBytesThatAreNotUTF8AsOneReplacementCharacterForEachRun(t *testing.T) {
	for _, c := range slices.Concat(runsOfInvalidBytes(), singleInvalidSequences()) {
		got, err := model.Paths(c.data, modelFile)
		if err != nil || len(got) != 1 || got[0].Path != c.want {
			t.Errorf("%s: Paths = %+v, %v, want the path %q", c.name, got, err, c.want)
		}
	}
}
