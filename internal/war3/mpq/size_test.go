package mpq

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

func randomBytes(seed uint64, length int) []byte {
	random := rand.New(rand.NewPCG(seed, 2026))
	out := make([]byte, length)
	for i := range out {
		out[i] = byte(random.UintN(256))
	}
	return out
}

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
			{Name: "one.bin", Data: randomBytes(1, 1)},
			{Name: `folder\sector.bin`, Data: randomBytes(2, 4096)},
			{Name: "more.bin", Data: randomBytes(3, 4097)},
			{Name: "three.bin", Data: randomBytes(4, 10000)},
		}, true},
		{"more files than the smallest table takes", 0, slices.Repeat([]File{{}}, 11), true},
		{"a file that compresses", 0,
			[]File{{Name: "war3map.lua", Data: []byte(strings.Repeat("print()\n", 900))}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sizes []fileSize
			for index := range tt.files {
				if tt.files[index].Name == "" {
					tt.files[index] = File{Name: "file" + string(rune('a'+index)), Data: randomBytes(uint64(index), 100)}
				}
				sizes = append(sizes, fileSize{tt.files[index].Name, int64(len(tt.files[index].Data))})
			}
			written, err := Write(tt.files, Options{Prefix: make([]byte, tt.prefix)})
			if err != nil {
				t.Fatal(err)
			}
			counted := maxArchiveSize(int64(tt.prefix), sizes)
			if counted < int64(len(written)) || (tt.exact && counted != int64(len(written))) {
				t.Errorf("counted %d bytes; the archive has %d (exact: %v)", counted, len(written), tt.exact)
			}
		})
	}
}

func TestCheckFitsRefusesWhatTheFormatsFieldsCannotHold(t *testing.T) {
	const most = 1<<32 - 1
	const largestAlone = 4290776748
	tests := []struct {
		name     string
		prefix   int64
		files    []fileSize
		tooLarge string
		fits     bool
	}{
		{"a small map", 512, []fileSize{{"war3map.lua", 5000}, {"war3map.w3i", 800}}, "", true},
		{"no file at all", 0, nil, "", true},
		{"the largest file an archive of one file holds", 0, []fileSize{{"f", largestAlone}}, "", true},
		{"one byte more", 0, []fileSize{{"f", largestAlone + 1}}, "", false},
		{"the largest file behind a header of 512 bytes", 512, []fileSize{{"f", largestAlone - 512}}, "", true},
		{"one byte more behind the header", 512, []fileSize{{"f", largestAlone - 511}}, "", false},
		{"a file whose size is the most a field holds", 0, []fileSize{{"small", 10}, {"f", most}}, "", false},
		{"a file whose size no field holds", 0, []fileSize{{"small", 10}, {"big.bin", most + 1}, {"f", most + 2}},
			"big.bin", false},
		{"files that fit each and not together", 0, []fileSize{{"a", 1 << 31}, {"b", 1 << 31}}, "", false},
		{"empty files whose names do not fit the list of the files", 0,
			slices.Repeat([]fileSize{{strings.Repeat("n", 1<<16), 0}}, 1<<16), "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tooLarge, fits := checkFits(tt.prefix, tt.files)
			if tooLarge != tt.tooLarge || fits != tt.fits {
				t.Errorf("roomFor = %q, %v, want %q, %v", tooLarge, fits, tt.tooLarge, tt.fits)
			}
		})
	}
}
