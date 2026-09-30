# Moonwell Systems Release 2 (v0.2.0): Dummies, Buffs and Auras — Design

- **Date:** 2026-09-30
- **Status:** The design was approved in chat on 2026-09-30; this written spec awaits review.
- **Builds on:** Part 1 of `2026-09-30-moonwell-systems-design.md`, which binds this release (rules §4, tooling §5,
  releases §6), and release 1 (`systems.scheduler`, `signal`, `scope`, `time`, `internal.callback`, `internal.check`).
- **Inputs:** `wc3-lib`'s `dummy/dummy.ts`, `dummy/warcraft-dummy.ts`, `buffs/buffs.ts`, `buffs/aura.ts`,
  `buffs/warcraft-buffs.ts` and their 13 tests (`tests/buffs.test.ts`, `tests/dummy.test.ts`).
- **Target:** moonwell-systems `v0.2.0`. It still needs moonwell-wrappers `v0.7.0` and Moonwell 0.5.2.

## 1. Scope and the maintainer's choices

Four modules: `systems.internal.ordered`, `systems.buffs`, `systems.aura` and `systems.dummy`. The maintainer chose
(2026-09-30):
- **Buff targets are Units only** (Unit wrappers), so the store can prune them itself and editor types are exact.
- **Pruning is automatic:** a store polls its units from the scheduler it is given.
- **The README gives a Pkl definition of the dummy unit type**, and the gate map uses exactly that definition.

The API style follows release 1: `new` constructors, getters, cancel functions, idempotent `dispose()`, failures
reported through an optional `onError` or printed (never rethrown), and errors raised at the caller's line.

## 2. `systems.internal.ordered`

An insertion-ordered map for keys that are handles or wrappers (Part 1 §4.4).

```lua
function Ordered.new()
function Ordered:set(key, value)   -- a new key goes last; an existing key keeps its place
function Ordered:get(key)
function Ordered:has(key)
function Ordered:delete(key)       -- returns whether the key was present
function Ordered:getSize()
function Ordered:keys()            -- a new array of the keys in insertion order
function Ordered:each(fn)          -- fn(key, value) for the entries present at the start that are still present
```

- `each` is safe against changes made by `fn`: deleted entries are skipped, and entries added during `each` wait for
  the next iteration. Nested `each` calls are allowed.
- Deleting keeps O(1) cost; the key array is compacted when no `each` is running.
- It is internal: its errors are programming errors and it raises plain Lua errors, not `[systems]` messages.

## 3. `systems.buffs`

### 3.1 API

```lua
---@param clock MoonwellSystems.Scheduler
---@param options {onError: fun(message: string)?, pollInterval: number?}?  -- pollInterval default 0.25
function BuffStore.new(clock, options)
function BuffStore:apply(unit, definition, source)    -- returns MoonwellSystems.Buff; source is any value, default nil
function BuffStore:get(unit, id, source)               -- with source nil, the first with this id from any source
function BuffStore:has(unit, id, source)
function BuffStore:stacks(unit, id)                    -- summed over every source
function BuffStore:list(unit)                          -- a new array in application order
function BuffStore:clearUnit(unit, reason)             -- reason default 'removed'; 'death' keeps survivors
function BuffStore:clearSource(source)                 -- reason 'source-lost'
function BuffStore:dispose()                           -- reason 'disposed'; stops the poll

function Buff:getUnit()  function Buff:getSource()  function Buff:getId()  function Buff:getDefinition()
function Buff:isActive()  function Buff:getStacks()  function Buff:getRemaining()  -- seconds, or nil if permanent
function Buff:own(release)                             -- runs at once if the buff has ended
function Buff:remove(reason)                           -- reason default 'dispelled'
Buff.data                                              -- a table for the buff's own state
```

A definition is a plain table:

| Field | Meaning |
| --- | --- |
| `id` | non-empty string; one instance per (unit, id, source) |
| `kind` | `'active'`, `'passive'` or `'aura'` |
| `stacking` | `'refresh'` (default), `'replace'`, `'stack'` or `'independent'` |
| `maxStacks` | positive integer, default 1 |
| `duration` | positive seconds; nil for a permanent buff |
| `removeOnDeath` | default true, except `'passive'` buffs, which survive death |
| `interval` | positive seconds between `onTick` calls; the first one interval after applying |
| `onApply(buff)`, `onStacks(buff, previous)`, `onTick(buff)`, `onRemove(buff, reason)` | optional callbacks |

Removal reasons: `'expired'`, `'dispelled'`, `'replaced'`, `'death'`, `'removed'`, `'source-lost'`, `'disposed'`,
`'error'`.

### 3.2 Behavior (from `wc3-lib`)

- **Stacking:** refresh keeps one stack and restarts the duration; replace removes the old instance (`'replaced'`) and
  applies a new one; stack adds a stack up to `maxStacks` and restarts the shared duration; independent gives each
  stack its own duration, and a capped application extends nothing.
- **Order on apply:** ticking starts before the expiry timer, so on a shared deadline the tick runs first (duration 3,
  interval 1 ticks three times).
- **A different definition table** for an existing (unit, id, source) raises
  `[systems] BuffStore.apply: use the same definition for a given id and source`.
- **Ending a buff:** its timers are cancelled, its `own` releases run in reverse, then `onRemove`, each exactly once.
- **Replacement hooks:** if `onRemove` of a replaced buff applies the same key again, `apply` returns that instance.
- **Index cleanup:** a unit's entry is dropped when its last buff ends, so the store keeps no removed units.

### 3.3 Failures

- A failing `onApply`, `onStacks` or `onTick` removes that buff with reason `'error'` and is reported (label
  `Buff callback`); it is not rethrown.
- A failing release or `onRemove` is reported (label `Buff release`) and the others still run.
- Every report goes to `options.onError` or is printed, as in release 1.

### 3.4 Pruning

`BuffStore.new` schedules `every(pollInterval, …)` on `clock`. Each poll visits the buffed units in first-buffed order:
- a disposed wrapper (`unit:isDisposed()`), or `unit:exists()` false: `clearUnit(unit, 'removed')`;
- otherwise `not unit:isAlive()`: `clearUnit(unit, 'death')`.

The disposed check comes first because `exists()` raises for a disposed wrapper. `dispose()` cancels the poll.

### 3.5 Checks

`apply` raises at the caller for: a non-Unit target (`expected a Unit`); a definition that is not a table, an empty
`id`, an unknown `kind` or `stacking`, a non-positive or non-finite `duration` or `interval`, a `maxStacks` that is not
a positive integer, or a callback field that is not a function (`expected a buff definition: <field>`); and a disposed
store (`the store is disposed`).

## 4. `systems.aura`

```lua
function Aura.new(store, definition, source, query) -- definition.kind must be 'aura'; query() returns Units
function Aura:start(interval)                        -- default 0.5; updates now, then on the store's scheduler
function Aura:update()
function Aura:dispose()                              -- stops and removes every buff it applied ('source-lost')
```

- `update` removes the buff from units `query` no longer returns (`'source-lost'`) and applies it to new ones.
- Each emitter (`source`) owns its instances, so two auras with the same definition never remove each other's buffs.
- Members keep the order `query` returned them in. That should be the engine's enumeration order (for example
  `group:getUnits()`), which is synchronized. `wc3-lib` asked for a sort by handle id; Part 1 §4.4 forbids that.
- A buff the store ended (a dispel, a death) is applied again on the next update while the unit is still returned.
- `start` twice restarts the timer; `start` after `dispose` raises.

## 5. `systems.dummy`

```lua
function Dummies.new(clock, options)      -- options: {onError: fun(message: string)?}?
function Dummies:cast(request)            -- returns MoonwellSystems.DummyLease
function Dummies:isDummy(unit)
function Dummies:sourceOf(unit)           -- the caster a live dummy acts for, or nil
function Dummies:getCount()
function Dummies:dispose()                -- removes every live dummy; casting afterwards raises

function DummyLease:getUnit()  function DummyLease:getSource()  function DummyLease:isActive()
function DummyLease:isOrderAccepted()  function DummyLease:dispose()
```

The request is a table:

| Field | Meaning |
| --- | --- |
| `owner` | the Player that owns the dummy |
| `typeId` | the dummy unit type (the map's object data, §5.2) |
| `x`, `y`, `facing` | spawn position; facing default 0 |
| `ability`, `level` | the ability to add and its level, default 1 |
| `order` | an order string, or an order id (integer) |
| `target` or `point` | a widget (Unit, Item, Destructable), or `{x, y}`; neither means an immediate order |
| `duration` | positive seconds; must cover cast point, channel time and projectile travel |
| `source` | the real caster Unit, for `sourceOf` |

### 5.1 Behavior

- `cast` creates a fresh unit (no pooling), adds Locust (`'Aloc'`), sets it invulnerable and `setPathing(false)`, adds
  the ability and sets its level, fills its mana, issues the order and schedules its removal after `duration`.
- A rejected order removes the dummy at once; the lease reports `isOrderAccepted() == false`.
- If the unit cannot get the ability (`addAbility` false and level 0), the dummy is removed and `cast` raises
  `the dummy cannot get ability <id>` at the caller.
- Leases live in an ordered map keyed by the dummy Unit, so `isDummy` and `sourceOf` cost one lookup and `dispose`
  removes dummies in cast order.
- If something else removed the dummy first, the lease's removal skips `unit:remove()` (the wrapper is disposed).
- Checks at the caller: `owner` a Player, `typeId` and `ability` integers, `order` a non-empty string or an integer,
  finite `x`, `y`, `facing`, a positive integer `level`, a positive finite `duration`, not both `target` and `point`,
  a `point` with finite `x` and `y`, `source` nil or a Unit.

### 5.2 The dummy unit type

The README gives this `objects/` definition (field names from Moonwell's generated `UnitProps.pkl`), and the gate map
uses it unchanged:

```pkl
units {
  ["dummy"] {
    id = "e000"                  // pick a free rawcode
    base = "ewsp"                // Wisp: no attack, no food
    name = "Dummy"
    modelFile = ".mdl"           // no model is drawn
    shadowImageUnit = ""
    normal = List("Aloc")        // Locust: unselectable and untargetable
    animationCastPoint = 0
    animationCastBackswing = 0
    collisionSize = 0
    manaMaximum = 10000
    foodCost = 0
    type = "fly"
  }
}
```

The plan's gate task confirms every field with `objects:check` and in game (the dummy casts, is invisible and blocks
nothing); a field that Moonwell's schema or the game rejects is corrected in both the README and this spec before
release.

## 6. Verification

Automated (Part 1 §5):
- suites `ordered`, `buffs`, `aura`, `dummy`, covering `wc3-lib`'s 13 tests adapted to this API (Units as targets,
  reported instead of rethrown failures), and the new rules: pruning of a disposed wrapper, a removed unit and a dead
  unit (with a surviving passive buff); `each` during deletion and insertion; order ids; the ability check; early
  removal of a dummy;
- the `blame` sweep over the new public classes, and `imports`;
- integration: one-module bundles for `systems.buffs`, `systems.aura`, `systems.dummy` (for example, buffs alone bundles
  no `systems.dummy`), LuaLS fixtures for the new API.

In-game gate (the maintainer, 3.0.0.24268): the `systems` run gets a release 2 part after release 1's checks:
1. **Dummy:** a dummy casts Storm Bolt (`AHtb`, order `thunderbolt`) at a hostile footman with the hero as source; the
   footman is stunned and damaged; the log prints `isDummy true`, `sourceOf` the hero, the count 1 while leased and 0
   after the duration; no dummy model is visible.
2. **Buff:** a `stack` buff (max 3) tints a footman and slows it, restoring both on removal; it prints each tick and
   `expired` at the end, and the footman's colour and speed come back.
3. **Pruning:** a buffed footman is killed and prints `death` within 0.25 s; another is removed with raw `RemoveUnit`
   and prints `removed`; a `passive` buff on the killed one prints nothing at death.
4. **Aura:** an aura on the hero with a range query; a footman moved into range gains the buff, and moved out loses it
   (`source-lost`).

Then tag `v0.2.0` and check tag consumption.

## 7. Out of scope

Native buff icons and ability-based buffs (separate adapters, later); dummy pooling; the damage pipeline (release 3),
which will use `Dummies:sourceOf`.
