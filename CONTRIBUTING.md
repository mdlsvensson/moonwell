# Contributing to Moonwell

## Layout

- `cli/`: the `@moonwell/cli` JSR package (Deno, `jsr:@std/*` only).
- `schema/`: the `moonwell` Pkl package. (Not named `pkl/`: on Windows a `pkl` folder in the working directory shadows
  the `pkl` executable for tools that launch it from the repo root.)
- `template/`: the project `init` scaffolds. It links to `../cli` and `../schema`, so use it to try changes.
  `init --link <dir>` creates such a linked project elsewhere; on Windows it must be on the same drive as the checkout,
  because Pkl cannot load a local dependency from another drive.
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

## In-game path list

`assets:paths` knows which paths the game ships from `cli/data/game-paths.txt`. To regenerate it after a game patch,
export the file names of the game's CASC storage (for example with CascView) to a text file, one per line, then run
`deno task gen:game-paths <that file> <game version>` and `deno task gen`, and commit both files.

## Natives

The editor declarations come from `cli/data/natives.json`: the names, types and signatures of `common.j` and
`blizzard.j`, plus the Lua globals the game provides and removes from `tools/natives/lua-extras.json` (hand-written,
from an in-game probe). For a new game version, export `war3.w3mod/scripts/common.j` and `war3.w3mod/scripts/blizzard.j`
from the game's CASC storage with CascView, keeping those relative paths, then run
`deno task gen:natives <export folder> <game version>` and `deno task gen`, and commit `cli/data/natives.json` and
`cli/src/embedded/natives.ts`. Comments in the `.j` files are Blizzard's text and are not copied.

## Release gate (manual, before every release)

1. Run every check above from a clean checkout.
2. Confirm `cli/data/game-paths.txt` starts with `# Warcraft III <version>`, not the "Not generated yet" placeholder:
   with the placeholder every in-game path is reported as `custom path, not imported`.
3. `cd template`, run `deno task setup` (it creates `moonwell.local.pkl` if missing; check its `gameExecutable`), then
   `deno task test`. Confirm "Moonwell is running." prints and the Captain north of the heroes changes colour every
   second (with ally colour mode off: Alt+A toggles it, and while it is on every unit shows blue, teal or red). Confirm
   the Warcraft III window is visible and stays open after the CLI exits.
4. Add `error "gate"` inside the `on_main` hook, run `deno task test` again, and confirm the on-screen error names
   `src/main.yue` and the right line. Record which chunk-name form the game used.
5. Run `deno task build --minify` and play `dist/bin/map.w3x` directly.
6. Open the packed map in World Editor and confirm it loads.
7. Assets, in a throwaway project so `template/` stays clean (a stray file there fails the embedded-template test):
   `deno run -A cli/src/main.ts init --link <temp dir>/assets-check`, then in that project put a `.blp` icon at
   `assets/ReplaceableTextures/CommandButtons/BTNMoonwell.blp` and run `deno task test`; the map must load. Close World
   Editor, run `deno task assets:sync`, open `maps/map.w3x` and confirm the Import Manager lists
   `ReplaceableTextures\CommandButtons\BTNMoonwell.blp`. Save the map in World Editor and close it, then confirm
   `deno task assets:check` reports no changes and `deno task build` succeeds (World Editor 3.00 saves the import with
   flag 29). Delete the icon, sync again and confirm it is gone.
8. Map settings, in another throwaway project from `init --link`. In its `moonwell.pkl`, set `info.name` and
   `loadingScreen.title`; a `players` entry for a slot the map has (such as `["0"]` with a `name`, `race` and
   `fixedStart`); `environment.soundEnvironment`, `environment.waterColor` and fog (`enabled = true`, `start`, `end`,
   `color`); and `gameplay.heroMaxLevel` and `gameplay.foodLimit`. For team settings, first enable custom forces in
   World Editor (Scenario > Force Properties), save the map, and set `forces["0"]`, such as `name`, `allied` and
   `sharedVision`. Run `deno task settings:check` and confirm it lists `war3map.w3i`, `war3map.lua` and
   `war3mapMisc.txt`. Run `deno task test`, then `deno task build --minify` and play `dist/bin/map.w3x` from the game's
   Maps folder. Confirm the lobby shows the map name, the slot and the team, and in the game the fog, water colour and
   ambient sound, the food ceiling, and that a hero cannot level past the set maximum. Open the packed map in World
   Editor and confirm Map Description, Loading Screen, Player Properties, Force Properties, Map Options (fog, water) and
   Gameplay Constants show the configured values. Confirm `maps/map.w3x` is unchanged (`git status`).
9. Object data, in another throwaway project: `deno run -A cli/src/main.ts init --link <temp dir>/objects-check`. It
   must be linked: a packaged project resolves `moonwell@0.2.0`, which has no `Objects.pkl`. Commit it to a new git
   repository (`git init`) so sub-step 5 can use `git status`. This proves what the tests cannot: that the game and
   World Editor read the files Moonwell writes, including per-level values past level 1.
   1. Run `deno task test`. Confirm the unit north of the heroes is the Captain (its model, name and icon) and still
      changes colour.
   2. Add `objects/gate.pkl` with: a hero based on the Paladin (`Hpal`) with a custom `name` and `startingStrength`,
      whose `hero` abilities are a custom ability based on Holy Light (`AHhb`) with `levels = 4`,
      `cooldown = List(1, 2, 3, 4)` and `properties { ["amountHealedOrDamaged"] = List(111, 222, 333, 444) }` (set
      `heroSkin` to the same list, as the game data does for every hero); a custom buff for that ability's `buffs`, with
      a new `icon`; a custom item with a new `name`, `goldCost` and `interfaceIcon`; and a custom building based on the
      Blacksmith (`hbla`) whose `researchesAvailable` is a custom upgrade with `levels = 2` and per-level names and
      tooltips (`name = List("...", "...")`, `tooltip = List("...", "...")`). Run `deno task objects:check`: it lists
      the ten files and reports `src/generated/objects.yue` stale until the next build. In `main.yue`, create the hero,
      the building and the item for player 0 (from `objects.heroes`, `objects.buildings` and `objects.items`), raise the
      hero to level 7 (`SetHeroLevel hero, 7, false`; a hero ability's level 4 needs hero level 7), and give player 0
      gold and lumber for the research (`SetPlayerState Player(0), PLAYER_STATE_RESOURCE_GOLD, 5000`, and the same for
      lumber).
   3. Run `deno task test`. The hero shows its name and strength. Learning the ability shows its level 1 to 4 tooltips,
      and healing a wounded unit heals 111, 222, 333 and 444 at the four levels, with the four cooldowns. The item shows
      its name and icon. The building offers only the custom upgrade; its button shows the level 1 tooltip, and after
      researching level 1 the level 2 tooltip (the research button shows the tooltip, not the name; level 2, like the
      Blacksmith's, needs a Keep, so it shows greyed out).
   4. In World Editor, change the standard Footman's hit points in the project's `maps/map.w3x` and save. Run
      `deno task build --minify`, play `dist/bin/map.w3x`, and confirm the Footman change survived next to the Moonwell
      objects.
   5. Open the packed map in World Editor. The Object Editor lists every custom object under Custom with the configured
      values: the per-level heal amounts and upgrade names, the item's price, the skin fields (models, icons), and the
      buff's icon (Holy Light applies no buff in game, so the buff is only checked here). Confirm `maps/map.w3x` is
      unchanged apart from sub-step 4's edit (`git status`), and that a second build leaves `src/generated/objects.yue`
      unchanged.
10. Editor, in another throwaway project: `deno run -A cli/src/main.ts init --link <temp dir>/editor-check`, committed
    to a new git repository (`git init`). Run `deno task setup` there and, if it prints a PATH command, run it once in
    PowerShell, then open a new terminal. Open the project folder itself in VS Code (File > Open Folder; opened any
    other way, lua-language-server finds no `.luarc.json`) and install the YueScript and Lua extensions it recommends.
    In `src/main.yue`, confirm that hovering or completing `CreateUnit` shows its parameter names and types, and that
    `mw.on_main` and `objects.units.captain` complete. In World Editor, place a unit in `maps/map.w3x`, reference it in
    a trigger (World Editor writes a `gg_unit_...` global only for a unit a trigger uses) and save; after
    `deno task check`, confirm typing `gg_unit_` offers every placed unit a trigger uses, including the template's
    `gg_unit_Hblm_0003` and `gg_unit_Hpal_0002`. Create `src/heroes/captain.yue` with
    `export default { greet: -> print "Hello" }`, add `import "heroes.captain"` to `src/main.yue`, save both, and
    confirm `captain.greet` completes after the import (lua-language-server indexes the git-ignored `.lua` files). Type
    `CreatUnit` for `CreateUnit` in `src/main.yue`: the editor underlines it, and `deno task check` fails with
    `src/main.yue:<line>:<column> › Unknown global CreatUnit.` and `Did you mean CreateUnit?`. Set
    `lint { unknownGlobals = "warning" }` in `moonwell.pkl` and confirm `deno task build` succeeds and prints the same
    lines as warnings; then undo both changes. Confirm `git status` shows no `.moonwell/` and no `src/**/*.lua` files.
11. Record the Warcraft III and World Editor versions in the changelog.

## Publishing

Publish the Pkl package before the CLI: a new project's `init` resolves `moonwell@<version>` from the GitHub release.

1. Set the version, unless it is already the one being released (as with the first release, 0.1.0). Bump it in
   `cli/deno.json`, `cli/src/version.ts` and `schema/PklProject`, re-resolve the template's Pkl dependencies
   (`cd template && pkl project resolve`), and run `deno task gen` so the embedded template carries the new
   `PklProject.deps.json`. Commit and push.
2. `pkl project package schema/` writes `.out/moonwell@<version>/`. It ends by asking pkg.pkl-lang.org whether the
   version is already published; if that check crashes (seen in sandboxed shells: "Unable to establish loopback
   connection"), add `--skip-publish-check`, which is safe for a version that was never released.
3. Push the release commit first: `--target main` tags whatever `main` is on GitHub, and 0.3.1 was first tagged on the
   commit before its version bump. Create a GitHub release in `mdlsvensson/moonwell` with a new tag `moonwell@<version>`
   on the pushed commit, and check that the tag names the release commit. Attach `moonwell@<version>.zip` and the
   metadata file `moonwell@<version>` (no extension). `package://pkg.pkl-lang.org/...` URIs redirect to these release
   assets.
4. `cd cli && deno publish`. It opens the browser to authorize with JSR. The first time, create the `@moonwell` scope
   and its `cli` package on jsr.io.
5. Check the published release from outside the repo:
   `deno run -A --min-dep-age=0 jsr:@moonwell/cli@<version> init my-map`, then in `my-map`
   `deno run -A --min-dep-age=0 jsr:@moonwell/cli@<version> build`. Deno refuses versions published less than 24 hours
   ago unless `--min-dep-age=0` is passed, so the project's own `deno task build` only works after that.
