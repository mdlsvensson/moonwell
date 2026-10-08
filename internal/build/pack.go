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

const infoName = "war3map.w3i"

var archiveMetadata = map[string]bool{"(attributes)": true, "(listfile)": true, "(signature)": true}

func packMap(view *mapdir.Folder, name string) ([]byte, error) {
	format, err := readInfoHeader(view)
	if err != nil {
		return nil, err
	}
	files, err := archiveFiles(view)
	if err != nil {
		return nil, err
	}
	var options mpq.Options
	if !format.IsHeaderless() {
		options.Prefix = mpq.HM3WHeader(name, 0, 0)
	}
	if tooLarge, fits := mpq.CheckFits(len(options.Prefix), files); !fits {
		return nil, errTooLarge(view, tooLarge)
	}
	archive, err := mpq.Write(files, options)
	if err != nil {
		return nil, withFile(err, view.DisplayPath(""))
	}
	return archive, nil
}

func readInfoHeader(view *mapdir.Folder) (w3i.Header, error) {
	info, found, err := view.Read(infoName)
	switch {
	case err != nil:
		return w3i.Header{}, err
	case !found:
		return w3i.Header{}, errNoMapInfo(view.DisplayPath(""))
	}
	return w3i.ReadHeader(info, view.DisplayPath(infoName))
}

func archiveFiles(view *mapdir.Folder) ([]mpq.File, error) {
	var files []mpq.File
	for _, name := range view.Files() {
		if archiveMetadata[toLowerASCII(name)] {
			continue
		}
		data, found, err := view.Read(name)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, fmt.Errorf("build.pack: the planned map lists %s and has no such file", name)
		}
		files = append(files, mpq.File{Name: strings.ReplaceAll(name, "/", `\`), Data: data})
	}
	return files, nil
}

func withFile(err error, file string) error {
	var failure *diag.Error
	if !errors.As(err, &failure) {
		return err
	}
	withFile := *failure
	withFile.File = file
	return &withFile
}

func errNoMapInfo(mapLabel string) error {
	return &diag.Error{
		Msg:  infoName + " is missing from the map folder.",
		File: mapLabel,
		Hint: "Save the source map from World Editor in folder format.",
	}
}

func errTooLarge(view *mapdir.Folder, file string) error {
	limit := strconv.FormatInt(mpq.MaxSize, 10) + " bytes"
	if file == "" {
		return &diag.Error{
			Msg:  "The map is too large to pack: an archive holds at most " + limit + ".",
			File: view.DisplayPath(""),
			Hint: "Take files out of the map or out of assets/.",
		}
	}
	inMap := view.CanonicalPath(strings.ReplaceAll(file, `\`, "/"))
	return &diag.Error{
		Msg:  inMap + " is too large to pack: a file of an archive holds at most " + limit + ".",
		File: view.DisplayPath(inMap),
		Hint: "Take the file out of the map or out of assets/, or make it smaller.",
	}
}
