import { join } from "@std/path";
import { MoonwellError } from "../shared/errors.ts";
import { listFiles } from "../shared/fs.ts";
import { buildHm3wHeader } from "../mpq/hm3w.ts";
import { writeMpq } from "../mpq/writer.ts";
import { isHeaderlessArchive, readW3iHeader } from "./w3i.ts";

/** Archive metadata a saved map folder may carry; stale copies must never be packed. */
const ARCHIVE_METADATA = new Set(["(ATTRIBUTES)", "(LISTFILE)", "(SIGNATURE)"]);

/** Packs a staged map folder into .w3x bytes. */
export async function packMap(mapDir: string, mapName: string): Promise<Uint8Array> {
  let info: Uint8Array;
  try {
    info = await Deno.readFile(join(mapDir, "war3map.w3i"));
  } catch (error) {
    if (!(error instanceof Deno.errors.NotFound)) throw error;
    throw new MoonwellError("war3map.w3i is missing from the map folder.", {
      file: mapDir,
      hint: "Save the source map from World Editor in folder format.",
    });
  }
  const paths = (await listFiles(mapDir)).filter((file) => !ARCHIVE_METADATA.has(file.toUpperCase()));
  const files = await Promise.all(
    paths.map(async (file) => ({
      name: file.split("/").join("\\"),
      data: await Deno.readFile(join(mapDir, ...file.split("/"))),
    })),
  );
  const prefix = isHeaderlessArchive(readW3iHeader(info)) ? undefined : buildHm3wHeader(mapName);
  return writeMpq(files, { prefix });
}
