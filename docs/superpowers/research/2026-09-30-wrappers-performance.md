# Wrappers performance on the game's Lua (2026-09-30)

Roadmap item 1.2 (`docs/superpowers/plans/2026-09-30-moonwell-roadmap-to-wc3-lib.md`). The WCSharp comparison (§4.1)
asked for a performance budget before the `wc3-lib` port builds systems on top of the wrappers. This note measures the
same work done through raw natives and through moonwell-wrappers v0.5.1, on Warcraft III 3.0.0.24268.

## Method

- **The probe:** `../wrappers-gate/src/probe_perf.yue`, run with `deno task gate perf` (instructions in
  `PROBE-PERF.md`). It was run once by the maintainer in a normal (not minified) build.
- **The raw results:** written with the Preload natives to `CustomMapData\moonwell-perf.pld`, and copied into
  `../wrappers-gate/PROBE-PERF-RESULTS.md`.
- **Timing:** each case times a loop of N iterations with `os.clock` and keeps the median of 3 runs.
  - The cases run one per second, starting after load.
  - Each loop body is exactly one call. We checked the compiled Lua: YueScript turns a trailing loop into a
    table-building expression, so every body ends in an explicit `return`.
- **Precision:**
  - `os.clock` ticks in steps of about 1 ms (measured: 1,003,265 ns).
  - With N = 100,000, a result is therefore quantised to about 10 ns per call, and a case that takes 30 ms has a
    rounding error of about 3%.
  - These are single-machine, single-run numbers. Ratios are more reliable than absolute values.

## Results

| Case | Raw | Wrapper | Ratio | Wrapper cost |
| --- | --- | --- | --- | --- |
| Empty loop body (`local value = i`) | < 10 ns | — | — | — |
| Plain Lua function call | 20 ns | — | — | — |
| `GetUnitX` / `unit:getX()` | 320 ns | 440 ns | 1.38 | +120 ns |
| `SetUnitX` / `unit:setX(x)` | 460 ns | 580 ns | 1.26 | +120 ns |
| `BlzSetSpecialEffectPosition` / `effect:setPosition` | 470 ns | 590 ns | 1.26 | +120 ns |
| Table lookup by handle / `Unit.fromHandle` (cached) | 20 ns | 70 ns | 3.5 | +50 ns |
| Enumerate 20 units in range and read each x | 19.4 µs | 29.2 µs | 1.51 | +9.8 µs |
| `fn()` / `pcall(fn)` / `Callback.call` | 20 ns | 50 ns / 70 ns | 2.5 / 3.5 | +50 ns |
| Reading 3 fields / `Options.read` with 3 fields | 20 ns | 950 ns | 47.5 | +930 ns |
| Missile tick, 100 missiles | 0.97 ms | 1.69 ms | 1.74 | +0.72 ms |

**How the missile tick was measured.** Each of 100 missiles moves inside a 400 × 400 square containing 20 footmen. Every
tick it sets its effect position and enumerates the units within 150 of itself, reading each unit's x. The raw variant
uses `GroupEnumUnitsInRange` with `BlzGroupUnitAt`. The wrapper variant uses `effect:setPosition`,
`group:enumInRange` and `group:getUnits()`. Both found about the same number of units (325,080 and 326,385 hits over
600 ticks; the missiles move on during each run). At 32 ticks per second a tick has 31.25 ms, so the raw tick uses 3.1%
of it and the wrapper tick 5.4%.

## What the numbers say

1. **Natives dominate.** A native call costs 300–470 ns, and a Lua function call only 20 ns. The wrapper's fixed cost
   per method is about 120 ns. That is `registry.require`: a function call, a registry lookup and a disposed check. On
   one native call it adds 26–38%.
2. **Enumeration costs the most.** A wrapped enumeration of 20 units costs 1.5 times the raw one: every unit is wrapped
   (`Unit.fromHandle`, about 50 ns more than a table lookup), and each call builds two snapshot tables. That makes the
   group the largest part of the missile tick's extra 7.2 µs per missile.
3. **Options tables belong on creation paths only.** `Options.read` costs about 1 µs. Today it runs only when objects
   are created and in one-shot helpers (`TextTag.float`, `Sound.playOnce`), never per tick, and that should stay so.
4. **The error boundary is cheap.** `Callback.call` costs 50 ns more than a direct call, and 20 ns more than a bare
   `pcall`. Per timer tick or per event, that is negligible.
5. **Budget.** Through the wrappers, 100 missiles take 5.4% of a tick, against 3.1% raw. Scaled linearly, 500 missiles
   would take 27% against 15%. For the port's systems the wrappers are affordable, but not free. The overhead scales
   with enumerations far more than with setters.

## Proposed budget and recommendations

- **Budget:** a system's per-tick work should stay under 10% of a tick (3 ms) at the load a map expects. The port's
  gates should measure it the same way. The wrappers meet this for 100 missiles with room to spare.
- **For the review (roadmap 2.1), two candidates:**
  - **A cheaper method prologue.** Methods could read the raw handle from the wrapper's own `handle` field, which is nil
    once disposed. That would replace the `registry.require` call and should save much of the 120 ns. It must keep the
    error message for a disposed wrapper.
  - **Cheaper enumeration.** Do not allocate two snapshot tables per call. For example, reuse a buffer, or add an
    iteration that wraps lazily. The snapshot semantics (safe against changes during iteration) must stay.
- **For decision D1 (how the port's adapters reach the game):** the numbers support option (c), a mix.
  - Adapters use the wrappers.
  - The innermost per-tick loops (missile movement and collision, knockback) may keep raw handles from `getHandle()`
    for the objects they own.
  - Otherwise, the wrappers must first get the cheaper paths above.

  Either way, the measured cost is well within budget at realistic loads.

## Also measured (context for roadmap 1.4)

- **Numbers.**
  - `math.maxinteger` is 2,147,483,647, so integers are 32-bit.
  - `2^24 + 1` equals `2^24`, so floats are single precision.
  - Both confirm `wc3-lib`'s `AGENTS.md` fact 1.
- **`os.time` exists as a function.** This agrees with Moonwell's Plan 3a probe, and contradicts `wc3-lib`'s fact 3,
  which says it does not exist. Whether calling it works, and what it returns, is still for 1.4 to check.
- **Preload writes work.** `PreloadGenClear`, `PreloadGenStart`, `Preload` and `PreloadGenEnd` wrote
  `CustomMapData\moonwell-perf.pld` on the local machine. The file is JASS: every line is wrapped in
  `call Preload( "…" )`. 1.4's round trip still needs the read back.
