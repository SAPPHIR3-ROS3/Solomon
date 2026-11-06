import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const server = await createServer({
  root: fileURLToPath(new URL("..", import.meta.url)), configFile: false,
  server: { middlewareMode: true, hmr: false },
  plugins: [{ name: "mock-chat-client", enforce: "pre",
    resolveId(id, importer) { if (id === "./chatClient" && importer?.endsWith("/messageDeletion.ts")) return "virtual:deletion-client"; },
    load(id) { if (id === "virtual:deletion-client") return "export const fetchLiveChat = (...args) => globalThis.deletionClient.fetch(...args); export const stopLiveChat = (...args) => globalThis.deletionClient.stop(...args); export const deleteLiveChatMessage = (...args) => globalThis.deletionClient.delete(...args);"; },
  }],
});
after(() => server.close());
const { removeQueuedMessage, deletePersistedMessage } = await server.ssrLoadModule("/src/chat/messageDeletion.ts");
const chat = { id: "chat", projectID: "project", messages: [{ id: "user-local", role: "user", content: "Prompt" }] };

test("deleting queued messages preserves order and removes empty queue entries", () => {
  const queues = new Map([["chat", [{ id: "a" }, { id: "b" }, { id: "c" }]]]);
  assert.equal(removeQueuedMessage(queues, "chat", "missing"), false);
  assert.equal(removeQueuedMessage(queues, "chat", "b"), true);
  assert.deepEqual(queues.get("chat").map(message => message.id), ["a", "c"]);
  removeQueuedMessage(queues, "chat", "a");
  removeQueuedMessage(queues, "chat", "c");
  assert.equal(queues.has("chat"), false);
});

test("deletion waits for stop completion and resolves optimistic IDs", async () => {
  const calls = [];
  let reads = 0;
  globalThis.deletionClient = {
    fetch: async () => { reads++; calls.push("fetch"); return { status: reads < 3 ? "running" : "success", messages: [{ id: "message-0", role: "user" }] }; },
    stop: async () => { calls.push("stop"); },
    delete: async (...args) => { calls.push(args); return { status: "success", messages: [] }; },
  };
  const result = await deletePersistedMessage(chat, "user-local", true);
  assert.deepEqual(result.messages, []);
  assert.deepEqual(calls, ["fetch", "stop", "fetch", "fetch", ["project", "chat", "message-0"]]);
});

test("a run finishing before stop is not a deletion failure", async () => {
  let reads = 0;
  globalThis.deletionClient = {
    fetch: async () => ({ status: ++reads === 1 ? "running" : "success", messages: [] }),
    stop: async () => { throw new Error("chat is not running"); },
    delete: async (...args) => args,
  };
  assert.deepEqual(await deletePersistedMessage(chat, "message-0", true), ["project", "chat", "message-0"]);
});

test("failed stops do not send a conflicting delete request", async () => {
  globalThis.deletionClient = {
    fetch: async () => ({ status: "running", messages: [] }),
    stop: async () => { throw new Error("stop failed"); },
    delete: async () => assert.fail("delete must not run"),
  };
  await assert.rejects(deletePersistedMessage(chat, "message-0", true), /stop failed/);
});
