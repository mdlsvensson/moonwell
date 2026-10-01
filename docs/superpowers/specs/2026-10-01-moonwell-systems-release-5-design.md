# Moonwell Systems Release 5 (v0.5.0): Persistence — Design

- **Date:** 2026-10-01
- **Status:** The design was approved in chat on 2026-10-01; this written spec awaits review.
- **Builds on:** Part 1 of `2026-09-30-moonwell-systems-design.md`, which binds this release (rules §4, tooling §5,
  releases §6; §3 allows raw natives for Preload files), and releases 1 to 4.
- **Inputs:** `wc3-lib`'s `persistence/codec.ts`, `format.ts`, `local-file.ts`, `sync.ts` and their tests
  (`tests/persistence.test.ts`); the port-needs note §6.4 and §6.5 (sync and Preload on one machine); and the three
  Preload probes of 2026-10-01 (§2).
- **Target:** moonwell-systems `v0.5.0`, the last release of the port. It still needs moonwell-wrappers `v0.7.0`
  (`wrappers.sync`) and Moonwell 0.5.2. The wrappers do not change.

## 1. Scope and the maintainer's choices

Three modules: `systems.codec`, `systems.sync` and `systems.savefile`, on one internal module
(`systems.internal.preload`). The maintainer chose (2026-10-01):
- **Saves travel through local files and sync only.** Codes are packed as small as possible, in 64 symbols, and are
  not designed for typing.
- **A save holds single values and lists** of one kind of value. No nested records.
- **A keyed check with a map secret:** the check value mixes the secret, the player's name and the data.
- **`systems.savefile` does the safe flow:** the steps that run on one machine only are inside the library, and a
  map's callbacks run on every machine at the same moment.
- **Three probes ran before this spec** (§2).

## 2. Measured by the probes (2026-10-01, Warcraft III 3.0.0.24268)

`../wrappers-gate/PROBE-PRELOAD-RESULTS.md` has the raw lines. The port-needs note already had: a Preload argument
keeps 259 characters; a cut line can crash the game; appending to one tooltip inside the file does not work; a sync
message over 255 bytes is cut.

**Carriers** (how the file's JASS hands text to Lua):
- **Lua run from the file does not work.** A file that closes its JASS function and switches to Lua with
  `//!beginusercode` (with or without a space) set nothing. The game did not crash.
- **A game cache works, but it makes handles.** A file that stores each chunk with
  `StoreString(InitGameCache("name"), …)` read back whole for 400 lines (76,000 characters). But `InitGameCache` on a
  known name returns a new handle every time (ids 1048696, then 1048697), and running a 64-line file left 64 handles
  behind. A file is read on one machine only, so those handles would exist on that machine only.
- **The file's JASS does not know blizzard.j's globals.** A file that kept one handle in `bj_lastCreatedGameCache`
  crashed the game to desktop.
- **Tooltips make no handles.** A file that sets tooltips left 0 handles behind.
- **One chunk per ability field.** Only level 0 of an ability keeps a tooltip of its own (`Amls`; the other levels
  did not), and of the six text fields only the tooltip and the extended tooltip do. A tooltip set from Lua kept
  9,000 characters. An ability id that no ability has keeps nothing.
- **48 of 50 standard unit abilities carry text** in both fields and restore (`Aflr` and `Aspa` do not). A file of 96
  chunks through all of them (18,240 characters, the longest argument 248) read back whole, restored every tooltip
  and left 0 handles behind, in 61 ms.

**Files:**
- An argument of exactly 255 and of exactly 259 characters reads back whole.
- A second write replaces the file. A missing file changes nothing.
- Writing took 3 to 12 ms. Reading took about 40 ms for a small file and 76 ms for 400 lines.

**Integers** (the game's are 32-bit, the test runner's 64-bit):
- `1 << 31`, `-2 >> 16` (65535), `~`, `|` and `//` work in the game's Lua, and `0xFFFFFFFF` reads -1.
- FNV-1a of a fixed text, with every product masked by `& 0xFFFFFFFF`, and the same value after the murmur3
  finalizer, read the same in the game as outside it. So a hash written with that mask agrees on both.

**Sync on one machine:** 45 packets of 250 bytes sent in one burst all arrived whole and in order within 0.10 s. A
packet of every printable ASCII byte (32 to 126) and one of every byte from 128 to 255 arrived whole.

## 3. Structure

- `systems.codec` is pure: it calls no native and requires no wrappers module.
- `systems.sync` builds on `wrappers.sync`, `wrappers.player` and a `Scheduler`.
- `systems.savefile` builds on the two and on `systems.internal.preload`, which alone calls the Preload natives and
  the tooltip natives, raw (Part 1 §3).
- Nothing in this release creates a handle on one machine only.

## 4. `systems.codec`

```
Codec.new({version, secret, schemas})
codec:encode(data, binding = "")   -> code
codec:decode(code, binding = "")   -> data            or  nil, reason, detail?
codec:getVersion()                 -> integer
codec:getMaxLength()               -> integer   the longest code the current schema can produce
```

- `version`: the current version, 1 to 9999. `encode` writes this one.
- `secret`: a non-empty string that the map keeps to itself.
- `schemas`: a list of `{version, fields, migrate?}`, one per supported version, the current one included.
  `migrate(data)` returns the data of the next version.
- `fields`: a list, at most 64, in encoding order. Each has a `key` (a non-empty string, unique in its schema) and a
  `kind`:

| kind | options | value |
| --- | --- | --- |
| `"integer"` | `min`, `max` | a whole number from `min` to `max`. Both lie within ±2,147,483,647, and `max - min` is at most 2,147,483,647. |
| `"boolean"` | | `true` or `false` |
| `"string"` | `maxLength` (0 to 4095) | a string of at most `maxLength` bytes; any bytes |
| `"list"` | `maxLength` (0 to 4095), `of` | an array of at most `maxLength` values; `of` is a field of one of the three kinds above, without a `key` |

- Numbers are whole. A map that wants 12.5 stores 125: the game's floats are single precision, so fractions do not
  survive exactly. A float with a whole value (`3.0`) is accepted and stored as the integer.
- `Codec.new` checks everything above and raises at the caller. It also raises when the current schema's longest
  code would pass 8,192 symbols.
- `encode` raises at the caller when the data does not fit the current schema, naming the field:
  `[systems] Codec.encode: field "gold": expected a whole number from 0 to 1000000`. A key that the schema does not
  have raises too (the first in sorted order), so a misspelled key is found.
- `decode` never raises for a bad code: codes come from files and other machines. Its reasons:
  - `format`: not a code (a symbol outside the alphabet, too short, an unknown layout, bits missing or left over);
  - `checksum`: the check value does not match: an edited code, another secret or another binding;
  - `version`: no schema for the code's version;
  - `schema`: the bits do not fit that version's schema, which means a schema changed without a new version;
  - `migration`: a `migrate` function is missing, failed, or returned data that does not fit the next schema. `detail`
    is the message.
- A `binding` is any string, typically the player's name: a code decodes only with the binding it was encoded with.

### 4.1 Layout

- The alphabet, in order of value 0 to 63: `A-Z`, `a-z`, `0-9`, `-`, `_`. A symbol is 6 bits, the highest first.
- The body is a row of bits: the layout number (6 bits, the value 1), the version (14 bits), then every field in
  schema order. It is padded with zero bits to whole symbols.
  - integer: `value - min` in as many bits as `max - min` needs (none when `min` equals `max`);
  - boolean: 1 bit;
  - string: its length in as many bits as `maxLength` needs, then 8 bits per byte;
  - list: its count in as many bits as `maxLength` needs, then each element.
- The code is the body's symbols, then 5 symbols of check value (30 bits).
- The check value: FNV-1a (32-bit) over the secret, a zero byte, the binding, a zero byte, the body's symbols, a
  zero byte and the secret again; then the murmur3 finalizer; then its low 30 bits.
- Gold from 0 to 1,000,000 takes 20 bits. A save of ten such numbers and a 16-byte name is 64 symbols.

### 4.2 Integers

- Packing never relies on a wrapped result (Part 1 §4.6): every offset is at most 2,147,483,647 by the rule on
  `max - min`, and bits are written and read at most 16 at a time.
- **The check value is the one exception.** Its multiplications are meant to wrap at 32 bits, and each product is
  masked with `& 0xFFFFFFFF` so the 64-bit test runner computes what the game computes. The third probe measured
  that they agree; the gate compares fixed codes from the tests with the game's (§8).
- The check is not cryptography. The secret is in the map script, and a player who reads it can forge a save. It
  stops everyone who does not.

## 5. `systems.sync`

```
Sync.new(clock, {prefix = "mwsync", timeout = 10, maxLength = 8192, onError?})
sync:start()
sync:ask(player, read, receive)
sync:dispose()
```

One primitive: ask one player's machine for a local value, and hand the answer to every machine at the same moment.

- `ask` must be called on every machine, in the same order, like any other game code. It numbers the request.
- `read()` runs on `player`'s machine only, at once, behind the callback boundary. It returns a string or nil.
- `receive(text, reason)` runs on every machine, from a sync event or a scheduler step, never inside `ask`:
  - with the text when it arrived whole;
  - `nil, "none"` when `read` returned nil;
  - `nil, "error"` when `read` failed, returned something that cannot be sent (not a string, longer than `maxLength`,
    or with a byte below 32), or when a packet was malformed. The failure is reported on the machine that saw it;
  - `nil, "absent"` when no human plays in that slot (the controller is not a user, or the slot is not playing);
  - `nil, "timeout"` after `timeout` seconds of the scheduler without an answer, for example when the player left;
  - `nil, "disposed"` when the system was disposed first.
- **Packets.** The text is cut into pieces of 220 bytes, each sent as `<request>.<index>.<count>.<piece>` under the
  prefix; an answer without text is `<request>.0.0.<N or E>`. Every packet is under the 255-byte limit. They are sent
  in one burst.
- **Only the asked player's packets for an open request are taken.** The sender is the one the engine reports,
  never one named in the data. Anything else is ignored.
- A request ends exactly once. `dispose()` ends the open ones in the order they were asked, removes the listener and
  cancels the timeouts. It is idempotent; `ask` and `start` raise afterwards.
- `start()` is needed before `ask`, and registers the listener (`Sync.on` of the wrappers).

It is also the safe way to share any other local value, such as `Time.localUtc()`.

## 6. `systems.savefile`

```
Savefile.new(clock, {codec, folder, prefix = "mwsave", timeout = 10, abilities?, onError?})
saves:start()
saves:save(player, slot, data)
saves:load(player, slot, callback)
saves:dispose()
```

- `folder` and `slot`: 1 to 32 letters, digits, `-` or `_`. The file is
  `Documents\Warcraft III\CustomMapData\<folder>\<slot>.pld` on the player's machine.
- **`save`** runs on every machine. It encodes the data with the player's name as the binding, so data that does not
  fit raises on every machine alike (`[systems] Savefile.save: field "gold": …`). Only the player's own machine
  writes the file. For a slot without a human it writes nothing.
- **`load`** asks the player's machine for the file (§5): it answers with the code, with nothing for a missing
  file, or with a marker for a damaged one. Every machine then decodes the code with the player's name and calls
  `callback(data, reason)` at the same moment. Reasons:
  - `missing`: no file;
  - `damaged`: the file does not hold what `save` writes;
  - the codec's `format`, `checksum`, `version`, `schema` and `migration` (its detail is reported through `onError`);
  - the sync system's `error`, `absent`, `timeout` and `disposed`.
- `start()` checks that every borrowed ability can carry text (it sets, reads and restores both fields, on every
  machine) and raises `[systems] Savefile.start: ability 'Aflr' cannot carry text` otherwise. Then it starts its sync
  system.
- `Savefile.new` raises when the codec's longest code does not fit the carriers.
- `dispose()` disposes the sync system; open loads end with `disposed`.

### 6.1 The file

- The text is `MWS1.<length of the code>.<code>`, cut into chunks of 190 characters.
- Each chunk has a carrier: the tooltip, then the extended tooltip, of each borrowed ability in turn. Its Preload
  argument closes the generated `Preload( "` line and adds a line of JASS:
  `call BlzSetAbilityTooltip(<id>, "<chunk>", 0)` (or `BlzSetAbilityExtendedTooltip`). The longest argument is 248
  characters, under the 259 that Preload keeps.
- **Reading:** remember every carrier's text, set the first to a marker, run `Preloader`, read the header from the
  first carrier and the chunks it calls for, and restore every carrier before returning. A marker still in place
  means `missing`. A wrong header, a length over the capacity, or a character outside the codec's alphabet means
  `damaged`.
- **The default carriers** are 24 abilities the probe found good: `Amls Aroc Amic Amil Aclf Acmg Adef Adis Afbt Afbk
  Aflk Agyb Agyv Ahea Ainf Aivs Amdf Aply Asth Aslo Asps Afsh Absk Ablo`. That is 48 chunks, 9,120 characters: room
  for the longest code a codec may have. `abilities` replaces the list (ability ids).
- A tooltip is changed only inside one call, on one machine, and restored before the call returns.

### 6.2 Limits, documented in the README

- **A file edited by hand can crash that player's game** when it is read: `Preloader` runs the file, and the second
  probe crashed on a file the game could not run. The library writes only the 64 symbols and lines within the
  limit.
- Reading freezes that player's game for 40 to 80 ms.
- Two or more machines are untested until the online checks before Moonwell 1.0; the gate runs on one.

## 7. Changes from `wc3-lib`

- Codes are bit-packed in 64 symbols, not hex text; they are about a third as long.
- Fields are `integer`, `boolean`, `string` and `list`. Non-integer numbers are gone; strings hold any bytes.
- The check value is keyed with a map secret. `encode` raises for data that does not fit, instead of returning a
  failure; `decode` returns `nil, reason`.
- There are no ports. The local store is internal, behind `Savefile`, and carries one chunk per ability field:
  `wc3-lib`'s appended tooltip read back only its first chunk.
- `SyncReceiver`, `WarcraftSyncTransport`, sessions and `expect` become one call, `sync:ask`, which also answers for
  a missing value, an absent player and a timeout.
- A map no longer writes a local-player check to save or load.

## 8. Verification

Automated (Part 1 §5):
- suite `codec`: every field kind at the exact edges of its range; lists; the layout's bit counts; fixed codes
  (golden vectors) that the gate compares in game; every `decode` reason; migrations; the `new` and `encode` checks at
  the caller's line;
- suite `sync`: an answer on the asked player's own machine and on another machine (the packets injected as the
  engine would deliver them); `none`, `error`, `absent`, `timeout`, `disposed`; packets from another player, for a
  closed request, out of order, duplicated and malformed; one end per request;
- suite `preload`: writing and reading through a simulated `Preloader` that runs the generated lines; a missing
  file, a damaged header, a foreign character; every tooltip restored; the argument lengths;
- suite `savefile`: the round trip, each reason, the checks of `new` and `start`, and that only the player's own
  machine writes;
- the `blame` sweep over the new modules, and `imports` (`systems.codec` loads no wrappers module and calls no
  native);
- integration: one-module bundles for the three entry points, LuaLS fixtures, and the new gate example.

In-game gate (the maintainer, 3.0.0.24268, one machine): its own run, `deno task gate systems-save`, from
`examples/gate-save.yue`. Nothing needs watching; the lines also go to a file.
1. **Parity:** three fixed sets of data encode to the same codes as in the tests.
2. **Round trip:** `save`, then `load`, gives the same data. The file is on disk.
3. **A missing slot** gives `missing`.
4. **Another binding:** a code decoded with another name gives `checksum`.
5. **A damaged file** gives `damaged`, and a code with one symbol changed gives `checksum`.
6. **The largest save** (a code near 8,192 symbols) round-trips; record the read time.
7. **A migration:** a version 1 file loads as version 2 data.
8. **`ask`:** a local value arrives; a slot without a human gives `absent`.
9. **Tooltips:** every borrowed ability reads as it did before the run.
10. No `[systems] … failed` line prints.

Then tag `v0.5.0` and check tag consumption with every gate example.

## 9. Out of scope

Codes for typing; nested records; non-integer numbers; compression; encryption; saving on a schedule; and the
two-player checks, which wait for the online step before Moonwell 1.0 with the other deferred ones.
