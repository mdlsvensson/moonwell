# Moonwell

**Build Warcraft III maps with YueScript gameplay and Pkl project data.**

- Gameplay in [YueScript](https://github.com/IppClub/YueScript), compiled to Lua 5.3 and bundled into your map. Only the
  modules you import are included, and runtime errors point at your `.yue` lines (in `--minify` builds, at the `.yue`
  file only).
- Project configuration in [Pkl](https://pkl-lang.org), validated against the versioned `moonwell` schema package.
- A Deno command-line tool: no Node.js, no `package.json`, no `node_modules`.

## Quickstart

Install [Deno](https://deno.com/) 2.9+ and [Pkl](https://pkl-lang.org) 0.32+.

```powershell
deno run -A jsr:@moonwell/cli init my-map
cd my-map
deno task build
```

`init` also writes `moonwell.local.pkl`, which points `launch.gameExecutable` at the default Battle.net install. If your
game is elsewhere, fix the path there, then play:

```powershell
deno task test
```

## A project

| Path                 | What                                                                                                   |
| -------------------- | ------------------------------------------------------------------------------------------------------ |
| `moonwell.pkl`       | Project manifest (`amends "@moonwell/Project.pkl"`), shared by the team; everyday settings written out |
| `moonwell.local.pkl` | This machine's settings, such as the game path; git-ignored, and `deno task setup` recreates it        |
| `src/main.yue`       | Gameplay entry                                                                                         |
| `maps/map.w3x/`      | World Editor map (folder format, Lua script mode)                                                      |
| `assets/`            | Files to import into the map                                                                           |
| `.asset-state/`      | Which source-map files `assets:sync` owns; commit it                                                   |
| `dist/`              | Build output                                                                                           |

`moonwell.local.pkl` amends `moonwell.pkl`, so any setting can be overridden there for your machine only. Lists such as
`launch.args` are replaced, not extended: `args = List("-launch", "-windowmode", "fullscreen")`.

Gameplay registers hooks with the `moonwell` module:

```yue
import "moonwell" as mw

mw.on_main ->
  print "Hello from YueScript"
```

Hooks: `before_config`, `on_config`, `before_main`, `on_main`. A failing hook prints its error with the `.yue` file and
line, and the other hooks still run. Module top-level code runs while the map script loads, so create game objects
inside hooks.

## Assets

Every file under `assets/` is imported into the built map at its relative path: `assets/Models/unit.mdx` becomes
`Models\unit.mdx`. Names starting with `.` are skipped. The `assets` block in `moonwell.pkl` maps files to exact in-map
paths and leaves files out:

```pkl
assets {
  paths { ["icons/BTNSword.blp"] = #"ReplaceableTextures\CommandButtons\BTNSword.blp"# }
  exclude = List("credits/")
}
```

Builds import assets into the staged copy only. To see them in World Editor, close the map there and run
`deno task assets:sync`. It writes the files and `war3map.imp` into `maps/<folder>`, and records what it owns in
`.asset-state/`. It never overwrites or deletes a file it does not own, and it refuses to touch an owned file you edited
in the map.

To check that a model's textures are imported, run `deno task assets:paths assets/Models/Knight.mdx`. It lists every
file the model references (textures, particle models, attachments) and whether a build imports it. A path that is not
found is either a built-in game file or a missing import. Run it without a file to check every model under `assets/`, or
on a model outside a project to see what it needs before importing it.

## Icons

`init` creates World Editor's icon folders under `assets/`. Icons named the game's way import at the paths the object
editor expects:

| Folder                                        | File name                                 | For                                  |
| --------------------------------------------- | ----------------------------------------- | ------------------------------------ |
| `ReplaceableTextures/CommandButtons/`         | `BTN<Name>.blp`                           | Abilities, units, items and upgrades |
| `ReplaceableTextures/CommandButtonsDisabled/` | `DISBTN<Name>.blp`, `DISPASBTN<Name>.blp` | Greyed-out versions                  |
| `ReplaceableTextures/PassiveButtons/`         | `PASBTN<Name>.blp`                        | Passive abilities                    |

Give every `BTN<Name>` a matching `DISBTN<Name>`, and every `PASBTN<Name>` a matching `DISPASBTN<Name>`: the game shows
a placeholder where a disabled icon is missing.

## Commands

| Command                                          | What                                                                             |
| ------------------------------------------------ | -------------------------------------------------------------------------------- |
| `deno task build [--entry src/x.yue] [--minify]` | Build `dist/bin/<map>.w3x`                                                       |
| `deno task test [--entry src/x.yue]`             | Stage the map and launch Warcraft III                                            |
| `deno task dev`                                  | Re-check on every save                                                           |
| `deno task check`                                | Compile and validate without building                                            |
| `deno task assets:check`                         | Show what `assets:sync` would change in the source map                           |
| `deno task assets:sync`                          | Write `assets/` into the source map for World Editor (close the map first)       |
| `deno task assets:paths [file]`                  | List the files a model references, and whether a build imports them              |
| `deno task setup`                                | Create a missing `moonwell.local.pkl` and download the pinned YueScript compiler |

The compiler is downloaded once per version and verified by checksum. It is cached in `MOONWELL_CACHE` when that is set,
else in `%LOCALAPPDATA%\moonwell` on Windows, else in `$XDG_CACHE_HOME/moonwell` or `~/.cache/moonwell`.

## Advanced settings

These are not in the generated files and keep their defaults unless you add them. The schema, `Project.pkl` in the
`moonwell` Pkl package, documents every setting.

| Setting       | Default  | What                                                                                        |
| ------------- | -------- | ------------------------------------------------------------------------------------------- |
| `yue.version` | `0.34.2` | YueScript compiler version. Only versions this CLI release pins a checksum for are accepted |
| `yue.path`    | none     | Your own `yue` binary instead of the downloaded one. Set it in `moonwell.local.pkl`         |

```pkl
yue {
  path = "C:\\tools\\yue.exe"
}
```

## Editor support

Nothing needs an editor plugin: the `pkl` and `deno` command-line tools do all the work. Editors with Pkl support (the
Pkl extension for VS Code, the IntelliJ plugin) add completion and hover docs for `moonwell.pkl`. They find the schema
through `PklProject`, so run their "sync projects" command once after `init`. If the extension cannot find `pkl`, set
its CLI path (`pkl.cli.path` in VS Code).

## Credits

The template map derives from TriggerHappy's [wc3-ts-template](https://github.com/cipherxof/wc3-ts-template) via
wc3-dev-framework. MIT licensed; see [LICENSE](LICENSE).
