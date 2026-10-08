package moonwell

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

type release map[string][]byte

func (r release) serve(t *testing.T) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		content, ok := r[strings.TrimPrefix(request.URL.Path, "/")]
		if !ok {
			http.NotFound(w, request)
			return
		}
		w.Write(content)
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

type installer struct {
	asset, target string
	command       func(home, base string) *exec.Cmd
}

func thisInstaller(t *testing.T) installer {
	t.Helper()
	if runtime.GOARCH != "amd64" {
		t.Skip("the install scripts install the x86-64 builds")
	}
	withEnv := func(cmd *exec.Cmd, env ...string) *exec.Cmd {
		cmd.Env = append(os.Environ(), env...)
		return cmd
	}
	switch runtime.GOOS {
	case "linux":
		return installer{"moonwell-linux-amd64", ".local/bin/moonwell", func(home, base string) *exec.Cmd {
			return withEnv(exec.Command("sh", "install.sh"), "HOME="+home, "MOONWELL_INSTALL_BASE="+base)
		}}
	case "windows":
		return installer{"moonwell-windows-amd64.exe", "bin/moonwell.exe", func(home, base string) *exec.Cmd {
			return withEnv(
				exec.Command("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", "install.ps1"),
				"MOONWELL_CACHE="+home, "MOONWELL_INSTALL_BASE="+base, "MOONWELL_INSTALL_NO_PATH=1",
			)
		}}
	}
	t.Skip("there is no install script for " + runtime.GOOS)
	return installer{}
}

func (i installer) checksums(content []byte) []byte {
	return []byte(sha256Hex([]byte("another system's file")) + "  moonwell-other-amd64\n" + sha256Hex(content) + "  " + i.asset + "\n")
}

func TestTheInstallScriptInstallsAndUpgrades(t *testing.T) {
	install := thisInstaller(t)
	home := t.TempDir()
	target := filepath.Join(home, filepath.FromSlash(install.target))
	for _, content := range []string{"the first executable", "the executable of a later run"} {
		files := release{install.asset: []byte(content), "checksums.txt": install.checksums([]byte(content))}
		output, err := install.command(home, files.serve(t)).CombinedOutput()
		if err != nil {
			t.Fatalf("the script failed: %v\n%s", err, output)
		}
		if !strings.Contains(string(output), "Installed Moonwell "+Version+" to ") {
			t.Errorf("output:\n%s", output)
		}
		installed, err := os.ReadFile(target)
		if err != nil || string(installed) != content {
			t.Fatalf("installed %q, error %v", installed, err)
		}
		if info, _ := os.Stat(target); runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
			t.Error("the installed file is not executable")
		}
	}
}

func TestTheInstallScriptRefusesABadDownloadAndKeepsTheInstalledFile(t *testing.T) {
	install := thisInstaller(t)
	content := []byte("the executable")
	for name, c := range map[string]struct {
		files release
		want  string
	}{
		"a wrong checksum":    {release{install.asset: []byte("something else"), "checksums.txt": install.checksums(content)}, "does not match its checksum"},
		"an unlisted file":    {release{install.asset: content, "checksums.txt": []byte(sha256Hex(content) + "  another-file\n")}, "does not list " + install.asset},
		"a missing download":  {release{"checksums.txt": install.checksums(content)}, ""},
		"no checksums at all": {release{install.asset: content}, ""},
	} {
		home := t.TempDir()
		target := filepath.Join(home, filepath.FromSlash(install.target))
		if err := os.MkdirAll(filepath.Dir(target), 0o777); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte("installed before"), 0o777); err != nil {
			t.Fatal(err)
		}
		output, err := install.command(home, c.files.serve(t)).CombinedOutput()
		if err == nil {
			t.Errorf("%s: the script succeeded:\n%s", name, output)
		}
		if !strings.Contains(string(output), c.want) {
			t.Errorf("%s: output:\n%s", name, output)
		}
		if kept, _ := os.ReadFile(target); string(kept) != "installed before" {
			t.Errorf("%s: the installed file is now %q", name, kept)
		}
	}
}

func TestTheInstallScriptsInstallThisVersion(t *testing.T) {
	for file, pattern := range map[string]string{
		"install.sh":  `(?m)^version="([^"]+)"$`,
		"install.ps1": `(?m)^\s*\$version = '([^']+)'$`,
	} {
		script, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		match := regexp.MustCompile(pattern).FindSubmatch(script)
		if match == nil {
			t.Errorf("%s has no version line", file)
		} else if string(match[1]) != Version {
			t.Errorf("%s installs version %s, version.go has %s", file, match[1], Version)
		}
		if strings.Contains(string(script), "\r\n") {
			t.Errorf("%s has CRLF line endings: sh refuses them", file)
		}
	}
}
