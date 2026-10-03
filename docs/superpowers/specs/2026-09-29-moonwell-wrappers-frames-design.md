# Moonwell Frame Wrappers (wrappers v0.5.0) — Design

- **Date:** 2026-09-29
- **Status:** Approved in chat on 2026-09-29; awaiting review of this written spec.
- **Builds on:** the wrapper specs for v0.1.0 (`2026-09-28-moonwell-wrappers-design.md`), v0.2.0
  (`2026-09-28-moonwell-wrappers-broad-design.md`), v0.3.0 (`2026-09-29-moonwell-wrappers-presentation-design.md`) and
  v0.4.0 (`2026-09-29-moonwell-wrappers-classic-ui-design.md`, released on `be7dc97`). Everything in those specs still
  applies unless this one changes it explicitly.
- **Target:** wrappers `v0.5.0` in the separate `mdlsvensson/moonwell-wrappers` repository. No Moonwell CLI change.

## 1. Intent and scope

Release C of the UI backlog item: the `BlzFrame` API, with its own ownership design. One module, `wrappers.frame`,
covers frame creation, the frame tree, layout, content, events and TOC loading, with an in-game probe and gate.

The release is additive: no existing call changes.

Decisions made in chat on 2026-09-29:

1. **Owned tree.** Frames made by the wrapper form a tree; destroying one disposes the wrappers of its whole subtree.
   Game frames are **borrowed** and can never be destroyed through a wrapper.
2. **Per-frame callbacks.** `frame:on(event, callback)` with removable tokens, like Trigger actions; no Trigger
   registration and no event getters.
3. **No machine-local getters**, as in releases A and B.
4. **TOC/FDF: library only.** `Frame.loadTOC` plus a README recipe; no Moonwell CLI change.

## 2. Module and dependencies

```text
src/wrappers/frame.lua    (new)
```

Still no umbrella module, no globals, literal requires only, and no game-object creation at import. `frame.lua`
imports `wrappers.player`, because event callbacks receive Player wrappers (the v0.2.0 rule: a module imports another
public module only to return its wrappers), and `wrappers.internal.*`.

## 3. Kinds of frame

One class, `MoonwellWrappers.Frame`, with a private kind per wrapper:

| Kind          | Made by                                                                   | `destroy()`                  |
| ------------- | ------------------------------------------------------------------------- | ---------------------------- |
| owned         | `Frame.create`, `Frame.createSimple`, `Frame.createByType`                 | yes (section 3.2)            |
| template part | `frame:findChild(name)`; `frame:getChild(index)` on an owned or part frame | raises                       |
| borrowed      | `Frame.origin`, `Frame.byName`, `Frame.fromHandle`; `getChild` on a borrowed frame; `getParent` of an unknown frame | raises |

`destroy()` on a template part or a borrowed frame raises
`[wrappers] Frame.destroy: only frames made by Frame.create, createSimple or createByType can be destroyed` before any
native runs.

All wrappers share one strong registry keyed by the raw handle: wrapping the same raw handle again returns the same
wrapper, whatever made it first. `Frame.byName` on a frame the wrapper created returns the owned wrapper.

### 3.1 The tree

- Every owned frame records the owned frames created with it as parent (**children**) and the template parts found
  through it (**parts**), each in an array in creation order.
- A template part found through another template part belongs to the same owned frame as that part.
- The parent of an owned frame may be owned or borrowed. An owned frame with a borrowed parent is a **root**.
- `setParent(parent)` on an owned frame moves it in the tree: out of its old parent's children (if owned), into the new
  parent's children (if owned), then BlzFrameSetParent. On a borrowed frame, `parent` must be borrowed too, otherwise it
  raises `a borrowed frame can only be re-parented to a borrowed frame`: destroying an owned frame must never silently
  destroy a game frame. `setParent` on a template part raises `a template part cannot be re-parented`.

### 3.2 Destruction

`frame:destroy()` on an owned frame:

1. Disposes the subtree's wrappers depth-first, in array order: for each owned descendant and each part, its callbacks
   are cleared, its internal trigger is destroyed (DestroyTrigger) and its registry entry removed.
2. Removes the frame from its owned parent's children.
3. Calls BlzDestroyFrame once, on the frame itself: the native destroys the children too.

A second `destroy()` does nothing. Every other method on a disposed wrapper raises `Frame is disposed`.

### 3.3 Create contexts

Each create call passes the next value of a module counter (starting at 1) as `createContext`. Frame creation must run
on every machine in the same order (section 5), so the counter agrees across machines. `findChild(name)` calls
BlzGetFrameByName(name, context) with the context of the owned frame the search starts from (for a part, its owner's),
so template parts are found without bookkeeping. A `findChild` that returns nil raises
`[wrappers] Frame.findChild: no frame named <name> in this frame`.

## 4. API

### 4.1 Factories and statics

- `Frame.create(template, parent, options?) -> Frame`: BlzCreateFrame(template, parent, priority, context). Options:
  `priority` (integer, 0).
- `Frame.createSimple(template, parent) -> Frame`: BlzCreateSimpleFrame(template, parent, context).
- `Frame.createByType(frameType, parent, options?) -> Frame`: BlzCreateFrameByType(frameType, name, parent, inherits,
  context). Options: `name` (string, `""`), `inherits` (string, `""`).
- `parent` is required for all three (any Frame wrapper). Factories raise `native returned nil`. What an unknown
  template or type returns is measured by the probe (section 8); if Warcraft returns a dead frame rather than nil, the
  factories get a check like `Image.create`'s before release, decided with the maintainer.
- `Frame.origin(originType, index?) -> Frame` (BlzGetOriginFrame; `index` defaults to 0) and
  `Frame.byName(name, context?) -> Frame` (BlzGetFrameByName; `context` defaults to 0): borrowed, or the existing
  wrapper. Both raise `no frame` when the native returns nil.
- `Frame.fromHandle(raw) -> Frame?`: existing wrapper, or a borrowed one; nil for nil.
- `Frame.loadTOC(path)`: BlzLoadTOCFile; raises `[wrappers] Frame.loadTOC: could not load <path>` when it returns false.
- `Frame.hideOrigin(flag)` (BlzHideOriginFrames) and `Frame.enableAutoPosition(flag)` (BlzEnableUIAutoPosition), for
  everyone.

### 4.2 Methods (every kind)

- Common: `getHandle()`, `isDisposed()`, `.handle`, `destroy()` (section 3).
- Structure (the same on every machine): `getName()` (BlzFrameGetName), `getParent() -> Frame?`, `getChildrenCount()`,
  `getChild(index) -> Frame` (zero-based, as the native; raises `no child <index>` on nil), `findChild(name) -> Frame`
  (owned and part frames only; a borrowed frame raises `findChild needs a frame made by Frame.create*`),
  `setParent(parent)`.
- Layout: `setPoint(point, relative, relativePoint, x, y)`, `setAbsPoint(point, x, y)`, `setAllPoints(relative)`,
  `clearPoints()`, `setSize(width, height)`, `setScale(scale)`, `setLevel(level)`.
- Content: `setText(text)`, `addText(text)`, `setTextColor(r, g, b, a)` and `setVertexColor(r, g, b, a)` (integers
  0–255, converted with BlzConvertColor(a, r, g, b)), `setFont(path, height, flags?)` (flags default 0),
  `setTextAlignment(vertical, horizontal)`, `setTextSizeLimit(size)`, `setTexture(path, flag?, blend?)` (0 and true),
  `setModel(path, cameraIndex?)` (0), `setSpriteAnimate(primaryProp, flags)`, `setAutoScroll(flag)`
  (BlzTextAreaFrameSetAutoScroll).
- Sliders: `setValue(value)`, `setMinMaxValue(min, max)`, `setStepSize(step)`.
- State: `setAlpha(alpha)`, `setEnabled(flag)`, `setTooltip(tooltip)`, `show(flag)` (BlzFrameSetVisible),
  `setVisibleFor(player)` (section 5), `releaseFocusFor(player)` (section 6).
- Events: `on(event, callback) -> FrameHandler`, `off(token)` (section 6).

Frame arguments (`parent`, `relative`, `tooltip`) take any Frame wrapper; Player arguments are converted with
`Handle.unwrap`. Values pass through to the natives unchanged; README states the frame coordinate space (0–0.8 wide,
0–0.6 high, origin bottom left).

### 4.3 Left out

Machine-local getters: `BlzFrameGetText`, `GetValue`, `IsVisible`, `GetEnable`, `GetAlpha`, `GetWidth`, `GetHeight`,
`GetTextSizeLimit`. Local actions: `BlzFrameSetFocus`, `BlzFrameClick`, `BlzFrameCageMouse`, the pixel conversions.
Map code reads synced values only in event callbacks.

## 5. Synchronization

Frames must be created, destroyed and re-parented on every machine in the same order, never inside a branch on the
local player: the create-context counter and the handle tables would diverge. README states this rule first.

`setVisibleFor(player)` calls BlzFrameSetVisible(raw, unwrap(player) == GetLocalPlayer()): the same call on every
machine with a machine-local boolean, as in releases A and B. `show(flag)` afterwards sets the same value for everyone.

## 6. Events

- `frame:on(event, callback) -> FrameHandler`: the first `on` of a frame creates its internal trigger (CreateTrigger
  and one TriggerAddAction); the first `on` for an event type registers it once
  (BlzTriggerRegisterFrameEvent(trigger, frame, event)). Callbacks for an event type run in the order added.
- The action reads `BlzGetTriggerFrameEvent()` and runs that type's live callbacks, each behind the callback boundary
  (`Callback.call`-style, label `Frame event`), with `(player, event)`: `player` is
  `Player.fromHandle(GetTriggerPlayer())`; `event` is a fresh table `{type, frame, text, value}` with
  `BlzGetTriggerFrameText()` and `BlzGetTriggerFrameValue()`, the event's synced data.
- `frame:off(token)` removes that callback at once, even during a firing; removing twice does nothing; a token from
  another frame raises `token belongs to another frame`. Warcraft cannot unregister a frame event: an event type with no
  callbacks left simply runs none.
- Disposal (section 3.2) clears every callback before any native runs, so a destroyed frame's callbacks never run. A
  callback may destroy its own frame or an ancestor; the gate checks it.
- `frame:releaseFocusFor(player)`: on that player's machine only, BlzFrameSetEnable(raw, false) then
  BlzFrameSetEnable(raw, true). A clicked button keeps keyboard focus and blocks hotkeys (community note); this is the
  common fix, called from a click callback with the callback's player. The probe measures both.

Types: `MoonwellWrappers.FrameHandler` (opaque token), `MoonwellWrappers.FrameEvent`
(`{type: frameeventtype, frame: MoonwellWrappers.Frame, text: string, value: number}`) and the alias
`MoonwellWrappers.FrameCallback` (`fun(player: MoonwellWrappers.Player, event: MoonwellWrappers.FrameEvent): ...`).

## 7. Validation and errors

Validated before any native that changes state: receivers and arguments (identity, disposal, kind), callbacks and
tokens, options (`internal/options.lua`), `findChild`/`getChild` targets, and the re-parenting rules. Getters used for
validation may run first. Value domains are left to Warcraft, as before.

## 8. Verification and release gate

Automated (the wrappers repo's suite, extended):

1. Native-double tests for every method: exact natives and arguments, color conversion, defaults.
2. The tree: owned, part and borrowed kinds; create contexts; `findChild` through parts; destroy disposing descendants
   and parts depth-first with one BlzDestroyFrame; `setParent` moving owned frames and its two refusals; identity
   through `byName`/`fromHandle`.
3. Events: one trigger per frame, one registration per event type, callbacks in order with player and event data,
   errors printed and later callbacks still run, `off` during a firing, tokens of another frame, a callback destroying
   its own frame or an ancestor.
4. `setVisibleFor` and `releaseFocusFor` with a stubbed GetLocalPlayer, as the local and as another player.
5. Lua 5.3.6 syntax; LuaLS positive and negative fixtures (callback parameters typed, a Unit passed as a parent);
   a direct LuaLS run over `src`.
6. Integration: fresh Moonwell 0.5.0 consumer, check and build (normal and minified); importing only `wrappers.frame`
   bundles `wrappers.player` and no other public module.

In-game (maintainer, Warcraft III Reforged 3.0.0.24268, World Editor 3.00), in `../wrappers-gate`:

- **Probe run `frame-init`** (first): measures community notes the wrappers rely on; answers go into README.
  1. Is `BlzGetOriginFrame(ORIGIN_FRAME_GAME_UI, 0)` non-nil in `on_main`, and does a frame created there show later?
  2. Does `BlzGetFrameByName` on a frame we created return the same userdata (identity)?
  3. Which built-in templates create without any TOC (`ScriptDialogButton`, `EscMenuBackdrop`,
     `QuestButtonBaseTemplate`, `BattleNetTextAreaTemplate`, `EscMenuEditBoxTemplate`)? After loading a map TOC that
     lists the standard template FDFs, do the missing ones create?
  4. What does `BlzCreateFrame` return for an unknown template name: nil, or a frame (and does it show or crash when
     used)? What does `BlzLoadTOCFile` return for a missing file?
  5. Destroying a parent: do its children disappear, including one re-parented onto it with BlzFrameSetParent?
  6. After clicking a button, do hotkeys (for example a hero ability) still work; does the disable/enable fix restore
     them?
  7. Destroying a button inside its own click event: no crash, no later events?
- **Gate runs `frames` and `frames-min`**, from `examples/gate.yue` with a new `frames = false` flag: a panel from a
  built-in or TOC-loaded template with a title, a text button, an edit box, a slider and a checkbox; clicks, text and
  value events printed with the clicking player; `releaseFocusFor`; a tooltip; `setVisibleFor` another player hides a
  frame; `setParent` into the panel; destroying the panel from its own close button disposes everything (a later
  `isDisposed()` print on a child is true) and no event prints afterwards. World Editor opens the packed map; then the
  tag consumption gate.

The multiplayer effects (`setVisibleFor`, `releaseFocusFor`, events from a second player, creation order across
machines) join Moonwell's pre-1.0 online checks and are recorded as deferred, not passed.

## 9. Plan phasing

One implementation plan, written in Moonwell's `docs/superpowers/plans/`, with a review after each phase:

1. Frame kinds, registry, factories and statics, structure methods, create contexts.
2. The tree: `findChild`/`getChild` parts, `setParent`, destruction.
3. Layout, content, slider and state setters; `setVisibleFor`; `releaseFocusFor`.
4. Events.
5. Import graph, bundle and editor coverage.
6. README (frames section with the sync rule, coordinate space and TOC recipe), CHANGELOG, CONTRIBUTING gate,
   `examples/gate.yue` (`frames` flag), the gate map's `frame-init` probe and `frames`/`frames-min` runs, the wrappers
   repo's AGENTS.md, and Moonwell's AGENTS.md (state, backlog: library-shipped assets).

## 10. Out of scope

Moonwell CLI changes, including assets shipped by libraries (backlog). Authoring help for FDF files. Syncing
machine-local frame state (for example `BlzSendSyncData` for typed text): 4d territory. Save/load. Simple-frame
specifics beyond `createSimple`. Trigger registration for frame events and event getters outside the frame module.
Wrapper calls inherit native synchronization requirements, as before.
