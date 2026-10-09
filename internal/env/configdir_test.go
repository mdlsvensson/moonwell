package env

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigDirIsTheFolderMoonwellHomeNames(t *testing.T) {
	named := t.TempDir()
	t.Setenv("MOONWELL_HOME", named)
	if got := DefaultConfigDir(); got != named {
		t.Errorf("DefaultConfigDir = %q, want %q", got, named)
	}
}

func TestDefaultConfigDirIsDotMoonwellInTheUsersHomeWithoutTheVariable(t *testing.T) {
	t.Setenv("MOONWELL_HOME", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("this system names no home folder")
	}
	if got, want := DefaultConfigDir(), filepath.Join(home, ".moonwell"); got != want {
		t.Errorf("DefaultConfigDir = %q, want %q", got, want)
	}
}
