# Moonwell Library Assets (Moonwell 0.6.0) — Design

- **Date:** 2026-10-01
- **Status:** The design was approved in chat on 2026-10-01; this written spec awaits review.
- **Builds on:** `2026-09-28-moonwell-lua-libraries-design.md` (§4 libraries: the manifest, fetching, `moonwell.lock`)
  and the assets part of `2026-09-24-moonwell-core-design.md` (§6.2). Everything in them still applies unless this spec
  changes it explicitly.
- **Roadmap:** phase 4, item 2, and the backlog entry "Assets shipped by libraries".
- **Target:** Moonwell 0.6.0 (`@moonwell/cli` and the Pkl package `moonwell@0.6.0`).

## 1. Summary and scope

A library can ship files for the map: models, icons, sounds, frame template `.toc` and `.fdf` files. They join the
map's asset import, so a map that lists the library gets them without copying anything.

The maintainer chose (2026-10-01):

- **Files only.** Object data stays in the map. The dummy unit of `systems.dummy` remains a Pkl block from that
  library's README, as chosen on 2026-09-30; a library does not ship objects.
- **The library says what it ships**, in a file of its own. The map names the repository and the tag.
- **The map's own asset wins** over a library's at the same in-map path, with a message. Two libraries at one path is
  an error.
- **No new in-game gate run** (§10).

No library ships files today; this is the mechanism for the first one that does.

## 2. The library's file

`moonwell-library.json`, at the library's root: the repository's root for a GitHub library, the `path` folder for a
local one.

```json
{
  "dir": "src",
  "assets": "assets"
}
```

- **`dir`** (optional): the folder module names start from. The same meaning as `dir` in the map's manifest.
- **`assets`** (optional): the folder whose files the map imports.
- Both are relative folder paths inside the library, with `/`: no empty, `.` or `..` segment, no `\`, no drive.
- The file must be a JSON object. An unknown key is an error naming the library, the file and the key, with the hint
  that the library may need a newer Moonwell. A value that is not such a path is an error too.
- A library without the file ships no assets and behaves exactly as today.

**Decision:** JSON, not Pkl: the file is read from a download before anything of the library is trusted, and reading
JSON runs nothing.

## 3. The map's manifest

No schema change beyond `dir`'s documentation.

- When the manifest's `dir` is set (not empty), it is the module folder, whatever the library's file says. Every
  existing manifest keeps working.
- When it is empty (the default), the library's file decides; without a file, or without `dir` in it, it is the
  library's root, as today.
- `assets` comes only from the library's file.

So a map can write just:

```pkl
libraries {
  ["example"] { github = "mdlsvensson/moonwell-example-lib"; tag = "v0.2.0" }
}
```

## 4. Fetching and storing

### 4.1 What is kept

- **Modules**, as today, in `.moonwell/libraries/<key>/`: the files under the module folder, except those in the
  assets folder when it lies inside the module folder (a library whose modules start at its root).
- **Assets**, in `.moonwell/library-assets/<key>/`: every file under the assets folder, relative to it, except files
  in a folder, or with a name, that starts with `.`. Module scanning never looks there.
- A declared assets folder with no file at that tag is an error ("Library <key> has no folder <assets> at <tag>"),
  like a `dir` that does not exist.
- A folder in `.moonwell/library-assets/` whose key is no longer in the manifest, or whose library no longer ships
  assets, is removed.

### 4.2 GitHub libraries

The download is as today. The file is read from the archive's root, then the two sets are written: the assets folder
first, then the modules folder with its stamp, each through a temporary folder that replaces the old one. The stamp is
written last, so an interrupted sync is repeated.

The stamp (`.moonwell-library.json` in the modules folder) gains `"layout": 2`. A folder whose stamp has no such value
is downloaded again. So after upgrading Moonwell every library is fetched once more, and a library that already ships
assets gets them. A stamp that matches its lock entry is also not enough when the entry has assets and
`.moonwell/library-assets/<key>/` is missing: the library is downloaded again.

### 4.3 Local libraries

`path` is the library's root. The file is read from `<path>/moonwell-library.json`. Modules are copied from
`<path>/<module folder>` as today (`.yue` and `.lua` files); assets are copied from `<path>/<assets>` (every file
outside dot-names), writing only changed files and removing files that disappeared. `dev` watches the assets folder
and the file too, with the folders it picks when it starts.

A local library whose `path` already points at its module folder finds no file there and ships no assets, as before.

### 4.4 `moonwell.lock`

An entry gains `assets`, only when the library ships assets:

```json
"example": {
  "github": "mdlsvensson/moonwell-example-lib",
  "tag": "v0.2.0",
  "dir": "",
  "commit": "…",
  "files": "sha256:…",
  "assets": "sha256:…"
}
```

- `dir` stays the manifest's value, and `files` the hash of the kept module files, computed as today. A lock written
  by Moonwell 0.5 stays valid.
- `assets` is the same kind of hash over the kept asset files.
- "The tag moved" is decided as today, by `commit` and `files`, and also by `assets` when the entry has it. An entry
  without `assets` whose download ships assets is compared by `commit` alone and gains the field without an error:
  that is the upgrade from 0.5, whose `files` hash may have counted files that are assets now (a library whose modules
  start at its root).

## 5. Importing

### 5.1 Collection

The assets of a build are the map's own (`assets/`, with the manifest's `paths` and `exclude`, as today) and then each
library's, in key order:

- A library file's in-map path is its path under the library's assets folder: `assets/Models/Golem.mdx` in the library
  becomes `Models\Golem.mdx`. The manifest's `paths` and `exclude` apply to the map's own files only.
- The same checks apply as to the map's files: a path that is unsafe, that names a map internal such as
  `war3map.lua`, that differs from another only in letter case, or that sits inside another asset file is an error,
  naming the library.

### 5.2 Clashes

- **The map's file wins.** When the map's own asset and a library's have the same in-map path, the library's is left
  out and a line says so: `assets/<source> replaces library <key>'s <path>`. That is how a map swaps a library's icon
  or model.
- **Two libraries** with the same in-map path are an error naming both libraries and the path.
- A library file that clashes with a file World Editor already has in the map is the existing error ("conflicts with a
  file or import already in the map"), naming the library.

### 5.3 Where they go

Everywhere the map's assets go, through the one list:

| Command | With library assets |
| --- | --- |
| `build`, `test`, `dev` | Imported into the staged map and listed in its `war3map.imp`. |
| `check` | Counted in "… asset(s)", and planned against the source map like the map's own. |
| `assets:check`, `assets:sync` | Listed as `library <key>: <path> -> <in-map path>`; `assets:sync` writes them into the source map and owns them in `.asset-state/`, so an upgraded or removed library's files are replaced or deleted by the next sync. |
| `assets:paths` | Count as imported paths; without a file argument, a library's models are checked too, under the heading `library <key>: <path>`. |

`assets:check`, `assets:sync` and `assets:paths` (inside a project) sync the libraries first, as `check` and `setup`
do. Today they do not touch libraries.

## 6. Errors

All are `MoonwellError`s with a file and a hint. New ones:

| Situation | Message names |
| --- | --- |
| `moonwell-library.json` is not a JSON object | the library, the file |
| an unknown key | the library, the key; hint: it may need a newer Moonwell |
| `dir` or `assets` is not a relative folder path | the library, the key and the value |
| the assets folder has no file at the tag, or is not a folder (local) | the library, the folder, the tag or path |
| two libraries import one path | both libraries, the path; hint: one of them has to be dropped or the clash reported to its author |
| a library asset's path is reserved or unsafe | the library and the path |

## 7. Code

- `cli/src/libraries/manifest.ts` (new): reads and validates `moonwell-library.json` from bytes.
- `cli/src/libraries/sync.ts`: the effective module folder, the assets set, `.moonwell/library-assets/`, the stamp's
  `layout`, the local copy.
- `cli/src/libraries/lock.ts`: the optional `assets` field.
- `cli/src/assets/collect.ts`: library assets, the override and the clash; `Asset` gains `library?: string`.
- `cli/src/assets/plan.ts`, `cli/src/pipeline.ts`, `cli/src/commands/{assets,assets-paths,check,dev}.ts`: pass the
  libraries through, sync before planning, the new lines.
- `schema/Project.pkl`: `dir`'s documentation.

## 8. Documentation

- README "Libraries": what a library can ship, `moonwell-library.json`, that `dir` may be left out, and the advice to
  libraries to keep their files under a folder of their own, such as `war3mapImported/<library>/`, so they do not
  clash with a map's or another library's.
- README "Assets": library assets join the import; the map's file wins.
- CHANGELOG `0.6.0`; AGENTS and the roadmap.
- The example library `mdlsvensson/moonwell-example-lib` gets a tag `v0.2.0` with the file and one small asset; its
  `v0.1.0` tag is not touched.

## 9. Testing

- **Unit:** the file's parsing (each error of §6); sync from an archive and from a local folder (modules without the
  assets folder, the assets set, dot-names skipped, a missing assets folder, removal when a library stops shipping
  assets); the stamp's `layout` forcing one download; the lock's `assets` (written, read, absent for a library without
  assets, the 0.5 upgrade without a "moved" error, a changed asset reported as moved); collection (order, the map's
  file winning with its line, two libraries clashing, a reserved path); the commands' lines.
- **End to end:** a project with a local library that ships a file: the staged map holds it and its `war3map.imp`
  lists it; a map asset at the same path replaces it; `assets:sync` writes it into the source map and removes it when
  the library is dropped.
- **Network** (`MOONWELL_NETWORK_TESTS=1`): the example library's `v0.2.0` downloads with its asset and locks an
  `assets` hash; `v0.1.0` still locks to the entry it always had.

## 10. Release gate

Steps 1 and 2 of CONTRIBUTING (every check, the generated data), the JSR and Pkl package checks, and the maintainer's
JSR `init` check. No in-game run: the change decides which files enter the import path that the assets gate (step 7)
already covered in the game and in World Editor, and the end-to-end test checks the staged map's bytes.

## 11. Plan phasing

One plan, `docs/superpowers/plans/2026-10-01-moonwell-library-assets.md`, of test-first tasks:

1. `moonwell-library.json`: reading and validating.
2. Sync: the module folder from the file, the assets set, the stamp, local libraries.
3. The lock's `assets`.
4. Collection: library assets, the override and the clash.
5. The commands: `build`, `check`, `assets:check`, `assets:sync`, `assets:paths`, `dev`.
6. The example library's `v0.2.0` and the network test.
7. Docs.
8. Release 0.6.0.

## 12. Departures found while planning

The plan's code was built and run in a scratch clone first (2026-10-01). It found:

- **The upgrade from a 0.5 lock compares the commit alone** when the download ships assets (§4.4, amended above).
- **The plan's order** puts the lock (task 3 here) before the sync (task 2), which writes the new field.
- **`assets:check`, `assets:sync` and `assets:paths` are tested in-process with real Pkl** (`cli/tests/pkl/`), beside
  one end-to-end build test, instead of all end to end.
- **Errors about a library's file name it as a URL** for a GitHub library
  (`https://github.com/<owner>/<repo>/blob/<tag>/moonwell-library.json`) and as its path for a local one.
- **The template's commented library example** now names the example library's `v0.2.0` without `dir`. That tag was
  pushed on 2026-10-01 (commit `0b69cfa`) so the network test could be written against it.

## 13. Out of scope

- **Object data shipped by libraries** (the maintainer's choice, §1).
- **Mapping or excluding a library's files from the map's manifest:** a map overrides a file by shipping its own at
  the same path.
- **Dependencies between libraries**, private repositories and plain URLs, as before.
- **`moonwell-library.json` in the wrappers and systems libraries** (`{"dir": "src"}`, so maps can drop `dir`): their
  next releases.
