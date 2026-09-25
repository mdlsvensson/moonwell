# Moonwell In-Game Paths — Design

- **Date:** 2026-09-25
- **Status:** Approved in conversation, pending spec review
- **Builds on:** `2026-09-25-moonwell-model-paths-design.md` (the `assets:paths` command), which this changes

## 1. Summary

`assets:paths` currently marks every reference that the project does not import as "not found". Most of those are files
that ship with the game, so a missing custom texture is buried among them. This change teaches Moonwell which paths the
game ships, and describes each reference path in the terms the modding community uses:

| Status | Meaning |
| --- | --- |
| `in-game path` | The game ships a file at this path |
| `in-game path, replaced` | The game ships this path, and the project imports a file there that overrides it |
| `custom path, imported` | Not an in-game path; the project imports it |
| `custom path, not imported` | Not an in-game path, and the project does not import it: the model will be missing this file |

Outside a Moonwell project, only `in-game path` or `custom path` is shown.

Paths are displayed the way World Editor's Import Manager shows them, with `\`, whatever separator the model stores.

The core design's hard constraints still apply: no Node.js and no npm packages; expected failures are `MoonwellError`;
async file I/O.

## 2. Output

```
assets/MovingGrass_DE.mdx
  texture  Doodads\LordaeronSummer\Plants\Corn\plant1_Grasspatch_Diffuse.tif  in-game path
  texture  Textures\Black32.blp                                               in-game path
  texture  team colour (slot 1)
  texture  Textures\MyMoss.blp                                                custom path, not imported
1 model, 4 paths: 2 in-game, 0 custom imported, 1 custom not imported.
```

- In a project, the summary counts in-game paths (replaced or not), `custom imported` and `custom not imported`. Outside
  a project, it reads `1 model, 4 paths: 2 in-game, 1 custom.`
- Replaceable textures have no path, no status, and are not counted in the parts of the summary (they are in the path
  count, as before).
- When Moonwell's in-game path list is empty (it has not been generated yet), the command prints the warning
  `Moonwell's in-game path list is empty, so every path shows as custom.` before the report.

## 3. Matching

Two comparisons, each case-insensitive and treating `/` and `\` alike:

- **Against imports** (unchanged): a reference is imported when a build's in-map target equals it, with a reference's
  trailing `.mdl` read as `.mdx`, because the game loads the `.mdx`.
- **Against the in-game list:** the same rule, plus: for textures, the extension does not count. Reforged models
  reference `.tif`, `.tga` or `.blp` textures that the game stores as `.dds` in HD and `.blp` in SD, so
  `Doodads\...\plant1_Normal.tif` is an in-game path when the game has `plant1_normal.dds` or `plant1_normal.blp` at that
  place. Texture extensions: `blp`, `dds`, `tga`, `tif`, `tiff`, `png`, `jpg`.

A reference is `in-game path, replaced` when it is both imported and in-game.

## 4. The in-game path list

- **Source file:** `cli/data/game-paths.txt`, committed. Its first line is a comment naming the game version it was
  generated from (`# Warcraft III 3.0.0.24268`), then one path per line: lowercase, `/` separators, sorted, unique.
  Lines starting with `#` are comments.
- **Generator:** `tools/gen-game-paths.ts <listfile> <game version>`, a repo-only task `deno task gen:game-paths`. It
  reads a plain text list of the game's stored file names, one per line (exported from the game's CASC storage with
  CascView or CascLib), and normalizes each line:
  1. Trim it; skip empty lines.
  2. Lowercase it and turn `\` into `/`.
  3. Drop storage prefixes: everything up to the last `:` (e.g. `war3.w3mod:`), then every leading folder up to and
     including the last folder whose name ends in `.w3mod` or `.mpq` (e.g. `_hd.w3mod/`, `_locales/enus.w3mod/`).
  4. Keep only the file types a model can reference: `mdx`, `mdl`, `pkfx`, and the texture extensions above.

  It writes the header and the sorted, unique paths.
- **Embedding:** `deno task gen` compresses the file with gzip into `cli/src/embedded/game-paths.ts`
  (`GAME_PATHS_GZIP_BASE64`). The freshness test decompresses it and compares the text with `cli/data/game-paths.txt`,
  so compressor differences between Deno versions cannot fail it.
- **Loading:** `cli/src/models/game-paths.ts` decompresses the list once per run and turns it into a set of match keys
  (§3).
- **Until the real list exists,** `cli/data/game-paths.txt` holds only a header saying so, and the command prints the
  warning in §2.

## 5. The one-time step

The maintainer exports the list of stored file names from their Warcraft III install with CascView (or CascLib's test
tool). They run `deno task gen:game-paths <list> <version>` and commit the result. The generator's normalization may
need adjusting once the real export format is seen; that adjustment is part of this step. The list is regenerated when
a game patch adds files.

## 6. Testing

- **Normalization unit tests** covering:
  - prefixes like `war3.w3mod:`, `war3.w3mod:_hd.w3mod:`, `_locales/enus.w3mod/` and `war3.mpq:`;
  - backslashes and letter case;
  - filtering by extension;
  - sorting and de-duplication;
  - the header.
- **Loader test:** a gzip round trip through the embedded format.
- **Match key tests:** a `.tif` reference matching a `.dds` in-game path; `.mdl` matching `.mdx`; a different folder not
  matching.
- **Command tests.** The command accepts an injected in-game path set for testing. The tests cover each of the four
  statuses, the summary lines in and outside a project, `\` display, and the empty-list warning.
- **The freshness test** covers `cli/src/embedded/game-paths.ts`.
