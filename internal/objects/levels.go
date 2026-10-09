package objects

import (
	"fmt"
	"math"
	"slices"

	"github.com/mdlsvensson/moonwell/internal/manifest"
)

var levelsField = map[manifest.Category]string{"abilities": "alev", "upgrades": "glvl"}

type levelCount struct {
	objectLevels     float64
	objectSetsLevels bool
	levelsField      *FieldMeta
	baseLevels       int
}

func (c levelCount) isTooFew() bool { return c.objectSetsLevels && c.objectLevels < 1 }

func (c levelCount) limit() float64 {
	if c.objectSetsLevels {
		return c.objectLevels
	}
	return float64(c.baseLevels)
}

func (r *objectResolver) countLevels(entries []fieldEntry, base BaseMeta) levelCount {
	count := levelCount{baseLevels: 1}
	if base.Levels != nil {
		count.baseLevels = max(*base.Levels, 1)
	}
	id, leveled := levelsField[r.category]
	i := slices.IndexFunc(entries, func(e fieldEntry) bool { return leveled && e.field.ID == id })
	if i < 0 {
		return count
	}
	number, isNumber := entries[i].value.(float64)
	if !isNumber || number != math.Trunc(number) || math.IsInf(number, 0) {
		return count
	}
	count.objectLevels, count.objectSetsLevels, count.levelsField = number, true, entries[i].field
	if count.isTooFew() {
		r.report(entries[i].path, errTooFewLevels(entries[i].field, number))
	}
	return count
}

type levelValue struct {
	path  string
	value any
}

func (r *objectResolver) expandLevels(entry fieldEntry, count levelCount) ([]levelValue, bool) {
	values, setsLevels := perLevelValues(entry)
	switch {
	case !setsLevels:
		return []levelValue{{entry.path, entry.value}}, !(count.isTooFew() && entry.field == count.levelsField)
	case !entry.field.PerLevel:
		r.report(entry.path, errNotPerLevel(entry.field))
	case len(values) == 0:
		r.report(entry.path, errNoLevels())
	case !count.isTooFew() && float64(len(values)) > count.limit():
		r.report(entry.path, errTooManyLevels(r, len(values), count))
	default:
		levels := make([]levelValue, len(values))
		for i, value := range values {
			levels[i] = levelValue{fmt.Sprintf("%s[%d]", entry.path, i), value}
		}
		return levels, true
	}
	return nil, false
}

func perLevelValues(entry fieldEntry) ([]any, bool) {
	values, isList := entry.value.([]any)
	if !isList {
		return nil, false
	}
	holdsAList := slices.ContainsFunc(values, func(value any) bool {
		_, nested := value.([]any)
		return nested
	})
	return values, !entry.field.List || holdsAList
}

func errTooFewLevels(field *FieldMeta, count float64) issue {
	return issue{
		describeField(field) + " must be at least 1, got " + formatNumber(count) + ".",
		"Every object has at least one level; use null to keep the base's.",
	}
}

func errNotPerLevel(field *FieldMeta) issue {
	if field.List {
		return issue{
			describeField(field) + " is not per level, so it takes one list, not a List of lists.",
			"Write one List<String>.",
		}
	}
	return issue{describeField(field) + " is not per level, so it takes one value, not a List.", "Write a single value."}
}

func errNoLevels() issue {
	return issue{"an empty List sets no levels.", "Use null to inherit every level from the base."}
}

func errTooManyLevels(r *objectResolver, given int, count levelCount) issue {
	levels := "levels"
	if field := r.metadata.FieldByRawcode(r.category, levelsField[r.category]); field != nil {
		levels = field.Name
	}
	if count.objectSetsLevels {
		return issue{
			fmt.Sprintf("%d levels given, but %s is %s.", given, levels, formatNumber(count.objectLevels)),
			fmt.Sprintf("Raise %s to %d, or remove values.", levels, given),
		}
	}
	base := describeBase(r.metadata, r.category, r.object.Base)
	return issue{
		fmt.Sprintf("%d levels given, but %s has %d.", given, base, count.baseLevels),
		fmt.Sprintf("Set %s = %d to add levels, or remove values.", levels, given),
	}
}
