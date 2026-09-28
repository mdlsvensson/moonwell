import { dirname, join } from "@std/path";
import { collectModules, libraryModuleRoots, PROJECT_MODULE_ROOTS } from "../bundle/modules.ts";
import type { CommandContext } from "../context.ts";
import { refreshLibraryView } from "../editor/library-view.ts";
import { refreshEditorFiles } from "../editor/refresh.ts";
import { addEditorFiles, luarcTemplateEntries, mergeLuarc } from "../editor/scaffold.ts";
import { planProjectObjects, syncProjectLibraries } from "../pipeline.ts";
import { ensureLocalManifest, loadProject } from "../project/project.ts";
import { installYueBin, reportEditorTools } from "../yue/bin.ts";
import { ensureYue } from "../yue/install.ts";

/**
 * Creates moonwell.local.pkl if this checkout has none, installs the project's pinned compiler into the user cache,
 * keeps a copy in the cache's bin folder for the editor and reports what the editor still needs, adds the editor files
 * a project lacks, adds the `.luarc.json` entries an older project lacks, syncs the manifest's libraries into
 * .moonwell/libraries (and moonwell.lock) and their Lua modules into .moonwell/lua, and writes .moonwell/types and the
 * macro module (.moonwell/yue/moonwell/macros.yue).
 */
export async function setup(ctx: CommandContext): Promise<string> {
  const project = await loadProject(ctx.root, ctx.run);
  if (await ensureLocalManifest(ctx.root)) {
    ctx.logger.info("Created moonwell.local.pkl. Check that launch.gameExecutable points at your Warcraft III.exe.");
  }
  const binary = await ensureYue(project.yue, ctx.install);
  ctx.logger.info(`YueScript ${project.yue.version}: ${binary}`);
  if (project.yue.path === null) {
    const { path, copied } = await installYueBin(binary, ctx.install.cacheRoot);
    if (copied) ctx.logger.info(`Copied YueScript for the editor to ${path}.`);
  }
  await reportEditorTools(ctx.run, ctx.logger, {
    version: project.yue.version,
    binDir: project.yue.path === null ? join(ctx.install.cacheRoot, "bin") : dirname(project.yue.path),
    os: Deno.build.os,
  });
  for (const added of await addEditorFiles(ctx.root)) ctx.logger.info(`Added ${added} for the editor.`);
  const merged = await mergeLuarc(ctx.root);
  if (merged === undefined) {
    const entries = luarcTemplateEntries();
    const paths = entries["runtime.path"].join(", ");
    const libraries = entries["workspace.library"].join(", ");
    ctx.logger.warn(
      ".luarc.json is not plain JSON, so setup left it alone. Make sure its runtime.path has " +
        `${paths} and its workspace.library has ${libraries}.`,
    );
  } else if (merged.length > 0) {
    ctx.logger.info(`Added ${merged.join(", ")} to .luarc.json.`);
  }
  await syncProjectLibraries(ctx, project);
  // Setup does not compile, so the view gets the libraries' Lua modules only.
  await refreshLibraryView(
    ctx.root,
    await collectModules(ctx.root, [...PROJECT_MODULE_ROOTS, ...libraryModuleRoots(Object.keys(project.libraries))]),
  );
  const objects = await planProjectObjects(ctx, project);
  await refreshEditorFiles(ctx.root, { objects: objects.objects, mapFolder: `maps/${project.map.folder}` });
  return binary;
}
