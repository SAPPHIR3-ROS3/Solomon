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

export async function deletePersistedMessage(chat: Chat, messageID: string, running: boolean) {
  const projectID = chat.projectID!;
  const latest = await settleChatStop(projectID, chat.id, running);
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

export async function settleChatStop(projectID: string, chatID: string, running = true) {
  let latest = await fetchLiveChat(projectID, chatID);
  if (running || latest.status === "running") {
    try {
      await stopLiveChat(projectID, chatID);
    } catch (reason) {
      latest = await fetchLiveChat(projectID, chatID);
      if (latest.status === "running") throw reason;
    }
    const deadline = Date.now() + 30000;
    do {
      latest = await fetchLiveChat(projectID, chatID);
      if (latest.status !== "running") break;
      if (Date.now() >= deadline) throw new Error("Chat is still stopping. Try deleting the message again.");
      await new Promise((resolve) => setTimeout(resolve, 50));
    } while (true);
  }
  return latest;
}
