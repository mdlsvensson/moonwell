package manifest

import (
	"path/filepath"

	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

func Load(e *env.Env) (*Project, error) {
	project, err := ReadProject(e)
	if err != nil {
		return nil, err
	}
	user, err := ReadUser(e)
	if err != nil {
		return nil, err
	}
	project.UserFile = user.File
	project.Launch = user.Launch
	project.Yue.Path = user.YuePath
	return project, nil
}

func IsProject(root string) bool {
	return fsx.Exists(filepath.Join(root, ProjectFile))
}

func truncateRunes(text string, limit int) string {
	count := 0
	for i := range text {
		if count == limit {
			return text[:i]
		}
		count++
	}
	return text
}
