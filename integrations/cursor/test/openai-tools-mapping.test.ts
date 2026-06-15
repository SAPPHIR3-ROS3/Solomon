import assert from "node:assert/strict";
import test from "node:test";
import { openAIToolsToMcpTools } from "../src/openai-tools.js";
import { processStreamEvent, proxyToolCorrectionMessage } from "../src/chat-helpers.js";
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

test("builds structured proxy correction message", () => {
  const msg = proxyToolCorrectionMessage(["Delete", "Task"], new Set(["readFile", "find"]));
  assert.ok(msg.includes("Delete"));
  assert.ok(msg.includes("find") && msg.includes("readFile"));
  assert.ok(msg.includes("Cursor built-in tools"));
  assert.ok(!msg.includes("[error]"));
  assert.ok(!msg.includes("solomon MCP"));
});

test("proxy correction suggests shell fallback when shell is allowed", () => {
  const msg = proxyToolCorrectionMessage(["ApplyPatch"], new Set(["readFile", "shell", "find"]));
  assert.ok(msg.includes("Default fallback"));
  assert.ok(msg.includes("Shell"));
});

test("proxy correction omits shell fallback when shell is not allowed", () => {
  const msg = proxyToolCorrectionMessage(["ApplyPatch"], new Set(["readFile", "find"]));
  assert.ok(!msg.includes("Default fallback"));
});

test("maps str_replace cursor alias into editFile", () => {
  const inv = bridgeToolInvocation(
    "str_replace",
    { path: "a.go", old_string: "foo", new_string: "bar" },
    bridgeCtx("editFile"),
  );
  assert.deepEqual(inv, {
    name: "editFile",
    args: { path: "a.go", oldString: "foo", newString: "bar" },
    intent: "edit file",
  });
});

test("maps ListDir cursor alias into find glob listing", () => {
  const inv = bridgeToolInvocation("ListDir", { path: "internal" }, bridgeCtx("find"));
  assert.deepEqual(inv, {
    name: "find",
    args: { pattern: "**/*", files: true, path: "internal" },
  });
});

test("maps ApplyPatch full content into editFile overwrite", () => {
  const inv = bridgeToolInvocation(
    "ApplyPatch",
    { path: "new.txt", patch: "hello world" },
    bridgeCtx("editFile"),
  );
  assert.deepEqual(inv, {
    name: "editFile",
    args: { path: "new.txt", oldString: "", newString: "hello world" },
    intent: "apply patch",
  });
});

test("rejects unified diff ApplyPatch as unmappable", () => {
  const inv = bridgeToolInvocation(
    "ApplyPatch",
    { path: "a.go", patch: "@@ -1,1 +1,1 @@\n-old\n+new" },
    bridgeCtx("editFile"),
  );
  assert.equal(inv, null);
});

test("harness tools clause steers cursor native bridge", () => {
  const clause = harnessToolsClause([
    { type: "function", function: { name: "readFile" } },
    { type: "function", function: { name: "shell" } },
  ]);
  assert.ok(clause.includes("Cursor built-in tools"));
  assert.ok(clause.includes("readFile, shell"));
  assert.ok(!clause.includes("solomon MCP"));
  assert.ok(!clause.includes("editFile to modify"));
});

test("maps disallowed cursor tool events into proxy tools", () => {
  const pending = [];
  let detected = false;
  let text = "";
  processStreamEvent(
    {
      type: "assistant",
      message: { content: [{ type: "tool_use", name: "Read", input: { path: "PLAN.md" } }] },
    } as any,
    false,
    (s) => { text += s; },
    () => {},
    pending,
    () => { detected = true; },
  );
  assert.equal(text, "");
  assert.equal(detected, true);
  assert.deepEqual(pending, [{ name: "readFile", args: { path: "PLAN.md" } }]);
});

test("converts OpenAI request tools into MCP tool definitions (legacy helper)", () => {
  const mcp = openAIToolsToMcpTools([
    {
      type: "function",
      function: {
        name: "find",
        description: "Search files",
        parameters: {
          type: "object",
          properties: { pattern: { type: "string" } },
          required: ["pattern"],
        },
      },
    },
    {
      type: "function",
      function: { name: "find", description: "duplicate" },
    },
    {
      type: "function",
      function: { name: "subagent", description: "Run nested agent" },
    },
  ]);
  assert.equal(mcp.length, 2);
  assert.equal(mcp[0]?.name, "find");
  assert.equal(mcp[0]?.description, "Search files");
  assert.deepEqual(mcp[0]?.inputSchema.required, ["pattern"]);
  assert.equal(mcp[1]?.name, "subagent");
});

test("passes through any allowed Solomon tool without manual mapping", () => {
  const ctx = bridgeCtx("loadSkill", "searchSkill");
  const inv = bridgeToolInvocation("loadSkill", { name: "babysit" }, ctx);
  assert.deepEqual(inv, { name: "loadSkill", args: { name: "babysit" } });
});

test("maps WebFetch cursor alias to fetchWeb when allowed", () => {
  const ctx = bridgeCtx("fetchWeb");
  const inv = bridgeToolInvocation("WebFetch", { url: "https://example.com" }, ctx);
  assert.deepEqual(inv, { name: "fetchWeb", args: { url: "https://example.com" } });
});

test("rejects tools not in the allowed set", () => {
  const ctx = bridgeCtx("readFile");
  assert.equal(bridgeToolInvocation("loadSkill", { name: "x" }, ctx), null);
  assert.equal(bridgeToolInvocation("WebFetch", { url: "https://x" }, ctx), null);
});

test("processStreamEvent respects allowed tool set for cursor aliases", () => {
  const pending = [];
  let detected = false;
  const blocked: string[] = [];
  processStreamEvent(
    {
      type: "tool_call",
      name: "WebSearch",
      status: "running",
      args: { query: "solomon mcp" },
    } as any,
    false,
    () => {},
    () => {},
    pending,
    () => { detected = true; },
    (name) => { blocked.push(name); },
    bridgeCtx("readFile"),
  );
  assert.equal(detected, false);
  assert.deepEqual(pending, []);
  assert.deepEqual(blocked, ["WebSearch"]);
});

test("internal tools mode still bridges mappable cursor tools to Solomon", () => {
  const pending = [];
  const native: CursorNativeToolEvent[] = [];
  let detected = false;
  processStreamEvent(
    {
      type: "tool_call",
      name: "Read",
      status: "running",
      args: { path: "main.go" },
    } as any,
    true,
    () => {},
    () => {},
    pending,
    () => { detected = true; },
    undefined,
    { allowedNames: null },
    (ev) => { native.push(ev); },
  );
  assert.equal(detected, true);
  assert.deepEqual(pending, [{ name: "readFile", args: { path: "main.go" } }]);
  assert.deepEqual(native, []);
});

test("internal tools mode emits formatted events for unmapped tools", () => {
  const pending = [];
  const native: CursorNativeToolEvent[] = [];
  let detected = false;
  processStreamEvent(
    {
      type: "tool_call",
      name: "BrowserNavigate",
      status: "running",
      args: { url: "https://example.com" },
    } as any,
    true,
    () => {},
    () => {},
    pending,
    () => { detected = true; },
    undefined,
    { allowedNames: null },
    (ev) => { native.push(ev); },
  );
  assert.equal(detected, false);
  assert.deepEqual(pending, []);
  assert.equal(native.length, 1);
  assert.equal(native[0]?.name, "BrowserNavigate");
  assert.equal(native[0]?.displayLine, "https://example.com");
});

test("internal tools mode forwards completed unmapped tool results with displayLine", () => {
  const native: CursorNativeToolEvent[] = [];
  processStreamEvent(
    {
      type: "tool_call",
      name: "BrowserNavigate",
      status: "completed",
      args: { url: "https://example.com" },
      result: { title: "Example Domain" },
    } as any,
    true,
    () => {},
    () => {},
    [],
    () => {},
    undefined,
    { allowedNames: null },
    (ev) => { native.push(ev); },
  );
  assert.equal(native.length, 1);
  assert.equal(native[0]?.status, "completed");
  assert.equal(native[0]?.displayLine, '{"title":"Example Domain"}');
});

test("unwraps solomon MCP find and subagent tool calls", () => {
  for (const [toolName, args, expected] of [
    ["find", { pattern: "foo", files: false }, { name: "find", args: { pattern: "foo", files: false } }],
    ["subagent", { task: "explore auth", sysPromptPath: "agent.tmpl" }, { name: "subagent", args: { task: "explore auth", sysPromptPath: "agent.tmpl" }, intent: "nested task" }],
  ] as const) {
    const pending = [];
    let detected = false;
    processStreamEvent(
      {
        type: "tool_call",
        name: "mcp",
        status: "running",
        args: { providerIdentifier: "solomon", toolName, args },
      } as any,
      false,
      () => {},
      () => {},
      pending,
      () => { detected = true; },
    );
    assert.equal(detected, true, toolName);
    assert.deepEqual(pending, [expected], toolName);
  }
});
