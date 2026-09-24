import type { Logger } from "../../src/shared/log.ts";

/** A logger that records lines instead of printing them. */
export function silentLogger(): Logger & { lines: string[] } {
  const lines: string[] = [];
  return {
    lines,
    info: (message) => lines.push(message),
    warn: (message) => lines.push(`warning: ${message}`),
    error: (message) => lines.push(message),
  };
}
