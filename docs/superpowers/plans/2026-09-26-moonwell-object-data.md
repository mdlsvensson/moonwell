# Moonwell Object Data (Plan 2c) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. The maintainer chose subagent-driven execution: one implementer and one independent review per task.

**Goal:** Author custom units, heroes, buildings, items, abilities, buffs and upgrades in Pkl under `objects/`, validate
them against metadata generated from the game's SLKs, append them to the staged map's modification files, and generate
`src/generated/objects.yue` with their ids.

**Architecture:** Three layers as the spec describes: committed `metadata.json` generated from the game files, Pkl
classes generated from it, and a read-only planner that appends Moonwell's objects to World Editor's verbatim bytes.
The planner runs before compiling (errors surface early) and its bytes are applied to the staged copy.

**Tech Stack:** Deno 2.9+, TypeScript, existing `jsr:@std/*` imports, Pkl 0.32+, YueScript for runtime tests.

**Spec:** `docs/superpowers/specs/2026-09-26-moonwell-object-data-design.md` (approved 2026-09-26, Q1–Q3 answered),
plus the binding core design §6.1 and the map settings design §7 (pipeline slot).

## Global Constraints

- No Node.js, npm packages, `package.json`, or `node_modules`; use the existing import maps only.
- All filesystem operations are async. Expected failures use `MoonwellError` with `file` and `hint`.
- Commit directly on `main` and push; inspect CI for the pushed commit afterward. Do not create a PR.
- Every code task follows red → green → review. Fix review findings and rerun affected tests before advancing.
- Before every commit run the full gate (see Completion). Do not commit a knowingly failing checkpoint.
- The game files (SLKs, strings) are Blizzard's: never commit them, never copy excerpts longer than a test needs.
  Hand-written miniature excerpts in tests are fine. The maintainer's export lives outside the repo; in the cloud
  session it is at `/tmp/gamedata/` (from `/mnt/project-files/moonwell/game-data/game-data-3.0.0.24268.zip`).
- Every binary format claim is pinned by a test against a World Editor save before production code relies on it.
  A claim not yet pinned is written as a named constant with a `// to verify (Vn)` comment and a failing-if-wrong test
  added in the fixture checkpoint (Task 6).
- Pkl lists are `List`, never `Listing`. Force lazy mapping checks with `.toMap()` in Pkl tests.
- Regenerate embedded files after template or metadata changes (`deno task gen`). Nothing scratch in `template/`.
- Do not bump versions or publish.

## What is already known (answers recorded before Task 1)

From the World Editor save in `cli/tests/fixtures/objects-v3-names/` (one custom object per tab, name only) and the
game data export:

| Question | Answer | Evidence |
| --- | --- | --- |
| V1 (empty map) | A map with no objects has none of the ten files. | `template/maps/map.w3x` |
| V1 (with objects) | With an object on every tab, every main and skin file is written. Whether World Editor writes files for empty tabs is still open. | names fixture |
| V2 | Version 3 in every main and skin file. | names fixture, offset 0 |
| V3 | Set count 1, set flag 0 per object. | names fixture |
| V4 | End token 0 after each modification in the custom table. Original table still open. | names fixture |
| V5 (partial) | Names go to the skin file. An object whose fields are all skin fields still appears in the main file with zero modifications, and vice versa. | names fixture |
| V6 (partial) | `w3a` unleveled field: level 0, data pointer 0. `w3q` name: level 1, data pointer 0. | names fixture |
| V7 (partial) | Names typed in World Editor are stored as `TRIGSTR_nnn` into `war3map.wts`. Moonwell writes literal strings (spec §4.3), so string values will differ from World Editor's bytes by design; the known-answer test compares with the TRIGSTR value substituted. | names fixture |
| V11 | Metadata columns match the spec's table. There is no `skin` column; `netsafe` = 1 marks exactly the name, tooltip, hotkey, button position, icon, model and art fields, and every field the names fixture wrote to a skin file has `netsafe` 1. Items: `useItem` rows of `unitmetadata.slk`. Ability level count: `abilitydata.slk` column `levels`; upgrades: `upgradedata.slk` column `maxlevel`. | game data |
| V12 | `unitbalance.slk` has `isbldg`. 123 standard unit ids start with an uppercase letter; the generator must check each is a hero (e.g. via `unitui.slk`/`unitdata.slk` hero markers) before relying on the rule. | game data |

Still open until the full fixture map (spec §12.2) arrives: V4 for the original table, V5 for every non-name field and
for modified standard objects, V6 for per-level data fields, V7 for int/real/unreal/bool/list values, V8, V9, V13.

## File Structure

| File | Responsibility |
| --- | --- |
| `cli/src/objectdata/modfile.ts` | Reader and appending writer for `w3u/w3t/w3h/w3a/w3q` and skin files |
| `tools/metadata/slk.ts`, `tools/metadata/txt.ts` | SYLK subset and INI readers |
| `tools/gen-metadata.ts`, `tools/metadata/overrides.json` | `deno task gen:metadata <folder> <version>` |
| `cli/data/metadata.json`, `cli/src/embedded/metadata.ts` | Committed metadata and its embedded gzip copy |
| `cli/src/objectdata/metadata.ts` | Loading embedded metadata; lookups |
| `tools/gen.ts` | Also renders `schema/generated/*.pkl` |
| `schema/ObjectFile.pkl`, `Objects.pkl`, `objects/*.pkl`, `generated/*.pkl`, `Project.pkl` | Public schema |
| `cli/src/objectdata/manifest.ts` | Evaluated-JSON shape check, `Project.objects` |
| `cli/src/objectdata/resolve.ts` | Resolution, validation, `ObjectDataError` |
| `cli/src/objectdata/plan.ts` | File discovery, skin split, planning and applying |
| `cli/src/objectdata/ids.ts` | Rawcode packing, `objects.yue` rendering, staleness |
| `cli/src/commands/objects-eval.ts`, `objects-check.ts` | New commands |
| `cli/tests/support/objectdata.ts` | Fixture paths, synthetic file builder, miniature metadata |
| `cli/tests/fixtures/objects-v3/` | The full World Editor save (Task 6) |

Task order: 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9. Tasks 1–5 need only the names fixture and the game data. Task 6
starts with the fixture checkpoint and is blocked until the maintainer delivers the §12.2 map.

---

### Task 1: Modification-file reader

**Files:** Create `cli/src/objectdata/modfile.ts`, `cli/tests/support/objectdata.ts`,
`cli/tests/unit/objectdata-modfile.test.ts`. Modify root `deno.json` `fmt.exclude` to add
`cli/tests/fixtures/objects-v3-names` (and later `objects-v3`).

**Interfaces:**

```ts
export type TableKind = "simple" | "leveled";
export type ModValue = { type: "int"; value: number } | { type: "real" | "unreal"; value: number }
  | { type: "string"; value: string };
export interface Modification { field: string; level: number; column: number; value: ModValue; end: string;
  start: number; stop: number }
export interface ObjectEntry { base: string; id: string; sets: { flag: number; mods: Modification[] }[];
  start: number; stop: number }
export interface ModTable { countOffset: number; start: number; stop: number; objects: ObjectEntry[] }
export interface ModFile { version: number; original: ModTable; custom: ModTable }
export function tableKind(fileName: string): TableKind // by extension, case-insensitive: w3a, w3d, w3q leveled
export function readModFile(bytes: Uint8Array, kind: TableKind, file: string): ModFile
```

`level`/`column` are 0 in simple tables. `end` is the end token as four characters, or `"\0\0\0\0"` for 0. Ids are
the four bytes as Latin-1 characters; the original table's custom id is kept as read. v1/v2 objects have one
implicit set with flag 0. `start`/`stop` are byte offsets so the writer can copy spans verbatim.

- [ ] **Step 1: Failing tests.**
  - Every file in `objects-v3-names` parses; assert the README table exactly (base, id, field, level, column,
    `TRIGSTR_*` value, skin versus main side, zero-modification entries), version 3, set count 1, flag 0, end 0.
  - The support module builds synthetic files (`buildModFile({ version, original, custom }, kind)`) with int, real,
    unreal and string values; the reader returns them exactly (floats compared as float32).
  - Synthetic v1 and v2 files (no set fields) parse; labelled synthetic in the test names.
  - Each malformed input throws `MoonwellError` naming the file and hinting to re-save in World Editor 3.00:
    unsupported version (0, 4), truncated header, count past end, unknown var type (4), missing NUL, trailing
    bytes, set count 0 or above a sane limit.
  - A `Uint8Array` view with a nonzero byte offset parses the same as a copy.
- [ ] **Step 2: Implement** with a bounds-checked little-endian cursor (reuse the pattern in `cli/src/w3i/map-info.ts`;
  do not share its private types). Decode strings as UTF-8 with `fatal: true`.
- [ ] **Step 3: Review, full gate, commit** `feat(objects): read object modification files`.

### Task 2: Appending writer

**Files:** Modify `modfile.ts`, `objectdata-modfile.test.ts`.

**Interfaces:**

```ts
export interface NewObject { base: string; id: string; mods: { field: string; level: number; column: number;
  value: ModValue }[] }
export function appendObjects(source: Uint8Array | undefined, kind: TableKind, objects: NewObject[],
  file: string): Uint8Array
```

With `source`, the result is: source bytes up to the custom count, the incremented count, the existing custom objects
verbatim, then `objects` encoded (set count 1, flag 0, end token 0 — constants marked `// to verify (V3, V4)` only for
cases the names fixture does not show). Without `source`, a new version-3 file with an empty original table. Objects
are written in the order given (the planner sorts). A string containing NUL, a non-finite float or a float outside
float32 range, or an int outside int32, is an internal error (the resolver rejects them first).

- [ ] **Step 1: Failing tests.**
  - Known answer from the names fixture: appending each fixture object (same base, id, field, level, column and
    `TRIGSTR_*` value) to `undefined` reproduces each main and skin file byte for byte, including the zero-modification
    entries. This proves encoding and the "both files" rule for names.
  - Appending to every fixture file keeps the prefix and existing objects byte-identical, increments the count, and
    re-reads cleanly with the new objects last.
  - Appending an empty list returns bytes equal to the source.
  - Synthetic v1/v2 sources get v1/v2-shaped new objects (no set fields).
- [ ] **Step 2: Implement.** **Step 3: Review, full gate, commit** `feat(objects): append objects to modification files`.

### Task 3: Game metadata

**Files:** Create `tools/metadata/slk.ts`, `tools/metadata/txt.ts`, `tools/gen-metadata.ts`,
`tools/metadata/overrides.json`, `cli/data/metadata.json`, `cli/src/objectdata/metadata.ts`,
`cli/tests/unit/{metadata-readers,gen-metadata,metadata}.test.ts`. Modify `tools/gen.ts` (embed
`cli/src/embedded/metadata.ts`), root `deno.json` (`gen:metadata` task; add the tools to `check`).

**Interfaces:** `metadata.json` exactly as spec §3.2, with `skin` from `netsafe` (V11 above) and `bases.*.levels`
from `abilitydata.levels` / `upgradedata.maxlevel`. `metadata.ts` exports `loadMetadata(): Metadata` (memoized) and
lookups: `fieldsFor(category, base)`, `fieldByRawcode(category, id)`, `fieldByName(category, base, name)`,
`baseOf(id)` (any category), `nearestBases(category, id, n)`.

- [ ] **Step 1: Failing reader tests** on hand-written excerpts: carried-forward `Y`, quoted strings with `;`, bare
  numbers, `E` records, missing cells, CRLF; INI sections, duplicate keys (last wins), comments `//`, quoted values.
- [ ] **Step 2: Failing generator tests** on a miniature game folder written into a temp dir: field records, friendly
  names (spec §3.3, including category-prefix and rawcode collision renames), overrides, base classification (hero,
  building via `isbldg`, unit), level counts, deterministic output, and failing when a previously released friendly
  name would change or disappear without an override.
- [ ] **Step 3: Implement**, then run `deno task gen:metadata /tmp/gamedata 3.0.0.24268` and `deno task gen`.
  Before committing `metadata.json`, check and record in the commit message: every uppercase-first standard unit id
  is a hero in the game data (V12), the counts per category, and the list of automatic renames.
- [ ] **Step 4: Invariant tests** on the committed `metadata.json`: unique rawcodes per category, unique friendly
  names per class, valid storage types and columns, the fields the names fixture wrote to skin files (`unam`, `anam`,
  `fnam`, `gnam`) are `skin: true`, and `uhpm` is `skin: false`. The freshness test covers the embedded copy.
- [ ] **Step 5: Review, full gate, commit** `feat(objects): generate object metadata from the game data`.

### Task 4: Pkl schema

**Files:** Create `schema/ObjectFile.pkl`, `schema/Objects.pkl`, `schema/objects/{Unit,Hero,Building,Item,Ability,
Buff,Upgrade}.pkl`, `schema/generated/*Props.pkl` (rendered by `tools/gen.ts`), `schema/tests/Objects.pkl`. Modify
`schema/Project.pkl` (`objects: Objects = new {}` or the shape settled in Step 1), `test:pkl` task, embedded freshness
test.

- [ ] **Step 1: Pkl spike, recorded as tests first:** in a scratch `init --link` project (outside `template/`), settle
  the exact `Objects.merge(import*("objects/**.pkl"))` shape and verify V15: nested folders match; a missing or empty
  `objects/` gives an empty result; `moonwell.local.pkl` amending `moonwell.pkl` resolves the glob relative to
  `moonwell.pkl`; the `import*` key is the path relative to the manifest. If a contract in spec §4.1 cannot hold, stop
  and report to the maintainer before continuing.
- [ ] **Step 2: Failing `schema/tests/Objects.pkl`:** id rules per category (hero uppercase-first; unit/building not),
  key pattern and keyword rejection, `base` length, typed property types (scalar, `List` per level, `List<String>`
  for list fields), `properties` values, duplicate key across two files names both files.
- [ ] **Step 3: Implement** the hand-written classes and the generator (doc comments carry label, rawcode, category,
  per-level, skin). Run `deno task gen`.
- [ ] **Step 4: Review, full gate, commit** `feat(objects): Pkl schema for custom objects`.

### Task 5: Manifest shape, resolution and validation

**Files:** Create `cli/src/objectdata/manifest.ts`, `cli/src/objectdata/resolve.ts`,
`cli/tests/unit/objectdata-{manifest,resolve}.test.ts`. Modify `cli/src/project/project.ts` (`Project.objects`),
`cli/src/shared/errors.ts` (`ObjectDataError`, multi-problem `formatError`), `cli/tests/unit/{project,errors}.test.ts`.

**Interfaces:**

```ts
export interface ResolvedField { id: string; name: string; level: number; column: number; skin: boolean;
  value: ModValue }
export interface ResolvedObject { category: Category; key: string; id: string; base: string; source: string;
  fields: ResolvedField[] }
export function resolveObjects(objects: ProjectObjects, existingIds: Set<string>, manifest: string):
  ResolvedObject[] // throws ObjectDataError with every problem
```

- [ ] **Step 1: Failing tests**, one per rule in spec §5.1 with the exact message and hint, using miniature metadata
  from the support module (not the real file, so tests stay stable across patches), plus: multi-problem rendering
  capped at 20 with `and N more`; `file` is the first problem's source; an absent `objects` key is an empty manifest;
  the shape check's version hint; per-level `List` beyond the object's own `levels` or the base's; `properties` by
  rawcode and by friendly name, and setting a field twice through two routes; booleans as 1/0; lists joined with
  commas; explicit `false`, `0`, `""` and `[]` kept as overrides.
- [ ] **Step 2: Implement.** Level and column numbering come from constants marked `// to verify (V6)`, taken from
  the names fixture where it shows them.
- [ ] **Step 3: Review, full gate, commit** `feat(objects): resolve and validate custom objects`.

### Task 6: Fixture checkpoint, planner and generated ids (blocked on the §12.2 map)

**Files:** Create `cli/tests/fixtures/objects-v3/` (files from `objects-fixture.w3x` unchanged, plus README with
provenance, SHA-256, the maintainer's value record and V1–V13 answers with offsets), `cli/src/objectdata/plan.ts`,
`cli/src/objectdata/ids.ts`, `cli/tests/unit/objectdata-{fixture,plan,ids}.test.ts`.

- [ ] **Step 1: Fixture checkpoint.** Commit the save and README. Write `objectdata-fixture.test.ts` asserting the
  maintainer's recorded values through the reader, and pinning each open V answer. Replace every `// to verify`
  constant with the observed value. If an answer contradicts the spec, stop, update the spec, and ask the maintainer
  to approve the revision before Step 2.
- [ ] **Step 2: Failing planner tests:** read-only on success and failure (snapshot the source folder); no objects
  needs no map; case-insensitive file names and two-spellings failure; changed files in the spec's stable order; skin
  split per the fixture; objects sorted by id; collision with existing custom ids in any table; a source file with
  a duplicate custom id inside it fails naming the file (spec §5.2).
- [ ] **Step 3: Known answer:** resolved objects equivalent to the fixture's custom objects, planned against the
  `objects-empty` save, reproduce World Editor's files byte for byte, with `TRIGSTR_*` values substituted for literal
  strings. Any remaining difference is documented in the README and justified, or fixed.
- [ ] **Step 4: `ids.ts` tests:** big-endian packing (`h000` = 1747988528), key sort, category order, the exact
  header, and staleness (missing file with objects fails; no objects and no file passes).
- [ ] **Step 5: Implement, review, full gate, commit** `feat(objects): plan object data against the source map`.

### Task 7: Pipeline and commands

**Files:** Create `cli/src/commands/objects-eval.ts`, `objects-check.ts`, `cli/tests/pkl/objects.test.ts`,
`cli/tests/yue/objects.test.ts`. Modify `cli/src/pipeline.ts`, `commands/check.ts`, `commands/dev.ts`, `main.ts`,
`project-files.ts` (`PROJECT_TASKS`), `cli/tests/unit/{pipeline,dev,main}.test.ts`.

- [ ] **Step 1: Failing tests** for spec §9: pipeline order (plan before compile, apply after staging, before
  settings); `Added N custom object(s) to M file(s).`; `check` fails on stale `objects.yue` with the hint; `dev`
  watches `objects/`; `objects:eval` prints JSON to stdout only; `objects:check` output and exit code; Pkl
  integration in an `init --link` project with nested `objects/` and without the folder; the generated module under
  `yue -e` matches a `FourCC` stub.
- [ ] **Step 2: Implement, review, full gate, commit** `feat(objects): build, check and dev support for objects`.

### Task 8: Template, docs and e2e

**Files:** Create `template/objects/units.pkl`, `template/src/generated/objects.yue`, `cli/tests/e2e/objects.test.ts`.
Modify `template/moonwell.pkl`, `template/src/main.yue`, `template/deno.json`, embedded output, README, CONTRIBUTING
(new gate step 9), CHANGELOG (Unreleased), AGENTS.md.

- [ ] **Step 1:** The Captain's model path comes from the fixture (the maintainer set the Captain's model on the
  custom Footman). Write the template files and regenerate.
- [ ] **Step 2: Failing e2e** per spec §10.3, then pass.
- [ ] **Step 3:** Docs per spec §11. **Step 4: Review, full gate, commit** `feat(objects): template Captain and docs`.

### Task 9: Maintainer release gate

- [ ] Walk the maintainer through CONTRIBUTING step 9 (spec §11) in a throwaway project; fix what it finds with a
  regression test each. Record the result in CHANGELOG. Version bump to 0.3.0 happens separately.

## Completion

The full gate, run before every commit and green on CI after every push:

```
deno task check && deno task lint && deno fmt --check && deno task test && deno task test:runtime \
  && deno task test:pkl && deno task test:e2e && deno task gen && git diff --exit-code
```
