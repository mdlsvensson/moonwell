package toolchain

import (
	"regexp"

	"github.com/mdlsvensson/moonwell/internal/fsx"
)

const (
	YueVersion = "0.34.3"
	PklVersion = "0.32.1"
)

const (
	yueReleases = "https://github.com/IppClub/YueScript/releases/download"
	pklReleases = "https://github.com/apple/pkl/releases/download/" + PklVersion + "/"
	pklPage     = "https://pkl-lang.org/main/current/pkl-cli/index.html#installation"
)

var YueScript = Tool{
	Name:  "yue",
	Title: "YueScript",
	Versions: map[string]map[string]Asset{
		"0.34.3": {
			"windows-x86_64": {
				URL:     yueReleases + "/v0.34.3/yue-windows-x64.7z",
				SHA256:  "548b2fe699f46080cbca6c3d5951df2bcbcbb6bbdd215744054020962e6b7075",
				Archive: "7z",
				Binary:  "yue.exe",
			},
			"linux-x86_64": {
				URL:     yueReleases + "/v0.34.3/yue-linux-x86_64.zip",
				SHA256:  "9f47c8c7d3b6aa6e439786ae4708b9e070edbb01876712e1212917decd01d916",
				Archive: "zip",
				Binary:  "yue",
			},
		},
		"0.34.2": {
			"windows-x86_64": {
				URL:     yueReleases + "/v0.34.2/yue-windows-x64.7z",
				SHA256:  "367e79f450dc60d96d248c8e1d97b4ec47729b963f64226888fb8e111fc349bf",
				Archive: "7z",
				Binary:  "yue.exe",
			},
			"linux-x86_64": {
				URL:     yueReleases + "/v0.34.2/yue-linux-x86_64.zip",
				SHA256:  "fffcaa3624bc61e0a2a40cb117fe59460d6503d08348857229ce7d2b2f2b7c2d",
				Archive: "zip",
				Binary:  "yue",
			},
		},
	},
	VersionArgs:       []string{"-v"},
	VersionPattern:    regexp.MustCompile(`Yuescript version: ([^` + fsx.ASCIISpace + `]+)`),
	ManualInstallHint: "build or install yue yourself and set yue.path in moonwell.local.pkl.",
}

var Pkl = Tool{
	Name:  "pkl",
	Title: "Pkl",
	Versions: map[string]map[string]Asset{
		PklVersion: {
			"windows-x86_64": {
				URL:    pklReleases + "pkl-windows-amd64.exe",
				SHA256: "8550a00fcf027335e42c5e2cd553b88e98845408cb1880b3e3d1860caf46d22a",
				Binary: "pkl.exe",
			},
			"linux-x86_64": {
				URL:    pklReleases + "pkl-linux-amd64",
				SHA256: "3180b62da95c0cad1d904e9bb6c5f4a8f9032413c21e53194bb91ff1ee5f3211",
				Binary: "pkl",
			},
		},
	},
	VersionArgs:       []string{"--version"},
	VersionPattern:    regexp.MustCompile(`Pkl (\d+\.\d+\.\d+)`),
	ManualInstallHint: "install Pkl 0.32 or newer yourself: " + pklPage,
}
