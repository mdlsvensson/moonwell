export const SETTINGS_FIXTURE = new URL("../fixtures/map-settings-v39/", import.meta.url);
export const fixtureBytes = () => Deno.readFile(new URL("war3map.w3i", SETTINGS_FIXTURE));
export const fixtureLua = () => Deno.readTextFile(new URL("war3map.lua", SETTINGS_FIXTURE));

/** Synthetic layout data for old versions; only v39 is editor-recorded. */
export function syntheticMapInfo(version: number): Uint8Array {
  const out: number[] = [];
  const int = (n: number) => {
    const b = new Uint8Array(4);
    new DataView(b.buffer).setInt32(0, n, true);
    out.push(...b);
  };
  const float = (n: number) => {
    const b = new Uint8Array(4);
    new DataView(b.buffer).setFloat32(0, n, true);
    out.push(...b);
  };
  const text = (s: string) => out.push(...new TextEncoder().encode(s), 0);
  const skip = (n: number) => out.push(...new Uint8Array(n));
  int(version);
  skip(version >= 28 ? 24 : 8);
  for (const s of ["TRIGSTR_001", "Author", "Description", "1-2"]) text(s);
  skip(56);
  int(0x40);
  out.push(65);
  int(0);
  if (version === 39) int(64);
  if (version >= 25) text("");
  for (const s of ["Loading", "Title", "Subtitle"]) text(s);
  if (version >= 28) {
    int(0);
    for (let i = 0; i < 4; i++) text("");
    int(0);
    float(1000);
    float(5000);
    float(0.5);
    out.push(1, 2, 3, 255);
    int(0);
    if (version === 39) skip(24);
    text("Default");
    out.push(65, 255, 255, 255, 255);
    int(1);
    if (version >= 31) skip(8);
    if (version >= 32) skip(8);
    if (version >= 33) skip(4);
    if (version === 39) skip(40);
    int(1);
    int(0);
    int(1);
    int(1);
    if (version === 39) int(64);
    int(1);
    text("Player 1");
    float(128);
    float(-896);
    skip(version >= 31 ? 16 : 8);
    int(1);
    int(3);
    int(1);
    text("Force 1");
  }
  out.push(0xde, 0xad, 0xbe, 0xef);
  return new Uint8Array(out);
}
