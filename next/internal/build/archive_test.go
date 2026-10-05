package build

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

// projectWith is a project by hand, in a folder of its own: its manifest's build.folder and the manifest that
// was evaluated.
func projectWith(root, buildFolder, manifestFile string) *manifest.Project {
	return &manifest.Project{
		Root:  root,
		File:  manifestFile,
		Map:   manifest.Map{Folder: "map.w3x", Entry: "src/main.yue"},
		Build: manifest.Build{Folder: buildFolder},
	}
}

// ---- the archive's place ----

func TestArchiveOfPlacesTheArchiveUnderBuildFolder(t *testing.T) {
	tests := []struct {
		written string // build.folder
		label   string // the archive, from the project folder
	}{
		{"dist/bin", "dist/bin/map.w3x"},
		// Every way the schema lets a folder be written names the folder.
		{"dist/bin/", "dist/bin/map.w3x"},
		{"./out", "out/map.w3x"},
		{"out//bin", "out/bin/map.w3x"},
		{`out\bin`, "out/bin/map.w3x"},
		// A folder whose name only starts as one Moonwell keeps for itself is the project's own.
		{"mapsout", "mapsout/map.w3x"},
		{"dist/stages", "dist/stages/map.w3x"},
		{"out/maps", "out/maps/map.w3x"},
	}
	for _, tt := range tests {
		t.Run(tt.written, func(t *testing.T) {
			root := t.TempDir()
			at, err := archiveOf(projectWith(root, tt.written, manifestName))
			if err != nil {
				t.Fatalf("archiveOf: %v", diag.Format(err))
			}
			if at.label != tt.label || at.file != filepath.Join(root, filepath.FromSlash(tt.label)) {
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
	// A folder in the archive's place, which a build would remove.
	_, err := archiveOf(projectWith(root, "out", localManifest))
	e := asError(t, err, "a folder in the archive's place")
	if e.Msg != "The build output out/map.w3x is a directory; refusing to replace it." || e.File != localManifest ||
		e.Hint != "Set build.folder to a folder that only holds build output, such as dist/bin." {
		t.Errorf("error = %+v", e)
	}
	outside := []string{"..", "../other", "/elsewhere", `\elsewhere`, "C:/elsewhere", `c:\elsewhere`, "out/../.."}
	for _, folder := range outside {
		_, err := archiveOf(projectWith(root, folder, localManifest))
		e := asError(t, err, folder)
		if !strings.HasPrefix(e.Msg, "The build output ") || !strings.HasSuffix(e.Msg, " is outside the project.") ||
			e.File != localManifest || e.Hint != "Set build.folder to a folder inside the project, such as dist/bin." {
			t.Errorf("%s: %+v", folder, e)
		}
	}
}

// What the schema refuses of a build.folder is refused for a manifest that was not checked against it, and so
// is what the schema lets through and no Windows can hold: each with the manifest as its file, and the value.
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
			_, err := archiveOf(projectWith(root, tt.written, localManifest))
			e := asError(t, err, tt.written)
			if !strings.Contains(e.Msg, tt.words) || e.File != localManifest || !strings.Contains(e.Hint, "dist/bin") {
				t.Errorf("error = %+v", e)
			}
		})
	}
}

func TestArchiveOfRefusesAFileOnTheWayToTheArchiveByItsName(t *testing.T) {
	tests := []struct {
		written string // build.folder
		file    string // the file on the way, from the project folder
	}{
		{"dist/bin", "dist/bin"},
		{"dist/bin/deep", "dist/bin"},
		{"out", "out"},
	}
	for _, tt := range tests {
		t.Run(tt.written+" below the file "+tt.file, func(t *testing.T) {
			root := t.TempDir()
			testkit.WriteFile(t, root, tt.file, []byte("a file"))
			_, err := archiveOf(projectWith(root, tt.written, manifestName))
			e := asError(t, err, "a file on the way")
			if e.Msg != tt.file+" is a file, not a folder." || e.File != tt.file || e.Hint == "" || e.Cause != nil {
				t.Errorf("error = %+v", e)
			}
		})
	}
}

// Every folder on the way to the archive is a real one, the first folder of build.folder too: behind a link, a
// build would remove and write a file where the link leads, which may be the source map.
func TestArchiveOfRefusesALinkOnTheWayToTheArchiveByItsStep(t *testing.T) {
	tests := []struct {
		name    string
		written string // build.folder
		link    string // the step that is a link, from the project folder
		target  string // what it is a link to, from the project folder
	}{
		{"the first folder to the source map", "out", "out", "maps/map.w3x/war3mapImported"},
		{"the first folder to a folder beside the source map", "out", "out", "maps/other"},
		{"a folder below the first", "dist/bin", "dist/bin", "maps/other"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			// A file that has the archive's name where each link leads: a build would remove it.
			testkit.WriteFile(t, root, "maps/map.w3x/war3mapImported/map.w3x", []byte("a file of the map"))
			testkit.WriteFile(t, root, "maps/other/map.w3x", []byte("a file beside the map"))
			maps := testkit.Snapshot(t, filepath.Join(root, "maps"))
			link := filepath.Join(root, filepath.FromSlash(tt.link))
			if err := os.MkdirAll(filepath.Dir(link), 0o777); err != nil {
				t.Fatal(err)
			}
			testkit.LinkDir(t, filepath.Join(root, filepath.FromSlash(tt.target)), link)
			_, err := clearedArchive(projectWith(root, tt.written, localManifest))
			e := asError(t, err, "a link on the way to the archive")
			if !strings.HasPrefix(e.Msg, tt.link+" is a link: ") || e.File != tt.written+"/map.w3x" || e.Hint == "" {
				t.Errorf("error = %+v", e)
			}
			if !reflect.DeepEqual(testkit.Snapshot(t, filepath.Join(root, "maps")), maps) {
				t.Error("the look for the archive's place changed what the link leads to")
			}
		})
	}
}

// ---- what a build removes, and what it writes ----

func TestRemoveArchiveRemovesAFileAndNamesAFailureFromTheProjectFolder(t *testing.T) {
	root := t.TempDir()
	at := place{file: testkit.WriteFile(t, root, "dist/bin/map.w3x", []byte("an archive")), label: "dist/bin/map.w3x"}
	for _, round := range []string{"an archive that is there", "an archive that is gone"} {
		if err := removeArchive(at); err != nil || fsx.Exists(at.file) {
			t.Errorf("%s: %v", round, err)
		}
	}
	// A folder that holds a file is not removed: a removal is of a file alone.
	testkit.WriteFile(t, root, "dist/bin/map.w3x/kept.txt", []byte("kept"))
	e := asError(t, removeArchive(at), "a folder in the archive's place")
	if !strings.HasPrefix(e.Msg, "Removing dist/bin/map.w3x failed: ") || strings.Contains(e.Msg, root) ||
		e.File != "dist/bin/map.w3x" || e.Cause == nil || e.Hint == "" {
		t.Errorf("error = %+v", e)
	}
	if !fsx.Exists(filepath.Join(at.file, "kept.txt")) {
		t.Error("the folder in the archive's place lost its file")
	}
}

func TestWriteArchiveMakesTheFoldersAndNamesAFailureFromTheProjectFolder(t *testing.T) {
	root := t.TempDir()
	at := place{file: filepath.Join(root, "dist", "bin", "campaign", "one.w3x"), label: "dist/bin/campaign/one.w3x"}
	if err := writeArchive(at, []byte("an archive")); err != nil {
		t.Fatal(diag.Format(err))
	}
	if held, _ := os.ReadFile(at.file); string(held) != "an archive" {
		t.Errorf("the archive holds %q", held)
	}
	// A folder in the archive's place, made after the look at the place: no system writes a file over it.
	blocked := place{file: filepath.Join(root, "dist", "bin", "campaign"), label: "dist/bin/campaign"}
	e := asError(t, writeArchive(blocked, []byte("an archive")), "a folder in the archive's place")
	if !strings.HasPrefix(e.Msg, "Writing dist/bin/campaign failed: ") || strings.Contains(e.Msg, root) ||
		e.File != "dist/bin/campaign" || e.Cause == nil || e.Hint == "" {
		t.Errorf("error = %+v", e)
	}
}
