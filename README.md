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

Then create `moonwell.local.pkl` to point at your game and play:

```pkl
amends "moonwell.pkl"
launch { gameExecutable = "C:\\Program Files (x86)\\Warcraft III\\_retail_\\x86_64\\Warcraft III.exe" }
```

```powershell
deno task test
```

## A project

| Path                 | What                                                                                      |
| -------------------- | ----------------------------------------------------------------------------------------- |
| `moonwell.pkl`       | Project manifest (`amends "@moonwell/Project.pkl"`); commented examples show every option |
| `moonwell.local.pkl` | Your machine's overrides, git-ignored                                                     |
| `src/main.yue`       | Gameplay entry                                                                            |
| `maps/map.w3x/`      | World Editor map (folder format, Lua script mode)                                         |
| `dist/`              | Build output                                                                              |

Gameplay registers hooks with the `moonwell` module:

```yue
import "moonwell" as mw

mw.on_main ->
  print "Hello from YueScript"
```

Hooks: `before_config`, `on_config`, `before_main`, `on_main`. A failing hook prints its error with the `.yue` file and
line, and the other hooks still run. Module top-level code runs while the map script loads, so create game objects
inside hooks.

## Commands

| Command                                          | What                                   |
| ------------------------------------------------ | -------------------------------------- |
| `deno task build [--entry src/x.yue] [--minify]` | Build `dist/bin/<map>.w3x`             |
| `deno task test [--entry src/x.yue]`             | Stage the map and launch Warcraft III  |
| `deno task dev`                                  | Re-check on every save                 |
| `deno task check`                                | Compile and validate without building  |
| `deno task setup`                                | Download the pinned YueScript compiler |

The compiler is downloaded once per version into `%LOCALAPPDATA%\moonwell` (or `~/.cache/moonwell`) and verified by
checksum. Set `yue { path = "..." }` in `moonwell.local.pkl` to use your own build.

## Credits

The template map derives from TriggerHappy's [wc3-ts-template](https://github.com/cipherxof/wc3-ts-template) via
wc3-dev-framework. MIT licensed; see [LICENSE](LICENSE).
