# Moonwell Presentation Wrappers (wrappers v0.3.0) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add world presentation wrappers (TextTag, Sound, Lightning, Image, Ubersplat, FogModifier), deeper Effect,
fire-and-forget helpers, local visibility helpers and item/destructable enumeration to `moonwell-wrappers`.

**Architecture:** Handwritten annotated Lua 5.3 modules in `../moonwell-wrappers/src/wrappers/`, following v0.2.0's
patterns: `Handle.new` registries, `Handle.unwrap`/`unwrapWidget` for arguments, strong caches with explicit
`destroy()`. A new `internal/options.lua` validates options tables. Objects the game can end on its own (expiring text
tags, sounds released when done) never get wrappers; static helpers that return nothing cover them.

**Tech Stack:** Lua 5.3 (game), YueScript 0.34.2 embedded Lua 5.4 test VM, LuaLS 3.19.1, Lua 5.3.6 `luac`, Deno
tooling, Moonwell 0.5.0 consumer fixtures.

**Spec:** `docs/superpowers/specs/2026-09-29-moonwell-wrappers-presentation-design.md` (builds on the v0.1.0 and v0.2.0
wrapper specs in the same folder).

## Global Constraints

- Product code, tests and library docs live in `C:/Users/mdlsvensson/Repo/moonwell-wrappers`; this plan and the spec
  stay in Moonwell. Commit on `main` in each repository; the maintainer pushes. No tags, no publishing.
- No Node.js, npm packages, `node:` or `npm:` specifiers. Deno tooling uses built-ins and `jsr:@std/*` only.
- Runtime Lua ships only under `src/wrappers/`. Literal `require`s only; no umbrella module; no globals; no native call
  or game-object creation at import time.
- The game's Lua lacks `collectgarbage`, `debug`, `io`, `package`, `dofile`, `loadfile`; `os` has only `clock`, `date`,
  `difftime`, `time`. Shipped code must not use them. `math` is available.
- Additive release: every v0.2.0 call keeps its behavior. The only observable change is `Effect.attach` accepting any
  Widget (a wrong argument reports `expected Widget wrapper` instead of `expected Unit wrapper`).
- A module imports another public module only to return its wrappers. Every new module imports only
  `wrappers.internal.*`. Convert wrapper arguments with `Handle.unwrap(value, 'Class', operation)` or
  `Handle.unwrapWidget(value, operation)`.
- Every public function carries LuaLS annotations. `fromHandle` stays conservatively nullable. The only diagnostic
  suppressions are line-local `---@diagnostic disable-next-line: param-type-mismatch` on native calls that pass a nil
  filter, each preceded by `-- Warcraft accepts a null filter; the generated JASS signature cannot express that.`
- Misuse errors read `[wrappers] <Class>.<method>: ...`. Validate receivers, arguments and options before any
  side-effecting native.
- Never iterate a table keyed by tables with `pairs` when the loop calls natives. Use arrays.
- No wrapper for an object the game can destroy by itself: no lifespan/permanence setters on TextTag, never
  `KillSoundWhenDone` on a wrapped Sound. No getters for machine-local values (`GetSoundIsPlaying`,
  `BlzGetLocalSpecialEffect*`).
- Before every commit in `moonwell-wrappers`, run all its checks (below). Before a Moonwell commit, run Moonwell's
  checks from its AGENTS.md (for a docs-only change, `deno fmt --check` is the relevant one, but run them all).

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

If `deno fmt --check` reports only formatting differences in files you changed, run `deno fmt` and re-run the check.
`deno task test <suite>` runs one suite (`tests/<suite>.lua`); suite names are lowercase letters only, and every
`tests/<name>.lua` except `support.lua` is discovered automatically.

Test helpers already in `tests/support.lua`: `native(name, fn)` defines a recording double; `eq`, `fails(fn, fragment)`,
`expectCall(name, ...)` (checks the most recent call to `name` exactly), `callCount(name)`, `totalCalls()`,
`callName(index)`, `resetCalls()`; `checkGetters(wrapper, {{native, method, returnValue, args...}})`,
`checkSetters(wrapper, {{native, method, args...}})` (the native receives the handle then the args) and
`checkDisposed(wrapper, methods)`. `PLAYER_RAW` is the raw handle every `Player(i)` double returns, so
`Player.fromIndex(0)` wraps it; `Player.fromHandle({})` is "another player". Each `test` resets the call log.

## Review focus

1. Nothing validates late: bad receivers, arguments and options fail before any side-effecting native (every task).
2. No wrapper can go stale: `float`, `playOnce`, `flash` and `flashOn` return nothing and never enter a registry;
   owned text tags are permanent; wrapped sounds are never released when done (Tasks 2, 3, 6).
3. Local visibility changes only local visuals: every machine makes the same native call with a machine-local boolean;
   `playOnce` with `player` starts and releases the sound on every machine (Tasks 2, 3, 5).
4. Enumeration callbacks only collect handles; filters run afterwards on a fresh array (Task 7).
5. Import graph: every new module loads no other public module (Task 8).

## Files and interfaces

| File                                       | Responsibility                                                      |
| ------------------------------------------ | ------------------------------------------------------------------- |
| `src/wrappers/internal/options.lua`        | `Options.read(options, fields, operation)` (spec §8)                |
| `src/wrappers/internal/callback.lua`       | new `Callback.optional(value, operation)`                           |
| `src/wrappers/texttag.lua`                 | TextTag, `TextTag.float` (spec §5.1)                                |
| `src/wrappers/sound.lua`                   | Sound, `Sound.playOnce` (spec §5.2)                                 |
| `src/wrappers/lightning.lua`               | Lightning (spec §5.3)                                               |
| `src/wrappers/image.lua`                   | Image (spec §5.4)                                                   |
| `src/wrappers/ubersplat.lua`               | Ubersplat (spec §5.5)                                               |
| `src/wrappers/fogmodifier.lua`             | FogModifier (spec §5.6)                                             |
| `src/wrappers/effect.lua`                  | Widget attach, presentation setters, `flash`, `flashOn` (spec §6)   |
| `src/wrappers/{item,destructable}.lua`     | `enumInRect(rect, filter?)` (spec §7)                               |
| `src/wrappers/group.lua`                   | uses `Callback.optional` instead of its local `checkFilter`         |
| `tests/{options,texttag,sound,lightning,image,ubersplat,fogmodifier}.lua` | New suites                           |
| `tests/{effect,item,destructable,imports}.lua` | Extended suites                                                 |
| `tools/integration.ts`, `tests/editor-*.{lua,yue}`, `examples/gate.yue` | Bundle, editor and in-game gate coverage |
| `README.md`, `CHANGELOG.md`, `CONTRIBUTING.md`, `AGENTS.md` | Library docs                                       |
| Moonwell `AGENTS.md`                       | State, next work and backlog                                        |

Interfaces defined in this plan:

- `Options.read(options, fields, operation) -> table<string, any>` (Task 1). `fields` maps option names to
  `{kind, default?}`; kinds are `'boolean'`, `'string'`, `'number'`, `'integer'`, `'nonnegative'` (finite, ≥ 0),
  `'color'` (`{r, g, b, a?}` integers; returned as a fresh `{r, g, b, a}` with `a` defaulting to 255) and `'Player'`
  (a Player wrapper, returned as its **raw** handle). Returns a fresh table holding every declared field (the default
  when absent; nil when absent without default). Never modifies `options`. Errors (level 4, blaming the public
  function's caller): `[wrappers] <operation>: expected an options table`, `... unknown option '<key>'`,
  `... option '<name>' expected a number` (`a boolean`, `a string`, `an integer`, `a finite non-negative number`),
  `... option '<name>' expected {r, g, b, a?} integers`; Player errors come from `Handle.unwrap`
  (`... expected Player wrapper`, `... Player is disposed`).
- `Callback.optional(value, operation)` (Task 7): nil or a function passes; anything else raises
  `[wrappers] <operation>: expected a callback function` at level 3, exactly like the existing `Callback.check`.
- Every new class: `fromHandle(raw)`, `getHandle()`, `isDisposed()`, `.handle`, `destroy()` (idempotent).

---

### Task 1: Options validation

**Files:**
- Create: `src/wrappers/internal/options.lua`
- Test: `tests/options.lua`

**Interfaces:**
- Consumes: `Handle.unwrap(value, name, operation)` from `src/wrappers/internal/handle.lua`.
- Produces: `Options.read(options, fields, operation)` as specified above; the annotation alias
  `MoonwellWrappers.OptionFields`.

- [ ] **Step 1: Write the failing test**

Create `tests/options.lua`:

```lua
local Options = require('wrappers.internal.options')
local Player = require('wrappers.player')
eq(totalCalls(), 0)

local fields = {
    size = {'number', 10}, count = {'integer'}, flag = {'boolean', false}, name = {'string', 'x'},
    life = {'nonnegative', 2}, color = {'color', {255, 255, 255}}, player = {'Player'},
}

test('defaults fill a fresh table and given values pass through', function()
    local owner = Player.fromIndex(0)
    local given = {size = 12, count = 3.0, player = owner, color = {1, 2, 3}}
    local o = Options.read(given, fields, 'Test.op')
    eq(o.size, 12); eq(o.count, 3.0); eq(o.flag, false); eq(o.name, 'x'); eq(o.life, 2); eq(o.player, PLAYER_RAW)
    eq(o.color[1], 1); eq(o.color[2], 2); eq(o.color[3], 3); eq(o.color[4], 255)
    eq(given.player, owner); eq(given.color[4], nil); eq(given.flag, nil); eq(o == given, false)
    local defaults = Options.read(nil, fields, 'Test.op')
    eq(defaults.count, nil); eq(defaults.player, nil); eq(defaults.color[1], 255); eq(defaults.color[4], 255)
    eq(Options.read(nil, fields, 'Test.op').color == defaults.color, false)
    eq(fields.color[2][4], nil)
    local full = Options.read({color = {1, 2, 3, 4}}, fields, 'Test.op')
    eq(full.color[4], 4)
end)

test('invalid options fail with the operation name before any native', function()
    fails(function() Options.read('big', fields, 'Test.op') end, 'Test.op: expected an options table')
    fails(function() Options.read({colour = {1, 2, 3}}, fields, 'Test.op') end, "Test.op: unknown option 'colour'")
    fails(function() Options.read({size = 'big'}, fields, 'Test.op') end, "Test.op: option 'size' expected a number")
    fails(function() Options.read({count = 1.5}, fields, 'Test.op') end, "option 'count' expected an integer")
    fails(function() Options.read({flag = 1}, fields, 'Test.op') end, "option 'flag' expected a boolean")
    fails(function() Options.read({name = 1}, fields, 'Test.op') end, "option 'name' expected a string")
    fails(function() Options.read({life = -1}, fields, 'Test.op') end,
        "option 'life' expected a finite non-negative number")
    fails(function() Options.read({life = 0 / 0}, fields, 'Test.op') end, 'finite non-negative')
    fails(function() Options.read({life = math.huge}, fields, 'Test.op') end, 'finite non-negative')
    fails(function() Options.read({color = {1, 2}}, fields, 'Test.op') end,
        "option 'color' expected {r, g, b, a?} integers")
    fails(function() Options.read({color = {1, 2, 3, 4, 5}}, fields, 'Test.op') end, "option 'color'")
    fails(function() Options.read({color = {1, 2, 3.5}}, fields, 'Test.op') end, "option 'color'")
    fails(function() Options.read({color = 'red'}, fields, 'Test.op') end, "option 'color'")
    fails(function() Options.read({player = {}}, fields, 'Test.op') end, 'Test.op: expected Player wrapper')
    eq(totalCalls(), 0)
end)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `deno task test options`
Expected: FAIL with `module 'wrappers.internal.options' not found`.

- [ ] **Step 3: Write minimal implementation**

Create `src/wrappers/internal/options.lua`:

```lua
local Handle = require('wrappers.internal.handle')

---@alias MoonwellWrappers.OptionFields table<string, table> Option name to {kind, default?}.

local Options = {}

---@param value unknown
---@return boolean
local function isInteger(value) return type(value) == 'number' and value % 1 == 0 end

local checks = {
    boolean = function(value) return type(value) == 'boolean' end,
    string = function(value) return type(value) == 'string' end,
    number = function(value) return type(value) == 'number' end,
    integer = isInteger,
    nonnegative = function(value)
        return type(value) == 'number' and value == value and value >= 0 and value ~= math.huge
    end,
}
local expected = {
    boolean = 'a boolean', string = 'a string', number = 'a number', integer = 'an integer',
    nonnegative = 'a finite non-negative number',
}

---Level 4 blames the caller of the public function that called Options.read.
---@param operation string
---@param message string
local function fail(operation, message) error('[wrappers] ' .. operation .. ': ' .. message, 4) end

---@param value unknown
---@param name string
---@param operation string
---@return integer[]
local function color(value, name, operation)
    if type(value) ~= 'table' or (#value ~= 3 and #value ~= 4) then
        fail(operation, "option '" .. name .. "' expected {r, g, b, a?} integers")
    end
    for index = 1, #value do
        if not isInteger(value[index]) then fail(operation, "option '" .. name .. "' expected {r, g, b, a?} integers") end
    end
    return {value[1], value[2], value[3], value[4] or 255}
end

---Validates an options table and returns a fresh table with every declared field, defaults filled in. Never modifies
---`options`. Colors come back as fresh {r, g, b, a}; Player options come back as raw player handles.
---@param options unknown
---@param fields MoonwellWrappers.OptionFields
---@param operation string
---@return table<string, any>
function Options.read(options, fields, operation)
    if options ~= nil and type(options) ~= 'table' then fail(operation, 'expected an options table') end
    local given = options or {}
    for key in pairs(given) do
        if fields[key] == nil then fail(operation, "unknown option '" .. tostring(key) .. "'") end
    end
    local result = {}
    for name, field in pairs(fields) do
        local kind, value = field[1], given[name]
        if value == nil then value = field[2] end
        if value ~= nil then
            if kind == 'color' then
                value = color(value, name, operation)
            elseif kind == 'Player' then
                value = Handle.unwrap(value, 'Player', operation)
            elseif not checks[kind](value) then
                fail(operation, "option '" .. name .. "' expected " .. expected[kind])
            end
        end
        result[name] = value
    end
    return result
end

return Options
```

(The `pairs` loops call no natives and only choose which error to raise first, so their order cannot affect game
state.)

- [ ] **Step 4: Run test to verify it passes**

Run: `deno task test options`
Expected: `options: SUITE PASSED: 2 tests`.

- [ ] **Step 5: Run all checks and commit**

Run every command in "Running the wrappers checks". Expected: all pass.

```bash
git add src/wrappers/internal/options.lua tests/options.lua
git commit -m "feat: validate options tables for presentation wrappers"
```

---

### Task 2: TextTag

**Files:**
- Create: `src/wrappers/texttag.lua`
- Test: `tests/texttag.lua`

**Interfaces:**
- Consumes: `Options.read` (Task 1); `Handle.new`, `Handle.unwrap`, `Handle.created`.
- Produces: `MoonwellWrappers.TextTag` with `fromHandle`, `create`, `float`, `getHandle`, `isDisposed`, `setText`,
  `setColor`, `setPosition`, `setPositionOnUnit`, `setVelocity`, `setSuspended`, `show`, `setVisibleFor`, `destroy`;
  `MoonwellWrappers.TextTagFloatOptions`.

- [ ] **Step 1: Write the failing test**

Create `tests/texttag.lua`:

```lua
for _, name in ipairs({'DestroyTextTag', 'SetTextTagPermanent', 'SetTextTagText', 'SetTextTagPos',
    'SetTextTagPosUnit', 'SetTextTagColor', 'SetTextTagVelocity', 'SetTextTagVisibility', 'SetTextTagSuspended',
    'SetTextTagLifespan', 'SetTextTagFadepoint'}) do
    native(name, function() end)
end
native('CreateTextTag', function() return {} end)
native('GetLocalPlayer', function() return PLAYER_RAW end)
local TextTag = require('wrappers.texttag')
local Player = require('wrappers.player')
local Unit = require('wrappers.unit')
eq(totalCalls(), 0)

test('owned text tags are permanent and forward exact arguments', function()
    eq(TextTag.fromHandle(nil), nil)
    local tag = TextTag.create()
    expectCall('SetTextTagPermanent', tag.handle, true)
    eq(TextTag.fromHandle(tag.handle), tag); eq(tag:getHandle(), tag.handle); eq(tag:isDisposed(), false)
    checkSetters(tag, {{'SetTextTagColor', 'setColor', 1, 2, 3, 4}, {'SetTextTagPos', 'setPosition', 5, 6, 7},
        {'SetTextTagVelocity', 'setVelocity', 0.1, 0.2}, {'SetTextTagSuspended', 'setSuspended', true},
        {'SetTextTagVisibility', 'show', false}})
    tag:setText('hello', 10)
    expectCall('SetTextTagText', tag.handle, 'hello', 10 * 0.023 / 10)
    local u = Unit.fromHandle({})
    tag:setPositionOnUnit(u, 16); expectCall('SetTextTagPosUnit', tag.handle, u.handle, 16)
    fails(function() tag:setPositionOnUnit(tag, 0) end, 'TextTag.setPositionOnUnit: expected Unit wrapper')
    eq(callCount('SetTextTagPosUnit'), 1)
    u:remove(); tag:destroy()
end)

test('setVisibleFor shows the tag only on that player machine', function()
    local tag = TextTag.create()
    tag:setVisibleFor(Player.fromIndex(0)); expectCall('SetTextTagVisibility', tag.handle, true)
    tag:setVisibleFor(Player.fromHandle({})); expectCall('SetTextTagVisibility', tag.handle, false)
    fails(function() tag:setVisibleFor(tag) end, 'TextTag.setVisibleFor: expected Player wrapper')
    eq(callCount('SetTextTagVisibility'), 2)
    tag:destroy()
end)

test('destroy is idempotent and guards every method', function()
    local tag = TextTag.create()
    local raw = tag.handle
    tag:destroy(); tag:destroy()
    expectCall('DestroyTextTag', raw); eq(callCount('DestroyTextTag'), 1); eq(tag.handle, nil)
    eq(tag:isDisposed(), true)
    checkDisposed(tag, {'getHandle', 'setText', 'setColor', 'setPosition', 'setPositionOnUnit', 'setVelocity',
        'setSuspended', 'show', 'setVisibleFor'})
end)

test('create fails clearly when the native returns nil', function()
    native('CreateTextTag', function() return nil end)
    fails(function() TextTag.create() end, 'TextTag.create')
    eq(callCount('SetTextTagPermanent'), 0)
    native('CreateTextTag', function() return {} end)
end)

test('float shows a temporary tag with documented defaults and no wrapper', function()
    local raw = {}
    native('CreateTextTag', function() return raw end)
    eq(TextTag.float('+10', 1, 2), nil)
    local v = 64 * 0.071 / 128
    expectCall('SetTextTagText', raw, '+10', 10 * 0.023 / 10)
    expectCall('SetTextTagPos', raw, 1, 2, 0)
    expectCall('SetTextTagColor', raw, 255, 255, 255, 255)
    expectCall('SetTextTagVelocity', raw, v * math.cos(math.rad(90)), v * math.sin(math.rad(90)))
    expectCall('SetTextTagVisibility', raw, true)
    expectCall('SetTextTagPermanent', raw, false)
    expectCall('SetTextTagLifespan', raw, 2); expectCall('SetTextTagFadepoint', raw, 1)
    eq(callCount('GetLocalPlayer'), 0)
    native('CreateTextTag', function() return {} end)
end)

test('float options change size, color, motion, timing and audience', function()
    local raw = {}
    native('CreateTextTag', function() return raw end)
    TextTag.float('crit', 3, 4, {size = 14, heightOffset = 32, color = {255, 0, 0}, speed = 128, angle = 0,
        lifespan = 3, fadepoint = 2, player = Player.fromHandle({})})
    local v = 128 * 0.071 / 128
    expectCall('SetTextTagText', raw, 'crit', 14 * 0.023 / 10)
    expectCall('SetTextTagPos', raw, 3, 4, 32)
    expectCall('SetTextTagColor', raw, 255, 0, 0, 255)
    expectCall('SetTextTagVelocity', raw, v * math.cos(math.rad(0)), v * math.sin(math.rad(0)))
    expectCall('SetTextTagVisibility', raw, false)
    expectCall('SetTextTagLifespan', raw, 3); expectCall('SetTextTagFadepoint', raw, 2)
    TextTag.float('me', 0, 0, {player = Player.fromIndex(0), color = {0, 0, 255, 128}})
    expectCall('SetTextTagVisibility', raw, true); expectCall('SetTextTagColor', raw, 0, 0, 255, 128)
    native('CreateTextTag', function() return {} end)
end)

test('float does nothing without a free text tag and rejects bad options first', function()
    native('CreateTextTag', function() return nil end)
    eq(TextTag.float('x', 0, 0), nil)
    eq(totalCalls(), 1)
    native('CreateTextTag', function() return {} end)
    fails(function() TextTag.float('x', 0, 0, {colour = {1, 2, 3}}) end, "TextTag.float: unknown option 'colour'")
    fails(function() TextTag.float('x', 0, 0, {lifespan = -1}) end, "option 'lifespan'")
    fails(function() TextTag.float('x', 0, 0, {player = {}}) end, 'TextTag.float: expected Player wrapper')
    eq(callCount('CreateTextTag'), 1)
end)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `deno task test texttag`
Expected: FAIL with `module 'wrappers.texttag' not found`.

- [ ] **Step 3: Write minimal implementation**

Create `src/wrappers/texttag.lua`:

```lua
local Handle = require('wrappers.internal.handle')
local Options = require('wrappers.internal.options')

---@class MoonwellWrappers.TextTag
---@field handle texttag? Read-only by convention; nil after destruction.
local TextTag = {}
---@type MoonwellWrappers.Registry<MoonwellWrappers.TextTag, texttag>
local registry = Handle.new(TextTag, 'TextTag')

---@class MoonwellWrappers.TextTagFloatOptions
---@field size number? Font size, as in World Editor; default 10.
---@field heightOffset number? Height above the ground; default 0.
---@field color integer[]? {r, g, b, a?}, each 0-255; default white, opaque.
---@field speed number? World Editor speed units; default 64.
---@field angle number? Direction of travel in degrees; default 90.
---@field lifespan number? Seconds until the game destroys the tag; default 2.
---@field fadepoint number? Seconds after which the tag fades; default 1.
---@field player MoonwellWrappers.Player? Show the tag to this player only; default everyone.

---@type MoonwellWrappers.OptionFields
local floatFields = {
    size = {'number', 10}, heightOffset = {'number', 0}, color = {'color', {255, 255, 255}},
    speed = {'number', 64}, angle = {'number', 90}, lifespan = {'nonnegative', 2}, fadepoint = {'nonnegative', 1},
    player = {'Player'},
}

---The TextTagSize2Height conversion, computed in Lua.
---@param size number
---@return number
local function height(size) return size * 0.023 / 10 end

---@param raw texttag?
---@return MoonwellWrappers.TextTag?
---@overload fun(raw: nil): nil
function TextTag.fromHandle(raw) return registry.wrap(raw) end
---Creates a permanent text tag: the game never destroys it, so the wrapper stays valid until destroy().
---@return MoonwellWrappers.TextTag
function TextTag.create()
    local raw = CreateTextTag()
    local tag = Handle.created(TextTag.fromHandle(raw), 'TextTag.create')
    SetTextTagPermanent(raw, true)
    return tag
end
---Shows floating text that the game destroys after its lifespan. Returns nothing, so no wrapper can go stale. Does
---nothing when the game has no free text tag.
---@param text string
---@param x number
---@param y number
---@param options MoonwellWrappers.TextTagFloatOptions?
function TextTag.float(text, x, y, options)
    local o = Options.read(options, floatFields, 'TextTag.float')
    local raw = CreateTextTag()
    if raw == nil then return end
    SetTextTagText(raw, text, height(o.size))
    SetTextTagPos(raw, x, y, o.heightOffset)
    SetTextTagColor(raw, o.color[1], o.color[2], o.color[3], o.color[4])
    local velocity, radians = o.speed * 0.071 / 128, math.rad(o.angle)
    SetTextTagVelocity(raw, velocity * math.cos(radians), velocity * math.sin(radians))
    SetTextTagVisibility(raw, o.player == nil or o.player == GetLocalPlayer())
    SetTextTagPermanent(raw, false)
    SetTextTagLifespan(raw, o.lifespan)
    SetTextTagFadepoint(raw, o.fadepoint)
end
---@return texttag
function TextTag:getHandle() return registry.require(self, 'TextTag.getHandle') end
---@return boolean
function TextTag:isDisposed() return registry.isDisposed(self, 'TextTag.isDisposed') end
---@param text string
---@param size number Font size, as in World Editor.
function TextTag:setText(text, size) SetTextTagText(registry.require(self, 'TextTag.setText'), text, height(size)) end
---@param r integer 0-255
---@param g integer 0-255
---@param b integer 0-255
---@param a integer 0-255
function TextTag:setColor(r, g, b, a) SetTextTagColor(registry.require(self, 'TextTag.setColor'), r, g, b, a) end
---@param x number
---@param y number
---@param heightOffset number
function TextTag:setPosition(x, y, heightOffset)
    SetTextTagPos(registry.require(self, 'TextTag.setPosition'), x, y, heightOffset)
end
---Places the tag at the unit once; it does not follow the unit.
---@param unit MoonwellWrappers.Unit
---@param heightOffset number
function TextTag:setPositionOnUnit(unit, heightOffset)
    local raw = registry.require(self, 'TextTag.setPositionOnUnit')
    SetTextTagPosUnit(raw, Handle.unwrap(unit, 'Unit', 'TextTag.setPositionOnUnit'), heightOffset)
end
---@param xvel number Native units.
---@param yvel number Native units.
function TextTag:setVelocity(xvel, yvel) SetTextTagVelocity(registry.require(self, 'TextTag.setVelocity'), xvel, yvel) end
---@param flag boolean
function TextTag:setSuspended(flag) SetTextTagSuspended(registry.require(self, 'TextTag.setSuspended'), flag) end
---@param flag boolean
function TextTag:show(flag) SetTextTagVisibility(registry.require(self, 'TextTag.show'), flag) end
---Shows the tag on that player's machine only. Only local visuals differ.
---@param player MoonwellWrappers.Player
function TextTag:setVisibleFor(player)
    local raw = registry.require(self, 'TextTag.setVisibleFor')
    SetTextTagVisibility(raw, Handle.unwrap(player, 'Player', 'TextTag.setVisibleFor') == GetLocalPlayer())
end
function TextTag:destroy()
    local raw = registry.dispose(self, 'TextTag.destroy')
    if raw then DestroyTextTag(raw) end
end

return TextTag
```

- [ ] **Step 4: Run test to verify it passes**

Run: `deno task test texttag`
Expected: `texttag: SUITE PASSED: 7 tests`.

- [ ] **Step 5: Run all checks and commit**

Run every command in "Running the wrappers checks". Expected: all pass.

```bash
git add src/wrappers/texttag.lua tests/texttag.lua
git commit -m "feat: TextTag wrapper with permanent owned tags and TextTag.float"
```

---

### Task 3: Sound

**Files:**
- Create: `src/wrappers/sound.lua`
- Test: `tests/sound.lua`

**Interfaces:**
- Consumes: `Options.read` (Task 1); `Handle.new`, `Handle.unwrap`, `Handle.created`.
- Produces: `MoonwellWrappers.Sound` with `fromHandle`, `create`, `playOnce`, `getHandle`, `isDisposed`, `play`,
  `playFor`, `stop`, `setVolume`, `setPitch`, `setChannel`, `setPosition`, `attachToUnit`, `setDistances`,
  `setDistanceCutoff`, `getDuration`, `destroy`; `MoonwellWrappers.SoundOptions`, `MoonwellWrappers.SoundPlayOnceOptions`.

- [ ] **Step 1: Write the failing test**

Create `tests/sound.lua`:

```lua
for _, name in ipairs({'StartSound', 'StopSound', 'KillSoundWhenDone', 'SetSoundVolume', 'SetSoundPitch',
    'SetSoundChannel', 'SetSoundPosition', 'AttachSoundToUnit', 'SetSoundDistances', 'SetSoundDistanceCutoff'}) do
    native(name, function() end)
end
native('CreateSound', function() return {} end)
native('GetLocalPlayer', function() return PLAYER_RAW end)
local Sound = require('wrappers.sound')
local Player = require('wrappers.player')
local Unit = require('wrappers.unit')
eq(totalCalls(), 0)

test('create uses documented defaults and options', function()
    eq(Sound.fromHandle(nil), nil)
    local s = Sound.create('a.flac')
    expectCall('CreateSound', 'a.flac', false, false, false, 10, 10, 'DefaultEAXON')
    eq(Sound.fromHandle(s.handle), s); eq(s:getHandle(), s.handle); eq(s:isDisposed(), false)
    local looped = Sound.create('b.flac', {looping = true, is3D = true, stopWhenOutOfRange = true, fadeIn = 1,
        fadeOut = 2, eax = 'SpellsEAX'})
    expectCall('CreateSound', 'b.flac', true, true, true, 1, 2, 'SpellsEAX')
    fails(function() Sound.create('c.flac', {loop = true}) end, "Sound.create: unknown option 'loop'")
    fails(function() Sound.create('c.flac', {fadeIn = 0.5}) end, "option 'fadeIn' expected an integer")
    eq(callCount('CreateSound'), 2)
    native('CreateSound', function() return nil end)
    fails(function() Sound.create('d.flac') end, 'Sound.create')
    native('CreateSound', function() return {} end)
    s:destroy(); looped:destroy()
end)

test('playback and settings forward exact arguments', function()
    local s = Sound.create('a.flac')
    checkSetters(s, {{'StartSound', 'play'}, {'SetSoundVolume', 'setVolume', 100}, {'SetSoundPitch', 'setPitch', 1.5},
        {'SetSoundChannel', 'setChannel', 5}, {'SetSoundPosition', 'setPosition', 1, 2, 3},
        {'SetSoundDistances', 'setDistances', 600, 4000}, {'SetSoundDistanceCutoff', 'setDistanceCutoff', 3000}})
    checkGetters(s, {{'GetSoundDuration', 'getDuration', 1500}})
    s:stop(); expectCall('StopSound', s.handle, false, false)
    s:stop(true); expectCall('StopSound', s.handle, false, true)
    local u = Unit.fromHandle({})
    s:attachToUnit(u); expectCall('AttachSoundToUnit', s.handle, u.handle)
    fails(function() s:attachToUnit(s) end, 'Sound.attachToUnit: expected Unit wrapper')
    eq(callCount('AttachSoundToUnit'), 1)
    u:remove(); s:destroy()
end)

test('playFor starts the sound only on that player machine', function()
    local s = Sound.create('a.flac')
    s:playFor(Player.fromIndex(0)); eq(callCount('StartSound'), 1); expectCall('StartSound', s.handle)
    s:playFor(Player.fromHandle({})); eq(callCount('StartSound'), 1)
    fails(function() s:playFor(s) end, 'Sound.playFor: expected Player wrapper')
    s:destroy()
end)

test('destroy stops and kills once, never when done, and guards every method', function()
    local s = Sound.create('a.flac')
    local raw = s.handle
    s:destroy(); s:destroy()
    expectCall('StopSound', raw, true, false); eq(callCount('StopSound'), 1); eq(s.handle, nil)
    eq(s:isDisposed(), true); eq(callCount('KillSoundWhenDone'), 0)
    checkDisposed(s, {'getHandle', 'play', 'playFor', 'stop', 'setVolume', 'setPitch', 'setChannel', 'setPosition',
        'attachToUnit', 'setDistances', 'setDistanceCutoff', 'getDuration'})
end)

test('playOnce starts and releases a sound without a wrapper', function()
    local raw = {}
    native('CreateSound', function() return raw end)
    eq(Sound.playOnce('hit.flac'), nil)
    expectCall('CreateSound', 'hit.flac', false, false, false, 10, 10, 'DefaultEAXON')
    expectCall('SetSoundVolume', raw, 127); expectCall('StartSound', raw); expectCall('KillSoundWhenDone', raw)
    eq(callCount('SetSoundPosition'), 0); eq(callCount('GetLocalPlayer'), 0)
    eq(callName(totalCalls()), 'KillSoundWhenDone')
    Sound.playOnce('hit.flac', {volume = 90, x = 1, y = 2})
    expectCall('CreateSound', 'hit.flac', false, true, false, 10, 10, 'DefaultEAXON')
    expectCall('SetSoundVolume', raw, 90); expectCall('SetSoundPosition', raw, 1, 2, 0)
    Sound.playOnce('hit.flac', {x = 1, y = 2, z = 3}); expectCall('SetSoundPosition', raw, 1, 2, 3)
    native('CreateSound', function() return {} end)
end)

test('playOnce for one player mutes the other machines but starts and releases everywhere', function()
    local raw = {}
    native('CreateSound', function() return raw end)
    Sound.playOnce('hit.flac', {player = Player.fromHandle({})})
    expectCall('SetSoundVolume', raw, 0); expectCall('StartSound', raw); expectCall('KillSoundWhenDone', raw)
    Sound.playOnce('hit.flac', {player = Player.fromIndex(0), volume = 50})
    expectCall('SetSoundVolume', raw, 50)
    eq(callCount('StartSound'), 2); eq(callCount('KillSoundWhenDone'), 2)
    native('CreateSound', function() return {} end)
end)

test('playOnce validates before creating a sound and fails on a nil sound', function()
    fails(function() Sound.playOnce('a', {x = 1}) end, 'Sound.playOnce: options x and y must be given together')
    fails(function() Sound.playOnce('a', {y = 1}) end, 'Sound.playOnce: options x and y must be given together')
    fails(function() Sound.playOnce('a', {volume = 'loud'}) end, "option 'volume'")
    eq(callCount('CreateSound'), 0)
    native('CreateSound', function() return nil end)
    fails(function() Sound.playOnce('missing.flac') end, 'Sound.playOnce')
    eq(callCount('StartSound'), 0)
    native('CreateSound', function() return {} end)
end)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `deno task test sound`
Expected: FAIL with `module 'wrappers.sound' not found`.

- [ ] **Step 3: Write minimal implementation**

Create `src/wrappers/sound.lua`:

```lua
local Handle = require('wrappers.internal.handle')
local Options = require('wrappers.internal.options')

---@class MoonwellWrappers.Sound
---@field handle sound? Read-only by convention; nil after destruction.
local Sound = {}
---@type MoonwellWrappers.Registry<MoonwellWrappers.Sound, sound>
local registry = Handle.new(Sound, 'Sound')

---@class MoonwellWrappers.SoundOptions
---@field looping boolean? Default false.
---@field is3D boolean? Default false.
---@field stopWhenOutOfRange boolean? Default false.
---@field fadeIn integer? Default 10.
---@field fadeOut integer? Default 10.
---@field eax string? Default "DefaultEAXON".

---@class MoonwellWrappers.SoundPlayOnceOptions
---@field volume integer? 0-127; default 127.
---@field x number? With y, makes the sound 3D at that point.
---@field y number? With x, makes the sound 3D at that point.
---@field z number? Default 0.
---@field player MoonwellWrappers.Player? Hear the sound on this player's machine only; default everyone.

---@type MoonwellWrappers.OptionFields
local createFields = {
    looping = {'boolean', false}, is3D = {'boolean', false}, stopWhenOutOfRange = {'boolean', false},
    fadeIn = {'integer', 10}, fadeOut = {'integer', 10}, eax = {'string', 'DefaultEAXON'},
}
---@type MoonwellWrappers.OptionFields
local playOnceFields = {
    volume = {'integer', 127}, x = {'number'}, y = {'number'}, z = {'number', 0}, player = {'Player'},
}

---@param raw sound?
---@return MoonwellWrappers.Sound?
---@overload fun(raw: nil): nil
function Sound.fromHandle(raw) return registry.wrap(raw) end
---Creates an owned sound. It is never released when done: destroy() is its only cleanup.
---@param path string
---@param options MoonwellWrappers.SoundOptions?
---@return MoonwellWrappers.Sound
function Sound.create(path, options)
    local o = Options.read(options, createFields, 'Sound.create')
    local raw = CreateSound(path, o.looping, o.is3D, o.stopWhenOutOfRange, o.fadeIn, o.fadeOut, o.eax)
    return Handle.created(Sound.fromHandle(raw), 'Sound.create')
end
---Plays a sound once and releases it when done. Returns nothing, so no wrapper can go stale. With `player`, the other
---machines play it at volume 0, so every machine starts and releases the sound identically.
---@param path string
---@param options MoonwellWrappers.SoundPlayOnceOptions?
function Sound.playOnce(path, options)
    local o = Options.read(options, playOnceFields, 'Sound.playOnce')
    if (o.x == nil) ~= (o.y == nil) then error('[wrappers] Sound.playOnce: options x and y must be given together', 2) end
    local is3D = o.x ~= nil
    local raw = Handle.created(CreateSound(path, false, is3D, false, 10, 10, 'DefaultEAXON'), 'Sound.playOnce')
    SetSoundVolume(raw, (o.player == nil or o.player == GetLocalPlayer()) and o.volume or 0)
    if is3D then SetSoundPosition(raw, o.x, o.y, o.z) end
    StartSound(raw)
    KillSoundWhenDone(raw)
end
---@return sound
function Sound:getHandle() return registry.require(self, 'Sound.getHandle') end
---@return boolean
function Sound:isDisposed() return registry.isDisposed(self, 'Sound.isDisposed') end
function Sound:play() StartSound(registry.require(self, 'Sound.play')) end
---Starts the sound on that player's machine only. The sound itself exists on every machine.
---@param player MoonwellWrappers.Player
function Sound:playFor(player)
    local raw = registry.require(self, 'Sound.playFor')
    if Handle.unwrap(player, 'Player', 'Sound.playFor') == GetLocalPlayer() then StartSound(raw) end
end
---@param fadeOut boolean?
function Sound:stop(fadeOut) StopSound(registry.require(self, 'Sound.stop'), false, fadeOut or false) end
---@param volume integer 0-127
function Sound:setVolume(volume) SetSoundVolume(registry.require(self, 'Sound.setVolume'), volume) end
---@param pitch number
function Sound:setPitch(pitch) SetSoundPitch(registry.require(self, 'Sound.setPitch'), pitch) end
---@param channel integer
function Sound:setChannel(channel) SetSoundChannel(registry.require(self, 'Sound.setChannel'), channel) end
---@param x number
---@param y number
---@param z number
function Sound:setPosition(x, y, z) SetSoundPosition(registry.require(self, 'Sound.setPosition'), x, y, z) end
---@param unit MoonwellWrappers.Unit
function Sound:attachToUnit(unit)
    local raw = registry.require(self, 'Sound.attachToUnit')
    AttachSoundToUnit(raw, Handle.unwrap(unit, 'Unit', 'Sound.attachToUnit'))
end
---@param min number
---@param max number
function Sound:setDistances(min, max) SetSoundDistances(registry.require(self, 'Sound.setDistances'), min, max) end
---@param cutoff number
function Sound:setDistanceCutoff(cutoff)
    SetSoundDistanceCutoff(registry.require(self, 'Sound.setDistanceCutoff'), cutoff)
end
---@return integer milliseconds
function Sound:getDuration() return GetSoundDuration(registry.require(self, 'Sound.getDuration')) end
function Sound:destroy()
    local raw = registry.dispose(self, 'Sound.destroy')
    if raw then StopSound(raw, true, false) end
end

return Sound
```

- [ ] **Step 4: Run test to verify it passes**

Run: `deno task test sound`
Expected: `sound: SUITE PASSED: 7 tests`.

- [ ] **Step 5: Run all checks and commit**

Run every command in "Running the wrappers checks". Expected: all pass.

```bash
git add src/wrappers/sound.lua tests/sound.lua
git commit -m "feat: Sound wrapper with playFor and Sound.playOnce"
```

---

### Task 4: Lightning and FogModifier

**Files:**
- Create: `src/wrappers/lightning.lua`, `src/wrappers/fogmodifier.lua`
- Test: `tests/lightning.lua`, `tests/fogmodifier.lua`

**Interfaces:**
- Consumes: `Handle.new`, `Handle.unwrap`, `Handle.created`.
- Produces: `MoonwellWrappers.Lightning` (`fromHandle`, `create`, `getHandle`, `isDisposed`, `move`, `setColor`,
  `destroy`); `MoonwellWrappers.FogModifier` (`fromHandle`, `radius`, `rect`, `getHandle`, `isDisposed`, `start`,
  `stop`, `destroy`).

- [ ] **Step 1: Write the failing tests**

Create `tests/lightning.lua`:

```lua
native('AddLightningEx', function() return {} end)
native('DestroyLightning', function() return true end)
native('MoveLightningEx', function() return true end)
native('SetLightningColor', function() return false end)
local Lightning = require('wrappers.lightning')
eq(totalCalls(), 0)

test('lightning creation, movement and color forward exact arguments', function()
    eq(Lightning.fromHandle(nil), nil)
    local bolt = Lightning.create('CLPB', 1, 2, 3, 4, 5, 6)
    expectCall('AddLightningEx', 'CLPB', false, 1, 2, 3, 4, 5, 6)
    eq(Lightning.fromHandle(bolt.handle), bolt); eq(bolt:getHandle(), bolt.handle); eq(bolt:isDisposed(), false)
    local checked = Lightning.create('DRAL', 1, 2, 3, 4, 5, 6, true)
    expectCall('AddLightningEx', 'DRAL', true, 1, 2, 3, 4, 5, 6)
    eq(bolt:move(7, 8, 9, 10, 11, 12), true); expectCall('MoveLightningEx', bolt.handle, false, 7, 8, 9, 10, 11, 12)
    bolt:move(0, 0, 0, 1, 1, 1, true); expectCall('MoveLightningEx', bolt.handle, true, 0, 0, 0, 1, 1, 1)
    eq(bolt:setColor(1, 0.5, 0, 1), false); expectCall('SetLightningColor', bolt.handle, 1, 0.5, 0, 1)
    bolt:destroy(); checked:destroy()
end)

test('lightning destruction is idempotent and guards every method', function()
    local bolt = Lightning.create('CLPB', 0, 0, 0, 1, 1, 1)
    local raw = bolt.handle
    bolt:destroy(); bolt:destroy()
    expectCall('DestroyLightning', raw); eq(callCount('DestroyLightning'), 1); eq(bolt.handle, nil)
    eq(bolt:isDisposed(), true)
    checkDisposed(bolt, {'getHandle', 'move', 'setColor'})
    native('AddLightningEx', function() return nil end)
    fails(function() Lightning.create('CLPB', 0, 0, 0, 1, 1, 1) end, 'Lightning.create')
    native('AddLightningEx', function() return {} end)
end)
```

Create `tests/fogmodifier.lua`:

```lua
native('CreateFogModifierRadius', function() return {} end)
native('CreateFogModifierRect', function() return {} end)
native('FogModifierStart', function() end)
native('FogModifierStop', function() end)
native('DestroyFogModifier', function() end)
local FogModifier = require('wrappers.fogmodifier')
local Player = require('wrappers.player')
local Rect = require('wrappers.rect')
eq(totalCalls(), 0)
local visible = {}

test('fog modifiers from a radius or a rect forward exact arguments', function()
    eq(FogModifier.fromHandle(nil), nil)
    local owner = Player.fromIndex(0)
    local near = FogModifier.radius(owner, visible, 1, 2, 300, true, false)
    expectCall('CreateFogModifierRadius', PLAYER_RAW, visible, 1, 2, 300, true, false)
    eq(FogModifier.fromHandle(near.handle), near); eq(near:getHandle(), near.handle); eq(near:isDisposed(), false)
    local area = Rect.fromHandle({})
    local far = FogModifier.rect(owner, visible, area, false, true)
    expectCall('CreateFogModifierRect', PLAYER_RAW, visible, area.handle, false, true)
    checkSetters(near, {{'FogModifierStart', 'start'}, {'FogModifierStop', 'stop'}})
    near:destroy(); far:destroy()
    eq(area:isDisposed(), false)
end)

test('fog modifier arguments are validated before the native', function()
    local area = Rect.fromHandle({})
    fails(function() FogModifier.radius(area, visible, 0, 0, 1, true, true) end,
        'FogModifier.radius: expected Player wrapper')
    fails(function() FogModifier.rect(Player.fromIndex(0), visible, {}, true, true) end,
        'FogModifier.rect: expected Rect wrapper')
    eq(callCount('CreateFogModifierRadius'), 0); eq(callCount('CreateFogModifierRect'), 0)
    native('CreateFogModifierRadius', function() return nil end)
    fails(function() FogModifier.radius(Player.fromIndex(0), visible, 0, 0, 1, true, true) end, 'FogModifier.radius')
    native('CreateFogModifierRadius', function() return {} end)
    native('CreateFogModifierRect', function() return nil end)
    fails(function() FogModifier.rect(Player.fromIndex(0), visible, area, true, true) end, 'FogModifier.rect')
    native('CreateFogModifierRect', function() return {} end)
end)

test('fog modifier destruction is idempotent and guards every method', function()
    local fog = FogModifier.radius(Player.fromIndex(0), visible, 0, 0, 1, true, true)
    local raw = fog.handle
    fog:destroy(); fog:destroy()
    expectCall('DestroyFogModifier', raw); eq(callCount('DestroyFogModifier'), 1); eq(fog.handle, nil)
    eq(fog:isDisposed(), true)
    checkDisposed(fog, {'getHandle', 'start', 'stop'})
end)
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `deno task test lightning fogmodifier`
Expected: both FAIL with `module 'wrappers.lightning' not found` / `module 'wrappers.fogmodifier' not found`.

- [ ] **Step 3: Write minimal implementation**

Create `src/wrappers/lightning.lua`:

```lua
local Handle = require('wrappers.internal.handle')

---@class MoonwellWrappers.Lightning
---@field handle lightning? Read-only by convention; nil after destruction.
local Lightning = {}
---@type MoonwellWrappers.Registry<MoonwellWrappers.Lightning, lightning>
local registry = Handle.new(Lightning, 'Lightning')

---@param raw lightning?
---@return MoonwellWrappers.Lightning?
---@overload fun(raw: nil): nil
function Lightning.fromHandle(raw) return registry.wrap(raw) end
---@param code string Lightning type, such as "CLPB".
---@param x1 number
---@param y1 number
---@param z1 number
---@param x2 number
---@param y2 number
---@param z2 number
---@param checkVisibility boolean? Default false.
---@return MoonwellWrappers.Lightning
function Lightning.create(code, x1, y1, z1, x2, y2, z2, checkVisibility)
    local raw = AddLightningEx(code, checkVisibility or false, x1, y1, z1, x2, y2, z2)
    return Handle.created(Lightning.fromHandle(raw), 'Lightning.create')
end
---@return lightning
function Lightning:getHandle() return registry.require(self, 'Lightning.getHandle') end
---@return boolean
function Lightning:isDisposed() return registry.isDisposed(self, 'Lightning.isDisposed') end
---@param x1 number
---@param y1 number
---@param z1 number
---@param x2 number
---@param y2 number
---@param z2 number
---@param checkVisibility boolean? Default false.
---@return boolean
function Lightning:move(x1, y1, z1, x2, y2, z2, checkVisibility)
    local raw = registry.require(self, 'Lightning.move')
    return MoveLightningEx(raw, checkVisibility or false, x1, y1, z1, x2, y2, z2)
end
---@param r number 0-1
---@param g number 0-1
---@param b number 0-1
---@param a number 0-1
---@return boolean
function Lightning:setColor(r, g, b, a) return SetLightningColor(registry.require(self, 'Lightning.setColor'), r, g, b, a) end
function Lightning:destroy()
    local raw = registry.dispose(self, 'Lightning.destroy')
    if raw then DestroyLightning(raw) end
end

return Lightning
```

Create `src/wrappers/fogmodifier.lua`:

```lua
local Handle = require('wrappers.internal.handle')

---@class MoonwellWrappers.FogModifier
---@field handle fogmodifier? Read-only by convention; nil after destruction.
local FogModifier = {}
---@type MoonwellWrappers.Registry<MoonwellWrappers.FogModifier, fogmodifier>
local registry = Handle.new(FogModifier, 'FogModifier')

---@param raw fogmodifier?
---@return MoonwellWrappers.FogModifier?
---@overload fun(raw: nil): nil
function FogModifier.fromHandle(raw) return registry.wrap(raw) end
---Creates a stopped modifier; call start().
---@param player MoonwellWrappers.Player
---@param state fogstate
---@param x number
---@param y number
---@param radius number
---@param useSharedVision boolean
---@param afterUnits boolean
---@return MoonwellWrappers.FogModifier
function FogModifier.radius(player, state, x, y, radius, useSharedVision, afterUnits)
    local rawPlayer = Handle.unwrap(player, 'Player', 'FogModifier.radius')
    local raw = CreateFogModifierRadius(rawPlayer, state, x, y, radius, useSharedVision, afterUnits)
    return Handle.created(FogModifier.fromHandle(raw), 'FogModifier.radius')
end
---Creates a stopped modifier; call start(). The modifier does not own the rect.
---@param player MoonwellWrappers.Player
---@param state fogstate
---@param rect MoonwellWrappers.Rect
---@param useSharedVision boolean
---@param afterUnits boolean
---@return MoonwellWrappers.FogModifier
function FogModifier.rect(player, state, rect, useSharedVision, afterUnits)
    local rawPlayer = Handle.unwrap(player, 'Player', 'FogModifier.rect')
    local rawRect = Handle.unwrap(rect, 'Rect', 'FogModifier.rect')
    local raw = CreateFogModifierRect(rawPlayer, state, rawRect, useSharedVision, afterUnits)
    return Handle.created(FogModifier.fromHandle(raw), 'FogModifier.rect')
end
---@return fogmodifier
function FogModifier:getHandle() return registry.require(self, 'FogModifier.getHandle') end
---@return boolean
function FogModifier:isDisposed() return registry.isDisposed(self, 'FogModifier.isDisposed') end
function FogModifier:start() FogModifierStart(registry.require(self, 'FogModifier.start')) end
function FogModifier:stop() FogModifierStop(registry.require(self, 'FogModifier.stop')) end
function FogModifier:destroy()
    local raw = registry.dispose(self, 'FogModifier.destroy')
    if raw then DestroyFogModifier(raw) end
end

return FogModifier
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `deno task test lightning fogmodifier`
Expected: `lightning: SUITE PASSED: 2 tests` and `fogmodifier: SUITE PASSED: 3 tests`.

- [ ] **Step 5: Run all checks and commit**

Run every command in "Running the wrappers checks". Expected: all pass.

```bash
git add src/wrappers/lightning.lua src/wrappers/fogmodifier.lua tests/lightning.lua tests/fogmodifier.lua
git commit -m "feat: Lightning and FogModifier wrappers"
```

---

### Task 5: Image and Ubersplat

**Files:**
- Create: `src/wrappers/image.lua`, `src/wrappers/ubersplat.lua`
- Test: `tests/image.lua`, `tests/ubersplat.lua`

**Interfaces:**
- Consumes: `Options.read` (Task 1); `Handle.new`, `Handle.unwrap`, `Handle.created`.
- Produces: `MoonwellWrappers.Image` (`fromHandle`, `create`, `getHandle`, `isDisposed`, `setPosition`, `show`,
  `setVisibleFor`, `setColor`, `setConstantHeight`, `setAboveWater`, `setType`, `destroy`);
  `MoonwellWrappers.Ubersplat` (`fromHandle`, `create`, `getHandle`, `isDisposed`, `show`, `setVisibleFor`, `finish`,
  `reset`, `destroy`); `MoonwellWrappers.UbersplatOptions`.

- [ ] **Step 1: Write the failing tests**

Create `tests/image.lua`:

```lua
for _, name in ipairs({'SetImageRenderAlways', 'ShowImage', 'SetImagePosition', 'SetImageColor',
    'SetImageConstantHeight', 'SetImageAboveWater', 'SetImageType', 'DestroyImage'}) do
    native(name, function() end)
end
native('CreateImage', function() return {} end)
native('GetLocalPlayer', function() return PLAYER_RAW end)
local Image = require('wrappers.image')
local Player = require('wrappers.player')
eq(totalCalls(), 0)

test('images are centered on creation, drawn and shown', function()
    eq(Image.fromHandle(nil), nil)
    local image = Image.create('aoe.blp', 256, 128, 100, 50, 1)
    expectCall('CreateImage', 'aoe.blp', 256, 128, 0, -28, -14, 0, 0, 0, 0, 1)
    expectCall('SetImageRenderAlways', image.handle, true); expectCall('ShowImage', image.handle, true)
    eq(Image.fromHandle(image.handle), image); eq(image:getHandle(), image.handle); eq(image:isDisposed(), false)
    image:setPosition(0, 0); expectCall('SetImagePosition', image.handle, -128, -64, 0)
    image:setPosition(10, 20, 5); expectCall('SetImagePosition', image.handle, -118, -44, 5)
    image:destroy()
end)

test('a wrapped image of unknown size cannot be centered', function()
    local foreign = Image.fromHandle({})
    fails(function() foreign:setPosition(0, 0) end,
        'Image.setPosition: size unknown for a wrapped image; use SetImagePosition')
    eq(callCount('SetImagePosition'), 0)
    foreign:destroy()
end)

test('image settings and local visibility forward exact arguments', function()
    local image = Image.create('aoe.blp', 64, 64, 0, 0, 2)
    checkSetters(image, {{'ShowImage', 'show', false}, {'SetImageColor', 'setColor', 1, 2, 3, 4},
        {'SetImageConstantHeight', 'setConstantHeight', true, 10}, {'SetImageAboveWater', 'setAboveWater', true, false},
        {'SetImageType', 'setType', 3}})
    image:setVisibleFor(Player.fromIndex(0)); expectCall('ShowImage', image.handle, true)
    image:setVisibleFor(Player.fromHandle({})); expectCall('ShowImage', image.handle, false)
    fails(function() image:setVisibleFor(image) end, 'Image.setVisibleFor: expected Player wrapper')
    image:destroy()
end)

test('image destruction is idempotent and guards every method', function()
    local image = Image.create('aoe.blp', 64, 64, 0, 0, 1)
    local raw = image.handle
    image:destroy(); image:destroy()
    expectCall('DestroyImage', raw); eq(callCount('DestroyImage'), 1); eq(image.handle, nil)
    eq(image:isDisposed(), true)
    checkDisposed(image, {'getHandle', 'setPosition', 'show', 'setVisibleFor', 'setColor', 'setConstantHeight',
        'setAboveWater', 'setType'})
    native('CreateImage', function() return nil end)
    fails(function() Image.create('aoe.blp', 64, 64, 0, 0, 1) end, 'Image.create')
    eq(callCount('SetImageRenderAlways'), 1)
    native('CreateImage', function() return {} end)
end)
```

Create `tests/ubersplat.lua`:

```lua
for _, name in ipairs({'SetUbersplatRenderAlways', 'ShowUbersplat', 'FinishUbersplat', 'ResetUbersplat',
    'DestroyUbersplat'}) do
    native(name, function() end)
end
native('CreateUbersplat', function() return {} end)
native('GetLocalPlayer', function() return PLAYER_RAW end)
local Ubersplat = require('wrappers.ubersplat')
local Player = require('wrappers.player')
eq(totalCalls(), 0)

test('ubersplats use documented defaults and options and are always rendered', function()
    eq(Ubersplat.fromHandle(nil), nil)
    local splat = Ubersplat.create('HMED', 1, 2)
    expectCall('CreateUbersplat', 1, 2, 'HMED', 255, 255, 255, 255, false, false)
    expectCall('SetUbersplatRenderAlways', splat.handle, true)
    eq(Ubersplat.fromHandle(splat.handle), splat); eq(splat:getHandle(), splat.handle); eq(splat:isDisposed(), false)
    local tinted = Ubersplat.create('OLAR', 3, 4, {color = {10, 20, 30}, forcePaused = true, noBirthTime = true})
    expectCall('CreateUbersplat', 3, 4, 'OLAR', 10, 20, 30, 255, true, true)
    fails(function() Ubersplat.create('HMED', 0, 0, {colour = {1, 2, 3}}) end,
        "Ubersplat.create: unknown option 'colour'")
    eq(callCount('CreateUbersplat'), 2)
    splat:destroy(); tinted:destroy()
end)

test('ubersplat controls and local visibility forward exact arguments', function()
    local splat = Ubersplat.create('HMED', 0, 0)
    checkSetters(splat, {{'ShowUbersplat', 'show', false}, {'FinishUbersplat', 'finish'},
        {'ResetUbersplat', 'reset'}})
    splat:setVisibleFor(Player.fromIndex(0)); expectCall('ShowUbersplat', splat.handle, true)
    splat:setVisibleFor(Player.fromHandle({})); expectCall('ShowUbersplat', splat.handle, false)
    fails(function() splat:setVisibleFor(splat) end, 'Ubersplat.setVisibleFor: expected Player wrapper')
    splat:destroy()
end)

test('ubersplat destruction is idempotent and guards every method', function()
    local splat = Ubersplat.create('HMED', 0, 0)
    local raw = splat.handle
    splat:destroy(); splat:destroy()
    expectCall('DestroyUbersplat', raw); eq(callCount('DestroyUbersplat'), 1); eq(splat.handle, nil)
    eq(splat:isDisposed(), true)
    checkDisposed(splat, {'getHandle', 'show', 'setVisibleFor', 'finish', 'reset'})
    native('CreateUbersplat', function() return nil end)
    fails(function() Ubersplat.create('HMED', 0, 0) end, 'Ubersplat.create')
    eq(callCount('SetUbersplatRenderAlways'), 1)
    native('CreateUbersplat', function() return {} end)
end)
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `deno task test image ubersplat`
Expected: both FAIL with `module 'wrappers.image' not found` / `module 'wrappers.ubersplat' not found`.

- [ ] **Step 3: Write minimal implementation**

Create `src/wrappers/image.lua`:

```lua
local Handle = require('wrappers.internal.handle')

---@class MoonwellWrappers.Image
---@field handle image? Read-only by convention; nil after destruction.
local Image = {}
---@type MoonwellWrappers.Registry<MoonwellWrappers.Image, image>
local registry = Handle.new(Image, 'Image')
---Sizes of images made by Image.create, for centering. Only indexed, never iterated.
---@type table<MoonwellWrappers.Image, number[]>
local sizes = setmetatable({}, {__mode = 'k'})

---@param raw image?
---@return MoonwellWrappers.Image?
---@overload fun(raw: nil): nil
function Image.fromHandle(raw) return registry.wrap(raw) end
---Creates a visible image centered on x, y.
---@param path string
---@param width number
---@param height number
---@param x number
---@param y number
---@param imageType integer 1 selection, 2 indicator, 3 occlusion mask, 4 ubersplat.
---@return MoonwellWrappers.Image
function Image.create(path, width, height, x, y, imageType)
    local raw = CreateImage(path, width, height, 0, x - width / 2, y - height / 2, 0, 0, 0, 0, imageType)
    local image = Handle.created(Image.fromHandle(raw), 'Image.create')
    sizes[image] = {width, height}
    SetImageRenderAlways(raw, true)
    ShowImage(raw, true)
    return image
end
---@return image
function Image:getHandle() return registry.require(self, 'Image.getHandle') end
---@return boolean
function Image:isDisposed() return registry.isDisposed(self, 'Image.isDisposed') end
---Centers the image on x, y. Fails for an image wrapped with fromHandle, whose size is unknown.
---@param x number
---@param y number
---@param z number? Default 0.
function Image:setPosition(x, y, z)
    local raw = registry.require(self, 'Image.setPosition')
    local size = sizes[self]
    if size == nil then
        error('[wrappers] Image.setPosition: size unknown for a wrapped image; use SetImagePosition', 2)
    end
    SetImagePosition(raw, x - size[1] / 2, y - size[2] / 2, z or 0)
end
---@param flag boolean
function Image:show(flag) ShowImage(registry.require(self, 'Image.show'), flag) end
---Shows the image on that player's machine only. Only local visuals differ.
---@param player MoonwellWrappers.Player
function Image:setVisibleFor(player)
    local raw = registry.require(self, 'Image.setVisibleFor')
    ShowImage(raw, Handle.unwrap(player, 'Player', 'Image.setVisibleFor') == GetLocalPlayer())
end
---@param r integer 0-255
---@param g integer 0-255
---@param b integer 0-255
---@param a integer 0-255
function Image:setColor(r, g, b, a) SetImageColor(registry.require(self, 'Image.setColor'), r, g, b, a) end
---@param flag boolean
---@param height number
function Image:setConstantHeight(flag, height)
    SetImageConstantHeight(registry.require(self, 'Image.setConstantHeight'), flag, height)
end
---@param flag boolean
---@param useWaterAlpha boolean
function Image:setAboveWater(flag, useWaterAlpha)
    SetImageAboveWater(registry.require(self, 'Image.setAboveWater'), flag, useWaterAlpha)
end
---@param imageType integer 1 selection, 2 indicator, 3 occlusion mask, 4 ubersplat.
function Image:setType(imageType) SetImageType(registry.require(self, 'Image.setType'), imageType) end
function Image:destroy()
    local raw = registry.dispose(self, 'Image.destroy')
    sizes[self] = nil
    if raw then DestroyImage(raw) end
end

return Image
```

Create `src/wrappers/ubersplat.lua`:

```lua
local Handle = require('wrappers.internal.handle')
local Options = require('wrappers.internal.options')

---@class MoonwellWrappers.Ubersplat
---@field handle ubersplat? Read-only by convention; nil after destruction.
local Ubersplat = {}
---@type MoonwellWrappers.Registry<MoonwellWrappers.Ubersplat, ubersplat>
local registry = Handle.new(Ubersplat, 'Ubersplat')

---@class MoonwellWrappers.UbersplatOptions
---@field color integer[]? {r, g, b, a?}, each 0-255; default white, opaque.
---@field forcePaused boolean? Default false.
---@field noBirthTime boolean? Default false.

---@type MoonwellWrappers.OptionFields
local createFields = {
    color = {'color', {255, 255, 255}}, forcePaused = {'boolean', false}, noBirthTime = {'boolean', false},
}

---@param raw ubersplat?
---@return MoonwellWrappers.Ubersplat?
---@overload fun(raw: nil): nil
function Ubersplat.fromHandle(raw) return registry.wrap(raw) end
---Creates an always-rendered splat.
---@param name string Splat name from Splats\UberSplatData.slk, such as "HMED".
---@param x number
---@param y number
---@param options MoonwellWrappers.UbersplatOptions?
---@return MoonwellWrappers.Ubersplat
function Ubersplat.create(name, x, y, options)
    local o = Options.read(options, createFields, 'Ubersplat.create')
    local c = o.color
    local raw = CreateUbersplat(x, y, name, c[1], c[2], c[3], c[4], o.forcePaused, o.noBirthTime)
    local splat = Handle.created(Ubersplat.fromHandle(raw), 'Ubersplat.create')
    SetUbersplatRenderAlways(raw, true)
    return splat
end
---@return ubersplat
function Ubersplat:getHandle() return registry.require(self, 'Ubersplat.getHandle') end
---@return boolean
function Ubersplat:isDisposed() return registry.isDisposed(self, 'Ubersplat.isDisposed') end
---@param flag boolean
function Ubersplat:show(flag) ShowUbersplat(registry.require(self, 'Ubersplat.show'), flag) end
---Shows the splat on that player's machine only. Only local visuals differ.
---@param player MoonwellWrappers.Player
function Ubersplat:setVisibleFor(player)
    local raw = registry.require(self, 'Ubersplat.setVisibleFor')
    ShowUbersplat(raw, Handle.unwrap(player, 'Player', 'Ubersplat.setVisibleFor') == GetLocalPlayer())
end
function Ubersplat:finish() FinishUbersplat(registry.require(self, 'Ubersplat.finish')) end
function Ubersplat:reset() ResetUbersplat(registry.require(self, 'Ubersplat.reset')) end
function Ubersplat:destroy()
    local raw = registry.dispose(self, 'Ubersplat.destroy')
    if raw then DestroyUbersplat(raw) end
end

return Ubersplat
```

If LuaLS reports the backslash in `Splats\UberSplatData.slk` inside the annotation, leave it: annotations are comments.
Write this file with the file-editing tool, not the shell (backslash pitfall).

- [ ] **Step 4: Run tests to verify they pass**

Run: `deno task test image ubersplat`
Expected: `image: SUITE PASSED: 4 tests` and `ubersplat: SUITE PASSED: 3 tests`.

- [ ] **Step 5: Run all checks and commit**

Run every command in "Running the wrappers checks". Expected: all pass.

```bash
git add src/wrappers/image.lua src/wrappers/ubersplat.lua tests/image.lua tests/ubersplat.lua
git commit -m "feat: Image and Ubersplat wrappers with local visibility"
```

---

### Task 6: Effect depth and flash

**Files:**
- Modify: `src/wrappers/effect.lua` (whole file below)
- Test: `tests/effect.lua`

**Interfaces:**
- Consumes: `Handle.unwrapWidget`, `Handle.unwrap`, `Handle.created`.
- Produces: `Effect.attach(model, Widget, attachmentPoint)`, `Effect.flash(model, x, y)`,
  `Effect.flashOn(model, Widget, attachmentPoint)`, and methods `setColor`, `setAlpha`, `setPlayerColor`,
  `setTimeScale`, `setOrientation`, `setHeight`, `setZ`, `playAnimation`.

- [ ] **Step 1: Write the failing tests**

In `tests/effect.lua`, replace the line

```lua
    fails(function() Effect.attach('model', {}, 'origin') end, 'Unit')
```

with

```lua
    fails(function() Effect.attach('model', {}, 'origin') end, 'Effect.attach: expected Widget wrapper')
```

Replace the header lines

```lua
local Effect = require('wrappers.effect')
local Unit = require('wrappers.unit')
eq(totalCalls(), 0)
```

with

```lua
local Effect = require('wrappers.effect')
local Unit = require('wrappers.unit')
local Item = require('wrappers.item')
local Player = require('wrappers.player')
eq(totalCalls(), 0)
```

Append to `tests/effect.lua`:

```lua
test('effects attach to any widget', function()
    local item = Item.fromHandle({})
    local e = Effect.attach('model.mdx', item, 'origin')
    expectCall('AddSpecialEffectTarget', 'model.mdx', item.handle, 'origin')
    e:destroy(); item:remove()
    fails(function() Effect.attach('model.mdx', item, 'origin') end, 'Effect.attach: Item is disposed')
    eq(callCount('AddSpecialEffectTarget'), 1)
end)

test('effect presentation setters forward exact arguments', function()
    local e, attack = Effect.create('model.mdx', 0, 0), {}
    checkSetters(e, {{'BlzSetSpecialEffectColor', 'setColor', 255, 0, 0}, {'BlzSetSpecialEffectAlpha', 'setAlpha', 128},
        {'BlzSetSpecialEffectTimeScale', 'setTimeScale', 0.5},
        {'BlzSetSpecialEffectOrientation', 'setOrientation', 1, 2, 3}, {'BlzSetSpecialEffectHeight', 'setHeight', 50},
        {'BlzSetSpecialEffectZ', 'setZ', 60}, {'BlzPlaySpecialEffect', 'playAnimation', attack}})
    native('BlzSetSpecialEffectColorByPlayer', function() end)
    e:setPlayerColor(Player.fromIndex(0)); expectCall('BlzSetSpecialEffectColorByPlayer', e.handle, PLAYER_RAW)
    fails(function() e:setPlayerColor(e) end, 'Effect.setPlayerColor: expected Player wrapper')
    eq(callCount('BlzSetSpecialEffectColorByPlayer'), 1)
    e:destroy()
    checkDisposed(e, {'setColor', 'setAlpha', 'setPlayerColor', 'setTimeScale', 'setOrientation', 'setHeight', 'setZ',
        'playAnimation'})
end)

test('flash creates and destroys at once without a wrapper', function()
    local raw = {}
    native('AddSpecialEffect', function() return raw end)
    eq(Effect.flash('boom.mdx', 1, 2), nil)
    eq(callName(1), 'AddSpecialEffect'); eq(callName(2), 'DestroyEffect'); eq(totalCalls(), 2)
    expectCall('AddSpecialEffect', 'boom.mdx', 1, 2); expectCall('DestroyEffect', raw)
    local u = Unit.fromHandle({})
    native('AddSpecialEffectTarget', function() return raw end)
    eq(Effect.flashOn('boom.mdx', u, 'chest'), nil)
    expectCall('AddSpecialEffectTarget', 'boom.mdx', u.handle, 'chest'); eq(callCount('DestroyEffect'), 2)
    native('AddSpecialEffect', function() return nil end)
    fails(function() Effect.flash('m', 0, 0) end, 'Effect.flash')
    native('AddSpecialEffectTarget', function() return nil end)
    fails(function() Effect.flashOn('m', u, 'origin') end, 'Effect.flashOn')
    fails(function() Effect.flashOn('m', {}, 'origin') end, 'Effect.flashOn: expected Widget wrapper')
    eq(callCount('DestroyEffect'), 2)
    native('AddSpecialEffect', function() return {} end)
    native('AddSpecialEffectTarget', function() return {} end)
    u:remove()
end)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `deno task test effect`
Expected: FAIL: `invalid targets and nil native results fail clearly` (message is still `expected Unit wrapper`),
`effects attach to any widget`, `effect presentation setters ...` (`attempt to call a nil value (method 'setColor')`)
and `flash ...`.

- [ ] **Step 3: Write minimal implementation**

Replace `src/wrappers/effect.lua` with:

```lua
local Handle = require('wrappers.internal.handle')

---@class MoonwellWrappers.Effect
---@field handle effect? Read-only by convention; nil after destruction.
local Effect = {}
---@type MoonwellWrappers.Registry<MoonwellWrappers.Effect, effect>
local registry = Handle.new(Effect, 'Effect')

---@param raw effect?
---@return MoonwellWrappers.Effect?
---@overload fun(raw: nil): nil
function Effect.fromHandle(raw) return registry.wrap(raw) end
---@param model string
---@param x number
---@param y number
---@return MoonwellWrappers.Effect
function Effect.create(model, x, y)
    return Handle.created(Effect.fromHandle(AddSpecialEffect(model, x, y)), 'Effect.create')
end
---@param model string
---@param target MoonwellWrappers.Widget
---@param attachmentPoint string
---@return MoonwellWrappers.Effect
function Effect.attach(model, target, attachmentPoint)
    local raw = Handle.unwrapWidget(target, 'Effect.attach')
    return Handle.created(Effect.fromHandle(AddSpecialEffectTarget(model, raw, attachmentPoint)), 'Effect.attach')
end
---Creates and destroys an effect at once, which plays its death animation. Returns nothing.
---@param model string
---@param x number
---@param y number
function Effect.flash(model, x, y)
    DestroyEffect(Handle.created(AddSpecialEffect(model, x, y), 'Effect.flash'))
end
---Attaches and destroys an effect at once, which plays its death animation. Returns nothing.
---@param model string
---@param target MoonwellWrappers.Widget
---@param attachmentPoint string
function Effect.flashOn(model, target, attachmentPoint)
    local raw = Handle.unwrapWidget(target, 'Effect.flashOn')
    DestroyEffect(Handle.created(AddSpecialEffectTarget(model, raw, attachmentPoint), 'Effect.flashOn'))
end
---@return effect
function Effect:getHandle() return registry.require(self, 'Effect.getHandle') end
---@return boolean
function Effect:isDisposed() return registry.isDisposed(self, 'Effect.isDisposed') end
---@param x number
---@param y number
---@param z number
function Effect:setPosition(x, y, z)
    BlzSetSpecialEffectPosition(registry.require(self, 'Effect.setPosition'), x, y, z)
end
---@param scale number
function Effect:setScale(scale) BlzSetSpecialEffectScale(registry.require(self, 'Effect.setScale'), scale) end
---@param r integer 0-255
---@param g integer 0-255
---@param b integer 0-255
function Effect:setColor(r, g, b) BlzSetSpecialEffectColor(registry.require(self, 'Effect.setColor'), r, g, b) end
---@param alpha integer 0-255
function Effect:setAlpha(alpha) BlzSetSpecialEffectAlpha(registry.require(self, 'Effect.setAlpha'), alpha) end
---@param player MoonwellWrappers.Player
function Effect:setPlayerColor(player)
    local raw = registry.require(self, 'Effect.setPlayerColor')
    BlzSetSpecialEffectColorByPlayer(raw, Handle.unwrap(player, 'Player', 'Effect.setPlayerColor'))
end
---@param scale number
function Effect:setTimeScale(scale)
    BlzSetSpecialEffectTimeScale(registry.require(self, 'Effect.setTimeScale'), scale)
end
---@param yaw number Radians.
---@param pitch number Radians.
---@param roll number Radians.
function Effect:setOrientation(yaw, pitch, roll)
    BlzSetSpecialEffectOrientation(registry.require(self, 'Effect.setOrientation'), yaw, pitch, roll)
end
---@param height number
function Effect:setHeight(height) BlzSetSpecialEffectHeight(registry.require(self, 'Effect.setHeight'), height) end
---@param z number
function Effect:setZ(z) BlzSetSpecialEffectZ(registry.require(self, 'Effect.setZ'), z) end
---@param animation animtype
function Effect:playAnimation(animation)
    BlzPlaySpecialEffect(registry.require(self, 'Effect.playAnimation'), animation)
end
function Effect:destroy()
    local raw = registry.dispose(self, 'Effect.destroy')
    if raw then DestroyEffect(raw) end
end

return Effect
```

- [ ] **Step 4: Run test to verify it passes**

Run: `deno task test effect`
Expected: `effect: SUITE PASSED: 5 tests`. Also run `deno task test imports`: Effect must still load no other public
module (expected: PASS).

- [ ] **Step 5: Run all checks and commit**

Run every command in "Running the wrappers checks". Expected: all pass. The existing LuaLS positive fixture calls
`Effect.attach('model.mdx', unit, 'origin')`; a Unit is a Widget, so it stays clean.

```bash
git add src/wrappers/effect.lua tests/effect.lua
git commit -m "feat: Effect presentation setters, widget attach and flash helpers"
```

---

### Task 7: Item and Destructable enumeration

**Files:**
- Modify: `src/wrappers/internal/callback.lua` (add `Callback.optional`)
- Modify: `src/wrappers/group.lua` (use `Callback.optional`; delete local `checkFilter`)
- Modify: `src/wrappers/item.lua`, `src/wrappers/destructable.lua`
- Test: `tests/item.lua`, `tests/destructable.lua`

**Interfaces:**
- Consumes: `Handle.unwrap`; the existing Item and Destructable registries.
- Produces: `Callback.optional(value, operation)`; `Item.enumInRect(rect, filter?) -> MoonwellWrappers.Item[]`;
  `Destructable.enumInRect(rect, filter?) -> MoonwellWrappers.Destructable[]`.

- [ ] **Step 1: Write the failing tests**

In `tests/item.lua`, after the line `local Player = require('wrappers.player')`, add
`local Rect = require('wrappers.rect')`. Append to `tests/item.lua`:

```lua
local enumerated, current, inNative, seenRect, seenFilter = {}, nil, false, nil, 'unset'
native('GetEnumItem', function() return current end)
native('EnumItemsInRect', function(rect, filter, callback)
    seenRect, seenFilter, inNative = rect, filter, true
    for _, raw in ipairs(enumerated) do current = raw; callback() end
    current, inNative = nil, false
end)

test('enumInRect returns a snapshot of the enumerated items', function()
    local area, a, b = Rect.fromHandle({}), {}, {}
    enumerated = {a, b}
    local all = Item.enumInRect(area)
    eq(#all, 2); eq(all[1], Item.fromHandle(a)); eq(all[2], Item.fromHandle(b))
    eq(seenRect, area.handle); eq(seenFilter, nil); eq(callCount('EnumItemsInRect'), 1)
    enumerated = {}
    eq(#Item.enumInRect(area), 0); eq(#all, 2)
end)

test('enumInRect filters after the native returns, as ordinary Lua', function()
    local area, a, b = Rect.fromHandle({}), {}, {}
    enumerated = {a, b}
    local seen = {}
    local kept = Item.enumInRect(area, function(item)
        assert(not inNative, 'filter ran inside the native enumeration')
        seen[#seen + 1] = item
        return item.handle == b
    end)
    eq(#seen, 2); eq(#kept, 1); eq(kept[1], Item.fromHandle(b))
    fails(function() Item.enumInRect(area, function() error('boom') end) end, 'boom')
end)

test('enumInRect validates its rect and filter before the native', function()
    fails(function() Item.enumInRect({}) end, 'Item.enumInRect: expected Rect wrapper')
    fails(function() Item.enumInRect(Rect.fromHandle({}), 'all') end, 'Item.enumInRect: expected a callback function')
    eq(callCount('EnumItemsInRect'), 0)
end)
```

In `tests/destructable.lua`, add `local Rect = require('wrappers.rect')` after the existing
`require('wrappers.destructable')` line (keep `eq(totalCalls(), 0)` after all requires). Append to
`tests/destructable.lua`:

```lua
local enumerated, current, inNative, seenRect, seenFilter = {}, nil, false, nil, 'unset'
native('GetEnumDestructable', function() return current end)
native('EnumDestructablesInRect', function(rect, filter, callback)
    seenRect, seenFilter, inNative = rect, filter, true
    for _, raw in ipairs(enumerated) do current = raw; callback() end
    current, inNative = nil, false
end)

test('enumInRect returns a snapshot of the enumerated destructables', function()
    local area, a, b = Rect.fromHandle({}), {}, {}
    enumerated = {a, b}
    local all = Destructable.enumInRect(area)
    eq(#all, 2); eq(all[1], Destructable.fromHandle(a)); eq(all[2], Destructable.fromHandle(b))
    eq(seenRect, area.handle); eq(seenFilter, nil); eq(callCount('EnumDestructablesInRect'), 1)
end)

test('enumInRect filters after the native returns, as ordinary Lua', function()
    local area, a, b = Rect.fromHandle({}), {}, {}
    enumerated = {a, b}
    local kept = Destructable.enumInRect(area, function(tree)
        assert(not inNative, 'filter ran inside the native enumeration')
        return tree.handle == a
    end)
    eq(#kept, 1); eq(kept[1], Destructable.fromHandle(a))
    fails(function() Destructable.enumInRect(area, function() error('boom') end) end, 'boom')
end)

test('enumInRect validates its rect and filter before the native', function()
    fails(function() Destructable.enumInRect({}) end, 'Destructable.enumInRect: expected Rect wrapper')
    fails(function() Destructable.enumInRect(Rect.fromHandle({}), 1) end,
        'Destructable.enumInRect: expected a callback function')
    eq(callCount('EnumDestructablesInRect'), 0)
end)
```

(Check the top of `tests/destructable.lua` first: if its local for the module has a different name than
`Destructable`, use that name in the appended tests.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `deno task test item destructable`
Expected: FAIL: `attempt to call a nil value (field 'enumInRect')` in the new tests; existing tests still pass.

- [ ] **Step 3: Write minimal implementation**

In `src/wrappers/internal/callback.lua`, after `Callback.check`, add:

```lua
---Accepts nil or a function; used for optional filters.
---@param value unknown
---@param operation string
function Callback.optional(value, operation)
    if value ~= nil and type(value) ~= 'function' then
        error('[wrappers] ' .. operation .. ': expected a callback function', 3)
    end
end
```

In `src/wrappers/group.lua`, delete the local function `checkFilter` (its doc comment `---Level 3 blames ...`, the two
`---@param` lines and the function body), and replace each of the four calls `checkFilter(filter, '<op>')` with
`Callback.optional(filter, '<op>')` (same operation strings). Group already requires `Callback`.

In `src/wrappers/item.lua`, add `local Callback = require('wrappers.internal.callback')` after the `Widget` require,
and add before `Widget.install(Item, registry)`:

```lua
---Returns a new array of the items in the rect. `filter` runs afterwards, as ordinary Lua, and keeps the items for
---which it returns truthy; its errors propagate.
---@param rect MoonwellWrappers.Rect
---@param filter (fun(item: MoonwellWrappers.Item): any)?
---@return MoonwellWrappers.Item[]
function Item.enumInRect(rect, filter)
    local rawRect = Handle.unwrap(rect, 'Rect', 'Item.enumInRect')
    Callback.optional(filter, 'Item.enumInRect')
    local raws = {}
    -- Warcraft accepts a null filter; the generated JASS signature cannot express that.
    ---@diagnostic disable-next-line: param-type-mismatch
    EnumItemsInRect(rawRect, nil, function() raws[#raws + 1] = GetEnumItem() end)
    local items = {}
    for index, raw in ipairs(raws) do items[index] = assert(Item.fromHandle(raw)) end
    if filter == nil then return items end
    local kept = {}
    for _, item in ipairs(items) do
        if filter(item) then kept[#kept + 1] = item end
    end
    return kept
end
```

In `src/wrappers/destructable.lua`, add `local Callback = require('wrappers.internal.callback')` after the `Widget`
require, and add before `Widget.install(Destructable, registry)`:

```lua
---Returns a new array of the destructables in the rect. `filter` runs afterwards, as ordinary Lua, and keeps the
---destructables for which it returns truthy; its errors propagate.
---@param rect MoonwellWrappers.Rect
---@param filter (fun(destructable: MoonwellWrappers.Destructable): any)?
---@return MoonwellWrappers.Destructable[]
function Destructable.enumInRect(rect, filter)
    local rawRect = Handle.unwrap(rect, 'Rect', 'Destructable.enumInRect')
    Callback.optional(filter, 'Destructable.enumInRect')
    local raws = {}
    -- Warcraft accepts a null filter; the generated JASS signature cannot express that.
    ---@diagnostic disable-next-line: param-type-mismatch
    EnumDestructablesInRect(rawRect, nil, function() raws[#raws + 1] = GetEnumDestructable() end)
    local destructables = {}
    for index, raw in ipairs(raws) do destructables[index] = assert(Destructable.fromHandle(raw)) end
    if filter == nil then return destructables end
    local kept = {}
    for _, destructable in ipairs(destructables) do
        if filter(destructable) then kept[#kept + 1] = destructable end
    end
    return kept
end
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `deno task test item destructable group imports`
Expected: all four suites print `SUITE PASSED`. Group's existing filter-type tests still pass with the same message.

- [ ] **Step 5: Run all checks and commit**

Run every command in "Running the wrappers checks". Expected: all pass. If LuaLS in the integration run reports
`param-type-mismatch` on the enumeration natives, the suppression line is missing or misplaced.

```bash
git add src/wrappers/internal/callback.lua src/wrappers/group.lua src/wrappers/item.lua src/wrappers/destructable.lua tests/item.lua tests/destructable.lua
git commit -m "feat: Item and Destructable enumeration in a rect"
```

---

### Task 8: Import graph, bundle and editor coverage

**Files:**
- Modify: `tests/imports.lua`, `tools/integration.ts`, `tests/editor-positive.lua`, `tests/editor-negative.lua`,
  `tests/editor-positive.yue`

**Interfaces:**
- Consumes: every module from Tasks 1–7.
- Produces: automated proof that new modules load no other public module, that a TextTag-only map bundles no other
  public module, and LuaLS coverage of the new API.

- [ ] **Step 1: Extend the import test**

In `tests/imports.lua`, replace the first line with:

```lua
for _, name in ipairs({'trigger', 'effect', 'timer', 'destructable', 'rect', 'region', 'texttag', 'sound', 'lightning',
    'image', 'ubersplat', 'fogmodifier'}) do
    require('wrappers.' .. name)
end
```

Run: `deno task test imports`
Expected: `imports: SUITE PASSED: 1 tests` (the modules were written to import only internals; if it fails, a module
imports a public module and must use `Handle.unwrap` instead).

- [ ] **Step 2: Extend the bundle checks in `tools/integration.ts`**

Replace the Unit-only exclusion line

```ts
for (const unused of ["effect", "trigger", "group", "timer", "destructable", "rect", "region", "force"]) {
```

with

```ts
for (
  const unused of [
    "effect",
    "trigger",
    "group",
    "timer",
    "destructable",
    "rect",
    "region",
    "force",
    "texttag",
    "sound",
    "lightning",
    "image",
    "ubersplat",
    "fogmodifier",
  ]
) {
```

(Only the list changes; the loop body stays. `deno fmt` decides the final layout.)

Replace the whole Trigger-only block, from the comment
`// Trigger takes wrapper arguments only, so a Trigger-only map must bundle no other public module.` through
`console.log("Moonwell: a Trigger-only map bundles no other public module");`, with:

```ts
// Trigger and TextTag take wrapper arguments only, so a map importing just one of them bundles no other public module.
const publicModules = [
  "unit",
  "player",
  "item",
  "destructable",
  "rect",
  "region",
  "force",
  "group",
  "timer",
  "effect",
  "trigger",
  "texttag",
  "sound",
  "lightning",
  "image",
  "ubersplat",
  "fogmodifier",
];
const soloEntries: Record<string, string> = {
  trigger: 'import "wrappers.trigger" as Trigger\nt = Trigger.create!\nt\\destroy!\n',
  texttag: 'import "wrappers.texttag" as TextTag\nt = TextTag.create!\nt\\destroy!\nTextTag.float "+1", 0, 0\n',
};
for (const [entry, source] of Object.entries(soloEntries)) {
  await Deno.writeTextFile(join(consumer, "src/main.yue"), source);
  await moonwell(["build"]);
  const soloBundle = await Deno.readTextFile(join(consumer, "dist/stage/map.w3x/war3map.lua"));
  // Guard the absence checks below: they would pass vacuously if the bundle held no wrapper module at all.
  if (!soloBundle.includes(`wrappers.${entry}`)) throw new Error(`${entry}-only bundle lacks wrappers.${entry}`);
  for (const unused of publicModules) {
    if (unused !== entry && soloBundle.includes(`wrappers.${unused}`)) {
      throw new Error(`${entry}-only bundle includes wrappers.${unused}`);
    }
  }
}
console.log("Moonwell: Trigger-only and TextTag-only maps bundle no other public module");
```

(`deno fmt` decides the array layout; run it after editing.)

- [ ] **Step 3: Extend the LuaLS positive fixtures**

In `tests/editor-positive.lua`, after `local Force = require('wrappers.force')`, add:

```lua
local TextTag = require('wrappers.texttag')
local Sound = require('wrappers.sound')
local Lightning = require('wrappers.lightning')
local Image = require('wrappers.image')
local Ubersplat = require('wrappers.ubersplat')
local FogModifier = require('wrappers.fogmodifier')
```

After the line `for _, member in ipairs(force:getPlayers()) do member:addGold(10) end`, add:

```lua
local tag = TextTag.create()
tag:setText('gate', 10)
tag:setPositionOnUnit(unit, 16)
tag:setVisibleFor(PlayerWrapper.fromIndex(0))
TextTag.float('+5', 0, 0, {size = 12, color = {255, 0, 0}, player = PlayerWrapper.fromIndex(0)})
local sound = Sound.create('war.flac', {is3D = true, fadeIn = 5})
sound:attachToUnit(unit)
sound:playFor(PlayerWrapper.fromIndex(0))
Sound.playOnce('war.flac', {volume = 100, x = 0, y = 0})
local bolt = Lightning.create('CLPB', 0, 0, 0, 100, 100, 0)
if bolt:move(0, 0, 0, 200, 200, 0, true) then bolt:setColor(1, 1, 1, 1) end
local image = Image.create('aoe.blp', 128, 128, 0, 0, 1)
image:setVisibleFor(PlayerWrapper.fromIndex(0))
local maybeImage = Image.fromHandle(image.handle)
if maybeImage then maybeImage:setPosition(10, 10) end
local splat = Ubersplat.create('HMED', 0, 0, {noBirthTime = true})
splat:finish()
local fog = FogModifier.rect(PlayerWrapper.fromIndex(0), FOG_OF_WAR_VISIBLE, area, true, false)
fog:start()
local itemEffect = Effect.attach('model.mdx', item, 'origin')
itemEffect:setPlayerColor(PlayerWrapper.fromIndex(0))
itemEffect:playAnimation(ANIM_TYPE_ATTACK)
Effect.flashOn('model.mdx', tree, 'origin')
Effect.flash('model.mdx', 0, 0)
for _, found in ipairs(Item.enumInRect(area, function(candidate) return candidate:getCharges() > 0 end)) do
    found:setCharges(1)
end
for _, found in ipairs(Destructable.enumInRect(area)) do found:kill() end
itemEffect:destroy()
fog:destroy()
splat:destroy()
image:destroy()
bolt:destroy()
sound:destroy()
tag:destroy()
```

In `tests/editor-positive.yue`, add `import "wrappers.texttag" as TextTag` after the last `import` line, and add as the
last line inside the `mw.on_main ->` block (same two-space indentation as its other lines):

```text
  TextTag.float "+1", 0, 0, size: 12, color: {255, 0, 0}
```

- [ ] **Step 4: Extend the LuaLS negative fixture**

In `tests/editor-negative.lua`, after `local Trigger = require('wrappers.trigger')`, add:

```lua
local Effect = require('wrappers.effect')
local TextTag = require('wrappers.texttag')
```

Before the final `return true`, add:

```lua
Effect.attach('model.mdx', Timer.create(), 'origin') -- EXPECT param-type-mismatch
TextTag.create():setVisibleFor(unit) -- EXPECT param-type-mismatch
TextTag.float('x', 0, 0, {size = 'big'}) -- EXPECT param-type-mismatch
```

- [ ] **Step 5: Run the integration check and settle the negative lines**

Run: `deno task test:integration`
Expected: PASS, with `LuaLS: 11 intentional type errors detected at the expected lines` and
`Moonwell: Trigger-only and TextTag-only maps bundle no other public module`.

If it fails only because LuaLS 3.19.1 reports a different code on the `TextTag.float('x', 0, 0, {size = 'big'})` line,
change that line's `EXPECT` to the reported code. If LuaLS reports nothing on that line, delete the line and record in
the task report that LuaLS 3.19.1 does not check option-table field types (runtime validation still does). Any other
mismatch is a real failure: fix the code or fixture.

- [ ] **Step 6: Run all checks and commit**

Run every command in "Running the wrappers checks". Expected: all pass.

```bash
git add tests/imports.lua tools/integration.ts tests/editor-positive.lua tests/editor-negative.lua tests/editor-positive.yue
git commit -m "test: import graph, TextTag-only bundle and editor fixtures for presentation wrappers"
```

---

### Task 9: Gate example and documentation

**Files:**
- Modify: `examples/gate.yue`, `README.md`, `CHANGELOG.md`, `CONTRIBUTING.md`, `AGENTS.md` (wrappers)
- Modify: Moonwell `AGENTS.md` (in `C:/Users/mdlsvensson/Repo/moonwell`)

**Interfaces:**
- Consumes: the full v0.3.0 API.
- Produces: the in-game gate the maintainer runs, and the docs.

- [ ] **Step 1: Add the presentation gate to `examples/gate.yue`**

Edit with the file-editing tool, not the shell (backslashes). Replace the third header comment line

```text
-- Type -gate in chat twice while the hero is alive. CONTRIBUTING lists every expected message.
```

with

```text
-- Type -gate in chat twice while the hero is alive. CONTRIBUTING lists every expected message.
-- Set presentation = true for a separate run of the v0.3.0 presentation gate only.
```

After `import "wrappers.effect" as Effect`, add:

```text
import "wrappers.texttag" as TextTag
import "wrappers.sound" as Sound
import "wrappers.lightning" as Lightning
import "wrappers.image" as Image
import "wrappers.ubersplat" as Ubersplat
import "wrappers.fogmodifier" as FogModifier
```

After `probes = false`, add `presentation = false`.

Before the `-- Start just after the map loads` comment, add:

```text
-- v0.3.0 presentation, around the map centre. Run with presentation = true.
presentationGate = (owner) ->
  other = Player.fromIndex 1
  marker = Unit.create owner, $FourCC("hfoo"), 0, 0, 270
  tag = TextTag.create!
  tag\setText "Wrapper text tag", 12
  tag\setColor 255, 220, 0, 255
  tag\setPosition -250, 250, 0
  TextTag.float "Wrapper float for another player", 0, 150, player: other
  bolt = Lightning.create "CLPB", -300, -250, 60, 300, -250, 60
  aoe = Image.create "ReplaceableTextures\\Selection\\SpellAreaOfEffect.blp", 256, 256, 0, 0, 1
  splat = Ubersplat.create "HMED", 350, 250
  near = FogModifier.radius owner, FOG_OF_WAR_VISIBLE, 1000, 1000, 300, true, false
  area = Rect.create -1200, -1200, -900, -900
  far = FogModifier.rect owner, FOG_OF_WAR_VISIBLE, area, true, false
  near\start!
  far\start!
  tinted = Effect.create "units\\human\\Footman\\Footman.mdl", -400, 0
  tinted\setColor 255, 0, 0
  tinted\setAlpha 160
  tinted\setTimeScale 0.5
  tinted\setOrientation math.pi / 2, 0, 0
  tinted\setHeight 100
  tinted\playAnimation ANIM_TYPE_ATTACK
  blue = Effect.create "units\\human\\Footman\\Footman.mdl", -550, 0
  blue\setPlayerColor other
  claws = Item.create $FourCC("ratc"), 200, -100
  onItem = Effect.attach "Abilities\\Spells\\Other\\TalkToMe\\TalkToMe.mdl", claws, "origin"
  bell = Sound.create "Sound\\Interface\\Warning.flac"
  voice = Sound.create "Units\\Human\\Footman\\FootmanYes1.flac", is3D: true
  voice\setDistances 600, 10000
  voice\setDistanceCutoff 3000
  print "Wrapper presentation started; sound duration", bell\getDuration!
  box = Rect.create -900, 500, -700, 700
  potions = {
    Item.create($FourCC("ratc"), -800, 600)
    Item.create($FourCC("phea"), -780, 620)
    Item.create($FourCC("phea"), -820, 580)
  }
  trees = {
    Destructable.create($FourCC("LTlt"), -850, 650, 270, 1, 0)
    Destructable.create($FourCC("LTlt"), -750, 650, 270, 1, 0)
  }
  potionId = $FourCC("phea")
  print "Wrapper enumerated potions", #Item.enumInRect(box, (item) -> item\getTypeId! == potionId)
  print "Wrapper enumerated trees", #Destructable.enumInRect(box)
  clap = "Abilities\\Spells\\Human\\ThunderClap\\ThunderClapCaster.mdl"
  steps = Timer.create!
  step = 0
  steps\start 2, true, (self) ->
    step += 1
    if step == 1
      TextTag.float "Wrapper float", 0, 100
      bell\play!
      print "Wrapper sound play"
    elseif step == 2
      bell\stop!
      Sound.playOnce "Sound\\Interface\\QuestNew.flac"
      print "Wrapper playOnce first call"
    elseif step == 3
      Sound.playOnce "Sound\\Interface\\QuestNew.flac"
      print "Wrapper playOnce second call"
    elseif step == 4
      voice\setPosition 0, 0, 0
      voice\play!
      print "Wrapper 3D sound"
    elseif step == 5
      bell\playFor other
      print "Wrapper playFor another player"
    elseif step == 6
      tag\setPositionOnUnit marker, 0
      tag\setText "Wrapper tag moved", 10
      bolt\move -300, 250, 60, 300, 250, 60
      bolt\setColor 0, 1, 0, 1
      aoe\setColor 0, 255, 0, 255
      aoe\setPosition 0, 300
      splat\finish!
      far\stop!
      Effect.flash clap, 0, -200
      print "Wrapper presentation changed"
    elseif step == 7
      aoe\setVisibleFor other
      Effect.flashOn clap, marker, "origin"
      print "Wrapper image hidden"
    elseif step == 10
      tag\destroy!
      bolt\destroy!
      aoe\destroy!
      splat\destroy!
      near\destroy!
      far\destroy!
      area\destroy!
      box\destroy!
      tinted\destroy!
      blue\destroy!
      onItem\destroy!
      claws\remove!
      bell\destroy!
      voice\destroy!
      item\remove! for item in *potions
      tree\remove! for tree in *trees
      marker\remove!
      self\destroy!
      print "Wrapper presentation cleanup passed"
```

Replace the body of the final `mw.on_main` block's timer callback so the whole block reads:

```text
mw.on_main ->
  start = Timer.create!
  start\start 0, false, (self) ->
    self\destroy!
    owner = Player.fromIndex 0
    if presentation
      presentationGate owner
    else
      foundationGate owner
      broadGate owner
```

Run: `deno task test:integration`
Expected: PASS, including `Gate example: every module builds and editor diagnostics are clean`. If LuaLS reports a
diagnostic in the gate, fix the gate code (for example a nullable result), not the library.

- [ ] **Step 2: Update `CONTRIBUTING.md`'s in-game gate**

Insert after step 6 (the `probes = true` step) a new step 7, and renumber the old steps 7, 8 and 9 to 8, 9 and 10:

```markdown
7. Presentation (v0.3.0): set `presentation = true` and run again; only the presentation gate runs, around the map
   centre. At start `Wrapper presentation started; sound duration <n>` prints (record `n`; 0 can mean the file was not
   loaded yet), then
   `Wrapper enumerated potions 2` and `Wrapper enumerated trees 2`. Visible at once: a yellow `Wrapper text tag`
   upper left; a chain lightning bolt below the footman; the area-of-effect circle centred under the footman; a
   building-base splat upper right; a red, half-transparent, slowed Footman model turned sideways, raised and attacking
   to the left, and a blue (player 2 colour) Footman model beside it; the talk-to-me mark on the claws item. The
   minimap shows a revealed circle towards the top right and a revealed square towards the bottom left.
   `Wrapper float for another player` never appears. At 2 s `Wrapper float` rises and fades, and the warning sound plays
   (`Wrapper sound play`). At 4 s it stops, `Wrapper playOnce first call` prints and the quest sound plays. If it is
   silent then but audible at 6 s (`Wrapper playOnce second call`), record first-play silence (spec §5.2) and stop:
   the fix is decided with the maintainer before release. At 8 s the footman voice plays from the centre
   (`Wrapper 3D sound`). At 10 s `Wrapper playFor another player` prints and nothing plays. At 12 s
   (`Wrapper presentation changed`): the tag jumps to the footman and reads `Wrapper tag moved`, the bolt moves above
   the footman and turns green, the circle turns green and moves up, the splat fades out, the bottom-left reveal ends
   and a thunder clap flashes below the footman. At 14 s the circle disappears and a thunder clap flashes on the
   footman (`Wrapper image hidden`). At 20 s everything disappears and `Wrapper presentation cleanup passed` prints. If
   a sound, model or splat never appears or plays in any run, its path or name may not exist in this game version:
   substitute one from World Editor and record it. Restore `presentation = false`.
```

In the renumbered step 8 (the `--minify` step), change `repeat steps 2–5` to `repeat steps 2–5 and 7`.

In the renumbered step 9 (two-player run), append: `The same online check covers v0.3.0's local visibility:
setVisibleFor, playFor and the player options of TextTag.float and Sound.playOnce show or play only for that player,
with no desync.`

- [ ] **Step 3: Update `README.md`**

Replace the intro sentence (line 3) with:

```markdown
Annotated Lua 5.3 library for Warcraft III. It provides Player, Unit, Item, Destructable, Rect, Region, Force, Timer,
Trigger, Group, Effect, TextTag, Sound, Lightning, Image, Ubersplat and FogModifier wrappers, editor completion, stable
handle identity and explicit cleanup.
```

Replace the Status paragraph with:

```markdown
**Status:** `v0.2.0` (broad coverage) released 2026-09-29; its in-game gate passed. v0.3.0 (presentation: text tags,
sounds, lightning, images, ubersplats, fog modifiers, deeper effects, item and destructable enumeration) is in
development on main. Multiplayer desync checks are deferred until before Moonwell 1.0. Dialogs, multiboards, frames and
other UI types are not wrapped yet.
```

In "Handles and cleanup", replace `Timer, Trigger, Group, Effect, Rect, Region and Force stay cached until you destroy
them;` with `Timer, Trigger, Group, Effect, Rect, Region, Force and the presentation classes stay cached until you
destroy them;`, replace `` `timer/trigger/group/effect/rect/region/force:destroy()`. `` with
`` `timer/trigger/group/effect/rect/region/force:destroy()`, and `destroy()` on the presentation classes. ``, and replace
`Groups do not own their units, and effects do not own their targets.` with `Groups do not own their units, effects do
not own their targets, and fog modifiers do not own their rects.`

In "Widgets", replace `` `unit:damageTarget` and `trigger:registerDeathEvent` `` with `` `unit:damageTarget`,
`trigger:registerDeathEvent`, `Effect.attach` and `Effect.flashOn` ``.

Add a new section after "Widgets":

```markdown
## Presentation

TextTag, Sound, Lightning, Image, Ubersplat and FogModifier wrappers exist only for objects the map owns: the game
never ends them on its own, so `destroy()` is their only cleanup. `TextTag.create()` makes a permanent tag, and a Sound
wrapper is never released when it finishes. One-shot presentation uses helpers that return nothing, so no wrapper can
go stale:

- `TextTag.float(text, x, y, options?)`: floating text that the game removes after its lifespan. Options: `size` (10),
  `heightOffset` (0), `color` (`{r, g, b, a?}`, white), `speed` (64) and `angle` (degrees, 90), `lifespan` (2),
  `fadepoint` (1) and `player` (show to one player only). It does nothing when the game has no free text tag.
- `Sound.playOnce(path, options?)`: plays a sound once and releases it. Options: `volume` (0–127, 127); `x`, `y` and
  `z` (given `x` and `y`, the sound is 3D at that point; `z` defaults to 0); `player` (hear it on one player's machine
  only; the others play it at volume 0).
- `Effect.flash(model, x, y)` and `Effect.flashOn(model, Widget, attachmentPoint)`: create and destroy an effect at
  once, which plays its death animation.

`Sound.create(path, options?)` takes `looping`, `is3D` and `stopWhenOutOfRange` (false), `fadeIn` and `fadeOut` (10)
and `eax` (`"DefaultEAXON"`). `Ubersplat.create(name, x, y, options?)` takes a splat name from
`Splats\UberSplatData.slk` and the options `color` (white), `forcePaused` and `noBirthTime` (false); the splat is always
rendered. Options tables reject unknown keys and wrong types.

`setVisibleFor(Player)` (TextTag, Image, Ubersplat) and `sound:playFor(Player)` compare with the local player, so only
local visuals and audio differ; the objects exist on every machine. `show(flag)` afterwards applies to everyone.
Lightning has no local visibility. There is no `sound:isPlaying()`, and Effect has no position getters: those natives
answer differently on each machine.

Value ranges are the natives': colors are integers 0–255, except `lightning:setColor`, which takes numbers 0–1. Sound
volume is 0–127 and `getDuration()` is in milliseconds. Text tag `size` is World Editor's font size; `setVelocity` takes
native units. Effect orientation is in radians. `Image.create(path, width, height, x, y, imageType)` centres the image
on `x, y` and makes it visible; image types are 1 selection, 2 indicator, 3 occlusion mask and 4 ubersplat.
`image:setPosition` also centres, so it fails on an image wrapped with `fromHandle`, whose size is unknown. Fog
modifiers start stopped.

`Item.enumInRect(Rect, filter?)` and `Destructable.enumInRect(Rect, filter?)` return a new dense array of what the
native enumerates. The filter runs afterwards as ordinary Lua and keeps the objects for which it returns truthy; its
errors propagate.
```

In the API reference table:

- In the `wrappers.item` row, replace `` `create(typeId, x, y)`; `` with `` `create(typeId, x, y)`,
  `enumInRect(Rect, filter?)`; ``.
- In the `wrappers.destructable` row, replace `` `create(typeId, x, y, facing, scale, variation)`; `` with
  `` `create(typeId, x, y, facing, scale, variation)`, `enumInRect(Rect, filter?)`; ``.
- Replace the `wrappers.effect` row's second column with: `` `create(model,x,y)`, `attach(model,Widget,attachmentPoint)`,
  `flash(model, x, y)`, `flashOn(model, Widget, attachmentPoint)`; `setPosition(x,y,z)`, `setScale(scale)`,
  `setColor(r, g, b)`, `setAlpha(a)`, `setPlayerColor(Player)`, `setTimeScale(scale)`, `setOrientation(yaw, pitch,
  roll)`, `setHeight(height)`, `setZ(z)`, `playAnimation(animtype)`, `destroy()` ``.
- Add rows after `wrappers.effect`:

```markdown
| `wrappers.texttag` | `create()`, `float(text, x, y, options?)`; `setText(text, size)`, `setColor(r, g, b, a)`, `setPosition(x, y, heightOffset)`, `setPositionOnUnit(Unit, heightOffset)`, `setVelocity(xvel, yvel)`, `setSuspended(flag)`, `show(flag)`, `setVisibleFor(Player)`, `destroy()` |
| `wrappers.sound` | `create(path, options?)`, `playOnce(path, options?)`; `play()`, `playFor(Player)`, `stop(fadeOut?)`, `setVolume(volume)`, `setPitch(pitch)`, `setChannel(channel)`, `setPosition(x, y, z)`, `attachToUnit(Unit)`, `setDistances(min, max)`, `setDistanceCutoff(cutoff)`, `getDuration()`, `destroy()` |
| `wrappers.lightning` | `create(code, x1, y1, z1, x2, y2, z2, checkVisibility?)`; `move(x1, y1, z1, x2, y2, z2, checkVisibility?)` and `setColor(r, g, b, a)` (both return boolean), `destroy()` |
| `wrappers.image` | `create(path, width, height, x, y, imageType)`; `setPosition(x, y, z?)`, `show(flag)`, `setVisibleFor(Player)`, `setColor(r, g, b, a)`, `setConstantHeight(flag, height)`, `setAboveWater(flag, useWaterAlpha)`, `setType(imageType)`, `destroy()` |
| `wrappers.ubersplat` | `create(name, x, y, options?)`; `show(flag)`, `setVisibleFor(Player)`, `finish()`, `reset()`, `destroy()` |
| `wrappers.fogmodifier` | `radius(Player, fogstate, x, y, radius, useSharedVision, afterUnits)`, `rect(Player, fogstate, Rect, useSharedVision, afterUnits)`; `start()`, `stop()`, `destroy()` |
```

Run `deno fmt` to realign the table, then `deno fmt --check`.

- [ ] **Step 4: Update `CHANGELOG.md`**

Insert above `## 0.2.0 (2026-09-29)`:

```markdown
## Unreleased

- New wrappers: TextTag, Sound, Lightning, Image, Ubersplat and FogModifier. They exist only for objects the map owns
  and destroys: `TextTag.create` makes a permanent tag, and a Sound wrapper is never released when done.
- Fire-and-forget helpers that return nothing: `TextTag.float`, `Sound.playOnce`, `Effect.flash`, `Effect.flashOn`.
- Local visibility: `setVisibleFor(Player)` on TextTag, Image and Ubersplat; `sound:playFor(Player)`; a `player` option
  on `TextTag.float` and `Sound.playOnce`.
- Effect: `setColor`, `setAlpha`, `setPlayerColor`, `setTimeScale`, `setOrientation`, `setHeight`, `setZ`,
  `playAnimation`.
- `Item.enumInRect(rect, filter?)` and `Destructable.enumInRect(rect, filter?)` return snapshots.
- **Changed:** `Effect.attach` accepts any widget (Unit, Item, Destructable). A wrong argument now reports
  `[wrappers] Effect.attach: expected Widget wrapper` (formerly `expected Unit wrapper`).
```

- [ ] **Step 5: Update the wrappers `AGENTS.md`**

In the list of design links, add after the v0.2.0 line:

```markdown
- `../moonwell/docs/superpowers/specs/2026-09-29-moonwell-wrappers-presentation-design.md` and
  `../moonwell/docs/superpowers/plans/2026-09-29-moonwell-wrappers-presentation.md` (v0.3.0)
```

Replace `v0.2.0 adds broad gameplay coverage; UI and presentation types remain in Moonwell's backlog.` with
`v0.2.0 adds broad gameplay coverage; v0.3.0 adds presentation (text tags, sounds, lightning, images, ubersplats, fog
modifiers), deeper effects and item/destructable enumeration. Classic UI and frames remain in Moonwell's backlog.`

Add to "Rules":

```markdown
- Presentation wrappers exist only for objects the map owns and destroys. Anything the game can end on its own (text
  tags with a lifespan, sounds released when done) goes through a helper that returns nothing. Local visibility helpers
  pass a machine-local boolean to the same native on every machine. No getters for machine-local values.
- Options tables go through `internal/options.lua`: unknown keys and wrong types fail before any native.
```

- [ ] **Step 6: Run all wrappers checks and commit**

Run every command in "Running the wrappers checks". Expected: all pass.

```bash
git add examples/gate.yue README.md CHANGELOG.md CONTRIBUTING.md AGENTS.md
git commit -m "docs: presentation wrappers, their in-game gate and changelog"
```

- [ ] **Step 7: Update Moonwell's `AGENTS.md`**

In `C:/Users/mdlsvensson/Repo/moonwell/AGENTS.md`:

After the "Wrappers v0.2.0, broad coverage, released" bullet in "State", add:

```markdown
- **Wrappers v0.3.0, presentation, implemented; in-game gate pending** (spec
  `docs/superpowers/specs/2026-09-29-moonwell-wrappers-presentation-design.md`, plan
  `docs/superpowers/plans/2026-09-29-moonwell-wrappers-presentation.md`): release A of the UI and presentation
  backlog item. TextTag, Sound, Lightning, Image, Ubersplat and FogModifier (owned objects only), `TextTag.float`,
  `Sound.playOnce`, `Effect.flash`/`flashOn`, `setVisibleFor`/`playFor`, deeper Effect and `Item`/`Destructable`
  `enumInRect`. Automated checks pass; the maintainer runs CONTRIBUTING's gate with `presentation = true`.
```

Replace "Next work, in order" item 1 with:

```markdown
1. **Finish wrappers v0.3.0:** the maintainer's in-game gate (CONTRIBUTING step 7 of the wrappers repo, normal and
   minified), including the first-play sound check; then release it like v0.2.0 (tag `v0.3.0`, tag consumption gate).
2. **Then choose the next sub-project with the maintainer:** wrappers release B (classic UI), C (frames), the
   YueScript port of `wc3-lib` (4d) or the Reforged map preview. Each needs a short design or a spec first.
```

In "Backlog", replace the "UI and presentation wrappers" bullet with:

```markdown
- **UI wrappers, releases B and C.** Split 2026-09-29 from "UI and presentation wrappers"; release A (presentation) is
  wrappers v0.3.0. B, v0.4.0: dialog and button, multiboard, leaderboard, quest, timer dialog. C, v0.5.0: the
  `BlzFrame` API, with its own ownership design (TOC/FDF loading, parent trees, local frames).
```

Run Moonwell's checks (AGENTS.md "Checks"); for this docs-only change `deno fmt --check` must pass.

```bash
git add AGENTS.md
git commit -m "docs: record wrappers v0.3.0 implementation state; split the UI backlog"
```
