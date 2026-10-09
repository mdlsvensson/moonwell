package testkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type Export struct {
	t    testing.TB
	root string
}

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

func (e Export) Path(segments ...string) string {
	e.t.Helper()
	path := e.root
	for _, name := range segments {
		if path == "" {
			break
		}
		path = e.findEntry(path, name)
	}
	return path
}

func (e Export) Entries(segments ...string) []string {
	e.t.Helper()
	dir := e.Path(segments...)
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
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

func (e Export) findEntry(dir, name string) string {
	e.t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		e.t.Fatalf("the game's files: %v", err)
		return ""
	}
	var matches []string
	for _, entry := range entries {
		if strings.ToLower(entry.Name()) == strings.ToLower(name) {
			matches = append(matches, entry.Name())
		}
	}
	if len(matches) != 1 {
		e.t.Fatalf("%s has %d entries named %s, whatever their letter case; want 1", dir, len(matches), name)
		return ""
	}
	return filepath.Join(dir, matches[0])
}
