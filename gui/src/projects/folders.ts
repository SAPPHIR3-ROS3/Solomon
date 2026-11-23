import { readViewState, writeViewState } from "./viewState";

type FolderPanel = "chats" | "chat-files" | "editor";
type FolderState = { expanded: string[]; rootCollapsed: boolean };

export const chatFolderOverrides = new Map<string, boolean>();

function route(projectID: string, panel: FolderPanel) {
  return `/__solomon/projects/${encodeURIComponent(projectID)}/folder-state?panel=${panel}`;
}

export async function loadFolderState(projectID: string, panel: FolderPanel, fallback: FolderState): Promise<FolderState> {
  try {
    const stored = await readViewState<FolderState>(route(projectID, panel));
    if (stored) return { expanded: Array.isArray(stored.expanded) ? stored.expanded : [], rootCollapsed: stored.rootCollapsed === true };
    if (fallback.expanded.length || fallback.rootCollapsed) await saveFolderState(projectID, panel, fallback);
  } catch {
    return fallback;
  }
  return fallback;
}

export function saveFolderState(projectID: string, panel: FolderPanel, state: FolderState): Promise<void> {
  return writeViewState(route(projectID, panel), state);
}
