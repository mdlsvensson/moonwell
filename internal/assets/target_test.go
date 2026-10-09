package assets

import (
	"strings"
	"testing"
)

func TestTargetPathTakesAPathAnAssetMayHaveWithSlashes(t *testing.T) {
	tests := []struct{ value, want string }{
		{"Textures/Icon.blp", "Textures/Icon.blp"},
		{`Textures\Icon.blp`, "Textures/Icon.blp"},
		{"war3mapImported/sound.wav", "war3mapImported/sound.wav"},
		{`WAR3MAPIMPORTED\sound.wav`, "WAR3MAPIMPORTED/sound.wav"},
		{"Units/war3map.lua", "Units/war3map.lua"},
		{"scripts/common.j", "scripts/common.j"},
		{"war3.blp", "war3.blp"},
	}
	for _, tt := range tests {
		if got, err := parseTargetPath(tt.value); err != nil || got != tt.want {
			t.Errorf("targetPath(%q) = %q, %v, want %q", tt.value, got, err, tt.want)
		}
	}
}

func TestTargetPathRefusesTheMapsOwnFiles(t *testing.T) {
	const internals, picture = "Assets cannot replace map internals", "settings.info.preview"
	tests := []struct{ value, hint string }{
		{"war3map.lua", internals},
		{"war3map.imp", internals},
		{"WAR3MAP.W3I", internals},
		{"war3mapMisc.txt", internals},
		{"war3campaign.w3u", internals},
		{"scripts/war3map.j", internals},
		{`Scripts\War3map.j`, internals},
		{"(listfile)", internals},
		{"(attributes)", internals},
		{"(signature)", internals},
		{"war3mapPreviews/a.tga", internals},
		{"war3mapPreview.tga", picture},
		{"WAR3MAPPREVIEW.BLP", picture},
		{"war3mapMap.blp", picture},
		{"war3mapMap.tga", picture},
	}
	for _, tt := range tests {
		_, err := parseTargetPath(tt.value)
		e := asDiagError(t, err, tt.value)
		if e.Msg != "Reserved map path: "+tt.value || !strings.Contains(e.Hint, tt.hint) || e.File != "" {
			t.Errorf("targetPath(%q): %+v, want a reserved path with a hint about %q", tt.value, e, tt.hint)
		}
	}
}

func TestTargetPathRefusesAPathThatLeavesTheMapOrThatWindowsCannotHold(t *testing.T) {
	for _, value := range []string{"../escape", "/absolute", `C:\escape`, "", "a//b.blp", "a/./b.blp", "con.blp", "icon.", "what?.blp", "a\tb.blp"} {
		got, err := parseTargetPath(value)
		e := asDiagError(t, err, value)
		if got != "" || e.Msg != "Invalid asset path: "+value || !strings.Contains(e.Hint, "relative path") || e.File != "" {
			t.Errorf("targetPath(%q) = %q, %+v", value, got, e)
		}
	}
}
