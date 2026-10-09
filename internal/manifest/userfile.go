package manifest

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

const UserFile = "config.toml"

const DefaultGameExecutable = `C:\Program Files (x86)\Warcraft III\_retail_\x86_64\Warcraft III.exe`

var defaultLaunchArgs = []string{"-launch", "-windowmode", "windowed"}

var userDefaults = map[string]any{
	"launch.args": defaultLaunchArgs,
}

type User struct {
	File      string
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
	return &User{File: fullPath, Launch: file.Launch, YuePath: file.Yue.Path, Libraries: file.Libraries}, nil
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

func UserFileText() string {
	return "[launch]\ngameExecutable = '" + DefaultGameExecutable + "'\n"
}

func EnsureUserFile(e *env.Env) (created bool, err error) {
	fullPath := UserFilePath(e)
	if err := os.MkdirAll(e.ConfigDir, 0o777); err != nil {
		return false, errUserFileNotWritten(fullPath, err)
	}
	file, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	switch {
	case errors.Is(err, fs.ErrExist), err != nil && pathExists(fullPath):
		return false, nil
	case err != nil:
		return false, errUserFileNotWritten(fullPath, err)
	}
	_, err = file.WriteString(UserFileText())
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(fullPath)
		return false, errUserFileNotWritten(fullPath, err)
	}
	return true, nil
}

func pathExists(path string) bool {
	info, err := fsx.Lstat(path)
	return err == nil && info != nil
}

func errUserFileNotWritten(fullPath string, cause error) error {
	return &diag.Error{
		Msg:  "Creating " + fullPath + " failed: " + fsx.Reason(cause),
		File: fullPath,
		Hint: "Make sure that the folder is one you may write to and that its disk has room, then run moonwell setup " +
			"again. The variable MOONWELL_HOME names another folder for this file.",
		Cause: cause,
	}
}
