import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, server: { middlewareMode: true, hmr: false } });
after(() => server.close());
const { createChatStreamBatch } = await server.ssrLoadModule("/src/chat/streamBatch.ts");
const { applyLiveStreamEvent } = await server.ssrLoadModule("/src/chat/chatClient.ts");
const { saveChat, getChat, updateChat, clearChatStore } = await server.ssrLoadModule("/src/chat/chatStore.ts");

test("a burst publishes once with exact text and the latest sequence", (t) => {
  t.mock.timers.enable({ apis: ["setTimeout"] });
  const published = [];
  const batch = createChatStreamBatch(events => published.push(events));
  const originals = Array.from({ length: 1000 }, (_, index) => ({ type: "assistant_delta", channel: "content", delta: `${index},`, seq: index + 1 }));
  for (const event of originals) batch.push(event);
  assert.equal(published.length, 0);
  t.mock.timers.tick(50);
  assert.equal(published.length, 1);
  assert.equal(published[0].length, 1);
  assert.equal(published[0][0].event.delta, originals.map(event => event.delta).join(""));
  assert.equal(published[0][0].event.seq, 1000);
  assert.equal(originals[0].delta, "0,");
  batch.flush();
  t.mock.timers.tick(100);
  assert.equal(published.length, 1);
});

test("continuous streaming publishes at the fixed deadline without waiting for silence", (t) => {
  t.mock.timers.enable({ apis: ["setTimeout"] });
  const published = [];
  const batch = createChatStreamBatch(events => published.push(events));
  for (let index = 0; index < 5; index++) {
    batch.push({ type: "assistant_delta", delta: "x" });
    t.mock.timers.tick(10);
  }
  assert.equal(published.length, 1);
  assert.equal(published[0][0].event.delta, "xxxxx");
  batch.push({ type: "assistant_delta", delta: "tail" });
  batch.flush(); // Stream close, disconnect or cancellation.
  t.mock.timers.tick(100);
  assert.equal(published.length, 2);
  assert.equal(published[1][0].event.delta, "tail");
});

test("reasoning, replay metadata and structural events preserve order and final state", (t) => {
  t.mock.timers.enable({ apis: ["setTimeout"] });
  const published = [];
  let chat = { id: "chat", projectID: "project", messages: [], source: "daemon" };
  const batch = createChatStreamBatch(events => {
    published.push(events);
    for (const { event, replay, imageOrigin } of events) chat = applyLiveStreamEvent(chat, event, imageOrigin, replay);
  });
  batch.push({ type: "assistant_start", turn: 1 });
  batch.push({ type: "assistant_delta", channel: "reasoning", delta: "think" }, true, "origin-a");
  batch.push({ type: "assistant_delta", channel: "content", delta: "hello" }, false, "origin-a");
  batch.push({ type: "assistant_delta", channel: "content", delta: " world" }, true, "origin-b");
  batch.push({ type: "tool_start", id: "tool", name: "find" });
  assert.equal(published[1].length, 3);
  assert.deepEqual(published[1].map(item => [item.event.channel, item.replay, item.imageOrigin]), [
    ["reasoning", true, "origin-a"], ["content", false, "origin-a"], ["content", true, "origin-b"],
  ]);
  assert.equal(chat.messages[0].reasoning, "think");
  assert.equal(chat.messages[0].content, "hello world");
  assert.equal(chat.messages[0].toolCalls[0].status, "running");
  batch.push({ type: "assistant_delta", channel: "content", delta: "!" });
  batch.push({ type: "run_end", exit_code: 0 });
  assert.equal(chat.messages[0].content, "hello world!");
  assert.equal(chat.status, "success");
  t.mock.timers.tick(100);
  assert.equal(published.length, 5);
});

test("stream updates retain historical messages and leave previous snapshots intact", () => {
  clearChatStore();
  const input = { id: "chat", projectID: "project", source: "daemon", messages: [
    { id: "old", role: "assistant", content: "history", toolCalls: [{ id: "done", name: "find", status: "success" }] },
    { id: "live", role: "assistant", content: "" },
  ] };
  const previous = saveChat(input);
  assert.notEqual(previous.messages[0], input.messages[0]);
  const next = updateChat("chat", current => applyLiveStreamEvent(current, { type: "assistant_delta", channel: "content", delta: "new" }));
  assert.equal(next.messages[0], previous.messages[0]);
  assert.equal(next.messages[0].toolCalls, previous.messages[0].toolCalls);
  assert.equal(previous.messages[1].content, "");
  assert.equal(next.messages[1].content, "new");
  assert.equal(getChat("chat"), next);
  clearChatStore();
});

test("a skill request sent by the server appears once, and snapshots keep it unless hidden", () => {
  const user = { id: "stream-user-1", role: "user", content: "/teach review cryptography" };
  let chat = { id: "chat", projectID: "project", messages: [], source: "daemon" };
  chat = applyLiveStreamEvent(chat, { type: "chat_start", chat: { id: "chat", messages: [] }, user }, "", false);
  assert.deepEqual(chat.messages.map(message => message.content), ["/teach review cryptography"]);
  // A normal message is already shown optimistically, so chat_start must not repeat it.
  const optimistic = { id: "chat", projectID: "project", messages: [{ id: "user-1", role: "user", content: "hello" }], source: "daemon" };
  const started = applyLiveStreamEvent(optimistic, { type: "chat_start", chat: { id: "chat", messages: [] }, user: { ...user, content: "hello" } }, "", false);
  assert.equal(started.messages.length, 1);
  const snapshot = applyLiveStreamEvent(chat, { type: "chat_snapshot", chat: { id: "chat", messages: [
    { role: "user", content: "/teach review cryptography" },
    { role: "user", content: "/help", guiHidden: true },
  ] } }, "", false);
  assert.deepEqual(snapshot.messages.map(message => message.content), ["/teach review cryptography"]);
});
