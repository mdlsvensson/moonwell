import { modelError, type ModelPath } from "./model-path.ts";

const PATH_SIZE = 260;
const TEXTURE_SIZE = 268; // uint32 replaceableId, char[260] path, uint32 flags
const FACE_EFFECT_SIZE = 340; // char[80] type, char[260] path
const NODE_FIXED_SIZE = 96; // uint32 size, char[80] name, int32 objectId, int32 parentId, uint32 flags
const NODE_FLAGS_OFFSET = 92;
const EMITTER_USES_MDL = 0x8000;
const EMITTER_USES_TGA = 0x10000;

const decoder = new TextDecoder();

export function isMdx(bytes: Uint8Array): boolean {
  return bytes.length >= 4 && bytes[0] === 0x4d && bytes[1] === 0x44 && bytes[2] === 0x4c && bytes[3] === 0x58;
}

/** Every file a binary MDX model references, in file order. Chunks without paths are skipped by their size. */
export function readMdxPaths(bytes: Uint8Array, file: string): ModelPath[] {
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  const fail = (problem: string): never => {
    throw modelError(file, problem);
  };
  const u32 = (offset: number) => view.getUint32(offset, true);
  const text = (offset: number, size: number): string => {
    const field = bytes.subarray(offset, offset + size);
    const end = field.indexOf(0);
    return decoder.decode(end < 0 ? field : field.subarray(0, end));
  };

  /** Reads the path of each size-prefixed node record in [start, end); the path sits `pathOffset` bytes after the node. */
  const nodeRecords = (
    tag: string,
    start: number,
    end: number,
    pathOffset: number,
    add: (flags: number, path: string) => void,
  ) => {
    let offset = start;
    while (offset < end) {
      if (offset + 4 > end) fail(`a ${tag} record is cut off`);
      const recordEnd = offset + u32(offset);
      if (recordEnd < offset + 4 + NODE_FIXED_SIZE || recordEnd > end) fail(`a ${tag} record has an invalid size`);
      const nodeSize = u32(offset + 4);
      if (nodeSize < NODE_FIXED_SIZE || offset + 4 + nodeSize > recordEnd) fail(`a ${tag} node has an invalid size`);
      const pathStart = offset + 4 + nodeSize + pathOffset;
      if (pathStart + PATH_SIZE > recordEnd) fail(`a ${tag} record is too small for its path`);
      add(u32(offset + 4 + NODE_FLAGS_OFFSET), text(pathStart, PATH_SIZE));
      offset = recordEnd;
    }
  };

  const paths: ModelPath[] = [];
  let offset = 4; // after the MDLX magic
  while (offset < bytes.length) {
    if (offset + 8 > bytes.length) fail("a chunk header is cut off");
    const tag = String.fromCharCode(...bytes.subarray(offset, offset + 4));
    const start = offset + 8;
    const end = start + u32(offset + 4);
    if (end > bytes.length) fail(`the ${tag} chunk runs past the end of the file`);
    switch (tag) {
      case "TEXS":
        if ((end - start) % TEXTURE_SIZE !== 0) fail("the TEXS chunk is not a whole number of textures");
        for (let at = start; at < end; at += TEXTURE_SIZE) {
          const path = text(at + 4, PATH_SIZE);
          paths.push({ kind: "texture", path: path === "" ? null : path, replaceableId: u32(at) });
        }
        break;
      case "FAFX":
        if ((end - start) % FACE_EFFECT_SIZE !== 0) fail("the FAFX chunk is not a whole number of face effects");
        for (let at = start; at < end; at += FACE_EFFECT_SIZE) {
          const path = text(at + 80, PATH_SIZE);
          if (path !== "") paths.push({ kind: "face effect", path, replaceableId: 0 });
        }
        break;
      case "PREM":
        nodeRecords("PREM", start, end, 16, (flags, path) => {
          if (path === "") return;
          const texture = (flags & EMITTER_USES_TGA) !== 0 && (flags & EMITTER_USES_MDL) === 0;
          paths.push({ kind: texture ? "particle texture" : "particle model", path, replaceableId: 0 });
        });
        break;
      case "ATCH":
        nodeRecords("ATCH", start, end, 0, (_flags, path) => {
          if (path !== "") paths.push({ kind: "attachment", path, replaceableId: 0 });
        });
        break;
      case "CORN":
        nodeRecords("CORN", start, end, 32, (_flags, path) => {
          if (path !== "") paths.push({ kind: "popcorn", path, replaceableId: 0 });
        });
        break;
    }
    offset = end;
  }
  return paths;
}
