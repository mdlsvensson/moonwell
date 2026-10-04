package assets

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/fsx"
)

// Two hashes a state file can hold.
var (
	zeros  = strings.Repeat("0", 64)
	sevens = strings.Repeat("7", 64)
)

// stateOf is the path of a state file that holds content, in a project of its own.
func stateOf(t *testing.T, content string) string {
	t.Helper()
	return put(t, t.TempDir(), ".asset-state/map.w3x.json", content)
}

func TestStateFileIsUnderAssetStateByTheMapFoldersName(t *testing.T) {
	root := t.TempDir()
	file, err := StateFile(root, "map.w3x")
	if err != nil || file != filepath.Join(root, ".asset-state", "map.w3x.json") {
		t.Errorf("StateFile = %q, %v", file, err)
	}
	file, err = StateFile(root, "../map.w3x")
	if e := asError(t, err, "a map folder outside maps/"); file != "" || !strings.Contains(e.Msg, "../map.w3x") || e.Hint == "" {
		t.Errorf("StateFile = %q, %+v", file, e)
	}
}

func TestReadStateOfAProjectWithoutAStateFileIsEmpty(t *testing.T) {
	file, err := StateFile(t.TempDir(), "map.w3x")
	if err != nil {
		t.Fatal(err)
	}
	if state, err := ReadState(file); err != nil || len(state.Files) != 0 {
		t.Errorf("ReadState = %+v, %v", state, err)
	}
}

func TestReadStateKeepsThePathsAsWrittenInTheOrderWritten(t *testing.T) {
	tests := []struct {
		name, content string
		want          []Owned
	}{
		{"the files in their order, a path that looks like a number where it stands",
			`{"version":1,"files":{"b.blp":"` + zeros + `","7":"` + sevens + `","A.blp":"` + zeros + `"}}`,
			[]Owned{{"b.blp", zeros}, {"7", sevens}, {"A.blp", zeros}}},
		{"a path with a backslash stays as it is written",
			`{"version":1,"files":{"Textures\\a.blp":"` + sevens + `"}}`, []Owned{{`Textures\a.blp`, sevens}}},
		{"as State.Bytes writes it",
			"{\n  \"version\": 1,\n  \"files\": {\n    \"a.blp\": \"" + zeros + "\"\n  }\n}\n", []Owned{{"a.blp", zeros}}},
		{"no files", `{"version":1,"files":{}}`, nil},
		{"the version written as a fraction, and members a state does not have",
			`{"files":{"a.blp":"` + zeros + `"},"note":[1,{"a":null}],"version":1.0}`, []Owned{{"a.blp", zeros}}},
		{"a path written twice has its last hash",
			`{"version":1,"files":{"a.blp":"bad","b.blp":"` + zeros + `","a.blp":"` + sevens + `"}}`,
			[]Owned{{"a.blp", sevens}, {"b.blp", zeros}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, err := ReadState(stateOf(t, tt.content))
			if err != nil || !slices.Equal(state.Files, tt.want) {
				t.Errorf("ReadState = %+v, %v, want %+v", state.Files, err, tt.want)
			}
		})
	}
}

func TestReadStateRefusesAFileThatIsNotAStateAndNamesIt(t *testing.T) {
	entry := func(path, hash string) string { return `{"version":1,"files":{` + path + `:` + hash + `}}` }
	valid := `"` + zeros + `"`
	tests := []struct{ name, content, problem string }{
		{"text", `not json`, "it is not JSON."},
		{"an empty file", ``, "it is not JSON."},
		{"a state with more after it", `{"version":1,"files":{}} {}`, "it is not JSON."},
		{"a byte order mark", "\xEF\xBB\xBF" + `{"version":1,"files":{}}`, "it is not JSON."},
		{"another version", `{"version":2,"files":{}}`, "version must be 1."},
		{"the version as text", `{"version":"1","files":{}}`, "version must be 1."},
		{"no version", `{"files":{}}`, "version must be 1."},
		{"the version under another spelling", `{"Version":1,"files":{}}`, "version must be 1."},
		{"null", `null`, "version must be 1."},
		{"a list", `[]`, "version must be 1."},
		{"no files", `{"version":1}`, "files must be an object."},
		{"the files as a list", `{"version":1,"files":[]}`, "files must be an object."},
		{"the files as null", `{"version":1,"files":null}`, "files must be an object."},
		{"a short hash", entry(`"a.blp"`, `"abc"`), "a.blp has no valid hash."},
		{"a hash in capitals", entry(`"a.blp"`, `"`+strings.Repeat("A", 64)+`"`), "a.blp has no valid hash."},
		{"a hash with a line break after it", entry(`"a.blp"`, `"`+zeros+`\n"`), "a.blp has no valid hash."},
		{"a number for a hash", entry(`"a.blp"`, `7`), "a.blp has no valid hash."},
		{"null for a hash", entry(`"a.blp"`, `null`), "a.blp has no valid hash."},
		{"a path listed twice by letter case", `{"version":1,"files":{"a.blp":` + valid + `,"A.BLP":` + valid + `}}`,
			"A.BLP is listed twice."},
		{"a path listed twice by its separator", `{"version":1,"files":{"t/a.blp":` + valid + `,"t\\a.blp":` + valid + `}}`,
			`t\a.blp is listed twice.`},
		// A forged state cannot make assets:sync remove one of the map's own files.
		{"one of the map's own files", entry(`"war3map.lua"`, valid), "Reserved map path: war3map.lua"},
		{"a path that leaves the map", entry(`"../a.blp"`, valid), "Invalid asset path: ../a.blp"},
		{"a bad path is found before its bad hash", entry(`"war3map.lua"`, `"abc"`), "Reserved map path: war3map.lua"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := stateOf(t, tt.content)
			state, err := ReadState(file)
			e := asError(t, err, tt.content)
			if e.Msg != "The asset ownership state is invalid: "+tt.problem || e.File != file ||
				!strings.HasPrefix(e.Hint, "Restore it from version control.") || len(state.Files) != 0 {
				t.Errorf("ReadState = %+v, %+v, want the problem %q at %s", state, e, tt.problem, file)
			}
		})
	}
}

func TestTheFirstBadEntryOfAStateIsTheFirstWrittenAlsoWhereAPathLooksLikeANumber(t *testing.T) {
	_, err := ReadState(stateOf(t, `{"version":1,"files":{"b.blp":"bad","7":"worse"}}`))
	if e := asError(t, err, "two bad hashes"); !strings.Contains(e.Msg, "b.blp has no valid hash") {
		t.Errorf("error = %+v, want it about b.blp, which is written before 7", e)
	}
}

func TestReadStateNamesAStateFileItCannotRead(t *testing.T) {
	// A folder where the file should be: reading it fails on every system.
	file := filepath.Join(t.TempDir(), ".asset-state", "map.w3x.json")
	if err := os.MkdirAll(file, 0o777); err != nil {
		t.Fatal(err)
	}
	_, err := ReadState(file)
	e := asError(t, err, "a folder in place of the state file")
	if !strings.Contains(e.Msg, "Reading the asset ownership state failed") || e.File != file || e.Hint == "" || e.Cause == nil {
		t.Errorf("error = %+v", e)
	}
}

func TestStateBytesAreTheFilesInTheOrderGivenWithTwoSpacesAndAFinalLineBreak(t *testing.T) {
	const icon = "ReplaceableTextures/CommandButtons/BTNSword.blp"
	tests := []struct {
		name  string
		state State
		want  string
	}{
		{"two files", State{Files: []Owned{{icon, zeros}, {"Textures/a.blp", sevens}}},
			"{\n  \"version\": 1,\n  \"files\": {\n    \"" + icon + "\": \"" + zeros + "\",\n    \"Textures/a.blp\": \"" + sevens + "\"\n  }\n}\n"},
		{"no files", State{}, "{\n  \"version\": 1,\n  \"files\": {}\n}\n"},
		{"a path that looks like a number stays where it is given",
			State{Files: []Owned{{"b.blp", zeros}, {"7", sevens}}},
			"{\n  \"version\": 1,\n  \"files\": {\n    \"b.blp\": \"" + zeros + "\",\n    \"7\": \"" + sevens + "\"\n  }\n}\n"},
		// Only the quote, the backslash and the control characters are escaped: markup, a line separator (U+2028)
		// and letters outside ASCII are written as they are.
		{"what JSON must escape and nothing more",
			State{Files: []Owned{{"a\"b\\c\n\x01<&>\xe2\x80\xa8\xc3\xa9.blp", zeros}}},
			"{\n  \"version\": 1,\n  \"files\": {\n    \"a\\\"b\\\\c\\n\\u0001<&>\xe2\x80\xa8\xc3\xa9.blp\": \"" + zeros + "\"\n  }\n}\n"},
	}
	for _, tt := range tests {
		if got := string(tt.state.Bytes()); got != tt.want {
			t.Errorf("%s: the state file is\n%s\nwant\n%s", tt.name, got, tt.want)
		}
	}
}

func TestAStateIsReadBackAsItWasWritten(t *testing.T) {
	state := State{Files: []Owned{
		{"war3mapImported/ui/frames.toc", fsx.SHA256Hex([]byte("toc"))},
		{"Models/H\xc3\xa9ro.mdx", fsx.SHA256Hex([]byte("model"))},
		{"12", zeros},
	}}
	read, err := ReadState(stateOf(t, string(state.Bytes())))
	if err != nil || !slices.Equal(read.Files, state.Files) {
		t.Errorf("ReadState = %+v, %v, want %+v", read.Files, err, state.Files)
	}
}
