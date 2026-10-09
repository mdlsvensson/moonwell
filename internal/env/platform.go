package env

import (
	"runtime"
)

func CurrentPlatform() string { return PlatformOf(runtime.GOOS, runtime.GOARCH) }

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
