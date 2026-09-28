import { assertEquals, assertStringIncludes } from "@std/assert";
import { fromFileUrl, join } from "@std/path";

const REPO = fromFileUrl(new URL("../../../", import.meta.url));
const MAIN = join(REPO, "cli", "src", "main.ts");

async function deno(args: string[], cwd: string) {
  const output = await new Deno.Command(Deno.execPath(), { args, cwd, stdout: "piped", stderr: "piped" }).output();
  const decoder = new TextDecoder();
  return { code: output.code, text: decoder.decode(output.stdout) + decoder.decode(output.stderr) };
}

async function newProject(): Promise<string> {
  const project = join(await Deno.makeTempDir({ prefix: "moonwell-e2e-" }), "my-map");
  const result = await deno(["run", "-A", MAIN, "init", "--link", project], REPO);
  assertEquals(result.code, 0, result.text);
  return project;
}

async function edit(path: string, from: string, to: string): Promise<void> {
  const text = await Deno.readTextFile(path);
  if (!text.includes(from)) throw new Error(`${path} does not contain ${from}`);
  await Deno.writeTextFile(path, text.replace(from, to));
}

const TYPO = "error: src/main.yue:9:10 › Unknown global CreatUnit.\n" +
  "hint: Did you mean CreateUnit? Declare your own globals with `global`, or add them to lint.globals in moonwell.pkl.";

Deno.test("check fails on a misspelt native, naming its position and the nearest name", async () => {
  const project = await newProject();
  await edit(join(project, "src", "main.yue"), "CreateUnit Player(0)", "CreatUnit Player(0)");
  const checked = await deno(["task", "check"], project);
  assertEquals(checked.code, 1, checked.text);
  assertStringIncludes(checked.text, TYPO);
});

Deno.test("with unknownGlobals = warning, build reports the typo and succeeds", async () => {
  const project = await newProject();
  await edit(join(project, "src", "main.yue"), "CreateUnit Player(0)", "CreatUnit Player(0)");
  await edit(join(project, "moonwell.pkl"), 'unknownGlobals = "error"', 'unknownGlobals = "warning"');
  const built = await deno(["task", "build"], project);
  assertEquals(built.code, 0, built.text);
  assertStringIncludes(built.text, TYPO.replace("error: ", "warning: "));
  assertStringIncludes(built.text, "Built dist/bin/map.w3x");
});

Deno.test("check accepts declared globals, lint.globals and the map's globals", async () => {
  const project = await newProject();
  await Deno.writeTextFile(join(project, "src", "state.yue"), "global Round = 1\n");
  const main = join(project, "src", "main.yue");
  const base = await Deno.readTextFile(main);
  await Deno.writeTextFile(
    main,
    `${base}\nimport "state"\nglobal Score = 0\nprint Score, Round, MyLibrary, gg_unit_Hblm_0003\n`,
  );
  await edit(join(project, "moonwell.pkl"), "globals = List()", 'globals = List("MyLibrary")');
  const checked = await deno(["task", "check"], project);
  assertEquals(checked.code, 0, checked.text);
});
