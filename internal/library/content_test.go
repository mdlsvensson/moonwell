package library

import (
	"testing"
)

func TestAFileIsHeldAgainstAFolderInAnotherSpellingAndNotAgainstOneInItsOwn(t *testing.T) {
	for _, local := range []bool{false, true} {
		kept := libraryContent{modules: archiveFilesOf("a.lua", "1", "Pack.lua", "2", "pack.lua/inner.lua", "3"), isLocal: local}
		hint := reportIt
		if local {
			hint = renameOne
		}
		e := asDiagError(t, kept.checkUsable("ex", manifestFile), "a module and a folder")
		if e.Msg != "Library ex: Pack.lua and pack.lua in its module folder differ only in letter case." || e.File != manifestFile || e.Hint != hint {
			t.Errorf("local %v: %+v", local, e)
		}
	}
	for what, kept := range map[string]libraryContent{
		"a file where a folder is":                  {modules: archiveFilesOf("util", "1", "util/b.lua", "2")},
		"a module and a folder of the map's files":  {modules: archiveFilesOf("Icons", "1"), assets: archiveFilesOf("icons/x.blp", "2"), shipsAssets: true},
		"a file whose name starts as a folder does": {modules: archiveFilesOf("Util.lua", "1", "util/b.lua", "2", "utility/c.lua", "3")},
	} {
		if err := kept.checkUsable("ex", manifestFile); err != nil {
			t.Errorf("%s: %v", what, err)
		}
	}
}

func TestAFileThatCannotBeUsedIsTheAuthorsToReportOrTheUsersOwnToRename(t *testing.T) {
	modules, assets := archiveFilesOf("a.lua", "1"), archiveFilesOf("x.blp", "1")
	cases := []struct {
		name          string
		kept          libraryContent
		local, author string
	}{
		{"a module's name", libraryContent{modules: archiveFilesOf("a.lua", "1", "aux.lua", "2")}, renameIt, reportIt},
		{"two modules", libraryContent{modules: archiveFilesOf("a.lua", "1", "A.lua", "2")}, renameOne, reportIt},
		{"modules in two folders", libraryContent{modules: archiveFilesOf("u/a.lua", "1", "U/b.lua", "2")}, renameAFolder, reportIt},
		{"an asset's name", libraryContent{modules: modules, assets: archiveFilesOf("nul", "1"), shipsAssets: true}, renameAnAsset, reportIt},
		{"two assets", libraryContent{modules: modules, assets: append(assets, archiveFilesOf("X.blp", "2")...), shipsAssets: true}, renameAssets, reportIt},
		{"assets in two folders", libraryContent{modules: modules, assets: archiveFilesOf("u/a", "1", "U/b", "2"), shipsAssets: true}, renameAnAssets, reportIt},
	}
	for _, c := range cases {
		for _, local := range []bool{false, true} {
			c.kept.isLocal = local
			want := c.author
			if local {
				want = c.local
			}
			if e := asDiagError(t, c.kept.checkUsable("ex", manifestFile), c.name); e.Hint != want || e.File != manifestFile {
				t.Errorf("%s, local %v: %+v", c.name, local, e)
			}
		}
	}
}
