# Moonwell Systems Release 4 (v0.4.0) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** moonwell-systems v0.4.0: `systems.geometry`, `systems.terrain`, `systems.missile` and `systems.knockback`.

**Architecture:** Two unchecked internal modules carry the per-tick work: `internal.vector` (vector functions on plain
numbers) and `internal.ground` (terrain sampling on a state table that owns a location, a probe item and a rect). The
public `geometry` and `terrain` modules are their checked versions. `Missiles` and `Knockbacks` take a `Scheduler`,
tick from it only while they have something to move, call raw natives on handles they own or have just enumerated,
and wrap a unit only for a callback.

**Tech Stack:** as releases 1 to 3 (annotated Lua 5.3, `yue -e` tooling, LuaLS 3.19.1, Lua 5.3.6 `luac`,
moonwell-wrappers v0.7.0, Moonwell 0.5.2).

**Spec:** `docs/superpowers/specs/2026-10-01-moonwell-systems-release-4-design.md` (and Part 1 of
`2026-09-30-moonwell-systems-design.md`).

**Verified in advance:** every code block below was run on 2026-10-01 in a scratch copy of the repository: the suites
(17 suites, 143 tests), the syntax check (43 files) and full integration (20 expected negative diagnostics, 12 entry
points, three gate examples) passed. Mutation checks on the missile, knockback and ground modules shaped the tests;
five lines they showed to be redundant were removed (see "Departures from the spec").

## Global Constraints

- **Repository:** `C:\Users\mdlsvensson\Repo\moonwell-systems`; Moonwell records in `C:\Users\mdlsvensson\Repo\moonwell`;
  the gate map in `C:\Users\mdlsvensson\Repo\wrappers-gate` (not under git). Commit on `main`, explicit paths only,
  each check run as its own command.
- **Checks**, from the repository root:
  - `yue -e tests/run.lua`
  - `MOONWELL_LUAC=../moonwell-wrappers/.tools/lua53/luac53.exe yue -e tools/check.lua`
  - `MOONWELL_LUALS="C:/Users/mdlsvensson/.antigravity-ide/extensions/sumneko.lua-3.19.1-win32-x64/server/bin/lua-language-server.exe" yue -e tools/integration.lua`
- **Write files with the file tools, not shell heredocs:** Git Bash turns `\\` into `\` and `\n` into a newline.
- **Messages:** `[systems] <Class>.<method>: <problem>`. New texts: `expected finite numbers`,
  `expected a non-negative angle`, `expected a non-negative radius`, `expected Terrain`, `expected an item type`,
  `expected finite coordinates`, `the terrain is disposed`, `expected Missiles`, `expected Missile`,
  `expected missile options: <field>`, `expected a missile request table`, `expected a missile request: <field>`,
  `expected a finite velocity`, `a followGround missile has no vertical velocity`, `expected Knockbacks`,
  `expected Knockback`, `expected knockback options: <field>`, `expected a live Unit`,
  `expected a knockback request table`, `expected a knockback request: <field>`, `the system is disposed`.
- **Callback labels:** `Missile callback`, `Missile end`, `Knockback pathing`, `Knockback end`.
- **End reasons.** Missile: `hit-limit`, `expired`, `range`, `ground`, `cancelled`, `disposed`, `error`. Knockback:
  `completed`, `replaced`, `interrupted`, `invalid`, `blocked`, `disposed`, `error`.
- **Levels and tail calls** as releases 1 to 3: 2 in a public function, 3 (+ depth) in a helper;
  `return (helper(...))`.
- **Private fields** use `---@field package`, never `private`. No field shares a name with a method of its class.
- **Lines** stay within 120 columns.
- **Rawcodes as integers:** `'wolg'` is 2003790951, `'phea'` 1885889889.

## Departures from the spec

Found while prototyping; the spec records them (§6.2, §7 and §11).

- **`Missile:getEffect()`**'s Effect may be destroyed by its owner: the wrappers' `destroy()` is idempotent, so the
  missile's own destroy then does nothing. No separate check is needed.
- **A missile's age is the plain sum of its steps.** The step that ends it is cut to the remaining lifetime, and that
  sum equals the lifetime exactly in every case tried; only the distance flown needs setting to `maxRange`.
- **Dead units are skipped when a contact is handled**, not when candidates are collected: one native fewer for every
  living unit near a missile's path.
- **Knockback `'invalid'`** is `not UnitAlive(handle)`. That covers a disposed wrapper, because a Unit wrapper is only
  disposed by `remove()`.
- **A knockback step that moves nowhere takes no pathing sample.**
- **The modules expose their classes** (`Missiles.Missile`, `Knockbacks.Knockback`), as `DamageSystem.Hit` does, so
  the blame sweep reaches their methods.
- **The gate also writes its lines to a file** (`CustomMapData\moonwell-systems-physics.pld`), so the maintainer need
  not screenshot the log.

---

### Task 1: Vectors and `systems.geometry`

**Files:** Create `src/systems/internal/vector.lua`, `src/systems/geometry.lua`, `tests/vector.lua`,
`tests/geometry.lua`; modify `tests/suites.lua`.

**Interfaces:** Produces, unchecked, `Vector.length(x, y, z)`, `Vector.turnToward(vx, vy, vz, tx, ty, tz, maxAngle)`
(returns `x, y, z`), `Vector.segmentSphere(fx, fy, fz, tx, ty, tz, cx, cy, cz, radius)` (returns a fraction or nil)
and `Vector.orientation(vx, vy, vz)` (returns `yaw, pitch`); and the same four on `Geometry`, checked, with
`Geometry.length`'s `z` defaulting to 0.

- [ ] **Step 1: Failing tests** — `tests/vector.lua`:

```lua
local Vector = require('systems.internal.vector')

local function near(actual, expected)
    if math.abs(actual - expected) > 1e-9 then
        error('expected about ' .. tostring(expected) .. ', got ' .. tostring(actual), 2)
    end
end

test('length', function()
    eq(Vector.length(3, 4, 0), 5.0); eq(Vector.length(0, 0, 0), 0.0); near(Vector.length(1, 2, 2), 3)
end)

test('turnToward keeps the speed and turns at most the given angle', function()
    local x, y, z = Vector.turnToward(10, 0, 0, 0, 1, 0, math.pi / 4)
    near(Vector.length(x, y, z), 10); near(math.atan(y, x), math.pi / 4); near(z, 0)
    -- Within reach: exactly the direction, at the old speed.
    x, y, z = Vector.turnToward(10, 0, 0, 0, 5, 0, math.pi)
    near(x, 0); near(y, 10); near(z, 0)
    -- Up toward a climbing direction.
    x, y, z = Vector.turnToward(10, 0, 0, 0, 0, 3, math.pi / 6)
    near(Vector.length(x, y, z), 10); near(math.atan(z, x), math.pi / 6); near(y, 0)
end)

test('turnToward: zero vectors, and a direction straight behind turns left', function()
    local x, y, z = Vector.turnToward(0, 0, 0, 1, 0, 0, 1)
    eq(x, 0); eq(y, 0); eq(z, 0)
    x, y, z = Vector.turnToward(3, 4, 5, 0, 0, 0, 1)
    eq(x, 3); eq(y, 4); eq(z, 5)
    x, y, z = Vector.turnToward(10, 0, 0, -1, 0, 0, math.pi / 2)
    near(x, 0); near(y, 10); near(z, 0)
    -- Moving straight up with the direction straight down: turns toward +x.
    x, y, z = Vector.turnToward(0, 0, 10, 0, 0, -1, math.pi / 2)
    near(x, 10); near(y, 0); near(z, 0)
end)

test('segmentSphere gives the earliest contact as a fraction', function()
    near(Vector.segmentSphere(0, 0, 0, 100, 0, 0, 50, 0, 0, 2), 0.48)
    eq(Vector.segmentSphere(0, 0, 0, 100, 0, 0, 1, 0, 0, 2), 0)      -- starts inside
    near(Vector.segmentSphere(0, 0, 0, 100, 0, 0, 50, 2, 0, 2), 0.5)  -- tangent
    eq(Vector.segmentSphere(0, 0, 0, 100, 0, 0, 50, 3, 0, 2), nil)    -- passes beside it
    eq(Vector.segmentSphere(0, 0, 0, 100, 0, 0, -10, 0, 0, 2), nil)   -- behind the start
    eq(Vector.segmentSphere(0, 0, 0, 100, 0, 0, 150, 0, 0, 2), nil)   -- beyond the end
    eq(Vector.segmentSphere(0, 0, 0, 100, 0, 0, 50, 0, 5, 2), nil)    -- above the path
    eq(Vector.segmentSphere(0, 0, 0, 0, 0, 0, 5, 0, 0, 2), nil)       -- no length, outside
    eq(Vector.segmentSphere(0, 0, 0, 0, 0, 0, 1, 0, 0, 2), 0)         -- no length, inside
end)

test('orientation: yaw from the horizontal direction, a negative pitch when climbing', function()
    local yaw, pitch = Vector.orientation(1, 0, 0)
    near(yaw, 0); near(pitch, 0)
    yaw, pitch = Vector.orientation(0, 1, 0)
    near(yaw, math.pi / 2); near(pitch, 0)
    yaw, pitch = Vector.orientation(-1, 0, 0)
    near(yaw, math.pi)
    yaw, pitch = Vector.orientation(1, 0, 1)
    near(yaw, 0); near(pitch, -math.pi / 4)
    yaw, pitch = Vector.orientation(0, 0, -1)
    near(yaw, 0); near(pitch, math.pi / 2)
    yaw, pitch = Vector.orientation(0, 0, 0)
    eq(yaw, 0); eq(pitch, 0)
end)
```

`tests/geometry.lua`:

```lua
local Geometry = require('systems.geometry')

test('the functions answer like the internal ones', function()
    eq(Geometry.length(3, 4), 5.0); eq(Geometry.length(2, 3, 6), 7.0)
    local x, y, z = Geometry.turnToward(10, 0, 0, -1, 0, 0, math.pi / 2)
    assert(math.abs(x) < 1e-9 and math.abs(y - 10) < 1e-9 and z == 0, x .. ' ' .. y .. ' ' .. z)
    eq(Geometry.segmentSphere(0, 0, 0, 100, 0, 0, 50, 2, 0, 2), 0.5)
    eq(Geometry.segmentSphere(0, 0, 0, 100, 0, 0, 50, 3, 0, 2), nil)
    local yaw, pitch = Geometry.orientation(0, 1, 0)
    assert(math.abs(yaw - math.pi / 2) < 1e-9 and pitch == 0, yaw .. ' ' .. pitch)
    eq(totalCalls(), 0)
end)

test('arguments are checked at the caller', function()
    failsAt(function() Geometry.length('3', 4) end, 'Geometry.length: expected finite numbers')
    failsAt(function() Geometry.length(3, 0 / 0) end, 'Geometry.length: expected finite numbers')
    failsAt(function() Geometry.turnToward(1, 0, 0, 0, 1, 0) end, 'Geometry.turnToward: expected finite numbers')
    failsAt(function() Geometry.turnToward(1, 0, 0, 0, math.huge, 0, 1) end,
        'Geometry.turnToward: expected finite numbers')
    failsAt(function() Geometry.turnToward(1, 0, 0, 0, 1, 0, -1) end,
        'Geometry.turnToward: expected a non-negative angle')
    failsAt(function() Geometry.segmentSphere(0, 0, 0, 1, 0, 0, 0, 0, 0) end,
        'Geometry.segmentSphere: expected finite numbers')
    failsAt(function() Geometry.segmentSphere(0, 0, 0, 1, 0, 0, 0, 0, 0, -1) end,
        'Geometry.segmentSphere: expected a non-negative radius')
    failsAt(function() Geometry.orientation(1, nil, 0) end, 'Geometry.orientation: expected finite numbers')
end)
```

In `tests/suites.lua`, insert `'vector', 'geometry'` after `'damage'` (wrap the line within 120 columns).

- [ ] **Step 2:** `yue -e tests/run.lua vector geometry` → both `ERROR …module 'systems.internal.vector' not found`
  and `…'systems.geometry' not found`.

- [ ] **Step 3: Implement** — `src/systems/internal/vector.lua`:

```lua
---Vector functions on plain numbers, without argument checks and without tables, for the per-tick loops (spec
---2026-10-01 release 4 §3). systems.geometry is the checked public version.
local Vector = {}

local sqrt, acos, cos, sin, atan = math.sqrt, math.acos, math.cos, math.sin, math.atan

---@param x number
---@param y number
---@param z number
---@return number
function Vector.length(x, y, z) return sqrt(x * x + y * y + z * z) end

---Rotates the velocity toward the direction (tx, ty, tz) by at most `maxAngle` radians, keeping its speed. A zero
---velocity stays zero; a zero direction leaves the velocity unchanged.
---@param vx number
---@param vy number
---@param vz number
---@param tx number
---@param ty number
---@param tz number
---@param maxAngle number
---@return number x
---@return number y
---@return number z
function Vector.turnToward(vx, vy, vz, tx, ty, tz, maxAngle)
    local speed = sqrt(vx * vx + vy * vy + vz * vz)
    local distance = sqrt(tx * tx + ty * ty + tz * tz)
    if distance == 0 or speed == 0 then return vx, vy, vz end
    local dx, dy, dz = tx / distance, ty / distance, tz / distance
    local ux, uy, uz = vx / speed, vy / speed, vz / speed
    local dot = ux * dx + uy * dy + uz * dz
    if dot > 1 then dot = 1 elseif dot < -1 then dot = -1 end
    if acos(dot) <= maxAngle then return dx * speed, dy * speed, dz * speed end
    -- Rotate u toward d inside their common plane: w is the unit vector in that plane at a right angle to u.
    local wx, wy, wz = dx - ux * dot, dy - uy * dot, dz - uz * dot
    local w = sqrt(wx * wx + wy * wy + wz * wz)
    if w < 1e-4 then
        -- The direction is straight behind: turn left in the horizontal plane, so every machine picks the same plane.
        wx, wy, wz = -uy, ux, 0
        w = sqrt(wx * wx + wy * wy)
        if w < 1e-4 then wx, wy, w = 1, 0, 1 end -- moving straight up or down
    end
    wx, wy, wz = wx / w, wy / w, wz / w
    local c, s = cos(maxAngle), sin(maxAngle)
    return (ux * c + wx * s) * speed, (uy * c + wy * s) * speed, (uz * c + wz * s) * speed
end

---The earliest contact of the segment from f to t with a sphere, as a fraction of the segment: 0 when it starts
---inside, nil when it misses (or has no length and starts outside).
---@param fx number
---@param fy number
---@param fz number
---@param tx number
---@param ty number
---@param tz number
---@param cx number
---@param cy number
---@param cz number
---@param radius number
---@return number? fraction
function Vector.segmentSphere(fx, fy, fz, tx, ty, tz, cx, cy, cz, radius)
    local x, y, z = fx - cx, fy - cy, fz - cz
    local c = x * x + y * y + z * z - radius * radius
    if c <= 0 then return 0 end
    local dx, dy, dz = tx - fx, ty - fy, tz - fz
    local a = dx * dx + dy * dy + dz * dz
    if a == 0 then return nil end
    local b = x * dx + y * dy + z * dz
    local discriminant = b * b - a * c
    if discriminant < 0 then return nil end
    local fraction = (-b - sqrt(discriminant)) / a
    if fraction >= 0 and fraction <= 1 then return fraction end
    return nil
end

---The yaw and pitch that point an effect along a velocity. A positive pitch points the nose down (measured on
---3.0.0.24268), so a climbing velocity gives a negative pitch. A zero velocity gives 0, 0.
---@param vx number
---@param vy number
---@param vz number
---@return number yaw
---@return number pitch
function Vector.orientation(vx, vy, vz)
    local horizontal = sqrt(vx * vx + vy * vy)
    if horizontal == 0 and vz == 0 then return 0, 0 end
    return atan(vy, vx), -atan(vz, horizontal)
end

return Vector
```

`src/systems/geometry.lua`:

```lua
local Check = require('systems.internal.check')
local Vector = require('systems.internal.vector')

---Pure vector helpers for missiles and other moving things. They take and return plain numbers, so nothing is
---allocated per call. Angles are radians.
local Geometry = {}

---Raises at the public function's caller unless the first `count` values are finite numbers.
---@param operation string
---@param count integer
---@param ... unknown
local function numbers(operation, count, ...)
    for index = 1, count do
        if not Check.finite((select(index, ...))) then
            error('[systems] ' .. operation .. ': expected finite numbers', 3)
        end
    end
end

---The length of a vector.
---@param x number
---@param y number
---@param z number? Default 0.
---@return number
function Geometry.length(x, y, z)
    if z == nil then z = 0 end
    numbers('Geometry.length', 3, x, y, z)
    return Vector.length(x, y, z)
end

---Rotates a velocity toward the direction (tx, ty, tz) by at most `maxAngle` radians, keeping its speed: the homing
---primitive, called from a missile's `steer` with a turn rate times `dt`. A zero velocity stays zero; a zero direction
---leaves the velocity unchanged; a direction straight behind turns left in the horizontal plane.
---@param vx number
---@param vy number
---@param vz number
---@param tx number
---@param ty number
---@param tz number
---@param maxAngle number Not negative.
---@return number x
---@return number y
---@return number z
function Geometry.turnToward(vx, vy, vz, tx, ty, tz, maxAngle)
    numbers('Geometry.turnToward', 7, vx, vy, vz, tx, ty, tz, maxAngle)
    if maxAngle < 0 then error('[systems] Geometry.turnToward: expected a non-negative angle', 2) end
    return Vector.turnToward(vx, vy, vz, tx, ty, tz, maxAngle)
end

---The earliest contact of the segment from (fx, fy, fz) to (tx, ty, tz) with a sphere, as a fraction of the segment:
---0 when the segment starts inside it, nil when it misses.
---@param fx number
---@param fy number
---@param fz number
---@param tx number
---@param ty number
---@param tz number
---@param cx number
---@param cy number
---@param cz number
---@param radius number Not negative.
---@return number? fraction
function Geometry.segmentSphere(fx, fy, fz, tx, ty, tz, cx, cy, cz, radius)
    numbers('Geometry.segmentSphere', 10, fx, fy, fz, tx, ty, tz, cx, cy, cz, radius)
    if radius < 0 then error('[systems] Geometry.segmentSphere: expected a non-negative radius', 2) end
    return Vector.segmentSphere(fx, fy, fz, tx, ty, tz, cx, cy, cz, radius)
end

---The yaw and pitch that point an effect along a velocity, for `effect:setOrientation(yaw, pitch, 0)`. A climbing
---velocity gives a negative pitch: in Warcraft a positive pitch points the nose down. A zero velocity gives 0, 0.
---@param vx number
---@param vy number
---@param vz number
---@return number yaw
---@return number pitch
function Geometry.orientation(vx, vy, vz)
    numbers('Geometry.orientation', 3, vx, vy, vz)
    return Vector.orientation(vx, vy, vz)
end

return Geometry
```

- [ ] **Step 4:** `yue -e tests/run.lua; echo "exit $?"` → `vector: SUITE PASSED: 5 tests`,
  `geometry: SUITE PASSED: 2 tests`, `All 14 suites passed`, `exit 0`.

- [ ] **Step 5: Commit** `src/systems/internal/vector.lua src/systems/geometry.lua tests/vector.lua tests/geometry.lua
  tests/suites.lua` — `feat: systems.geometry`.

---

### Task 2: Ground sampling and `systems.terrain`

**Files:** Create `src/systems/internal/ground.lua`, `src/systems/terrain.lua`, `tests/terrain.lua`; modify
`tests/suites.lua`.

**Interfaces:** Produces, unchecked, `Ground.new(itemType?)` (a state table), `Ground.height(state, x, y)`,
`Ground.inBounds(state, x, y)`, `Ground.isWalkable(state, x, y)`, `Ground.isClear(state, x, y)` and
`Ground.dispose(state)`; and, checked, `Terrain.new(options?)`, `:height`, `:isWalkable`, `:isClear`, `:inBounds`,
`:dispose`.

- [ ] **Step 1: Failing tests** — `tests/terrain.lua`:

```lua
PATHING_TYPE_WALKABILITY = {}
-- The world: a ground function, unwalkable terrain, obstacles the terrain check misses, and items on the ground.
local ground = function() return 0 end
local unwalkable = function() return false end
local obstacle = function() return false end
local items, enumerated = {}, nil

native('Location', function(x, y) return {x = x, y = y} end)
native('MoveLocation', function(location, x, y) location.x, location.y = x, y end)
native('GetLocationZ', function(location) return ground(location.x, location.y) end)
native('RemoveLocation', function() end)
native('GetWorldBounds', function() return {minX = -1000, minY = -500, maxX = 1000, maxY = 500} end)
native('GetRectMinX', function(rect) return rect.minX end)
native('GetRectMinY', function(rect) return rect.minY end)
native('GetRectMaxX', function(rect) return rect.maxX end)
native('GetRectMaxY', function(rect) return rect.maxY end)
native('Rect', function(minX, minY, maxX, maxY) return {minX = minX, minY = minY, maxX = maxX, maxY = maxY} end)
native('SetRect', function(rect, minX, minY, maxX, maxY)
    rect.minX, rect.minY, rect.maxX, rect.maxY = minX, minY, maxX, maxY
end)
native('RemoveRect', function() end)
native('IsTerrainPathable', function(x, y) return unwalkable(x, y) end)
native('CreateItem', function(itemType, x, y)
    local item = {itemType = itemType, x = x, y = y, visible = true}
    items[#items + 1] = item
    return item
end)
-- Placing an item shows it; an obstacle, or another visible item on the spot, displaces it.
native('SetItemPosition', function(item, x, y)
    item.visible = true
    local displaced = obstacle(x, y)
    for _, other in ipairs(items) do
        if other ~= item and other.visible and math.abs(other.x - x) < 16 and math.abs(other.y - y) < 16 then
            displaced = true
        end
    end
    item.x, item.y = displaced and x + 80 or x, y
end)
native('GetItemX', function(item) return item.x end)
native('GetItemY', function(item) return item.y end)
native('SetItemVisible', function(item, flag) item.visible = flag end)
native('IsItemVisible', function(item) return item.visible end)
native('EnumItemsInRect', function(rect, _, callback)
    for _, item in ipairs(items) do
        if item.x >= rect.minX and item.x <= rect.maxX and item.y >= rect.minY and item.y <= rect.maxY then
            enumerated = item
            callback()
        end
    end
end)
native('GetEnumItem', function() return enumerated end)
native('RemoveItem', function(item) item.removed = true end)
local Terrain = require('systems.terrain')
eq(totalCalls(), 0)

local function reset()
    ground, unwalkable, obstacle = function() return 0 end, function() return false end, function() return false end
    items = {}
    resetCalls()
end

test('nothing is created until first use; height reads the ground through one location', function()
    reset()
    local terrain = Terrain.new()
    eq(totalCalls(), 0)
    ground = function(x, y) return x + y end
    eq(terrain:height(3, 4), 7); eq(terrain:height(10, 20), 30)
    eq(callCount('Location'), 1); eq(callCount('MoveLocation'), 1)
end)

test('inBounds keeps 64 units from the world edge; isWalkable adds the terrain', function()
    reset()
    local terrain = Terrain.new()
    eq(terrain:inBounds(936, 0), true); eq(terrain:inBounds(937, 0), false)
    eq(terrain:inBounds(-936, -436), true); eq(terrain:inBounds(0, -437), false); eq(terrain:inBounds(0, 437), false)
    eq(callCount('GetWorldBounds'), 1); eq(callCount('RemoveRect'), 1)
    unwalkable = function(x) return x > 100 end
    eq(terrain:isWalkable(100, 0), true); eq(terrain:isWalkable(101, 0), false)
    resetCalls()
    eq(terrain:isWalkable(2000, 0), false)
    eq(callCount('IsTerrainPathable'), 0) -- out of bounds: the terrain is not asked
    eq(callCount('GetWorldBounds'), 0)    -- the bounds were read once
end)

test('isClear sees obstacles the terrain check misses, with one hidden item', function()
    reset()
    local terrain = Terrain.new()
    obstacle = function(x) return x >= 40 and x <= 60 end
    eq(terrain:isWalkable(50, 0), true)
    eq(terrain:isClear(50, 0), false); eq(terrain:isClear(0, 0), true); eq(terrain:isClear(61, 0), true)
    eq(callCount('CreateItem'), 1); eq(callCount('Rect'), 1)
    eq(items[1].itemType, 2003790951); eq(items[1].visible, false)
    unwalkable = function() return true end
    resetCalls()
    eq(terrain:isClear(0, 0), false)
    eq(callCount('SetItemPosition'), 0) -- unwalkable terrain: no item is placed
    reset()
    Terrain.new({itemType = 1885889889}):isClear(0, 0)
    eq(items[1].itemType, 1885889889)
end)

test('isClear hides the visible items nearby for the check and shows them again', function()
    reset()
    local lying = CreateItem(1, 5, 5)
    local stowed = CreateItem(1, -5, 5)
    stowed.visible = false
    local far = CreateItem(1, 500, 0)
    local terrain = Terrain.new()
    resetCalls()
    eq(terrain:isClear(0, 0), true)
    eq(lying.visible, true); eq(stowed.visible, false); eq(far.visible, true)
    eq(callCount('SetItemVisible'), 4) -- hide the lying item, show and hide the probe, show the lying item again
    local probe = items[4]
    eq(probe.x, 0); eq(probe.y, 0); eq(probe.visible, false)
    -- Without the hiding, the lying item would displace the probe.
    eq(SetItemPosition(probe, 0, 0), nil); eq(probe.x, 80)
end)

test('dispose removes the handles once; queries then raise', function()
    reset()
    local terrain = Terrain.new()
    terrain:height(0, 0); terrain:isClear(0, 0)
    resetCalls()
    terrain:dispose(); terrain:dispose()
    eq(callCount('RemoveLocation'), 1); eq(callCount('RemoveItem'), 1); eq(callCount('RemoveRect'), 1)
    failsAt(function() terrain:height(0, 0) end, 'Terrain.height: the terrain is disposed')
    failsAt(function() terrain:isClear(0, 0) end, 'Terrain.isClear: the terrain is disposed')
    local unused = Terrain.new()
    resetCalls()
    unused:dispose()
    eq(totalCalls(), 0)
end)

test('arguments are checked at the caller', function()
    reset()
    failsAt(function() Terrain.new(5) end, 'Terrain.new: expected an options table')
    failsAt(function() Terrain.new({itemType = 'wolg'}) end, 'Terrain.new: expected an item type')
    failsAt(function() Terrain.new({itemType = 1.5}) end, 'Terrain.new: expected an item type')
    local terrain = Terrain.new()
    failsAt(function() terrain:height('1', 2) end, 'Terrain.height: expected finite coordinates')
    failsAt(function() terrain:isWalkable(1, 0 / 0) end, 'Terrain.isWalkable: expected finite coordinates')
    failsAt(function() terrain:isClear(math.huge, 0) end, 'Terrain.isClear: expected finite coordinates')
    failsAt(function() terrain:inBounds(nil, 0) end, 'Terrain.inBounds: expected finite coordinates')
    failsAt(function() Terrain.height({}, 1, 1) end, 'Terrain.height: expected Terrain')
    failsAt(function() Terrain.dispose({}) end, 'Terrain.dispose: expected Terrain')
    eq(totalCalls(), 0)
end)
```

In `tests/suites.lua`, insert `'terrain'` after `'geometry'`.

- [ ] **Step 2:** `yue -e tests/run.lua terrain` → `terrain: ERROR …module 'systems.terrain' not found`.

- [ ] **Step 3: Implement** — `src/systems/internal/ground.lua`:

```lua
---Terrain sampling without argument checks, for the per-tick loops (spec 2026-10-01 release 4 §3 and §5). A state
---table owns the handles, each created on first use. systems.terrain is the checked public version. Raw natives by
---design (Part 1 §3): there is no wrapper for a location, and these run per missile and per knockback every tick.
local Ground = {}

---@class MoonwellSystems.GroundState
---@field itemType integer
---@field location location?
---@field item item? The hidden probe item of isClear.
---@field rect rect? Moved around the point to find other items.
---@field hide (fun(): ...)? The enumeration callback that hides them.
---@field hidden item[] The items hidden for the running check.
---@field count integer
---@field minX number? The world bounds, shrunk by MARGIN; nil until first read.
---@field minY number
---@field maxX number
---@field maxY number

local WAND = 2003790951 -- 'wolg', a standard item: no object data is needed
local MARGIN = 64       -- SetUnitX outside the world bounds can crash the game: stay this far inside
local NEAR = 32         -- other items this close would displace the probe item
local TOLERANCE = 100   -- the probe item landing within 10 units (squared) counts as "stayed"

---@param itemType integer?
---@return MoonwellSystems.GroundState
function Ground.new(itemType)
    return {itemType = itemType or WAND, hidden = {}, count = 0, minY = 0, maxX = 0, maxY = 0}
end

---The ground's absolute height.
---@param state MoonwellSystems.GroundState
---@param x number
---@param y number
---@return number
function Ground.height(state, x, y)
    local location = state.location
    if location then
        MoveLocation(location, x, y)
    else
        location = Location(x, y)
        state.location = location
    end
    return GetLocationZ(location)
end

---@param state MoonwellSystems.GroundState
---@param x number
---@param y number
---@return boolean
function Ground.inBounds(state, x, y)
    local minX = state.minX
    if not minX then
        local world = GetWorldBounds()
        minX = GetRectMinX(world) + MARGIN
        state.minX, state.minY = minX, GetRectMinY(world) + MARGIN
        state.maxX, state.maxY = GetRectMaxX(world) - MARGIN, GetRectMaxY(world) - MARGIN
        RemoveRect(world)
    end
    return x >= minX and x <= state.maxX and y >= state.minY and y <= state.maxY
end

---In bounds and walkable terrain. It does not see trees or buildings (measured on 3.0.0.24268).
---@param state MoonwellSystems.GroundState
---@param x number
---@param y number
---@return boolean
function Ground.isWalkable(state, x, y)
    -- Warcraft's name is inverted: IsTerrainPathable returns true when the point is NOT pathable.
    return Ground.inBounds(state, x, y) and not IsTerrainPathable(x, y, PATHING_TYPE_WALKABILITY)
end

---isWalkable, and no tree, building or other pathing blocker: an item placed on a blocked point lands elsewhere.
---@param state MoonwellSystems.GroundState
---@param x number
---@param y number
---@return boolean
function Ground.isClear(state, x, y)
    if not Ground.isWalkable(state, x, y) then return false end
    local item, rect, hidden = state.item, state.rect, state.hidden
    if not item or not rect then
        item = CreateItem(state.itemType, x, y)
        rect = Rect(0, 0, 0, 0)
        state.item, state.rect = item, rect
        local probe = item
        state.hide = function()
            local found = GetEnumItem()
            if found ~= probe and IsItemVisible(found) then
                state.count = state.count + 1
                hidden[state.count] = found
                SetItemVisible(found, false)
            end
        end
    end
    -- Other items would displace the probe item as a tree does: hide the visible ones nearby for the check.
    SetRect(rect, x - NEAR, y - NEAR, x + NEAR, y + NEAR)
    state.count = 0
    -- Warcraft accepts a null filter; the generated JASS signature cannot express that.
    ---@diagnostic disable-next-line: param-type-mismatch
    EnumItemsInRect(rect, nil, state.hide)
    SetItemVisible(item, true)
    SetItemPosition(item, x, y)
    local dx, dy = GetItemX(item) - x, GetItemY(item) - y
    SetItemVisible(item, false)
    for index = 1, state.count do
        SetItemVisible(hidden[index], true)
        hidden[index] = nil
    end
    return dx * dx + dy * dy <= TOLERANCE
end

---Removes the owned handles. Idempotent; a later call creates them again.
---@param state MoonwellSystems.GroundState
function Ground.dispose(state)
    if state.location then RemoveLocation(state.location); state.location = nil end
    if state.item then RemoveItem(state.item); state.item = nil end
    if state.rect then RemoveRect(state.rect); state.rect = nil end
    state.hide = nil
end

return Ground
```

`src/systems/terrain.lua`:

```lua
local Check = require('systems.internal.check')
local Ground = require('systems.internal.ground')

---Terrain queries: ground height, walkability and the world bounds. A Terrain owns one location, one hidden item and
---one rect, each created on first use; dispose() removes them.
---@class MoonwellSystems.Terrain
---@field package state MoonwellSystems.GroundState? Nil once disposed.
local Terrain = {}
Terrain.__index = Terrain

---@param options {itemType: integer?}? `itemType` is the item isClear places; default 'wolg', a standard item.
---@return MoonwellSystems.Terrain
function Terrain.new(options)
    if options == nil then options = {} end
    if type(options) ~= 'table' then error('[systems] Terrain.new: expected an options table', 2) end
    local itemType = options.itemType
    if itemType ~= nil and (math.type(itemType) == nil or math.floor(itemType) ~= itemType) then
        error('[systems] Terrain.new: expected an item type', 2)
    end
    return setmetatable({state = Ground.new(itemType)}, Terrain)
end

---The live state of a query's receiver; raises at the public function's caller.
---@param self unknown
---@param operation string
---@param x unknown
---@param y unknown
---@return MoonwellSystems.GroundState
local function query(self, operation, x, y)
    local state = Check.receiver(self, Terrain, 'Terrain', operation, 1).state
    if not state then error('[systems] ' .. operation .. ': the terrain is disposed', 3) end
    if not Check.finite(x) or not Check.finite(y) then
        error('[systems] ' .. operation .. ': expected finite coordinates', 3)
    end
    return state
end

---The ground's absolute height (GetLocationZ). It follows temporary terrain deformations while they last.
---@param x number
---@param y number
---@return number
function Terrain:height(x, y)
    local state = query(self, 'Terrain.height', x, y)
    return Ground.height(state, x, y)
end

---In bounds and walkable terrain. It does not see trees or buildings: use isClear for those.
---@param x number
---@param y number
---@return boolean
function Terrain:isWalkable(x, y)
    local state = query(self, 'Terrain.isWalkable', x, y)
    return Ground.isWalkable(state, x, y)
end

---isWalkable, and no tree, building or other pathing blocker. It places a hidden item on the point and reads where it
---landed; visible items within 32 units are hidden for the check and shown again.
---@param x number
---@param y number
---@return boolean
function Terrain:isClear(x, y)
    local state = query(self, 'Terrain.isClear', x, y)
    return Ground.isClear(state, x, y)
end

---Inside the world bounds, 64 units from their edge. Moving a unit outside the world bounds can crash the game.
---@param x number
---@param y number
---@return boolean
function Terrain:inBounds(x, y)
    local state = query(self, 'Terrain.inBounds', x, y)
    return Ground.inBounds(state, x, y)
end

---Removes the owned handles. Queries afterwards raise. Idempotent.
function Terrain:dispose()
    local terrain = Check.receiver(self, Terrain, 'Terrain', 'Terrain.dispose')
    if not terrain.state then return end
    Ground.dispose(terrain.state)
    terrain.state = nil
end

return Terrain
```

- [ ] **Step 4:** `yue -e tests/run.lua; echo "exit $?"` → `terrain: SUITE PASSED: 6 tests`, `All 15 suites passed`,
  `exit 0`.

- [ ] **Step 5: Commit** `src/systems/internal/ground.lua src/systems/terrain.lua tests/terrain.lua tests/suites.lua`
  — `feat: systems.terrain`.

---

### Task 3: `systems.missile`

**Files:** Create `src/systems/missile.lua`, `tests/missile.lua`; modify `tests/suites.lua`.

**Interfaces:**
- Consumes `Vector.segmentSphere`, `Vector.orientation`, `Ground.new`, `Ground.height`, `Ground.dispose`,
  `Callback.call`, `Callback.report`, `Callback.optional`, `Check.receiver`, `Check.finite`, `Scheduler` (`every`,
  `getStep`), and the wrappers' `Effect` (`create`, `setPosition`, `setScale`, `setOrientation`, `getHandle`,
  `isDisposed`, `destroy`) and `Unit.fromHandle`.
- Produces `Missiles.new(clock, options?)`, `:launch(request)`, `:getCount()`, `:dispose()`, and `Missiles.Missile`
  with `getPosition`, `getVelocity`, `setVelocity`, `getAge`, `getTravelled`, `getHitCount`, `getEffect`, `isActive`,
  `dispose` and the `data` field.

- [ ] **Step 1: Failing tests** — `tests/missile.lua`:

```lua
-- The world: units (raw handles are tables), a ground function and the effects created.
local world, effects, queries = {}, {}, {}
local ground = function() return 0 end

native('CreateGroup', function() return {units = {}} end)
native('DestroyGroup', function() end)
native('GroupClear', function(group) group.units = {} end)
native('GroupEnumUnitsInRange', function(group, x, y, radius)
    queries[#queries + 1] = {x = x, y = y, radius = radius}
    group.units = {}
    for _, unit in ipairs(world) do
        if (unit.x - x) ^ 2 + (unit.y - y) ^ 2 <= radius ^ 2 then group.units[#group.units + 1] = unit end
    end
end)
native('BlzGroupGetSize', function(group) return #group.units end)
native('BlzGroupUnitAt', function(group, index) return group.units[index + 1] end)
native('UnitAlive', function(unit) return unit.alive end)
native('GetUnitX', function(unit) return unit.x end)
native('GetUnitY', function(unit) return unit.y end)
native('BlzGetUnitCollisionSize', function(unit) return unit.size end)
native('GetUnitFlyHeight', function(unit) return unit.fly end)
native('Location', function(x, y) return {x = x, y = y} end)
native('MoveLocation', function(location, x, y) location.x, location.y = x, y end)
native('GetLocationZ', function(location) return ground(location.x, location.y) end)
native('RemoveLocation', function() end)
native('AddSpecialEffect', function(model, x, y)
    if model == 'missing.mdl' then return nil end
    local effect = {model = model, x = x, y = y, turns = 0, destroyed = 0}
    effects[#effects + 1] = effect
    return effect
end)
native('BlzSetSpecialEffectPosition', function(effect, x, y, z) effect.x, effect.y, effect.z = x, y, z end)
native('BlzSetSpecialEffectOrientation', function(effect, yaw, pitch, roll)
    effect.yaw, effect.pitch, effect.roll = yaw, pitch, roll
    effect.turns = effect.turns + 1
end)
native('BlzSetSpecialEffectScale', function(effect, scale) effect.scale = scale end)
native('DestroyEffect', function(effect) effect.destroyed = effect.destroyed + 1 end)
local Missiles = require('systems.missile')
local Scheduler = require('systems.scheduler')
local Geometry = require('systems.geometry')
local Effect = require('wrappers.effect')
eq(totalCalls(), 0)

-- Every test starts with setup(): an empty flat world, a clock with the given step, and a system whose targets have
-- their centre at their feet and a radius cap of 16, so coordinates read as in the tests' comments.
local function setup(step, options)
    world, effects, queries = {}, {}, {}
    ground = function() return 0 end
    local clock = Scheduler.new(step or 1)
    local merged = {targetOffset = 0, maxTargetRadius = 16}
    for key, value in pairs(options or {}) do merged[key] = value end
    resetCalls()
    return Missiles.new(clock, merged), clock
end
-- Adds a unit of collision size 1 to the world, after the ones already there.
local function target(name, x, y, fields)
    local unit = {name = name, x = x, y = y or 0, alive = true, size = 1, fly = 0}
    for key, value in pairs(fields or {}) do unit[key] = value end
    world[#world + 1] = unit
    return unit
end
-- A missile from the origin at ground level, flying +x at 100 per second, with radius 1.
local function shot(fields)
    local request = {x = 0, y = 0, height = 0, vx = 100, vy = 0, radius = 1, lifetime = 10}
    for key, value in pairs(fields or {}) do request[key] = value end
    return request
end
local function join(list) return table.concat(list, ',') end
local function recorder(hits) return function(_, unit) hits[#hits + 1] = unit.handle.name end end

test('a swept missile hits fast crossings by distance, then in enumeration order', function()
    local system, clock = setup()
    target('c', 80); target('b', 30); target('a', 30)
    local hits = {}
    local missile = system:launch(shot({maxHits = 3, onHit = recorder(hits)}))
    clock:advance()
    eq(join(hits), 'b,a,c'); eq(missile:isActive(), false); eq(missile:getHitCount(), 3)
    local x, y, z = missile:getPosition()
    eq(x, 78.0); eq(y, 0.0); eq(z, 0.0)
    eq(#PRINTED, 0)
end)

test('ties keep the enumeration order, whatever the handles are', function()
    local system, clock = setup()
    target('9', 50); target('4', 50); target('7', 50)
    local hits = {}
    system:launch(shot({maxHits = 3, onHit = recorder(hits)}))
    clock:advance()
    eq(join(hits), '9,4,7')
end)

test('the sphere test uses height, and a unit is hit once', function()
    local system, clock = setup()
    target('low', 0); target('high', 0, 0, {fly = 5})
    local hits = {}
    system:launch(shot({vx = 0, maxHits = 10, onHit = recorder(hits)}))
    clock:advance(); clock:advance()
    eq(join(hits), 'low')
end)

test('tangent contacts count; units behind the travel do not', function()
    local system, clock = setup()
    target('beside', 50, 2); target('behind', -10)
    local hits = {}
    local missile = system:launch(shot({onHit = recorder(hits)}))
    clock:advance()
    eq(join(hits), 'beside')
    local x, y = missile:getPosition()
    eq(x, 50.0); eq(y, 0.0)
end)

test('lifetime and range clip the sweep, and a missile that stands still expires', function()
    local system, clock = setup()
    target('far', 40)
    local hits, ends = {}, {}
    local onEnd = function(missile, reason) ends[#ends + 1] = reason end
    local short = system:launch(shot({lifetime = 0.25, onHit = recorder(hits), onEnd = onEnd}))
    local ranged = system:launch(shot({maxRange = 20, onHit = recorder(hits), onEnd = onEnd}))
    local still = system:launch(shot({vx = 0, lifetime = 0.1, onEnd = onEnd}))
    clock:advance()
    eq(join(hits), ''); eq(join(ends), 'expired,range,expired')
    eq((short:getPosition()), 25.0); eq(short:getAge(), 0.25); eq(short:getTravelled(), 25.0)
    eq((ranged:getPosition()), 20.0); eq(ranged:getTravelled(), 20); eq(ranged:getAge(), 0.2)
    eq(still:isActive(), false); eq(system:getCount(), 0)
    -- The range ends a missile before a lifetime that would end it later in the same step.
    local both = system:launch(shot({lifetime = 0.5, maxRange = 20, onEnd = onEnd}))
    clock:advance()
    eq(ends[4], 'range'); eq(both:getAge(), 0.2)
    -- When both end it at the same moment, the lifetime is named.
    system:launch(shot({lifetime = 0.2, maxRange = 20, onEnd = onEnd}))
    clock:advance()
    eq(ends[5], 'expired')
    -- The distance flown ends at exactly maxRange, where adding up the steps would miss it.
    system, clock = setup(0.25)
    local exact = system:launch(shot({vx = 700, maxRange = 90}))
    clock:advance()
    eq(exact:isActive(), false); eq(exact:getTravelled(), 90)
end)

test('a missile launched in a callback waits for the next step; a unit killed by a hit is skipped', function()
    local system, clock = setup()
    target('first', 10)
    local second = target('second', 20)
    local hits, child = {}, nil
    system:launch(shot({maxHits = 3, onHit = function(_, unit)
        hits[#hits + 1] = unit.handle.name
        second.alive = false
        child = system:launch(shot())
    end}))
    clock:advance()
    eq(join(hits), 'first'); eq((child:getPosition()), 0); eq(child:isActive(), true)
    clock:advance()
    eq(child:isActive(), false); eq(child:getHitCount(), 1)
end)

test('filter decides who is hit, and a refused unit is asked again', function()
    local system, clock = setup()
    target('ally', 0, 0, {team = 'ally'}); target('enemy', 0, 0, {team = 'enemy'})
    local hits, asked = {}, {}
    local missile = system:launch(shot({vx = 0, maxHits = 5, onHit = recorder(hits), filter = function(unit, missile)
        asked[#asked + 1] = unit.handle.name
        eq(missile:isActive(), true)
        return unit.handle.team == 'enemy'
    end}))
    clock:advance(); clock:advance()
    eq(join(hits), 'enemy'); eq(join(asked), 'ally,enemy,ally'); eq(missile:getHitCount(), 1)
    -- A filter that disposes the missile ends the step: nothing is hit.
    local quitter = system:launch(shot({vx = 0, onHit = recorder(hits), filter = function(_, missile)
        missile:dispose()
        return true
    end}))
    clock:advance()
    eq(join(hits), 'enemy'); eq(quitter:getHitCount(), 0); eq(#PRINTED, 0)
end)

test('a target larger than maxTargetRadius is hit as if it had that radius', function()
    local system, clock = setup()
    target('giant', 40, 30, {size = 500})
    local hits = {}
    system:launch(shot({onHit = recorder(hits)}))
    clock:advance()
    eq(join(hits), '') -- 30 away from the path: beyond 1 + 16
    system, clock = setup()
    target('giant', 40, 15, {size = 500})
    local missile = system:launch(shot({onHit = recorder(hits)}))
    clock:advance()
    eq(join(hits), 'giant'); eq((missile:getPosition()), 32.0) -- 17 from the centre, not at the start
end)

test('heights are above the ground: the start, the targets and the landing follow the terrain', function()
    local system, clock = setup(0.25)
    ground = function(x) return x >= 50 and 100 or 0 end
    -- Launched on the hill at 240 above it: level with the gryphon's fly height, far above the footman.
    target('footman', 100); target('gryphon', 100, 0, {fly = 240})
    local hits = {}
    local high = system:launch(shot({x = 60, height = 240, onHit = recorder(hits)}))
    eq(select(3, high:getPosition()), 340)
    -- Launched on the low ground at 60: the hill's side is in its way.
    local reason
    local low = system:launch(shot({height = 60, onEnd = function(_, why) reason = why end}))
    clock:advance(); clock:advance()
    eq(join(hits), 'gryphon')
    eq(reason, 'ground')
    local x, _, z = low:getPosition()
    eq(x, 50.0); eq(z, 100)
end)

test('terrain = false: the ground is flat at 0 and no terrain is sampled', function()
    local system, clock = setup(1, {terrain = false})
    ground = function() return 1000 end
    target('gryphon', 50, 0, {fly = 240})
    local hits = {}
    local missile = system:launch(shot({height = 240, onHit = recorder(hits)}))
    eq(select(3, missile:getPosition()), 240)
    clock:advance()
    eq(join(hits), 'gryphon'); eq(callCount('Location'), 0); eq(callCount('GetLocationZ'), 0)
    local reason
    system:launch(shot({height = 10, vz = -100, onEnd = function(_, why) reason = why end}))
    clock:advance()
    eq(reason, 'ground')
end)

test('gravity arcs a missile into the ground', function()
    local system, clock = setup(0.1)
    local reason
    local missile = system:launch(shot({vz = 100, az = -200, onEnd = function(_, why) reason = why end}))
    for _ = 1, 20 do
        if missile:isActive() then clock:advance() end
    end
    eq(reason, 'ground')
    local x, _, z = missile:getPosition()
    eq(z, 0); assert(x > 90 and x < 110, 'landed at ' .. x)
end)

test('followGround keeps the height over a rise and never lands', function()
    local system, clock = setup(0.25)
    ground = function(x) return x >= 50 and 100 or 0 end
    local missile = system:launch(shot({height = 60, followGround = true, model = 'bolt.mdl'}))
    clock:advance()
    eq(select(3, missile:getPosition()), 60)
    clock:advance()
    local x, _, z = missile:getPosition()
    eq(x, 50.0); eq(z, 160); eq(missile:isActive(), true)
    assert(effects[1].pitch < 0, 'the bolt noses up the rise: ' .. tostring(effects[1].pitch))
    failsAt(function() missile:setVelocity(100, 0, 5) end,
        'Missile.setVelocity: a followGround missile has no vertical velocity')
    failsAt(function() system:launch(shot({followGround = true, vz = 1})) end,
        'Missiles.launch: expected a missile request: followGround')
    failsAt(function() system:launch(shot({followGround = true, az = -1})) end,
        'Missiles.launch: expected a missile request: followGround')
end)

test('steering with turnToward homes on a target beside the path', function()
    local system, clock = setup(1 / 32)
    target('goal', 0, 500)
    local hit = false
    local missile = system:launch(shot({vx = 300, lifetime = 20, onHit = function() hit = true end,
        steer = function(missile, dt)
            local x, y, z = missile:getPosition()
            local vx, vy, vz = missile:getVelocity()
            missile:setVelocity(Geometry.turnToward(vx, vy, vz, 0 - x, 500 - y, 0 - z, math.pi * dt))
        end}))
    for _ = 1, 400 do
        if missile:isActive() then clock:advance() end
    end
    eq(hit, true); eq(#PRINTED, 0)
end)

test('a model becomes an effect that is placed, scaled, turned along the travel and destroyed once', function()
    local system, clock = setup()
    ground = function() return 10 end
    local missile = system:launch(shot({vx = 0, vy = 100, height = 60, model = 'bolt.mdl', scale = 2, lifetime = 2}))
    local effect = effects[1]
    expectCall('AddSpecialEffect', 'bolt.mdl', 0, 0)
    eq(effect.z, 70); eq(effect.scale, 2); eq(effect.yaw, math.pi / 2); eq(effect.pitch, 0); eq(effect.turns, 1)
    eq(missile:getEffect().handle, effect)
    clock:advance()
    eq(effect.y, 100.0); eq(effect.z, 70); eq(effect.turns, 1) -- the same direction: not turned again
    missile:setVelocity(100, 0, 0)
    clock:advance()
    eq(effect.yaw, 0); eq(effect.turns, 2); eq(effect.destroyed, 1); eq(missile:isActive(), false)
    missile:dispose(); system:dispose()
    eq(effect.destroyed, 1); eq(callCount('DestroyEffect'), 1)
end)

test('an Effect is taken over; face = false leaves its orientation alone', function()
    local system, clock = setup()
    local own = Effect.create('own.mdl', 5, 5)
    local missile = system:launch(shot({effect = own, face = false, lifetime = 1}))
    eq(missile:getEffect(), own); eq(effects[1].x, 0); eq(effects[1].turns, 0)
    clock:advance()
    eq(effects[1].x, 100.0); eq(effects[1].turns, 0); eq(own:isDisposed(), true); eq(effects[1].destroyed, 1)
    -- An effect its owner destroyed first is not destroyed again.
    local early = Effect.create('own.mdl', 0, 0)
    local second = system:launch(shot({effect = early}))
    early:destroy()
    second:dispose()
    eq(effects[2].destroyed, 1); eq(#PRINTED, 0)
end)

test('the query covers the segment and the radii, with one reused group', function()
    local system, clock = setup()
    target('enumerated, but out of reach', 50, 60)
    system:launch(shot({radius = 2, lifetime = 2}))
    clock:advance()
    eq(queries[1].x, 50.0); eq(queries[1].y, 0.0); eq(queries[1].radius, 68.0)
    -- The unit fails the flat test, so nothing more is read from it.
    eq(callCount('GetUnitX'), 1); eq(callCount('BlzGetUnitCollisionSize'), 0); eq(callCount('UnitAlive'), 0)
    clock:advance()
    eq(callCount('CreateGroup'), 1); eq(callCount('GroupClear'), 2)
    system:dispose(); system:dispose()
    eq(callCount('DestroyGroup'), 1)
end)

test('a hit callback may dispose the missile: the dispatch stops and the effect is destroyed once', function()
    local system, clock = setup()
    target('first', 10); target('second', 20)
    local hits = {}
    local missile = system:launch(shot({maxHits = 10, model = 'bolt.mdl', onHit = function(missile, unit)
        hits[#hits + 1] = unit.handle.name
        missile:dispose()
    end}))
    clock:advance()
    missile:dispose(); system:dispose()
    eq(join(hits), 'first'); eq(effects[1].destroyed, 1)
end)

test('a missile ended by its own steer, or by another missile, does not move', function()
    local system, clock = setup()
    target('unit', 10)
    local moved = false
    local later
    local first = system:launch(shot({onHit = function() later:dispose() end}))
    later = system:launch(shot({y = 500, steer = function() moved = true end}))
    local quitter = system:launch(shot({y = 900, steer = function(missile) missile:dispose() end}))
    clock:advance()
    eq(first:isActive(), false); eq(moved, false); eq((later:getPosition()), 0)
    eq((quitter:getPosition()), 0); eq(quitter:getAge(), 0); eq(system:getCount(), 0)
end)

test('a failing callback ends that missile with error; the others still move', function()
    local messages = {}
    local system, clock = setup(0.5, {onError = function(message) messages[#messages + 1] = message end})
    target('unit', 10)
    local ends = {}
    local onEnd = function(_, reason) ends[#ends + 1] = reason end
    system:launch(shot({steer = function() error('bad steer', 0) end, onEnd = onEnd}))
    local hitter = system:launch(shot({model = 'bolt.mdl', onEnd = onEnd,
        onHit = function() error('hit failed', 0) end}))
    system:launch(shot({y = 100, filter = function() error('bad filter', 0) end, onEnd = onEnd}))
    target('other', 10, 100)
    local healthy = system:launch(shot({y = 500}))
    clock:advance()
    eq(join(ends), 'error,error,error'); eq(join(messages), 'bad steer,hit failed,bad filter')
    eq(hitter:isActive(), false); eq(effects[1].destroyed, 1)
    eq((healthy:getPosition()), 50.0); eq(system:getCount(), 1)
    -- A failing onEnd is reported and changes nothing else.
    system:launch(shot({lifetime = 0.5, y = 900, onEnd = function() error('end failed', 0) end}))
    clock:advance()
    eq(messages[4], 'end failed'); eq(#PRINTED, 0)
    system, clock = setup()
    system:launch(shot({steer = function() error('printed steer', 0) end}))
    clock:advance()
    eq(PRINTED[1], '[systems] Missile callback failed: printed steer')
end)

test('each missile reports exactly one end reason', function()
    local system, clock = setup()
    target('unit', 50)
    local ends = {}
    local onEnd = function(_, reason) ends[#ends + 1] = reason end
    system:launch(shot({onEnd = onEnd}))
    system:launch(shot({vx = 0, vy = 100, lifetime = 0.5, onEnd = onEnd}))
    system:launch(shot({vx = 0, vy = 100, maxRange = 20, onEnd = onEnd}))
    system:launch(shot({vx = 0, vy = 100, onEnd = onEnd})):dispose()
    clock:advance()
    system:launch(shot({vx = 0, vy = 100, onEnd = onEnd}))
    system:dispose()
    eq(join(ends), 'cancelled,hit-limit,expired,range,disposed')
end)

test('the system ticks only while missiles fly', function()
    local system, clock = setup()
    eq(clock:getPending(), 0)
    local first = system:launch(shot({lifetime = 1}))
    system:launch(shot({lifetime = 2}))
    eq(clock:getPending(), 1); eq(system:getCount(), 2)
    clock:advance()
    eq(first:isActive(), false); eq(clock:getPending(), 1); eq(system:getCount(), 1)
    clock:advance()
    eq(clock:getPending(), 0); eq(system:getCount(), 0)
    local third = system:launch(shot())
    eq(clock:getPending(), 1)
    third:dispose()
    eq(clock:getPending(), 0)
end)

test('dispose inside a callback ends every missile and removes the handles', function()
    local system, clock = setup()
    target('unit', 10)
    local ends = {}
    local onEnd = function(_, reason) ends[#ends + 1] = reason end
    system:launch(shot({onHit = function() system:dispose() end, onEnd = onEnd}))
    system:launch(shot({y = 500, onEnd = onEnd}))
    clock:advance()
    eq(join(ends), 'disposed,disposed'); eq(system:getCount(), 0); eq(clock:getPending(), 0)
    eq(callCount('DestroyGroup'), 1); eq(callCount('RemoveLocation'), 1)
    failsAt(function() system:launch(shot()) end, 'Missiles.launch: the system is disposed')
    eq(#PRINTED, 0)
end)

test('launch checks its request at the caller, before any effect is created', function()
    local system = setup()
    failsAt(function() system:launch(5) end, 'Missiles.launch: expected a missile request table')
    local own = Effect.create('own.mdl', 0, 0)
    local gone = Effect.create('own.mdl', 0, 0)
    gone:destroy()
    resetCalls()
    local cases = {
        {'x', {x = '0'}}, {'y', {y = 0 / 0}}, {'vx', {vx = math.huge}}, {'vy', {vy = false}}, {'radius', {radius = -1}},
        {'lifetime', {lifetime = 0}}, {'height', {height = 'high'}}, {'vz', {vz = {}}}, {'az', {az = '1'}},
        {'maxRange', {maxRange = 0}}, {'maxHits', {maxHits = 0}}, {'maxHits', {maxHits = 1.5}},
        {'velocity', {vx = 1e200}}, {'acceleration', {ax = 1e200}}, {'followGround', {followGround = 1}},
        {'face', {face = 'yes'}}, {'model', {model = ''}}, {'model', {model = 5}}, {'effect', {effect = {}}},
        {'effect', {effect = gone}}, {'model and effect', {model = 'bolt.mdl', effect = own}},
        {'scale', {scale = 2}}, {'scale', {scale = 2, effect = own}}, {'filter', {filter = 5}},
        {'steer', {steer = 'x'}}, {'onHit', {onHit = {}}}, {'onEnd', {onEnd = 1}},
        {'lifetime', {lifetime = -1, model = 'bolt.mdl'}}, {'onEnd', {onEnd = 1, model = 'bolt.mdl', scale = 2}},
    }
    for _, case in ipairs(cases) do
        failsAt(function() system:launch(shot(case[2])) end,
            'Missiles.launch: expected a missile request: ' .. case[1])
    end
    eq(callCount('AddSpecialEffect'), 0); eq(system:getCount(), 0)
    failsAt(function() system:launch(shot({model = 'missing.mdl'})) end,
        'Missiles.launch: [wrappers] Effect.create: native returned nil')
    eq(system:getCount(), 0)
    failsAt(function() Missiles.launch({}, shot()) end, 'Missiles.launch: expected Missiles')
end)

test('launch copies the request, and a missile carries its data', function()
    local system, clock = setup()
    local request = shot({data = {damage = 5}, lifetime = 1})
    local missile = system:launch(request)
    request.vx, request.lifetime = 999, 99
    clock:advance()
    eq((missile:getPosition()), 100.0); eq(missile:isActive(), false); eq(missile.data.damage, 5)
    eq(missile:getAge(), 1); eq(missile:getTravelled(), 100.0)
end)

test('new and the missile methods check their arguments at the caller', function()
    local clock = Scheduler.new(1)
    failsAt(function() Missiles.new({}) end, 'Missiles.new: expected Scheduler')
    failsAt(function() Missiles.new(clock, 5) end, 'Missiles.new: expected an options table')
    failsAt(function() Missiles.new(clock, {onError = 5}) end, 'Missiles.new: expected a callback function')
    for _, case in ipairs({{'terrain', 'yes'}, {'targetOffset', '50'}, {'maxTargetRadius', -1}}) do
        failsAt(function() Missiles.new(clock, {[case[1]] = case[2]}) end,
            'Missiles.new: expected missile options: ' .. case[1])
    end
    local system = setup()
    local missile = system:launch(shot())
    failsAt(function() missile:setVelocity(1, 2) end, 'Missile.setVelocity: expected a finite velocity')
    failsAt(function() missile:setVelocity(1e200, 0, 0) end, 'Missile.setVelocity: expected a finite velocity')
    failsAt(function() missile.getPosition({}) end, 'Missile.getPosition: expected Missile')
    failsAt(function() missile.dispose({}) end, 'Missile.dispose: expected Missile')
    failsAt(function() Missiles.getCount({}) end, 'Missiles.getCount: expected Missiles')
    failsAt(function() Missiles.dispose({}) end, 'Missiles.dispose: expected Missiles')
end)
```

In `tests/suites.lua`, insert `'missile'` after `'terrain'`.

- [ ] **Step 2:** `yue -e tests/run.lua missile` → `missile: ERROR …module 'systems.missile' not found`.

- [ ] **Step 3: Implement** — `src/systems/missile.lua`:

```lua
local Callback = require('systems.internal.callback')
local Check = require('systems.internal.check')
local Ground = require('systems.internal.ground')
local Vector = require('systems.internal.vector')
local Scheduler = require('systems.scheduler')
local Effect = require('wrappers.effect')
local Unit = require('wrappers.unit')

local sqrt = math.sqrt
local segmentSphere, orientation = Vector.segmentSphere, Vector.orientation
local height = Ground.height

---Missiles with swept collision: each step tests the whole segment a missile travels, so a fast missile never jumps
---over a unit. The system ticks from its scheduler while missiles are in flight. The per-tick loop calls raw natives
---on the handles it owns or has just enumerated (spec Part 1 §3), and wraps a unit only for a callback.
---@class MoonwellSystems.Missiles
---@field package clock MoonwellSystems.Scheduler
---@field package onError (fun(message: string): ...)?
---@field package ground MoonwellSystems.GroundState? Nil with `terrain = false`: the ground is then the plane z = 0.
---@field package targetOffset number
---@field package maxTargetRadius number
---@field package list MoonwellSystems.Missile[] In launch order; ended missiles leave after the step.
---@field package count integer Missiles in flight.
---@field package ended boolean Whether `list` holds ended missiles.
---@field package ticking boolean
---@field package cancel (fun())? Cancels the scheduler task; nil while nothing flies.
---@field package group group? One reused group for every query.
---@field package units unit[] The contacts of the missile being advanced, by fraction.
---@field package fractions number[]
---@field package disposed boolean
local Missiles = {}
Missiles.__index = Missiles

---@alias MoonwellSystems.MissileEnd 'hit-limit'|'expired'|'range'|'ground'|'cancelled'|'disposed'|'error'

---@class MoonwellSystems.MissileOptions
---@field onError (fun(message: string): ...)? Receives callback failures; default prints them.
---@field terrain boolean? Sample the ground under missiles and targets. Default true; false means flat ground at 0.
---@field targetOffset number? A target's centre above its feet. Default 50.
---@field maxTargetRadius number? The largest collision size hit exactly; larger targets are capped. Default 128.

---@class MoonwellSystems.MissileRequest
---@field x number
---@field y number
---@field height number? Start height above the ground. Default 60.
---@field vx number Units per second.
---@field vy number
---@field vz number? Default 0.
---@field ax number? Constant acceleration; default 0.
---@field ay number?
---@field az number? For example -1400 for an arc.
---@field radius number Not negative.
---@field lifetime number Positive seconds.
---@field maxRange number? Positive distance; the missile then ends with 'range'.
---@field maxHits integer? Default 1; more than 1 pierces.
---@field followGround boolean? Keep `height` above the ground; vz and az must then be 0.
---@field model string? An effect model; the missile creates and owns the effect.
---@field effect MoonwellWrappers.Effect? An effect to take over instead of `model`.
---@field scale number? The created effect's scale; only with `model`.
---@field face boolean? Turn the effect along its travel. Default true.
---@field filter (fun(unit: MoonwellWrappers.Unit, missile: MoonwellSystems.Missile): ...)? Default: any living unit.
---@field steer (fun(missile: MoonwellSystems.Missile, dt: number): ...)? Runs first in every step.
---@field onHit (fun(missile: MoonwellSystems.Missile, unit: MoonwellWrappers.Unit): ...)?
---@field onEnd (fun(missile: MoonwellSystems.Missile, reason: MoonwellSystems.MissileEnd): ...)?
---@field data any Kept as `missile.data`.

---A missile in flight, returned by launch().
---@class MoonwellSystems.Missile
---@field data any The request's `data`.
---@field package system MoonwellSystems.Missiles
---@field package active boolean
---@field package x number
---@field package y number
---@field package z number Absolute.
---@field package vx number
---@field package vy number
---@field package vz number
---@field package ax number
---@field package ay number
---@field package az number
---@field package radius number
---@field package lifetime number
---@field package maxRange number?
---@field package maxHits integer
---@field package followGround boolean
---@field package height number
---@field package effect MoonwellWrappers.Effect?
---@field package raw effect? The effect's handle, for the loop.
---@field package face boolean
---@field package yaw number The orientation last applied.
---@field package pitch number
---@field package filter function?
---@field package steer function?
---@field package onHit function?
---@field package onEnd function?
---@field package age number
---@field package travelled number
---@field package hits table<unit, true> Looked up, never iterated.
---@field package hitCount integer
local Missile = {}
Missile.__index = Missile
Missiles.Missile = Missile

local NUMBERS = {'x', 'y', 'vx', 'vy', 'radius', 'lifetime'}
local OPTIONAL = {'height', 'vz', 'ax', 'ay', 'az', 'maxRange', 'scale'}
local CALLBACKS = {'filter', 'steer', 'onHit', 'onEnd'}

-- Ending

---@param system MoonwellSystems.Missiles
local function stop(system)
    local cancel = system.cancel
    if cancel then
        system.cancel = nil
        cancel()
    end
end

---Ends a missile once: releases its place, destroys its effect, then runs onEnd.
---@param missile MoonwellSystems.Missile
---@param reason MoonwellSystems.MissileEnd
local function finish(missile, reason)
    if not missile.active then return end
    missile.active = false
    local system = missile.system
    system.count = system.count - 1
    system.ended = true
    missile.hits = {}
    local effect = missile.effect
    if effect then effect:destroy() end -- does nothing if its owner destroyed it first
    missile.raw = nil
    local onEnd = missile.onEnd
    if onEnd then Callback.call('Missile end', system.onError, onEnd, missile, reason) end
    if system.count == 0 and not system.ticking then stop(system) end
end

-- A step

---Moves the missile's effect to its position and, when asked, turns it along the travel (dx, dy, dz).
---@param missile MoonwellSystems.Missile
---@param dx number
---@param dy number
---@param dz number
local function show(missile, dx, dy, dz)
    local raw = missile.raw
    if not raw then return end
    BlzSetSpecialEffectPosition(raw, missile.x, missile.y, missile.z)
    if missile.face and (dx ~= 0 or dy ~= 0 or dz ~= 0) then
        local yaw, pitch = orientation(dx, dy, dz)
        if yaw ~= missile.yaw or pitch ~= missile.pitch then
            missile.yaw, missile.pitch = yaw, pitch
            BlzSetSpecialEffectOrientation(raw, yaw, pitch, 0)
        end
    end
end

---@param system MoonwellSystems.Missiles
---@param missile MoonwellSystems.Missile
---@param dt number
local function advance(system, missile, dt)
    local onError = system.onError
    local remaining = missile.lifetime - missile.age
    local expiring = remaining <= dt
    local seconds = expiring and remaining or dt
    local steer = missile.steer
    if steer then
        if not Callback.call('Missile callback', onError, steer, missile, seconds) then
            finish(missile, 'error')
            return
        end
        if not missile.active then return end
    end
    -- Semi-implicit Euler: velocity first, then position. Stable, and the same on every machine.
    local vx, vy, vz = missile.vx + missile.ax * seconds, missile.vy + missile.ay * seconds,
        missile.vz + missile.az * seconds
    missile.vx, missile.vy, missile.vz = vx, vy, vz
    local speed = sqrt(vx * vx + vy * vy + vz * vz)
    local maxRange, ranging = missile.maxRange, false
    if maxRange and speed > 0 then
        local left = (maxRange - missile.travelled) / speed
        if left <= seconds then
            ranging = true
            if left < seconds then seconds, expiring = left, false end
        end
    end
    local fx, fy, fz = missile.x, missile.y, missile.z
    local tx, ty, tz = fx + vx * seconds, fy + vy * seconds, fz + vz * seconds
    local ground, follow = system.ground, missile.followGround
    if follow then tz = (ground and height(ground, tx, ty) or 0) + missile.height end

    -- Candidates: the units within reach of the segment, in the engine's enumeration order.
    local group = system.group
    if not group then
        group = CreateGroup()
        system.group = group
    end
    local radius, cap = missile.radius, system.maxTargetRadius
    local dx, dy = tx - fx, ty - fy
    -- Warcraft accepts a null filter; the generated JASS signature cannot express that.
    ---@diagnostic disable-next-line: param-type-mismatch
    GroupEnumUnitsInRange(group, (fx + tx) / 2, (fy + ty) / 2, sqrt(dx * dx + dy * dy) / 2 + radius + cap, nil)
    local units, fractions, hits, offset = system.units, system.fractions, missile.hits, system.targetOffset
    local contacts = 0
    for index = 0, BlzGroupGetSize(group) - 1 do
        local unit = BlzGroupUnitAt(group, index)
        if not hits[unit] then
            local ux, uy = GetUnitX(unit), GetUnitY(unit)
            -- A flat test with the largest radius first: most units fail it, and need no further native.
            if segmentSphere(fx, fy, 0, tx, ty, 0, ux, uy, 0, radius + cap) then
                local size = BlzGetUnitCollisionSize(unit)
                if size > cap then size = cap end
                local uz = (ground and height(ground, ux, uy) or 0) + GetUnitFlyHeight(unit) + offset
                local fraction = segmentSphere(fx, fy, fz, tx, ty, tz, ux, uy, uz, radius + size)
                if fraction then
                    -- Insertion sort by fraction; a tie keeps the enumeration order.
                    local place = contacts
                    while place > 0 and fractions[place] > fraction do
                        units[place + 1], fractions[place + 1] = units[place], fractions[place]
                        place = place - 1
                    end
                    units[place + 1], fractions[place + 1] = unit, fraction
                    contacts = contacts + 1
                end
            end
        end
    end
    GroupClear(group)

    local filter, onHit = missile.filter, missile.onHit
    for index = 1, contacts do
        local unit = units[index]
        -- Dead units are enumerated too, and an earlier hit of this step may have killed this one.
        if UnitAlive(unit) then
            local target = Unit.fromHandle(unit) --[[@as MoonwellWrappers.Unit]]
            local allowed = true
            if filter then
                local ok, result = pcall(filter, target, missile)
                if not ok then
                    Callback.report('Missile callback', onError, result)
                    finish(missile, 'error')
                    return
                end
                if not missile.active then return end
                allowed = result and true or false
            end
            if allowed then
                hits[unit] = true
                missile.hitCount = missile.hitCount + 1
                local fraction = fractions[index]
                missile.x, missile.y, missile.z = fx + dx * fraction, fy + dy * fraction, fz + (tz - fz) * fraction
                show(missile, dx, dy, tz - fz)
                if onHit and not Callback.call('Missile callback', onError, onHit, missile, target) then
                    finish(missile, 'error')
                    return
                end
                if not missile.active then return end
                if missile.hitCount >= missile.maxHits then
                    finish(missile, 'hit-limit')
                    return
                end
            end
        end
    end

    -- The ground is checked at the step's end only, so a hit earlier in the same step still counts.
    local landed = false
    if not follow then
        local surface = ground and height(ground, tx, ty) or 0
        if tz < surface then tz, landed = surface, true end
    end
    missile.x, missile.y, missile.z = tx, ty, tz
    missile.age = missile.age + seconds
    missile.travelled = ranging and maxRange or missile.travelled + speed * seconds
    show(missile, dx, dy, tz - fz)
    if landed then
        finish(missile, 'ground')
    elseif expiring then
        finish(missile, 'expired')
    elseif ranging then
        finish(missile, 'range')
    end
end

---One scheduler step: advances the missiles that were in flight when it began, in launch order.
---@param system MoonwellSystems.Missiles
local function tick(system)
    local list, dt = system.list, system.clock:getStep()
    system.ticking = true
    for index = 1, #list do
        local missile = list[index]
        if missile.active then advance(system, missile, dt) end
    end
    system.ticking = false
    if system.ended then
        local kept = {}
        for _, missile in ipairs(system.list) do
            if missile.active then kept[#kept + 1] = missile end
        end
        system.list, system.ended = kept, false
    end
    if system.count == 0 then stop(system) end
end

-- Missiles

---@param clock MoonwellSystems.Scheduler Drives the missiles; they move once per scheduler step.
---@param options MoonwellSystems.MissileOptions?
---@return MoonwellSystems.Missiles
function Missiles.new(clock, options)
    Check.receiver(clock, Scheduler, 'Scheduler', 'Missiles.new')
    if options == nil then options = {} end
    if type(options) ~= 'table' then error('[systems] Missiles.new: expected an options table', 2) end
    Callback.optional(options.onError, 'Missiles.new')
    local terrain, offset, cap = options.terrain, options.targetOffset, options.maxTargetRadius
    if terrain ~= nil and type(terrain) ~= 'boolean' then
        error('[systems] Missiles.new: expected missile options: terrain', 2)
    end
    if offset == nil then offset = 50 end
    if not Check.finite(offset) then error('[systems] Missiles.new: expected missile options: targetOffset', 2) end
    if cap == nil then cap = 128 end
    if not Check.finite(cap) or cap < 0 then
        error('[systems] Missiles.new: expected missile options: maxTargetRadius', 2)
    end
    return setmetatable({
        clock = clock, onError = options.onError, ground = terrain ~= false and Ground.new() or nil,
        targetOffset = offset, maxTargetRadius = cap, list = {}, count = 0, ended = false, ticking = false,
        units = {}, fractions = {}, disposed = false,
    }, Missiles)
end

---@param request table
---@return string? field The first invalid field, or nil.
local function invalid(request)
    for _, name in ipairs(NUMBERS) do
        if not Check.finite(request[name]) then return name end
    end
    for _, name in ipairs(OPTIONAL) do
        if request[name] ~= nil and not Check.finite(request[name]) then return name end
    end
    if request.radius < 0 then return 'radius' end
    if request.lifetime <= 0 then return 'lifetime' end
    if request.maxRange ~= nil and request.maxRange <= 0 then return 'maxRange' end
    local maxHits = request.maxHits
    if maxHits ~= nil and (math.type(maxHits) == nil or math.floor(maxHits) ~= maxHits or maxHits < 1) then
        return 'maxHits'
    end
    local vz, ax, ay, az = request.vz or 0, request.ax or 0, request.ay or 0, request.az or 0
    if not Check.finite(request.vx * request.vx + request.vy * request.vy + vz * vz) then return 'velocity' end
    if not Check.finite(ax * ax + ay * ay + az * az) then return 'acceleration' end
    for _, name in ipairs({'followGround', 'face'}) do
        if request[name] ~= nil and type(request[name]) ~= 'boolean' then return name end
    end
    if request.followGround and (vz ~= 0 or az ~= 0) then return 'followGround' end
    local model, effect = request.model, request.effect
    if model ~= nil and (type(model) ~= 'string' or model == '') then return 'model' end
    if effect ~= nil and (getmetatable(effect) ~= Effect or effect:isDisposed()) then return 'effect' end
    if model ~= nil and effect ~= nil then return 'model and effect' end
    if request.scale ~= nil and model == nil then return 'scale' end
    for _, name in ipairs(CALLBACKS) do
        if request[name] ~= nil and type(request[name]) ~= 'function' then return name end
    end
    return nil
end

---Launches a missile. It first moves on the next scheduler step.
---@param request MoonwellSystems.MissileRequest
---@return MoonwellSystems.Missile
function Missiles:launch(request)
    local system = Check.receiver(self, Missiles, 'Missiles', 'Missiles.launch')
    if system.disposed then error('[systems] Missiles.launch: the system is disposed', 2) end
    if type(request) ~= 'table' then error('[systems] Missiles.launch: expected a missile request table', 2) end
    local field = invalid(request)
    if field then error('[systems] Missiles.launch: expected a missile request: ' .. field, 2) end
    local x, y, above = request.x, request.y, request.height or 60
    local ground = system.ground
    local z = (ground and height(ground, x, y) or 0) + above
    local vx, vy, vz = request.vx, request.vy, request.vz or 0
    local effect = request.effect
    if request.model then
        local ok, created = pcall(Effect.create, request.model, x, y)
        if not ok then
            -- The wrappers' message without its position, raised at this function's caller.
            local reason = tostring(created):gsub('^.-:%d+: ', '')
            error('[systems] Missiles.launch: ' .. reason, 2)
        end
        effect = created
        if request.scale then effect:setScale(request.scale) end
    end
    local face = request.face ~= false
    local yaw, pitch = 0, 0
    if effect then
        effect:setPosition(x, y, z)
        if face and (vx ~= 0 or vy ~= 0 or vz ~= 0) then
            yaw, pitch = orientation(vx, vy, vz)
            effect:setOrientation(yaw, pitch, 0)
        end
    end
    ---@type MoonwellSystems.Missile
    local missile = setmetatable({
        data = request.data, system = system, active = true, x = x, y = y, z = z, vx = vx, vy = vy, vz = vz,
        ax = request.ax or 0, ay = request.ay or 0, az = request.az or 0, radius = request.radius,
        lifetime = request.lifetime, maxRange = request.maxRange, maxHits = request.maxHits or 1,
        followGround = request.followGround == true, height = above, effect = effect,
        raw = effect and effect:getHandle() or nil, face = face, yaw = yaw, pitch = pitch, filter = request.filter,
        steer = request.steer, onHit = request.onHit, onEnd = request.onEnd, age = 0, travelled = 0, hits = {},
        hitCount = 0,
    }, Missile)
    system.list[#system.list + 1] = missile
    system.count = system.count + 1
    if not system.cancel then
        system.cancel = system.clock:every(system.clock:getStep(), function() tick(system) end)
    end
    return missile
end

---Missiles in flight.
---@return integer
function Missiles:getCount() return Check.receiver(self, Missiles, 'Missiles', 'Missiles.getCount').count end

---Ends every missile with 'disposed', in launch order, and removes the owned handles. Launching afterwards raises.
---Idempotent.
function Missiles:dispose()
    local system = Check.receiver(self, Missiles, 'Missiles', 'Missiles.dispose')
    if system.disposed then return end
    system.disposed = true
    local list = system.list
    for index = 1, #list do finish(list[index], 'disposed') end
    system.list = {}
    stop(system)
    if system.group then DestroyGroup(system.group); system.group = nil end
    if system.ground then Ground.dispose(system.ground) end
end

-- Missile

---The position; z is absolute.
---@return number x
---@return number y
---@return number z
function Missile:getPosition()
    local missile = Check.receiver(self, Missile, 'Missile', 'Missile.getPosition')
    return missile.x, missile.y, missile.z
end

---@return number vx
---@return number vy
---@return number vz
function Missile:getVelocity()
    local missile = Check.receiver(self, Missile, 'Missile', 'Missile.getVelocity')
    return missile.vx, missile.vy, missile.vz
end

---Sets the velocity, for example from `steer`. With followGround, vz must be 0.
---@param vx number
---@param vy number
---@param vz number
function Missile:setVelocity(vx, vy, vz)
    local missile = Check.receiver(self, Missile, 'Missile', 'Missile.setVelocity')
    if not Check.finite(vx) or not Check.finite(vy) or not Check.finite(vz)
        or not Check.finite(vx * vx + vy * vy + vz * vz) then
        error('[systems] Missile.setVelocity: expected a finite velocity', 2)
    end
    if missile.followGround and vz ~= 0 then
        error('[systems] Missile.setVelocity: a followGround missile has no vertical velocity', 2)
    end
    missile.vx, missile.vy, missile.vz = vx, vy, vz
end

---Seconds flown.
---@return number
function Missile:getAge() return Check.receiver(self, Missile, 'Missile', 'Missile.getAge').age end
---Distance flown.
---@return number
function Missile:getTravelled() return Check.receiver(self, Missile, 'Missile', 'Missile.getTravelled').travelled end
---Units hit so far.
---@return integer
function Missile:getHitCount() return Check.receiver(self, Missile, 'Missile', 'Missile.getHitCount').hitCount end
---The missile's Effect, or nil. Do not destroy it: the missile does.
---@return MoonwellWrappers.Effect?
function Missile:getEffect() return Check.receiver(self, Missile, 'Missile', 'Missile.getEffect').effect end
---False once the missile has ended.
---@return boolean
function Missile:isActive() return Check.receiver(self, Missile, 'Missile', 'Missile.isActive').active end

---Ends the missile with 'cancelled'. Idempotent.
function Missile:dispose()
    finish(Check.receiver(self, Missile, 'Missile', 'Missile.dispose'), 'cancelled')
end

return Missiles
```

- [ ] **Step 4:** `yue -e tests/run.lua; echo "exit $?"` → `missile: SUITE PASSED: 25 tests`,
  `All 16 suites passed`, `exit 0`.

- [ ] **Step 5: Commit** `src/systems/missile.lua tests/missile.lua tests/suites.lua` — `feat: systems.missile`.

---

### Task 4: `systems.knockback`

**Files:** Create `src/systems/knockback.lua`, `tests/knockback.lua`; modify `tests/suites.lua`.

**Interfaces:**
- Consumes `Ground.new`, `Ground.inBounds`, `Ground.isWalkable`, `Ground.isClear`, `Ground.dispose`, `Ordered`,
  `Callback`, `Check`, `Scheduler`, and the wrappers' `Unit` (`getHandle`, `isDisposed`).
- Produces `Knockbacks.new(clock, options?)`, `:apply(unit, request)`, `:get(unit)`, `:getCount()`, `:dispose()`, and
  `Knockbacks.Knockback` with `getUnit`, `isActive`, `getRemaining`, `dispose`.

- [ ] **Step 1: Failing tests** — `tests/knockback.lua`:

```lua
PATHING_TYPE_WALKABILITY, UNIT_TYPE_FLYING = {}, {}
-- The world: unwalkable terrain, obstacles the terrain check misses, items on the ground and the world bounds.
local unwalkable = function() return false end
local obstacle = function() return false end
local items, enumerated = {}, nil
local bounds = {minX = -10000, minY = -10000, maxX = 10000, maxY = 10000}

native('UnitAlive', function(unit) return unit.alive end)
native('GetUnitX', function(unit) return unit.x end)
native('GetUnitY', function(unit) return unit.y end)
native('SetUnitX', function(unit, x) unit.x = x end)
native('SetUnitY', function(unit, y) unit.y = y end)
native('IsUnitType', function(unit, kind) return kind == UNIT_TYPE_FLYING and unit.flying == true end)
native('RemoveUnit', function(unit) unit.alive = false end)
native('GetWorldBounds', function() return bounds end)
native('GetRectMinX', function(rect) return rect.minX end)
native('GetRectMinY', function(rect) return rect.minY end)
native('GetRectMaxX', function(rect) return rect.maxX end)
native('GetRectMaxY', function(rect) return rect.maxY end)
native('Rect', function(minX, minY, maxX, maxY) return {minX = minX, minY = minY, maxX = maxX, maxY = maxY} end)
native('SetRect', function(rect, minX, minY, maxX, maxY)
    rect.minX, rect.minY, rect.maxX, rect.maxY = minX, minY, maxX, maxY
end)
native('RemoveRect', function() end)
native('RemoveLocation', function() end)
native('IsTerrainPathable', function(x, y) return unwalkable(x, y) end)
native('CreateItem', function(itemType, x, y)
    local item = {itemType = itemType, x = x, y = y, visible = true}
    items[#items + 1] = item
    return item
end)
native('SetItemPosition', function(item, x, y)
    item.visible = true
    item.x, item.y = obstacle(x, y) and x + 80 or x, y
end)
native('GetItemX', function(item) return item.x end)
native('GetItemY', function(item) return item.y end)
native('SetItemVisible', function(item, flag) item.visible = flag end)
native('IsItemVisible', function(item) return item.visible end)
native('EnumItemsInRect', function(rect, _, callback)
    for _, item in ipairs(items) do
        if item.x >= rect.minX and item.x <= rect.maxX and item.y >= rect.minY and item.y <= rect.maxY then
            enumerated = item
            callback()
        end
    end
end)
native('GetEnumItem', function() return enumerated end)
native('RemoveItem', function() end)
local Knockbacks = require('systems.knockback')
local Scheduler = require('systems.scheduler')
local Unit = require('wrappers.unit')
eq(totalCalls(), 0)

-- Every test starts with setup(): a clear world, a clock with the given step and a system with the given options.
local function setup(step, options)
    unwalkable, obstacle = function() return false end, function() return false end
    items = {}
    bounds = {minX = -10000, minY = -10000, maxX = 10000, maxY = 10000}
    local clock = Scheduler.new(step or 1)
    resetCalls()
    return Knockbacks.new(clock, options), clock
end
-- A Unit wrapper on a fresh raw handle at the origin.
local function footman(fields)
    local raw = {x = 0, y = 0, alive = true}
    for key, value in pairs(fields or {}) do raw[key] = value end
    return Unit.fromHandle(raw), raw
end
local function push(fields)
    local request = {angle = 0, distance = 10, duration = 1}
    for key, value in pairs(fields or {}) do request[key] = value end
    return request
end
local function near(actual, expected)
    if math.abs(actual - expected) > 1e-9 then
        error('expected about ' .. tostring(expected) .. ', got ' .. tostring(actual), 2)
    end
end
local function join(list) return table.concat(list, ',') end
local function recorder(reasons) return function(_, reason) reasons[#reasons + 1] = reason end end

test('a replacement owns the unit, and the old handle cannot cancel it', function()
    local system, clock = setup()
    local unit, raw = footman()
    local reasons = {}
    local first = system:apply(unit, push({onEnd = recorder(reasons)}))
    local second = system:apply(unit, push({angle = math.pi / 2, duration = 0.5, onEnd = recorder(reasons)}))
    eq(system:get(unit), second); eq(system:getCount(), 1); eq(first:isActive(), false)
    first:dispose()
    clock:advance()
    near(raw.x, 0); near(raw.y, 10)
    eq(join(reasons), 'replaced,completed'); eq(second:isActive(), false); eq(system:getCount(), 0)
    eq(system:get(unit), nil); eq(#PRINTED, 0)
end)

test('an onEnd that applies again cannot take the unit from the newest knockback', function()
    local system, clock = setup(0.5)
    local unit, raw = footman()
    local nested
    system:apply(unit, push({onEnd = function()
        nested = system:apply(unit, push({angle = math.pi / 2, distance = 30}))
    end}))
    local outer = system:apply(unit, push({distance = 20}))
    clock:advance()
    eq(outer:isActive(), false); eq(nested:isActive(), true); eq(system:get(unit), nested)
    near(raw.x, 0); near(raw.y, 15)
    system:dispose()
    eq(nested:isActive(), false)
end)

test('a dead, removed or disposed unit ends with invalid; a refused move ends with blocked', function()
    local system, clock = setup(0.1, {pathing = function(unit) return unit.handle.free == true end})
    local reasons = {}
    local dead, deadRaw = footman({free = true})
    local disposed = footman({free = true})
    local stuck, stuckRaw = footman()
    system:apply(dead, push({onEnd = recorder(reasons)}))
    system:apply(disposed, push({onEnd = recorder(reasons)}))
    system:apply(stuck, push({onEnd = recorder(reasons)}))
    deadRaw.alive = false
    disposed:remove()
    clock:advance()
    eq(join(reasons), 'invalid,invalid,blocked'); eq(system:getCount(), 0)
    eq(stuckRaw.x, 0); eq(callCount('SetUnitX'), 0); eq(#PRINTED, 0)
end)

test('linear falloff covers the distance and decelerates', function()
    local system, clock = setup(0.5)
    local unit, raw = footman()
    local knockback = system:apply(unit, push({distance = 300, falloff = 'linear'}))
    eq(knockback:getRemaining(), 1); eq(knockback:getUnit(), unit)
    clock:advance()
    near(raw.x, 225); eq(knockback:getRemaining(), 0.5) -- three quarters of the distance in the first half
    clock:advance()
    near(raw.x, 300); eq(knockback:getRemaining(), 0); eq(system:getCount(), 0)
end)

test('a unit moves from where it is, so walking adds to the push', function()
    local system, clock = setup(0.5)
    local unit, raw = footman()
    system:apply(unit, push({distance = 100}))
    clock:advance()
    near(raw.x, 50)
    raw.x, raw.y = 60, 7
    clock:advance()
    near(raw.x, 110); near(raw.y, 7)
end)

test('terrain pathing samples the move and refuses it before moving', function()
    local system, clock = setup(1, {pathing = 'terrain', sampleStep = 10})
    local sampled = {}
    unwalkable = function(x)
        sampled[#sampled + 1] = x
        return x == 20
    end
    local unit, raw = footman()
    local reasons = {}
    system:apply(unit, push({distance = 30, onEnd = recorder(reasons)}))
    clock:advance()
    eq(join(sampled), '10.0,20.0'); eq(join(reasons), 'blocked'); eq(raw.x, 0); eq(callCount('SetUnitX'), 0)
    eq(callCount('CreateItem'), 0) -- the terrain policy places no item
    local free, freeClock = setup(1, {pathing = 'none'})
    local other, otherRaw = footman()
    free:apply(other, push({distance = 30}))
    resetCalls()
    freeClock:advance()
    eq(otherRaw.x, 30.0); eq(callCount('IsTerrainPathable'), 0)
end)

test('obstacles pathing, the default, sees what terrain pathing misses', function()
    local system, clock = setup(0.25)
    obstacle = function(x) return x >= 40 and x <= 60 end
    local unit, raw = footman()
    local reasons = {}
    system:apply(unit, push({distance = 100, onEnd = recorder(reasons)}))
    clock:advance()
    near(raw.x, 25)
    clock:advance()
    near(raw.x, 25); eq(join(reasons), 'blocked'); eq(callCount('CreateItem'), 1); eq(items[1].visible, false)
    local terrain, terrainClock = setup(0.25, {pathing = 'terrain'})
    obstacle = function(x) return x >= 40 and x <= 60 end
    local other, otherRaw = footman()
    terrain:apply(other, push({distance = 100}))
    for _ = 1, 4 do terrainClock:advance() end
    near(otherRaw.x, 100)
end)

test('flying units skip the ground checks, but no policy leaves the world bounds', function()
    local system, clock = setup(1)
    obstacle = function() return true end
    local flyer, flyerRaw = footman({flying = true})
    system:apply(flyer, push({distance = 50}))
    clock:advance()
    eq(flyerRaw.x, 50.0); eq(callCount('CreateItem'), 0)
    local reasons = {}
    for _, pathing in ipairs({'obstacles', 'terrain', 'none', function() return true end}) do
        local bounded, boundedClock = setup(1, {pathing = pathing})
        bounds = {minX = -100, minY = -100, maxX = 100, maxY = 100}
        local unit, raw = footman({flying = true})
        bounded:apply(unit, push({distance = 40, onEnd = recorder(reasons)}))
        boundedClock:advance()
        eq(raw.x, 0) -- 40 is past the bounds shrunk by 64
    end
    eq(join(reasons), 'blocked,blocked,blocked,blocked')
end)

test('a move of too many samples is refused before any is taken', function()
    local system, clock = setup(1, {pathing = 'terrain', sampleStep = 1})
    bounds = {minX = -1e31, minY = -1e31, maxX = 1e31, maxY = 1e31}
    local unit = footman()
    local reasons = {}
    system:apply(unit, push({distance = 1e30, onEnd = recorder(reasons)}))
    clock:advance()
    eq(join(reasons), 'blocked'); eq(callCount('IsTerrainPathable'), 0)
end)

test('a pathing function gets the move; a failing one ends the knockback with error', function()
    local messages, seen = {}, {}
    local system, clock = setup(1, {
        onError = function(message) messages[#messages + 1] = message end,
        pathing = function(unit, fromX, fromY, toX, toY)
            if unit.handle.cursed then error('pathing broke', 0) end
            seen[#seen + 1] = table.concat({fromX, fromY, toX, toY}, ' ')
            return true
        end,
    })
    local unit, raw = footman({x = 5, y = 6})
    local cursed, cursedRaw = footman({cursed = true})
    local reasons = {}
    system:apply(unit, push({onEnd = recorder(reasons)}))
    system:apply(cursed, push({onEnd = recorder(reasons)}))
    clock:advance()
    eq(join(seen), '5 6 15.0 6.0'); eq(raw.x, 15.0)
    eq(join(reasons), 'completed,error'); eq(cursedRaw.x, 0); eq(join(messages), 'pathing broke')
    eq(#PRINTED, 0)
    -- A pathing function that ends the knockback itself: the unit does not move.
    local quitting
    local other, otherClock = setup(1, {pathing = function()
        quitting:dispose()
        return true
    end})
    local still, stillRaw = footman()
    quitting = other:apply(still, push())
    otherClock:advance()
    eq(stillRaw.x, 0); eq(quitting:isActive(), false)
end)

test('a failing onEnd is reported, and dispose still ends everything in apply order', function()
    local messages = {}
    local system, clock = setup(1, {onError = function(message) messages[#messages + 1] = message end})
    local ended = {}
    system:apply(footman(), push({onEnd = function()
        ended[#ended + 1] = 'first'
        error('end failed', 0)
    end}))
    system:apply(footman(), push({onEnd = function(_, reason) ended[#ended + 1] = reason end}))
    system:dispose(); system:dispose()
    eq(join(ended), 'first,disposed'); eq(join(messages), 'end failed'); eq(system:getCount(), 0)
    eq(clock:getPending(), 0)
    failsAt(function() system:apply(footman(), push()) end, 'Knockbacks.apply: the system is disposed')
    local printing = setup()
    printing:apply(footman(), push({onEnd = function() error('printed end', 0) end})):dispose()
    eq(PRINTED[1], '[systems] Knockback end failed: printed end')
end)

test('the system ticks only while units are pushed, and dispose releases the handles', function()
    local system, clock = setup(0.5)
    eq(clock:getPending(), 0)
    local reasons = {}
    local first = system:apply(footman(), push({duration = 0.5}))
    local second = system:apply(footman(), push({onEnd = recorder(reasons)}))
    eq(clock:getPending(), 1)
    clock:advance()
    eq(first:isActive(), false); eq(clock:getPending(), 1)
    second:dispose(); second:dispose()
    eq(join(reasons), 'interrupted'); eq(clock:getPending(), 0); eq(second:getRemaining(), 0)
    system:apply(footman(), push())
    eq(clock:getPending(), 1)
    clock:advance()
    resetCalls()
    system:dispose()
    eq(clock:getPending(), 0); eq(callCount('RemoveItem'), 1); eq(callCount('RemoveRect'), 1)
end)

test('arguments are checked at the caller', function()
    local clock = Scheduler.new(1)
    failsAt(function() Knockbacks.new({}) end, 'Knockbacks.new: expected Scheduler')
    failsAt(function() Knockbacks.new(clock, 5) end, 'Knockbacks.new: expected an options table')
    failsAt(function() Knockbacks.new(clock, {onError = 5}) end, 'Knockbacks.new: expected a callback function')
    failsAt(function() Knockbacks.new(clock, {pathing = 'walls'}) end,
        'Knockbacks.new: expected knockback options: pathing')
    failsAt(function() Knockbacks.new(clock, {sampleStep = 0}) end,
        'Knockbacks.new: expected knockback options: sampleStep')
    local system = setup()
    local unit = footman()
    local gone = footman()
    gone:remove()
    failsAt(function() system:apply({}, push()) end, 'Knockbacks.apply: expected a live Unit')
    failsAt(function() system:apply(gone, push()) end, 'Knockbacks.apply: expected a live Unit')
    failsAt(function() system:apply(unit, 5) end, 'Knockbacks.apply: expected a knockback request table')
    local cases = {
        {'angle', {angle = '0'}}, {'distance', {distance = -1}}, {'distance', {distance = 0 / 0}},
        {'duration', {duration = 0}}, {'falloff', {falloff = 'quadratic'}}, {'onEnd', {onEnd = 5}},
    }
    for _, case in ipairs(cases) do
        failsAt(function() system:apply(unit, push(case[2])) end,
            'Knockbacks.apply: expected a knockback request: ' .. case[1])
    end
    eq(system:getCount(), 0)
    failsAt(function() system:get({}) end, 'Knockbacks.get: expected Unit')
    failsAt(function() Knockbacks.getCount({}) end, 'Knockbacks.getCount: expected Knockbacks')
    local knockback = system:apply(unit, push())
    failsAt(function() knockback.getUnit({}) end, 'Knockback.getUnit: expected Knockback')
    failsAt(function() knockback.dispose({}) end, 'Knockback.dispose: expected Knockback')
end)
```

`tests/suites.lua` becomes:

```lua
-- Every behavior suite, in run order. tools/check.lua fails when a suite file is missing here.
return {'internal', 'ordered', 'scheduler', 'signal', 'scope', 'time', 'buffs', 'aura', 'dummy', 'damage', 'vector',
    'geometry', 'terrain', 'missile', 'knockback', 'imports', 'blame'}
```

  (`'imports'` and `'blame'` are already there; this step adds `'knockback'`.)

- [ ] **Step 2:** `yue -e tests/run.lua knockback` → `knockback: ERROR …module 'systems.knockback' not found`.

- [ ] **Step 3: Implement** — `src/systems/knockback.lua`:

```lua
local Callback = require('systems.internal.callback')
local Check = require('systems.internal.check')
local Ground = require('systems.internal.ground')
local Ordered = require('systems.internal.ordered')
local Scheduler = require('systems.scheduler')
local Unit = require('wrappers.unit')

local sqrt, ceil, cos, sin = math.sqrt, math.ceil, math.cos, math.sin

---Knockbacks: a unit is pushed a distance along an angle over a duration. One knockback per unit; a new one replaces
---the old. The system ticks from its scheduler while units are being pushed, and moves them with SetUnitX/Y, which
---keeps their orders and never pauses them. Raw natives on the unit handles by design (spec Part 1 §3).
---@class MoonwellSystems.Knockbacks
---@field package clock MoonwellSystems.Scheduler
---@field package onError (fun(message: string): ...)?
---@field package pathing 'obstacles'|'terrain'|'none'|MoonwellSystems.KnockbackPathing
---@field package sampleStep number
---@field package ground MoonwellSystems.GroundState
---@field package active MoonwellSystems.Ordered Unit to its Knockback, in apply order.
---@field package visit fun(unit: MoonwellWrappers.Unit, item: MoonwellSystems.Knockback)
---@field package ticking boolean
---@field package cancel (fun())? Cancels the scheduler task; nil while nothing is pushed.
---@field package disposed boolean
local Knockbacks = {}
Knockbacks.__index = Knockbacks

---@alias MoonwellSystems.KnockbackEnd 'completed'|'replaced'|'interrupted'|'invalid'|'blocked'|'disposed'|'error'

---Whether a unit may move from (fromX, fromY) to (toX, toY).
---@alias MoonwellSystems.KnockbackPathing
---| fun(unit: MoonwellWrappers.Unit, fromX: number, fromY: number, toX: number, toY: number): ...

---@class MoonwellSystems.KnockbackOptions
---@field onError (fun(message: string): ...)? Receives callback failures; default prints them.
---@field pathing ('obstacles'|'terrain'|'none'|MoonwellSystems.KnockbackPathing)? Default 'obstacles'.
---@field sampleStep number? The largest gap between pathing samples along a move. Default 32.

---@class MoonwellSystems.KnockbackRequest
---@field angle number Radians, as math.atan(dy, dx) gives. (The wrappers' facings are degrees.)
---@field distance number Not negative.
---@field duration number Positive seconds.
---@field falloff ('none'|'linear')? 'linear' decays the speed to zero at the end. Default 'none'.
---@field onEnd (fun(knockback: MoonwellSystems.Knockback, reason: MoonwellSystems.KnockbackEnd): ...)?

---One unit's knockback, returned by apply().
---@class MoonwellSystems.Knockback
---@field package system MoonwellSystems.Knockbacks
---@field package unit MoonwellWrappers.Unit
---@field package raw unit The unit's handle, for the loop.
---@field package active boolean
---@field package dirX number
---@field package dirY number
---@field package speed number The starting speed.
---@field package duration number
---@field package linear boolean
---@field package elapsed number
---@field package onEnd function?
local Knockback = {}
Knockback.__index = Knockback
Knockbacks.Knockback = Knockback

local MAX_SAMPLES = 4096

---@param system MoonwellSystems.Knockbacks
local function stop(system)
    local cancel = system.cancel
    if cancel then
        system.cancel = nil
        cancel()
    end
end

---Ends a knockback once: releases the unit (unless a newer knockback owns it), then runs onEnd.
---@param item MoonwellSystems.Knockback
---@param reason MoonwellSystems.KnockbackEnd
local function finish(item, reason)
    if not item.active then return end
    item.active = false
    local system = item.system
    local active = system.active
    if active:get(item.unit) == item then active:delete(item.unit) end
    local onEnd = item.onEnd
    if onEnd then Callback.call('Knockback end', system.onError, onEnd, item, reason) end
    if active:getSize() == 0 and not system.ticking then stop(system) end
end

---Whether the move is allowed. Nil when the pathing function failed and the knockback has ended.
---@param system MoonwellSystems.Knockbacks
---@param item MoonwellSystems.Knockback
---@param fx number
---@param fy number
---@param tx number
---@param ty number
---@return boolean?
local function allowed(system, item, fx, fy, tx, ty)
    local ground = system.ground
    -- SetUnitX outside the world bounds can crash the game: no policy allows it.
    if not Ground.inBounds(ground, tx, ty) then return false end
    local pathing = system.pathing
    if pathing == 'none' then return true end
    if type(pathing) == 'function' then
        local ok, result = pcall(pathing, item.unit, fx, fy, tx, ty)
        if not ok then
            Callback.report('Knockback pathing', system.onError, result)
            finish(item, 'error')
            return nil
        end
        return result and true or false
    end
    if IsUnitType(item.raw, UNIT_TYPE_FLYING) then return true end
    local dx, dy = tx - fx, ty - fy
    local samples = ceil(sqrt(dx * dx + dy * dy) / system.sampleStep)
    if samples > MAX_SAMPLES then return false end
    local check = pathing == 'terrain' and Ground.isWalkable or Ground.isClear
    for sample = 1, samples do
        if not check(ground, fx + dx * sample / samples, fy + dy * sample / samples) then return false end
    end
    return true
end

---@param system MoonwellSystems.Knockbacks
---@param item MoonwellSystems.Knockback
---@param dt number
local function advance(system, item, dt)
    local raw = item.raw
    -- False for a dead unit and for a removed one, which covers a disposed wrapper: Unit wrappers end by remove().
    if not UnitAlive(raw) then
        finish(item, 'invalid')
        return
    end
    local duration, t0 = item.duration, item.elapsed
    local t1 = t0 + dt
    if t1 > duration then t1 = duration end
    -- The exact distance for this step; linear: speed(t) = speed * (1 - t / duration).
    local travel = t1 - t0
    if item.linear then travel = travel - (t1 * t1 - t0 * t0) / (2 * duration) end
    travel = travel * item.speed
    local fx, fy = GetUnitX(raw), GetUnitY(raw)
    local tx, ty = fx + item.dirX * travel, fy + item.dirY * travel
    local free = allowed(system, item, fx, fy, tx, ty)
    if free == nil then return end
    if not free then
        finish(item, 'blocked')
        return
    end
    if not item.active then return end
    SetUnitX(raw, tx)
    SetUnitY(raw, ty)
    item.elapsed = t1
    if t1 >= duration then finish(item, 'completed') end
end

---@param clock MoonwellSystems.Scheduler Drives the knockbacks; units move once per scheduler step.
---@param options MoonwellSystems.KnockbackOptions?
---@return MoonwellSystems.Knockbacks
function Knockbacks.new(clock, options)
    Check.receiver(clock, Scheduler, 'Scheduler', 'Knockbacks.new')
    if options == nil then options = {} end
    if type(options) ~= 'table' then error('[systems] Knockbacks.new: expected an options table', 2) end
    Callback.optional(options.onError, 'Knockbacks.new')
    local pathing, sampleStep = options.pathing, options.sampleStep
    if pathing == nil then pathing = 'obstacles' end
    if type(pathing) ~= 'function' and pathing ~= 'obstacles' and pathing ~= 'terrain' and pathing ~= 'none' then
        error('[systems] Knockbacks.new: expected knockback options: pathing', 2)
    end
    if sampleStep == nil then sampleStep = 32 end
    if not Check.finite(sampleStep) or sampleStep <= 0 then
        error('[systems] Knockbacks.new: expected knockback options: sampleStep', 2)
    end
    ---@type MoonwellSystems.Knockbacks
    local system = setmetatable({
        clock = clock, onError = options.onError, pathing = pathing, sampleStep = sampleStep, ground = Ground.new(),
        active = Ordered.new(), ticking = false, disposed = false,
    }, Knockbacks)
    local dt = clock:getStep()
    system.visit = function(_, item) advance(system, item, dt) end
    return system
end

---@param value unknown
---@return boolean
local function liveUnit(value) return getmetatable(value) == Unit and not value:isDisposed() end

---@param request table
---@return string? field The first invalid field, or nil.
local function invalid(request)
    if not Check.finite(request.angle) then return 'angle' end
    if not Check.finite(request.distance) or request.distance < 0 then return 'distance' end
    if not Check.finite(request.duration) or request.duration <= 0 then return 'duration' end
    local falloff = request.falloff
    if falloff ~= nil and falloff ~= 'none' and falloff ~= 'linear' then return 'falloff' end
    if request.onEnd ~= nil and type(request.onEnd) ~= 'function' then return 'onEnd' end
    return nil
end

---Pushes a unit, replacing its current knockback (which ends with 'replaced'). It first moves on the next scheduler
---step.
---@param unit MoonwellWrappers.Unit
---@param request MoonwellSystems.KnockbackRequest
---@return MoonwellSystems.Knockback
function Knockbacks:apply(unit, request)
    local system = Check.receiver(self, Knockbacks, 'Knockbacks', 'Knockbacks.apply')
    if system.disposed then error('[systems] Knockbacks.apply: the system is disposed', 2) end
    if not liveUnit(unit) then error('[systems] Knockbacks.apply: expected a live Unit', 2) end
    if type(request) ~= 'table' then error('[systems] Knockbacks.apply: expected a knockback request table', 2) end
    local field = invalid(request)
    if field then error('[systems] Knockbacks.apply: expected a knockback request: ' .. field, 2) end
    local linear = request.falloff == 'linear'
    ---@type MoonwellSystems.Knockback
    local item = setmetatable({
        system = system, unit = unit, raw = unit:getHandle(), active = true, dirX = cos(request.angle),
        dirY = sin(request.angle), speed = (linear and 2 or 1) * request.distance / request.duration,
        duration = request.duration, linear = linear, elapsed = 0, onEnd = request.onEnd,
    }, Knockback)
    local previous = system.active:get(unit)
    -- Install first: if the old knockback's onEnd applies again, that newer one replaces this one.
    system.active:set(unit, item)
    if previous then finish(previous, 'replaced') end
    if not system.cancel then
        system.cancel = system.clock:every(system.clock:getStep(), function()
            system.ticking = true
            system.active:each(system.visit)
            system.ticking = false
            if system.active:getSize() == 0 then stop(system) end
        end)
    end
    return item
end

---The unit's active knockback, or nil.
---@param unit MoonwellWrappers.Unit
---@return MoonwellSystems.Knockback?
function Knockbacks:get(unit)
    local system = Check.receiver(self, Knockbacks, 'Knockbacks', 'Knockbacks.get')
    if getmetatable(unit) ~= Unit then error('[systems] Knockbacks.get: expected Unit', 2) end
    return system.active:get(unit)
end

---Active knockbacks.
---@return integer
function Knockbacks:getCount()
    return Check.receiver(self, Knockbacks, 'Knockbacks', 'Knockbacks.getCount').active:getSize()
end

---Ends every knockback with 'disposed', in apply order, and removes the owned handles. Applying afterwards raises.
---Idempotent.
function Knockbacks:dispose()
    local system = Check.receiver(self, Knockbacks, 'Knockbacks', 'Knockbacks.dispose')
    if system.disposed then return end
    system.disposed = true
    system.active:each(function(_, item) finish(item, 'disposed') end)
    stop(system)
    Ground.dispose(system.ground)
end

---@return MoonwellWrappers.Unit
function Knockback:getUnit() return Check.receiver(self, Knockback, 'Knockback', 'Knockback.getUnit').unit end
---False once the knockback has ended.
---@return boolean
function Knockback:isActive() return Check.receiver(self, Knockback, 'Knockback', 'Knockback.isActive').active end
---Seconds left; 0 once it has ended.
---@return number
function Knockback:getRemaining()
    local item = Check.receiver(self, Knockback, 'Knockback', 'Knockback.getRemaining')
    return item.active and item.duration - item.elapsed or 0
end

---Ends the knockback with 'interrupted'. Idempotent.
function Knockback:dispose()
    finish(Check.receiver(self, Knockback, 'Knockback', 'Knockback.dispose'), 'interrupted')
end

return Knockbacks
```

- [ ] **Step 4:** `yue -e tests/run.lua; echo "exit $?"` → `knockback: SUITE PASSED: 13 tests`,
  `All 17 suites passed`, `exit 0`.

- [ ] **Step 5: Commit** `src/systems/knockback.lua tests/knockback.lua tests/suites.lua` —
  `feat: systems.knockback`.

---

### Task 5: Sweep, imports, fixtures and integration

**Files:** Modify `tests/blame.lua`, `tests/imports.lua`, `tests/editor-positive.lua`, `tests/editor-positive.yue`,
`tests/editor-negative.lua`, `tools/integration.lua`.

- [ ] **Step 1: The sweep.** In `tests/blame.lua`, the module list becomes:

```lua
local modules = {'scheduler', 'signal', 'scope', 'time', 'buffs', 'aura', 'dummy', 'damage', 'geometry', 'terrain',
    'missile', 'knockback'}
```

- [ ] **Step 2: Imports.** In `tests/imports.lua`, before `test('importing every module calls no native', …)` (so no
  earlier test has loaded a wrappers module):

```lua
test('geometry and terrain load no wrappers module', function()
    require('systems.geometry')
    require('systems.terrain')
    eq(totalCalls(), 0)
    for name in pairs(package.loaded) do assert(not name:find('^wrappers%.'), name) end
end)
```

and at the end of the file:

```lua
test('the missile and knockback modules call no native at import and load no group module', function()
    require('systems.missile')
    require('systems.knockback')
    eq(totalCalls(), 0)
    eq(package.loaded['wrappers.effect'] ~= nil, true)
    eq(package.loaded['wrappers.group'], nil)
end)
```

Run `yue -e tests/run.lua; echo "exit $?"` → `imports: SUITE PASSED: 7 tests`, `blame: SUITE PASSED: 1 tests`,
`All 17 suites passed` (143 tests in all).

- [ ] **Step 3: Fixtures.** In `tests/editor-positive.lua`, after `print(current and current.phase)`:

```lua
local Geometry = require('systems.geometry')
local Terrain = require('systems.terrain')
local Missiles = require('systems.missile')
local Knockbacks = require('systems.knockback')
local Effect = require('wrappers.effect')
local terrain = scope:add(Terrain.new({itemType = 2003790951}))
print(terrain:height(0, 0), terrain:isWalkable(0, 0), terrain:isClear(0, 0), terrain:inBounds(0, 0))
print(Geometry.length(3, 4), Geometry.segmentSphere(0, 0, 0, 1, 0, 0, 1, 0, 0, 1), Geometry.orientation(1, 0, 0))
local bolt = 'Abilities\\Weapons\\BallistaMissile\\BallistaMissile.mdl'
local missiles = scope:add(Missiles.new(clock, {terrain = true, targetOffset = 50, maxTargetRadius = 128}))
local missile = missiles:launch({x = 0, y = 0, height = 60, vx = 900, vy = 0, az = -100, radius = 16, lifetime = 2,
    maxRange = 1000, maxHits = 3, model = bolt, scale = 1.5, data = {damage = 40},
    filter = function(unit, flying) return unit ~= hero and flying:isActive() end,
    steer = function(flying, dt)
        local x, y, z = flying:getPosition()
        local vx, vy, vz = flying:getVelocity()
        flying:setVelocity(Geometry.turnToward(vx, vy, vz, hero:getX() - x, hero:getY() - y, -z, 3 * dt))
    end,
    onHit = function(flying, unit) damage:deal({source = hero, target = unit, amount = flying.data.damage}) end,
    onEnd = function(flying, reason) print(flying:getAge(), flying:getTravelled(), flying:getHitCount(), reason) end})
print(missile:getEffect(), missiles:getCount())
missiles:launch({x = 0, y = 0, vx = 500, vy = 0, radius = 8, lifetime = 1, followGround = true, face = false,
    effect = Effect.create(bolt, 0, 0)}):dispose()
local knockbacks = scope:add(Knockbacks.new(clock, {sampleStep = 16, pathing = function(unit, fromX, fromY, toX, toY)
    return unit ~= hero and terrain:isClear(toX, toY) and fromX ~= fromY
end}))
local knockback = knockbacks:apply(hero, {angle = math.atan(1, 0), distance = 300, duration = 0.4, falloff = 'linear',
    onEnd = function(ended, reason) print(ended:getUnit():getName(), reason) end})
print(knockback:isActive(), knockback:getRemaining(), knockbacks:get(hero) == knockback, knockbacks:getCount())
```

In `tests/editor-positive.yue`, after the `systems.damage` import:

```
import "systems.geometry" as Geometry
import "systems.missile" as Missiles
import "systems.knockback" as Knockbacks
```

and at the end of the `mw.on_main` body:

```
  missiles = scope\add Missiles.new clock
  missiles\launch
    x: 0, y: 0, vx: 900, vy: 0, radius: 16, lifetime: 1, model: "bolt.mdl"
    steer: (missile, dt) ->
      vx, vy, vz = missile\getVelocity!
      missile\setVelocity Geometry.turnToward vx, vy, vz, 0, 1, 0, 3 * dt
    onHit: (missile, unit) -> print unit\getName!, missile\getHitCount!
  knockbacks = scope\add Knockbacks.new clock, pathing: "terrain"
  print knockbacks\getCount!
```

In `tests/editor-negative.lua`, before `return true`:

```lua
local Geometry = require('systems.geometry')
local Terrain = require('systems.terrain')
local Missiles = require('systems.missile')
local Knockbacks = require('systems.knockback')
Geometry.length('3', 4) -- EXPECT param-type-mismatch
Terrain.new():height(0) -- EXPECT missing-parameter
Missiles.new(clock):nonexistent() -- EXPECT undefined-field
Missiles.new(clock, {terrain = 'yes'}) -- EXPECT assign-type-mismatch
Knockbacks.new(clock, {pathing = 'walls'}) -- EXPECT assign-type-mismatch
Knockbacks.new(clock):apply(clock, {angle = 0, distance = 1, duration = 1}) -- EXPECT param-type-mismatch
```

- [ ] **Step 4: Integration.** In `tools/integration.lua`:
- `public` becomes:

```lua
local public = {'scheduler', 'signal', 'scope', 'time', 'buffs', 'aura', 'dummy', 'damage', 'geometry', 'terrain',
    'missile', 'knockback'}
```

- add to `entries`, after `damage`:

```lua
    geometry = {source = 'import "systems.geometry" as Geometry\nprint Geometry.length 3, 4\n', systems = {},
        wrappers = {}},
    terrain = {source = 'import "systems.terrain" as Terrain\nt = Terrain.new!\nt\\dispose!\n', systems = {},
        wrappers = {}},
    missile = {source = 'import "systems.missile" as Missiles\nimport "systems.scheduler" as Scheduler\n'
        .. 'm = Missiles.new Scheduler.new!\nm\\dispose!\n', systems = {scheduler = true},
        wrappers = {unit = true, player = true, item = true, timer = true, effect = true}},
    knockback = {source = 'import "systems.knockback" as Knockbacks\nimport "systems.scheduler" as Scheduler\n'
        .. 'k = Knockbacks.new Scheduler.new!\nk\\dispose!\n', systems = {scheduler = true},
        wrappers = unitFamily},
```

- the gate loop's list becomes `{'gate', 'gate-damage', 'gate-physics'}`, and its summary line
  `'Gate examples: all three build and their editor diagnostics are clean; game execution remains manual'`.

  Integration needs `examples/gate-physics.yue`, which Task 6 creates: run integration in Task 6, Step 2.

- [ ] **Step 5:** Run `yue -e tests/run.lua; echo "exit $?"` and the syntax check. Expected: all 17 suites pass; the
  syntax check counts 43 files.

- [ ] **Step 6: Commit** `tests/blame.lua tests/imports.lua tests/editor-positive.lua tests/editor-positive.yue
  tests/editor-negative.lua tools/integration.lua` — `test: sweep, imports, fixtures and bundles for release 4`.

---

### Task 6: The gate example, the gate map and the docs

**Files:** Create `examples/gate-physics.yue`; modify `README.md`, `CHANGELOG.md`, `CONTRIBUTING.md`, `AGENTS.md`;
modify (not under git) `../wrappers-gate/gate.ts`.

- [ ] **Step 1: The gate example** — `examples/gate-physics.yue` (no function ends in a bare `return`: LuaLS flags
  that in the compiled Lua as `redundant-return`):

```
-- The moonwell-systems physics gate (release 4). Built by the gate map's `systems-physics` run (../wrappers-gate,
-- deno task gate systems-physics); CONTRIBUTING lists every expected message. It starts just after the map loads, and
-- also writes its lines to CustomMapData\moonwell-systems-physics.pld.
import "moonwell" as mw
import "moonwell.macros" as {:$FourCC}
import "wrappers.player" as Player
import "wrappers.unit" as Unit
import "wrappers.timer" as Timer
import "wrappers.destructable" as Destructable
import "wrappers.item" as Item
import "systems.scheduler" as Scheduler
import "systems.geometry" as Geometry
import "systems.terrain" as Terrain
import "systems.missile" as Missiles
import "systems.knockback" as Knockbacks

BOLT = "Abilities\\Weapons\\BallistaMissile\\BallistaMissile.mdl"
FOOTMAN = $FourCC "hfoo"
TREE = $FourCC "LTlt"
MOVE = 851986

lines = {}
say = (...) ->
  parts = {"Physics"}
  for index = 1, select "#", ...
    parts[#parts + 1] = tostring (select index, ...)
  line = table.concat parts, " "
  print line
  lines[#lines + 1] = line
save = ->
  PreloadGenClear!
  PreloadGenStart!
  for line in *lines
    Preload (line\gsub "\"", "'")
  PreloadGenEnd "moonwell-systems-physics.pld"
  PreloadGenClear!
round = (value) -> string.format "%.1f", value

physicsGate = (owner) ->
  clock = Scheduler.new!
  clock\start!
  hostile = Player.fromIndex 12
  terrain = Terrain.new!
  missiles = Missiles.new clock
  knockbacks = Knockbacks.new clock
  throughTrees = Knockbacks.new clock, pathing: "terrain"
  enemy = (unit) -> unit\getOwner! == hostile
  nobody = -> false
  -- Paused units do not fight on their own, so the gate decides everything that happens.
  target = (x, y) ->
    unit = Unit.create hostile, FOOTMAN, x, y, 180
    unit\pause true
    unit
  ally = (x, y) ->
    unit = Unit.create owner, FOOTMAN, x, y, 0
    unit\pause true
    unit
  -- A hill for step 5: a permanent crater with a negative depth raises the ground.
  TerrainDeformCrater 250, -150, 200, -150, 1, true
  target(-100, 600)
  ally(-300, 450)
  for x in *{-100, 100, 300}
    target x, 450
  quarry = target 0, 100
  Destructable.create TREE, 512, -448, 270, 1, 0
  Destructable.create TREE, 512, -576, 270, 1, 0
  slider = ally(-600, -400)
  stopped = ally 212, -448
  passer = ally 212, -576
  swapped = ally(-600, -550)
  walker = Unit.create owner, FOOTMAN, -600, -700, 0

  -- 1. A straight missile: it hits the hostile footman, and its bolt points along its travel.
  clock\after 1, ->
    missiles\launch
      x: -600, y: 600, vx: 900, vy: 0, radius: 16, lifetime: 2, model: BOLT, filter: enemy
      onHit: (missile, unit) -> say "1 hit", unit\getName!, "at x", round (missile\getPosition!)
      onEnd: (missile, reason) -> say "1 end", reason, "travelled", round missile\getTravelled!

  -- 2. Piercing and the filter: it passes the allied footman and hits the three hostile ones in order.
  clock\after 2.5, ->
    hits = {}
    missiles\launch
      x: -600, y: 450, vx: 900, vy: 0, radius: 16, lifetime: 2, maxHits: 3, model: BOLT, filter: enemy
      onHit: (_, unit) -> hits[#hits + 1] = round unit\getX!
      onEnd: (_, reason) -> say "2 end", reason, "hit the footmen at x", table.concat hits, " "

  -- 3. Range: nothing is in its way, and it ends at exactly maxRange.
  clock\after 4, ->
    missiles\launch
      x: -600, y: 300, vx: 900, vy: 0, radius: 16, lifetime: 2, maxRange: 600, model: BOLT, filter: enemy
      onEnd: (missile, reason) -> say "3 end", reason, "travelled exactly 600", missile\getTravelled! == 600

  -- 4. An arc: gravity pulls it down; the bolt noses up, then down, and lands on the ground.
  clock\after 5.5, ->
    missiles\launch
      x: -600, y: 150, height: 20, vx: 500, vy: 0, vz: 500, az: -1000, radius: 16, lifetime: 5, model: BOLT
      filter: nobody
      onEnd: (missile, reason) ->
        x, y, z = missile\getPosition!
        say "4 end", reason, "at x", round(x), "height above the ground", round(z - terrain\height(x, y))

  -- 5. The hill: a followGround missile rides over it; a straight one ends on its slope.
  clock\after 7.5, ->
    peak = 0
    missiles\launch
      x: -600, y: -150, vx: 600, vy: 0, radius: 16, lifetime: 5, maxRange: 1200, followGround: true, model: BOLT
      filter: nobody
      steer: (missile) ->
        _, _, z = missile\getPosition!
        peak = math.max peak, z
      onEnd: (_, reason) -> say "5 followGround end", reason, "highest z", round peak
    missiles\launch
      x: -600, y: -200, vx: 600, vy: 0, radius: 16, lifetime: 5, model: BOLT, filter: nobody
      onEnd: (missile, reason) -> say "5 straight end", reason, "at x", round (missile\getPosition!)

  -- 6. Homing: steer turns the missile toward a footman that stands beside its path.
  clock\after 10, ->
    missiles\launch
      x: -600, y: 0, vx: 600, vy: 0, radius: 16, lifetime: 5, model: BOLT
      filter: (unit) -> unit == quarry
      steer: (missile, dt) ->
        x, y, z = missile\getPosition!
        vx, vy, vz = missile\getVelocity!
        aimZ = terrain\height(quarry\getX!, quarry\getY!) + 50 - z
        missile\setVelocity Geometry.turnToward vx, vy, vz, quarry\getX! - x, quarry\getY! - y, aimZ, 2.5 * dt
      onHit: -> say "6 homing hit the footman beside its path"
      onEnd: (_, reason) -> say "6 end", reason

  -- 7. Knockbacks: a slide, a tree in the way, the same push under terrain pathing, a replacement, a walking unit.
  clock\after 12, ->
    push = (system, label, unit, request) ->
      startX, startY = unit\getX!, unit\getY!
      request.onEnd = (_, reason) ->
        say "7", label, reason, "moved", round(unit\getX! - startX), round(unit\getY! - startY)
      system\apply unit, request
    push knockbacks, "slide", slider, angle: 0, distance: 300, duration: 0.6, falloff: "linear"
    push knockbacks, "into a tree", stopped, angle: 0, distance: 400, duration: 0.8
    push throughTrees, "terrain pathing through a tree", passer, angle: 0, distance: 400, duration: 0.8
    push knockbacks, "first push", swapped, angle: 0, distance: 300, duration: 1
    push knockbacks, "second push", swapped, angle: math.pi / 2, distance: 100, duration: 0.5
    walker\issuePointOrder "move", 600, -700
    push knockbacks, "walker", walker, angle: math.pi / 2, distance: 150, duration: 0.5
  clock\after 12.6, -> say "7 walker keeps its move order", walker\getCurrentOrder! == MOVE

  -- 8. An item on the ground does not block isClear; a tree does, and the terrain check misses it.
  clock\after 14, ->
    potion = Item.create $FourCC("phea"), -800, 0
    say "8 isClear on a lying item", terrain\isClear(-800, 0), "the item is still visible",
      IsItemVisible potion\getHandle!
    say "8 at the tree: isWalkable", terrain\isWalkable(512, -448), "isClear", terrain\isClear(512, -448)
    potion\remove!

  -- 9. Speed: 100 missiles among 20 footmen, then 100 knockbacks, each timed over 320 steps of a clock of their own.
  clock\after 15, ->
    bench = Scheduler.new!
    crowd = for index = 0, 19
      target 1700 + (index % 5) * 80, 1700 + math.floor(index / 5) * 80
    fliers = Missiles.new bench
    for index = 1, 100
      angle = index * 0.0628
      fliers\launch
        x: 1860 + 100 * math.cos(angle), y: 1820 + 100 * math.sin(angle), vx: 10 * math.cos(angle + 1.57)
        vy: 10 * math.sin(angle + 1.57), radius: 16, lifetime: 60, model: BOLT, filter: nobody
    started = os.clock!
    for _ = 1, 320
      bench\advance!
    say "9 missiles:", string.format("%.3f", (os.clock! - started) * 1000 / 320), "ms per step; in flight",
      fliers\getCount!
    fliers\dispose!
    herd = for index = 0, 99
      ally 1700 + (index % 10) * 40, 1300 + math.floor(index / 10) * 40
    pushed = Knockbacks.new bench
    for unit in *herd
      pushed\apply unit, angle: 0, distance: 60, duration: 15
    started = os.clock!
    for _ = 1, 320
      bench\advance!
    say "9 knockbacks:", string.format("%.3f", (os.clock! - started) * 1000 / 320), "ms per step; active",
      pushed\getCount!
    pushed\dispose!
    for unit in *crowd
      unit\remove!
    for unit in *herd
      unit\remove!
    bench\dispose!

  clock\after 17, ->
    missiles\dispose!
    knockbacks\dispose!
    throughTrees\dispose!
    terrain\dispose!
    clock\dispose!
    say "gate done"
    save!
  say "gate started"

mw.on_main ->
  start = Timer.create!
  start\start 0, false, (self) ->
    self\destroy!
    SetCameraField CAMERA_FIELD_TARGET_DISTANCE, 2400, 0
    SetCameraPosition 0, -50
    physicsGate Player.fromIndex 0
```

- [ ] **Step 2: Integration.** Run the integration check. Expected:
  `LuaLS: 20 intentional type errors detected at the expected lines`,
  `Moonwell: every entry point (12) bundles only what it imports`,
  `Gate examples: all three build and their editor diagnostics are clean; game execution remains manual`,
  `Integration passed`.

- [ ] **Step 3: The gate map.** In `../wrappers-gate/gate.ts`:
- after the `//   systems-damage …` usage line, add:

```ts
//   systems-physics   moonwell-systems release 4 physics gate (../moonwell-systems/examples/gate-physics.yue)
```

- `copiedRuns` gains `"systems-physics": "../moonwell-systems/examples/gate-physics.yue",`.

  Run `deno task gate systems-physics --no-launch` in `../wrappers-gate` → `Gate map: gate-maps/systems-physics.w3x`.

- [ ] **Step 4: README.**
- The first paragraph's list becomes: "a deterministic scheduler, signals, ownership scopes, time helpers, script
  buffs, auras, dummy casters, a damage pipeline, missiles and knockbacks today; save codes in a later release".
- After the `systems.damage` section, add:

````markdown
### `systems.geometry`

- `Geometry.length(x, y, z = 0)`
- `Geometry.turnToward(vx, vy, vz, tx, ty, tz, maxAngle)` returns `x, y, z`: the velocity turned toward a direction
  by at most `maxAngle`, at the same speed
- `Geometry.segmentSphere(fx, fy, fz, tx, ty, tz, cx, cy, cz, radius)` returns the fraction (0 to 1) of the segment
  at which it first touches the sphere, or nil
- `Geometry.orientation(vx, vy, vz)` returns `yaw, pitch` for `effect:setOrientation(yaw, pitch, 0)`

Pure functions on plain numbers, so nothing is allocated per call. Angles are radians. In Warcraft a positive pitch
points an effect's nose down (measured on 3.0.0.24268), so `orientation` gives a negative pitch for a climbing
velocity.

### `systems.terrain`

- `Terrain.new({itemType?})`
- `height(x, y)`, `isWalkable(x, y)`, `isClear(x, y)`, `inBounds(x, y)`, `dispose()`

A Terrain owns one location, one hidden item and one rect, each created on first use. Measured on 3.0.0.24268:

- `isWalkable` reads the terrain only (`IsTerrainPathable`): it does not see trees or buildings.
- `isClear` sees them: it places a hidden item (`itemType`, default `'wolg'`) on the point and reads where it landed.
  Visible items within 32 units are hidden for the check and shown again. The hidden item stays where it last landed.
- `height` is `GetLocationZ`. It follows temporary terrain deformations, such as a Thunder Clap ripple, while they
  last; w3ts marks it as possibly different between machines then.
- A unit's absolute height is `terrain:height(x, y)` plus `GetUnitFlyHeight`; `BlzGetUnitZ` gives only the ground
  height.
- `inBounds` is true up to 64 units from the world's edge. Moving a unit outside the world bounds can crash the game.

### `systems.missile`

- `Missiles.new(clock, {onError?, terrain = true, targetOffset = 50, maxTargetRadius = 128})`
- `launch(request)` returns a missile; `getCount()`, `dispose()`
- a missile: `getPosition()` (x, y, absolute z), `getVelocity()`, `setVelocity(vx, vy, vz)`, `getAge()`,
  `getTravelled()`, `getHitCount()`, `getEffect()`, `isActive()`, `dispose()`, and its `data`

The request is a table: `x`, `y`, `height` (above the ground, default 60), `vx`, `vy`, `vz?`, `ax?`, `ay?`, `az?`,
`radius`, `lifetime`, `maxRange?`, `maxHits?` (default 1; more pierces), `followGround?`, `model?` or `effect?`,
`scale?`, `face?` (default true), `filter?(unit, missile)`, `steer?(missile, dt)`, `onHit?(missile, unit)`,
`onEnd?(missile, reason)` and `data?`. It ends with `hit-limit`, `expired`, `range`, `ground`, `cancelled`,
`disposed` or `error`.

- **Swept collision.** Each step tests the whole segment the missile travels against a sphere around every unit near
  it, so a fast missile never jumps over a unit. Hits come in order of distance; a tie keeps the engine's enumeration
  order. A unit is hit at most once.
- **Pass a `filter`.** Without one, a missile hits every living unit, the one it was launched from included. Each
  missile has its own filter, so one system serves every team.
- **Heights are above the ground.** A target's centre is the ground at its feet, plus its fly height, plus
  `targetOffset`. A missile whose step ends below the ground ends with `ground`, on the ground. `followGround` keeps
  the missile at its `height` over hills instead. `terrain = false` makes the ground flat at 0 and samples nothing.
- **The effect** (`model`, or an `effect` you hand over) is moved, turned along the travel unless `face = false`, and
  destroyed when the missile ends.
- **Targets larger than `maxTargetRadius`** are hit as if they had that radius: the search around the path uses it.
- **Callbacks** may dispose any missile or the system. A failing `steer`, `filter` or `onHit` ends that missile with
  `error` and is reported; the others still move.
- The system ticks from its scheduler only while missiles are in flight.

```yue
import "systems.missile" as Missiles
import "systems.geometry" as Geometry

missiles = Missiles.new clock
missiles\launch
  x: hero\getX!, y: hero\getY!, vx: 900, vy: 0, radius: 16, lifetime: 1.2, maxHits: 3
  model: "Abilities\\Weapons\\BallistaMissile\\BallistaMissile.mdl"
  filter: (unit) -> unit\getOwner! ~= owner
  onHit: (missile, unit) -> damage\deal source: hero, target: unit, amount: 40

-- An arc: gravity pulls it down, and it ends with "ground" where it lands.
missiles\launch x: 0, y: 0, vx: 500, vy: 0, vz: 500, az: -1000, radius: 16, lifetime: 5, onEnd: explode

-- Homing: steer runs first in every step.
missiles\launch
  x: 0, y: 0, vx: 600, vy: 0, radius: 16, lifetime: 5
  steer: (missile, dt) ->
    x, y, z = missile\getPosition!
    vx, vy, vz = missile\getVelocity!
    missile\setVelocity Geometry.turnToward vx, vy, vz, target\getX! - x, target\getY! - y, 0, 2.5 * dt
```

### `systems.knockback`

- `Knockbacks.new(clock, {onError?, pathing = "obstacles", sampleStep = 32})`
- `apply(Unit, request)` returns a knockback; `get(Unit)`, `getCount()`, `dispose()`
- a knockback: `getUnit()`, `isActive()`, `getRemaining()`, `dispose()`

The request is a table: `angle` (radians, as `math.atan(dy, dx)` gives; the wrappers' unit facings are degrees),
`distance`, `duration`, `falloff?` (`"none"`, or `"linear"` to slow to a stop) and `onEnd?(knockback, reason)`. It
ends with `completed`, `replaced`, `interrupted`, `invalid`, `blocked`, `disposed` or `error`.

- **One knockback per unit:** a new one replaces the old, which ends with `replaced`.
- **The unit is moved with `SetUnitX/Y`,** from where it is in each step. It keeps its orders and is never paused
  (measured: `SetUnitPosition` would clear the order), so it keeps walking while pushed. Stun it yourself if it
  should not.
- **Pathing** is checked before each move, and a refused move ends the knockback with `blocked`:
  `"obstacles"` (the default) stops at unwalkable terrain, trees and buildings; `"terrain"` only at unwalkable
  terrain, so units slide through trees; `"none"` never; a function `(unit, fromX, fromY, toX, toY)` decides itself.
  Flying units skip the `"obstacles"` and `"terrain"` checks.
- **No policy moves a unit outside the world bounds:** that can crash the game.
- A unit pushed into an obstacle under `"terrain"` or `"none"` is not stuck: it can walk out (measured).

```yue
import "systems.knockback" as Knockbacks

knockbacks = Knockbacks.new clock
angle = math.atan target\getY! - caster\getY!, target\getX! - caster\getX!
knockbacks\apply target, angle: angle, distance: 300, duration: 0.4, falloff: "linear"
```
````

- "Changes from wc3-lib" gains:

```markdown
- Missiles and knockbacks have no ports: `Missiles` and `Knockbacks` are the Warcraft systems, and they tick from the
  scheduler they are given (`update(dt)` is not public).
- Vectors are plain numbers, not `{x, y, z}` tables.
- Missile heights are above the ground by default (`terrain = false` gives the old flat behavior), and `targetOffset`
  replaces the `centerHeight` callback. Each missile has its own `filter`; `followGround` and the effect's facing are
  new; contact ties follow the enumeration order, not handle ids.
- A knockback takes an angle and a distance (`knockbackVelocity` is gone). Pathing has a default that sees trees and
  buildings, and no policy leaves the world bounds.
```

- [ ] **Step 5: CONTRIBUTING.** After release 3's gate steps (before the `v0.1.0: passed…` record), add:

```markdown
Release 4 (v0.4.0) has its own run, `deno task gate systems-physics`, about 18 seconds, with the camera zoomed out.
Its lines also go to `Documents\Warcraft III\CustomMapData\moonwell-systems-physics.pld`. Hostile footmen stand in
the upper rows, a small hill rises right of the centre, and your footmen stand by two trees in the lower rows.
`Physics gate started` prints first.

21. At 1 s a bolt flies east along the top row, pointing east, and hits the hostile footman:
    `Physics 1 hit Footman at x <about -147>`, then `Physics 1 end hit-limit travelled <about 453>`.
22. At 2.5 s a bolt passes your footman and hits the three hostile ones:
    `Physics 2 end hit-limit hit the footmen at x -100.0 100.0 300.0`.
23. At 4 s: `Physics 3 end range travelled exactly 600 true`.
24. At 5.5 s a bolt climbs nose up, turns over and comes down nose first:
    `Physics 4 end ground at x <about -80> height above the ground 0.0`.
25. At 7.5 s two bolts fly east at the hill. One rides over it:
    `Physics 5 followGround end range highest z <about 210>`. The other ends on its slope:
    `Physics 5 straight end ground at x <n>` (record n).
26. At 10 s a bolt curves north into the footman beside its path:
    `Physics 6 homing hit the footman beside its path`, then `Physics 6 end hit-limit`.
27. At 12 s, in the lower rows: `Physics 7 first push replaced moved 0.0 0.0` at once; the upper-left footman slides
    east and slows (`Physics 7 slide completed moved 300.0 0.0`); the footman pushed at the upper tree stops in front
    of it (`Physics 7 into a tree blocked moved <n> 0.0`, n under 260); the one pushed at the lower tree goes through
    it (`Physics 7 terrain pathing through a tree completed moved 400.0 0.0`);
    `Physics 7 second push completed moved 0.0 100.0`; `Physics 7 walker completed moved <x> <y>` with both above 0;
    and `Physics 7 walker keeps its move order true`.
28. At 14 s: `Physics 8 isClear on a lying item true the item is still visible true` and
    `Physics 8 at the tree: isWalkable true isClear false`.
29. At 15 s the game freezes briefly, then `Physics 9 missiles: <ms> ms per step; in flight 100` and
    `Physics 9 knockbacks: <ms> ms per step; active 100`. Both must be under 3; record them.
30. At 17 s: `Physics gate done`. No `[systems] … failed` line prints in the run.
```

  In the "Publication and tag gate" paragraph, the list of gate examples becomes "(`examples/gate.yue`,
  `examples/gate-damage.yue`, `examples/gate-physics.yue`)".

- [ ] **Step 6: CHANGELOG and AGENTS.**
- CHANGELOG: a new first section:

```markdown
## Unreleased

Release 4 of the wc3-lib port (spec `2026-10-01-moonwell-systems-release-4-design` in the Moonwell repository).

- `systems.geometry`: `length`, `turnToward`, `segmentSphere` and `orientation`, on plain numbers.
- `systems.terrain`: ground height, terrain walkability, `isClear` (which also sees trees and buildings, by placing
  a hidden item) and the world bounds.
- `systems.missile`: missiles with swept collision, heights above the ground, a filter per missile, piercing, range,
  gravity, steering, `followGround`, and an effect that faces its travel.
- `systems.knockback`: one knockback per unit, by angle, distance and duration, with linear falloff and pathing
  policies; no policy leaves the world bounds.
- The per-tick loops call raw natives on handles the systems own, and allocate nothing per call.
```

- AGENTS: "Release 3 (v0.3.0): damage. Releases 4 and 5: physics; persistence." becomes "Release 3 (v0.3.0): damage.
  Release 4 (v0.4.0): geometry, terrain, missile, knockback, on `internal/vector.lua` and `internal/ground.lua`.
  Release 5: persistence."; the Process line gains `deno task gate systems-physics` for release 4; three new
  pitfalls:

```markdown
- Hot loops (missile and knockback steps) call raw natives and the unchecked `internal/vector.lua` and
  `internal/ground.lua`; the public `geometry` and `terrain` modules are for maps. Do not add argument checks or
  table allocations to the internal ones.
- A mutation that removes a loop bound can make a test loop forever and eat memory: run mutation checks with a
  timeout per run.
- A YueScript function that ends in a bare `return` compiles to Lua that LuaLS flags (`redundant-return`): end gate
  functions with a statement instead.
```

- [ ] **Step 7: Checks and commit.** Run the three checks. Commit `examples/gate-physics.yue README.md CHANGELOG.md
  CONTRIBUTING.md AGENTS.md` — `docs: release 4 docs and the physics gate`.

---

### Task 7: Release

- [ ] **Step 1:** The maintainer runs `deno task gate systems-physics` in `../wrappers-gate`, watches steps 21 to 27
  (the bolts' facing, the arc, the hill, the homing curve, the knockbacks) and says when `Physics gate done` has
  printed. Read `Documents\Warcraft III\CustomMapData\moonwell-systems-physics.pld` for the lines. Steps 21 to 30 of
  CONTRIBUTING must hold; record the measured values. If a reading contradicts the spec (for example, a number over
  the 3 ms budget, or a knockback that passes the tree under `'obstacles'`), stop and fix it test-first before
  releasing.
- [ ] **Step 2:** Record: CHANGELOG `## 0.4.0 (<date>)` with a release-gate section; CONTRIBUTING `v0.4.0:` record
  and the measured values in steps 21 to 30; README Status (`v0.4.0`, the new modules) and the `tag = "v0.4.0"`
  example. Run the three checks. Commit, push, tag `v0.4.0` on the verified commit, push the tag, GitHub pre-release
  from the changelog section (`gh` at `C:\Program Files\GitHub CLI\gh.exe`). Verify each step's exit code separately.
- [ ] **Step 3:** Tag consumption in `../systems-tag-check-040` (a map made with `init --link`, wrappers `v0.7.0`,
  systems `v0.4.0`): with each of the three gate examples in turn as `src/main.yue`: check, build, build `--minify`.
  The lock records both tags' commits; the fetched systems files match the tag's `src/` byte for byte; the lock is
  unchanged after removing `.moonwell/` and checking again. Record it in CONTRIBUTING and the tag in AGENTS; commit;
  push.
- [ ] **Step 4:** Moonwell records: `AGENTS.md` (a state bullet for v0.4.0 with the probe's and the gate's
  measurements; next work becomes release 5, which starts with the Preload carrier probe), the roadmap's phase 3
  status and the `CHANGELOG.md` Unreleased line; commit; push; `gh run list`.
