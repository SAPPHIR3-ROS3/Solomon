import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, server: { middlewareMode: true, hmr: false } });
after(() => server.close());
const { applyLiveStreamEvent, preserveLiveWorkState, liveChatFromPayload } = await server.ssrLoadModule("/src/chat/chatClient.ts");
const agent = (id, status = "running") => ({ id, name: "subagent", status: "running", result: { status: "success", subchatId: `child-${id}`, subchatStatus: status } });
const chatWith = (...tools) => ({ id: "parent", projectID: "project", source: "daemon", title: "Parent", messages: tools.map((tool, index) => ({ id: `message-${index}`, role: "assistant", content: "", toolCalls: [tool] })) });
const getTool = (chat, id) => chat.messages.flatMap(message => message.toolCalls ?? []).find(tool => tool.id === id);

test("failed results retain the subchat link and leave concurrent agents active", () => {
  const previous = chatWith(agent("first"), agent("second"));
  const next = applyLiveStreamEvent(previous, { type: "tool_result", id: "first", result: { ok: false, error: "Provider unavailable" } });
  const failed = getTool(next, "first");
  assert.equal(failed.result.subchatId, "child-first");
  assert.equal(failed.result.subchatStatus, "paused");
  assert.equal(failed.result.error, "Provider unavailable");
  assert.equal(failed.status, "error");
  assert.equal(getTool(next, "second").result.subchatStatus, "running");
  assert.equal(getTool(previous, "first").status, "running");
});

test("late spawn events update their original message, not the newest assistant", () => {
  const previous = chatWith({ id: "first", name: "subagent", status: "running" }, agent("second"));
  const next = applyLiveStreamEvent(previous, { type: "subagent_start", id: "first", subchatId: "child-first", status: "running" });
  assert.equal(getTool(next, "first").result.subchatId, "child-first");
  assert.equal(getTool(next, "second").result.subchatId, "child-second");
});

test("delayed results cannot reactivate terminal agents in earlier messages", () => {
  for (const status of ["done", "paused", "cancelled"]) {
    const next = applyLiveStreamEvent(chatWith(agent("first", status), agent("second")), { type: "tool_result", id: "first", result: { ok: true, subchatId: "child-first", status: "running" } });
    assert.equal(getTool(next, "first").result.subchatStatus, status);
    assert.equal(getTool(next, "first").status, status === "done" ? "success" : "error");
  }
});

test("partial snapshots retain subchat identity and active status by tool ID", () => {
  const previous = chatWith(agent("first"), agent("second", "paused"));
  const snapshot = liveChatFromPayload({ id: "parent", messages: previous.messages.map(message => ({ ...message, toolCalls: message.toolCalls.map(tool => ({ id: tool.id, name: tool.name, status: "success", result: { status: "success" } })) })) }, "project");
  const next = preserveLiveWorkState(previous, snapshot);
  assert.equal(getTool(next, "first").result.subchatId, "child-first");
  assert.equal(getTool(next, "first").status, "running");
  assert.equal(getTool(next, "second").result.subchatStatus, "paused");
  const finished = preserveLiveWorkState(previous, chatWith(agent("first", "done")));
  assert.equal(getTool(finished, "first").status, "success");
});

test("assistant end and replayed tool starts do not erase subchat metadata", () => {
  const previous = chatWith(agent("first"));
  const ended = applyLiveStreamEvent(previous, { type: "assistant_end", tool_calls: [{ id: "first", name: "subagent" }] });
  const replayed = applyLiveStreamEvent(ended, { type: "tool_start", id: "first", name: "subagent" });
  assert.equal(getTool(replayed, "first").result.subchatId, "child-first");
  assert.equal(getTool(replayed, "first").status, "running");
});

test("replaced subchat identities do not inherit old terminal state", () => {
  const next = applyLiveStreamEvent(chatWith(agent("first", "done")), { type: "subagent_start", id: "first", subchatId: "new-child", status: "running" });
  assert.equal(getTool(next, "first").result.subchatId, "new-child");
  assert.equal(getTool(next, "first").result.subchatStatus, "running");
});
