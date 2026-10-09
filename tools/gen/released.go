package main

import (
	"errors"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/objects"
)

func checkReleasedNamesKept(metadata *objects.Metadata, checkout string, overrides nameOverrides) error {
	data, found, err := fsx.ReadFileIfExists(pathIn(checkout, metadataPath))
	switch {
	case err != nil:
		return errInCheckout(checkout, metadataPath, err)
	case !found:
		return nil
	}
	released, err := decodeMetadata(data)
	if err != nil {
		return err
	}
	var problems []string
	for _, list := range objects.FieldLists {
		problems = append(problems, changedNames(list, released.Fields[list], metadata.Fields[list], overrides)...)
	}
	if len(problems) > 0 {
		return errReleasedNames(problems)
	}
	return nil
}

func changedNames(list string, released, current []objects.FieldMeta, overrides nameOverrides) []string {
	names := map[string]string{}
	for _, field := range current {
		names[field.ID] = field.Name
	}
	var problems []string
	for _, field := range released {
		name, ok := names[field.ID]
		pinned, _ := overrides.pinnedName(list, field.ID)
		switch {
		case !ok && !slices.Contains(overrides.Removed[list], displayRawcode(field.ID)):
			problems = append(problems, describeRemoval(list, field))
		case ok && name != field.Name && pinned != name:
			problems = append(problems, describeRename(list, field, name))
		}
	}
	return problems
}

func describeRemoval(list string, released objects.FieldMeta) string {
	return describeNamedField(list, released) + " would disappear"
}

func describeRename(list string, released objects.FieldMeta, name string) string {
	return describeNamedField(list, released) + ` would become "` + name + `"`
}

func errReleasedNames(problems []string) error {
	return errors.New("released friendly names would change. In " + overridesPath + ", pin a name that would " +
		`become another under "names" (the released name to keep it, the new one to change it on purpose), and ` +
		`list a field that would disappear under "removed":` + "\n  " + strings.Join(problems, "\n  "))
}
