import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";

const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, server: { middlewareMode: true, hmr: false }, esbuild: { jsx: "automatic" } });
after(() => server.close());
const { applyLiveStreamEvent } = await server.ssrLoadModule("/src/chat/events.ts");
const { ToolCallCard } = await server.ssrLoadModule("/src/chat/ChatToolActivity.tsx");

test("an incomplete tool start immediately renders a loading card and fills the same card later", () => {
  const initial = { messages: [{ id: "user", role: "user", content: "Run" }] };
  const started = applyLiveStreamEvent(initial, { type: "tool_start", id: "call-1" });
  const tool = started.messages.at(-1).toolCalls[0];
  const loading = renderToStaticMarkup(createElement(ToolCallCard, { tool }));
  assert.match(loading, /chat-tool-card is-running/);
  assert.match(loading, /Loading tool call/);
  assert.match(loading, /role="status"/);
  assert.doesNotMatch(loading, /Intent missing/);
  const ready = applyLiveStreamEvent(started, { type: "tool_start", id: "call-1", name: "shell", arguments: { intent: "List files", command: "ls" } });
  assert.equal(ready.messages.length, 2);
  assert.equal(ready.messages.at(-1).toolCalls.length, 1);
  const filled = renderToStaticMarkup(createElement(ToolCallCard, { tool: ready.messages.at(-1).toolCalls[0] }));
  assert.match(filled, /List files/);
  assert.doesNotMatch(filled, /Loading tool call/);
});
