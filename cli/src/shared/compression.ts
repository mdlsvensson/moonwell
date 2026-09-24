async function transform(data: Uint8Array, stream: { readable: ReadableStream<Uint8Array>; writable: WritableStream<Uint8Array> }): Promise<Uint8Array> {
  const output = new Blob([data.slice()]).stream().pipeThrough(stream as unknown as TransformStream<Uint8Array, Uint8Array>);
  return new Uint8Array(await new Response(output).arrayBuffer());
}

/** zlib-wrapped deflate (what MPQ compression type 0x02 expects). */
export const deflate = (data: Uint8Array) => transform(data, new CompressionStream("deflate") as unknown as { readable: ReadableStream<Uint8Array>; writable: WritableStream<Uint8Array> });
export const inflate = (data: Uint8Array) => transform(data, new DecompressionStream("deflate") as unknown as { readable: ReadableStream<Uint8Array>; writable: WritableStream<Uint8Array> });
/** Raw deflate (what zip method 8 uses). */
export const deflateRaw = (data: Uint8Array) => transform(data, new CompressionStream("deflate-raw") as unknown as { readable: ReadableStream<Uint8Array>; writable: WritableStream<Uint8Array> });
export const inflateRaw = (data: Uint8Array) => transform(data, new DecompressionStream("deflate-raw") as unknown as { readable: ReadableStream<Uint8Array>; writable: WritableStream<Uint8Array> });
