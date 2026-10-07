package main

import (
	"errors"
	"regexp"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/objects"
)

// ---- the pins ----

// overridesPath is the names that are pinned, written by hand, by its path from the checkout.
const overridesPath = "tools/metadata/overrides.json"

// overrides is tools/metadata/overrides.json: the friendly names that are pinned, by the list of the field and
// its id, and the fields whose leaving the game is acknowledged, by their list.
type overrides struct {
	Names   map[string]map[string]string `json:"names"`
	Removed map[string][]string          `json:"removed"`
}

// readOverrides reads the overrides of a checkout. A file that leaves a key out pins nothing, or lists no field
// as removed.
func readOverrides(checkout string) (overrides, error) {
	var pins overrides
	if err := readHandWritten(checkout, overridesPath, &pins); err != nil {
		return overrides{}, err
	}
	return pins, nil
}

// pinned is the name that the overrides pin for a field of a list, and whether they pin one. The fields of units
// and of items are one table, so a pin under either list is of both.
func (o overrides) pinned(list, id string) (string, bool) {
	lists := []string{list}
	if list == "units" || list == "items" {
		lists = []string{"units", "items"}
	}
	for _, under := range lists {
		if name, has := o.Names[under][id]; has {
			return name, true
		}
	}
	return "", false
}

// ---- the names of the fields of a table ----

// rename is a field whose friendly name is not the name of its label: the first list it is in, its id, and what
// is reported of it, which is the two names and why they differ.
type rename struct{ list, id, change string }

// assignNames gives the fields of one table their friendly names. It returns, for each field, what is reported
// of a name that is not the name of its label ("" for a field that is named after its label), and a line for
// each field whose name cannot stand. list is the first list of the table, and abilities the ids of the game's
// abilities.
//
// A field is named after its label. A pin of the overrides replaces that name. Fields whose names then clash
// take their category before the name, and those that still clash take their rawcode after it; a pinned name is
// left as it is pinned. What cannot stand after that is a name that still clashes, and one that no property can
// have.
func assignNames(
	records []objects.FieldMeta, list string, pins overrides, abilities []string,
) (changes, problems []string) {
	fromLabel := nameAfterLabels(records)
	pinned := applyPins(records, list, pins)
	groups := groupsOf(records, list, abilities)
	renamedBy := renameClashes(records, groups, pinned)
	for _, i := range clashing(records, groups) {
		problems = append(problems, stillClashes(list, records[i]))
	}
	changes = make([]string, len(records))
	for i, field := range records {
		if !friendlyName.MatchString(field.Name) || nameIsTaken(field.Name) {
			problems = append(problems, noFriendlyName(list, field))
		}
		changes[i] = changeOf(fromLabel[i], field.Name, pinned[i], renamedBy[i])
	}
	return changes, problems
}

// nameAfterLabels gives every field the name of its label, and returns those names.
func nameAfterLabels(records []objects.FieldMeta) []string {
	fromLabel := make([]string, len(records))
	for i := range records {
		fromLabel[i] = camelCase(records[i].Label)
		records[i].Name = fromLabel[i]
	}
	return fromLabel
}

// applyPins gives every field that the overrides pin its pinned name, and returns which fields those are.
func applyPins(records []objects.FieldMeta, list string, pins overrides) []bool {
	pinned := make([]bool, len(records))
	for i := range records {
		if name, has := pins.pinned(list, records[i].ID); has {
			records[i].Name, pinned[i] = name, true
		}
	}
	return pinned
}

// renaming is a way to rename a field whose name clashes with another's: the name it gives the field, made of
// the name the field has, and the reason that is reported for it.
type renaming struct {
	reason string
	rename func(field objects.FieldMeta) string
}

// The two renamings of a clash, in the order they are tried: the category of the field before its name, and
// then the rawcode after it.
var renamings = []renaming{
	{"category prefix", func(field objects.FieldMeta) string { return field.Category + capitalize(field.Name) }},
	{"rawcode", func(field objects.FieldMeta) string { return field.Name + capitalize(field.ID) }},
}

// renameClashes renames the fields whose names clash, by each renaming in turn: every field that clashes when a
// renaming's turn comes is renamed by it, but for a pinned field, which keeps its pin. It returns, for each
// field, the reasons of the renamings that renamed it, in their order.
func renameClashes(records []objects.FieldMeta, groups [][]string, pinned []bool) [][]string {
	renamedBy := make([][]string, len(records))
	for _, way := range renamings {
		for _, i := range clashing(records, groups) {
			if pinned[i] {
				continue
			}
			records[i].Name = way.rename(records[i])
			renamedBy[i] = append(renamedBy[i], way.reason)
		}
	}
	return renamedBy
}

// pinReason is the reason that is reported for a name that the overrides pin.
const pinReason = "override"

// changeOf is what is reported of a field whose name the overrides pin or a renaming made: the name of its label,
// the name it has, and why. It is "" for a field that is neither pinned nor renamed. A pin is reported also
// where it pins the name of the label.
func changeOf(fromLabel, name string, pinned bool, renamedBy []string) string {
	reasons := renamedBy
	if pinned {
		reasons = append([]string{pinReason}, renamedBy...)
	}
	if len(reasons) == 0 {
		return ""
	}
	return `"` + fromLabel + `" -> "` + name + `" (` + strings.Join(reasons, " and ") + ")"
}

// groupsOf is, for each field of a table, the groups that its name must be the only one of its kind in: a group
// is the fields that can meet in one object. A field of the units' table is in a group for each kind of object
// that uses it. A field of an ability that is of some base abilities alone is in the group of each of them; any
// other field of an ability is in the group of the fields that every ability has, and in that of each base
// ability that it is not kept from. The fields of every other table are one group.
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

// mentionedAbilities is every base ability that the game's data names, each once and in the order they are first
// named: the abilities of the game's table, then those in the two lists of each field.
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

// clashing is the fields, each by its index and in their order, whose name is twice in one of their groups. A
// field that lists a base ability twice is twice in that group, and clashes there with itself.
func clashing(records []objects.FieldMeta, groups [][]string) []int {
	sharing := map[string][]int{} // the fields that have a name in a group, by the group, a NUL and the name
	for i, field := range records {
		for _, group := range groups[i] {
			named := group + "\x00" + field.Name
			sharing[named] = append(sharing[named], i)
		}
	}
	var clash []int
	for _, fields := range sharing {
		if len(fields) > 1 {
			clash = append(clash, fields...)
		}
	}
	slices.Sort(clash)
	return slices.Compact(clash)
}

// ---- the name of a label ----

// What a label's punctuation becomes in its name: a mark that is dropped joins the letters around it, and a sign
// that is spelled is a word of its own.
var (
	// The apostrophes, the straight one and the curled one, U+2019, the exclamation mark and the full stop.
	droppedMarks = strings.NewReplacer("'", "", "\xE2\x80\x99", "", "!", "", ".", "")
	spelledSigns = strings.NewReplacer("/", " Or ", "&", " And ", "+", " Plus ", "%", " Percent ")
	// notWord is what stands between two words: everything that is no letter of ASCII and no digit.
	notWord = regexp.MustCompile(`[^A-Za-z0-9]+`)
)

// camelCase is the friendly name of a label: its words joined, each but the first with a capital, so that
// "Hit Points Maximum (Base)" is hitPointsMaximumBase. The first word starts with a small letter, and is all
// small where the label has it all in capitals. An apostrophe, "!" and "." are dropped, "/" reads Or, "&" And,
// "+" Plus and "%" Percent, and every other character that is no letter and no digit ends a word.
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

// capitalize is a word with a capital for its first letter. The words are of ASCII: a word of a label, a name,
// a category, and a rawcode.
func capitalize(word string) string {
	if word == "" {
		return word
	}
	return strings.ToUpper(word[:1]) + word[1:]
}

// ---- the names that are released ----

// keepsReleasedNames holds the fields to the names they have in the data/metadata.json of the checkout, which is
// what authors write: a field of that file must still be there and have its name, unless the overrides pin the
// name it has now or list the field as removed. A checkout without the file has released no name.
func keepsReleasedNames(metadata *objects.Metadata, checkout string, pins overrides) error {
	data, found, err := fsx.ReadIfThere(fileIn(checkout, metadataPath))
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

// changedNames is a line for each released field of a list that the fields of now do not have, and for each
// whose name is another now.
func changedNames(list string, released, current []objects.FieldMeta, pins overrides) []string {
	names := map[string]string{}
	for _, field := range current {
		names[field.ID] = field.Name
	}
	var problems []string
	for _, field := range released {
		name, still := names[field.ID]
		pinned, _ := pins.pinned(list, field.ID)
		switch {
		case !still && !slices.Contains(pins.Removed[list], field.ID):
			problems = append(problems, wouldDisappear(list, field))
		case still && name != field.Name && pinned != name:
			problems = append(problems, wouldBecome(list, field, name))
		}
	}
	return problems
}

// ---- the names that no field can have ----

// The names that no field can have as its property. Every part of the generator that gives a field its name, or
// checks one, reads these two lists, so that a name is refused where it is made and not only where the schema is
// rendered.
var (
	// reservedNames is the names that an object module has of itself: the properties of Object.pkl (base, source,
	// properties), the id that the module of each category declares, and output, which every Pkl module has.
	reservedNames = []string{"id", "base", "source", "properties", "output"}

	// pklKeywords is the keywords of Pkl 0.32 and the words it reserves for a later version.
	pklKeywords = strings.Fields("abstract amends as case class const delete else extends external false fixed for " +
		"function hidden if import in is let local module new nothing null open out outer override private protected " +
		"public read record super switch this throw trace true typealias unknown vararg when")
)

// nameIsTaken reports whether Pkl or an object module has the name already, so that no field's property can.
func nameIsTaken(name string) bool {
	return slices.Contains(reservedNames, name) || slices.Contains(pklKeywords, name)
}

// pklIdentifier is a name that Pkl reads as an identifier without quoting it.
var pklIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// friendlyName is the form of a name that the generator gives a field: letters of ASCII and digits, the first
// a small letter.
var friendlyName = regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)

// ---- errors ----

// stillClashes is the line for a field whose name another field has after both renamings.
func stillClashes(list string, field objects.FieldMeta) string {
	return list + " " + field.ID + ` "` + field.Name + `" (` + field.Label + "): clashes with another field"
}

// noFriendlyName is the line for a field whose name no property can have.
func noFriendlyName(list string, field objects.FieldMeta) string {
	return list + " " + field.ID + ` "` + field.Name + `" (` + field.Label + "): not a valid property name, a Pkl " +
		"keyword or a reserved name; add a name for it to " + overridesPath
}

// wouldDisappear is the line for a released field of a list that the game's data have not, and that the
// overrides do not list as removed.
func wouldDisappear(list string, released objects.FieldMeta) string {
	return list + " " + released.ID + ` "` + released.Name + `" would disappear`
}

// wouldBecome is the line for a released field of a list whose name would be another now, and that no pin holds
// to it.
func wouldBecome(list string, released objects.FieldMeta, name string) string {
	return list + " " + released.ID + ` "` + released.Name + `" would become "` + name + `"`
}

// errReleasedNames refuses fields that would change what authors write, with a line for each name.
func errReleasedNames(problems []string) error {
	return errors.New("released friendly names would change. Pin each in " + overridesPath + ` "names", or list ` +
		`a field the game no longer has under "removed":` + "\n  " + strings.Join(problems, "\n  "))
}
