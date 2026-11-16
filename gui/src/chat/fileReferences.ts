export const ADD_FILE_TO_CHAT_EVENT = "solomon:add-file-to-chat";
export type FileChatReference = { projectID: string; tag: string };
export function addFileToChat(reference: FileChatReference) {
  window.dispatchEvent(new CustomEvent<FileChatReference>(ADD_FILE_TO_CHAT_EVENT, { detail: reference }));
}

export const WORKSPACE_FILE_REFERENCE = "application/x-solomon-file-reference";

export function readWorkspaceFileReference(value: string, projectID?: string): { projectID: string; path: string } | null {
  try {
    const reference = JSON.parse(value);
    if (!projectID || reference?.projectID !== projectID || typeof reference.path !== "string" || !reference.path) return null;
    return { projectID, path: reference.path };
  } catch {
    return null;
  }
}
