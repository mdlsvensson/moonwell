package model_test

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	olddiag "github.com/mdlsvensson/moonwell/internal/diag"
	oldmodels "github.com/mdlsvensson/moonwell/internal/models"
	oldtestkit "github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/oracle"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/war3/model"
)

// What this file leaves out, and why. The two trees agree on every model made of ASCII, on text outside ASCII
// that is valid UTF-8, and on invalid bytes that come one sequence at a time, and those are compared here. They
// differ by intent in two things. The inputs that show a difference are pinned in model_test.go and given to
// neither tree in a comparison; TestOracleLeavesOutOnlyWhatTheTreesReadDifferently proves of each that the other
// tree reads it differently.
//
//   - White space outside ASCII in a text model. The other tree takes the no-break space, the other spaces outside
//     ASCII, the line and paragraph separators and a byte order mark that is not dropped for white space between
//     tokens, and this tree takes them for text. Left out: a text with one of those characters beside a token
//     (TestOnlyASCIIWhiteSpaceSeparatesTheTokensOfATextModel), and a text given to ReadMDL with its byte order
//     mark (TestPathsDropsALeadingByteOrderMarkAndReadMDLTakesItsTextAsItIs).
//   - A run of invalid bytes, in a path of a binary model or in a text model that Paths decodes. The other tree
//     reads each invalid sequence of the run as U+FFFD, and this tree reads the whole run as one. Left out: a path
//     with two invalid sequences in a row (TestPathsReadsBytesThatAreNotUTF8AsOneReplacementCharacterForEachRun).
//
// comparison.model applies the note to every input: it does not give ReadMDL, or Paths when it reads the bytes as
// text, a text with one of those characters anywhere, and it does not compare a reading in which the other tree
// read two U+FFFD in a row. Each test of built models says how many readings it means to leave out so: three of
// the models of the tests, at most one in fifty of the altered models, and none of the others.

// comparison runs the other tree's package and this one on the same input, compares what they return through the
// oracle, and counts the comparisons it made, the readings that gave paths, the readings the other tree refused and
// the readings it left out.
type comparison struct {
	t       *testing.T
	count   int
	read    int
	refused int
	leftOut int
	// problems are the messages of the refusals compared.
	problems map[string]bool
}

func newComparison(t *testing.T) *comparison {
	return &comparison{t: t, problems: map[string]bool{}}
}

// said is what an error tells its reader, in the shape both trees are compared in.
type said struct {
	Expected        bool // a diag error
	Msg, File, Hint string
	Line, Column    int
}

func saidByTheOtherTree(err error) said {
	var failure *olddiag.Error
	if errors.As(err, &failure) {
		return said{true, failure.Msg, failure.File, failure.Hint, failure.Line, failure.Column}
	}
	return said{Msg: err.Error()}
}

func saidByThisTree(err error) said {
	var failure *diag.Error
	if errors.As(err, &failure) {
		return said{true, failure.Msg, failure.File, failure.Hint, failure.Line, failure.Column}
	}
	return said{Msg: err.Error()}
}

// errors compares two errors: that both are nil or both are not, and for two errors every word they say.
func (c *comparison) errors(what string, want, got error) {
	c.t.Helper()
	c.count++
	if oracle.Errors(c.t, what, want, got) {
		c.values(what+": the error", saidByTheOtherTree(want), saidByThisTree(got))
	}
}

func (c *comparison) values(what string, want, got any) {
	c.t.Helper()
	c.count++
	oracle.Values(c.t, what, want, got)
}

func (c *comparison) bytes(what string, want, got []byte) {
	c.t.Helper()
	c.count++
	oracle.Bytes(c.t, what, want, got)
}

// reading compares what one reader returned in both trees: the error, the paths, and what Describe says of each
// path.
func (c *comparison) reading(what string, want []oldmodels.Path, wantErr error, got []model.Path, gotErr error) {
	c.t.Helper()
	c.errors(what, wantErr, gotErr)
	c.values(what, want, got)
	var wantDescribed, gotDescribed []string
	for _, path := range want {
		wantDescribed = append(wantDescribed, oldmodels.Describe(path))
	}
	for _, path := range got {
		gotDescribed = append(gotDescribed, model.Describe(path))
	}
	c.values(what+": described", wantDescribed, gotDescribed)
	switch {
	case wantErr != nil:
		c.refused++
		c.problems[wantErr.Error()] = true
	case len(want) > 0:
		c.read++
	}
}

// byteOrderMark is what Paths drops from the start of a text.
const byteOrderMark = "\xEF\xBB\xBF"

// otherSpaces are the characters outside ASCII that are white space to the other tree and text to this one: the
// no-break space, the ogham space mark, the eleven spaces from the en quad to the hair space, the line and the
// paragraph separator, the narrow no-break space, the medium mathematical space, the ideographic space and the
// byte order mark.
const otherSpaces = "\xC2\xA0\xE1\x9A\x80" +
	"\xE2\x80\x80\xE2\x80\x81\xE2\x80\x82\xE2\x80\x83\xE2\x80\x84\xE2\x80\x85\xE2\x80\x86\xE2\x80\x87\xE2\x80\x88\xE2\x80\x89\xE2\x80\x8A" +
	"\xE2\x80\xA8\xE2\x80\xA9\xE2\x80\xAF\xE2\x81\x9F\xE3\x80\x80" + byteOrderMark

// spaceOutsideASCII reports whether a text holds a character the two trees take differently, wherever it stands.
func spaceOutsideASCII(text string) bool {
	return strings.ContainsAny(text, otherSpaces)
}

// readFromARunOfInvalidBytes reports whether what the other tree read has two U+FFFD in a row, in a path or in the
// name of a block its error gives: it reads them from invalid sequences in a row.
func readFromARunOfInvalidBytes(paths []oldmodels.Path, err error) bool {
	run := replacement + replacement
	return err != nil && strings.Contains(err.Error(), run) ||
		slices.ContainsFunc(paths, func(path oldmodels.Path) bool { return strings.Contains(path.Path, run) })
}

// model compares everything both trees make of the bytes: whether they are MDX, and what ReadMDX, Paths and
// ReadMDL read from them. ReadMDX and ReadMDL are given every input, also the ones Paths gives the other reader.
// A reading that the note at the top of this file leaves out is counted and not compared.
func (c *comparison) model(what string, data []byte) {
	c.t.Helper()
	text := string(data)
	c.values(what+": IsMDX", oldmodels.IsMDX(data), model.IsMDX(data))

	want, wantErr := oldmodels.ReadMDX(data, modelFile)
	if readFromARunOfInvalidBytes(want, wantErr) {
		c.leftOut++
	} else {
		got, gotErr := model.ReadMDX(data, modelFile)
		c.reading(what+": ReadMDX", want, wantErr, got, gotErr)
	}

	want, wantErr = oldmodels.Paths(data, modelFile)
	asText := !oldmodels.IsMDX(data) && !strings.Contains(text, "\x00")
	if readFromARunOfInvalidBytes(want, wantErr) || asText && spaceOutsideASCII(strings.TrimPrefix(text, byteOrderMark)) {
		c.leftOut++
	} else {
		got, gotErr := model.Paths(data, modelFile)
		c.reading(what+": Paths", want, wantErr, got, gotErr)
	}

	if spaceOutsideASCII(text) {
		c.leftOut++
	} else {
		want, wantErr = oldmodels.ReadMDL(text, modelFile)
		got, gotErr := model.ReadMDL(text, modelFile)
		c.reading(what+": ReadMDL", want, wantErr, got, gotErr)
	}
}

// summary logs what a test compared.
func (c *comparison) summary(inputs int) {
	c.t.Helper()
	c.t.Logf("%d inputs, %d readings with paths, %d refused with %d different messages, %d left out, %d comparisons",
		inputs, c.read, c.refused, len(c.problems), c.leftOut, c.count)
}

// leavesOut fails the test unless as many readings were left out as the test means to leave out.
func (c *comparison) leavesOut(readings int, which string) {
	c.t.Helper()
	if c.leftOut != readings {
		c.t.Errorf("%d readings were left out, want %d: %s", c.leftOut, readings, which)
	}
}

// compared fails the test unless a refusal with each of the words was compared.
func (c *comparison) compared(words ...string) {
	c.t.Helper()
	for _, wanted := range words {
		found := false
		for problem := range c.problems {
			found = found || strings.Contains(problem, wanted)
		}
		if !found {
			c.t.Errorf("no refusal with %q was compared", wanted)
		}
	}
}

// binaryRefusals and textRefusals are the words of every refusal the two readers have.
var (
	binaryRefusals = []string{
		"a chunk header is cut off", "runs past the end of the file", "not a whole number of textures",
		"not a whole number of face effects", "a record that is cut off", "a record with an invalid size",
		"a node with an invalid size", "a record too small for its path",
	}
	textRefusals = []string{
		"a string is never closed", "a } has no matching {", "block is never closed", "the unnamed block",
		"it has no Version or Model block",
	}
)

// TestOracleLeavesOutOnlyWhatTheTreesReadDifferently keeps the note above honest: the other tree reads every
// input that is pinned and not compared in another way than this tree does. One that it comes to read the same
// belongs in a comparison.
func TestOracleLeavesOutOnlyWhatTheTreesReadDifferently(t *testing.T) {
	leftOut := 0
	for name, space := range spacesOutsideASCII() {
		other, err := oldmodels.ReadMDL(spacedText(space), modelFile)
		if err != nil || len(other) != 1 || other[0].Path != "a.blp" {
			t.Errorf("%s: the other tree reads %+v, %v, and not the image, so the text need not be left out", name, other, err)
		}
		leftOut++
	}
	if _, err := oldmodels.ReadMDL(byteOrderMark+header, modelFile); err != nil {
		t.Errorf("the other tree refuses a text with a byte order mark too, so it need not be left out: %v", err)
	}
	leftOut++
	for _, c := range runsOfInvalidBytes() {
		other, err := oldmodels.Paths(c.data, modelFile)
		if err != nil || len(other) != 1 || other[0].Path == c.want || !readFromARunOfInvalidBytes(other, nil) {
			t.Errorf("%s: the other tree reads %+v, %v, so the model need not be left out", c.name, other, err)
		}
		leftOut++
	}
	for name, space := range spacesOutsideASCII() {
		if !spaceOutsideASCII(space) {
			t.Errorf("%s is not among the characters a comparison leaves out", name)
		}
	}
	t.Logf("%d inputs left out", leftOut)
}

func TestOracleOnTheCarriedBuilders(t *testing.T) {
	c := newComparison(t)
	body := []byte{1, 2, 3}
	path := `Textures\Knight.blp`
	c.bytes("MDX", oldtestkit.MDX(), testkit.MDX())
	c.bytes("MDX of chunks", oldtestkit.MDX(body, body), testkit.MDX(body, body))
	c.bytes("Chunk", oldtestkit.Chunk("TEXS", body), testkit.Chunk("TEXS", body))
	c.bytes("Texture", oldtestkit.Texture(path, 11), testkit.Texture(path, 11))
	c.bytes("Emitter", oldtestkit.Emitter(path, 0x18000), testkit.Emitter(path, 0x18000))
	c.bytes("Attachment", oldtestkit.Attachment(path), testkit.Attachment(path))
	c.bytes("Popcorn", oldtestkit.Popcorn(path), testkit.Popcorn(path))
	c.bytes("FaceEffect", oldtestkit.FaceEffect("Head", path), testkit.FaceEffect("Head", path))
	c.values("the emitter flags", []int{oldtestkit.EmitterUsesMDL, oldtestkit.EmitterUsesTGA},
		[]int{testkit.EmitterUsesMDL, testkit.EmitterUsesTGA})
	t.Logf("%d comparisons", c.count)
}

func TestOracleOnDescribe(t *testing.T) {
	c := newComparison(t)
	kinds := []model.Kind{model.Texture, model.ParticleModel, model.ParticleTexture, model.Attachment, model.Popcorn,
		model.FaceEffect, ""}
	for _, kind := range kinds {
		for _, path := range []string{"", `Textures\A.blp`, " "} {
			for _, slot := range []int64{0, 1, 2, 3, 11, -1, 1<<32 - 1, 1<<63 - 1, -1 << 63} {
				want := oldmodels.Describe(oldmodels.Path{Kind: oldmodels.Kind(kind), Path: path, ReplaceableID: slot})
				got := model.Describe(model.Path{Kind: kind, Path: path, ReplaceableID: slot})
				c.values(fmt.Sprintf("Describe of %s %q in slot %d", kind, path, slot), want, got)
			}
		}
	}
	c.values("the kinds", []oldmodels.Kind{oldmodels.Texture, oldmodels.ParticleModel, oldmodels.ParticleTexture,
		oldmodels.Attachment, oldmodels.Popcorn, oldmodels.FaceEffect}, kinds[:6])
	t.Logf("%d comparisons", c.count)
}

// input is the bytes of one model, whole or not, with a name for a failure.
type input struct {
	name string
	data []byte
}

// modelsOfTheTests are every model the tests of this package read or refuse, but the ones the note above leaves
// out.
func modelsOfTheTests() []input {
	inputs := []input{
		{"the knight", []byte(knight)},
		{"the knight with CRLF", []byte(strings.ReplaceAll(knight, "\n", "\r\n"))},
		{"a texture file", []byte{0x42, 0x4c, 0x50, 0x31, 0, 0, 0, 0}},
		{"the knight with a byte order mark", []byte(byteOrderMark + knight)},
	}
	for _, binary := range binaryModels() {
		inputs = append(inputs, input{binary.name, binary.data})
	}
	for _, damaged := range damagedModels() {
		inputs = append(inputs, input{damaged.name, damaged.data})
	}
	for _, broken := range brokenTexts() {
		inputs = append(inputs, input{broken.source, []byte(broken.source)})
	}
	for _, source := range slices.Concat(textsThatAreNoModel(), headerOnlyTexts()) {
		inputs = append(inputs, input{source, []byte(source)})
	}
	for _, text := range slices.Concat(emitterTexts(), statementTexts()) {
		inputs = append(inputs, input{text.name, []byte(header + text.blocks)}, input{text.name + ", no header", []byte(text.blocks)})
	}
	for _, single := range singleInvalidSequences() {
		inputs = append(inputs, input{single.name, single.data})
	}
	return inputs
}

func TestOracleOnTheModelsOfTheTests(t *testing.T) {
	c := newComparison(t)
	inputs := modelsOfTheTests()
	for _, in := range inputs {
		c.model(in.name, in.data)
	}
	c.compared(slices.Concat(binaryRefusals, textRefusals, []string{"neither a binary MDX nor a text MDL"})...)
	if c.read < 60 {
		t.Errorf("only %d readings gave paths", c.read)
	}
	c.leavesOut(3, "ReadMDL of the knight with a byte order mark and of the two binary models with one in a path")
	c.summary(len(inputs))
}

// TestOracleOnModelsCutAtEveryLength proves that both trees read the same paths or report the same problem
// wherever a model ends.
func TestOracleOnModelsCutAtEveryLength(t *testing.T) {
	c := newComparison(t)
	whole := []input{
		{"the model with every chunk", modelWithEveryChunk()},
		{"the model with one texture", testkit.MDX(testkit.Chunk("TEXS", testkit.Texture(`Textures\A.blp`, 0)))},
		{"the knight", []byte(knight)},
		{"the text with other letters", []byte(textWithOtherLetters)},
	}
	cuts := 0
	for _, in := range whole {
		before := c.read
		for length := range len(in.data) + 1 {
			c.model(fmt.Sprintf("%s cut at %d bytes", in.name, length), in.data[:length:length])
			cuts++
		}
		if c.read == before {
			t.Errorf("%s: no cut gave paths", in.name)
		}
	}
	c.compared("a chunk header is cut off", "runs past the end of the file", "a string is never closed", "block is never closed",
		"it has no Version or Model block")
	c.leavesOut(0, "none")
	c.summary(cuts)
}

// seededName is a short name of small letters.
func seededName(random *rand.Rand) string {
	name := make([]byte, 1+random.IntN(12))
	for i := range name {
		name[i] = byte('a' + random.IntN(26))
	}
	return string(name)
}

// seededPath is a path as a model may hold one: none, with backslashes, with forward slashes, with spaces, with
// letters outside ASCII, of printable ASCII of any kind, or longer than the 259 bytes the field has room for.
func seededPath(random *rand.Rand) string {
	name := seededName(random)
	switch random.IntN(8) {
	case 0:
		return ""
	case 1:
		return `Textures\` + name + ".blp"
	case 2:
		return "Units/Human/" + name + "/" + name + ".mdx"
	case 3:
		return `war3mapImported\a folder\` + name + " " + name + ".tga"
	case 4:
		return "M\xC3\xA5ne\\\xE6\x9C\x88/" + name + "\xF0\x9F\x8C\x99.pkb"
	case 5:
		printable := make([]byte, random.IntN(40))
		for i := range printable {
			printable[i] = byte(' ' + random.IntN(95))
		}
		return string(printable)
	case 6:
		return strings.Repeat(`Folder\`+name+"/", 40)[:200+random.IntN(120)]
	}
	return name
}

// seededChunk is a chunk of any kind with up to three entries, or a chunk without paths.
func seededChunk(random *rand.Rand) []byte {
	entries := func(entry func() []byte) []byte {
		var body []byte
		for range random.IntN(4) {
			body = append(body, entry()...)
		}
		return body
	}
	flags := []uint32{0, testkit.EmitterUsesMDL, testkit.EmitterUsesTGA, testkit.EmitterUsesMDL | testkit.EmitterUsesTGA,
		0x10001, 0x7FFF, 0xFFFFFFFF, 0xFFFF7FFF, 0xFFFEFFFF}
	slots := []uint32{0, 0, 1, 2, 11, 0x7FFFFFFF, 0x80000000, 0xFFFFFFFF}
	switch random.IntN(7) {
	case 0:
		return testkit.Chunk("TEXS", entries(func() []byte {
			return testkit.Texture(seededPath(random), slots[random.IntN(len(slots))])
		}))
	case 1:
		return testkit.Chunk("PREM", entries(func() []byte {
			return testkit.Emitter(seededPath(random), flags[random.IntN(len(flags))])
		}))
	case 2:
		return testkit.Chunk("ATCH", entries(func() []byte { return testkit.Attachment(seededPath(random)) }))
	case 3:
		return testkit.Chunk("CORN", entries(func() []byte { return testkit.Popcorn(seededPath(random)) }))
	case 4:
		return testkit.Chunk("FAFX", entries(func() []byte { return testkit.FaceEffect(seededName(random), seededPath(random)) }))
	}
	tags := []string{"VERS", "MODL", "SEQS", "GEOS", "texs", "PRE2", "\x00\x00\x00\x00", "T\xC9XS"}
	body := make([]byte, random.IntN(60))
	for i := range body {
		body[i] = byte(random.IntN(128))
	}
	return testkit.Chunk(tags[random.IntN(len(tags))], body)
}

func TestOracleOnSeededBinaryModels(t *testing.T) {
	c := newComparison(t)
	random := rand.New(rand.NewPCG(13, 2026))
	const models = 300
	for i := range models {
		chunks := make([][]byte, random.IntN(8))
		for j := range chunks {
			chunks[j] = seededChunk(random)
		}
		data := testkit.MDX(chunks...)
		if _, err := oldmodels.ReadMDX(data, modelFile); err != nil {
			t.Errorf("seeded binary model %d: the other tree refuses a whole model: %v", i, err)
		}
		c.model(fmt.Sprintf("seeded binary model %d", i), data)
	}
	if c.read < models {
		t.Errorf("only %d readings gave paths", c.read)
	}
	c.leavesOut(0, "none")
	c.summary(models)
}

// altered is data with one change: a number set to a value at or past an edge of what a size or a flag may be, a
// byte put in, or a byte taken out.
func altered(random *rand.Rand, data []byte) []byte {
	edges := []uint32{0, 1, 3, 4, 8, 95, 96, 97, 99, 100, 101, 104, 259, 260, 264, 268, 340, 356, 360, 372, 404, 652,
		0x8000, 0x10000, 0x18000, 0x7FFFFFFF, 0x80000000, 0xFFFFFFFF}
	at := random.IntN(len(data) - 3)
	switch random.IntN(8) {
	case 0:
		return slices.Concat(data[:at], []byte{byte(random.IntN(256))}, data[at:])
	case 1:
		return slices.Concat(data[:at], data[at+1:])
	}
	return testkit.SetU32(data, at, edges[random.IntN(len(edges))])
}

// sizeOffsets are the offsets in the model with every chunk of the numbers that say how large something is: the
// size of each chunk, of each record and of each node, found by walking the model as the builders wrote it.
func sizeOffsets(t *testing.T, data []byte) []int {
	t.Helper()
	u32 := func(at int) int {
		return int(data[at]) | int(data[at+1])<<8 | int(data[at+2])<<16 | int(data[at+3])<<24
	}
	var offsets []int
	for at := 4; at < len(data); {
		tag, size := string(data[at:at+4]), u32(at+4)
		offsets = append(offsets, at+4)
		if tag == "PREM" || tag == "ATCH" || tag == "CORN" {
			for record := at + 8; record < at+8+size; record += u32(record) {
				offsets = append(offsets, record, record+4)
			}
		}
		at += 8 + size
	}
	if len(offsets) < 15 {
		t.Fatalf("found only %d sizes in the model", len(offsets))
	}
	return offsets
}

// TestOracleOnAlteredBinaryModels compares models with one thing wrong: every size of the model with every chunk
// set to every value near it and to the edges, and seeded changes anywhere in the model.
func TestOracleOnAlteredBinaryModels(t *testing.T) {
	c := newComparison(t)
	whole := modelWithEveryChunk()
	var inputs []input
	for _, at := range sizeOffsets(t, whole) {
		size := int64(whole[at]) | int64(whole[at+1])<<8
		for _, value := range []int64{0, 3, 4, 95, 96, 99, 100, 103, 104, size - 261, size - 260, size - 5, size - 4, size - 1,
			size + 1, size + 4, size + 260, size + 268, 0x7FFFFFFF, 0x80000000, 0xFFFFFFFF} {
			inputs = append(inputs, input{fmt.Sprintf("the size at %d set to %d", at, value), testkit.SetU32(whole, at, uint32(value))})
		}
	}
	random := rand.New(rand.NewPCG(7, 2026))
	for i := range 1200 {
		inputs = append(inputs, input{fmt.Sprintf("seeded change %d", i), altered(random, whole)})
	}
	for _, in := range inputs {
		c.model(in.name, in.data)
	}
	// A seeded change may write a number of several invalid bytes into a path, or move the start of a path onto
	// bytes that are not text.
	if c.leftOut > len(inputs)/50 {
		t.Errorf("%d readings of %d altered models were left out", c.leftOut, len(inputs))
	}
	c.compared(binaryRefusals...)
	if c.read < len(inputs)/4 {
		t.Errorf("only %d readings gave paths", c.read)
	}
	c.summary(len(inputs))
}

// tokensOf splits a text whose tokens are apart by white space, but for a comma that follows its token, into
// its tokens.
func tokensOf(source string) []string {
	var tokens []string
	for _, field := range strings.Fields(source) {
		if word, endsAStatement := strings.CutSuffix(field, ","); endsAStatement && word != "" {
			tokens = append(tokens, word, ",")
			continue
		}
		tokens = append(tokens, field)
	}
	return tokens
}

// blocksOf splits tokens into the blocks that are in no other block.
func blocksOf(tokens []string) [][]string {
	var blocks [][]string
	depth, start := 0, 0
	for i, token := range tokens {
		switch token {
		case "{":
			depth++
		case "}":
			depth--
			if depth == 0 {
				blocks = append(blocks, tokens[start:i+1])
				start = i + 1
			}
		}
	}
	return blocks
}

// blocksOfTheTests are the blocks of the texts the tests read: the knight's, and blocks with a path in more than
// one place.
func blocksOfTheTests(t *testing.T) [][]string {
	t.Helper()
	_, knightBlocks, _ := strings.Cut(knight, "\n") // without its comment
	sources := []string{knightBlocks, "Textures 1 {\n\tBitmap {\n\t\tImage \"a.blp\",\n\t}\n}\n", "Model \"A\" {\n\tNumGeosets 0,\n}\n"}
	for _, text := range emitterTexts() {
		sources = append(sources, text.blocks)
	}
	var blocks [][]string
	for _, source := range sources {
		blocks = append(blocks, blocksOf(tokensOf(source))...)
	}
	if len(blocks) < 20 {
		t.Fatalf("found only %d blocks in the texts of the tests", len(blocks))
	}
	return blocks
}

// isWord reports whether a token is a word, which needs white space between itself and a word beside it.
func isWord(token string) bool {
	return !strings.ContainsAny(token, `{},"`)
}

// seededGap is what stands between two tokens: white space of every kind ASCII has and comments. Where the tokens
// allow it, it is at times nothing, or a comment that starts right after the token. Once in forty times it is
// one of those two where the tokens do not allow it, which makes one word of two or of a word and a comment.
func seededGap(random *rand.Rand, afterWord, beforeWord bool) string {
	gaps := []string{" ", " ", "\t", "\n", "\r\n", "  ", "\n\t", "\r\n\t\t", "\v", "\f", "\r", " // a note\n", "\t//\r\n",
		" // { \"quoted\" }, \n", "\n// Path \"no.mdx\",\n", " //// \r\n", "\n//\n//\n"}
	comments := []string{"//x\n", "// }\r\n", "//\n"}
	anywhere := random.IntN(40) == 0
	switch {
	case (!afterWord || anywhere) && random.IntN(5) == 0:
		return comments[random.IntN(len(comments))]
	case (!afterWord || !beforeWord || anywhere) && random.IntN(3) == 0:
		return ""
	}
	return gaps[random.IntN(len(gaps))]
}

// seededText writes the tokens with a seeded gap after each.
func seededText(random *rand.Rand, tokens []string) string {
	var out strings.Builder
	for i, token := range tokens {
		out.WriteString(token)
		last := i == len(tokens)-1
		out.WriteString(seededGap(random, isWord(token), !last && isWord(tokens[i+1])))
	}
	return out.String()
}

// seededTokens are some of the blocks in a seeded order, at times with one block inside another, mostly after a
// header, and at times with one token taken out, which leaves most texts broken.
func seededTokens(random *rand.Rand, blocks [][]string) []string {
	blocks = slices.Clone(blocks)
	random.Shuffle(len(blocks), func(i, j int) { blocks[i], blocks[j] = blocks[j], blocks[i] })
	var tokens []string
	for _, block := range blocks[:random.IntN(len(blocks)+1)] {
		if len(tokens) > 0 && random.IntN(6) == 0 {
			// Inside the block before it: in front of that block's closing brace.
			tokens = slices.Concat(tokens[:len(tokens)-1], block, []string{"}"})
			continue
		}
		tokens = append(tokens, block...)
	}
	if random.IntN(4) > 0 {
		tokens = slices.Concat(tokensOf(header), tokens)
	}
	if len(tokens) > 0 && random.IntN(5) == 0 {
		at := random.IntN(len(tokens))
		tokens = slices.Delete(tokens, at, at+1)
	}
	return tokens
}

func TestOracleOnSeededTextModels(t *testing.T) {
	c := newComparison(t)
	random := rand.New(rand.NewPCG(31, 2026))
	blocks := blocksOfTheTests(t)
	const models = 400
	for i := range models {
		source := seededText(random, seededTokens(random, blocks))
		c.model(fmt.Sprintf("seeded text model %d: %q", i, source), []byte(source))
	}
	c.compared("a } has no matching {", "block is never closed", "it has no Version or Model block")
	if c.read < models {
		t.Errorf("only %d readings gave paths", c.read)
	}
	c.leavesOut(0, "none")
	c.summary(models)
}

// modelFilesUnder returns the .mdx and .mdl files below a folder, which need not exist. It passes over folders
// whose name starts with a dot: they hold tools and what a test run left, not the files of a project.
func modelFilesUnder(t *testing.T, folder string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(folder, func(path string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil && path == folder && errors.Is(err, fs.ErrNotExist):
			return fs.SkipAll
		case err != nil:
			return err
		case entry.IsDir() && path != folder && strings.HasPrefix(entry.Name(), "."):
			return fs.SkipDir
		}
		extension := strings.ToLower(filepath.Ext(path))
		if !entry.IsDir() && (extension == ".mdx" || extension == ".mdl") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// TestOracleOnTheModelFilesOnDisk compares every model file of the template and of the two libraries, where they
// are checked out beside this repository. There may be none.
func TestOracleOnTheModelFilesOnDisk(t *testing.T) {
	c := newComparison(t)
	root := testkit.RepoRoot(t)
	files := 0
	for _, folder := range []string{
		filepath.Join(root, "template", "assets"),
		filepath.Join(filepath.Dir(root), "moonwell-wrappers"),
		filepath.Join(filepath.Dir(root), "moonwell-systems"),
	} {
		for _, file := range modelFilesUnder(t, folder) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			c.model(file, bytes.Clone(data))
			files++
		}
	}
	c.summary(files)
}
