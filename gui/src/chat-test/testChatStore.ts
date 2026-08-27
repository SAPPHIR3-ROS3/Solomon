import { useSyncExternalStore } from "react";
import { createNewFakeChat, initialFakeChats, newPlaceholderChatID, type FakeChat } from "./fakeChats";

function cloneFakeChat(chat: FakeChat): FakeChat {
  return {
    ...chat,
    messages: chat.messages.map((message) => ({ ...message })),
  };
}

// Deliberately volatile: the store survives component navigation/remounts,
// but a full page reload starts again from the fixture chats.
let testChats = initialFakeChats.map(cloneFakeChat);
const listeners = new Set<() => void>();

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

function getSnapshot() {
  return testChats;
}

function notify() {
  listeners.forEach((listener) => listener());
}

export function useTestChatStore(): FakeChat[] {
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}

export function getTestChat(chatID: string): FakeChat | null {
  return testChats.find((chat) => chat.id === chatID) ?? null;
}

export function saveTestChat(chat: FakeChat): FakeChat {
  const savedChat = cloneFakeChat(chat);
  const chatIndex = testChats.findIndex((candidate) => candidate.id === savedChat.id);

  testChats = chatIndex === -1
    ? [...testChats, savedChat]
    : testChats.map((candidate, index) => index === chatIndex ? savedChat : candidate);
  notify();
  return savedChat;
}

export function createTestChat(): FakeChat {
  const chat = createNewFakeChat();
  saveTestChat(chat);
  return chat;
}

export function createTemporaryWorkspaceChat(workspaceID: string): FakeChat {
  const createdAt = Date.now();
  const placeholderID = newPlaceholderChatID(new Date(createdAt));
  const chat = {
    ...createNewFakeChat(workspaceID, placeholderID, createdAt),
    id: placeholderID,
  };
  return saveTestChat(chat);
}

export function updateTestChat(
  chatID: string,
  update: (chat: FakeChat) => FakeChat,
): FakeChat | null {
  const currentChat = testChats.find((chat) => chat.id === chatID);
  if (!currentChat) return null;

  const nextChat = update(currentChat);
  if (nextChat === currentChat) return currentChat;
  return saveTestChat(nextChat);
}

export function resetFakeChatSubagents(chatID: string): FakeChat | null {
  return updateTestChat(chatID, (current) => {
    let changed = false;
    const messages = current.messages.map((message) => {
      if (!message.toolCalls?.some((tool) => tool.name === "subagent" && tool.status === "interrupted")) return message;
      changed = true;
      return {
        ...message,
        toolCalls: message.toolCalls.map((tool) => (
          tool.name === "subagent" && tool.status === "interrupted" ? { ...tool, status: "running" as const } : tool
        )),
      };
    });
    return changed ? { ...current, messages } : current;
  });
}
