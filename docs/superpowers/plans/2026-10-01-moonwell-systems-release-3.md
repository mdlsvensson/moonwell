# Moonwell Systems Release 3 (v0.3.0) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** moonwell-systems v0.3.0: `systems.damage`, the damage pipeline on `wrappers.damage`.

**Architecture:** One module. `DamageSystem.start()` adds one `Damage.onDamaging` listener, one `Damage.onDamaged`
listener and one wrappers `Timer`. A DAMAGING event creates a `Hit` and runs the `beforeArmor` listeners; the matching
DAMAGED event runs the `afterArmor` listeners and the observers on the same Hit. Hit setters write to the wrappers
event at once and work only on the hit being handled. `deal` queues script damage and drains the queue when no hit is
in flight. Listener lists are copied on change, so a dispatch never allocates.

**Tech Stack:** as releases 1 and 2 (annotated Lua 5.3, `yue -e` tooling, LuaLS 3.19.1, Lua 5.3.6 `luac`,
moonwell-wrappers v0.7.0, Moonwell 0.5.2).

**Spec:** `docs/superpowers/specs/2026-10-01-moonwell-systems-release-3-design.md` (and Part 1 of
`2026-09-30-moonwell-systems-design.md`).

**Verified in advance:** every code block below was run on 2026-10-01 in a scratch copy of the repository: the suites
(12 suites, 90 tests), the syntax check (32 files) and full integration (14 expected negative diagnostics, 8 entry
points, both gate examples) passed, and Task 2's file passed Task 2's 17 tests on its own.

## Global Constraints

- **Repository:** `C:\Users\mdlsvensson\Repo\moonwell-systems`; Moonwell records in `C:\Users\mdlsvensson\Repo\moonwell`;
  the gate map in `C:\Users\mdlsvensson\Repo\wrappers-gate` (not under git). Commit on `main`, explicit paths only,
  each check run as its own command.
- **Checks**, from the repository root:
  - `yue -e tests/run.lua`
  - `MOONWELL_LUAC=../moonwell-wrappers/.tools/lua53/luac53.exe yue -e tools/check.lua`
  - `MOONWELL_LUALS="C:/Users/mdlsvensson/.antigravity-ide/extensions/sumneko.lua-3.19.1-win32-x64/server/bin/lua-language-server.exe" yue -e tools/integration.lua`
- **Write files with the file tools, not shell heredocs:** Git Bash turns `\\` into `\` and `\n` into a newline.
- **Messages:** `[systems] <Class>.<method>: <problem>`. New texts: `expected DamageSystem`, `expected Hit`,
  `expected an options table`, `expected damage options: <field>`, `the system is disposed`,
  `the system is not started`, `expected a finite priority`, `expected a damage request table`,
  `expected a damage request: <field>`, `the queue is full (<n> deals)`, `expected a finite non-negative amount`,
  `expected an attack type`, `expected a damage type`, `expected a weapon type`, `observers cannot change a hit`,
  `types can change only before armor`, `the hit is not being handled`.
- **Callback labels:** `Damage listener`, `Damage sourceOf`, `Damage chain`.
- **Levels and tail calls** as releases 1 and 2: 2 in a public function, 3 (+ depth) in a helper;
  `return (helper(...))`.
- **Private fields** use `---@field package`, never `private`. No field shares a name with a method of its class.
- **Lines** stay within 120 columns.

---

### Task 1: `Callback.report`

**Files:** Modify `src/systems/internal/callback.lua`, `tests/internal.lua`.

**Interfaces:** Produces `Callback.report(label, onError, message)`: sends `message` to `onError`, or prints
`[systems] <label> failed: <message>`. `Callback.call` keeps its behavior and now uses it.

- [ ] **Step 1: Failing test** — in `tests/internal.lua`, before `test('an unprintable error is still reported', …)`:

```lua
test('report sends a message to onError, or prints it', function()
    local messages = {}
    Callback.report('Probe', function(message) messages[#messages + 1] = message end, 'first')
    eq(#messages, 1); eq(messages[1], 'first'); eq(#PRINTED, 0)
    Callback.report('Probe', nil, 'second')
    eq(#PRINTED, 1); eq(PRINTED[1], '[systems] Probe failed: second')
    Callback.report('Probe', function() error('handler broke', 0) end, 'third')
    eq(PRINTED[2], '[systems] Probe error handler failed: handler broke')
    eq(PRINTED[3], '[systems] Probe failed: third')
end)
```

- [ ] **Step 2:** `yue -e tests/run.lua internal` → `internal: 1/7 tests failed` (`attempt to call a nil value (field
  'report')`).

- [ ] **Step 3: Implement** — in `src/systems/internal/callback.lua`, replace `Callback.call` and its comment with:

```lua
---Reports a failure: to `onError(message)`, itself behind the boundary, or printed as
---`[systems] <label> failed: <message>`.
---@param label string
---@param onError (fun(message: string): ...)?
---@param message unknown
function Callback.report(label, onError, message)
    local reported = text(message)
    if onError then
        local handled, failure = pcall(onError, reported)
        if handled then return end
        print('[systems] ' .. label .. ' error handler failed: ' .. text(failure))
    end
    print('[systems] ' .. label .. ' failed: ' .. reported)
end

---Runs `fn(...)` behind the boundary and reports a failure (see report).
---@param label string
---@param onError (fun(message: string): ...)?
---@param fn function
---@param ... any
---@return boolean succeeded
function Callback.call(label, onError, fn, ...)
    local ok, message = pcall(fn, ...)
    if ok then return true end
    Callback.report(label, onError, message)
    return false
end
```

- [ ] **Step 4:** `yue -e tests/run.lua; echo "exit $?"` → `internal: SUITE PASSED: 7 tests`, every other suite as
  before, `exit 0`.

- [ ] **Step 5: Commit** `src/systems/internal/callback.lua tests/internal.lua` — `feat: Callback.report`.

---

### Task 2: The pipeline and the Hit

**Files:** Create `src/systems/damage.lua`, `tests/damage.lua`; modify `tests/suites.lua`.

**Interfaces:**
- Consumes `Callback.call`, `Callback.check`, `Check.receiver`, `Check.finite`; the wrappers' `Damage.onDamaging`,
  `Damage.onDamaged`, `Damage.off` (imported as `Events`), `Timer.create`, `timer:start`, `timer:destroy`.
- Produces `DamageSystem.new(options)`, `:start()`, `:beforeArmor(callback, priority)`,
  `:afterArmor(callback, priority)`, `:observe(callback, priority)` (each returns a remove function),
  `:getCurrent()`, `:dispose()`, and `DamageSystem.Hit` with `setAmount`, `cancel`, `setAttackType`, `setDamageType`,
  `setWeaponType`, `isLethal`. `new` already checks and stores every option; Task 3 uses `sourceOf`, `maxQueue` and
  `maxChain`.

- [ ] **Step 1: Failing tests** — `tests/damage.lua` (the doubles for `UnitDamageTarget` and `RemoveUnit` are used
  from Task 3 on):

```lua
bj_MAX_PLAYER_SLOTS = 28
EVENT_PLAYER_UNIT_DAMAGING, EVENT_PLAYER_UNIT_DAMAGED = {}, {}
ATTACK_TYPE_NORMAL, ATTACK_TYPE_CHAOS = {}, {}
DAMAGE_TYPE_NORMAL, DAMAGE_TYPE_UNIVERSAL = {}, {}
WEAPON_TYPE_WHOKNOWS, WEAPON_TYPE_METAL = {}, {}
local actions, timers, writes, dealt = {}, {}, {}, {}
local raw -- the event the game is delivering
local omitPost, onDeal, rejected = false, nil, false

-- Delivers one event: runs the action of every enabled trigger registered for it. Returns `data`, whose fields the
-- setter doubles change.
local function fire(event, data)
    local saved = raw
    raw = data
    for _, action in ipairs(actions) do
        if action.trigger.enabled and action.trigger.events[1] == event then action.callback() end
    end
    raw = saved
    return data
end

native('Player', function(index) return {index = index} end)
native('CreateTrigger', function() return {events = {}, enabled = true} end)
native('TriggerRegisterPlayerUnitEvent', function(trigger, _, event) trigger.events[#trigger.events + 1] = event end)
native('TriggerAddAction', function(trigger, callback)
    actions[#actions + 1] = {trigger = trigger, callback = callback}
    return {}
end)
native('EnableTrigger', function(trigger) trigger.enabled = true end)
native('DisableTrigger', function(trigger) trigger.enabled = false end)
native('CreateTimer', function()
    local timer = {}
    timers[#timers + 1] = timer
    return timer
end)
native('TimerStart', function(timer, _, _, callback) timer.callback = callback end)
native('PauseTimer', function(timer) timer.callback = nil end)
native('DestroyTimer', function() end)
native('GetEventDamageSource', function() return raw.source end)
native('BlzGetEventDamageTarget', function() return raw.target end)
native('GetEventDamage', function() return raw.amount end)
native('BlzGetEventIsAttack', function() return raw.isAttack end)
native('BlzGetEventAttackType', function() return raw.attackType end)
native('BlzGetEventDamageType', function() return raw.damageType end)
native('BlzGetEventWeaponType', function() return raw.weaponType end)
native('BlzSetEventDamage', function(amount) raw.amount = amount; writes[#writes + 1] = amount end)
native('BlzSetEventAttackType', function(value) raw.attackType = value end)
native('BlzSetEventDamageType', function(value) raw.damageType = value end)
native('BlzSetEventWeaponType', function(value) raw.weaponType = value end)
native('GetWidgetLife', function(handle) return handle.life end)
native('RemoveUnit', function() end)
-- The game's side of a script hit: DAMAGING, then DAMAGED with half the amount (the armor).
native('UnitDamageTarget', function(source, target, amount, _, _, attackType, damageType, weaponType)
    if rejected then return false end
    dealt[#dealt + 1] = target.name
    local event = fire(EVENT_PLAYER_UNIT_DAMAGING, {source = source, target = target, amount = amount,
        isAttack = false, attackType = attackType, damageType = damageType, weaponType = weaponType})
    if onDeal then onDeal() end
    if not omitPost then
        fire(EVENT_PLAYER_UNIT_DAMAGED, {source = source, target = target, amount = event.amount / 2,
            isAttack = false, attackType = event.attackType, damageType = event.damageType,
            weaponType = event.weaponType})
    end
    return true
end)
local DamageSystem = require('systems.damage')
local Unit = require('wrappers.unit')
eq(totalCalls(), 0)

-- Units are named: one raw handle per name.
local handles = {}
local function handle(name)
    handles[name] = handles[name] or {name = name}
    return handles[name]
end
local function unit(name) return Unit.fromHandle(handle(name)) end
local function nameOf(wrapper) return wrapper and wrapper.handle.name or 'nobody' end
local function join(list) return table.concat(list, ',') end

local function data(target, amount, source)
    return {source = handle(source or 's'), target = handle(target), amount = amount, isAttack = false,
        attackType = ATTACK_TYPE_NORMAL, damageType = DAMAGE_TYPE_NORMAL, weaponType = WEAPON_TYPE_WHOKNOWS}
end
-- A native hit's two events, from unit 's' unless `source` names another.
local function pre(target, amount, source)
    return fire(EVENT_PLAYER_UNIT_DAMAGING, data(target, amount or 10, source))
end
local function post(target, amount, source)
    return fire(EVENT_PLAYER_UNIT_DAMAGED, data(target, amount or 5, source))
end
-- The end of the engine turn: runs every started timer.
local function settle()
    for _, timer in ipairs(timers) do
        local callback = timer.callback
        if callback then
            timer.callback = nil
            callback()
        end
    end
end

-- Every test starts with new(): it disposes the previous test's system and resets the doubles.
local live
local function new(options)
    if live then live:dispose() end
    writes, dealt, omitPost, onDeal, rejected = {}, {}, false, nil, false
    live = DamageSystem.new(options)
    return live
end

test('a failed start registers nothing and can be retried', function()
    local system = new()
    local seen, created = 0, 0
    system:observe(function() seen = seen + 1 end)
    native('CreateTrigger', function()
        created = created + 1
        if created == 2 then return nil end
        return {events = {}, enabled = true}
    end)
    failsAt(function() system:start() end, 'DamageSystem.start: [wrappers] Damage.onDamaged: native returned nil')
    eq(callCount('DestroyTimer'), 1); eq(callCount('DisableTrigger'), 1)
    pre('t'); post('t'); eq(seen, 0)
    system:start()
    eq(callCount('CreateTimer'), 2); eq(callCount('EnableTrigger'), 1)
    pre('t'); post('t'); eq(seen, 1)
end)

test('nothing is created before start; start is idempotent; dispose releases and is final', function()
    local system = new()
    resetCalls()
    local seen = 0
    system:observe(function() seen = seen + 1 end)
    eq(totalCalls(), 0)
    pre('t'); post('t'); eq(seen, 0)
    system:start(); system:start()
    eq(callCount('CreateTimer'), 1)
    pre('t'); post('t'); eq(seen, 1)
    system:dispose(); system:dispose()
    eq(callCount('DestroyTimer'), 1)
    pre('t'); post('t'); eq(seen, 1)
    failsAt(function() system:start() end, 'DamageSystem.start: the system is disposed')
    failsAt(function() system:observe(function() end) end, 'DamageSystem.observe: the system is disposed')
    eq(#PRINTED, 0)
end)

test('phases run in priority order and the hit records each amount', function()
    local system = new()
    local order, observed = {}, nil
    system:beforeArmor(function(hit) order[#order + 1] = 'late:' .. hit.phase end, 10)
    system:beforeArmor(function(hit) order[#order + 1] = 'early:' .. hit.amount end, -1)
    system:beforeArmor(function() order[#order + 1] = 'default' end)
    system:afterArmor(function(hit) order[#order + 1] = 'after:' .. hit.phase .. ':' .. hit.amount end)
    system:observe(function(hit) order[#order + 1] = 'observe:' .. hit.phase; observed = hit end)
    system:start()
    pre('t', 10); eq(system:getCurrent(), nil)
    post('t', 4)
    eq(join(order), 'early:10,default,late:beforeArmor,after:afterArmor:4,observe:observe')
    eq(nameOf(observed.source), 's'); eq(nameOf(observed.dealer), 's'); eq(nameOf(observed.target), 't')
    eq(observed.initialAmount, 10); eq(observed.beforeArmorAmount, 10); eq(observed.armorAmount, 4)
    eq(observed.amount, 4); eq(observed.isAttack, false); eq(observed.metadata, nil)
    eq(observed.cancelled, false); eq(observed.paired, true)
    eq(observed.attackType, ATTACK_TYPE_NORMAL); eq(observed.damageType, DAMAGE_TYPE_NORMAL)
    eq(observed.weaponType, WEAPON_TYPE_WHOKNOWS)
    eq(system:getCurrent(), nil); eq(#PRINTED, 0)
end)

test('a removed listener stops at once and an added one waits for the next hit', function()
    local system = new()
    local seen, remove = {}, nil
    system:beforeArmor(function()
        seen[#seen + 1] = 'first'
        remove()
        system:beforeArmor(function() seen[#seen + 1] = 'added' end)
    end)
    remove = system:beforeArmor(function() seen[#seen + 1] = 'removed' end)
    system:start()
    pre('a'); post('a'); pre('b')
    eq(join(seen), 'first,first,added'); eq(#PRINTED, 0)
end)

test('an observer added before armor waits for the next hit', function()
    local system = new()
    local seen = {}
    system:beforeArmor(function()
        system:observe(function(hit) seen[#seen + 1] = nameOf(hit.target) end)
    end)
    system:start()
    pre('first'); post('first'); pre('second'); post('second')
    eq(join(seen), 'second')
end)

test('a nested native hit restores the current hit', function()
    local system = new()
    local seen = {}
    system:beforeArmor(function(hit)
        if nameOf(hit.target) == 'outer' then
            pre('inner'); post('inner')
            eq(system:getCurrent(), hit)
            seen[#seen + 1] = 'outer resumed'
        else
            seen[#seen + 1] = 'inner current ' .. tostring(system:getCurrent() == hit)
        end
    end)
    system:observe(function(hit) seen[#seen + 1] = 'observed ' .. nameOf(hit.target) end)
    system:start()
    pre('outer'); post('outer')
    eq(join(seen), 'inner current true,observed inner,outer resumed,observed outer')
    eq(system:getCurrent(), nil); eq(#PRINTED, 0)
end)

test('a missing DAMAGED expires at the settle without taking another hit', function()
    local system = new()
    local seen, last = {}, nil
    system:observe(function(hit)
        seen[#seen + 1] = nameOf(hit.target) .. ':' .. tostring(hit.paired)
        last = hit
    end)
    system:start()
    pre('outer'); pre('missing'); post('outer')
    settle()
    post('missing', 3)
    eq(join(seen), 'outer:true,missing:false')
    eq(last.initialAmount, 3); eq(last.beforeArmorAmount, 3); eq(last.armorAmount, 3); eq(last.amount, 3)
    eq(#PRINTED, 0)
end)

test('the pending limit drops the oldest hit silently', function()
    local system = new({maxPending = 2})
    local seen = {}
    system:observe(function(hit) seen[#seen + 1] = nameOf(hit.target) .. ':' .. tostring(hit.paired) end)
    system:start()
    pre('a'); pre('b'); pre('c')
    post('c'); post('b'); post('a')
    eq(join(seen), 'c:true,b:true,a:false'); eq(#PRINTED, 0)
end)

test('a hit with no source runs the pipeline; one with no target is ignored', function()
    local system = new()
    local seen, count = nil, 0
    system:beforeArmor(function(hit) count = count + 1; hit:setAmount(1) end)
    system:observe(function(hit) seen = hit end)
    system:start()
    local event = {target = handle('t'), amount = 10, isAttack = false, attackType = ATTACK_TYPE_NORMAL,
        damageType = DAMAGE_TYPE_NORMAL, weaponType = WEAPON_TYPE_WHOKNOWS}
    fire(EVENT_PLAYER_UNIT_DAMAGING, event)
    eq(event.amount, 1)
    event.amount = 0.5
    fire(EVENT_PLAYER_UNIT_DAMAGED, event)
    eq(seen.source, nil); eq(seen.dealer, nil); eq(seen.paired, true); eq(seen.amount, 0.5)
    fire(EVENT_PLAYER_UNIT_DAMAGING, {source = handle('s'), amount = 10})
    fire(EVENT_PLAYER_UNIT_DAMAGED, {source = handle('s'), amount = 10})
    eq(count, 1); eq(#PRINTED, 0)
end)

test('a failing listener is reported and the others still run', function()
    local messages = {}
    local system = new({onError = function(message) messages[#messages + 1] = message end})
    local seen = {}
    system:beforeArmor(function(hit)
        if nameOf(hit.target) == 'bad' then error('bad listener', 0) end
    end)
    system:beforeArmor(function(hit) seen[#seen + 1] = nameOf(hit.target) end)
    system:start()
    pre('bad'); post('bad'); pre('good'); post('good')
    eq(join(seen), 'bad,good'); eq(#messages, 1); eq(messages[1], 'bad listener')
    eq(system:getCurrent(), nil); eq(#PRINTED, 0)
    system = new()
    system:observe(function() error('printed listener', 0) end)
    system:start()
    pre('t'); post('t')
    eq(#PRINTED, 1); eq(PRINTED[1], '[systems] Damage listener failed: printed listener')
end)

test('dispose inside a listener stops the remaining listeners of the hit', function()
    local system = new()
    local called = false
    system:beforeArmor(function() system:dispose() end)
    system:beforeArmor(function() called = true end)
    system:afterArmor(function() called = true end)
    system:start()
    resetCalls()
    pre('t'); post('t')
    eq(called, false); eq(callCount('DestroyTimer'), 1); eq(system:getCurrent(), nil); eq(#PRINTED, 0)
end)

test('setAmount and cancel change the hit and the game event', function()
    local system = new()
    local amounts = {}
    system:beforeArmor(function(hit)
        hit:setAmount(hit.amount * 2)
        if nameOf(hit.target) == 'cancel' then
            hit:cancel(); hit:cancel()
            hit:setAmount(50)
        end
    end)
    system:afterArmor(function(hit)
        if nameOf(hit.target) == 'cancel' then hit:setAmount(99) else hit:setAmount(math.max(hit.amount - 1, 0)) end
    end)
    system:observe(function(hit)
        amounts[#amounts + 1] = table.concat({hit.initialAmount, hit.beforeArmorAmount, hit.armorAmount, hit.amount,
            tostring(hit.cancelled)}, '/')
    end)
    system:start()
    eq(pre('t', 10).amount, 20); eq(post('t', 8).amount, 7)
    eq(pre('cancel', 10).amount, 0)
    eq(post('cancel', 3).amount, 0) -- the game still reported 3: the system zeroes a cancelled hit again
    pre('zero', 0); post('zero', 0)
    eq(join(amounts), '10/20/8/7/false,10/0/3/0/true,0/0/0/0/false')
    eq(join(writes), '20,7,20,0,0,0,0'); eq(#PRINTED, 0)
end)

test('type setters reach the game before armor; after armor the hit shows the game\'s types', function()
    local system = new()
    local seen
    system:beforeArmor(function(hit)
        hit:setAttackType(ATTACK_TYPE_CHAOS)
        hit:setDamageType(DAMAGE_TYPE_UNIVERSAL)
        hit:setWeaponType(WEAPON_TYPE_METAL)
        seen = hit
    end)
    system:start()
    local event = pre('t')
    eq(event.attackType, ATTACK_TYPE_CHAOS); eq(event.damageType, DAMAGE_TYPE_UNIVERSAL)
    eq(event.weaponType, WEAPON_TYPE_METAL)
    eq(seen.attackType, ATTACK_TYPE_CHAOS); eq(seen.damageType, DAMAGE_TYPE_UNIVERSAL)
    eq(seen.weaponType, WEAPON_TYPE_METAL)
    post('t')
    eq(seen.attackType, ATTACK_TYPE_NORMAL); eq(#PRINTED, 0)
end)

test('setters work only in their phase and only on the hit being handled', function()
    local system = new()
    local outer, kept
    system:beforeArmor(function(hit)
        if nameOf(hit.target) == 'outer' then
            outer = hit
            pre('inner'); post('inner')
            hit:setAmount(7)
        else
            failsAt(function() outer:setAmount(1) end, 'Hit.setAmount: the hit is not being handled')
            failsAt(function() outer:cancel() end, 'Hit.cancel: the hit is not being handled')
        end
    end)
    system:afterArmor(function(hit)
        failsAt(function() hit:setAttackType(ATTACK_TYPE_CHAOS) end,
            'Hit.setAttackType: types can change only before armor')
        failsAt(function() hit:setDamageType(DAMAGE_TYPE_UNIVERSAL) end,
            'Hit.setDamageType: types can change only before armor')
        failsAt(function() hit:setWeaponType(WEAPON_TYPE_METAL) end,
            'Hit.setWeaponType: types can change only before armor')
    end)
    system:observe(function(hit)
        kept = hit
        failsAt(function() hit:setAmount(1) end, 'Hit.setAmount: observers cannot change a hit')
        failsAt(function() hit:cancel() end, 'Hit.cancel: observers cannot change a hit')
        failsAt(function() hit:setDamageType(DAMAGE_TYPE_UNIVERSAL) end,
            'Hit.setDamageType: observers cannot change a hit')
    end)
    system:start()
    eq(pre('outer').amount, 7)
    failsAt(function() outer:setAmount(1) end, 'Hit.setAmount: the hit is not being handled')
    post('outer')
    failsAt(function() kept:setAmount(1) end, 'Hit.setAmount: the hit is not being handled')
    eq(join(writes), '7'); eq(#PRINTED, 0)
end)

test('setters and isLethal check their arguments at the caller', function()
    local system = new()
    local ran = false
    system:beforeArmor(function(hit)
        for _, bad in ipairs({'1', -1, 0 / 0, math.huge}) do
            failsAt(function() hit:setAmount(bad) end, 'Hit.setAmount: expected a finite non-negative amount')
        end
        failsAt(function() hit:setAttackType(nil) end, 'Hit.setAttackType: expected an attack type')
        failsAt(function() hit:setDamageType(nil) end, 'Hit.setDamageType: expected a damage type')
        failsAt(function() hit:setWeaponType(nil) end, 'Hit.setWeaponType: expected a weapon type')
        failsAt(function() hit.setAmount({}, 1) end, 'Hit.setAmount: expected Hit')
        failsAt(function() hit.isLethal({}) end, 'Hit.isLethal: expected Hit')
        ran = true
    end)
    system:start()
    pre('t')
    eq(ran, true); eq(#writes, 0); eq(#PRINTED, 0)
end)

test('isLethal compares the amount with the target\'s life', function()
    local system = new()
    local answers = {}
    system:afterArmor(function(hit)
        answers[#answers + 1] = tostring(hit:isLethal())
        if nameOf(hit.target) == 'gone' then
            hit.target:remove()
            answers[#answers + 1] = tostring(hit:isLethal())
        end
    end)
    system:start()
    handle('t').life = 10
    pre('t'); post('t', 9)
    pre('t'); post('t', 9.6)
    handle('gone').life = 1
    pre('gone'); post('gone', 5)
    eq(join(answers), 'false,true,true,false'); eq(#PRINTED, 0)
end)

test('new and the listener functions check their arguments at the caller', function()
    failsAt(function() DamageSystem.new(5) end, 'DamageSystem.new: expected an options table')
    local cases = {{'sourceOf', 5}, {'onError', 'x'}, {'maxQueue', 0}, {'maxChain', 1.5}, {'maxPending', '2'}}
    for _, case in ipairs(cases) do
        failsAt(function() DamageSystem.new({[case[1]] = case[2]}) end,
            'DamageSystem.new: expected damage options: ' .. case[1])
    end
    local system = new({maxQueue = 1, maxChain = 1, maxPending = 1})
    failsAt(function() system:beforeArmor(5) end, 'DamageSystem.beforeArmor: expected a callback function')
    failsAt(function() system:afterArmor(nil) end, 'DamageSystem.afterArmor: expected a callback function')
    failsAt(function() system:observe(function() end, 'high') end, 'DamageSystem.observe: expected a finite priority')
    failsAt(function() system:beforeArmor(function() end, 0 / 0) end,
        'DamageSystem.beforeArmor: expected a finite priority')
    failsAt(function() DamageSystem.getCurrent({}) end, 'DamageSystem.getCurrent: expected DamageSystem')
    failsAt(function() DamageSystem.dispose({}) end, 'DamageSystem.dispose: expected DamageSystem')
end)
```

`tests/suites.lua` becomes:

```lua
-- Every behavior suite, in run order. tools/check.lua fails when a suite file is missing here.
return {'internal', 'ordered', 'scheduler', 'signal', 'scope', 'time', 'buffs', 'aura', 'dummy', 'damage', 'imports',
    'blame'}
```

- [ ] **Step 2:** `yue -e tests/run.lua damage` → `damage: ERROR …module 'systems.damage' not found`.

- [ ] **Step 3: Implement** — `src/systems/damage.lua`:

```lua
local Callback = require('systems.internal.callback')
local Check = require('systems.internal.check')
local Events = require('wrappers.damage')
local Timer = require('wrappers.timer')

---A damage pipeline on wrappers.damage: listeners before armor, after armor and once the final amount is known.
---Nothing is created before start().
---@class MoonwellSystems.DamageSystem
---@field package before MoonwellSystems.DamageListener[] Replaced, never changed in place: a dispatch keeps its list.
---@field package after MoonwellSystems.DamageListener[]
---@field package observers MoonwellSystems.DamageListener[]
---@field package pending MoonwellSystems.Hit[] Hits whose DAMAGED has not come, oldest first.
---@field package sourceOf (fun(dealer: MoonwellWrappers.Unit): MoonwellWrappers.Unit?)?
---@field package onError (fun(message: string): ...)?
---@field package maxQueue integer
---@field package maxChain integer
---@field package maxPending integer
---@field package nextListener integer The id of the newest listener; a hit runs the listeners up to its cutoff.
---@field package depth integer Hits whose listeners are running.
---@field package running boolean
---@field package disposed boolean
---@field package current MoonwellSystems.Hit?
---@field package timer MoonwellWrappers.Timer? The settle timer.
---@field package scheduled boolean Whether the settle timer is running.
---@field package onSettle fun(): ...
---@field package damagingToken MoonwellWrappers.DamageListener?
---@field package damagedToken MoonwellWrappers.DamageListener?
local DamageSystem = {}
DamageSystem.__index = DamageSystem

---@class MoonwellSystems.DamageOptions
---@field sourceOf (fun(dealer: MoonwellWrappers.Unit): MoonwellWrappers.Unit?)? Credits a hit to another Unit.
---@field onError (fun(message: string): ...)? Receives failures; default prints them.
---@field maxQueue integer? Most deals that may wait. Default 128.
---@field maxChain integer? Most deals in one chain. Default 64.
---@field maxPending integer? Most hits awaiting their DAMAGED event. Default 64.

---@class MoonwellSystems.DamageListener
---@field id integer
---@field priority number
---@field callback function? Nil once removed.

---One hit, passed to every listener of every phase. The fields are for reading: change the hit with its methods.
---@class MoonwellSystems.Hit
---@field source MoonwellWrappers.Unit? The credited Unit; nil when the game gives no source.
---@field dealer MoonwellWrappers.Unit? The Unit the game reported.
---@field target MoonwellWrappers.Unit
---@field amount number The current amount.
---@field isAttack boolean The game's value: false for script damage.
---@field attackType attacktype
---@field damageType damagetype
---@field weaponType weapontype
---@field metadata any The deal request's metadata; nil for every other hit.
---@field phase 'beforeArmor'|'afterArmor'|'observe'
---@field initialAmount number The amount the game first reported.
---@field beforeArmorAmount number The amount after the beforeArmor listeners.
---@field armorAmount number? The amount DAMAGED reported; nil before that.
---@field cancelled boolean
---@field paired boolean False when DAMAGED came without a known DAMAGING.
---@field package system MoonwellSystems.DamageSystem
---@field package cutoff integer
---@field package event (MoonwellWrappers.DamagingEvent|MoonwellWrappers.DamagedEvent)? Set while modifiers run.
local Hit = {}
Hit.__index = Hit
DamageSystem.Hit = Hit

local DEFAULTS = {maxQueue = 128, maxChain = 64, maxPending = 64}
local LIMITS = {'maxQueue', 'maxChain', 'maxPending'}
local LISTS = {'before', 'after', 'observers'}

-- Hit

---The wrappers event to write to. Warcraft's setters act on the innermost event, so only the hit being handled may
---change, and only while its modifier listeners run.
---@param hit MoonwellSystems.Hit
---@param operation string
---@param types boolean? Whether the setter changes a type.
---@return MoonwellWrappers.DamagingEvent|MoonwellWrappers.DamagedEvent
local function writable(hit, operation, types)
    if getmetatable(hit) ~= Hit then error('[systems] ' .. operation .. ': expected Hit', 3) end
    if hit.system.current == hit then
        if hit.phase == 'observe' then error('[systems] ' .. operation .. ': observers cannot change a hit', 3) end
        if types and hit.phase ~= 'beforeArmor' then
            error('[systems] ' .. operation .. ': types can change only before armor', 3)
        end
        if hit.event then return hit.event end
    end
    error('[systems] ' .. operation .. ': the hit is not being handled', 3)
end

---Sets the amount. Does nothing on a cancelled hit.
---@param amount number Finite and not negative.
function Hit:setAmount(amount)
    local event = writable(self, 'Hit.setAmount')
    if not Check.finite(amount) or amount < 0 then
        error('[systems] Hit.setAmount: expected a finite non-negative amount', 2)
    end
    if self.cancelled then return end
    event:setAmount(amount)
    self.amount = amount
end

---Cancels the hit: the amount becomes 0 and stays 0 in the later phases.
function Hit:cancel()
    local event = writable(self, 'Hit.cancel')
    if self.cancelled then return end
    self.cancelled = true
    event:setAmount(0)
    self.amount = 0
end

---Sets the attack type, which picks the armor table. Before armor only.
---@param attackType attacktype
function Hit:setAttackType(attackType)
    local event = writable(self, 'Hit.setAttackType', true) --[[@as MoonwellWrappers.DamagingEvent]]
    if attackType == nil then error('[systems] Hit.setAttackType: expected an attack type', 2) end
    event:setAttackType(attackType)
    self.attackType = attackType
end

---Sets the damage type, which decides immunity and spell reduction. Before armor only.
---@param damageType damagetype
function Hit:setDamageType(damageType)
    local event = writable(self, 'Hit.setDamageType', true) --[[@as MoonwellWrappers.DamagingEvent]]
    if damageType == nil then error('[systems] Hit.setDamageType: expected a damage type', 2) end
    event:setDamageType(damageType)
    self.damageType = damageType
end

---Sets the weapon type, which decides the impact sound. Before armor only.
---@param weaponType weapontype
function Hit:setWeaponType(weaponType)
    local event = writable(self, 'Hit.setWeaponType', true) --[[@as MoonwellWrappers.DamagingEvent]]
    if weaponType == nil then error('[systems] Hit.setWeaponType: expected a weapon type', 2) end
    event:setWeaponType(weaponType)
    self.weaponType = weaponType
end

---Whether the current amount would kill the target (0.405 is Warcraft's death threshold). A heuristic for afterArmor:
---later listeners, mana shield and native effects can still change the outcome. False for a disposed target wrapper.
---@return boolean
function Hit:isLethal()
    if getmetatable(self) ~= Hit then error('[systems] Hit.isLethal: expected Hit', 2) end
    local target = self.target
    if target:isDisposed() then return false end
    return target:getLife() - self.amount <= 0.405
end

-- The pipeline

---@param value unknown
---@return boolean
local function whole(value) return math.type(value) ~= nil and math.floor(value) == value end

---Runs the hit's listeners of one list: those that existed when the hit began and are still registered (dispose()
---and a remove function clear the callback).
---@param system MoonwellSystems.DamageSystem
---@param list MoonwellSystems.DamageListener[]
---@param hit MoonwellSystems.Hit
local function dispatch(system, list, hit)
    local cutoff = hit.cutoff
    for index = 1, #list do
        local listener = list[index]
        local callback = listener.callback
        if callback and listener.id <= cutoff then
            Callback.call('Damage listener', system.onError, callback, hit)
        end
    end
end

---Starts the settle timer, once per engine turn: when it fires, hits still pending never got their DAMAGED event.
---@param system MoonwellSystems.DamageSystem
local function arm(system)
    if system.scheduled then return end
    system.scheduled = true
    system.timer:start(0, false, system.onSettle)
end

---@param system MoonwellSystems.DamageSystem
---@param event MoonwellWrappers.DamagingEvent
local function damaging(system, event)
    if not system.running then return end
    local target = event.target
    if not target then return end
    arm(system)
    local dealer, amount = event.source, event.amount
    ---@type MoonwellSystems.Hit
    local hit = setmetatable({
        source = dealer, dealer = dealer, target = target, amount = amount,
        isAttack = event.isAttack, attackType = event.attackType, damageType = event.damageType,
        weaponType = event.weaponType, phase = 'beforeArmor', initialAmount = amount,
        beforeArmorAmount = amount, cancelled = false, paired = true, system = system, cutoff = system.nextListener,
    }, Hit)
    local pending = system.pending
    if #pending >= system.maxPending then table.remove(pending, 1) end
    pending[#pending + 1] = hit
    local previous = system.current
    system.current = hit
    system.depth = system.depth + 1
    hit.event = event
    dispatch(system, system.before, hit)
    hit.event = nil
    system.depth = system.depth - 1
    system.current = previous
    hit.beforeArmorAmount = hit.amount
end

---@param system MoonwellSystems.DamageSystem
---@param event MoonwellWrappers.DamagedEvent
local function damaged(system, event)
    if not system.running then return end
    local target = event.target
    if not target then return end
    arm(system)
    local dealer, isAttack, amount = event.source, event.isAttack, event.amount
    local pending = system.pending
    ---@type MoonwellSystems.Hit?
    local hit
    -- The newest matching hit: native hits nest, and unrelated hits without a DAMAGED stay until the settle.
    for index = #pending, 1, -1 do
        local candidate = pending[index]
        if candidate.dealer == dealer and candidate.target == target and candidate.isAttack == isAttack then
            hit = candidate
            table.remove(pending, index)
            break
        end
    end
    if not hit then
        hit = setmetatable({
            source = dealer, dealer = dealer, target = target, isAttack = isAttack,
            initialAmount = amount, beforeArmorAmount = amount, cancelled = false, paired = false, system = system,
            cutoff = system.nextListener,
        }, Hit)
    end
    hit.phase = 'afterArmor'
    hit.armorAmount = amount
    hit.attackType, hit.damageType, hit.weaponType = event.attackType, event.damageType, event.weaponType
    if hit.cancelled then
        if amount ~= 0 then event:setAmount(0) end
        amount = 0
    end
    hit.amount = amount
    local previous = system.current
    system.current = hit
    system.depth = system.depth + 1
    hit.event = event
    dispatch(system, system.after, hit)
    hit.event = nil
    hit.phase = 'observe'
    dispatch(system, system.observers, hit)
    system.depth = system.depth - 1
    system.current = previous
end

---@param system MoonwellSystems.DamageSystem
local function settle(system)
    system.scheduled = false
    if not system.running then return end
    if #system.pending > 0 then system.pending = {} end
end

---Removes what start() added.
---@param system MoonwellSystems.DamageSystem
local function release(system)
    if system.damagingToken then Events.off(system.damagingToken); system.damagingToken = nil end
    if system.damagedToken then Events.off(system.damagedToken); system.damagedToken = nil end
    if system.timer then system.timer:destroy(); system.timer = nil end
    system.scheduled = false
end

-- DamageSystem

---Creates a stopped system: add listeners, then call start().
---@param options MoonwellSystems.DamageOptions?
---@return MoonwellSystems.DamageSystem
function DamageSystem.new(options)
    if options == nil then options = {} end
    if type(options) ~= 'table' then error('[systems] DamageSystem.new: expected an options table', 2) end
    for _, name in ipairs({'sourceOf', 'onError'}) do
        if options[name] ~= nil and type(options[name]) ~= 'function' then
            error('[systems] DamageSystem.new: expected damage options: ' .. name, 2)
        end
    end
    local limits = {}
    for _, name in ipairs(LIMITS) do
        local value = options[name]
        if value == nil then value = DEFAULTS[name] end
        if not whole(value) or value < 1 then
            error('[systems] DamageSystem.new: expected damage options: ' .. name, 2)
        end
        limits[name] = value
    end
    ---@type MoonwellSystems.DamageSystem
    local system = setmetatable({
        before = {}, after = {}, observers = {}, pending = {}, sourceOf = options.sourceOf,
        onError = options.onError, maxQueue = limits.maxQueue, maxChain = limits.maxChain,
        maxPending = limits.maxPending, nextListener = 0, depth = 0, running = false, disposed = false,
        scheduled = false,
    }, DamageSystem)
    system.onSettle = function() settle(system) end
    return system
end

---Registers for Warcraft's damage events. Idempotent while running; raises after dispose().
function DamageSystem:start()
    local system = Check.receiver(self, DamageSystem, 'DamageSystem', 'DamageSystem.start')
    if system.disposed then error('[systems] DamageSystem.start: the system is disposed', 2) end
    if system.running then return end
    local ok, failure = pcall(function()
        system.timer = Timer.create()
        system.damagingToken = Events.onDamaging(function(event) damaging(system, event) end)
        system.damagedToken = Events.onDamaged(function(event) damaged(system, event) end)
    end)
    if not ok then
        release(system)
        -- The wrappers' message without its position, raised at this function's caller.
        local reason = tostring(failure):gsub('^.-:%d+: ', '')
        error('[systems] DamageSystem.start: ' .. reason, 2)
    end
    system.running = true
end

---@param system MoonwellSystems.DamageSystem
---@param key 'before'|'after'|'observers'
---@param callback fun(hit: MoonwellSystems.Hit): ...
---@param priority number?
---@param operation string
---@return fun() remove
local function listen(system, key, callback, priority, operation)
    if system.disposed then error('[systems] ' .. operation .. ': the system is disposed', 3) end
    Callback.check(callback, operation, 1)
    if priority == nil then priority = 0 end
    if not Check.finite(priority) then error('[systems] ' .. operation .. ': expected a finite priority', 3) end
    system.nextListener = system.nextListener + 1
    ---@type MoonwellSystems.DamageListener
    local listener = {id = system.nextListener, priority = priority, callback = callback}
    local list, copy, placed = system[key], {}, false
    for index = 1, #list do
        if not placed and list[index].priority > priority then
            copy[#copy + 1] = listener
            placed = true
        end
        copy[#copy + 1] = list[index]
    end
    if not placed then copy[#copy + 1] = listener end
    system[key] = copy
    return function()
        if not listener.callback then return end
        listener.callback = nil
        local kept = {}
        for _, other in ipairs(system[key]) do
            if other ~= listener then kept[#kept + 1] = other end
        end
        system[key] = kept
    end
end

---Adds a modifier that runs before armor. Lower `priority` runs first; equal priorities keep registration order. A
---listener added while a hit is in flight waits for the next hit.
---@param callback fun(hit: MoonwellSystems.Hit): ...
---@param priority number? Default 0.
---@return fun() remove Idempotent.
function DamageSystem:beforeArmor(callback, priority)
    local system = Check.receiver(self, DamageSystem, 'DamageSystem', 'DamageSystem.beforeArmor')
    return (listen(system, 'before', callback, priority, 'DamageSystem.beforeArmor'))
end

---Adds a modifier that runs after armor, the last point where the amount can change.
---@param callback fun(hit: MoonwellSystems.Hit): ...
---@param priority number? Default 0.
---@return fun() remove Idempotent.
function DamageSystem:afterArmor(callback, priority)
    local system = Check.receiver(self, DamageSystem, 'DamageSystem', 'DamageSystem.afterArmor')
    return (listen(system, 'after', callback, priority, 'DamageSystem.afterArmor'))
end

---Adds an observer that runs once the hit's final amount is known. Observers cannot change the hit.
---@param callback fun(hit: MoonwellSystems.Hit): ...
---@param priority number? Default 0.
---@return fun() remove Idempotent.
function DamageSystem:observe(callback, priority)
    local system = Check.receiver(self, DamageSystem, 'DamageSystem', 'DamageSystem.observe')
    return (listen(system, 'observers', callback, priority, 'DamageSystem.observe'))
end

---The hit whose listeners are running (the innermost one), or nil outside damage events.
---@return MoonwellSystems.Hit?
function DamageSystem:getCurrent()
    return Check.receiver(self, DamageSystem, 'DamageSystem', 'DamageSystem.getCurrent').current
end

---Stops listening and drops every listener and pending hit. Inside a listener, the hit's remaining listeners do
---not run. Idempotent.
function DamageSystem:dispose()
    local system = Check.receiver(self, DamageSystem, 'DamageSystem', 'DamageSystem.dispose')
    if system.disposed then return end
    system.disposed = true
    system.running = false
    system.pending = {}
    for _, key in ipairs(LISTS) do
        for _, listener in ipairs(system[key]) do listener.callback = nil end
        system[key] = {}
    end
    release(system)
end

return DamageSystem
```

- [ ] **Step 4:** `yue -e tests/run.lua; echo "exit $?"` → `damage: SUITE PASSED: 17 tests`, `All 12 suites passed`,
  `exit 0`.

- [ ] **Step 5: Commit** `src/systems/damage.lua tests/damage.lua tests/suites.lua` —
  `feat: systems.damage pipeline and Hit`.

---

### Task 3: Script damage and attribution

**Files:** Modify `src/systems/damage.lua`, `tests/damage.lua`.

**Interfaces:**
- Consumes Task 2's module, `Callback.report`, and the wrappers' `unit:damageTarget`, `unit:isDisposed`.
- Produces `DamageSystem:deal(request)` (returns nothing), the `sourceOf` resolver (`hit.source` against
  `hit.dealer`), `hit.metadata`, and the `maxQueue` and `maxChain` limits.

- [ ] **Step 1: Failing tests** — append to `tests/damage.lua`:

```lua
-- Script damage and attribution.

local function deal(system, target, amount, metadata, source)
    system:deal({source = unit(source or 's'), target = unit(target), amount = amount, metadata = metadata})
end

test('deal needs a started system, runs at once and carries its metadata', function()
    local system = new()
    local seen = {}
    system:beforeArmor(function(hit) hit:setAmount(hit.amount + 2) end)
    system:observe(function(hit)
        seen[#seen + 1] = tostring(hit.metadata) .. ':' .. hit.amount .. ':' .. tostring(hit.isAttack)
    end)
    failsAt(function() system:deal({source = unit('s'), target = unit('t'), amount = 10}) end,
        'DamageSystem.deal: the system is not started')
    system:start()
    resetCalls()
    deal(system, 't', 10, 'spell')
    eq(join(seen), 'spell:6.0:false')
    expectCall('UnitDamageTarget', handle('s'), handle('t'), 10, false, false, ATTACK_TYPE_NORMAL, DAMAGE_TYPE_NORMAL,
        WEAPON_TYPE_WHOKNOWS)
    system:deal({source = unit('s'), target = unit('t'), amount = 0, attack = true, ranged = true,
        attackType = ATTACK_TYPE_CHAOS, damageType = DAMAGE_TYPE_UNIVERSAL, weaponType = WEAPON_TYPE_METAL})
    expectCall('UnitDamageTarget', handle('s'), handle('t'), 0, true, true, ATTACK_TYPE_CHAOS, DAMAGE_TYPE_UNIVERSAL,
        WEAPON_TYPE_METAL)
    eq(system:getCurrent(), nil); eq(#PRINTED, 0)
end)

test('a nested native hit never inherits metadata, and the same two units claim it once', function()
    local system = new()
    local seen, nested = {}, false
    system:beforeArmor(function(hit)
        seen[#seen + 1] = 'pre ' .. nameOf(hit.target) .. ' ' .. tostring(hit.metadata)
        if nameOf(hit.target) == 'outer' and not nested then
            nested = true
            pre('inner'); post('inner')
            pre('outer'); post('outer')
        end
    end)
    system:observe(function(hit) seen[#seen + 1] = 'post ' .. nameOf(hit.target) .. ' ' .. tostring(hit.metadata) end)
    system:start()
    deal(system, 'outer', 10, 'spell')
    pre('attack'); post('attack')
    eq(join(seen), 'pre outer spell,pre inner nil,post inner nil,pre outer nil,post outer nil,post outer spell,'
        .. 'pre attack nil,post attack nil')
    eq(#PRINTED, 0)
end)

test('deals from listeners run first in first out after the hit, and the chain limit recovers', function()
    local messages = {}
    local system = new({maxChain = 3, onError = function(message) messages[#messages + 1] = message end})
    local order = {}
    system:beforeArmor(function(hit)
        local target = nameOf(hit.target)
        order[#order + 1] = 'pre:' .. target
        deal(system, target .. '+', 1)
        if target == 'a' then deal(system, 'b', 1) end
    end)
    system:observe(function(hit) order[#order + 1] = 'post:' .. nameOf(hit.target) end)
    system:start()
    deal(system, 'a', 10)
    eq(join(dealt), 'a,a+,b')
    eq(join(order), 'pre:a,post:a,pre:a+,post:a+,pre:b,post:b')
    eq(#messages, 1); eq(messages[1], 'more than 3 deals in one chain; 2 queued deals dropped')
    eq(system:getCurrent(), nil)
    deal(system, 'c', 1)
    eq(join(dealt), 'a,a+,b,c,c+,c++')
    eq(#messages, 2); eq(messages[2], 'more than 3 deals in one chain; 1 queued deals dropped')
    eq(#PRINTED, 0)
end)

test('a deal from an afterArmor listener waits until the observers have run', function()
    local system = new()
    local order = {}
    system:afterArmor(function(hit)
        if nameOf(hit.target) == 'a' then deal(system, 'b', 1) end
    end)
    system:observe(function(hit) order[#order + 1] = nameOf(hit.target) end)
    system:start()
    pre('a'); post('a')
    eq(join(order), 'a,b'); eq(system:getCurrent(), nil); eq(#PRINTED, 0)
end)

test('separate deals do not share a chain', function()
    local system = new({maxChain = 2})
    system:start()
    for _ = 1, 5 do deal(system, 't', 1) end
    eq(#dealt, 5); eq(#PRINTED, 0)
end)

test('a full queue raises at the listener and keeps the accepted deals', function()
    local messages = {}
    local system = new({maxQueue = 2, onError = function(message) messages[#messages + 1] = message end})
    system:beforeArmor(function(hit)
        if nameOf(hit.target) == 'a' then
            for _, target in ipairs({'b', 'c', 'd'}) do deal(system, target, 1) end
        end
    end)
    system:start()
    deal(system, 'a', 1)
    eq(join(dealt), 'a,b,c'); eq(#messages, 1)
    assert(messages[1]:find('DamageSystem.deal: the queue is full (2 deals)', 1, true), messages[1])
    assert(messages[1]:find('^tests/damage%.lua:%d+: '), messages[1])
end)

test('the chain budget survives a settle', function()
    local messages = {}
    local system = new({maxChain = 2, onError = function(message) messages[#messages + 1] = message end})
    system:beforeArmor(function(hit)
        if nameOf(hit.target) == 'spell' then deal(system, 'spell', 1) end
    end)
    onDeal = function() pre('missing') end
    system:start()
    deal(system, 'spell', 1)
    eq(join(dealt), 'spell')
    settle(); settle(); settle()
    eq(join(dealt), 'spell,spell')
    eq(#messages, 1); eq(messages[1], 'more than 2 deals in one chain; 1 queued deals dropped')
end)

test('a deal with no DAMAGED releases its metadata before the next hit', function()
    local system = new()
    local seen = {}
    system:beforeArmor(function(hit) seen[#seen + 1] = tostring(hit.metadata) end)
    system:observe(function(hit)
        seen[#seen + 1] = 'observed ' .. tostring(hit.metadata) .. ' ' .. tostring(hit.paired)
    end)
    system:start()
    omitPost = true
    deal(system, 'same', 0, 'cancelled-spell')
    post('same') -- must not pair with the deal's hit
    pre('same'); post('same')
    eq(join(seen), 'cancelled-spell,observed nil false,nil,observed nil true'); eq(#PRINTED, 0)
end)

test('a queued deal with a disposed wrapper is skipped, and a rejected native call is silent', function()
    local system = new()
    local seen = 0
    system:beforeArmor(function(hit)
        if nameOf(hit.target) == 'a' then
            deal(system, 'doomed', 1); deal(system, 'b', 1)
            unit('doomed'):remove()
        end
    end)
    system:observe(function() seen = seen + 1 end)
    system:start()
    deal(system, 'a', 1)
    eq(join(dealt), 'a,b'); eq(seen, 2)
    rejected = true
    resetCalls()
    deal(system, 't', 1); deal(system, 't', 1)
    eq(callCount('UnitDamageTarget'), 2); eq(seen, 2); eq(#PRINTED, 0)
end)

test('deal copies the request', function()
    local system = new()
    local request = {source = unit('s'), target = unit('later'), amount = 3}
    system:beforeArmor(function(hit)
        if nameOf(hit.target) == 'a' then
            system:deal(request)
            request.target, request.amount = unit('changed'), 99
        end
    end)
    system:start()
    deal(system, 'a', 1)
    eq(join(dealt), 'a,later')
    expectCall('UnitDamageTarget', handle('s'), handle('later'), 3, false, false, ATTACK_TYPE_NORMAL,
        DAMAGE_TYPE_NORMAL, WEAPON_TYPE_WHOKNOWS)
    eq(#PRINTED, 0)
end)

test('dispose inside a listener drops the queued deals', function()
    local system = new()
    system:beforeArmor(function()
        deal(system, 'never', 1)
        system:dispose()
    end)
    system:start()
    deal(system, 'outer', 1)
    eq(join(dealt), 'outer'); eq(system:getCurrent(), nil)
    failsAt(function() system:deal({source = unit('s'), target = unit('t'), amount = 1}) end,
        'DamageSystem.deal: the system is disposed')
    eq(#PRINTED, 0)
end)

test('sourceOf credits another Unit; nil, a failure and a wrong value keep the dealer', function()
    local messages, answer = {}, nil
    local system = new({
        onError = function(message) messages[#messages + 1] = message end,
        sourceOf = function(dealer)
            if answer == 'fail' then error('resolver broke', 0) end
            if answer == 'wrong' then return 5 end
            if nameOf(dealer) == 'dummy' then return unit('hero') end
            return nil
        end,
    })
    local seen = {}
    system:observe(function(hit)
        seen[#seen + 1] = nameOf(hit.source) .. ' via ' .. nameOf(hit.dealer) .. ' ' .. tostring(hit.metadata)
    end)
    system:start()
    pre('t', 10, 'dummy'); post('t', 5, 'dummy')
    pre('t'); post('t')
    deal(system, 't', 1, 'spell', 'dummy')
    post('t', 5, 'dummy')
    eq(join(seen), 'hero via dummy nil,s via s nil,hero via dummy spell,hero via dummy nil'); eq(#messages, 0)
    answer = 'fail'
    pre('t', 10, 'dummy'); post('t', 5, 'dummy')
    answer = 'wrong'
    pre('t', 10, 'dummy'); post('t', 5, 'dummy')
    eq(seen[5], 'dummy via dummy nil'); eq(seen[6], 'dummy via dummy nil')
    eq(#messages, 2); eq(messages[1], 'resolver broke'); eq(messages[2], 'expected a Unit or nil')
    eq(#PRINTED, 0)
    system = new({sourceOf = function() error('printed resolver', 0) end})
    system:start()
    pre('t'); post('t')
    eq(#PRINTED, 1); eq(PRINTED[1], '[systems] Damage sourceOf failed: printed resolver')
end)

test('deal checks its arguments at the caller', function()
    local system = new()
    system:start()
    resetCalls()
    failsAt(function() system:deal(5) end, 'DamageSystem.deal: expected a damage request table')
    local gone = unit('gone')
    gone:remove()
    local cases = {
        {'source', {source = 5}}, {'source', {source = gone}}, {'target', {target = {}}}, {'target', {target = gone}},
        {'amount', {amount = -1}}, {'amount', {amount = 0 / 0}}, {'amount', {amount = '1'}},
        {'attack', {attack = 1}}, {'ranged', {ranged = 'yes'}},
    }
    for _, case in ipairs(cases) do
        local request = {source = unit('s'), target = unit('t'), amount = 1}
        for key, value in pairs(case[2]) do request[key] = value end
        failsAt(function() system:deal(request) end, 'DamageSystem.deal: expected a damage request: ' .. case[1])
    end
    eq(callCount('UnitDamageTarget'), 0)
    failsAt(function() DamageSystem.deal({}, {}) end, 'DamageSystem.deal: expected DamageSystem')
end)
```

- [ ] **Step 2:** `yue -e tests/run.lua damage` → `damage: 13/30 tests failed`: every new test fails, twelve with
  `attempt to call a nil value (method 'deal')` and one with `expected a,b, got a`.

- [ ] **Step 3: Implement** — fourteen edits to `src/systems/damage.lua`, in file order. Each "Replace" text occurs
  exactly once.

1. **The module comment and the Unit import.** Replace

```lua
local Timer = require('wrappers.timer')

---A damage pipeline on wrappers.damage: listeners before armor, after armor and once the final amount is known.
---Nothing is created before start().
```

with

```lua
local Timer = require('wrappers.timer')
local Unit = require('wrappers.unit')

---A damage pipeline on wrappers.damage: listeners before armor, after armor and once the final amount is known; script
---damage that never nests inside another hit's listeners; and attribution of a hit to its real caster. Nothing is
---created before start().
```

2. **The system's script-damage fields.** Replace

```lua
---@field package damagedToken MoonwellWrappers.DamageListener?
```

with

```lua
---@field package damagedToken MoonwellWrappers.DamageListener?
---@field package queue MoonwellSystems.DamageDeal[] Deals waiting for the current hit to finish.
---@field package chain integer Deals issued since the queue was last empty.
---@field package draining boolean
---@field package invocation MoonwellSystems.DamageDeal? The deal whose damageTarget call is running.
```

3. **The request and deal classes, before the `DamageListener` class.** Replace

```lua
---@class MoonwellSystems.DamageListener
```

with

```lua
---@class MoonwellSystems.DamageRequest
---@field source MoonwellWrappers.Unit The Unit that deals the damage.
---@field target MoonwellWrappers.Unit
---@field amount number Finite and not negative.
---@field attack boolean? Default false.
---@field ranged boolean? Default false.
---@field attackType attacktype? Default ATTACK_TYPE_NORMAL.
---@field damageType damagetype? Default DAMAGE_TYPE_NORMAL.
---@field weaponType weapontype? Default WEAPON_TYPE_WHOKNOWS.
---@field metadata any The resulting hit's metadata.

---A queued copy of a request.
---@class MoonwellSystems.DamageDeal
---@field source MoonwellWrappers.Unit
---@field target MoonwellWrappers.Unit
---@field amount number
---@field attack boolean
---@field ranged boolean
---@field attackType attacktype
---@field damageType damagetype
---@field weaponType weapontype
---@field metadata any
---@field claimed boolean Whether a hit has taken the metadata.
---@field hit MoonwellSystems.Hit? The hit that took it.

---@class MoonwellSystems.DamageListener
```

4. **`resolve`, `forget` and `drain`, before `damaging`.** Replace

```lua
---@param system MoonwellSystems.DamageSystem
---@param event MoonwellWrappers.DamagingEvent
local function damaging(system, event)
```

with

```lua
---The Unit a hit is credited to: what sourceOf answers, or the dealer.
---@param system MoonwellSystems.DamageSystem
---@param dealer MoonwellWrappers.Unit?
---@return MoonwellWrappers.Unit?
local function resolve(system, dealer)
    local sourceOf = system.sourceOf
    if not sourceOf or not dealer then return dealer end
    local ok, result = pcall(sourceOf, dealer)
    if not ok then
        Callback.report('Damage sourceOf', system.onError, result)
        return dealer
    end
    if result == nil then return dealer end
    if getmetatable(result) ~= Unit then
        Callback.report('Damage sourceOf', system.onError, 'expected a Unit or nil')
        return dealer
    end
    return result
end

---@param pending MoonwellSystems.Hit[]
---@param hit MoonwellSystems.Hit
local function forget(pending, hit)
    for index = #pending, 1, -1 do
        if pending[index] == hit then table.remove(pending, index); return end
    end
end

---Deals queued requests one at a time, while no hit is being handled and none is pending.
---@param system MoonwellSystems.DamageSystem
local function drain(system)
    if not system.running or system.draining or system.depth > 0 then return end
    system.draining = true
    while system.running and #system.queue > 0 and #system.pending == 0 do
        if system.chain >= system.maxChain then
            local dropped = #system.queue
            system.queue = {}
            system.chain = 0
            Callback.report('Damage chain', system.onError,
                'more than ' .. system.maxChain .. ' deals in one chain; ' .. dropped .. ' queued deals dropped')
            break
        end
        system.chain = system.chain + 1
        local deal = table.remove(system.queue, 1)
        local source, target = deal.source, deal.target
        if not source:isDisposed() and not target:isDisposed() then
            local previous = system.invocation
            system.invocation = deal
            source:damageTarget(target, deal.amount, deal.attack, deal.ranged, deal.attackType, deal.damageType,
                deal.weaponType)
            system.invocation = previous
            -- A hit of this deal that is still pending got no DAMAGED: drop it, so nothing later pairs with it.
            if deal.hit then forget(system.pending, deal.hit) end
        end
    end
    system.draining = false
    if #system.queue == 0 then system.chain = 0 end
end

---@param system MoonwellSystems.DamageSystem
---@param event MoonwellWrappers.DamagingEvent
local function damaging(system, event)
```

5. **`damaging`: claim the metadata.** Replace

```lua
    local dealer, amount = event.source, event.amount
```

with

```lua
    local dealer = event.source
    local metadata
    local deal = system.invocation
    if deal and not deal.claimed and deal.source == dealer and deal.target == target then
        deal.claimed = true
        metadata = deal.metadata
    else
        deal = nil
    end
    local amount = event.amount
```

6. **`damaging`: the resolved source and the metadata.** Replace

```lua
        source = dealer, dealer = dealer, target = target, amount = amount,
        isAttack = event.isAttack, attackType = event.attackType, damageType = event.damageType,
        weaponType = event.weaponType, phase = 'beforeArmor', initialAmount = amount,
```

with

```lua
        source = resolve(system, dealer), dealer = dealer, target = target, amount = amount,
        isAttack = event.isAttack, attackType = event.attackType, damageType = event.damageType,
        weaponType = event.weaponType, metadata = metadata, phase = 'beforeArmor', initialAmount = amount,
```

7. **`damaging`: remember the deal's hit.** Replace

```lua
    pending[#pending + 1] = hit
```

with

```lua
    pending[#pending + 1] = hit
    if deal then deal.hit = hit end
```

8. **`damaged`: the resolved source of an unpaired hit.** Replace

```lua
            source = dealer, dealer = dealer, target = target, isAttack = isAttack,
```

with

```lua
            source = resolve(system, dealer), dealer = dealer, target = target, isAttack = isAttack,
```

9. **`damaged`: run queued deals at the end.** Replace

```lua
    system.depth = system.depth - 1
    system.current = previous
end

---@param system MoonwellSystems.DamageSystem
local function settle(system)
```

with

```lua
    system.depth = system.depth - 1
    system.current = previous
    drain(system)
end

---@param system MoonwellSystems.DamageSystem
local function settle(system)
```

10. **`settle`: run queued deals.** Replace

```lua
    if #system.pending > 0 then system.pending = {} end
end
```

with

```lua
    if #system.pending > 0 then system.pending = {} end
    drain(system)
end
```

11. **`DamageSystem.new`: the script-damage state.** Replace

```lua
        scheduled = false,
    }, DamageSystem)
```

with

```lua
        scheduled = false, queue = {}, chain = 0, draining = false,
    }, DamageSystem)
```

12. **`deal` and its checks, before `getCurrent`.** Replace

```lua
---The hit whose listeners are running (the innermost one), or nil outside damage events.
```

with

```lua
---@param value unknown
---@return boolean
local function liveUnit(value) return getmetatable(value) == Unit and not value:isDisposed() end

---@param request table
---@return string? field The first invalid field, or nil.
local function invalid(request)
    if not liveUnit(request.source) then return 'source' end
    if not liveUnit(request.target) then return 'target' end
    if not Check.finite(request.amount) or request.amount < 0 then return 'amount' end
    if request.attack ~= nil and type(request.attack) ~= 'boolean' then return 'attack' end
    if request.ranged ~= nil and type(request.ranged) ~= 'boolean' then return 'ranged' end
    return nil
end

---Deals damage through the pipeline. Outside damage events it runs at once; inside a listener it is queued and runs,
---first in first out, after the current hit. The hit it causes carries `request.metadata`.
---@param request MoonwellSystems.DamageRequest
function DamageSystem:deal(request)
    local system = Check.receiver(self, DamageSystem, 'DamageSystem', 'DamageSystem.deal')
    if system.disposed then error('[systems] DamageSystem.deal: the system is disposed', 2) end
    if not system.running then error('[systems] DamageSystem.deal: the system is not started', 2) end
    if type(request) ~= 'table' then error('[systems] DamageSystem.deal: expected a damage request table', 2) end
    local field = invalid(request)
    if field then error('[systems] DamageSystem.deal: expected a damage request: ' .. field, 2) end
    local queue = system.queue
    if #queue >= system.maxQueue then
        error('[systems] DamageSystem.deal: the queue is full (' .. system.maxQueue .. ' deals)', 2)
    end
    queue[#queue + 1] = {
        source = request.source, target = request.target, amount = request.amount,
        attack = request.attack == true, ranged = request.ranged == true,
        attackType = request.attackType or ATTACK_TYPE_NORMAL,
        damageType = request.damageType or DAMAGE_TYPE_NORMAL,
        weaponType = request.weaponType or WEAPON_TYPE_WHOKNOWS,
        metadata = request.metadata, claimed = false,
    }
    drain(system)
end

---The hit whose listeners are running (the innermost one), or nil outside damage events.
```

13. **`dispose`: drop the queue.** Replace

```lua
---Stops listening and drops every listener and pending hit. Inside a listener, the hit's remaining listeners do
---not run. Idempotent.
```

with

```lua
---Stops listening and drops every listener, queued deal and pending hit. Inside a listener, the hit's remaining
---listeners do not run. Idempotent.
```

14. **`dispose`: reset the queue and the chain.** Replace

```lua
    system.pending = {}
    for _, key in ipairs(LISTS) do
```

with

```lua
    system.queue, system.pending, system.chain = {}, {}, 0
    for _, key in ipairs(LISTS) do
```

- [ ] **Step 4:** `yue -e tests/run.lua; echo "exit $?"` → `damage: SUITE PASSED: 30 tests`, `All 12 suites passed`,
  `exit 0`. The file has 549 lines.

- [ ] **Step 5: Commit** `src/systems/damage.lua tests/damage.lua` — `feat: systems.damage deal and attribution`.

---

### Task 4: Sweep, imports, fixtures and integration

**Files:** Modify `tests/blame.lua`, `tests/imports.lua`, `tests/editor-positive.lua`, `tests/editor-positive.yue`,
`tests/editor-negative.lua`, `tools/integration.lua`.

- [ ] **Step 1: The sweep** — `tests/blame.lua` becomes (it now also sweeps every class a module exposes, such as
  `DamageSystem.Hit`):

```lua
-- Every error a public function raises for wrong arguments is a [systems] error at the caller's line (spec §4.2).
-- The sweep calls every function of every module, and of every class a module exposes (such as DamageSystem.Hit),
-- with an empty table as its first argument.
local modules = {'scheduler', 'signal', 'scope', 'time', 'buffs', 'aura', 'dummy', 'damage'}

---@return integer checked How many functions raised.
local function sweep(name, class, wrong)
    local checked = 0
    for key, fn in pairs(class) do
        if type(fn) == 'function' then
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
    return checked
end

test('every public function given a wrong argument points at its caller', function()
    local wrong = {}
    for _, name in ipairs(modules) do
        local module = require('systems.' .. name)
        assert(sweep(name, module, wrong) > 0, name .. ': no function raised')
        for key, class in pairs(module) do
            if key ~= '__index' and type(class) == 'table' and class.__index == class then
                assert(sweep(name .. '.' .. key, class, wrong) > 0, name .. '.' .. key .. ': no function raised')
            end
        end
    end
    table.sort(wrong)
    assert(#wrong == 0, #wrong .. ' misplaced errors:\n' .. table.concat(wrong, '\n'))
end)
```

- [ ] **Step 2: Imports** — in `tests/imports.lua`, before the `the dummy module calls no native…` test (so that no
  earlier test has loaded `systems.dummy`):

```lua
test('the damage module calls no native at import and loads no dummy or trigger module', function()
    require('systems.damage')
    eq(totalCalls(), 0)
    eq(package.loaded['wrappers.damage'] ~= nil, true)
    eq(package.loaded['systems.dummy'], nil)
    eq(package.loaded['wrappers.trigger'], nil)
end)
```

Run `yue -e tests/run.lua; echo "exit $?"` → `imports: SUITE PASSED: 5 tests`, `blame: SUITE PASSED: 1 tests`,
`All 12 suites passed` (90 tests in all).

- [ ] **Step 3: Fixtures.** In `tests/editor-positive.lua`, before `print(scope:isActive())`:

```lua
local DamageSystem = require('systems.damage')
local damage = scope:add(DamageSystem.new({sourceOf = function(dealer) return dummies:sourceOf(dealer) end,
    onError = function(message) print(message) end, maxQueue = 16, maxChain = 8, maxPending = 8}))
damage:start()
scope:own(damage:beforeArmor(function(hit)
    hit:setAmount(hit.amount * 2)
    hit:setAttackType(ATTACK_TYPE_MAGIC); hit:setDamageType(DAMAGE_TYPE_MAGIC); hit:setWeaponType(WEAPON_TYPE_WHOKNOWS)
end, -1))
scope:own(damage:afterArmor(function(hit) if hit:isLethal() then hit:cancel() end end))
scope:own(damage:observe(function(hit)
    local source, dealer = hit.source, hit.dealer
    print(source and source:getName(), dealer and dealer:getName(), hit.target:getName(), hit.amount, hit.metadata,
        hit.phase, hit.initialAmount, hit.beforeArmorAmount, hit.armorAmount, hit.cancelled, hit.paired, hit.isAttack)
end))
damage:deal({source = hero, target = hero, amount = 5, attack = true, ranged = false, damageType = DAMAGE_TYPE_MAGIC,
    metadata = 'spell'})
local current = damage:getCurrent()
print(current and current.phase)
```

In `tests/editor-positive.yue`, add after the `systems.time` import:

```
import "systems.dummy" as Dummies
import "systems.damage" as DamageSystem
```

and at the end of the `mw.on_main` body:

```
  dummies = scope\add Dummies.new clock
  damage = scope\add DamageSystem.new sourceOf: dummies\sourceOf
  damage\start!
  damage\beforeArmor (hit) -> hit\setAmount hit.amount * 2
  damage\afterArmor ((hit) -> hit\cancel! if hit\isLethal!), 5
  damage\observe (hit) -> print hit.amount, hit.metadata
```

In `tests/editor-negative.lua`, before `return true` (LuaLS checks a nil-able value only through a local, so
`getCurrent()` is assigned first):

```lua
local DamageSystem = require('systems.damage')
local damage = DamageSystem.new()
damage:beforeArmor('x') -- EXPECT param-type-mismatch
damage:nonexistent() -- EXPECT undefined-field
damage:observe(function(hit) hit:setAmount('1') end) -- EXPECT param-type-mismatch
local current = damage:getCurrent()
print(current.amount) -- EXPECT need-check-nil
DamageSystem.new({maxQueue = 'many'}) -- EXPECT assign-type-mismatch
```

- [ ] **Step 4: Integration.** In `tools/integration.lua`:
- `public` becomes `{'scheduler', 'signal', 'scope', 'time', 'buffs', 'aura', 'dummy', 'damage'}`;
- add to `entries`, after `dummy`:

```lua
    damage = {source = 'import "systems.damage" as DamageSystem\nd = DamageSystem.new!\nd\\dispose!\n', systems = {},
        wrappers = {unit = true, player = true, item = true, timer = true, damage = true}},
```

- replace the gate block (from `-- The gate example builds…` to before `print('Integration passed')`) with:

```lua
-- The gate examples build and have clean editor diagnostics.
Lib.remove(consumer .. '/lua/positive.lua')
for _, name in ipairs({'gate', 'gate-damage'}) do
    Lib.copy('examples/' .. name .. '.yue', consumer .. '/src/main.yue')
    moonwell('check'); moonwell('build --minify')
    compileEditor()
    clean(Lib.diagnose(luals, consumer, name), 'Gate example ' .. name)
end
print('Gate examples: both build and their editor diagnostics are clean; game execution remains manual')
```

  Integration needs `examples/gate-damage.yue`, which Task 5 creates: run integration in Task 5, Step 2.

- [ ] **Step 5:** Run `yue -e tests/run.lua; echo "exit $?"` and the syntax check. Expected: all 12 suites pass; the
  syntax check counts 32 files.

- [ ] **Step 6: Commit** `tests/blame.lua tests/imports.lua tests/editor-positive.lua tests/editor-positive.yue
  tests/editor-negative.lua tools/integration.lua` — `test: sweep, imports, fixtures and bundles for release 3`.

---

### Task 5: The gate example, the gate map and the docs

**Files:** Create `examples/gate-damage.yue`; modify `README.md`, `CHANGELOG.md`, `CONTRIBUTING.md`, `AGENTS.md`;
modify (not under git) `../wrappers-gate/gate.ts`.

- [ ] **Step 1: The gate example** — `examples/gate-damage.yue`:

```
-- The moonwell-systems damage gate (release 3). Built by the gate map's `systems-damage` run (../wrappers-gate,
-- deno task gate systems-damage); CONTRIBUTING lists every expected message. It starts just after the map loads.
-- The dummy type e000 comes from the map's object data (README, "The dummy unit type").
import "moonwell" as mw
import "moonwell.macros" as {:$FourCC}
import "wrappers.player" as Player
import "wrappers.unit" as Unit
import "wrappers.timer" as Timer
import "systems.scheduler" as Scheduler
import "systems.dummy" as Dummies
import "systems.damage" as DamageSystem

nameOf = (unit) -> if unit then unit\getName! else "nobody"

damageGate = (owner) ->
  clock = Scheduler.new!
  clock\start!
  hostile = Player.fromIndex 12
  hero = Unit.create owner, $FourCC("Hpal"), -300, 0, 0
  attacker = Unit.create owner, $FourCC("hfoo"), -150, 150, 0
  victim = Unit.create hostile, $FourCC("hfoo"), 0, 150, 180
  breaker = Unit.create hostile, $FourCC("hspt"), 0, -150, 180
  -- Paused units do not fight on their own, so every hit in the log is one the gate caused.
  for unit in *{hero, attacker, victim, breaker}
    unit\pause true
  victim\setMaxLife 2000
  victim\setLife 2000
  dummies = Dummies.new clock
  damage = DamageSystem.new sourceOf: dummies\sourceOf
  damage\beforeArmor (hit) ->
    if hit.metadata == "crit"
      hit\setAmount hit.amount * 2
    elseif hit.metadata == "cancel"
      hit\cancel!
    elseif hit.metadata == "chain"
      damage\deal source: attacker, target: victim, amount: 5, metadata: "follow-up"
      print "Damage chain: follow-up queued"
    elseif hit.metadata == "immune"
      print "Damage immune: before armor ran, amount", hit.amount
  damage\afterArmor (hit) ->
    hit\setAmount math.min hit.amount, 50 if hit.metadata == "crit"
  damage\observe (hit) ->
    print "Damage #{hit.metadata or 'native'}:", nameOf(hit.source), "via", nameOf(hit.dealer), "->",
      nameOf(hit.target), hit.initialAmount, ">", hit.beforeArmorAmount, ">", hit.armorAmount, ">", hit.amount,
      "attack", hit.isAttack
    attacker\pause true if hit.isAttack
  damage\start!
  strike = (number, request) ->
    before = request.target\getLife!
    damage\deal request
    print "Damage step #{number} life lost", before - request.target\getLife!
  clock\after 1, -> strike 1, source: attacker, target: victim, amount: 100, metadata: "baseline"
  clock\after 2, -> strike 2, source: attacker, target: victim, amount: 100, metadata: "crit"
  clock\after 3, -> strike 3, source: attacker, target: victim, amount: 100, metadata: "cancel"
  clock\after 4, ->
    lease = dummies\cast
      owner: owner, typeId: $FourCC("e000"), x: -300, y: 0, ability: $FourCC("AHtb"), order: "thunderbolt"
      target: victim, duration: 2, source: hero
    print "Damage step 4 dummy cast accepted", lease\isOrderAccepted!
  clock\after 5, -> strike 5, source: attacker, target: victim, amount: 100, metadata: "chain"
  clock\after 6, ->
    strike 6, source: attacker, target: breaker, amount: 100, damageType: DAMAGE_TYPE_MAGIC, metadata: "immune"
  clock\after 7, ->
    attacker\pause false
    print "Damage step 7 attack ordered", attacker\issueTargetOrder "attack", victim
  clock\after 9, ->
    damage\dispose!
    attacker\pause true
    attacker\damageTarget victim, 10, false, false, ATTACK_TYPE_NORMAL, DAMAGE_TYPE_NORMAL, WEAPON_TYPE_WHOKNOWS
    dummies\dispose!
    clock\dispose!
    print "Damage gate done"
  print "Damage gate started"

mw.on_main ->
  start = Timer.create!
  start\start 0, false, (self) ->
    self\destroy!
    damageGate Player.fromIndex 0
```

- [ ] **Step 2: Integration.** Run the integration check. Expected:
  `LuaLS: 14 intentional type errors detected at the expected lines`,
  `Moonwell: every entry point (8) bundles only what it imports`,
  `Gate examples: both build and their editor diagnostics are clean; game execution remains manual`,
  `Integration passed`.

- [ ] **Step 3: The gate map.** In `../wrappers-gate/gate.ts`:
- after the `//   systems …` usage line, add:

```ts
//   systems-damage    moonwell-systems release 3 damage gate (../moonwell-systems/examples/gate-damage.yue)
```

- `copiedRuns` gains `"systems-damage": "../moonwell-systems/examples/gate-damage.yue",`;
- in `build`, the copied entry becomes ``entry = `src/gate_${name.replaceAll("-", "_")}.yue`;`` (a module file name
  without a hyphen).

  Run `deno task gate systems-damage --no-launch` in `../wrappers-gate` → `Gate map: gate-maps/systems-damage.w3x`.

- [ ] **Step 4: README.**
- The first paragraph's list becomes: "a deterministic scheduler, signals, ownership scopes, time helpers, script
  buffs, auras, dummy casters and a damage pipeline today; physics and save codes in later releases".
- After the `systems.dummy` section (after "The dummy unit type"), add:

````markdown
### `systems.damage`

- `DamageSystem.new({sourceOf?, onError?, maxQueue = 128, maxChain = 64, maxPending = 64})`
- `start()`; `beforeArmor(callback, priority = 0)`, `afterArmor(callback, priority = 0)` and
  `observe(callback, priority = 0)`: each returns a remove function; lower priority runs first
- `deal(request)`, `getCurrent()`, `dispose()`
- a hit, to read: `source`, `dealer`, `target`, `amount`, `isAttack`, `attackType`, `damageType`, `weaponType`,
  `metadata`, `phase`, `initialAmount`, `beforeArmorAmount`, `armorAmount`, `cancelled`, `paired`
- a hit, to change: `setAmount(n)`, `cancel()`, `setAttackType(t)`, `setDamageType(t)`, `setWeaponType(t)`; and
  `isLethal()`

Every hit in the map runs three phases: the `beforeArmor` listeners (the amount and the types can change), the
`afterArmor` listeners (the amount can change) and the observers (nothing can change). A listener gets the hit. Its
fields are for reading; change it with its methods. A setter raises at your line when its phase has passed, when an
observer calls it, or when the hit is not the one being handled. A cancelled hit stays at 0.

`deal` takes a table: `source`, `target`, `amount`, and optionally `attack`, `ranged`, `attackType`, `damageType`,
`weaponType` and `metadata`. Outside damage events it runs at once. Inside a listener it is queued and runs after the
current hit, first in first out, so script damage never nests. Only the hit that `deal` causes carries its `metadata`.
A listener that answers every hit with another `deal` is stopped after `maxChain` deals, and a full queue raises at the
`deal` line.

`sourceOf(dealer)` credits a hit to another Unit: `hit.source` is its answer (or the dealer), and `hit.dealer` is the
unit the game reported. With `sourceOf: dummies\sourceOf`, a dummy's damage counts for its real caster while the
dummy's lease lasts.

- `hit.source` and `hit.dealer` are nil when the game gives no source.
- `isAttack` is the game's value: false for script damage, even with `attack: true`.
- A hit whose DAMAGED event never comes gets no `afterArmor` or observer call.
- To react to one unit's hits, look `hit.target` up in your own table inside one listener.

```yue
import "systems.damage" as DamageSystem

damage = DamageSystem.new sourceOf: dummies\sourceOf
damage\beforeArmor (hit) -> hit\setAmount hit.amount * 2 if hit.metadata == "crit"
damage\afterArmor (hit) -> hit\cancel! if shields[hit.target]
damage\observe (hit) -> print hit.amount
damage\start!
damage\deal source: hero, target: enemy, amount: 50, metadata: "crit"
```
````

- "Changes from wc3-lib" gains:

```markdown
- The damage system has no port: `DamageSystem.new` is the Warcraft system, and a `deal` request is one flat table.
- A hit changes through setter methods, which raise at the listener's line; observers get the hit itself, and its
  setters raise there. `invalid-amount` is gone.
- `sourceOf`, `hit.dealer` and hits with no source are new (wc3-lib dropped hits without a source).
- Only failures are reported: a missing or unpaired DAMAGED event and a rejected native call are silent. A full queue
  raises instead of returning false, and `deal` returns nothing.
```

- [ ] **Step 5: CONTRIBUTING.** After release 2's gate steps (before the `v0.1.0: passed…` record), add:

```markdown
Release 3 (v0.3.0) has its own run, `deno task gate systems-damage`, about 10 seconds. A paladin stands on the left;
above the centre your footman faces a hostile footman; a hostile Spell Breaker stands below. All four are paused.
`Damage gate started` prints first. A hit prints as
`Damage <metadata>: <source> via <dealer> -> <target> <initial> > <before armor> > <after armor> > <final> attack <b>`.

13. At 1 s: `Damage baseline: Footman via Footman -> Footman 100.0 > 100.0 > <X> > <X> attack false`, then
    `Damage step 1 life lost <X>` (100 reduced by armor; record X).
14. At 2 s: `Damage crit: … 100.0 > 200.0 > <2X> > 50.0 attack false`, then `Damage step 2 life lost 50.0`.
15. At 3 s: `Damage step 3 life lost 0.0`. Record whether a `Damage cancel: … 100.0 > 0.0 > 0.0 > 0.0` line prints
    before it (whether Warcraft sends DAMAGED for a zero amount).
16. At 4 s: `Damage step 4 dummy cast accepted true`; a Storm Bolt hits the hostile footman and
    `Damage native: Paladin via Dummy -> Footman …` prints (record the amounts).
17. At 5 s: `Damage chain: follow-up queued`, then the `Damage chain: …` line, then
    `Damage follow-up: … 5.0 > 5.0 > …`, then `Damage step 5 life lost <n>` (both hits).
18. At 6 s: `Damage step 6 life lost <n>` for the Spell Breaker. Record n, and whether
    `Damage immune: before armor ran, amount 100.0` and a `Damage immune: …` hit line print.
19. At 7 s: `Damage step 7 attack ordered true`; your footman attacks, and one
    `Damage native: Footman via Footman -> Footman … attack true` line prints.
20. At 9 s: `Damage gate done`, and no line after it. No `[systems] … failed` line prints in the run.
```

  In the "Publication and tag gate" paragraph, "`examples/gate.yue` as `src/main.yue`" becomes "each gate example
  (`examples/gate.yue`, `examples/gate-damage.yue`) in turn as `src/main.yue`".

- [ ] **Step 6: CHANGELOG and AGENTS.**
- CHANGELOG: a new first section:

```markdown
## Unreleased

Release 3 of the wc3-lib port (spec `2026-10-01-moonwell-systems-release-3-design` in the Moonwell repository).

- `systems.damage`: listeners before armor, after armor and once the final amount is known, on `wrappers.damage`. A
  hit changes through setter methods that raise at the listener's line. `deal` queues script damage so it never nests,
  and carries `metadata`. `sourceOf` credits a hit to another Unit, for example a dummy's damage to its caster.
- `Callback.report` in `systems.internal.callback`.
- The blame sweep also covers the classes a module exposes (`DamageSystem.Hit`).
```

- AGENTS: "Releases 3 to 5: damage; physics; persistence." becomes "Release 3 (v0.3.0): damage. Releases 4 and 5:
  physics; persistence."; the Process line names both gate runs (`deno task gate systems` and
  `deno task gate systems-damage`); two new pitfalls:

```markdown
- LuaLS's `--check` mangles a project path that contains `--` (it reads it as an option): keep work folders out of
  such paths.
- LuaLS reports `need-check-nil` for a nil-able local, not for a chained call (`a:b().c`): assign the result first.
```

- [ ] **Step 7: Checks and commit.** Run the three checks. Commit `examples/gate-damage.yue README.md CHANGELOG.md
  CONTRIBUTING.md AGENTS.md` — `docs: release 3 docs and the damage gate`.

---

### Task 6: Release

- [ ] **Step 1:** The maintainer runs `deno task gate systems-damage` in `../wrappers-gate` and sends the F12 log.
  Steps 13 to 20 of CONTRIBUTING must hold; record the measured values (X, the cancel line, the Storm Bolt amounts,
  the Spell Breaker's readings). If a reading contradicts the spec (for example, a failure line prints for the
  immune hit), stop and fix it test-first before releasing; if it only settles an open measurement, write it into the
  README notes and spec §5.2.
- [ ] **Step 2:** Record: CHANGELOG `## 0.3.0 (<date>)` with a release-gate section; CONTRIBUTING `v0.3.0:` record;
  README Status (`v0.3.0`, the new module) and the `tag = "v0.3.0"` example; AGENTS tag list. Run the three checks.
  Commit, push, tag `v0.3.0` on the verified commit, push the tag, GitHub pre-release from the changelog section
  (`gh` at `C:\Program Files\GitHub CLI\gh.exe`). Verify each step's exit code separately.
- [ ] **Step 3:** Tag consumption in `../systems-tag-check-030` (a map made with `init --link`, wrappers `v0.7.0`,
  systems `v0.3.0`): with each gate example in turn as `src/main.yue`: check, build, build `--minify`. The lock
  records both tags' commits; the fetched systems files match the tag's `src/` byte for byte; the lock is unchanged
  after removing `.moonwell/` and checking again. Record it in CONTRIBUTING; commit; push.
- [ ] **Step 4:** Moonwell records: `AGENTS.md` (a state bullet for v0.3.0 with the gate's measurements; next work
  becomes the spec for release 4: geometry, terrain, missile and knockback), the roadmap's phase 3 status, and the
  `CHANGELOG.md` Unreleased line; commit; push; `gh run list`.
