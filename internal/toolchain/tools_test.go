package toolchain

import (
	"testing"

	"github.com/mdlsvensson/moonwell/internal/env"
)

func TestPklPinsVersion0321ForWindowsAndLinux(t *testing.T) {
	const releases = "https://github.com/apple/pkl/releases/download/0.32.1/"
	want := map[string]Asset{
		"windows-x86_64": {
			URL:    releases + "pkl-windows-amd64.exe",
			SHA256: "8550a00fcf027335e42c5e2cd553b88e98845408cb1880b3e3d1860caf46d22a",
			Binary: "pkl.exe",
		},
		"linux-x86_64": {
			URL:    releases + "pkl-linux-amd64",
			SHA256: "3180b62da95c0cad1d904e9bb6c5f4a8f9032413c21e53194bb91ff1ee5f3211",
			Binary: "pkl",
		},
	}
	if PklVersion != "0.32.1" || Pkl.Name != "pkl" || len(Pkl.Versions) != 1 || len(Pkl.Versions[PklVersion]) != len(want) {
		t.Fatalf("PklVersion = %s, Pkl = %+v", PklVersion, Pkl)
	}
	for platform, asset := range want {
		if Pkl.Versions[PklVersion][platform] != asset {
			t.Errorf("the download for %s = %+v", platform, Pkl.Versions[PklVersion][platform])
		}
	}
}

func TestKnownVersionsPinTheDefaultForWindowsAndLinux(t *testing.T) {
	const releases = "https://github.com/IppClub/YueScript/releases/download"
	want := map[string]map[string]Asset{
		"0.34.3": {
			"windows-x86_64": {releases + "/v0.34.3/yue-windows-x64.7z",
				"548b2fe699f46080cbca6c3d5951df2bcbcbb6bbdd215744054020962e6b7075", "7z", "yue.exe"},
			"linux-x86_64": {releases + "/v0.34.3/yue-linux-x86_64.zip",
				"9f47c8c7d3b6aa6e439786ae4708b9e070edbb01876712e1212917decd01d916", "zip", "yue"},
		},
		"0.34.2": {
			"windows-x86_64": {releases + "/v0.34.2/yue-windows-x64.7z",
				"367e79f450dc60d96d248c8e1d97b4ec47729b963f64226888fb8e111fc349bf", "7z", "yue.exe"},
			"linux-x86_64": {releases + "/v0.34.2/yue-linux-x86_64.zip",
				"fffcaa3624bc61e0a2a40cb117fe59460d6503d08348857229ce7d2b2f2b7c2d", "zip", "yue"},
		},
	}
	if YueVersion != "0.34.3" || YueScript.Name != "yue" || len(YueScript.Versions) != len(want) {
		t.Fatalf("YueVersion = %s, YueScript = %+v", YueVersion, YueScript)
	}
	for version, platforms := range want {
		if len(YueScript.Versions[version]) != len(platforms) {
			t.Errorf("version %s has the downloads %+v", version, YueScript.Versions[version])
		}
		for platform, asset := range platforms {
			if YueScript.Versions[version][platform] != asset {
				t.Errorf("%s for %s = %+v", version, platform, YueScript.Versions[version][platform])
			}
		}
	}
	for _, system := range []string{"windows", "linux"} {
		if _, found := YueScript.Versions[YueVersion][env.PlatformOf(system, "amd64")]; !found {
			t.Errorf("no download of %s for %s", YueVersion, system)
		}
	}
}
