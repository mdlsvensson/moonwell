package manifest

import (
	"path/filepath"
	"reflect"
	"testing"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func TestLoadTakesTheProjectFromItsFileAndTheMachineFromTheUsers(t *testing.T) {
	e := newProjectEnv(t, "[build]\nminify = true\n\n[yue]\nversion = \"0.34.2\"\n")
	testkit.WriteFile(t, e.ConfigDir, UserFile, []byte(
		"[launch]\ngameExecutable = 'D:\\Games\\Warcraft III.exe'\nargs = [\"-launch\"]\n\n[yue]\npath = 'C:\\tools\\yue.exe'\n"))
	project, err := Load(e)
	if err != nil {
		t.Fatal(err)
	}
	if !project.Build.Minify || project.ManifestName != "moonwell.toml" || project.Root != e.Root {
		t.Errorf("project = %+v", project)
	}
	if project.UserFile != filepath.Join(e.ConfigDir, "config.toml") {
		t.Errorf("UserFile = %q", project.UserFile)
	}
	if derefOrNil(project.Launch.GameExecutable) != `D:\Games\Warcraft III.exe` || !reflect.DeepEqual(project.Launch.Args, []string{"-launch"}) {
		t.Errorf("Launch = %+v", project.Launch)
	}
	if project.Yue.Version != "0.34.2" || derefOrNil(project.Yue.Path) != `C:\tools\yue.exe` {
		t.Errorf("Yue = %+v: the version is the project's and the path the machine's", project.Yue)
	}
}

func TestLoadWithoutAUsersFileStartsNoGameAndUsesTheDefaultArguments(t *testing.T) {
	project, err := Load(newProjectEnv(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	if project.Launch.GameExecutable != nil || len(project.Launch.Args) != 3 || project.Yue.Path != nil {
		t.Errorf("Launch = %+v, Yue = %+v", project.Launch, project.Yue)
	}
}

func TestLoadRefusesAProjectFirstAndThenTheUsersFileEachByItsOwnName(t *testing.T) {
	e := newProjectEnv(t, "[build]\nfolder = \"\"\n")
	testkit.WriteFile(t, e.ConfigDir, UserFile, []byte("[yue]\npath = \"\"\n"))
	_, err := Load(e)
	if diagErr := asDiagError(t, err, "a broken project file"); diagErr.File != "moonwell.toml" {
		t.Errorf("got %q in %q, want the project's file first", diagErr.Msg, diagErr.File)
	}
	testkit.WriteFile(t, e.Root, ProjectFile, nil)
	_, err = Load(e)
	if diagErr := asDiagError(t, err, "a broken user's file"); diagErr.File != filepath.Join(e.ConfigDir, "config.toml") {
		t.Errorf("got %q in %q, want the user's file", diagErr.Msg, diagErr.File)
	}
}

func TestIsProjectAsksForMoonwellTomlAlone(t *testing.T) {
	if IsProject(newEnv(t, map[string]string{"moonwell.pkl": "", "PklProject": ""}).Root) {
		t.Error("a folder with a Pkl manifest of 0.11 counts as a project")
	}
	if !IsProject(newEnv(t, map[string]string{"moonwell.toml": ""}).Root) {
		t.Error("a folder with moonwell.toml does not count as a project")
	}
}

func TestTheTemplatesSettingsFileWritesTheDefaultsAndNothingElse(t *testing.T) {
	template, err := moonwell.TemplateFiles()
	if err != nil {
		t.Fatal(err)
	}
	var written *Project
	for _, file := range template {
		if file.Path == ProjectFile {
			written = mustReadProject(t, string(file.Data))
		}
	}
	if written == nil {
		t.Fatal("the template has no moonwell.toml")
	}
	bare := mustReadProject(t, "")
	written.Root, bare.Root = "", ""
	if !reflect.DeepEqual(settingsOf(written), settingsOf(bare)) {
		t.Errorf("the template's file gives %+v, an empty file %+v: a test that writes its own file would not start from the template's settings",
			settingsOf(written), settingsOf(bare))
	}
}

func settingsOf(project *Project) []any {
	return []any{
		project.Map, project.Build, project.Yue, project.Lint.UnknownGlobals, len(project.Lint.Globals),
		project.Assets.Paths.Len(), len(project.Assets.Exclude), len(project.Libraries), project.Settings,
	}
}

func TestLoadTakesALibraryFromTheFolderTheUsersFileNamesForItsRepository(t *testing.T) {
	wrappers := absolutePath("moonwell-wrappers")
	e := newProjectEnv(t, `
[[libraries]]
name = "wrappers"
github = "mdlsvensson/moonwell-wrappers"
tag = "v0.10.0"
dir = "src"

[[libraries]]
name = "systems"
github = "mdlsvensson/moonwell-systems"
tag = "v0.5.0"

[[libraries]]
name = "beside"
path = "../beside"
`)
	testkit.WriteFile(t, e.ConfigDir, UserFile, []byte(
		"[[libraries]]\ngithub = \"MDLSvensson/Moonwell-Wrappers\"\npath = '"+wrappers+"'\n\n"+
			"[[libraries]]\ngithub = \"someone/unused\"\npath = '"+absolutePath("unused")+"'\n"))
	project, err := Load(e)
	if err != nil {
		t.Fatal(err)
	}
	overridden := project.Libraries["wrappers"]
	if derefOrNil(overridden.Path) != wrappers || overridden.OverriddenIn != filepath.Join(e.ConfigDir, "config.toml") {
		t.Errorf("wrappers = %+v, want the folder of the user's file, whatever the letter case of the repository", overridden)
	}
	if derefOrNil(overridden.Tag) != "v0.10.0" || derefOrNil(overridden.GitHub) != "mdlsvensson/moonwell-wrappers" || overridden.Dir != "src" {
		t.Errorf("wrappers = %+v, want its repository, tag and dir as the project wrote them", overridden)
	}
	if systems := project.Libraries["systems"]; systems.Path != nil || systems.OverriddenIn != "" {
		t.Errorf("systems = %+v, want it untouched: the user's file does not name its repository", systems)
	}
	if beside := project.Libraries["beside"]; derefOrNil(beside.Path) != "../beside" || beside.OverriddenIn != "" {
		t.Errorf("beside = %+v, want the project's own path", beside)
	}
	if len(project.Libraries) != 3 {
		t.Errorf("the project has %d libraries, want 3: the user's file adds none", len(project.Libraries))
	}
}
