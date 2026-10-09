package env

import (
	"os"
	"path/filepath"
)

const configDirVariable = "MOONWELL_HOME"

func DefaultConfigDir() string {
	if override := os.Getenv(configDirVariable); override != "" {
		return override
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".moonwell")
}
