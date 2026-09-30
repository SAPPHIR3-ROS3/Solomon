export const ADD_FILE_TO_CHAT_EVENT = "solomon:add-file-to-chat";
export type FileChatReference = { projectID: string; tag: string };
export function addFileToChat(reference: FileChatReference) {
  window.dispatchEvent(new CustomEvent<FileChatReference>(ADD_FILE_TO_CHAT_EVENT, { detail: reference }));
}
