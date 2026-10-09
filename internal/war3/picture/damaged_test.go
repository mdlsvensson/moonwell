package picture_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/war3/picture"
)

const damageSeed = 256

const startOfAPicture = 2048

type tally struct{ read, refused int }

func (c *tally) readOrRefused(t *testing.T, what string, data []byte, displayPath string) {
	t.Helper()
	var read *picture.Picture
	var err error
	if value := testkit.PanicValue(func() { read, err = picture.Read(data, displayPath) }); value != nil {
		t.Fatalf("%s: Read panics: %v", what, value)
	}
	var failure *diag.Error
	switch {
	case err == nil && read != nil:
		c.read++
	case err != nil && read == nil && errors.As(err, &failure) && failure.File == displayPath:
		c.refused++
	default:
		t.Fatalf("%s: Read = %+v, %v; want a picture, or an error of %s", what, read, err, displayPath)
	}
}

func lengthsToCut(whole int) []int {
	var lengths []int
	for length := range min(startOfAPicture, whole) {
		lengths = append(lengths, length)
	}
	for length := startOfAPicture; length < whole-1; length += 97 {
		lengths = append(lengths, length)
	}
	return append(lengths, whole-1)
}

func TestADamagedPictureIsReadOrRefusedByNameAndNeverPanics(t *testing.T) {
	t.Parallel()
	source := testkit.NewPixels(256)
	var damaged tally
	for _, whole := range []struct {
		name, displayPath string
		data              []byte
	}{
		{"a plain TGA", "preview.tga", testkit.TGA(source, testkit.TGAOptions{ID: 7})},
		{"a run-length TGA", "preview.tga",
			testkit.TGA(source, testkit.TGAOptions{RLE: true, Depth: 24, FromTop: true, ID: 7})},
		{"a PNG", "preview.png", testkit.PNG(source, "rgba")},
		{"an interlaced PNG", "preview.png", testkit.PNG(source, "interlaced")},
		{"a BLP", "preview.blp", testkit.BLP(256, 1)},
	} {
		for _, length := range lengthsToCut(len(whole.data)) {
			what := fmt.Sprintf("%s cut at %d bytes", whole.name, length)
			damaged.readOrRefused(t, what, whole.data[:length:length], whole.displayPath)
		}
		start, rest := whole.data[:startOfAPicture], whole.data[startOfAPicture:]
		for index := range uint64(150) {
			what := fmt.Sprintf("%s, change %d of seed %d", whole.name, index, damageSeed)
			anywhere := testkit.MutateBytes(whole.data, damageSeed, index)
			damaged.readOrRefused(t, what+" to the whole file", anywhere, whole.displayPath)
			inStart := slices.Concat(testkit.MutateBytes(start, damageSeed, index), rest)
			damaged.readOrRefused(t, what+" to its first 2048 bytes", inStart, whole.displayPath)
		}
	}
	if damaged.read == 0 || damaged.refused == 0 {
		t.Errorf("%d damaged pictures were read and %d refused; want some of each", damaged.read, damaged.refused)
	}
}
