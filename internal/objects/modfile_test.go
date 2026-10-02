package objects_test

import (
	"bytes"
	"encoding/binary"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

const noEnd = "\x00\x00\x00\x00"

// The fixture README's table: one custom object per tab, its name as a TRIGSTR reference.
var fixtureNames = []struct {
	ext, base, id, field string
	skin                 bool
	level                int32
	value                string
}{
	{"w3u", "hpea", "h000", "unam", true, 0, "TRIGSTR_012"},
	{"w3t", "ratf", "I000", "unam", true, 0, "TRIGSTR_013"},
	{"w3b", "DTrf", "B000", "bnam", true, 0, "TRIGSTR_014"},
	{"w3d", "UObb", "D000", "dnam", false, 0, "TRIGSTR_015"},
	{"w3a", "ANab", "A000", "anam", true, 0, "TRIGSTR_016"},
	{"w3h", "BNab", "B000", "fnam", true, 0, "TRIGSTR_017"},
	{"w3q", "Rhme", "R000", "gnam", true, 1, "TRIGSTR_018"},
}

func int32Bytes(n int32) []byte { return binary.LittleEndian.AppendUint32(nil, uint32(n)) }

func readMod(t *testing.T, data []byte, kind objects.TableKind, file string) *objects.ModFile {
	t.Helper()
	parsed, err := objects.ReadModFile(data, kind, file)
	if err != nil {
		t.Fatalf("ReadModFile(%s): %v", file, err)
	}
	return parsed
}

func expectFileError(t *testing.T, data []byte, problem string) {
	t.Helper()
	_, err := objects.ReadModFile(data, objects.Simple, "map/war3map.w3u")
	e := asError(t, err, problem)
	if e.File != "map/war3map.w3u" || !strings.Contains(e.Msg, problem) || !strings.Contains(e.Hint, "World Editor 3.00") {
		t.Errorf("error = %+v, want %q", e, problem)
	}
}

func TestTableKindIsLeveledForW3aW3dAndW3q(t *testing.T) {
	for name, want := range map[string]objects.TableKind{
		"war3map.w3u": objects.Simple, "war3mapSkin.W3T": objects.Simple, "war3map.w3b": objects.Simple,
		"war3map.w3h": objects.Simple, "war3map.w3a": objects.Leveled, "war3mapSkin.W3D": objects.Leveled,
		"map/war3map.w3q": objects.Leveled,
	} {
		if got := objects.KindOf(name); got != want {
			t.Errorf("KindOf(%s) = %v", name, got)
		}
	}
}

func TestEveryWorldEditorNamesFileParsesAsTheFixtureREADMERecords(t *testing.T) {
	for _, name := range fixtureNames {
		for _, skin := range []bool{false, true} {
			file := "war3map." + name.ext
			if skin {
				file = "war3mapSkin." + name.ext
			}
			data := fixture(t, file)
			parsed := readMod(t, data, objects.KindOf(file), file)
			if parsed.Version != 3 || parsed.Original.CountOffset != 4 || parsed.Original.Start != 8 ||
				parsed.Original.Stop != 8 || len(parsed.Original.Objects) != 0 {
				t.Errorf("%s: version %d, original %+v", file, parsed.Version, parsed.Original)
			}
			custom := parsed.Custom
			if custom.CountOffset != 8 || custom.Start != 12 || custom.Stop != len(data) || len(custom.Objects) != 1 {
				t.Fatalf("%s: custom %+v", file, custom)
			}
			object := custom.Objects[0]
			if object.Base != name.base || object.ID != name.id || object.Start != 12 || object.Stop != len(data) ||
				len(object.Sets) != 1 || object.Sets[0].Flag != 0 {
				t.Errorf("%s: object %+v", file, object)
			}
			mods := object.Sets[0].Mods
			if skin != name.skin {
				if len(mods) != 0 {
					t.Errorf("%s has modifications %+v", file, mods)
				}
				continue
			}
			want := []objects.Modification{{
				Field: name.field, Level: name.level, Value: objects.TextValue(name.value), End: noEnd,
				Start: 32, Stop: len(data),
			}}
			if !reflect.DeepEqual(mods, want) {
				t.Errorf("%s: mods %+v, want %+v", file, mods, want)
			}
		}
	}
}

func TestSyntheticFilesRoundTripIntRealUnrealAndStringValues(t *testing.T) {
	custom := []testkit.SyntheticObject{{
		Base: "hfoo", ID: "h001",
		Sets: []testkit.SyntheticSet{
			{Mods: []testkit.SyntheticMod{
				{Field: "uhpm", Value: objects.IntValue(-250)},
				{Field: "umvs", Value: objects.RealValue("real", 0.1)},
				{Field: "ucbs", Value: objects.RealValue("unreal", 1.3)},
				{Field: "unam", Value: objects.TextValue("Møønwell"), End: "h001"},
			}},
			{Flag: 7, Mods: []testkit.SyntheticMod{{Field: "utip", Value: objects.TextValue("")}}},
		},
	}}
	original := []testkit.SyntheticObject{{
		Base: "hpea", ID: noEnd, Mods: []testkit.SyntheticMod{{Field: "ugol", Value: objects.IntValue(2147483647)}},
	}}
	for _, kind := range []objects.TableKind{objects.Simple, objects.Leveled} {
		data := testkit.BuildModFile(3, original, custom, kind)
		parsed := readMod(t, data, kind, "war3map.w3u")
		object := parsed.Custom.Objects[0]
		type value struct {
			field string
			value objects.ModValue
			end   string
		}
		var got [][]value
		for _, set := range object.Sets {
			var values []value
			for _, mod := range set.Mods {
				values = append(values, value{mod.Field, mod.Value, mod.End})
			}
			got = append(got, values)
		}
		want := [][]value{
			{
				{"uhpm", objects.IntValue(-250), noEnd},
				{"umvs", objects.RealValue("real", float64(float32(0.1))), noEnd},
				{"ucbs", objects.RealValue("unreal", float64(float32(1.3))), noEnd},
				{"unam", objects.TextValue("Møønwell"), "h001"},
			},
			{{"utip", objects.TextValue(""), noEnd}},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("values = %+v", got)
		}
		if object.Sets[0].Flag != 0 || object.Sets[1].Flag != 7 || parsed.Original.Objects[0].ID != noEnd ||
			parsed.Original.Objects[0].Sets[0].Mods[0].Value != objects.IntValue(2147483647) ||
			parsed.Custom.Stop != len(data) || object.Stop != len(data) {
			t.Errorf("parsed = %+v", parsed)
		}
		// The first custom modification follows base, id, set count, set flag and modification count.
		first := object.Sets[0].Mods[0]
		size := 16
		if kind == objects.Leveled {
			size = 24
		}
		if first.Start != parsed.Custom.Start+20 || first.Stop-first.Start != size {
			t.Errorf("the first modification spans %d to %d", first.Start, first.Stop)
		}
	}
	leveled := testkit.BuildModFile(3, nil, []testkit.SyntheticObject{{
		Base: "AHbz", ID: "A001",
		Mods: []testkit.SyntheticMod{{Field: "Hbz1", Level: 3, Column: 1, Value: objects.RealValue("unreal", 2.5)}},
	}}, objects.Leveled)
	mod := readMod(t, leveled, objects.Leveled, "war3map.w3a").Custom.Objects[0].Sets[0].Mods[0]
	if mod.Field != "Hbz1" || mod.Level != 3 || mod.Column != 1 || mod.Value != objects.RealValue("unreal", 2.5) {
		t.Errorf("mod = %+v", mod)
	}
}

func TestSyntheticV1AndV2FilesWithoutSetFieldsParseWithOneImplicitSet(t *testing.T) {
	for _, version := range []int32{1, 2} {
		for _, kind := range []objects.TableKind{objects.Simple, objects.Leveled} {
			data := testkit.BuildModFile(version,
				[]testkit.SyntheticObject{{Base: "hpea", ID: noEnd, Mods: []testkit.SyntheticMod{{Field: "ugol", Value: objects.IntValue(90)}}}},
				[]testkit.SyntheticObject{
					{Base: "hfoo", ID: "h001", Mods: []testkit.SyntheticMod{{Field: "unam", Level: 2, Value: objects.TextValue("A")}}},
					{Base: "hfoo", ID: "h002"},
				}, kind)
			parsed := readMod(t, data, kind, "war3map.w3u")
			stop, level := 36, int32(0)
			if kind == objects.Leveled {
				stop, level = 44, 2
			}
			want := []objects.ModSet{{Mods: []objects.Modification{{
				Field: "ugol", Value: objects.IntValue(90), End: noEnd, Start: 20, Stop: stop,
			}}}}
			if parsed.Version != version || !reflect.DeepEqual(parsed.Original.Objects[0].Sets, want) {
				t.Errorf("v%d: original %+v", version, parsed.Original.Objects[0].Sets)
			}
			custom := parsed.Custom.Objects
			if len(custom) != 2 || custom[0].ID != "h001" || custom[1].ID != "h002" || len(custom[0].Sets) != 1 ||
				len(custom[1].Sets) != 1 || custom[0].Sets[0].Flag != 0 || custom[0].Sets[0].Mods[0].Level != level ||
				len(custom[1].Sets[0].Mods) != 0 || custom[1].Stop != len(data) {
				t.Errorf("v%d: custom %+v", version, custom)
			}
		}
	}
}

func TestMalformedFilesAreFileErrorsThatPointToWorldEditor300(t *testing.T) {
	valid := fixture(t, "war3mapSkin.w3u")
	with := func(offset int, value int32) []byte {
		return slices.Concat(valid[:offset], int32Bytes(value), valid[offset+4:])
	}
	expectFileError(t, with(0, 0), "unsupported version 0")
	expectFileError(t, with(0, 4), "unsupported version 4")
	expectFileError(t, valid[:2], "truncated")
	expectFileError(t, valid[:6], "truncated")
	for length := 9; length < len(valid); length++ {
		expectFileError(t, valid[:length], "")
	}
	expectFileError(t, with(8, 1000), "count")
	expectFileError(t, with(8, -1), "count")
	expectFileError(t, with(28, 0x7fffffff), "count")
	expectFileError(t, with(36, 4), "unknown value type 4")
	// Replaces the string's NUL and the end token with text, so no NUL remains after the string starts.
	expectFileError(t, slices.Concat(valid[:51], []byte("xxxxx")), "unterminated string")
	invalidUTF8 := bytes.Clone(valid)
	invalidUTF8[40] = 0xff
	expectFileError(t, invalidUTF8, "invalid UTF-8")
	expectFileError(t, append(bytes.Clone(valid), 0), "trailing bytes")
	expectFileError(t, with(20, 0), "set count 0")
	expectFileError(t, with(20, 1000), "set count 1000")
	expectFileError(t, with(20, -1), "set count -1")
}

func TestASliceOfALargerBufferParsesTheSameAsACopy(t *testing.T) {
	for _, file := range []string{"war3mapSkin.w3q", "war3map.w3d", "war3mapSkin.w3u"} {
		data := fixture(t, file)
		padded := slices.Concat(make([]byte, 3), data, make([]byte, 4))
		view := readMod(t, padded[3:3+len(data)], objects.KindOf(file), file)
		if !reflect.DeepEqual(view, readMod(t, data, objects.KindOf(file), file)) {
			t.Errorf("%s parses differently as a slice", file)
		}
	}
}

var appended = []objects.NewObject{
	{Base: "hfoo", ID: "X001", Mods: []objects.NewMod{
		{Field: "uhpm", Value: objects.IntValue(-250)},
		{Field: "umvs", Level: 2, Column: 1, Value: objects.RealValue("real", 0.1)},
		{Field: "ucbs", Column: 3, Value: objects.RealValue("unreal", 1.3)},
		{Field: "unam", Level: 1, Value: objects.TextValue("Møønwell")},
	}},
	{Base: "hpea", ID: "X002", Mods: []objects.NewMod{}},
}

// appendedFor is appended as the table kind allows it: simple tables have no level or column.
func appendedFor(kind objects.TableKind) []objects.NewObject {
	var out []objects.NewObject
	for _, object := range appended {
		copied := objects.NewObject{Base: object.Base, ID: object.ID}
		for _, mod := range object.Mods {
			if kind == objects.Simple {
				mod.Level, mod.Column = 0, 0
			}
			copied.Mods = append(copied.Mods, mod)
		}
		out = append(out, copied)
	}
	return out
}

// asSynthetic is what a file holds for the appended objects, as the independent builder writes them.
func asSynthetic(added []objects.NewObject) []testkit.SyntheticObject {
	var out []testkit.SyntheticObject
	for _, object := range added {
		synthetic := testkit.SyntheticObject{Base: object.Base, ID: object.ID, Mods: []testkit.SyntheticMod{}}
		for _, mod := range object.Mods {
			synthetic.Mods = append(synthetic.Mods, testkit.SyntheticMod{
				Field: mod.Field, Level: int32(mod.Level), Column: int32(mod.Column), Value: mod.Value,
			})
		}
		out = append(out, synthetic)
	}
	return out
}

func appendObjects(t *testing.T, source []byte, kind objects.TableKind, added []objects.NewObject, file string) []byte {
	t.Helper()
	data, err := objects.AppendObjects(source, kind, added, file)
	if err != nil {
		t.Fatalf("AppendObjects(%s): %v", file, err)
	}
	return data
}

func TestAppendingEachNamesFixtureObjectToNoFileReproducesWorldEditorsFiles(t *testing.T) {
	for _, name := range fixtureNames {
		for _, skin := range []bool{false, true} {
			file := "war3map." + name.ext
			if skin {
				file = "war3mapSkin." + name.ext
			}
			mods := []objects.NewMod{}
			if skin == name.skin {
				mods = append(mods, objects.NewMod{Field: name.field, Level: int64(name.level), Value: objects.TextValue(name.value)})
			}
			added := []objects.NewObject{{Base: name.base, ID: name.id, Mods: mods}}
			if got := appendObjects(t, nil, objects.KindOf(file), added, file); !bytes.Equal(got, fixture(t, file)) {
				t.Errorf("%s is not World Editor's file", file)
			}
		}
	}
}

func TestAppendingToEveryNamesFixtureFileKeepsItsBytesAndAddsTheObjectsLast(t *testing.T) {
	for _, name := range fixtureNames {
		for _, prefix := range []string{"war3map", "war3mapSkin"} {
			file := prefix + "." + name.ext
			kind := objects.KindOf(file)
			source := fixture(t, file)
			before := readMod(t, source, kind, file)
			data := appendObjects(t, source, kind, appendedFor(kind), file)
			custom := before.Custom
			if !bytes.Equal(data[:custom.CountOffset], source[:custom.CountOffset]) ||
				!bytes.Equal(data[custom.CountOffset:custom.Start], int32Bytes(int32(len(custom.Objects)+2))) ||
				!bytes.Equal(data[custom.Start:custom.Stop], source[custom.Start:custom.Stop]) {
				t.Errorf("%s: the existing bytes changed", file)
			}
			after := readMod(t, data, kind, file)
			count := len(after.Custom.Objects)
			if after.Version != 3 || !reflect.DeepEqual(after.Original, before.Original) ||
				!reflect.DeepEqual(after.Custom.Objects[:count-2], before.Custom.Objects) || after.Custom.Stop != len(data) {
				t.Errorf("%s: the file reads differently after appending", file)
			}
			// The appended objects read back as an independently built file holds them.
			expected := testkit.BuildModFile(3, nil, asSynthetic(appendedFor(kind)), kind)
			if !bytes.Equal(data[custom.Stop:], expected[12:]) {
				t.Errorf("%s: the appended objects are not encoded as expected", file)
			}
		}
	}
}

func TestAppendingNoObjectsReturnsTheSourceBytes(t *testing.T) {
	for _, file := range []string{"war3map.w3u", "war3mapSkin.w3q", "war3map.w3d"} {
		source := fixture(t, file)
		if got := appendObjects(t, source, objects.KindOf(file), nil, file); !bytes.Equal(got, source) {
			t.Errorf("%s changed", file)
		}
	}
}

func TestSyntheticV1V2AndV3SourcesGetObjectsInTheirOwnVersionsShape(t *testing.T) {
	original := []testkit.SyntheticObject{{
		Base: "hpea", ID: noEnd, Mods: []testkit.SyntheticMod{{Field: "ugol", Value: objects.IntValue(90)}},
	}}
	existing := []testkit.SyntheticObject{
		{Base: "hfoo", ID: "h001", Mods: []testkit.SyntheticMod{{Field: "unam", Level: 2, Value: objects.TextValue("A")}}},
		{Base: "hfoo", ID: "h002", Mods: []testkit.SyntheticMod{{Field: "utip", Value: objects.TextValue("B"), End: "h002"}}},
	}
	// A v3 object with two sets and a nonzero flag, which Moonwell never writes, must still be copied verbatim.
	multiSet := testkit.SyntheticObject{Base: "hfoo", ID: "h003", Sets: []testkit.SyntheticSet{
		{Flag: 7},
		{Mods: []testkit.SyntheticMod{{Field: "uhpm", Value: objects.IntValue(5)}}},
	}}
	for _, version := range []int32{1, 2, 3} {
		for _, kind := range []objects.TableKind{objects.Simple, objects.Leveled} {
			custom := existing
			if version >= 3 {
				custom = append(slices.Clone(existing), multiSet)
			}
			source := testkit.BuildModFile(version, original, custom, kind)
			added := appendedFor(kind)
			want := testkit.BuildModFile(version, original, append(slices.Clone(custom), asSynthetic(added)...), kind)
			if got := appendObjects(t, source, kind, added, "war3map.w3u"); !bytes.Equal(got, want) {
				t.Errorf("v%d, kind %v: the appended file differs", version, kind)
			}
		}
	}
}

func TestAppendingToAMalformedSourceIsTheReadersFileError(t *testing.T) {
	valid := fixture(t, "war3mapSkin.w3u")
	_, err := objects.AppendObjects(valid[:20], objects.Simple, appendedFor(objects.Simple), "map/war3mapSkin.w3u")
	if e := asError(t, err, "a cut-off source"); e.File != "map/war3mapSkin.w3u" {
		t.Errorf("error = %+v", e)
	}
}

func TestValuesTheResolverShouldHaveRejectedAreInternalErrors(t *testing.T) {
	appendOne := func(object objects.NewObject) error {
		_, err := objects.AppendObjects(nil, objects.Leveled, []objects.NewObject{object}, "war3map.w3a")
		return err
	}
	withValue := func(value objects.ModValue) objects.NewObject {
		return objects.NewObject{Base: "hfoo", ID: "X001", Mods: []objects.NewMod{{Field: "unam", Value: value}}}
	}
	internal := func(err error, problem string) {
		t.Helper()
		if _, isUserError := diag.First(err); err == nil || isUserError || !strings.Contains(err.Error(), problem) {
			t.Errorf("error = %v, want an internal error about %s", err, problem)
		}
	}
	internal(appendOne(withValue(objects.TextValue("a\x00b"))), "NUL")
	internal(appendOne(withValue(objects.TextValue("a\xffb"))), "surrogate")
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), 3.5e38, -1e39} {
		internal(appendOne(withValue(objects.RealValue("real", value))), "float32")
		internal(appendOne(withValue(objects.RealValue("unreal", value))), "float32")
	}
	for _, value := range []float64{2147483648, -2147483649, 1.5, math.NaN()} {
		internal(appendOne(withValue(objects.ModValue{Type: "int", Number: value})), "int32")
	}
	internal(appendOne(objects.NewObject{Base: "hfoo", ID: "X001", Mods: []objects.NewMod{
		{Field: "unam", Level: 1 << 31, Value: objects.IntValue(1)},
	}}), "int32")
	for _, id := range []string{"X01", "X0001", "X00€", ""} {
		internal(appendOne(objects.NewObject{Base: "hfoo", ID: id}), "object id")
		internal(appendOne(objects.NewObject{Base: id, ID: "X001"}), "object id")
		internal(appendOne(objects.NewObject{Base: "hfoo", ID: "X001", Mods: []objects.NewMod{
			{Field: id, Value: objects.IntValue(1)},
		}}), "object id")
	}
	for _, mod := range []objects.NewMod{
		{Field: "unam", Level: 1, Value: objects.IntValue(1)},
		{Field: "unam", Column: 1, Value: objects.IntValue(1)},
	} {
		_, err := objects.AppendObjects(nil, objects.Simple,
			[]objects.NewObject{{Base: "hfoo", ID: "X001", Mods: []objects.NewMod{mod}}}, "war3map.w3u")
		internal(err, "simple table")
	}
	// The largest finite float32 and Latin-1 ids are accepted.
	latin := objects.NewObject{Base: "hfoo", ID: "\x00ÿéA", Mods: []objects.NewMod{
		{Field: "unam", Value: objects.RealValue("real", -3.4028234663852886e38)},
	}}
	data, err := objects.AppendObjects(nil, objects.Leveled, []objects.NewObject{latin}, "war3map.w3a")
	if err != nil || !bytes.Equal(data[16:20], []byte{0, 0xff, 0xe9, 'A'}) {
		t.Errorf("a Latin-1 id: % X, %v", data[16:20], err)
	}
}
