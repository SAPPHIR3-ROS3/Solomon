import assert from "node:assert/strict";
import test from "node:test";
import {
  allowedToolNamesFromRequest,
  filterInvocations,
  isValidInvocation,
  limitInvocations,
  parseToolInvocationsFromText,
  requestUsesNativeTools,
  toolArgumentsJSON,
} from "../src/openai-tools.js";
import { collapseExactRepeat, nextTextChunk, processStreamEvent, proxyToolCorrectionMessage } from "../src/chat-helpers.js";
import { bridgeToolInvocation } from "../src/legacy.js";
import { harnessToolsClause } from "../src/harness-prompt.js";
import type { CursorNativeToolEvent } from "../src/cursor-native-tools.js";
import type { ChatCompletionTool } from "../src/openai-types.js";

function bridgeCtx(...names: string[]) {
  return { allowedNames: new Set(names) };
}

const tools: ChatCompletionTool[] = [
  { type: "function", function: { name: "readFile" } },
  { type: "function", function: { name: "shell" } },
];

test("parses proxy XML tool blocks from assistant text", () => {
  const parsed = parseToolInvocationsFromText(
    'before\n<tool_calls><tool name="readFile"><args>{"path":"README.md"}</args></tool></tool_calls>',
  );
  assert.equal(parsed.content, "before");
  assert.deepEqual(parsed.invocations, [{ name: "readFile", args: { path: "README.md" } }]);
});

test("serializes proxy tool intent into native tool arguments", () => {
  const args = toolArgumentsJSON({
    name: "editFile",
    intent: "update title",
    args: { path: "PLAN.md", oldString: "old", newString: "new" },
  });
  assert.deepEqual(JSON.parse(args), {
    path: "PLAN.md",
    oldString: "old",
    newString: "new",
    intent: "update title",
  });
});

test("removes empty markdown fences left after parsing tool blocks", () => {
  const parsed = parseToolInvocationsFromText(
    'before\n```xml\n<tool_calls><tool name="readFile"><args>{"path":"README.md"}</args></tool></tool_calls>\n```',
  );
  assert.equal(parsed.content, "before");
  assert.deepEqual(parsed.invocations, [{ name: "readFile", args: { path: "README.md" } }]);
});

test("parses qwen-style JSON tool blocks", () => {
  const parsed = parseToolInvocationsFromText(
    '<tool_call>{"name":"shell","arguments":{"command":"go test ./..."}}</tool_call>',
  );
  assert.equal(parsed.content, "");
  assert.deepEqual(parsed.invocations, [
    { name: "shell", args: { command: "go test ./..." } },
  ]);
});

test("tool_choice restricts the allowed tool names", () => {
  const allowed = allowedToolNamesFromRequest(tools, {
    type: "function",
    function: { name: "shell" },
  });
  const invs = [
    { name: "readFile", args: { path: "README.md" } },
    { name: "shell", args: { command: "pwd" } },
  ];
  assert.deepEqual(filterInvocations(invs, allowed), [
    { name: "shell", args: { command: "pwd" } },
  ]);
});

test("tool_choice none disables native tools", () => {
  assert.equal(requestUsesNativeTools(tools, "none"), false);
  const allowed = allowedToolNamesFromRequest(tools, "none");
  assert.deepEqual(filterInvocations([{ name: "shell", args: {} }], allowed), []);
});

test("parallel_tool_calls false limits to one invocation", () => {
  const invs = [
    { name: "readFile", args: { path: "a" } },
    { name: "readFile", args: { path: "b" } },
  ];
  assert.deepEqual(limitInvocations(invs, false), [{ name: "readFile", args: { path: "a" } }]);
  assert.deepEqual(limitInvocations(invs, true), invs);
  assert.deepEqual(limitInvocations(invs, undefined), invs);
});

test("rejects empty editFile invocations before native tool emission", () => {
  const inv = {
    name: "editFile",
    args: { path: "PLAN.md", oldString: "", newString: "" },
    intent: "cursor edit",
  };
  assert.equal(isValidInvocation(inv), false);
  assert.deepEqual(filterInvocations([inv], null), []);
});

test("deduplicates cumulative cursor text snapshots", () => {
  let text = "";
  text += nextTextChunk(text, "ciao");
  text += nextTextChunk(text, "ciao mondo");
  text += nextTextChunk(text, "ciao mondo");
  text += nextTextChunk(text, "!");
  assert.equal(text, "ciao mondo!");
});

test("collapses repeated text inside a single cursor block", () => {
  assert.equal(collapseExactRepeat("ciaociao"), "ciao");
  assert.equal(collapseExactRepeat("ciao mondo"), "ciao mondo");
});

test("unwraps solomon MCP tool_call events into proxy tools", () => {
  const pending = [];
  let detected = false;
  let text = "";
  processStreamEvent(
    {
      type: "tool_call",
      name: "mcp",
      status: "running",
      args: {
        providerIdentifier: "solomon",
        toolName: "editFile",
        args: { path: "PLAN.md", oldString: "a", newString: "b", intent: "demo" },
      },
    } as any,
    false,
    (s) => { text += s; },
    () => {},
    pending,
    () => { detected = true; },
  );
  assert.equal(text, "");
  assert.equal(detected, true);
  assert.deepEqual(pending, [
    { name: "editFile", args: { path: "PLAN.md", oldString: "a", newString: "b" }, intent: "demo" },
  ]);
});

test("ignores MCP tool_call events from other providers", () => {
  const pending = [];
  let detected = false;
  const blocked: string[] = [];
  processStreamEvent(
    {
      type: "tool_call",
      name: "mcp",
      status: "running",
      args: { providerIdentifier: "other", toolName: "readFile", args: { path: "x" } },
    } as any,
    false,
    () => {},
    () => {},
    pending,
    () => { detected = true; },
    (name) => { blocked.push(name); },
  );
  assert.equal(detected, false);
  assert.deepEqual(pending, []);
  assert.deepEqual(blocked, ["mcp:external"]);
});

test("maps Cursor Task tool events into subagent", () => {
  const pending = [];
  let detected = false;
  processStreamEvent(
    {
      type: "assistant",
      message: {
        content: [{
          type: "tool_use",
          name: "Task",
          input: { description: "explore auth", prompt: "find login flow" },
        }],
      },
    } as any,
    false,
    () => {},
    () => {},
    pending,
    () => { detected = true; },
  );
  assert.equal(detected, true);
  assert.equal(pending[0]?.name, "subagent");
  assert.equal(pending[0]?.args.task, "find login flow");
  assert.equal(pending[0]?.args.sysPromptPath, ".solomon/cursor-task-sys.txt");
});

test("maps Cursor SemanticSearch tool events into find", () => {
  const pending = [];
  let detected = false;
  processStreamEvent(
    {
      type: "tool_call",
      name: "SemanticSearch",
      status: "running",
      args: { query: "auth middleware", target_directories: ["internal"] },
    } as any,
    false,
    () => {},
    () => {},
    pending,
    () => { detected = true; },
  );
  assert.equal(detected, true);
  assert.deepEqual(pending, [{
    name: "find",
    args: { pattern: "auth middleware", files: false, path: "internal" },
  }]);
});

test("maps Cursor Delete tool events into editFile delete", () => {
  const pending = [];
  let detected = false;
  processStreamEvent(
    {
      type: "tool_call",
      name: "Delete",
      status: "running",
      args: { path: "tmp/old.txt" },
    } as any,
    false,
    () => {},
    () => {},
    pending,
    () => { detected = true; },
  );
  assert.equal(detected, true);
  assert.deepEqual(pending, [{
    name: "editFile",
    args: { path: "tmp/old.txt", delete: true },
    intent: "delete file",
  }]);
});

test("accepts editFile delete invocations for native tool emission", () => {
  assert.equal(isValidInvocation({
    name: "editFile",
    args: { path: "x.go", delete: true, intent: "remove file" },
  }), true);
});

test("rejects editFile delete invocations with oldString or newString", () => {
  assert.equal(isValidInvocation({
    name: "editFile",
    args: { path: "x.go", delete: true, oldString: "a", intent: "remove file" },
  }), false);
  assert.equal(isValidInvocation({
    name: "editFile",
    args: { path: "x.go", delete: true, newString: "b", intent: "remove file" },
  }), false);
});

test("unwraps solomon MCP editFile delete tool calls", () => {
  const pending = [];
  let detected = false;
  processStreamEvent(
    {
      type: "tool_call",
      name: "mcp",
      status: "running",
      args: {
        providerIdentifier: "solomon",
        toolName: "editFile",
        args: { path: "tmp/old.txt", delete: true, intent: "cleanup" },
      },
    } as any,
    false,
    () => {},
    () => {},
    pending,
    () => { detected = true; },
  );
  assert.equal(detected, true);
  assert.deepEqual(pending, [{
    name: "editFile",
    args: { path: "tmp/old.txt", delete: true },
    intent: "cleanup",
  }]);
});

