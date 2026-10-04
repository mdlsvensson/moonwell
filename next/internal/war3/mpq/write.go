package mpq

import (
	"bytes"
	"cmp"
	"compress/zlib"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/next/internal/binio"
	"github.com/mdlsvensson/moonwell/next/internal/diag"
)

// File is one file of an archive.
type File struct {
	Name string // the archive path, with backslashes
	Data []byte
}

// Options change how an archive is written.
type Options struct {
	Prefix          []byte // before the archive; a multiple of 512 bytes
	SectorSizeShift int    // sectors of 512 << shift bytes; zero means 3
}

const (
	// alignment is what the length of a prefix must be a multiple of: a reader looks for the archive at every
	// 512th byte.
	alignment = 512
	// smallestSector is the sector size in bytes that a shift of zero stands for in an archive's header.
	smallestSector = 512
	// defaultSectorShift gives sectors of 4096 bytes.
	defaultSectorShift = 3
	// listfileName is the file of an archive that names its other files, one to a line.
	listfileName = "(listfile)"

	magic         = 0x1A51504D // "MPQ\x1A"
	headerSize    = 32
	formatVersion = 0 // format 1

	// The tables and the offsets before a file's sectors are made of words of four bytes. An entry of the hash
	// table or of the block table is four words.
	wordSize   = 4
	entryWords = 4

	// minHashSlots is the smallest hash table; a table has a power of two of slots.
	minHashSlots = 16
	// free is every word of a hash table slot that holds no file.
	free = 0xFFFFFFFF
	// neutral is the locale and the platform of a file that is for all of them.
	neutral = 0

	// The flags of a block. Every block exists; one with data is stored as sectors, each compressed or not.
	flagExists     = 0x80000000
	flagCompressed = 0x00000200

	// compressedWithZlib is the first byte of a sector that is stored compressed.
	compressedWithZlib = 0x02
)

// Write builds an archive of files, in their order, followed by a (listfile) that names them.
//
// The archive is laid out as the prefix, a header, the data of each file, the hash table and the block table.
// Positions inside it are counted from the header. A (listfile) among the files is left out, and two files whose
// names are one archive path are refused.
func Write(files []File, options Options) ([]byte, error) {
	if len(options.Prefix)%alignment != 0 {
		return nil, errPrefixLength()
	}
	if err := checkPaths(files); err != nil {
		return nil, err
	}
	entries := withListfile(files)
	shift := cmp.Or(options.SectorSizeShift, defaultSectorShift)
	data, blocks, err := storeFiles(entries, smallestSector<<shift)
	if err != nil {
		return nil, err
	}
	hashes := hashTable(entries)
	hashTableAt := headerSize + len(data)
	blockTableAt := hashTableAt + len(hashes)*wordSize

	var w binio.Writer
	w.Write(options.Prefix)
	writeHeader(&w, header{
		archiveSize:  blockTableAt + len(blocks)*wordSize,
		sectorShift:  shift,
		hashTableAt:  hashTableAt,
		blockTableAt: blockTableAt,
		hashSlots:    len(hashes) / entryWords,
		blocks:       len(entries),
	})
	w.Write(data)
	writeTable(&w, hashes, HashTableKey)
	writeTable(&w, blocks, BlockTableKey)
	return w.Bytes(), nil
}

// pathKey is name as the game compares archive paths: with the upper-casing HashString uses. Two names with one
// key have the same hashes, so the game finds one file under both.
func pathKey(name string) string {
	key := []byte(name)
	for i, c := range key {
		key[i] = upper(c)
	}
	return string(key)
}

// checkPaths refuses the first file whose name is the archive path of a file before it.
func checkPaths(files []File) error {
	seen := map[string]string{}
	for _, file := range files {
		key := pathKey(file.Name)
		if earlier, taken := seen[key]; taken {
			return errDuplicatePath(file.Name, earlier)
		}
		seen[key] = file.Name
	}
	return nil
}

// withListfile returns what the archive holds: the files in their order without any (listfile) among them, then
// a (listfile) that names each on a line of its own.
func withListfile(files []File) []File {
	listfileKey := pathKey(listfileName)
	entries := make([]File, 0, len(files)+1)
	var names strings.Builder
	for _, file := range files {
		if pathKey(file.Name) == listfileKey {
			continue
		}
		entries = append(entries, file)
		names.WriteString(file.Name + "\r\n")
	}
	return append(entries, File{Name: listfileName, Data: []byte(names.String())})
}

// header is what the archive's header says besides its fixed fields. The positions are counted from the header's
// first byte.
type header struct {
	archiveSize  int // from the header to the end of the block table
	sectorShift  int
	hashTableAt  int
	blockTableAt int
	hashSlots    int
	blocks       int
}

// writeHeader writes the 32 bytes an archive starts with.
func writeHeader(w *binio.Writer, h header) {
	w.U32(magic)
	w.U32(headerSize)
	w.U32(uint32(h.archiveSize))
	w.U16(formatVersion)
	w.U16(uint16(h.sectorShift))
	w.U32(uint32(h.hashTableAt))
	w.U32(uint32(h.blockTableAt))
	w.U32(uint32(h.hashSlots))
	w.U32(uint32(h.blocks))
}

// storeFiles returns the data of every entry as the archive holds it, one after another, and the block table: for
// each entry, in order, where its data starts, how long it is stored and in plain, and its flags.
func storeFiles(entries []File, sectorSize int) (data []byte, blocks []uint32, err error) {
	packer := newSectorPacker()
	for _, entry := range entries {
		stored, err := storeFile(packer, entry.Data, sectorSize)
		if err != nil {
			return nil, nil, err
		}
		flags := uint32(flagExists)
		if len(entry.Data) > 0 {
			flags |= flagCompressed
		}
		blocks = append(blocks, uint32(headerSize+len(data)), uint32(len(stored)), uint32(len(entry.Data)), flags)
		data = append(data, stored...)
	}
	return data, blocks, nil
}

// storeFile returns a file as the archive holds it: cut into sectors, with a table before them that says where
// each sector starts and where the last one ends, counted from the table's first byte. An empty file is stored
// as nothing at all.
func storeFile(packer *sectorPacker, data []byte, sectorSize int) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	sectorCount := (len(data) + sectorSize - 1) / sectorSize
	tableSize := (sectorCount + 1) * wordSize
	var table binio.Writer
	var sectors []byte
	for raw := range slices.Chunk(data, sectorSize) {
		sector, err := packer.sector(raw)
		if err != nil {
			return nil, err
		}
		table.U32(uint32(tableSize + len(sectors)))
		sectors = append(sectors, sector...)
	}
	table.U32(uint32(tableSize + len(sectors)))
	table.Write(sectors)
	return table.Bytes(), nil
}

// sectorPacker compresses the sectors of an archive, one after another. It keeps one compressor for all of them:
// to make a compressor costs several times what it costs to compress a sector with it. Each sector is compressed
// on its own all the same, from a compressor that remembers nothing of the sector before.
type sectorPacker struct {
	packed     bytes.Buffer
	compressor *zlib.Writer
}

func newSectorPacker() *sectorPacker {
	p := &sectorPacker{}
	p.compressor = zlib.NewWriter(&p.packed)
	return p
}

// sector returns a sector as the archive holds it: compressed with zlib behind the byte that says so, or as it is
// when that would not be shorter. A reader tells the two apart by the length. The bytes returned are the packer's
// own until its next sector.
func (p *sectorPacker) sector(raw []byte) ([]byte, error) {
	p.packed.Reset()
	p.packed.WriteByte(compressedWithZlib)
	p.compressor.Reset(&p.packed)
	if _, err := p.compressor.Write(raw); err != nil {
		return nil, err
	}
	if err := p.compressor.Close(); err != nil {
		return nil, err
	}
	if p.packed.Len() < len(raw) {
		return p.packed.Bytes(), nil
	}
	return raw, nil
}

// hashTable returns the table the game finds a file in by its name. The slot of an entry holds two hashes of the
// name, the locale and platform, and the entry's index in the block table.
func hashTable(entries []File) []uint32 {
	slots := hashSlots(len(entries))
	table := slices.Repeat([]uint32{free}, slots*entryWords)
	for index, entry := range entries {
		slot := freeSlot(table, entry.Name)
		copy(table[slot*entryWords:], []uint32{
			HashString(entry.Name, NameA), HashString(entry.Name, NameB), neutral, uint32(index),
		})
	}
	return table
}

// hashSlots is the size of the hash table for count entries: the smallest that leaves a third of its slots free,
// so that a search for a name ends soon.
func hashSlots(count int) int {
	slots := minHashSlots
	for slots*2 < count*3 {
		slots *= 2
	}
	return slots
}

// freeSlot is the slot for name: the one its hash points at, or the next free one after it, going round at the
// end of the table. That is the order a reader searches in.
func freeSlot(table []uint32, name string) int {
	mask := len(table)/entryWords - 1 // the number of slots is a power of two
	slot := int(HashString(name, TableOffset)) & mask
	for table[slot*entryWords+3] != free {
		slot = (slot + 1) & mask
	}
	return slot
}

// writeTable writes a table's words, encrypted with key.
func writeTable(w *binio.Writer, table []uint32, key uint32) {
	EncryptBlock(table, key)
	for _, word := range table {
		w.U32(word)
	}
}

// ---- errors ----

// errPrefixLength says that the archive would not start where a reader looks for it.
func errPrefixLength() error {
	return &diag.Error{Msg: "The archive prefix must be a multiple of 512 bytes."}
}

// errDuplicatePath says that two of the files would be one file of the archive.
func errDuplicatePath(name, earlier string) error {
	return &diag.Error{
		Msg:  "Duplicate archive path '" + name + "' (also '" + earlier + "').",
		Hint: "Archive paths are case-insensitive; rename one of the files.",
	}
}
