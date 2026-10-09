package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mdlsvensson/moonwell/internal/build"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func emptyObjectProject(t *testing.T) string {
	t.Helper()
	root := newProject(t, "map")
	removeFile(t, root, "objects")
	removeFile(t, root, objects.IDsFile)
	return root
}

func objectFile(body string) string { return "amends \"@moonwell/ObjectFile.pkl\"\n\n" + body + "\n" }

var (
	captain        = objectFile(`units { ["captain"] { id = "h000"; base = "hfoo"; name = "Captain" } }`)
	paladin        = objectFile(`heroes { ["paladin"] { id = "H000"; base = "Hpal"; properties { ["uhpm"] = 900 } } }`)
	onlyCaptain, _ = objects.RenderIDs([]objects.Resolved{{Category: "units", Key: "captain", ID: "h000"}})
	bothObjects, _ = objects.RenderIDs([]objects.Resolved{
		{Category: "heroes", Key: "paladin", ID: "H000"}, {Category: "units", Key: "captain", ID: "h000"},
	})
	noObjects, _ = objects.RenderIDs(nil)
)

func idsLine(status string) string { return "  " + objects.IDsFile + ": " + status }

func writeObjects(t *testing.T, root string) {
	t.Helper()
	writeFile(t, root, "objects/heroes.pkl", paladin)
	writeFile(t, root, "objects/human/barracks/units.pkl", captain)
}

func jsonObject(t *testing.T, text string) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		t.Fatal(err, text)
	}
	return value
}

func assertEmptyObjects(t *testing.T, value map[string]any) {
	t.Helper()
	if len(value) != len(manifest.Categories) {
		t.Fatal(value)
	}
	for _, category := range manifest.Categories {
		if objects, isObject := value[string(category)].(map[string]any); !isObject || len(objects) != 0 {
			t.Fatal(value)
		}
	}
}

func TestPklObjectsEvalStdoutOnlyWithoutCompilerOrLock(t *testing.T) {
	root := emptyObjectProject(t)
	writeObjects(t, root)
	writeFile(t, root, "dist/.lock", "999999")
	before := testkit.Snapshot(t, root)
	e, log, ran := newPklOnlyEnv(t, root)
	printed, err := runCommandIn(t, background, e, "objects:eval")
	if err != nil || len(printed) != 1 || len(log.Lines()) != 0 {
		t.Fatalf("%v %v %v", err, printed, log.Lines())
	}
	checkOnlyPklRan(t, ran)
	value := jsonObject(t, printed[0])
	if len(value) != 7 {
		t.Fatal(value)
	}
	unit := value["units"].(map[string]any)["captain"].(map[string]any)
	wanted := jsonObject(t, `{"id":"h000","base":"hfoo","source":"objects/human/barracks/units.pkl","fields":[`+
		`{"rawcode":"unam","name":"name","level":0,"column":0,"skin":true,"type":"string","value":"Captain"}]}`)
	if !reflect.DeepEqual(unit, wanted) {
		t.Fatal(unit)
	}
	hero := value["heroes"].(map[string]any)["paladin"].(map[string]any)
	if hero["source"] != "objects/heroes.pkl" || hero["fields"].([]any)[0].(map[string]any)["rawcode"] != "uhpm" {
		t.Fatal(hero)
	}
	r := mustSucceedWithPklOnly(t, root, "objects:eval")
	if r.output != "" || r.stdout != printed[0] || !strings.HasPrefix(r.stdout, "{\n  \"heroes\": {") ||
		!strings.HasSuffix(r.stdout, "}") {
		t.Fatalf("%+v", r)
	}
	after := testkit.Snapshot(t, root)
	if _, written := after[manifest.ObjectsModule]; !written {
		t.Errorf("eval left no %s", manifest.ObjectsModule)
	}
	delete(after, manifest.ObjectsModule)
	delete(after, ".moonwell")
	checkSameFiles(t, before, after, "eval")
}

func TestPklObjectsEvalInvalidStderrOnly(t *testing.T) {
	root := emptyObjectProject(t)
	writeFile(t, root, "objects/units.pkl", objectFile(`units { ["captain"] { id = "h000"; base = "zzzz" } }`))
	r := mustFailWithPklOnly(t, root, []string{
		"error: objects/units.pkl \xe2\x80\xba units[\"captain\"].base: 'zzzz' is not a standard unit.",
	}, "objects:eval")
	if r.stdout != "" {
		t.Fatal(r.stdout)
	}
}

func TestPklObjectsCommandsWithoutObjectsFolder(t *testing.T) {
	root := emptyObjectProject(t)
	before := testkit.Snapshot(t, root)
	r := mustSucceedWithPklOnly(t, root, "objects:eval")
	assertEmptyObjects(t, jsonObject(t, r.stdout))
	e, log, ran := newPklOnlyEnv(t, root)
	lines := mustRunCommand(t, e, log, "objects:check")
	if !slices.Equal(lines, []string{
		idsLine("current"), "Object data valid: 0 object(s), 0 internal file(s) would change during build.",
	}) {
		t.Fatal(lines)
	}
	checkNoProgramRan(t, ran)
	checkSameFiles(t, before, testkit.Snapshot(t, root), "empty objects")
}

func TestPklObjectsCommandsNeedTheSourceMapAlsoWithoutObjects(t *testing.T) {
	root := emptyObjectProject(t)
	removeFile(t, root, "maps/map.w3x")
	for _, name := range []string{"objects:eval", "objects:check"} {
		e, log, _ := newPklOnlyEnv(t, root)
		printed, err := runCommandIn(t, background, e, name)
		diagErr := asDiagError(t, err, name+" without the map")
		if diagErr.File != manifest.ProjectFile || diagErr.Hint == "" ||
			!strings.Contains(diagErr.Msg, "Source map folder maps/map.w3x not found") {
			t.Errorf("%s: error = %+v", name, diagErr)
		}
		if len(printed) != 0 || len(log.Lines()) != 0 {
			t.Errorf("%s printed %q and logged %q before it failed", name, printed, log.Lines())
		}
	}
}

func TestPklObjectsCheckMissingStaleCurrent(t *testing.T) {
	root := emptyObjectProject(t)
	writeObjects(t, root)
	before := testkit.Snapshot(t, filepath.Join(root, "maps"))
	r := mustFailWithPklOnly(t, root, []string{"  war3map.w3u\n  war3mapSkin.w3u\n" + idsLine("missing") + "\nerror: " +
		objects.IDsFile + " \xe2\x80\xba The file is missing, but the project has objects.\n" +
		"hint: Run moonwell build, test or dev to regenerate it."}, "objects:check")
	if r.stdout != "" || exists(root, objects.IDsFile) || strings.Contains(r.output, "Object data valid") {
		t.Fatalf("objects:check wrote the ids module, printed for other programs, or summed up a failure: %+v", r)
	}
	writeFile(t, root, objects.IDsFile, noObjects)
	r = mustFailWithPklOnly(t, root,
		[]string{idsLine("stale"), "does not match the project's objects."}, "objects:check")
	if strings.Contains(r.output, "Object data valid") {
		t.Fatalf("objects:check summed up a failure:\n%s", r.output)
	}
	writeFile(t, root, objects.IDsFile, bothObjects)
	e, log, ran := newPklOnlyEnv(t, root)
	lines := mustRunCommand(t, e, log, "objects:check")
	if !slices.Equal(lines, []string{
		"  war3map.w3u", "  war3mapSkin.w3u", idsLine("current"),
		"Object data valid: 2 object(s), 2 internal file(s) would change during build.",
	}) {
		t.Fatal(lines)
	}
	checkOnlyPklRan(t, ran)
	checkSameFiles(t, before, testkit.Snapshot(t, filepath.Join(root, "maps")), "objects planning")
	if exists(root, "dist/stage") || exists(root, "dist/.lock") {
		t.Fatal("objects:check staged the map, or took the build lock")
	}
}

func TestPklObjectsCommandsTakeNoBuildLock(t *testing.T) {
	root := newProject(t, "map")
	holdBuildLock(t, root)
	if r := mustSucceedWithPklOnly(t, root, "objects:check"); !strings.Contains(r.output, "Object data valid: 1 object(s)") {
		t.Errorf("objects:check beside a build: %+v", r)
	}
	if r := mustSucceedWithPklOnly(t, root, "objects:eval"); !strings.Contains(r.stdout, `"captain"`) {
		t.Errorf("objects:eval beside a build: %+v", r)
	}
	if !exists(root, "dist/.lock") {
		t.Error("a command removed the lock of the build beside it")
	}
}

func TestPklCheckRefusesMissingStaleBeforeCompile(t *testing.T) {
	root := emptyObjectProject(t)
	writeObjects(t, root)
	e, _, _ := newPklOnlyEnv(t, root)
	_, err := runCommandIn(t, background, e, "check")
	checkContains(t, asDiagError(t, err, "missing ids").Msg, "is missing, but the project has objects")
	if exists(root, objects.IDsFile) {
		t.Fatal("check wrote the ids module")
	}
	writeFile(t, root, objects.IDsFile, onlyCaptain)
	_, err = runCommandIn(t, background, e, "check")
	diagErr := asDiagError(t, err, "stale ids")
	checkContains(t, diagErr.Msg, "does not match the project's objects")
	if diagErr.Hint != "Run moonwell build, test or dev to regenerate it." ||
		readFile(t, root, objects.IDsFile) != onlyCaptain {
		t.Fatal(diagErr)
	}
	writeFile(t, root, objects.IDsFile, strings.ReplaceAll(bothObjects, "\n", "\r\n"))
	_, err = runCommandIn(t, background, e, "check")
	diagErr = asDiagError(t, err, "the compiler")
	if diagErr.Cause == nil || !strings.Contains(diagErr.Cause.Error(), "tried to") {
		t.Fatalf("check with a current ids module did not get as far as the compiler: %+v", diagErr)
	}
	writeFile(t, root, "objects/bad.pkl", objectFile(`items { ["claws"] { id = "h000"; base = "ratf" } }`))
	_, err = runCommandIn(t, background, e, "check")
	if err == nil {
		t.Fatal("check took an object that is not valid")
	}
	checkContains(t, diag.Format(err), "objects/bad.pkl")
}

func startDevIn(t *testing.T, e *env.Env, log *testkit.LogRecorder) (until func(what string, done func() bool)) {
	t.Helper()
	ctx, stop := context.WithCancel(background)
	ended := make(chan error, 1)
	go func() {
		ended <- build.Dev(ctx, e, build.WatchTiming{Interval: 10 * time.Millisecond, Debounce: 20 * time.Millisecond})
	}()
	t.Cleanup(func() {
		stop()
		select {
		case err := <-ended:
			if err != nil {
				t.Errorf("dev = %v", diag.Format(err))
			}
		case <-time.After(30 * time.Second):
			t.Errorf("dev did not end when it was told to stop; it logged %q", log.Lines())
		}
	})
	return func(what string, done func() bool) {
		t.Helper()
		for deadline := time.Now().Add(time.Minute); !done(); time.Sleep(5 * time.Millisecond) {
			if time.Now().After(deadline) || len(ended) > 0 {
				t.Fatalf("dev did not get to %s; it logged %q", what, log.Lines())
			}
		}
	}
}

func TestPklDevRefreshesGeneratedObjectsOnChanges(t *testing.T) {
	root := emptyObjectProject(t)
	writeFile(t, root, "objects/human/barracks/units.pkl", captain)
	e, log, _ := newPklOnlyEnv(t, root)
	until := startDevIn(t, e, log)
	watching := "Watching src/, assets/, objects/, lua/ and the project's settings."
	until("the line that says what it watches", func() bool {
		return slices.ContainsFunc(log.Lines(), func(line string) bool { return strings.HasPrefix(line, watching) })
	})
	if readFile(t, root, objects.IDsFile) != onlyCaptain {
		t.Fatal("dev's first check did not write the ids module")
	}
	writeFile(t, root, "objects/heroes.pkl", paladin)
	until("an ids module with both objects", func() bool {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(objects.IDsFile)))
		return err == nil && string(data) == bothObjects
	})
	if lines := strings.Join(log.Lines(), "\n"); strings.Contains(lines, "Check passed") {
		t.Errorf("a check passed in a world without a compiler:\n%s", lines)
	}
}
