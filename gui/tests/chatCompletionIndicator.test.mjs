import assert from "node:assert/strict";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";

const root = fileURLToPath(new URL("..", import.meta.url));
const server = await createServer({ root, configFile: false, server: { middlewareMode: true, hmr: false }, esbuild: { jsx: "automatic" } });
const { markChatCompletionRead, markChatCompletionUnread, shouldMarkChatCompletionUnread } = await server.ssrLoadModule("/src/chat/unreadCompletion.ts");
const { ChatListButton } = await server.ssrLoadModule("/src/shell/SidePanel.tsx");

const base = { chatID: "chat-1", currentCursor: 8, eventSequence: 9, eventType: "run_end", isReplay: false, selectedChatID: null };

test("marks a new live completion unread only when its chat is not selected", () => {
  assert.equal(shouldMarkChatCompletionUnread(base), true);
  assert.equal(shouldMarkChatCompletionUnread({ ...base, selectedChatID: "chat-1" }), false);
  assert.equal(shouldMarkChatCompletionUnread({ ...base, eventType: "assistant_end" }), false);
});

test("accepts a replayed completion only when its sequence is newer than the stored cursor", () => {
  assert.equal(shouldMarkChatCompletionUnread({ ...base, isReplay: true }), true);
  assert.equal(shouldMarkChatCompletionUnread({ ...base, isReplay: true, eventSequence: 8 }), false);
  assert.equal(shouldMarkChatCompletionUnread({ ...base, isReplay: true, eventSequence: undefined }), false);
});

test("unread and read transitions are immutable and idempotent", () => {
  const initial = new Set(["other"]);
  const unread = markChatCompletionUnread(initial, "chat-1");
  assert.equal(initial.has("chat-1"), false);
  assert.equal(markChatCompletionUnread(unread, "chat-1"), unread);
  const read = markChatCompletionRead(unread, "chat-1");
  assert.equal(unread.has("chat-1"), true);
  assert.equal(read.has("chat-1"), false);
  assert.equal(markChatCompletionRead(read, "chat-1"), read);
});

test("renders a steady green indicator for completed unread chats but keeps active work blue", () => {
  const props = { chatID: "chat-1", dateTime: "2026-01-01T00:00:00Z", isUnreadCompleted: true, onClick: () => {}, onContextMenu: () => {}, onContextMenuKey: () => {}, timeLabel: "now", title: "Build" };
  const unread = renderToStaticMarkup(createElement(ChatListButton, props));
  assert.match(unread, /side-panel-chat is-unread-completed/);
  assert.match(unread, /side-panel-chat-working-dot is-completed/);
  assert.match(unread, /completed, unread/);
  const active = renderToStaticMarkup(createElement(ChatListButton, { ...props, isWorking: true }));
  assert.match(active, /side-panel-chat is-working/);
  assert.match(active, /side-panel-chat-working-dot"/);
  assert.doesNotMatch(active, /is-completed/);
});

await server.close();
