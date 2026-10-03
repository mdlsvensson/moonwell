package project

import (
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/ordered"
)

var (
	moonwellPackage = regexp.MustCompile(`/moonwell@\d+$`)
	resolvedVersion = regexp.MustCompile(`@(\d+\.\d+\.\d+[^/]*)$`)
)

// ReadPackageVersion returns the resolved version of the `moonwell` package in a PklProject.deps.json document.
func ReadPackageVersion(depsJSON string) (string, error) {
	deps, err := ordered.Decode([]byte(depsJSON))
	if err != nil {
		return "", &diag.Error{
			Msg:   "PklProject.deps.json is not valid JSON.",
			File:  "PklProject.deps.json",
			Cause: err,
			Hint:  "Fix or regenerate it with `pkl project resolve`.",
		}
	}
	if document, ok := deps.(*ordered.Object); ok {
		resolved, _ := document.Get("resolvedDependencies")
		if dependencies, ok := resolved.(*ordered.Object); ok {
			for key, entry := range dependencies.All() {
				dependency, isObject := entry.(*ordered.Object)
				if !moonwellPackage.MatchString(key) || !isObject {
					continue
				}
				entryURI, _ := dependency.Get("uri")
				uri, _ := entryURI.(string)
				if match := resolvedVersion.FindStringSubmatch(uri); match != nil {
					return match[1], nil
				}
			}
		}
	}
	return "", &diag.Error{
		Msg:  "The moonwell Pkl package is not a resolved dependency.",
		File: "PklProject.deps.json",
		Hint: "Declare it in PklProject and run `pkl project resolve`.",
	}
}

const releases = "https://github.com/mdlsvensson/moonwell/releases/download/moonwell@"

// InstallLine is the command that installs a Moonwell version on this machine.
func InstallLine(version string) string {
	if runtime.GOOS == "windows" {
		return "irm " + releases + version + "/install.ps1 | iex"
	}
	return "curl -fsSL " + releases + version + "/install.sh | sh"
}

// hasExecutable reports whether a Moonwell version was released as an executable: 0.8.0 and later. Earlier versions
// ran on Deno.
func hasExecutable(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return false
	}
	major, errMajor := strconv.Atoi(parts[0])
	minor, errMinor := strconv.Atoi(parts[1])
	return errMajor == nil && errMinor == nil && (major > 0 || minor >= 8)
}

// CheckPackageVersion fails unless the project's Pkl package and this executable have the same major and minor
// version.
func CheckPackageVersion(packageVersion, cliVersion string) error {
	pkg, cli := strings.Split(packageVersion, "."), strings.Split(cliVersion, ".")
	if len(pkg) >= 2 && len(cli) >= 2 && pkg[0] == cli[0] && pkg[1] == cli[1] {
		return nil
	}
	minor := ""
	if len(cli) > 1 {
		minor = cli[1]
	}
	// Two ways out: the executable for the project's version, or the project moved to this executable's version. A
	// version before 0.8.0 ran on Deno and has no executable.
	move := "se moonwell@" + cli[0] + "." + minor + ".x in PklProject and run `pkl project resolve`."
	hint := "U" + move
	if hasExecutable(packageVersion) {
		hint = "Install Moonwell " + packageVersion + " (" + InstallLine(packageVersion) + "), or u" + move
	}
	return &diag.Error{
		Msg:  "Pkl package moonwell@" + packageVersion + " does not match Moonwell CLI " + cliVersion + ".",
		File: "PklProject",
		Hint: hint,
	}
}
