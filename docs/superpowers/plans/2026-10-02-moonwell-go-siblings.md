# Moonwell in Go, Plan 5f: The Sibling Repositories — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** No Deno in moonwell-wrappers, moonwell-systems or wrappers-gate: their tools are Lua run with `yue -e`, and
they run the `moonwell` program.

**Architecture:** moonwell-systems already has the pattern: `tools/lib.lua` (shell, files, a JSON reader, LuaLS),
`tests/run.lua`, `tools/check.lua` and `tools/integration.lua`. The wrappers get the same files; the gate map gets a
`gate.lua` and a `preview-probe.lua` on a small library of its own.

**Tech Stack:** Lua 5.4 as the YueScript compiler runs it (`yue -e`), `io.popen` for other programs. No library code
changes, so no tags.

**Spec:** `docs/superpowers/specs/2026-10-02-moonwell-go-toolchain-design.md` §10.

## Global Constraints

- No runtime module changes in either library (`src/`): tools, tests' runners and documents only.
- Every check a Deno tool made is still made, with the same failures: nothing is dropped silently.
- The tools find `moonwell` on the PATH, or run the program `MOONWELL` names.
- The tools' own Lua passes the Lua 5.3.6 syntax check, like the libraries' code.

## Decisions

- **`MOONWELL` names an executable,** not a command line: a path to a built `moonwell`. `go run ./cmd/moonwell` cannot
  stand in, because it runs in the checkout, and the commands must run in the consumer project.
- **`init --link` runs in the Moonwell checkout** (`MOONWELL_REPO`, else `../moonwell`), with the consumer's absolute
  path: the Go program finds the checkout by walking up from its working directory. The Deno CLI found it from its own
  file, so the old tools ran `init --link` from their own repository.
- **`MOONWELL_CLI` of moonwell-systems is gone;** `MOONWELL` replaces it. `MOONWELL_PKL` of moonwell-wrappers stays:
  its folder is put before the PATH of every command the tools run.
- **The wrappers keep one process per suite** (`tools/test.lua`), as `tools/test.ts` had: their support file installs
  globals for the whole run. moonwell-systems keeps its in-process runner.
- **The preview probe uses a project of its own.** `preview-probe.ts` imported the TypeScript CLI's packer and
  settings planner. The Lua tool creates a linked project under `gate-maps/preview/project/`, and for each variant
  writes the changed map folder as that project's source map, sets the map's name and description in its local
  manifest, and runs `moonwell build`. Only public commands are used, and the gate map's own manifests are not
  touched.
- **wrappers-gate is not a git repository:** its files change in place.

## Amendments made while implementing

- **The gate map's tools use the wrappers' `tools/lib.lua`** (`../moonwell-wrappers/tools/`), not a library of their
  own: the gate map already needs that checkout for `examples/gate.yue`.
- **The preview probe's project is this project again:** its manifest, objects, assets, Lua modules and generated
  ids are copied in, so a variant's map holds what the Deno tool's held. Maps 9 to 11 have the same 27 files with the
  same contents as the ones the Deno tool built. The first set (1 to 8) builds, but its old maps were no longer on
  disk to compare with.
- **Checked against the Deno tools' maps:** of the 28 gate maps, the three built since the libraries last changed
  (`additions`, `probe-extras`, `probe-input`) are identical in all 23 files; the others differ only by library code
  released after they were built (wrappers v0.8.0's `fromEvent`, object data added since).
- **Stale generated entries** (`src/probe_preview2_main.yue`, `_steps`, `_timer`) were removed from the gate map: the
  Lua tool writes them into its own project.
- **Comments** in moonwell-systems' five gate examples and in the gate map's probes name `yue -e gate.lua <run>`.
  Nothing under either library's `src/` changed.
- **The deleted `gate.ts`, `preview-probe.ts`, `deno.json` and `deno.lock`** of the gate map have no history to be
  found in; they are quoted in full in Plan 5f's working session and nowhere else.

## Inventory

| Deno tool | Lua tool | Checks kept |
| --- | --- | --- |
| wrappers `tools/test.ts` | `tools/test.lua` | YueScript 0.34.2; every `tests/<name>.lua` but `support`, one process each; `SUITE PASSED` |
| wrappers `tools/check-lua.ts` | `tools/check.lua` | Lua 5.3.6; every `.lua` under `src/`, `tests/` and now `tools/`; Lua 5.4 syntax refused |
| wrappers `tools/run.ts` | `tools/lib.lua` | LuaLS 3.19.1; the report as JSON; `-- EXPECT` markers |
| wrappers `tools/integration.ts` | `tools/integration.lua` | positive and negative fixtures, the natives check, normal and minified builds, the bundle run, unused modules, nine one-module maps, the gate example |
| wrappers `deno task check`, `lint`, `fmt` | none | their subject, the TypeScript, is gone |
| systems `tools/integration.lua` | the same file | unchanged; it runs `moonwell` |
| gate `gate.ts` | `gate.lua` | 11 variants, 12 probes, 5 copied runs, `all`, `--no-launch`, exit 2 for an unknown run |
| gate `preview-probe.ts` | `preview-probe.lua` | 8 + 3 maps, `2`, `--no-install`, the TGA, BLP and DDS pictures |

---

### Task 1: moonwell-wrappers

- [x] `tools/lib.lua` (from moonwell-systems, with the `MOONWELL_PKL` path), `tools/test.lua`, `tools/check.lua`,
      `tools/integration.lua`.
- [x] The four checks pass: the suites, the syntax check, integration (with `moonwell` 0.8.0 built from the
      checkout), and the same numbers as before (the behavior tests, the negative diagnostics).
- [x] Delete `tools/*.ts`, `deno.json`, `deno.lock`. `CONTRIBUTING.md`, `AGENTS.md`, `README.md` and the changelog
      name the new commands.
- [x] Commit: `Tools in Lua: no Deno`.

### Task 2: moonwell-systems

- [x] `tools/integration.lua` runs `moonwell` (or `MOONWELL`), and `init --link` in the Moonwell checkout.
- [x] Its suites, syntax check and integration pass. Its documents follow.
- [x] Commit: `Integration runs the moonwell program`.

### Task 3: wrappers-gate

- [x] `gate.lua`: every run builds (`yue -e gate.lua all`, and each probe and copied run with `--no-launch`).
- [x] `preview-probe.lua`: both sets of maps build with `--no-install`; their files equal those of the maps the Deno
      tool built, where those are still on disk.
- [x] Delete `gate.ts`, `preview-probe.ts`, `deno.json`, `deno.lock`; re-resolve `PklProject.deps.json`; `GATE.md`
      and the `PROBE-*.md` files name `yue -e gate.lua <run>`.

### Task 4: Close

- [x] Moonwell's `AGENTS.md`, changelog and roadmap say Plan 5f is done; both libraries are pushed and their CI, where
      they have one, is read.
- [x] Commit: `docs: Plan 5f of the Go toolchain is implemented`.
