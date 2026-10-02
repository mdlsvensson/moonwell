package main

import (
	"errors"
	"os"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/models"
)

// GenerateGamePaths writes the in-game path list for a CASC file-name export to target and returns its path count.
// It fails, without writing, when the export has no model or texture path.
func GenerateGamePaths(list, version, target string) (int, error) {
	rendered := models.RenderGamePaths(list, version)
	// One line for the version, one per path, each ended by a line break.
	count := strings.Count(rendered, "\n") - 1
	if count == 0 {
		return 0, errors.New("no model or texture paths were recognized in the listfile; check the export's format " +
			"(one file name per line, UTF-8). The path list was not changed.")
	}
	if err := os.WriteFile(target, []byte(rendered), 0o666); err != nil {
		return 0, err
	}
	return count, nil
}
