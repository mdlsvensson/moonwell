package assets

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/mapdir"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/war3/imp"
)

// The stories: a project on disk, and runs of assets:sync or of a build in it, one after the other, each in the
// project as the runs before it left it. The recorded test holds what every run plans and what it leaves behind.

// storyManifest is the manifest that errors about the assets block of a story's project name.
const storyManifest = "moonwell.pkl"

// mapFolder is the source map of every project here that has one.
const mapFolder = "map.w3x"

// shippedUnder is where the files that a library ships are, from the project folder.
const shippedUnder = ".moonwell/library-assets/"

// sourceLabel is how the source map of a story is named in errors: its path from the project folder.
const sourceLabel = "maps/" + mapFolder

// stageFolder is where a build of the stories stages the map, from the project folder.
const stageFolder = "dist/stage/" + mapFolder

// project is a project on disk.
type project struct {
	name      string
	files     map[string]string // by path from the project folder, with "/": what each file holds
	folders   []string          // folders to make besides those the files are in
	block     string            // the manifest's assets block as pkl prints it; "" for a block that sets nothing
	libraries []string          // the keys of the libraries, whose files are under shippedUnder
	refused   bool              // whether the project is refused
}

// holding is files that each hold their own name, so that no two hold the same bytes.
func holding(names ...string) map[string]string {
	files := map[string]string{}
	for _, name := range names {
		files[name] = bytesOf(name)
	}
	return files
}

// bytesOf is what holding puts into the file of a name.
func bytesOf(name string) string { return "the bytes of " + name }

// onDisk writes the project into a folder of its own.
func (p project) onDisk(t testing.TB) string {
	t.Helper()
	root := t.TempDir()
	for _, folder := range p.folders {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(folder)), 0o777); err != nil {
			t.Fatal(err)
		}
	}
	for name, content := range p.files {
		testkit.WriteFile(t, root, name, []byte(content))
	}
	return root
}

// assets is the project's assets block.
func (p project) assets(t testing.TB) manifest.Assets {
	t.Helper()
	return blockOf(t, cmp.Or(p.block, noBlock))
}

// shippedIn is the project's libraries, each with the folder its files are in.
func (p project) shippedIn(root string) []Library {
	var libraries []Library
	for _, key := range p.libraries {
		dir := filepath.Join(root, filepath.FromSlash(shippedUnder), key)
		libraries = append(libraries, Library{Key: key, Dir: dir})
	}
	return libraries
}

// The kinds of refusal a run can have. A refusal of the first kind names no file of the project, or the manifest.
// Each of the others is about one file, which it names from the project folder, and has a note in the recording.
const (
	asWhole = "whole"
	// byName is a refusal about a file of the map, named by the map's label, or about the state file.
	byName = "by name"
	// reworded is the refusal of a write to a place that changed after the assets were checked.
	reworded = "reworded"
	// bySystem is the refusal for a file of the map that cannot be read, or that is a folder.
	bySystem = "by system"
)

// notes is the note the recording has beside a run with a refusal of each kind: the row of the table of what a
// user can notice, in the design of this program (its §8), that the refusal is an instance of.
var notes = map[string]string{
	byName: `§8, the rule of its rows on a file that cannot be used: it "is named from the project folder"`,
	reworded: `§8, "assets:sync: a file or folder that appears where a new asset goes, between the check and ` +
		`the write, stops the sync with "changed after the assets were checked""`,
	bySystem: `§8, "A folder where a map file belongs, and a file that cannot be read [...], is a named error ` +
		`with the file, not an internal error"`,
}

// run is one run of assets:sync, or of a build, in a project as the runs before it left it.
type run struct {
	edit      func(t testing.TB, root string) // what changes in the project before the run
	block     string
	libraries []string
	build     bool                            // the changes go into a staged copy, and no state is written
	meddle    func(t testing.TB, root string) // what another program does between the plan and its writes
	// The context of the plan and of the writes, for a run that makes its own: else one that is never cancelled.
	planCtx, syncCtx func(t testing.TB, root string) *countdown
	refused          string // the kind of its refusal; "" for a run that is taken
	about            string // the file a refusal names, from the project folder with "/"
}

// story is a project and the runs in it, one after the other.
type story struct {
	name    string
	project project
	runs    []run
}

// planned is a plan in one shape, whoever made it.
type planned struct {
	Assets  []Asset
	Changes []plannedChange
	Owned   []Owned
}

// plannedChange is one change of a plan, its file by the path from the folder the plan is for, with "/".
type plannedChange struct {
	Name   string
	Bytes  []byte
	Remove bool
}

// plannedBy is a plan of Plan.
func plannedBy(result *Result) *planned {
	p := &planned{Assets: result.Assets, Changes: []plannedChange{}, Owned: append([]Owned{}, result.State.Files...)}
	for _, change := range result.Changes {
		p.Changes = append(p.Changes, plannedChange{change.Name, change.Bytes, change.Remove})
	}
	return p
}

// ran is what a run did.
type ran struct {
	plan *planned // nil for a plan that was refused
	err  error
	asks [2]int // how often the plan and the writes asked their contexts
}

// contextOf is the context of one step of a run in the project at root.
func contextOf(t testing.TB, root string, own func(testing.TB, string) *countdown) *countdown {
	if own != nil {
		return own(t, root)
	}
	return &countdown{Context: context.Background(), limit: never}
}

// in makes the run in the project at root, as a command does it: it collects, reads the state, opens the map,
// plans, and then syncs, or stages for a build.
func (r run) in(t testing.TB, root string) (did ran) {
	t.Helper()
	p := project{block: r.block, libraries: r.libraries}
	stateFile, err := StateFile(root, mapFolder)
	if err != nil {
		t.Fatal(err)
	}
	assets, _, err := Collect(root, p.assets(t), storyManifest, p.shippedIn(root))
	if err != nil {
		return ran{err: err}
	}
	owned, err := ReadState(stateFile)
	if err != nil {
		return ran{err: err}
	}
	folder, err := mapdir.Open(filepath.Join(root, filepath.FromSlash(sourceLabel)), sourceLabel)
	if err != nil {
		return ran{err: err}
	}
	planCtx, syncCtx := contextOf(t, root, r.planCtx), contextOf(t, root, r.syncCtx)
	result, err := Plan(planCtx, folder, assets, owned)
	did.asks[0] = planCtx.asks
	if err != nil {
		did.err = err
		return did
	}
	did.plan = plannedBy(result)
	if r.meddle != nil {
		r.meddle(t, root)
	}
	if r.build {
		did.err = folder.With(result.Changes).StageTo(filepath.Join(root, filepath.FromSlash(stageFolder)))
		return did
	}
	did.err = Sync(syncCtx, folder, result, stateFile)
	did.asks[1] = syncCtx.asks
	return did
}

// ---- what a story is made of ----

// with adds files that hold a text of their own to files: a name, then the text, and so on.
func with(files map[string]string, pairs ...string) map[string]string {
	for i := 0; i < len(pairs); i += 2 {
		files[pairs[i]] = pairs[i+1]
	}
	return files
}

// putting is an edit that writes files of the project: a name from the project folder with "/", then the text
// the file holds, and so on.
func putting(pairs ...string) func(testing.TB, string) {
	return func(t testing.TB, root string) {
		t.Helper()
		for i := 0; i < len(pairs); i += 2 {
			testkit.WriteFile(t, root, pairs[i], []byte(pairs[i+1]))
		}
	}
}

// removing is an edit that removes files of the project, and folders with all that is in them.
func removing(names ...string) func(testing.TB, string) {
	return func(t testing.TB, root string) {
		t.Helper()
		for _, name := range names {
			if err := os.RemoveAll(filepath.Join(root, filepath.FromSlash(name))); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// renaming is an edit that renames a file or a folder of the project, by way of a third name: a system that
// ignores letter case takes two spellings of a name for one.
func renaming(from, to string) func(testing.TB, string) {
	return func(t testing.TB, root string) {
		t.Helper()
		at := func(name string) string { return filepath.Join(root, filepath.FromSlash(name)) }
		aside := at(from + ".aside")
		if err := errors.Join(os.Rename(at(from), aside), os.Rename(aside, at(to))); err != nil {
			t.Fatal(err)
		}
	}
}

// several is edits made one after the other.
func several(edits ...func(testing.TB, string)) func(testing.TB, string) {
	return func(t testing.TB, root string) {
		t.Helper()
		for _, edit := range edits {
			edit(t, root)
		}
	}
}

// indexOf is what a war3map.imp with the entries holds.
func indexOf(entries ...imp.Entry) string { return string(imp.Write(entries)) }

// stopped is a context that is cancelled from the ask after limit on, and before that ask has another program
// do something in the project.
func stopped(limit int, meddle func(testing.TB, string)) func(testing.TB, string) *countdown {
	return func(t testing.TB, root string) *countdown {
		before := map[int]func(){limit + 1: func() { meddle(t, root) }}
		return &countdown{Context: context.Background(), limit: limit, before: before}
	}
}

// beforeAsk is a context that is never cancelled, and before an ask, counted from 1, has another program do
// something in the project.
func beforeAsk(ask int, meddle func(testing.TB, string)) func(testing.TB, string) *countdown {
	return func(t testing.TB, root string) *countdown {
		before := map[int]func(){ask: func() { meddle(t, root) }}
		return &countdown{Context: context.Background(), limit: never, before: before}
	}
}

// ---- the stories ----

// scenarioStories is the stories of a project's life: assets mapped, changed, staged, renamed and removed, what
// World Editor does to the index between two runs, the libraries, and each thing in the map that is in the way.
func scenarioStories() []story {
	const m, s = sourceLabel + "/", shippedUnder
	const sword = "ReplaceableTextures/CommandButtons/BTNSword.blp"
	const mapped = `{"paths":{"icons/disabled.blp":"ReplaceableTextures/CommandButtonsDisabled/DISBTNSword.blp"}}`
	const spellings = `{"paths":{"a.blp":"Sound/Music/a.blp","b.blp":"sound/music/b.blp",` +
		`"c.blp":"SOUND/Effects/c.blp"}}`
	return []story{
		{"mapped assets beside the editor's own import, and nothing to do the second time",
			project{files: with(holding("assets/"+sword, "assets/icons/disabled.blp", m+"war3mapImported/existing.wav"),
				m+"war3map.imp", indexOf(imp.Entry{Flag: 5, Path: "existing.wav"}))},
			[]run{{block: mapped}, {block: mapped}}},
		{"an asset is changed, staged by a build, synced, renamed by a mapping and removed",
			project{files: holding("assets/Models/unit.mdx", m+"unmanaged.txt")},
			[]run{{}, {edit: putting("assets/Models/unit.mdx", "second"), build: true}, {},
				{block: `{"paths":{"Models/unit.mdx":"Models/renamed.mdx"}}`},
				{edit: removing("assets/Models/unit.mdx")}}},
		{"World Editor saves the owned imports with its own flag, and a third asset is added",
			project{files: holding("assets/Textures/a.blp", "assets/Textures/b.blp")},
			[]run{{}, {edit: putting(m+"war3map.imp",
				indexOf(imp.Entry{Flag: 29, Path: `Textures\a.blp`}, imp.Entry{Flag: 29, Path: `Textures\b.blp`}))},
				{edit: putting("assets/Textures/c.blp", "a third")}}},
		{"a file of the map where an asset goes, then an owned file edited by hand, with and without its asset",
			project{files: with(holding("assets/a.blp"), m+"a.blp", "editor owned")},
			[]run{{refused: byName, about: m + "a.blp"}, {edit: removing(m + "a.blp")},
				{edit: putting(m+"a.blp", "manual edit"), refused: byName, about: m + "a.blp"},
				{edit: removing("assets/a.blp"), refused: byName, about: m + "a.blp"},
				{build: true, refused: byName, about: m + "a.blp"}}},
		{"an asset below a file of the map",
			project{files: holding("assets/a.blp", m+"Textures")},
			[]run{{block: `{"paths":{"a.blp":"textures/a.blp"}}`, refused: byName, about: m + "Textures"}}},
		{"a folder the map spells in its own way, and new folders in three spellings",
			project{files: holding(m+"Textures/existing.blp", "assets/textures/new.blp", "assets/a.blp",
				"assets/b.blp", "assets/c.blp")},
			[]run{{block: spellings}, {block: spellings}}},
		{"nothing to own, then one file, then nothing again",
			project{}, []run{{}, {edit: putting("assets/a.blp", "one")}, {edit: removing("assets/a.blp")}}},
		{"a folder appears where a new file goes, after the plan",
			project{files: holding("assets/a.blp", "assets/b.blp")},
			[]run{{meddle: putting(m+"b.blp/inner.txt", "another program's"), refused: reworded, about: m + "b.blp"}}},
		{"a write fails after a file was replaced: a file is where the folder of a new file goes",
			project{files: with(holding("assets/a.blp"), m+"war3mapImported/existing.wav", "editor",
				m+"war3map.imp", indexOf(imp.Entry{Flag: 5, Path: "existing.wav"}))},
			[]run{{}, {edit: putting("assets/a.blp", "second", "assets/Sound/b.blp", "new"),
				meddle: putting(m+"Sound", "another program's"), refused: byName, about: m + "Sound/b.blp"}}},
		{"a file that cannot be taken out again when the sync is interrupted",
			project{files: holding("assets/a.blp", "assets/b.blp")},
			[]run{{syncCtx: stopped(1, several(removing(m+"a.blp"), putting(m+"a.blp/inner.txt", "another program's"))),
				refused: byName, about: m + "a.blp"}}},
		{"nothing to import and nothing owned, in a map whose index does not read",
			project{files: map[string]string{m + "war3map.imp": "\x09\x09"}}, []run{{}, {build: true}}},
		{"a library's files beside the map's own, and the library dropped",
			project{files: holding("assets/Models/Own.mdx", "assets/icons/shared.blp",
				s+"ui/war3mapImported/ui/frames.toc", s+"ui/icons/shared.blp", s+"unlisted/never.txt")},
			[]run{{libraries: []string{"ui"}}, {libraries: []string{"ui"}}, {}}},
		{"a library's file where the map has a file of its own",
			project{files: holding(s+"ui/Models/Golem.mdx", m+"Models/Golem.mdx")},
			[]run{{libraries: []string{"ui"}, refused: byName, about: m + "Models/Golem.mdx"}}},
	}
}

// seededStories is stories for what the scenarios do not reach.
func seededStories(t testing.TB) []story {
	const m, s = sourceLabel + "/", shippedUnder
	const state = ".asset-state/" + mapFolder + ".json"
	e := func(flag uint8, path string) imp.Entry { return imp.Entry{Flag: flag, Path: path} }
	one := func(name string, files map[string]string, r run) story {
		return story{name, project{files: files}, []run{r}}
	}
	saved := string(testkit.Fixture(t, "imports-we3/war3map-flag29.imp"))
	return []story{
		one("an import World Editor made where an asset goes, without a file",
			with(holding("assets/Textures/a.blp"), m+"war3map.imp", indexOf(e(29, `textures\A.blp`))),
			run{refused: byName, about: m + "Textures/a.blp"}),
		one("an import in the folder World Editor imports into, where an asset goes",
			with(holding("assets/war3mapImported/a.wav"), m+"war3map.imp", indexOf(e(8, "A.wav"))),
			run{refused: byName, about: m + "war3mapImported/a.wav"}),
		one("an index that lists a path twice",
			with(holding("assets/a.blp"), m+"war3map.imp",
				indexOf(e(13, `Textures\x.blp`), e(5, "y.wav"), e(29, "textures/X.BLP"))),
			run{refused: byName, about: m + "war3map.imp"}),
		one("an index that is cut short",
			with(holding("assets/a.blp"), m+"war3map.imp", indexOf(e(13, "x.blp"))[:10]),
			run{refused: byName, about: m + "war3map.imp"}),
		one("an index with a flag World Editor does not write",
			with(holding("assets/a.blp"), m+"war3map.imp", indexOf(e(7, "x.blp"))),
			run{refused: byName, about: m + "war3map.imp"}),
		one("an index with a path that leaves the map",
			with(holding("assets/a.blp"), m+"War3Map.imp", indexOf(e(13, `..\x.blp`))),
			run{refused: byName, about: m + "War3Map.imp"}),
		one("an index named in capitals, with the editor's own import",
			with(holding("assets/a.blp", m+"war3mapImported/own.wav"), m+"WAR3MAP.IMP", indexOf(e(8, "own.wav"))),
			run{}),
		one("the index World Editor 3.00 saved",
			with(holding("assets/a.blp", m+"wa3mapPreview.tga"), m+"war3map.imp", saved), run{}),
		{"an owned file gone from the map, an asset changed, one added and one left out",
			project{files: holding("assets/a.blp", "assets/b.blp", "assets/c.blp", "assets/d.blp")},
			[]run{{}, {edit: several(removing(m+"a.blp"),
				putting("assets/b.blp", "second", "assets/Sound/e.blp", "new")), block: `{"exclude":["c.blp"]}`}}},
		one("an asset named as a folder of the map", holding("assets/textures", m+"Textures/x.blp"),
			run{refused: byName, about: m + "Textures"}),
		{"an asset named as an empty folder of the map",
			project{files: holding("assets/empty"), folders: []string{m + "Empty"}},
			[]run{{refused: byName, about: m + "Empty"}}},
		{"an asset below an owned file that no asset wants",
			project{files: holding("assets/data")},
			[]run{{}, {edit: several(removing("assets/data"), putting("assets/data/inner.txt", "inner")),
				refused: byName, about: m + "data"}}},
		one("two assets without room, the first below a file",
			holding("assets/a/inner.blp", "assets/b.blp", m+"a", m+"b.blp"), run{refused: byName, about: m + "a"}),
		one("two assets without room, the first at a file",
			holding("assets/a/inner.blp", "assets/b.blp", m+"a", m+"0.blp"),
			run{block: `{"paths":{"b.blp":"0.blp"}}`, refused: byName, about: m + "0.blp"}),
		{"an owned import that World Editor saved without a custom path",
			project{files: holding("assets/war3mapImported/a.wav")},
			[]run{{}, {edit: putting(m+"war3map.imp", indexOf(e(5, "a.wav")))}, {}, {}}},
		{"the last owned file is removed from a map without an index",
			project{files: holding("assets/a.blp")}, []run{{}, {edit: removing("assets/a.blp", m+"war3map.imp")}}},
		{"an owned file whose import is gone from the index",
			project{files: holding("assets/a.blp")}, []run{{}, {edit: putting(m+"war3map.imp", indexOf())}}},
		{"an owned file under another spelling in the map",
			project{files: holding("assets/Models/Unit.mdx")},
			[]run{{}, {edit: several(renaming(m+"Models/Unit.mdx", m+"Models/UNIT.mdx"),
				renaming(m+"Models", m+"models"), putting("assets/Models/Unit.mdx", "second"))}}},
		one("names outside ASCII and with a space, and a file without bytes",
			with(holding("assets/caf\xc3\xa9/\xc3\x89cole.blp", "assets/My Icons/a b.blp"), "assets/empty.blp", ""),
			run{}),
		one("a path that looks like a number, alone", holding("assets/7"), run{}),
		{"a build with an asset changed, one removed and one in a new folder",
			project{files: holding("assets/Models/unit.mdx", "assets/old.blp", m+"war3map.w3i")},
			[]run{{}, {edit: several(putting("assets/Models/unit.mdx", "second", "assets/sound/theme.mp3", "theme"),
				removing("assets/old.blp")), build: true}}},
		one("a library's file below a file of the map", holding(s+"ui/ui/frame.fdf", m+"UI"),
			run{libraries: []string{"ui"}, refused: byName, about: m + "UI"}),
		one("a folder named as the index", holding("assets/a.blp", m+"war3map.imp/stray.txt"),
			run{refused: bySystem, about: m + "war3map.imp"}),
		{"an owned file is edited by hand after the plan",
			project{files: holding("assets/0.blp", "assets/a.blp")},
			[]run{{}, {edit: putting("assets/0.blp", "second", "assets/a.blp", "second"),
				meddle: putting(m+"a.blp", "edited by hand"), refused: byName, about: m + "a.blp"}}},
		one("a file appears where a new asset goes, after the plan", holding("assets/0.blp", "assets/a.blp"),
			run{meddle: putting(m+"a.blp", "the editor's"), refused: byName, about: m + "a.blp"}),
		one("the state file cannot be written", holding("assets/a.blp"),
			run{syncCtx: beforeAsk(3, putting(state+"/in the way.txt", "another program's")),
				refused: byName, about: state}),
		// Another program gets at the state file after the sync began. The ask before the state file is the last:
		// the second where a.blp alone is written, the third where the index is written too.
		{"the state file is changed before it is written",
			project{files: holding("assets/a.blp")},
			[]run{{}, {edit: putting("assets/a.blp", "second"),
				syncCtx: beforeAsk(2, putting(state, "another program's")), refused: byName, about: state}}},
		{"the state file is removed before it is written",
			project{files: holding("assets/a.blp")},
			[]run{{}, {edit: putting("assets/a.blp", "second"), syncCtx: beforeAsk(2, removing(state)),
				refused: byName, about: state}}},
		one("a state file is made before the first is written", holding("assets/a.blp"),
			run{syncCtx: beforeAsk(3, putting(state, "another program's")), refused: byName, about: state}),
		{"the state file is changed before it is removed",
			project{files: holding("assets/a.blp")},
			[]run{{}, {edit: removing("assets/a.blp"), syncCtx: beforeAsk(3, putting(state, "another program's")),
				refused: byName, about: state}}},
		// A state file that needs no write is not looked at again.
		{"the state file is changed while a sync that does not write it writes the index",
			project{files: holding("assets/a.blp")},
			[]run{{}, {edit: putting(m+"war3map.imp", indexOf()),
				syncCtx: beforeAsk(1, putting(state, "another program's"))}}},
		// Nor is a state file looked for that a sync has neither to write nor to remove: the sync leaves nothing
		// owned, and the state file is gone when it begins.
		{"a state file is made while a sync that owns nothing, and found none, writes the map",
			project{files: holding("assets/a.blp")},
			[]run{{}, {edit: removing("assets/a.blp"), meddle: removing(state),
				syncCtx: beforeAsk(2, putting(state, "another program's"))}}},
	}
}

// interruptedProject is a project after a sync, with an asset changed, one added and one removed since: its plan
// asks its context before anything, before each of the two owned files and before each of the two assets, and its
// sync before each of four changes and before the state file.
func interruptedProject() project {
	const m = sourceLabel + "/"
	first, dropped := fsx.SHA256Hex([]byte("first")), fsx.SHA256Hex([]byte("dropped"))
	owned := State{Files: []Owned{{"a.blp", first}, {"dropped.blp", dropped}}}
	index := indexOf(imp.Entry{Flag: 29, Path: "a.blp"}, imp.Entry{Flag: 29, Path: "dropped.blp"})
	return project{files: map[string]string{
		"assets/a.blp": "second", "assets/b.blp": "new", m + "a.blp": "first", m + "dropped.blp": "dropped",
		m + "war3map.imp": index, ".asset-state/" + mapFolder + ".json": string(owned.Bytes()),
	}}
}

// interruptedAsks is how often the plan of interruptedProject asks its context, and how often its sync does.
const interruptedAsks = 5

// interruptedStories is the plan and the sync of interruptedProject with a context that is cancelled at each of
// their asks, one after the other, and at none: twelve projects, each with one run.
func interruptedStories() []story {
	limited := func(limit int) func(testing.TB, string) *countdown {
		return func(testing.TB, string) *countdown {
			return &countdown{Context: context.Background(), limit: limit}
		}
	}
	var stories []story
	for _, step := range []string{"the plan", "the sync"} {
		for limit := range interruptedAsks + 1 {
			r := run{planCtx: limited(limit), refused: asWhole}
			if step == "the sync" {
				r = run{syncCtx: limited(limit), refused: asWhole}
			}
			if limit == interruptedAsks {
				r.refused = ""
			}
			name := fmt.Sprintf("%s, cancelled at ask %d", step, limit+1)
			stories = append(stories, story{name, interruptedProject(), []run{r}})
		}
	}
	return stories
}

// ---- the recording ----

// longest is the most bytes of a file that a recording writes out; a longer file stands as its digest.
const longest = 64

// held is the bytes of a file as a recording holds them.
func held(data []byte) string {
	if len(data) > longest {
		return testkit.Digest(data)
	}
	return strconv.Quote(string(data))
}

// below is a file of a refusal as a recording names it: a path on disk below the project folder is written from
// "<root>", with "/".
func below(root, file string) string {
	if rel, err := filepath.Rel(root, file); err == nil && filepath.IsAbs(file) && filepath.IsLocal(rel) {
		return "<root>/" + filepath.ToSlash(rel)
	}
	return testkit.Shown(file)
}

// fileOf is the file that a refusal names.
func fileOf(err error) string {
	failure, _ := diag.First(err)
	return failure.File
}

// outcome is what a run came to: what it planned, the file its refusal names, and the project as it left it.
type outcome struct {
	plan      *planned // nil for a plan that was refused
	refused   bool
	refusedAt string            // as below writes it
	left      map[string][]byte // testkit.Snapshot of the project afterwards
}

// lines is the outcome as a recording holds it. The files that a run leaves are those of the project outside
// assets/ and the libraries' files, which no run writes; the imports of an index that reads follow it.
func (o outcome) lines() string {
	var out strings.Builder
	if o.refused {
		fmt.Fprintf(&out, "refused: %s\n", o.refusedAt)
	}
	if o.plan != nil {
		out.WriteString("planned:\n")
		for _, change := range o.plan.Changes {
			what := held(change.Bytes)
			if change.Remove {
				what = "removed"
			}
			fmt.Fprintf(&out, "  %s: %s\n", testkit.Shown(change.Name), what)
		}
		for _, owned := range o.plan.Owned {
			fmt.Fprintf(&out, "  owned: %s %s\n", testkit.Shown(owned.Path), owned.Hash)
		}
	}
	out.WriteString("left:\n")
	for _, name := range slices.Sorted(func(yield func(string) bool) {
		for name := range o.left {
			if !strings.HasPrefix(name, "assets") && !strings.HasPrefix(name, ".moonwell") && !yield(name) {
				return
			}
		}
	}) {
		data := o.left[name]
		if data == nil {
			fmt.Fprintf(&out, "  %s/\n", testkit.Shown(name))
			continue
		}
		fmt.Fprintf(&out, "  %s: %s\n", testkit.Shown(name), held(data))
		if entries, err := imp.Read(data, name); err == nil && mapdir.Key(filepath.Base(name)) == "war3map.imp" {
			for _, entry := range entries {
				fmt.Fprintf(&out, "    import %d %s\n", entry.Flag, testkit.Shown(entry.Path))
			}
		}
	}
	return out.String()
}

// recordedStories is the recording of the stories: the runs of each, one after the other in a project of its
// own. do makes a run in the project at root and gives what it came to.
func recordedStories(t testing.TB, do func(r run, root string) outcome) []byte {
	t.Helper()
	var out strings.Builder
	for _, s := range slices.Concat(scenarioStories(), seededStories(t), interruptedStories()) {
		s.project.folders = append(s.project.folders, sourceLabel)
		root := s.project.onDisk(t)
		for i, r := range s.runs {
			if r.edit != nil {
				r.edit(t, root)
			}
			fmt.Fprintf(&out, "== %s (run %d)\n", s.name, i+1)
			if note := notes[r.refused]; note != "" {
				fmt.Fprintf(&out, "note: %s\n", note)
			}
			out.WriteString(do(r, root).lines())
		}
	}
	return []byte(out.String())
}

// outcomeOf makes the run in the project at root.
func outcomeOf(t testing.TB, r run, root string) outcome {
	t.Helper()
	did := r.in(t, root)
	made := outcome{plan: did.plan, left: testkit.Snapshot(t, root)}
	if did.err != nil {
		made.refused, made.refusedAt = true, below(root, fileOf(did.err))
	}
	return made
}

// TestTheStoriesOfThePlanAndTheSyncAreAsRecorded holds every run of every story to a recording: what the run
// plans (each change, and what the map then owns), the file its refusal names, and what it leaves on disk, the
// map with its index of imports, the state file and the stage among it. A short file stands whole and a longer
// one as its digest. A run that plans, writes or puts back another byte is a line of a diff under its story.
func TestTheStoriesOfThePlanAndTheSyncAreAsRecorded(t *testing.T) {
	testkit.Recorded(t, "stories.txt", recordedStories(t, func(r run, root string) outcome {
		return outcomeOf(t, r, root)
	}))
}
