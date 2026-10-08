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

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/war3/imp"
)

const manifestName = "moonwell.local.pkl"

const noBlock = `{"paths":{},"exclude":[]}`

func asError(t testing.TB, err error, what string) *diag.Error {
	t.Helper()
	var failure *diag.Error
	if !errors.As(err, &failure) {
		t.Fatalf("%s: got %v, want a *diag.Error", what, err)
	}
	return failure
}

func put(t testing.TB, root, file string, content ...string) string {
	t.Helper()
	held := "asset"
	if len(content) > 0 {
		held = content[0]
	}
	return testkit.WriteFile(t, root, file, []byte(held))
}

func blockOf(t testing.TB, document string) manifest.Assets {
	t.Helper()
	var block manifest.Assets
	if err := json.Unmarshal([]byte(document), &block); err != nil {
		t.Fatalf("the assets block %s: %v", document, err)
	}
	return block
}

func shipped(root string, keys ...string) []Library {
	var libraries []Library
	for _, key := range keys {
		libraries = append(libraries, Library{Key: key, Dir: filepath.Join(root, "libraries", key)})
	}
	return libraries
}

func collect(t testing.TB, root, block string, libraries ...string) ([]Asset, []string) {
	t.Helper()
	assets, replaced, err := Collect(root, blockOf(t, block), manifestName, shipped(root, libraries...))
	if err != nil {
		t.Fatalf("Collect: %v", diag.Format(err))
	}
	return assets, replaced
}

func refused(t testing.TB, root, block string, libraries ...string) *diag.Error {
	t.Helper()
	assets, replaced, err := Collect(root, blockOf(t, block), manifestName, shipped(root, libraries...))
	if assets != nil || replaced != nil {
		t.Errorf("Collect returned %d assets and %q beside its error", len(assets), replaced)
	}
	return asError(t, err, "the assets block "+block)
}

var background = context.Background()

const (
	mapLabel  = "maps/map.w3x"
	stateName = ".asset-state/map.w3x.json"
)

type site struct {
	t      testing.TB
	root   string
	mapDir string
	state  string
}

func newSite(t testing.TB) *site {
	t.Helper()
	root := t.TempDir()
	s := &site{t: t, root: root, mapDir: filepath.Join(root, "maps", "map.w3x"),
		state: filepath.Join(root, filepath.FromSlash(stateName))}
	for _, dir := range []string{s.mapDir, filepath.Join(root, "assets")} {
		if err := os.MkdirAll(dir, 0o777); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func (s *site) open() *mapdir.Folder {
	s.t.Helper()
	folder, err := mapdir.Open(s.mapDir, mapLabel)
	if err != nil {
		s.t.Fatalf("opening the source map: %v", diag.Format(err))
	}
	return folder
}

func (s *site) plan(ctx context.Context, block string, libraries ...string) (*mapdir.Folder, *Result, error) {
	s.t.Helper()
	assets, _ := collect(s.t, s.root, block, libraries...)
	owned, err := ReadState(s.root, stateName)
	if err != nil {
		s.t.Fatalf("reading the state: %v", diag.Format(err))
	}
	folder := s.open()
	result, err := Plan(ctx, folder, assets, owned)
	return folder, result, err
}

func (s *site) planned(block string, libraries ...string) (*mapdir.Folder, *Result) {
	s.t.Helper()
	folder, result, err := s.plan(background, block, libraries...)
	if err != nil {
		s.t.Fatalf("Plan: %v", diag.Format(err))
	}
	return folder, result
}

func (s *site) refusedPlan(block string, libraries ...string) *diag.Error {
	s.t.Helper()
	_, result, err := s.plan(background, block, libraries...)
	if result != nil {
		s.t.Errorf("Plan returned %+v beside its error", result)
	}
	return asError(s.t, err, "a refused plan")
}

func (s *site) synced(block string, libraries ...string) *Result {
	s.t.Helper()
	folder, result := s.planned(block, libraries...)
	if err := Sync(background, folder, result, s.root, stateName); err != nil {
		s.t.Fatalf("Sync: %v", diag.Format(err))
	}
	return result
}

func (s *site) owns(names ...string) {
	s.t.Helper()
	var state State
	for _, name := range names {
		state.Files = append(state.Files, Owned{Path: name, Hash: fsx.SHA256Hex([]byte(s.inMap(name)))})
	}
	testkit.WriteFile(s.t, s.root, stateName, state.Bytes())
}

const missing = "<missing>"

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

func (s *site) inMap(name string) string {
	s.t.Helper()
	return textOf(s.t, filepath.Join(s.mapDir, filepath.FromSlash(name)))
}

func (s *site) stateText() string {
	s.t.Helper()
	return textOf(s.t, s.state)
}

func (s *site) imports() []imp.Entry {
	s.t.Helper()
	entries, err := imp.Read([]byte(s.inMap("war3map.imp")), "war3map.imp")
	if err != nil {
		s.t.Fatalf("the map's war3map.imp: %v", diag.Format(err))
	}
	return entries
}

func (s *site) setImports(entries ...imp.Entry) {
	s.t.Helper()
	testkit.WriteFile(s.t, s.mapDir, "war3map.imp", imp.Write(entries))
}

func (s *site) unchanged(before map[string][]byte, what string) {
	s.t.Helper()
	if after := testkit.Snapshot(s.t, s.root); !maps.EqualFunc(before, after, slices.Equal) {
		s.t.Errorf("%s changed the project: it holds %q", what, slices.Sorted(maps.Keys(after)))
	}
}

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

const never = math.MaxInt

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

type row struct{ library, source, target string }

func rows(assets []Asset) []row {
	list := []row{}
	for _, asset := range assets {
		list = append(list, row{asset.Library, asset.Source, asset.Target})
	}
	return list
}
