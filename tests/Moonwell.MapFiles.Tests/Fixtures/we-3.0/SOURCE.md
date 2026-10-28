# World Editor 3.0 save: one new object per tab, name only

Copied unchanged on 2026-10-26 from wc3-cs `moonwell/tests/Moonwell.Build.Tests/Fixtures/WorldEditorObjects/`
(moonwell commit `bd0e5de`), every file except its `README.md`. That README describes the files: Sven saved them with
World Editor 3.00 (Warcraft III 3.0.0.24268) on 2026-09-26, one new custom object on each Object Editor tab with only
its name changed, in Moonwell's template map. This is actual editor output.

| Files                             | Base → custom id | Changed field                               |
| --------------------------------- | ---------------- | ------------------------------------------- |
| `war3map.w3u` / `war3mapSkin.w3u` | `hpea` → `h000`  | `unam` (skin file)                          |
| `war3map.w3t` / `war3mapSkin.w3t` | `ratf` → `I000`  | `unam` (skin file)                          |
| `war3map.w3b` / `war3mapSkin.w3b` | `DTrf` → `B000`  | `bnam` (skin file)                          |
| `war3map.w3d` / `war3mapSkin.w3d` | `UObb` → `D000`  | `dnam` (main file)                          |
| `war3map.w3a` / `war3mapSkin.w3a` | `ANab` → `A000`  | `anam` (skin file, level 0, data pointer 0) |
| `war3map.w3h` / `war3mapSkin.w3h` | `BNab` → `B000`  | `fnam` (skin file)                          |
| `war3map.w3q` / `war3mapSkin.w3q` | `Rhme` → `R000`  | `gnam` (skin file, level 1, data pointer 0) |

The SHA-256 of every file matches the list in that README.
