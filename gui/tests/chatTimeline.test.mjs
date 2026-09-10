import assert from "node:assert/strict";
import { test } from "node:test";
import { createServer } from "vite";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";

const server = await createServer({ root: new URL("..", import.meta.url).pathname, configFile: false, server: { middlewareMode: true, hmr: false }, esbuild: { jsx: "automatic" } });
const { ChatMessageGroups } = await server.ssrLoadModule("/src/chat/ChatMessageGroups.tsx");

test("groups intermediate assistant replies into collapsed tool activity", () => {
  const tool = (id, checkpointSeq) => ({
    checkpointSeq,
    id,
    input: "command",
    intent: "run command",
    name: "shell",
    result: { output: "done", status: "success" },
    status: "success",
  });
  const indexed = (id, sequence, content, reasoning, toolCalls = []) => ({
    checkpoint: { branch: "", label: "[#" + String(sequence).padStart(3, "0") + "]", sequence },
    index: sequence,
    message: { content, id, reasoning, role: "assistant", toolCalls },
    toolMessageIDs: new Map(),
  });
  const messages = [
    indexed("a1", 0, "", "first thought", [tool("t1", 1)]),
    indexed("a2", 2, "intermediate response", "second thought"),
    indexed("a3", 3, "", "third thought", [tool("t2", 4)]),
    indexed("a4", 5, "final response", "final thought"),
  ];
  const html = renderToStaticMarkup(createElement(ChatMessageGroups, { messages, onOpenSubagent: () => {}, onStopTool: () => {} }));
  assert.equal(html.includes("intermediate response"), false);
  assert.equal(html.includes("final response"), true);
  assert.equal((html.match(/chat-tool-activity-controls/g) ?? []).length, 1);
  assert.equal((html.match(/chat-tool-card/g) ?? []).length, 0);
});

await server.close();
