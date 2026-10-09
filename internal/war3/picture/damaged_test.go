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

const mutationSeed = 256

const pictureStartLength = 2048

type outcomeCounts struct{ accepted, rejected int }

func (c *outcomeCounts) record(t *testing.T, what string, data []byte, displayPath string) {
	t.Helper()
	var read *picture.Picture
	var err error
	if value := testkit.PanicValue(func() { read, err = picture.Read(data, displayPath) }); value != nil {
		t.Fatalf("%s: Read panics: %v", what, value)
	}
	var diagErr *diag.Error
	switch {
	case err == nil && read != nil:
		c.accepted++
	case err != nil && read == nil && errors.As(err, &diagErr) && diagErr.File == displayPath:
		c.rejected++
	default:
		t.Fatalf("%s: Read = %+v, %v; want a picture, or an error of %s", what, read, err, displayPath)
	}
}

func truncationLengths(whole int) []int {
	var lengths []int
	for length := range min(pictureStartLength, whole) {
		lengths = append(lengths, length)
	}
	for length := pictureStartLength; length < whole-1; length += 97 {
		lengths = append(lengths, length)
	}
	return append(lengths, whole-1)
}

func TestADamagedPictureIsReadOrRefusedByNameAndNeverPanics(t *testing.T) {
	t.Parallel()
	source := testkit.NewPixels(256)
	var damaged outcomeCounts
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
		for _, length := range truncationLengths(len(whole.data)) {
			what := fmt.Sprintf("%s cut at %d bytes", whole.name, length)
			damaged.record(t, what, whole.data[:length:length], whole.displayPath)
		}
		start, rest := whole.data[:pictureStartLength], whole.data[pictureStartLength:]
		for index := range uint64(150) {
			what := fmt.Sprintf("%s, change %d of seed %d", whole.name, index, mutationSeed)
			anywhere := testkit.MutateBytes(whole.data, mutationSeed, index)
			damaged.record(t, what+" to the whole file", anywhere, whole.displayPath)
			inStart := slices.Concat(testkit.MutateBytes(start, mutationSeed, index), rest)
			damaged.record(t, what+" to its first 2048 bytes", inStart, whole.displayPath)
		}
	}
	if damaged.accepted == 0 || damaged.rejected == 0 {
		t.Errorf("%d damaged pictures were read and %d refused; want some of each", damaged.accepted, damaged.rejected)
	}
}
