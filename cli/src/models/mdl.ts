import { modelError, type ModelPath } from "./model-path.ts";

interface Token {
  type: "string" | "word" | "{" | "}" | ",";
  value: string;
}

interface Block {
  name: string;
  strings: Map<string, string>;
  numbers: Map<string, number>;
  flags: Set<string>;
}

function tokenize(text: string, file: string): Token[] {
  const tokens: Token[] = [];
  let i = 0;
  while (i < text.length) {
    const char = text[i];
    if (/\s/.test(char)) {
      i++;
    } else if (char === "/" && text[i + 1] === "/") {
      const end = text.indexOf("\n", i);
      i = end < 0 ? text.length : end;
    } else if (char === "{" || char === "}" || char === ",") {
      tokens.push({ type: char, value: char });
      i++;
    } else if (char === '"') {
      const end = text.indexOf('"', i + 1);
      if (end < 0) throw modelError(file, "a string is never closed");
      tokens.push({ type: "string", value: text.slice(i + 1, end) });
      i = end + 1;
    } else {
      let end = i;
      while (end < text.length && !/[\s{},"]/.test(text[end])) end++;
      tokens.push({ type: "word", value: text.slice(i, end) });
      i = end;
    }
  }
  return tokens;
}

/** The reference a closed block makes, if it is a path-bearing block with a path. */
function blockPath(block: Block): ModelPath | undefined {
  const path = block.strings.get("Path") ?? "";
  switch (block.name) {
    case "Bitmap": {
      const image = block.strings.get("Image") ?? "";
      return {
        kind: "texture",
        path: image === "" ? null : image,
        replaceableId: block.numbers.get("ReplaceableId") ?? 0,
      };
    }
    case "ParticleEmitter": {
      const texture = block.flags.has("EmitterUsesTGA") && !block.flags.has("EmitterUsesMDL");
      return path === ""
        ? undefined
        : { kind: texture ? "particle texture" : "particle model", path, replaceableId: 0 };
    }
    case "Attachment":
      return path === "" ? undefined : { kind: "attachment", path, replaceableId: 0 };
    case "ParticleEmitterPopcorn":
      return path === "" ? undefined : { kind: "popcorn", path, replaceableId: 0 };
    case "FaceFX":
      return path === "" ? undefined : { kind: "face effect", path, replaceableId: 0 };
  }
  return undefined;
}

/** Every file a text MDL model references, in file order. */
export function readMdlPaths(text: string, file: string): ModelPath[] {
  const paths: ModelPath[] = [];
  const stack: Block[] = [];
  let statement: Token[] = [];
  const finishStatement = () => {
    const block = stack.at(-1);
    const [key, value] = statement;
    if (block !== undefined && key?.type === "word") {
      if (statement.length === 1) block.flags.add(key.value);
      else if (statement.length === 2 && value.type === "string") block.strings.set(key.value, value.value);
      else if (statement.length === 2 && /^-?\d+$/.test(value.value)) block.numbers.set(key.value, Number(value.value));
    }
    statement = [];
  };
  for (const token of tokenize(text, file)) {
    if (token.type === "{") {
      const name = statement[0]?.type === "word" ? statement[0].value : "";
      statement = [];
      stack.push({ name, strings: new Map(), numbers: new Map(), flags: new Set() });
    } else if (token.type === "}") {
      finishStatement();
      const block = stack.pop();
      if (block === undefined) throw modelError(file, "a } has no matching {");
      const ref = blockPath(block);
      if (ref !== undefined) paths.push(ref);
    } else if (token.type === ",") {
      finishStatement();
    } else {
      statement.push(token);
    }
  }
  if (stack.length > 0) throw modelError(file, `the ${stack.at(-1)!.name || "unnamed"} block is never closed`);
  return paths;
}
