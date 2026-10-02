# World Editor object fixture: one new object per tab, name only

Saved by the maintainer with World Editor 3.00 (Warcraft III 3.0.0.24268) on 2026-09-26: one new custom object on each
Object Editor tab, with only its name changed, in the template map. Copied unchanged from that save (commit `10a2441`).
This is actual editor output.

| File                              | Base → custom id | Changed field                               |
| --------------------------------- | ---------------- | ------------------------------------------- |
| `war3map.w3u` / `war3mapSkin.w3u` | `hpea` → `h000`  | `unam` (skin file)                          |
| `war3map.w3t` / `war3mapSkin.w3t` | `ratf` → `I000`  | `unam` (skin file)                          |
| `war3map.w3b` / `war3mapSkin.w3b` | `DTrf` → `B000`  | `bnam` (skin file)                          |
| `war3map.w3d` / `war3mapSkin.w3d` | `UObb` → `D000`  | `dnam` (main file)                          |
| `war3map.w3a` / `war3mapSkin.w3a` | `ANab` → `A000`  | `anam` (skin file, level 0, data pointer 0) |
| `war3map.w3h` / `war3mapSkin.w3h` | `BNab` → `B000`  | `fnam` (skin file)                          |
| `war3map.w3q` / `war3mapSkin.w3q` | `Rhme` → `R000`  | `gnam` (skin file, level 1, data pointer 0) |

Observations (format 3):

- Every file is written, main and skin, even when one side has no modifications: the object then appears with zero
  modifications in that file.
- Header: version `3`, original-object count, custom-object count. Each object: base id, custom id, set count `1`, set
  flags `0`, modification count, modifications.
- A modification: field id, value type (`3` = string), for `w3a`/`w3d`/`w3q` a level and a data pointer, the value, then
  a four-byte end marker (`0`).
- Names are stored as `TRIGSTR_*` references into `war3map.wts`.

SHA-256:

```
f4282b6f84c9ab7c0eabb0049a8d77a664faad48077e1de84dd8cb7e0931d6c8  war3map.w3a
40ea49f669f78dc1ecd0c7f96a63870f69fbe5d1e4c9958a90b1d412491977c6  war3map.w3b
844d1993a7866afda7294c841001a763c58a8352bd2dcda0c06130e9d63a76d1  war3map.w3d
e46e3d3dabe223ee727d50d296818a761441d887b385261a2d32dd481483cb64  war3map.w3h
f3917cb743612b540549283d54980269a6c16b7deaf26c66bb06897eef8bb567  war3map.w3q
77141c3fa28130d12e0f898dfa718165feb0aa5f8c99759ecb99201bbb501f37  war3map.w3t
04eb9b989a0326e1f39ec13f3cb54e7582c390d9a84535cdef41808f9043bea5  war3map.w3u
fa44bd352212a9bcb4ba9da80a4c27fcdb275b132cde3354680390e761c1e6be  war3map.wts
266382d89d2ee37a741d5ccc194ec8fb153a17e295b8b8f31b92a17c81d977a3  war3mapSkin.w3a
c1da9904a1c31d43e929604a37b655d34e956269101ae2312d5f1523a26f82ac  war3mapSkin.w3b
e89036856e6510d9a7aa67905ff89f6850ba1b9d3513a13b89481d615f756632  war3mapSkin.w3d
c32de0151a54c39677c947ef83c37b046684a6e9e47b26bdc800c09cead017ec  war3mapSkin.w3h
701da9d8f3889246c0040899affa2e8480986d7d3ffbc414f68c4e954ffb6d3a  war3mapSkin.w3q
2a79e2ec0a0fa0df71e1667302a30a31fb7fd9d025e1cf9d907177e53383b5ac  war3mapSkin.w3t
fe5dba8d1ebc7b8a33718822aa73d63519723f4d3620020252aa99f2b4c9b934  war3mapSkin.w3u
```
