# Moonwell Systems Release 4 (v0.4.0): Physics — Design

- **Date:** 2026-10-01
- **Status:** The design was approved in chat on 2026-10-01; this written spec awaits review.
- **Builds on:** Part 1 of `2026-09-30-moonwell-systems-design.md`, which binds this release (rules §4, tooling §5,
  releases §6; §3 allows raw natives for terrain sampling and the innermost physics loops), and releases 1 to 3.
- **Inputs:** `wc3-lib`'s `physics/geometry.ts`, `physics/missile/`, `physics/knockback/`, `physics/warcraft-terrain.ts`
  and their 25 tests (`tests/physics.test.ts`); the performance note
  (`docs/superpowers/research/2026-09-30-wrappers-performance.md`); and the physics probe of 2026-10-01 (§2).
- **Target:** moonwell-systems `v0.4.0`. It still needs moonwell-wrappers `v0.7.0` and Moonwell 0.5.2. The wrappers do
  not change.

## 1. Scope and the maintainer's choices

Four modules: `systems.geometry`, `systems.terrain`, `systems.missile` and `systems.knockback`, on two internal ones
(`systems.internal.vector`, `systems.internal.ground`). The maintainer chose (2026-10-01):
- **Missile heights are terrain-aware by default:** a map gives heights above the ground, and the system samples the
  terrain under missiles and targets.
- **A missile's visual is a model path or the map's own Effect;** the system moves it, turns it along its travel and
  destroys it.
- **A probe ran before this spec** (§2).

With the design, the maintainer approved:
- knockback pathing defaults to `'obstacles'`, which also sees trees and buildings;
- every pathing policy refuses a move outside the world bounds;
- a knockback is given as an angle, a distance and a duration;
- a `followGround` missile option;
- terrain sampling can be switched off per system.

## 2. Measured by the probe (2026-10-01, Warcraft III 3.0.0.24268)

The probe is `../wrappers-gate/src/probe_physics.yue` (`deno task gate probe-physics`); its raw lines are in
`../wrappers-gate/PROBE-PHYSICS-RESULTS.md`.

- **`IsTerrainPathable(x, y, PATHING_TYPE_WALKABILITY)` sees only the terrain.** It read walkable at every sampled
  point inside a tree, inside a farm and on a standing unit. It read blocked in the map's boundary strip. Flyability
  read the same.
- **Placing an item sees trees and buildings.** An item placed at a tree's centre, or 48 from it, landed about 80 away;
  at 96 it stayed. At a farm's centre it landed about 93 away. On a standing footman it stayed.
- **A unit moved into an obstacle is not stuck.** `SetUnitX/Y` put footmen exactly at the tree's and the farm's centre;
  both accepted a move order and walked out. `SetUnitPosition` snapped to a free point.
- **`SetUnitX` keeps the unit's order; `SetUnitPosition` clears it.**
- **Heights.** `GetLocationZ` read 0 on flat ground and 200 at the centre of a hill raised with a permanent
  `TerrainDeformCrater`. `BlzGetUnitZ` read the ground height for a footman and for a gryphon: it does not include the
  fly height. `GetUnitFlyHeight` read 240 for the gryphon and 0 for the footman. So a unit's absolute height is
  `GetLocationZ` at its position plus `GetUnitFlyHeight`.
- **A temporary deformation changes `GetLocationZ` while it lasts:** a 3-second crater of depth 150 read -25, -75 and
  -125 at half, one and a half and two and a half seconds, and 0 after it ended.
- **Effect orientation** (`BlzSetSpecialEffectOrientation`, radians): yaw 0 points east (+x), yaw pi/2 points north
  (+y), and a positive pitch points the nose down.
- **Effect height:** `BlzSetSpecialEffectPosition` takes an absolute z (a bolt at the hill's ground height plus 120
  hovered just above the footman standing on the hill).

## 3. Structure and speed

- The performance note sets the budget: a system's per-tick work stays under 3 ms (10% of a tick) at the load a map
  expects. Natives cost 300 to 470 ns each and a wrapper method about 70 ns more, so the loops call raw natives on
  handles the system owns or has just enumerated (Part 1 §3), and wrap a unit only when a callback needs it.
- Hot paths allocate nothing per call: vector functions take and return numbers, not tables.
- The public modules check their arguments (Part 1 §4.2); the loops use the unchecked internal ones:
  - `systems.internal.vector`: the vector functions without checks;
  - `systems.internal.ground`: terrain sampling without checks, on a state table that owns the handles.
- Both systems tick from the scheduler they are given, at its step, and only while they have something to move: the
  first launch or apply starts a repeating task, and the last end cancels it.
- Collections of missiles and knockbacks keep creation order (Part 1 §4.4). The set of units a missile has hit is keyed
  by unit handle and only ever looked up, never iterated.

## 4. `systems.geometry`

Pure functions on numbers. Angles are radians.

```lua
function Geometry.length(x, y, z)                                   -- z default 0
function Geometry.turnToward(vx, vy, vz, tx, ty, tz, maxAngle)      -- returns x, y, z
function Geometry.segmentSphere(fx, fy, fz, tx, ty, tz, cx, cy, cz, radius)  -- returns a fraction 0..1, or nil
function Geometry.orientation(vx, vy, vz)                           -- returns yaw, pitch
```

- `turnToward` rotates a velocity toward the direction `(tx, ty, tz)` by at most `maxAngle`, keeping its speed. A zero
  velocity stays zero; a zero direction leaves the velocity unchanged; a direction straight behind turns left in the
  horizontal plane (straight up or down: toward +x).
- `segmentSphere` gives the earliest contact of the segment from `f` to `t` with a sphere, as a fraction of the
  segment; 0 when the segment starts inside it; nil when it misses, or when the segment has no length and starts
  outside.
- `orientation` gives the yaw and pitch that point an effect along a velocity: `yaw = atan(vy, vx)` and
  `pitch = -atan(vz, horizontal speed)` (a positive pitch points down, §2). A zero velocity gives 0, 0.
- Every argument must be a finite number (`radius` and `maxAngle` not negative); otherwise the function raises
  `[systems] Geometry.<name>: expected finite numbers` at the caller.

## 5. `systems.terrain`

```lua
function Terrain.new(options)      -- options: {itemType: integer?}?; creates nothing
function Terrain:height(x, y)      -- the ground's absolute height (GetLocationZ)
function Terrain:isWalkable(x, y)  -- in bounds and walkable terrain; does not see trees or buildings
function Terrain:isClear(x, y)     -- isWalkable, and no tree, building or other pathing blocker (the item trick)
function Terrain:inBounds(x, y)    -- inside the world bounds, 64 units from their edge
function Terrain:dispose()         -- removes the owned handles; idempotent; later calls raise
```

- The handles are created on first use: one location for `height`; one item and one rect for `isClear`; the world
  bounds are read once.
- `isClear` moves the hidden item to the point and reads where it landed: more than 10 units away means blocked.
  Other items would displace it too, so the visible items within 32 units are hidden for the check and shown again, in
  the engine's enumeration order. The item is hidden again afterwards and stays where it landed.
- `itemType` is the probe item's type, default `'wolg'` (a standard item, so a map needs no object data).
- `height` follows temporary terrain deformations while they last (§2). w3ts marks `GetLocationZ` as possibly
  different between machines then; the multiplayer checks before Moonwell 1.0 cover it (Part 1 §6).

## 6. `systems.missile`

### 6.1 API

```lua
---@param clock MoonwellSystems.Scheduler
---@param options {onError: fun(message: string)?, terrain: boolean?, targetOffset: number?, maxTargetRadius: number?}?
function Missiles.new(clock, options)   -- terrain default true; targetOffset default 50; maxTargetRadius default 128
function Missiles:launch(request)       -- returns MoonwellSystems.Missile
function Missiles:getCount()
function Missiles:dispose()             -- ends every missile ('disposed'); launching afterwards raises

function Missile:getPosition()          -- x, y, z (absolute)
function Missile:getVelocity()          -- vx, vy, vz
function Missile:setVelocity(vx, vy, vz)
function Missile:getAge()  function Missile:getTravelled()  function Missile:getHitCount()
function Missile:getEffect()            -- the Effect, or nil
function Missile:isActive()
function Missile:dispose()              -- ends it with 'cancelled'
Missile.data                            -- the request's data
```

The request is a table:

| Field | Meaning |
| --- | --- |
| `x`, `y` | start position |
| `height` | start height above the ground; default 60 |
| `vx`, `vy`, `vz` | velocity in units per second; `vz` default 0 |
| `ax`, `ay`, `az` | constant acceleration, for example `az = -1400` for an arc; default 0 |
| `radius` | collision radius, not negative |
| `lifetime` | positive seconds; required, also for a missile that stands still |
| `maxRange` | positive distance after which it ends with `'range'` |
| `maxHits` | positive integer, default 1; more than 1 pierces |
| `followGround` | keeps the missile at `height` above the ground; `vz` and `az` must then be 0 or absent |
| `model` or `effect` | a model path, or an Effect whose ownership passes to the missile; at most one |
| `scale` | the created effect's scale; only with `model` |
| `face` | turn the effect along the velocity; default true |
| `filter(unit, missile)` | whether the missile may hit this Unit; default: every living unit, the caster included |
| `steer(missile, dt)` | runs first in every step, to set the velocity (homing) or dispose the missile |
| `onHit(missile, unit)` | runs for each hit, in contact order |
| `onEnd(missile, reason)` | runs once, after the effect is destroyed |
| `data` | any value, kept as `missile.data` |

End reasons: `'hit-limit'`, `'expired'`, `'range'`, `'ground'`, `'cancelled'`, `'disposed'`, `'error'`.

### 6.2 A step

For each missile, in launch order, with `dt` the scheduler's step:

1. `seconds` is `dt`, cut to the lifetime that remains.
2. `steer` runs.
3. The acceleration is added to the velocity (semi-implicit Euler: velocity first, then position).
4. `seconds` is cut so the travel does not pass `maxRange`.
5. The end point is the position plus velocity times `seconds`. With `followGround`, its z is the ground there plus
   `height`.
6. **Candidates:** one reused group enumerates the units within reach of the segment's midpoint: half the segment's
   horizontal length, plus `radius`, plus `maxTargetRadius`. A unit not yet hit is first asked
   `IsUnitInRangeXY` around that midpoint, with half the length plus `radius` (§12); most units fail it and are read
   no further. For the others, the target sphere has the unit's position, a centre height of ground +
   `GetUnitFlyHeight` + `targetOffset`, and a radius of `BlzGetUnitCollisionSize`, capped at `maxTargetRadius`. The
   swept test is the segment against a sphere of the two radii added.
7. Contacts are ordered by fraction, with ties in the engine's enumeration order.
8. For each contact, while the missile is active: the unit must still be alive and pass `filter`; then it is marked
   hit, the missile moves to the contact point, `onHit` runs, and at `maxHits` the missile ends with `'hit-limit'`. A
   unit that fails `filter` is not marked, so it is asked again in a later step.
9. Otherwise the missile moves to the end point. Without `followGround`, an end point below the ground puts it on the
   ground and ends it with `'ground'`. Then `'expired'` and `'range'` are checked, in that order.
10. The effect moves with the missile, and turns when `face` is true and the velocity changed.

- A missile launched during a step first moves in the next one. A callback may dispose any missile or the system.
- With `terrain = false` the ground is the plane z = 0 everywhere: nothing samples the terrain.
- A target larger than `maxTargetRadius` is hit as if it had that radius.

### 6.3 Failures and checks

- A failing `steer`, `filter` or `onHit` ends that missile with `'error'` and is reported (label `Missile callback`);
  the other missiles still move. A failing `onEnd` is reported (label `Missile end`). Nothing is rethrown.
- `launch` raises at the caller for: a disposed system; a request that is not a table; a field that is not a finite
  number where one is needed, a negative `radius`, a `lifetime` or `maxRange` that is not positive, a `maxHits` that is
  not a positive integer, a velocity or acceleration whose square overflows, both `model` and `effect`, `scale`
  without `model`, `followGround` with a vertical velocity or acceleration, a callback that is not a function
  (`expected a missile request: <field>`); and a model that Warcraft cannot load (the wrappers' message).
- `setVelocity` raises for values that are not finite. The request is copied: changing the table later changes
  nothing. Checks come before the effect is created or taken over.

## 7. `systems.knockback`

```lua
---@param clock MoonwellSystems.Scheduler
---@param options {onError: fun(message: string)?, pathing: string|function|nil, sampleStep: number?}?
function Knockbacks.new(clock, options)   -- pathing default 'obstacles'; sampleStep default 32
function Knockbacks:apply(unit, request)  -- returns MoonwellSystems.Knockback; replaces the unit's current one
function Knockbacks:get(unit)             -- the unit's active knockback, or nil
function Knockbacks:getCount()
function Knockbacks:dispose()             -- ends every knockback ('disposed'); applying afterwards raises

function Knockback:getUnit()  function Knockback:isActive()  function Knockback:getRemaining()
function Knockback:dispose()              -- ends it with 'interrupted'
```

The request is a table: `angle` (radians, as `math.atan(dy, dx)` gives; the wrappers' facings are degrees),
`distance` (not negative), `duration` (positive seconds), `falloff` (`'none'`, the default, or `'linear'`: the speed
decays to zero at the end) and `onEnd(knockback, reason)`.

End reasons: `'completed'`, `'replaced'`, `'interrupted'`, `'invalid'`, `'blocked'`, `'disposed'`, `'error'`.

- **One knockback per unit.** A new one is installed first and the old one then ends with `'replaced'`, so an `onEnd`
  that applies again cannot take the unit back from the newest one.
- **A step** moves each unit, in apply order, by the exact displacement for that step (for `'linear'`, the integral of
  the decaying speed), from where the unit is now. So over the whole duration it moves `distance`, plus whatever the
  unit walked.
- **It moves with `SetUnitX/Y`:** the unit keeps its order and is never paused (§2). Stunning it is the map's choice.
- **`'invalid'`:** the unit is dead or removed (`UnitAlive` is false). That covers a disposed wrapper: a Unit wrapper
  is only disposed by `remove()`.
- **Pathing**, checked before each move; a refused move ends the knockback with `'blocked'`:

  | `pathing` | A move is refused when |
  | --- | --- |
  | `'obstacles'` | a sample point is not clear (`isClear`: terrain, trees, buildings) |
  | `'terrain'` | a sample point is not walkable terrain (`isWalkable`) |
  | `'none'` | never, except by the bounds rule below |
  | a function `(unit, fromX, fromY, toX, toY)` | it returns false |

  - Samples lie along the move, at most `sampleStep` apart, ending at the destination; a move of more than 4096
    samples is refused.
  - Flying units (`UNIT_TYPE_FLYING`) skip the `'obstacles'` and `'terrain'` samples.
  - **Every policy refuses a destination outside the world bounds** (`inBounds`): `SetUnitX` out there can crash the
    game.
- **Failures:** a failing pathing function ends that knockback with `'error'`; a failing `onEnd` is reported (label
  `Knockback end`). Both are reported, not rethrown.
- **Checks at the caller:** a Unit that is not a live wrapper; a request that is not a table; `angle`, `distance` or
  `duration` not finite, a negative `distance`, a `duration` that is not positive, an unknown `falloff`, an `onEnd`
  that is not a function (`expected a knockback request: <field>`); in `new`, an unknown `pathing` or a `sampleStep`
  that is not positive.

## 8. Changes from `wc3-lib`

- No ports: `Missiles` and `Knockbacks` are the Warcraft systems, driven by the scheduler they are given.
- Vectors are numbers, not `{x, y, z}` tables.
- Heights are above the ground by default, with `terrain = false` for the old flat behavior; `targetOffset` replaces
  the `centerHeight` callback.
- Each missile has its own `filter`, so one system serves every team.
- Contact ties are broken by enumeration order, not handle ids. A target over `maxTargetRadius` is capped, not an
  error.
- The visual is a model or an Effect, and it faces its travel. The `MissileVisual` interface is gone.
- `followGround` is new.
- A knockback takes an angle and a distance; `knockbackVelocity` is gone. Pathing has a default, sees trees and
  buildings, and is bounded by the world.
- Failures are reported, never rethrown; `update(dt)` is not public.

## 9. Verification

Automated (Part 1 §5):
- suites `vector` and `geometry` (the functions, their edge cases and the argument checks), `terrain` (lazy handles,
  heights, walkability, the item check with its hiding of nearby items, bounds, dispose), `missile` and `knockback`:
  `wc3-lib`'s 25 tests adapted to this API, and the new rules (heights above the ground and fly heights, `filter`,
  enumeration-order ties, the radius cap, `followGround`, effect ownership and facing, ticking only while something
  moves, the pathing policies, flying units, the bounds rule);
- the `blame` sweep over the new modules and classes, and `imports`;
- integration: one-module bundles for the four entry points (`systems.geometry` and `systems.terrain` bundle no
  wrappers module), LuaLS fixtures, and the new gate example.

In-game gate (the maintainer, 3.0.0.24268): its own run, `deno task gate systems-physics`, from
`examples/gate-physics.yue`:
1. **A straight missile** hits a hostile footman and ends with `'hit-limit'`; its bolt points along its travel.
2. **Piercing and the filter:** a missile with `maxHits = 3` passes an allied footman and hits three hostile ones in
   distance order.
3. **Range:** a missile with nothing in its way ends with `'range'` at exactly `maxRange`.
4. **An arc:** a missile with gravity noses up, then down, and ends with `'ground'` on the ground.
5. **Hills:** on a hill raised by the gate, a `followGround` missile rides over it, and a straight one ends with
   `'ground'` on its slope.
6. **Homing:** a `steer` callback with `turnToward` curves a missile into a target beside its path.
7. **Knockback:** a footman pushed 300 units with `'linear'` falloff moves 300 and ends `'completed'`; one pushed at
   a tree ends `'blocked'` in front of it; under `'terrain'` the same push goes through the tree; a second apply ends
   the first with `'replaced'`; a walking footman keeps its order.
8. **An item on the ground** does not block `isClear`.
9. **Speed:** 100 missiles among 20 footmen, and 100 knockbacks, each timed over 320 steps; both must stay under 3 ms
   per step. Record the numbers.

Then tag `v0.4.0` and check tag consumption with every gate example.

## 10. Out of scope

A spatial grid (add it only after measuring a real slowdown); missiles that hit destructables, items or other
missiles; bouncing; units as missile visuals; pausing or stunning knocked units; destroying trees in a knockback's
path; persistence (release 5).

## 11. Departures found while planning (2026-10-01)

The plan's code was prototyped and checked with mutations before it was written down. That changed these details:

- An Effect handed to a missile may be destroyed by its owner first: the wrappers' `destroy()` is idempotent, so the
  missile's own destroy then does nothing.
- A missile's age is the plain sum of its steps (the last step is cut to the lifetime that remains, and the sum then
  equals the lifetime); only the distance flown is set to exactly `maxRange`.
- Dead units are skipped when a contact is handled, not when candidates are collected (§6.2 steps 6 and 8): one
  native fewer for every living unit near a missile's path.
- A knockback step that moves nowhere takes no pathing sample.
- The modules expose their classes (`Missiles.Missile`, `Knockbacks.Knockback`), as `DamageSystem.Hit` does, so the
  blame sweep reaches their methods.
- The gate also writes its lines to `CustomMapData\moonwell-systems-physics.pld`, so the log need not be
  screenshotted.

## 12. Departures found at the gate (2026-10-01)

The first gate run passed its missile and knockback steps but read 3.603 ms per step for 100 missiles among 20
footmen, over the budget of §3, and two of its steps could not be judged by eye. A second probe
(`../wrappers-gate/PROBE-MISSILE-PERF-RESULTS.md`) measured the natives before the loop was changed:

- `GroupEnumUnitsInRange` tests unit origins, and clears the group before it fills it. So the search keeps its
  `maxTargetRadius` (§6.2 step 6), and the group is never cleared between steps.
- `IsUnitInRangeXY` is true up to its range plus the unit's collision size, in every direction, for units and
  buildings. A unit the step touches is within half the step, `radius` and its own radius of the step's midpoint, so
  this one native rules out a unit that is too far, and nothing else is read from it. The first loop read seven
  natives from every enumerated unit; in the probe the search fell from 4.1 ms to 1.9 ms per step.
- Knockback pathing costs about 3 µs per unit and step with `'none'`, 4 µs with `'terrain'` and 27 µs with
  `'obstacles'` (the item placement). 100 knockbacks under `'obstacles'` read 2.647 ms per step: within the budget,
  and documented in the README.
- The gate (§9) is two runs. `deno task gate systems-physics` covers steps 1 to 6, 8 and the missile half of 9; its
  homing bolt starts at right angles to its target, because the first one finished its turn in two steps and looked
  straight. `deno task gate systems-knockback` (`examples/gate-knockback.yue`) covers step 7 and the knockback half
  of 9 with one footman at a time, each push announced a second before it happens: five simultaneous pushes could
  not be followed.
