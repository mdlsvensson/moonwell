/** Levenshtein distance between `a` and `b`. */
export function editDistance(a: string, b: string): number {
  let previous = Array.from({ length: b.length + 1 }, (_, j) => j);
  for (let i = 1; i <= a.length; i++) {
    const current = [i];
    for (let j = 1; j <= b.length; j++) {
      current[j] = Math.min(previous[j] + 1, current[j - 1] + 1, previous[j - 1] + (a[i - 1] === b[j - 1] ? 0 : 1));
    }
    previous = current;
  }
  return previous[b.length];
}

/** `a`, `a or b`, `a, b or c`; with `max`, the rest as `and N more`. */
export function joinWords(words: string[], conjunction: "and" | "or", max = words.length): string {
  const shown = words.slice(0, max);
  if (words.length > max) return `${shown.join(", ")} and ${words.length - max} more`;
  return shown.length < 2 ? shown.join("") : `${shown.slice(0, -1).join(", ")} ${conjunction} ${shown.at(-1)}`;
}

/**
 * Up to `max` of `names` closest to `key`, ignoring letter case: at most a quarter of its length in edits (at least
 * one), nearest first, then by name. `key` itself is never returned. Tighter than object data's rule, because it
 * searches thousands of global names.
 */
export function closestNames(names: Iterable<string>, key: string, max = 3): string[] {
  const wanted = key.toLowerCase();
  const limit = Math.max(1, Math.floor(wanted.length / 4));
  const matches: Array<{ name: string; distance: number }> = [];
  for (const name of names) {
    if (name === key || Math.abs(name.length - wanted.length) > limit) continue;
    const distance = editDistance(wanted, name.toLowerCase());
    if (distance <= limit) matches.push({ name, distance });
  }
  return matches
    .sort((a, b) => a.distance - b.distance || (a.name < b.name ? -1 : a.name > b.name ? 1 : 0))
    .slice(0, max)
    .map(({ name }) => name);
}
