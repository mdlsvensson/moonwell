package main

import (
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/objects"
)

type nameChange struct{ list, id, description string }

func assignNames(
	records []objects.FieldMeta, list string, overrides nameOverrides, abilities []string,
) (changes, problems []string) {
	labelNames := nameFromLabels(records)
	pinned := applyPins(records, list, overrides)
	groups := clashGroups(records, list, abilities)
	renameReasons := renameClashes(records, groups, pinned)
	clashingWith := findClashes(records, groups)
	for _, i := range slices.Sorted(maps.Keys(clashingWith)) {
		problems = append(problems, describeClash(list, records[i], records[clashingWith[i]]))
	}
	changes = make([]string, len(records))
	for i, field := range records {
		if !friendlyName.MatchString(field.Name) || isReservedName(field.Name) {
			problems = append(problems, describeUnusableName(list, field, pinned[i]))
		}
		changes[i] = describeChange(labelNames[i], field.Name, pinned[i], renameReasons[i])
	}
	return changes, problems
}

func nameFromLabels(records []objects.FieldMeta) []string {
	labelNames := make([]string, len(records))
	for i := range records {
		labelNames[i] = camelCase(records[i].Label)
		records[i].Name = labelNames[i]
	}
	return labelNames
}

func applyPins(records []objects.FieldMeta, list string, overrides nameOverrides) []bool {
	pinned := make([]bool, len(records))
	for i := range records {
		if name, ok := overrides.pinnedName(list, records[i].ID); ok {
			records[i].Name, pinned[i] = name, true
		}
	}
	return pinned
}

type renameRule struct {
	reason string
	rename func(field objects.FieldMeta) string
}

var renameRules = []renameRule{
	{"category prefix", func(field objects.FieldMeta) string { return field.Category + capitalize(field.Name) }},
	{"rawcode", func(field objects.FieldMeta) string { return field.Name + capitalize(displayRawcode(field.ID)) }},
}

func renameClashes(records []objects.FieldMeta, groups [][]string, pinned []bool) [][]string {
	renameReasons := make([][]string, len(records))
	for _, rule := range renameRules {
		for i := range findClashes(records, groups) {
			if pinned[i] {
				continue
			}
			records[i].Name = rule.rename(records[i])
			renameReasons[i] = append(renameReasons[i], rule.reason)
		}
	}
	return renameReasons
}

const pinReason = "override"

func describeChange(labelName, name string, pinned bool, renameReasons []string) string {
	reasons := renameReasons
	if pinned {
		reasons = []string{pinReason}
	}
	if len(reasons) == 0 {
		return ""
	}
	return `"` + labelName + `" -> "` + name + `" (` + strings.Join(reasons, " and ") + ")"
}

func clashGroups(records []objects.FieldMeta, list string, abilities []string) [][]string {
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
	add := func(ids []string) {
		for _, id := range ids {
			if !seen[id] {
				seen[id] = true
				mentioned = append(mentioned, id)
			}
		}
	}
	add(abilities)
	for _, field := range records {
		add(field.Specific)
		add(field.NotSpecific)
	}
	return mentioned
}

func findClashes(records []objects.FieldMeta, groups [][]string) map[int]int {
	firstByKey := map[string]int{}
	clashingWith := map[int]int{}
	for i, field := range records {
		for _, group := range groups[i] {
			key := group + "\x00" + field.Name
			firstIndex, ok := firstByKey[key]
			switch {
			case !ok:
				firstByKey[key] = i
			case firstIndex != i:
				clashingWith[i] = firstIndex
				if _, exists := clashingWith[firstIndex]; !exists {
					clashingWith[firstIndex] = i
				}
			}
		}
	}
	return clashingWith
}

var (
	removedPunctuation = strings.NewReplacer("'", "", "\xE2\x80\x99", "", "!", "", ".", "")
	spelledOutSigns    = strings.NewReplacer("/", " Or ", "&", " And ", "+", " Plus ", "%", " Percent ")
	nonWordRun         = regexp.MustCompile(`[^A-Za-z0-9]+`)
)

func camelCase(label string) string {
	var name strings.Builder
	for _, word := range nonWordRun.Split(spelledOutSigns.Replace(removedPunctuation.Replace(label)), -1) {
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

func describeNamedField(list string, field objects.FieldMeta) string {
	return list + " " + displayRawcode(field.ID) + ` "` + field.Name + `"`
}

func describeClash(list string, field, other objects.FieldMeta) string {
	return describeNamedField(list, field) + " (" + field.Label + "): " + displayRawcode(other.ID) + " (" + other.Label +
		") has this name too, and one object can have both: pin another name for one of the two in " + overridesPath
}

func describeUnusableName(list string, field objects.FieldMeta, pinned bool) string {
	line := describeNamedField(list, field) + " (" + field.Label + "): "
	const why = "no property can have this name (a keyword of Pkl, a name that every object has, or not a small " +
		"letter with letters and digits after it); pin another "
	if pinned {
		return line + "the pin is refused: " + why + "in " + overridesPath
	}
	return line + why + `under "names", ` + fsx.QuoteJSON(list) + ", " + fsx.QuoteJSON(displayRawcode(field.ID)) +
		" in " + overridesPath
}
