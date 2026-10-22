import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, server: { middlewareMode: true, hmr: false } });
after(() => server.close());
const { chatRequest, contextualChatError } = await server.ssrLoadModule("/src/chat/chatRequest.ts");

test("network errors identify the operation and exclude URL secrets", async (t) => {
  t.mock.method(globalThis, "fetch", async () => { throw new TypeError("Load failed"); });
  await assert.rejects(chatRequest("Unable to send message", "https://user:password@localhost:8080/chat/messages?token=secret#private"), error => {
    assert.match(error.message, /Unable to send message/);
    assert.match(error.message, /https:\/\/localhost:8080\/chat\/messages/);
    assert.match(error.message, /Load failed/);
    assert.doesNotMatch(error.message, /password|secret|private/);
    return true;
  });
});

test("cancellation stays recognizable", async (t) => {
  const abort = new DOMException("Aborted", "AbortError");
  t.mock.method(globalThis, "fetch", async () => { throw abort; });
  await assert.rejects(chatRequest("Unable to load chat", "http://localhost/chat"), error => error === abort);
});

test("stream errors retain context and original cause", () => {
  const cause = new TypeError("Load failed");
  const error = contextualChatError(cause, "Chat event stream interrupted", "http://localhost/events");
  assert.match(error.message, /Chat event stream interrupted.*\/events.*Load failed/);
  assert.equal(error.cause, cause);
});
