# Moonwell Broad Wrapper Library (wrappers v0.2.0) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the `moonwell-wrappers` library from six classes to broad everyday coverage: Item, Destructable,
Rect, Region and Force, deeper Unit/Player/Trigger/Group, a shared widget layer and weak widget caches.

**Architecture:** Handwritten annotated Lua 5.3 modules in `../moonwell-wrappers/src/wrappers/`, as in v0.1.0. The
internal handle module gains a table of loaded registries so wrapper *arguments* convert without importing their
module (`Handle.unwrap`, `Handle.unwrapWidget`); modules import each other only to *return* wrappers. An internal widget
module copies shared widget methods onto Unit, Item and Destructable. Unit, Item and Destructable caches become
weak-valued.

**Tech Stack:** Lua 5.3 (game), YueScript 0.34.2 embedded Lua 5.4 test VM, LuaLS 3.19.1, Lua 5.3.6 `luac`, Deno
tooling, Moonwell 0.5.0 consumer fixtures.

**Spec:** `docs/superpowers/specs/2026-09-28-moonwell-wrappers-broad-design.md` (builds on
`docs/superpowers/specs/2026-09-28-moonwell-wrappers-design.md`).

## Global Constraints

- Product code, tests and library docs live in `C:/Users/mdlsvensson/Repo/moonwell-wrappers`; this plan and the
  spec stay in Moonwell. Commit on `main` in each repository; the maintainer pushes. No tags, no publishing.
- No Node.js, npm packages, `node:` or `npm:` specifiers. Deno tooling uses built-ins and `jsr:@std/*` only.
- Runtime Lua ships only under `src/wrappers/`. Literal `require`s only; no umbrella module; no globals; no native call
  or game-object creation at import time.
- The game's Lua lacks `collectgarbage`, `debug`, `io`, `package`, `dofile`, `loadfile`. Shipped code must not use them.
  Tests may use `collectgarbage` and `package.loaded` (they run in the Yue VM only).
- Additive release: every v0.1.0 call keeps its behavior. The only behavior change is the weak Unit cache.
- Every public function carries LuaLS annotations. `fromHandle` stays conservatively nullable. The only diagnostic
  suppressions are line-local `---@diagnostic disable-next-line: param-type-mismatch` on native calls that pass a nil
  filter, each preceded by the existing comment `-- Warcraft accepts a null filter; the generated JASS signature cannot
  express that.`
- Misuse errors read `[wrappers] <Class>.<method>: ...`. Validate before any side-effecting native.
- Never iterate a table keyed by tables with `pairs` when the loop calls natives: key order differs between clients and
  could desync. Use arrays.
- Before every commit in `moonwell-wrappers`, run all its checks (below). Before a Moonwell commit, run Moonwell's
  checks from its AGENTS.md.

## Running the wrappers checks

From `C:/Users/mdlsvensson/Repo/moonwell-wrappers`, in PowerShell:

```powershell
$env:MOONWELL_LUAC = (Resolve-Path .tools/lua53/luac53.exe).Path
$env:MOONWELL_LUALS = "$HOME/.antigravity-ide/extensions/sumneko.lua-3.19.1-win32-x64/server/bin/lua-language-server.exe"
deno task check
deno task lint
deno fmt --check
deno task test
deno task check:lua
deno task test:integration
```

If `deno fmt --check` reports only formatting differences in files you changed, run `deno fmt` and re-run the
check.

`deno task test <suite>` runs one suite (`tests/<suite>.lua`). After Task 1, `deno task test` with no argument runs
every `tests/<name>.lua` except `support.lua` and the `editor-*` fixtures.

## Review focus

1. Wrong-class, forged or disposed arguments fail before any side-effecting native (every task).
2. Weak caches: identity stable while referenced; explicit removal still immediate; strong classes never collected
   (Task 1). Nothing iterates a cache.
3. Trigger tokens: foreign/wrong-kind tokens rejected; double removal harmless; removal mid-firing takes effect; destroy
   order is TriggerClearConditions → DestroyCondition (each owned boolexpr) → DestroyTrigger (Task 6).
4. Group filters never leave a half-filtered group; filters see a snapshot (Task 7).
5. Import graph: Trigger, Effect, Timer, Destructable, Rect and Region load no other public module (Tasks 1–3, 8).
6. LuaLS actually distinguishes Widget from Timer and TriggerAction from TriggerCondition (Task 8).

## Files and interfaces

| File                                        | Responsibility                                                                     |
| ------------------------------------------- | ---------------------------------------------------------------------------------- |
| `src/wrappers/internal/handle.lua`          | `Handle.new(class, name, options?)`, `Handle.unwrap`, `Handle.unwrapWidget`, `Handle.created` |
| `src/wrappers/internal/widget.lua`          | `MoonwellWrappers.Widget` annotation; `Widget.install(class, registry)`            |
| `src/wrappers/internal/callback.lua`        | `check`, `nonnegative`, `call`, new `test(label, fn, argument) -> boolean`         |
| `src/wrappers/{item,destructable}.lua`      | New weak widget classes (spec §5)                                                  |
| `src/wrappers/{rect,region,force}.lua`      | New strong classes (spec §5)                                                       |
| `src/wrappers/{unit,player}.lua`            | Depth (spec §6)                                                                    |
| `src/wrappers/trigger.lua`                  | Registrations, conditions, tokens (spec §7)                                        |
| `src/wrappers/group.lua`                    | Enumerations, filters, `forEach`, `first` (spec §8)                                |
| `src/wrappers/effect.lua`                   | Drops its Unit import (spec §2)                                                    |
| `tests/support.lua`                         | New helpers `checkGetters`, `checkSetters`, `checkDisposed`                        |
| `tests/{handle,imports,item,destructable,rect,region,force,player}.lua` | New suites                             |
| `tools/test.ts`                             | Suite discovery                                                                    |
| `tools/integration.ts`, `tests/editor-*.lua`, `examples/gate.yue` | Editor, bundle and in-game gate coverage                     |
| `README.md`, `CHANGELOG.md`, `CONTRIBUTING.md`, `AGENTS.md`       | Library docs                                                 |
| Moonwell `AGENTS.md`                        | State, next work and backlog                                                       |

Interfaces every task relies on (defined in Task 1):

- `Handle.new(class, name, options?) -> registry` where `options = {weak = boolean?, widget = boolean?}` and
  `registry = {name, wrap(raw), require(value, operation), dispose(value, operation), isDisposed(value, operation),
  member(value)}`. `member(value)` returns the raw handle, `false` when disposed, or `nil` for a non-member.
- `Handle.unwrap(value, className, operation) -> raw` — errors `[wrappers] <operation>: expected <className> wrapper`
  or `[wrappers] <operation>: <className> is disposed`.
- `Handle.unwrapWidget(value, operation) -> raw` — errors `... expected Widget wrapper` or `... <Class> is disposed`.
- `Widget.install(class, registry)` — adds `getLife`, `setLife`, `getX`, `getY` where the class has none.
- `Callback.test(label, fn, argument) -> boolean` (Task 6) — pcall; on error prints
  `[wrappers] <label> failed: <message>` and returns false; otherwise returns the truthiness of the result.

---

### Task 1: Registry lookup, weak caches and the widget layer

**Files:**
- Modify: `src/wrappers/internal/handle.lua` (whole file below)
- Create: `src/wrappers/internal/widget.lua`
- Modify: `src/wrappers/unit.lua`, `src/wrappers/trigger.lua`, `src/wrappers/group.lua`, `src/wrappers/effect.lua`
- Modify: `tools/test.ts`
- Create: `tests/handle.lua`, `tests/imports.lua`

**Interfaces:**
- Produces: everything in "Interfaces every task relies on" except `Callback.test`.

- [ ] **Step 1: Make the runner discover suites**

In `tools/test.ts`, replace the line starting `const suites =` with:

```ts
const suites = Deno.args.length ? Deno.args : [...Deno.readDirSync("tests")]
  .map((entry) => entry.name.match(/^([a-z]+)\.lua$/)?.[1])
  .filter((name): name is string => name !== undefined && name !== "support")
  .sort();
```

Run: `deno task test`
Expected: the five existing suites pass (`SUITE PASSED` for effect, group, timer, trigger, unit).

- [ ] **Step 2: Write the failing registry tests**

Create `tests/handle.lua`:

```lua
native('CreateTimer', function() return {} end)
local Handle = require('wrappers.internal.handle')
local Widget = require('wrappers.internal.widget')
local Unit = require('wrappers.unit')
local Timer = require('wrappers.timer')
eq(totalCalls(), 0)

test('unwrap converts loaded classes and names the operation', function()
    local u = Unit.fromHandle({})
    eq(Handle.unwrap(u, 'Unit', 'Test.op'), u.handle)
    fails(function() Handle.unwrap({}, 'Unit', 'Test.op') end, '[wrappers] Test.op: expected Unit wrapper')
    fails(function() Handle.unwrap(u, 'Nope', 'Test.op') end, '[wrappers] Test.op: expected Nope wrapper')
    u:remove()
    fails(function() Handle.unwrap(u, 'Unit', 'Test.op') end, '[wrappers] Test.op: Unit is disposed')
end)

test('unwrapWidget accepts every widget class and rejects others', function()
    local Fake = {}
    local fakes = Handle.new(Fake, 'FakeWidget', {widget = true})
    local u, f = Unit.fromHandle({}), fakes.wrap({})
    eq(Handle.unwrapWidget(u, 'Test.op'), u.handle)
    eq(Handle.unwrapWidget(f, 'Test.op'), f.handle)
    fails(function() Handle.unwrapWidget(Timer.create(), 'Test.op') end, '[wrappers] Test.op: expected Widget wrapper')
    fails(function() Handle.unwrapWidget({}, 'Test.op') end, 'expected Widget wrapper')
    u:remove()
    fails(function() Handle.unwrapWidget(u, 'Test.op') end, '[wrappers] Test.op: Unit is disposed')
end)

test('duplicate registry names are rejected', function()
    fails(function() Handle.new({}, 'Unit') end, 'duplicate registry: Unit')
end)

test('widget install copies shared methods without replacing class methods', function()
    local Fake = {}
    function Fake:getX() return 'own' end
    local fakes = Handle.new(Fake, 'FakeInstall', {widget = true})
    Widget.install(Fake, fakes)
    local f = fakes.wrap({})
    eq(f:getX(), 'own')
    native('GetWidgetY', function() return 9 end)
    eq(f:getY(), 9); expectCall('GetWidgetY', f.handle)
    native('GetWidgetLife', function() return 5 end)
    eq(f:getLife(), 5); expectCall('GetWidgetLife', f.handle)
    native('SetWidgetLife', function() end)
    f:setLife(3); expectCall('SetWidgetLife', f.handle, 3)
    fakes.dispose(f, 'Test.dispose')
    fails(function() f:getY() end, '[wrappers] FakeInstall.getY: FakeInstall is disposed')
end)

-- Wrap inside a helper so no register of the test body keeps the wrapper alive.
local function wrapAndMark(fromHandle, raw, probe, kind)
    probe[fromHandle(raw)] = kind
end

test('weak caches release unreferenced wrappers; strong caches keep them', function()
    local rawUnit, rawTimer = {}, {}
    local held = Unit.fromHandle({})
    collectgarbage(); collectgarbage()
    eq(Unit.fromHandle(held.handle), held)
    local probe = setmetatable({}, {__mode = 'k'})
    wrapAndMark(Unit.fromHandle, rawUnit, probe, 'unit')
    wrapAndMark(Timer.fromHandle, rawTimer, probe, 'timer')
    collectgarbage(); collectgarbage()
    local left = {}
    for _, kind in pairs(probe) do left[kind] = true end
    eq(left.unit, nil); eq(left.timer, true)
    local fresh = Unit.fromHandle(rawUnit)
    eq(fresh:isDisposed(), false); eq(fresh.handle, rawUnit)
    fresh:remove(); held:remove()
end)
```

Create `tests/imports.lua`:

```lua
for _, name in ipairs({'trigger', 'effect', 'timer'}) do require('wrappers.' .. name) end
eq(totalCalls(), 0)

test('modules that only take wrapper arguments load no other public module', function()
    for _, name in ipairs({'unit', 'player', 'group'}) do eq(package.loaded['wrappers.' .. name], nil) end
end)
```

- [ ] **Step 3: Run the new suites to verify they fail**

Run: `deno task test handle imports`
Expected: `handle` fails loading `wrappers.internal.widget` (module not found); `imports` fails because
`wrappers.unit` and `wrappers.player` are loaded by trigger/effect.

- [ ] **Step 4: Replace `src/wrappers/internal/handle.lua`**

```lua
---@class MoonwellWrappers.Registry<T, H>
---@field name string
---@field wrap fun(raw: H?): T?
---@field require fun(value: unknown, operation: string): H
---@field dispose fun(value: unknown, operation: string): H?
---@field isDisposed fun(value: unknown, operation: string): boolean
---@field member fun(value: unknown): H|false|nil

---@class MoonwellWrappers.RegistryOptions
---@field weak boolean? Weak-valued cache: an unreferenced wrapper may be collected and recreated later.
---@field widget boolean? Join the widget family that Handle.unwrapWidget consults.

local Handle = {}
---@type table<string, MoonwellWrappers.Registry>
local loaded = {}
---@type MoonwellWrappers.Registry[]
local widgets = {}

---Private membership, rather than fields or metatables, authenticates instances.
---@param class table
---@param name string
---@param options MoonwellWrappers.RegistryOptions?
---@return MoonwellWrappers.Registry
function Handle.new(class, name, options)
    if loaded[name] then error('[wrappers] duplicate registry: ' .. name, 2) end
    options = options or {}
    local byHandle = options.weak and setmetatable({}, {__mode = 'v'}) or {}
    local members = setmetatable({}, {__mode = 'k'})
    local registry = {name = name}
    class.__index = class
    local function member(value, operation)
        local raw = members[value]
        if raw == nil then error('[wrappers] ' .. operation .. ': expected ' .. name .. ' wrapper', 3) end
        return raw
    end
    function registry.member(value) return members[value] end
    function registry.wrap(raw)
        if raw == nil then return nil end
        if byHandle[raw] then return byHandle[raw] end
        local value = setmetatable({handle = raw}, class)
        byHandle[raw] = value
        members[value] = raw
        return value
    end
    function registry.require(value, operation)
        local raw = member(value, operation)
        if raw == false then error('[wrappers] ' .. operation .. ': ' .. name .. ' is disposed', 3) end
        return raw
    end
    function registry.dispose(value, operation)
        local raw = member(value, operation)
        if raw == false then return nil end
        members[value] = false
        byHandle[raw] = nil
        value.handle = nil
        return raw
    end
    function registry.isDisposed(value, operation)
        return member(value, operation) == false
    end
    loaded[name] = registry
    if options.widget then widgets[#widgets + 1] = registry end
    return registry
end

---Converts a wrapper argument without importing its module: a caller holding one has loaded it.
---@param value unknown
---@param name string
---@param operation string
---@return any
function Handle.unwrap(value, name, operation)
    local registry = loaded[name]
    if registry == nil then error('[wrappers] ' .. operation .. ': expected ' .. name .. ' wrapper', 2) end
    return registry.require(value, operation)
end

---Converts a Unit, Item or Destructable argument.
---@param value unknown
---@param operation string
---@return widget
function Handle.unwrapWidget(value, operation)
    for _, registry in ipairs(widgets) do
        local raw = registry.member(value)
        if raw == false then error('[wrappers] ' .. operation .. ': ' .. registry.name .. ' is disposed', 2) end
        if raw ~= nil then return raw end
    end
    error('[wrappers] ' .. operation .. ': expected Widget wrapper', 2)
end

---@generic H
---@param raw H?
---@param operation string
---@return H
function Handle.created(raw, operation)
    if raw == nil then error('[wrappers] ' .. operation .. ': native returned nil', 3) end
    return raw
end

return Handle
```

- [ ] **Step 5: Create `src/wrappers/internal/widget.lua`**

```lua
---Annotation-only base of Unit, Item and Destructable. There is no Widget module class and no
---Widget.fromHandle: Warcraft has no reliable handle-type check to pick the wrapper class for a raw widget.
---@class MoonwellWrappers.Widget
---@field getHandle fun(self: MoonwellWrappers.Widget): widget
---@field isDisposed fun(self: MoonwellWrappers.Widget): boolean
---@field getLife fun(self: MoonwellWrappers.Widget): number
---@field setLife fun(self: MoonwellWrappers.Widget, value: number)
---@field getX fun(self: MoonwellWrappers.Widget): number
---@field getY fun(self: MoonwellWrappers.Widget): number

local Widget = {}

---Copies the shared widget methods onto a class, keeping any method the class defines itself.
---@param class table
---@param registry MoonwellWrappers.Registry
function Widget.install(class, registry)
    local name = registry.name
    local shared = {
        {'getLife', function(self) return GetWidgetLife(registry.require(self, name .. '.getLife')) end},
        {'setLife', function(self, value) SetWidgetLife(registry.require(self, name .. '.setLife'), value) end},
        {'getX', function(self) return GetWidgetX(registry.require(self, name .. '.getX')) end},
        {'getY', function(self) return GetWidgetY(registry.require(self, name .. '.getY')) end},
    }
    for _, entry in ipairs(shared) do
        if rawget(class, entry[1]) == nil then class[entry[1]] = entry[2] end
    end
end

return Widget
```

- [ ] **Step 6: Migrate the existing modules**

In `src/wrappers/unit.lua`:

- Add `local Widget = require('wrappers.internal.widget')` after the Handle require.
- Change the class line to `---@class MoonwellWrappers.Unit: MoonwellWrappers.Widget`.
- Change the registry line to `local registry = Handle.new(Unit, 'Unit', {weak = true, widget = true})`.
- In `Unit.create`, replace `local rawOwner = PlayerWrapper.getHandle(owner)` with
  `local rawOwner = Handle.unwrap(owner, 'Player', 'Unit.create')`.
- In `Unit:setOwner`, replace `SetUnitOwner(raw, PlayerWrapper.getHandle(owner), changeColor)` with
  `SetUnitOwner(raw, Handle.unwrap(owner, 'Player', 'Unit.setOwner'), changeColor)`.
- Before `return Unit`, add:

```lua
-- Unit defines all four shared methods itself (GetUnitX/GetUnitY keep v0.1.0's mapping); install is a no-op here.
Widget.install(Unit, registry)
```

In `src/wrappers/trigger.lua`:

- Delete `local Unit = require('wrappers.unit')` and `local PlayerWrapper = require('wrappers.player')`.
- In `registerUnitEvent`, replace `Unit.getHandle(unit)` with `Handle.unwrap(unit, 'Unit', 'Trigger.registerUnitEvent')`.
- In `registerPlayerUnitEvent`, replace `PlayerWrapper.getHandle(player)` with
  `Handle.unwrap(player, 'Player', 'Trigger.registerPlayerUnitEvent')`.

In `src/wrappers/group.lua`, replace `Unit.getHandle(unit)` in `add`, `remove` and `contains` with
`Handle.unwrap(unit, 'Unit', 'Group.add')`, `Handle.unwrap(unit, 'Unit', 'Group.remove')` and
`Handle.unwrap(unit, 'Unit', 'Group.contains')`.

In `src/wrappers/effect.lua`, delete `local Unit = require('wrappers.unit')` and in `Effect.attach` replace
`local raw = Unit.getHandle(target)` with `local raw = Handle.unwrap(target, 'Unit', 'Effect.attach')`.

- [ ] **Step 7: Run all suites**

Run: `deno task test`
Expected: all seven suites print `SUITE PASSED` (effect, group, handle, imports, timer, trigger, unit).

- [ ] **Step 8: Run every check and commit**

Run the full command list in "Running the wrappers checks". Expected: all pass; integration ends with the gate
example message.

```bash
git add src tests tools
git commit -m "feat: registry lookup, weak unit cache and widget layer

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Item and Destructable

**Files:**
- Modify: `tests/support.lua` (append helpers)
- Create: `src/wrappers/item.lua`, `src/wrappers/destructable.lua`
- Create: `tests/item.lua`, `tests/destructable.lua`
- Modify: `tests/imports.lua`

**Interfaces:**
- Consumes: `Handle.new(..., {weak = true, widget = true})`, `Handle.unwrap`, `Widget.install`.
- Produces: `MoonwellWrappers.Item` (`wrappers.item`), `MoonwellWrappers.Destructable` (`wrappers.destructable`).

- [ ] **Step 1: Append test helpers to `tests/support.lua`**

```lua
-- Rows: {nativeName, methodName, returnValue, methodArgs...}; the native receives the handle then the same args.
function checkGetters(wrapper, rows)
    for _, row in ipairs(rows) do
        local value = row[3]
        native(row[1], function() return value end)
        eq(wrapper[row[2]](wrapper, table.unpack(row, 4)), value)
        expectCall(row[1], wrapper.handle, table.unpack(row, 4))
    end
end
-- Rows: {nativeName, methodName, methodArgs...}.
function checkSetters(wrapper, rows)
    for _, row in ipairs(rows) do
        native(row[1], function() end)
        wrapper[row[2]](wrapper, table.unpack(row, 3))
        expectCall(row[1], wrapper.handle, table.unpack(row, 3))
    end
end
function checkDisposed(wrapper, methods)
    local before = totalCalls()
    for _, name in ipairs(methods) do fails(function() wrapper[name](wrapper) end, 'disposed') end
    eq(totalCalls(), before)
end
```

- [ ] **Step 2: Write the failing tests**

Create `tests/item.lua`:

```lua
native('CreateItem', function() return {} end)
native('RemoveItem', function() end)
local Handle = require('wrappers.internal.handle')
local Item = require('wrappers.item')
local Player = require('wrappers.player')
eq(totalCalls(), 0)

test('item identity, factory and widget family', function()
    eq(Item.fromHandle(nil), nil)
    local i = Item.create(1918989411, 10, 20)
    expectCall('CreateItem', 1918989411, 10, 20)
    eq(Item.fromHandle(i.handle), i); eq(i:getHandle(), i.handle); eq(i:isDisposed(), false)
    eq(Handle.unwrapWidget(i, 'Test.op'), i.handle)
    native('CreateItem', function() return nil end)
    fails(function() Item.create(1, 0, 0) end, 'Item.create')
    native('CreateItem', function() return {} end)
    i:remove()
end)

test('item getters read current native state', function()
    local i, owner = Item.fromHandle({}), {}
    checkGetters(i, {{'GetItemTypeId', 'getTypeId', 1918989411}, {'GetItemName', 'getName', 'Claws'},
        {'GetItemLevel', 'getLevel', 2}, {'GetItemCharges', 'getCharges', 3}, {'IsItemOwned', 'isOwned', true},
        {'IsItemPowerup', 'isPowerup', false}, {'IsItemVisible', 'isVisible', true},
        {'IsItemInvulnerable', 'isInvulnerable', false}, {'GetWidgetLife', 'getLife', 75},
        {'GetWidgetX', 'getX', 5}, {'GetWidgetY', 'getY', 6}})
    native('GetItemPlayer', function() return owner end)
    eq(i:getOwner(), Player.fromHandle(owner)); expectCall('GetItemPlayer', i.handle)
    native('GetItemPlayer', function() return nil end)
    fails(function() i:getOwner() end, 'Item.getOwner')
    i:remove()
end)

test('item mutations forward exact arguments', function()
    local i, p = Item.fromHandle({}), Player.fromIndex(0)
    checkSetters(i, {{'SetItemPosition', 'setPosition', 1, 2}, {'SetItemCharges', 'setCharges', 4},
        {'SetItemVisible', 'setVisible', false}, {'SetItemInvulnerable', 'setInvulnerable', true},
        {'SetItemDroppable', 'setDroppable', false}, {'SetItemPawnable', 'setPawnable', true},
        {'SetWidgetLife', 'setLife', 30}})
    native('SetItemPlayer', function() end)
    i:setOwner(p, true); expectCall('SetItemPlayer', i.handle, PLAYER_RAW, true)
    fails(function() i:setOwner(i, true) end, 'Item.setOwner: expected Player wrapper')
    eq(callCount('SetItemPlayer'), 1)
    i:remove()
end)

test('item removal is idempotent and guards every method', function()
    local i = Item.fromHandle({})
    local raw = i.handle
    i:remove(); i:remove()
    expectCall('RemoveItem', raw); eq(callCount('RemoveItem'), 1); eq(i.handle, nil); eq(i:isDisposed(), true)
    checkDisposed(i, {'getHandle', 'getTypeId', 'getName', 'getLevel', 'setPosition', 'getCharges', 'setCharges',
        'getOwner', 'setOwner', 'isOwned', 'isPowerup', 'isVisible', 'setVisible', 'isInvulnerable',
        'setInvulnerable', 'setDroppable', 'setPawnable', 'getLife', 'setLife', 'getX', 'getY'})
end)
```

Create `tests/destructable.lua`:

```lua
native('CreateDestructable', function() return {} end)
native('RemoveDestructable', function() end)
local Handle = require('wrappers.internal.handle')
local Destructable = require('wrappers.destructable')
eq(totalCalls(), 0)

test('destructable identity, factory and widget family', function()
    eq(Destructable.fromHandle(nil), nil)
    local d = Destructable.create(1280601204, 10, 20, 270, 1.2, 3)
    expectCall('CreateDestructable', 1280601204, 10, 20, 270, 1.2, 3)
    eq(Destructable.fromHandle(d.handle), d); eq(d:getHandle(), d.handle)
    eq(Handle.unwrapWidget(d, 'Test.op'), d.handle)
    native('CreateDestructable', function() return nil end)
    fails(function() Destructable.create(1, 0, 0, 0, 1, 0) end, 'Destructable.create')
    native('CreateDestructable', function() return {} end)
    d:remove()
end)

test('destructable natives receive exact arguments', function()
    local d = Destructable.fromHandle({})
    checkGetters(d, {{'GetDestructableTypeId', 'getTypeId', 1280601204}, {'GetDestructableName', 'getName', 'Tree'},
        {'GetDestructableMaxLife', 'getMaxLife', 50}, {'IsDestructableInvulnerable', 'isInvulnerable', false},
        {'GetWidgetLife', 'getLife', 40}, {'GetWidgetX', 'getX', 1}, {'GetWidgetY', 'getY', 2}})
    checkSetters(d, {{'SetDestructableMaxLife', 'setMaxLife', 500}, {'KillDestructable', 'kill'},
        {'DestructableRestoreLife', 'restore', 100, true}, {'SetDestructableInvulnerable', 'setInvulnerable', true},
        {'ShowDestructable', 'show', false}, {'SetDestructableAnimation', 'setAnimation', 'death'},
        {'QueueDestructableAnimation', 'queueAnimation', 'stand'}, {'SetWidgetLife', 'setLife', 10}})
    eq(d:isDisposed(), false)
    d:remove()
end)

test('destructable removal is idempotent and guards every method', function()
    local d = Destructable.fromHandle({})
    local raw = d.handle
    d:remove(); d:remove()
    expectCall('RemoveDestructable', raw); eq(callCount('RemoveDestructable'), 1); eq(d.handle, nil)
    checkDisposed(d, {'getHandle', 'getTypeId', 'getName', 'getMaxLife', 'setMaxLife', 'kill', 'restore',
        'isInvulnerable', 'setInvulnerable', 'show', 'setAnimation', 'queueAnimation', 'getLife', 'setLife',
        'getX', 'getY'})
end)
```

In `tests/imports.lua`, change the first line's list to `{'trigger', 'effect', 'timer', 'destructable'}` and the
checked list to `{'unit', 'player', 'group', 'item'}`.

- [ ] **Step 3: Run to verify failure**

Run: `deno task test item destructable imports`
Expected: item and destructable fail with module `wrappers.item` / `wrappers.destructable` not found.

- [ ] **Step 4: Create `src/wrappers/item.lua`**

```lua
local Handle = require('wrappers.internal.handle')
local Widget = require('wrappers.internal.widget')
local PlayerWrapper = require('wrappers.player')

---@class MoonwellWrappers.Item: MoonwellWrappers.Widget
---@field handle item? Read-only by convention; nil after removal.
local Item = {}
---@type MoonwellWrappers.Registry<MoonwellWrappers.Item, item>
local registry = Handle.new(Item, 'Item', {weak = true, widget = true})

---@param raw item?
---@return MoonwellWrappers.Item?
---@overload fun(raw: nil): nil
function Item.fromHandle(raw) return registry.wrap(raw) end
---@param typeId integer
---@param x number
---@param y number
---@return MoonwellWrappers.Item
function Item.create(typeId, x, y)
    return Handle.created(Item.fromHandle(CreateItem(typeId, x, y)), 'Item.create')
end
---@return item
function Item:getHandle() return registry.require(self, 'Item.getHandle') end
---@return boolean
function Item:isDisposed() return registry.isDisposed(self, 'Item.isDisposed') end
---@return integer
function Item:getTypeId() return GetItemTypeId(registry.require(self, 'Item.getTypeId')) end
---@return string
function Item:getName() return GetItemName(registry.require(self, 'Item.getName')) end
---@return integer
function Item:getLevel() return GetItemLevel(registry.require(self, 'Item.getLevel')) end
---@param x number
---@param y number
function Item:setPosition(x, y) SetItemPosition(registry.require(self, 'Item.setPosition'), x, y) end
---@return integer
function Item:getCharges() return GetItemCharges(registry.require(self, 'Item.getCharges')) end
---@param charges integer
function Item:setCharges(charges) SetItemCharges(registry.require(self, 'Item.setCharges'), charges) end
---@return MoonwellWrappers.Player
function Item:getOwner()
    local owner = GetItemPlayer(registry.require(self, 'Item.getOwner'))
    return Handle.created(PlayerWrapper.fromHandle(owner), 'Item.getOwner')
end
---@param owner MoonwellWrappers.Player
---@param changeColor boolean
function Item:setOwner(owner, changeColor)
    local raw = registry.require(self, 'Item.setOwner')
    SetItemPlayer(raw, Handle.unwrap(owner, 'Player', 'Item.setOwner'), changeColor)
end
---@return boolean
function Item:isOwned() return IsItemOwned(registry.require(self, 'Item.isOwned')) end
---@return boolean
function Item:isPowerup() return IsItemPowerup(registry.require(self, 'Item.isPowerup')) end
---@return boolean
function Item:isVisible() return IsItemVisible(registry.require(self, 'Item.isVisible')) end
---@param visible boolean
function Item:setVisible(visible) SetItemVisible(registry.require(self, 'Item.setVisible'), visible) end
---@return boolean
function Item:isInvulnerable() return IsItemInvulnerable(registry.require(self, 'Item.isInvulnerable')) end
---@param flag boolean
function Item:setInvulnerable(flag) SetItemInvulnerable(registry.require(self, 'Item.setInvulnerable'), flag) end
---@param flag boolean
function Item:setDroppable(flag) SetItemDroppable(registry.require(self, 'Item.setDroppable'), flag) end
---@param flag boolean
function Item:setPawnable(flag) SetItemPawnable(registry.require(self, 'Item.setPawnable'), flag) end
function Item:remove()
    local raw = registry.dispose(self, 'Item.remove')
    if raw then RemoveItem(raw) end
end

Widget.install(Item, registry)
return Item
```

- [ ] **Step 5: Create `src/wrappers/destructable.lua`**

```lua
local Handle = require('wrappers.internal.handle')
local Widget = require('wrappers.internal.widget')

---@class MoonwellWrappers.Destructable: MoonwellWrappers.Widget
---@field handle destructable? Read-only by convention; nil after removal.
local Destructable = {}
---@type MoonwellWrappers.Registry<MoonwellWrappers.Destructable, destructable>
local registry = Handle.new(Destructable, 'Destructable', {weak = true, widget = true})

---@param raw destructable?
---@return MoonwellWrappers.Destructable?
---@overload fun(raw: nil): nil
function Destructable.fromHandle(raw) return registry.wrap(raw) end
---@param typeId integer
---@param x number
---@param y number
---@param facing number
---@param scale number
---@param variation integer
---@return MoonwellWrappers.Destructable
function Destructable.create(typeId, x, y, facing, scale, variation)
    local raw = CreateDestructable(typeId, x, y, facing, scale, variation)
    return Handle.created(Destructable.fromHandle(raw), 'Destructable.create')
end
---@return destructable
function Destructable:getHandle() return registry.require(self, 'Destructable.getHandle') end
---@return boolean
function Destructable:isDisposed() return registry.isDisposed(self, 'Destructable.isDisposed') end
---@return integer
function Destructable:getTypeId() return GetDestructableTypeId(registry.require(self, 'Destructable.getTypeId')) end
---@return string
function Destructable:getName() return GetDestructableName(registry.require(self, 'Destructable.getName')) end
---@return number
function Destructable:getMaxLife() return GetDestructableMaxLife(registry.require(self, 'Destructable.getMaxLife')) end
---@param value number
function Destructable:setMaxLife(value)
    SetDestructableMaxLife(registry.require(self, 'Destructable.setMaxLife'), value)
end
function Destructable:kill() KillDestructable(registry.require(self, 'Destructable.kill')) end
---@param life number
---@param birth boolean
function Destructable:restore(life, birth)
    DestructableRestoreLife(registry.require(self, 'Destructable.restore'), life, birth)
end
---@return boolean
function Destructable:isInvulnerable()
    return IsDestructableInvulnerable(registry.require(self, 'Destructable.isInvulnerable'))
end
---@param flag boolean
function Destructable:setInvulnerable(flag)
    SetDestructableInvulnerable(registry.require(self, 'Destructable.setInvulnerable'), flag)
end
---@param visible boolean
function Destructable:show(visible) ShowDestructable(registry.require(self, 'Destructable.show'), visible) end
---@param animation string
function Destructable:setAnimation(animation)
    SetDestructableAnimation(registry.require(self, 'Destructable.setAnimation'), animation)
end
---@param animation string
function Destructable:queueAnimation(animation)
    QueueDestructableAnimation(registry.require(self, 'Destructable.queueAnimation'), animation)
end
function Destructable:remove()
    local raw = registry.dispose(self, 'Destructable.remove')
    if raw then RemoveDestructable(raw) end
end

Widget.install(Destructable, registry)
return Destructable
```

- [ ] **Step 6: Run all suites**

Run: `deno task test`
Expected: all suites pass, including item, destructable and imports.

- [ ] **Step 7: Run every check and commit**

Run the full check list. Expected: all pass.

```bash
git add src tests
git commit -m "feat: add Item and Destructable wrappers

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Rect, Region and Force

**Files:**
- Create: `src/wrappers/rect.lua`, `src/wrappers/region.lua`, `src/wrappers/force.lua`
- Create: `tests/rect.lua`, `tests/region.lua`, `tests/force.lua`
- Modify: `tests/imports.lua`

**Interfaces:**
- Consumes: `Handle.new`, `Handle.unwrap`, `Handle.created`, test helpers from Task 2.
- Produces: `MoonwellWrappers.Rect` (`wrappers.rect`), `MoonwellWrappers.Region` (`wrappers.region`),
  `MoonwellWrappers.Force` (`wrappers.force`). Class name strings for `Handle.unwrap`: `'Rect'`, `'Region'`, `'Force'`.

- [ ] **Step 1: Write the failing tests**

Create `tests/rect.lua`:

```lua
native('Rect', function() return {} end)
native('GetWorldBounds', function() return {} end)
native('RemoveRect', function() end)
local Rect = require('wrappers.rect')
eq(totalCalls(), 0)

test('rect factories, bounds, changes and destruction', function()
    local r = Rect.create(-10, -20, 30, 40)
    expectCall('Rect', -10, -20, 30, 40)
    eq(Rect.fromHandle(nil), nil); eq(Rect.fromHandle(r.handle), r); eq(r:getHandle(), r.handle)
    local world = Rect.worldBounds()
    expectCall('GetWorldBounds'); assert(world ~= r)
    checkGetters(r, {{'GetRectMinX', 'getMinX', -10}, {'GetRectMinY', 'getMinY', -20},
        {'GetRectMaxX', 'getMaxX', 30}, {'GetRectMaxY', 'getMaxY', 40},
        {'GetRectCenterX', 'getCenterX', 10}, {'GetRectCenterY', 'getCenterY', 10}})
    checkSetters(r, {{'SetRect', 'set', 0, 0, 5, 5}, {'MoveRectTo', 'moveTo', 7, 8}})
    local raw = r.handle
    r:destroy(); r:destroy()
    expectCall('RemoveRect', raw); eq(callCount('RemoveRect'), 1); eq(r.handle, nil); eq(r:isDisposed(), true)
    checkDisposed(r, {'getHandle', 'getMinX', 'getMinY', 'getMaxX', 'getMaxY', 'getCenterX', 'getCenterY', 'set',
        'moveTo'})
    world:destroy()
end)

test('rect factories fail on nil natives', function()
    native('Rect', function() return nil end)
    fails(function() Rect.create(0, 0, 1, 1) end, 'Rect.create')
    native('GetWorldBounds', function() return nil end)
    fails(Rect.worldBounds, 'Rect.worldBounds')
    native('Rect', function() return {} end)
    native('GetWorldBounds', function() return {} end)
end)
```

Create `tests/region.lua`:

```lua
native('CreateRegion', function() return {} end)
native('RemoveRegion', function() end)
native('Rect', function() return {} end)
native('RemoveRect', function() end)
local Region = require('wrappers.region')
local Rect = require('wrappers.rect')
local Unit = require('wrappers.unit')
eq(totalCalls(), 0)

test('region areas and queries forward wrapped arguments', function()
    local g, r, u = Region.create(), Rect.create(0, 0, 1, 1), Unit.fromHandle({})
    eq(Region.fromHandle(nil), nil); eq(Region.fromHandle(g.handle), g)
    for _, name in ipairs({'RegionAddRect', 'RegionClearRect', 'RegionAddCell', 'RegionClearCell'}) do
        native(name, function() end)
    end
    g:addRect(r); expectCall('RegionAddRect', g.handle, r.handle)
    g:clearRect(r); expectCall('RegionClearRect', g.handle, r.handle)
    g:addCell(5, 6); expectCall('RegionAddCell', g.handle, 5, 6)
    g:clearCell(5, 6); expectCall('RegionClearCell', g.handle, 5, 6)
    native('IsPointInRegion', function() return true end)
    eq(g:containsPoint(1, 2), true); expectCall('IsPointInRegion', g.handle, 1, 2)
    native('IsUnitInRegion', function() return false end)
    eq(g:containsUnit(u), false); expectCall('IsUnitInRegion', g.handle, u.handle)
    fails(function() g:addRect(u) end, 'Region.addRect: expected Rect wrapper')
    fails(function() g:containsUnit(r) end, 'Region.containsUnit: expected Unit wrapper')
    eq(callCount('RegionAddRect'), 1); eq(callCount('IsUnitInRegion'), 1)
    local raw = g.handle
    g:destroy(); g:destroy()
    expectCall('RemoveRegion', raw); eq(callCount('RemoveRegion'), 1); eq(callCount('RemoveRect'), 0)
    checkDisposed(g, {'getHandle', 'addRect', 'clearRect', 'addCell', 'clearCell', 'containsPoint', 'containsUnit'})
    r:destroy(); u:remove()
    native('CreateRegion', function() return nil end); fails(Region.create, 'Region.create')
    native('CreateRegion', function() return {} end)
end)
```

Create `tests/force.lua`:

```lua
local enumerated, current = {}, nil
native('CreateForce', function() return {} end)
native('DestroyForce', function() end)
native('ForForce', function(_, callback)
    for _, raw in ipairs(enumerated) do current = raw; callback() end
end)
native('GetEnumPlayer', function() return current end)
local Force = require('wrappers.force')
local Player = require('wrappers.player')
eq(totalCalls(), 0)

test('force membership and enumerations clear first', function()
    local f, p = Force.create(), Player.fromIndex(0)
    eq(Force.fromHandle(nil), nil); eq(Force.fromHandle(f.handle), f)
    for _, name in ipairs({'ForceAddPlayer', 'ForceRemovePlayer', 'ForceClear', 'ForceEnumPlayers',
        'ForceEnumAllies', 'ForceEnumEnemies'}) do native(name, function() end) end
    native('IsPlayerInForce', function() return true end)
    f:add(p); expectCall('ForceAddPlayer', f.handle, PLAYER_RAW)
    eq(f:contains(p), true); expectCall('IsPlayerInForce', PLAYER_RAW, f.handle)
    f:remove(p); expectCall('ForceRemovePlayer', f.handle, PLAYER_RAW)
    f:clear(); expectCall('ForceClear', f.handle)
    resetCalls(); f:enumPlayers()
    eq(callName(1), 'ForceClear'); expectCall('ForceEnumPlayers', f.handle, nil)
    resetCalls(); f:enumAllies(p)
    eq(callName(1), 'ForceClear'); expectCall('ForceEnumAllies', f.handle, PLAYER_RAW, nil)
    resetCalls(); f:enumEnemies(p)
    eq(callName(1), 'ForceClear'); expectCall('ForceEnumEnemies', f.handle, PLAYER_RAW, nil)
    resetCalls()
    fails(function() f:enumAllies(f) end, 'Force.enumAllies: expected Player wrapper')
    fails(function() f:add({}) end, 'Force.add: expected Player wrapper')
    eq(totalCalls(), 0)
    f:destroy()
    native('CreateForce', function() return nil end); fails(Force.create, 'Force.create')
    native('CreateForce', function() return {} end)
end)

test('player snapshots are dense and independent', function()
    local f, a, b = Force.create(), {}, {}
    enumerated = {a, b}
    local first = f:getPlayers()
    eq(callCount('ForForce'), 1)
    eq(#first, 2); eq(first[1], Player.fromHandle(a)); eq(first[2], Player.fromHandle(b))
    enumerated = {}
    eq(#f:getPlayers(), 0); eq(#first, 2)
    local raw = f.handle
    f:destroy(); f:destroy()
    expectCall('DestroyForce', raw); eq(callCount('DestroyForce'), 1); eq(f.handle, nil)
    checkDisposed(f, {'getHandle', 'add', 'remove', 'contains', 'clear', 'enumPlayers', 'enumAllies',
        'enumEnemies', 'getPlayers'})
end)
```

In `tests/imports.lua`, change the first line's list to
`{'trigger', 'effect', 'timer', 'destructable', 'rect', 'region'}` and the checked list to
`{'unit', 'player', 'group', 'item', 'force'}`.

- [ ] **Step 2: Run to verify failure**

Run: `deno task test rect region force imports`
Expected: rect, region and force fail with their modules not found; imports fails on `wrappers.rect`.

- [ ] **Step 3: Create `src/wrappers/rect.lua`**

The local is `RectWrapper` because the native constructor is the global `Rect`.

```lua
local Handle = require('wrappers.internal.handle')

---@class MoonwellWrappers.Rect
---@field handle rect? Read-only by convention; nil after destruction.
local RectWrapper = {}
---@type MoonwellWrappers.Registry<MoonwellWrappers.Rect, rect>
local registry = Handle.new(RectWrapper, 'Rect')

---@param raw rect?
---@return MoonwellWrappers.Rect?
---@overload fun(raw: nil): nil
function RectWrapper.fromHandle(raw) return registry.wrap(raw) end
---@param minX number
---@param minY number
---@param maxX number
---@param maxY number
---@return MoonwellWrappers.Rect
function RectWrapper.create(minX, minY, maxX, maxY)
    return Handle.created(RectWrapper.fromHandle(Rect(minX, minY, maxX, maxY)), 'Rect.create')
end
---GetWorldBounds allocates a new rect on every call: the result is owned; destroy it.
---@return MoonwellWrappers.Rect
function RectWrapper.worldBounds()
    return Handle.created(RectWrapper.fromHandle(GetWorldBounds()), 'Rect.worldBounds')
end
---@return rect
function RectWrapper:getHandle() return registry.require(self, 'Rect.getHandle') end
---@return boolean
function RectWrapper:isDisposed() return registry.isDisposed(self, 'Rect.isDisposed') end
---@return number
function RectWrapper:getMinX() return GetRectMinX(registry.require(self, 'Rect.getMinX')) end
---@return number
function RectWrapper:getMinY() return GetRectMinY(registry.require(self, 'Rect.getMinY')) end
---@return number
function RectWrapper:getMaxX() return GetRectMaxX(registry.require(self, 'Rect.getMaxX')) end
---@return number
function RectWrapper:getMaxY() return GetRectMaxY(registry.require(self, 'Rect.getMaxY')) end
---@return number
function RectWrapper:getCenterX() return GetRectCenterX(registry.require(self, 'Rect.getCenterX')) end
---@return number
function RectWrapper:getCenterY() return GetRectCenterY(registry.require(self, 'Rect.getCenterY')) end
---@param minX number
---@param minY number
---@param maxX number
---@param maxY number
function RectWrapper:set(minX, minY, maxX, maxY) SetRect(registry.require(self, 'Rect.set'), minX, minY, maxX, maxY) end
---@param x number
---@param y number
function RectWrapper:moveTo(x, y) MoveRectTo(registry.require(self, 'Rect.moveTo'), x, y) end
function RectWrapper:destroy()
    local raw = registry.dispose(self, 'Rect.destroy')
    if raw then RemoveRect(raw) end
end

return RectWrapper
```

- [ ] **Step 4: Create `src/wrappers/region.lua`**

```lua
local Handle = require('wrappers.internal.handle')

---@class MoonwellWrappers.Region
---@field handle region? Read-only by convention; nil after destruction.
local Region = {}
---@type MoonwellWrappers.Registry<MoonwellWrappers.Region, region>
local registry = Handle.new(Region, 'Region')

---@param raw region?
---@return MoonwellWrappers.Region?
---@overload fun(raw: nil): nil
function Region.fromHandle(raw) return registry.wrap(raw) end
---@return MoonwellWrappers.Region
function Region.create() return Handle.created(Region.fromHandle(CreateRegion()), 'Region.create') end
---@return region
function Region:getHandle() return registry.require(self, 'Region.getHandle') end
---@return boolean
function Region:isDisposed() return registry.isDisposed(self, 'Region.isDisposed') end
---@param rect MoonwellWrappers.Rect
function Region:addRect(rect)
    local raw = registry.require(self, 'Region.addRect')
    RegionAddRect(raw, Handle.unwrap(rect, 'Rect', 'Region.addRect'))
end
---@param rect MoonwellWrappers.Rect
function Region:clearRect(rect)
    local raw = registry.require(self, 'Region.clearRect')
    RegionClearRect(raw, Handle.unwrap(rect, 'Rect', 'Region.clearRect'))
end
---@param x number
---@param y number
function Region:addCell(x, y) RegionAddCell(registry.require(self, 'Region.addCell'), x, y) end
---@param x number
---@param y number
function Region:clearCell(x, y) RegionClearCell(registry.require(self, 'Region.clearCell'), x, y) end
---@param x number
---@param y number
---@return boolean
function Region:containsPoint(x, y) return IsPointInRegion(registry.require(self, 'Region.containsPoint'), x, y) end
---@param unit MoonwellWrappers.Unit
---@return boolean
function Region:containsUnit(unit)
    local raw = registry.require(self, 'Region.containsUnit')
    return IsUnitInRegion(raw, Handle.unwrap(unit, 'Unit', 'Region.containsUnit'))
end
---A region does not own its rects.
function Region:destroy()
    local raw = registry.dispose(self, 'Region.destroy')
    if raw then RemoveRegion(raw) end
end

return Region
```

- [ ] **Step 5: Create `src/wrappers/force.lua`**

```lua
local Handle = require('wrappers.internal.handle')
local PlayerWrapper = require('wrappers.player')

---@class MoonwellWrappers.Force
---@field handle force? Read-only by convention; nil after destruction.
local Force = {}
---@type MoonwellWrappers.Registry<MoonwellWrappers.Force, force>
local registry = Handle.new(Force, 'Force')

---@param raw force?
---@return MoonwellWrappers.Force?
---@overload fun(raw: nil): nil
function Force.fromHandle(raw) return registry.wrap(raw) end
---@return MoonwellWrappers.Force
function Force.create() return Handle.created(Force.fromHandle(CreateForce()), 'Force.create') end
---@return force
function Force:getHandle() return registry.require(self, 'Force.getHandle') end
---@return boolean
function Force:isDisposed() return registry.isDisposed(self, 'Force.isDisposed') end
---@param player MoonwellWrappers.Player
function Force:add(player)
    local raw = registry.require(self, 'Force.add')
    ForceAddPlayer(raw, Handle.unwrap(player, 'Player', 'Force.add'))
end
---@param player MoonwellWrappers.Player
function Force:remove(player)
    local raw = registry.require(self, 'Force.remove')
    ForceRemovePlayer(raw, Handle.unwrap(player, 'Player', 'Force.remove'))
end
---@param player MoonwellWrappers.Player
---@return boolean
function Force:contains(player)
    local raw = registry.require(self, 'Force.contains')
    return IsPlayerInForce(Handle.unwrap(player, 'Player', 'Force.contains'), raw)
end
function Force:clear() ForceClear(registry.require(self, 'Force.clear')) end
---Clears the force, then adds every player.
function Force:enumPlayers()
    local raw = registry.require(self, 'Force.enumPlayers')
    ForceClear(raw)
    -- Warcraft accepts a null filter; the generated JASS signature cannot express that.
    ---@diagnostic disable-next-line: param-type-mismatch
    ForceEnumPlayers(raw, nil)
end
---Clears the force, then adds the allies of a player.
---@param player MoonwellWrappers.Player
function Force:enumAllies(player)
    local raw = registry.require(self, 'Force.enumAllies')
    local rawPlayer = Handle.unwrap(player, 'Player', 'Force.enumAllies')
    ForceClear(raw)
    -- Warcraft accepts a null filter; the generated JASS signature cannot express that.
    ---@diagnostic disable-next-line: param-type-mismatch
    ForceEnumAllies(raw, rawPlayer, nil)
end
---Clears the force, then adds the enemies of a player.
---@param player MoonwellWrappers.Player
function Force:enumEnemies(player)
    local raw = registry.require(self, 'Force.enumEnemies')
    local rawPlayer = Handle.unwrap(player, 'Player', 'Force.enumEnemies')
    ForceClear(raw)
    -- Warcraft accepts a null filter; the generated JASS signature cannot express that.
    ---@diagnostic disable-next-line: param-type-mismatch
    ForceEnumEnemies(raw, rawPlayer, nil)
end
---A new dense snapshot; later force changes do not alter it.
---@return MoonwellWrappers.Player[]
function Force:getPlayers()
    local raw = registry.require(self, 'Force.getPlayers')
    local result = {}
    ForForce(raw, function()
        local player = PlayerWrapper.fromHandle(GetEnumPlayer())
        if player then result[#result + 1] = player end
    end)
    return result
end
function Force:destroy()
    local raw = registry.dispose(self, 'Force.destroy')
    if raw then DestroyForce(raw) end
end

return Force
```

- [ ] **Step 6: Run all suites**

Run: `deno task test`
Expected: all suites pass.

- [ ] **Step 7: Run every check and commit**

Run the full check list. Expected: all pass (the new nil-filter lines produce no LuaLS diagnostic in integration).

```bash
git add src tests
git commit -m "feat: add Rect, Region and Force wrappers

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Unit depth

**Files:**
- Modify: `src/wrappers/unit.lua` (whole file below)
- Modify: `tests/unit.lua` (prepend constants and a require; append tests)

**Interfaces:**
- Consumes: `Handle.unwrap`, `Handle.unwrapWidget`, `MoonwellWrappers.Item` (`wrappers.item`), `Widget.install`.
- Produces: the Unit methods in spec §6. Unit now imports Item.

- [ ] **Step 1: Write the failing tests**

At the top of `tests/unit.lua`, before `local Player = require('wrappers.player')`, add:

```lua
UNIT_TYPE_HERO, UNIT_TYPE_DEAD, UNIT_STATE_MANA = {}, {}, {}
native('RemoveItem', function() end)
```

After `local Unit = require('wrappers.unit')`, add `local Item = require('wrappers.item')`.

Append to `tests/unit.lua`:

```lua
test('hero methods forward exact arguments', function()
    local u = Unit.fromHandle({})
    native('IsUnitType', function(_, kind) return kind == UNIT_TYPE_HERO end)
    eq(u:isHero(), true); expectCall('IsUnitType', u.handle, UNIT_TYPE_HERO)
    checkGetters(u, {{'GetHeroProperName', 'getHeroName', 'Arthas'}, {'GetHeroLevel', 'getLevel', 3},
        {'GetHeroXP', 'getXP', 200}, {'GetHeroSkillPoints', 'getSkillPoints', 1},
        {'GetHeroStr', 'getStr', 17, true}, {'GetHeroAgi', 'getAgi', 18, false}, {'GetHeroInt', 'getInt', 19, true},
        {'UnitModifySkillPoints', 'modifySkillPoints', true, -1}, {'ReviveHero', 'revive', false, 1, 2, true}})
    checkSetters(u, {{'SetHeroLevel', 'setLevel', 4, true}, {'SetHeroXP', 'setXP', 300, false},
        {'AddHeroXP', 'addXP', 50, true}, {'SetHeroStr', 'setStr', 20, false}, {'SetHeroAgi', 'setAgi', 21, true},
        {'SetHeroInt', 'setInt', 22, false}, {'SelectHeroSkill', 'selectSkill', 1097361000}})
    u:remove()
end)

test('ability methods forward exact arguments', function()
    local u = Unit.fromHandle({})
    checkGetters(u, {{'UnitAddAbility', 'addAbility', true, 1097361000},
        {'UnitRemoveAbility', 'removeAbility', false, 1097361000},
        {'GetUnitAbilityLevel', 'getAbilityLevel', 2, 1097361000},
        {'SetUnitAbilityLevel', 'setAbilityLevel', 3, 1097361000, 3},
        {'BlzGetUnitAbilityCooldownRemaining', 'getCooldownRemaining', 1.5, 1097361000}})
    checkSetters(u, {{'BlzUnitHideAbility', 'hideAbility', 1097361000, true},
        {'BlzUnitDisableAbility', 'disableAbility', 1097361000, true, false},
        {'BlzStartUnitAbilityCooldown', 'startCooldown', 1097361000, 5},
        {'BlzEndUnitAbilityCooldown', 'endCooldown', 1097361000}})
    native('UnitMakeAbilityPermanent', function() return true end)
    eq(u:makeAbilityPermanent(1097361000, true), true)
    expectCall('UnitMakeAbilityPermanent', u.handle, true, 1097361000)
    u:remove()
end)

test('inventory validates slots and wraps items', function()
    local u, rawItem = Unit.fromHandle({}), {}
    native('UnitInventorySize', function() return 6 end)
    native('UnitItemInSlot', function(_, slot) if slot == 2 then return rawItem end end)
    eq(u:getInventorySize(), 6); expectCall('UnitInventorySize', u.handle)
    local item = u:getItemInSlot(2)
    eq(item, Item.fromHandle(rawItem)); expectCall('UnitItemInSlot', u.handle, 2)
    eq(u:getItemInSlot(0), nil)
    for _, bad in ipairs({-1, 6, 1.5, 0/0, math.huge, '1'}) do
        fails(function() u:getItemInSlot(bad) end, 'Unit.getItemInSlot: expected an inventory slot index')
        fails(function() u:removeItemFromSlot(bad) end, 'Unit.removeItemFromSlot: expected an inventory slot index')
        fails(function() u:dropItemToSlot(item, bad) end, 'Unit.dropItemToSlot: expected an inventory slot index')
    end
    eq(callCount('UnitItemInSlot'), 2); eq(callCount('UnitRemoveItemFromSlot'), 0); eq(callCount('UnitDropItemSlot'), 0)
    native('UnitRemoveItemFromSlot', function() return rawItem end)
    eq(u:removeItemFromSlot(2), item); expectCall('UnitRemoveItemFromSlot', u.handle, 2)
    native('UnitAddItemById', function() return nil end)
    eq(u:addItemById(1), nil); expectCall('UnitAddItemById', u.handle, 1)
    for _, row in ipairs({{'UnitAddItem', 'addItem', true}, {'UnitHasItem', 'hasItem', false},
        {'UnitUseItem', 'useItem', true}}) do
        native(row[1], function() return row[3] end)
        eq(u[row[2]](u, item), row[3]); expectCall(row[1], u.handle, rawItem)
    end
    native('UnitRemoveItem', function() end)
    u:removeItem(item); expectCall('UnitRemoveItem', u.handle, rawItem)
    native('UnitDropItemPoint', function() return true end)
    eq(u:dropItemAt(item, 3, 4), true); expectCall('UnitDropItemPoint', u.handle, rawItem, 3, 4)
    native('UnitDropItemSlot', function() return true end)
    eq(u:dropItemToSlot(item, 5), true); expectCall('UnitDropItemSlot', u.handle, rawItem, 5)
    fails(function() u:addItem(u) end, 'Unit.addItem: expected Item wrapper')
    item:remove()
    fails(function() u:hasItem(item) end, 'Unit.hasItem: Item is disposed')
    eq(callCount('UnitAddItem'), 1); eq(callCount('UnitHasItem'), 1)
    u:remove()
end)

test('state and presentation methods', function()
    local u, kind, p = Unit.fromHandle({}), {}, Player.fromIndex(0)
    native('GetUnitState', function() return 40 end)
    eq(u:getMana(), 40); expectCall('GetUnitState', u.handle, UNIT_STATE_MANA)
    native('SetUnitState', function() end)
    u:setMana(10); expectCall('SetUnitState', u.handle, UNIT_STATE_MANA, 10)
    checkGetters(u, {{'BlzGetUnitMaxMana', 'getMaxMana', 100}, {'GetUnitMoveSpeed', 'getMoveSpeed', 270},
        {'IsUnitPaused', 'isPaused', false}, {'BlzIsUnitInvulnerable', 'isInvulnerable', true},
        {'IsUnitHidden', 'isHidden', false}, {'GetUnitName', 'getName', 'Footman'},
        {'GetUnitCurrentOrder', 'getCurrentOrder', 851983}, {'IsUnitType', 'isType', true, kind}})
    checkSetters(u, {{'BlzSetUnitMaxMana', 'setMaxMana', 150}, {'BlzSetUnitMaxHP', 'setMaxLife', 500},
        {'SetUnitMoveSpeed', 'setMoveSpeed', 300}, {'SetUnitX', 'setX', 5}, {'SetUnitY', 'setY', 6},
        {'SetUnitVertexColor', 'setVertexColor', 255, 128, 0, 200}, {'SetUnitAnimation', 'setAnimation', 'attack'},
        {'PauseUnit', 'pause', true}, {'SetUnitInvulnerable', 'setInvulnerable', false}, {'ShowUnit', 'show', false},
        {'UnitApplyTimedLife', 'applyTimedLife', 1112045413, 5}})
    native('SetUnitScale', function() end)
    u:setScale(1.5); expectCall('SetUnitScale', u.handle, 1.5, 1.5, 1.5)
    native('IsUnitAlly', function() return true end)
    eq(u:isAlly(p), true); expectCall('IsUnitAlly', u.handle, PLAYER_RAW)
    native('IsUnitEnemy', function() return false end)
    eq(u:isEnemy(p), false); expectCall('IsUnitEnemy', u.handle, PLAYER_RAW)
    fails(function() u:isAlly(u) end, 'Unit.isAlly: expected Player wrapper')
    u:remove()
end)

test('isAlive combines the dead type and the removed type id', function()
    local u, dead, typeId = Unit.fromHandle({}), false, 1
    native('IsUnitType', function(_, kind) eq(kind, UNIT_TYPE_DEAD); return dead end)
    native('GetUnitTypeId', function() return typeId end)
    eq(u:isAlive(), true)
    dead = true; eq(u:isAlive(), false)
    dead, typeId = false, 0; eq(u:isAlive(), false)
    u:remove()
end)

test('orders and damage accept any widget target', function()
    local u, target, item = Unit.fromHandle({}), Unit.fromHandle({}), Item.fromHandle({})
    local attack, damage, weapon = {}, {}, {}
    native('UnitDamageTarget', function() return true end)
    eq(u:damageTarget(item, 50, true, false, attack, damage, weapon), true)
    expectCall('UnitDamageTarget', u.handle, item.handle, 50, true, false, attack, damage, weapon)
    native('IssueTargetOrder', function() return true end)
    eq(u:issueTargetOrder('smart', item), true); expectCall('IssueTargetOrder', u.handle, 'smart', item.handle)
    checkGetters(u, {{'IssueImmediateOrderById', 'issueOrderById', true, 851972},
        {'IssuePointOrderById', 'issuePointOrderById', false, 851986, 1, 2}})
    native('IssueTargetOrderById', function() return true end)
    eq(u:issueTargetOrderById(851983, target), true)
    expectCall('IssueTargetOrderById', u.handle, 851983, target.handle)
    fails(function() u:damageTarget(Player.fromIndex(0), 1, true, false, attack, damage, weapon) end,
        'Unit.damageTarget: expected Widget wrapper')
    fails(function() u:issueTargetOrderById(1, Player.fromIndex(0)) end,
        'Unit.issueTargetOrderById: expected Widget wrapper')
    eq(callCount('UnitDamageTarget'), 1); eq(callCount('IssueTargetOrderById'), 1)
    item:remove(); target:remove(); u:remove()
end)

test('new unit methods reject a removed receiver', function()
    local u = Unit.fromHandle({})
    u:remove()
    checkDisposed(u, {'isHero', 'getHeroName', 'getLevel', 'setLevel', 'getXP', 'setXP', 'addXP', 'getStr', 'setStr',
        'getAgi', 'setAgi', 'getInt', 'setInt', 'getSkillPoints', 'modifySkillPoints', 'selectSkill', 'revive',
        'addAbility', 'removeAbility', 'getAbilityLevel', 'setAbilityLevel', 'makeAbilityPermanent', 'hideAbility',
        'disableAbility', 'startCooldown', 'endCooldown', 'getCooldownRemaining', 'getInventorySize', 'getItemInSlot',
        'addItem', 'addItemById', 'removeItem', 'removeItemFromSlot', 'hasItem', 'dropItemAt', 'dropItemToSlot',
        'useItem', 'getMana', 'setMana', 'getMaxMana', 'setMaxMana', 'setMaxLife', 'getMoveSpeed', 'setMoveSpeed',
        'setX', 'setY', 'setScale', 'setVertexColor', 'setAnimation', 'pause', 'isPaused', 'setInvulnerable',
        'isInvulnerable', 'show', 'isHidden', 'isType', 'isAlly', 'isEnemy', 'getName', 'getCurrentOrder', 'isAlive',
        'damageTarget', 'applyTimedLife', 'issueOrderById', 'issuePointOrderById', 'issueTargetOrderById'})
end)
```

- [ ] **Step 2: Run to verify failure**

Run: `deno task test unit`
Expected: FAIL — the new tests fail with `attempt to call a nil value (method 'isHero')` and similar.

- [ ] **Step 3: Replace `src/wrappers/unit.lua`**

```lua
local Handle = require('wrappers.internal.handle')
local Widget = require('wrappers.internal.widget')
local PlayerWrapper = require('wrappers.player')
local Item = require('wrappers.item')

---@class MoonwellWrappers.Unit: MoonwellWrappers.Widget
---@field handle unit? Read-only by convention; nil after removal.
local Unit = {}
---@type MoonwellWrappers.Registry<MoonwellWrappers.Unit, unit>
local registry = Handle.new(Unit, 'Unit', {weak = true, widget = true})

---@param raw unit?
---@return MoonwellWrappers.Unit?
---@overload fun(raw: nil): nil
function Unit.fromHandle(raw) return registry.wrap(raw) end

---@param owner MoonwellWrappers.Player
---@param typeId integer
---@param x number
---@param y number
---@param facing number
---@return MoonwellWrappers.Unit
function Unit.create(owner, typeId, x, y, facing)
    local rawOwner = Handle.unwrap(owner, 'Player', 'Unit.create')
    return Handle.created(Unit.fromHandle(CreateUnit(rawOwner, typeId, x, y, facing)), 'Unit.create')
end
---@return unit
function Unit:getHandle() return registry.require(self, 'Unit.getHandle') end
---@return boolean
function Unit:isDisposed() return registry.isDisposed(self, 'Unit.isDisposed') end
---@return integer
function Unit:getTypeId() return GetUnitTypeId(registry.require(self, 'Unit.getTypeId')) end
---@return MoonwellWrappers.Player
function Unit:getOwner()
    return Handle.created(PlayerWrapper.fromHandle(GetOwningPlayer(registry.require(self, 'Unit.getOwner'))), 'Unit.getOwner')
end
---@param owner MoonwellWrappers.Player
---@param changeColor boolean
function Unit:setOwner(owner, changeColor)
    local raw = registry.require(self, 'Unit.setOwner')
    SetUnitOwner(raw, Handle.unwrap(owner, 'Player', 'Unit.setOwner'), changeColor)
end
---@return number
function Unit:getX() return GetUnitX(registry.require(self, 'Unit.getX')) end
---@return number
function Unit:getY() return GetUnitY(registry.require(self, 'Unit.getY')) end
---Uses SetUnitPosition, which respects pathing.
---@param x number
---@param y number
function Unit:setPosition(x, y) SetUnitPosition(registry.require(self, 'Unit.setPosition'), x, y) end
---Uses SetUnitX, which ignores pathing.
---@param x number
function Unit:setX(x) SetUnitX(registry.require(self, 'Unit.setX'), x) end
---Uses SetUnitY, which ignores pathing.
---@param y number
function Unit:setY(y) SetUnitY(registry.require(self, 'Unit.setY'), y) end
---@return number
function Unit:getFacing() return GetUnitFacing(registry.require(self, 'Unit.getFacing')) end
---@param facing number
function Unit:setFacing(facing) SetUnitFacing(registry.require(self, 'Unit.setFacing'), facing) end
---@return number
function Unit:getLife() return GetWidgetLife(registry.require(self, 'Unit.getLife')) end
---@param value number
function Unit:setLife(value) SetWidgetLife(registry.require(self, 'Unit.setLife'), value) end
---@return integer
function Unit:getMaxLife() return BlzGetUnitMaxHP(registry.require(self, 'Unit.getMaxLife')) end
---@param value integer
function Unit:setMaxLife(value) BlzSetUnitMaxHP(registry.require(self, 'Unit.setMaxLife'), value) end
---@return number
function Unit:getMana() return GetUnitState(registry.require(self, 'Unit.getMana'), UNIT_STATE_MANA) end
---@param value number
function Unit:setMana(value) SetUnitState(registry.require(self, 'Unit.setMana'), UNIT_STATE_MANA, value) end
---@return integer
function Unit:getMaxMana() return BlzGetUnitMaxMana(registry.require(self, 'Unit.getMaxMana')) end
---@param value integer
function Unit:setMaxMana(value) BlzSetUnitMaxMana(registry.require(self, 'Unit.setMaxMana'), value) end
---@return number
function Unit:getMoveSpeed() return GetUnitMoveSpeed(registry.require(self, 'Unit.getMoveSpeed')) end
---@param speed number
function Unit:setMoveSpeed(speed) SetUnitMoveSpeed(registry.require(self, 'Unit.setMoveSpeed'), speed) end
---@param color playercolor
function Unit:setColor(color) SetUnitColor(registry.require(self, 'Unit.setColor'), color) end
---@param scale number Uniform scale.
function Unit:setScale(scale) SetUnitScale(registry.require(self, 'Unit.setScale'), scale, scale, scale) end
---@param red integer 0-255
---@param green integer 0-255
---@param blue integer 0-255
---@param alpha integer 0-255
function Unit:setVertexColor(red, green, blue, alpha)
    SetUnitVertexColor(registry.require(self, 'Unit.setVertexColor'), red, green, blue, alpha)
end
---@param animation string
function Unit:setAnimation(animation) SetUnitAnimation(registry.require(self, 'Unit.setAnimation'), animation) end
---@param flag boolean
function Unit:pause(flag) PauseUnit(registry.require(self, 'Unit.pause'), flag) end
---@return boolean
function Unit:isPaused() return IsUnitPaused(registry.require(self, 'Unit.isPaused')) end
---@param flag boolean
function Unit:setInvulnerable(flag) SetUnitInvulnerable(registry.require(self, 'Unit.setInvulnerable'), flag) end
---@return boolean
function Unit:isInvulnerable() return BlzIsUnitInvulnerable(registry.require(self, 'Unit.isInvulnerable')) end
---@param visible boolean
function Unit:show(visible) ShowUnit(registry.require(self, 'Unit.show'), visible) end
---@return boolean
function Unit:isHidden() return IsUnitHidden(registry.require(self, 'Unit.isHidden')) end
---@param kind unittype
---@return boolean
function Unit:isType(kind) return IsUnitType(registry.require(self, 'Unit.isType'), kind) end
---@param player MoonwellWrappers.Player
---@return boolean
function Unit:isAlly(player)
    local raw = registry.require(self, 'Unit.isAlly')
    return IsUnitAlly(raw, Handle.unwrap(player, 'Player', 'Unit.isAlly'))
end
---@param player MoonwellWrappers.Player
---@return boolean
function Unit:isEnemy(player)
    local raw = registry.require(self, 'Unit.isEnemy')
    return IsUnitEnemy(raw, Handle.unwrap(player, 'Player', 'Unit.isEnemy'))
end
---@return string
function Unit:getName() return GetUnitName(registry.require(self, 'Unit.getName')) end
---@return integer
function Unit:getCurrentOrder() return GetUnitCurrentOrder(registry.require(self, 'Unit.getCurrentOrder')) end
---Not dead and not removed. UnitAlive is a common.ai native missing from Moonwell's natives.
---@return boolean
function Unit:isAlive()
    local raw = registry.require(self, 'Unit.isAlive')
    return not IsUnitType(raw, UNIT_TYPE_DEAD) and GetUnitTypeId(raw) ~= 0
end
function Unit:kill() KillUnit(registry.require(self, 'Unit.kill')) end
function Unit:remove()
    local raw = registry.dispose(self, 'Unit.remove')
    if raw then RemoveUnit(raw) end
end
---@param buffId integer
---@param duration number
function Unit:applyTimedLife(buffId, duration)
    UnitApplyTimedLife(registry.require(self, 'Unit.applyTimedLife'), buffId, duration)
end
---@param target MoonwellWrappers.Widget
---@param amount number
---@param attack boolean
---@param ranged boolean
---@param attackType attacktype
---@param damageType damagetype
---@param weaponType weapontype
---@return boolean
function Unit:damageTarget(target, amount, attack, ranged, attackType, damageType, weaponType)
    local raw = registry.require(self, 'Unit.damageTarget')
    local rawTarget = Handle.unwrapWidget(target, 'Unit.damageTarget')
    return UnitDamageTarget(raw, rawTarget, amount, attack, ranged, attackType, damageType, weaponType)
end

-- Hero. Warcraft's own behavior applies to non-heroes.

---@return boolean
function Unit:isHero() return IsUnitType(registry.require(self, 'Unit.isHero'), UNIT_TYPE_HERO) end
---@return string
function Unit:getHeroName() return GetHeroProperName(registry.require(self, 'Unit.getHeroName')) end
---@return integer
function Unit:getLevel() return GetHeroLevel(registry.require(self, 'Unit.getLevel')) end
---@param level integer
---@param showEffect boolean
function Unit:setLevel(level, showEffect) SetHeroLevel(registry.require(self, 'Unit.setLevel'), level, showEffect) end
---@return integer
function Unit:getXP() return GetHeroXP(registry.require(self, 'Unit.getXP')) end
---@param xp integer
---@param showEffect boolean
function Unit:setXP(xp, showEffect) SetHeroXP(registry.require(self, 'Unit.setXP'), xp, showEffect) end
---@param xp integer
---@param showEffect boolean
function Unit:addXP(xp, showEffect) AddHeroXP(registry.require(self, 'Unit.addXP'), xp, showEffect) end
---@param includeBonuses boolean
---@return integer
function Unit:getStr(includeBonuses) return GetHeroStr(registry.require(self, 'Unit.getStr'), includeBonuses) end
---@param value integer
---@param permanent boolean
function Unit:setStr(value, permanent) SetHeroStr(registry.require(self, 'Unit.setStr'), value, permanent) end
---@param includeBonuses boolean
---@return integer
function Unit:getAgi(includeBonuses) return GetHeroAgi(registry.require(self, 'Unit.getAgi'), includeBonuses) end
---@param value integer
---@param permanent boolean
function Unit:setAgi(value, permanent) SetHeroAgi(registry.require(self, 'Unit.setAgi'), value, permanent) end
---@param includeBonuses boolean
---@return integer
function Unit:getInt(includeBonuses) return GetHeroInt(registry.require(self, 'Unit.getInt'), includeBonuses) end
---@param value integer
---@param permanent boolean
function Unit:setInt(value, permanent) SetHeroInt(registry.require(self, 'Unit.setInt'), value, permanent) end
---@return integer
function Unit:getSkillPoints() return GetHeroSkillPoints(registry.require(self, 'Unit.getSkillPoints')) end
---@param delta integer
---@return boolean
function Unit:modifySkillPoints(delta)
    return UnitModifySkillPoints(registry.require(self, 'Unit.modifySkillPoints'), delta)
end
---@param abilityId integer
function Unit:selectSkill(abilityId) SelectHeroSkill(registry.require(self, 'Unit.selectSkill'), abilityId) end
---@param x number
---@param y number
---@param showEffect boolean
---@return boolean
function Unit:revive(x, y, showEffect) return ReviveHero(registry.require(self, 'Unit.revive'), x, y, showEffect) end

-- Abilities (integer ids).

---@param abilityId integer
---@return boolean
function Unit:addAbility(abilityId) return UnitAddAbility(registry.require(self, 'Unit.addAbility'), abilityId) end
---@param abilityId integer
---@return boolean
function Unit:removeAbility(abilityId)
    return UnitRemoveAbility(registry.require(self, 'Unit.removeAbility'), abilityId)
end
---@param abilityId integer
---@return integer
function Unit:getAbilityLevel(abilityId)
    return GetUnitAbilityLevel(registry.require(self, 'Unit.getAbilityLevel'), abilityId)
end
---@param abilityId integer
---@param level integer
---@return integer
function Unit:setAbilityLevel(abilityId, level)
    return SetUnitAbilityLevel(registry.require(self, 'Unit.setAbilityLevel'), abilityId, level)
end
---@param abilityId integer
---@param permanent boolean
---@return boolean
function Unit:makeAbilityPermanent(abilityId, permanent)
    return UnitMakeAbilityPermanent(registry.require(self, 'Unit.makeAbilityPermanent'), permanent, abilityId)
end
---@param abilityId integer
---@param hidden boolean
function Unit:hideAbility(abilityId, hidden)
    BlzUnitHideAbility(registry.require(self, 'Unit.hideAbility'), abilityId, hidden)
end
---@param abilityId integer
---@param disabled boolean
---@param hideUI boolean
function Unit:disableAbility(abilityId, disabled, hideUI)
    BlzUnitDisableAbility(registry.require(self, 'Unit.disableAbility'), abilityId, disabled, hideUI)
end
---@param abilityId integer
---@param seconds number
function Unit:startCooldown(abilityId, seconds)
    BlzStartUnitAbilityCooldown(registry.require(self, 'Unit.startCooldown'), abilityId, seconds)
end
---@param abilityId integer
function Unit:endCooldown(abilityId) BlzEndUnitAbilityCooldown(registry.require(self, 'Unit.endCooldown'), abilityId) end
---@param abilityId integer
---@return number
function Unit:getCooldownRemaining(abilityId)
    return BlzGetUnitAbilityCooldownRemaining(registry.require(self, 'Unit.getCooldownRemaining'), abilityId)
end

-- Inventory. Slots are zero-based.

---@param raw unit
---@param slot unknown
---@param operation string
local function checkSlot(raw, slot, operation)
    if type(slot) ~= 'number' or slot % 1 ~= 0 or slot < 0 or slot >= UnitInventorySize(raw) then
        error('[wrappers] ' .. operation .. ': expected an inventory slot index', 3)
    end
end
---@return integer
function Unit:getInventorySize() return UnitInventorySize(registry.require(self, 'Unit.getInventorySize')) end
---@param slot integer
---@return MoonwellWrappers.Item?
function Unit:getItemInSlot(slot)
    local raw = registry.require(self, 'Unit.getItemInSlot')
    checkSlot(raw, slot, 'Unit.getItemInSlot')
    return Item.fromHandle(UnitItemInSlot(raw, slot))
end
---@param item MoonwellWrappers.Item
---@return boolean
function Unit:addItem(item)
    local raw = registry.require(self, 'Unit.addItem')
    return UnitAddItem(raw, Handle.unwrap(item, 'Item', 'Unit.addItem'))
end
---@param typeId integer
---@return MoonwellWrappers.Item?
function Unit:addItemById(typeId)
    return Item.fromHandle(UnitAddItemById(registry.require(self, 'Unit.addItemById'), typeId))
end
---@param item MoonwellWrappers.Item
function Unit:removeItem(item)
    local raw = registry.require(self, 'Unit.removeItem')
    UnitRemoveItem(raw, Handle.unwrap(item, 'Item', 'Unit.removeItem'))
end
---@param slot integer
---@return MoonwellWrappers.Item?
function Unit:removeItemFromSlot(slot)
    local raw = registry.require(self, 'Unit.removeItemFromSlot')
    checkSlot(raw, slot, 'Unit.removeItemFromSlot')
    return Item.fromHandle(UnitRemoveItemFromSlot(raw, slot))
end
---@param item MoonwellWrappers.Item
---@return boolean
function Unit:hasItem(item)
    local raw = registry.require(self, 'Unit.hasItem')
    return UnitHasItem(raw, Handle.unwrap(item, 'Item', 'Unit.hasItem'))
end
---@param item MoonwellWrappers.Item
---@param x number
---@param y number
---@return boolean
function Unit:dropItemAt(item, x, y)
    local raw = registry.require(self, 'Unit.dropItemAt')
    return UnitDropItemPoint(raw, Handle.unwrap(item, 'Item', 'Unit.dropItemAt'), x, y)
end
---@param item MoonwellWrappers.Item
---@param slot integer
---@return boolean
function Unit:dropItemToSlot(item, slot)
    local raw = registry.require(self, 'Unit.dropItemToSlot')
    local rawItem = Handle.unwrap(item, 'Item', 'Unit.dropItemToSlot')
    checkSlot(raw, slot, 'Unit.dropItemToSlot')
    return UnitDropItemSlot(raw, rawItem, slot)
end
---@param item MoonwellWrappers.Item
---@return boolean
function Unit:useItem(item)
    local raw = registry.require(self, 'Unit.useItem')
    return UnitUseItem(raw, Handle.unwrap(item, 'Item', 'Unit.useItem'))
end

-- Orders.

---@param order string
---@return boolean
function Unit:issueOrder(order) return IssueImmediateOrder(registry.require(self, 'Unit.issueOrder'), order) end
---@param order string
---@param x number
---@param y number
---@return boolean
function Unit:issuePointOrder(order, x, y)
    return IssuePointOrder(registry.require(self, 'Unit.issuePointOrder'), order, x, y)
end
---@param order string
---@param target MoonwellWrappers.Widget
---@return boolean
function Unit:issueTargetOrder(order, target)
    local raw = registry.require(self, 'Unit.issueTargetOrder')
    return IssueTargetOrder(raw, order, Handle.unwrapWidget(target, 'Unit.issueTargetOrder'))
end
---@param orderId integer
---@return boolean
function Unit:issueOrderById(orderId)
    return IssueImmediateOrderById(registry.require(self, 'Unit.issueOrderById'), orderId)
end
---@param orderId integer
---@param x number
---@param y number
---@return boolean
function Unit:issuePointOrderById(orderId, x, y)
    return IssuePointOrderById(registry.require(self, 'Unit.issuePointOrderById'), orderId, x, y)
end
---@param orderId integer
---@param target MoonwellWrappers.Widget
---@return boolean
function Unit:issueTargetOrderById(orderId, target)
    local raw = registry.require(self, 'Unit.issueTargetOrderById')
    return IssueTargetOrderById(raw, orderId, Handle.unwrapWidget(target, 'Unit.issueTargetOrderById'))
end

-- Unit defines all four shared methods itself (GetUnitX/GetUnitY keep v0.1.0's mapping); install is a no-op here.
Widget.install(Unit, registry)
return Unit
```

In the existing test `'disposal is idempotent and guards all receiver operations'`, the disposed-method list keeps its
v0.1.0 names; it still passes because every listed method rejects a disposed receiver.

- [ ] **Step 4: Run all suites**

Run: `deno task test`
Expected: all suites pass. In `tests/imports.lua`, `item` is not loaded because no module in its require list imports
Unit.

- [ ] **Step 5: Run every check and commit**

Run the full check list. Expected: all pass.

```bash
git add src tests
git commit -m "feat: deepen Unit with hero, ability, inventory, state and order methods

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Player depth

**Files:**
- Modify: `src/wrappers/player.lua` (insert before `return PlayerWrapper`)
- Create: `tests/player.lua`

**Interfaces:**
- Produces: Player methods in spec §6. There is still no `Player.local`.

- [ ] **Step 1: Write the failing tests**

Create `tests/player.lua`:

```lua
PLAYER_STATE_RESOURCE_GOLD, PLAYER_STATE_RESOURCE_LUMBER = {}, {}
local Player = require('wrappers.player')
eq(totalCalls(), 0)

test('resources read, set and add through player state', function()
    local p, gold, lumber = Player.fromIndex(0), 100, 50
    native('GetPlayerState', function(_, state)
        if state == PLAYER_STATE_RESOURCE_GOLD then return gold end
        return lumber
    end)
    native('SetPlayerState', function(_, state, value)
        if state == PLAYER_STATE_RESOURCE_GOLD then gold = value else lumber = value end
    end)
    eq(p:getGold(), 100); expectCall('GetPlayerState', PLAYER_RAW, PLAYER_STATE_RESOURCE_GOLD)
    eq(p:getLumber(), 50); expectCall('GetPlayerState', PLAYER_RAW, PLAYER_STATE_RESOURCE_LUMBER)
    p:setGold(10); expectCall('SetPlayerState', PLAYER_RAW, PLAYER_STATE_RESOURCE_GOLD, 10)
    p:setLumber(20); expectCall('SetPlayerState', PLAYER_RAW, PLAYER_STATE_RESOURCE_LUMBER, 20)
    p:addGold(5); eq(gold, 15)
    p:addLumber(-5); eq(lumber, 15); expectCall('SetPlayerState', PLAYER_RAW, PLAYER_STATE_RESOURCE_LUMBER, 15)
end)

test('alliances take player wrappers', function()
    local p, other, setting = Player.fromIndex(0), Player.fromHandle({}), {}
    native('GetPlayerAlliance', function() return true end)
    eq(p:getAlliance(other, setting), true); expectCall('GetPlayerAlliance', PLAYER_RAW, other.handle, setting)
    native('SetPlayerAlliance', function() end)
    p:setAlliance(other, setting, false); expectCall('SetPlayerAlliance', PLAYER_RAW, other.handle, setting, false)
    native('IsPlayerAlly', function() return false end)
    eq(p:isAlly(other), false); expectCall('IsPlayerAlly', PLAYER_RAW, other.handle)
    native('IsPlayerEnemy', function() return true end)
    eq(p:isEnemy(other), true); expectCall('IsPlayerEnemy', PLAYER_RAW, other.handle)
    fails(function() p:isAlly({}) end, 'Player.isAlly: expected Player wrapper')
    fails(function() p:setAlliance({}, setting, true) end, 'Player.setAlliance: expected Player wrapper')
    eq(callCount('IsPlayerAlly'), 1); eq(callCount('SetPlayerAlliance'), 1)
end)

test('tech, slot and start location', function()
    local p, controller, slot, race = Player.fromIndex(0), {}, {}, {}
    checkGetters(p, {{'GetPlayerTechCount', 'getTechCount', 2, 1382118509, true},
        {'GetPlayerController', 'getController', controller}, {'GetPlayerSlotState', 'getSlotState', slot},
        {'GetPlayerRace', 'getRace', race}, {'GetPlayerTeam', 'getTeam', 1},
        {'GetPlayerStartLocationX', 'getStartX', -512}, {'GetPlayerStartLocationY', 'getStartY', 256}})
    checkSetters(p, {{'SetPlayerTechResearched', 'setTechResearched', 1382118509, 2},
        {'AddPlayerTechResearched', 'addTechResearched', 1382118509, 1},
        {'SetPlayerTechMaxAllowed', 'setTechMaxAllowed', 1382118509, 3},
        {'SetPlayerAbilityAvailable', 'setAbilityAvailable', 1097361000, false}})
end)

test('isLocal compares with the local player without a local factory', function()
    local p, other = Player.fromIndex(0), Player.fromHandle({})
    native('GetLocalPlayer', function() return PLAYER_RAW end)
    eq(p:isLocal(), true); eq(other:isLocal(), false); eq(callCount('GetLocalPlayer'), 2)
    eq(rawget(Player, 'local'), nil)
end)
```

- [ ] **Step 2: Run to verify failure**

Run: `deno task test player`
Expected: FAIL with `attempt to call a nil value (method 'getGold')`.

- [ ] **Step 3: Add the methods to `src/wrappers/player.lua`**

Insert before `return PlayerWrapper`:

```lua
---@return integer
function PlayerWrapper:getGold()
    return GetPlayerState(registry.require(self, 'Player.getGold'), PLAYER_STATE_RESOURCE_GOLD)
end
---@param value integer
function PlayerWrapper:setGold(value)
    SetPlayerState(registry.require(self, 'Player.setGold'), PLAYER_STATE_RESOURCE_GOLD, value)
end
---Reads the current gold, then sets it.
---@param amount integer
function PlayerWrapper:addGold(amount)
    local raw = registry.require(self, 'Player.addGold')
    SetPlayerState(raw, PLAYER_STATE_RESOURCE_GOLD, GetPlayerState(raw, PLAYER_STATE_RESOURCE_GOLD) + amount)
end
---@return integer
function PlayerWrapper:getLumber()
    return GetPlayerState(registry.require(self, 'Player.getLumber'), PLAYER_STATE_RESOURCE_LUMBER)
end
---@param value integer
function PlayerWrapper:setLumber(value)
    SetPlayerState(registry.require(self, 'Player.setLumber'), PLAYER_STATE_RESOURCE_LUMBER, value)
end
---Reads the current lumber, then sets it.
---@param amount integer
function PlayerWrapper:addLumber(amount)
    local raw = registry.require(self, 'Player.addLumber')
    SetPlayerState(raw, PLAYER_STATE_RESOURCE_LUMBER, GetPlayerState(raw, PLAYER_STATE_RESOURCE_LUMBER) + amount)
end
---@param other MoonwellWrappers.Player
---@param setting alliancetype
---@return boolean
function PlayerWrapper:getAlliance(other, setting)
    local raw = registry.require(self, 'Player.getAlliance')
    return GetPlayerAlliance(raw, registry.require(other, 'Player.getAlliance'), setting)
end
---@param other MoonwellWrappers.Player
---@param setting alliancetype
---@param value boolean
function PlayerWrapper:setAlliance(other, setting, value)
    local raw = registry.require(self, 'Player.setAlliance')
    SetPlayerAlliance(raw, registry.require(other, 'Player.setAlliance'), setting, value)
end
---@param other MoonwellWrappers.Player
---@return boolean
function PlayerWrapper:isAlly(other)
    local raw = registry.require(self, 'Player.isAlly')
    return IsPlayerAlly(raw, registry.require(other, 'Player.isAlly'))
end
---@param other MoonwellWrappers.Player
---@return boolean
function PlayerWrapper:isEnemy(other)
    local raw = registry.require(self, 'Player.isEnemy')
    return IsPlayerEnemy(raw, registry.require(other, 'Player.isEnemy'))
end
---@param techId integer
---@param specificOnly boolean
---@return integer
function PlayerWrapper:getTechCount(techId, specificOnly)
    return GetPlayerTechCount(registry.require(self, 'Player.getTechCount'), techId, specificOnly)
end
---@param techId integer
---@param level integer
function PlayerWrapper:setTechResearched(techId, level)
    SetPlayerTechResearched(registry.require(self, 'Player.setTechResearched'), techId, level)
end
---@param techId integer
---@param levels integer
function PlayerWrapper:addTechResearched(techId, levels)
    AddPlayerTechResearched(registry.require(self, 'Player.addTechResearched'), techId, levels)
end
---@param techId integer
---@param maximum integer
function PlayerWrapper:setTechMaxAllowed(techId, maximum)
    SetPlayerTechMaxAllowed(registry.require(self, 'Player.setTechMaxAllowed'), techId, maximum)
end
---@param abilityId integer
---@param available boolean
function PlayerWrapper:setAbilityAvailable(abilityId, available)
    SetPlayerAbilityAvailable(registry.require(self, 'Player.setAbilityAvailable'), abilityId, available)
end
---@return mapcontrol
function PlayerWrapper:getController() return GetPlayerController(registry.require(self, 'Player.getController')) end
---@return playerslotstate
function PlayerWrapper:getSlotState() return GetPlayerSlotState(registry.require(self, 'Player.getSlotState')) end
---@return race
function PlayerWrapper:getRace() return GetPlayerRace(registry.require(self, 'Player.getRace')) end
---@return integer
function PlayerWrapper:getTeam() return GetPlayerTeam(registry.require(self, 'Player.getTeam')) end
---@return number
function PlayerWrapper:getStartX() return GetPlayerStartLocationX(registry.require(self, 'Player.getStartX')) end
---@return number
function PlayerWrapper:getStartY() return GetPlayerStartLocationY(registry.require(self, 'Player.getStartY')) end
---True only on the machine of this player. Branching on it must not change synchronized game state.
---@return boolean
function PlayerWrapper:isLocal() return registry.require(self, 'Player.isLocal') == GetLocalPlayer() end
```

- [ ] **Step 4: Run all suites**

Run: `deno task test`
Expected: all suites pass.

- [ ] **Step 5: Run every check and commit**

Run the full check list. Expected: all pass.

```bash
git add src tests
git commit -m "feat: deepen Player with resources, alliances, tech and slot methods

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: Trigger registrations, conditions and tokens

**Files:**
- Modify: `src/wrappers/internal/callback.lua` (whole file below)
- Modify: `src/wrappers/trigger.lua` (whole file below)
- Modify: `tests/trigger.lua` (replace the prologue; append tests)

**Interfaces:**
- Consumes: `Handle.unwrap`, `Handle.unwrapWidget`, `Handle.created`.
- Produces: `Callback.test`; `MoonwellWrappers.TriggerAction`, `MoonwellWrappers.TriggerCondition`; the Trigger methods
  in spec §7. `addAction` returns a `TriggerAction`.

- [ ] **Step 1: Write the failing tests**

Replace the first four lines of `tests/trigger.lua` (`local actions = {}` through `native('DestroyTrigger', ...)`)
with:

```lua
local actions, conditions = {}, {}
local lastAction, lastBoolexpr, lastCondition
native('CreateTrigger', function() return {} end)
native('TriggerAddAction', function(_, callback)
    actions[#actions + 1] = callback; lastAction = {}; return lastAction
end)
native('Condition', function(fn) conditions[#conditions + 1] = fn; lastBoolexpr = {}; return lastBoolexpr end)
native('TriggerAddCondition', function() lastCondition = {}; return lastCondition end)
for _, name in ipairs({'DestroyTrigger', 'TriggerRemoveAction', 'TriggerClearActions', 'TriggerRemoveCondition',
    'TriggerClearConditions', 'DestroyCondition', 'CreateRegion', 'RemoveRegion', 'RemoveItem'}) do
    native(name, function() end)
end
native('CreateRegion', function() return {} end)
```

After the existing `local Player = require('wrappers.player')` line add:

```lua
local Region = require('wrappers.region')
local Item = require('wrappers.item')
```

Append to `tests/trigger.lua`:

```lua
test('new registrations forward exact arguments', function()
    local t, u, p, g, item = Trigger.create(), Unit.fromHandle({}), Player.fromIndex(0), Region.create(),
        Item.fromHandle({})
    local event, state, op = {}, {}, {}
    for _, name in ipairs({'TriggerRegisterPlayerUnitEvent', 'TriggerRegisterPlayerEvent',
        'TriggerRegisterPlayerChatEvent', 'TriggerRegisterEnterRegion', 'TriggerRegisterLeaveRegion',
        'TriggerRegisterDeathEvent', 'TriggerRegisterUnitInRange', 'TriggerRegisterUnitStateEvent',
        'TriggerRegisterGameEvent', 'TriggerExecute'}) do native(name, function() end) end
    resetCalls()
    t:registerAnyUnitEvent(event)
    eq(callCount('TriggerRegisterPlayerUnitEvent'), bj_MAX_PLAYER_SLOTS); eq(callCount('Player'), bj_MAX_PLAYER_SLOTS)
    expectCall('Player', bj_MAX_PLAYER_SLOTS - 1)
    expectCall('TriggerRegisterPlayerUnitEvent', t.handle, PLAYER_RAW, event, nil)
    t:registerPlayerEvent(p, event); expectCall('TriggerRegisterPlayerEvent', t.handle, PLAYER_RAW, event)
    t:registerChatEvent(p, '-go', true); expectCall('TriggerRegisterPlayerChatEvent', t.handle, PLAYER_RAW, '-go', true)
    t:registerEnterRegion(g); expectCall('TriggerRegisterEnterRegion', t.handle, g.handle, nil)
    t:registerLeaveRegion(g); expectCall('TriggerRegisterLeaveRegion', t.handle, g.handle, nil)
    t:registerDeathEvent(item); expectCall('TriggerRegisterDeathEvent', t.handle, item.handle)
    t:registerDeathEvent(u); expectCall('TriggerRegisterDeathEvent', t.handle, u.handle)
    t:registerUnitInRange(u, 300); expectCall('TriggerRegisterUnitInRange', t.handle, u.handle, 300, nil)
    t:registerUnitStateEvent(u, state, op, 50)
    expectCall('TriggerRegisterUnitStateEvent', t.handle, u.handle, state, op, 50)
    t:registerGameEvent(event); expectCall('TriggerRegisterGameEvent', t.handle, event)
    native('TriggerEvaluate', function() return false end)
    eq(t:evaluate(), false); expectCall('TriggerEvaluate', t.handle)
    t:execute(); expectCall('TriggerExecute', t.handle)
    t:destroy(); g:destroy(); item:remove(); u:remove()
end)

test('new registrations validate before natives', function()
    local t, u, p, item = Trigger.create(), Unit.fromHandle({}), Player.fromIndex(0), Item.fromHandle({})
    resetCalls()
    fails(function() t:registerEnterRegion(u) end, 'Trigger.registerEnterRegion: expected Region wrapper')
    fails(function() t:registerLeaveRegion(p) end, 'Trigger.registerLeaveRegion: expected Region wrapper')
    fails(function() t:registerDeathEvent(p) end, 'Trigger.registerDeathEvent: expected Widget wrapper')
    fails(function() t:registerChatEvent(u, 'x', true) end, 'Trigger.registerChatEvent: expected Player wrapper')
    fails(function() t:registerPlayerEvent(u, {}) end, 'Trigger.registerPlayerEvent: expected Player wrapper')
    fails(function() t:registerUnitStateEvent(p, {}, {}, 1) end, 'Trigger.registerUnitStateEvent: expected Unit wrapper')
    for _, bad in ipairs({-1, math.huge, 0/0, '1'}) do
        fails(function() t:registerUnitInRange(u, bad) end, 'Trigger.registerUnitInRange')
    end
    fails(function() t:addCondition(nil) end, 'callback')
    item:remove()
    fails(function() t:registerDeathEvent(item) end, 'Trigger.registerDeathEvent: Item is disposed')
    eq(callCount('RemoveItem'), 1); eq(totalCalls(), 1)
    t:destroy(); u:remove()
end)

test('conditions own boolexprs and failures evaluate false', function()
    local t, pass = Trigger.create(), nil
    local token = t:addCondition(function(self) eq(self, t); return pass end)
    local boolexpr, nativeCondition, check = lastBoolexpr, lastCondition, conditions[#conditions]
    expectCall('TriggerAddCondition', t.handle, boolexpr)
    eq(check(), false)
    pass = 1; eq(check(), true)
    t:addCondition(function() error('condition probe') end)
    eq(conditions[#conditions](), false); eq(#PRINTED, 1)
    assert(PRINTED[1]:find('[wrappers] Trigger condition failed:', 1, true))
    assert(PRINTED[1]:find('condition probe', 1, true))
    t:removeCondition(token)
    expectCall('TriggerRemoveCondition', t.handle, nativeCondition); expectCall('DestroyCondition', boolexpr)
    eq(check(), false)
    t:removeCondition(token); eq(callCount('TriggerRemoveCondition'), 1); eq(callCount('DestroyCondition'), 1)
    native('Condition', function() return nil end)
    fails(function() t:addCondition(function() return true end) end, 'Trigger.addCondition')
    native('Condition', function(fn) conditions[#conditions + 1] = fn; lastBoolexpr = {}; return lastBoolexpr end)
    t:destroy()
end)

test('action tokens remove individual actions, even mid-firing', function()
    local t, hits, second = Trigger.create(), {}, nil
    t:addAction(function(self) hits[#hits + 1] = 'first'; self:removeAction(second) end)
    local runFirst = actions[#actions]
    second = t:addAction(function() hits[#hits + 1] = 'second' end)
    local runSecond, secondNative = actions[#actions], lastAction
    runFirst(); runSecond()
    eq(#hits, 1); eq(hits[1], 'first')
    expectCall('TriggerRemoveAction', t.handle, secondNative)
    t:removeAction(second); eq(callCount('TriggerRemoveAction'), 1)
    t:destroy()
end)

test('tokens are checked for kind and owner', function()
    local a, b = Trigger.create(), Trigger.create()
    local action, condition = a:addAction(function() end), a:addCondition(function() return true end)
    resetCalls()
    fails(function() b:removeAction(action) end, 'Trigger.removeAction: token belongs to another trigger')
    fails(function() a:removeAction(condition) end, 'Trigger.removeAction: expected TriggerAction token')
    fails(function() a:removeCondition(action) end, 'Trigger.removeCondition: expected TriggerCondition token')
    fails(function() a:removeAction({}) end, 'Trigger.removeAction: expected TriggerAction token')
    eq(totalCalls(), 0)
    a:destroy(); b:destroy()
end)

test('clear operations release callbacks and owned boolexprs', function()
    local t, hits = Trigger.create(), 0
    local action = t:addAction(function() hits = hits + 1 end)
    local runAction = actions[#actions]
    local kept = t:addCondition(function() return true end)
    local keptBoolexpr, keptCheck = lastBoolexpr, conditions[#conditions]
    local removed = t:addCondition(function() return true end)
    t:removeCondition(removed)
    resetCalls()
    t:clearConditions()
    eq(callName(1), 'TriggerClearConditions'); expectCall('TriggerClearConditions', t.handle)
    expectCall('DestroyCondition', keptBoolexpr); eq(callCount('DestroyCondition'), 1)
    eq(keptCheck(), false)
    t:removeCondition(kept); eq(callCount('TriggerRemoveCondition'), 0)
    t:clearActions(); expectCall('TriggerClearActions', t.handle)
    runAction(); eq(hits, 0)
    t:removeAction(action); eq(callCount('TriggerRemoveAction'), 0)
    t:destroy()
end)

test('destroy clears conditions, destroys boolexprs, then the trigger', function()
    local t = Trigger.create()
    local condition = t:addCondition(function() return true end)
    local boolexpr = lastBoolexpr
    t:addAction(function() end)
    local raw = t.handle
    resetCalls()
    t:destroy(); t:destroy()
    eq(callName(1), 'TriggerClearConditions'); eq(callName(2), 'DestroyCondition'); eq(callName(3), 'DestroyTrigger')
    eq(totalCalls(), 3)
    expectCall('TriggerClearConditions', raw); expectCall('DestroyCondition', boolexpr); expectCall('DestroyTrigger', raw)
    fails(function() t:removeCondition(condition) end, 'disposed')
    checkDisposed(t, {'registerAnyUnitEvent', 'registerPlayerEvent', 'registerChatEvent', 'registerEnterRegion',
        'registerLeaveRegion', 'registerDeathEvent', 'registerUnitInRange', 'registerUnitStateEvent',
        'registerGameEvent', 'addCondition', 'removeAction', 'removeCondition', 'clearActions', 'clearConditions',
        'evaluate', 'execute'})
end)
```

- [ ] **Step 2: Run to verify failure**

Run: `deno task test trigger`
Expected: FAIL — the four v0.1.0 tests pass; the new ones fail with nil methods (`registerAnyUnitEvent`,
`addCondition`, `removeAction`).

- [ ] **Step 3: Replace `src/wrappers/internal/callback.lua`**

```lua
local Callback = {}

---@param value unknown
---@param operation string
function Callback.check(value, operation)
    if type(value) ~= 'function' then error('[wrappers] ' .. operation .. ': expected a callback function', 3) end
end

---@param value unknown
---@param operation string
function Callback.nonnegative(value, operation)
    if type(value) ~= 'number' or value ~= value or value < 0 or value == math.huge then
        error('[wrappers] ' .. operation .. ': expected a finite non-negative number', 3)
    end
end

---@param label string
---@param message unknown
local function report(label, message)
    local printable, text = pcall(tostring, message)
    print('[wrappers] ' .. label .. ' failed: ' .. (printable and text or '<unprintable error>'))
end

---@generic T
---@param label string
---@param fn fun(value: T): ...
---@param argument T
function Callback.call(label, fn, argument)
    local ok, message = pcall(fn, argument)
    if not ok then report(label .. ' callback', message) end
end

---Runs a predicate behind the callback boundary: an error is printed and counts as false.
---@generic T
---@param label string
---@param fn fun(value: T): any
---@param argument T
---@return boolean
function Callback.test(label, fn, argument)
    local ok, result = pcall(fn, argument)
    if not ok then
        report(label, result)
        return false
    end
    return not not result
end

return Callback
```

- [ ] **Step 4: Replace `src/wrappers/trigger.lua`**

```lua
local Handle = require('wrappers.internal.handle')
local Callback = require('wrappers.internal.callback')

---Opaque token returned by Trigger:addAction; pass it to Trigger:removeAction.
---@class MoonwellWrappers.TriggerAction

---Opaque token returned by Trigger:addCondition; pass it to Trigger:removeCondition.
---@class MoonwellWrappers.TriggerCondition

---@class MoonwellWrappers.Trigger
---@field handle trigger? Read-only by convention; nil after destruction.
local Trigger = {}
---@type MoonwellWrappers.Registry<MoonwellWrappers.Trigger, trigger>
local registry = Handle.new(Trigger, 'Trigger')

---@class MoonwellWrappers.TriggerCell
---@field trigger MoonwellWrappers.Trigger
---@field kind 'TriggerAction'|'TriggerCondition'
---@field native? any
---@field callback (fun(trigger: MoonwellWrappers.Trigger): ...)?
---@field predicate (fun(trigger: MoonwellWrappers.Trigger): any)?
---@field boolexpr conditionfunc?

-- Arrays, not sets: clearing calls natives in order, and pairs order over table keys differs between clients.
---@type table<MoonwellWrappers.Trigger, {actions: MoonwellWrappers.TriggerCell[], conditions: MoonwellWrappers.TriggerCell[]}>
local states = {}
---@type table<table, MoonwellWrappers.TriggerCell>
local cells = setmetatable({}, {__mode = 'k'})

---@param trigger MoonwellWrappers.Trigger
local function stateOf(trigger)
    local state = states[trigger]
    if not state then
        state = {actions = {}, conditions = {}}
        states[trigger] = state
    end
    return state
end

---@param list MoonwellWrappers.TriggerCell[]
---@param cell MoonwellWrappers.TriggerCell
---@return MoonwellWrappers.TriggerCell[]
local function without(list, cell)
    local result = {}
    for _, item in ipairs(list) do
        if item ~= cell then result[#result + 1] = item end
    end
    return result
end

---@param trigger MoonwellWrappers.Trigger
---@param token unknown
---@param kind string
---@param operation string
---@return MoonwellWrappers.TriggerCell
local function ownedCell(trigger, token, kind, operation)
    local cell = cells[token]
    if not cell or cell.kind ~= kind then error('[wrappers] ' .. operation .. ': expected ' .. kind .. ' token', 3) end
    if cell.trigger ~= trigger then error('[wrappers] ' .. operation .. ': token belongs to another trigger', 3) end
    return cell
end

---Makes every condition closure a no-op and returns the boolexprs to destroy, in insertion order.
---@param state {conditions: MoonwellWrappers.TriggerCell[]}
---@return conditionfunc[]
local function releaseConditions(state)
    local boolexprs = {}
    for _, cell in ipairs(state.conditions) do
        cell.predicate = nil
        boolexprs[#boolexprs + 1] = cell.boolexpr
    end
    state.conditions = {}
    return boolexprs
end

---@param raw trigger?
---@return MoonwellWrappers.Trigger?
---@overload fun(raw: nil): nil
function Trigger.fromHandle(raw) return registry.wrap(raw) end
---@return MoonwellWrappers.Trigger
function Trigger.create() return Handle.created(Trigger.fromHandle(CreateTrigger()), 'Trigger.create') end
---@return trigger
function Trigger:getHandle() return registry.require(self, 'Trigger.getHandle') end
---@return boolean
function Trigger:isDisposed() return registry.isDisposed(self, 'Trigger.isDisposed') end
function Trigger:enable() EnableTrigger(registry.require(self, 'Trigger.enable')) end
function Trigger:disable() DisableTrigger(registry.require(self, 'Trigger.disable')) end
---@return boolean
function Trigger:isEnabled() return IsTriggerEnabled(registry.require(self, 'Trigger.isEnabled')) end
---@return boolean
function Trigger:evaluate() return TriggerEvaluate(registry.require(self, 'Trigger.evaluate')) end
function Trigger:execute() TriggerExecute(registry.require(self, 'Trigger.execute')) end

---@param unit MoonwellWrappers.Unit
---@param event unitevent
function Trigger:registerUnitEvent(unit, event)
    local raw = registry.require(self, 'Trigger.registerUnitEvent')
    TriggerRegisterUnitEvent(raw, Handle.unwrap(unit, 'Unit', 'Trigger.registerUnitEvent'), event)
end
---@param player MoonwellWrappers.Player
---@param event playerunitevent
function Trigger:registerPlayerUnitEvent(player, event)
    local raw = registry.require(self, 'Trigger.registerPlayerUnitEvent')
    local rawPlayer = Handle.unwrap(player, 'Player', 'Trigger.registerPlayerUnitEvent')
    -- Warcraft accepts a null filter; the generated JASS signature cannot express that.
    ---@diagnostic disable-next-line: param-type-mismatch
    TriggerRegisterPlayerUnitEvent(raw, rawPlayer, event, nil)
end
---Registers the event for every player slot, like TriggerRegisterAnyUnitEventBJ.
---@param event playerunitevent
function Trigger:registerAnyUnitEvent(event)
    local raw = registry.require(self, 'Trigger.registerAnyUnitEvent')
    for index = 0, bj_MAX_PLAYER_SLOTS - 1 do
        -- Warcraft accepts a null filter; the generated JASS signature cannot express that.
        ---@diagnostic disable-next-line: param-type-mismatch
        TriggerRegisterPlayerUnitEvent(raw, Player(index), event, nil)
    end
end
---@param player MoonwellWrappers.Player
---@param event playerevent
function Trigger:registerPlayerEvent(player, event)
    local raw = registry.require(self, 'Trigger.registerPlayerEvent')
    TriggerRegisterPlayerEvent(raw, Handle.unwrap(player, 'Player', 'Trigger.registerPlayerEvent'), event)
end
---@param player MoonwellWrappers.Player
---@param text string
---@param exactMatch boolean
function Trigger:registerChatEvent(player, text, exactMatch)
    local raw = registry.require(self, 'Trigger.registerChatEvent')
    local rawPlayer = Handle.unwrap(player, 'Player', 'Trigger.registerChatEvent')
    TriggerRegisterPlayerChatEvent(raw, rawPlayer, text, exactMatch)
end
---@param region MoonwellWrappers.Region
function Trigger:registerEnterRegion(region)
    local raw = registry.require(self, 'Trigger.registerEnterRegion')
    local rawRegion = Handle.unwrap(region, 'Region', 'Trigger.registerEnterRegion')
    -- Warcraft accepts a null filter; the generated JASS signature cannot express that.
    ---@diagnostic disable-next-line: param-type-mismatch
    TriggerRegisterEnterRegion(raw, rawRegion, nil)
end
---@param region MoonwellWrappers.Region
function Trigger:registerLeaveRegion(region)
    local raw = registry.require(self, 'Trigger.registerLeaveRegion')
    local rawRegion = Handle.unwrap(region, 'Region', 'Trigger.registerLeaveRegion')
    -- Warcraft accepts a null filter; the generated JASS signature cannot express that.
    ---@diagnostic disable-next-line: param-type-mismatch
    TriggerRegisterLeaveRegion(raw, rawRegion, nil)
end
---@param widget MoonwellWrappers.Widget
function Trigger:registerDeathEvent(widget)
    local raw = registry.require(self, 'Trigger.registerDeathEvent')
    TriggerRegisterDeathEvent(raw, Handle.unwrapWidget(widget, 'Trigger.registerDeathEvent'))
end
---@param unit MoonwellWrappers.Unit
---@param range number
function Trigger:registerUnitInRange(unit, range)
    local raw = registry.require(self, 'Trigger.registerUnitInRange')
    local rawUnit = Handle.unwrap(unit, 'Unit', 'Trigger.registerUnitInRange')
    Callback.nonnegative(range, 'Trigger.registerUnitInRange')
    -- Warcraft accepts a null filter; the generated JASS signature cannot express that.
    ---@diagnostic disable-next-line: param-type-mismatch
    TriggerRegisterUnitInRange(raw, rawUnit, range, nil)
end
---@param unit MoonwellWrappers.Unit
---@param state unitstate
---@param op limitop
---@param value number
function Trigger:registerUnitStateEvent(unit, state, op, value)
    local raw = registry.require(self, 'Trigger.registerUnitStateEvent')
    TriggerRegisterUnitStateEvent(raw, Handle.unwrap(unit, 'Unit', 'Trigger.registerUnitStateEvent'), state, op, value)
end
---@param timeout number
---@param periodic boolean
function Trigger:registerTimerEvent(timeout, periodic)
    local raw = registry.require(self, 'Trigger.registerTimerEvent')
    Callback.nonnegative(timeout, 'Trigger.registerTimerEvent')
    TriggerRegisterTimerEvent(raw, timeout, periodic)
end
---@param event gameevent
function Trigger:registerGameEvent(event)
    TriggerRegisterGameEvent(registry.require(self, 'Trigger.registerGameEvent'), event)
end

---@param callback fun(trigger: MoonwellWrappers.Trigger): ...
---@return MoonwellWrappers.TriggerAction
function Trigger:addAction(callback)
    local raw = registry.require(self, 'Trigger.addAction')
    Callback.check(callback, 'Trigger.addAction')
    ---@type MoonwellWrappers.TriggerCell
    local cell = {trigger = self, kind = 'TriggerAction', callback = callback}
    local state = stateOf(self)
    state.actions[#state.actions + 1] = cell
    cell.native = TriggerAddAction(raw, function()
        local current = cell.callback
        if current then Callback.call('Trigger', current, self) end
    end)
    ---@type MoonwellWrappers.TriggerAction
    local token = {}
    cells[token] = cell
    return token
end
---Removing a token twice, or after clearActions, does nothing.
---@param token MoonwellWrappers.TriggerAction
function Trigger:removeAction(token)
    local raw = registry.require(self, 'Trigger.removeAction')
    local cell = ownedCell(self, token, 'TriggerAction', 'Trigger.removeAction')
    if not cell.callback then return end
    cell.callback = nil
    local state = stateOf(self)
    state.actions = without(state.actions, cell)
    TriggerRemoveAction(raw, cell.native)
end
function Trigger:clearActions()
    local raw = registry.require(self, 'Trigger.clearActions')
    local state = stateOf(self)
    for _, cell in ipairs(state.actions) do cell.callback = nil end
    state.actions = {}
    TriggerClearActions(raw)
end

---The predicate's result counts as truthy or falsy. An error is printed and counts as false.
---@param predicate fun(trigger: MoonwellWrappers.Trigger): any
---@return MoonwellWrappers.TriggerCondition
function Trigger:addCondition(predicate)
    local raw = registry.require(self, 'Trigger.addCondition')
    Callback.check(predicate, 'Trigger.addCondition')
    ---@type MoonwellWrappers.TriggerCell
    local cell = {trigger = self, kind = 'TriggerCondition', predicate = predicate}
    cell.boolexpr = Handle.created(Condition(function()
        local current = cell.predicate
        if not current then return false end
        return Callback.test('Trigger condition', current, self)
    end), 'Trigger.addCondition')
    local state = stateOf(self)
    state.conditions[#state.conditions + 1] = cell
    cell.native = TriggerAddCondition(raw, cell.boolexpr)
    ---@type MoonwellWrappers.TriggerCondition
    local token = {}
    cells[token] = cell
    return token
end
---Removing a token twice, or after clearConditions, does nothing.
---@param token MoonwellWrappers.TriggerCondition
function Trigger:removeCondition(token)
    local raw = registry.require(self, 'Trigger.removeCondition')
    local cell = ownedCell(self, token, 'TriggerCondition', 'Trigger.removeCondition')
    if not cell.predicate then return end
    cell.predicate = nil
    local state = stateOf(self)
    state.conditions = without(state.conditions, cell)
    TriggerRemoveCondition(raw, cell.native)
    DestroyCondition(cell.boolexpr)
end
function Trigger:clearConditions()
    local raw = registry.require(self, 'Trigger.clearConditions')
    local boolexprs = releaseConditions(stateOf(self))
    TriggerClearConditions(raw)
    for _, boolexpr in ipairs(boolexprs) do DestroyCondition(boolexpr) end
end

function Trigger:destroy()
    local raw = registry.dispose(self, 'Trigger.destroy')
    if not raw then return end
    local state = stateOf(self)
    states[self] = nil
    for _, cell in ipairs(state.actions) do cell.callback = nil end
    local boolexprs = releaseConditions(state)
    TriggerClearConditions(raw)
    for _, boolexpr in ipairs(boolexprs) do DestroyCondition(boolexpr) end
    DestroyTrigger(raw)
end

return Trigger
```

- [ ] **Step 4b: Check the v0.1.0 trigger tests still hold**

The v0.1.0 test `'trigger identity and exact native registrations'` asserts `eq(callCount('DestroyTrigger'), 1)` and
`expectCall('DestroyTrigger', raw)`; both still hold. `'self destruction suppresses other retained actions'` swaps the
DestroyTrigger double; TriggerClearConditions (a no-op double) now runs first, which the test does not constrain.

- [ ] **Step 5: Run all suites**

Run: `deno task test`
Expected: all suites pass.

- [ ] **Step 6: Run every check and commit**

Run the full check list. Expected: all pass (`Player(index)` in `registerAnyUnitEvent` resolves to the native, since
trigger.lua no longer has a local named Player).

```bash
git add src tests
git commit -m "feat: trigger events, predicate conditions and removable actions

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: Group enumerations and helpers

**Files:**
- Modify: `src/wrappers/group.lua` (whole file below)
- Modify: `tests/group.lua` (prologue natives; append tests)

**Interfaces:**
- Consumes: `Handle.unwrap`, `Callback.check`, `Callback.nonnegative`, `MoonwellWrappers.Rect`.
- Produces: `enumInRect`, `enumOfPlayer`, `enumSelected`, the optional `filter` on `enumInRange`, `forEach`, `first`.

- [ ] **Step 1: Write the failing tests**

At the top of `tests/group.lua` add:

```lua
native('Rect', function() return {} end)
native('RemoveRect', function() end)
```

After `local Unit = require('wrappers.unit')` add:

```lua
local Rect = require('wrappers.rect')
local Player = require('wrappers.player')

-- A native group double backed by an array; every enumeration native appends `initial`.
local function nativeGroup(initial)
    local members = {}
    native('GroupClear', function() members = {} end)
    native('BlzGroupGetSize', function() return #members end)
    native('BlzGroupUnitAt', function(_, index) return members[index + 1] end)
    native('GroupAddUnit', function(_, raw) members[#members + 1] = raw end)
    native('GroupRemoveUnit', function(_, raw)
        for i = #members, 1, -1 do if members[i] == raw then table.remove(members, i) end end
    end)
    local function enum() for _, raw in ipairs(initial) do members[#members + 1] = raw end end
    for _, name in ipairs({'GroupEnumUnitsInRange', 'GroupEnumUnitsInRect', 'GroupEnumUnitsOfPlayer',
        'GroupEnumUnitsSelected'}) do native(name, enum) end
    return function() return members end
end
```

Append:

```lua
test('enumerations clear first and forward exact arguments', function()
    local g, r, p = Group.create(), Rect.create(0, 0, 1, 1), Player.fromIndex(0)
    nativeGroup({})
    resetCalls(); g:enumInRect(r)
    eq(callName(1), 'GroupClear'); expectCall('GroupEnumUnitsInRect', g.handle, r.handle, nil)
    resetCalls(); g:enumOfPlayer(p)
    eq(callName(1), 'GroupClear'); expectCall('GroupEnumUnitsOfPlayer', g.handle, PLAYER_RAW, nil)
    resetCalls(); g:enumSelected(p)
    eq(callName(1), 'GroupClear'); expectCall('GroupEnumUnitsSelected', g.handle, PLAYER_RAW, nil)
    resetCalls()
    fails(function() g:enumInRect(p) end, 'Group.enumInRect: expected Rect wrapper')
    fails(function() g:enumOfPlayer(r) end, 'Group.enumOfPlayer: expected Player wrapper')
    fails(function() g:enumInRange(0, 0, 10, 'filter') end, 'Group.enumInRange: expected a callback function')
    fails(function() g:enumSelected(p, 5) end, 'Group.enumSelected: expected a callback function')
    eq(totalCalls(), 0)
    g:destroy(); r:destroy()
end)

test('filters run after native enumeration and remove rejected units', function()
    local a, b, c = {}, {}, {}
    local members = nativeGroup({a, b, c})
    local g, seen = Group.create(), {}
    resetCalls()
    g:enumInRange(0, 0, 500, function(unit) seen[#seen + 1] = unit; return unit.handle ~= b end)
    eq(callName(1), 'GroupClear'); eq(callName(2), 'GroupEnumUnitsInRange')
    expectCall('GroupEnumUnitsInRange', g.handle, 0, 0, 500, nil)
    eq(#seen, 3); eq(seen[1], Unit.fromHandle(a))
    eq(#members(), 2); eq(members()[1], a); eq(members()[2], c)
    expectCall('GroupRemoveUnit', g.handle, b)
    g:enumInRange(0, 0, 500)
    eq(#members(), 3)
    g:destroy()
end)

test('a failing filter clears the group and re-raises', function()
    local members = nativeGroup({{}, {}})
    local g = Group.create()
    fails(function() g:enumOfPlayer(Player.fromIndex(0), function() error('filter probe') end) end, 'filter probe')
    eq(#members(), 0); eq(#PRINTED, 0)
    g:destroy()
end)

test('filters that change the group see a stable snapshot', function()
    nativeGroup({{}, {}})
    local g, extra, count = Group.create(), Unit.fromHandle({}), 0
    g:enumInRange(0, 0, 1, function() count = count + 1; g:add(extra); return true end)
    eq(count, 2)
    extra:remove(); g:destroy()
end)

test('forEach iterates a snapshot and first uses FirstOfGroup', function()
    local a, b = {}, {}
    nativeGroup({a, b})
    local g, seen = Group.create(), {}
    g:enumInRange(0, 0, 1)
    g:forEach(function(unit) seen[#seen + 1] = unit; g:clear() end)
    eq(#seen, 2); eq(seen[2], Unit.fromHandle(b))
    fails(function() g:forEach(nil) end, 'Group.forEach: expected a callback function')
    g:enumInRange(0, 0, 1)
    fails(function() g:forEach(function() error('each probe') end) end, 'each probe')
    native('FirstOfGroup', function() return a end)
    eq(g:first(), Unit.fromHandle(a)); expectCall('FirstOfGroup', g.handle)
    native('FirstOfGroup', function() return nil end)
    eq(g:first(), nil)
    g:destroy()
    checkDisposed(g, {'enumInRect', 'enumOfPlayer', 'enumSelected', 'forEach', 'first'})
end)
```

- [ ] **Step 2: Run to verify failure**

Run: `deno task test group`
Expected: FAIL — `enumInRect` is nil; the filter test fails because the v0.1.0 `enumInRange` ignores its fourth argument.

- [ ] **Step 3: Replace `src/wrappers/group.lua`**

```lua
local Handle = require('wrappers.internal.handle')
local Callback = require('wrappers.internal.callback')
local Unit = require('wrappers.unit')

---@class MoonwellWrappers.Group
---@field handle group? Read-only by convention; nil after destruction.
local Group = {}
---@type MoonwellWrappers.Registry<MoonwellWrappers.Group, group>
local registry = Handle.new(Group, 'Group')

---Raw member handles in native order, skipping nil entries.
---@param raw group
---@return unit[]
local function members(raw)
    local result = {}
    for index = 0, BlzGroupGetSize(raw) - 1 do
        local unit = BlzGroupUnitAt(raw, index)
        if unit then result[#result + 1] = unit end
    end
    return result
end

---@param filter unknown
---@param operation string
local function checkFilter(filter, operation)
    if filter ~= nil then Callback.check(filter, operation) end
end

---Runs the filter over a snapshot, then removes rejected units. If the filter raises, the group is cleared and the
---error re-raised, so a half-filtered group never escapes.
---@param raw group
---@param filter (fun(unit: MoonwellWrappers.Unit): any)?
local function applyFilter(raw, filter)
    if filter == nil then return end
    local rejected = {}
    local ok, message = pcall(function()
        for _, unit in ipairs(members(raw)) do
            if not filter(assert(Unit.fromHandle(unit))) then rejected[#rejected + 1] = unit end
        end
    end)
    if not ok then
        GroupClear(raw)
        error(message, 0)
    end
    for _, unit in ipairs(rejected) do GroupRemoveUnit(raw, unit) end
end

---@param raw group?
---@return MoonwellWrappers.Group?
---@overload fun(raw: nil): nil
function Group.fromHandle(raw) return registry.wrap(raw) end
---@return MoonwellWrappers.Group
function Group.create() return Handle.created(Group.fromHandle(CreateGroup()), 'Group.create') end
---@return group
function Group:getHandle() return registry.require(self, 'Group.getHandle') end
---@return boolean
function Group:isDisposed() return registry.isDisposed(self, 'Group.isDisposed') end
---@param unit MoonwellWrappers.Unit
function Group:add(unit)
    local raw = registry.require(self, 'Group.add')
    GroupAddUnit(raw, Handle.unwrap(unit, 'Unit', 'Group.add'))
end
---@param unit MoonwellWrappers.Unit
function Group:remove(unit)
    local raw = registry.require(self, 'Group.remove')
    GroupRemoveUnit(raw, Handle.unwrap(unit, 'Unit', 'Group.remove'))
end
---@param unit MoonwellWrappers.Unit
---@return boolean
function Group:contains(unit)
    local raw = registry.require(self, 'Group.contains')
    return IsUnitInGroup(Handle.unwrap(unit, 'Unit', 'Group.contains'), raw)
end
function Group:clear() GroupClear(registry.require(self, 'Group.clear')) end
---Clears the group, then adds the units within radius; `filter` keeps units for which it returns truthy.
---@param x number
---@param y number
---@param radius number
---@param filter (fun(unit: MoonwellWrappers.Unit): any)?
function Group:enumInRange(x, y, radius, filter)
    local raw = registry.require(self, 'Group.enumInRange')
    Callback.nonnegative(radius, 'Group.enumInRange')
    checkFilter(filter, 'Group.enumInRange')
    GroupClear(raw)
    -- Warcraft accepts a null filter; the generated JASS signature cannot express that.
    ---@diagnostic disable-next-line: param-type-mismatch
    GroupEnumUnitsInRange(raw, x, y, radius, nil)
    applyFilter(raw, filter)
end
---@param rect MoonwellWrappers.Rect
---@param filter (fun(unit: MoonwellWrappers.Unit): any)?
function Group:enumInRect(rect, filter)
    local raw = registry.require(self, 'Group.enumInRect')
    local rawRect = Handle.unwrap(rect, 'Rect', 'Group.enumInRect')
    checkFilter(filter, 'Group.enumInRect')
    GroupClear(raw)
    -- Warcraft accepts a null filter; the generated JASS signature cannot express that.
    ---@diagnostic disable-next-line: param-type-mismatch
    GroupEnumUnitsInRect(raw, rawRect, nil)
    applyFilter(raw, filter)
end
---@param player MoonwellWrappers.Player
---@param filter (fun(unit: MoonwellWrappers.Unit): any)?
function Group:enumOfPlayer(player, filter)
    local raw = registry.require(self, 'Group.enumOfPlayer')
    local rawPlayer = Handle.unwrap(player, 'Player', 'Group.enumOfPlayer')
    checkFilter(filter, 'Group.enumOfPlayer')
    GroupClear(raw)
    -- Warcraft accepts a null filter; the generated JASS signature cannot express that.
    ---@diagnostic disable-next-line: param-type-mismatch
    GroupEnumUnitsOfPlayer(raw, rawPlayer, nil)
    applyFilter(raw, filter)
end
---Inherits the native's synchronization behavior for selections.
---@param player MoonwellWrappers.Player
---@param filter (fun(unit: MoonwellWrappers.Unit): any)?
function Group:enumSelected(player, filter)
    local raw = registry.require(self, 'Group.enumSelected')
    local rawPlayer = Handle.unwrap(player, 'Player', 'Group.enumSelected')
    checkFilter(filter, 'Group.enumSelected')
    GroupClear(raw)
    -- Warcraft accepts a null filter; the generated JASS signature cannot express that.
    ---@diagnostic disable-next-line: param-type-mismatch
    GroupEnumUnitsSelected(raw, rawPlayer, nil)
    applyFilter(raw, filter)
end
---@return integer
function Group:getSize() return BlzGroupGetSize(registry.require(self, 'Group.getSize')) end
---@return MoonwellWrappers.Unit[]
function Group:getUnits()
    local result = {}
    for _, unit in ipairs(members(registry.require(self, 'Group.getUnits'))) do
        result[#result + 1] = assert(Unit.fromHandle(unit))
    end
    return result
end
---Iterates a snapshot; errors propagate to the caller.
---@param callback fun(unit: MoonwellWrappers.Unit): ...
function Group:forEach(callback)
    local raw = registry.require(self, 'Group.forEach')
    Callback.check(callback, 'Group.forEach')
    for _, unit in ipairs(members(raw)) do callback(assert(Unit.fromHandle(unit))) end
end
---@return MoonwellWrappers.Unit?
function Group:first() return Unit.fromHandle(FirstOfGroup(registry.require(self, 'Group.first'))) end
function Group:destroy()
    local raw = registry.dispose(self, 'Group.destroy')
    if raw then DestroyGroup(raw) end
end

return Group
```

- [ ] **Step 4: Run all suites**

Run: `deno task test`
Expected: all suites pass, including the three v0.1.0 group tests.

- [ ] **Step 5: Run every check and commit**

Run the full check list. Expected: all pass.

```bash
git add src tests
git commit -m "feat: group enumerations with Lua filters, forEach and first

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: Editor fixtures, integration and the gate example

**Files:**
- Modify: `tests/editor-positive.lua`, `tests/editor-negative.lua`
- Modify: `tools/integration.ts`
- Modify: `examples/gate.yue` (whole file below)

**Interfaces:**
- Consumes: every public module.

- [ ] **Step 1: Extend the positive fixture**

In `tests/editor-positive.lua`, add after the existing requires:

```lua
local Item = require('wrappers.item')
local Destructable = require('wrappers.destructable')
local Rect = require('wrappers.rect')
local Region = require('wrappers.region')
local Force = require('wrappers.force')
```

Insert before the line `effect:destroy()`:

```lua
local item = Item.create(1918989411, 0, 0)
item:setLife(item:getLife() + 1)
unit:issueTargetOrder('smart', item)
unit:damageTarget(item, 10, true, false, ATTACK_TYPE_NORMAL, DAMAGE_TYPE_NORMAL, WEAPON_TYPE_WHOKNOWS)
local slotItem = unit:getItemInSlot(0)
if slotItem then slotItem:setCharges(2) end
local tree = Destructable.create(1280601204, 0, 0, 270, 1, 0)
trigger:registerDeathEvent(tree)
local area = Rect.create(-100, -100, 100, 100)
local region = Region.create()
region:addRect(area)
trigger:registerEnterRegion(region)
local action = trigger:addAction(function(self) self:disable() end)
local condition = trigger:addCondition(function() return unit:isAlive() end)
trigger:removeAction(action)
trigger:removeCondition(condition)
group:enumInRect(area, function(member) return member:getTypeId() == unit:getTypeId() end)
group:forEach(function(member) member:setMana(0) end)
local force = Force.create()
force:add(PlayerWrapper.fromIndex(0))
for _, member in ipairs(force:getPlayers()) do member:addGold(10) end
force:destroy()
region:destroy()
area:destroy()
tree:remove()
item:remove()
```

- [ ] **Step 2: Extend the negative fixture**

In `tests/editor-negative.lua`, add after the existing requires:

```lua
local Item = require('wrappers.item')
local Trigger = require('wrappers.trigger')
```

Insert before the final `return true`:

```lua
local trigger = Trigger.create()
trigger:registerDeathEvent(Timer.create()) -- EXPECT param-type-mismatch
trigger:removeAction(trigger:addCondition(function() return true end)) -- EXPECT param-type-mismatch
Item.create(1, 0, 0):nonexistentMethod() -- EXPECT undefined-field
unit:getItemInSlot(0):setCharges(1) -- EXPECT need-check-nil
```

- [ ] **Step 3: Extend the bundle checks in `tools/integration.ts`**

Replace the line
`for (const unused of ["effect", "trigger", "group", "timer"]) {` with
`for (const unused of ["effect", "trigger", "group", "timer", "destructable", "rect", "region", "force"]) {`.

After the line `console.log("Moonwell: normal/minified builds, unused-module exclusion and bundled runtime passed");`
insert:

```ts
// Trigger takes wrapper arguments only, so a Trigger-only map must bundle no other public module.
await Deno.writeTextFile(
  join(consumer, "src/main.yue"),
  'import "wrappers.trigger" as Trigger\nt = Trigger.create!\nt\\destroy!\n',
);
await moonwell(["build"]);
const triggerBundle = await Deno.readTextFile(join(consumer, "dist/stage/map.w3x/war3map.lua"));
const publicModules = ["unit", "player", "item", "destructable", "rect", "region", "force", "group", "timer", "effect"];
for (const unused of publicModules) {
  if (triggerBundle.includes(`wrappers.${unused}`)) throw new Error(`Trigger-only bundle includes wrappers.${unused}`);
}
console.log("Moonwell: a Trigger-only map bundles no other public module");
```

Replace the final log line's text `"Gate example: all six modules build and editor diagnostics are clean; game execution remains manual"` with
`"Gate example: every module builds and editor diagnostics are clean; game execution remains manual"`.

- [ ] **Step 4: Replace `examples/gate.yue`**

```yue
-- Copy to src/main.yue in a disposable Moonwell map configured with this library.
-- Run once normally, then set probes = true to verify callback errors remain visible and recover.
-- Type -gate in chat while the hero is alive. CONTRIBUTING lists every expected message.
import "moonwell" as mw
import "moonwell.macros" as {:$FourCC}
import "wrappers.player" as Player
import "wrappers.unit" as Unit
import "wrappers.item" as Item
import "wrappers.destructable" as Destructable
import "wrappers.rect" as Rect
import "wrappers.region" as Region
import "wrappers.force" as Force
import "wrappers.timer" as Timer
import "wrappers.trigger" as Trigger
import "wrappers.group" as Group
import "wrappers.effect" as Effect

probes = false

-- v0.1.0 foundation: same observations as the first release.
foundationGate = (owner) ->
  unit = Unit.create owner, $FourCC("hfoo"), 0, 0, 270
  assert Unit.fromHandle(unit\getHandle!) == unit
  unit\setPosition 100, 0
  unit\setColor PLAYER_COLOR_RED
  unit\setLife 200
  unit\issuePointOrder "move", 300, 0
  group = Group.create!
  group\enumInRange 100, 0, 600
  assert group\contains unit
  print "Wrapper group size", group\getSize!
  effect = Effect.attach "Abilities\\Spells\\Human\\HolyBolt\\HolyBoltSpecialArt.mdl", unit, "origin"
  death = Trigger.create!
  death\registerUnitEvent unit, EVENT_UNIT_DEATH
  death\addAction -> print "Wrapper unit death event"
  periodic = Trigger.create!
  periodic\registerTimerEvent 1, true
  events = 0
  periodic\addAction ->
    events += 1
    if probes and events == 1
      error "intentional trigger probe"
    print "Wrapper trigger tick", events
  timer = Timer.create!
  ticks = 0
  timer\start 1, true, (self) ->
    ticks += 1
    if probes and ticks == 1
      error "intentional timer probe"
    print "Wrapper timer tick", ticks
    if ticks == 3
      unit\kill!
    if ticks == 5
      effect\destroy!
      group\destroy!
      death\destroy!
      periodic\destroy!
      unit\remove!
      self\destroy!
      effect\destroy!
      group\destroy!
      death\destroy!
      periodic\destroy!
      unit\remove!
      self\destroy!
      print "Wrapper gate cleanup passed"

-- Weak cache probe. RemoveUnit is called directly to stand in for the game removing units on its own (decay):
-- those wrappers are never disposed. The game has no collectgarbage, so allocate until a weak sentinel is collected.
churnProbe = (owner) ->
  footman = $FourCC("hfoo")
  keep = [Unit.create(owner, footman, 600, y * 60, 0) for y = 1, 5]
  stale = {}
  for i = 1, 20
    unit = Unit.create owner, footman, 800, i * 30, 0
    stale[] = unit if i <= 5
    RemoveUnit unit\getHandle!
  sentinel = setmetatable {}, __mode: "k"
  sentinel[{}] = true
  junk = {}
  collector = Timer.create!
  collector\start 0.1, true, (self) ->
    for i = 1, 2000
      junk[i] = {}
    if next(sentinel) ~= nil
      return
    self\destroy!
    fresh = [Unit.create(owner, footman, 1000, y * 30, 0) for y = 1, 20]
    for unit in *fresh
      found = Unit.fromHandle unit\getHandle!
      for old in *stale
        assert found ~= old, "stale wrapper resolved to a new unit"
    for unit in *keep
      assert Unit.fromHandle(unit\getHandle!) == unit, "live wrapper lost identity"
    print "Wrapper weak cache probe passed"
    unit\remove! for unit in *fresh
    unit\remove! for unit in *keep

-- v0.2.0 coverage.
broadGate = (owner) ->
  hero = Unit.create owner, $FourCC("Hpal"), -300, 0, 0
  assert hero\isHero!
  hero\addXP 1000, true
  print "Wrapper hero level", hero\getLevel!, "strength", hero\getStr(true)
  slow = $FourCC("Aslo")
  assert hero\addAbility(slow)
  hero\setAbilityLevel slow, 2
  hero\startCooldown slow, 30
  print "Wrapper ability level", hero\getAbilityLevel(slow), "cooldown", hero\getCooldownRemaining(slow)
  claws = assert hero\addItemById($FourCC("ratc")), "no item added"
  assert hero\getItemInSlot(0) == claws
  assert hero\dropItemAt(claws, -200, 0)
  claws\setCharges 3
  print "Wrapper item", claws\getName!, "charges", claws\getCharges!
  pickups = Trigger.create!
  pickups\registerAnyUnitEvent EVENT_PLAYER_UNIT_PICKUP_ITEM
  pickups\addCondition -> Unit.fromHandle(GetTriggerUnit!) == hero
  pickups\addAction -> print "Wrapper item picked up", Item.fromHandle(GetManipulatedItem!) == claws
  tree = Destructable.create $FourCC("LTlt"), 300, 300, 270, 1, 0
  deaths = Trigger.create!
  deaths\registerDeathEvent tree
  deaths\addAction -> print "Wrapper tree death event"
  area = Rect.create 200, -100, 400, 100
  zone = Region.create!
  zone\addRect area
  entries = Trigger.create!
  entries\registerEnterRegion zone
  entries\addAction -> print "Wrapper region entered"
  team = Force.create!
  team\enumAllies owner
  print "Wrapper force players", #team\getPlayers!
  chat = Trigger.create!
  chat\registerChatEvent owner, "-gate", true
  chat\addCondition -> hero\isAlive!
  removed = chat\addAction -> print "ERROR removed action ran"
  chat\addAction -> print "Wrapper chat accepted"
  chat\removeAction removed
  probe = Trigger.create!
  if probes
    probe\registerTimerEvent 1, false
    probe\addCondition -> error "intentional condition probe"
    probe\addAction -> print "ERROR condition probe action ran"
  heroes = Group.create!
  heroes\enumInRange(-300, 0, 800, (unit) -> unit\isHero!)
  print "Wrapper filtered heroes", heroes\getSize!
  steps = Timer.create!
  step = 0
  steps\start 2, true, (self) ->
    step += 1
    if step == 1
      hero\issueTargetOrder "smart", claws
    elseif step == 2
      tree\kill!
    elseif step == 3
      tree\restore tree\getMaxLife!, true
      print "Wrapper tree restored", tree\getLife!
    elseif step == 15
      assert hero\hasItem(claws), "hero did not pick up the item"
      hero\removeItem claws
      claws\remove!
      heroes\destroy!
      team\destroy!
      trigger\destroy! for trigger in *{pickups, deaths, entries, chat, probe}
      zone\destroy!
      area\destroy!
      tree\remove!
      hero\remove!
      self\destroy!
      print "Wrapper broad cleanup passed"
      churnProbe owner

mw.on_main ->
  owner = Player.fromIndex 0
  foundationGate owner
  broadGate owner
```

- [ ] **Step 5: Run the integration test**

Run the full check list; `deno task test:integration` is the one this task changes.
Expected: positive fixtures clean; the negative check reports `8 intentional type errors detected at the expected
lines`; both bundle checks pass; the gate example builds and has clean diagnostics.

If LuaLS reports a diagnostic inside a library file (for example on the `MoonwellWrappers.Widget` or token class
declarations), fix the annotation in the library, not the fixture, and re-run. If the negative count differs because
LuaLS does not flag `trigger:removeAction(trigger:addCondition(...))`, add the annotation line
`---@field private actionToken nil` under `---@class MoonwellWrappers.TriggerAction` and
`---@field private conditionToken nil` under `---@class MoonwellWrappers.TriggerCondition`, then re-run.

- [ ] **Step 6: Commit**

```bash
git add tests tools examples src
git commit -m "test: editor fixtures, trigger-only bundle check and v0.2.0 gate example

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 9: Documentation, final review and handoff

**Files:**
- Modify: `README.md`, `CHANGELOG.md`, `CONTRIBUTING.md`, `AGENTS.md` (wrappers repo)
- Modify: Moonwell `AGENTS.md`; this plan's execution ledger

- [ ] **Step 1: Update `README.md`**

1. Replace the first paragraph's second sentence with: `It provides Player, Unit, Item, Destructable, Rect, Region,
   Force, Timer, Trigger, Group and Effect wrappers, editor completion, stable handle identity and explicit cleanup.`
2. Replace the **Status** paragraph with: `**Status:** \`v0.1.0\` released 2026-09-28. \`v0.2.0\` (broad coverage) is
   on main; its in-game gate is pending, so use \`v0.1.0\` from GitHub until it is tagged. UI and presentation types
   (dialogs, multiboards, sounds, text tags) are not wrapped yet.`
3. In "Handles and cleanup", replace the sentences from `Factories return non-null wrappers` through
   `Player wrappers stay cached for the game session.` with:

   ```markdown
   Factories return non-null wrappers or raise an error if the native returns nil. Rewrapping the same live handle
   returns the same Lua table while any reference to that wrapper exists.

   Unit, Item and Destructable use a weak cache: the game removes these on its own (decay, used powerups, dead trees),
   so a wrapper nothing references may be collected, and a later `fromHandle` returns a fresh wrapper. Keep a reference
   (a variable, table key or closure) wherever identity matters. Timer, Trigger, Group, Effect, Rect, Region and Force
   stay cached until you destroy them; Player wrappers stay cached for the game session.
   ```

   and replace `Use \`unit:remove()\` and \`timer/trigger/group/effect:destroy()\`.` with
   `Use \`unit/item/destructable:remove()\` and \`timer/trigger/group/effect/rect/region/force:destroy()\`.`
4. Add a section after "Handles and cleanup":

   ```markdown
   ## Widgets

   Unit, Item and Destructable are widgets (`MoonwellWrappers.Widget` in the editor). All three have `getLife()`,
   `setLife(value)`, `getX()` and `getY()`. Parameters typed Widget accept any of them: `unit:issueTargetOrder`,
   `unit:issueTargetOrderById`, `unit:damageTarget` and `trigger:registerDeathEvent`. There is no `Widget.fromHandle`:
   convert a raw widget with the class you know it is, for example `Item.fromHandle(GetManipulatedItem())`.
   ```

5. Replace the API reference table with:

   ```markdown
   | Module                  | Factories and methods beyond the common handle methods |
   | ----------------------- | ------------------------------------------------------ |
   | `wrappers.player`       | `fromIndex(index)`; `getId()`, `getName()`, `getColor()`, `getState(playerstate)`, `setState(playerstate, integer)`; `getGold()`, `setGold(n)`, `addGold(n)`, `getLumber()`, `setLumber(n)`, `addLumber(n)`; `getAlliance(Player, alliancetype)`, `setAlliance(Player, alliancetype, flag)`, `isAlly(Player)`, `isEnemy(Player)`; `getTechCount(techId, specificOnly)`, `setTechResearched(techId, level)`, `addTechResearched(techId, levels)`, `setTechMaxAllowed(techId, max)`, `setAbilityAvailable(abilityId, flag)`; `getController()`, `getSlotState()`, `getRace()`, `getTeam()`, `getStartX()`, `getStartY()`, `isLocal()` |
   | `wrappers.unit`         | `create(Player, typeId, x, y, facing)`; `getTypeId()`, `getName()`, `getOwner()`, `setOwner(Player, changeColor)`; `getX()`, `getY()`, `setPosition(x,y)`, `setX(x)`, `setY(y)`, `getFacing()`, `setFacing(degrees)`; `getLife()`, `setLife(v)`, `getMaxLife()`, `setMaxLife(n)`, `getMana()`, `setMana(v)`, `getMaxMana()`, `setMaxMana(n)`, `getMoveSpeed()`, `setMoveSpeed(v)`; `setColor(playercolor)`, `setScale(s)`, `setVertexColor(r,g,b,a)`, `setAnimation(name)`, `pause(flag)`, `isPaused()`, `setInvulnerable(flag)`, `isInvulnerable()`, `show(flag)`, `isHidden()`; `isType(unittype)`, `isAlly(Player)`, `isEnemy(Player)`, `isAlive()`, `getCurrentOrder()`; `kill()`, `remove()`, `applyTimedLife(buffId, seconds)`, `damageTarget(Widget, amount, attack, ranged, attacktype, damagetype, weapontype)`; hero: `isHero()`, `getHeroName()`, `getLevel()`, `setLevel(level, showEffect)`, `getXP()`, `setXP(xp, showEffect)`, `addXP(xp, showEffect)`, `getStr/getAgi/getInt(includeBonuses)`, `setStr/setAgi/setInt(value, permanent)`, `getSkillPoints()`, `modifySkillPoints(delta)`, `selectSkill(abilityId)`, `revive(x, y, showEffect)`; abilities: `addAbility(id)`, `removeAbility(id)`, `getAbilityLevel(id)`, `setAbilityLevel(id, level)`, `makeAbilityPermanent(id, permanent)`, `hideAbility(id, hidden)`, `disableAbility(id, disabled, hideUI)`, `startCooldown(id, seconds)`, `endCooldown(id)`, `getCooldownRemaining(id)`; inventory: `getInventorySize()`, `getItemInSlot(slot)`, `addItem(Item)`, `addItemById(typeId)`, `removeItem(Item)`, `removeItemFromSlot(slot)`, `hasItem(Item)`, `dropItemAt(Item, x, y)`, `dropItemToSlot(Item, slot)`, `useItem(Item)`; orders (return boolean): `issueOrder(order)`, `issuePointOrder(order, x, y)`, `issueTargetOrder(order, Widget)`, `issueOrderById(id)`, `issuePointOrderById(id, x, y)`, `issueTargetOrderById(id, Widget)` |
   | `wrappers.item`         | `create(typeId, x, y)`; `getTypeId()`, `getName()`, `getLevel()`, `setPosition(x, y)`, `getCharges()`, `setCharges(n)`, `getOwner()`, `setOwner(Player, changeColor)`, `isOwned()`, `isPowerup()`, `isVisible()`, `setVisible(flag)`, `isInvulnerable()`, `setInvulnerable(flag)`, `setDroppable(flag)`, `setPawnable(flag)`, `remove()` |
   | `wrappers.destructable` | `create(typeId, x, y, facing, scale, variation)`; `getTypeId()`, `getName()`, `getMaxLife()`, `setMaxLife(v)`, `kill()`, `restore(life, birth)`, `isInvulnerable()`, `setInvulnerable(flag)`, `show(flag)`, `setAnimation(name)`, `queueAnimation(name)`, `remove()` |
   | `wrappers.rect`         | `create(minX, minY, maxX, maxY)`, `worldBounds()`; `getMinX()`, `getMinY()`, `getMaxX()`, `getMaxY()`, `getCenterX()`, `getCenterY()`, `set(minX, minY, maxX, maxY)`, `moveTo(x, y)`, `destroy()` |
   | `wrappers.region`       | `create()`; `addRect(Rect)`, `clearRect(Rect)`, `addCell(x, y)`, `clearCell(x, y)`, `containsPoint(x, y)`, `containsUnit(Unit)`, `destroy()` |
   | `wrappers.force`        | `create()`; `add(Player)`, `remove(Player)`, `contains(Player)`, `clear()`, `enumPlayers()`, `enumAllies(Player)`, `enumEnemies(Player)`, `getPlayers()`, `destroy()` |
   | `wrappers.timer`        | `create()`; `start(timeout, periodic, callback)`, `pause()`, `resume()`, `getElapsed()`, `getRemaining()`, `getTimeout()`, `destroy()` |
   | `wrappers.trigger`      | `create()`; `enable()`, `disable()`, `isEnabled()`, `evaluate()`, `execute()`; `registerUnitEvent(Unit, unitevent)`, `registerPlayerUnitEvent(Player, playerunitevent)`, `registerAnyUnitEvent(playerunitevent)`, `registerPlayerEvent(Player, playerevent)`, `registerChatEvent(Player, text, exactMatch)`, `registerEnterRegion(Region)`, `registerLeaveRegion(Region)`, `registerDeathEvent(Widget)`, `registerUnitInRange(Unit, range)`, `registerUnitStateEvent(Unit, unitstate, limitop, value)`, `registerTimerEvent(timeout, periodic)`, `registerGameEvent(gameevent)`; `addAction(callback)` and `addCondition(predicate)` return tokens for `removeAction(token)` and `removeCondition(token)`; `clearActions()`, `clearConditions()`, `destroy()` |
   | `wrappers.group`        | `create()`; `add(Unit)`, `remove(Unit)`, `contains(Unit)`, `clear()`; `enumInRange(x, y, radius, filter?)`, `enumInRect(Rect, filter?)`, `enumOfPlayer(Player, filter?)`, `enumSelected(Player, filter?)`; `getSize()`, `getUnits()`, `forEach(callback)`, `first()`, `destroy()` |
   | `wrappers.effect`       | `create(model,x,y)`, `attach(model,Unit,attachmentPoint)`; `setPosition(x,y,z)`, `setScale(scale)`, `destroy()` |
   ```

6. After the table, replace the paragraph starting `Player indices must be integers` with:

   ```markdown
   Player indices must be integers below `bj_MAX_PLAYER_SLOTS`, including neutral slots. Players have no destruction
   method. `setPosition` uses SetUnitPosition, which respects pathing; `setX`/`setY` use SetUnitX/SetUnitY, which do
   not. Inventory slots are zero-based integers below `getInventorySize()`. `getItemInSlot`, `removeItemFromSlot`,
   `addItemById` and `group:first()` return nil when there is nothing. Hero methods pass through to the natives, so
   Warcraft's behavior applies to non-heroes. `isLocal()` is true only on that player's machine: never change
   synchronized game state inside a branch on it. `enumSelected` inherits the native's synchronization behavior.
   `Rect.worldBounds()` allocates a new rect each call; destroy it. SetPlayerName is deliberately outside the API.
   ```

7. Replace the paragraph starting `` `enumInRange` clears the group`` with:

   ```markdown
   Every group enumeration clears the group first and passes no native filter. The optional `filter` then runs over a
   snapshot and removes the units for which it returns falsy; "of type" is a filter such as
   `(u) -> u\getTypeId! == id`. If the filter raises, the group is cleared and the error propagates. Negative,
   infinite and NaN radii fail before clearing. `getUnits()` returns a dense one-based array, skips nil native entries
   and preserves native order without promising sorting. Later group changes do not alter the array. `forEach` iterates
   the same kind of snapshot; its errors propagate. `Force.getPlayers()` is a snapshot in the same way.
   ```

8. In "Callbacks", replace the two sentences starting `Trigger registrations have no native filter argument` with:

   ```markdown
   Registrations pass no native filter; filter inside a condition or an action. Warcraft cannot unregister an event,
   so destroying the trigger is the only way to remove one. `addAction` and `addCondition` return tokens; removing a
   token takes effect at once, even during a firing, and removing it twice does nothing. A token from another trigger
   raises an error. A condition's result counts as truthy or falsy; if it raises, the error is printed with
   `[wrappers] Trigger condition failed:` and the condition counts as false. The trigger owns each condition's
   boolexpr and destroys it on removal, on `clearConditions()` and on `destroy()`.
   ```

- [ ] **Step 2: Update `CHANGELOG.md`**

Insert above `## 0.1.0 (2026-09-28)`:

```markdown
## Unreleased (0.2.0)

- New wrappers: Item, Destructable, Rect, Region and Force.
- Unit: hero, ability, inventory, mana, movement, presentation and by-id order methods, `isAlive()` and
  `damageTarget`. `issueTargetOrder` accepts any widget (Unit, Item, Destructable).
- Player: gold and lumber, alliances, tech, slot state, start location and `isLocal()`.
- Trigger: any-player unit, player, chat, region, death, range, unit-state and game events; predicate conditions;
  `addAction` returns a token; `removeAction`, `removeCondition`, `clearActions`, `clearConditions`, `evaluate`,
  `execute`.
- Group: `enumInRect`, `enumOfPlayer`, `enumSelected`, an optional Lua filter on every enumeration, `forEach`, `first`.
- **Changed:** Unit, Item and Destructable wrappers use a weak cache. A wrapper nothing references may be collected,
  and a later `fromHandle` returns a fresh one; identity is unchanged while any reference exists.
- Trigger and Effect no longer import Unit or Player: wrapper arguments convert through the loaded classes.

### Release gate

Pending: automated checks, the in-game gate (including the weak cache probe and a two-player LAN run) and tag
consumption.
```

- [ ] **Step 3: Update `CONTRIBUTING.md`**

1. Replace the sentence `The two line-local LuaLS suppressions for null boolexpr filters document a mismatch in
   Moonwell's generated JASS signatures.` with `The line-local LuaLS suppressions on native calls that pass a null
   boolexpr filter document a mismatch in Moonwell's generated JASS signatures; each carries the same comment.`
2. Replace the numbered steps 1–6 of "In-game release gate (maintainer)" with:

   ```markdown
   1. Create a disposable Moonwell map and configure this checkout with the local path example in README. Copy
      `examples/gate.yue` to its `src/main.yue`; run check and test.
   2. Foundation (as in v0.1.0): the footman appears, moves, changes life and color (disable ally color mode with Alt+A
      if needed), and the attached effect appears. Timer and trigger ticks print; the death event prints at tick 3;
      `Wrapper gate cleanup passed` prints at tick 5 and no later foundation ticks print.
   3. Broad: at start, `Wrapper hero level` (above 1), `Wrapper ability level 2` with a cooldown near 30,
      `Wrapper item <item name> charges 3`, `Wrapper force players <count>` and `Wrapper filtered heroes 1`
      print. `Wrapper region entered` prints when the footman walks into the rect. Within a few seconds
      `Wrapper item picked up true`, `Wrapper tree death event` and `Wrapper tree restored` print, and the tree stands
      again.
   4. Type `-gate` in chat before 30 seconds: `Wrapper chat accepted` prints once and `ERROR removed action ran`
      never prints. At 30 seconds `Wrapper broad cleanup passed` prints and the hero, item and tree disappear.
   5. Then `Wrapper weak cache probe passed` prints and the probe's footmen disappear. No assertion error prints.
   6. Set `probes = true` and run again. The intentional timer and trigger errors print and both callbacks continue;
      `[wrappers] Trigger condition failed: ... intentional condition probe` prints and `ERROR condition probe action
      ran` never prints. Restore `probes = false`.
   7. Build with `--minify` and play the packed map; repeat steps 2–5. Open the packed map in World Editor.
   8. Host a two-player LAN game of the minified map and play until the weak cache probe passes on both machines. No
      desync. If a LAN game is not possible, record that and decide with the maintainer before tagging.
   9. Record results here and in CHANGELOG, including Warcraft/editor versions. Automated native doubles cannot replace
      this gate. Do not declare the release ready while this is pending. If the weak cache probe fails, stop: the
      spec's fallback is strong widget caches plus `forget()`.
   ```

3. Keep the paragraph starting `Passed 2026-09-28, confirmed by the maintainer` and prefix it with `v0.1.0: `.
4. In "First publication and tag gate", replace `v0.1.0` in the sentence `publish an immutable \`v0.1.0\` tag` with
   `vX.Y.Z` and keep the recorded v0.1.0 result paragraph, prefixed `v0.1.0: `.

- [ ] **Step 4: Update the wrappers `AGENTS.md`**

1. Add after the two design-history bullets:
   `- \`../moonwell/docs/superpowers/specs/2026-09-28-moonwell-wrappers-broad-design.md\` and
   \`../moonwell/docs/superpowers/plans/2026-09-28-moonwell-wrappers-broad.md\` (v0.2.0)`
2. Replace `Broader wrapper coverage is deferred in Moonwell's backlog.` with `v0.2.0 adds broad gameplay coverage; UI
   and presentation types remain in Moonwell's backlog.`
3. Replace the rule `- Cache identity by raw native handle. Cleanup invalidates ...` with:
   `- Cache identity by raw native handle. Unit, Item and Destructable caches are weak-valued; all others are strong.
   Cleanup invalidates wrappers/callbacks before native destruction and is idempotent. Never destroy game objects
   through garbage collection, and never iterate a table keyed by tables when the loop calls natives.`
4. Replace `- Existing two line-local nil-filter diagnostic exceptions compensate ...` with:
   `- Line-local nil-filter diagnostic exceptions, each with the standard comment, compensate for generated JASS type
   limitations. Do not add broad diagnostic suppression.`
5. Add a rule: `- A module imports another public module only to return its wrappers. Convert wrapper arguments with
   \`Handle.unwrap\` or \`Handle.unwrapWidget\`.`

- [ ] **Step 5: Run every wrappers check and commit**

Run `deno fmt` (formats the markdown tables), then the full check list. Expected: all pass.

```bash
git add README.md CHANGELOG.md CONTRIBUTING.md AGENTS.md
git commit -m "docs: document wrappers v0.2.0 and its release gate

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 6: Independent final review**

Request a fresh-context review (superpowers:requesting-code-review) of the whole v0.2.0 diff
(`git diff 3a62111..HEAD` in `moonwell-wrappers`) against the spec, with the "Review focus" list above. Fix important
findings test-first, re-run all checks, and commit fixes separately.

- [ ] **Step 7: Update Moonwell `AGENTS.md` and this plan**

In Moonwell `AGENTS.md`:

1. After the Plan 4c bullet in "State", add:

   ```markdown
   - **Wrappers v0.2.0, broad coverage** (2026-09-28, spec
     `docs/superpowers/specs/2026-09-28-moonwell-wrappers-broad-design.md`, plan
     `docs/superpowers/plans/2026-09-28-moonwell-wrappers-broad.md`): implemented in `../moonwell-wrappers` on main
     (not tagged). Adds Item, Destructable, Rect, Region and Force; deeper Unit, Player, Trigger (conditions, removable
     actions) and Group (filtered enumerations); a widget layer; weak Unit/Item/Destructable caches. Automated checks
     pass; the in-game gate (with the weak cache probe and a two-player LAN run) and tag consumption are pending.
   ```

2. Replace "Next work" item 1 with: `1. **Wrappers v0.2.0 release gate.** The maintainer runs the in-game gate in
   \`../moonwell-wrappers/CONTRIBUTING.md\`, including the weak cache probe and a two-player LAN run. If the probe
   fails, apply the spec's fallback (strong widget caches plus \`forget()\`). Then tag \`v0.2.0\` and run tag
   consumption. After that, choose the next sub-project with the maintainer.`
3. In "Backlog", replace the "Broad wrapper library" bullet with: `- **UI and presentation wrappers.** Dialog and
   button, multiboard, leaderboard, quest, timer dialog and frame; sound, text tag, lightning, image, ubersplat and
   fog modifier; item and destructable enumeration. Deferred from wrappers v0.2.0 (2026-09-28).`

Add an "Execution ledger" section at the end of this plan recording each task's result, deviations and the final
review. Run Moonwell's checks from its AGENTS.md (`deno task check`, `deno task lint`, `deno fmt --check`, and the test
tasks), then commit in Moonwell:

```bash
git add AGENTS.md docs/superpowers/plans/2026-09-28-moonwell-wrappers-broad.md
git commit -m "docs: record wrappers v0.2.0 implementation state

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 8: Hand off**

Report to the maintainer: commits in both repositories, test counts, the integration evidence, and the pending
in-game gate (steps 1–9 in CONTRIBUTING, including LAN). Do not push, tag or publish.

## Execution ledger

- 2026-09-28: executed with superpowers:subagent-driven-development (a fresh Opus implementer per task, a task review
  after each, one final whole-branch review). Commits on `main` in `../moonwell-wrappers`, base `4f91f8d`.
- Tasks 1–6 and 8: clean reviews (commits `1df3ebe`, `cfc941a`, `95d594f`, `b9b90a5`, `b2f851d`, `9443c7f`, `737d2ab`).
- Task 7: one fix round (`f1b28ba`). Ruling: `forEach` and group filters wrap the whole snapshot before any callback,
  overriding this plan's lazy wrapping; spec §8 says `forEach` iterates a `getUnits()` snapshot, and lazy wrapping
  revived a unit removed mid-loop as a live wrapper.
- Task 8 deviations accepted: the need-check-nil fixture goes through a local (LuaLS 3.19.1 does not flag chained
  calls); the gate's churn probe allocates throwaway tables. Ruling: the gate adds a self-removing chat action.
- Task 9: Steps 1–5 in `2e6776a`, one fix round (`88be7ef`). Ruling: the gate's weak cache probe creates and removes
  units only at fixed times and only prints GC and check results; the plan's GC-gated creation would itself desync the
  LAN gate. `deno fmt` fixed the pre-existing README formatting.
- Final review: ready "with fixes"; one fix wave (`830cf31`) switched the gate ability to Storm Bolt (Slow has one
  level), added a self-removing condition to the gate and stand-in tests, documented that weak tables keyed by widget
  wrappers are nondeterministic, and made group filter errors blame the caller. Scoped re-review: all addressed.
- Result: 69 behavior tests in 13 suites, Lua 5.3.6 syntax, integration (8 negative diagnostics, Unit-only and
  Trigger-only bundle checks, gate builds), direct LuaLS over `src` clean. The integration LuaLS runs do not diagnose
  library files, so each task also ran LuaLS directly over `src`.
- Pending (maintainer): the in-game gate in the wrappers CONTRIBUTING, including the LAN run, then tag consumption.
