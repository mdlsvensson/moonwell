export type Platform = "windows-x86_64" | "linux-x86_64";

export interface YueAsset {
  url: string;
  sha256: string;
  archive: "7z" | "zip";
  /** Path of the compiler inside the archive. */
  binary: string;
}

export type KnownVersions = Record<string, Partial<Record<Platform, YueAsset>>>;

export const DEFAULT_YUE_VERSION = "0.34.2";

const RELEASES = "https://github.com/IppClub/YueScript/releases/download";

/** Compiler builds Moonwell can install, with checksums verified at pin time. */
export const KNOWN_YUE: KnownVersions = {
  "0.34.2": {
    "windows-x86_64": {
      url: `${RELEASES}/v0.34.2/yue-windows-x64.7z`,
      sha256: "367e79f450dc60d96d248c8e1d97b4ec47729b963f64226888fb8e111fc349bf",
      archive: "7z",
      binary: "yue.exe",
    },
    "linux-x86_64": {
      url: `${RELEASES}/v0.34.2/yue-linux-x86_64.zip`,
      sha256: "fffcaa3624bc61e0a2a40cb117fe59460d6503d08348857229ce7d2b2f2b7c2d",
      archive: "zip",
      binary: "yue",
    },
  },
};

export function currentPlatform(os: string = Deno.build.os, arch: string = Deno.build.arch): Platform | undefined {
  if (arch !== "x86_64") return undefined;
  if (os === "windows") return "windows-x86_64";
  if (os === "linux") return "linux-x86_64";
  return undefined;
}
