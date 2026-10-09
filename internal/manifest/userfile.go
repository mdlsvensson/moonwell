package manifest

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/env"
)

const UserFile = "config.toml"

var defaultLaunchArgs = []string{"-launch", "-windowmode", "windowed"}

var userDefaults = map[string]any{
	"launch.args": defaultLaunchArgs,
}

type User struct {
	FilePath  string
	Launch    Launch
	YuePath   *string
	Libraries []LocalLibrary
}

type LocalLibrary struct {
	GitHub string `json:"github"`
	Path   string `json:"path"`
}

type userFile struct {
	Launch    Launch         `json:"launch"`
	Yue       userYue        `json:"yue"`
	Libraries []LocalLibrary `json:"libraries"`
}

type userYue struct {
	Path *string `json:"path"`
}

func UserFilePath(e *env.Env) string {
	return filepath.Join(e.ConfigDir, UserFile)
}

func ReadUser(e *env.Env) (*User, error) {
	fullPath := UserFilePath(e)
	if e.ConfigDir == "" {
		return &User{FilePath: fullPath, Launch: Launch{Args: slices.Clone(defaultLaunchArgs)}}, nil
	}
	settings, _, err := readSettingsFile(fullPath, fullPath, userDefaults)
	if err != nil {
		return nil, err
	}
	var file userFile
	if err := decodeSettings(settings, fullPath, &file); err != nil {
		return nil, err
	}
	if err := file.checkRules(fullPath); err != nil {
		return nil, err
	}
	return &User{FilePath: fullPath, Launch: file.Launch, YuePath: file.Yue.Path, Libraries: file.Libraries}, nil
}

func (u *userFile) checkRules(fullPath string) error {
	c := &ruleChecker{file: fullPath}
	c.check(!isSetAndEmpty(u.Launch.GameExecutable), "launch.gameExecutable", "must not be empty")
	c.check(!isSetAndEmpty(u.Yue.Path), "yue.path", "must not be empty")
	seen := map[string]bool{}
	for index, library := range u.Libraries {
		setting := fmt.Sprintf("libraries[%d]", index)
		c.check(gitHubRepoName.MatchString(library.GitHub), setting+".github", `must be a repository as "owner/repo"`)
		c.check(!seen[strings.ToLower(library.GitHub)], setting+".github",
			"is "+library.GitHub+", the repository of an entry above it")
		seen[strings.ToLower(library.GitHub)] = true
		c.check(filepath.IsAbs(library.Path), setting+".path", "must be an absolute path to the library's folder")
	}
	return c.err
}
