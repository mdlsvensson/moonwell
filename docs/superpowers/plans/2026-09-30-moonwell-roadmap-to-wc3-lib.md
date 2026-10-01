# Roadmap: from wrappers v0.5.0 to the wc3-lib port (2026-09-30)

This is a roadmap, not a task plan. It orders all the remaining work toward sub-project 4d, the YueScript port of
`wc3-lib`. It also places every other backlog item before or after that port. Each item that changes code still follows
the usual process when its turn comes. Small bounded items get a short design in chat and the maintainer's approval.
Larger items get a spec in `docs/superpowers/specs/` and a TDD plan in `docs/superpowers/plans/`.

The order is:

0. Free wins (small, uncomplicated items, done first).
1. Groundwork for the review and the port (a tool, measurements, an inventory and a probe).
2. The review of moonwell-wrappers, then its refactor.
3. The port.
4. The rest of the backlog.

## Where we are

- Moonwell CLI 0.5.0 is released. `main` has two unreleased changes: `UnitAlive` is now a known native, and the
  documentation is updated.
- moonwell-wrappers v0.5.0 is released:
  - 24 modules, about 3,400 lines of annotated Lua;
  - 167 behavior tests;
  - every release has passed its in-game gate and tag consumption.
- `wc3-lib` (`../wc3-lib`, `@mdlsvensson/wc3-lib` 0.1.1 on JSR) is about 6,000 lines of TypeScript for
  TypeScriptToLua:
  - its modules are core (scheduler, clock, scope, signal), time, dummy, buffs, damage, physics (missiles, knockback,
    terrain) and persistence (codec, format, Preload files, sync);
  - each has a pure core, a port interface and a thin `warcraft*.ts` adapter;
  - it has 77 behavior tests;
  - its `AGENTS.md` records runtime facts measured in game and an "opt-in" rule: an unused system costs nothing.

## Phase 0: free wins

**Done 2026-09-30.** 0.1: the maintainer confirmed that extension 0.2.10 opens no window, and the note is gone. 0.2:
Moonwell 0.5.1 (GitHub release `moonwell@0.5.1` on `c883e4c`; gate steps 3 to 12 not re-run by the maintainer's
decision). 0.3: wrappers v0.5.1 (`af9961e`, tag consumption passed).

Each item here is small and needs nothing else first. Do them in this order.

### 0.1 Drop the `yue` console-window note (docs only)

- **Why it's done now:** the fix we contributed (`windowsHide`, pigpigyyy/yuescript-vscode#11) shipped in YueScript
  extension 0.2.10 on 2026-09-27.
- **What to do:**
  - The maintainer updates the extension and confirms that no console window opens.
  - Remove the "A `yue` console window on Windows" item from Moonwell's `README.md`.
  - Remove the 0.2.9 remark from the Plan 3a bullet in `AGENTS.md`.
  - Change the editor gate's extension version in `CONTRIBUTING.md`, if it names one.

### 0.2 Moonwell 0.5.1 patch release

It releases what is already on `main`, and fixes the one deferred item.

- **`UnitAlive` is a known native.** It is already on `main`. The wrappers need a released Moonwell with this change
  before they can call `UnitAlive` (item 2.2).
- **`assets:sync` and Ctrl+C.** Today Ctrl+C is checked only while writing. Also check it while planning, so the command
  stops before it writes anything. This is the only item under "Open, deliberately deferred"; that section becomes
  empty.
- **Release gate:** the automated steps 1–6. The maintainer runs `deno publish` and the `init` check from JSR.

### 0.3 Editor error for effects on items and destructables (wrappers v0.5.1)

This is a backlog item, and the design was already decided on 2026-09-29.

- **Editor:** `Effect.attach` and `Effect.flashOn` annotate their target as `MoonwellWrappers.Unit`. This makes LuaLS
  flag an Item or a Destructable argument.
- **Runtime:** it stays permissive (`Handle.unwrapWidget`), for custom models.
- **Tests:** two new negative lines in the editor fixture, one for an Item and one for a Destructable.
- **Docs:** the README row and the note are updated.
- **Also in this patch:** update the stale comment on `unit:isAlive()` once 0.2 has shipped.
- **Gate:** the automated checks and tag consumption. The runtime is unchanged, so no in-game gate is needed.

## Phase 1: groundwork

These items produce a tool, numbers and a list. Together they make the review factual and let the port's biggest
decision (item D1) be made from evidence.

### 1.1 Static native-call check (backlog; short design first)

**Done 2026-09-30, differently from the plan below** (wrappers `227e142`). A planted test showed that LuaLS with
Moonwell's `natives.d.lua` already reports a misspelt native (`undefined-global`), a missing or extra argument
(`missing-parameter`, `redundant-parameter`) and a wrong type (`param-type-mismatch`). So the maintainer approved
automating the direct LuaLS run in the wrappers' `test:integration` instead of writing a tokenizer:
- it checks `src/wrappers` against the native declarations, and every library file must be clean;
- `tests/natives-negative.lua` must report exactly its four planted mistakes.

The consumer's positive run turned out to diagnose the library's copies in `.moonwell/lua/` as well. The port's library
can reuse the same pattern.

A check in the wrappers' `deno task test`. It reads every native call in `src/wrappers/` and compares it with Moonwell's
`cli/data/natives.json`, checking that:

- the native exists;
- the call passes the right number of arguments.

The WCSharp note (§7.3) describes the idea. Two questions remain open:

- **How to find calls.** My recommendation: a small Lua tokenizer in `tools/`, not a regex.
- **Where `natives.json` comes from.** My recommendation: a copied snapshot in `tools/`, plus a test that compares it
  with `../moonwell/cli/data/natives.json` when that folder exists.

**Why it's before the review:** the refactor will touch every module, and this check guards native names and argument
counts at no cost. The port's library reuses the same check.

### 1.2 Performance measurement (research, gate-map probe)

**Done 2026-09-30:** `docs/superpowers/research/2026-09-30-wrappers-performance.md`.
- A wrapped native call costs about 120 ns more than the raw call (1.26–1.38 times).
- A wrapped enumeration of 20 units costs 1.5 times the raw one.
- A 100-missile tick takes 1.69 ms through the wrappers against 0.97 ms raw: 5.4% of a tick against 3.1%.
- The note proposes a budget (a system's work under 10% of a tick) and two review candidates (a cheaper method
  prologue and cheaper enumeration), and leans towards option (c) for D1.

The WCSharp note (§4.1) asks for a performance budget before 4d builds systems on top of the wrappers.

- **What to measure,** using `os.clock` in the gate map, each over N calls, raw native against wrapper:
  - `unit:getX`/`setX`;
  - `effect:setPosition`;
  - `Unit.fromHandle` on a cached handle;
  - one `Group` enumeration;
  - a timer callback through `Callback.call`;
  - one options-table validation.
- **Hot paths it covers:** missiles and knockback at 32 ticks per second; damage events.
- **Output:** a research note with the numbers and a proposed budget, for example the wrapper cost per missile per tick.

### 1.3 Port-needs inventory (research)

**Done 2026-09-30:** `docs/superpowers/research/2026-09-30-wc3-lib-port-needs.md`.
- Of 76 natives `wc3-lib` calls, the wrappers call 46. Damage event data, sync, `SetUnitPathing`,
  `BlzGetUnitCollisionSize` and walkability are missing; Preload files and ground height are best left raw.
- New design inputs: the port needs insertion-ordered collections, because `pairs` over handle-keyed tables can desync.
  Handle ids must not order anything; the port uses sequence numbers it owns.
- The note recommends option (c) for D1 and lists five open questions for the port's spec.

A research note that maps what `wc3-lib`'s adapters and cores use against the wrappers.

- **Every native call** is marked covered, missing, or best left raw. A first pass over the adapters found these not
  covered:
  - damage event data (`BlzGetEvent*`, `BlzSetEvent*`, `GetEventDamage`);
  - sync (`BlzSendSyncData`, `BlzTriggerRegisterPlayerSyncEvent`, `BlzGetTriggerSyncData`);
  - Preload file I/O;
  - ability tooltips (`BlzGetAbilityTooltip`, `BlzSetAbilityTooltip`);
  - locations and `GetLocationZ`;
  - `IsTerrainPathable`, `BlzGetUnitCollisionSize`, `SetUnitPathing`;
  - generic unit state (`GetUnitState`, `SetUnitState`).
- **Every design input** already gathered goes into the note:
  - the w3ts comparison §2.1 and §7.5;
  - the WCSharp comparison §2.1 and §7.4: event multiplexing with deferred updates, one multiplexed timer, sync packets
    of 255 characters, Preload save and load, synchronized time, dummy recycling.
- **`wc3-lib`'s `AGENTS.md` §2–§4:** its runtime facts, its invariants and its decisions that must survive the port.
  The TypeScriptToLua hazards drop out; YueScript's own pitfalls replace them.

### 1.4 Probe batch for the port (gate map, one run: `deno task gate probe-port`)

**Done 2026-09-30:** results in the port-needs note, §6.
- `os.time()` works, and integers wrap silently at 2^31.
- The undefend order detects removal: Defend level 0; death is level 1.
- Sync arrives in order but silently cuts at 255 characters.
- Preload keeps 259 characters per line; a cut line crashed the game on read.
- Appending chunks inside a Preload file does not work, so `wc3-lib`'s local store cannot read back codes over one
  chunk.

**Phase 1 is complete.**

These are questions that single-player can answer:

- **Does `os.time` work?** The two records disagree:
  - Moonwell's Plan 3a probe lists `os.time`;
  - `wc3-lib`'s `AGENTS.md` fact 3 says it does not exist.

  The 1.2 probe found that it exists as a function, so what remains is whether calling it works and what it returns.
- **Number width.** Partly answered by the 1.2 probe: `math.maxinteger` is 2^31−1 and `2^24 + 1` equals `2^24`. So
  integers are 32-bit and floats single precision, as `wc3-lib` says; the port's codec and scheduler depend on it.
  What remains is whether integers wrap past 2^31−1.
- **Unit removal detection** with the undefend-order trick that unit indexers use. This decides the backlog item
  "Automatic disposal of Unit wrappers on removal". The world-bounds leave event already failed (the probe of
  2026-09-29).
- **`BlzSendSyncData` round trip** for the local player: the 255-character limit and the order of messages.
- **Preload write and read round trip** on the local machine. The write works: the 1.2 probe wrote its results with
  Preload. The read back is still open.

Questions that need two machines (for example `GetLocationZ` differing between them) join the online checks in phase 4.

### D1: decision point (the maintainer)

With 1.2 and 1.3 in hand, decide how the port's adapters reach the game:

- **(a) Through the wrappers only.** The wrappers must then cover everything in 1.3.
- **(b) Raw natives in adapters.** The wrappers are used only where the map sees objects.
- **(c) A mix,** with the performance budget deciding the hot paths.

This decides how much of 1.3's "missing" list becomes wrappers work in phase 2.

**Decided 2026-09-30: (c), a mix** (the port-needs note, §4).
- The port's public API takes and returns wrappers, and its adapters use wrapper methods.
- Raw natives stay in four places: Preload files with the tooltip mailbox, ground height, walkability sampling, and the
  innermost per-tick loops on objects the system owns.
- The maintainer also decided that **damage event data belongs in the wrappers**, usable by any map.

So 2.3 adds `unit:getCollisionSize()`, `unit:setPathing(flag)`, sync (registration, sending, event data) and damage
event data (reading and changing a hit).

## Phase 2: wrappers review and refactor

### 2.1 Review (research note; the maintainer chooses the findings)

**Done 2026-09-30:** `docs/superpowers/research/2026-09-30-wrappers-review.md`.
- It found no correctness bugs.
- The maintainer chose every finding for v0.6.0: error locations (R1, R11), a cheaper method prologue (R2),
  one-table enumeration (R3), `UnitAlive` (R4), `exists()` (R5), sorted option errors (R12), and the docs R6, R7 and R9.
- The port prerequisites (2.3) are a separate v0.7.0.

A module-by-module audit of `src/wrappers/`, its internals, its tests and its docs. The result is a list of numbered
findings, each with a severity and a recommendation. It checks:

- **Consistency:**
  - method names (`get`/`set`/`is`), option keys, the error message format (`Class.method: …`);
  - nullable returns, removable tokens, idempotent cleanup;
  - behavior when a wrapper has been disposed.
- **Repeated patterns.** For example, the owned-object lifecycle is repeated across text tags, sounds, lightning,
  images, ubersplats, fog modifiers and effects. Does it belong in an internal helper, or is repeating it clearer?
- **Size and splitting.** `frame.lua` (496 lines), `unit.lua` (371) and `trigger.lua` (273).
- **Registry and caches.** Strong and weak caches, and identity after disposal.
- **Hot-path cost** against the budget from 1.2. For example, skipping repeated validation on setters, or local caching
  of natives.
- **Tests.**
  - Gaps in the behavior tests.
  - The quality of the native doubles in `tests/support.lua`.
  - Whether the harness (`tools/test.ts`, `tests/support.lua`) should become shared by the wrappers and the port's
    library, or stay copied.
- **Documentation.** The README is 389 lines, and its API table has single rows over 1,000 characters. Would per-module
  sections, or a separate API reference, read better?
- **Gaps from 1.3** that D1 assigned to the wrappers.

The maintainer picks the findings to act on, by multiple-choice questions.

### 2.2 Refactor release (wrappers v0.6.0; spec, plan and full gate)

**Released 2026-09-30:** wrappers `v0.6.0` on `933b580`; tag consumption passed.
- **Spec and plan:** `2026-09-30-moonwell-wrappers-refactor`.
- **In game:**
  - Errors point at the caller.
  - The fixed method cost fell from about 120 ns to 70 ns.
  - Enumerating 20 units costs 27.0 µs, down from 29.2 µs.
  - The missile tick is unchanged: natives dominate it.
  - A removed unit's `exists()` reads `false` from the next frame.
- **Gate:** frames and the minified runs were not re-run, by the maintainer's decision.

It contains:

- the chosen findings;
- `unit:isAlive()` through `UnitAlive`, which requires Moonwell 0.5.1 or later; README and CONTRIBUTING state the
  minimum;
- breaking changes, which are allowed before 1.0; the CHANGELOG lists each one with a migration line.

It gets the full in-game gate, normal and minified, and tag consumption.

### 2.3 Port prerequisites (wrappers v0.7.0, or folded into 2.2 if small)

What D1 assigned to the wrappers. Likely candidates from the backlog's "Wrappers candidate additions" and from 1.3:

- **Trigger registrations:** sync and timer expiry.
- **Damage event data.**
- **Event helpers** such as `Unit.fromEvent()`.
- **Automatic Unit disposal on removal,** if 1.4 found a working detection method.

**Released 2026-09-30:** wrappers `v0.7.0` on `e9c2880`; tag consumption passed. D1 and the maintainer's scope choice
kept it to `unit:getCollisionSize()`, `unit:setPathing(flag)`, `wrappers.damage` and `wrappers.sync` (spec and plan
`2026-09-30-moonwell-wrappers-port-prerequisites`). Event helpers, timer-expiry registration and automatic Unit disposal
stay in phase 4.1 and the backlog.

## Phase 3: the wc3-lib port (sub-project 4d; spec first)

### Spec decisions

- **Repository name and home:** a sibling repository consumed like the wrappers (`dir = "src"`).
- **Adapter strategy:** from D1.
- **Module layout:** one module per `wc3-lib` entry point, which preserves the opt-in rule. That rule matches the
  wrappers' rule: nothing is created at import.
- **Test strategy:** port the 77 behavior tests to the `yue -e` harness with native doubles, plus the static native
  check from 1.1.
- **Gate:** an in-game gate per release, modelled on `wc3-lib`'s `testbed/main.ts`.
- **Runtime facts:** carry over `wc3-lib`'s measured facts, corrected by 1.4.

### Suggested releases

Each release gets its own plan and gate:

1. core (scheduler, Warcraft clock, scope, signal) and time;
2. dummy and buffs, including auras;
3. damage;
4. physics: geometry, terrain, missiles and knockback;
5. persistence: codec, format, Preload files and sync.

**Spec:** `docs/superpowers/specs/2026-09-30-moonwell-systems-design.md` (2026-09-30): the library is
`mdlsvensson/moonwell-systems`, annotated Lua on the wrappers, with Lua-only tooling. **Release 1 released
2026-09-30:** moonwell-systems `v0.1.0` on `1725436` (plan `2026-09-30-moonwell-systems-release-1`); gate and tag
consumption passed. **Release 2 released 2026-10-01:** `v0.2.0` on `afabc3d` (spec and plan
`2026-09-30-moonwell-systems-release-2`); gate and tag consumption passed. **Release 3 released 2026-10-01:** `v0.3.0`
on `d67d3fc` (spec and plan `2026-10-01-moonwell-systems-release-3`); gate and tag consumption passed. **Release 4
released 2026-10-01:** `v0.4.0` on `cfa21b6` (spec and plan `2026-10-01-moonwell-systems-release-4`); both gate
runs and tag consumption passed. **Release 5 released 2026-10-01:** `v0.5.0` on `11331a8` (spec and plan
`2026-10-01-moonwell-systems-release-5`); gate and tag consumption passed. **Phase 3 is complete.**

## Phase 4: after the port (the rest of the backlog, in suggested order)

Every item needs a short design first, as the backlog says.

1. **Wrappers candidate additions the port did not need:**
   - `WeatherEffect`;
   - spell effects from ability data (`Effect.flashSpell` via `AddSpellEffectById`);
   - the remaining Trigger registrations: player state, key, mouse, alliance change and game state.

   **Released 2026-10-01** as wrappers `v0.8.0` on `d823b1b` (spec and plan
   `2026-10-01-moonwell-wrappers-additions`): `wrappers.input` for keys and the mouse, `wrappers.weathereffect`,
   `Effect.abilityArt` in place of spell-effect constructors, four Trigger registrations (with timer expiry) and
   `fromEvent()`. The gate and tag consumption passed.
2. **Assets shipped by libraries.** Frame template `.toc` and `.fdf` files, and possibly other files a library needs.
   The port's dummy units need object data from the map. Whether libraries should ship object data too can be designed
   together with this.

   **Released 2026-10-01** as Moonwell 0.6.0 (spec and plan `2026-10-01-moonwell-library-assets`): a library names
   its module folder and a folder of files for the map in its own `moonwell-library.json`. Files only, by the
   maintainer's choice: object data stays in the map, and the dummy unit stays a pasted Pkl block.
3. **Custom map preview for Reforged.**
4. **Other gameplay languages.** Teal first: its compiler is Lua, so it needs no Node.js, and its types map to the
   annotated Lua we already write. Then Fennel, TypeScript and C#; the last two need an answer to the "no Node.js" rule.
5. **Online multiplayer and desync checks: the very last step before 1.0,** as the maintainer decided. The backlog entry
   lists what they cover. They now also cover:
   - the port's sync and save systems;
   - synchronized time;
   - the machine-local questions left over from 1.4.

## Backlog coverage

| Backlog item                                                  | Where                |
| ------------------------------------------------------------- | -------------------- |
| `assets:sync` Ctrl+C during planning (deferred)               | 0.2                  |
| Editor error for effects attached to items and destructables  | 0.3                  |
| Static native-call check for the wrappers                     | 1.1 (done)           |
| Automatic disposal of Unit wrappers on removal                | 1.4 probe, then 2.3  |
| Wrappers candidate additions                                  | 2.3, 4.1 (done)      |
| YueScript port of `wc3-lib` (4d)                              | Phase 3 (done)       |
| Assets shipped by libraries                                   | 4.2 (done)           |
| Custom map preview for Reforged                               | 4.3                  |
| Fennel, TypeScript, C# and Teal support                       | 4.4                  |
| Online multiplayer and desync checks                          | 4.5 (last before 1.0) |
| UI wrappers, releases B and C                                 | Done (v0.4.0, v0.5.0) |
