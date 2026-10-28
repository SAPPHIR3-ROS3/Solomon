import { nativeBindings } from "../platform";

export function clipboardImageFiles(data: DataTransfer): File[] {
  const items = Array.from(data.items ?? [])
    .filter((item) => item.kind === "file")
    .flatMap((item) => item.getAsFile() ?? [])
    .flatMap(imageFile);
  return items.length ? items : Array.from(data.files ?? []).flatMap(imageFile);
}

function imageFile(file: File): File[] {
  if (file.type.startsWith("image/")) return [file];
  if (file.type) return [];
  // Some desktop file managers omit MIME types for dragged files.
  const mime: Record<string, string> = { png: "image/png", jpg: "image/jpeg", jpeg: "image/jpeg", webp: "image/webp" };
  const type = mime[file.name.toLowerCase().split(".").pop() ?? ""];
  return type ? [new File([file], file.name, { type, lastModified: file.lastModified })] : [];
}

// While dragging, browsers protect file bytes; only item types may be visible.
export function isImageFileDrag(data: DataTransfer): boolean {
  const items = Array.from(data.items ?? []);
  if (items.length) return items.some((item) => item.kind === "file" && (!item.type || item.type.startsWith("image/")));
  const files = Array.from(data.files ?? []);
  if (files.length) return files.some((file) => file.type.startsWith("image/"));
  return Array.from(data.types ?? []).includes("Files");
}

export function nativeImageDropPaths(data: DataTransfer): string[] {
  const uris = data.getData("text/uri-list");
  const text = uris || data.getData("text/plain");
  const lines = text.split(/\r?\n/).map((line) => line.trim()).filter((line) => line && !line.startsWith("#"));
  return lines.flatMap((line) => {
    // URI lists preserve spaces and percent-escaped characters in filenames.
    if (line.startsWith("file:")) {
      try {
        const url = new URL(line);
        if (url.protocol !== "file:" || (url.hostname && url.hostname !== "localhost")) return [];
        line = decodeURIComponent(url.pathname);
        if (/^\/[a-z]:\//i.test(line)) line = line.slice(1);
      } catch { return []; }
    } else if (uris) return [];
    else if ((line.startsWith('"') && line.endsWith('"')) || (line.startsWith("'") && line.endsWith("'"))) line = line.slice(1, -1);
    return /^(\/|[a-z]:[\\/])/i.test(line) && /\.(png|jpe?g|webp|gif)$/i.test(line) ? [line] : [];
  });
}

export function supportsNativeImageDrop(): boolean {
  return Boolean(nativeBindings()?.ServerBridge?.ReadDroppedImages);
}

export async function readNativeDroppedImages(paths: string[]): Promise<File[]> {
  const read = nativeBindings()?.ServerBridge?.ReadDroppedImages;
  if (!read || !paths.length) return [];
  const images = await read(paths);
  return Promise.all(images.map(async (image) => {
    const blob = await (await fetch(image.data)).blob();
    return new File([blob], image.name, { type: blob.type });
  }));
}

export async function readNativeClipboardImage(): Promise<File[]> {
  const read = nativeBindings()?.ServerBridge?.ClipboardImage;
  if (!read) return [];
  const url = await read();
  if (!url) return [];
  const blob = await (await fetch(url)).blob();
  const extension = blob.type.split("/")[1].replace("jpeg", "jpg");
  return [new File([blob], `clipboard.${extension}`, { type: blob.type })];
}
