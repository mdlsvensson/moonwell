package objects

import (
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
)

type Result struct {
	Changes []mapdir.Change
	Objects []Resolved
	IDs     string
}

func Plan(source *mapdir.Folder, objects manifest.Objects, metadata *Metadata) (*Result, error) {
	if objects.IsEmpty() {
		return &Result{Objects: []Resolved{}, IDs: noIDs}, nil
	}
	files, err := readObjectFiles(source)
	if err != nil {
		return nil, err
	}
	resolved, err := Resolve(metadata, objects, files.customIDs())
	if err != nil {
		return nil, err
	}
	changes, err := files.planChanges(resolved)
	if err != nil {
		return nil, err
	}
	ids, err := RenderIDs(resolved)
	if err != nil {
		return nil, err
	}
	return &Result{Changes: changes, Objects: resolved, IDs: ids}, nil
}
