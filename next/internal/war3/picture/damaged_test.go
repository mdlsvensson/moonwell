package picture_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/war3/picture"
)

// damageSeed is the seed of the changes that TestADamagedPictureIsReadOrRefusedByNameAndNeverPanics makes. A
// failure names the picture and the index of the change: testkit.ChangedBytes makes the same bytes of the three
// again.
const damageSeed = 256

// startOfAPicture is the bytes at the start of a picture that every length is cut in and that half of the changes
// are made in: the header of each format lies in them, the palette of a BLP, and the first packets or the first
// compressed bytes of the pixels.
const startOfAPicture = 2048

// tally counts the damaged pictures that read and the ones that were refused.
type tally struct{ read, refused int }

// readOrRefused gives Read the bytes under the name. It stops the test when Read panics, when it returns neither
// a picture nor an error or both, and when the error is not a *diag.Error with the name the test gave.
func (c *tally) readOrRefused(t *testing.T, what string, data []byte, file string) {
	t.Helper()
	var read *picture.Picture
	var err error
	if value := testkit.Panic(func() { read, err = picture.Read(data, file) }); value != nil {
		t.Fatalf("%s: Read panics: %v", what, value)
	}
	var failure *diag.Error
	switch {
	case err == nil && read != nil:
		c.read++
	case err != nil && read == nil && errors.As(err, &failure) && failure.File == file:
		c.refused++
	default:
		t.Fatalf("%s: Read = %+v, %v; want a picture, or an error of %s", what, read, err, file)
	}
}

// lengthsToCut are every length below startOfAPicture and from there every 97th up to the whole, and one byte
// less than the whole. The bytes after the start are pixels, packets of pixels or compressed pixels, one like
// the other: a step that is a prime and no multiple of a pixel's bytes cuts them at every place inside a pixel
// and inside a packet.
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

// TestADamagedPictureIsReadOrRefusedByNameAndNeverPanics gives Read a picture of each format and way of storing
// it, cut at the lengths of lengthsToCut and after seeded changes of its bytes: 150 anywhere in the file, and
// 150 in its start, 1500 in all. A change of a PNG fails a check value, so the PNG reader meets its damage in
// the cuts. No cut picture reads: the pixels of each, and the first mipmap of the BLP, end where the file ends.
func TestADamagedPictureIsReadOrRefusedByNameAndNeverPanics(t *testing.T) {
	t.Parallel()
	source := testkit.NewPixels(256)
	for _, whole := range []struct {
		name, file string
		data       []byte
		// What Read makes of the cuts and of the changes. The bytes of a TGA and of a BLP are the test kit's
		// own, so their numbers are exact: one that differs is a reading that changed. The bytes of a PNG are a
		// compressor's, which another version of Go may write otherwise: of a PNG no input reads, a change
		// fails a check value, and how many there are is not written down.
		cut, changed tally
		compressed   bool
	}{
		{name: "a plain TGA", file: "preview.tga", data: testkit.TGA(source, testkit.TGAOptions{ID: 7}),
			cut: tally{0, 4731}, changed: tally{196, 104}},
		{name: "a run-length TGA", file: "preview.tga",
			data: testkit.TGA(source, testkit.TGAOptions{RLE: true, Depth: 24, FromTop: true, ID: 7}),
			cut:  tally{0, 3055}, changed: tally{98, 202}},
		{name: "a PNG", file: "preview.png", data: testkit.PNG(source, "rgba"), compressed: true},
		{name: "an interlaced PNG", file: "preview.png", data: testkit.PNG(source, "interlaced"), compressed: true},
		{name: "a BLP", file: "preview.blp", data: testkit.BLP(256, 1), cut: tally{0, 2716}, changed: tally{194, 106}},
	} {
		var cut, changed tally
		for _, length := range lengthsToCut(len(whole.data)) {
			cut.readOrRefused(t, fmt.Sprintf("%s cut at %d bytes", whole.name, length), whole.data[:length:length], whole.file)
		}
		start, rest := whole.data[:startOfAPicture], whole.data[startOfAPicture:]
		for index := range uint64(150) {
			what := fmt.Sprintf("%s, change %d of seed %d", whole.name, index, damageSeed)
			anywhere := testkit.ChangedBytes(whole.data, damageSeed, index)
			changed.readOrRefused(t, what+" to the whole file", anywhere, whole.file)
			inStart := slices.Concat(testkit.ChangedBytes(start, damageSeed, index), rest)
			changed.readOrRefused(t, what+" to its first 2048 bytes", inStart, whole.file)
		}
		switch {
		case whole.compressed && (cut.read != 0 || changed.read != 0 || cut.refused < 2000 || changed.refused != 300):
			t.Errorf("%s: cut: %+v, changed: %+v; want none read", whole.name, cut, changed)
		case !whole.compressed && (cut != whole.cut || changed != whole.changed):
			t.Errorf("%s: cut: %+v, changed: %+v; want %+v and %+v", whole.name, cut, changed, whole.cut, whole.changed)
		}
	}
}
