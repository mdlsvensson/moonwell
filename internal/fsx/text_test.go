package fsx

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestWithoutMarkDropsOneMarkAtTheVeryStartAndNoOther(t *testing.T) {
	for _, c := range []struct{ name, text, want string }{
		{"no mark", "a", "a"},
		{"a mark at the start", bom + "a", "a"},
		{"a mark only", bom, ""},
		{"two marks: the second is content", bom + bom + "a", bom + "a"},
		{"a mark further in", "a" + bom, "a" + bom},
		{"a mark after a space", " " + bom + "a", " " + bom + "a"},
		{"a mark cut short", "\xEF\xBB", "\xEF\xBB"},
		{"nothing", "", ""},
	} {
		if got := TrimBOM(c.text); got != c.want {
			t.Errorf("%s: WithoutMark(%q) = %q, want %q", c.name, c.text, got, c.want)
		}
		if got := TrimBOM([]byte(c.text)); string(got) != c.want {
			t.Errorf("%s, as bytes: WithoutMark(%q) = %q, want %q", c.name, c.text, got, c.want)
		}
	}
}

func TestTrimASCIISpaceTakesTheSixCharactersOfASCIIAndNoOther(t *testing.T) {
	for text, want := range map[string]string{
		" \t\n\v\f\ra b \r\f\v\n\t ": "a b",
		" \t\n\v\f\r":                "",
		"":                           "",
		"\xC2\xA0a\xC2\x85":          "\xC2\xA0a\xC2\x85",
		"\xE2\x80\xA8a ":             "\xE2\x80\xA8a",
		" \xE3\x80\x80":              "\xE3\x80\x80",
		bom + " a":                   bom + " a",
	} {
		if got := TrimASCIISpace(text); got != want {
			t.Errorf("TrimASCIISpace(%q) = %q, want %q", text, got, want)
		}
	}
}

func TestDecodeText(t *testing.T) {
	for _, c := range []struct{ name, bytes, want string }{
		{"valid text is unchanged", "héro 1 \u2603\n", "héro 1 \u2603\n"},
		{"a leading byte order mark is dropped", bom + "a", "a"},
		{"a byte order mark further in stays", "a" + bom, "a" + bom},
		{"an invalid byte becomes U+FFFD", "a\xFFb", "a\uFFFDb"},
		{"a run of invalid bytes becomes one U+FFFD", "a\xFF\xFE\xC0b", "a\uFFFDb"},
		{"no bytes are no text", "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := DecodeText([]byte(c.bytes)); got != c.want {
				t.Errorf("DecodeText(%q) = %q, want %q", c.bytes, got, c.want)
			}
		})
	}
}

func TestTextWithMarkKeepsTheMarkAsideAndRefusesInvalidBytes(t *testing.T) {
	for _, c := range []struct {
		name, bytes, mark, text string
		ok                      bool
	}{
		{"no mark", "Count = 0\n", "", "Count = 0\n", true},
		{"a mark", bom + "Count = 0\n", bom, "Count = 0\n", true},
		{"a mark only", bom, bom, "", true},
		{"an empty file", "", "", "", true},
		{"only the first mark is kept aside", bom + bom + "a", bom, bom + "a", true},
		{"a mark further in is text", "a" + bom, "", "a" + bom, true},
		{"text outside ASCII", bom + "h\xC3\xA9ro\n", bom, "h\xC3\xA9ro\n", true},
		{"an invalid byte", "a\xFFb", "", "", false},
		{"an invalid byte after a mark", bom + "a\xFFb", "", "", false},
		{"a mark cut short", "\xEF\xBB", "", "", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			mark, text, ok := SplitBOM([]byte(c.bytes))
			if mark != c.mark || text != c.text || ok != c.ok {
				t.Errorf("TextWithMark(%q) = %q, %q, %v, want %q, %q, %v", c.bytes, mark, text, ok, c.mark, c.text, c.ok)
			}
			if ok && mark+text != c.bytes {
				t.Errorf("the mark and the text make %q, want the bytes given", mark+text)
			}
		})
	}
}

func TestQuotedEscapesTheQuoteTheBackslashAndTheControlCharacters(t *testing.T) {
	const long = `\` + "u00"
	for _, c := range []struct{ name, text, want string }{
		{"no text", "", `""`},
		{"plain text", "Models/unit.mdx", `"Models/unit.mdx"`},
		{"the quote and the backslash", `a"b\c`, `"a\"b\\c"`},
		{"each short escape", "\b\f\n\r\t", `"\b\f\n\r\t"`},
		{"the first and the last control character", "\x00\x1F", `"` + long + "00" + long + `1f"`},
		{"hexadecimal digits in lower case", "\x0B\x1A\x1E", `"` + long + "0b" + long + "1a" + long + `1e"`},
		{"markup and the slash", "<a href='x/y'>&</a>", `"<a href='x/y'>&</a>"`},
		{"DEL", "a\x7Fb", "\"a\x7Fb\""},
		{"a character outside ASCII", "h\xC3\xA9ro", "\"h\xC3\xA9ro\""},
		{"the line and the paragraph separator", "\xE2\x80\xA8\xE2\x80\xA9", "\"\xE2\x80\xA8\xE2\x80\xA9\""},
		{"a character beyond the basic plane", "\xF0\x9F\x8C\x99", "\"\xF0\x9F\x8C\x99\""},
		{"a byte that is not UTF-8", "a\xFFb", "\"a\xFFb\""},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := QuoteJSON(c.text); got != c.want {
				t.Errorf("Quoted(%q) = %s, want %s", c.text, got, c.want)
			}
		})
	}
}

func TestQuotedEscapesExactlyTheBytesItMustAndWritesJSON(t *testing.T) {
	for b := range 256 {
		text := string([]byte{byte(b)})
		got := QuoteJSON(text)
		escaped := b < 0x20 || b == '"' || b == '\\'
		if kept := got == `"`+text+`"`; kept == escaped {
			t.Errorf("Quoted of the byte %#02x = %s, want it escaped: %v", b, got, escaped)
		}
		if escaped && b < 0x20 && shortEscapes[byte(b)] == "" && got != fmt.Sprintf(`"\u00%02x"`, b) {
			t.Errorf("Quoted of the byte %#02x = %s, want the escape with four hexadecimal digits in lower case", b, got)
		}
		var back string
		if err := json.Unmarshal([]byte(got), &back); b < 0x80 && (err != nil || back != text) {
			t.Errorf("Quoted of the byte %#02x = %s, which reads back as %q, %v", b, got, back, err)
		}
	}
}
