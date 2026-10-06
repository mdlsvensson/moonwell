package w3i_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/war3/w3i"
)

// readers are the three doors of a package that reads and edits a war3map.w3i, each as the error it returns,
// and how an error of theirs is written into a recording.
type readers struct {
	read    func(data []byte, depth w3i.Depth) error
	header  func(data []byte) error
	edits   func(source []byte, edits []w3i.Edit) error
	refusal func(input string, err error) testkit.Refusal
}

// refusals is what the readers say of every input they refuse among these: the file World Editor saved, cut
// where each part of it ends; the two layouts without details, read Extended; every altered file; a header of
// two bytes; and edits that overlap. An altered file that is read is left out.
func refusals(t *testing.T, r readers) []testkit.Refusal {
	t.Helper()
	var said []testkit.Refusal
	add := func(name string, data []byte, depth w3i.Depth) {
		if err := r.read(data, depth); err != nil {
			said = append(said, r.refusal(fmt.Sprintf("%s, depth %d", name, depth), err))
		}
	}
	saved := testkit.Fixture(t, "map-settings-v39/war3map.w3i")
	for _, length := range []int{2, 30, 145, 157, 250, 280, 570} {
		add(fmt.Sprintf("the fixture cut at %d bytes", length), saved[:length], w3i.Extended)
	}
	for _, version := range []int32{18, 25} {
		add(fmt.Sprintf("synthetic version %d", version), testkit.SyntheticMapInfo(version), w3i.Extended)
	}
	for _, file := range alteredFiles(t) {
		// A layout without details says the same of every file that is read Extended, so its files are read
		// Basic; the newest layout is read at both depths.
		old := strings.HasPrefix(file.name, "synthetic version 18 ") || strings.HasPrefix(file.name, "synthetic version 25 ")
		if old || strings.HasPrefix(file.name, "synthetic version 39 ") {
			add(file.name, file.data, w3i.Basic)
		}
		if !old {
			add(file.name, file.data, w3i.Extended)
		}
	}
	source := testkit.SyntheticMapInfo(39)
	return append(said,
		r.refusal("a header of two bytes", r.header([]byte{1, 2})),
		r.refusal("edits that overlap", r.edits(source, []w3i.Edit{edit(4, 8, ""), edit(6, 9, "")})),
	)
}

// TestRefusalsAreAsRecorded holds what the package says of every input of refusals to the recording: the file,
// the message and the hint of each error, whole. The tests of an error beside it ask for its file, its
// distinguishing words and a hint; the words themselves are held here, where a change of them is one line of a
// diff.
func TestRefusalsAreAsRecorded(t *testing.T) {
	said := refusals(t, readers{
		read: func(data []byte, depth w3i.Depth) error {
			_, err := w3i.Read(data, mapInfoFile, depth)
			return err
		},
		header: func(data []byte) error {
			_, err := w3i.ReadHeader(data)
			return err
		},
		edits: func(source []byte, edits []w3i.Edit) error {
			_, err := w3i.ApplyEdits(source, edits)
			return err
		},
		refusal: testkit.RefusalOf,
	})
	if len(said) < 500 {
		t.Errorf("only %d inputs are refused", len(said))
	}
	testkit.Recorded(t, "refusals.txt", testkit.Refusals(said))
}
