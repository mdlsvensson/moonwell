package manifest

import (
	"path/filepath"
	"strings"

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
	project.useLocalLibraries(user)
	return project, nil
}

func (p *Project) useLocalLibraries(user *User) {
	for name, library := range p.Libraries {
		if library.GitHub == nil {
			continue
		}
		for _, local := range user.Libraries {
			if strings.EqualFold(local.GitHub, *library.GitHub) {
				library.Path, library.OverriddenIn = &local.Path, user.File
				p.Libraries[name] = library
			}
		}
	}
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
