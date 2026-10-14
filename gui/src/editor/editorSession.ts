import { saveProjectFile } from "../projects/projects";

export type OpenFile = { id: string; path: string; content: string; saved: string };
export type EditorSession = { files: OpenFile[]; activeTabId: string; expanded: string[]; rootCollapsed: boolean };
const sessions = new Map<string, EditorSession>();
const saves = new Map<string, { content: string; promise: Promise<void> }>();
const storageKey = (projectID: string) => `solomon.editor-session.v1.${projectID}`;
const emptySession = (): EditorSession => ({ files: [], activeTabId: "", expanded: [], rootCollapsed: false });

export function loadEditorSession(projectID?: string): EditorSession {
  if (!projectID) return emptySession();
  const cached = sessions.get(projectID);
  if (cached) return cached;
  try {
    const value = JSON.parse(window.localStorage.getItem(storageKey(projectID)) ?? "null");
    if (!value || typeof value !== "object") return emptySession();
    const files = Array.isArray(value.files) ? value.files.filter((file: OpenFile) => file && typeof file.id === "string" && typeof file.path === "string" && typeof file.content === "string" && typeof file.saved === "string") : [];
    return { files, activeTabId: typeof value.activeTabId === "string" ? value.activeTabId : "", expanded: Array.isArray(value.expanded) ? value.expanded.filter((path: unknown) => typeof path === "string") : [], rootCollapsed: value.rootCollapsed === true };
  } catch { return emptySession(); }
}

export function storeEditorSession(projectID: string, session: EditorSession) {
  sessions.set(projectID, session);
  try { window.localStorage.setItem(storageKey(projectID), JSON.stringify(session)); } catch { /* Keep the in-memory draft if storage is full or unavailable. */ }
}

// Serial writes survive view unmounts, so an older request cannot overwrite
// newer text. Identical pending writes share the same request.
export function saveEditorFile(projectID: string, path: string, content: string): Promise<void> {
  const key = JSON.stringify([projectID, path]);
  const previous = saves.get(key);
  if (previous?.content === content) return previous.promise;
  const promise = (previous?.promise.catch(() => undefined) ?? Promise.resolve()).then(async () => {
    await saveProjectFile(projectID, path, content);
    const session = loadEditorSession(projectID);
    storeEditorSession(projectID, { ...session, files: session.files.map((file) => file.path === path ? { ...file, saved: content } : file) });
  });
  saves.set(key, { content, promise });
  const cleanup = () => { if (saves.get(key)?.promise === promise) saves.delete(key); };
  void promise.then(cleanup, cleanup);
  return promise;
}

export async function waitForEditorSaves(projectID: string) {
  const prefix = JSON.stringify([projectID]).slice(0, -1) + ",";
  await Promise.all([...saves].filter(([key]) => key.startsWith(prefix)).map(([, save]) => save.promise.catch(() => undefined)));
}
