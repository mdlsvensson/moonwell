package mpq

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

// noise is length bytes that do not compress, the same bytes for the same seed.
func noise(seed uint64, length int) []byte {
	random := rand.New(rand.NewPCG(seed, 2026))
	out := make([]byte, length)
	for i := range out {
		out[i] = byte(random.UintN(256))
	}
	return out
}

// The guard counts an archive as Write lays one out, with every sector stored as it is. For files that do not
// compress that is the archive's size to the byte, and for files that do it is more.
func TestLargestArchiveIsTheSizeOfAnArchiveWhoseFilesDoNotCompress(t *testing.T) {
	tests := []struct {
		name   string
		prefix int
		files  []File
		exact  bool
	}{
		{"no file", 0, nil, true},
		{"an empty file", 0, []File{{Name: "empty.txt"}}, true},
		{"files that end inside, at and after a sector", 512, []File{
			{Name: "one.bin", Data: noise(1, 1)},
			{Name: `folder\sector.bin`, Data: noise(2, 4096)},
			{Name: "more.bin", Data: noise(3, 4097)},
			{Name: "three.bin", Data: noise(4, 10000)},
		}, true},
		// With eleven files and the list of them, a table of sixteen slots leaves less than a third free.
		{"more files than the smallest table takes", 0, slices.Repeat([]File{{}}, 11), true},
		{"a file that compresses", 0,
			[]File{{Name: "war3map.lua", Data: []byte(strings.Repeat("print()\n", 900))}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sizes []sized
			for at := range tt.files {
				if tt.files[at].Name == "" {
					tt.files[at] = File{Name: "file" + string(rune('a'+at)), Data: noise(uint64(at), 100)}
				}
				sizes = append(sizes, sized{tt.files[at].Name, int64(len(tt.files[at].Data))})
			}
			written, err := Write(tt.files, Options{Prefix: make([]byte, tt.prefix)})
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
