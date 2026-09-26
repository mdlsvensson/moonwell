# Moonwell Map Settings (Plan 2b) — Design

- **Date:** 2026-09-25
- **Status:** Approved by the maintainer on 2026-09-25
- **Builds on:** `2026-09-24-moonwell-core-design.md`, §§3.3, 5 and 6.3
- **Scope:** Map settings only; object data and publishing remain separate work.

## 1. Summary

Authors configure map settings in the project's existing `moonwell.pkl`. Moonwell validates them, patches a staged
copy of the World Editor map, and keeps its metadata and Lua initialization consistent. The source map remains
unchanged. `settings:check` reports which internal files would change without staging or compiling gameplay.

Success means every setting described in core §6.3 works through the normal build/test pipeline, defaults preserve
the source map, invalid or unsafe edits produce actionable errors, and unrelated map bytes and Lua code survive.

The reference is `C:/Users/mdlsvensson/Repo/wc3-dev-framework/scripts/map-settings/`, its `map-settings-schema.pkl`,
and `scripts/tests/map-settings*.ts`. It is read-only input to this port. No runtime dependency on that repository,
Node.js, or npm is introduced. File I/O is async; expected failures use `MoonwellError` with `file` and `hint`.

## 2. Approach

Use the reference's offset-based binary edits and section/key text merges, with an independent Deno implementation
of the structural Lua reading needed for safe edits. This keeps the behavior established in the core design and
fits Moonwell's existing plan-then-apply pattern.

Alternatives considered:

- Reconstruct complete map-info files and Lua functions. This needs substantially more format knowledge and risks
  losing unknown Reforged fields, comments, or unrelated initialization.
- Inject runtime overrides only. This leaves lobby/map metadata inconsistent and cannot replace the required
  internal text-file changes.

The focused port is recommended. Its main new component is the Lua structure reader; its boundaries and refusal
conditions are part of this design, not an assumption that regular expressions can safely edit Lua.

## 3. Manifest contract

`schema/MapSettings.pkl` defines the settings module and its nested classes. `Project.pkl` imports it and exposes
`settings: MapSettings = new {}`. There is no separate project-level `map-settings.pkl` or extra Pkl evaluation.

All scalar defaults are `null` (inherit); group and index mappings default to empty. Missing `settings` in evaluated
JSON means empty settings, preserving compatibility with the published 0.1.x schema. Unknown settings or nested
fields fail, even when their supplied value is null. Omitted/null scalar fields normalize to no override. Explicit
`false`, `0`, and `""` remain overrides. Empty groups have no effect and do not require reading map files.

| Group | Fields | Values |
| --- | --- | --- |
| `info` | `name`, `author`, `description`, `recommendedPlayers` | Text without NUL; empty clears |
| `loadingScreen` | `background` | Integer -1..2147483647; -1 selects a custom model |
| | `model`, `text`, `title`, `subtitle` | Text without NUL; empty clears |
| `gameplay` | `heroMaxLevel`, `foodLimit` | Integers 1..10000 and 0..300 respectively |
| `players["id"]` | `name` | Text without NUL |
| | `controller` | `user`, `computer`, `neutral`, `rescuable` |
| | `race` | `selectable`, `human`, `orc`, `undead`, `nightelf` |
| | `fixedStart` | Boolean |
| | `x`, `y` | Finite numbers -10000000..10000000 |
| `forces["index"]` | `name` | Text without NUL |
| | `allied`, `alliedVictory`, `sharedVision`, `sharedControl`, `sharedAdvancedControl` | Boolean |
| `environment` | `soundEnvironment` | Text without NUL |
| | `waterColor` | Four integer channels, RGBA, 0..255 |
| `environment.fog` | `enabled` | Boolean |
| | `style` | Integer 0..2, preserving the reference's numeric interface |
| | `start`, `end` | Finite numbers -10000000..10000000 |
| | `density` | Finite number 0..1 |
| | `color` | Four integer channels, RGBA, 0..255 |
| `gameplayConstants`, `gameInterface` | Section → key → value mappings | Single-line raw strings |

Player IDs and force indices are canonical decimal strings `"0"`..`"23"` (no leading zeroes). Player IDs refer to
existing records by ID; forces refer to existing records by index. Neither changes record count or team membership.
Null-only entries normalize away; they do not require the referenced slot to exist.

Colors use constrained Pkl `List<Int>`, never `Listing`. Pkl validates types/ranges; TypeScript validates evaluated
JSON defensively and checks map-dependent conditions. Fog start must not exceed fog end after inheriting any
unspecified endpoint from the map.

Raw section and key names match `[A-Za-z_][A-Za-z0-9_]*`; duplicate names differing only in case are rejected within
each override mapping. Values may be empty but cannot contain CR, LF, or NUL. Dictionary handling must treat names
such as `constructor` as data, not inherited object properties.

`gameplay.heroMaxLevel` maps to `[Misc] MaxHeroLevel`; `gameplay.foodLimit` maps to `[Misc] FoodCeiling`. A raw override
of the same key is accepted only if its string exactly equals the typed value's decimal representation. Matching
section/key names ignores case. This conflict check concerns configured overrides, not the source file's old value.

## 4. Map-info patching

New modules under `cli/src/w3i/` read field spans and replace only selected spans. They support versions 18, 25, 28,
31, 32, 33, and 39. The existing small `cli/src/map/w3i.ts` packaging-header reader retains its public API; this work
does not broaden packaging behavior or introduce an unrelated refactor.

- Info/loading fields work on all supported versions, except a loading-screen model override requires version 25+.
  Even an explicit empty model is an override and is rejected on version 18. `background` and `model` remain separate
  settings; setting a model does not silently choose a background.
- Player, force, and environment overrides require version 28+ with the Lua script-language field set to 1.
- Force overrides require custom forces already enabled in World Editor (map flag `0x40`). Missing slots/forces fail
  with instructions to create/configure them in World Editor first.
- Player controllers and races use the reference's numeric mappings. Force flags are allied=1, alliedVictory=2,
  sharedVision=8, sharedControl=16, sharedAdvancedControl=32; unspecified and unknown bits are preserved.
- Water color sets the custom-water flag `0x10000`. Fog `enabled` controls flag `0x2000`; other fog fields do not
  implicitly enable fog. Sound environment and colors otherwise follow the reference.
- Preserve every byte outside changed spans, including trailing tables, unknown flags, version-39 extensions, race
  skin preferences, and start-location priorities. Never deserialize and regenerate the entire file.
- Unchanged `TRIGSTR_*` references stay intact. Explicit text overrides replace only that field with literal text;
  `war3map.wts` is never rewritten or dereferenced.
- Bounds, counts, string termination, UTF-8 decoding, and extended player records are checked before edits are
  returned. Reject unsupported versions or malformed required structure with a map-file error and a re-save hint.
  Empty settings do not attempt to validate otherwise unused binary sections.

The reference's real version-39 fixture records Warcraft III 3.0.0.24268. Copy its map-info/Lua pair and provenance
README into test fixtures, preserving its hash and independently recorded offsets. Older-version fixtures are
synthetic and must be labeled as such; passing them is not a claim of in-game validation on old clients.

## 5. Coordinated Lua edits

Metadata alone is insufficient: World Editor emits some settings again as native calls. Patch the original staged
Lua before bundle injection, so the final bundle line table includes all settings edits.

### 5.1 Structural reader

`cli/src/settings/lua-structure.ts` supplies source ranges for top-level global function declarations and direct
call statements within their bodies. A focused lexer retains numbers and character offsets and handles quoted
strings, escapes, long-bracket strings/comments, and line comments. The existing bundler lexer drops numbers and
offsets, so it cannot directly serve this contract; leave its existing API and behavior intact.

Track parentheses, brackets, table constructors, and Lua block boundaries, including nested functions, `if` branches,
`for`/`while` blocks, standalone `do`, and `repeat`/`until`. Only standalone direct calls count as editable editor
calls. Calls inside assignments, expressions, nested blocks/functions, or member/method calls do not count.
Literal numeric arguments identify player IDs and start-location indices; expression-valued identifiers are not
guessed. Whitespace, multiline calls, comments, and optional semicolons must not change recognition.

Reject unterminated strings/comments, mismatched delimiters/blocks, duplicate required functions/calls, or any
ambiguous target shape. Validate required call arity and literal identification before replacing it. Unrelated
source is preserved verbatim. This is a structural editor, not a complete Lua compiler; runtime tests separately
compile patched scripts. It must fail safely where it cannot establish an edit's boundaries.

### 5.2 Edit rules

Only require the functions/calls needed by active settings:

- `info.name` and `info.description`: replace the unique direct `SetMapName` / `SetMapDescription` in `config()`.
- Players: require a unique `InitCustomPlayerSlots()` call in `config()` and a corresponding global function.
  Verify each edited player's `SetPlayerStartLocation(Player(id), index)` agrees with its record order in map info.
  Replace controller, race preference, and race-selectable calls when requested. Set/insert player name as needed.
  Insert/replace/remove `ForcePlayerStartLocation` for an explicit fixed-start value. Coordinates replace the
  corresponding `DefineStartLocation` in `config()` using both effective coordinates from patched map info.
- Forces: names affect map info only. Flag edits require `InitCustomTeams()` in `config()` and a matching function.
  Verify existing `SetPlayerTeam` calls agree with inherited membership. Append effective allied-victory state and
  pairwise alliance calls at the function's end, after editor calls, as the reference does. Preserve nonmember teams.
- Environment: require global `main()` and a direct initialization anchor (the first `CreateAllUnits()` or
  `InitBlizzard()` in source order). Replace relevant direct sound/water/fog initialization calls with effective
  values immediately before that anchor. Empty sound environment emits `NewSoundEnvironment("Default")`, while
  map info stores the explicitly empty value. Fog RGB is normalized to 0..1; alpha is retained in map info.
  Disabled fog emits `ResetTerrainFog()`. Treat both existing `SetTerrainFogEx` and `ResetTerrainFog` as fog
  initialization when replacing it, so an old reset cannot undo the override.

Lua strings use Lua-compatible escaping (three-digit decimal escapes for control characters), not JSON escapes.
Apply validated nonoverlapping edits from the end of the source and re-read the result for structural integrity.
All values coordinated with binary fields use the patched binary values, including float32 rounding.

Every build starts with a fresh source-map copy. Force-call additions need not be idempotent when applied repeatedly
to already-patched Lua; normal builds must be deterministic and never accumulate additions between runs.

## 6. Internal text files

`gameplayConstants` merges into `war3mapMisc.txt`; `gameInterface` merges into `war3mapSkin.txt`.

Match section/key names case-insensitively, preserving existing spelling and key indentation. Replace every matching
occurrence when the source repeats a section or key. Add missing keys to the existing section, or append a missing
section. Preserve unrelated lines, standalone comments, and existing LF/CRLF convention. A replaced value replaces
the rest of that key's line; inline text is not parsed as a separate comment. Missing files are created only when
they have nonempty overrides, using LF. Empty strings explicitly write `Key=`. Reapplying a text merge is idempotent.

## 7. Planning, commands, and pipeline

`planMapSettings(mapDir, settings)` is async and read-only. It validates and computes all binary, Lua, and text edits
before returning any writes. Its result lists changed files in stable order: `war3map.w3i`, `war3map.lua`,
`war3mapMisc.txt`, `war3mapSkin.txt`. Omit files whose resulting bytes equal their source bytes. An empty configuration
returns an empty plan without reading map content. Neither planning nor applying mutates the input settings object.

`applySettingsPlan(plan)` writes the planned bytes only to the staged map. Build/test already hold the build lock.
A failure aborts the build through its existing stale-output cleanup; staged files may be partial and will be replaced
on the next build. There is no source-map settings sync or rollback system in this scope.

Pipeline order follows core §5: compile → stage → settings → assets → bundle injection → pack/launch. Object data
will precede settings when Plan 2c arrives. Settings never appear in `war3map.imp`.

`deno task settings:check` loads the manifest through `loadProject`, validates the source-map folder, runs the same
planner, and prints relative file names followed by a summary, for example:

```text
  war3map.w3i
  war3map.lua
  war3mapMisc.txt
Map settings valid: 3 internal file(s) would change during build.
```

With no changes the summary says zero. Failure exits 1; success exits 0. The command neither installs Yue nor
compiles, stages, writes map files, or takes a build lock. The existing CLI log in `dist/moonwell.log` is the only
normal logging write; "without writing" means no settings/map changes, consistent with the other check commands.

General `check` also runs the planner. When no effective settings are configured, it preserves the existing ability
to check gameplay without a source map. With effective settings, the source map must exist. `dev` inherits this
validation through `check` on manifest changes; this plan does not add map-file watching.

Add dispatch/help, `PROJECT_TASKS`, the template task, and documentation. Expected config errors name the selected
manifest and `settings.<path>`; malformed/missing map data and unsafe Lua edits name the relevant map file. I/O
failures give file context and an actionable hint rather than appearing as internal errors.

## 8. Template and documentation

The template's enabled `settings` block spells out everyday scalar fields at `null`, nested `environment.fog`, and
empty `players`/`forces` mappings. Defaults therefore inherit the supplied map; they do not replace its name, heroes,
colors, or forces. Include an active player `"0"` example with every player field at `null` (normalized to no effect),
so users can edit the fields of an existing template slot directly.

Inside `forces`, show a commented `"0"` entry with all force fields at `null` and a comment: first enable custom forces
in World Editor's Scenario → Force Properties, configure the team, and save the map. This is the only commented
settings example needed for the default template. Explain that player entries must refer to slots already saved in
World Editor. Color comments show `List(r, g, b, a)`; no color uses `Listing`.

The two raw section/key mappings are advanced escape hatches: omit them from the template, document them in the schema
and README with examples and the typed/raw conflict rule. All other fields in §3 are everyday settings and are
written out, including loading model/background and all environment fields. Nullable defaults do not require a
World Editor prerequisite merely to appear in the file.

README covers inheritance/clearing, supported versions, existing-slot/team limits, `settings:check`, and Lua/editor
prerequisites. CHANGELOG gains an Unreleased entry. CONTRIBUTING's manual release gate gains map-name, player/team,
environment, and gameplay-constant checks in a throwaway project, including opening the built map in World Editor.
Regenerate embedded template files with `deno task gen` after template edits. No version bump or release is included.

## 9. Components

| File or area | Responsibility |
| --- | --- |
| `schema/MapSettings.pkl`, `schema/Project.pkl` | Public schema and manifest wiring |
| `cli/src/settings/options.ts` | Typed normalized settings and JSON validation |
| `cli/src/w3i/map-info.ts`, `patch.ts` | Checked span reader and byte-preserving binary patches |
| `cli/src/settings/lua-structure.ts`, `lua.ts` | Structural ranges and coordinated Lua edits |
| `cli/src/settings/text.ts` | Pure section/key text merging |
| `cli/src/settings/plan.ts` | Async read-only planning and staged application |
| Project parser, pipeline, `commands/check.ts` | Load once and integrate shared planning |
| `commands/settings-check.ts`, main, project tasks | Dedicated report command |
| Template, embedded output, README, CONTRIBUTING, CHANGELOG | User-facing workflow |

## 10. Verification and execution

The implementation plan will follow the existing plans' small tasks, exact file/interface descriptions, failing
tests, implementation steps, checks, and review after each task. Tests must establish behavior independently of the
implementation, especially binary offsets and Lua edit boundaries.

- Schema/JSON: all fields and limits, null/omitted values, explicit false/zero/empty, unknown fields, color replacement
  through Pkl `List`, case collisions, and typed/raw conflicts. Force lazy Pkl mapping constraints with `.toMap()`.
- Binary: all seven versions; fixed golden offsets in the real v39 fixture; Unicode; clearing; inherited WTS
  references; version-18 model rejection; unsupported versions; truncation/invalid UTF-8/counts/player records;
  missing slots/forces; custom-force and Lua flags; effective fog bounds; preservation of unknown bytes and tails.
- Text: LF/CRLF, comments, duplicate source keys/sections, new sections/files, empty overrides, and idempotence.
- Lua: real editor script plus adversarial strings/comments, multiline calls, nested blocks/functions, expressions
  containing calls, duplicate/missing functions/calls, wrong player/start/team linkage, optional-call insertion,
  control-character escaping, inherited values, and unchanged unrelated initialization.
- Runtime suite: compile and execute patched fixture scripts under Yue's Lua VM with stubbed natives, invoking
  `config()` and `main()` to observe effective player/team/environment calls and initialization order. This supports
  the structural tests without adding an external-tool dependency to unit tests.
- Planner/commands: read-only source snapshots on success and failure; unchanged-file filtering; no compiler for
  `settings:check`; check/dev integration; actionable errors; no-op settings compatibility with older manifests.
- End-to-end: configure representative overrides in an `init --link` temporary project, build twice, inspect archive
  entries with the existing test MPQ reader, verify coordinated Lua and bundle placement, source preservation, and
  deterministic output without accumulated force calls. Exercise the staged `test` path without launching a real
  game using the established test harness.

Before commits, run all required repository checks: `deno task check`, `deno task lint`, `deno fmt --check`,
`deno task test`, `deno task test:runtime`, `deno task test:pkl`, and `deno task test:e2e`. Commit on `main`; the
maintainer pushes. After the maintainer reports a push, inspect the matching commit's Ubuntu and Windows CI runs and
investigate failures. Automated checks do not substitute for the manual game release gate.

## 11. Out of scope

- Creating/removing players or forces, changing team membership, or enabling custom forces automatically.
- Rewriting WTS, terrain, weather, light environment, HD-only environment extensions, or arbitrary Lua code.
- Full Lua compiler/parser tooling, new dependencies, source-map settings sync, or separate settings files.
- Object data, metadata extraction, version bumps, publishing, and unrelated asset improvements.
