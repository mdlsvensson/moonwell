# Moonwell Wrappers Port Prerequisites (v0.7.0) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wrappers v0.7.0: `unit:getCollisionSize()`, `unit:setPathing(flag)`, a `wrappers.damage` module (DAMAGING
and DAMAGED listeners with the event's data and setters) and a `wrappers.sync` module (checked sends, listeners per
prefix), so the `wc3-lib` port can use wrappers instead of raw natives there.

**Architecture:** A new internal module, `internal/listeners.lua`, holds what damage and sync share: one trigger per key
(a damage phase or a sync prefix), created by the first listener, disabled while the key has no listeners and never
destroyed; listener tokens; and routing with `Frame:on`'s rules. `damage.lua` builds one event table per firing, with
two metatables (DAMAGING has type setters, DAMAGED only `setAmount`) and a weak "live" set that makes setters raise once
the firing is over. `sync.lua` checks the prefix and a 255-byte limit before `BlzSendSyncData`.

**Tech Stack:**
- annotated Lua 5.3 (tests run under `yue -e`, Lua 5.4);
- Deno tools, `jsr:@std/*` only;
- LuaLS 3.19.1 and YueScript 0.34.2;
- Moonwell at `../moonwell` (0.5.2).

**Spec:** `docs/superpowers/specs/2026-09-30-moonwell-wrappers-port-prerequisites-design.md`

## Global Constraints

- **Repository and commits:** code is in `C:\Users\mdlsvensson\Repo\moonwell-wrappers`; Moonwell records in
  `C:\Users\mdlsvensson\Repo\moonwell`. Commit on `main`, staging explicit paths only (never `git add -A`). End every
  commit message with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Commit only when the task's checks pass.
- **Tooling:** no Node.js, no npm packages, no `node:` or `npm:` specifiers.
- **Additions only:** no existing public signature, error message or behavior changes.
- **Error locations:** every error raised by the new code points at the line that called the public function or method.
  Levels count from the function that calls `error`: 2 inside a public function, 3 in a helper called by a public
  function, `3 + depth` in `Handle.created`/`Handle.unwrap` when a helper sits in between. Never tail-call a raising
  helper from a public function: write `return (helper(...))`. A tail call to a native is fine.
- **Message texts** (exact): `'[wrappers] ' .. operation .. ': expected a callback function'` (existing
  `Callback.check`), `expected DamageListener token`, `expected SyncListener token`, `the damage event is over`,
  `expected a finite number`, `expected an attack type`, `expected a damage type`, `expected a weapon type`,
  `expected a non-empty prefix string`, `expected a string`, `data is <N> bytes, over the 255-byte limit`.
- **Callback labels:** `Damage listener` and `Sync listener` (printed as `[wrappers] <label> callback failed: …`).
- **Import rule:** importing a module calls no native and creates nothing.
- **Test closures:** a closure passed to `failsAt` calls the wrapper as a statement, never `return wrapper(...)`.
- **YueScript in the gate:** no `//`; no bare `nil` statement; a local named like an outer local reuses it (pick new
  names); a trailing loop as a function's last statement becomes a table expression; `close` is a keyword.
- **Files with backslashes** (YueScript `\` method calls inside TypeScript strings) are edited with the file-editing
  tool, never shell heredocs or `sed`.
- **Checks** (all from the wrappers repository): `deno task test`, `deno task check:lua`, `deno task check`,
  `deno task lint`, `deno fmt --check`, `deno task test:integration` (with `MOONWELL_LUALS` and `MOONWELL_LUAC` set as
  in CONTRIBUTING).

---

### Task 1: Unit collision size and pathing

**Files:**
- Modify: `src/wrappers/unit.lua` (after `Unit:isHidden`, around line 105)
- Test: `tests/unit.lua` (the `state and presentation methods` test, around line 206)

**Interfaces:**
- Produces: `Unit:getCollisionSize(): number`, `Unit:setPathing(flag: boolean)`.

- [ ] **Step 1: Write the failing test**

In `tests/unit.lua`, test `state and presentation methods`, add a row to each table:

```lua
    checkGetters(u, {{'BlzGetUnitMaxMana', 'getMaxMana', 100}, {'GetUnitMoveSpeed', 'getMoveSpeed', 270},
        {'IsUnitPaused', 'isPaused', false}, {'BlzIsUnitInvulnerable', 'isInvulnerable', true},
        {'IsUnitHidden', 'isHidden', false}, {'GetUnitName', 'getName', 'Footman'},
        {'GetUnitCurrentOrder', 'getCurrentOrder', 851983}, {'IsUnitType', 'isType', true, kind},
        {'BlzGetUnitCollisionSize', 'getCollisionSize', 16}})
    checkSetters(u, {{'BlzSetUnitMaxMana', 'setMaxMana', 150}, {'BlzSetUnitMaxHP', 'setMaxLife', 500},
        {'SetUnitMoveSpeed', 'setMoveSpeed', 300}, {'SetUnitX', 'setX', 5}, {'SetUnitY', 'setY', 6},
        {'SetUnitVertexColor', 'setVertexColor', 255, 128, 0, 200}, {'SetUnitAnimation', 'setAnimation', 'attack'},
        {'PauseUnit', 'pause', true}, {'SetUnitInvulnerable', 'setInvulnerable', false}, {'ShowUnit', 'show', false},
        {'UnitApplyTimedLife', 'applyTimedLife', 1112045413, 5}, {'SetUnitPathing', 'setPathing', false}})
```

- [ ] **Step 2: Run it to see it fail**

Run: `deno task test unit`
Expected: `FAIL state and presentation methods: … attempt to call a nil value (field '?')` (the method does not exist).

- [ ] **Step 3: Implement**

In `src/wrappers/unit.lua`, after the `Unit:isHidden` function:

```lua
---The unit's collision radius, as the game uses it for pathing.
---@return number
function Unit:getCollisionSize() return BlzGetUnitCollisionSize(registry.require(self, 'Unit.getCollisionSize')) end
---With false the unit ignores pathing: it walks through units, trees and cliffs until set back to true.
---@param flag boolean
function Unit:setPathing(flag) SetUnitPathing(registry.require(self, 'Unit.setPathing'), flag) end
```

- [ ] **Step 4: Run the tests**

Run: `deno task test unit blame`
Expected: both suites print `SUITE PASSED`.

- [ ] **Step 5: Commit**

```bash
git add src/wrappers/unit.lua tests/unit.lua
git commit -m "feat: unit:getCollisionSize() and unit:setPathing(flag)"
```

---

### Task 2: Shared listeners and the damage listeners

**Files:**
- Create: `src/wrappers/internal/listeners.lua`
- Create: `src/wrappers/damage.lua` (listeners, triggers, routing and the event's fields; setters come in Task 3)
- Test: `tests/damage.lua`

**Interfaces:**
- Consumes: `Handle.created(raw, operation, depth)` (`internal/handle.lua`), `Callback.check(value, operation)` and
  `Callback.call(label, fn, ...)` (`internal/callback.lua`), `Unit.fromHandle(raw)`.
- Produces:
  - `Listeners.new(kind: string, register: fun(trigger, key), route: fun(key, cells)): MoonwellWrappers.ListenerSet`;
  - `Listeners.add(set, key, callback, operation): table` (token; call it as `return (Listeners.add(...))`);
  - `Listeners.remove(set, token, operation)` (raises at level 3: call it as a statement from the public function);
  - `Listeners.call(cells, label, ...)`;
  - `Damage.onDamaging(callback)`, `Damage.onDamaged(callback)` returning `MoonwellWrappers.DamageListener`,
    `Damage.off(token)`;
  - the module-local classes `DamagingEvent` and `DamagedEvent` (Task 3 adds their setters) and the weak set `live`.

- [ ] **Step 1: Write the failing tests**

Create `tests/damage.lua`:

```lua
EVENT_PLAYER_UNIT_DAMAGING, EVENT_PLAYER_UNIT_DAMAGED = {}, {}
ATTACK_NORMAL, ATTACK_CHAOS, DAMAGE_NORMAL, DAMAGE_UNIVERSAL, WEAPON_NONE, WEAPON_METAL = {}, {}, {}, {}, {}, {}
local actions = {}
native('CreateTrigger', function() return {events = {}, enabled = true} end)
native('TriggerRegisterPlayerUnitEvent', function(trigger, _, event) trigger.events[#trigger.events + 1] = event end)
native('TriggerAddAction', function(trigger, callback)
    actions[#actions + 1] = {trigger = trigger, callback = callback}
    return {}
end)
native('EnableTrigger', function(trigger) trigger.enabled = true end)
native('DisableTrigger', function(trigger) trigger.enabled = false end)
local hit = {}
native('GetEventDamageSource', function() return hit.source end)
native('BlzGetEventDamageTarget', function() return hit.target end)
native('GetEventDamage', function() return hit.amount end)
native('BlzGetEventIsAttack', function() return hit.isAttack end)
native('BlzGetEventAttackType', function() return hit.attackType end)
native('BlzGetEventDamageType', function() return hit.damageType end)
native('BlzGetEventWeaponType', function() return hit.weaponType end)
native('BlzSetEventDamage', function(amount) hit.amount = amount end)
native('BlzSetEventAttackType', function(value) hit.attackType = value end)
native('BlzSetEventDamageType', function(value) hit.damageType = value end)
native('BlzSetEventWeaponType', function(value) hit.weaponType = value end)
local Damage = require('wrappers.damage')
local Unit = require('wrappers.unit')
eq(totalCalls(), 0)

local SOURCE, TARGET = {}, {}
-- Simulates one hit: sets the event data, then runs the action of every enabled trigger registered for `event`.
-- `options.amount` replaces the default 10; `options.noSource` makes the game give no source. Returns the hit, whose
-- fields the setter doubles change.
local function fire(event, options)
    options = options or {}
    hit = {source = SOURCE, target = TARGET, amount = options.amount or 10, isAttack = true,
        attackType = ATTACK_NORMAL, damageType = DAMAGE_NORMAL, weaponType = WEAPON_NONE}
    if options.noSource then hit.source = nil end
    local current = hit
    for _, action in ipairs(actions) do
        if action.trigger.enabled and action.trigger.events[1] == event then action.callback() end
    end
    return current
end

test('the first listener of a phase creates its trigger; an empty phase is disabled, never destroyed', function()
    local first = Damage.onDamaging(function() end)
    eq(callCount('CreateTrigger'), 1); eq(callCount('TriggerAddAction'), 1)
    eq(callCount('TriggerRegisterPlayerUnitEvent'), 28)
    expectCall('TriggerRegisterPlayerUnitEvent', actions[1].trigger, PLAYER_RAW, EVENT_PLAYER_UNIT_DAMAGING, nil)
    local second = Damage.onDamaging(function() end)
    eq(callCount('CreateTrigger'), 1)
    local after = Damage.onDamaged(function() end)
    eq(callCount('CreateTrigger'), 2)
    expectCall('TriggerRegisterPlayerUnitEvent', actions[2].trigger, PLAYER_RAW, EVENT_PLAYER_UNIT_DAMAGED, nil)
    Damage.off(first); eq(callCount('DisableTrigger'), 0)
    Damage.off(second); eq(callCount('DisableTrigger'), 1); eq(actions[1].trigger.enabled, false)
    Damage.off(after); eq(actions[2].trigger.enabled, false)
    local again = Damage.onDamaging(function() end)
    eq(callCount('CreateTrigger'), 2); eq(callCount('EnableTrigger'), 1); eq(actions[1].trigger.enabled, true)
    Damage.off(again)
    eq(callCount('DestroyTrigger'), 0)
end)

test('listeners share one event with the hit data, in the order added', function()
    local seen = {}
    local a = Damage.onDamaging(function(event) seen[#seen + 1] = {'a', event} end)
    local b = Damage.onDamaging(function(event) seen[#seen + 1] = {'b', event} end)
    local other = Damage.onDamaged(function() seen[#seen + 1] = {'damaged'} end)
    fire(EVENT_PLAYER_UNIT_DAMAGING)
    eq(#seen, 2); eq(seen[1][1], 'a'); eq(seen[2][1], 'b'); eq(seen[1][2], seen[2][2])
    local event = seen[1][2]
    eq(event.source, Unit.fromHandle(SOURCE)); eq(event.target, Unit.fromHandle(TARGET))
    eq(event.amount, 10); eq(event.isAttack, true)
    eq(event.attackType, ATTACK_NORMAL); eq(event.damageType, DAMAGE_NORMAL); eq(event.weaponType, WEAPON_NONE)
    fire(EVENT_PLAYER_UNIT_DAMAGED)
    eq(#seen, 3); eq(seen[3][1], 'damaged')
    fire(EVENT_PLAYER_UNIT_DAMAGING)
    eq(seen[4][2] ~= event, true)
    eq(#PRINTED, 0)
    Damage.off(a); Damage.off(b); Damage.off(other)
end)

test('a hit with no source gives a nil source', function()
    local seen
    local token = Damage.onDamaged(function(event) seen = event end)
    fire(EVENT_PLAYER_UNIT_DAMAGED, {noSource = true})
    eq(seen.source, nil); eq(seen.target, Unit.fromHandle(TARGET))
    Damage.off(token)
end)

test('a listener added during a firing waits; one removed during a firing is skipped', function()
    local log, late, second = {}, nil, nil
    local first = Damage.onDamaging(function()
        log[#log + 1] = 'first'
        if not late then late = Damage.onDamaging(function() log[#log + 1] = 'late' end) end
        Damage.off(second)
    end)
    second = Damage.onDamaging(function() log[#log + 1] = 'second' end)
    fire(EVENT_PLAYER_UNIT_DAMAGING)
    eq(table.concat(log, ','), 'first')
    fire(EVENT_PLAYER_UNIT_DAMAGING)
    eq(table.concat(log, ','), 'first,first,late')
    Damage.off(first); Damage.off(late); Damage.off(second)
end)

test('a failing listener is printed and the next one still runs', function()
    local count = 0
    local a = Damage.onDamaging(function() error('intentional damage probe') end)
    local b = Damage.onDamaging(function() count = count + 1 end)
    fire(EVENT_PLAYER_UNIT_DAMAGING)
    eq(count, 1); eq(#PRINTED, 1)
    assert(PRINTED[1]:find('[wrappers] Damage listener callback failed:', 1, true), PRINTED[1])
    assert(PRINTED[1]:find('intentional damage probe', 1, true), PRINTED[1])
    Damage.off(a); Damage.off(b)
end)

test('module functions check their arguments at the caller', function()
    failsAt(function() Damage.onDamaging(nil) end, 'Damage.onDamaging: expected a callback function')
    failsAt(function() Damage.onDamaged('x') end, 'Damage.onDamaged: expected a callback function')
    failsAt(function() Damage.off({}) end, 'Damage.off: expected DamageListener token')
    failsAt(function() Damage.off(nil) end, 'Damage.off: expected DamageListener token')
    local token = Damage.onDamaged(function() end)
    Damage.off(token); Damage.off(token)
    eq(callCount('CreateTrigger'), 0)
end)
```

- [ ] **Step 2: Run it to see it fail**

Run: `deno task test damage`
Expected: the suite fails at load with `module 'wrappers.damage' not found`.

- [ ] **Step 3: Implement the shared listeners**

Create `src/wrappers/internal/listeners.lua`:

```lua
local Handle = require('wrappers.internal.handle')
local Callback = require('wrappers.internal.callback')

---Listener lists shared by wrappers.damage and wrappers.sync. Each key (a damage phase, a sync prefix) has at most one
---trigger, created by its first listener. The trigger is disabled while the key has no listeners and enabled again by
---the next one; it is never destroyed, so a listener that removes itself never destroys the trigger running it.
local Listeners = {}

---@class MoonwellWrappers.ListenerCell
---@field key any
---@field callback function? Nil once removed.

---@class MoonwellWrappers.ListenerSet
---@field kind string The token's class name, for errors.
---@field register fun(trigger: trigger, key: any) Registers a new key's trigger for its events.
---@field route fun(key: any, cells: MoonwellWrappers.ListenerCell[]) The trigger's action.
---@field lists table<any, MoonwellWrappers.ListenerCell[]> Live listeners per key, in the order added.
---@field triggers table<any, trigger>
---@field cells table<table, MoonwellWrappers.ListenerCell> Token to cell.

---@param list MoonwellWrappers.ListenerCell[]
---@param cell MoonwellWrappers.ListenerCell
---@return MoonwellWrappers.ListenerCell[]
local function without(list, cell)
    local result = {}
    for _, value in ipairs(list) do
        if value ~= cell then result[#result + 1] = value end
    end
    return result
end

---@param kind string
---@param register fun(trigger: trigger, key: any)
---@param route fun(key: any, cells: MoonwellWrappers.ListenerCell[])
---@return MoonwellWrappers.ListenerSet
function Listeners.new(kind, register, route)
    return {kind = kind, register = register, route = route, lists = {}, triggers = {},
        cells = setmetatable({}, {__mode = 'k'})}
end

---Adds a listener for `key` and returns its token. The public function must call this as `return (Listeners.add(...))`:
---a failed CreateTrigger raises at the public function's caller.
---@param set MoonwellWrappers.ListenerSet
---@param key any
---@param callback function
---@param operation string
---@return table
function Listeners.add(set, key, callback, operation)
    local trigger, list = set.triggers[key], set.lists[key]
    if not trigger then
        trigger = Handle.created(CreateTrigger(), operation, 1)
        set.register(trigger, key)
        TriggerAddAction(trigger, function() set.route(key, set.lists[key]) end)
        set.triggers[key] = trigger
        list = {}
        set.lists[key] = list
    elseif #list == 0 then
        EnableTrigger(trigger)
    end
    ---@type MoonwellWrappers.ListenerCell
    local cell = {key = key, callback = callback}
    list[#list + 1] = cell
    local token = {}
    set.cells[token] = cell
    return token
end

---Removes a listener at once, even during a firing. Removing it twice does nothing. Call it as a statement from the
---public function: a wrong token raises at that function's caller.
---@param set MoonwellWrappers.ListenerSet
---@param token table
---@param operation string
function Listeners.remove(set, token, operation)
    local cell = token ~= nil and set.cells[token] or nil
    if not cell then error('[wrappers] ' .. operation .. ': expected ' .. set.kind .. ' token', 3) end
    if not cell.callback then return end
    cell.callback = nil
    local list = without(set.lists[cell.key], cell)
    set.lists[cell.key] = list
    if #list == 0 then DisableTrigger(set.triggers[cell.key]) end
end

---Runs the live listeners of one firing behind the callback boundary. Listeners added during the firing wait for the
---next one (the loop bound is read once); removed ones are skipped at once.
---@param cells MoonwellWrappers.ListenerCell[]
---@param label string
---@param ... any Passed to every listener.
function Listeners.call(cells, label, ...)
    for index = 1, #cells do
        local callback = cells[index].callback
        if callback then Callback.call(label, callback, ...) end
    end
end

return Listeners
```

- [ ] **Step 4: Implement the damage listeners**

Create `src/wrappers/damage.lua`:

```lua
local Callback = require('wrappers.internal.callback')
local Listeners = require('wrappers.internal.listeners')
local Unit = require('wrappers.unit')

---Damage events for every unit: listeners before armor (DAMAGING) and after armor (DAMAGED). Each phase has one shared
---trigger, created by its first listener. Nothing is created at import.
local Damage = {}

---@class MoonwellWrappers.DamageListener

---The hit's data, read once when the firing starts. Every listener of one firing gets the same table, so a setter's
---change is visible to the listeners after it.
---@class MoonwellWrappers.DamageEvent
---@field source MoonwellWrappers.Unit? Nil when the game gives no source.
---@field target MoonwellWrappers.Unit
---@field amount number
---@field isAttack boolean
---@field attackType attacktype
---@field damageType damagetype
---@field weaponType weapontype

---Before armor: the amount and the attack, damage and weapon types can still change.
---@class MoonwellWrappers.DamagingEvent: MoonwellWrappers.DamageEvent
local DamagingEvent = {}
DamagingEvent.__index = DamagingEvent

---After armor: only the amount can still change.
---@class MoonwellWrappers.DamagedEvent: MoonwellWrappers.DamageEvent
local DamagedEvent = {}
DamagedEvent.__index = DamagedEvent

---Events whose firing is still running; setters raise for any other.
---@type table<table, true>
local live = setmetatable({}, {__mode = 'k'})

---@param trigger trigger
---@param class table DamagingEvent or DamagedEvent.
local function register(trigger, class)
    local event = class == DamagingEvent and EVENT_PLAYER_UNIT_DAMAGING or EVENT_PLAYER_UNIT_DAMAGED
    for index = 0, bj_MAX_PLAYER_SLOTS - 1 do
        -- Warcraft accepts a null filter; the generated JASS signature cannot express that.
        ---@diagnostic disable-next-line: param-type-mismatch
        TriggerRegisterPlayerUnitEvent(trigger, Player(index), event, nil)
    end
end

---@param class table DamagingEvent or DamagedEvent.
---@param cells MoonwellWrappers.ListenerCell[]
local function route(class, cells)
    local event = setmetatable({
        source = Unit.fromHandle(GetEventDamageSource()),
        target = Unit.fromHandle(BlzGetEventDamageTarget()),
        amount = GetEventDamage(),
        isAttack = BlzGetEventIsAttack(),
        attackType = BlzGetEventAttackType(),
        damageType = BlzGetEventDamageType(),
        weaponType = BlzGetEventWeaponType(),
    }, class)
    live[event] = true
    Listeners.call(cells, 'Damage listener', event)
    live[event] = nil
end

local listeners = Listeners.new('DamageListener', register, route)

---Runs `callback` for every hit before armor, behind the callback boundary.
---@param callback fun(event: MoonwellWrappers.DamagingEvent): ...
---@return MoonwellWrappers.DamageListener
function Damage.onDamaging(callback)
    Callback.check(callback, 'Damage.onDamaging')
    return (Listeners.add(listeners, DamagingEvent, callback, 'Damage.onDamaging'))
end

---Runs `callback` for every hit after armor, behind the callback boundary.
---@param callback fun(event: MoonwellWrappers.DamagedEvent): ...
---@return MoonwellWrappers.DamageListener
function Damage.onDamaged(callback)
    Callback.check(callback, 'Damage.onDamaged')
    return (Listeners.add(listeners, DamagedEvent, callback, 'Damage.onDamaged'))
end

---Removes a listener at once, even during a firing. Removing it twice does nothing.
---@param token MoonwellWrappers.DamageListener
function Damage.off(token)
    Listeners.remove(listeners, token, 'Damage.off')
end

return Damage
```

- [ ] **Step 5: Run the tests**

Run: `deno task test damage unit blame imports`
Expected: all four print `SUITE PASSED`.

- [ ] **Step 6: Commit**

```bash
git add src/wrappers/internal/listeners.lua src/wrappers/damage.lua tests/damage.lua
git commit -m "feat: wrappers.damage listeners for the DAMAGING and DAMAGED phases"
```

---

### Task 3: Damage event setters and the stale check

**Files:**
- Modify: `src/wrappers/damage.lua` (after the `live` table, before `register`)
- Test: `tests/damage.lua` (append)

**Interfaces:**
- Consumes: `DamagingEvent`, `DamagedEvent`, `live` from Task 2.
- Produces: `DamagingEvent:setAmount(amount)`, `:setAttackType(attacktype)`, `:setDamageType(damagetype)`,
  `:setWeaponType(weapontype)`; `DamagedEvent:setAmount(amount)`.

- [ ] **Step 1: Write the failing tests**

Append to `tests/damage.lua`:

```lua
test('setAmount changes the hit and the shared event', function()
    local seen
    local a = Damage.onDamaging(function(event) event:setAmount(event.amount * 2) end)
    local b = Damage.onDamaging(function(event) seen = event.amount end)
    local result = fire(EVENT_PLAYER_UNIT_DAMAGING)
    eq(seen, 20); eq(result.amount, 20); expectCall('BlzSetEventDamage', 20)
    Damage.off(a); Damage.off(b)
    local c = Damage.onDamaged(function(event) event:setAmount(3); seen = event end)
    result = fire(EVENT_PLAYER_UNIT_DAMAGED)
    eq(result.amount, 3); eq(seen.amount, 3)
    eq(#PRINTED, 0)
    Damage.off(c)
end)

test('a DAMAGING event changes the types; a DAMAGED event has no type setters', function()
    local seen
    local a = Damage.onDamaging(function(event)
        event:setAttackType(ATTACK_CHAOS); event:setDamageType(DAMAGE_UNIVERSAL); event:setWeaponType(WEAPON_METAL)
        seen = event
    end)
    local result = fire(EVENT_PLAYER_UNIT_DAMAGING)
    eq(result.attackType, ATTACK_CHAOS); eq(result.damageType, DAMAGE_UNIVERSAL); eq(result.weaponType, WEAPON_METAL)
    eq(seen.attackType, ATTACK_CHAOS); eq(seen.damageType, DAMAGE_UNIVERSAL); eq(seen.weaponType, WEAPON_METAL)
    Damage.off(a)
    local b = Damage.onDamaged(function(event) seen = event end)
    fire(EVENT_PLAYER_UNIT_DAMAGED)
    eq(seen.setAttackType, nil); eq(seen.setDamageType, nil); eq(seen.setWeaponType, nil)
    eq(#PRINTED, 0)
    Damage.off(b)
end)

test('setters check their arguments at the caller', function()
    local ran = false
    local token = Damage.onDamaging(function(event)
        failsAt(function() event:setAmount('1') end, 'DamagingEvent.setAmount: expected a finite number')
        failsAt(function() event:setAmount(0 / 0) end, 'DamagingEvent.setAmount: expected a finite number')
        failsAt(function() event:setAmount(math.huge) end, 'DamagingEvent.setAmount: expected a finite number')
        failsAt(function() event:setAmount(-math.huge) end, 'DamagingEvent.setAmount: expected a finite number')
        failsAt(function() event:setAttackType(nil) end, 'DamagingEvent.setAttackType: expected an attack type')
        failsAt(function() event:setDamageType(nil) end, 'DamagingEvent.setDamageType: expected a damage type')
        failsAt(function() event:setWeaponType(nil) end, 'DamagingEvent.setWeaponType: expected a weapon type')
        event:setAmount(-5)
        ran = true
    end)
    fire(EVENT_PLAYER_UNIT_DAMAGING)
    eq(#PRINTED, 0); eq(ran, true)
    eq(callCount('BlzSetEventDamage'), 1); expectCall('BlzSetEventDamage', -5)
    eq(callCount('BlzSetEventAttackType') + callCount('BlzSetEventDamageType') + callCount('BlzSetEventWeaponType'), 0)
    Damage.off(token)
end)

test('setters raise once the firing is over', function()
    local damaging, damaged
    local a = Damage.onDamaging(function(event) damaging = event end)
    local b = Damage.onDamaged(function(event) damaged = event end)
    fire(EVENT_PLAYER_UNIT_DAMAGING); fire(EVENT_PLAYER_UNIT_DAMAGED)
    Damage.off(a); Damage.off(b)
    resetCalls()
    failsAt(function() damaging:setAmount(1) end, 'DamagingEvent.setAmount: the damage event is over')
    failsAt(function() damaging:setAttackType(ATTACK_CHAOS) end, 'DamagingEvent.setAttackType: the damage event is over')
    failsAt(function() damaging:setDamageType(DAMAGE_UNIVERSAL) end,
        'DamagingEvent.setDamageType: the damage event is over')
    failsAt(function() damaging:setWeaponType(WEAPON_METAL) end, 'DamagingEvent.setWeaponType: the damage event is over')
    failsAt(function() damaged:setAmount(1) end, 'DamagedEvent.setAmount: the damage event is over')
    eq(totalCalls(), 0)
end)

test('a nested hit gets its own event and the outer one stays live', function()
    local outer, inner, depth = nil, nil, 0
    local token = Damage.onDamaging(function(event)
        depth = depth + 1
        if depth == 1 then
            outer = event
            local saved = hit
            fire(EVENT_PLAYER_UNIT_DAMAGING, {amount = 1})
            hit = saved
            event:setAmount(7)
        else
            inner = event
        end
    end)
    local result = fire(EVENT_PLAYER_UNIT_DAMAGING)
    eq(#PRINTED, 0)
    eq(outer ~= inner, true); eq(inner.amount, 1); eq(outer.amount, 7); eq(result.amount, 7)
    failsAt(function() inner:setAmount(2) end, 'DamagingEvent.setAmount: the damage event is over')
    Damage.off(token)
end)
```

- [ ] **Step 2: Run it to see it fail**

Run: `deno task test damage`
Expected: the five new tests fail with `attempt to call a nil value (method 'setAmount')` (or `setAttackType`).

- [ ] **Step 3: Implement the setters**

In `src/wrappers/damage.lua`, insert after the `live` table:

```lua
---@param event table
---@param amount unknown
---@param operation string
local function setAmount(event, amount, operation)
    if not live[event] then error('[wrappers] ' .. operation .. ': the damage event is over', 3) end
    if type(amount) ~= 'number' or amount ~= amount or amount == math.huge or amount == -math.huge then
        error('[wrappers] ' .. operation .. ': expected a finite number', 3)
    end
    BlzSetEventDamage(amount)
    event.amount = amount
end

---@param event table
---@param value unknown
---@param operation string
---@param what string For the error, e.g. 'an attack type'.
local function checkType(event, value, operation, what)
    if not live[event] then error('[wrappers] ' .. operation .. ': the damage event is over', 3) end
    if value == nil then error('[wrappers] ' .. operation .. ': expected ' .. what, 3) end
end

---Sets the hit's amount (BlzSetEventDamage). Negative amounts are passed to the native unchanged.
---@param amount number
function DamagingEvent:setAmount(amount) setAmount(self, amount, 'DamagingEvent.setAmount') end
---Sets the attack type, which picks the armor table (BlzSetEventAttackType).
---@param attackType attacktype
function DamagingEvent:setAttackType(attackType)
    checkType(self, attackType, 'DamagingEvent.setAttackType', 'an attack type')
    BlzSetEventAttackType(attackType)
    self.attackType = attackType
end
---Sets the damage type, which decides immunity and spell reduction (BlzSetEventDamageType).
---@param damageType damagetype
function DamagingEvent:setDamageType(damageType)
    checkType(self, damageType, 'DamagingEvent.setDamageType', 'a damage type')
    BlzSetEventDamageType(damageType)
    self.damageType = damageType
end
---Sets the weapon type, which decides the impact sound (BlzSetEventWeaponType).
---@param weaponType weapontype
function DamagingEvent:setWeaponType(weaponType)
    checkType(self, weaponType, 'DamagingEvent.setWeaponType', 'a weapon type')
    BlzSetEventWeaponType(weaponType)
    self.weaponType = weaponType
end
---Sets the hit's amount after armor (BlzSetEventDamage). Negative amounts are passed to the native unchanged.
---@param amount number
function DamagedEvent:setAmount(amount) setAmount(self, amount, 'DamagedEvent.setAmount') end
```

- [ ] **Step 4: Run the tests**

Run: `deno task test damage blame`
Expected: both print `SUITE PASSED`.

- [ ] **Step 5: Commit**

```bash
git add src/wrappers/damage.lua tests/damage.lua
git commit -m "feat: damage event setters that raise once the firing is over"
```

---

### Task 4: `wrappers.sync`

**Files:**
- Create: `src/wrappers/sync.lua`
- Test: `tests/sync.lua`

**Interfaces:**
- Consumes: `Listeners.new/add/remove/call` (Task 2), `Callback.check`, `PlayerWrapper.fromHandle(raw)`.
- Produces: `Sync.send(prefix: string, data: string): boolean`, `Sync.on(prefix, callback): MoonwellWrappers.SyncListener`,
  `Sync.off(token)`.

- [ ] **Step 1: Write the failing tests**

Create `tests/sync.lua`:

```lua
bj_MAX_PLAYERS = 24
local actions = {}
native('CreateTrigger', function() return {prefixes = {}, enabled = true} end)
native('BlzTriggerRegisterPlayerSyncEvent', function(trigger, _, prefix)
    trigger.prefixes[#trigger.prefixes + 1] = prefix
    return {}
end)
native('TriggerAddAction', function(trigger, callback)
    actions[#actions + 1] = {trigger = trigger, callback = callback}
    return {}
end)
native('EnableTrigger', function(trigger) trigger.enabled = true end)
native('DisableTrigger', function(trigger) trigger.enabled = false end)
native('BlzSendSyncData', function() return true end)
local message = {}
native('GetTriggerPlayer', function() return message.player end)
native('BlzGetTriggerSyncData', function() return message.data end)
local Sync = require('wrappers.sync')
local Player = require('wrappers.player')
eq(totalCalls(), 0)

-- Simulates a synced message arriving: runs the action of every enabled trigger registered for `prefix`.
local function deliver(prefix, data, who)
    message = {player = who or PLAYER_RAW, data = data}
    for _, action in ipairs(actions) do
        if action.trigger.enabled and action.trigger.prefixes[1] == prefix then action.callback() end
    end
end

test('one trigger per prefix, registered for every player, disabled when empty', function()
    local a = Sync.on('load', function() end)
    eq(callCount('CreateTrigger'), 1); eq(callCount('BlzTriggerRegisterPlayerSyncEvent'), 24)
    expectCall('BlzTriggerRegisterPlayerSyncEvent', actions[1].trigger, PLAYER_RAW, 'load', false)
    local b = Sync.on('load', function() end)
    local c = Sync.on('save', function() end)
    eq(callCount('CreateTrigger'), 2)
    Sync.off(a); Sync.off(b)
    eq(actions[1].trigger.enabled, false); eq(actions[2].trigger.enabled, true)
    local again = Sync.on('load', function() end)
    eq(callCount('CreateTrigger'), 2); eq(actions[1].trigger.enabled, true)
    Sync.off(again); Sync.off(c)
    eq(callCount('DestroyTrigger'), 0)
end)

test('listeners get the player and the data, in order', function()
    local seen = {}
    local a = Sync.on('load', function(player, data) seen[#seen + 1] = {'a', player, data} end)
    local b = Sync.on('load', function(player, data) seen[#seen + 1] = {'b', player, data} end)
    local other = Sync.on('save', function() seen[#seen + 1] = {'save'} end)
    local sender = {}
    deliver('load', 'hello', sender)
    eq(#seen, 2); eq(seen[1][1], 'a'); eq(seen[2][1], 'b')
    eq(seen[1][2], Player.fromHandle(sender)); eq(seen[1][3], 'hello'); eq(seen[2][3], 'hello')
    eq(#PRINTED, 0)
    Sync.off(a); Sync.off(b); Sync.off(other)
end)

test('a listener added during a firing waits; a failing one is printed and the next still runs', function()
    local log, late = {}, nil
    local first = Sync.on('load', function()
        log[#log + 1] = 'first'
        if not late then late = Sync.on('load', function() log[#log + 1] = 'late' end) end
    end)
    local broken = Sync.on('load', function() error('intentional sync probe') end)
    deliver('load', 'x')
    eq(table.concat(log, ','), 'first'); eq(#PRINTED, 1)
    assert(PRINTED[1]:find('[wrappers] Sync listener callback failed:', 1, true), PRINTED[1])
    Sync.off(broken)
    deliver('load', 'x')
    eq(table.concat(log, ','), 'first,first,late')
    Sync.off(first); Sync.off(late)
end)

test('send checks the prefix and the 255-byte limit', function()
    local longest = string.rep('a', 255)
    eq(Sync.send('load', longest), true); expectCall('BlzSendSyncData', 'load', longest)
    eq(Sync.send('load', ''), true)
    failsAt(function() Sync.send('load', string.rep('a', 256)) end,
        'Sync.send: data is 256 bytes, over the 255-byte limit')
    failsAt(function() Sync.send('load', string.rep('\u{e9}', 128)) end,
        'Sync.send: data is 256 bytes, over the 255-byte limit')
    failsAt(function() Sync.send('load', 5) end, 'Sync.send: expected a string')
    failsAt(function() Sync.send('', 'x') end, 'Sync.send: expected a non-empty prefix string')
    failsAt(function() Sync.send(nil, 'x') end, 'Sync.send: expected a non-empty prefix string')
    eq(callCount('BlzSendSyncData'), 2)
    native('BlzSendSyncData', function() return false end)
    eq(Sync.send('load', 'x'), false)
end)

test('on and off check their arguments at the caller', function()
    failsAt(function() Sync.on('', function() end) end, 'Sync.on: expected a non-empty prefix string')
    failsAt(function() Sync.on(7, function() end) end, 'Sync.on: expected a non-empty prefix string')
    failsAt(function() Sync.on('load', nil) end, 'Sync.on: expected a callback function')
    failsAt(function() Sync.off({}) end, 'Sync.off: expected SyncListener token')
    local token = Sync.on('load', function() end)
    Sync.off(token); Sync.off(token)
end)
```

- [ ] **Step 2: Run it to see it fail**

Run: `deno task test sync`
Expected: the suite fails at load with `module 'wrappers.sync' not found`.

- [ ] **Step 3: Implement**

Create `src/wrappers/sync.lua`:

```lua
local Callback = require('wrappers.internal.callback')
local Listeners = require('wrappers.internal.listeners')
local PlayerWrapper = require('wrappers.player')

---Synced messages between players. Send from the local player only (inside a local-player branch); listeners run on
---every machine, in the same order, some frames after the send. Each prefix has one shared trigger, created by its
---first listener. Nothing is created at import.
local Sync = {}

---@class MoonwellWrappers.SyncListener

---The game cuts longer messages to 255 bytes and still reports success (measured on 3.0.0.24268).
local LIMIT = 255

---@param prefix unknown
---@param operation string
local function checkPrefix(prefix, operation)
    if type(prefix) ~= 'string' or prefix == '' then
        error('[wrappers] ' .. operation .. ': expected a non-empty prefix string', 3)
    end
end

---@param trigger trigger
---@param prefix string
local function register(trigger, prefix)
    for index = 0, bj_MAX_PLAYERS - 1 do BlzTriggerRegisterPlayerSyncEvent(trigger, Player(index), prefix, false) end
end

---@param _ string
---@param cells MoonwellWrappers.ListenerCell[]
local function route(_, cells)
    Listeners.call(cells, 'Sync listener', PlayerWrapper.fromHandle(GetTriggerPlayer()), BlzGetTriggerSyncData())
end

local listeners = Listeners.new('SyncListener', register, route)

---Sends `data` to every player under `prefix`. Call it for the local player only.
---@param prefix string
---@param data string At most 255 bytes.
---@return boolean sent What BlzSendSyncData returned.
function Sync.send(prefix, data)
    checkPrefix(prefix, 'Sync.send')
    if type(data) ~= 'string' then error('[wrappers] Sync.send: expected a string', 2) end
    if #data > LIMIT then
        error('[wrappers] Sync.send: data is ' .. #data .. ' bytes, over the ' .. LIMIT .. '-byte limit', 2)
    end
    return BlzSendSyncData(prefix, data)
end

---Runs `callback` with the sending Player and the data for every message under `prefix`, behind the callback boundary.
---@param prefix string
---@param callback fun(player: MoonwellWrappers.Player, data: string): ...
---@return MoonwellWrappers.SyncListener
function Sync.on(prefix, callback)
    checkPrefix(prefix, 'Sync.on')
    Callback.check(callback, 'Sync.on')
    return (Listeners.add(listeners, prefix, callback, 'Sync.on'))
end

---Removes a listener at once, even during a firing. Removing it twice does nothing.
---@param token MoonwellWrappers.SyncListener
function Sync.off(token)
    Listeners.remove(listeners, token, 'Sync.off')
end

return Sync
```

- [ ] **Step 4: Run the tests**

Run: `deno task test sync damage blame`
Expected: all three print `SUITE PASSED`.

- [ ] **Step 5: Commit**

```bash
git add src/wrappers/sync.lua tests/sync.lua
git commit -m "feat: wrappers.sync with a 255-byte send check and listeners per prefix"
```

---

### Task 5: Sweep, imports, editor fixtures and integration

**Files:**
- Modify: `tests/blame.lua` (append a test)
- Modify: `tests/imports.lua` (append two tests)
- Modify: `tests/editor-negative.lua` (before `return true`)
- Modify: `tests/editor-positive.lua` (before `return true`) and `tests/editor-positive.yue` (imports and the end of
  `mw.on_main`)
- Modify: `tools/integration.ts` (the Unit-only unused list, `publicModules`, `soloEntries`, the log line)

**Interfaces:**
- Consumes: everything from Tasks 1–4.

- [ ] **Step 1: Extend the sweep and the import tests**

Append to `tests/blame.lua`:

```lua
test('damage and sync functions given wrong arguments point at their caller', function()
    local wrong, checked = {}, 0
    for _, name in ipairs({'damage', 'sync'}) do
        for key, fn in pairs(require('wrappers.' .. name)) do
            if type(fn) == 'function' then
                local ok, err = pcall(function() fn({}) end)
                assert(not ok, name .. '.' .. key .. ' accepted a table')
                checked = checked + 1
                if not tostring(err):find('^%./tests/blame%.lua:%d+: ') then
                    wrong[#wrong + 1] = name .. '.' .. key .. ' -> ' .. tostring(err)
                end
            end
        end
    end
    eq(checked, 6)
    table.sort(wrong)
    assert(#wrong == 0, #wrong .. ' misplaced errors:\n' .. table.concat(wrong, '\n'))
end)
```

Append to `tests/imports.lua` (the sync test must come before the damage test, which loads Unit):

```lua
test('the sync module loads the Player module and no other', function()
    require('wrappers.sync')
    eq(totalCalls(), 0)
    eq(package.loaded['wrappers.player'] ~= nil, true)
    for _, name in ipairs({'unit', 'group', 'item', 'force'}) do eq(package.loaded['wrappers.' .. name], nil) end
end)

test('the damage module loads Unit and what Unit loads, and nothing else', function()
    require('wrappers.damage')
    eq(totalCalls(), 0)
    eq(package.loaded['wrappers.unit'] ~= nil, true)
    for _, name in ipairs({'group', 'force'}) do eq(package.loaded['wrappers.' .. name], nil) end
end)
```

- [ ] **Step 2: Run them**

Run: `deno task test blame imports`
Expected: both print `SUITE PASSED` (the code from Tasks 2–4 already satisfies them; if the sweep reports a misplaced
error, fix the level or the tail call in the named function, not the test).

- [ ] **Step 3: Add the editor fixtures**

In `tests/editor-negative.lua`, before `return true`:

```lua
local Damage = require('wrappers.damage')
local Sync = require('wrappers.sync')
Damage.onDamaged(function(event) event:setDamageType(DAMAGE_TYPE_UNIVERSAL) end) -- EXPECT undefined-field
Sync.on('load', function(player) Group.create():add(player) end) -- EXPECT param-type-mismatch
Damage.off(Sync.on('load', function() end)) -- EXPECT param-type-mismatch
```

In `tests/editor-positive.lua`, before `return true`:

```lua
local Damage = require('wrappers.damage')
local Sync = require('wrappers.sync')
local damaging = Damage.onDamaging(function(event)
    event:setAmount(event.amount * 2)
    event:setDamageType(DAMAGE_TYPE_UNIVERSAL)
    local source = event.source
    if source then print(source:getName(), event.target:getName()) end
end)
Damage.off(damaging)
Damage.off(Damage.onDamaged(function(event) event:setAmount(event.amount + 1) end))
local synced = Sync.on('load', function(player, data) print(player:getName(), data) end)
Sync.off(synced)
if Sync.send('load', 'code') then print('sent') end
print(unit:getCollisionSize())
unit:setPathing(true)
```

(If `unit` is removed earlier in the positive file, place these lines before the `unit:remove()` line instead.)

In `tests/editor-positive.yue`, add after the existing imports:

```
import "wrappers.damage" as Damage
import "wrappers.sync" as Sync
```

and at the end of the `mw.on_main` block (two-space indentation, after the `dialog\addButton` line):

```
  listener = Damage.onDamaging (event) ->
    event\setAmount event.amount * 2
    event\setDamageType DAMAGE_TYPE_UNIVERSAL
  Damage.off listener
  Sync.on "load", (player, data) -> print player\getName!, data
  print unit\getCollisionSize!
  unit\setPathing false
```

- [ ] **Step 4: Update the integration script**

In `tools/integration.ts` (edit with the file-editing tool; the YueScript `\` must stay a doubled `\\` inside the
TypeScript strings):
- add `"damage",` and `"sync",` to the Unit-only bundle's unused list (after `"frame",`);
- add `"damage",` and `"sync",` to `publicModules` (after `"frame",`);
- add two `soloEntries`:

```ts
  damage: {
    source: 'import "wrappers.damage" as Damage\nt = Damage.onDamaged (event) -> event\\setAmount 0\nDamage.off t\n',
    allowed: ["unit", "player", "item"],
  },
  sync: {
    source: 'import "wrappers.sync" as Sync\nt = Sync.on "load", (player, data) -> print data\nSync.off t\n',
    allowed: ["player"],
  },
```

- change the log line to
  `"Moonwell: Trigger-, TextTag-, Multiboard-, Dialog-, Frame-, Damage- and Sync-only maps bundle only what they import"`.

- [ ] **Step 5: Run every check**

Run: `deno task test`, `deno task check:lua`, `deno task check`, `deno task lint`, `deno fmt --check`,
`deno task test:integration`.
Expected: every suite passes; `check:lua` counts 3 more files than before (listeners, damage, sync); integration prints
`LuaLS: 20 intentional type errors detected at the expected lines`, the native-call check stays clean, and the new
Damage- and Sync-only bundle line. If LuaLS reports a diagnostic in `src/wrappers/damage.lua`, `sync.lua` or
`internal/listeners.lua`, fix the annotation (not by a blanket `---@diagnostic disable`), and re-run.

- [ ] **Step 6: Commit**

```bash
git add tests/blame.lua tests/imports.lua tests/editor-negative.lua tests/editor-positive.lua tests/editor-positive.yue tools/integration.ts
git commit -m "test: sweep, imports, editor fixtures and bundles for damage and sync"
```

---

### Task 6: Documentation

**Files:**
- Modify: `README.md` (intro line 3, the `wrappers.unit` API section, two new API sections after `wrappers.frame`'s)
- Modify: `CHANGELOG.md` (a `## Unreleased` section above `## 0.6.0`)
- Modify: `AGENTS.md` (the scope paragraph around line 24)

- [ ] **Step 1: README**

Intro (line 3–5): add `Damage and Sync` so the list reads
`… Quest, DefeatCondition, TimerDialog and Frame wrappers, Damage and Sync modules, editor completion, …`.

In `### wrappers.unit`, change the presentation bullet's end to
`` `setInvulnerable(flag)`, `isInvulnerable()`, `show(flag)`, `isHidden()`, `getCollisionSize()`, `setPathing(flag)` ``.

After the `### wrappers.frame` section (before `## Callbacks`), add:

````markdown
### `wrappers.damage`

- `onDamaging(callback)` (before armor) and `onDamaged(callback)` (after armor) return a token for `off(token)`
- the event: `source` (a Unit, or nil when the game gives none), `target`, `amount`, `isAttack`, `attackType`,
  `damageType`, `weaponType`
- DAMAGING events: `setAmount(n)`, `setAttackType(t)`, `setDamageType(t)`, `setWeaponType(t)`; DAMAGED events:
  `setAmount(n)`

Every listener of one hit gets the same event, so a change is visible to the listeners after it. Setters work only
while the hit's listeners run; afterwards (for example from a timer) they raise `the damage event is over`. Each phase
has one shared trigger, created by the first listener and disabled while the phase has none.

```yue
import "wrappers.damage" as Damage

Damage.onDamaging (event) ->
  if event.isAttack
    event\setAmount event.amount * 1.5
```

### `wrappers.sync`

- `send(prefix, data)` returns what `BlzSendSyncData` returns; `data` is at most 255 bytes
- `on(prefix, callback)` returns a token for `off(token)`; the callback gets `(Player, data)`

Call `send` for the local player only, inside a local-player branch; the listeners run on every machine, in the same
order, some frames later (about 0.09 s on one machine). The game cuts longer messages to 255 bytes and still reports
success, so `send` raises instead. Split longer data yourself.

```yue
import "wrappers.sync" as Sync

Sync.on "load", (player, data) -> print player\getName!, data
Sync.send "load", code if Player.fromIndex(0)\isLocal!
```
````

- [ ] **Step 2: CHANGELOG**

Above `## 0.6.0 (2026-09-30)`:

```markdown
## Unreleased

- New `unit:getCollisionSize()` and `unit:setPathing(flag)`.
- New `wrappers.damage`: listeners before armor (`onDamaging`) and after armor (`onDamaged`), removable with
  `Damage.off`. One event per hit, shared by its listeners, with the source, target, amount, attack flag and types.
  DAMAGING events can change the amount and the three types; DAMAGED events only the amount. Setters raise once the
  hit's listeners have run.
- New `wrappers.sync`: `Sync.send` raises for data over 255 bytes (the game cuts it silently) and for an empty prefix;
  `Sync.on` and `Sync.off` manage listeners per prefix.
- Nothing existing changes; there are no migrations.
```

- [ ] **Step 3: AGENTS.md**

In the scope paragraph (the one ending `v0.5.0 adds frames.`), append:
`v0.7.0 adds damage events and sync, for the wc3-lib port, on a shared internal listener list
(`internal/listeners.lua`).`

- [ ] **Step 4: Check and commit**

Run: `deno fmt --check` (Markdown is not formatted by it if excluded; the command must still pass).

```bash
git add README.md CHANGELOG.md AGENTS.md
git commit -m "docs: damage, sync and the two Unit methods"
```

---

### Task 7: The in-game gate run

**Files:**
- Modify: `examples/gate.yue` (header comment, imports, a `port = false` flag, a `portGate` function, the dispatch)
- Modify: `CONTRIBUTING.md` (a new step 13 after step 12, before the `v0.1.0:` records)
- Modify (not under git): `../wrappers-gate/gate.ts` (the `port` flag and run)

**Interfaces:**
- Consumes: `Damage`, `Sync`, `unit:getCollisionSize()`, `unit:setPathing(flag)`, `unit:damageTarget(...)`,
  `unit:issuePointOrder(order, x, y)`, `Destructable.create(typeId, x, y, facing, scale, variation)`.

- [ ] **Step 1: gate.yue**

Header: after the `frames = true` comment line add
`-- Set port = true for a separate run of the v0.7.0 port prerequisites gate only (damage, pathing, sync).`

Imports: after `import "wrappers.frame" as Frame` add:

```
import "wrappers.damage" as Damage
import "wrappers.sync" as Sync
```

Flags: after `frames = false` add `port = false`.

Before the `-- Start just after the map loads` comment, add:

```
-- v0.7.0 port prerequisites. Run with port = true: one step per second. CONTRIBUTING step 13 lists every message.
portGate = (owner) ->
  hostile = Player.fromIndex 12
  striker = Unit.create owner, $FourCC("hfoo"), -200, 0, 0
  dummy = Unit.create hostile, $FourCC("hfoo"), 0, 0, 180
  striker\pause true
  dummy\pause true
  dummy\setMaxLife 5000
  dummy\setLife 5000
  print "Wrapper collision size", striker\getCollisionSize!
  for row = -4, 4
    Destructable.create $FourCC("LTlt"), 448, row * 128, 270, 1, 0
  step = 0
  nestedDone = false
  storedEvent = nil
  damaging = Damage.onDamaging (event) ->
    if event.target == dummy
      if step == 2
        before = event.amount
        event\setAmount before * 2
        source = event.source
        sourceName = source and source\getName! or "none"
        print "Wrapper damaging", sourceName, event.target\getName!, event.isAttack, before, event.amount
      elseif step == 3
        event\setAttackType ATTACK_TYPE_MAGIC
      elseif step == 5 and not nestedDone
        nestedDone = true
        storedEvent = event
        striker\damageTarget dummy, 10, false, false, ATTACK_TYPE_NORMAL, DAMAGE_TYPE_NORMAL, WEAPON_TYPE_WHOKNOWS
        event\setAmount 0
  damaged = Damage.onDamaged (event) ->
    if event.target == dummy
      BlzSetEventAttackType ATTACK_TYPE_MAGIC if step == 4
      print "Wrapper damaged step", step, "amount", event.amount
  strike = ->
    lifeBefore = dummy\getLife!
    striker\damageTarget dummy, 100, true, false, ATTACK_TYPE_NORMAL, DAMAGE_TYPE_NORMAL, WEAPON_TYPE_WHOKNOWS
    print "Wrapper life loss step", step, lifeBefore - dummy\getLife!
  walker = nil
  longest = string.rep "s", 255
  Sync.on "wgate", (player, data) ->
    print "Wrapper sync received from", player\getName!, #data, data == longest
  Sync.on string.rep("p", 16), (player, data) -> print "Wrapper sync prefix 16 arrived", data
  Sync.on string.rep("q", 17), (player, data) -> print "Wrapper sync prefix 17 arrived whole", data
  Sync.on string.rep("q", 16), (player, data) -> print "Wrapper sync prefix 17 arrived cut to 16", data
  Sync.on string.rep("r", 32), (player, data) -> print "Wrapper sync prefix 32 arrived whole", data
  Sync.on string.rep("r", 16), (player, data) -> print "Wrapper sync prefix 32 arrived cut to 16", data
  steps = Timer.create!
  steps\start 1, true, (self) ->
    step += 1
    if step <= 5
      strike!
    elseif step == 6
      staleOk, staleMessage = pcall ->
        storedEvent\setAmount 1 if storedEvent
        return
      print "Wrapper stale damage event", staleOk, staleMessage
      Damage.off damaging
      Damage.off damaged
    elseif step == 7
      strike!
      print "Wrapper damage listeners removed"
    elseif step == 8
      walker = Unit.create owner, $FourCC("hfoo"), 250, 0, 0
      walker\setPathing false
      walker\issuePointOrder "move", 650, 0
      print "Wrapper pathing walker ordered through the trees"
    elseif step == 12
      print "Wrapper pathing walker x", walker and walker\getX! or "none"
    elseif step == 13
      if owner\isLocal!
        print "Wrapper sync sent", Sync.send "wgate", longest
        rejectedOk, rejectedMessage = pcall ->
          Sync.send "wgate", longest .. "s"
          return
        print "Wrapper sync 256 bytes rejected", not rejectedOk, rejectedMessage
        Sync.send string.rep("p", 16), "sixteen"
        Sync.send string.rep("q", 17), "seventeen"
        Sync.send string.rep("r", 32), "thirty-two"
    elseif step == 15
      self\destroy!
      print "Wrapper port gate done"
  print "Wrapper port gate started: watch steps 1 to 15, one per second"
```

Dispatch: in `mw.on_main`, make `port` the first branch:

```
    if port
      portGate owner
    elseif frames
      framesGate!
```

(keep the other branches as they are).

- [ ] **Step 2: Build it**

Run: `deno task test:integration` (it builds `examples/gate.yue` and checks its editor diagnostics).
Expected: passes, with no new diagnostic in the gate. If the gate reports `need-check-nil` for `storedEvent` or
`walker`, the guards above are the fix; adjust the guard, not the check.

- [ ] **Step 3: The gate map run**

In `../wrappers-gate/gate.ts`:
- add `//   port              v0.7.0 port prerequisites gate (step 13)` to the header list after `frames-min`;
- add `port: boolean` to the `Run` type and `port: false` to every existing entry of `runs`;
- add `"port": { probes: false, presentation: false, ui: false, frames: false, port: true, minify: false },`;
- in `variant`, after the `frames` line: `text = replace(text, "port = false", \`port = ${run.port}\`);`.

Run from `../wrappers-gate`: `deno task gate port --no-launch`
Expected: `Gate map: gate-maps/port.w3x`.

- [ ] **Step 4: CONTRIBUTING step 13**

After step 12 (before the `v0.1.0:` record), add:

```markdown
13. Port prerequisites (v0.7.0): in the gate map, `deno task gate port` (normal build only). Only the port gate runs; a
    footman stands at the centre with another one to its left, and a line of trees runs north–south to their right.
    Messages, one step per second:
    - at start: `Wrapper collision size <n>` (record it) and `Wrapper port gate started`;
    - step 1: `Wrapper damaged step 1 amount <X>` and `Wrapper life loss step 1 <X>`: the baseline, 100 reduced by armor;
    - step 2: `Wrapper damaging Footman Footman true 100.0 200.0`, then `Wrapper damaged step 2 amount <Y>` and
      `Wrapper life loss step 2 <Y>`, with Y about twice X;
    - step 3: the attack type changed to magic before armor: `Wrapper damaged step 3 amount <Z>` with Z different from X
      (record it);
    - step 4: the attack type changed with the raw native after armor: `Wrapper life loss step 4 <X>`, unchanged, which
      confirms that type changes after armor do nothing;
    - step 5: a nested 10-damage hit inside the outer hit's DAMAGING listener, then `setAmount 0` on the outer event.
      Record every `Wrapper damaged step 5` line and the life loss: if the outer line reads amount 0 and the life loss is
      only the nested hit's, the outer setter still works after a nested hit;
    - step 6: `Wrapper stale damage event false <…>DamagingEvent.setAmount: the damage event is over`;
    - step 7: `Wrapper life loss step 7 <X>` with no `Wrapper damaged` line, then `Wrapper damage listeners removed`;
    - step 8: a third footman appears left of the trees and walks straight through them; at step 12
      `Wrapper pathing walker x <n>` with n over 448;
    - step 13: `Wrapper sync sent true`, `Wrapper sync 256 bytes rejected true <…>over the 255-byte limit`, then shortly
      `Wrapper sync received from <your name> 255 true` and the prefix lines: record which of `prefix 16 arrived`,
      `prefix 17 arrived whole/cut to 16` and `prefix 32 arrived whole/cut to 16` print;
    - step 15: `Wrapper port gate done`, and no `[wrappers] ... failed` line at any point.
```

- [ ] **Step 5: Commit**

```bash
git add examples/gate.yue CONTRIBUTING.md
git commit -m "test: the v0.7.0 port prerequisites gate run"
```

---

### Task 8: Release

**Files:**
- Modify: `CHANGELOG.md`, `CONTRIBUTING.md`, `README.md`, `AGENTS.md` (wrappers)
- Modify: `AGENTS.md`, `docs/superpowers/plans/2026-09-30-moonwell-roadmap-to-wc3-lib.md`, `CHANGELOG.md` (Moonwell)

- [ ] **Step 1: Automated checks on the release candidate**

Run every check in Global Constraints and record the numbers (suites, Lua files, expected negatives).

- [ ] **Step 2: The in-game gate (the maintainer)**

Ask the maintainer to run `deno task gate port` and send the message log (F12) and a screenshot of the walker. Give the
instructions one step at a time, as for v0.6.0. The existing runs and the minified builds are not re-run (spec §6).

- [ ] **Step 3: Act on the two measurements**

- **Nested damage (step 5):** if the outer `setAmount 0` did not take effect, add a README "Reported native caveats"
  line: after a listener deals damage, the outer hit's setters no longer change it. No code change.
- **Prefix lengths (step 13):** if every prefix arrived whole, keep only the non-empty check and note the measured
  lengths in the README sync section. If a prefix arrived cut or not at all, stop and propose a length limit (a new
  check in `checkPrefix`, a test and a message) to the maintainer before tagging.

- [ ] **Step 4: Records, tag and release**

- CHANGELOG: `## Unreleased` becomes `## 0.7.0 (<date>)` with a `### Release gate` section (automated numbers and the
  gate results, including collision size, the step 3 value, the nested result and the prefix result).
- CONTRIBUTING: a `v0.7.0:` gate record after `v0.6.0:`; later, the tag consumption record.
- README: the Status paragraph and the GitHub example's `tag` name `v0.7.0`.
- wrappers AGENTS.md: a v0.7.0 note beside the v0.6.0 one.
- Commit, push `main`, tag `v0.7.0` on the verified commit, push the tag, and create the GitHub pre-release with the
  CHANGELOG section as notes (`gh release create v0.7.0 --prerelease`).
- Tag consumption: a fresh map (`../wrappers-tag-check-070`) with the README GitHub configuration; check and build;
  `moonwell.lock` records the tag's commit; remove `.moonwell/`, check again, the lock is unchanged. Record it and
  push.

- [ ] **Step 5: Moonwell records**

- `AGENTS.md`: a "Wrappers v0.7.0, the port prerequisites, released" state bullet; next work becomes phase 3, the spec
  for the `wc3-lib` port.
- Roadmap: 2.3 marked released with the tag commit.
- `CHANGELOG.md` Unreleased: a documentation line for the v0.7.0 records.
- Commit and push; check CI with `gh run list`.
