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
// which order, with which header, and the refusal of a map that is larger than an archive can be. How large that
// is, war3/mpq knows.

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
	if tooLarge, fits := mpq.RoomFor(len(options.Prefix), files); !fits {
		return nil, errTooLarge(view, tooLarge)
	}
	archive, err := mpq.Write(files, options)
	if err != nil {
		return nil, named(err, view.Label(""))
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
	return w3i.ReadHeader(info, view.Label(infoName))
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

// named is a refusal of the archive's writer, which knows no file, with the file it is about. Any other error
// stays as it is.
func named(err error, file string) error {
	var failure *diag.Error
	if !errors.As(err, &failure) {
		return err
	}
	withFile := *failure
	withFile.File = file
	return &withFile
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
	limit := strconv.FormatInt(mpq.MaxSize, 10) + " bytes"
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
