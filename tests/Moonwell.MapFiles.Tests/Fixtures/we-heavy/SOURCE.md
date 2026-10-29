# World Editor save with edits on every tab

Sven saved this map with World Editor on 2026-10-26, starting from Moonwell's template map
(wc3-cs `moonwell/template/MoonwellMap/map`), as the folder map `Documents\Warcraft III\Maps\ObjectEditorTest.w3m`.
Only its object files and `war3map.wts` are copied here, unchanged. This is actual editor output.

| Kind | Changed standard objects | New objects | Levels |
| --- | --- | --- | --- |
| Units | `hfoo` (`uhpm` 999, name), `hmtt` (name) | `h000` from `hmtm`, hero `H001` from `Hamg` | |
| Items | `afac` (name, model) | `I000` from `ratf` | |
| Destructables | `ITf2` (`bhps` 0, name) | `B000` from `NTtw` | |
| Doodads | | `D000` from `YOs1` (`dvar` 2) | `dvg1` on variation 2 |
| Abilities | | `A000` from `AHhb` | `Hhb1`, `acdn`, `atp1`, `aub1` on levels 1-3 |
| Buffs | `BHav` (`frac`, `ftip`, `ftac`) | `B000` from `Bbsk` | |
| Upgrades | | `R000` from `Rhme` | `gtp1`, `gnsf`, `ghk1` on level 2 |

Every file is version 3 with set count 1. Every modification sits in the file Moonwell's skin flag predicts (skin fields
in `war3mapSkin.*`, the rest in `war3map.*`), and every ability data pointer matches the metadata. Names are
`TRIGSTR_007` to `TRIGSTR_027` in `war3map.wts` (UTF-8 with a BOM).
