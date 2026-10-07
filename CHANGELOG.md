# Changelog

## Unreleased

- **A library with only `github`, or only `tag`, is now refused by Pkl.** `libraries` in `moonwell.pkl` takes a
  `github` and a `tag`, or a `path`. Moonwell refused anything else before, in its own words; the schema now says
  so itself, so the editor marks the line and Pkl's message names it. A manifest that built before still builds.
- A global function of a Lua module is found also when a string that says `local` stands before it
  (`kind = "local"`, then `function Init() end`). A use of such a function was reported as an unknown global.
- A "did you mean" hint counts two neighbouring letters that changed places as one mistake: `biuld` names `build`,
  `--hepl` names `--help` and `tset` names `test`. This holds for every such hint: a command, a flag, a global,
  and a field or a base object of an object.
- An error about the asset ownership state names `.asset-state/<map>.json` from the project folder, as every other
  file is named, and not by its full path. A link on the way to a file Moonwell keeps below `.moonwell/` or
  `dist/stage/lua/` is refused with that file named. A file in the place of the folder `.asset-state` or
  `dist/stage` gives the same error on Windows and on Linux.
- A manifest that does not evaluate to a Moonwell project (one that does not amend `@moonwell/Project.pkl`) is
  refused with the place of the value that does not fit, as the manifest writes it (`map.folder`), and what the
  value is.
- A path in `assets.paths` or `assets.exclude` that no asset may have ("Invalid asset path", "Reserved map path")
  is reported with `moonwell.pkl` as its file, and its hint ends with "Fix the assets block in moonwell.pkl.", as
  the other mistakes in that block are. It named no file.
- `assets:paths` with a file that does not exist, that is a folder, or that cannot be read reports it as every
  other file is reported: the file before the message, from the folder the command runs in, and a hint. A model
  whose name starts with two dots (`..knight.mdx`) is named from that folder too, and not by its whole path.
- A folder of a library below `.moonwell/`, or `moonwell.lock`, that cannot be removed is reported as "Removing …
  failed", where it said "Writing … failed".
- A `yue.path` that is there and cannot be started (a folder, a file that is no program) is reported with
  `moonwell.local.pkl` as its file and a hint that says what to do there. A `yue.path` that does not exist has the
  same hint.
- On Windows, `init` names a file it could not write, and the `PklProject` whose dependencies did not resolve, with
  `/`, as every other error names a file: `my-map/src/main.yue`, where it said `my-map\src\main.yue`. A failed
  `pkl project resolve` has a hint.

## 0.9.1 (2026-10-03)

- **Pkl no longer has to be installed first.** Moonwell uses the `pkl` on the PATH when it is Pkl 0.32 or newer, as
  before. Otherwise it downloads Pkl 0.32.1 once into its cache (about 100 MB, checked against a pinned checksum) and
  runs that. Before, a missing or older `pkl` stopped every command.
- An older `pkl` on the PATH is reported with a warning, since a `pkl` command you type yourself still runs it.
- `moonwell setup` copies Moonwell's own Pkl to the cache's `bin` folder, next to `yue`, so `pkl project resolve` works
  from a shell. On Windows the install line already put that folder on the PATH; elsewhere `setup` prints the command
  that does. It does so before it reads the project, so it works in a project that is still on an older package.
- Moonwell downloads Pkl for Windows and Linux on x86-64. On another system (a Moonwell built from source), a missing
  or older `pkl` fails as before, with the install hint.
- **Nothing to change in a project:** this version reads projects on the `moonwell@0.9.0` package as they are.

### Release gate

Steps 1 and 2 (CONTRIBUTING) passed 2026-10-03 on Windows, and CI passed on Ubuntu and Windows. Steps 3 to 13 were not
re-run: nothing that reaches a map changed. The program was also run with no `pkl` on the PATH and an empty cache:
`init --link` downloaded Pkl 0.32.1 once, `setup` copied it to `bin` and printed the PATH command, and `check` passed.

Published 2026-10-03 as the GitHub release `moonwell@0.9.1`, built by the release workflow, whose last job ran the
README's install line on Ubuntu and Windows, then `moonwell init my-map` and `moonwell build`.

## 0.9.0 (2026-10-02)

- **The preview picture can be a PNG.** `settings.info.preview` takes a `.png` beside a `.tga` and a `.blp`, with the
  same rules: 256×256 or 512×512 pixels, at a path from the project folder, not under `assets/`.
- A PNG of any kind is read: 8 or 16 bits, colour, grey or palette, interlaced or not. Transparency is dropped and each
  pixel keeps the colour the file stores for it, as for a TGA.
- Nothing changes in the game: a build writes a PNG into the map as the same `war3mapMap.tga` it writes for a TGA of
  that picture.
- A PNG of another size fails before its pixels are read, and a file that is not a PNG or cannot be read fails `check`
  and the build, naming the file. The message for another extension now reads
  `must be a .tga, a .blp or a .png file`.
- **To get it in a project,** install this version (run the install line again), set `moonwell@0.9.0` in `PklProject`
  and run `pkl project resolve`. The 0.9.0 program refuses a project that is still on a 0.8 package, and says so.

### Release gate

Steps 1 and 2 (CONTRIBUTING) passed 2026-10-02 on Windows, and CI passed on Ubuntu and Windows. Steps 3 to 13 were not
re-run, by the maintainer's decision: the picture reaches the map in the layout step 13 passed for 0.7.0, and a test
shows that a PNG and a TGA of one picture give the same bytes. A PNG written by Windows' own encoder was built into a
map as well, and its pixels read back from the map's `war3mapMap.tga`.

Published 2026-10-02 as the GitHub release `moonwell@0.9.0`, built by the release workflow, whose last job ran the
README's install line on Ubuntu and Windows, then `moonwell init my-map` and `moonwell build`.

## 0.8.1 (2026-10-02)

- **YueScript 0.34.3 is the compiler,** in place of 0.34.2. The floor division operator `//` now works: 0.34.2
  compiled a file that used it to no Lua at all.
- **To get it in a project,** install this version (run the install line again), set `moonwell@0.8.1` in `PklProject`
  and run `pkl project resolve`. A project that stays on the 0.8.0 package keeps YueScript 0.34.2 and builds as before,
  with either program.
- **Bitwise operators (`&`, `|`, `~`, `<<`, `>>`) still do not build from YueScript,** but they now fail with a
  message that says so, at the line that uses one, instead of the compiler's own text or an empty file:
  `YueScript compiled this file but could not rewrite its Lua`. Put such code in a Lua file under `lua/`, which is
  bundled as written; the game's Lua 5.3 has the operators.
- Nothing else changes in a build: 24 YueScript sources (the template, both libraries' examples, the gate map's
  probes) compile to the same Lua with both versions, in normal and minified builds.
- `moonwell setup` puts the new compiler in the `bin` folder for the editor, as it does for any version.

### Release gate

Steps 1 and 2 (CONTRIBUTING) passed 2026-10-02 on Windows, and CI passed on Ubuntu and Windows. Steps 3 to 13 were not
re-run, by the maintainer's decision: compiled Lua is unchanged for code that built before, and `//` is an operator of
the game's Lua 5.3.

Published 2026-10-02 as the GitHub release `moonwell@0.8.1`, built by the release workflow, whose last job ran the
README's install line on Ubuntu and Windows, then `moonwell init my-map` and `moonwell build`.

## 0.8.0 (2026-10-02)

- **Moonwell is one program, `moonwell`, and no longer needs Deno.** Install it with one line (PowerShell on Windows,
  `sh` on Linux; see the README), then run `moonwell build` where you ran `deno task build`. The same goes for every
  command: `init`, `setup`, `build`, `test`, `dev`, `check`, `assets:check`, `assets:sync`, `assets:paths`,
  `settings:check`, `objects:eval` and `objects:check`, with the same flags.
- **Nothing else changes for a project.** The manifest, the Pkl package, the map formats, the libraries and the editor
  support are as in 0.7.0, and a build writes the same files, byte for byte. The caches under `dist/` and `.moonwell/`
  carry over, so the first build after upgrading recompiles and downloads nothing.
- **Upgrading a project from 0.7:** install `moonwell`; set `moonwell@0.8.0` in `PklProject`; run
  `pkl project resolve`; delete `deno.json` and `deno.lock`. `moonwell setup` says so when it finds a `deno.json` an
  older Moonwell wrote. A new project has no `deno.json`, and `init` ends with `cd <dir> && moonwell build`.
- **A project is pinned by its Pkl package version.** The program refuses a project whose `moonwell` package has
  another major or minor version, and its hint gives the install line of the version the project wants.
- Messages that named a `deno task` name the `moonwell` command. The reason an operating system gives inside a message
  ("Reading x failed: ...") is worded by Go now. Everything else a command prints is unchanged.
- The packed `.w3x` holds the same files in the same order with the same contents; its compressed bytes differ,
  because Go's zlib compressor is not Deno's.
- `dev` looks at the watched files four times a second instead of waiting for the operating system's events. It
  watches what it watched before.
- Releases are built by GitHub Actions from the tag: `moonwell-windows-amd64.exe` and `moonwell-linux-amd64` with their
  SHA-256 checksums, the two install scripts, and the Pkl package. Nothing is published to JSR again;
  `@moonwell/cli@0.7.0` is the last version there.
- For contributors: the code is Go with the standard library only (`cmd/moonwell`, `internal/`, `tools/gen`), the
  TypeScript and every `deno.json` are gone, and the checks are `go vet ./...`, `gofmt -l .` and `go test ./...`.
  The tools of moonwell-wrappers, moonwell-systems and the gate map are Lua run with `yue -e`, and run `moonwell`.
- Documentation: the design (`2026-10-02-moonwell-go-toolchain-design`) and its plans (`2026-10-02-moonwell-go-*`).

### How it was checked

The Go program is a redesign, not a translation, so it was checked from outside. Every test of the TypeScript CLI has a
Go counterpart (626 tests in 35 packages). While both programs existed, a conformance suite ran each of them on its own
copy of the same project and compared the exit code, every printed line, every written file, and the packed map
unpacked: a new project, the gate map with both libraries, a project with every map setting, one with objects of every
category, one with assets on World Editor 3.00's import file, a preview picture as TGA and as BLP, Lua modules with the
example library by tag, and five failing projects. Its last green run on Ubuntu and Windows is commit `f82dd66`; it
found one difference on the way (the folder in `setup`'s PATH command when `yue.path` is set), which was fixed. The
generators were checked the same way: the Go ones write `schema/generated/`, `natives.json`, `metadata.json` and
`game-paths.txt` as the TypeScript ones did, from the same game exports.

### Release gate

A staged map folder equal to 0.7.0's is a folder that passed its gates, so what is new to the game and the machine was
run by hand, with the `moonwell` built from the release commit (CONTRIBUTING, steps 1 to 6):

- Steps 1 and 2 passed 2026-10-02 on Windows, and CI passed on Ubuntu and Windows: `go vet`, `gofmt` and 626 tests,
  with `pkl`, the YueScript compiler and the network required.
- Steps 3 to 6 passed 2026-10-02, run by the maintainer (their setup is Warcraft III Reforged 3.0.0.24268 with World
  Editor 3.00): `setup` and `test` in the template, with the game started and left open by the program and the Captain
  changing colour; the error position of a failing hook; a minified build played as a packed map; and the packed map
  opened in World Editor.
- Steps 7 to 13 were not run by hand: they are covered by the conformance suite, whose last green run is commit
  `f82dd66`.

Published 2026-10-02 as the GitHub release `moonwell@0.8.0`, built by the release workflow from the tag. Its last job
ran the install line of the release on Ubuntu and Windows, then `moonwell init my-map` and `moonwell build`, on machines
that had neither the program nor the Pkl package.

At first the README's install line answered 404: the release had been marked as a pre-release, like every
release before it, and GitHub's `releases/latest` skips those. From 0.8.0 on, Moonwell's releases are full releases, and
the release workflow now runs the README's line itself. The maintainer then installed 0.8.0 with that line on Windows:
the downloaded program started without a SmartScreen warning.

## 0.7.0 (2026-10-01)

- **A picture of your own in the game's map list.** `settings.info.preview = "preview.tga"` names a `.tga` or a `.blp`
  of 256×256 or 512×512 pixels, at a path from the project folder. Reforged ignores `war3mapPreview.tga` and shows the
  map's minimap file, so a build puts the picture in the minimap's place in the staged map, keeps World Editor's minimap
  as `war3mapMinimap.blp`, and adds one call at the end of `main()` in `war3map.lua`
  (`BlzChangeMinimapTerrainTex("war3mapMinimap.blp")`), so the game itself shows the normal minimap. The source map is
  never changed.
- The picture is checked strictly, because one the game cannot read closes the game the moment the map is selected in
  the list. A TGA (24 or 32 bits, plain or run-length encoded, rows from the top or the bottom) is written into the map
  again in the one layout the game was seen to accept, fully opaque. A BLP must be a BLP1 with JPEG or palette content
  and is used as it is. Another size, format or extension fails `build`, `test`, `check`, `dev` and `settings:check`.
- `settings:check` lists the preview's files, a file a build removes as `war3mapMap.blp (removed)`. `dev` starts a cycle
  when the picture changes.
- An asset named `war3mapPreview.*` or `war3mapMap.*` is refused as before; its hint now names the setting.
- Limits, in the README: the game draws its start location markers over the picture, placed for a 256×256 one; and a
  World Editor trigger that sets the minimap at map initialization is overridden by the build's call (gameplay code in
  an `on_main` hook runs later and wins).
- Documentation: the design and plan (`2026-10-01-moonwell-map-preview`), with the measurements of two probes.

### Release gate

Steps 1 and 2 (CONTRIBUTING) passed 2026-10-01 on Windows, and CI passed on Ubuntu and Windows: the type check, lint and
format; 486 unit tests, 30 runtime tests, the Pkl tests (47, and 28 Pkl-backed tests), 34 end-to-end tests and 2 network
tests. Among them: a real build whose packed map holds `war3mapMap.tga` and `war3mapMinimap.blp`, no `war3mapMap.blp`,
and the call as the last statement of `main()`. 62 mutations of the new code are each caught by a test.

Two probes came before the design, on Warcraft III Reforged 3.0.0.24268 (`../wrappers-gate/PROBE-PREVIEW-RESULTS.md`):
eight maps in the single-player map list, and three maps for the call's behavior and timing.

The new step 13 (the map preview) passed 2026-10-01 on Warcraft III Reforged 3.0.0.24268, tested by the maintainer: a
map built from a 24-bit TGA written by Pillow showed the picture in the single-player map list and the normal minimap in
the game. Steps 3 to 12 were not re-run: their code is unchanged.

Published to JSR 2026-10-01; the maintainer checked `init` and `build` from `jsr:@moonwell/cli@0.7.0`.

## 0.6.0 (2026-10-01)

- **Libraries can ship files for the map.** A library names a folder of them in a `moonwell-library.json` at its root
  (`{ "dir": "src", "assets": "assets" }`), and every map that lists the library imports those files, each at its path
  in that folder, with the map's own assets: in builds, `check`, `assets:check`, `assets:sync` and `assets:paths`. The
  map's own file wins over a library's at the same in-map path, and the command says so; two libraries at one path fail.
  The same file can name the library's module folder (`dir`), so a map may leave `dir` out of its manifest; a `dir` in
  the manifest still wins.
- `moonwell.lock` records an `assets` hash for a library that ships files. Existing lock files stay valid. The first run
  after upgrading downloads each GitHub library once more.
- `assets:check`, `assets:sync` and `assets:paths` sync the libraries first, as `check` and `setup` do.
- A local library's `path` is the library's root: its `moonwell-library.json` is read from there, and `dev` watches its
  assets folder and that file.
- The template's commented library example names `mdlsvensson/moonwell-example-lib` `v0.2.0`, which has such a file.
- Documentation: the `moonwell-wrappers` v0.5.1 review, the performance and port-needs research notes with their probe
  results, and the designs and implementation plans for `moonwell-wrappers` v0.6.0 (the refactor after the review) and
  v0.7.0 (the port prerequisites: damage events, sync, collision size and pathing), both released 2026-09-30.
- Documentation: the design of `moonwell-systems`, the `wc3-lib` port, and the plan for its release 1 (v0.1.0, released
  2026-09-30); a backlog item for replacing Deno. The design and plan of its release 2 (v0.2.0: buffs, auras and
  dummies, released 2026-10-01), of its release 3 (v0.3.0: the damage pipeline, released 2026-10-01) and of its release
  4 (v0.4.0: geometry, terrain, missiles and knockbacks, released 2026-10-01), with the measurements of its two probes
  in that spec. The design and plan of its release 5 (v0.5.0: save codes, sync and save files, released 2026-10-01),
  with the measurements of three Preload probes in that spec. The port of `wc3-lib` is complete.
- Documentation: the design and plan of `moonwell-wrappers` v0.8.0 (input listeners, weather effects, art from ability
  data, four Trigger registrations and `fromEvent()`, released 2026-10-01), with the measurements of its two probes in
  that spec.

### Release gate

Steps 1 and 2 (CONTRIBUTING) passed 2026-10-01 on Windows: the type check, lint and format; 466 unit tests, 30 runtime
tests, the Pkl tests (46, and 27 Pkl-backed tests), 33 end-to-end tests and 2 network tests. Among them: a real build
whose staged map holds a local library's file and lists it in `war3map.imp`, with the map's own file replacing another;
`assets:check`, `assets:sync` and `assets:paths` with a library's files; and the downloads of
`mdlsvensson/moonwell-example-lib` `v0.2.0` (which ships a file and names its own module folder) and `v0.1.0` (which
locks to the entry it always had). 34 mutations of the new code are each caught by a test. Steps 3 to 12 were not
re-run: the release decides which files enter the import path that step 7 already covered in the game and in World
Editor, and the end-to-end test checks the staged map's bytes.

Published to JSR 2026-10-01; `init` and `build` from `jsr:@moonwell/cli@0.6.0` were checked in a new folder.

## 0.5.2 (2026-09-30)

- Builds stop with an error naming the file when YueScript reports success but writes no Lua for a file with code.
  YueScript 0.34.2 does this, in normal and minified builds, for any file that uses the floor division operator `//`,
  which until now silently left that module out of the map. README: write `math.floor(a / b)` instead. Reported upstream
  as IppClub/YueScript#256.

### Release gate

Steps 1 and 2 (CONTRIBUTING) passed 2026-09-30 on Windows, and CI passed on Ubuntu and Windows: every check, including
two new compile tests (an empty output for a file with code fails; a file using `//` fails, normal and minified). Steps
3 to 12 were not re-run: the release only adds a failure after compilation, and the e2e tests build real maps, normal
and minified.

## 0.5.1 (2026-09-30)

- `assets:sync` stops at Ctrl+C while it plans, before printing or writing anything, and then says that nothing was
  written; an interrupt during writing still undoes every change.
- Natives: `UnitAlive(unit)`, a common.ai native that map Lua can call (confirmed on Warcraft III 3.0.0.24268 by the
  wrappers probe run, 2026-09-29), is now known: the unknown-global check accepts it and `natives.d.lua` declares it.
- README: the note about a `yue` console window on Windows is gone. YueScript extension 0.2.10 hides the window (our
  fix, pigpigyyy/yuescript-vscode#11).
- Documentation: approved design and implementation plan for the separate `moonwell-wrappers` library (sub-project 4c),
  released as `mdlsvensson/moonwell-wrappers` `v0.1.0` after its in-game and first-tag consumption gates (recorded in
  AGENTS.md).
- Documentation: research notes comparing w3ts and WCSharp with `moonwell-wrappers`, with the probe results
  (`docs/superpowers/research/`).
- Documentation: design and implementation plan for `moonwell-wrappers` v0.4.0 (classic UI: dialogs, multiboards,
  leaderboards, quests, defeat conditions, timer dialogs), released 2026-09-29.
- Documentation: design and implementation plan for `moonwell-wrappers` v0.5.0 (frames), released 2026-09-29.
- Documentation: the roadmap from `moonwell-wrappers` v0.5.0 to the `wc3-lib` port.

### Release gate

Steps 1 and 2 (CONTRIBUTING) passed 2026-09-30 on Windows: every check passes from a clean checkout, including
`test:network`, and `cli/data/game-paths.txt` is the 3.0.0.24268 list. Steps 3 to 12 were not re-run, by the
maintainer's decision: this release changes no code that runs in the game or in World Editor (a native name the editor
and the unknown-global check know, and `assets:sync`'s interrupt handling, which unit and Pkl tests cover).

Published to JSR 2026-09-30; the maintainer checked `init` and `build` from `jsr:@moonwell/cli@0.5.1`.

## 0.5.0 (2026-09-28)

- Lua modules: `.lua` files in `lua/` are modules named by their path, sharing one namespace with `src/` (a name both
  define fails the build), and `x/init.lua` answers to `x`. They are bundled unchanged when required, and runtime errors
  in them name the `.lua` file and line, also in `--minify` builds. Their top-level globals are known to the
  unknown-global check. New projects have a `lua/` folder, and `.luarc.json` resolves it in the editor; `setup` adds the
  entries to older projects' `.luarc.json`. `dev` watches `lua/`. A leading byte order mark and a `#` first line are
  skipped, as standalone Lua does, and `moonwell` is reserved for the built-in module.
- Libraries: the new `libraries` block in `moonwell.pkl` brings YueScript and Lua modules from a GitHub tag (`github`,
  `tag`, `dir`) or a local folder (`path`) into `.moonwell/libraries/`, and `moonwell.lock` records each tag's commit; a
  moved tag fails the command. Library modules keep their own names and share one namespace with `src/` and `lua/`; a
  `.lua` file next to a `.yue` file of the same name is its compiled output and is skipped. A local library copies only
  its `.yue` and `.lua` files, and no library keeps files in folders whose name starts with `.`. The editor sees library
  modules in `.moonwell/lua/`, and `.luarc.json` leaves the copies in `.moonwell/libraries/` out of the workspace;
  `setup` adds both entries to older projects' `.luarc.json`.
- Changed: `global` lines and the top-level globals of Lua modules count as known names only for modules the map
  requires (its entry, or the `--entry` file, and every module reached from it). Files under `src/` that the map does
  not require are no longer checked for unknown globals. A global declared only in a file nothing imports is now
  reported as unknown where a required module uses it: import the file, or add the name to `lint.globals`.

### Release gate

Passed 2026-09-28 on Warcraft III Reforged 3.0.0.24268 and World Editor 3.00.

Steps 1 to 6 (CONTRIBUTING), tested by the maintainer in `template/` apart from steps 1 and 2: every check passes from a
clean checkout, including `test:network`, and `cli/data/game-paths.txt` is the 3.0.0.24268 list. `deno task test` prints
"Moonwell is running." and the Captain changes colour; an `error "gate"` in `on_main` names `src/main.yue` and the right
line. The `--minify` build plays from `dist/bin/map.w3x`, and the packed map opens in World Editor. Steps 7 to 11
(assets, map settings, object data, editor, macros) were not re-run: this release does not change their code, and step
12 covers its editor changes.

Lua modules (CONTRIBUTING step 12, first part) passed 2026-09-28 on Warcraft III Reforged 3.0.0.24268, tested by the
maintainer in a new project with a module (`lua/greeter.lua`) and a global-style file (`lua/counter.lua`) used from
`src/main.yue`: both print in the game; an error inside the module in a `--minify` build names `lua/greeter.lua:5`; the
editor completes the module's function and knows the global. `deno task check` and `build` pass, and the Lua extension's
lua-language-server, run from the command line, resolves the module and flags an unknown global.

Libraries (CONTRIBUTING step 12, second part) passed 2026-09-28 in a new project using
`mdlsvensson/moonwell-example-lib` `v0.1.0` by tag: `check` downloads it and locks commit `c07126f`; the editor
completes `loud.shout`, and the game shows the shout. Deleting `.moonwell/` and checking again leaves `moonwell.lock`
unchanged. Pointed at a local clone through `moonwell.local.pkl`, the game shows the changed `hello`, `moonwell.lock`
stays unchanged, and a `loud.lua` the editor wrote beside `loud.yue` in the clone does not break `check`; switching back
to the tag downloads it again and checks it against the lock. lua-language-server, run from the command line, reports no
duplicate definitions. CI runs the network test (`test:network`) on Ubuntu and Windows.

## 0.4.0 (2026-09-28)

- Editor support for VS Code's YueScript extension: new projects get `yueconfig.yue`, `.luarc.json` and a recommendation
  of the YueScript and Lua (`sumneko.lua`) extensions, and `check`, `build`, `test` and `dev` write LuaLS declarations
  to `.moonwell/types/` for every native, Blizzard.j function and global of Warcraft III 3.0.0.24268, the Moonwell
  runtime, the project's object ids and the map's own globals. `.gitignore` gains `.moonwell/` and `src/**/*.lua` (the
  `.lua` files the extension writes on save). See "Editor setup" in the README.
- The editor declarations include `require`, so `import` lines are not flagged as an undefined global (the game's Lua
  has no `package` library, which `.luarc.json` turns off, but the Moonwell runtime defines `require`). Hook callbacks
  may return a value, so a YueScript callback, which returns its last expression, is not flagged as returning too many
  values.
- `setup` adds the editor files and `.gitignore` lines older projects lack, keeps a copy of the pinned YueScript in the
  cache's `bin` folder, and prints the command that puts it on PATH when `yue` is missing there or another version.
- `setup` now also plans the project's objects to write the editor declarations, so objects that do not resolve, or a
  missing or unreadable source map when there are objects, make `setup` fail as they make `check` fail.
- `check`, `build`, `test` and `dev` report every global a gameplay file uses that nothing defines, with its file, line
  and column and the nearest known name (`Did you mean CreateUnit?`). Known globals are the game's natives, every
  common.j and Blizzard.j function, global and constant, the game's Lua libraries, the source map's `war3map.lua`
  globals, names declared with `global` under `src/` and the new `lint.globals` list. `lint.unknownGlobals = "warning"`
  reports them without failing. New projects show the `lint` block in `moonwell.pkl`.
- Macros: `import "moonwell.macros" as {:$FourCC}` gives gameplay code `$FourCC "hfoo"`, which compiles to the rawcode's
  integer (`1751543663`) and fails the compile on anything but a 4-character string literal. The template creates a
  standard Footman with it next to the Captain.
- After upgrading, run `deno task setup` once: it adds the editor files and the new `.gitignore` lines. Until then,
  `check` and `build` leave `.moonwell/` untracked in a 0.3 project. `check` and `build` now fail on unknown globals by
  default, so a project that relies on `global *`, `global ^` or globals defined elsewhere should list them in
  `lint.globals`, or set `lint.unknownGlobals = "warning"` for a while.

### Release gate

Passed 2026-09-27 on Warcraft III Reforged 3.0.0.24268 and World Editor 3.00.

Steps 1 to 6 (CONTRIBUTING), tested by the maintainer in `template/` apart from steps 1 and 2: every check passes from a
clean checkout, and `cli/data/game-paths.txt` is the 3.0.0.24268 list. `deno task test` prints "Moonwell is running.",
the Captain changes colour and the Footman stands beside it, and the game window stays open after the CLI exits. An
`error "gate"` in `on_main` shows `[moonwell] on_main failed: src/main.yue:6: gate`. The `--minify` build plays from
`dist/bin/map.w3x`, and the packed map opens in World Editor. Steps 7 to 9 (assets, map settings, object data) were not
re-run: this release does not change their code.

Editor and unknown globals (CONTRIBUTING step 10) passed 2026-09-27, tested by the maintainer in Antigravity IDE (a VS
Code fork) with the YueScript extension 0.2.9 and the Lua extension 3.19.1, and re-run in full after Plan 3b:

- In the editor: completion and hover for natives, `mw.on_main`, `objects.units.captain`, a second project module, and
  the `gg_unit_` global of a unit placed in World Editor and used by a trigger. `CreatUnit` is underlined.
- `deno task check` fails with `src/main.yue:9:10 › Unknown global CreatUnit.` and `Did you mean CreateUnit?`; with
  `lint.unknownGlobals = "warning"`, `deno task build` prints the same lines as warnings and succeeds. `git status`
  shows no generated files.
- The re-run found two editor warnings, now fixed: `import` lines were flagged as using an undefined `require`, and a
  hook callback whose last line has a value was flagged as returning too many values. The fix was checked with the Lua
  extension's lua-language-server run on the project from the command line (`--check`): no warnings remain for the
  template's code, and `CreatUnit` is still flagged.

Macros and the game's Lua (CONTRIBUTING step 11) passed 2026-09-27 on Warcraft III Reforged 3.0.0.24268, tested by the
maintainer in a new project:

- `deno task test` shows the standard Footman made with `$FourCC("hfoo")` beside the Captain. The built map contains
  `CreateUnit(Player(0), 1751543663, -45, -650, 270)`.
- The Lua probe prints `missing: collectgarbage dofile loadfile debug io package` and `os: clock date difftime time`, as
  `tools/natives/lua-extras.json` and the README record.
- In the editor, the `moonwell.macros` import and `$FourCC("hfoo")` show no error or warning; the Lua extension's
  lua-language-server, run from the command line on the project, finds no problems either.

## 0.3.1 (2026-09-27)

- Fixed: after World Editor 3.00 saved a map with synced assets, every command that reads `war3map.imp` (`build`,
  `assets:check`, `assets:sync`) failed with "unknown flag 29". World Editor saves custom-path imports with flag 29,
  which is now read as a custom path. `assets:sync` keeps the flag an owned import already has, so saving in World
  Editor leaves nothing to sync.
- `assets:paths` with no file argument reports every model it can read and marks each unreadable one in its place,
  instead of stopping at the first; the command still fails when any model was unreadable.
- Ctrl+C during `assets:sync` undoes every change it already made to the source map and exits with code 130. A second
  Ctrl+C still exits at once.
- `assets:sync` writes no `.asset-state/<map>.json` when it owns no files, and removes the file once the last asset is
  gone. An invalid path in the state file now carries the state file's hint.
- `assets:paths` reads large text `.mdl` models with far less memory (about a tenth for a 70 MB model).

### Release gate

Assets (CONTRIBUTING step 7) passed 2026-09-27 on Warcraft III Reforged 3.0.0.24268 and World Editor 3.00, tested by the
maintainer, including a World Editor save after `assets:sync`: both custom imports came back with flag 29,
`assets:check` reported no changes and `build` succeeded. The other steps were not re-run; this release changes only
asset code.

## 0.3.0 (2026-09-26)

- Object data: custom units, heroes, buildings, items, abilities, buffs and upgrades in Pkl files under `objects/`,
  merged by `moonwell.pkl` and typed by the `moonwell` package's `ObjectFile.pkl`, with friendly field names from World
  Editor's labels, per-level values as a `List`, and `properties` for ability-specific fields by name or rawcode.
- Builds add the objects to the staged map's modification files and their `war3mapSkin.*` counterparts. World Editor's
  own objects, including modified standard objects, are kept byte for byte; the source map is never changed.
- Builds write `src/generated/objects.yue` with every object's id for gameplay code; `check` fails when it is stale.
- Every object problem is reported at once, with its file, object and field, and the nearest standard ids or field names
  as hints.
- `objects:check` validates the objects and lists the files a build would change; `objects:eval` prints the resolved
  objects as JSON. `check` and `dev` validate objects too, and `dev` watches `objects/`.
- The template's footman is now the Captain, a custom unit in `objects/units.pkl`.
- New projects get a `.gitattributes` that checks text out with LF line endings and keeps World Editor's map files and
  `assets/` byte for byte.

### Release gate

Object data (CONTRIBUTING step 9) passed 2026-09-26 on Warcraft III Reforged 3.0.0.24268 and World Editor 3.00, tested
by the maintainer.

- In game, the Captain shows its model, name and icon. A hero based on the Paladin shows its custom name and strength
  and learns a custom Holy Light to level 4, which heals the four per-level amounts. A custom item shows its name and
  icon. A custom Blacksmith offers only its custom upgrade, with each level's tooltip.
- A Footman modified in World Editor's source map keeps its change next to the Moonwell objects.
- The packed map opens in World Editor, and the Object Editor lists the custom objects with their values.

## 0.2.0 (2026-09-26)

- Assets: files under `assets/` are imported into builds. The manifest's `assets` block maps them to exact paths and
  excludes files. The `assets:check` and `assets:sync` commands write them into the source map for World Editor, with
  ownership tracking and rollback.
- `assets:paths` lists the files a model (`.mdx` or `.mdl`) references as in-game or custom paths, and whether the
  project imports them.
- `init` creates World Editor's icon folders under `assets/ReplaceableTextures/`.
- Map settings: the manifest's `settings` block sets the map's name, author, description and loading screen, existing
  player slots and forces, sound environment, water colour and fog, and hero level and food limits. Advanced raw
  `gameplayConstants` and `gameInterface` mappings set any key in `war3mapMisc.txt` and `war3mapSkin.txt`. Null settings
  keep the map's values; the new project template writes every everyday setting out at `null`.
- Builds apply settings to the staged map: `war3map.w3i` is patched byte for byte (versions 18, 25, 28, 31, 32, 33 and
  39), the matching World Editor calls in `war3map.lua` are edited to agree, and the text files are merged key by key.
  The source map in `maps/` is never changed.
- `settings:check` lists the internal files the settings would change, without building or writing the map. `check` and
  `dev` validate settings too.

### Release gate

Assets (CONTRIBUTING step 7) passed 2026-09-26, tested by the maintainer.

Map settings (CONTRIBUTING step 8) passed 2026-09-26 on Warcraft III Reforged 3.0.0.24268 and World Editor 3.00, after
three fixes the gate found: a player name no longer adds `SetPlayerName` (it crashed the game on lobby creation), the
hero level constant is `MaxHeroLevel`, and map-info colours are written blue, green, red, alpha.

- The lobby shows the map name, author, description, player name and race, and the force name.
- In game, the fog and water are red, the food ceiling is 50 and a hero stops at the maximum level.
- The packed map opens in World Editor with the configured description, loading screen, player and force properties,
  fog, water tint, sound environment and gameplay constants.

## 0.1.0 (2026-09-25)

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
