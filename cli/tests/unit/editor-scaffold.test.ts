import { assert, assertEquals } from "@std/assert";
import { decodeBase64, encodeBase64 } from "@std/encoding/base64";
import { join } from "@std/path";
import { addEditorFiles, EDITOR_FILES } from "../../src/editor/scaffold.ts";
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
