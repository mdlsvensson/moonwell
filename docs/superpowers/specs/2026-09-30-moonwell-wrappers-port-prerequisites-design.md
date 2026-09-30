# Moonwell Wrappers Port Prerequisites (wrappers v0.7.0) — Design

- **Date:** 2026-09-30
- **Status:** The design was approved in chat on 2026-09-30; this written spec awaits review.
- **Builds on:** the wrapper specs for v0.1.0 to v0.6.0, the last being
  `2026-09-30-moonwell-wrappers-refactor-design.md`. Everything in them still applies unless this spec changes it
  explicitly: the error-location rule (errors point at the caller, no raising tail calls), the one-lookup prologue, the
  callback boundary and the opt-in rule (nothing is created at import).
- **Inputs:**
  - the port-needs note, `docs/superpowers/research/2026-09-30-wc3-lib-port-needs.md` (§2 the missing natives, §4 D1,
    §6.4 the sync probe);
  - decision D1 in the roadmap (option (c), a mix; damage event data goes in the wrappers);
  - `wc3-lib`'s adapters `damage/warcraft.ts` and `persistence/sync.ts`.
- **Target:** wrappers `v0.7.0` in `mdlsvensson/moonwell-wrappers`. It still requires Moonwell 0.5.1 or later; every
  native it uses is in Moonwell's `natives.json`. The Moonwell CLI does not change.

## 1. Intent and scope

Roadmap item 2.3: what D1 assigned to the wrappers, so the `wc3-lib` port (phase 3) can build on wrappers instead of raw
natives in those places. Four additions, and nothing else (the maintainer chose "D1 only"):

1. `unit:getCollisionSize()`;
2. `unit:setPathing(flag)`;
3. a `wrappers.damage` module: listeners for the DAMAGING and DAMAGED phases, with the event's data and setters;
4. a `wrappers.sync` module: sending checked messages and listening per prefix.

Every change is an addition. No existing signature, message or behavior changes, so the CHANGELOG has no migration
lines.

## 2. Unit

```lua
---@return number
function Unit:getCollisionSize()   -- BlzGetUnitCollisionSize
---@param flag boolean
function Unit:setPathing(flag)     -- SetUnitPathing
```

Both use the one-lookup prologue (`registry.require(self, 'Unit.<name>')`) and wrap their return in parentheses where
they return a native's result. `setPathing` passes `flag` through, like `pause` and `setInvulnerable`. The README notes
that `setPathing(false)` lets the unit walk through units, trees and cliffs until it is set back, which is what dummies
need.

## 3. `wrappers.damage`

### 3.1 API

```lua
local Damage = require 'wrappers.damage'

---@param callback fun(event: MoonwellWrappers.DamagingEvent): ...
---@return MoonwellWrappers.DamageListener
function Damage.onDamaging(callback)

---@param callback fun(event: MoonwellWrappers.DamagedEvent): ...
---@return MoonwellWrappers.DamageListener
function Damage.onDamaged(callback)

---Removes a listener at once, even during a firing. Removing it twice does nothing.
---@param token MoonwellWrappers.DamageListener
function Damage.off(token)
```

`Damage.onDamaging` and `Damage.onDamaged` raise "expected a callback function" for a non-function (through
`Callback.check`). `Damage.off` raises "expected DamageListener token" for anything that is not a token it returned.

### 3.2 Triggers

Each phase has at most one trigger, created by its first listener:
- it is registered with `TriggerRegisterPlayerUnitEvent` for every slot `0 .. bj_MAX_PLAYER_SLOTS - 1`, with
  `EVENT_PLAYER_UNIT_DAMAGING` or `EVENT_PLAYER_UNIT_DAMAGED` and a nil filter, as `Trigger:registerAnyUnitEvent` does;
- its one action routes the event to the phase's listeners (§3.4);
- it is disabled (`DisableTrigger`) when the phase's last listener is removed, and enabled again when a listener is
  added. It is never destroyed, so a listener that removes itself never destroys the trigger that is running it.

Importing the module creates nothing.

### 3.3 The event

Each firing builds one event table and gives it to every listener of that firing. Its fields are read once, when the
firing starts:

| Field | Type | Native |
| --- | --- | --- |
| `source` | `MoonwellWrappers.Unit?` | `GetEventDamageSource`, through `Unit.fromHandle`; nil when the game gives none |
| `target` | `MoonwellWrappers.Unit` | `BlzGetEventDamageTarget`, through `Unit.fromHandle` |
| `amount` | `number` | `GetEventDamage` |
| `isAttack` | `boolean` | `BlzGetEventIsAttack` |
| `attackType` | `attacktype` | `BlzGetEventAttackType` |
| `damageType` | `damagetype` | `BlzGetEventDamageType` |
| `weaponType` | `weapontype` | `BlzGetEventWeaponType` |

Two classes share these fields:
- **`MoonwellWrappers.DamagingEvent`** (before armor) has `setAmount`, `setAttackType`, `setDamageType` and
  `setWeaponType`.
- **`MoonwellWrappers.DamagedEvent`** (after armor) has only `setAmount`.

`wc3-lib` documents that type changes after armor do nothing; the split lets LuaLS flag them, and the gate confirms the
claim (§6).

Each setter calls its native (`BlzSetEventDamage`, `BlzSetEventAttackType`, `BlzSetEventDamageType`,
`BlzSetEventWeaponType`) and updates the matching field, so later listeners of the same firing see the change.

Setter checks, all raising at the caller's line:
- **The event is over:** after the firing's last listener has run, every setter raises
  "`DamagingEvent.setAmount`: the damage event is over" (with the class and method of the call). This catches a setter
  called from a timer or a stored event.
- **`setAmount`:** the amount must be a finite number ("expected a finite number"). Negative amounts are passed to the native
  unchanged; what the game does with them is not specified here.
- **Type setters:** the value must not be nil ("expected an attack type", "a damage type", "a weapon type").

### 3.4 Routing

The same rules as `Frame:on`:
- listeners run in the order they were added;
- a listener added during a firing waits for the next firing; one removed during a firing is skipped at once;
- each runs behind the callback boundary, labelled `Damage listener`, so an error prints and the next listener still
  runs.

### 3.5 Nested damage

A listener may deal damage (`unit:damageTarget`), which fires the phases again inside the outer firing. Each firing has
its own event; the outer event stays live until its own firing ends. Whether the game's event responses (and so
`BlzSetEventDamage`) still refer to the outer hit after a nested one returns is **not known**. The gate measures it
(§6, step 3). If the outer hit's setters do not take effect after a nested hit, the README documents it as a native
caveat; the wrappers do not try to work around it in this release.

## 4. `wrappers.sync`

### 4.1 API

```lua
local Sync = require 'wrappers.sync'

---Sends `data` to every player under `prefix`. Call it for the local player only (inside a local-player branch); the
---listeners run on every machine.
---@param prefix string
---@param data string At most 255 bytes.
---@return boolean sent What BlzSendSyncData returned.
function Sync.send(prefix, data)

---@param prefix string
---@param callback fun(player: MoonwellWrappers.Player, data: string): ...
---@return MoonwellWrappers.SyncListener
function Sync.on(prefix, callback)

---Removes a listener at once, even during a firing. Removing it twice does nothing.
---@param token MoonwellWrappers.SyncListener
function Sync.off(token)
```

### 4.2 Checks

- **`prefix`** (both `send` and `on`): a non-empty string, else "expected a non-empty prefix string". No length limit
  unless the gate finds one (§6, step 6); if it does, the limit and its message are added before release.
- **`data`**: a string of at most 255 bytes (`#data`), else "expected a string" or "data is N bytes, over the 255-byte
  limit". The 1.4 probe showed that the game cuts a 256-character message to 255 characters while `BlzSendSyncData`
  still returns true; the check makes that loud. The wrappers do not split long data: the port chunks its own.

### 4.3 Triggers

Each prefix has at most one trigger, created by its first listener and registered with
`BlzTriggerRegisterPlayerSyncEvent(trigger, Player(i), prefix, false)` for every player `0 .. bj_MAX_PLAYERS - 1`. Its
action reads `GetTriggerPlayer()` and `BlzGetTriggerSyncData()` once and calls each listener with the Player wrapper
and the data. Enabling, disabling, ordering and the callback boundary (labelled `Sync listener`) follow §3.2 and §3.4.

The README says: `send` belongs inside a local-player branch; listeners run on every machine in the same order; the
message arrives some frames later (about 0.09 s in the probe), not during `send`.

## 5. Documentation

- README: the two Unit methods in the Unit section; new API sections for `wrappers.damage` and `wrappers.sync` with a
  short example each; the nested-damage and prefix results from the gate.
- CHANGELOG `0.7.0`: the additions, the gate record, no migrations.
- CONTRIBUTING: the new gate run.
- Moonwell's `AGENTS.md` and roadmap: 2.3 released, next is phase 3 (the port's spec).

## 6. Verification and release gate

Automated checks, as in CONTRIBUTING:
- `deno task test`: behavior tests with native doubles for every rule in §2 to §4, including:
  - the shared event (a second listener sees the first's `setAmount`), order, a listener added during a firing waiting,
    `off` during a firing, an erroring listener not stopping the next;
  - the stale event raising from a setter after the firing;
  - the phase trigger created once, disabled when empty and enabled again;
  - 255 bytes accepted and 256 rejected, a non-string and an empty prefix rejected, one trigger per prefix;
  - the `blame` sweep extended to the new functions and event methods;
- `check:lua` with Lua 5.3.6;
- `test:integration`: Moonwell builds, the native-call check, and LuaLS negatives including a type setter on a
  `DamagedEvent`.

In-game gate (the maintainer, 3.0.0.24268), one new run `deno task gate port`, normal build only, backed by a `port`
flag in `examples/gate.yue`:
1. **Damage before armor:** a footman attacks a target; a DAMAGING listener doubles the amount. The log shows source,
   target, `isAttack` and the amounts before and after, and the target's life loss matches the doubled amount.
2. **Damage after armor:** a DAMAGED listener sets the damage type; the log shows it had no effect on the hit.
3. **Nested damage:** a DAMAGING listener deals a small nested hit with `damageTarget`, then sets the outer amount; the
   log shows whether the outer change took effect (§3.5).
4. **Stale event:** a setter called from a zero-second timer raises; the gate prints the message in a `pcall`.
5. **Unit:** `getCollisionSize()` of the footman is printed, and a unit with `setPathing(false)` is ordered through a
   line of trees and arrives on the far side.
6. **Sync:** a 255-byte message round-trips intact; `send` rejects a 256-byte one; prefixes of 16, 17 and 32 characters
   are sent and the log shows which arrived intact.

The existing runs are not re-run: this release adds modules and two methods and changes no existing code path. Tag
consumption as for every release. The online and multiplayer checks (sync between two machines) stay deferred to
before Moonwell 1.0.

## 7. Plan phasing

One plan, `docs/superpowers/plans/2026-09-30-moonwell-wrappers-port-prerequisites.md`, of test-first tasks:

1. The Unit methods.
2. `wrappers.damage`: listeners, triggers and routing.
3. `wrappers.damage`: the event, its setters and the stale check.
4. `wrappers.sync`.
5. The blame sweep, LuaLS negatives and integration.
6. Docs.
7. The gate additions and the gate map's `port` run.
8. Release.

## 8. Out of scope

- **Sync chunking and sessions:** the port's persistence layer.
- **A damage system** (phases beyond the game's two, metadata, lethal checks): the port's damage module.
- **Event unit helpers** (`Unit.fromEvent()`), **timer-expiry registration** and **automatic Unit disposal:** phase 4.1
  and the backlog, as chosen.
- **A Trigger registration for sync** (`trigger:registerSyncEvent`): `Sync.on` covers the need.
