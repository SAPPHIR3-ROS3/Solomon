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
  assert.equal(failed.result.subchatStatus, "failed");
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


test("partial assistant end lists retain existing subagent cards", () => {
  const previous = chatWith(agent("first"));
  const next = applyLiveStreamEvent(previous, { type: "assistant_end", tool_calls: [{ id: "read", name: "read_file" }] });
  assert.deepEqual(next.messages[0].toolCalls.map(tool => tool.id), ["first", "read"]);
  assert.equal(getTool(next, "first").result.subchatId, "child-first");
});

test("snapshots with missing tool calls retain subagent cards in their original message", () => {
  const previous = chatWith(agent("first"), agent("second", "done"));
  previous.messages[0].checkpointSeq = 1;
  for (const toolCalls of [undefined, [], [{ id: "read", name: "read_file" }]]) {
    const next = preserveLiveWorkState(previous, {
      ...previous,
      messages: [{ ...previous.messages[0], id: "persisted-first", toolCalls }, previous.messages[1]],
    });
    assert.equal(next.messages[0].toolCalls[0].id, "first");
    assert.equal(getTool(next, "first").result.subchatId, "child-first");
    assert.equal(getTool(next, "second").result.subchatStatus, "done");
    assert.equal(next.messages.flatMap(message => message.toolCalls ?? []).filter(tool => tool.id === "first").length, 1);
  }
});

test("snapshot reconciliation does not restore deleted messages", () => {
  const previous = chatWith(agent("first"), agent("second"));
  const next = preserveLiveWorkState(previous, { ...previous, messages: [previous.messages[1]] });
  assert.equal(getTool(next, "first"), undefined);
});

test("terminal snapshots close activity while other subagents keep running", () => {
  for (const state of ["done", "paused", "cancelled"]) {
    const previous = chatWith(agent("first"), agent("second"));
    const snapshot = liveChatFromPayload({
      ...previous,
      messages: previous.messages.map(message => ({
        ...message,
        toolCalls: message.toolCalls.map(tool => tool.id === "first" ? {
          ...tool,
          status: state === "done" ? "success" : "error",
          result: { status: "success", subchatId: "child-first", subagentStatus: state },
        } : tool),
      })),
    }, "project");
    const next = preserveLiveWorkState(previous, snapshot);
    assert.equal(getTool(next, "first").result.subchatStatus, state);
    assert.equal(getTool(next, "first").status, state === "done" ? "success" : "error");
    assert.equal(getTool(next, "second").status, "running");
  }
});

test("shared checkpoints do not duplicate subagents in later chain messages", () => {
  const previous = chatWith(agent("first"));
  previous.messages[0].checkpointSeq = 1;
  const next = preserveLiveWorkState(previous, { ...previous, messages: [
    previous.messages[0],
    { id: "later", role: "assistant", checkpointSeq: 1, content: "Working", toolCalls: [] },
  ] });
  assert.equal(next.messages.flatMap(message => message.toolCalls ?? []).length, 1);
});

test("failed snapshots without subchat metadata stop previously running agents", () => {
  const previous = chatWith(agent("first"));
  const next = preserveLiveWorkState(previous, { ...previous, messages: [{
    ...previous.messages[0], toolCalls: [{ id: "first", name: "subagent", status: "error", result: { status: "error", error: "Run ended" } }],
  }] });
  assert.equal(getTool(next, "first").result.subchatStatus, "failed");
  assert.equal(getTool(next, "first").status, "error");
});

test("completed synchronous results close running agents without inventing a pause", () => {
  const previous = chatWith({ ...agent("first"), sync: true });
  const next = applyLiveStreamEvent(previous, { type: "tool_result", id: "first", result: { ok: true, output: "Finished" } });
  assert.equal(getTool(next, "first").result.subchatStatus, "done");
  assert.equal(getTool(next, "first").status, "success");
});

test("background spawn acknowledgements keep agents running", () => {
  const next = applyLiveStreamEvent(chatWith(agent("first")), { type: "tool_result", id: "first", result: { ok: true } });
  assert.equal(getTool(next, "first").result.subchatStatus, "running");
});

test("resume conflicts retain the running subchat instead of marking it failed", () => {
  const previous = chatWith({ id: "resume", name: "subagent", status: "running" });
  const next = applyLiveStreamEvent(previous, { type: "tool_result", id: "resume", result: { ok: false, error: "subchat existing-child is running; use /subagent stop first" } });
  const tool = getTool(next, "resume");
  assert.equal(tool.result.subchatId, "existing-child");
  assert.equal(tool.result.subchatStatus, "running");
  assert.equal(tool.status, "running");
  assert.match(tool.result.error, /is running/);
});


test("server snapshots authoritatively reconcile every lifecycle transition", () => {
  const states = ["queued", "running", "done", "paused", "cancelled", "failed", "interrupted"];
  for (const before of states) {
    for (const after of states) {
      const previous = chatWith(agent("first", before));
      const snapshot = liveChatFromPayload(chatWith(agent("first", after)), "project");
      const tool = getTool(preserveLiveWorkState(previous, snapshot), "first");
      assert.equal(tool.result.subchatStatus, after, `${before} -> ${after}`);
      assert.equal(tool.status, ["running", "queued"].includes(after) ? "running" : after === "done" ? "success" : "error", `${before} -> ${after}`);
    }
  }
});

test("a replayed start targets the original tool without duplicating it", () => {
  const previous = chatWith(agent("first", "done"), agent("second"));
  const next = applyLiveStreamEvent(previous, { type: "tool_start", id: "first", name: "subagent" });
  assert.equal(next.messages.flatMap(message => message.toolCalls ?? []).filter(tool => tool.id === "first").length, 1);
  assert.equal(getTool(next, "first").result.subchatStatus, "done");
  assert.equal(getTool(next, "first").status, "success");
});

test("an inner tool failure leaves the enclosing subagent active until its lifecycle ends", () => {
  const previous = chatWith(agent("parent"));
  previous.messages.push({ id: "inner", role: "assistant", content: "", toolCalls: [{ id: "inner-tool", name: "shell", status: "running" }] });
  const next = applyLiveStreamEvent(previous, { type: "tool_result", id: "inner-tool", result: { ok: false, error: "Command failed" } });
  assert.equal(getTool(next, "inner-tool").status, "error");
  assert.equal(getTool(next, "parent").result.subchatStatus, "running");
  const finished = preserveLiveWorkState(next, chatWith(agent("parent", "done")));
  assert.equal(getTool(finished, "parent").status, "success");
});

test("the completed subchat lifecycle overrides a failed invocation result", () => {
  const previous = chatWith(agent("first"));
  const next = applyLiveStreamEvent(previous, { type: "tool_result", id: "first", result: {
    status: "error", error: "An earlier tool failed", subchatId: "child-first", subagentStatus: "done",
  } });
  assert.equal(getTool(next, "first").result.subchatStatus, "done");
  assert.equal(getTool(next, "first").status, "success");
});
