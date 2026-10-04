package manifest

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// PackageBaseURI is where the moonwell Pkl package is published, without its version.
const PackageBaseURI = "package://pkg.pkl-lang.org/github.com/mdlsvensson/moonwell/moonwell"

// PklProject renders the PklProject of a map project. It depends on the published moonwell package of version, or,
// when local is not empty, on the package in the folder local: a checkout's schema/.
func PklProject(version, local string) string {
	line := `  ["moonwell"] { uri = "` + PackageBaseURI + "@" + version + `" }`
	if local != "" {
		line = `  ["moonwell"] = import("` + local + `/PklProject")`
	}
	return "amends \"pkl:Project\"\n\ndependencies {\n" + line + "\n}\n"
}

// DefaultGameExecutable is the Warcraft III executable of a Battle.net install on Windows.
const DefaultGameExecutable = `C:\Program Files (x86)\Warcraft III\_retail_\x86_64\Warcraft III.exe`

// LocalPkl renders moonwell.local.pkl as a new project gets it: this machine's settings, amending the shared
// manifest.
func LocalPkl() string {
	executable := strings.ReplaceAll(DefaultGameExecutable, `\`, `\\`)
	return strings.Join([]string{
		"// Settings for this machine only. Git-ignored, so each checkout has its own; `moonwell setup` recreates it.",
		"// It amends moonwell.pkl, so anything set here overrides the shared manifest.",
		"",
		`amends "moonwell.pkl"`,
		"",
		"launch {",
		`  gameExecutable = "` + executable + `"  // your Warcraft III.exe`,
		"}",
		"",
	}, "\n")
}

// EnsureLocalManifest creates moonwell.local.pkl in root unless it exists, and reports whether it did. It never
// overwrites.
func EnsureLocalManifest(root string) (created bool, err error) {
	file, err := os.OpenFile(filepath.Join(root, localManifest), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if errors.Is(err, fs.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if _, err := file.WriteString(LocalPkl()); err != nil {
		file.Close()
		return false, err
	}
	return true, file.Close()
}
