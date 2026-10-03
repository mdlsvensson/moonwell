# Moonwell Wrappers Additions (v0.8.0) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** moonwell-wrappers v0.8.0: `wrappers.input`, `wrappers.weathereffect`, `Effect.abilityArt`, four Trigger
registrations and `fromEvent()` on six classes.

**Architecture:** `wrappers.input` is a third user of `internal/listeners.lua`: one trigger per player and key
(registered for all 16 modifier values, down and up) and per player and kind of mouse event, with a held state per key
that tells a press from a repeat. `wrappers.weathereffect` is a small owned wrapper like `wrappers.fogmodifier`. The
rest are single functions on existing classes. Nothing is created at import.

**Tech Stack:** annotated Lua 5.3, Deno tooling (`deno task test`, `check:lua`, `test:integration`), YueScript 0.34.2,
LuaLS 3.19.1, Lua 5.3.6 `luac`, Moonwell 0.5.2.

**Spec:** `docs/superpowers/specs/2026-10-01-moonwell-wrappers-additions-design.md`.

**Verified in advance:** every code block below was run on 2026-10-01 in a scratch copy of the repository
(`.test-work/proto8`): the suites (34 suites, 218 tests), the syntax check (71 files) and full integration (27 expected
negative diagnostics, nine one-module bundles, the gate example with clean diagnostics) passed. 66 mutations of the new
code are each caught by a test. The compiled gate ran to its end on stub natives.

## Global Constraints

- **Repository:** `C:\Users\mdlsvensson\Repo\moonwell-wrappers`; Moonwell records in
  `C:\Users\mdlsvensson\Repo\moonwell`; the gate map in `C:\Users\mdlsvensson\Repo\wrappers-gate` (not under git).
  Commit on `main`, explicit paths only.
- **Checks**, each its own command, from the repository root:
  - `deno task check`
  - `deno task lint`
  - `deno fmt --check`
  - `deno task test`
  - `MOONWELL_LUAC=.tools/lua53/luac53.exe deno task check:lua`
  - `MOONWELL_LUALS="C:/Users/mdlsvensson/.antigravity-ide/extensions/sumneko.lua-3.19.1-win32-x64/server/bin/lua-language-server.exe" deno task test:integration`
    (about four minutes)
- **Write files with the file tools, not shell heredocs:** Git Bash turns `\\` into `\` and `\n` into a newline, and a
  Python heredoc turns `\a` into a bell character. YueScript sources and the README pieces contain backslashes.
- **Messages:** `[wrappers] <Class>.<method>: <problem>`, raised at the caller's line: level 2 in a public function, 3
  from a checker helper, and `return (helper(...))` instead of a tail call. `failsAt` in tests checks the position.
- **Callback label:** `Input listener`.
- **No module creates a game object at import.** Every test file ends its setup with `eq(totalCalls(), 0)`.
- **A module imports another public module only to return its wrappers:** `wrappers.input` imports `wrappers.player`;
  `wrappers.weathereffect` imports none.

## Departure from the spec

The spec's plan phasing lists `wrappers.input` as two tasks (keys, then the mouse). It is one module file with one
listener set, so this plan writes it in one task (Task 5), with the keys and the mouse as separate tests.

## File Structure

| File | Responsibility |
| --- | --- |
| `src/wrappers/input.lua` (new) | `Input`: key and mouse listeners, the held state |
| `src/wrappers/weathereffect.lua` (new) | `WeatherEffect`: create, enable, destroy |
| `src/wrappers/effect.lua` | `Effect.abilityArt`, and the model check of the four constructors |
| `src/wrappers/trigger.lua` | Four registrations |
| `src/wrappers/{unit,player,item,destructable,timer,region}.lua` | `fromEvent()` |
| `tests/input.lua`, `tests/weathereffect.lua` (new) | The two new suites |
| `tests/blame.lua`, `tests/imports.lua`, `tests/editor-*.lua`, `tests/editor-positive.yue`, `tools/integration.ts` | The sweep, the fixtures and the one-module bundles |
| `examples/gate.yue` | The `additions` gate |

---

### Task 1: `Effect.abilityArt` and the model checks

**Files:** Modify `src/wrappers/effect.lua`, `tests/effect.lua`.

**Interfaces:**
- Produces: `Effect.abilityArt(abilityId: integer, effectType: effecttype, index?: integer): string?`; the four
  constructors raise `Effect.<name>: expected a model path` for a model that is not a string.

- [ ] **Step 1: Write the failing tests.** Append to `tests/effect.lua`, after an empty line:

```lua
test('abilityArt reads one entry of the art list, and nil for none', function()
    local caster, art = {}, {'first.mdl', 'second.mdl'}
    native('GetAbilityEffectById', function(_, _, index) return art[index + 1] or '' end)
    eq(Effect.abilityArt(1095267427, caster), 'first.mdl'); expectCall('GetAbilityEffectById', 1095267427, caster, 0)
    eq(Effect.abilityArt(1095267427, caster, 2), 'second.mdl')
    expectCall('GetAbilityEffectById', 1095267427, caster, 1)
    eq(Effect.abilityArt(1095267427, caster, 3), nil)
    native('GetAbilityEffectById', function() return nil end)
    eq(Effect.abilityArt(1095267427, caster), nil)
    resetCalls()
    for _, bad in ipairs({0, -1, 1.5, '1', 0/0}) do
        failsAt(function() Effect.abilityArt(1095267427, caster, bad) end,
            'Effect.abilityArt: expected a positive integer index')
    end
    eq(totalCalls(), 0)
end)

test('a model that is not a string fails at the caller, before any native', function()
    local u = Unit.fromHandle({})
    for _, bad in ipairs({false, 7, {}}) do
        failsAt(function() Effect.create(bad, 0, 0) end, 'Effect.create: expected a model path')
        failsAt(function() Effect.attach(bad, u, 'origin') end, 'Effect.attach: expected a model path')
        failsAt(function() Effect.flash(bad, 0, 0) end, 'Effect.flash: expected a model path')
        failsAt(function() Effect.flashOn(bad, u, 'origin') end, 'Effect.flashOn: expected a model path')
    end
    failsAt(function() Effect.create(nil, 0, 0) end, 'Effect.create: expected a model path')
    failsAt(function() Effect.flash(Effect.abilityArt(1, {}), 0, 0) end, 'Effect.flash: expected a model path')
    eq(callCount('AddSpecialEffect'), 0); eq(callCount('AddSpecialEffectTarget'), 0)
    Effect.create('', 0, 0):destroy(); expectCall('AddSpecialEffect', '', 0, 0)
    u:remove()
end)
```

- [ ] **Step 2: Run them to see them fail**

Run: `deno task test effect`
Expected: `2/7 tests failed`: `attempt to call a nil value (field 'abilityArt')` and `expected failure`.

- [ ] **Step 3: Implement.** In `src/wrappers/effect.lua`, above `Effect.fromHandle`'s annotations, add the checker,
followed by an empty line:

```lua
---A model that is not a string, such as a missing Effect.abilityArt, fails at the caller instead of drawing nothing.
---@param model unknown
---@param operation string
local function checkModel(model, operation)
    if type(model) ~= 'string' then error('[wrappers] ' .. operation .. ': expected a model path', 3) end
end
```

After `function Effect.fromHandle(raw) return registry.wrap(raw) end`, add:

```lua
---The art an ability's data names for an effect type: a model path, or a lightning code for EFFECT_TYPE_LIGHTNING.
---Returns nil when the ability has none. Past the last entry of a list the game reads the last entry.
---@param abilityId integer
---@param effectType effecttype
---@param index integer? Which entry of a list, from 1. Default 1.
---@return string?
function Effect.abilityArt(abilityId, effectType, index)
    if index == nil then index = 1 end
    if type(index) ~= 'number' or index % 1 ~= 0 or index < 1 then
        error('[wrappers] Effect.abilityArt: expected a positive integer index', 2)
    end
    local art = GetAbilityEffectById(abilityId, effectType, index - 1)
    if art == '' then return nil end
    return art
end
```

Then make the check the first line of each constructor's body:

```lua
function Effect.create(model, x, y)
    checkModel(model, 'Effect.create')
```

```lua
function Effect.attach(model, target, attachmentPoint)
    checkModel(model, 'Effect.attach')
```

```lua
function Effect.flash(model, x, y)
    checkModel(model, 'Effect.flash')
```

```lua
function Effect.flashOn(model, target, attachmentPoint)
    checkModel(model, 'Effect.flashOn')
```

- [ ] **Step 4: Run the tests**

Run: `deno task test effect blame`
Expected: `effect: SUITE PASSED: 7 tests` and `blame: SUITE PASSED: 2 tests`.

- [ ] **Step 5: Commit**

```bash
git add src/wrappers/effect.lua tests/effect.lua
git commit -m "feat: Effect.abilityArt, and a model check in the Effect constructors"
```

---

### Task 2: Four Trigger registrations

**Files:** Modify `src/wrappers/trigger.lua`, `tests/trigger.lua`.

**Interfaces:**
- Produces: `Trigger:registerPlayerStateEvent(player, state, op, value)`,
  `Trigger:registerPlayerAllianceChange(player, alliance)`, `Trigger:registerGameStateEvent(state, op, value)`,
  `Trigger:registerTimerExpireEvent(timer)`.

- [ ] **Step 1: Write the failing test.** Append to `tests/trigger.lua`, after an empty line:

```lua
test('player state, alliance, game state and timer expiry registrations forward exact arguments', function()
    for _, name in ipairs({'TriggerRegisterPlayerStateEvent', 'TriggerRegisterPlayerAllianceChange',
        'TriggerRegisterGameStateEvent', 'TriggerRegisterTimerExpireEvent', 'PauseTimer', 'DestroyTimer'}) do
        native(name, function() end)
    end
    local Timer = require('wrappers.timer')
    local t, p, timer = Trigger.create(), Player.fromHandle({}), Timer.fromHandle({})
    local state, op, alliance = {}, {}, {}
    t:registerPlayerStateEvent(p, state, op, 1000)
    expectCall('TriggerRegisterPlayerStateEvent', t.handle, p.handle, state, op, 1000)
    t:registerPlayerAllianceChange(p, alliance)
    expectCall('TriggerRegisterPlayerAllianceChange', t.handle, p.handle, alliance)
    t:registerGameStateEvent(state, op, 12.5)
    expectCall('TriggerRegisterGameStateEvent', t.handle, state, op, 12.5)
    t:registerTimerExpireEvent(timer)
    expectCall('TriggerRegisterTimerExpireEvent', t.handle, timer.handle)
    failsAt(function() t:registerPlayerStateEvent(timer, state, op, 1) end,
        'Trigger.registerPlayerStateEvent: expected Player wrapper')
    failsAt(function() t:registerPlayerAllianceChange(timer, alliance) end,
        'Trigger.registerPlayerAllianceChange: expected Player wrapper')
    failsAt(function() t:registerTimerExpireEvent(p) end, 'Trigger.registerTimerExpireEvent: expected Timer wrapper')
    local raw = timer.handle
    timer:destroy()
    failsAt(function() t:registerTimerExpireEvent(timer) end, 'Trigger.registerTimerExpireEvent: Timer is disposed')
    eq(callCount('TriggerRegisterPlayerStateEvent'), 1); eq(callCount('TriggerRegisterPlayerAllianceChange'), 1)
    eq(callCount('TriggerRegisterTimerExpireEvent'), 1); expectCall('DestroyTimer', raw)
    t:destroy()
    checkDisposed(t, {'registerPlayerStateEvent', 'registerPlayerAllianceChange', 'registerGameStateEvent',
        'registerTimerExpireEvent'})
    eq(callCount('TriggerRegisterGameStateEvent'), 1)
end)
```

- [ ] **Step 2: Run it to see it fail**

Run: `deno task test trigger`
Expected: `1/15 tests failed`: `attempt to call a nil value (method 'registerPlayerStateEvent')`.

- [ ] **Step 3: Implement.** In `src/wrappers/trigger.lua`, after `Trigger:registerGameEvent`, add:

```lua
---Fires inside SetPlayerState, at every change to a value that satisfies the comparison (measured on 3.0.0.24268).
---@param player MoonwellWrappers.Player
---@param state playerstate
---@param op limitop
---@param value number
function Trigger:registerPlayerStateEvent(player, state, op, value)
    local raw = registry.require(self, 'Trigger.registerPlayerStateEvent')
    local rawPlayer = Handle.unwrap(player, 'Player', 'Trigger.registerPlayerStateEvent')
    TriggerRegisterPlayerStateEvent(raw, rawPlayer, state, op, value)
end
---Fires inside SetPlayerAlliance when this player's setting of this kind toward any player really changes. The event
---names no player: GetTriggerPlayer() is nil (measured on 3.0.0.24268).
---@param player MoonwellWrappers.Player
---@param alliance alliancetype
function Trigger:registerPlayerAllianceChange(player, alliance)
    local raw = registry.require(self, 'Trigger.registerPlayerAllianceChange')
    local rawPlayer = Handle.unwrap(player, 'Player', 'Trigger.registerPlayerAllianceChange')
    TriggerRegisterPlayerAllianceChange(raw, rawPlayer, alliance)
end
---Fires when the comparison becomes true, by a set or by the game's clock (measured for GAME_STATE_TIME_OF_DAY on
---3.0.0.24268).
---@param state gamestate
---@param op limitop
---@param value number
function Trigger:registerGameStateEvent(state, op, value)
    TriggerRegisterGameStateEvent(registry.require(self, 'Trigger.registerGameStateEvent'), state, op, value)
end
---Fires at every expiry of the timer, before the timer's own callback (measured on 3.0.0.24268). The trigger does not
---own the timer.
---@param timer MoonwellWrappers.Timer
function Trigger:registerTimerExpireEvent(timer)
    local raw = registry.require(self, 'Trigger.registerTimerExpireEvent')
    TriggerRegisterTimerExpireEvent(raw, Handle.unwrap(timer, 'Timer', 'Trigger.registerTimerExpireEvent'))
end
```

- [ ] **Step 4: Run the tests**

Run: `deno task test trigger blame`
Expected: `trigger: SUITE PASSED: 15 tests` and `blame: SUITE PASSED: 2 tests`.

- [ ] **Step 5: Commit**

```bash
git add src/wrappers/trigger.lua tests/trigger.lua
git commit -m "feat: Trigger registrations for player state, alliance change, game state and timer expiry"
```

---

### Task 3: `fromEvent()` on six classes

**Files:** Modify `src/wrappers/{unit,player,item,destructable,timer,region}.lua` and the six suites of the same names
under `tests/`.

**Interfaces:**
- Produces: `Unit.fromEvent()`, `Player.fromEvent()`, `Item.fromEvent()`, `Destructable.fromEvent()`,
  `Timer.fromEvent()`, `Region.fromEvent()`, each returning its wrapper or nil.

- [ ] **Step 1: Write the failing tests.** Append one test to each suite, after an empty line.

`tests/unit.lua`:

```lua
test('fromEvent wraps the unit of the running event, and nil when it has none', function()
    local raw = {}
    native('GetTriggerUnit', function() return raw end)
    local found = Unit.fromEvent()
    eq(found, Unit.fromHandle(raw)); eq(found.handle, raw); eq(callCount('GetTriggerUnit'), 1)
    native('GetTriggerUnit', function() return nil end)
    eq(Unit.fromEvent(), nil)
end)
```

`tests/player.lua`:

```lua
test('fromEvent wraps the player of the running event, and nil when it has none', function()
    local raw = {}
    native('GetTriggerPlayer', function() return raw end)
    local found = Player.fromEvent()
    eq(found, Player.fromHandle(raw)); eq(found.handle, raw); eq(callCount('GetTriggerPlayer'), 1)
    native('GetTriggerPlayer', function() return nil end)
    eq(Player.fromEvent(), nil)
end)
```

`tests/item.lua`:

```lua
test('fromEvent wraps the item of the running event, and nil when it has none', function()
    local raw = {}
    native('GetManipulatedItem', function() return raw end)
    local found = Item.fromEvent()
    eq(found, Item.fromHandle(raw)); eq(found.handle, raw); eq(callCount('GetManipulatedItem'), 1)
    native('GetManipulatedItem', function() return nil end)
    eq(Item.fromEvent(), nil)
end)
```

`tests/destructable.lua`:

```lua
test('fromEvent wraps the destructable of the running event, and nil when it has none', function()
    local raw = {}
    native('GetTriggerDestructable', function() return raw end)
    local found = Destructable.fromEvent()
    eq(found, Destructable.fromHandle(raw)); eq(found.handle, raw); eq(callCount('GetTriggerDestructable'), 1)
    native('GetTriggerDestructable', function() return nil end)
    eq(Destructable.fromEvent(), nil)
end)
```

`tests/timer.lua`:

```lua
test('fromEvent wraps the timer of the running event, and nil when it has none', function()
    local raw = {}
    native('GetExpiredTimer', function() return raw end)
    local found = Timer.fromEvent()
    eq(found, Timer.fromHandle(raw)); eq(found.handle, raw); eq(callCount('GetExpiredTimer'), 1)
    native('GetExpiredTimer', function() return nil end)
    eq(Timer.fromEvent(), nil)
end)
```

`tests/region.lua`:

```lua
test('fromEvent wraps the region of the running event, and nil when it has none', function()
    local raw = {}
    native('GetTriggeringRegion', function() return raw end)
    local found = Region.fromEvent()
    eq(found, Region.fromHandle(raw)); eq(found.handle, raw); eq(callCount('GetTriggeringRegion'), 1)
    native('GetTriggeringRegion', function() return nil end)
    eq(Region.fromEvent(), nil)
end)
```

- [ ] **Step 2: Run them to see them fail**

Run: `deno task test unit player item destructable timer region`
Expected: one failed test in each of the six suites: `attempt to call a nil value (field 'fromEvent')`.

- [ ] **Step 3: Implement.** In each module, directly after its `fromHandle` line
(`function <Class>.fromHandle(raw) return registry.wrap(raw) end`), add the three lines below.

`src/wrappers/unit.lua`:

```lua
---The unit the running event is about (GetTriggerUnit), or nil when it has none.
---@return MoonwellWrappers.Unit?
function Unit.fromEvent() return registry.wrap(GetTriggerUnit()) end
```

`src/wrappers/player.lua` (the class table is named `PlayerWrapper` there):

```lua
---The player the running event is about (GetTriggerPlayer), or nil when it has none.
---@return MoonwellWrappers.Player?
function PlayerWrapper.fromEvent() return registry.wrap(GetTriggerPlayer()) end
```

`src/wrappers/item.lua`:

```lua
---The item the running event is about (GetManipulatedItem), or nil when it has none.
---@return MoonwellWrappers.Item?
function Item.fromEvent() return registry.wrap(GetManipulatedItem()) end
```

`src/wrappers/destructable.lua`:

```lua
---The destructable the running event is about (GetTriggerDestructable), or nil when it has none.
---@return MoonwellWrappers.Destructable?
function Destructable.fromEvent() return registry.wrap(GetTriggerDestructable()) end
```

`src/wrappers/timer.lua`:

```lua
---The timer whose expiry is running (GetExpiredTimer), or nil outside one.
---@return MoonwellWrappers.Timer?
function Timer.fromEvent() return registry.wrap(GetExpiredTimer()) end
```

`src/wrappers/region.lua`:

```lua
---The region the running event is about (GetTriggeringRegion), or nil when it has none.
---@return MoonwellWrappers.Region?
function Region.fromEvent() return registry.wrap(GetTriggeringRegion()) end
```

- [ ] **Step 4: Run the tests**

Run: `deno task test unit player item destructable timer region blame`
Expected: every suite passes: unit 17 tests, player 5, item 9, destructable 8, timer 7, region 2, blame 2.

- [ ] **Step 5: Commit**

```bash
git add src/wrappers/unit.lua src/wrappers/player.lua src/wrappers/item.lua src/wrappers/destructable.lua src/wrappers/timer.lua src/wrappers/region.lua tests/unit.lua tests/player.lua tests/item.lua tests/destructable.lua tests/timer.lua tests/region.lua
git commit -m "feat: fromEvent() on Unit, Player, Item, Destructable, Timer and Region"
```

---

### Task 4: `wrappers.weathereffect`

**Files:** Create `src/wrappers/weathereffect.lua`, `tests/weathereffect.lua`.

**Interfaces:**
- Consumes: `Handle.new`, `Handle.unwrap`, `Handle.created` from `wrappers.internal.handle`.
- Produces: `WeatherEffect.create(rect, effectId)`, `WeatherEffect.fromHandle(raw)`, and the methods `getHandle`,
  `isDisposed`, `enable(flag)`, `enableFor(player)`, `destroy`. The registry name is `WeatherEffect`.

- [ ] **Step 1: Write the failing tests.** Create `tests/weathereffect.lua`:

```lua
local ids = {}
native('AddWeatherEffect', function() return {} end)
native('EnableWeatherEffect', function() end)
native('RemoveWeatherEffect', function() end)
native('GetHandleId', function(raw)
    assert(raw ~= nil, 'GetHandleId of nil')
    return ids[raw] or 1
end)
native('GetLocalPlayer', function() return PLAYER_RAW end)
local WeatherEffect = require('wrappers.weathereffect')
local Rect = require('wrappers.rect')
local Player = require('wrappers.player')
eq(totalCalls(), 0)

test('a weather effect is created over a rect it does not own, and is not enabled', function()
    eq(WeatherEffect.fromHandle(nil), nil)
    local area = Rect.fromHandle({})
    local rain = WeatherEffect.create(area, 1380018290)
    expectCall('AddWeatherEffect', area.handle, 1380018290)
    eq(WeatherEffect.fromHandle(rain.handle), rain); eq(rain:getHandle(), rain.handle); eq(rain:isDisposed(), false)
    eq(callCount('EnableWeatherEffect'), 0)
    rain:destroy()
    eq(area:isDisposed(), false)
end)

test('enable passes the flag, and enableFor compares with the local player', function()
    local rain = WeatherEffect.create(Rect.fromHandle({}), 1380018290)
    checkSetters(rain, {{'EnableWeatherEffect', 'enable', true}})
    rain:enableFor(Player.fromIndex(0)); expectCall('EnableWeatherEffect', rain.handle, true)
    rain:enableFor(Player.fromHandle({})); expectCall('EnableWeatherEffect', rain.handle, false)
    failsAt(function() rain:enableFor(rain) end, 'WeatherEffect.enableFor: expected Player wrapper')
    eq(callCount('EnableWeatherEffect'), 4)
    rain:destroy()
end)

test('an unknown id is removed and raises at the caller; a nil result and a wrong rect raise too', function()
    local area, invalid = Rect.fromHandle({}), {}
    ids[invalid] = -1
    native('AddWeatherEffect', function() return invalid end)
    failsAt(function() WeatherEffect.create(area, 2054847098) end,
        'WeatherEffect.create: unknown weather effect id: 2054847098')
    expectCall('RemoveWeatherEffect', invalid); eq(callCount('RemoveWeatherEffect'), 1)
    native('AddWeatherEffect', function() return nil end)
    failsAt(function() WeatherEffect.create(area, 1) end, 'WeatherEffect.create: native returned nil')
    native('AddWeatherEffect', function() return {} end)
    resetCalls()
    failsAt(function() WeatherEffect.create({}, 1) end, 'WeatherEffect.create: expected Rect wrapper')
    eq(totalCalls(), 0)
end)

test('destroy is idempotent and guards every method', function()
    local rain = WeatherEffect.create(Rect.fromHandle({}), 1380018290)
    local raw = rain.handle
    rain:destroy(); rain:destroy()
    expectCall('RemoveWeatherEffect', raw); eq(callCount('RemoveWeatherEffect'), 1); eq(rain.handle, nil)
    eq(rain:isDisposed(), true)
    checkDisposed(rain, {'getHandle', 'enable', 'enableFor'})
end)

test('a handle the game uses again gets a wrapper of its own', function()
    local shared = {}
    native('AddWeatherEffect', function() return shared end)
    local area = Rect.fromHandle({})
    local first = WeatherEffect.create(area, 1380018290)
    first:destroy()
    local second = WeatherEffect.create(area, 1380018290)
    eq(second ~= first, true); eq(second.handle, shared); eq(first:isDisposed(), true); eq(second:isDisposed(), false)
    second:enable(true); expectCall('EnableWeatherEffect', shared, true)
    fails(function() first:enable(true) end, 'WeatherEffect.enable: WeatherEffect is disposed')
    second:destroy()
    native('AddWeatherEffect', function() return {} end)
end)
```

- [ ] **Step 2: Run them to see them fail**

Run: `deno task test weathereffect`
Expected: `module 'wrappers.weathereffect' not found`.

- [ ] **Step 3: Implement.** Create `src/wrappers/weathereffect.lua`:

```lua
local Handle = require('wrappers.internal.handle')

---@class MoonwellWrappers.WeatherEffect
---@field handle weathereffect? Read-only by convention; nil after destruction.
local WeatherEffect = {}
---@type MoonwellWrappers.Registry<MoonwellWrappers.WeatherEffect, weathereffect>
local registry = Handle.new(WeatherEffect, 'WeatherEffect')

---@param raw weathereffect?
---@return MoonwellWrappers.WeatherEffect?
---@overload fun(raw: nil): nil
function WeatherEffect.fromHandle(raw) return registry.wrap(raw) end
---Creates a weather effect over the rect. It is not drawn until enable(true). The effect does not own the rect, which
---may be destroyed afterwards. An unknown id raises an error: Warcraft returns an invalid effect (handle id -1) rather
---than nil, which this removes first.
---@param rect MoonwellWrappers.Rect
---@param effectId integer For example FourCC('RAhr').
---@return MoonwellWrappers.WeatherEffect
function WeatherEffect.create(rect, effectId)
    local raw = AddWeatherEffect(Handle.unwrap(rect, 'Rect', 'WeatherEffect.create'), effectId)
    -- Only compared with -1, which every machine gets for the same unknown id; removing it was safe in game.
    if raw ~= nil and GetHandleId(raw) == -1 then
        RemoveWeatherEffect(raw)
        error('[wrappers] WeatherEffect.create: unknown weather effect id: ' .. tostring(effectId), 2)
    end
    return (Handle.created(WeatherEffect.fromHandle(raw), 'WeatherEffect.create'))
end
---@return weathereffect
function WeatherEffect:getHandle() return (registry.require(self, 'WeatherEffect.getHandle')) end
---@return boolean
function WeatherEffect:isDisposed() return (registry.isDisposed(self, 'WeatherEffect.isDisposed')) end
---@param flag boolean
function WeatherEffect:enable(flag) EnableWeatherEffect(registry.require(self, 'WeatherEffect.enable'), flag) end
---Draws the effect on that player's machine only; enable(flag) afterwards applies to everyone.
---@param player MoonwellWrappers.Player
function WeatherEffect:enableFor(player)
    local raw = registry.require(self, 'WeatherEffect.enableFor')
    EnableWeatherEffect(raw, Handle.unwrap(player, 'Player', 'WeatherEffect.enableFor') == GetLocalPlayer())
end
function WeatherEffect:destroy()
    local raw = registry.dispose(self, 'WeatherEffect.destroy')
    if raw then RemoveWeatherEffect(raw) end
end

return WeatherEffect
```

- [ ] **Step 4: Run the tests**

Run: `deno task test weathereffect`
Expected: `weathereffect: SUITE PASSED: 5 tests`.

- [ ] **Step 5: Commit**

```bash
git add src/wrappers/weathereffect.lua tests/weathereffect.lua
git commit -m "feat: wrappers.weathereffect"
```

---

### Task 5: `wrappers.input`

**Files:** Create `src/wrappers/input.lua`, `tests/input.lua`.

**Interfaces:**
- Consumes: `Listeners.new(kind, register, route)`, `Listeners.add(set, key, callback, operation)`,
  `Listeners.remove(set, token, operation)`, `Listeners.call(cells, label, ...)` and the set's `cells` and `lists`
  from `wrappers.internal.listeners`; `Options.read(options, fields, operation)`; `Callback.check`; `Handle.unwrap`.
- Produces: `Input.onKeyDown(player, key, callback, options?)`, `Input.onKeyUp(player, key, callback)`,
  `Input.onMouseDown(player, callback)`, `Input.onMouseUp(player, callback)`, `Input.onMouseMove(player, callback)`,
  `Input.off(token)`.

The module keeps one trigger per player and key for good, so each test makes players and keys of its own
(`newPlayer()`, `numbered()`).

- [ ] **Step 1: Write the failing tests.** Create `tests/input.lua`:

```lua
EVENT_PLAYER_MOUSE_DOWN, EVENT_PLAYER_MOUSE_UP, EVENT_PLAYER_MOUSE_MOVE = {}, {}, {}
local actions, numbers, nextNumber, event = {}, {}, 0, {}
native('CreateTrigger', function() return {registrations = {}, enabled = true} end)
native('BlzTriggerRegisterPlayerKeyEvent', function(trigger, player, key, meta, down)
    trigger.player, trigger.key = player, key
    trigger.registrations[#trigger.registrations + 1] = meta .. (down and ' down' or ' up')
    return {}
end)
native('TriggerRegisterPlayerEvent', function(trigger, player, kind)
    trigger.player, trigger.kind = player, kind
    return {}
end)
native('TriggerAddAction', function(trigger, callback)
    actions[#actions + 1] = {trigger = trigger, callback = callback}
    return {}
end)
native('EnableTrigger', function(trigger) trigger.enabled = true end)
native('DisableTrigger', function(trigger) trigger.enabled = false end)
native('GetPlayerId', function(raw) return numbers[raw] end)
native('GetHandleId', function(raw) return numbers[raw] end)
native('GetTriggerPlayer', function() return event.player end)
native('BlzGetTriggerPlayerIsKeyDown', function() return event.down end)
native('BlzGetTriggerPlayerMetaKey', function() return event.meta end)
native('BlzGetTriggerPlayerMouseX', function() return event.x end)
native('BlzGetTriggerPlayerMouseY', function() return event.y end)
native('BlzGetTriggerPlayerMouseButton', function() return event.button end)
local Input = require('wrappers.input')
local Player = require('wrappers.player')
eq(totalCalls(), 0)

-- The module keeps one trigger per player and key for good, so every test uses players and keys of its own.
local function numbered()
    nextNumber = nextNumber + 1
    local raw = {}
    numbers[raw] = nextNumber
    return raw
end
local function newPlayer() return Player.fromHandle(numbered()) end
local function triggerOf(who, what)
    for _, action in ipairs(actions) do
        local trigger = action.trigger
        local matches = trigger.key == what or trigger.kind == what
        if trigger.player == who.handle and matches then return trigger, action end
    end
end
-- Simulates a synced input event: runs the action of the trigger registered for it, if that trigger is enabled.
local function deliver(who, what, data)
    data.player = who.handle
    event = data
    local trigger, action = triggerOf(who, what)
    if trigger and trigger.enabled then action.callback() end
end
local function press(who, key, meta) deliver(who, key, {down = true, meta = meta or 0}) end
local function release(who, key, meta) deliver(who, key, {down = false, meta = meta or 0}) end

test('one trigger per player and key, registered for all 16 modifier values, down and up', function()
    local who, other, q, w = newPlayer(), newPlayer(), numbered(), numbered()
    local down = Input.onKeyDown(who, q, function() end)
    eq(callCount('CreateTrigger'), 1); eq(callCount('BlzTriggerRegisterPlayerKeyEvent'), 32)
    local trigger = triggerOf(who, q)
    eq(trigger.player, who.handle); eq(trigger.key, q)
    local expected = {}
    for meta = 0, 15 do
        expected[#expected + 1] = meta .. ' down'
        expected[#expected + 1] = meta .. ' up'
    end
    eq(table.concat(trigger.registrations, ','), table.concat(expected, ','))
    local up = Input.onKeyUp(who, q, function() end)
    eq(callCount('CreateTrigger'), 1); eq(callCount('BlzTriggerRegisterPlayerKeyEvent'), 32)
    local second = Input.onKeyDown(who, w, function() end)
    local third = Input.onKeyUp(other, q, function() end)
    eq(callCount('CreateTrigger'), 3); eq(callCount('BlzTriggerRegisterPlayerKeyEvent'), 96)
    Input.off(down)
    eq(trigger.enabled, true)
    Input.off(up)
    eq(trigger.enabled, false); eq(triggerOf(who, w).enabled, true); eq(triggerOf(other, q).enabled, true)
    local again = Input.onKeyUp(who, q, function() end)
    eq(callCount('CreateTrigger'), 3); eq(trigger.enabled, true)
    Input.off(again); Input.off(second); Input.off(third)
    eq(callCount('DestroyTrigger'), 0)
end)

test('onKeyDown runs once per press with the modifiers, and onKeyUp at the release', function()
    local who, q, log = newPlayer(), numbered(), {}
    local function note(name)
        return function(player, meta, repeated, extra)
            eq(player, who); eq(extra, nil)
            log[#log + 1] = name .. ' ' .. meta .. ' ' .. tostring(repeated)
        end
    end
    local down, up = Input.onKeyDown(who, q, note('down')), Input.onKeyUp(who, q, note('up'))
    press(who, q, 3); press(who, q, 3); press(who, q, 1); release(who, q, 1)
    press(who, q); release(who, q)
    eq(table.concat(log, ', '), 'down 3 false, up 1 nil, down 0 false, up 0 nil')
    eq(#PRINTED, 0)
    Input.off(down); Input.off(up)
end)

test('the option repeats passes the repeated downs and says which they are', function()
    local who, q, log = newPlayer(), numbered(), {}
    local every = Input.onKeyDown(who, q, function(_, _, repeated) log[#log + 1] = 'every ' .. tostring(repeated) end,
        {repeats = true})
    local once = Input.onKeyDown(who, q, function(_, _, repeated) log[#log + 1] = 'once ' .. tostring(repeated) end,
        {repeats = false})
    press(who, q); press(who, q); press(who, q); release(who, q); press(who, q)
    eq(table.concat(log, ', '), 'every false, once false, every true, every true, every false, once false')
    Input.off(every); Input.off(once)
end)

test('what is held is kept per player and key, and forgotten when the last listener of a key is removed', function()
    local who, other, q, w, log = newPlayer(), newPlayer(), numbered(), numbered(), {}
    local function note(name) return function() log[#log + 1] = name end end
    local tokens = {Input.onKeyDown(who, q, note('q')), Input.onKeyDown(who, w, note('w')),
        Input.onKeyDown(other, q, note('other q'))}
    press(who, q); press(who, w); press(other, q); press(who, q); press(who, w); press(other, q)
    eq(table.concat(log, ', '), 'q, w, other q')
    -- An up listener alone keeps the key's state: the trigger stays enabled.
    local up = Input.onKeyUp(who, q, function() end)
    Input.off(tokens[1])
    tokens[1] = Input.onKeyDown(who, q, note('q'))
    press(who, q)
    eq(#log, 3)
    -- With no listener left the release is never seen, so the next listener starts over.
    Input.off(tokens[1]); Input.off(up)
    release(who, q)
    tokens[1] = Input.onKeyDown(who, q, note('q again'))
    press(who, q); press(who, q)
    eq(table.concat(log, ', '), 'q, w, other q, q again')
    for _, token in ipairs(tokens) do Input.off(token) end
end)

test('mouse listeners get the point and the button, and a move the point alone', function()
    local who, other, left, log = newPlayer(), newPlayer(), {}, {}
    local function note(name)
        return function(player, x, y, ...)
            eq(player, who)
            log[#log + 1] = name .. ' ' .. x .. ' ' .. y .. ' ' .. select('#', ...) .. ' ' .. tostring(... == left)
        end
    end
    local down, up = Input.onMouseDown(who, note('down')), Input.onMouseUp(who, note('up'))
    local move = Input.onMouseMove(who, note('move'))
    eq(callCount('CreateTrigger'), 3); eq(callCount('TriggerRegisterPlayerEvent'), 3)
    eq(callCount('BlzTriggerRegisterPlayerKeyEvent'), 0)
    for _, kind in ipairs({EVENT_PLAYER_MOUSE_DOWN, EVENT_PLAYER_MOUSE_UP, EVENT_PLAYER_MOUSE_MOVE}) do
        eq(triggerOf(who, kind).player, who.handle)
    end
    local also = Input.onMouseDown(who, function() log[#log + 1] = 'also' end)
    local far = Input.onMouseDown(other, function() log[#log + 1] = 'other' end)
    eq(callCount('CreateTrigger'), 4)
    deliver(who, EVENT_PLAYER_MOUSE_DOWN, {x = 1, y = 2, button = left})
    deliver(who, EVENT_PLAYER_MOUSE_UP, {x = 3, y = 4, button = left})
    deliver(who, EVENT_PLAYER_MOUSE_MOVE, {x = 5, y = 6, button = left})
    eq(table.concat(log, ', '), 'down 1 2 1 true, also, up 3 4 1 true, move 5 6 0 false')
    Input.off(move)
    eq(triggerOf(who, EVENT_PLAYER_MOUSE_MOVE).enabled, false)
    eq(triggerOf(who, EVENT_PLAYER_MOUSE_DOWN).enabled, true)
    deliver(who, EVENT_PLAYER_MOUSE_MOVE, {x = 7, y = 8})
    eq(#log, 4)
    Input.off(down); Input.off(up); Input.off(also); Input.off(far)
end)

test('a listener added during a firing waits; a failing one is printed and the next still runs', function()
    local who, q, log, late = newPlayer(), numbered(), {}, nil
    local first
    first = Input.onKeyDown(who, q, function()
        log[#log + 1] = 'first'
        if not late then late = Input.onKeyDown(who, q, function() log[#log + 1] = 'late' end) end
    end)
    local broken = Input.onKeyDown(who, q, function() error('intentional input probe') end)
    local last = Input.onKeyDown(who, q, function()
        log[#log + 1] = 'last'
        Input.off(first)
    end)
    press(who, q)
    eq(table.concat(log, ','), 'first,last'); eq(#PRINTED, 1)
    assert(PRINTED[1]:find('[wrappers] Input listener callback failed:', 1, true), PRINTED[1])
    assert(PRINTED[1]:find('intentional input probe', 1, true), PRINTED[1])
    Input.off(broken)
    release(who, q); press(who, q)
    eq(table.concat(log, ','), 'first,last,last,late')
    Input.off(first); Input.off(last); Input.off(late)
end)

test('arguments are checked at the caller, before any native', function()
    local who, q = newPlayer(), numbered()
    resetCalls()
    local function nothing() end
    failsAt(function() Input.onKeyDown({}, q, nothing) end, 'Input.onKeyDown: expected Player wrapper')
    failsAt(function() Input.onKeyDown(who, nil, nothing) end, 'Input.onKeyDown: expected a key, such as OSKEY_Q')
    failsAt(function() Input.onKeyDown(who, q, 'cast') end, 'Input.onKeyDown: expected a callback function')
    failsAt(function() Input.onKeyDown(who, q, nothing, true) end, 'Input.onKeyDown: expected an options table')
    failsAt(function() Input.onKeyDown(who, q, nothing, {repeat_ = true}) end,
        "Input.onKeyDown: unknown option 'repeat_'")
    failsAt(function() Input.onKeyDown(who, q, nothing, {repeats = 1}) end,
        "Input.onKeyDown: option 'repeats' expected a boolean")
    failsAt(function() Input.onKeyUp({}, q, nothing) end, 'Input.onKeyUp: expected Player wrapper')
    failsAt(function() Input.onKeyUp(who, nil, nothing) end, 'Input.onKeyUp: expected a key, such as OSKEY_Q')
    failsAt(function() Input.onKeyUp(who, q, nil) end, 'Input.onKeyUp: expected a callback function')
    for _, name in ipairs({'onMouseDown', 'onMouseUp', 'onMouseMove'}) do
        failsAt(function() Input[name]({}, nothing) end, 'Input.' .. name .. ': expected Player wrapper')
        failsAt(function() Input[name](who, nil) end, 'Input.' .. name .. ': expected a callback function')
    end
    failsAt(function() Input.off({}) end, 'Input.off: expected InputListener token')
    failsAt(function() Input.off(nil) end, 'Input.off: expected InputListener token')
    eq(totalCalls(), 0)
    local token = Input.onMouseMove(who, nothing)
    Input.off(token); Input.off(token)
    native('CreateTrigger', function() return nil end)
    failsAt(function() Input.onKeyDown(who, q, nothing) end, 'Input.onKeyDown: native returned nil')
    failsAt(function() Input.onMouseDown(who, nothing) end, 'Input.onMouseDown: native returned nil')
    native('CreateTrigger', function() return {registrations = {}, enabled = true} end)
end)
```

- [ ] **Step 2: Run them to see them fail**

Run: `deno task test input`
Expected: `module 'wrappers.input' not found`.

- [ ] **Step 3: Implement.** Create `src/wrappers/input.lua`:

```lua
local Handle = require('wrappers.internal.handle')
local Callback = require('wrappers.internal.callback')
local Listeners = require('wrappers.internal.listeners')
local Options = require('wrappers.internal.options')
local PlayerWrapper = require('wrappers.player')

---Keyboard and mouse input of one player. The events are synced: listeners run on every machine, in the same order,
---some frames after the input, so they may change game state. Each player and key, and each player and kind of mouse
---event, has one shared trigger, created by its first listener. Nothing is created at import.
local Input = {}

---@class MoonwellWrappers.InputListener

---@class MoonwellWrappers.InputSource
---@field player player
---@field key oskeytype? Set for a key's trigger.
---@field event playerevent? Set for a mouse trigger.
---@field button boolean? True when the mouse event carries a button.

---What each listener key's trigger registers for; filled before the key's first listener is added.
---@type table<string, MoonwellWrappers.InputSource>
local sources = {}
---The keys that are down, by listener key: a down while one is set is a repeat. Changed only inside the synced
---events and when a key's last listener is removed, so it is the same on every machine.
---@type table<string, boolean>
local held = {}
local KEY_DOWN_OPTIONS = {repeats = {'boolean', false}}

---@param trigger trigger
---@param id string
local function register(trigger, id)
    local source = sources[id]
    local key = source.key
    if key then
        -- The game matches the modifier keys exactly (measured on 3.0.0.24268): a registration for no modifier does
        -- not fire while Shift is held. So every one of the 16 combinations is registered, for down and for up.
        for meta = 0, 15 do
            BlzTriggerRegisterPlayerKeyEvent(trigger, source.player, key, meta, true)
            BlzTriggerRegisterPlayerKeyEvent(trigger, source.player, key, meta, false)
        end
    else
        TriggerRegisterPlayerEvent(trigger, source.player, source.event)
    end
end

---@param id string
---@param cells MoonwellWrappers.ListenerCell[]
local function route(id, cells)
    local source = sources[id]
    local player = PlayerWrapper.fromHandle(GetTriggerPlayer())
    if source.key then
        local down, repeated = BlzGetTriggerPlayerIsKeyDown(), false
        if down then
            repeated = held[id] == true
            held[id] = true
        else
            held[id] = nil
        end
        Listeners.call(cells, 'Input listener', down, player, BlzGetTriggerPlayerMetaKey(), repeated)
    elseif source.button then
        Listeners.call(cells, 'Input listener', player, BlzGetTriggerPlayerMouseX(), BlzGetTriggerPlayerMouseY(),
            BlzGetTriggerPlayerMouseButton())
    else
        Listeners.call(cells, 'Input listener', player, BlzGetTriggerPlayerMouseX(), BlzGetTriggerPlayerMouseY())
    end
end

local listeners = Listeners.new('InputListener', register, route)

---@param player player
---@param key oskeytype
---@return string id The listener key of this player and key.
local function keySource(player, key)
    local id = 'k' .. GetPlayerId(player) .. ':' .. GetHandleId(key)
    if not sources[id] then sources[id] = {player = player, key = key} end
    return id
end

---@param kind string
---@param player player
---@param event playerevent
---@param button boolean
---@return string id The listener key of this player and kind of mouse event.
local function mouseSource(kind, player, event, button)
    local id = kind .. GetPlayerId(player)
    if not sources[id] then sources[id] = {player = player, event = event, button = button} end
    return id
end

---Runs `callback` when `player` presses `key`, whatever modifier keys are held; `meta` is the sum of the held
---METAKEY_SHIFT, METAKEY_CTRL, METAKEY_ALT and METAKEY_WINKEYS. It runs once per press: the downs the game repeats
---while the key stays held are skipped, unless the option `repeats` is true; `repeated` is true for those.
---@param player MoonwellWrappers.Player
---@param key oskeytype
---@param callback fun(player: MoonwellWrappers.Player, meta: integer, repeated: boolean): ...
---@param options? {repeats: boolean?}
---@return MoonwellWrappers.InputListener
function Input.onKeyDown(player, key, callback, options)
    local rawPlayer = Handle.unwrap(player, 'Player', 'Input.onKeyDown')
    if key == nil then error('[wrappers] Input.onKeyDown: expected a key, such as OSKEY_Q', 2) end
    Callback.check(callback, 'Input.onKeyDown')
    local repeats = Options.read(options, KEY_DOWN_OPTIONS, 'Input.onKeyDown').repeats
    return (Listeners.add(listeners, keySource(rawPlayer, key), function(down, who, meta, repeated)
        if down and (repeats or not repeated) then callback(who, meta, repeated) end
    end, 'Input.onKeyDown'))
end

---Runs `callback` when `player` lets go of `key`, with the modifier keys held at that moment.
---@param player MoonwellWrappers.Player
---@param key oskeytype
---@param callback fun(player: MoonwellWrappers.Player, meta: integer): ...
---@return MoonwellWrappers.InputListener
function Input.onKeyUp(player, key, callback)
    local rawPlayer = Handle.unwrap(player, 'Player', 'Input.onKeyUp')
    if key == nil then error('[wrappers] Input.onKeyUp: expected a key, such as OSKEY_Q', 2) end
    Callback.check(callback, 'Input.onKeyUp')
    return (Listeners.add(listeners, keySource(rawPlayer, key), function(down, who, meta)
        if not down then callback(who, meta) end
    end, 'Input.onKeyUp'))
end

---Runs `callback` when `player` presses a mouse button, with the world point under the cursor.
---@param player MoonwellWrappers.Player
---@param callback fun(player: MoonwellWrappers.Player, x: number, y: number, button: mousebuttontype): ...
---@return MoonwellWrappers.InputListener
function Input.onMouseDown(player, callback)
    local rawPlayer = Handle.unwrap(player, 'Player', 'Input.onMouseDown')
    Callback.check(callback, 'Input.onMouseDown')
    local id = mouseSource('d', rawPlayer, EVENT_PLAYER_MOUSE_DOWN, true)
    return (Listeners.add(listeners, id, callback, 'Input.onMouseDown'))
end

---Runs `callback` when `player` lets go of a mouse button, with the world point under the cursor.
---@param player MoonwellWrappers.Player
---@param callback fun(player: MoonwellWrappers.Player, x: number, y: number, button: mousebuttontype): ...
---@return MoonwellWrappers.InputListener
function Input.onMouseUp(player, callback)
    local rawPlayer = Handle.unwrap(player, 'Player', 'Input.onMouseUp')
    Callback.check(callback, 'Input.onMouseUp')
    local id = mouseSource('u', rawPlayer, EVENT_PLAYER_MOUSE_UP, true)
    return (Listeners.add(listeners, id, callback, 'Input.onMouseUp'))
end

---Runs `callback` with the world point under the cursor of `player` whenever the mouse moves: 150 to 190 synced
---events a second while it does (measured on 3.0.0.24268). Remove the listener when it is not needed.
---@param player MoonwellWrappers.Player
---@param callback fun(player: MoonwellWrappers.Player, x: number, y: number): ...
---@return MoonwellWrappers.InputListener
function Input.onMouseMove(player, callback)
    local rawPlayer = Handle.unwrap(player, 'Player', 'Input.onMouseMove')
    Callback.check(callback, 'Input.onMouseMove')
    local id = mouseSource('m', rawPlayer, EVENT_PLAYER_MOUSE_MOVE, false)
    return (Listeners.add(listeners, id, callback, 'Input.onMouseMove'))
end

---Removes a listener at once, even during a firing. Removing it twice does nothing.
---@param token MoonwellWrappers.InputListener
function Input.off(token)
    Listeners.remove(listeners, token, 'Input.off')
    local id = listeners.cells[token].key
    -- A release that arrives while the key's trigger is disabled is never seen, so the state starts over.
    if #listeners.lists[id] == 0 then held[id] = nil end
end

return Input
```

- [ ] **Step 4: Run the tests**

Run: `deno task test input`
Expected: `input: SUITE PASSED: 7 tests`.

- [ ] **Step 5: Commit**

```bash
git add src/wrappers/input.lua tests/input.lua
git commit -m "feat: wrappers.input"
```

---

### Task 6: The sweep, the editor fixtures and integration

**Files:** Modify `tests/blame.lua`, `tests/imports.lua`, `tests/editor-positive.lua`, `tests/editor-positive.yue`,
`tests/editor-negative.lua`, `tools/integration.ts`.

- [ ] **Step 1: The blame sweep.** In `tests/blame.lua`, add `'weathereffect'` to the list of modules: its last line
becomes

```lua
    'timerdialog', 'trigger', 'ubersplat', 'unit', 'weathereffect',
```

and the second test covers the input functions too. Its first line, its loop and its count become:

```lua
test('damage, sync and input functions given wrong arguments point at their caller', function()
```

```lua
    for _, name in ipairs({'damage', 'sync', 'input'}) do
```

```lua
    eq(checked, 12)
```

- [ ] **Step 2: The import test.** In `tests/imports.lua`, the module list at the top gains `'weathereffect'`; its
second line becomes

```lua
    'image', 'ubersplat', 'fogmodifier', 'multiboard', 'leaderboard', 'quest', 'defeatcondition', 'timerdialog',
    'weathereffect'}) do
```

and before the test `the damage module loads Unit and what Unit loads, and nothing else` add, followed by an empty
line:

```lua
test('the input module loads the Player module and no other', function()
    require('wrappers.input')
    eq(totalCalls(), 0)
    eq(package.loaded['wrappers.player'] ~= nil, true)
    for _, name in ipairs({'unit', 'group', 'item', 'force'}) do eq(package.loaded['wrappers.' .. name], nil) end
end)
```

- [ ] **Step 3: Run the two suites**

Run: `deno task test blame imports`
Expected: `blame: SUITE PASSED: 2 tests` and `imports: SUITE PASSED: 6 tests`.

- [ ] **Step 4: The positive fixtures.** In `tests/editor-positive.lua`, between `unit:setPathing(true)` and
`unit:remove()` add:

```lua
local Input = require('wrappers.input')
local WeatherEffect = require('wrappers.weathereffect')
local pressed = Input.onKeyDown(PlayerWrapper.fromIndex(0), OSKEY_Q, function(player, meta, repeated)
    if meta == METAKEY_SHIFT and not repeated then player:addGold(1) end
end, {repeats = true})
Input.off(pressed)
Input.off(Input.onKeyUp(PlayerWrapper.fromIndex(0), OSKEY_Q, function(player, meta) print(player:getName(), meta) end))
local clicked = Input.onMouseDown(PlayerWrapper.fromIndex(0), function(player, x, y, button)
    if button == MOUSE_BUTTON_TYPE_LEFT then print(player:getName(), x + y) end
end)
Input.off(clicked)
Input.off(Input.onMouseUp(PlayerWrapper.fromIndex(0), function(_, x, y, button) print(x, y, button) end))
Input.off(Input.onMouseMove(PlayerWrapper.fromIndex(0), function(player, x, y) print(player:getId(), x, y) end))
local weatherArea = Rect.create(-512, -512, 512, 512)
local rain = WeatherEffect.create(weatherArea, 1380018290)
rain:enable(true)
rain:enableFor(PlayerWrapper.fromIndex(0))
local maybeRain = WeatherEffect.fromHandle(rain.handle)
if maybeRain then maybeRain:enable(false) end
rain:destroy()
weatherArea:destroy()
local clap = Effect.abilityArt(1095267427, EFFECT_TYPE_CASTER)
if clap then Effect.flash(clap, 0, 0) end
local third = Effect.abilityArt(1095263859, EFFECT_TYPE_SPECIAL, 3)
if third then Effect.create(third, 0, 0):destroy() end
local events = Trigger.create()
events:registerPlayerStateEvent(PlayerWrapper.fromIndex(0), PLAYER_STATE_RESOURCE_GOLD, GREATER_THAN_OR_EQUAL, 1000)
events:registerPlayerAllianceChange(PlayerWrapper.fromIndex(0), ALLIANCE_SHARED_VISION)
events:registerGameStateEvent(GAME_STATE_TIME_OF_DAY, GREATER_THAN_OR_EQUAL, 18)
events:registerTimerExpireEvent(Timer.create())
events:addAction(function()
    local eventUnit, eventPlayer, eventItem = Unit.fromEvent(), PlayerWrapper.fromEvent(), Item.fromEvent()
    if eventUnit and eventPlayer and eventItem then
        print(eventUnit:getName(), eventPlayer:getName(), eventItem:getName())
    end
    local eventTree, eventTimer, eventRegion = Destructable.fromEvent(), Timer.fromEvent(), Region.fromEvent()
    if eventTree and eventTimer and eventRegion then
        print(eventTree:getName(), eventTimer:getTimeout(), eventRegion:containsPoint(0, 0))
    end
end)
events:destroy()
```

In `tests/editor-positive.yue`, after `import "wrappers.sync" as Sync` add:

```yuescript
import "wrappers.input" as Input
import "wrappers.weathereffect" as WeatherEffect
import "wrappers.rect" as Rect
import "wrappers.effect" as Effect
import "wrappers.trigger" as Trigger
```

and append to the end of the file (inside `mw.on_main`, two spaces of indent):

```yuescript
  cast = Input.onKeyDown Player.fromIndex(0), OSKEY_Q, (player, meta) ->
    player\addGold 1 if meta == METAKEY_NONE
  Input.off cast
  Input.onMouseDown Player.fromIndex(0), (player, x, y, button) ->
    print player\getName!, x, y if button == MOUSE_BUTTON_TYPE_LEFT
  rain = WeatherEffect.create Rect.create(-512, -512, 512, 512), 1380018290
  rain\enable true
  clap = Effect.abilityArt 1095267427, EFFECT_TYPE_CASTER
  Effect.flash clap, 0, 0 if clap
  gold = Trigger.create!
  gold\registerPlayerStateEvent Player.fromIndex(0), PLAYER_STATE_RESOURCE_GOLD, GREATER_THAN_OR_EQUAL, 1000
  gold\addAction ->
    rich = Player.fromEvent!
    print rich\getName! if rich
```

- [ ] **Step 5: The negative fixture.** In `tests/editor-negative.lua`, before the final `return true` add:

```lua
local Input = require('wrappers.input')
local WeatherEffect = require('wrappers.weathereffect')
local owner = PlayerWrapper.fromIndex(0)
Input.onKeyDown(owner, OSKEY_Q, function(player) Group.create():add(player) end) -- EXPECT param-type-mismatch
Input.onMouseDown(unit, function() end) -- EXPECT param-type-mismatch
Input.off(Sync.on('load', function() end)) -- EXPECT param-type-mismatch
WeatherEffect.create(unit, 1380018290) -- EXPECT param-type-mismatch
Effect.flash(Effect.abilityArt(1095267427, EFFECT_TYPE_CASTER), 0, 0) -- EXPECT param-type-mismatch
trigger:registerTimerExpireEvent(unit) -- EXPECT param-type-mismatch
local eventUnit = Unit.fromEvent()
eventUnit:kill() -- EXPECT need-check-nil
```

- [ ] **Step 6: The one-module bundles.** In `tools/integration.ts`, add to the end of the `unused` list (after
`"sync",`) and to the end of `publicModules` (after `"sync",`), each time:

```ts
  "input",
  "weathereffect",
```

(In the `unused` list the two lines are indented by four spaces, as its neighbours are.) In `soloEntries`, after the
`sync` entry, add:

```ts
  input: {
    source: 'import "wrappers.input" as Input\nimport "wrappers.player" as Player\n' +
      "t = Input.onMouseMove Player.fromIndex(0), (player, x, y) -> print x, y\nInput.off t\n",
    allowed: ["player"],
  },
  weathereffect: {
    source: 'import "wrappers.weathereffect" as WeatherEffect\n' +
      "w = WeatherEffect.fromHandle AddWeatherEffect GetWorldBounds!, 1380018290\nw\\destroy! if w\n",
    allowed: [],
  },
```

and the closing message becomes:

```ts
  "Moonwell: Trigger-, Damage-, Sync-, Input-, WeatherEffect-, TextTag-, Multiboard-, Dialog- and Frame-only maps " +
    "bundle only what they import",
```

- [ ] **Step 7: Run every check**

Run, each as its own command: `deno task check`, `deno task lint`, `deno fmt --check`, `deno task test`,
`MOONWELL_LUAC=.tools/lua53/luac53.exe deno task check:lua`, and the integration command of the Global Constraints.
Expected: 34 suites pass (218 tests); `Lua 5.3.6 syntax: 71 files passed`; integration prints
`LuaLS: 27 intentional type errors detected at the expected lines` and
`Moonwell: Trigger-, Damage-, Sync-, Input-, WeatherEffect-, TextTag-, Multiboard-, Dialog- and Frame-only maps bundle only what they import`.
(The gate example in integration is still the v0.7.0 one; Task 8 extends it.)

- [ ] **Step 8: Commit**

```bash
git add tests/blame.lua tests/imports.lua tests/editor-positive.lua tests/editor-positive.yue tests/editor-negative.lua tools/integration.ts
git commit -m "test: the sweep, editor fixtures and one-module bundles for input and weather effects"
```

---

### Task 7: Documentation

**Files:** Modify `README.md`, `CHANGELOG.md`, `CONTRIBUTING.md`, `AGENTS.md`. Scratch (git-ignored):
`.test-work/docs8.py` and seven pieces under `.test-work/docs8/`.

The edits are applied by a script from piece files, then formatted by `deno fmt`, which reflows Markdown.

- [ ] **Step 1: Write the pieces** under `.test-work/docs8/`, with the file tools.

`readme-input.md`:

````markdown
### `wrappers.input`

- `onKeyDown(Player, key, callback, options?)`: the callback gets `(Player, meta, repeated)`; the option `repeats`
  (false) also passes the downs the game repeats while the key is held
- `onKeyUp(Player, key, callback)`: the callback gets `(Player, meta)`
- `onMouseDown(Player, callback)` and `onMouseUp(Player, callback)`: the callback gets `(Player, x, y, button)`
- `onMouseMove(Player, callback)`: the callback gets `(Player, x, y)`
- each returns a token for `off(token)`

Listeners are for one player's keyboard and mouse. The events are synced: a listener runs on every machine, in the same
order, some frames after the input, so it may change game state. Each player and key, and each player and kind of mouse
event, has one shared trigger, created by its first listener and disabled while it has none.

Keys are the `OSKEY_` constants. A key listener runs whatever modifier keys are held. `meta` is the sum of
`METAKEY_SHIFT` (1), `METAKEY_CTRL` (2), `METAKEY_ALT` (4) and `METAKEY_WINKEYS` (8); compare it to ask for one
combination, for example `meta == METAKEY_CTRL + METAKEY_SHIFT`. `button` is `MOUSE_BUTTON_TYPE_LEFT`, `_MIDDLE` or
`_RIGHT`.

`onKeyDown` runs once per press. The game repeats the down while a key stays held; those are skipped unless `repeats` is
true, and `repeated` is true for them. The module tells a press from a repeat by remembering that the key is down until
its release arrives. If a release never arrives, the next press of that key reads as a repeat, and its own release then
clears the state. That is a possibility, not something measured.

Measured on 3.0.0.24268 (the v0.8.0 probes):

- A held key repeats: the first repeat after half a second, then about 30 a second.
- The game matches the modifier keys exactly, which is why the module registers every combination: a raw
  `BlzTriggerRegisterPlayerKeyEvent` for no modifier does not fire while Shift is held.
- Shift is a key of its own (`OSKEY_LSHIFT`), with its bit already set in `meta` when it goes down.
- Typing in the chat box runs no key listener.
- Mouse points are world coordinates under the cursor. A click on the interface (the minimap) runs the listeners too.
- A quick click delivers its down and its up at the same moment.
- `onMouseMove` runs 150 to 190 times a second while the mouse moves, each a synced event. Add the listener only while
  it is needed, and remove it afterwards.

```yue
import "wrappers.input" as Input

cast = Input.onKeyDown player, OSKEY_Q, (player, meta) ->
  castFor player if meta == METAKEY_NONE

Input.onMouseDown player, (player, x, y, button) ->
  print player\getName!, "clicked at", x, y if button == MOUSE_BUTTON_TYPE_LEFT

Input.off cast
```
````

`readme-weather.md`:

````markdown
### `wrappers.weathereffect`

- `create(Rect, effectId)`
- `enable(flag)`, `enableFor(Player)`, `destroy()`

A weather effect is not drawn until `enable(true)`. `enableFor(Player)` compares with the local player, as
`setVisibleFor` does: the effect exists on every machine and is drawn on that player's only; `enable(flag)` afterwards
applies to everyone. There is no getter for whether it is enabled. The effect reads its rect when it is created and does
not own it, so the rect may be destroyed at once. `create` raises `unknown weather effect id` for an id the game does not
know: Warcraft returns an invalid effect for one, not nil.

The game's weather ids, each created by the v0.8.0 gate on 3.0.0.24268:

| Ids                                                            | Weather                                                  |
| -------------------------------------------------------------- | -------------------------------------------------------- |
| `RAhr`, `RAlr`                                                 | Ashenvale rain, heavy and light                          |
| `RLhr`, `RLlr`                                                 | Lordaeron rain, heavy and light                          |
| `SNbs`, `SNhs`, `SNls`                                         | Northrend blizzard, and snow heavy and light             |
| `WOcw`, `WOlw`                                                 | Outland wind, heavy and light                            |
| `WNcw`                                                         | Wind, heavy                                              |
| `LRaa`, `LRma`                                                 | Rays of light, rays of moonlight                         |
| `MEds`                                                         | Dalaran shield                                           |
| `FDbh`, `FDbl`, `FDgh`, `FDgl`, `FDrh`, `FDrl`, `FDwh`, `FDwl` | Dungeon fog: blue, green, red and white, heavy and light |

```yue
import "wrappers.weathereffect" as WeatherEffect
import "wrappers.rect" as Rect

area = Rect.create -1024, -1024, 1024, 1024
rain = WeatherEffect.create area, FourCC "RAhr"
area\destroy!
rain\enable true
```
````

`readme-effect.md` (it starts with an empty line):

````markdown

`Effect.abilityArt(abilityId, effecttype, index?)` returns the art an ability's data names: a model path, for the
constructors above or a missile's model, and for `EFFECT_TYPE_LIGHTNING` a lightning code for `Lightning.create`. It
returns nil when the ability has none. `index` picks an entry of a list, from 1; past the last entry the game reads the
last one, so the number of entries cannot be read. Abilities such as Blizzard keep their art on their buff and read nil.
The four constructors raise `expected a model path` for a model that is not a string, so a missing art fails at the line
that uses it; an empty string still gives an effect that draws nothing.

```yue
clap = Effect.abilityArt FourCC("AHtc"), EFFECT_TYPE_CASTER
Effect.flash clap, x, y if clap
```
````

`readme-trigger.md` (it starts with an empty line):

```markdown

Measured on 3.0.0.24268 (the v0.8.0 probes):

- `registerPlayerStateEvent` fires inside `SetPlayerState` (so inside `player:setGold`), at every change to a value that
  satisfies the comparison, not only when the limit is crossed. `Player.fromEvent()` is the player.
- `registerPlayerAllianceChange` fires inside `SetPlayerAlliance` when that player's setting of that kind toward any
  player really changes. The event names no player: `Player.fromEvent()` is nil. The generic
  `registerPlayerEvent(Player, EVENT_PLAYER_ALLIANCE_CHANGED)` fired only for `ALLIANCE_PASSIVE`.
- `registerGameStateEvent` with `GAME_STATE_TIME_OF_DAY` fires when the comparison becomes true, by a set or by the
  game's clock, and not again while it stays true.
- `registerTimerExpireEvent` fires at every expiry, before the timer's own callback, also when it is registered after
  the timer started. `Timer.fromEvent()` is the timer. The trigger does not own the timer.
```

`changelog.md`:

```markdown
## Unreleased

- New `wrappers.input`: `onKeyDown`, `onKeyUp`, `onMouseDown`, `onMouseUp` and `onMouseMove` listeners for one player,
  removable with `Input.off`. Key listeners run whatever modifier keys are held and get them as `meta`; `onKeyDown` runs
  once per press unless the option `repeats` is true.
- New `wrappers.weathereffect`: `create(Rect, effectId)`, `enable(flag)`, `enableFor(Player)` and `destroy()`. An
  unknown id raises.
- New `Effect.abilityArt(abilityId, effecttype, index?)`: the model path or lightning code an ability's data names, or
  nil.
- New Trigger registrations: `registerPlayerStateEvent`, `registerPlayerAllianceChange`, `registerGameStateEvent` and
  `registerTimerExpireEvent`.
- New `fromEvent()` on Unit, Player, Item, Destructable, Timer and Region: the wrapper of the object the running event
  is about, or nil.
- **Changed:** `Effect.create`, `Effect.attach`, `Effect.flash` and `Effect.flashOn` raise `expected a model path` for
  a model that is not a string; before, the value went to the game. Migration: pass a string, and check the result of
  `Effect.abilityArt` for nil first.
```

`contributing.md`:

```markdown
14. Additions (v0.8.0): in the gate map, `deno task gate additions` (normal build only). Only the additions gate runs.
    Every line starts with `Wrapper additions` and is also written to
    `Documents\Warcraft III\CustomMapData\moonwell-wrappers-additions.pld`.
    - Printed at once, nothing to watch:
      - `art: Thunder Clap caster <a path ending in ThunderClapCaster.mdl>`, `art: Thunder Clap missile nil`,
        `art: Flame Strike special, third entry <a path ending in FlameStrike.mdl>`,
        `art: Chain Lightning lightning CLPB` and
        `art: a missing art refused true [wrappers] Effect.create: expected a model path`;
      - `player state: gold 1000 and the event's player is the owner: true`, and the same line for 1001;
      - `alliance change: firings after the same value 0 and after two changes 2`;
      - `event: the dying unit is the footman: true`, `event: picked up Claws of Attack +3 by the hero: true`,
        `event: the dying destructable is the tree: true` and
        `event: the entered region is the zone: true by the hero: true`;
      - `weather ids created: 21 <the ids>` and `weather ids refused: 0`. If an id is refused, take it out of the
        README's table before the release;
      - `weather: an unknown id refused true [wrappers] WeatherEffect.create: unknown weather effect id: 2054847098`;
      - `rain: created and its rect destroyed. Not enabled: no rain yet` and `gate started`;
      - within a second: `game state: the time of day reached 12.00` and
        `timer expiry: trigger for this timer true, then callback`.
    - Rain, in the middle of the screen. It takes a second or two to start and to stop:
      - no rain until `rain 1: enabled for everyone NOW` (4 s), then rain;
      - after `rain 2: enabled for another player only NOW` (10 s) the rain stops;
      - after `rain 3: enabled for you only NOW` (16 s) it falls again;
      - after `rain 4: destroyed NOW` (22 s) it stops.
    - At 27 s, `art: a Thunder Clap appears NOW in the middle`: the effect plays at the centre.
    - From 30 s the game shows six input steps, one at a time; do each, then press Esc. After each Esc one line prints:
      1. tap Q once: `input 1: downs 1 with repeats 1 of which repeated 0 ups 1 meta 0`;
      2. hold Q for about two seconds: `input 2: downs 1 with repeats <many> of which repeated <one fewer> ups 1`;
      3. Shift with Q: `input 3: downs 1 … ups 1 meta 1`;
      4. a left click on the ground: `input 4: … clicks 1 (left at <x> <y>) releases 1`;
      5. moving the mouse for two seconds: `input 5: … moves <over 100>`, then `input: every listener removed`;
      6. Q and a click after the listeners are removed: every count is 0 (`input 6: downs 0 … moves 0`), then
         `gate done`.
    - No `[wrappers] ... failed` line at any point.
```

`agents.md` (it starts with an empty line):

```markdown

v0.8.0 (2026-10-01): the additions the port did not need (Moonwell spec and plan
`2026-10-01-moonwell-wrappers-additions`; probes `../wrappers-gate/PROBE-EXTRAS-RESULTS.md`).

- `wrappers.input` is the third user of `internal/listeners.lua`. Its listener keys are strings: `k<id>:<code>`, from the player id and
  the key code, for a key (one trigger, registered for all 16 modifier values, down and up, because the game matches modifiers
  exactly), and `d`, `u`, `m` plus the player id for mouse down, up and move. The held state that tells a press from a
  repeat lives per key and changes only in the synced events and in `Input.off`.
- `wrappers.weathereffect`: an unknown id gives a handle with id -1, which `create` removes and raises for, like
  `Image.create`. Weather handles are used again by the game after removal.
- `Effect.abilityArt` reads `GetAbilityEffectById` (index from 1 in the wrapper, 0 in the native; `""` becomes nil). The
  four Effect constructors check that the model is a string: the release's one change to existing behavior.
- `fromEvent()` is `fromHandle` of one native per class. Other event responses stay `fromHandle(GetKillingUnit())`; a
  `wrappers.event` module was rejected because it would bundle every widget class.
- The gate map's `additions` run (`deno task gate additions`, CONTRIBUTING step 14) writes its lines to
  `CustomMapData\moonwell-wrappers-additions.pld`. `.test-work/dry_gate_additions.lua` runs the compiled gate on stub
  natives; run it before handing a gate run to the maintainer.
```

- [ ] **Step 2: Write the script** `.test-work/docs8.py`:

```python
# Applies the v0.8.0 documentation to README, CHANGELOG, CONTRIBUTING and AGENTS (plan Task 8). Run from the root of
# the repository, or of its scratch copy: python <path>/docs8.py <folder of the pieces>. Then run `deno fmt`.
import sys

PIECES = sys.argv[1]


def read(path):
    return open(path, encoding='utf-8').read()


def piece(name):
    return read(PIECES + '/' + name)


def edit(path, pairs):
    text = read(path)
    for old, new in pairs:
        assert text.count(old) == 1, (path, old)
        text = text.replace(old, new)
    open(path, 'w', encoding='utf-8', newline='\n').write(text)


edit('README.md', [
    ('Quest, DefeatCondition, TimerDialog and Frame wrappers, Damage and Sync modules, editor completion, stable handle\n'
     'identity and explicit cleanup.\n',
     'Quest, DefeatCondition, TimerDialog, Frame and WeatherEffect wrappers, Damage, Sync and Input modules, editor\n'
     'completion, stable handle identity and explicit cleanup.\n'),
    ('- `fromHandle(raw)`: return the cached wrapper, or nil when passed nil. Never creates a game object.\n',
     '- `fromHandle(raw)`: return the cached wrapper, or nil when passed nil. Never creates a game object.\n'
     '- `fromEvent()` (Unit, Player, Item, Destructable, Timer and Region only): return the wrapper of the object the\n'
     '  running event is about, or nil when it has none. It reads `GetTriggerUnit`, `GetTriggerPlayer`,\n'
     '  `GetManipulatedItem`, `GetTriggerDestructable`, `GetExpiredTimer` and `GetTriggeringRegion`. Wrap any other event\n'
     '  response with `fromHandle`, for example `Unit.fromHandle(GetKillingUnit())`.\n'),
    ('it is, for example `Item.fromHandle(GetManipulatedItem())`.\n',
     'it is, for example `Item.fromHandle(GetSoldItem())`.\n'),
    ('LuaLS 3.19.1 conservatively treats `fromHandle` results as nullable, including for a known non-null input. Narrow the\n'
     'result with an `if`, or use `assert` when the handle is known to exist:\n',
     'LuaLS 3.19.1 conservatively treats `fromHandle` results as nullable, including for a known non-null input, and\n'
     '`fromEvent` results are nullable too. Narrow the result with an `if`, or use `assert` when the handle is known to\n'
     'exist:\n'),
    ('local triggered = Unit.fromHandle(GetTriggerUnit())\n', 'local triggered = Unit.fromEvent()\n'),
    ('- `fromIndex(index)`\n', '- `fromIndex(index)`, `fromEvent()`\n'),
    ('- `create(Player, typeId, x, y, facing)`\n', '- `create(Player, typeId, x, y, facing)`, `fromEvent()`\n'),
    ('- `create(typeId, x, y)`, `enumInRect(Rect, filter?)`\n',
     '- `create(typeId, x, y)`, `enumInRect(Rect, filter?)`, `fromEvent()`\n'),
    ('- `create(typeId, x, y, facing, scale, variation)`, `enumInRect(Rect, filter?)`\n',
     '- `create(typeId, x, y, facing, scale, variation)`, `enumInRect(Rect, filter?)`, `fromEvent()`\n'),
    ('- `create()`\n- `addRect(Rect)`,', '- `create()`, `fromEvent()`\n- `addRect(Rect)`,'),
    ('- `create()`\n- `start(timeout, periodic, callback)`,', '- `create()`, `fromEvent()`\n- `start(timeout, periodic, callback)`,'),
    ('  `registerUnitStateEvent(Unit, unitstate, limitop, value)`, `registerTimerEvent(timeout, periodic)`,\n'
     '  `registerGameEvent(gameevent)`\n',
     '  `registerUnitStateEvent(Unit, unitstate, limitop, value)`, `registerTimerEvent(timeout, periodic)`,\n'
     '  `registerGameEvent(gameevent)`, `registerPlayerStateEvent(Player, playerstate, limitop, value)`,\n'
     '  `registerPlayerAllianceChange(Player, alliancetype)`, `registerGameStateEvent(gamestate, limitop, value)`,\n'
     '  `registerTimerExpireEvent(Timer)`\n'),
    ('- `clearActions()`, `clearConditions()`, `destroy()`\n',
     '- `clearActions()`, `clearConditions()`, `destroy()`\n' + piece('readme-trigger.md')),
    ('- `create(model,x,y)`, `attach(model,Unit,attachmentPoint)`, `flash(model, x, y)`,\n'
     '  `flashOn(model, Unit, attachmentPoint)`\n',
     '- `create(model,x,y)`, `attach(model,Unit,attachmentPoint)`, `flash(model, x, y)`,\n'
     '  `flashOn(model, Unit, attachmentPoint)`, `abilityArt(abilityId, effecttype, index?)`\n'),
    ('  `setTimeScale(scale)`, `setOrientation(yaw, pitch, roll)`, `setHeight(height)`, `setZ(z)`, `playAnimation(animtype)`,\n'
     '  `destroy()`\n',
     '  `setTimeScale(scale)`, `setOrientation(yaw, pitch, roll)`, `setHeight(height)`, `setZ(z)`, `playAnimation(animtype)`,\n'
     '  `destroy()`\n' + piece('readme-effect.md')),
    ('### `wrappers.dialog`\n', piece('readme-weather.md') + '### `wrappers.dialog`\n'),
    ('Player indices must be integers below `bj_MAX_PLAYER_SLOTS`, including neutral slots.',
     piece('readme-input.md') + 'Player indices must be integers below `bj_MAX_PLAYER_SLOTS`, including neutral slots.'),
    ('its label and nothing else is affected. The labels are `Timer`, `Trigger`, `Dialog button` and `Frame event`, printed as\n',
     'its label and nothing else is affected. The labels are `Timer`, `Trigger`, `Dialog button`, `Frame event`,\n'
     '`Damage listener`, `Sync listener` and `Input listener`, printed as\n'),
    ('ordinary natives, then convert handles with `fromHandle` as needed.',
     'ordinary natives, then convert handles with `fromHandle` as needed, or use `fromEvent()`.'),
])
edit('CHANGELOG.md', [('# Changelog\n\n', '# Changelog\n\n' + piece('changelog.md'))])
edit('CONTRIBUTING.md', [
    ('    - step 15: `Wrapper port gate done`, and no `[wrappers] ... failed` line at any point.\n\n',
     '    - step 15: `Wrapper port gate done`, and no `[wrappers] ... failed` line at any point.\n'
     + piece('contributing.md')),
    ('    player, and frame events from the second player printing that player\'s name on both machines; no desync.\n',
     '    player, and frame events from the second player printing that player\'s name on both machines; no desync. It\n'
     '    also covers v0.8.0: `wrappers.input` listeners running on both machines for the second player\'s keys and mouse,\n'
     '    and `weather:enableFor` drawing only for that player; no desync.\n'),
])
agents = read('AGENTS.md').rstrip('\n') + '\n' + piece('agents.md')
agents_pairs = [
    ('leaderboards, quests, defeat conditions, timer dialogs). v0.5.0 adds frames. v0.7.0 adds damage events and sync, for the\n'
     'wc3-lib port, on a shared internal listener list (`internal/listeners.lua`). Do not add gameplay systems, implicit\n',
     'leaderboards, quests, defeat conditions, timer dialogs). v0.5.0 adds frames. v0.7.0 adds damage events and sync, for the\n'
     'wc3-lib port, on a shared internal listener list (`internal/listeners.lua`). v0.8.0 adds input listeners, weather\n'
     'effects, art from ability data, four Trigger registrations and `fromEvent()`. Do not add gameplay systems, implicit\n'),
]
open('AGENTS.md', 'w', encoding='utf-8', newline='\n').write(agents)
edit('AGENTS.md', agents_pairs)
print('done')
```

- [ ] **Step 3: Apply and format**

```bash
python .test-work/docs8.py .test-work/docs8
deno fmt README.md CHANGELOG.md CONTRIBUTING.md AGENTS.md
deno fmt --check
```

Expected: `done`, and the check is clean. `git diff --stat` shows the four files: README 135 lines added and 24
removed, CONTRIBUTING 37 and 1, CHANGELOG 17, AGENTS 19 and 1.

- [ ] **Step 4: Read the result.** Check in `README.md` that no code span is broken across two lines by the
reflow, that the weather table kept its eight rows, and that the two new sections sit before `### wrappers.dialog`
and before the paragraph that starts `Player indices must be integers`.

- [ ] **Step 5: Commit**

```bash
git add README.md CHANGELOG.md CONTRIBUTING.md AGENTS.md
git commit -m "docs: input, weather effects, ability art, the new registrations and fromEvent"
```

---

### Task 8: The gate

**Files:** Modify `examples/gate.yue`; in the gate map (not under git) `gate.ts`. Scratch:
`.test-work/dry_gate_additions.lua`.

- [ ] **Step 1: The gate example.** In `examples/gate.yue`, add to the header comment, after the line about
`port = true`:

```yuescript
-- Set additions = true for a separate run of the v0.8.0 additions gate only (art, events, weather, input).
```

after `import "wrappers.sync" as Sync`:

```yuescript
import "wrappers.input" as Input
import "wrappers.weathereffect" as WeatherEffect
```

after `port = false`:

```yuescript
additions = false
```

before the comment `-- Start just after the map loads`, the new gate, followed by an empty line (write it with a
file tool: it contains backslashes):

```yuescript
-- v0.8.0 additions. Run with additions = true. CONTRIBUTING step 14 lists every message; they are also written to
-- CustomMapData\moonwell-wrappers-additions.pld. First lines to read, then rain, one effect, and six input steps that
-- the game shows one at a time.
ADDITIONS_WEATHER = {
  "RAhr", "RAlr", "RLhr", "RLlr", "SNbs", "SNhs", "SNls", "WOcw", "WOlw", "WNcw", "LRaa", "LRma", "MEds", "FDbh"
  "FDbl", "FDgh", "FDgl", "FDrh", "FDrl", "FDwh", "FDwl"
}
ADDITIONS_INPUT = {
  "tap Q once"
  "hold Q down for about two seconds, then let go"
  "hold Shift, tap Q once, then let go of Shift"
  "left-click the ground once"
  "move the mouse over the ground for about two seconds"
  "tap Q once and left-click the ground once (the listeners are removed now)"
}
additionsGate = (owner) ->
  lines = {}
  say = (...) ->
    parts = {"Wrapper additions"}
    for index = 1, select "#", ...
      parts[#parts + 1] = tostring (select index, ...)
    line = table.concat parts, " "
    print line
    lines[#lines + 1] = line
    line
  save = ->
    PreloadGenClear!
    PreloadGenStart!
    for line in *lines
      -- A Preload line keeps 259 characters: longer lines continue on the next.
      for at = 1, #line, 200
        Preload (line\sub(at, at + 199)\gsub "\"", "'")
    PreloadGenEnd "moonwell-wrappers-additions.pld"
    PreloadGenClear!

  -- Art from ability data.
  clapId = $FourCC "AHtc"
  clap = assert Effect.abilityArt(clapId, EFFECT_TYPE_CASTER), "Thunder Clap has no caster art"
  say "art: Thunder Clap caster", clap
  say "art: Thunder Clap missile", Effect.abilityArt clapId, EFFECT_TYPE_MISSILE
  say "art: Flame Strike special, third entry", Effect.abilityArt $FourCC("AHfs"), EFFECT_TYPE_SPECIAL, 3
  say "art: Chain Lightning lightning", Effect.abilityArt $FourCC("AOcl"), EFFECT_TYPE_LIGHTNING
  missingOk, missingMessage = pcall Effect.create, Effect.abilityArt(clapId, EFFECT_TYPE_MISSILE), 0, 0
  say "art: a missing art refused", not missingOk, missingMessage

  -- The four registrations, with fromEvent.
  rich = Trigger.create!
  rich\registerPlayerStateEvent owner, PLAYER_STATE_RESOURCE_GOLD, GREATER_THAN_OR_EQUAL, 1000
  rich\addAction ->
    say "player state: gold", owner\getGold!, "and the event's player is the owner:", Player.fromEvent! == owner
  owner\setGold 500
  owner\setGold 1000
  owner\setGold 1001
  rich\destroy!
  other = Player.fromIndex 1
  shared = owner\getAlliance other, ALLIANCE_SHARED_VISION
  changes = 0
  allied = Trigger.create!
  allied\registerPlayerAllianceChange owner, ALLIANCE_SHARED_VISION
  allied\addAction -> changes += 1
  owner\setAlliance other, ALLIANCE_SHARED_VISION, shared
  same = changes
  owner\setAlliance other, ALLIANCE_SHARED_VISION, not shared
  owner\setAlliance other, ALLIANCE_SHARED_VISION, shared
  say "alliance change: firings after the same value", same, "and after two changes", changes
  allied\destroy!
  noon = Trigger.create!
  noon\registerGameStateEvent GAME_STATE_TIME_OF_DAY, GREATER_THAN_OR_EQUAL, 12
  noon\addAction (self) ->
    say "game state: the time of day reached", string.format("%.2f", GetFloatGameState GAME_STATE_TIME_OF_DAY)
    self\destroy!
  SuspendTimeOfDay false
  SetFloatGameState GAME_STATE_TIME_OF_DAY, 11.99
  order = {}
  ticker = Timer.create!
  expiry = Trigger.create!
  expiry\registerTimerExpireEvent ticker
  expiry\addAction -> order[#order + 1] = "trigger for this timer " .. tostring(Timer.fromEvent! == ticker)
  ticker\start 0.5, false, (self) ->
    order[#order + 1] = "callback"
    say "timer expiry:", table.concat order, ", then "
    expiry\destroy!
    self\destroy!

  -- fromEvent for a unit, an item, a destructable and a region.
  footman = Unit.create owner, $FourCC("hfoo"), -700, -500, 0
  fallen = Trigger.create!
  fallen\registerUnitEvent footman, EVENT_UNIT_DEATH
  fallen\addAction -> say "event: the dying unit is the footman:", Unit.fromEvent! == footman
  footman\kill!
  hero = Unit.create owner, $FourCC("Hpal"), -700, -300, 0
  pickups = Trigger.create!
  pickups\registerUnitEvent hero, EVENT_UNIT_PICKUP_ITEM
  pickups\addAction ->
    picked = Item.fromEvent!
    say "event: picked up", picked and picked\getName!, "by the hero:", Unit.fromEvent! == hero
  assert hero\addItemById($FourCC("ratc")), "no item added"
  tree = Destructable.create $FourCC("LTlt"), -704, -704, 270, 1, 0
  felled = Trigger.create!
  felled\registerDeathEvent tree
  felled\addAction -> say "event: the dying destructable is the tree:", Destructable.fromEvent! == tree
  tree\kill!
  area = Rect.create -1000, 200, -800, 400
  zone = Region.create!
  zone\addRect area
  entered = Trigger.create!
  entered\registerEnterRegion zone
  entered\addAction ->
    say "event: the entered region is the zone:", Region.fromEvent! == zone, "by the hero:", Unit.fromEvent! == hero
  hero\setPosition -900, 300

  -- Weather: which of the game's ids exist, and an unknown one.
  field = Rect.create -1024, -1024, 1024, 1024
  created, refused = {}, {}
  for code in *ADDITIONS_WEATHER
    ok, made = pcall WeatherEffect.create, field, FourCC code
    if ok
      made\destroy!
      created[#created + 1] = code
    else
      refused[#refused + 1] = code
  say "weather ids created:", #created, table.concat created, " "
  say "weather ids refused:", #refused, table.concat refused, " "
  unknownOk, unknownMessage = pcall WeatherEffect.create, field, FourCC "zzzz"
  say "weather: an unknown id refused", not unknownOk, unknownMessage
  rain = WeatherEffect.create field, $FourCC "RAhr"
  field\destroy!
  say "rain: created and its rect destroyed. Not enabled: no rain yet"
  save!

  -- The six input steps; Esc ends each one.
  inputGate = ->
    counts = {}
    reset = ->
      counts = {
        downs: 0, every: 0, repeated: 0, ups: 0, meta: "-", clicks: 0, click: "-", releases: 0, moves: 0
      }
    reset!
    tokens = {}
    listen = (token) -> tokens[#tokens + 1] = token
    listen Input.onKeyDown owner, OSKEY_Q, (player, meta) ->
      counts.downs += 1
      counts.meta = meta .. (player == owner and "" or " from another player")
    every = (_, _, repeated) ->
      counts.every += 1
      counts.repeated += 1 if repeated
    listen Input.onKeyDown owner, OSKEY_Q, every, repeats: true
    listen Input.onKeyUp owner, OSKEY_Q, -> counts.ups += 1
    listen Input.onMouseDown owner, (_, x, y, button) ->
      counts.clicks += 1
      name = button == MOUSE_BUTTON_TYPE_LEFT and "left" or button == MOUSE_BUTTON_TYPE_RIGHT and "right" or "other"
      counts.click = name .. string.format(" at %.0f %.0f", x, y)
    listen Input.onMouseUp owner, -> counts.releases += 1
    listen Input.onMouseMove owner, -> counts.moves += 1
    step = 1
    prompt = (result) ->
      ClearTextMessages!
      DisplayTimedTextToPlayer GetLocalPlayer!, 0, 0, 600, result if result
      text = "Wrapper additions gate done. Say so."
      if ADDITIONS_INPUT[step]
        text = "Step " .. step .. " of " .. #ADDITIONS_INPUT .. ": " .. ADDITIONS_INPUT[step] .. ". Then press Esc."
      DisplayTimedTextToPlayer GetLocalPlayer!, 0, 0, 600, text
    escape = Trigger.create!
    escape\registerPlayerEvent owner, EVENT_PLAYER_END_CINEMATIC
    escape\addAction (self) ->
      result = say "input " .. step .. ": downs", counts.downs, "with repeats", counts.every, "of which repeated",
        counts.repeated, "ups", counts.ups, "meta", counts.meta, "clicks", counts.clicks, "(" .. counts.click .. ")",
        "releases", counts.releases, "moves", counts.moves
      reset!
      step += 1
      if step == #ADDITIONS_INPUT
        for token in *tokens
          Input.off token
        say "input: every listener removed"
      if step > #ADDITIONS_INPUT
        self\destroy!
        say "gate done"
      save!
      prompt result
    prompt!

  local thunder
  tick = 0
  steps = Timer.create!
  steps\start 1, true, (self) ->
    tick += 1
    if tick == 4
      rain\enable true
      say "rain 1: enabled for everyone NOW"
    elseif tick == 10
      rain\enableFor other
      say "rain 2: enabled for another player only NOW"
    elseif tick == 16
      rain\enableFor owner
      say "rain 3: enabled for you only NOW"
    elseif tick == 22
      rain\destroy!
      say "rain 4: destroyed NOW"
    elseif tick == 27
      thunder = Effect.create clap, 0, 0
      say "art: a Thunder Clap appears NOW in the middle"
    elseif tick == 30
      thunder\destroy!
      self\destroy!
      save!
      inputGate!
  say "gate started: lines to read first, then watch the middle of the screen"
```

and in `mw.on_main`, the line `    if port` becomes:

```yuescript
    if additions
      additionsGate owner
    elseif port
```

- [ ] **Step 2: Integration with the new gate example**

Run the integration command of the Global Constraints.
Expected: it ends with
`Gate example: every module builds and editor diagnostics are clean; game execution remains manual`, after
`Check passed: 34 module(s) reachable from main`.

- [ ] **Step 3: Commit the gate example**

```bash
git add examples/gate.yue
git commit -m "test: the v0.8.0 additions gate run"
```

- [ ] **Step 4: The gate map's run.** In `C:\Users\mdlsvensson\Repo\wrappers-gate\gate.ts`, add the usage line after
the `port` one:

```ts
//   additions         v0.8.0 additions gate (step 14)
```

replace the `Run` type and the `runs` table with:

```ts
type Run = {
  probes: boolean;
  presentation: boolean;
  ui: boolean;
  frames: boolean;
  port: boolean;
  additions: boolean;
  minify: boolean;
};
const off: Run = {
  probes: false,
  presentation: false,
  ui: false,
  frames: false,
  port: false,
  additions: false,
  minify: false,
};
const runs: Record<string, Run> = {
  "core": off,
  "probes": { ...off, probes: true },
  "presentation": { ...off, presentation: true },
  "core-min": { ...off, minify: true },
  "presentation-min": { ...off, presentation: true, minify: true },
  "ui": { ...off, ui: true },
  "ui-min": { ...off, ui: true, minify: true },
  "frames": { ...off, frames: true },
  "frames-min": { ...off, frames: true, minify: true },
  "port": { ...off, port: true },
  "additions": { ...off, additions: true },
};
```

and in `variant`, after the `port = false` replacement, add:

```ts
  text = replace(text, "additions = false", `additions = ${run.additions}`);
```

- [ ] **Step 5: Build the run without launching**

Run, in the gate map: `deno task gate additions --no-launch`
Expected: `Gate map: gate-maps/additions.w3x`, and `dist/stage/lua/gate_additions.lua` exists.

- [ ] **Step 6: Dry run.** Create `.test-work/dry_gate_additions.lua` in the wrappers repository:

```lua
-- Dry run of the compiled additions gate outside the game: the real wrappers on stub natives, with the events the
-- gate waits for simulated, a clock driven by hand, and the six input steps played. It shows that the gate runs to its
-- end and prints what CONTRIBUTING expects; it proves nothing about the game.
--   GATE_DIR=<folder of the compiled gate> GATE_MODULE=<its module name> SRC=<wrappers src> yue -e <this file>
package.path = os.getenv('SRC') .. '/?.lua;' .. os.getenv('GATE_DIR') .. '/?.lua;' .. package.path
local function handle(kind) return setmetatable({kind = kind}, {__tostring = function() return kind end}) end
local stubbed = {}
setmetatable(_G, {__index = function(_, name)
    local value
    if name:match('^[A-Z][A-Z0-9_]*$') then
        value = handle(name)
    elseif name:match('^Create') then
        value = function() return handle(name) end
    else
        value = function() return nil end
        stubbed[#stubbed + 1] = name
    end
    rawset(_G, name, value)
    return value
end})
bj_MAX_PLAYER_SLOTS, bj_MAX_PLAYERS = 28, 24
function FourCC(code) return (string.unpack('>I4', code)) end
local players = {}
function Player(index)
    players[index] = players[index] or {index = index, gold = 0, vision = {}}
    return players[index]
end
function GetPlayerId(player) return player.index end
function GetLocalPlayer() return Player(0) end
function Rect() return handle('rect') end

-- Triggers, and the events the gate registers.
local context, clock, timers = {}, 0, {}
local triggers = {}
function CreateTrigger()
    local trigger = {actions = {}, events = {}, enabled = true}
    triggers[#triggers + 1] = trigger
    return trigger
end
function TriggerAddAction(trigger, action) trigger.actions[#trigger.actions + 1] = action; return {} end
function TriggerClearConditions() end
function DestroyTrigger(trigger) trigger.enabled, trigger.actions = false, {} end
function EnableTrigger(trigger) trigger.enabled = true end
function DisableTrigger(trigger) trigger.enabled = false end
local function register(kind)
    return function(trigger, subject, detail) trigger.events[#trigger.events + 1] = {kind, subject, detail}; return {} end
end
TriggerRegisterPlayerStateEvent = register('state')
TriggerRegisterPlayerAllianceChange = register('alliance')
TriggerRegisterUnitEvent = register('unit')
TriggerRegisterDeathEvent = register('death')
TriggerRegisterEnterRegion = register('enter')
TriggerRegisterTimerExpireEvent = register('expire')
TriggerRegisterPlayerEvent = register('player')
function TriggerRegisterGameStateEvent(trigger) trigger.events[#trigger.events + 1] = {'noon'}; return {} end
function BlzTriggerRegisterPlayerKeyEvent(trigger, player, key)
    trigger.events[#trigger.events + 1] = {'key', player, key}
    return {}
end
local function fire(kind, subject, detail, data)
    context = data or {}
    for index = 1, #triggers do
        local trigger, matches = triggers[index], false
        for _, event in ipairs(trigger.events) do
            if event[1] == kind and event[2] == subject and (detail == nil or event[3] == detail) then matches = true end
        end
        if matches and trigger.enabled then
            for _, action in ipairs(trigger.actions) do action() end
        end
    end
    context = {}
end
function GetTriggerPlayer() return context.player end
function GetTriggerUnit() return context.unit end
function GetManipulatedItem() return context.item end
function GetTriggerDestructable() return context.destructable end
function GetTriggeringRegion() return context.region end
function GetExpiredTimer() return context.timer end
function BlzGetTriggerPlayerIsKeyDown() return context.down end
function BlzGetTriggerPlayerMetaKey() return context.meta end
function BlzGetTriggerPlayerMouseX() return context.x end
function BlzGetTriggerPlayerMouseY() return context.y end
function BlzGetTriggerPlayerMouseButton() return context.button end

-- Player state, alliances and the time of day, as the probe measured them.
function GetPlayerState(player) return player.gold end
function SetPlayerState(player, state, value)
    local changed = player.gold ~= value
    player.gold = value
    if changed and value >= 1000 then fire('state', player, nil, {player = player}) end
end
function GetPlayerAlliance(giver, taker) return giver.vision[taker] ~= false end
function SetPlayerAlliance(giver, taker, kind, flag)
    local changed = GetPlayerAlliance(giver, taker) ~= flag
    giver.vision[taker] = flag
    if changed then fire('alliance', giver, kind) end
end
local time = 8
function SetFloatGameState(_, value) time = value end
function GetFloatGameState() return time end

-- Widgets.
function CreateUnit() return handle('unit') end
function KillUnit(unit) fire('unit', unit, EVENT_UNIT_DEATH, {unit = unit}) end
function UnitAddItemById(unit)
    local item = handle('item')
    fire('unit', unit, EVENT_UNIT_PICKUP_ITEM, {unit = unit, item = item})
    return item
end
function GetItemName() return 'Claws of Attack +3' end
function KillDestructable(tree) fire('death', tree, nil, {destructable = tree}) end
local regions = {}
function CreateRegion() local region = handle('region'); regions[#regions + 1] = region; return region end
function SetUnitPosition(unit) fire('enter', regions[1], nil, {region = regions[1], unit = unit}) end

-- Effects and weather.
function GetAbilityEffectById(ability, kind, index)
    if ability == FourCC('AHtc') then return kind == EFFECT_TYPE_CASTER and 'ThunderClapCaster.mdl' or '' end
    if ability == FourCC('AHfs') then return index >= 2 and 'FlameStrike.mdl' or 'FlameStrike' .. (index + 1) .. '.mdl' end
    if ability == FourCC('AOcl') then return kind == EFFECT_TYPE_LIGHTNING and 'CLPB' or '' end
    return ''
end
function AddSpecialEffect() return handle('effect') end
local weather = {}
function AddWeatherEffect(_, id)
    local known = id ~= FourCC('zzzz') and id ~= FourCC('WNcw')
    local effect = handle('weather')
    weather[effect] = known and 1 or -1
    return effect
end
function GetHandleId(raw) return weather[raw] or 81 end
function EnableWeatherEffect(_, flag) print('  (rain drawn here: ' .. tostring(flag) .. ')') end

-- Timers and files.
function CreateTimer() local timer = {}; timers[#timers + 1] = timer; return timer end
function TimerStart(timer, timeout, periodic, callback)
    timer.due, timer.timeout, timer.periodic, timer.callback = clock + timeout, timeout, periodic, callback
end
function PauseTimer(timer) timer.due = nil end
function DestroyTimer(timer) timer.due = nil end
local files, buffer = {}, {}
function PreloadGenClear() buffer = {} end
function Preload(text) assert(#text <= 259, 'a line over 259 characters'); buffer[#buffer + 1] = text end
function PreloadGenEnd(path) files[path] = buffer end
function DisplayTimedTextToPlayer(_, _, _, _, text) print('  PROMPT ' .. text) end
local main
package.preload['moonwell'] = function() return {on_main = function(fn) main = fn end} end

local function advance(seconds)
    local finish = clock + seconds
    while clock < finish do
        clock = clock + 0.05
        if time < 12 then
            time = time + 0.0025
            if time >= 12 then fire('noon') end
        end
        for index = 1, #timers do
            local timer = timers[index]
            if timer.due and timer.due <= clock then
                timer.due = timer.periodic and timer.due + timer.timeout or nil
                fire('expire', timer, nil, {timer = timer})
                if timer.callback then timer.callback() end
            end
        end
    end
end

require(os.getenv('GATE_MODULE'))
main()
advance(31)
local owner = Player(0)
local function key(down, meta) fire('key', owner, OSKEY_Q, {player = owner, down = down, meta = meta}) end
local function mouse(kind, x, y)
    fire('player', owner, kind, {player = owner, x = x, y = y, button = MOUSE_BUTTON_TYPE_LEFT})
end
local function escape() fire('player', owner, EVENT_PLAYER_END_CINEMATIC, {player = owner}) end
key(true, 0); key(false, 0); escape()
key(true, 0); for _ = 1, 40 do key(true, 0) end; key(false, 0); escape()
key(true, 1); key(false, 1); escape()
mouse(EVENT_PLAYER_MOUSE_DOWN, 120, -340); mouse(EVENT_PLAYER_MOUSE_UP, 120, -340); escape()
for index = 1, 300 do mouse(EVENT_PLAYER_MOUSE_MOVE, index, index) end; escape()
key(true, 0); key(false, 0); mouse(EVENT_PLAYER_MOUSE_DOWN, 1, 1); mouse(EVENT_PLAYER_MOUSE_UP, 1, 1); escape()
escape()
for path, lines in pairs(files) do print('FILE ' .. path .. ': ' .. #lines .. ' lines') end
table.sort(stubbed)
print('natives without a stub of their own: ' .. table.concat(stubbed, ', '))
```

Run, from the wrappers repository:

```bash
GATE_DIR=../wrappers-gate/dist/stage/lua GATE_MODULE=gate_additions SRC=src yue -e .test-work/dry_gate_additions.lua
```

Expected: every line of CONTRIBUTING step 14 in order, ending with `Wrapper additions gate done`, the prompt
`Wrapper additions gate done. Say so.` and `FILE moonwell-wrappers-additions.pld: 32 lines`; no Lua error. (The stub
refuses one weather id, `WNcw`, to exercise that branch: `weather ids created: 20` there.)

- [ ] **Step 7: The maintainer runs the gate.** Give exactly this:

> `deno task gate additions` in `C:\Users\mdlsvensson\Repo\wrappers-gate`. About 30 seconds to watch, then six steps
> at your own pace.
>
> 1. Rain, in the middle of the screen (it takes a second or two to start and stop): none at first; it starts after
>    `rain 1`; stops after `rain 2`; starts again after `rain 3`; stops after `rain 4`.
> 2. At `a Thunder Clap appears NOW`: one effect plays in the middle.
> 3. Then the game shows six steps, one at a time. Do each, then press Esc. When it reads
>    `Wrapper additions gate done. Say so.`, say so, with what you saw of the rain and the effect.

Read the lines from `Documents\Warcraft III\CustomMapData\moonwell-wrappers-additions.pld` and compare them with
CONTRIBUTING step 14. If a weather id was refused, remove it from the README's table and from `ADDITIONS_WEATHER`, and
say so in the release record.

---

### Task 9: Release

**Files:** Modify `README.md`, `CHANGELOG.md`, `CONTRIBUTING.md`, `AGENTS.md`; Moonwell's `AGENTS.md`, `CHANGELOG.md`
and the roadmap.

- [ ] **Step 1: Record the gate.** In `CHANGELOG.md` rename `## Unreleased` to `## 0.8.0 (2026-10-01)` (the date of
the gate run) and add a `### Release gate` section under its list, in the form of 0.7.0's: the automated checks (34
suites with the new `input` (7 tests) and `weathereffect` (5 tests) suites; Lua 5.3.6 syntax, 71 files; the builds, the
nine one-module bundles and bundled execution; LuaLS fixtures with 27 expected negative diagnostics; the native-call
check; the gate example), 66 mutations each caught by a test, then the in-game gate with the measured lines (the
weather ids created, the counts of the six input steps, what the maintainer saw of the rain and the effect), and the
two probes before the design with their results file. In `CONTRIBUTING.md` add a `v0.8.0:` record after the v0.7.0 one
in the in-game gate section. In `README.md` update the Status paragraph to `v0.8.0` and the two `v0.7.0` mentions of
the GitHub example (`To use the published ... tag` and `tag = "..."`). In `AGENTS.md` add `v0.8.0` to the list of tags
in the first paragraph and the gate result to the v0.8.0 paragraph.

- [ ] **Step 2: Run every check** of the Global Constraints. Expected: as Task 6, Step 7, with the new gate example.

- [ ] **Step 3: Commit, tag and push**

```bash
git add README.md CHANGELOG.md CONTRIBUTING.md AGENTS.md
git commit -m "release: v0.8.0"
git tag v0.8.0
git push origin main v0.8.0
```

Then create the GitHub pre-release:
`"C:\Program Files\GitHub CLI\gh.exe" release create v0.8.0 --prerelease --title "v0.8.0" --notes "<the CHANGELOG's 0.8.0 list>"`.

- [ ] **Step 4: Tag consumption.** In a fresh map (`../wrappers-tag-check-080`, made with Moonwell's `init --link`),
use the README's GitHub configuration, copy `examples/gate.yue` to `src/main.yue`, run check, build and
`build --minify`; confirm `moonwell.lock` records the tag's commit and that the fetched files match the tag's `src/`
(33 files); remove the map's `.moonwell/` and check again: the lock must not change. Record it in `CONTRIBUTING.md`
(tag gate section) and `AGENTS.md`, commit `docs: record the v0.8.0 tag consumption`, and push.

- [ ] **Step 5: Moonwell's records.** In `C:\Users\mdlsvensson\Repo\moonwell`: `AGENTS.md` gains a state bullet for
wrappers v0.8.0 (what it adds, the probes' findings, the gate, the tag) and "Next work" says phase 4 item 1 is done
and item 2 (assets shipped by libraries) is next; the roadmap marks item 1 of phase 4 released; `CHANGELOG.md`'s
Unreleased section mentions the design and plan. Run `deno fmt --check` there, commit
`docs: wrappers v0.8.0 is released (roadmap phase 4, item 1)`, push, and check CI with `gh run list`.
