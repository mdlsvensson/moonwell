package cli

import (
	"path/filepath"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

var gateRuns = []string{"start", "objects", "settings", "assets"}

func newGateProject(t *testing.T) string {
	t.Helper()
	root := compiling(t)
	if err := fsx.CopyTree(filepath.Join(testkit.RepoRoot(t), "gate"), root); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestEveryRunOfTheGateBuildsPlainAndMinified(t *testing.T) {
	root := newGateProject(t)
	for _, run := range gateRuns {
		entry := "src/gate_" + run + ".yue"
		mustSucceed(t, root, "build", "--entry", entry)
		mustSucceed(t, root, "build", "--minify", "--entry", entry)
	}
}

func TestTheGateProjectHasObjectsOfEveryFileSettingsAPreviewAndAnImport(t *testing.T) {
	root := newGateProject(t)
	mustSucceed(t, root, "build", "--entry", "src/gate_assets.yue")

	objects := mustSucceed(t, root, "objects:check")
	checkContains(t, objects.output,
		"war3map.w3u", "war3map.w3t", "war3map.w3h", "war3map.w3a", "war3map.w3q",
		"war3mapSkin.w3u", "war3mapSkin.w3t", "war3mapSkin.w3h", "war3mapSkin.w3a", "war3mapSkin.w3q")

	settings := mustSucceed(t, root, "settings:check")
	checkContains(t, settings.output,
		"war3map.w3i", "war3map.lua", "war3mapMisc.txt", "war3mapMinimap.blp", "war3mapMap.blp (removed)", "war3mapMap.tga")

	if _, found := readArchiveFile(t, openBuiltArchive(t, root), `war3mapImported\gate-probe.tga`); !found {
		t.Fatal("the archive does not hold the picture of the run assets")
	}
}
