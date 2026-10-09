package build

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func projectWith(root, buildFolder, manifestFile string) *manifest.Project {
	return &manifest.Project{
		Root:         root,
		ManifestName: manifestFile,
		Map:          manifest.Map{Folder: "map.w3x", Entry: "src/main.yue"},
		Build:        manifest.Build{Folder: buildFolder},
	}
}

func TestArchiveOfPlacesTheArchiveUnderBuildFolder(t *testing.T) {
	tests := []struct {
		written string
		label   string
	}{
		{"dist/bin", "dist/bin/map.w3x"},
		{"dist/bin/", "dist/bin/map.w3x"},
		{"./out", "out/map.w3x"},
		{"out//bin", "out/bin/map.w3x"},
		{`out\bin`, "out/bin/map.w3x"},
		{"mapsout", "mapsout/map.w3x"},
		{"dist/stages", "dist/stages/map.w3x"},
		{"out/maps", "out/maps/map.w3x"},
	}
	for _, tt := range tests {
		t.Run(tt.written, func(t *testing.T) {
			root := t.TempDir()
			at, err := archivePath(projectWith(root, tt.written, manifestName))
			if err != nil {
				t.Fatalf("archiveOf: %v", diag.Format(err))
			}
			if at.displayPath != tt.label || at.fullPath != filepath.Join(root, filepath.FromSlash(tt.label)) {
				t.Errorf("archiveOf = %+v, want %s", at, tt.label)
			}
			if held := testkit.Snapshot(t, root); len(held) != 0 {
				t.Errorf("the look for the archive's place made %q", held)
			}
		})
	}
}

func TestArchiveOfRefusesAFolderOrAPlaceOutsideTheProjectNamingTheEvaluatedManifest(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "out", "map.w3x"), 0o777); err != nil {
		t.Fatal(err)
	}
	_, err := archivePath(projectWith(root, "out", localManifest))
	e := asDiagError(t, err, "a folder in the archive's place")
	if e.Msg != "The build output out/map.w3x is a directory; refusing to replace it." || e.File != localManifest ||
		e.Hint != "Set build.folder to a folder that only holds build output, such as dist/bin." {
		t.Errorf("error = %+v", e)
	}
	outside := []string{"..", "../other", "/elsewhere", `\elsewhere`, "C:/elsewhere", `c:\elsewhere`, "out/../.."}
	for _, dir := range outside {
		_, err := archivePath(projectWith(root, dir, localManifest))
		e := asDiagError(t, err, dir)
		if !strings.HasPrefix(e.Msg, "The build output ") || !strings.HasSuffix(e.Msg, " is outside the project.") ||
			e.File != localManifest || e.Hint != "Set build.folder to a folder inside the project, such as dist/bin." {
			t.Errorf("%s: %+v", dir, e)
		}
	}
}

func TestArchiveOfRefusesABuildFolderThatNamesNoFolderOrOneThatCannotHoldAnArchive(t *testing.T) {
	tests := []struct {
		written string
		words   string
	}{
		{"", `names no folder: ""`},
		{".", `names no folder: "."`},
		{"./", `names no folder: "./"`},
		{"maps", "maps/"},
		{"maps/out", "maps/"},
		{"MAPS", "maps/"},
		{"src", "src/"},
		{"./Src/built", "src/"},
		{"dist/stage", "dist/stage/"},
		{`Dist\STAGE\out`, "dist/stage/"},
		{"out:bin", `a name that Windows cannot hold: "out:bin"`},
		{"out/con", `a name that Windows cannot hold: "out/con"`},
		{"dist/bin.", `a name that Windows cannot hold: "dist/bin."`},
		{"dist/what?", `a name that Windows cannot hold: "dist/what?"`},
	}
	for _, tt := range tests {
		t.Run(tt.written, func(t *testing.T) {
			root := t.TempDir()
			_, err := archivePath(projectWith(root, tt.written, localManifest))
			e := asDiagError(t, err, tt.written)
			if !strings.Contains(e.Msg, tt.words) || e.File != localManifest || !strings.Contains(e.Hint, "dist/bin") {
				t.Errorf("error = %+v", e)
			}
		})
	}
}

func TestTheStageAndTheArchiveRefuseAMapFolderWindowsCannotHoldByTheManifest(t *testing.T) {
	for _, dir := range []string{"map?.w3x", "con.w3x", "campaign./one.w3x"} {
		p := projectWith(t.TempDir(), "dist/bin", localManifest)
		p.Map.Folder = dir
		_, ofArchive := archivePath(p)
		_, ofStage := stageOutputFile(p)
		for what, err := range map[string]error{"the archive": ofArchive, "the stage": ofStage} {
			e := asDiagError(t, err, what+" of map.folder "+dir)
			if e.Msg != `map.folder has a name that Windows cannot hold: "`+dir+`".` || e.File != localManifest ||
				!strings.Contains(e.Hint, "such as map.w3x") {
				t.Errorf("%s of map.folder %q: error = %+v", what, dir, e)
			}
		}
	}
}

func TestArchiveOfRefusesAFileOnTheWayToTheArchiveByItsName(t *testing.T) {
	tests := []struct {
		written string
		file    string
	}{
		{"dist/bin", "dist/bin"},
		{"dist/bin/deep", "dist/bin"},
		{"out", "out"},
	}
	for _, tt := range tests {
		t.Run(tt.written+" below the file "+tt.file, func(t *testing.T) {
			root := t.TempDir()
			testkit.WriteFile(t, root, tt.file, []byte("a file"))
			_, err := archivePath(projectWith(root, tt.written, manifestName))
			e := asDiagError(t, err, "a file on the way")
			if e.Msg != tt.file+" is a file, not a folder." || e.File != tt.file || e.Hint == "" || e.Cause != nil {
				t.Errorf("error = %+v", e)
			}
		})
	}
}

func TestArchiveOfRefusesALinkOnTheWayToTheArchiveByItsStep(t *testing.T) {
	tests := []struct {
		name    string
		written string
		symlink string
		target  string
	}{
		{"the first folder to the source map", "out", "out", "maps/map.w3x/war3mapImported"},
		{"the first folder to a folder beside the source map", "out", "out", "maps/other"},
		{"a folder below the first", "dist/bin", "dist/bin", "maps/other"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			testkit.WriteFile(t, root, "maps/map.w3x/war3mapImported/map.w3x", []byte("a file of the map"))
			testkit.WriteFile(t, root, "maps/other/map.w3x", []byte("a file beside the map"))
			maps := testkit.Snapshot(t, filepath.Join(root, "maps"))
			symlink := filepath.Join(root, filepath.FromSlash(tt.symlink))
			if err := os.MkdirAll(filepath.Dir(symlink), 0o777); err != nil {
				t.Fatal(err)
			}
			testkit.LinkDir(t, filepath.Join(root, filepath.FromSlash(tt.target)), symlink)
			_, err := prepareArchivePath(projectWith(root, tt.written, localManifest))
			e := asDiagError(t, err, "a link on the way to the archive")
			if !strings.HasPrefix(e.Msg, tt.symlink+" is a link: ") || e.File != tt.written+"/map.w3x" || e.Hint == "" {
				t.Errorf("error = %+v", e)
			}
			if !reflect.DeepEqual(testkit.Snapshot(t, filepath.Join(root, "maps")), maps) {
				t.Error("the look for the archive's place changed what the link leads to")
			}
		})
	}
}

func TestRemoveArchiveRemovesAFileAndNamesAFailureFromTheProjectFolder(t *testing.T) {
	root := t.TempDir()
	at := outputFile{fullPath: testkit.WriteFile(t, root, "dist/bin/map.w3x", []byte("an archive")), displayPath: "dist/bin/map.w3x"}
	for _, round := range []string{"an archive that is there", "an archive that is gone"} {
		if err := removeArchive(at); err != nil || fsx.Exists(at.fullPath) {
			t.Errorf("%s: %v", round, err)
		}
	}
	testkit.WriteFile(t, root, "dist/bin/map.w3x/kept.txt", []byte("kept"))
	e := asDiagError(t, removeArchive(at), "a folder in the archive's place")
	if !strings.HasPrefix(e.Msg, "Removing dist/bin/map.w3x failed: ") || strings.Contains(e.Msg, root) ||
		e.File != "dist/bin/map.w3x" || e.Cause == nil || e.Hint == "" {
		t.Errorf("error = %+v", e)
	}
	if !fsx.Exists(filepath.Join(at.fullPath, "kept.txt")) {
		t.Error("the folder in the archive's place lost its file")
	}
}

func TestWriteArchiveWritesBesideThePlaceAndMovesTheWholeArchiveThere(t *testing.T) {
	root := t.TempDir()
	at := outputFile{fullPath: filepath.Join(root, "dist", "bin", "campaign", "one.w3x"), displayPath: "dist/bin/campaign/one.w3x"}
	holds := func(archive string) map[string][]byte {
		return map[string][]byte{
			"dist": nil, "dist/bin": nil, "dist/bin/campaign": nil, "dist/bin/campaign/one.w3x": []byte(archive),
		}
	}
	if err := writeArchive(at, []byte("an archive")); err != nil {
		t.Fatal(diag.Format(err))
	}
	if held := testkit.Snapshot(t, root); !reflect.DeepEqual(held, holds("an archive")) {
		t.Errorf("after the first write the project holds %q", held)
	}
	testkit.WriteFile(t, root, "dist/bin/campaign/one.w3x.tmp", []byte("a cut arch"))
	if err := writeArchive(at, []byte("a second archive")); err != nil {
		t.Fatal(diag.Format(err))
	}
	if held := testkit.Snapshot(t, root); !reflect.DeepEqual(held, holds("a second archive")) {
		t.Errorf("after the second write the project holds %q", held)
	}
}

func TestWriteArchiveWritesThroughNoLinkBesideThePlace(t *testing.T) {
	root := t.TempDir()
	at := outputFile{fullPath: filepath.Join(root, "dist", "bin", "one.w3x"), displayPath: "dist/bin/one.w3x"}
	target := testkit.WriteFile(t, root, "maps/map.w3x/war3map.w3i", []byte("a file of the map"))
	if err := os.MkdirAll(filepath.Dir(at.fullPath), 0o777); err != nil {
		t.Fatal(err)
	}
	testkit.LinkFile(t, target, at.fullPath+".tmp")
	if err := writeArchive(at, []byte("an archive")); err != nil {
		t.Fatal(diag.Format(err))
	}
	if held, _ := os.ReadFile(target); string(held) != "a file of the map" {
		t.Errorf("the file the link leads to holds %q", held)
	}
	if info, err := os.Lstat(at.fullPath); err != nil || !info.Mode().IsRegular() || fsx.Exists(at.fullPath+".tmp") {
		t.Errorf("the archive is no file of its own (%v), or the link is still there", err)
	}
}

func TestWriteArchiveThatFailsLeavesThePlaceAsItWasAndNothingBesideIt(t *testing.T) {
	root := t.TempDir()
	const label = "dist/bin/one.w3x"
	at := outputFile{fullPath: testkit.WriteFile(t, root, label, []byte("the archive before")), displayPath: label}
	testkit.WriteFile(t, root, "dist/bin/one.w3x.tmp/kept.txt", []byte("kept"))
	before := testkit.Snapshot(t, root)
	e := asDiagError(t, writeArchive(at, []byte("a newer archive")), "a write beside the place that fails")
	if !strings.HasPrefix(e.Msg, "Writing dist/bin/one.w3x failed: ") || strings.Contains(e.Msg, root) ||
		e.File != "dist/bin/one.w3x" || e.Cause == nil || e.Hint == "" {
		t.Errorf("error = %+v", e)
	}
	if !reflect.DeepEqual(testkit.Snapshot(t, root), before) {
		t.Error("a write that failed changed the archive that was there, or what stood beside it")
	}
	blocked := outputFile{fullPath: filepath.Join(root, "dist", "bin"), displayPath: "dist/bin"}
	e = asDiagError(t, writeArchive(blocked, []byte("a newer archive")), "a folder in the archive's place")
	if !strings.HasPrefix(e.Msg, "Writing dist/bin failed: ") || strings.Contains(e.Msg, root) ||
		e.File != "dist/bin" || e.Cause == nil || e.Hint == "" {
		t.Errorf("error = %+v", e)
	}
	if !reflect.DeepEqual(testkit.Snapshot(t, root), before) {
		t.Error("a move that failed left what it wrote beside the place, or changed the place")
	}
}
