# Changelog

## 0.1.0 (unreleased)

First release: the toolchain.

- `moonwell init [--link]` scaffolds a map project: `deno.json` tasks, `PklProject`, a `moonwell.pkl` manifest with the
  everyday settings written out, and a git-ignored `moonwell.local.pkl` for machine settings.
- `build`, `test`, `dev`, `check` and `setup` commands. `setup` installs the pinned, checksum-verified YueScript
  compiler and recreates a missing `moonwell.local.pkl`.
- A YueScript module bundler with a Lua runtime: `before_config`, `on_config`, `before_main` and `on_main` hooks, and
  runtime errors reported at `.yue` file and line.
- An MPQ archive writer that packs the map into `<build.folder>/<map>.w3x`.
- The `moonwell` Pkl package: the project manifest schema.

### Release gate

Passed 2026-09-25 on Warcraft III Reforged 3.0.0.24268 and World Editor 3.00:

- `deno task test` launches the staged map. "Moonwell is running." prints and the footman changes colour every second
  (ally colour mode off).
- An `error` in `on_main` is reported on screen as `src/main.yue:<line>`. The exact chunk-name form was not recorded,
  but the runtime rewrote it correctly.
- The `--minify` build plays from the game's own Maps folder as a regular custom game.
- The packed map opens and test-runs in World Editor.
