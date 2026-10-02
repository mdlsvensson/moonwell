package settings_test

import (
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/ordered"
	"github.com/mdlsvensson/moonwell/internal/settings"
)

func TestTextPatchesPreserveUnrelatedContentAndUpdateDuplicateKeys(t *testing.T) {
	for _, nl := range []string{"\n", "\r\n"} {
		original := strings.Join([]string{
			"// keep", "[Misc]", "MaxHeroLevel=10", "Keep=42", "[Other]", "X=y", "[misc]", "maxherolevel=12", "",
		}, nl)
		wanted := sections(t, `{"Misc":{"MaxHeroLevel":"25","Added":"0"},"CustomSkin":{"Text":""}}`)
		patched := settings.PatchText(original, wanted)
		for _, line := range []string{"// keep", "Keep=42", "X=y", "MaxHeroLevel=25", "maxherolevel=25", "Added=0", "Text="} {
			if !strings.Contains(patched, line) {
				t.Errorf("patched text lacks %q:\n%q", line, patched)
			}
		}
		if !strings.Contains(patched, "[Other]"+nl+"X=y") {
			t.Errorf("the Other section was disturbed: %q", patched)
		}
		if again := settings.PatchText(patched, wanted); again != patched {
			t.Errorf("patching twice changed the text:\n%q\n%q", patched, again)
		}
		if same := settings.PatchText(original, settings.Sections{}); same != original {
			t.Errorf("no sections changed the text: %q", same)
		}
	}
	if got := settings.PatchText("", sections(t, `{"Misc":{"FoodCeiling":"0"}}`)); got != "[Misc]\nFoodCeiling=0" {
		t.Errorf("an empty source = %q", got)
	}
}

func TestTypedGameplayMergesAreCaseInsensitiveWithoutMutatingSettings(t *testing.T) {
	s := validated(t, `{"gameplay":{"foodLimit":200},"gameplayConstants":{"misc":{"foodceiling":"200"}}}`)
	merged, err := settings.GameplaySections(s, "moonwell.pkl")
	if err != nil {
		t.Fatal(err)
	}
	if got := ordered.Stringify(&merged, 0); got != `{"misc":{"foodceiling":"200"}}` {
		t.Errorf("merged = %s", got)
	}
	if got := ordered.Stringify(&s.GameplayConstants, 0); got != `{"misc":{"foodceiling":"200"}}` {
		t.Errorf("the settings were changed: %s", got)
	}
	// The merge is a copy: changing it leaves the settings alone.
	misc, _ := merged.Get("misc")
	misc.Set("foodceiling", "changed")
	if got := ordered.Stringify(&s.GameplayConstants, 0); got != `{"misc":{"foodceiling":"200"}}` {
		t.Errorf("the merge shares its sections with the settings: %s", got)
	}

	conflict := validated(t, `{"gameplay":{"foodLimit":200},"gameplayConstants":{"misc":{"foodceiling":"0200"}}}`)
	_, err = settings.GameplaySections(conflict, "moonwell.pkl")
	e := asError(t, err, "a conflict")
	if e.Msg != "Conflicting typed and raw gameplay constant: FoodCeiling." || e.File != "moonwell.pkl" ||
		e.Hint != "Remove the raw FoodCeiling override or make it equal to settings.gameplay.foodLimit." {
		t.Errorf("error = %+v", e)
	}

	typed := validated(t, `{"gameplay":{"heroMaxLevel":25,"foodLimit":150},"gameplayConstants":{"Other":{"A":"1"}}}`)
	merged, err = settings.GameplaySections(typed, "moonwell.pkl")
	if err != nil {
		t.Fatal(err)
	}
	if got := ordered.Stringify(&merged, 0); got != `{"Other":{"A":"1"},"Misc":{"MaxHeroLevel":"25","FoodCeiling":"150"}}` {
		t.Errorf("typed constants = %s", got)
	}
}

func TestTextPatchesRecognizeTabIndentedKeysAndCaseOnlyMatches(t *testing.T) {
	original := "[mIsC] // keep heading comment\r\n\tfoodceiling = 100\r\n"
	patched := settings.PatchText(original, sections(t, `{"Misc":{"FoodCeiling":"200"}}`))
	if patched != "[mIsC] // keep heading comment\r\n\tfoodceiling=200\r\n" {
		t.Errorf("patched = %q", patched)
	}
}

func TestTextPatchesAddKeysToEmptySectionsAndCreateMissingSections(t *testing.T) {
	got := settings.PatchText("[Misc]\n[Skin]\n",
		sections(t, `{"Misc":{"FoodCeiling":"0"},"Skin":{"Text":""},"New":{"Value":"1"}}`))
	if want := "[Misc]\nFoodCeiling=0\n[Skin]\nText=\n\n[New]\nValue=1\n"; got != want {
		t.Errorf("patched = %q, want %q", got, want)
	}
}

func TestTextPatchesReplaceTheEntireExistingValueLine(t *testing.T) {
	got := settings.PatchText("[Misc]\nFoodCeiling=100 ; stale note\n", sections(t, `{"Misc":{"FoodCeiling":"200"}}`))
	if got != "[Misc]\nFoodCeiling=200\n" {
		t.Errorf("patched = %q", got)
	}
}

func TestTextPatchesKeepTheINILayout(t *testing.T) {
	for _, nl := range []string{"\n", "\r\n"} {
		lines := func(parts ...string) string { return strings.Join(parts, nl) }
		for _, c := range []struct{ source, sections, want string }{
			// A new key follows the section's last entry, and the final newline survives.
			{lines("[Misc]", "A=1", ""), `{"Misc":{"B":"2"}}`, lines("[Misc]", "A=1", "B=2", "")},
			// A blank line separating sections stays between them, not before the new key.
			{lines("[A]", "X=1", "", "[B]", "Y=2", ""), `{"A":{"K":"v"}}`, lines("[A]", "X=1", "K=v", "", "[B]", "Y=2", "")},
			// A new section gets one blank separator line and keeps the final newline.
			{lines("[A]", "X=1", ""), `{"New":{"K":"v"}}`, lines("[A]", "X=1", "", "[New]", "K=v", "")},
			// No second separator when the source already ends with a blank line.
			{lines("[A]", "X=1", "", ""), `{"New":{"K":"v"}}`, lines("[A]", "X=1", "", "[New]", "K=v", "")},
			// Without a final newline in the source, none is added.
			{lines("[A]", "X=1"), `{"New":{"K":"v"}}`, lines("[A]", "X=1", "", "[New]", "K=v")},
		} {
			wanted := sections(t, c.sections)
			patched := settings.PatchText(c.source, wanted)
			if patched != c.want {
				t.Errorf("PatchText(%q) = %q, want %q", c.source, patched, c.want)
			}
			if again := settings.PatchText(patched, wanted); again != patched {
				t.Errorf("patching %q twice gives %q", patched, again)
			}
		}
	}
}

func TestSectionHeadersFollowedByCommentsAreRecognised(t *testing.T) {
	for _, header := range []string{"[Misc] ; comment", "[Misc]; comment", "[Misc] // comment", "  [Misc]\t;"} {
		got := settings.PatchText(header+"\nA=1\n", sections(t, `{"Misc":{"B":"2"}}`))
		if want := header + "\nA=1\nB=2\n"; got != want {
			t.Errorf("PatchText with header %q = %q", header, got)
		}
	}
}
