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
				library.Path, library.OverriddenIn = &local.Path, user.FilePath
				p.Libraries[name] = library
			}
		}
	}
}

func IsProject(root string) bool {
	return fsx.Exists(filepath.Join(root, ProjectFile))
}
