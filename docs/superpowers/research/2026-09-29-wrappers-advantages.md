# Where moonwell-wrappers is ahead of w3ts, with examples (2026-09-29)

A companion to `2026-09-29-w3ts-comparison.md`, whose section 3 lists these points briefly. Each section shows the w3ts
pattern (TypeScript, shortened from w3ts 3.0.2), what goes wrong or stays unguarded, and what moonwell-wrappers v0.3.0
does instead: gameplay code in YueScript, library internals in Lua. w3ts is a capable, far broader library; these are
the places where our narrower design buys correctness. Claims that rest on engine behaviour we have not measured are
marked as such.

## 1. Destroyed wrappers are disposed

**w3ts.** `destroy()` calls the native and nothing else. The wrapper keeps its handle and its cache entry, and every
method still runs:

```ts
const t = Timer.create();
t.destroy();                          // DestroyTimer(t.handle)
t.start(1, false, () => print("hi")); // TimerStart on a destroyed timer: no error from the library
```

What Warcraft does with a dead handle is undefined, and nothing points at the mistake. The same holds for units after
`destroy()` (`RemoveUnit`), effects, groups and the rest.

**Ours.** Every registry tracks membership. Disposal happens before the native call, is idempotent, and every later use
fails with a message naming the operation:

```moon
t = Timer.create!
t\destroy!
t\destroy!                      -- harmless: already disposed
t\isDisposed!                   -- true
t.handle                        -- nil
t\start 1, false, -> print "hi" -- error: [wrappers] Timer.start: Timer is disposed
```

The mechanism (`src/wrappers/internal/handle.lua`):

```lua
function registry.dispose(value, operation)
    local raw = member(value, operation)
    if raw == false then return nil end -- second destroy: nothing to do
    members[value] = false              -- every later require() raises "is disposed"
    byHandle[raw] = nil                 -- a reused handle gets a fresh wrapper
    value.handle = nil
    return raw                          -- the caller destroys the native object after this
end
```

A disposed wrapper passed as an argument fails too: `group\add removedUnit` raises `Group.add: Unit is disposed`.
`unit\remove!` disposes that wrapper everywhere it is held, including arrays returned earlier by `group\getUnits!`.

## 2. Callback errors are reported, not lost

**w3ts.** Callbacks go to the natives as they are:

```ts
Timer.create().start(1, true, () => {
  const target = Unit.fromHandle(GetTriggerUnit())!; // undefined here: no triggering unit in a timer
  target.kill();                                     // Lua error inside the timer callback
});
```

Warcraft swallows errors raised inside Lua timer and trigger callbacks: nothing reaches the screen or the log (Moonwell
AGENTS.md, "Pitfalls already paid for"). The bug shows up only as behaviour that silently stops.

**Ours.** Timer callbacks, trigger actions and trigger conditions run behind one boundary
(`src/wrappers/internal/callback.lua`):

```lua
function Callback.call(label, fn, argument)
    local ok, message = pcall(fn, argument)
    if not ok then report(label .. ' callback', message) end
end
```

The v0.3.0 gate's probe run recorded the result in game (the same probes passed since v0.1.0), with periodic ticks
continuing afterwards:

```
[wrappers] Timer callback failed: war3map.lua:2580: intentional timer probe
[wrappers] Trigger callback failed: war3map.lua:2573: intentional trigger probe
[wrappers] Trigger condition failed: war3map.lua:2691: intentional condition probe
```

A failing condition counts as false, as a failing native condition does, but the error is printed first.

**Compared with WCSharp** (`2026-09-29-wcsharp-comparison.md`): its guards print only in debug builds, and they cover a
whole batch. In release builds, `Delay` and its event dispatcher still catch errors but print nothing, and one failing
action ends the batch: the rest of `Delay`'s queue is dropped unrun, and later registrations miss that event. Ours
wraps each callback separately, always, so one failure affects nothing else.

## 3. No native filters or conditions to leak

**w3ts.** Every enumeration wraps its callback in `Filter()` and every `addCondition` in `Condition()`, and nothing ever
destroys the resulting boolexpr:

```ts
group.enumUnitsInRange(x, y, 500, () => IsUnitEnemy(GetFilterUnit(), owner));
// → GroupEnumUnitsInRange(group, x, y, 500, Filter(fn))          no DestroyBoolExpr
trigger.addCondition(() => GetUnitTypeId(GetTriggerUnit()) === id);
// → TriggerAddCondition(trigger, Condition(fn))                  never destroyed, not even by destroy()
```

With a fresh closure on each call, this probably allocates a new boolexpr every time (not verified in game), so a
periodic enumeration leaks steadily. The filter also runs inside the native enumeration, where errors are silent
(section 2) and units must be read through `GetFilterUnit()`.

**Ours.** Enumerations pass no native filter. The optional filter runs afterwards, over a snapshot, as ordinary Lua that
receives wrappers; its errors propagate to the caller, and a failing filter leaves the group empty instead of half
filtered:

```moon
enemies = Group.create!
enemies\enumInRange x, y, 500, (u) -> u\isEnemy owner
-- a typo inside the filter raises at this line, with a normal stack trace
```

Conditions are owned by their trigger. Each has its boolexpr destroyed on `removeCondition`, `clearConditions` and
`destroy`:

```lua
-- Trigger:removeCondition, after unlinking the cell
TriggerRemoveCondition(raw, cell.native)
DestroyCondition(cell.boolexpr)
```

## 4. Timer and trigger lifecycles are guarded

**w3ts.** `addAction` returns the raw `triggeraction`; removing actions and conditions is the natives' business, and the
condition's boolexpr is never released (section 3). Timers store nothing, so a callback closure lives as long as the
native keeps it.

**Ours.** Each `start` gets a new generation; a callback from a replaced schedule or a destroyed timer does nothing, and a
one-shot timer drops its callback after firing:

```lua
TimerStart(raw, timeout, periodic, function()
    local current = state.callback
    if state.generation ~= generation or not current then return end -- stale schedule: ignore
    if not periodic then state.callback = nil end                     -- one-shot: release the closure
    Callback.call('Timer', current, self)
end)
```

Trigger actions and conditions return opaque tokens. Removal takes effect at once, even during a firing; removing twice
does nothing; a token from another trigger raises:

```moon
once = nil
once = trig\addAction ->
  print "runs a single time"
  trig\removeAction once -- safe inside its own action
other\removeAction once  -- error: [wrappers] Trigger.removeAction: token belongs to another trigger
```

## 5. The timer is handed to its callback

**w3ts.** A callback that needs its timer asks the game, and w3ts's own documentation warns the call may crash:

```ts
Timer.create().start(2, false, () => {
  Timer.fromExpired()?.destroy(); // GetExpiredTimer(): "@bug Might crash the game if called when
});                               //  there is no expired timer" (w3ts's comment)
```

**Ours.** The callback receives its Timer; `GetExpiredTimer` is never needed:

```moon
Timer.create!\start 2, false, (timer) -> timer\destroy!
```

## 6. Identity is a contract, not a side effect of garbage collection

**w3ts.** One `WeakMap` keyed by the handle userdata serves every type. Whether a wrapper, and anything stored on it,
survives depends on when the game's Lua releases that userdata, which the library neither controls nor documents. A
destroyed object's entry is never removed.

```ts
const t = Trigger.create();
(t as any).owner = hero;                      // data attached to the wrapper
// ... later, inside the trigger's action:
const same = Trigger.fromEvent();             // the same object only while the WeakMap entry survives
```

**Ours.** The cache policy is part of the API. Timer, Trigger, Group, Rect, Region, Force, Effect and the presentation
classes are cached strongly until destroyed, so `fromHandle` always returns the same table. Unit, Item and Destructable,
which the game can remove by itself, are cached weakly (by value); the v0.2.0 gate measured
`collected=true stale=true identity=true`, and the README states the consequence:

> Keep a reference (a variable, table key or closure) wherever identity matters. Do not key weak tables
> (`__mode = 'k'`) by Unit, Item or Destructable wrappers for game data: each client's collector drops those entries at
> its own time, so game logic that reads such a table can desync.

Disposal removes the cache entry (section 1), so a handle the game reuses later gets a fresh wrapper, never a dead one.

**Compared with WCSharp:** WCSharp.Api has no wrappers at all (the handle is the object and every call compiles to the
native), so it has no identity problem to solve. Against such a zero-cost API, our caches are the cost of wrapping, not
an advantage; the advantage that remains is wrappers that know they are disposed.

## 7. No getters that answer differently on each machine

**w3ts.** Machine-local values are ordinary getters, marked only in doc comments:

```ts
if (effect.x > 0) unit.kill();              // effect.x is BlzGetLocalSpecialEffectX: may differ per machine → desync
const me = MapPlayer.fromLocal();           // a different player on every machine
if (unit.z > 100) unit.damageTarget(...);   // BlzGetUnitZ: "@async"
```

**Ours.** Effect has no position getters, Sound has no `isPlaying`, and the README says why. Local visibility never
branches around a native: every machine calls the same native with a machine-local boolean.

```lua
-- TextTag:setVisibleFor: identical call everywhere, only the flag differs
SetTextTagVisibility(raw, Handle.unwrap(player, 'Player', 'TextTag.setVisibleFor') == GetLocalPlayer())

-- Sound.playOnce with a player: every machine starts and releases the sound; the others hear it at volume 0
SetSoundVolume(raw, (o.player == nil or o.player == GetLocalPlayer()) and o.volume or 0)
StartSound(raw)
KillSoundWhenDone(raw)
```

Handle ids are the same trap. w3ts exposes `GetHandleId` as `Handle.id`, and WCSharp marks `GetHandleId` obsolete
everywhere because in Lua it is "prone to desyncs", most likely because an id is recycled only once every reference
is released, which in Lua depends on each machine's garbage collector. Ours exposes no id; its registry keys by the handle itself, as WCSharp recommends.

The one deliberate exception, `sound:playFor(player)`, starts the sound only on that player's machine; spec §4 and the
README record it. `player:isLocal()` is documented: never change synchronized state inside a branch on it. (Our
`getName()` on units, items and destructables returns localised text and should get the same warning; see the
comparison note, section 2.1.)

## 8. Importing a module does nothing in the game

**w3ts.** Loading the package runs game code:

```ts
// globals/index.ts: wraps every player slot at import
for (let i = 0; i < bj_MAX_PLAYER_SLOTS; i++) Players[i] = MapPlayer.fromHandle(Player(i));
// system/sync.ts: a trigger created when the module loads
private static eventTrigger = Trigger.create();
// hooks/index.ts: replaces the map's entry points at import
main = hookedMain; config = hookedConfig;
```

When that code runs depends on import order, and some objects must not be created during initialisation at all (w3ts
itself notes that quests, leaderboards and multiboards crash the game there).

**Ours.** A rule in the wrappers AGENTS.md: modules use literal requires and create no game objects when imported;
natives run only when a method is called. Requiring every wrapper module at the top of `main.yue` is always safe.

## 9. What the map owns is explicit

**w3ts.** A text tag with a lifespan is created like any other and keeps a live-looking wrapper after the game destroys
it:

```ts
const tag = TextTag.create()!;
tag.setText("+10 gold", 10, true);
tag.setPermanent(false);
tag.setLifespan(2);
// 2 s later the game destroys the tag, but `tag` still looks valid. Text tag ids are reused, so a later
// tag.setText(...) can change a different, newer tag.
```

The same applies to a sound after `killWhenDone()`, and `Rectangle.getWorldBounds()` wraps a new rect on every call
without saying that it must be destroyed.

**Ours.** Wrappers exist only for objects the map owns and destroys; anything the game ends by itself goes through a
helper that returns nothing:

```moon
TextTag.float "+10 gold", x, y, lifespan: 2, color: {255, 220, 0} -- returns nothing: no wrapper to go stale
Sound.playOnce "Sound\\Interface\\QuestNew.flac"                  -- released by the game when done
Effect.flash "Abilities\\Spells\\Human\\ThunderClap\\ThunderClapCaster.mdl", x, y

label = TextTag.create! -- permanent: valid until label\destroy!
bounds = Rect.worldBounds! -- documented as a new rect each call: destroy it
```

## 10. Arguments are checked before any native runs

**w3ts.** Most factories return `T | undefined`, which pushes a check or a `!` onto every call site, while some claim
non-null without checking (`Timer.create`, `Point.create`). Wide natives stay positional:

```ts
const s = Sound.create("Sound\\Interface\\QuestNew.flac", false, false, false, 10, 10, "DefaultEAXON");
const img = Image.create(path, 256, 256, 0, x - 128, y - 128, 0, 0, 0, 0, ImageType.Selection);
```

**Ours.** Factories return a wrapper or raise (`[wrappers] Sound.create: native returned nil`). Wide calls take an
options table whose unknown keys and wrong types fail before any native runs, and numbers are range-checked where the
game would misbehave:

```moon
s = Sound.create "Sound\\Interface\\QuestNew.flac", is3D: true
Sound.create path, is3d: true       -- error: [wrappers] Sound.create: unknown option 'is3d'
TextTag.float "hi", x, y, size: "10" -- error: [wrappers] TextTag.float: option 'size' expected a number
t\start 0/0, false, -> nil           -- error: [wrappers] Timer.start: expected a finite non-negative number
aoe = Image.create path, 256, 256, x, y, 1 -- centred on x, y and visible
```

## 11. Every release is tested, in Lua and in game

**w3ts.** There are no tests. Reading the source turned up bugs of the kind a forwarding test catches, for example:

```ts
public set invulnerable(flag: boolean) {
  SetItemInvulnerable(this.handle, true);          // ignores `flag`
}
public removeType(whichUnitType: unittype) {
  return UnitAddType(this.handle, whichUnitType);  // adds instead of removing
}
public setChannel(channel: number) {
  SetSoundDistanceCutoff(this.handle, channel);    // wrong native
}
```

**Ours.** 107 behaviour tests run the real modules against native doubles that record every call, plus Lua 5.3.6 syntax
checks, real Moonwell builds (normal and minified), LuaLS positive and negative fixtures, and a recorded in-game gate per
release. A typical test (`tests/item.lua`):

```lua
test('item mutations forward exact arguments', function()
    local i = Item.fromHandle({})
    checkSetters(i, {{'SetItemCharges', 'setCharges', 4}, {'SetItemVisible', 'setVisible', false},
        {'SetItemInvulnerable', 'setInvulnerable', true}}) -- shortened
    -- ...
end)
```

`checkSetters` asserts the native's name and the exact arguments, so `removeType`/`setChannel`-style mistakes fail.
One honest caveat: this test passes `true` to `setInvulnerable`, the same value w3ts hard-codes, so it would not have
caught that particular bug. Forwarding tests should use a value that differs from the obvious default. That is a small
item for the v0.3.1 patch.

## 12. Behaviour is measured on the current game

**w3ts.** Many notes come from older JASS documentation and are not re-checked against current patches. One disagrees
with what we saw: w3ts says `FinishUbersplat` does nothing, while in our v0.3.0 gate the splat faded right after
`splat\finish!` (a control run is still due).

**Ours.** Each release gate records what Warcraft 3.0.0.24268 actually did, and the README states it with the evidence:
Chain Lightning fades by itself while Drain Life stays; `lightning:setColor` stores the colour but shows no visible
change; effects attached to items and destructables are not drawn; the weak cache collects unreferenced widget wrappers.
When w3ts and our measurements disagree, ours are dated and reproducible with `examples/gate.yue`.
