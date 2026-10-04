package objmod_test

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"testing"

	olddiag "github.com/mdlsvensson/moonwell/internal/diag"
	oldobjects "github.com/mdlsvensson/moonwell/internal/objects"
	oldkit "github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/oracle"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/war3/objmod"
)

// comparison runs the other tree's package and this one on the same input, compares what they return through the
// oracle, and counts the comparisons it made.
type comparison struct {
	t     *testing.T
	count int
}

// said is what an error tells its reader, in the shape both trees are compared in.
type said struct {
	Expected        bool // a diag error
	Msg, File, Hint string
	Line, Column    int
}

func saidByTheOtherTree(err error) said {
	var failure *olddiag.Error
	if errors.As(err, &failure) {
		return said{true, failure.Msg, failure.File, failure.Hint, failure.Line, failure.Column}
	}
	return said{Msg: reduced(err.Error())}
}

func saidByThisTree(err error) said {
	var failure *diag.Error
	if errors.As(err, &failure) {
		return said{true, failure.Msg, failure.File, failure.Hint, failure.Line, failure.Column}
	}
	return said{Msg: reduced(err.Error())}
}

// reduced is the message of an error that is not a diag error, with the two refusals the trees spell differently
// brought to what they share. Any other message is compared whole.
func reduced(message string) string {
	return refusedReal(refusedText(message))
}

// refusedText reduces the refusal of a text to the text it names. The two trees word the sentence differently:
// this tree quotes the text as Go does and calls bytes that are not UTF-8 by that name. Every text the oracle has
// refused starts with a word of its own in small letters, which both ways of quoting leave as it is, so the
// comparison still tells which text was refused.
func refusedText(message string) string {
	quoted, found := strings.CutPrefix(message, `Cannot write "`)
	if !found || !strings.Contains(message, ": it contains NUL or ") {
		return message
	}
	end := strings.IndexFunc(quoted, func(r rune) bool { return r < 'a' || r > 'z' })
	return "refuses the text that starts with " + quoted[:max(end, 0)]
}

// infinities turns the other tree's spelling of the two infinite numbers into this tree's.
var infinities = strings.NewReplacer("-Infinity", "-Inf", "Infinity", "+Inf")

// refusedReal reduces the refusal of a real that is not finite to the number it names. The sentence is the same
// in both trees; the other tree writes an infinite number out as Infinity and this one as +Inf.
func refusedReal(message string) string {
	number, found := strings.CutPrefix(message, "Cannot write ")
	number, isReal := strings.CutSuffix(number, " as a float32.")
	if !found || !isReal {
		return message
	}
	return "refuses the real " + infinities.Replace(number)
}

// errors compares two errors: that both are nil or both are not, and for two errors every word they say.
func (c *comparison) errors(what string, want, got error) {
	c.t.Helper()
	c.count++
	if oracle.Errors(c.t, what, want, got) {
		c.values(what+": the error", saidByTheOtherTree(want), saidByThisTree(got))
	}
}

func (c *comparison) values(what string, want, got any) {
	c.t.Helper()
	c.count++
	oracle.Values(c.t, what, want, got)
}

func (c *comparison) bytes(what string, want, got []byte) {
	c.t.Helper()
	c.count++
	oracle.Bytes(c.t, what, want, got)
}

// ---- this tree's inputs in the other tree's types ----

func otherKind(kind objmod.TableKind) oldobjects.TableKind {
	if kind == objmod.Leveled {
		return oldobjects.Leveled
	}
	return oldobjects.Simple
}

// otherValue is a value as the other tree takes it: its type by name and every number as a float64.
func otherValue(t *testing.T, value objmod.Value) oldobjects.ModValue {
	t.Helper()
	other := oldobjects.ModValue{Text: value.Text}
	switch value.Type {
	case objmod.Int:
		other.Type, other.Number = "int", float64(value.Int)
	case objmod.Real:
		other.Type, other.Number = "real", float64(value.Real)
	case objmod.Unreal:
		other.Type, other.Number = "unreal", float64(value.Real)
	case objmod.String:
		other.Type = "string"
	default:
		t.Fatalf("the other tree has no value type %d", value.Type)
	}
	return other
}

func otherObjects(t *testing.T, added []objmod.NewObject) []oldobjects.NewObject {
	t.Helper()
	var others []oldobjects.NewObject
	for _, object := range added {
		other := oldobjects.NewObject{Base: object.Base.String(), ID: object.ID.String()}
		for _, mod := range object.Mods {
			other.Mods = append(other.Mods, oldobjects.NewMod{
				Field: mod.Field.String(), Level: int64(mod.Level), Column: int64(mod.Column), Value: otherValue(t, mod.Value),
			})
		}
		others = append(others, other)
	}
	return others
}

func otherMods(t *testing.T, mods []testkit.SyntheticMod) []oldkit.SyntheticMod {
	t.Helper()
	var others []oldkit.SyntheticMod
	for _, mod := range mods {
		others = append(others, oldkit.SyntheticMod{
			Field: mod.Field, Level: mod.Level, Column: mod.Column, Value: otherValue(t, mod.Value), End: mod.End,
		})
	}
	return others
}

// otherSynthetic is the objects of a synthetic file as the other tree's builder takes them.
func otherSynthetic(t *testing.T, objects []testkit.SyntheticObject) []oldkit.SyntheticObject {
	t.Helper()
	var others []oldkit.SyntheticObject
	for _, object := range objects {
		other := oldkit.SyntheticObject{Base: object.Base, ID: object.ID, Mods: otherMods(t, object.Mods)}
		for _, set := range object.Sets {
			other.Sets = append(other.Sets, oldkit.SyntheticSet{Flag: set.Flag, Mods: otherMods(t, set.Mods)})
		}
		others = append(others, other)
	}
	return others
}

// ---- what the other tree read, in this tree's types ----

// fromOld turns the file the other tree read into this tree's File, so that the oracle compares two values of one
// shape. It returns an error for anything it cannot convert: an id that is not four characters of one byte each, a
// value type that is not one of the four, a number its type cannot hold.
func fromOld(old *oldobjects.ModFile) (*objmod.File, error) {
	if old == nil {
		return nil, nil
	}
	c := &conversion{}
	file := &objmod.File{Version: old.Version, Original: c.table(old.Original), Custom: c.table(old.Custom)}
	return file, c.problem
}

// conversion keeps the first thing it could not convert.
type conversion struct{ problem error }

func (c *conversion) refuse(format string, args ...any) {
	if c.problem == nil {
		c.problem = fmt.Errorf(format, args...)
	}
}

// each converts a list and keeps a nil one nil: JSON writes that differently from an empty one.
func each[Old, New any](old []Old, convert func(Old) New) []New {
	if old == nil {
		return nil
	}
	converted := make([]New, 0, len(old))
	for _, item := range old {
		converted = append(converted, convert(item))
	}
	return converted
}

func (c *conversion) table(old oldobjects.ModTable) objmod.Table {
	return objmod.Table{
		CountOffset: old.CountOffset, Start: old.Start, Stop: old.Stop, Objects: each(old.Objects, c.object),
	}
}

func (c *conversion) object(old oldobjects.ObjectEntry) objmod.Object {
	return objmod.Object{
		Base: c.id(old.Base), ID: c.id(old.ID), Sets: each(old.Sets, c.set), Start: old.Start, Stop: old.Stop,
	}
}

func (c *conversion) set(old oldobjects.ModSet) objmod.Set {
	return objmod.Set{Flag: old.Flag, Mods: each(old.Mods, c.modification)}
}

func (c *conversion) modification(old oldobjects.Modification) objmod.Modification {
	return objmod.Modification{
		Field: c.id(old.Field), Level: old.Level, Column: old.Column, Value: c.value(old.Value), End: c.id(old.End),
		Start: old.Start, Stop: old.Stop,
	}
}

func (c *conversion) id(old string) objmod.ID {
	parsed, ok := objmod.ParseID(old)
	if !ok {
		c.refuse("the id %q is not four characters of one byte each", old)
	}
	return parsed
}

// value converts a value whose number is a float64 whatever its type. A text beside a number, which neither tree
// reads, is carried over so that the comparison shows it.
func (c *conversion) value(old oldobjects.ModValue) objmod.Value {
	value := objmod.Value{Text: old.Text}
	switch old.Type {
	case "int":
		value.Type = objmod.Int
		if old.Number != math.Trunc(old.Number) || old.Number < math.MinInt32 || old.Number > math.MaxInt32 {
			c.refuse("the int %v is not an int32", old.Number)
			return value
		}
		value.Int = int32(old.Number)
	case "real", "unreal":
		value.Type, value.Real = objmod.Real, float32(old.Number)
		if old.Type == "unreal" {
			value.Type = objmod.Unreal
		}
		if float64(value.Real) != old.Number && !math.IsNaN(old.Number) {
			c.refuse("the %s %v is not a float32", old.Type, old.Number)
		}
	case "string":
		value.Type = objmod.String
		if old.Number != 0 {
			c.refuse("the string %q comes with the number %v", old.Text, old.Number)
		}
	default:
		c.refuse("the value type %q is not one of the four", old.Type)
	}
	return value
}

func TestFromOldRefusesWhatItCannotConvert(t *testing.T) {
	withObject := func(object oldobjects.ObjectEntry) *oldobjects.ModFile {
		return &oldobjects.ModFile{Version: 3, Custom: oldobjects.ModTable{Objects: []oldobjects.ObjectEntry{object}}}
	}
	withMod := func(mod oldobjects.Modification) *oldobjects.ModFile {
		sets := []oldobjects.ModSet{{Mods: []oldobjects.Modification{mod}}}
		return withObject(oldobjects.ObjectEntry{Base: "hfoo", ID: "h001", Sets: sets})
	}
	withIDs := func(field, end string) *oldobjects.ModFile {
		return withMod(oldobjects.Modification{Field: field, Value: oldobjects.IntValue(1), End: end})
	}
	withValue := func(value oldobjects.ModValue) *oldobjects.ModFile {
		return withMod(oldobjects.Modification{Field: "unam", Value: value, End: nulls})
	}
	for _, c := range []struct {
		name  string
		old   *oldobjects.ModFile
		words string
	}{
		{"a field of three characters", withIDs("una", nulls), "id"},
		{"an end token above U+00FF", withIDs("unam", "X00\xE2\x82\xAC"), "id"},
		{"an object id of five characters", withObject(oldobjects.ObjectEntry{Base: "hfoo", ID: "h0001"}), "id"},
		{"a base that is empty", withObject(oldobjects.ObjectEntry{ID: "h001"}), "id"},
		{"a value type that is not one of the four", withValue(oldobjects.ModValue{Type: "bool", Number: 1}), "value type"},
		{"a value without a type", withValue(oldobjects.ModValue{}), "value type"},
		{"an int with a fraction", withValue(oldobjects.ModValue{Type: "int", Number: 1.5}), "int32"},
		{"an int past the largest", withValue(oldobjects.ModValue{Type: "int", Number: 2147483648}), "int32"},
		{"an int that is not a number", withValue(oldobjects.ModValue{Type: "int", Number: math.NaN()}), "int32"},
		{"a real that needs 64 bits", withValue(oldobjects.RealValue("real", 0.1)), "float32"},
		{"an unreal past the largest float32", withValue(oldobjects.RealValue("unreal", 1e39)), "float32"},
		{"a string with a number", withValue(oldobjects.ModValue{Type: "string", Number: 1, Text: "a"}), "number"},
	} {
		if _, err := fromOld(c.old); err == nil || !strings.Contains(err.Error(), c.words) {
			t.Errorf("%s: fromOld's error is %v, want one about %s", c.name, err, c.words)
		}
	}
	whole := withMod(oldobjects.Modification{
		Field: "umvs", Level: 2, Column: 1, Value: oldobjects.RealValue("unreal", 0.5), End: "h001", Start: 3, Stop: 9,
	})
	wantSets := []objmod.Set{{Mods: []objmod.Modification{{
		Field: id("umvs"), Level: 2, Column: 1, Value: unrealValue(0.5), End: id("h001"), Start: 3, Stop: 9,
	}}}}
	want := &objmod.File{Version: 3, Custom: objmod.Table{
		Objects: []objmod.Object{{Base: id("hfoo"), ID: id("h001"), Sets: wantSets}},
	}}
	got, err := fromOld(whole)
	if err != nil {
		t.Fatalf("fromOld refuses a file it can convert: %v", err)
	}
	oracle.Values(t, "a file fromOld can convert", want, got)
	if none, err := fromOld(nil); none != nil || err != nil {
		t.Errorf("fromOld of no file is %+v, %v", none, err)
	}
}

// eachValue calls visit with every value of a file.
func eachValue(file *objmod.File, visit func(*objmod.Value)) {
	if file == nil {
		return
	}
	for _, objects := range [][]objmod.Object{file.Original.Objects, file.Custom.Objects} {
		for _, object := range objects {
			for _, set := range object.Sets {
				for i := range set.Mods {
					visit(&set.Mods[i].Value)
				}
			}
		}
	}
}

// takeReals returns the reals of a file as text and sets those that are not finite to zero. A file that reads may
// hold a real that is infinite or not a number, which the oracle cannot compare: JSON has no way to write it. So
// the reals are compared as text, and the file with those reals at zero.
func takeReals(file *objmod.File) []string {
	var reals []string
	eachValue(file, func(value *objmod.Value) {
		if value.Type != objmod.Real && value.Type != objmod.Unreal {
			return
		}
		number := float64(value.Real)
		reals = append(reals, strconv.FormatFloat(number, 'g', -1, 32))
		if math.IsNaN(number) || math.IsInf(number, 0) {
			value.Real = 0
		}
	})
	return reals
}

// ---- the comparisons ----

// reads compares Read of data as a table of the given kind, and returns the other tree's error.
func (c *comparison) reads(what string, data []byte, kind objmod.TableKind) error {
	c.t.Helper()
	want, wantErr := oldobjects.ReadModFile(data, otherKind(kind), modFile)
	got, gotErr := objmod.Read(data, kind, modFile)
	c.errors(what, wantErr, gotErr)
	converted, err := fromOld(want)
	if err != nil {
		c.t.Errorf("%s: what the other tree read cannot be compared: %v", what, err)
		return wantErr
	}
	c.values(what+": the reals", takeReals(converted), takeReals(got))
	c.values(what, converted, got)
	return wantErr
}

// appends compares Append of objects to a source and, when the other tree wrote a file, what both trees read from
// it. It returns the other tree's error.
func (c *comparison) appends(what string, source []byte, kind objmod.TableKind, added []objmod.NewObject) error {
	c.t.Helper()
	want, wantErr := oldobjects.AppendObjects(source, otherKind(kind), otherObjects(c.t, added), modFile)
	got, gotErr := objmod.Append(source, kind, added, modFile)
	c.errors(what, wantErr, gotErr)
	if wantErr != nil {
		// The other tree returns what it had written beside its error. This one returns nothing, which is all
		// there is to compare.
		if got != nil {
			c.t.Errorf("%s: a refused Append returned % X", what, got)
		}
		return wantErr
	}
	c.bytes(what, want, got)
	if err := c.reads(what+", read again", want, kind); err != nil {
		c.t.Errorf("%s: the other tree does not read what it wrote: %v", what, err)
	}
	return nil
}

// ---- the inputs ----

// input is the bytes of one modification file, whole or not, with its table kind and a name for a failure.
type input struct {
	name string
	data []byte
	kind objmod.TableKind
}

// worldEditorsFiles returns the fixture's fourteen files.
func worldEditorsFiles(t *testing.T) []input {
	t.Helper()
	var files []input
	for _, file := range fixtureFiles() {
		files = append(files, input{file, fixture(t, file), objmod.KindOf(file)})
	}
	if len(files) != 14 {
		t.Fatalf("the fixture has %d files, want 14", len(files))
	}
	return files
}

// syntheticTables returns the two tables of a synthetic file: objects with values of every type, with ids that
// are not ASCII, with no modifications, with end tokens, and in version 3 with several sets.
func syntheticTables(version int32) (original, custom []testkit.SyntheticObject) {
	original = []testkit.SyntheticObject{
		{Base: "hpea", ID: nulls, Mods: []testkit.SyntheticMod{
			{Field: "ugol", Value: intValue(90)},
			{Field: "umvs", Level: 1, Column: 2, Value: realValue(270.5)},
			{Field: "utip", Level: 3, Value: textValue("A tip")},
		}},
		{Base: "hfoo", ID: nulls},
	}
	custom = []testkit.SyntheticObject{
		{Base: "hfoo", ID: "h001", Mods: []testkit.SyntheticMod{
			{Field: "uhpm", Value: intValue(-250)},
			{Field: "umvs", Level: 2, Column: 1, Value: realValue(0.1)},
			{Field: "ucbs", Column: 3, Value: unrealValue(1.3)},
			{Field: "unam", Level: 1, Value: textValue(moonwell), End: "h001"},
			{Field: "utip", Value: textValue("")},
		}},
		{Base: "hfoo", ID: "h002"},
		{Base: "\x00\xC3\xBF\xC3\xA9A", ID: "\xC3\xBF\xC3\xBF\xC3\xBF\xC3\xBF", Mods: []testkit.SyntheticMod{
			{Field: "\xC2\x80\x00\x7F\xC2\xA0", Level: math.MinInt32, Column: math.MaxInt32, Value: intValue(math.MinInt32),
				End: "\xC3\xBF\x00\x00\x01"},
		}},
	}
	if version >= 3 {
		custom = append(custom, testkit.SyntheticObject{Base: "hfoo", ID: "h003", Sets: []testkit.SyntheticSet{
			{Flag: 7},
			{Flag: -1, Mods: []testkit.SyntheticMod{
				{Field: "uhpm", Value: intValue(5)}, {Field: "ucbs", Value: unrealValue(-0.25)},
			}},
			{Mods: []testkit.SyntheticMod{{Field: "unam", Level: 9, Value: textValue("B")}}},
		}})
	}
	return original, custom
}

var (
	versions = []int32{1, 2, 3}
	kinds    = []objmod.TableKind{objmod.Simple, objmod.Leveled}
)

// synthetic is a file for the test kit to build.
type synthetic struct {
	name             string
	version          int32
	kind             objmod.TableKind
	original, custom []testkit.SyntheticObject
}

// syntheticFiles returns three files of every version and kind: one with objects in both tables, one whose custom
// table is empty, as in a map that only changes standard objects, and one with both tables empty.
func syntheticFiles() []synthetic {
	var files []synthetic
	for _, version := range versions {
		for _, kind := range kinds {
			original, custom := syntheticTables(version)
			name := fmt.Sprintf("synthetic version %d, kind %d", version, kind)
			files = append(files,
				synthetic{name, version, kind, original, custom},
				synthetic{name + " without custom objects", version, kind, original, nil},
				synthetic{name + " without objects", version, kind, nil, nil},
			)
		}
	}
	return files
}

// syntheticSources returns the synthetic files as the test kit of this tree builds them.
func syntheticSources() []input {
	var files []input
	for _, file := range syntheticFiles() {
		data := testkit.BuildModFile(file.version, file.original, file.custom, file.kind)
		files = append(files, input{file.name, data, file.kind})
	}
	return files
}

// kindOtherThan is the table kind a file is not.
func kindOtherThan(kind objmod.TableKind) objmod.TableKind {
	if kind == objmod.Leveled {
		return objmod.Simple
	}
	return objmod.Leveled
}

// wholeFiles returns every file that reads: World Editor's and the synthetic ones.
func wholeFiles(t *testing.T) []input {
	t.Helper()
	return append(worldEditorsFiles(t), syntheticSources()...)
}

// customObjects returns the custom objects of a file as objects to append: every modification of every set, in
// the file's order.
func customObjects(file *objmod.File) []objmod.NewObject {
	var objects []objmod.NewObject
	for _, object := range file.Custom.Objects {
		added := objmod.NewObject{Base: object.Base, ID: object.ID}
		for _, set := range object.Sets {
			for _, mod := range set.Mods {
				added.Mods = append(added.Mods, objmod.NewMod{
					Field: mod.Field, Level: mod.Level, Column: mod.Column, Value: mod.Value,
				})
			}
		}
		objects = append(objects, added)
	}
	return objects
}

// edgeObjects returns objects whose values are at the edges of what both trees can write: the ends of int32, reals
// that need every bit of a float32, texts that are empty, long or not ASCII, ids of any four bytes, and in a leveled
// table the ends of the level and the column.
func edgeObjects(kind objmod.TableKind) []objmod.NewObject {
	var mods []objmod.NewMod
	add := func(field string, value objmod.Value) {
		mods = append(mods, objmod.NewMod{Field: id(field), Value: value})
	}
	for _, n := range []int32{math.MinInt32, math.MaxInt32, 0, -1, 1} {
		add("uhpm", intValue(n))
	}
	// A tenth, the largest and the smallest finite numbers, the smallest above zero, the largest below a normal
	// number, zero with a sign, the smallest normal number, a third, and an odd number above 2^24.
	for _, bits := range []uint32{0x3DCCCCCD, 0x7F7FFFFF, 0xFF7FFFFF, 0x00000001, 0x807FFFFF, 0x80000000, 0x00800000,
		0x3EAAAAAB, 0x4B800001} {
		add("umvs", realValue(math.Float32frombits(bits)))
		add("ucbs", unrealValue(math.Float32frombits(bits)))
	}
	for _, text := range []string{"", moonwell, "\xEF\xBB\xBFa text after a byte order mark", "\t\r\n\x01\x7F \"\\",
		"\xF0\x9F\x8C\x99 \xE6\x9C\x88", strings.Repeat("long ", 2000)} {
		add("unam", textValue(text))
	}
	objects := []objmod.NewObject{
		{Base: id("hfoo"), ID: id("X001"), Mods: mods},
		{Base: objmod.ID{0, 0xFF, 0xE9, 'A'}, ID: objmod.ID{0xFF, 0xFF, 0xFF, 0xFF}},
		{Base: objmod.ID{0, 0, 0, 1}, ID: objmod.ID{1, 0, 0, 0}, Mods: []objmod.NewMod{
			{Field: objmod.ID{0x80, 0, 0x7F, 0xA0}, Value: intValue(3)}, {Field: objmod.ID{0, 0, 1, 0}, Value: intValue(4)},
		}},
	}
	if kind == objmod.Leveled {
		objects = append(objects, objmod.NewObject{Base: id("AHbz"), ID: id("A001"), Mods: []objmod.NewMod{
			{Field: id("Hbz1"), Level: math.MinInt32, Column: math.MaxInt32, Value: unrealValue(2.5)},
			{Field: id("Hbz1"), Level: math.MaxInt32, Column: math.MinInt32, Value: textValue("a level")},
			{Field: id("Hbz1"), Level: 1, Column: -1, Value: intValue(1)},
		}})
	}
	return objects
}

// ---- the tests ----

func TestOracleOnTableKinds(t *testing.T) {
	c := &comparison{t: t}
	names := append(fixtureFiles(), "WAR3MAP.W3A", "war3mapSkin.w3Q", "map/war3map.w3d", `map\war3map.w3d`, ".w3d", "w3a",
		"war3map.w3a ", "war3map.w3a\n", "war3map.w3a/", "x.w3a/y", `x.w3a\y`, "a.b.w3q", "a.w3q.b", "war3map.w3ab",
		"war3map.xw3a", "war3map.w3", "war3map.w3\xC3\xA4", "war3map.\xE2\x84\xAA3a", "war3map.w3a\xFF", "\xFF.w3a",
		"war3map..w3a", "war3mapw3a", "", ".", "war3map.w3U", "war3map.W3u", "war3map.doo")
	leveled := 0
	for _, name := range names {
		want, got := oldobjects.KindOf(name) == oldobjects.Leveled, objmod.KindOf(name) == objmod.Leveled
		c.values(fmt.Sprintf("whether %q is leveled", name), want, got)
		if want {
			leveled++
		}
	}
	if leveled < 10 || leveled > len(names)-10 {
		t.Errorf("the other tree calls %d of %d names leveled, which compares too little", leveled, len(names))
	}
	t.Logf("%d names, %d comparisons", len(names), c.count)
}

// TestOracleOnIDs compares ParseID with what the other tree's writer makes of an id, and String with what its
// reader makes of four bytes.
func TestOracleOnIDs(t *testing.T) {
	c := &comparison{t: t}
	type parsed struct {
		OK bool
		ID []int
	}
	texts := []string{"hfoo", "X001", nulls, "\x00\xC3\xBF\xC3\xA9A", "\xC3\xBF\xC3\xBF\xC3\xBF\xC3\xBF", "a\x00b\x7F",
		"\xC2\x80\xC2\x81\xC2\x82\xC2\x83", "", "X", "X01", "X0001", "X00\xE2\x82\xAC", "X00\xC4\x80", "X00\xFF",
		"\xFF\xFF\xFF\xFF", "\xC3\xBF\xC3\xA9", "X00\xEF\xBF\xBD", "X00\xF0\x9F\x8C\x99", "hfoo\x00", " hfo", "hf o",
		"\xC3\xBFbc", "ab\xC3", "abc\xC3\xBF\xC3\xBF"}
	accepted := 0
	for _, text := range texts {
		object := oldobjects.NewObject{Base: text, ID: "X001"}
		data, err := oldobjects.AppendObjects(nil, oldobjects.Simple, []oldobjects.NewObject{object}, modFile)
		want := parsed{OK: err == nil}
		if want.OK {
			accepted++
			want.ID = []int{int(data[12]), int(data[13]), int(data[14]), int(data[15])}
		}
		made, ok := objmod.ParseID(text)
		got := parsed{OK: ok}
		if ok {
			got.ID = []int{int(made[0]), int(made[1]), int(made[2]), int(made[3])}
		} else if made != (objmod.ID{}) {
			t.Errorf("ParseID(%q) refuses and returns %v", text, made)
		}
		c.values(fmt.Sprintf("the id %q", text), want, got)
	}
	if accepted < 5 || accepted > len(texts)-5 {
		t.Errorf("the other tree accepts %d of %d ids, which compares too little", accepted, len(texts))
	}
	for b := range 256 {
		raw := objmod.ID{byte(b), 'x', byte(255 - b), byte(b)}
		file := testkit.Concat(testkit.U32(3), testkit.U32(0), testkit.U32(1), raw[:], []byte("X001"), testkit.U32(1),
			testkit.U32(0), testkit.U32(0))
		read, err := oldobjects.ReadModFile(file, oldobjects.Simple, modFile)
		if err != nil {
			t.Fatalf("the other tree does not read an object with the base % X: %v", raw[:], err)
		}
		c.values(fmt.Sprintf("the id % X as text", raw[:]), read.Custom.Objects[0].Base, raw.String())
	}
	t.Logf("%d texts, 256 ids, %d comparisons", len(texts), c.count)
}

func TestOracleOnWorldEditorsFiles(t *testing.T) {
	c := &comparison{t: t}
	files := worldEditorsFiles(t)
	for _, file := range files {
		if err := c.reads(file.name, file.data, file.kind); err != nil {
			t.Errorf("%s: the other tree does not read it: %v", file.name, err)
		}
		c.reads(file.name+" as a table of the other kind", file.data, kindOtherThan(file.kind))
	}
	t.Logf("%d files, %d comparisons", len(files), c.count)
}

// TestOracleOnAppendingWorldEditorsObjects appends every custom object of every fixture file to no source and to
// every fixture file of its table kind.
func TestOracleOnAppendingWorldEditorsObjects(t *testing.T) {
	c := &comparison{t: t}
	files, appends := worldEditorsFiles(t), 0
	for _, from := range files {
		objects := customObjects(mustRead(t, from.data, from.kind, from.name))
		if len(objects) != 1 {
			t.Fatalf("%s has %d custom objects, want 1", from.name, len(objects))
		}
		targets := []input{{name: "no source", kind: from.kind}}
		for _, to := range files {
			if to.kind == from.kind {
				targets = append(targets, to)
			}
		}
		for _, to := range targets {
			what := fmt.Sprintf("the object of %s appended to %s", from.name, to.name)
			if err := c.appends(what, to.data, from.kind, objects); err != nil {
				t.Errorf("%s: the other tree wrote nothing: %v", what, err)
			}
			appends++
		}
	}
	// Eight files of simple tables, each object to nine targets; six of leveled tables, each to seven.
	if appends != 8*9+6*7 {
		t.Errorf("%d appends, want %d", appends, 8*9+6*7)
	}
	t.Logf("%d appends, %d comparisons", appends, c.count)
}

// TestOracleOnSyntheticSources builds the files of every version and kind with both test kits (with objects in both
// tables, with an empty custom table, with two empty tables) and appends to each: objects of every value type, no
// objects, objects at the edges of what the values hold, and its own custom objects again.
func TestOracleOnSyntheticSources(t *testing.T) {
	c := &comparison{t: t}
	files := syntheticFiles()
	for _, file := range files {
		kind := file.kind
		source := testkit.BuildModFile(file.version, file.original, file.custom, kind)
		other := oldkit.BuildModFile(file.version, otherSynthetic(t, file.original), otherSynthetic(t, file.custom),
			otherKind(kind))
		c.bytes(file.name+": the two builders", other, source)
		if err := c.reads(file.name, source, kind); err != nil {
			t.Fatalf("%s: the other tree does not read it: %v", file.name, err)
		}
		own := customObjects(mustRead(t, source, kind, file.name))
		for _, added := range []struct {
			name    string
			objects []objmod.NewObject
		}{
			{"two objects", appended(kind)},
			{"no objects", nil},
			{"an empty list of objects", []objmod.NewObject{}},
			{"objects at the edges", edgeObjects(kind)},
			{"its own custom objects", own},
		} {
			what := file.name + ": " + added.name
			if err := c.appends(what, source, kind, added.objects); err != nil {
				t.Errorf("%s: the other tree wrote nothing: %v", what, err)
			}
		}
	}
	if len(files) != 18 {
		t.Errorf("%d synthetic files, want 18: three of each version and kind", len(files))
	}
	t.Logf("%d files, %d comparisons", len(files), c.count)
}

// TestOracleOnNewFiles appends to no source, which gives a new file.
func TestOracleOnNewFiles(t *testing.T) {
	c := &comparison{t: t}
	for _, kind := range kinds {
		for _, added := range []struct {
			name    string
			objects []objmod.NewObject
		}{
			{"two objects", appended(kind)},
			{"no objects", nil},
			{"an empty list of objects", []objmod.NewObject{}},
			{"objects at the edges", edgeObjects(kind)},
		} {
			what := fmt.Sprintf("a new file of kind %d: %s", kind, added.name)
			if err := c.appends(what, nil, kind, added.objects); err != nil {
				t.Errorf("%s: the other tree wrote nothing: %v", what, err)
			}
		}
	}
	t.Logf("%d comparisons", c.count)
}

// TestOracleOnSourcesWithRealsThatAreNotFinite puts bits that are not a finite number where a file holds a real.
// Both trees read such a file and copy it when they append.
func TestOracleOnSourcesWithRealsThatAreNotFinite(t *testing.T) {
	c := &comparison{t: t}
	for _, kind := range kinds {
		reals := []testkit.SyntheticMod{{Field: "umvs", Value: realValue(1)}, {Field: "ucbs", Value: unrealValue(2)}}
		source := testkit.BuildModFile(3, nil, []testkit.SyntheticObject{{Base: "hfoo", ID: "h001", Mods: reals}}, kind)
		mods := mustRead(t, source, kind, modFile).Custom.Objects[0].Sets[0].Mods
		for _, mod := range mods {
			// The four bytes of the value come before the four of the end token.
			for _, bits := range []uint32{0x7F800000, 0xFF800000, 0x7FC00000, 0x7F800001, 0xFFFFFFFF, 0x7FFFFFFF} {
				what := fmt.Sprintf("kind %d with %s set to the bits %08X", kind, mod.Field, bits)
				data := testkit.SetU32(source, mod.Stop-8, bits)
				if err := c.reads(what, data, kind); err != nil {
					t.Errorf("%s: the other tree does not read it: %v", what, err)
				}
				if err := c.appends(what+", appended to", data, kind, appended(kind)); err != nil {
					t.Errorf("%s: the other tree wrote nothing: %v", what, err)
				}
			}
		}
	}
	t.Logf("%d comparisons", c.count)
}

func TestOracleOnMalformedFiles(t *testing.T) {
	c := &comparison{t: t}
	files := malformedFiles(t)
	for _, file := range files {
		if c.reads(file.name, file.data, objmod.Simple) == nil {
			t.Errorf("%s: the other tree reads it", file.name)
		}
		if c.appends(file.name+", appended to", file.data, objmod.Simple, appended(objmod.Simple)) == nil {
			t.Errorf("%s: the other tree appends to it", file.name)
		}
		c.reads(file.name+" as a leveled table", file.data, objmod.Leveled)
	}
	t.Logf("%d files, %d comparisons", len(files), c.count)
}

// TestOracleOnFilesCutAtEveryLength proves that both trees report the same first problem wherever a file ends.
func TestOracleOnFilesCutAtEveryLength(t *testing.T) {
	c := &comparison{t: t}
	files, cuts := wholeFiles(t), 0
	for _, file := range files {
		for length := range len(file.data) {
			what := fmt.Sprintf("%s cut at %d bytes", file.name, length)
			cut := file.data[:length:length]
			if c.reads(what, cut, file.kind) == nil {
				t.Errorf("%s: the other tree reads it", what)
			}
			if c.appends(what+", appended to", cut, file.kind, appended(file.kind)) == nil {
				t.Errorf("%s: the other tree appends to it", what)
			}
			cuts++
		}
	}
	t.Logf("%d files, %d cuts, %d comparisons", len(files), cuts, c.count)
}

// changed is a file with one thing in it changed, at the offset at.
type changed struct {
	input
	at int
}

// alteredFiles returns a file once for each value of each number in it that decides how the rest is read: a count
// of objects, of sets or of modifications and a value type, each set to values at and past the edges of what is
// valid. It also returns the file once for each of its texts with a first byte that is not UTF-8, and once
// without the text's NUL.
func alteredFiles(t *testing.T, file input) []changed {
	t.Helper()
	counts := []int32{-1, 0, 1, 2, 1000, math.MaxInt32, math.MinInt32}
	setCounts := []int32{-1, 0, 2, 64, 65}
	types := []int32{-1, 0, 1, 2, 3, 4}
	var files []changed
	number := func(what string, at int, values []int32) {
		for _, value := range values {
			name := fmt.Sprintf("%s with %s at %d set to %d", file.name, what, at, value)
			files = append(files, changed{input{name, testkit.SetU32(file.data, at, uint32(value)), file.kind}, at})
		}
	}
	oneByte := func(what string, at int, value byte) {
		data := bytes.Clone(file.data)
		data[at] = value
		files = append(files, changed{input{fmt.Sprintf("%s with %s at %d", file.name, what, at), data, file.kind}, at})
	}
	parsed := mustRead(t, file.data, file.kind, file.name)
	for _, table := range []objmod.Table{parsed.Original, parsed.Custom} {
		number("the object count", table.CountOffset, counts)
		for _, object := range table.Objects {
			at := object.Start + 8 // after the base and the id
			if parsed.Version >= 3 {
				number("the set count", at, setCounts)
				at += 4
			}
			for _, set := range object.Sets {
				if parsed.Version >= 3 {
					at += 4 // the flag
				}
				number("the modification count", at, counts)
				at += 4
				for _, mod := range set.Mods {
					number("the value type", mod.Start+4, types)
					if mod.Value.Type == objmod.String {
						// The text, its NUL and the end token are the last bytes of the modification.
						oneByte("a text that is not UTF-8", mod.Stop-5-len(mod.Value.Text), 0xFF)
						oneByte("a text without its NUL", mod.Stop-5, 'x')
					}
					at = mod.Stop
				}
			}
		}
	}
	return files
}

// TestOracleOnAlteredFiles compares files with one wrong number or text, whole and cut at each of the lengths
// just after the change: the bytes may run out between a wrong value and the place where it is checked, and both
// trees must then name the same problem.
func TestOracleOnAlteredFiles(t *testing.T) {
	c := &comparison{t: t}
	files, altered, cuts, read := wholeFiles(t), 0, 0, 0
	for _, file := range files {
		for _, change := range alteredFiles(t, file) {
			altered++
			if c.reads(change.name, change.data, file.kind) == nil {
				read++
				if err := c.appends(change.name+", appended to", change.data, file.kind, appended(file.kind)); err != nil {
					t.Errorf("%s: the other tree reads it and does not append to it: %v", change.name, err)
				}
			}
			for length := change.at + 1; length < min(len(change.data), change.at+25); length++ {
				c.reads(fmt.Sprintf("%s, cut at %d bytes", change.name, length), change.data[:length:length], file.kind)
				cuts++
			}
		}
	}
	if read < altered/20 || read > altered*19/20 {
		t.Errorf("%d of %d altered files still read, which leaves one of the two outcomes untried", read, altered)
	}
	t.Logf("%d altered files of %d, %d of them still read, %d cuts, %d comparisons",
		altered, len(files), read, cuts, c.count)
}

// mutated is data with one to three changes at random places: a byte set, a number set to a value at an edge of
// what a count, a type or a version may be, a byte put in or taken out.
func mutated(random *rand.Rand, data []byte) []byte {
	edges := []uint32{0, 1, 2, 3, 4, 5, 64, 65, 0xFFFFFFFF, 0x7FFFFFFF, 0x80000000, 0x7FC00000}
	data = bytes.Clone(data)
	for range 1 + random.IntN(3) {
		at := random.IntN(len(data))
		switch random.IntN(5) {
		case 0:
			data[at] = byte(random.IntN(256))
		case 1:
			data[at] = []byte{0, 1, 3, 0x7F, 0x80, 0xC3, 0xFF}[random.IntN(7)]
		case 2:
			copy(data[at:], testkit.U32(edges[random.IntN(len(edges))]))
		case 3:
			data = slices.Insert(data, at, byte(random.IntN(256)))
		case 4:
			data = slices.Delete(data, at, at+1)
		}
		if len(data) == 0 {
			return data
		}
	}
	return data
}

func TestOracleOnMutatedFiles(t *testing.T) {
	c := &comparison{t: t}
	random := rand.New(rand.NewPCG(3, 2026))
	files, mutations, read := wholeFiles(t), 0, 0
	for _, file := range files {
		for i := range 400 {
			what := fmt.Sprintf("%s, mutation %d", file.name, i)
			data := mutated(random, file.data)
			mutations++
			if c.reads(what, data, file.kind) != nil {
				continue
			}
			read++
			if err := c.appends(what+", appended to", data, file.kind, appended(file.kind)); err != nil {
				t.Errorf("%s: the other tree reads it and does not append to it: %v", what, err)
			}
		}
	}
	if read < mutations/20 || read > mutations*19/20 {
		t.Errorf("%d of %d mutations still read, which leaves one of the two outcomes untried", read, mutations)
	}
	t.Logf("%d mutations of %d files, %d of them still read, %d comparisons", mutations, len(files), read, c.count)
}

// TestOracleOnObjectsThatCannotBeWritten appends what both trees refuse: a text with a NUL or bytes that are not
// UTF-8, a real that is infinite or not a number, and a level or a column in a simple table. No refusal is a diag
// error, and the first modification that cannot be written is the one named.
func TestOracleOnObjectsThatCannotBeWritten(t *testing.T) {
	c := &comparison{t: t}
	unam := id("unam")
	text := func(s string) objmod.NewMod { return objmod.NewMod{Field: unam, Value: textValue(s)} }
	notANumber, infinite := float32(math.NaN()), float32(math.Inf(1))
	nan := objmod.NewMod{Field: id("umvs"), Value: realValue(notANumber)}
	above := objmod.NewMod{Field: id("ucbs"), Value: unrealValue(infinite)}
	below := objmod.NewMod{Field: id("umvs"), Value: realValue(-infinite)}
	levelOnReal := objmod.NewMod{Field: id("umvs"), Column: 2, Value: unrealValue(notANumber)}
	level := objmod.NewMod{Field: id("ulev"), Level: 1, Value: intValue(1)}
	column := objmod.NewMod{Field: id("ucol"), Column: -2, Value: realValue(1)}
	good := objmod.NewMod{Field: id("uhpm"), Value: intValue(5)}
	both := objmod.NewMod{Field: unam, Level: math.MinInt32, Column: math.MaxInt32, Value: intValue(1)}
	levelOnText := objmod.NewMod{Field: unam, Level: 4, Value: textValue("first\x00")}
	object := func(mods ...objmod.NewMod) objmod.NewObject {
		return objmod.NewObject{Base: id("hfoo"), ID: id("X001"), Mods: mods}
	}
	one := func(mods ...objmod.NewMod) []objmod.NewObject { return []objmod.NewObject{object(mods...)} }
	cases := []struct {
		name    string
		kind    objmod.TableKind
		objects []objmod.NewObject
	}{
		{"a text with a NUL", objmod.Leveled, one(text("first\x00b"))},
		{"a text that is only a NUL", objmod.Leveled, one(text("\x00"))},
		{"a text that is not UTF-8", objmod.Leveled, one(text("second\xFFb"))},
		{"a text with half a surrogate pair", objmod.Leveled, one(text("third\xED\xA0\x80"))},
		{"a text cut inside a character", objmod.Simple, one(text("fourth\xC3"))},
		{"a level in a simple table", objmod.Simple, one(level)},
		{"a column in a simple table", objmod.Simple, one(column)},
		{"a level and a column in a simple table", objmod.Simple, one(both)},
		{"two texts, the one with a NUL first", objmod.Leveled, one(good, text("first\x00"), text("second\xFF"))},
		{"two texts, the one with a NUL last", objmod.Leveled, one(text("second\xFF"), good, text("first\x00"))},
		{"a text and then a level", objmod.Simple, one(text("first\x00"), level)},
		{"a level and then a text", objmod.Simple, one(level, text("first\x00"))},
		{"a column and then a level", objmod.Simple, one(column, level)},
		{"a level on a text that cannot be written", objmod.Simple, one(levelOnText)},
		{"a text in the third object", objmod.Leveled,
			[]objmod.NewObject{object(good), object(), object(text("second\xFF"))}},
		{"a level in the second object and a text in the first", objmod.Simple,
			[]objmod.NewObject{object(text("fifth\xFF")), object(level)}},
		{"a level in the first object and a text in the second", objmod.Simple,
			[]objmod.NewObject{object(level), object(text("fifth\xFF"))}},
		{"a real that is not a number", objmod.Leveled, one(nan)},
		{"an unreal that is infinite", objmod.Simple, one(above)},
		{"a real that is infinite below zero", objmod.Leveled, one(below)},
		{"an unreal that is not a number", objmod.Simple,
			one(objmod.NewMod{Field: unam, Value: unrealValue(math.Float32frombits(0xFFC00001))})},
		{"three reals, the one that is not a number first", objmod.Leveled, one(good, nan, above, below)},
		{"three reals, the one that is not a number last", objmod.Leveled, one(below, good, above, nan)},
		{"a text and then a real", objmod.Leveled, one(text("first\x00"), above)},
		{"a real and then a text", objmod.Simple, one(above, text("first\x00"))},
		{"a real and then a column", objmod.Simple, one(below, column)},
		{"a column and then a real", objmod.Simple, one(column, below)},
		{"a column on a real that cannot be written", objmod.Simple, one(levelOnReal)},
		{"a real in the second object", objmod.Leveled, []objmod.NewObject{object(good), object(good, nan)}},
	}
	for _, x := range cases {
		sources := []input{{name: "no source"}}
		for _, file := range wholeFiles(t) {
			if file.kind == x.kind {
				sources = append(sources, file)
			}
		}
		for _, source := range sources {
			what := x.name + ", appended to " + source.name
			err := c.appends(what, source.data, x.kind, x.objects)
			if _, expected := olddiag.First(err); err == nil || expected {
				t.Errorf("%s: the other tree's error is %v, want one that is not a diag error", what, err)
			}
		}
	}
	// A source that does not read is refused before the objects are looked at.
	cut := fixture(t, "war3mapSkin.w3u")[:20]
	err := c.appends("a text with a NUL, appended to a cut source", cut, objmod.Simple, cases[0].objects)
	if _, expected := olddiag.First(err); !expected {
		t.Errorf("a cut source: the other tree's error is %v, want a diag error", err)
	}
	t.Logf("%d cases, %d comparisons", len(cases), c.count)
}

// TestWhatTheOtherTreeWritesAndThisTreeRefuses records the two places where Append differs from the other tree
// on purpose, so it compares nothing through the oracle. Both are bugs in the caller that the other tree writes
// into the file without an error:
//
//   - a value whose type is none of the four, which the other tree writes as type -1 with the number as a real,
//     and which neither tree reads back;
//   - an id of four NUL bytes as the base or the id of an object or as the field of a modification, which is an
//     id the caller forgot to set.
func TestWhatTheOtherTreeWritesAndThisTreeRefuses(t *testing.T) {
	noType := oldobjects.ModValue{Type: "bool", Number: 1.5, Text: "a text"}
	one := oldobjects.IntValue(1)
	for _, c := range []struct {
		name      string
		other     oldobjects.NewObject
		this      objmod.NewObject
		words     string
		readsBack bool // whether the file the other tree writes is one that reads
	}{
		{"a value type that is none of the four",
			oldobjects.NewObject{Base: "hfoo", ID: "X001", Mods: []oldobjects.NewMod{{Field: "unam", Value: noType}}},
			oneMod(objmod.NewMod{Field: id("unam"), Value: objmod.Value{Type: -1, Real: 1.5, Text: "a text"}})[0],
			"value type -1", false},
		{"a base of four NULs",
			oldobjects.NewObject{Base: nulls, ID: "X001"},
			objmod.NewObject{ID: id("X001")}, "its base is four NUL bytes", true},
		{"an id of four NULs",
			oldobjects.NewObject{Base: "hfoo", ID: nulls},
			objmod.NewObject{Base: id("hfoo")}, "its id is four NUL bytes", true},
		{"a field of four NULs",
			oldobjects.NewObject{Base: "hfoo", ID: "X001", Mods: []oldobjects.NewMod{{Field: nulls, Value: one}}},
			oneMod(objmod.NewMod{Value: intValue(1)})[0], "its field is four NUL bytes", true},
	} {
		for _, kind := range kinds {
			what := fmt.Sprintf("%s in a table of kind %d", c.name, kind)
			written, err := oldobjects.AppendObjects(nil, otherKind(kind), []oldobjects.NewObject{c.other}, modFile)
			if err != nil || len(written) == 0 {
				t.Errorf("%s: the other tree writes nothing, so this is no difference any more: %v", what, err)
			}
			if _, err := objmod.Read(written, kind, modFile); (err == nil) != c.readsBack {
				t.Errorf("%s: reading what the other tree wrote gives the error %v", what, err)
			}
			data, err := objmod.Append(nil, kind, []objmod.NewObject{c.this}, modFile)
			var fileError *diag.Error
			if err == nil || errors.As(err, &fileError) || !strings.Contains(err.Error(), c.words) {
				t.Errorf("%s: this tree's error is %v, want one with %q that is not a *diag.Error", what, err, c.words)
			}
			if data != nil {
				t.Errorf("%s: this tree's refusal returned % X", what, data)
			}
		}
	}
}
