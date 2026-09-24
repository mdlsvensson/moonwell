import type { Logger } from "./shared/log.ts";
import { type Runner, runProcess } from "./shared/process.ts";
import { defaultInstallDeps, type InstallDeps } from "./yue/install.ts";
import { type Spawn, spawnDetached } from "./launch.ts";

/** Everything a command needs from the outside world; tests substitute parts of it. */
export interface CommandContext {
  root: string;
  logger: Logger;
  run: Runner;
  install: InstallDeps;
  spawn: Spawn;
}

export function createContext(root: string, logger: Logger): CommandContext {
  return { root, logger, run: runProcess, install: defaultInstallDeps(logger), spawn: spawnDetached };
}
