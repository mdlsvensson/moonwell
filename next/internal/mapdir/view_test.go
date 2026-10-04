package mapdir

import (
	"reflect"
	"slices"
	"testing"

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
		})
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
	cases := []struct {
		view              *Folder
		name, words, file string
	}{
		{folder, "WAR3MAP.LUA/x.txt", "war3map.lua in the map is a file, not a folder", label + "/war3map.lua"},
		{folder, "textures/old.blp/deep/x.txt", "Textures/Old.blp in the map is a file, not a folder",
			label + "/Textures/Old.blp"},
		{planned, "sound/music/theme.mp3/x.txt", "Sound/Music/theme.mp3 in the map is a file, not a folder",
			label + "/Sound/Music/theme.mp3"},
		{folder, "textures", "textures would replace a folder in the map", label + "/Textures"},
		{planned, "sound/MUSIC", "sound/MUSIC would replace a folder in the map", label + "/Sound/Music"},
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
