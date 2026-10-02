package text

import (
	"slices"
	"testing"
)

func TestUTF16LenAndOffset(t *testing.T) {
	if got := UTF16Len("a🌙"); got != 3 {
		t.Errorf("UTF16Len = %d, want 3", got)
	}
	s := "-- 🌙\r\nfunction"
	if got := UTF16Offset(s, len("-- 🌙\r\n")); got != 7 {
		t.Errorf("UTF16Offset = %d, want 7", got)
	}
	if got := UTF16Offset("abc", 99); got != 3 {
		t.Errorf("UTF16Offset past the end = %d, want 3", got)
	}
}

func TestSortIsJavaScriptsOrder(t *testing.T) {
	// U+E000 sorts after an astral rune in UTF-16 units, and before it in bytes.
	got := []string{"", "🌙", "b", "a", "ab", ""}
	Sort(got)
	want := []string{"", "a", "ab", "b", "🌙", ""}
	if !slices.Equal(got, want) {
		t.Errorf("Sort = %q, want %q", got, want)
	}
	if Less("a", "a") || !Less("A", "a") || !Less("a", "ab") {
		t.Error("Less is wrong for plain ASCII")
	}
	if Compare("🌙", "🌛") >= 0 {
		t.Error("two astral runes with one lead unit compare by the trail unit")
	}
}

func TestQuoteIsJSONStringify(t *testing.T) {
	cases := map[string]string{
		"plain":           `"plain"`,
		`a"b\c`:           `"a\"b\\c"`,
		"line\nbreak\r\t": `"line\nbreak\r\t"`,
		"\a\b\f\x00\x1f":  `"\u0007\b\f\u0000\u001f"`,
		"<é>& ":           "\"<é>& \"",
		"src/héro.yue":    `"src/héro.yue"`,
	}
	for input, want := range cases {
		if got := Quote(input); got != want {
			t.Errorf("Quote(%q) = %s, want %s", input, got, want)
		}
	}
}

func TestSpaceAndTrim(t *testing.T) {
	for _, r := range []rune{' ', '\t', '\n', '\v', '\f', '\r', 0xA0, 0xFEFF, 0x2003, 0x2028, 0x3000} {
		if !IsSpace(r) {
			t.Errorf("IsSpace(%U) = false", r)
		}
	}
	for _, r := range []rune{'a', 0x200B, 0x85, 0} {
		if IsSpace(r) {
			t.Errorf("IsSpace(%U) = true", r)
		}
	}
	if got := Trim("\xEF\xBB\xBF  a b \r\n"); got != "a b" {
		t.Errorf("Trim = %q", got)
	}
}

func TestCaseMapping(t *testing.T) {
	if got := Upper("straße.txt"); got != "STRASSE.TXT" {
		t.Errorf("Upper = %q", got)
	}
	if got := Upper("war3map.lua"); got != "WAR3MAP.LUA" {
		t.Errorf("Upper = %q", got)
	}
	if got := Lower("İcon.BLP"); got != "i̇con.blp" {
		t.Errorf("Lower = %q", got)
	}
	if got := Lower("Textures\\Élan.BLP"); got != "textures\\élan.blp" {
		t.Errorf("Lower = %q", got)
	}
}

func TestLossyReplacesEachMaximalInvalidSequence(t *testing.T) {
	cases := []struct {
		in   []byte
		want string
	}{
		{[]byte("plain é 🌙"), "plain é 🌙"},
		{[]byte{0xFF, 0xFF}, "��"},
		{[]byte{0xE2, 0x82, 'A'}, "�A"},   // a cut-off three-byte sequence is one replacement
		{[]byte{0xE2, 0x82}, "�"},         // also at the end
		{[]byte{0xC0, 0x80}, "��"},        // an overlong form: each byte on its own
		{[]byte{0xED, 0xA0, 0x80}, "���"}, // a surrogate
		{[]byte{0xF0, 0x9F, 0x8C, 'x'}, "�x"},
		{[]byte{0xEF, 0xBB, 0xBF, 'a'}, "\xEF\xBB\xBFa"}, // the byte order mark stays
	}
	for _, c := range cases {
		if got := Lossy(c.in); got != c.want {
			t.Errorf("Lossy(% X) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDecodeAndStrictDropALeadingByteOrderMark(t *testing.T) {
	if got := Decode([]byte{0xEF, 0xBB, 0xBF, 'a', 0xFF}); got != "a�" {
		t.Errorf("Decode = %q", got)
	}
	if got, ok := Strict([]byte{0xEF, 0xBB, 0xBF, 'a'}); !ok || got != "a" {
		t.Errorf("Strict = %q, %v", got, ok)
	}
	if _, ok := Strict([]byte{'a', 0xFF}); ok {
		t.Error("Strict accepted invalid bytes")
	}
}
