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

	"github.com/mdlsvensson/moonwell/next/internal/build"
	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/objects"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

// The tests of this file are about a project's objects: how the manifest merges the files under objects/, what
// objects:eval prints and objects:check says, and how far check and dev get with the ids module. Each runs the
// real pkl in a project that init made, and takes the time that takes. None needs the compiler: where a command
// asks for it, the world lets pkl alone run, and the command fails at the compiler's download.

// emptyObjectProject is a project that init made, without its object files and its ids module. Without wiring,
// its manifest does not merge the files under objects/ either.
func emptyObjectProject(t *testing.T, wiring bool) string {
	t.Helper()
	root := newProject(t, "map")
	remove(t, root, "objects")
	remove(t, root, objects.IDsFile)
	if !wiring {
		var kept []string
		for line := range strings.SplitSeq(read(t, root, "moonwell.pkl"), "\n") {
			if !strings.Contains(line, "Objects") {
				kept = append(kept, line)
			}
		}
		write(t, root, "moonwell.pkl", strings.Join(kept, "\n"))
	}
	return root
}

// objectFile is a file under objects/ with this body.
func objectFile(body string) string { return "amends \"@moonwell/ObjectFile.pkl\"\n\n" + body + "\n" }

var (
	captain = objectFile(`units { ["captain"] { id = "h000"; base = "hfoo"; name = "Captain" } }`)
	paladin = objectFile(`heroes { ["paladin"] { id = "H000"; base = "Hpal"; properties { ["uhpm"] = 900 } } }`)
	// onlyCaptain and bothObjects are the ids module of a project with the captain, and with the paladin too.
	onlyCaptain = objects.RenderIDs([]objects.Resolved{{Category: "units", Key: "captain", ID: "h000"}})
	bothObjects = objects.RenderIDs([]objects.Resolved{
		{Category: "heroes", Key: "paladin", ID: "H000"}, {Category: "units", Key: "captain", ID: "h000"},
	})
)

// idsLine is the line objects:check says of the ids module.
func idsLine(status string) string { return "  " + objects.IDsFile + ": " + status }

// writeObjects gives the project a hero in one file and a unit in another, two folders down.
func writeObjects(t *testing.T, root string) {
	t.Helper()
	write(t, root, "objects/heroes.pkl", paladin)
	write(t, root, "objects/human/barracks/units.pkl", captain)
}

// evaluatePkl has the real pkl print a manifest of root as JSON.
func evaluatePkl(t *testing.T, root, file string) env.RunResult {
	t.Helper()
	args := []string{"eval", "--format", "json", "--project-dir", ".", file}
	result, err := env.Run(background, "pkl", args, env.RunOptions{Dir: root})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// jsonObject is a JSON object as Go values.
func jsonObject(t *testing.T, text string) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		t.Fatal(err, text)
	}
	return value
}

// assertEmptyObjects fails unless value has every category, each without an object.
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

// ---- how the manifest merges the files under objects/ ----

func TestPklObjectsNestedMergeSources(t *testing.T) {
	root := emptyObjectProject(t, true)
	writeObjects(t, root)
	write(t, root, "objects/notes.txt", "not Pkl")
	r := evaluatePkl(t, root, "moonwell.local.pkl")
	if r.Code != 0 {
		t.Fatal(r.Stderr)
	}
	value := jsonObject(t, r.Stdout)["objects"].(map[string]any)
	hero := value["heroes"].(map[string]any)["paladin"].(map[string]any)
	unit := value["units"].(map[string]any)["captain"].(map[string]any)
	if hero["source"] != "objects/heroes.pkl" || unit["source"] != "objects/human/barracks/units.pkl" ||
		unit["name"] != "Captain" {
		t.Fatal(value)
	}
	r = evaluatePkl(t, root, "moonwell.pkl")
	if r.Code != 0 || !reflect.DeepEqual(jsonObject(t, r.Stdout)["objects"], value) {
		t.Fatal(r.Stderr)
	}
}

func TestPklObjectsMissingAndEmptyFolder(t *testing.T) {
	root := emptyObjectProject(t, true)
	for range 2 {
		r := evaluatePkl(t, root, "moonwell.local.pkl")
		if r.Code != 0 {
			t.Fatal(r.Stderr)
		}
		assertEmptyObjects(t, jsonObject(t, r.Stdout)["objects"].(map[string]any))
		write(t, root, "objects/empty/.gitkeep", "")
	}
}

func TestPklObjectsWithoutWiring(t *testing.T) {
	root := emptyObjectProject(t, false)
	write(t, root, "objects/heroes.pkl", paladin)
	r := evaluatePkl(t, root, "moonwell.local.pkl")
	if r.Code != 0 {
		t.Fatal(r.Stderr)
	}
	assertEmptyObjects(t, jsonObject(t, r.Stdout)["objects"].(map[string]any))
}

func TestPklObjectsDuplicateKeyNamesBothFiles(t *testing.T) {
	root := emptyObjectProject(t, true)
	write(t, root, "objects/a.pkl", paladin)
	write(t, root, "objects/b/c.pkl", paladin)
	r := evaluatePkl(t, root, "moonwell.local.pkl")
	if r.Code != 1 {
		t.Fatal(r)
	}
	contains(t, r.Stderr, `heroes["paladin"] is defined in both objects/a.pkl and objects/b/c.pkl`)
}

func TestPklObjectsInvalidObjectNamesOwnFile(t *testing.T) {
	root := emptyObjectProject(t, true)
	write(t, root, "objects/bad.pkl", objectFile(`units { ["captain"] { id = "H000"; base = "hfoo" } }`))
	r := evaluatePkl(t, root, "moonwell.local.pkl")
	if r.Code != 1 {
		t.Fatal(r)
	}
	contains(t, r.Stderr, "objects/bad.pkl")
}

// ---- objects:eval ----

// The lock file is that of a build that is long gone: objects:eval neither minds it nor takes it over.
func TestPklObjectsEvalStdoutOnlyWithoutCompilerOrLock(t *testing.T) {
	root := emptyObjectProject(t, true)
	writeObjects(t, root)
	write(t, root, "dist/.lock", "999999")
	before := testkit.Snapshot(t, root)
	e, log, ran := pklOnly(t, root)
	printed, err := commandIn(t, background, e, "objects:eval")
	if err != nil || len(printed) != 1 || len(log.Lines()) != 0 {
		t.Fatalf("%v %v %v", err, printed, log.Lines())
	}
	onlyPkl(t, ran)
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
	// The whole line: the JSON is all that is printed, on the stream for other programs, as one text that the
	// program ends with a line break.
	r := okWithPklAlone(t, root, "objects:eval")
	if r.output != "" || r.stdout != printed[0] || !strings.HasPrefix(r.stdout, "{\n  \"heroes\": {") ||
		!strings.HasSuffix(r.stdout, "}") {
		t.Fatalf("%+v", r)
	}
	sameFiles(t, before, testkit.Snapshot(t, root), "eval")
}

func TestPklObjectsEvalInvalidStderrOnly(t *testing.T) {
	root := emptyObjectProject(t, true)
	write(t, root, "objects/units.pkl", objectFile(`units { ["captain"] { id = "h000"; base = "zzzz" } }`))
	// "\xe2\x80\xba" is the mark between the place and the message.
	r := failsWithPklAlone(t, root, []string{
		"error: objects/units.pkl \xe2\x80\xba units[\"captain\"].base: 'zzzz' is not a standard unit.",
	}, "objects:eval")
	if r.stdout != "" {
		t.Fatal(r.stdout)
	}
}

func TestPklObjectsCommandsWithoutObjectsFolder(t *testing.T) {
	root := emptyObjectProject(t, true)
	before := testkit.Snapshot(t, root)
	r := okWithPklAlone(t, root, "objects:eval")
	assertEmptyObjects(t, jsonObject(t, r.stdout))
	e, log, ran := pklOnly(t, root)
	lines := logged(t, e, log, "objects:check")
	if !slices.Equal(lines, []string{
		idsLine("current"), "Object data valid: 0 object(s), 0 internal file(s) would change during build.",
	}) {
		t.Fatal(lines)
	}
	onlyPkl(t, ran)
	sameFiles(t, before, testkit.Snapshot(t, root), "empty objects")
}

// The two commands open the source map as a build does, so they need its folder also in a project whose
// manifest has no objects.
func TestPklObjectsCommandsNeedTheSourceMapAlsoWithoutObjects(t *testing.T) {
	root := emptyObjectProject(t, true)
	remove(t, root, "maps/map.w3x")
	for _, name := range []string{"objects:eval", "objects:check"} {
		e, log, _ := pklOnly(t, root)
		printed, err := commandIn(t, background, e, name)
		failure := asError(t, err, name+" without the map")
		if failure.File != "moonwell.local.pkl" || failure.Hint == "" ||
			!strings.Contains(failure.Msg, "Source map folder maps/map.w3x not found") {
			t.Errorf("%s: error = %+v", name, failure)
		}
		if len(printed) != 0 || len(log.Lines()) != 0 {
			t.Errorf("%s printed %q and logged %q before it failed", name, printed, log.Lines())
		}
	}
}

// ---- objects:check ----

func TestPklObjectsCheckMissingStaleCurrent(t *testing.T) {
	root := emptyObjectProject(t, true)
	writeObjects(t, root)
	before := testkit.Snapshot(t, filepath.Join(root, "maps"))
	r := failsWithPklAlone(t, root, []string{"  war3map.w3u\n  war3mapSkin.w3u\n" + idsLine("missing") + "\nerror: " +
		objects.IDsFile + " \xe2\x80\xba The file is missing, but the manifest has objects.\n" +
		"hint: Run moonwell build, test or dev to regenerate it."}, "objects:check")
	if r.stdout != "" || exists(root, objects.IDsFile) || strings.Contains(r.output, "Object data valid") {
		t.Fatalf("objects:check wrote the ids module, printed for other programs, or summed up a failure: %+v", r)
	}
	write(t, root, objects.IDsFile, objects.RenderIDs(nil))
	r = failsWithPklAlone(t, root,
		[]string{idsLine("stale"), "does not match the objects in the manifest."}, "objects:check")
	if strings.Contains(r.output, "Object data valid") {
		t.Fatalf("objects:check summed up a failure:\n%s", r.output)
	}
	write(t, root, objects.IDsFile, bothObjects)
	e, log, ran := pklOnly(t, root)
	lines := logged(t, e, log, "objects:check")
	if !slices.Equal(lines, []string{
		"  war3map.w3u", "  war3mapSkin.w3u", idsLine("current"),
		"Object data valid: 2 object(s), 2 internal file(s) would change during build.",
	}) {
		t.Fatal(lines)
	}
	onlyPkl(t, ran)
	sameFiles(t, before, testkit.Snapshot(t, filepath.Join(root, "maps")), "objects planning")
	if exists(root, "dist/stage") || exists(root, "dist/.lock") {
		t.Fatal("objects:check staged the map, or took the build lock")
	}
}

// Beside a build that runs, the two commands go on: they read the project and write nothing.
func TestPklObjectsCommandsTakeNoBuildLock(t *testing.T) {
	root := newProject(t, "map")
	holdBuildLock(t, root)
	if r := okWithPklAlone(t, root, "objects:check"); !strings.Contains(r.output, "Object data valid: 1 object(s)") {
		t.Errorf("objects:check beside a build: %+v", r)
	}
	if r := okWithPklAlone(t, root, "objects:eval"); !strings.Contains(r.stdout, `"captain"`) {
		t.Errorf("objects:eval beside a build: %+v", r)
	}
	if !exists(root, "dist/.lock") {
		t.Error("a command removed the lock of the build beside it")
	}
}

// ---- check and dev, as far as the ids module ----

// The world lets pkl alone run, so a check that gets as far as the compiler fails at its download, with the
// refusal as the failure's cause.
func TestPklCheckRefusesMissingStaleBeforeCompile(t *testing.T) {
	root := emptyObjectProject(t, true)
	writeObjects(t, root)
	e, _, _ := pklOnly(t, root)
	_, err := commandIn(t, background, e, "check")
	contains(t, asError(t, err, "missing ids").Msg, "is missing, but the manifest has objects")
	if exists(root, objects.IDsFile) {
		t.Fatal("check wrote the ids module")
	}
	write(t, root, objects.IDsFile, onlyCaptain)
	_, err = commandIn(t, background, e, "check")
	failure := asError(t, err, "stale ids")
	contains(t, failure.Msg, "does not match the objects in the manifest")
	if failure.Hint != "Run moonwell build, test or dev to regenerate it." ||
		read(t, root, objects.IDsFile) != onlyCaptain {
		t.Fatal(failure)
	}
	// A checkout with CRLF line endings has the module that the manifest renders.
	write(t, root, objects.IDsFile, strings.ReplaceAll(bothObjects, "\n", "\r\n"))
	_, err = commandIn(t, background, e, "check")
	failure = asError(t, err, "the compiler")
	if failure.Cause == nil || !strings.Contains(failure.Cause.Error(), "tried to") {
		t.Fatalf("check with a current ids module did not get as far as the compiler: %+v", failure)
	}
	write(t, root, "objects/bad.pkl", objectFile(`items { ["claws"] { id = "h000"; base = "ratf" } }`))
	_, err = commandIn(t, background, e, "check")
	if err == nil {
		t.Fatal("check took an object that is not valid")
	}
	contains(t, diag.Format(err), "objects/bad.pkl")
}

// devIn runs dev in the world e beside the test, at a pace of milliseconds, until the test ends: it is then told
// to stop, and a dev that has not ended half a minute later fails the test. until waits for done to hold,
// however long the machine takes over it within a minute, and fails the test with what dev logged when it does
// not, or when dev ends first.
func devIn(t *testing.T, e *env.Env, log *testkit.Recorder) (until func(what string, done func() bool)) {
	t.Helper()
	ctx, stop := context.WithCancel(background)
	ended := make(chan error, 1)
	go func() {
		ended <- build.Dev(ctx, e, build.Pace{Interval: 10 * time.Millisecond, Debounce: 20 * time.Millisecond})
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

// Each of dev's checks writes the ids module before it asks for the compiler: the world lets pkl alone run, so
// every check fails at the compiler's download, and the module is current all the same.
func TestPklDevRefreshesGeneratedObjectsOnChanges(t *testing.T) {
	root := emptyObjectProject(t, true)
	write(t, root, "objects/human/barracks/units.pkl", captain)
	e, log, _ := pklOnly(t, root)
	until := devIn(t, e, log)
	watching := "Watching src/, assets/, objects/, lua/ and the project manifests."
	until("the line that says what it watches", func() bool {
		return slices.ContainsFunc(log.Lines(), func(line string) bool { return strings.HasPrefix(line, watching) })
	})
	if read(t, root, objects.IDsFile) != onlyCaptain {
		t.Fatal("dev's first check did not write the ids module")
	}
	write(t, root, "objects/heroes.pkl", paladin)
	// The module is read as it is on disk, and a look may find it while dev writes it.
	until("an ids module with both objects", func() bool {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(objects.IDsFile)))
		return err == nil && string(data) == bothObjects
	})
	if lines := strings.Join(log.Lines(), "\n"); strings.Contains(lines, "Check passed") {
		t.Errorf("a check passed in a world without a compiler:\n%s", lines)
	}
}
