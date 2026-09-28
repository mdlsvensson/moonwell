import { assert, assertEquals } from "@std/assert";
import { decodeBase64, encodeBase64 } from "@std/encoding/base64";
import { join } from "@std/path";
import { addEditorFiles, EDITOR_FILES, mergeLuarc } from "../../src/editor/scaffold.ts";
import { TEMPLATE_FILES } from "../../src/embedded/template.ts";
import { loadNatives } from "../../src/natives/natives.ts";

const b64 = (text: string) => encodeBase64(new TextEncoder().encode(text));
const FILES = [
  { path: "yueconfig.yue", base64: b64("return {}\n") },
  { path: ".luarc.json", base64: b64("{}\n") },
  { path: ".vscode/extensions.json", base64: b64('{"recommendations":[]}\n') },
  { path: "src/main.yue", base64: b64("print 1\n") },
];

Deno.test("the template ships every editor file", () => {
  const paths = new Set(TEMPLATE_FILES.map((file) => file.path));
  for (const path of EDITOR_FILES) assertEquals(paths.has(path), true, path);
});

Deno.test("the template's .luarc.json indexes the compiled .lua files and suggests every game global", async () => {
  const file = TEMPLATE_FILES.find((entry) => entry.path === ".luarc.json");
  const config = JSON.parse(new TextDecoder().decode(decodeBase64(file!.base64)));
  assertEquals(config["workspace.useGitIgnore"], false);
  // The YueScript extension asks for completion at a placeholder word, so the typed prefix never narrows the list:
  // LuaLS must be allowed to suggest every native and game global at once.
  const natives = await loadNatives();
  const names = natives.functions.length + natives.globals.length;
  const limit = config["completion.maxSuggestCount"];
  assert(typeof limit === "number" && limit >= names, `completion.maxSuggestCount ${limit} < ${names} names`);
});

Deno.test("addEditorFiles adds missing files and .gitignore lines, and never overwrites", async () => {
  const root = await Deno.makeTempDir({ prefix: "moonwell-scaffold-" });
  await Deno.writeTextFile(join(root, ".luarc.json"), "mine\n");
  await Deno.writeTextFile(join(root, ".gitignore"), "dist/\r\n.moonwell/\r\n");
  assertEquals(await addEditorFiles(root, FILES), [
    "yueconfig.yue",
    ".vscode/extensions.json",
    ".gitignore (src/**/*.lua)",
  ]);
  assertEquals(await Deno.readTextFile(join(root, ".luarc.json")), "mine\n");
  assertEquals(await Deno.readTextFile(join(root, "yueconfig.yue")), "return {}\n");
  assertEquals(await Deno.readTextFile(join(root, ".gitignore")), "dist/\r\n.moonwell/\r\nsrc/**/*.lua\n");
  assertEquals(await addEditorFiles(root, FILES), []);
});

Deno.test("addEditorFiles creates .gitignore when there is none", async () => {
  const root = await Deno.makeTempDir({ prefix: "moonwell-scaffold-" });
  await addEditorFiles(root, FILES);
  assertEquals(await Deno.readTextFile(join(root, ".gitignore")), ".moonwell/\nsrc/**/*.lua\n");
});

Deno.test("mergeLuarc adds the template's missing runtime.path and workspace.library entries, keeping the rest", async () => {
  const root = await Deno.makeTempDir({ prefix: "moonwell-luarc-" });
  try {
    await Deno.writeTextFile(
      join(root, ".luarc.json"),
      JSON.stringify({
        "runtime.path": ["src/?.lua"],
        "workspace.library": [".moonwell/types", "extra"],
        "diagnostics.globals": ["X"],
      }),
    );
    assertEquals(await mergeLuarc(root), ["src/?/init.lua", "lua/?.lua", "lua/?/init.lua"]);
    const config = JSON.parse(await Deno.readTextFile(join(root, ".luarc.json")));
    assertEquals(config["runtime.path"], ["src/?.lua", "src/?/init.lua", "lua/?.lua", "lua/?/init.lua"]);
    assertEquals(config["workspace.library"], [".moonwell/types", "extra"]);
    assertEquals(config["diagnostics.globals"], ["X"]);
    assertEquals(await mergeLuarc(root), []);
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});

Deno.test("mergeLuarc leaves a .luarc.json that is not a JSON object alone", async () => {
  const root = await Deno.makeTempDir({ prefix: "moonwell-luarc-" });
  try {
    const text = '// a comment\n{ "runtime.path": [] }\n';
    await Deno.writeTextFile(join(root, ".luarc.json"), text);
    assertEquals(await mergeLuarc(root), undefined);
    assertEquals(await Deno.readTextFile(join(root, ".luarc.json")), text);
    await Deno.remove(join(root, ".luarc.json"));
    assertEquals(await mergeLuarc(root), [], "no file: nothing to merge");
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});
