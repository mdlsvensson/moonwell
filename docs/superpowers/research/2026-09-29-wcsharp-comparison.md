# WCSharp compared with moonwell-wrappers (2026-09-29)

The same exercise as `2026-09-29-w3ts-comparison.md`, for the C# library WCSharp: what it is, what we learn (especially
about the issues our gates found), where our wrappers are ahead or behind, and what it changes in the w3ts conclusions
(section 8). Compared: WCSharp 3.3.9 (`Orden4/WCSharp` master, commit `77fbe8e`, 2026-04-05; its generated `Docs/`
folder was not read) and moonwell-wrappers v0.3.0 (commit `5da96d8`). Nothing here is a decision.

## 1. What WCSharp is

- C# compiled to Lua with CSharp.lua. About 47,000 lines of C#, in two very different layers.
- **WCSharp.Api (29,000 lines, 13,000 of them generated ability field accessors):** a zero-cost API. Every
  Warcraft handle type is a C# class (`unit`, `timer`, `effect`...) whose members are `extern` and carry a CSharp.lua
  template, for example `/// @CSharpLua.Template = "GroupAddUnit({this}, {0})"`. The compiler inlines the native call:
  there are no wrapper objects, no cache, no validation, and the "object" at run time is the raw handle itself.
  `Dispose()` maps straight to the destroy native. Base classes follow the game's hierarchy (`handle`, `agent`,
  `widget`). `Common.cs` and `Blizzard.cs` expose every native and BJ function; `AI.cs` exposes common.ai natives such
  as `UnitAlive` and `UnitInvis`.
- **Systems (about 13,000 lines):** Timers (one native timer at 1/32 s multiplexing every action), Events (one trigger
  per event type, dispatching to registrations filtered by unit, type, player, ability...), Buffs and Auras, Missiles,
  Knockbacks, Dummies (recycled dummy casters), Lightnings, Effects (timed destruction), Sync (chunked
  `BlzSendSyncData`), SaveLoad (Preload file I/O, hashed), DateTime (synchronised real time), JSON, W3MMD, and Shared
  utilities (`Delay`, `Util`, group extensions).
- Tooling: a Roslyn-based checker that compares every API template's argument count and types with `common.j`, a
  generator for jassdoc-based XML documentation, and a field-accessor generator. No behaviour tests.
- Its per-native lore is thin compared with w3ts: the API carries almost no prose. Its knowledge sits in the systems'
  comments and in design choices.

## 2. What we learn, against the issues our gates found

| Our finding or open question | What WCSharp says or does | Takeaway |
| --- | --- | --- |
| `UnitAlive` is missing from our natives, so `isAlive()` avoids it. | Uses `UnitAlive` in its API (`unit.Alive`) and on hot paths of its Lightning and Missile systems. | Second independent library relying on it at run time: very likely callable from map Lua. The probe stays, now with high expectations. |
| `Effect.flash` creates and destroys at once; the gate's thunder clap showed. | `EffectSystem` exists "to circumvent issues with certain special effects not showing up if deleted immediately in Reforged": it destroys after a delay (default one tick). | **New risk:** `Effect.flash` shows only models whose death animation plays; others may show nothing. Probe with a few models; document; consider an optional delay. |
| `lightning:setColor` showed no visible change on Drain Life (green to red). | The Lightning system fades bolts by lowering alpha through `SetLightningColor` every tick, and documents a channel of 0 as removing that colour, so the colour is a multiplier on the bolt's texture. | Suggests `SetLightningColor` does work, at least for alpha. Our test may have been poor: red times a green texture is little or nothing, and lightning likely draws additively. Probe alpha on Drain Life and a tint on a light-coloured bolt before keeping the README's "do not rely on it". |
| Lightning heights: absolute or relative? (the gate's refuted hypothesis). | Always passes absolute z: `GetLocationZ` of the ground, or `BlzGetUnitZ + GetUnitFlyHeight` of a unit, plus an offset. | Confirms our reading that `Lightning.create` and `move` take absolute z. Worth a README line: on uneven ground, add the terrain height. |
| Chain Lightning faded by itself. | Nothing; its system creates with `AddLightning` and keeps any type. | Nothing new. |
| Effects attached to items and destructables are not drawn. | `widget.AddSpecialEffect(model, point)` is offered on every widget, with no note. | Nothing new; our measurement stands. |
| Image off-centre, image invalid path, ubersplat `finish`. | Plain templates, no notes. | Nothing new. |
| Weak cache and identity. | No wrappers, so no cache: the handle is the object, and handles are used directly as dictionary keys. `GetHandleId` is marked obsolete everywhere: "prone to desyncs. Use the object itself as a key instead." | **New, important:** in Lua, handle ids can differ between machines, most likely because an id is recycled only once every reference is released, and in Lua that depends on each client's garbage collector (our explanation; WCSharp gives none). Our wrappers never expose ids, and our registry keys by the handle, which WCSharp's whole design also relies on. The README should warn against `GetHandleId` in synchronized logic. |
| Errors inside callbacks are silent. | `TimerSystem` catches errors only after `EnableDebug()`; `Delay` and the event dispatcher always catch them but print only after `EnableDebug()`, which the documentation recommends leaving off in release builds. So in a release build their errors are silent, or swallowed. | Confirms the problem is real and known. Our always-on, per-callback boundary goes further (section 3). |
| Text printed during map load is lost; gates start from a zero-second timer. | `Delay.Add` runs work after a zero-second timer, also "to circumvent various issues, such as unit AI locking up if you give them a new order at the same time as they start an attack". | **New caveat:** issuing an order to a unit inside its own attack event can lock its AI; delay the order by a zero-second timer. |
| `os` functions exist in map Lua (Plan 3a probe). | `WcDateTime` uses `os.time`/`os.date`, warns that the local time can desync, and synchronises it by sending every player's time and picking the earliest, latest or average. | Consistent with our probe; input for 4d. |

### 2.1 Other things worth knowing

- **Unit removal can be observed.** WCSharp raises its own "unit is created" and "unit is removed" events from a
  region covering the world bounds: entering means created, leaving means removed. If Warcraft fires the leave event
  for units the game removes by itself (decay, `RemoveUnit`), our Unit registry could dispose such wrappers instead of
  leaving live-looking wrappers of dead units. This needs an in-game probe before any design.
- **Instant facing.** Its `Facing` setter uses `BlzSetUnitFacingEx`, which turns the unit at once. Our
  `unit:setFacing` uses `SetUnitFacing`, which turns at the unit's turn rate. Document ours; an instant variant is a
  candidate.
- **Sync (for 4d):** `BlzSendSyncData` carries at most 255 characters. WCSharp sends a JSON header plus 240-character
  packets, and relies on its tests suggesting that one player's messages arrive in order and are never interleaved; it
  still keeps one message per player in flight. Sync events are registered for every player.
- **Save and load (for 4d):** Preload file I/O as in w3ts, but reading back through the tooltips of about 30 standard
  abilities (restored afterwards), base64 content, and a hash of the data and the player's name against tampering. Save
  and load run only on the owning player's machine, and the result is shared with `SyncSystem`.
- **Event dispatch (for 4d or a wrappers events design):** one native trigger per event type; the dispatcher runs as the
  trigger's condition and returns false (conditions are cheaper than actions); registration changes made during a
  dispatch are deferred until it finishes; unregistering is supported because the native registration never changes.
- **Timers (for 4d):** one periodic native timer at 1/32 s drives everything; roots per timeout; tick-interval changes
  apply after a zero-second delay. Recommends power-of-two tick intervals.
- **Missiles** keep their own coordinates and move the effect every tick; they never read an effect's position back
  (consistent with our "no machine-local getters" rule). Terrain height comes from `GetLocationZ` on a reused location,
  used in synchronized state; w3ts marks `GetLocationZ` as possibly asynchronous (deformations, destructables).
- **Dummies** are recycled after a delay (default 2 s) and parked at (0, 0); a dummy's cast ability is removed after
  its cast ends.
- **An API checker** compares each template with `common.j` (argument count and types). A similar check over our
  wrappers against `cli/data/natives.json` would catch wrong-native and wrong-arity mistakes statically.

## 3. Where our implementation is ahead

WCSharp.Api makes a different trade than w3ts: no wrappers at all. That removes some of w3ts's problems (stale caches)
and keeps others (dead handles, no validation). Compared point by point with the w3ts list in
`2026-09-29-wrappers-advantages.md`:

1. **Disposal tracking (holds).** `Dispose()` is the native; the variable keeps the dead handle, and nothing stops
   further calls.
2. **Error boundary (holds, refined).** WCSharp prints errors only in debug builds, and its guards cover a whole batch:
   one exception ends the batch. In `Delay`, the queued actions after the failing one are then dropped without
   running (the list is cleared after the loop). In `TimerSystem`, the roots after the failing one miss that tick. In an
   event dispatch, the later registrations miss that event. Ours isolates every callback: one failure affects nothing
   else.
3. **Filters and conditions (neutral).** The API passes a boolexpr through (default null); the systems create one
   condition per trigger for the program's lifetime. No leak, but no protection either.
4. **Lifecycle guards (holds).** Native `triggeraction`/`triggercondition` handles only.
5. **Timer passed to callback (holds for the API; its TimerSystem passes its own `Timer` object).**
6. **Identity (revised):** WCSharp has no identity problem, because there is no wrapper. Our strong and weak caches are
   the cost of wrapping, not an advantage over a zero-cost API. What remains ours: wrappers that know they are disposed.
7. **Desync discipline (holds, narrower gap).** WCSharp names machine-local members (`LocalX`, `LocalPitch`,
   `player.LocalPlayer`), marks `GetHandleId` obsolete and warns on local time. Better than w3ts. It still exposes
   `BlzGetUnitZ` as `unit.Z` and `GetLocalPlayer` freely; ours exposes no machine-local getter at all.
8. **Nothing at import (mostly holds).** C# static initialisers run on first use of a class: `Delay` creates its timer
   and `TimerSystem` starts its periodic timer then. Lazier than w3ts, but a first use during map initialisation still
   creates objects then.
9. **Explicit ownership (holds).** `Util.CreateFloatText` and `CreateDamageText` return a text tag with a four-second
   lifespan: once the game removes it, the returned handle can name a newer tag. Ours returns nothing.
10. **Validation (holds).** Templates pass arguments straight to natives. The C# compiler checks types at build time,
    which is a real advantage over Lua; values (ranges, NaN, unknown option keys) are unchecked.
11. **Tests and gates (holds).** No behaviour tests; its static API checker is a good idea we could adopt.
12. **Measured behaviour (holds).** Its knowledge comes from real maps but is not recorded per game version.

## 4. Where WCSharp is ahead

1. **Zero cost.** No table per handle, no registry lookup, no pcall: calls compile to the native itself. For hot paths
   (missiles at 32 ticks per second) this matters, and our wrappers should be judged by it before 4d builds systems on
   top of them.
2. **Complete API.** Every native and BJ function, common.ai natives, skins, typed object-data fields (the ability
   accessors list which abilities use each field), and every handle type, including frames, multiboards, camera
   setups and game caches.
3. **Static types.** C# checks handle types, argument counts and overloads at build time; LuaLS gives us warnings only.
4. **Gameplay systems.** Buffs, auras, missiles, knockbacks, dummies, events, sync, save and load, synchronised time,
   W3MMD: essentially the scope planned for our 4d port of `wc3-lib`, already built and used in maps.
5. **Event multiplexing** that supports unregistering, which native triggers cannot do.

## 5. Hazards noticed while reading

Not necessarily bugs in their context, but lessons for our designs:

- `Delay.ExecuteAll` drops the rest of its queue when one action throws (the list is cleared after the loop).
- `TimerSystem.Action` in release mode lets one failing root stop the tick for the roots after it, and a root's
  cleanup (`Active == false`) is then skipped until the next tick.
- `SyncSystem.HandleSyncPacket` logs a packet without header and then still uses the missing message (a null
  reference in C#).
- `SaveSystem` lists `1098085480` for five different abilities (`Asth`, `Agyv`, `Aast`, `Abtl`, `Sbtl`), so those entries
  all reuse one tooltip slot.
- `Util.GetZ` (`GetLocationZ`) feeds synchronized missile state, although w3ts marks that native as possibly async.

## 6. Gaps in ours that WCSharp shows

In addition to the w3ts list (comparison §6): `BlzSetUnitFacingEx` (instant facing), `CreateUnitByName` and corpses,
dead destructables and `CreateDestructableZ`, `AddSpellEffect` by model or ability id on widgets (`AddIndicator` too),
`GetSoundFileDuration(path)`, `CreateMIDISound` and SLK-based sounds, lightning colour getters, unit events for created
and removed units.

## 7. Follow-ups for the maintainer to choose from

1. **Extend the probe run** (AGENTS.md next work item 1): lightning alpha on Drain Life and a tint on a light-coloured
   bolt; `Effect.flash` with several models, including one without a death animation; whether decay and `RemoveUnit`
   fire the world-bounds leave event.
2. **Extend the v0.3.1 doc patch:** `GetHandleId` desyncs; lightning z is absolute; `setFacing` turns at the unit's turn
   rate; orders issued inside a unit's attack event; `Effect.flash` depends on the death animation.
3. **A static native-call check** for the wrappers, like WCSharp's API checker, against `cli/data/natives.json`.
4. **4d design inputs:** event multiplexing with deferred updates and unregistering, one multiplexed timer, sync
   packets, save and load, synchronised time, dummy recycling; plus a performance budget for wrappers on hot paths.
5. **Candidate addition:** automatic disposal of Unit wrappers on removal, if the probe shows the leave event fires.

## 8. What changes in the w3ts conclusions

- **UnitAlive:** evidence is stronger (two libraries); the probe remains.
- **Effect.flash:** new risk (section 2); w3ts did not raise it.
- **Lightning colour:** our "no visible change" conclusion is weakened; WCSharp's fading relies on the native.
- **Identity (w3ts advantages §6):** identity guarantees are a cost of wrapping. Against a zero-cost API they are not an
  advantage; against w3ts's weak-keyed wrappers they still are.
- **Error boundary (advantages §2):** strengthened: WCSharp shows the opt-in, batch-level alternative and its failure
  modes.
- **Desync discipline (advantages §7):** add `GetHandleId`; note that naming local members (`LocalX`) is a reasonable
  middle ground that w3ts lacks.
- **Breadth and systems (w3ts comparison §4):** WCSharp is broader still, and its systems are the closest prior art for
  4d.
- **Tests (advantages §11):** neither library has behaviour tests; WCSharp's static API checker is worth copying.

## 9. Verified in game (probe run, 2026-09-29)

Two runs of the probe map (`../wrappers-gate/src/probe.yue`, results in `../wrappers-gate/PROBE-RESULTS.md`) on
Warcraft III 3.0.0.24268 answered the open questions from this note and the w3ts note:

| Question | Answer on 3.0.0.24268 |
| --- | --- |
| `UnitAlive` callable from map Lua | Yes; it agrees with `isAlive()` before and after a kill |
| World-bounds leave on unit removal (WCSharp's "unit is removed") | No: not for `RemoveUnit`, an exploded death, a summoned timed-life death, or a normal death left 120 s to decay (the corpse's disappearance was not checked on screen); enter did fire at creation |
| `SetLightningColor` visible | No: neither colour (v0.3.0 gate) nor alpha 0.2 on Drain Life |
| Lightning types that fade by themselves | Chain Lightning (v0.3.0 gate), Healing Wave (`HWPB`) and Spirit Link (`SPLK`); Drain Life stays |
| `FinishUbersplat` | Works: the splat fades, while a control splat stays (w3ts's "does nothing" is outdated) |
| `ResetUbersplat` after `finish` | No visible effect: the splat does not come back |
| `StartSound` on a sound that is still playing | Cuts the playing sound off, and nothing plays |
| `CreateImage` with a wrong path | A non-nil image whose `GetHandleId` is -1 (a valid image had 8); `DestroyImage` on it does not crash |
| `Effect.flash` (create and destroy at once) | All six standard models tested showed, the same as when destroyed 0.1 s later; WCSharp's concern was not reproduced |

What this settles: `UnitAlive` can be used (Moonwell's natives list lacks it); automatic Unit disposal cannot be built on
the world-bounds leave event; the README's advice not to rely on `lightning:setColor` stands and now covers alpha; the
`Effect.flash` risk from section 2 did not materialise for standard models; `Image.create` cannot detect a wrong path by
nil, but can by handle id -1.
