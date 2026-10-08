package env

import (
	"path/filepath"
	"runtime"
	"testing"
)

var cacheVariables = []string{"MOONWELL_CACHE", "LOCALAPPDATA", "XDG_CACHE_HOME", "HOME", "USERPROFILE"}

func TestDefaultCacheDirTakesTheFirstFolderTheEnvironmentNames(t *testing.T) {
	override, local, xdg := t.TempDir(), t.TempDir(), t.TempDir()
	home, profile := t.TempDir(), t.TempDir()
	const everySystem, windows, notWindows = "", "windows", "not windows"
	cases := []struct {
		name   string
		system string
		set    map[string]string
		want   string
	}{
		{
			"MOONWELL_CACHE is the folder itself, before every other", everySystem,
			map[string]string{"MOONWELL_CACHE": override, "LOCALAPPDATA": local, "XDG_CACHE_HOME": xdg, "HOME": home},
			override,
		},
		{
			"LOCALAPPDATA before XDG_CACHE_HOME on Windows", windows,
			map[string]string{"LOCALAPPDATA": local, "XDG_CACHE_HOME": xdg, "HOME": home},
			filepath.Join(local, "moonwell"),
		},
		{
			"LOCALAPPDATA means nothing on other systems", notWindows,
			map[string]string{"LOCALAPPDATA": local, "XDG_CACHE_HOME": xdg, "HOME": home},
			filepath.Join(xdg, "moonwell"),
		},
		{
			"XDG_CACHE_HOME before HOME", everySystem,
			map[string]string{"XDG_CACHE_HOME": xdg, "HOME": home, "USERPROFILE": profile},
			filepath.Join(xdg, "moonwell"),
		},
		{
			"HOME before USERPROFILE", everySystem,
			map[string]string{"HOME": home, "USERPROFILE": profile},
			filepath.Join(home, ".cache", "moonwell"),
		},
		{
			"USERPROFILE when there is no HOME", everySystem,
			map[string]string{"USERPROFILE": profile},
			filepath.Join(profile, ".cache", "moonwell"),
		},
		{
			"the working folder when the environment names none", everySystem,
			nil,
			filepath.Join(".cache", "moonwell"),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			onWindows := runtime.GOOS == "windows"
			if (c.system == windows && !onWindows) || (c.system == notWindows && onWindows) {
				t.Skip("the case is for another system")
			}
			for _, name := range cacheVariables {
				t.Setenv(name, c.set[name])
			}
			if got := DefaultCacheDir(); got != c.want {
				t.Errorf("DefaultCacheDir = %q, want %q", got, c.want)
			}
		})
	}
}

func TestPlatformOfNamesThePlatformsWithDownloads(t *testing.T) {
	cases := []struct{ goos, goarch, want string }{
		{"windows", "amd64", "windows-x86_64"},
		{"linux", "amd64", "linux-x86_64"},
		{"darwin", "amd64", ""},
		{"darwin", "arm64", ""},
		{"linux", "arm64", ""},
		{"windows", "arm64", ""},
	}
	for _, c := range cases {
		if got := PlatformOf(c.goos, c.goarch); got != c.want {
			t.Errorf("PlatformOf(%s, %s) = %q, want %q", c.goos, c.goarch, got, c.want)
		}
	}
}

func TestCurrentPlatformIsThatOfTheSystemTheProgramWasBuiltFor(t *testing.T) {
	if got, want := CurrentPlatform(), PlatformOf(runtime.GOOS, runtime.GOARCH); got != want {
		t.Errorf("CurrentPlatform = %q, want %q", got, want)
	}
}
