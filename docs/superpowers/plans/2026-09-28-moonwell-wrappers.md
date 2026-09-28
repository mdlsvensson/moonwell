# Moonwell Native Wrappers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox syntax for tracking. The maintainer instructed implementation on 2026-09-28; execute inline,
> review each task, then request an independent final review. Follow AGENTS.md: main, maintainer pushes.

**Goal:** Ship a locally verified annotated Lua wrapper library for the six classes in the approved 4c spec.

**Architecture:** A separate `../moonwell-wrappers` repository holds handwritten modules in `src/wrappers`.
An internal handle registry enforces identity and disposal; a callback boundary protects asynchronous native calls.
Consumers use Moonwell's existing path/GitHub library sync without CLI changes.

**Tech Stack:** Lua 5.3, LuaLS annotations, YueScript 0.34.2 test VM, Deno tooling, Moonwell 0.5.0 consumer fixtures.

**Spec:** `docs/superpowers/specs/2026-09-28-moonwell-wrappers-design.md`.

## Global constraints

- Product files belong in `C:/Users/mdlsvensson/Repo/moonwell-wrappers`; spec/plan remain here.
- No Node.js, npm packages or specifiers. Tools use Deno built-ins; no new dependencies are needed.
- All Lua ships under `src/`; no generated native globals ship in the library.
- No runtime dependency on Moonwell, no umbrella require, no import-time native calls.
- Annotated public classes use `MoonwellWrappers.*`; native lowercase types come from Moonwell.
- No publish or push. Game and first-tag checks stay explicitly pending until the maintainer performs them.
- New repo commits require all its automated checks. Moonwell commits require all AGENTS.md checks.

## Review focus

1. A wrong class or disposed argument must fail before a side-effecting native (tasks 1–4).
2. A callback that restarts/destroys its timer must not lose the new schedule (task 2).
3. Native cleanup that reenters Lua or raises must leave the old wrapper disposed (tasks 1–3).
4. Snapshot holes and subsequent membership changes must not corrupt returned arrays (task 4).
5. LuaLS must actually distinguish wrapper classes and narrow nil; annotations alone are insufficient (task 5).

## Files and interfaces

| File | Responsibility |
| --- | --- |
| `src/wrappers/internal/handle.lua` | `new(class, name)` registry: `wrap(raw)`, `require(value, operation)`, `dispose(value, operation)` |
| `src/wrappers/internal/callback.lua` | `call(label, fn, argument)`, `check(fn, operation)`, `nonnegative(value, operation)` |
| `src/wrappers/{player,unit,timer,trigger,group,effect}.lua` | Public methods exactly as spec section 5 |
| `tests/support.lua` | Assertions and call-recording doubles only for unavailable Warcraft natives |
| `tests/{unit,timer,trigger,group,effect}.lua` | Behavior tests loading actual library modules |
| `tools/test.ts` | Run Lua test suites through Yue with a test-only module loader |
| `tools/check-lua.ts` | Run pinned Lua 5.3.6 luac parser over source modules |
| `tools/integration.ts` | Fresh Moonwell consumer, normal/minified builds, LuaLS positive/negative tests |
| `README.md`, `CONTRIBUTING.md`, `CHANGELOG.md` | Usage, tooling, gate and unreleased status |

Registry membership is maintained outside wrapper objects, so a forged table/metatable cannot pass validation.
Dispose returns the raw handle once, then nil on later calls; it clears `.handle` before returning.
Public factories assert native creation success before wrap. Each class declares concrete LuaLS signatures, while
internal helper types are generic. Avoid dynamic method generation: readable explicit wrappers support diagnostics.

## Task 1: Registry, Player and Unit

- [x] Create minimal Deno configuration, ignore rules, test support and runner; establish pinned Yue invocation.
- [x] Write `tests/unit.lua` first. Tests include these assertions with recorded native arguments:

```lua
local p = Player.fromIndex(0)
local u = Unit.create(p, 1751543663, 10, 20, 270)
eq(Unit.fromHandle(u.handle), u)
eq(Unit.fromHandle(nil), nil)
u:setPosition(30, 40)
expectCall("SetUnitPosition", u.handle, 30, 40)
local raw = u.handle
u:remove()
eq(u.handle, nil)
eq(u:isDisposed(), true)
u:remove()
eq(callCount("RemoveUnit"), 1)
fails(function() u:getLife() end, "disposed")
fails(function() Unit.create({}, 1751543663, 0, 0, 0) end, "Player")
```

- [x] Run `deno task test unit`; expect missing module failure, then implement the registry and both classes.
  The registry uses strong raw-handle keys and private per-instance raw-handle membership. Player validates its
  zero-based integer index against bj_MAX_PLAYER_SLOTS. Unit unwraps Player/Unit arguments before invoking natives.
- [x] Cover every section-5 mapping with literal return values and ordered native arguments; additionally test nil
  factory results, current native values, no import side effects, kill versus removal, reentrant cleanup, failed
  native destruction, forged receivers and argument disposal. Run full `deno task test`; expect pass.
- [x] Review the task against spec sections 3–5 and 7; record findings and results in the execution ledger.

## Task 2: Timer and callback errors

- [x] Write `tests/timer.lua` first, using a TimerStart double that captures each real wrapper callback:

```lua
local t = Timer.create()
local hits = 0
t:start(1, true, function(self) eq(self, t); hits = hits + 1 end)
local old = capturedTick()
t:start(2, false, function(self) self:destroy() end)
old()
eq(hits, 0)
capturedTick()()
eq(t:isDisposed(), true)
```

- [x] Run `deno task test timer`; expect missing module failure. Implement callback pcall/reporting and validation.
  Timer stores a replaceable callback cell and generation token; closure reads current cell only after token/liveness
  checks. Destroy clears the cell before PauseTimer/DestroyTimer. Single-shot delivery clears only its own callback
  before invocation, preserving a schedule installed from inside it.
- [x] Test all getters, pause/resume, replacement, one-shot retention, self-restart, self-destruction, stale callbacks,
  invalid callbacks, negative/NaN/infinite timeouts, printed errors and subsequent successful ticks.
- [x] Run full `deno task test`; expect pass. Review task and record evidence.

## Task 3: Trigger

- [x] Write `tests/trigger.lua`, including registrations with exact handles and explicit nil filters:

```lua
trigger:registerPlayerUnitEvent(player, event)
expectCall("TriggerRegisterPlayerUnitEvent", trigger.handle, player.handle, event, nil)
trigger:addAction(function(self) self:destroy() end)
local action = capturedAction()
action()
action()
eq(callCount("DestroyTrigger"), 1)
```

- [x] Run `deno task test trigger`; expect missing module failure. Implement public trigger methods using the
  registry and callback helper. Use callback cells so disposal releases user closures even if a native retains its
  action thunk. Validate receiver, object arguments and timing before any mutation.
- [x] Test all native mappings, callback failures with later success, multiple actions, disposal in one action,
  invalid wrapper types and cleanup failures. Run all tests; expect pass. Review and record evidence.

## Task 4: Group and Effect

- [x] Write `tests/group.lua` and `tests/effect.lua` before their production modules:

```lua
local snapshot = group:getUnits()
eq(#snapshot, 2) -- native indices contain unitA, nil, unitB
eq(snapshot[1], Unit.fromHandle(unitA))
group:clear()
eq(#snapshot, 2)
group:destroy()
eq(callCount("RemoveUnit"), 0)
effect:destroy()
eq(callCount("RemoveUnit"), 0)
```

- [x] Run both suites; expect missing module failures. Implement explicit Group and Effect methods. Group snapshots
  skip nil handles; enumeration validates radius before clearing and passes nil filter. Effect.attach unwraps Unit.
- [x] Test all mappings, range boundaries, snapshot independence, disposed members, disposal idempotence and no
  target ownership. Run all tests; expect pass. Review and record evidence.

## Task 5: Compatibility, editor and consumer integration

- [x] Use pinned Lua 5.3.6 `luac -p` for syntax validation. Accept a `MOONWELL_LUAC` override; require output version
  to match 5.3.6. If unavailable locally, fetch official Lua 5.3.6 source and compile with an available C compiler;
  keep downloaded tools outside source control and document checksums and reproduction.
- [x] Write the consumer test using `MOONWELL_REPO`, `MOONWELL_PKL`, `MOONWELL_YUE` and `MOONWELL_LUALS` overrides.
  Create an isolated ignored `.test-work/consumer`, scaffold with the real CLI `init --link`, configure the library
  path, then run check/build and minified build. Keep the sample source free of unavailable native references.
- [x] Add direct Lua and compiled Yue positive fixtures; negative fixtures call `group:add(Timer.create())`, an
  unknown Unit method, and assign callback/narrowed values to incompatible types. Run LuaLS with diagnostics output
  and assert exact expected categories/locations, not merely a nonzero exit code.
- [x] Verify unused Effect/Trigger/Group modules stay out of a Unit-only bundle; run a bundle consumer with native
  doubles to verify imports execute. Fix any editor/runtime mismatch with failing regression evidence first.
- [x] Run `deno task check`, `deno task lint`, `deno fmt --check`, `deno task test`, `deno task check:lua`,
  `deno task test:integration`; expect all automated checks pass, or explicitly report external blockers.
- [x] Review tooling and test evidence; record versions and results.

## Task 6: Documentation, final review and handoff

- [x] Write README with complete path/GitHub examples, all public methods, explicit cleanup, native escape hatches,
  typed editor setup and callback limitations. Label GitHub v0.1.0 as a future release until published.
- [x] Write CONTRIBUTING with reproducible commands, required tool versions, release instructions and the full
  in-game/first-tag gate. Add MIT license and Unreleased changelog; do not claim game verification.
- [x] Add a runnable consumer gate example covering all six wrappers and callback probes; make cleanup explicit.
- [x] Request an independent fresh-context review of the whole library and tests against the approved spec. Fix
  important findings using regression tests, then rerun affected checks and the full automated suite.
- [x] Update Moonwell AGENTS.md with implementation state and outstanding manual gates. Run its required checks
  before committing its documentation. Commit the new library on main only after its automated checks pass.
- [x] Hand off local paths, test evidence and pending maintainer gate; do not push, create release tags or publish.

## Execution ledger

- 2026-09-28: Spec approved and implementation authorized. Separate local repository initialized on main.
- Ruling: Execute inline with per-task self-review and an independent final review; the user delegated implementation
  choices and asked to proceed. Do not stop for another planning permission round.
- Pre-flight: Tasks 2–4 consume Task 1 registry; Task 3 consumes Task 2 callback helper. Interfaces above agree.
- Ruling: Keep progress in this plan rather than shell-specific skill scratch scripts; it survives the Windows
  environment and records all deviations alongside the task checklist.
- Tasks 1–4: implemented and self-reviewed. Tests failed first with each missing module, then all 23 behavior tests
  passed under YueScript 0.34.2. Independent runtime review found no critical/important issues; minor diagnostic
  context concern retained: invalid wrapper arguments name their conversion method (e.g. Unit.getHandle).
- Task 5: normal/minified consumer builds, real bundled runtime and unused-module exclusion passed. LuaLS 3.19.1
  positive Lua/Yue fixtures passed; all four intentional negative diagnostics matched their expected locations.
- Ruling: LuaLS merges overload return types. Keep fromHandle honestly nullable, narrow/assert at call sites, and
  validate factory results with Handle.created. No broad type suppression. Two local nil-filter exceptions bridge
  native JASS annotations. Documented in spec and README; runtime behavior is unchanged.
- Task 5: Lua 5.3.6 parser checked 16 source/test Lua files and rejected a Lua 5.4-only control fixture. The binary
  download returned HTML, so the checker was compiled from official Lua source using TinyCC 0.9.27 in ignored .tools.
- Ruling: defer commits to the final full verification rather than committing partially verified repository setup;
  this preserves AGENTS.md's all-checks-before-commit rule. No pushes, tags or publications performed.
- Task 6: independent final review found a PATH-only compiler configuration defect in integration tooling. Reproduced
  failure first; fixed by omitting the manifest override when unset and resolving provided overrides against the
  library root. Full integration then passed with PATH-only and relative override configurations. No important runtime
  findings. Minor conversion-operation diagnostic context is documented above; no unsafe behavior is deferred.
- Tasks 5–6: gate example compiles/builds and LuaLS reports no diagnostics. Checks, lint, formatting, 23 behavior tests,
  16-file Lua 5.3.6 syntax check and all consumer/editor integration checks pass. Library committed as `3b923d5`.
- Parent repository verification: check/lint/format pass; 443 unit, 28 runtime, 25 Pkl integration, 32 e2e and 1 network
  tests pass, plus the Pkl schema tests. The initial formatting finding in AGENTS.md was fixed and formatting rechecked.
- 2026-09-28 manual gate: maintainer confirmed normal gameplay including cleanup, intentional callback-error recovery
  through later ticks and cleanup, minified packed-map gameplay, and opening the packed map in World Editor. Game
  3.0.0.24268; World Editor 3.00 (file version 3.0.0.24268). The probe screenshot shows both intentional errors and
  subsequent ticks/death/cleanup in F12, correcting the earlier blanket statement about print retention. Disposable
  gate projects, logs and map hashes are under the wrapper repository's ignored `.test-work/release-gate-2026-09-28/`.
- Release: `mdlsvensson/moonwell-wrappers` was created, main pushed and `v0.1.0` tagged on `c1209f5`
  (GitHub pre-release). The first GitHub-tag consumption gate passed 2026-09-28 with Moonwell 0.5.0: check, normal and
  minified builds; `moonwell.lock` recorded `c1209f5`, the fetched files matched the tag's `src/`, and the lock stayed
  unchanged after removing the map's `.moonwell/`. Recorded in the wrapper repository as `3a62111`.
