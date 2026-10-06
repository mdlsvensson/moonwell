package model_test

import (
	"encoding/binary"
	"errors"
	"fmt"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/war3/model"
)

// modelWithEveryChunk is a binary model with every chunk that holds paths, chunks that hold none, and a path that
// is not ASCII.
func modelWithEveryChunk() []byte {
	return testkit.MDX(
		testkit.Chunk("VERS", testkit.U32(800)),
		testkit.Chunk("TEXS", testkit.Concat(
			testkit.Texture(`Textures\Knight.blp`, 0), testkit.Texture("", 1), testkit.Texture("Textures/M\xC3\xA5ne.blp", 0),
		)),
		testkit.Chunk("ZZZZ", make([]byte, 13)),
		testkit.Chunk("PREM", testkit.Concat(
			testkit.Emitter(`Abilities\Spells\Human\Heal.mdx`, testkit.EmitterUsesMDL),
			testkit.Emitter(`Textures\Spark.blp`, testkit.EmitterUsesTGA),
			testkit.Emitter("", 0),
		)),
		testkit.Chunk("ATCH", testkit.Concat(testkit.Attachment(`Models\Sword.mdx`), testkit.Attachment(""))),
		testkit.Chunk("CORN", testkit.Popcorn(`Effects\Fire.pkfx`)),
		testkit.Chunk("FAFX", testkit.FaceEffect("Head", `FaceFX\Knight.facefx`)),
	)
}

// textWithOtherLetters is a text model with CRLF line ends, comments, and letters outside ASCII in a comment, a
// block name, a string and a path.
const textWithOtherLetters = "// M\xC3\xA5ne \xE6\x9C\x88\r\nVersion {\r\n\tFormatVersion 800,\r\n}\r\n" +
	"Model \"M\xC3\xA5ne\" {\r\n}\r\nTextures 2 {\r\n\tBitmap { // \xF0\x9F\x8C\x99\r\n" +
	"\t\tImage \"Textures\\M\xC3\xA5ne\xE6\x9C\x88.blp\",\r\n\t}\r\n" +
	"\tBitm\xC3\xA5p {\r\n\t\tImage \"b.blp\",\r\n\t}\r\n}\r\n" +
	"ParticleEmitter \"\xE6\x9C\x88\" {\r\n\tEmitterUsesTGA,\r\n" +
	"\tParticle {\r\n\t\tPath \"\xF0\x9F\x8C\x99.blp\",\r\n\t}\r\n}\r\n"

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

// sizesNear are the numbers a size is set to: the edges of what a size may be, the sizes of the fixed parts of
// a record and the ones beside them, and the size itself moved by a little and by a path's length.
func sizesNear(size uint32) []uint32 {
	return append(testkit.EdgeNumbers(), 3, 4, 95, 96, 99, 100, 103, 104, size-261, size-260, size-5, size-4, size-1,
		size+1, size+4, size+260, size+268)
}

// damageSeed is the seed of the changes that TestADamagedModelIsReadOrRefusedByNameAndNeverPanics makes. A failure
// names the model and the index of the change: testkit.ChangedBytes, and testkit.Changed for a change of the
// text, make the same model of the three again.
const damageSeed = 800

// tally counts the readings of damaged models that gave paths and the ones that were refused.
type tally struct{ read, refused int }

// readOrRefused gives the bytes to Paths, to ReadMDX and to ReadMDL, whichever format they are. It stops the test
// when a reader panics, when it returns paths with an error, and when an error is not a *diag.Error with the name
// the test gave.
func (c *tally) readOrRefused(t *testing.T, what string, data []byte) {
	t.Helper()
	for name, read := range map[string]func() ([]model.Path, error){
		"Paths":   func() ([]model.Path, error) { return model.Paths(data, modelFile) },
		"ReadMDX": func() ([]model.Path, error) { return model.ReadMDX(data, modelFile) },
		"ReadMDL": func() ([]model.Path, error) { return model.ReadMDL(string(data), modelFile) },
	} {
		var paths []model.Path
		var err error
		if value := testkit.Panic(func() { paths, err = read() }); value != nil {
			t.Fatalf("%s: %s panics: %v", what, name, value)
		}
		var failure *diag.Error
		switch {
		case err == nil:
			c.read++
		case paths == nil && errors.As(err, &failure) && failure.File == modelFile:
			c.refused++
		default:
			t.Fatalf("%s: %s = %+v, %v; want paths, or an error of %s", what, name, paths, err, modelFile)
		}
	}
}

// TestADamagedModelIsReadOrRefusedByNameAndNeverPanics gives the three readers two binary and two text models,
// each cut at every length and after each of 1500 seeded changes of its bytes; a text model also after each of
// 1500 seeded changes of its lines, quotes and white space; and the binary model with every chunk with each of
// its sizes set to the numbers of sizesNear.
func TestADamagedModelIsReadOrRefusedByNameAndNeverPanics(t *testing.T) {
	var damaged tally
	for _, whole := range []struct {
		name string
		data []byte
		text bool
	}{
		{"the model with every chunk", modelWithEveryChunk(), false},
		{"the model with one texture", testkit.MDX(testkit.Chunk("TEXS", testkit.Texture(`Textures\A.blp`, 0))), false},
		{"the knight", []byte(knight), true},
		{"the text with other letters", []byte(textWithOtherLetters), true},
	} {
		for length := range len(whole.data) + 1 {
			damaged.readOrRefused(t, fmt.Sprintf("%s cut at %d bytes", whole.name, length), whole.data[:length:length])
		}
		for index := range uint64(1500) {
			what := fmt.Sprintf("%s, change %d of seed %d", whole.name, index, damageSeed)
			damaged.readOrRefused(t, what+" to its bytes", testkit.ChangedBytes(whole.data, damageSeed, index))
			if whole.text {
				text := testkit.Changed(string(whole.data), damageSeed, index)
				damaged.readOrRefused(t, what+" to its text", []byte(text))
			}
		}
	}
	// A size is no number to trust: every size of the model with every chunk, set to each number at an edge and
	// near itself.
	whole := modelWithEveryChunk()
	for _, at := range sizeOffsets(t, whole) {
		for _, size := range sizesNear(binary.LittleEndian.Uint32(whole[at:])) {
			damaged.readOrRefused(t, fmt.Sprintf("the size at %d set to %d", at, size), testkit.SetU32(whole, at, size))
		}
	}
	// The floor is against a test that passes because it gave the readers nothing.
	if damaged.read == 0 || damaged.refused == 0 {
		t.Errorf("%d readings of damaged models gave paths and %d were refused; want some of each", damaged.read,
			damaged.refused)
	}
}
