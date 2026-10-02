package testkit

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/mpq"
)

// MPQ is an opened MPQ format-1 archive: enough of a reader to verify what Moonwell writes.
type MPQ struct {
	// HeaderOffset is where the archive starts: 512 after an HM3W header, else 0.
	HeaderOffset int
	// Blocks is the number of files, the (listfile) included.
	Blocks int

	data       []byte
	sectorSize int
	hashSize   int
	hashes     []uint32
	blocks     []uint32
}

// OpenMPQ finds the archive in data and decrypts its tables.
func OpenMPQ(data []byte) (*MPQ, error) {
	le := binary.LittleEndian
	base := -1
	for offset := 0; offset+32 <= len(data); offset += 512 {
		if le.Uint32(data[offset:]) == 0x1a51504d {
			base = offset
			break
		}
	}
	if base < 0 {
		return nil, errors.New("no MPQ header found")
	}
	header := data[base : base+32]
	archive := &MPQ{
		HeaderOffset: base,
		Blocks:       int(le.Uint32(header[28:])),
		data:         data,
		sectorSize:   512 << le.Uint16(header[14:]),
		hashSize:     int(le.Uint32(header[24:])),
	}
	table := func(position uint32, count int, key uint32) []uint32 {
		words := make([]uint32, count)
		for i := range words {
			words[i] = le.Uint32(data[base+int(position)+i*4:])
		}
		mpq.DecryptBlock(words, key)
		return words
	}
	archive.hashes = table(le.Uint32(header[16:]), archive.hashSize*4, mpq.HashTableKey)
	archive.blocks = table(le.Uint32(header[20:]), archive.Blocks*4, mpq.BlockTableKey)
	return archive, nil
}

func (m *MPQ) find(name string) (int, bool) {
	a, b := mpq.HashString(name, mpq.NameA), mpq.HashString(name, mpq.NameB)
	slot := int(mpq.HashString(name, mpq.TableOffset)) & (m.hashSize - 1)
	for range m.hashSize {
		index := m.hashes[slot*4+3]
		if index == 0xffffffff {
			return 0, false
		}
		if m.hashes[slot*4] == a && m.hashes[slot*4+1] == b {
			return int(index), true
		}
		slot = (slot + 1) & (m.hashSize - 1)
	}
	return 0, false
}

// Read returns the content of the file name; false when the archive has no such file.
func (m *MPQ) Read(name string) ([]byte, bool, error) {
	index, ok := m.find(name)
	if !ok {
		return nil, false, nil
	}
	le := binary.LittleEndian
	start := m.HeaderOffset + int(m.blocks[index*4])
	size := int(m.blocks[index*4+2])
	flags := m.blocks[index*4+3]
	if size == 0 {
		return []byte{}, true, nil
	}
	if flags&0x200 == 0 {
		return m.data[start : start+size], true, nil
	}
	count := (size + m.sectorSize - 1) / m.sectorSize
	out := make([]byte, 0, size)
	for i := range count {
		chunk := m.data[start+int(le.Uint32(m.data[start+i*4:])) : start+int(le.Uint32(m.data[start+(i+1)*4:]))]
		expected := min(m.sectorSize, size-i*m.sectorSize)
		if len(chunk) >= expected {
			out = append(out, chunk...)
			continue
		}
		if chunk[0] != 0x02 {
			return nil, true, fmt.Errorf("unsupported compression mask %d", chunk[0])
		}
		reader, err := zlib.NewReader(bytes.NewReader(chunk[1:]))
		if err != nil {
			return nil, true, err
		}
		sector, err := io.ReadAll(reader)
		if err != nil {
			return nil, true, err
		}
		out = append(out, sector...)
	}
	return out, true, nil
}

// Listfile returns the names in the archive's (listfile), in order.
func (m *MPQ) Listfile() ([]string, error) {
	data, ok, err := m.Read("(listfile)")
	if err != nil || !ok {
		return nil, err
	}
	var names []string
	for _, name := range strings.Split(string(data), "\r\n") {
		if name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}
