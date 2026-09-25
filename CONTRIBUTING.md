# Contributing to Moonwell

## Layout

- `cli/`: the `@moonwell/cli` JSR package (Deno, `jsr:@std/*` only).
- `pkl/`: the `moonwell` Pkl package.
- `template/`: the project `init` scaffolds. It links to `../cli` and `../pkl`, so use it to try changes.
- `tools/gen.ts`: regenerates `cli/src/embedded/` from `cli/runtime/` and `template/`. Run it after changing either; a
  unit test fails when the embedded copies are stale.

## Checks

```powershell
deno task check        # type-check
deno task lint
deno task test         # unit tests, no external tools
deno task test:runtime # needs yue (downloaded automatically)
deno task test:pkl     # needs pkl
deno task test:e2e     # needs pkl and yue
```

`.github/workflows/ci.yml` runs all of these (plus `deno fmt --check`) on Ubuntu and Windows.

Never add `package.json`, `node_modules` or `npm:` imports.

Versions must agree. `cli/deno.json` `version`, `cli/src/version.ts` and `pkl/PklProject` `package.version` always carry
the same number, and a unit test enforces it.

## Release gate (manual, before every release)

1. Run every check above from a clean checkout.
2. `cd template`, create `moonwell.local.pkl` with your `gameExecutable`, and run `deno task test`. Confirm "Moonwell is
   running." prints and the footman changes colour every second. Confirm the Warcraft III window is visible and stays
   open after the CLI exits.
3. Add `error "gate"` inside the `on_main` hook, run `deno task test` again, and confirm the on-screen error names
   `src/main.yue` and the right line. Record which chunk-name form the game used.
4. Run `deno task build --minify` and play `dist/bin/map.w3x` directly.
5. Open the packed map in World Editor and confirm it loads.
6. Record the Warcraft III and World Editor versions in the changelog.

## Publishing

1. Bump the version in `cli/deno.json`, `cli/src/version.ts` and `pkl/PklProject`.
2. Re-resolve the template's Pkl dependencies (`cd template && pkl project resolve`), then run `deno task gen` so the
   embedded template carries the new `PklProject.deps.json`. Commit both.
3. `pkl project package pkl/` produces the package zip and metadata. Create a GitHub release tagged `moonwell@<version>`
   in `mdlsvensson/moonwell` and attach both files.
4. `cd cli && deno publish`.
