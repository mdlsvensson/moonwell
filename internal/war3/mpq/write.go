package mpq

import (
	"bytes"
	"cmp"
	"compress/zlib"
	"fmt"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/binio"
	"github.com/mdlsvensson/moonwell/internal/diag"
)

type File struct {
	Name string
	Data []byte
}

type Options struct {
	Prefix          []byte
	SectorSizeShift int
}

const (
	alignment          = 512
	smallestSector     = 512
	defaultSectorShift = 3
	listfileName       = "(listfile)"
	listfileNewline    = "\r\n"

	magic         = 0x1A51504D
	headerSize    = 32
	formatVersion = 0

	wordSize   = 4
	entryWords = 4

	blockIndexWord = 3

	minHashSlots  = 16
	emptySlot     = 0xFFFFFFFF
	neutralLocale = 0

	flagExists     = 0x80000000
	flagCompressed = 0x00000200

	compressedWithZlib = 0x02
)

func Write(files []File, options Options) ([]byte, error) {
	if len(options.Prefix)%alignment != 0 {
		return nil, fmt.Errorf("mpq.Write: the prefix is %d bytes, which is no multiple of %d: a reader would "+
			"not find the archive", len(options.Prefix), alignment)
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
	hashTableOffset := headerSize + len(data)
	blockTableOffset := hashTableOffset + len(hashes)*wordSize

	var w binio.Writer
	w.Write(options.Prefix)
	writeHeader(&w, header{
		archiveSize:      blockTableOffset + len(blocks)*wordSize,
		sectorShift:      shift,
		hashTableOffset:  hashTableOffset,
		blockTableOffset: blockTableOffset,
		hashSlots:        len(hashes) / entryWords,
		blocks:           len(entries),
	})
	w.Write(data)
	writeTable(&w, hashes, HashTableKey)
	writeTable(&w, blocks, BlockTableKey)
	return w.Bytes(), nil
}

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

func withListfile(files []File) []File {
	listfileKey := pathKey(listfileName)
	entries := make([]File, 0, len(files)+1)
	var names strings.Builder
	for _, file := range files {
		if pathKey(file.Name) == listfileKey {
			continue
		}
		entries = append(entries, file)
		names.WriteString(file.Name + listfileNewline)
	}
	return append(entries, File{Name: listfileName, Data: []byte(names.String())})
}

func pathKey(name string) string {
	key := []byte(name)
	for i, c := range key {
		key[i] = toUpperASCII(c)
	}
	return string(key)
}

type header struct {
	archiveSize      int
	sectorShift      int
	hashTableOffset  int
	blockTableOffset int
	hashSlots        int
	blocks           int
}

func writeHeader(w *binio.Writer, h header) {
	w.U32(magic)
	w.U32(headerSize)
	w.U32(uint32(h.archiveSize))
	w.U16(formatVersion)
	w.U16(uint16(h.sectorShift))
	w.U32(uint32(h.hashTableOffset))
	w.U32(uint32(h.blockTableOffset))
	w.U32(uint32(h.hashSlots))
	w.U32(uint32(h.blocks))
}

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

func storeFile(packer *sectorPacker, data []byte, sectorSize int) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	sectorCount := (len(data) + sectorSize - 1) / sectorSize
	tableSize := (sectorCount + 1) * wordSize
	var table binio.Writer
	var sectors []byte
	for raw := range slices.Chunk(data, sectorSize) {
		sector, err := packer.pack(raw)
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

type sectorPacker struct {
	packed     bytes.Buffer
	compressor *zlib.Writer
}

func newSectorPacker() *sectorPacker {
	p := &sectorPacker{}
	p.compressor = zlib.NewWriter(&p.packed)
	return p
}

func (p *sectorPacker) pack(raw []byte) ([]byte, error) {
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

func hashTable(entries []File) []uint32 {
	slots := hashSlots(len(entries))
	table := slices.Repeat([]uint32{emptySlot}, slots*entryWords)
	for index, entry := range entries {
		slot := freeSlot(table, entry.Name)
		copy(table[slot*entryWords:], []uint32{
			HashString(entry.Name, NameA), HashString(entry.Name, NameB), neutralLocale, uint32(index),
		})
	}
	return table
}

func hashSlots(count int) int {
	slots := minHashSlots
	for slots*2 < count*3 {
		slots *= 2
	}
	return slots
}

func freeSlot(table []uint32, name string) int {
	mask := len(table)/entryWords - 1
	slot := int(HashString(name, TableOffset)) & mask
	for table[slot*entryWords+blockIndexWord] != emptySlot {
		slot = (slot + 1) & mask
	}
	return slot
}

func writeTable(w *binio.Writer, table []uint32, key uint32) {
	EncryptBlock(table, key)
	for _, word := range table {
		w.U32(word)
	}
}

func errDuplicatePath(name, earlier string) error {
	return &diag.Error{
		Msg:  "Duplicate archive path '" + name + "' (also '" + earlier + "').",
		Hint: "Archive paths are case-insensitive; rename one of the files.",
	}
}
