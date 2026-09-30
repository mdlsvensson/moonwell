# moonwell-wrappers v0.5.1 review (2026-09-30)

Roadmap item 2.1 (`docs/superpowers/plans/2026-09-30-moonwell-roadmap-to-wc3-lib.md`), reviewed by Claude on Opus 5.5.
The maintainer chooses the findings for the refactor release (2.2).

## Scope and method

- **Code:** all 24 modules in `src/wrappers/` and the four internal ones.
  - Read line by line: `internal/*`, `unit`, `player`, `timer`, `trigger`, `group`, `effect`, `texttag`, `lightning`,
    `sound`, `item`, `destructable`, `rect`, `region`, `force`, `image`, `ubersplat` and `fogmodifier`.
  - Checked against today's findings: the classic UI and frame modules. They were reviewed line by line at their
    release two days earlier.
- **Tests:** 29 suites, 167 tests, `tests/support.lua`.
- **Docs:** README (389 lines), CONTRIBUTING and AGENTS.md.
- **Inputs:** the performance note (`2026-09-30-wrappers-performance.md`) and the port-needs note
  (`2026-09-30-wc3-lib-port-needs.md`, with D1 decided).
- **One experiment:** `.test-work/levels.lua` in the wrappers repository, run with the tests' native stand-ins. It shows
  which line each kind of error blames (R1).

**Summary.** The review found **no correctness bugs**. The same rules hold in every module:

| Rule | Holds |
| --- | --- |
| Ownership | Presentation objects are owned; game-ended ones return nothing |
| Cleanup | Disposal comes before native destruction, and repeating it is harmless |
| Determinism | Ordered arrays where natives are called; the only `pairs` loops are in `Options.read` and call none |
| Local visibility | The same native on every machine; `Sound:playFor` is the one documented exception |
| Callbacks | Isolated for events; errors propagate for callbacks that run immediately |
| Nothing at import | No game objects are created when a module loads |

The findings are about developer experience, cost on hot paths, documentation, and a few additions.

## Findings

Severity is about impact on map authors, not effort.

### R1. Which line an error blames depends on the call path (medium)

Moonwell's error report shows `file:line`, so the line an error blames is what the map author sees. In the experiment:

| Error | Blamed line |
| --- | --- |
| A disposed wrapper as receiver (`u:getX()` after `remove()`) | the caller's line |
| A non-wrapper as receiver (`Unit.getX({})`) | `unit.lua:44`, inside the library |
| A wrong argument type (`Unit.create({}, …)`) | `unit.lua:24` |
| A wrong widget (`u:issueTargetOrder('x', {})`) | `unit.lua:348` |
| A disposed widget argument | `unit.lua:348` |
| A wrong receiver to `remove()` | `unit.lua:134` |
| A wrong receiver to `isDisposed()` | the caller's line, but only because a tail call drops a frame |

**Why it happens:** `registry.require` raises at level 3 for a disposed wrapper, which is right. But it reaches the
wrong-type error through a second helper, `member`, which adds a frame, and `Handle.unwrap` and `unwrapWidget` add
another. The messages are always right (they name the operation); only the location varies. The tests check message
fragments only (`fails(fn, fragment)`), so this never showed.

**Recommendation.** Give every error path an explicit level, so that each blames the map author's line. Add a test
helper that asserts the blamed file for each kind of error. The game's Lua has no `debug` library, so the levels must be
counted, not discovered.

### R2. The fixed per-method cost (medium)

Every method starts with `registry.require(self, operation)`. On the happy path that makes two function calls
(`require`, then `member`) and one weak-table lookup. The performance note measured about 120 ns over the raw native
(1.26–1.38 times).

**Recommendation.** Make the happy path one call: `require` looks up the private membership table directly and returns
the handle. Only the failure path goes on to the error helpers, which also fixes R1 there. Authentication stays
membership-based, not the public `handle` field, so a table with a `handle` field is still rejected. Re-run the
performance probe (`deno task gate perf`) before and after, to report the saving honestly.

### R3. Enumeration allocates two tables per call (medium)

`Group:forEach`, `getUnits` and filtered enumerations first build a table of raw handles (`members`), then a table of
wrappers (`wrapAll`). A filter adds a `rejected` table and a `pcall` closure. This is the largest part of the missile
tick's overhead: 20 units cost 29.2 µs wrapped against 19.4 µs raw.

**Recommendation.** Wrap straight from `BlzGroupUnitAt` into one table, keeping the snapshot semantics: every unit is
wrapped before any callback runs, so a unit removed mid-iteration keeps its disposed wrapper. The same applies to
`Item.enumInRect` and `Destructable.enumInRect`, which already use one table plus one for the filter.

### R4. Liveness uses a workaround (planned; medium for the port)

`unit:isAlive()` avoids `UnitAlive`, so the library still works with Moonwell 0.5.0. `wc3-lib` defines liveness a third
way (type id, life above 0.405, not dead). Moonwell 0.5.1 knows `UnitAlive`, and it is confirmed in game.

**Recommendation.** Use `UnitAlive` and require Moonwell 0.5.1 or later (README, CONTRIBUTING). The port then uses
`isAlive()` as its only definition of liveness.

### R5. No way to ask whether a wrapped widget still exists (low to medium)

A unit the game removes by itself (decay, removal by other code) leaves a wrapper that is not disposed, and whose methods
call natives on a dead handle. `wc3-lib` prunes such units by polling `GetUnitTypeId(unit) ~= 0`. The 1.4 probe found
that the undefend order detects removal exactly, but only with a Defend ability in every map's object data.

**Recommendation.** Add `unit:exists()` (type id ≠ 0), and the same for Item and Destructable if the probe confirms
their type ids read 0 after removal. It gives maps and the port a cheap test, and polling can be built on it. Keep
automatic disposal in the backlog until libraries can ship object data (roadmap 4.2); an opt-in undefend-based helper
could follow then.

### R6. The API reference is hard to read (medium, docs)

The README's API reference is one Markdown table. Its rows are up to 1,874 characters long, and 26 lines exceed 400
characters. It renders as a very wide table, and a one-method change is a diff of a huge line.

**Recommendation.** Give each module its own subsection with one line per method group (factories, getters, setters,
cleanup), keeping the same content. Generating it from the LuaLS annotations would stop drift, but it is a bigger job
and is not proposed now.

### R7. The callback rule is not stated in one place (low, docs)

The code follows one rule everywhere:
- **Callbacks that run immediately let errors propagate:** filters, `forEach`, `enumInRect`.
- **Event callbacks are isolated and printed:** timers, trigger actions, conditions (which count as false), dialog
  buttons and frame events.

The README's "Callbacks" section covers timers and triggers only. The others are documented in their own sections.

**Recommendation.** State the rule once in "Callbacks", listing all five boundaries by their printed labels
(`Timer`, `Trigger`, `Trigger condition`, `Dialog button`, `Frame event`).

### R8. The same boilerplate in every module (information; keep)

Every class repeats `fromHandle`, `getHandle`, `isDisposed` and `destroy` (or `remove`): about 12 lines each. A shared
generator would save about 250 lines. But it would lose the per-class LuaLS types (`---@return effect`, `unit`, …) that
the editor relies on, and hide which native each class destroys.

**Recommendation.** Keep the repetition. The native-call check (roadmap 1.1) now catches mistakes in it.

### R9. Native-faithful conventions differ between classes (low; keep and document)

The wrappers keep the natives' conventions, so they differ between classes:

| What | Varies |
| --- | --- |
| Colour ranges | 0–255 for units, text tags, images and effects; 0–1 for `Lightning:setColor` |
| Angles | Degrees for unit facing and `TextTag.float`; radians for `Effect:setOrientation` |
| Visibility names | `show`/`isHidden` for units and destructables; `setVisible`/`isVisible` for items |

The annotations say which each method uses.

**Recommendation.** Keep them native-faithful, as renaming breaks maps and hides the native. Add a short "Units and
conventions" table to the README.

### R10. Module size (information; keep)

`frame.lua` has 496 lines, `unit.lua` 372 and `trigger.lua` 273. Each is one class, with sections in the order of the
README. Splitting a class across files makes LuaLS and the reader jump between files for no gain yet.

**Recommendation.** Keep them. Revisit `frame.lua` if it grows past about 700 lines.

### R11. Tests check messages, not locations (low; goes with R1)

**Recommendation.** With R1, add a `failsAt(fn, fragment, file)` helper and assert the blamed file for the error kinds in
R1's table.

### R12. `Options.read` reports its first error in hash order (information)

When several options are wrong, which one is reported first follows `pairs` order, which can differ between machines. It
changes only the text of an error, never game state.

**Recommendation.** No change; or sort the field names once per field table if identical messages everywhere matter.

## Port prerequisites (for 2.3, not review findings)

D1 decided these additions (port-needs note §4):
- `unit:getCollisionSize()` and `unit:setPathing(flag)`;
- sync: a trigger registration, sending with the 255-character limit enforced (the probe showed a silent cut), and the
  event's data;
- damage event data: reading and changing a hit.

Two open questions from the port-needs note concern the wrappers:
- **The callback boundary:** a public `wrappers.callback`, or a copy in the port. Recommendation: a copy in the port. It
  is 20 lines, and a public module would be more API to keep stable.
- **Ordered collections:** where they live. Recommendation: in the port. The wrappers keep ordered arrays inline.

## Proposed v0.6.0 (if every recommendation is taken)

**Code:**
- R1 and R11: explicit error levels, with tests for where each error points;
- R2: a one-call method prologue;
- R3: one-table enumeration;
- R4: `UnitAlive`, requiring Moonwell 0.5.1;
- R5: `exists()`.

**Docs:** R6 (per-module API reference), R7 (the callback rule) and R9 (the conventions table).

**Gate:** the full in-game gate, normal and minified, and a before-and-after performance run.

2.3's additions can join the same release or follow as v0.7.0.
