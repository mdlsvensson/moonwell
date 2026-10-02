package mpq

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/text"
)

// File is one file of an archive.
type File struct {
	// Name is the archive path with backslash separators, such as `war3mapImported\icon.blp`.
	Name string
	Data []byte
}

// Options change how an archive is written.
type Options struct {
	// Prefix is put before the archive; its length must be a multiple of 512. A map's legacy HM3W header goes here.
	Prefix []byte
	// SectorSizeShift sets the sector size to 512 << shift. Zero means 3: sectors of 4096 bytes.
	SectorSizeShift int
}

const (
	headerSize   = 32
	magic        = 0x1a51504d // "MPQ\x1A"
	fileExists   = 0x80000000
	fileCompress = 0x00000200
	empty        = 0xffffffff
)

// Write builds an MPQ format-1 archive of files, in their order, followed by a (listfile) that names them.
func Write(files []File, options Options) ([]byte, error) {
	if len(options.Prefix)%512 != 0 {
		return nil, &diag.Error{Msg: "The archive prefix must be a multiple of 512 bytes."}
	}
	shift := options.SectorSizeShift
	if shift == 0 {
		shift = 3
	}
	sectorSize := 512 << shift

	seen := map[string]string{}
	for _, file := range files {
		key := text.Upper(file.Name)
		if previous, ok := seen[key]; ok {
			return nil, &diag.Error{
				Msg:  "Duplicate archive path '" + file.Name + "' (also '" + previous + "').",
				Hint: "Archive paths are case-insensitive; rename one of the files.",
			}
		}
		seen[key] = file.Name
	}
	var entries []File
	var listfile strings.Builder
	for _, file := range files {
		if text.Upper(file.Name) == "(LISTFILE)" {
			continue
		}
		entries = append(entries, file)
		listfile.WriteString(file.Name)
		listfile.WriteString("\r\n")
	}
	entries = append(entries, File{Name: "(listfile)", Data: []byte(listfile.String())})

	var body bytes.Buffer
	blocks := make([]uint32, len(entries)*4)
	offset := headerSize
	for i, entry := range entries {
		encoded, err := encodeFile(entry.Data, sectorSize)
		if err != nil {
			return nil, err
		}
		blocks[i*4] = uint32(offset)
		blocks[i*4+1] = uint32(len(encoded))
		blocks[i*4+2] = uint32(len(entry.Data))
		blocks[i*4+3] = fileExists
		if len(entry.Data) > 0 {
			blocks[i*4+3] |= fileCompress
		}
		body.Write(encoded)
		offset += len(encoded)
	}

	hashSize := 16
	for float64(hashSize) < float64(len(entries))*1.5 {
		hashSize *= 2
	}
	hashes := make([]uint32, hashSize*4)
	for i := range hashes {
		hashes[i] = empty
	}
	for index, entry := range entries {
		slot := int(HashString(entry.Name, TableOffset)) & (hashSize - 1)
		for hashes[slot*4+3] != empty {
			slot = (slot + 1) & (hashSize - 1)
		}
		hashes[slot*4] = HashString(entry.Name, NameA)
		hashes[slot*4+1] = HashString(entry.Name, NameB)
		hashes[slot*4+2] = 0 // locale 0 (neutral), platform 0
		hashes[slot*4+3] = uint32(index)
	}
	EncryptBlock(hashes, HashTableKey)
	EncryptBlock(blocks, BlockTableKey)

	hashPosition := offset
	blockPosition := hashPosition + hashSize*16
	archiveSize := blockPosition + len(entries)*16

	le := binary.LittleEndian
	out := make([]byte, 0, len(options.Prefix)+archiveSize)
	out = append(out, options.Prefix...)
	out = le.AppendUint32(out, magic)
	out = le.AppendUint32(out, headerSize)
	out = le.AppendUint32(out, uint32(archiveSize))
	out = le.AppendUint16(out, 0) // format version 1
	out = le.AppendUint16(out, uint16(shift))
	out = le.AppendUint32(out, uint32(hashPosition))
	out = le.AppendUint32(out, uint32(blockPosition))
	out = le.AppendUint32(out, uint32(hashSize))
	out = le.AppendUint32(out, uint32(len(entries)))
	out = append(out, body.Bytes()...)
	for _, word := range hashes {
		out = le.AppendUint32(out, word)
	}
	for _, word := range blocks {
		out = le.AppendUint32(out, word)
	}
	return out, nil
}

// encodeFile writes a sector offset table followed by the sectors; each sector is zlib (mask 0x02) or raw when
// compression does not help.
func encodeFile(data []byte, sectorSize int) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	count := (len(data) + sectorSize - 1) / sectorSize
	sectors := make([][]byte, 0, count)
	for i := range count {
		raw := data[i*sectorSize : min(len(data), (i+1)*sectorSize)]
		var packed bytes.Buffer
		packed.WriteByte(0x02)
		compressor := zlib.NewWriter(&packed)
		if _, err := compressor.Write(raw); err != nil {
			return nil, err
		}
		if err := compressor.Close(); err != nil {
			return nil, err
		}
		if packed.Len() < len(raw) {
			sectors = append(sectors, packed.Bytes())
		} else {
			sectors = append(sectors, raw)
		}
	}
	tableSize := (count + 1) * 4
	out := make([]byte, tableSize, tableSize+len(data))
	position := tableSize
	for i, sector := range sectors {
		binary.LittleEndian.PutUint32(out[i*4:], uint32(position))
		out = append(out, sector...)
		position += len(sector)
	}
	binary.LittleEndian.PutUint32(out[count*4:], uint32(position))
	return out, nil
}
