# Moonwell Lua Modules and Libraries (sub-project 4a and 4b) — Design

- **Date:** 2026-09-28
- **Status:** Sections 1–4 approved by the maintainer in chat on 2026-09-28; this written spec awaits review
- **Builds on:** `2026-09-24-moonwell-core-design.md` (§1 roadmap item 4, §5 bundler);
  `2026-09-27-moonwell-editor-dx-design.md` (`.moonwell/`, `.luarc.json`, the unknown-global check, macros)
- **Scope:** Plain Lua modules in a project (4a) and libraries fetched from GitHub tags or local folders (4b). Two
  plans (4a, 4b), one release (0.5.0). Typed native wrappers (4c) and a YueScript port of `wc3-lib` (4d) are on the
  backlog, as are TypeScript and C# support; they build on this.

## 1. Summary

Today the bundler takes only `src/**/*.yue`. After this sub-project a map can also use:

1. **Lua modules** in a committed `lua/` folder: an author's own Lua, or a Lua library copied in by hand.
2. **Libraries** listed in `moonwell.pkl`, fetched from a GitHub tag (locked by commit in `moonwell.lock`) or read
   from a local folder, containing YueScript and/or Lua modules.

All three kinds of module share one namespace and are reached the same way, with `import`/`require`. Only reachable
modules are bundled, as before.

Success means: a YueScript file imports a Lua module from `lua/` and a module from a GitHub library, `check` passes,
the editor completes both, the map runs them in the game, and a fresh clone with `moonwell.lock` fetches the same
library commit.

Decisions are marked **Decision:** with a one-line rationale. Claims not yet confirmed are marked **To verify** and
collected in §9; the first task of Plan 4a checks them.

## 2. Verified facts this design relies on

Checked on 2026-09-28:

- **The YueScript extension writes its Lua output next to the source.** `writeLuaBuildFile` in
  `pigpigyyy/yuescript-vscode` `src/extension.ts` writes to `getLuaPath(document.uri.fsPath)`, the `.yue` path with a
  `.lua` extension; there is no setting for another location. So `src/**/*.lua` stays git-ignored editor output, and a
  project's own Lua cannot live in `src/`.
- **GitHub tag archives.** `https://codeload.github.com/<owner>/<repo>/zip/refs/tags/<tag>` (tag URL-encoded; `@` as
  `%40`) and `https://github.com/<owner>/<repo>/archive/refs/tags/<tag>.zip` return the same bytes (HTTP 200). The zip
  has one top folder (`moonwell-moonwell-0.4.0/` for repository `moonwell`, tag `moonwell@0.4.0`), and its zip comment
  is the tag's 40-character commit SHA (`27cd6ecf70f565375755e52ff9d8efab7a7d65fc`). A missing tag returns HTTP 404.
- **`cli/src/yue/unzip.ts`** extracts stored and deflated entries of non-ZIP64 archives, which is what GitHub serves.
- **The bundler's Lua lexer** (`cli/src/bundle/lexer.ts`) already finds `require("x")`/`require "x"` in any Lua,
  skipping comments and strings; `resolveGraph` follows them.

## 3. Lua modules (4a)

### 3.1 Where and how named

Lua modules live in `lua/**/*.lua`, committed. A module is named by its path under `lua/` with `/` as `.`:
`lua/utils/timer.lua` is `utils.timer`. Folder and file names cannot contain dots (the rule `src/` already has).

**Decision:** a separate `lua/` folder (maintainer's choice, option A), because `src/**/*.lua` is the editor's output.

### 3.2 One namespace

`src/**/*.yue`, `lua/**/*.lua` and every library's modules (§4) are one namespace. `import "x"` / `require "x"`
resolves `x`, then `x.init` (Lua's `?/init.lua` convention, which existing Lua libraries rely on), for every kind of
module. A name defined by two files fails the command before anything compiles, naming both files.

### 3.3 Bundling

A Lua module goes into the bundle as its source, unchanged, wrapped in `__mw.define(name, function(...) ... end)` like
a compiled YueScript module; its `require` calls are followed the same way. Runtime errors inside it name
`lua/<path>.lua` and the line, through the same line table. With `--minify`, Lua modules are not minified (the
compiler cannot minify plain Lua) and keep their line mapping; YueScript modules are minified as today.

Top-level `local` variables of a Lua module stay local to it (the wrapper is a function); assignments without
`local` create real globals, so "global-style" Lua libraries work once something requires them.

### 3.4 Unknown globals

Amended 2026-09-28 during Plan 4b: the check covers only the `src/` modules reachable from the entry (the map's entry,
or `--entry`, and every module reached from it through `import`/`require`), and only those reachable modules' `global`
lines and top-level Lua globals are known names. A file nothing imports is not checked, and its globals do not count.

The unknown-global check (editor-dx spec §5) still checks only the project's `src/**/*.yue`. `.lua` files are not
checked (`yue -g` reads only YueScript). The globals a Lua module defines at its top level become known names:

- `function Name(` (not `function Name.x(` or `Name:x(`), and
- a statement `Name = ...` or `Name, Other = ...` without `local`,

at block depth 0, outside any function, `do`, `if`, loop or table constructor. A name the file declares at its top
level with `local a, b`, `local a = ...` or `local function a` is left out, so `local Timer` followed by `Timer = {}` is
not a global. Globals a Lua file assigns only inside functions are not seen; they belong in `lint.globals`, as the
README says.

### 3.5 Editor

The template's `.luarc.json` gains `lua/?.lua` and `lua/?/init.lua` in `runtime.path`, so lua-language-server resolves
`import "utils.timer"` to the workspace file and completes its fields and globals (**To verify**, V1).

`setup` adds missing entries to an existing project's `.luarc.json` arrays (`runtime.path`, `workspace.library`, and
`workspace.ignoreDir` since §5.4's amendment), keeping every other key and value. **Decision:** this is the only file
`setup` edits rather than creates, because projects from 0.4 would otherwise not resolve `lua/`; it only inserts missing
array entries and rewrites the file as formatted JSON.

`init` creates `lua/` with a `.gitkeep`, so the folder is visible in a new project.

## 4. Libraries (4b)

### 4.1 Manifest

```pkl
libraries {
  ["wc3-lib"] { github = "mdlsvensson/wc3-lib-yue"; tag = "v0.1.0"; dir = "src" }
  ["init"] { github = "someone/TotalInitialization"; tag = "v5.3" }
}
```

New class `Library` in `schema/Project.pkl` and `libraries: Mapping<String, Library> = new {}`:

- the key: a folder name, `[A-Za-z0-9_-]+`;
- `github`: `owner/repo` (`[A-Za-z0-9-]+/[A-Za-z0-9._-]+`), nullable;
- `tag`: a non-empty string, nullable;
- `path`: a non-empty path, nullable; a local folder, relative to the project root or absolute;
- `dir`: the folder inside the library that module names start from; relative, no `..`; default `""`, the root.

When `path` is set the library is local and `github`/`tag` are ignored. Otherwise `github` and `tag` are both
required; `parseProject` reports a library with neither, or with only one of them, naming the evaluated manifest and
the key. **Decision:** `path` wins, so `moonwell.local.pkl` can point a library at a checkout
(`libraries { ["wc3-lib"] { path = "../wc3-lib-yue" } }`) while `moonwell.pkl` keeps naming the release, like
`yue.path`.

The template's `moonwell.pkl` shows an empty `libraries {}` block with a commented example.

### 4.2 Fetching

`check`, `build`, `test` and `dev` bring `.moonwell/libraries/<key>/` up to date at the start of every compile
(`compileProject`, after the objects are planned), and `setup` does after adding the editor files (amended 2026-09-28
during Plan 4b):

- **GitHub:** download `codeload.github.com/<owner>/<repo>/zip/refs/tags/<tag>`, read the commit from the zip comment,
  strip the archive's single top folder whatever its name, keep the files under `dir`, and write them to a temporary
  folder that replaces `.moonwell/libraries/<key>/` only when complete. A stamp file in it (`.moonwell-library.json`) records the lock entry
  it holds; a library whose stamp equals its lock entry is not downloaded again.
- **Local `path`:** the files under `<path>/<dir>` are copied into `.moonwell/libraries/<key>/`, writing only changed
  files and removing files that disappeared; no lock entry.
- Amended 2026-09-28 during Plan 4b: neither kind keeps a file inside a folder whose name starts with `.` (such as
  `.git/` or `.github/`), and a local library copies only its `.yue` and `.lua` files. `files` in the lock hashes the
  files kept. `dev` ignores changes under a local library's dot-folders.
- A folder in `.moonwell/libraries/` whose key is no longer in the manifest is removed.

**Decision:** automatic, like the pinned `yue` download (maintainer's choice, approach 1), so a clone needs no extra
step; the lock keeps it reproducible. Private repositories and plain URLs are out of scope.

### 4.3 `moonwell.lock`

A committed JSON file at the project root:

```json
{
  "libraries": {
    "wc3-lib": {
      "github": "mdlsvensson/wc3-lib-yue",
      "tag": "v0.1.0",
      "dir": "src",
      "commit": "27cd6ecf70f565375755e52ff9d8efab7a7d65fc",
      "files": "sha256:<hex>"
    }
  }
}
```

`files` is the SHA-256 of, for each kept file sorted by POSIX path, `<path>\n<sha256 of its bytes>\n`.

- A GitHub library with no lock entry, or whose `github`, `tag` or `dir` differ from its entry: download it and write
  the entry. This is the upgrade path, and the change shows in `git diff`.
- A library whose `github`, `tag` and `dir` match its entry but whose download has another `commit` or `files`: the
  command fails, naming the library, the tag and both commits ("the tag moved"), with the hint to delete the library's
  entry from `moonwell.lock` if the move was intended.
- Entries for libraries no longer in the manifest are removed. A library that is now local keeps its entry: its `path`
  usually comes from the uncommitted `moonwell.local.pkl`, so it must not change the committed lock, and switching back
  to the tag checks the download against the entry. (Amended 2026-09-28 during Plan 4b.)
- The file is written only when its content changes, sorted by key, with two-space indentation. It is removed when no
  GitHub library remains.

### 4.4 Library modules

A library's modules are the `.yue` and `.lua` files under its folder, named by their path there (§3.1 rules). They
join the one namespace (§3.2) under their own names: a library's internal `require "core.x"` keeps working. A `.lua`
file beside a `.yue` file of the same stem (`core/x.lua` next to `core/x.yue`) is that module's compiled output, as the
editor writes it on save, and is skipped (amended 2026-09-28 during Plan 4b).
**Decision:** no prefix by key (maintainer's choice, option A), because rewriting a library's own requires is fragile.

Libraries cannot declare dependencies on other libraries: the map lists every library it uses. A library module that
requires a module nothing provides fails with the usual "Module not found" error at that library file and line.

## 5. Pipeline

### 5.1 Collecting modules

A new step lists every module: name, project-relative POSIX path (`src/main.yue`, `lua/utils/timer.lua`,
`.moonwell/libraries/wc3-lib/core/scheduler.yue`), kind (`yue` or `lua`) and origin (the project or a library key).
It reports clashes (§3.2) and dotted names (§3.1) before any compile.

### 5.2 Compiling and resolving

Every `.yue` module, the project's and the libraries', is compiled by the existing step, with its cache, `--path` for
macros and the same options. Project output stays at `dist/stage/lua/<path under src>.lua`; library output goes to
`dist/stage/lua/.libraries/<key>/<path>.lua`. `.lua` modules are not compiled: their source is read where it is.
Resolving a name (§3.2) looks it up in the collected list. The graph, reachability and bundle layout are unchanged.

### 5.3 Unknown globals

Amended 2026-09-28 during Plan 4b: known names come from the modules reachable from the entry, not from every module.

The check (editor-dx spec §5) runs `yue -g` on the `src/**/*.yue` modules reachable from the entry only. Known names
add: `global` lines of every reachable `.yue` module, the project's and the libraries', and top-level globals (§3.4) of
every reachable `.lua` module, the project's and the libraries'.

### 5.4 Editor view of libraries

lua-language-server reads Lua, and each library sits under its own key. After compiling, Moonwell writes
`.moonwell/lua/` (git-ignored, under `.moonwell/`): every library module as `<module path>.lua`, a Lua module copied
and a YueScript module as its compiled Lua. `.luarc.json`'s `workspace.library` gains `.moonwell/lua`, so
`import "core.scheduler"` completes (**To verify**, V2). `workspace.ignoreDir` gains `.moonwell/libraries`: without
it, lua-language-server diagnoses the library copies as workspace files (amended 2026-09-28 during Plan 4b). Files
are written only when their content differs; files of modules that disappeared are removed. `setup`, which does not
compile, writes the Lua modules only, and keeps the files of YueScript modules as the last compile wrote them (amended
2026-09-28 during Plan 4b).

### 5.5 `dev`

`dev` also watches `lua/` and the folder of each local `path` library, and re-runs `check` on a change there.

## 6. Error handling

Every expected failure is a `MoonwellError` with the file and a hint:

- **Clash:** `Module utils.timer is defined by lua/utils/timer.lua and .moonwell/libraries/x/utils/timer.yue.`; hint:
  rename one, or narrow the library's `dir`.
- **Download:** network failure, HTTP 404 (tag or repository not found) or an unreadable archive, each naming the
  library key and the GitHub URL, with a hint to check the repository and tag there.
- **Moved tag:** §4.3.
- **Local library:** a `path` or `path/dir` that is not a folder names the manifest and the key.
- **Lock:** an unreadable or malformed `moonwell.lock` names it, with the hint to fix or delete it.
- **Writing `.moonwell/libraries/` or `.moonwell/lua/`:** names the folder, with the hint to close programs using it
  and retry.
- **Manifest:** a library with only one of `github` and `tag` (§4.1).

## 7. Testing

- **Unit** (`deno task test`, no network, no yue): collecting modules (names, `init`, clashes, dotted names); the Lua
  top-level global scan (functions, assignments, lists, `local`, nested blocks and tables, strings and comments); the
  lock rules (new, changed tag or `dir`, moved tag, removed, local); zip handling (top folder, `dir`, commit from the
  comment) on small fixture zips; downloads through a stand-in fetcher (404, network error); `.luarc.json` merging;
  the manifest's `libraries` parsing.
- **Pkl** (`deno task test:pkl`): `libraries` defaults, key and `github` patterns, `dir` constraints.
- **Runtime** (real yue): a project with a Lua module and a library `.yue` module compiles, bundles and runs under
  `yue -e`, and a runtime error in each maps to the right file and line, in normal and `--minify` builds.
- **E2E** (real yue and Pkl): `init` creates `lua/`; a project using a `lua/` module and a local `path` library passes
  `check` and `build`; a clash fails `check`; a YueScript file using a global a `lua/` module defines passes the
  unknown-global check; `.moonwell/lua/` holds the library's modules.
- **Network** (only when `MOONWELL_NETWORK_TESTS=1`; CI sets it): fetching the §10 test library by tag writes
  `moonwell.lock` with its commit, and a second run downloads nothing.

## 8. Release gate

A new CONTRIBUTING step, in a throwaway `init --link` project on Warcraft III Reforged 3.0.0.24268:

1. Add `lua/greeter.lua` returning a table with a function that prints, and a global-style `lua/counter.lua` that
   defines a global function. Use both from `src/main.yue`. The editor completes the module's function and the global;
   `deno task check` passes; in the game both print.
2. Add the §10 library by GitHub tag, run `deno task check`, and commit `moonwell.lock`. Use a library function in
   `src/main.yue`; it completes in the editor and runs in the game. Delete `.moonwell/`, run `deno task check`, and
   confirm the same commit is fetched and `moonwell.lock` is unchanged.
3. Point the library at a local checkout with `path` in `moonwell.local.pkl`, change a function there, and confirm
   `deno task test` runs the change.

## 9. To verify (Plan 4a's first task)

1. **V1:** lua-language-server, with `lua/?.lua` in `runtime.path`, resolves `import "utils.timer"` in a `.yue` file
   (through the extension's `.lua` output) to `lua/utils/timer.lua`, and completes a global a `lua/` file defines at
   top level.
2. **V2:** it resolves `require "core.scheduler"` to `.moonwell/lua/core/scheduler.lua` when `.moonwell/lua` is in
   `workspace.library`.
3. **V3:** in the game, a global-style Lua library bundled by Moonwell and required from `main.yue` defines its globals
   and runs.
4. **V4:** a lightweight and an annotated tag both give an archive whose zip comment is the commit SHA.

## 10. Inputs needed from the maintainer

- A public GitHub repository with at least one tag, holding a small library (a `.lua` and a `.yue` module), for the
  network test and the gate: `mdlsvensson/moonwell-example-lib`, tag `v0.1.0` (commit
  `c07126f080c3887ba667596d08aa21df3b3a20f7`), with its modules under `src/`.
- One run of V1–V3 in the editor and the game during Plan 4a, and the release gate (§8).

## 11. Implementation order

1. **Plan 4a — Lua modules.** Starts with V1–V4; then module collection and resolution, bundling Lua, the top-level
   global scan, `.luarc.json` and `setup` merging, `init`'s `lua/`, README. Ends with gate step 1.
2. **Plan 4b — libraries.** The `libraries` schema and parsing, fetching and the lock, library modules in the
   pipeline, `.moonwell/lua/`, `dev` watching, README. Ends with gate steps 2 and 3.

**Decision:** one release, 0.5.0, after Plan 4b, because the gate covers both and 4b builds on 4a's module list.

## 12. Out of scope

- Typed native wrappers (4c) and the YueScript port of `wc3-lib` (4d); TypeScript and C# support (backlog).
- Private repositories, plain URLs, JSR packages, and dependencies between libraries.
- Checking Lua files for unknown globals, or minifying plain Lua.
