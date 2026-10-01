# Moonwell Custom Map Preview (Moonwell 0.7.0) — Design

- **Date:** 2026-10-01
- **Status:** Approved by the maintainer on 2026-10-01.
- **Builds on:** `2026-09-25-moonwell-map-settings-design.md` (settings patch the staged map only; the planner returns
  the new bytes of internal files). Everything in it still applies unless this spec changes it explicitly.
- **Roadmap:** phase 4, item 3, and the backlog entry "Custom map preview for Reforged".
- **Target:** Moonwell 0.7.0 (`@moonwell/cli` and the Pkl package `moonwell@0.7.0`).

## 1. Summary and scope

A map can show a picture of its own in the game's map list instead of its minimap. The manifest names the picture;
the build puts it into the staged map and keeps the minimap for the game itself.

Reforged ignores `war3mapPreview.tga`, the file Warcraft III used before, and shows `war3mapMap.blp`, the minimap
World Editor writes. So the picture has to take the minimap's place, and a script call gives the game its minimap
back. Both parts were measured before this design (§2).

The maintainer chose (2026-10-01): the picture is a BLP or a TGA. Moonwell does not read PNG; every image editor
exports TGA, and a PNG reader can be added later.

## 2. What the probes measured

Two probes on Warcraft III Reforged 3.0.0.24268 (`../wrappers-gate/PROBE-PREVIEW-RESULTS.md`).

**The map list** (the single-player custom game screen), eight maps that differed only in their picture files:

- `war3mapPreview.tga` is ignored: the list shows the minimap.
- `war3mapMap.blp` replaced by another BLP1 is shown, at 256×256 and at 512×512.
- `war3mapMap.tga` is shown when the map has no `war3mapMap.blp` (32-bit, uncompressed, rows from the bottom, 256×256).
- A `war3mapMap.blp` that holds TGA or DDS bytes closes the game the moment the map is selected.
- The game draws the start location markers over the picture. On the 512 picture the marker was smaller and nearer
  the top left: the markers are placed for a 256 picture.

**The minimap in the game**, with World Editor's minimap kept in the map as `war3mapMinimap.blp`:

- `BlzChangeMinimapTerrainTex(path)` draws the picture it names at once and returns true: a BLP1 with a palette,
  World Editor's own JPEG BLP1, and `war3mapMap.blp` itself. A path that does not exist returns false and changes
  nothing.
- Called once after World Editor's `main` body, the game starts with that picture. A zero-second timer works too.
- Called before World Editor's `main` body, it returns true but has no effect, and it confuses the calls after it.

## 3. The manifest

```pkl
settings {
  info {
    preview = "preview.tga"
  }
}
```

- `settings.info.preview` (`String?`, default `null`): the picture's path from the project folder, with `/`. `null`
  changes nothing.
- The path is relative and stays inside the project: no empty, `.` or `..` segment, no drive, and no symlink on the
  way. It names a regular file.
- A path under `assets/` is an error: that file would also be imported as an asset under its own name. The hint says
  to keep it beside `moonwell.pkl`.
- An empty string is an error; the other `info` fields use it to clear text, and a picture has nothing to clear.

## 4. The picture

The file's extension decides how it is read: `.blp` or `.tga`, in any letter case. Another extension is an error whose
hint says to export the picture as TGA.

**Size.** 256×256 or 512×512, the two sizes seen to work. Any other size is an error. The README recommends 256×256,
where the game's start location markers sit right.

**BLP.** Checked by its header and used as it is:

- the first four bytes are `BLP1` (a `BLP2` file, the World of Warcraft format, gets a hint of its own);
- the content field is 0 (JPEG) or 1 (palette);
- the file is at least as long as its header (156 bytes), and its first mipmap lies inside the file.

**TGA.** Read and written again in the one layout the game was seen to accept, because a file the game cannot read
closes it:

- read: image type 2 (true colour) or 10 (true colour, run-length encoded), 24 or 32 bits a pixel, no colour map,
  rows from the top or from the bottom, any length of the ID field;
- refused, each with a message that names what was found: another image type, a colour map, another depth, rows
  stored right to left, pixel data that ends early or a run that overruns the picture;
- written: type 2, 32 bits, rows from the bottom, every pixel opaque (alpha 255), with an empty ID field and nothing
  after the pixels. The alpha of the source is dropped: a preview has no use for it, and a channel an editor left
  empty would make the picture invisible.

**Decision:** validation is strict and names the file, because the failure in the game is not an error message but a
closed game, in the map list, for everyone who has the map.

## 5. What a build changes

In the staged map only, with the other settings (after objects, before assets and bundle injection). The source map
is never written.

| File in the staged map | Change |
| --- | --- |
| `war3mapMinimap.blp` | New: the bytes of World Editor's `war3mapMap.blp`. |
| `war3mapMap.blp` | A BLP picture: replaced by it. A TGA picture: removed. |
| `war3mapMap.tga` | A TGA picture: new, the rewritten picture. |
| `war3map.lua` | One line as the last statement of `main()`: `BlzChangeMinimapTerrainTex("war3mapMinimap.blp")`. |

- The line goes directly before the `end` of the global function `main`, on a line of its own with the body's
  indentation and the file's line ending. That is "after World Editor's main body", the point the probe measured.
  Moonwell's runtime calls this `main` and then the `on_main` hooks, so gameplay code that sets a minimap of its own
  runs later and wins.
- File names are matched as the game matches them, without letter case: a map saved with `war3mapmap.blp` is changed
  under that name.
- `war3map.imp` is not changed. None of the three files is an import in World Editor's sense, and the built map is
  not meant to be saved from World Editor again.

Errors, all naming the source map's file to fix:

| Situation | Message and hint |
| --- | --- |
| The source map has no `war3mapMap.blp` | The preview needs the map's minimap; re-save the map in World Editor. |
| The source map already has `war3mapMinimap.blp` or `war3mapMap.tga` | The name is taken; remove that import from the map. |
| `war3map.lua` is missing, or has not exactly one global `main()` | As the other Lua settings report it. |

## 6. Commands

- `build`, `test`: apply the changes of §5. The line "Applied map settings to N internal file(s)" counts them.
- `check`, `dev`: plan them against the source map and report every error of §3 to §5 without writing.
- `settings:check`: lists each file, as today; a removed file is listed as `war3mapMap.blp (removed)`.
- `dev` also starts a cycle when the picture's file changes. It watches the file the manifest named when `dev`
  started, like the folders of local libraries.
- An asset whose in-map path starts with `war3mapPreview.` or `war3mapMap.` is refused as before; its hint now names
  `settings.info.preview`.

## 7. Code

- `cli/src/settings/picture.ts` (new): `readPreviewPicture(bytes, extension, file)` returns the bytes to put in the
  map and their in-map extension; the TGA reader and writer and the BLP check live here, with no file system access.
- `cli/src/settings/preview.ts` (new): resolves and reads the manifest's path, and plans the files of §5 against a
  map folder.
- `cli/src/settings/lua.ts`: the `main()` edit.
- `cli/src/settings/plan.ts`: `planMapSettings` takes the project folder, calls the preview planner, and a
  `SettingsChange` can remove a file (`bytes: null`); `applySettingsPlan` removes it. The w3i is not read for a
  manifest whose only `info` setting is the preview.
- `cli/src/settings/options.ts`: `info.preview`.
- `cli/src/commands/{settings-check,check,dev}.ts`, `cli/src/pipeline.ts`: pass the project folder; the removed
  line; the watched file.
- `cli/src/assets/paths.ts`: the hint.
- `schema/MapSettings.pkl`: `preview` in `MapInfo`. `template/moonwell.pkl`: `preview = null` with a comment.

## 8. Documentation

- README "Map settings": the setting, the two formats and sizes, that the picture replaces the minimap in the map
  list only, the start location markers, and that gameplay code may still call `BlzChangeMinimapTerrainTex` itself.
- README "Assets": `war3mapPreview.tga` does nothing in Reforged; use the setting.
- CONTRIBUTING: a release gate step for the preview (§10).
- CHANGELOG `0.7.0`; AGENTS (state, the backlog entry, the probes' findings) and the roadmap.

## 9. Testing

- **Unit, the picture:** a TGA of each accepted kind (type 2 and 10, 24 and 32 bits, both row orders, with an ID
  field) gives the same written bytes; each refusal of §4; both sizes and a wrong one; the BLP check (JPEG and palette
  content accepted; `BLP2`, another content value, a short file and a mipmap outside the file refused).
- **Unit, the plan:** the changes for a BLP and for a TGA picture, in order; the removal; names matched without
  letter case; each error of §3 and §5; a manifest with only `preview` does not read the w3i; `applySettingsPlan`
  removes the file.
- **Unit, the Lua edit:** the line is the last statement of `main`, with LF and with CRLF; a missing or doubled `main`
  fails; the other Lua settings still apply beside it.
- **Unit, commands:** `settings:check`'s lines; `dev` starts a cycle for the picture; the asset hint.
- **Pkl:** the schema accepts `preview`, refuses an empty string, and the template's manifest evaluates.
- **End to end:** a project with a TGA picture builds; the packed map holds `war3mapMap.tga`, `war3mapMinimap.blp`
  (equal to the source map's minimap) and no `war3mapMap.blp`, and its `war3map.lua` makes the call at the end of
  `main`.
- The pictures the tests use are built in code, as the probe's were; no binary fixture is added besides what the
  template map already has.

## 10. Release gate

Steps 1 and 2 of CONTRIBUTING, and one new in-game step, run once by the maintainer in a throwaway project from
`init --link` with a TGA picture Claude generates:

1. Build, and copy the packed map into the game's map folder.
2. The map list shows the picture, not the minimap.
3. In the game, the minimap shows the terrain, not the picture.

The other in-game steps are not re-run: their code is unchanged.

## 11. Plan phasing

One plan, `docs/superpowers/plans/2026-10-01-moonwell-map-preview.md`, of test-first tasks:

1. The picture: the TGA reader and writer, the BLP check.
2. The Lua edit.
3. The plan: the setting, the files, the removal, the errors.
4. The commands: `settings:check`, `check`, `dev`, the pipeline, the asset hint.
5. The schema, the template and the end-to-end test.
6. Docs and the gate step.
7. Release 0.7.0.

## 12. Departures found while planning

The plan's code was built and run in a scratch clone first (2026-10-01). It found:

- **`readPreviewPicture(bytes, file)`** takes the extension from the file's name instead of a parameter of its own.
- **`preview` is kept beside `info`** in the validated settings (`MapSettings.preview`), not inside it: every other
  `info` field is stored in `war3map.w3i`, and the map-info code walks them all.
- **The call's line has the indentation of `main`'s `end`**, not of its body. World Editor indents neither.
- **`dev`'s "Watching" line names the picture.**
- **A World Editor trigger that sets the minimap at map initialization is overridden:** it runs inside `main`, before
  the build's call. The README says to set the minimap from gameplay code instead. An earlier point for the call is not
  known to work: before World Editor's main body it does nothing (§2).
- **The settings tests with a manifest** run in-process with real Pkl (`cli/tests/pkl/settings.test.ts`), beside one
  end-to-end build and the `dev` test.

## 13. Out of scope

- **A minimap set by a World Editor trigger at map initialization** (§12).
- **PNG and other formats** (the maintainer's choice, §1).
- **Other sizes**, non-square pictures and resizing: only what was seen to work is accepted.
- **Hiding the start location markers** on the picture: no way to do it is known.
- **`war3mapPreview.tga`:** it stays a reserved asset path; Reforged does not read it.
- **The online lobby:** the probe looked at the single-player map list. The lobby of a hosted game belongs to the
  online checks before 1.0.
