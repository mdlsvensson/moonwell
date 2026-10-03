# Replacing Deno: the reasons, the work, and the paths (2026-10-01)

A report for the maintainer, who decides. It is not a design: it says why Deno should go, what Deno does for Moonwell
today, what any replacement has to provide, and which paths exist with what each costs. The design starts when the
maintainer says so (roadmap 5.1).

The maintainer's premise (2026-10-01): Moonwell targets four gameplay languages, Lua, Teal, Fennel and YueScript.
TypeScript and C# support are dropped. Nothing a map author writes or runs is JavaScript.

## 1. The short version

- **Deno is the only JavaScript in the project, and it is the part every user has to install.** The compilers Moonwell
  drives are native programs (`yue`, `pkl`) or Lua programs (Teal's `tl.lua`, Fennel's `fennel.lua`). The libraries
  are Lua. Only the CLI that connects them is TypeScript.
- **The CLI is 8,300 lines with 12,400 lines of tests**, and it uses nothing from the JavaScript ecosystem beyond
  Deno's standard library. What it needs from its platform is a short, ordinary list: files, processes, a file
  watcher, HTTPS downloads, zip, SHA-256 and byte arithmetic.
- **There are two real families of replacement.** Rewrite the CLI in a compiled language (Rust or Go) and ship one
  binary; or write it in the Lua family and run it on a small host program that supplies what plain Lua lacks.
- **The costs are a rewrite with no new feature, and owning our own binaries.** The rewrite can be made safe: the
  current CLI is a working reference, so every ported part can be compared with it byte for byte.
- **My lean,** which §9 argues and which six small spikes would confirm or overturn: a thin native host (Rust with an
  embedded Lua 5.4) and the CLI's logic in the Lua family on top of it, ported behind byte-for-byte comparison with
  today's CLI.

## 2. What Deno does for Moonwell today

Measured on `main` at `d214bc8`.

### 2.1 The code

| Part | Lines | Notes |
| --- | --- | --- |
| `cli/src` | 8,326 | 16 areas; the largest are settings (1,478), object data (1,190), commands (778), libraries (596), assets (592) and the YueScript driver (564) |
| `tools/` | 1,052 | generators for the embedded files, the in-game path list, the metadata and the natives |
| `cli/tests` | 12,440 | 81 files: 486 unit tests, 30 runtime, 28 Pkl-backed, 34 end-to-end, 2 network |
| `cli/runtime` | 155 | the Lua runtime and the macro module that go into maps; not Deno code |
| moonwell-wrappers `tools/` | 357 | test runner, syntax check and integration check, in TypeScript |
| moonwell-systems `tools/` | (Lua) | already runs on `yue -e`, with no Deno |

The CLI imports only Deno's standard library: `@std/path`, `@std/fs`, `@std/assert`, `@std/encoding/base64` and
`@std/cli/parse-args`. There is no `npm:` and no `node:` import, by the project's rule.

### 2.2 What the code asks of its platform

| Need | Used for | Today |
| --- | --- | --- |
| Files and folders | everything: read, write, list, stat, rename, remove, temp folders, symlink checks | `Deno.readFile`, `readDir`, `stat`, `lstat`, `rename`, `remove`, `mkdir` (about 150 call sites) |
| Processes | `pkl` (evaluate the manifest, resolve the project), `yue` (compile, list globals), the game | `Deno.Command`, with output captured or detached |
| A file watcher | `dev` | `Deno.watchFs` (6 uses) |
| HTTPS downloads | the `yue` binary, a library's tag archive from GitHub | `fetch` |
| Zip and deflate | library archives, the `yue` download | `DecompressionStream`, `CompressionStream` |
| SHA-256 | `moonwell.lock`, asset hashes | `crypto.subtle.digest` |
| Byte arithmetic | the MPQ writer and its encryption, `war3map.w3i`, object data, models, pictures | `DataView`, typed arrays |
| Text | UTF-8 decoding that fails on bad bytes, JSON (38 uses), regular expressions (96 uses) | built in |
| Signals | Ctrl+C during `assets:sync` rolls back | `Deno.addSignalListener` |
| Asynchrony | 115 `async` functions | built in; almost all of it is sequential file work |

Nothing on this list is special to JavaScript. Two entries are worth a note for a Lua-family port: Lua has no JSON and
no regular expressions built in (its own patterns are weaker), so both come from a library or the host.

### 2.3 What a map author sees of Deno

- They install Deno before anything else.
- A new project starts with `deno run -A jsr:@moonwell/cli@<version> init my-map`.
- Every project has a `deno.json` with eleven tasks; every command is `deno task <name>`.
- Deno refuses a JSR version published less than 24 hours ago unless `--min-dep-age=0` is passed.
- `-A` gives the CLI every permission, so Deno's permission model protects nothing here.

### 2.4 What a release needs from Deno

- `deno publish` to JSR, which only the maintainer can run (it opens a browser to sign in).
- Then a check of `init` and `build` from JSR, which fails in the agent's shells whenever Pkl has to download.
- The Pkl package is released separately, as a GitHub release. So a release already has two homes.

### 2.5 What is already not Deno

- `pkl`: a native executable the user installs; the CLI runs it.
- `yue`: a native executable the CLI downloads and pins. It carries a full **Lua 5.4** interpreter (checked today:
  integers, bitwise operators, `io.popen`; no file system library, no sockets) and the compiler as a Lua module
  (`require("yue")`, with `to_lua`). `yue -e file.lua` runs any Lua script.
- The editor support: lua-language-server and the YueScript extension.
- moonwell-systems' tooling and every library test: Lua.

So a Lua interpreter is on every user's machine the moment Moonwell is set up. Teal's and Fennel's compilers are each a
single Lua file with no dependencies, and they could run on that interpreter today.

## 3. Why Deno should go

1. **It is a runtime from another world, and the user pays for it.** A map author writes YueScript, Teal, Fennel or
   Lua and Pkl. To do that they install a JavaScript runtime, keep a `deno.json`, and learn `deno task` and one of
   Deno's quirks. Every one of those is a thing to explain that has nothing to do with making a map.
2. **The project is split across two ecosystems for no gain.** The libraries, their tests, their tools (in
   moonwell-systems) and the runtime are Lua. The CLI and its tests are TypeScript. Nothing is shared between the
   halves. 32-bit arithmetic was written once in TypeScript (the MPQ encryption) and once in Lua (the save codec), and
   the CLI carries a hand-written reader of Lua source (460 lines) because it cannot ask a Lua for help.
3. **The people who would contribute know Lua, not TypeScript.** The Warcraft III scripting community writes Lua,
   JASS and, in part, TypeScript through TypeScriptToLua, which this project has just decided not to target.
4. **The next two features are Lua programs.** Teal and Fennel are compiled by Lua code. A Lua-hosted CLI runs them
   in its own process. (Honesty requires saying that the Deno CLI could run them too, through `yue -e`; this reason
   is about fit, not necessity.)
5. **Releases get simpler.** JSR, the browser sign-in, the 24-hour rule and the split between JSR and the GitHub
   release all go. One GitHub release would hold the program and the Pkl package.
6. **Version pinning becomes ours.** Today a project pins the CLI through a JSR version in `deno.json`, and pins the
   schema through `PklProject`; the CLI has a check that the two agree. One mechanism can replace both.
7. **Less to break under us.** Deno is a large runtime that changes quickly (the dependency-age rule is one example
   that reached our users). The CLI uses little of it, so we carry that churn for the benefit of a small API.

### What it costs, said plainly

- **A rewrite that adds no feature.** 8,300 lines of working, tested code are replaced by code that does the same.
- **The binary formats are where a rewrite can hurt.** A wrong byte in the MPQ archive, the map info or the object
  data is a map that does not load or a game that closes. These parts are well tested today.
- **We give up a good platform.** Deno supplies files, processes, watching, HTTPS, zip and hashing on three operating
  systems with no work from us, plus TypeScript's type checker, formatter, linter and test runner.
- **We start shipping executables.** That means a build for each operating system, a place to download them, a way
  to pin and update them, and on Windows the question of unsigned programs (§6.5).
- **12,400 lines of tests have to come along,** in some form (§7).

None of these is a reason to keep Deno. They are the reason to choose the path, and the order of the work, with care.

## 4. What any replacement must keep true

- **The same bytes.** A built map, `moonwell.lock`, the generated editor files and every command's output are the
  same as today's for the same project, except where a change is chosen on purpose.
- **Windows first.** The maintainer and the game are on Windows; CI also runs Ubuntu. macOS is untested today and
  can stay so, or be added.
- **One thing to install,** besides Pkl and the game, and no runtime behind it.
- **A project can pin the version** it is built with, and a mismatch is reported with the way to fix it.
- **Pkl stays.** The manifest, the schema package and `pkl eval` are untouched by all of this.
- **The tests keep their reach.** Whatever form they take, the behaviours they pin today stay pinned.
- **No JavaScript left:** no Deno, no Node.js, no `deno.json` in a project, in the CLI, the template, the wrappers'
  tools or CI.

## 5. The decisions, one axis at a time

The paths in §6 are combinations of these.

- **A. Where the logic lives.** In a compiled language, all of it; or in the Lua family, on a host.
- **B. The host,** if the logic is in the Lua family: the program that starts, loads the Lua code and supplies files,
  processes, HTTPS and the rest. Borrowed (`yue`, luvi) or our own (C++, or Rust with an embedded Lua).
- **C. The language of the logic,** if it is in the Lua family: annotated Lua checked by lua-language-server (what the
  two libraries are written in), Teal (typed, compiled by `tl`), or YueScript.
- **D. How the compilers run.** `tl.lua` and `fennel.lua` in process, or through an external interpreter. `yue` as
  the downloaded binary of today, or linked into our program.
- **E. How it reaches the user and how a project pins it** (§6.5).
- **F. How the port is made safe and what happens to the tests** (§7).

## 6. The paths

### Path 0: keep the TypeScript, hide Deno

`deno compile` turns today's CLI into one executable per operating system that carries the Deno runtime inside it.
Users would install no Deno and have no `deno.json`.

- **Gives:** reasons 1, 5 and 6 of §3 at almost no cost; nothing is rewritten.
- **Keeps:** the TypeScript, its tests, Deno as the tool we develop with. Reasons 2, 3, 4 and 7 remain.
- **Costs:** large executables (tens of megabytes; not measured), and the distribution work of §6.5, which every
  path needs anyway.
- **Verdict:** it does not do what was asked. It is here because it could be a **first stage** of any other path: it
  moves users off Deno at once and lets the rewrite happen behind an unchanged command line.

### Path 1: Lua on the `yue` binary we already ship

The CLI becomes Lua code that the pinned `yue` executable runs (`yue -e moonwell.lua build`). moonwell-systems' tools
work this way now.

- **Gives:** no new program at all. Lua 5.4 with integers and bitwise operators; the YueScript compiler in process;
  `tl.lua` and `fennel.lua` in process.
- **Lacks:** `yue`'s Lua has only the standard library. There is no way to list a folder, make one, read a file's
  time or size, watch for changes, download over HTTPS or handle Ctrl+C. Each has to go through `io.popen` and the
  operating system's own commands (`dir` and `cmd.exe` on Windows, `ls` and `sh` elsewhere, `curl` and `tar` on both),
  with two sets of quoting rules and a new process for every call. SHA-256 and inflate can be written in Lua.
- **Bootstrap:** something has to fetch `yue` first; that is a small install script.
- **Risk:** file handling through shell commands is the fragile kind of code: paths with spaces or non-ASCII letters,
  error messages in the user's language, slow process starts on Windows.
- **Verdict:** right for library tooling, where it is used. Too thin for the CLI, unless it is given libraries, which
  is Path 3 by another road.

### Path 2: Lua on luvi

luvi is a ready-made host from the Luvit project: a Lua engine with libuv (files, processes, file watching, sockets,
timers), miniz (zip), OpenSSL, LPeg and a regular-expression library. `luvi app -o moonwell.exe` glues a folder of Lua
code onto it and gives one executable. Prebuilt hosts exist for Windows 10+, Linux and macOS.

- **Gives:** everything of §2.2 from existing parts, with **no compiler in our build**: we only add Lua files to a
  prebuilt host. `tl.lua` and `fennel.lua` run on it.
- **Limits:** its usual engine is LuaJIT, which speaks Lua 5.1: no integer type, no `//`, no `&` and `|` operators
  (a `bit` library instead). The game is Lua 5.3, so code could not be shared with the libraries as written, and
  32-bit arithmetic needs care. luvi can also be built with standard Lua, but then we build it ourselves (whether
  prebuilt standard-Lua hosts are published was not checked). HTTPS comes from Luvit's own Lua packages, a small
  ecosystem. `yue` stays a separate download, as today.
- **Risk:** a small community behind the host; we depend on its releases for security fixes in OpenSSL and libuv.
- **Verdict:** the cheapest way to a real single-file Lua CLI. The price is the 5.1 dialect and a host we do not
  control.

### Path 3: our own host in C++, with `yue` inside

YueScript's own build already produces a program that contains Lua 5.4 and the compiler. We would extend that build:
add a file system and process library (libuv through `luv`, or smaller ones), miniz, a SHA-256 routine and a way to
download (the operating system's own HTTP, or the `curl` program that Windows 10+, macOS and Linux all ship), and
embed our Lua code, `tl.lua` and `fennel.lua` as data.

- **Gives:** one executable that contains all four compilers and the CLI. Nothing is downloaded at build time. Lua
  5.4 throughout. It is the most "Lua-native" result.
- **Changes:** the compiler versions are fixed by the Moonwell release. Today a project can pin its own `yue`
  version or point at its own binary; that would go, or stay as an escape hatch.
- **Costs:** we own a C and C++ build for three operating systems, its CI, and the upkeep of every C library in it.
  HTTPS is the hard part in C; leaning on `curl` avoids it.
- **Risk:** C and C++ are where memory bugs live, and none of the project's code is C++ today.
- **Verdict:** the best result for a Lua-only project, at the highest upkeep.

### Path 4: a thin Rust host with an embedded Lua, the logic in the Lua family

A small Rust program embeds Lua 5.4 through the `mlua` library (which can build Lua from source into the program, on
Windows, Linux and macOS) and gives it a handful of modules: files, processes, watching, HTTPS, zip, SHA-256, signals.
Rust has a mature library for each. All of Moonwell's own logic, from the MPQ writer to the commands, is Lua-family
code embedded in the program, as are `tl.lua` and `fennel.lua`.

- **Gives:** one executable; Lua 5.4; the hard platform parts from well-kept libraries in a memory-safe language;
  cross-platform release builds that are routine. The host is perhaps one or two thousand lines and changes rarely,
  so day-to-day work and contributions are in Lua.
- **`yue`:** either stays the pinned download of today (no risk), or its C++ source is compiled into the program
  beside the embedded Lua (one executable with all four compilers, as in Path 3). Whether that links cleanly is not
  known and is a spike (§9).
- **Costs:** a second language in the repository, though a small and stable part; the host's API is a mini-runtime
  that we design and have to keep small; errors cross a Rust and Lua boundary.
- **Verdict:** Path 3's result with far less upkeep, if the `yue` question resolves either way.

### Path 5: all of it in Rust

The whole CLI is rewritten in Rust. Lua is embedded only to run `tl.lua` and `fennel.lua`.

- **Gives:** one language for the tool; the strongest types and the best fit for binary formats; one executable; the
  richest library choice.
- **Costs:** the largest distance from today's code: TypeScript's records and garbage-collected objects do not carry
  over line by line, so this is a redesign as much as a port. Rust is the steepest of the languages here, for the
  maintainer and for contributors. And the logic again lives in a language that is not the project's, which was the
  second and third reason for leaving TypeScript.
- **Verdict:** the most robust tool and the least "Lua project".

### Path 6: all of it in Go

As Path 5, in Go. Go's standard library has zip, SHA-256, HTTPS and processes; watching is one well-known library.
Building for another operating system is a setting, with no C compiler. `tl.lua` and `fennel.lua` would run on a
Lua written in Go (a Lua 5.1 engine, slower than the real one), or simply through `yue -e`.

- **Gives:** the quickest road to a solid single executable; an easy language; trivial release builds.
- **Costs:** as Path 5 on the question of fit: the logic is not in the project's languages.
- **Verdict:** the pragmatic compiled choice. It solves the user's side completely and the project's side not at all.

### 6.1 Side by side

| | 0: hide Deno | 1: `yue` | 2: luvi | 3: C++ host | 4: Rust host + Lua | 5: Rust | 6: Go |
| --- | --- | --- | --- | --- | --- | --- | --- |
| User installs no runtime | yes | yes | yes | yes | yes | yes | yes |
| JavaScript gone from the project | no | yes | yes | yes | yes | yes | yes |
| Logic in the project's languages | no | yes | yes | yes | yes | no | no |
| Lua version of the tool | n/a | 5.4 | 5.1 (LuaJIT) | 5.4 | 5.4 | n/a | n/a |
| Teal and Fennel compile in process | no | yes | yes | yes | yes | yes (embedded) | slow or via `yue` |
| `yue` | download | is the host | download | built in | download or built in | download | download |
| Platform parts come from | Deno | shell commands | luvi | C libraries we add | Rust libraries | Rust libraries | Go standard library |
| We compile native code in CI | no | no | no | yes, C++ | yes, Rust | yes, Rust | yes, Go |
| Size of the rewrite | none | all, plus workarounds | all | all, plus the host | all, plus a small host | all, as a redesign | all |
| Main risk | not a removal | fragile file handling | 5.1 dialect, small host community | C++ upkeep | host API design; linking `yue` | steep language, poor fit | poor fit |

### 6.2 The language of the logic (Paths 1 to 4)

| | Annotated Lua + lua-language-server | Teal | YueScript |
| --- | --- | --- | --- |
| What the project knows | both libraries are written this way; the checks exist | new to the project; it is roadmap item 5.2 | the gameplay language; no typed code in it yet |
| Type checking | by the editor's server, run in CI; helpful, not strict | by the compiler; records, enums, integers apart from numbers | none |
| Build step | none | `tl` compiles to Lua, in process | `yue` compiles to Lua |
| Fit for 8,000 lines of byte handling | workable | best of the three | weakest |
| Side effect | none | Moonwell uses Teal support daily, which tests it hard | Moonwell uses its own flagship language |

### 6.3 How the compilers run

- **Teal and Fennel:** each compiler is one Lua file. Every path with a Lua inside runs them in process. A compiled
  CLI embeds a Lua for them or calls `yue -e`.
- **YueScript:** today's pinned download works on every path. Linking it in (Paths 3 and 4) removes the download and
  the per-project `yue` pin together.

### 6.4 Pkl

Unchanged on every path. The CLI runs `pkl eval --format json`, so the new CLI needs a JSON reader and nothing else.
One possible improvement, separate from this work: download a pinned `pkl` as `yue` is downloaded, so that Moonwell is
the only thing to install.

### 6.5 Distribution and pinning (every path, including Path 0)

Three shapes, from simplest:

1. **One `moonwell` on the PATH.** Installed by a script or a package manager (Scoop or winget on Windows, Homebrew
   elsewhere). The project's `PklProject` names the schema version; the CLI refuses a version that does not match
   and says how to get the right one. This is today's check without JSR.
2. **A launcher that follows the project's pin.** The installed `moonwell` is tiny; it reads the version the project
   names, downloads that release once into the cache and runs it. Old projects keep building with their version.
   This is how Rust's and Bazel's launchers work, and it is closest to what `deno.json` gives today.
3. **A program inside the project,** fetched by a small script that is committed with it. Nothing is installed
   machine-wide.

To settle: on Windows, a program downloaded through a browser and not signed draws a SmartScreen warning. A download
made by a script or a package manager does not carry that mark, as far as I know; this needs checking before the shape
is chosen. Deno itself is signed, so this is new to us.

Whichever shape: a release becomes one GitHub release with the executables and the Pkl package, built by CI from a
tag.

## 7. How to make the port safe

The current CLI is not only what gets replaced. It is the **reference**: a program that gives the right answer for any
input. That allows a port that never has to trust itself.

- **Compare, part by part.** For each ported part, feed the old and the new the same input and compare the bytes: the
  packed archive, the patched `war3map.w3i`, each object file, the bundled `war3map.lua`, `moonwell.lock`, the
  generated editor files, the rewritten pictures. The fixtures (four folders, 56 KB) are plain files and serve both.
- **Compare, end to end.** Run both CLIs on the template, the gate map and the test projects, and compare the staged
  map folder and the packed map. When they are equal everywhere, the old CLI can go.
- **The tests.** The 81 test files are the written record of every behaviour and every pitfall paid for. Three
  options: port them all (the most work, the least loss); port the unit tests of the byte-level parts and replace the
  rest by the comparisons above while both CLIs exist, then keep the end-to-end ones; or keep running the TypeScript
  tests against the new program through its command line where they can. The first is the safe default.
- **No features meanwhile.** The maintainer's order already puts this before the other languages, so the CLI holds
  still while it is ported.
- **Order inside the port:** the leaves first (paths, hashing, zip, MPQ, map info, pictures, object data), then the
  planners, then the commands, `dev` last. Each leaf is checked against the reference before anything is built on it.
- **The wrappers' tools** (357 lines) become Lua scripts like moonwell-systems', early: it is small, and it is the
  model for how tests run without Deno.

Three ways to stage it:

1. **Straight:** build the new CLI beside the old, switch when the comparisons are equal, delete the TypeScript.
2. **Distribution first:** Path 0 as stage one (users leave Deno now), then the rewrite behind the same command.
3. **By command:** the new program takes over one command at a time and hands the others to the old. It keeps both
   alive in every user's install for a while, which is the opposite of the goal; I would not.

## 8. What stays the same on every path

The Pkl schema and package; the manifest; the map formats and their bytes; the Lua runtime in maps; `moonwell.lock`;
the editor files and extensions; the libraries and their tags; the in-game gates. For a map author the change is the
install line and `deno task build` becoming `moonwell build`.

## 9. My lean, and what would settle it

**Path 4**, with the logic in the Lua family, staged "straight" or "distribution first", and ported behind the
comparisons of §7. The reasons:

- It is the only path besides Path 3 that removes JavaScript, puts the logic in the project's own languages **and**
  keeps Lua 5.4, the dialect nearest the game's.
- It gets files, HTTPS, zip, hashing and watching from maintained libraries in a memory-safe language, where Path 3
  gets them from C and Path 1 from shell commands.
- The native part is small and rarely touched, so the second language costs little; in Paths 5 and 6 it is the whole
  tool.
- If linking `yue` in turns out hard, nothing is lost: the pinned download of today keeps working.

For the language of the logic I lean to **Teal**, less firmly: a tool where one wrong byte closes the game deserves
the strictest checker of the three, and using Teal daily would prove the Teal support of item 5.2. The counterweight
is that the project's habit and its existing checks are annotated Lua. A side-by-side port of one part would decide.

If the upkeep of any native code is unwanted, **Path 2 (luvi)** is the fallback: no compiler anywhere, at the price of
the Lua 5.1 dialect.

Six spikes, each small and thrown away afterwards, would turn the open questions into facts:

1. **A thin host:** Rust with `mlua` and Lua 5.4, built in CI for Windows and Linux, that runs embedded Lua, lists a
   folder, runs `pkl`, downloads a file over HTTPS, hashes and unzips it. It gives the executable's size and start
   time.
2. **Teal and Fennel inside it:** load `tl.lua` and `fennel.lua` and compile a sample for Lua 5.3.
3. **`yue` inside it:** try compiling YueScript's source into the same program; or decide to keep the download.
4. **One real part, ported twice:** the MPQ writer with its encryption, in annotated Lua and in Teal, with bytes
   equal to today's for the template map. It measures the port's pace and decides §6.2.
5. **`deno compile` of today's CLI:** size, start time, and whether `pkl` and `yue` run from it. It decides whether
   "distribution first" is worth it.
6. **luvi:** whether a standard-Lua host is published, and an HTTPS download from a bundled program on Windows. It
   keeps the fallback honest.

## 10. What this report did not verify

- Checked today against the projects' own pages: luvi's engines, parts and platforms; that `tl.lua` is one file with
  no dependencies and runs on Lua 5.1 to 5.4; that YueScript builds as a Lua module, a binary and a C++ library;
  `mlua`'s Lua versions, vendored build and Windows support.
- Checked today on this machine: what the pinned `yue` binary's Lua has and lacks.
- From memory, not checked: that `fennel.lua` is one dependency-free file; the size of a `deno compile` executable;
  the Go Lua engine's version and speed; the Windows SmartScreen behaviour of §6.5; whether luvi publishes prebuilt
  standard-Lua hosts.
- Not estimated: time. The spikes of §9 are what would make an estimate honest.
