/** Writes cli/data/game-paths.txt from a CASC file-name export: `deno task gen:game-paths <listfile> <game version>`. */
import { join } from "@std/path";
import { renderGamePaths } from "../cli/src/models/game-paths.ts";
import { REPO } from "./gen.ts";

if (import.meta.main) {
  const [listfile, version] = Deno.args;
  if (listfile === undefined || version === undefined) {
    console.error("Usage: deno task gen:game-paths <listfile> <game version, e.g. 3.0.0.24268>");
    Deno.exit(1);
  }
  const text = renderGamePaths(await Deno.readTextFile(listfile), version);
  await Deno.writeTextFile(join(REPO, "cli", "data", "game-paths.txt"), text);
  console.log(`wrote cli/data/game-paths.txt: ${text.split("\n").length - 2} paths. Now run \`deno task gen\`.`);
}
