package model_test

import (
	"encoding/binary"
	"errors"
	"fmt"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/war3/model"
)

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

const textWithOtherLetters = "// M\xC3\xA5ne \xE6\x9C\x88\r\nVersion {\r\n\tFormatVersion 800,\r\n}\r\n" +
	"Model \"M\xC3\xA5ne\" {\r\n}\r\nTextures 2 {\r\n\tBitmap { // \xF0\x9F\x8C\x99\r\n" +
	"\t\tImage \"Textures\\M\xC3\xA5ne\xE6\x9C\x88.blp\",\r\n\t}\r\n" +
	"\tBitm\xC3\xA5p {\r\n\t\tImage \"b.blp\",\r\n\t}\r\n}\r\n" +
	"ParticleEmitter \"\xE6\x9C\x88\" {\r\n\tEmitterUsesTGA,\r\n" +
	"\tParticle {\r\n\t\tPath \"\xF0\x9F\x8C\x99.blp\",\r\n\t}\r\n}\r\n"

func sizeOffsets(t *testing.T, data []byte) []int {
	t.Helper()
	u32 := func(offset int) int {
		return int(data[offset]) | int(data[offset+1])<<8 | int(data[offset+2])<<16 | int(data[offset+3])<<24
	}
	var offsets []int
	for offset := 4; offset < len(data); {
		tag, size := string(data[offset:offset+4]), u32(offset+4)
		offsets = append(offsets, offset+4)
		if tag == "PREM" || tag == "ATCH" || tag == "CORN" {
			for record := offset + 8; record < offset+8+size; record += u32(record) {
				offsets = append(offsets, record, record+4)
			}
		}
		offset += 8 + size
	}
	if len(offsets) < 15 {
		t.Fatalf("found only %d sizes in the model", len(offsets))
	}
	return offsets
}

func sizesNear(size uint32) []uint32 {
	return append(testkit.EdgeNumbers(), 3, 4, 95, 96, 99, 100, 103, 104, size-261, size-260, size-5, size-4, size-1,
		size+1, size+4, size+260, size+268)
}

const mutationSeed = 800

type outcomeCounts struct{ accepted, rejected int }

func (c *outcomeCounts) record(t *testing.T, what string, data []byte) {
	t.Helper()
	for name, read := range map[string]func() ([]model.Path, error){
		"Paths":   func() ([]model.Path, error) { return model.ReadPaths(data, modelFile) },
		"ReadMDX": func() ([]model.Path, error) { return model.ReadMDX(data, modelFile) },
		"ReadMDL": func() ([]model.Path, error) { return model.ReadMDL(string(data), modelFile) },
	} {
		var paths []model.Path
		var err error
		if value := testkit.PanicValue(func() { paths, err = read() }); value != nil {
			t.Fatalf("%s: %s panics: %v", what, name, value)
		}
		var diagErr *diag.Error
		switch {
		case err == nil:
			c.accepted++
		case paths == nil && errors.As(err, &diagErr) && diagErr.File == modelFile:
			c.rejected++
		default:
			t.Fatalf("%s: %s = %+v, %v; want paths, or an error of %s", what, name, paths, err, modelFile)
		}
	}
}

func TestADamagedModelIsReadOrRefusedByNameAndNeverPanics(t *testing.T) {
	var damaged outcomeCounts
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
			damaged.record(t, fmt.Sprintf("%s cut at %d bytes", whole.name, length), whole.data[:length:length])
		}
		for index := range uint64(1500) {
			what := fmt.Sprintf("%s, change %d of seed %d", whole.name, index, mutationSeed)
			damaged.record(t, what+" to its bytes", testkit.MutateBytes(whole.data, mutationSeed, index))
			if whole.text {
				text := testkit.MutateText(string(whole.data), mutationSeed, index)
				damaged.record(t, what+" to its text", []byte(text))
			}
		}
	}
	whole := modelWithEveryChunk()
	for _, offset := range sizeOffsets(t, whole) {
		for _, size := range sizesNear(binary.LittleEndian.Uint32(whole[offset:])) {
			damaged.record(t, fmt.Sprintf("the size at %d set to %d", offset, size), testkit.SetU32(whole, offset, size))
		}
	}
	if damaged.accepted == 0 || damaged.rejected == 0 {
		t.Errorf("%d readings of damaged models gave paths and %d were refused; want some of each", damaged.accepted,
			damaged.rejected)
	}
}
