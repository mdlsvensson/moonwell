import { join } from "@std/path";
import { MACROS_YUE } from "../embedded/macros.ts";
import { MoonwellError } from "../shared/errors.ts";
import { sha256Hex } from "../shared/fs.ts";

/** Where `check`, `build`, `test`, `dev` and `setup` write the macro module, relative to the project root (spec §6). */
export const MACROS_FILE = ".moonwell/yue/moonwell/macros.yue";

/** How a `yue` run finds `import "moonwell.macros"`. */
export interface MacroSearch {
  /** The compiler's `--path`: yue tries each `.lua` pattern with `.yue`, so this finds `moonwell/macros.yue`. */
  path: string;
  /** SHA-256 of the macro module. It joins the compile and `yue -g` cache keys, so a changed macro redoes every file. */
  hash: string;
}

export async function macroSearch(root: string): Promise<MacroSearch> {
  // Lua's search path splits on ";" and puts the module name in place of "?", so neither may be in the root.
  if (root.includes(";") || root.includes("?")) {
    throw new MoonwellError(
      `The project folder's path contains ";" or "?", which YueScript's module search cannot handle.`,
      { file: root, hint: "Move the project to a folder whose path has neither character." },
    );
  }
  return {
    path: join(root, ".moonwell", "yue", "?.lua"),
    hash: await sha256Hex(new TextEncoder().encode(MACROS_YUE)),
  };
}

/** The `--path` arguments of a `yue` run; none without `macros`. They go before the source file. */
export function macroPathArgs(macros: MacroSearch | undefined): string[] {
  return macros === undefined ? [] : ["--path", macros.path];
}
