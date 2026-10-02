package cli_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mdlsvensson/moonwell/internal/cli"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/layout"
	"github.com/mdlsvensson/moonwell/internal/logging"
	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/proc"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func emptyObjectProject(t *testing.T, wiring bool) string {
	t.Helper()
	root := newProject(t, "map")
	remove(t, root, "objects")
	remove(t, root, layout.ObjectIDsFile)
	if !wiring {
		text := read(t, root, "moonwell.pkl")
		var lines []string
		for _, line := range strings.Split(text, "\n") {
			if strings.Contains(line, "Objects") {
				continue
			}
			lines = append(lines, line)
		}
		write(t, root, "moonwell.pkl", strings.Join(lines, "\n"))
	}
	return root
}

func objectFile(body string) string { return "amends \"@moonwell/ObjectFile.pkl\"\n\n" + body + "\n" }

var captain = objectFile(`units { ["captain"] { id = "h000"; base = "hfoo"; name = "Captain" } }`)
var paladin = objectFile(`heroes { ["paladin"] { id = "H000"; base = "Hpal"; properties { ["uhpm"] = 900 } } }`)
var generated = objects.RenderIDs([]objects.Resolved{{Category: "heroes", Key: "paladin", ID: "H000"}, {Category: "units", Key: "captain", ID: "h000"}})

func writeObjects(t *testing.T, root string) {
	t.Helper()
	write(t, root, "objects/heroes.pkl", paladin)
	write(t, root, "objects/human/barracks/units.pkl", captain)
}

func evaluatePkl(t *testing.T, root, file string) proc.Result {
	t.Helper()
	r, err := proc.Run(background, "pkl", []string{"eval", "--format", "json", "--project-dir", ".", file}, proc.Options{Dir: root})
	if err != nil {
		t.Fatal(err)
	}
	return r
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
	if len(value) != len(objects.Categories) {
		t.Fatal(value)
	}
	for _, category := range objects.Categories {
		item, ok := value[string(category)].(map[string]any)
		if !ok || len(item) != 0 {
			t.Fatal(value)
		}
	}
}

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
	if hero["source"] != "objects/heroes.pkl" || unit["source"] != "objects/human/barracks/units.pkl" || unit["name"] != "Captain" {
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

func TestPklObjectsEvalStdoutOnlyWithoutCompilerOrLock(t *testing.T) {
	root := emptyObjectProject(t, true)
	writeObjects(t, root)
	write(t, root, "dist/.lock", "999999")
	before := testkit.Snapshot(t, root)
	env, log, _ := pklOnly(t, root)
	var printed []string
	_, err := cli.ObjectsEval(background, env, func(s string) { printed = append(printed, s) })
	if err != nil || len(printed) != 1 || len(log.Lines) != 0 {
		t.Fatalf("%v %v %v", err, printed, log.Lines)
	}
	value := jsonObject(t, printed[0])
	if len(value) != 7 {
		t.Fatal(value)
	}
	unit := value["units"].(map[string]any)["captain"].(map[string]any)
	wanted := jsonObject(t, `{"id":"h000","base":"hfoo","source":"objects/human/barracks/units.pkl","fields":[{"rawcode":"unam","name":"name","level":0,"column":0,"skin":true,"type":"string","value":"Captain"}]}`)
	if !reflect.DeepEqual(unit, wanted) {
		t.Fatal(unit)
	}
	hero := value["heroes"].(map[string]any)["paladin"].(map[string]any)
	if hero["source"] != "objects/heroes.pkl" || hero["fields"].([]any)[0].(map[string]any)["rawcode"] != "uhpm" {
		t.Fatal(hero)
	}
	r := ok(t, root, "objects:eval")
	if r.output != "" || !reflect.DeepEqual(jsonObject(t, r.stdout), value) {
		t.Fatal(r)
	}
	sameFiles(t, before, testkit.Snapshot(t, root), "eval")
}

func TestPklObjectsEvalInvalidStderrOnly(t *testing.T) {
	root := emptyObjectProject(t, true)
	write(t, root, "objects/units.pkl", objectFile(`units { ["captain"] { id = "h000"; base = "zzzz" } }`))
	r := fails(t, root, []string{`error: objects/units.pkl › units["captain"].base: 'zzzz' is not a standard unit.`}, "objects:eval")
	if r.stdout != "" {
		t.Fatal(r.stdout)
	}
}

func TestPklObjectsCommandsWithoutObjectsFolder(t *testing.T) {
	root := emptyObjectProject(t, true)
	before := testkit.Snapshot(t, root)
	r := ok(t, root, "objects:eval")
	assertEmptyObjects(t, jsonObject(t, r.stdout))
	env, log, _ := pklOnly(t, root)
	if _, err := cli.ObjectsCheck(background, env); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(log.Lines, []string{"  " + layout.ObjectIDsFile + ": current", "Object data valid: 0 object(s), 0 internal file(s) would change during build."}) {
		t.Fatal(log.Lines)
	}
	sameFiles(t, before, testkit.Snapshot(t, root), "empty objects")
}

func TestPklObjectsCheckMissingStaleCurrent(t *testing.T) {
	root := emptyObjectProject(t, true)
	writeObjects(t, root)
	before := testkit.Snapshot(t, filepath.Join(root, "maps"))
	r := fails(t, root, []string{"  war3map.w3u\n  war3mapSkin.w3u\n  " + layout.ObjectIDsFile + ": missing\nerror: " + layout.ObjectIDsFile + " › The file is missing, but the manifest has objects.\nhint: Run moonwell build, test or dev to regenerate it."}, "objects:check")
	if r.stdout != "" || exists(root, layout.ObjectIDsFile) {
		t.Fatal("check wrote generated module")
	}
	write(t, root, layout.ObjectIDsFile, objects.RenderIDs(nil))
	fails(t, root, []string{layout.ObjectIDsFile + ": stale", "does not match the objects in the manifest."}, "objects:check")
	write(t, root, layout.ObjectIDsFile, generated)
	env, log, _ := pklOnly(t, root)
	plan, err := cli.ObjectsCheck(background, env)
	if err != nil || len(plan.Objects) != 2 {
		t.Fatalf("%v %+v", err, plan)
	}
	if !reflect.DeepEqual(log.Lines, []string{"  war3map.w3u", "  war3mapSkin.w3u", "  " + layout.ObjectIDsFile + ": current", "Object data valid: 2 object(s), 2 internal file(s) would change during build."}) {
		t.Fatal(log.Lines)
	}
	sameFiles(t, before, testkit.Snapshot(t, filepath.Join(root, "maps")), "objects planning")
	if exists(root, "dist/stage") || exists(root, "dist/.lock") {
		t.Fatal("check staged")
	}
}

func TestPklCheckRefusesMissingStaleBeforeCompile(t *testing.T) {
	root := emptyObjectProject(t, true)
	writeObjects(t, root)
	env, _, _ := pklOnly(t, root)
	_, err := cli.Check(background, env, false)
	contains(t, asError(t, err, "missing IDs").Msg, "is missing, but the manifest has objects")
	if exists(root, layout.ObjectIDsFile) {
		t.Fatal("check generated")
	}
	stale := objects.RenderIDs([]objects.Resolved{{Category: "units", Key: "captain", ID: "h000"}})
	write(t, root, layout.ObjectIDsFile, stale)
	_, err = cli.Check(background, env, false)
	e := asError(t, err, "stale IDs")
	contains(t, e.Msg, "does not match the objects in the manifest")
	if e.Hint != "Run moonwell build, test or dev to regenerate it." || read(t, root, layout.ObjectIDsFile) != stale {
		t.Fatal(e)
	}
	write(t, root, layout.ObjectIDsFile, strings.ReplaceAll(generated, "\n", "\r\n"))
	_, err = cli.Check(background, env, false)
	contains(t, asError(t, err, "compiler").Msg, "tried to")
	write(t, root, "objects/bad.pkl", objectFile(`items { ["claws"] { id = "h000"; base = "ratf" } }`))
	_, err = cli.Check(background, env, false)
	if err == nil {
		t.Fatal("invalid object accepted")
	}
	contains(t, diag.Format(err), "objects/bad.pkl")
}

// startDev records complete terminal lines on a channel: the tests never read a logger's slice while dev writes it.
func startDev(t *testing.T, root string, pklOnlyRun bool) (wait func(string), stop func()) {
	t.Helper()
	env, _, _ := pklOnly(t, root)
	if !pklOnlyRun {
		env, _ = newEnv(root)
	}
	lines := make(chan string, 256)
	env.Log = logging.New(func(s string) { lines <- s }, "")
	ctx, cancel := context.WithCancel(background)
	done := make(chan error, 1)
	go func() {
		done <- cli.Dev(ctx, env, cli.DevOptions{Interval: 10 * time.Millisecond, Debounce: 20 * time.Millisecond})
	}()
	stopped := false
	stop = func() {
		if stopped {
			return
		}
		stopped = true
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(30 * time.Second):
			t.Error("dev did not stop")
		}
	}
	t.Cleanup(stop)
	wait = func(wanted string) {
		t.Helper()
		timer := time.NewTimer(30 * time.Second)
		defer timer.Stop()
		var seen []string
		for {
			select {
			case line := <-lines:
				seen = append(seen, line)
				if strings.Contains(line, wanted) {
					return
				}
			case <-timer.C:
				t.Fatalf("waiting for %q:\n%s", wanted, strings.Join(seen, "\n"))
			case err := <-done:
				t.Fatalf("dev ended: %v", err)
			}
		}
	}
	return
}

func TestPklDevRefreshesGeneratedObjectsOnChanges(t *testing.T) {
	root := emptyObjectProject(t, true)
	write(t, root, "objects/human/barracks/units.pkl", captain)
	wait, stop := startDev(t, root, true)
	defer stop()
	wait("Watching src/, assets/, objects/, lua/ and the project manifests.")
	if read(t, root, layout.ObjectIDsFile) != objects.RenderIDs([]objects.Resolved{{Category: "units", Key: "captain", ID: "h000"}}) {
		t.Fatal("initial generation")
	}
	write(t, root, "objects/heroes.pkl", paladin)
	wait("tried to")
	if read(t, root, layout.ObjectIDsFile) != generated {
		t.Fatal("regeneration")
	}
}
