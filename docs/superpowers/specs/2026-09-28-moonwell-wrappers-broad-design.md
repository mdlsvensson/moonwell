# Moonwell Broad Wrapper Library (wrappers v0.2.0) — Design

- **Date:** 2026-09-28
- **Status:** Approved in chat on 2026-09-28; awaiting review of this written spec.
- **Builds on:** `2026-09-28-moonwell-wrappers-design.md` (sub-project 4c, released as wrappers `v0.1.0`, tag commit
  `c1209f5`). Everything in that spec still applies unless this one changes it explicitly.
- **Target:** wrappers `v0.2.0` in the separate `mdlsvensson/moonwell-wrappers` repository. No Moonwell CLI change.

## 1. Intent and scope

Extend 4c's focused foundation toward broad everyday coverage. The maintainer chose **breadth, curated**: add the
gameplay-object handle types, each with a hand-picked everyday method set in 4c's style, and deepen the existing six
classes where maps use them most. UI and presentation types follow in a later release.

In this release:

- New classes: Item, Destructable, Rect, Region, Force (section 5).
- A shared widget layer for Unit, Item and Destructable (section 3).
- Deeper Unit (hero, abilities, inventory, state, presentation, orders) and Player (resources, alliances, tech, slot)
  (section 6).
- Trigger: more event registrations, conditions, and removal of individual actions and conditions (section 7).
- Group: more enumerations with an optional Lua filter, and iteration helpers (section 8).
- Weak identity caches for widgets (section 4).

The release is additive: every v0.1.0 call keeps its behavior. The only observable change is the Unit cache's weakness
(section 4), recorded in the CHANGELOG. Handwritten Lua 5.3 with LuaLS annotations remains the implementation approach;
code generation was considered and rejected again (a curated surface gains little from it and needs escape hatches for
every validation, conversion and callback special case).

## 2. Modules and dependencies

```text
src/wrappers/
  player.lua unit.lua timer.lua trigger.lua group.lua effect.lua     (v0.1.0, extended)
  item.lua destructable.lua rect.lua region.lua force.lua            (new)
  internal/handle.lua internal/callback.lua                          (v0.1.0, extended)
  internal/widget.lua                                                (new)
```

Still no umbrella module, no globals, literal requires only, and no game-object creation at import.

**Argument conversion without imports.** `internal/handle.lua` keeps a table of loaded registries keyed by class name.
Each `Handle.new(class, name, options)` adds its registry there.

- `Handle.unwrap(value, name, operation) -> raw`: converts a wrapper argument of class `name`. If that class's module
  was never loaded, no value can be a member, so the call fails with the usual `expected <name> wrapper` error.
- `Handle.unwrapWidget(value, operation) -> raw`: accepts a member of any loaded widget class (Unit, Item,
  Destructable), failing with `expected Widget wrapper`.

A caller holding a wrapper necessarily loaded its module, so conversions need no import. **Rule: a module imports
another public module only to return that class's wrappers.** Taking one as a parameter never needs an import.
Existing v0.1.0 conversions (`PlayerWrapper.getHandle(owner)`, `Unit.getHandle(unit)`) move to `Handle.unwrap`, with
identical error behavior.

Resulting import graph (acyclic):

| Module       | Imports (public)            | Why                                               |
| ------------ | --------------------------- | ------------------------------------------------- |
| player       | —                           |                                                   |
| item         | player                      | `getOwner()` returns Player                       |
| unit         | player, item                | `getOwner()`; inventory methods return Item       |
| destructable | —                           |                                                   |
| rect         | —                           |                                                   |
| region       | —                           |                                                   |
| force        | player                      | `getPlayers()` returns Player[]                   |
| group        | unit                        | snapshots and `first()` return Unit               |
| timer        | —                           |                                                   |
| trigger      | —                           | callbacks receive the Trigger only                |
| effect       | —                           | (v0.1.0 imported unit only for conversion)        |

Trigger and Effect drop their v0.1.0 imports of Unit and Player. Integration verifies that a map importing only
`wrappers.trigger` bundles no other public module.

## 3. Widget layer

`internal/widget.lua` provides:

- `Widget.install(class, registry)`: copies `getLife()`/`setLife(value)` (GetWidgetLife/SetWidgetLife) and
  `getX()`/`getY()` (GetWidgetX/GetWidgetY) onto the class, **only for names the class does not define itself**.
  Functions are copied, not reached through a metatable chain. Unit keeps its own v0.1.0 `getX`/`getY` (GetUnitX/Y),
  so its native mapping is unchanged.
- Widget family membership: each widget class registers with the family through `Handle.new` options, which is what
  `Handle.unwrapWidget` consults.

For LuaLS, `MoonwellWrappers.Widget` is an annotation-only base class declaring the four shared methods;
`MoonwellWrappers.Unit: MoonwellWrappers.Widget`, and likewise Item and Destructable. There is **no `Widget` module and
no `Widget.fromHandle`**: Warcraft has no reliable handle-type check, so a raw `widget` cannot be routed to the right
class. Code that receives a raw widget converts it with the specific class it knows it has.

## 4. Identity and lifetime

| Cache                | Classes                                            | Cleanup     |
| -------------------- | -------------------------------------------------- | ----------- |
| Weak-valued          | Unit, Item, Destructable                           | `remove()`  |
| Strong until cleanup | Timer, Trigger, Group, Effect, Rect, Region, Force | `destroy()` |
| Strong for session   | Player                                             | none        |

**Why weak for widgets.** Group enumerations and trigger events wrap every unit, item and destructable they touch.
Units decay, powerups are consumed and trees die without any wrapper call, so a strong cache would grow without bound
and hold stale handles.

**Behavior.** `Handle.new(class, name, {weak = true})` makes the handle-to-wrapper table weak-valued (`__mode = 'v'`).
The membership table is already weak-keyed. A widget wrapper that nothing references may be collected; wrapping the
same handle later creates a fresh wrapper. That is indistinguishable in use because wrapper fields are read-only by
contract. While any reference exists (a variable, a table key, a closure, a snapshot array), `fromHandle` returns the
same table. Explicit `remove()` still invalidates immediately and removes the entry, as in 4c. Garbage collection never
calls a native, and nothing in the library iterates the cache, so collection timing cannot affect game state.

Unit `kill()` still leaves the wrapper valid; a naturally removed widget's wrapper is simply collected once unreferenced.
Holding a wrapper of a removed widget and calling methods on it is the caller's error, as with any stale raw handle.

**Risk and fallback.** The in-game gate (section 10, step 7) must confirm that a handle Lua still references is not
recycled into a different game object while referenced, and that weak collection under unit churn causes no multiplayer
desync. If either fails, fall back to strong caches for widgets plus an explicit `forget()` (drop from the cache without
a native call), and revise this section before release.

## 5. New classes

All have 4c's common members: `fromHandle(raw)`, `getHandle()`, `isDisposed()`, `.handle`. Parameters named after a
class take that wrapper. Factories raise if the native returns nil. Coordinates, facings, scales and life are numbers;
rawcodes are integers.

### Item (`wrappers.item`, weak, widget)

- `create(typeId, x, y) -> Item` (CreateItem).
- `getTypeId()`, `getName()`, `getLevel()`.
- `setPosition(x, y)` (SetItemPosition). Widget layer: `getX/getY/getLife/setLife`.
- `getCharges() -> integer`, `setCharges(n)`.
- `getOwner() -> Player` (GetItemPlayer), `setOwner(Player, changeColor)` (SetItemPlayer), `isOwned()`, `isPowerup()`.
- `isVisible()`/`setVisible(b)`, `isInvulnerable()`/`setInvulnerable(b)`, `setDroppable(b)`, `setPawnable(b)`.
- `remove()` (RemoveItem).

### Destructable (`wrappers.destructable`, weak, widget)

- `create(typeId, x, y, facing, scale, variation) -> Destructable` (CreateDestructable).
- `getTypeId()`, `getName()`. Widget layer: `getX/getY/getLife/setLife`.
- `getMaxLife() -> number`/`setMaxLife(v)`, `kill()`, `restore(life, birth)` (DestructableRestoreLife).
- `isInvulnerable()`/`setInvulnerable(b)`, `show(b)` (ShowDestructable).
- `setAnimation(name)`, `queueAnimation(name)`.
- `remove()` (RemoveDestructable).

### Rect (`wrappers.rect`, strong)

- `create(minX, minY, maxX, maxY) -> Rect` (native `Rect`).
- `worldBounds() -> Rect`: GetWorldBounds allocates a new rect on each call, so the result is owned and should be
  destroyed.
- `getMinX()`, `getMinY()`, `getMaxX()`, `getMaxY()`, `getCenterX()`, `getCenterY()`.
- `set(minX, minY, maxX, maxY)` (SetRect), `moveTo(x, y)` (MoveRectTo).
- `destroy()` (RemoveRect).

Preplaced rects are wrapped with `Rect.fromHandle(gg_rct_Name)`; as in 4c, wrapping transfers no cleanup duty.

### Region (`wrappers.region`, strong)

- `create() -> Region` (CreateRegion).
- `addRect(Rect)`, `clearRect(Rect)`, `addCell(x, y)`, `clearCell(x, y)`.
- `containsPoint(x, y)` (IsPointInRegion), `containsUnit(Unit)` (IsUnitInRegion).
- `destroy()` (RemoveRegion). A region does not own its rects.

### Force (`wrappers.force`, strong)

- `create() -> Force` (CreateForce).
- `add(Player)`, `remove(Player)`, `contains(Player)` (IsPlayerInForce), `clear()`.
- `enumPlayers()`, `enumAllies(Player)`, `enumEnemies(Player)`: clear first, then the ForceEnum native with a nil
  filter (same line-local diagnostic exception as 4c's nil filters).
- `getPlayers() -> Player[]`: a new dense one-based snapshot built with ForForce and GetEnumPlayer. The enumeration
  callback only collects handles; no user code runs inside it.
- `destroy()` (DestroyForce).

### Deliberately excluded

`location` (coordinates are the Lua idiom), pure-Lua conveniences such as `rect:containsPoint`, user-data natives (Lua
tables serve), and item/destructable enumeration in rects (deferred with the UI and presentation types).

## 6. Deeper Unit and Player

### Unit

Hero (natives' own behavior applies to non-heroes; no pre-check):

- `isHero()` (IsUnitType with UNIT_TYPE_HERO), `getHeroName()` (GetHeroProperName).
- `getLevel()`/`setLevel(level, showEffect)`; `getXP()`/`setXP(xp, showEffect)`/`addXP(xp, showEffect)`.
- `getStr(includeBonuses)`/`setStr(value, permanent)`; same for `Agi` and `Int`.
- `getSkillPoints()`, `modifySkillPoints(delta) -> boolean`, `selectSkill(abilityId)`,
  `revive(x, y, showEffect) -> boolean`.

Abilities (integer ids; no ability wrapper, no Blz ability fields):

- `addAbility(id) -> boolean`, `removeAbility(id) -> boolean`, `getAbilityLevel(id)`,
  `setAbilityLevel(id, level) -> integer`.
- `makeAbilityPermanent(id, permanent) -> boolean`, `hideAbility(id, hidden)`, `disableAbility(id, disabled, hideUI)`.
- `startCooldown(id, seconds)`, `endCooldown(id)`, `getCooldownRemaining(id) -> number`.

Inventory (slots zero-based; slot arguments must be integers in `[0, getInventorySize())`):

- `getInventorySize()`, `getItemInSlot(slot) -> Item?`.
- `addItem(Item) -> boolean`, `addItemById(typeId) -> Item?`, `removeItem(Item)`, `removeItemFromSlot(slot) -> Item?`.
- `hasItem(Item)`, `dropItemAt(Item, x, y) -> boolean` (UnitDropItemPoint), `dropItemToSlot(Item, slot) -> boolean`,
  `useItem(Item) -> boolean`.

State and presentation:

- `getMana()`/`setMana(v)` (GetUnitState/SetUnitState, UNIT_STATE_MANA); `getMaxMana()`, `setMaxMana(n)`,
  `setMaxLife(n)` (Blz natives, integers).
- `getMoveSpeed()`/`setMoveSpeed(v)`.
- `setX(x)`/`setY(y)` (SetUnitX/SetUnitY): ignore pathing; distinct from `setPosition`, which keeps SetUnitPosition.
- `setScale(scale)` (uniform, passed three times to SetUnitScale), `setVertexColor(r, g, b, a)` (integers 0–255),
  `setAnimation(name)`.
- `pause(b)`/`isPaused()`, `setInvulnerable(b)`/`isInvulnerable()` (BlzIsUnitInvulnerable), `show(b)`/`isHidden()`.
- `isType(unittype)`, `isAlly(Player)`, `isEnemy(Player)`, `getName()`, `getCurrentOrder() -> integer`.
- `isAlive()`: `not IsUnitType(raw, UNIT_TYPE_DEAD) and GetUnitTypeId(raw) ~= 0`. (`UnitAlive` is a common.ai native
  absent from Moonwell's natives and editor types.)
- `damageTarget(Widget, amount, attack, ranged, attacktype, damagetype, weapontype) -> boolean`.
- `applyTimedLife(buffId, seconds)`.

Orders:

- `issueOrderById(id)`, `issuePointOrderById(id, x, y)`, `issueTargetOrderById(id, Widget)`, all returning boolean.
- `issueTargetOrder(order, target)` now accepts any Widget (was Unit only). Additive: Unit targets behave as before.

### Player

- `getGold()`/`setGold(n)`/`addGold(n)`, same for `Lumber`: GetPlayerState/SetPlayerState with the resource states;
  `add` reads then sets.
- `getAlliance(Player, alliancetype)`, `setAlliance(Player, alliancetype, b)`, `isAlly(Player)`, `isEnemy(Player)`.
- `getTechCount(techId, specificOnly)`, `setTechResearched(techId, level)`, `addTechResearched(techId, levels)`,
  `setTechMaxAllowed(techId, max)`, `setAbilityAvailable(abilityId, b)`.
- `getController()`, `getSlotState()`, `getRace()`, `getTeam()`, `getStartX()`/`getStartY()`
  (GetPlayerStartLocationX/Y).
- `isLocal()`: compares with GetLocalPlayer. It is the only local-player helper; there is no `Player.local()` factory,
  so a local player value does not spread by accident. README warns that branching on it must not change synchronized
  game state.

SetPlayerName stays excluded, as in 4c.

## 7. Trigger

### Registrations

All pass a nil native filter where the native takes one. Registrations return nothing: Warcraft cannot unregister an
event, so destroying the trigger is the only removal.

- `registerAnyUnitEvent(playerunitevent)`: registers for every player index in `[0, bj_MAX_PLAYER_SLOTS)`, like
  TriggerRegisterAnyUnitEventBJ. Item pickup/drop/use/sell are playerunitevents and use this or
  `registerPlayerUnitEvent`.
- `registerPlayerEvent(Player, playerevent)`.
- `registerChatEvent(Player, text, exactMatch)` (TriggerRegisterPlayerChatEvent).
- `registerEnterRegion(Region)`, `registerLeaveRegion(Region)`.
- `registerDeathEvent(Widget)`.
- `registerUnitInRange(Unit, range)`: range validated finite and non-negative.
- `registerUnitStateEvent(Unit, unitstate, limitop, value)`, `registerGameEvent(gameevent)`.

### Conditions, actions and tokens

- `addCondition(predicate) -> TriggerCondition`, where `predicate: fun(trigger: Trigger): any` and the result is taken
  as truthy/falsy. The trigger creates and owns one `Condition(fn)` boolexpr per predicate and registers it with
  TriggerAddCondition. The predicate runs behind the callback boundary: an error prints
  `[wrappers] Trigger condition failed: ...` and the condition evaluates **false**.
- `addAction(callback) -> TriggerAction`: as in v0.1.0, now returning an opaque token.
- `removeAction(token)` (TriggerRemoveAction), `removeCondition(token)` (TriggerRemoveCondition, then DestroyCondition).
- `clearActions()` (TriggerClearActions), `clearConditions()` (TriggerClearConditions, then destroy every owned
  boolexpr).
- Tokens are opaque Lua tables (`MoonwellWrappers.TriggerAction`, `MoonwellWrappers.TriggerCondition`) with no public
  fields, authenticated by private membership like wrappers. A token from another trigger raises an error; removing a
  token twice, or after a clear, is harmless. Removal makes the retained closure a no-op immediately, including within a
  firing already in progress.
- `destroy()` order: dispose the wrapper and make all closures no-ops; TriggerClearConditions; DestroyCondition for each
  owned boolexpr; DestroyTrigger. Repeated destroy is harmless.
- `evaluate() -> boolean` (TriggerEvaluate), `execute()` (TriggerExecute).

## 8. Group

- `enumInRect(Rect, filter?)`, `enumOfPlayer(Player, filter?)`, `enumSelected(Player, filter?)`, and
  `enumInRange(x, y, radius, filter?)` (v0.1.0 calls unchanged).
- Every enumeration clears the group, runs the native with a nil filter, then, if `filter` is given, evaluates
  `filter(unit)` for every member of a snapshot and removes those returning falsy. No `Filter(...)` boolexpr is created,
  so there is no native callback boundary or boolexpr ownership; the filter is ordinary synchronous user code.
- If the filter raises, the group is cleared and the error is re-raised, so a half-filtered group never escapes.
- A non-function, non-nil filter is rejected before clearing.
- Enumerating "of type" is a filter: `(u) -> u\getTypeId! == id`. GroupEnumUnitsOfType takes a unit name string and is
  not wrapped.
- `forEach(fn)`: iterates a `getUnits()` snapshot; errors propagate to the caller.
- `first() -> Unit?` (FirstOfGroup).
- `enumSelected` inherits the native's synchronization behavior; README notes it.

## 9. Types and errors

- Classes `MoonwellWrappers.Item`, `.Destructable`, `.Rect`, `.Region`, `.Force`; annotation-only
  `MoonwellWrappers.Widget`; opaque `MoonwellWrappers.TriggerAction` and `.TriggerCondition`.
- Nullable returns are annotated nullable: `getItemInSlot`, `removeItemFromSlot`, `addItemById`, `first`, plus every
  `fromHandle` (still conservatively nullable in LuaLS 3.19.1).
- Each nil-filter native call carries the same line-local diagnostic exception as 4c's two. No other suppressions.
- Misuse raises `[wrappers] Class.method: ...` errors. Validated: wrapper identity and disposal (including Widget
  arguments), callback/predicate/filter types, token type and ownership, inventory slot range, finite non-negative
  ranges and timeouts. All other value domains are left to Warcraft.

## 10. Verification and release gate

Automated (the wrappers repo's CONTRIBUTING suite, extended):

1. Native-double tests for every new method: exact native and arguments, return values, wrapper conversion, rejection
   of disposed receivers and arguments.
2. `Handle.unwrap` and the widget family: each widget class converts; a Timer where a Widget is expected is rejected;
   an unloaded class's name fails cleanly.
3. Weak cache: identity stable while referenced; a collected wrapper is recreated fresh (forced with `collectgarbage`
   in the test VM only, never in shipped code); explicit removal still clears the entry immediately. Strong classes are
   not collected.
4. Trigger: add/remove/double-remove/foreign tokens, removal during a firing, a throwing condition evaluating false,
   clear operations, destroy order including boolexpr destruction.
5. Group: filters run after enumeration, a throwing filter leaves the group cleared, filter type validation. Force
   snapshot through a ForForce double.
6. Lua 5.3.6 syntax check.
7. LuaLS positive and negative fixtures: Widget parameters accept Unit/Item/Destructable and reject Timer; nullable
   returns require narrowing; token types check.
8. Integration: fresh Moonwell 0.5.0 consumer, check and build (normal and minified); importing only `wrappers.trigger`
   bundles no other public module.

In-game (maintainer, Warcraft III Reforged 3.0.0.24268, World Editor 3.00), extending `examples/gate.yue`, normal and
minified:

1. Item created, picked up (item event fires), dropped and removed.
2. Destructable killed (death event fires) and restored.
3. A unit entering a region built from a rect fires the trigger; a force snapshot lists the expected players.
4. A hero gains XP and levels, adds and levels an ability and starts its cooldown, fills and empties its inventory.
5. A condition gates an action; a removed action no longer fires; a throwing condition prints and evaluates false.
6. Group enumerations with a filter select the expected units.
7. Weak-cache probe: spawn and kill waves of units, let them decay, allocate to provoke collection; wrappers of live
   units keep identity, and no stale wrapper resolves to a newly created unit.
8. Two-player LAN run of the gate with unit churn: no desync. If LAN is not possible, record it as unverified and
   decide with the maintainer before release.
9. Open the packed map in World Editor. Then the tag consumption gate as in v0.1.0.

## 11. Plan phasing

One implementation plan, written in Moonwell's `docs/superpowers/plans/`, with a review after each phase:

1. Internal: registry table, `Handle.unwrap`/`unwrapWidget`, weak option, widget layer; migrate v0.1.0 conversions;
   Unit weak cache.
2. Item and Destructable.
3. Rect, Region and Force.
4. Unit depth.
5. Player depth.
6. Trigger registrations, conditions and tokens.
7. Group enumerations and helpers.
8. README API reference, CHANGELOG, CONTRIBUTING gate, `examples/gate.yue`, integration and LuaLS fixtures, the
   wrappers repo's AGENTS.md, and Moonwell's AGENTS.md state and backlog.

## 12. Out of scope

UI types (dialog, multiboard, leaderboard, quest, timerdialog, frame) and presentation types (sound, texttag,
lightning, image, ubersplat, fogmodifier) — the next backlog candidates. Also: item/destructable enumeration, location,
ability and buff wrappers, Blz object fields, w3ts compatibility, code generation, schedulers, implicit cleanup,
resource pooling and the wc3-lib port. Wrapper calls inherit native synchronization requirements.
