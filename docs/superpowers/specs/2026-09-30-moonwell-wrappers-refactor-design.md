# Moonwell Wrappers Refactor (wrappers v0.6.0) — Design

- **Date:** 2026-09-30
- **Status:** The findings were chosen in chat on 2026-09-30; this written spec awaits review.
- **Builds on:** the wrapper specs for v0.1.0 to v0.5.0 (`2026-09-28-moonwell-wrappers-design.md`,
  `2026-09-28-moonwell-wrappers-broad-design.md`, `2026-09-29-moonwell-wrappers-presentation-design.md`,
  `2026-09-29-moonwell-wrappers-classic-ui-design.md` and `2026-09-29-moonwell-wrappers-frames-design.md`). Everything in
  those specs still applies unless this one changes it explicitly.
- **Inputs:**
  - the review, `docs/superpowers/research/2026-09-30-wrappers-review.md` (findings R1–R12);
  - the performance note, `2026-09-30-wrappers-performance.md`;
  - the port-needs note, `2026-09-30-wc3-lib-port-needs.md`.
- **Target:** wrappers `v0.6.0` in `mdlsvensson/moonwell-wrappers`. It requires Moonwell 0.5.1 or later. The Moonwell
  CLI does not change.

## 1. Intent and scope

Roadmap item 2.2: the refactor release that follows the review. The maintainer chose every finding with a change (R1–R7,
R9, R11, R12). R8 and R10 were "keep" and change nothing. The port prerequisites (roadmap 2.3: collision size,
pathing, sync, damage event data) are **not** in this release; they get their own spec for v0.7.0.

The release changes no public signature and no error message. What changes:
- **Error locations:** errors point at the map author's line (R1).
- **Speed:** cheaper methods and enumeration (R2, R3).
- **Liveness:** `isAlive()` uses `UnitAlive` (R4).
- **A new method:** `exists()` (R5).
- **Options:** the first reported options error is the same on every machine (R12).
- **Docs:** R6, R7 and R9.

The one compatibility change is the minimum Moonwell version: 0.5.1, which knows `UnitAlive`.

## 2. Error locations (R1, R11)

### 2.1 The rule

Every error the wrappers raise points at the line that called the public function or method. Its message stays the same.

The game's Lua has no `debug` library, so the level of each `error` call is counted by hand. Two measured facts shape
the design (experiments in `.test-work/`, 2026-09-30):

1. **Today the levels vary with the call path.** A disposed receiver points at the caller; a wrong receiver or a wrong
   argument points inside the library (for example `unit.lua:348`).
2. **A tail call in the path drops the position entirely.** `return raise()` produced the bare message `boom`, while
   `return (raise())` pointed at the right line. Lua 5.3 and 5.4 treat tail calls the same way.

### 2.2 Levels

Levels count from the function that calls `error`:

| Helper | Called from | Level |
| --- | --- | --- |
| `registry.require`, `registry.dispose`, `registry.isDisposed` (failure path) | a public method | 3 |
| `Handle.unwrap`, `Handle.unwrapWidget` | a public method or factory | 3 |
| `Handle.created` | a public factory | 3 |
| `Callback.check`, `Callback.optional`, `Callback.nonnegative` | a public method | 3 |
| `Options.read`'s `fail` | `Options.read`, from a public method | 4 |
| Module-local checkers (`checkSlot`, multiboard and leaderboard helpers, trigger and frame token checks) | a public method | 3 |
| A public function itself | — | 2 |

Today `registry.require` reaches its wrong-receiver error through a second helper (`member`), which adds a frame. The
new `require` raises from its own body (§3). A helper that must raise from a shared function takes the level as a
parameter and adds one for itself.

### 2.3 No tail calls into a raising helper

A public function must not end in a tail call to anything that can raise. That covers `registry.require`,
`registry.isDisposed`, `Handle.created`, `Handle.unwrap`, `unwrapWidget` and the checkers. The fix is to wrap the
returned call in parentheses, which Lua does not treat as a tail call:

```lua
function Unit:getHandle() return (registry.require(self, 'Unit.getHandle')) end
function Unit.create(owner, typeId, x, y, facing)
    local rawOwner = Handle.unwrap(owner, 'Player', 'Unit.create')
    return (Handle.created(Unit.fromHandle(CreateUnit(rawOwner, typeId, x, y, facing)), 'Unit.create'))
end
```

A call that is only an argument (`GetUnitX(registry.require(self, …))`) is not a tail call and stays as it is. A tail
call to a native (`return GetUnitX(…)`) is harmless, because natives raise no wrapper errors.

### 2.4 Tests

`tests/support.lua` gains `failsAt(fn, fragment)`. It passes only when the error message contains the fragment **and**
the position names the calling test file, so a message with no position or with a library position fails. Two kinds
of test use it:

1. **A sweep** (`tests/blame.lua`) over every loaded class.
   - It calls every function of the class table with an empty table as receiver, under `pcall`.
   - Every call that fails with `expected <Class> wrapper` must point at the sweep's own line.
   - Every function that takes a receiver is covered without a list, so a new method with a tail call fails
     automatically.
   - Static functions that accept `{}`, or fail with another message, are skipped by the message check.
   - The sweep also asserts that it checked at least one method per class, so a rename cannot silently empty it.
2. **Targeted tests,** one per other error kind:
   - a disposed receiver;
   - a wrong and a disposed wrapper argument (`unwrap`);
   - a wrong and a disposed widget argument (`unwrapWidget`);
   - a native returning nil in a factory (`created`);
   - a callback check;
   - an options error (a wrong type and an unknown key);
   - an inventory slot;
   - a trigger token, a frame token, a multiboard cell and a leaderboard item.

Existing `fails` assertions keep working. The new tests are added, not rewritten.

## 3. A cheaper method prologue (R2)

`registry.require` gets a direct happy path:

```lua
function registry.require(value, operation)
    local raw = members[value]
    if raw then return raw end
    if raw == false then error('[wrappers] ' .. operation .. ': ' .. name .. ' is disposed', 3) end
    error('[wrappers] ' .. operation .. ': expected ' .. name .. ' wrapper', 3)
end
```

That is one call and one lookup in the private membership table, instead of two calls. Authentication stays by
membership: a table with a `handle` field is still rejected. `dispose` and `isDisposed` get the same shape, and `member`
remains only for `unwrapWidget`.

**Measured, not assumed.** The performance probe (`../wrappers-gate`, `deno task gate perf`) runs on v0.5.1 and on the
release candidate. The CHANGELOG reports both results. If the prologue saves less than 20 ns per call, the release says
so; the change stays either way, because it also fixes R1's extra frame.

## 4. One-table enumeration (R3)

In `group.lua`:
- `members` and `wrapAll` are replaced by one function that walks `BlzGroupUnitAt` and wraps each unit into a single
  array, skipping nil.
- `getUnits`, `forEach` and `first` keep their results and semantics. Every unit is still wrapped before any callback
  runs, so a unit removed mid-iteration keeps its disposed wrapper.
- A filtered enumeration still collects the rejected units and removes them after the filter has run, and still clears
  the group if the filter raises. It takes the raw handles from its own snapshot, not from `wrapper.handle`.

`Item.enumInRect` and `Destructable.enumInRect` already wrap in one pass; they are unchanged. The performance probe
measures `groupEnum20` and the missile tick before and after (§3).

## 5. Liveness and existence (R4, R5)

- **`unit:isAlive()`** returns `UnitAlive(raw)`. The 2026-09-29 probe found that it agrees with the old definition
  before and after a kill. The comment about Moonwell 0.5.0 goes. README, CONTRIBUTING and `AGENTS.md` state the
  minimum: Moonwell 0.5.1.
- **`unit:exists()`** returns `GetUnitTypeId(raw) ~= 0`. It is true for a living unit and for a corpse, and false once
  the game has removed the unit (decay, or removal by code that bypassed the wrapper). A disposed wrapper raises, like
  every other method; check `isDisposed()` first.
- **`item:exists()` and `destructable:exists()`** use `GetItemTypeId` and `GetDestructableTypeId`. They ship only if the
  in-game gate (§8) shows the type id reads 0 after the object is removed with the raw native. Otherwise they are left
  out, and the README says why.

## 6. Deterministic options errors (R12)

`Options.read` checks unknown keys, then fields, in sorted order:

- **Field names:** sorted once per field table and cached in a weak-keyed table.
- **Unknown keys in the given table:** reported in sorted order of `tostring(key)`.

So when several options are wrong, every machine reports the same one. Results, defaults and messages are otherwise
unchanged.

## 7. Documentation (R6, R7, R9)

- **R6: the API reference.** The README's single table becomes one subsection per module. Each module lists its
  factories and statics, then its methods by group (as the table did), one line per group. The content, order and
  wording of the entries stay the same. `exists()` and the `isAlive` note are added.
- **R7: the callback rule.** "Callbacks" opens with the rule: callbacks that run immediately let errors propagate
  (enumeration filters, `forEach`, `enumInRect`), and event callbacks are isolated and printed. It lists all five labels
  (`Timer`, `Trigger`, `Trigger condition`, `Dialog button`, `Frame event`), then keeps the existing timer and trigger
  details.
- **R9: conventions.** A short table: colour ranges (0–255, except `Lightning:setColor` at 0–1), angles (degrees for
  unit facing and `TextTag.float`, radians for `Effect:setOrientation`) and visibility names (`show`/`isHidden` for
  units and destructables, `setVisible`/`isVisible` for items). It names the natives each follows.
- **Error locations:** a sentence in "Handles and cleanup" says that wrapper errors point at the calling line.

## 8. Verification and release gate

Automated checks, all as in CONTRIBUTING:
- `deno task test`, with the new `blame` suite and the targeted tests;
- `check:lua` with Lua 5.3.6;
- `test:integration` (Moonwell builds, LuaLS fixtures, and the native-call check against Moonwell's declarations, which
  now include `UnitAlive`).

In-game gate (the maintainer, 3.0.0.24268):
1. **The existing gate runs, normal and minified,** on the gate map (`deno task gate core`, `core-min`, `probes`,
   `presentation`, `ui`, `frames` and their minified runs, as CONTRIBUTING lists). They show that nothing regressed.
   `probes` also shows that callback errors still print.
2. **Performance, before and after:** `deno task gate perf` on v0.5.1 (already measured on 2026-09-30) and on the
   release candidate.
3. **`exists()`:** a new gate step in `examples/gate.yue`, behind the existing `probes` flag.
   - It removes a unit, an item and a destructable with the raw natives (`RemoveUnit`, `RemoveItem`,
     `RemoveDestructable`), bypassing their wrappers, then prints `exists()` for each.
   - It kills a second unit and prints `isAlive()` and `exists()`, expecting `false true`.
   - Item and destructable `exists()` ship only if they printed false.
4. **Error locations in game:** one intentional error, a wrong-type argument such as `unit:issueTargetOrder('smart', {})`
   inside a `pcall`. It prints the message, which must name the gate file and line, not a wrappers file. This is added to
   the `probes` run.

Tag consumption as for every release. The online and multiplayer checks stay deferred to before Moonwell 1.0.

## 9. Plan phasing

One plan, `docs/superpowers/plans/2026-09-30-moonwell-wrappers-refactor.md`, of test-first tasks in this order:

1. `failsAt` and the `blame` sweep, which fail on today's code.
2. `Handle`: the prologue and the levels (§2.2, §3).
3. Parentheses on every raising tail call, module by module, until the sweep passes, then the targeted tests.
4. One-table enumeration.
5. `isAlive` and `exists()`.
6. Sorted options errors.
7. Docs.
8. The gate additions.

## 10. Out of scope

- **The port prerequisites** (collision size, pathing, sync, damage event data): v0.7.0, with its own spec.
- **Automatic disposal** of removed widgets: backlog, after libraries can ship object data (roadmap 4.2).
- **API renames** to smooth the native-faithful conventions (R9): documented, not changed.
- **Generating the API reference** from the LuaLS annotations.
