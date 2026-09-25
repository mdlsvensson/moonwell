# Moonwell Model Paths and Icon Folders — Design

- **Date:** 2026-09-25
- **Status:** Approved in conversation, pending spec review
- **Builds on:** `2026-09-24-moonwell-core-design.md` (§6.2 Assets, as implemented by Plan 2a)

## 1. Summary

Two additions to Moonwell's asset support:

1. **`assets:paths`:** a command that lists the files a Warcraft III model (`.mdx` or `.mdl`) references, as a model
   viewer shows them. It marks each one as found or not found among the files the project imports. This catches the
   common mistake of importing a model but not its textures, or importing them at the wrong path.
2. **Icon folders:** `init` creates the standard icon folders under `assets/`, so a user who knows World Editor's icon
   paths can see where icons go.

The hard constraints of the core design still apply: no Node.js and no npm packages; expected failures are
`MoonwellError`; async file I/O.

## 2. The `assets:paths` command

`deno task assets:paths [file]`

- **With a file** (any `.mdx` or `.mdl`, inside `assets/` or anywhere else), it reports that model.
- **Without a file**, it reports every `.mdx`/`.mdl` the build would import from `assets/` (after `assets.paths` and
  `assets.exclude`), one block per model, in import order.
- **Found** means the path is an in-map path the build would produce: an asset's target after `assets.paths` mapping and
  `exclude`. Matching ignores letter case and `/` versus `\`. A reference ending in `.mdl` also matches the imported
  `.mdx`, because the game loads the `.mdx` when a model asks for the `.mdl`. The rule applies to references only: an
  imported `.mdl` is never loaded, so it does not satisfy a reference to either `.mdl` or `.mdx`.
- **Outside a Moonwell project** (no `moonwell.pkl`), the command still lists the paths, without the found / not-found
  column. Without a file argument outside a project, it fails with a hint to pass a file.
- It only reports and always exits 0 once the model has been read. A path that is not found is often a built-in game
  file, so failing on it would be wrong.
- An unreadable model fails with a `MoonwellError` naming the file (in its `file` field, printed once by the error
  formatter), with the message `Not a readable model: <problem>.`

Example output:

```
assets/Models/Knight.mdx
  texture          Textures\Knight.blp                found
  texture          Textures\KnightCape.blp            not found
  texture          team colour (slot 1)
  particle model   Abilities\Spells\Human\Heal.mdx    not found
1 model, 4 paths: 1 found in assets/, 2 not found (built-in game files or missing imports).
```

Replaceable textures have no path and no found column. The summary counts every reference, and found plus not found
counts only references that have a path.

## 3. The model reader

`cli/src/models/paths.ts` exports a pure function:

```ts
type ModelPathKind = "texture" | "particle model" | "particle texture" | "attachment" | "popcorn" | "face effect";
interface ModelPath { kind: ModelPathKind; path: string | null; replaceableId: number }
function modelPaths(bytes: Uint8Array, file: string): ModelPath[];
```

It detects the format from the content: bytes starting with `MDLX` are binary MDX, anything else is read as MDL text.
Results are in file order. `path` is `null` only for a replaceable texture with an empty path.

### 3.1 MDX (binary)

After the 4-byte `MDLX` magic, the file is a sequence of chunks: a 4-byte ASCII tag, a little-endian `uint32` byte size,
then that many bytes. The reader handles five tags and skips every other chunk by its size, so unknown and future
(Reforged) chunks never break it. Strings are fixed-size, NUL-padded, read up to the first NUL.

| Tag | Layout (per record) | Reported as |
| --- | --- | --- |
| `TEXS` | fixed 268 bytes: `uint32 replaceableId`, `char[260] path`, `uint32 flags` | `texture` |
| `PREM` | `uint32 inclusiveSize`, node, `float32 ×4`, `char[260] path`, ... | `particle model` or `particle texture` |
| `ATCH` | `uint32 inclusiveSize`, node, `char[260] path`, ... | `attachment` |
| `CORN` | `uint32 inclusiveSize`, node, `float32 ×3`, `float32[3]` colour, `float32` alpha, `uint32 replaceableId`, `char[260] path`, ... | `popcorn` |
| `FAFX` | fixed 340 bytes: `char[80] type`, `char[260] path` | `face effect` |

A **node** starts with its own `uint32 inclusiveSize` (counting itself), then `char[80] name`, `int32 objectId`,
`int32 parentId`, `uint32 flags`; the rest of the node is skipped using its size. Records with `inclusiveSize` are
stepped over by that size. A `PREM` record is a `particle texture` when its node flags have the `EmitterUsesTGA`
bit (`0x10000`) and not the `EmitterUsesMDL` bit (`0x8000`). In every other case, including neither bit set, it is a
`particle model`, which is the classic emitter's default.

Only records with a non-empty path are reported, except textures. A texture with an empty path is reported with
`path: null` and its `replaceableId`: 1 is shown as "team colour (slot 1)", 2 as "team glow (slot 2)", and any other ID
as "replaceable texture (slot N)".

**Errors:** a file shorter than a chunk header, a chunk or record running past its parent's end, a `TEXS`/`FAFX` size
that is not a multiple of the record size, or an `inclusiveSize` smaller than its fixed fields each throw
`MoonwellError` ("Not a readable model: <problem>.", naming the file in its `file` field), with a hint to re-export it.

The layouts were checked against the open-source mdx-m3-viewer MDLX parser.

### 3.2 MDL (text)

A small tokenizer produces quoted strings, words or numbers, `{`, `}` and `,`, and skips `//` comments. The reader keeps
a stack of block names. A block opens as `Name [ "label" | number ] {`.

- In a `Bitmap` block: `Image "<path>"` and `ReplaceableId <n>` make one `texture` when the block closes. The same
  empty-path rule as MDX applies.
- In a `ParticleEmitter` block: `Path "<path>"`, read from the block itself or from its nested `Particle` block (where
  exporters write it), makes a `particle model`, or a `particle texture` when the emitter block has the
  `EmitterUsesTGA` flag and not `EmitterUsesMDL`.
- In an `Attachment` block: `Path` makes an `attachment`.
- In a `ParticleEmitterPopcorn` block: `Path` makes a `popcorn`.
- In a `FaceFX` block: `Path` makes a `face effect`.

Everything else is ignored. An unterminated string or unbalanced braces throw the same `MoonwellError`. An MDL must open
a top-level `Version` or `Model` block (real exporters write both); otherwise it is not a readable model. This keeps an
empty file, plain text or a Git LFS pointer from being reported as a model with no references.

## 4. The command module

`cli/src/commands/assets-paths.ts`:

- It loads the project when `moonwell.pkl` exists, then collects the in-map targets with the existing `collectAssets`.
  This reads the files but writes nothing and takes no build lock.
- With a file argument, the path is resolved against the project folder (where `deno task` runs), not the shell's
  current directory, then read and reported.
- Without one, it reports each collected asset whose target ends in `.mdx` or `.mdl`, using the bytes already read. Its
  heading is `assets/<source>`.
- Output goes through the logger, like the other commands. The column layout pads the kind and path columns to the
  widest entry in each model's block.
- `cli/src/main.ts` dispatches `assets:paths` with an optional positional file. `PROJECT_TASKS` gains `assets:paths`, and
  the template's `deno.json` is regenerated.

## 5. Icon folders in the template

`template/assets/` gets three folders, each holding an empty `.gitkeep`, which keeps the folder in git and is never
imported because asset names starting with `.` are skipped:

| Folder | Holds |
| --- | --- |
| `ReplaceableTextures/CommandButtons/` | `BTN<Name>.blp`: icons for abilities, units, items and upgrades |
| `ReplaceableTextures/CommandButtonsDisabled/` | `DISBTN<Name>.blp` and `DISPASBTN<Name>.blp`: greyed-out versions |
| `ReplaceableTextures/PassiveButtons/` | `PASBTN<Name>.blp`: passive ability icons |

The old top-level `template/assets/.gitkeep` is removed. `war3mapImported/` is deliberately left out: `assets.paths`
covers custom paths, and World Editor's default import folder adds nothing.

The README gets an "Icons" section with the naming rules. For every `BTN<Name>`, a map needs a matching `DISBTN<Name>`,
because the game shows a placeholder when an unavailable ability has no disabled icon. For every `PASBTN<Name>`, it
needs a `DISPASBTN<Name>` in `CommandButtonsDisabled`.

## 6. Testing

- **Unit tests for the reader** (`cli/tests/unit/model-paths.test.ts`) use a test-only MDX builder in
  `cli/tests/support/mdx.ts`, which writes chunks and records with the layouts in §3.1. The tests cover:
  - every tag: textures, both emitter kinds, attachments, popcorn and face effects;
  - replaceable textures 1, 2 and 11;
  - an unknown chunk being skipped;
  - truncated files and bad sizes throwing `MoonwellError`;
  - MDL samples for each block, including comments, nested blocks, the emitter flags, and an unterminated string.
- **Tests for the command**, which need pkl, use a temporary `init --link` project:
  - a model under `assets/` whose texture is present, and one that is missing;
  - a texture imported through an `assets.paths` mapping;
  - an `.mdl` reference found as `.mdx`;
  - the no-argument mode listing every imported model;
  - a file outside a project, listed without the found column.
- **`init` test:** the scaffolded project has the three icon folders, and a build imports nothing from them.
- **Main dispatch:** `assets:paths` is a known command.

## 7. Out of scope

- Generating `DISBTN` icons from `BTN` icons. That needs BLP decoding and encoding.
- Checking against the game's own file list to tell built-in files from missing ones.
- Warnings from `check` or `build` about missing references.
- Reading animations, geometry or any other model data.
