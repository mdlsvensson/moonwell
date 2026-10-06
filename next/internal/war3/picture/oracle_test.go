package picture_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	olddiag "github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/settings"
	oldtestkit "github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/oracle"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/war3/picture"
)

// No file is left out of the comparisons of this package: the two trees are to say the same of every file under
// every name. The tests that read hundreds of pictures run beside the others (t.Parallel): each makes its own
// pictures and shares nothing with the rest.

// comparison gives the other tree's reader and this one the same file, compares what they return through the
// oracle, and counts the comparisons it made, the pictures both trees took and the files both refused.
type comparison struct {
	t        *testing.T
	count    int
	accepted int
	refused  int
	// messages are the refusals of the other tree, each with the number of files it was given for.
	messages map[string]int
}

func newComparison(t *testing.T) *comparison {
	return &comparison{t: t, messages: map[string]int{}}
}

// said is what an error tells its reader, in the shape both trees are compared in.
type said struct {
	Expected        bool // a diag error
	Msg, File, Hint string
	Line, Column    int
	Cause           string
}

// causeText is the text of the error a diag error wraps, or nothing.
func causeText(cause error) string {
	if cause == nil {
		return ""
	}
	return cause.Error()
}

func saidByTheOtherTree(err error) said {
	var failure *olddiag.Error
	if errors.As(err, &failure) {
		return said{true, failure.Msg, failure.File, failure.Hint, failure.Line, failure.Column, causeText(failure.Cause)}
	}
	return said{Msg: err.Error()}
}

func saidByThisTree(err error) said {
	var failure *diag.Error
	if errors.As(err, &failure) {
		return said{true, failure.Msg, failure.File, failure.Hint, failure.Line, failure.Column, causeText(failure.Cause)}
	}
	return said{Msg: err.Error()}
}

// read compares what both trees make of data under the name file: that both take it or both refuse it, every word
// of a refusal, and the extension and the bytes of a picture. Each tree is given bytes of its own, and this tree
// must leave its bytes as they were.
func (c *comparison) read(what string, data []byte, file string) {
	c.t.Helper()
	given := bytes.Clone(data)
	want, wantErr := settings.ReadPicture(bytes.Clone(data), file)
	got, gotErr := picture.Read(given, file)
	if !bytes.Equal(given, data) {
		c.t.Errorf("%s: Read changed the bytes it was given", what)
	}
	c.count++
	if oracle.Errors(c.t, what, wantErr, gotErr) {
		c.count++
		oracle.Values(c.t, what+": the error", saidByTheOtherTree(wantErr), saidByThisTree(gotErr))
		if got != nil {
			c.t.Errorf("%s: a refused picture returned %d bytes", what, len(got.Bytes))
		}
		c.refused++
		c.messages[wantErr.Error()]++
		return
	}
	if want == nil || got == nil {
		return
	}
	c.accepted++
	c.count += 2
	oracle.Values(c.t, what+": the extension", want.Extension, got.Extension)
	oracle.Bytes(c.t, what, want.Bytes, got.Bytes)
}

// atLeast fails the test unless both trees took at least accepted pictures and refused at least refused files, so
// that a test which compared nothing, or only refusals, does not pass. It logs what was compared.
func (c *comparison) atLeast(accepted, refused int) {
	c.t.Helper()
	if c.accepted < accepted || c.refused < refused {
		c.t.Errorf("%d pictures were accepted and %d files refused, want at least %d and %d",
			c.accepted, c.refused, accepted, refused)
	}
	c.log()
}

// exactly fails the test unless both trees took accepted pictures and refused refused files. It logs what was
// compared.
func (c *comparison) exactly(accepted, refused int) {
	c.t.Helper()
	if c.accepted != accepted || c.refused != refused {
		c.t.Errorf("%d pictures were accepted and %d files refused, want %d and %d",
			c.accepted, c.refused, accepted, refused)
	}
	c.log()
}

func (c *comparison) log() {
	c.t.Helper()
	c.t.Logf("%d accepted, %d refused with %d different messages, %d comparisons",
		c.accepted, c.refused, len(c.messages), c.count)
}

// refusedWith is the number of files the other tree refused with a message that has these words.
func (c *comparison) refusedWith(words string) int {
	count := 0
	for message, files := range c.messages {
		if strings.Contains(message, words) {
			count += files
		}
	}
	return count
}

// TestOracleOnThePicturesOfTheTestKit proves that the builders of this tree's test kit write what the other
// tree's do, so that both trees' tests lean on the same second implementation of the formats.
func TestOracleOnThePicturesOfTheTestKit(t *testing.T) {
	count := 0
	same := func(what string, want, got []byte) {
		t.Helper()
		count++
		oracle.Bytes(t, what, want, got)
	}
	const size = 64
	for name, sources := range map[string][2][]byte{
		"colour": {oldtestkit.NewPixels(size).RGBA, testkit.NewPixels(size).RGBA},
		"grey":   {oldtestkit.GreyPixels(size).RGBA, testkit.GreyPixels(size).RGBA},
	} {
		same(name+" pixels", sources[0], sources[1])
		want, got := oldtestkit.Pixels{Size: size, RGBA: sources[0]}, testkit.Pixels{Size: size, RGBA: sources[1]}
		for _, layout := range tgaLayouts() {
			same(fmt.Sprintf("%s TGA, %+v", name, layout),
				oldtestkit.TGA(want, oldtestkit.TGAOptions(layout)), testkit.TGA(got, layout))
		}
		for _, kind := range pngKinds[name] {
			same(name+" PNG, "+kind, oldtestkit.PNG(want, kind), testkit.PNG(got, kind))
		}
	}
	for _, content := range []uint32{0, 1} {
		same(fmt.Sprintf("BLP with content %d", content), oldtestkit.BLP(size, content), testkit.BLP(size, content))
	}
	same("PNG header", oldtestkit.PNGHeader(256, 512, true), testkit.PNGHeader(256, 512, true))
	if want := 2*(1+len(tgaLayouts())) + 8 + 3; count != want {
		t.Errorf("%d comparisons, want %d", count, want)
	}
	t.Logf("%d comparisons", count)
}

// tgaLayouts are the layouts the test kit writes a TGA in: plain and run-length encoded, 24 and 32 bits, rows
// from the bottom and from the top, without and with an ID field, and of 32 bits with the picture's own alpha,
// with none and with all of it.
func tgaLayouts() []testkit.TGAOptions {
	transparent := byte(0)
	var layouts []testkit.TGAOptions
	for _, rle := range []bool{false, true} {
		for _, fromTop := range []bool{false, true} {
			for _, id := range []int{0, 5} {
				layouts = append(layouts,
					testkit.TGAOptions{RLE: rle, Depth: 24, FromTop: fromTop, ID: id},
					testkit.TGAOptions{RLE: rle, Depth: 32, FromTop: fromTop, ID: id},
					testkit.TGAOptions{RLE: rle, Depth: 32, FromTop: fromTop, ID: id, Alpha: &transparent},
					testkit.TGAOptions{RLE: rle, Depth: 32, FromTop: fromTop, ID: id, Alpha: opaque()},
				)
			}
		}
	}
	return layouts
}

func TestOracleOnEveryTGALayout(t *testing.T) {
	t.Parallel()
	c := newComparison(t)
	for _, size := range []int{256, 512} {
		for _, name := range []string{"colour", "grey"} {
			source := pngSource(name, size)
			for _, layout := range tgaLayouts() {
				what := fmt.Sprintf("%s picture of %d, rle %v, %d bits, from top %v, id %d, alpha %v",
					name, size, layout.RLE, layout.Depth, layout.FromTop, layout.ID, layout.Alpha != nil)
				c.read(what, testkit.TGA(source, layout), "art/Preview.TGA")
			}
		}
	}
	c.exactly(2*2*32, 0)
}

// TestOracleOnTGAHeaderFieldsTheReaderPassesOver sets the fields of a TGA's header that say nothing about how its
// pixels lie: the colour map's own fields, the origin, the alpha bits and the two bits above the row order.
func TestOracleOnTGAHeaderFieldsTheReaderPassesOver(t *testing.T) {
	c := newComparison(t)
	for _, rle := range []bool{false, true} {
		source := testkit.TGA(testkit.NewPixels(256), testkit.TGAOptions{RLE: rle, Depth: 24, FromTop: true})
		for offset := 3; offset < 12; offset++ {
			c.read(fmt.Sprintf("rle %v, byte %d set", rle, offset),
				edited(slices.Clone(source), func(b []byte) { b[offset] = 0xff }), "preview.tga")
		}
		for _, descriptor := range []byte{0x00, 0x08, 0x0f, 0x20, 0x2f, 0x40, 0x80, 0xc0, 0xef} {
			c.read(fmt.Sprintf("rle %v, descriptor %#x", rle, descriptor),
				edited(slices.Clone(source), func(b []byte) { b[17] = descriptor }), "preview.tga")
		}
		footer := append(slices.Clone(source), "TRUEVISION-XFILE.\x00"...)
		c.read(fmt.Sprintf("rle %v, a footer", rle), footer, "preview.tga")
		// An ID field as long as its one byte can say.
		long := testkit.TGA(testkit.NewPixels(256), testkit.TGAOptions{RLE: rle, ID: 255})
		c.read(fmt.Sprintf("rle %v, an ID field of 255 bytes", rle), long, "preview.tga")
	}
	c.exactly(2*(9+9+1+1), 0)
}

// packets is stored pixels of step bytes as run-length packets of seeded lengths and kinds, which run from one
// row into the next. A packet that repeats stands for its first pixel alone, so the packets are a picture of their
// own.
func packets(random *rand.Rand, stored []byte, step int) []byte {
	var out []byte
	for at := 0; at < len(stored); {
		length := 1 + random.IntN(min(128, (len(stored)-at)/step))
		if random.IntN(2) == 0 {
			out = append(out, byte(0x80|(length-1)))
			out = append(out, stored[at:at+step]...)
		} else {
			out = append(out, byte(length-1))
			out = append(out, stored[at:at+length*step]...)
		}
		at += length * step
	}
	return out
}

// TestOracleOnRunLengthPacketsOfAnyLength gives both trees packets the test kit does not write: packets of every
// length that run over the ends of rows, and bytes of no meaning read as packets, which end early or overrun the
// picture.
func TestOracleOnRunLengthPacketsOfAnyLength(t *testing.T) {
	t.Parallel()
	c := newComparison(t)
	random := rand.New(rand.NewPCG(16, 2026))
	for _, depth := range []int{24, 32} {
		for _, fromTop := range []bool{false, true} {
			plain := testkit.TGA(testkit.NewPixels(256), testkit.TGAOptions{Depth: depth, FromTop: fromTop})
			header := edited(slices.Clone(plain[:tgaHeaderSize]), func(b []byte) { b[2] = 10 })
			what := fmt.Sprintf("%d bits, from top %v", depth, fromTop)
			for i := range 10 {
				data := slices.Concat(header, packets(random, plain[tgaHeaderSize:], depth/8))
				c.read(fmt.Sprintf("%s, packets %d", what, i), data, "preview.tga")
			}
			// Of every second file each byte has its high bit set, so each packet repeats a pixel and a few
			// thousand bytes stand for more pixels than the picture has.
			for i := range 60 {
				noise := make([]byte, random.IntN(12000))
				for at := range noise {
					noise[at] = byte(random.IntN(256)) | byte(i%2*0x80)
				}
				c.read(fmt.Sprintf("%s, noise %d", what, i), slices.Concat(header, noise), "preview.tga")
			}
		}
	}
	if early, overrun := c.refusedWith("ends early"), c.refusedWith("overruns"); early < 20 || overrun < 20 {
		t.Errorf("%d files ended early and %d had a run that overruns, want at least 20 of each", early, overrun)
	}
	c.atLeast(40, 200)
}

// palettePNG is a PNG of size pixels a side with a palette of that many colours, of which every second one is
// see-through when transparent is set. The encoder stores a pixel in the fewest bits the palette allows.
func palettePNG(size, colours int, transparent bool) []byte {
	palette := make(color.Palette, colours)
	for index := range palette {
		alpha := byte(255)
		if transparent && index%2 == 1 {
			alpha = byte(index)
		}
		palette[index] = color.NRGBA{R: byte(index * 37), G: byte(255 - index*11), B: byte(index * 5), A: alpha}
	}
	indexed := image.NewPaletted(image.Rect(0, 0, size, size), palette)
	for at := range indexed.Pix {
		indexed.Pix[at] = byte((at%size*3 + at/size*5) % colours)
	}
	var out bytes.Buffer
	if err := png.Encode(&out, indexed); err != nil {
		panic(err)
	}
	return out.Bytes()
}

func TestOracleOnEveryPNGKind(t *testing.T) {
	t.Parallel()
	c := newComparison(t)
	for _, size := range []int{256, 512} {
		for _, name := range []string{"colour", "grey"} {
			for _, kind := range pngKinds[name] {
				c.read(fmt.Sprintf("%s picture of %d as %s", name, size, kind),
					testkit.PNG(pngSource(name, size), kind), "art/Preview.PNG")
			}
		}
	}
	// The kinds meant for colour, of the picture of greys, whose alpha is 255.
	for _, kind := range pngKinds["colour"] {
		c.read("grey picture of 256 as "+kind, testkit.PNG(testkit.GreyPixels(256), kind), "preview.png")
	}
	for _, alpha := range []byte{0, 1, 128, 254, 255} {
		for _, kind := range []string{"rgba", "rgba16", "interlaced"} {
			c.read(fmt.Sprintf("alpha %d as %s", alpha, kind), testkit.PNG(withAlpha(alpha), kind), "preview.png")
		}
	}
	for kind, transparent := range transparentColourPNGs() {
		c.read(kind+" with a transparent colour", transparent.data, "preview.png")
	}
	for _, colours := range []int{2, 4, 16, 200} {
		for _, transparent := range []bool{false, true} {
			c.read(fmt.Sprintf("a palette of %d colours, transparent %v", colours, transparent),
				palettePNG(256, colours, transparent), "preview.png")
		}
	}
	c.exactly(2*8+5+15+4+8, 0)
}

func TestOracleOnBLPs(t *testing.T) {
	c := newComparison(t)
	for _, size := range []int{256, 512} {
		for _, content := range []uint32{0, 1} {
			c.read(fmt.Sprintf("size %d, content %d", size, content), testkit.BLP(size, content), "Preview.BLP")
		}
	}
	whole := testkit.BLP(256, 1)
	rest := uint32(len(whole) - 156)
	c.read("a first mipmap right after the header", edited(slices.Clone(whole), putU32(28, 156)), "preview.blp")
	c.read("a first mipmap of everything after the header",
		edited(edited(slices.Clone(whole), putU32(28, 156)), putU32(92, rest)), "preview.blp")
	c.read("a first mipmap of one byte at the end",
		edited(edited(slices.Clone(whole), putU32(28, 155+rest)), putU32(92, 1)), "preview.blp")
	c.read("bytes after the first mipmap", append(slices.Clone(whole), 1, 2, 3), "preview.blp")
	c.exactly(8, 0)

	// What goes into the map is the file itself.
	result, err := picture.Read(whole, "preview.blp")
	if err != nil || !bytes.Equal(result.Bytes, testkit.BLP(256, 1)) {
		t.Errorf("a BLP is not returned as it is: %v", err)
	}
}

// TestOracleOnRefusedPictures gives both trees every file the tests of this package expect Read to refuse.
func TestOracleOnRefusedPictures(t *testing.T) {
	c := newComparison(t)
	cases := slices.Concat(tgaRefusals(), sizeRefusals(), blpRefusals(), pngSizeRefusals(), notPNGRefusals(),
		damagedPNGRefusals(), nameRefusals(), otherFormatRefusals())
	for _, refusal := range cases {
		c.read(refusal.name+" as "+refusal.file, refusal.data, refusal.file)
	}
	if len(cases) < 60 {
		t.Errorf("only %d refusals were compared", len(cases))
	}
	c.exactly(0, len(cases))
}

// TestOracleOnNames gives both trees one picture of each format under names in every letter case, with folders
// before them and with dots in other places than before the extension.
func TestOracleOnNames(t *testing.T) {
	c := newComparison(t)
	pictures := map[string][]byte{
		"tga": plainTGA(), "png": testkit.PNG(testkit.NewPixels(256), "rgb"), "blp": testkit.BLP(256, 1),
	}
	for _, name := range acceptedNames {
		c.read(fmt.Sprintf("a TGA named %q", name), pictures["tga"], name)
	}
	accepted := len(acceptedNames)
	for _, format := range []string{"tga", "png", "blp"} {
		upper := string(bytes.ToUpper([]byte(format)))
		mixed := upper[:1] + format[1:]
		for _, name := range []string{
			"preview." + format, "PREVIEW." + upper, "Preview." + mixed, "art/sub/preview." + format,
			`art\sub\Preview.` + upper, `C:\maps\my map/preview.` + mixed, "." + format + "." + format,
		} {
			c.read(fmt.Sprintf("a %s named %q", upper, name), pictures[format], name)
			accepted++
		}
		// The last two names have a letter outside ASCII that lower case turns into an ASCII one, with or
		// without a mark: the Kelvin sign before the extension and the dotted capital I after it.
		for _, name := range []string{
			"." + format, "preview/." + format, "preview." + format + "/", "preview." + format + `\`,
			"preview." + format + ".bak", "preview" + format,
			"preview.\xE2\x84\xAA" + format, "preview." + format + "\xC4\xB0",
		} {
			c.read(fmt.Sprintf("a %s named %q", upper, name), pictures[format], name)
		}
	}
	c.exactly(accepted, 3*8)
}

// cutLengths are every length below 200 and a few hundred seeded lengths from there up to one byte less than
// whole.
func cutLengths(random *rand.Rand, whole, seeded int) []int {
	var lengths []int
	for length := range min(200, whole) {
		lengths = append(lengths, length)
	}
	for range seeded {
		lengths = append(lengths, 200+random.IntN(whole-200))
	}
	return append(lengths, whole-1)
}

// TestOracleOnPicturesCutShort proves that both trees say the same of a picture wherever it ends. A picture of
// these sizes has too many bytes to cut at each of them, so every length inside and near the header is taken, and
// of the lengths after it a seeded few hundred.
func TestOracleOnPicturesCutShort(t *testing.T) {
	t.Parallel()
	c := newComparison(t)
	random := rand.New(rand.NewPCG(14, 2026))
	source := testkit.NewPixels(256)
	cuts := 0
	for _, whole := range []struct {
		name, file string
		data       []byte
	}{
		{"a plain TGA", "preview.tga", testkit.TGA(source, testkit.TGAOptions{ID: 7})},
		{"a run-length TGA", "preview.tga",
			testkit.TGA(source, testkit.TGAOptions{RLE: true, Depth: 24, FromTop: true, ID: 7})},
		{"a PNG", "preview.png", testkit.PNG(source, "rgba")},
		{"an interlaced PNG", "preview.png", testkit.PNG(source, "interlaced")},
		{"a BLP", "preview.blp", testkit.BLP(256, 1)},
	} {
		for _, length := range cutLengths(random, len(whole.data), 300) {
			c.read(fmt.Sprintf("%s cut at %d bytes", whole.name, length), whole.data[:length:length], whole.file)
			cuts++
		}
	}
	c.exactly(0, cuts)
}

// headerSizes are the bytes at the start of a file of each format that say how the rest is laid out.
var headerSizes = map[string]int{"tga": 18, "png": pngHeaderEnd, "blp": 156}

// edgeValues are numbers at and beside the edges of what a header may say: sizes, depths, kinds and offsets.
var edgeValues = []uint32{0, 1, 2, 3, 8, 9, 10, 11, 16, 24, 32, 127, 128, 155, 156, 255, 256, 257, 511, 512, 513,
	1024, 0x10, 0x20, 0x7FFFFFFF, 0x80000000, 0xFFFFFFFF}

// changed is data with one change in its first 200 bytes, half of the time inside the header: one byte set, or
// four bytes set to a number in either byte order.
func changed(random *rand.Rand, data []byte, headerSize int) []byte {
	data = bytes.Clone(data)
	region := 200
	if random.IntN(2) == 0 {
		region = headerSize
	}
	value := edgeValues[random.IntN(len(edgeValues))]
	if random.IntN(2) == 0 {
		value = random.Uint32()
	}
	switch random.IntN(3) {
	case 0:
		data[random.IntN(region)] = byte(value)
	case 1:
		binary.LittleEndian.PutUint32(data[random.IntN(region-3):], value)
	case 2:
		binary.BigEndian.PutUint32(data[random.IntN(region-3):], value)
	}
	return data
}

// TestOracleOnPicturesWithAChangedHeader proves that both trees agree on a picture whose header says something
// else than its builder wrote: on the picture when it still reads, and on the refusal when it does not.
func TestOracleOnPicturesWithAChangedHeader(t *testing.T) {
	t.Parallel()
	random := rand.New(rand.NewPCG(15, 2026))
	source := testkit.NewPixels(256)
	for _, whole := range []struct {
		name, format string
		data         []byte
		// The changes that leave a picture both trees take, and those both refuse: at least this many.
		accepted, refused int
	}{
		{"a plain TGA", "tga", testkit.TGA(source, testkit.TGAOptions{}), 100, 50},
		{"a run-length TGA", "tga", testkit.TGA(source, testkit.TGAOptions{RLE: true, FromTop: true}), 50, 50},
		// Every change of a PNG fails a check value; TestOracleOnPNGHeadersOfEveryKind changes headers and
		// writes their check values anew.
		{"a PNG", "png", testkit.PNG(source, "rgb"), 0, 300},
		{"a BLP", "blp", testkit.BLP(256, 1), 100, 30},
	} {
		c := newComparison(t)
		for i := range 400 {
			data := changed(random, whole.data, headerSizes[whole.format])
			c.read(fmt.Sprintf("%s, change %d", whole.name, i), data, "preview."+whole.format)
		}
		t.Log(whole.name)
		c.atLeast(whole.accepted, whole.refused)
	}
}

// pngHeader is the signature and the header chunk of a PNG that says these things of itself, with a check value
// that is right.
func pngHeader(width, height uint32, depth, colourType, compression, filter, interlace byte) []byte {
	fields := binary.BigEndian.AppendUint32(binary.BigEndian.AppendUint32(nil, width), height)
	fields = append(fields, depth, colourType, compression, filter, interlace)
	return slices.Concat(testkit.PNGHeader(1, 1, false)[:8], pngChunk("IHDR", fields))
}

// TestOracleOnPNGHeadersOfEveryKind gives both trees PNG headers of every bit depth, colour type and interlace
// method, valid or not, and of sizes at the edges: each alone, and each before the pixels of a whole picture that
// was stored in another way.
func TestOracleOnPNGHeadersOfEveryKind(t *testing.T) {
	c := newComparison(t)
	whole := testkit.PNG(testkit.NewPixels(256), "rgb")
	pixels := whole[pngHeaderEnd:]
	header := func(what string, header []byte) {
		t.Helper()
		c.read(what+", alone", header, "preview.png")
		c.read(what+", before pixels", slices.Concat(header, pixels), "preview.png")
	}
	for _, depth := range []byte{0, 1, 2, 4, 8, 16, 32} {
		for _, colourType := range []byte{0, 1, 2, 3, 4, 5, 6, 7} {
			for _, interlace := range []byte{0, 1, 2} {
				header(fmt.Sprintf("depth %d, colour type %d, interlace %d", depth, colourType, interlace),
					pngHeader(256, 256, depth, colourType, 0, 0, interlace))
			}
		}
	}
	header("another compression method", pngHeader(256, 256, 8, 2, 1, 0, 0))
	header("another filter method", pngHeader(256, 256, 8, 2, 0, 1, 0))
	sizes := []uint32{0, 1, 255, 256, 257, 512, 1024, 0x7FFFFFFF, 0x80000000, 0x80000100, 0xFFFFFFFF}
	for _, width := range sizes {
		for _, height := range sizes {
			header(fmt.Sprintf("%dx%d", width, height), pngHeader(width, height, 8, 2, 0, 0, 0))
		}
	}
	// Of all of them, the one header the pixels were stored under gives a picture.
	c.exactly(2, 2*(7*8*3+2+len(sizes)*len(sizes))-2)
}
