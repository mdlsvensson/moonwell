/** Tasks every project's deno.json exposes. Plan 2 adds the data-layer commands. */
export const PROJECT_TASKS = ["build", "test", "dev", "check", "setup"] as const;

export const PACKAGE_BASE_URI = "package://pkg.pkl-lang.org/github.com/mdlsvensson/moonwell/moonwell";

/** deno.json for a map project; `cli` is a JSR specifier or a path to cli/src/main.ts. */
export function projectDenoJson(cli: string): string {
  const tasks = Object.fromEntries(PROJECT_TASKS.map((task) => [task, `deno run -A ${cli} ${task}`]));
  return `${JSON.stringify({ tasks }, null, 2)}\n`;
}

/** PklProject for a map project, depending on a published or a local moonwell package. */
export function projectPklProject(dependency: { version: string } | { local: string }): string {
  const line = "local" in dependency
    ? `  ["moonwell"] = import("${dependency.local}/PklProject")`
    : `  ["moonwell"] { uri = "${PACKAGE_BASE_URI}@${dependency.version}" }`;
  return `amends "pkl:Project"\n\ndependencies {\n${line}\n}\n`;
}

/** Default Warcraft III executable of a Battle.net install on Windows. */
export const DEFAULT_GAME_EXECUTABLE = String.raw`C:\Program Files (x86)\Warcraft III\_retail_\x86_64\Warcraft III.exe`;

/** moonwell.local.pkl as init and setup create it: this machine's settings, amending the shared manifest. */
export function projectLocalPkl(): string {
  const executable = DEFAULT_GAME_EXECUTABLE.replaceAll("\\", "\\\\");
  return [
    "// Settings for this machine only. Git-ignored, so each checkout has its own; `deno task setup` recreates it.",
    "// It amends moonwell.pkl, so anything set here overrides the shared manifest.",
    "",
    'amends "moonwell.pkl"',
    "",
    "launch {",
    `  gameExecutable = "${executable}"  // your Warcraft III.exe`,
    "}",
    "",
  ].join("\n");
}

/** Template files init generates itself instead of copying. */
export const TEMPLATE_EXCLUDE = ["deno.json", "PklProject", "PklProject.deps.json", "moonwell.local.pkl"] as const;
