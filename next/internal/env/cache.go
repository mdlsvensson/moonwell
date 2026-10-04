package env

import (
	"os"
	"path/filepath"
	"runtime"
)

// DefaultCacheDir is the per-user folder downloads are kept in: MOONWELL_CACHE when it is set, else a folder
// "moonwell" in %LOCALAPPDATA% on Windows, else in $XDG_CACHE_HOME, else in .cache of the home folder. A variable
// that is set to nothing counts as not set.
func DefaultCacheDir() string {
	if override := os.Getenv("MOONWELL_CACHE"); override != "" {
		return override
	}
	return filepath.Join(userCacheDir(), "moonwell")
}

// userCacheDir is the folder this system keeps a user's caches in.
func userCacheDir() string {
	if runtime.GOOS == "windows" {
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			return local
		}
	}
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return xdg
	}
	return filepath.Join(homeDir(), ".cache")
}

// homeDir is the user's home folder: HOME, else USERPROFILE (a Windows shell sets only that one), else the working
// folder, so that a cache folder always has a name.
func homeDir() string {
	for _, name := range []string{"HOME", "USERPROFILE"} {
		if home := os.Getenv(name); home != "" {
			return home
		}
	}
	return "."
}

// PlatformOf names the platform of a Go operating system and architecture as the lists of downloads do; "" for one
// there are no downloads for.
func PlatformOf(goos, goarch string) string {
	if goarch != "amd64" {
		return ""
	}
	switch goos {
	case "windows":
		return "windows-x86_64"
	case "linux":
		return "linux-x86_64"
	}
	return ""
}

// CurrentPlatform is the platform the program runs on.
func CurrentPlatform() string { return PlatformOf(runtime.GOOS, runtime.GOARCH) }
