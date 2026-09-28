# Moonwell Native Wrappers (sub-project 4c) — Design

- **Date:** 2026-09-28
- **Status:** Approved and implemented locally on 2026-09-28. Automated and in-game checks pass; first-tag consumption
  remains pending. Library commit: `3b923d5` in the separate `moonwell-wrappers` repository.
- **Builds on:** `2026-09-28-moonwell-lua-libraries-design.md` (4a and 4b, released in Moonwell 0.5.0).
- **Target:** First independent library release, `v0.1.0`.

## 1. Intent and scope

Provide an optional object-style API for everyday Warcraft III gameplay, usable from YueScript and Lua, with editor
completion, typed parameters and return values, predictable handle identity and explicit resource cleanup.

The maintainer selected a focused foundation: Player, Unit, Timer, Trigger, Group and Effect. Broad coverage of other
handle types is a separate backlog item in Moonwell's AGENTS.md. This library provides primitives on which gameplay
libraries, including the future wc3-lib port, can build. It does not implement that port's scheduler or other systems.

Use handwritten Lua 5.3 with LuaLS annotations. Types support editor diagnostics; Moonwell does not add a static type
checker or runtime Warcraft handle-type introspection. Existing raw natives remain available alongside the wrappers.

## 2. Repository and distribution

Use a separate repository, proposed name `mdlsvensson/moonwell-wrappers`, with MIT licensing and independent `vX.Y.Z`
tags. The repository name is a design choice, not a claim that the remote repository already exists.

```text
moonwell-wrappers/
  src/wrappers/
    player.lua
    unit.lua
    timer.lua
    trigger.lua
    group.lua
    effect.lua
    internal/handle.lua
    internal/callback.lua
  tests/                 Lua tests, native doubles and editor fixtures
  tools/                 Deno test runner and verification helpers
  examples/              documented YueScript and Lua usage
  deno.json
  README.md
  CONTRIBUTING.md
  CHANGELOG.md
  LICENSE
```

Only `src/` is the published module root. Tests, examples and tools are outside it. No package.json, Node.js, npm
dependencies, custom library manifest or install script. Deno tooling uses only the existing project's permitted
`jsr:@std/*` family. There is no required runtime library dependency, including on the built-in `moonwell` module.

Consuming maps use Moonwell >=0.5.0 and configure:

```pkl
libraries {
  ["wrappers"] {
    github = "mdlsvensson/moonwell-wrappers"
    tag = "v0.1.0"
    dir = "src"
  }
}
```

For local development, override the same key in `moonwell.local.pkl` with `path = "../moonwell-wrappers"` and
`dir = "src"`. Moonwell handles syncing, the lock and the editor view. No CLI change is planned.

Import each class from `wrappers.<name>`. Do not add an umbrella module in v0.1.0: the existing bundler follows
literal requires, so separate imports let maps include only the modules they need. All internal requires are literal.
Do not create global aliases such as `Unit` or `Player`; exports are module-local tables.

## 3. Calling convention

Factories use dot calls; instance methods use colon calls in Lua and backslash calls in YueScript. Native constants
and numeric object ids pass through unchanged. Methods expecting another wrapped object accept that class only;
raw handles are converted explicitly with `fromHandle`. Mutators return no value unless specified below.

```yue
import "wrappers.unit" as Unit
import "wrappers.player" as Player
import "moonwell.macros" as {:$FourCC}

footman = Unit.create Player.fromIndex(0), $FourCC("hfoo"), 0, 0, 270
footman\setLife 250
footman\setPosition 100, 200
```

Use explicit getters and setters, not intercepted properties, overloaded constructors or method chaining. Values
come from the game when queried; wrappers do not cache health, position, ownership or other mutable native state.

## 4. Identity, interoperability and lifetime

Every class has `fromHandle(handleOrNil)`: nil returns nil; a live handle returns its existing wrapper or creates one.
Factories return a non-nil wrapper or raise an error if the native returns nil. Calling `fromHandle` transfers no
automatic cleanup responsibility: wrapping an existing object never schedules its destruction.

Use a separate strong cache for each class, keyed by the native handle itself, not a numeric handle id. This keeps
wrapper identity stable even when user references disappear. Entries for disposable objects live until explicit
cleanup. Document this cost: code that wraps short-lived objects must arrange their cleanup through the wrappers.
Player wrappers remain cached for the session. Garbage collection never calls Warcraft natives.

Each instance exposes `handle` as a documented read-only-by-convention field. It is the raw handle while active and
nil after disposal; its annotation is consequently nullable. `getHandle()` returns the non-null native handle or
raises an error if disposed. `isDisposed()` is safe at any time, including after disposal.

Unit uses `remove()`; Timer, Trigger, Group and Effect use `destroy()`. Player has no cleanup method. Cleanup is
idempotent: mark the wrapper disposed and remove its cache entry before invoking the destruction native, keeping
the raw handle locally for that call. Reentrant calls consequently see disposal immediately. Clear stored callbacks
and the public handle. Other instance methods reject disposed receivers and disposed wrapper arguments before
invoking natives. A native failure during destruction is reported; the wrapper stays disposed.

`Unit.kill()` is not disposal: a dead unit still has a handle and its wrapper remains valid until removal. Destroying
a group does not remove its units; destroying an effect does not remove its attachment target.

Calling RemoveUnit/DestroyTimer/etc. directly bypasses wrapper tracking. Callers must use wrapper cleanup for wrapped
objects. Likewise, never pass a previously destroyed raw handle to `fromHandle`: there is no reliable generic native
liveness check. Numeric handle-id reuse must not revive a disposed wrapper.

## 5. Initial public surface

All six classes have the shared methods in section 4. Below, Player and Unit parameters mean wrappers; Warcraft enum
parameters retain their native annotation types. Coordinates, facing, durations and life are numbers. Rawcodes and
player indices are integers. Defaults are explicit; optional convenience overloads are out of scope.

### Player

- `fromIndex(index) -> Player`: zero-based Warcraft player index; validate an integer in `[0, bj_MAX_PLAYER_SLOTS)`.
- `getId() -> integer`, `getName() -> string`, `getColor() -> playercolor`.
- `getState(state: playerstate) -> integer`, `setState(state: playerstate, value: integer)`.

Do not expose SetPlayerName in this first release: the Moonwell handoff records a lobby crash associated with that
native. This scope decision does not assert that every possible runtime use crashes.

### Unit

- `create(owner: Player, typeId: integer, x, y, facing) -> Unit`.
- `getTypeId() -> integer`, `getOwner() -> Player`, `setOwner(owner: Player, changeColor: boolean)`.
- `getX() -> number`, `getY() -> number`, `setPosition(x, y)` using SetUnitPosition.
- `getFacing() -> number`, `setFacing(facing)`.
- `getLife() -> number`, `setLife(value)` using GetWidgetLife/SetWidgetLife; `getMaxLife() -> integer` using BlzGetUnitMaxHP.
- `setColor(color: playercolor)`, `kill()`, `remove()`.
- `issueOrder(order: string) -> boolean`, `issuePointOrder(order: string, x, y) -> boolean`,
  `issueTargetOrder(order: string, target: Unit) -> boolean`, using the corresponding string-order natives.

Target orders accept units in v0.1.0. Raw natives remain the escape hatch for item/destructable targets. Do not hide
Warcraft's pathing behavior behind setPosition or silently replace it with SetUnitX/SetUnitY.

### Timer

- `create() -> Timer`.
- `start(timeout, periodic: boolean, callback: fun(timer: Timer): ...)`.
- `pause()`, `resume()`, `getElapsed() -> number`, `getRemaining() -> number`, `getTimeout() -> number`, `destroy()`.

`start` replaces the previous schedule. A one-shot timer stays allocated after firing, so it can be restarted or
explicitly destroyed. Destroy pauses before DestroyTimer and invalidates any retained callback. A callback may
restart or destroy its own timer. Each start gets a generation token; old callbacks do nothing after replacement or
disposal. No post-callback cleanup may erase a newer schedule installed by that callback. Pause/resume retain the
current generation. Reject non-finite or negative timeouts.

### Trigger

- `create() -> Trigger`, `enable()`, `disable()`, `isEnabled() -> boolean`, `destroy()`.
- `registerUnitEvent(unit: Unit, event: unitevent)`.
- `registerPlayerUnitEvent(player: Player, event: playerunitevent)`; passes nil as the native filter.
- `registerTimerEvent(timeout, periodic: boolean)`; validates timeouts as Timer.start does.
- `addAction(callback: fun(trigger: Trigger): ...)`.

Each action is registered with TriggerAddAction and runs behind the callback boundary in section 6. Disposal makes
retained action closures no-ops. Registrations and actions return no public native tokens; individual removal,
conditions, boolexpr ownership and higher-level event subscriptions are outside this release. Users can filter inside
an action and read event context through existing natives, e.g. Unit.fromHandle(GetTriggerUnit()).

### Group

- `create() -> Group`, `add(unit: Unit)`, `remove(unit: Unit)`, `contains(unit: Unit) -> boolean`, `clear()`, `destroy()`.
- `enumInRange(x, y, radius)`: clear first, then GroupEnumUnitsInRange with a nil filter; reject non-finite or negative radius.
- `getSize() -> integer` using BlzGroupGetSize.
- `getUnits() -> Unit[]`: a new dense, one-based Lua array using BlzGroupUnitAt over the native zero-based indices,
  skipping nil handles. Preserve native enumeration order without promising a sorted order.

The returned array is a snapshot of membership, not live native state. Editing the group later does not edit the
array. Each element uses the shared Unit cache. Later disposal of a unit invalidates that element normally. A snapshot
avoids native enumeration callback ownership and lets callers use ordinary loops and predicates.

### Effect

- `create(model: string, x, y) -> Effect` using AddSpecialEffect.
- `attach(model: string, target: Unit, attachmentPoint: string) -> Effect` using AddSpecialEffectTarget.
- `setPosition(x, y, z)` using BlzSetSpecialEffectPosition; `setScale(scale)` using BlzSetSpecialEffectScale; `destroy()`.

Native behavior governs attached-effect transforms and destruction animations; the wrapper adds no alternative
coordinate model or lifetime scheduler.

## 6. Errors and callback boundaries

Synchronous misuse raises ordinary Lua errors prefixed `[wrappers]` and naming the class and operation. These are
gameplay errors, not CLI MoonwellError instances. Validate wrapper identity, disposed state, required callbacks,
player indices and the numeric ranges explicitly specified above. Leave other native-domain rules to Warcraft;
annotations are not a promise of exhaustive runtime validation.

Timer and trigger native callbacks use pcall. On failure, print a contextual `[wrappers] Timer callback failed: ...`
or Trigger equivalent. Callback return values are discarded; annotations allow returns because YueScript implicitly
returns its last expression. A callback error does not implicitly destroy the timer, stop later periodic ticks or
disable the trigger. The library promises no traceback or source-line rewriting beyond what the host supplies.
No debug, io, package, filesystem, dynamic require or unavailable Lua library is required.

Callbacks are synchronous and must not yield or use TriggerSleepAction. Scheduling further work uses timers. Print
errors are visible on screen. The maintainer's 2026-09-28 gate also confirmed wrapper callback errors, ticks and cleanup
messages in Warcraft 3.0.0.24268's F12 log.

## 7. Types and module dependencies

Annotate exported classes as `MoonwellWrappers.Player`, `.Unit`, `.Timer`, `.Trigger`, `.Group` and `.Effect`; these
names must not collide with native lowercase handle types such as `unit` or `player`. All exported functions have
parameter and return annotations, including the nil-preserving fromHandle overload and nullable handle field.

Verified during implementation: LuaLS 3.19.1 merges overload return types, so fromHandle has a conservative nullable
editor result even for a known non-null input. Callers narrow it with an `if` or use `assert` when appropriate;
factories and getOwner validate the conversion result and retain non-null return types. Runtime nil behavior is
unchanged. Two line-local diagnostic exceptions cover native nil filters that the generated JASS types cannot express.

Moonwell supplies native declarations and copies these Lua modules into its existing editor view. The new library
does not redefine Warcraft natives or ship duplicate global declarations. LuaLS must verify both direct Lua usage
and the Lua emitted by YueScript; no editor success is assumed solely because annotations were written.

Runtime dependency direction is acyclic: Unit imports Player; Group imports Unit; Effect imports Unit; Trigger may
import Unit and Player; Timer imports only internal helpers. The shared identity helper must not import public
classes. Imports perform no game-object creation or event registration; callers initialize gameplay in on_main or
another appropriate game callback.

## 8. Verification and release gate

1. Test-first Lua tests run under the pinned YueScript embedded runtime, orchestrated by Deno. Native doubles record
   exact arguments, return values and callbacks. Check every method's native mapping, every wrapper conversion,
   null handling, stable identity, cleanup ordering, double cleanup, use after cleanup and absent import-time effects.
2. Test callbacks with captured functions: success, error reporting, repeated ticks, restart, self-destruction,
   replacement, reentrant cleanup and callbacks retained after disposal. Verify failures do not suppress future runs.
3. Test group snapshot behavior, nil enumeration entries, dense indexing and disposal of members after a snapshot.
4. Syntax-check all shipped Lua for Lua 5.3 compatibility. The Yue embedded test VM is Lua 5.4, so passing runtime
   tests alone does not establish Lua 5.3 compatibility. The plan must select a pinned Lua 5.3 checker and document
   its installation without Node.js.
5. Run LuaLS against a Moonwell consumer fixture after check generates native types and the library editor view.
   Positive Lua and compiled Yue examples have no unexpected diagnostics. Negative fixtures deliberately pass a
   Timer where a Unit is required and use invalid method names; assert that those errors are diagnosed. Check inferred
   callback parameter types and fromHandle nil narrowing explicitly.
6. A fresh Moonwell 0.5.0 project consumes the local checkout via path, then passes check and build, normal and minified.
   Verify only imported modules and their dependencies enter the bundle. The library's own tests must cover Lua source
   quality because Moonwell's unknown-global check does not lint library Lua.
7. Maintainer in-game gate on Warcraft III Reforged 3.0.0.24268: create and wrap the same unit, move and recolor it,
   change life, issue an order, enumerate it in a group, attach/destroy an effect, observe a unit event and run a
   periodic timer. Destroy a timer from its own callback; verify it stops. Deliberately fail timer and trigger
   callbacks, verify visible reporting and subsequent invocations, then remove the probe errors. Remove/destroy all
   owned objects and confirm a second cleanup is harmless. Run the packed minified map as well.
8. After the maintainer pushes the repository and first tag, consume v0.1.0 by GitHub configuration in a fresh map;
   verify the recorded commit, successful build, and unchanged lock after deleting the cache and fetching again.

The new repository's CONTRIBUTING.md will state its checks and gate. Release evidence goes in its CHANGELOG.md.
Publication and the in-game gate are distinct from automated test completion. No release is claimed before both pass.

## 9. Implementation boundaries

This spec and its forthcoming plan live in Moonwell's design history. Product code, library tests and library release
documentation belong in the separate repository. Once this spec is approved, write the task-by-task implementation
plan here, including repository bootstrap, tooling verification, TDD tasks, reviews and the manual gate. Use main and
leave pushing to the maintainer as instructed by AGENTS.md. Do not publish the library as a JSR or Pkl package.

Out of scope: broad handle coverage, w3ts compatibility guarantees, code generation from every native, properties
implemented with metamethod interception, implicit cleanup, resource pooling, ownership graphs, custom subclasses,
multiplayer synchronization helpers, schedulers and the wc3-lib port. Wrapper calls inherit native synchronization
requirements; wrapping a native does not make a local-player-only mutation synchronized.
