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

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/war3/imp"
)

const storyManifest = "moonwell.pkl"

const mapFolder = "map.w3x"

const shippedUnder = ".moonwell/library-assets/"

const sourceLabel = "maps/" + mapFolder

const stageFolder = "dist/stage/" + mapFolder

type project struct {
	files     map[string]string
	folders   []string
	block     string
	libraries []string
}

func holding(names ...string) map[string]string {
	files := map[string]string{}
	for _, name := range names {
		files[name] = bytesOf(name)
	}
	return files
}

func bytesOf(name string) string { return "the bytes of " + name }

func (p project) onDisk(t testing.TB) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range p.folders {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(dir)), 0o777); err != nil {
			t.Fatal(err)
		}
	}
	for name, content := range p.files {
		testkit.WriteFile(t, root, name, []byte(content))
	}
	return root
}

func (p project) assets(t testing.TB) manifest.Assets {
	t.Helper()
	return blockOf(t, cmp.Or(p.block, noBlock))
}

func (p project) shippedIn(root string) []Library {
	var libraries []Library
	for _, key := range p.libraries {
		dir := filepath.Join(root, filepath.FromSlash(shippedUnder), key)
		libraries = append(libraries, Library{Key: key, Dir: dir})
	}
	return libraries
}

type run struct {
	edit             func(t testing.TB, root string)
	block            string
	libraries        []string
	build            bool
	meddle           func(t testing.TB, root string)
	planCtx, syncCtx func(t testing.TB, root string) *countdown
}

type story struct {
	name    string
	project project
	runs    []run
}

type planned struct {
	Changes []plannedChange
	Owned   []Owned
}

type plannedChange struct {
	Name   string
	Bytes  []byte
	Remove bool
}

func plannedBy(result *Result) *planned {
	p := &planned{Changes: []plannedChange{}, Owned: append([]Owned{}, result.State.Files...)}
	for _, change := range result.Changes {
		p.Changes = append(p.Changes, plannedChange{change.Path, change.Data, change.Remove})
	}
	return p
}

type ran struct {
	plan *planned
	err  error
	asks [2]int
}

func contextOf(t testing.TB, root string, own func(testing.TB, string) *countdown) *countdown {
	if own != nil {
		return own(t, root)
	}
	return &countdown{Context: context.Background(), limit: never}
}

func (r run) in(t testing.TB, root string) (did ran) {
	t.Helper()
	p := project{block: r.block, libraries: r.libraries}
	stateFile, err := StateFilePath(root, mapFolder)
	if err != nil {
		t.Fatal(err)
	}
	assets, _, err := Collect(root, p.assets(t), storyManifest, p.shippedIn(root))
	if err != nil {
		return ran{err: err}
	}
	owned, err := ReadState(root, stateFile)
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
		did.err = folder.WithChanges(result.Changes).StageTo(filepath.Join(root, filepath.FromSlash(stageFolder)))
		return did
	}
	did.err = Sync(syncCtx, folder, result, root, stateFile)
	did.asks[1] = syncCtx.asks
	return did
}

func with(files map[string]string, pairs ...string) map[string]string {
	for i := 0; i < len(pairs); i += 2 {
		files[pairs[i]] = pairs[i+1]
	}
	return files
}

func putting(pairs ...string) func(testing.TB, string) {
	return func(t testing.TB, root string) {
		t.Helper()
		for i := 0; i < len(pairs); i += 2 {
			testkit.WriteFile(t, root, pairs[i], []byte(pairs[i+1]))
		}
	}
}

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

func several(edits ...func(testing.TB, string)) func(testing.TB, string) {
	return func(t testing.TB, root string) {
		t.Helper()
		for _, edit := range edits {
			edit(t, root)
		}
	}
}

func indexOf(entries ...imp.Entry) string { return string(imp.Write(entries)) }

func stopped(limit int, meddle func(testing.TB, string)) func(testing.TB, string) *countdown {
	return func(t testing.TB, root string) *countdown {
		before := map[int]func(){limit + 1: func() { meddle(t, root) }}
		return &countdown{Context: context.Background(), limit: limit, before: before}
	}
}

func beforeAsk(ask int, meddle func(testing.TB, string)) func(testing.TB, string) *countdown {
	return func(t testing.TB, root string) *countdown {
		before := map[int]func(){ask: func() { meddle(t, root) }}
		return &countdown{Context: context.Background(), limit: never, before: before}
	}
}

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
			[]run{{}, {edit: removing(m + "a.blp")},
				{edit: putting(m+"a.blp", "manual edit")},
				{edit: removing("assets/a.blp")},
				{build: true}}},
		{"an asset below a file of the map",
			project{files: holding("assets/a.blp", m+"Textures")},
			[]run{{block: `{"paths":{"a.blp":"textures/a.blp"}}`}}},
		{"a folder the map spells in its own way, and new folders in three spellings",
			project{files: holding(m+"Textures/existing.blp", "assets/textures/new.blp", "assets/a.blp",
				"assets/b.blp", "assets/c.blp")},
			[]run{{block: spellings}, {block: spellings}}},
		{"nothing to own, then one file, then nothing again",
			project{}, []run{{}, {edit: putting("assets/a.blp", "one")}, {edit: removing("assets/a.blp")}}},
		{"a folder appears where a new file goes, after the plan",
			project{files: holding("assets/a.blp", "assets/b.blp")},
			[]run{{meddle: putting(m+"b.blp/inner.txt", "another program's")}}},
		{"a write fails after a file was replaced: a file is where the folder of a new file goes",
			project{files: with(holding("assets/a.blp"), m+"war3mapImported/existing.wav", "editor",
				m+"war3map.imp", indexOf(imp.Entry{Flag: 5, Path: "existing.wav"}))},
			[]run{{}, {edit: putting("assets/a.blp", "second", "assets/Sound/b.blp", "new"),
				meddle: putting(m+"Sound", "another program's")}}},
		{"a file that cannot be taken out again when the sync is interrupted",
			project{files: holding("assets/a.blp", "assets/b.blp")},
			[]run{{syncCtx: stopped(1,
				several(removing(m+"a.blp"), putting(m+"a.blp/inner.txt", "another program's")))}}},
		{"nothing to import and nothing owned, in a map whose index does not read",
			project{files: map[string]string{m + "war3map.imp": "\x09\x09"}}, []run{{}, {build: true}}},
		{"a library's files beside the map's own, and the library dropped",
			project{files: holding("assets/Models/Own.mdx", "assets/icons/shared.blp",
				s+"ui/war3mapImported/ui/frames.toc", s+"ui/icons/shared.blp", s+"unlisted/never.txt")},
			[]run{{libraries: []string{"ui"}}, {libraries: []string{"ui"}}, {}}},
		{"a library's file where the map has a file of its own",
			project{files: holding(s+"ui/Models/Golem.mdx", m+"Models/Golem.mdx")},
			[]run{{libraries: []string{"ui"}}}},
	}
}

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
			run{}),
		one("an import in the folder World Editor imports into, where an asset goes",
			with(holding("assets/war3mapImported/a.wav"), m+"war3map.imp", indexOf(e(8, "A.wav"))),
			run{}),
		one("an index that lists a path twice",
			with(holding("assets/a.blp"), m+"war3map.imp",
				indexOf(e(13, `Textures\x.blp`), e(5, "y.wav"), e(29, "textures/X.BLP"))),
			run{}),
		one("an index that is cut short",
			with(holding("assets/a.blp"), m+"war3map.imp", indexOf(e(13, "x.blp"))[:10]),
			run{}),
		one("an index with a flag World Editor does not write",
			with(holding("assets/a.blp"), m+"war3map.imp", indexOf(e(7, "x.blp"))),
			run{}),
		one("an index with a path that leaves the map",
			with(holding("assets/a.blp"), m+"War3Map.imp", indexOf(e(13, `..\x.blp`))),
			run{}),
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
			run{}),
		{"an asset named as an empty folder of the map",
			project{files: holding("assets/empty"), folders: []string{m + "Empty"}},
			[]run{{}}},
		{"an asset below an owned file that no asset wants",
			project{files: holding("assets/data")},
			[]run{{}, {edit: several(removing("assets/data"), putting("assets/data/inner.txt", "inner"))}}},
		one("two assets without room, the first below a file",
			holding("assets/a/inner.blp", "assets/b.blp", m+"a", m+"b.blp"), run{}),
		one("two assets without room, the first at a file",
			holding("assets/a/inner.blp", "assets/b.blp", m+"a", m+"0.blp"),
			run{block: `{"paths":{"b.blp":"0.blp"}}`}),
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
			run{libraries: []string{"ui"}}),
		one("a folder named as the index", holding("assets/a.blp", m+"war3map.imp/stray.txt"),
			run{}),
		{"an owned file is edited by hand after the plan",
			project{files: holding("assets/0.blp", "assets/a.blp")},
			[]run{{}, {edit: putting("assets/0.blp", "second", "assets/a.blp", "second"),
				meddle: putting(m+"a.blp", "edited by hand")}}},
		one("a file appears where a new asset goes, after the plan", holding("assets/0.blp", "assets/a.blp"),
			run{meddle: putting(m+"a.blp", "the editor's")}),
		one("the state file cannot be written", holding("assets/a.blp"),
			run{syncCtx: beforeAsk(3, putting(state+"/in the way.txt", "another program's"))}),
		{"the state file is changed before it is written",
			project{files: holding("assets/a.blp")},
			[]run{{}, {edit: putting("assets/a.blp", "second"),
				syncCtx: beforeAsk(2, putting(state, "another program's"))}}},
		{"the state file is removed before it is written",
			project{files: holding("assets/a.blp")},
			[]run{{}, {edit: putting("assets/a.blp", "second"), syncCtx: beforeAsk(2, removing(state))}}},
		one("a state file is made before the first is written", holding("assets/a.blp"),
			run{syncCtx: beforeAsk(3, putting(state, "another program's"))}),
		{"the state file is changed before it is removed",
			project{files: holding("assets/a.blp")},
			[]run{{}, {edit: removing("assets/a.blp"),
				syncCtx: beforeAsk(3, putting(state, "another program's"))}}},
		{"the state file is changed while a sync that does not write it writes the index",
			project{files: holding("assets/a.blp")},
			[]run{{}, {edit: putting(m+"war3map.imp", indexOf()),
				syncCtx: beforeAsk(1, putting(state, "another program's"))}}},
		{"a state file is made while a sync that owns nothing, and found none, writes the map",
			project{files: holding("assets/a.blp")},
			[]run{{}, {edit: removing("assets/a.blp"), meddle: removing(state),
				syncCtx: beforeAsk(2, putting(state, "another program's"))}}},
	}
}

func interruptedProject() project {
	const m = sourceLabel + "/"
	first, dropped := fsx.SHA256Hex([]byte("first")), fsx.SHA256Hex([]byte("dropped"))
	owned := State{Files: []Owned{{"a.blp", first}, {"dropped.blp", dropped}}}
	index := indexOf(imp.Entry{Flag: 29, Path: "a.blp"}, imp.Entry{Flag: 29, Path: "dropped.blp"})
	return project{files: map[string]string{
		"assets/a.blp": "second", "assets/b.blp": "new", m + "a.blp": "first", m + "dropped.blp": "dropped",
		m + "war3map.imp": index, ".asset-state/" + mapFolder + ".json": string(owned.Encode()),
	}}
}

const interruptedAsks = 5

func interruptedStories() []story {
	limited := func(limit int) func(testing.TB, string) *countdown {
		return func(testing.TB, string) *countdown {
			return &countdown{Context: context.Background(), limit: limit}
		}
	}
	var stories []story
	for _, step := range []string{"the plan", "the sync"} {
		for limit := range interruptedAsks + 1 {
			r := run{planCtx: limited(limit)}
			if step == "the sync" {
				r = run{syncCtx: limited(limit)}
			}
			name := fmt.Sprintf("%s, cancelled at ask %d", step, limit+1)
			stories = append(stories, story{name, interruptedProject(), []run{r}})
		}
	}
	return stories
}

const longest = 64

func held(data []byte) string {
	if len(data) > longest {
		return testkit.Digest(data)
	}
	return strconv.Quote(string(data))
}

const underRoot = "<root>/"

func below(root, file string) string {
	if rel, err := filepath.Rel(root, file); err == nil && filepath.IsAbs(file) && filepath.IsLocal(rel) {
		return underRoot + filepath.ToSlash(rel)
	}
	return testkit.QuoteIfNeeded(file)
}

const internalError = "(an internal error)"

func fileOf(err error) string {
	failure, expected := diag.FirstProblem(err)
	if !expected {
		return internalError
	}
	return failure.File
}

type outcome struct {
	plan      *planned
	refused   bool
	refusedAt string
	left      map[string][]byte
}

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
			fmt.Fprintf(&out, "  %s: %s\n", testkit.QuoteIfNeeded(change.Name), what)
		}
		for _, owned := range o.plan.Owned {
			fmt.Fprintf(&out, "  owned: %s %s\n", testkit.QuoteIfNeeded(owned.Path), owned.Hash)
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
			fmt.Fprintf(&out, "  %s/\n", testkit.QuoteIfNeeded(name))
			continue
		}
		fmt.Fprintf(&out, "  %s: %s\n", testkit.QuoteIfNeeded(name), held(data))
		if entries, err := imp.Read(data, name); err == nil && mapdir.Key(filepath.Base(name)) == "war3map.imp" {
			for _, entry := range entries {
				fmt.Fprintf(&out, "    import %d %s\n", entry.Flag, testkit.QuoteIfNeeded(entry.Path))
			}
		}
	}
	return out.String()
}

func recordedStories(t testing.TB) []byte {
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
			out.WriteString(outcomeOf(t, r, root).lines())
		}
	}
	return []byte(out.String())
}

func outcomeOf(t testing.TB, r run, root string) outcome {
	t.Helper()
	did := r.in(t, root)
	made := outcome{plan: did.plan, left: testkit.Snapshot(t, root)}
	if did.err != nil {
		made.refused, made.refusedAt = true, below(root, fileOf(did.err))
	}
	return made
}

func TestTheStoriesOfThePlanAndTheSyncAreAsRecorded(t *testing.T) {
	testkit.CheckRecorded(t, "stories.txt", recordedStories(t))
}
