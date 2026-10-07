package build

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
	"github.com/mdlsvensson/moonwell/internal/war3/mpq"
	"github.com/mdlsvensson/moonwell/internal/war3/w3i"
)

// This file holds the packing of a planned map: which of its files an archive holds, under which names and in
// which order, with which header, and how large an archive can be.

// infoName is the file of a map that says which format the map has.
const infoName = "war3map.w3i"

// archiveMetadata is the files an archive keeps about itself, which a map saved as a folder may hold from the
// archive it once was: they describe that archive and are never packed. The names are in lower case; an archive
// finds a file by its name in any letter case of ASCII.
var archiveMetadata = map[string]bool{"(attributes)": true, "(listfile)": true, "(signature)": true}

// pack is the planned map as an archive; name is the map's name, for the header of a map that has one.
//
// The archive is packed from the view and not read back from the stage. Both hold the same bytes as long as
// nothing writes the source map between the staging and the packing: the stage copies the map's files when it
// is written, and the archive reads them when it is packed.
//
// The archive's files are in the order of Folder.Files: the files the map has on disk, each folder's entries by
// the bytes of their names with a folder's own entries where the folder stands, and then the files the plan
// adds, in the order they were first planned. A map of a format that is packed without a header gets none, and
// any other gets the HM3W header before its archive.
func pack(view *mapdir.Folder, name string) ([]byte, error) {
	format, err := formatOf(view)
	if err != nil {
		return nil, err
	}
	files, err := archiveFiles(view)
	if err != nil {
		return nil, err
	}
	var options mpq.Options
	if !format.Headerless() {
		options.Prefix = mpq.HM3WHeader(name, 0, 0)
	}
	if tooLarge, fits := roomFor(int64(len(options.Prefix)), sizesOf(files)); !fits {
		return nil, errTooLarge(view, tooLarge)
	}
	archive, err := mpq.Write(files, options)
	if err != nil {
		return nil, named(err, view.Label(""), "Check the files of the source map, then try again.")
	}
	return archive, nil
}

// formatOf is the start of the map's war3map.w3i, which says how the map is packed.
func formatOf(view *mapdir.Folder) (w3i.Header, error) {
	info, found, err := view.Read(infoName)
	switch {
	case err != nil:
		return w3i.Header{}, err
	case !found:
		return w3i.Header{}, errNoMapInfo(view.Label(""))
	}
	format, err := w3i.ReadHeader(info)
	if err != nil {
		return w3i.Header{}, named(err, view.Label(infoName), "Save the map again in World Editor.")
	}
	return format, nil
}

// archiveFiles is the files of the view as an archive names them, with "\" for "/", in the view's order and
// without the files an archive keeps about itself.
func archiveFiles(view *mapdir.Folder) ([]mpq.File, error) {
	var files []mpq.File
	for _, name := range view.Files() {
		if archiveMetadata[lowerASCII(name)] {
			continue
		}
		data, found, err := view.Read(name)
		if err != nil {
			return nil, err
		}
		if !found {
			// A plain error: Files lists the files the view has, and Read finds each of them, so a file that is
			// listed and not found is a mistake in Moonwell and nothing the user can put right.
			return nil, fmt.Errorf("build.pack: the planned map lists %s and has no such file", name)
		}
		files = append(files, mpq.File{Name: strings.ReplaceAll(name, "/", `\`), Data: data})
	}
	return files, nil
}

// named is a failure of a package that knows no file, with the file it is about and how to put it right: an
// expected failure gets the file, and the hint where it has none. Any other error stays as it is.
func named(err error, file, hint string) error {
	var failure *diag.Error
	if !errors.As(err, &failure) {
		return err
	}
	withFile := *failure
	withFile.File = file
	if withFile.Hint == "" {
		withFile.Hint = hint
	}
	return &withFile
}

// ---- the size an archive can have ----

// An archive as war3/mpq lays one out: a header, the data of each file and of the list of the files, the hash
// table and the block table. A file's data is its bytes in sectors, behind a word for where each sector starts
// and one for where the last ends.
const (
	archiveHeader = 32   // the header, in bytes
	sectorBytes   = 4096 // a sector of a file, before it is compressed
	wordBytes     = 4    // a position or a size
	entryBytes    = 16   // a slot of the hash table, and a block of the block table
	fewestSlots   = 16   // the smallest hash table
	// largestWord is the largest number a word holds: the format writes every position and size as one.
	largestWord = 1<<32 - 1
)

// sized is a file of an archive by its name there and the number of its bytes.
type sized struct {
	name string
	size int64
}

// sizesOf is the files by their sizes, in their order.
func sizesOf(files []mpq.File) []sized {
	sizes := make([]sized, len(files))
	for at, file := range files {
		sizes[at] = sized{name: file.Name, size: int64(len(file.Data))}
	}
	return sizes
}

// roomFor reports whether the format can hold an archive of the files behind a prefix of so many bytes. Where it
// cannot, tooLarge is the first file that is too large by itself, and "" where the files are too large together.
//
// The format writes as a word: the size of each file; where each file's data starts and how long it is, counted
// from the archive's header; where the two tables start; and the archive's size, from its header to the end of
// the block table, which is the largest of them. So a file must not be larger than a word holds, and the archive
// must not be: it is counted with its prefix, the header of 512 bytes that a map of an older format has, since
// a reader finds a position by adding where the archive starts, and holds the sum in a word too. The archive
// is counted at its largest, with no sector compressed, so the answer depends on the sizes alone.
func roomFor(prefix int64, files []sized) (tooLarge string, fits bool) {
	for _, file := range files {
		if file.size > largestWord {
			return file.name, false
		}
	}
	return "", largestArchive(prefix, files) <= largestWord
}

// largestArchive is the most bytes an archive of the files takes, with its prefix: the bytes it takes when no
// sector of it is stored shorter than it is.
func largestArchive(prefix int64, files []sized) int64 {
	total := prefix + archiveHeader
	var list int64
	for _, file := range files {
		total += largestStored(file.size)
		list += int64(len(file.name)) + 2 // the file's line of the list: its name and "\r\n"
	}
	entries := int64(len(files)) + 1 // the files, and the list of them
	return total + largestStored(list) + (hashSlots(entries)+entries)*entryBytes
}

// largestStored is the most bytes the data of a file of size bytes takes in an archive: its bytes, and a word
// for each sector and one more. A file without a byte takes none.
func largestStored(size int64) int64 {
	if size == 0 {
		return 0
	}
	sectors := (size + sectorBytes - 1) / sectorBytes
	return size + (sectors+1)*wordBytes
}

// hashSlots is the size of the hash table of an archive with so many entries: the smallest power of two, from
// sixteen, that leaves a third of its slots free.
func hashSlots(entries int64) int64 {
	slots := int64(fewestSlots)
	for slots*2 < entries*3 {
		slots *= 2
	}
	return slots
}

// ---- errors ----

func errNoMapInfo(mapLabel string) error {
	return &diag.Error{
		Msg:  infoName + " is missing from the map folder.",
		File: mapLabel,
		Hint: "Save the source map from World Editor in folder format.",
	}
}

// errTooLarge is the refusal of a map the format cannot hold: file is the file of the archive that is too large
// by itself, by its name there, and "" where the map is too large as a whole.
func errTooLarge(view *mapdir.Folder, file string) error {
	limit := strconv.FormatInt(largestWord, 10) + " bytes"
	if file == "" {
		return &diag.Error{
			Msg:  "The map is too large to pack: an archive holds at most " + limit + ".",
			File: view.Label(""),
			Hint: "Take files out of the map or out of assets/.",
		}
	}
	inMap := view.Name(strings.ReplaceAll(file, `\`, "/"))
	return &diag.Error{
		Msg:  inMap + " is too large to pack: a file of an archive holds at most " + limit + ".",
		File: view.Label(inMap),
		Hint: "Take the file out of the map or out of assets/, or make it smaller.",
	}
}
