package ini_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/tools/gen/ini"
)

var sections = strings.Join([]string{
	"// a comment",
	"ignored=before any section",
	"[First]\t",
	"Name=Footman",
	`Tip="Quoted; with = signs"`,
	`List="one","two"`,
	"  // indented comment",
	"Name=Captain",
	"",
	"[Second]",
	"Empty=",
	"no equals sign",
	"[First]",
	"Hotkey=F",
}, "\r\n")

const (
	merged      = "[a]\nName=One\nTip=T\n"
	mergedLater = "[a]\nName=Two\n[b]\nName=B\n"
)

func TestParseReadsSectionsUnquotesValuesAndKeepsTheLastDuplicateKey(t *testing.T) {
	want := ini.File{
		"First":  {"Name": "Captain", "Tip": "Quoted; with = signs", "List": `"one","two"`, "Hotkey": "F"},
		"Second": {"Empty": ""},
	}
	if got := ini.Parse(sections); !reflect.DeepEqual(got, want) {
		t.Errorf("sections: %q, want %q", got, want)
	}
}

func TestAddMergesIntoAnExistingResultLaterTextsWinning(t *testing.T) {
	file := ini.Parse(merged)
	file.Add(mergedLater)
	want := ini.File{"a": {"Name": "Two", "Tip": "T"}, "b": {"Name": "B"}}
	if !reflect.DeepEqual(file, want) {
		t.Errorf("sections: %q, want %q", file, want)
	}
	file.Add("Name=Three\nTip=U\n")
	if !reflect.DeepEqual(file, want) {
		t.Errorf("after a text of keys without a section: %q, want %q", file, want)
	}
	added := ini.File{}
	added.Add(sections)
	if !reflect.DeepEqual(added, ini.Parse(sections)) {
		t.Errorf("added to a file without sections: %q, want %q", added, ini.Parse(sections))
	}
}

var lines = []struct {
	name, text string
	want       ini.File
}{
	{"no text", "", ini.File{}},
	{"keys before any section", "Name=One\nTip=T\n", ini.File{}},
	{"a section without keys", "[a]\n", ini.File{"a": {}}},
	{"a section without a name", "[]\nName=One\n", ini.File{"": {"Name": "One"}}},
	{"ASCII white space of every kind around a header, a name, a key and a value",
		"\v[\f a \t]\r\n \tName\v=\f One \r\n\tTip\t=\t\"T\"\t\n", ini.File{"a": {"Name": "One", "Tip": "T"}}},
	{"a value keeps what is after its first equals sign", "[a]\nName=a=b\n=c\n",
		ini.File{"a": {"Name": "a=b", "": "c"}}},
	{"a value of one quoted string loses its quotes, and no other value does",
		"[a]\nA=\"\"\nB=\"\nC=\"x\" y\nD= \" x \" \nE=\"a\"\"b\"\nF='x'\n",
		ini.File{"a": {"A": "", "B": `"`, "C": `"x" y`, "D": " x ", "E": `"a""b"`, "F": "'x'"}}},
	{"a line that starts or ends like a header and is none", "[a]\n[b\nc]\n[d]x\n[e]=1\n",
		ini.File{"a": {"[e]": "1"}}},
	{"a comment after a key is part of the value, and a comment line is none",
		"[a]\nName=One // note\n//Tip=T\n", ini.File{"a": {"Name": "One // note"}}},
	{"a carriage return alone ends no line", "[a]\nName=One\rTip=T\n", ini.File{"a": {"Name": "One\rTip=T"}}},
}

func TestParseReadsLines(t *testing.T) {
	for _, c := range lines {
		if got := ini.Parse(c.text); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

func TestAValueWithAQuoteAtOneEndOnlyIsKeptAsItIsWritten(t *testing.T) {
	got := ini.Parse("[a]\nA=\"open\nB=shut\"\nC=\"x\nD=x\"\nE=\"x\"\n")
	want := ini.File{"a": {"A": `"open`, "B": `shut"`, "C": `"x`, "D": `x"`, "E": "x"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%q, want %q", got, want)
	}
}

var widerSpace = []struct {
	name, text string
	want       ini.File
}{
	{"a no-break space around a key and a value",
		"[a]\nName\xC2\xA0=\xC2\xA0One\xC2\xA0\n\xE3\x80\x80Tip=T\n",
		ini.File{"a": {"Name\xC2\xA0": "\xC2\xA0One\xC2\xA0", "\xE3\x80\x80Tip": "T"}}},
	{"a byte order mark before a header that is not the first line, and a line separator before a comment",
		"[a]\nName=One\n\xEF\xBB\xBF[b]\nName=Two\n\xE2\x80\xA8// Tip=T\n",
		ini.File{"a": {"Name": "Two", "\xE2\x80\xA8// Tip": "T"}}},
	{"a no-break space inside a value and inside the name of a section",
		"[a\xC2\xA0b]\nName=One\xC2\xA0Two\n",
		ini.File{"a\xC2\xA0b": {"Name": "One\xC2\xA0Two"}}},
}

func TestWhiteSpaceOutsideASCIIIsText(t *testing.T) {
	for _, c := range widerSpace {
		if got := ini.Parse(c.text); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
	if got := ini.Parse("\xEF\xBB\xBF[a]\nName=One\n"); len(got) != 0 {
		t.Errorf("a byte order mark before the first header: %q, want no section", got)
	}
}
