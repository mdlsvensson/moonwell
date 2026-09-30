# Moonwell Systems Release 1 (v0.1.0) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A new library, `moonwell-systems`, released as v0.1.0 with `systems.scheduler`, `systems.signal`,
`systems.scope` and `systems.time`, plus the Lua-only tooling (test runner, syntax check, integration) every later
release reuses.

**Architecture:** Annotated Lua 5.3 under `src/systems/`. Pure modules take plain values; `Scheduler:start()` is the one
place release 1 touches the game, through a wrappers `Timer`. Two internal modules hold the callback boundary and
receiver checks. The tooling is Lua run with `yue -e`: `tests/run.lua` runs suites in fresh environments,
`tools/check.lua` runs `luac53 -p`, `tools/integration.lua` drives the Moonwell CLI and LuaLS through `io.popen`.

**Tech Stack:**
- annotated Lua 5.3 (tests run under `yue -e`, Lua 5.4 with 64-bit integers);
- YueScript 0.34.2 (`yue`), LuaLS 3.19.1, Lua 5.3.6 `luac`;
- Moonwell at `../moonwell` (0.5.2) and moonwell-wrappers at `../moonwell-wrappers` (v0.7.0);
- no Deno inside the new repository.

**Spec:** `docs/superpowers/specs/2026-09-30-moonwell-systems-design.md`

## Global Constraints

- **Repositories:** the library is `C:\Users\mdlsvensson\Repo\moonwell-systems` (created in Task 1); Moonwell records
  are in `C:\Users\mdlsvensson\Repo\moonwell`. Commit on `main`, staging explicit paths only. End every commit message
  with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Commit only when the task's checks pass, and run each
  check as its own command (never behind a pipe that hides its exit code).
- **No Node.js, no Deno configuration** in the new repository; tools are Lua run with `yue -e`.
- **Messages:** `[systems] <Class>.<method>: <problem>`. Exact texts used in this plan:
  `expected Scheduler`, `expected Signal`, `expected Scope`, `expected a callback function`,
  `expected a finite positive step`, `expected a finite non-negative delay`, `expected a finite positive interval`,
  `the scheduler is disposed`, `already started`, `cannot advance during a tick`, `the signal is disposed`,
  `expected a finite priority`, `expected a value with dispose, destroy or remove`, `expected a date table`,
  `expected a date with numeric year, month, day, hour, minute and second`, `expected a number`.
- **Callback labels:** `Scheduler task`, `Signal listener`, `Scope release`; printed as
  `[systems] <label> failed: <message>`, and `[systems] <label> error handler failed: <message>` when `onError` itself
  fails.
- **Error levels:** 2 inside a public function; 3 (+ `depth`) in a helper it calls. Never tail-call a raising helper:
  `return (helper(...))`.
- **Test positions:** suites load with `dofile('tests/<suite>.lua')`, so an error that points at a test reads
  `tests/<suite>.lua:<line>: …`. A closure passed to `failsAt` calls the function as a statement.
- **Time range:** −2,147,483,648 to 2,147,483,647 Unix seconds (1901-12-13 20:45:52 to 2038-01-19 03:14:07).
- **Environment variables** (all optional): `MOONWELL_WRAPPERS` (wrappers checkout, default `../moonwell-wrappers`),
  `MOONWELL_CLI` (command line, default `deno run -A <abs>/../moonwell/cli/src/main.ts`), `MOONWELL_YUE` (default
  `yue`), `MOONWELL_LUALS` (default `lua-language-server`), `MOONWELL_LUAC` (default `luac`). Pkl must be on `PATH` for
  `init`.
- **Checks** (from the new repository root): `yue -e tests/run.lua`, `yue -e tools/check.lua`,
  `yue -e tools/integration.lua`.

---

### Task 1: Repository skeleton and the test runner

**Files:**
- Create: `../moonwell-systems/.gitignore`, `.gitattributes`, `LICENSE`
- Create: `tests/run.lua`, `tests/suites.lua`, `tests/support.lua`, `tests/smoke.lua` (removed again in Task 2)

**Interfaces:**
- Produces: the runner contract. A suite is `tests/<name>.lua`, listed in `tests/suites.lua`, using the globals from
  `tests/support.lua`: `eq(actual, expected)`, `fails(fn, fragment)`, `failsAt(fn, fragment)`,
  `native(name, implementation)`, `resetCalls()`, `callCount(name)`, `totalCalls()`, `expectCall(name, ...)`,
  `test(name, fn)`, `finish()` (returns `tests, failed`), and `PRINTED` (lines printed during the current test).

- [ ] **Step 1: Create the repository**

```bash
mkdir -p /c/Users/mdlsvensson/Repo/moonwell-systems && cd /c/Users/mdlsvensson/Repo/moonwell-systems && git init -b main
cp ../moonwell-wrappers/LICENSE LICENSE
```

`.gitignore`:

```
.test-work/
.tools/
.moonwell/
*.log
```

`.gitattributes`:

```
* text=auto eol=lf
```

- [ ] **Step 2: Write the support helpers**

`tests/support.lua`:

```lua
-- Test helpers, loaded fresh for every suite by tests/run.lua. A copy of the wrappers' helpers without their
-- Warcraft fixtures.
local calls, implementations, tests, failed = {}, {}, 0, 0
local realPrint = print
PRINTED = {}
print = function(...)
    local parts = table.pack(...)
    for index = 1, parts.n do parts[index] = tostring(parts[index]) end
    PRINTED[#PRINTED + 1] = table.concat(parts, '\t', 1, parts.n)
end

function eq(actual, expected)
    if actual ~= expected then error('expected ' .. tostring(expected) .. ', got ' .. tostring(actual), 2) end
end

function fails(fn, fragment)
    local ok, err = pcall(fn)
    assert(not ok, 'expected failure')
    assert(tostring(err):find(fragment, 1, true), tostring(err))
end

---Like fails, and the error must point at a line in a test file: errors blame their caller (spec §4.2).
function failsAt(fn, fragment)
    local ok, err = pcall(fn)
    assert(not ok, 'expected failure')
    local message = tostring(err)
    assert(message:find(fragment, 1, true), message)
    assert(message:find('^tests/[%w_]+%.lua:%d+: '), 'expected the calling test line in: ' .. message)
end

function native(name, implementation)
    implementations[name] = implementation
    _G[name] = function(...)
        calls[#calls + 1] = {name = name, args = table.pack(...)}
        return implementations[name](...)
    end
end

function resetCalls() calls = {}; PRINTED = {} end
function callCount(name)
    local count = 0
    for _, call in ipairs(calls) do if call.name == name then count = count + 1 end end
    return count
end
function totalCalls() return #calls end
function expectCall(name, ...)
    local expected = table.pack(...)
    for index = #calls, 1, -1 do
        if calls[index].name == name then
            eq(calls[index].args.n, expected.n)
            for position = 1, expected.n do eq(calls[index].args[position], expected[position]) end
            return
        end
    end
    error('no call to ' .. name, 2)
end

function test(name, fn)
    tests = tests + 1
    resetCalls()
    local ok, err = pcall(fn)
    if not ok then failed = failed + 1; realPrint('  FAIL ' .. name .. ': ' .. tostring(err)) end
end

function finish() return tests, failed end
```

- [ ] **Step 3: Write the runner**

`tests/run.lua`:

```lua
-- Runs the behavior suites, each in a fresh environment: yue -e tests/run.lua [suite ...] (spec §5).
local wrappers = os.getenv('MOONWELL_WRAPPERS') or '../moonwell-wrappers'
package.path = './src/?.lua;./tests/?.lua;' .. wrappers .. '/src/?.lua;' .. package.path
local names = {...}
if #names == 0 then names = dofile('tests/suites.lua') end

local globals, loaded = {}, {}
for key, value in pairs(_G) do globals[key] = value end
for key in pairs(package.loaded) do loaded[key] = true end
local realPrint = print

-- Restores the globals and forgets every module loaded since the runner started.
local function reset()
    local extra = {}
    for key in pairs(_G) do if globals[key] == nil then extra[#extra + 1] = key end end
    for _, key in ipairs(extra) do _G[key] = nil end
    for key, value in pairs(globals) do _G[key] = value end
    local modules = {}
    for key in pairs(package.loaded) do if not loaded[key] then modules[#modules + 1] = key end end
    for _, key in ipairs(modules) do package.loaded[key] = nil end
end

local failures = 0
for _, name in ipairs(names) do
    if not name:match('^[a-z]+$') then error('invalid suite name: ' .. name) end
    reset()
    local ok, tests, failed = pcall(function()
        dofile('tests/support.lua')
        dofile('tests/' .. name .. '.lua')
        return finish()
    end)
    reset()
    if not ok then
        failures = failures + 1
        realPrint(name .. ': ERROR ' .. tostring(tests))
    elseif failed > 0 then
        failures = failures + 1
        realPrint(name .. ': ' .. failed .. '/' .. tests .. ' tests failed')
    else
        realPrint(name .. ': SUITE PASSED: ' .. tests .. ' tests')
    end
end
if failures > 0 then
    realPrint(failures .. ' suite(s) failed')
    os.exit(1)
end
realPrint('All ' .. #names .. ' suites passed')
```

`tests/suites.lua`:

```lua
-- Every behavior suite, in run order. tools/check.lua fails when a suite file is missing here.
return {'smoke'}
```

- [ ] **Step 4: Prove the runner with a throwaway suite**

`tests/smoke.lua`:

```lua
LEAK = true
test('a passing test', function() eq(1, 1) end)
test('a failing test is counted', function() eq(1, 2) end)
test('print is captured', function() print('hidden', 2); eq(PRINTED[1], 'hidden\t2') end)
test('failsAt sees this file', function() failsAt(function() error('boom', 1) end, 'boom') end)
```

Run: `cd /c/Users/mdlsvensson/Repo/moonwell-systems && yue -e tests/run.lua; echo "exit $?"`
Expected: `  FAIL a failing test is counted: …expected 2, got 1`, then `smoke: 1/4 tests failed`,
`1 suite(s) failed`, `exit 1`. (`error('boom', 1)` points at the closure's line in `tests/smoke.lua`.)

Then check isolation: run `yue -e tests/run.lua smoke smoke; echo "exit $?"` — both runs report `1/4`, so the second
started fresh. Delete the failing test line from `tests/smoke.lua`, run `yue -e tests/run.lua; echo "exit $?"`, expect
`smoke: SUITE PASSED: 3 tests`, `All 1 suites passed`, `exit 0`.

- [ ] **Step 5: Commit**

```bash
git add .gitignore .gitattributes LICENSE tests/run.lua tests/suites.lua tests/support.lua tests/smoke.lua
git commit -m "chore: repository skeleton and the Lua test runner"
```

---

### Task 2: The internal modules

**Files:**
- Create: `src/systems/internal/callback.lua`, `src/systems/internal/check.lua`
- Create: `tests/internal.lua`; Modify: `tests/suites.lua`; Delete: `tests/smoke.lua`

**Interfaces:**
- Produces:
  - `Callback.check(value, operation, depth?)`: raises `expected a callback function` at level `3 + depth`;
  - `Callback.optional(value, operation, depth?)`: the same for a non-nil non-function;
  - `Callback.call(label, onError, fn, ...) -> boolean`: runs `fn(...)` behind the boundary;
  - `Check.receiver(value, class, name, operation, depth?) -> value`: raises `expected <name>` at level `3 + depth`
    unless `getmetatable(value) == class`;
  - `Check.finite(value) -> boolean`.

- [ ] **Step 1: Write the failing tests**

`tests/internal.lua`:

```lua
local Callback = require('systems.internal.callback')
local Check = require('systems.internal.check')

-- Stand-ins for public functions: the checks raise at the caller of these.
local function api(value) Callback.check(value, 'Probe.api') end
local function maybe(value) Callback.optional(value, 'Probe.maybe') end
local function helper(value) Callback.check(value, 'Probe.helper', 1) end
local function viaHelper(value) helper(value) end
local Class = {}
local function method(self) Check.receiver(self, Class, 'Probe', 'Probe.method') end

test('call runs the function with its arguments and reports success', function()
    local seen
    eq(Callback.call('Probe', nil, function(a, b) seen = a + b end, 2, 3), true)
    eq(seen, 5); eq(#PRINTED, 0)
end)

test('a failure goes to onError, or is printed', function()
    local messages = {}
    eq(Callback.call('Probe', function(message) messages[#messages + 1] = message end, error, 'first'), false)
    eq(#messages, 1); eq(messages[1], 'first'); eq(#PRINTED, 0)
    eq(Callback.call('Probe', nil, error, 'second'), false)
    eq(#PRINTED, 1); eq(PRINTED[1], '[systems] Probe failed: second')
end)

test('a failing onError is printed with the failure', function()
    eq(Callback.call('Probe', function() error('handler broke', 0) end, error, 'task broke', 0), false)
    eq(#PRINTED, 2)
    eq(PRINTED[1], '[systems] Probe error handler failed: handler broke')
    eq(PRINTED[2], '[systems] Probe failed: task broke')
end)

test('an unprintable error is still reported', function()
    local weird = setmetatable({}, {__tostring = function() error('no') end})
    Callback.call('Probe', nil, error, weird)
    eq(PRINTED[1], '[systems] Probe failed: <unprintable error>')
end)

test('check and optional raise at the public caller', function()
    failsAt(function() api(5) end, 'Probe.api: expected a callback function')
    api(function() end)
    failsAt(function() maybe('x') end, 'Probe.maybe: expected a callback function')
    maybe(nil); maybe(print)
    failsAt(function() viaHelper(nil) end, 'Probe.helper: expected a callback function')
end)

test('receiver and finite', function()
    failsAt(function() method({}) end, 'Probe.method: expected Probe')
    method(setmetatable({}, Class))
    eq(Check.finite(1), true); eq(Check.finite(-2.5), true)
    eq(Check.finite(0 / 0), false); eq(Check.finite(math.huge), false); eq(Check.finite(-math.huge), false)
    eq(Check.finite('1'), false); eq(Check.finite(nil), false)
end)
```

`tests/suites.lua` becomes `return {'internal'}`. Delete `tests/smoke.lua`.

- [ ] **Step 2: Run it to see it fail**

Run: `yue -e tests/run.lua; echo "exit $?"`
Expected: `internal: ERROR …module 'systems.internal.callback' not found`, `exit 1`.

- [ ] **Step 3: Implement**

`src/systems/internal/callback.lua`:

```lua
---The callback boundary (spec 2026-09-30 moonwell-systems §4.3): a copy of the wrappers' one, so no library reaches
---into another's internals. Nothing is rethrown: an error rethrown inside a timer or trigger callback is silent in game.
local Callback = {}

---@param value unknown
---@param operation string
---@param depth integer? Helper frames between the public function and this call.
function Callback.check(value, operation, depth)
    if type(value) ~= 'function' then
        error('[systems] ' .. operation .. ': expected a callback function', 3 + (depth or 0))
    end
end

---Accepts nil or a function.
---@param value unknown
---@param operation string
---@param depth integer?
function Callback.optional(value, operation, depth)
    if value ~= nil and type(value) ~= 'function' then
        error('[systems] ' .. operation .. ': expected a callback function', 3 + (depth or 0))
    end
end

---@param message unknown
---@return string
local function text(message)
    local printable, result = pcall(tostring, message)
    return printable and result or '<unprintable error>'
end

---Runs `fn(...)` behind the boundary. A failure goes to `onError(message)`, itself behind the boundary, or is printed
---as `[systems] <label> failed: <message>`.
---@param label string
---@param onError fun(message: string)?
---@param fn function
---@param ... any
---@return boolean succeeded
function Callback.call(label, onError, fn, ...)
    local ok, message = pcall(fn, ...)
    if ok then return true end
    local reported = text(message)
    if onError then
        local handled, failure = pcall(onError, reported)
        if handled then return false end
        print('[systems] ' .. label .. ' error handler failed: ' .. text(failure))
    end
    print('[systems] ' .. label .. ' failed: ' .. reported)
    return false
end

return Callback
```

`src/systems/internal/check.lua`:

```lua
---Argument and receiver checks shared by the systems modules.
local Check = {}

---Returns `value` when its metatable is `class`; raises `expected <name>` at the public function's caller otherwise.
---@generic T
---@param value unknown
---@param class T
---@param name string
---@param operation string
---@param depth integer? Helper frames between the public function and this call.
---@return T
function Check.receiver(value, class, name, operation, depth)
    if getmetatable(value) ~= class then error('[systems] ' .. operation .. ': expected ' .. name, 3 + (depth or 0)) end
    return value
end

---True for a number that is neither NaN nor infinite. (In Warcraft's Lua NaN compares equal to itself, so NaN cannot be
---detected there; this check catches it everywhere else.)
---@param value unknown
---@return boolean
function Check.finite(value)
    return type(value) == 'number' and value == value and value ~= math.huge and value ~= -math.huge
end

return Check
```

- [ ] **Step 4: Run the tests**

Run: `yue -e tests/run.lua; echo "exit $?"`
Expected: `internal: SUITE PASSED: 6 tests`, `All 1 suites passed`, `exit 0`.

- [ ] **Step 5: Commit**

```bash
git add src/systems/internal/callback.lua src/systems/internal/check.lua tests/internal.lua tests/suites.lua
git rm -q tests/smoke.lua
git commit -m "feat: the callback boundary and receiver checks"
```

---

### Task 3: `systems.scheduler`, the pure part

**Files:**
- Create: `src/systems/scheduler.lua`, `tests/scheduler.lua`; Modify: `tests/suites.lua`

**Interfaces:**
- Consumes: `Callback.check/optional/call`, `Check.receiver/finite`.
- Produces: `Scheduler.new(stepSeconds?, onError?)`, `:after(seconds, fn) -> fun()`, `:every(seconds, fn) -> fun()`,
  `:advance()`, `:getTick()`, `:getElapsed()`, `:getPending()`, `:getStep()`, `:ticks(seconds)`, `:dispose()`.
  Task 4 adds `:start()` and the timer part of `:dispose()`; the fields `timer` and `disposed` are shared with it.

- [ ] **Step 1: Write the failing tests**

`tests/scheduler.lua`:

```lua
local Scheduler = require('systems.scheduler')

local function joined(list) return table.concat(list, ',') end

test('cancels later callbacks and defers additions during dispatch', function()
    local clock = Scheduler.new(0.25)
    local calls, cancel = {}, function() end
    clock:after(0, function()
        calls[#calls + 1] = 'first'; cancel()
        clock:after(0, function() calls[#calls + 1] = 'new' end)
    end)
    cancel = clock:after(0, function() calls[#calls + 1] = 'cancelled' end)
    clock:advance()
    eq(joined(calls), 'first')
    clock:advance()
    eq(joined(calls), 'first,new')
end)

test('rounds deadlines up, recurs and releases cancelled work', function()
    local clock = Scheduler.new(0.25)
    local count = 0
    local cancel = clock:every(0.3, function() count = count + 1 end)
    clock:advance(); eq(count, 0)
    clock:advance(); eq(count, 1)
    clock:advance(); clock:advance(); eq(count, 2)
    cancel(); cancel()
    clock:advance(); clock:advance(); eq(count, 2)
    eq(clock:getPending(), 0); eq(clock:getTick(), 6); eq(clock:getElapsed(), 1.5); eq(clock:getStep(), 0.25)
    clock:dispose(); clock:dispose()
    failsAt(function() clock:after(1, function() end) end, 'Scheduler.after: the scheduler is disposed')
    failsAt(function() clock:every(1, function() end) end, 'Scheduler.every: the scheduler is disposed')
    clock:advance(); eq(clock:getTick(), 6)
end)

test('a failing task is cancelled and reported; advancing during a tick raises', function()
    local messages = {}
    local clock = Scheduler.new(1, function(message) messages[#messages + 1] = message end)
    local later = 0
    clock:every(1, function() clock:advance() end)
    clock:after(1, function() later = later + 1 end)
    clock:advance(); clock:advance()
    eq(#messages, 1)
    assert(messages[1]:find('Scheduler.advance: cannot advance during a tick', 1, true), messages[1])
    eq(later, 1); eq(clock:getPending(), 0); eq(#PRINTED, 0)
end)

test('without onError a failure is printed', function()
    local clock = Scheduler.new(1)
    clock:after(1, function() error('intentional scheduler probe') end)
    clock:advance()
    eq(#PRINTED, 1)
    assert(PRINTED[1]:find('[systems] Scheduler task failed:', 1, true), PRINTED[1])
    assert(PRINTED[1]:find('intentional scheduler probe', 1, true), PRINTED[1])
end)

test('tick rounding absorbs float error in non power-of-two steps', function()
    local clock = Scheduler.new(0.01)
    eq(clock:ticks(0.07), 7); eq(clock:ticks(0.071), 8); eq(clock:ticks(0), 1)
    failsAt(function() clock:ticks(-1) end, 'Scheduler.ticks: expected a finite non-negative delay')
end)

test('the heap runs due tasks by deadline, then creation order, and removes mid-heap tasks', function()
    local clock = Scheduler.new(1)
    local order = {}
    clock:after(3, function() order[#order + 1] = 'c3' end)
    clock:every(1, function() order[#order + 1] = 'r1' end)
    clock:after(1, function() order[#order + 1] = 'a1' end)
    local cancel = clock:after(2, function() order[#order + 1] = 'cancelled' end)
    clock:after(2, function() order[#order + 1] = 'b2' end)
    for index = 0, 19 do clock:after(5 + index % 3, function() end) end
    cancel()
    clock:advance(); clock:advance(); clock:advance()
    eq(joined(order), 'r1,a1,r1,b2,c3,r1')
    eq(clock:getPending(), 21)
end)

test('dispose during a tick stops the rest of the tick', function()
    local clock = Scheduler.new(1)
    local ran = {}
    clock:after(1, function() ran[#ran + 1] = 'first'; clock:dispose() end)
    clock:after(1, function() ran[#ran + 1] = 'second' end)
    clock:advance()
    eq(joined(ran), 'first'); eq(clock:getPending(), 0)
end)

test('arguments are checked at the caller', function()
    failsAt(function() Scheduler.new(0) end, 'Scheduler.new: expected a finite positive step')
    failsAt(function() Scheduler.new(-1) end, 'Scheduler.new: expected a finite positive step')
    failsAt(function() Scheduler.new(math.huge) end, 'Scheduler.new: expected a finite positive step')
    failsAt(function() Scheduler.new('1') end, 'Scheduler.new: expected a finite positive step')
    failsAt(function() Scheduler.new(1, 5) end, 'Scheduler.new: expected a callback function')
    local clock = Scheduler.new()
    eq(clock:getStep(), 1 / 32)
    failsAt(function() clock:after(-1, function() end) end, 'Scheduler.after: expected a finite non-negative delay')
    failsAt(function() clock:after(0 / 0, function() end) end, 'Scheduler.after: expected a finite non-negative delay')
    failsAt(function() clock:after(1, nil) end, 'Scheduler.after: expected a callback function')
    failsAt(function() clock:every(0, function() end) end, 'Scheduler.every: expected a finite positive interval')
    failsAt(function() clock:every(1, 'x') end, 'Scheduler.every: expected a callback function')
    failsAt(function() Scheduler.advance({}) end, 'Scheduler.advance: expected Scheduler')
    failsAt(function() Scheduler.after({}, 1, print) end, 'Scheduler.after: expected Scheduler')
    eq(clock:getPending(), 0)
end)
```

`tests/suites.lua` becomes `return {'internal', 'scheduler'}`.

- [ ] **Step 2: Run it to see it fail**

Run: `yue -e tests/run.lua scheduler; echo "exit $?"`
Expected: `scheduler: ERROR …module 'systems.scheduler' not found`, `exit 1`.

- [ ] **Step 3: Implement**

`src/systems/scheduler.lua`:

```lua
local Callback = require('systems.internal.callback')
local Check = require('systems.internal.check')

---A deterministic fixed-step clock. Delays round up to whole ticks (at least one), and tasks due on the same tick run
---in creation order. Pure: drive it with `advance()`, or with `start()` in a map.
---@class MoonwellSystems.Scheduler
---@field package step number
---@field package onError fun(message: string)?
---@field package heap MoonwellSystems.SchedulerTask[]
---@field package tick integer
---@field package sequence integer
---@field package advancing boolean
---@field package disposed boolean
---@field package timer MoonwellWrappers.Timer?
local Scheduler = {}
Scheduler.__index = Scheduler

---@class MoonwellSystems.SchedulerTask
---@field due integer
---@field order integer Creation sequence; breaks ties between equal deadlines.
---@field interval integer 0 for a one-shot task.
---@field callback function? Nil once cancelled or run.
---@field index integer Position in the heap, or 0 once removed.

---Absorbs single-precision error in seconds / step: 0.07 / 0.01 lands slightly above 7.
local EPSILON = 1e-4

local function earlier(a, b) return a.due < b.due or (a.due == b.due and a.order < b.order) end

---Moves the task at `index` toward the root. Returns whether it moved.
local function up(heap, index)
    local task, moved = heap[index], false
    while index > 1 do
        local parentIndex = index // 2
        local parent = heap[parentIndex]
        if not earlier(task, parent) then break end
        heap[index] = parent; parent.index = index
        index = parentIndex; moved = true
    end
    heap[index] = task; task.index = index
    return moved
end

---Moves the task at `index` toward the leaves.
local function down(heap, index)
    local task, size = heap[index], #heap
    while true do
        local left = index * 2
        if left > size then break end
        local right = left + 1
        local child = (right <= size and earlier(heap[right], heap[left])) and right or left
        if not earlier(heap[child], task) then break end
        heap[index] = heap[child]; heap[index].index = index
        index = child
    end
    heap[index] = task; task.index = index
end

local function sift(heap, index)
    if not up(heap, index) then down(heap, index) end
end

---Idempotent: removing twice, or after the task ran, does nothing.
local function remove(heap, task)
    task.callback = nil
    local index = task.index
    if index == 0 then return end
    task.index = 0
    local last = heap[#heap]
    heap[#heap] = nil
    if last == task then return end
    heap[index] = last; last.index = index
    sift(heap, index)
end

local function validDelay(seconds) return Check.finite(seconds) and seconds >= 0 end
local function toTicks(step, seconds) return math.max(1, math.ceil(seconds / step - EPSILON)) end

---@param stepSeconds number? Seconds per tick; finite and positive. Default 1/32, a 0.03125 s Warcraft timer.
---@param onError fun(message: string)? Receives task failures; default prints them.
---@return MoonwellSystems.Scheduler
function Scheduler.new(stepSeconds, onError)
    if stepSeconds == nil then stepSeconds = 1 / 32 end
    if not Check.finite(stepSeconds) or stepSeconds <= 0 then
        error('[systems] Scheduler.new: expected a finite positive step', 2)
    end
    Callback.optional(onError, 'Scheduler.new')
    return setmetatable({step = stepSeconds, onError = onError, heap = {}, tick = 0, sequence = 0, advancing = false,
        disposed = false}, Scheduler)
end

---@param self unknown
---@param seconds unknown
---@param callback unknown
---@param repeating boolean
---@param operation string
---@return fun()
local function schedule(self, seconds, callback, repeating, operation)
    local scheduler = Check.receiver(self, Scheduler, 'Scheduler', operation, 1)
    if scheduler.disposed then error('[systems] ' .. operation .. ': the scheduler is disposed', 3) end
    if repeating then
        if not Check.finite(seconds) or seconds <= 0 then
            error('[systems] ' .. operation .. ': expected a finite positive interval', 3)
        end
    elseif not validDelay(seconds) then
        error('[systems] ' .. operation .. ': expected a finite non-negative delay', 3)
    end
    Callback.check(callback, operation, 1)
    local ticks = toTicks(scheduler.step, seconds)
    scheduler.sequence = scheduler.sequence + 1
    ---@type MoonwellSystems.SchedulerTask
    local task = {due = scheduler.tick + ticks, order = scheduler.sequence, interval = repeating and ticks or 0,
        callback = callback, index = 0}
    local heap = scheduler.heap
    heap[#heap + 1] = task
    up(heap, #heap)
    return function() remove(scheduler.heap, task) end
end

---Runs `callback` once, `seconds` from now (rounded up to whole ticks, at least one).
---@param seconds number
---@param callback fun()
---@return fun() cancel Idempotent.
function Scheduler:after(seconds, callback) return (schedule(self, seconds, callback, false, 'Scheduler.after')) end

---Runs `callback` every `seconds` (rounded up to whole ticks), starting one interval from now. A repeating task that
---fails is cancelled.
---@param seconds number
---@param callback fun()
---@return fun() cancel Idempotent.
function Scheduler:every(seconds, callback) return (schedule(self, seconds, callback, true, 'Scheduler.every')) end

---Advances one tick and runs every task now due, in (due tick, creation order). Tasks scheduled during the tick are due
---at least one tick later. A failing task is cancelled and reported.
function Scheduler:advance()
    local scheduler = Check.receiver(self, Scheduler, 'Scheduler', 'Scheduler.advance')
    if scheduler.disposed then return end
    if scheduler.advancing then error('[systems] Scheduler.advance: cannot advance during a tick', 2) end
    scheduler.advancing = true
    scheduler.tick = scheduler.tick + 1
    while true do
        local heap = scheduler.heap
        local task = heap[1]
        if not task or task.due > scheduler.tick then break end
        local callback = task.callback
        if task.interval == 0 then
            remove(heap, task)
        else
            task.due = scheduler.tick + task.interval
            sift(heap, 1)
        end
        if not Callback.call('Scheduler task', scheduler.onError, callback) then remove(scheduler.heap, task) end
    end
    scheduler.advancing = false
end

---@return integer
function Scheduler:getTick() return Check.receiver(self, Scheduler, 'Scheduler', 'Scheduler.getTick').tick end
---Simulated seconds so far: getTick() * the step.
---@return number
function Scheduler:getElapsed()
    local scheduler = Check.receiver(self, Scheduler, 'Scheduler', 'Scheduler.getElapsed')
    return scheduler.tick * scheduler.step
end
---Tasks that have not run or been cancelled.
---@return integer
function Scheduler:getPending() return #Check.receiver(self, Scheduler, 'Scheduler', 'Scheduler.getPending').heap end
---@return number
function Scheduler:getStep() return Check.receiver(self, Scheduler, 'Scheduler', 'Scheduler.getStep').step end

---Whole ticks a delay occupies under this scheduler's rounding.
---@param seconds number
---@return integer
function Scheduler:ticks(seconds)
    local scheduler = Check.receiver(self, Scheduler, 'Scheduler', 'Scheduler.ticks')
    if not validDelay(seconds) then error('[systems] Scheduler.ticks: expected a finite non-negative delay', 2) end
    return toTicks(scheduler.step, seconds)
end

---Cancels every task. Scheduling afterwards raises; advancing does nothing. Idempotent.
function Scheduler:dispose()
    local scheduler = Check.receiver(self, Scheduler, 'Scheduler', 'Scheduler.dispose')
    if scheduler.disposed then return end
    scheduler.disposed = true
    for _, task in ipairs(scheduler.heap) do task.callback = nil; task.index = 0 end
    scheduler.heap = {}
end

return Scheduler
```

- [ ] **Step 4: Run the tests**

Run: `yue -e tests/run.lua; echo "exit $?"`
Expected: `internal: SUITE PASSED: 6 tests`, `scheduler: SUITE PASSED: 8 tests`, `exit 0`.

- [ ] **Step 5: Commit**

```bash
git add src/systems/scheduler.lua tests/scheduler.lua tests/suites.lua
git commit -m "feat: systems.scheduler, a deterministic fixed-step clock"
```

---

### Task 4: `Scheduler:start()` and the timer

**Files:**
- Modify: `src/systems/scheduler.lua` (the `require` block, a `stopTimer` helper, `start`, `dispose`)
- Test: `tests/scheduler.lua` (append)

**Interfaces:**
- Consumes: `wrappers.timer` (`Timer.create()`, `timer:start(timeout, periodic, callback)`, `timer:destroy()`), which
  calls the natives `CreateTimer`, `TimerStart`, `PauseTimer`, `DestroyTimer`.
- Produces: `Scheduler:start() -> fun()`.

- [ ] **Step 1: Write the failing test**

Append to `tests/scheduler.lua`:

```lua
test('start drives advance from one periodic timer; stop and dispose destroy it', function()
    local timers = {}
    native('CreateTimer', function()
        local timer = {}
        timers[#timers + 1] = timer
        return timer
    end)
    native('TimerStart', function(timer, timeout, periodic, fn) timer.timeout, timer.periodic, timer.fn = timeout, periodic, fn end)
    native('PauseTimer', function() end)
    native('DestroyTimer', function(timer) timer.destroyed = true end)
    local clock = Scheduler.new(0.5)
    local runs = 0
    clock:every(0.5, function() runs = runs + 1 end)
    local stop = clock:start()
    eq(#timers, 1); eq(timers[1].timeout, 0.5); eq(timers[1].periodic, true)
    failsAt(function() clock:start() end, 'Scheduler.start: already started')
    timers[1].fn(); timers[1].fn()
    eq(clock:getTick(), 2); eq(runs, 2)
    stop(); stop()
    eq(timers[1].destroyed, true); eq(callCount('DestroyTimer'), 1)
    local stopAgain = clock:start()
    eq(#timers, 2)
    stop()
    eq(timers[2].destroyed, nil)
    clock:dispose()
    eq(timers[2].destroyed, true)
    stopAgain()
    eq(callCount('DestroyTimer'), 2)
    failsAt(function() clock:start() end, 'Scheduler.start: the scheduler is disposed')
    failsAt(function() Scheduler.start({}) end, 'Scheduler.start: expected Scheduler')
end)
```

- [ ] **Step 2: Run it to see it fail**

Run: `yue -e tests/run.lua scheduler; echo "exit $?"`
Expected: `  FAIL start drives advance …: …attempt to call a nil value (method 'start')`, `exit 1`.

- [ ] **Step 3: Implement**

In `src/systems/scheduler.lua`, add after the `Check` require:

```lua
local Timer = require('wrappers.timer')
```

Add before `Scheduler:dispose`:

```lua
---Destroys `timer` if it is still the scheduler's; a stale stop function does nothing.
local function stopTimer(scheduler, timer)
    if scheduler.timer ~= timer then return end
    scheduler.timer = nil
    timer:destroy()
end

---Drives `advance()` from one periodic wrappers Timer with this scheduler's step. Importing the module creates nothing.
---@return fun() stop Destroys the timer; idempotent.
function Scheduler:start()
    local scheduler = Check.receiver(self, Scheduler, 'Scheduler', 'Scheduler.start')
    if scheduler.disposed then error('[systems] Scheduler.start: the scheduler is disposed', 2) end
    if scheduler.timer then error('[systems] Scheduler.start: already started', 2) end
    local timer = Timer.create()
    scheduler.timer = timer
    timer:start(scheduler.step, true, function() scheduler:advance() end)
    return function() stopTimer(scheduler, timer) end
end
```

In `Scheduler:dispose`, after `scheduler.heap = {}`, add:

```lua
    if scheduler.timer then stopTimer(scheduler, scheduler.timer) end
```

and change its comment to `---Cancels every task and stops the timer. Scheduling afterwards raises; advancing does
nothing. Idempotent.`

- [ ] **Step 4: Run the tests**

Run: `yue -e tests/run.lua; echo "exit $?"`
Expected: `scheduler: SUITE PASSED: 9 tests`, `exit 0`.

- [ ] **Step 5: Commit**

```bash
git add src/systems/scheduler.lua tests/scheduler.lua
git commit -m "feat: Scheduler:start() drives the clock from one wrappers Timer"
```

---

### Task 5: `systems.signal`

**Files:**
- Create: `src/systems/signal.lua`, `tests/signal.lua`; Modify: `tests/suites.lua`

**Interfaces:**
- Produces: `Signal.new(onError?)`, `:subscribe(callback, priority?) -> fun()`, `:emit(...)`, `:getCount()`,
  `:dispose()`.

- [ ] **Step 1: Write the failing tests**

`tests/signal.lua`:

```lua
local Signal = require('systems.signal')

local function joined(list) return table.concat(list, ',') end

test('snapshots additions and honors removals with priority ordering', function()
    local signal = Signal.new()
    local seen, off = {}, function() end
    signal:subscribe(function(value)
        seen[#seen + 1] = value; off()
        signal:subscribe(function(other) seen[#seen + 1] = other + 10 end)
    end, -1)
    off = signal:subscribe(function(value) seen[#seen + 1] = value + 1 end)
    signal:emit(1)
    eq(joined(seen), '1')
    signal:emit(2)
    eq(joined(seen), '1,2,12')
    signal:dispose(); signal:emit(3)
    eq(joined(seen), '1,2,12')
    failsAt(function() signal:subscribe(function() end) end, 'Signal.subscribe: the signal is disposed')
end)

test('lower priority first, equal priorities in subscription order, every argument passed', function()
    local signal = Signal.new()
    local seen = {}
    signal:subscribe(function(a, b) seen[#seen + 1] = 'five' .. a .. b end, 5)
    signal:subscribe(function(a, b) seen[#seen + 1] = 'low' .. a .. b end, -1)
    signal:subscribe(function(a, b) seen[#seen + 1] = 'zeroA' .. a .. b end)
    local off = signal:subscribe(function(a, b) seen[#seen + 1] = 'zeroB' .. a .. b end, 0)
    eq(signal:getCount(), 4)
    signal:emit('x', 1)
    eq(joined(seen), 'lowx1,zeroAx1,zeroBx1,fivex1')
    off(); off()
    eq(signal:getCount(), 3)
end)

test('a failing listener is isolated; onError receives it', function()
    local messages, count = {}, 0
    local signal = Signal.new(function(message) messages[#messages + 1] = message end)
    signal:subscribe(function() error('intentional signal probe') end)
    signal:subscribe(function() count = count + 1 end)
    signal:emit()
    eq(count, 1); eq(#messages, 1); eq(#PRINTED, 0)
    assert(messages[1]:find('intentional signal probe', 1, true), messages[1])
    local plain = Signal.new()
    plain:subscribe(function() error('printed probe') end)
    plain:emit()
    eq(#PRINTED, 1)
    assert(PRINTED[1]:find('[systems] Signal listener failed:', 1, true), PRINTED[1])
end)

test('arguments are checked at the caller', function()
    failsAt(function() Signal.new(5) end, 'Signal.new: expected a callback function')
    local signal = Signal.new()
    failsAt(function() signal:subscribe(nil) end, 'Signal.subscribe: expected a callback function')
    failsAt(function() signal:subscribe(print, 'high') end, 'Signal.subscribe: expected a finite priority')
    failsAt(function() signal:subscribe(print, math.huge) end, 'Signal.subscribe: expected a finite priority')
    failsAt(function() Signal.emit({}) end, 'Signal.emit: expected Signal')
    eq(signal:getCount(), 0)
end)
```

`tests/suites.lua` becomes `return {'internal', 'scheduler', 'signal'}`.

- [ ] **Step 2: Run it to see it fail**

Run: `yue -e tests/run.lua signal; echo "exit $?"`
Expected: `signal: ERROR …module 'systems.signal' not found`.

- [ ] **Step 3: Implement**

`src/systems/signal.lua`:

```lua
local Callback = require('systems.internal.callback')
local Check = require('systems.internal.check')

---An event with prioritized listeners. Lower priority runs first; equal priorities keep subscription order. Each
---listener runs behind the callback boundary.
---@class MoonwellSystems.Signal
---@field package listeners MoonwellSystems.SignalListener[]
---@field package onError fun(message: string)?
---@field package disposed boolean
local Signal = {}
Signal.__index = Signal

---@class MoonwellSystems.SignalListener
---@field priority number
---@field callback function? Nil once unsubscribed.

---@param onError fun(message: string)? Receives listener failures; default prints them.
---@return MoonwellSystems.Signal
function Signal.new(onError)
    Callback.optional(onError, 'Signal.new')
    return setmetatable({listeners = {}, onError = onError, disposed = false}, Signal)
end

---Adds a listener. Lower `priority` runs first; equal priorities keep subscription order.
---@param callback fun(...: any): any
---@param priority number? Default 0.
---@return fun() unsubscribe Idempotent.
function Signal:subscribe(callback, priority)
    local signal = Check.receiver(self, Signal, 'Signal', 'Signal.subscribe')
    if signal.disposed then error('[systems] Signal.subscribe: the signal is disposed', 2) end
    Callback.check(callback, 'Signal.subscribe')
    if priority == nil then priority = 0 end
    if not Check.finite(priority) then error('[systems] Signal.subscribe: expected a finite priority', 2) end
    ---@type MoonwellSystems.SignalListener
    local listener = {priority = priority, callback = callback}
    local list = signal.listeners
    local position = #list + 1
    for index = 1, #list do
        if list[index].priority > priority then position = index; break end
    end
    table.insert(list, position, listener)
    return function()
        listener.callback = nil
        local current = signal.listeners
        for index = 1, #current do
            if current[index] == listener then table.remove(current, index); return end
        end
    end
end

---Calls every listener with the arguments. Listeners added during the call wait for the next one; removed ones are
---skipped at once.
---@param ... any
function Signal:emit(...)
    local signal = Check.receiver(self, Signal, 'Signal', 'Signal.emit')
    local list = signal.listeners
    local snapshot = table.move(list, 1, #list, 1, {})
    for index = 1, #snapshot do
        local callback = snapshot[index].callback
        if callback then Callback.call('Signal listener', signal.onError, callback, ...) end
    end
end

---@return integer
function Signal:getCount() return #Check.receiver(self, Signal, 'Signal', 'Signal.getCount').listeners end

---Removes every listener. Subscribing afterwards raises; emitting does nothing. Idempotent.
function Signal:dispose()
    local signal = Check.receiver(self, Signal, 'Signal', 'Signal.dispose')
    signal.disposed = true
    for _, listener in ipairs(signal.listeners) do listener.callback = nil end
    signal.listeners = {}
end

return Signal
```

- [ ] **Step 4: Run the tests**

Run: `yue -e tests/run.lua; echo "exit $?"`
Expected: `signal: SUITE PASSED: 4 tests`, `exit 0`.

- [ ] **Step 5: Commit**

```bash
git add src/systems/signal.lua tests/signal.lua tests/suites.lua
git commit -m "feat: systems.signal with prioritized, isolated listeners"
```

---

### Task 6: `systems.scope`

**Files:**
- Create: `src/systems/scope.lua`, `tests/scope.lua`; Modify: `tests/suites.lua`

**Interfaces:**
- Produces: `Scope.new(onError?)`, `:own(release) -> release`, `:add(value) -> value`, `:isActive()`, `:dispose()`.

- [ ] **Step 1: Write the failing tests**

`tests/scope.lua`:

```lua
local Scope = require('systems.scope')

local function joined(list) return table.concat(list, ',') end

test('releases in reverse order, continues after failures and releases late owners at once', function()
    local seen, messages = {}, {}
    local scope = Scope.new(function(message) messages[#messages + 1] = message end)
    scope:own(function() seen[#seen + 1] = 'first' end)
    scope:own(function() error('boom') end)
    scope:add({dispose = function() seen[#seen + 1] = 'second' end})
    eq(scope:isActive(), true)
    scope:dispose(); scope:dispose()
    eq(scope:isActive(), false)
    eq(joined(seen), 'second,first')
    eq(#messages, 1); assert(messages[1]:find('boom', 1, true), messages[1])
    scope:own(function() seen[#seen + 1] = 'late' end)
    scope:add({destroy = function() seen[#seen + 1] = 'late add' end})
    eq(joined(seen), 'second,first,late,late add')
end)

test('without onError a failing release is printed and the others still run', function()
    local seen = {}
    local scope = Scope.new()
    scope:own(function() seen[#seen + 1] = 'ran' end)
    scope:own(function() error('printed release probe') end)
    scope:dispose()
    eq(joined(seen), 'ran'); eq(#PRINTED, 1)
    assert(PRINTED[1]:find('[systems] Scope release failed:', 1, true), PRINTED[1])
end)

test('add prefers dispose, then destroy, then remove, and returns the value', function()
    local seen = {}
    local both = {dispose = function(self) seen[#seen + 1] = 'dispose'; eq(self ~= nil, true) end,
        destroy = function() seen[#seen + 1] = 'destroy' end}
    local destroyable = {destroy = function() seen[#seen + 1] = 'destroy' end}
    local removable = {remove = function() seen[#seen + 1] = 'remove' end}
    local scope = Scope.new()
    eq(scope:add(both), both); eq(scope:add(destroyable), destroyable); eq(scope:add(removable), removable)
    scope:dispose()
    eq(joined(seen), 'remove,destroy,dispose')
end)

test('arguments are checked at the caller', function()
    failsAt(function() Scope.new('x') end, 'Scope.new: expected a callback function')
    local scope = Scope.new()
    failsAt(function() scope:own(5) end, 'Scope.own: expected a callback function')
    failsAt(function() scope:add({}) end, 'Scope.add: expected a value with dispose, destroy or remove')
    failsAt(function() scope:add(7) end, 'Scope.add: expected a value with dispose, destroy or remove')
    failsAt(function() Scope.dispose({}) end, 'Scope.dispose: expected Scope')
end)
```

`tests/suites.lua` becomes `return {'internal', 'scheduler', 'signal', 'scope'}`.

- [ ] **Step 2: Run it to see it fail**

Run: `yue -e tests/run.lua scope; echo "exit $?"`
Expected: `scope: ERROR …module 'systems.scope' not found`.

- [ ] **Step 3: Implement**

`src/systems/scope.lua`:

```lua
local Callback = require('systems.internal.callback')
local Check = require('systems.internal.check')

---An ownership stack: register how to release each thing you create, then dispose once. Releases run in reverse order,
---and one failing release never stops the others.
---@class MoonwellSystems.Scope
---@field package releases fun()[]
---@field package onError fun(message: string)?
---@field package disposed boolean
local Scope = {}
Scope.__index = Scope

local METHODS = {'dispose', 'destroy', 'remove'}

---@param onError fun(message: string)? Receives release failures; default prints them.
---@return MoonwellSystems.Scope
function Scope.new(onError)
    Callback.optional(onError, 'Scope.new')
    return setmetatable({releases = {}, onError = onError, disposed = false}, Scope)
end

---@param scope MoonwellSystems.Scope
---@param release fun()
local function keep(scope, release)
    if scope.disposed then
        Callback.call('Scope release', scope.onError, release)
    else
        scope.releases[#scope.releases + 1] = release
    end
end

---Takes ownership of a release function. Owning after dispose releases at once.
---@param release fun()
---@return fun()
function Scope:own(release)
    local scope = Check.receiver(self, Scope, 'Scope', 'Scope.own')
    Callback.check(release, 'Scope.own')
    keep(scope, release)
    return release
end

---Owns anything with a dispose, destroy or remove method (checked in that order), such as a Scheduler, a Timer or a
---Unit. Adding after dispose releases at once.
---@generic T
---@param value T
---@return T
function Scope:add(value)
    local scope = Check.receiver(self, Scope, 'Scope', 'Scope.add')
    local method
    if type(value) == 'table' then
        for _, name in ipairs(METHODS) do
            if type(value[name]) == 'function' then method = value[name]; break end
        end
    end
    if not method then error('[systems] Scope.add: expected a value with dispose, destroy or remove', 2) end
    keep(scope, function() method(value) end)
    return value
end

---True until dispose().
---@return boolean
function Scope:isActive() return not Check.receiver(self, Scope, 'Scope', 'Scope.isActive').disposed end

---Runs every release in reverse registration order. Idempotent.
function Scope:dispose()
    local scope = Check.receiver(self, Scope, 'Scope', 'Scope.dispose')
    if scope.disposed then return end
    scope.disposed = true
    local releases = scope.releases
    scope.releases = {}
    for index = #releases, 1, -1 do Callback.call('Scope release', scope.onError, releases[index]) end
end

return Scope
```

- [ ] **Step 4: Run the tests**

Run: `yue -e tests/run.lua; echo "exit $?"`
Expected: `scope: SUITE PASSED: 4 tests`, `exit 0`.

- [ ] **Step 5: Commit**

```bash
git add src/systems/scope.lua tests/scope.lua tests/suites.lua
git commit -m "feat: systems.scope, an ownership stack released in reverse"
```

---

### Task 7: `systems.time`

**Files:**
- Create: `src/systems/time.lua`, `tests/time.lua`; Modify: `tests/suites.lua`

**Interfaces:**
- Produces: `Time.isLeapYear(year)`, `Time.utcToUnix(date)`, `Time.unixToUtc(seconds)`, `Time.dayOfWeek(seconds)`,
  `Time.formatUtc(date)`, `Time.formatDuration(seconds)`, `Time.localUtc()`; the class `MoonwellSystems.UtcDate`.

- [ ] **Step 1: Write the failing tests**

`tests/time.lua`:

```lua
local Time = require('systems.time')

local MIN, MAX = -2147483648, 2147483647

local function same(actual, expected)
    for _, key in ipairs({'year', 'month', 'day', 'hour', 'minute', 'second'}) do eq(actual[key], expected[key]) end
end

test('calendar conversion: epoch, negative timestamps, leap days and century rules', function()
    eq(Time.isLeapYear(2000), true); eq(Time.isLeapYear(1900), false); eq(Time.isLeapYear(2024), true)
    eq(Time.isLeapYear(0), false); eq(Time.isLeapYear(2024.5), false)
    eq(Time.utcToUnix({year = 1970, month = 1, day = 1}), 0)
    same(Time.unixToUtc(-1), {year = 1969, month = 12, day = 31, hour = 23, minute = 59, second = 59})
    eq(Time.utcToUnix({year = 2000, month = 2, day = 29, hour = 12}), 951825600)
    eq(Time.utcToUnix({year = 2023, month = 2, day = 29}), nil)
    eq(Time.utcToUnix({year = 2024, month = 13, day = 1}), nil)
    eq(Time.utcToUnix({year = 2024, month = 2, day = 30}), nil)
    eq(Time.utcToUnix({year = 2024, month = 2, day = 1, minute = 60}), nil)
    eq(Time.unixToUtc(0 / 0), nil); eq(Time.unixToUtc(0.5), nil); eq(Time.unixToUtc('0'), nil)
    for _, year in ipairs({1902, 1970, 2000, 2024, 2037}) do
        local date = {year = year, month = 12, day = 31, hour = 23, minute = 59, second = 59}
        same(Time.unixToUtc(Time.utcToUnix(date)), date)
    end
    eq(math.type(Time.utcToUnix({year = 2000.0, month = 1.0, day = 1.0})), 'integer')
end)

test('the range is 32-bit, checked at its edges', function()
    local first = {year = 1901, month = 12, day = 13, hour = 20, minute = 45, second = 52}
    local last = {year = 2038, month = 1, day = 19, hour = 3, minute = 14, second = 7}
    eq(Time.utcToUnix(first), MIN); eq(Time.utcToUnix(last), MAX)
    eq(Time.utcToUnix({year = 1901, month = 12, day = 13, hour = 20, minute = 45, second = 51}), nil)
    eq(Time.utcToUnix({year = 2038, month = 1, day = 19, hour = 3, minute = 14, second = 8}), nil)
    eq(Time.utcToUnix({year = 1900, month = 1, day = 1}), nil); eq(Time.utcToUnix({year = 2100, month = 1, day = 1}), nil)
    same(Time.unixToUtc(MIN), first); same(Time.unixToUtc(MAX), last)
    eq(Time.unixToUtc(MIN - 1), nil); eq(Time.unixToUtc(MAX + 1), nil)
    eq(Time.dayOfWeek(MAX + 1), nil)
end)

test('display helpers format durations, UTC dates and weekdays', function()
    eq(Time.formatDuration(0), '0:00'); eq(Time.formatDuration(65.9), '1:05')
    eq(Time.formatDuration(3725), '1:02:05'); eq(Time.formatDuration(-3), '0:00')
    eq(Time.formatUtc(Time.unixToUtc(951782400)), '2000-02-29 00:00:00')
    eq(Time.formatUtc({year = 12, month = 3, day = 4, hour = 5, minute = 6, second = 7}), '0012-03-04 05:06:07')
    eq(Time.dayOfWeek(0), 4); eq(Time.dayOfWeek(-86400), 3); eq(Time.dayOfWeek(951782400), 2)
    eq(Time.dayOfWeek(0.5), nil)
end)

test('localUtc reads os.time and returns nil when it is missing, raises or is out of range', function()
    local original = os.time
    local ok, err = pcall(function()
        os.time = function() return 1790760132 end
        eq(Time.localUtc(), 1790760132)
        os.time = function() error('unavailable') end
        eq(Time.localUtc(), nil)
        os.time = function() return MAX + 1 end
        eq(Time.localUtc(), nil)
        os.time = function() return 5.5 end
        eq(Time.localUtc(), nil)
        os.time = nil
        eq(Time.localUtc(), nil)
    end)
    os.time = original
    assert(ok, err)
    eq(math.type(Time.localUtc()), 'integer')
end)

test('arguments are checked at the caller', function()
    failsAt(function() Time.utcToUnix(5) end, 'Time.utcToUnix: expected a date table')
    failsAt(function() Time.formatUtc({year = 2000}) end,
        'Time.formatUtc: expected a date with numeric year, month, day, hour, minute and second')
    failsAt(function() Time.formatUtc(nil) end,
        'Time.formatUtc: expected a date with numeric year, month, day, hour, minute and second')
    failsAt(function() Time.formatDuration('5') end, 'Time.formatDuration: expected a number')
end)
```

`tests/suites.lua` becomes `return {'internal', 'scheduler', 'signal', 'scope', 'time'}`.

- [ ] **Step 2: Run it to see it fail**

Run: `yue -e tests/run.lua time; echo "exit $?"`
Expected: `time: ERROR …module 'systems.time' not found`.

- [ ] **Step 3: Implement**

`src/systems/time.lua`:

```lua
---Calendar and display helpers for UTC dates and Unix seconds, and the local clock. Timestamps are Warcraft's 32-bit
---integers: from -2147483648 (1901-12-13 20:45:52) to 2147483647 (2038-01-19 03:14:07). Pure except localUtc().
local Time = {}

---@class MoonwellSystems.UtcDate
---@field year integer
---@field month integer 1..12
---@field day integer 1..31, valid for the month.
---@field hour integer? 0..23; default 0 in utcToUnix.
---@field minute integer? 0..59; default 0 in utcToUnix.
---@field second integer? 0..59; default 0 in utcToUnix.

local MIN, MAX = -2147483648, 2147483647

local function integer(value, low, high)
    return math.type(value) ~= nil and value >= low and value <= high and math.floor(value) == value
end

---Whether `year` (1..9999) is a Gregorian leap year.
---@param year integer
---@return boolean
function Time.isLeapYear(year)
    return integer(year, 1, 9999) and year % 4 == 0 and (year % 100 ~= 0 or year % 400 == 0)
end

local function daysBeforeYear(year)
    local prior = year - 1
    return prior * 365 + prior // 4 - prior // 100 + prior // 400
end

local function daysInMonth(year, month)
    if month == 2 then return Time.isLeapYear(year) and 29 or 28 end
    if month == 4 or month == 6 or month == 9 or month == 11 then return 30 end
    return 31
end

---Converts a UTC date to Unix seconds. Missing time fields read 0.
---@param date MoonwellSystems.UtcDate
---@return integer? seconds Nil when a field is out of range or not an integer, or the date is outside the range.
function Time.utcToUnix(date)
    if type(date) ~= 'table' then error('[systems] Time.utcToUnix: expected a date table', 2) end
    local year, month, day = date.year, date.month, date.day
    local hour, minute, second = date.hour or 0, date.minute or 0, date.second or 0
    if not (integer(year, 1, 9999) and integer(month, 1, 12)) then return nil end
    year, month = math.tointeger(year), math.tointeger(month)
    if not (integer(day, 1, daysInMonth(year, month)) and integer(hour, 0, 23) and integer(minute, 0, 59)
        and integer(second, 0, 59)) then
        return nil
    end
    day, hour, minute, second = math.tointeger(day), math.tointeger(hour), math.tointeger(minute), math.tointeger(second)
    local days = daysBeforeYear(year) - 719162 + day - 1
    for earlier = 1, month - 1 do days = days + daysInMonth(year, earlier) end
    local rest = hour * 3600 + minute * 60 + second
    -- Warcraft's integers wrap silently, so check the range before multiplying: 24855 days and 11647 s is MAX,
    -- -24856 days and 74752 s is MIN.
    if days > 24855 or (days == 24855 and rest > 11647) then return nil end
    if days < -24856 or (days == -24856 and rest < 74752) then return nil end
    if days < 0 then return (days + 1) * 86400 + (rest - 86400) end
    return days * 86400 + rest
end

---Converts Unix seconds to a UTC date.
---@param seconds integer
---@return MoonwellSystems.UtcDate? date Nil for a non-integer or a value outside the range.
function Time.unixToUtc(seconds)
    if not integer(seconds, MIN, MAX) then return nil end
    seconds = math.tointeger(seconds)
    local day, rest = seconds // 86400, seconds % 86400
    local absoluteDay = day + 719162
    local low, high = 1, 10000
    while high - low > 1 do
        local middle = (low + high) // 2
        if daysBeforeYear(middle) <= absoluteDay then low = middle else high = middle end
    end
    local year, month, dayOfYear = low, 1, absoluteDay - daysBeforeYear(low)
    while dayOfYear >= daysInMonth(year, month) do
        dayOfYear = dayOfYear - daysInMonth(year, month)
        month = month + 1
    end
    local hour = rest // 3600
    rest = rest - hour * 3600
    local minute = rest // 60
    return {year = year, month = month, day = dayOfYear + 1, hour = hour, minute = minute, second = rest - minute * 60}
end

---0 = Sunday .. 6 = Saturday.
---@param seconds integer
---@return integer? weekday Nil for a value unixToUtc rejects.
function Time.dayOfWeek(seconds)
    if not integer(seconds, MIN, MAX) then return nil end
    return (math.tointeger(seconds) // 86400 + 4) % 7
end

---"YYYY-MM-DD HH:MM:SS".
---@param date MoonwellSystems.UtcDate Every field present.
---@return string
function Time.formatUtc(date)
    local fields = type(date) == 'table'
        and {date.year, date.month, date.day, date.hour, date.minute, date.second} or {}
    for index = 1, 6 do
        if type(fields[index]) ~= 'number' then
            error('[systems] Time.formatUtc: expected a date with numeric year, month, day, hour, minute and second', 2)
        end
        fields[index] = math.floor(fields[index])
    end
    return string.format('%04d-%02d-%02d %02d:%02d:%02d', table.unpack(fields, 1, 6))
end

---Whole-second countdown text: "M:SS", or "H:MM:SS" from one hour. Negative reads "0:00".
---@param seconds number
---@return string
function Time.formatDuration(seconds)
    if type(seconds) ~= 'number' then error('[systems] Time.formatDuration: expected a number', 2) end
    local total = seconds > 0 and math.floor(seconds) or 0
    local hours = total // 3600
    local minutes = (total - hours * 3600) // 60
    local rest = total - hours * 3600 - minutes * 60
    if hours > 0 then return string.format('%d:%02d:%02d', hours, minutes, rest) end
    return string.format('%d:%02d', minutes, rest)
end

---This machine's clock as Unix seconds (os.time, present in Warcraft 3.0.0.24268). Local and untrusted: sync it before
---it affects shared state.
---@return integer? seconds Nil when os.time is missing, raises or gives a value outside the range.
function Time.localUtc()
    local clock = os.time
    if type(clock) ~= 'function' then return nil end
    local ok, seconds = pcall(clock)
    if ok and integer(seconds, MIN, MAX) then return math.tointeger(seconds) end
    return nil
end

return Time
```

- [ ] **Step 4: Run the tests**

Run: `yue -e tests/run.lua; echo "exit $?"`
Expected: `time: SUITE PASSED: 5 tests`, `All 5 suites passed`, `exit 0`.

- [ ] **Step 5: Commit**

```bash
git add src/systems/time.lua tests/time.lua tests/suites.lua
git commit -m "feat: systems.time, 32-bit calendar helpers and the local clock"
```

---

### Task 8: Sweep, imports, syntax check, fixtures and integration

**Files:**
- Create: `tests/blame.lua`, `tests/imports.lua`; Modify: `tests/suites.lua`
- Create: `tools/lib.lua`, `tools/check.lua`, `tools/integration.lua`
- Create: `tests/editor-positive.lua`, `tests/editor-positive.yue`, `tests/editor-negative.lua`,
  `tests/natives-negative.lua`

**Interfaces:**
- Produces: `tools/lib.lua` for later releases: `Lib.windows`, `Lib.quote(text)`, `Lib.run(command, cwd?) -> code,
  output`, `Lib.must(command, cwd?) -> output`, `Lib.cwd()`, `Lib.slash(path)`, `Lib.read(path)`, `Lib.write(path,
  text)`, `Lib.copy(from, to)`, `Lib.copyTree(from, to)`, `Lib.mkdir(path)`, `Lib.remove(path)`, `Lib.files(dir,
  suffix)`, `Lib.decodeJson(text)`, `Lib.diagnose(luals, project, name)`, `Lib.expectMarked(fixture, report,
  reportedName, label) -> count`.

- [ ] **Step 1: The sweep and the import tests**

`tests/blame.lua`:

```lua
-- Every error a public function raises for wrong arguments is a [systems] error at the caller's line (spec §4.2).
-- The sweep calls every function of every module with an empty table as its first argument.
local modules = {'scheduler', 'signal', 'scope', 'time'}

test('every public function given a wrong argument points at its caller', function()
    local wrong = {}
    for _, name in ipairs(modules) do
        local checked = 0
        for key, fn in pairs(require('systems.' .. name)) do
            if type(fn) == 'function' and key ~= '__index' then
                local ok, err = pcall(function() fn({}) end)
                if not ok then
                    checked = checked + 1
                    local message = tostring(err)
                    if not message:find('^tests/blame%.lua:%d+: %[systems%] ') then
                        wrong[#wrong + 1] = name .. '.' .. key .. ' -> ' .. message
                    end
                end
            end
        end
        assert(checked > 0, name .. ': no function raised')
    end
    table.sort(wrong)
    assert(#wrong == 0, #wrong .. ' misplaced errors:\n' .. table.concat(wrong, '\n'))
end)
```

`tests/imports.lua`:

```lua
-- Importing a module calls no native and creates nothing; time loads no wrappers module (spec §4.1).
test('the time module loads no wrappers module', function()
    require('systems.time')
    eq(totalCalls(), 0)
    for name in pairs(package.loaded) do assert(not name:find('^wrappers%.'), name) end
end)

test('importing every module calls no native', function()
    for _, name in ipairs({'scheduler', 'signal', 'scope', 'time'}) do require('systems.' .. name) end
    eq(totalCalls(), 0)
    eq(package.loaded['wrappers.timer'] ~= nil, true)
    eq(package.loaded['wrappers.unit'], nil)
end)
```

(No natives are defined in these suites, so a native call at import would raise and fail the suite.)

`tests/suites.lua` becomes `return {'internal', 'scheduler', 'signal', 'scope', 'time', 'imports', 'blame'}`.

Run: `yue -e tests/run.lua; echo "exit $?"`
Expected: `All 7 suites passed`, `exit 0`. If the sweep lists a function, fix its level or tail call, not the test.

- [ ] **Step 2: The shared tool library**

`tools/lib.lua`:

```lua
-- Shell, file and LuaLS helpers for the Lua tools (run with yue -e). No Deno (spec §5).
local Lib = {}
Lib.windows = package.config:sub(1, 1) == '\\'

function Lib.quote(text)
    if Lib.windows then return '"' .. text .. '"' end
    return "'" .. text:gsub("'", "'\\''") .. "'"
end

---Converts a path to the platform's separators.
function Lib.native(path)
    if Lib.windows then return (path:gsub('/', '\\')) end
    return path
end

---Forward slashes, for Pkl and URIs.
function Lib.slash(path) return (path:gsub('\\', '/')) end

---Runs a command line, in `cwd` if given. Returns the exit code and the combined output.
function Lib.run(command, cwd)
    local line = command .. ' 2>&1'
    if cwd then line = (Lib.windows and 'cd /d ' or 'cd ') .. Lib.quote(Lib.native(cwd)) .. ' && ' .. line end
    -- cmd.exe strips the outer quotes of a line that starts with a quote; wrap it once more.
    if Lib.windows then line = '"' .. line .. '"' end
    local pipe = assert(io.popen(line, 'r'))
    local output = pipe:read('a')
    local _, _, code = pipe:close()
    return code or 0, output
end

function Lib.must(command, cwd)
    local code, output = Lib.run(command, cwd)
    if code ~= 0 then error(command .. ' (exit ' .. code .. ')\n' .. output, 0) end
    return output
end

function Lib.cwd() return (Lib.must(Lib.windows and 'cd' or 'pwd'):gsub('%s+$', '')) end

function Lib.read(path)
    local file = assert(io.open(path, 'rb'))
    local text = file:read('a')
    file:close()
    return text
end

function Lib.write(path, text)
    local file = assert(io.open(path, 'wb'))
    file:write(text)
    file:close()
end

function Lib.copy(from, to) Lib.write(to, Lib.read(from)) end

function Lib.mkdir(path)
    if Lib.windows then
        Lib.must('if not exist ' .. Lib.quote(Lib.native(path)) .. ' mkdir ' .. Lib.quote(Lib.native(path)))
    else
        Lib.must('mkdir -p ' .. Lib.quote(path))
    end
end

function Lib.copyTree(from, to)
    if Lib.windows then
        Lib.must('xcopy /e /i /q /y ' .. Lib.quote(Lib.native(from)) .. ' ' .. Lib.quote(Lib.native(to)))
    else
        Lib.mkdir(to)
        Lib.must('cp -R ' .. Lib.quote(from) .. '/. ' .. Lib.quote(to))
    end
end

function Lib.remove(path) os.remove(path) end

---Every file under `dir` whose name ends with `suffix`, as paths relative to the current directory, sorted.
function Lib.files(dir, suffix)
    local root = Lib.cwd()
    local output = Lib.windows
        and Lib.must('dir /s /b ' .. Lib.quote(Lib.native(dir) .. '\\*' .. suffix))
        or Lib.must('find ' .. Lib.quote(dir) .. ' -type f -name ' .. Lib.quote('*' .. suffix))
    local files = {}
    for line in output:gmatch('[^\r\n]+') do
        local path = Lib.slash(line)
        local prefix = Lib.slash(root) .. '/'
        if path:sub(1, #prefix) == prefix then path = path:sub(#prefix + 1) end
        files[#files + 1] = path
    end
    table.sort(files)
    return files
end

---A small JSON decoder for LuaLS reports: objects, arrays, strings, numbers, true, false and null (as nil).
function Lib.decodeJson(text)
    local position = 1
    local value
    local function skip() position = text:find('[^ \t\r\n]', position) or #text + 1 end
    local function fail(what) error('invalid JSON at ' .. position .. ': ' .. what, 0) end
    local escapes = {['"'] = '"', ['\\'] = '\\', ['/'] = '/', b = '\b', f = '\f', n = '\n', r = '\r', t = '\t'}
    local function str()
        local parts = {}
        position = position + 1
        while true do
            local char = text:sub(position, position)
            if char == '' then fail('unterminated string') end
            if char == '"' then position = position + 1; return table.concat(parts) end
            if char == '\\' then
                local kind = text:sub(position + 1, position + 1)
                if kind == 'u' then
                    parts[#parts + 1] = utf8.char(tonumber(text:sub(position + 2, position + 5), 16))
                    position = position + 6
                else
                    parts[#parts + 1] = escapes[kind] or fail('bad escape')
                    position = position + 2
                end
            else
                parts[#parts + 1] = char
                position = position + 1
            end
        end
    end
    function value()
        skip()
        local char = text:sub(position, position)
        if char == '{' then
            local object = {}
            position = position + 1; skip()
            if text:sub(position, position) == '}' then position = position + 1; return object end
            while true do
                skip()
                local key = str()
                skip()
                if text:sub(position, position) ~= ':' then fail('expected :') end
                position = position + 1
                object[key] = value()
                skip()
                local next = text:sub(position, position)
                position = position + 1
                if next == '}' then return object end
                if next ~= ',' then fail('expected , or }') end
            end
        elseif char == '[' then
            local array = {}
            position = position + 1; skip()
            if text:sub(position, position) == ']' then position = position + 1; return array end
            while true do
                array[#array + 1] = value()
                skip()
                local next = text:sub(position, position)
                position = position + 1
                if next == ']' then return array end
                if next ~= ',' then fail('expected , or ]') end
            end
        elseif char == '"' then
            return str()
        elseif text:sub(position, position + 3) == 'true' then
            position = position + 4; return true
        elseif text:sub(position, position + 4) == 'false' then
            position = position + 5; return false
        elseif text:sub(position, position + 3) == 'null' then
            position = position + 4; return nil
        else
            local number = text:match('^-?%d+%.?%d*[eE]?[-+]?%d*', position)
            if not number or number == '' then fail('unexpected ' .. char) end
            position = position + #number
            return tonumber(number)
        end
    end
    local result = value()
    skip()
    if position <= #text then fail('trailing text') end
    return result
end

---Runs LuaLS over `project` and returns its report: a table from file URI to a list of diagnostics.
function Lib.diagnose(luals, project, name)
    local version = Lib.must(Lib.quote(luals) .. ' --version')
    if not version:find('3.19.1', 1, true) then error('LuaLS 3.19.1 is required, found: ' .. version, 0) end
    local parent = project .. '/..'
    local report = parent .. '/' .. name .. '.json'
    Lib.remove(report)
    local _, output = Lib.run(Lib.quote(luals) .. ' ' .. table.concat({
        '--check=' .. Lib.quote(Lib.native(project)),
        '--checklevel=Hint',
        '--check_format=json',
        '--check_out_path=' .. Lib.quote(Lib.native(report)),
        '--logpath=' .. Lib.quote(Lib.native(parent .. '/' .. name .. '-logs')),
        '--metapath=' .. Lib.quote(Lib.native(parent .. '/luals-meta')),
    }, ' '))
    Lib.write(parent .. '/' .. name .. '.log', output)
    local ok, text = pcall(Lib.read, report)
    if not ok then return {} end -- LuaLS writes no report when it finds nothing
    return Lib.decodeJson(text) or {}
end

---Compares a report with the `-- EXPECT <code>` markers of `fixture`, reported as a file named `reportedName`. A
---diagnostic in any other file fails. Returns the number of expected diagnostics.
function Lib.expectMarked(fixture, report, reportedName, label)
    local expected, actual = {}, {}
    local line = 0
    for text in (Lib.read(fixture) .. '\n'):gmatch('(.-)\r?\n') do
        local code = text:match('%-%- EXPECT ([%w%-]+)')
        if code then expected[#expected + 1] = line .. ':' .. code end
        line = line + 1
    end
    for file, diagnostics in pairs(report) do
        for _, diagnostic in ipairs(diagnostics) do
            if file:sub(-#reportedName - 1) ~= '/' .. reportedName then
                error(label .. ': unexpected diagnostic in ' .. file .. ': ' .. diagnostic.message, 0)
            end
            actual[#actual + 1] = diagnostic.range.start.line .. ':' .. diagnostic.code
        end
    end
    table.sort(expected); table.sort(actual)
    if table.concat(expected, ' ') ~= table.concat(actual, ' ') then
        error(label .. ': expected ' .. table.concat(expected, ' ') .. '; got ' .. table.concat(actual, ' '), 0)
    end
    return #actual
end

return Lib
```

- [ ] **Step 3: The syntax check**

`tools/check.lua`:

```lua
-- Lua 5.3.6 syntax of every file under src/ and tests/, and every suite listed: yue -e tools/check.lua (spec §5).
package.path = './tools/?.lua;' .. package.path
local Lib = require('lib')
local luac = os.getenv('MOONWELL_LUAC') or 'luac'
local version = Lib.must(Lib.quote(luac) .. ' -v')
if not version:find('Lua 5.3.6', 1, true) then error('Lua 5.3.6 luac is required, found: ' .. version, 0) end
local count = 0
for _, dir in ipairs({'src', 'tests', 'tools'}) do
    for _, file in ipairs(Lib.files(dir, '.lua')) do
        Lib.must(Lib.quote(luac) .. ' -p ' .. Lib.quote(file))
        count = count + 1
    end
end
Lib.mkdir('.test-work')
Lib.write('.test-work/lua54-syntax.lua', 'local x <const> = 1\nreturn x\n')
if Lib.run(Lib.quote(luac) .. ' -p .test-work/lua54-syntax.lua') == 0 then error('Lua 5.4 syntax was accepted', 0) end
-- Every suite file must be listed, so none is skipped silently.
local listed = {}
for _, name in ipairs(dofile('tests/suites.lua')) do listed[name] = true end
local helpers = {run = true, suites = true, support = true}
for _, file in ipairs(Lib.files('tests', '.lua')) do
    local name = file:match('^tests/([a-z]+)%.lua$')
    if name and not helpers[name] and not listed[name] then error('suite not listed in tests/suites.lua: ' .. name, 0) end
end
print('Lua 5.3.6 syntax: ' .. count .. ' files passed; Lua 5.4-only syntax rejected; every suite listed')
```

(Fixture files contain a hyphen, so `^tests/([a-z]+)%.lua$` does not match them.)

Run: `MOONWELL_LUAC=../moonwell-wrappers/.tools/lua53/luac53.exe yue -e tools/check.lua; echo "exit $?"`
Expected: `Lua 5.3.6 syntax: <n> files passed; …; every suite listed`, `exit 0`.

- [ ] **Step 4: The editor fixtures**

`tests/natives-negative.lua`: copy `../moonwell-wrappers/tests/natives-negative.lua` unchanged (its planted native
mistakes prove the native check reports each kind).

`tests/editor-positive.lua`:

```lua
local Scheduler = require('systems.scheduler')
local Signal = require('systems.signal')
local Scope = require('systems.scope')
local Time = require('systems.time')

local scope = Scope.new(function(message) print(message) end)
local clock = scope:add(Scheduler.new(1 / 32))
scope:own(clock:start())
local cancel = clock:every(1, function() print(clock:getElapsed(), clock:getTick(), clock:getPending()) end)
clock:after(0.5, function() cancel() end)
print(clock:ticks(0.5), clock:getStep())
local changed = scope:add(Signal.new())
scope:own(changed:subscribe(function(value, text) print(value, text) end, -1))
changed:emit(1, 'one')
print(changed:getCount())
local now = Time.localUtc()
if now then
    local date = Time.unixToUtc(now)
    if date then print(Time.formatUtc(date), Time.dayOfWeek(now)) end
end
print(Time.formatDuration(125), Time.isLeapYear(2024), Time.utcToUnix({year = 2000, month = 2, day = 29}))
print(scope:isActive())
scope:dispose()
return true
```

`tests/editor-positive.yue`:

```
import "moonwell" as mw
import "systems.scheduler" as Scheduler
import "systems.signal" as Signal
import "systems.scope" as Scope
import "systems.time" as Time

mw.on_main ->
  scope = Scope.new!
  clock = scope\add Scheduler.new!
  scope\own clock\start!
  clock\every 1, -> print clock\getElapsed!
  changed = scope\add Signal.new!
  changed\subscribe (value) -> print value
  changed\emit 1
  print Time.formatDuration 65
```

`tests/editor-negative.lua`:

```lua
local Scheduler = require('systems.scheduler')
local Signal = require('systems.signal')
local Scope = require('systems.scope')
local Time = require('systems.time')
local clock = Scheduler.new()
clock:after('1', function() end) -- EXPECT param-type-mismatch
clock:nonexistent() -- EXPECT undefined-field
Scope.new():own(5) -- EXPECT param-type-mismatch
Time.formatDuration('5') -- EXPECT param-type-mismatch
Signal.new():subscribe(function() end, 'high') -- EXPECT param-type-mismatch
local date = Time.unixToUtc(0)
print(date.year) -- EXPECT need-check-nil
return true
```

If LuaLS reports a different code for a line that is genuinely wrong, change that line's marker to the reported code and
note it in the commit message; if it reports nothing for a line, that is a missing annotation to fix in `src/`.

- [ ] **Step 5: The integration script**

`tools/integration.lua`:

```lua
-- Builds a consumer map with both libraries, runs LuaLS and checks one-module bundles: yue -e tools/integration.lua.
package.path = './tools/?.lua;' .. package.path
local Lib = require('lib')

local root = Lib.slash(Lib.cwd())
local function absolute(path)
    if path:match('^%a:[/\\]') or path:sub(1, 1) == '/' then return Lib.slash(path) end
    return root .. '/' .. path
end
local wrappers = absolute(os.getenv('MOONWELL_WRAPPERS') or '../moonwell-wrappers')
local cli = os.getenv('MOONWELL_CLI') or ('deno run -A ' .. Lib.quote(Lib.native(absolute('../moonwell/cli/src/main.ts'))))
local yue = os.getenv('MOONWELL_YUE') or 'yue'
local luals = os.getenv('MOONWELL_LUALS') or 'lua-language-server'

local work = root .. '/.test-work/integration-' .. os.time()
local consumer = work .. '/consumer'
Lib.mkdir(work)
print('Consumer: ' .. consumer)
local function moonwell(args) return Lib.must(cli .. ' ' .. args, consumer) end

Lib.must(cli .. ' init --link ' .. Lib.quote(Lib.native(consumer)), root)
Lib.write(consumer .. '/moonwell.local.pkl', table.concat({
    'amends "moonwell.pkl"',
    'libraries {',
    '  ["wrappers"] { path = "' .. wrappers .. '"; dir = "src" }',
    '  ["systems"] { path = "' .. root .. '"; dir = "src" }',
    '}',
    os.getenv('MOONWELL_YUE') and ('yue { path = "' .. Lib.slash(absolute(yue)) .. '" }') or '',
    '',
}, '\n'))

local function compileEditor()
    Lib.must(Lib.quote(yue) .. ' -l -c --target=5.3 --path ' .. Lib.quote(consumer .. '/.moonwell/yue/?.lua')
        .. ' -o src/main.lua src/main.yue', consumer)
end

-- Positive fixtures: a clean check, normal and minified builds, and no editor diagnostics.
Lib.copy('tests/editor-positive.yue', consumer .. '/src/main.yue')
moonwell('check'); moonwell('build'); moonwell('build --minify')
compileEditor()
Lib.copy('tests/editor-positive.lua', consumer .. '/lua/positive.lua')
local positive = Lib.diagnose(luals, consumer, 'positive')
for file, diagnostics in pairs(positive) do
    if #diagnostics > 0 then error('Positive editor diagnostics in ' .. file .. ': ' .. diagnostics[1].message, 0) end
end
print('LuaLS: positive Lua and compiled Yue fixtures clean')

Lib.copy('tests/editor-negative.lua', consumer .. '/lua/negative.lua')
local intended = Lib.expectMarked('tests/editor-negative.lua', Lib.diagnose(luals, consumer, 'negative'),
    'negative.lua', 'Editor negative fixture')
Lib.remove(consumer .. '/lua/negative.lua')
print('LuaLS: ' .. intended .. ' intentional type errors detected at the expected lines')

-- The library's own files against Moonwell's native declarations, with the wrappers they require.
local source = work .. '/source'
Lib.copyTree('src/systems', source .. '/systems')
Lib.copyTree(wrappers .. '/src/wrappers', source .. '/wrappers')
Lib.mkdir(source .. '/types')
for _, name in ipairs({'natives.d.lua', 'moonwell.d.lua'}) do
    Lib.copy(consumer .. '/.moonwell/types/' .. name, source .. '/types/' .. name)
end
Lib.copy('tests/natives-negative.lua', source .. '/natives-negative.lua')
Lib.write(source .. '/.luarc.json', '{"runtime.version": "Lua 5.3", "runtime.path": ["?.lua", "?/init.lua"], '
    .. '"runtime.builtin": {"io": "disable", "debug": "disable", "package": "disable"}, '
    .. '"workspace.library": ["types"], "workspace.useGitIgnore": false, "workspace.checkThirdParty": false}')
local planted = Lib.expectMarked('tests/natives-negative.lua', Lib.diagnose(luals, source, 'natives'),
    'natives-negative.lua', 'Native-call check')
print('LuaLS: src/systems is clean against Moonwell\'s natives; ' .. planted .. ' planted mistakes detected')

-- A map importing one entry point bundles only that module, what it requires, and the wrappers it names.
local public = {'scheduler', 'signal', 'scope', 'time'}
local wrappersPublic = {'unit', 'player', 'item', 'destructable', 'rect', 'region', 'force', 'group', 'timer', 'effect',
    'trigger', 'texttag', 'sound', 'lightning', 'image', 'ubersplat', 'fogmodifier', 'dialog', 'multiboard',
    'leaderboard', 'quest', 'defeatcondition', 'timerdialog', 'frame', 'damage', 'sync'}
local entries = {
    time = {source = 'import "systems.time" as Time\nprint Time.formatDuration 5\n', wrappers = {}},
    signal = {source = 'import "systems.signal" as Signal\ns = Signal.new!\ns\\dispose!\n', wrappers = {}},
    scope = {source = 'import "systems.scope" as Scope\ns = Scope.new!\ns\\dispose!\n', wrappers = {}},
    scheduler = {source = 'import "systems.scheduler" as Scheduler\nc = Scheduler.new!\nc\\dispose!\n',
        wrappers = {timer = true}},
}
local function bundles(bundle, name)
    return bundle:find(name .. '"', 1, true) ~= nil or bundle:find(name .. "'", 1, true) ~= nil
end
for _, entry in ipairs(public) do
    Lib.write(consumer .. '/src/main.yue', entries[entry].source)
    moonwell('build')
    local bundle = Lib.read(consumer .. '/dist/stage/map.w3x/war3map.lua')
    if not bundles(bundle, 'systems.' .. entry) then error(entry .. '-only bundle lacks systems.' .. entry, 0) end
    for _, other in ipairs(public) do
        if other ~= entry and bundles(bundle, 'systems.' .. other) then
            error(entry .. '-only bundle includes systems.' .. other, 0)
        end
    end
    for _, name in ipairs(wrappersPublic) do
        local allowed = entries[entry].wrappers[name]
        if bundles(bundle, 'wrappers.' .. name) ~= (allowed == true) then
            error(entry .. '-only bundle: wrappers.' .. name .. (allowed and ' missing' or ' included'), 0)
        end
    end
end
print('Moonwell: Scheduler-, Signal-, Scope- and Time-only maps bundle only what they import')

-- The gate example builds and has clean editor diagnostics.
local gate = io.open('examples/gate.yue', 'rb')
if gate then
    gate:close()
    Lib.copy('examples/gate.yue', consumer .. '/src/main.yue')
    Lib.remove(consumer .. '/lua/positive.lua')
    moonwell('check'); moonwell('build --minify')
    compileEditor()
    for file, diagnostics in pairs(Lib.diagnose(luals, consumer, 'gate')) do
        if #diagnostics > 0 then error('Gate example diagnostics in ' .. file .. ': ' .. diagnostics[1].message, 0) end
    end
    print('Gate example: builds and editor diagnostics are clean; game execution remains manual')
end
print('Integration passed')
```

(The `bundles` check matches a module name followed by a closing quote, so `wrappers.timer` does not match
`wrappers.timerdialog`.)

- [ ] **Step 6: Run everything**

```bash
export MOONWELL_LUAC=../moonwell-wrappers/.tools/lua53/luac53.exe
export MOONWELL_LUALS="C:/Users/mdlsvensson/.antigravity-ide/extensions/sumneko.lua-3.19.1-win32-x64/server/bin/lua-language-server.exe"
yue -e tests/run.lua; echo "tests exit $?"
yue -e tools/check.lua; echo "check exit $?"
yue -e tools/integration.lua; echo "integration exit $?"
```

Expected: `All 7 suites passed`; the syntax line; integration prints the positive, negative (6), native (4 planted),
bundle lines and `Integration passed`; every exit 0. Fix the tools on Windows as needed (quoting, separators) and the
library annotations if LuaLS flags them; never weaken a check to pass.

- [ ] **Step 7: Commit**

```bash
git add tests/blame.lua tests/imports.lua tests/suites.lua tools/lib.lua tools/check.lua tools/integration.lua tests/editor-positive.lua tests/editor-positive.yue tests/editor-negative.lua tests/natives-negative.lua
git commit -m "test: sweep, imports, Lua syntax check, editor fixtures and integration in Lua"
```

---

### Task 9: Docs, the gate example and the gate map run

**Files:**
- Create: `README.md`, `AGENTS.md`, `CONTRIBUTING.md`, `CHANGELOG.md`, `examples/gate.yue`
- Modify (not under git): `../wrappers-gate/gate.ts`, `../wrappers-gate/moonwell.local.pkl`

- [ ] **Step 1: The gate example**

`examples/gate.yue`:

```
-- The moonwell-systems in-game gate. Built by the gate map's `systems` run (../wrappers-gate, deno task gate systems);
-- CONTRIBUTING lists every expected message. It starts just after the map loads.
import "moonwell" as mw
import "moonwell.macros" as {:$FourCC}
import "wrappers.player" as Player
import "wrappers.unit" as Unit
import "wrappers.timer" as Timer
import "systems.scheduler" as Scheduler
import "systems.signal" as Signal
import "systems.scope" as Scope
import "systems.time" as Time

systemsGate = (owner) ->
  clock = Scheduler.new!
  clock\start!
  reference = Timer.create!
  reference\start 60, false, -> print "Systems reference timer expired"
  clock\after 1, ->
    print "Systems after 1 s: reference", reference\getElapsed!, "elapsed", clock\getElapsed!
  halves = 0
  stopHalves = clock\every 0.5, ->
    halves += 1
    print "Systems every 0.5 s: run", halves, "reference", reference\getElapsed!
  clock\after 2.1, -> stopHalves!
  order = {}
  for name in *{"first", "second", "third"}
    clock\after 0.25, -> table.insert order, name
  clock\after 0.3, -> print "Systems same-tick order:", table.concat order, " "
  failures = 0
  clock\every 0.5, ->
    failures += 1
    error "intentional systems probe #{failures}"
  signal = Signal.new!
  seen = {}
  signal\subscribe ((value) -> table.insert seen, "five:#{value}"), 5
  signal\subscribe ((value) -> table.insert seen, "low:#{value}"), -1
  signal\subscribe (-> error "intentional signal probe"), 0
  signal\emit 7
  print "Systems signal order:", table.concat seen, " "
  scope = Scope.new!
  footman = scope\add Unit.create owner, $FourCC("hfoo"), 0, 0, 270
  held = scope\add Timer.create!
  scope\own -> print "Systems scope release function ran first"
  clock\after 3, ->
    scope\dispose!
    print "Systems scope disposed: timer disposed", held\isDisposed!, "unit disposed", footman\isDisposed!
  now = Time.localUtc!
  if now
    date = Time.unixToUtc now
    print "Systems local UTC", now, date and Time.formatUtc(date) or "out of range", "weekday", Time.dayOfWeek now
  else
    print "Systems local UTC unavailable"
  print "Systems duration 3725 s", Time.formatDuration 3725
  clock\after 4, ->
    tick = clock\getTick!
    clock\dispose!
    later = Timer.create!
    later\start 1, false, (self) ->
      self\destroy!
      reference\destroy!
      print "Systems gate done: tick at dispose", tick, "tick now", clock\getTick!, "failures", failures
  print "Systems gate started"

mw.on_main ->
  start = Timer.create!
  start\start 0, false, (self) ->
    self\destroy!
    systemsGate Player.fromIndex 0
```

(`for name in *{…}` gives each closure its own `name`. Every function ends in a statement, not a trailing loop.)

- [ ] **Step 2: README, AGENTS, CONTRIBUTING, CHANGELOG**

`README.md`: title `# Moonwell Systems`; a paragraph (opt-in Warcraft III systems for Moonwell maps, ported from
wc3-lib, annotated Lua 5.3 on moonwell-wrappers); **Status** `v0.1.0 (<date>)`: scheduler, signal, scope and time;
the `libraries` configuration from spec §2 (both libraries, tags `v0.7.0` and `v0.1.0`) and a local-checkout variant
with `path`; "Rules" (spec §4 in short: nothing at import, explicit dispose, callbacks isolated with `onError` or
printed, errors at the caller, determinism, 32-bit numbers); one API section per module with the signatures of spec
§7–§10 and a short YueScript example each (a started scheduler with `after`/`every`/cancel; a signal with priorities;
a scope holding a scheduler, a timer and a unit; `formatUtc(unixToUtc(Time.localUtc()))` with the "local and
untrusted" note); "Changes from wc3-lib" (the list of spec §7–§10 marked *new* or changed: printed failures instead of
rethrows, `start()`, isolated signal listeners, scope `add` with destroy/remove, the 32-bit time range, `os.time`,
dropped `SimulationTime`/`LocalWallTime`).

`AGENTS.md`: handoff for coding agents: what this is (spec path in `../moonwell`), the rules of spec §4, the tooling of
§5 with the commands and environment variables, the release process (spec, plan, TDD, gate, tag), and the pitfalls
(`yue -e` is Lua 5.4 with 64-bit integers, so test ranges at their edges; NaN is undetectable in game; the runner's
fresh environment does not restore tables mutated in place, such as `os.time`; cmd.exe quoting in `tools/lib.lua`).

`CONTRIBUTING.md`: tools (YueScript 0.34.2, LuaLS 3.19.1, Lua 5.3.6 `luac` — the wrappers' CONTRIBUTING shows how to
build `luac53.exe` —, Pkl on PATH, Moonwell and moonwell-wrappers checkouts beside this one); the three check commands;
the in-game gate for v0.1.0, `deno task gate systems` in `../wrappers-gate`, with the expected messages:
1. `Systems gate started`, `Systems signal order: low:7 five:7` and one `[systems] Signal listener failed: …intentional
   signal probe`, `Systems local UTC <n> <date> weekday <d>` (record it), `Systems duration 3725 s 1:02:05`;
2. `Systems same-tick order: first second third`;
3. `[systems] Scheduler task failed: …intentional systems probe 1` exactly once;
4. `Systems every 0.5 s: run 1..4` with the reference near 0.5, 1.0, 1.5, 2.0; `Systems after 1 s: reference ~1.0
   elapsed 1.0`;
5. at 3 s `Systems scope release function ran first`, then `Systems scope disposed: timer disposed true unit disposed
   true`, and the footman disappears;
6. at 5 s `Systems gate done: tick at dispose <t> tick now <t> failures 1` (the two ticks equal), and no systems line
   after it;
then the publication and tag-consumption steps (as in the wrappers' CONTRIBUTING, with both libraries).

`CHANGELOG.md`: `# Changelog`, `## Unreleased`, bullets for the four modules and the changes from wc3-lib.

- [ ] **Step 3: The gate map run**

In `../wrappers-gate/moonwell.local.pkl`, inside `libraries { … }`, add
`["systems"] { path = "../moonwell-systems"; dir = "src" }`.

In `../wrappers-gate/gate.ts`:
- header comment: `//   systems           moonwell-systems gate (../moonwell-systems/examples/gate.yue)`;
- a map of runs copied from another repository:
  `const copiedRuns: Record<string, string> = { "systems": "../moonwell-systems/examples/gate.yue" };`
- in `build`, before the `probeRuns` lookup: if `copiedRuns[name]`, copy that file to `src/gate_<name>.yue` and use it
  as the entry, not minified.

Run from `../wrappers-gate`: `deno task gate systems --no-launch`
Expected: `Gate map: gate-maps/systems.w3x`.

- [ ] **Step 4: Checks and commit**

Run the three checks of Task 8 Step 6 (integration now also builds the gate example). Then:

```bash
git add README.md AGENTS.md CONTRIBUTING.md CHANGELOG.md examples/gate.yue
git commit -m "docs: README, AGENTS, CONTRIBUTING, CHANGELOG and the in-game gate"
```

---

### Task 10: Release and Moonwell's records

- [ ] **Step 1: The in-game gate (the maintainer)**

Ask the maintainer to run `deno task gate systems` in `../wrappers-gate` and send the F12 log, one step at a time.
Record the results in CONTRIBUTING and CHANGELOG (`## 0.1.0 (<date>)` with a `### Release gate` section: automated
counts and the gate lines). Fix and re-run anything that fails.

- [ ] **Step 2: GitHub (ask first)**

Ask the maintainer before creating the public repository. Then:

```bash
"/c/Program Files/GitHub CLI/gh.exe" repo create mdlsvensson/moonwell-systems --public --description "Opt-in Warcraft III systems for Moonwell maps, ported from wc3-lib" --source . --push
git tag v0.1.0 && git push origin v0.1.0
"/c/Program Files/GitHub CLI/gh.exe" release create v0.1.0 --prerelease --title v0.1.0 --notes-file <the 0.1.0 section>
```

- [ ] **Step 3: Tag consumption**

A fresh map `../systems-tag-check-010` (`init --link`), with both libraries from GitHub (wrappers `v0.7.0`, systems
`v0.1.0`) and `examples/gate.yue` as `src/main.yue`: check, build, build `--minify`; `moonwell.lock` records both
commits; the fetched `systems` files match the tag's `src/`; after removing `.moonwell/` and checking again, the lock is
unchanged. Record it in CONTRIBUTING and AGENTS; commit and push.

- [ ] **Step 4: Moonwell's records**

In `../moonwell`:
- `AGENTS.md`: a state bullet "moonwell-systems v0.1.0, release 1 of the port, released"; next work becomes the spec for
  release 2 (dummy, buffs, aura, `systems.internal.ordered`); add the backlog item **Replace Deno in Moonwell's
  toolchain** (the maintainer's plan, 2026-09-30: the CLI and the wrappers' tools; moonwell-systems already needs none).
- Roadmap: phase 3 release 1 marked released, with the spec and plan names.
- `CHANGELOG.md` Unreleased: a documentation line for the systems spec and plan.

Commit, push, and check CI with `gh run list`.
