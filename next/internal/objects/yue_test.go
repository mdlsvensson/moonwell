package objects_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/objects"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/tooltest"
)

// The generated module, compiled by the real compiler and run in its Lua. The test starts the compiler four
// times, and is skipped on a machine without one (tooltest.Yue).

// idsCheck is a Lua script that loads the compiled module from objects.lua beside it and holds it against the
// objects it was generated from: a table for every category, no entry but theirs, and each id the integer that
// FourCC gives for its rawcode. It prints "objects-ok" when all of it holds.
func idsCheck(generated []objects.Resolved) string {
	var expected, categories []string
	for _, entry := range generated {
		expected = append(expected, `  { "`+string(entry.Category)+`", "`+entry.Key+`", "`+entry.ID+`" },`)
	}
	for _, category := range manifest.Categories {
		categories = append(categories, `"`+string(category)+`"`)
	}
	// FourCC as the game defines it in Lua: the four bytes big-endian.
	return `
local function FourCC(id) return string.unpack(">I4", id) end
local objects = dofile("objects.lua")
local expected = {
` + strings.Join(expected, "\n") + `
}
local count = 0
for _, category in ipairs({ ` + strings.Join(categories, ", ") + ` }) do
  assert(type(objects[category]) == "table", "missing category " .. category)
  for _ in pairs(objects[category]) do count = count + 1 end
end
assert(count == #expected, "unexpected entries: " .. count)
for _, entry in ipairs(expected) do
  local category, key, id = entry[1], entry[2], entry[3]
  local value = objects[category][key]
  assert(math.type(value) == "integer", category .. "." .. key .. " is not an integer")
  assert(value == FourCC(id), category .. "." .. key .. " is " .. tostring(value) .. ", FourCC gives " .. FourCC(id))
end
io.write("objects-ok")
`
}

func TestTheGeneratedIDsModuleCompilesAndEachIDEqualsFourCC(t *testing.T) {
	compiler := tooltest.Yue(t)
	// One object per category and several units, with keys that sort and ids that span the rawcode alphabet.
	generated := []objects.Resolved{
		object("heroes", "paladin", "H000"),
		object("units", "captain", "h000"),
		object("units", "archer_2", "hz9Z"),
		object("units", "Zealot", "e001"),
		object("buildings", "keep", "h00A"),
		object("items", "claws", "I0zz"),
		object("abilities", "holy_light", "A000"),
		object("buffs", "blessed", "B000"),
		object("upgrades", "plating", "R000"),
	}
	check := idsCheck(generated)
	// The two ways a build compiles a module: rewritten to keep its lines, and minified.
	for _, mode := range []string{"-r", "-m"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			testkit.WriteFile(t, dir, "objects.yue", []byte(objects.RenderIDs(generated)))
			arguments := []string{"--target=5.3", mode, "-o", "objects.lua", "objects.yue"}
			compiled, err := env.Run(context.Background(), compiler, arguments, env.RunOptions{Dir: dir})
			if err != nil || compiled.Code != 0 {
				t.Fatalf("compiling failed with exit code %d (%v):\n%s\n%s", compiled.Code, err, compiled.Stdout, compiled.Stderr)
			}
			if printed := tooltest.RunLua(t, testkit.WriteFile(t, dir, "check.lua", []byte(check))); printed != "objects-ok" {
				t.Errorf("the check printed %q", printed)
			}
		})
	}
}
