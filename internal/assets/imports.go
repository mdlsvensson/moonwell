// Package assets imports the files under a project's assets/ folder, and those its libraries ship, into a map:
// it resolves each file's in-map path, keeps war3map.imp (World Editor's import index) in step, and for
// assets:sync writes into the source map and records which files it owns.
package assets

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"slices"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/text"
)

// Import is one entry of war3map.imp (version 1), World Editor's import index.
type Import struct {
	// Flag 0, 5 or 8: the file lives under war3mapImported\. Flag 10, 13 or 29: Path is the full in-map path.
	// World Editor 3.00 saves custom-path imports as 29 (13 plus the undocumented bit 0x10).
	Flag uint8
	Path string
}

var (
	customPathFlags = []uint8{10, 13, 29}
	importFlags     = []uint8{0, 5, 8, 10, 13, 29}
)

// ImportPath is the in-map path of an import.
func ImportPath(entry Import) string {
	if slices.Contains(customPathFlags, entry.Flag) {
		return entry.Path
	}
	return `war3mapImported\` + entry.Path
}

// ReadImports reads a war3map.imp. file is the name errors give.
func ReadImports(data []byte, file string) ([]Import, error) {
	corrupt := func(problem string) ([]Import, error) {
		return nil, &diag.Error{
			Msg:  "war3map.imp is unreadable: " + problem + ".",
			File: file,
			Hint: "Open and re-save the map in World Editor.",
		}
	}
	if len(data) < 8 {
		return corrupt("it is truncated")
	}
	if version := binary.LittleEndian.Uint32(data); version != 1 {
		return corrupt(fmt.Sprintf("version %d is not supported (expected 1)", version))
	}
	count := binary.LittleEndian.Uint32(data[4:])
	entries := []Import{}
	offset := 8
	for i := range count {
		if offset >= len(data) {
			return corrupt("it is truncated")
		}
		flag := data[offset]
		offset++
		if !slices.Contains(importFlags, flag) {
			return corrupt(fmt.Sprintf("entry %d has unknown flag %d", i, flag))
		}
		length := bytes.IndexByte(data[offset:], 0)
		if length < 0 {
			return corrupt("it is truncated")
		}
		if length == 0 {
			return corrupt(fmt.Sprintf("entry %d has an empty path", i))
		}
		path, ok := text.Strict(data[offset : offset+length])
		if !ok {
			return corrupt(fmt.Sprintf("entry %d is not valid UTF-8", i))
		}
		entries = append(entries, Import{Flag: flag, Path: path})
		offset += length + 1
	}
	if offset != len(data) {
		return corrupt("it has trailing data")
	}
	return entries, nil
}

// WriteImports writes a war3map.imp.
func WriteImports(entries []Import) []byte {
	out := binary.LittleEndian.AppendUint32(nil, 1)
	out = binary.LittleEndian.AppendUint32(out, uint32(len(entries)))
	for _, entry := range entries {
		out = append(out, entry.Flag)
		out = append(out, entry.Path...)
		out = append(out, 0)
	}
	return out
}
