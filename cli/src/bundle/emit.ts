import { MoonwellError } from "../shared/errors.ts";
import type { CompiledModule } from "../yue/compile.ts";

function sourceLines(text: string): string[] {
  const lines = text.replace(/\r\n/g, "\n").split("\n");
  if (lines.length > 1 && lines[lines.length - 1] === "") lines.pop();
  return lines;
}

/**
 * Renders the bundle block; `firstLine` is the war3map.lua line number the leading "do" will occupy.
 * Minified modules keep no source lines, so their errors name the module file only.
 */
export function emitBundle(
  input: { runtime: string; modules: CompiledModule[]; entry: string; firstLine: number; minify?: boolean },
): string {
  const out: string[] = ["do", ...sourceLines(input.runtime)];
  const ranges: string[] = [];
  const nextLine = () => input.firstLine + out.length;
  for (const module of input.modules) {
    out.push(`__mw.define(${JSON.stringify(module.name)}, function(...)`);
    const start = nextLine();
    const body = sourceLines(module.source);
    out.push(...body);
    ranges.push(
      `{${start}, ${start + body.length - 1}, ${JSON.stringify(module.name)}, ${JSON.stringify(module.sourcePath)}},`,
    );
    out.push("end)");
  }
  out.push("__mw.lines = {", ...ranges, "}");
  if (input.minify) out.push("__mw.minified = true");
  out.push("__mw.install()", `__mw.boot(${JSON.stringify(input.entry)})`, "end");
  return out.join("\n") + "\n";
}

/** Appends the bundle to a World Editor war3map.lua that defines global main and config. */
export function injectBundle(war3mapLua: string, bundle: (firstLine: number) => string, file = "war3map.lua"): string {
  for (const name of ["main", "config"]) {
    if (!new RegExp(`^\\s*function\\s+${name}\\s*\\(`, "m").test(war3mapLua)) {
      throw new MoonwellError(`The map script does not define function ${name}().`, {
        file,
        hint: "Save the map in World Editor with Lua as the script language (Scenario › Map Options).",
      });
    }
  }
  const base = war3mapLua.endsWith("\n") ? war3mapLua : `${war3mapLua}\n`;
  const firstLine = base.split("\n").length;
  return base + bundle(firstLine);
}
