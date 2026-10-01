# Moonwell Systems Release 3 (v0.3.0): the Damage Pipeline — Design

- **Date:** 2026-10-01
- **Status:** The design was approved in chat on 2026-10-01; this written spec awaits review.
- **Builds on:** Part 1 of `2026-09-30-moonwell-systems-design.md`, which binds this release (rules §4, tooling §5,
  releases §6), releases 1 and 2, and `wrappers.damage` (moonwell-wrappers v0.7.0).
- **Inputs:** `wc3-lib`'s `damage/system.ts`, `damage/warcraft.ts` and their 16 tests (`tests/damage.test.ts`); the
  wrappers v0.7.0 port gate (a type change works only before armor; an outer hit's setters still work after a nested
  hit; `isAttack` is false for `damageTarget`).
- **Target:** moonwell-systems `v0.3.0`. It still needs moonwell-wrappers `v0.7.0` and Moonwell 0.5.2. The wrappers do
  not change.

## 1. Scope and the maintainer's choices

One module, `systems.damage`: listeners before armor, after armor and after the final amount is known; script damage
with metadata, queued so that it never nests; and attribution of a hit to its real caster. The maintainer chose
(2026-10-01):
- **Attribution is a resolver option:** `sourceOf`, a function from the dealing unit to the credited Unit. The module
  does not import `systems.dummy`; a map passes `dummies\sourceOf`.
- **Listeners change a hit with setter methods**, like `wrappers.damage`, so a wrong value or a change in the wrong
  phase raises at the listener's line.

The API style follows releases 1 and 2.

## 2. API

```lua
---@param options {sourceOf: (fun(dealer: Unit): Unit?)?, onError: fun(message: string)?,
---  maxQueue: integer?, maxChain: integer?, maxPending: integer?}?   -- defaults 128, 64, 64
function DamageSystem.new(options)                    -- creates nothing
function DamageSystem:start()                         -- idempotent while running; raises after dispose
function DamageSystem:beforeArmor(callback, priority) -- callback(hit); priority default 0; returns a remove function
function DamageSystem:afterArmor(callback, priority)
function DamageSystem:observe(callback, priority)
function DamageSystem:deal(request)
function DamageSystem:getCurrent()                    -- the hit being handled (the innermost one), or nil
function DamageSystem:dispose()                       -- idempotent
```

- The module returns `DamageSystem` (`import "systems.damage" as DamageSystem`), so a map can also import
  `wrappers.damage` as `Damage`. The LuaLS classes are `MoonwellSystems.DamageSystem` and `MoonwellSystems.Hit`.
- `start()` adds one `Damage.onDamaging` listener, one `Damage.onDamaged` listener and one wrappers `Timer`. It is all
  or nothing: if a registration raises, what was added is removed and the error is raised at the `start` line.
- Listeners may be added before or after `start()`. Adding one after `dispose()` raises.
- `dispose()` removes both wrappers listeners, destroys the timer and drops every listener, queued deal and pending
  frame. Called inside a listener, the hit's remaining listeners do not run.

### 2.1 The request of `deal`

| Field | Meaning |
| --- | --- |
| `source` | the Unit that deals the damage |
| `target` | the Unit to damage |
| `amount` | finite and not negative |
| `attack`, `ranged` | booleans, default false |
| `attackType`, `damageType`, `weaponType` | default `ATTACK_TYPE_NORMAL`, `DAMAGE_TYPE_NORMAL`, `WEAPON_TYPE_WHOKNOWS` |
| `metadata` | any value; the resulting hit's `metadata` |

`deal` copies the fields, so changing the table afterwards changes nothing. It calls
`source:damageTarget(target, amount, attack, ranged, attackType, damageType, weaponType)`.

## 3. The Hit

One Hit table per hit, passed to every listener of every phase.

| Field | Meaning |
| --- | --- |
| `source` | the credited Unit (§6); nil when the game gives no source |
| `dealer` | the Unit the game reported; nil when it gives none |
| `target` | the damaged Unit |
| `amount` | the current amount |
| `isAttack` | the game's value: false for script damage, even with `attack = true` |
| `attackType`, `damageType`, `weaponType` | the game's values in the current phase |
| `metadata` | the `deal` request's metadata; nil for every other hit |
| `phase` | `'beforeArmor'`, `'afterArmor'` or `'observe'` |
| `initialAmount` | the amount the game first reported |
| `beforeArmorAmount` | the amount after the `beforeArmor` listeners |
| `armorAmount` | the amount DAMAGED reported, after armor; nil before that |
| `cancelled` | whether `cancel()` was called |
| `paired` | false when DAMAGED came without a known DAMAGING (§5) |

The fields are for reading. Writing one changes nothing in the game.

```lua
function Hit:setAmount(amount)       -- finite and not negative; does nothing on a cancelled hit
function Hit:cancel()                -- the amount becomes 0 and stays 0 in the later phases
function Hit:setAttackType(attackType)   function Hit:setDamageType(damageType)   function Hit:setWeaponType(weaponType)
function Hit:isLethal()              -- target:getLife() - amount <= 0.405; false for a disposed target wrapper
```

- A setter calls the wrappers event's setter at once and updates the field, so later listeners see the change.
- A setter works only while the hit is the one being handled (`system:getCurrent()`) in a modifier phase. This guard
  is needed: Warcraft's setters act on the innermost event, so a listener of a nested native hit must not change the
  outer hit through a kept reference. Otherwise it raises:
  - in an observer: `[systems] Hit.setAmount: observers cannot change a hit`;
  - a type setter after armor: `[systems] Hit.setDamageType: types can change only before armor` (the game ignores a
    later change; measured by the wrappers v0.7.0 gate);
  - at any other time: `[systems] Hit.setAmount: the hit is not being handled`.
- `isLethal()` is a heuristic for `afterArmor`: later listeners, mana shield and native effects can still change the
  outcome.

## 4. The pipeline

**DAMAGING.** The system resolves the source (§6), claims metadata (§5.1), creates the Hit with phase `'beforeArmor'`,
records it as pending, and runs the `beforeArmor` listeners. Then `beforeArmorAmount` is set.

**DAMAGED.** The system finds the hit's pending frame (§5.2), or creates an unpaired Hit. It sets `armorAmount` and
`amount` to the event's amount, the three type fields to the event's values, and the phase to `'afterArmor'`. For a
cancelled hit, `armorAmount` is still what the game reported, but `amount` is 0 and the system sets the event's amount
to 0. It runs the `afterArmor` listeners, then sets the phase to `'observe'` and runs the observers. Then queued deals
run (§5.1).

Listener order, in every phase:
- lower priority first; equal priorities in registration order; a priority must be a finite number;
- a listener added while a hit is in flight waits for the next hit: a hit runs the listeners that existed when its
  DAMAGING began (for an unpaired hit, when its DAMAGED began);
- a removed listener stops at once, even during a dispatch;
- `getCurrent()` is the hit whose listeners are running, restored to the outer hit after a nested native hit, and nil
  outside damage events.

Hits with no source run the pipeline too (`source` and `dealer` nil), so shields and caps still apply.

## 5. Script damage, pairing and limits

### 5.1 `deal`

- Outside damage events, with nothing pending, `deal` runs at once.
- Inside a listener, or while a frame is pending, the request is queued. Queued requests run first in first out when
  the current hit has finished its DAMAGED phase or the pending frames have settled (§5.2). So script damage never
  nests inside another hit's listeners.
- **Metadata:** while a request's `damageTarget` call runs, the first DAMAGING event whose dealer and target are the
  request's `source` and `target` claims its metadata. A native hit nested inside it does not inherit it, and a hit
  between the same two units claims it at most once.
- A queued request whose `source` or `target` wrapper has been disposed by the time it runs is skipped.
- A `damageTarget` call that returns false is not reported: a dead or invulnerable target is a normal outcome.
  Observers show what landed.
- **`maxQueue`:** `deal` raises `[systems] DamageSystem.deal: the queue is full (<n> deals)` at the caller's line, so
  the report names the listener that looped.
- **`maxChain`:** the system counts the deals issued since the queue was last empty. When the count reaches
  `maxChain`, the rest of the queue is dropped and reported (label `Damage chain`, message
  `more than <n> deals in one chain; <k> queued deals dropped`). This stops listeners that answer every hit with
  another hit.

### 5.2 Pairing

- A DAMAGED event takes the newest pending frame with the same dealer, target and `isAttack`. Native hits nest, so the
  newest match is the right one.
- Warcraft does not always send DAMAGED (`wc3-lib` notes a hit fully blocked by spell immunity; the gate records it).
  A zero-second one-shot Timer, started by the first damage event after it last fired, drops every frame still
  pending and then runs queued deals. A dropped hit gets no `afterArmor` or observer call.
- A request's own frame that is still pending when its `damageTarget` call returns is dropped at once, so its metadata
  cannot reach a later hit.
- **`maxPending`:** when the pending frames reach the limit, the oldest is dropped.
- A DAMAGED event with no frame runs `afterArmor` and the observers on a new Hit with `paired = false`, no metadata,
  and `initialAmount` and `beforeArmorAmount` equal to the DAMAGED amount.
- None of these is reported: they are game outcomes, not failures.

## 6. Attribution

`options.sourceOf(dealer)` runs once per hit, when the Hit is created and the game gave a dealer:
- a Unit result becomes `hit.source`; nil keeps the dealer;
- a failure, or a result that is neither nil nor a Unit, is reported (label `Damage sourceOf`) and keeps the dealer.

A paired hit keeps the source resolved in DAMAGING. In a map:

```yue
dummies = Dummies.new clock
damage = DamageSystem.new sourceOf: dummies\sourceOf
damage\observe (hit) -> print hit.source\getName!, "via", hit.dealer\getName!
```

`Dummies:sourceOf` answers only while the dummy's lease is live, so a dummy's `duration` must cover its damage
(release 2's rule).

## 7. Failures and checks

Reported to `options.onError(message)` or printed as `[systems] <label> failed: <message>`, never rethrown:
- a failing listener (label `Damage listener`); the other listeners still run;
- a failing `sourceOf` (label `Damage sourceOf`);
- a chain over `maxChain` (label `Damage chain`).

`systems.internal.callback` gains `Callback.report(label, onError, message)`, the reporting half of `Callback.call`,
for the chain report.

Raised at the caller's line:
- `DamageSystem.new`: options that are not a table, `sourceOf` or `onError` that is not a function, a limit that is
  not a positive integer (`expected damage options: <field>`);
- `beforeArmor`, `afterArmor`, `observe`: a callback that is not a function, a priority that is not a finite number, a
  disposed system;
- `deal`: a system that is not started or is disposed; a request that is not a table; `source` or `target` that is
  not a live Unit wrapper; an `amount` that is not finite or is negative; `attack` or `ranged` that is not a boolean
  (`expected a damage request: <field>`); a full queue;
- the Hit setters (§3), and `setAmount` with a value that is not finite or is negative.

## 8. Changes from `wc3-lib`

- No port interface and no `createWarcraftDamage`: `DamageSystem.new` is the Warcraft system.
- The request is one flat table; `options` and `detail` are gone (the types are Hit fields).
- Setter methods replace mutable fields, and observers get the Hit itself, not a snapshot: its setters raise there.
  The `invalid-amount` issue is gone, because a setter rejects the value at the listener's line.
- `sourceOf`, `dealer` and hits with no source are new (`wc3-lib` dropped hits without a source).
- Only failures are reported. A missing DAMAGED, an unpaired DAMAGED, a dropped pending frame and a rejected native
  call are silent. A full queue raises instead of returning false, and `deal` returns nothing.
- Failures are never rethrown (`wc3-lib` threw when no `onError` was given).
- `isLethal` is a Hit method.

## 9. Verification

Automated (Part 1 §5):
- suite `damage`: `wc3-lib`'s 16 tests adapted to this API and run through the real wrappers on native doubles
  (ordered phases, removal and addition during a hit, nested native hits and `getCurrent`, queued deals and the chain
  limit, the queue limit, missing and unpaired DAMAGED, metadata claimed once, a failing listener, dispose inside a
  listener, start and dispose handles); and the new rules: every setter's phase guard, the outer-hit guard during a
  nested hit, `sourceOf` (a Unit, nil, a failure, a wrong value), hits with no source, a skipped deal with a disposed
  wrapper, `isLethal`;
- suite `internal`: `Callback.report`;
- the `blame` sweep over `DamageSystem` and `Hit`, and `imports`;
- integration: a one-module bundle for `systems.damage` (it bundles no `systems.dummy`, `systems.buffs` or
  `systems.scheduler`), LuaLS positive and negative fixtures for the new API, and the new gate example.

In-game gate (the maintainer, 3.0.0.24268): its own run, `deno task gate systems-damage`, from
`examples/gate-damage.yue`; releases 1 and 2 are not re-run. About 10 seconds, one step per second:
1. **Baseline:** a 100-damage `deal` between two footmen prints the amount before armor, after armor and in the
   observer; the life lost equals the final amount.
2. **Modifiers:** a `beforeArmor` listener doubles the hit with metadata `"crit"`, and an `afterArmor` listener caps
   it; the life lost equals the cap.
3. **Cancel:** a cancelled hit changes no life. Record whether the observer line prints (whether DAMAGED fires for a
   zero amount).
4. **Attribution:** a dummy's Storm Bolt with the paladin as source prints `source Paladin dealer Dummy`.
5. **Queue:** a listener's follow-up `deal` prints after the first hit's observer line.
6. **Spell immunity:** magic damage on a Spell Breaker. Record whether the observer line prints; no failure line may
   print either way.
7. **A real attack:** a footman ordered to attack prints `isAttack true`.
8. **Dispose:** after `dispose()`, a further hit prints nothing.

Then tag `v0.3.0` and check tag consumption with both gate examples.

## 10. Out of scope

Per-unit listener indexes (a listener looks its target up in its own table); a stage after the game has changed the
unit's life; damage-over-time helpers (a buff with `onTick` calling `deal`); shields as a ready-made buff; missiles
and knockback (release 4).
