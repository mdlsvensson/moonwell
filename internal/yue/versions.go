// Package yue installs the YueScript compiler and runs it: compiling a project's sources and listing the globals
// they use.
package yue

import "runtime"

// DefaultVersion is the compiler version a project gets unless its manifest says otherwise.
const DefaultVersion = "0.34.2"

// Asset is a compiler build Moonwell can download.
type Asset struct {
	URL    string
	SHA256 string
	// Archive is "7z" or "zip".
	Archive string
	// Binary is the path of the compiler inside the archive.
	Binary string
}

const releases = "https://github.com/IppClub/YueScript/releases/download"

// Known are the compiler builds Moonwell can install, by version and then by platform, with checksums verified
// when the version was pinned.
var Known = map[string]map[string]Asset{
	"0.34.2": {
		"windows-x86_64": {
			URL:     releases + "/v0.34.2/yue-windows-x64.7z",
			SHA256:  "367e79f450dc60d96d248c8e1d97b4ec47729b963f64226888fb8e111fc349bf",
			Archive: "7z",
			Binary:  "yue.exe",
		},
		"linux-x86_64": {
			URL:     releases + "/v0.34.2/yue-linux-x86_64.zip",
			SHA256:  "fffcaa3624bc61e0a2a40cb117fe59460d6503d08348857229ce7d2b2f2b7c2d",
			Archive: "zip",
			Binary:  "yue",
		},
	},
}

// Platform names the platform of a Go operating system and architecture as Known does; "" for one Moonwell has no
// compiler for.
func Platform(goos, goarch string) string {
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

// CurrentPlatform is the platform Moonwell runs on.
func CurrentPlatform() string { return Platform(runtime.GOOS, runtime.GOARCH) }

// machine names this machine as "os/architecture", with the architecture names x86_64 and aarch64.
func machine() string {
	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		arch = "x86_64"
	case "arm64":
		arch = "aarch64"
	}
	return runtime.GOOS + "/" + arch
}
