# Moonwell Wrappers Refactor (v0.6.0) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wrappers v0.6.0. Every wrapper error points at the caller's line. Methods and enumeration get cheaper.
`isAlive()` uses `UnitAlive`, and `exists()` is new. Options errors come out in a deterministic order. The README gets
a per-module API reference, the callback rule and a conventions table.

**Architecture:** `internal/handle.lua` gets a one-lookup happy path and explicit error levels, with an optional `depth`
for helpers that add a frame. Every public function that returns a call into a raising helper wraps it in parentheses,
so it is not a tail call. A sweep test proves both for every class. The other items are local changes to `group.lua`,
`unit.lua`, `item.lua`, `destructable.lua`, `internal/options.lua` and the README.

**Tech Stack:**
- annotated Lua 5.3 (the tests run under `yue -e`, which is Lua 5.4; tail-call behaviour is the same);
- Deno tools, `jsr:@std/*` only;
- LuaLS 3.19.1 and YueScript 0.34.2;
- Moonwell at `../moonwell` (0.5.2, which declares `UnitAlive`).

**Spec:** `docs/superpowers/specs/2026-09-30-moonwell-wrappers-refactor-design.md`

## Global Constraints

- **Repository and commits:** all code is in `C:\Users\mdlsvensson\Repo\moonwell-wrappers`. Commit on `main`. End every
  commit message with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- **Tooling:** no Node.js, no npm packages, no `node:` or `npm:` specifiers.
- **No behaviour change beyond the spec:** no public signature changes, and no error message text changes.
- **Levels, counted from the function that calls `error`:**
  - 3 for registry and Handle helpers and the checkers, called from a public function;
  - 4 for `Options.read`'s `fail`;
  - 2 inside a public function itself.
  - A helper that adds a frame passes `depth` (+1).
- **Never tail-call** a raising helper from a public function; write `return (helper(...))`. A tail call to a native is
  fine.
- **Chunk names:** test files load as `./tests/<suite>.lua`, so an error that points at a test reads
  `./tests/<suite>.lua:<line>: …`.
- **Test closures:** a closure passed to `failsAt` must call the wrapper as a statement, never `return wrapper(...)`,
  because a tail call there hides the position too.
- **Files with backslashes** (Windows paths in YueScript strings) are edited with the file-editing tool, never shell
  heredocs or `sed`.
- **Every check before each commit:**
  - `deno task test`;
  - `deno task check`;
  - `deno task lint`;
  - `deno fmt --check`;
  - `deno task check:lua` (with `MOONWELL_LUAC` set);
  - `deno task test:integration` (with `MOONWELL_LUALS` and `MOONWELL_LUAC` set; see CONTRIBUTING).

  For this machine:

  ```bash
  export MOONWELL_LUALS="C:/Users/mdlsvensson/.antigravity-ide/extensions/sumneko.lua-3.19.1-win32-x64/server/bin/lua-language-server.exe"
  export MOONWELL_LUAC="$(pwd)/.tools/lua53/luac53.exe"
  ```

---

### Task 1: `failsAt` and the blame sweep (red)

**Files:**
- Modify: `tests/support.lua` (after `fails`)
- Create: `tests/blame.lua`

**Interfaces:**
- Produces: `failsAt(fn, fragment)`, a global test helper used by Tasks 3, 4 and 6.

- [ ] **Step 1: Add `failsAt` to `tests/support.lua`, right after `fails`**

```lua
---Like fails, and the error must point at a line in a test file: wrapper errors blame their caller (spec 2026-09-30
---§2). `fn` must call the wrapper as a statement: `return wrapper(...)` is a tail call and hides the position.
function failsAt(fn, fragment)
    local ok, err = pcall(fn)
    assert(not ok, 'expected failure')
    local message = tostring(err)
    assert(message:find(fragment, 1, true), message)
    assert(message:find('^%./tests/[%w_]+%.lua:%d+: '), 'expected the calling test line in: ' .. message)
end
```

- [ ] **Step 2: Create `tests/blame.lua`**

```lua
-- Every wrapper error points at the line that called the public function (spec 2026-09-30 §2). The sweep calls every
-- function of every class with an empty table as its first argument: each "expected <Class> wrapper" error must point
-- at this file. No natives are defined here, so functions that reach a native fail differently and are skipped.
local modules = {
    'defeatcondition', 'destructable', 'dialog', 'effect', 'fogmodifier', 'force', 'frame', 'group', 'image', 'item',
    'leaderboard', 'lightning', 'multiboard', 'player', 'quest', 'rect', 'region', 'sound', 'texttag', 'timer',
    'timerdialog', 'trigger', 'ubersplat', 'unit',
}

test('every function given a wrong wrapper points at its caller', function()
    local wrong = {}
    for _, name in ipairs(modules) do
        local class = require('wrappers.' .. name)
        local checked = 0
        for key, fn in pairs(class) do
            if type(fn) == 'function' then
                local ok, err = pcall(function() fn({}) end)
                local message = tostring(err)
                if not ok and message:find('expected %w+ wrapper') then
                    checked = checked + 1
                    if not message:find('^%./tests/blame%.lua:%d+: ') then
                        wrong[#wrong + 1] = name .. '.' .. tostring(key) .. ' -> ' .. message
                    end
                end
            end
        end
        assert(checked > 0, name .. ': no function was checked')
    end
    table.sort(wrong)
    assert(#wrong == 0, #wrong .. ' misplaced errors:\n' .. table.concat(wrong, '\n'))
end)
```

- [ ] **Step 3: Run it and confirm that it fails**

Run: `deno task test blame`

Expected: FAIL with "misplaced errors", listing, among others, `unit.getX -> ./src/wrappers\unit.lua:44: …` and the
`getHandle`/`isDisposed` functions with no position at all.

- [ ] **Step 4: Commit the red test? No.** It stays uncommitted until Task 3 makes it pass. The tree must be green at
  every commit.

### Task 2: `Handle` levels and the happy path

**Files:**
- Modify: `src/wrappers/internal/handle.lua`
- Modify: `src/wrappers/timerdialog.lua:17-22` (`live`)
- Modify: `src/wrappers/leaderboard.lua:38-43` (`itemOf`)
- Test: `tests/handle.lua` (the existing tests must still pass)

**Interfaces:**
- Produces:
  - `registry.require(value, operation, depth?)`, `registry.dispose(value, operation)` and
    `registry.isDisposed(value, operation)`;
  - `Handle.unwrap(value, name, operation, depth?)`, `Handle.unwrapWidget(value, operation, depth?)` and
    `Handle.created(raw, operation, depth?)`.
  - `depth` defaults to 0, and each helper frame between the public function and the call adds 1.

- [ ] **Step 1: Replace the body of `Handle.new` from `local function member` through `registry.isDisposed`**

```lua
    local function expected(operation) return '[wrappers] ' .. operation .. ': expected ' .. name .. ' wrapper' end
    local function disposed(operation) return '[wrappers] ' .. operation .. ': ' .. name .. ' is disposed' end
    function registry.member(value) return members[value] end
    function registry.wrap(raw)
        if raw == nil then return nil end
        if byHandle[raw] then return byHandle[raw] end
        local value = setmetatable({handle = raw}, class)
        byHandle[raw] = value
        members[value] = raw
        return value
    end
    ---One lookup on the happy path. Errors point at the caller of the public function that called this (level 3),
    ---plus `depth` for helper frames in between.
    function registry.require(value, operation, depth)
        local raw = members[value]
        if raw then return raw end
        local level = 3 + (depth or 0)
        if raw == false then error(disposed(operation), level) end
        error(expected(operation), level)
    end
    function registry.dispose(value, operation)
        local raw = members[value]
        if raw == false then return nil end
        if raw == nil then error(expected(operation), 3) end
        members[value] = false
        byHandle[raw] = nil
        value.handle = nil
        return raw
    end
    function registry.isDisposed(value, operation)
        local raw = members[value]
        if raw == nil then error(expected(operation), 3) end
        return raw == false
    end
```

The field annotations at the top of the file change to `require fun(value: unknown, operation: string, depth:
integer?): H`.

- [ ] **Step 2: Replace `Handle.unwrap`, `Handle.unwrapWidget` and `Handle.created`**

```lua
---Converts a wrapper argument without importing its module: a caller holding one has loaded it. Errors point at the
---caller of the public function (level 3), plus `depth` for helper frames in between.
---@param value unknown
---@param name string
---@param operation string
---@param depth integer?
---@return any
function Handle.unwrap(value, name, operation, depth)
    local registry = loaded[name]
    local raw = registry and registry.member(value)
    if raw then return raw end
    local level = 3 + (depth or 0)
    if raw == false then error('[wrappers] ' .. operation .. ': ' .. name .. ' is disposed', level) end
    error('[wrappers] ' .. operation .. ': expected ' .. name .. ' wrapper', level)
end

---Converts a Unit, Item or Destructable argument.
---@param value unknown
---@param operation string
---@param depth integer?
---@return widget
function Handle.unwrapWidget(value, operation, depth)
    local level = 3 + (depth or 0)
    for _, registry in ipairs(widgets) do
        local raw = registry.member(value)
        if raw == false then error('[wrappers] ' .. operation .. ': ' .. registry.name .. ' is disposed', level) end
        if raw ~= nil then return raw end
    end
    error('[wrappers] ' .. operation .. ': expected Widget wrapper', level)
end

---@generic H
---@param raw H?
---@param operation string
---@param depth integer?
---@return H
function Handle.created(raw, operation, depth)
    if raw == nil then error('[wrappers] ' .. operation .. ': native returned nil', 3 + (depth or 0)) end
    return raw
end
```

- [ ] **Step 3: Give the two helpers that add a frame their depth**

`timerdialog.lua`, in `live`:

```lua
    local raw = registry.require(dialog, operation, 1)
    local timer = timers[dialog]
    if timer ~= nil then Handle.unwrap(timer, 'Timer', operation, 1) end
    return raw
```

`leaderboard.lua`, in `itemOf` (its own `error(..., 3)` stays):

```lua
    local raw = registry.require(board, operation, 1)
    local p = Handle.unwrap(player, 'Player', operation, 1)
```

- [ ] **Step 4: Run the whole suite**

Run: `deno task test`

Expected: every suite passes except `blame`, which still lists the tail-call cases (Task 3).

### Task 3: No tail calls into raising helpers; targeted tests; commit

**Files:**
- Modify: every `src/wrappers/*.lua` with a matching line (72 today, plus `timerdialog.lua`'s `return live(`)
- Test: `tests/unit.lua`, `tests/trigger.lua`, `tests/leaderboard.lua`, `tests/timerdialog.lua`

- [ ] **Step 1: Write the one-off rewrite script `.test-work/untail.ts`** (ignored folder, not committed)

```ts
// Wraps `return <raising helper>(...)` in parentheses so it is not a tail call (spec 2026-09-30 §2.3).
const pattern =
  /return ((?:[a-z][A-Za-z]*Registry|registry)\.(?:require|isDisposed|dispose)|Handle\.(?:created|unwrap|unwrapWidget)|live)\(/;
let total = 0;
for (const entry of Deno.readDirSync("src/wrappers")) {
  if (!entry.name.endsWith(".lua")) continue;
  const path = `src/wrappers/${entry.name}`;
  let text = Deno.readTextFileSync(path);
  let from = 0;
  for (;;) {
    const match = pattern.exec(text.slice(from));
    if (!match) break;
    const start = from + match.index + "return ".length;
    let index = start + match[1].length; // the opening parenthesis
    let depth = 0;
    let quote: string | null = null;
    for (; index < text.length; index++) {
      const c = text[index];
      if (quote) {
        if (c === "\\") index++;
        else if (c === quote) quote = null;
      } else if (c === "'" || c === '"') quote = c;
      else if (c === "(") depth++;
      else if (c === ")" && --depth === 0) break;
    }
    text = text.slice(0, start) + "(" + text.slice(start, index + 1) + ")" + text.slice(index + 1);
    from = index + 2;
    total++;
  }
  Deno.writeTextFileSync(path, text);
}
console.log(`wrapped ${total} returns`);
```

- [ ] **Step 2: Run it, then check that no unwrapped raising return is left**

```bash
deno run -A .test-work/untail.ts
grep -nE "return ((registry|[a-z]+Registry)\.(require|isDisposed|dispose)|Handle\.(created|unwrap|unwrapWidget)|live)\(" src/wrappers/*.lua
```

Expected: `wrapped 73 returns` (or the current count), and the grep prints nothing. Read the diff (`git diff --stat`,
then a sample of the files): every change is exactly `return (`…`)`.

- [ ] **Step 3: Run the sweep**

Run: `deno task test blame`

Expected: PASS. If a function is still listed, it is a raising helper reached through another local helper. Give that
helper a `depth` as in Task 2, Step 3, and re-run.

- [ ] **Step 4: Add the targeted tests**

These are the errors the sweep does not reach. Each goes at the end of its suite, using that suite's existing
stand-ins.

`tests/unit.lua`:

```lua
test('errors point at the caller: disposed receiver, arguments, factories and slots', function()
    local p = Player.fromIndex(0)
    local u = Unit.create(p, 1751543663, 0, 0, 0)
    local gone = Unit.create(p, 1751543663, 0, 0, 0)
    gone:remove()
    failsAt(function() gone:getX() end, 'Unit.getX: Unit is disposed')
    failsAt(function() Unit.create({}, 1, 0, 0, 0) end, 'Unit.create: expected Player wrapper')
    failsAt(function() u:issueTargetOrder('smart', {}) end, 'Unit.issueTargetOrder: expected Widget wrapper')
    failsAt(function() u:issueTargetOrder('smart', gone) end, 'Unit.issueTargetOrder: Unit is disposed')
    failsAt(function() u:getItemInSlot(99) end, 'Unit.getItemInSlot: expected an inventory slot index')
    native('CreateUnit', function() return nil end)
    failsAt(function() Unit.create(p, 1, 0, 0, 0) end, 'Unit.create: native returned nil')
end)
```

(If this suite's `CreateUnit` stand-in is defined with other behaviour that later tests rely on, restore it at the end of
the test with the suite's original definition.)

`tests/trigger.lua`:

```lua
test('token and callback errors point at the caller', function()
    local trigger, other = Trigger.create(), Trigger.create()
    local token = other:addAction(function() end)
    failsAt(function() trigger:removeAction({}) end, 'Trigger.removeAction: expected TriggerAction token')
    failsAt(function() trigger:removeAction(token) end, 'Trigger.removeAction: token belongs to another trigger')
    failsAt(function() trigger:addAction(nil) end, 'Trigger.addAction: expected a callback function')
    failsAt(function() trigger:registerTimerEvent(-1, false) end, 'expected a finite non-negative number')
end)
```

`tests/leaderboard.lua` (through `itemOf`, depth 1):

```lua
test('item errors point at the caller', function()
    local board = Leaderboard.create()
    failsAt(function() board:setItemValue(Player.fromIndex(1), 5) end, 'Leaderboard.setItemValue: player has no item')
    failsAt(function() board:setItemValue({}, 5) end, 'Leaderboard.setItemValue: expected Player wrapper')
end)
```

(Use the suite's local names for `Leaderboard` and `Player`. If it creates boards with arguments, pass the same ones.)

`tests/timerdialog.lua` (through `live`, depth 1):

```lua
test('a destroyed timer is reported at the caller', function()
    local timer = Timer.create()
    local dialog = TimerDialog.create(timer)
    timer:destroy()
    failsAt(function() dialog:setTitle('x') end, 'TimerDialog.setTitle: Timer is disposed')
end)
```

- [ ] **Step 5: Run every check and commit**

Run every check in the Global Constraints.

Expected: every suite passes, including `blame`. The integration and LuaLS runs are clean: the parentheses change no
types.

```bash
git add tests/support.lua tests/blame.lua tests/unit.lua tests/trigger.lua tests/leaderboard.lua tests/timerdialog.lua src/wrappers
git commit -m "fix: wrapper errors point at the caller's line

Explicit error levels with a depth for helper frames, a one-lookup happy path in registry.require, and no tail calls
into raising helpers (a tail call dropped the position entirely). A sweep checks every class.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task 4: Options: levels, depth and sorted checks

**Files:**
- Modify: `src/wrappers/internal/options.lua`
- Modify: `src/wrappers/multiboard.lua` (`cellOptions` passes depth 1)
- Test: `tests/options.lua`, `tests/texttag.lua`, `tests/multiboard.lua`

**Interfaces:**
- Produces: `Options.read(options, fields, operation, depth?)`.

- [ ] **Step 1: Write the failing tests**

`tests/options.lua` (direct calls, so positions are not checked here):

```lua
test('several errors report the first in sorted order', function()
    fails(function() Options.read({zeta = 1, alpha = 2}, fields, 'Test.op') end, "unknown option 'alpha'")
    fails(function() Options.read({size = 'a', name = 1}, fields, 'Test.op') end, "option 'name' expected a string")
end)
```

`tests/texttag.lua` (the public path, so positions are checked):

```lua
test('option errors point at the caller', function()
    failsAt(function() TextTag.float('x', 0, 0, {size = 'big'}) end, "TextTag.float: option 'size' expected a number")
    failsAt(function() TextTag.float('x', 0, 0, {bogus = 1}) end, "TextTag.float: unknown option 'bogus'")
    failsAt(function() TextTag.float('x', 0, 0, {color = {1}}) end, "option 'color' expected {r, g, b, a?} integers")
end)
```

`tests/multiboard.lua` (through `cellOptions`, depth 1):

```lua
test('cell option errors point at the caller', function()
    local board = Multiboard.create(1, 1)
    failsAt(function() board:setCell(1, 1, {bogus = 1}) end, "Multiboard.setCell: unknown option 'bogus'")
    failsAt(function() board:setCell(1, 1, {}) end, 'Multiboard.setCell: expected at least one option')
    failsAt(function() board:setCell(9, 1, {value = 'x'}) end, 'outside 1..1')
end)
```

Run: `deno task test options texttag multiboard`

Expected: FAIL. The sorted test may pass or fail depending on hash order, and the position assertions fail for
`color` and for the multiboard unknown option.

- [ ] **Step 2: Implement in `internal/options.lua`**

Replace `fail`, `color` and `Options.read` with:

```lua
---@param operation string
---@param message string
---@param level integer Counted from fail.
local function fail(operation, message, level) error('[wrappers] ' .. operation .. ': ' .. message, level) end

---@param value unknown
---@param name string
---@param operation string
---@param level integer The level that points at the caller from Options.read; this frame adds one.
---@return integer[]
local function color(value, name, operation, level)
    local message = "option '" .. name .. "' expected {r, g, b, a?} integers"
    if type(value) ~= 'table' or (#value ~= 3 and #value ~= 4) then fail(operation, message, level + 1) end
    for index = 1, #value do
        if not isInteger(value[index]) then fail(operation, message, level + 1) end
    end
    return {value[1], value[2], value[3], value[4] or 255}
end

---Field names in sorted order, once per field table, so every machine reports the same first error.
local orders = setmetatable({}, {__mode = 'k'})
---@param fields MoonwellWrappers.OptionFields
---@return string[]
local function namesOf(fields)
    local names = orders[fields]
    if names == nil then
        names = {}
        for name in pairs(fields) do names[#names + 1] = name end
        table.sort(names)
        orders[fields] = names
    end
    return names
end

---Validates an options table and returns a fresh table with every declared field, defaults filled in. Never modifies
---`options`. Colors come back as fresh {r, g, b, a}; Player options come back as raw player handles. Errors point at
---the caller of the public function (plus `depth` for helper frames in between), and several errors report the first
---in sorted order.
---@param options unknown
---@param fields MoonwellWrappers.OptionFields
---@param operation string
---@param depth integer?
---@return table<string, any>
function Options.read(options, fields, operation, depth)
    depth = depth or 0
    local level = 4 + depth
    if options ~= nil and type(options) ~= 'table' then fail(operation, 'expected an options table', level) end
    local given = options or {}
    local unknown = {}
    for key in pairs(given) do
        if fields[key] == nil then unknown[#unknown + 1] = tostring(key) end
    end
    if #unknown > 0 then
        table.sort(unknown)
        fail(operation, "unknown option '" .. unknown[1] .. "'", level)
    end
    local result = {}
    for _, name in ipairs(namesOf(fields)) do
        local field = fields[name]
        local kind, value = field[1], given[name]
        if value == nil then value = field[2] end
        if value ~= nil then
            if kind == 'color' then
                value = color(value, name, operation, level)
            elseif kind == 'Player' then
                value = Handle.unwrap(value, 'Player', operation, depth + 1)
            elseif not checks[kind](value) then
                fail(operation, "option '" .. name .. "' expected " .. expected[kind], level)
            end
        end
        result[name] = value
    end
    return result
end
```

`multiboard.lua`, in `cellOptions`: `local o = Options.read(options, cellFields, operation, 1)`.

- [ ] **Step 3: Run the tests.** Run `deno task test`; expected: every suite passes.

- [ ] **Step 4: Run every check and commit**

```bash
git add src/wrappers/internal/options.lua src/wrappers/multiboard.lua tests/options.lua tests/texttag.lua tests/multiboard.lua
git commit -m "fix: options errors point at the caller and come in sorted order

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task 5: One-table enumeration

**Files:**
- Modify: `src/wrappers/group.lua` (`members`, `wrapAll`, `applyFilter`, `getUnits`, `forEach`)
- Test: `tests/group.lua` (the existing tests must pass unchanged)

- [ ] **Step 1: Write a test that pins the snapshot semantics** (the existing suite may already have one; keep both)

```lua
test('a unit removed during forEach keeps its disposed wrapper, and getUnits keeps native order', function()
    local group = Group.create()
    local a, b = Unit.fromHandle({}), Unit.fromHandle({})
    group:add(a); group:add(b)
    local seen = {}
    group:forEach(function(unit)
        seen[#seen + 1] = unit
        if unit == a then b:remove() end
    end)
    eq(#seen, 2); eq(seen[1], a); eq(seen[2], b); eq(b:isDisposed(), true)
end)
```

(Use this suite's group stand-ins. If `add`, `BlzGroupGetSize` and `BlzGroupUnitAt` are modelled, the test runs as
written. Otherwise build the group the way the existing `forEach` test does.)

Run: `deno task test group`. Expected: PASS on the old code, since it pins the behaviour.

- [ ] **Step 2: Replace `members` and `wrapAll` with `snapshot`, and update their callers**

```lua
---Wrappers of the members in native order, skipping nil entries. Every unit is wrapped before any callback runs, so a
---unit removed mid-iteration keeps its disposed wrapper. `raws`, when given, receives the raw handle at each index.
---@param raw group
---@param raws unit[]?
---@return MoonwellWrappers.Unit[]
local function snapshot(raw, raws)
    local result = {}
    for index = 0, BlzGroupGetSize(raw) - 1 do
        local unit = BlzGroupUnitAt(raw, index)
        if unit then
            result[#result + 1] = assert(Unit.fromHandle(unit))
            if raws then raws[#result] = unit end
        end
    end
    return result
end

---Runs the filter over a snapshot, then removes rejected units. If the filter raises, the group is cleared and the
---error re-raised, so a half-filtered group never escapes.
---@param raw group
---@param filter (fun(unit: MoonwellWrappers.Unit): any)?
local function applyFilter(raw, filter)
    if filter == nil then return end
    local raws = {}
    local units, rejected = snapshot(raw, raws), {}
    local ok, message = pcall(function()
        for index, unit in ipairs(units) do
            if not filter(unit) then rejected[#rejected + 1] = raws[index] end
        end
    end)
    if not ok then
        GroupClear(raw)
        error(message, 0)
    end
    for _, unit in ipairs(rejected) do GroupRemoveUnit(raw, unit) end
end
```

```lua
function Group:getUnits() return snapshot(registry.require(self, 'Group.getUnits')) end
```

```lua
function Group:forEach(callback)
    local raw = registry.require(self, 'Group.forEach')
    Callback.check(callback, 'Group.forEach')
    for _, unit in ipairs(snapshot(raw)) do callback(unit) end
end
```

Then check that nothing else uses the old names: `grep -n "members(\|wrapAll(" src/wrappers/group.lua` must print
nothing.

- [ ] **Step 3: Run every check and commit**

```bash
git add src/wrappers/group.lua tests/group.lua
git commit -m "perf: group enumeration builds one table

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task 6: `isAlive` through `UnitAlive`; `exists()`

**Files:**
- Modify: `src/wrappers/unit.lua` (`isAlive`, and a new `exists`)
- Modify: `src/wrappers/item.lua` and `src/wrappers/destructable.lua` (new `exists`)
- Modify: `tests/unit.lua`, `tests/item.lua`, `tests/destructable.lua`
- Modify: `tests/editor-positive.lua` (exercise `exists()`)

- [ ] **Step 1: Write the failing tests**

In `tests/unit.lua`, replace the test `isAlive combines the dead type and the removed type id` with:

```lua
test('isAlive asks UnitAlive; exists asks for a type id', function()
    local alive, typeId = true, 1751543663
    native('UnitAlive', function() return alive end)
    native('GetUnitTypeId', function() return typeId end)
    local u = Unit.create(Player.fromIndex(0), 1751543663, 0, 0, 0)
    eq(u:isAlive(), true); eq(u:exists(), true)
    alive = false; eq(u:isAlive(), false); eq(u:exists(), true)
    typeId = 0; eq(u:exists(), false)
    u:remove()
    failsAt(function() u:exists() end, 'Unit.exists: Unit is disposed')
end)
```

(Keep the suite's other uses of `GetUnitTypeId` working. If later tests need the suite's original stand-in, restore it
at the end of this test.) Also add `'exists'` next to `'isAlive'` in the suite's disposed-methods list (line ~266).

In `tests/item.lua` and `tests/destructable.lua`, the same shape, using `GetItemTypeId` and `GetDestructableTypeId`:

```lua
test('exists asks for a type id', function()
    local typeId = 1
    native('GetItemTypeId', function() return typeId end)
    local item = Item.create(1, 0, 0)
    eq(item:exists(), true)
    typeId = 0; eq(item:exists(), false)
end)
```

Run: `deno task test unit item destructable`. Expected: FAIL (`exists` is nil; `isAlive` does not call `UnitAlive`).

- [ ] **Step 2: Implement**

`unit.lua`, replacing `isAlive`:

```lua
---Not dead and not removed, by the UnitAlive native (known to Moonwell since 0.5.1).
---@return boolean
function Unit:isAlive() return UnitAlive(registry.require(self, 'Unit.isAlive')) end
---True while the game still has the unit, dead or alive; false once the game has removed it (decay, or removal by code
---that bypassed this wrapper). A disposed wrapper raises, like every method.
---@return boolean
function Unit:exists() return GetUnitTypeId(registry.require(self, 'Unit.exists')) ~= 0 end
```

`item.lua` and `destructable.lua`, after `isDisposed`:

```lua
---True while the game still has the item; false once it was removed (a used powerup, used-up charges, or removal by
---code that bypassed this wrapper). A disposed wrapper raises, like every method.
---@return boolean
function Item:exists() return GetItemTypeId(registry.require(self, 'Item.exists')) ~= 0 end
```

```lua
---True while the game still has the destructable, dead or alive; false once it was removed by code that bypassed this
---wrapper. A disposed wrapper raises, like every method.
---@return boolean
function Destructable:exists() return GetDestructableTypeId(registry.require(self, 'Destructable.exists')) ~= 0 end
```

`tests/editor-positive.lua`: after the existing `unit:isAlive()` use (or after `local unit = …`), add
`if unit:exists() and unit:isAlive() then unit:kill() end`.

- [ ] **Step 3: Run every check.** The integration's native-call check now sees `UnitAlive` in Moonwell 0.5.2's
  declarations; it must stay clean.

- [ ] **Step 4: Commit**

```bash
git add src/wrappers/unit.lua src/wrappers/item.lua src/wrappers/destructable.lua tests/unit.lua tests/item.lua tests/destructable.lua tests/editor-positive.lua
git commit -m "feat: isAlive uses UnitAlive; exists() for units, items and destructables

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task 7: Documentation

**Files:** `README.md`, `CHANGELOG.md`, `CONTRIBUTING.md`, `AGENTS.md`

- [ ] **Step 1: R6, the API reference as one subsection per module**

Write `.test-work/api.ts` (not committed). It reads `README.md`, finds the table under `## API reference`, and replaces
each row `| \`wrappers.x\` | <content> |` with:

```text
### `wrappers.x`

- <group 1>
- <group 2>
```

A group is the content split on `"; "`, trimmed, with the table's escaped pipes (`\|`) unescaped. The table's
header and separator rows are removed. The text above the table stays, followed by the sentence: "Each module lists its
factories and methods beyond the common handle methods (`fromHandle`, `getHandle`, `isDisposed`, and `destroy` or
`remove`)." Run it, then read the result. Every method in the old table must appear exactly once:

```bash
git diff --word-diff README.md | grep -c "{+"
```

That count is expected to be only structure; check a sample by eye. Add `exists()` to `wrappers.unit`, `wrappers.item`
and `wrappers.destructable`.

- [ ] **Step 2: R7, the rule at the start of `## Callbacks`**

```markdown
Two kinds of callback. Callbacks that run immediately let their errors propagate to your code: `enumInRange`,
`enumInRect`, `enumOfPlayer` and `enumSelected` filters, `Group:forEach`, and the `Item.enumInRect` and
`Destructable.enumInRect` filters. Event callbacks run later, from the game, behind a boundary: an error is printed with
its label and nothing else is affected. The labels are `Timer`, `Trigger`, `Trigger condition` (the condition counts as
false), `Dialog button` and `Frame event`, printed as `[wrappers] <label> callback failed: …` (conditions:
`[wrappers] Trigger condition failed: …`).
```

Check the printed label format against `internal/callback.lua` (`report(label .. ' callback', …)`; `Callback.test`
reports its label as given).

- [ ] **Step 3: R9, the conventions table, as a new section before `## API reference`**

```markdown
## Units and conventions

The wrappers keep each native's conventions, so they differ between classes:

| What | Convention | Where |
| --- | --- | --- |
| Colours | 0–255 per channel | units, effects, text tags, images, classic UI, frames |
| Colours | 0–1 per channel | `lightning:setColor` (`SetLightningColor`) |
| Angles | degrees | unit facing, `TextTag.float`'s `angle` |
| Angles | radians | `effect:setOrientation` (`BlzSetSpecialEffectOrientation`) |
| Visibility | `show(flag)` / `isHidden()` | units and destructables (`ShowUnit`, `IsUnitHidden`) |
| Visibility | `setVisible(flag)` / `isVisible()` | items (`SetItemVisible`, `IsItemVisible`) |
| Durations | seconds | everything except `sound:getDuration()` (milliseconds) |
```

Verify every row against the annotations before writing it (for example, which classes the frame colours use).

- [ ] **Step 4: Error locations, the version floor, `isAlive` and `exists`**

- **"Handles and cleanup":** "Wrapper errors point at the line that called the wrapper."
- **Moonwell 0.5.1 or later** is stated wherever the README and CONTRIBUTING name the Moonwell version. Find them with
  `grep -n "0\.5\.0" README.md CONTRIBUTING.md`, and change only the requirement lines, not the release history.
- **The Widgets section:** a sentence on `exists()` (a removed object's wrapper is not disposed; `exists()` tells you
  it is gone).

- [ ] **Step 5: CHANGELOG**

Add `## Unreleased` with one bullet per change:
- error locations;
- the cheaper method prologue;
- one-table enumeration;
- `isAlive` through `UnitAlive`, with the Moonwell floor;
- `exists()`;
- sorted options errors;
- the README restructure.

- [ ] **Step 6: Run every check and commit**

```bash
git add README.md CHANGELOG.md CONTRIBUTING.md AGENTS.md
git commit -m "docs: per-module API reference, the callback rule, conventions and exists()

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task 8: Gate additions

**Files:** `examples/gate.yue` (the `probes` block), `CONTRIBUTING.md` (step 6)

- [ ] **Step 1: Add to the `if probes` block of `examples/gate.yue`, after the condition probe**

Use the file-editing tool. The body of the `pcall` closure must not end in the call: YueScript returns the last
expression, and that would be a tail call.

```yue
    -- v0.6.0: exists() after removal that bypasses the wrapper, and where a wrong-type error points.
    gone = Unit.create owner, $FourCC("hfoo"), -700, -700, 0
    RemoveUnit gone\getHandle!
    corpse = Unit.create owner, $FourCC("hfoo"), -700, -600, 0
    corpse\kill!
    potion = Item.create $FourCC("phea"), -700, -500
    RemoveItem potion\getHandle!
    stump = Destructable.create $FourCC("LTlt"), -704, -384, 270, 1, 0
    RemoveDestructable stump\getHandle!
    print "Wrapper exists after raw removal", gone\exists!, potion\exists!, stump\exists!
    print "Wrapper killed unit alive, exists", corpse\isAlive!, corpse\exists!
    _, located = pcall ->
      hero\issueTargetOrder "smart", {}
      return
    print "Wrapper error location", located
```

If `owner` or `hero` has another name in that scope, use the names already there.

- [ ] **Step 2: Update CONTRIBUTING step 6**

Add the expected lines:
- `Wrapper exists after raw removal false false false`: an item or destructable printing `true` means its `exists()`
  must be dropped before release;
- `Wrapper killed unit alive, exists false true`;
- `Wrapper error location <gate source file>:<line>: [wrappers] Unit.issueTargetOrder: expected Widget wrapper`, where
  the file must be the gate's own source, not a `wrappers/*.lua` file.

- [ ] **Step 3: Run every check.** `test:integration` builds the gate example with clean editor diagnostics.

- [ ] **Step 4: Commit**

```bash
git add examples/gate.yue CONTRIBUTING.md
git commit -m "test: gate checks exists() after raw removal and where errors point

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task 9: Release candidate, gate and release

- [ ] **Step 1: The final review** of the whole diff since `v0.5.1`, against the spec, by Opus.
- [ ] **Step 2: The maintainer's in-game gate** (spec §8):
  - `core`, `probes`, `presentation`, `ui` and `frames`, and the minified runs, as CONTRIBUTING lists them;
  - `deno task gate perf`, compared with v0.5.1's `PROBE-PERF-RESULTS.md`.

  Record the results in CONTRIBUTING and the CHANGELOG, including the measured saving.
- [ ] **Step 3: If an item or destructable `exists()` printed `true`,** remove it, its tests and its docs, and say why
  in the CHANGELOG.
- [ ] **Step 4: The release.**
  - Set the CHANGELOG heading to `## 0.6.0 (<date>)`, and the README status and tag example to `v0.6.0`.
  - Commit, tag `v0.6.0`, push, and create the GitHub pre-release.
  - Run tag consumption as in CONTRIBUTING, with a fresh `init --link` map.
- [ ] **Step 5: Moonwell's records:**
  - AGENTS.md: the state bullet, and next work (roadmap 2.3, the v0.7.0 spec);
  - the roadmap's 2.2 status;
  - a Moonwell CHANGELOG documentation line.
