export function normalizeTemporaryWorkspacePath(value: string) {
  return value
    .trim()
    .replaceAll("\\", "/")
    .replace(/^~\/?/, "")
    .replace(/^\/+|\/+$/g, "");
}

