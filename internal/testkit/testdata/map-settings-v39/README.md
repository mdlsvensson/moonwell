# World Editor fixture

Copied unchanged from this project's source map on 2026-09-21. The header records
Warcraft III 3.0.0.24268, map-info format 39. This is actual editor output, not a
fixture serialized by the library under test.

`war3map.w3i` SHA-256:
`969a64caa8c04393fd79dc0418378e410fa1cb33f0fe2a39f6cc3761646438ce`

The Lua file provides an independent cross-check of player IDs, controllers,
races, fixed starts, coordinates and force membership. Unit tests pin binary
offsets and preservation of untouched bytes. They do not prove game acceptance.

Format-39 observations used by the reader:

- Four bytes between loading background and model path (offset 141, value 64).
- A 24-byte block after weather and before sound environment.
- A 40-byte block after the three camera zoom values and before player count.
- Four bytes between each player's race and fixed-start flag (value 64, matching
  `RACE_PREF_USER_SELECTABLE` race skin in the accompanying Lua).

Unknown fields are preserved verbatim. These observations establish this fixture's
layout; they do not identify every new field or establish when it was introduced.
Legacy layout reference: https://github.com/ChiefOfGxBxL/WC3MapSpecification/blob/master/Info/0-33.md

## `war3map-colors.w3i`

`war3map.w3i` after the maintainer opened the map in World Editor 3.00 on 2026-09-26, turned on custom water
tint with red (255, 0, 0), set the fog colour to red (255, 0, 0), picked the "Dungeon" sound environment, and
saved. It shows that colours are stored blue, green, red, alpha; that water tint sets flag `0x10000`; and that the
sound environment is stored by its internal name (`Dungeon`). World Editor also rewrote the save counter (offset 4),
the unknown field at offset 141, and the three camera zoom values (1250).

SHA-256: `fcd917acdc17ae8f8f10790fbec9e4d7dcba54dfeeab1b64ec7af0913992f122`
