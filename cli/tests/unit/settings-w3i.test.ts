import { assertEquals, assertStringIncludes, assertThrows } from "@std/assert";
import { validateMapSettings } from "../../src/settings/options.ts";
import { patchMapInfo } from "../../src/w3i/patch.ts";
import { readMapInfo } from "../../src/w3i/map-info.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import { fixtureBytes, syntheticMapInfo } from "../support/map-settings.ts";

const settings = (input: unknown) => validateMapSettings(input);
const expectMapError = (run: () => unknown) => {
  const error = assertThrows(run, MoonwellError);
  assertEquals(error.file, "map/war3map.w3i");
  return error;
};

Deno.test("v39 loading patch preserves independently recorded extension and tail", async () => {
  const bytes = await fixtureBytes();
  assertEquals([...bytes.subarray(141, 145)], [64, 0, 0, 0]);
  const replacement = new TextEncoder().encode("Loading.mdx\0Text\0Title\0Subtitle\0");
  const expected = new Uint8Array([...bytes.subarray(0, 145), ...replacement, ...bytes.subarray(149)]);
  const values = settings({
    loadingScreen: { model: "Loading.mdx", text: "Text", title: "Title", subtitle: "Subtitle" },
  });
  assertEquals(patchMapInfo(bytes, values), expected);
  const padded = new Uint8Array(bytes.length + 7);
  padded.set(bytes, 3);
  assertEquals(patchMapInfo(padded.subarray(3, 3 + bytes.length), values), expected);
  assertEquals(readMapInfo(bytes, true).details!.players[0].x.start, 311);
  assertEquals(readMapInfo(bytes, true).details!.forces[0].flags.start, 563);
});

Deno.test("colours, water tint and sound environment match a World Editor save", async () => {
  const source = await fixtureBytes();
  const editor = await Deno.readFile(new URL("../fixtures/map-settings-v39/war3map-colors.w3i", import.meta.url));
  const red = [255, 0, 0, 255];
  const details = readMapInfo(editor, true).details!;
  assertEquals(details.waterColor.map((channel) => channel.value), red);
  assertEquals(details.fog.color.map((channel) => channel.value), red);
  assertEquals(details.soundEnvironment.value, "Dungeon");
  const patched = patchMapInfo(
    source,
    settings({ environment: { soundEnvironment: "Dungeon", waterColor: red, fog: { enabled: true, color: red } } }),
  );
  assertEquals(patched.length, editor.length);
  // World Editor also rewrote its save counter, an unknown field and the three camera zoom values.
  const editorOnly = new Set([4, 141, 230, 231, 234, 235, 238, 239]);
  for (let i = 0; i < editor.length; i++) {
    if (!editorOnly.has(i)) assertEquals(patched[i], editor[i], `byte ${i}`);
  }
});

Deno.test("invalid required map structure is a file error", async () => {
  const bytes = await fixtureBytes();
  for (const length of [2, 145, 157, 250, 280, 570]) {
    expectMapError(() => readMapInfo(bytes.subarray(0, length), true, "map/war3map.w3i"));
  }
});

Deno.test("all supported versions preserve every unrelated byte and allow explicit clears", () => {
  for (const version of [18, 25, 28, 31, 32, 33, 39]) {
    const source = syntheticMapInfo(version);
    const values = settings({
      info: { name: "Møønwell", author: "", description: "TRIGSTR_001" },
      loadingScreen: { title: "Changed", background: 7 },
    });
    const changed = patchMapInfo(source, values, "map/war3map.w3i");
    assertEquals(patchMapInfo(source, values, "map/war3map.w3i"), changed);
    const parsed = readMapInfo(changed);
    assertEquals(parsed.info.name.value, "Møønwell");
    assertEquals(parsed.info.author.value, "");
    const restored = patchMapInfo(
      changed,
      settings({
        info: { name: "TRIGSTR_001", author: "Author", description: "Description" },
        loadingScreen: { title: "Title", background: 0 },
      }),
    );
    assertEquals(restored, source);
  }
});

Deno.test("extended edits preserve old-version unknown bytes", () => {
  for (const version of [28, 31, 32, 33, 39]) {
    const source = syntheticMapInfo(version);
    const values = settings({
      players: { "0": { x: 256, controller: "computer" } },
      forces: { "0": { name: "Blue", sharedVision: true } },
      environment: { soundEnvironment: "Dungeon", waterColor: [2, 3, 4, 255], fog: { start: 2000 } },
    });
    const changed = patchMapInfo(source, values);
    const d = readMapInfo(changed, true).details!;
    assertEquals(d.players[0].x.value, 256);
    assertEquals(d.players[0].controller.value, 2);
    assertEquals(d.forces[0].name.value, "Blue");
    assertEquals(d.forces[0].flags.value, 11);
    assertEquals(d.fog.start.value, 2000);
    const restored = patchMapInfo(
      changed,
      settings({
        players: { "0": { x: 128, controller: "user" } },
        forces: { "0": { name: "Force 1", sharedVision: false } },
        environment: { soundEnvironment: "Default", waterColor: [255, 255, 255, 255], fog: { start: 1000 } },
      }),
    );
    // Water-color customization is a monotonic map flag; undo that flag for byte comparison.
    const originalFlag = new DataView(source.buffer).getInt32(readMapInfo(source).flags.start, true);
    new DataView(restored.buffer).setInt32(readMapInfo(restored).flags.start, originalFlag, true);
    assertEquals(restored, source);
  }
});

Deno.test("unsupported and absent structures fail as map-file errors", () => {
  for (const version of [18, 25]) {
    const source = syntheticMapInfo(version);
    expectMapError(() => patchMapInfo(source, settings({ players: { "0": { name: "P" } } }), "map/war3map.w3i"));
  }
  expectMapError(() =>
    patchMapInfo(syntheticMapInfo(18), settings({ loadingScreen: { model: "" } }), "map/war3map.w3i")
  );
  const unsupported = syntheticMapInfo(39);
  new DataView(unsupported.buffer).setInt32(0, 40, true);
  expectMapError(() => readMapInfo(unsupported, false, "map/war3map.w3i"));
  const source = syntheticMapInfo(39);
  expectMapError(() => patchMapInfo(source, settings({ players: { "2": { name: "P" } } }), "map/war3map.w3i"));
  expectMapError(() => patchMapInfo(source, settings({ forces: { "1": { name: "F" } } }), "map/war3map.w3i"));
  expectMapError(() => patchMapInfo(source, settings({ environment: { fog: { start: 6000 } } }), "map/war3map.w3i"));
});

Deno.test("invalid strings, counts, player fields, and script mode are file errors", () => {
  const source = syntheticMapInfo(39);
  const d = readMapInfo(source, true).details!;
  const broken = (change: (bytes: Uint8Array, view: DataView) => void) => {
    const bytes = source.slice();
    change(bytes, new DataView(bytes.buffer));
    expectMapError(() => readMapInfo(bytes, true, "map/war3map.w3i"));
  };
  broken((bytes) => {
    bytes[readMapInfo(source).info.name.start] = 0xff;
  });
  broken((_, view) => view.setInt32(d.players[0].id.start - 4, 0, true));
  broken((_, view) => view.setInt32(d.players[0].id.start - 4, 25, true));
  broken((_, view) => view.setInt32(d.forces[0].flags.start - 4, 0, true));
  broken((_, view) => view.setInt32(d.players[0].controller.start, 0, true));
  broken((_, view) => view.setInt32(d.players[0].race.start, 5, true));
  broken((_, view) => view.setInt32(d.players[0].fixedStart.start, 2, true));
  broken((_, view) => view.setFloat32(d.players[0].x.start, Number.NaN, true));
  broken((_, view) => view.setInt32(d.soundEnvironment.end + 5, 0, true));

  const playerStart = d.players[0].id.start;
  const forceCountStart = d.forces[0].flags.start - 4;
  const duplicate = new Uint8Array(source.length + forceCountStart - playerStart);
  duplicate.set(source.subarray(0, forceCountStart), 0);
  duplicate.set(source.subarray(playerStart, forceCountStart), forceCountStart);
  duplicate.set(source.subarray(forceCountStart), forceCountStart * 2 - playerStart);
  new DataView(duplicate.buffer).setInt32(playerStart - 4, 2, true);
  expectMapError(() => readMapInfo(duplicate, true, "map/war3map.w3i"));
});

Deno.test("non-fog edits ignore unused inherited fog; fog edits reject nonfinite inheritance", () => {
  const source = syntheticMapInfo(39);
  const fog = readMapInfo(source, true).details!.fog;
  new DataView(source.buffer).setFloat32(fog.end.start, Number.POSITIVE_INFINITY, true);
  patchMapInfo(source, settings({ info: { name: "Safe" } }), "map/war3map.w3i");
  patchMapInfo(source, settings({ environment: { soundEnvironment: "Safe" } }), "map/war3map.w3i");
  expectMapError(() => patchMapInfo(source, settings({ environment: { fog: { enabled: true } } }), "map/war3map.w3i"));
  assertEquals(patchMapInfo(source, settings({}), "map/war3map.w3i"), source);
});

Deno.test("force edits require the editor's custom-forces flag", () => {
  const source = syntheticMapInfo(39);
  new DataView(source.buffer).setInt32(readMapInfo(source).flags.start, 0, true);
  expectMapError(() => patchMapInfo(source, settings({ forces: { "0": { name: "X" } } }), "map/war3map.w3i"));
});

Deno.test("map-dependent errors name the settings path that needs the map", () => {
  const source = syntheticMapInfo(39);
  const noForces = syntheticMapInfo(39);
  new DataView(noForces.buffer).setInt32(readMapInfo(noForces).flags.start, 0, true);
  const cases: [Uint8Array, unknown, string][] = [
    [source, { players: { "7": { name: "P" } } }, 'settings.players["7"]: player 7 does not exist in the source map.'],
    [source, { forces: { "1": { name: "F" } } }, 'settings.forces["1"]: force 1 does not exist in the source map.'],
    [noForces, { forces: { "0": { name: "F" } } }, 'settings.forces["0"]: force overrides require custom forces'],
    [source, { environment: { fog: { start: 6000 } } }, "settings.environment.fog: start, end and density must be"],
    [syntheticMapInfo(18), { loadingScreen: { model: "" } }, "settings.loadingScreen.model: custom loading-screen"],
  ];
  for (const [bytes, input, message] of cases) {
    assertStringIncludes(
      expectMapError(() => patchMapInfo(bytes, settings(input), "map/war3map.w3i")).message,
      message,
    );
  }
});
