import { decodeBase64 } from "@std/encoding/base64";
import { exists } from "@std/fs";
import { dirname, fromFileUrl, join, relative, resolve } from "@std/path";
import type { CommandContext } from "../context.ts";
import { TEMPLATE_FILES } from "../embedded/template.ts";
import { projectDenoJson, projectPklProject } from "../project-files.ts";
import { checkPkl, PKL_INSTALL_HINT } from "../project/project.ts";
import { MoonwellError } from "../shared/errors.ts";
import { removeIfExists, toPosix } from "../shared/fs.ts";
import { VERSION } from "../version.ts";

/** Scaffolds a project into `dir` (new or empty) and resolves its Pkl dependencies. A failed init leaves nothing. */
export async function init(dir: string, ctx: CommandContext, options: { link?: boolean } = {}): Promise<string> {
  const target = resolve(ctx.root, dir);
  const existed = await exists(target);
  if (existed) {
    if (!(await exists(target, { isDirectory: true }))) {
      throw new MoonwellError(`${dir} is not a directory.`, { hint: "Choose a new or empty directory." });
    }
    for await (const _ of Deno.readDir(target)) {
      throw new MoonwellError(`${dir} is not empty.`, { hint: "Choose a new or empty directory." });
    }
  }
  const links = options.link ? localLinks(target) : undefined;
  await checkPkl(ctx.run);

  try {
    await writeProject(target, links);
    const result = await ctx.run("pkl", ["project", "resolve"], { cwd: target, hint: PKL_INSTALL_HINT });
    if (result.code !== 0) {
      throw new MoonwellError(`pkl project resolve failed:\n${(result.stderr || result.stdout).trim()}`, {
        file: join(dir, "PklProject"),
      });
    }
  } catch (error) {
    await undoInit(target, existed);
    throw error;
  }
  ctx.logger.info(`Created ${dir}. Next: cd ${dir} && deno task build`);
  return target;
}

async function writeProject(target: string, links: { cli: string; pkl: string } | undefined): Promise<void> {
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
}

/** Removes what init wrote: the whole directory if init created it, else only its contents (it was empty). */
async function undoInit(target: string, existed: boolean): Promise<void> {
  try {
    if (!existed) {
      await removeIfExists(target);
      return;
    }
    for await (const entry of Deno.readDir(target)) await removeIfExists(join(target, entry.name));
  } catch {
    // Best effort: the original error matters more.
  }
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
