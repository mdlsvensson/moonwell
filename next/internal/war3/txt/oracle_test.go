package txt_test

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/ordered"
	oldsettings "github.com/mdlsvensson/moonwell/internal/settings"
	"github.com/mdlsvensson/moonwell/next/internal/oracle"
	"github.com/mdlsvensson/moonwell/next/internal/war3/txt"
)

// What this file leaves out, and why. The two trees agree on every text and every name made of ASCII, and those
// are compared here. They differ by intent in three things. The inputs that show a difference are pinned in
// txt_test.go and given to neither tree in a comparison; TestOracleLeavesOutOnlyWhatTheTreesMergeDifferently proves
// of each that the other tree makes another text of it.
//
//   - White space outside ASCII. The other tree takes the no-break space, the other spaces outside ASCII, the byte
//     order mark and the line and paragraph separators for white space, and this tree takes them for text. Left
//     out: a source with one of those characters beside a header or a key or alone on a line, and a key that holds
//     one (TestOnlyASCIIWhiteSpaceIsWhiteSpace). The same characters inside a value, a comment or a section name
//     are compared.
//   - A line or paragraph separator inside the comment that follows a header. The other tree takes such a line
//     for no header and this tree takes it for one (TestALineOrParagraphSeparatorIsPartOfAHeadersComment).
//   - Letter case outside ASCII. The other tree compares two names by their small letters and this tree by case
//     folding. Left out: the long s against s, the final sigma against sigma, and the dotted capital I against an
//     i with a dot above (TestNamesMatchByCaseFoldingOutsideASCIIToo). Other letters outside ASCII are compared.
//
// A list that holds a section or a key twice in one spelling cannot be given to the other tree, whose sections are
// maps: TestMergeTakesTheSameNameTwice covers it for this tree alone.

// comparison merges with the other tree's package and with this one, compares the two texts through the oracle,
// and counts the comparisons it made and the merges that changed their source.
type comparison struct {
	t       *testing.T
	count   int
	changed int
}

// otherTreeSections are the sections as the other tree takes them. It stops the test when its maps cannot hold
// them in the order given: a name that comes twice, or one its maps move to the front.
func otherTreeSections(t *testing.T, what string, sections []txt.Section) oldsettings.Sections {
	t.Helper()
	var held oldsettings.Sections
	var names []string
	for _, section := range sections {
		fields := &ordered.Map[string]{}
		var keys []string
		for _, field := range section.Fields {
			fields.Set(field.Key, field.Value)
			keys = append(keys, field.Key)
		}
		if !slices.Equal(fields.Keys(), keys) {
			t.Fatalf("%s: the other tree holds the keys of %s as %q, not as %q", what, section.Name, fields.Keys(), keys)
		}
		held.Set(section.Name, fields)
		names = append(names, section.Name)
	}
	if !slices.Equal(held.Keys(), names) {
		t.Fatalf("%s: the other tree holds the sections as %q, not as %q", what, held.Keys(), names)
	}
	return held
}

func (c *comparison) texts(what, want, got string) {
	c.t.Helper()
	c.count++
	oracle.Bytes(c.t, what, []byte(want), []byte(got))
}

// merges compares the text both trees make of the source, and the text both make of that text when it is merged
// a second time.
func (c *comparison) merges(what, source string, sections []txt.Section) {
	c.t.Helper()
	held := otherTreeSections(c.t, what, sections)
	want := oldsettings.PatchText(source, held)
	c.texts(what, want, txt.Merge(source, sections))
	c.texts(what+", merged again", oldsettings.PatchText(want, held), txt.Merge(want, sections))
	if want != source {
		c.changed++
	}
}

// TestOracleLeavesOutOnlyWhatTheTreesMergeDifferently keeps the list above honest: every merge that is pinned
// there and not compared gives another text in the other tree. One that comes to give the same text belongs in a
// comparison.
func TestOracleLeavesOutOnlyWhatTheTreesMergeDifferently(t *testing.T) {
	leftOut := slices.Concat(whiteSpaceMerges(), separatorMerges(), caseFoldingMerges())
	for _, m := range leftOut {
		if other := oldsettings.PatchText(m.source, otherTreeSections(t, m.name, m.sections)); other == m.want {
			t.Errorf("%s: both trees give %q, so the merge need not be left out", m.name, other)
		}
	}
	t.Logf("%d merges left out", len(leftOut))
}

func TestOracleOnTheCarriedMerges(t *testing.T) {
	c := &comparison{t: t}
	carried := carriedMerges()
	for _, m := range carried {
		c.merges(m.name, m.source, m.sections)
		c.merges(m.name+", no sections", m.source, nil)
	}
	if c.changed < len(carried)-5 {
		t.Errorf("only %d of %d merges changed their source", c.changed, len(carried))
	}
	t.Logf("%d merges, %d comparisons", 2*len(carried), c.count)
}

// shapedSources are sources in each shape a file may have: empty, with LF and with CRLF, with and without a final
// newline, a key and a section that come twice, headers followed by comments, indented keys, and names in other
// ASCII letter case than the sections ask for. The last bodies hold characters that are white space outside ASCII
// in the places where they are text to both trees: inside a value, a comment and a section name, and the
// next-line character, which is white space to neither, before a header.
func shapedSources() []string {
	bodies := [][]string{
		{},
		{""},
		{"", ""},
		{"[Misc]"},
		{"[Misc]", "FoodCeiling=100"},
		{"[Misc]", "FoodCeiling=100", "FoodCeiling=300"},
		{"[Misc]", "FoodCeiling=100", "", "[Skin]", "Text=old", "", "[misc]", "foodceiling=1", "Other=2"},
		{"[MISC] // constants", "\tFOODCEILING = 100", "\tMaxHeroLevel\t=\t10", "", "[skin] ; texts", "  text = old ; note"},
		{"[Misc];", "  [Skin]  //", "[New]\t; made by hand", "value=0"},
		{"// a file of notes", "; and nothing else", "", ""},
		{"FoodCeiling=100", "[Other]", "FoodCeiling=100", "Text=", "", "", ""},
		{"[Misc]", "", "", "[Skin]", "", ""},
		{"[Misc] not a header", "[Skin", "Skin]", "[]", "=1", "FoodCeiling", "[Misc]=1", "[Text=x]"},
		{"[Misc]", "Food Ceiling=1", "FoodCeiling==2", "FoodCeiling =", " \t "},
		{"[Sk\xC3\xA5n]", "T\xC3\xA9xt=m\xC3\xA5ne \xE6\x9C\x88", "[Misc]", "FoodCeiling=\xC3\x85"},
		{"\v[Misc]\v", "\vFoodCeiling\v=\v100", "\v", "\v[Skin]\v; texts", "Text=old", "\v \t\f"},
		{"[Misc]", "FoodCeiling=1\xC2\xA02", "// a\xC2\xA0note", "; b\xE2\x80\xA8c", "[Mi\xC2\xA0sc]", "Text=a\xC2\xA0b\xE3\x80\x80"},
		{"[Skin] ; a\xC2\xA0note", "Text=\xC2\xA0", "\xC2\x85[Misc]", "FoodCeiling=100"},
	}
	var sources []string
	for _, body := range bodies {
		for _, nl := range []string{"\n", "\r\n"} {
			sources = append(sources, strings.Join(body, nl), strings.Join(body, nl)+nl)
		}
	}
	return append(sources, "[Misc]\nFoodCeiling=100\r\n[Skin]\r\nText=old\n", "[Misc]\r\r\nFoodCeiling=100\r", "[Misc]\f\n\fText\f=\f1\n")
}

// shapedSections are section lists that set a key that is there, add a key to a section that is there, add a
// section, and do several of those in one call, so that the order they are applied in shows.
func shapedSections() [][]txt.Section {
	return [][]txt.Section{
		nil,
		sections(section("Misc")),
		sections(section("Misc", "FoodCeiling", "200")),
		sections(section("misc", "FOODCEILING", "200")),
		sections(section("Misc", "Added", "1")),
		sections(section("New", "Value", "1")),
		sections(section("Skin", "Text", "")),
		sections(section("Misc", "FoodCeiling", "200", "MaxHeroLevel", "25", "Added", "0"), section("Skin", "Text", "new", "More", "x=y"),
			section("New", "Value", "1", "Second", "2")),
		sections(section("New", "Value", "1"), section("Skin", "More", "1", "Text", "new"), section("Misc", "Added", "0", "FoodCeiling", "2")),
		sections(section("Misc", "FoodCeiling", "1"), section("MISC", "foodceiling", "2", "FoodCeiling", "3"), section("misc", "Z", "4")),
		sections(section("New", "a", "1", "A", "2"), section("NEW", "b", "3")),
		sections(section("Sk\xC3\xA5n", "T\xC3\xA9xt", "\xC3\xA5"), section("SK\xC3\x85N", "t\xC3\x89XT", "\xE6\x9C\x88", "\xC3\x96", "")),
		sections(section("Misc", "FoodCeiling", " 5 ; five"), section("Skin", "Text", "[x]")),
		sections(section("Skin", "Text", "one\ntwo"), section("two", "K", "v"), section("Misc", "FoodCeiling", "one\r\n[Late]"),
			section("Late", "K", "v")),
		sections(section("mi\xC2\xA0SC", "text", "new", "Added", "x\xC2\xA0y"), section("Misc", "FoodCeiling", "3\xC2\xA04")),
		// Names that are written and not found again: every merge adds them once more.
		sections(section("Misc", "Food Ceiling", "1", "a=b", "2"), section("A]B", "K", "1")),
	}
}

func TestOracleOnEveryShape(t *testing.T) {
	c := &comparison{t: t}
	sources, lists := shapedSources(), shapedSections()
	for i, source := range sources {
		for j, list := range lists {
			c.merges(fmt.Sprintf("source %d %q with sections %d", i, source, j), source, list)
		}
	}
	if merges := len(sources) * len(lists); c.changed < merges*3/4 {
		t.Errorf("only %d of %d merges changed their source", c.changed, merges)
	}
	t.Logf("%d sources, %d section lists, %d comparisons", len(sources), len(lists), c.count)
}

// generator makes sources and section lists of ASCII from a seed.
type generator struct{ random *rand.Rand }

func (g generator) pick(options ...string) string { return options[g.random.IntN(len(options))] }

var (
	generatedSections = []string{"Misc", "misc", "MISC", "Skin", "CustomSkin", "FrameDef", "A"}
	generatedKeys     = []string{"FoodCeiling", "foodceiling", "FOODCEILING", "MaxHeroLevel", "Text", "K", "k"}
	generatedValues   = []string{"", "0", "25", "a b", "x=y", "1 ; note", " lead", "trail ", "[v]"}
)

// space is white space that both trees take for white space, or none.
func (g generator) space() string {
	return g.pick("", "", "", " ", "\t", "  ", " \t", "\f", "\r", "\v")
}

// line is one line of a source: a header, a line that sets a key, a blank line, a comment, or something that is
// none of them.
func (g generator) line() string {
	switch g.random.IntN(10) {
	case 0, 1, 2:
		comment := g.pick("", "", "", "; note", "// note", ";", "//", "x", "; a\rb", "/ note")
		return g.space() + "[" + g.pick(generatedSections...) + "]" + g.space() + comment
	case 3, 4, 5, 6:
		return g.space() + g.pick(generatedKeys...) + g.space() + "=" + g.space() + g.pick(generatedValues...)
	case 7:
		return g.space()
	case 8:
		return g.pick("// keep", "; keep", "//", "// K=1", "; [Misc]")
	}
	return g.pick("no equals", "=novalue", "[unclosed", "[]", "[a]b=c", "][", "[Misc]=1", "[A=b]", "K", "[ Misc ]", "[Misc][Skin]")
}

// source is up to twelve lines, with LF, with CRLF or with both, and with no final newline, one or two.
func (g generator) source() string {
	style := g.random.IntN(3)
	newline := func() string {
		if style == 0 || style == 2 && g.random.IntN(2) == 0 {
			return "\n"
		}
		return "\r\n"
	}
	var text strings.Builder
	count := g.random.IntN(13)
	for i := range count {
		if i > 0 {
			text.WriteString(newline())
		}
		text.WriteString(g.line())
	}
	for range g.random.IntN(3) {
		text.WriteString(newline())
	}
	return text.String()
}

// distinct picks up to limit different names.
func (g generator) distinct(names []string, limit int) []string {
	var picked []string
	for _, i := range g.random.Perm(len(names))[:g.random.IntN(limit+1)] {
		picked = append(picked, names[i])
	}
	return picked
}

// sections is up to three sections of up to three fields each. No name comes twice in one spelling.
func (g generator) sections() []txt.Section {
	var list []txt.Section
	for _, name := range g.distinct(generatedSections, 3) {
		made := txt.Section{Name: name}
		for _, key := range g.distinct(generatedKeys, 3) {
			made.Fields = append(made.Fields, txt.Field{Key: key, Value: g.pick(generatedValues...)})
		}
		list = append(list, made)
	}
	return list
}

func TestOracleOnGeneratedTexts(t *testing.T) {
	c := &comparison{t: t}
	g := generator{rand.New(rand.NewPCG(11, 2026))}
	const merges = 800
	for i := range merges {
		source := g.source()
		c.merges(fmt.Sprintf("generated %d, source %q", i, source), source, g.sections())
	}
	if c.changed < merges/2 {
		t.Errorf("only %d of %d merges changed their source", c.changed, merges)
	}
	t.Logf("%d merges, %d of them changed their source, %d comparisons", merges, c.changed, c.count)
}
