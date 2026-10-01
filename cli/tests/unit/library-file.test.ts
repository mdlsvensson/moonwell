import { assertEquals, assertThrows } from "@std/assert";
import { LIBRARY_FILE, parseLibraryFile } from "../../src/libraries/manifest.ts";
import { MoonwellError } from "../../src/shared/errors.ts";

const bytes = (text: string) => new TextEncoder().encode(text);
const WHERE = "https://github.com/owner/lib/blob/v1/moonwell-library.json";
const REPORT = "Report it to the library's author, or use another tag of the library.";

Deno.test("a library without the file ships modules from its root and no assets", () => {
  assertEquals(LIBRARY_FILE, "moonwell-library.json");
  assertEquals(parseLibraryFile("ex", undefined, WHERE), { dir: null, assets: null });
});

Deno.test("the file names the module folder and the assets folder, each optional", () => {
  assertEquals(parseLibraryFile("ex", bytes('{ "dir": "src", "assets": "assets" }'), WHERE), {
    dir: "src",
    assets: "assets",
  });
  assertEquals(parseLibraryFile("ex", bytes('{"assets":"files/for the map"}'), WHERE), {
    dir: null,
    assets: "files/for the map",
  });
  assertEquals(parseLibraryFile("ex", bytes('{"dir":"lua/lib"}'), WHERE), { dir: "lua/lib", assets: null });
  assertEquals(parseLibraryFile("ex", bytes("{}"), WHERE), { dir: null, assets: null });
});

Deno.test("a file that is not a JSON object is refused, naming the library and the file", () => {
  for (
    const [text, problem] of [["{", "is not valid JSON."], ["[]", "is not a JSON object."], [
      "null",
      "is not a JSON object.",
    ]]
  ) {
    const error = assertThrows(() => parseLibraryFile("ex", bytes(text), WHERE), MoonwellError);
    assertEquals(error.message, `Library ex: moonwell-library.json ${problem}`);
    assertEquals(error.file, WHERE);
    assertEquals(error.hint, REPORT);
  }
  const invalid = assertThrows(
    () => parseLibraryFile("ex", new Uint8Array([...bytes('{"dir":"s'), 0xff, ...bytes('"}')]), WHERE),
    MoonwellError,
  );
  assertEquals(invalid.message, "Library ex: moonwell-library.json is not valid JSON.");
});

Deno.test("an unknown key is refused, the first in sorted order, with the hint about a newer Moonwell", () => {
  const error = assertThrows(
    () => parseLibraryFile("ex", bytes('{"objects":"objects","dir":"src","extra":1}'), WHERE),
    MoonwellError,
  );
  assertEquals(error.message, 'Library ex: moonwell-library.json has an unknown key "extra".');
  assertEquals(error.file, WHERE);
  assertEquals(error.hint, "This Moonwell knows dir and assets; the library may need a newer Moonwell.");
});

Deno.test("a folder must be a relative path of plain names", () => {
  for (const value of ["", ".", "..", "a/../b", "/abs", "a//b", "a/", "a\\b", "C:/x", 7, null, ["src"]]) {
    for (const name of ["dir", "assets"]) {
      const error = assertThrows(
        () => parseLibraryFile("ex", bytes(JSON.stringify({ [name]: value })), WHERE),
        MoonwellError,
      );
      assertEquals(
        error.message,
        `Library ex: moonwell-library.json has ${name} = ${
          JSON.stringify(value)
        }, which is not a folder inside the library.`,
      );
      assertEquals(error.hint, REPORT);
    }
  }
});
