package picture_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/war3/picture"
)

const tgaHeaderSize = 18

type checkError struct {
	name  string
	data  []byte
	file  string
	words string
}

func (r checkError) check(t *testing.T) *diag.Error {
	t.Helper()
	result, err := picture.Read(r.data, r.file)
	var diagErr *diag.Error
	if !errors.As(err, &diagErr) || diagErr.File != r.file {
		t.Errorf("%s: got %v, want an error naming %s", r.name, err, r.file)
		return nil
	}
	if !strings.Contains(diagErr.Msg, r.words) || diagErr.Hint == "" {
		t.Errorf("%s: message %q, hint %q, want a message with %q and a hint", r.name, diagErr.Msg, diagErr.Hint, r.words)
	}
	if result != nil {
		t.Errorf("%s: a refused picture returned %d bytes", r.name, len(result.Data))
	}
	return diagErr
}

func opaque() *byte {
	alpha := byte(255)
	return &alpha
}

func plainTGA() []byte { return testkit.TGA(testkit.NewPixels(256), testkit.TGAOptions{}) }

func withEdit(data []byte, change func([]byte)) []byte {
	change(data)
	return data
}

func setU32At(offset int, value uint32) func([]byte) {
	return func(b []byte) { binary.LittleEndian.PutUint32(b[offset:], value) }
}

func TestEveryAcceptedTGAIsWrittenAsTheOneLayoutTheGameWasSeenToAccept(t *testing.T) {
	for _, size := range []int{256, 512} {
		source := testkit.NewPixels(size)
		expected := testkit.TGA(source, testkit.TGAOptions{Alpha: opaque()})
		if expected[2] != 2 || expected[16] != 32 || expected[17] != 8 {
			t.Fatalf("the expected TGA has header % X", expected[:tgaHeaderSize])
		}
		for _, rle := range []bool{false, true} {
			for _, depth := range []int{24, 32} {
				for _, fromTop := range []bool{false, true} {
					for _, id := range []int{0, 5} {
						data := testkit.TGA(source, testkit.TGAOptions{RLE: rle, Depth: depth, FromTop: fromTop, ID: id})
						result, err := picture.Read(data, "art/Preview.TGA")
						if err != nil || result.Extension != "tga" || !bytes.Equal(result.Data, expected) {
							t.Errorf("size %d, rle %v, %d bits, from top %v, id %d: %v", size, rle, depth, fromTop, id, err)
						}
					}
				}
			}
		}
	}
}

func TestAWrittenTGAStartsWithThePicturesBottomRowAndHasNothingAfterItsPixels(t *testing.T) {
	source := testkit.NewPixels(256)
	result, err := picture.Read(testkit.TGA(source, testkit.TGAOptions{FromTop: true}), "preview.tga")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Data) != tgaHeaderSize+256*256*4 {
		t.Errorf("the written TGA has %d bytes", len(result.Data))
	}
	if got := result.Data[tgaHeaderSize : tgaHeaderSize+4]; !bytes.Equal(got, []byte{200, 40, 255, 255}) {
		t.Errorf("the first pixel is %v", got)
	}
	footer := append(testkit.TGA(source, testkit.TGAOptions{}), "TRUEVISION-XFILE.\x00"...)
	again, err := picture.Read(footer, "preview.tga")
	if err != nil || !bytes.Equal(again.Data, result.Data) {
		t.Errorf("a TGA with a footer: %v", err)
	}
}

func TestTheAlphaBitsOfATGAsDescriptorDoNotMoveItsRows(t *testing.T) {
	const descriptor = 17
	for _, options := range []testkit.TGAOptions{{}, {FromTop: true}, {RLE: true}} {
		source := testkit.TGA(testkit.NewPixels(256), options)
		want, err := picture.Read(source, "preview.tga")
		if err != nil {
			t.Fatal(err)
		}
		for _, bits := range []byte{0x01, 0x02, 0x04, 0x08, 0x0F} {
			with := withEdit(slices.Clone(source), func(b []byte) { b[descriptor] = b[descriptor]&0xF0 | bits })
			got, err := picture.Read(with, "preview.tga")
			if err != nil || !bytes.Equal(got.Data, want.Data) {
				t.Errorf("%+v with the alpha bits %#x: %v, or another picture", options, bits, err)
			}
		}
	}
}

func rlePacket(count int) []byte { return []byte{byte(0x80 | (count - 1)), 1, 2, 3, 255} }

func runsOfAllButOnePixel() []byte {
	header := testkit.TGA(testkit.NewPixels(256), testkit.TGAOptions{RLE: true})[:tgaHeaderSize]
	return slices.Concat(header, bytes.Repeat(rlePacket(128), 511), rlePacket(127))
}

func tgaRefusals() []checkError {
	const (
		early    = "its pixel data ends early"
		overruns = "a run of pixels overruns the picture"
	)
	plain, runs := plainTGA(), runsOfAllButOnePixel()
	withID := testkit.TGA(testkit.NewPixels(256), testkit.TGAOptions{ID: 9})
	rle := testkit.TGA(testkit.NewPixels(256), testkit.TGAOptions{RLE: true})
	set := func(offset int, value byte) []byte {
		return withEdit(plainTGA(), func(b []byte) { b[offset] = value })
	}
	cases := []checkError{
		{"a header cut short", make([]byte, 17), "", "a TGA header has 18 bytes"},
		{"no bytes", nil, "", "a TGA header has 18 bytes"},
		{"a colour map", set(1, 1), "", "a TGA with a colour map"},
		{"a grey picture", set(2, 3), "", "image type 3, not a true-colour picture"},
		{"16 bits a pixel", set(16, 16), "", "16 bits a pixel, not 24 or 32"},
		{"rows from right to left", set(17, plain[17]|0x10), "", "rows run from right to left"},
		{"a plain picture one byte short", plain[:len(plain)-1], "", early},
		{"an ID field that takes the place of the last pixels", withID[:len(withID)-9], "", early},
		{"a run-length picture one byte short", rle[:len(rle)-1], "", early},
		{"a run-length picture of one byte", rle[:tgaHeaderSize+1], "", early},
		{"a run-length picture without packets", rle[:tgaHeaderSize], "", early},
		{"a run of two pixels where one is left", slices.Concat(runs, rlePacket(2)), "", overruns},
		{"a packet of two pixels where one is left", slices.Concat(runs, []byte{1, 1, 2, 3, 255, 1, 2, 3, 255}), "", overruns},
		{"a last run whose pixel is cut off", slices.Concat(runs, []byte{0x80, 1, 2}), "", early},
	}
	for i := range cases {
		cases[i].file = "preview.tga"
	}
	return cases
}

func TestATGATheReaderDoesNotKnowIsRefusedByWhatItIs(t *testing.T) {
	for _, c := range tgaRefusals() {
		c.check(t)
	}
	result, err := picture.Read(slices.Concat(runsOfAllButOnePixel(), rlePacket(1)), "preview.tga")
	if err != nil || result.Data[tgaHeaderSize+2] != 3 {
		t.Errorf("a picture of runs: %v", err)
	}
}

func sizedTGA(width, height uint16) []byte {
	return withEdit(plainTGA(), func(b []byte) {
		binary.LittleEndian.PutUint16(b[12:], width)
		binary.LittleEndian.PutUint16(b[14:], height)
	})
}

func sizeRefusals() []checkError {
	return []checkError{
		{"a TGA of 128", sizedTGA(128, 128), "preview.tga", "is 128x128 pixels; it must be 256x256 or 512x512"},
		{"a wide TGA", sizedTGA(512, 256), "preview.tga", "is 512x256 pixels"},
		{"a tall TGA", sizedTGA(256, 512), "preview.tga", "is 256x512 pixels"},
		{"a TGA of 1024", sizedTGA(1024, 1024), "preview.tga", "is 1024x1024 pixels"},
		{"a TGA without pixels", sizedTGA(0, 0), "preview.tga", "is 0x0 pixels"},
		{"a BLP of 1024", testkit.BLP(1024, 1), "preview.blp", "is 1024x1024 pixels"},
		{"a BLP of 128", testkit.BLP(128, 1), "preview.blp", "is 128x128 pixels"},
		{"a wide BLP", withEdit(testkit.BLP(256, 1), setU32At(12, 512)), "preview.blp", "is 512x256 pixels"},
	}
}

func TestOnlyTheTwoSizesSeenToWorkAreAccepted(t *testing.T) {
	for _, c := range sizeRefusals() {
		if diagErr := c.check(t); diagErr != nil && !strings.Contains(diagErr.Hint, "256x256") {
			t.Errorf("%s: hint = %q", c.name, diagErr.Hint)
		}
	}
}

func TestABLP1WithJPEGOrPaletteContentIsUsedAsItIs(t *testing.T) {
	for _, size := range []int{256, 512} {
		for _, content := range []uint32{0, 1} {
			data := testkit.BLP(size, content)
			result, err := picture.Read(data, "Preview.BLP")
			if err != nil || result.Extension != "blp" || !bytes.Equal(result.Data, data) {
				t.Errorf("size %d, content %d: %v", size, content, err)
			}
		}
	}
	length := uint32(len(testkit.BLP(256, 1)))
	for _, c := range []struct {
		name         string
		offset, size uint32
	}{
		{"right after the header", 156, 256 * 256},
		{"of every byte after the header", 156, length - 156},
		{"of the last byte", length - 1, 1},
	} {
		data := withEdit(withEdit(testkit.BLP(256, 1), setU32At(28, c.offset)), setU32At(92, c.size))
		if result, err := picture.Read(data, "preview.blp"); err != nil || !bytes.Equal(result.Data, data) {
			t.Errorf("a first mipmap %s: %v", c.name, err)
		}
	}
}

func blpRefusals() []checkError {
	whole := testkit.BLP(256, 1)
	const outside = "its first mipmap lies outside the file"
	with := func(change func([]byte)) []byte { return withEdit(testkit.BLP(256, 1), change) }
	cases := []checkError{
		{"a BLP2", with(func(b []byte) { b[3] = '2' }), "", "is a BLP2 file, the World of Warcraft format"},
		{"a TGA", plainTGA(), "", "is not a BLP file: it does not start with BLP1"},
		{"no bytes", nil, "", "is not a BLP file"},
		{"three bytes of the magic", []byte("BLP"), "", "is not a BLP file"},
		{"a header cut short", whole[:155], "", "is cut short: a BLP header has 156 bytes"},
		{"a header alone", whole[:156], "", outside},
		{"the magic alone", whole[:4], "", "a BLP header has 156 bytes"},
		{"an unknown content type", with(setU32At(4, 2)), "", "has the unknown BLP content type 2"},
		{"a first mipmap inside the header", with(setU32At(28, 155)), "", outside},
		{"a first mipmap without bytes", with(setU32At(92, 0)), "", outside},
		{"a first mipmap far past the end", with(setU32At(28, 0xFFFFFFFF)), "", outside},
		{"a first mipmap of a size past every file", with(setU32At(92, 0xFFFFFFFF)), "", outside},
		{"a file one byte short", whole[:len(whole)-1], "", outside},
	}
	for i := range cases {
		cases[i].file = "preview.blp"
	}
	return cases
}

func TestABLPTheGameCouldNotReadIsRefused(t *testing.T) {
	for _, c := range blpRefusals() {
		diagErr := c.check(t)
		if c.name == "a BLP2" && diagErr != nil && !strings.Contains(diagErr.Hint, "BLP1") {
			t.Errorf("%s: hint = %q", c.name, diagErr.Hint)
		}
	}
}

var pngKinds = map[string][]string{
	"colour": {"rgba", "rgb", "rgba16", "rgb16", "interlaced"},
	"grey":   {"grey", "grey16", "palette"},
}

func pngSource(name string, size int) testkit.Pixels {
	if name == "grey" {
		return testkit.GreyPixels(size)
	}
	return testkit.NewPixels(size)
}

func TestAPNGOfAnyKindGoesInAsTheTGAThatATGAOfTheSamePictureGives(t *testing.T) {
	for _, size := range []int{256, 512} {
		for name, kinds := range pngKinds {
			source := pngSource(name, size)
			expected, err := picture.Read(testkit.TGA(source, testkit.TGAOptions{}), "preview.tga")
			if err != nil {
				t.Fatal(err)
			}
			for _, kind := range kinds {
				result, err := picture.Read(testkit.PNG(source, kind), "art/Preview.PNG")
				if err != nil || result.Extension != "tga" || !bytes.Equal(result.Data, expected.Data) {
					t.Errorf("size %d, %s: %v", size, kind, err)
				}
			}
		}
	}
}

func withAlpha(alpha byte) testkit.Pixels {
	source := testkit.NewPixels(256)
	for index := 3; index < len(source.RGBA); index += 4 {
		source.RGBA[index] = alpha
	}
	return source
}

func TestAPNGsTransparencyIsDroppedAndItsStoredColourKept(t *testing.T) {
	for _, alpha := range []byte{7, 0} {
		source := withAlpha(alpha)
		expected := testkit.TGA(source, testkit.TGAOptions{Alpha: opaque()})
		for _, kind := range []string{"rgba", "rgba16", "interlaced"} {
			result, err := picture.Read(testkit.PNG(source, kind), "preview.png")
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(result.Data, expected) {
				t.Errorf("alpha %d, %s: the picture is not the opaque TGA of its stored colours", alpha, kind)
			}
			if got := result.Data[tgaHeaderSize : tgaHeaderSize+4]; !bytes.Equal(got, []byte{200, 40, 255, 255}) {
				t.Errorf("alpha %d, %s: the first pixel is %v", alpha, kind, got)
			}
		}
	}
}

func pngChunk(name string, data []byte) []byte {
	out := binary.BigEndian.AppendUint32(nil, uint32(len(data)))
	out = append(out, name...)
	out = append(out, data...)
	return binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(out[4:]))
}

const pngHeaderEnd = 8 + 12 + 13

func withTransparentColour(data []byte, samples ...uint16) []byte {
	var colour []byte
	for _, sample := range samples {
		colour = binary.BigEndian.AppendUint16(colour, sample)
	}
	return slices.Concat(data[:pngHeaderEnd], pngChunk("tRNS", colour), data[pngHeaderEnd:])
}

func to16Bit(sample byte) uint16 { return uint16(sample)<<8 | 0xa5 }

func transparentColourPNGs() map[string]struct {
	source testkit.Pixels
	data   []byte
} {
	colour, grey := testkit.NewPixels(256), testkit.GreyPixels(256)
	return map[string]struct {
		source testkit.Pixels
		data   []byte
	}{
		"rgb":    {colour, withTransparentColour(testkit.PNG(colour, "rgb"), 255, 40, 200)},
		"rgb16":  {colour, withTransparentColour(testkit.PNG(colour, "rgb16"), to16Bit(255), to16Bit(40), to16Bit(200))},
		"grey":   {grey, withTransparentColour(testkit.PNG(grey, "grey"), 251)},
		"grey16": {grey, withTransparentColour(testkit.PNG(grey, "grey16"), to16Bit(251))},
	}
}

func TestAPNGsTransparentColourIsKeptAsItIsStored(t *testing.T) {
	for kind, c := range transparentColourPNGs() {
		expected := testkit.TGA(c.source, testkit.TGAOptions{Alpha: opaque()})
		result, err := picture.Read(c.data, "preview.png")
		if err != nil {
			t.Errorf("%s: %v", kind, err)
			continue
		}
		if !bytes.Equal(result.Data, expected) {
			t.Errorf("%s: the picture is not the opaque TGA of its stored colours; its first pixel is %v, want %v",
				kind, result.Data[tgaHeaderSize:tgaHeaderSize+4], expected[tgaHeaderSize:tgaHeaderSize+4])
		}
	}
}

func pngSizeRefusals() []checkError {
	cases := []checkError{
		{"a header of 128", testkit.PNGHeader(128, 128, false), "", "is 128x128 pixels; it must be 256x256 or 512x512"},
		{"a wide header", testkit.PNGHeader(512, 256, false), "", "is 512x256 pixels"},
		{"a tall interlaced header", testkit.PNGHeader(256, 512, true), "", "is 256x512 pixels"},
		{"a huge header", testkit.PNGHeader(60000, 60000, false), "", "is 60000x60000 pixels"},
		{"a picture of 64", testkit.PNG(testkit.NewPixels(64), "rgb"), "", "is 64x64 pixels"},
	}
	for i := range cases {
		cases[i].file = "preview.png"
	}
	return cases
}

func TestAPNGsSizeIsJudgedBeforeItsPixelsAreRead(t *testing.T) {
	for _, c := range pngSizeRefusals() {
		if diagErr := c.check(t); diagErr != nil && !strings.Contains(diagErr.Hint, "256x256") {
			t.Errorf("%s: hint = %q", c.name, diagErr.Hint)
		}
	}
}

func notPNGRefusals() []checkError {
	const words = "is not a PNG file"
	return []checkError{
		{"a TGA", plainTGA(), "preview.png", words + ": it does not start with a PNG signature"},
		{"a BLP", testkit.BLP(256, 1), "preview.png", words},
		{"no bytes", nil, "preview.png", words},
		{"seven bytes of the signature", testkit.PNGHeader(256, 256, false)[:7], "preview.png", words},
	}
}

func damagedPNGRefusals() []checkError {
	whole := testkit.PNG(testkit.NewPixels(256), "rgb")
	changed := withEdit(slices.Clone(whole), func(b []byte) { b[len(b)/2] ^= 0xff })
	var cases []checkError
	for _, c := range []struct {
		name string
		data []byte
	}{
		{"the signature alone", whole[:8]},
		{"a header cut short", whole[:20]},
		{"a header and no pixel", testkit.PNGHeader(256, 256, false)},
		{"pixel data cut short", whole[:len(whole)/2]},
		{"no end", whole[:len(whole)-12]},
		{"a changed byte", changed},
	} {
		cases = append(cases, checkError{c.name, c.data, "preview.png", "is a PNG that could not be read: "})
	}
	return cases
}

func TestAFileThatIsNotAReadablePNGIsRefused(t *testing.T) {
	for _, c := range notPNGRefusals() {
		if diagErr := c.check(t); diagErr != nil && !strings.Contains(diagErr.Hint, "PNG") {
			t.Errorf("%s: hint = %q", c.name, diagErr.Hint)
		}
	}
	for _, c := range damagedPNGRefusals() {
		diagErr := c.check(t)
		if diagErr == nil {
			continue
		}
		if diagErr.Cause == nil || !strings.HasSuffix(diagErr.Msg, ".") || strings.HasSuffix(diagErr.Msg, "..") ||
			strings.Contains(diagErr.Msg, "png: ") || !strings.Contains(diagErr.Hint, "PNG") {
			t.Errorf("%s: %+v", c.name, diagErr)
		}
	}
}

func nameRefusals() []checkError {
	const words = "must be a .tga, a .blp or a .png file"
	png := testkit.PNG(testkit.NewPixels(256), "rgb")
	return []checkError{
		{"another extension", plainTGA(), "preview.jpg", words},
		{"no extension", plainTGA(), "preview", words},
		{"no name at all", plainTGA(), "", words},
		{"a TGA whose name only starts with a dot", plainTGA(), "art/.tga", words},
		{"a PNG whose name only starts with a dot", png, "art/.png", words},
		{"a name that only starts with a dot, after a backslash", plainTGA(), `art\.tga`, words},
		{"a name that ends with a dot", plainTGA(), "preview.tga.", words},
		{"an extension on a folder", plainTGA(), "art.tga/preview", words},
		{"an extension on a folder, before a backslash", plainTGA(), `art.tga\preview`, words},
		{"an extension with a space after it", plainTGA(), "preview.tga ", words},
		{"a longer extension", plainTGA(), "preview.tga2", words},
	}
}

func otherFormatRefusals() []checkError {
	return []checkError{
		{"a BLP under a .tga name", testkit.BLP(256, 1), "preview.tga", "is a TGA"},
		{"a PNG under a .tga name", testkit.PNG(testkit.NewPixels(256), "rgb"), "preview.tga", "is a TGA"},
		{"a PNG under a .blp name", testkit.PNG(testkit.NewPixels(256), "rgb"), "preview.blp", "is not a BLP file"},
	}
}

func TestTheExtensionDecidesHowAPictureIsReadAndAnotherExtensionIsRefused(t *testing.T) {
	for _, c := range nameRefusals() {
		if diagErr := c.check(t); diagErr != nil && !strings.Contains(diagErr.Hint, "TGA") {
			t.Errorf("%s: hint = %q", c.name, diagErr.Hint)
		}
	}
	for _, c := range otherFormatRefusals() {
		c.check(t)
	}
}

var acceptedNames = []string{
	"preview.tga", "preview.TGA", "Preview.Tga", "art/preview.tga", `art\preview.tga`, `art\sub/Preview.TGA`,
	"art.png/preview.tga", `art.blp\preview.tga`, "preview.png.tga", "preview..tga", "..tga", "a.tga", " .tga",
	"C:/maps/my map/preview.tga", "pr\xC3\xA9view.tga",
}

func TestTheExtensionIsWhatFollowsTheLastDotOfTheLastPartOfThePathInAnyLetterCase(t *testing.T) {
	data := plainTGA()
	expected := testkit.TGA(testkit.NewPixels(256), testkit.TGAOptions{Alpha: opaque()})
	for _, name := range acceptedNames {
		result, err := picture.Read(data, name)
		if err != nil || result.Extension != "tga" || !bytes.Equal(result.Data, expected) {
			t.Errorf("%q: %v", name, err)
		}
	}
	for _, other := range []struct {
		name, extension string
		data            []byte
	}{
		{"Preview.BLP", "blp", testkit.BLP(256, 1)},
		{`art\Preview.Png`, "tga", testkit.PNG(testkit.NewPixels(256), "rgb")},
	} {
		if result, err := picture.Read(other.data, other.name); err != nil || result.Extension != other.extension {
			t.Errorf("%q: %v", other.name, err)
		}
	}
}

func TestReadLeavesTheBytesItIsGivenAsTheyAre(t *testing.T) {
	for name, data := range map[string][]byte{
		"preview.tga": testkit.TGA(testkit.NewPixels(256), testkit.TGAOptions{RLE: true, FromTop: true}),
		"preview.png": testkit.PNG(testkit.NewPixels(256), "rgba"),
		"preview.blp": testkit.BLP(256, 1),
	} {
		before := slices.Clone(data)
		if _, err := picture.Read(data, name); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !bytes.Equal(data, before) {
			t.Errorf("%s: Read changed the bytes it was given", name)
		}
	}
}
