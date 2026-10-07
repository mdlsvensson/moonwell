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

// writeMetadata is the mode metadata: it writes data/metadata.json from an export of the game's object data,
// and prints how many fields and standard objects each category has and which friendly names are not the names
// of their labels. It writes nothing when the export lacks a file or a table lacks a column, a name needs a
// pin, a unit breaks the rule for heroes, or a released name would change.
func writeMetadata(checkout string, args []string, out io.Writer) error {
	folder, version := args[0], args[1]
	pins, err := readOverrides(checkout)
	if err != nil {
		return err
	}
	game, err := readExport(folder) // every table and text the metadata is made from
	if err != nil {
		return err
	}
	fields, renames, err := nameFields(game, pins)
	if err != nil {
		return err
	}
	bases, err := standardObjects(game)
	if err != nil {
		return err
	}
	metadata := &objects.Metadata{Format: 1, Game: version, Fields: fields, Bases: bases}
	if err := keepsReleasedNames(metadata, checkout, pins); err != nil {
		return err
	}
	return writeAndReport(checkout, metadata, renames, out)
}

// writeAndReport writes the metadata into the checkout and prints the report: how many fields each list has
// and how many standard objects each category, the renames, in the order of the lists and in each list in the
// order of the ids' bytes, and the command that is to be run next, which writes the schema from the file.
func writeAndReport(checkout string, metadata *objects.Metadata, renames []rename, out io.Writer) error {
	if err := os.WriteFile(fileIn(checkout, metadataPath), []byte(renderMetadata(metadata)), 0o666); err != nil {
		return errInCheckout(checkout, metadataPath, err)
	}
	var fields, bases []string
	for _, list := range objects.FieldLists {
		fields = append(fields, fsx.Quoted(list)+":"+strconv.Itoa(len(metadata.Fields[list])))
	}
	for _, category := range manifest.Categories {
		bases = append(bases, fsx.Quoted(string(category))+":"+strconv.Itoa(len(metadata.Bases[category])))
	}
	fmt.Fprintln(out, "fields: {"+strings.Join(fields, ",")+"}")
	fmt.Fprintln(out, "bases: {"+strings.Join(bases, ",")+"}")
	fmt.Fprintln(out, "renamed ("+strconv.Itoa(len(renames))+"):")
	slices.SortStableFunc(renames, func(a, b rename) int {
		byList := slices.Index(objects.FieldLists, a.list) - slices.Index(objects.FieldLists, b.list)
		if byList != 0 {
			return byList
		}
		return strings.Compare(a.id, b.id)
	})
	for _, renamed := range renames {
		fmt.Fprintln(out, "  "+renamed.list+" "+renamed.id+" "+renamed.change)
	}
	fmt.Fprintln(out, "wrote "+metadataPath+". Now run `"+commandLine+"`.")
	return nil
}

// ---- the text of the file ----

// renderMetadata is the text of data/metadata.json: one field or one standard object on a line, everything in a
// fixed order. The lists of fields stand in the order of objects.FieldLists, each in the order it has; the
// categories in their order, the objects of each in the order of the ids' bytes. A text is written as
// fsx.Quoted writes it.
func renderMetadata(metadata *objects.Metadata) string {
	var fields, bases []string
	for _, list := range objects.FieldLists {
		var entries []string
		for _, field := range metadata.Fields[list] {
			entries = append(entries, renderField(field))
		}
		fields = append(fields, "    "+fsx.Quoted(list)+": "+entriesBlock("[", entries, "]"))
	}
	for _, category := range manifest.Categories {
		var entries []string
		for _, id := range slices.Sorted(maps.Keys(metadata.Bases[category])) {
			entries = append(entries, fsx.Quoted(id)+": "+renderBase(metadata.Bases[category][id]))
		}
		bases = append(bases, "    "+fsx.Quoted(string(category))+": "+entriesBlock("{", entries, "}"))
	}
	return strings.Join([]string{
		"{",
		`  "format": ` + strconv.Itoa(metadata.Format) + ",",
		`  "game": ` + fsx.Quoted(metadata.Game) + ",",
		"  \"fields\": {\n" + strings.Join(fields, ",\n") + "\n  },",
		"  \"bases\": {\n" + strings.Join(bases, ",\n") + "\n  }",
		"}",
		"",
	}, "\n")
}

// entriesBlock writes the entries of a list of fields, or of the objects of a category, between its two
// brackets: each on a line of its own, six spaces in, and the closing bracket four spaces in. Without an entry
// it is the two brackets alone.
func entriesBlock(opening string, entries []string, closing string) string {
	if len(entries) == 0 {
		return opening + closing
	}
	return opening + "\n      " + strings.Join(entries, ",\n      ") + "\n    " + closing
}

// renderField writes a field as a JSON object on one line, without a space in it, its keys in the order of the
// file.
func renderField(field objects.FieldMeta) string {
	return `{"id":` + fsx.Quoted(field.ID) +
		`,"name":` + fsx.Quoted(field.Name) +
		`,"label":` + fsx.Quoted(field.Label) +
		`,"category":` + fsx.Quoted(field.Category) +
		`,"type":` + fsx.Quoted(field.Type) +
		`,"storage":` + fsx.Quoted(field.Storage) +
		`,"list":` + strconv.FormatBool(field.List) +
		`,"perLevel":` + strconv.FormatBool(field.PerLevel) +
		`,"column":` + strconv.Itoa(field.Column) +
		`,"skin":` + strconv.FormatBool(field.Skin) +
		`,"use":` + renderTexts(field.Use) +
		`,"specific":` + renderTexts(field.Specific) +
		`,"notSpecific":` + renderTexts(field.NotSpecific) + "}"
}

// renderBase writes a standard object as a JSON object on one line: its name, and its levels where it has a
// count of them.
func renderBase(base objects.BaseMeta) string {
	if base.Levels == nil {
		return `{"name":` + fsx.Quoted(base.Name) + "}"
	}
	return `{"name":` + fsx.Quoted(base.Name) + `,"levels":` + strconv.Itoa(*base.Levels) + "}"
}

// renderTexts writes a list of texts as a JSON list on one line.
func renderTexts(texts []string) string {
	quoted := make([]string, len(texts))
	for i, text := range texts {
		quoted[i] = fsx.Quoted(text)
	}
	return "[" + strings.Join(quoted, ",") + "]"
}
