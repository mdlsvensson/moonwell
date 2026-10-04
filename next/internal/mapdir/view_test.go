package mapdir

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

// put is a change that writes content; drop is one that removes the file.
func put(name, content string) Change { return Change{Name: name, Bytes: []byte(content)} }
func drop(name string) Change         { return Change{Name: name, Remove: true} }

func TestAViewSeesWhatWasPlannedAndItsFolderDoesNot(t *testing.T) {
	folder, dir := open(t, map[string]string{"a.txt": "old", "gone.txt": "leaving", "kept.txt": "kept"})
	before := testkit.Snapshot(t, dir)
	view := folder.With([]Change{put("a.txt", "new"), drop("gone.txt"), put("made.txt", "made")})

	cases := []struct {
		name            string
		inView, inFirst string
	}{
		{"a.txt", "new", "old"},
		{"gone.txt", "<missing>", "leaving"},
		{"kept.txt", "kept", "kept"},
		{"made.txt", "made", "<missing>"},
	}
	for _, c := range cases {
		if got := read(t, view, c.name); got != c.inView {
			t.Errorf("the view reads %s as %q, want %q", c.name, got, c.inView)
		}
		if got := view.Has(c.name); got != (c.inView != "<missing>") {
			t.Errorf("the view's Has(%q) = %v", c.name, got)
		}
		if got := read(t, folder, c.name); got != c.inFirst {
			t.Errorf("the folder it came from reads %s as %q, want %q", c.name, got, c.inFirst)
		}
		if got := folder.Has(c.name); got != (c.inFirst != "<missing>") {
			t.Errorf("the folder's Has(%q) = %v", c.name, got)
		}
	}
	// The files the folder has, in the order of the scan, then the ones the changes add.
	if got, want := view.Files(), []string{"a.txt", "kept.txt", "made.txt"}; !slices.Equal(got, want) {
		t.Errorf("the view's Files = %q, want %q", got, want)
	}
	if got, want := folder.Files(), []string{"a.txt", "gone.txt", "kept.txt"}; !slices.Equal(got, want) {
		t.Errorf("the folder's Files = %q, want %q", got, want)
	}
	if got := folder.Changes(); len(got) != 0 {
		t.Errorf("the folder's Changes = %v", got)
	}
	if after := testkit.Snapshot(t, dir); !reflect.DeepEqual(after, before) {
		t.Errorf("a view wrote into the folder: %v", after)
	}
}

func TestChangesHoldsEachFileOnceInTheOrderFirstPlanned(t *testing.T) {
	disk := map[string]string{"a.txt": "a", "b.txt": "b", "war3mapskin.txt": "skin"}
	cases := []struct {
		name  string
		steps [][]Change
		want  []Change
	}{
		{"every planned write is a change, even of the bytes the file has",
			[][]Change{{put("a.txt", "a")}}, []Change{put("a.txt", "a")}},
		{"a later change replaces the earlier one where it stands",
			[][]Change{{put("a.txt", "1"), put("b.txt", "1")}, {put("a.txt", "2")}},
			[]Change{put("a.txt", "2"), put("b.txt", "1")}},
		{"within one call too",
			[][]Change{{put("a.txt", "1"), put("b.txt", "1"), put("a.txt", "2")}},
			[]Change{put("a.txt", "2"), put("b.txt", "1")}},
		{"a change lands on the spelling the file has",
			[][]Change{{put("war3mapSkin.txt", "new"), drop("A.TXT")}},
			[]Change{put("war3mapskin.txt", "new"), drop("a.txt")}},
		{"and on the spelling an earlier change gave a new file",
			[][]Change{{put("New.txt", "1")}, {put("NEW.TXT", "2")}}, []Change{put("New.txt", "2")}},
		{"a new file is named as given, with forward slashes",
			[][]Change{{put(`Textures\New.blp`, "new")}}, []Change{put("Textures/New.blp", "new")}},
		{"removing a file the view does not have does nothing",
			[][]Change{{drop("missing.txt")}}, []Change{}},
		{"a file removed twice is removed once",
			[][]Change{{drop("a.txt")}, {drop("A.txt")}}, []Change{drop("a.txt")}},
		{"a file removed and written again is one write, under the spelling it has",
			[][]Change{{drop("a.txt"), put("b.txt", "1")}, {put("A.TXT", "back")}},
			[]Change{put("a.txt", "back"), put("b.txt", "1")}},
		{"a new file that is removed again changes nothing",
			[][]Change{{put("new.txt", "1"), put("b.txt", "1"), put("other.txt", "1")}, {drop("NEW.txt")}},
			[]Change{put("b.txt", "1"), put("other.txt", "1")}},
		{"and written once more it is planned last",
			[][]Change{{put("new.txt", "1"), put("b.txt", "1")}, {drop("new.txt")}, {put("new.txt", "2")}},
			[]Change{put("b.txt", "1"), put("new.txt", "2")}},
		{"a new file removed in the call that planned it changes nothing",
			[][]Change{{put("new.txt", "1"), put("b.txt", "1"), drop("NEW.txt"), put("other.txt", "1")}},
			[]Change{put("b.txt", "1"), put("other.txt", "1")}},
		{"and written once more in that call it is planned last, as it is then named",
			[][]Change{{put("new.txt", "1"), put("b.txt", "1"), drop("new.txt"), put("New.txt", "2")}},
			[]Change{put("b.txt", "1"), put("New.txt", "2")}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			view, _ := open(t, disk)
			for _, step := range c.steps {
				view = view.With(step)
			}
			if got := view.Changes(); !slices.EqualFunc(got, c.want, sameChange) {
				t.Errorf("Changes = %s, want %s", show(got), show(c.want))
			}
			// Each change is found where Changes lists it.
			for _, change := range c.want {
				content := string(change.Bytes)
				if change.Remove {
					content = "<missing>"
				}
				if got := read(t, view, change.Name); got != content {
					t.Errorf("the view reads %s as %q, want %q", change.Name, got, content)
				}
			}
		})
	}
}

func TestWithSpellsTheFoldersOfANewFileAsTheMapAndEarlierChangesDo(t *testing.T) {
	disk := map[string]string{"Textures/Old.blp": "old", "war3map.lua": "script"}
	cases := []struct {
		name  string
		steps [][]string // the new files of each With, as a planner names them
		want  []string   // as the view names them, in the order planned
	}{
		{"two spellings of a new folder in one call",
			[][]string{{"Sound/a.mp3", "sound/b.mp3"}}, []string{"Sound/a.mp3", "Sound/b.mp3"}},
		{"and in two calls",
			[][]string{{"Sound/a.mp3"}, {`SOUND\b.mp3`}}, []string{"Sound/a.mp3", "Sound/b.mp3"}},
		{"a folder the map has",
			[][]string{{"textures/New.blp", `TEXTURES\Sub\Other.blp`}},
			[]string{"Textures/New.blp", "Textures/Sub/Other.blp"}},
		{"nested new folders keep the first spellings",
			[][]string{{"A/B/x"}, {"a/b/y", "a/C/z", "A/c/b/w"}}, []string{"A/B/x", "A/B/y", "A/C/z", "A/C/b/w"}},
		{"the last name of a new file stays as given",
			[][]string{{"Sound/Theme.mp3", "sound/THEME.wav"}}, []string{"Sound/Theme.mp3", "Sound/THEME.wav"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			folder, _ := open(t, disk)
			view := folder
			var given []string
			for _, step := range c.steps {
				var changes []Change
				for _, name := range step {
					changes = append(changes, put(name, name))
				}
				view = view.With(changes)
				given = append(given, step...)
			}
			if got := names(view.Changes()); !slices.Equal(got, c.want) {
				t.Errorf("Changes names the files %q, want %q", got, c.want)
			}
			wantFiles := append([]string{"Textures/Old.blp", "war3map.lua"}, c.want...)
			if got := view.Files(); !slices.Equal(got, wantFiles) {
				t.Errorf("Files = %q, want %q", got, wantFiles)
			}
			// Laid one file a view, each file gets the name Place gave before it was laid: the same name.
			one := folder
			for i, name := range given {
				if got := view.Name(strings.ToUpper(name)); got != c.want[i] {
					t.Errorf("Name of %s in capitals = %q, want %q", name, got, c.want[i])
				}
				if got := read(t, view, name); got != name {
					t.Errorf("the view reads %s as %q", name, got)
				}
				placed, err := one.Place(name)
				if err != nil || placed != c.want[i] {
					t.Errorf("Place(%q) = %q, %v, want %q", name, placed, err, c.want[i])
				}
				one = one.With([]Change{put(name, name)})
				if got := one.Name(name); got != placed {
					t.Errorf("With stores %s as %q, Place said %q", name, got, placed)
				}
			}
		})
	}
}

func TestWithRespellsOnlyTheFoldersItKnowsAndLeavesTheRestOfANameAsGiven(t *testing.T) {
	disk := map[string]string{"war3map.w3i": "info", "Textures/Old.blp": "old"}
	cases := []struct {
		name          string
		given, stored string
	}{
		// None of these can be written. Each stays in the plan as it was given, so that the check before the first
		// write refuses the plan, and the name is not taken for the file its tidied name would be.
		{"a leading slash before a file the map has", "/WAR3MAP.W3I", "/WAR3MAP.W3I"},
		{"two leading slashes", "//war3map.w3i", "//war3map.w3i"},
		{"a leading slash before a folder the map has", "/textures/New.blp", "/textures/New.blp"},
		{"an empty folder name", "a//b", "a//b"},
		{"a way out of a folder", "a/../b", "a/../b"},
		{"a dot", "./war3map.w3i", "./war3map.w3i"},
		{"a slash at the end", "new/", "new/"},
		{"no name", "", ""},
		// The folder the map has is respelled; what follows it stays.
		{"an empty folder name below a folder the map has", "textures//New.blp", "Textures//New.blp"},
		{"a way out of a folder the map has", `TEXTURES\..\WAR3MAP.W3I`, "Textures/../WAR3MAP.W3I"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			folder, _ := open(t, disk)
			view := folder.With([]Change{put(c.given, "new")})
			if got, want := names(view.Changes()), []string{c.stored}; !slices.Equal(got, want) {
				t.Errorf("Changes names the files %q, want %q", got, want)
			}
			// The files the map has are as they were.
			if got := read(t, view, "war3map.w3i"); got != "info" {
				t.Errorf("the view reads war3map.w3i as %q", got)
			}
		})
	}
}

func TestWithGivesTwoSpellingsOfAFolderOne(t *testing.T) {
	folder, _ := open(t, map[string]string{"textures/Old.blp": "old"})
	view := folder.With([]Change{put("Textures/new.blp", "new"), put("Sound/a.mp3", "a"), put("sound/b.mp3", "b")})
	want := []string{"textures/new.blp", "Sound/a.mp3", "Sound/b.mp3"}
	if got := names(view.Changes()); !slices.Equal(got, want) {
		t.Errorf("Changes names the files %q, want %q", got, want)
	}
}

// names is the name of each change.
func names(changes []Change) []string {
	listed := []string{}
	for _, change := range changes {
		listed = append(listed, change.Name)
	}
	return listed
}

func TestWithLaysManyNewFilesInOneCallQuickly(t *testing.T) {
	folder, _ := open(t, map[string]string{"war3map.w3i": "info"})
	const count = 20000
	changes := make([]Change, count)
	for i := range changes {
		// The first file of each of fifty folders spells the folders; the rest name them in lower case.
		name := fmt.Sprintf("units/folder%d/Model%d.mdx", i%50, i)
		if i < 50 {
			name = fmt.Sprintf("Units/Folder%d/Model%d.mdx", i, i)
		}
		changes[i] = Change{Name: name}
	}
	start := time.Now()
	view := folder.With(changes)
	planned := view.Changes()
	files := view.Files()
	took := time.Since(start)

	if len(planned) != count || len(files) != count+1 {
		t.Fatalf("the view plans %d changes and has %d files, want %d and %d", len(planned), len(files), count, count+1)
	}
	for i, change := range planned {
		if want := fmt.Sprintf("Units/Folder%d/Model%d.mdx", i%50, i); change.Name != want {
			t.Fatalf("change %d is named %q, want %q", i, change.Name, want)
		}
	}
	// The work is in proportion to the files: a slow machine does it in a fraction of this bound, while work that
	// grows with the square of them does not.
	if took > 2*time.Second {
		t.Errorf("laying %d new files took %v", count, took)
	}
}

func sameChange(a, b Change) bool {
	return a.Name == b.Name && a.Remove == b.Remove && string(a.Bytes) == string(b.Bytes)
}

// show lists changes as "name=content" and "-name".
func show(changes []Change) []string {
	shown := []string{}
	for _, change := range changes {
		if change.Remove {
			shown = append(shown, "-"+change.Name)
		} else {
			shown = append(shown, change.Name+"="+string(change.Bytes))
		}
	}
	return shown
}

func TestAViewOverAViewLeavesTheFirstAsItWas(t *testing.T) {
	folder, _ := open(t, map[string]string{"a.txt": "a"})
	first := folder.With([]Change{put("a.txt", "1"), put("new.txt", "1")})
	second := first.With([]Change{put("A.txt", "2"), drop("new.txt"), put("more.txt", "2")})
	if got, want := show(first.Changes()), []string{"a.txt=1", "new.txt=1"}; !slices.Equal(got, want) {
		t.Errorf("the first view's Changes = %s, want %s", got, want)
	}
	if got, want := show(second.Changes()), []string{"a.txt=2", "more.txt=2"}; !slices.Equal(got, want) {
		t.Errorf("the second view's Changes = %s, want %s", got, want)
	}
	if got := read(t, first, "new.txt"); got != "1" {
		t.Errorf("the first view reads new.txt as %q", got)
	}
	// The list a caller gets is its own.
	listed := first.Changes()
	listed[0] = drop("a.txt")
	if got := read(t, first, "a.txt"); got != "1" {
		t.Errorf("after changing the list Changes returned, the view reads a.txt as %q", got)
	}
}

func TestAViewKeepsTheSpellingOfAFileItRemoves(t *testing.T) {
	folder, _ := open(t, map[string]string{"war3mapMap.blp": "minimap"})
	view := folder.With([]Change{drop("war3mapmap.BLP")})
	if view.Has("war3mapMap.blp") {
		t.Error("the view still has the file it removes")
	}
	// An error about the file names it as the source map spells it.
	if got := view.Label("WAR3MAPMAP.BLP"); got != label+"/war3mapMap.blp" {
		t.Errorf("Label = %q", got)
	}
	if got, want := view.Files(), []string{}; !slices.Equal(got, want) {
		t.Errorf("Files = %q", got)
	}
}

func TestPlaceKeepsTheSpellingOfFoldersAndFilesTheMapHas(t *testing.T) {
	folder, _ := open(t, map[string]string{"Textures/Old.blp": "old", "war3map.lua": "script", "Units/Hero/a.txt": ""})
	planned := folder.With([]Change{put("Sound/Music/theme.mp3", "theme"), drop("Units/Hero/a.txt")})
	cases := []struct {
		view        *Folder
		name, place string
	}{
		{folder, "textures/New.blp", "Textures/New.blp"},
		{folder, `TEXTURES\sub\New.blp`, "Textures/sub/New.blp"},
		// Two new folders below a folder the map has are spelled as given.
		{folder, "textures/Sub/Deep/New.blp", "Textures/Sub/Deep/New.blp"},
		{folder, "textures/old.BLP", "Textures/Old.blp"},
		{folder, "WAR3MAP.LUA", "war3map.lua"},
		{folder, "top.txt", "top.txt"},
		{folder, "sound/music/b.mp3", "sound/music/b.mp3"},
		// A folder an earlier change planned keeps the spelling that change gave it.
		{planned, "sound/music/b.mp3", "Sound/Music/b.mp3"},
		{planned, "SOUND/Effects/hit.wav", "Sound/Effects/hit.wav"},
		{planned, "sound/music/THEME.mp3", "Sound/Music/theme.mp3"},
		// A folder of the map stays one when the view removes the file in it.
		{planned, "units/hero/b.txt", "Units/Hero/b.txt"},
	}
	for _, c := range cases {
		got, err := c.view.Place(c.name)
		if err != nil || got != c.place {
			t.Errorf("Place(%q) = %q, %v, want %q", c.name, got, err, c.place)
		}
	}
}

func TestPlaceRefusesAWayThroughAFileAndANameThatIsAFolder(t *testing.T) {
	folder, _ := open(t, map[string]string{"Textures/Old.blp": "old", "war3map.lua": "script"})
	planned := folder.With([]Change{put("Sound/Music/theme.mp3", "theme")})
	removing := folder.With([]Change{drop("WAR3MAP.LUA"), drop("textures/old.blp")})
	cases := []struct {
		view              *Folder
		name, words, file string
	}{
		{folder, "WAR3MAP.LUA/x.txt", "war3map.lua in the map is a file, not a folder", label + "/war3map.lua"},
		// A file of the map never becomes a folder, so it is on the way even in a view that removes it.
		{removing, "WAR3MAP.LUA/x.txt", "war3map.lua in the map is a file, not a folder", label + "/war3map.lua"},
		{removing, "textures/old.blp/deep/x.txt", "Textures/Old.blp in the map is a file, not a folder",
			label + "/Textures/Old.blp"},
		{folder, "textures/old.blp/deep/x.txt", "Textures/Old.blp in the map is a file, not a folder",
			label + "/Textures/Old.blp"},
		{planned, "sound/music/theme.mp3/x.txt", "Sound/Music/theme.mp3 in the map is a file, not a folder",
			label + "/Sound/Music/theme.mp3"},
		{folder, "textures", "textures would replace a folder in the map", label + "/Textures"},
		{planned, "sound/MUSIC", "sound/MUSIC would replace a folder in the map", label + "/Sound/Music"},
		// A removal leaves the folder: one whose only file the view removes is in the way, as IsFolder says.
		{removing, "TEXTURES", "TEXTURES would replace a folder in the map", label + "/Textures"},
	}
	for _, c := range cases {
		got, err := c.view.Place(c.name)
		e := asError(t, err)
		if got != "" || !contains(e.Msg, c.words) || e.File != c.file || e.Hint == "" {
			t.Errorf("Place(%q) = %q, %+v, want %q at %s", c.name, got, e, c.words, c.file)
		}
		if !contains(e.Msg, c.name) {
			t.Errorf("Place(%q) does not name what was asked for: %q", c.name, e.Msg)
		}
	}
}

// A planner asks for the place of a fixed name or of one it has checked, so a name no file can have is its bug:
// it shows at the planner, as a plain error, and not only when the plan is written.
func TestPlaceRefusesANameNoFileCanHaveAsItsCallersBug(t *testing.T) {
	folder, _ := open(t, map[string]string{"war3map.w3i": "info", "Textures/Old.blp": "old"})
	unwritable := []string{
		"", "/WAR3MAP.W3I", "//war3map.w3i", "/textures/New.blp", "a//b", "a/../b", "./war3map.w3i", "new/",
		"../outside.txt", "textures//New.blp", `TEXTURES\..\WAR3MAP.W3I`, "what?.blp", "Sound/nul.mp3", "Sound/a.mp3 ",
	}
	for _, name := range unwritable {
		placed, err := folder.Place(name)
		text := asPlannersBug(t, err)
		if placed != "" || !contains(text, fmt.Sprintf("Cannot place %q", name)) || !contains(text, "relative path") {
			t.Errorf("Place(%q) = %q, %q, want a refusal that names it", name, placed, text)
		}
	}
}

func TestIsFolderIsAFolderTheScanFoundOrOneAPlannedChangeMakes(t *testing.T) {
	dir := write(t, map[string]string{"Textures/Old.blp": "old", "Units/Hero/a.txt": "", "war3map.lua": "script"})
	if err := os.Mkdir(filepath.Join(dir, "Empty"), 0o777); err != nil {
		t.Fatal(err)
	}
	folder, err := Open(dir, label)
	if err != nil {
		t.Fatal(err)
	}
	planned := folder.With([]Change{put("Sound/Music/theme.mp3", "theme"), drop("Units/Hero/a.txt")})
	takenBack := planned.With([]Change{drop("sound/music/THEME.mp3")})
	oneLeft := planned.With([]Change{put("sound/Effects/hit.wav", "hit")}).With([]Change{drop("Sound/Music/theme.mp3")})
	cases := []struct {
		what string
		view *Folder
		name string
		want bool
	}{
		{"a folder of the map", folder, "Textures", true},
		{"in another letter case", folder, "textures", true},
		{"a folder below a folder, with a backslash", folder, `UNITS\hero`, true},
		{"a folder without a file", folder, "empty", true},
		{"a file of the map", folder, "war3map.lua", false},
		{"a file in a folder", folder, "Textures/Old.blp", false},
		{"a name the map does not have", folder, "Sound", false},
		{"the map folder itself", folder, "", false},
		{"a folder's name with a slash after it", folder, "Textures/", false},
		{"a folder a planned file makes", planned, "sound", true},
		{"and the folder below it", planned, "SOUND/music", true},
		{"the planned file itself", planned, "Sound/Music/theme.mp3", false},
		// A removal takes a file away and leaves the folder it was in.
		{"a folder of the map whose one file the view removes", planned, "units/hero", true},
		{"and the folder above it", planned, "Units", true},
		// A folder that only planned files make is one as long as the view plans a file in it.
		{"a planned folder whose one write is taken back", takenBack, "sound", false},
		{"and the folder that was below it", takenBack, "sound/music", false},
		{"a planned folder with one of its two writes taken back", oneLeft, "sound", true},
		{"the folder of the write that is left", oneLeft, "Sound/effects", true},
		{"the folder of the write that is taken back", oneLeft, "sound/music", false},
	}
	for _, c := range cases {
		if got := c.view.IsFolder(c.name); got != c.want {
			t.Errorf("%s: IsFolder(%q) = %v, want %v", c.what, c.name, got, c.want)
		}
	}
}
