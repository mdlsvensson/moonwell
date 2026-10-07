package main

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/objects"
)

// generatedFile is one file a mode renders: its path from the checkout, with "/", and its text.
type generatedFile struct{ path, text string }

// schemaModule is the Pkl module of a category of objects.
type schemaModule struct {
	category manifest.Category
	name     string // as in schema/objects/<name>.pkl
}

// schemaModules is the module of each category, in the order of the categories.
var schemaModules = []schemaModule{
	{"heroes", "Hero"}, {"units", "Unit"}, {"buildings", "Building"}, {"items", "Item"},
	{"abilities", "Ability"}, {"buffs", "Buff"}, {"upgrades", "Upgrade"},
}

// writeSchema is the mode without a name: it writes schema/generated/ from the object metadata of the checkout,
// and removes whatever else lies there, since the folder holds nothing but the schema. It prints a line for each
// file it wrote, which is none for a file that was as rendered, and for each entry it removed. It writes and
// removes nothing when a field's name cannot be a property.
func writeSchema(checkout string, _ []string, out io.Writer) error {
	metadata, err := readMetadata(checkout)
	if err != nil {
		return err
	}
	files, err := renderSchema(metadata)
	if err != nil {
		return err
	}
	if err := writeGenerated(checkout, files, out); err != nil {
		return err
	}
	return removeOthers(checkout, files, out)
}

// readMetadata reads the object metadata of a checkout: its data/metadata.json, and not the one this program
// was built with.
func readMetadata(checkout string) (*objects.Metadata, error) {
	data, err := os.ReadFile(fileIn(checkout, metadataPath))
	if err != nil {
		return nil, errInCheckout(checkout, metadataPath, err)
	}
	return decodeMetadata(data)
}

// decodeMetadata reads the text of a data/metadata.json. The bytes go to the decoder as they are.
func decodeMetadata(data []byte) (*objects.Metadata, error) {
	var metadata objects.Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, errNoJSON(metadataPath, err)
	}
	return &metadata, nil
}

// writeGenerated writes each file that the checkout does not hold as it is rendered, and prints a line for each
// it wrote.
func writeGenerated(checkout string, files []generatedFile, out io.Writer) error {
	for _, file := range files {
		wrote, err := fsx.WriteIfChanged(fileIn(checkout, file.path), file.text)
		if err != nil {
			return errInCheckout(checkout, file.path, err)
		}
		if wrote {
			fmt.Fprintln(out, "wrote "+file.path)
		}
	}
	return nil
}

// removeOthers removes what the folder of the schema holds beside the files, a folder with all that is below
// it, and prints a line for each entry it removed.
func removeOthers(checkout string, files []generatedFile, out io.Writer) error {
	entries, err := os.ReadDir(fileIn(checkout, schemaFolder))
	if err != nil {
		return errInCheckout(checkout, schemaFolder, err)
	}
	for _, entry := range entries {
		path := schemaFolder + "/" + entry.Name()
		if slices.ContainsFunc(files, func(file generatedFile) bool { return file.path == path }) {
			continue
		}
		if err := os.RemoveAll(fileIn(checkout, path)); err != nil {
			return errInCheckout(checkout, path, err)
		}
		fmt.Fprintln(out, "removed "+path)
	}
	return nil
}

// renderSchema renders schema/generated/<Name>Props.pkl for each category of objects. It fails for a name no
// Pkl property can have, and for one that two fields of a module share.
func renderSchema(metadata *objects.Metadata) ([]generatedFile, error) {
	var files []generatedFile
	var problems []string
	for _, module := range schemaModules {
		fields := typedFields(metadata, module.category)
		problems = append(problems, unusableNames(module.name+"Props", fields)...)
		files = append(files, generatedFile{
			path: schemaFolder + "/" + module.name + "Props.pkl",
			text: renderModule(metadata.Game, module, fields),
		})
	}
	if len(problems) > 0 {
		return nil, errUnusableNames(problems)
	}
	return files, nil
}

// typedFields is the fields that have a typed property in the module of a category, in the order the module has
// them: by name, and by id where two share a name. A field that only the copies of some base abilities have is
// none of them: it is set through properties.
func typedFields(metadata *objects.Metadata, category manifest.Category) []objects.FieldMeta {
	list, use := objects.FieldSource(category)
	var fields []objects.FieldMeta
	for _, field := range metadata.Fields[list] {
		if (use == "" || slices.Contains(field.Use, use)) && len(field.Specific) == 0 {
			fields = append(fields, field)
		}
	}
	slices.SortStableFunc(fields, func(a, b objects.FieldMeta) int {
		return cmp.Or(strings.Compare(a.Name, b.Name), strings.Compare(a.ID, b.ID))
	})
	return fields
}

// unusableNames is a line for each field of a module whose name no property can have, and for each that has the
// name of the field before it. The fields are in the order of the module.
func unusableNames(module string, fields []objects.FieldMeta) []string {
	var problems []string
	latest := map[string]objects.FieldMeta{} // the last field of each name so far
	for _, field := range fields {
		where := module + ": field " + fsx.Quoted(displayRawcode(field.ID)) + " (" + field.Label + ")"
		if !pklIdentifier.MatchString(field.Name) || nameIsTaken(field.Name) {
			problems = append(problems, noPropertyName(where, field.Name))
		}
		if other, shared := latest[field.Name]; shared {
			problems = append(problems, sharedName(where, field.Name, other))
		}
		latest[field.Name] = field
	}
	return problems
}

// displayRawcode is a rawcode as an author sees it: the one id of three letters is padded with a NUL to four
// bytes in the metadata.
func displayRawcode(id string) string { return strings.TrimRight(id, "\x00") }

// renderModule is the text of the module of a category for a version of the game: its head, and a property with
// its doc comment for each field.
func renderModule(game string, module schemaModule, fields []objects.FieldMeta) string {
	what := "the fields of " + string(module.category)
	if module.category == "abilities" {
		what = "the ability fields not specific to one base ability"
	}
	lines := []string{
		// The dash is an em dash.
		"// GENERATED by `" + commandLine + "` from " + metadataPath + " (game " + game + ") \xE2\x80\x94 do not edit.",
		"",
		"/// The typed properties of `" + module.name + ".pkl`: " + what + ", named after their World Editor labels.",
		"abstract module moonwell.generated." + module.name + "Props",
		"",
		`extends "../objects/Object.pkl"`,
	}
	for _, field := range fields {
		property := field.Name + ": " + pklType(field)
		lines = append(lines, "", "/// "+oneLine(field.Label), "///", "/// "+facts(field), property)
	}
	return strings.Join(lines, "\n") + "\n"
}

// facts is the line of a field's doc comment that says what the game knows of it: its rawcode, its category and
// its type, and how its value is given.
func facts(field objects.FieldMeta) string {
	rawcode, category := displayRawcode(field.ID), oneLine(field.Category)
	parts := []string{"Field `" + rawcode + "` (" + category + ", `" + field.Type + "`)."}
	if field.PerLevel {
		parts = append(parts, "Per level: a `List` sets levels 1, 2, ...")
	}
	if field.List {
		parts = append(parts, "A comma-separated list: a `List<String>` is joined with commas.")
	}
	if field.Skin {
		parts = append(parts, "Skin field.")
	}
	return strings.Join(parts, " ")
}

// pklType is the type of a field's property. Every property may be left out; a field with levels takes one value
// or a list of them; and a field that is a list takes its text or the list.
func pklType(field objects.FieldMeta) string {
	if field.List {
		if field.PerLevel {
			return "(String|List<String>|List<List<String>>)?"
		}
		return "(String|List<String>)?"
	}
	scalar := "Number"
	switch {
	case field.Storage == "string":
		scalar = "String"
	case field.Storage == "int" && field.Type == "bool":
		scalar = "Boolean"
	case field.Storage == "int":
		scalar = "Int"
	}
	if field.PerLevel {
		return "(" + scalar + "|List<" + scalar + ">)?"
	}
	return scalar + "?"
}

// oneLine writes a text on one line: each run of ASCII white space is one space, and none stands at an end.
func oneLine(text string) string {
	isSpace := func(r rune) bool { return strings.ContainsRune(fsx.ASCIISpace, r) }
	return strings.Join(strings.FieldsFunc(text, isSpace), " ")
}

// ---- errors ----

// errNoJSON names a file of the checkout, by its path from there, that the JSON decoder refused.
func errNoJSON(path string, cause error) error { return errors.New(path + ": " + cause.Error()) }

// errUnusableNames refuses to render the schema, with a line for each name that stands in its way.
func errUnusableNames(problems []string) error {
	return errors.New("Cannot render the Pkl schema; fix the names in tools/metadata/overrides.json:\n" +
		strings.Join(problems, "\n"))
}

// noPropertyName is the line for a field, which where names, whose name no Pkl property can have.
func noPropertyName(where, name string) string {
	return where + " has the name " + fsx.Quoted(name) + ", which is reserved or not a Pkl identifier."
}

// sharedName is the line for a field, which where names, that has the name of another field of its module.
func sharedName(where, name string, other objects.FieldMeta) string {
	return where + " has the name " + fsx.Quoted(name) + ", as does field " + fsx.Quoted(displayRawcode(other.ID)) + "."
}
