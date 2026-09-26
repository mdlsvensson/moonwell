# Moonwell Object Data (Plan 2c) — Design

- **Date:** 2026-09-26
- **Status:** Draft, awaiting the maintainer's inputs (§12) and approval
- **Builds on:** `2026-09-24-moonwell-core-design.md`, §§3, 4, 5, 6.1, 9 and 11; `2026-09-25-moonwell-map-settings-design.md`
  (pipeline placement, planner pattern, fixture provenance)
- **Scope:** Custom units, heroes, buildings, items, abilities, buffs and upgrades authored in Pkl and written into the
  staged map's modification files, plus the generated `src/generated/objects.yue`. Target release 0.3.0.

## 1. Summary

Authors define custom objects in Pkl files under `objects/`, in any layout. Moonwell validates them against metadata
generated from the game's own SLKs, appends them to the staged map's modification files (`war3map.w3u`, `.w3t`,
`.w3h`, `.w3a`, `.w3q` and their `war3mapSkin.*` counterparts), and writes `src/generated/objects.yue`, which maps each
object's key to its integer id for gameplay code. Objects that World Editor already saved in the source map, custom or
modified standard, are preserved byte for byte. The source map is never written.

Success means: every rule in core §6.1 works through `build`, `test`, `check` and `dev`; Moonwell's output for a set of
objects matches what World Editor 3.00 writes for the same objects (§10.3); invalid objects fail before anything is
compiled or staged, with errors that name the file, the object and the field; and the manual gate (§11) shows each
object kind working in the game and in World Editor.

**Lessons from Plan 2b.** The map settings gate found three bugs that unit tests could not: colours are stored blue,
green, red, alpha; the hero level key is `MaxHeroLevel`; and a Lua call World Editor never writes crashed the game.
All three were claims from memory. This design therefore marks every format claim not yet confirmed against a World
Editor save as **to verify** (collected in §4.6), and the plan captures and studies the fixtures (§12) before any
reader, writer or generator code is written. Community documentation of the modification-file format is used only
as a starting hypothesis.

Decisions made while writing this design are marked **Decision:** with a one-line rationale. Questions for the
maintainer are marked **Question:** and collected in §14.

## 2. Approach

A metadata-driven port, as core §6.1 describes, with three layers:

1. **Metadata** (`cli/data/metadata.json`): generated once per game patch by the repo-only `gen:metadata` task from
   SLKs the maintainer extracts, committed, and embedded into the CLI.
2. **Schema**: `deno task gen` renders typed Pkl property classes from the same metadata into `schema/generated/`, so
   editors complete and type-check field names; hand-written classes in `schema/objects/` extend them.
3. **Planner**: the CLI resolves each evaluated object against the metadata and the source map's existing tables,
   validates the whole manifest, and computes new file bytes by appending to the existing tables without re-encoding
   anything World Editor wrote.

Alternatives considered:

- **Vendor a JSON copy of the reference framework's object tables** (from npm `war3-objectdata-th`). Rejected: it is an
  npm artefact, its provenance and patch level are unclear, and it would repeat the "from memory" risk.
- **Re-serialize whole modification files from a parsed model.** Simpler writer, but any field Moonwell misreads (an
  unknown v3 set flag, a new var type) would be silently corrupted. Appending to verbatim bytes (§6.3) confines the
  risk to Moonwell's own objects, as the map-info patcher does for `w3i`.
- **Emit `BlzSetUnit*` calls at runtime instead of object data.** Covers only a fraction of fields, runs after World
  Editor's preplacement, and leaves the Object Editor view of the built map empty.

## 3. Metadata

### 3.1 Inputs and generator

`deno task gen:metadata <folder> <game version>` (new `tools/gen-metadata.ts`, repo-only like `gen:game-paths`) reads
the files listed in §12.1 from a local folder that keeps the game's relative paths, and writes
`cli/data/metadata.json`. The SLKs themselves are never committed.

**Decision:** the generator takes the folder as an argument, like `gen:game-paths`; rationale: no machine-specific path
in the repo, and the same command works after a game patch.

The SLK reader (`tools/metadata/slk.ts`) handles the SYLK subset the game files use: `ID`, `B` (bounds), `C` records
with `X`/`Y`/`K` (cell, with the last `Y` carried forward), quoted strings, and `E`. It ignores `F`, `P` and other
formatting records. The first row is the header. The `.txt` reader handles `[section]` / `Key=value` INI files, used for
`WorldEditStrings.txt` and the per-race strings files.

The generator:

- Reads `UnitMetaData`, `AbilityMetaData`, `AbilityBuffMetaData` and `UpgradeMetaData`. Each row becomes a field
  (§3.2). Doodad and destructable metadata are not read.
- Resolves each field's `displayName` (a `WESTRING_*` key) through `WorldEditStrings.txt` into its World Editor label,
  and derives the friendly name from it (§3.3).
- Reads the base-object id lists: `UnitData` (with `UnitBalance` for the building flag), `ItemData`, `AbilityData`
  (with its level count), `AbilityBuffData` and `UpgradeData` (with its level count). Base names for doc comments and
  error suggestions come from the English strings files (§12.1).
- Classifies base units: an id whose first character is an uppercase letter is a hero (the rule the game's
  `IsHeroUnitId` applies); otherwise a unit with the building flag is a building; otherwise a unit. **To verify** (V12):
  the building column in `UnitBalance` and that no standard non-hero id starts with an uppercase letter.
- Loads the previous `metadata.json`, if any, and fails when a friendly name that existed before would change or
  disappear, unless `tools/metadata/overrides.json` pins it. Friendly names are public API once released.
- Writes JSON sorted by category and field id, with a header recording the game version, so regenerating on the same
  input is byte-identical.

### 3.2 `metadata.json` shape

```json
{
  "format": 1,
  "game": "3.0.0.24268",
  "fields": {
    "units": [
      {
        "id": "uhpm", "name": "hitPointsMaximumBase", "label": "Hit Points Maximum (Base)", "category": "stats",
        "type": "int", "storage": "int", "list": false, "perLevel": false, "column": 0, "skin": false,
        "use": ["unit", "hero", "building"], "specific": [], "notSpecific": []
      }
    ],
    "items": [], "abilities": [], "buffs": [], "upgrades": []
  },
  "bases": {
    "units": { "hfoo": { "name": "Footman" } }, "heroes": {}, "buildings": {}, "items": {},
    "abilities": { "AHhb": { "name": "Holy Light", "levels": 3 } }, "buffs": {}, "upgrades": {}
  }
}
```

Per field:

| Key | Meaning | Source (to verify, V11) |
| --- | --- | --- |
| `id` | Field rawcode written into modification files | `ID` |
| `name`, `label` | Friendly name (§3.3) and World Editor label | `displayName` via `WorldEditStrings.txt` |
| `category` | World Editor category (art, combat, stats, text, ...) | `category` |
| `type` | Metadata type (`int`, `real`, `unreal`, `bool`, `string`, `unitList`, `modelFile`, ...) | `type` |
| `storage` | Var type written to the file: `int`, `real`, `unreal` or `string` | derived from `type` |
| `list` | Comma-separated list type (`*List` types) | derived from `type` |
| `perLevel`, `column` | Per-level field; data column 0 (none) or 1–9 (A–I) | `repeat`, `data` |
| `skin` | Written to the `war3mapSkin.*` file | see below |
| `use` | Unit-metadata applicability: `unit`, `hero`, `building`, `item` | `useUnit`, `useHero`, `useBuilding`, `useItem` |
| `specific`, `notSpecific` | Base ids the field is limited to, or excluded from | `useSpecific`, `notSpecific` |

**Item fields.** Items use `w3t` but, per community documentation, their fields are the `useItem` rows of
`UnitMetaData`. The generator places those rows under `fields.items`. **To verify** (V11).

**Skin flag.** Core §11 left open which column identifies skin fields. The generator takes it from a metadata column if
the 1.32+ SLKs carry one; otherwise from a committed list in `tools/metadata/overrides.json`. Either way, a unit test
checks the flag against the fixture: every field World Editor wrote to a `war3mapSkin.*` file must be `skin: true`, and
every field it wrote to a main file must be `skin: false` (§10.3). **To verify** (V5, V11).

**Decision:** `storage` maps `bool` to `int` and every non-numeric type to `string`; rationale: that is the community
description of the four var types, and V7 confirms it against the fixture before the writer relies on it.

### 3.3 Friendly names

A field's friendly name is its World Editor label in lower camel case, without punctuation: "Hit Points Maximum
(Base)" becomes `hitPointsMaximumBase`. Names are unique within each category's Pkl class (§4.2). When two fields of a
class share a name, both get their category prefixed (`artModelFile`); a remaining clash appends the rawcode. Names
that are Pkl keywords, or that clash with the reserved object properties `id`, `base`, `source` and `properties`, need
an override. Overrides and every automatic rename are listed by the generator.

**Decision:** labels, not SLK `field` column names; rationale: the SLK names are ambiguous (the ability data columns are
all `Data`, and `Buttonpos` is two fields told apart by `index`), while labels are what authors see in the Object
Editor. See Question Q2.

Ability-specific fields (the `useSpecific` rows: Holy Light's "Amount Healed/Damaged", and so on) get friendly names too,
unique among the fields that apply to the same base ability. They are addressed through `properties` (§4.3).

### 3.4 Embedding

`deno task gen` embeds `metadata.json` gzip-compressed as `cli/src/embedded/metadata.ts`, like `game-paths.ts`, and
renders `schema/generated/*.pkl` from it (§4.2). The existing freshness test covers both. Until the maintainer's SLKs
arrive, `metadata.json` does not exist; plan tasks before the generator task use hand-written metadata in test
support.

**Decision:** fold core's `gen:schema` into `deno task gen`; rationale: the Pkl classes are derived from a committed
file exactly like the embedded modules, so one command and one freshness test keep them in step.

## 4. Manifest contract

### 4.1 Wiring

`schema/Project.pkl` gains `objects: Objects = new {}`. The template's `moonwell.pkl` writes the wiring out (core §3.3):

```pkl
import "@moonwell/Objects.pkl"
objects = Objects.merge(import*("objects/**.pkl"))
```

Each file under `objects/` amends `@moonwell/ObjectFile.pkl`. `import*` keys each module by its path relative to the
manifest (`objects/heroes/paladin.pkl`), which becomes each object's `source`. Missing `objects` in evaluated JSON means
no objects, so a 0.2 manifest keeps working. **To verify** in `test:pkl` (Pkl 0.32, not the maintainer): the glob
matches nested folders; a missing or empty `objects/` folder yields an empty mapping; `moonwell.local.pkl` amending
`moonwell.pkl` resolves the glob relative to `moonwell.pkl`. The exact Pkl shape of `Objects.merge` (module amend versus
a result class) is settled by the plan's first Pkl test; the wiring lines above are the public contract.

### 4.2 Schema

```
schema/
  ObjectFile.pkl           the seven optional category mappings
  Objects.pkl              merge(files): duplicate-key detection, `source` recording
  objects/                 Unit.pkl Hero.pkl Building.pkl Item.pkl Ability.pkl Buff.pkl Upgrade.pkl
  generated/               UnitProps.pkl HeroProps.pkl BuildingProps.pkl ItemProps.pkl
                           AbilityProps.pkl BuffProps.pkl UpgradeProps.pkl   (from metadata.json, committed)
```

`ObjectFile.pkl` declares `heroes`, `units`, `buildings`, `items`, `abilities`, `buffs`, `upgrades`, each
`Mapping<Key, T> = new {}`, where `Key` matches `[A-Za-z_][A-Za-z0-9_]*` and is not a Lua or YueScript keyword.

**Decision:** reject keyword keys (`end`, `and`, `class`, ...) in Pkl; rationale: `objects.units.end` cannot be written
in gameplay code, and quoting keys in `objects.yue` would only move the problem to the caller.

Every object class has:

- `id`: four ASCII letters or digits. Heroes must start with an uppercase letter; units and buildings must not (the
  game treats uppercase-first unit ids as heroes). Items, abilities, buffs and upgrades take any four alphanumerics.
- `base: String(length == 4)`: the standard object it copies.
- the typed properties of its generated `*Props` class: every field of that category, typed `Int`, `Float`/`Number`,
  `Boolean` or `String`; list fields also accept `List<String>`; per-level fields also accept a `List` of their type
  (and list fields `List<List<String>>`). Doc comments carry the World Editor label, the rawcode, the category and
  whether the field is per-level or a skin field.
- `properties: Mapping<String, Value> = new {}`: the escape hatch, keyed by friendly name or field rawcode.

Unit, hero and building classes get the fields whose `use` includes `unit`, `hero` or `building` respectively. Ability,
buff and upgrade classes get only the fields without `specific` restrictions; ability-specific data fields go through
`properties` (Question Q1).

**Decision:** `base` is a constrained `String`, not the generated `Bases.pkl` union core §6.1 describes; rationale: a
Pkl type error over a union of several hundred literals prints all of them, whereas the CLI can report the one or two
nearest ids with their names. Bases remain listed in the README-linked docs and in `objects:eval` errors.

**Decision:** lists are Pkl `List`, never `Listing` (AGENTS.md hard rule), so a written-out list replaces rather than
appends.

`Objects.merge(files)` throws a Pkl error naming both files when two files define the same key in the same category,
and returns the merged mappings with each object's `source` set. Mapping values are type-checked lazily, so the Pkl
tests force them with `.toMap()` (AGENTS.md).

### 4.3 Values

As core §6.1, restated with the level rules:

- A scalar sets the field, at level 1 for a per-level field.
- A `List` sets levels 1..n of a per-level field; later levels inherit from the base. A `List` on a field that is not
  per-level is an error, except `List<String>` on a list field, which is joined with commas.
- On a per-level list field, `List<String>` is one level's list and `List<List<String>>` sets levels.
- `null` (or omitting the property) inherits from the base. Explicit `false`, `0`, `""` and empty lists are overrides.
- `Boolean` is written as `1` or `0`.
- Strings are written literally; `TRIGSTR_` references are not created and `war3map.wts` is never changed (as map
  settings, §4 of that design).

`properties` keys resolve in this order: an exact field rawcode of the category; else a friendly name among the fields
that apply to the object's base. Setting the same field through a typed property and `properties`, or through both a
rawcode and a friendly name, is an error.

## 5. Validation

### 5.1 Stages

1. **Pkl**, at evaluation: key and id patterns, `base` length, property types, duplicate keys across files. Pkl errors
   pass through verbatim (core §8), since Pkl names the file and line.
2. **Shape**, in `parseProject`: the evaluated JSON is checked defensively (`cli/src/objectdata/manifest.ts`), like
   `validateMapSettings`. A mismatch means a schema/CLI version problem and uses the existing "Is the moonwell Pkl
   package the version this CLI expects?" hint.
3. **Semantics**, in the planner, over the whole manifest and the source map's tables, before anything is written:
   - an id that is not four ASCII alphanumerics (Pkl checks this too; the CLI repeats it defensively);
   - a duplicate id across all categories;
   - a `base` that is not a standard object of the object's category, with up to three nearest ids (edit distance,
     then shared prefix) and their names;
   - an id that equals any standard object id of any category, or any custom id already in the source map's tables;
   - an unknown `properties` key (with the nearest friendly names), or a field that does not apply to the base
     (`use`, `specific`, `notSpecific`);
   - a `List` on a field that is not per-level;
   - more list entries than the level count: the object's own `levels` value if set, else the base's;
   - a value of the wrong storage type in `properties` (non-integer for `int`, non-finite or out-of-float32-range for
     `real`/`unreal`, NUL in a string).

### 5.2 Error format

Each problem renders as core §6.1 specifies: `<source> › <category>["<key>"].<field>: <problem>`, for example

```text
error: objects/heroes.pkl › heroes["paladin"].base: 'Hpla' is not a standard hero.
hint: Did you mean 'Hpal' (Paladin)?
```

Validation collects every problem instead of stopping at the first. `ObjectDataError extends MoonwellError` carries the
list; `formatError` prints each as its own `error:` line with its own hint, at most 20, then `and N more`. `file` is
the first problem's source, so existing callers keep working.

**Decision:** report all problems; rationale: object files are long and typo-prone, and one build per typo is slow,
while settings have few fields and stop at the first.

Map-file problems (unsupported version, truncation, bad counts, unknown var type, duplicate custom id inside a file,
two files differing only in letter case) name the source-map file, as `maps/map.w3x/war3map.w3a`, with the hint to open
and re-save the map in World Editor 3.00.

## 6. Modification files

`cli/src/objectdata/modfile.ts` reads and writes the simple tables (`w3u`, `w3t`, `w3h`) and the leveled tables
(`w3a`, `w3q`), and their `war3mapSkin.*` counterparts. `w3b`, `w3d` and every other file are copied untouched by
staging. **To verify** (V13): that `w3h` is simple and `w3q` leveled on WE 3.00.

### 6.1 Layout (starting hypothesis, to verify)

The community description, which the plan checks field by field against the fixture before writing code:

```
int32  version                      1, 2 or 3
table  original                     modified standard objects
table  custom                       custom objects
table: int32 count, then count objects
object:
  char[4] originalId                base id
  char[4] customId                  0 in the original table
  v3 only: int32 setCount, then per set: int32 setFlag, modifications
  v1/v2:   modifications
modifications: int32 count, then count of:
  char[4] fieldId
  leveled tables only: int32 level, int32 dataPointer
  int32  varType                    0 int, 1 real, 2 unreal, 3 string
  value                             int32 | float32 | float32 | UTF-8, NUL-terminated
  int32  endToken                   0, or an object id
```

All integers little-endian; ids are the four ASCII bytes in reading order. **To verify:** V2 (version), V3 (sets),
V4 (end token), V6 (level and data pointer numbering), V7 (value encodings).

### 6.2 Reader

The reader returns, per table, each object's ids, its byte span, and its parsed modifications (per set in v3). It
checks bounds, counts, var types and string termination, and rejects trailing bytes. Unknown `setFlag` values are
kept, not interpreted. The reader is also used for the collision check (§5.1) and by the test support.

Versions 1 and 2 are read with the v1/v2 layout. **To verify** (V16): whether they differ at all. No WE 3.00 save can
show this; unless the maintainer supplies an older map (§12.3), v1/v2 fixtures are synthetic, derived from the v3
fixture by removing the set fields, and labelled synthetic like the older `w3i` fixtures.

### 6.3 Writer: append, never re-encode

The new file is: the source file's bytes up to the custom table's count, the incremented count, the existing custom
objects' bytes verbatim, then Moonwell's objects encoded in World Editor's shape. The version and the original table
are untouched. A file with no Moonwell objects is not written.

**Decision:** append to verbatim bytes rather than re-serialize; rationale: an unknown flag or value in an object World
Editor wrote can never be corrupted, and the round-trip property is trivially byte-exact.

Encoding of Moonwell's objects follows the fixture exactly: set count and flag (V3), end token (V4), level and data
pointer numbering (V6), value encodings (V7), and the order of objects and fields (V8). New objects are appended sorted
by id, so the output does not depend on file layout or Pkl evaluation order. The target is byte-identity with World
Editor for the same objects (§10.3); any remaining difference is documented in the fixture README and justified.

### 6.4 Skin split and versions

- **v3 maps** (and new files): each field goes to the main or the skin file by its `skin` flag. An object with only
  main fields, or only skin fields, appears only in the file(s) World Editor would write it to. **To verify** (V5):
  whether World Editor writes a skin entry for every custom object or only for ones with skin fields, and whether the
  skin file's version and set layout equal the main file's.
- **v1/v2 maps**, which predate skin files: all fields go to the main file. **To verify** in the gate only if the
  maintainer supplies such a map; otherwise this path is covered by unit tests alone and documented as such.
- **New files** (the map has none of that table): written at the version World Editor 3.00 writes, with the skin split
  it uses, as observed in the fixture.

**Decision:** new files use World Editor 3.00's version and skin split, not version 2 as core §6.1 proposed; rationale:
that exact shape is what the fixture proves World Editor produces and the current game loads, while "v2 still loads"
is an unverified memory claim.

A map whose main file is v3 but whose skin file is missing, or whose two files have different versions, is written
following the main file's version; the case is covered by a unit test. **To verify** (V1): which files a map with no
objects has, from the `objects-empty.w3x` save.

### 6.5 File names

The planner finds the ten files case-insensitively, as `settingsFileNames` does for settings, and writes to the
existing spelling. Two names differing only in case fail.

## 7. Merge rules with existing map objects

- Everything World Editor saved in the source map stays: modified standard objects (original tables) and custom
  objects (custom tables), in both main and skin files, byte for byte.
- Moonwell only adds custom objects. A Moonwell id equal to an existing custom id in any of the source map's tables is
  an error, with the hint to change the Pkl id or delete the object in World Editor.
- Moonwell does not modify standard objects, and does not edit or remove objects made in World Editor.

**Decision:** Moonwell objects cannot modify standard objects in 0.3.0; rationale: it would need field-level merging
with World Editor's own edits of the same object and precedence rules, which deserve their own design. See Question Q3.

Because every build restages from the source map, output is deterministic and never accumulates objects.

## 8. Generated ids

`src/generated/objects.yue` exports one table per category, in the fixed category order, keys sorted, each id the
rawcode packed big-endian (matching `FourCC`), with the rawcode in a comment:

```yue
-- GENERATED by Moonwell from objects/**.pkl; do not edit.
export heroes = {}
export units = {
  captain: 1747988528 -- h000
}
export buildings = {}
export items = {}
export abilities = {}
export buffs = {}
export upgrades = {}
```

- `build`, `test` and `dev` rewrite it before compiling, only when its content changes (`writeTextIfChanged`).
- `check` never writes it. It fails when the file differs from what the manifest produces, or is missing while the
  manifest has objects, with the hint `run deno task build, test or dev to regenerate it`. A project with no objects
  and no file passes, so 0.2 projects keep passing `check`.
- `dev` already ignores `src/generated/` changes, so its own rewrite does not retrigger it.

**Decision:** `build`/`test`/`dev` write the file and `check` only compares; rationale: core §6.1 requires check to fail
when stale, and the commands that already write outputs are the natural place to refresh a committed file.

## 9. Pipeline and commands

### 9.1 Planning and pipeline

`planObjectData(mapDir, objects, { manifest, sourceLabel }): Promise<ObjectPlan>` is async and read-only. With no
objects it returns an empty plan and the empty `objects.yue` text without reading the map (the map folder is not even
required). Otherwise it reads the source map's modification files, validates (§5), and returns
`{ changes: { name, bytes }[], generated: string, objects: ResolvedObject[] }`, with `name` relative to the map folder
and changed files in a stable order (`w3u`, `w3t`, `w3h`, `w3a`, `w3q`, then the skin files). `applyObjectPlan(plan,
stagedDir)` writes the bytes into the staged folder.

Updated `prepareStage` order (core §5 and map settings §7, which reserved this slot):

1. Lock, load project (unchanged).
2. **Plan object data against `maps/<folder>`** and write `objects.yue` if changed. Invalid objects fail here, before
   compiling.
3. Compile Yue and resolve the graph (unchanged; the generated module is now current).
4. Stage the source map (unchanged).
5. **Apply the object plan** to the staged copy.
6. Map settings, then assets, then bundle injection, then pack or launch (unchanged).

**Decision:** plan once against the source map, before compiling, and apply the same bytes after staging; rationale:
errors surface before the slow steps, and staging copies the same files, so the plan is valid for the staged copy. A
source map re-saved by World Editor between steps 2 and 4 is overwritten in the staged copy by the planned bytes, which
is acceptable because the next build replans.

Object data never touches `war3map.w3i`, `war3map.lua`, the `.txt` files or `war3map.imp`, so it cannot conflict with
settings or assets. The packer already includes every staged file, so the new skin files are packed without change.

### 9.2 Commands

| Command | Behaviour |
| --- | --- |
| `build`, `test` | As above. Log `Added N custom object(s) to M file(s).` when N > 0. |
| `check` | Runs the planner against the source map (read-only) and the staleness check (§8). |
| `dev` | Also watches `objects/` recursively; each cycle refreshes `objects.yue`, then runs `check`. |
| `objects:eval` | Core §4: prints the validated, resolved manifest as JSON on stdout (logs stay on stderr): per object its key, id, base, source and resolved fields (rawcode, friendly name, level, column, skin, value). No Yue, no lock. |
| `objects:check` | New, like `settings:check`: lists the modification files that would change and whether `objects.yue` is current, then `Object data valid: N object(s), M internal file(s) would change during build.` Read-only, no Yue, no lock; exits 1 when invalid or stale. |

**Decision:** add `objects:check`; rationale: it mirrors `assets:check` and `settings:check`, so each data layer has one
fast, read-only command, and it is what `objects.yue` staleness errors can point to.

Both new commands join dispatch, the help text, `PROJECT_TASKS` and the template's `deno.json`; `objects:eval` needs a
stdout writer next to the stderr logger in `main`.

### 9.3 Components

| File or area | Responsibility |
| --- | --- |
| `tools/gen-metadata.ts`, `tools/metadata/slk.ts`, `txt.ts`, `overrides.json` | SLK/INI reading, metadata generation, name stability |
| `cli/data/metadata.json`, `cli/src/embedded/metadata.ts` | Committed metadata and its embedded copy |
| `tools/gen.ts` | Also renders `schema/generated/*.pkl` |
| `schema/ObjectFile.pkl`, `Objects.pkl`, `objects/`, `generated/`, `Project.pkl` | Public schema and wiring |
| `cli/src/objectdata/metadata.ts` | Loading the embedded metadata; lookups by category, rawcode, friendly name, base |
| `cli/src/objectdata/manifest.ts` | Evaluated-JSON shape check, `Project.objects` type |
| `cli/src/objectdata/resolve.ts` | Friendly names and `properties` to fields, validation (§5), `ObjectDataError` |
| `cli/src/objectdata/modfile.ts` | Reader and appending writer (§6) |
| `cli/src/objectdata/plan.ts` | File discovery, skin split, planning and applying |
| `cli/src/objectdata/ids.ts` | Rawcode packing, `objects.yue` rendering |
| `cli/src/pipeline.ts`, `commands/check.ts`, `dev.ts`, `objects-eval.ts`, `objects-check.ts`, `main.ts`, `project-files.ts` | Integration |
| `cli/src/shared/errors.ts` | Multi-problem rendering |
| Template, embedded output, README, CONTRIBUTING, CHANGELOG, AGENTS.md | User-facing workflow |

## 10. Testing

### 10.1 Fixtures and provenance

`cli/tests/fixtures/objects-v3/` holds the modification files, `war3map.w3i` and `war3map.wts` from the maintainer's
World Editor 3.00 save (§12.2), copied unchanged, plus a `README.md` in the style of
`cli/tests/fixtures/map-settings-v39/README.md`: who saved it, when, with which game and editor version, the SHA-256 of
every file, the list of objects and every value exactly as entered in the Object Editor (the maintainer's own record,
which tests assert against; never the reader's output), and the answers to V1–V13 with byte offsets. The listing of the
`objects-empty.w3x` save is recorded there too. The folder joins `fmt.exclude` like the settings fixture.

### 10.2 Unit tests (`deno task test`; need neither Pkl nor yue)

- SLK and INI readers on hand-written excerpts (carried-forward `Y`, quoted strings, `E` records, missing cells).
- Generator on a hand-written miniature game folder: field records, friendly names, collision renames, overrides, base
  classification, level counts, and the name-stability check against a previous `metadata.json`.
- Invariants of the committed `metadata.json`: unique rawcodes per category, unique friendly names per class, valid
  storage types and columns, and the fixture's skin split (§3.2).
- Reader: the fixture's objects and values as recorded in the README; golden offsets; each malformed input (truncation,
  bad count, unknown var type, missing NUL, trailing bytes, unsupported version); synthetic v1/v2.
- Writer: appending to every fixture file leaves the prefix and existing objects byte-identical and re-reads cleanly;
  new files; skin split; per-level entries and data columns; objects sorted by id.
- **Known answer:** Pkl-equivalent definitions of the fixture's custom objects (as resolved objects, so no Pkl needed),
  planned against the `objects-empty` save, produce byte-identical files to World Editor's (§6.3).
- Validation: every rule in §5.1, each with the exact message and hint; multi-problem rendering and its cap.
- Planner: read-only on success and failure (source snapshots); empty manifest needs no map; case-insensitive names.
- `objects.yue` rendering, packing (`h000` equals `string.unpack(">I4", "h000")`), key sorting, and staleness logic.
- Pipeline order (extends `pipeline.test.ts`) and `dev` watching `objects/`.

### 10.3 Other suites

- **`test:pkl`:** `schema/tests/Objects.pkl` for id rules per category, key and keyword rules, value and list types,
  `properties`, merge duplicates naming both files, and the `import*` behaviour in §4.1 (forcing mappings with
  `.toMap()`). CLI tests in `cli/tests/pkl/objects.test.ts` run `objects:eval` and `objects:check` in an `init --link`
  project, including a nested `objects/` layout and a project without the folder.
- **`test:runtime`:** the generated `objects.yue` compiles, and under `yue -e` each id equals a Lua `FourCC` stub.
- **`test:e2e`:** `init`, build twice, read the archive with the test MPQ reader, parse the modification files with
  the reader, and assert the template's Captain, a skin field in `war3mapSkin.w3u`, identical output on both builds,
  an unchanged source map and a current `objects.yue`.

All checks in AGENTS.md must pass before each commit. Automated tests do not replace the in-game gate.

## 11. Template, documentation and release gate

**Template.** `template/objects/units.pkl` restores the original footman-as-Captain:

```pkl
amends "@moonwell/ObjectFile.pkl"

units {
  ["captain"] {
    id = "h000"
    base = "hfoo"
    name = "Captain"
    modelFile = "<the Captain model path exactly as World Editor stores it, from the fixture>"
  }
}
```

(Field names are placeholders until the metadata exists.) `template/src/main.yue` imports `generated.objects` and
creates `objects.units.captain` instead of `FourCC("hfoo")`. `template/src/generated/objects.yue` is committed and
must be current (e2e checks it). `moonwell.pkl` gains the two wiring lines with a comment pointing to `objects/`.
Every new template file is embedded by `deno task gen`; nothing else may be left in `template/`.

**Docs.** README: authoring objects, file layout, values and levels, `properties`, errors, `objects:check`,
`objects:eval`, what is preserved from World Editor, and the out-of-scope list. CHANGELOG: an Unreleased entry.
AGENTS.md: state and next work.

**Manual release gate**, a new CONTRIBUTING step 9 (the current step 9 becomes 10), in a throwaway `init --link`
project:

1. Run `deno task test`. Confirm the unit north of the heroes is the Captain (model and name) and still changes colour.
2. Add `objects/gate.pkl` with: a hero based on the Paladin with a custom name and starting strength, whose hero
   abilities include a custom ability based on Holy Light with `levels = 4` and a per-level heal amount
   (`List(111, 222, 333, 444)`) and per-level cooldown; that ability's custom buff with a new icon; a custom item with
   a new name, gold cost and icon; a custom building based on the Blacksmith that researches a custom two-level upgrade
   with per-level names. In `main.yue`, create the hero, the building and the item for player 0, and give the hero
   enough experience to reach level 4.
3. In game: the hero shows its name and strength; learning the ability shows the level 1 to 4 tooltips; healing a unit
   heals 111, 222, 333 and 444 at the four levels; the target shows the custom buff icon; the item shows its name and
   price; the building offers the upgrade with both level names.
4. In World Editor, first modify a standard object (the Footman's hit points) in the throwaway project's source map
   and save. Build again (`deno task build --minify`), play `dist/bin/map.w3x` and confirm the Footman change survived
   next to the Moonwell objects.
5. Open the packed map in World Editor: the Object Editor lists every custom object under Custom, with the configured
   values, including the skin fields (model, icons). Confirm `maps/map.w3x` is unchanged apart from step 4's edit and
   that a second build leaves `src/generated/objects.yue` unchanged.

## 12. Inputs needed from the maintainer

These block the plan. The plan's first task records them in the repo (§10.1) and answers V1–V13 before any other code.

### 12.1 Game files, exported with CascView

From Warcraft III Reforged 3.0.0.24268 (`C:\Program Files (x86)\Warcraft III`). Paths below are as CascView's file
list names them; the tree may show `war3.w3mod` and `enus.w3mod` as folders. If a file is not at its path, search
CascView for the file name and note where it was.

Export these, keeping the folder structure, to `C:\Users\mdlsvensson\moonwell-game-data\3.0.0.24268\` (outside the
repo; the files are Blizzard's and are never committed):

| In-game path | Used for |
| --- | --- |
| `war3.w3mod:units\unitmetadata.slk` | Unit, hero, building and item fields |
| `war3.w3mod:units\abilitymetadata.slk` | Ability fields |
| `war3.w3mod:units\abilitybuffmetadata.slk` | Buff fields |
| `war3.w3mod:units\upgrademetadata.slk` | Upgrade fields |
| `war3.w3mod:units\unitdata.slk` | Base unit ids |
| `war3.w3mod:units\unitbalance.slk` | Building flag |
| `war3.w3mod:units\itemdata.slk` | Base item ids |
| `war3.w3mod:units\abilitydata.slk` | Base ability ids and level counts |
| `war3.w3mod:units\abilitybuffdata.slk` | Base buff ids |
| `war3.w3mod:units\upgradedata.slk` | Base upgrade ids and level counts |
| `war3.w3mod:_locales\enus.w3mod:ui\worldeditstrings.txt` | Field labels (friendly names) |
| `war3.w3mod:_locales\enus.w3mod:units\` (whole folder) | Base object names for docs and suggestions |

Simplest: export the whole `war3.w3mod:units\` folder (it also holds `unitui.slk` and the `*skin.txt`/`*func.txt`
files, which help answer V11) plus the two `enus.w3mod` items. Tell the agent the folder path and whether any file was
elsewhere.

### 12.2 World Editor test map

In World Editor 3.00:

1. File → New Map: 32×32, any tileset. Scenario → Map Options: Lua script. Save in folder format as
   `objects-fixture.w3x`. Copy that folder as `objects-empty.w3x` before opening the Object Editor.
2. In the Object Editor, turn on View → Display Values as Raw Data (Ctrl+D) while recording values, and create the
   objects below with "New Custom ...", keeping the ids World Editor proposes. Write down each object's id and every
   value exactly as entered, with the field's label and rawcode (this record becomes the fixture README and the test
   expectations).

| Tab | Object | Fields to set |
| --- | --- | --- |
| Units | Custom unit from Footman (`hfoo`) | Name `Moonwell Footman`; Hit Points Maximum (Base) `1234`; Attack 1 Damage Base `17`; Normal abilities: add one ability (list field); **Model File**: the Captain's model; **Icon - Game Interface**: the Captain's icon; **Scaling Value** `1.25` |
| Units | Custom hero from Paladin (`Hpal`) | Name `Moonwell Paladin`; Proper Names `Arthas,Uther`; Primary Attribute Agility; Starting Strength `30`; Strength per Level `3.5`; Hero abilities: include the custom ability below; **Model File**: the Archmage's |
| Units | Custom building from Blacksmith (`hbla`) | Name `Moonwell Smithy`; Hit Points `777`; Researches Available: only the custom upgrade below; **Model File**: the Scout Tower's |
| Items | Custom item from Claws of Attack +15 (`ratf`) | Name `Moonwell Claws`; Gold Cost `123`; Level `7`; **Interface Icon**: any other icon |
| Abilities | Custom ability from Holy Light (`AHhb`) | Name `Moonwell Light`; Levels `4`; Amount Healed/Damaged levels 1–4 `111, 222, 333, 444`; Cooldown levels 1–4 `1.5, 2.5, 3.5, 4.5`; Tooltip - Normal level 2 only `Level two`; Targets Allowed level 1 `air,ground`; Buffs: the custom buff below; **Icon - Normal** and **Art - Target** changed |
| Buffs | Custom buff from Inner Fire (`Binf`) | Tooltip `Moonwell buff`; **Icon** and **Art - Target** changed |
| Upgrades | Custom upgrade from Iron Forged Swords (`Rhme`) | Levels `2`; Name levels 1–2 `Moonwell I`, `Moonwell II`; Gold Base `77`; **Icon** per level (two different icons) |
| Units | Standard Footman (`hfoo`), modified | Hit Points Maximum (Base) `999`; **Icon - Game Interface** changed |
| Abilities | Standard Holy Light (`AHhb`), modified | Amount Healed/Damaged level 1 `55` |

Bold fields are expected to be skin fields; the fixture decides (V5). Items marked as lists or per-level exercise
V6 and V7.

3. Save. Deliver both folders (`objects-fixture.w3x`, `objects-empty.w3x`) and the written record, e.g. by copying
   them to a scratch folder next to the repo.

### 12.3 Optional

Any map saved by an older World Editor (before 1.32) that has custom objects. It would give a real v1/v2 fixture
(V16); without it those versions are covered by synthetic fixtures only.

## 13. To verify against the fixture

Answered in the fixture README by the plan's first task, with offsets; each answer is then pinned by a unit test.

- **V1** Which of the ten files exist in `objects-empty.w3x` and in `objects-fixture.w3x`.
- **V2** The version each file carries (expected 3).
- **V3** v3 per-object sets: set count, flag values, whether any object has more than one set.
- **V4** The end token after each modification: 0, the base id or the custom id; the same in both tables?
- **V5** Which fields land in the skin files; whether an object with no skin fields still gets a skin entry; whether
  the skin files share the main files' version and layout; whether modified standard objects split the same way.
- **V6** Level numbering in leveled files (0 for unleveled fields, levels from 1) and data pointer values (A = 1?).
- **V7** Var types used for bool, list, unreal and model fields; list separator and quoting; string encoding; whether
  typed strings are stored literally or as `TRIGSTR_` references into `war3map.wts`.
- **V8** Order of objects within a table and of modifications within an object.
- **V9** How a changed level count is stored, and how per-level values beyond the base's levels appear.
- **V10** Custom id bytes in the original table (expected zero).
- **V11** Metadata SLK column names and meanings (§3.2), the skin marker, and whether item fields come from
  `UnitMetaData`.
- **V12** The building flag column and the uppercase-hero rule over all standard unit ids.
- **V13** `w3h` simple, `w3q` leveled, and how upgrade effect fields use the level and data pointer.
- **V14** (gate) The game and World Editor accept Moonwell's files (§11).
- **V15** (`test:pkl`) `import*` behaviour (§4.1).
- **V16** Whether v1 and v2 differ (only with §12.3's map).

## 14. Questions for the maintainer

- **Q1** Ability-specific data fields ("Amount Healed/Damaged" and similar) are set through `properties` by friendly
  name or rawcode, validated by the CLI, without editor completion. Is that acceptable for 0.3.0, or should the
  generator also emit a typed class per base ability now (several hundred small classes)?
- **Q2** Friendly names come from World Editor labels (`hitPointsMaximumBase`, `modelFile`) rather than SLK column
  names (`HP`, `file`). Agreed?
- **Q3** Modifying standard objects from Pkl (for example changing the Footman itself) is out of scope for 0.3.0;
  World Editor edits to them are preserved. Agreed, or is it needed in this plan?

## 15. Implementation order

1. Maintainer inputs (§12). Commit the fixtures with their README and answer V1–V13. Revise this design where the
   answers contradict it, and get approval for the revision before continuing.
2. Modification-file reader, then the appending writer, test-first against the fixture (§6).
3. SLK/INI readers and `gen:metadata`; generate and commit `metadata.json`; embed it.
4. Schema generation in `deno task gen`; `ObjectFile.pkl`, `Objects.pkl`, object classes, `Project.pkl` wiring; Pkl
   tests.
5. Manifest shape check and `Project.objects`; resolution and validation with multi-problem errors.
6. Planner, skin split, `objects.yue` rendering; the known-answer test.
7. Pipeline, `check`, `dev`, `objects:eval`, `objects:check`.
8. Template, embedded output, README, CHANGELOG, CONTRIBUTING gate, AGENTS.md; e2e.
9. Maintainer runs the release gate (§11). Version bump to 0.3.0 follows CONTRIBUTING's Publishing steps separately.

## 16. Out of scope

- Modifying standard objects from Pkl (Q3); editing or removing objects made in World Editor.
- Doodads and destructables (`w3d`/`w3b`), which are preserved as-is.
- `TRIGSTR_` creation and `war3map.wts` changes; localized field labels.
- Custom objects based on other custom objects; placing units; per-unit skins (`BlzCreateUnitWithSkin` skin ids).
- Importing `.w3o` exports or reading packed `.w3x` maps.
- Value ranges from the metadata (`minVal`/`maxVal`) and enum types for metadata types such as `attackType`; they stay
  typed as their storage type in 0.3.0.
- Compile-time rawcode macros (sub-project 3) and typed native wrappers (sub-project 4).
