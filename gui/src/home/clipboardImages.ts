import type {} from "../platform";

export function clipboardImageFiles(data: DataTransfer): File[] {
  const items = Array.from(data.items ?? [])
    .filter((item) => item.kind === "file" && item.type.startsWith("image/"))
    .flatMap((item) => item.getAsFile() ?? []);
  return items.length ? items : Array.from(data.files ?? []).filter((file) => file.type.startsWith("image/"));
}

export async function readNativeClipboardImage(): Promise<File[]> {
  const read = window.go?.main?.ServerBridge?.ClipboardImage;
  if (!read) return [];
  const url = await read();
  if (!url) return [];
  const blob = await (await fetch(url)).blob();
  const extension = blob.type.split("/")[1].replace("jpeg", "jpg");
  return [new File([blob], `clipboard.${extension}`, { type: blob.type })];
}
