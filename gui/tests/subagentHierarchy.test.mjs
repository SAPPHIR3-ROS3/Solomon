import assert from "node:assert/strict";
import { after, test } from "node:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, server: { middlewareMode: true, hmr: false } });
after(() => server.close());
const { buildSubagentTree, orderSubagentTools, openSubchatPath } = await server.ssrLoadModule("/src/chat/subagents/hierarchy.ts");
const { SubagentCard } = await server.ssrLoadModule("/src/chat/ChatToolActivity.tsx");
const { SubagentChatPanel, ChatMessageGroups } = await server.ssrLoadModule("/src/chat/ChatMessageGroups.tsx");
const { indexChatMessages } = await server.ssrLoadModule("/src/chat/chatMessageUtils.ts");
const { toolCallFromPayload } = await server.ssrLoadModule("/src/chat/payload.ts");
const agent = depth => ({ id: `call-${depth}`, name: "subagent", subagentDepth: depth, parentSubchatId: depth > 1 ? `sub-${depth - 1}` : undefined, result: { status: "success", subchatId: `sub-${depth}`, subchatStatus: "done" } });

test("nested cards retain hierarchy metadata and show their level", () => {
  for (const depth of [2, 3, 4]) {
    const tool = toolCallFromPayload(agent(depth), 0);
    const html = renderToStaticMarkup(createElement(SubagentCard, { tool, onOpenSubagent() {} }));
    assert.match(html, new RegExp(`L${depth} Subagent`));
    assert.match(html, new RegExp(`data-parent-subchat="sub-${depth - 1}"`));
    assert.match(html, new RegExp(`--subagent-depth:${depth}`));
  }
});

test("descendants appear immediately below their caller", () => {
  const independent = { ...agent(1), id: "other", result: { status: "success", subchatId: "other" } };
  const tools = orderSubagentTools([agent(3), independent, agent(2), agent(4), agent(1)]);
  assert.deepEqual(tools.map(tool => tool.id), ["other", "call-1", "call-2", "call-3", "call-4"]);
  const messages = indexChatMessages([{ id: "message", role: "assistant", content: "", toolCalls: tools }]);
  const html = renderToStaticMarkup(createElement(ChatMessageGroups, { messages, onOpenSubagent() {} }));
  assert.ok(html.indexOf("L2 Subagent") < html.indexOf("L3 Subagent"));
  assert.ok(html.indexOf("L3 Subagent") < html.indexOf("L4 Subagent"));
});

test("subchat panels expose child navigation and a return to the parent", () => {
  const messages = [{ id: "inner", role: "assistant", content: "", toolCalls: [agent(3)] }];
  const html = renderToStaticMarkup(createElement(SubagentChatPanel, { tool: agent(2), messages, onCollapse() {}, onBack() {}, onOpenSubagent() {} }));
  assert.match(html, /L2 Subagent chat/);
  assert.match(html, /Back to parent subchat/);
  assert.match(html, /aria-label="Open L3 Subagent chat"/);
  assert.match(html, /role="button"/);
  assert.match(html, /--subagent-base-depth:3/);
});

test("navigation reaches level four and reuses ancestors instead of forming a cycle", () => {
  let path = [];
  for (const depth of [1, 2, 3, 4]) path = openSubchatPath(path, agent(depth));
  assert.deepEqual(path.map(tool => tool.subagentDepth), [1, 2, 3, 4]);
  path = openSubchatPath(path, agent(2));
  assert.deepEqual(path.map(tool => tool.subagentDepth), [1, 2]);
  assert.equal(openSubchatPath(path, { id: "missing", name: "subagent" }), path);
});


test("siblings share a branch attached to their calling subagent", () => {
  const first = agent(2);
  const second = { ...agent(2), id: "second-child", result: { status: "success", subchatId: "second-child" } };
  const tree = buildSubagentTree([first, agent(1), second, agent(3)]);
  assert.equal(tree.length, 1);
  assert.equal(tree[0].tool.id, "call-1");
  assert.deepEqual(tree[0].children.map(node => node.tool.id), ["call-2", "second-child"]);
  assert.equal(tree[0].children[0].children[0].tool.id, "call-3");
  const messages = indexChatMessages([{ id: "message", role: "assistant", content: "", toolCalls: [agent(1), first, second, agent(3)] }]);
  const html = renderToStaticMarkup(createElement(ChatMessageGroups, { messages, onOpenSubagent() {} }));
  assert.match(html, /data-subchat-id="sub-1"/);
  assert.equal((html.match(/class="chat-subagent-children"/g) ?? []).length, 2);
  assert.ok(html.indexOf('data-subchat-id="sub-1"') < html.indexOf('class="chat-subagent-children"'));
});
