import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, server: { middlewareMode: true, hmr: false } });
after(() => server.close());
const { createChatStreamBatch } = await server.ssrLoadModule("/src/chat/streamBatch.ts");
const { applyLiveStreamEvent } = await server.ssrLoadModule("/src/chat/chatClient.ts");
const { saveChat, getChat, updateChat, clearChatStore } = await server.ssrLoadModule("/src/chat/chatStore.ts");

test("replayed assistant messages use the event time", () => {
  const ts = "2026-10-20T13:00:00Z";
  const chat = applyLiveStreamEvent({ id: "chat", projectID: "project", messages: [], source: "daemon" }, { type: "assistant_start", ts, turn: 0 }, "", true);
  assert.equal(chat.messages[0].createdAt, Date.parse(ts));
});

test("a live compaction snapshot shows the summary before the next assistant response", () => {
  let chat = { id: "chat", projectID: "project", messages: [], source: "daemon", status: "running" };
  chat = applyLiveStreamEvent(chat, { type: "chat_snapshot", chat: {
    id: "chat", status: "running", messages: [
      { id: "compaction-0", role: "assistant", kind: "compaction", content: "", summary: "Continue the prototype.", retainedMessages: [{ role: "user", content: "Earlier request" }] },
      { id: "m-1", role: "user", content: "Next request" },
    ],
  } });
  assert.equal(chat.status, "running");
  assert.equal(chat.messages[0].kind, "compaction");
  assert.equal(chat.messages[0].summary, "Continue the prototype.");
  assert.equal(chat.messages[0].retainedMessages[0].content, "Earlier request");
  chat = applyLiveStreamEvent(chat, { type: "assistant_start", turn: 0, checkpoint_seq: 0 });
  chat = applyLiveStreamEvent(chat, { type: "assistant_delta", channel: "content", delta: "New answer" });
  assert.deepEqual(chat.messages.map(message => message.content), ["", "Next request", "New answer"]);
  assert.equal(chat.messages[0].kind, "compaction");
});

test("a newer compaction replaces the previous summary instead of stacking it", () => {
  let chat = { id: "chat", projectID: "project", messages: [], source: "daemon", status: "running" };
  chat = applyLiveStreamEvent(chat, { type: "chat_snapshot", chat: { id: "chat", messages: [
    { id: "m-archive-0-0", role: "user", content: "old request" },
    { id: "compaction-0", role: "assistant", kind: "compaction", content: "", summary: "First summary" },
    { id: "m-1", role: "user", content: "next" },
  ] } });
  chat = applyLiveStreamEvent(chat, { type: "chat_snapshot", chat: { id: "chat", messages: [
    { id: "compaction-1", role: "assistant", kind: "compaction", content: "", summary: "Second summary", historyAvailable: true },
    { id: "m-1", role: "user", content: "pending" },
  ] } });
  assert.deepEqual(chat.messages.map((message) => message.summary ?? message.content), ["old request", "Second summary", "pending"]);
  assert.equal(chat.messages.filter((message) => message.kind === "compaction").length, 1);
  assert.equal(chat.messages.find((message) => message.kind === "compaction").historyAvailable, false);
});

test("later and already seen compaction cards keep loaded archive", () => {
  let chat = { id: "chat", projectID: "project", messages: [], source: "daemon", status: "running" };
  chat = applyLiveStreamEvent(chat, { type: "chat_snapshot", chat: { id: "chat", messages: [
    { id: "m-archive-0-0", role: "user", content: "old request" },
    { id: "compaction-0", role: "assistant", kind: "compaction", content: "", summary: "First summary" },
    { id: "m-1", role: "user", content: "next" },
  ] } });
  chat = applyLiveStreamEvent(chat, { type: "chat_snapshot", chat: { id: "chat", messages: [
    { id: "compaction-2", role: "assistant", kind: "compaction", content: "", summary: "Later summary", historyAvailable: true },
    { id: "m-1", role: "user", content: "next" },
  ] } });
  assert.deepEqual(chat.messages.map((message) => message.summary ?? message.content), ["old request", "Later summary", "next"]);
  chat = applyLiveStreamEvent(chat, { type: "chat_snapshot", chat: { id: "chat", messages: [
    { id: "compaction-0", role: "assistant", kind: "compaction", content: "", summary: "First summary", historyAvailable: true },
    { id: "m-1", role: "user", content: "next" },
  ] } });
  assert.deepEqual(chat.messages.map((message) => message.summary ?? message.content), ["old request", "First summary", "next"]);
  assert.equal(chat.messages.filter((message) => message.kind === "compaction").length, 1);
});

test("an unrelated compaction card drops loaded archive", () => {
  let chat = { id: "chat", projectID: "project", messages: [], source: "daemon", status: "running" };
  chat = applyLiveStreamEvent(chat, { type: "chat_snapshot", chat: { id: "chat", messages: [
    { id: "m-archive-0-0", role: "user", content: "old request" },
    { id: "compaction-5", role: "assistant", kind: "compaction", content: "", summary: "Fifth summary" },
  ] } });
  chat = applyLiveStreamEvent(chat, { type: "chat_snapshot", chat: { id: "chat", messages: [
    { id: "compaction-1", role: "assistant", kind: "compaction", content: "", summary: "Other summary", historyAvailable: true },
  ] } });
  assert.deepEqual(chat.messages.map((message) => message.summary ?? message.content), ["Other summary"]);
});

test("the same compaction keeps loaded history and a single summary", () => {
  let chat = { id: "chat", projectID: "project", messages: [], source: "daemon", status: "running" };
  chat = applyLiveStreamEvent(chat, { type: "chat_snapshot", chat: { id: "chat", messages: [
    { id: "m-archive-0-0", role: "user", content: "old request" },
    { id: "compaction-0", role: "assistant", kind: "compaction", content: "", summary: "Only summary" },
    { id: "m-1", role: "user", content: "next" },
  ] } });
  chat = applyLiveStreamEvent(chat, { type: "chat_snapshot", chat: { id: "chat", messages: [
    { id: "compaction-0", role: "assistant", kind: "compaction", content: "", summary: "Only summary", historyAvailable: true },
    { id: "m-1", role: "user", content: "next" },
  ] } });
  assert.deepEqual(chat.messages.map((message) => message.summary ?? message.content), ["old request", "Only summary", "next"]);
  assert.equal(chat.messages.filter((message) => message.kind === "compaction").length, 1);
});

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
  // An earlier request that never got a reply is history, not the request that just started.
  const interrupted = { id: "chat", projectID: "project", messages: [{ id: "msg-7", role: "user", content: "previous interrupted request" }], source: "daemon" };
  const next = applyLiveStreamEvent(interrupted, { type: "chat_start", chat: { id: "chat", messages: [] }, user: { ...user, content: "/teach new request" } }, "", false);
  assert.deepEqual(next.messages.map(message => message.content), ["previous interrupted request", "/teach new request"]);
  const snapshot = applyLiveStreamEvent(chat, { type: "chat_snapshot", chat: { id: "chat", messages: [
    { role: "user", content: "/teach review cryptography" },
    { role: "user", content: "/help", guiHidden: true },
  ] } }, "", false);
  assert.deepEqual(snapshot.messages.map(message => message.content), ["/teach review cryptography"]);
});

test("regenerate replaces the last answer while preserving the original user attachments", () => {
  const user = { id: "user", role: "user", content: "question", images: [{ id: "image" }] };
  const history = { id: "earlier", role: "assistant", content: "earlier reply" };
  const chat = { id: "chat", messages: [history, user, { id: "old", role: "assistant", content: "old answer" }], status: "running" };
  const regenerated = applyLiveStreamEvent(chat, { type: "regenerate_start" });
  assert.deepEqual(regenerated.messages, [history, user]);
  assert.equal(regenerated.messages[1], user);
  assert.equal(regenerated.status, "running");
  assert.equal(chat.messages.length, 3);
});

test("a delayed spawn result does not reactivate a completed subagent", () => {
  const chat = { id: "chat", messages: [{ id: "assistant", role: "assistant", content: "", toolCalls: [{
    id: "agent", name: "subagent", status: "success", result: { status: "success", subchatId: "child", subchatStatus: "done" },
  }] }] };
  const next = applyLiveStreamEvent(chat, { type: "tool_result", id: "agent", result: { status: "success", subchatId: "child", subagentStatus: "running" } });
  assert.equal(next.messages[0].toolCalls[0].result.subchatStatus, "done");
  assert.equal(next.messages[0].toolCalls[0].status, "success");
});
