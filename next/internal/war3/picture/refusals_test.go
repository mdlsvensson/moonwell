package picture_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/war3/picture"
)

// unreadablePNG is how the refusal of a PNG that the decoder gave up starts; the decoder's own words follow it.
const unreadablePNG = "The preview picture is a PNG that could not be read: "

// refusals is what read says of every file the lists of this package hold as refused: the TGA files the reader
// does not know, the sizes the game does not show, the BLP and the PNG files the game could not read, and the
// names that are of no picture. A file that is read is left out. The decoder's own words in the refusal of a
// damaged PNG, which the error carries as its cause, are another version of Go's to change, so the recording
// has <reason> in their place.
func refusals(read func(data []byte, file string) error,
	refusal func(input string, err error) testkit.Refusal) []testkit.Refusal {
	var said []testkit.Refusal
	for _, c := range slices.Concat(tgaRefusals(), sizeRefusals(), blpRefusals(), pngSizeRefusals(), notPNGRefusals(),
		damagedPNGRefusals(), nameRefusals(), otherFormatRefusals()) {
		err := read(c.data, c.file)
		if err == nil {
			continue
		}
		words := refusal(c.name+", as "+c.file, err)
		if cause := errors.Unwrap(err); cause != nil && strings.HasPrefix(words.Message, unreadablePNG) {
			reason := strings.TrimPrefix(cause.Error(), "png: ")
			words.Message = string(testkit.Placed([]byte(words.Message), "", reason))
		}
		said = append(said, words)
	}
	return said
}

// TestRefusalsAreAsRecorded holds what Read says of every file of refusals to the recording: the file, the
// message and the hint of each error, whole. The tests of an error beside it ask for its file, its
// distinguishing words and a hint; the words themselves are held here, where a change of them is one line of a
// diff.
func TestRefusalsAreAsRecorded(t *testing.T) {
	said := refusals(func(data []byte, file string) error {
		_, err := picture.Read(data, file)
		return err
	}, testkit.RefusalOf)
	if len(said) < 60 {
		t.Errorf("only %d files are refused", len(said))
	}
	testkit.Recorded(t, "refusals.txt", testkit.Refusals(said))
}
