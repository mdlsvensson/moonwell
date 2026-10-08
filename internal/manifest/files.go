package manifest

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

const packageBaseURI = "package://pkg.pkl-lang.org/github.com/mdlsvensson/moonwell/moonwell"

func PklProject(version, local string) string {
	line := `  ["moonwell"] { uri = "` + packageBaseURI + "@" + version + `" }`
	if local != "" {
		line = `  ["moonwell"] = import("` + local + `/PklProject")`
	}
	return "amends \"pkl:Project\"\n\ndependencies {\n" + line + "\n}\n"
}

const DefaultGameExecutable = `C:\Program Files (x86)\Warcraft III\_retail_\x86_64\Warcraft III.exe`

func LocalPkl() string {
	executable := strings.ReplaceAll(DefaultGameExecutable, `\`, `\\`)
	return strings.Join([]string{
		"// Settings for this machine only. Git-ignored, so each checkout has its own; `moonwell setup` recreates it.",
		"// It amends moonwell.pkl, so anything set here overrides the shared manifest.",
		"",
		`amends "` + SharedFile + `"`,
		"",
		"launch {",
		`  gameExecutable = "` + executable + `"  // your Warcraft III.exe`,
		"}",
		"",
	}, "\n")
}

func EnsureLocalManifest(root string) (created bool, err error) {
	path := filepath.Join(root, LocalFile)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	switch {
	case errors.Is(err, fs.ErrExist), err != nil && isTaken(path):
		return false, nil
	case err != nil:
		return false, errLocalManifestNotWritten(err)
	}
	_, err = file.WriteString(LocalPkl())
	if closed := file.Close(); err == nil {
		err = closed
	}
	if err != nil {
		_ = os.Remove(path)
		return false, errLocalManifestNotWritten(err)
	}
	return true, nil
}

func isTaken(path string) bool {
	info, err := fsx.Lstat(path)
	return err == nil && info != nil
}

func errLocalManifestNotWritten(cause error) error {
	return &diag.Error{
		Msg:  "Creating " + LocalFile + " failed: " + fsx.Reason(cause),
		File: LocalFile,
		Hint: "Make sure that the project folder is one you may write to and that its disk has room, then run " +
			"moonwell setup again.",
		Cause: cause,
	}
}
