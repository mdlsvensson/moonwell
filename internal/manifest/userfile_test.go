package manifest

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func newUserEnv(t *testing.T, settings string) *env.Env {
	t.Helper()
	e, _ := testkit.Env(t, t.TempDir())
	testkit.WriteFile(t, e.ConfigDir, UserFile, []byte(settings))
	return e
}

func absolutePath(name string) string {
	if runtime.GOOS == "windows" {
		return `C:\work\` + name
	}
	return "/work/" + name
}

func TestReadUserWithoutAFileGivesTheDefaultsAndNoError(t *testing.T) {
	e, _ := testkit.Env(t, t.TempDir())
	user, err := ReadUser(e)
	if err != nil {
		t.Fatal(err)
	}
	if user.File != filepath.Join(e.ConfigDir, "config.toml") {
		t.Errorf("File = %q", user.File)
	}
	if user.Launch.GameExecutable != nil || !reflect.DeepEqual(user.Launch.Args, []string{"-launch", "-windowmode", "windowed"}) {
		t.Errorf("Launch = %+v", user.Launch)
	}
	if user.YuePath != nil || len(user.Libraries) != 0 {
		t.Errorf("YuePath = %v, Libraries = %+v", user.YuePath, user.Libraries)
	}
}

func TestReadUserReadsEverySettingOfAFullFile(t *testing.T) {
	wrappers, systems := absolutePath("moonwell-wrappers"), absolutePath("moonwell-systems")
	user, err := ReadUser(newUserEnv(t, `
[launch]
gameExecutable = 'D:\Games\Warcraft III.exe'
args = ["-launch", "-windowmode", "fullscreen"]

[yue]
path = 'C:\tools\yue.exe'

[[libraries]]
github = "mdlsvensson/moonwell-wrappers"
path = '`+wrappers+`'

[[libraries]]
github = "mdlsvensson/moonwell-systems"
path = '`+systems+`'
`))
	if err != nil {
		t.Fatal(err)
	}
	if derefOrNil(user.Launch.GameExecutable) != `D:\Games\Warcraft III.exe` ||
		!reflect.DeepEqual(user.Launch.Args, []string{"-launch", "-windowmode", "fullscreen"}) {
		t.Errorf("Launch = %+v", user.Launch)
	}
	if derefOrNil(user.YuePath) != `C:\tools\yue.exe` {
		t.Errorf("YuePath = %v", user.YuePath)
	}
	want := []LocalLibrary{
		{GitHub: "mdlsvensson/moonwell-wrappers", Path: wrappers}, {GitHub: "mdlsvensson/moonwell-systems", Path: systems},
	}
	if !reflect.DeepEqual(user.Libraries, want) {
		t.Errorf("Libraries = %+v, want %+v", user.Libraries, want)
	}
}

func TestReadUserTakesAnEmptyListOfArgumentsAsNoArguments(t *testing.T) {
	user, err := ReadUser(newUserEnv(t, "[launch]\nargs = []\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(user.Launch.Args) != 0 {
		t.Errorf("Args = %q, want none: a list in the file replaces the default one", user.Launch.Args)
	}
}

func TestReadUserRefusesAValueOutsideItsRuleNamingTheSettingAndTheFile(t *testing.T) {
	library := "[[libraries]]\ngithub = \"a/b\"\npath = '" + absolutePath("b") + "'\n"
	for _, tt := range []struct{ name, settings, setting string }{
		{"an empty game path", "[launch]\ngameExecutable = \"\"\n", "launch.gameExecutable"},
		{"an empty compiler path", "[yue]\npath = \"\"\n", "yue.path"},
		{"a library that names no repository", "[[libraries]]\npath = '" + absolutePath("b") + "'\n", "libraries[0].github"},
		{"a library by its address", "[[libraries]]\ngithub = \"github.com/a/b\"\npath = '" + absolutePath("b") + "'\n", "libraries[0].github"},
		{"a repository twice, in another case", library + strings.Replace(library, "a/b", "A/B", 1), "libraries[1].github"},
		{"a relative folder", "[[libraries]]\ngithub = \"a/b\"\npath = \"../b\"\n", "libraries[0].path"},
		{"no folder", "[[libraries]]\ngithub = \"a/b\"\n", "libraries[0].path"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := newUserEnv(t, tt.settings)
			_, err := ReadUser(e)
			if err == nil {
				t.Fatal("ReadUser took it")
			}
			diagErr := asDiagError(t, err, "the refusal")
			if diagErr.File != filepath.Join(e.ConfigDir, "config.toml") || !strings.HasPrefix(diagErr.Msg, tt.setting) || diagErr.Hint == "" {
				t.Errorf("got %q in %q, want an error about %s in the user's file, with a hint", diagErr.Msg, diagErr.File, tt.setting)
			}
		})
	}
}

func TestReadUserRefusesASettingOfAProjectAndAFileThatIsNoTOML(t *testing.T) {
	for _, tt := range []struct{ name, settings, named string }{
		{"a project's table", "[map]\nfolder = \"map.w3x\"\n", "map"},
		{"a compiler version", "[yue]\nversion = \"0.34.3\"\n", "'yue' has invalid keys: version"},
		{"a tag for a library", "[[libraries]]\ngithub = \"a/b\"\npath = '" + absolutePath("b") + "'\ntag = \"v1\"\n", "'libraries[0]' has invalid keys: tag"},
		{"a number for the game", "[launch]\ngameExecutable = 5\n", "'launch.gameExecutable'"},
		{"no TOML", "[launch\n", "not valid TOML"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := newUserEnv(t, tt.settings)
			_, err := ReadUser(e)
			if err == nil {
				t.Fatal("ReadUser took it")
			}
			diagErr := asDiagError(t, err, "the refusal")
			if diagErr.File != filepath.Join(e.ConfigDir, "config.toml") || !strings.Contains(diagErr.Msg, tt.named) {
				t.Errorf("got %q in %q, want an error that holds %q in the user's file", diagErr.Msg, diagErr.File, tt.named)
			}
		})
	}
}

func TestEnsureUserFileCreatesTheFileWithTheDefaultGameAndItsFolderWhenMissing(t *testing.T) {
	e, _ := testkit.Env(t, t.TempDir())
	e.ConfigDir = filepath.Join(e.ConfigDir, "not", "there", "yet")
	created, err := EnsureUserFile(e)
	if err != nil || !created {
		t.Fatalf("EnsureUserFile = %v, %v, want it created", created, err)
	}
	user, err := ReadUser(e)
	if err != nil {
		t.Fatalf("the file EnsureUserFile wrote is not read: %v", err)
	}
	if derefOrNil(user.Launch.GameExecutable) != DefaultGameExecutable || user.YuePath != nil || len(user.Libraries) != 0 {
		t.Errorf("the new file gives %+v, want the default game and nothing else", user)
	}
}

func TestEnsureUserFileLeavesAFileThatIsThereAsItIs(t *testing.T) {
	const own = "[launch]\ngameExecutable = 'D:\\mine.exe'\n"
	e := newUserEnv(t, own)
	created, err := EnsureUserFile(e)
	if err != nil || created {
		t.Fatalf("EnsureUserFile = %v, %v, want nothing created", created, err)
	}
	data, err := os.ReadFile(UserFilePath(e))
	if err != nil || string(data) != own {
		t.Errorf("the file reads %q, %v, want it untouched", data, err)
	}
}

func TestEnsureUserFileReportsAFolderThatCannotBeMadeNamingTheFile(t *testing.T) {
	e, _ := testkit.Env(t, t.TempDir())
	blocker := testkit.WriteFile(t, e.ConfigDir, "a-file", []byte("x"))
	e.ConfigDir = filepath.Join(blocker, "below")
	_, err := EnsureUserFile(e)
	if err == nil {
		t.Fatal("EnsureUserFile made a folder below a file")
	}
	diagErr := asDiagError(t, err, "the refusal")
	if diagErr.File != UserFilePath(e) || !strings.Contains(diagErr.Hint, "MOONWELL_HOME") {
		t.Errorf("got %q in %q with the hint %q", diagErr.Msg, diagErr.File, diagErr.Hint)
	}
}
