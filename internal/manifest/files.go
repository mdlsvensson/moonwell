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

// packageBaseURI is where the moonwell Pkl package is published, without its version.
const packageBaseURI = "package://pkg.pkl-lang.org/github.com/mdlsvensson/moonwell/moonwell"

// PklProject renders the PklProject of a map project. It depends on the published moonwell package of version, or,
// when local is not empty, on the package in the folder local: a checkout's schema/.
func PklProject(version, local string) string {
	line := `  ["moonwell"] { uri = "` + packageBaseURI + "@" + version + `" }`
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
// overwrites: what is under the name, a file or a folder, is the user's.
//
// A file that could not be written whole is removed, so that the next call does not take half a file for the
// user's.
func EnsureLocalManifest(root string) (created bool, err error) {
	path := filepath.Join(root, localManifest)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	switch {
	// The look at the name is there for a folder under it: one system says of it that it exists, as of a file,
	// and another that it is a folder, which is no answer of "exists".
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
		// A removal that fails is passed over: the failure to write is what the user has to know.
		_ = os.Remove(path)
		return false, errLocalManifestNotWritten(err)
	}
	return true, nil
}

// isTaken reports whether something is under the name at path: a file, a folder, or a link, wherever it leads.
// The name itself is looked at, and a look that fails finds nothing.
func isTaken(path string) bool {
	info, err := fsx.Lstat(path)
	return err == nil && info != nil
}

// ---- errors ----

func errLocalManifestNotWritten(cause error) error {
	return &diag.Error{
		Msg:  "Creating " + localManifest + " failed: " + fsx.Reason(cause),
		File: localManifest,
		Hint: "Make sure that the project folder is one you may write to and that its disk has room, then run " +
			"moonwell setup again.",
		Cause: cause,
	}
}
