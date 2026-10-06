package objmod_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/war3/objmod"
)

// unwritableObjects is objects that Append refuses, with a name for the recording.
type unwritableObjects struct {
	name    string
	kind    objmod.TableKind
	objects []objmod.NewObject
}

// whatAppendRefuses is one list of objects for each thing Append cannot write, and for a text and a real each
// way of being one.
func whatAppendRefuses() []unwritableObjects {
	mod := func(field string, value objmod.Value) []objmod.NewObject {
		return oneMod(objmod.NewMod{Field: id(field), Value: value})
	}
	real32 := func(value float64) objmod.Value { return realValue(float32(value)) }
	return []unwritableObjects{
		{"a base of four NULs", objmod.Simple, []objmod.NewObject{{ID: id("X001")}}},
		{"an id of four NULs", objmod.Simple, []objmod.NewObject{{Base: id("hfoo")}}},
		{"a field of four NULs", objmod.Simple, oneMod(objmod.NewMod{Value: intValue(1)})},
		{"a value type of 7", objmod.Simple, mod("unam", objmod.Value{Type: 7})},
		{"a value type of -1", objmod.Leveled, mod("unam", objmod.Value{Type: -1})},
		{"a level and a column in a simple table", objmod.Simple,
			oneMod(objmod.NewMod{Field: id("unam"), Level: 2, Column: 3, Value: intValue(1)})},
		{"a column in a simple table", objmod.Simple,
			oneMod(objmod.NewMod{Field: id("unam"), Column: 1, Value: intValue(1)})},
		{"a text with a NUL", objmod.Leveled, mod("unam", textValue("a\x00b"))},
		{"a text that is not UTF-8", objmod.Leveled, mod("unam", textValue("a\xFFb"))},
		{"a text of a NUL in a simple table", objmod.Simple, mod("unam", textValue("\x00"))},
		{"a real of minus infinity", objmod.Leveled, mod("uhpm", real32(math.Inf(-1)))},
		{"a real of infinity", objmod.Simple, mod("uhpm", real32(math.Inf(1)))},
		{"an unreal that is not a number", objmod.Leveled, mod("uhpm", unrealValue(float32(math.NaN())))},
	}
}

// writers are the two doors of the package, each as the error it returns, and how an error of theirs is
// written into a recording.
type writers struct {
	read     func(data []byte, kind objmod.TableKind) error
	appendTo func(refused unwritableObjects) error
	refusal  func(input string, err error) testkit.Refusal
}

// refusals is what the writers say of every input they refuse among these: the malformed files, as a simple and
// as a leveled table; two files of the test kit's making with each number that decides how the rest is read
// set to each number at an edge; and the objects Append cannot write. A file that is read is left out.
func refusals(t *testing.T, w writers) []testkit.Refusal {
	t.Helper()
	var said []testkit.Refusal
	read := func(name string, data []byte, kind objmod.TableKind) {
		if err := w.read(data, kind); err != nil {
			said = append(said, w.refusal(name, err))
		}
	}
	for _, file := range malformedFiles(t) {
		read(file.name, file.data, objmod.Simple)
		read(file.name+", as a leveled table", file.data, objmod.Leveled)
	}
	for _, file := range filesToDamage(t) {
		if file.name != "synthetic version 3, kind 0" && file.name != "synthetic version 1, kind 1" {
			continue
		}
		for _, at := range numberOffsets(t, file) {
			for _, number := range append(testkit.EdgeNumbers(), 3, 4, 5, 64, 65, 1000) {
				name := fmt.Sprintf("%s with the number at %d set to %d", file.name, at, number)
				read(name, testkit.SetU32(file.data, at, number), file.kind)
			}
		}
	}
	for _, refused := range whatAppendRefuses() {
		said = append(said, w.refusal("appended: "+refused.name, w.appendTo(refused)))
	}
	return said
}

// TestRefusalsAreAsRecorded holds what the package says of every input of refusals to the recording: the file,
// the message and the hint of each error, whole. The tests of an error beside it ask for its file, its
// distinguishing words and a hint; the words themselves are held here, where a change of them is one line of a
// diff.
func TestRefusalsAreAsRecorded(t *testing.T) {
	said := refusals(t, writers{
		read: func(data []byte, kind objmod.TableKind) error {
			_, err := objmod.Read(data, kind, modFile)
			return err
		},
		appendTo: func(refused unwritableObjects) error {
			_, err := objmod.Append(nil, refused.kind, refused.objects, modFile)
			if err == nil {
				t.Errorf("%s is appended", refused.name)
			}
			return err
		},
		refusal: testkit.RefusalOf,
	})
	if len(said) < 300 {
		t.Errorf("only %d inputs are refused", len(said))
	}
	testkit.Recorded(t, "refusals.txt", testkit.Refusals(said))
}
