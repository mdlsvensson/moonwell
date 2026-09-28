import { join } from "@std/path";
import type { SourceModule } from "../bundle/modules.ts";
import { MoonwellError } from "../shared/errors.ts";
import { listFiles, removeFileIfExists, writeTextIfChanged } from "../shared/fs.ts";
import type { CompiledModule } from "../yue/compile.ts";

/** The editor's view of library modules: Lua by module path, which `.luarc.json`'s `workspace.library` lists. */
export const LIBRARY_VIEW_DIR = ".moonwell/lua";

/**
 * Brings `.moonwell/lua/` up to date with the libraries' modules (spec §5.4): each module with a `library` as
 * `<name with "." as "/">.lua`, a Lua module's source or a YueScript module's compiled output from `loadCompiled`
 * (skipped when there is none). Files are written only when their content differs, and every other file under the
 * folder is removed. Returns the POSIX paths it wrote.
 */
export async function refreshLibraryView(
  root: string,
  modules: readonly SourceModule[],
  loadCompiled?: (module: SourceModule) => CompiledModule | undefined,
): Promise<string[]> {
  const files = new Map<string, string>();
  for (const module of modules) {
    if (module.library === undefined) continue;
    const text = module.kind === "lua" ? module.source ?? "" : loadCompiled?.(module)?.source;
    if (text === undefined) continue;
    files.set(`${module.name.split(".").join("/")}.lua`, text);
  }
  const dir = join(root, ...LIBRARY_VIEW_DIR.split("/"));
  const written: string[] = [];
  try {
    for (const [file, text] of files) {
      if (await writeTextIfChanged(join(dir, ...file.split("/")), text)) written.push(`${LIBRARY_VIEW_DIR}/${file}`);
    }
    for (const file of await existingFiles(dir)) {
      if (!files.has(file)) await removeFileIfExists(join(dir, ...file.split("/")));
    }
  } catch (cause) {
    if (cause instanceof MoonwellError) throw cause;
    const reason = cause instanceof Error ? cause.message : String(cause);
    throw new MoonwellError(`Writing ${LIBRARY_VIEW_DIR} failed: ${reason}`, {
      file: LIBRARY_VIEW_DIR,
      cause,
      hint: "The editor reads .moonwell/; make sure it is a folder you can write, then retry.",
    });
  }
  return written;
}

async function existingFiles(dir: string): Promise<string[]> {
  try {
    return await listFiles(dir);
  } catch (error) {
    if (error instanceof Deno.errors.NotFound) return [];
    throw error;
  }
}
