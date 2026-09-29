# Moonwell Frame Wrappers (wrappers v0.5.0) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the `BlzFrame` API to `moonwell-wrappers` as one `wrappers.frame` module with an owned frame tree,
per-frame event callbacks and TOC loading, plus its in-game probe and gate.

**Architecture:** One annotated Lua 5.3 module, `src/wrappers/frame.lua`, built in three tasks (core and tree, setters,
events). All frame wrappers share one strong `Handle.new` registry; a private state per wrapper records its kind (owned,
template part, borrowed), its owned children and parts, its create context and its event callbacks. Owned frames
dispose their whole subtree on `destroy()` and call `BlzDestroyFrame` once.

**Tech Stack:** Lua 5.3 (game), YueScript 0.34.2 embedded Lua 5.4 test VM, LuaLS 3.19.1, Lua 5.3.6 `luac`, Deno
tooling, Moonwell 0.5.0 consumer fixtures.

**Spec:** `docs/superpowers/specs/2026-09-29-moonwell-wrappers-frames-design.md` (builds on the v0.1.0–v0.4.0 wrapper
specs in the same folder).

## Global Constraints

- Product code, tests and library docs live in `C:/Users/mdlsvensson/Repo/moonwell-wrappers`; this plan and the spec
  stay in Moonwell. The gate map `C:/Users/mdlsvensson/Repo/wrappers-gate` is not under git. Commit on `main` in each
  repository. No tags, no publishing.
- No Node.js, npm packages, `node:` or `npm:` specifiers. Deno tooling uses built-ins and `jsr:@std/*` only.
- Runtime Lua ships only under `src/wrappers/`. Literal `require`s only; no umbrella module; no globals; no native call
  or game-object creation at import time.
- The game's Lua lacks `collectgarbage`, `debug`, `io`, `package`, `dofile`, `loadfile`. Shipped code must not use
  them.
- Additive release: every v0.4.0 call keeps its behavior. `Callback.call` gains varargs (Task 3); its one-argument
  callers are unchanged.
- `frame.lua` imports `wrappers.player` (event callbacks receive Player wrappers) and `wrappers.internal.*` only.
- Every public function carries LuaLS annotations. `fromHandle` stays conservatively nullable. No diagnostic
  suppressions; the direct LuaLS run at Hint level must report no problems (so no unused locals).
- Misuse errors read `[wrappers] Frame.<method>: ...`. Validate receivers, arguments, options and kinds before any
  native that changes state. Assign `local raw = registry.require(...)` before calling a second native in the same
  statement (Lua does not promise argument evaluation order).
- Never iterate a table keyed by tables with `pairs` when the loop calls natives. Use arrays.
- No machine-local getters: no `getText`, `getValue`, `isVisible`, `isEnabled`, `getAlpha`, `getWidth`, `getHeight`,
  `getTextSizeLimit`; no `setFocus`, `click`, `cageMouse` or pixel conversions.
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
Remove-Item -Recurse -Force .test-work/luals-src/src
Copy-Item -Recurse src .test-work/luals-src/src
& $env:MOONWELL_LUALS --check=.test-work/luals-src --checklevel=Hint
```

Expected: all pass, and the last command prints `no problems found`. (`.test-work/luals-src` holds `types/` with the
native declarations and a `.luarc.json`; the v0.4.0 plan in this folder says how to recreate it if missing.)
`deno task test <suite>` runs one suite (`tests/<suite>.lua`; lowercase letters only; discovered automatically).

Test helpers in `tests/support.lua`: `native(name, fn)`, `eq`, `fails(fn, fragment)`, `expectCall(name, ...)` (the most
recent call to `name`, exact arguments), `callCount`, `totalCalls`, `callName(index)`, `resetCalls`,
`checkSetters(wrapper, rows)` (each row `{native, method, args...}`; the native receives the handle then the args; rows
with booleans run again flipped; defines each native as a no-op), `checkGetters`, `checkDisposed(wrapper, methods)`
(each method called with no arguments must raise `disposed` and call no native). `PLAYER_RAW` is what every `Player(i)`
double returns; `Player.fromHandle({})` is "another player". Each `test` resets the call log and `PRINTED`.

## Review focus

1. The tree: kinds are right; `destroy()` disposes exactly the owned subtree and its parts, removes the frame from its
   owned parent, and calls BlzDestroyFrame once; borrowed frames and parts can never be destroyed; re-parenting never
   puts a borrowed frame under an owned one and never makes a cycle (Task 1).
2. Nothing validates late (every task).
3. Events: a disposed frame's callbacks never run, `off` works during a firing, callbacks added during a firing wait for
   the next one (Task 3).
4. Local visibility and focus change only that player's machine (Task 2).

## Files and interfaces

| File                                                        | Responsibility                                    |
| ----------------------------------------------------------- | ------------------------------------------------- |
| `src/wrappers/frame.lua`                                    | Frame (spec §3–§7)                                |
| `src/wrappers/internal/callback.lua`                        | `Callback.call(label, fn, ...)` with varargs      |
| `tests/frame.lua`, `tests/framestyle.lua`, `tests/frameevents.lua` | New suites                                 |
| `tests/imports.lua`, `tools/integration.ts`, `tests/editor-*.{lua,yue}` | Import graph, bundle and editor coverage |
| `examples/gate.yue`, `README.md`, `CHANGELOG.md`, `CONTRIBUTING.md`, `AGENTS.md` | Gate example and library docs  |
| Moonwell `AGENTS.md`                                        | State, next work and backlog                      |
| `../wrappers-gate/gate.ts`, `assets/war3mapImported/wrappers-gate.toc`, `src/probe_frame.yue`, `PROBE-FRAME.md` | Gate runs and probe (not under git) |

Interfaces produced (all in `wrappers.frame`):

- Task 1: `Frame.create(template, parent, options?)`, `Frame.createSimple(template, parent)`,
  `Frame.createByType(frameType, parent, options?)`, `Frame.origin(originType, index?)`, `Frame.byName(name, context?)`,
  `Frame.fromHandle(raw)`, `Frame.loadTOC(path)`, `Frame.hideOrigin(flag)`, `Frame.enableAutoPosition(flag)`; methods
  `getHandle`, `isDisposed`, `getName`, `getParent`, `getChildrenCount`, `getChild(index)`, `findChild(name)`,
  `setParent(parent)`, `destroy()`. Private: `states[frame]` (`MoonwellWrappers.FrameState`), `releaseEvents(state)`.
- Task 2: `setPoint`, `setAbsPoint`, `setAllPoints`, `clearPoints`, `setSize`, `setScale`, `setLevel`, `setText`,
  `addText`, `setTextColor`, `setVertexColor`, `setFont`, `setTextAlignment`, `setTextSizeLimit`, `setTexture`,
  `setModel`, `setSpriteAnimate`, `setAutoScroll`, `setValue`, `setMinMaxValue`, `setStepSize`, `setAlpha`,
  `setEnabled`, `setTooltip`, `show`, `setVisibleFor(player)`, `releaseFocusFor(player)`.
- Task 3: `on(eventType, callback) -> MoonwellWrappers.FrameHandler`, `off(token)`; types `FrameHandler`,
  `FrameEvent`, `FrameCallback`.

Error messages (exact fragments the tests check): `expected Frame wrapper`, `Frame is disposed`,
`native returned nil`, `Frame.origin: no frame`, `Frame.byName: no frame named <name>`,
`Frame.getChild: no child <index>`, `Frame.findChild: no frame named <name> in this frame`,
`Frame.findChild: findChild needs a frame made by Frame.create*`,
`Frame.destroy: only frames made by Frame.create, createSimple or createByType can be destroyed`,
`Frame.setParent: a template part cannot be re-parented`,
`Frame.setParent: a borrowed frame can only be re-parented to a borrowed frame`,
`Frame.setParent: a frame cannot be re-parented into its own subtree`, `Frame.loadTOC: could not load <path>`,
`Frame.on: expected a frame event type`, `Frame.on: expected a callback function`,
`Frame.off: expected FrameHandler token`, `Frame.off: token belongs to another frame`.

Ruling recorded here: the spec does not mention cycles; `setParent` onto the frame itself, its descendants or its parts
raises (`into its own subtree`), because the tree would otherwise recurse forever on `destroy()`.

---

### Task 1: Frame core and tree

**Files:**
- Create: `src/wrappers/frame.lua`
- Test: `tests/frame.lua`

**Interfaces:**
- Consumes: `Handle.new`, `Handle.created`, `Options.read` (internal).
- Produces: the Task 1 interface above; the file ends with `return Frame`, and Tasks 2 and 3 insert code before that
  line.

- [ ] **Step 1: Write the failing test**

Create `tests/frame.lua`:

```lua
ORIGIN_FRAME_GAME_UI = {}
local byName = {}
local function newFrame(name, parent, context)
    local raw = {name = name, parent = parent, children = {}, context = context}
    if parent then parent.children[#parent.children + 1] = raw end
    byName[name .. '#' .. context] = raw
    return raw
end
-- The 'Panel' template has two parts, found by name with the panel's create context.
local function createFrame(template, parent, _, context)
    local raw = newFrame(template, parent, context)
    if template == 'Panel' then
        newFrame('PanelTitle', raw, context)
        newFrame('PanelClose', raw, context)
    end
    return raw
end
local ORIGIN = newFrame('GameUI', nil, 0)
native('BlzGetOriginFrame', function(_, index) if index == 0 then return ORIGIN end end)
native('BlzCreateFrame', createFrame)
native('BlzCreateSimpleFrame', function(template, parent, context) return newFrame(template, parent, context) end)
native('BlzCreateFrameByType', function(_, name, parent, _, context) return newFrame(name, parent, context) end)
native('BlzGetFrameByName', function(name, context) return byName[name .. '#' .. context] end)
native('BlzFrameGetName', function(raw) return raw.name end)
native('BlzFrameGetParent', function(raw) return raw.parent end)
native('BlzFrameGetChildrenCount', function(raw) return #raw.children end)
native('BlzFrameGetChild', function(raw, index) return raw.children[index + 1] end)
native('BlzLoadTOCFile', function(path) return path == 'ok.toc' end)
for _, name in ipairs({'BlzFrameSetParent', 'BlzDestroyFrame', 'BlzHideOriginFrames', 'BlzEnableUIAutoPosition',
    'DestroyTrigger'}) do
    native(name, function() end)
end
local Frame = require('wrappers.frame')
eq(totalCalls(), 0)

test('factories pass the parent, options and a fresh create context', function()
    local ui = Frame.origin(ORIGIN_FRAME_GAME_UI)
    expectCall('BlzGetOriginFrame', ORIGIN_FRAME_GAME_UI, 0)
    eq(ui.handle, ORIGIN); eq(Frame.origin(ORIGIN_FRAME_GAME_UI, 0), ui)
    local a = Frame.create('Panel', ui)
    local context = a.handle.context
    expectCall('BlzCreateFrame', 'Panel', ORIGIN, 0, context)
    local b = Frame.create('Panel', a, {priority = 2})
    expectCall('BlzCreateFrame', 'Panel', a.handle, 2, context + 1)
    local c = Frame.createSimple('Bar', ui)
    expectCall('BlzCreateSimpleFrame', 'Bar', ORIGIN, context + 2)
    local d = Frame.createByType('BUTTON', b, {name = 'Ok', inherits = 'ScriptDialogButton'})
    expectCall('BlzCreateFrameByType', 'BUTTON', 'Ok', b.handle, 'ScriptDialogButton', context + 3)
    local e = Frame.createByType('BACKDROP', ui)
    expectCall('BlzCreateFrameByType', 'BACKDROP', '', ORIGIN, '', context + 4)
    eq(Frame.fromHandle(d.handle), d); eq(Frame.fromHandle(nil), nil)
    eq(d:getHandle(), d.handle); eq(d:isDisposed(), false)
    a:destroy(); c:destroy(); e:destroy()
    eq(d:isDisposed(), true)
end)

test('factories check the parent and options first and raise on nil', function()
    local ui = Frame.origin(ORIGIN_FRAME_GAME_UI)
    resetCalls()
    fails(function() Frame.create('Panel', nil) end, 'Frame.create: expected Frame wrapper')
    fails(function() Frame.createSimple('Bar', {}) end, 'Frame.createSimple: expected Frame wrapper')
    fails(function() Frame.createByType('BUTTON', ui, {inherit = 'x'}) end, "Frame.createByType: unknown option 'inherit'")
    fails(function() Frame.create('Panel', ui, {priority = 1.5}) end, "option 'priority' expected an integer")
    eq(totalCalls(), 0)
    native('BlzCreateFrame', function() return nil end)
    fails(function() Frame.create('Missing', ui) end, 'Frame.create: native returned nil')
    native('BlzCreateFrame', createFrame)
end)

test('origin, byName and fromHandle give borrowed frames that cannot be destroyed', function()
    fails(function() Frame.origin(ORIGIN_FRAME_GAME_UI, 3) end, 'Frame.origin: no frame')
    local ui = Frame.origin(ORIGIN_FRAME_GAME_UI)
    eq(Frame.byName('GameUI'), ui); expectCall('BlzGetFrameByName', 'GameUI', 0)
    fails(function() Frame.byName('Nothing', 2) end, 'Frame.byName: no frame named Nothing')
    resetCalls()
    fails(function() ui:destroy() end,
        'Frame.destroy: only frames made by Frame.create, createSimple or createByType can be destroyed')
    eq(totalCalls(), 0); eq(ui:isDisposed(), false)
    local panel = Frame.create('Panel', ui)
    eq(Frame.byName('Panel', panel.handle.context), panel)
    panel:destroy()
    eq(panel:isDisposed(), true)
end)

test('structure: names, parents, children and template parts', function()
    local ui = Frame.origin(ORIGIN_FRAME_GAME_UI)
    local panel = Frame.create('Panel', ui)
    eq(panel:getName(), 'Panel'); eq(panel:getParent(), ui); eq(ui:getParent(), nil)
    eq(panel:getChildrenCount(), 2)
    local title = panel:getChild(0)
    eq(title:getName(), 'PanelTitle'); eq(panel:findChild('PanelTitle'), title)
    expectCall('BlzGetFrameByName', 'PanelTitle', panel.handle.context)
    eq(title:getParent(), panel)
    local close = title:findChild('PanelClose')
    eq(close:getName(), 'PanelClose'); eq(panel:getChild(1), close)
    fails(function() panel:getChild(5) end, 'Frame.getChild: no child 5')
    fails(function() panel:findChild('Nope') end, 'Frame.findChild: no frame named Nope in this frame')
    fails(function() ui:findChild('PanelTitle') end, 'Frame.findChild: findChild needs a frame made by Frame.create*')
    resetCalls()
    fails(function() title:destroy() end, 'only frames made by Frame.create')
    eq(totalCalls(), 0)
    local root = newFrame('Root', nil, 0)
    newFrame('RootChild', root, 0)
    local rootFrame = Frame.fromHandle(root)
    local rootChild = rootFrame:getChild(0)
    eq(rootChild:getParent(), rootFrame)
    fails(function() rootChild:destroy() end, 'only frames made by Frame.create')
    panel:destroy()
    eq(title:isDisposed(), true); eq(close:isDisposed(), true)
end)

test('destroy disposes owned descendants and template parts, then destroys once', function()
    local ui = Frame.origin(ORIGIN_FRAME_GAME_UI)
    local panel = Frame.create('Panel', ui)
    local title = panel:findChild('PanelTitle')
    local button = Frame.createByType('BUTTON', title)
    local inner = Frame.create('Panel', button)
    local innerTitle = inner:findChild('PanelTitle')
    local other = Frame.create('Panel', ui)
    resetCalls()
    local raw = panel.handle
    panel:destroy(); panel:destroy()
    eq(callCount('BlzDestroyFrame'), 1); expectCall('BlzDestroyFrame', raw)
    for _, frame in ipairs({panel, title, button, inner, innerTitle}) do eq(frame:isDisposed(), true) end
    eq(panel.handle, nil); eq(other:isDisposed(), false)
    checkDisposed(button, {'getHandle', 'getName', 'getParent', 'getChildrenCount', 'getChild', 'findChild',
        'setParent'})
    button:destroy()
    eq(callCount('BlzDestroyFrame'), 1)
    local child = Frame.create('Panel', other)
    child:destroy()
    other:destroy()
    eq(callCount('BlzDestroyFrame'), 3)
end)

test('setParent moves owned frames and refuses parts, borrowed-under-owned and cycles', function()
    local ui = Frame.origin(ORIGIN_FRAME_GAME_UI)
    local a, b = Frame.create('Panel', ui), Frame.create('Panel', ui)
    local c = Frame.create('Panel', a)
    c:setParent(b); expectCall('BlzFrameSetParent', c.handle, b.handle)
    a:destroy(); eq(c:isDisposed(), false)
    b:destroy(); eq(c:isDisposed(), true)
    local d, e = Frame.create('Panel', ui), Frame.create('Panel', ui)
    local f = Frame.create('Panel', d)
    f:setParent(ui)
    d:destroy(); eq(f:isDisposed(), false)
    local title = e:findChild('PanelTitle')
    local g = Frame.create('Panel', e)
    resetCalls()
    fails(function() title:setParent(ui) end, 'Frame.setParent: a template part cannot be re-parented')
    fails(function() ui:setParent(e) end, 'Frame.setParent: a borrowed frame can only be re-parented to a borrowed frame')
    fails(function() e:setParent(e) end, 'Frame.setParent: a frame cannot be re-parented into its own subtree')
    fails(function() e:setParent(g) end, 'into its own subtree')
    fails(function() e:setParent(title) end, 'into its own subtree')
    fails(function() e:setParent(nil) end, 'Frame.setParent: expected Frame wrapper')
    eq(totalCalls(), 0)
    local loose = Frame.fromHandle(newFrame('Loose', nil, 0))
    loose:setParent(ui); expectCall('BlzFrameSetParent', loose.handle, ui.handle)
    e:destroy(); f:destroy()
    eq(g:isDisposed(), true)
end)

test('statics load TOC files and toggle the game UI', function()
    Frame.loadTOC('ok.toc'); expectCall('BlzLoadTOCFile', 'ok.toc')
    fails(function() Frame.loadTOC('missing.toc') end, 'Frame.loadTOC: could not load missing.toc')
    Frame.hideOrigin(true); expectCall('BlzHideOriginFrames', true)
    Frame.enableAutoPosition(false); expectCall('BlzEnableUIAutoPosition', false)
end)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `deno task test frame`
Expected: FAIL, `module 'wrappers.frame' not found`.

- [ ] **Step 3: Write the implementation**

Create `src/wrappers/frame.lua`:

```lua
local Handle = require('wrappers.internal.handle')
local Options = require('wrappers.internal.options')

---A Blz frame. Owned frames (made by Frame.create, createSimple or createByType) can be destroyed, which disposes
---their whole subtree. Template parts (findChild, getChild) belong to the owned frame they were found through.
---Borrowed frames are the game's and are never destroyed through a wrapper.
---@class MoonwellWrappers.Frame
---@field handle framehandle? Read-only by convention; nil after disposal.
local Frame = {}
---@type MoonwellWrappers.Registry<MoonwellWrappers.Frame, framehandle>
local registry = Handle.new(Frame, 'Frame')

---@class MoonwellWrappers.FrameCreateOptions
---@field priority integer? Default 0.

---@class MoonwellWrappers.FrameByTypeOptions
---@field name string? The frame's name, for Frame.byName and findChild; default "".
---@field inherits string? A template to inherit from; default "".

---@type MoonwellWrappers.OptionFields
local createFields = {priority = {'integer', 0}}
---@type MoonwellWrappers.OptionFields
local byTypeFields = {name = {'string', ''}, inherits = {'string', ''}}

---@class MoonwellWrappers.FrameCell
---@field frame MoonwellWrappers.Frame
---@field eventType frameeventtype
---@field callback function? Nil once removed or disposed.

---@class MoonwellWrappers.FrameState
---@field kind 'owned'|'part'|'borrowed'
---@field owner MoonwellWrappers.Frame A part's owned frame; an owned or borrowed frame's owner is itself.
---@field parent MoonwellWrappers.Frame? An owned frame's owned parent; nil for a root.
---@field context integer An owned frame's create context; a part's is its owner's; 0 when borrowed.
---@field children MoonwellWrappers.Frame[] Owned frames under an owned frame, in creation order.
---@field parts MoonwellWrappers.Frame[] Template parts found through an owned frame, in the order found.
---@field trigger trigger? Created by the first on().
---@field cells MoonwellWrappers.FrameCell[] Live event callbacks, in the order added.
---@field byType table<frameeventtype, MoonwellWrappers.FrameCell[]> Callbacks by event type; lookup only.

-- Keyed by wrapper; only indexed, never iterated.
---@type table<MoonwellWrappers.Frame, MoonwellWrappers.FrameState>
local states = {}
-- Every machine creates frames in the same order, so this counter agrees across machines.
local lastContext = 0

---@return integer
local function nextContext()
    lastContext = lastContext + 1
    return lastContext
end

---@param list any[]
---@param item any
---@return any[]
local function without(list, item)
    local result = {}
    for _, value in ipairs(list) do
        if value ~= item then result[#result + 1] = value end
    end
    return result
end

---@param kind 'owned'|'part'|'borrowed'
---@param owner MoonwellWrappers.Frame
---@param context integer
---@return MoonwellWrappers.FrameState
local function newState(kind, owner, context)
    return {kind = kind, owner = owner, context = context, children = {}, parts = {}, cells = {}, byType = {}}
end

---Wraps a raw handle the game gave us. A known frame keeps its wrapper and kind; a new one becomes a part of `owner`,
---or borrowed when `owner` is nil.
---@param raw framehandle
---@param owner MoonwellWrappers.Frame?
---@return MoonwellWrappers.Frame
local function adopt(raw, owner)
    local frame = assert(registry.wrap(raw))
    if not states[frame] then
        if owner then
            states[frame] = newState('part', owner, states[owner].context)
            local parts = states[owner].parts
            parts[#parts + 1] = frame
        else
            states[frame] = newState('borrowed', frame, 0)
        end
    end
    return frame
end

---The owned frame whose destruction also destroys a frame parented to `parent`; nil for a borrowed parent.
---@param parent MoonwellWrappers.Frame
---@return MoonwellWrappers.Frame?
local function ownerOf(parent)
    local state = states[parent]
    if state.kind == 'borrowed' then return nil end
    return state.owner
end

---Records a frame a factory just made as owned, under the owned frame that `parent` belongs to.
---@param frame MoonwellWrappers.Frame
---@param parent MoonwellWrappers.Frame
---@param context integer
---@return MoonwellWrappers.Frame
local function own(frame, parent, context)
    local state = newState('owned', frame, context)
    state.parent = ownerOf(parent)
    states[frame] = state
    if state.parent then
        local children = states[state.parent].children
        children[#children + 1] = frame
    end
    return frame
end

---Clears a frame's callbacks, then destroys its internal trigger.
---@param state MoonwellWrappers.FrameState
local function releaseEvents(state)
    for _, cell in ipairs(state.cells) do cell.callback = nil end
    state.cells, state.byType = {}, {}
    if state.trigger then DestroyTrigger(state.trigger) end
end

---Disposes a frame's wrapper and its owned descendants and parts, depth-first. Never calls BlzDestroyFrame.
---@param frame MoonwellWrappers.Frame
local function dispose(frame)
    local state = states[frame]
    states[frame] = nil
    for _, child in ipairs(state.children) do dispose(child) end
    for _, part in ipairs(state.parts) do dispose(part) end
    releaseEvents(state)
    registry.dispose(frame, 'Frame.destroy')
end

---@param raw framehandle?
---@return MoonwellWrappers.Frame?
---@overload fun(raw: nil): nil
function Frame.fromHandle(raw)
    if raw == nil then return nil end
    return adopt(raw, nil)
end
---Creates a frame from a template the game knows (built in, or loaded with Frame.loadTOC). Create frames on every
---machine in the same order, never inside a branch on the local player.
---@param template string
---@param parent MoonwellWrappers.Frame
---@param options MoonwellWrappers.FrameCreateOptions?
---@return MoonwellWrappers.Frame
function Frame.create(template, parent, options)
    local parentRaw = registry.require(parent, 'Frame.create')
    local o = Options.read(options, createFields, 'Frame.create')
    local context = nextContext()
    local raw = BlzCreateFrame(template, parentRaw, o.priority, context)
    local frame = Handle.created(registry.wrap(raw), 'Frame.create')
    return own(frame, parent, context)
end
---@param template string
---@param parent MoonwellWrappers.Frame
---@return MoonwellWrappers.Frame
function Frame.createSimple(template, parent)
    local parentRaw = registry.require(parent, 'Frame.createSimple')
    local context = nextContext()
    local frame = Handle.created(registry.wrap(BlzCreateSimpleFrame(template, parentRaw, context)),
        'Frame.createSimple')
    return own(frame, parent, context)
end
---Creates a frame of a type such as "BACKDROP", "TEXT" or "GLUETEXTBUTTON", optionally inheriting a template.
---@param frameType string
---@param parent MoonwellWrappers.Frame
---@param options MoonwellWrappers.FrameByTypeOptions?
---@return MoonwellWrappers.Frame
function Frame.createByType(frameType, parent, options)
    local parentRaw = registry.require(parent, 'Frame.createByType')
    local o = Options.read(options, byTypeFields, 'Frame.createByType')
    local context = nextContext()
    local raw = BlzCreateFrameByType(frameType, o.name, parentRaw, o.inherits, context)
    local frame = Handle.created(registry.wrap(raw), 'Frame.createByType')
    return own(frame, parent, context)
end
---A game frame such as ORIGIN_FRAME_GAME_UI. Borrowed: never destroyed through the wrapper.
---@param originType originframetype
---@param index integer? Default 0.
---@return MoonwellWrappers.Frame
function Frame.origin(originType, index)
    local raw = BlzGetOriginFrame(originType, index or 0)
    if raw == nil then error('[wrappers] Frame.origin: no frame', 2) end
    return adopt(raw, nil)
end
---The frame with that name and create context: the existing wrapper, or a borrowed one.
---@param name string
---@param context integer? Default 0.
---@return MoonwellWrappers.Frame
function Frame.byName(name, context)
    local raw = BlzGetFrameByName(name, context or 0)
    if raw == nil then error('[wrappers] Frame.byName: no frame named ' .. tostring(name), 2) end
    return adopt(raw, nil)
end
---Loads a .toc file that lists .fdf files, so their templates can be created.
---@param path string In-map path, for example "war3mapImported\\templates.toc".
function Frame.loadTOC(path)
    if not BlzLoadTOCFile(path) then error('[wrappers] Frame.loadTOC: could not load ' .. tostring(path), 2) end
end
---Hides (true) or shows (false) the game's own UI, for everyone.
---@param flag boolean
function Frame.hideOrigin(flag) BlzHideOriginFrames(flag) end
---@param flag boolean
function Frame.enableAutoPosition(flag) BlzEnableUIAutoPosition(flag) end

---@return framehandle
function Frame:getHandle() return registry.require(self, 'Frame.getHandle') end
---@return boolean
function Frame:isDisposed() return registry.isDisposed(self, 'Frame.isDisposed') end
---@return string
function Frame:getName() return BlzFrameGetName(registry.require(self, 'Frame.getName')) end
---@return MoonwellWrappers.Frame?
function Frame:getParent()
    local raw = BlzFrameGetParent(registry.require(self, 'Frame.getParent'))
    if raw == nil then return nil end
    local state = states[self]
    if state.kind == 'part' then return adopt(raw, state.owner) end
    return adopt(raw, nil)
end
---@return integer
function Frame:getChildrenCount() return BlzFrameGetChildrenCount(registry.require(self, 'Frame.getChildrenCount')) end
---The child at a zero-based index. Under an owned frame or part it is a template part (or an owned frame made there).
---@param index integer
---@return MoonwellWrappers.Frame
function Frame:getChild(index)
    local raw = BlzFrameGetChild(registry.require(self, 'Frame.getChild'), index)
    if raw == nil then error('[wrappers] Frame.getChild: no child ' .. tostring(index), 2) end
    local state = states[self]
    if state.kind == 'borrowed' then return adopt(raw, nil) end
    return adopt(raw, state.owner)
end
---Finds a template part by name, with the create context of the owned frame this frame belongs to.
---@param name string
---@return MoonwellWrappers.Frame
function Frame:findChild(name)
    registry.require(self, 'Frame.findChild')
    local state = states[self]
    if state.kind == 'borrowed' then
        error('[wrappers] Frame.findChild: findChild needs a frame made by Frame.create*', 2)
    end
    local raw = BlzGetFrameByName(name, states[state.owner].context)
    if raw == nil then
        error('[wrappers] Frame.findChild: no frame named ' .. tostring(name) .. ' in this frame', 2)
    end
    return adopt(raw, state.owner)
end
---Moves the frame under another frame. An owned frame moves in the tree; a borrowed frame may only move under another
---borrowed frame; a template part cannot move.
---@param parent MoonwellWrappers.Frame
function Frame:setParent(parent)
    local raw = registry.require(self, 'Frame.setParent')
    local parentRaw = registry.require(parent, 'Frame.setParent')
    local state = states[self]
    local up = ownerOf(parent)
    if state.kind == 'part' then error('[wrappers] Frame.setParent: a template part cannot be re-parented', 2) end
    if state.kind == 'borrowed' then
        if up ~= nil then
            error('[wrappers] Frame.setParent: a borrowed frame can only be re-parented to a borrowed frame', 2)
        end
    else
        local walk = up
        while walk ~= nil do
            if walk == self then
                error('[wrappers] Frame.setParent: a frame cannot be re-parented into its own subtree', 2)
            end
            walk = states[walk].parent
        end
        if state.parent then
            local old = states[state.parent]
            old.children = without(old.children, self)
        end
        state.parent = up
        if up then
            local children = states[up].children
            children[#children + 1] = self
        end
    end
    BlzFrameSetParent(raw, parentRaw)
end
---Destroys an owned frame and everything under it: every wrapper in its subtree is disposed and their callbacks never
---run again. Borrowed frames and template parts raise.
function Frame:destroy()
    if registry.isDisposed(self, 'Frame.destroy') then return end
    local raw = registry.require(self, 'Frame.destroy')
    local state = states[self]
    if state.kind ~= 'owned' then
        error('[wrappers] Frame.destroy: only frames made by Frame.create, createSimple or createByType can be destroyed',
            2)
    end
    if state.parent then
        local parent = states[state.parent]
        parent.children = without(parent.children, self)
    end
    dispose(self)
    BlzDestroyFrame(raw)
end

return Frame
```

- [ ] **Step 4: Run test to verify it passes**

Run: `deno task test frame`
Expected: `frame: SUITE PASSED: 7 tests`.

- [ ] **Step 5: Run all checks and commit**

Run every command in "Running the wrappers checks". Expected: all pass, LuaLS `no problems found`. If LuaLS flags a
line (for example the `'owned'|'part'|'borrowed'` literal type or `assert(registry.wrap(raw))`), fix the annotation,
not the behavior.

```bash
git add src/wrappers/frame.lua tests/frame.lua
git commit -m "feat: Frame with an owned tree, template parts and borrowed game frames"
```

---

### Task 2: Frame setters, local visibility and focus

**Files:**
- Modify: `src/wrappers/frame.lua` (insert before the final `return Frame`)
- Test: `tests/framestyle.lua`

**Interfaces:**
- Consumes: `registry`, `Handle.unwrap` in `frame.lua`.
- Produces: the Task 2 interface above.

- [ ] **Step 1: Write the failing test**

Create `tests/framestyle.lua`:

```lua
ORIGIN_FRAME_GAME_UI = {}
local ORIGIN = {}
native('BlzGetOriginFrame', function() return ORIGIN end)
native('BlzCreateFrameByType', function() return {} end)
native('BlzConvertColor', function(a, r, g, b) return ((a * 256 + r) * 256 + g) * 256 + b end)
native('GetLocalPlayer', function() return PLAYER_RAW end)
for _, name in ipairs({'BlzDestroyFrame', 'DestroyTrigger'}) do native(name, function() end) end
local Frame = require('wrappers.frame')
local Player = require('wrappers.player')
eq(totalCalls(), 0)

local setters = {{'BlzFrameSetAbsPoint', 'setAbsPoint', 'center', 0.4, 0.3}, {'BlzFrameClearAllPoints', 'clearPoints'},
    {'BlzFrameSetSize', 'setSize', 0.1, 0.05}, {'BlzFrameSetScale', 'setScale', 1.5}, {'BlzFrameSetLevel', 'setLevel', 2},
    {'BlzFrameSetText', 'setText', 'Hi'}, {'BlzFrameAddText', 'addText', 'more'},
    {'BlzFrameSetTextAlignment', 'setTextAlignment', 'top', 'left'},
    {'BlzFrameSetTextSizeLimit', 'setTextSizeLimit', 12}, {'BlzFrameSetSpriteAnimate', 'setSpriteAnimate', 1, 0},
    {'BlzTextAreaFrameSetAutoScroll', 'setAutoScroll', true}, {'BlzFrameSetValue', 'setValue', 3},
    {'BlzFrameSetMinMaxValue', 'setMinMaxValue', 0, 10}, {'BlzFrameSetStepSize', 'setStepSize', 1},
    {'BlzFrameSetAlpha', 'setAlpha', 128}, {'BlzFrameSetEnable', 'setEnabled', false},
    {'BlzFrameSetVisible', 'show', true}}

test('setters forward exact arguments on owned and borrowed frames', function()
    local ui = Frame.origin(ORIGIN_FRAME_GAME_UI)
    local frame = Frame.createByType('TEXT', ui)
    checkSetters(frame, setters)
    checkSetters(ui, {{'BlzFrameSetVisible', 'show', false}, {'BlzFrameSetAlpha', 'setAlpha', 200}})
    frame:destroy()
end)

test('colors convert with BlzConvertColor and defaults fill optional arguments', function()
    local frame = Frame.createByType('TEXT', Frame.origin(ORIGIN_FRAME_GAME_UI))
    for _, name in ipairs({'BlzFrameSetTextColor', 'BlzFrameSetVertexColor', 'BlzFrameSetFont', 'BlzFrameSetTexture',
        'BlzFrameSetModel'}) do
        native(name, function() end)
    end
    frame:setTextColor(255, 204, 0, 128)
    expectCall('BlzConvertColor', 128, 255, 204, 0)
    expectCall('BlzFrameSetTextColor', frame.handle, ((128 * 256 + 255) * 256 + 204) * 256 + 0)
    frame:setVertexColor(1, 2, 3, 4)
    expectCall('BlzConvertColor', 4, 1, 2, 3)
    expectCall('BlzFrameSetVertexColor', frame.handle, ((4 * 256 + 1) * 256 + 2) * 256 + 3)
    frame:setFont('font.ttf', 0.012); expectCall('BlzFrameSetFont', frame.handle, 'font.ttf', 0.012, 0)
    frame:setFont('font.ttf', 0.012, 1); expectCall('BlzFrameSetFont', frame.handle, 'font.ttf', 0.012, 1)
    frame:setTexture('icon.blp'); expectCall('BlzFrameSetTexture', frame.handle, 'icon.blp', 0, true)
    frame:setTexture('icon.blp', 1, false); expectCall('BlzFrameSetTexture', frame.handle, 'icon.blp', 1, false)
    frame:setModel('model.mdx'); expectCall('BlzFrameSetModel', frame.handle, 'model.mdx', 0)
    frame:setModel('model.mdx', 2); expectCall('BlzFrameSetModel', frame.handle, 'model.mdx', 2)
    frame:destroy()
end)

test('frame arguments are unwrapped and checked first', function()
    local ui = Frame.origin(ORIGIN_FRAME_GAME_UI)
    local frame, tip = Frame.createByType('TEXT', ui), Frame.createByType('TEXT', ui)
    for _, name in ipairs({'BlzFrameSetPoint', 'BlzFrameSetAllPoints', 'BlzFrameSetTooltip'}) do
        native(name, function() end)
    end
    frame:setPoint('top', ui, 'bottom', 0, -0.01)
    expectCall('BlzFrameSetPoint', frame.handle, 'top', ui.handle, 'bottom', 0, -0.01)
    frame:setAllPoints(ui); expectCall('BlzFrameSetAllPoints', frame.handle, ui.handle)
    frame:setTooltip(tip); expectCall('BlzFrameSetTooltip', frame.handle, tip.handle)
    resetCalls()
    fails(function() frame:setPoint('top', {}, 'bottom', 0, 0) end, 'Frame.setPoint: expected Frame wrapper')
    fails(function() frame:setAllPoints(nil) end, 'Frame.setAllPoints: expected Frame wrapper')
    fails(function() frame:setTooltip(Player.fromIndex(0)) end, 'Frame.setTooltip: expected Frame wrapper')
    tip:destroy()
    fails(function() frame:setTooltip(tip) end, 'Frame.setTooltip: Frame is disposed')
    eq(callCount('BlzFrameSetPoint') + callCount('BlzFrameSetAllPoints') + callCount('BlzFrameSetTooltip'), 0)
    frame:destroy()
end)

test('setVisibleFor and releaseFocusFor act on that player machine only', function()
    local frame = Frame.createByType('BUTTON', Frame.origin(ORIGIN_FRAME_GAME_UI))
    local enabled = {}
    native('BlzFrameSetVisible', function() end)
    native('BlzFrameSetEnable', function(_, flag) enabled[#enabled + 1] = tostring(flag) end)
    frame:setVisibleFor(Player.fromIndex(0)); expectCall('BlzFrameSetVisible', frame.handle, true)
    frame:setVisibleFor(Player.fromHandle({})); expectCall('BlzFrameSetVisible', frame.handle, false)
    frame:releaseFocusFor(Player.fromHandle({})); eq(#enabled, 0)
    frame:releaseFocusFor(Player.fromIndex(0)); eq(table.concat(enabled, ','), 'false,true')
    expectCall('BlzFrameSetEnable', frame.handle, true)
    fails(function() frame:setVisibleFor(frame) end, 'Frame.setVisibleFor: expected Player wrapper')
    fails(function() frame:releaseFocusFor(nil) end, 'Frame.releaseFocusFor: expected Player wrapper')
    frame:destroy()
end)

test('disposed frames guard every setter', function()
    local frame = Frame.createByType('TEXT', Frame.origin(ORIGIN_FRAME_GAME_UI))
    frame:destroy()
    checkDisposed(frame, {'setPoint', 'setAbsPoint', 'setAllPoints', 'clearPoints', 'setSize', 'setScale', 'setLevel',
        'setText', 'addText', 'setTextColor', 'setVertexColor', 'setFont', 'setTextAlignment', 'setTextSizeLimit',
        'setTexture', 'setModel', 'setSpriteAnimate', 'setAutoScroll', 'setValue', 'setMinMaxValue', 'setStepSize',
        'setAlpha', 'setEnabled', 'setTooltip', 'show', 'setVisibleFor', 'releaseFocusFor'})
end)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `deno task test framestyle`
Expected: FAIL (for example `attempt to call a nil value (field 'setAbsPoint')`).

- [ ] **Step 3: Write the implementation**

In `src/wrappers/frame.lua`, insert before the final `return Frame`:

```lua
---@param point framepointtype
---@param relative MoonwellWrappers.Frame
---@param relativePoint framepointtype
---@param x number
---@param y number
function Frame:setPoint(point, relative, relativePoint, x, y)
    local raw = registry.require(self, 'Frame.setPoint')
    BlzFrameSetPoint(raw, point, registry.require(relative, 'Frame.setPoint'), relativePoint, x, y)
end
---Places a point of the frame at screen coordinates: 0-0.8 wide, 0-0.6 high, origin bottom left.
---@param point framepointtype
---@param x number
---@param y number
function Frame:setAbsPoint(point, x, y) BlzFrameSetAbsPoint(registry.require(self, 'Frame.setAbsPoint'), point, x, y) end
---@param relative MoonwellWrappers.Frame
function Frame:setAllPoints(relative)
    local raw = registry.require(self, 'Frame.setAllPoints')
    BlzFrameSetAllPoints(raw, registry.require(relative, 'Frame.setAllPoints'))
end
function Frame:clearPoints() BlzFrameClearAllPoints(registry.require(self, 'Frame.clearPoints')) end
---@param width number
---@param height number
function Frame:setSize(width, height) BlzFrameSetSize(registry.require(self, 'Frame.setSize'), width, height) end
---@param scale number
function Frame:setScale(scale) BlzFrameSetScale(registry.require(self, 'Frame.setScale'), scale) end
---@param level integer
function Frame:setLevel(level) BlzFrameSetLevel(registry.require(self, 'Frame.setLevel'), level) end
---@param text string
function Frame:setText(text) BlzFrameSetText(registry.require(self, 'Frame.setText'), text) end
---@param text string
function Frame:addText(text) BlzFrameAddText(registry.require(self, 'Frame.addText'), text) end
---@param r integer 0-255
---@param g integer 0-255
---@param b integer 0-255
---@param a integer 0-255
function Frame:setTextColor(r, g, b, a)
    local raw = registry.require(self, 'Frame.setTextColor')
    BlzFrameSetTextColor(raw, BlzConvertColor(a, r, g, b))
end
---@param r integer 0-255
---@param g integer 0-255
---@param b integer 0-255
---@param a integer 0-255
function Frame:setVertexColor(r, g, b, a)
    local raw = registry.require(self, 'Frame.setVertexColor')
    BlzFrameSetVertexColor(raw, BlzConvertColor(a, r, g, b))
end
---@param path string
---@param height number
---@param flags integer? Default 0.
function Frame:setFont(path, height, flags)
    BlzFrameSetFont(registry.require(self, 'Frame.setFont'), path, height, flags or 0)
end
---@param vertical textaligntype
---@param horizontal textaligntype
function Frame:setTextAlignment(vertical, horizontal)
    BlzFrameSetTextAlignment(registry.require(self, 'Frame.setTextAlignment'), vertical, horizontal)
end
---@param size integer
function Frame:setTextSizeLimit(size) BlzFrameSetTextSizeLimit(registry.require(self, 'Frame.setTextSizeLimit'), size) end
---@param path string
---@param flag integer? Default 0.
---@param blend boolean? Default true.
function Frame:setTexture(path, flag, blend)
    local raw = registry.require(self, 'Frame.setTexture')
    if blend == nil then blend = true end
    BlzFrameSetTexture(raw, path, flag or 0, blend)
end
---@param path string
---@param cameraIndex integer? Default 0.
function Frame:setModel(path, cameraIndex)
    BlzFrameSetModel(registry.require(self, 'Frame.setModel'), path, cameraIndex or 0)
end
---@param primaryProp integer
---@param flags integer
function Frame:setSpriteAnimate(primaryProp, flags)
    BlzFrameSetSpriteAnimate(registry.require(self, 'Frame.setSpriteAnimate'), primaryProp, flags)
end
---For text areas.
---@param flag boolean
function Frame:setAutoScroll(flag) BlzTextAreaFrameSetAutoScroll(registry.require(self, 'Frame.setAutoScroll'), flag) end
---@param value number
function Frame:setValue(value) BlzFrameSetValue(registry.require(self, 'Frame.setValue'), value) end
---@param min number
---@param max number
function Frame:setMinMaxValue(min, max) BlzFrameSetMinMaxValue(registry.require(self, 'Frame.setMinMaxValue'), min, max) end
---@param step number
function Frame:setStepSize(step) BlzFrameSetStepSize(registry.require(self, 'Frame.setStepSize'), step) end
---@param alpha integer 0-255
function Frame:setAlpha(alpha) BlzFrameSetAlpha(registry.require(self, 'Frame.setAlpha'), alpha) end
---@param flag boolean
function Frame:setEnabled(flag) BlzFrameSetEnable(registry.require(self, 'Frame.setEnabled'), flag) end
---Shows `tooltip` while the mouse is over this frame.
---@param tooltip MoonwellWrappers.Frame
function Frame:setTooltip(tooltip)
    local raw = registry.require(self, 'Frame.setTooltip')
    BlzFrameSetTooltip(raw, registry.require(tooltip, 'Frame.setTooltip'))
end
---@param flag boolean
function Frame:show(flag) BlzFrameSetVisible(registry.require(self, 'Frame.show'), flag) end
---Shows the frame on that player's machine only. Only local visuals differ.
---@param player MoonwellWrappers.Player
function Frame:setVisibleFor(player)
    local raw = registry.require(self, 'Frame.setVisibleFor')
    BlzFrameSetVisible(raw, Handle.unwrap(player, 'Player', 'Frame.setVisibleFor') == GetLocalPlayer())
end
---On that player's machine only, disables and re-enables the frame, so a clicked button gives keyboard focus back
---(hotkeys work again). Call it from a click callback with the callback's player.
---@param player MoonwellWrappers.Player
function Frame:releaseFocusFor(player)
    local raw = registry.require(self, 'Frame.releaseFocusFor')
    if Handle.unwrap(player, 'Player', 'Frame.releaseFocusFor') == GetLocalPlayer() then
        BlzFrameSetEnable(raw, false)
        BlzFrameSetEnable(raw, true)
    end
end
```

- [ ] **Step 4: Run test to verify it passes**

Run: `deno task test framestyle frame`
Expected: `framestyle: SUITE PASSED: 5 tests` and `frame: SUITE PASSED: 7 tests`.

- [ ] **Step 5: Run all checks and commit**

Run every command in "Running the wrappers checks". Expected: all pass.

```bash
git add src/wrappers/frame.lua tests/framestyle.lua
git commit -m "feat: Frame layout, content and state setters with local visibility and focus release"
```

---

### Task 3: Frame events

**Files:**
- Modify: `src/wrappers/internal/callback.lua`, `src/wrappers/frame.lua`
- Test: `tests/frameevents.lua`

**Interfaces:**
- Consumes: `states`, `registry`, `without`, `releaseEvents` (Task 1) in `frame.lua`; `wrappers.player`.
- Produces: `Callback.call(label, fn, ...)`; `Frame:on`, `Frame:off`; types `MoonwellWrappers.FrameHandler`,
  `MoonwellWrappers.FrameEvent`, `MoonwellWrappers.FrameCallback`.

- [ ] **Step 1: Write the failing test**

Create `tests/frameevents.lua`:

```lua
ORIGIN_FRAME_GAME_UI = {}
local ORIGIN = {}
native('BlzGetOriginFrame', function() return ORIGIN end)
native('BlzCreateFrameByType', function(_, _, parent) return {parent = parent} end)
native('BlzDestroyFrame', function() end)
local actions = {}
native('CreateTrigger', function() return {events = {}} end)
native('TriggerAddAction', function(trigger, callback)
    actions[#actions + 1] = {trigger = trigger, callback = callback}
    return {}
end)
native('DestroyTrigger', function(trigger) trigger.destroyed = true end)
native('BlzTriggerRegisterFrameEvent', function(trigger, raw, eventType)
    trigger.events[#trigger.events + 1] = {raw = raw, type = eventType}
end)
local current = {}
native('BlzGetTriggerFrameEvent', function() return current.type end)
native('BlzGetTriggerFrameText', function() return current.text end)
native('BlzGetTriggerFrameValue', function() return current.value end)
native('GetTriggerPlayer', function() return current.player end)
local Frame = require('wrappers.frame')
local Player = require('wrappers.player')
eq(totalCalls(), 0)

-- Simulates Warcraft firing a frame event: runs the action of every trigger registered for that frame and event type
-- (a destroyed trigger only when `evenIfDestroyed` is set).
local function fire(raw, eventType, text, value, who, evenIfDestroyed)
    current = {type = eventType, text = text or '', value = value or 0, player = who or PLAYER_RAW}
    for _, action in ipairs(actions) do
        if evenIfDestroyed or not action.trigger.destroyed then
            for _, registration in ipairs(action.trigger.events) do
                if registration.raw == raw and registration.type == eventType then
                    action.callback()
                    break
                end
            end
        end
    end
end

test('on makes one internal trigger per frame and registers each event type once', function()
    local ui = Frame.origin(ORIGIN_FRAME_GAME_UI)
    local button = Frame.createByType('BUTTON', ui)
    button:on('click', function() end); button:on('click', function() end); button:on('enter', function() end)
    eq(callCount('CreateTrigger'), 1); eq(callCount('TriggerAddAction'), 1)
    eq(callCount('BlzTriggerRegisterFrameEvent'), 2)
    expectCall('BlzTriggerRegisterFrameEvent', actions[#actions].trigger, button.handle, 'enter')
    local quiet = Frame.createByType('TEXT', ui)
    quiet:destroy(); eq(callCount('DestroyTrigger'), 0)
    button:destroy(); eq(callCount('DestroyTrigger'), 1)
end)

test('callbacks run in order with the player and the event data', function()
    local box = Frame.createByType('EDITBOX', Frame.origin(ORIGIN_FRAME_GAME_UI))
    local seen = {}
    box:on('enter', function(player, event) seen[#seen + 1] = {'first', player, event} end)
    box:on('enter', function(player, event) seen[#seen + 1] = {'second', player, event} end)
    box:on('click', function() seen[#seen + 1] = {'click'} end)
    local other = {}
    fire(box.handle, 'enter', 'hello', 2.5, other)
    eq(#seen, 2); eq(seen[1][1], 'first'); eq(seen[2][1], 'second')
    eq(seen[1][2], Player.fromHandle(other))
    local event = seen[1][3]
    eq(event.type, 'enter'); eq(event.frame, box); eq(event.text, 'hello'); eq(event.value, 2.5)
    eq(seen[2][3] ~= event, true)
    fire(box.handle, 'mouse')
    eq(#seen, 2)
    box:destroy()
end)

test('a failing callback is printed and the next one still runs', function()
    local button = Frame.createByType('BUTTON', Frame.origin(ORIGIN_FRAME_GAME_UI))
    local count = 0
    button:on('click', function() error('intentional frame probe') end)
    button:on('click', function() count = count + 1 end)
    fire(button.handle, 'click'); fire(button.handle, 'click')
    eq(count, 2); eq(#PRINTED, 2)
    assert(PRINTED[1]:find('[wrappers] Frame event callback failed:', 1, true), PRINTED[1])
    assert(PRINTED[1]:find('intentional frame probe', 1, true), PRINTED[1])
    button:destroy()
end)

test('off removes a callback at once; callbacks added during a firing wait for the next', function()
    local ui = Frame.origin(ORIGIN_FRAME_GAME_UI)
    local button, other = Frame.createByType('BUTTON', ui), Frame.createByType('BUTTON', ui)
    local log = {}
    local second, added
    button:on('click', function() log[#log + 1] = 'first'; button:off(second) end)
    second = button:on('click', function() log[#log + 1] = 'second' end)
    button:on('click', function()
        log[#log + 1] = 'third'
        if not added then added = button:on('click', function() log[#log + 1] = 'late' end) end
    end)
    fire(button.handle, 'click')
    eq(table.concat(log, ','), 'first,third')
    log = {}
    fire(button.handle, 'click')
    eq(table.concat(log, ','), 'first,third,late')
    button:off(second)
    fails(function() other:off(added) end, 'Frame.off: token belongs to another frame')
    fails(function() button:off({}) end, 'Frame.off: expected FrameHandler token')
    button:destroy(); other:destroy()
end)

test('destroy clears the callbacks and triggers of the whole subtree', function()
    local ui = Frame.origin(ORIGIN_FRAME_GAME_UI)
    local panel = Frame.createByType('BACKDROP', ui)
    local button = Frame.createByType('BUTTON', panel)
    local count = 0
    panel:on('enter', function() count = count + 1 end)
    button:on('click', function() count = count + 1 end)
    local panelRaw, buttonRaw = panel.handle, button.handle
    resetCalls()
    panel:destroy()
    eq(callCount('DestroyTrigger'), 2); eq(callCount('BlzDestroyFrame'), 1)
    fire(panelRaw, 'enter', nil, nil, nil, true); fire(buttonRaw, 'click', nil, nil, nil, true)
    eq(count, 0)
end)

test('a callback may destroy its own frame or an ancestor', function()
    local ui = Frame.origin(ORIGIN_FRAME_GAME_UI)
    local panel = Frame.createByType('BACKDROP', ui)
    local close = Frame.createByType('BUTTON', panel)
    local after = {}
    close:on('click', function() panel:destroy(); after[#after + 1] = 'closed' end)
    close:on('click', function() after[#after + 1] = 'ERROR later callback ran' end)
    fire(close.handle, 'click')
    eq(table.concat(after, ','), 'closed'); eq(panel:isDisposed(), true); eq(close:isDisposed(), true)
    local solo = Frame.createByType('BUTTON', ui)
    solo:on('click', function() solo:destroy() end)
    fire(solo.handle, 'click'); eq(solo:isDisposed(), true)
    eq(#PRINTED, 0)
end)

test('on checks the frame, event type and callback before any native', function()
    local ui = Frame.origin(ORIGIN_FRAME_GAME_UI)
    local button = Frame.createByType('BUTTON', ui)
    resetCalls()
    fails(function() button:on('click', nil) end, 'Frame.on: expected a callback function')
    fails(function() button:on(nil, function() end) end, 'Frame.on: expected a frame event type')
    eq(totalCalls(), 0)
    button:destroy()
    fails(function() button:on('click', function() end) end, 'Frame.on: Frame is disposed')
    fails(function() button:off({}) end, 'Frame.off: Frame is disposed')
    ui:on('click', function() end)
    eq(callCount('CreateTrigger'), 1)
end)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `deno task test frameevents`
Expected: FAIL (for example `attempt to call a nil value (method 'on')`).

- [ ] **Step 3: Give `Callback.call` varargs**

In `src/wrappers/internal/callback.lua`, replace:

```lua
---@generic T
---@param label string
---@param fn fun(value: T): ...
---@param argument T
function Callback.call(label, fn, argument)
    local ok, message = pcall(fn, argument)
    if not ok then report(label .. ' callback', message) end
end
```

with:

```lua
---Runs a callback behind the callback boundary: an error is printed and does not propagate.
---@param label string
---@param fn function
---@param ... any Passed to fn.
function Callback.call(label, fn, ...)
    local ok, message = pcall(fn, ...)
    if not ok then report(label .. ' callback', message) end
end
```

- [ ] **Step 4: Add events to `frame.lua`**

In `src/wrappers/frame.lua`, after `local Options = require('wrappers.internal.options')`, add:

```lua
local Callback = require('wrappers.internal.callback')
local PlayerWrapper = require('wrappers.player')
```

Before the final `return Frame`, insert:

```lua
---Opaque token returned by Frame:on; pass it to Frame:off.
---@class MoonwellWrappers.FrameHandler

---@class MoonwellWrappers.FrameEvent
---@field type frameeventtype
---@field frame MoonwellWrappers.Frame
---@field text string The event's synced text (edit boxes).
---@field value number The event's synced value (sliders, check boxes, popup menus, the mouse wheel).

---@alias MoonwellWrappers.FrameCallback fun(player: MoonwellWrappers.Player, event: MoonwellWrappers.FrameEvent): ...

---@type table<MoonwellWrappers.FrameHandler, MoonwellWrappers.FrameCell>
local handlers = setmetatable({}, {__mode = 'k'})

---The internal trigger's action: runs the live callbacks for the event type that fired, in the order added. Callbacks
---added during this firing wait for the next one; removed or disposed ones are skipped at once.
---@param frame MoonwellWrappers.Frame
local function route(frame)
    local state = states[frame]
    if not state then return end
    local eventType = BlzGetTriggerFrameEvent()
    local list = state.byType[eventType]
    if not list then return end
    local player = PlayerWrapper.fromHandle(GetTriggerPlayer())
    local text, value = BlzGetTriggerFrameText(), BlzGetTriggerFrameValue()
    for index = 1, #list do
        local callback = list[index].callback
        if callback then
            Callback.call('Frame event', callback, player, {type = eventType, frame = frame, text = text, value = value})
        end
    end
end

---Runs `callback` when the event fires for this frame, behind the callback boundary. It receives the Player who caused
---the event and the event's synced data.
---@param eventType frameeventtype For example FRAMEEVENT_CONTROL_CLICK.
---@param callback MoonwellWrappers.FrameCallback
---@return MoonwellWrappers.FrameHandler
function Frame:on(eventType, callback)
    local raw = registry.require(self, 'Frame.on')
    if eventType == nil then error('[wrappers] Frame.on: expected a frame event type', 2) end
    Callback.check(callback, 'Frame.on')
    local state = states[self]
    if not state.trigger then
        local trigger = Handle.created(CreateTrigger(), 'Frame.on')
        TriggerAddAction(trigger, function() route(self) end)
        state.trigger = trigger
    end
    local list = state.byType[eventType]
    if not list then
        list = {}
        state.byType[eventType] = list
        BlzTriggerRegisterFrameEvent(state.trigger, raw, eventType)
    end
    ---@type MoonwellWrappers.FrameCell
    local cell = {frame = self, eventType = eventType, callback = callback}
    list[#list + 1] = cell
    state.cells[#state.cells + 1] = cell
    ---@type MoonwellWrappers.FrameHandler
    local token = {}
    handlers[token] = cell
    return token
end
---Removes a callback at once, even during a firing. Removing it twice does nothing.
---@param token MoonwellWrappers.FrameHandler
function Frame:off(token)
    registry.require(self, 'Frame.off')
    local cell = handlers[token]
    if not cell then error('[wrappers] Frame.off: expected FrameHandler token', 2) end
    if cell.frame ~= self then error('[wrappers] Frame.off: token belongs to another frame', 2) end
    if not cell.callback then return end
    cell.callback = nil
    local state = states[self]
    state.byType[cell.eventType] = without(state.byType[cell.eventType], cell)
    state.cells = without(state.cells, cell)
end
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `deno task test frameevents frame framestyle trigger timer dialog`
Expected: `frameevents: SUITE PASSED: 7 tests`, and the other suites pass as before (`Callback.call` still works with
one argument).

- [ ] **Step 6: Run all checks and commit**

Run every command in "Running the wrappers checks". Expected: all pass.

```bash
git add src/wrappers/internal/callback.lua src/wrappers/frame.lua tests/frameevents.lua
git commit -m "feat: Frame event callbacks with removable tokens and synced event data"
```

---

### Task 4: Import graph, bundle and editor coverage

**Files:**
- Modify: `tests/imports.lua`, `tools/integration.ts`, `tests/editor-positive.lua`, `tests/editor-negative.lua`

**Interfaces:**
- Consumes: `wrappers.frame`.
- Produces: proof that `wrappers.frame` loads only `wrappers.player` among public modules; LuaLS coverage.

- [ ] **Step 1: Extend the import test**

Append to `tests/imports.lua`:

```lua

test('the frame module loads the Player module and no other', function()
    require('wrappers.frame')
    eq(totalCalls(), 0)
    eq(package.loaded['wrappers.player'] ~= nil, true)
    for _, name in ipairs({'unit', 'group', 'item', 'force'}) do eq(package.loaded['wrappers.' .. name], nil) end
end)
```

Run: `deno task test imports`
Expected: `imports: SUITE PASSED: 3 tests`.

- [ ] **Step 2: Extend `tools/integration.ts`**

In the Unit-only exclusion list, add `"frame",` after `"timerdialog",`. In `publicModules`, add `"frame",` after
`"timerdialog",`. In `soloEntries`, add after the `dialog` entry:

```ts
  frame: { source: 'import "wrappers.frame" as Frame\nFrame.hideOrigin false\n', allowed: ["player"] },
```

Change the final message of that block to
`"Moonwell: Trigger-, TextTag-, Multiboard-, Dialog- and Frame-only maps bundle only what they import"`. Run `deno fmt`.

- [ ] **Step 3: Extend the LuaLS positive fixture**

In `tests/editor-positive.lua`, after `local TimerDialog = require('wrappers.timerdialog')`, add
`local Frame = require('wrappers.frame')`. After the line `dialog:destroy()`, add:

```lua
local gameUi = Frame.origin(ORIGIN_FRAME_GAME_UI)
local panel = Frame.create('EscMenuBackdrop', gameUi, {priority = 1})
panel:setAbsPoint(FRAMEPOINT_CENTER, 0.4, 0.3)
panel:setSize(0.3, 0.2)
local okButton = Frame.createByType('GLUETEXTBUTTON', panel, {name = 'Ok', inherits = 'ScriptDialogButton'})
okButton:setPoint(FRAMEPOINT_BOTTOM, panel, FRAMEPOINT_BOTTOM, 0, 0.02)
okButton:setText('OK')
okButton:setTextColor(255, 255, 255, 255)
local okToken = okButton:on(FRAMEEVENT_CONTROL_CLICK, function(player, event)
    okButton:releaseFocusFor(player)
    print(player:getName(), event.text, event.value, event.frame:getName())
end)
okButton:off(okToken)
panel:setVisibleFor(PlayerWrapper.fromIndex(0))
local maybeParent = okButton:getParent()
if maybeParent then maybeParent:setAlpha(255) end
panel:destroy()
```

- [ ] **Step 4: Extend the LuaLS negative fixture**

In `tests/editor-negative.lua`, after `local Multiboard = require('wrappers.multiboard')`, add
`local Frame = require('wrappers.frame')`. Before the final `return true`, add:

```lua
Frame.create('EscMenuBackdrop', unit) -- EXPECT param-type-mismatch
Frame.origin(ORIGIN_FRAME_GAME_UI):on(FRAMEEVENT_CONTROL_CLICK, function(player) Group.create():add(player) end) -- EXPECT param-type-mismatch
```

- [ ] **Step 5: Run the integration check**

Run: `deno task test:integration`
Expected: PASS, with `LuaLS: 15 intentional type errors detected at the expected lines` and the new bundle message. If
LuaLS reports a different code on one of the new negative lines, change that line's `EXPECT`; if it reports nothing on
the callback line, delete that line and say so in the task report. Any other mismatch is a real failure.

- [ ] **Step 6: Run all checks and commit**

Run every command in "Running the wrappers checks". Expected: all pass.

```bash
git add tests/imports.lua tools/integration.ts tests/editor-positive.lua tests/editor-negative.lua
git commit -m "test: import graph, Frame-only bundle and editor fixtures for frames"
```

---

### Task 5: Gate example and documentation

**Files:**
- Modify: `examples/gate.yue`, `README.md`, `CHANGELOG.md`, `CONTRIBUTING.md`, `AGENTS.md` (wrappers)
- Modify: Moonwell `AGENTS.md`

**Interfaces:**
- Consumes: the full v0.5.0 API.
- Produces: the in-game gate and the docs.

- [ ] **Step 1: Add the frames gate to `examples/gate.yue`**

Edit with the file-editing tool (backslashes). After the header line
`-- Set ui = true for a separate run of the v0.4.0 classic UI gate only.`, add:

```text
-- Set frames = true for a separate run of the v0.5.0 frames gate only; it loads war3mapImported\wrappers-gate.toc.
```

After `import "wrappers.timerdialog" as TimerDialog`, add `import "wrappers.frame" as Frame`. After `ui = false`, add
`frames = false`.

Before the `-- v0.4.0 classic UI, after the dialog` comment, add:

```text
-- v0.5.0 frames. Run with frames = true; the templates come from the gate map's TOC file.
framesGate = (owner) ->
  other = Player.fromIndex 1
  Frame.loadTOC "war3mapImported\\wrappers-gate.toc"
  ui = Frame.origin ORIGIN_FRAME_GAME_UI
  panel = Frame.create "EscMenuBackdrop", ui
  panel\setAbsPoint FRAMEPOINT_CENTER, 0.4, 0.35
  panel\setSize 0.32, 0.3
  title = Frame.create "EscMenuTitleTextTemplate", panel
  title\setPoint FRAMEPOINT_TOP, panel, FRAMEPOINT_TOP, 0, -0.03
  title\setText "Wrapper frames"
  title\setTextColor 255, 220, 0, 255
  button = Frame.createByType "GLUETEXTBUTTON", panel, name: "WrapperClick", inherits: "ScriptDialogButton"
  button\setPoint FRAMEPOINT_TOP, title, FRAMEPOINT_BOTTOM, 0, -0.01
  button\setSize 0.14, 0.035
  button\setText "Click me"
  clicks = 0
  button\on FRAMEEVENT_CONTROL_CLICK, (player) ->
    clicks += 1
    button\releaseFocusFor player
    print "Wrapper frame clicked by", player\getName!, "count", clicks
  tip = Frame.create "EscMenuLabelTextTemplate", panel
  tip\setPoint FRAMEPOINT_BOTTOM, button, FRAMEPOINT_TOP, 0, 0.005
  tip\setText "Wrapper tooltip"
  button\setTooltip tip
  box = Frame.create "EscMenuEditBoxTemplate", panel
  box\setPoint FRAMEPOINT_TOP, button, FRAMEPOINT_BOTTOM, 0, -0.01
  box\setSize 0.2, 0.03
  box\on FRAMEEVENT_EDITBOX_ENTER, (player, event) ->
    print "Wrapper edit box enter by", player\getName!, event.text
  slider = Frame.create "EscMenuSliderTemplate", panel
  slider\setPoint FRAMEPOINT_TOP, box, FRAMEPOINT_BOTTOM, 0, -0.015
  slider\setSize 0.2, 0.012
  slider\setMinMaxValue 0, 10
  slider\setStepSize 1
  slider\setValue 5
  slider\on FRAMEEVENT_SLIDER_VALUE_CHANGED, (player, event) ->
    print "Wrapper slider value", event.value
  check = Frame.create "QuestCheckBox", panel
  check\setPoint FRAMEPOINT_TOPLEFT, slider, FRAMEPOINT_BOTTOMLEFT, 0, -0.015
  check\on FRAMEEVENT_CHECKBOX_CHECKED, (player) -> print "Wrapper checkbox checked by", player\getName!
  check\on FRAMEEVENT_CHECKBOX_UNCHECKED, (player) -> print "Wrapper checkbox unchecked by", player\getName!
  hidden = Frame.create "EscMenuLabelTextTemplate", panel
  hidden\setPoint FRAMEPOINT_BOTTOMLEFT, panel, FRAMEPOINT_BOTTOMLEFT, 0.03, 0.03
  hidden\setText "ERROR visible for another player"
  hidden\setVisibleFor other
  badge = Frame.createByType "BACKDROP", ui, name: "WrapperBadge"
  badge\setTexture "ReplaceableTextures\\CommandButtons\\BTNFootman.blp"
  badge\setSize 0.04, 0.04
  badge\setAbsPoint FRAMEPOINT_TOPLEFT, 0.02, 0.55
  close = Frame.createByType "GLUETEXTBUTTON", panel, name: "WrapperClose", inherits: "ScriptDialogButton"
  close\setPoint FRAMEPOINT_BOTTOMRIGHT, panel, FRAMEPOINT_BOTTOMRIGHT, -0.03, 0.025
  close\setSize 0.09, 0.03
  close\setText "Close"
  close\on FRAMEEVENT_CONTROL_CLICK, ->
    panel\destroy!
    print "Wrapper frames closed; badge disposed", badge\isDisposed!, "button disposed", button\isDisposed!
  print "Wrapper frames shown:", panel\getName!, "children", panel\getChildrenCount!
  mover = Timer.create!
  mover\start 5, false, (self) ->
    self\destroy!
    unless panel\isDisposed!
      badge\setParent panel
      badge\clearPoints!
      badge\setPoint FRAMEPOINT_TOPRIGHT, panel, FRAMEPOINT_TOPRIGHT, -0.03, -0.03
      print "Wrapper badge moved into the panel"
```

In the final `mw.on_main` block, change `if ui` to `if frames` followed by `framesGate owner`, then `elseif ui`, so the
block reads:

```text
mw.on_main ->
  start = Timer.create!
  start\start 0, false, (self) ->
    self\destroy!
    owner = Player.fromIndex 0
    if frames
      framesGate owner
    elseif ui
      uiGate owner
    elseif presentation
      presentationGate owner
    else
      foundationGate owner
      broadGate owner
```

Run: `deno task test:integration`
Expected: PASS, including the gate example line. If LuaLS reports a diagnostic in the gate, fix the gate code.

- [ ] **Step 2: Update `CONTRIBUTING.md`'s in-game gate**

In step 1, extend the sentence about runs that never print the weak cache probe with: "the frames run (step 9) never
prints it either and ends when you click its `Close` button".

Insert after step 8 a new step 9, and renumber the old steps 9, 10 and 11 to 10, 11 and 12:

```markdown
9. Frames (v0.5.0): first run the gate map's `frame-init` probe (`../wrappers-gate`, `deno task gate frame-init`,
   instructions in its `PROBE-FRAME.md`) and record its answers in the results below and in README. If a template the
   gate uses did not create after loading the gate map's TOC, change the gate to one that did before continuing. Then
   set `frames = true` and run again; only the frames gate runs. `Wrapper frames shown: <name> children <n>` prints
   (record both). A panel appears in the centre with a yellow title `Wrapper frames`, a `Click me` button, an edit box, a
   slider, a check box and a `Close` button; a footman icon appears top left; `ERROR visible for another player` never
   appears. Hovering `Click me` shows `Wrapper tooltip`. Clicking it prints `Wrapper frame clicked by <your name> count
   1` (then 2, …); right after a click, pressing Enter opens the chat box (focus was released). Typing in the edit box
   and pressing Enter prints `Wrapper edit box enter by <your name> <text>`. Dragging the slider prints
   `Wrapper slider value <n>`. Ticking and unticking the check box print `Wrapper checkbox checked by <your name>` and
   `... unchecked ...`. At 5 s the footman icon jumps into the panel's top right (`Wrapper badge moved into the panel`);
   click `Close` only after that. Clicking `Close` removes the panel and the icon, prints
   `Wrapper frames closed; badge disposed true button disposed true`, and no `[wrappers] ... failed` line prints.
   Restore `frames = false`.
```

In the renumbered step 10 (the `--minify` step), change `repeat steps 2–5, 7 and 8 (without the probe)` to
`repeat steps 2–5 and 7–9 (without the probes)`. In the renumbered step 11 (two-player run), append: `It also covers
v0.5.0: frames created in the same order on both machines, `setVisibleFor` and `releaseFocusFor` acting only for that
player, and frame events from the second player printing that player's name on both machines; no desync.`

- [ ] **Step 3: Update `README.md`**

Replace in the intro sentence `Quest, DefeatCondition and TimerDialog wrappers,` with
`Quest, DefeatCondition, TimerDialog and Frame wrappers,`.

Replace the Status paragraph's last sentence `Frames are not wrapped yet.` with `v0.5.0 (frames) is in development on
main.`

In "Handles and cleanup", replace `the presentation and classic UI classes stay cached until you destroy them;` with
`the presentation, classic UI and frame classes stay cached until you destroy them (game frames for the session);`.

Add a new section after "Classic UI" (before "## Editor types"):

````markdown
## Frames

`wrappers.frame` wraps the `BlzFrame` API. Create, destroy and re-parent frames on every machine in the same order,
never inside a branch on the local player: frame handles and the wrapper's create contexts must agree across machines.
Screen coordinates run 0–0.8 wide and 0–0.6 high, from the bottom left.

There are three kinds of frame:

- **Owned** frames come from `Frame.create(template, parent, options?)` (option `priority`),
  `Frame.createSimple(template, parent)` and `Frame.createByType(frameType, parent, options?)` (options `name`,
  `inherits`). `frame:destroy()` destroys the frame and everything under it; the wrappers of its owned descendants and
  template parts are disposed at once and their callbacks never run again.
- **Template parts** are the frames a template creates inside a frame: `frame:findChild(name)` finds one by name (the
  wrapper passes each owned frame its own create context, so names never clash), and `frame:getChild(index)` by
  zero-based index. A part belongs to the owned frame it was found through and cannot be destroyed or re-parented.
- **Borrowed** frames are the game's: `Frame.origin(ORIGIN_FRAME_GAME_UI)`, `Frame.byName(name, context?)`,
  `Frame.fromHandle(raw)`. They can be parents but never be destroyed through a wrapper, and can only be re-parented
  under another borrowed frame, so destroying one of your frames never silently destroys a game frame.

`frame:setParent(parent)` moves an owned frame, and it is then destroyed with its new parent. A frame cannot be moved
into its own subtree.

Events go to callbacks: `frame:on(FRAMEEVENT_CONTROL_CLICK, function(player, event) ... end)` returns a token for
`frame:off(token)`. The callback receives the Player who caused the event and `event` with `type`, `frame`, `text`
(edit boxes) and `value` (sliders, check boxes, popup menus, the mouse wheel): the event's synced data. Callbacks run
behind the same error boundary as trigger actions and may destroy their own frame. After a click, a button keeps the
keyboard focus and hotkeys stop working; call `frame:releaseFocusFor(player)` in the click callback.

There are no getters for text, values, visibility, enabled state, alpha or size: they answer differently on each
machine (typed text, dragged sliders, local visibility). Read synced values in event callbacks. `setVisibleFor(Player)`
compares with the local player, as for the presentation classes. Colors (`setTextColor`, `setVertexColor`) are
integers 0–255.

Templates other than the built-in ones come from `.fdf` files listed in a `.toc` file. Put the TOC under `assets/`, for
example `assets/war3mapImported/templates.toc` (imported as `war3mapImported\templates.toc`), with one FDF path per line
and an empty last line:

```text
UI\FrameDef\UI\EscMenuTemplates.fdf
UI\FrameDef\Glue\StandardTemplates.fdf

```

Then load it in a hook, before creating frames: `Frame.loadTOC("war3mapImported\\templates.toc")`. It raises when the
game cannot load the file. `Frame.hideOrigin(flag)` hides the game's own UI and `Frame.enableAutoPosition(flag)` turns
its automatic layout off or on, for everyone.
````

In the API reference table, add after the `wrappers.timerdialog` row:

```markdown
| `wrappers.frame` | `create(template, parent, options?)`, `createSimple(template, parent)`, `createByType(frameType, parent, options?)`, `origin(originType, index?)`, `byName(name, context?)`, `loadTOC(path)`, `hideOrigin(flag)`, `enableAutoPosition(flag)`; `getName()`, `getParent()`, `getChildrenCount()`, `getChild(index)`, `findChild(name)`, `setParent(Frame)`, `setPoint(point, Frame, relativePoint, x, y)`, `setAbsPoint(point, x, y)`, `setAllPoints(Frame)`, `clearPoints()`, `setSize(width, height)`, `setScale(scale)`, `setLevel(level)`, `setText(text)`, `addText(text)`, `setTextColor(r, g, b, a)`, `setVertexColor(r, g, b, a)`, `setFont(path, height, flags?)`, `setTextAlignment(vertical, horizontal)`, `setTextSizeLimit(size)`, `setTexture(path, flag?, blend?)`, `setModel(path, cameraIndex?)`, `setSpriteAnimate(primaryProp, flags)`, `setAutoScroll(flag)`, `setValue(value)`, `setMinMaxValue(min, max)`, `setStepSize(step)`, `setAlpha(alpha)`, `setEnabled(flag)`, `setTooltip(Frame)`, `show(flag)`, `setVisibleFor(Player)`, `releaseFocusFor(Player)`, `on(eventType, callback)`, `off(token)`, `destroy()` |
```

Run `deno fmt`, then `deno fmt --check`.

- [ ] **Step 4: Update `CHANGELOG.md`**

Insert above `## 0.4.0 (2026-09-29)`:

```markdown
## Unreleased

- New `wrappers.frame`: the `BlzFrame` API with an owned frame tree. Frames made by `Frame.create`, `createSimple` and
  `createByType` are owned; `destroy()` disposes the wrappers of their whole subtree. Template parts (`findChild`,
  `getChild`) belong to their frame; game frames (`Frame.origin`, `byName`, `fromHandle`) are borrowed and never
  destroyed through a wrapper. Create contexts are allocated automatically.
- Frame events go to callbacks: `frame:on(eventType, callback)` with removable tokens; callbacks receive the Player and
  the event's synced text and value. `releaseFocusFor(player)` gives keyboard focus back after a click.
- `Frame.loadTOC` raises when a TOC file cannot be loaded; README shows how to import templates.
- `setVisibleFor(Player)` on frames. No getters for machine-local frame state.
```

- [ ] **Step 5: Update the wrappers `AGENTS.md`**

In the list of design links, add after the v0.4.0 entry:

```markdown
- `../moonwell/docs/superpowers/specs/2026-09-29-moonwell-wrappers-frames-design.md` and
  `../moonwell/docs/superpowers/plans/2026-09-29-moonwell-wrappers-frames.md` (v0.5.0)
```

Replace `Frames remain in Moonwell's backlog.` with `v0.5.0 adds frames.`

Add to "Rules":

```markdown
- Frames: one registry for every frame; a private kind per wrapper (owned, template part, borrowed). Only owned frames
  are destroyed, and their destroy disposes the owned subtree and its parts. Never let a borrowed frame end up under an
  owned one.
```

At the end of "Verification", append: `v0.5.0: the gate map has a `frame-init` probe
(`../wrappers-gate/src/probe_frame.yue`, `deno task gate frame-init`) and the runs `frames` and `frames-min`; the
frames gate loads `war3mapImported\wrappers-gate.toc` from the gate map's assets.`

- [ ] **Step 6: Run all wrappers checks and commit**

Run every command in "Running the wrappers checks". Expected: all pass.

```bash
git add examples/gate.yue README.md CHANGELOG.md CONTRIBUTING.md AGENTS.md
git commit -m "docs: frame wrappers, their in-game gate and changelog"
```

- [ ] **Step 7: Update Moonwell's `AGENTS.md`**

In `C:/Users/mdlsvensson/Repo/moonwell/AGENTS.md`, after the "Wrappers v0.4.0, classic UI, released" bullet in
"State", add:

```markdown
- **Wrappers v0.5.0, frames, implemented; in-game gate pending** (spec
  `docs/superpowers/specs/2026-09-29-moonwell-wrappers-frames-design.md`, plan
  `docs/superpowers/plans/2026-09-29-moonwell-wrappers-frames.md`): release C of the UI backlog item. `wrappers.frame`
  with an owned tree (owned frames, template parts, borrowed game frames), automatic create contexts, per-frame event
  callbacks with the player and synced event data, `releaseFocusFor`, `setVisibleFor` and `Frame.loadTOC`. Automated
  checks pass; the maintainer runs the gate map's `frame-init` probe, then CONTRIBUTING's gate with `frames = true`
  (`deno task gate frames`, then `frames-min`).
```

Replace "Next work, in order" item 1 with:

```markdown
1. **Finish wrappers v0.5.0:** the `frame-init` probe and the in-game gate (the wrappers repo's CONTRIBUTING steps 9 and
   10, in `../wrappers-gate`); record the probe's answers in README; then release it like v0.4.0 (tag `v0.5.0`, tag
   consumption gate).
2. **Then choose the next sub-project with the maintainer:** the editor error for effects attached to items and
   destructables, the YueScript port of `wc3-lib` (4d) or the Reforged map preview. Each needs a short design or a spec
   first. 4d has design inputs in the w3ts comparison §2.1 and the WCSharp comparison §2.1, whose systems are the
   closest prior art.
```

In "Backlog", in the "UI wrappers, releases B and C" bullet, replace `C, v0.5.0: the `BlzFrame` API, with its own
ownership design (TOC/FDF loading, parent trees, local frames).` with `C is wrappers v0.5.0, implemented (spec
`docs/superpowers/specs/2026-09-29-moonwell-wrappers-frames-design.md`), gate pending.` Add a new backlog bullet after
it:

```markdown
- **Assets shipped by libraries.** A library could ship files such as a frame template `.toc` and its `.fdf` files for
  the map to import (added 2026-09-29 with wrappers v0.5.0, whose README shows the manual recipe). Needs a design for
  where they live in a library and how they join the map's `assets/` import.
```

Run Moonwell's checks (AGENTS.md "Checks").

```bash
git add AGENTS.md
git commit -m "docs: record wrappers v0.5.0 implementation state"
```

---

### Task 6: Gate map runs, TOC file and the `frame-init` probe

**Files (in `C:/Users/mdlsvensson/Repo/wrappers-gate`, not under git):**
- Modify: `gate.ts`
- Create: `assets/war3mapImported/wrappers-gate.toc`, `src/probe_frame.yue`, `PROBE-FRAME.md`

**Interfaces:**
- Consumes: `examples/gate.yue` with `frames = false` (Task 5); `wrappers.frame`.
- Produces: `deno task gate frames|frames-min|frame-init [--no-launch]`.

- [ ] **Step 1: Add the runs to `gate.ts`**

Edit with the file-editing tool. In the header comment, after the `ui-min` line, add:

```ts
//   frame-init        v0.5.0 probe of community frame notes (src/probe_frame.yue, PROBE-FRAME.md)
//   frames            v0.5.0 frames gate (step 9)
//   frames-min        minified frames gate (step 10)
```

and change the `core-min`, `presentation-min` and `ui-min` lines' step numbers from 9 to 10, and the `ui` line's
`(step 8)` stays.

Add `frames: boolean` to the `runs` record type and `frames: false` to every existing entry, and add:

```ts
  "frames": { probes: false, presentation: false, ui: false, frames: true, minify: false },
  "frames-min": { probes: false, presentation: false, ui: false, frames: true, minify: true },
```

Add `"frame-init": "src/probe_frame.yue"` to `probeRuns`. In `variant`, add `frames: boolean` to the parameter type and,
after the `ui = false` replacement, `text = replace(text, "frames = false", `frames = ${run.frames}`);`.

- [ ] **Step 2: Write the TOC file**

Create `assets/war3mapImported/wrappers-gate.toc` with the file-editing tool (backslashes), ending with an empty line:

```text
UI\FrameDef\UI\EscMenuTemplates.fdf
UI\FrameDef\Glue\StandardTemplates.fdf
UI\FrameDef\Glue\BattleNetTemplates.fdf
UI\FrameDef\UI\QuestDialog.fdf

```

- [ ] **Step 3: Write the probe `src/probe_frame.yue`**

```text
-- Wrappers v0.5.0 frame-init probe (Moonwell spec 2026-09-29-moonwell-wrappers-frames-design.md §8): community notes
-- about frames, on the game version in use. Build and launch with `deno task gate frame-init`; PROBE-FRAME.md says
-- what to watch. Every result line starts with "PROBE".
import "moonwell" as mw
import "wrappers.timer" as Timer
import "wrappers.frame" as Frame

clock = nil
now = -> string.format "%.2f", clock\getElapsed!
say = (...) -> print "PROBE", now!, ...

icon = "ReplaceableTextures\\CommandButtons\\BTNFootman.blp"
templates = {"ScriptDialogButton", "EscMenuBackdrop", "EscMenuTitleTextTemplate", "EscMenuLabelTextTemplate",
  "EscMenuEditBoxTemplate", "EscMenuSliderTemplate", "QuestCheckBox", "QuestButtonBaseTemplate",
  "BattleNetTextAreaTemplate"}
early = {}

-- A visible footman icon made with raw natives, so the probe measures the game rather than the wrappers.
square = (name, parent, x, y) ->
  frame = BlzCreateFrameByType "BACKDROP", name, parent, "", 0
  BlzFrameSetTexture frame, icon, 0, true
  BlzFrameSetSize frame, 0.04, 0.04
  BlzFrameSetAbsPoint frame, FRAMEPOINT_CENTER, x, y
  frame

-- Tries each template with raw natives and destroys what it creates.
tryTemplates = (label, context) ->
  ui = BlzGetOriginFrame ORIGIN_FRAME_GAME_UI, 0
  for index, name in ipairs templates
    frame = BlzCreateFrame name, ui, 0, context + index
    if frame
      say label, name, "created, handle id", GetHandleId(frame), "name", BlzFrameGetName(frame)
      BlzDestroyFrame frame
    else
      say label, name, "nil"

mw.on_main ->
  origin = BlzGetOriginFrame ORIGIN_FRAME_GAME_UI, 0
  early.origin = origin ~= nil
  early.square = square("ProbeInitSquare", origin, 0.1, 0.5) if origin
  clock = Timer.create!
  clock\start 3600, false, -> nil
  steps = Timer.create!
  step = 0
  local unknown, parentSquare, buttons
  steps\start 1, true, (self) ->
    step += 1
    ui = BlzGetOriginFrame ORIGIN_FRAME_GAME_UI, 0
    if step == 1
      say "1) origin frame in on_main", early.origin, "- is a footman icon visible top left (made in on_main)?"
      created = BlzCreateFrameByType "TEXT", "ProbeIdentity", ui, "", 7
      say "2) identity: BlzGetFrameByName returns the created frame", BlzGetFrameByName("ProbeIdentity", 7) == created
      BlzDestroyFrame created
      tryTemplates "3a) without TOC:", 100
      say "3b) BlzLoadTOCFile wrappers-gate.toc", BlzLoadTOCFile("war3mapImported\\wrappers-gate.toc")
      tryTemplates "3c) after TOC:", 200
      say "4a) BlzLoadTOCFile missing.toc", BlzLoadTOCFile("war3mapImported\\missing.toc")
      unknown = BlzCreateFrame "WrapperNoSuchTemplate", ui, 0, 300
      say "4b) unknown template gives a frame", unknown ~= nil, "handle id", unknown and GetHandleId(unknown) or "none"
    elseif step == 3
      parentSquare = square "ProbeParent", ui, 0.3, 0.45
      child = BlzCreateFrameByType "BACKDROP", "ProbeChild", parentSquare, "", 0
      BlzFrameSetTexture child, icon, 0, true
      BlzFrameSetSize child, 0.04, 0.04
      BlzFrameSetPoint child, FRAMEPOINT_LEFT, parentSquare, FRAMEPOINT_RIGHT, 0.01, 0
      moved = square "ProbeMoved", ui, 0.45, 0.45
      BlzFrameSetParent moved, parentSquare
      say "5) three footman icons in a row at the top; at 8 s the first (with its child and a re-parented one) is destroyed"
    elseif step == 8
      BlzDestroyFrame parentSquare
      say "5) parent destroyed: did all three icons disappear? child still found by name:", BlzGetFrameByName("ProbeChild", 0) ~= nil
    elseif step == 10
      gameUi = Frame.origin ORIGIN_FRAME_GAME_UI
      plain = Frame.createByType "GLUETEXTBUTTON", gameUi, name: "ProbePlain", inherits: "ScriptDialogButton"
      plain\setAbsPoint FRAMEPOINT_CENTER, 0.3, 0.3
      plain\setSize 0.12, 0.035
      plain\setText "Plain"
      plain\on FRAMEEVENT_CONTROL_CLICK, -> say "6) Plain clicked: press Enter now - does the chat box open?"
      released = Frame.createByType "GLUETEXTBUTTON", gameUi, name: "ProbeReleased", inherits: "ScriptDialogButton"
      released\setAbsPoint FRAMEPOINT_CENTER, 0.5, 0.3
      released\setSize 0.12, 0.035
      released\setText "Released"
      released\on FRAMEEVENT_CONTROL_CLICK, (player) ->
        released\releaseFocusFor player
        say "6) Released clicked: press Enter now - does the chat box open?"
      doomed = Frame.createByType "GLUETEXTBUTTON", gameUi, name: "ProbeDoomed", inherits: "ScriptDialogButton"
      doomed\setAbsPoint FRAMEPOINT_CENTER, 0.4, 0.22
      doomed\setSize 0.12, 0.035
      doomed\setText "Destroy me"
      doomed\on FRAMEEVENT_CONTROL_CLICK, ->
        doomed\destroy!
        say "7) destroyed from its own click; no crash so far"
      buttons = {plain, released}
      say "6) click Plain then press Enter; click Released then press Enter; 7) click Destroy me. You have until 40 s."
    elseif step == 40
      button\destroy! for button in *buttons
      if unknown
        say "4c) using the unknown-template frame in 5 s (may crash): screenshot the F12 log now"
    elseif step == 45
      if unknown
        BlzFrameSetText unknown, "unknown"
        BlzFrameSetVisible unknown, true
        say "4c) used the unknown-template frame: no crash"
    elseif step == 47
      BlzDestroyFrame early.square if early.square
      self\destroy!
      say "done: screenshot the F12 log"
```

- [ ] **Step 4: Write `PROBE-FRAME.md`**

```markdown
# Wrappers v0.5.0 frame-init probe: what to do

One run, about 50 seconds. It checks community notes about frames that the wrappers rely on. From this folder:

    deno task gate frame-init

Every result line in the log starts with `PROBE` and the game time. Answer as they print:

1. At start, is a footman icon visible top left (made in `on_main`)?
2. (Log only: identity.)
3. (Log only: which templates create without and after the TOC.)
4. (Log only: missing TOC and unknown template.)
5. At 3 s three footman icons appear in a row near the top. At 8 s: do all three disappear?
6. From 10 s: click `Plain`, then press Enter: does the chat box open? Close it; click `Released`, then press Enter:
   does it open now?
7. Click `Destroy me`: does it disappear without a crash?

At 40 s the log says `screenshot the F12 log now` if the unknown template gave a frame: do so, since the next step may
crash. At `done`, screenshot the F12 log again (scroll to see every line) and paste the screenshots with your answers.

Then the gate itself: `deno task gate frames`, and `deno task gate frames-min` (the wrappers repo's CONTRIBUTING steps
9 and 10).
```

- [ ] **Step 5: Build every new run without launching**

From `C:/Users/mdlsvensson/Repo/wrappers-gate`:

```powershell
deno task gate frame-init --no-launch
deno task gate frames --no-launch
deno task gate frames-min --no-launch
deno task gate ui --no-launch
```

Expected: each exits 0 and prints `Gate map: gate-maps/<run>.w3x`; `src/gate_frames.yue` contains `frames = true` on
its flag line. If the probe fails to compile (for example the `local unknown, parentSquare, buttons` line), fix the
probe. Nothing to commit: this folder is not under git.

- [ ] **Step 6: Hand over to the maintainer**

Report that `deno task gate frame-init` (PROBE-FRAME.md) is ready, then `deno task gate frames` and `frames-min`.
