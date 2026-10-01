# Moonwell Systems Release 5 (v0.5.0) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** moonwell-systems v0.5.0, the last release of the port: `systems.codec`, `systems.sync` and
`systems.savefile`.

**Architecture:** `systems.codec` is pure: a schema compiles to bit widths, values are written at most 16 bits at a
time into 64 symbols, and a keyed 30-bit check value ends the code. `systems.sync` asks one player's machine for a
text and delivers the answer, or the reason there is none, on every machine from a sync event or a scheduler step.
`systems.savefile` composes the two with `systems.internal.preload`, which writes a code as lines of JASS that set
the tooltips of borrowed abilities and reads it back with `Preloader`. Nothing creates a handle on one machine only.

**Tech Stack:** as releases 1 to 4 (annotated Lua 5.3, `yue -e` tooling, LuaLS 3.19.1, Lua 5.3.6 `luac`,
moonwell-wrappers v0.7.0, Moonwell 0.5.2).

**Spec:** `docs/superpowers/specs/2026-10-01-moonwell-systems-release-5-design.md` (and Part 1 of
`2026-09-30-moonwell-systems-design.md`).

**Verified in advance:** every code block below was run on 2026-10-01 in a scratch copy of the repository: the suites
(21 suites, 185 tests), the syntax check (51 files) and full integration (26 expected negative diagnostics, 15 entry
points, five gate examples) passed. The codec's fixed codes match a second implementation written from the spec's
layout alone. 92 mutations of the four new modules are each caught by a test.

## Global Constraints

- **Repository:** `C:\Users\mdlsvensson\Repo\moonwell-systems`; Moonwell records in `C:\Users\mdlsvensson\Repo\moonwell`;
  the gate map in `C:\Users\mdlsvensson\Repo\wrappers-gate` (not under git). Commit on `main`, explicit paths only,
  each check run as its own command. End every commit message with
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- **Checks**, from the repository root:
  - `yue -e tests/run.lua`
  - `MOONWELL_LUAC=../moonwell-wrappers/.tools/lua53/luac53.exe yue -e tools/check.lua`
  - `MOONWELL_LUALS="C:/Users/mdlsvensson/.antigravity-ide/extensions/sumneko.lua-3.19.1-win32-x64/server/bin/lua-language-server.exe" yue -e tools/integration.lua`
- **Write files with the file tools, not shell heredocs:** Git Bash turns `\\` into `\` and `\n` into a newline, and
  a Python heredoc turns `\a` into a bell character.
- **Messages:** `[systems] <Class>.<method>: <problem>`. New texts are in the code blocks; the classes are `Codec`,
  `Sync` and `Savefile`.
- **Callback labels:** `Sync read`, `Sync receive`, `Sync send`, `Sync packet`, `Savefile migration`,
  `Savefile callback`.
- **Reasons.** `decode`: `format`, `checksum`, `version`, `schema`, `migration`. `ask`: `none`, `error`, `absent`,
  `timeout`, `disposed`. `load`: `missing`, `damaged`, and all of those except `none`.
- **Levels and tail calls** as releases 1 to 4: 2 in a public function, 3 (+ depth) in a helper;
  `return (helper(...))`.
- **Private fields** use `---@field package`, never `private`. No field shares a name with a method of its class.
- **Lines** stay within 120 columns.
- **Integers:** the game's are 32-bit, the test runner's 64-bit. Only the check value's products may wrap, and each
  is masked with `& 0xFFFFFFFF`. Bitwise operators and `//` are fine in the library's Lua; yue 0.34.2 writes an empty
  file for a YueScript source that uses either, so the gate example has none.

## Departures from the spec

Found while prototyping; the spec records them (§10).

- **`getMaxLength()` is the longest code of any schema**, not of the current one, and the 8,192-symbol limit applies
  to each: a file saved under an older schema must still fit the carriers and the sync limit. A test of an older,
  longer schema found this.
- After the check value matches, every mismatch with the schema is `schema`; `format` is only for text that is not a
  code.
- An empty text is an answer of its own (`<request>.0.0.S`), and a piece that arrives twice is malformed.
- A file whose chunks do not add up to its header's length is `damaged`.
- **Found while executing:** a dry run of the compiled gate example, on stub natives outside the game, showed that
  its `ask` step disposed the sync system from the `absent` answer, one step after asking and before the local
  clock's answer could arrive. The example below disposes it at the end instead.

## File Structure

| File | Responsibility |
| --- | --- |
| `src/systems/codec.lua` | Schemas, bit packing, the check value, `encode`, `decode`, migrations |
| `src/systems/internal/preload.lua` | Local files: `write`, `read`, `verify`, `capacity`; the only caller of the Preload and tooltip natives |
| `src/systems/sync.lua` | `Sync`: `ask` one machine, packets, timeouts |
| `src/systems/savefile.lua` | `Savefile`: `save`, `load`, the borrowed abilities |
| `tests/codec.lua`, `tests/preload.lua`, `tests/sync.lua`, `tests/savefile.lua` | The four suites |
| `examples/gate-save.yue` | The in-game gate |

---

### Task 1: `systems.codec`

**Files:** Create `src/systems/codec.lua`, `tests/codec.lua`; modify `tests/suites.lua`.

**Interfaces:** Produces `Codec.new(options)`, `codec:encode(data, binding?)` (a string; raises for data that does
not fit), `codec:decode(code, binding?)` (the data, or `nil, reason, detail?`), `codec:getVersion()` and
`codec:getMaxLength()`.

- [ ] **Step 1: Failing tests** — `tests/codec.lua`:

```lua
local Codec = require('systems.codec')
eq(totalCalls(), 0)

local SECRET = 'k3-vale-of-ash'
local MAX = 2147483647
-- A codec of one version with these fields.
local function codec(fields, secret) return Codec.new({version = 1, secret = secret or SECRET, schemas = {
    {version = 1, fields = fields}}}) end
local function int(key, min, max) return {key = key, kind = 'integer', min = min, max = max} end
-- Deep equality of two values made of tables, strings, numbers and booleans.
local function same(actual, expected, path)
    path = path or 'data'
    if type(expected) ~= 'table' then
        if actual ~= expected or math.type(actual) ~= math.type(expected) then
            error(path .. ': expected ' .. tostring(expected) .. ', got ' .. tostring(actual), 2)
        end
        return
    end
    assert(type(actual) == 'table', path .. ': expected a table')
    for key, value in pairs(expected) do same(actual[key], value, path .. '.' .. tostring(key)) end
    for key in pairs(actual) do assert(expected[key] ~= nil, path .. ': unexpected key ' .. tostring(key)) end
end

local HERO = {int('gold', 0, 1000000), {key = 'hero', kind = 'string', maxLength = 16},
    {key = 'hardMode', kind = 'boolean'},
    {key = 'items', kind = 'list', maxLength = 6, of = {kind = 'integer', min = 0, max = MAX}}}
local EDGES = {int('low', -MAX, 0), int('high', 0, MAX), int('fixed', 7, 7),
    {key = 'flags', kind = 'list', maxLength = 5, of = {kind = 'boolean'}},
    {key = 'names', kind = 'list', maxLength = 3, of = {kind = 'string', maxLength = 4}}}

test('a code round-trips every kind of value', function()
    local saves = codec(HERO)
    local data = {gold = 500, hero = 'Hpal', hardMode = true, items = {1227894832, 1227894833, MAX}}
    local code = saves:encode(data, 'WorldEdit')
    same(saves:decode(code, 'WorldEdit'), data)
    assert(code:find('^[A-Za-z0-9_-]+$'), code)
    -- Empty strings and lists, false, and every byte in a string.
    local bytes = {}
    for byte = 0, 255 do bytes[#bytes + 1] = string.char(byte) end
    local wide = codec({{key = 'text', kind = 'string', maxLength = 256},
        {key = 'none', kind = 'string', maxLength = 0},
        {key = 'off', kind = 'boolean'}, {key = 'empty', kind = 'list', maxLength = 4, of = {kind = 'boolean'}},
        {key = 'words', kind = 'list', maxLength = 2, of = {kind = 'string', maxLength = 3}}})
    data = {text = table.concat(bytes), none = '', off = false, empty = {}, words = {'', 'abc'}}
    same(wide:decode(wide:encode(data)), data)
    -- The data given is not kept or changed, and a whole float is stored as its integer.
    local given = {gold = 3.0, hero = '', hardMode = false, items = {2.0}}
    local decoded = saves:decode(saves:encode(given))
    same(decoded, {gold = 3, hero = '', hardMode = false, items = {2}})
    eq(math.type(given.gold), 'float'); eq(math.type(given.items[1]), 'float')
end)

test('integers round-trip at the exact edges of their ranges', function()
    for _, range in ipairs({{-MAX, 0}, {0, MAX}, {-MAX, -MAX}, {7, 7}, {-5, 5}, {-1073741824, 1073741823},
        {MAX - 1, MAX}, {65535, 65536 + 65535}}) do
        local min, max = range[1], range[2]
        local saves = codec({int('value', min, max)})
        local problem = 'expected a whole number from ' .. min .. ' to ' .. max
        for _, value in ipairs({min, max, min + (max - min) // 2}) do
            eq(saves:decode(saves:encode({value = value})).value, value)
        end
        if min > -MAX then
            failsAt(function() saves:encode({value = min - 1}) end, problem)
        end
        if max < MAX then
            failsAt(function() saves:encode({value = max + 1}) end, problem)
        end
    end
end)

test('values take the bits their range needs, in 64 symbols', function()
    -- 20 header bits and 20 for the gold: 40 bits are 7 symbols, and 5 more for the check value.
    local gold = codec({int('gold', 0, 1000000)})
    eq(#gold:encode({gold = 1000000}), 12); eq(gold:getMaxLength(), 12)
    eq(gold:encode({gold = 0}):sub(1, 3), 'BAA') -- layout 1, then the version in 14 bits
    -- A fixed value takes no bits; a boolean one; a range of 2 to 3 one.
    eq(codec({int('fixed', 7, 7)}):getMaxLength(), 4 + 5)
    eq(codec({{key = 'on', kind = 'boolean'}, int('small', 2, 3), {key = 'off', kind = 'boolean'},
        int('more', 0, 1)}):getMaxLength(), 4 + 5)
    eq(codec({{key = 'on', kind = 'boolean'}, int('small', 2, 3), {key = 'off', kind = 'boolean'},
        int('more', 0, 1), int('last', 0, 1)}):getMaxLength(), 5 + 5)
    -- The spec's example: ten numbers and a 16-byte name.
    local fields = {{key = 'name', kind = 'string', maxLength = 16}}
    for index = 1, 10 do fields[#fields + 1] = int('n' .. index, 0, 1000000) end
    local ten = codec(fields)
    local data = {name = 'SixteenBytesLong'}
    for index = 1, 10 do data['n' .. index] = 1000000 end
    eq(ten:getMaxLength(), 64); eq(#ten:encode(data), 64)
    data.name = ''
    eq(#ten:encode(data), 43) -- a shorter string makes a shorter code
    -- A list's longest code counts every value.
    eq(codec(HERO):getMaxLength(), 66)
    -- The longest code counts every version: an older code may be longer than a current one.
    eq(Codec.new({version = 2, secret = SECRET, schemas = {{version = 2, fields = {}},
        {version = 1, fields = {int('gold', 0, 1000000)}, migrate = function() return {} end}}}):getMaxLength(), 12)
    eq(codec({}):getVersion(), 1)
end)

test('fixed codes: the same in every Lua, and as an independent implementation of the layout gives', function()
    eq(codec(HERO):encode({gold = 500, hero = 'Hpal', hardMode = true, items = {1227894832, 1227894833, MAX}},
        'WorldEdit'), 'BAAQAfQiQ4MLZckwMDCSYGBj_____8WQGq')
    local edges = Codec.new({version = 2, secret = SECRET, schemas = {{version = 2, fields = EDGES}}})
    local code = edges:encode({low = -MAX, high = MAX, fixed = 7, flags = {true, false, true},
        names = {'', 'ab', '\0\255\128\n'}})
    eq(code, 'BAAgAAAAH____93CYWKAH_ABQ-Isnq')
    same(edges:decode(code), {low = -MAX, high = MAX, fixed = 7, flags = {true, false, true},
        names = {'', 'ab', '\0\255\128\n'}})
    local empty = Codec.new({version = 9999, secret = 'x', schemas = {{version = 9999, fields = {}}}})
    eq(empty:encode({}, 'Player \xc3\x85ke'), 'BnDwdyUOJ')
end)

test('encode raises at the caller for data that does not fit, and names the field', function()
    local saves = codec(HERO)
    local function with(key, value)
        local data = {gold = 1, hero = 'a', hardMode = false, items = {}}
        data[key] = value
        return function() saves:encode(data) end
    end
    failsAt(with('gold', nil), '[systems] Codec.encode: field "gold": expected a whole number from 0 to 1000000')
    failsAt(with('gold', 1.5), 'field "gold": expected a whole number')
    failsAt(with('gold', '5'), 'field "gold": expected a whole number')
    failsAt(with('gold', 0 / 0), 'field "gold": expected a whole number')
    failsAt(with('gold', math.huge), 'field "gold": expected a whole number')
    failsAt(with('gold', 1000001), 'field "gold": expected a whole number')
    failsAt(with('gold', -1), 'field "gold": expected a whole number')
    failsAt(with('hero', 5), 'field "hero": expected a string of at most 16 bytes')
    failsAt(with('hero', string.rep('x', 17)), 'field "hero": expected a string of at most 16 bytes')
    failsAt(with('hardMode', 1), 'field "hardMode": expected true or false')
    failsAt(with('items', 'none'), 'field "items": expected a list of at most 6 values')
    failsAt(with('items', {1, 2, 3, 4, 5, 6, 7}), 'field "items": expected a list of at most 6 values')
    failsAt(with('items', {1, nil, 3}), 'field "items": expected a list of at most 6 values')
    failsAt(with('items', {1, count = 1}), 'field "items": expected a list of at most 6 values')
    failsAt(with('items', {1, -1}), 'field "items": value 2: expected a whole number from 0 to 2147483647')
    -- A key the schema does not have: the first in sorted order.
    failsAt(with('glod', 5), '[systems] Codec.encode: unknown field "glod"')
    local stray = {gold = 1, hero = 'a', hardMode = false, items = {}, [3] = 4}
    for index = 1, 40 do stray['key' .. index] = index end
    failsAt(function() saves:encode(stray) end, 'unknown field "3"')
    failsAt(function() saves:encode('gold') end, '[systems] Codec.encode: expected a data table')
    failsAt(function() saves:encode({gold = 1, hero = 'a', hardMode = false, items = {}}, 5) end,
        '[systems] Codec.encode: expected a binding string')
    -- The first field in schema order is reported.
    failsAt(function() saves:encode({}) end, 'field "gold"')
end)

test('decode reports a bad code without raising', function()
    local saves = codec(HERO)
    local data = {gold = 500, hero = 'Hpal', hardMode = true, items = {7}}
    local code = saves:encode(data, 'WorldEdit')
    local function reason(text, binding, decoder)
        local decoded, why, detail = (decoder or saves):decode(text, binding or 'WorldEdit')
        eq(decoded, nil); eq(detail, nil)
        return why
    end
    eq(reason(nil), 'format'); eq(reason(42), 'format'); eq(reason({}), 'format')
    eq(reason(''), 'format'); eq(reason('BAAQAAAA'), 'format') -- 8 symbols: one short of the shortest code
    eq(reason(code .. '!'), 'format'); eq(reason(code:sub(1, 3) .. ' ' .. code:sub(5)), 'format')
    eq(reason(code .. '\n'), 'format'); eq(reason(string.rep('A', 8193)), 'format')
    -- A valid check value, but a layout number this library does not know.
    eq(reason('AAAQgVggv', ''), 'format')
    -- Any changed symbol, another binding and another secret fail the check.
    for index = 1, #code do
        local symbol = code:sub(index, index) == 'A' and 'B' or 'A'
        eq(reason(code:sub(1, index - 1) .. symbol .. code:sub(index + 1)), 'checksum')
    end
    eq(reason(code, 'worldedit'), 'checksum'); eq(reason(code, ''), 'checksum')
    eq(reason(code, 'WorldEdit', codec(HERO, SECRET .. '!')), 'checksum')
    eq(reason(code:sub(1, -2)), 'checksum'); eq(reason(code .. 'A'), 'checksum')
    eq(reason(string.rep('A', 8192)), 'checksum')
    -- The binding and the body are told apart: "ab" + "c" is not "a" + "bc".
    local plain = codec({})
    assert(plain:encode({}, 'ab') ~= plain:encode({}, 'a'))
    assert(codec({}, 'ab'):encode({}, 'c') ~= codec({}, 'a'):encode({}, 'bc'))
    eq(plain:encode({}), plain:encode({}, ''))
    failsAt(function() saves:decode(code, 5) end, '[systems] Codec.decode: expected a binding string')
    same(saves:decode(code, 'WorldEdit'), data)
end)

test('a version without a schema is reported', function()
    local two = Codec.new({version = 2, secret = SECRET, schemas = {{version = 2, fields = {}}}})
    local newer = two:encode({})
    local decoded, why = codec({}):decode(newer)
    eq(decoded, nil); eq(why, 'version') -- a newer code than the codec knows
    local three = Codec.new({version = 3, secret = SECRET, schemas = {{version = 3, fields = {}},
        {version = 1, fields = {}, migrate = function(data) return data end}}})
    decoded, why = three:decode(newer)
    eq(decoded, nil); eq(why, 'version') -- an older one whose schema is gone
end)

test('bits that do not fit the schema are reported', function()
    local function why(writer, data, reader)
        local decoded, reason = codec(reader):decode(codec(writer):encode(data))
        eq(decoded, nil)
        return reason
    end
    eq(why({int('a', 0, 10)}, {a = 10}, {int('a', 0, 10), {key = 'b', kind = 'boolean'}}), 'schema') -- bits missing
    eq(why({int('a', 0, 10)}, {a = 10}, {}), 'schema') -- bits left over in the last symbol
    eq(why({int('a', 0, 1000000)}, {a = 0}, {}), 'schema') -- a symbol left over
    eq(why({int('a', 0, 15)}, {a = 15}, {int('a', 0, 10)}), 'schema') -- out of the range
    eq(why({{key = 's', kind = 'string', maxLength = 15}}, {s = string.rep('x', 12)},
        {{key = 's', kind = 'string', maxLength = 10}}), 'schema') -- longer than the string may be
    eq(why({{key = 's', kind = 'string', maxLength = 15}}, {s = string.rep('x', 12)},
        {{key = 's', kind = 'list', maxLength = 10, of = {kind = 'boolean'}}}), 'schema')
    eq(why({{key = 'l', kind = 'list', maxLength = 3, of = {kind = 'integer', min = 0, max = 15}}}, {l = {1, 15}},
        {{key = 'l', kind = 'list', maxLength = 3, of = {kind = 'integer', min = 0, max = 10}}}), 'schema')
    eq(why({{key = 'l', kind = 'list', maxLength = 3, of = {kind = 'boolean'}}}, {l = {true, true, true}},
        {{key = 'l', kind = 'list', maxLength = 3, of = {kind = 'string', maxLength = 200}}}), 'schema')
    -- The same bits under another schema of the same shape decode: only the version tells schemas apart.
    same(codec({int('b', 0, 15)}):decode(codec({int('a', 0, 15)}):encode({a = 9})), {b = 9})
end)

test('older codes migrate one version at a time', function()
    local one = codec({int('gold', 0, 100)})
    local old = one:encode({gold = 40}, 'Ann')
    local calls = {}
    local function schemas(overrides)
        local list = {
            {version = 1, fields = {int('gold', 0, 100)}, migrate = function(data)
                calls[#calls + 1] = 'one'
                return {gold = data.gold, gems = 2.0}
            end},
            {version = 2, fields = {int('gold', 0, 100), int('gems', 0, 9)}, migrate = function(data)
                calls[#calls + 1] = 'two'
                return {coins = data.gold * 10 + data.gems}
            end},
            {version = 3, fields = {int('coins', 0, 100000)}},
        }
        for version, override in pairs(overrides or {}) do
            for key, value in pairs(override) do list[version][key] = value ~= 'none' and value or nil end
        end
        return list
    end
    local three = Codec.new({version = 3, secret = SECRET, schemas = schemas()})
    local data = three:decode(old, 'Ann')
    same(data, {coins = 402}); eq(table.concat(calls, ','), 'one,two')
    same(three:decode(three:encode({coins = 5})), {coins = 5}) -- a current code is not migrated
    eq(#calls, 2)
    eq(three:getVersion(), 3)
    local function failure(overrides, version)
        local decoded, why, detail = Codec.new({version = version or 3, secret = SECRET,
            schemas = schemas(overrides)}):decode(old, 'Ann')
        eq(decoded, nil); eq(why, 'migration')
        return detail
    end
    eq(failure({[1] = {migrate = 'none'}}), 'schema 1 has no migrate function')
    assert(failure({[2] = {migrate = function() error('boom') end}}):find('boom$'))
    eq(failure({[1] = {migrate = function() return {gold = 1} end}}),
        'version 2: field "gems": expected a whole number from 0 to 9')
    eq(failure({[1] = {migrate = function() return 'gold' end}}), 'version 2: expected a data table')
    eq(failure({[1] = {migrate = function() return {gold = 1, gems = 1, extra = true} end}}),
        'version 2: unknown field "extra"')
    -- A schema missing between the code's version and the current one.
    local decoded, why, detail = Codec.new({version = 3, secret = SECRET, schemas = {schemas()[1], schemas()[3]}})
        :decode(old, 'Ann')
    eq(decoded, nil); eq(why, 'migration'); eq(detail, 'no schema for version 2')
end)

test('new checks its options at the caller', function()
    local function with(overrides)
        local options = {version = 1, secret = SECRET, schemas = {{version = 1, fields = {}}}}
        for key, value in pairs(overrides) do options[key] = value ~= 'none' and value or nil end
        return function() Codec.new(options) end
    end
    local function field(given) return with({schemas = {{version = 1, fields = {given}}}}) end
    failsAt(function() Codec.new() end, '[systems] Codec.new: expected an options table')
    for _, version in ipairs({'none', 0, 10000, 1.5, '1'}) do
        failsAt(with({version = version}), '[systems] Codec.new: expected a version from 1 to 9999')
    end
    for _, secret in ipairs({'none', '', 5}) do
        failsAt(with({secret = secret}), '[systems] Codec.new: expected a secret: a non-empty string')
    end
    failsAt(with({schemas = 'none'}), '[systems] Codec.new: expected a list of schemas')
    failsAt(with({schemas = {}}), '[systems] Codec.new: no schema for the current version 1')
    failsAt(with({schemas = {'one'}}), '[systems] Codec.new: expected a schema table')
    for _, version in ipairs({'none', 0, 2, 1.5}) do
        failsAt(with({schemas = {{version = version, fields = {}}}}), 'expected a schema version from 1 to 1')
    end
    failsAt(with({schemas = {{version = 1, fields = {}}, {version = 1, fields = {}}}}), 'schema 1 is given twice')
    failsAt(with({schemas = {{version = 1, fields = {}, migrate = true}}}),
        'schema 1: expected migrate to be a function')
    failsAt(with({schemas = {{version = 1}}}), 'schema 1: expected a list of at most 64 fields')
    local many = {}
    for index = 1, 65 do many[index] = {key = 'k' .. index, kind = 'boolean'} end
    failsAt(with({schemas = {{version = 1, fields = many}}}), 'schema 1: expected a list of at most 64 fields')
    many[65] = nil
    Codec.new({version = 1, secret = SECRET, schemas = {{version = 1, fields = many}}})
    failsAt(field('gold'), 'schema 1, field 1: expected a field table')
    failsAt(field({kind = 'boolean'}), 'schema 1, field 1: expected a key: a non-empty string')
    failsAt(field({key = '', kind = 'boolean'}), 'schema 1, field 1: expected a key: a non-empty string')
    failsAt(field({key = 5, kind = 'boolean'}), 'schema 1, field 1: expected a key: a non-empty string')
    failsAt(with({schemas = {{version = 1, fields = {{key = 'a', kind = 'boolean'}, {key = 'a', kind = 'boolean'}}}}}),
        'schema 1, field "a": the key is used twice')
    failsAt(field({key = 'a', kind = 'number'}),
        'schema 1, field "a": expected kind: "integer", "boolean", "string" or "list"')
    for _, range in ipairs({{nil, 5}, {0, nil}, {0.5, 5}, {5, 4}, {-MAX - 1, 0}, {0, MAX + 1}, {'0', 5}}) do
        failsAt(field({key = 'a', kind = 'integer', min = range[1], max = range[2]}),
            'schema 1, field "a": expected min and max: whole numbers within -2147483647 to 2147483647')
    end
    failsAt(field(int('a', -1, MAX)), 'schema 1, field "a": max - min is over 2147483647')
    failsAt(field(int('a', -MAX, 1)), 'schema 1, field "a": max - min is over 2147483647')
    for _, kind in ipairs({'string', 'list'}) do
        for _, maxLength in ipairs({'none', -1, 4096, 1.5}) do
            failsAt(field({key = 'a', kind = kind, maxLength = maxLength ~= 'none' and maxLength or nil,
                of = {kind = 'boolean'}}), 'schema 1, field "a": expected maxLength: a whole number from 0 to 4095')
        end
    end
    failsAt(field({key = 'a', kind = 'list', maxLength = 2}), 'schema 1, field "a": of: expected a field table')
    failsAt(field({key = 'a', kind = 'list', maxLength = 2,
        of = {kind = 'list', maxLength = 2, of = {kind = 'boolean'}}}),
        'schema 1, field "a": of: a list cannot hold lists')
    failsAt(field({key = 'a', kind = 'list', maxLength = 2, of = {kind = 'integer', min = 3, max = 2}}),
        'schema 1, field "a": of: expected min and max')
    -- The longest code is limited, for every schema.
    local long = {key = 'a', kind = 'string', maxLength = 4095}
    eq(Codec.new({version = 1, secret = SECRET, schemas = {{version = 1, fields = {long}}}}):getMaxLength(), 5471)
    failsAt(with({schemas = {{version = 1, fields = {long, {key = 'b', kind = 'string', maxLength = 2040}}}}}),
        'schema 1: its longest code would pass 8192 symbols')
    eq(Codec.new({version = 1, secret = SECRET, schemas = {{version = 1, fields = {long,
        {key = 'b', kind = 'string', maxLength = 2037}}}}}):getMaxLength(), 8189)
    failsAt(with({version = 2, schemas = {{version = 2, fields = {}}, {version = 1, fields = {
        {key = 'a', kind = 'list', maxLength = 4095, of = {kind = 'string', maxLength = 4095}}}}}}),
        'schema 1: its longest code would pass 8192 symbols')
end)

test('a codec keeps its own copy of the schemas, and its methods check their receiver', function()
    local fields = {int('gold', 0, 100)}
    local schema = {version = 1, fields = fields}
    local saves = Codec.new({version = 1, secret = SECRET, schemas = {schema}})
    local code = saves:encode({gold = 5})
    fields[1].max = 1; fields[1].key = 'other'; fields[2] = int('more', 0, 1); schema.version = 9
    eq(saves:encode({gold = 5}), code); eq(saves:decode(code).gold, 5)
    failsAt(function() saves.encode({}, {}) end, '[systems] Codec.encode: expected Codec')
    failsAt(function() saves.decode({}, code) end, '[systems] Codec.decode: expected Codec')
    failsAt(function() saves.getVersion({}) end, '[systems] Codec.getVersion: expected Codec')
    failsAt(function() saves.getMaxLength({}) end, '[systems] Codec.getMaxLength: expected Codec')
end)
```

In `tests/suites.lua`, insert `'codec'` after `'knockback'`.

- [ ] **Step 2:** `yue -e tests/run.lua codec` → `codec: ERROR …module 'systems.codec' not found`.

- [ ] **Step 3: Implement** — `src/systems/codec.lua`:

```lua
local Check = require('systems.internal.check')

---Save codes: data packed by a versioned schema into 64 symbols, with a keyed check value and migrations (spec
---2026-10-01 release 5 §4). Pure: it calls no native, so a code is the same on every machine.
---@class MoonwellSystems.Codec
---@field package version integer
---@field package secret string
---@field package schemas table<integer, MoonwellSystems.CodecCompiledSchema>
---@field package maxLength integer
local Codec = {}
Codec.__index = Codec

---@class MoonwellSystems.CodecField
---@field key string? The data key; required in a schema's fields, absent in a list's `of`.
---@field kind 'integer'|'boolean'|'string'|'list'
---@field min integer? integer: the smallest value.
---@field max integer? integer: the largest value; `max - min` is at most 2147483647.
---@field maxLength integer? string: the most bytes; list: the most values. 0 to 4095.
---@field of MoonwellSystems.CodecField? list: the kind of every value: integer, boolean or string.

---@class MoonwellSystems.CodecSchema
---@field version integer From 1 to the codec's current version.
---@field fields MoonwellSystems.CodecField[] In encoding order; at most 64.
---@field migrate (fun(data: table): table)? Returns the data of the next version.

---@class MoonwellSystems.CodecOptions
---@field version integer The current version, 1 to 9999; `encode` writes this one.
---@field secret string Mixed into every check value. Keep it to the map.
---@field schemas MoonwellSystems.CodecSchema[] One per supported version, the current one included.

---@class MoonwellSystems.CodecCompiledField
---@field key string?
---@field kind string
---@field min integer
---@field max integer
---@field width integer Bits of an integer's offset, or of a string's or list's length.
---@field maxLength integer
---@field of MoonwellSystems.CodecCompiledField?

---@class MoonwellSystems.CodecCompiledSchema
---@field fields MoonwellSystems.CodecCompiledField[]
---@field keys table<string, true>
---@field migrate (fun(data: table): table)?

local ALPHABET = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_'
local MAX_INT = 2147483647
local MAX_VERSION = 9999
local MAX_FIELDS = 64
local MAX_COUNT = 4095          -- the longest string or list
local MAX_SYMBOLS = 8192        -- the longest code
local LAYOUT = 1
local HEADER_BITS = 20          -- the layout number (6) and the version (14)
local CHECK_SYMBOLS = 5         -- 30 bits of check value
local MAX_BITS = (MAX_SYMBOLS - CHECK_SYMBOLS) * 6

---@type table<integer, integer>
local VALUES = {}
for index = 1, #ALPHABET do VALUES[ALPHABET:byte(index)] = index - 1 end

---The whole number `value` holds, or nil. math.tointeger alone would also convert strings on Lua 5.4.
---@param value unknown
---@return integer?
local function whole(value)
    if type(value) ~= 'number' then return nil end
    return math.tointeger(value)
end

---How many bits a value from 0 to `span` needs.
---@param span integer
---@return integer
local function widthOf(span)
    local width = 0
    while span > 0 do
        width = width + 1
        span = span >> 1
    end
    return width
end

-- Compiling a schema

---@param field unknown
---@param keyed boolean Whether the field is one of a schema's (with a key) or a list's `of`.
---@return MoonwellSystems.CodecCompiledField? compiled
---@return string? problem
local function compileField(field, keyed)
    if type(field) ~= 'table' then return nil, 'expected a field table' end
    local kind = field.kind
    local compiled = {key = field.key, kind = kind, min = 0, max = 0, width = 0, maxLength = 0}
    if kind == 'integer' then
        local min, max = whole(field.min), whole(field.max)
        if not min or not max or min < -MAX_INT or max > MAX_INT or min > max then
            return nil, 'expected min and max: whole numbers within ' .. -MAX_INT .. ' to ' .. MAX_INT
        end
        -- max - min without leaving the 32-bit range: with min below zero, MAX_INT + min cannot overflow.
        if min < 0 and max > MAX_INT + min then return nil, 'max - min is over ' .. MAX_INT end
        compiled.min, compiled.max, compiled.width = min, max, widthOf(max - min)
    elseif kind == 'string' or kind == 'list' then
        local maxLength = whole(field.maxLength)
        if not maxLength or maxLength < 0 or maxLength > MAX_COUNT then
            return nil, 'expected maxLength: a whole number from 0 to ' .. MAX_COUNT
        end
        compiled.maxLength, compiled.width = maxLength, widthOf(maxLength)
        if kind == 'list' then
            if not keyed then return nil, 'a list cannot hold lists' end
            local of, problem = compileField(field.of, false)
            if not of then return nil, 'of: ' .. tostring(problem) end
            compiled.of = of
        end
    elseif kind ~= 'boolean' then
        return nil, 'expected kind: "integer", "boolean", "string" or "list"'
    end
    return compiled
end

---The most bits a field's value takes: at most 134 million, for a full list of full strings.
---@param field MoonwellSystems.CodecCompiledField
---@return integer
local function worstBits(field)
    if field.kind == 'integer' then return field.width end
    if field.kind == 'boolean' then return 1 end
    if field.kind == 'string' then return field.width + 8 * field.maxLength end
    return field.width + field.maxLength * worstBits(field.of)
end

---@param schema unknown
---@param current integer
---@return MoonwellSystems.CodecCompiledSchema? compiled
---@return string|integer detail The problem, or the longest code in symbols.
local function compileSchema(schema, current)
    if type(schema) ~= 'table' then return nil, 'expected a schema table' end
    local version = whole(schema.version)
    if not version or version < 1 or version > current then
        return nil, 'expected a schema version from 1 to ' .. current
    end
    local label = 'schema ' .. version
    if schema.migrate ~= nil and type(schema.migrate) ~= 'function' then
        return nil, label .. ': expected migrate to be a function'
    end
    local fields = schema.fields
    if type(fields) ~= 'table' or #fields > MAX_FIELDS then
        return nil, label .. ': expected a list of at most ' .. MAX_FIELDS .. ' fields'
    end
    local compiled = {fields = {}, keys = {}, migrate = schema.migrate}
    local bits = HEADER_BITS
    for index = 1, #fields do
        local field, problem = compileField(fields[index], true)
        local key = type(fields[index]) == 'table' and fields[index].key or nil
        local shown = type(key) == 'string' and key ~= '' and '"' .. key .. '"' or tostring(index)
        local name = label .. ', field ' .. shown
        if not field then return nil, name .. ': ' .. tostring(problem) end
        if type(key) ~= 'string' or key == '' then return nil, name .. ': expected a key: a non-empty string' end
        if compiled.keys[key] then return nil, name .. ': the key is used twice' end
        compiled.keys[key] = true
        compiled.fields[index] = field
        bits = bits + worstBits(field)
        -- Checked field by field, so the sum stays far inside the 32-bit range.
        if bits > MAX_BITS then
            return nil, label .. ': its longest code would pass ' .. MAX_SYMBOLS .. ' symbols'
        end
    end
    return compiled, (bits + 5) // 6 + CHECK_SYMBOLS
end

-- Values

---The value as the field stores it, or a problem.
---@param field MoonwellSystems.CodecCompiledField
---@param value unknown
---@return any normal
---@return string? problem
local function normal(field, value)
    local kind = field.kind
    if kind == 'integer' then
        local number = whole(value)
        if not number or number < field.min or number > field.max then
            return nil, 'expected a whole number from ' .. field.min .. ' to ' .. field.max
        end
        return number
    elseif kind == 'boolean' then
        if type(value) ~= 'boolean' then return nil, 'expected true or false' end
        return value
    elseif kind == 'string' then
        if type(value) ~= 'string' or #value > field.maxLength then
            return nil, 'expected a string of at most ' .. field.maxLength .. ' bytes'
        end
        return value
    end
    local problem = 'expected a list of at most ' .. field.maxLength .. ' values'
    if type(value) ~= 'table' then return nil, problem end
    local count = #value
    local entries = 0
    for _ in pairs(value) do entries = entries + 1 end
    if entries ~= count or count > field.maxLength then return nil, problem end
    local list = {}
    for index = 1, count do
        local item, why = normal(field.of, value[index])
        if why then return nil, 'value ' .. index .. ': ' .. why end
        list[index] = item
    end
    return list
end

---A copy of `data` as the schema stores it, or a problem that names the field.
---@param schema MoonwellSystems.CodecCompiledSchema
---@param data unknown
---@return table? copy
---@return string? problem
local function fit(schema, data)
    if type(data) ~= 'table' then return nil, 'expected a data table' end
    local unknown = {}
    for key in pairs(data) do
        if not schema.keys[key] then unknown[#unknown + 1] = tostring(key) end
    end
    if #unknown > 0 then
        table.sort(unknown)
        return nil, 'unknown field "' .. unknown[1] .. '"'
    end
    local copy = {}
    for _, field in ipairs(schema.fields) do
        local value, problem = normal(field, data[field.key])
        if problem then return nil, 'field "' .. field.key .. '": ' .. problem end
        copy[field.key] = value
    end
    return copy
end

-- Bits. At most 16 are written or read at once, so no intermediate value leaves the 32-bit range. The writer's
-- mask changes nothing while that holds; it makes a 64-bit Lua fail as the game would if it ever did not.

---@class MoonwellSystems.CodecWriter
---@field symbols string[]
---@field pending integer Bits not yet written as a symbol.
---@field count integer How many bits `pending` holds; under 6 between calls.

---@param writer MoonwellSystems.CodecWriter
---@param value integer From 0 to 2^width - 1.
---@param width integer At most 31.
local function put(writer, value, width)
    if width > 16 then
        put(writer, value >> 16, width - 16)
        value, width = value & 0xFFFF, 16
    end
    local pending, count = ((writer.pending << width) | value) & 0xFFFFFFFF, writer.count + width
    local symbols = writer.symbols
    while count >= 6 do
        count = count - 6
        local index = (pending >> count) & 63
        symbols[#symbols + 1] = ALPHABET:sub(index + 1, index + 1)
    end
    writer.pending, writer.count = pending & ((1 << count) - 1), count
end

---@param writer MoonwellSystems.CodecWriter
---@param field MoonwellSystems.CodecCompiledField
---@param value any
local function write(writer, field, value)
    local kind = field.kind
    if kind == 'integer' then
        put(writer, value - field.min, field.width)
    elseif kind == 'boolean' then
        put(writer, value and 1 or 0, 1)
    elseif kind == 'string' then
        put(writer, #value, field.width)
        for index = 1, #value do put(writer, value:byte(index), 8) end
    else
        put(writer, #value, field.width)
        for index = 1, #value do write(writer, field.of, value[index]) end
    end
end

---@class MoonwellSystems.CodecReader
---@field text string
---@field at integer The next symbol to read.
---@field pending integer
---@field count integer

---@param reader MoonwellSystems.CodecReader
---@param width integer At most 31.
---@return integer? value Nil when the symbols run out.
local function take(reader, width)
    if width > 16 then
        local high = take(reader, width - 16)
        local low = take(reader, 16)
        if not high or not low then return nil end
        return (high << 16) | low
    end
    local pending, count = reader.pending, reader.count
    while count < width do
        local symbol = VALUES[reader.text:byte(reader.at)]
        if not symbol then return nil end
        reader.at = reader.at + 1
        pending, count = (pending << 6) | symbol, count + 6
    end
    count = count - width
    reader.pending, reader.count = pending & ((1 << count) - 1), count
    return (pending >> count) & ((1 << width) - 1)
end

---@param reader MoonwellSystems.CodecReader
---@param field MoonwellSystems.CodecCompiledField
---@return any value Nil when the bits do not fit the field.
local function read(reader, field)
    local kind = field.kind
    if kind == 'integer' then
        local offset = take(reader, field.width)
        -- field.max - field.min cannot overflow: compileField checked it.
        if not offset or offset > field.max - field.min then return nil end
        return field.min + offset
    elseif kind == 'boolean' then
        local bit = take(reader, 1)
        if not bit then return nil end
        return bit == 1
    end
    local count = take(reader, field.width)
    if not count or count > field.maxLength then return nil end
    local values = {}
    if kind == 'string' then
        for index = 1, count do
            local byte = take(reader, 8)
            if not byte then return nil end
            values[index] = string.char(byte)
        end
        return table.concat(values)
    end
    for index = 1, count do
        local value = read(reader, field.of)
        if value == nil then return nil end
        values[index] = value
    end
    return values
end

-- The check value

---FNV-1a over `text`. The multiplication is meant to wrap at 32 bits; the mask makes a 64-bit Lua agree with the
---game's 32-bit one (measured in game, spec §4.2).
---@param hash integer
---@param text string
---@return integer
local function feed(hash, text)
    for index = 1, #text do hash = ((hash ~ text:byte(index)) * 16777619) & 0xFFFFFFFF end
    -- A zero byte ends every part, so "ab" + "c" and "a" + "bc" differ.
    return (hash * 16777619) & 0xFFFFFFFF
end

---30 bits that mix the secret, the binding and the body.
---@param secret string
---@param binding string
---@param body string
---@return integer
local function checkValue(secret, binding, body)
    local hash = feed(feed(feed(0x811c9dc5, secret), binding), body)
    for index = 1, #secret do hash = ((hash ~ secret:byte(index)) * 16777619) & 0xFFFFFFFF end
    -- The murmur3 finalizer: every input bit reaches every output bit.
    hash = hash ~ (hash >> 16)
    hash = (hash * 0x85ebca6b) & 0xFFFFFFFF
    hash = hash ~ (hash >> 13)
    hash = (hash * 0xc2b2ae35) & 0xFFFFFFFF
    hash = hash ~ (hash >> 16)
    return hash & 0x3FFFFFFF
end

---@param value integer 30 bits.
---@return string
local function checkSymbols(value)
    local writer = {symbols = {}, pending = 0, count = 0}
    put(writer, value, 30)
    return table.concat(writer.symbols)
end

-- Codec

---Checks every schema. Raises at the caller for a wrong option.
---@param options MoonwellSystems.CodecOptions
---@return MoonwellSystems.Codec
function Codec.new(options)
    if type(options) ~= 'table' then error('[systems] Codec.new: expected an options table', 2) end
    local version = whole(options.version)
    if not version or version < 1 or version > MAX_VERSION then
        error('[systems] Codec.new: expected a version from 1 to ' .. MAX_VERSION, 2)
    end
    local secret = options.secret
    if type(secret) ~= 'string' or secret == '' then
        error('[systems] Codec.new: expected a secret: a non-empty string', 2)
    end
    local schemas = options.schemas
    if type(schemas) ~= 'table' then error('[systems] Codec.new: expected a list of schemas', 2) end
    local compiled, maxLength = {}, 0
    for index = 1, #schemas do
        local schema, detail = compileSchema(schemas[index], version)
        if not schema then error('[systems] Codec.new: ' .. detail, 2) end
        local number = whole(schemas[index].version) --[[@as integer]]
        if compiled[number] then error('[systems] Codec.new: schema ' .. number .. ' is given twice', 2) end
        compiled[number] = schema
        -- The longest code of any version: an older code may be longer than a current one.
        local symbols = detail --[[@as integer]]
        if symbols > maxLength then maxLength = symbols end
    end
    if not compiled[version] then
        error('[systems] Codec.new: no schema for the current version ' .. version, 2)
    end
    return setmetatable({version = version, secret = secret, schemas = compiled, maxLength = maxLength}, Codec)
end

---The code for `data` under the current schema. Raises at the caller when the data does not fit, naming the field.
---@param data table One value per field of the current schema, and no other key.
---@param binding string? Mixed into the check value, typically the player's name; `decode` needs the same.
---@return string code
function Codec:encode(data, binding)
    local codec = Check.receiver(self, Codec, 'Codec', 'Codec.encode')
    if binding == nil then binding = '' end
    if type(binding) ~= 'string' then error('[systems] Codec.encode: expected a binding string', 2) end
    local schema = codec.schemas[codec.version]
    local copy, problem = fit(schema, data)
    if not copy then error('[systems] Codec.encode: ' .. tostring(problem), 2) end
    local writer = {symbols = {}, pending = 0, count = 0}
    put(writer, LAYOUT, 6)
    put(writer, codec.version, 14)
    for _, field in ipairs(schema.fields) do write(writer, field, copy[field.key]) end
    -- Zero bits fill the last symbol.
    if writer.count > 0 then put(writer, 0, 6 - writer.count) end
    local body = table.concat(writer.symbols)
    return body .. checkSymbols(checkValue(codec.secret, binding, body))
end

---The data of a code, migrated to the current version. It never raises for a bad code: codes come from files and
---from other machines.
---@param code string
---@param binding string? The binding the code was encoded with.
---@return table? data
---@return ('format'|'checksum'|'version'|'schema'|'migration')? reason
---@return string? detail The message of a `migration` failure.
function Codec:decode(code, binding)
    local codec = Check.receiver(self, Codec, 'Codec', 'Codec.decode')
    if binding == nil then binding = '' end
    if type(binding) ~= 'string' then error('[systems] Codec.decode: expected a binding string', 2) end
    if type(code) ~= 'string' or #code < 4 + CHECK_SYMBOLS or #code > MAX_SYMBOLS
        or not code:find('^[A-Za-z0-9_-]+$') then
        return nil, 'format'
    end
    local body = code:sub(1, -CHECK_SYMBOLS - 1)
    if checkSymbols(checkValue(codec.secret, binding, body)) ~= code:sub(-CHECK_SYMBOLS) then
        return nil, 'checksum'
    end
    local reader = {text = body, at = 1, pending = 0, count = 0}
    if take(reader, 6) ~= LAYOUT then return nil, 'format' end
    local version = take(reader, 14)
    local schema = codec.schemas[version]
    if not schema then return nil, 'version' end
    local data = {}
    for _, field in ipairs(schema.fields) do
        local value = read(reader, field)
        if value == nil then return nil, 'schema' end
        data[field.key] = value
    end
    -- Nothing may be left but the zero bits that filled the last symbol.
    if reader.at <= #body or reader.pending ~= 0 then return nil, 'schema' end
    while version < codec.version do
        local migrate = schema.migrate
        if not migrate then return nil, 'migration', 'schema ' .. version .. ' has no migrate function' end
        local ok, migrated = pcall(migrate, data)
        if not ok then return nil, 'migration', tostring(migrated) end
        version = version + 1
        schema = codec.schemas[version]
        if not schema then return nil, 'migration', 'no schema for version ' .. version end
        local copy, problem = fit(schema, migrated)
        if not copy then return nil, 'migration', 'version ' .. version .. ': ' .. tostring(problem) end
        data = copy
    end
    return data
end

---The current version: the one `encode` writes.
---@return integer
function Codec:getVersion() return Check.receiver(self, Codec, 'Codec', 'Codec.getVersion').version end

---The longest code any of its schemas can produce, in symbols: the longest it writes or reads.
---@return integer
function Codec:getMaxLength() return Check.receiver(self, Codec, 'Codec', 'Codec.getMaxLength').maxLength end

return Codec
```

- [ ] **Step 4:** `yue -e tests/run.lua; echo "exit $?"` → `codec: SUITE PASSED: 11 tests`, `All 18 suites passed`,
  `exit 0`.

- [ ] **Step 5: Commit** `src/systems/codec.lua tests/codec.lua tests/suites.lua` — `feat: systems.codec`.

---

### Task 2: Local files (`systems.internal.preload`)

**Files:** Create `src/systems/internal/preload.lua`, `tests/preload.lua`; modify `tests/suites.lua`.

**Interfaces:** Produces, unchecked, `Files.capacity(abilities)`, `Files.verify(abilities)` (the first ability that
cannot carry text, or nil), `Files.write(path, abilities, code)` and `Files.read(path, abilities)` (the code, or
`nil, 'missing'` or `nil, 'damaged'`).

- [ ] **Step 1: Failing tests** — `tests/preload.lua`. Its `Preloader` runs a file's lines as the game does, and
  refuses a line that is not one whole tooltip call:

```lua
-- The game, as far as save files go: tooltips per ability, the lines buffered for a file, and the files written.
-- Preloader runs a file's lines as the game does: each must be one whole tooltip call, or the game could not run it.
local texts, files, buffer, dead = {}, {}, {}, {}
local function field(name, key)
    native('BlzGetAbility' .. name, function(ability, level)
        eq(level, 0)
        return texts[key .. ability]
    end)
    native('BlzSetAbility' .. name, function(ability, text, level)
        eq(level, 0); eq(type(text), 'string')
        if not dead[key .. ability] then texts[key .. ability] = text end
    end)
end
field('Tooltip', 't'); field('ExtendedTooltip', 'e')
native('PreloadGenClear', function() buffer = {} end)
native('PreloadGenStart', function() end)
-- Preload keeps 259 characters (measured in game).
native('Preload', function(text) buffer[#buffer + 1] = text:sub(1, 259) end)
native('PreloadGenEnd', function(path) files[path] = buffer end)
native('Preloader', function(path)
    for _, line in ipairs(files[path] or {}) do
        local name, ability, text = line:match('^"%)\ncall (BlzSetAbility%a+)%((%d+), "([^"]*)", 0%)\n//$')
        assert(name, 'a line the game cannot run: ' .. line)
        _G[name](math.tointeger(tonumber(ability)), text, 0)
    end
end)
local Files = require('systems.internal.preload')
eq(totalCalls(), 0)

local A, B, C = 1097690227, 1097035619, 1097689443
-- Every test starts with setup(): no files, and a tooltip and an extended tooltip of its own for each ability.
local function setup()
    texts, files, buffer, dead = {}, {}, {}, {}
    for _, ability in ipairs({A, B, C}) do
        texts['t' .. ability], texts['e' .. ability] = 'tip ' .. ability, 'more ' .. ability
    end
    resetCalls()
end
local function code(length)
    local symbols = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_'
    local parts = {}
    for index = 1, length do
        local at = (index * 13) % 64 + 1
        parts[index] = symbols:sub(at, at)
    end
    return table.concat(parts)
end
local function restored()
    for _, ability in ipairs({A, B, C}) do
        eq(texts['t' .. ability], 'tip ' .. ability); eq(texts['e' .. ability], 'more ' .. ability)
    end
end
-- A file whose lines set the carriers of {A, B, C} to `chunks`, in order.
local function plant(path, chunks)
    local lines = {}
    for index, chunk in ipairs(chunks) do
        local ability = ({A, B, C})[(index - 1) // 2 + 1]
        local name = index % 2 == 1 and 'BlzSetAbilityTooltip' or 'BlzSetAbilityExtendedTooltip'
        lines[index] = '")\ncall ' .. name .. '(' .. ability .. ', "' .. chunk .. '", 0)\n//'
    end
    files[path] = lines
end

test('a code is written one chunk per ability field, and read back', function()
    setup()
    local saved = code(500)
    Files.write('Vale\\slot1.pld', {A, B}, saved)
    local lines = files['Vale\\slot1.pld']
    eq(#lines, 3) -- "MWS1.500." and 500 symbols are 509 characters
    eq(lines[1], '")\ncall BlzSetAbilityTooltip(' .. A .. ', "MWS1.500.' .. saved:sub(1, 181) .. '", 0)\n//')
    eq(lines[2], '")\ncall BlzSetAbilityExtendedTooltip(' .. A .. ', "' .. saved:sub(182, 371) .. '", 0)\n//')
    eq(lines[3], '")\ncall BlzSetAbilityTooltip(' .. B .. ', "' .. saved:sub(372) .. '", 0)\n//')
    for _, line in ipairs(lines) do assert(#line <= 248, #line) end
    -- The buffer is cleared before and after, so nothing else reaches the file or the next one.
    eq(callCount('PreloadGenClear'), 2); eq(callCount('PreloadGenStart'), 1); eq(callCount('PreloadGenEnd'), 1)
    restored() -- writing touches no tooltip
    resetCalls()
    local read, reason = Files.read('Vale\\slot1.pld', {A, B})
    eq(read, saved); eq(reason, nil)
    expectCall('Preloader', 'Vale\\slot1.pld')
    restored()
    -- A second write replaces the file.
    Files.write('Vale\\slot1.pld', {A, B}, 'short')
    eq(Files.read('Vale\\slot1.pld', {A, B}), 'short')
    eq(#files['Vale\\slot1.pld'], 1)
end)

test('the longest code fills every carrier, and a line stays within what Preload keeps', function()
    setup()
    eq(Files.capacity({A}), 370); eq(Files.capacity({A, B}), 750)
    local abilities = {}
    for index = 1, 24 do abilities[index] = A + index end
    eq(Files.capacity(abilities), 9110)
    local saved = code(750)
    Files.write('Vale\\full.pld', {A, B}, saved)
    eq(#files['Vale\\full.pld'], 4)
    eq(Files.read('Vale\\full.pld', {A, B}), saved)
    -- A text that ends exactly at a chunk's end takes no chunk more: "MWS1.371." and 371 symbols are 380 characters.
    Files.write('Vale\\even.pld', {A, B}, code(371))
    eq(#files['Vale\\even.pld'], 2)
    eq(Files.read('Vale\\even.pld', {A, B}), code(371))
    -- A full chunk in the extended tooltip of the largest ability id makes the longest line.
    Files.write('Vale\\wide.pld', {2147483647, A}, code(500))
    eq(#files['Vale\\wide.pld'][2], 248)
    restored()
end)

test('a missing file reads as missing, and restores the first carrier', function()
    setup()
    local read, reason = Files.read('Vale\\none.pld', {A, B})
    eq(read, nil); eq(reason, 'missing')
    restored()
end)

test('a file that does not hold what its header says reads as damaged, and every carrier is restored', function()
    local function damaged(chunks)
        setup()
        plant('Vale\\bad.pld', chunks)
        local read, reason = Files.read('Vale\\bad.pld', {A, B, C})
        eq(read, nil); eq(reason, 'damaged')
        restored()
    end
    damaged({'MWS2.5.abcde'})                       -- another format
    damaged({'abcde'})                              -- no header
    damaged({'MWS1.abcde'})                         -- no length
    damaged({'MWS1.0.'})                            -- an empty code
    damaged({'MWS1.05.abcde'})                      -- a length with a leading zero
    damaged({'MWS1.12345.abcde'})                   -- a length of five digits
    damaged({'MWS1.9999.' .. code(180)})            -- more than the carriers hold
    damaged({'MWS1.5.abcd'})                        -- shorter than its length says
    damaged({'MWS1.5.abcdef'})                      -- longer
    damaged({'MWS1.5.ab!de'})                       -- a character outside the alphabet
    damaged({'MWS1.5.ab de'})
    damaged({'MWS1.300.' .. code(181)})             -- the second chunk never arrived: the tooltip is still there
    damaged({'MWS1.300.' .. code(181), code(100)})  -- or arrived short
    damaged({'MWS1.300.' .. code(180), code(119)})  -- or the first chunk is short
    -- What write produces, planted by hand, is not damaged.
    setup()
    plant('Vale\\good.pld', {'MWS1.300.' .. code(181), code(119)})
    eq(Files.read('Vale\\good.pld', {A, B, C}), code(181) .. code(119))
    plant('Vale\\edge.pld', {'MWS1.1.x'})
    eq(Files.read('Vale\\edge.pld', {A, B, C}), 'x')
    -- A stale chunk beyond the code's length is ignored.
    plant('Vale\\stale.pld', {'MWS1.5.abcde', 'left over'})
    eq(Files.read('Vale\\stale.pld', {A, B, C}), 'abcde')
    restored()
end)

test('an ability without a tooltip is damaged, not an error', function()
    setup()
    texts['t' .. A] = nil
    dead['t' .. A] = true
    local read, reason = Files.read('Vale\\none.pld', {A})
    eq(read, nil); eq(reason, 'damaged')
    eq(texts['t' .. A], nil); eq(texts['e' .. A], 'more ' .. A)
end)

test('verify finds the first ability that cannot carry text, and restores every text', function()
    setup()
    eq(Files.verify({A, B, C}), nil)
    restored()
    dead['t' .. B] = true
    eq(Files.verify({A, B, C}), B)
    restored()
    setup()
    dead['e' .. C] = true; dead['t' .. B] = true
    eq(Files.verify({A, C, B}), C) -- the first in the list's order, by its extended tooltip alone
    restored()
    setup()
    texts['t' .. A], texts['e' .. A] = nil, nil
    dead['t' .. A], dead['e' .. A] = true, true
    eq(Files.verify({A}), A) -- an ability the game does not have
    eq(Files.verify({}), nil)
end)
```

In `tests/suites.lua`, insert `'preload'` after `'codec'`.

- [ ] **Step 2:** `yue -e tests/run.lua preload` → `preload: ERROR …module 'systems.internal.preload' not found`.

- [ ] **Step 3: Implement** — `src/systems/internal/preload.lua`:

```lua
---Local save files through the Preload natives (spec 2026-10-01 release 5 §6.1). A file is JASS that the game runs
---with Preloader; each of its lines hands one chunk to Lua through the tooltip, or the extended tooltip, of a borrowed
---ability. Raw natives by design (Part 1 §3). Everything here is local to the machine that calls it, and creates no
---handle (measured on 3.0.0.24268). No argument checks: systems.savefile is the checked public module.
local Files = {}

---Characters per line. With its JASS around it, a line's Preload argument is at most 248 characters; Preload keeps
---259, and a cut line can crash the game.
local CHUNK = 190
local MAGIC = 'MWS1'
---The first carrier holds this while the file runs; still there afterwards, no file ran.
local MARKER = 'MWS?'
---The header is the magic, a dot, the code's length (at most 4 digits) and a dot.
local HEADER = #MAGIC + 6

---Carriers are numbered from 0: the tooltip, then the extended tooltip, of each ability in turn.
---@param abilities integer[]
---@param carrier integer
---@return string?
local function get(abilities, carrier)
    local ability = abilities[carrier // 2 + 1]
    if carrier % 2 == 0 then return BlzGetAbilityTooltip(ability, 0) end
    return BlzGetAbilityExtendedTooltip(ability, 0)
end

---@param abilities integer[]
---@param carrier integer
---@param text string
local function set(abilities, carrier, text)
    local ability = abilities[carrier // 2 + 1]
    if carrier % 2 == 0 then
        BlzSetAbilityTooltip(ability, text, 0)
    else
        BlzSetAbilityExtendedTooltip(ability, text, 0)
    end
end

---The longest code that fits the carriers of `abilities`.
---@param abilities integer[]
---@return integer
function Files.capacity(abilities) return #abilities * 2 * CHUNK - HEADER end

---The first ability whose tooltip or extended tooltip does not keep a text, or nil when all do. Every text is
---restored.
---@param abilities integer[]
---@return integer? ability
function Files.verify(abilities)
    for carrier = 0, #abilities * 2 - 1 do
        local original = get(abilities, carrier)
        set(abilities, carrier, MARKER)
        local kept = get(abilities, carrier) == MARKER
        if original then set(abilities, carrier, original) end
        if not kept then return abilities[carrier // 2 + 1] end
    end
    return nil
end

---Writes `code` to the file at `path`, replacing it. `code` holds only the codec's 64 symbols.
---@param path string
---@param abilities integer[]
---@param code string
function Files.write(path, abilities, code)
    local text = MAGIC .. '.' .. #code .. '.' .. code
    -- PreloadGenStart does not drop lines buffered before it: clear first.
    PreloadGenClear()
    PreloadGenStart()
    local carrier = 0
    for at = 1, #text, CHUNK do
        local native = carrier % 2 == 0 and 'BlzSetAbilityTooltip' or 'BlzSetAbilityExtendedTooltip'
        -- The argument closes the generated `call Preload( "` line, adds one of its own and comments out the rest.
        Preload('")\ncall ' .. native .. '(' .. string.format('%d', abilities[carrier // 2 + 1]) .. ', "'
            .. text:sub(at, at + CHUNK - 1) .. '", 0)\n//')
        carrier = carrier + 1
    end
    PreloadGenEnd(path)
    PreloadGenClear()
end

---The code in the carriers after the file ran, or nil for anything `write` does not produce.
---@param abilities integer[]
---@param first string? The first carrier's text.
---@return string?
local function parse(abilities, first)
    if type(first) ~= 'string' then return nil end
    local digits = first:match('^' .. MAGIC .. '%.([1-9]%d?%d?%d?)%.')
    if not digits then return nil end
    local length = math.tointeger(tonumber(digits)) --[[@as integer]]
    local header = #MAGIC + 2 + #digits
    local total = header + length
    if length > Files.capacity(abilities) then return nil end
    local parts = {first}
    for carrier = 1, (total - 1) // CHUNK do
        -- A carrier without a text adds nothing, and the length below then fails.
        parts[#parts + 1] = get(abilities, carrier)
    end
    local text = table.concat(parts)
    if #text ~= total then return nil end
    local code = text:sub(header + 1)
    if not code:find('^[A-Za-z0-9_-]+$') then return nil end
    return code
end

---Runs the file at `path` and returns the code it holds. Every carrier's text is restored before it returns.
---@param path string
---@param abilities integer[]
---@return string? code
---@return ('missing'|'damaged')? reason
function Files.read(path, abilities)
    local count = #abilities * 2
    local originals = {}
    for carrier = 0, count - 1 do originals[carrier] = get(abilities, carrier) end
    set(abilities, 0, MARKER)
    Preloader(path)
    local first = get(abilities, 0)
    local code, reason
    if first == MARKER then
        reason = 'missing'
    else
        code = parse(abilities, first)
        if not code then reason = 'damaged' end
    end
    for carrier = 0, count - 1 do
        if originals[carrier] then set(abilities, carrier, originals[carrier]) end
    end
    return code, reason
end

return Files
```

- [ ] **Step 4:** `yue -e tests/run.lua; echo "exit $?"` → `preload: SUITE PASSED: 6 tests`,
  `All 19 suites passed`, `exit 0`.

- [ ] **Step 5: Commit** `src/systems/internal/preload.lua tests/preload.lua tests/suites.lua` —
  `feat: local save files through Preload`.

---

### Task 3: `systems.sync`

**Files:** Create `src/systems/sync.lua`, `tests/sync.lua`; modify `tests/suites.lua`.

**Interfaces:**
- Consumes `Scheduler` (`after`), the wrappers' `Sync.send`, `Sync.on`, `Sync.off`, and `Player` (`getController`,
  `getSlotState`, `isLocal`).
- Produces `Sync.new(clock, options?)`, `sync:start()`, `sync:ask(player, read, receive)`, `sync:dispose()`.

- [ ] **Step 1: Failing tests** — `tests/sync.lua`:

```lua
bj_MAX_PLAYERS, bj_MAX_PLAYER_SLOTS = 24, 28
MAP_CONTROL_USER, MAP_CONTROL_COMPUTER = {}, {}
PLAYER_SLOT_STATE_PLAYING, PLAYER_SLOT_STATE_EMPTY = {}, {}
-- The game: player slots, which of them this machine is, the sync messages it sent, and the message being delivered.
local slots, here, sent, accepts, message, actions = {}, nil, {}, true, {}, {}
for index = 0, 27 do slots[index] = {controller = MAP_CONTROL_USER, state = PLAYER_SLOT_STATE_PLAYING} end
native('Player', function(index) return slots[index] end)
native('GetLocalPlayer', function() return here end)
native('GetPlayerController', function(player) return player.controller end)
native('GetPlayerSlotState', function(player) return player.state end)
native('CreateTrigger', function() return {prefixes = {}, enabled = true} end)
native('BlzTriggerRegisterPlayerSyncEvent', function(trigger, _, prefix)
    trigger.prefixes[#trigger.prefixes + 1] = prefix
    return {}
end)
native('TriggerAddAction', function(trigger, callback)
    actions[#actions + 1] = {trigger = trigger, callback = callback}
    return {}
end)
native('EnableTrigger', function(trigger) trigger.enabled = true end)
native('DisableTrigger', function(trigger) trigger.enabled = false end)
native('BlzSendSyncData', function(prefix, data)
    if accepts then sent[#sent + 1] = {prefix = prefix, data = data} end
    return accepts
end)
native('GetTriggerPlayer', function() return message.player end)
native('BlzGetTriggerSyncData', function() return message.data end)
local Sync = require('systems.sync')
local Scheduler = require('systems.scheduler')
local Players = require('wrappers.player')
eq(totalCalls(), 0)

-- A message arrives on this machine: the action of every enabled trigger registered for the prefix runs.
local function deliver(data, from, prefix)
    message = {player = from, data = data}
    for _, action in ipairs(actions) do
        if action.trigger.enabled and action.trigger.prefixes[1] == (prefix or 'mwsync') then action.callback() end
    end
end
-- Everything this machine sent arrives, as from the local player.
local function flush(order)
    local packets = sent
    sent = {}
    for position = 1, #packets do
        local packet = packets[order and order[position] or position]
        deliver(packet.data, here, packet.prefix)
    end
end
-- Every test starts with setup(): this machine is player `me`, with a clock of one-second steps and a started system.
local function setup(me, options)
    for index = 0, 27 do slots[index].controller, slots[index].state = MAP_CONTROL_USER, PLAYER_SLOT_STATE_PLAYING end
    here, sent, accepts = slots[me or 0], {}, true
    local clock = Scheduler.new(1)
    local system = Sync.new(clock, options)
    system:start()
    resetCalls()
    return system, clock, Players.fromIndex(0), Players.fromIndex(1)
end
-- A receive callback that records its calls as "text/reason".
local function recorder(log)
    return function(text, reason) log[#log + 1] = tostring(text) .. '/' .. tostring(reason) end
end
local function never() error('read ran on a machine that was not asked') end

test('the asked player\'s own machine reads, sends, and receives with everyone', function()
    local system, clock, me = setup(0)
    local log, reads = {}, 0
    system:ask(me, function() reads = reads + 1; return 'hello' end, recorder(log))
    eq(reads, 1); eq(#log, 0) -- read ran at once; receive never runs inside ask
    eq(#sent, 1); eq(sent[1].prefix, 'mwsync'); eq(sent[1].data, '1.1.1.hello')
    flush()
    eq(table.concat(log, ' '), 'hello/nil')
    eq(clock:getPending(), 0) -- the timeout is cancelled
    for _ = 1, 20 do clock:advance() end
    eq(#log, 1); eq(#PRINTED, 0)
    system:dispose()
end)

test('another machine does not read, and receives the same answer', function()
    local system, clock, asked = setup(1)
    local log = {}
    system:ask(asked, never, recorder(log))
    eq(#sent, 0); eq(#log, 0)
    deliver('1.1.1.hello', slots[0])
    eq(table.concat(log, ' '), 'hello/nil'); eq(clock:getPending(), 0)
    system:dispose()
end)

test('a long text travels in pieces of 220 bytes, which may arrive in any order', function()
    local system, _, me = setup(0)
    local bytes = {}
    for byte = 32, 126 do bytes[#bytes + 1] = string.char(byte) end
    for byte = 128, 255 do bytes[#bytes + 1] = string.char(byte) end
    local text = table.concat(bytes) .. string.rep('x', 277) -- 500 bytes: every byte a text may hold
    local got
    system:ask(me, function() return text end, function(answer, reason) got = {answer, reason} end)
    eq(#sent, 3)
    eq(sent[1].data, '1.1.3.' .. text:sub(1, 220)); eq(sent[2].data, '1.2.3.' .. text:sub(221, 440))
    eq(sent[3].data, '1.3.3.' .. text:sub(441))
    for _, packet in ipairs(sent) do assert(#packet.data <= 255) end
    local first, second, third = sent[1], sent[2], sent[3]
    sent = {}
    deliver(third.data, here); deliver(first.data, here)
    eq(got, nil)
    deliver(second.data, here)
    eq(got[1], text); eq(got[2], nil)
    -- A text of exactly one piece, and of one byte more.
    local sizes = {}
    system:ask(me, function() return string.rep('a', 220) end, function() end)
    sizes[1] = #sent
    system:ask(me, function() return string.rep('a', 221) end, function() end)
    sizes[2] = #sent - sizes[1]
    eq(sizes[1], 1); eq(sizes[2], 2); eq(sent[3].data, '3.2.2.a')
    system:dispose()
end)

test('no value, an empty text and a failing read each have an answer', function()
    local system, _, me = setup(0, {maxLength = 10})
    local log = {}
    local function ask(read)
        system:ask(me, read, recorder(log))
        local packet = sent[#sent].data
        flush()
        return packet
    end
    eq(ask(function() return nil end), '1.0.0.N'); eq(log[1], 'nil/none')
    eq(ask(function() return '' end), '2.0.0.S'); eq(log[2], '/nil')
    eq(#PRINTED, 0)
    eq(ask(function() error('disk on fire') end), '3.0.0.E'); eq(log[3], 'nil/error')
    assert(PRINTED[1]:find('^%[systems%] Sync read failed: .*disk on fire$'), PRINTED[1])
    -- What cannot be sent: another type, a text over maxLength, a byte below 32.
    for index, value in ipairs({5, {}, string.rep('x', 11), 'a\nb', 'a\0b', '\31'}) do
        eq(ask(function() return value end), (3 + index) .. '.0.0.E'); eq(log[3 + index], 'nil/error')
        eq(PRINTED[1 + index],
            '[systems] Sync read failed: expected a string of at most 10 bytes, none below 32')
    end
    eq(ask(function() return string.rep('x', 10) end), '10.1.1.xxxxxxxxxx'); eq(log[10], 'xxxxxxxxxx/nil')
    eq(ask(function() return ' ~\127\255' end), '11.1.1. ~\127\255')
    system:dispose()
end)

test('a slot without a human is absent, on the next step', function()
    for _, change in ipairs({{'controller', MAP_CONTROL_COMPUTER}, {'state', PLAYER_SLOT_STATE_EMPTY}}) do
        local system, clock, me = setup(0)
        slots[0][change[1]] = change[2]
        local log = {}
        system:ask(me, never, recorder(log))
        eq(#log, 0); eq(#sent, 0)
        clock:advance()
        eq(table.concat(log, ' '), 'nil/absent'); eq(clock:getPending(), 0)
        deliver('1.1.1.late', slots[0])
        eq(#log, 1)
        system:dispose()
    end
end)

test('a request without an answer times out, and a late answer is ignored', function()
    local system, clock, asked = setup(1, {timeout = 3})
    local log = {}
    system:ask(asked, never, recorder(log))
    clock:advance(); clock:advance()
    eq(#log, 0)
    deliver('1.1.2.half', slots[0])
    clock:advance()
    eq(table.concat(log, ' '), 'nil/timeout'); eq(clock:getPending(), 0)
    deliver('1.2.2.rest', slots[0]); deliver('1.0.0.N', slots[0])
    eq(#log, 1); eq(#PRINTED, 0)
    -- The default is ten seconds.
    system:dispose()
    system, clock, asked = setup(1)
    system:ask(asked, never, recorder(log))
    for _ = 1, 9 do clock:advance() end
    eq(#log, 1)
    clock:advance()
    eq(log[2], 'nil/timeout')
    system:dispose()
end)

test('only the asked player\'s packets for an open request are taken', function()
    local system, _, asked = setup(1)
    local log = {}
    system:ask(asked, never, recorder(log))
    deliver('1.1.1.forged', slots[1])   -- another player
    deliver('1.0.0.E', slots[2])
    deliver('2.1.1.early', slots[0])    -- a request nobody asked
    deliver('0.1.1.zero', slots[0])
    deliver('01.1.1.padded', slots[0])  -- not a number as this library writes it
    deliver('1234567890.1.1.x', slots[0])
    deliver('hello', slots[0]); deliver('', slots[0]); deliver('1.1.1', slots[0]); deliver('a.1.1.x', slots[0])
    deliver('1.1.1.other prefix', slots[0], 'other')
    eq(#log, 0); eq(#PRINTED, 0)
    deliver('1.1.1.real', slots[0])
    eq(table.concat(log, ' '), 'real/nil')
    deliver('1.1.1.again', slots[0])    -- the request is closed
    eq(#log, 1); eq(#PRINTED, 0)
    system:dispose()
end)

test('a malformed packet from the asked player ends the request with error, once', function()
    local function malformed(packets, options)
        local system, clock, asked = setup(1, options)
        local log = {}
        system:ask(asked, never, recorder(log))
        for index, packet in ipairs(packets) do
            eq(#log, 0, index)
            deliver(packet, slots[0])
        end
        eq(table.concat(log, ' '), 'nil/error')
        eq(PRINTED[1], '[systems] Sync packet failed: a malformed packet for request 1'); eq(#PRINTED, 1)
        eq(clock:getPending(), 0)
        system:dispose()
    end
    malformed({'1.0.0.X'}); malformed({'1.0.0.'}); malformed({'1.0.0.NE'})
    malformed({'1.0.1.x'}); malformed({'1.1.0.x'})
    malformed({'1.2.1.x'})                              -- a piece beyond the count
    malformed({'1.01.1.x'}); malformed({'1.1.01.x'})    -- padded numbers
    malformed({'1.1.2.a', '1.2.3.b'})                   -- the count changed
    malformed({'1.1.2.a', '1.1.2.a'})                   -- a piece twice
    malformed({'1.1.1.'})                               -- an empty piece
    malformed({'1.1.1.' .. string.rep('x', 221)})       -- a piece over 220 bytes
    malformed({'1.1.2.' .. string.rep('x', 220), '1.2.2.' .. string.rep('x', 81)}, {maxLength = 300})
    malformed({'1.1.3.x'}, {maxLength = 440})           -- more pieces than maxLength needs
    malformed({'1.1.1000.x'})
    -- The largest text maxLength allows arrives.
    local system, _, asked = setup(1, {maxLength = 300})
    local log = {}
    system:ask(asked, never, recorder(log))
    deliver('1.1.2.' .. string.rep('x', 220), slots[0]); deliver('1.2.2.' .. string.rep('x', 80), slots[0])
    eq(log[1], string.rep('x', 300) .. '/nil'); eq(#PRINTED, 0)
    system:dispose()
end)

test('requests are numbered in the order asked and answered apart', function()
    local system, _, me, other = setup(0)
    local log = {}
    system:ask(me, function() return 'first' end, recorder(log))
    system:ask(other, never, recorder(log))
    system:ask(me, function() return 'third' end, recorder(log))
    eq(sent[1].data, '1.1.1.first'); eq(sent[2].data, '3.1.1.third')
    flush({2, 1})
    deliver('2.1.1.second', slots[1])
    eq(table.concat(log, ' '), 'third/nil first/nil second/nil')
    -- A callback may ask again.
    local nested = {}
    system:ask(me, function() return 'outer' end, function(text)
        nested[#nested + 1] = text
        system:ask(me, function() return 'inner' end, function(inner) nested[#nested + 1] = inner end)
    end)
    flush(); flush()
    eq(table.concat(nested, ' '), 'outer inner')
    system:dispose()
end)

test('dispose ends the open requests in order, stops listening, and is idempotent', function()
    local system, clock, me, other = setup(0)
    local log = {}
    local function tag(name) return function(text, reason) log[#log + 1] = name .. ':' .. tostring(reason) end end
    system:ask(other, never, tag('a'))
    system:ask(me, function() return 'answered' end, tag('b'))
    system:ask(other, never, tag('c'))
    flush()
    eq(table.concat(log, ' '), 'b:nil')
    system:dispose(); system:dispose()
    eq(table.concat(log, ' '), 'b:nil a:disposed c:disposed')
    eq(clock:getPending(), 0); eq(callCount('DisableTrigger'), 1)
    deliver('1.1.1.late', slots[1])
    eq(#log, 3); eq(#PRINTED, 0)
    failsAt(function() system:ask(me, never, never) end, '[systems] Sync.ask: the system is disposed')
    failsAt(function() system:start() end, '[systems] Sync.start: the system is disposed')
    -- A system that never started disposes too.
    Sync.new(clock):dispose()
end)

test('failures are printed, or go to onError, and nothing is rethrown', function()
    local system, clock, me = setup(0)
    system:ask(me, function() return 'x' end, function() error('receive broke') end)
    flush()
    assert(PRINTED[1]:find('^%[systems%] Sync receive failed: .*receive broke$'), PRINTED[1])
    eq(clock:getPending(), 0)
    system:dispose()
    local reported = {}
    system, clock, me = setup(0, {onError = function(text) reported[#reported + 1] = text end, timeout = 2})
    system:ask(me, function() error('read broke') end, function() error('receive broke') end)
    flush()
    eq(#reported, 2); eq(#PRINTED, 0)
    assert(reported[1]:find('read broke$')); assert(reported[2]:find('receive broke$'))
    -- A message the game refuses is reported; the request then times out.
    accepts = false
    local log = {}
    system:ask(me, function() return 'lost' end, recorder(log))
    eq(reported[3], 'the game refused a sync message'); eq(#sent, 0)
    clock:advance(); clock:advance()
    eq(table.concat(log, ' '), 'nil/timeout')
    system:dispose()
end)

test('new, start and ask check their arguments at the caller', function()
    local clock = Scheduler.new(1)
    failsAt(function() Sync.new({}) end, '[systems] Sync.new: expected Scheduler')
    failsAt(function() Sync.new(clock, 5) end, '[systems] Sync.new: expected an options table')
    for _, prefix in ipairs({'', 5, 'two words', 'dot.ted', string.rep('p', 33)}) do
        failsAt(function() Sync.new(clock, {prefix = prefix}) end,
            '[systems] Sync.new: expected a prefix of 1 to 32 letters, digits, - or _')
    end
    for _, timeout in ipairs({0, -1, 'soon', math.huge}) do
        failsAt(function() Sync.new(clock, {timeout = timeout}) end, '[systems] Sync.new: expected a positive timeout')
    end
    for _, maxLength in ipairs({0, 65536, 1.5, '8'}) do
        failsAt(function() Sync.new(clock, {maxLength = maxLength}) end,
            '[systems] Sync.new: expected maxLength: a whole number from 1 to 65535')
    end
    failsAt(function() Sync.new(clock, {onError = 5}) end, '[systems] Sync.new: expected a callback function')
    here = slots[0]
    local system = Sync.new(clock, {prefix = string.rep('p', 32), maxLength = 65535, timeout = 0.5})
    local me = Players.fromIndex(0)
    failsAt(function() system:ask(me, never, never) end, '[systems] Sync.ask: call start() first')
    system:start(); system:start()
    eq(callCount('CreateTrigger'), 1)
    failsAt(function() system.ask({}, me, never, never) end, '[systems] Sync.ask: expected Sync')
    failsAt(function() system:ask(slots[0], never, never) end, '[systems] Sync.ask: expected Player')
    failsAt(function() system:ask(me, 'read', never) end, '[systems] Sync.ask: expected a callback function')
    failsAt(function() system:ask(me, never, nil) end, '[systems] Sync.ask: expected a callback function')
    failsAt(function() system.start({}) end, '[systems] Sync.start: expected Sync')
    failsAt(function() system.dispose({}) end, '[systems] Sync.dispose: expected Sync')
    eq(clock:getPending(), 0) -- a refused ask opens no request
    system:dispose()
    eq(callCount('DisableTrigger'), 1) -- two starts made one listener, and it is gone
end)
```

In `tests/suites.lua`, insert `'sync'` after `'preload'`.

- [ ] **Step 2:** `yue -e tests/run.lua sync` → `sync: ERROR …module 'systems.sync' not found`.

- [ ] **Step 3: Implement** — `src/systems/sync.lua`:

```lua
local Callback = require('systems.internal.callback')
local Check = require('systems.internal.check')
local Scheduler = require('systems.scheduler')
local Player = require('wrappers.player')
local Messages = require('wrappers.sync')

---Asks one player's machine for a local value and hands the answer to every machine at the same moment (spec
---2026-10-01 release 5 §5): the safe way to share a save file's code, a local clock or anything else that only one
---machine knows. `ask` is called on every machine, like any other game code; only `read` runs on one.
---@class MoonwellSystems.Sync
---@field package clock MoonwellSystems.Scheduler
---@field package prefix string
---@field package timeout number
---@field package maxLength integer
---@field package onError (fun(message: string): ...)?
---@field package requests table<integer, MoonwellSystems.SyncRequest> Open requests by number; never iterated.
---@field package nextRequest integer
---@field package listener MoonwellWrappers.SyncListener? Nil until start().
---@field package disposed boolean
local Sync = {}
Sync.__index = Sync

---@class MoonwellSystems.SyncRequest
---@field id integer
---@field player MoonwellWrappers.Player
---@field receive fun(text: string?, reason: string?): ...
---@field cancel (fun())? Cancels the scheduler task that ends the request.
---@field pieces string[]
---@field count integer The number of pieces, once the first arrived; 0 before.
---@field received integer
---@field length integer

---@class MoonwellSystems.SyncOptions
---@field prefix string? The sync prefix; 1 to 32 letters, digits, `-` or `_`. Default `"mwsync"`.
---@field timeout number? Scheduler seconds a request waits for its answer. Default 10.
---@field maxLength integer? The longest text an answer may be, in bytes; at most 65535. Default 8192.
---@field onError (fun(message: string): ...)?

---Bytes of text per packet. With its header a packet stays under the 255 bytes a sync message keeps.
local PIECE = 220
local MAX_LENGTH = 65535

---Ends an open request: forgets it, cancels its timeout and runs `receive` behind the boundary.
---@param system MoonwellSystems.Sync
---@param request MoonwellSystems.SyncRequest
---@param text string?
---@param reason string?
local function finish(system, request, text, reason)
    system.requests[request.id] = nil
    local cancel = request.cancel
    request.cancel = nil
    if cancel then cancel() end
    Callback.call('Sync receive', system.onError, request.receive, text, reason)
end

---Sends one packet from this machine. A refused send is reported; the request then ends by its timeout.
---@param system MoonwellSystems.Sync
---@param packet string
local function send(system, packet)
    if not Messages.send(system.prefix, packet) then
        Callback.report('Sync send', system.onError, 'the game refused a sync message')
    end
end

---Runs `read` on this machine and sends what it returned: the text in pieces, or one packet that says why there is
---none.
---@param system MoonwellSystems.Sync
---@param request MoonwellSystems.SyncRequest
---@param read fun(): string?
local function answer(system, request, read)
    local id = request.id
    local ok, text = pcall(read)
    if not ok then
        Callback.report('Sync read', system.onError, text)
        send(system, id .. '.0.0.E')
    elseif text == nil then
        send(system, id .. '.0.0.N')
    elseif type(text) ~= 'string' or #text > system.maxLength or text:find('[\0-\31]') then
        Callback.report('Sync read', system.onError,
            'expected a string of at most ' .. system.maxLength .. ' bytes, none below 32')
        send(system, id .. '.0.0.E')
    elseif text == '' then
        send(system, id .. '.0.0.S')
    else
        local count = (#text + PIECE - 1) // PIECE
        for index = 1, count do
            local piece = text:sub((index - 1) * PIECE + 1, index * PIECE)
            send(system, id .. '.' .. index .. '.' .. count .. '.' .. piece)
        end
    end
end

---A packet's number: canonical decimal digits (no leading zero) up to `max`, or nil.
---@param digits string
---@param max integer
---@return integer?
local function number(digits, max)
    if #digits > 9 or (#digits > 1 and digits:sub(1, 1) == '0') then return nil end
    local value = math.tointeger(tonumber(digits))
    if not value or value > max then return nil end
    return value
end

---One packet arrived, on every machine alike.
---@param system MoonwellSystems.Sync
---@param sender MoonwellWrappers.Player The player the engine reports, never one named in the data.
---@param data string
local function arrived(system, sender, data)
    local idText, indexText, countText, payload = data:match('^(%d+)%.(%d+)%.(%d+)%.(.*)$')
    if not idText then return end
    local id = number(idText, 999999999)
    local request = id and system.requests[id]
    -- Only the asked player's packets for an open request are taken.
    if not request or sender ~= request.player then return end
    local index, count = number(indexText, 999), number(countText, 999)
    if count == 0 and index == 0 then
        if payload == 'N' then return finish(system, request, nil, 'none') end
        if payload == 'E' then return finish(system, request, nil, 'error') end
        if payload == 'S' then return finish(system, request, '', nil) end
    elseif index and count and index >= 1 and index <= count and count <= (system.maxLength + PIECE - 1) // PIECE
        and (request.count == 0 or request.count == count) and not request.pieces[index]
        and #payload >= 1 and #payload <= PIECE and request.length + #payload <= system.maxLength then
        request.count = count
        request.pieces[index] = payload
        request.received = request.received + 1
        request.length = request.length + #payload
        if request.received == count then finish(system, request, table.concat(request.pieces, '', 1, count), nil) end
        return
    end
    Callback.report('Sync packet', system.onError, 'a malformed packet for request ' .. request.id)
    finish(system, request, nil, 'error')
end

---@param value unknown
---@return boolean
local function identifier(value)
    return type(value) == 'string' and #value >= 1 and #value <= 32 and value:find('^[A-Za-z0-9_-]+$') ~= nil
end

---@param clock MoonwellSystems.Scheduler Times the requests out; it must run on every machine alike.
---@param options MoonwellSystems.SyncOptions?
---@return MoonwellSystems.Sync
function Sync.new(clock, options)
    Check.receiver(clock, Scheduler, 'Scheduler', 'Sync.new')
    if options == nil then options = {} end
    if type(options) ~= 'table' then error('[systems] Sync.new: expected an options table', 2) end
    local prefix, timeout, maxLength = options.prefix, options.timeout, options.maxLength
    if prefix == nil then prefix = 'mwsync' end
    if not identifier(prefix) then
        error('[systems] Sync.new: expected a prefix of 1 to 32 letters, digits, - or _', 2)
    end
    if timeout == nil then timeout = 10 end
    if not Check.finite(timeout) or timeout <= 0 then error('[systems] Sync.new: expected a positive timeout', 2) end
    if maxLength == nil then maxLength = 8192 end
    if type(maxLength) ~= 'number' or not math.tointeger(maxLength) or maxLength < 1 or maxLength > MAX_LENGTH then
        error('[systems] Sync.new: expected maxLength: a whole number from 1 to ' .. MAX_LENGTH, 2)
    end
    Callback.optional(options.onError, 'Sync.new')
    return setmetatable({
        clock = clock, prefix = prefix, timeout = timeout, maxLength = math.tointeger(maxLength),
        onError = options.onError, requests = {}, nextRequest = 1, disposed = false,
    }, Sync)
end

---Starts listening for answers. Call it once, on every machine, before the first `ask`. Idempotent.
function Sync:start()
    local system = Check.receiver(self, Sync, 'Sync', 'Sync.start')
    if system.disposed then error('[systems] Sync.start: the system is disposed', 2) end
    if system.listener then return end
    system.listener = Messages.on(system.prefix, function(sender, data) arrived(system, sender, data) end)
end

---Asks `player`'s machine for a text. Call it on every machine, in the same order, like any other game code.
---`receive` runs later, never inside this call, and on every machine at the same moment.
---@param player MoonwellWrappers.Player
---@param read fun(): string? Runs on `player`'s machine only, at once. Returns the text (no byte below 32), or nil.
---@param receive fun(text: string?, reason: ('none'|'error'|'absent'|'timeout'|'disposed')?): ...
function Sync:ask(player, read, receive)
    local system = Check.receiver(self, Sync, 'Sync', 'Sync.ask')
    if system.disposed then error('[systems] Sync.ask: the system is disposed', 2) end
    if not system.listener then error('[systems] Sync.ask: call start() first', 2) end
    if getmetatable(player) ~= Player then error('[systems] Sync.ask: expected Player', 2) end
    Callback.check(read, 'Sync.ask')
    Callback.check(receive, 'Sync.ask')
    ---@type MoonwellSystems.SyncRequest
    local request = {id = system.nextRequest, player = player, receive = receive, pieces = {}, count = 0,
        received = 0, length = 0}
    system.nextRequest = request.id + 1
    system.requests[request.id] = request
    -- Both are the same on every machine: whether a human plays in the slot.
    if player:getController() ~= MAP_CONTROL_USER or player:getSlotState() ~= PLAYER_SLOT_STATE_PLAYING then
        request.cancel = system.clock:after(0, function()
            request.cancel = nil
            finish(system, request, nil, 'absent')
        end)
        return
    end
    request.cancel = system.clock:after(system.timeout, function()
        request.cancel = nil
        finish(system, request, nil, 'timeout')
    end)
    if player:isLocal() then answer(system, request, read) end
end

---Ends every open request with 'disposed', in the order they were asked, and stops listening. `ask` and `start`
---raise afterwards. Idempotent.
function Sync:dispose()
    local system = Check.receiver(self, Sync, 'Sync', 'Sync.dispose')
    if system.disposed then return end
    system.disposed = true
    if system.listener then Messages.off(system.listener); system.listener = nil end
    for id = 1, system.nextRequest - 1 do
        local request = system.requests[id]
        if request then finish(system, request, nil, 'disposed') end
    end
end

return Sync
```

- [ ] **Step 4:** `yue -e tests/run.lua; echo "exit $?"` → `sync: SUITE PASSED: 12 tests`, `All 20 suites passed`,
  `exit 0`.

- [ ] **Step 5: Commit** `src/systems/sync.lua tests/sync.lua tests/suites.lua` — `feat: systems.sync`.

---

### Task 4: `systems.savefile`

**Files:** Create `src/systems/savefile.lua`, `tests/savefile.lua`; modify `tests/suites.lua`.

**Interfaces:**
- Consumes `Codec` (`encode`, `decode`, `getMaxLength`), `Sync` (`new`, `start`, `ask`, `dispose`), `Files`
  (`capacity`, `verify`, `write`, `read`) and `Player` (`getName`, `isLocal`).
- Produces `Savefile.new(clock, options)`, `saves:start()`, `saves:save(player, slot, data)`,
  `saves:load(player, slot, callback)`, `saves:dispose()`.

- [ ] **Step 1: Failing tests** — `tests/savefile.lua`:

```lua
bj_MAX_PLAYERS, bj_MAX_PLAYER_SLOTS = 24, 28
MAP_CONTROL_USER, MAP_CONTROL_COMPUTER = {}, {}
PLAYER_SLOT_STATE_PLAYING = {}
-- The game: player slots, which of them this machine is, sync messages, tooltips and this machine's files.
local slots, here, sent, message, actions = {}, nil, {}, {}, {}
local texts, files, buffer, dead = {}, {}, {}, {}
for index = 0, 27 do
    slots[index] = {controller = MAP_CONTROL_USER, state = PLAYER_SLOT_STATE_PLAYING, name = 'Player ' .. index}
end
native('Player', function(index) return slots[index] end)
native('GetLocalPlayer', function() return here end)
native('GetPlayerController', function(player) return player.controller end)
native('GetPlayerSlotState', function(player) return player.state end)
native('GetPlayerName', function(player) return player.name end)
native('CreateTrigger', function() return {prefixes = {}, enabled = true} end)
native('BlzTriggerRegisterPlayerSyncEvent', function(trigger, _, prefix)
    trigger.prefixes[#trigger.prefixes + 1] = prefix
    return {}
end)
native('TriggerAddAction', function(trigger, callback)
    actions[#actions + 1] = {trigger = trigger, callback = callback}
    return {}
end)
native('EnableTrigger', function(trigger) trigger.enabled = true end)
native('DisableTrigger', function(trigger) trigger.enabled = false end)
native('BlzSendSyncData', function(prefix, data)
    sent[#sent + 1] = {prefix = prefix, data = data}
    return true
end)
native('GetTriggerPlayer', function() return message.player end)
native('BlzGetTriggerSyncData', function() return message.data end)
-- Every ability has a tooltip and an extended tooltip of its own, unless it is in `dead`.
local function field(name, key)
    native('BlzGetAbility' .. name, function(ability)
        if dead[ability] then return nil end
        return texts[key .. ability] or key .. ' of ' .. ability
    end)
    native('BlzSetAbility' .. name, function(ability, text)
        if not dead[ability] then texts[key .. ability] = text end
    end)
end
field('Tooltip', 't'); field('ExtendedTooltip', 'e')
native('PreloadGenClear', function() buffer = {} end)
native('PreloadGenStart', function() end)
native('Preload', function(text) buffer[#buffer + 1] = text:sub(1, 259) end)
native('PreloadGenEnd', function(path) files[path] = buffer end)
native('Preloader', function(path)
    for _, line in ipairs(files[path] or {}) do
        local name, ability, text = line:match('^"%)\ncall (BlzSetAbility%a+)%((%d+), "([^"]*)", 0%)\n//$')
        assert(name, 'a line the game cannot run: ' .. line)
        _G[name](math.tointeger(tonumber(ability)), text, 0)
    end
end)
local Savefile = require('systems.savefile')
local Codec = require('systems.codec')
local Scheduler = require('systems.scheduler')
local Players = require('wrappers.player')
eq(totalCalls(), 0)

local AMLS = 1097690227
local function codec(version, extra)
    local schemas = {{version = 1, fields = {{key = 'gold', kind = 'integer', min = 0, max = 100},
        {key = 'items', kind = 'list', maxLength = 3, of = {kind = 'integer', min = 0, max = 2147483647}}},
        migrate = extra}}
    if version == 2 then
        schemas[2] = {version = 2, fields = {{key = 'coins', kind = 'integer', min = 0, max = 1000}}}
    end
    return Codec.new({version = version or 1, secret = 'k3-vale-of-ash', schemas = schemas})
end
-- A message arrives on this machine, and everything this machine sent arrives as from the local player.
local function deliver(data, from, prefix)
    message = {player = from, data = data}
    for _, action in ipairs(actions) do
        if action.trigger.enabled and action.trigger.prefixes[1] == (prefix or 'mwsave') then action.callback() end
    end
end
local function flush()
    local packets = sent
    sent = {}
    for _, packet in ipairs(packets) do deliver(packet.data, here, packet.prefix) end
    return packets
end
-- Every test starts with setup(): this machine is player `me`; its files are kept unless `fresh`. Returns a started
-- system, its clock, and players 0 and 1.
local function setup(me, options, fresh)
    for index = 0, 27 do
        slots[index].controller, slots[index].state = MAP_CONTROL_USER, PLAYER_SLOT_STATE_PLAYING
        slots[index].name = 'Player ' .. index
    end
    here, sent, texts, dead = slots[me or 0], {}, {}, {}
    if fresh ~= false then files = {} end
    local clock = Scheduler.new(1)
    local merged = {codec = codec(), folder = 'Vale'}
    for key, value in pairs(options or {}) do merged[key] = value end
    local saves = Savefile.new(clock, merged)
    saves:start()
    resetCalls()
    return saves, clock, Players.fromIndex(0), Players.fromIndex(1)
end
local function loaded(log)
    return function(data, reason)
        log[#log + 1] = data and ('gold ' .. tostring(data.gold) .. ' items ' .. table.concat(data.items, ','))
            or reason
    end
end
-- A file as save writes it, holding `code`.
local function plant(path, code)
    files[path] = {'")\ncall BlzSetAbilityTooltip(' .. AMLS .. ', "MWS1.' .. #code .. '.' .. code .. '", 0)\n//'}
end
-- Every tooltip reads as it did at first (the stub stores the ones that were ever set).
local function untouched()
    for key, text in pairs(texts) do
        if text ~= key:sub(1, 1) .. ' of ' .. key:sub(2) then error('a tooltip was left changed: ' .. key) end
    end
end
local function fourCC(code) return (string.unpack('>I4', code)) end

test('save writes on the player\'s own machine, and load gives the data to every machine', function()
    local saves, clock, me = setup(0)
    saves:save(me, 'slot1', {gold = 40, items = {7, 8}})
    local lines = files['Vale\\slot1.pld']
    eq(#lines, 1)
    assert(lines[1]:find('^"%)\ncall BlzSetAbilityTooltip%(' .. AMLS .. ', "MWS1%.%d+%.[%w_-]+", 0%)\n//$'), lines[1])
    eq(#sent, 0)
    local log = {}
    saves:load(me, 'slot1', loaded(log))
    eq(#log, 0); eq(callCount('Preloader'), 1); expectCall('Preloader', 'Vale\\slot1.pld')
    local packets = flush()
    eq(table.concat(log, ' | '), 'gold 40 items 7,8'); eq(clock:getPending(), 0); eq(#PRINTED, 0)
    eq(packets[1].prefix, 'mwsave')
    untouched()
    saves:dispose()
    -- Another machine: nothing is written or read there, and the same packets give the same data.
    saves, clock, me = setup(1)
    saves:save(me, 'slot1', {gold = 40, items = {7, 8}})
    eq(next(files), nil); eq(callCount('PreloadGenStart'), 0)
    saves:load(me, 'slot1', loaded(log))
    eq(callCount('Preloader'), 0); eq(#sent, 0)
    for _, packet in ipairs(packets) do deliver(packet.data, slots[0]) end
    eq(log[2], 'gold 40 items 7,8'); eq(clock:getPending(), 0)
    untouched()
    saves:dispose()
end)

test('slots are files of their own, and a second save replaces the first', function()
    local saves, _, me = setup(0)
    saves:save(me, 'a', {gold = 1, items = {}})
    saves:save(me, 'b-2_X', {gold = 2, items = {}})
    saves:save(me, 'a', {gold = 3, items = {9}})
    assert(files['Vale\\a.pld'] and files['Vale\\b-2_X.pld'])
    local log = {}
    saves:load(me, 'b-2_X', loaded(log)); saves:load(me, 'a', loaded(log))
    flush()
    eq(table.concat(log, ' | '), 'gold 2 items  | gold 3 items 9')
    saves:dispose()
end)

test('a load says why there is no data', function()
    local function why(prepare, options, me, keep)
        local saves, clock, player = setup(me or 0, options, not keep)
        local log = {}
        local finish = prepare and prepare(saves, player, clock)
        saves:load(player, 'slot1', loaded(log))
        flush()
        if finish then finish() end
        saves:dispose()
        eq(#log, 1)
        untouched()
        return log[1]
    end
    eq(why(), 'missing')
    eq(why(function() plant('Vale\\slot1.pld', 'not!a!code') end), 'damaged')
    eq(why(function() files['Vale\\slot1.pld'] = {'")\ncall BlzSetAbilityTooltip(' .. AMLS .. ', "junk", 0)\n//'} end),
        'damaged')
    eq(why(function() plant('Vale\\slot1.pld', 'abc') end), 'format')
    -- A code made for another name, and one with a symbol changed.
    eq(why(function(saves, player)
        saves:save(player, 'slot1', {gold = 5, items = {}})
        slots[0].name = 'Somebody else'
    end), 'checksum')
    local good = codec():encode({gold = 5, items = {}}, 'Player 0')
    eq(why(function() plant('Vale\\slot1.pld', good) end), 'gold 5 items ')
    eq(why(function() plant('Vale\\slot1.pld', (good:sub(1, 1) == 'B' and 'C' or 'B') .. good:sub(2)) end), 'checksum')
    -- The codec's other reasons pass through.
    local two = codec(2):encode({coins = 5}, 'Player 0')
    eq(why(function() plant('Vale\\slot1.pld', two) end), 'version')
    local other = Codec.new({version = 1, secret = 'k3-vale-of-ash', schemas = {{version = 1, fields = {}}}})
    eq(why(function() plant('Vale\\slot1.pld', other:encode({}, 'Player 0')) end), 'schema')
    -- No human in the slot; no answer; a system disposed first.
    eq(why(function(_, _, clock)
        slots[0].controller = MAP_CONTROL_COMPUTER
        return function() clock:advance() end
    end), 'absent')
    eq(why(function(_, _, clock)
        return function() for _ = 1, 3 do clock:advance() end end
    end, {timeout = 3}, 1), 'timeout')
    eq(why(nil, nil, 1), 'disposed')
    eq(#PRINTED, 0)
    -- An answer longer than any code of the codec (26 symbols here) is refused before it is decoded.
    local saves, _, asked = setup(1)
    local log = {}
    saves:load(asked, 'slot1', loaded(log))
    deliver('1.1.1.' .. string.rep('A', 27), slots[0])
    eq(log[1], 'error'); eq(PRINTED[1], '[systems] Sync packet failed: a malformed packet for request 1')
    saves:load(asked, 'slot1', loaded(log))
    deliver('2.1.1.' .. string.rep('A', 26), slots[0])
    eq(log[2], 'checksum')
    saves:dispose()
end)

test('an older file is migrated; a failing migration is reported', function()
    local saves, _, me = setup(0)
    saves:save(me, 'slot1', {gold = 40, items = {1, 2}})
    saves:dispose()
    saves, _, me = setup(0, {codec = codec(2, function(data) return {coins = data.gold * 10 + #data.items} end)}, false)
    local got
    saves:load(me, 'slot1', function(data, reason) got = {data, reason} end)
    flush()
    eq(got[1].coins, 402); eq(got[2], nil); eq(#PRINTED, 0)
    saves:dispose()
    saves, _, me = setup(0, {codec = codec(2, function() error('no idea') end)}, false)
    saves:load(me, 'slot1', function(data, reason) got = {data, reason} end)
    flush()
    eq(got[1], nil); eq(got[2], 'migration')
    assert(PRINTED[1]:find('^%[systems%] Savefile migration failed: .*no idea$'), PRINTED[1]); eq(#PRINTED, 1)
    saves:dispose()
end)

test('save raises on every machine alike for data that does not fit', function()
    for me = 0, 1 do
        local saves, _, player = setup(me)
        failsAt(function() saves:save(player, 'slot1', {gold = 101, items = {}}) end,
            '[systems] Savefile.save: field "gold": expected a whole number from 0 to 100')
        failsAt(function() saves:save(player, 'slot1', {gold = 1, items = {}, extra = 1}) end,
            '[systems] Savefile.save: unknown field "extra"')
        failsAt(function() saves:save(player, 'slot1') end, '[systems] Savefile.save: expected a data table')
        eq(next(files), nil)
        saves:dispose()
    end
end)

test('the default abilities hold the longest code a codec may have', function()
    local big = Codec.new({version = 1, secret = 's', schemas = {{version = 1, fields = {
        {key = 'a', kind = 'string', maxLength = 4095}, {key = 'b', kind = 'string', maxLength = 2037}}}}})
    eq(big:getMaxLength(), 8189)
    local saves, _, me = setup(0, {codec = big})
    local data = {a = string.rep('\255', 4095), b = string.rep('z', 2037)}
    saves:save(me, 'big', data)
    local lines = files['Vale\\big.pld']
    eq(#lines, 44) -- "MWS1.8189." and 8189 symbols in chunks of 190
    for _, line in ipairs(lines) do assert(#line <= 248) end
    eq(lines[1]:match('%((%d+),'), tostring(AMLS)); eq(lines[2]:match('%((%d+),'), tostring(AMLS))
    eq(lines[44]:match('%((%d+),'), tostring(fourCC('Afsh'))) -- the 22nd of the default abilities
    local got
    saves:load(me, 'big', function(loaded) got = loaded end)
    eq(#sent, 38) -- 8189 bytes in packets of 220
    flush()
    eq(got.a, data.a); eq(got.b, data.b)
    saves:dispose()
end)

test('start checks every borrowed ability, and save and load need it', function()
    local clock = Scheduler.new(1)
    here, texts, dead = slots[0], {}, {}
    local me = Players.fromIndex(0)
    -- A prefix of its own: the wrappers keep one trigger per prefix, and other tests made the default one.
    local saves = Savefile.new(clock, {codec = codec(), folder = 'Vale', prefix = 'starts'})
    failsAt(function() saves:save(me, 'slot1', {gold = 1, items = {}}) end,
        '[systems] Savefile.save: call start() first')
    failsAt(function() saves:load(me, 'slot1', print) end, '[systems] Savefile.load: call start() first')
    dead[fourCC('Aclf')] = true
    failsAt(function() saves:start() end, "[systems] Savefile.start: ability 'Aclf' cannot carry text")
    failsAt(function() saves:load(me, 'slot1', print) end, '[systems] Savefile.load: call start() first')
    eq(callCount('CreateTrigger'), 0); untouched()
    dead = {}
    saves:start()
    eq(callCount('CreateTrigger'), 1); untouched()
    resetCalls()
    saves:start()
    eq(totalCalls(), 0) -- a second start does nothing
    -- A map's own list replaces the default.
    local own = Savefile.new(clock, {codec = codec(), folder = 'Vale', abilities = {5, 6.0}, prefix = 'own'})
    own:start()
    own:save(me, 'slot1', {gold = 1, items = {}})
    eq(files['Vale\\slot1.pld'][1]:match('%((%d+),'), '5')
    own:dispose(); saves:dispose()
end)

test('a failing callback is reported, and onError receives it', function()
    local saves, _, me = setup(0)
    saves:load(me, 'slot1', function() error('callback broke') end)
    flush()
    assert(PRINTED[1]:find('^%[systems%] Savefile callback failed: .*callback broke$'), PRINTED[1]); eq(#PRINTED, 1)
    saves:dispose()
    local reported = {}
    saves, _, me = setup(0, {onError = function(text) reported[#reported + 1] = text end})
    saves:load(me, 'slot1', function() error('callback broke') end)
    flush()
    eq(#reported, 1); eq(#PRINTED, 0)
    saves:dispose()
end)

test('dispose ends open loads, is idempotent, and later calls raise', function()
    local saves, clock, me, other = setup(0)
    local log = {}
    saves:load(other, 'slot1', loaded(log))
    saves:dispose(); saves:dispose()
    eq(table.concat(log, ' '), 'disposed'); eq(clock:getPending(), 0); eq(callCount('DisableTrigger'), 1)
    failsAt(function() saves:save(me, 'slot1', {gold = 1, items = {}}) end,
        '[systems] Savefile.save: the system is disposed')
    failsAt(function() saves:load(me, 'slot1', print) end, '[systems] Savefile.load: the system is disposed')
    failsAt(function() saves:start() end, '[systems] Savefile.start: the system is disposed')
end)

test('new, save and load check their arguments at the caller', function()
    local clock = Scheduler.new(1)
    local function with(overrides)
        local options = {codec = codec(), folder = 'Vale'}
        for key, value in pairs(overrides) do options[key] = value ~= 'none' and value or nil end
        return function() Savefile.new(clock, options) end
    end
    failsAt(function() Savefile.new({}, {}) end, '[systems] Savefile.new: expected Scheduler')
    failsAt(function() Savefile.new(clock) end, '[systems] Savefile.new: expected an options table')
    failsAt(with({codec = 'none'}), '[systems] Savefile.new: expected a codec')
    failsAt(with({codec = {}}), '[systems] Savefile.new: expected a codec')
    for _, folder in ipairs({'none', '', 'two words', 'a\\b', 'a.b', string.rep('f', 33), 5}) do
        failsAt(with({folder = folder}), '[systems] Savefile.new: expected a folder of 1 to 32 letters, digits, - or _')
    end
    for _, abilities in ipairs({5, {}}) do
        failsAt(with({abilities = abilities}), '[systems] Savefile.new: expected abilities: a list of ability ids')
    end
    for _, abilities in ipairs({{'Amls'}, {1.5}, {0}, {7, 7}, {7, -1}}) do
        failsAt(with({abilities = abilities}),
            '[systems] Savefile.new: expected abilities: a list of ability ids, each once')
    end
    local big = Codec.new({version = 1, secret = 's', schemas = {{version = 1, fields = {
        {key = 'a', kind = 'string', maxLength = 300}}}}})
    failsAt(with({codec = big, abilities = {7}}),
        "[systems] Savefile.new: the codec's longest code is 410 symbols, but the abilities hold 370")
    Savefile.new(clock, {codec = big, folder = 'Vale', abilities = {7, 8}})
    -- An older schema's longer code counts too: a file from that version must still be readable.
    local shrunk = Codec.new({version = 2, secret = 's', schemas = {{version = 2, fields = {}},
        {version = 1, fields = {{key = 'a', kind = 'string', maxLength = 300}}, migrate = function() return {} end}}})
    failsAt(with({codec = shrunk, abilities = {7}}), 'longest code is 410 symbols')
    failsAt(with({prefix = 'two words'}),
        '[systems] Savefile.new: expected a prefix of 1 to 32 letters, digits, - or _')
    failsAt(with({timeout = 0}), '[systems] Savefile.new: expected a positive timeout')
    failsAt(with({onError = 5}), '[systems] Savefile.new: expected a callback function')
    local saves, _, me = setup(0)
    for _, slot in ipairs({'', 'two words', 'a\\b', '..', string.rep('s', 33), 5}) do
        failsAt(function() saves:save(me, slot, {gold = 1, items = {}}) end,
            '[systems] Savefile.save: expected a slot of 1 to 32 letters, digits, - or _')
        failsAt(function() saves:load(me, slot, print) end,
            '[systems] Savefile.load: expected a slot of 1 to 32 letters, digits, - or _')
    end
    failsAt(function() saves:save(slots[0], 'a', {}) end, '[systems] Savefile.save: expected Player')
    failsAt(function() saves:load({}, 'a', print) end, '[systems] Savefile.load: expected Player')
    failsAt(function() saves:load(me, 'a') end, '[systems] Savefile.load: expected a callback function')
    failsAt(function() saves.save({}, me, 'a', {}) end, '[systems] Savefile.save: expected Savefile')
    failsAt(function() saves.load({}, me, 'a', print) end, '[systems] Savefile.load: expected Savefile')
    failsAt(function() saves.start({}) end, '[systems] Savefile.start: expected Savefile')
    failsAt(function() saves.dispose({}) end, '[systems] Savefile.dispose: expected Savefile')
    eq(next(files), nil); eq(#sent, 0)
    saves:dispose()
end)
```

`tests/suites.lua` becomes:

```lua
-- Every behavior suite, in run order. tools/check.lua fails when a suite file is missing here.
return {'internal', 'ordered', 'scheduler', 'signal', 'scope', 'time', 'buffs', 'aura', 'dummy', 'damage', 'vector',
    'geometry', 'terrain', 'missile', 'knockback', 'codec', 'preload', 'sync', 'savefile', 'imports', 'blame'}
```

  (`'imports'` and `'blame'` are already there; this step adds `'savefile'`.)

- [ ] **Step 2:** `yue -e tests/run.lua savefile` → `savefile: ERROR …module 'systems.savefile' not found`.

- [ ] **Step 3: Implement** — `src/systems/savefile.lua`:

```lua
local Callback = require('systems.internal.callback')
local Check = require('systems.internal.check')
local Files = require('systems.internal.preload')
local Codec = require('systems.codec')
local Scheduler = require('systems.scheduler')
local Sync = require('systems.sync')
local Player = require('wrappers.player')

---A player's saved data, in a local file on that player's machine (spec 2026-10-01 release 5 §6). `save` and `load`
---are called on every machine, like any other game code: the steps that run on one machine only are inside, and a
---load's callback runs on every machine at the same moment.
---@class MoonwellSystems.Savefile
---@field package codec MoonwellSystems.Codec
---@field package folder string
---@field package abilities integer[] Their tooltips carry a file's text while it is read.
---@field package onError (fun(message: string): ...)?
---@field package sync MoonwellSystems.Sync
---@field package started boolean
---@field package disposed boolean
local Savefile = {}
Savefile.__index = Savefile

---@class MoonwellSystems.SavefileOptions
---@field codec MoonwellSystems.Codec
---@field folder string The folder under CustomMapData; 1 to 32 letters, digits, `-` or `_`.
---@field prefix string? The sync prefix. Default `"mwsave"`.
---@field timeout number? Scheduler seconds a load waits for the player's machine. Default 10.
---@field abilities integer[]? Ability ids whose tooltips carry the file; default 24 standard unit abilities.
---@field onError (fun(message: string): ...)?

---Standard unit abilities whose tooltip and extended tooltip keep text (measured on 3.0.0.24268).
local DEFAULT = {'Amls', 'Aroc', 'Amic', 'Amil', 'Aclf', 'Acmg', 'Adef', 'Adis', 'Afbt', 'Afbk', 'Aflk', 'Agyb', 'Agyv',
    'Ahea', 'Ainf', 'Aivs', 'Amdf', 'Aply', 'Asth', 'Aslo', 'Asps', 'Afsh', 'Absk', 'Ablo'}
---What the player's machine answers for a file that does not hold what `save` writes. Not one of a code's symbols.
local DAMAGED = '!'

---The ability id of a four-letter code, as the game's FourCC gives it. Pure, so nothing is called at import.
---@param code string
---@return integer
local function id(code)
    local a, b, c, d = code:byte(1, 4)
    return ((a * 256 + b) * 256 + c) * 256 + d
end

---@param ability integer
---@return string
local function code(ability)
    return string.char((ability >> 24) & 255, (ability >> 16) & 255, (ability >> 8) & 255, ability & 255)
end

---@param value unknown
---@return boolean
local function identifier(value)
    return type(value) == 'string' and #value >= 1 and #value <= 32 and value:find('^[A-Za-z0-9_-]+$') ~= nil
end

---The message of an error another module raised, without its position and its label.
---@param message unknown
---@return string
local function reason(message)
    return (tostring(message):gsub('^.-:%d+: ', ''):gsub('^%[systems%] [%w.]+: ', ''))
end

---@param clock MoonwellSystems.Scheduler Times loads out; it must run on every machine alike.
---@param options MoonwellSystems.SavefileOptions
---@return MoonwellSystems.Savefile
function Savefile.new(clock, options)
    Check.receiver(clock, Scheduler, 'Scheduler', 'Savefile.new')
    if type(options) ~= 'table' then error('[systems] Savefile.new: expected an options table', 2) end
    local codec, folder = options.codec, options.folder
    if getmetatable(codec) ~= Codec then error('[systems] Savefile.new: expected a codec', 2) end
    if not identifier(folder) then
        error('[systems] Savefile.new: expected a folder of 1 to 32 letters, digits, - or _', 2)
    end
    local abilities = {}
    if options.abilities == nil then
        for index, name in ipairs(DEFAULT) do abilities[index] = id(name) end
    else
        local given, seen = options.abilities, {}
        if type(given) ~= 'table' or #given < 1 then
            error('[systems] Savefile.new: expected abilities: a list of ability ids', 2)
        end
        for index = 1, #given do
            local ability = type(given[index]) == 'number' and math.tointeger(given[index]) or nil
            if not ability or ability < 1 or seen[ability] then
                error('[systems] Savefile.new: expected abilities: a list of ability ids, each once', 2)
            end
            seen[ability] = true
            abilities[index] = ability
        end
    end
    local longest = codec:getMaxLength()
    if longest > Files.capacity(abilities) then
        error('[systems] Savefile.new: the codec\'s longest code is ' .. longest .. ' symbols, but the abilities hold '
            .. Files.capacity(abilities), 2)
    end
    Callback.optional(options.onError, 'Savefile.new')
    local prefix = options.prefix
    if prefix == nil then prefix = 'mwsave' end
    local ok, sync = pcall(Sync.new, clock, {prefix = prefix, timeout = options.timeout, maxLength = longest,
        onError = options.onError})
    if not ok then error('[systems] Savefile.new: ' .. reason(sync), 2) end
    return setmetatable({codec = codec, folder = folder, abilities = abilities, onError = options.onError,
        sync = sync, started = false, disposed = false}, Savefile)
end

---The receiver of a method that needs a started, live system, and the file path of `slot`; raises at the public
---function's caller.
---@param self unknown
---@param operation string
---@param player unknown
---@param slot unknown
---@return MoonwellSystems.Savefile system
---@return string path
local function ready(self, operation, player, slot)
    local system = Check.receiver(self, Savefile, 'Savefile', operation, 1)
    if system.disposed then error('[systems] ' .. operation .. ': the system is disposed', 3) end
    if not system.started then error('[systems] ' .. operation .. ': call start() first', 3) end
    if getmetatable(player) ~= Player then error('[systems] ' .. operation .. ': expected Player', 3) end
    if not identifier(slot) then
        error('[systems] ' .. operation .. ': expected a slot of 1 to 32 letters, digits, - or _', 3)
    end
    return system, system.folder .. '\\' .. slot .. '.pld'
end

---Checks that every borrowed ability can carry text, and starts listening for loads. Call it once, on every
---machine, before the first `save` or `load`. Idempotent.
function Savefile:start()
    local system = Check.receiver(self, Savefile, 'Savefile', 'Savefile.start')
    if system.disposed then error('[systems] Savefile.start: the system is disposed', 2) end
    if system.started then return end
    local bad = Files.verify(system.abilities)
    if bad then error('[systems] Savefile.start: ability \'' .. code(bad) .. '\' cannot carry text', 2) end
    system.sync:start()
    system.started = true
end

---Saves `data` for `player` under `slot`. Call it on every machine: data that does not fit the codec's schema raises
---on all of them alike, and only the player's own machine writes the file. The code is bound to the player's name.
---@param player MoonwellWrappers.Player
---@param slot string 1 to 32 letters, digits, `-` or `_`.
---@param data table
function Savefile:save(player, slot, data)
    local system, path = ready(self, 'Savefile.save', player, slot)
    local ok, encoded = pcall(system.codec.encode, system.codec, data, player:getName())
    if not ok then error('[systems] Savefile.save: ' .. reason(encoded), 2) end
    if player:isLocal() then Files.write(path, system.abilities, encoded) end
end

---Loads `player`'s data from `slot`. Call it on every machine: the player's machine reads the file and sends its
---code, and `callback` runs later on every machine at the same moment, with the data or with nil and a reason.
---@param player MoonwellWrappers.Player
---@param slot string
---@param callback fun(data: table?, reason: string?): ... Reasons: `missing`, `damaged`, `format`, `checksum`,
---`version`, `schema`, `migration`, `error`, `absent`, `timeout`, `disposed`.
function Savefile:load(player, slot, callback)
    local system, path = ready(self, 'Savefile.load', player, slot)
    Callback.check(callback, 'Savefile.load')
    local onError = system.onError
    system.sync:ask(player, function()
        local found, why = Files.read(path, system.abilities)
        if why == 'damaged' then return DAMAGED end
        return found
    end, function(text, why)
        local data
        if text == DAMAGED then
            why = 'damaged'
        elseif text then
            local detail
            data, why, detail = system.codec:decode(text, player:getName())
            if detail then Callback.report('Savefile migration', onError, detail) end
        elseif why == 'none' then
            why = 'missing'
        end
        Callback.call('Savefile callback', onError, callback, data, why)
    end)
end

---Ends the open loads with 'disposed' and stops listening. `save`, `load` and `start` raise afterwards. Idempotent.
function Savefile:dispose()
    local system = Check.receiver(self, Savefile, 'Savefile', 'Savefile.dispose')
    if system.disposed then return end
    system.disposed = true
    system.sync:dispose()
end

return Savefile
```

- [ ] **Step 4:** `yue -e tests/run.lua; echo "exit $?"` → `savefile: SUITE PASSED: 10 tests`,
  `All 21 suites passed`, `exit 0`.

- [ ] **Step 5: Commit** `src/systems/savefile.lua tests/savefile.lua tests/suites.lua` — `feat: systems.savefile`.

---

### Task 5: Sweep, imports, fixtures and integration

**Files:** Modify `tests/blame.lua`, `tests/imports.lua`, `tests/editor-positive.lua`, `tests/editor-positive.yue`,
`tests/editor-negative.lua`, `tools/integration.lua`.

- [ ] **Step 1: The sweep.** In `tests/blame.lua`, the module list's second line becomes:

```lua
    'missile', 'knockback', 'codec', 'sync', 'savefile'}
```

- [ ] **Step 2: Imports.** In `tests/imports.lua`, before `test('importing every module calls no native', …)` (so no
  earlier test has loaded a wrappers module):

```lua
test('the codec loads no wrappers module', function()
    require('systems.codec')
    eq(totalCalls(), 0)
    for name in pairs(package.loaded) do assert(not name:find('^wrappers%.'), name) end
end)
```

and at the end of the file, after an empty line:

```lua
test('the sync and savefile modules call no native at import and load no trigger module', function()
    require('systems.sync')
    eq(package.loaded['systems.codec'] ~= nil and package.loaded['systems.internal.preload'] == nil, true)
    require('systems.savefile')
    eq(totalCalls(), 0)
    eq(package.loaded['wrappers.sync'] ~= nil, true)
    eq(package.loaded['systems.internal.preload'] ~= nil, true)
    eq(package.loaded['wrappers.trigger'], nil)
end)

```

Run `yue -e tests/run.lua; echo "exit $?"` → `imports: SUITE PASSED: 9 tests`, `blame: SUITE PASSED: 1 tests`,
`All 21 suites passed` (185 tests in all).

- [ ] **Step 3: Fixtures.** In `tests/editor-positive.lua`, after the line that prints `knockbacks:getCount()`:

```lua
local Codec = require('systems.codec')
local Sync = require('systems.sync')
local Savefile = require('systems.savefile')
local codec = Codec.new({version = 2, secret = 'k3-vale-of-ash', schemas = {
    {version = 1, fields = {{key = 'gold', kind = 'integer', min = 0, max = 1000000}},
        migrate = function(old) return {gold = old.gold, items = {}, name = '', hardMode = false} end},
    {version = 2, fields = {{key = 'gold', kind = 'integer', min = 0, max = 1000000},
        {key = 'items', kind = 'list', maxLength = 6, of = {kind = 'integer', min = 0, max = 2147483647}},
        {key = 'name', kind = 'string', maxLength = 16}, {key = 'hardMode', kind = 'boolean'}}}}})
local code = codec:encode({gold = 500, items = {1, 2}, name = 'Hero', hardMode = true}, owner:getName())
local decoded, why, detail = codec:decode(code, owner:getName())
print(decoded and decoded.gold, why, detail, codec:getVersion(), codec:getMaxLength())
local sync = scope:add(Sync.new(clock, {prefix = 'ask', timeout = 5, maxLength = 100}))
sync:start()
sync:ask(owner, function() return tostring(Time.localUtc()) end, function(text, reason) print(text, reason) end)
local saves = scope:add(Savefile.new(clock, {codec = codec, folder = 'Vale', prefix = 'save', timeout = 5,
    abilities = {1097690227, 1097035619}, onError = print}))
saves:start()
saves:save(owner, 'slot1', {gold = 1, items = {}, name = '', hardMode = false})
saves:load(owner, 'slot1', function(data, reason) print(data and data.gold, reason) end)
```

In `tests/editor-positive.yue`, after the `systems.knockback` import:

```
import "systems.codec" as Codec
import "systems.savefile" as Savefile
import "wrappers.player" as Player
```

and at the end of the `mw.on_main` body:

```
  codec = Codec.new
    version: 1, secret: "k3-vale-of-ash"
    schemas: {{version: 1, fields: {{key: "gold", kind: "integer", min: 0, max: 100}}}}
  saves = scope\add Savefile.new clock, codec: codec, folder: "Vale"
  saves\start!
  owner = Player.fromIndex 0
  saves\save owner, "slot1", gold: 5
  saves\load owner, "slot1", (data, reason) -> print data and data.gold, reason
```

In `tests/editor-negative.lua`, before `return true`:

```lua
local Codec = require('systems.codec')
local Sync = require('systems.sync')
local Savefile = require('systems.savefile')
Codec.new({version = '1', secret = 's', schemas = {}}) -- EXPECT assign-type-mismatch
Codec.new({version = 1, secret = 's', schemas = {}}):encode('data') -- EXPECT param-type-mismatch
Sync.new(clock):ask(clock, print, print) -- EXPECT param-type-mismatch
Sync.new(clock, {timeout = 'soon'}) -- EXPECT assign-type-mismatch
Savefile.new(clock, {codec = clock, folder = 'Vale'}) -- EXPECT assign-type-mismatch
Savefile.new(clock):start() -- EXPECT missing-parameter
```

- [ ] **Step 4: Integration.** In `tools/integration.lua`:
- the second line of `public` becomes:

```lua
    'missile', 'knockback', 'codec', 'sync', 'savefile'}
```

- add to `entries`, after `knockback`:

```lua
    codec = {source = 'import "systems.codec" as Codec\n'
        .. 'c = Codec.new version: 1, secret: "s", schemas: {{version: 1, fields: {}}}\nprint c\\getVersion!\n',
        systems = {}, wrappers = {}},
    sync = {source = 'import "systems.sync" as Sync\nimport "systems.scheduler" as Scheduler\n'
        .. 's = Sync.new Scheduler.new!\ns\\dispose!\n', systems = {scheduler = true},
        wrappers = {timer = true, player = true, sync = true}},
    savefile = {source = 'import "systems.savefile" as Savefile\nprint Savefile\n',
        systems = {codec = true, sync = true, scheduler = true},
        wrappers = {timer = true, player = true, sync = true}},
```

- the gate loop's line and its summary become:

```lua
for _, name in ipairs({'gate', 'gate-damage', 'gate-physics', 'gate-knockback', 'gate-save'}) do
```

```lua
print('Gate examples: all five build and their editor diagnostics are clean; game execution remains manual')
```

  Integration needs `examples/gate-save.yue`, which Task 6 creates: run integration in Task 6, Step 2.

- [ ] **Step 5:** Run `yue -e tests/run.lua; echo "exit $?"` and the syntax check. Expected: all 21 suites pass; the
  syntax check counts 51 files.

- [ ] **Step 6: Commit** `tests/blame.lua tests/imports.lua tests/editor-positive.lua tests/editor-positive.yue
  tests/editor-negative.lua tools/integration.lua` — `test: sweep, imports, fixtures and bundles for release 5`.

---

### Task 6: The gate example, the gate map and the docs

**Files:** Create `examples/gate-save.yue`; modify `README.md`, `CHANGELOG.md`, `CONTRIBUTING.md`, `AGENTS.md`;
modify (not under git) `../wrappers-gate/gate.ts`.

- [ ] **Step 1: The gate example** — `examples/gate-save.yue` (no bitwise operator and no `//`; no function ends in
  a bare `return`):

```
-- The moonwell-systems persistence gate (release 5). Built by the gate map's `systems-save` run (../wrappers-gate,
-- deno task gate systems-save); CONTRIBUTING lists every expected message. Nothing needs watching: it starts just
-- after the map loads, and also writes its lines to CustomMapData\moonwell-systems-save.pld. Its save files go to
-- CustomMapData\moonwell-gate\.
import "moonwell" as mw
import "wrappers.player" as Player
import "wrappers.timer" as Timer
import "systems.scheduler" as Scheduler
import "systems.time" as Time
import "systems.codec" as Codec
import "systems.sync" as Sync
import "systems.savefile" as Savefile
import "systems.internal.preload" as Files

SECRET = "k3-vale-of-ash"
MAX = 2147483647
FOLDER = "moonwell-gate"
-- The abilities a Savefile borrows by default.
ABILITIES = [FourCC(code) for code in *{
  "Amls", "Aroc", "Amic", "Amil", "Aclf", "Acmg", "Adef", "Adis", "Afbt", "Afbk", "Aflk", "Agyb", "Agyv", "Ahea"
  "Ainf", "Aivs", "Amdf", "Aply", "Asth", "Aslo", "Asps", "Afsh", "Absk", "Ablo"
}]

lines = {}
say = (...) ->
  parts = {"Save"}
  for index = 1, select "#", ...
    parts[#parts + 1] = tostring (select index, ...)
  line = table.concat parts, " "
  print line
  lines[#lines + 1] = line
save = ->
  PreloadGenClear!
  PreloadGenStart!
  for line in *lines
    Preload (line\gsub "\"", "'")
  PreloadGenEnd "moonwell-systems-save.pld"
  PreloadGenClear!
tooltips = ->
  parts = {}
  for ability in *ABILITIES
    parts[#parts + 1] = BlzGetAbilityTooltip(ability, 0) .. "|" .. BlzGetAbilityExtendedTooltip(ability, 0)
  table.concat parts, "/"

heroFields = {
  {key: "gold", kind: "integer", min: 0, max: 1000000}
  {key: "hero", kind: "string", maxLength: 16}
  {key: "hardMode", kind: "boolean"}
  {key: "items", kind: "list", maxLength: 6, of: {kind: "integer", min: 0, max: MAX}}
}
heroData = gold: 500, hero: "Hpal", hardMode: true, items: {1227894832, 1227894833, MAX}
describe = (data) ->
  "gold " .. data.gold .. " hero " .. data.hero .. " hardMode " .. tostring(data.hardMode) .. " items " ..
    table.concat data.items, ","

saveGate = (owner) ->
  clock = Scheduler.new!
  clock\start!
  before = tooltips!

  -- 1. Parity: the codes the test suite expects, from the game's 32-bit integers.
  hero = Codec.new version: 1, secret: SECRET, schemas: {{version: 1, fields: heroFields}}
  heroCode = hero\encode heroData, "WorldEdit"
  edges = Codec.new version: 2, secret: SECRET, schemas: {{version: 2, fields: {
    {key: "low", kind: "integer", min: -MAX, max: 0}
    {key: "high", kind: "integer", min: 0, max: MAX}
    {key: "fixed", kind: "integer", min: 7, max: 7}
    {key: "flags", kind: "list", maxLength: 5, of: {kind: "boolean"}}
    {key: "names", kind: "list", maxLength: 3, of: {kind: "string", maxLength: 4}}
  }}}
  edgeCode = edges\encode low: -MAX, high: MAX, fixed: 7, flags: {true, false, true}, names: {"", "ab", "\0\255\128\n"}
  empty = Codec.new version: 9999, secret: "x", schemas: {{version: 9999, fields: {}}}
  say "1 parity: hero", heroCode == "BAAQAfQiQ4MLZckwMDCSYGBj_____8WQGq", "edges",
    edgeCode == "BAAgAAAAH____93CYWKAH_ABQ-Isnq", "empty", empty\encode({}, "Player \xc3\x85ke") == "BnDwdyUOJ"
  decoded = edges\decode edgeCode
  say "1 the edges decode:", decoded and decoded.low == -MAX and decoded.high == MAX and decoded.fixed == 7 and
    decoded.names[3] == "\0\255\128\n" and math.type(decoded.high)

  -- 4. Another binding does not decode.
  _, why = hero\decode heroCode, "Somebody else"
  say "4 another binding:", why

  saves = Savefile.new clock, codec: hero, folder: FOLDER
  saves\start!

  -- 2. A round trip through the file and through sync.
  clock\after 1, ->
    saves\save owner, "slot1", heroData
    saves\load owner, "slot1", (data, reason) -> say "2 round trip:", data and describe(data) or reason

  -- 3. A slot that was never saved.
  clock\after 2, ->
    saves\load owner, "nothing-here", (data, reason) -> say "3 a missing slot:", data, reason

  -- 5. A file that does not hold a code, and a code with one symbol changed.
  clock\after 3, ->
    Files.write FOLDER .. "\\damaged.pld", ABILITIES, "not!a!code"
    saves\load owner, "damaged", (data, reason) -> say "5 a damaged file:", data, reason
    good = hero\encode heroData, owner\getName!
    changed = good\sub(1, 5) .. (good\sub(6, 6) == "A" and "B" or "A") .. good\sub(7)
    Files.write FOLDER .. "\\changed.pld", ABILITIES, changed
    saves\load owner, "changed", (data, reason) -> say "5 a changed symbol:", data, reason

  -- 6. The largest save a codec may have: 8189 symbols, through 44 tooltips and 38 sync packets.
  clock\after 4, ->
    big = Codec.new version: 1, secret: SECRET, schemas: {{version: 1, fields: {
      {key: "a", kind: "string", maxLength: 4095}
      {key: "b", kind: "string", maxLength: 2037}
    }}}
    bigSaves = Savefile.new clock, codec: big, folder: FOLDER, prefix: "mwbig"
    bigSaves\start!
    data = a: string.rep("\255", 4095), b: string.rep("z", 2037)
    started = os.clock!
    bigSaves\save owner, "big", data
    wrote = (os.clock! - started) * 1000
    started = os.clock!
    bigSaves\load owner, "big", (loaded, reason) ->
      say "6 the largest save:", loaded and loaded.a == data.a and loaded.b == data.b or reason, "of",
        big\getMaxLength!, "symbols arrived after", string.format("%.2f", os.clock! - started), "s"
      bigSaves\dispose!
    say "6 saving took", string.format("%.0f", wrote), "ms, and reading and sending",
      string.format("%.0f", (os.clock! - started) * 1000), "ms"

  -- 7. A file of version 1 loads as version 2 data.
  clock\after 5.5, ->
    old = Codec.new version: 1, secret: SECRET, schemas: {
      {version: 1, fields: {{key: "gold", kind: "integer", min: 0, max: 100}}}
    }
    oldSaves = Savefile.new clock, codec: old, folder: FOLDER, prefix: "mwold"
    oldSaves\start!
    oldSaves\save owner, "old", gold: 40
    oldSaves\dispose!
    newer = Codec.new version: 2, secret: SECRET, schemas: {
      {
        version: 1, fields: {{key: "gold", kind: "integer", min: 0, max: 100}}
        migrate: (data) -> {coins: data.gold * 10}
      }
      {version: 2, fields: {{key: "coins", kind: "integer", min: 0, max: 1000}}}
    }
    newSaves = Savefile.new clock, codec: newer, folder: FOLDER, prefix: "mwnew"
    newSaves\start!
    newSaves\load owner, "old", (data, reason) ->
      say "7 a migration: coins", data and data.coins, reason
      newSaves\dispose!

  -- 8. Asking a machine for a local value, and a slot without a human. The second answer comes first: it needs no
  -- message.
  sync = Sync.new clock
  sync\start!
  clock\after 6.5, ->
    sync\ask owner, (-> tostring Time.localUtc!), (text, reason) ->
      say "8 ask: the local clock arrived:", text ~= nil and #text > 0, reason
    sync\ask Player.fromIndex(5), (-> "never"), (text, reason) ->
      say "8 ask a slot without a human:", text, reason

  -- 9. Every borrowed tooltip reads as it did before the run.
  clock\after 8, ->
    say "9 tooltips unchanged:", tooltips! == before
    sync\dispose!
    saves\dispose!
    clock\dispose!
    say "gate done"
    save!
  say "gate started as", owner\getName!

mw.on_main ->
  start = Timer.create!
  start\start 0, false, (self) ->
    self\destroy!
    saveGate Player.fromIndex 0
```

- [ ] **Step 2: Integration.** Run the integration check. Expected:
  `LuaLS: 26 intentional type errors detected at the expected lines`,
  `Moonwell: every entry point (15) bundles only what it imports`,
  `Gate examples: all five build and their editor diagnostics are clean; game execution remains manual`,
  `Integration passed`.

- [ ] **Step 3: The gate map.** In `../wrappers-gate/gate.ts`:
- after the `//   systems-knockback …` usage line, add:

```ts
//   systems-save      moonwell-systems release 5 persistence gate (../moonwell-systems/examples/gate-save.yue)
```

- `copiedRuns` gains `"systems-save": "../moonwell-systems/examples/gate-save.yue",`.

  Run `deno task gate systems-save --no-launch` in `../wrappers-gate` → `Gate map: gate-maps/systems-save.w3x`.

- [ ] **Step 4: README.**
- The first paragraph's list becomes: "a deterministic scheduler, signals, ownership scopes, time helpers, script
  buffs, auras, dummy casters, a damage pipeline, missiles, knockbacks and save files". The words about a later
  release go.
- After the `systems.knockback` section, add:

````markdown
### `systems.codec`

- `Codec.new({version, secret, schemas})`
- `encode(data, binding = "")` returns the code
- `decode(code, binding = "")` returns the data, or `nil, reason, detail`
- `getVersion()`, `getMaxLength()` (the longest code any of its schemas can produce)

A code is data packed by a schema into 64 symbols (`A-Z a-z 0-9 - _`), ending in a check value. It is pure: the same
on every machine, in the game and outside it.

- **`schemas`** is a list of `{version, fields, migrate?}`, one per version the map still reads; `version` (1 to 9999)
  is the one `encode` writes. `migrate(data)` returns the data of the next version, and a code of an older version
  is migrated one version at a time.
- **`fields`** (at most 64) each have a `key` and a `kind`:
  - `"integer"` with `min` and `max`: a whole number in that range. Both lie within ±2,147,483,647, and `max - min`
    is at most 2,147,483,647. It takes only the bits its range needs: gold from 0 to 1,000,000 takes 20.
  - `"boolean"`: one bit.
  - `"string"` with `maxLength` (at most 4095 bytes): any bytes, so names in any language fit.
  - `"list"` with `maxLength` (at most 4095) and `of`, a field of one of the three kinds above without a `key`: an
    array of that kind.
- **Numbers are whole.** A map that wants 12.5 stores 125: the game's floats are single precision.
- **`encode` raises** at your line when the data does not fit, and names the field:
  `[systems] Codec.encode: field "gold": expected a whole number from 0 to 1000000`. A key the schema does not have
  raises too.
- **`decode` never raises for a bad code.** Its reasons: `format` (not a code), `checksum` (an edited code, another
  secret or another binding), `version` (no schema for the code's version), `schema` (the bits do not fit that
  version's schema: a schema changed without a new version) and `migration` (with the message as `detail`).
- **`secret` and `binding`.** The check value mixes the map's secret, the binding (typically the player's name) and
  the data, so a code cannot be edited, or handed to another player, without the secret. This is not cryptography:
  the secret is in the map script, and whoever reads it can forge a save.
- A codec's longest code is limited to 8,192 symbols; `Codec.new` raises for a schema that could pass it.

```yue
import "systems.codec" as Codec

codec = Codec.new
  version: 2
  secret: "k3-vale-of-ash"
  schemas: {
    {
      version: 1
      fields: {{key: "gold", kind: "integer", min: 0, max: 1000000}}
      migrate: (old) -> {gold: old.gold, hero: "", hardMode: false, items: {}}
    }
    {
      version: 2
      fields: {
        {key: "gold", kind: "integer", min: 0, max: 1000000}
        {key: "hero", kind: "string", maxLength: 16}
        {key: "hardMode", kind: "boolean"}
        {key: "items", kind: "list", maxLength: 6, of: {kind: "integer", min: 0, max: 2147483647}}
      }
    }
  }

code = codec\encode {gold: 500, hero: "Hpal", hardMode: true, items: {itemType}}, player\getName!
data, reason = codec\decode code, player\getName!
```

### `systems.sync`

- `Sync.new(clock, {prefix = "mwsync", timeout = 10, maxLength = 8192, onError?})`
- `start()`, `ask(Player, read, receive)`, `dispose()`

One player's machine knows something the others do not: a file on its disk, its clock. `ask` gets it to every
machine safely.

- **Call `ask` on every machine,** in the same order, like any other game code. Do not put it inside a
  local-player check.
- **`read()`** runs on that player's machine only, at once, and returns a string (no byte below 32) or nil.
- **`receive(text, reason)`** runs on every machine at the same moment, later, never inside `ask`: with the text; or
  with `nil` and `"none"` (read returned nil), `"error"` (read failed or returned something that cannot be sent),
  `"absent"` (no human plays in that slot), `"timeout"` (no answer within `timeout` seconds of the scheduler, for
  example because the player left) or `"disposed"`.
- Only the asked player's packets are taken, and the sender is the one the game reports.
- The text travels in packets of 220 bytes; a sync message keeps 255.

```yue
import "systems.sync" as Sync
import "systems.time" as Time

sync = Sync.new clock
sync\start!
sync\ask player, (-> tostring Time.localUtc!), (text, reason) ->
  print player\getName!, "says the time is", text or reason
```

### `systems.savefile`

- `Savefile.new(clock, {codec, folder, prefix = "mwsave", timeout = 10, abilities?, onError?})`
- `start()`, `save(Player, slot, data)`, `load(Player, slot, callback)`, `dispose()`

A player's data in a file on that player's own machine:
`Documents\Warcraft III\CustomMapData\<folder>\<slot>.pld`. `folder` and `slot` are 1 to 32 letters, digits, `-`
or `_`.

- **Call `save` and `load` on every machine,** like any other game code. The steps that run on one machine only are
  inside: you write no local-player check.
- **`save`** encodes the data with the player's name as the binding, so data that does not fit raises on every
  machine alike; only the player's own machine writes the file.
- **`load`** asks the player's machine for the file (through `systems.sync`), decodes it, and calls
  `callback(data, reason)` on every machine at the same moment. Without data, `reason` is `missing`, `damaged`, one
  of the codec's (`format`, `checksum`, `version`, `schema`, `migration`) or one of the sync system's (`error`,
  `absent`, `timeout`, `disposed`).
- **Borrowed tooltips.** A file is JASS that the game runs, and it hands its text to Lua through the tooltip and
  the extended tooltip of standard unit abilities: 24 by default, enough for the longest code. Each is changed only
  while the file is read, on that one machine, and restored before `load` returns. `start()` checks that each can
  carry text. `abilities` replaces the list (ability ids). No handle is created.
- **A file edited by hand can crash that player's game** when it is read: the game runs it. The library writes only
  the codec's 64 symbols, in lines the game keeps whole.
- **Reading freezes that player's game for 40 to 80 ms.** Load at a quiet moment.
- **Two machines are untested:** everything was measured on one. The two-player checks wait for the online step
  before Moonwell 1.0.

```yue
import "systems.savefile" as Savefile

saves = Savefile.new clock, codec: codec, folder: "ValeOfAsh"
saves\start!

saves\save player, "slot1", {gold: 500, hero: "Hpal", hardMode: true, items: {itemType}}

saves\load player, "slot1", (data, reason) ->
  if data
    player\setGold data.gold
  else
    print player\getName!, "has no save:", reason
```
````

- "Changes from wc3-lib" gains:

```markdown
- Save codes are packed as bits in 64 symbols, about a third as long as wc3-lib's hex text. Fields are `integer`,
  `boolean`, `string` and `list`; non-integer numbers are gone, and strings hold any bytes.
- The check value is keyed with a map secret. `encode` raises for data that does not fit instead of returning a
  failure, and `decode` returns `nil, reason`.
- The local store has no port and is not public: `Savefile` owns it. It carries one chunk per ability field,
  because wc3-lib's appended tooltip read back only its first chunk in game.
- `SyncReceiver`, `WarcraftSyncTransport`, sessions and `expect` are one call, `sync:ask`, which also answers for a
  missing value, an absent player and a timeout.
- A map writes no local-player check to save or load.
```

- [ ] **Step 5: CONTRIBUTING.** After release 4's gate steps (before the `v0.1.0: passed…` record), add:

```markdown
Release 5 (v0.5.0) has its own run, `deno task gate systems-save`, about 9 seconds. Nothing needs watching. Its lines
also go to `Documents\Warcraft III\CustomMapData\moonwell-systems-save.pld`, and its save files to
`CustomMapData\moonwell-gate\`. `Save gate started as <your name>` prints first.

37. At once: `Save 1 parity: hero true edges true empty true` (the game's 32-bit integers give the codes the test
    suite expects) and `Save 1 the edges decode: integer`.
38. `Save 4 another binding: checksum`.
39. At 1 s: `Save 2 round trip: gold 500 hero Hpal hardMode true items 1227894832,1227894833,2147483647`.
40. At 2 s: `Save 3 a missing slot: nil missing`.
41. At 3 s: `Save 5 a damaged file: nil damaged` and `Save 5 a changed symbol: nil checksum`.
42. At 4 s: `Save 6 saving took <ms> ms, and reading and sending <ms> ms`, then
    `Save 6 the largest save: true of 8189 symbols arrived after <s> s`. Record the three numbers.
43. At 5.5 s: `Save 7 a migration: coins 400 nil`.
44. At 6.5 s: `Save 8 ask a slot without a human: nil absent`, then
    `Save 8 ask: the local clock arrived: true nil`.
45. At 8 s: `Save 9 tooltips unchanged: true`, then `Save gate done`.
46. No `[systems] … failed` line prints in the run, and `CustomMapData\moonwell-gate\` holds `slot1.pld`,
    `damaged.pld`, `changed.pld`, `big.pld` and `old.pld`.

Two machines are not part of this gate: the online checks before Moonwell 1.0 cover them.
```

  In the "Publication and tag gate" paragraph, the list of gate examples gains `examples/gate-save.yue`.

- [ ] **Step 6: CHANGELOG and AGENTS.**
- CHANGELOG: a new first section:

```markdown
## Unreleased

Release 5 of the wc3-lib port, the last (spec `2026-10-01-moonwell-systems-release-5-design` in the Moonwell
repository).

- `systems.codec`: save codes packed by a versioned schema into 64 symbols (integers by their range, booleans,
  strings and lists), with a check value keyed by a map secret, a binding such as the player's name, and migrations.
- `systems.sync`: `ask` one player's machine for a local value; the answer, or the reason there is none, reaches
  every machine at the same moment.
- `systems.savefile`: `save` and `load` a player's data in a local file. The steps that run on one machine only are
  inside the library.
- A save file is carried by the tooltips of borrowed standard abilities, which are restored at once; nothing creates
  a handle on one machine only.
```

- AGENTS: "Release 5: persistence." becomes "Release 5 (v0.5.0): codec, sync, savefile, on `internal/preload.lua`.";
  the Process line gains `deno task gate systems-save` for release 5; new pitfalls:

```markdown
- The game's integers are 32-bit and the test runner's 64-bit. Arithmetic that is meant to wrap (the codec's check
  value) masks every product with `& 0xFFFFFFFF`, so both agree; the codec writes at most 16 bits at a time; and
  fixed codes in `tests/codec.lua` are compared in game by the gate. Those codes came from a second implementation
  written from the spec's layout alone; a change to the layout needs a new layout number, not new fixed codes.
- `math.tointeger` converts strings on Lua 5.4 and not on 5.3: check `type(x) == 'number'` first.
- A Preload file must hold only lines the game can run, none over 259 characters: anything else can crash the game
  when `Preloader` runs it. The tests run every generated line through a simulated `Preloader` that refuses any
  other shape.
- Reading a save happens on one machine, so nothing on that path may create a handle. A game cache makes one per
  call (measured); tooltips make none (`../wrappers-gate/PROBE-PRELOAD-RESULTS.md`).
- The wrappers keep one trigger per sync prefix for every listener: a test that counts `CreateTrigger` needs a
  prefix no other test used.
- yue 0.34.2 writes an empty file for a source with a bitwise operator, as for `//`: gate examples keep such code
  out (the library is Lua).
```

- [ ] **Step 7: Checks and commit.** Run the three checks. Commit `examples/gate-save.yue README.md CHANGELOG.md
  CONTRIBUTING.md AGENTS.md` — `docs: release 5 docs and the save gate`.

---

### Task 7: Release

- [ ] **Step 1:** The maintainer runs `deno task gate systems-save` in `../wrappers-gate` and says when
  `Save gate done` has printed; nothing needs watching. Read
  `Documents\Warcraft III\CustomMapData\moonwell-systems-save.pld` for the lines, and list
  `CustomMapData\moonwell-gate\` for the files. Steps 37 to 46 of CONTRIBUTING must hold; record the three numbers
  of step 42. If a reading contradicts the spec (a parity line that reads false, a tooltip left changed, a wrong
  reason), stop and fix it test-first before releasing.
- [ ] **Step 2:** Record: CHANGELOG `## 0.5.0 (<date>)` with a release-gate section; CONTRIBUTING `v0.5.0:` record;
  README Status (`v0.5.0`, the three new modules, the port complete) and the `tag = "v0.5.0"` example. Run the three
  checks. Commit, push, tag `v0.5.0` on the verified commit, push the tag, GitHub pre-release from the changelog
  section (`gh` at `C:\Program Files\GitHub CLI\gh.exe`). Verify each step's exit code separately.
- [ ] **Step 3:** Tag consumption in `../systems-tag-check-050` (a map made with `init --link`, wrappers `v0.7.0`,
  systems `v0.5.0`): with each of the five gate examples in turn as `src/main.yue`: check, build, build `--minify`.
  The lock records both tags' commits; the fetched systems files match the tag's `src/` byte for byte; the lock is
  unchanged after removing `.moonwell/` and checking again. Record it in CONTRIBUTING and the tag in AGENTS; commit;
  push.
- [ ] **Step 4:** Moonwell records: `AGENTS.md` (a state bullet for v0.5.0 with the probes' and the gate's
  measurements; phase 3 of the roadmap is complete, so next work becomes phase 4), the roadmap's phase 3 status,
  the backlog's 4d item, and the `CHANGELOG.md` Unreleased line; commit; push; `gh run list`.
