import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";

const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, server: { middlewareMode: true, hmr: false }, esbuild: { jsx: "automatic" } });
after(() => server.close());
const { SubagentCard } = await server.ssrLoadModule("/src/chat/ChatToolActivity.tsx");
const { SubagentChatPanel } = await server.ssrLoadModule("/src/chat/ChatMessageGroups.tsx");
const tool = { id: "subagent-1", name: "subagent", input: "Complete task instructions", intent: "Verify GPU placement", status: "running" };
const renderCard = (overrides = {}) => renderToStaticMarkup(createElement(SubagentCard, { tool: { ...tool, ...overrides } }));
const renderPanel = (messages, overrides = {}) => renderToStaticMarkup(createElement(SubagentChatPanel, { messages, tool: { ...tool, ...overrides }, onCollapse() {} }));

test("the compact subagent card shows intent instead of task", () => {
  const html = renderCard();
  assert.match(html, /Verify GPU placement/);
  assert.doesNotMatch(html, /Complete task instructions/);
  assert.match(renderCard({ intent: "" }), /Intent missing/);
  assert.doesNotMatch(renderCard({ intent: undefined }), /Complete task instructions/);
});

test("subagent intent stays within eight words and eighty characters", () => {
  assert.match(renderCard({ intent: " one two\nthree four five six seven eight nine ten " }), /one two three four five six seven eight…/);
  const long = "a".repeat(100);
  assert.match(renderCard({ intent: long }), new RegExp(`${"a".repeat(80)}…`));
  assert.doesNotMatch(renderCard({ intent: long }), new RegExp("a".repeat(81)));
});

test("the expanded chat renders task once and retains the transcript", () => {
  const html = renderPanel([
    { id: "task", role: "user", content: tool.input },
    { id: "reply", role: "assistant", content: "Progress report" },
    { id: "followup", role: "user", content: "Follow-up instructions" },
  ]);
  assert.equal(html.split(tool.input).length - 1, 1);
  assert.doesNotMatch(html, /class="chat-subchat-task"/);
  assert.match(html, /Progress report/);
  assert.match(html, /Follow-up instructions/);
  assert.match(html, /chat-copy-message/);
});

test("task comparison tolerates line endings and surrounding whitespace", () => {
  const html = renderPanel([{ id: "task", role: "user", content: " First line\nSecond line " }], { input: "First line\r\nSecond line" });
  assert.doesNotMatch(html, /class="chat-subchat-task"/);
});

test("the expanded chat shows the full task while loading or when absent from the transcript", () => {
  for (const messages of [undefined, [], [{ id: "reply", role: "assistant", content: "Progress report" }]]) {
    const html = renderPanel(messages);
    assert.match(html, /class="chat-subchat-task"/);
    assert.equal(html.split(tool.input).length - 1, 1);
  }
  const loading = renderToStaticMarkup(createElement(SubagentChatPanel, {
    isLoading: true, messages: [{ id: "task", role: "user", content: tool.input }], tool, onCollapse() {},
  }));
  assert.match(loading, /Loading subchat/);
  assert.equal(loading.split(tool.input).length - 1, 1);
});
