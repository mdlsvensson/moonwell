package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

const modulePath = "github.com/mdlsvensson/moonwell"

const moduleFile = "module " + modulePath + "\n"

const anotherModule = "module example.com/other\n"

var outputFolders = []string{"data", "schema/generated"}

type fakeCheckout struct {
	t    testing.TB
	root string
	none bool
}

func newFakeCheckout(t testing.TB) fakeCheckout {
	t.Helper()
	c := fakeCheckout{t: t, root: t.TempDir()}
	c.writeFile("go.mod", moduleFile)
	return c
}

func newDirWithoutCheckout(t testing.TB, goMod string) fakeCheckout {
	t.Helper()
	c := fakeCheckout{t: t, root: t.TempDir(), none: true}
	if goMod != "" {
		c.writeFile("go.mod", goMod)
	}
	return c
}

func (c fakeCheckout) path(name string) string {
	return filepath.Join(c.root, filepath.FromSlash(name))
}

func (c fakeCheckout) writeFile(name, text string) {
	c.t.Helper()
	testkit.WriteFile(c.t, c.root, name, []byte(text))
}

func (c fakeCheckout) makeDir(name string) string {
	c.t.Helper()
	dir := c.path(name)
	if err := os.MkdirAll(dir, 0o777); err != nil {
		c.t.Fatal(err)
	}
	return dir
}

func (c fakeCheckout) copyRealFiles(names ...string) {
	c.t.Helper()
	for _, name := range names {
		for _, file := range realFiles(c.t, name) {
			c.writeFile(file, string(realFile(c.t, file)))
		}
	}
}

func realFile(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(testkit.RepoRoot(t), filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func realFiles(t testing.TB, name string) []string {
	t.Helper()
	root := testkit.RepoRoot(t)
	var files []string
	found := func(file string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		below, err := filepath.Rel(root, file)
		files = append(files, filepath.ToSlash(below))
		return err
	}
	if err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(name)), found); err != nil {
		t.Fatal(err)
	}
	return files
}

func (c fakeCheckout) runGen(args ...string) (output string, files map[string][]byte, err error) {
	c.t.Helper()
	output, err = c.runGenBelow("", args...)
	return output, c.readAll(), err
}

func (c fakeCheckout) runGenBelow(below string, args ...string) (output string, err error) {
	c.t.Helper()
	dir := c.startsIn(below)
	var out bytes.Buffer
	err = run(dir, args, &out)
	return out.String(), err
}

func (c fakeCheckout) startsIn(below string) (dir string) {
	c.t.Helper()
	checkNotRealCheckout(c.t, c.path(below))
	checkWritesOnlyOwnCheckout(c.t, c.root, c.path(below), !c.none)
	return c.makeDir(below)
}

func (c fakeCheckout) readAll() map[string][]byte {
	c.t.Helper()
	return testkit.Snapshot(c.t, c.root)
}

func asNew(files map[string][]byte) bool {
	return len(files) == 1 && string(files["go.mod"]) == moduleFile
}

func withGoMod(files map[string]string) map[string]string {
	files["go.mod"] = moduleFile
	return files
}

func (c fakeCheckout) readOutputs() map[string][]byte {
	c.t.Helper()
	found := map[string][]byte{}
	for _, dir := range outputFolders {
		if !fsx.IsDir(c.path(dir)) {
			continue
		}
		found[dir] = nil
		for name, data := range testkit.Snapshot(c.t, c.path(dir)) {
			found[dir+"/"+name] = data
		}
	}
	return found
}

const generatorPackage = "./tools/gen"

func buildProgram(t testing.TB, pkg string) string {
	t.Helper()
	program := filepath.Join(t.TempDir(), "gen")
	if runtime.GOOS == "windows" {
		program += ".exe"
	}
	build := exec.Command("go", "build", "-o", program, pkg)
	build.Dir = testkit.RepoRoot(t)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("%s does not build: %v\n%s", pkg, err, output)
	}
	return program
}

func (c fakeCheckout) runProgram(program, below string, args ...string) (code int, stdout, stderr string) {
	c.t.Helper()
	dir := c.startsIn(below)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	started := exec.CommandContext(ctx, program, args...)
	started.Dir = dir
	var printed, said bytes.Buffer
	started.Stdout, started.Stderr = &printed, &said
	if err := started.Run(); err != nil {
		var exited *exec.ExitError
		if !errors.As(err, &exited) {
			c.t.Fatal(err)
		}
		code = exited.ExitCode()
	}
	return code, printed.String(), said.String()
}

func checkNotRealCheckout(t testing.TB, dir string) {
	t.Helper()
	realCheckout, err := os.Stat(testkit.RepoRoot(t))
	if err != nil {
		t.Fatal(err)
		return
	}
	full, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
		return
	}
	for at := full; ; at = filepath.Dir(at) {
		if info, err := os.Stat(at); err == nil && os.SameFile(info, realCheckout) {
			t.Fatalf("%q is in the real checkout, %s: no generator is run there", dir, at)
			return
		}
		if at == filepath.Dir(at) {
			return
		}
	}
}

func checkWritesOnlyOwnCheckout(t testing.TB, root, dir string, ofACheckout bool) {
	t.Helper()
	if below, err := filepath.Rel(root, dir); err != nil || !filepath.IsLocal(below) {
		t.Fatalf("%s is not the folder of the run, %s, nor below it: nothing is started there", dir, root)
		return
	}
	for above := filepath.Dir(root); ; above = filepath.Dir(above) {
		data, err := os.ReadFile(filepath.Join(above, "go.mod"))
		if err == nil && bytes.Contains(data, []byte(modulePath)) {
			t.Fatalf("%s is inside a checkout, %s: a generator that is started there could write into it", root, above)
			return
		}
		if above == filepath.Dir(above) {
			break
		}
	}
	if isCheckout := namesTheModuleUpTo(root, dir); isCheckout != ofACheckout {
		t.Fatalf("at or above %s a go.mod names this module: %v; the run is of a checkout: %v",
			dir, isCheckout, ofACheckout)
	}
}

func namesTheModuleUpTo(root, dir string) bool {
	for at := dir; ; at = filepath.Dir(at) {
		if data, err := os.ReadFile(filepath.Join(at, "go.mod")); err == nil && moduleLine.Match(data) {
			return true
		}
		if at == root || at == filepath.Dir(at) {
			return false
		}
	}
}

type recordingTB struct {
	testing.TB
	reports []string
}

type stopped struct{}

func (l *recordingTB) Helper()                   {}
func (l *recordingTB) Logf(string, ...any)       {}
func (l *recordingTB) Error(args ...any)         { l.reports = append(l.reports, fmt.Sprint(args...)) }
func (l *recordingTB) Errorf(f string, a ...any) { l.reports = append(l.reports, fmt.Sprintf(f, a...)) }
func (l *recordingTB) Fatal(args ...any)         { l.Error(args...); panic(stopped{}) }
func (l *recordingTB) Fatalf(f string, a ...any) { l.Errorf(f, a...); panic(stopped{}) }

func captureReports(t testing.TB, reporting func(tb testing.TB)) string {
	t.Helper()
	heard := &recordingTB{TB: t}
	func() {
		defer func() {
			if raised := recover(); raised != nil && raised != (stopped{}) {
				panic(raised)
			}
		}()
		reporting(heard)
	}()
	return strings.Join(heard.reports, "\n")
}

func toTexts(outputs map[string][]byte) map[string]string {
	files := map[string]string{}
	for name, data := range outputs {
		if data != nil {
			files[name] = string(data)
		}
	}
	return files
}

func writeExportFile(t testing.TB, name, text string) string {
	t.Helper()
	return testkit.WriteFile(t, t.TempDir(), name, []byte(text))
}

func newSLK(columns []string, rows ...[]any) string {
	lines := []string{"ID;PWXL;N;E"}
	header := make([]any, len(columns))
	for i, column := range columns {
		header[i] = column
	}
	for y, row := range append([][]any{header}, rows...) {
		first := true
		for x, cell := range row {
			if cell == nil {
				continue
			}
			line := "C;X" + strconv.Itoa(x+1) + ";"
			if first {
				line += "Y" + strconv.Itoa(y+1) + ";"
			}
			switch cell := cell.(type) {
			case int:
				line += "K" + strconv.Itoa(cell)
			case string:
				line += `K"` + cell + `"`
			}
			lines, first = append(lines, line), false
		}
	}
	return strings.Join(append(lines, "E", ""), "\r\n")
}

var (
	unitMeta = []string{
		"ID", "field", "slk", "index", "category", "displayName", "type", "useHero", "useUnit", "useBuilding", "useItem",
		"useSpecific", "netsafe",
	}
	abilityMeta = []string{
		"ID", "field", "slk", "index", "repeat", "data", "category", "displayName", "type", "useUnit", "useHero", "useItem",
		"useSpecific", "notSpecific", "netsafe",
	}
	buffMeta    = []string{"ID", "field", "category", "displayName", "type", "netsafe"}
	upgradeMeta = []string{"ID", "field", "repeat", "effectType", "category", "displayName", "type", "netsafe"}
	balanceMeta = []string{"unitBalanceID", "isbldg", "Primary"}
)

const (
	humanUnitStrings    = stringsDir + "/humanunitstrings.txt"
	humanAbilityStrings = stringsDir + "/humanabilitystrings.txt"
	humanUpgradeStrings = stringsDir + "/humanupgradestrings.txt"
	itemStrings         = stringsDir + "/itemstrings.txt"
)

func miniExport() map[string]string {
	return map[string]string{
		unitFieldsTable: newSLK(unitMeta,
			[]any{"uhpm", "HP", "UnitBalance", -1, "stats", "WESTRING_UHPM", "int", 1, 1, 1, 0, nil, 0},
			[]any{"unam", "Name", "Profile", 0, "text", "WESTRING_UNAM", "string", 1, 1, 1, 1, nil, 1},
			[]any{"umdl", "file", "Profile", 0, "art", "WESTRING_UMDL", "model", 1, 1, 0, 0, nil, 1},
			[]any{"ifil", "file", "ItemData", 0, "art", "WESTRING_IFIL", "model", 0, 0, 0, 1, nil, 1},
			[]any{"ushr", "shadowOnWater", "Profile", -1, "art", "WESTRING_USHR", "bool", 0, 0, 1, 0, nil, 11},
			[]any{"uabi", "abilList", "UnitAbilities", -1, "abil", "WESTRING_UABI", "abilityList", 1, 1, 1, 0, nil, 0},
			[]any{"udea", "deathType", "UnitData", -1, "stats", "WESTRING_UDEA", "deathType", 1, 1, 1, 0, nil, 0},
			[]any{"upro", "Propernames", "Profile", -1, "text", "WESTRING_UPRO", "stringList", 1, 0, 0, 0, nil, 1},
			[]any{"ucls", "class", "Profile", -1, "stats", "WESTRING_UCLS", "string", 1, 1, 1, 0, nil, 0},
			[]any{"uver", "fileVerFlags", "Profile", -1, "art", "WESTRING_UVER", "versionFlags", 1, 1, 1, 0, nil, 1},
			[]any{nil, "orphan", "Profile", -1, "stats", "WESTRING_UHPM", "int", 1, 1, 1, 0, nil, 0},
		),
		abilityFieldsTable: newSLK(abilityMeta,
			[]any{"anam", "Name", "Profile", 0, 0, 0, "text", "WESTRING_ANAM", "string", 1, 1, 1, nil, nil, 1},
			[]any{"alev", "levels", "AbilityData", -1, 0, 0, "stats", "WESTRING_ALEV", "int", 1, 1, 1, nil, nil, 0},
			[]any{"acdn", "Cool", "AbilityData", -1, 4, 0, "stats", "WESTRING_ACDN", "unreal", 1, 1, 1, nil, nil, 0},
			[]any{"aare", "Area", "AbilityData", -1, 4, 0, "stats", "WESTRING_AARE", "unreal", 1, 1, 1, nil, "AHhb", 0},
			[]any{"Hhb2", "Data", "AbilityData", -1, 4, 2, "data", "WESTRING_HHB2", "unreal", 1, 1, 1, "AHhb", nil, 0},
			[]any{"Hhb1", "Data", "AbilityData", -1, 4, 1, "data", "WESTRING_HHB1", "unreal", 1, 1, 1, "AHhb", nil, ""},
			[]any{"Htb1", "Data", "AbilityData", -1, 4, 1, "data", "WESTRING_HTB1", "unreal", 1, 1, 1, "AHtb", nil, 0},
			[]any{"Hdc1", "Data", "AbilityData", -1, 4, 12, "data", "WESTRING_HDC1", "int", 1, 1, 1, "AHtb,AHhb", nil, 0},
			[]any{"atp1", "Tip", "Profile", 0, 3, 0, "text", "WESTRING_ATP1", "string", 1, 1, 0, nil, nil, 1},
		),
		buffFieldsTable: newSLK(buffMeta,
			[]any{"fnam", "EditorName", "text", "WESTRING_FNAM", "string", 1},
			[]any{"fart", "Buffart", "art", "WESTRING_FART", "icon", 1},
		),
		upgradeFieldsTable: newSLK(upgradeMeta,
			[]any{"gnam", "Name", 1, nil, "text", "WESTRING_GNAM", "string", 1},
			[]any{"gef1", "effect1", 0, "EffectID", "data", "WESTRING_GEF1", "upgradeEffect", 0},
			[]any{"gba1", "base1", 0, "Base", "data", "WESTRING_GBA1", "unreal", 0},
			[]any{"gmo1", "mod1", 0, "Mod", "data", "WESTRING_GMO1", "unreal", 0},
			[]any{"gpct", "pct", 0, nil, "data", "WESTRING_GPCT", "unreal", 0},
		),
		unitsTable: newSLK([]string{"unitID", "comment(s)"},
			[]any{"hfoo", "footman"}, []any{"Hpal", "paladin"}, []any{"hbar", "barracks"}, []any{"nzzz", "unnamed critter"},
		),
		balanceTable: newSLK(balanceMeta,
			[]any{"hfoo", 0, "_"}, []any{"Hpal", 0, "STR"}, []any{"hbar", 1, "_"}, []any{"nzzz", 0, "_"},
		),
		itemsTable: newSLK([]string{"itemID", "comment"}, []any{"ratf", "claws"}),
		abilitiesTable: newSLK([]string{"alias", "comments", "levels"},
			[]any{"AHhb", "holy light", 3}, []any{"AHtb", "storm bolt", 3}, []any{nil, "row without an id", 1},
		),
		buffsTable:    newSLK([]string{"alias", "comments"}, []any{"Binf", "inner fire"}, []any{"BHbd", "blizzard"}),
		upgradesTable: newSLK([]string{"upgradeid", "comments", "maxlevel"}, []any{"Rhme", "swords", 3}),
		labelsFile: strings.Join([]string{
			"[WorldEditStrings]",
			"WESTRING_UHPM=Hit Points Maximum (Base)",
			"WESTRING_UNAM=Name",
			"WESTRING_UMDL=WESTRING_MODELFILE",
			"WESTRING_MODELFILE=Model File",
			"WESTRING_IFIL=Model File",
			"WESTRING_USHR=Shadow on Water",
			"WESTRING_UABI=Abilities - Normal",
			"WESTRING_UDEA=Death Type",
			"WESTRING_UPRO=Proper Names (Hero's +1.)",
			"WESTRING_UCLS=Class",
			"WESTRING_UVER=Model File - Extra Versions",
			"WESTRING_ANAM=Name",
			"WESTRING_ALEV=Levels",
			"WESTRING_ACDN=Cooldown",
			"WESTRING_AARE=Area of Effect",
			"WESTRING_HHB2=Area of Effect",
			"WESTRING_HHB1=Amount Healed/Damaged",
			"WESTRING_HTB1=Cooldown",
			"WESTRING_HDC1=Damage Dealt (%)",
			"WESTRING_ATP1=Tooltip - Normal",
			"WESTRING_FNAM=Name",
			"WESTRING_FART=Icon",
			"WESTRING_GNAM=Name",
			"WESTRING_GEF1=Effect 1",
			"WESTRING_GBA1=Effect 1 - %s",
			"WESTRING_GMO1=Effect 1 - %s",
			"WESTRING_GPCT=% Bonus & More",
			"",
		}, "\r\n"),
		humanUnitStrings: strings.Join([]string{
			"[hfoo]\t",
			"Name=Footman",
			"[Hpal]",
			`Name="|cffffcc00Paladin|r"`,
			"[hbar]",
			"Name=Barracks",
		}, "\r\n"),
		humanAbilityStrings: strings.Join([]string{
			"[AHhb]",
			"Name=Holy Light",
			"[AHtb]",
			"Name=Storm Bolt",
			"[Binf]",
			"Bufftip=Inner Fire",
			"[BHbd]",
			"EditorName=Blizzard (Caster)",
			"Bufftip=Blizzard",
		}, "\n"),
		humanUpgradeStrings: "[Rhme]\nName=Iron Forged Swords,Steel Forged Swords\n",
		itemStrings:         "[ratf]\nName=Claws of Attack +15\n",
	}
}

func writeExport(t testing.TB, dir string, change func(files map[string]string)) string {
	t.Helper()
	files := miniExport()
	if change != nil {
		change(files)
	}
	if err := os.MkdirAll(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	for name, text := range files {
		testkit.WriteFile(t, dir, name, []byte(text))
	}
	return dir
}

func newExportDir(t testing.TB, change func(files map[string]string)) string {
	t.Helper()
	return writeExport(t, t.TempDir(), change)
}

func withRow(table string, records ...string) string {
	return strings.Replace(table, "\r\nE\r\n", "\r\n"+strings.Join(records, "\r\n")+"\r\nE\r\n", 1)
}

const unitClassPins = `{"names": {"units": {"ucls": "unitClass"}}, "removed": {}}`

func checkContains(t testing.TB, text string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if !strings.Contains(text, part) {
			t.Errorf("%q is missing from:\n%s", part, text)
		}
	}
}

func describeDifference(want, got string) string {
	index := firstDifferenceOffset(want, got)
	return fmt.Sprintf("the two part at offset %d, where the line wanted is %q and the line got is %q",
		index, lineAt(want, index), lineAt(got, index))
}

func firstLine(text string) string { return lineAt(text, 0) }

func firstDifferenceOffset(a, b string) int {
	index := 0
	for index < len(a) && index < len(b) && a[index] == b[index] {
		index++
	}
	return index
}

func lineAt(text string, offset int) string {
	offset = min(offset, len(text))
	start := strings.LastIndexByte(text[:offset], '\n') + 1
	end := len(text)
	if length := strings.IndexByte(text[offset:], '\n'); length >= 0 {
		end = offset + length
	}
	return text[max(start, offset-60):min(end, offset+60)]
}
