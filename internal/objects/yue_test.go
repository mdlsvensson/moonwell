package objects_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/proc"
	"github.com/mdlsvensson/moonwell/internal/yuetest"
)

// The generated module, compiled by the real compiler and run in its Lua.

func TestTheGeneratedObjectsModuleCompilesAndEachIDEqualsFourCC(t *testing.T) {
	compiler := yuetest.Need(t)
	// One object per category and several units, with keys that sort and ids that span the rawcode alphabet.
	generated := []objects.Resolved{
		{Category: "heroes", Key: "paladin", ID: "H000"},
		{Category: "units", Key: "captain", ID: "h000"},
		{Category: "units", Key: "archer_2", ID: "hz9Z"},
		{Category: "units", Key: "Zealot", ID: "e001"},
		{Category: "buildings", Key: "keep", ID: "h00A"},
		{Category: "items", Key: "claws", ID: "I0zz"},
		{Category: "abilities", Key: "holy_light", ID: "A000"},
		{Category: "buffs", Key: "blessed", ID: "B000"},
		{Category: "upgrades", Key: "plating", ID: "R000"},
	}
	var expected, categories []string
	for _, object := range generated {
		expected = append(expected, `  { "`+string(object.Category)+`", "`+object.Key+`", "`+object.ID+`" },`)
	}
	for _, category := range objects.Categories {
		categories = append(categories, `"`+string(category)+`"`)
	}
	// FourCC as the game defines it in Lua: the four bytes big-endian.
	check := `
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
	for _, mode := range []string{"-r", "-m"} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "objects.yue"), []byte(objects.RenderIDs(generated)), 0o666); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "check.lua"), []byte(check), 0o666); err != nil {
			t.Fatal(err)
		}
		options := proc.Options{Dir: dir}
		compiled, err := proc.Run(context.Background(), compiler, []string{"--target=5.3", mode, "-o", "objects.lua", "objects.yue"}, options)
		if err != nil || compiled.Code != 0 {
			t.Fatalf("%s: compiling failed: %v\n%s\n%s", mode, err, compiled.Stdout, compiled.Stderr)
		}
		result, err := proc.Run(context.Background(), compiler, []string{"-e", "check.lua"}, options)
		if err != nil || result.Code != 0 || result.Stdout != "objects-ok" {
			t.Errorf("%s: %v\n%s\n%s", mode, err, result.Stdout, result.Stderr)
		}
	}
}
