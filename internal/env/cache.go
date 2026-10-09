package env

import (
	"os"
	"path/filepath"
	"runtime"
)

func DefaultCacheDir() string {
	if override := os.Getenv("MOONWELL_CACHE"); override != "" {
		return override
	}
	return filepath.Join(userCacheDir(), "moonwell")
}

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

func homeDir() string {
	for _, name := range []string{"HOME", "USERPROFILE"} {
		if home := os.Getenv(name); home != "" {
			return home
		}
	}
	return "."
}
