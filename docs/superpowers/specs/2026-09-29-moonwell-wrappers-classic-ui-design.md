# Moonwell Classic UI Wrappers (wrappers v0.4.0) — Design

- **Date:** 2026-09-29
- **Status:** Approved in chat on 2026-09-29; awaiting review of this written spec.
- **Builds on:** `2026-09-28-moonwell-wrappers-design.md` (v0.1.0), `2026-09-28-moonwell-wrappers-broad-design.md`
  (v0.2.0) and `2026-09-29-moonwell-wrappers-presentation-design.md` (v0.3.0, release A; v0.3.1 on `94d650f` added
  native caveats). Everything in those specs still applies unless this one changes it explicitly.
- **Inputs:** the w3ts comparison §2.1 (`docs/superpowers/research/2026-09-29-w3ts-comparison.md`), whose classic UI
  notes are w3ts's and not yet measured by us.
- **Target:** wrappers `v0.4.0` in the separate `mdlsvensson/moonwell-wrappers` repository. No Moonwell CLI change.

## 1. Intent and scope

Release B of the UI backlog item: classic UI. Dialog with buttons, Multiboard, Leaderboard, Quest with quest items,
DefeatCondition and TimerDialog, with one in-game gate. Release C (the `BlzFrame` API) follows with its own spec.

The release is additive: no existing call changes.

Decisions made in chat on 2026-09-29:

1. Dialog clicks are handled by **per-button callbacks** owned by the dialog, not by Trigger registration and event
   getters.
2. Multiboard rows and columns are **one-based**, as Lua arrays are.
3. Multiboard cells have **no item wrapper**: cell methods get, change and release the native item in one call.
4. All five areas ship together as v0.4.0.

## 2. Modules and dependencies

```text
src/wrappers/
  dialog.lua multiboard.lua leaderboard.lua quest.lua defeatcondition.lua timerdialog.lua    (new)
```

Still no umbrella module, no globals, literal requires only, and no game-object creation at import. `dialog.lua`
imports `wrappers.player`, because its callbacks receive Player wrappers (the v0.2.0 rule: a module imports another
public module only to return its wrappers). Every other new module imports only `wrappers.internal.*` and converts
Player and Timer arguments with `Handle.unwrap`.

## 3. Lifetime

### Owned wrappers

Dialog, Multiboard, Leaderboard, Quest, DefeatCondition and TimerDialog have strong caches until `destroy()`, like
Timer and the release A classes. The game never ends any of them on its own. Each has the common members:
`fromHandle(raw)`, `getHandle()`, `isDisposed()`, `.handle`. Factories raise `native returned nil`. Wrapping a
preplaced or foreign handle with `fromHandle` transfers no cleanup duty, as in v0.1.0.

### Children owned by a parent

Warcraft has no native that removes a single dialog button or quest item: they die with their parent. So:

- A **DialogButton** belongs to the Dialog that made it. `dialog:clear()` and `dialog:destroy()` dispose every button
  of that dialog (their `isDisposed()` becomes true, their callbacks can never run again).
- A **QuestItem** belongs to the Quest that made it. `quest:destroy()` disposes every item of that quest.

Children have `getHandle()`, `isDisposed()` and `.handle`, but **no `destroy()` and no `fromHandle`**: a child wrapped
from a raw handle would have no parent to dispose it. Children are made only by their parent's `addButton`/`addItem`.

Each parent keeps its children in an array (insertion order), never iterated with `pairs`, so disposal order is the
same on every machine.

### Referenced, not owned

A TimerDialog references a Timer but does not own it (section 8). A Leaderboard or Multiboard never owns the Player
wrappers passed to it.

## 4. Local visibility

`setVisibleFor(player)` exists on Multiboard and TimerDialog, with release A's rule: it calls the class's display
native with `Handle.unwrap(player, 'Player', operation) == GetLocalPlayer()`, so every machine makes the same call with
a machine-local boolean. `show(b)` afterwards sets the same value for everyone.

- **Dialog** has no `setVisibleFor`: DialogDisplay already takes the player, so `show(player)`/`hide(player)` make the
  same call on every machine.
- **Leaderboard** has no `setVisibleFor`: each player sees the leaderboard assigned to them with PlayerSetLeaderboard
  (section 6), again the same call on every machine.
- No getters for display state (`IsMultiboardDisplayed`, `IsTimerDialogDisplayed`, `IsLeaderboardDisplayed`) or for
  `IsMultiboardMinimized`: they answer differently on each machine after local visibility or a player's own click.

README repeats the warning: branching on the local player must not change synchronized game state.

## 5. Dialog (`wrappers.dialog`)

### 5.1 Dialog

- `create() -> Dialog` (DialogCreate).
- `setMessage(text)` (DialogSetMessage).
- `addButton(text, callback) -> DialogButton` or `addButton(text, options?, callback?) -> DialogButton`. If the second
  argument is a function it is the callback and there are no options; otherwise it is the options table (or nil), and
  the third argument is the callback (or nil). Options (`MoonwellWrappers.DialogButtonOptions`):

  | Option        | Type    | Default | Effect                                                                          |
  | ------------- | ------- | ------- | ------------------------------------------------------------------------------- |
  | `hotkey`      | string  | nil     | One ASCII letter or digit; passed as `string.byte(hotkey:upper())`, else 0      |
  | `quit`        | boolean | false   | DialogAddQuitButton instead of DialogAddButton                                  |
  | `scoreScreen` | boolean | false   | DialogAddQuitButton's `doScoreScreen`; `true` without `quit` raises             |

  A `hotkey` that is not a single ASCII letter or digit raises
  `[wrappers] Dialog.addButton: option 'hotkey' expected one letter or digit`.
- `show(player)`, `hide(player)`: DialogDisplay(player, dialog, true/false).
- `clear()`: disposes every button, then DialogClear. The dialog's internal trigger stays.
- `destroy()`: disposes every button, destroys the internal trigger (if any), then DialogDestroy.

### 5.2 Clicks

The first `addButton` with a callback creates the dialog's **internal trigger**: CreateTrigger,
TriggerRegisterDialogEvent(trigger, dialog) and one TriggerAddAction. The dialog module uses these natives directly
and does not import `wrappers.trigger`; the internal trigger is never exposed.

The action looks up `GetClickedButton()` among the dialog's live buttons (a table keyed by the raw button handle, used
only for lookup). If the button has a callback, it runs behind the callback boundary
(`Callback.call('Dialog button', callback, Player.fromHandle(GetTriggerPlayer()))`): an error is printed with the
`[wrappers]` prefix and does not stop later clicks. A disposed button's callback is never run, because disposal
removes it from the lookup table and clears its callback field first.

A callback may hide, clear or destroy its own dialog. Destroying disposes the buttons and the trigger that is running
the action; the action holds no reference that is used after the callback returns. The gate checks all three.

Button callbacks have the type `fun(player: MoonwellWrappers.Player): ...`.

### 5.3 DialogButton

- `getHandle()`, `isDisposed()`, `.handle`, `getDialog() -> Dialog` (valid after disposal too, for identity checks).

## 6. Leaderboard (`wrappers.leaderboard`)

Items are keyed by player: a leaderboard holds at most one item per player (LeaderboardHasPlayerItem), and the natives'
zero-based item indexes never appear in the API.

- `create(label?) -> Leaderboard` (CreateLeaderboard, then LeaderboardSetLabel if `label` is given).
- `setLabel(text)`, `setLabelColor(r, g, b, a)`, `setValueColor(r, g, b, a)` (integers 0–255).
- `setStyle(options)`: LeaderboardSetStyle with options `label`, `names`, `values`, `icons` (booleans, all default
  true; `MoonwellWrappers.LeaderboardStyleOptions`).
- `addItem(player, label, value)`: raises `player already has an item` if LeaderboardHasPlayerItem; otherwise
  LeaderboardAddItem(lb, label, value, player), then **resize**.
- `removeItem(player)`: raises `player has no item` if not LeaderboardHasPlayerItem; otherwise
  LeaderboardRemovePlayerItem, then **resize**.
- `setItemValue(player, value)`, `setItemLabel(player, label)`, `setItemLabelColor(player, r, g, b, a)`,
  `setItemValueColor(player, r, g, b, a)`, `setItemStyle(player, options)` (options `label`, `value`, `icon`, all
  default true; `MoonwellWrappers.LeaderboardItemStyleOptions`): each raises `player has no item` if needed, then calls
  the `LeaderboardSetItem*` native with `LeaderboardGetPlayerIndex(lb, player)`.
- `hasItem(player) -> boolean`, `getItemCount() -> integer`.
- `sortByValue(ascending)`, `sortByLabel(ascending)`, `sortByPlayer(ascending)`.
- `assign(player)`: PlayerSetLeaderboard(player, lb). A player sees the one leaderboard assigned to them.
- `show(b)` (LeaderboardDisplay). As in CreateLeaderboardBJ, assign first, then show.
- `destroy()` (DestroyLeaderboard).

**Resize** is `LeaderboardSetSizeByItemCount(lb, LeaderboardGetItemCount(lb))`, as LeaderboardResizeBJ does: a
leaderboard starts with no rows (w3ts) and does not grow by itself.

## 7. Multiboard (`wrappers.multiboard`)

Rows and columns are **one-based**; the wrapper subtracts one before calling a native. README states it once, in the
Multiboard section.

- `create(rows, columns, title?) -> Multiboard`: CreateMultiboard, MultiboardSetColumnCount(columns), then
  `setRowCount(rows)`, then MultiboardSetTitleText if `title` is given. `rows` and `columns` are non-negative integers,
  validated before CreateMultiboard.
- `setRowCount(count)`: changes the row count **one row at a time**, from MultiboardGetRowCount to `count`, with one
  MultiboardSetRowCount per step. w3ts reports that bigger steps are unsafe; stepping costs nothing if that is wrong,
  and the gate's probe measures it (section 10).
- `setColumnCount(count)` (MultiboardSetColumnCount), `getRowCount()`, `getColumnCount()`.
- `setTitle(text)`, `getTitle()` (MultiboardGetTitleText), `setTitleColor(r, g, b, a)`.
- `setCell(row, column, options)`, `setRow(row, options)`, `setColumn(column, options)`, `setAll(options)`: cell
  options (`MoonwellWrappers.MultiboardCellOptions`), all optional, at least one required
  (`expected at least one option`):

  | Option      | Type            | Native (one cell)             | Native (`setAll`)               |
  | ----------- | --------------- | ----------------------------- | ------------------------------- |
  | `value`     | string          | MultiboardSetItemValue        | MultiboardSetItemsValue         |
  | `color`     | `{r, g, b, a?}` | MultiboardSetItemValueColor   | MultiboardSetItemsValueColor    |
  | `icon`      | string          | MultiboardSetItemIcon         | MultiboardSetItemsIcon          |
  | `showValue` | boolean         | MultiboardSetItemStyle        | MultiboardSetItemsStyle         |
  | `showIcon`  | boolean         | (with `showValue`)            | (with `showValue`)              |
  | `width`     | number ≥ 0      | MultiboardSetItemWidth        | MultiboardSetItemsWidth         |

  `showValue` and `showIcon` go together (the natives take both): giving one without the other raises
  `options 'showValue' and 'showIcon' go together`. `width` is a fraction of the screen width, as in the native.
  Natives are called in the table's order.

  For each cell, `setCell` calls MultiboardGetItem(raw, row - 1, column - 1), applies the options, and calls
  MultiboardReleaseItem on that item. No native errors in between, so every item obtained is released; there is no
  MultiboardItem wrapper. `setRow` and `setColumn` do the same for each cell of the row or column, in ascending order.
  A `row` or `column` that is not an integer in `1..count` (counts from MultiboardGetRowCount/GetColumnCount) raises
  `[wrappers] Multiboard.setCell: row 5 outside 1..4` before any item is obtained. Options are validated before the
  counts are read.
- `show(b)` (MultiboardDisplay), `setVisibleFor(player)` (section 4), `minimize(b)` (MultiboardMinimize).
- `Multiboard.suppressDisplay(flag)` (MultiboardSuppressDisplay): hides or allows every multiboard, for everyone.
- `destroy()` (DestroyMultiboard).

## 8. Quest, DefeatCondition and TimerDialog

### 8.1 Quest (`wrappers.quest`)

- `create(options?) -> Quest`: CreateQuest, then each given option's setter; `required` and `discovered` are always
  set. Options (`MoonwellWrappers.QuestOptions`): `title`, `description`, `icon` (strings, default not set),
  `required` (boolean, true), `discovered` (boolean, true).
- `setTitle(text)`, `setDescription(text)`, `setIcon(path)`.
- `setRequired(b)`, `setCompleted(b)`, `setFailed(b)`, `setDiscovered(b)`, `setEnabled(b)`, each with its getter
  `isRequired()`, `isCompleted()`, `isFailed()`, `isDiscovered()`, `isEnabled()`. These values are set by map code and
  are the same on every machine.
- `addItem(description) -> QuestItem`: QuestCreateItem, then QuestItemSetDescription.
- `destroy()`: disposes every item, then DestroyQuest.
- `Quest.flashButton()` (FlashQuestDialogButton) and `Quest.refresh()` (ForceQuestDialogUpdate).

QuestItem: `setDescription(text)`, `setCompleted(b)`, `isCompleted()`, `getQuest() -> Quest`, plus the child members
of section 3.

### 8.2 DefeatCondition (`wrappers.defeatcondition`)

- `create(description?) -> DefeatCondition` (CreateDefeatCondition, then DefeatConditionSetDescription if given).
- `setDescription(text)`, `destroy()` (DestroyDefeatCondition).

### 8.3 TimerDialog (`wrappers.timerdialog`)

- `create(timer, title?) -> TimerDialog`: CreateTimerDialog(Handle.unwrap(timer, 'Timer', ...)), then
  TimerDialogSetTitle if `title` is given. A new timer dialog is hidden until `show(true)`, as with the native.
- The dialog keeps its Timer wrapper in a private table. Every method except `destroy()` and `isDisposed()` first
  checks it with `Handle.unwrap`, so it raises `Timer is disposed` once the timer is gone. README: destroy the timer
  dialog before its timer.
- `setTitle(text)`, `setTitleColor(r, g, b, a)`, `setTimeColor(r, g, b, a)`, `setSpeed(factor)`,
  `setRealTimeRemaining(seconds)`, `show(b)` (TimerDialogDisplay), `setVisibleFor(player)`.
- `destroy()` (DestroyTimerDialog). It never destroys the Timer.

## 9. Types, errors and validation

- Classes `MoonwellWrappers.Dialog`, `.DialogButton`, `.Multiboard`, `.Leaderboard`, `.Quest`, `.QuestItem`,
  `.DefeatCondition`, `.TimerDialog`; option types `MoonwellWrappers.DialogButtonOptions`, `.MultiboardCellOptions`,
  `.LeaderboardStyleOptions`, `.LeaderboardItemStyleOptions`, `.QuestOptions`, all with optional fields. Every
  `fromHandle` stays conservatively nullable.
- Options go through `internal/options.lua` (unknown keys, value types, colors, defaults in a fresh table).
- Also validated before any native that changes state: receivers and arguments (identity, disposal), callbacks
  (function or nil), hotkeys, `scoreScreen` without `quit`, `showValue`/`showIcon` pairing, non-negative integer row
  and column counts, one-based indexes in range, and the leaderboard's one-item-per-player rule. Validation may read
  counts with getters first. All other value domains are left to Warcraft, as before.

## 10. Verification and release gate

Automated (the wrappers repo's suite, extended):

1. Native-double tests for every method: exact natives and arguments, one-based to zero-based conversion, one row per
   MultiboardSetRowCount step (up and down), MultiboardReleaseItem after every MultiboardGetItem, range errors before
   any item is obtained, `setAll` using the `MultiboardSetItems*` natives, leaderboard resizing after add and remove,
   the one-item-per-player errors.
2. Dialog: the overloaded `addButton` forms, hotkey conversion and rejection, `scoreScreen` without `quit`, the internal
   trigger created once and only for a callback, click routing to the right callback with the clicking Player, errors
   printed and later clicks still routed, `clear`/`destroy` disposing buttons so a later click runs nothing, and a
   callback that destroys its own dialog.
3. Children: disposal with the parent, no `destroy`/`fromHandle`, `getDialog`/`getQuest`.
4. TimerDialog raising after its Timer is disposed, but `destroy()` still working.
5. `setVisibleFor` with a stubbed GetLocalPlayer, as the local and as another player; setter checks with flipped
   booleans (`checkSetters`).
6. Lua 5.3.6 syntax check; LuaLS positive and negative fixtures (button callback parameter typed as Player, options
   types, nullable `fromHandle`); a direct LuaLS run over `src`.
7. Integration: fresh Moonwell 0.5.0 consumer, check and build (normal and minified); importing only
   `wrappers.multiboard` bundles no other public module, and importing only `wrappers.dialog` bundles
   `wrappers.player` and no other public module.

In-game (maintainer, Warcraft III Reforged 3.0.0.24268, World Editor 3.00), in the gate map `../wrappers-gate`:

- **Probe run `ui-init`** (a new `src/probe_ui.yue`, first): measures w3ts's unmeasured claims, recorded in the WCSharp
  note §9 style and in README.
  1. Quest, Leaderboard and Multiboard created directly in `on_main` (not from a timer): does the game crash, and do
     they work once shown later?
  2. A Dialog and a Multiboard shown directly in `on_main`: are they visible?
  3. A multiboard with MultiboardSetRowCount from 0 straight to 5, next to one stepped by the wrapper: do both show
     five rows?
  4. What a new multiboard's cells show before any cell option (icons, values).

  Creation at module top level (while the map script loads) is not probed: Moonwell's README already says to create
  game objects inside hooks. If step 1 crashes, it is repeated with each type alone to find which one, and README says
  so; the API does not change.
- **Gate run `ui`** (normal) and **`ui-min`** (minified), from `examples/gate.yue` with a new `ui = false` flag,
  started from a zero-second timer as before:
  1. A dialog with three buttons (a hotkey, a normal one, one that clears and re-adds its buttons); each click prints
     the clicking player; the hotkey works; clicking hides the dialog; a click after `clear` runs only the new
     buttons; a button that destroys its own dialog works and nothing prints afterwards.
  2. A multiboard with a title, 3 × 3 cells set with value, color, icon, style and width, a row and a column set at
     once, `setAll`, rows grown from 3 to 6 and back to 2, minimize, and `setVisibleFor` another player hides it.
  3. A leaderboard assigned to player 1 and shown: items for two players with labels, values and colors, sorted by
     value, one removed (the board shrinks), style changes.
  4. A quest with two items, required and optional, completed, failed and discovered in turn, visible in the quest log
     (F9); `flashButton` flashes the quest button; a defeat condition appears in the log.
  5. A timer dialog on a running Timer: title, colors, speed; `setVisibleFor` another player hides it; destroyed before
     its timer.
  6. World Editor opens the packed map. Then the tag consumption gate.

The multiplayer effects of `setVisibleFor`, `assign` and per-player dialogs join Moonwell's pre-1.0 online checks and
are recorded as deferred, not passed.

## 11. Plan phasing

One implementation plan, written in Moonwell's `docs/superpowers/plans/`, with a review after each phase:

1. Dialog and DialogButton.
2. Multiboard.
3. Leaderboard.
4. Quest with QuestItem, DefeatCondition and TimerDialog.
5. README API reference and caveats, CHANGELOG, CONTRIBUTING gate, `examples/gate.yue` (`ui` flag), the gate map's
   `ui-init` probe and `ui`/`ui-min` runs, integration and LuaLS fixtures, the wrappers repo's AGENTS.md, and Moonwell's
   AGENTS.md state and backlog.

## 12. Out of scope

Frames (release C). Trigger registration for dialog events and event-response getters (`GetClickedButton`,
`GetClickedDialog`) outside the dialog module. A MultiboardItem wrapper. `MultiboardClear`. Quest messages
(`QuestMessageBJ` is a text helper, not a handle). Game caches. Getters for machine-local state. The victory and
defeat dialogs of `blizzard.j`. Wrapper calls inherit native synchronization requirements, as before.
