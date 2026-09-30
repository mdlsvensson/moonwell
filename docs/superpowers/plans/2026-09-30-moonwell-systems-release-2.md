# Moonwell Systems Release 2 (v0.2.0) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** moonwell-systems v0.2.0: `systems.internal.ordered`, `systems.buffs`, `systems.aura` and `systems.dummy`.

**Architecture:** An insertion-ordered map keys every unit-keyed collection. `BuffStore` keeps an ordered map from Unit
to its buffs, drives expiry and ticks from a `Scheduler`, and polls its units every 0.25 s. `Aura` reconciles a query
against a store. `Dummies` creates fresh Unit wrappers, configures them through wrapper methods and removes them on a
scheduler timer. Every callback runs behind `systems.internal.callback`.

**Tech Stack:** as release 1 (annotated Lua 5.3, `yue -e` tooling, LuaLS 3.19.1, Lua 5.3.6 `luac`, moonwell-wrappers
v0.7.0, Moonwell 0.5.2).

**Spec:** `docs/superpowers/specs/2026-09-30-moonwell-systems-release-2-design.md` (and Part 1 of
`2026-09-30-moonwell-systems-design.md`).

## Global Constraints

- **Repository:** `C:\Users\mdlsvensson\Repo\moonwell-systems`; Moonwell records in `C:\Users\mdlsvensson\Repo\moonwell`.
  Commit on `main`, explicit paths only, each check run as its own command. End every commit message with
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- **Checks:** `yue -e tests/run.lua`, `yue -e tools/check.lua` (with `MOONWELL_LUAC`), `yue -e tools/integration.lua`
  (with `MOONWELL_LUALS`).
- **Messages:** `[systems] <Class>.<method>: <problem>`. New texts: `expected Unit`, `expected Player`,
  `expected BuffStore`, `expected Scheduler`, `expected an options table`, `expected a finite positive poll interval`,
  `expected a buff definition table`, `expected a buff definition: <field>`, `expected a removal reason`,
  `the store is disposed`, `use the same definition for a given id and source`, `expected an aura buff definition`,
  `expected a finite positive interval`, `the aura is disposed`, `expected a cast request table`,
  `expected a cast request: <field>`, `the manager is disposed`, `the dummy cannot get ability <id>`.
- **Callback labels:** `Buff callback`, `Buff release`, `Buff poll`, `Aura query`, `Aura apply`, `Dummy release`.
- **Removal reasons:** `expired`, `dispelled`, `replaced`, `death`, `removed`, `source-lost`, `disposed`, `error`.
- **Levels and tail calls** as release 1: 2 in a public function, 3 (+ depth) in a helper; `return (helper(...))`.
- **Private fields** use `---@field package`, never `private`.
- **Locust** is `'Aloc'` = 1097625443.

---

### Task 1: `systems.internal.ordered`

**Files:** Create `src/systems/internal/ordered.lua`, `tests/ordered.lua`; modify `tests/suites.lua`.

**Interfaces:** Produces `Ordered.new()`, `:set(key, value)`, `:get(key)`, `:has(key)`, `:delete(key) -> boolean`,
`:getSize()`, `:keys() -> array`, `:each(fn)`. Internal fields `order`, `values`, `positions`, `count`, `holes`,
`iterating` (never a field named `keys`, which would hide the method).

- [ ] **Step 1: Failing tests** — `tests/ordered.lua`:

```lua
local Ordered = require('systems.internal.ordered')

local function collect(map)
    local seen = {}
    map:each(function(key, value) seen[#seen + 1] = tostring(key) .. '=' .. tostring(value) end)
    return table.concat(seen, ',')
end

test('keeps insertion order; set on an existing key keeps its place', function()
    local map = Ordered.new()
    map:set('b', 1); map:set('a', 2); map:set('c', 3); map:set('b', 4)
    eq(collect(map), 'b=4,a=2,c=3'); eq(map:getSize(), 3)
    eq(map:get('a'), 2); eq(map:has('z'), false); eq(map:get('z'), nil)
    eq(table.concat(map:keys(), ','), 'b,a,c')
end)

test('delete, and deleting during each skips the deleted entry', function()
    local map = Ordered.new()
    for _, key in ipairs({'a', 'b', 'c', 'd'}) do map:set(key, true) end
    eq(map:delete('b'), true); eq(map:delete('b'), false)
    local seen = {}
    map:each(function(key) seen[#seen + 1] = key; if key == 'a' then map:delete('c') end end)
    eq(table.concat(seen, ','), 'a,d'); eq(map:getSize(), 2); eq(table.concat(map:keys(), ','), 'a,d')
end)

test('entries added during each wait for the next; a re-added key goes last', function()
    local map = Ordered.new()
    map:set('a', 1); map:set('b', 2)
    local seen = {}
    map:each(function(key)
        seen[#seen + 1] = key
        if key == 'a' then map:set('z', 9); map:delete('a'); map:set('a', 5) end
    end)
    eq(table.concat(seen, ','), 'a,b'); eq(table.concat(map:keys(), ','), 'b,z,a')
end)

test('nested each works, and an error inside each leaves the map usable and compacting', function()
    local map = Ordered.new()
    map:set('a', 1); map:set('b', 2)
    local seen = {}
    map:each(function(outer) map:each(function(inner) seen[#seen + 1] = outer .. inner end) end)
    eq(table.concat(seen, ','), 'aa,ab,ba,bb')
    eq(pcall(function() map:each(function() error('stop') end) end), false)
    for index = 1, 100 do map:set(index, index) end
    for index = 1, 100 do map:delete(index) end
    eq(map:getSize(), 2); eq(table.concat(map:keys(), ','), 'a,b')
    assert(#map.order <= 4, 'expected compaction, order has ' .. #map.order)
end)

test('tables are keys by identity', function()
    local first, second = {}, {}
    local map = Ordered.new()
    map:set(first, 'one'); map:set(second, 'two')
    eq(map:get(first), 'one'); eq(map:keys()[2], second)
end)
```

`tests/suites.lua`: insert `'ordered'` after `'internal'`.

- [ ] **Step 2:** `yue -e tests/run.lua ordered` → `ordered: ERROR …module 'systems.internal.ordered' not found`.

- [ ] **Step 3: Implement** — `src/systems/internal/ordered.lua`:

```lua
---An insertion-ordered map for keys that are handles or wrappers (spec Part 1 §4.4): never iterate those with pairs.
---Internal: its errors are programming errors and use plain Lua messages.
---@class MoonwellSystems.Ordered
---@field package order any[] Keys in insertion order; HOLE marks a deleted key until compaction.
---@field package values table<any, any>
---@field package positions table<any, integer>
---@field package count integer
---@field package holes integer
---@field package iterating integer Running each() calls; compaction waits until none runs.
local Ordered = {}
Ordered.__index = Ordered

local HOLE = setmetatable({}, {__tostring = function() return '<deleted>' end})

---@return MoonwellSystems.Ordered
function Ordered.new()
    return setmetatable({order = {}, values = {}, positions = {}, count = 0, holes = 0, iterating = 0}, Ordered)
end

---Drops the holes once they outnumber the live keys, so deleting stays O(1) amortized.
local function compact(map)
    if map.iterating > 0 or map.holes * 2 <= #map.order then return end
    local order = {}
    for _, key in ipairs(map.order) do
        if key ~= HOLE then
            order[#order + 1] = key
            map.positions[key] = #order
        end
    end
    map.order, map.holes = order, 0
end

---A new key goes last; an existing key keeps its place.
function Ordered:set(key, value)
    if key == nil or value == nil then error('Ordered.set: key and value must not be nil', 2) end
    if self.positions[key] == nil then
        self.order[#self.order + 1] = key
        self.positions[key] = #self.order
        self.count = self.count + 1
    end
    self.values[key] = value
end

function Ordered:get(key) return self.values[key] end
function Ordered:has(key) return self.positions[key] ~= nil end
function Ordered:getSize() return self.count end

---@return boolean deleted
function Ordered:delete(key)
    local position = self.positions[key]
    if position == nil then return false end
    self.order[position] = HOLE
    self.positions[key], self.values[key] = nil, nil
    self.count, self.holes = self.count - 1, self.holes + 1
    compact(self)
    return true
end

---@return any[]
function Ordered:keys()
    local keys = {}
    for _, key in ipairs(self.order) do if key ~= HOLE then keys[#keys + 1] = key end end
    return keys
end

---Calls fn(key, value) for the entries present at the start that are still present. Entries added during the call wait
---for the next one. An error in fn propagates after the map is restored.
---@param fn fun(key: any, value: any)
function Ordered:each(fn)
    self.iterating = self.iterating + 1
    local order, limit = self.order, #self.order
    local ok, err = pcall(function()
        for index = 1, limit do
            local key = order[index]
            if key ~= HOLE and self.positions[key] == index then fn(key, self.values[key]) end
        end
    end)
    self.iterating = self.iterating - 1
    compact(self)
    if not ok then error(err, 0) end
end

return Ordered
```

(`self.positions[key] == index` also skips a key deleted and re-added during the loop: its new position is beyond
`limit`.)

- [ ] **Step 4:** `yue -e tests/run.lua; echo "exit $?"` → `ordered: SUITE PASSED: 5 tests`, `exit 0`.

- [ ] **Step 5: Commit** `src/systems/internal/ordered.lua tests/ordered.lua tests/suites.lua` —
  `feat: systems.internal.ordered, an insertion-ordered map`.

---

### Task 2: `systems.buffs`

**Files:** Create `src/systems/buffs.lua`, `tests/buffs.lua`; modify `tests/suites.lua`.

**Interfaces:**
- Consumes: `Ordered`, `Callback`, `Check`, `Scheduler` (class, `:after`, `:every`, `:getTick`, `:getStep`, `:ticks`),
  `wrappers.unit` (class; `:isDisposed()`, `:exists()`, `:isAlive()`).
- Produces: `BuffStore.new(clock, options?)`, `:apply(unit, definition, source?)`, `:get`, `:has`, `:stacks`, `:list`,
  `:clearUnit(unit, reason?)`, `:clearSource(source)`, `:getScheduler()`, `:dispose()`; `Buff` methods `getUnit`,
  `getSource`, `getId`, `getDefinition`, `isActive`, `getStacks`, `getRemaining`, `own`, `remove`; field `data`.
  Task 3 uses `store:getScheduler()` and `buff:isActive()`, `buff:remove(reason)`.

- [ ] **Step 1: Failing tests** — `tests/buffs.lua`:

```lua
local Scheduler = require('systems.scheduler')
local BuffStore = require('systems.buffs')
local Unit = require('wrappers.unit')

local function newUnit() return Unit.fromHandle({}) end
local function quiet(clock, onError) return BuffStore.new(clock, {pollInterval = 100, onError = onError}) end

test('independent stacks expire separately and respect the cap', function()
    local clock = Scheduler.new(1)
    local buffs, u, changes = quiet(clock), newUnit(), {}
    local poison = {id = 'poison', kind = 'active', stacking = 'independent', maxStacks = 2, duration = 2,
        onStacks = function(buff) changes[#changes + 1] = buff:getStacks() end}
    local buff = buffs:apply(u, poison, 'caster')
    clock:advance()
    eq(buffs:apply(u, poison, 'caster'), buff)
    buffs:apply(u, poison, 'caster')
    eq(buff:getStacks(), 2)
    clock:advance(); eq(buff:getStacks(), 1)
    clock:advance(); eq(buff:isActive(), false)
    eq(table.concat(changes, ','), '2,1')
    eq(clock:getPending(), 1) -- only the store's poll
end)

test('refresh extends a buff and removal releases owned effects exactly once', function()
    local clock = Scheduler.new(1)
    local buffs, u, cleaned = quiet(clock), newUnit(), 0
    local haste = {id = 'haste', kind = 'active', duration = 2,
        onApply = function(buff) buff:own(function() cleaned = cleaned + 1 end) end}
    local buff = buffs:apply(u, haste)
    clock:advance(); buffs:apply(u, haste); clock:advance()
    eq(buff:isActive(), true)
    clock:advance(); buff:remove()
    eq(cleaned, 1); eq(#buffs:list(u), 0)
end)

test('callbacks can remove their buff; death keeps passive buffs; a disposed store refuses', function()
    local clock = Scheduler.new(1)
    local buffs, u = quiet(clock), newUnit()
    local ephemeral = buffs:apply(u, {id = 'cancel', kind = 'active', duration = 2,
        onApply = function(buff) buff:remove() end})
    eq(ephemeral:isActive(), false); eq(clock:getPending(), 1)
    buffs:apply(u, {id = 'talent', kind = 'passive'})
    buffs:apply(u, {id = 'poison', kind = 'active', duration = 5})
    buffs:apply(u, {id = 'kept', kind = 'active', removeOnDeath = false})
    buffs:clearUnit(u, 'death'); eq(#buffs:list(u), 2)
    buffs:clearUnit(u, 'removed'); eq(#buffs:list(u), 0)
    buffs:dispose(); buffs:dispose()
    failsAt(function() buffs:apply(u, {id = 'x', kind = 'passive'}) end, 'BuffStore.apply: the store is disposed')
    eq(clock:getPending(), 0)
end)

test('a failing release is reported and the rest still run', function()
    local clock, messages, cleanup = Scheduler.new(1), {}, 0
    local buffs, u = quiet(clock, function(message) messages[#messages + 1] = message end), newUnit()
    local buff = buffs:apply(u, {id = 'cleanup', kind = 'active', duration = 1, onApply = function(b)
        b:own(function() cleanup = cleanup + 1 end)
        b:own(function() error('broken effect') end)
    end})
    buff:remove()
    eq(cleanup, 1); eq(buff:isActive(), false); eq(#messages, 1)
    assert(messages[1]:find('broken effect', 1, true), messages[1])
    eq(clock:getPending(), 1)
end)

test('periodic buffs tick, report remaining time and stop on removal', function()
    local clock = Scheduler.new(1)
    local buffs, u, ticks = quiet(clock), newUnit(), 0
    local dot = {id = 'dot', kind = 'active', duration = 3, interval = 1, onTick = function() ticks = ticks + 1 end}
    local buff = buffs:apply(u, dot)
    eq(buff:getRemaining(), 3)
    clock:advance(); clock:advance()
    eq(ticks, 2); eq(buff:getRemaining(), 1)
    clock:advance()
    eq(buff:isActive(), false); eq(buff:getRemaining(), 0)
    clock:advance()
    eq(ticks, 3); eq(clock:getPending(), 1)
    eq(buffs:apply(u, {id = 'permanent', kind = 'passive'}):getRemaining(), nil)
end)

test('a failing tick or onApply removes the buff with reason error', function()
    local clock, messages, reasons = Scheduler.new(1), {}, {}
    local buffs, u = quiet(clock, function(message) messages[#messages + 1] = message end), newUnit()
    local record = function(_, reason) reasons[#reasons + 1] = reason end
    local buff = buffs:apply(u, {id = 'x', kind = 'active', interval = 1,
        onTick = function() error('tick') end, onRemove = record})
    clock:advance()
    eq(buff:isActive(), false); eq(table.concat(reasons, ','), 'error'); eq(#messages, 1)
    local plain = quiet(clock)
    local failed = plain:apply(u, {id = 'y', kind = 'active', onApply = function() error('apply') end, onRemove = record})
    eq(failed:isActive(), false); eq(table.concat(reasons, ','), 'error,error'); eq(#PRINTED, 1)
    assert(PRINTED[1]:find('[systems] Buff callback failed:', 1, true), PRINTED[1])
end)

test('lookups, replace and one definition per key', function()
    local clock = Scheduler.new(1)
    local buffs, u, other = quiet(clock), newUnit(), newUnit()
    local sunder = {id = 'sunder', kind = 'active', stacking = 'stack', maxStacks = 5}
    buffs:apply(u, sunder, 'a'); buffs:apply(u, sunder, 'a'); buffs:apply(u, sunder, 'b')
    eq(buffs:stacks(u, 'sunder'), 3); eq(buffs:has(u, 'sunder', 'b'), true)
    eq(buffs:get(u, 'sunder'):getSource(), 'a'); eq(buffs:get(u, 'sunder'):getId(), 'sunder')
    eq(buffs:has(other, 'sunder'), false); eq(buffs:get(u, 'sunder'):getUnit(), u)
    eq(buffs:getScheduler(), clock)
    local reasons = {}
    local shield = {id = 'shield', kind = 'active', stacking = 'replace',
        onRemove = function(_, reason) reasons[#reasons + 1] = reason end}
    local first = buffs:apply(u, shield)
    local second = buffs:apply(u, shield)
    eq(first:isActive(), false); eq(second:isActive(), true); eq(reasons[1], 'replaced')
    eq(second:getDefinition(), shield)
    failsAt(function() buffs:apply(u, {id = 'shield', kind = 'active'}) end,
        'BuffStore.apply: use the same definition for a given id and source')
    buffs:clearSource('a')
    eq(buffs:has(u, 'sunder', 'a'), false); eq(buffs:has(u, 'sunder', 'b'), true)
end)

test('the poll clears removed, disposed and dead units; passive buffs survive death', function()
    native('GetUnitTypeId', function(raw) return raw.gone and 0 or 1 end)
    native('UnitAlive', function(raw) return not raw.dead end)
    native('RemoveUnit', function() end)
    local clock, reasons = Scheduler.new(0.25), {}
    local function track(name)
        return {id = name, kind = 'active', onRemove = function(_, reason) reasons[#reasons + 1] = name .. ':' .. reason end}
    end
    local gone, dead, disposed, fine = newUnit(), newUnit(), newUnit(), newUnit()
    local buffs = BuffStore.new(clock)
    buffs:apply(gone, track('gone')); buffs:apply(dead, track('dead'))
    buffs:apply(dead, {id = 'talent', kind = 'passive',
        onRemove = function(_, reason) reasons[#reasons + 1] = 'talent:' .. reason end})
    buffs:apply(disposed, track('disposed')); buffs:apply(fine, track('fine'))
    gone.handle.gone = true; dead.handle.dead = true
    disposed:remove()
    resetCalls()
    clock:advance()
    eq(table.concat(reasons, ','), 'gone:removed,dead:death,disposed:removed')
    eq(buffs:has(dead, 'talent'), true); eq(buffs:has(fine, 'fine'), true)
    eq(callCount('GetUnitTypeId'), 3)
    buffs:dispose()
    eq(table.concat(reasons, ','), 'gone:removed,dead:death,disposed:removed,talent:disposed,fine:disposed')
    eq(clock:getPending(), 0)
end)

test('arguments are checked at the caller', function()
    local clock = Scheduler.new(1)
    failsAt(function() BuffStore.new({}) end, 'BuffStore.new: expected Scheduler')
    failsAt(function() BuffStore.new(clock, 5) end, 'BuffStore.new: expected an options table')
    failsAt(function() BuffStore.new(clock, {pollInterval = 0}) end,
        'BuffStore.new: expected a finite positive poll interval')
    failsAt(function() BuffStore.new(clock, {onError = 5}) end, 'BuffStore.new: expected a callback function')
    local buffs, u = quiet(clock), newUnit()
    failsAt(function() buffs:apply({}, {id = 'x', kind = 'active'}) end, 'BuffStore.apply: expected Unit')
    failsAt(function() buffs:apply(u, 5) end, 'BuffStore.apply: expected a buff definition table')
    local bad = {
        {'id', {id = '', kind = 'active'}}, {'kind', {id = 'x', kind = 'weird'}},
        {'stacking', {id = 'x', kind = 'active', stacking = 'pile'}},
        {'maxStacks', {id = 'x', kind = 'active', maxStacks = 0}},
        {'duration', {id = 'x', kind = 'active', duration = -1}},
        {'interval', {id = 'x', kind = 'active', interval = 0}},
        {'removeOnDeath', {id = 'x', kind = 'active', removeOnDeath = 'yes'}},
        {'onTick', {id = 'x', kind = 'active', onTick = 5}},
    }
    for _, case in ipairs(bad) do
        failsAt(function() buffs:apply(u, case[2]) end, 'BuffStore.apply: expected a buff definition: ' .. case[1])
    end
    failsAt(function() buffs:clearUnit(u, 'gone') end, 'BuffStore.clearUnit: expected a removal reason')
    local buff = buffs:apply(u, {id = 'x', kind = 'active'})
    failsAt(function() buff:remove('bad') end, 'Buff.remove: expected a removal reason')
    failsAt(function() buff:own(5) end, 'Buff.own: expected a callback function')
    failsAt(function() buff.getStacks({}) end, 'Buff.getStacks: expected Buff')
    eq(#buffs:list(u), 1)
end)
```

`tests/suites.lua`: add `'buffs'` after `'time'`.

- [ ] **Step 2:** `yue -e tests/run.lua buffs` → `buffs: ERROR …module 'systems.buffs' not found`.

- [ ] **Step 3: Implement** — `src/systems/buffs.lua`:

```lua
local Callback = require('systems.internal.callback')
local Check = require('systems.internal.check')
local Ordered = require('systems.internal.ordered')
local Scheduler = require('systems.scheduler')
local Unit = require('wrappers.unit')

---Script buffs on Units: stacking, expiry, periodic ticks and owned effects that are released exactly once. The store
---polls its units and clears buffs from removed and dead ones.
---@class MoonwellSystems.BuffStore
---@field package clock MoonwellSystems.Scheduler
---@field package onError fun(message: string)?
---@field package units MoonwellSystems.Ordered Unit -> MoonwellSystems.Buff[], in first-buffed order.
---@field package disposed boolean
---@field package stopPoll fun()
local BuffStore = {}
BuffStore.__index = BuffStore

---@alias MoonwellSystems.BuffRemoval 'expired'|'dispelled'|'replaced'|'death'|'removed'|'source-lost'|'disposed'|'error'

---@class MoonwellSystems.BuffDefinition
---@field id string One instance per (unit, id, source).
---@field kind 'active'|'passive'|'aura'
---@field stacking ('refresh'|'replace'|'stack'|'independent')? Default 'refresh'.
---@field maxStacks integer? Default 1.
---@field duration number? Seconds; nil for a permanent buff.
---@field removeOnDeath boolean? Default true, except passive buffs.
---@field interval number? Seconds between onTick calls.
---@field onApply fun(buff: MoonwellSystems.Buff)?
---@field onStacks fun(buff: MoonwellSystems.Buff, previous: integer)?
---@field onTick fun(buff: MoonwellSystems.Buff)?
---@field onRemove fun(buff: MoonwellSystems.Buff, reason: MoonwellSystems.BuffRemoval)?

---@class MoonwellSystems.BuffStoreOptions
---@field onError fun(message: string)? Receives callback and release failures; default prints them.
---@field pollInterval number? Seconds between checks for removed and dead units; default 0.25.

---A buff on one unit. `data` is free for the buff's own state.
---@class MoonwellSystems.Buff
---@field data table
---@field package store MoonwellSystems.BuffStore
---@field package unit MoonwellWrappers.Unit
---@field package definition MoonwellSystems.BuffDefinition
---@field package source any
---@field package layers {cancel: fun()?, expires: integer?}[]
---@field package releases fun()[]
---@field package sharedCancel fun()?
---@field package sharedExpires integer?
---@field package tickCancel fun()?
---@field package live boolean
local Buff = {}
Buff.__index = Buff

local KINDS = {active = true, passive = true, aura = true}
local STACKING = {refresh = true, replace = true, stack = true, independent = true}
local REASONS = {expired = true, dispelled = true, replaced = true, death = true, removed = true,
    ['source-lost'] = true, disposed = true, error = true}
local CALLBACKS = {'onApply', 'onStacks', 'onTick', 'onRemove'}

local function positive(value) return Check.finite(value) and value > 0 end

---@return string? field The first invalid field, or nil.
local function invalid(definition)
    if type(definition.id) ~= 'string' or definition.id == '' then return 'id' end
    if not KINDS[definition.kind] then return 'kind' end
    if definition.stacking ~= nil and not STACKING[definition.stacking] then return 'stacking' end
    local cap = definition.maxStacks
    if cap ~= nil and not (math.type(cap) ~= nil and cap >= 1 and math.floor(cap) == cap) then return 'maxStacks' end
    if definition.duration ~= nil and not positive(definition.duration) then return 'duration' end
    if definition.interval ~= nil and not positive(definition.interval) then return 'interval' end
    if definition.removeOnDeath ~= nil and type(definition.removeOnDeath) ~= 'boolean' then return 'removeOnDeath' end
    for _, name in ipairs(CALLBACKS) do
        if definition[name] ~= nil and type(definition[name]) ~= 'function' then return name end
    end
    return nil
end

local function removedOnDeath(definition)
    if definition.removeOnDeath ~= nil then return definition.removeOnDeath end
    return definition.kind ~= 'passive'
end

local function detach(buff)
    local units = buff.store.units
    local list = units:get(buff.unit)
    if not list then return end
    for index = #list, 1, -1 do
        if list[index] == buff then table.remove(list, index) end
    end
    if #list == 0 then units:delete(buff.unit) end
end

---Ends a buff: cancels its timers, runs its releases in reverse, then onRemove. Idempotent.
local function finish(buff, reason)
    if not buff.live then return end
    buff.live = false
    detach(buff)
    if buff.sharedCancel then buff.sharedCancel(); buff.sharedCancel = nil end
    if buff.tickCancel then buff.tickCancel(); buff.tickCancel = nil end
    for _, layer in ipairs(buff.layers) do if layer.cancel then layer.cancel() end end
    buff.layers = {}
    local releases, onError = buff.releases, buff.store.onError
    buff.releases = {}
    for index = #releases, 1, -1 do Callback.call('Buff release', onError, releases[index]) end
    local onRemove = buff.definition.onRemove
    if onRemove then Callback.call('Buff release', onError, onRemove, buff, reason) end
end

---Runs a definition callback; a failure removes the buff with reason 'error'.
local function run(buff, callback, ...)
    if callback and buff.live and not Callback.call('Buff callback', buff.store.onError, callback, buff, ...) then
        finish(buff, 'error')
    end
end

local function addStack(buff)
    if not buff.live then return end
    local definition, clock = buff.definition, buff.store.clock
    local policy, previous = definition.stacking or 'refresh', #buff.layers
    local duration = definition.duration
    if previous == 0 or ((policy == 'stack' or policy == 'independent') and previous < (definition.maxStacks or 1)) then
        local layer = {}
        buff.layers[#buff.layers + 1] = layer
        if policy == 'independent' and duration then
            layer.expires = clock:getTick() + clock:ticks(duration)
            layer.cancel = clock:after(duration, function()
                if not buff.live then return end
                local before = #buff.layers
                for index = #buff.layers, 1, -1 do
                    if buff.layers[index] == layer then table.remove(buff.layers, index) end
                end
                if #buff.layers == 0 then finish(buff, 'expired') else run(buff, definition.onStacks, before) end
            end)
        end
    end
    -- A capped independent application extends nothing.
    if policy ~= 'independent' and duration then
        if buff.sharedCancel then buff.sharedCancel() end
        buff.sharedExpires = clock:getTick() + clock:ticks(duration)
        buff.sharedCancel = clock:after(duration, function() finish(buff, 'expired') end)
    end
    if previous > 0 and previous ~= #buff.layers then run(buff, definition.onStacks, previous) end
end

---Ticking starts before the first stack's expiry timer, so on a shared deadline the tick runs first.
local function startTicking(buff)
    local definition = buff.definition
    if not (definition.interval and definition.onTick) then return end
    buff.tickCancel = buff.store.clock:every(definition.interval, function() run(buff, definition.onTick) end)
end

local function find(store, unit, id, source)
    for _, buff in ipairs(store.units:get(unit) or {}) do
        if buff.definition.id == id and buff.source == source then return buff end
    end
    return nil
end

local function clear(store, unit, reason)
    local list = store.units:get(unit)
    if not list then return end
    for _, buff in ipairs(table.move(list, 1, #list, 1, {})) do
        if reason ~= 'death' or removedOnDeath(buff.definition) then finish(buff, reason) end
    end
end

local function poll(store)
    for _, unit in ipairs(store.units:keys()) do
        if store.units:has(unit) then
            Callback.call('Buff poll', store.onError, function()
                if unit:isDisposed() or not unit:exists() then
                    clear(store, unit, 'removed')
                elseif not unit:isAlive() then
                    clear(store, unit, 'death')
                end
            end)
        end
    end
end

---@param clock MoonwellSystems.Scheduler Drives expiry, ticks and the poll.
---@param options MoonwellSystems.BuffStoreOptions?
---@return MoonwellSystems.BuffStore
function BuffStore.new(clock, options)
    Check.receiver(clock, Scheduler, 'Scheduler', 'BuffStore.new')
    if options ~= nil and type(options) ~= 'table' then error('[systems] BuffStore.new: expected an options table', 2) end
    options = options or {}
    Callback.optional(options.onError, 'BuffStore.new')
    local interval = options.pollInterval or 0.25
    if not positive(interval) then error('[systems] BuffStore.new: expected a finite positive poll interval', 2) end
    local store = setmetatable({clock = clock, onError = options.onError, units = Ordered.new(), disposed = false},
        BuffStore)
    store.stopPoll = clock:every(interval, function() poll(store) end)
    return store
end

---Applies a buff, or applies it again according to its stacking policy.
---@param unit MoonwellWrappers.Unit
---@param definition MoonwellSystems.BuffDefinition
---@param source any? Who applied it; buffs with the same id and different sources are separate. Default nil.
---@return MoonwellSystems.Buff
function BuffStore:apply(unit, definition, source)
    local store = Check.receiver(self, BuffStore, 'BuffStore', 'BuffStore.apply')
    if store.disposed then error('[systems] BuffStore.apply: the store is disposed', 2) end
    Check.receiver(unit, Unit, 'Unit', 'BuffStore.apply')
    if type(definition) ~= 'table' then error('[systems] BuffStore.apply: expected a buff definition table', 2) end
    local field = invalid(definition)
    if field then error('[systems] BuffStore.apply: expected a buff definition: ' .. field, 2) end
    local existing = find(store, unit, definition.id, source)
    if existing then
        if existing.definition ~= definition then
            error('[systems] BuffStore.apply: use the same definition for a given id and source', 2)
        end
        if (definition.stacking or 'refresh') ~= 'replace' then
            addStack(existing)
            return existing
        end
        finish(existing, 'replaced')
        -- onRemove may have applied a replacement; never create two instances for one key.
        local replacement = find(store, unit, definition.id, source)
        if replacement then return replacement end
        if store.disposed then error('[systems] BuffStore.apply: the store is disposed', 2) end
    end
    ---@type MoonwellSystems.Buff
    local buff = setmetatable({store = store, unit = unit, definition = definition, source = source, layers = {},
        releases = {}, live = true, data = {}}, Buff)
    local list = store.units:get(unit)
    if list then list[#list + 1] = buff else store.units:set(unit, {buff}) end
    startTicking(buff)
    addStack(buff)
    run(buff, definition.onApply)
    return buff
end

---The buff with this id from `source`; with source nil, the first with this id from any source.
---@return MoonwellSystems.Buff?
function BuffStore:get(unit, id, source)
    local store = Check.receiver(self, BuffStore, 'BuffStore', 'BuffStore.get')
    for _, buff in ipairs(store.units:get(unit) or {}) do
        if buff.definition.id == id and (source == nil or buff.source == source) then return buff end
    end
    return nil
end

---@return boolean
function BuffStore:has(unit, id, source)
    Check.receiver(self, BuffStore, 'BuffStore', 'BuffStore.has')
    return self:get(unit, id, source) ~= nil
end

---Stacks of this id summed over every source.
---@return integer
function BuffStore:stacks(unit, id)
    local store = Check.receiver(self, BuffStore, 'BuffStore', 'BuffStore.stacks')
    local total = 0
    for _, buff in ipairs(store.units:get(unit) or {}) do
        if buff.definition.id == id then total = total + #buff.layers end
    end
    return total
end

---The unit's buffs in application order, as a new array.
---@return MoonwellSystems.Buff[]
function BuffStore:list(unit)
    local list = Check.receiver(self, BuffStore, 'BuffStore', 'BuffStore.list').units:get(unit) or {}
    return table.move(list, 1, #list, 1, {})
end

---Removes the unit's buffs. With reason 'death', buffs that survive death are kept.
---@param reason MoonwellSystems.BuffRemoval? Default 'removed'.
function BuffStore:clearUnit(unit, reason)
    local store = Check.receiver(self, BuffStore, 'BuffStore', 'BuffStore.clearUnit')
    if reason == nil then reason = 'removed' end
    if not REASONS[reason] then error('[systems] BuffStore.clearUnit: expected a removal reason', 2) end
    clear(store, unit, reason)
end

---Removes every buff applied by `source`, with reason 'source-lost'.
function BuffStore:clearSource(source)
    local store = Check.receiver(self, BuffStore, 'BuffStore', 'BuffStore.clearSource')
    local matched = {}
    store.units:each(function(_, list)
        for _, buff in ipairs(list) do if buff.source == source then matched[#matched + 1] = buff end end
    end)
    for _, buff in ipairs(matched) do finish(buff, 'source-lost') end
end

---@return MoonwellSystems.Scheduler
function BuffStore:getScheduler() return Check.receiver(self, BuffStore, 'BuffStore', 'BuffStore.getScheduler').clock end

---Stops the poll and removes every buff with reason 'disposed'. Applying afterwards raises. Idempotent.
function BuffStore:dispose()
    local store = Check.receiver(self, BuffStore, 'BuffStore', 'BuffStore.dispose')
    if store.disposed then return end
    store.disposed = true
    store.stopPoll()
    local all = {}
    store.units:each(function(_, list) for _, buff in ipairs(list) do all[#all + 1] = buff end end)
    for _, buff in ipairs(all) do finish(buff, 'disposed') end
end

---@return MoonwellWrappers.Unit
function Buff:getUnit() return Check.receiver(self, Buff, 'Buff', 'Buff.getUnit').unit end
---@return any
function Buff:getSource() return Check.receiver(self, Buff, 'Buff', 'Buff.getSource').source end
---@return string
function Buff:getId() return Check.receiver(self, Buff, 'Buff', 'Buff.getId').definition.id end
---@return MoonwellSystems.BuffDefinition
function Buff:getDefinition() return Check.receiver(self, Buff, 'Buff', 'Buff.getDefinition').definition end
---False once the buff has ended.
---@return boolean
function Buff:isActive() return Check.receiver(self, Buff, 'Buff', 'Buff.isActive').live end
---@return integer
function Buff:getStacks() return #Check.receiver(self, Buff, 'Buff', 'Buff.getStacks').layers end

---Seconds until the buff (or its last independent stack) expires: 0 once ended, nil if permanent.
---@return number?
function Buff:getRemaining()
    local buff = Check.receiver(self, Buff, 'Buff', 'Buff.getRemaining')
    if not buff.live then return 0 end
    local expires = buff.sharedExpires
    for _, layer in ipairs(buff.layers) do
        if layer.expires and (expires == nil or layer.expires > expires) then expires = layer.expires end
    end
    if expires == nil then return nil end
    local clock = buff.store.clock
    return math.max(0, expires - clock:getTick()) * clock:getStep()
end

---Registers the inverse of a change the buff made. Runs at once if the buff has ended.
---@param release fun()
function Buff:own(release)
    local buff = Check.receiver(self, Buff, 'Buff', 'Buff.own')
    Callback.check(release, 'Buff.own')
    if buff.live then
        buff.releases[#buff.releases + 1] = release
    else
        Callback.call('Buff release', buff.store.onError, release)
    end
end

---Ends the buff: timers, releases in reverse, then onRemove. Idempotent.
---@param reason MoonwellSystems.BuffRemoval? Default 'dispelled'.
function Buff:remove(reason)
    local buff = Check.receiver(self, Buff, 'Buff', 'Buff.remove')
    if reason == nil then reason = 'dispelled' end
    if not REASONS[reason] then error('[systems] Buff.remove: expected a removal reason', 2) end
    finish(buff, reason)
end

return BuffStore
```

- [ ] **Step 4:** `yue -e tests/run.lua; echo "exit $?"` → `buffs: SUITE PASSED: 9 tests`, `exit 0`.

- [ ] **Step 5: Commit** `src/systems/buffs.lua tests/buffs.lua tests/suites.lua` —
  `feat: systems.buffs, script buffs on Units with automatic pruning`.

---

### Task 3: `systems.aura`

**Files:** Create `src/systems/aura.lua`, `tests/aura.lua`; modify `tests/suites.lua`.

**Interfaces:** Consumes `BuffStore` (class, `:apply`, `:getScheduler`), `Buff:isActive/remove`, `Ordered`. Produces
`Aura.new(store, definition, source, query, onError?)`, `:start(interval?) -> self`, `:update()`, `:dispose()`.

- [ ] **Step 1: Failing tests** — `tests/aura.lua`:

```lua
local Scheduler = require('systems.scheduler')
local BuffStore = require('systems.buffs')
local Aura = require('systems.aura')
local Unit = require('wrappers.unit')

local function newUnit() return Unit.fromHandle({}) end

test('emitters own independent contributions and recover after dispel', function()
    local clock = Scheduler.new(1)
    local buffs, u = BuffStore.new(clock, {pollInterval = 100}), newUnit()
    local armor = {id = 'armor', kind = 'aura'}
    local targets = {u}
    local a = Aura.new(buffs, armor, 'a', function() return targets end)
    local b = Aura.new(buffs, armor, 'b', function() return {u} end)
    a:update(); b:update(); eq(#buffs:list(u), 2)
    a:dispose(); eq(#buffs:list(u), 1)
    buffs:clearUnit(u, 'dispelled'); b:update(); eq(#buffs:list(u), 1)
    targets = {}; b:dispose(); eq(#buffs:list(u), 0)
end)

test('start updates at once and on its interval; dispose stops the timer', function()
    local clock = Scheduler.new(1)
    local buffs, u, v = BuffStore.new(clock, {pollInterval = 100}), newUnit(), newUnit()
    local targets = {u}
    local aura = Aura.new(buffs, {id = 'a', kind = 'aura'}, 'src', function() return targets end)
    eq(aura:start(2), aura)
    eq(buffs:has(u, 'a'), true)
    targets = {v}
    clock:advance(); eq(buffs:has(v, 'a'), false)
    clock:advance(); eq(buffs:has(v, 'a'), true); eq(buffs:has(u, 'a'), false)
    aura:dispose(); aura:dispose()
    eq(clock:getPending(), 1); eq(buffs:has(v, 'a'), false)
    failsAt(function() aura:start() end, 'Aura.start: the aura is disposed')
end)

test('members follow the query order; a failing query is reported and the timer goes on', function()
    local clock, messages = Scheduler.new(1), {}
    local buffs, u, v = BuffStore.new(clock, {pollInterval = 100}), newUnit(), newUnit()
    local order, broken = {}, false
    local definition = {id = 'ordered', kind = 'aura',
        onApply = function(buff) order[#order + 1] = buff:getUnit() == u and 'u' or 'v' end}
    local aura = Aura.new(buffs, definition, 'src', function()
        if broken then error('query probe') end
        return {v, u}
    end, function(message) messages[#messages + 1] = message end)
    aura:start(1)
    eq(table.concat(order, ','), 'v,u')
    broken = true
    clock:advance()
    eq(#messages, 1); assert(messages[1]:find('query probe', 1, true), messages[1])
    eq(clock:getPending(), 2) -- the poll and the aura
    broken = false
    buffs:clearUnit(u, 'dispelled')
    clock:advance()
    eq(table.concat(order, ','), 'v,u,u')
end)

test('arguments are checked at the caller', function()
    local clock = Scheduler.new(1)
    local buffs = BuffStore.new(clock, {pollInterval = 100})
    failsAt(function() Aura.new({}, {id = 'a', kind = 'aura'}, nil, print) end, 'Aura.new: expected BuffStore')
    failsAt(function() Aura.new(buffs, {id = 'a', kind = 'active'}, nil, print) end,
        'Aura.new: expected an aura buff definition')
    failsAt(function() Aura.new(buffs, {id = 'a', kind = 'aura'}, nil, 5) end, 'Aura.new: expected a callback function')
    failsAt(function() Aura.new(buffs, {id = 'a', kind = 'aura'}, nil, print, 5) end,
        'Aura.new: expected a callback function')
    local aura = Aura.new(buffs, {id = 'a', kind = 'aura'}, nil, function() return {} end)
    failsAt(function() aura:start(0) end, 'Aura.start: expected a finite positive interval')
    failsAt(function() Aura.update({}) end, 'Aura.update: expected Aura')
end)
```

`tests/suites.lua`: add `'aura'` after `'buffs'`.

- [ ] **Step 2:** `yue -e tests/run.lua aura` → `aura: ERROR …module 'systems.aura' not found`.

- [ ] **Step 3: Implement** — `src/systems/aura.lua`:

```lua
local Callback = require('systems.internal.callback')
local Check = require('systems.internal.check')
local Ordered = require('systems.internal.ordered')
local BuffStore = require('systems.buffs')

---Keeps an aura buff on the Units a query returns. Each emitter (source) owns its instances, so two auras with the
---same definition never remove each other's buffs. The query decides range, team and visibility, and should return
---Units in the engine's enumeration order (for example group:getUnits()), which is the same on every machine.
---@class MoonwellSystems.Aura
---@field package store MoonwellSystems.BuffStore
---@field package definition MoonwellSystems.BuffDefinition
---@field package source any
---@field package query fun(): MoonwellWrappers.Unit[]
---@field package onError fun(message: string)?
---@field package members MoonwellSystems.Ordered Unit -> MoonwellSystems.Buff
---@field package stop fun()?
---@field package disposed boolean
local Aura = {}
Aura.__index = Aura

---@param store MoonwellSystems.BuffStore
---@param definition MoonwellSystems.BuffDefinition kind 'aura'.
---@param source any The emitter.
---@param query fun(): MoonwellWrappers.Unit[]
---@param onError fun(message: string)? Receives query and apply failures; default prints them.
---@return MoonwellSystems.Aura
function Aura.new(store, definition, source, query, onError)
    Check.receiver(store, BuffStore, 'BuffStore', 'Aura.new')
    if type(definition) ~= 'table' or definition.kind ~= 'aura' then
        error('[systems] Aura.new: expected an aura buff definition', 2)
    end
    Callback.check(query, 'Aura.new')
    Callback.optional(onError, 'Aura.new')
    return setmetatable({store = store, definition = definition, source = source, query = query, onError = onError,
        members = Ordered.new(), disposed = false}, Aura)
end

---Reconciles once: removes the buff from Units the query no longer returns and applies it to new ones.
function Aura:update()
    local aura = Check.receiver(self, Aura, 'Aura', 'Aura.update')
    if aura.disposed then return end
    local wanted
    if not Callback.call('Aura query', aura.onError, function() wanted = aura.query() end) then return end
    if type(wanted) ~= 'table' then
        Callback.call('Aura query', aura.onError, error, 'the query must return an array of Units', 0)
        return
    end
    local present = {}
    for _, unit in ipairs(wanted) do present[unit] = true end
    aura.members:each(function(unit, buff)
        if not present[unit] then
            aura.members:delete(unit)
            buff:remove('source-lost')
        end
    end)
    for _, unit in ipairs(wanted) do
        if aura.disposed then break end
        local current = aura.members:get(unit)
        if not (current and current:isActive()) then
            local applied
            if Callback.call('Aura apply', aura.onError, function()
                applied = aura.store:apply(unit, aura.definition, aura.source)
            end) then
                if aura.disposed then applied:remove('source-lost') else aura.members:set(unit, applied) end
            end
        end
    end
end

---Updates now and then every `interval` seconds on the store's scheduler. Starting again restarts the timer.
---@param interval number? Default 0.5.
---@return MoonwellSystems.Aura
function Aura:start(interval)
    local aura = Check.receiver(self, Aura, 'Aura', 'Aura.start')
    if aura.disposed then error('[systems] Aura.start: the aura is disposed', 2) end
    if interval == nil then interval = 0.5 end
    if not (Check.finite(interval) and interval > 0) then
        error('[systems] Aura.start: expected a finite positive interval', 2)
    end
    if aura.stop then aura.stop() end
    aura:update()
    aura.stop = aura.store:getScheduler():every(interval, function() aura:update() end)
    return aura
end

---Stops the timer and removes every buff this aura applied ('source-lost'). Idempotent.
function Aura:dispose()
    local aura = Check.receiver(self, Aura, 'Aura', 'Aura.dispose')
    if aura.disposed then return end
    aura.disposed = true
    if aura.stop then aura.stop(); aura.stop = nil end
    local members = aura.members
    aura.members = Ordered.new()
    members:each(function(_, buff) buff:remove('source-lost') end)
end

return Aura
```

- [ ] **Step 4:** `yue -e tests/run.lua; echo "exit $?"` → `aura: SUITE PASSED: 4 tests`, `exit 0`.

- [ ] **Step 5: Commit** `src/systems/aura.lua tests/aura.lua tests/suites.lua` — `feat: systems.aura`.

---

### Task 4: `systems.dummy`

**Files:** Create `src/systems/dummy.lua`, `tests/dummy.lua`; modify `tests/suites.lua`.

**Interfaces:** Consumes `Scheduler`, `Ordered`, `wrappers.unit` (`Unit.create`, `:addAbility`, `:getAbilityLevel`,
`:setAbilityLevel`, `:setInvulnerable`, `:setPathing`, `:getMaxMana`, `:setMana`, the six `issue…Order…` methods,
`:remove`, `:isDisposed`), `wrappers.player` (class). Produces `Dummies.new(clock, options?)`, `:cast(request)`,
`:isDummy`, `:sourceOf`, `:getCount`, `:dispose`; `DummyLease` `getUnit`, `getSource`, `isActive`,
`isOrderAccepted`, `dispose`.

- [ ] **Step 1: Failing tests** — `tests/dummy.lua`:

```lua
bj_MAX_PLAYER_SLOTS = 28
UNIT_STATE_MANA = {}
local PLAYER_RAW, LOCUST, MISSING = {}, 1097625443, 666
local accept = true
native('Player', function() return PLAYER_RAW end)
native('CreateUnit', function(owner, typeId, x, y, facing)
    return {owner = owner, typeId = typeId, x = x, y = y, facing = facing, abilities = {}}
end)
native('RemoveUnit', function(raw) raw.removed = true end)
native('UnitAddAbility', function(raw, id)
    if id == MISSING then return false end
    raw.abilities[id] = 1
    return true
end)
native('GetUnitAbilityLevel', function(raw, id) return raw.abilities[id] or 0 end)
native('SetUnitAbilityLevel', function(raw, id, level) raw.abilities[id] = level; return level end)
native('SetUnitInvulnerable', function(raw, flag) raw.invulnerable = flag end)
native('SetUnitPathing', function(raw, flag) raw.pathing = flag end)
native('BlzGetUnitMaxMana', function() return 300 end)
native('SetUnitState', function(raw, _, value) raw.mana = value end)
for _, name in ipairs({'IssueImmediateOrder', 'IssuePointOrder', 'IssueTargetOrder', 'IssueImmediateOrderById',
    'IssuePointOrderById', 'IssueTargetOrderById'}) do
    native(name, function(raw, order, a, b) raw.issued = {name, order, a, b}; return accept end)
end
local Scheduler = require('systems.scheduler')
local Dummies = require('systems.dummy')
local Unit = require('wrappers.unit')
local Player = require('wrappers.player')

local function request(fields)
    local result = {owner = Player.fromIndex(0), typeId = 1, x = 0, y = 0, ability = 2, order = 'slow', duration = 2}
    for key, value in pairs(fields or {}) do result[key] = value end
    return result
end

test('a lease lives through its duration and cleans up once; the dummy is configured', function()
    local clock = Scheduler.new(1)
    local dummies = Dummies.new(clock)
    local lease = dummies:cast(request())
    local raw = lease:getUnit().handle
    eq(lease:isOrderAccepted(), true); eq(dummies:getCount(), 1); eq(lease:isActive(), true)
    eq(raw.abilities[LOCUST], 1); eq(raw.invulnerable, true); eq(raw.pathing, false)
    eq(raw.abilities[2], 1); eq(raw.mana, 300); eq(raw.facing, 0)
    eq(raw.issued[1], 'IssueImmediateOrder'); eq(raw.issued[2], 'slow')
    clock:advance(); eq(dummies:getCount(), 1)
    clock:advance(); eq(dummies:getCount(), 0); eq(raw.removed, true); eq(lease:isActive(), false)
    lease:dispose()
    eq(callCount('RemoveUnit'), 1); eq(clock:getPending(), 0)
end)

test('a missing ability and a rejected order remove the dummy at once', function()
    local clock = Scheduler.new(1)
    local dummies = Dummies.new(clock)
    failsAt(function() dummies:cast(request({ability = MISSING})) end,
        'Dummies.cast: the dummy cannot get ability 666')
    eq(dummies:getCount(), 0); eq(callCount('RemoveUnit'), 1)
    accept = false
    local lease = dummies:cast(request())
    accept = true
    eq(lease:isOrderAccepted(), false); eq(lease:isActive(), false)
    eq(dummies:getCount(), 0); eq(clock:getPending(), 0)
end)

test('dispose removes every dummy in cast order and refuses new casts', function()
    local clock = Scheduler.new(1)
    local dummies = Dummies.new(clock)
    local first = dummies:cast(request({duration = 4})):getUnit().handle
    local second = dummies:cast(request({duration = 4})):getUnit().handle
    dummies:dispose(); dummies:dispose()
    eq(dummies:getCount(), 0); eq(clock:getPending(), 0); eq(callCount('RemoveUnit'), 2)
    expectCall('RemoveUnit', second) -- the last removal was the second cast
    eq(first.removed, true)
    failsAt(function() dummies:cast(request()) end, 'Dummies.cast: the manager is disposed')
end)

test('dummies attribute to their caster only while leased', function()
    local clock = Scheduler.new(1)
    local dummies = Dummies.new(clock)
    local hero = Unit.fromHandle({})
    local lease = dummies:cast(request({source = hero}))
    local unit = lease:getUnit()
    eq(dummies:isDummy(unit), true); eq(dummies:sourceOf(unit), hero); eq(lease:getSource(), hero)
    eq(dummies:sourceOf(hero), nil); eq(dummies:isDummy(hero), false)
    clock:advance(); clock:advance()
    eq(dummies:isDummy(unit), false); eq(dummies:sourceOf(unit), nil)
end)

test('point, target, order ids, level and facing reach the natives', function()
    local clock = Scheduler.new(1)
    local dummies = Dummies.new(clock)
    local raw = dummies:cast(request({point = {x = 5, y = 6}, level = 3, facing = 90})):getUnit().handle
    eq(raw.issued[1], 'IssuePointOrder'); eq(raw.issued[3], 5); eq(raw.issued[4], 6)
    eq(raw.abilities[2], 3); eq(raw.facing, 90)
    local target = Unit.fromHandle({})
    raw = dummies:cast(request({target = target, order = 852075})):getUnit().handle
    eq(raw.issued[1], 'IssueTargetOrderById'); eq(raw.issued[2], 852075); eq(raw.issued[3], target.handle)
    raw = dummies:cast(request({order = 852075})):getUnit().handle
    eq(raw.issued[1], 'IssueImmediateOrderById')
    raw = dummies:cast(request({target = target})):getUnit().handle
    eq(raw.issued[1], 'IssueTargetOrder')
    raw = dummies:cast(request({point = {x = 1, y = 2}, order = 852075})):getUnit().handle
    eq(raw.issued[1], 'IssuePointOrderById')
end)

test('a dummy removed by other code is not removed again', function()
    local clock = Scheduler.new(1)
    local dummies = Dummies.new(clock)
    local lease = dummies:cast(request())
    lease:getUnit():remove()
    clock:advance(); clock:advance()
    eq(callCount('RemoveUnit'), 1); eq(dummies:getCount(), 0)
end)

test('arguments are checked at the caller', function()
    local clock = Scheduler.new(1)
    failsAt(function() Dummies.new({}) end, 'Dummies.new: expected Scheduler')
    failsAt(function() Dummies.new(clock, 5) end, 'Dummies.new: expected an options table')
    failsAt(function() Dummies.new(clock, {onError = 5}) end, 'Dummies.new: expected a callback function')
    local dummies = Dummies.new(clock)
    failsAt(function() dummies:cast(5) end, 'Dummies.cast: expected a cast request table')
    local bad = {
        {'owner', {owner = {}}}, {'typeId', {typeId = 1.5}}, {'x', {x = 0 / 0}}, {'y', {y = math.huge}},
        {'facing', {facing = 'north'}}, {'ability', {ability = '2'}}, {'level', {level = 0}},
        {'order', {order = ''}}, {'duration', {duration = 0}}, {'point', {point = {x = 1}}},
        {'target', {target = 5}}, {'target and point', {target = Unit.fromHandle({}), point = {x = 1, y = 1}}},
        {'source', {source = 'hero'}},
    }
    for _, case in ipairs(bad) do
        failsAt(function() dummies:cast(request(case[2])) end, 'Dummies.cast: expected a cast request: ' .. case[1])
    end
    eq(callCount('CreateUnit'), 0)
    failsAt(function() Dummies.getCount({}) end, 'Dummies.getCount: expected Dummies')
end)
```

`tests/suites.lua`: add `'dummy'` after `'aura'`.

- [ ] **Step 2:** `yue -e tests/run.lua dummy` → `dummy: ERROR …module 'systems.dummy' not found`.

- [ ] **Step 3: Implement** — `src/systems/dummy.lua`:

```lua
local Callback = require('systems.internal.callback')
local Check = require('systems.internal.check')
local Ordered = require('systems.internal.ordered')
local Scheduler = require('systems.scheduler')
local Unit = require('wrappers.unit')
local PlayerWrapper = require('wrappers.player')

---Fresh dummy casters: each cast creates a unit, configures it, orders the cast and removes it after `duration`.
---No pooling. The dummy unit type comes from the map's object data (README).
---@class MoonwellSystems.Dummies
---@field package clock MoonwellSystems.Scheduler
---@field package onError fun(message: string)?
---@field package leases MoonwellSystems.Ordered Unit -> MoonwellSystems.DummyLease, in cast order.
---@field package disposed boolean
local Dummies = {}
Dummies.__index = Dummies

---Ownership of one live dummy. Disposing it, or its duration running out, removes the unit.
---@class MoonwellSystems.DummyLease
---@field package manager MoonwellSystems.Dummies
---@field package unit MoonwellWrappers.Unit
---@field package source MoonwellWrappers.Unit?
---@field package accepted boolean
---@field package cancel fun()?
---@field package live boolean
local DummyLease = {}
DummyLease.__index = DummyLease

---@class MoonwellSystems.DummyCast
---@field owner MoonwellWrappers.Player
---@field typeId integer The dummy unit type.
---@field x number
---@field y number
---@field facing number? Default 0.
---@field ability integer
---@field level integer? Default 1.
---@field order string|integer An order string, or an order id.
---@field target MoonwellWrappers.Widget? A target widget; set target or point, not both.
---@field point {x: number, y: number}?
---@field duration number Seconds; cover cast point, channel time and projectile travel.
---@field source MoonwellWrappers.Unit? The real caster, for sourceOf.

local LOCUST = 1097625443 -- 'Aloc'

local function integer(value) return math.type(value) ~= nil and math.floor(value) == value end

---@return string? field The first invalid field, or nil.
local function invalid(request)
    if getmetatable(request.owner) ~= PlayerWrapper then return 'owner' end
    if not integer(request.typeId) then return 'typeId' end
    if not Check.finite(request.x) then return 'x' end
    if not Check.finite(request.y) then return 'y' end
    if request.facing ~= nil and not Check.finite(request.facing) then return 'facing' end
    if not integer(request.ability) then return 'ability' end
    if request.level ~= nil and not (integer(request.level) and request.level >= 1) then return 'level' end
    local order = request.order
    if not ((type(order) == 'string' and order ~= '') or math.type(order) == 'integer') then return 'order' end
    if not (Check.finite(request.duration) and request.duration > 0) then return 'duration' end
    local target, point = request.target, request.point
    if target ~= nil and point ~= nil then return 'target and point' end
    if target ~= nil and not (type(target) == 'table' and type(target.getLife) == 'function') then return 'target' end
    if point ~= nil and not (type(point) == 'table' and Check.finite(point.x) and Check.finite(point.y)) then
        return 'point'
    end
    if request.source ~= nil and getmetatable(request.source) ~= Unit then return 'source' end
    return nil
end

---@param unit MoonwellWrappers.Unit
---@param request MoonwellSystems.DummyCast
---@return boolean accepted
local function issue(unit, request)
    local order, target, point = request.order, request.target, request.point
    if math.type(order) == 'integer' then
        if target ~= nil then return unit:issueTargetOrderById(order, target) end
        if point ~= nil then return unit:issuePointOrderById(order, point.x, point.y) end
        return unit:issueOrderById(order)
    end
    if target ~= nil then return unit:issueTargetOrder(order, target) end
    if point ~= nil then return unit:issuePointOrder(order, point.x, point.y) end
    return unit:issueOrder(order)
end

---@param lease MoonwellSystems.DummyLease
local function release(lease)
    if not lease.live then return end
    lease.live = false
    if lease.cancel then lease.cancel(); lease.cancel = nil end
    lease.manager.leases:delete(lease.unit)
    local unit = lease.unit
    Callback.call('Dummy release', lease.manager.onError, function()
        if not unit:isDisposed() then unit:remove() end
    end)
end

---@param clock MoonwellSystems.Scheduler Schedules each dummy's removal.
---@param options {onError: fun(message: string)?}?
---@return MoonwellSystems.Dummies
function Dummies.new(clock, options)
    Check.receiver(clock, Scheduler, 'Scheduler', 'Dummies.new')
    if options ~= nil and type(options) ~= 'table' then error('[systems] Dummies.new: expected an options table', 2) end
    options = options or {}
    Callback.optional(options.onError, 'Dummies.new')
    return setmetatable({clock = clock, onError = options.onError, leases = Ordered.new(), disposed = false}, Dummies)
end

---Creates a dummy, configures it (Locust, invulnerable, no pathing, the ability, full mana), orders the cast and
---schedules its removal. A rejected order removes the dummy at once.
---@param request MoonwellSystems.DummyCast
---@return MoonwellSystems.DummyLease
function Dummies:cast(request)
    local dummies = Check.receiver(self, Dummies, 'Dummies', 'Dummies.cast')
    if dummies.disposed then error('[systems] Dummies.cast: the manager is disposed', 2) end
    if type(request) ~= 'table' then error('[systems] Dummies.cast: expected a cast request table', 2) end
    local field = invalid(request)
    if field then error('[systems] Dummies.cast: expected a cast request: ' .. field, 2) end
    local unit = Unit.create(request.owner, request.typeId, request.x, request.y, request.facing or 0)
    ---@type MoonwellSystems.DummyLease
    local lease = setmetatable({manager = dummies, unit = unit, source = request.source, accepted = false, live = true},
        DummyLease)
    dummies.leases:set(unit, lease)
    unit:addAbility(LOCUST)
    unit:setInvulnerable(true)
    unit:setPathing(false)
    if not unit:addAbility(request.ability) and unit:getAbilityLevel(request.ability) == 0 then
        release(lease)
        error('[systems] Dummies.cast: the dummy cannot get ability ' .. request.ability, 2)
    end
    unit:setAbilityLevel(request.ability, request.level or 1)
    unit:setMana(unit:getMaxMana())
    local issued, accepted = pcall(issue, unit, request)
    if not issued then
        release(lease)
        error(accepted, 0)
    end
    lease.accepted = accepted
    if accepted then
        lease.cancel = dummies.clock:after(request.duration, function() release(lease) end)
    else
        release(lease)
    end
    return lease
end

---True while `unit` is a live dummy of this manager.
---@return boolean
function Dummies:isDummy(unit) return Check.receiver(self, Dummies, 'Dummies', 'Dummies.isDummy').leases:has(unit) end

---The caster a live dummy acts for; nil for other units and for dummies cast without a source.
---@return MoonwellWrappers.Unit?
function Dummies:sourceOf(unit)
    local lease = Check.receiver(self, Dummies, 'Dummies', 'Dummies.sourceOf').leases:get(unit)
    return lease and lease.source
end

---@return integer
function Dummies:getCount() return Check.receiver(self, Dummies, 'Dummies', 'Dummies.getCount').leases:getSize() end

---Removes every live dummy in cast order. Casting afterwards raises. Idempotent.
function Dummies:dispose()
    local dummies = Check.receiver(self, Dummies, 'Dummies', 'Dummies.dispose')
    if dummies.disposed then return end
    dummies.disposed = true
    dummies.leases:each(function(_, lease) release(lease) end)
end

---@return MoonwellWrappers.Unit
function DummyLease:getUnit() return Check.receiver(self, DummyLease, 'DummyLease', 'DummyLease.getUnit').unit end
---@return MoonwellWrappers.Unit?
function DummyLease:getSource() return Check.receiver(self, DummyLease, 'DummyLease', 'DummyLease.getSource').source end
---False once the dummy has been removed.
---@return boolean
function DummyLease:isActive() return Check.receiver(self, DummyLease, 'DummyLease', 'DummyLease.isActive').live end
---Whether the game accepted the cast order.
---@return boolean
function DummyLease:isOrderAccepted()
    return Check.receiver(self, DummyLease, 'DummyLease', 'DummyLease.isOrderAccepted').accepted
end
---Removes the dummy now. Idempotent.
function DummyLease:dispose() release(Check.receiver(self, DummyLease, 'DummyLease', 'DummyLease.dispose')) end

return Dummies
```

- [ ] **Step 4:** `yue -e tests/run.lua; echo "exit $?"` → `dummy: SUITE PASSED: 7 tests`, `exit 0`.

- [ ] **Step 5: Commit** `src/systems/dummy.lua tests/dummy.lua tests/suites.lua` —
  `feat: systems.dummy, fresh dummy casters on the wrappers`.

---

### Task 5: Sweep, imports, fixtures and integration

**Files:** Modify `tests/blame.lua`, `tests/imports.lua`, `tests/editor-positive.lua`, `tests/editor-negative.lua`,
`tools/integration.lua`.

- [ ] **Step 1:** In `tests/blame.lua`, set `local modules = {'scheduler', 'signal', 'scope', 'time', 'buffs', 'aura',
  'dummy'}`. In `tests/imports.lua`, extend the module list of `importing every module calls no native` with
  `'buffs', 'aura', 'dummy'` (it then also loads `wrappers.unit`: replace its last line with
  `eq(package.loaded['wrappers.group'], nil)`), and append:

```lua
test('buffs and aura load no dummy module', function()
    require('systems.aura')
    eq(totalCalls(), 0)
    eq(package.loaded['systems.buffs'] ~= nil, true)
    eq(package.loaded['systems.dummy'], nil)
end)
```

Run `yue -e tests/run.lua; echo "exit $?"` → all suites pass. A misplaced error listed by the sweep is fixed in the
module (level or tail call).

- [ ] **Step 2: Fixtures.** Append to `tests/editor-positive.lua` before `print(scope:isActive())`:

```lua
local BuffStore = require('systems.buffs')
local Aura = require('systems.aura')
local Dummies = require('systems.dummy')
local Unit = require('wrappers.unit')
local PlayerWrapper = require('wrappers.player')
local owner = PlayerWrapper.fromIndex(0)
local hero = Unit.create(owner, 1215324524, 0, 0, 0)
local buffs = scope:add(BuffStore.new(clock, {pollInterval = 0.5}))
local slow = {id = 'slow', kind = 'active', stacking = 'stack', maxStacks = 3, duration = 5, interval = 1,
    onApply = function(buff) buff:own(function() print(buff:getUnit():getName()) end) end,
    onTick = function(buff) print(buff:getStacks(), buff:getRemaining()) end,
    onRemove = function(buff, reason) print(buff:getId(), reason) end}
local applied = buffs:apply(hero, slow, 'caster')
print(applied:isActive(), buffs:has(hero, 'slow'), buffs:stacks(hero, 'slow'), #buffs:list(hero))
local aura = scope:add(Aura.new(buffs, {id = 'devotion', kind = 'aura'}, hero, function() return {hero} end))
aura:start(0.5)
local dummies = scope:add(Dummies.new(clock))
local lease = dummies:cast({owner = owner, typeId = 1697656880, x = 0, y = 0, ability = 1095267426,
    order = 'thunderbolt', target = hero, duration = 2, source = hero})
print(lease:isOrderAccepted(), dummies:sourceOf(lease:getUnit()), dummies:getCount())
```

Rawcodes as integers (computed with a FourCC script): `1215324524` is `'Hpal'`, `1697656880` is `'e000'`,
`1095267426` is `'AHtb'`.

Append to `tests/editor-negative.lua` before `return true`:

```lua
local BuffStore = require('systems.buffs')
local Dummies = require('systems.dummy')
BuffStore.new(clock):apply(clock, {id = 'x', kind = 'active'}) -- EXPECT param-type-mismatch
Dummies.new(5) -- EXPECT param-type-mismatch
Dummies.new(clock):nonexistent() -- EXPECT undefined-field
```

- [ ] **Step 3: Integration.** In `tools/integration.lua`:
- `public` becomes `{'scheduler', 'signal', 'scope', 'time', 'buffs', 'aura', 'dummy'}`;
- each entry gains a `systems` set of other systems modules it may bundle, and the check becomes
  `if other ~= entry and bundles(bundle, 'systems.' .. other) ~= (entries[entry].systems[other] == true) then error(...)`;
  the existing four entries get `systems = {}` (scheduler: `{}`);
- new entries:

```lua
    buffs = {source = 'import "systems.buffs" as BuffStore\nimport "systems.scheduler" as Scheduler\n'
        .. 's = BuffStore.new Scheduler.new!\ns\\dispose!\n',
        systems = {scheduler = true}, wrappers = {unit = true, player = true, item = true, timer = true}},
    aura = {source = 'import "systems.aura" as Aura\nprint Aura\n',
        systems = {buffs = true, scheduler = true}, wrappers = {unit = true, player = true, item = true, timer = true}},
    dummy = {source = 'import "systems.dummy" as Dummies\nimport "systems.scheduler" as Scheduler\n'
        .. 'd = Dummies.new Scheduler.new!\nd\\dispose!\n',
        systems = {scheduler = true}, wrappers = {unit = true, player = true, item = true, timer = true}},
```

- the summary line becomes `'Moonwell: every entry point bundles only what it imports'`.

- [ ] **Step 4:** Run the three checks. Expected: all suites pass; the syntax check counts 30 files; integration
  passes with `9 intentional type errors`. If LuaLS reports a diagnostic in `src/systems`, fix the annotation.

- [ ] **Step 5: Commit** `tests/blame.lua tests/imports.lua tests/editor-positive.lua tests/editor-negative.lua
  tools/integration.lua` — `test: sweep, imports, fixtures and bundles for release 2`.

---

### Task 6: Docs, the dummy type and the gate

**Files:** Modify `README.md`, `CHANGELOG.md`, `CONTRIBUTING.md`, `AGENTS.md`, `examples/gate.yue`; modify (not under
git) `../wrappers-gate/objects/units.pkl`.

- [ ] **Step 1: The dummy type in the gate map.** Add to `../wrappers-gate/objects/units.pkl`, inside `units { }`, the
  spec §5.2 definition. Run `deno run -A ../moonwell/cli/src/main.ts objects:check` in `../wrappers-gate`. If a field is
  rejected, correct it here and in spec §5.2 (commit that spec change in `../moonwell`), and use the corrected
  definition in the README.

- [ ] **Step 2: The gate example.** In `examples/gate.yue`, add imports
  `import "systems.buffs" as BuffStore`, `import "systems.aura" as Aura`, `import "systems.dummy" as Dummies`, and,
  **above** `systemsGate` (a later definition would compile as an unknown global), this function:

```
-- Release 2: dummies, buffs, pruning and an aura, one step at a time. CONTRIBUTING lists every message.
releaseTwoGate = (owner) ->
  clock = Scheduler.new!
  clock\start!
  hostile = Player.fromIndex 12
  hero = Unit.create owner, $FourCC("Hpal"), -300, 0, 0
  bolted = Unit.create hostile, $FourCC("hfoo"), 0, 250, 270
  bolted\pause true
  dummies = Dummies.new clock
  lease = dummies\cast
    owner: owner, typeId: $FourCC("e000"), x: -300, y: 0, ability: $FourCC("AHtb"), order: "thunderbolt"
    target: bolted, duration: 2, source: hero
  dummy = lease\getUnit!
  print "Systems dummy cast accepted", lease\isOrderAccepted!, "isDummy", dummies\isDummy(dummy), "source is hero",
    dummies\sourceOf(dummy) == hero, "count", dummies\getCount!
  boltedLife = bolted\getLife!
  clock\after 2.5, ->
    print "Systems dummy gone: count", dummies\getCount!, "active", lease\isActive!, "life lost",
      boltedLife - bolted\getLife!
  buffs = BuffStore.new clock
  slowed = Unit.create owner, $FourCC("hfoo"), 200, -250, 90
  baseSpeed = slowed\getMoveSpeed!
  slow =
    id: "gate-slow"
    kind: "active"
    stacking: "stack"
    maxStacks: 3
    duration: 3
    interval: 1
    onApply: (buff) ->
      target = buff\getUnit!
      target\setVertexColor 80, 80, 255, 255
      target\setMoveSpeed baseSpeed * 0.5
      buff\own ->
        target\setVertexColor 255, 255, 255, 255
        target\setMoveSpeed baseSpeed
    onStacks: (buff, previous) -> print "Systems buff stacks", previous, "->", buff\getStacks!
    onTick: (buff) -> print "Systems buff tick: stacks", buff\getStacks!, "remaining", buff\getRemaining!
    onRemove: (buff, reason) ->
      print "Systems buff removed:", reason, "speed restored", buff\getUnit!\getMoveSpeed! == baseSpeed
  clock\after 3, -> buffs\apply slowed, slow
  clock\after 3.5, -> buffs\apply slowed, slow
  doomed = Unit.create hostile, $FourCC("hfoo"), 300, 350, 0
  vanished = Unit.create hostile, $FourCC("hfoo"), 400, 350, 0
  marked = (name) ->
    {id: "gate-mark-#{name}", kind: "active", onRemove: (buff, reason) -> print "Systems buff pruned:", name, reason}
  clock\after 7, ->
    buffs\apply doomed, marked "killed"
    buffs\apply doomed,
      id: "gate-talent", kind: "passive", onRemove: (buff, reason) -> print "Systems passive buff removed:", reason
    buffs\apply vanished, marked "removed"
    doomed\kill!
    RemoveUnit vanished\getHandle!
  walker = Unit.create owner, $FourCC("hfoo"), 900, 0, 180
  aura = Aura.new buffs, {
    id: "gate-aura", kind: "aura"
    onApply: (buff) -> print "Systems aura applied to walker", buff\getUnit! == walker
    onRemove: (buff, reason) -> print "Systems aura removed:", reason
  }, hero, ->
    dx, dy = walker\getX! - hero\getX!, walker\getY! - hero\getY!
    if dx * dx + dy * dy <= 400 * 400 then {walker} else {}
  clock\after 8, -> aura\start 0.5
  clock\after 9, -> walker\setPosition -250, 0
  clock\after 11, -> walker\setPosition 900, 0
  clock\after 12.5, ->
    aura\dispose!
    buffs\dispose!
    dummies\dispose!
    clock\dispose!
    print "Systems release 2 done"
  print "Systems release 2 started"
```

  In `systemsGate`'s final `later` callback, after the `Systems gate done` print, add `releaseTwoGate owner`.

  Build it: `yue -e tools/integration.lua` (the gate example must build with clean editor diagnostics). Then check the
  compiled `releaseTwoGate` in the consumer's `src/main.lua`: every name is `local`, and `releaseTwoGate` is defined
  before `systemsGate`.

- [ ] **Step 3: Docs.**
- README: Status `v0.2.0`; API sections `systems.buffs`, `systems.aura`, `systems.dummy` from spec §3–§5 with one
  YueScript example each; "The dummy unit type" with the (possibly corrected) Pkl definition from spec §5.2; "Changes
  from wc3-lib" gains: failures reported instead of rethrown, Units only as buff targets, automatic pruning with the
  disposed check, aura order from the query (no handle-id sort), order ids.
- CHANGELOG: `## Unreleased` with the four modules.
- CONTRIBUTING: step 7 of the in-game gate (release 2), after release 1's six:
  1. `Systems release 2 started`; `Systems dummy cast accepted true isDummy true source is hero true count 1`; the
     footman at the top is stunned and damaged by a Storm Bolt; no dummy model is visible;
  2. about 2.5 s later `Systems dummy gone: count 0 active false life lost <n>` (n > 0);
  3. from 3 s the footman below turns blue and slows; `Systems buff stacks 1 -> 2` at 3.5 s; ticks print
     `Systems buff tick: stacks 2 remaining …`; at 6.5 s `Systems buff removed: expired speed restored true` and the
     colour returns;
  4. at 7 s `Systems buff pruned: killed death` and `Systems buff pruned: removed removed` within 0.25 s; no
     `Systems passive buff removed` yet;
  5. at 9 s `Systems aura applied to walker true` as the walker jumps next to the hero; at 11 s
     `Systems aura removed: source-lost` as it jumps away;
  6. at 12.5 s `Systems passive buff removed: disposed`, then `Systems release 2 done`; no `[systems] … failed` line.
- AGENTS: release 2 modules in the overview; pitfall: an ordered map field must never be named like its methods.

- [ ] **Step 4: Checks and commit** `README.md CHANGELOG.md CONTRIBUTING.md AGENTS.md examples/gate.yue` —
  `docs: release 2 docs, the dummy type and the gate`. Rebuild the gate map: `deno task gate systems --no-launch` in
  `../wrappers-gate`.

---

### Task 7: Release

- [ ] **Step 1:** The maintainer runs `deno task gate systems`; collect the F12 log. Release 1's six steps must still
  pass, then release 2's.
- [ ] **Step 2:** Record: CHANGELOG `## 0.2.0 (<date>)` with a release-gate section; CONTRIBUTING `v0.2.0:` record;
  README Status and the `tag = "v0.2.0"` example; commit, push, tag `v0.2.0`, push the tag, GitHub pre-release from
  the changelog section.
- [ ] **Step 3:** Tag consumption in `../systems-tag-check-020` (wrappers `v0.7.0`, systems `v0.2.0`, the gate example):
  check, build, build `--minify`; the lock records both commits; the fetched files match the tag; the lock is unchanged
  after removing `.moonwell/`. Record it; commit; push.
- [ ] **Step 4:** Moonwell records: AGENTS state bullet and next work (release 3, damage), roadmap, CHANGELOG line;
  commit; push; `gh run list`.
