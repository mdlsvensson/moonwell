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

	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

// moduleFile is the go.mod of a scratch checkout: the line the generator knows a checkout of Moonwell by.
const moduleFile = "module github.com/mdlsvensson/moonwell\n"

// outputFolders is the folders of a checkout that the generator writes into, each by its path from the checkout.
var outputFolders = []string{"data", "schema/generated"}

// checkout is a scratch checkout of Moonwell under the test's temporary folder: a folder with a go.mod that
// names this module, and what the test puts there. The generator writes data/ and schema/generated/ of the
// checkout it finds, so a test runs it in one of these and never in the real checkout, which is only read.
type checkout struct {
	t    testing.TB
	root string // the folder, as a full path
}

// newCheckout makes a scratch checkout that holds its go.mod and nothing else.
func newCheckout(t testing.TB) checkout {
	t.Helper()
	c := checkout{t, t.TempDir()}
	c.write("go.mod", moduleFile)
	return c
}

// path is the full path of what the checkout has at name, a path from the checkout with "/".
func (c checkout) path(name string) string { return filepath.Join(c.root, filepath.FromSlash(name)) }

// write writes a file of the checkout, with the folders it is in. name is its path from the checkout, with "/".
func (c checkout) write(name, text string) {
	c.t.Helper()
	testkit.WriteFile(c.t, c.root, name, []byte(text))
}

// folder makes a folder of the checkout, with the folders it is in, and returns its full path.
func (c checkout) folder(name string) string {
	c.t.Helper()
	folder := c.path(name)
	if err := os.MkdirAll(folder, 0o777); err != nil {
		c.t.Fatal(err)
	}
	return folder
}

// carry copies into the scratch checkout what the real one has at each name, a path from the checkout with "/":
// a file, or every file below a folder. Each lands at the path it has in the real checkout.
func (c checkout) carry(names ...string) {
	c.t.Helper()
	for _, name := range names {
		for _, file := range realFiles(c.t, name) {
			c.write(file, string(realFile(c.t, file)))
		}
	}
}

// realFile reads a file of the real checkout, the one these tests are part of, by its path from there with "/".
func realFile(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(testkit.RepoRoot(t), filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// realFiles is the files the real checkout has at name: name itself for a file, and every file below it for a
// folder, each as a path from the checkout with "/".
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

// run runs one command line of the generator in the checkout. It returns what the run printed, what lies below
// the folders the generator writes afterwards (outputs), and the error the run ended with.
func (c checkout) run(args ...string) (printed string, files map[string][]byte, err error) {
	c.t.Helper()
	printed, err = c.runBelow("", args...)
	return printed, c.outputs(), err
}

// runBelow runs one command line of the generator in a folder of the checkout, which it makes: below is its path
// from the checkout with "/", and "" is the checkout itself. It returns what the run printed and the error the run
// ended with. It is the one place of the tests that calls run. It asks first whether the folder is of the real
// checkout, and makes it after: so it calls run for no folder of the real checkout, and makes none there.
func (c checkout) runBelow(below string, args ...string) (printed string, err error) {
	c.t.Helper()
	notInTheRealCheckout(c.t, c.path(below))
	dir := c.folder(below)
	var out bytes.Buffer
	err = run(dir, args, &out)
	return out.String(), err
}

// all is everything the checkout holds, by its path from the checkout with "/": a file with its bytes, and a
// folder as nil. A file that a run writes outside the folders of the generator shows here.
func (c checkout) all() map[string][]byte {
	c.t.Helper()
	return testkit.Snapshot(c.t, c.root)
}

// outputs is what the checkout has at and below data/ and schema/generated/, by its path from the checkout with
// "/": a file with its bytes, and a folder as nil. A folder of the two that is not there has no entry.
func (c checkout) outputs() map[string][]byte {
	c.t.Helper()
	found := map[string][]byte{}
	for _, folder := range outputFolders {
		if !fsx.IsDir(c.path(folder)) {
			continue
		}
		found[folder] = nil
		for name, data := range testkit.Snapshot(c.t, c.path(folder)) {
			found[folder+"/"+name] = data
		}
	}
	return found
}

// generatorPackage is the generator's package, as go build names it from the root of the module.
const generatorPackage = "./next/tools/gen"

// builtProgram builds a program of this module with the go that runs the tests, into a folder of the test, and
// returns the file. pkg names the package from the root of the module, which is the folder the build runs in.
// The build writes that file and nothing else; it takes a second or two.
func builtProgram(t testing.TB, pkg string) string {
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

// startIn starts a built program with dir as its working folder, waits for its end, and returns its exit code
// and what it wrote to each stream. dir is a folder of a scratch checkout, and never one of the real checkout: a
// generator writes into the checkout it finds.
func startIn(t testing.TB, program, dir string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	notInTheRealCheckout(t, dir)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	started := exec.CommandContext(ctx, program, args...)
	started.Dir = dir
	var printed, said bytes.Buffer
	started.Stdout, started.Stderr = &printed, &said
	if err := started.Run(); err != nil {
		var exited *exec.ExitError
		if !errors.As(err, &exited) {
			t.Fatal(err)
		}
		code = exited.ExitCode()
	}
	return code, printed.String(), said.String()
}

// notInTheRealCheckout stops the test when dir is the real checkout, the one these tests are part of, or a
// folder below it. A generator writes into the checkout it finds, and the real one is only read: so no program
// is started there, and run is not called for it. A folder that is no full path is one from the folder of the
// test, which is in the real checkout.
func notInTheRealCheckout(t testing.TB, dir string) {
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

// listener is a test that keeps what is reported to it, where a real test would fail: the reports of what must
// fail are read through it. Everything else is the real test's.
type listener struct {
	testing.TB
	reports []string
}

// stopped is what a listener raises where a real test would stop.
type stopped struct{}

func (l *listener) Helper()                   {}
func (l *listener) Logf(string, ...any)       {}
func (l *listener) Error(args ...any)         { l.reports = append(l.reports, fmt.Sprint(args...)) }
func (l *listener) Errorf(f string, a ...any) { l.reports = append(l.reports, fmt.Sprintf(f, a...)) }
func (l *listener) Fatal(args ...any)         { l.Error(args...); panic(stopped{}) }
func (l *listener) Fatalf(f string, a ...any) { l.Errorf(f, a...); panic(stopped{}) }

// listenTo runs what would fail a test with a listener for its test, up to where a real test would stop, and
// returns what was reported, a report on a line.
func listenTo(t testing.TB, reporting func(tb testing.TB)) string {
	t.Helper()
	heard := &listener{TB: t}
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

// texts is the files among the outputs of a checkout, each with its text. The folders are left out.
func texts(outputs map[string][]byte) map[string]string {
	files := map[string]string{}
	for name, data := range outputs {
		if data != nil {
			files[name] = string(data)
		}
	}
	return files
}

// exported writes a file as an export of the game's files gives one, outside every checkout, and returns its
// full path.
func exported(t testing.TB, name, text string) string {
	t.Helper()
	return testkit.WriteFile(t, t.TempDir(), name, []byte(text))
}

// ---- a miniature export of the game's object data ----

// sylk is the text of a table as the game's .slk files have one: a row with the names of the columns, and a row
// for each record. A cell is a text, a whole number, or nil for a cell that the row does not have.
func sylk(columns []string, rows ...[]any) string {
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

// The columns of the miniature's tables of fields and of its balance table.
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

// The miniature's files of strings, each by its path from the folder of the export.
const (
	humanUnitStrings    = stringsFolder + "/humanunitstrings.txt"
	humanAbilityStrings = stringsFolder + "/humanabilitystrings.txt"
	humanUpgradeStrings = stringsFolder + "/humanupgradestrings.txt"
	itemStrings         = stringsFolder + "/itemstrings.txt"
)

// miniExport is a hand-written miniature of an export of the game's object data: every file that the mode
// metadata reads, by its path from the folder of the export, in the shape the game's files have. The ids and
// the labels are the game's where a test is about them (Holy Light, Footman); the values are made up.
func miniExport() map[string]string {
	return map[string]string{
		unitFieldsTable: sylk(unitMeta,
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
		abilityFieldsTable: sylk(abilityMeta,
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
		buffFieldsTable: sylk(buffMeta,
			[]any{"fnam", "EditorName", "text", "WESTRING_FNAM", "string", 1},
			[]any{"fart", "Buffart", "art", "WESTRING_FART", "icon", 1},
		),
		upgradeFieldsTable: sylk(upgradeMeta,
			[]any{"gnam", "Name", 1, nil, "text", "WESTRING_GNAM", "string", 1},
			[]any{"gef1", "effect1", 0, "EffectID", "data", "WESTRING_GEF1", "upgradeEffect", 0},
			[]any{"gba1", "base1", 0, "Base", "data", "WESTRING_GBA1", "unreal", 0},
			[]any{"gmo1", "mod1", 0, "Mod", "data", "WESTRING_GMO1", "unreal", 0},
			[]any{"gpct", "pct", 0, nil, "data", "WESTRING_GPCT", "unreal", 0},
		),
		unitsTable: sylk([]string{"unitID", "comment(s)"},
			[]any{"hfoo", "footman"}, []any{"Hpal", "paladin"}, []any{"hbar", "barracks"}, []any{"nzzz", "unnamed critter"},
		),
		balanceTable: sylk(balanceMeta,
			[]any{"hfoo", 0, "_"}, []any{"Hpal", 0, "STR"}, []any{"hbar", 1, "_"}, []any{"nzzz", 0, "_"},
		),
		itemsTable: sylk([]string{"itemID", "comment"}, []any{"ratf", "claws"}),
		abilitiesTable: sylk([]string{"alias", "comments", "levels"},
			[]any{"AHhb", "holy light", 3}, []any{"AHtb", "storm bolt", 3}, []any{nil, "row without an id", 1},
		),
		buffsTable:    sylk([]string{"alias", "comments"}, []any{"Binf", "inner fire"}, []any{"BHbd", "blizzard"}),
		upgradesTable: sylk([]string{"upgradeid", "comments", "maxlevel"}, []any{"Rhme", "swords", 3}),
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

// writeExport writes the miniature export into dir, after change has adjusted its files, and returns dir: the
// folder of the export. change may be nil.
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

// exportedGame writes the miniature export, after change has adjusted its files, into a folder of the test that
// is outside every checkout, and returns the folder.
func exportedGame(t testing.TB, change func(files map[string]string)) string {
	t.Helper()
	return writeExport(t, t.TempDir(), change)
}

// withRow gives a table of the miniature one more row: the lines are those of the row's C records, the first
// with the Y of a row that the table has not.
func withRow(table string, records ...string) string {
	return strings.Replace(table, "\r\nE\r\n", "\r\n"+strings.Join(records, "\r\n")+"\r\nE\r\n", 1)
}

// unitClassPins is the text of an overrides file with the one pin that the miniature needs: its label Class
// gives a name that is a keyword of Pkl.
const unitClassPins = `{"names": {"units": {"ucls": "unitClass"}}, "removed": {}}`

// contains fails the test for each part that the text lacks.
func contains(t testing.TB, text string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if !strings.Contains(text, part) {
			t.Errorf("%q is missing from:\n%s", part, text)
		}
	}
}

// parting says, for a report, where two texts part: the offset, and the line of each there. No report shows
// more of what a tree printed or wrote than such a line.
func parting(want, got string) string {
	at := partingOffset(want, got)
	return fmt.Sprintf("the two part at offset %d, where the line wanted is %q and the line got is %q",
		at, lineAt(want, at), lineAt(got, at))
}

// firstLine is the first line of a text, for a report, cut as lineAt cuts a line.
func firstLine(text string) string { return lineAt(text, 0) }

// partingOffset is the offset of the first byte in which two texts differ: the length of the shorter when it is
// the start of the other.
func partingOffset(a, b string) int {
	at := 0
	for at < len(a) && at < len(b) && a[at] == b[at] {
		at++
	}
	return at
}

// lineAt is the line of a text that holds the byte at an offset, for a report: without its line break, and of a
// long line the sixty bytes before the offset and the sixty from it.
func lineAt(text string, offset int) string {
	offset = min(offset, len(text))
	start := strings.LastIndexByte(text[:offset], '\n') + 1
	end := len(text)
	if length := strings.IndexByte(text[offset:], '\n'); length >= 0 {
		end = offset + length
	}
	return text[max(start, offset-60):min(end, offset+60)]
}
