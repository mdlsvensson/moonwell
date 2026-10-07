package testkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Export is the game's files as one environment variable names them: a folder they were exported into, or the
// list of their names. A test reads them and never writes them.
type Export struct {
	t    testing.TB
	root string // "" when the test was skipped or failed for want of the export
}

// NeedExport returns the game's files that the environment variable names: MOONWELL_GAME_SCRIPTS and
// MOONWELL_GAME_DATA each name a folder, MOONWELL_GAME_LISTFILE a file. They are on the maintainer's machine
// and nowhere else, so without the variable it skips the test, or fails it when MOONWELL_REQUIRE_EXPORTS=1. A
// variable that names nothing fails the test.
func NeedExport(t testing.TB, variable string) Export {
	t.Helper()
	root := os.Getenv(variable)
	if root == "" && os.Getenv("MOONWELL_REQUIRE_EXPORTS") == "1" {
		t.Fatalf("%s is not set, and MOONWELL_REQUIRE_EXPORTS=1 requires the game's files", variable)
		return Export{t: t}
	}
	if root == "" {
		t.Skipf("%s is not set: the game's files are not read", variable)
		return Export{t: t}
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("%s names what is not there: %v", variable, err)
		return Export{t: t}
	}
	return Export{t, root}
}

// Path is the export itself, or what the names lead to below it: each name is an entry of the folder before it,
// found without regard to letter case, and the path spells it as the folder does. It fails the test, and
// returns "", when a folder has no entry of a name or more than one.
func (e Export) Path(below ...string) string {
	e.t.Helper()
	path := e.root
	for _, name := range below {
		if path == "" {
			break
		}
		path = e.entry(path, name)
	}
	return path
}

// Entries is the names of the entries of the folder that the names lead to, as the folder spells them and in
// the order of their bytes.
func (e Export) Entries(below ...string) []string {
	e.t.Helper()
	folder := e.Path(below...)
	if folder == "" {
		return nil
	}
	entries, err := os.ReadDir(folder)
	if err != nil {
		e.t.Fatalf("the game's files: %v", err)
		return nil
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

// entry is the path of the folder's one entry with the name, whatever the letter case of either. It fails the
// test, and returns "", when the folder has none or several.
func (e Export) entry(folder, name string) string {
	e.t.Helper()
	entries, err := os.ReadDir(folder)
	if err != nil {
		e.t.Fatalf("the game's files: %v", err)
		return ""
	}
	var named []string
	for _, entry := range entries {
		if strings.ToLower(entry.Name()) == strings.ToLower(name) {
			named = append(named, entry.Name())
		}
	}
	if len(named) != 1 {
		e.t.Fatalf("%s has %d entries named %s, whatever their letter case; want 1", folder, len(named), name)
		return ""
	}
	return filepath.Join(folder, named[0])
}
