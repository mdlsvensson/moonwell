import { decodeBase64 } from "@std/encoding/base64";
import { exists } from "@std/fs";
import { dirname, fromFileUrl, join, relative, resolve } from "@std/path";
import type { CommandContext } from "../context.ts";
import { TEMPLATE_FILES } from "../embedded/template.ts";
import { projectDenoJson, projectPklProject } from "../project-files.ts";
import { MoonwellError } from "../shared/errors.ts";
import { toPosix } from "../shared/fs.ts";
import { VERSION } from "../version.ts";

/** Scaffolds a project into `dir` (new or empty) and resolves its Pkl dependencies. */
export async function init(dir: string, ctx: CommandContext, options: { link?: boolean } = {}): Promise<string> {
  const target = resolve(ctx.root, dir);
  if (await exists(target)) {
    for await (const _ of Deno.readDir(target)) {
      throw new MoonwellError(`${dir} is not empty.`, { hint: "Choose a new or empty directory." });
    }
  }
  const links = options.link ? localLinks(target) : undefined;

  for (const file of TEMPLATE_FILES) {
    const path = join(target, ...file.path.split("/"));
    await Deno.mkdir(dirname(path), { recursive: true });
    await Deno.writeFile(path, decodeBase64(file.base64));
  }
  await Deno.writeTextFile(join(target, "deno.json"), projectDenoJson(links?.cli ?? `jsr:@moonwell/cli@${VERSION}`));
  await Deno.writeTextFile(
    join(target, "PklProject"),
    projectPklProject(links ? { local: links.pkl } : { version: VERSION }),
  );

  const result = await ctx.run("pkl", ["project", "resolve"], {
    cwd: target,
    notFoundHint: "Install Pkl 0.32 or newer: https://pkl-lang.org/main/current/pkl-cli/index.html#installation",
  });
  if (result.code !== 0) {
    throw new MoonwellError(`pkl project resolve failed:\n${(result.stderr || result.stdout).trim()}`, {
      file: join(dir, "PklProject"),
    });
  }
  ctx.logger.info(`Created ${dir}. Next: cd ${dir} && deno task build`);
  return target;
}

/** Paths from `target` to this checkout's CLI entry and Pkl package; only valid when running from files. */
function localLinks(target: string): { cli: string; pkl: string } {
  if (!import.meta.url.startsWith("file:")) {
    throw new MoonwellError("--link only works when Moonwell runs from a local checkout.");
  }
  const repo = resolve(dirname(fromFileUrl(import.meta.url)), "..", "..", "..");
  const link = (path: string) => toPosix(relative(target, path));
  return { cli: link(join(repo, "cli", "src", "main.ts")), pkl: link(join(repo, "pkl")) };
}
