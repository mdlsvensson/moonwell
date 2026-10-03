# w3ts compared with moonwell-wrappers (2026-09-29)

Context gathering, not a design: what the TypeScript library w3ts does, what we learn from it (especially about the
issues our own gates found), where our wrappers are ahead, where they are behind, and the gaps inside what we already
cover. Compared: w3ts 3.0.2 (`cipherxof/w3ts` master, a downloaded copy; types from `war3-types-strict` 1.33.0) and
moonwell-wrappers v0.3.0 (commit `5da96d8`). Nothing here is a decision; section 7 lists follow-ups for the maintainer.

**Revised after the WCSharp comparison** (`2026-09-29-wcsharp-comparison.md`, §8). Where this note's conclusions changed:

- `UnitAlive` (section 2): WCSharp relies on it too, so the evidence is stronger; the probe remains.
- `Effect.flash` (not raised here): WCSharp works around effects that do not show when destroyed at once in Reforged.
  Our flash depends on the model's death animation; the probe run now covers it.
- Lightning colour (section 2): WCSharp fades bolts through `SetLightningColor`'s alpha and treats the colour as a
  multiplier on the texture, which weakens our "no visible change" conclusion; the probe run now covers it.
- Identity (section 3, point 6): our caches are the cost of wrapping. They are an advantage over w3ts's weak-keyed
  wrappers, not over a zero-cost API such as WCSharp.Api, where the handle is the object.
- Desync discipline (section 3, point 7): add that w3ts's `Handle.id` returns `GetHandleId`, which WCSharp marks
  obsolete as prone to desyncs in Lua; ours exposes no handle id.
- Breadth and systems (section 4): WCSharp is broader still, and its systems are the closest prior art for 4d.

**Verified in game** (probe run, 2026-09-29; table in the WCSharp note, section 9): `UnitAlive` exists; `FinishUbersplat`
works and `ResetUbersplat` does not bring a finished splat back; replaying a sound that still plays cuts it off; a wrong
image path gives an image with handle id -1 whose `DestroyImage` does not crash (w3ts's crash warning did not hold);
`Effect.flash` showed for six standard models.

**Verified in game for release B** (`ui-init` probe, 2026-09-29, 3.0.0.24268; `../wrappers-gate/PROBE-UI-RESULTS.md`):
a dialog and a multiboard shown directly in `on_main` did not appear (w3ts's note holds); creating a quest, leaderboard
and multiboard in `on_main` did not crash, and each worked later (w3ts's crash is about global initialisation, not
probed); one `MultiboardSetRowCount` from 0 to 5 worked (the one-row-at-a-time note did not reproduce; the wrapper
steps anyway); a new multiboard's cells show an eye icon and no text.

## 1. What w3ts is

- TypeScript compiled to Lua 5.3 with TypeScriptToLua. About 6,000 lines of handle classes (about 30 classes in 28 files;
  Unit alone is 1,500 lines), 900 lines of systems and 200 lines of utilities. No tests of any kind.
- One base class, `Handle<T>`, with a single module-level `WeakMap` from raw handle to wrapper for every type (weak
  keys). `Foo.fromHandle(raw)` returns the cached wrapper or builds one. `Foo.create(...)` returns `Foo | undefined`;
  the older constructors (deprecated) raise instead.
- API style: properties (`unit.life = 50`, `timer.remaining`), `string | number` order arguments, optional arguments
  with defaults, and event helpers (`Unit.fromEvent()`, `Unit.fromEnum()`, `Unit.fromFilter()`,
  `Timer.fromExpired()`).
- `destroy()` only calls the native. The wrapper keeps its handle, stays in the cache and accepts further calls.
- Callbacks and filters are passed to the natives as they are (`Filter(fn)`, `Condition(fn)`, `ForGroup`), with no
  error boundary.
- Systems: chunked network sync (`SyncRequest` over `BlzSendSyncData`), file I/O through the Preload exploit, host
  detection, elapsed game time, base64 and binary readers and writers, `Color` with player colours and `|c` codes, an
  `OrderId` enum of 367 orders, `sleep()` as a Promise over a timer, and `main`/`config` script hooks.
- Its most valuable content is the per-native lore in its doc comments (`@bug`, `@note`, `@async`), largely carried
  over from the community JASS documentation.

## 2. What we learn, against the issues our gates found

| Our finding | What w3ts says | Takeaway |
| --- | --- | --- |
| Effects attached to items and destructables are not drawn (v0.3.0 gate). | Only a general note: a missing attachment point falls back to the model's origin. Nothing on items. | Nothing new. Our in-game evidence is more specific than its generic claim, which would predict an effect at the item's origin. Keep the editor-error backlog item. |
| Chain Lightning fades by itself; `setColor` shows no visible change. | w3ts has no Lightning class at all. | Our findings are original; nothing to compare. |
| The image circle under the footman was "slightly off-centre". | Image position is the bottom-left corner; the origin arguments shift it further negative; the texture's border should be fully transparent; only the Selection type draws above water. | Confirms our centring maths (position minus half the size). The small offset stays unexplained; w3ts's border note hints at edge pixels of the texture, but that is a guess. |
| (none yet) | An invalid image path makes `CreateImage` return an invalid image (id -1), not nil; `DestroyImage` on an invalid image may crash; destroying an image recycles its id at once. | **New risk for us:** `Handle.created` only catches nil, so `Image.create` with a wrong path would hand back a wrapper whose `destroy()` may crash the game. Needs an in-game probe. |
| A tree at the box edge was missed by `EnumDestructablesInRect`. | Nothing. | Nothing new. |
| We called `splat:finish()` at step 6 and the splat faded. | `FinishUbersplat` and `ResetUbersplat` "do nothing". | Contradicts w3ts, whose note likely comes from old documentation. The fade might instead be the splat's own lifetime, so a control run (a splat without `finish()`) would settle it. `reset()` is untested by us. |
| Errors in Lua timer and trigger callbacks are silent in game. | No error boundary anywhere. | We are ahead: our pcall boundary and `[wrappers] ... failed:` messages address a problem w3ts leaves open. |
| Text printed while the map loads never reaches the screen, so gates start from a zero-second timer. | Dialogs and multiboards cannot be shown at map init. `CreateQuest`, `CreateLeaderboard` and `CreateMultiboard` crash the game in global initialisation. | **Direct input for release B (classic UI).** Its design must say when these may be created and shown, and its gate must cover it. |
| `SetPlayerName` crashed lobby creation (map settings, config time). | Exposes a name setter for in-game use. | Nothing new; our finding concerns `config`, theirs is gameplay time. |
| `FirstOfGroup` can return nil while the group still holds removed units. | The same bug, with the same cause. | Confirms our README. |
| First-play and 3D sounds worked; `getDuration` can be 0 until loaded. | Sound limits: a sound handle "can only be played once" (probably: not again while it plays); at most four plays of the same path and 16 sounds in total; the same path needs 0.1 s between starts. `SetSoundPitch` behaves oddly. Position, distances, cones and velocity only apply to 3D sounds. `SetSoundPlayPosition` must follow `StartSound` immediately. | **New for our docs:** If w3ts is right, `Sound.playOnce` called in a burst silently drops sounds past these limits, and `sound:play()` on a sound that is still playing does not restart it. Worth a README note once a gate probe confirms it. |
| Weak cache: collected, stale, identity (v0.2.0 probe). | A weak-keyed `WeakMap` for every type. | See section 3: their choice loses identity for owned objects; ours keeps it. |
| `UnitAlive` is a common.ai native missing from our natives, so `isAlive()` uses `IsUnitType(UNIT_TYPE_DEAD)` and the type id. | `isAlive()` calls `UnitAlive` directly. | **New:** since w3ts is widely used, `UnitAlive` is probably callable from map Lua despite being a common.ai native. Probe it in game. If it exists, add it to `tools/natives/lua-extras.json` and consider using it. |

### 2.1 Other native caveats we did not have

Worth adopting in our README where the method exists, or keeping for later designs:

- **Locale-dependent getters.** `GetUnitName`, `GetItemName` and `GetDestructableName` (and item tooltips and icons)
  return each client's localised text. Our `unit:getName()`, `item:getName()` and `destructable:getName()` are fine to
  display but not to branch on in synchronized logic. This sits awkwardly with our rule "no getters for machine-local
  values"; at the least it must be documented.
- **Other machine-local getters.** `BlzGetUnitZ`, `GetLocationZ`, `BlzGetLocalSpecialEffectX/Y/Z`, the camera getters,
  `IsMultiboardMinimized` and frame text are local. We already have no effect position getters; release B/C must keep
  this in mind.
- **Units:** `GetUnitX/Y` of a unit loaded in a transport returns its position before loading. `SetUnitX/Y` does not
  cancel orders (`SetUnitPosition` does), and a unit with move speed 0 moves but its model does not. `SetUnitInvulnerable`
  relies on the `Avul` ability and crashes without it. `AddHeroXP` with a negative amount lowers experience but never the
  level. `SetUnitScale` uses only its X argument (our uniform `setScale(s)` already matches this).
- **Destructables:** `DestructableRestoreLife` with 0 or more than the maximum gives full life; below 0.5 gives 0.5.
- **Timers:** `TimerGetRemaining` can be wrong after a pause and restart. `GetExpiredTimer` may crash when no timer has
  expired; we already avoid it by passing the timer to its callback.
- **Triggers:** `TriggerEvaluate` runs every condition, in order, with no short-circuit, and a condition that errors
  counts as false (ours behaves the same). Destroying a trigger whose action is sleeping corrupts the handle stack; our
  "callbacks must not yield" rule already avoids it.
- **Groups:** `GroupEnumUnitsOfPlayer` includes units with Locust; the other enumerations do not. The "counted"
  enumerations misbehave with large counts (we do not expose them; keep it so).
- **Images:** the four image types draw in a fixed order (selection over occlusion mask over indicator over ubersplat);
  images of one type draw in creation order.
- **For release B (classic UI):** the init-time crashes above; `MultiboardSetRowCount` is only safe changing by one row
  at a time; every `MultiboardGetItem` handle must be released with `MultiboardReleaseItem`; leaderboards start with no
  rows; dialog buttons die with `DialogClear`/`DialogDestroy`, so button wrappers need an ownership rule; there are at
  most 255 game caches.
- **For 4d (wc3-lib port, save codes):** `BlzSendSyncData` carries at most 255 characters per call (w3ts sends chunks of
  244 plus a base64 header), and sync events should be registered only for playing human players. File I/O works only
  through `PreloadGenStart`/`Preload`/`PreloadGenEnd`: files are confined to `CustomMapData`, the extension must be
  `.txt` or `.pld`, each `Preload` call holds 259 characters, quotes must be escaped, files cannot be deleted, and
  reading uses `Preloader` plus an ability-icon trick. Host detection syncs each player's `os.clock()` time since
  config (the game's `os.clock` exists, which matches our Plan 3a probe).

## 3. Where our implementation is ahead

1. **Disposal is tracked.** Our `destroy()`/`remove()` invalidates the wrapper before the native call, is idempotent,
   and every later call fails with a clear `[wrappers] X is disposed`. w3ts's `destroy()` leaves the handle in the
   wrapper and the cache; later calls pass a dead handle to natives.
2. **Callbacks have an error boundary.** Timer callbacks, trigger actions and conditions run under pcall and print their
   error; ticks and later actions continue. w3ts's callback errors vanish silently, the pitfall our own gates hit.
3. **No filter or condition leaks.** w3ts passes `Filter(fn)` on every enumeration and `Condition(fn)` in
   `addCondition`, and never destroys them; with a fresh closure each time that likely leaks one boolexpr per call (not
   verified in game). We pass no native filters, run filters afterwards as ordinary Lua with errors propagating, and
   the trigger owns and destroys each condition's boolexpr.
4. **Stale-callback safety.** Timer schedules carry a generation; restarting or destroying a timer makes old callbacks
   no-ops. Trigger actions and conditions return removable tokens that work even during a firing. w3ts has
   `removeAction(triggeraction)` with raw natives only.
5. **The timer is passed to its callback.** No `GetExpiredTimer`, which w3ts itself flags as possibly crashing.
6. **Identity for owned objects.** Our strong caches keep Timer, Trigger, Group and the presentation wrappers identical
   for their whole life. Weak caches are used only for Unit, Item and Destructable, which the game can remove by itself,
   and the README warns against weak-keyed game tables. w3ts keys every type weakly by the handle userdata, so whether a
   wrapper survives depends on when the game's Lua releases that userdata, which neither library controls or documents.
   w3ts also never removes a destroyed object's entry; we drop it on disposal, so a later wrapper of a reused handle is
   always fresh.
7. **Desync discipline.** No getters for machine-local values (w3ts exposes them, marked `@async` in comments only);
   local visibility compares with the local player and passes the result to the same native on every machine;
   `Sound.playOnce` with `player` plays at volume 0 elsewhere so every machine starts and releases the sound alike.
   w3ts offers `MapPlayer.fromLocal()`, an easy desync footgun.
8. **Nothing happens at import.** Our modules create no game objects when required. w3ts creates a `Players` array, the
   `SyncRequest` trigger and a game-time timer hook at module load, and replaces the global `main` and `config`.
9. **Ownership is explicit.** Owned wrappers versus fire-and-forget helpers that return nothing (`TextTag.float`,
   `Sound.playOnce`, `Effect.flash`) mean no wrapper can outlive an object the game ends. In w3ts, a text tag with a
   lifespan or a sound with `killWhenDone` keeps a live-looking wrapper after the game destroys it, and text tag ids are
   reused. `Rect.worldBounds()` is documented as a new owned rect; w3ts's `getWorldBounds()` leaks one per call.
10. **Validation and non-null contracts.** Factories raise on nil, and argument types, option tables (unknown keys,
    wrong types) and ranges are checked before any native runs. w3ts returns `undefined` from most factories, and some
    (`Timer.create`, `Point.create`) do not check at all.
11. **Tests and gates.** 107 behaviour tests with native doubles, Lua 5.3.6 syntax checks, real Moonwell builds, LuaLS
    fixtures, and a recorded in-game gate per release. w3ts has no tests, and reading it turned up several bugs this
    kind of test catches (section 5).
12. **Measured behaviour.** Our README records what Warcraft 3.0.0.24268 actually did (lightning fading, colour,
    attachments, weak cache collection). Some of w3ts's notes are inherited from older patches and may be stale (see
    the ubersplat row in section 2).

## 4. Where w3ts is ahead

1. **Breadth.** Every common handle type: also Camera and CameraSetup, GameCache, Point, WeatherEffect, Dialog and
   DialogButton, Multiboard, Leaderboard, Quest, TimerDialog and Frame. Within shared classes it covers far more natives
   (Unit: about 240 members to our 87).
2. **Systems.** Network sync, file I/O, host detection, game time, binary encoding, colours, order ids. We have none;
   the ones a save-code system needs belong to the 4d port.
3. **Documentation of native quirks.** Its accumulated `@bug`/`@note` lore is broader than ours (section 2.1).
4. **Maturity and field use.** Years of real maps, including multiplayer. Our desync and online checks are deferred to
   before 1.0.
5. **Ergonomics.** Properties, overloads, defaults, event helpers (`Unit.fromEvent()` versus our
   `Unit.fromHandle(GetTriggerUnit())`), typed skins and fields, subclassable classes.
6. **Ecosystem.** npm package, docs site and a project template. This is not comparable for us given the "no Node.js"
   rule, and it is not a goal.

Neither is simply better: w3ts optimises for coverage and convenience, while ours optimises for correctness under
Warcraft's failure modes (silent errors, stale handles, desyncs) and pays for it with a much smaller surface.

## 5. Bugs in w3ts noticed while reading

They show the kind of mistake our native-double tests exist to catch, and warn against trusting its code blindly as a
reference:

- `Item.invulnerable = flag` always passes `true`.
- `Item.getField`/`setField` compare against `unit...field` prefixes, so item fields appear never to match.
- `Unit.removeType` calls `UnitAddType`.
- `Sound.setChannel` calls `SetSoundDistanceCutoff`.
- `Region.containsPoint` and `Leaderboard.hasPlayerItem` do not return their result.
- `Frame.getEventText` returns `BlzGetTriggerFrameValue` instead of the text.
- `Timer.create` and `Point.create` never check for a nil handle.

## 6. Gaps in ours, within what we already cover

Not a to-do list: many are deliberate omissions. Listed so a later design can choose.

- **Trigger registrations:** player state, key (`BlzTriggerRegisterPlayerKeyEvent`), mouse, sync, alliance change,
  game state, timer expire, filtered unit event, command and upgrade events. Dialog and frame events come with releases
  B and C.
- **Effect:** spell effects from ability data (`AddSpellEffectById`, `AddSpellEffectTargetById`), separate yaw, pitch
  and roll, `setTime`, sub-animations, `playWithTimeScale`, matrix scale.
- **Sound:** `getFileDuration(path)`, cone and velocity (3D), `setParamsFromLabel`, play position, dialogue and facial
  animation keys. `isPlaying` and `isLoading` stay out on purpose (machine-local).
- **Presentation:** `WeatherEffect` (`AddWeatherEffect`, enable, remove) was not part of v0.3.0.
- **Unit:** armor, attack cooldown, acquire range, turn speed, fly height, propulsion window, skins, Blz object fields,
  buffs (count, remove), timed-life pause, build and instant orders, rally points, stock, waygates, look-at, sleep.
- **Player:** handicap, visibility queries (`IsVisibleToPlayer`, fogged, masked), unit and structure counts, scores,
  `RemovePlayer`, tax rates.
- **Item:** abilities, tooltip and icon setters, user data, skins, drop id.
- **Convenience:** event helpers, order-id constants, colour codes.
- **Not in any plan so far:** Camera, CameraSetup, GameCache, Point (locations).

## 7. Follow-ups for the maintainer to choose from

1. **Doc patch (wrappers v0.3.1):** locale-dependent `getName()`; sound limits and replay; Locust in `enumOfPlayer`;
   `getRemaining` after pause; unit notes (transport position, `setX` versus `setPosition`, `Avul`, negative XP);
   restore-life semantics; image type layering and water.
2. **In-game probes:** whether `UnitAlive` exists in map Lua; what `CreateImage` returns for a wrong path and whether
   `image:destroy()` survives it; a splat without `finish()` as control for the ubersplat finding; `sound:play()` while
   the sound plays.
3. **Release B design inputs:** creation and display timing (init crashes), multiboard row changes, multiboard item
   release, dialog button ownership.
4. **Candidate additions:** WeatherEffect, spell effects (for example `Effect.flashSpell`), more Trigger registrations,
   event helpers.
5. **4d design inputs:** sync chunking and file I/O as summarised in section 2.1.

**Chosen 2026-09-29:** all five went onto Moonwell's AGENTS.md. Items 2 and 1 are "Next work" items 1 and 2; items 3
and 4 extend the release B backlog entry and add "Wrappers candidate additions"; item 5 is referenced from the next
sub-project choice. The maintainer also asked for `2026-09-29-wrappers-advantages.md`, which explains section 3 with
code examples.
