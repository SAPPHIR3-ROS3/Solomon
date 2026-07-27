export type ThreadFolder = {
  id: string;
  name: string;
  path: string;
  threadCount: number;
};

export type ThreadSidebarData = {
  folders: ThreadFolder[];
  userName: string;
};

export async function fetchThreadSidebarData(signal?: AbortSignal): Promise<ThreadSidebarData> {
  const response = await fetch("/__solomon/thread-folders", { signal });
  if (!response.ok) throw new Error(`Unable to load thread folders: ${response.status}`);
  const payload: unknown = await response.json();
  if (!payload || typeof payload !== "object" || !("folders" in payload) || !Array.isArray(payload.folders)) {
    return { folders: [], userName: "" };
  }
  return {
    folders: payload.folders.filter(isThreadFolder),
    userName: "userName" in payload && typeof payload.userName === "string" ? payload.userName : "",
  };
}

function isThreadFolder(value: unknown): value is ThreadFolder {
  return Boolean(
    value
      && typeof value === "object"
      && "id" in value && typeof value.id === "string"
      && "name" in value && typeof value.name === "string"
      && "path" in value && typeof value.path === "string"
      && "threadCount" in value && typeof value.threadCount === "number",
  );
}
