package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

const captainFile = `amends "@moonwell/ObjectFile.pkl"

units {
  ["captain"] {
    id = "h000"
    base = "hfoo"
    name = "Captain"
  }
}
`

const paladinFile = `amends "@moonwell/ObjectFile.pkl"

heroes {
  ["paladin"] {
    id = "H000"
    base = "Hpal"
  }
}
`

func TestHasObjectFilesFindsAPklFileAtAnyDepthBelowObjectsAndNothingElse(t *testing.T) {
	for _, tt := range []struct {
		name  string
		files map[string]string
		want  bool
	}{
		{"no folder", map[string]string{"src/main.yue": ""}, false},
		{"an empty folder", map[string]string{"objects/.gitkeep": ""}, false},
		{"other files only", map[string]string{"objects/notes.txt": "", "objects/units.pkl.bak": ""}, false},
		{"a file in the folder", map[string]string{"objects/units.pkl": ""}, true},
		{"a file two folders down", map[string]string{"objects/heroes/human/paladin.pkl": ""}, true},
		{"a pkl file beside the folder", map[string]string{"units.pkl": "", "lua/objects/units.pkl": ""}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := HasObjectFiles(newEnv(t, tt.files).Root)
			if err != nil || got != tt.want {
				t.Errorf("HasObjectFiles = %v, %v, want %v", got, err, tt.want)
			}
		})
	}
}

func TestHasObjectFilesRefusesAnObjectsFolderThatIsALink(t *testing.T) {
	e := newEnv(t, map[string]string{"src/main.yue": ""})
	outside := t.TempDir()
	testkit.WriteFile(t, outside, "units.pkl", []byte(captainFile))
	testkit.LinkDir(t, outside, filepath.Join(e.Root, "objects"))
	has, err := HasObjectFiles(e.Root)
	if err == nil {
		t.Fatalf("HasObjectFiles = %v for a link, want a refusal", has)
	}
	diagErr := asDiagError(t, err, "the refusal")
	if diagErr.File != "objects" || !strings.HasPrefix(diagErr.Msg, "Symlinks are not supported: ") || diagErr.Hint == "" {
		t.Errorf("got %q in %q with the hint %q", diagErr.Msg, diagErr.File, diagErr.Hint)
	}
}

func TestEvaluateObjectsWritesItsModuleAndRunsPklOnItInTheProjectFolder(t *testing.T) {
	e := newEnv(t, map[string]string{
		"objects/units.pkl": "", "PklProject": PklProjectText(moonwell.Version, ""), "PklProject.deps.json": depsJSON(moonwell.Version),
	})
	var calls []runCall
	const evaluated = `{"heroes":{},"units":{"captain":{"id":"h000","base":"hfoo","source":"../objects/units.pkl","properties":{}}},` +
		`"buildings":{},"items":{},"abilities":{},"buffs":{},"upgrades":{}}`
	e.Run = fakeRun(&calls, map[string]env.RunResult{"my-pkl eval": {Stdout: evaluated}})
	objects, err := EvaluateObjects(background, e, "my-pkl")
	if err != nil {
		t.Fatal(err)
	}
	want := []runCall{{"my-pkl eval --format json --project-dir . .moonwell/objects.pkl", e.Root}}
	if len(calls) != 1 || calls[0] != want[0] {
		t.Errorf("ran %+v, want %+v", calls, want)
	}
	module, err := os.ReadFile(filepath.Join(e.Root, ".moonwell", "objects.pkl"))
	if err != nil || !strings.Contains(string(module), `Objects.merge(import*("../objects/**.pkl"))`) {
		t.Errorf("the module reads %q, %v", module, err)
	}
	captain, found := objects.Units.Get("captain")
	if !found || captain.ID != "h000" || captain.Source != "objects/units.pkl" {
		t.Errorf("captain = %+v, %v, want it from objects/units.pkl", captain, found)
	}
}

func TestEvaluateObjectsRefusesBeforeItRunsPklWhenThePackageIsNotResolvedOrOfAnotherVersion(t *testing.T) {
	for _, tt := range []struct {
		name  string
		files map[string]string
		file  string
		words string
	}{
		{"no PklProject", map[string]string{"objects/units.pkl": "", "PklProject.deps.json": depsJSON(moonwell.Version)},
			"PklProject", "This project has object files and no PklProject."},
		{"no resolved dependencies", map[string]string{"objects/units.pkl": "", "PklProject": PklProjectText(moonwell.Version, "")},
			"PklProject", "PklProject.deps.json is missing."},
		{"a package of another version", map[string]string{
			"objects/units.pkl": "", "PklProject": PklProjectText("0.1.0", ""), "PklProject.deps.json": depsJSON("0.1.0"),
		}, "PklProject", "0.1.0"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t, tt.files)
			_, err := EvaluateObjects(background, e, "pkl")
			if err == nil {
				t.Fatal("EvaluateObjects took it")
			}
			if diagErr := asDiagError(t, err, "the refusal"); diagErr.File != tt.file || !strings.Contains(diagErr.Msg, tt.words) || diagErr.Hint == "" {
				t.Errorf("got %q in %q with the hint %q, want %q in %s, with a hint", diagErr.Msg, diagErr.File, diagErr.Hint, tt.words, tt.file)
			}
			if _, err := os.Stat(filepath.Join(e.Root, ".moonwell")); err == nil {
				t.Error("the module was written before the package was checked")
			}
		})
	}
}

func TestEvaluateObjectsReportsWhatPklPrintsWhenItFailsAndOutputThatIsNoJSON(t *testing.T) {
	for _, tt := range []struct {
		name   string
		result env.RunResult
		want   string
	}{
		{"pkl fails", env.RunResult{ExitCode: 1, Stderr: "-- Pkl Error --\nat ../objects/units.pkl line 5\n"}, "at objects/units.pkl line 5"},
		{"pkl fails and prints to standard output", env.RunResult{ExitCode: 1, Stdout: "a refusal\n"}, "a refusal"},
		{"no JSON", env.RunResult{Stdout: "units {}\n"}, "units {}"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t, map[string]string{
				"objects/units.pkl": "", "PklProject": PklProjectText(moonwell.Version, ""), "PklProject.deps.json": depsJSON(moonwell.Version),
			})
			var calls []runCall
			e.Run = fakeRun(&calls, map[string]env.RunResult{"pkl eval": tt.result})
			_, err := EvaluateObjects(background, e, "pkl")
			if err == nil {
				t.Fatal("EvaluateObjects took it")
			}
			if diagErr := asDiagError(t, err, "the refusal"); diagErr.File != "objects" || !strings.Contains(diagErr.Msg, tt.want) || strings.Contains(diagErr.Msg, "../") {
				t.Errorf("got %q in %q, want %q in objects", diagErr.Msg, diagErr.File, tt.want)
			}
		})
	}
}

func TestEvaluateObjectsGathersEveryObjectFileWithRealPkl(t *testing.T) {
	e, pkl := newLinkedEnv(t, map[string]string{"objects/units.pkl": captainFile, "objects/heroes/paladin.pkl": paladinFile})
	objects, err := EvaluateObjects(background, e, pkl)
	if err != nil {
		t.Fatal(err)
	}
	captain, _ := objects.Units.Get("captain")
	paladin, _ := objects.Heroes.Get("paladin")
	if captain.Source != "objects/units.pkl" || paladin.Source != "objects/heroes/paladin.pkl" {
		t.Errorf("the captain is from %q and the paladin from %q, want their files below objects/", captain.Source, paladin.Source)
	}
	if name, _ := captain.Typed.Get("name"); captain.ID != "h000" || captain.Base != "hfoo" || name != "Captain" {
		t.Errorf("captain = %+v", captain)
	}
}

func TestEvaluateObjectsReportsAKeyThatTwoFilesDefineNamingBothWithRealPkl(t *testing.T) {
	e, pkl := newLinkedEnv(t, map[string]string{"objects/units.pkl": captainFile, "objects/more.pkl": captainFile})
	_, err := EvaluateObjects(background, e, pkl)
	if err == nil {
		t.Fatal("EvaluateObjects took a key that two files define")
	}
	message := asDiagError(t, err, "the refusal").Msg
	if !strings.Contains(message, "objects/units.pkl") || !strings.Contains(message, "objects/more.pkl") || strings.Contains(message, "../objects/") {
		t.Errorf("got %q, want both files named", message)
	}
}

func TestEvaluateObjectsReportsAValueOutsideTheSchemaNamingItsFileWithRealPkl(t *testing.T) {
	broken := strings.Replace(captainFile, `"h000"`, `"toolong"`, 1)
	e, pkl := newLinkedEnv(t, map[string]string{"objects/units.pkl": broken})
	_, err := EvaluateObjects(background, e, pkl)
	if err == nil {
		t.Fatal("EvaluateObjects took an id of seven characters")
	}
	if message := asDiagError(t, err, "the refusal").Msg; !strings.Contains(message, "units.pkl") || !strings.Contains(message, "toolong") {
		t.Errorf("got %q, want the file and the value named", message)
	}
}
