package assets

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/mapdir"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/war3/imp"
)

// manifestName is the manifest the projects of these tests were evaluated from.
const manifestName = "moonwell.local.pkl"

// noBlock is the assets block of a manifest that sets nothing in it.
const noBlock = `{"paths":{},"exclude":[]}`

// asError is err as the expected failure it must be.
func asError(t testing.TB, err error, what string) *diag.Error {
	t.Helper()
	var failure *diag.Error
	if !errors.As(err, &failure) {
		t.Fatalf("%s: got %v, want a *diag.Error", what, err)
	}
	return failure
}

// put writes a file of the project, with "asset" in it unless the test gives its content.
func put(t testing.TB, root, file string, content ...string) string {
	t.Helper()
	held := "asset"
	if len(content) > 0 {
		held = content[0]
	}
	return testkit.WriteFile(t, root, file, []byte(held))
}

// blockOf is a manifest's assets block as pkl prints it, with its paths in the order of the text.
func blockOf(t testing.TB, document string) manifest.Assets {
	t.Helper()
	var block manifest.Assets
	if err := json.Unmarshal([]byte(document), &block); err != nil {
		t.Fatalf("the assets block %s: %v", document, err)
	}
	return block
}

// shipped is the libraries with these keys, each with its files in the folder libraries/<key> of the project.
func shipped(root string, keys ...string) []Library {
	var libraries []Library
	for _, key := range keys {
		libraries = append(libraries, Library{Key: key, Dir: filepath.Join(root, "libraries", key)})
	}
	return libraries
}

// collect is what a project with this assets block and these libraries imports. It must not be refused.
func collect(t testing.TB, root, block string, libraries ...string) ([]Asset, []string) {
	t.Helper()
	assets, replaced, err := Collect(root, blockOf(t, block), manifestName, shipped(root, libraries...))
	if err != nil {
		t.Fatalf("Collect: %v", diag.Format(err))
	}
	return assets, replaced
}

// refused is the failure that a project with this assets block and these libraries is refused with.
func refused(t testing.TB, root, block string, libraries ...string) *diag.Error {
	t.Helper()
	assets, replaced, err := Collect(root, blockOf(t, block), manifestName, shipped(root, libraries...))
	if assets != nil || replaced != nil {
		t.Errorf("Collect returned %d assets and %q beside its error", len(assets), replaced)
	}
	return asError(t, err, "the assets block "+block)
}

// ---- a project with a source map, for the plan and the sync ----

var background = context.Background()

// mapLabel is how the source map of a site is named in errors.
const mapLabel = "maps/map.w3x"

// site is a project with a source map and an assets folder, both empty.
type site struct {
	t      testing.TB
	root   string // the project folder
	mapDir string // maps/map.w3x in it
	state  string // the ownership file of that map
}

func newSite(t testing.TB) *site {
	t.Helper()
	root := t.TempDir()
	state, err := StateFile(root, "map.w3x")
	if err != nil {
		t.Fatal(err)
	}
	s := &site{t: t, root: root, mapDir: filepath.Join(root, "maps", "map.w3x"), state: state}
	for _, dir := range []string{s.mapDir, filepath.Join(root, "assets")} {
		if err := os.MkdirAll(dir, 0o777); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

// open is the source map as a command opens it.
func (s *site) open() *mapdir.Folder {
	s.t.Helper()
	folder, err := mapdir.Open(s.mapDir, mapLabel)
	if err != nil {
		s.t.Fatalf("opening the source map: %v", diag.Format(err))
	}
	return folder
}

// plan is the source map as a command opens it, and Plan's answer for what the project imports and owns.
func (s *site) plan(ctx context.Context, block string, libraries ...string) (*mapdir.Folder, *Result, error) {
	s.t.Helper()
	assets, _ := collect(s.t, s.root, block, libraries...)
	owned, err := ReadState(s.state)
	if err != nil {
		s.t.Fatalf("reading the state: %v", diag.Format(err))
	}
	folder := s.open()
	result, err := Plan(ctx, folder, assets, owned)
	return folder, result, err
}

// planned is a plan that must not be refused.
func (s *site) planned(block string, libraries ...string) (*mapdir.Folder, *Result) {
	s.t.Helper()
	folder, result, err := s.plan(background, block, libraries...)
	if err != nil {
		s.t.Fatalf("Plan: %v", diag.Format(err))
	}
	return folder, result
}

// refusedPlan is the failure a plan is refused with. A refused plan returns nothing beside it.
func (s *site) refusedPlan(block string, libraries ...string) *diag.Error {
	s.t.Helper()
	_, result, err := s.plan(background, block, libraries...)
	if result != nil {
		s.t.Errorf("Plan returned %+v beside its error", result)
	}
	return asError(s.t, err, "a refused plan")
}

// synced plans and writes, as assets:sync does. Neither may be refused.
func (s *site) synced(block string, libraries ...string) *Result {
	s.t.Helper()
	folder, result := s.planned(block, libraries...)
	if err := Sync(background, folder, result, s.state); err != nil {
		s.t.Fatalf("Sync: %v", diag.Format(err))
	}
	return result
}

// owns writes the state file: the map's files under these names, each with the hash of what it holds.
func (s *site) owns(names ...string) {
	s.t.Helper()
	var state State
	for _, name := range names {
		state.Files = append(state.Files, Owned{Path: name, Hash: fsx.SHA256Hex([]byte(s.inMap(name)))})
	}
	testkit.WriteFile(s.t, s.root, ".asset-state/map.w3x.json", state.Bytes())
}

// missing is what inMap and stateText give for a file that is not there.
const missing = "<missing>"

// textOf is what the file at path holds, or missing.
func textOf(t testing.TB, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return missing
	case err != nil:
		t.Fatal(err)
	}
	return string(data)
}

// inMap is what the source map holds under name, with "/".
func (s *site) inMap(name string) string {
	s.t.Helper()
	return textOf(s.t, filepath.Join(s.mapDir, filepath.FromSlash(name)))
}

// stateText is what the state file holds.
func (s *site) stateText() string {
	s.t.Helper()
	return textOf(s.t, s.state)
}

// imports is the entries of the source map's war3map.imp.
func (s *site) imports() []imp.Entry {
	s.t.Helper()
	entries, err := imp.Read([]byte(s.inMap("war3map.imp")), "war3map.imp")
	if err != nil {
		s.t.Fatalf("the map's war3map.imp: %v", diag.Format(err))
	}
	return entries
}

// setImports writes the source map's war3map.imp.
func (s *site) setImports(entries ...imp.Entry) {
	s.t.Helper()
	testkit.WriteFile(s.t, s.mapDir, "war3map.imp", imp.Write(entries))
}

// unchanged fails the test unless the project holds what it held at before, a snapshot of it.
func (s *site) unchanged(before map[string][]byte, what string) {
	s.t.Helper()
	if after := testkit.Snapshot(s.t, s.root); !maps.EqualFunc(before, after, slices.Equal) {
		s.t.Errorf("%s changed the project: it holds %q", what, slices.Sorted(maps.Keys(after)))
	}
}

// names is the name of each change, a removal with a "-" before it.
func names(changes []mapdir.Change) []string {
	list := []string{}
	for _, change := range changes {
		if change.Remove {
			list = append(list, "-"+change.Name)
		} else {
			list = append(list, change.Name)
		}
	}
	return list
}

// never is a limit no countdown reaches.
const never = math.MaxInt

// countdown is a context that reads as cancelled from an ask of Err on: the first limit asks are answered with
// nil. Before it answers an ask, it runs what the test wants to happen in the project at that moment, by the
// number of the ask from 1: what another program does between two files.
type countdown struct {
	context.Context
	asks, limit int
	before      map[int]func()
}

func (c *countdown) Err() error {
	c.asks++
	if meddle := c.before[c.asks]; meddle != nil {
		meddle()
	}
	if c.asks > c.limit {
		return context.Canceled
	}
	return nil
}

// row is an asset without its bytes.
type row struct{ library, source, target string }

func rows(assets []Asset) []row {
	list := []row{}
	for _, asset := range assets {
		list = append(list, row{asset.Library, asset.Source, asset.Target})
	}
	return list
}
