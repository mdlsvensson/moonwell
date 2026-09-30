# Moonwell Systems: the wc3-lib Port — Design

- **Date:** 2026-09-30
- **Status:** The design was approved in chat on 2026-09-30; this written spec awaits review.
- **Roadmap:** phase 3 (sub-project 4d), `docs/superpowers/plans/2026-09-30-moonwell-roadmap-to-wc3-lib.md`.
- **Inputs:**
  - `docs/superpowers/research/2026-09-30-wc3-lib-port-needs.md` (§3 design inputs, §5 open questions, §6 the 1.4
    probe);
  - `docs/superpowers/research/2026-09-30-wrappers-performance.md`;
  - decision D1 (option (c), a mix) and wrappers v0.7.0 (`wrappers.damage`, `wrappers.sync`, collision size, pathing);
  - `wc3-lib` itself (`../wc3-lib`, TypeScript, about 3,000 lines of library code and 77 tests).
- **Target:** a new sibling library, `mdlsvensson/moonwell-systems`, released as `v0.1.0` to `v0.5.0`. It requires
  moonwell-wrappers `v0.7.0` or later and Moonwell 0.5.2 or later. The Moonwell CLI does not change.

This spec has two parts. Part 1 fixes what every release shares. Part 2 designs release 1 in full. Releases 2 to 5 each
get their own short spec, which builds on Part 1, when they start.

# Part 1: the port as a whole

## 1. Intent and scope

Bring `wc3-lib`'s systems (scheduler, buffs and auras, dummies, damage pipeline, missiles and knockback, save codes,
time) to Moonwell maps, written for Warcraft's Lua and built on the wrappers. It is a port of the behavior, not a
line-by-line translation: where `wc3-lib`'s design existed only because it ran as TypeScript, or where the probes found
it wrong for the game, the port does what is right for Lua (the maintainer's instruction: "take the one that is the
best design, not whether it's the way it was before").

## 2. Repository and consumption

- Repository `../moonwell-systems`, GitHub `mdlsvensson/moonwell-systems`, a public library like the wrappers, consumed
  with `dir = "src"`. Modules live in `src/systems/` and are named `systems.<name>`.
- Moonwell libraries cannot declare dependencies (libraries spec §4), so a map lists both libraries:

  ```pkl
  libraries {
    ["wrappers"] { github = "mdlsvensson/moonwell-wrappers"; tag = "v0.7.0"; dir = "src" }
    ["systems"] { github = "mdlsvensson/moonwell-systems"; tag = "v0.1.0"; dir = "src" }
  }
  ```

  The README states the minimum wrappers version. A systems module that requires a missing wrappers module fails the
  build with Moonwell's usual "Module not found" error.
- Tags are immutable GitHub pre-releases, as for the wrappers.

## 3. Language and structure

- **Annotated Lua 5.3**, like the wrappers: LuaLS classes named `MoonwellSystems.<Name>`, so YueScript and Lua maps get
  editor types. Maps use it from YueScript as usual.
- **Pure logic stays pure.** Modules whose logic needs no game (scheduler, signal, scope, calendar, save codec, geometry,
  the damage pipeline's ordering) call no native and take plain values. They are tested with plain values.
- **Game-facing systems call the wrappers directly.** `wc3-lib` put every native behind a swappable "port" interface
  because its tests ran as JavaScript, where natives cannot exist. In Lua the real code runs against native doubles, as
  the wrappers' tests do, so the indirection buys nothing, costs a function call per operation in hot loops, and makes
  every map wire adapters. There are no port interfaces.
- **Public API takes and returns wrappers** (`MoonwellWrappers.Unit`, `Player`, `Timer`, …).
- **Raw natives only in three places** (D1): Preload files, terrain sampling (ground height, walkability), and the
  innermost physics loops, which may use `getHandle()` of objects the system itself owns.

## 4. Shared rules

### 4.1 Lifecycle

- Importing a module creates nothing and calls no native.
- Everything that owns a handle or a timer starts explicitly (`new`, `start`) and has an idempotent `dispose()`.

### 4.2 Errors

- Messages read `[systems] <Class>.<method>: <problem>`, for example `[systems] Scheduler.after: expected a finite
  non-negative delay`.
- Every error points at the line that called the public function, with the wrappers' rules: explicit levels (2 in a
  public function, 3 in a helper it calls) and no tail call into a raising helper (`return (helper(...))`). A sweep
  test, like the wrappers' `tests/blame.lua`, calls every public function with wrong arguments and checks the position.

### 4.3 Callbacks

- Every user callback runs behind a boundary: `systems.internal.callback`, a copy of the wrappers' `Callback` (the
  maintainer chose a copy over making the wrappers' one public).
- A failure goes to the owning system's optional `onError(message)`, itself called behind the boundary, or is printed as
  `[systems] <label> failed: <message>`.
- Nothing is rethrown: an error rethrown inside a timer or trigger callback is silent in game.

### 4.4 Determinism

- Collections keyed by units or other handles keep insertion order: `systems.internal.ordered`, an ordered map and set
  that allow removal during iteration (added in release 2, the first to need it).
- No `pairs` loop over a table keyed by handles or wrappers when the loop calls natives or changes game state.
- Ties are broken by sequence numbers the system assigns, never by `GetHandleId` (handle ids can differ between
  machines in Lua). Engine enumeration order is synchronized and may be kept.

### 4.5 Units

- One liveness definition: `unit:isAlive()` (the wrappers' `UnitAlive`).
- A system that tracks units drops removed ones by polling `unit:exists()` every 0.25 s from its scheduler (the
  maintainer's choice; it needs no object data, unlike the undefend order).

### 4.6 Numbers and YueScript

- Warcraft's integers are 32-bit and wrap silently; floats are single precision (the 1.2 and 1.4 probes). Code checks
  ranges before multiplying and never relies on a wrapped result.
- The tests run under `yue -e`, which is Lua 5.4 with 64-bit integers, so overflow cannot be observed there: range
  checks are tested at their boundaries instead.
- The library is Lua, so YueScript's `//` bug does not apply to it; its examples and gate code avoid `//`.

## 5. Tooling without Deno

The maintainer plans to drop Deno eventually. The new repository has no Deno configuration of its own:

- **Tests:** `tests/run.lua`, run with `yue -e tests/run.lua [suite ...]`. The suites are listed in `tests/suites.lua`.
  Each suite runs in a fresh environment: the runner restores a snapshot of `_G` and clears `systems.*` and `wrappers.*`
  from `package.loaded` between suites. `tests/support.lua` is a small copy of the wrappers' helpers (`eq`, `fails`,
  `failsAt`, `native`, call counts). `package.path` includes `src/` and the wrappers' `src/` (`../moonwell-wrappers/src`
  by default, or `MOONWELL_WRAPPERS`).
- **Lua 5.3 syntax:** `tools/check.lua` runs `luac53 -p` (path from `MOONWELL_LUAC`) on every file under `src/`.
- **Integration:** `tools/integration.lua` shells out to the Moonwell CLI (`MOONWELL_CLI`, a command line) to make a
  consumer map with both libraries as local paths, check and build it normal and minified, run LuaLS (`MOONWELL_LUALS`)
  over positive and negative fixtures, and check that a map importing one entry point bundles only that module and what
  it requires.
- Moonwell's CLI itself stays on Deno. A new backlog item, "Replace Deno in Moonwell's toolchain", records the
  maintainer's plan for a later design.

## 6. Releases

| Release | Modules | Notes |
| --- | --- | --- |
| v0.1.0 | `systems.scheduler`, `systems.signal`, `systems.scope`, `systems.time` | Part 2 |
| v0.2.0 | `systems.dummy`, `systems.buffs`, `systems.aura`, `systems.internal.ordered` | removal polling |
| v0.3.0 | `systems.damage` | pipeline on `wrappers.damage` |
| v0.4.0 | `systems.geometry`, `systems.terrain`, `systems.missile`, `systems.knockback` | hot loops, perf probe |
| v0.5.0 | `systems.codec`, `systems.savefile`, `systems.sync` | starts with the Preload carrier probe |

Each release: a short spec, a plan of test-first tasks, the automated checks, an in-game gate run on the gate map
(`../wrappers-gate`, with the systems library added as a local path), tag, GitHub pre-release and tag consumption with
both libraries. Online and multiplayer checks wait for the step before Moonwell 1.0.

`wc3-lib`'s save store is broken in game (port-needs §6.5): lines over 259 characters are cut and can crash the game,
and appending inside a file does not work. Release 5 starts with a probe of multi-line carriers (one tooltip or ability
per chunk, joined in Lua) before its spec.

# Part 2: release 1 (v0.1.0)

## 7. `systems.scheduler`

A deterministic fixed-step clock, pure except for `start()`.

```lua
---@param stepSeconds number? Seconds per tick; finite and positive. Default 1/32.
---@param onError fun(message: string)? Receives task failures; default prints them.
---@return MoonwellSystems.Scheduler
function Scheduler.new(stepSeconds, onError)
function Scheduler:after(seconds, callback)   -- returns fun() that cancels
function Scheduler:every(seconds, callback)   -- returns fun() that cancels
function Scheduler:advance()
function Scheduler:getTick()       -- integer ticks advanced
function Scheduler:getElapsed()    -- getTick() * stepSeconds
function Scheduler:getPending()    -- tasks not yet run or cancelled
function Scheduler:getStep()
function Scheduler:ticks(seconds)  -- whole ticks a delay occupies
function Scheduler:start()         -- drives advance() from one wrappers Timer; returns fun() that stops it
function Scheduler:dispose()
```

Behavior, from `wc3-lib` unless marked:
- **Rounding:** a delay becomes `max(1, ceil(seconds / step - 1e-4))` ticks; the epsilon absorbs single-precision error
  (for example 0.07 / 0.01).
- **Order:** tasks live in a binary min-heap keyed by (due tick, creation sequence), so tasks due on the same tick run in
  creation order, repeating ones included. A tick costs O(k log n) for the k tasks due.
- **During a tick:** tasks scheduled while it runs are due at least one tick later. A task cancelled by an earlier task
  of the same tick does not run.
- **Repeating:** `every` first runs one interval from now; `every(0, …)` raises.
- **Failures:** a failing task is cancelled, repeating or not, and its message goes to `onError` or is printed as
  `[systems] Scheduler task failed: …` (new: `wc3-lib` rethrew after the tick, which is silent in game).
- **Reentrancy:** `advance()` from inside a task raises `[systems] Scheduler.advance: cannot advance during a tick`.
- **Cancel functions** are idempotent: cancelling twice, or after the task ran, does nothing.
- **`start()`** (new; `wc3-lib` had a separate `startWarcraftClock`): creates one periodic wrappers `Timer` with period
  `step` whose callback calls `advance()`. A second `start()` while running raises. The returned stop function and
  `dispose()` destroy the timer; stopping twice does nothing. The module requires `wrappers.timer`, which importing
  does not call.
- **`dispose()`** cancels every task and stops the timer. Scheduling afterwards raises (`the scheduler is disposed`);
  `advance()` afterwards does nothing.
- **Checks:** a step or delay that is not a finite non-negative number raises `expected a finite non-negative delay`
  (`expected a finite positive step` for the step); a non-function callback raises `expected a callback function`.

## 8. `systems.signal`

```lua
function Signal.new(onError)                  -- onError: fun(message: string)?
function Signal:subscribe(callback, priority) -- priority: finite number, default 0; returns fun() that unsubscribes
function Signal:emit(...)                     -- passes every argument to each listener
function Signal:getCount()                    -- live listeners
function Signal:dispose()
```

- Lower priority runs first; equal priorities keep subscription order.
- An emit delivers to the listeners present when it started: one added during the emit waits for the next, one removed
  during it is skipped.
- Each listener runs behind the callback boundary, labelled `Signal listener` (new: in `wc3-lib` an error escaped
  `emit` and stopped the rest).
- Subscribing after `dispose()` raises; emitting after it does nothing.

## 9. `systems.scope`

```lua
function Scope.new(onError)     -- onError: fun(message: string)?
function Scope:own(release)     -- a function; returns it
function Scope:add(value)       -- anything with dispose, destroy or remove (checked in that order); returns value
function Scope:isActive()
function Scope:dispose()
```

- `dispose()` runs releases in reverse order of registration and is idempotent.
- One failing release never stops the others; each failure goes to `onError` or is printed as
  `[systems] Scope release failed: …` (new: `wc3-lib` rethrew the first).
- Owning or adding after `dispose()` releases at once.
- `add` raises `expected a value with dispose, destroy or remove` for anything else, so wrappers (`timer:destroy()`,
  `unit:remove()`) and systems (`clock:dispose()`) can be added directly.

## 10. `systems.time`

Pure calendar and display helpers, plus the local clock.

```lua
function Time.isLeapYear(year)          -- integer 1..9999
function Time.utcToUnix(date)           -- {year, month, day, hour?, minute?, second?} -> integer or nil
function Time.unixToUtc(seconds)        -- integer -> {year, month, day, hour, minute, second} or nil
function Time.dayOfWeek(seconds)        -- 0 = Sunday .. 6 = Saturday, or nil
function Time.formatUtc(date)           -- "YYYY-MM-DD HH:MM:SS"
function Time.formatDuration(seconds)   -- "M:SS", or "H:MM:SS" from one hour; negative reads 0:00
function Time.localUtc()                -- os.time() as integer Unix seconds, or nil
```

Changes from `wc3-lib`:
- **Range:** timestamps are 32-bit integers, from −2,147,483,648 (1901-12-13 20:45:52) to 2,147,483,647 (2038-01-19
  03:14:07). `utcToUnix` returns nil for a date outside it and checks the day count before multiplying, so a result
  never wraps. `unixToUtc` returns nil for a non-integer or a value outside it. (`wc3-lib` accepted years 1 to 9999,
  which do not fit Warcraft's integers.)
- **Local clock:** `localUtc()` returns `os.time()`, which the 1.4 probe found present and correct (`wc3-lib` read
  `os.date("!*t")`, believing `os.time` absent). It returns nil when `os.time` is missing, raises or gives a value
  outside the range. The value is this machine's clock and untrusted: sync it before it affects shared state.
- **Dropped:** `SimulationTime` (use `scheduler:getElapsed()`) and `LocalWallTime` (use `localUtc()`), one-line
  wrappers that only existed as TypeScript types.

## 11. Verification

Automated (§5):
- behavior suites `scheduler`, `signal`, `scope`, `time`, covering `wc3-lib`'s ten core and time tests (adapted to
  the changes above) and the new rules: failures printed or passed to `onError`, `start()` and `dispose()` with timer
  doubles, the time range boundaries, `localUtc()` with a missing, raising and out-of-range `os.time`;
- `imports`: importing every module calls no native and creates nothing;
- `blame`: every public function given wrong arguments points at its caller;
- Lua 5.3.6 syntax, LuaLS positive and negative fixtures, and integration builds, including one-entry-point bundles
  (`systems.time` alone bundles no wrappers module; `systems.scheduler` bundles `wrappers.timer` and what it requires).

In-game gate (the maintainer, 3.0.0.24268), one run `deno task gate systems` on the gate map, normal build:
1. **Timing:** a started scheduler runs `after(1)` and `every(0.5)`; the log prints `os.clock()` differences near 1.0
   and 0.5 s, and `getElapsed()` matching them.
2. **Order:** three tasks due on the same tick print in creation order.
3. **Failure:** a repeating task that raises prints `[systems] Scheduler task failed: …` once and never runs again,
   while the others go on.
4. **Signal and scope:** priorities print lowest first; a failing listener prints and the next runs; a scope with a
   timer, a unit and a release function releases them in reverse order, and the unit disappears.
5. **Time:** `Time.localUtc()` and `formatUtc(unixToUtc(...))` print the current UTC time; `formatDuration` prints a
   countdown.
6. **Stop:** `dispose()` stops the timer; no task prints afterwards.

Then tag `v0.1.0` and check tag consumption in a fresh map with both libraries from GitHub.

## 12. Plan phasing (release 1)

One plan, `docs/superpowers/plans/2026-09-30-moonwell-systems-release-1.md`:
1. Repository skeleton: README, AGENTS, CONTRIBUTING, `.luarc.json`, the Lua test runner, support, check and
   integration scripts, the GitHub repository.
2. `systems.internal.callback`.
3. `systems.scheduler` (pure part).
4. `Scheduler:start()` and `dispose()` with the timer.
5. `systems.signal`.
6. `systems.scope`.
7. `systems.time`.
8. Sweep, imports, editor fixtures and integration.
9. Docs, the gate example and the gate map's `systems` run.
10. Release, and Moonwell's records (including the backlog item for Deno).

## 13. Out of scope

- Releases 2 to 5 (their own specs).
- A Moonwell feature for dependencies between libraries.
- Replacing Deno in Moonwell's CLI (backlog).
- Online and multiplayer checks (before Moonwell 1.0).
