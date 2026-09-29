# Moonwell Classic UI Wrappers (wrappers v0.4.0) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add classic UI wrappers (Dialog with DialogButton, Multiboard, Leaderboard, Quest with QuestItem,
DefeatCondition, TimerDialog) to `moonwell-wrappers`, with their in-game gate and a probe for w3ts's unmeasured claims.

**Architecture:** Handwritten annotated Lua 5.3 modules in `../moonwell-wrappers/src/wrappers/`, following v0.3.0's
patterns: `Handle.new` registries with strong caches, `Handle.unwrap` for arguments, `Options.read` for options tables,
explicit `destroy()`. Children that die with their parent (dialog buttons, quest items) are wrappers disposed by the
parent. Multiboard cell handles never become wrappers: each cell call gets and releases them. Dialog clicks go to
per-button callbacks through one internal trigger per dialog.

**Tech Stack:** Lua 5.3 (game), YueScript 0.34.2 embedded Lua 5.4 test VM, LuaLS 3.19.1, Lua 5.3.6 `luac`, Deno
tooling, Moonwell 0.5.0 consumer fixtures.

**Spec:** `docs/superpowers/specs/2026-09-29-moonwell-wrappers-classic-ui-design.md` (builds on the v0.1.0, v0.2.0 and
v0.3.0 wrapper specs in the same folder).

## Global Constraints

- Product code, tests and library docs live in `C:/Users/mdlsvensson/Repo/moonwell-wrappers`; this plan and the spec
  stay in Moonwell. The gate map `C:/Users/mdlsvensson/Repo/wrappers-gate` is not under git. Commit on `main` in each
  repository; the maintainer pushes. No tags, no publishing.
- No Node.js, npm packages, `node:` or `npm:` specifiers. Deno tooling uses built-ins and `jsr:@std/*` only.
- Runtime Lua ships only under `src/wrappers/`. Literal `require`s only; no umbrella module; no globals; no native call
  or game-object creation at import time.
- The game's Lua lacks `collectgarbage`, `debug`, `io`, `package`, `dofile`, `loadfile`; `os` has only `clock`, `date`,
  `difftime`, `time`. Shipped code must not use them.
- Additive release: every v0.3.1 call keeps its behavior.
- A module imports another public module only to return its wrappers. `dialog.lua` imports `wrappers.player` (its
  callbacks receive Player wrappers); every other new module imports only `wrappers.internal.*`. Convert wrapper
  arguments with `Handle.unwrap(value, 'Class', operation)`.
- Every public function carries LuaLS annotations. `fromHandle` stays conservatively nullable. No diagnostic
  suppressions are needed in this release; add none.
- Misuse errors read `[wrappers] <Class>.<method>: ...`. Validate receivers, arguments and options before any native
  that changes state (getters that read counts may run first).
- Never iterate a table keyed by tables with `pairs` when the loop calls natives. Use arrays.
- Rows and columns in the Multiboard API are one-based; natives get `index - 1`. No MultiboardItem wrapper: every
  `MultiboardGetItem` is followed by `MultiboardReleaseItem` in the same call.
- No getters for machine-local state: no `isMinimized`, no `isDisplayed` on Multiboard, Leaderboard or TimerDialog.
- DialogButton and QuestItem have no `destroy()` and no `fromHandle`.
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

The integration run does not diagnose the library's own files. After changing `src/`, also run LuaLS directly over the
source, with the native declarations already in `.test-work/luals-src/types`:

```powershell
Remove-Item -Recurse -Force .test-work/luals-src/src
Copy-Item -Recurse src .test-work/luals-src/src
& $env:MOONWELL_LUALS --check=.test-work/luals-src --checklevel=Hint
```

Expected: `no problems found`. If `.test-work/luals-src` is missing, create it: copy `src` there, copy
`natives.d.lua` and `moonwell.d.lua` from any `.test-work/integration-*/consumer/.moonwell/types/` into
`.test-work/luals-src/types/`, and write `.test-work/luals-src/.luarc.json`:

```json
{
  "runtime.version": "Lua 5.3",
  "runtime.path": ["src/?.lua", "src/?/init.lua"],
  "runtime.builtin": { "io": "disable", "debug": "disable", "package": "disable" },
  "workspace.library": ["types"],
  "workspace.checkThirdParty": false
}
```

Test helpers already in `tests/support.lua`: `native(name, fn)` defines a recording double; `eq`, `fails(fn, fragment)`,
`expectCall(name, ...)` (checks the most recent call to `name` exactly), `callCount(name)`, `totalCalls()`,
`callName(index)`, `resetCalls()`; `checkGetters(wrapper, {{native, method, returnValue, args...}})`,
`checkSetters(wrapper, {{native, method, args...}})` (the native receives the handle then the args; a row with boolean
arguments runs again with each flipped) and `checkDisposed(wrapper, methods)` (calls each method with no arguments and
expects `disposed`). `PLAYER_RAW` is the raw handle every `Player(i)` double returns, so `Player.fromIndex(0)` wraps it;
`Player.fromHandle({})` is "another player". Each `test` resets the call log and `PRINTED` (the lines `print` wrote).

## Review focus

1. Nothing validates late: bad receivers, arguments and options fail before any native that changes state (every task).
2. Children: a disposed DialogButton's callback can never run; `clear`/`destroy` dispose every child; children have no
   `destroy`/`fromHandle` (Tasks 1, 4).
3. Every `MultiboardGetItem` is released in the same call; ranges are checked before any item is obtained; one-based
   indexes become zero-based exactly once (Task 2).
4. Local visibility changes only local visuals; no getter for machine-local state exists (Tasks 2, 5).
5. Import graph: only `dialog.lua` loads another public module, and only `wrappers.player` (Task 6).

## Files and interfaces

| File                                                    | Responsibility                                   |
| ------------------------------------------------------- | ------------------------------------------------ |
| `src/wrappers/dialog.lua`                               | Dialog, DialogButton (spec §5)                   |
| `src/wrappers/multiboard.lua`                           | Multiboard (spec §7)                             |
| `src/wrappers/leaderboard.lua`                          | Leaderboard (spec §6)                            |
| `src/wrappers/quest.lua`                                | Quest, QuestItem (spec §8.1)                     |
| `src/wrappers/defeatcondition.lua`                      | DefeatCondition (spec §8.2)                      |
| `src/wrappers/timerdialog.lua`                          | TimerDialog (spec §8.3)                          |
| `tests/{dialog,multiboard,leaderboard,quest,defeatcondition,timerdialog}.lua` | New suites                 |
| `tests/imports.lua`, `tools/integration.ts`, `tests/editor-*.{lua,yue}` | Import graph, bundle and editor coverage |
| `examples/gate.yue`, `README.md`, `CHANGELOG.md`, `CONTRIBUTING.md`, `AGENTS.md` | Gate example and library docs |
| Moonwell `AGENTS.md`                                    | State, next work and backlog                     |
| `../wrappers-gate/gate.ts`, `src/probe_ui.yue`, `PROBE-UI.md` | Gate runs `ui`/`ui-min` and the `ui-init` probe (not under git) |

Interfaces used from earlier releases (unchanged):

- `Handle.new(class, name, options?) -> registry` with `registry.wrap(raw)`, `registry.require(value, operation)`,
  `registry.dispose(value, operation)` (returns the raw handle the first time, nil afterwards; sets `.handle = nil`),
  `registry.isDisposed(value, operation)` (raises `expected <Name> wrapper` for a non-member).
- `Handle.unwrap(value, name, operation)`: raises `[wrappers] <operation>: expected <Name> wrapper` or
  `... <Name> is disposed`.
- `Handle.created(raw, operation)`: returns `raw`, or raises `[wrappers] <operation>: native returned nil`.
- `Options.read(options, fields, operation) -> table`: kinds `'boolean'`, `'string'`, `'number'`, `'integer'`,
  `'nonnegative'`, `'color'` (returns a fresh `{r, g, b, a}`, `a` defaulting to 255) and `'Player'`. Errors:
  `expected an options table`, `unknown option '<key>'`, `option '<name>' expected a boolean` (`a string`, `a number`,
  `an integer`, `a finite non-negative number`), `option '<name>' expected {r, g, b, a?} integers`.
- `Callback.check(value, operation)`, `Callback.optional(value, operation)` (raise
  `[wrappers] <operation>: expected a callback function`), `Callback.call(label, fn, argument)` (runs under pcall and
  prints `[wrappers] <label> callback failed: <message>` on error).

---

### Task 1: Dialog and DialogButton

**Files:**
- Create: `src/wrappers/dialog.lua`
- Test: `tests/dialog.lua`

**Interfaces:**
- Consumes: `Handle`, `Callback`, `Options` (internal), `wrappers.player` (`PlayerWrapper.fromHandle(raw)`).
- Produces: `Dialog.fromHandle(raw)`, `Dialog.create()`, `dialog:getHandle()`, `dialog:isDisposed()`,
  `dialog:setMessage(text)`, `dialog:addButton(text, options?|callback?, callback?) -> DialogButton`,
  `dialog:show(player)`, `dialog:hide(player)`, `dialog:clear()`, `dialog:destroy()`; `button:getHandle()`,
  `button:isDisposed()`, `button:getDialog()`. Types `MoonwellWrappers.Dialog`, `MoonwellWrappers.DialogButton`,
  `MoonwellWrappers.DialogButtonOptions`, alias `MoonwellWrappers.DialogButtonCallback`.

- [ ] **Step 1: Write the failing test**

Create `tests/dialog.lua`:

```lua
local actions, lastTrigger = {}, nil
native('DialogCreate', function() return {} end)
native('DialogAddButton', function() return {} end)
native('DialogAddQuitButton', function() return {} end)
native('CreateTrigger', function() lastTrigger = {}; return lastTrigger end)
native('TriggerAddAction', function(_, callback) actions[#actions + 1] = callback; return {} end)
for _, name in ipairs({'DialogSetMessage', 'DialogDisplay', 'DialogClear', 'DialogDestroy', 'DestroyTrigger',
    'TriggerRegisterDialogEvent'}) do
    native(name, function() end)
end
local clicked, clicker = nil, PLAYER_RAW
native('GetClickedButton', function() return clicked end)
native('GetTriggerPlayer', function() return clicker end)
local Dialog = require('wrappers.dialog')
local Player = require('wrappers.player')
eq(totalCalls(), 0)

-- Simulates Warcraft firing every dialog trigger for a click on the raw button `raw` by the raw player `who`.
local function click(raw, who)
    clicked, clicker = raw, who or PLAYER_RAW
    for _, action in ipairs(actions) do action() end
end

test('dialogs keep identity and forward exact natives', function()
    eq(Dialog.fromHandle(nil), nil)
    local d = Dialog.create()
    eq(Dialog.fromHandle(d.handle), d); eq(d:getHandle(), d.handle); eq(d:isDisposed(), false)
    d:setMessage('Choose'); expectCall('DialogSetMessage', d.handle, 'Choose')
    local p = Player.fromIndex(0)
    d:show(p); expectCall('DialogDisplay', p.handle, d.handle, true)
    d:hide(p); expectCall('DialogDisplay', p.handle, d.handle, false)
    fails(function() d:show(d) end, 'Dialog.show: expected Player wrapper')
    fails(function() d:hide(d) end, 'Dialog.hide: expected Player wrapper')
    eq(callCount('DialogDisplay'), 2)
    d:destroy()
end)

test('addButton accepts a callback, options, or options and a callback', function()
    local d = Dialog.create()
    local plain = d:addButton('Plain')
    expectCall('DialogAddButton', d.handle, 'Plain', 0)
    eq(plain:getDialog(), d); eq(plain:isDisposed(), false); eq(plain:getHandle(), plain.handle)
    d:addButton('Keyed', {hotkey = 's'}); expectCall('DialogAddButton', d.handle, 'Keyed', string.byte('S'))
    d:addButton('Digit', {hotkey = '7'}, function() end)
    expectCall('DialogAddButton', d.handle, 'Digit', string.byte('7'))
    d:addButton('Called', function() end); expectCall('DialogAddButton', d.handle, 'Called', 0)
    d:addButton('Quit', {quit = true}); expectCall('DialogAddQuitButton', d.handle, false, 'Quit', 0)
    d:addButton('Score', {quit = true, scoreScreen = true, hotkey = 'Q'})
    expectCall('DialogAddQuitButton', d.handle, true, 'Score', string.byte('Q'))
    d:destroy()
end)

test('the internal trigger is created once, and only for a callback', function()
    local d = Dialog.create()
    d:addButton('A'); d:addButton('B', {hotkey = 'B'})
    eq(callCount('CreateTrigger'), 0)
    d:addButton('C', function() end); d:addButton('D', nil, function() end)
    eq(callCount('CreateTrigger'), 1); eq(callCount('TriggerRegisterDialogEvent'), 1)
    eq(callCount('TriggerAddAction'), 1)
    expectCall('TriggerRegisterDialogEvent', lastTrigger, d.handle)
    local trigger = lastTrigger
    d:destroy(); expectCall('DestroyTrigger', trigger)
    local quiet = Dialog.create()
    quiet:destroy(); eq(callCount('DestroyTrigger'), 1)
end)

test('bad buttons fail before any native', function()
    local d = Dialog.create()
    resetCalls()
    fails(function() d:addButton('x', {hotkey = 'ab'}) end, "Dialog.addButton: option 'hotkey' expected one letter or digit")
    fails(function() d:addButton('x', {hotkey = '!'}) end, "option 'hotkey' expected one letter or digit")
    fails(function() d:addButton('x', {hotkey = ''}) end, "option 'hotkey' expected one letter or digit")
    fails(function() d:addButton('x', {hotkey = 5}) end, "option 'hotkey' expected a string")
    fails(function() d:addButton('x', {scoreScreen = true}) end, "Dialog.addButton: option 'scoreScreen' needs 'quit'")
    fails(function() d:addButton('x', {colour = 1}) end, "unknown option 'colour'")
    fails(function() d:addButton('x', nil, 5) end, 'Dialog.addButton: expected a callback function')
    fails(function() d:addButton('x', function() end, function() end) end, 'Dialog.addButton: a callback given twice')
    fails(function() d:addButton('x', 5) end, 'Dialog.addButton: expected an options table')
    eq(totalCalls(), 0)
    d:destroy()
end)

test('a click runs that button callback with the clicking player', function()
    local d = Dialog.create()
    local seen = {}
    local stay = d:addButton('Stay', function(player) seen[#seen + 1] = {'stay', player} end)
    local leave = d:addButton('Leave', {hotkey = 'L'}, function(player) seen[#seen + 1] = {'leave', player} end)
    local silent = d:addButton('Silent')
    local other = {}
    click(stay.handle); click(leave.handle, other); click(silent.handle); click({})
    eq(#seen, 2)
    eq(seen[1][1], 'stay'); eq(seen[1][2], Player.fromIndex(0))
    eq(seen[2][1], 'leave'); eq(seen[2][2], Player.fromHandle(other))
    d:destroy()
end)

test('a failing callback is printed and later clicks still run', function()
    local d = Dialog.create()
    local count = 0
    local bad = d:addButton('Bad', function() error('intentional button probe') end)
    local good = d:addButton('Good', function() count = count + 1 end)
    click(bad.handle); click(good.handle); click(bad.handle); click(good.handle)
    eq(count, 2); eq(#PRINTED, 2)
    assert(PRINTED[1]:find('[wrappers] Dialog button callback failed:', 1, true), PRINTED[1])
    assert(PRINTED[1]:find('intentional button probe', 1, true), PRINTED[1])
    d:destroy()
end)

test('clear and destroy dispose the buttons, so later clicks run nothing', function()
    local d = Dialog.create()
    local count = 0
    local first = d:addButton('First', function() count = count + 1 end)
    local firstRaw = first.handle
    d:clear()
    expectCall('DialogClear', d.handle)
    eq(first:isDisposed(), true); eq(first.handle, nil); eq(first:getDialog(), d)
    fails(function() first:getHandle() end, 'DialogButton is disposed')
    click(firstRaw); eq(count, 0)
    local second = d:addButton('Second', function() count = count + 10 end)
    eq(callCount('CreateTrigger'), 1)
    click(second.handle); eq(count, 10)
    local secondRaw, raw = second.handle, d.handle
    d:destroy(); d:destroy()
    eq(callCount('DialogDestroy'), 1); expectCall('DialogDestroy', raw)
    eq(second:isDisposed(), true); eq(d:isDisposed(), true); eq(d.handle, nil)
    click(secondRaw); eq(count, 10)
    checkDisposed(d, {'getHandle', 'setMessage', 'addButton', 'show', 'hide', 'clear'})
end)

test('a callback may hide, clear or destroy its own dialog', function()
    local d = Dialog.create()
    local p = Player.fromIndex(0)
    local after = 0
    local hide = d:addButton('Hide', function(player) d:hide(player) end)
    local clear = d:addButton('Clear', function() d:clear() end)
    local trigger, raw = lastTrigger, d.handle
    click(hide.handle); expectCall('DialogDisplay', p.handle, raw, false)
    click(clear.handle); eq(clear:isDisposed(), true); eq(hide:isDisposed(), true)
    local destroy = d:addButton('Destroy', function() d:destroy(); after = after + 1 end)
    click(destroy.handle)
    eq(after, 1); eq(d:isDisposed(), true); eq(destroy:isDisposed(), true)
    expectCall('DestroyTrigger', trigger); expectCall('DialogDestroy', raw)
    eq(#PRINTED, 0)
end)

test('buttons have no destroy or fromHandle, and getDialog checks its receiver', function()
    local d = Dialog.create()
    local b = d:addButton('B')
    eq(b.destroy, nil); eq(b.fromHandle, nil)
    fails(function() b.getDialog(d) end, 'DialogButton.getDialog: expected DialogButton wrapper')
    d:destroy()
end)

test('create and addButton fail clearly when the native returns nil', function()
    native('DialogCreate', function() return nil end)
    fails(function() Dialog.create() end, 'Dialog.create: native returned nil')
    native('DialogCreate', function() return {} end)
    local d = Dialog.create()
    native('DialogAddButton', function() return nil end)
    fails(function() d:addButton('x') end, 'Dialog.addButton: native returned nil')
    native('DialogAddButton', function() return {} end)
    d:destroy()
end)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `deno task test dialog`
Expected: FAIL, `module 'wrappers.dialog' not found`.

- [ ] **Step 3: Write the implementation**

Create `src/wrappers/dialog.lua`:

```lua
local Handle = require('wrappers.internal.handle')
local Callback = require('wrappers.internal.callback')
local Options = require('wrappers.internal.options')
local PlayerWrapper = require('wrappers.player')

---@class MoonwellWrappers.Dialog
---@field handle dialog? Read-only by convention; nil after destruction.
local Dialog = {}
---@type MoonwellWrappers.Registry<MoonwellWrappers.Dialog, dialog>
local registry = Handle.new(Dialog, 'Dialog')

---A button belongs to the dialog that made it: the dialog's clear() and destroy() dispose it.
---@class MoonwellWrappers.DialogButton
---@field handle button? Read-only by convention; nil once its dialog is cleared or destroyed.
local DialogButton = {}
---@type MoonwellWrappers.Registry<MoonwellWrappers.DialogButton, button>
local buttonRegistry = Handle.new(DialogButton, 'DialogButton')

---@alias MoonwellWrappers.DialogButtonCallback fun(player: MoonwellWrappers.Player): ...

---@class MoonwellWrappers.DialogButtonOptions
---@field hotkey string? One letter or digit that clicks the button.
---@field quit boolean? The button quits the game for the clicking player; default false.
---@field scoreScreen boolean? With quit, show the score screen first; default false.

---@type MoonwellWrappers.OptionFields
local buttonFields = {hotkey = {'string'}, quit = {'boolean', false}, scoreScreen = {'boolean', false}}

---@class MoonwellWrappers.DialogState
---@field buttons MoonwellWrappers.DialogButton[] Live buttons in creation order.
---@field byHandle table<button, MoonwellWrappers.DialogButton> Click lookup only; never iterated.
---@field trigger trigger? Created by the first button with a callback.

-- Keyed by wrapper; only indexed, never iterated.
---@type table<MoonwellWrappers.Dialog, MoonwellWrappers.DialogState>
local states = {}
---@type table<MoonwellWrappers.DialogButton, {dialog: MoonwellWrappers.Dialog, callback: function?}>
local buttonInfo = setmetatable({}, {__mode = 'k'})

---@param dialog MoonwellWrappers.Dialog
---@return MoonwellWrappers.DialogState
local function stateOf(dialog)
    local state = states[dialog]
    if not state then
        state = {buttons = {}, byHandle = {}}
        states[dialog] = state
    end
    return state
end

---Disposes every button first, so no callback can run for a button the game has removed.
---@param state MoonwellWrappers.DialogState
local function disposeButtons(state)
    for _, button in ipairs(state.buttons) do
        buttonInfo[button].callback = nil
        buttonRegistry.dispose(button, 'Dialog.clear')
    end
    state.buttons, state.byHandle = {}, {}
end

---The dialog trigger's action: finds the clicked button among the dialog's live buttons.
---@param dialog MoonwellWrappers.Dialog
local function route(dialog)
    local state = states[dialog]
    if not state then return end
    local button = state.byHandle[GetClickedButton()]
    if not button then return end
    local callback = buttonInfo[button].callback
    if callback then Callback.call('Dialog button', callback, PlayerWrapper.fromHandle(GetTriggerPlayer())) end
end

---@param raw dialog?
---@return MoonwellWrappers.Dialog?
---@overload fun(raw: nil): nil
function Dialog.fromHandle(raw) return registry.wrap(raw) end
---@return MoonwellWrappers.Dialog
function Dialog.create() return Handle.created(Dialog.fromHandle(DialogCreate()), 'Dialog.create') end
---@return dialog
function Dialog:getHandle() return registry.require(self, 'Dialog.getHandle') end
---@return boolean
function Dialog:isDisposed() return registry.isDisposed(self, 'Dialog.isDisposed') end
---@param text string
function Dialog:setMessage(text) DialogSetMessage(registry.require(self, 'Dialog.setMessage'), text) end

---Adds a button. Pass a callback as the second argument, or options and then an optional callback. The callback
---receives the Player who clicked and runs behind the callback boundary: an error is printed, later clicks still run.
---@param text string
---@param options MoonwellWrappers.DialogButtonOptions|MoonwellWrappers.DialogButtonCallback|nil
---@param callback MoonwellWrappers.DialogButtonCallback?
---@return MoonwellWrappers.DialogButton
function Dialog:addButton(text, options, callback)
    local raw = registry.require(self, 'Dialog.addButton')
    local settings, handler = options, callback
    if type(options) == 'function' then
        if callback ~= nil then error('[wrappers] Dialog.addButton: a callback given twice', 2) end
        settings, handler = nil, options
    end
    Callback.optional(handler, 'Dialog.addButton')
    local o = Options.read(settings, buttonFields, 'Dialog.addButton')
    local hotkey = 0
    if o.hotkey ~= nil then
        if not o.hotkey:match('^[A-Za-z0-9]$') then
            error("[wrappers] Dialog.addButton: option 'hotkey' expected one letter or digit", 2)
        end
        hotkey = string.byte(o.hotkey:upper())
    end
    if o.scoreScreen and not o.quit then error("[wrappers] Dialog.addButton: option 'scoreScreen' needs 'quit'", 2) end
    local state = stateOf(self)
    if handler and not state.trigger then
        local trigger = Handle.created(CreateTrigger(), 'Dialog.addButton')
        TriggerRegisterDialogEvent(trigger, raw)
        TriggerAddAction(trigger, function() route(self) end)
        state.trigger = trigger
    end
    local buttonRaw
    if o.quit then
        buttonRaw = DialogAddQuitButton(raw, o.scoreScreen, text, hotkey)
    else
        buttonRaw = DialogAddButton(raw, text, hotkey)
    end
    local button = Handle.created(buttonRegistry.wrap(buttonRaw), 'Dialog.addButton')
    buttonInfo[button] = {dialog = self, callback = handler}
    state.buttons[#state.buttons + 1] = button
    state.byHandle[buttonRaw] = button
    return button
end

---Shows the dialog to one player; every machine makes the same call.
---@param player MoonwellWrappers.Player
function Dialog:show(player)
    local raw = registry.require(self, 'Dialog.show')
    DialogDisplay(Handle.unwrap(player, 'Player', 'Dialog.show'), raw, true)
end
---@param player MoonwellWrappers.Player
function Dialog:hide(player)
    local raw = registry.require(self, 'Dialog.hide')
    DialogDisplay(Handle.unwrap(player, 'Player', 'Dialog.hide'), raw, false)
end
---Removes every button and disposes their wrappers. The dialog stays usable.
function Dialog:clear()
    local raw = registry.require(self, 'Dialog.clear')
    local state = states[self]
    if state then disposeButtons(state) end
    DialogClear(raw)
end
function Dialog:destroy()
    local raw = registry.dispose(self, 'Dialog.destroy')
    if not raw then return end
    local state = states[self]
    states[self] = nil
    if state then
        disposeButtons(state)
        if state.trigger then DestroyTrigger(state.trigger) end
    end
    DialogDestroy(raw)
end

---@return button
function DialogButton:getHandle() return buttonRegistry.require(self, 'DialogButton.getHandle') end
---@return boolean
function DialogButton:isDisposed() return buttonRegistry.isDisposed(self, 'DialogButton.isDisposed') end
---The dialog that made this button; still answers after the button is disposed.
---@return MoonwellWrappers.Dialog
function DialogButton:getDialog()
    buttonRegistry.isDisposed(self, 'DialogButton.getDialog')
    return buttonInfo[self].dialog
end

return Dialog
```

- [ ] **Step 4: Run test to verify it passes**

Run: `deno task test dialog`
Expected: `dialog: SUITE PASSED: 10 tests`.

- [ ] **Step 5: Run all checks and commit**

Run every command in "Running the wrappers checks", including the direct LuaLS run. Expected: all pass (the direct
LuaLS run reports no problems). If LuaLS flags the reassignment of `settings`/`handler`, narrow with a local
`---@cast` on that line rather than changing behavior.

```bash
git add src/wrappers/dialog.lua tests/dialog.lua
git commit -m "feat: Dialog with per-button callbacks and buttons owned by their dialog"
```

---

### Task 2: Multiboard

**Files:**
- Create: `src/wrappers/multiboard.lua`
- Test: `tests/multiboard.lua`

**Interfaces:**
- Consumes: `Handle`, `Options` (internal).
- Produces: `Multiboard.fromHandle(raw)`, `Multiboard.create(rows, columns, title?)`,
  `Multiboard.suppressDisplay(flag)`, and methods `getHandle`, `isDisposed`, `setRowCount(count)`,
  `setColumnCount(count)`, `getRowCount()`, `getColumnCount()`, `setTitle(text)`, `getTitle()`,
  `setTitleColor(r, g, b, a)`, `setCell(row, column, options)`, `setRow(row, options)`, `setColumn(column, options)`,
  `setAll(options)`, `show(flag)`, `setVisibleFor(player)`, `minimize(flag)`, `destroy()`. Type
  `MoonwellWrappers.MultiboardCellOptions`.

- [ ] **Step 1: Write the failing test**

Create `tests/multiboard.lua`:

```lua
local rows, columns, steps, items, released = {}, {}, {}, {}, {}
local function newBoard() local raw = {}; rows[raw], columns[raw] = 0, 0; return raw end
native('CreateMultiboard', newBoard)
native('MultiboardSetRowCount', function(raw, count) rows[raw] = count; steps[#steps + 1] = count end)
native('MultiboardSetColumnCount', function(raw, count) columns[raw] = count end)
native('MultiboardGetRowCount', function(raw) return rows[raw] end)
native('MultiboardGetColumnCount', function(raw) return columns[raw] end)
native('MultiboardGetItem', function(_, row, column)
    local item = {row = row, column = column}
    items[#items + 1] = item
    return item
end)
native('MultiboardReleaseItem', function(item) released[#released + 1] = item end)
for _, name in ipairs({'MultiboardSetTitleText', 'MultiboardSetTitleTextColor', 'MultiboardSetItemValue',
    'MultiboardSetItemValueColor', 'MultiboardSetItemIcon', 'MultiboardSetItemStyle', 'MultiboardSetItemWidth',
    'MultiboardSetItemsValue', 'MultiboardSetItemsValueColor', 'MultiboardSetItemsIcon', 'MultiboardSetItemsStyle',
    'MultiboardSetItemsWidth', 'MultiboardDisplay', 'MultiboardMinimize', 'MultiboardSuppressDisplay',
    'DestroyMultiboard'}) do
    native(name, function() end)
end
native('MultiboardGetTitleText', function() return '' end)
native('GetLocalPlayer', function() return PLAYER_RAW end)
local Multiboard = require('wrappers.multiboard')
local Player = require('wrappers.player')
eq(totalCalls(), 0)

local function names()
    local list = {}
    for index = 1, totalCalls() do list[#list + 1] = callName(index) end
    return table.concat(list, ' ')
end

test('create sets columns, steps rows one at a time and sets the title', function()
    steps = {}
    local board = Multiboard.create(3, 2, 'Scores')
    expectCall('MultiboardSetColumnCount', board.handle, 2)
    eq(table.concat(steps, ','), '1,2,3')
    expectCall('MultiboardSetTitleText', board.handle, 'Scores')
    eq(board:getRowCount(), 3); eq(board:getColumnCount(), 2)
    eq(Multiboard.fromHandle(board.handle), board); eq(Multiboard.fromHandle(nil), nil)
    eq(board:getHandle(), board.handle); eq(board:isDisposed(), false)
    local untitled = Multiboard.create(0, 0)
    eq(callCount('MultiboardSetTitleText'), 1)
    board:destroy(); untitled:destroy()
end)

test('setRowCount steps one row at a time, up and down', function()
    local board = Multiboard.create(3, 1)
    steps = {}
    board:setRowCount(6); eq(table.concat(steps, ','), '4,5,6')
    steps = {}
    board:setRowCount(2); eq(table.concat(steps, ','), '5,4,3,2')
    steps = {}
    board:setRowCount(2); eq(#steps, 0)
    board:setColumnCount(4); expectCall('MultiboardSetColumnCount', board.handle, 4)
    board:destroy()
end)

test('counts must be non-negative integers', function()
    fails(function() Multiboard.create(-1, 2) end, 'Multiboard.create: expected a non-negative integer row count')
    fails(function() Multiboard.create(1, 1.5) end, 'Multiboard.create: expected a non-negative integer column count')
    fails(function() Multiboard.create('3', 1) end, 'Multiboard.create: expected a non-negative integer row count')
    eq(totalCalls(), 0)
    local board = Multiboard.create(1, 1)
    resetCalls()
    fails(function() board:setRowCount('3') end, 'Multiboard.setRowCount: expected a non-negative integer row count')
    fails(function() board:setRowCount(math.huge) end, 'expected a non-negative integer row count')
    fails(function() board:setColumnCount(-2) end,
        'Multiboard.setColumnCount: expected a non-negative integer column count')
    eq(callCount('MultiboardSetRowCount'), 0); eq(callCount('MultiboardSetColumnCount'), 0)
    board:destroy()
end)

test('setCell converts to zero-based, applies options in order and releases the item', function()
    local board = Multiboard.create(2, 3)
    items, released = {}, {}
    resetCalls()
    board:setCell(2, 3, {value = 'Arthas', color = {255, 204, 0}, icon = 'arthas.blp', showValue = true,
        showIcon = false, width = 0.1})
    eq(#items, 1); eq(items[1].row, 1); eq(items[1].column, 2)
    local item = items[1]
    expectCall('MultiboardSetItemValue', item, 'Arthas')
    expectCall('MultiboardSetItemValueColor', item, 255, 204, 0, 255)
    expectCall('MultiboardSetItemIcon', item, 'arthas.blp')
    expectCall('MultiboardSetItemStyle', item, true, false)
    expectCall('MultiboardSetItemWidth', item, 0.1)
    eq(#released, 1); eq(released[1], item)
    eq(names(), 'MultiboardGetRowCount MultiboardGetColumnCount MultiboardGetItem MultiboardSetItemValue ' ..
        'MultiboardSetItemValueColor MultiboardSetItemIcon MultiboardSetItemStyle MultiboardSetItemWidth ' ..
        'MultiboardReleaseItem')
    resetCalls()
    board:setCell(1, 1, {value = 'x'})
    eq(names(), 'MultiboardGetRowCount MultiboardGetColumnCount MultiboardGetItem MultiboardSetItemValue ' ..
        'MultiboardReleaseItem')
    board:setCell(1, 1, {showValue = false, showIcon = true})
    expectCall('MultiboardSetItemStyle', items[#items], false, true)
    board:destroy()
end)

test('bad cells fail before any item is obtained', function()
    local board = Multiboard.create(4, 2)
    resetCalls()
    fails(function() board:setCell(5, 1, {value = 'x'}) end, 'Multiboard.setCell: row 5 outside 1..4')
    fails(function() board:setCell(0, 1, {value = 'x'}) end, 'Multiboard.setCell: row 0 outside 1..4')
    fails(function() board:setCell(1, 3, {value = 'x'}) end, 'Multiboard.setCell: column 3 outside 1..2')
    fails(function() board:setCell(1.5, 1, {value = 'x'}) end, 'Multiboard.setCell: row 1.5 outside 1..4')
    fails(function() board:setCell('1', 1, {value = 'x'}) end, 'Multiboard.setCell: row 1 outside 1..4')
    fails(function() board:setRow(9, {value = 'x'}) end, 'Multiboard.setRow: row 9 outside 1..4')
    fails(function() board:setColumn(3, {value = 'x'}) end, 'Multiboard.setColumn: column 3 outside 1..2')
    eq(callCount('MultiboardGetItem'), 0)
    resetCalls()
    fails(function() board:setCell(1, 1, {}) end, 'Multiboard.setCell: expected at least one option')
    fails(function() board:setCell(1, 1) end, 'Multiboard.setCell: expected at least one option')
    fails(function() board:setAll({showValue = true}) end,
        "Multiboard.setAll: options 'showValue' and 'showIcon' go together")
    fails(function() board:setRow(1, {showIcon = false}) end, "options 'showValue' and 'showIcon' go together")
    fails(function() board:setCell(1, 1, {colour = {1, 2, 3}}) end, "unknown option 'colour'")
    fails(function() board:setCell(1, 1, {width = -1}) end, "option 'width' expected a finite non-negative number")
    eq(totalCalls(), 0)
    board:destroy()
end)

test('setRow and setColumn set each cell in ascending order and release each item', function()
    local board = Multiboard.create(2, 3)
    items, released = {}, {}
    board:setRow(2, {value = 'r'})
    eq(#items, 3); eq(#released, 3)
    for index, item in ipairs(items) do eq(item.row, 1); eq(item.column, index - 1); eq(released[index], item) end
    items, released = {}, {}
    board:setColumn(3, {icon = 'i.blp'})
    eq(#items, 2); eq(#released, 2)
    for index, item in ipairs(items) do eq(item.row, index - 1); eq(item.column, 2); eq(released[index], item) end
    board:destroy()
end)

test('setAll uses the whole-board natives', function()
    local board = Multiboard.create(2, 2)
    resetCalls()
    board:setAll({value = '-', color = {1, 2, 3, 4}, icon = 'x.blp', showValue = true, showIcon = false, width = 0.05})
    expectCall('MultiboardSetItemsValue', board.handle, '-')
    expectCall('MultiboardSetItemsValueColor', board.handle, 1, 2, 3, 4)
    expectCall('MultiboardSetItemsIcon', board.handle, 'x.blp')
    expectCall('MultiboardSetItemsStyle', board.handle, true, false)
    expectCall('MultiboardSetItemsWidth', board.handle, 0.05)
    eq(callCount('MultiboardGetItem'), 0); eq(totalCalls(), 5)
    board:destroy()
end)

test('title, display and minimize forward exact arguments', function()
    local board = Multiboard.create(1, 1)
    checkSetters(board, {{'MultiboardSetTitleText', 'setTitle', 'Kills'},
        {'MultiboardSetTitleTextColor', 'setTitleColor', 1, 2, 3, 4}, {'MultiboardDisplay', 'show', true},
        {'MultiboardMinimize', 'minimize', false}})
    checkGetters(board, {{'MultiboardGetTitleText', 'getTitle', 'Kills'}})
    Multiboard.suppressDisplay(true); expectCall('MultiboardSuppressDisplay', true)
    Multiboard.suppressDisplay(false); expectCall('MultiboardSuppressDisplay', false)
    board:setVisibleFor(Player.fromIndex(0)); expectCall('MultiboardDisplay', board.handle, true)
    board:setVisibleFor(Player.fromHandle({})); expectCall('MultiboardDisplay', board.handle, false)
    fails(function() board:setVisibleFor(board) end, 'Multiboard.setVisibleFor: expected Player wrapper')
    eq(board.isMinimized, nil); eq(board.isDisplayed, nil)
    board:destroy()
end)

test('destroy is idempotent and guards every method', function()
    local board = Multiboard.create(1, 1)
    local raw = board.handle
    board:destroy(); board:destroy()
    expectCall('DestroyMultiboard', raw); eq(callCount('DestroyMultiboard'), 1)
    eq(board.handle, nil); eq(board:isDisposed(), true)
    checkDisposed(board, {'getHandle', 'setRowCount', 'setColumnCount', 'getRowCount', 'getColumnCount', 'setTitle',
        'getTitle', 'setTitleColor', 'setCell', 'setRow', 'setColumn', 'setAll', 'show', 'setVisibleFor', 'minimize'})
end)

test('create fails clearly when the native returns nil', function()
    native('CreateMultiboard', function() return nil end)
    fails(function() Multiboard.create(1, 1) end, 'Multiboard.create: native returned nil')
    eq(callCount('MultiboardSetColumnCount'), 0)
    native('CreateMultiboard', newBoard)
end)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `deno task test multiboard`
Expected: FAIL, `module 'wrappers.multiboard' not found`.

- [ ] **Step 3: Write the implementation**

Create `src/wrappers/multiboard.lua`:

```lua
local Handle = require('wrappers.internal.handle')
local Options = require('wrappers.internal.options')

---Rows and columns count from 1. Cell handles are obtained and released inside each call, so none can leak.
---@class MoonwellWrappers.Multiboard
---@field handle multiboard? Read-only by convention; nil after destruction.
local Multiboard = {}
---@type MoonwellWrappers.Registry<MoonwellWrappers.Multiboard, multiboard>
local registry = Handle.new(Multiboard, 'Multiboard')

---@class MoonwellWrappers.MultiboardCellOptions
---@field value string? The cell's text.
---@field color integer[]? {r, g, b, a?}, each 0-255, for the text.
---@field icon string? Icon path.
---@field showValue boolean? Show the text; give together with showIcon.
---@field showIcon boolean? Show the icon; give together with showValue.
---@field width number? Width as a fraction of the screen width.

---@type MoonwellWrappers.OptionFields
local cellFields = {
    value = {'string'}, color = {'color'}, icon = {'string'}, showValue = {'boolean'}, showIcon = {'boolean'},
    width = {'nonnegative'},
}

---Level 3 blames the caller of the public method.
---@param options unknown
---@param operation string
---@return table<string, any>
local function cellOptions(options, operation)
    local o = Options.read(options, cellFields, operation)
    if o.value == nil and o.color == nil and o.icon == nil and o.showValue == nil and o.showIcon == nil
        and o.width == nil then
        error('[wrappers] ' .. operation .. ': expected at least one option', 3)
    end
    if (o.showValue == nil) ~= (o.showIcon == nil) then
        error('[wrappers] ' .. operation .. ": options 'showValue' and 'showIcon' go together", 3)
    end
    return o
end

---@param value unknown
---@param what string
---@param operation string
local function checkCount(value, what, operation)
    if type(value) ~= 'number' or value % 1 ~= 0 or value < 0 then
        error('[wrappers] ' .. operation .. ': expected a non-negative integer ' .. what, 3)
    end
end

---@param value unknown
---@param count integer
---@param what string
---@param operation string
local function checkIndex(value, count, what, operation)
    if type(value) ~= 'number' or value % 1 ~= 0 or value < 1 or value > count then
        error('[wrappers] ' .. operation .. ': ' .. what .. ' ' .. tostring(value) .. ' outside 1..' .. count, 3)
    end
end

---Gets one cell (one-based), applies the options in a fixed order and releases the cell handle.
---@param raw multiboard
---@param row integer
---@param column integer
---@param o table<string, any>
local function setCell(raw, row, column, o)
    local item = MultiboardGetItem(raw, row - 1, column - 1)
    if o.value ~= nil then MultiboardSetItemValue(item, o.value) end
    if o.color ~= nil then MultiboardSetItemValueColor(item, o.color[1], o.color[2], o.color[3], o.color[4]) end
    if o.icon ~= nil then MultiboardSetItemIcon(item, o.icon) end
    if o.showValue ~= nil then MultiboardSetItemStyle(item, o.showValue, o.showIcon) end
    if o.width ~= nil then MultiboardSetItemWidth(item, o.width) end
    MultiboardReleaseItem(item)
end

---@param raw multiboard?
---@return MoonwellWrappers.Multiboard?
---@overload fun(raw: nil): nil
function Multiboard.fromHandle(raw) return registry.wrap(raw) end
---@param rows integer
---@param columns integer
---@param title string?
---@return MoonwellWrappers.Multiboard
function Multiboard.create(rows, columns, title)
    checkCount(rows, 'row count', 'Multiboard.create')
    checkCount(columns, 'column count', 'Multiboard.create')
    local raw = CreateMultiboard()
    local board = Handle.created(Multiboard.fromHandle(raw), 'Multiboard.create')
    MultiboardSetColumnCount(raw, columns)
    board:setRowCount(rows)
    if title ~= nil then MultiboardSetTitleText(raw, title) end
    return board
end
---Hides (true) or allows (false) every multiboard, for everyone.
---@param flag boolean
function Multiboard.suppressDisplay(flag) MultiboardSuppressDisplay(flag) end
---@return multiboard
function Multiboard:getHandle() return registry.require(self, 'Multiboard.getHandle') end
---@return boolean
function Multiboard:isDisposed() return registry.isDisposed(self, 'Multiboard.isDisposed') end
---Changes the row count one row at a time: w3ts reports that bigger steps are unsafe.
---@param count integer
function Multiboard:setRowCount(count)
    local raw = registry.require(self, 'Multiboard.setRowCount')
    checkCount(count, 'row count', 'Multiboard.setRowCount')
    local current = MultiboardGetRowCount(raw)
    local step = count >= current and 1 or -1
    while current ~= count do
        current = current + step
        MultiboardSetRowCount(raw, current)
    end
end
---@param count integer
function Multiboard:setColumnCount(count)
    local raw = registry.require(self, 'Multiboard.setColumnCount')
    checkCount(count, 'column count', 'Multiboard.setColumnCount')
    MultiboardSetColumnCount(raw, count)
end
---@return integer
function Multiboard:getRowCount() return MultiboardGetRowCount(registry.require(self, 'Multiboard.getRowCount')) end
---@return integer
function Multiboard:getColumnCount()
    return MultiboardGetColumnCount(registry.require(self, 'Multiboard.getColumnCount'))
end
---@param text string
function Multiboard:setTitle(text) MultiboardSetTitleText(registry.require(self, 'Multiboard.setTitle'), text) end
---@return string
function Multiboard:getTitle() return MultiboardGetTitleText(registry.require(self, 'Multiboard.getTitle')) end
---@param r integer 0-255
---@param g integer 0-255
---@param b integer 0-255
---@param a integer 0-255
function Multiboard:setTitleColor(r, g, b, a)
    MultiboardSetTitleTextColor(registry.require(self, 'Multiboard.setTitleColor'), r, g, b, a)
end
---@param row integer From 1.
---@param column integer From 1.
---@param options MoonwellWrappers.MultiboardCellOptions
function Multiboard:setCell(row, column, options)
    local raw = registry.require(self, 'Multiboard.setCell')
    local o = cellOptions(options, 'Multiboard.setCell')
    checkIndex(row, MultiboardGetRowCount(raw), 'row', 'Multiboard.setCell')
    checkIndex(column, MultiboardGetColumnCount(raw), 'column', 'Multiboard.setCell')
    setCell(raw, row, column, o)
end
---@param row integer From 1.
---@param options MoonwellWrappers.MultiboardCellOptions
function Multiboard:setRow(row, options)
    local raw = registry.require(self, 'Multiboard.setRow')
    local o = cellOptions(options, 'Multiboard.setRow')
    checkIndex(row, MultiboardGetRowCount(raw), 'row', 'Multiboard.setRow')
    for column = 1, MultiboardGetColumnCount(raw) do setCell(raw, row, column, o) end
end
---@param column integer From 1.
---@param options MoonwellWrappers.MultiboardCellOptions
function Multiboard:setColumn(column, options)
    local raw = registry.require(self, 'Multiboard.setColumn')
    local o = cellOptions(options, 'Multiboard.setColumn')
    checkIndex(column, MultiboardGetColumnCount(raw), 'column', 'Multiboard.setColumn')
    for row = 1, MultiboardGetRowCount(raw) do setCell(raw, row, column, o) end
end
---Applies the options to every cell with the whole-board natives.
---@param options MoonwellWrappers.MultiboardCellOptions
function Multiboard:setAll(options)
    local raw = registry.require(self, 'Multiboard.setAll')
    local o = cellOptions(options, 'Multiboard.setAll')
    if o.value ~= nil then MultiboardSetItemsValue(raw, o.value) end
    if o.color ~= nil then MultiboardSetItemsValueColor(raw, o.color[1], o.color[2], o.color[3], o.color[4]) end
    if o.icon ~= nil then MultiboardSetItemsIcon(raw, o.icon) end
    if o.showValue ~= nil then MultiboardSetItemsStyle(raw, o.showValue, o.showIcon) end
    if o.width ~= nil then MultiboardSetItemsWidth(raw, o.width) end
end
---@param flag boolean
function Multiboard:show(flag) MultiboardDisplay(registry.require(self, 'Multiboard.show'), flag) end
---Shows the multiboard on that player's machine only. Only local visuals differ.
---@param player MoonwellWrappers.Player
function Multiboard:setVisibleFor(player)
    local raw = registry.require(self, 'Multiboard.setVisibleFor')
    MultiboardDisplay(raw, Handle.unwrap(player, 'Player', 'Multiboard.setVisibleFor') == GetLocalPlayer())
end
---@param flag boolean
function Multiboard:minimize(flag) MultiboardMinimize(registry.require(self, 'Multiboard.minimize'), flag) end
function Multiboard:destroy()
    local raw = registry.dispose(self, 'Multiboard.destroy')
    if raw then DestroyMultiboard(raw) end
end

return Multiboard
```

- [ ] **Step 4: Run test to verify it passes**

Run: `deno task test multiboard`
Expected: `multiboard: SUITE PASSED: 10 tests`.

- [ ] **Step 5: Run all checks and commit**

Run every command in "Running the wrappers checks", including the direct LuaLS run. Expected: all pass.

```bash
git add src/wrappers/multiboard.lua tests/multiboard.lua
git commit -m "feat: Multiboard with one-based cell methods that release every cell handle"
```

---

### Task 3: Leaderboard

**Files:**
- Create: `src/wrappers/leaderboard.lua`
- Test: `tests/leaderboard.lua`

**Interfaces:**
- Consumes: `Handle`, `Options` (internal).
- Produces: `Leaderboard.fromHandle(raw)`, `Leaderboard.create(label?)`, and methods `getHandle`, `isDisposed`,
  `setLabel(text)`, `setLabelColor(r, g, b, a)`, `setValueColor(r, g, b, a)`, `setStyle(options?)`,
  `addItem(player, label, value)`, `removeItem(player)`, `setItemValue(player, value)`, `setItemLabel(player, label)`,
  `setItemLabelColor(player, r, g, b, a)`, `setItemValueColor(player, r, g, b, a)`, `setItemStyle(player, options?)`,
  `hasItem(player)`, `getItemCount()`, `sortByValue(ascending)`, `sortByLabel(ascending)`, `sortByPlayer(ascending)`,
  `assign(player)`, `show(flag)`, `destroy()`. Types `MoonwellWrappers.LeaderboardStyleOptions`,
  `MoonwellWrappers.LeaderboardItemStyleOptions`.

- [ ] **Step 1: Write the failing test**

Create `tests/leaderboard.lua`:

```lua
local function position(raw, p)
    for index, q in ipairs(raw.items) do if q == p then return index end end
end
native('CreateLeaderboard', function() return {items = {}} end)
native('LeaderboardHasPlayerItem', function(raw, p) return position(raw, p) ~= nil end)
native('LeaderboardGetPlayerIndex', function(raw, p) return (position(raw, p) or 0) - 1 end)
native('LeaderboardAddItem', function(raw, _, _, p) raw.items[#raw.items + 1] = p end)
native('LeaderboardRemovePlayerItem', function(raw, p) table.remove(raw.items, position(raw, p)) end)
native('LeaderboardGetItemCount', function(raw) return #raw.items end)
for _, name in ipairs({'LeaderboardSetLabel', 'LeaderboardSetLabelColor', 'LeaderboardSetValueColor',
    'LeaderboardSetStyle', 'LeaderboardSetSizeByItemCount', 'LeaderboardSetItemValue', 'LeaderboardSetItemLabel',
    'LeaderboardSetItemLabelColor', 'LeaderboardSetItemValueColor', 'LeaderboardSetItemStyle',
    'LeaderboardSortItemsByValue', 'LeaderboardSortItemsByLabel', 'LeaderboardSortItemsByPlayer',
    'PlayerSetLeaderboard', 'LeaderboardDisplay', 'DestroyLeaderboard'}) do
    native(name, function() end)
end
local Leaderboard = require('wrappers.leaderboard')
local Player = require('wrappers.player')
eq(totalCalls(), 0)

local mutating = {'LeaderboardAddItem', 'LeaderboardRemovePlayerItem', 'LeaderboardSetSizeByItemCount',
    'LeaderboardSetItemValue', 'LeaderboardSetItemLabel', 'LeaderboardSetItemLabelColor',
    'LeaderboardSetItemValueColor', 'LeaderboardSetItemStyle', 'LeaderboardSetStyle'}

test('create sets the optional label and keeps identity', function()
    local board = Leaderboard.create('Kills')
    expectCall('LeaderboardSetLabel', board.handle, 'Kills')
    eq(Leaderboard.fromHandle(board.handle), board); eq(Leaderboard.fromHandle(nil), nil)
    eq(board:getHandle(), board.handle); eq(board:isDisposed(), false)
    local plain = Leaderboard.create()
    eq(callCount('LeaderboardSetLabel'), 1)
    board:destroy(); plain:destroy()
end)

test('items are keyed by player and resize the board', function()
    local board = Leaderboard.create()
    local red, blue = Player.fromIndex(0), Player.fromHandle({})
    board:addItem(red, 'Red', 3)
    expectCall('LeaderboardAddItem', board.handle, 'Red', 3, red.handle)
    expectCall('LeaderboardSetSizeByItemCount', board.handle, 1)
    board:addItem(blue, 'Blue', 5)
    expectCall('LeaderboardSetSizeByItemCount', board.handle, 2)
    eq(board:getItemCount(), 2); eq(board:hasItem(blue), true)
    board:setItemValue(blue, 9); expectCall('LeaderboardSetItemValue', board.handle, 1, 9)
    board:setItemLabel(red, 'R'); expectCall('LeaderboardSetItemLabel', board.handle, 0, 'R')
    board:setItemLabelColor(blue, 1, 2, 3, 4); expectCall('LeaderboardSetItemLabelColor', board.handle, 1, 1, 2, 3, 4)
    board:setItemValueColor(red, 5, 6, 7, 8); expectCall('LeaderboardSetItemValueColor', board.handle, 0, 5, 6, 7, 8)
    board:setItemStyle(blue, {icon = false}); expectCall('LeaderboardSetItemStyle', board.handle, 1, true, true, false)
    board:setItemStyle(red); expectCall('LeaderboardSetItemStyle', board.handle, 0, true, true, true)
    board:removeItem(red)
    expectCall('LeaderboardRemovePlayerItem', board.handle, red.handle)
    expectCall('LeaderboardSetSizeByItemCount', board.handle, 1)
    eq(board:hasItem(red), false)
    board:setItemValue(blue, 1); expectCall('LeaderboardSetItemValue', board.handle, 0, 1)
    board:destroy()
end)

test('one item per player: misuse fails before any change', function()
    local board = Leaderboard.create()
    local red, blue = Player.fromIndex(0), Player.fromHandle({})
    board:addItem(red, 'Red', 3)
    resetCalls()
    fails(function() board:addItem(red, 'Again', 1) end, 'Leaderboard.addItem: player already has an item')
    fails(function() board:removeItem(blue) end, 'Leaderboard.removeItem: player has no item')
    fails(function() board:setItemValue(blue, 1) end, 'Leaderboard.setItemValue: player has no item')
    fails(function() board:setItemLabel(blue, 'x') end, 'Leaderboard.setItemLabel: player has no item')
    fails(function() board:setItemLabelColor(blue, 1, 2, 3, 4) end, 'Leaderboard.setItemLabelColor: player has no item')
    fails(function() board:setItemValueColor(blue, 1, 2, 3, 4) end, 'Leaderboard.setItemValueColor: player has no item')
    fails(function() board:setItemStyle(blue) end, 'Leaderboard.setItemStyle: player has no item')
    fails(function() board:addItem(board, 'x', 1) end, 'Leaderboard.addItem: expected Player wrapper')
    fails(function() board:setItemValue(board, 1) end, 'Leaderboard.setItemValue: expected Player wrapper')
    fails(function() board:hasItem(board) end, 'Leaderboard.hasItem: expected Player wrapper')
    fails(function() board:setItemStyle(red, {icons = false}) end, "Leaderboard.setItemStyle: unknown option 'icons'")
    fails(function() board:setStyle({labels = true}) end, "Leaderboard.setStyle: unknown option 'labels'")
    fails(function() board:setStyle({names = 1}) end, "option 'names' expected a boolean")
    for _, name in ipairs(mutating) do eq(callCount(name), 0) end
    board:destroy()
end)

test('style, colors, sorting, assignment and display forward exact arguments', function()
    local board = Leaderboard.create()
    checkSetters(board, {{'LeaderboardSetLabel', 'setLabel', 'Gold'},
        {'LeaderboardSetLabelColor', 'setLabelColor', 1, 2, 3, 4}, {'LeaderboardSetValueColor', 'setValueColor', 5, 6, 7, 8},
        {'LeaderboardSortItemsByValue', 'sortByValue', true}, {'LeaderboardSortItemsByLabel', 'sortByLabel', false},
        {'LeaderboardSortItemsByPlayer', 'sortByPlayer', true}, {'LeaderboardDisplay', 'show', true}})
    board:setStyle({names = false}); expectCall('LeaderboardSetStyle', board.handle, true, false, true, true)
    board:setStyle(); expectCall('LeaderboardSetStyle', board.handle, true, true, true, true)
    local red = Player.fromIndex(0)
    board:assign(red); expectCall('PlayerSetLeaderboard', red.handle, board.handle)
    fails(function() board:assign(board) end, 'Leaderboard.assign: expected Player wrapper')
    eq(board.isDisplayed, nil)
    board:destroy()
end)

test('destroy is idempotent and guards every method', function()
    local board = Leaderboard.create()
    local raw = board.handle
    board:destroy(); board:destroy()
    expectCall('DestroyLeaderboard', raw); eq(callCount('DestroyLeaderboard'), 1)
    eq(board.handle, nil); eq(board:isDisposed(), true)
    checkDisposed(board, {'getHandle', 'setLabel', 'setLabelColor', 'setValueColor', 'setStyle', 'addItem',
        'removeItem', 'setItemValue', 'setItemLabel', 'setItemLabelColor', 'setItemValueColor', 'setItemStyle',
        'hasItem', 'getItemCount', 'sortByValue', 'sortByLabel', 'sortByPlayer', 'assign', 'show'})
end)

test('create fails clearly when the native returns nil', function()
    native('CreateLeaderboard', function() return nil end)
    fails(function() Leaderboard.create('x') end, 'Leaderboard.create: native returned nil')
    eq(callCount('LeaderboardSetLabel'), 0)
    native('CreateLeaderboard', function() return {items = {}} end)
end)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `deno task test leaderboard`
Expected: FAIL, `module 'wrappers.leaderboard' not found`.

- [ ] **Step 3: Write the implementation**

Create `src/wrappers/leaderboard.lua`:

```lua
local Handle = require('wrappers.internal.handle')
local Options = require('wrappers.internal.options')

---Items are keyed by player: each player has at most one item.
---@class MoonwellWrappers.Leaderboard
---@field handle leaderboard? Read-only by convention; nil after destruction.
local Leaderboard = {}
---@type MoonwellWrappers.Registry<MoonwellWrappers.Leaderboard, leaderboard>
local registry = Handle.new(Leaderboard, 'Leaderboard')

---@class MoonwellWrappers.LeaderboardStyleOptions
---@field label boolean? Show the leaderboard's label; default true.
---@field names boolean? Show the item labels; default true.
---@field values boolean? Show the values; default true.
---@field icons boolean? Show the icons; default true.

---@class MoonwellWrappers.LeaderboardItemStyleOptions
---@field label boolean? Show this item's label; default true.
---@field value boolean? Show this item's value; default true.
---@field icon boolean? Show this item's icon; default true.

---@type MoonwellWrappers.OptionFields
local styleFields = {label = {'boolean', true}, names = {'boolean', true}, values = {'boolean', true},
    icons = {'boolean', true}}
---@type MoonwellWrappers.OptionFields
local itemStyleFields = {label = {'boolean', true}, value = {'boolean', true}, icon = {'boolean', true}}

---A leaderboard starts with no rows and does not grow by itself (LeaderboardResizeBJ does the same).
---@param raw leaderboard
local function resize(raw) LeaderboardSetSizeByItemCount(raw, LeaderboardGetItemCount(raw)) end

---Returns the leaderboard handle, the player's zero-based item index and the player handle. Level 3 blames the caller
---of the public method.
---@param board MoonwellWrappers.Leaderboard
---@param player MoonwellWrappers.Player
---@param operation string
---@return leaderboard, integer, player
local function itemOf(board, player, operation)
    local raw = registry.require(board, operation)
    local p = Handle.unwrap(player, 'Player', operation)
    if not LeaderboardHasPlayerItem(raw, p) then error('[wrappers] ' .. operation .. ': player has no item', 3) end
    return raw, LeaderboardGetPlayerIndex(raw, p), p
end

---@param raw leaderboard?
---@return MoonwellWrappers.Leaderboard?
---@overload fun(raw: nil): nil
function Leaderboard.fromHandle(raw) return registry.wrap(raw) end
---@param label string?
---@return MoonwellWrappers.Leaderboard
function Leaderboard.create(label)
    local raw = CreateLeaderboard()
    local board = Handle.created(Leaderboard.fromHandle(raw), 'Leaderboard.create')
    if label ~= nil then LeaderboardSetLabel(raw, label) end
    return board
end
---@return leaderboard
function Leaderboard:getHandle() return registry.require(self, 'Leaderboard.getHandle') end
---@return boolean
function Leaderboard:isDisposed() return registry.isDisposed(self, 'Leaderboard.isDisposed') end
---@param text string
function Leaderboard:setLabel(text) LeaderboardSetLabel(registry.require(self, 'Leaderboard.setLabel'), text) end
---@param r integer 0-255
---@param g integer 0-255
---@param b integer 0-255
---@param a integer 0-255
function Leaderboard:setLabelColor(r, g, b, a)
    LeaderboardSetLabelColor(registry.require(self, 'Leaderboard.setLabelColor'), r, g, b, a)
end
---@param r integer 0-255
---@param g integer 0-255
---@param b integer 0-255
---@param a integer 0-255
function Leaderboard:setValueColor(r, g, b, a)
    LeaderboardSetValueColor(registry.require(self, 'Leaderboard.setValueColor'), r, g, b, a)
end
---@param options MoonwellWrappers.LeaderboardStyleOptions?
function Leaderboard:setStyle(options)
    local raw = registry.require(self, 'Leaderboard.setStyle')
    local o = Options.read(options, styleFields, 'Leaderboard.setStyle')
    LeaderboardSetStyle(raw, o.label, o.names, o.values, o.icons)
end
---Adds the player's item and resizes the board. A player has at most one item.
---@param player MoonwellWrappers.Player
---@param label string
---@param value integer
function Leaderboard:addItem(player, label, value)
    local raw = registry.require(self, 'Leaderboard.addItem')
    local p = Handle.unwrap(player, 'Player', 'Leaderboard.addItem')
    if LeaderboardHasPlayerItem(raw, p) then error('[wrappers] Leaderboard.addItem: player already has an item', 2) end
    LeaderboardAddItem(raw, label, value, p)
    resize(raw)
end
---Removes the player's item and resizes the board.
---@param player MoonwellWrappers.Player
function Leaderboard:removeItem(player)
    local raw, _, p = itemOf(self, player, 'Leaderboard.removeItem')
    LeaderboardRemovePlayerItem(raw, p)
    resize(raw)
end
---@param player MoonwellWrappers.Player
---@param value integer
function Leaderboard:setItemValue(player, value)
    local raw, index = itemOf(self, player, 'Leaderboard.setItemValue')
    LeaderboardSetItemValue(raw, index, value)
end
---@param player MoonwellWrappers.Player
---@param label string
function Leaderboard:setItemLabel(player, label)
    local raw, index = itemOf(self, player, 'Leaderboard.setItemLabel')
    LeaderboardSetItemLabel(raw, index, label)
end
---@param player MoonwellWrappers.Player
---@param r integer 0-255
---@param g integer 0-255
---@param b integer 0-255
---@param a integer 0-255
function Leaderboard:setItemLabelColor(player, r, g, b, a)
    local raw, index = itemOf(self, player, 'Leaderboard.setItemLabelColor')
    LeaderboardSetItemLabelColor(raw, index, r, g, b, a)
end
---@param player MoonwellWrappers.Player
---@param r integer 0-255
---@param g integer 0-255
---@param b integer 0-255
---@param a integer 0-255
function Leaderboard:setItemValueColor(player, r, g, b, a)
    local raw, index = itemOf(self, player, 'Leaderboard.setItemValueColor')
    LeaderboardSetItemValueColor(raw, index, r, g, b, a)
end
---@param player MoonwellWrappers.Player
---@param options MoonwellWrappers.LeaderboardItemStyleOptions?
function Leaderboard:setItemStyle(player, options)
    local raw, index = itemOf(self, player, 'Leaderboard.setItemStyle')
    local o = Options.read(options, itemStyleFields, 'Leaderboard.setItemStyle')
    LeaderboardSetItemStyle(raw, index, o.label, o.value, o.icon)
end
---@param player MoonwellWrappers.Player
---@return boolean
function Leaderboard:hasItem(player)
    local raw = registry.require(self, 'Leaderboard.hasItem')
    return LeaderboardHasPlayerItem(raw, Handle.unwrap(player, 'Player', 'Leaderboard.hasItem'))
end
---@return integer
function Leaderboard:getItemCount() return LeaderboardGetItemCount(registry.require(self, 'Leaderboard.getItemCount')) end
---@param ascending boolean
function Leaderboard:sortByValue(ascending)
    LeaderboardSortItemsByValue(registry.require(self, 'Leaderboard.sortByValue'), ascending)
end
---@param ascending boolean
function Leaderboard:sortByLabel(ascending)
    LeaderboardSortItemsByLabel(registry.require(self, 'Leaderboard.sortByLabel'), ascending)
end
---@param ascending boolean
function Leaderboard:sortByPlayer(ascending)
    LeaderboardSortItemsByPlayer(registry.require(self, 'Leaderboard.sortByPlayer'), ascending)
end
---Makes this the leaderboard that player sees. Assign, then show.
---@param player MoonwellWrappers.Player
function Leaderboard:assign(player)
    local raw = registry.require(self, 'Leaderboard.assign')
    PlayerSetLeaderboard(Handle.unwrap(player, 'Player', 'Leaderboard.assign'), raw)
end
---@param flag boolean
function Leaderboard:show(flag) LeaderboardDisplay(registry.require(self, 'Leaderboard.show'), flag) end
function Leaderboard:destroy()
    local raw = registry.dispose(self, 'Leaderboard.destroy')
    if raw then DestroyLeaderboard(raw) end
end

return Leaderboard
```

- [ ] **Step 4: Run test to verify it passes**

Run: `deno task test leaderboard`
Expected: `leaderboard: SUITE PASSED: 6 tests`.

- [ ] **Step 5: Run all checks and commit**

Run every command in "Running the wrappers checks", including the direct LuaLS run. Expected: all pass.

```bash
git add src/wrappers/leaderboard.lua tests/leaderboard.lua
git commit -m "feat: Leaderboard with player-keyed items that resize the board"
```

---

### Task 4: Quest, QuestItem and DefeatCondition

**Files:**
- Create: `src/wrappers/quest.lua`, `src/wrappers/defeatcondition.lua`
- Test: `tests/quest.lua`, `tests/defeatcondition.lua`

**Interfaces:**
- Consumes: `Handle`, `Options` (internal).
- Produces: `Quest.fromHandle(raw)`, `Quest.create(options?)`, `Quest.flashButton()`, `Quest.refresh()`, methods
  `getHandle`, `isDisposed`, `setTitle`, `setDescription`, `setIcon`, `setRequired`/`isRequired`,
  `setCompleted`/`isCompleted`, `setFailed`/`isFailed`, `setDiscovered`/`isDiscovered`, `setEnabled`/`isEnabled`,
  `addItem(description) -> QuestItem`, `destroy()`; QuestItem `getHandle`, `isDisposed`, `setDescription`,
  `setCompleted`, `isCompleted`, `getQuest`. `DefeatCondition.fromHandle(raw)`, `DefeatCondition.create(description?)`,
  `getHandle`, `isDisposed`, `setDescription`, `destroy`. Types `MoonwellWrappers.Quest`, `.QuestItem`,
  `.QuestOptions`, `.DefeatCondition`.

- [ ] **Step 1: Write the failing tests**

Create `tests/quest.lua`:

```lua
native('CreateQuest', function() return {} end)
native('QuestCreateItem', function() return {} end)
for _, name in ipairs({'QuestSetTitle', 'QuestSetDescription', 'QuestSetIconPath', 'QuestSetRequired',
    'QuestSetCompleted', 'QuestSetFailed', 'QuestSetDiscovered', 'QuestSetEnabled', 'QuestItemSetDescription',
    'QuestItemSetCompleted', 'DestroyQuest', 'FlashQuestDialogButton', 'ForceQuestDialogUpdate'}) do
    native(name, function() end)
end
local Quest = require('wrappers.quest')
eq(totalCalls(), 0)

test('create applies options and always sets required and discovered', function()
    local quest = Quest.create({title = 'Rescue', description = 'Find the prince', icon = 'q.blp', required = false})
    expectCall('QuestSetTitle', quest.handle, 'Rescue')
    expectCall('QuestSetDescription', quest.handle, 'Find the prince')
    expectCall('QuestSetIconPath', quest.handle, 'q.blp')
    expectCall('QuestSetRequired', quest.handle, false)
    expectCall('QuestSetDiscovered', quest.handle, true)
    eq(Quest.fromHandle(quest.handle), quest); eq(Quest.fromHandle(nil), nil)
    eq(quest:getHandle(), quest.handle); eq(quest:isDisposed(), false)
    local plain = Quest.create()
    eq(callCount('QuestSetTitle'), 1)
    expectCall('QuestSetRequired', plain.handle, true); expectCall('QuestSetDiscovered', plain.handle, true)
    quest:destroy(); plain:destroy()
end)

test('bad options fail before CreateQuest', function()
    fails(function() Quest.create({titel = 'x'}) end, "Quest.create: unknown option 'titel'")
    fails(function() Quest.create({required = 'yes'}) end, "Quest.create: option 'required' expected a boolean")
    fails(function() Quest.create('Rescue') end, 'Quest.create: expected an options table')
    eq(totalCalls(), 0)
end)

test('setters, getters and statics forward exact arguments', function()
    local quest = Quest.create()
    checkSetters(quest, {{'QuestSetTitle', 'setTitle', 'T'}, {'QuestSetDescription', 'setDescription', 'D'},
        {'QuestSetIconPath', 'setIcon', 'i.blp'}, {'QuestSetRequired', 'setRequired', false},
        {'QuestSetCompleted', 'setCompleted', true}, {'QuestSetFailed', 'setFailed', true},
        {'QuestSetDiscovered', 'setDiscovered', false}, {'QuestSetEnabled', 'setEnabled', false}})
    checkGetters(quest, {{'IsQuestRequired', 'isRequired', true}, {'IsQuestCompleted', 'isCompleted', false},
        {'IsQuestFailed', 'isFailed', true}, {'IsQuestDiscovered', 'isDiscovered', false},
        {'IsQuestEnabled', 'isEnabled', true}})
    Quest.flashButton(); expectCall('FlashQuestDialogButton')
    Quest.refresh(); expectCall('ForceQuestDialogUpdate')
    quest:destroy()
end)

test('quest items belong to their quest', function()
    local quest = Quest.create()
    local first = quest:addItem('Kill the ogre')
    expectCall('QuestCreateItem', quest.handle)
    expectCall('QuestItemSetDescription', first.handle, 'Kill the ogre')
    local second = quest:addItem('Return')
    eq(first:getQuest(), quest); eq(first:getHandle(), first.handle); eq(first:isDisposed(), false)
    checkSetters(first, {{'QuestItemSetDescription', 'setDescription', 'Changed'},
        {'QuestItemSetCompleted', 'setCompleted', true}})
    checkGetters(first, {{'IsQuestItemCompleted', 'isCompleted', true}})
    eq(first.destroy, nil); eq(first.fromHandle, nil)
    fails(function() first.getQuest(quest) end, 'QuestItem.getQuest: expected QuestItem wrapper')
    local raw = quest.handle
    quest:destroy(); quest:destroy()
    expectCall('DestroyQuest', raw); eq(callCount('DestroyQuest'), 1)
    eq(first:isDisposed(), true); eq(second:isDisposed(), true); eq(first.handle, nil)
    eq(first:getQuest(), quest)
    checkDisposed(first, {'getHandle', 'setDescription', 'setCompleted', 'isCompleted'})
    checkDisposed(quest, {'getHandle', 'setTitle', 'setDescription', 'setIcon', 'setRequired', 'setCompleted',
        'setFailed', 'setDiscovered', 'setEnabled', 'isRequired', 'isCompleted', 'isFailed', 'isDiscovered',
        'isEnabled', 'addItem'})
end)

test('create and addItem fail clearly when the native returns nil', function()
    native('CreateQuest', function() return nil end)
    fails(function() Quest.create() end, 'Quest.create: native returned nil')
    eq(callCount('QuestSetRequired'), 0)
    native('CreateQuest', function() return {} end)
    local quest = Quest.create()
    native('QuestCreateItem', function() return nil end)
    fails(function() quest:addItem('x') end, 'Quest.addItem: native returned nil')
    eq(callCount('QuestItemSetDescription'), 0)
    native('QuestCreateItem', function() return {} end)
    quest:destroy()
end)
```

Create `tests/defeatcondition.lua`:

```lua
native('CreateDefeatCondition', function() return {} end)
for _, name in ipairs({'DefeatConditionSetDescription', 'DestroyDefeatCondition'}) do native(name, function() end) end
local DefeatCondition = require('wrappers.defeatcondition')
eq(totalCalls(), 0)

test('defeat conditions set their description and keep identity', function()
    local condition = DefeatCondition.create('Lose the prince')
    expectCall('DefeatConditionSetDescription', condition.handle, 'Lose the prince')
    eq(DefeatCondition.fromHandle(condition.handle), condition); eq(DefeatCondition.fromHandle(nil), nil)
    eq(condition:getHandle(), condition.handle); eq(condition:isDisposed(), false)
    checkSetters(condition, {{'DefeatConditionSetDescription', 'setDescription', 'Changed'}})
    local plain = DefeatCondition.create()
    eq(callCount('DefeatConditionSetDescription'), 2)
    local raw = condition.handle
    condition:destroy(); condition:destroy()
    expectCall('DestroyDefeatCondition', raw); eq(callCount('DestroyDefeatCondition'), 1)
    eq(condition.handle, nil); eq(condition:isDisposed(), true)
    checkDisposed(condition, {'getHandle', 'setDescription'})
    plain:destroy()
end)

test('create fails clearly when the native returns nil', function()
    native('CreateDefeatCondition', function() return nil end)
    fails(function() DefeatCondition.create('x') end, 'DefeatCondition.create: native returned nil')
    eq(callCount('DefeatConditionSetDescription'), 0)
    native('CreateDefeatCondition', function() return {} end)
end)
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `deno task test quest defeatcondition`
Expected: both FAIL with `module 'wrappers.quest' not found` and `module 'wrappers.defeatcondition' not found`.

- [ ] **Step 3: Write the implementations**

Create `src/wrappers/quest.lua`:

```lua
local Handle = require('wrappers.internal.handle')
local Options = require('wrappers.internal.options')

---@class MoonwellWrappers.Quest
---@field handle quest? Read-only by convention; nil after destruction.
local Quest = {}
---@type MoonwellWrappers.Registry<MoonwellWrappers.Quest, quest>
local registry = Handle.new(Quest, 'Quest')

---An item belongs to the quest that made it: the quest's destroy() disposes it.
---@class MoonwellWrappers.QuestItem
---@field handle questitem? Read-only by convention; nil once its quest is destroyed.
local QuestItem = {}
---@type MoonwellWrappers.Registry<MoonwellWrappers.QuestItem, questitem>
local itemRegistry = Handle.new(QuestItem, 'QuestItem')

---@class MoonwellWrappers.QuestOptions
---@field title string?
---@field description string?
---@field icon string? Icon path.
---@field required boolean? Required (true) or optional (false); default true.
---@field discovered boolean? Shown in the quest log; default true.

---@type MoonwellWrappers.OptionFields
local questFields = {
    title = {'string'}, description = {'string'}, icon = {'string'}, required = {'boolean', true},
    discovered = {'boolean', true},
}

-- Keyed by wrapper; only indexed, never iterated. Each list is in creation order.
---@type table<MoonwellWrappers.Quest, MoonwellWrappers.QuestItem[]>
local itemsOf = {}
---@type table<MoonwellWrappers.QuestItem, MoonwellWrappers.Quest>
local questOf = setmetatable({}, {__mode = 'k'})

---@param raw quest?
---@return MoonwellWrappers.Quest?
---@overload fun(raw: nil): nil
function Quest.fromHandle(raw) return registry.wrap(raw) end
---@param options MoonwellWrappers.QuestOptions?
---@return MoonwellWrappers.Quest
function Quest.create(options)
    local o = Options.read(options, questFields, 'Quest.create')
    local raw = CreateQuest()
    local quest = Handle.created(Quest.fromHandle(raw), 'Quest.create')
    if o.title ~= nil then QuestSetTitle(raw, o.title) end
    if o.description ~= nil then QuestSetDescription(raw, o.description) end
    if o.icon ~= nil then QuestSetIconPath(raw, o.icon) end
    QuestSetRequired(raw, o.required)
    QuestSetDiscovered(raw, o.discovered)
    return quest
end
---Flashes the quest button, for everyone.
function Quest.flashButton() FlashQuestDialogButton() end
---Updates an open quest log.
function Quest.refresh() ForceQuestDialogUpdate() end
---@return quest
function Quest:getHandle() return registry.require(self, 'Quest.getHandle') end
---@return boolean
function Quest:isDisposed() return registry.isDisposed(self, 'Quest.isDisposed') end
---@param text string
function Quest:setTitle(text) QuestSetTitle(registry.require(self, 'Quest.setTitle'), text) end
---@param text string
function Quest:setDescription(text) QuestSetDescription(registry.require(self, 'Quest.setDescription'), text) end
---@param path string
function Quest:setIcon(path) QuestSetIconPath(registry.require(self, 'Quest.setIcon'), path) end
---@param flag boolean
function Quest:setRequired(flag) QuestSetRequired(registry.require(self, 'Quest.setRequired'), flag) end
---@return boolean
function Quest:isRequired() return IsQuestRequired(registry.require(self, 'Quest.isRequired')) end
---@param flag boolean
function Quest:setCompleted(flag) QuestSetCompleted(registry.require(self, 'Quest.setCompleted'), flag) end
---@return boolean
function Quest:isCompleted() return IsQuestCompleted(registry.require(self, 'Quest.isCompleted')) end
---@param flag boolean
function Quest:setFailed(flag) QuestSetFailed(registry.require(self, 'Quest.setFailed'), flag) end
---@return boolean
function Quest:isFailed() return IsQuestFailed(registry.require(self, 'Quest.isFailed')) end
---@param flag boolean
function Quest:setDiscovered(flag) QuestSetDiscovered(registry.require(self, 'Quest.setDiscovered'), flag) end
---@return boolean
function Quest:isDiscovered() return IsQuestDiscovered(registry.require(self, 'Quest.isDiscovered')) end
---@param flag boolean
function Quest:setEnabled(flag) QuestSetEnabled(registry.require(self, 'Quest.setEnabled'), flag) end
---@return boolean
function Quest:isEnabled() return IsQuestEnabled(registry.require(self, 'Quest.isEnabled')) end
---@param description string
---@return MoonwellWrappers.QuestItem
function Quest:addItem(description)
    local raw = registry.require(self, 'Quest.addItem')
    local itemRaw = QuestCreateItem(raw)
    local item = Handle.created(itemRegistry.wrap(itemRaw), 'Quest.addItem')
    QuestItemSetDescription(itemRaw, description)
    local list = itemsOf[self] or {}
    itemsOf[self] = list
    list[#list + 1] = item
    questOf[item] = self
    return item
end
---Disposes every item of the quest, then destroys it.
function Quest:destroy()
    local raw = registry.dispose(self, 'Quest.destroy')
    if not raw then return end
    for _, item in ipairs(itemsOf[self] or {}) do itemRegistry.dispose(item, 'Quest.destroy') end
    itemsOf[self] = nil
    DestroyQuest(raw)
end

---@return questitem
function QuestItem:getHandle() return itemRegistry.require(self, 'QuestItem.getHandle') end
---@return boolean
function QuestItem:isDisposed() return itemRegistry.isDisposed(self, 'QuestItem.isDisposed') end
---@param text string
function QuestItem:setDescription(text)
    QuestItemSetDescription(itemRegistry.require(self, 'QuestItem.setDescription'), text)
end
---@param flag boolean
function QuestItem:setCompleted(flag)
    QuestItemSetCompleted(itemRegistry.require(self, 'QuestItem.setCompleted'), flag)
end
---@return boolean
function QuestItem:isCompleted() return IsQuestItemCompleted(itemRegistry.require(self, 'QuestItem.isCompleted')) end
---The quest that made this item; still answers after the item is disposed.
---@return MoonwellWrappers.Quest
function QuestItem:getQuest()
    itemRegistry.isDisposed(self, 'QuestItem.getQuest')
    return questOf[self]
end

return Quest
```

Create `src/wrappers/defeatcondition.lua`:

```lua
local Handle = require('wrappers.internal.handle')

---@class MoonwellWrappers.DefeatCondition
---@field handle defeatcondition? Read-only by convention; nil after destruction.
local DefeatCondition = {}
---@type MoonwellWrappers.Registry<MoonwellWrappers.DefeatCondition, defeatcondition>
local registry = Handle.new(DefeatCondition, 'DefeatCondition')

---@param raw defeatcondition?
---@return MoonwellWrappers.DefeatCondition?
---@overload fun(raw: nil): nil
function DefeatCondition.fromHandle(raw) return registry.wrap(raw) end
---A defeat condition listed in the quest log.
---@param description string?
---@return MoonwellWrappers.DefeatCondition
function DefeatCondition.create(description)
    local raw = CreateDefeatCondition()
    local condition = Handle.created(DefeatCondition.fromHandle(raw), 'DefeatCondition.create')
    if description ~= nil then DefeatConditionSetDescription(raw, description) end
    return condition
end
---@return defeatcondition
function DefeatCondition:getHandle() return registry.require(self, 'DefeatCondition.getHandle') end
---@return boolean
function DefeatCondition:isDisposed() return registry.isDisposed(self, 'DefeatCondition.isDisposed') end
---@param text string
function DefeatCondition:setDescription(text)
    DefeatConditionSetDescription(registry.require(self, 'DefeatCondition.setDescription'), text)
end
function DefeatCondition:destroy()
    local raw = registry.dispose(self, 'DefeatCondition.destroy')
    if raw then DestroyDefeatCondition(raw) end
end

return DefeatCondition
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `deno task test quest defeatcondition`
Expected: `quest: SUITE PASSED: 5 tests` and `defeatcondition: SUITE PASSED: 2 tests`.

- [ ] **Step 5: Run all checks and commit**

Run every command in "Running the wrappers checks", including the direct LuaLS run. Expected: all pass.

```bash
git add src/wrappers/quest.lua src/wrappers/defeatcondition.lua tests/quest.lua tests/defeatcondition.lua
git commit -m "feat: Quest with items owned by their quest, and DefeatCondition"
```

---

### Task 5: TimerDialog

**Files:**
- Create: `src/wrappers/timerdialog.lua`
- Test: `tests/timerdialog.lua`

**Interfaces:**
- Consumes: `Handle` (internal); Timer wrappers as arguments (via `Handle.unwrap(timer, 'Timer', ...)`).
- Produces: `TimerDialog.fromHandle(raw)`, `TimerDialog.create(timer, title?)`, methods `getHandle`, `isDisposed`,
  `setTitle`, `setTitleColor(r, g, b, a)`, `setTimeColor(r, g, b, a)`, `setSpeed(factor)`,
  `setRealTimeRemaining(seconds)`, `show(flag)`, `setVisibleFor(player)`, `destroy()`. Type
  `MoonwellWrappers.TimerDialog`.

- [ ] **Step 1: Write the failing test**

Create `tests/timerdialog.lua`:

```lua
native('CreateTimer', function() return {} end)
native('CreateTimerDialog', function() return {} end)
native('GetLocalPlayer', function() return PLAYER_RAW end)
for _, name in ipairs({'PauseTimer', 'DestroyTimer', 'TimerDialogSetTitle', 'TimerDialogSetTitleColor',
    'TimerDialogSetTimeColor', 'TimerDialogSetSpeed', 'TimerDialogSetRealTimeRemaining', 'TimerDialogDisplay',
    'DestroyTimerDialog'}) do
    native(name, function() end)
end
local TimerDialog = require('wrappers.timerdialog')
local Timer = require('wrappers.timer')
local Player = require('wrappers.player')
eq(totalCalls(), 0)

local methods = {'getHandle', 'setTitle', 'setTitleColor', 'setTimeColor', 'setSpeed', 'setRealTimeRemaining', 'show',
    'setVisibleFor'}

test('create shows the given timer with an optional title, hidden until show', function()
    local timer = Timer.create()
    local dialog = TimerDialog.create(timer, 'Next wave')
    expectCall('CreateTimerDialog', timer.handle)
    expectCall('TimerDialogSetTitle', dialog.handle, 'Next wave')
    eq(callCount('TimerDialogDisplay'), 0)
    eq(TimerDialog.fromHandle(dialog.handle), dialog); eq(TimerDialog.fromHandle(nil), nil)
    eq(dialog:getHandle(), dialog.handle); eq(dialog:isDisposed(), false)
    local untitled = TimerDialog.create(timer)
    eq(callCount('TimerDialogSetTitle'), 1)
    resetCalls()
    fails(function() TimerDialog.create(dialog) end, 'TimerDialog.create: expected Timer wrapper')
    eq(totalCalls(), 0)
    dialog:destroy(); untitled:destroy(); timer:destroy()
end)

test('setters forward exact arguments', function()
    local timer = Timer.create()
    local dialog = TimerDialog.create(timer)
    checkSetters(dialog, {{'TimerDialogSetTitle', 'setTitle', 'T'},
        {'TimerDialogSetTitleColor', 'setTitleColor', 1, 2, 3, 4}, {'TimerDialogSetTimeColor', 'setTimeColor', 5, 6, 7, 8},
        {'TimerDialogSetSpeed', 'setSpeed', 2}, {'TimerDialogSetRealTimeRemaining', 'setRealTimeRemaining', 30},
        {'TimerDialogDisplay', 'show', true}})
    dialog:setVisibleFor(Player.fromIndex(0)); expectCall('TimerDialogDisplay', dialog.handle, true)
    dialog:setVisibleFor(Player.fromHandle({})); expectCall('TimerDialogDisplay', dialog.handle, false)
    fails(function() dialog:setVisibleFor(timer) end, 'TimerDialog.setVisibleFor: expected Player wrapper')
    eq(dialog.isDisplayed, nil)
    dialog:destroy(); timer:destroy()
end)

test('methods raise once the timer is disposed, but destroy still works', function()
    local timer = Timer.create()
    local dialog = TimerDialog.create(timer)
    timer:destroy()
    resetCalls()
    for _, method in ipairs(methods) do
        fails(function() dialog[method](dialog) end, 'TimerDialog.' .. method .. ': Timer is disposed')
    end
    eq(totalCalls(), 0); eq(dialog:isDisposed(), false)
    local raw = dialog.handle
    dialog:destroy(); dialog:destroy()
    expectCall('DestroyTimerDialog', raw); eq(callCount('DestroyTimerDialog'), 1)
    eq(callCount('DestroyTimer'), 0)
end)

test('destroy never destroys the timer and guards every method', function()
    local timer = Timer.create()
    local dialog = TimerDialog.create(timer)
    dialog:destroy()
    eq(callCount('DestroyTimer'), 0); eq(timer:isDisposed(), false)
    eq(dialog.handle, nil); eq(dialog:isDisposed(), true)
    checkDisposed(dialog, methods)
    timer:destroy()
end)

test('create fails clearly when the native returns nil', function()
    local timer = Timer.create()
    native('CreateTimerDialog', function() return nil end)
    fails(function() TimerDialog.create(timer, 'x') end, 'TimerDialog.create: native returned nil')
    eq(callCount('TimerDialogSetTitle'), 0)
    native('CreateTimerDialog', function() return {} end)
    timer:destroy()
end)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `deno task test timerdialog`
Expected: FAIL, `module 'wrappers.timerdialog' not found`.

- [ ] **Step 3: Write the implementation**

Create `src/wrappers/timerdialog.lua`:

```lua
local Handle = require('wrappers.internal.handle')

---Shows a Timer's countdown. It does not own the timer: destroy the timer dialog first.
---@class MoonwellWrappers.TimerDialog
---@field handle timerdialog? Read-only by convention; nil after destruction.
local TimerDialog = {}
---@type MoonwellWrappers.Registry<MoonwellWrappers.TimerDialog, timerdialog>
local registry = Handle.new(TimerDialog, 'TimerDialog')
---The Timer each dialog made by create shows. Only indexed, never iterated.
---@type table<MoonwellWrappers.TimerDialog, MoonwellWrappers.Timer>
local timers = setmetatable({}, {__mode = 'k'})

---Returns the dialog's handle after checking the dialog and, when known, its Timer.
---@param dialog MoonwellWrappers.TimerDialog
---@param operation string
---@return timerdialog
local function live(dialog, operation)
    local raw = registry.require(dialog, operation)
    local timer = timers[dialog]
    if timer ~= nil then Handle.unwrap(timer, 'Timer', operation) end
    return raw
end

---@param raw timerdialog?
---@return MoonwellWrappers.TimerDialog?
---@overload fun(raw: nil): nil
function TimerDialog.fromHandle(raw) return registry.wrap(raw) end
---Creates a hidden timer dialog for the timer; call show(true) to display it.
---@param timer MoonwellWrappers.Timer
---@param title string?
---@return MoonwellWrappers.TimerDialog
function TimerDialog.create(timer, title)
    local raw = CreateTimerDialog(Handle.unwrap(timer, 'Timer', 'TimerDialog.create'))
    local dialog = Handle.created(TimerDialog.fromHandle(raw), 'TimerDialog.create')
    timers[dialog] = timer
    if title ~= nil then TimerDialogSetTitle(raw, title) end
    return dialog
end
---@return timerdialog
function TimerDialog:getHandle() return live(self, 'TimerDialog.getHandle') end
---@return boolean
function TimerDialog:isDisposed() return registry.isDisposed(self, 'TimerDialog.isDisposed') end
---@param text string
function TimerDialog:setTitle(text) TimerDialogSetTitle(live(self, 'TimerDialog.setTitle'), text) end
---@param r integer 0-255
---@param g integer 0-255
---@param b integer 0-255
---@param a integer 0-255
function TimerDialog:setTitleColor(r, g, b, a)
    TimerDialogSetTitleColor(live(self, 'TimerDialog.setTitleColor'), r, g, b, a)
end
---@param r integer 0-255
---@param g integer 0-255
---@param b integer 0-255
---@param a integer 0-255
function TimerDialog:setTimeColor(r, g, b, a)
    TimerDialogSetTimeColor(live(self, 'TimerDialog.setTimeColor'), r, g, b, a)
end
---@param factor number
function TimerDialog:setSpeed(factor) TimerDialogSetSpeed(live(self, 'TimerDialog.setSpeed'), factor) end
---@param seconds number
function TimerDialog:setRealTimeRemaining(seconds)
    TimerDialogSetRealTimeRemaining(live(self, 'TimerDialog.setRealTimeRemaining'), seconds)
end
---@param flag boolean
function TimerDialog:show(flag) TimerDialogDisplay(live(self, 'TimerDialog.show'), flag) end
---Shows the timer dialog on that player's machine only. Only local visuals differ.
---@param player MoonwellWrappers.Player
function TimerDialog:setVisibleFor(player)
    local raw = live(self, 'TimerDialog.setVisibleFor')
    TimerDialogDisplay(raw, Handle.unwrap(player, 'Player', 'TimerDialog.setVisibleFor') == GetLocalPlayer())
end
---Destroys the timer dialog, never its timer. Works after the timer is destroyed.
function TimerDialog:destroy()
    local raw = registry.dispose(self, 'TimerDialog.destroy')
    if not raw then return end
    timers[self] = nil
    DestroyTimerDialog(raw)
end

return TimerDialog
```

- [ ] **Step 4: Run test to verify it passes**

Run: `deno task test timerdialog`
Expected: `timerdialog: SUITE PASSED: 5 tests`.

- [ ] **Step 5: Run all checks and commit**

Run every command in "Running the wrappers checks", including the direct LuaLS run. Expected: all pass.

```bash
git add src/wrappers/timerdialog.lua tests/timerdialog.lua
git commit -m "feat: TimerDialog that raises once its timer is disposed"
```

---

### Task 6: Import graph, bundle and editor coverage

**Files:**
- Modify: `tests/imports.lua`, `tools/integration.ts`, `tests/editor-positive.lua`, `tests/editor-negative.lua`,
  `tests/editor-positive.yue`

**Interfaces:**
- Consumes: every module from Tasks 1–5.
- Produces: automated proof that the new modules load no other public module (dialog: only Player), that Multiboard-only
  and Dialog-only maps bundle only what they should, and LuaLS coverage of the new API.

- [ ] **Step 1: Extend the import test**

Replace the whole of `tests/imports.lua` with:

```lua
for _, name in ipairs({'trigger', 'effect', 'timer', 'destructable', 'rect', 'region', 'texttag', 'sound', 'lightning',
    'image', 'ubersplat', 'fogmodifier', 'multiboard', 'leaderboard', 'quest', 'defeatcondition', 'timerdialog'}) do
    require('wrappers.' .. name)
end
eq(totalCalls(), 0)

test('modules that only take wrapper arguments load no other public module', function()
    for _, name in ipairs({'unit', 'player', 'group', 'item', 'force'}) do eq(package.loaded['wrappers.' .. name], nil) end
end)

test('the dialog module loads the Player module and no other', function()
    require('wrappers.dialog')
    eq(totalCalls(), 0)
    eq(package.loaded['wrappers.player'] ~= nil, true)
    for _, name in ipairs({'unit', 'group', 'item', 'force'}) do eq(package.loaded['wrappers.' .. name], nil) end
end)
```

Run: `deno task test imports`
Expected: `imports: SUITE PASSED: 2 tests`.

- [ ] **Step 2: Extend the bundle checks in `tools/integration.ts`**

In the Unit-only exclusion list (the array after `for (\n  const unused of [`), add after `"fogmodifier",`:

```ts
    "dialog",
    "multiboard",
    "leaderboard",
    "quest",
    "defeatcondition",
    "timerdialog",
```

Replace the whole solo-bundle block, from the comment
`// Trigger and TextTag take wrapper arguments only, so a map importing just one of them bundles no other public module.`
through `console.log("Moonwell: Trigger-only and TextTag-only maps bundle no other public module");`, with:

```ts
// A map importing just one module bundles only that module and the public modules it returns wrappers of.
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
  "dialog",
  "multiboard",
  "leaderboard",
  "quest",
  "defeatcondition",
  "timerdialog",
];
const soloEntries: Record<string, { source: string; allowed: string[] }> = {
  trigger: { source: 'import "wrappers.trigger" as Trigger\nt = Trigger.create!\nt\\destroy!\n', allowed: [] },
  texttag: {
    source: 'import "wrappers.texttag" as TextTag\nt = TextTag.create!\nt\\destroy!\nTextTag.float "+1", 0, 0\n',
    allowed: [],
  },
  multiboard: {
    source: 'import "wrappers.multiboard" as Multiboard\nb = Multiboard.create 1, 1\nb\\destroy!\n',
    allowed: [],
  },
  dialog: { source: 'import "wrappers.dialog" as Dialog\nd = Dialog.create!\nd\\destroy!\n', allowed: ["player"] },
};
// "wrappers.timer" is a prefix of "wrappers.timerdialog"; match whole module names by the closing quote.
const bundles = (bundle: string, name: string) => bundle.includes(`wrappers.${name}"`) || bundle.includes(`wrappers.${name}'`);
for (const [entry, { source, allowed }] of Object.entries(soloEntries)) {
  await Deno.writeTextFile(join(consumer, "src/main.yue"), source);
  await moonwell(["build"]);
  const soloBundle = await Deno.readTextFile(join(consumer, "dist/stage/map.w3x/war3map.lua"));
  // Guard the absence checks below: they would pass vacuously if the bundle held no wrapper module at all.
  if (!bundles(soloBundle, entry)) throw new Error(`${entry}-only bundle lacks wrappers.${entry}`);
  for (const name of allowed) {
    if (!bundles(soloBundle, name)) throw new Error(`${entry}-only bundle lacks wrappers.${name}`);
  }
  for (const unused of publicModules) {
    if (unused !== entry && !allowed.includes(unused) && bundles(soloBundle, unused)) {
      throw new Error(`${entry}-only bundle includes wrappers.${unused}`);
    }
  }
}
console.log("Moonwell: Trigger-, TextTag-, Multiboard- and Dialog-only maps bundle only what they import");
```

Before relying on `bundles`, confirm how module names appear in a bundle: open any
`.test-work/integration-*/consumer/dist/stage/map.w3x/war3map.lua` and search for `wrappers.texttag`. If the name is
followed by something other than a quote (for example the bundle uses `["wrappers.texttag"] =` it is a `"`, which
works), adapt `bundles` so it matches the whole module name and not a prefix, and say so in the task report.

Run `deno fmt` (it decides the final layout).

- [ ] **Step 3: Extend the LuaLS positive fixtures**

In `tests/editor-positive.lua`, after `local FogModifier = require('wrappers.fogmodifier')`, add:

```lua
local Dialog = require('wrappers.dialog')
local Multiboard = require('wrappers.multiboard')
local Leaderboard = require('wrappers.leaderboard')
local Quest = require('wrappers.quest')
local DefeatCondition = require('wrappers.defeatcondition')
local TimerDialog = require('wrappers.timerdialog')
```

After the line `for _, found in ipairs(Destructable.enumInRect(area)) do found:kill() end`, add:

```lua
local dialog = Dialog.create()
dialog:setMessage('Choose')
local stay = dialog:addButton('Stay', function(player) player:addGold(10) end)
dialog:addButton('Leave', {hotkey = 'L', quit = true}, function(player) print(player:getName()) end)
if stay:getDialog() == dialog then dialog:show(PlayerWrapper.fromIndex(0)) end
local board = Multiboard.create(2, 2, 'Scores')
board:setCell(1, 1, {value = 'Name', color = {255, 204, 0}, showValue = true, showIcon = false})
board:setRow(2, {width = 0.05})
board:setAll({icon = 'x.blp'})
board:setVisibleFor(PlayerWrapper.fromIndex(0))
local leaders = Leaderboard.create('Kills')
leaders:addItem(PlayerWrapper.fromIndex(0), 'Red', 3)
leaders:setStyle({icons = false})
leaders:setItemStyle(PlayerWrapper.fromIndex(0), {value = true})
leaders:assign(PlayerWrapper.fromIndex(0))
local quest = Quest.create({title = 'Rescue', required = false})
local step = quest:addItem('Find the prince')
step:setCompleted(step:getQuest():isDiscovered())
local defeat = DefeatCondition.create('Lose the prince')
local countdown = TimerDialog.create(timer, 'Next wave')
countdown:setVisibleFor(PlayerWrapper.fromIndex(0))
countdown:destroy()
defeat:destroy()
quest:destroy()
leaders:destroy()
board:destroy()
dialog:destroy()
```

In `tests/editor-positive.yue`, add `import "wrappers.dialog" as Dialog` after the last `import` line, and add as the
last lines inside the `mw.on_main ->` block (same two-space indentation as its other lines):

```text
  dialog = Dialog.create!
  dialog\addButton "Stay", (player) -> player\addGold 10
```

- [ ] **Step 4: Extend the LuaLS negative fixture**

In `tests/editor-negative.lua`, after `local TextTag = require('wrappers.texttag')`, add:

```lua
local Dialog = require('wrappers.dialog')
local Multiboard = require('wrappers.multiboard')
```

Before the final `return true`, add:

```lua
Dialog.create():addButton('x', nil, function(player) Group.create():add(player) end) -- EXPECT param-type-mismatch
Multiboard.create(1, 1):setVisibleFor(unit) -- EXPECT param-type-mismatch
```

- [ ] **Step 5: Run the integration check and settle the negative lines**

Run: `deno task test:integration`
Expected: PASS, with `LuaLS: 13 intentional type errors detected at the expected lines` and
`Moonwell: Trigger-, TextTag-, Multiboard- and Dialog-only maps bundle only what they import`.

If it fails only because LuaLS 3.19.1 reports a different code on the `Dialog.create():addButton(...)` line, change
that line's `EXPECT` to the reported code. If LuaLS reports nothing there (it does not infer the callback's parameter
type), delete the line and record in the task report that LuaLS 3.19.1 does not type that callback parameter. Any other
mismatch is a real failure: fix the code or fixture.

- [ ] **Step 6: Run all checks and commit**

Run every command in "Running the wrappers checks". Expected: all pass.

```bash
git add tests/imports.lua tools/integration.ts tests/editor-positive.lua tests/editor-negative.lua tests/editor-positive.yue
git commit -m "test: import graph, solo bundles and editor fixtures for classic UI wrappers"
```

---

### Task 7: Gate example and documentation

**Files:**
- Modify: `examples/gate.yue`, `README.md`, `CHANGELOG.md`, `CONTRIBUTING.md`, `AGENTS.md` (wrappers)
- Modify: Moonwell `AGENTS.md` (in `C:/Users/mdlsvensson/Repo/moonwell`)

**Interfaces:**
- Consumes: the full v0.4.0 API.
- Produces: the in-game gate the maintainer runs, and the docs.

- [ ] **Step 1: Add the classic UI gate to `examples/gate.yue`**

Edit with the file-editing tool, not the shell (backslashes). After the header comment line
`-- Set presentation = true for a separate run of the v0.3.0 presentation gate only.`, add:

```text
-- Set ui = true for a separate run of the v0.4.0 classic UI gate only.
```

After `import "wrappers.fogmodifier" as FogModifier`, add:

```text
import "wrappers.dialog" as Dialog
import "wrappers.multiboard" as Multiboard
import "wrappers.leaderboard" as Leaderboard
import "wrappers.quest" as Quest
import "wrappers.defeatcondition" as DefeatCondition
import "wrappers.timerdialog" as TimerDialog
```

After `presentation = false`, add `ui = false`.

Before the `-- Start just after the map loads` comment, add:

```text
-- v0.4.0 classic UI, after the dialog: a board, a leaderboard, quests and a timer dialog, one step every 3 seconds.
uiBoards = (owner) ->
  other = Player.fromIndex 1
  footman = "ReplaceableTextures\\CommandButtons\\BTNFootman.blp"
  board = Multiboard.create 3, 3, "Wrapper multiboard"
  board\setTitleColor 255, 220, 0, 255
  board\setAll {showValue: true, showIcon: false, width: 0.05}
  board\setColumn 1, {width: 0.08}
  board\setCell 1, 1, {value: "Name", color: {255, 204, 0}}
  board\setCell 1, 2, {value: "Kills"}
  board\setCell 2, 1, {value: "Footman", icon: footman, showValue: true, showIcon: true}
  board\setRow 3, {value: "Row 3"}
  board\setColumn 3, {icon: footman, showValue: false, showIcon: true}
  board\show true
  print "Wrapper multiboard", board\getTitle!, board\getRowCount!, board\getColumnCount!
  local leaders, quest, optional, first, defeat, countdown, clock
  steps = Timer.create!
  step = 0
  steps\start 3, true, (self) ->
    step += 1
    if step == 1
      board\setRowCount 6
      board\setRow 6, {value: "Row 6"}
      print "Wrapper multiboard rows", board\getRowCount!
    elseif step == 2
      board\setRowCount 2
      print "Wrapper multiboard rows", board\getRowCount!
    elseif step == 3
      board\minimize true
      print "Wrapper multiboard minimized"
    elseif step == 4
      board\minimize false
      board\setVisibleFor other
      print "Wrapper multiboard hidden for another player"
    elseif step == 5
      board\destroy!
      leaders = Leaderboard.create "Wrapper leaderboard"
      leaders\setLabelColor 255, 220, 0, 255
      leaders\addItem owner, "You", 5
      leaders\addItem other, "Other", 9
      leaders\setItemValueColor other, 0, 128, 255, 255
      leaders\sortByValue false
      leaders\assign owner
      leaders\show true
      print "Wrapper leaderboard items", leaders\getItemCount!
    elseif step == 6
      leaders\setItemValue owner, 12
      leaders\setItemLabel owner, "You (12)"
      leaders\sortByValue false
      leaders\setStyle {icons: false}
      print "Wrapper leaderboard re-sorted"
    elseif step == 7
      leaders\removeItem other
      print "Wrapper leaderboard items", leaders\getItemCount!, "has other", leaders\hasItem(other)
    elseif step == 8
      leaders\destroy!
      quest = Quest.create {title: "Wrapper quest", description: "The first item completes 3 s later.", icon: footman}
      first = quest\addItem "Wrapper first item"
      quest\addItem "Wrapper second item"
      optional = Quest.create {title: "Wrapper optional quest", description: "Fails 6 s later.", required: false}
      defeat = DefeatCondition.create "Wrapper defeat condition"
      Quest.flashButton!
      print "Wrapper quests created: open the quest log (F9)"
    elseif step == 9
      first\setCompleted true
      Quest.refresh!
      print "Wrapper quest item completed", first\isCompleted!
    elseif step == 10
      quest\setCompleted true
      optional\setFailed true
      Quest.refresh!
      print "Wrapper quest completed", quest\isCompleted!, "optional failed", optional\isFailed!
    elseif step == 11
      clock = Timer.create!
      clock\start 60, false, -> nil
      countdown = TimerDialog.create clock, "Wrapper timer dialog"
      countdown\setTitleColor 255, 220, 0, 255
      countdown\setTimeColor 0, 255, 0, 255
      countdown\show true
      print "Wrapper timer dialog shown"
    elseif step == 12
      countdown\setSpeed 4
      print "Wrapper timer dialog speed 4"
    elseif step == 13
      countdown\setRealTimeRemaining 10
      print "Wrapper timer dialog remaining 10"
    elseif step == 14
      countdown\setVisibleFor other
      print "Wrapper timer dialog hidden for another player"
    elseif step == 15
      countdown\destroy!
      clock\destroy!
      quest\destroy!
      optional\destroy!
      defeat\destroy!
      self\destroy!
      print "Wrapper ui cleanup passed; quest item disposed", first\isDisposed!

-- v0.4.0 classic UI. Run with ui = true: the dialog first; its last button starts uiBoards.
uiGate = (owner) ->
  dialog = Dialog.create!
  dialog\setMessage "Wrapper dialog"
  -- Warcraft hides a dialog when a button is clicked; show it again a moment later.
  reshow = (player) ->
    later = Timer.create!
    later\start 0.5, false, (self) ->
      self\destroy!
      dialog\show player unless dialog\isDisposed!
  dialog\addButton "Keyed (K)", {hotkey: "K"}, (player) ->
    print "Wrapper dialog keyed by", player\getName!
    reshow player
  dialog\addButton "Plain", (player) ->
    print "Wrapper dialog plain by", player\getName!
    reshow player
  dialog\addButton "Rebuild", (player) ->
    dialog\clear!
    dialog\addButton "After clear", (clicker) ->
      print "Wrapper dialog after clear by", clicker\getName!
      reshow clicker
    dialog\addButton "Destroy dialog", ->
      dialog\destroy!
      print "Wrapper dialog destroyed from its own button"
      uiBoards owner
    print "Wrapper dialog rebuilt"
    reshow player
  dialog\show owner
  print "Wrapper dialog shown: press K, click Plain, Rebuild, After clear, then Destroy dialog"
```

Replace the final `mw.on_main` block so it reads:

```text
mw.on_main ->
  start = Timer.create!
  start\start 0, false, (self) ->
    self\destroy!
    owner = Player.fromIndex 0
    if ui
      uiGate owner
    elseif presentation
      presentationGate owner
    else
      foundationGate owner
      broadGate owner
```

Run: `deno task test:integration`
Expected: PASS, including `Gate example: every module builds and editor diagnostics are clean`. If LuaLS reports a
diagnostic in the gate, fix the gate code (for example a nullable result), not the library. If YueScript rejects
`local leaders, quest, optional, first, defeat, countdown, clock`, declare them as `leaders, quest, optional, first,
defeat, countdown, clock = nil, nil, nil, nil, nil, nil, nil` instead.

- [ ] **Step 2: Update `CONTRIBUTING.md`'s in-game gate**

In step 1, the sentence that ends "ends at about 20 seconds with `Wrapper presentation cleanup passed`." (it wraps
across two lines) gets this clause before its final period: "; the classic UI run (step 8) never prints it either and
ends about 45 seconds after its dialog is destroyed, with `Wrapper ui cleanup passed`".

Insert after step 7 a new step 8, and renumber the old steps 8, 9 and 10 to 9, 10 and 11:

```markdown
8. Classic UI (v0.4.0): first run the gate map's `ui-init` probe (`../wrappers-gate`, `deno task gate ui-init`,
   instructions in its `PROBE-UI.md`) and record its answers in the results below and in README. Then set `ui = true`
   and run again; only the classic UI gate runs. A dialog titled `Wrapper dialog` appears with `Keyed (K)`, `Plain` and
   `Rebuild` (`Wrapper dialog shown: ...` prints). Press K: the dialog closes, `Wrapper dialog keyed by <your name>`
   prints and the dialog comes back half a second later. Click `Plain`: `Wrapper dialog plain by <your name>`. Click
   `Rebuild`: `Wrapper dialog rebuilt` prints and the dialog comes back with only `After clear` and `Destroy dialog`.
   Click `After clear`: `Wrapper dialog after clear by <your name>`. Click `Destroy dialog`:
   `Wrapper dialog destroyed from its own button` prints, the dialog never comes back, and no `[wrappers] ... failed`
   line prints. Times below count from that click. At once a multiboard titled `Wrapper multiboard` (yellow) appears top
   right with 3 × 3 cells: a yellow `Name` and `Kills` in row 1; `Footman` with its icon in row 2; `Row 3` in row 3;
   footman icons down column 3; a wider first column. `Wrapper multiboard Wrapper multiboard 3 3` prints. At 3 s it
   grows to 6 rows with `Row 6` in the last (`Wrapper multiboard rows 6`); at 6 s it shrinks to 2
   (`Wrapper multiboard rows 2`); at 9 s it minimizes to its title; at 12 s it expands, then disappears
   (`Wrapper multiboard hidden for another player`). At 15 s a leaderboard titled `Wrapper leaderboard` (yellow)
   appears with `Other 9` (blue value) above `You 5`; `Wrapper leaderboard items 2` prints. At 18 s `You (12)` moves to
   the top and the icons disappear. At 21 s `Other` is removed and the board shrinks to one row
   (`Wrapper leaderboard items 1 has other false`). At 24 s the leaderboard disappears, the quest button flashes and
   `Wrapper quests created: open the quest log (F9)` prints: the log lists `Wrapper quest` (required, footman icon, two
   items), `Wrapper optional quest` (optional) and the defeat condition `Wrapper defeat condition`. At 27 s the first
   item shows as completed (`Wrapper quest item completed true`); at 30 s the quest shows completed and the optional
   quest failed (`Wrapper quest completed true optional failed true`). At 33 s a timer dialog `Wrapper timer dialog`
   (yellow title, green time) counts down from about 1:00; at 36 s it counts faster (`speed 4`; record what it shows);
   at 39 s it shows about 0:10; at 42 s it disappears. At 45 s `Wrapper ui cleanup passed; quest item disposed true`
   prints and the quests leave the log. Restore `ui = false`.
```

In the renumbered step 9 (the `--minify` step), change `repeat steps 2–5 and 7` to `repeat steps 2–5, 7 and 8 (without
the probe)`.

In the renumbered step 10 (two-player run), append: `It also covers v0.4.0: `setVisibleFor` on Multiboard and
TimerDialog shows only for that player; `leaderboard:assign` and a dialog shown to one player appear only on that
player's screen; a dialog click by the second player prints that player's name on both machines; no desync.`

- [ ] **Step 3: Update `README.md`**

Replace the intro sentence (lines 3–5) with:

```markdown
Annotated Lua 5.3 library for Warcraft III. It provides Player, Unit, Item, Destructable, Rect, Region, Force, Timer,
Trigger, Group, Effect, TextTag, Sound, Lightning, Image, Ubersplat, FogModifier, Dialog, Multiboard, Leaderboard,
Quest, DefeatCondition and TimerDialog wrappers, editor completion, stable handle identity and explicit cleanup.
```

In the Status paragraph, replace its last sentence `Dialogs, multiboards, frames and other UI types are not wrapped
yet.` with `v0.4.0 (classic UI: dialogs, multiboards, leaderboards, quests, defeat conditions and timer dialogs) is in
development on main; frames are not wrapped yet.`

In "Handles and cleanup", replace `Timer, Trigger, Group, Effect, Rect, Region, Force and the presentation classes stay
cached until you destroy them;` with `Timer, Trigger, Group, Effect, Rect, Region, Force and the presentation and
classic UI classes stay cached until you destroy them;`.

Add a new section after the "Presentation" section (before "## Editor types"):

```markdown
## Classic UI

Dialog, Multiboard, Leaderboard, Quest, DefeatCondition and TimerDialog wrap objects the map owns and destroys, like the
presentation classes. Create and show them from a timer or trigger once the map has started, as the gate does with a
zero-second timer. w3ts reports that dialogs and multiboards cannot be shown during map initialization, and that
creating quests, leaderboards and multiboards there can crash the game (not yet measured by our probe).

**Dialogs.** `dialog:addButton(text, callback)` or `dialog:addButton(text, options?, callback?)` returns a
DialogButton. The callback receives the Player who clicked, runs behind the same error boundary as trigger actions, and
may hide, clear or destroy its own dialog. Options: `hotkey` (one letter or digit), `quit` (the button quits the game
for the clicking player) and `scoreScreen` (with `quit`, show the score screen first). Warcraft hides a dialog when a
button is clicked. Buttons belong to their dialog: `dialog:clear()` and `dialog:destroy()` dispose them, and a disposed
button's callback never runs. Buttons have no `destroy()` and no `fromHandle`; `button:getDialog()` returns the dialog.
`show(Player)` and `hide(Player)` act for one player with the same call on every machine.

**Multiboards.** Rows and columns count from 1. `setCell(row, column, options)`, `setRow(row, options)`,
`setColumn(column, options)` and `setAll(options)` take `value`, `color` (`{r, g, b, a?}`), `icon`, `width` (a fraction
of the screen width) and `showValue` with `showIcon` (always together); at least one option is required. A cell outside
the board raises an error. The wrapper obtains and releases the native cell handles itself, so none can leak.
`setRowCount` changes the count one row at a time, because w3ts reports that bigger steps are unsafe.
`Multiboard.suppressDisplay(flag)` hides or allows every multiboard. There is no `isMinimized()`: each player minimizes
a multiboard on their own machine.

**Leaderboards.** Items are keyed by player, one per player. `addItem(Player, label, value)` and `removeItem(Player)`
resize the board to fit (a leaderboard starts with no rows); the item setters take the player. `assign(Player)` makes
the leaderboard the one that player sees; then call `show(true)`.

**Quests.** `Quest.create(options?)` takes `title`, `description`, `icon`, `required` (true) and `discovered` (true).
`quest:addItem(description)` returns a QuestItem, which belongs to its quest as a button belongs to its dialog:
`quest:destroy()` disposes it. `Quest.flashButton()` flashes the quest button; `Quest.refresh()` updates an open quest
log. `DefeatCondition.create(description?)` lists a defeat condition in the quest log.

**Timer dialogs.** `TimerDialog.create(Timer, title?)` makes a hidden countdown of that timer; call `show(true)`. It does
not own the timer: destroy the timer dialog first. Once the timer is destroyed, every method except `destroy()` raises
`Timer is disposed`.

`setVisibleFor(Player)` on Multiboard and TimerDialog compares with the local player, as for the presentation classes.
There are no getters for whether a multiboard, leaderboard or timer dialog is displayed or minimized: they answer
differently on each machine.
```

In the API reference table, add rows after `wrappers.fogmodifier`:

```markdown
| `wrappers.dialog` | `create()`; `setMessage(text)`, `addButton(text, options?, callback?)` (returns a DialogButton), `show(Player)`, `hide(Player)`, `clear()`, `destroy()`. DialogButton: `getDialog()` |
| `wrappers.multiboard` | `create(rows, columns, title?)`, `suppressDisplay(flag)`; `setRowCount(count)`, `setColumnCount(count)`, `getRowCount()`, `getColumnCount()`, `setTitle(text)`, `getTitle()`, `setTitleColor(r, g, b, a)`, `setCell(row, column, options)`, `setRow(row, options)`, `setColumn(column, options)`, `setAll(options)`, `show(flag)`, `setVisibleFor(Player)`, `minimize(flag)`, `destroy()` |
| `wrappers.leaderboard` | `create(label?)`; `setLabel(text)`, `setLabelColor(r, g, b, a)`, `setValueColor(r, g, b, a)`, `setStyle(options?)`, `addItem(Player, label, value)`, `removeItem(Player)`, `setItemValue(Player, value)`, `setItemLabel(Player, label)`, `setItemLabelColor(Player, r, g, b, a)`, `setItemValueColor(Player, r, g, b, a)`, `setItemStyle(Player, options?)`, `hasItem(Player)`, `getItemCount()`, `sortByValue(ascending)`, `sortByLabel(ascending)`, `sortByPlayer(ascending)`, `assign(Player)`, `show(flag)`, `destroy()` |
| `wrappers.quest` | `create(options?)`, `flashButton()`, `refresh()`; `setTitle(text)`, `setDescription(text)`, `setIcon(path)`, `setRequired(flag)`/`isRequired()`, `setCompleted(flag)`/`isCompleted()`, `setFailed(flag)`/`isFailed()`, `setDiscovered(flag)`/`isDiscovered()`, `setEnabled(flag)`/`isEnabled()`, `addItem(description)` (returns a QuestItem), `destroy()`. QuestItem: `setDescription(text)`, `setCompleted(flag)`, `isCompleted()`, `getQuest()` |
| `wrappers.defeatcondition` | `create(description?)`; `setDescription(text)`, `destroy()` |
| `wrappers.timerdialog` | `create(Timer, title?)`; `setTitle(text)`, `setTitleColor(r, g, b, a)`, `setTimeColor(r, g, b, a)`, `setSpeed(factor)`, `setRealTimeRemaining(seconds)`, `show(flag)`, `setVisibleFor(Player)`, `destroy()` |
```

Run `deno fmt` to realign the table, then `deno fmt --check`.

- [ ] **Step 4: Update `CHANGELOG.md`**

Insert above `## 0.3.1 (2026-09-29)`:

```markdown
## Unreleased

- New classic UI wrappers: Dialog, Multiboard, Leaderboard, Quest, DefeatCondition and TimerDialog, all owned by the map
  and destroyed explicitly.
- Dialog buttons take a callback that receives the clicking Player; the dialog owns one internal trigger. Buttons and
  quest items belong to their dialog or quest, which dispose them on `clear()`/`destroy()`.
- Multiboard rows and columns count from 1; cell methods (`setCell`, `setRow`, `setColumn`, `setAll`) obtain and release
  the native cell handles themselves. `setRowCount` changes the count one row at a time.
- Leaderboard items are keyed by player and resize the board.
- `setVisibleFor(Player)` on Multiboard and TimerDialog. No getters for display or minimized state.
- A TimerDialog raises once its Timer is destroyed, except for `destroy()`.
```

- [ ] **Step 5: Update the wrappers `AGENTS.md`**

In the list of design links, add after the v0.3.0 entry:

```markdown
- `../moonwell/docs/superpowers/specs/2026-09-29-moonwell-wrappers-classic-ui-design.md` and
  `../moonwell/docs/superpowers/plans/2026-09-29-moonwell-wrappers-classic-ui.md` (v0.4.0)
```

Replace `Classic UI and frames remain in Moonwell's backlog.` with `v0.4.0 adds classic UI (dialogs, multiboards,
leaderboards, quests, defeat conditions, timer dialogs). Frames remain in Moonwell's backlog.`

Add to "Rules":

```markdown
- Children that die with their parent (dialog buttons, quest items) are wrappers owned by the parent: its `clear()` or
  `destroy()` disposes them, and they have no `destroy()` or `fromHandle`. Native handles that must be released
  (multiboard items) never become wrappers: get and release them in the same call.
```

In "Verification", append to the paragraph's end: `v0.4.0: the gate map has a `ui-init` probe
(`../wrappers-gate/src/probe_ui.yue`, `deno task gate ui-init`) and the runs `ui` and `ui-min`.`

- [ ] **Step 6: Run all wrappers checks and commit**

Run every command in "Running the wrappers checks". Expected: all pass.

```bash
git add examples/gate.yue README.md CHANGELOG.md CONTRIBUTING.md AGENTS.md
git commit -m "docs: classic UI wrappers, their in-game gate and changelog"
```

- [ ] **Step 7: Update Moonwell's `AGENTS.md`**

In `C:/Users/mdlsvensson/Repo/moonwell/AGENTS.md`:

After the "Wrappers v0.3.1, native caveats, released" bullet in "State", add:

```markdown
- **Wrappers v0.4.0, classic UI, implemented; in-game gate pending** (spec
  `docs/superpowers/specs/2026-09-29-moonwell-wrappers-classic-ui-design.md`, plan
  `docs/superpowers/plans/2026-09-29-moonwell-wrappers-classic-ui.md`): release B of the UI backlog item. Dialog
  (per-button callbacks, buttons owned by the dialog), Multiboard (one-based cell methods that release every cell
  handle, rows changed one at a time), Leaderboard (items keyed by player), Quest with QuestItem, DefeatCondition and
  TimerDialog. Automated checks pass; the maintainer runs the gate map's `ui-init` probe, then CONTRIBUTING's gate with
  `ui = true` (`deno task gate ui`, then `ui-min`).
```

Replace "Next work, in order" item 1 with:

```markdown
1. **Finish wrappers v0.4.0:** the `ui-init` probe and the in-game gate (the wrappers repo's CONTRIBUTING step 8,
   normal and minified); record the probe's answers in README and move the measured w3ts notes out of the backlog item
   below; then release it like v0.3.0 (tag `v0.4.0`, tag consumption gate).
2. **Then choose the next sub-project with the maintainer:** wrappers release C (frames), the editor error for effects
   attached to items and destructables, the YueScript port of `wc3-lib` (4d) or the Reforged map preview. Each needs a
   short design or a spec first. 4d has design inputs in the w3ts comparison §2.1 and the WCSharp comparison §2.1.
```

In "Backlog", in the "UI wrappers, releases B and C" bullet, replace `B, v0.4.0: dialog and button, multiboard,
leaderboard, quest, timer dialog.` with `B, v0.4.0: implemented (spec
`docs/superpowers/specs/2026-09-29-moonwell-wrappers-classic-ui-design.md`), gate pending.`

Run Moonwell's checks (AGENTS.md "Checks"); for this docs-only change `deno fmt --check` must pass.

```bash
git add AGENTS.md
git commit -m "docs: record wrappers v0.4.0 implementation state"
```

---

### Task 8: Gate map runs and the `ui-init` probe

**Files (in `C:/Users/mdlsvensson/Repo/wrappers-gate`, not under git):**
- Modify: `gate.ts`
- Create: `src/probe_ui.yue`, `PROBE-UI.md`

**Interfaces:**
- Consumes: `../moonwell-wrappers/examples/gate.yue` with its `ui = false` flag (Task 7), and the v0.4.0 modules.
- Produces: `deno task gate ui|ui-min|ui-init [--no-launch]`.

- [ ] **Step 1: Add the runs to `gate.ts`**

Edit with the file-editing tool. Replace the header comment's run list lines

```ts
//   presentation-min  minified presentation gate (step 8)
```

with

```ts
//   presentation-min  minified presentation gate (step 8)
//   ui-init           v0.4.0 probe of w3ts's classic UI claims (src/probe_ui.yue, PROBE-UI.md)
//   ui                v0.4.0 classic UI gate (step 8)
//   ui-min            minified classic UI gate (step 9)
```

Replace the `runs` declaration with:

```ts
const runs: Record<string, { probes: boolean; presentation: boolean; ui: boolean; minify: boolean }> = {
  "core": { probes: false, presentation: false, ui: false, minify: false },
  "probes": { probes: true, presentation: false, ui: false, minify: false },
  "presentation": { probes: false, presentation: true, ui: false, minify: false },
  "core-min": { probes: false, presentation: false, ui: false, minify: true },
  "presentation-min": { probes: false, presentation: true, ui: false, minify: true },
  "ui": { probes: false, presentation: false, ui: true, minify: false },
  "ui-min": { probes: false, presentation: false, ui: true, minify: true },
};
// Probe runs build a hand-written entry instead of a gate.yue variant.
const probeRuns: Record<string, string> = { "probe": "src/probe.yue", "ui-init": "src/probe_ui.yue" };
```

Replace the `variant` function's signature and first two replacements:

```ts
function variant(probes: boolean, presentation: boolean): string {
```

becomes

```ts
function variant(run: { probes: boolean; presentation: boolean; ui: boolean }): string {
```

and

```ts
  let text = replace(source, "probes = false", `probes = ${probes}`);
  text = replace(text, "presentation = false", `presentation = ${presentation}`);
```

becomes

```ts
  let text = replace(source, "probes = false", `probes = ${run.probes}`);
  text = replace(text, "presentation = false", `presentation = ${run.presentation}`);
  text = replace(text, "ui = false", `ui = ${run.ui}`);
```

In `build`, replace

```ts
  let entry = "src/probe.yue";
  let minify = false;
  if (name !== "probe") {
    const run = runs[name];
    entry = `src/gate_${name.replace("-min", "")}.yue`;
    minify = run.minify;
    await Deno.writeTextFile(join(root, entry), variant(run.probes, run.presentation));
  }
```

with

```ts
  let entry = probeRuns[name];
  let minify = false;
  if (!entry) {
    const run = runs[name];
    entry = `src/gate_${name.replace("-min", "")}.yue`;
    minify = run.minify;
    await Deno.writeTextFile(join(root, entry), variant(run));
  }
```

Replace the usage check

```ts
if (!(name in runs) && name !== "probe") {
  console.error(`Usage: deno task gate <${Object.keys(runs).join("|")}|probe|all> [--no-launch]`);
```

with

```ts
if (!(name in runs) && !(name in probeRuns)) {
  console.error(`Usage: deno task gate <${[...Object.keys(runs), ...Object.keys(probeRuns)].join("|")}|all> [--no-launch]`);
```

- [ ] **Step 2: Write the probe `src/probe_ui.yue`**

Create with the file-editing tool:

```text
-- Wrappers v0.4.0 ui-init probe (Moonwell spec 2026-09-29-moonwell-wrappers-classic-ui-design.md §10): w3ts's
-- unmeasured classic UI claims, on the game version in use. Build and launch with `deno task gate ui-init`;
-- PROBE-UI.md says what to watch. Every result line starts with "PROBE". If the game crashes at start, set one create
-- flag to false at a time and run again to find which type crashes.
import "moonwell" as mw
import "wrappers.player" as Player
import "wrappers.timer" as Timer
import "wrappers.dialog" as Dialog
import "wrappers.multiboard" as Multiboard
import "wrappers.leaderboard" as Leaderboard
import "wrappers.quest" as Quest

createQuest = true
createLeaderboard = true
createMultiboard = true

clock = nil
now = -> string.format "%.2f", clock\getElapsed!
say = (...) -> print "PROBE", now!, ...

-- Made directly in on_main, before the game starts. Text printed then never reaches the log, so results print later.
early = {}

mw.on_main ->
  owner = Player.fromIndex 0
  early.quest = Quest.create {title: "Probe init quest", description: "Created directly in on_main."} if createQuest
  early.leaderboard = Leaderboard.create "Probe init leaderboard" if createLeaderboard
  if createMultiboard
    early.board = Multiboard.create 2, 2, "Probe init board"
    early.board\setAll {value: "init", showValue: true, showIcon: false}
    early.board\show true
  early.dialog = Dialog.create!
  early.dialog\setMessage "Probe init dialog"
  early.dialog\addButton "Close", -> say "init dialog clicked"
  early.dialog\show owner
  clock = Timer.create!
  clock\start 3600, false, -> nil
  steps = Timer.create!
  step = 0
  local direct, stepped, defaults
  steps\start 1, true, (self) ->
    step += 1
    if step == 1
      say "on_main created quest", early.quest ~= nil, "leaderboard", early.leaderboard ~= nil, "multiboard", early.board ~= nil
      say "1) Is the Probe init dialog visible? 2) Is the Probe init board visible top right?"
    elseif step == 8
      early.board\destroy! if early.board
      if early.leaderboard
        early.leaderboard\addItem owner, "Probe", 1
        early.leaderboard\assign owner
        early.leaderboard\show true
        say "3) Is Probe init leaderboard visible now, with one item?"
    elseif step == 14
      early.leaderboard\destroy! if early.leaderboard
      if early.quest
        Quest.flashButton!
        say "4) Open the quest log (F9): is Probe init quest listed?"
    elseif step == 20
      raw = CreateMultiboard!
      MultiboardSetColumnCount raw, 1
      MultiboardSetRowCount raw, 5
      direct = Multiboard.fromHandle raw
      direct\setTitle "Direct 0 to 5"
      direct\setAll {value: "direct", showValue: true, showIcon: false}
      direct\show true
      say "5) Direct MultiboardSetRowCount 0 to 5: rows", direct\getRowCount!, "- does the board show 5 rows?"
    elseif step == 26
      direct\destroy!
      stepped = Multiboard.create 5, 1, "Stepped 0 to 5"
      stepped\setAll {value: "stepped", showValue: true, showIcon: false}
      stepped\show true
      say "6) Stepped by the wrapper: rows", stepped\getRowCount!, "- does the board show 5 rows?"
    elseif step == 32
      stepped\destroy!
      defaults = Multiboard.create 2, 2, "Defaults"
      defaults\show true
      say "7) A new 2 x 2 board with no cell options: what do its cells show (icons, text, nothing)?"
    elseif step == 38
      defaults\destroy!
      early.dialog\destroy! unless early.dialog\isDisposed!
      early.quest\destroy! if early.quest
      self\destroy!
      say "done: screenshot the F12 log"
```

- [ ] **Step 3: Write `PROBE-UI.md`**

```markdown
# Wrappers v0.4.0 ui-init probe: what to do

One run, about 40 seconds. It checks w3ts's classic UI claims that our gates have not measured. From this folder:

    deno task gate ui-init

Every result line in the log starts with `PROBE` and the game time. Answer the numbered questions as they print:

1. At start, is a dialog `Probe init dialog` with a `Close` button visible? (Click it if so.)
2. At start, is a multiboard `Probe init board` visible top right?
3. At 8 s, does `Probe init leaderboard` appear with one item?
4. At 14 s, open the quest log (F9): is `Probe init quest` listed?
5. At 20 s, a board made with one `MultiboardSetRowCount` from 0 to 5: does it show 5 rows of `direct`?
6. At 26 s, the same made by the wrapper one row at a time: 5 rows of `stepped`?
7. At 32 s, a new 2 x 2 board with no cell options: what do the cells show?

If the game crashes at the start, say so; then set `createQuest`, `createLeaderboard` or `createMultiboard` in
`src/probe_ui.yue` to false, one at a time, and run again to find the type that crashes.

At `done`, screenshot the F12 log and paste it with your answers.
```

- [ ] **Step 4: Build every new run without launching**

From `C:/Users/mdlsvensson/Repo/wrappers-gate`:

```powershell
deno task gate ui-init --no-launch
deno task gate ui --no-launch
deno task gate ui-min --no-launch
deno task gate presentation --no-launch
```

Expected: each prints `Gate map: gate-maps/<run>.w3x` and exits 0. The `presentation` build checks that the old runs
still generate. If the probe fails to compile, fix `src/probe_ui.yue` (for example the `local direct, stepped,
defaults` line, as in Task 7 Step 1). Nothing to commit: this folder is not under git. Record in the task report that
the four builds succeeded.

- [ ] **Step 5: Hand over to the maintainer**

Report that the probe and gate runs are ready: `deno task gate ui-init` (PROBE-UI.md), then `deno task gate ui` and
`deno task gate ui-min` (the wrappers repo's CONTRIBUTING step 8 and 9). The release (tag, consumption gate) waits for
those results.
