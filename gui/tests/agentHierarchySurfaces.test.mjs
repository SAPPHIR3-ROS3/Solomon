import assert from "node:assert/strict";
import { after, test } from "node:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const server = await createServer({ root: fileURLToPath(new URL("..", import.meta.url)), configFile: false, server: { middlewareMode: true, hmr: false } });
after(() => server.close());
const { AgentTreeItem, activeAgentNodesFromPayload } = await server.ssrLoadModule("/src/shell/ActiveAgentsPage.tsx");
const { SubagentActivityIndicator } = await server.ssrLoadModule("/src/chat/ChatMessageParts.tsx");

function node(depth) {
  return { id: `sub-${depth}`, title: `Task ${depth}`, kind: "subagent", projectID: "project", subagentDepth: depth, parentSubchatId: depth > 1 ? `sub-${depth - 1}` : undefined, status: depth === 4 ? "running" : "done", children: depth < 4 ? [node(depth + 1)] : [] };
}

test("active agents preserve completed ancestors and display each nested level", () => {
  const [parsed] = activeAgentNodesFromPayload({ id: "chat", title: "Main", kind: "chat", status: "", projectID: "project", children: [node(1)] });
  assert.equal(parsed.children[0].status, "done");
  assert.equal(parsed.children[0].children[0].subagentDepth, 2);
  const html = renderToStaticMarkup(createElement(AgentTreeItem, { node: parsed, onOpen() {} }));
  for (const depth of [2, 3, 4]) {
    assert.match(html, new RegExp(`L${depth} Subagent`));
    assert.match(html, new RegExp(`data-subagent-depth="${depth}"`));
  }
  assert.equal((html.match(/<ul>/g) ?? []).length, 4);
});

test("the expanded subagent pill displays levels and progressive indentation", () => {
  const subagents = [4, 2, 1, 3].map(depth => ({ messageID: "message", tool: { id: `call-${depth}`, name: "subagent", input: `Task ${depth}`, subagentDepth: depth, parentSubchatId: depth > 1 ? `sub-${depth - 1}` : undefined, result: { status: "success", subchatId: `sub-${depth}`, subchatStatus: "running" } } }));
  const html = renderToStaticMarkup(createElement(SubagentActivityIndicator, { subagents, isExpanded: true, onOpenSubagent() {}, onToggle() {} }));
  for (const depth of [2, 3, 4]) {
    assert.match(html, new RegExp(`L${depth} Subagent`));
    assert.match(html, new RegExp(`--subagent-indent:${(depth - 1) * 16}px`));
  }
  assert.ok(html.indexOf('data-subagent-depth="1"') < html.indexOf('data-subagent-depth="2"'));
  assert.ok(html.indexOf('data-subagent-depth="2"') < html.indexOf('data-subagent-depth="3"'));
});
