package model_test

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/war3/model"
)

// refusals is what the three readers say of every model they refuse among these: the damaged binary models,
// through ReadMDX and through Paths; a texture file, through Paths; the broken texts and the texts that are no
// model, through ReadMDL and through Paths; and the binary model with every chunk with each of its sizes set to
// numbers at the edges and beside itself, through ReadMDX. read is given the name of a reader and the bytes,
// and returns that reader's error. A model that is read is left out.
func refusals(t *testing.T, read func(reader string, data []byte) error,
	refusal func(input string, err error) testkit.Refusal) []testkit.Refusal {
	t.Helper()
	var said []testkit.Refusal
	add := func(name, reader string, data []byte) {
		if err := read(reader, data); err != nil {
			said = append(said, refusal(name+", through "+reader, err))
		}
	}
	for _, damaged := range damagedModels() {
		add(damaged.name, "ReadMDX", damaged.data)
		add(damaged.name, "Paths", damaged.data)
	}
	add("a texture file", "Paths", []byte{0x42, 0x4c, 0x50, 0x31, 0, 0, 0, 0})
	for _, broken := range brokenTexts() {
		add(broken.source, "ReadMDL", []byte(broken.source))
		add(broken.source, "Paths", []byte(broken.source))
	}
	for _, source := range textsThatAreNoModel() {
		add(source, "ReadMDL", []byte(source))
		add(source, "Paths", []byte(source))
	}
	whole := modelWithEveryChunk()
	for _, at := range sizeOffsets(t, whole) {
		size := binary.LittleEndian.Uint32(whole[at:])
		for _, changed := range []uint32{0, 4, 95, 0x7FFFFFFF, 0x80000000, 0xFFFFFFFF, size - 1, size + 1} {
			add(fmt.Sprintf("the size at %d set to %d", at, changed), "ReadMDX", testkit.SetU32(whole, at, changed))
		}
	}
	return said
}

// TestRefusalsAreAsRecorded holds what the readers say of every model of refusals to the recording: the file,
// the message and the hint of each error, whole. The tests of an error beside it ask for its file, its
// distinguishing words and a hint; the words themselves are held here, where a change of them is one line of a
// diff.
func TestRefusalsAreAsRecorded(t *testing.T) {
	said := refusals(t, func(reader string, data []byte) error {
		var err error
		switch reader {
		case "ReadMDX":
			_, err = model.ReadMDX(data, modelFile)
		case "ReadMDL":
			_, err = model.ReadMDL(string(data), modelFile)
		default:
			_, err = model.Paths(data, modelFile)
		}
		return err
	}, testkit.RefusalOf)
	if len(said) < 150 {
		t.Errorf("only %d models are refused", len(said))
	}
	testkit.Recorded(t, "refusals.txt", testkit.Refusals(said))
}
