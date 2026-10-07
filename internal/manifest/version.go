package manifest

import (
	"encoding/json"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

var (
	moonwellPackage = regexp.MustCompile(`/moonwell@\d+$`)         // the key of the package among the dependencies
	resolvedVersion = regexp.MustCompile(`@(\d+\.\d+\.\d+[^/]*)$`) // the version at the end of a resolved address
)

// readPackageVersion returns the version the moonwell Pkl package is resolved to in a PklProject.deps.json
// document.
func readPackageVersion(deps []byte) (string, error) {
	text := []byte(fsx.DecodeText(deps))
	var whole json.RawMessage
	if err := json.Unmarshal(text, &whole); err != nil {
		return "", errDepsNotJSON(err)
	}
	var document struct {
		Resolved Ordered[json.RawMessage] `json:"resolvedDependencies"`
	}
	// A document of another shape fails here, and has no dependencies to find the package among.
	_ = json.Unmarshal(text, &document)
	for key, entry := range document.Resolved.All() {
		var dependency struct {
			URI string `json:"uri"`
		}
		if !moonwellPackage.MatchString(key) || json.Unmarshal(entry, &dependency) != nil {
			continue
		}
		if version := resolvedVersion.FindStringSubmatch(dependency.URI); version != nil {
			return version[1], nil
		}
	}
	return "", errNotResolved()
}

// checkPackageVersion fails unless the project's Pkl package and this program have the same major and minor
// version.
func checkPackageVersion(packageVersion, programVersion string) error {
	pkg, program := strings.Split(packageVersion, "."), strings.Split(programVersion, ".")
	if len(pkg) >= 2 && len(program) >= 2 && pkg[0] == program[0] && pkg[1] == program[1] {
		return nil
	}
	return errVersionMismatch(packageVersion, programVersion)
}

const releases = "https://github.com/mdlsvensson/moonwell/releases/download/moonwell@"

// installLine is the command that installs a Moonwell version on this machine.
func installLine(version string) string {
	if runtime.GOOS == "windows" {
		return "irm " + releases + version + "/install.ps1 | iex"
	}
	return "curl -fsSL " + releases + version + "/install.sh | sh"
}

// hasInstallScript reports whether a Moonwell version can be installed with installLine: 0.8.0 is the first that
// has an install script.
func hasInstallScript(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return false
	}
	major, errMajor := strconv.Atoi(parts[0])
	minor, errMinor := strconv.Atoi(parts[1])
	return errMajor == nil && errMinor == nil && (major > 0 || minor >= 8)
}

// ---- errors ----

func errDepsNotJSON(cause error) error {
	return &diag.Error{
		Msg:   "PklProject.deps.json is not valid JSON.",
		File:  depsFile,
		Hint:  "Fix or regenerate it with `pkl project resolve`.",
		Cause: cause,
	}
}

func errNotResolved() error {
	return &diag.Error{
		Msg:  "The moonwell Pkl package is not a resolved dependency.",
		File: depsFile,
		Hint: "Declare it in PklProject and run `pkl project resolve`.",
	}
}

// errVersionMismatch names the two ways out: the program of the project's version, or the project moved to this
// program's version. A package without an install script leaves only the second.
func errVersionMismatch(packageVersion, programVersion string) error {
	major, minor, _ := strings.Cut(programVersion, ".")
	minor, _, _ = strings.Cut(minor, ".")
	move := "se moonwell@" + major + "." + minor + ".x in PklProject and run `pkl project resolve`."
	hint := "U" + move
	if hasInstallScript(packageVersion) {
		hint = "Install Moonwell " + packageVersion + " (" + installLine(packageVersion) + "), or u" + move
	}
	return &diag.Error{
		Msg:  "Pkl package moonwell@" + packageVersion + " does not match Moonwell CLI " + programVersion + ".",
		File: pklProjectFile,
		Hint: hint,
	}
}
