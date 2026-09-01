import { useSyncExternalStore } from "react";
import type { Chat, ChatMessage } from "./chatTypes";

const ACTIVE_CHAT_KEY = "solomon.active-chat";
const CHAT_STREAM_CURSOR_PREFIX = "solomon.chat-stream-cursor.v1";

export type ActiveChatSelection = {
  chatID: string;
  projectID: string;
};

function cloneMessage(message: ChatMessage): ChatMessage {
  return {
    ...message,
    images: message.images?.map((image) => ({ ...image })),
    retainedMessages: message.retainedMessages?.map((retained) => ({
      ...retained,
      images: retained.images?.map((image) => ({ ...image })),
    })),
    stats: message.stats ? { ...message.stats } : undefined,
    toolCalls: message.toolCalls?.map((tool) => ({
      ...tool,
      parameters: tool.parameters?.map((parameter) => ({ ...parameter })),
      result: tool.result ? {
        ...tool.result,
        items: tool.result.items ? [...tool.result.items] : undefined,
        todoItems: tool.result.todoItems?.map((item) => ({ ...item })),
      } : undefined,
    })),
  };
}

export function cloneChat(chat: Chat): Chat {
  return {
    ...chat,
    messages: chat.messages.map(cloneMessage),
    source: "daemon",
  };
}

// The browser-side cache contains only chats loaded from the real daemon API.
let chats: Chat[] = [];
const listeners = new Set<() => void>();

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

function getSnapshot() {
  return chats;
}

function notify() {
  listeners.forEach((listener) => listener());
}

export function useChatStore(): Chat[] {
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}

export function getChat(chatID: string): Chat | null {
  return chats.find((chat) => chat.id === chatID) ?? null;
}

export function saveChat(chat: Chat): Chat {
  const savedChat = cloneChat(chat);
  const chatIndex = chats.findIndex((candidate) => candidate.id === savedChat.id);
  chats = chatIndex === -1
    ? [...chats, savedChat]
    : chats.map((candidate, index) => index === chatIndex ? savedChat : candidate);
  notify();
  return savedChat;
}

export function updateChat(chatID: string, update: (chat: Chat) => Chat): Chat | null {
  const current = getChat(chatID);
  if (!current) return null;
  const next = update(current);
  if (next === current) return current;
  return saveChat(next);
}

export function clearChatStore(): void {
  chats = [];
  notify();
}

export function rememberActiveChat(projectID: string, chatID: string): void {
  try {
    if (typeof window === "undefined") return;
    window.localStorage.setItem(ACTIVE_CHAT_KEY, JSON.stringify({ chatID, projectID }));
  } catch {
    // Persistence is a convenience; private browsing/storage restrictions must
    // not prevent the daemon chat from working.
  }
}

export function getRememberedActiveChat(): ActiveChatSelection | null {
  try {
    if (typeof window === "undefined") return null;
    const value: unknown = JSON.parse(window.localStorage.getItem(ACTIVE_CHAT_KEY) ?? "null");
    if (!value || typeof value !== "object") return null;
    const record = value as Record<string, unknown>;
    if (typeof record.projectID !== "string" || typeof record.chatID !== "string") return null;
    if (!record.projectID || !record.chatID) return null;
    return { chatID: record.chatID, projectID: record.projectID };
  } catch {
    return null;
  }
}

export function forgetRememberedActiveChat(): void {
  try {
    if (typeof window !== "undefined") window.localStorage.removeItem(ACTIVE_CHAT_KEY);
  } catch {
    // Ignore storage restrictions.
  }
}

function chatStreamCursorKey(projectID: string, chatID: string): string {
  return `${CHAT_STREAM_CURSOR_PREFIX}.${encodeURIComponent(projectID)}.${encodeURIComponent(chatID)}`;
}

export function getChatStreamCursor(projectID: string, chatID: string): number {
  try {
    if (typeof window === "undefined") return 0;
    const value = Number(window.localStorage.getItem(chatStreamCursorKey(projectID, chatID)) ?? 0);
    return Number.isFinite(value) && value > 0 ? Math.floor(value) : 0;
  } catch {
    return 0;
  }
}

export function saveChatStreamCursor(projectID: string, chatID: string, cursor: number): void {
  try {
    if (typeof window === "undefined") return;
    const value = Number.isFinite(cursor) && cursor > 0 ? Math.floor(cursor) : 0;
    if (value > 0) window.localStorage.setItem(chatStreamCursorKey(projectID, chatID), String(value));
    else window.localStorage.removeItem(chatStreamCursorKey(projectID, chatID));
  } catch {
    // Stream replay remains correct for the current page when storage is unavailable.
  }
}

export function forgetChatStreamCursor(projectID: string, chatID: string): void {
  saveChatStreamCursor(projectID, chatID, 0);
}
