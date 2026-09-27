import { join } from "@std/path";
import type { CommandContext } from "../context.ts";
import { type MapGlobals, readMapGlobals } from "../editor/map-globals.ts";
import { readSourceScript } from "../editor/refresh.ts";
import { loadNatives, type Natives } from "../natives/natives.ts";
import type { Project } from "../project/project.ts";
import { formatProblem, MAX_PROBLEMS, type Problem, ProblemsError } from "../shared/errors.ts";
import { closestNames, joinWords } from "../shared/names.ts";
import { type GlobalUse, listGlobalUses } from "./uses.ts";

export const UNKNOWN_GLOBAL_HINT =
  "Declare your own globals with `global`, or add them to lint.globals in moonwell.pkl.";

const NAME = /^[A-Za-z_][A-Za-z0-9_]*$/;

/**
 * The names a source's `global` lines declare (spec §5.1): `global Score = 0`, `global a, b`, `global const K = 1`,
 * `global class Boss`. `global *` and `global ^` name nothing (spec §5.5).
 */
export function declaredGlobals(source: string): string[] {
  const names: string[] = [];
  for (const line of source.split(/\r?\n/)) {
    const match = /^\s*global\s+(.*)$/.exec(line);
    if (!match) continue;
    let rest = match[1].replace(/--.*$/, "").trim();
    const keyword = /^(const|class)\s+(.*)$/.exec(rest);
    if (keyword?.[1] === "class") {
      const name = /^[A-Za-z_][A-Za-z0-9_]*/.exec(keyword[2]);
      if (name) names.push(name[0]);
      continue;
    }
    if (keyword) rest = keyword[2];
    for (const part of rest.split("=")[0].split(",")) {
      const name = part.trim();
      if (NAME.test(name)) names.push(name);
    }
  }
  return names;
}

/** Every name a gameplay file may use as a global (spec §5.1). */
export function knownGlobals(inputs: {
  natives: Natives;
  /** The source map's war3map.lua; absent when there is none. */
  map?: MapGlobals;
  declared: Iterable<string>;
  /** lint.globals from the manifest. */
  extra: readonly string[];
}): Set<string> {
  const known = new Set<string>();
  for (const fn of inputs.natives.functions) known.add(fn.name);
  for (const global of inputs.natives.globals) known.add(global.name);
  for (const name of inputs.natives.lua.globals) known.add(name);
  for (const global of inputs.map?.globals ?? []) known.add(global.name);
  for (const fn of inputs.map?.functions ?? []) known.add(fn);
  for (const name of inputs.declared) known.add(name);
  for (const name of inputs.extra) known.add(name);
  return known;
}

/**
 * One problem per use of a name not in `known`, sorted by file, line and column (spec §5.4). `uses` is keyed by path
 * under src/. `removed` lists the standard-library globals the game takes away.
 */
export function unknownGlobalProblems(
  uses: Record<string, GlobalUse[]>,
  known: ReadonlySet<string>,
  removed: readonly string[],
): Problem[] {
  const hints = new Map<string, string>();
  const hintFor = (name: string): string => {
    let hint = hints.get(name);
    if (hint === undefined) {
      const closest = closestNames(known, name);
      hint = removed.includes(name)
        ? `Warcraft III's Lua does not provide ${name}.`
        : closest.length > 0
        ? `Did you mean ${joinWords(closest, "or")}? ${UNKNOWN_GLOBAL_HINT}`
        : UNKNOWN_GLOBAL_HINT;
      hints.set(name, hint);
    }
    return hint;
  };
  const problems: Problem[] = [];
  for (const [file, fileUses] of Object.entries(uses)) {
    for (const use of fileUses) {
      if (known.has(use.name)) continue;
      problems.push({
        file: `src/${file}`,
        line: use.line,
        column: use.column,
        message: `Unknown global ${use.name}.`,
        hint: hintFor(use.name),
      });
    }
  }
  return problems.sort((a, b) =>
    (a.file < b.file ? -1 : a.file > b.file ? 1 : 0) || a.line! - b.line! || a.column! - b.column!
  );
}

/**
 * Checks every compiled source for unknown globals (spec §5). With `lint.unknownGlobals = "error"` any unknown use
 * throws a `ProblemsError` listing all of them; with `"warning"` they are logged and returned.
 */
export async function checkUnknownGlobals(
  ctx: CommandContext,
  project: Project,
  compiled: { yue: string; hashes: Record<string, string> },
  natives?: Natives,
): Promise<Problem[]> {
  const uses = await listGlobalUses({ yue: compiled.yue, root: ctx.root, hashes: compiled.hashes, run: ctx.run });
  const declared: string[] = [];
  for (const file of Object.keys(compiled.hashes)) {
    declared.push(...declaredGlobals(await Deno.readTextFile(join(ctx.root, "src", ...file.split("/")))));
  }
  const label = `maps/${project.map.folder}/war3map.lua`;
  const script = await readSourceScript(join(ctx.root, ...label.split("/")), label);
  const data = natives ?? await loadNatives();
  const known = knownGlobals({
    natives: data,
    map: script === undefined ? undefined : readMapGlobals(script),
    declared,
    extra: project.lint.globals,
  });
  const problems = unknownGlobalProblems(uses, known, data.lua.removed);
  if (problems.length === 0) return problems;
  if (project.lint.unknownGlobals === "error") throw new ProblemsError(problems);
  for (const problem of problems.slice(0, MAX_PROBLEMS)) ctx.logger.warn(formatProblem(problem));
  const more = problems.length - MAX_PROBLEMS;
  if (more > 0) ctx.logger.warn(`and ${more} more unknown global(s)`);
  return problems;
}
