import { assertEquals } from "@std/assert";
import { closestNames, editDistance, joinWords } from "../../src/shared/names.ts";

Deno.test("editDistance counts insertions, deletions and substitutions", () => {
  assertEquals(editDistance("kitten", "sitting"), 3);
  assertEquals(editDistance("", "abc"), 3);
  assertEquals(editDistance("same", "same"), 0);
});

Deno.test("joinWords joins with commas and a conjunction, and caps the list", () => {
  assertEquals(joinWords([], "or"), "");
  assertEquals(joinWords(["a"], "or"), "a");
  assertEquals(joinWords(["a", "b"], "or"), "a or b");
  assertEquals(joinWords(["a", "b", "c"], "and"), "a, b and c");
  assertEquals(joinWords(["a", "b", "c", "d"], "or", 2), "a, b and 2 more");
});

Deno.test("closestNames finds names a few edits away, ignoring case, nearest first", () => {
  const names = ["CreateUnit", "CreateItem", "print", "Player", "GetTriggerUnit", "Cos", "I2S"];
  assertEquals(closestNames(names, "CreatUnit"), ["CreateUnit"]);
  assertEquals(closestNames(names, "createunit"), ["CreateUnit"]);
  assertEquals(closestNames(names, "prnt"), ["print"]);
  assertEquals(closestNames(names, "GetTriggerUnt"), ["GetTriggerUnit"]);
  // A short name allows one edit, so "io" matches nothing here.
  assertEquals(closestNames(names, "io"), []);
  // The name itself is never suggested.
  assertEquals(closestNames(names, "print"), []);
  // At most three by default, ties broken by name.
  assertEquals(closestNames(["ae", "ad", "ac", "ab"], "aa"), ["ab", "ac", "ad"]);
  assertEquals(closestNames(["ae", "ad", "ac", "ab"], "aa", 1), ["ab"]);
});
