package settings

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/mapdir"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/war3/w3i"
)

// ---- the scripts ----

// script is a war3map.lua, or a text given as one.
type script struct{ name, text string }

// scripts is the fixture's script, every script the tests of the Lua patch or see refused, and the fixture's in
// other layouts.
func scripts(t testing.TB) []script {
	t.Helper()
	fixture := fixtureLua(t)
	notHeld := swapped(t, fixture, "ForcePlayerStartLocation(Player(1), 1)\r\n", "")
	all := []script{
		{"the fixture", fixture},
		{"without SetMapName", swapped(t, fixture, "SetMapName(", "Other(")},
		{"with a second config()", fixture + "\nfunction config() SetMapName(\"x\") end"},
		{"with SetMapName of an object", swapped(t, fixture, "SetMapName(", "object.SetMapName(")},
		{"a script that does not read", "function (((unreadable"},
		{"with player 1 not held to its start", notHeld},
		{"with a SetPlayerName", swapped(t, fixture, "SetPlayerColor(Player(1), ConvertPlayerColor(1))",
			"SetPlayerColor(Player(1), ConvertPlayerColor(1))\r\nSetPlayerName(Player(1), \"TRIGSTR_006\")")},
		{"a main() alone, indented", indentedMain},
	}
	for _, shape := range unsafeShapes {
		name := fmt.Sprintf("with %q for %q", shape.new, shape.old)
		all = append(all, script{name, swapped(t, fixture, shape.old, shape.new)})
	}
	for _, c := range joinable {
		all = append(all, script{fmt.Sprintf("the script %q", c.source), c.source})
	}
	others := slices.Concat(slices.Sorted(maps.Keys(minimapSources)), slices.Sorted(maps.Keys(withoutOneMain)))
	for _, source := range others {
		all = append(all, script{fmt.Sprintf("the script %q", source), source})
	}
	// A call that is added takes its line ending, its indentation and its semicolon from the layout, and the one
	// call a setting adds to a player is the one that holds the player to its start: each layout comes once more
	// without that call for player 1.
	all = append(all, layouts(t, "", fixture)...)
	return append(all, layouts(t, "with player 1 not held to its start, ", notHeld)...)
}

// layouts is a script of the fixture's shape written in other ways that say the same. Each is named by what
// stands before its name.
func layouts(t testing.TB, before, fixture string) []script {
	t.Helper()
	oneLine := strings.ReplaceAll(swapped(t, fixture, "--\r\n", ""), "\r\n", " ")
	together := swapped(t, fixture,
		"SetPlayerStartLocation(Player(0), 0)\r\nForcePlayerStartLocation(Player(0), 0)\r\nSetPlayerColor",
		"SetPlayerStartLocation(Player(0), 0)ForcePlayerStartLocation(Player(0), 0)SetPlayerColor")
	together = swapped(t, together, "NewSoundEnvironment(\"Default\")\r\n", "")
	together = swapped(t, together, "SetMapMusic(\"Music\", true, 0)\r\nCreateAllUnits()\r\n",
		"SetMapMusic(\"Music\", true, 0)\r\nNewSoundEnvironment(\"Default\")ResetTerrainFog()CreateAllUnits()")
	return []script{
		{before + "with the line endings of Unix", strings.ReplaceAll(fixture, "\r\n", "\n")},
		{before + "with a semicolon after every call", strings.ReplaceAll(fixture, ")\r\n", ");\r\n")},
		{before + "with every line indented", strings.ReplaceAll(fixture, "\r\n", "\r\n\t  ")},
		{before + "on one line", oneLine},
		{before + "on one line with semicolons", strings.ReplaceAll(oneLine, ") ", "); ")},
		{before + "with calls that touch", together},
	}
}

// inPlainDecimal reports whether the settings make a script take, from the map info given as bytes, a number
// that is not 0 and is below 0.000001 in size or from 1e21: a position of a player whose position is set, or a
// start, an end or a density of a fog that is set and shown. A script takes such a number in plain decimal.
func inPlainDecimal(s manifest.Settings, patchedInfo []byte) bool {
	info, err := w3i.Read(patchedInfo, infoFile, w3i.Extended)
	if err != nil {
		return false
	}
	small := func(values ...float32) bool {
		return slices.ContainsFunc(values, func(value float32) bool {
			size := math.Abs(float64(value))
			return size != 0 && (size < 0.000001 || size >= 1e21)
		})
	}
	for _, player := range info.Details.Players {
		override := s.Players[int(player.ID.Value)]
		if (override.X != nil || override.Y != nil) && small(player.X.Value, player.Y.Value) {
			return true
		}
	}
	fog := info.Details.Fog
	return s.Environment.Fog != (manifest.Fog{}) && info.Flags.Value&fogOn != 0 &&
		small(fog.Start.Value, fog.End.Value, fog.Density.Value)
}

// onEveryCore runs each piece of work on one of as many goroutines as the machine runs at once, and returns when
// all are done.
func onEveryCore(pieces int, work func(piece int)) {
	queue := make(chan int)
	var workers sync.WaitGroup
	for range runtime.GOMAXPROCS(0) {
		workers.Go(func() {
			for piece := range queue {
				work(piece)
			}
		})
	}
	for piece := range pieces {
		queue <- piece
	}
	close(queue)
	workers.Wait()
}

// ---- the map folders ----

// planProject is a project folder that holds, at each path a document names as its preview, a picture of the
// kind the path says. The picture that documents name in two spellings is written under both: a file system
// that keeps the spellings apart then has two files and another has one, and both give the same bytes for either.
func planProject(t testing.TB) string {
	t.Helper()
	root := t.TempDir()
	picture := testkit.NewPixels(256)
	packed := testkit.TGA(picture, testkit.TGAOptions{RLE: true, Depth: 24, FromTop: true})
	for _, file := range []struct {
		name string
		data []byte
	}{
		{"preview.blp", testkit.BLP(256, 1)},
		{"p.blp", testkit.BLP(512, 0)},
		{"preview.tga", plainTGA()},
		{"art/Preview.TGA", packed},
		{"art/preview.tga", packed},
		{"art/Preview.PNG", testkit.PNG(picture, "rgba")},
		{"art/p.png", testkit.PNG(testkit.GreyPixels(256), "grey")},
	} {
		testkit.WriteFile(t, root, file.name, file.data)
	}
	return root
}

// sourceMap is a map folder that the settings are planned for.
type sourceMap struct {
	name  string
	files map[string][]byte
	// every says that every document is planned for the folder. For the others it is those of routeDocuments.
	every bool
	// returns says that the main() of its script ends in a return of a value, after which no call can stand.
	returns bool
}

// sourceMaps is the map folders the settings are planned for. The first two get every document: the fixture as
// it is, which has neither of the optional text files and no minimap, and the fixture with every file a plan
// reads, each text file behind a byte order mark. The others get the documents of routeDocuments: the fixture
// with each optional file alone and with all, with files that hold nothing, under other spellings, and each
// folder that a plan refuses for a file it lacks, cannot read as text or cannot patch, or for a name it has.
func sourceMaps(t testing.TB) []sourceMap {
	t.Helper()
	info, script := fixtureInfo(t), fixtureLua(t)
	misc := "[Misc]\r\nFoodCeiling=100\r\nKeep=1\r\n\r\n[Other]\r\nA=0\r\n"
	skin := "[CustomSkin]\nTest=old\n\n[A]\nOld=1"
	type files = map[string][]byte
	fixture := func(more files) files {
		all := files{"war3map.w3i": info, "war3map.lua": []byte(script)}
		maps.Copy(all, more)
		return all
	}
	returnsValue := swapped(t, script, "RunInitializationTriggers()\r\nend",
		"RunInitializationTriggers()\r\nreturn 1\r\nend")
	return []sourceMap{
		{name: "the fixture", every: true, files: fixture(nil)},
		{name: "every file, each text file behind a byte order mark", every: true, files: fixture(files{
			"war3map.lua": []byte(byteOrderMark + script), "war3mapMisc.txt": []byte(byteOrderMark + misc),
			"war3mapSkin.txt": []byte(byteOrderMark + skin), "war3mapMap.blp": minimapBytes})},
		{name: "every file", files: fixture(files{
			"war3mapMisc.txt": []byte(misc), "war3mapSkin.txt": []byte(skin), "war3mapMap.blp": minimapBytes})},
		{name: "with war3mapMisc.txt alone", files: fixture(files{"war3mapMisc.txt": []byte(misc)})},
		{name: "with war3mapSkin.txt alone", files: fixture(files{"war3mapSkin.txt": []byte(skin)})},
		{name: "with the minimap alone", files: fixture(files{"war3mapMap.blp": minimapBytes})},
		{name: "with text files that hold nothing",
			files: fixture(files{"war3mapMisc.txt": {}, "war3mapSkin.txt": {}})},
		{name: "every file under another spelling", files: files{
			"WAR3MAP.W3I": info, "War3Map.Lua": []byte(script), "WAR3MAPMISC.TXT": []byte(misc),
			"war3mapskin.txt": []byte(skin), "WAR3MAPMAP.BLP": minimapBytes}},
		{name: "an empty folder"},
		{name: "without the script", files: files{"war3map.w3i": info, "war3mapMap.blp": minimapBytes}},
		{name: "without the map info", files: files{"war3map.lua": []byte(script), "war3mapMap.blp": minimapBytes}},
		{name: "a script that is not UTF-8", files: fixture(files{
			"war3map.lua": {0x66, 0xff, 0x66}, "war3mapMap.blp": minimapBytes})},
		{name: "text files that are not UTF-8", files: fixture(files{
			"war3mapMisc.txt": {0xc3}, "war3mapSkin.txt": []byte(byteOrderMark + "[A]\n\xff")})},
		{name: "a script without SetMapName", files: fixture(files{
			"war3map.lua": []byte(swapped(t, script, "SetMapName(", "Other(")), "war3mapMap.blp": minimapBytes})},
		{name: "a script whose main() returns a value", returns: true,
			files: fixture(files{"war3map.lua": []byte(returnsValue), "war3mapMap.blp": minimapBytes})},
		{name: "bytes that are no map info", files: fixture(files{
			"war3map.w3i": []byte("not a map info"), "war3mapMap.blp": minimapBytes})},
		{name: "with the name the minimap is kept under",
			files: fixture(files{"war3mapMap.blp": minimapBytes, "war3mapminimap.blp": {1}})},
		{name: "with the name a TGA preview takes",
			files: fixture(files{"war3mapMap.blp": minimapBytes, "War3mapMap.TGA": {1}})},
	}
}

// onDisk writes the map folder into a temporary folder and returns its path.
func (m sourceMap) onDisk(t testing.TB) string {
	t.Helper()
	dir := t.TempDir()
	for name, data := range m.files {
		testkit.WriteFile(t, dir, name, data)
	}
	return dir
}

// held is the bytes of the file the map folder has under name, in any letter case.
func (m sourceMap) held(name string) []byte {
	for spelled, data := range m.files {
		if mapdir.Key(spelled) == mapdir.Key(name) {
			return data
		}
	}
	return nil
}

// changeTo is the change a plan makes to the file under name, in any letter case.
func changeTo(changes []mapdir.Change, name string) (mapdir.Change, bool) {
	for _, change := range changes {
		if mapdir.Key(change.Name) == mapdir.Key(name) {
			return change, true
		}
	}
	return mapdir.Change{}, false
}

// ---- the recording ----

// The two notes of the recording. Each stands beside a result that the table of what a user can notice, in the
// design of this program (its §8), has a row for, and quotes that row.
const (
	notePlainDecimal = `§8, "A number below 0.000001 or from 1e21 in war3map.lua is written in plain decimal"`
	noteAfterAReturn = `§8, "A preview on a map whose main() ends in return <value> is refused"`
)

// fileOf is the file that a refusal names.
func fileOf(err error) string {
	failure, expected := diag.First(err)
	if !expected {
		return "(an error without a file)"
	}
	return failure.File
}

// recordedPlan is what a plan came to: its changes, or the file its refusal names.
type recordedPlan struct {
	changes   []mapdir.Change
	refused   bool
	refusedAt string
	note      string
}

// lines is the plan as a recording holds it, each line indented: the file a refusal names; or a line for each
// change, which holds a text file whole, as Go quotes a string, and any other file as its digest.
func (p recordedPlan) lines() string {
	var out strings.Builder
	if p.note != "" {
		fmt.Fprintf(&out, "  note: %s\n", p.note)
	}
	switch {
	case p.refused:
		fmt.Fprintf(&out, "  refused: %s\n", p.refusedAt)
	case len(p.changes) == 0:
		out.WriteString("  no change\n")
	}
	for _, change := range p.changes {
		what := testkit.Digest(change.Bytes)
		switch {
		case change.Remove:
			what = "removed"
		case strings.HasSuffix(mapdir.Key(change.Name), ".txt"):
			what = strconv.Quote(string(change.Bytes))
		}
		fmt.Fprintf(&out, "  %s: %s\n", change.Name, what)
	}
	return out.String()
}

// recordedScripts is what the settings of one document make of every script, for the fixture's map info with the
// settings in it: for each script its text with the settings, "unchanged", or "refused: " and the file the
// refusal names. A document that the fixture's map info refuses has no script to patch.
type recordedScripts struct {
	refused   bool
	refusedAt string
	scripts   []string
	note      string
}

// lines is the scripts as a recording holds them: one digest for all of them, each after its name; "as they are"
// where no script changes; or the file that the map info's refusal names.
func (r recordedScripts) lines(sources []script) string {
	var out strings.Builder
	if r.note != "" {
		fmt.Fprintf(&out, "  note: %s\n", r.note)
	}
	if r.refused {
		fmt.Fprintf(&out, "  refused: %s\n", r.refusedAt)
		return out.String()
	}
	if !slices.ContainsFunc(r.scripts, func(made string) bool { return made != "unchanged" }) {
		out.WriteString("  as they are\n")
		return out.String()
	}
	var all bytes.Buffer
	for i, made := range r.scripts {
		fmt.Fprintf(&all, "%s\n%s\n", sources[i].name, made)
	}
	fmt.Fprintf(&out, "  %s\n", testkit.Digest(all.Bytes()))
	return out.String()
}

// recorder gives the recording what one document makes: of every script, and of one map folder. The folder is
// on disk at dir, and root is the project folder with the preview pictures.
type recorder struct {
	scripts func(document string, sources []script) recordedScripts
	plan    func(source sourceMap, dir, root, document string) recordedPlan
}

// compact is a document on one line, without the white space between its values.
func compact(t testing.TB, document string) string {
	t.Helper()
	var line bytes.Buffer
	if err := json.Compact(&line, []byte(document)); err != nil {
		t.Fatalf("settings %s: %v", document, err)
	}
	return line.String()
}

// recordedSettings is the recording. It has a part for each document: what the document makes of the scripts,
// when it is one that every script is patched for, and then its plan for each map folder it is planned for.
// Every document is planned for the map folders that say so, and the documents of routeDocuments for the others.
// The documents come in the order of their lists, each once.
func recordedSettings(t testing.TB, by recorder) []byte {
	t.Helper()
	forScripts, forEveryMap := scriptDocuments(), slices.Concat(documents(), constantDocuments(), duplicateDocuments)
	var all []string
	for _, document := range slices.Concat(forEveryMap, forScripts, routeDocuments) {
		if !slices.Contains(all, document) {
			all = append(all, document)
		}
	}
	sources, root, folders := scripts(t), planProject(t), sourceMaps(t)
	dirs := make([]string, len(folders))
	for i, source := range folders {
		dirs[i] = source.onDisk(t)
	}
	// A document that does not decode stops the test here, where a test may be stopped: the documents are made
	// into their parts on several goroutines.
	titles := make([]string, len(all))
	for i, document := range all {
		projectOf(t, root, document)
		titles[i] = compact(t, document)
	}
	parts := make([]string, len(all))
	onEveryCore(len(all), func(i int) {
		document := all[i]
		var part strings.Builder
		fmt.Fprintf(&part, "== %s\n", titles[i])
		if slices.Contains(forScripts, document) {
			fmt.Fprintf(&part, "the %d scripts:\n%s", len(sources), by.scripts(document, sources).lines(sources))
		}
		for m, source := range folders {
			planned := slices.Contains(routeDocuments, document)
			if source.every {
				planned = slices.Contains(forEveryMap, document)
			}
			if planned {
				fmt.Fprintf(&part, "%s:\n%s", source.name, by.plan(source, dirs[m], root, document).lines())
			}
		}
		parts[i] = part.String()
	})
	return []byte(strings.Join(parts, ""))
}

// scriptsWith is what the settings of a document make of every script.
func scriptsWith(t testing.TB, document string, sources []script) recordedScripts {
	t.Helper()
	s := settingsOf(t, document)
	info, err := patchInfo(fixtureInfo(t), s, infoFile)
	if err != nil {
		return recordedScripts{refused: true, refusedAt: fileOf(err)}
	}
	made := recordedScripts{}
	if inPlainDecimal(s, info) {
		made.note = notePlainDecimal
	}
	for _, source := range sources {
		text, err := afterInfo(source.text, s, info)
		if err != nil {
			text = refusedAt(fileOf(err))
		}
		made.scripts = append(made.scripts, madeOf(source, text))
	}
	return made
}

// refusedAt is what stands for a script that the settings do not go into: the file its refusal names.
func refusedAt(file string) string { return "refused: " + file }

// madeOf is what stands for a script with the settings: its text, or "unchanged" for the script as it was.
func madeOf(source script, text string) string {
	if text == source.text {
		return "unchanged"
	}
	return text
}

// planFor is the plan of a document for a map folder.
func planFor(t testing.TB, source sourceMap, dir, root, document string) recordedPlan {
	t.Helper()
	project := projectOf(t, root, document)
	folder, err := mapdir.Open(dir, mapLabel)
	if err != nil {
		t.Errorf("%s: %v", source.name, err)
		return recordedPlan{}
	}
	changes, err := Plan(folder, project)
	if err != nil {
		refused := recordedPlan{refused: true, refusedAt: fileOf(err)}
		// The one refusal for the script of a map whose main() returns a value is that of the call a preview adds.
		if source.returns && project.Settings.Info.Preview != nil && refused.refusedAt == folder.Label(luaName) {
			refused.note = noteAfterAReturn
		}
		return refused
	}
	planned := recordedPlan{changes: changes}
	info := source.held(infoName)
	if change, patched := changeTo(changes, infoName); patched {
		info = change.Bytes
	}
	if inPlainDecimal(project.Settings, info) {
		planned.note = notePlainDecimal
	}
	return planned
}

// TestTheSettingsOfEveryDocumentAreAsRecorded holds what every settings document makes of every script, and
// what it changes in map folders of every kind, to a recording: one file for some seven thousand scripts and
// some seven hundred plans. A script stands in it with the others of its document, as one digest, and a plan as
// its changes: a text file whole, any other file as its digest. A byte that changes in what the settings write is
// a line of a diff under the document, and for a plan under the map folder.
func TestTheSettingsOfEveryDocumentAreAsRecorded(t *testing.T) {
	testkit.Recorded(t, "settings.txt", recordedSettings(t, recorder{
		scripts: func(document string, sources []script) recordedScripts {
			return scriptsWith(t, document, sources)
		},
		plan: func(source sourceMap, dir, root, document string) recordedPlan {
			return planFor(t, source, dir, root, document)
		},
	}))
}
