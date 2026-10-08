package main

import (
	"errors"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/objects"
)

const overridesPath = "tools/metadata/overrides.json"

type overrides struct {
	Names   map[string]map[string]string `json:"names"`
	Removed map[string][]string          `json:"removed"`
}

func readOverrides(checkout string) (overrides, error) {
	var pins overrides
	if err := readHandWritten(checkout, overridesPath, &pins); err != nil {
		return overrides{}, err
	}
	return pins, nil
}

func (o overrides) pinnedName(list, id string) (string, bool) {
	for _, under := range oneTable(list) {
		if name, has := o.Names[under][displayRawcode(id)]; has {
			return name, true
		}
	}
	return "", false
}

func oneTable(list string) []string {
	if list == "units" || list == "items" {
		return []string{"units", "items"}
	}
	return []string{list}
}

func (o overrides) unusedPins(fields map[string][]objects.FieldMeta) []string {
	var problems []string
	for _, list := range slices.Sorted(maps.Keys(o.Names)) {
		if !slices.Contains(objects.FieldLists, list) {
			problems = append(problems, noSuchList("names", list))
			continue
		}
		for _, id := range slices.Sorted(maps.Keys(o.Names[list])) {
			if !hasField(fields, oneTable(list), id) {
				problems = append(problems, pinOfNoField(list, id))
			}
		}
	}
	for _, list := range slices.Sorted(maps.Keys(o.Removed)) {
		if !slices.Contains(objects.FieldLists, list) {
			problems = append(problems, noSuchList("removed", list))
		}
	}
	return problems
}

func hasField(fields map[string][]objects.FieldMeta, lists []string, id string) bool {
	for _, list := range lists {
		for _, field := range fields[list] {
			if displayRawcode(field.ID) == id {
				return true
			}
		}
	}
	return false
}

type rename struct{ list, id, change string }

func assignNames(
	records []objects.FieldMeta, list string, pins overrides, abilities []string,
) (changes, problems []string) {
	fromLabel := nameAfterLabels(records)
	pinned := applyPins(records, list, pins)
	groups := groupsOf(records, list, abilities)
	renamedBy := renameClashes(records, groups, pinned)
	with := clashes(records, groups)
	for _, i := range slices.Sorted(maps.Keys(with)) {
		problems = append(problems, stillClashes(list, records[i], records[with[i]]))
	}
	changes = make([]string, len(records))
	for i, field := range records {
		if !friendlyName.MatchString(field.Name) || isReservedName(field.Name) {
			problems = append(problems, noFriendlyName(list, field, pinned[i]))
		}
		changes[i] = changeOf(fromLabel[i], field.Name, pinned[i], renamedBy[i])
	}
	return changes, problems
}

func nameAfterLabels(records []objects.FieldMeta) []string {
	fromLabel := make([]string, len(records))
	for i := range records {
		fromLabel[i] = camelCase(records[i].Label)
		records[i].Name = fromLabel[i]
	}
	return fromLabel
}

func applyPins(records []objects.FieldMeta, list string, pins overrides) []bool {
	pinned := make([]bool, len(records))
	for i := range records {
		if name, has := pins.pinnedName(list, records[i].ID); has {
			records[i].Name, pinned[i] = name, true
		}
	}
	return pinned
}

type renaming struct {
	reason string
	rename func(field objects.FieldMeta) string
}

var renamings = []renaming{
	{"category prefix", func(field objects.FieldMeta) string { return field.Category + capitalize(field.Name) }},
	{"rawcode", func(field objects.FieldMeta) string { return field.Name + capitalize(displayRawcode(field.ID)) }},
}

func renameClashes(records []objects.FieldMeta, groups [][]string, pinned []bool) [][]string {
	renamedBy := make([][]string, len(records))
	for _, way := range renamings {
		for i := range clashes(records, groups) {
			if pinned[i] {
				continue
			}
			records[i].Name = way.rename(records[i])
			renamedBy[i] = append(renamedBy[i], way.reason)
		}
	}
	return renamedBy
}

const pinReason = "override"

func changeOf(fromLabel, name string, pinned bool, renamedBy []string) string {
	reasons := renamedBy
	if pinned {
		reasons = []string{pinReason}
	}
	if len(reasons) == 0 {
		return ""
	}
	return `"` + fromLabel + `" -> "` + name + `" (` + strings.Join(reasons, " and ") + ")"
}

func groupsOf(records []objects.FieldMeta, list string, abilities []string) [][]string {
	mentioned := mentionedAbilities(records, abilities)
	groups := make([][]string, len(records))
	for i, field := range records {
		switch {
		case list == "units":
			groups[i] = field.Use
		case list != "abilities":
			groups[i] = []string{"all"}
		case len(field.Specific) > 0:
			groups[i] = field.Specific
		default:
			groups[i] = []string{"common"}
			for _, base := range mentioned {
				if !slices.Contains(field.NotSpecific, base) {
					groups[i] = append(groups[i], base)
				}
			}
		}
	}
	return groups
}

func mentionedAbilities(records []objects.FieldMeta, abilities []string) []string {
	var mentioned []string
	seen := map[string]bool{}
	note := func(ids []string) {
		for _, id := range ids {
			if !seen[id] {
				seen[id] = true
				mentioned = append(mentioned, id)
			}
		}
	}
	note(abilities)
	for _, field := range records {
		note(field.Specific)
		note(field.NotSpecific)
	}
	return mentioned
}

func clashes(records []objects.FieldMeta, groups [][]string) map[int]int {
	first := map[string]int{}
	with := map[int]int{}
	for i, field := range records {
		for _, group := range groups[i] {
			inGroup := group + "\x00" + field.Name
			earlier, taken := first[inGroup]
			switch {
			case !taken:
				first[inGroup] = i
			case earlier != i:
				with[i] = earlier
				if _, has := with[earlier]; !has {
					with[earlier] = i
				}
			}
		}
	}
	return with
}

var (
	droppedMarks = strings.NewReplacer("'", "", "\xE2\x80\x99", "", "!", "", ".", "")
	spelledSigns = strings.NewReplacer("/", " Or ", "&", " And ", "+", " Plus ", "%", " Percent ")
	notWord      = regexp.MustCompile(`[^A-Za-z0-9]+`)
)

func camelCase(label string) string {
	var name strings.Builder
	for _, word := range notWord.Split(spelledSigns.Replace(droppedMarks.Replace(label)), -1) {
		switch {
		case word == "":
		case name.Len() > 0:
			name.WriteString(capitalize(word))
		case word == strings.ToUpper(word):
			name.WriteString(strings.ToLower(word))
		default:
			name.WriteString(strings.ToLower(word[:1]) + word[1:])
		}
	}
	return name.String()
}

func capitalize(word string) string {
	if word == "" {
		return word
	}
	return strings.ToUpper(word[:1]) + word[1:]
}

func checkReleasedNamesKept(metadata *objects.Metadata, checkout string, pins overrides) error {
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
		problems = append(problems, changedNames(list, released.Fields[list], metadata.Fields[list], pins)...)
	}
	if len(problems) > 0 {
		return errReleasedNames(problems)
	}
	return nil
}

func changedNames(list string, released, current []objects.FieldMeta, pins overrides) []string {
	names := map[string]string{}
	for _, field := range current {
		names[field.ID] = field.Name
	}
	var problems []string
	for _, field := range released {
		name, still := names[field.ID]
		pinned, _ := pins.pinnedName(list, field.ID)
		switch {
		case !still && !slices.Contains(pins.Removed[list], displayRawcode(field.ID)):
			problems = append(problems, wouldDisappear(list, field))
		case still && name != field.Name && pinned != name:
			problems = append(problems, wouldBecome(list, field, name))
		}
	}
	return problems
}

var (
	reservedNames = []string{"id", "base", "source", "properties", "output"}

	pklKeywords = strings.Fields("abstract amends as case class const delete else extends external false fixed for " +
		"function hidden if import in is let local module new nothing null open out outer override private protected " +
		"public read record super switch this throw trace true typealias unknown vararg when")
)

func isReservedName(name string) bool {
	return slices.Contains(reservedNames, name) || slices.Contains(pklKeywords, name)
}

var pklIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var friendlyName = regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)

func noSuchList(key, list string) string {
	return key + "." + list + " is none of the lists of fields (" + joinNames(objects.FieldLists) +
		"): correct its name in " + overridesPath
}

func pinOfNoField(list, id string) string {
	return "the pin of " + fsx.QuoteJSON(id) + " under names." + list + " names no field: no field of " +
		strings.Join(oneTable(list), " or ") + " has that id; correct the id or take the pin out of " + overridesPath
}

func fieldNamed(list string, field objects.FieldMeta) string {
	return list + " " + displayRawcode(field.ID) + ` "` + field.Name + `"`
}

func stillClashes(list string, field, other objects.FieldMeta) string {
	return fieldNamed(list, field) + " (" + field.Label + "): " + displayRawcode(other.ID) + " (" + other.Label +
		") has this name too, and one object can have both: pin another name for one of the two in " + overridesPath
}

func noFriendlyName(list string, field objects.FieldMeta, pinned bool) string {
	line := fieldNamed(list, field) + " (" + field.Label + "): "
	const why = "no property can have this name (a keyword of Pkl, a name that every object has, or not a small " +
		"letter with letters and digits after it); pin another "
	if pinned {
		return line + "the pin is refused: " + why + "in " + overridesPath
	}
	return line + why + `under "names", ` + fsx.QuoteJSON(list) + ", " + fsx.QuoteJSON(displayRawcode(field.ID)) +
		" in " + overridesPath
}

func wouldDisappear(list string, released objects.FieldMeta) string {
	return fieldNamed(list, released) + " would disappear"
}

func wouldBecome(list string, released objects.FieldMeta, name string) string {
	return fieldNamed(list, released) + ` would become "` + name + `"`
}

func errReleasedNames(problems []string) error {
	return errors.New("released friendly names would change. In " + overridesPath + ", pin a name that would " +
		`become another under "names" (the released name to keep it, the new one to change it on purpose), and ` +
		`list a field that would disappear under "removed":` + "\n  " + strings.Join(problems, "\n  "))
}
