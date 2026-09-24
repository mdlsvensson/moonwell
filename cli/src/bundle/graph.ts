import { MoonwellError } from "../shared/errors.ts";
import type { CompiledModule } from "../yue/compile.ts";
import { findRequires } from "./lexer.ts";

/** Walks literal requires from `entry`; returns reachable modules with dependencies before dependents. */
export function resolveGraph(
  entry: string,
  load: (name: string) => CompiledModule | undefined,
  builtins: ReadonlySet<string>,
): CompiledModule[] {
  const ordered: CompiledModule[] = [];
  const state = new Map<string, "visiting" | "done">();
  const stack: string[] = [];

  const visit = (name: string, from?: { module: CompiledModule; line: number }) => {
    if (builtins.has(name) || state.get(name) === "done") return;
    if (state.get(name) === "visiting") {
      const cycle = [...stack.slice(stack.indexOf(name)), name].join(" → ");
      throw new MoonwellError(`Circular require: ${cycle}`, {
        file: from?.module.sourcePath,
        line: from?.line,
        hint: "Move the shared code into a module that both can require.",
      });
    }
    const module = load(name);
    if (!module) {
      throw new MoonwellError(`Module '${name}' not found.`, {
        file: from?.module.sourcePath,
        line: from?.line,
        hint: `Expected src/${name.split(".").join("/")}.yue. Built-in modules: ${[...builtins].join(", ")}.`,
      });
    }
    state.set(name, "visiting");
    stack.push(name);
    for (const call of findRequires(module.source)) {
      if (call.name === undefined) {
        throw new MoonwellError("require must be called with a single string literal.", {
          file: module.sourcePath,
          line: call.line,
          hint: "Moonwell bundles modules at build time and cannot follow computed module names.",
        });
      }
      visit(call.name, { module, line: call.line });
    }
    stack.pop();
    state.set(name, "done");
    ordered.push(module);
  };

  visit(entry);
  return ordered;
}
