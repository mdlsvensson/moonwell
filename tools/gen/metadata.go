package main

import (
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/objects"
)

func writeMetadata(checkout string, args []string, out io.Writer) error {
	exportDir, version := args[0], args[1]
	overrides, err := readOverrides(checkout)
	if err != nil {
		return err
	}
	game, err := readExport(exportDir)
	if err != nil {
		return err
	}
	fields, nameChanges, err := buildNamedFields(game, overrides)
	if err != nil {
		return err
	}
	bases, err := buildBases(game)
	if err != nil {
		return err
	}
	metadata := &objects.Metadata{Format: 1, Game: version, Fields: fields, Bases: bases}
	if err := checkReleasedNamesKept(metadata, checkout, overrides); err != nil {
		return err
	}
	if err := os.WriteFile(pathIn(checkout, metadataPath), []byte(renderMetadata(metadata)), 0o666); err != nil {
		return errInCheckout(checkout, metadataPath, err)
	}
	reportCounts(out, metadata)
	reportNameChanges(out, nameChanges)
	fmt.Fprintln(out, "wrote "+metadataPath+". Now run `"+commandLine+"`.")
	return nil
}

func reportCounts(out io.Writer, metadata *objects.Metadata) {
	var fields, bases []string
	for _, list := range objects.FieldLists {
		fields = append(fields, fsx.QuoteJSON(list)+":"+strconv.Itoa(len(metadata.Fields[list])))
	}
	for _, category := range manifest.Categories {
		bases = append(bases, fsx.QuoteJSON(string(category))+":"+strconv.Itoa(len(metadata.Bases[category])))
	}
	fmt.Fprintln(out, "fields: {"+strings.Join(fields, ",")+"}")
	fmt.Fprintln(out, "bases: {"+strings.Join(bases, ",")+"}")
}

func reportNameChanges(out io.Writer, nameChanges []nameChange) {
	fmt.Fprintln(out, "renamed ("+strconv.Itoa(len(nameChanges))+"):")
	slices.SortStableFunc(nameChanges, func(a, b nameChange) int {
		byList := slices.Index(objects.FieldLists, a.list) - slices.Index(objects.FieldLists, b.list)
		if byList != 0 {
			return byList
		}
		return strings.Compare(a.id, b.id)
	})
	for _, change := range nameChanges {
		fmt.Fprintln(out, "  "+change.list+" "+change.id+" "+change.description)
	}
}

func renderMetadata(metadata *objects.Metadata) string {
	var fields, bases []string
	for _, list := range objects.FieldLists {
		var entries []string
		for _, field := range metadata.Fields[list] {
			entries = append(entries, renderField(field))
		}
		fields = append(fields, "    "+fsx.QuoteJSON(list)+": "+entriesBlock("[", entries, "]"))
	}
	for _, category := range manifest.Categories {
		var entries []string
		for _, id := range slices.Sorted(maps.Keys(metadata.Bases[category])) {
			entries = append(entries, fsx.QuoteJSON(id)+": "+renderBase(metadata.Bases[category][id]))
		}
		bases = append(bases, "    "+fsx.QuoteJSON(string(category))+": "+entriesBlock("{", entries, "}"))
	}
	return strings.Join([]string{
		"{",
		`  "format": ` + strconv.Itoa(metadata.Format) + ",",
		`  "game": ` + fsx.QuoteJSON(metadata.Game) + ",",
		"  \"fields\": {\n" + strings.Join(fields, ",\n") + "\n  },",
		"  \"bases\": {\n" + strings.Join(bases, ",\n") + "\n  }",
		"}",
		"",
	}, "\n")
}

func entriesBlock(opening string, entries []string, closing string) string {
	if len(entries) == 0 {
		return opening + closing
	}
	return opening + "\n      " + strings.Join(entries, ",\n      ") + "\n    " + closing
}

func renderField(field objects.FieldMeta) string {
	return `{"id":` + fsx.QuoteJSON(field.ID) +
		`,"name":` + fsx.QuoteJSON(field.Name) +
		`,"label":` + fsx.QuoteJSON(field.Label) +
		`,"category":` + fsx.QuoteJSON(field.Category) +
		`,"type":` + fsx.QuoteJSON(field.Type) +
		`,"storage":` + fsx.QuoteJSON(field.Storage) +
		`,"list":` + strconv.FormatBool(field.List) +
		`,"perLevel":` + strconv.FormatBool(field.PerLevel) +
		`,"column":` + strconv.Itoa(field.Column) +
		`,"skin":` + strconv.FormatBool(field.Skin) +
		`,"use":` + renderTexts(field.Use) +
		`,"specific":` + renderTexts(field.Specific) +
		`,"notSpecific":` + renderTexts(field.NotSpecific) + "}"
}

func renderBase(base objects.BaseMeta) string {
	if base.Levels == nil {
		return `{"name":` + fsx.QuoteJSON(base.Name) + "}"
	}
	return `{"name":` + fsx.QuoteJSON(base.Name) + `,"levels":` + strconv.Itoa(*base.Levels) + "}"
}

func renderTexts(texts []string) string {
	quoted := make([]string, len(texts))
	for i, text := range texts {
		quoted[i] = fsx.QuoteJSON(text)
	}
	return "[" + strings.Join(quoted, ",") + "]"
}
