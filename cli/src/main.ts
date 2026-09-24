import { parseArgs } from "@std/cli/parse-args";
import { join } from "@std/path";
import { build } from "./commands/build.ts";
import { check } from "./commands/check.ts";
import { setup } from "./commands/setup.ts";
import { test } from "./commands/test.ts";
import { createContext } from "./context.ts";
import { formatError } from "./shared/errors.ts";
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
  const logger = createLogger({ write, file: command === "init" ? undefined : join(root, "dist", "moonwell.log") });
  const ctx = createContext(root, logger);
  const stage = { entry: flags.entry, minify: flags.minify ? true : undefined };
  try {
    switch (command) {
      case "setup":
        await setup(ctx);
        break;
      case "build":
        await build(ctx, stage);
        break;
      case "test":
        await test(ctx, stage);
        break;
      case "check":
        await check(ctx);
        break;
      default:
        write(`Unknown command '${command}'.\n\n${USAGE}`);
        return 1;
    }
    return 0;
  } catch (error) {
    logger.error(formatError(error));
    return 1;
  }
}

if (import.meta.main) Deno.exit(await main(Deno.args));
