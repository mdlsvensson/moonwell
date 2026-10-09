package objects

import (
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/war3/objmod"
)

type Value struct {
	Type   objmod.ValueType
	Number float64
	Text   string
}

type Field struct {
	ID            string
	Name          string
	Level, Column int
	Skin          bool
	Value         Value
}

type Resolved struct {
	Category manifest.Category
	Key      string
	ID, Base string
	Source   string
	Fields   []Field
}

func Resolve(metadata *Metadata, objects manifest.Objects, existing map[string]bool) ([]Resolved, error) {
	r := &resolver{metadata: metadata, existing: existing, idOwners: map[string]idOwner{}}
	resolved := []Resolved{}
	for _, category := range manifest.Categories {
		for key, object := range objects.ByCategory(category).All() {
			if resolvedObject, ok := r.resolveObject(category, key, object); ok {
				resolved = append(resolved, resolvedObject)
			}
		}
	}
	if len(r.problems) > 0 {
		return nil, r.problems
	}
	return resolved, nil
}

type resolver struct {
	metadata *Metadata
	existing map[string]bool
	idOwners map[string]idOwner
	problems diag.Problems
}

type idOwner struct{ path, source string }

func (r *resolver) resolveObject(category manifest.Category, key string, object manifest.Object) (Resolved, bool) {
	path := string(category) + "[" + fsx.QuoteJSON(key) + "]"
	objResolver := &objectResolver{resolver: r, category: category, path: path, object: object}
	objResolver.checkID()
	objResolver.claimID()
	base, known := r.metadata.Bases[category][object.Base]
	if !known {
		objResolver.report(".base", errNotABase(r.metadata, category, object.Base))
		return Resolved{}, false
	}
	return Resolved{
		Category: category, Key: key, ID: object.ID, Base: object.Base, Source: object.Source, Fields: objResolver.resolveFields(base),
	}, true
}

type objectResolver struct {
	*resolver
	category manifest.Category
	path     string
	object   manifest.Object
}

type issue struct{ msg, hint string }

func (r *objectResolver) report(path string, found issue) {
	r.problems = append(r.problems, diag.Problem{
		File: r.object.Source, Msg: r.path + path + ": " + found.msg, Hint: found.hint,
	})
}

func (r *objectResolver) checkID() {
	id := r.object.ID
	if !isFourLettersOrDigits(id) {
		r.report(".id", errNotAnID(id, r.category))
		return
	}
	startsUppercase := id[0] >= 'A' && id[0] <= 'Z'
	switch {
	case r.category == "heroes" && !startsUppercase:
		r.report(".id", errNotAHeroID(id))
	case (r.category == "units" || r.category == "buildings") && startsUppercase:
		r.report(".id", errHeroID(id, r.category))
	}
	if category, standard, found := r.metadata.BaseOf(id); found {
		r.report(".id", errStandardID(id, category, standard))
	}
	if r.existing[id] {
		r.report(".id", errIDInTheMap(id))
	}
}

func (r *objectResolver) claimID() {
	if first, taken := r.idOwners[r.object.ID]; taken {
		r.report(".id", errIDTwice(r.object.ID, first))
		return
	}
	r.idOwners[r.object.ID] = idOwner{r.path, r.object.Source}
}

func isFourLettersOrDigits(id string) bool {
	if len(id) != 4 {
		return false
	}
	for i := range len(id) {
		c := id[i]
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

func formatNamedID(id, name string) string {
	return "'" + strings.TrimRight(id, "\x00") + "' (" + name + ")"
}

var exampleIDs = map[manifest.Category]string{
	"heroes": "H000", "units": "h000", "buildings": "h000", "items": "I000", "abilities": "A000", "buffs": "B000",
	"upgrades": "R000",
}

var singular = map[manifest.Category]string{
	"heroes": "hero", "units": "unit", "buildings": "building", "items": "item", "abilities": "ability",
	"buffs": "buff", "upgrades": "upgrade",
}

func errNotAnID(id string, category manifest.Category) issue {
	return issue{"'" + id + "' is not four ASCII letters or digits.", "Use an id such as '" + exampleIDs[category] + "'."}
}

func errNotAHeroID(id string) issue {
	return issue{
		"'" + id + "' must start with an uppercase letter: the game treats exactly those unit ids as heroes.",
		"Use an id such as '" + exampleIDs["heroes"] + "'.",
	}
}

func errHeroID(id string, category manifest.Category) issue {
	return issue{
		"'" + id + "' must not start with an uppercase letter: the game would treat it as a hero.",
		"Use an id such as '" + exampleIDs[category] + "', or make the object a hero.",
	}
}

func errStandardID(id string, category manifest.Category, standard BaseMeta) issue {
	return issue{
		"'" + id + "' is the id of a standard " + singular[category] + " (" + standard.Name + ").",
		"Moonwell adds custom objects and cannot modify standard ones: pick an id no standard object uses.",
	}
}

func errIDInTheMap(id string) issue {
	return issue{
		"'" + id + "' is already the id of a custom object in the map.",
		"Change the id in Pkl, or delete the object in World Editor.",
	}
}

func errIDTwice(id string, first idOwner) issue {
	return issue{
		"'" + id + "' is also the id of " + first.path + " (" + first.source + ").",
		"Give each object its own id.",
	}
}

func errNotABase(metadata *Metadata, category manifest.Category, id string) issue {
	var hints []string
	if other, base, found := metadata.BaseOf(id); found {
		hints = append(hints, "'"+id+"' is a standard "+singular[other]+" ("+base.Name+").")
	}
	var nearest []string
	for _, candidate := range metadata.NearestBases(category, id, 3) {
		nearest = append(nearest, formatNamedID(candidate.ID, candidate.Name))
	}
	if len(nearest) > 0 {
		hints = append(hints, "Did you mean "+diag.JoinWords(nearest, "or", -1)+"?")
	}
	return issue{"'" + id + "' is not a standard " + singular[category] + ".", strings.Join(hints, " ")}
}
