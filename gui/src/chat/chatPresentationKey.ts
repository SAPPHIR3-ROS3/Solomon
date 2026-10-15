import type { ChatMessage } from "./chatTypes";

// A streamed message has a temporary ID; its saved snapshot has a server ID.
// The checkpoint identifies the same response throughout that transition.
// Keep this identity local to presentation: actions still use the server ID.
export function chatPresentationKey(message: ChatMessage): string {
  return message.checkpointSeq !== undefined
    ? `${message.role}:checkpoint:${message.checkpointSeq}`
    : `${message.role}:id:${message.id}`;
}
