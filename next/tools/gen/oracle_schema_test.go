package main

import "github.com/mdlsvensson/moonwell/next/internal/objects"

// ---- the runs of the mode without a name ----

// namelessUsage is what this tree says of a line that gives the mode without a name an argument.
const namelessUsage = "error: Usage: go run ./tools/gen\n"

// withFields lays a data/metadata.json with the fields, by the list they are in.
func withFields(fields map[string][]objects.FieldMeta) func(checkout) {
	return func(c checkout) { c.write(metadataPath, metadataText(c.t, fields)) }
}

// oneBuff lays a data/metadata.json whose one field is a field of buffs.
func oneBuff(id, name string) func(checkout) {
	return withFields(map[string][]objects.FieldMeta{"buffs": {field(id, name, nil)}})
}

// inTheWayOfTheSchema lays a metadata, and a file at a place where the schema needs a folder.
func inTheWayOfTheSchema(place string) func(checkout) {
	return func(c checkout) {
		oneBuff("fnam", "name")(c)
		c.write(place, "a file\n")
	}
}

// strays lays what does not belong in the folder of the schema, and a file beside the folder: a module that is
// stale, two stray files, a stray folder with a file two folders down, and a stray folder that is empty.
func strays(c checkout) {
	c.write("schema/generated/HeroProps.pkl", "stale\n")
	c.write("schema/generated/Stray.pkl", "stray\n")
	c.write("schema/generated/UnitProps.pkl.orig", "stray\n")
	c.write("schema/generated/old/deeper/Left.pkl", "stray\n")
	c.folder("schema/generated/empty")
	c.write("schema/Kept.pkl", "beside the folder\n")
}

// schemaRuns is the runs of the mode without a name: the committed metadata into an empty folder of the schema,
// over the committed schema, and over a schema with a stale file, stray files and stray folders; an empty first
// argument; a metadata with a field of every kind the schema renders, and one without fields; a run in a folder
// below the checkout, below the go.mod of another module, and in the nearer of two checkouts; a name that two
// fields share, a keyword, a reserved name, and every kind of name no property can have at once; a metadata that
// is not there, without and with its folder, one that is a folder, one that is cut short and one that is no JSON;
// a file at the place of the folder of the schema, and of the folder above it; and a folder that is no checkout.
func schemaRuns() []oracleRun {
	committed := generatedNames()
	runs := []oracleRun{
		{name: "the committed metadata, into an empty folder of the schema", asCommitted: committed,
			lay: func(c checkout) {
				c.carry(metadataPath)
				c.folder(schemaFolder)
			}},
		{name: "the committed metadata, over the committed schema", asCommitted: committed,
			lay: func(c checkout) { c.carry(metadataPath, schemaFolder) }},
		{name: "the committed metadata, over a schema with a stale file and strays", asCommitted: committed,
			lay: func(c checkout) {
				c.carry(metadataPath, schemaFolder)
				strays(c)
			}},
		{name: "an empty first argument", line: words(""), lay: oneBuff("fnam", "name")},
		{name: "a field of every kind", lay: withFields(fieldsOfEveryKind())},
		{name: "a metadata without fields", lay: func(c checkout) { c.write(metadataPath, "{}") }},
		{name: "in a folder below the checkout", below: "tools/gen/slk", lay: oneBuff("fnam", "name")},
		{name: "below the go.mod of another module", below: "other/deeper",
			lay: func(c checkout) {
				oneBuff("fnam", "name")(c)
				c.write("other/go.mod", anotherModule)
				c.write("other/data/metadata.json", metadataOfOneBuff(c.t, "foth", "other"))
			}},
		{name: "in the nearer of two checkouts", below: "inner/deeper",
			lay: func(c checkout) {
				oneBuff("fabo", "above")(c)
				c.write("schema/generated/Stray.pkl", "stray\n")
				c.write("inner/go.mod", moduleFile)
				c.write("inner/data/metadata.json", metadataOfOneBuff(c.t, "fnea", "nearer"))
			}},
		{name: "a name that two fields share", lay: withFields(map[string][]objects.FieldMeta{
			"units": {field("uaaa", "same", used("hero")), field("ubbb", "same", used("hero", "unit"))},
		})},
		{name: "a name that is a keyword", lay: oneBuff("fcls", "class")},
		{name: "a name that is reserved", lay: oneBuff("fout", "output")},
		{name: "names of every kind that no property can have, over a schema with strays",
			lay: func(c checkout) {
				withFields(fieldsWithUnusableNames())(c)
				strays(c)
			}},
		{name: "no metadata in the data folder", class: "FromCheckout", lay: noList},
		{name: "no data folder", class: "FromCheckout"},
		{name: "a folder at the place of the metadata", class: "FromCheckout",
			lay: func(c checkout) { c.folder(metadataPath) }},
		{name: "a file at the place of the folder of the schema", class: "FromCheckout",
			lay: inTheWayOfTheSchema("schema/generated")},
		{name: "a file at the place of the folder above the schema's", class: "FromCheckout",
			lay: inTheWayOfTheSchema("schema")},
		{name: "a metadata that is cut short", lay: func(c checkout) { c.write(metadataPath, `{"format": 1,`) }},
		{name: "a metadata that is no JSON", lay: func(c checkout) { c.write(metadataPath, "not JSON\n") }},
		{name: "something after an empty first argument", class: "CountRefused", refusal: namelessUsage,
			line: words("", "more"), lay: oneBuff("fnam", "name")},
		{name: "a label and a category with white space outside ASCII", class: "WiderSpace",
			lay: withFields(map[string][]objects.FieldMeta{"buffs": {field("fnbs", "noBreak", noBreakSpaces)}}),
			apart: map[string][]place{"schema/generated/BuffProps.pkl": {
				{"\n/// No Break\n", "\n/// \xC2\xA0No\xC2\xA0\xC2\xA0Break\xC2\xA0\n"},
				{"(art and sound, ", "(art\xC2\xA0and sound, "},
			}}},
	}
	return append(runs, noCheckoutRuns("the mode without a name", nil)...)
}

// noBreakSpaces gives a field a label with no-break spaces at its ends and two inside, and a category with one
// inside.
func noBreakSpaces(meta *objects.FieldMeta) {
	meta.Label, meta.Category = "\xC2\xA0No\xC2\xA0\xC2\xA0Break\xC2\xA0", "art\xC2\xA0and sound"
}

// fieldsOfEveryKind is fields of every kind that the schema renders in its own way: every type of a property,
// without and with levels; a bool that is stored as an int, as another kind of number, as a text, and without a
// storage; a skin field; a list; a label and a category with runs of ASCII white space; the id of three letters;
// fields by their use, in every module of the unit file; fields that only some base abilities have, or do not
// have; and names that differ in their letter case alone.
func fieldsOfEveryKind() map[string][]objects.FieldMeta {
	perLevel := func(inner func(*objects.FieldMeta)) func(*objects.FieldMeta) {
		return func(meta *objects.FieldMeta) {
			if inner != nil {
				inner(meta)
			}
			meta.PerLevel, meta.Column = true, 1
		}
	}
	list := func(fieldType string) func(*objects.FieldMeta) {
		return func(meta *objects.FieldMeta) { meta.Type, meta.Storage, meta.List = fieldType, "string", true }
	}
	return map[string][]objects.FieldMeta{
		"units": {
			field("uall", "everywhere", used("unit", "hero", "building")),
			field("uher", "heroOnly", used("hero")),
			field("ubld", "buildingOnly", used("building")),
			field("uitm", "itemOnly", used("item")),
			field("unam", "name", func(meta *objects.FieldMeta) {
				meta.Label, meta.Category, meta.Type, meta.Storage = "Name", "text", "string", "string"
				meta.Skin, meta.Use = true, []string{"unit", "hero"}
			}),
			field("uabi", "normal", func(meta *objects.FieldMeta) {
				list("abilityList")(meta)
				meta.Label, meta.Use = "Normal", []string{"unit", "building"}
			}),
		},
		"items": {field("iitm", "itemField", used("item")), field("iuni", "unitField", used("unit"))},
		"abilities": {
			field("aint", "anInt", nil),
			field("abol", "aBool", typed("bool", "int")),
			field("arel", "aReal", typed("real", "real")),
			field("aunr", "anUnreal", typed("unreal", "unreal")),
			field("astr", "aString", typed("string", "string")),
			field("alst", "aList", list("unitList")),
			field("alvi", "levelInt", perLevel(nil)),
			field("alvb", "levelBool", perLevel(typed("bool", "int"))),
			field("alvr", "levelReal", perLevel(typed("unreal", "unreal"))),
			field("alvs", "levelString", perLevel(typed("string", "string"))),
			field("alvl", "levelList", perLevel(list("targetList"))),
			field("aenu", "anEnum", typed("attackBits", "int")),
			field("abor", "boolAsReal", typed("bool", "real")),
			field("abou", "boolAsUnreal", typed("bool", "unreal")),
			field("abos", "boolAsString", typed("bool", "string")),
			field("abon", "boolWithoutAStorage", typed("bool", "")),
			field("alor", "levelBoolAsReal", perLevel(typed("bool", "real"))),
			field("Crs\x00", "chanceToMiss", perLevel(labelled("Chance to Miss"))),
			field("aexc", "excluded", func(meta *objects.FieldMeta) { meta.NotSpecific = []string{"AHhb"} }),
			field("Hhb1", "amountHealed", func(meta *objects.FieldMeta) { meta.Specific = []string{"AHhb"} }),
		},
		"buffs": {
			field("fart", "art", func(meta *objects.FieldMeta) {
				meta.Label, meta.Category = "\tArt -\r\n \v\fIcon  ", " art\n\tand sound "
			}),
			field("bzzz", "zeta", nil), field("baaa", "alpha", nil), field("bmmm", "Mu", nil),
			field("bmu2", "mu", nil), field("bund", "_under", nil), field("bdig", "a1", nil),
		},
		"upgrades": {field("gnam", "name", perLevel(typed("string", "string")))},
	}
}

// fieldsWithUnusableNames is fields with every kind of name that no property can have, in every list: names that
// two and four fields of a module share, one of them with an id that is quoted with escapes; keywords and reserved
// names; and names that are no identifier, an empty one and one with a letter outside ASCII among them.
func fieldsWithUnusableNames() map[string][]objects.FieldMeta {
	return map[string][]objects.FieldMeta{
		"units": {
			field("uaaa", "same", used("hero")), field("ubbb", "same", used("hero", "unit")),
			field("u2nd", "2nd", used("unit", "building")),
		},
		"items": {field("iout", "output", used("item")), field("idsh", "a-b", used("item"))},
		"abilities": {
			field("acls", "class", nil), field("anon", "", nil),
			field("Crs\x00", "import", labelled("Chance to Miss")),
		},
		"buffs": {
			field("fccc", "same", labelled("Third")), field("Crs\x00", "same", labelled("First")),
			field("fbbb", "same", labelled("Second")), field("f\"\\\x01", "same", labelled("Quoted <&>")),
			field("flin", "name\n", nil),
		},
		"upgrades": {
			field("gnam", "n\xC3\xA4me", nil), field("gidd", "id", nil),
			field("gfun", "function", labelled("  A  label\twith white space ")),
		},
	}
}
