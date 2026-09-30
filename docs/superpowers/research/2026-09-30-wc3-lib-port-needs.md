# What the wc3-lib port needs from the wrappers (2026-09-30)

Roadmap item 1.3 (`docs/superpowers/plans/2026-09-30-moonwell-roadmap-to-wc3-lib.md`). It compares what `wc3-lib` 0.1.1
(`../wc3-lib`, TypeScript for TypeScriptToLua) uses in Warcraft with what moonwell-wrappers v0.5.1 offers. It also
collects the design inputs the port's spec must answer. Sources:
- `wc3-lib`'s own natives list (`docs/natives.md`), its adapters (`*/warcraft*.ts`), its cores and its `AGENTS.md`;
- the w3ts and WCSharp comparisons;
- the performance note of the same day (`2026-09-30-wrappers-performance.md`).

## 1. Shape of wc3-lib

Every system is a pure core, generic over the target type (`T`, a `unit` in Warcraft), plus a port interface and one
thin adapter that alone calls natives. The systems are:

- the scheduler (one 1/32 s timer), scope and signal;
- time (UTC from `os.date`);
- dummies;
- buffs and auras;
- damage (a DAMAGING and DAMAGED pair);
- missiles and knockback;
- persistence: a save-code codec, Preload local files, and multiplayer sync in chunks.

Importing a module allocates nothing, which matches the wrappers' rule. The cores need no natives at all. So in the port
they can be tested in `yue -e` without native doubles, and only the adapters need doubles, as in the wrappers'
`tests/support.lua`.

## 2. Natives: covered, missing, or best left raw

A mechanical comparison found 76 natives that `wc3-lib` calls. The wrappers call 46 of them, and 30 not at all. Of those
30, four are helpers or constructors rather than capabilities (`BJDebugMsg`, `FourCC`, `Preloader`, `Location`). The
table is by feature. "Wrapper" names the method that covers the need today.

| Need (adapter) | Natives | Wrapper today | Verdict |
| --- | --- | --- | --- |
| Heartbeat (clock) | `CreateTimer`, `TimerStart`, `PauseTimer`, `DestroyTimer` | `Timer.create`, `timer:start(t, true, cb)`, `pause`, `destroy` | Covered |
| Unit events (damage) | `CreateTrigger`, `TriggerRegisterPlayerUnitEvent`, `TriggerAddAction`, `TriggerRemoveAction`, `DestroyTrigger` | `Trigger.create`, `registerAnyUnitEvent(EVENT_PLAYER_UNIT_DAMAGING)`, `addAction` returning a token, `removeAction`, `destroy` | Covered |
| Damage event data | `GetEventDamage`, `GetEventDamageSource`, `BlzGetEventDamageTarget`, `BlzGetEventIsAttack`, `BlzGetEventAttackType/DamageType/WeaponType` | — | **Missing** |
| Changing damage in the event | `BlzSetEventDamage`, `BlzSetEventAttackType/DamageType/WeaponType` | — | **Missing** |
| Dealing damage | `UnitDamageTarget` | `unit:damageTarget(target, amount, attack, ranged, attackType, damageType, weaponType)` | Covered |
| Units (dummy, physics) | `CreateUnit`, `RemoveUnit`, `GetUnitX/Y`, `SetUnitX/Y`, `IsUnitType`, `GetWidgetLife`, `GetUnitTypeId` | `Unit.create`, `remove`, `getX/getY`, `setX/setY`, `isType`, `getLife`, `getTypeId` | Covered |
| Dummy setup | `UnitAddAbility`, `GetUnitAbilityLevel`, `SetUnitAbilityLevel`, `SetUnitInvulnerable`, `GetUnitState`/`SetUnitState` (mana) | `addAbility`, `getAbilityLevel`, `setAbilityLevel`, `setInvulnerable`, `getMana`/`setMana`/`getMaxMana` | Covered |
| Dummy pathing | `SetUnitPathing` | — | **Missing** (one method) |
| Orders | `IssuePointOrder`, `IssueTargetOrder`, `IssueImmediateOrder` | `issuePointOrder`, `issueTargetOrder`, `issueOrder` | Covered |
| Hit radius | `BlzGetUnitCollisionSize` | — | **Missing** (one method) |
| Unit searches | `CreateGroup`, `GroupEnumUnitsInRange`, `FirstOfGroup`, `GroupRemoveUnit`, `GroupClear`, `DestroyGroup` | `Group.create`, `enumInRange`, `first`, `remove`, `clear`, `forEach`, `getUnits` | Covered, at 1.5 times the raw cost (performance note) |
| Missile visuals | `AddSpecialEffect`, `BlzSetSpecialEffectPosition`, `DestroyEffect` | `Effect.create`, `setPosition`, `destroy` | Covered |
| Terrain walkability | `IsTerrainPathable` (returns true when **not** pathable) | — | **Missing**; a small stateless query |
| Ground height | `Location`, `MoveLocation`, `GetLocationZ`, `RemoveLocation` | — (no Point wrapper) | **Leave raw**: opt-in, and possibly different between machines (w3ts) |
| Sync | `BlzTriggerRegisterPlayerSyncEvent`, `BlzGetTriggerSyncData`, `BlzSendSyncData`, `GetTriggerPlayer` | — | **Missing**; in the backlog's candidate additions |
| Who is local | `GetLocalPlayer`, `Player`, `GetPlayerId` | `Player.fromIndex`, `player:isLocal()`, `getId()` | Covered |
| Local files | `PreloadGenClear/Start/End`, `Preload`, `Preloader`, `BlzGetAbilityTooltip`, `BlzSetAbilityTooltip` | — | **Leave raw**: a narrow, machine-local trick with a borrowed tooltip; it belongs in the port's persistence adapter |
| Tie-break ordering | `GetHandleId` | — (never exposed) | **Replace**: see §3.2 |
| Ability codes | `FourCC` | Moonwell's `$FourCC` macro | Covered |

## 3. Design inputs for the port's spec

### 3.1 Ordered collections

Several cores iterate TypeScript `Map`s and `Set`s keyed by units, in insertion order:

- buffs, `byTarget` (`buffs.ts:270`);
- auras, `members`;
- knockback, `active` (`knockback/system.ts:141,158`);
- missiles, the in-flight set.

TypeScriptToLua compiles these to ordered collections. A plain Lua table keyed by handles or wrappers iterates with
`pairs` in hash order, which depends on addresses and can differ between machines. So an iteration that calls natives,
or that changes game state, would desync. The wrappers' `AGENTS.md` already forbids "iterate a table keyed by tables
when the loop calls natives". **The port needs a small ordered map and set**, with insertion order and removal during
iteration, and must never use `pairs` over handle-keyed tables.

### 3.2 Handle ids must not order anything

`wc3-lib` breaks missile contact ties by `GetHandleId`, and sorts some other results by handle id (`patterns.md`: "missile
hits sort by distance along the path, then handle ID"). The WCSharp comparison found that handle ids in Lua can differ
between machines, because an id is reused only after garbage collection. Wrapper identity is no substitute: Unit wrappers
are weakly cached, so a wrapper collected on one machine and not on another gets re-created there.

**The port must order by keys it owns:** a sequence number the system assigns when it first sees a target, kept in a
strong ordered map until the target leaves. Engine enumeration order (`GroupEnumUnitsInRange`) is synchronized and can
be kept.

### 3.3 Error boundaries

`wc3-lib`'s update loops "collect errors and keep going". The wrappers' `Callback.call` does the same per callback, and
costs about 70 ns (performance note). The port's scheduler, signals and system loops should isolate each callback the
same way. Open question: expose the wrappers' internal `Callback` module as a public `wrappers.callback`, or give the
port its own copy.

### 3.4 Liveness and removed units

Three definitions exist today:

| Where | Definition |
| --- | --- |
| `wc3-lib`'s `living` | type id ≠ 0, life > 0.405, and not `UNIT_TYPE_DEAD` |
| The wrappers' `isAlive` | not dead, and type id ≠ 0 |
| The `UnitAlive` native | confirmed in map Lua; known to Moonwell since 0.5.1 |

The port and the wrappers should share one definition. Roadmap item 2.2 plans `isAlive` through `UnitAlive`. For units
the game removes by itself, `wc3-lib` does not use events: `trackWarcraftBuffTargets` prunes its store every 0.25 s,
keeping only units whose type id is still non-zero. That polling is a working alternative for the backlog item
"Automatic disposal of Unit wrappers on removal". The 1.4 probe found that the undefend order works too (§6.3), but it
needs object data in every map.

### 3.5 Numbers and time

Measured by the 1.2 probe:

- integers are 32-bit (`math.maxinteger` = 2^31−1) and floats single precision, as `wc3-lib` says;
- `os.time` exists as a function, although `wc3-lib`'s `AGENTS.md` fact 3 says it does not. The 1.4 probe found that it
  also works (§6.1).

`wc3-lib` reads UTC through `os.date("!*t")`, which works. The codec's bounds (±2^31−1, a checksum folded to 31 bits)
carry over unchanged.

### 3.6 Compiler hazards change

The TypeScriptToLua hazards (`finally`, loop closures, truthiness of `0` and `""`, `this: void`) drop out. YueScript's
own hazards replace them:

- floor division `//` empties a file with `-r` and `-m` (Moonwell 0.5.2 now fails the build; IppClub/YueScript#256);
- a function whose last statement is a loop returns a table built by that loop (found by the performance probe; bodies
  end in an explicit `return`);
- the loop variable is constant;
- `close` is a keyword;
- the last expression is returned implicitly.

Lua semantics stay: `0` and `""` are truthy, `%` is floored, and `table.sort` is unstable.

### 3.7 Things that are already right

- **One heartbeat timer:** a periodic `Timer`.
- **Explicit start and dispose:** the wrappers' explicit cleanup.
- **Nothing at import:** the same rule on both sides.
- **Isolated callbacks:** `Callback.call`.
- **Deterministic ordering by creation order:** the wrappers already use creation order for frame contexts.

The two designs agree on all of these.

## 4. What this means for decision D1

D1 asks how the port's adapters reach the game: (a) through the wrappers only, (b) raw natives, or (c) a mix.

- **(a) Wrappers only** would need all of §2's missing rows as wrapper additions first. That includes two, Preload files
  and ground height, that do not fit the wrappers' rules: they are machine-local tricks with no owned object to wrap.
- **(b) Raw natives** would give up the wrappers' checks and identity in exactly the code maps call most. The maps'
  own objects (their units) would also cross the boundary as raw handles, while the rest of a Moonwell map uses wrappers.
- **(c) A mix, recommended.**
  - The port's public API takes and returns wrappers (`T` = `MoonwellWrappers.Unit`).
  - Adapters use wrapper methods.
  - Raw natives stay in four places: damage event data (unless 2.3 adds it), Preload files with the tooltip mailbox,
    ground height and walkability sampling, and the innermost per-tick loops (missiles, knockback). The last may keep
    `getHandle()` results for objects the system owns, which the performance note allows for.

**Wrapper additions under (c), for roadmap 2.3:**
- `unit:getCollisionSize()` and `unit:setPathing(flag)`;
- sync: a trigger registration, sending, and the event's data;
- damage event data, if the maintainer wants damage available to maps without the port; otherwise it stays raw inside
  the port's damage adapter.

Walkability could be a small `wrappers.terrain` helper (`isWalkable(x, y)`, with Warcraft's inverted name hidden), or
stay raw.

## 5. Open questions for the port's spec

1. **Callback boundary:** a public `wrappers.callback`, or a copy in the port (§3.3).
2. **Ordered collections:** where the ordered map and set live. Inside the port, or in the wrappers as `internal/`?
3. **Where damage event data lives:** in the wrappers (for every map) or in the port only.
4. **One liveness definition** (§3.4), and whether the port prunes removed units by polling, as `wc3-lib` does, or waits
   for the backlog's automatic disposal.
5. **Library layout:** one library with one module per `wc3-lib` entry point (the opt-in rule), and its name.
6. **Preload carriers:** how a save code longer than one line is read back (§6.5). This needs its own probe when the
   persistence release is designed.

## 6. Measured by the 1.4 probe (2026-09-30)

The probe is `../wrappers-gate/src/probe_port.yue`, run with `deno task gate probe-port` on 3.0.0.24268. The raw lines
are in `../wrappers-gate/PROBE-PORT-RESULTS.md`. The first run crashed to desktop (§6.5); the second ran all 130
seconds.

### 6.1 Time

`os.time()` works. It returned the integer 1,790,760,132, the correct UTC epoch for the `os.date("!*t")` of that moment.
`os.time(fields)` reads its fields as local time, so passing it the UTC fields gave a value 7,200 lower, the maintainer's
UTC offset. So `wc3-lib`'s `AGENTS.md` fact 3 ("`os.time` does not exist") is wrong on 3.0.0.24268; `readWarcraftUtc`
through `os.date` works too. The epoch is a 32-bit integer, so it overflows in January 2038.

### 6.2 Numbers

Integers wrap silently:
- `math.maxinteger + 1` is `math.mininteger` (−2^31);
- `65536 * 65536` is `0`.

Parsing and printing large numbers:
- `tonumber` of a number past the 32-bit range gives an imprecise float (`2.147484e+09`).
- `string.format("%d", 2^31)` raises "number has no integer representation", while `-2^31` formats.
- Floats are single precision: `0.1 + 0.2` is `0.299999982`, and `16777217.0` prints as `1.677722e+07`.

The port's codec must keep every intermediate value within ±2^31−1 (as `wc3-lib` already does) and never multiply two
values that could overflow.

### 6.3 Unit removal: the "undefend" order works

Every unit got a Defend copy with no research requirement, and a trigger caught order 852056 (`undefend`):

| Case | Events at death | Events at removal |
| --- | --- | --- |
| `RemoveUnit` on a living unit | 2, Defend level 1 | 2, level 0, in the same instant |
| Exploding death | 2, level 1 | 2, level 0, in the same instant |
| Timed life (2 s) | 2, level 1 | 2, level 0, when the corpse was gone (93 s later) |
| Normal death | 2, level 1 | 2, level 0, when the corpse was gone (93 s later) |
| Alive control unit | none | none |

Every event came twice. In every event the unit already reported dead, with 0 life and its type id unchanged.

**So removal is: `undefend` with the Defend copy at level 0.** Deduplicate the pair. The same order at level 1 means
death. This makes the backlog item "Automatic disposal of Unit wrappers on removal" possible, where the world-bounds leave
event failed. But every unit then needs the Defend copy, which means a custom ability in the map's object data. A library
cannot ship object data yet (backlog "Assets shipped by libraries"). Polling type ids, as `wc3-lib` does (§3.4), needs
no object data.

### 6.4 Sync on one machine

`BlzTriggerRegisterPlayerSyncEvent` registered, and every `BlzSendSyncData` returned true. All seven messages arrived
together about 0.09 s later, in the order sent. A colon in the data arrived intact. **A 256-character message arrived
cut to 255 characters**, although the send had returned true. So the sender must enforce the limit itself; `wc3-lib`'s
chunks of 160 plus a short header stay under it.

### 6.5 Preload: a 259-character limit, a crash, and no appending

- **Lines keep at most 259 characters.** Lines of 258, 259, 260 and 300 characters were kept at 258, 259, 259 and 259.
- **A cut line can crash the game.** In the first run, the second and third lines were 268 characters long. They lost
  the closing `", 0)` of their `BlzSetAbilityTooltip` call. Reading the file back with `Preloader` crashed Warcraft to
  desktop: an `ACCESS_VIOLATION` at address 0. So a save file must never contain a line over 259 characters.
- **Appending in the file does not work.** The files were written with every line whole. Each later line sets the
  tooltip to `BlzGetAbilityTooltip(id, 0) + "<chunk>"`, as `wc3-lib` does. Reading back gave only the first line's
  chunk: 150 of 450 characters, and 209 of 551.
- **A missing file reads as nothing.** The tooltip kept the value set before `Preloader`, and the borrowed tooltip was
  restored.

**`wc3-lib`'s `PreloadLocalStore` is therefore broken for any code longer than one chunk (90 characters, since its hex
doubles the length):**
- with its 180-character chunks, the later lines are 268 characters, so the file can crash the game when it is loaded;
- even within the limit, only the first chunk reads back.

Its own tests use fake ports, and its `AGENTS.md` records writes as "issued-unverified". The port needs another way to
carry more than one line, for example one tooltip (or ability) per chunk, read back and joined in Lua. That needs its
own probe (open question 6).
