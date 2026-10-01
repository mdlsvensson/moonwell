import { assetPath, lstatOrUndefined, pathKey, safeJoin } from "../assets/paths.ts";
import { MoonwellError } from "../shared/errors.ts";
import { type PreviewPicture, readPreviewPicture } from "./picture.ts";

/**
 * Reads the picture that `settings.info.preview` names: `preview` is a path from the project folder `root`. Errors
 * about the setting name `manifestFile`; errors about the picture name its path.
 */
export async function loadPreviewPicture(
  root: string,
  preview: string,
  manifestFile?: string,
): Promise<PreviewPicture> {
  let path: string;
  try {
    path = assetPath(preview);
  } catch (cause) {
    throw new MoonwellError(`settings.info.preview must be a path inside the project, not "${preview}".`, {
      file: manifestFile,
      cause,
      hint: 'Name a picture in the project folder, such as "preview.tga" beside moonwell.pkl.',
    });
  }
  if (pathKey(path).startsWith("assets/")) {
    throw new MoonwellError(`settings.info.preview names a file under assets/: ${path}`, {
      file: manifestFile,
      hint: "Keep the picture outside assets/, for example beside moonwell.pkl: every file under assets/ is also " +
        "imported into the map under its own name.",
    });
  }
  const file = await safeJoin(root, path);
  const info = await lstatOrUndefined(file);
  if (!info?.isFile) {
    throw new MoonwellError(
      info === undefined
        ? `settings.info.preview names a file that does not exist: ${path}`
        : `settings.info.preview does not name a file: ${path}`,
      { file: manifestFile, hint: "The path starts at the project folder, where moonwell.pkl is." },
    );
  }
  let bytes: Uint8Array;
  try {
    bytes = await Deno.readFile(file);
  } catch (cause) {
    const reason = cause instanceof Error ? cause.message : String(cause);
    throw new MoonwellError(`Reading the preview picture failed: ${reason}`, {
      file: path,
      cause,
      hint: "Make sure no other program has the picture locked.",
    });
  }
  return readPreviewPicture(bytes, path);
}
