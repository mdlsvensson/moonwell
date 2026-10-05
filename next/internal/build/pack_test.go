package build

import (
	"encoding/binary"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/mapdir"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/war3/mpq"
)

// ---- what the tests of the archive are made of ----

// mapLabel is how the map of these tests is named in errors.
const mapLabel = "maps/map.w3x"

// mapInfo is the start of a war3map.w3i of a format version, saved by a game of a version.
func mapInfo(version int32, major, minor uint32) string {
	info := make([]byte, 64)
	binary.LittleEndian.PutUint32(info[0:], uint32(version))
	binary.LittleEndian.PutUint32(info[12:], major)
	binary.LittleEndian.PutUint32(info[16:], minor)
	return string(info)
}

// modernInfo is the war3map.w3i of a map that is packed without a header before the archive.
var modernInfo = mapInfo(39, 3, 0)

// viewOf opens a map folder that holds the files, each named with "/".
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

// smallMap is a map with its info, its script and one imported file.
func smallMap(info string) map[string]string {
	return map[string]string{
		"war3map.w3i":           info,
		"war3map.lua":           "function main() end",
		"war3mapImported/a.txt": "asset",
	}
}

// opened is an archive as a reader of it, which must find one.
func opened(t testing.TB, archive []byte) *testkit.MPQ {
	t.Helper()
	reader, err := testkit.OpenMPQ(archive)
	if err != nil {
		t.Fatal(err)
	}
	return reader
}

// fileOf is a file of an archive, which must hold it.
func fileOf(t testing.TB, archive *testkit.MPQ, name string) string {
	t.Helper()
	data, found, err := archive.Read(name)
	if err != nil || !found {
		t.Fatalf("the archive's %s: found %v, %v", name, found, err)
	}
	return string(data)
}

// namesIn is the names an archive lists, in its order.
func namesIn(t testing.TB, archive *testkit.MPQ) []string {
	t.Helper()
	names, err := archive.Listfile()
	if err != nil {
		t.Fatal(err)
	}
	return names
}

// packedOf is the archive of a view, which must be packed.
func packedOf(t testing.TB, view *mapdir.Folder, name string) []byte {
	t.Helper()
	archive, err := pack(view, name)
	if err != nil {
		t.Fatalf("pack: %v", diag.Format(err))
	}
	return archive
}

// ---- pack ----

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
	// A file of that name below a folder is a file of the map as any other.
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

// The archive is packed from the view, not from a folder on disk: it holds what the plan writes, without what
// the plan removes, though no stage was written.
func TestPackPacksThePlannedViewAndNotWhatIsOnDisk(t *testing.T) {
	view := viewOf(t, smallMap(modernInfo)).With([]mapdir.Change{
		{Name: "war3map.lua", Bytes: []byte("function main() end -- bundled")},
		{Name: "war3mapImported/a.txt", Remove: true},
		{Name: "icons/new.blp", Bytes: []byte("new")},
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

// The order of the archive's files is that of Folder.Files: each folder's entries by the bytes of their names,
// a folder's files where the folder stands, and then the files the plan adds, in the order they were planned.
func TestPackKeepsTheOrderOfTheViewsFiles(t *testing.T) {
	files := smallMap(modernInfo)
	files["Zeta.txt"] = "upper case sorts first"
	files["alpha/b.txt"] = "b"
	files["alpha/B/c.txt"] = "c"
	files["war3map.doo"] = "doodads"
	view := viewOf(t, files).With([]mapdir.Change{
		{Name: "zz/late.txt", Bytes: []byte("late")},
		{Name: "alpha/a.txt", Bytes: []byte("a")},
		{Name: "war3map.doo", Bytes: []byte("patched")},
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

// ---- the size an archive can have ----

// noise is length bytes that do not compress, the same bytes for the same seed.
func noise(seed uint64, length int) string {
	random := rand.New(rand.NewPCG(seed, 2026))
	out := make([]byte, length)
	for i := range out {
		out[i] = byte(random.UintN(256))
	}
	return string(out)
}

// The guard counts an archive as war3/mpq lays one out, with every sector stored as it is. For files that do not
// compress that is the archive's size to the byte, and for files that do it is more.
func TestLargestArchiveIsTheSizeOfAnArchiveWhoseFilesDoNotCompress(t *testing.T) {
	tests := []struct {
		name   string
		prefix int
		files  []mpq.File
		exact  bool
	}{
		{"no file", 0, nil, true},
		{"an empty file", 0, []mpq.File{{Name: "empty.txt"}}, true},
		{"files that end inside, at and after a sector", 512, []mpq.File{
			{Name: "one.bin", Data: []byte(noise(1, 1))},
			{Name: `folder\sector.bin`, Data: []byte(noise(2, 4096))},
			{Name: "more.bin", Data: []byte(noise(3, 4097))},
			{Name: "three.bin", Data: []byte(noise(4, 10000))},
		}, true},
		// With eleven files and the list of them, a table of sixteen slots leaves less than a third free.
		{"more files than the smallest table takes", 0, slices.Repeat([]mpq.File{{}}, 11), true},
		{"a file that compresses", 0,
			[]mpq.File{{Name: "war3map.lua", Data: []byte(strings.Repeat("print()\n", 900))}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sizes []sized
			for at := range tt.files {
				if tt.files[at].Name == "" {
					tt.files[at] = mpq.File{Name: "file" + string(rune('a'+at)), Data: []byte(noise(uint64(at), 100))}
				}
				sizes = append(sizes, sized{tt.files[at].Name, int64(len(tt.files[at].Data))})
			}
			written, err := mpq.Write(tt.files, mpq.Options{Prefix: make([]byte, tt.prefix)})
			if err != nil {
				t.Fatal(err)
			}
			counted := largestArchive(int64(tt.prefix), sizes)
			if counted < int64(len(written)) || (tt.exact && counted != int64(len(written))) {
				t.Errorf("counted %d bytes; the archive has %d (exact: %v)", counted, len(written), tt.exact)
			}
		})
	}
}

func TestRoomForRefusesWhatTheFormatsFieldsCannotHold(t *testing.T) {
	const most = 1<<32 - 1 // what a field of 32 bits holds
	// One file named "f", stored in sectors of 4096 bytes: n bytes, four for each sector's end and four more,
	// the list of the files (3 bytes, in a sector of its own with 8 for its table), the header (32), a hash table
	// of 16 slots and a block table of 2 blocks, 16 bytes each. With n = 4290776748 that is 4294967295 bytes.
	const largestAlone = 4290776748
	tests := []struct {
		name     string
		prefix   int64
		files    []sized
		tooLarge string // the file that is refused; "" for the map as a whole
		fits     bool
	}{
		{"a small map", 512, []sized{{"war3map.lua", 5000}, {"war3map.w3i", 800}}, "", true},
		{"no file at all", 0, nil, "", true},
		{"the largest file an archive of one file holds", 0, []sized{{"f", largestAlone}}, "", true},
		{"one byte more", 0, []sized{{"f", largestAlone + 1}}, "", false},
		{"the largest file behind a header of 512 bytes", 512, []sized{{"f", largestAlone - 512}}, "", true},
		{"one byte more behind the header", 512, []sized{{"f", largestAlone - 511}}, "", false},
		{"a file whose size is the most a field holds", 0, []sized{{"small", 10}, {"f", most}}, "", false},
		{"a file whose size no field holds", 0, []sized{{"small", 10}, {"big.bin", most + 1}, {"f", most + 2}},
			"big.bin", false},
		{"files that fit each and not together", 0, []sized{{"a", 1 << 31}, {"b", 1 << 31}}, "", false},
		// 65536 names of 65536 bytes: the list of them alone is more than a word counts.
		{"empty files whose names do not fit the list of the files", 0,
			slices.Repeat([]sized{{strings.Repeat("n", 1<<16), 0}}, 1<<16), "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tooLarge, fits := roomFor(tt.prefix, tt.files)
			if tooLarge != tt.tooLarge || fits != tt.fits {
				t.Errorf("roomFor = %q, %v, want %q, %v", tooLarge, fits, tt.tooLarge, tt.fits)
			}
		})
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
