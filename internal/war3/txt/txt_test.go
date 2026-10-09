package txt_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/war3/txt"
)

type mergeCase struct {
	name     string
	source   string
	sections []txt.Section
	want     string
}

func section(name string, keysAndValues ...string) txt.Section {
	made := txt.Section{Name: name}
	for i := 0; i < len(keysAndValues); i += 2 {
		made.Fields = append(made.Fields, txt.Field{Key: keysAndValues[i], Value: keysAndValues[i+1]})
	}
	return made
}

func sections(list ...txt.Section) []txt.Section { return list }

var lineBreaks = []struct{ name, text string }{{"LF", "\n"}, {"CRLF", "\r\n"}}

func checkMerges(t *testing.T, merges []mergeCase) {
	t.Helper()
	for _, m := range merges {
		if got := txt.Merge(m.source, m.sections); got != m.want {
			t.Errorf("%s: Merge(%q) = %q, want %q", m.name, m.source, got, m.want)
		}
	}
}

func duplicateKeyMerges() []mergeCase {
	wanted := sections(section("Misc", "MaxHeroLevel", "25", "Added", "0"), section("CustomSkin", "Text", ""))
	merges := []mergeCase{{"an empty source", "", sections(section("Misc", "FoodCeiling", "0")), "[Misc]\nFoodCeiling=0"}}
	for _, nl := range lineBreaks {
		source := strings.Join([]string{
			"// keep", "[Misc]", "MaxHeroLevel=10", "Keep=42", "[Other]", "X=y", "[misc]", "maxherolevel=12", "",
		}, nl.text)
		want := strings.Join([]string{
			"// keep", "[Misc]", "MaxHeroLevel=25", "Keep=42", "[Other]", "X=y", "[misc]", "maxherolevel=25", "Added=0",
			"", "[CustomSkin]", "Text=", "",
		}, nl.text)
		merges = append(merges, mergeCase{"a key in two sections of one name, " + nl.name, source, wanted, want})
	}
	return merges
}

func TestMergeKeepsUnrelatedContentAndSetsEveryDuplicateKey(t *testing.T) {
	checkMerges(t, duplicateKeyMerges())
}

func tabAndCaseMerges() []mergeCase {
	return []mergeCase{{
		"a tab-indented key under a header in other letters",
		"[mIsC] // keep heading comment\r\n\tfoodceiling = 100\r\n",
		sections(section("Misc", "FoodCeiling", "200")),
		"[mIsC] // keep heading comment\r\n\tfoodceiling=200\r\n",
	}}
}

func TestMergeRecognizesTabIndentedKeysAndCaseOnlyMatches(t *testing.T) {
	checkMerges(t, tabAndCaseMerges())
}

func emptyAndMissingSectionMerges() []mergeCase {
	return []mergeCase{{
		"two empty sections and a missing one",
		"[Misc]\n[Skin]\n",
		sections(section("Misc", "FoodCeiling", "0"), section("Skin", "Text", ""), section("New", "Value", "1")),
		"[Misc]\nFoodCeiling=0\n[Skin]\nText=\n\n[New]\nValue=1\n",
	}}
}

func TestMergeAddsKeysToEmptySectionsAndCreatesMissingSections(t *testing.T) {
	checkMerges(t, emptyAndMissingSectionMerges())
}

func wholeLineMerges() []mergeCase {
	return []mergeCase{{
		"a value followed by a note",
		"[Misc]\nFoodCeiling=100 ; stale note\n",
		sections(section("Misc", "FoodCeiling", "200")),
		"[Misc]\nFoodCeiling=200\n",
	}}
}

func TestMergeReplacesTheEntireExistingValueLine(t *testing.T) {
	checkMerges(t, wholeLineMerges())
}

func layoutMerges() []mergeCase {
	var merges []mergeCase
	for _, nl := range lineBreaks {
		lines := func(parts ...string) string { return strings.Join(parts, nl.text) }
		merges = append(merges,
			mergeCase{"a new key follows the section's last entry, and the final newline stays, " + nl.name,
				lines("[Misc]", "A=1", ""), sections(section("Misc", "B", "2")), lines("[Misc]", "A=1", "B=2", "")},
			mergeCase{"a blank line between sections stays between them, not before the new key, " + nl.name,
				lines("[A]", "X=1", "", "[B]", "Y=2", ""), sections(section("A", "K", "v")),
				lines("[A]", "X=1", "K=v", "", "[B]", "Y=2", "")},
			mergeCase{"a new section gets one blank line before it and keeps the final newline, " + nl.name,
				lines("[A]", "X=1", ""), sections(section("New", "K", "v")), lines("[A]", "X=1", "", "[New]", "K=v", "")},
			mergeCase{"no second blank line when the source ends with one, " + nl.name,
				lines("[A]", "X=1", "", ""), sections(section("New", "K", "v")), lines("[A]", "X=1", "", "[New]", "K=v", "")},
			mergeCase{"no final newline is added to a source without one, " + nl.name,
				lines("[A]", "X=1"), sections(section("New", "K", "v")), lines("[A]", "X=1", "", "[New]", "K=v")},
		)
	}
	return merges
}

func TestMergeKeepsTheLayout(t *testing.T) {
	checkMerges(t, layoutMerges())
}

func headerCommentMerges() []mergeCase {
	var merges []mergeCase
	for _, header := range []string{"[Misc] ; comment", "[Misc]; comment", "[Misc] // comment", "  [Misc]\t;"} {
		merges = append(merges, mergeCase{"the header " + header, header + "\nA=1\n", sections(section("Misc", "B", "2")),
			header + "\nA=1\nB=2\n"})
	}
	return merges
}

func TestSectionHeadersFollowedByCommentsAreRecognised(t *testing.T) {
	checkMerges(t, headerCommentMerges())
}

func edgeMerges() []mergeCase {
	return []mergeCase{
		{"a source of one line break", "\n", sections(section("New", "K", "v")), "\n[New]\nK=v\n"},
		{"a source of one line without a break", "// note", sections(section("New", "K", "v")), "// note\n\n[New]\nK=v"},
		{"mixed line breaks become CRLF", "[A]\nX=1\r\nY=2\n", sections(section("A", "Z", "3")),
			"[A]\r\nX=1\r\nY=2\r\nZ=3\r\n"},
		{"a key twice in one section", "[A]\nK=1\nX=0\nk = 2\n", sections(section("A", "K", "9")), "[A]\nK=9\nX=0\nk=9\n"},
		{"a new key goes to the last section of the name", "[A]\nX=1\n[B]\n[a]\nY=2\n", sections(section("A", "N", "3")),
			"[A]\nX=1\n[B]\n[a]\nY=2\nN=3\n"},
		{"a new key goes before the blank lines that end its section", "[A]\nX=1\n\n \t\n[B]\n",
			sections(section("A", "K", "v")), "[A]\nX=1\nK=v\n\n \t\n[B]\n"},
		{"a new key goes after a comment line of its section", "[A]\nX=1\n// note\n\n[B]\n", sections(section("A", "K", "v")),
			"[A]\nX=1\n// note\nK=v\n\n[B]\n"},
		{"a second field finds the section the first one made", "", sections(section("New", "A", "1", "B", "2")),
			"[New]\nA=1\nB=2"},
		{"a later section finds a section made earlier in the call", "", sections(section("New", "A", "1"), section("NEW", "B", "2")),
			"[New]\nA=1\nB=2"},
		{"sections and fields are applied in the order given", "[B]\n[A]\n",
			sections(section("C", "K", "1"), section("A", "Y", "2", "X", "3"), section("B", "Z", "4"), section("D", "K", "5")),
			"[B]\nZ=4\n[A]\nY=2\nX=3\n\n[C]\nK=1\n\n[D]\nK=5\n"},
		{"lines before the first header are in no section", "K=1\n[A]\n", sections(section("A", "K", "2")), "K=1\n[A]\nK=2\n"},
		{"the same key in another section is left alone", "[A]\nK=1\n[B]\nK=1\n", sections(section("B", "K", "2")),
			"[A]\nK=1\n[B]\nK=2\n"},
		{"a section without fields adds nothing", "[A]\n", sections(section("New")), "[A]\n"},
		{"no sections", "[A]\nK=1", nil, "[A]\nK=1"},
		{"an unclosed header is no header", "[A\nK=1\n", sections(section("A", "K", "2")), "[A\nK=1\n\n[A]\nK=2\n"},
		{"text after a header that is no comment", "[A] x\nK=1\n", sections(section("A", "K", "2")),
			"[A] x\nK=1\n\n[A]\nK=2\n"},
		{"a header's name is compared with its spaces", "[ A ]\nK=1\n", sections(section("A", "K", "2")),
			"[ A ]\nK=1\n\n[A]\nK=2\n"},
		{"a line without an equals sign sets no key", "[A]\nK\n", sections(section("A", "K", "2")), "[A]\nK\nK=2\n"},
		{"a value is written as given", "[A]\nK=1\n", sections(section("A", "K", " x ; y = z ")), "[A]\nK= x ; y = z \n"},
		{"a form feed is white space", "\f[A]\f\n\fK\f=1\n", sections(section("A", "K", "2")), "\f[A]\f\n\fK=2\n"},
		{"a vertical tab before a header", "\v[Misc]\nA=1\n", sections(section("Misc", "B", "2")), "\v[Misc]\nA=1\nB=2\n"},
		{"a vertical tab after a header and around a key", "[Misc]\v\n\vA\v=\v1\n", sections(section("Misc", "A", "2")),
			"[Misc]\v\n\vA=2\n"},
		{"a vertical tab before a key", "[Misc]\n\vA=1\n", sections(section("Misc", "A", "2")), "[Misc]\n\vA=2\n"},
		{"a line of one vertical tab inside a section", "[Misc]\nA=1\n\v\n[Other]\n", sections(section("Misc", "B", "2")),
			"[Misc]\nA=1\nB=2\n\v\n[Other]\n"},
		{"a last line of one vertical tab", "[A]\nX=1\n\v", sections(section("Misc", "B", "2")), "[A]\nX=1\n\v\n[Misc]\nB=2"},
		{"a carriage return that ends no line is white space", "[A]\r\r\nK=1\r\n\r\r\n[B]\r\n", sections(section("A", "N", "2")),
			"[A]\r\r\nK=1\r\nN=2\r\n\r\r\n[B]\r\n"},
		{"a carriage return in a header's comment", "[A] ; a\rb\nK=1\n", sections(section("A", "K", "2")),
			"[A] ; a\rb\nK=1\n\n[A]\nK=2\n"},
	}
}

func TestMergeAtTheEdgesOfTheLayout(t *testing.T) {
	checkMerges(t, edgeMerges())
}

func settledMerges() []mergeCase {
	return slices.Concat(duplicateKeyMerges(), tabAndCaseMerges(), emptyAndMissingSectionMerges(), wholeLineMerges(),
		layoutMerges(), headerCommentMerges(), edgeMerges())
}

type repeatedMergeCase struct {
	mergeCase
	again string
}

func regrowingMerges() []repeatedMergeCase {
	return []repeatedMergeCase{
		{mergeCase{"a key with a space", "[A]\n", sections(section("A", "my key", "1")), "[A]\nmy key=1\n"},
			"[A]\nmy key=1\nmy key=1\n"},
		{mergeCase{"a key with an equals sign", "[A]\n", sections(section("A", "a=b", "1")), "[A]\na=b=1\n"},
			"[A]\na=b=1\na=b=1\n"},
		{mergeCase{"a section name with a closing bracket", "", sections(section("A]B", "K", "1")), "[A]B]\nK=1"},
			"[A]B]\nK=1\n\n[A]B]\nK=1"},
		{mergeCase{"a value with a line break", "[A]\nK=1\n", sections(section("A", "K", "1\n[B]"), section("B", "X", "2")),
			"[A]\nK=1\n[B]\n\n[B]\nX=2\n"}, "[A]\nK=1\n[B]\n[B]\n\n[B]\nX=2\n"},
	}
}

func TestMergingAgainAddsWhatItCannotFindInItsOwnText(t *testing.T) {
	for _, r := range regrowingMerges() {
		checkMerges(t, []mergeCase{r.mergeCase, {r.name + ", merged again", r.want, r.sections, r.again}})
	}
}

func carriedMerges() []mergeCase {
	carried := settledMerges()
	for _, r := range regrowingMerges() {
		carried = append(carried, r.mergeCase)
	}
	return carried
}

func TestMergeTakesTheSameNameTwice(t *testing.T) {
	checkMerges(t, []mergeCase{
		{"a key twice in one section: the last value stays", "", sections(section("A", "K", "1", "K", "2")), "[A]\nK=2"},
		{"a section twice", "[A]\nK=0\n", sections(section("A", "K", "1"), section("B", "X", "2"), section("A", "K", "3", "N", "4")),
			"[A]\nK=3\nN=4\n\n[B]\nX=2\n"},
	})
}

func TestMergingAgainChangesNothing(t *testing.T) {
	for _, m := range settledMerges() {
		if again := txt.Merge(m.want, m.sections); again != m.want {
			t.Errorf("%s: merging %q twice gives %q", m.name, m.want, again)
		}
	}
}

func TestMergingNoSectionsReturnsTheSource(t *testing.T) {
	sources := []string{"", "\n", "\r\n", "\n\n", "a", "a\n", "a\r\n", "a\r\nb", "\n[A]\n\nK = 1 ; note\n\n\n", " \t\n", "a\r", "a\r\r\n"}
	for _, m := range carriedMerges() {
		if lf := strings.ReplaceAll(m.source, "\r\n", ""); !strings.Contains(m.source, "\r\n") || !strings.Contains(lf, "\n") {
			sources = append(sources, m.source)
		}
	}
	for _, source := range sources {
		for _, none := range [][]txt.Section{nil, {}, sections(section("A"))} {
			if same := txt.Merge(source, none); same != source {
				t.Errorf("Merge(%q) with %d sections = %q", source, len(none), same)
			}
		}
	}
}

func whiteSpaceMerges() []mergeCase {
	add, set := sections(section("Misc", "B", "2")), sections(section("Misc", "A", "2"))
	return []mergeCase{
		{"a no-break space before a header", "\xC2\xA0[Misc]\nA=1\n", add, "\xC2\xA0[Misc]\nA=1\n\n[Misc]\nB=2\n"},
		{"a no-break space after a header", "[Misc]\xC2\xA0\nA=1\n", add, "[Misc]\xC2\xA0\nA=1\n\n[Misc]\nB=2\n"},
		{"a no-break space before a key", "[Misc]\n\xC2\xA0A=1\n", set, "[Misc]\n\xC2\xA0A=1\nA=2\n"},
		{"a no-break space after a key", "[Misc]\nA\xC2\xA0=1\n", set, "[Misc]\nA\xC2\xA0=1\nA=2\n"},
		{"a no-break space inside a key", "[Misc]\nA\xC2\xA0B=1\n", sections(section("Misc", "A\xC2\xA0B", "2")),
			"[Misc]\nA\xC2\xA0B=2\n"},
		{"a line of one no-break space inside a section", "[Misc]\nA=1\n\xC2\xA0\n[Other]\n", add,
			"[Misc]\nA=1\n\xC2\xA0\nB=2\n[Other]\n"},
		{"a last line of one no-break space", "[A]\nX=1\n\xC2\xA0", add, "[A]\nX=1\n\xC2\xA0\n\n[Misc]\nB=2"},
		{"an ideographic space before a header", "\xE3\x80\x80[Misc]\nA=1\n", add, "\xE3\x80\x80[Misc]\nA=1\n\n[Misc]\nB=2\n"},
		{"a byte order mark before a header", "\xEF\xBB\xBF[Misc]\nA=1\n", add, "\xEF\xBB\xBF[Misc]\nA=1\n\n[Misc]\nB=2\n"},
	}
}

func TestOnlyASCIIWhiteSpaceIsWhiteSpace(t *testing.T) {
	checkMerges(t, whiteSpaceMerges())
}

func separatorMerges() []mergeCase {
	add := sections(section("Misc", "B", "2"))
	return []mergeCase{
		{"a line separator in a header's comment", "[Misc] ; a\xE2\x80\xA8b\nA=1\n", add, "[Misc] ; a\xE2\x80\xA8b\nA=1\nB=2\n"},
		{"a paragraph separator in a header's comment", "[Misc] // a\xE2\x80\xA9b\nA=1\n", add,
			"[Misc] // a\xE2\x80\xA9b\nA=1\nB=2\n"},
	}
}

func TestALineOrParagraphSeparatorIsPartOfAHeadersComment(t *testing.T) {
	checkMerges(t, separatorMerges())
}

func caseFoldingMerges() []mergeCase {
	return []mergeCase{
		{"a long s is an s", "[Misc]\nMa\xC5\xBF=1\n", sections(section("Misc", "MAS", "2")), "[Misc]\nMa\xC5\xBF=2\n"},
		{"a final sigma is a sigma", "[\xCE\x9F\xCE\x94\xCE\x9F\xCE\xA3]\nA=1\n",
			sections(section("\xCE\xBF\xCE\xB4\xCE\xBF\xCF\x82", "B", "2")), "[\xCE\x9F\xCE\x94\xCE\x9F\xCE\xA3]\nA=1\nB=2\n"},
		{"a dotted capital I is not an i with a dot above", "[\xC4\xB0]\nA=1\n", sections(section("i\xCC\x87", "B", "2")),
			"[\xC4\xB0]\nA=1\n\n[i\xCC\x87]\nB=2\n"},
	}
}

func TestNamesMatchByCaseFoldingOutsideASCIIToo(t *testing.T) {
	checkMerges(t, caseFoldingMerges())
	checkMerges(t, []mergeCase{{"an accented letter in both cases", "[\xC3\x89t\xC3\xA9]\nA=1\n",
		sections(section("\xC3\xA9T\xC3\x89", "a", "2")), "[\xC3\x89t\xC3\xA9]\nA=2\n"}})
}
