import { assertEquals, assertRejects } from "@std/assert";
import { join } from "@std/path";
import { generateGamePaths } from "../../../tools/gen-game-paths.ts";

Deno.test("generateGamePaths writes the list and returns the path count", async () => {
  const dir = await Deno.makeTempDir();
  try {
    const target = join(dir, "game-paths.txt");
    const count = await generateGamePaths("war3.w3mod:Units/Human/Footman/Footman.mdx\n", "2.0.0", target);
    assertEquals(count, 1);
    assertEquals(await Deno.readTextFile(target), "# Warcraft III 2.0.0\nunits/human/footman/footman.mdx\n");
  } finally {
    await Deno.remove(dir, { recursive: true });
  }
});

Deno.test("generateGamePaths throws and keeps the existing list when no path is recognized", async () => {
  const dir = await Deno.makeTempDir();
  try {
    const target = join(dir, "game-paths.txt");
    await Deno.writeTextFile(target, "# Warcraft III 1.0.0\nunits/old.mdx\n");
    await assertRejects(() => generateGamePaths("war3.w3mod:Sound/Hit.wav\n", "2.0.0", target), Error);
    assertEquals(await Deno.readTextFile(target), "# Warcraft III 1.0.0\nunits/old.mdx\n");
  } finally {
    await Deno.remove(dir, { recursive: true });
  }
});
