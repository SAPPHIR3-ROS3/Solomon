import type { Chat, ChatMessage } from "./chatTypes";
import { deleteLiveChatMessage, fetchLiveChat, stopLiveChat } from "./chatClient";

export function removeQueuedMessage(queues: Map<string, ChatMessage[]>, chatID: string, messageID: string): boolean {
  const queue = queues.get(chatID);
  if (!queue?.some((message) => message.id === messageID)) return false;
  const remaining = queue.filter((message) => message.id !== messageID);
  if (remaining.length) queues.set(chatID, remaining);
  else queues.delete(chatID);
  return true;
}

export async function deletePersistedMessage(chat: Chat, messageID: string) {
  const projectID = chat.projectID!;
  const latest = await settleChatStop(projectID, chat.id);
  let target = messageID;
  if (messageID.startsWith("user-")) {
    const users = chat.messages.filter((message) => message.role === "user");
    const index = users.findIndex((message) => message.id === messageID);
    const persisted = latest.messages.filter((message) => message.role === "user")[index];
    if (!persisted) throw new Error("Message has not been saved yet. Try again.");
    target = persisted.id;
  }
  return deleteLiveChatMessage(projectID, chat.id, target);
}

export async function settleChatStop(projectID: string, chatID: string, includeSubagents = false) {
  if (!includeSubagents) {
    const latest = await fetchLiveChat(projectID, chatID);
    if (latest.status !== "running") return latest;
  }
  try {
    await stopLiveChat(projectID, chatID, includeSubagents);
  } catch (reason) {
    if (includeSubagents) throw reason;
    const latest = await fetchLiveChat(projectID, chatID);
    if (latest.status === "running") throw reason;
    return latest;
  }
  const deadline = Date.now() + 30000;
  while (true) {
    const latest = await fetchLiveChat(projectID, chatID);
    if (latest.status !== "running") return latest;
    if (Date.now() >= deadline) throw new Error("Chat is still stopping.");
    await new Promise((resolve) => setTimeout(resolve, 50));
  }
}
