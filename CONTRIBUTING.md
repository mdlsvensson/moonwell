# Contributing to Moonwell

## Layout

- `cli/`: the `@moonwell/cli` JSR package (Deno, `jsr:@std/*` only).
- `schema/`: the `moonwell` Pkl package. (Not named `pkl/`: on Windows a `pkl` folder in the working directory shadows
  the `pkl` executable for tools that launch it from the repo root.)
- `template/`: the project `init` scaffolds. It links to `../cli` and `../schema`, so use it to try changes.
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

Versions must agree. `cli/deno.json` `version`, `cli/src/version.ts` and `schema/PklProject` `package.version` always
carry the same number, and a unit test enforces it.

## Release gate (manual, before every release)

1. Run every check above from a clean checkout.
2. `cd template`, run `deno task setup` (it creates `moonwell.local.pkl` if missing; check its `gameExecutable`), then
   `deno task test`. Confirm "Moonwell is running." prints and the footman north of the heroes changes colour every
   second (with ally colour mode off: Alt+A toggles it, and while it is on every unit shows blue, teal or red). Confirm
   the Warcraft III window is visible and stays open after the CLI exits.
3. Add `error "gate"` inside the `on_main` hook, run `deno task test` again, and confirm the on-screen error names
   `src/main.yue` and the right line. Record which chunk-name form the game used.
4. Run `deno task build --minify` and play `dist/bin/map.w3x` directly.
5. Open the packed map in World Editor and confirm it loads.
6. Assets, in a throwaway project so `template/` stays clean (a stray file there fails the embedded-template test):
   `deno run -A cli/src/main.ts init --link <temp dir>/assets-check`, then in that project put a `.blp` icon at
   `assets/ReplaceableTextures/CommandButtons/BTNMoonwell.blp` and run `deno task test`; the map must load. Close World
   Editor, run `deno task assets:sync`, open `maps/map.w3x` and confirm the Import Manager lists
   `ReplaceableTextures\CommandButtons\BTNMoonwell.blp`. Delete the icon, sync again and confirm it is gone.
7. Record the Warcraft III and World Editor versions in the changelog.

## Publishing

Publish the Pkl package before the CLI: a new project's `init` resolves `moonwell@<version>` from the GitHub release.

1. Set the version, unless it is already the one being released (as with the first release, 0.1.0). Bump it in
   `cli/deno.json`, `cli/src/version.ts` and `schema/PklProject`, re-resolve the template's Pkl dependencies
   (`cd template && pkl project resolve`), and run `deno task gen` so the embedded template carries the new
   `PklProject.deps.json`. Commit and push.
2. `pkl project package schema/` writes `.out/moonwell@<version>/`. It ends by asking pkg.pkl-lang.org whether the
   version is already published; if that check crashes (seen in sandboxed shells: "Unable to establish loopback
   connection"), add `--skip-publish-check`, which is safe for a version that was never released.
3. Create a GitHub release in `mdlsvensson/moonwell` with a new tag `moonwell@<version>` on the pushed commit. Attach
   `moonwell@<version>.zip` and the metadata file `moonwell@<version>` (no extension). `package://pkg.pkl-lang.org/...`
   URIs redirect to these release assets.
4. `cd cli && deno publish`. It opens the browser to authorize with JSR. The first time, create the `@moonwell` scope
   and its `cli` package on jsr.io.
5. Check the published release from outside the repo:
   `deno run -A --min-dep-age=0 jsr:@moonwell/cli@<version> init my-map`, then in `my-map`
   `deno run -A --min-dep-age=0 jsr:@moonwell/cli@<version> build`. Deno refuses versions published less than 24 hours
   ago unless `--min-dep-age=0` is passed, so the project's own `deno task build` only works after that.
