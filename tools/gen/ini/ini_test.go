package ini_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/tools/gen/ini"
)

func section(t *testing.T, file *ini.File, name string) map[string]string {
	t.Helper()
	found, ok := file.Get(name)
	if !ok {
		t.Fatalf("no section %s", name)
	}
	values := map[string]string{}
	for key, value := range found.All() {
		values[key] = value
	}
	return values
}

func TestParseReadsSectionsUnquotesValuesAndKeepsTheLastDuplicateKey(t *testing.T) {
	text := strings.Join([]string{
		"\xEF\xBB\xBF// a comment",
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
	file := ini.Parse(text, nil)
	if !reflect.DeepEqual(file.Keys(), []string{"First", "Second"}) {
		t.Errorf("sections: %q", file.Keys())
	}
	want := map[string]string{"Name": "Captain", "Tip": "Quoted; with = signs", "List": `"one","two"`, "Hotkey": "F"}
	if got := section(t, file, "First"); !reflect.DeepEqual(got, want) {
		t.Errorf("First: %v", got)
	}
	if got := section(t, file, "Second"); !reflect.DeepEqual(got, map[string]string{"Empty": ""}) {
		t.Errorf("Second: %v", got)
	}
}

func TestParseMergesIntoAnExistingResultLaterFilesWinning(t *testing.T) {
	file := ini.Parse("[a]\nName=One\nTip=T\n", nil)
	ini.Parse("[a]\nName=Two\n[b]\nName=B\n", file)
	if got := section(t, file, "a"); !reflect.DeepEqual(got, map[string]string{"Name": "Two", "Tip": "T"}) {
		t.Errorf("a: %v", got)
	}
	if got := section(t, file, "b"); !reflect.DeepEqual(got, map[string]string{"Name": "B"}) {
		t.Errorf("b: %v", got)
	}
}
