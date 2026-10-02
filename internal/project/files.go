package project

import "strings"

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

// LocalPkl renders moonwell.local.pkl as init and setup create it: this machine's settings, amending the shared
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
