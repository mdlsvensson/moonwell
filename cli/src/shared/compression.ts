type Duplex = { readable: ReadableStream<Uint8Array>; writable: WritableStream<Uint8Array> };

async function transform(data: Uint8Array, stream: Duplex): Promise<Uint8Array> {
  const output = new Blob([data.slice()]).stream().pipeThrough(
    stream as unknown as TransformStream<Uint8Array, Uint8Array>,
  );
  return new Uint8Array(await new Response(output).arrayBuffer());
}

/** zlib-wrapped deflate (what MPQ compression type 0x02 expects). */
export const deflate = (data: Uint8Array) => transform(data, new CompressionStream("deflate") as unknown as Duplex);
export const inflate = (data: Uint8Array) => transform(data, new DecompressionStream("deflate") as unknown as Duplex);
/** Raw deflate (what zip method 8 uses). */
export const deflateRaw = (data: Uint8Array) =>
  transform(data, new CompressionStream("deflate-raw") as unknown as Duplex);
export const inflateRaw = (data: Uint8Array) =>
  transform(data, new DecompressionStream("deflate-raw") as unknown as Duplex);
/** gzip (the embedded in-game path list). */
export const gzip = (data: Uint8Array) => transform(data, new CompressionStream("gzip") as unknown as Duplex);
export const gunzip = (data: Uint8Array) => transform(data, new DecompressionStream("gzip") as unknown as Duplex);
