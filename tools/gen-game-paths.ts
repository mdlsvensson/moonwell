/**
 * Writes cli/data/game-paths.txt from a CASC file-name export:
 * `deno task gen:game-paths <listfile> <game version>`.
 */
import { join } from "@std/path";
import { renderGamePaths } from "../cli/src/models/game-paths.ts";
import { REPO } from "./gen.ts";

/**
 * Writes the in-game path list for a CASC file-name export to `target` and returns its path count. Throws, without
 * writing, when the export has no model or texture path.
 */
export async function generateGamePaths(listText: string, version: string, target: string): Promise<number> {
  const text = renderGamePaths(listText, version);
  const count = text.split("\n").length - 2;
  if (count === 0) {
    throw new Error(
      "no model or texture paths were recognized in the listfile; check the export's format " +
        "(one file name per line, UTF-8). The path list was not changed.",
    );
  }
  await Deno.writeTextFile(target, text);
  return count;
}

if (import.meta.main) {
  const [listfile, version] = Deno.args;
  if (listfile === undefined || version === undefined) {
    console.error("Usage: deno task gen:game-paths <listfile> <game version, e.g. 3.0.0.24268>");
    Deno.exit(1);
  }
  try {
    const count = await generateGamePaths(
      await Deno.readTextFile(listfile),
      version,
      join(REPO, "cli", "data", "game-paths.txt"),
    );
    console.log(`wrote cli/data/game-paths.txt: ${count} paths. Now run \`deno task gen\`.`);
  } catch (error) {
    console.error(`error: ${error instanceof Error ? error.message : String(error)}`);
    Deno.exit(1);
  }
}
