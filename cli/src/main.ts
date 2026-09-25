import { parseArgs } from "@std/cli/parse-args";
import { existsSync } from "@std/fs";
import { join } from "@std/path";
import { assetsPaths } from "./commands/assets-paths.ts";
import { assets } from "./commands/assets.ts";
import { build } from "./commands/build.ts";
import { check } from "./commands/check.ts";
import { dev } from "./commands/dev.ts";
import { init } from "./commands/init.ts";
import { setup } from "./commands/setup.ts";
import { test } from "./commands/test.ts";
import { createContext } from "./context.ts";
import { formatError, MoonwellError } from "./shared/errors.ts";
import { releaseHeldLocks } from "./shared/lock.ts";
import { createLogger } from "./shared/log.ts";
import { VERSION } from "./version.ts";

const USAGE = `Moonwell ${VERSION}: Warcraft III maps with YueScript gameplay and Pkl data

Usage: moonwell <command> [options]

Commands:
  init <dir> [--link]            Create a project (--link: use this local Moonwell checkout)
  setup                          Install the pinned YueScript compiler
  build [--entry f] [--minify]   Build <build.folder>/<map.folder>
  test [--entry f]               Stage the map and launch Warcraft III
  dev                            Watch sources and report errors on save
  check                          Compile and validate without building a map
  assets:check                   Show what assets:sync would change in the source map
  assets:sync                    Write assets/ into the source map (close it in World Editor first)
  assets:paths [file]            List the files a model references, as in-game or custom paths

Options:
  -h, --help                     Show this help
  -v, --version                  Show the version`;

export async function main(
  args: string[],
  root: string = Deno.cwd(),
  write: (line: string) => void = (line) => console.error(line),
): Promise<number> {
  const flags = parseArgs(args, {
    string: ["entry"],
    boolean: ["minify", "link", "help", "version"],
    alias: { h: "help", v: "version" },
  });
  const [command] = flags._.map(String);
  if (flags.version) {
    write(VERSION);
    return 0;
  }
  if (flags.help || command === undefined) {
    write(USAGE);
    return 0;
  }
  // Only a project gets dist/moonwell.log; running elsewhere must not create dist/.
  const inProject = command !== "init" && existsSync(join(root, "moonwell.pkl"));
  const logger = createLogger({ write, file: inProject ? join(root, "dist", "moonwell.log") : undefined });
  const ctx = createContext(root, logger);
  const stage = { entry: flags.entry, minify: flags.minify ? true : undefined };
  // Ctrl+C: dev stops watching and returns once its running check has released the build lock; other commands
  // (and a second Ctrl+C in dev) remove the lock this process holds and exit at once.
  const interrupt = new AbortController();
  const onSigint = () => {
    if (command === "dev" && !interrupt.signal.aborted) {
      interrupt.abort();
      return;
    }
    releaseHeldLocks();
    Deno.exit(130);
  };
  const handlesSigint = ["build", "test", "check", "dev", "assets:check", "assets:sync"].includes(command);
  if (handlesSigint) Deno.addSignalListener("SIGINT", onSigint);
  try {
    switch (command) {
      case "init": {
        const dir = flags._[1] === undefined ? undefined : String(flags._[1]);
        if (dir === undefined) throw new MoonwellError("init needs a directory.", { hint: "moonwell init my-map" });
        await init(dir, ctx, { link: flags.link });
        break;
      }
      case "setup":
        await setup(ctx);
        break;
      case "build":
        await build(ctx, stage);
        break;
      case "test":
        await test(ctx, stage);
        break;
      case "dev":
        await dev(ctx, { signal: interrupt.signal });
        if (interrupt.signal.aborted) return 130;
        break;
      case "check":
        await check(ctx);
        break;
      case "assets:check":
        await assets(ctx, "check");
        break;
      case "assets:sync":
        await assets(ctx, "sync");
        break;
      case "assets:paths":
        await assetsPaths(ctx, flags._[1] === undefined ? undefined : String(flags._[1]));
        break;
      default:
        write(`Unknown command '${command}'.\n\n${USAGE}`);
        return 1;
    }
    return 0;
  } catch (error) {
    logger.error(formatError(error));
    return 1;
  } finally {
    if (handlesSigint) Deno.removeSignalListener("SIGINT", onSigint);
  }
}

if (import.meta.main) Deno.exit(await main(Deno.args));
