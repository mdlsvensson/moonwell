package build

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

const mapLabel = "maps/map.w3x"

func mapInfo(version int32, major, minor uint32) string {
	info := make([]byte, 64)
	binary.LittleEndian.PutUint32(info[0:], uint32(version))
	binary.LittleEndian.PutUint32(info[12:], major)
	binary.LittleEndian.PutUint32(info[16:], minor)
	return string(info)
}

var modernInfo = mapInfo(39, 3, 0)

func viewOf(t testing.TB, files map[string]string) *mapdir.Folder {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "maps", "map.w3x")
	if err := os.MkdirAll(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	for name, data := range files {
		testkit.WriteFile(t, dir, name, []byte(data))
	}
	view, err := mapdir.Open(dir, mapLabel)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	return view
}

func smallMap(info string) map[string]string {
	return map[string]string{
		"war3map.w3i":           info,
		"war3map.lua":           "function main() end",
		"war3mapImported/a.txt": "asset",
	}
}

func opened(t testing.TB, archive []byte) *testkit.MPQ {
	t.Helper()
	reader, err := testkit.OpenMPQ(archive)
	if err != nil {
		t.Fatal(err)
	}
	return reader
}

func fileOf(t testing.TB, archive *testkit.MPQ, name string) string {
	t.Helper()
	data, found, err := archive.Read(name)
	if err != nil || !found {
		t.Fatalf("the archive's %s: found %v, %v", name, found, err)
	}
	return string(data)
}

func namesIn(t testing.TB, archive *testkit.MPQ) []string {
	t.Helper()
	names, err := archive.Listfile()
	if err != nil {
		t.Fatal(err)
	}
	return names
}

func packedOf(t testing.TB, view *mapdir.Folder, name string) []byte {
	t.Helper()
	archive, err := pack(view, name)
	if err != nil {
		t.Fatalf("pack: %v", diag.Format(err))
	}
	return archive
}

func TestPackWritesAHeaderlessArchiveForModernMapsWithBackslashPaths(t *testing.T) {
	archive := opened(t, packedOf(t, viewOf(t, smallMap(modernInfo)), "map"))
	if archive.HeaderOffset != 0 {
		t.Errorf("HeaderOffset = %d", archive.HeaderOffset)
	}
	if got := fileOf(t, archive, `war3mapImported\a.txt`); got != "asset" {
		t.Errorf("the asset reads as %q", got)
	}
	if got := fileOf(t, archive, "war3map.lua"); got != "function main() end" {
		t.Errorf("the script reads as %q", got)
	}
}

func TestPackPrefixesHM3WForOlderMaps(t *testing.T) {
	tests := []struct {
		name string
		info string
	}{
		{"a format before the game's version is recorded", mapInfo(25, 0, 0)},
		{"the newest format, saved by a game before 1.31", mapInfo(39, 1, 30)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packed := packedOf(t, viewOf(t, smallMap(tt.info)), "Old Map")
			if string(packed[:4]) != "HM3W" || string(packed[8:16]) != "Old Map\x00" ||
				opened(t, packed).HeaderOffset != 512 {
				t.Errorf("the archive starts with %q", packed[:16])
			}
		})
	}
}

func TestPackRequiresTheMapInfo(t *testing.T) {
	_, err := pack(viewOf(t, map[string]string{"war3map.lua": "function main() end"}), "map")
	e := asError(t, err, "a map without war3map.w3i")
	if e.Msg != "war3map.w3i is missing from the map folder." || e.File != mapLabel ||
		e.Hint != "Save the source map from World Editor in folder format." {
		t.Errorf("error = %+v", e)
	}
}

func TestPackNamesTheMapInfoItCannotRead(t *testing.T) {
	files := smallMap("\x27\x00")
	_, err := pack(viewOf(t, files), "map")
	e := asError(t, err, "a war3map.w3i of two bytes")
	if !strings.Contains(e.Msg, "war3map.w3i is truncated") || e.File != mapLabel+"/war3map.w3i" || e.Hint == "" {
		t.Errorf("error = %+v", e)
	}
}

func TestPackSkipsStaleArchiveMetadataFromTheMapFolder(t *testing.T) {
	files := smallMap(modernInfo)
	files["(attributes)"] = "stale"
	files["(ListFile)"] = "stale\r\n"
	files["(SIGNATURE)"] = "stale"
	files["war3mapImported/(signature)"] = "kept"
	archive := opened(t, packedOf(t, viewOf(t, files), "map"))
	for _, name := range []string{"(attributes)", "(signature)"} {
		if _, found, _ := archive.Read(name); found {
			t.Errorf("the stale %s was packed", name)
		}
	}
	want := []string{"war3map.lua", "war3map.w3i", `war3mapImported\(signature)`, `war3mapImported\a.txt`}
	if names := namesIn(t, archive); !slices.Equal(names, want) {
		t.Errorf("the archive lists %q, want %q", names, want)
	}
}

func TestPackPacksThePlannedViewAndNotWhatIsOnDisk(t *testing.T) {
	view := viewOf(t, smallMap(modernInfo)).WithChanges([]mapdir.Change{
		{Path: "war3map.lua", Data: []byte("function main() end -- bundled")},
		{Path: "war3mapImported/a.txt", Remove: true},
		{Path: "icons/new.blp", Data: []byte("new")},
	})
	archive := opened(t, packedOf(t, view, "map"))
	if got := fileOf(t, archive, "war3map.lua"); got != "function main() end -- bundled" {
		t.Errorf("the script reads as %q", got)
	}
	if _, found, _ := archive.Read(`war3mapImported\a.txt`); found {
		t.Error("the file the plan removes was packed")
	}
	want := []string{"war3map.lua", "war3map.w3i", `icons\new.blp`}
	if names := namesIn(t, archive); !slices.Equal(names, want) {
		t.Errorf("the archive lists %q, want %q", names, want)
	}
}

func TestPackKeepsTheOrderOfTheViewsFiles(t *testing.T) {
	files := smallMap(modernInfo)
	files["Zeta.txt"] = "upper case sorts first"
	files["alpha/b.txt"] = "b"
	files["alpha/B/c.txt"] = "c"
	files["war3map.doo"] = "doodads"
	view := viewOf(t, files).WithChanges([]mapdir.Change{
		{Path: "zz/late.txt", Data: []byte("late")},
		{Path: "alpha/a.txt", Data: []byte("a")},
		{Path: "war3map.doo", Data: []byte("patched")},
	})
	want := []string{
		"Zeta.txt", `alpha\B\c.txt`, `alpha\b.txt`, "war3map.doo", "war3map.lua", "war3map.w3i",
		`war3mapImported\a.txt`, `zz\late.txt`, `alpha\a.txt`,
	}
	inView := []string{}
	for _, name := range view.Files() {
		inView = append(inView, strings.ReplaceAll(name, "/", `\`))
	}
	if !slices.Equal(inView, want) {
		t.Fatalf("the view lists %q, want %q", inView, want)
	}
	if names := namesIn(t, opened(t, packedOf(t, view, "map"))); !slices.Equal(names, want) {
		t.Errorf("the archive lists %q, want %q", names, want)
	}
}

func TestTheRefusalsOfAMapThatIsTooLargeNameTheFileOrTheMap(t *testing.T) {
	view := viewOf(t, map[string]string{"War3Map.w3i": modernInfo})
	file := asError(t, errTooLarge(view, "war3map.w3i"), "a file that is too large")
	if file.File != mapLabel+"/War3Map.w3i" || !strings.Contains(file.Msg, "War3Map.w3i is too large") ||
		file.Hint == "" {
		t.Errorf("error = %+v", file)
	}
	whole := asError(t, errTooLarge(view, ""), "a map that is too large")
	if whole.File != mapLabel || !strings.Contains(whole.Msg, "The map is too large") || whole.Hint == "" {
		t.Errorf("error = %+v", whole)
	}
}
