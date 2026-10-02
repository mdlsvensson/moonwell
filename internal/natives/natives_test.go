package natives

import (
	"slices"
	"testing"
)

func TestLoadReadsTheEmbeddedNatives(t *testing.T) {
	natives := Load()
	if natives != Load() {
		t.Error("Load parsed twice")
	}
	if natives.GameVersion == "" || len(natives.Types) == 0 || len(natives.Globals) == 0 {
		t.Fatalf("natives = version %q, %d types, %d globals", natives.GameVersion, len(natives.Types), len(natives.Globals))
	}
	find := func(name string) *Function {
		for i := range natives.Functions {
			if natives.Functions[i].Name == name {
				return &natives.Functions[i]
			}
		}
		t.Fatalf("no function %s", name)
		return nil
	}
	create := find("CreateUnit")
	wantParams := []Param{
		{"id", "player"}, {"unitid", "integer"}, {"x", "real"}, {"y", "real"}, {"face", "real"},
	}
	if create.Source != "common.j" || create.Returns != "unit" || !slices.Equal(create.Params, wantParams) {
		t.Errorf("CreateUnit = %+v", create)
	}
	if alive := find("UnitAlive"); alive.Source != "lua" && alive.Source != "common.j" {
		t.Errorf("UnitAlive = %+v", alive)
	}
	if !slices.Contains(natives.Lua.Removed, "collectgarbage") || !slices.Contains(natives.Lua.Globals, "pairs") {
		t.Errorf("lua = %+v", natives.Lua)
	}
	hasHandle := slices.ContainsFunc(natives.Types, func(kind Type) bool { return kind.Name == "unit" && kind.Extends == "widget" })
	if !hasHandle {
		t.Error("type unit extends widget is missing")
	}
}
