package settings

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

type script struct{ name, text string }

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
	all = append(all, layouts(t, "", fixture)...)
	return append(all, layouts(t, "with player 1 not held to its start, ", notHeld)...)
}

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

type sourceMap struct {
	name  string
	files map[string][]byte
	every bool
}

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
		{name: "a script whose main() returns a value",
			files: fixture(files{"war3map.lua": []byte(returnsValue), "war3mapMap.blp": minimapBytes})},
		{name: "bytes that are no map info", files: fixture(files{
			"war3map.w3i": []byte("not a map info"), "war3mapMap.blp": minimapBytes})},
		{name: "with the name the minimap is kept under",
			files: fixture(files{"war3mapMap.blp": minimapBytes, "war3mapminimap.blp": {1}})},
		{name: "with the name a TGA preview takes",
			files: fixture(files{"war3mapMap.blp": minimapBytes, "War3mapMap.TGA": {1}})},
	}
}

func (m sourceMap) onDisk(t testing.TB) string {
	t.Helper()
	dir := t.TempDir()
	for name, data := range m.files {
		testkit.WriteFile(t, dir, name, data)
	}
	return dir
}

func fileOf(err error) string {
	failure, expected := diag.First(err)
	if !expected {
		return "(an error without a file)"
	}
	return failure.File
}

type recordedPlan struct {
	changes   []mapdir.Change
	refused   bool
	refusedAt string
}

func (p recordedPlan) lines() string {
	var out strings.Builder
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

type recordedScripts struct {
	refused   bool
	refusedAt string
	scripts   []string
}

func (r recordedScripts) lines(sources []script) string {
	var out strings.Builder
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

func compact(t testing.TB, document string) string {
	t.Helper()
	var line bytes.Buffer
	if err := json.Compact(&line, []byte(document)); err != nil {
		t.Fatalf("settings %s: %v", document, err)
	}
	return line.String()
}

func recordedSettings(t testing.TB) []byte {
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
			fmt.Fprintf(&part, "the %d scripts:\n%s", len(sources), scriptsWith(t, document, sources).lines(sources))
		}
		for m, source := range folders {
			planned := slices.Contains(routeDocuments, document)
			if source.every {
				planned = slices.Contains(forEveryMap, document)
			}
			if planned {
				fmt.Fprintf(&part, "%s:\n%s", source.name, planFor(t, source, dirs[m], root, document).lines())
			}
		}
		parts[i] = part.String()
	})
	return []byte(strings.Join(parts, ""))
}

func scriptsWith(t testing.TB, document string, sources []script) recordedScripts {
	t.Helper()
	s := settingsOf(t, document)
	info, err := patchInfo(fixtureInfo(t), s, infoFile)
	if err != nil {
		return recordedScripts{refused: true, refusedAt: fileOf(err)}
	}
	made := recordedScripts{}
	for _, source := range sources {
		text, err := afterInfo(source.text, s, info)
		if err != nil {
			text = refusedAt(fileOf(err))
		}
		made.scripts = append(made.scripts, madeOf(source, text))
	}
	return made
}

func refusedAt(file string) string { return "refused: " + file }

func madeOf(source script, text string) string {
	if text == source.text {
		return "unchanged"
	}
	return text
}

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
		return recordedPlan{refused: true, refusedAt: fileOf(err)}
	}
	return recordedPlan{changes: changes}
}

func TestTheSettingsOfEveryDocumentAreAsRecorded(t *testing.T) {
	testkit.Recorded(t, "settings.txt", recordedSettings(t))
}
