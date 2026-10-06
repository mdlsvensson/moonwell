package objmod_test

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/war3/objmod"
)

// modFile is the name the tests give Read and Append for their errors.
const modFile = "map/war3map.w3u"

// moonwell is a text with characters that are not ASCII: an o with a stroke, twice.
const moonwell = "M\xC3\xB8\xC3\xB8nwell"

// nulls is the id of four NUL bytes as the test kit takes it: the id of a standard object in the original table.
const nulls = "\x00\x00\x00\x00"

// noEnd is the usual end token of a modification.
var noEnd = objmod.ID{}

// fixtureNames is the table of the fixture's README: one custom object per tab of the Object Editor, its name a
// TRIGSTR reference. skin says which of the two files holds the name: the skin file or the main one.
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

// fixtureFiles are the names of the fixture's fourteen files: the main and the skin file of every tab.
func fixtureFiles() []string {
	var files []string
	for _, name := range fixtureNames {
		files = append(files, "war3map."+name.ext, "war3mapSkin."+name.ext)
	}
	return files
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	return testkit.Fixture(t, "objects-v3-names/"+name)
}

// id is the ID a test wrote out as four characters.
func id(s string) objmod.ID {
	parsed, ok := objmod.ParseID(s)
	if !ok {
		panic(fmt.Sprintf("%q is not an id", s))
	}
	return parsed
}

func intValue(n int32) objmod.Value      { return objmod.Value{Type: objmod.Int, Int: n} }
func realValue(n float32) objmod.Value   { return objmod.Value{Type: objmod.Real, Real: n} }
func unrealValue(n float32) objmod.Value { return objmod.Value{Type: objmod.Unreal, Real: n} }
func textValue(s string) objmod.Value    { return objmod.Value{Type: objmod.String, Text: s} }

// mustRead reads a file the test knows to be whole.
func mustRead(t *testing.T, data []byte, kind objmod.TableKind, file string) *objmod.File {
	t.Helper()
	parsed, err := objmod.Read(data, kind, file)
	if err != nil {
		t.Fatalf("Read(%s): %v", file, err)
	}
	return parsed
}

// mustAppend appends objects the test knows can be written.
func mustAppend(t *testing.T, source []byte, kind objmod.TableKind, added []objmod.NewObject, file string) []byte {
	t.Helper()
	data, err := objmod.Append(source, kind, added, file)
	if err != nil {
		t.Fatalf("Append(%s): %v", file, err)
	}
	return data
}

// refusal checks that err is a file error that names the file, has the words and points to World Editor 3.00.
func refusal(t *testing.T, what string, err error, file, words string) {
	t.Helper()
	var failure *diag.Error
	if !errors.As(err, &failure) {
		t.Errorf("%s: got %v, want a *diag.Error", what, err)
		return
	}
	if failure.File != file || !strings.Contains(failure.Msg, words) {
		t.Errorf("%s: error %q of file %s, want file %s and a message with %q", what, failure.Msg, failure.File, file, words)
	}
	if !strings.Contains(failure.Hint, "World Editor 3.00") {
		t.Errorf("%s: hint %q, want one that points to World Editor 3.00", what, failure.Hint)
	}
}

func TestParseIDTakesFourCharactersOfOneByteEach(t *testing.T) {
	for _, c := range []struct {
		name, text string
		want       objmod.ID
		ok         bool
	}{
		{"four letters", "hfoo", objmod.ID{'h', 'f', 'o', 'o'}, true},
		{"NUL and characters above ASCII", "\x00\xC3\xBF\xC3\xA9A", objmod.ID{0, 0xFF, 0xE9, 'A'}, true},
		{"four NULs", nulls, objmod.ID{}, true},
		{"three characters", "X01", objmod.ID{}, false},
		{"five characters", "X0001", objmod.ID{}, false},
		{"no characters", "", objmod.ID{}, false},
		{"a euro sign, which is above U+00FF", "X00\xE2\x82\xAC", objmod.ID{}, false},
		{"U+0100, the first character that is too large", "X00\xC4\x80", objmod.ID{}, false},
		{"a byte that is not UTF-8", "X00\xFF", objmod.ID{}, false},
		{"four bytes that are two characters", "\xC3\xBF\xC3\xA9", objmod.ID{}, false},
	} {
		if got, ok := objmod.ParseID(c.text); got != c.want || ok != c.ok {
			t.Errorf("%s: ParseID(%q) = %v, %v, want %v, %v", c.name, c.text, got, ok, c.want, c.ok)
		}
	}
}

func TestAnIDIsWrittenOutAsOneCharacterPerByte(t *testing.T) {
	if got := (objmod.ID{0, 0xFF, 0xE9, 'A'}).String(); got != "\x00\xC3\xBF\xC3\xA9A" {
		t.Errorf("String() = %q", got)
	}
	for b := range 256 {
		written := objmod.ID{byte(b), 'a', byte(255 - b), byte(b)}
		if back, ok := objmod.ParseID(written.String()); !ok || back != written {
			t.Errorf("%v is written %q, which parses as %v, %v", written, written.String(), back, ok)
		}
	}
}

func TestTableKindIsLeveledForW3aW3dAndW3q(t *testing.T) {
	for name, want := range map[string]objmod.TableKind{
		"war3map.w3u": objmod.Simple, "war3mapSkin.W3T": objmod.Simple, "war3map.w3b": objmod.Simple,
		"war3map.w3h": objmod.Simple, "war3map.w3a": objmod.Leveled, "war3mapSkin.W3D": objmod.Leveled,
		"map/war3map.w3q": objmod.Leveled, "w3a": objmod.Simple, "war3map.w3a.bak": objmod.Simple,
	} {
		if got := objmod.KindOf(name); got != want {
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
			parsed := mustRead(t, data, objmod.KindOf(file), file)
			if parsed.Version != 3 || parsed.Original.CountOffset != 4 || parsed.Original.Start != 8 ||
				parsed.Original.Stop != 8 || len(parsed.Original.Objects) != 0 {
				t.Errorf("%s: version %d, original %+v", file, parsed.Version, parsed.Original)
			}
			custom := parsed.Custom
			if custom.CountOffset != 8 || custom.Start != 12 || custom.Stop != len(data) || len(custom.Objects) != 1 {
				t.Fatalf("%s: custom %+v", file, custom)
			}
			object := custom.Objects[0]
			if object.Base != id(name.base) || object.ID != id(name.id) || object.Start != 12 ||
				object.Stop != len(data) || len(object.Sets) != 1 || object.Sets[0].Flag != 0 {
				t.Errorf("%s: object %+v", file, object)
			}
			mods := object.Sets[0].Mods
			if skin != name.skin {
				if len(mods) != 0 {
					t.Errorf("%s has modifications %+v", file, mods)
				}
				continue
			}
			want := []objmod.Modification{{
				Field: id(name.field), Level: name.level, Value: textValue(name.value), End: noEnd,
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
				{Field: "uhpm", Value: intValue(-250)},
				{Field: "umvs", Value: realValue(0.1)},
				{Field: "ucbs", Value: unrealValue(1.3)},
				{Field: "unam", Value: textValue(moonwell), End: "h001"},
			}},
			{Flag: 7, Mods: []testkit.SyntheticMod{{Field: "utip", Value: textValue("")}}},
		},
	}}
	original := []testkit.SyntheticObject{{
		Base: "hpea", ID: nulls, Mods: []testkit.SyntheticMod{{Field: "ugol", Value: intValue(math.MaxInt32)}},
	}}
	type value struct {
		field objmod.ID
		value objmod.Value
		end   objmod.ID
	}
	want := [][]value{
		{
			{id("uhpm"), intValue(-250), noEnd},
			{id("umvs"), realValue(0.1), noEnd},
			{id("ucbs"), unrealValue(1.3), noEnd},
			{id("unam"), textValue(moonwell), id("h001")},
		},
		{{id("utip"), textValue(""), noEnd}},
	}
	for _, kind := range []objmod.TableKind{objmod.Simple, objmod.Leveled} {
		data := testkit.BuildModFile(3, original, custom, kind)
		parsed := mustRead(t, data, kind, "war3map.w3u")
		object := parsed.Custom.Objects[0]
		var got [][]value
		for _, set := range object.Sets {
			var values []value
			for _, mod := range set.Mods {
				values = append(values, value{mod.Field, mod.Value, mod.End})
			}
			got = append(got, values)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("values = %+v", got)
		}
		if object.Sets[0].Flag != 0 || object.Sets[1].Flag != 7 || parsed.Original.Objects[0].ID != noEnd ||
			parsed.Original.Objects[0].Sets[0].Mods[0].Value != intValue(math.MaxInt32) ||
			parsed.Custom.Stop != len(data) || object.Stop != len(data) {
			t.Errorf("parsed = %+v", parsed)
		}
		// The first custom modification follows base, id, set count, set flag and modification count.
		first := object.Sets[0].Mods[0]
		size := 16
		if kind == objmod.Leveled {
			size = 24
		}
		if first.Start != parsed.Custom.Start+20 || first.Stop-first.Start != size {
			t.Errorf("the first modification spans %d to %d", first.Start, first.Stop)
		}
	}
	leveled := testkit.BuildModFile(3, nil, []testkit.SyntheticObject{{
		Base: "AHbz", ID: "A001",
		Mods: []testkit.SyntheticMod{{Field: "Hbz1", Level: 3, Column: 1, Value: unrealValue(2.5)}},
	}}, objmod.Leveled)
	mod := mustRead(t, leveled, objmod.Leveled, "war3map.w3a").Custom.Objects[0].Sets[0].Mods[0]
	if mod.Field != id("Hbz1") || mod.Level != 3 || mod.Column != 1 || mod.Value != unrealValue(2.5) {
		t.Errorf("mod = %+v", mod)
	}
}

func TestSyntheticV1AndV2FilesWithoutSetFieldsParseWithOneImplicitSet(t *testing.T) {
	for _, version := range []int32{1, 2} {
		for _, kind := range []objmod.TableKind{objmod.Simple, objmod.Leveled} {
			original := []testkit.SyntheticObject{
				{Base: "hpea", ID: nulls, Mods: []testkit.SyntheticMod{{Field: "ugol", Value: intValue(90)}}},
			}
			data := testkit.BuildModFile(version, original, []testkit.SyntheticObject{
				{Base: "hfoo", ID: "h001", Mods: []testkit.SyntheticMod{{Field: "unam", Level: 2, Value: textValue("A")}}},
				{Base: "hfoo", ID: "h002"},
			}, kind)
			parsed := mustRead(t, data, kind, "war3map.w3u")
			stop, level := 36, int32(0)
			if kind == objmod.Leveled {
				stop, level = 44, 2
			}
			want := []objmod.Set{{Mods: []objmod.Modification{{
				Field: id("ugol"), Value: intValue(90), End: noEnd, Start: 20, Stop: stop,
			}}}}
			if parsed.Version != version || !reflect.DeepEqual(parsed.Original.Objects[0].Sets, want) {
				t.Errorf("v%d: original %+v", version, parsed.Original.Objects[0].Sets)
			}
			custom := parsed.Custom.Objects
			if len(custom) != 2 || custom[0].ID != id("h001") || custom[1].ID != id("h002") || len(custom[0].Sets) != 1 ||
				len(custom[1].Sets) != 1 || custom[0].Sets[0].Flag != 0 || custom[0].Sets[0].Mods[0].Level != level ||
				len(custom[1].Sets[0].Mods) != 0 || custom[1].Stop != len(data) {
				t.Errorf("v%d: custom %+v", version, custom)
			}
		}
	}
}

// malformed is a simple table that does not read, and the words its error has.
type malformed struct {
	name  string
	data  []byte
	words string
}

// malformedFiles returns the skin file of the units with one thing wrong in each, and cut at every length after
// its counts.
func malformedFiles(t *testing.T) []malformed {
	t.Helper()
	valid := fixture(t, "war3mapSkin.w3u")
	with := func(offset int, value int32) []byte { return testkit.SetU32(valid, offset, uint32(value)) }
	invalidUTF8 := bytes.Clone(valid)
	invalidUTF8[40] = 0xFF
	cases := []malformed{
		{"version 0", with(0, 0), "unsupported version 0"},
		{"version 4", with(0, 4), "unsupported version 4"},
		{"cut inside the version", valid[:2], "truncated"},
		{"cut inside the count of original objects", valid[:6], "truncated"},
		{"a count of 1000 custom objects", with(8, 1000), "object count 1000"},
		{"a count of -1 custom objects", with(8, -1), "object count -1"},
		{"a count of modifications past the end", with(28, math.MaxInt32), "modification count"},
		{"value type 4", with(36, 4), "unknown value type 4"},
		// The string's NUL and the end token become text, so no NUL is left after the string starts.
		{"a string without its NUL", testkit.Concat(valid[:51], []byte("xxxxx")), "unterminated string"},
		{"a string that is not UTF-8", invalidUTF8, "invalid UTF-8"},
		{"a byte after the custom objects", append(bytes.Clone(valid), 0), "trailing bytes"},
		{"a set count of 0", with(20, 0), "set count 0"},
		{"a set count of 1000", with(20, 1000), "set count 1000"},
		{"a set count of -1", with(20, -1), "set count -1"},
	}
	for length := 9; length < len(valid); length++ {
		cases = append(cases, malformed{fmt.Sprintf("cut at %d bytes", length), valid[:length:length], ""})
	}
	return cases
}

func TestMalformedFilesAreFileErrorsThatPointToWorldEditor300(t *testing.T) {
	for _, c := range malformedFiles(t) {
		parsed, err := objmod.Read(c.data, objmod.Simple, modFile)
		refusal(t, c.name, err, modFile, c.words)
		if parsed != nil {
			t.Errorf("%s: a refused file returned %+v", c.name, parsed)
		}
	}
}

func TestARefusalSaysWhatIsWrongWithTheFileAndToSaveTheMapAgain(t *testing.T) {
	// The malformed files by their names, each with the whole of what its error says is wrong.
	problems := map[string]string{}
	for _, pair := range [][2]string{
		{"version 0", "unsupported version 0"},
		{"cut inside the version", "truncated file"},
		{"cut inside the count of original objects", "truncated file"},
		{"a count of 1000 custom objects", "object count 1000 past end of file"},
		{"a count of -1 custom objects", "object count -1 past end of file"},
		{"a count of modifications past the end", "modification count 2147483647 past end of file"},
		{"value type 4", "unknown value type 4"},
		{"a string without its NUL", "unterminated string"},
		{"a string that is not UTF-8", "invalid UTF-8 in a string"},
		{"a byte after the custom objects", "trailing bytes after the custom objects"},
		{"a set count of 0", "unsupported set count 0"},
		{"a set count of -1", "unsupported set count -1"},
	} {
		problems[pair[0]] = pair[1]
	}
	found := 0
	for _, c := range malformedFiles(t) {
		problem, listed := problems[c.name]
		if !listed {
			continue
		}
		found++
		_, err := objmod.Read(c.data, objmod.Simple, modFile)
		want := diag.Error{
			Msg:  "Cannot read object data: " + problem + ".",
			File: modFile,
			Hint: "Open and re-save this map in World Editor 3.00.",
		}
		var failure *diag.Error
		if !errors.As(err, &failure) || *failure != want {
			t.Errorf("%s: got %+v, want %+v", c.name, err, want)
		}
	}
	if found != len(problems) {
		t.Errorf("%d of the %d files were found among the malformed ones", found, len(problems))
	}
}

func TestAppendSaysWhatItCannotWrite(t *testing.T) {
	mod := func(field string, value objmod.Value) objmod.NewMod {
		return objmod.NewMod{Field: id(field), Value: value}
	}
	for _, c := range []struct {
		kind    objmod.TableKind
		objects []objmod.NewObject
		words   string
	}{
		{objmod.Simple, []objmod.NewObject{{ID: id("X001")}},
			`Cannot write the object "X001": its base is four NUL bytes.`},
		{objmod.Simple, []objmod.NewObject{{Base: id("hfoo")}},
			`Cannot write an object based on "hfoo": its id is four NUL bytes.`},
		{objmod.Simple, oneMod(objmod.NewMod{Value: intValue(1)}),
			`Cannot write a modification of "X001": its field is four NUL bytes.`},
		{objmod.Simple, oneMod(mod("unam", objmod.Value{Type: 7})),
			"Cannot write unam: its value type 7 is none of the four."},
		{objmod.Simple, oneMod(objmod.NewMod{Field: id("unam"), Level: 2, Column: 3, Value: intValue(1)}),
			"Cannot write unam at level 2, column 3: a simple table has neither."},
		{objmod.Leveled, oneMod(mod("unam", textValue("a\x00b"))),
			`Cannot write "a\x00b": it contains NUL or invalid UTF-8.`},
		{objmod.Leveled, oneMod(mod("uhpm", realValue(float32(math.Inf(-1))))), "Cannot write -Inf as a float32."},
		{objmod.Leveled, oneMod(mod("uhpm", unrealValue(float32(math.NaN())))), "Cannot write NaN as a float32."},
	} {
		_, err := objmod.Append(nil, c.kind, c.objects, "war3map.w3a")
		if err == nil || err.Error() != c.words {
			t.Errorf("got %v, want %s", err, c.words)
		}
	}
}

// What follows the first thing wrong is not what the file meant, so it is not read: the error is of the first
// object or modification that is wrong, whatever is wrong with the ones after it.
func TestTheFirstProblemIsReportedWhenMoreObjectsOrModificationsFollowIt(t *testing.T) {
	typed := func(number int32) testkit.SyntheticMod {
		return testkit.SyntheticMod{Field: "unam", Value: objmod.Value{Type: objmod.ValueType(number)}}
	}
	object := func(mods ...testkit.SyntheticMod) testkit.SyntheticObject {
		return testkit.SyntheticObject{Base: "hfoo", ID: "h000", Mods: mods}
	}
	whole := textValue("whole")
	for _, c := range []struct {
		name   string
		custom []testkit.SyntheticObject
	}{
		{"two wrong objects", []testkit.SyntheticObject{object(typed(9)), object(typed(8))}},
		{"a wrong object and then a whole one",
			[]testkit.SyntheticObject{object(typed(9)), object(testkit.SyntheticMod{Field: "unam", Value: whole})}},
		{"two wrong modifications of one object", []testkit.SyntheticObject{object(typed(9), typed(8))}},
		{"a wrong modification and then two whole ones", []testkit.SyntheticObject{object(typed(9),
			testkit.SyntheticMod{Field: "unam", Value: whole}, testkit.SyntheticMod{Field: "uhpm", Value: intValue(7)})}},
	} {
		for _, version := range []int32{1, 2, 3} {
			for _, kind := range []objmod.TableKind{objmod.Simple, objmod.Leveled} {
				what := fmt.Sprintf("%s, version %d, kind %d", c.name, version, kind)
				parsed, err := objmod.Read(testkit.BuildModFile(version, nil, c.custom, kind), kind, modFile)
				refusal(t, what, err, modFile, "unknown value type 9")
				if parsed != nil {
					t.Errorf("%s: a refused file returned %+v", what, parsed)
				}
			}
		}
	}
}

// A count is refused when its items would not fit in the bytes that are left if each had its smallest size: an
// object without modifications, which has a count of sets and a flag from version 3 on, and a modification whose
// value is an empty string, which has a level and a column in a leveled table. So a file of such items reads, and
// the same file without its last byte has a count that is wrong.
func TestACountFitsWhenItsItemsOfTheSmallestSizeFillTheRestOfTheFile(t *testing.T) {
	bare := testkit.SyntheticObject{Base: "hfoo", ID: "h000"}
	empty := testkit.SyntheticMod{Field: "unam", Value: textValue("")}
	modified := testkit.SyntheticObject{Base: "hfoo", ID: "h000", Mods: []testkit.SyntheticMod{empty, empty, empty}}
	for _, version := range []int32{1, 2, 3} {
		for _, kind := range []objmod.TableKind{objmod.Simple, objmod.Leveled} {
			what := fmt.Sprintf("version %d, kind %d", version, kind)
			objects := testkit.BuildModFile(version, nil, []testkit.SyntheticObject{bare, bare, bare}, kind)
			if got := mustRead(t, objects, kind, modFile); len(got.Custom.Objects) != 3 {
				t.Errorf("%s: three objects without modifications read as %+v", what, got.Custom)
			}
			_, err := objmod.Read(objects[:len(objects)-1], kind, modFile)
			refusal(t, what+", objects without the last byte", err, modFile, "object count 3 past end of file")

			mods := testkit.BuildModFile(version, nil, []testkit.SyntheticObject{modified}, kind)
			if got := mustRead(t, mods, kind, modFile); len(got.Custom.Objects[0].Sets[0].Mods) != 3 {
				t.Errorf("%s: three modifications of empty strings read as %+v", what, got.Custom)
			}
			_, err = objmod.Read(mods[:len(mods)-1], kind, modFile)
			refusal(t, what+", modifications without the last byte", err, modFile, "modification count 3 past end of file")
		}
	}
}

func TestAnObjectHasOneToSixtyFourSets(t *testing.T) {
	object := func(sets int) []testkit.SyntheticObject {
		return []testkit.SyntheticObject{{Base: "hfoo", ID: "h000", Sets: make([]testkit.SyntheticSet, sets)}}
	}
	for _, sets := range []int{1, 2, 63, 64} {
		parsed := mustRead(t, testkit.BuildModFile(3, nil, object(sets), objmod.Simple), objmod.Simple, modFile)
		if len(parsed.Custom.Objects[0].Sets) != sets {
			t.Errorf("an object of %d sets reads with %d", sets, len(parsed.Custom.Objects[0].Sets))
		}
	}
	_, err := objmod.Read(testkit.BuildModFile(3, nil, object(65), objmod.Simple), objmod.Simple, modFile)
	refusal(t, "an object of 65 sets", err, modFile, "unsupported set count 65")
	// A count of 64 is no wrong count, so an object with fewer sets than it says ends too soon.
	counted := testkit.SetU32(testkit.BuildModFile(3, nil, object(1), objmod.Simple), 20, 64)
	_, err = objmod.Read(counted, objmod.Simple, modFile)
	refusal(t, "a set count of 64 before one set", err, modFile, "truncated")
}

func TestASliceOfALargerBufferParsesTheSameAsACopy(t *testing.T) {
	for _, file := range []string{"war3mapSkin.w3q", "war3map.w3d", "war3mapSkin.w3u"} {
		data := fixture(t, file)
		padded := testkit.Concat(make([]byte, 3), data, make([]byte, 4))
		view := mustRead(t, padded[3:3+len(data)], objmod.KindOf(file), file)
		if !reflect.DeepEqual(view, mustRead(t, data, objmod.KindOf(file), file)) {
			t.Errorf("%s parses differently as a slice", file)
		}
	}
}

// appended returns two objects to append, as the table kind allows them: simple tables have no level or column.
func appended(kind objmod.TableKind) []objmod.NewObject {
	objects := []objmod.NewObject{
		{Base: id("hfoo"), ID: id("X001"), Mods: []objmod.NewMod{
			{Field: id("uhpm"), Value: intValue(-250)},
			{Field: id("umvs"), Level: 2, Column: 1, Value: realValue(0.1)},
			{Field: id("ucbs"), Column: 3, Value: unrealValue(1.3)},
			{Field: id("unam"), Level: 1, Value: textValue(moonwell)},
		}},
		{Base: id("hpea"), ID: id("X002"), Mods: []objmod.NewMod{}},
	}
	if kind == objmod.Simple {
		for _, object := range objects {
			for i := range object.Mods {
				object.Mods[i].Level, object.Mods[i].Column = 0, 0
			}
		}
	}
	return objects
}

// asSynthetic is what a file holds for appended objects, as the test kit's builder writes them.
func asSynthetic(added []objmod.NewObject) []testkit.SyntheticObject {
	var out []testkit.SyntheticObject
	for _, object := range added {
		synthetic := testkit.SyntheticObject{Base: object.Base.String(), ID: object.ID.String()}
		synthetic.Mods = []testkit.SyntheticMod{}
		for _, mod := range object.Mods {
			synthetic.Mods = append(synthetic.Mods, testkit.SyntheticMod{
				Field: mod.Field.String(), Level: mod.Level, Column: mod.Column, Value: mod.Value,
			})
		}
		out = append(out, synthetic)
	}
	return out
}

func TestAppendingEachNamesFixtureObjectToNoFileReproducesWorldEditorsFiles(t *testing.T) {
	for _, name := range fixtureNames {
		for _, skin := range []bool{false, true} {
			file := "war3map." + name.ext
			if skin {
				file = "war3mapSkin." + name.ext
			}
			mods := []objmod.NewMod{}
			if skin == name.skin {
				mods = append(mods, objmod.NewMod{Field: id(name.field), Level: name.level, Value: textValue(name.value)})
			}
			added := []objmod.NewObject{{Base: id(name.base), ID: id(name.id), Mods: mods}}
			if got := mustAppend(t, nil, objmod.KindOf(file), added, file); !bytes.Equal(got, fixture(t, file)) {
				t.Errorf("%s is not World Editor's file", file)
			}
		}
	}
}

func TestAppendingToEveryNamesFixtureFileKeepsItsBytesAndAddsTheObjectsLast(t *testing.T) {
	for _, file := range fixtureFiles() {
		kind := objmod.KindOf(file)
		source := fixture(t, file)
		before := mustRead(t, source, kind, file)
		data := mustAppend(t, source, kind, appended(kind), file)
		custom := before.Custom
		if !bytes.Equal(data[:custom.CountOffset], source[:custom.CountOffset]) ||
			!bytes.Equal(data[custom.CountOffset:custom.Start], testkit.U32(uint32(len(custom.Objects)+2))) ||
			!bytes.Equal(data[custom.Start:custom.Stop], source[custom.Start:custom.Stop]) {
			t.Errorf("%s: the existing bytes changed", file)
		}
		after := mustRead(t, data, kind, file)
		count := len(after.Custom.Objects)
		if after.Version != 3 || !reflect.DeepEqual(after.Original, before.Original) ||
			!reflect.DeepEqual(after.Custom.Objects[:count-2], before.Custom.Objects) || after.Custom.Stop != len(data) {
			t.Errorf("%s: the file reads differently after appending", file)
		}
		// The appended objects are the bytes the test kit's builder writes for them.
		expected := testkit.BuildModFile(3, nil, asSynthetic(appended(kind)), kind)
		if !bytes.Equal(data[custom.Stop:], expected[12:]) {
			t.Errorf("%s: the appended objects are not encoded as expected", file)
		}
	}
}

func TestAppendingNoObjectsReturnsTheSourceBytes(t *testing.T) {
	for _, file := range []string{"war3map.w3u", "war3mapSkin.w3q", "war3map.w3d"} {
		source := fixture(t, file)
		if got := mustAppend(t, source, objmod.KindOf(file), nil, file); !bytes.Equal(got, source) {
			t.Errorf("%s changed", file)
		}
	}
}

func TestSyntheticV1V2AndV3SourcesGetObjectsInTheirOwnVersionsShape(t *testing.T) {
	original := []testkit.SyntheticObject{{
		Base: "hpea", ID: nulls, Mods: []testkit.SyntheticMod{{Field: "ugol", Value: intValue(90)}},
	}}
	existing := []testkit.SyntheticObject{
		{Base: "hfoo", ID: "h001", Mods: []testkit.SyntheticMod{{Field: "unam", Level: 2, Value: textValue("A")}}},
		{Base: "hfoo", ID: "h002", Mods: []testkit.SyntheticMod{{Field: "utip", Value: textValue("B"), End: "h002"}}},
	}
	// A version 3 object with two sets and a flag that is not 0, which Append never writes, is still copied as it is.
	multiSet := testkit.SyntheticObject{Base: "hfoo", ID: "h003", Sets: []testkit.SyntheticSet{
		{Flag: 7},
		{Mods: []testkit.SyntheticMod{{Field: "uhpm", Value: intValue(5)}}},
	}}
	for _, version := range []int32{1, 2, 3} {
		for _, kind := range []objmod.TableKind{objmod.Simple, objmod.Leveled} {
			custom := existing
			if version >= 3 {
				custom = append(slices.Clone(existing), multiSet)
			}
			source := testkit.BuildModFile(version, original, custom, kind)
			added := appended(kind)
			want := testkit.BuildModFile(version, original, append(slices.Clone(custom), asSynthetic(added)...), kind)
			if got := mustAppend(t, source, kind, added, "war3map.w3u"); !bytes.Equal(got, want) {
				t.Errorf("v%d, kind %v: the appended file differs", version, kind)
			}
		}
	}
}

func TestAppendingToAMalformedSourceIsTheReadersFileError(t *testing.T) {
	valid := fixture(t, "war3mapSkin.w3u")
	for _, c := range []struct {
		name   string
		source []byte
		words  string
	}{
		{"a source cut after the id of its object", valid[:20], "object count 1 past end of file"},
		{"a source cut inside its string", valid[:45], "unterminated string"},
		{"a source that is empty and not nil", []byte{}, "truncated"},
		{"a source of another version", testkit.SetU32(valid, 0, 4), "unsupported version 4"},
	} {
		data, err := objmod.Append(c.source, objmod.Simple, appended(objmod.Simple), "map/war3mapSkin.w3u")
		refusal(t, c.name, err, "map/war3mapSkin.w3u", c.words)
		if data != nil {
			t.Errorf("%s: a refused source returned % X", c.name, data)
		}
	}
}

// oneMod is one object to append with one modification.
func oneMod(mod objmod.NewMod) []objmod.NewObject {
	return []objmod.NewObject{{Base: id("hfoo"), ID: id("X001"), Mods: []objmod.NewMod{mod}}}
}

// unwritable is objects that Append must refuse, and the words its error has.
type unwritable struct {
	name    string
	kind    objmod.TableKind
	objects []objmod.NewObject
	words   string
}

// refusedByAppend checks that Append refuses the objects, added to no source and to a source, with an error that
// is not a file error and has the words, and that it returns no bytes.
func refusedByAppend(t *testing.T, cases []unwritable) {
	t.Helper()
	for _, c := range cases {
		for _, source := range [][]byte{nil, testkit.BuildModFile(2, nil, nil, c.kind)} {
			data, err := objmod.Append(source, c.kind, c.objects, "war3map.w3a")
			var fileError *diag.Error
			if err == nil || errors.As(err, &fileError) || !strings.Contains(err.Error(), c.words) {
				t.Errorf("%s: error = %v, want one with %q that is not a *diag.Error", c.name, err, c.words)
			}
			if data != nil {
				t.Errorf("%s: refused objects returned % X", c.name, data)
			}
		}
	}
}

func TestWhatAppendCannotWriteIsAnErrorThatIsNotAFileError(t *testing.T) {
	text := func(s string) []objmod.NewObject {
		return oneMod(objmod.NewMod{Field: id("unam"), Value: textValue(s)})
	}
	refusedByAppend(t, []unwritable{
		{"a text with a NUL", objmod.Leveled, text("a\x00b"), "NUL"},
		{"a text that is not UTF-8", objmod.Leveled, text("a\xFFb"), "UTF-8"},
		{"a text with a NUL in a simple table", objmod.Simple, text("\x00"), "NUL"},
		{"a level in a simple table", objmod.Simple,
			oneMod(objmod.NewMod{Field: id("unam"), Level: 1, Value: intValue(1)}), "simple table"},
		{"a column in a simple table", objmod.Simple,
			oneMod(objmod.NewMod{Field: id("unam"), Column: 1, Value: intValue(1)}), "simple table"},
	})
}

func TestAppendRefusesAValueTypeThatIsNoneOfTheFour(t *testing.T) {
	var cases []unwritable
	// The last is 3, the type of a string, in its low 32 bits, which are the ones a file stores.
	for _, valueType := range []objmod.ValueType{-1, 4, 7, 1<<32 | 3} {
		value := objmod.Value{Type: valueType, Int: 1, Real: 1, Text: "a"}
		for _, kind := range []objmod.TableKind{objmod.Simple, objmod.Leveled} {
			cases = append(cases, unwritable{
				fmt.Sprintf("the value type %d in a table of kind %d", valueType, kind), kind,
				oneMod(objmod.NewMod{Field: id("unam"), Value: value}), fmt.Sprintf("value type %d", valueType),
			})
		}
	}
	refusedByAppend(t, cases)
}

func TestAppendRefusesARealThatIsNotFinite(t *testing.T) {
	var cases []unwritable
	for _, c := range []struct {
		number float32
		words  string
	}{
		{float32(math.NaN()), "NaN"},
		{math.Float32frombits(0x7F800001), "NaN"},
		{float32(math.Inf(1)), "+Inf"},
		{float32(math.Inf(-1)), "-Inf"},
	} {
		for _, value := range []objmod.Value{realValue(c.number), unrealValue(c.number)} {
			cases = append(cases, unwritable{
				fmt.Sprintf("%v as a value of type %d", c.number, value.Type), objmod.Leveled,
				oneMod(objmod.NewMod{Field: id("umvs"), Value: value}), c.words,
			})
		}
	}
	refusedByAppend(t, cases)
	// Only a real is looked at: the Real of an int or of a string is not written.
	for _, value := range []objmod.Value{
		{Type: objmod.Int, Int: 1, Real: float32(math.NaN())},
		{Type: objmod.String, Text: "a", Real: float32(math.Inf(1))},
	} {
		mustAppend(t, nil, objmod.Leveled, oneMod(objmod.NewMod{Field: id("unam"), Value: value}), "war3map.w3a")
	}
}

func TestAppendRefusesAnIDOfFourNULs(t *testing.T) {
	mods := []objmod.NewMod{{Field: id("unam"), Value: intValue(1)}}
	refusedByAppend(t, []unwritable{
		{"an object without a base", objmod.Simple,
			[]objmod.NewObject{{ID: id("X001"), Mods: mods}}, "its base is four NUL bytes"},
		{"an object without an id", objmod.Simple,
			[]objmod.NewObject{{Base: id("hfoo"), Mods: mods}}, "its id is four NUL bytes"},
		{"an object without a base, an id or a modification", objmod.Leveled,
			[]objmod.NewObject{{}}, "its base is four NUL bytes"},
		{"a modification without a field", objmod.Leveled,
			oneMod(objmod.NewMod{Value: intValue(1)}), "its field is four NUL bytes"},
		{"a second object without an id", objmod.Leveled,
			[]objmod.NewObject{{Base: id("hfoo"), ID: id("X001"), Mods: mods}, {Base: id("hfoo")}},
			"its id is four NUL bytes"},
	})
	// An id with NUL bytes in it is an id as long as one byte is not NUL.
	partly := objmod.ID{0, 0, 0, 1}
	object := objmod.NewObject{Base: partly, ID: partly, Mods: []objmod.NewMod{{Field: partly, Value: intValue(1)}}}
	mustAppend(t, nil, objmod.Simple, []objmod.NewObject{object}, "war3map.w3u")
}

// TestAppendLeavesItsSourceAsItIs gives Append a source with spare capacity, where an append in place would go.
func TestAppendLeavesItsSourceAsItIs(t *testing.T) {
	for _, file := range []string{"war3mapSkin.w3u", "war3mapSkin.w3q"} {
		kind := objmod.KindOf(file)
		whole := fixture(t, file)
		for _, added := range [][]objmod.NewObject{nil, {}, appended(kind)} {
			buffer := bytes.Repeat([]byte{0xAA}, len(whole)+4096)
			source := buffer[:copy(buffer, whole)]
			before := bytes.Clone(buffer)
			data := mustAppend(t, source, kind, added, file)
			if !bytes.Equal(buffer, before) {
				t.Errorf("%s with %d objects: Append changed its source or the bytes after it", file, len(added))
			}
			// Nothing written into the result may show in the source, or in the capacity it has to spare.
			data = append(data, 0x55)
			for i := range data {
				data[i] = 0x55
			}
			if !bytes.Equal(buffer, before) {
				t.Errorf("%s with %d objects: the result shares its bytes with the source", file, len(added))
			}
		}
	}
}

func TestAppendWritesEveryNumberAndEveryID(t *testing.T) {
	// In a new leveled file the first object's id is at 16 and its first value at 48.
	valueAt := func(value objmod.Value) []byte {
		t.Helper()
		object := objmod.NewObject{Base: id("hfoo"), ID: objmod.ID{0, 0xFF, 0xE9, 'A'}, Mods: []objmod.NewMod{
			{Field: id("unam"), Level: math.MinInt32, Column: math.MaxInt32, Value: value},
		}}
		data := mustAppend(t, nil, objmod.Leveled, []objmod.NewObject{object}, "war3map.w3a")
		if !bytes.Equal(data[16:20], []byte{0, 0xFF, 0xE9, 'A'}) {
			t.Errorf("the id is written % X", data[16:20])
		}
		if !bytes.Equal(data[40:48], testkit.Concat(testkit.U32(0x80000000), testkit.U32(0x7FFFFFFF))) {
			t.Errorf("the level and the column are written % X", data[40:48])
		}
		return data[48:52]
	}
	for _, c := range []struct {
		name  string
		value objmod.Value
		want  uint32
	}{
		{"the smallest int", intValue(math.MinInt32), 0x80000000},
		{"the largest int", intValue(math.MaxInt32), 0x7FFFFFFF},
		{"the real furthest below zero", realValue(-math.MaxFloat32), 0xFF7FFFFF},
		{"a real that needs every bit", unrealValue(math.Float32frombits(0x3DCCCCCD)), 0x3DCCCCCD},
		{"an unreal below zero", unrealValue(-1.5), 0xBFC00000},
		{"the smallest real above zero", realValue(math.Float32frombits(1)), 1},
		// The text of a value that is not a String is not written, so nothing is wrong with it.
		{"an int with a text", objmod.Value{Type: objmod.Int, Int: 7, Text: "a\x00b"}, 7},
	} {
		if got := valueAt(c.value); !bytes.Equal(got, testkit.U32(c.want)) {
			t.Errorf("%s is written % X, want % X", c.name, got, testkit.U32(c.want))
		}
	}
}
