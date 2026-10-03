# Contributing to Moonwell

## Layout

One Go module at the root, with no dependencies beyond the standard library.

- `cmd/moonwell/`: the program. It reads the working directory and the signals, and calls `internal/cli`.
- `internal/`: one package per area (`cli`, `pipeline`, `project`, `settings`, `objects`, `assets`, `library`, `yue`,
  `bundle`, `lint`, `editor`, `mpq`, `w3i`, `models`, ...), each with its tests beside it. `internal/testkit` holds what
  tests share. Packages depend downward only: `cli` on `pipeline` and the areas, the areas on the shared packages.
- `embed.go`, `version.go`: package `moonwell`, the files the program carries (`template/`, `runtime/`, `data/`) and its
  version.
- `runtime/`: `moonwell.lua`, bundled into every map, and `macros.yue`.
- `data/`: the game data the program carries: the in-game path list, the object metadata and the natives.
- `schema/`: the `moonwell` Pkl package. (Not named `pkl/`: on Windows a `pkl` folder in the working directory shadows
  the `pkl` executable for tools that launch it from the repo root.)
- `template/`: the project `init` scaffolds. It is a linked project (its `PklProject` imports `../schema`), so use it to
  try changes. Nothing stray may be left in it: every file there goes into every new project, and a test pins the list.
- `tools/gen/`: the generators (below).
- `install.ps1`, `install.sh`: the install scripts a release serves.

## Working on Moonwell

- `go run ./cmd/moonwell <command>` runs the program from the checkout; in `template/`, `go run ../cmd/moonwell build`.
- `moonwell init --link <dir>`, run in the checkout or below it, creates a project that imports the checkout's
  `schema/`. On Windows the folder must be on the same drive as the checkout: Pkl cannot load a local dependency from
  another drive. A linked project is built with whatever `moonwell` is run in it.
- `go build -o <a folder on your PATH> ./cmd/moonwell` gives you the checkout's program as `moonwell`.

## Checks

```
go vet ./...
gofmt -l .        # must print nothing
go test ./...
```

`go test` runs everything. A test that needs `pkl` or the YueScript compiler is skipped when it is missing (the compiler
is downloaded once into the user's cache; `pkl` must be on the PATH, since tests do not use the Pkl that `moonwell`
downloads for itself); with `MOONWELL_REQUIRE_TOOLS=1` it fails instead. `MOONWELL_NETWORK_TESTS=1` adds the tests
that download the example library and the pinned Pkl, and `MOONWELL_TEST_YUE` names a compiler to use.
`.github/workflows/ci.yml` runs the three commands on Ubuntu and Windows with both variables set.

## Rules

- **Standard library only.** No third-party module and no cgo: `go.mod` has no `require`, and a test keeps it so.
- **Errors.** An expected failure is a `*diag.Error` (or `diag.Problems`) with the file and a hint. Any other error, and
  any panic, is reported as an internal "please report" error, so a user's mistake must never reach it.
- **Style.** `gofmt` and `go vet` are clean. Markdown is wrapped at 120 by hand.
- **Versions must agree.** `version.go`, `schema/PklProject`, both install scripts and the README's examples carry the
  same number, and tests check it.
- **Generated files.** After changing `data/metadata.json`, run `go run ./tools/gen`; a test fails when
  `schema/generated/` is stale. `template/`, `runtime/` and `data/` need no step: they are embedded when the program is
  built.

## The generators

`tools/gen` writes the generated files. Run it in the checkout:

| Command                                              | Writes                   | From                                         |
| ---------------------------------------------------- | ------------------------ | -------------------------------------------- |
| `go run ./tools/gen`                                 | `schema/generated/*.pkl` | `data/metadata.json`                         |
| `go run ./tools/gen game-paths <listfile> <version>` | `data/game-paths.txt`    | a list of the game's file names              |
| `go run ./tools/gen natives <folder> <version>`      | `data/natives.json`      | `common.j`, `blizzard.j`, `lua-extras.json`  |
| `go run ./tools/gen metadata <folder> <version>`     | `data/metadata.json`     | the game's SLK and text files, the overrides |

`<version>` is the game's, such as `3.0.0.24268`. The folders are exports made with CascView, which keeps the game's
relative paths (`war3.w3mod/...`). Nothing of the game's files is committed except the names, types and signatures the
data files hold.

- **In-game path list.** `assets:paths` knows which paths the game ships from `data/game-paths.txt`. After a game
  patch, export the file names of the game's CASC storage to a text file, one per line, and run the `game-paths` mode.
- **Natives.** The editor declarations and the unknown-global check come from `data/natives.json`: the names, types and
  signatures of `common.j` and `blizzard.j`, plus the Lua globals the game provides and removes, from
  `tools/natives/lua-extras.json` (hand-written, from an in-game probe). For a new game version, export
  `war3.w3mod/scripts/common.j` and `war3.w3mod/scripts/blizzard.j` and run the `natives` mode. Comments in the `.j`
  files are Blizzard's text and are not copied.
- **Object metadata.** `data/metadata.json` holds every object field (its rawcode, friendly name, type and where it
  applies) and every standard object. Export `war3.w3mod/units/` and `war3.w3mod/_locales/enus.w3mod/` and run the
  `metadata` mode, then `go run ./tools/gen` for the schema. A friendly name that is already released never changes
  silently: the generator fails, and `tools/metadata/overrides.json` pins the name or acknowledges a removed field.

## Release gate (manual, before every release)

1. Run every check above from a clean checkout. Then build the checkout's program into the folder the install script
   uses, so that the steps below run it as `moonwell`:
   `go build -o "$env:LOCALAPPDATA\moonwell\bin\moonwell.exe" ./cmd/moonwell` on Windows,
   `go build -o ~/.local/bin/moonwell ./cmd/moonwell` on Linux. `moonwell --version` must print the version being
   released.
2. Confirm `data/game-paths.txt` starts with `# Warcraft III <version>`, not the "Not generated yet" placeholder:
   with the placeholder every in-game path is reported as `custom path, not imported`.
3. `cd template`, run `moonwell setup` (it creates `moonwell.local.pkl` if missing; check its `gameExecutable`), then
   `moonwell test`. Confirm "Moonwell is running." prints and the Captain north of the heroes changes colour every
   second (with ally colour mode off: Alt+A toggles it, and while it is on every unit shows blue, teal or red). Confirm
   the Warcraft III window is visible and stays open after `moonwell` exits.
4. Add `error "gate"` inside the `on_main` hook, run `moonwell test` again, and confirm the on-screen error names
   `src/main.yue` and the right line. Record which chunk-name form the game used.
5. Run `moonwell build --minify` and play `dist/bin/map.w3x` directly.
6. Open the packed map in World Editor and confirm it loads.
7. Assets, in a throwaway project so `template/` stays clean (a stray file there fails the template test):
   `moonwell init --link <temp dir>/assets-check` in the checkout, then in that project put a `.blp` icon at
   `assets/ReplaceableTextures/CommandButtons/BTNMoonwell.blp` and run `moonwell test`; the map must load. Close World
   Editor, run `moonwell assets:sync`, open `maps/map.w3x` and confirm the Import Manager lists
   `ReplaceableTextures\CommandButtons\BTNMoonwell.blp`. Save the map in World Editor and close it, then confirm
   `moonwell assets:check` reports no changes and `moonwell build` succeeds (World Editor 3.00 saves the import with
   flag 29). Delete the icon, sync again and confirm it is gone.
8. Map settings, in another throwaway project from `init --link`. In its `moonwell.pkl`, set `info.name` and
   `loadingScreen.title`; a `players` entry for a slot the map has (such as `["0"]` with a `name`, `race` and
   `fixedStart`); `environment.soundEnvironment`, `environment.waterColor` and fog (`enabled = true`, `start`, `end`,
   `color`); and `gameplay.heroMaxLevel` and `gameplay.foodLimit`. For team settings, first enable custom forces in
   World Editor (Scenario > Force Properties), save the map, and set `forces["0"]`, such as `name`, `allied` and
   `sharedVision`. Run `moonwell settings:check` and confirm it lists `war3map.w3i`, `war3map.lua` and
   `war3mapMisc.txt`. Run `moonwell test`, then `moonwell build --minify` and play `dist/bin/map.w3x` from the game's
   Maps folder. Confirm the lobby shows the map name, the slot and the team, and in the game the fog, water colour and
   ambient sound, the food ceiling, and that a hero cannot level past the set maximum. Open the packed map in World
   Editor and confirm Map Description, Loading Screen, Player Properties, Force Properties, Map Options (fog, water) and
   Gameplay Constants show the configured values. Confirm `maps/map.w3x` is unchanged (`git status`).
9. Object data, in another throwaway project: `moonwell init --link <temp dir>/objects-check` in the checkout. Commit
   it to a new git repository (`git init`) so sub-step 5 can use `git status`. This proves what the tests cannot: that
   the game and World Editor read the files Moonwell writes, including per-level values past level 1.
   1. Run `moonwell test`. Confirm the unit north of the heroes is the Captain (its model, name and icon) and still
      changes colour.
   2. Add `objects/gate.pkl` with: a hero based on the Paladin (`Hpal`) with a custom `name` and `startingStrength`,
      whose `hero` abilities are a custom ability based on Holy Light (`AHhb`) with `levels = 4`,
      `cooldown = List(1, 2, 3, 4)` and `properties { ["amountHealedOrDamaged"] = List(111, 222, 333, 444) }` (set
      `heroSkin` to the same list, as the game data does for every hero); a custom buff for that ability's `buffs`, with
      a new `icon`; a custom item with a new `name`, `goldCost` and `interfaceIcon`; and a custom building based on the
      Blacksmith (`hbla`) whose `researchesAvailable` is a custom upgrade with `levels = 2` and per-level names and
      tooltips (`name = List("...", "...")`, `tooltip = List("...", "...")`). Run `moonwell objects:check`: it lists
      the ten files and reports `src/generated/objects.yue` stale until the next build. In `main.yue`, create the hero,
      the building and the item for player 0 (from `objects.heroes`, `objects.buildings` and `objects.items`), raise the
      hero to level 7 (`SetHeroLevel hero, 7, false`; a hero ability's level 4 needs hero level 7), and give player 0
      gold and lumber for the research (`SetPlayerState Player(0), PLAYER_STATE_RESOURCE_GOLD, 5000`, and the same for
      lumber).
   3. Run `moonwell test`. The hero shows its name and strength. Learning the ability shows its level 1 to 4 tooltips,
      and healing a wounded unit heals 111, 222, 333 and 444 at the four levels, with the four cooldowns. The item shows
      its name and icon. The building offers only the custom upgrade; its button shows the level 1 tooltip, and after
      researching level 1 the level 2 tooltip (the research button shows the tooltip, not the name; level 2, like the
      Blacksmith's, needs a Keep, so it shows greyed out).
   4. In World Editor, change the standard Footman's hit points in the project's `maps/map.w3x` and save. Run
      `moonwell build --minify`, play `dist/bin/map.w3x`, and confirm the Footman change survived next to the Moonwell
      objects.
   5. Open the packed map in World Editor. The Object Editor lists every custom object under Custom with the configured
      values: the per-level heal amounts and upgrade names, the item's price, the skin fields (models, icons), and the
      buff's icon (Holy Light applies no buff in game, so the buff is only checked here). Confirm `maps/map.w3x` is
      unchanged apart from sub-step 4's edit (`git status`), and that a second build leaves `src/generated/objects.yue`
      unchanged.
10. Editor, in another throwaway project: `moonwell init --link <temp dir>/editor-check` in the checkout, committed
    to a new git repository (`git init`). Run `moonwell setup` there and, if it prints a PATH command, run it once in
    PowerShell, then open a new terminal. Open the project folder itself in VS Code (File > Open Folder; opened any
    other way, lua-language-server finds no `.luarc.json`) and install the YueScript and Lua extensions it recommends.
    In `src/main.yue`, confirm that hovering or completing `CreateUnit` shows its parameter names and types, and that
    `mw.on_main` and `objects.units.captain` complete. In World Editor, place a unit in `maps/map.w3x`, reference it in
    a trigger (World Editor writes a `gg_unit_...` global only for a unit a trigger uses) and save; after
    `moonwell check`, confirm typing `gg_unit_` offers every placed unit a trigger uses, including the template's
    `gg_unit_Hblm_0003` and `gg_unit_Hpal_0002`. Create `src/heroes/captain.yue` with
    `export default { greet: -> print "Hello" }`, add `import "heroes.captain"` to `src/main.yue`, save both, and
    confirm `captain.greet` completes after the import (lua-language-server indexes the git-ignored `.lua` files). Type
    `CreatUnit` for `CreateUnit` in `src/main.yue`: the editor underlines it, and `moonwell check` fails with
    `src/main.yue:<line>:<column> › Unknown global CreatUnit.` and `Did you mean CreateUnit?`. Set
    `lint { unknownGlobals = "warning" }` in `moonwell.pkl` and confirm `moonwell build` succeeds and prints the same
    lines as warnings; then undo both changes. Confirm `git status` shows no `.moonwell/` and no `src/**/*.lua` files.
11. Macros and the game's Lua, in the step 10 project: run `moonwell test` and confirm a standard Footman stands beside
    the Captain (the template makes it with `$FourCC("hfoo")`). In the editor (the project opened as step 10 says),
    confirm `src/main.yue` shows no error on the `moonwell.macros` import or on `$FourCC("hfoo")`, and that hovering
    `$FourCC("hfoo")` is quiet (the YueScript extension finds the module through `yueconfig.yue`'s `include`). Then save
    the Lua probe below as `src/probe.yue`, run `moonwell test --entry src/probe.yue`, and confirm the game shows
    `missing: collectgarbage dofile loadfile debug io package` (the `removed` list in `tools/natives/lua-extras.json`)
    and `os: clock date difftime time` (as README's "The game's Lua" says). Printed lines show on screen for a few
    seconds only; the message log (F12) does not keep them. Delete `src/probe.yue` afterwards.

    ```yue
    import "moonwell" as mw

    names = {
      "assert", "collectgarbage", "dofile", "error", "getmetatable", "ipairs", "load", "loadfile", "next", "pairs",
      "pcall", "print", "rawequal", "rawget", "rawlen", "rawset", "require", "select", "setmetatable", "tonumber",
      "tostring", "type", "xpcall", "_VERSION", "_G", "coroutine", "debug", "io", "math", "os", "package", "string",
      "table", "utf8", "FourCC", "__jarray"
    }

    mw.on_main ->
      missing = [name for name in *names when _G[name] == nil]
      print "missing: " .. table.concat missing, " "
      for lib in *{"os", "debug", "package"}
        t = _G[lib]
        if type(t) == "table"
          keys = [key for key in pairs t]
          table.sort keys
          print lib .. ": " .. table.concat keys, " "
    ```
12. Lua modules and libraries, in another throwaway project from `init --link`: add `lua/greeter.lua` that returns a
    table with a function that prints, and a global-style `lua/counter.lua` that defines a global function
    (`function CountUp() print("counted") end`). Use both from `src/main.yue` (`import "greeter"`, `require "counter"`,
    `CountUp!`). Confirm the editor completes the module's function and the global, `moonwell check` passes, and in the
    game (`moonwell test`) both print. Then add `error "lua gate"` inside the module's function, run
    `moonwell test --minify`, and confirm the game's error names `lua/greeter.lua` and the right line (Lua modules keep
    their lines in minified builds). Remove the error afterwards.

    Then add the example library by tag in `moonwell.pkl`
    (`libraries { ["example"] { github = "mdlsvensson/moonwell-example-lib"; tag = "v0.1.0"; dir = "src" } }`), use it
    from `src/main.yue` (`import "example.loud"`, `print loud.shout "Moonwell"`), run `moonwell check` and commit
    `moonwell.lock`. Confirm the editor completes `loud.shout`, and the game prints the shout. Check with the Lua
    extension's bundled lua-language-server, in the editor or from the command line
    (`<extensions>/sumneko.lua-<version>/server/bin/lua-language-server --check=<project> --checklevel=Hint`), that the
    library's modules give no duplicate-definition diagnostics between `.moonwell/libraries/` and `.moonwell/lua/`.
    Delete `.moonwell/`, run `moonwell check` again, and confirm `moonwell.lock` is unchanged. Finally clone the
    library next to the project, point `moonwell.local.pkl` at it
    (`libraries { ["example"] { path = "../moonwell-example-lib"; dir = "src" } }`), change `hello` in its
    `src/example/greet.lua`, and confirm `moonwell test` runs the change and `moonwell.lock` is unchanged.
13. Map preview, in another throwaway project from `init --link`: put a 256×256 `.tga` or `.png` beside `moonwell.pkl`
    and set `settings.info.preview` to its name (both go into the map as the same `.tga`). Confirm
    `moonwell settings:check` lists `war3map.lua`, `war3mapMinimap.blp`, `war3mapMap.blp (removed)` and
    `war3mapMap.tga`. Run `moonwell build` and copy `dist/bin/map.w3x` into the game's `Maps` folder. Open the
    single-player custom game screen and select the map: the list must show the picture, not the minimap. Start the
    game: the minimap must show the terrain, not the picture. A picture the game cannot read closes the game when the
    map is selected, so a crash there is this step failing.
14. Record the Warcraft III and World Editor versions in the changelog.


## Publishing

A release is built by `.github/workflows/release.yml` from a tag `moonwell@<version>`. The tag's name is fixed: the Pkl
package's download address is built from it.

1. Set the version in `version.go`, `schema/PklProject`, `install.ps1`, `install.sh` and the README's two examples (a
   test names any that was missed), re-resolve the template's Pkl
   dependencies (`cd template && pkl project resolve`), and write the changelog's section, headed
   `## <version> (<date>)`: the release notes are its entries up to the first `###` heading. Commit, push, and wait for
   CI.
2. Run the release gate above.
3. Tag the pushed commit and push the tag: `git tag moonwell@<version>`, `git push origin moonwell@<version>`. The
   workflow fails at once unless the tag, `version.go` and `schema/PklProject` agree. It then runs the checks, builds
   `moonwell-windows-amd64.exe` and `moonwell-linux-amd64` with their `checksums.txt`, packages the schema
   (`moonwell@<version>.zip` and the metadata file `moonwell@<version>`, which `package://pkg.pkl-lang.org/...` URIs
   redirect to), and creates the GitHub release with those files and the two install scripts.
4. Read the workflow's result (`gh run watch`). Its last job is the check from outside the repository: on Ubuntu and
   Windows it runs the install line of the new version, then `moonwell init my-map` and `moonwell build`.
5. On your own machine, run the install line once and build a project. A release that fails after the tag is pushed is
   fixed with a new version, not by moving the tag: projects and the Pkl package index remember what a tag held.

A Moonwell release is a full release, never a pre-release. The README's install line downloads from
`releases/latest/download/`, and GitHub's "latest" skips pre-releases: marking the newest release as a pre-release
makes that line answer 404, which Windows PowerShell reports as "The connection was closed unexpectedly". The workflow
creates the release as the latest one, and its install job runs the README's line, so it fails if that ever breaks.
