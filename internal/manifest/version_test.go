package manifest

import (
	"runtime"
	"strings"
	"testing"
)

func resolvedDeps(version string) string {
	return `{"schemaVersion":1,"resolvedDependencies":{
		"package://pkg.pkl-lang.org/github.com/mdlsvensson/moonwell/moonwell@0":{"type":"local",
		"uri":"projectpackage://pkg.pkl-lang.org/github.com/mdlsvensson/moonwell/moonwell@` + version + `","path":"../schema"}}}`
}

func TestReadPackageVersionFindsTheResolvedMoonwellVersion(t *testing.T) {
	const remote = `{"resolvedDependencies":{
		"package://example.org/other@1":{"type":"remote","uri":"projectpackage://example.org/other@1.2.3"},
		"package://pkg.pkl-lang.org/github.com/mdlsvensson/moonwell/moonwell@0":{"type":"remote",
		"uri":"projectpackage://pkg.pkl-lang.org/github.com/mdlsvensson/moonwell/moonwell@0.9.1-rc.1"}}}`
	found := []struct{ name, deps, want string }{
		{"a local package", resolvedDeps("0.1.3"), "0.1.3"},
		{"a remote package beside another", remote, "0.9.1-rc.1"},
		{"behind a byte order mark", "\xEF\xBB\xBF" + resolvedDeps("0.9.0"), "0.9.0"},
		{"after a dependency of its name that is no mapping", `{"resolvedDependencies":{
			"package://x/moonwell@0":"1.0.0","package://y/moonwell@1":{"uri":"p://y/moonwell@1.2.3"}}}`, "1.2.3"},
	}
	for _, tt := range found {
		if got, err := readPackageVersion([]byte(tt.deps)); err != nil || got != tt.want {
			t.Errorf("%s: readPackageVersion = %q, %v, want %q", tt.name, got, err, tt.want)
		}
	}
	refused := []struct{ name, deps, word string }{
		{"no dependency", `{"resolvedDependencies":{}}`, "not a resolved"},
		{"no dependencies", `{}`, "not a resolved"},
		{"a list", `[]`, "not a resolved"},
		{"null", `null`, "not a resolved"},
		{"dependencies that are no mapping", `{"resolvedDependencies":[]}`, "not a resolved"},
		{"a dependency that is no mapping", `{"resolvedDependencies":{"package://x/moonwell@0":"1.0.0"}}`, "not a resolved"},
		{"an address without a version", `{"resolvedDependencies":{"package://x/moonwell@0":{"uri":3}}}`, "not a resolved"},
		{"another package", `{"resolvedDependencies":{"package://x/other@0":{"uri":"p://x/other@0.9.1"}}}`, "not a resolved"},
		{"not JSON", `{ not json`, "not valid JSON"},
		{"an empty file", ``, "not valid JSON"},
	}
	for _, tt := range refused {
		version, err := readPackageVersion([]byte(tt.deps))
		failure := asError(t, err, tt.name)
		if version != "" || failure.File != "PklProject.deps.json" || !strings.Contains(failure.Msg, tt.word) ||
			!strings.Contains(failure.Hint, "pkl project resolve") {
			t.Errorf("%s: %q, %+v", tt.name, version, failure)
		}
	}
}

func TestCheckPackageVersionComparesTheMajorAndMinorNumbers(t *testing.T) {
	for _, same := range [][2]string{{"0.1.9", "0.1.0"}, {"0.9.1", "0.9.1"}, {"1.2.0-rc.1", "1.2.7"}, {"0.9", "0.9.1"}} {
		if err := checkPackageVersion(same[0], same[1]); err != nil {
			t.Errorf("checkPackageVersion(%q, %q) = %v", same[0], same[1], err)
		}
	}
	install := "curl -fsSL https://github.com/mdlsvensson/moonwell/releases/download/moonwell@0.8.2/install.sh | sh"
	if runtime.GOOS == "windows" {
		install = "irm https://github.com/mdlsvensson/moonwell/releases/download/moonwell@0.8.2/install.ps1 | iex"
	}
	if got := installCommand("0.8.2"); got != install {
		t.Errorf("installLine = %q, want %q", got, install)
	}
	tests := []struct {
		name, pkg, program string
		installs           bool
		move               string
	}{
		{"before the first install script", "0.7.0", "0.8.0", false, "moonwell@0.8.x"},
		{"a package with an install script", "0.8.2", "0.9.0", true, "moonwell@0.9.x"},
		{"a later major version", "1.0.0", "0.9.1", true, "moonwell@0.9.x"},
		{"a version that is no version", "latest", "0.9.1", false, "moonwell@0.9.x"},
		{"a minor version that starts as the program's does", "0.1.0", "0.10.0", false, "moonwell@0.10.x"},
		{"the minor version before the program's", "0.9.1", "0.10.0", true, "moonwell@0.10.x"},
		{"no version", "", "0.9.1", false, "moonwell@0.9.x"},
		{"a version with a part that is no number", "0.x.1", "1.0.0", false, "moonwell@1.0.x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			failure := asError(t, checkPackageVersion(tt.pkg, tt.program), tt.name)
			if failure.File != "PklProject" || !strings.Contains(failure.Msg, "moonwell@"+tt.pkg) || !strings.Contains(failure.Msg, tt.program) {
				t.Errorf("error = %+v", failure)
			}
			if !strings.Contains(failure.Hint, tt.move) || !strings.Contains(failure.Hint, "pkl project resolve") {
				t.Errorf("the hint %q does not say how to move to %s", failure.Hint, tt.move)
			}
			if got := strings.Contains(failure.Hint, installCommand(tt.pkg)); got != tt.installs {
				t.Errorf("the hint %q names the install line: %v, want %v", failure.Hint, got, tt.installs)
			}
		})
	}
}
