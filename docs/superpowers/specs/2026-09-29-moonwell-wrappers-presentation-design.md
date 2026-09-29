# Moonwell Presentation Wrappers (wrappers v0.3.0) — Design

- **Date:** 2026-09-29
- **Status:** Approved in chat on 2026-09-29; awaiting review of this written spec.
- **Builds on:** `2026-09-28-moonwell-wrappers-design.md` (v0.1.0) and `2026-09-28-moonwell-wrappers-broad-design.md`
  (v0.2.0, tag commit `7baa81e`). Everything in those specs still applies unless this one changes it explicitly.
- **Target:** wrappers `v0.3.0` in the separate `mdlsvensson/moonwell-wrappers` repository. No Moonwell CLI change.

## 1. Intent and scope

The backlog item "UI and presentation wrappers" is split into three releases, each with its own spec, plan and in-game
gate:

- **A, v0.3.0 (this spec): world presentation and enumeration.** TextTag, Sound, Lightning, Image, Ubersplat and
  FogModifier; deeper Effect; item and destructable enumeration in a rect.
- **B, v0.4.0: classic UI.** Dialog and button, multiboard, leaderboard, quest, timer dialog.
- **C, v0.5.0: frames.** The `BlzFrame` API, with its own ownership design (TOC/FDF loading, parent trees, local
  frames).

A comes first: it stays closest to v0.2.0's handle and cleanup patterns. The release is additive. The only observable
change to an existing call is that `Effect.attach` accepts any Widget, so a wrong argument now fails with
`expected Widget wrapper` instead of `expected Unit wrapper` (recorded in the CHANGELOG).

## 2. Modules and dependencies

```text
src/wrappers/
  texttag.lua sound.lua lightning.lua image.lua ubersplat.lua fogmodifier.lua    (new)
  effect.lua item.lua destructable.lua                                           (extended)
  internal/options.lua                                                           (new)
```

Still no umbrella module, no globals, literal requires only, and no game-object creation at import. Every new module
imports only `wrappers.internal.*`. Player, Unit, Rect and Widget arguments are converted with `Handle.unwrap` and
`Handle.unwrapWidget` (v0.2.0 rule: a module imports another public module only to return its wrappers). Item and
Destructable keep their v0.2.0 imports; Effect still imports no public module.

## 3. Lifetime

### Owned wrappers only

Every new class has a strong cache until `destroy()`, like Timer and Effect. A wrapper exists only for an object the map
owns and destroys explicitly; the game never ends it on its own:

- `TextTag.create` makes the tag permanent, and the wrapper has no lifespan, fadepoint, age or permanence setters. (A
  non-permanent tag is destroyed by the game when its lifespan ends, and text tag ids are reused, so a wrapper of one
  could later resolve to an unrelated tag.)
- A `Sound` wrapper is never passed to `KillSoundWhenDone`. `destroy()` kills it immediately.

Wrapping preplaced or foreign handles with `fromHandle` transfers no cleanup duty, as in v0.1.0.

### Fire-and-forget helpers

One-shot presentation goes through static helpers that **return nothing**, so no wrapper can go stale:

- `TextTag.float(text, x, y, options?)` (section 5.1).
- `Sound.playOnce(path, options?)` (section 5.2).
- `Effect.flash(model, x, y)` and `Effect.flashOn(model, widget, attachmentPoint)` (section 6).

## 4. Local visibility

`setVisibleFor(player)` exists on TextTag, Image and Ubersplat. It calls the class's show native with
`Handle.unwrap(player, 'Player', operation) == GetLocalPlayer()`, so every machine makes the same call with a
machine-local boolean and only local visuals differ. `show(b)` afterwards sets the same value for everyone.

- **Lightning has no `setVisibleFor`.** It has no show native, and a local alpha would make `GetLightningColorA` return
  different values on each machine.
- **Sound** has `playFor(player)`, which calls `StartSound` only on that player's machine. The sound itself is still
  created everywhere.
- The helpers' `player` options (TextTag.float, Sound.playOnce) use the same comparison.

README repeats v0.2.0's warning: branching on the local player must not change synchronized game state.

## 5. New classes

All have the common members: `fromHandle(raw)`, `getHandle()`, `isDisposed()`, `.handle`. Factories raise
`native returned nil`. Colors are integers 0–255 unless stated; the natives' own value ranges are passed through, and
README states the range per method.

### 5.1 TextTag (`wrappers.texttag`, strong)

- `create() -> TextTag`: CreateTextTag, then SetTextTagPermanent(raw, true).
- `setText(text, size)`: SetTextTagText with height `size * 0.023 / 10` (the TextTagSize2Height conversion, computed in
  Lua).
- `setColor(r, g, b, a)` (SetTextTagColor).
- `setPosition(x, y, heightOffset)` (SetTextTagPos).
- `setPositionOnUnit(unit, heightOffset)` (SetTextTagPosUnit): places the tag once at the unit; it does not follow it.
- `setVelocity(xvel, yvel)` (SetTextTagVelocity, native units).
- `setSuspended(b)`, `show(b)` (SetTextTagVisibility), `setVisibleFor(player)`.
- `destroy()` (DestroyTextTag).

`TextTag.float(text, x, y, options?)`:

| Option         | Type            | Default              | Effect                                                    |
| -------------- | --------------- | -------------------- | --------------------------------------------------------- |
| `size`         | number          | 10                   | as `setText`                                              |
| `heightOffset` | number          | 0                    | SetTextTagPos                                             |
| `color`        | `{r, g, b, a?}` | `{255, 255, 255}`    | SetTextTagColor; `a` defaults to 255                      |
| `speed`        | number          | 64                   | SetTextTagVelocity, see below                             |
| `angle`        | number, degrees | 90                   | SetTextTagVelocity, see below                             |
| `lifespan`     | number ≥ 0      | 2                    | SetTextTagLifespan                                        |
| `fadepoint`    | number ≥ 0      | 1                    | SetTextTagFadepoint                                       |
| `player`       | Player          | nil (everyone)       | visibility as `setVisibleFor`                             |

The velocity is `v = speed * 0.071 / 128` (the TextTagSpeed2Velocity conversion), passed as `v * cos(angle)` and
`v * sin(angle)` with the angle in radians. It calls SetTextTagPermanent(raw, false). If CreateTextTag returns nil (the game's text tag limit), `float` does
nothing: missing floating text is not an error.

### 5.2 Sound (`wrappers.sound`, strong)

- `create(path, options?) -> Sound`: CreateSound. Options: `looping`, `is3D`, `stopWhenOutOfRange` (booleans, false),
  `fadeIn`, `fadeOut` (integers, 10), `eax` (string, `"DefaultEAXON"`).
- `play()` (StartSound), `playFor(player)` (section 4), `stop(fadeOut?)` (StopSound with killWhenDone false and
  `fadeOut or false`).
- `setVolume(volume)` (0–127), `setPitch(pitch)`, `setChannel(channel)`, `setPosition(x, y, z)`,
  `attachToUnit(unit)` (AttachSoundToUnit), `setDistances(min, max)`, `setDistanceCutoff(cutoff)`.
- `getDuration() -> integer` (GetSoundDuration, milliseconds).
- `destroy()`: StopSound(raw, true, false).

No `isPlaying`: GetSoundIsPlaying answers differently on each machine (and differs after `playFor`).

`Sound.playOnce(path, options?)`: options `volume` (integer, 127), `x`, `y`, `z` (numbers; giving `x` or `y` requires
both, makes the sound 3D and sets its position, `z` defaults to 0) and `player`. Steps: CreateSound(path, false, is3D,
false, 10, 10, "DefaultEAXON"), SetSoundVolume, SetSoundPosition if 3D, then StartSound on every machine and
KillSoundWhenDone. With `player`, the other machines set volume 0 before starting, so every machine starts and kills the
sound identically. A nil from CreateSound raises (unlike `float`: a missing sound file is a map error).

**First-play risk.** A sound whose file was not loaded before may be silent the first time it starts. The gate checks
it (section 9, in-game step 2); if it is silent, the fix (Preload, or a deferred start) is decided with the maintainer and this
section revised before release.

### 5.3 Lightning (`wrappers.lightning`, strong)

- `create(code, x1, y1, z1, x2, y2, z2, checkVisibility?) -> Lightning` (AddLightningEx; `checkVisibility` defaults to
  false).
- `move(x1, y1, z1, x2, y2, z2, checkVisibility?) -> boolean` (MoveLightningEx).
- `setColor(r, g, b, a) -> boolean` (SetLightningColor; **numbers 0–1**, the native's range).
- `destroy()` (DestroyLightning).

### 5.4 Image (`wrappers.image`, strong)

- `create(path, width, height, x, y, imageType) -> Image`: CreateImage(path, width, height, 0, x - width / 2,
  y - height / 2, 0, 0, 0, 0, imageType), then SetImageRenderAlways(raw, true) and ShowImage(raw, true), because a new
  image is otherwise not drawn. `x, y` is the **center**. README lists the image types (1 selection, 2 indicator,
  3 occlusion mask, 4 ubersplat) and the file requirements.
- The wrapper keeps its size in a private weak-keyed table (never iterated), so `setPosition(x, y, z?)` also centers:
  SetImagePosition(raw, x - width / 2, y - height / 2, z or 0).
- `show(b)` (ShowImage), `setVisibleFor(player)`, `setColor(r, g, b, a)` (SetImageColor),
  `setConstantHeight(flag, height)`, `setAboveWater(flag, useWaterAlpha)`, `setType(imageType)`.
- `destroy()` (DestroyImage).

The gate confirms the centering (section 9, in-game step 4).

### 5.5 Ubersplat (`wrappers.ubersplat`, strong)

- `create(name, x, y, options?) -> Ubersplat`: CreateUbersplat(x, y, name, r, g, b, a, forcePaused, noBirthTime), then
  SetUbersplatRenderAlways(raw, true). Options: `color` (`{r, g, b, a?}`, white), `forcePaused`, `noBirthTime`
  (booleans, false).
- `show(b)` (ShowUbersplat), `setVisibleFor(player)`, `finish()` (FinishUbersplat), `reset()` (ResetUbersplat).
- `destroy()` (DestroyUbersplat).

### 5.6 FogModifier (`wrappers.fogmodifier`, strong)

- `radius(player, fogstate, x, y, radius, useSharedVision, afterUnits) -> FogModifier` (CreateFogModifierRadius).
- `rect(player, fogstate, rect, useSharedVision, afterUnits) -> FogModifier` (CreateFogModifierRect). The modifier does
  not own the Rect.
- A new modifier is stopped, as with the native: `start()` (FogModifierStart), `stop()` (FogModifierStop).
- `destroy()` (DestroyFogModifier).

## 6. Effect

- `attach(model, target, attachmentPoint)` accepts any Widget (Unit, Item, Destructable) via `Handle.unwrapWidget`.
  AddSpecialEffectTarget already takes a widget.
- `setColor(r, g, b)` (BlzSetSpecialEffectColor), `setAlpha(a)` (BlzSetSpecialEffectAlpha),
  `setPlayerColor(player)` (BlzSetSpecialEffectColorByPlayer).
- `setTimeScale(scale)`, `setOrientation(yaw, pitch, roll)` (radians), `setHeight(height)`, `setZ(z)`.
- `playAnimation(animtype)` (BlzPlaySpecialEffect).
- `Effect.flash(model, x, y)`: AddSpecialEffect then DestroyEffect at once, which plays the death animation.
  `Effect.flashOn(model, widget, attachmentPoint)`: the same with AddSpecialEffectTarget. Both raise if the native
  returns nil and return nothing.

No position getters: `BlzGetLocalSpecialEffect*` values are local to each machine.

## 7. Enumeration

- `Item.enumInRect(rect, filter?) -> Item[]` (EnumItemsInRect).
- `Destructable.enumInRect(rect, filter?) -> Destructable[]` (EnumDestructablesInRect).

Each validates the filter (function or nil) first, then calls the native with a nil boolexpr (the same line-local
diagnostic exception as v0.2.0's nil filters) and a callback that **only collects** GetEnumItem/GetEnumDestructable
handles. After the native returns, the handles are wrapped into a dense one-based array; if `filter` is given, it runs
on each wrapper as ordinary Lua and falsy results are dropped. An error in the filter propagates; nothing is
half-built, because the result is a fresh array.

## 8. Types, errors and validation

- Classes `MoonwellWrappers.TextTag`, `.Sound`, `.Lightning`, `.Image`, `.Ubersplat`, `.FogModifier`; option types
  `MoonwellWrappers.TextTagFloatOptions`, `.SoundOptions`, `.SoundPlayOnceOptions`, `.UbersplatOptions`, all with
  optional fields. Every `fromHandle` stays conservatively nullable.
- `internal/options.lua` validates an options table against a field spec: the table's type (table or nil), unknown keys
  (`[wrappers] TextTag.float: unknown option 'colour'`), each value's type (including Player wrappers and color arrays
  of three or four integers), and fills defaults into a fresh table (the caller's table is not modified).
- Also validated: wrapper identity and disposal of receivers and arguments, filter types, and finite non-negative
  `lifespan` and `fadepoint`. All other value domains are left to Warcraft, as in v0.2.0.

## 9. Verification and release gate

Automated (the wrappers repo's CONTRIBUTING suite, extended):

1. Native-double tests for every new method: exact native and arguments, conversions (text size, speed and angle),
   image centering on create and move, rejection of disposed receivers and arguments.
2. Options: defaults, unknown keys, wrong types, color arrays, the caller's table left unmodified.
3. Fire-and-forget: `float` sets permanence false with lifespan and fadepoint, and does nothing when CreateTextTag
   returns nil; `playOnce` always starts and kills the sound, and sets volume 0 on other machines with `player`;
   `flash`/`flashOn` create and destroy immediately.
4. `setVisibleFor` and `playFor` with a stubbed GetLocalPlayer, as the local and as another player.
5. Enumeration: the native callback only collects; the filter runs after; a throwing filter propagates; a non-function
   filter is rejected before the native runs.
6. Lua 5.3.6 syntax check.
7. LuaLS positive and negative fixtures: options types, `Effect.attach` accepting Item and rejecting Timer, nullable
   `fromHandle` narrowing.
8. Integration: fresh Moonwell 0.5.0 consumer, check and build (normal and minified); importing only `wrappers.texttag`
   bundles no other public module.

In-game (maintainer, Warcraft III Reforged 3.0.0.24268, World Editor 3.00), extending `examples/gate.yue`, normal and
minified:

1. An owned text tag is created, changed and destroyed; `float` rises and fades; `float` for another player does not
   show locally.
2. An owned sound plays, stops, and plays at a 3D position; `playOnce` of a file not loaded before is audible on its
   first call (section 5.2); `playFor` for another player is silent.
3. Lightning is drawn, moved, recolored and destroyed.
4. An image appears centered on a marked point, is recolored and moved, and `setVisibleFor` another player hides it.
5. An ubersplat is shown, finished and destroyed.
6. A radius and a rect fog modifier reveal their areas; stopping one restores the fog.
7. Effect color, alpha, player color, time scale, orientation, height and `playAnimation` take effect; `attach` works on
   an Item; `flash` plays the death animation.
8. `Item.enumInRect` and `Destructable.enumInRect` with a filter return the expected objects.
9. World Editor opens the packed map. Then the tag consumption gate.

The multiplayer effects of `setVisibleFor`, `playFor` and the `player` options join Moonwell's pre-1.0 online checks and
are recorded as deferred, not passed.

## 10. Plan phasing

One implementation plan, written in Moonwell's `docs/superpowers/plans/`, with a review after each phase:

1. `internal/options.lua` and TextTag.
2. Sound.
3. Lightning, Image, Ubersplat and FogModifier.
4. Effect depth and flash.
5. Item and Destructable enumeration.
6. README API reference, CHANGELOG, CONTRIBUTING gate, `examples/gate.yue`, integration and LuaLS fixtures, the wrappers
   repo's AGENTS.md, and Moonwell's AGENTS.md state and backlog (the UI item split into releases B and C).

## 11. Out of scope

Classic UI (release B) and frames (release C). Getters for machine-local values (GetSoundIsPlaying,
`BlzGetLocalSpecialEffect*`). Wrappers for expiring text tags or sounds. Lightning local visibility. Location, ability
and buff wrappers, Blz object fields, w3ts compatibility, code generation, schedulers, implicit cleanup, resource pooling
and the wc3-lib port, as in v0.2.0. Wrapper calls inherit native synchronization requirements.
