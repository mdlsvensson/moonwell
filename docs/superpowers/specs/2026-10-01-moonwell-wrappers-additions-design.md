# Moonwell Wrappers Additions (wrappers v0.8.0) — Design

- **Date:** 2026-10-01
- **Status:** The design was approved in chat on 2026-10-01; this written spec awaits review.
- **Builds on:** the wrapper specs for v0.1.0 to v0.7.0, the last being
  `2026-09-30-moonwell-wrappers-port-prerequisites-design.md`. Everything in them still applies unless this spec changes
  it explicitly: the error-location rule (errors point at the caller, no raising tail calls), the one-lookup prologue,
  the callback boundary, the opt-in rule (nothing is created at import) and the listener list of v0.7.0
  (`internal/listeners.lua`).
- **Inputs:**
  - the roadmap's phase 4, item 1, and the "Wrappers candidate additions" backlog entry (w3ts comparison §6 and §7);
  - two probes run on 2026-10-01, Warcraft III 3.0.0.24268 (`../wrappers-gate/PROBE-EXTRAS-RESULTS.md`); §2 gives what
    this design rests on;
  - the maintainer's choice of callbacks with the event's data for keyboard and mouse input.
- **Target:** wrappers `v0.8.0` in `mdlsvensson/moonwell-wrappers`. It still requires Moonwell 0.5.1 or later; every
  native it uses is in Moonwell's `natives.json`. The Moonwell CLI does not change.

## 1. Intent and scope

The wrappers additions the `wc3-lib` port did not need, in one release:

1. `wrappers.input`: listeners for keys and for the mouse (§3);
2. `wrappers.weathereffect`: an owned weather effect (§4);
3. `Effect.abilityArt`: the art an ability's data names, for the existing Effect constructors (§5);
4. four Trigger registrations: player state, alliance change, game state and timer expiry (§6);
5. `fromEvent()` on six classes, for the object an event is about (§7).

One existing behavior changes: the four Effect constructors raise for a model that is not a string (§5.2). Everything
else is an addition.

## 2. What the probes measured

On 3.0.0.24268, one machine. The full record is `../wrappers-gate/PROBE-EXTRAS-RESULTS.md`.

**Keys.**
- A held key repeats: holding Q for 1.6 s gave 37 key-down events and one key-up. The first repeat came after 0.5 s,
  then about 30 a second.
- The modifiers must match the registration exactly. With Shift held, the events registered for no modifier did not
  fire, down or up; one registered for Shift did, and `BlzGetTriggerPlayerMetaKey()` read 1.
- `BlzGetTriggerPlayerIsKeyDown()` tells down from up, and `BlzGetTriggerPlayerKey() == OSKEY_Q` holds.
  `GetHandleId(OSKEY_Q)` is the key's code, 81.
- Typing in the chat box fires no key event.

**Mouse.**
- Mouse down and up carry the world point under the cursor and the button, which compares equal to
  `MOUSE_BUTTON_TYPE_LEFT`, `_MIDDLE` or `_RIGHT`.
- A click on the interface (the minimap) fires too, with a world point.
- Mouse move fires 150 to 190 times a second while the mouse moves.

**Weather.**
- A weather effect is not drawn until `EnableWeatherEffect`. It keeps being drawn after its rect is removed, and
  stops when it is removed.
- An unknown effect id gives a handle with id -1, not nil; enabling and removing that handle did not crash.
- Weather handle ids are small and used again: after one is removed, the next effect has the same id and compares
  equal to the removed handle.

**Art from ability data.**
- `GetAbilityEffectById(ability, type, index)` reads a model path, or a lightning code for
  `EFFECT_TYPE_LIGHTNING` (`CLPB` for Chain Lightning). The index starts at 0.
- A missing entry, and an unknown ability, read `""`.
- Every index past the last entry reads the last entry, so the number of entries cannot be read.
- `AddSpellEffectById` returns nil when the ability has no such art. An effect it makes looked the same as one made by
  `AddSpecialEffect` with the path `GetAbilityEffectById` read.

**Trigger events.**
- Timer expiry: the trigger fires at every expiry, before the timer's own callback, also for a timer started without a
  callback and for a trigger registered after the timer started. `GetExpiredTimer()` is the timer.
- Player state: the event fires inside `SetPlayerState`, at every change to a value that satisfies the comparison
  (gold set to 1000, 1001 and 2000 fired each time with `>= 1000`; the same value set again did not).
  `GetTriggerPlayer()` is the player and `GetEventPlayerState()` the state.
- Game state: a time-of-day event fires when the comparison becomes true, by a set or by the clock; a set from 13 to
  14 with `>= 12` did not fire again.
- Alliance change: `TriggerRegisterPlayerAllianceChange(trigger, player, kind)` fires inside `SetPlayerAlliance` when
  that player's setting of that kind toward any player really changes. `GetTriggerPlayer()` is nil, and nothing names
  the other player. The generic `EVENT_PLAYER_ALLIANCE_CHANGED` fired only for the passive setting.

## 3. `wrappers.input`

### 3.1 API

```lua
local Input = require 'wrappers.input'

---Fires once when `player` presses `key`, with any modifier keys held.
---@param player MoonwellWrappers.Player
---@param key oskeytype
---@param callback fun(player: MoonwellWrappers.Player, meta: integer, repeated: boolean): ...
---@param options? {repeats: boolean?}
---@return MoonwellWrappers.InputListener
function Input.onKeyDown(player, key, callback, options)

---@param player MoonwellWrappers.Player
---@param key oskeytype
---@param callback fun(player: MoonwellWrappers.Player, meta: integer): ...
---@return MoonwellWrappers.InputListener
function Input.onKeyUp(player, key, callback)

---@param player MoonwellWrappers.Player
---@param callback fun(player: MoonwellWrappers.Player, x: number, y: number, button: mousebuttontype): ...
---@return MoonwellWrappers.InputListener
function Input.onMouseDown(player, callback)
function Input.onMouseUp(player, callback)      -- the same signature

---@param player MoonwellWrappers.Player
---@param callback fun(player: MoonwellWrappers.Player, x: number, y: number): ...
---@return MoonwellWrappers.InputListener
function Input.onMouseMove(player, callback)

---Removes a listener at once, even during a firing. Removing it twice does nothing.
---@param token MoonwellWrappers.InputListener
function Input.off(token)
```

```yuescript
import "wrappers.input" as Input

cast = Input.onKeyDown player, OSKEY_Q, (player, meta) ->
  castFor player if meta == METAKEY_NONE

Input.onMouseDown player, (player, x, y, button) ->
  print player\getName!, "clicked at", x, y if button == MOUSE_BUTTON_TYPE_LEFT

Input.off cast
```

Listeners are for one player. The events are synced: a listener runs on every machine, in the same order, some frames
after the key or the mouse moved, so it may change game state.

### 3.2 Keys

- **Any modifiers.** A key listener fires whatever modifier keys are held. `meta` is what
  `BlzGetTriggerPlayerMetaKey()` read: the sum of `METAKEY_SHIFT` (1), `METAKEY_CTRL` (2), `METAKEY_ALT` (4) and
  `METAKEY_WINKEYS` (8), or `METAKEY_NONE` (0). A map that wants one combination compares:
  `meta == METAKEY_CTRL + METAKEY_SHIFT`. There is no option for it.
- **Repeats.** `onKeyDown` fires once per press: the downs the game sends while the key stays held are skipped. With
  `{repeats = true}` it fires for those too, and `repeated` is true for them. Without the option `repeated` is always
  false.
- **How a press is told from a repeat.** The module keeps, for each player and key that has a listener, whether the
  key is held: set by a down, cleared by an up, and cleared when the key's last listener is removed. A down while it
  is set is a repeat. The state changes only in the synced events, so it is the same on every machine.
- **One missed release costs one press.** If the game never sends the up (the README says so as a possibility, not as
  something measured), the next press reads as a repeat; its own up then clears the state.
- **`onKeyUp`** fires at every release, with the modifiers held at that moment.

Checks, raising at the caller's line: `player` through `Handle.unwrap`; `key` must not be nil ("expected a key, such
as OSKEY_Q"); `callback` through `Callback.check`; `options` through `Options.read` with the one field
`repeats = {'boolean', false}`.

### 3.3 Mouse

- `onMouseDown` and `onMouseUp` pass the world point (`BlzGetTriggerPlayerMouseX`, `...MouseY`) and the button
  (`BlzGetTriggerPlayerMouseButton`), every button to every listener.
- `onMouseMove` passes the world point.
- The README says, as measured: a click on the interface fires too; a quick click delivers down and up at the same
  moment; mouse move fires 150 to 190 times a second while the mouse moves, each one a synced event, so listen to it
  only while it is needed and remove the listener afterwards.

### 3.4 Triggers

The module uses one listener set (`Listeners.new('InputListener', …)`), with string keys:

| Listeners | Key | The key's one trigger is registered with |
| --- | --- | --- |
| `onKeyDown`, `onKeyUp` | `k<player id>:<key code>` | `BlzTriggerRegisterPlayerKeyEvent` for each of the 16 modifier values, key down and key up: 32 events |
| `onMouseDown` | `d<player id>` | `TriggerRegisterPlayerEvent` with `EVENT_PLAYER_MOUSE_DOWN` |
| `onMouseUp` | `u<player id>` | `EVENT_PLAYER_MOUSE_UP` |
| `onMouseMove` | `m<player id>` | `EVENT_PLAYER_MOUSE_MOVE` |

The key code is `GetHandleId(key)`. As in v0.7.0, a trigger is created by its key's first listener, disabled while the
key has no listeners, enabled again by the next and never destroyed. Importing the module creates nothing.

A key's down and up listeners share one trigger because the held state needs both events whichever kind is listened
to. Its action reads `BlzGetTriggerPlayerIsKeyDown()`, `BlzGetTriggerPlayerMetaKey()` and `GetTriggerPlayer()` once,
updates the held state, and calls every listener of the key; each listener's own filter (down or up, repeats or not)
is a closure the module wraps around the map's callback.

Routing follows v0.7.0: listeners run in the order added; one added during a firing waits for the next; one removed
is skipped at once; each runs behind the callback boundary, labelled `Input listener`.

## 4. `wrappers.weathereffect`

```lua
local WeatherEffect = require 'wrappers.weathereffect'

---Creates a weather effect over the rect. It is not drawn until enable(true). The effect does not own the rect.
---@param rect MoonwellWrappers.Rect
---@param effectId integer For example FourCC('RAhr').
---@return MoonwellWrappers.WeatherEffect
function WeatherEffect.create(rect, effectId)

function WeatherEffect.fromHandle(raw)
function WeatherEffect:getHandle()
function WeatherEffect:isDisposed()
---@param flag boolean
function WeatherEffect:enable(flag)        -- EnableWeatherEffect(raw, flag)
---Draws it on that player's machine only.
---@param player MoonwellWrappers.Player
function WeatherEffect:enableFor(player)   -- EnableWeatherEffect(raw, player == GetLocalPlayer())
function WeatherEffect:destroy()           -- RemoveWeatherEffect
```

- **An unknown id raises.** When `AddWeatherEffect` returns a handle with id -1, `create` removes it and raises
  "unknown weather effect id: <id>" at the caller's line, as `Image.create` does for a wrong path.
- **Enabling.** `enable` and `enableFor` follow `show(flag)` and `setVisibleFor(player)` of the presentation classes:
  the effect exists on every machine and only its drawing differs. `enable(flag)` afterwards applies to everyone.
  There is no getter for whether it is enabled.
- **The rect** is read at creation: the map may destroy it afterwards (measured).
- **Ids are used again** by the game after `destroy()`. The registry already drops a destroyed wrapper's handle, so a
  later effect with an equal handle gets a wrapper of its own, and the old wrapper stays disposed.
- The README lists the game's weather ids. The gate creates each one and the list keeps those that were created
  (§9).

## 5. Effect

### 5.1 `Effect.abilityArt`

```lua
---The art an ability's data names for an effect type: a model path, or a lightning code for EFFECT_TYPE_LIGHTNING.
---Returns nil when the ability has none.
---@param abilityId integer
---@param effectType effecttype
---@param index integer? Which entry of a list, from 1; default 1. Past the last entry the game reads the last.
---@return string?
function Effect.abilityArt(abilityId, effectType, index)
```

```yuescript
clap = Effect.abilityArt FourCC("AHtc"), EFFECT_TYPE_CASTER
Effect.flash clap, x, y if clap
```

- It calls `GetAbilityEffectById(abilityId, effectType, index - 1)` and returns nil for `""` or nil.
- `index` must be an integer of at least 1, else "expected a positive integer index".
- The result goes to the existing constructors (`Effect.create`, `attach`, `flash`, `flashOn`), to anything else that
  takes a model path, and to `Lightning.create` for a lightning code.
- The README says: abilities such as Blizzard keep their art on their buff, so the ability reads nil; an index past
  the last entry reads the last entry.

One function replaces four constructors mirroring `AddSpellEffectById` and `AddSpellEffectTargetById`: the probe showed
the same effect either way, the path also serves a missile's model and a lightning, and a list's later entries are
reachable only through the path.

### 5.2 Model checks

`Effect.create`, `Effect.attach`, `Effect.flash` and `Effect.flashOn` raise "expected a model path" at the caller's
line when `model` is not a string. So an art that is missing (nil) fails where it is used instead of drawing nothing.
An empty string is still passed to the game, which returns an effect that draws nothing.

This is the release's one change to existing behavior; the CHANGELOG has a migration line for it.

## 6. Trigger

Four registrations, named after their natives like the existing ones:

```lua
---@param player MoonwellWrappers.Player
---@param state playerstate
---@param op limitop
---@param value number
function Trigger:registerPlayerStateEvent(player, state, op, value)   -- TriggerRegisterPlayerStateEvent

---@param player MoonwellWrappers.Player
---@param alliance alliancetype
function Trigger:registerPlayerAllianceChange(player, alliance)       -- TriggerRegisterPlayerAllianceChange

---@param state gamestate
---@param op limitop
---@param value number
function Trigger:registerGameStateEvent(state, op, value)             -- TriggerRegisterGameStateEvent

---@param timer MoonwellWrappers.Timer
function Trigger:registerTimerExpireEvent(timer)                      -- TriggerRegisterTimerExpireEvent
```

Each uses the one-lookup prologue and converts its wrapper argument with `Handle.unwrap`. The README records what
§2 measured for each: when it fires, and that the alliance event names no player.

Keys and the mouse get no Trigger registration: `wrappers.input` covers them, and the mouse events remain reachable
through `registerPlayerEvent`.

## 7. `fromEvent()`

A static function on six classes, each returning the wrapper of the object the running event is about, or nil when
the event has none:

| Function | Native |
| --- | --- |
| `Unit.fromEvent()` | `GetTriggerUnit` |
| `Player.fromEvent()` | `GetTriggerPlayer` |
| `Item.fromEvent()` | `GetManipulatedItem` |
| `Destructable.fromEvent()` | `GetTriggerDestructable` |
| `Timer.fromEvent()` | `GetExpiredTimer` |
| `Region.fromEvent()` | `GetTriggeringRegion` |

Each is `fromHandle` of its native, with the same nullable return type. The other event responses (`GetKillingUnit`,
`GetSpellTargetUnit` and the rest) keep the pattern the README already shows: `Unit.fromHandle(GetKillingUnit())`.

A single `wrappers.event` module with a function per event response was considered and rejected: it would have to
import Unit, Item, Destructable and Player to return their wrappers, so a map using one of its functions would bundle
all four.

## 8. Documentation

- README: new API sections for `wrappers.input` and `wrappers.weathereffect` with a short example each; the
  `abilityArt` entry and the model check under Effect; the four registrations under Trigger with their measured
  behavior; `fromEvent()` in the six class lists and in "Handles and cleanup", replacing the
  `Unit.fromHandle(GetTriggerUnit())` example.
- CHANGELOG `0.8.0`: the additions, the one migration line (§5.2), the gate record.
- CONTRIBUTING: the new gate run.
- AGENTS: the release, the input listener keys and the held state.
- Moonwell's `AGENTS.md`, CHANGELOG and roadmap: phase 4 item 1 released.

## 9. Verification and release gate

Automated checks, as in CONTRIBUTING:

- `deno task test`: behavior tests with native doubles for every rule in §3 to §7, including:
  - one trigger per player and key with 32 registrations; down and up told apart; a repeat skipped by default and
    passed with `repeats = true`; the held state cleared by an up and by removing the last listener; modifiers
    passed through; two players and two keys kept apart;
  - the mouse listeners' arguments; one trigger per player and kind; the trigger disabled when empty and enabled
    again;
  - `Input.off` during a firing, and a wrong token;
  - a weather effect of id -1 removed and raised; `enableFor` with the local and another player; a reused handle
    getting a new wrapper;
  - `abilityArt` for a path, `""`, nil and each bad index; each Effect constructor raising for a non-string model;
  - the four registrations' native arguments; each `fromEvent()` for an object and for nil;
  - the `blame` sweep extended to every new function;
- `check:lua` with Lua 5.3.6;
- `test:integration`: Moonwell builds, the native-call check, the import test (`wrappers.input` and
  `wrappers.weathereffect` bundle only what they import), and LuaLS positives and negatives for the new functions.

In-game gate (the maintainer, 3.0.0.24268), one new run `deno task gate additions`, normal build only, backed by an
`additions` flag in `examples/gate.yue`. A dry run of the compiled gate on stub natives comes first. The gate shows
one thing at a time:

1. **Printed at once, nothing to watch:** `abilityArt` for a path, a missing art and a list's third entry; the error
   of `Effect.create` with a missing art; the four registrations firing as §2 says, with `Player.fromEvent()` and
   `Timer.fromEvent()`; `fromEvent()` of a unit, an item, a destructable and a region event; which of the README's
   weather ids were created, and the error for an unknown one.
2. **Rain:** it falls after `enable(true)`; stops after `enableFor` another player; falls again after `enableFor` the
   maintainer's player; stops after `destroy()`. Each change is announced by a line.
3. **Art:** a Thunder Clap from `Effect.create` with `abilityArt`'s path appears.
4. **Input, one step at a time; Esc moves to the next:** tap Q (one down, one up); hold Q for two seconds (the default
   listener counts one down, the `repeats` listener many); Shift with Q (the down arrives with `meta` 1); a left
   click on the ground (down and up with the point and the button); moving the mouse (a count); after `Input.off`,
   Q and a click print nothing.

The existing runs are not re-run: the release adds modules and functions, and its one change to existing code is a
type check in front of four natives. Tag consumption as for every release. Two machines stay deferred to the online
checks before Moonwell 1.0, which now also cover `wrappers.input` and `weather:enableFor`.

## 10. Plan phasing

One plan, `docs/superpowers/plans/2026-10-01-moonwell-wrappers-additions.md`, of test-first tasks:

1. `Effect.abilityArt` and the model checks.
2. The four Trigger registrations.
3. `fromEvent()` on the six classes.
4. `wrappers.weathereffect`.
5. `wrappers.input` (one task: the module is one file with one listener set; keys and the mouse are separate tests).
6. The blame sweep, the LuaLS fixtures and integration.
7. Docs.
8. The gate additions and the gate map's `additions` run.
9. Release.

## 11. Out of scope

- **An option for one modifier combination**, and **key state queries** (`isKeyDown`): a listener's `meta` covers the
  first, and the second would answer only for keys that have listeners.
- **Mouse wheel and the cursor's screen position:** frame events and machine-local values.
- **Constructors that mirror `AddSpellEffectById`**, and **ability sounds** (`GetAbilitySoundById`).
- **The other event responses as wrappers** (§7), **filtered unit events**, **command and upgrade events.**
- **Automatic disposal of Unit wrappers:** its own backlog item.
