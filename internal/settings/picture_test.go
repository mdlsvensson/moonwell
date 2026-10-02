package settings_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/settings"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

// refusedPicture checks that the picture is refused with message, naming file and giving a hint.
func refusedPicture(t *testing.T, data []byte, file, message string) *diag.Error {
	t.Helper()
	_, err := settings.ReadPicture(data, file)
	e := asError(t, err, message)
	if !strings.Contains(e.Msg, message) || e.File != file || e.Hint == "" {
		t.Errorf("error = %+v, want %q", e, message)
	}
	return e
}

func opaque() *byte {
	alpha := byte(255)
	return &alpha
}

func TestEveryAcceptedTGAIsRewrittenAsTheOneLayoutTheGameWasSeenToAccept(t *testing.T) {
	for _, size := range []int{256, 512} {
		picture := testkit.NewPixels(size)
		// Plain, 32 bits, rows from the bottom, opaque: what the probe's map 6 held.
		expected := testkit.TGA(picture, testkit.TGAOptions{Alpha: opaque()})
		if expected[2] != 2 || expected[16] != 32 || expected[17] != 8 {
			t.Fatalf("the expected TGA has header % X", expected[:18])
		}
		for _, rle := range []bool{false, true} {
			for _, depth := range []int{24, 32} {
				for _, fromTop := range []bool{false, true} {
					for _, id := range []int{0, 5} {
						source := testkit.TGA(picture, testkit.TGAOptions{RLE: rle, Depth: depth, FromTop: fromTop, ID: id})
						result, err := settings.ReadPicture(source, "art/Preview.TGA")
						if err != nil || result.Extension != "tga" || !bytes.Equal(result.Bytes, expected) {
							t.Errorf("size %d, rle %v, %d bits, from top %v, id %d: %v", size, rle, depth, fromTop, id, err)
						}
					}
				}
			}
		}
	}
}

func TestARewrittenTGAStartsWithThePicturesBottomRowAndHasNothingAfterItsPixels(t *testing.T) {
	picture := testkit.NewPixels(256)
	result, err := settings.ReadPicture(testkit.TGA(picture, testkit.TGAOptions{FromTop: true}), "preview.tga")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Bytes) != 18+256*256*4 {
		t.Errorf("the rewritten TGA has %d bytes", len(result.Bytes))
	}
	// Blue, green, red, alpha of the bottom-left pixel, whose red is its row number.
	if got := result.Bytes[18:22]; !bytes.Equal(got, []byte{200, 40, 255, 255}) {
		t.Errorf("the first pixel is %v", got)
	}
	// Bytes after the pixels, such as a TGA 2.0 footer, are left out.
	footer := append(testkit.TGA(picture, testkit.TGAOptions{}), "TRUEVISION-XFILE.\x00"...)
	again, err := settings.ReadPicture(footer, "preview.tga")
	if err != nil || !bytes.Equal(again.Bytes, result.Bytes) {
		t.Errorf("a TGA with a footer: %v", err)
	}
}

func TestATGATheReaderDoesNotKnowIsRefusedByWhatItIs(t *testing.T) {
	const file = "preview.tga"
	plain := func() []byte { return testkit.TGA(testkit.NewPixels(256), testkit.TGAOptions{}) }
	edited := func(change func([]byte)) []byte {
		data := plain()
		change(data)
		return data
	}
	early := "is cut short: its pixel data ends early."
	refusedPicture(t, make([]byte, 17), file, "is cut short: a TGA header has 18 bytes.")
	refusedPicture(t, edited(func(b []byte) { b[1] = 1 }), file, "is a TGA with a colour map.")
	refusedPicture(t, edited(func(b []byte) { b[2] = 3 }), file, "is a TGA of image type 3, not a true-colour picture.")
	refusedPicture(t, edited(func(b []byte) { b[16] = 16 }), file, "is a TGA with 16 bits a pixel, not 24 or 32.")
	refusedPicture(t, edited(func(b []byte) { b[17] |= 0x10 }), file, "is a TGA whose rows run from right to left.")
	refusedPicture(t, plain()[:len(plain())-1], file, early)
	withID := testkit.TGA(testkit.NewPixels(256), testkit.TGAOptions{ID: 9})
	refusedPicture(t, withID[:len(withID)-9], file, early)
	rle := testkit.TGA(testkit.NewPixels(256), testkit.TGAOptions{RLE: true})
	refusedPicture(t, rle[:len(rle)-1], file, early)
	refusedPicture(t, rle[:19], file, early)
	refusedPicture(t, rle[:18], file, early)

	// 511 runs of 128 pixels and one of 127 leave room for one pixel; the last packet holds two.
	run := func(count int) []byte { return []byte{byte(0x80 | (count - 1)), 1, 2, 3, 255} }
	packets := slices.Concat(bytes.Repeat(run(128), 511), run(127))
	header := slices.Clone(rle[:18])
	for _, last := range [][]byte{run(2), {1, 1, 2, 3, 255, 1, 2, 3, 255}} {
		refusedPicture(t, slices.Concat(header, packets, last), file, "a run of pixels overruns the picture.")
	}
	result, err := settings.ReadPicture(slices.Concat(header, packets, run(1)), file)
	if err != nil || result.Bytes[18+2] != 3 {
		t.Errorf("a picture of runs: %v", err)
	}
	// A last run whose pixel is cut off would otherwise be filled in with zeros.
	refusedPicture(t, slices.Concat(header, packets, []byte{0x80, 1, 2}), file, early)
}

func TestOnlyTheTwoSizesSeenToWorkAreAccepted(t *testing.T) {
	sized := func(width, height uint16) []byte {
		data := testkit.TGA(testkit.NewPixels(256), testkit.TGAOptions{})
		binary.LittleEndian.PutUint16(data[12:], width)
		binary.LittleEndian.PutUint16(data[14:], height)
		return data
	}
	e := refusedPicture(t, sized(128, 128), "preview.tga", "is 128x128 pixels; it must be 256x256 or 512x512.")
	if !strings.Contains(e.Hint, "256x256") {
		t.Errorf("hint = %q", e.Hint)
	}
	refusedPicture(t, sized(512, 256), "preview.tga", "is 512x256 pixels")
	refusedPicture(t, sized(256, 512), "preview.tga", "is 256x512 pixels")
	refusedPicture(t, testkit.BLP(1024, 1), "preview.blp", "is 1024x1024 pixels")
	wide := testkit.BLP(256, 1)
	binary.LittleEndian.PutUint32(wide[12:], 512)
	refusedPicture(t, wide, "preview.blp", "is 512x256 pixels")
}

func TestABLP1WithJPEGOrPaletteContentIsUsedAsItIs(t *testing.T) {
	for _, size := range []int{256, 512} {
		for _, content := range []uint32{0, 1} {
			data := testkit.BLP(size, content)
			result, err := settings.ReadPicture(data, "Preview.BLP")
			if err != nil || result.Extension != "blp" || !bytes.Equal(result.Bytes, data) {
				t.Errorf("%s: %v", fmt.Sprintf("size %d, content %d", size, content), err)
			}
		}
	}
}

func TestABLPTheGameCouldNotReadIsRefused(t *testing.T) {
	const file = "preview.blp"
	edited := func(change func([]byte)) []byte {
		data := testkit.BLP(256, 1)
		change(data)
		return data
	}
	put := func(offset int, value uint32) func([]byte) {
		return func(b []byte) { binary.LittleEndian.PutUint32(b[offset:], value) }
	}
	other := refusedPicture(t, edited(func(b []byte) { b[3] = 0x32 }), file, "is a BLP2 file, the World of Warcraft format.")
	if !strings.Contains(other.Hint, "BLP1") {
		t.Errorf("hint = %q", other.Hint)
	}
	// TGA bytes under a .blp name closed the game in the probe.
	refusedPicture(t, testkit.TGA(testkit.NewPixels(256), testkit.TGAOptions{}), file, "is not a BLP file: it does not start with BLP1.")
	refusedPicture(t, nil, file, "is not a BLP file")
	refusedPicture(t, testkit.BLP(256, 1)[:155], file, "is cut short: a BLP header has 156 bytes.")
	refusedPicture(t, edited(put(4, 2)), file, "has the unknown BLP content type 2.")
	refusedPicture(t, edited(put(28, 155)), file, "its first mipmap lies outside the file.")
	refusedPicture(t, edited(put(92, 0)), file, "its first mipmap lies outside the file.")
	whole := testkit.BLP(256, 1)
	refusedPicture(t, whole[:len(whole)-1], file, "its first mipmap lies outside the file.")
}

func TestAPNGOfAnyKindGoesInAsTheTGAThatATGAOfTheSamePictureGives(t *testing.T) {
	kinds := map[string][]string{
		"colour": {"rgba", "rgb", "rgba16", "rgb16", "interlaced"},
		"grey":   {"grey", "grey16", "palette"},
	}
	for _, size := range []int{256, 512} {
		for name, picture := range map[string]testkit.Pixels{"colour": testkit.NewPixels(size), "grey": testkit.GreyPixels(size)} {
			expected, err := settings.ReadPicture(testkit.TGA(picture, testkit.TGAOptions{}), "preview.tga")
			if err != nil {
				t.Fatal(err)
			}
			for _, kind := range kinds[name] {
				result, err := settings.ReadPicture(testkit.PNG(picture, kind), "art/Preview.PNG")
				if err != nil || result.Extension != "tga" || !bytes.Equal(result.Bytes, expected.Bytes) {
					t.Errorf("size %d, %s: %v", size, kind, err)
				}
			}
		}
	}
}

func TestAPNGsTransparencyIsDroppedAndItsStoredColourKept(t *testing.T) {
	// The picture's alpha is 7 everywhere, and then 0: a reader that blends with a background, or that reads the
	// colour through the alpha, changes every pixel, and loses a fully transparent one altogether.
	for _, alpha := range []byte{7, 0} {
		picture := testkit.NewPixels(256)
		for at := 3; at < len(picture.RGBA); at += 4 {
			picture.RGBA[at] = alpha
		}
		expected := testkit.TGA(picture, testkit.TGAOptions{Alpha: opaque()})
		for _, kind := range []string{"rgba", "rgba16", "interlaced"} {
			result, err := settings.ReadPicture(testkit.PNG(picture, kind), "preview.png")
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(result.Bytes, expected) {
				t.Errorf("alpha %d, %s: the picture is not the opaque TGA of its stored colours", alpha, kind)
			}
			// Blue, green, red, alpha of the bottom-left pixel, whose red is its row number.
			if got := result.Bytes[18:22]; !bytes.Equal(got, []byte{200, 40, 255, 255}) {
				t.Errorf("alpha %d, %s: the first pixel is %v", alpha, kind, got)
			}
		}
	}
}

func TestAPNGsSizeIsJudgedBeforeItsPixelsAreRead(t *testing.T) {
	const file = "preview.png"
	// These files hold a header and no pixel: only a reader that decodes first would call them damaged.
	e := refusedPicture(t, testkit.PNGHeader(128, 128, false), file, "is 128x128 pixels; it must be 256x256 or 512x512.")
	if !strings.Contains(e.Hint, "256x256") {
		t.Errorf("hint = %q", e.Hint)
	}
	refusedPicture(t, testkit.PNGHeader(512, 256, false), file, "is 512x256 pixels")
	refusedPicture(t, testkit.PNGHeader(256, 512, true), file, "is 256x512 pixels")
	refusedPicture(t, testkit.PNGHeader(60000, 60000, false), file, "is 60000x60000 pixels")
	refusedPicture(t, testkit.PNG(testkit.NewPixels(64), "rgb"), file, "is 64x64 pixels")
}

func TestAFileThatIsNotAReadablePNGIsRefused(t *testing.T) {
	const file = "preview.png"
	// A TGA under a .png name is read as a PNG and refused, never passed through.
	other := refusedPicture(t, testkit.TGA(testkit.NewPixels(256), testkit.TGAOptions{}), file,
		"is not a PNG file: it does not start with a PNG signature.")
	if !strings.Contains(other.Hint, "PNG") {
		t.Errorf("hint = %q", other.Hint)
	}
	refusedPicture(t, nil, file, "is not a PNG file")
	whole := testkit.PNG(testkit.NewPixels(256), "rgb")
	const unreadable = "is a PNG that could not be read: "
	damaged := []struct {
		name string
		data []byte
	}{
		{"the signature alone", whole[:8]},
		{"a header cut short", whole[:20]},
		{"a header and no pixel", testkit.PNGHeader(256, 256, false)},
		{"pixel data cut short", whole[:len(whole)/2]},
		{"no end", whole[:len(whole)-12]},
	}
	for _, entry := range damaged {
		e := refusedPicture(t, entry.data, file, unreadable)
		if e.Cause == nil || !strings.HasSuffix(e.Msg, ".") || strings.HasSuffix(e.Msg, "..") {
			t.Errorf("%s: %+v", entry.name, e)
		}
	}
	// One changed byte inside the pixel data fails its chunk's check value.
	changed := slices.Clone(whole)
	changed[len(changed)/2] ^= 0xff
	refusedPicture(t, changed, file, unreadable)
}

func TestTheExtensionDecidesHowAPictureIsReadAndAnotherExtensionIsRefused(t *testing.T) {
	const message = "must be a .tga, a .blp or a .png file."
	picture := testkit.TGA(testkit.NewPixels(256), testkit.TGAOptions{})
	e := refusedPicture(t, picture, "preview.jpg", message)
	if !strings.Contains(e.Hint, "TGA") {
		t.Errorf("hint = %q", e.Hint)
	}
	refusedPicture(t, picture, "preview", message)
	refusedPicture(t, picture, "art/.tga", message)
	refusedPicture(t, testkit.PNG(testkit.NewPixels(256), "rgb"), "art/.png", message)
	// A BLP or a PNG under a .tga name is read as a TGA and refused, never passed through.
	if _, err := settings.ReadPicture(testkit.BLP(256, 1), "preview.tga"); err == nil {
		t.Error("a BLP under a .tga name was accepted")
	}
	if _, err := settings.ReadPicture(testkit.PNG(testkit.NewPixels(256), "rgb"), "preview.tga"); err == nil {
		t.Error("a PNG under a .tga name was accepted")
	}
}
