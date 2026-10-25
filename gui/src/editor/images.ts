export function imageMimeType(path: string): string | undefined {
  const extension = path.split(".").at(-1)?.toLowerCase();
  return ({ png: "image/png", jpg: "image/jpeg", jpeg: "image/jpeg", jfif: "image/jpeg", gif: "image/gif", webp: "image/webp", svg: "image/svg+xml", avif: "image/avif", bmp: "image/bmp", ico: "image/x-icon" } as Record<string, string>)[extension ?? ""];
}
export function isImageFile(path: string): boolean { return imageMimeType(path) !== undefined; }
