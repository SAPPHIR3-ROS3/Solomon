import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import assert from "node:assert/strict";
import test from "node:test";
import { openAIToolsToMcpTools } from "../src/openai-tools.js";
import { processStreamEvent, proxyToolCorrectionMessage, nativeInvocationsFromText } from "../src/chat-helpers.js";
import { bridgeToolInvocation, mapCursorToolInvocation } from "../src/legacy.js";
import {
  hardDenyCorrectionHint,
  isBrowserCursorTool,
  shouldBlockDeferredSolomonTool,
  shouldHardDenyCursorTool,
  shouldRedirectCursorTool,
  shouldStopProxyOnBlockedTool,
} from "../src/tool-policy.js";
import { harnessToolsClause, harnessPreamble, harnessToolCatalog, sortToolNamesForHarness } from "../src/harness-prompt.js";
import { DEFAULT_SUBAGENT_SYS_PROMPT, ensureDefaultSubagentSysPrompt } from "../src/cursor-agent.js";
import { DEFAULT_SUBAGENT_SYS_PATH } from "../src/legacy.js";
import type { CursorNativeToolEvent } from "../src/cursor-native-tools.js";
import { finalizeTurnToolResults } from "../src/chat/turn.js";
import { createAgentToolStreamState, drainAgentToolStream, shouldForceStopProxyRun } from "../src/chat/helpers/stream-loop.js";
import { forceStopRun } from "../src/run-control.js";
import type { ChatCompletionTool } from "../src/openai-types.js";

function bridgeCtx(...names: string[]) {
  return { allowedNames: new Set(names) };
}

const tools: ChatCompletionTool[] = [
  { type: "function", function: { name: "readFile" } },
  { type: "function", function: { name: "shell" } },
];

test("builds orchestrate-first proxy correction message (2.5)", () => {
  const msg = proxyToolCorrectionMessage(["Read", "Shell"], new Set(["orchestrate", "searchTools"]));
  assert.ok(msg.includes("Read"));
  assert.ok(msg.includes("orchestrate"));
  assert.ok(msg.includes("searchTools"));
  assert.ok(msg.includes("sdk.ReadFile"));
  assert.ok(msg.includes("sdk.Shell"));
  assert.ok(msg.includes("Blocked by Solomon proxy"));
  assert.ok(!msg.includes("Default fallback"));
  assert.ok(!msg.includes("Prefer Read"));
  assert.ok(!msg.match(/\buse (Read|StrReplace|Shell|Task)\b/));
  assert.ok(!msg.includes("readFile, editFile"));
});

test("redirect correction maps Task to native subagent (2.5)", () => {
  const msg = proxyToolCorrectionMessage(["Task"], new Set(["subagent", "orchestrate"]));
  assert.ok(msg.includes("native subagent"));
  assert.ok(!msg.match(/\buse Task\b/));
});

test("redirect correction maps StrReplace to orchestrate SDK (2.5)", () => {
  const msg = proxyToolCorrectionMessage(["StrReplace"], new Set(["orchestrate", "searchTools"]));
  assert.ok(msg.includes("sdk.ReplaceInFile"));
  assert.ok(!msg.match(/\buse StrReplace\b/));
});

test("deferred MCP blocks get orchestrate guidance (2.5)", () => {
  const msg = proxyToolCorrectionMessage(["mcp:editFile"], new Set(["orchestrate", "searchTools"]));
  assert.ok(msg.includes("searchTools"));
  assert.ok(msg.includes("orchestrate"));
  assert.ok(!msg.includes("editFile via function"));
});

test("policy marks cursor built-ins for redirect block (2.3)", () => {
  for (const name of ["Read", "StrReplace", "Shell", "Grep", "Task", "WebSearch", "TodoWrite"]) {
    assert.equal(shouldRedirectCursorTool(name), true, name);
  }
  assert.equal(shouldRedirectCursorTool("orchestrate"), false);
  assert.equal(shouldBlockDeferredSolomonTool("readFile"), true);
  assert.equal(shouldBlockDeferredSolomonTool("subagent"), false);
  assert.equal(shouldStopProxyOnBlockedTool("mcp:editFile"), true);
  assert.equal(shouldStopProxyOnBlockedTool("mcp:external"), true);
});

test("policy hard-denies cursor-only tools (2.4)", () => {
  for (const name of [
    "AskQuestion",
    "GenerateImage",
    "Await",
    "ApplyPatch",
    "browser_navigate",
    "browser_click",
    "BrowserNavigate",
  ]) {
    assert.equal(shouldHardDenyCursorTool(name), true, name);
    assert.equal(shouldRedirectCursorTool(name), false, name);
    assert.equal(shouldStopProxyOnBlockedTool(name), true, name);
  }
  assert.equal(isBrowserCursorTool("browser_tabs"), true);
  assert.equal(isBrowserCursorTool("Read"), false);
  assert.equal(hardDenyCorrectionHint("AskQuestion"), "Ask the user in plain text instead of AskQuestion.");
  assert.equal(hardDenyCorrectionHint("mcp:external")?.includes("External MCP"), true);
  assert.equal(hardDenyCorrectionHint("ApplyPatch")?.includes("orchestrate"), true);
});

test("bridgeToolInvocation blocks redirect tools even when mapped target is allowed (2.3)", () => {
  assert.equal(
    bridgeToolInvocation("Read", { path: "main.go" }, bridgeCtx("readFile", "orchestrate")),
    null,
  );
  assert.deepEqual(
    mapCursorToolInvocation("Read", { path: "main.go" }, bridgeCtx("readFile")),
    { name: "readFile", args: { path: "main.go" } },
  );
});

test("maps str_replace cursor alias into editFile via mapper", () => {
  const inv = mapCursorToolInvocation(
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

test("maps ListDir cursor alias into find glob listing via mapper", () => {
  const inv = mapCursorToolInvocation("ListDir", { path: "internal" }, bridgeCtx("find"));
  assert.deepEqual(inv, {
    name: "find",
    args: { pattern: "**/*", files: true, path: "internal" },
  });
});

test("hard-denies ApplyPatch instead of bridging to editFile (2.4)", () => {
  assert.equal(
    bridgeToolInvocation(
      "ApplyPatch",
      { path: "new.txt", patch: "hello world" },
      bridgeCtx("editFile", "orchestrate"),
    ),
    null,
  );
  assert.equal(
    mapCursorToolInvocation(
      "ApplyPatch",
      { path: "new.txt", patch: "hello world" },
      bridgeCtx("editFile"),
    ),
    null,
  );
});

test("default subagent sys prompt is orchestrate-first (2.7)", () => {
  assert.ok(DEFAULT_SUBAGENT_SYS_PROMPT.includes("orchestrate"));
  assert.ok(DEFAULT_SUBAGENT_SYS_PROMPT.includes("searchTools"));
  assert.ok(DEFAULT_SUBAGENT_SYS_PROMPT.includes("subagent"));
  assert.ok(!DEFAULT_SUBAGENT_SYS_PROMPT.includes("Use normal Cursor built-in tools"));
  assert.ok(!DEFAULT_SUBAGENT_SYS_PROMPT.match(/\buse (Read|StrReplace|Shell|Task)\b/i));
});

test("ensureDefaultSubagentSysPrompt writes default only when missing (2.7)", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "solomon-subagent-sys-"));
  try {
    const target = path.join(root, DEFAULT_SUBAGENT_SYS_PATH);
    ensureDefaultSubagentSysPrompt(root);
    assert.ok(fs.existsSync(target));
    const written = fs.readFileSync(target, "utf8");
    assert.ok(written.includes("orchestrate"));
    assert.ok(written.includes("searchTools"));
    fs.writeFileSync(target, "custom user prompt\n", "utf8");
    ensureDefaultSubagentSysPrompt(root);
    assert.equal(fs.readFileSync(target, "utf8"), "custom user prompt\n");
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
});

test("harness tools clause describes Solomon native tools (2.6)", () => {
  const nativeTools: ChatCompletionTool[] = [
    { type: "function", function: { name: "orchestrate", description: "Run Go WASM script" } },
    { type: "function", function: { name: "searchTools", description: "Discover deferred tools" } },
    { type: "function", function: { name: "subagent", description: "Nested agent run" } },
    { type: "function", function: { name: "readFile", description: "Deferred read" } },
  ];
  const clause = harnessToolsClause(nativeTools);
  assert.ok(clause.startsWith("[Harness] Session tools: searchTools, orchestrate, subagent"));
  assert.ok(clause.includes("searchTools (discover deferred"));
  assert.ok(clause.includes("orchestrate SDK use only"));
  assert.ok(clause.includes("<tool_calls> XML"));
  assert.ok(!clause.includes("Cursor built-in"));
  assert.ok(!clause.includes("Prefer Read"));
  assert.ok(!clause.includes("Shell fallback"));
  assert.ok(!clause.match(/\buse (Read|StrReplace|Shell|Task)\b/));
});

test("harness preamble is orchestrate-first with blocked cursor surfaces (2.6)", () => {
  const tools: ChatCompletionTool[] = [
    { type: "function", function: { name: "orchestrate", description: "Run Go WASM script" } },
    { type: "function", function: { name: "searchTools", description: "Discover deferred tools" } },
    { type: "function", function: { name: "switchMode", description: "Switch agent/chat" } },
  ];
  const preamble = harnessPreamble(tools);
  assert.ok(preamble.includes("[Harness] Workflow (orchestrate-first)"));
  assert.ok(preamble.includes("searchTools"));
  assert.ok(preamble.includes("switchMode"));
  assert.ok(preamble.includes("browser_*"));
  assert.ok(preamble.includes("ApplyPatch"));
  assert.ok(preamble.includes("[Harness] Session tools"));
  assert.ok(preamble.includes("[Harness] Tool catalog"));
  assert.ok(preamble.includes("- searchTools: Discover deferred tools"));
  assert.ok(preamble.includes("- orchestrate: Run Go WASM script"));
  assert.ok(!preamble.includes("prefer Read for inspection"));
  assert.ok(!preamble.includes("Use normal Cursor built-in tools"));
  assert.ok(!preamble.includes("Prefer Read for inspection"));
  assert.ok(!preamble.match(/\bcall your available tools normally\b/));
});

test("harness tool catalog deduplicates and orders native entry tools first (2.6)", () => {
  const catalog = harnessToolCatalog([
    { type: "function", function: { name: "readFile", description: "Deferred" } },
    { type: "function", function: { name: "orchestrate", description: "A" } },
    { type: "function", function: { name: "searchTools", description: "B" } },
    { type: "function", function: { name: "orchestrate", description: "duplicate" } },
  ]);
  assert.equal((catalog.match(/- orchestrate:/g) ?? []).length, 1);
  assert.ok(catalog.indexOf("- searchTools:") < catalog.indexOf("- orchestrate:"));
  assert.ok(catalog.indexOf("- orchestrate:") < catalog.indexOf("- readFile:"));
  assert.deepEqual(
    sortToolNamesForHarness(["readFile", "orchestrate", "searchTools", "shell"]),
    ["searchTools", "orchestrate", "readFile", "shell"],
  );
});

test("blocks Read cursor tool instead of bridging to readFile (2.3)", () => {
  const pending = [];
  let detected = false;
  const blocked: string[] = [];
  processStreamEvent(
    {
      type: "assistant",
      message: { content: [{ type: "tool_use", name: "Read", input: { path: "PLAN.md" } }] },
    } as any,
    false,
    () => {},
    () => {},
    pending,
    () => { detected = true; },
    (name) => { blocked.push(name); },
    bridgeCtx("readFile", "orchestrate"),
  );
  assert.equal(detected, true);
  assert.deepEqual(pending, []);
  assert.deepEqual(blocked, ["Read"]);
});

test("native XML blocks deferred readFile invocations (2.3)", () => {
  const parsed = nativeInvocationsFromText(
    '<tool_calls><tool name="readFile"><args>{"path":"main.go"}</args></tool></tool_calls>',
    { allowedNames: new Set(["orchestrate", "searchTools"]), parallelToolCalls: true },
  );
  assert.deepEqual(parsed.invocations, []);
  assert.deepEqual(parsed.blockedTools, ["readFile"]);
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

test("maps WebFetch cursor alias to fetchWeb via mapper when allowed", () => {
  const ctx = bridgeCtx("fetchWeb");
  const inv = mapCursorToolInvocation("WebFetch", { url: "https://example.com" }, ctx);
  assert.deepEqual(inv, { name: "fetchWeb", args: { url: "https://example.com" } });
  assert.equal(bridgeToolInvocation("WebFetch", { url: "https://example.com" }, ctx), null);
});

test("rejects tools not in the allowed set", () => {
  const ctx = bridgeCtx("readFile");
  assert.equal(bridgeToolInvocation("loadSkill", { name: "x" }, ctx), null);
  assert.equal(bridgeToolInvocation("WebFetch", { url: "https://x" }, ctx), null);
});

test("processStreamEvent blocks redirect cursor aliases (2.3)", () => {
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
    bridgeCtx("orchestrate", "searchTools"),
  );
  assert.equal(detected, true);
  assert.deepEqual(pending, []);
  assert.deepEqual(blocked, ["WebSearch"]);
});

test("internal tools mode still blocks redirect cursor tools (2.3)", () => {
  const pending = [];
  const native: CursorNativeToolEvent[] = [];
  let detected = false;
  const blocked: string[] = [];
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
    (name) => { blocked.push(name); },
    { allowedNames: null },
    (ev) => { native.push(ev); },
  );
  assert.equal(detected, true);
  assert.deepEqual(pending, []);
  assert.deepEqual(blocked, ["Read"]);
  assert.deepEqual(native, []);
});

test("hard deny proxy correction omits orchestrate footer (2.4)", () => {
  const msg = proxyToolCorrectionMessage(["AskQuestion"], new Set(["orchestrate"]));
  assert.ok(msg.includes("plain text"));
  assert.ok(!msg.includes("never Cursor built-ins"));
});

test("internal tools mode still hard-blocks browser tools (2.4)", () => {
  const pending = [];
  const native: CursorNativeToolEvent[] = [];
  let detected = false;
  const blocked: string[] = [];
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
    (name) => { blocked.push(name); },
    { allowedNames: null },
    (ev) => { native.push(ev); },
  );
  assert.equal(detected, true);
  assert.deepEqual(pending, []);
  assert.deepEqual(blocked, ["BrowserNavigate"]);
  assert.deepEqual(native, []);
});

test("blocks hard-deny tools on stream path (2.4)", () => {
  for (const [name, args] of [
    ["AskQuestion", { questions: [{ id: "q1", prompt: "Pick one", options: [{ id: "a", label: "A" }] }] }],
    ["GenerateImage", { description: "icon", filename: "icon.png" }],
    ["Await", { task_id: "bg-1" }],
    ["browser_navigate", { url: "https://example.com" }],
  ] as const) {
    const pending = [];
    let detected = false;
    const blocked: string[] = [];
    processStreamEvent(
      {
        type: "tool_call",
        name,
        status: "running",
        args,
      } as any,
      false,
      () => {},
      () => {},
      pending,
      () => { detected = true; },
      (name) => { blocked.push(name); },
      bridgeCtx("orchestrate"),
    );
    assert.equal(detected, true, name);
    assert.deepEqual(pending, [], name);
    assert.deepEqual(blocked, [name], name);
  }
});

test("native XML blocks hard-deny tool names (2.4)", () => {
  const parsed = nativeInvocationsFromText(
    '<tool_calls><tool name="GenerateImage"><args>{"description":"logo"}</args></tool></tool_calls>',
    { allowedNames: new Set(["orchestrate"]), parallelToolCalls: true },
  );
  assert.deepEqual(parsed.invocations, []);
  assert.deepEqual(parsed.blockedTools, ["GenerateImage"]);
});

test("internal tools mode forwards completed unmapped non-browser tool results (2.4)", () => {
  const native: CursorNativeToolEvent[] = [];
  processStreamEvent(
    {
      type: "tool_call",
      name: "SwitchMode",
      status: "completed",
      args: { target_mode_id: "plan" },
      result: { ok: true },
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
  assert.equal(native[0]?.displayLine, '{"ok":true}');
});

test("completed browser tool events stay blocked (2.4)", () => {
  const native: CursorNativeToolEvent[] = [];
  const blocked: string[] = [];
  processStreamEvent(
    {
      type: "tool_call",
      name: "browser_navigate",
      status: "completed",
      args: { url: "https://example.com" },
      result: { title: "Example Domain" },
    } as any,
    true,
    () => {},
    () => {},
    [],
    () => {},
    (name) => { blocked.push(name); },
    { allowedNames: null },
    (ev) => { native.push(ev); },
  );
  assert.deepEqual(blocked, ["browser_navigate"]);
  assert.deepEqual(native, []);
});

test("blocks deferred solomon MCP tools and passes native subagent (2.3)", () => {
  for (const [toolName, args, expectBlocked] of [
    ["find", { pattern: "foo", files: false }, "mcp:find"],
    ["subagent", { task: "explore auth", sysPromptPath: "agent.tmpl" }, null],
  ] as const) {
    const pending = [];
    let detected = false;
    const blocked: string[] = [];
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
      (name) => { blocked.push(name); },
      bridgeCtx("orchestrate", "searchTools", "subagent"),
    );
    assert.equal(detected, true, toolName);
    if (expectBlocked) {
      assert.deepEqual(pending, [], toolName);
      assert.deepEqual(blocked, [expectBlocked], toolName);
    } else {
      assert.deepEqual(blocked, [], toolName);
      assert.equal(pending[0]?.name, "subagent", toolName);
    }
  }
});

const nativeTurnOpts = {
  nativeTools: true,
  allowedNames: new Set(["orchestrate", "searchTools", "subagent"]),
  parallelToolCalls: true,
};

test("finalizeTurnToolResults applies redirect correction on blocked cursor tools (2.10)", () => {
  const stream = finalizeTurnToolResults(
    { pendingBridged: [], blockedTools: ["Read", "Shell"] },
    "I'll inspect the repo.",
    nativeTurnOpts,
  );
  assert.deepEqual(stream.bridged, []);
  assert.ok(stream.proxyCorrection?.includes("Blocked by Solomon proxy: Read, Shell"));
  assert.ok(stream.proxyCorrection?.includes("sdk.ReadFile"));
  assert.ok(stream.proxyCorrection?.includes("sdk.Shell"));
  const nonstream = finalizeTurnToolResults(
    { pendingBridged: [], blockedTools: ["Read", "Shell"] },
    "I'll inspect the repo.",
    nativeTurnOpts,
  );
  assert.equal(stream.proxyCorrection, nonstream.proxyCorrection);
});

test("finalizeTurnToolResults hard-denies without orchestrate footer (2.10)", () => {
  const result = finalizeTurnToolResults(
    { pendingBridged: [], blockedTools: ["AskQuestion"] },
    "",
    nativeTurnOpts,
  );
  assert.ok(result.proxyCorrection?.includes("plain text instead of AskQuestion"));
  assert.ok(!result.proxyCorrection?.includes("never Cursor built-ins"));
});

test("finalizeTurnToolResults passes native orchestrate without correction (2.10)", () => {
  const xml = '<tool_calls><tool name="orchestrate"><args>{"code":"package main"}</args></tool></tool_calls>';
  const result = finalizeTurnToolResults(
    { pendingBridged: [], blockedTools: [] },
    xml,
    nativeTurnOpts,
  );
  assert.equal(result.bridged.length, 1);
  assert.equal(result.bridged[0]?.name, "orchestrate");
  assert.equal(result.proxyCorrection, undefined);
});

test("finalizeTurnToolResults merges SDK bridged and native XML invocations (2.10)", () => {
  const result = finalizeTurnToolResults(
    {
      pendingBridged: [{ name: "subagent", args: { task: "explore auth" } }],
      blockedTools: [],
    },
    '<tool_calls><tool name="orchestrate"><args>{"code":"package main"}</args></tool></tool_calls>',
    nativeTurnOpts,
  );
  assert.equal(result.bridged.length, 2);
  assert.deepEqual(result.bridged.map((inv) => inv.name).sort(), ["orchestrate", "subagent"]);
  assert.equal(result.proxyCorrection, undefined);
});

test("drainAgentToolStream blocks redirect tools for stream and nonstream paths (2.10)", async () => {
  async function* events() {
    yield {
      type: "tool_call",
      name: "StrReplace",
      status: "running",
      args: { path: "main.go", old_string: "a", new_string: "b" },
    };
    yield { type: "assistant", message: { content: [{ type: "text", text: "should not arrive" }] } };
  }
  let cancelled = false;
  const run = {
    stream: () => events(),
    supports: () => true,
    cancel: async () => { cancelled = true; },
  };
  const state = createAgentToolStreamState();
  let text = "";
  await drainAgentToolStream(
    run as any,
    false,
    bridgeCtx("orchestrate", "searchTools"),
    { onText: (t) => { text += t; }, onThinking: () => {} },
    state,
  );
  assert.equal(cancelled, true);
  assert.deepEqual(state.blockedTools, ["StrReplace"]);
  assert.deepEqual(state.pendingBridged, []);
  assert.equal(state.toolDetected, true);
  assert.equal(text, "");
  const finalized = finalizeTurnToolResults(state, text, nativeTurnOpts);
  assert.ok(finalized.proxyCorrection?.includes("sdk.ReplaceInFile"));
});

async function drainMockRun(
  events: unknown[],
  bridgeNames: string[],
  allowCursorInternalTools = false,
) {
  let cancelled = false;
  let cancelCalls = 0;
  const run = {
    stream: async function* () {
      for (const event of events) {
        yield event;
      }
    },
    supports: (cap: string) => cap === "cancel",
    cancel: async () => {
      cancelled = true;
      cancelCalls += 1;
    },
  };
  const state = createAgentToolStreamState();
  let text = "";
  await drainAgentToolStream(
    run as any,
    allowCursorInternalTools,
    bridgeCtx(...bridgeNames),
    { onText: (t) => { text += t; }, onThinking: () => {} },
    state,
  );
  return { cancelled, cancelCalls, state, text };
}

test("shouldForceStopProxyRun covers bridged and blocked states (2.11)", () => {
  assert.equal(shouldForceStopProxyRun({ pendingBridged: [], blockedTools: [], toolDetected: false }), false);
  assert.equal(
    shouldForceStopProxyRun({
      pendingBridged: [{ name: "subagent", args: { task: "x" } }],
      blockedTools: [],
      toolDetected: true,
    }),
    true,
  );
  assert.equal(
    shouldForceStopProxyRun({ pendingBridged: [], blockedTools: ["Read"], toolDetected: true }),
    true,
  );
  assert.equal(
    shouldForceStopProxyRun({ pendingBridged: [], blockedTools: ["readFile"], toolDetected: false }),
    true,
  );
});

test("forceStopRun calls run.cancel when supported (2.11)", async () => {
  let cancelled = false;
  const run = {
    supports: (cap: string) => cap === "cancel",
    cancel: async () => { cancelled = true; },
  };
  await forceStopRun(run as any);
  assert.equal(cancelled, true);
  cancelled = false;
  await forceStopRun({ supports: () => false, cancel: async () => { cancelled = true; } } as any);
  assert.equal(cancelled, false);
});

test("drainAgentToolStream forceStopRun on bridged native subagent (2.11)", async () => {
  const { cancelled, cancelCalls, state, text } = await drainMockRun(
    [
      {
        type: "tool_call",
        name: "mcp",
        status: "running",
        args: { providerIdentifier: "solomon", toolName: "subagent", args: { task: "explore auth" } },
      },
      { type: "assistant", message: { content: [{ type: "text", text: "late text" }] } },
    ],
    ["orchestrate", "searchTools", "subagent"],
  );
  assert.equal(cancelled, true);
  assert.equal(cancelCalls, 1);
  assert.equal(state.pendingBridged[0]?.name, "subagent");
  assert.deepEqual(state.blockedTools, []);
  assert.equal(text, "");
});

test("drainAgentToolStream forceStopRun on hard-denied AskQuestion (2.11)", async () => {
  const { cancelled, state, text } = await drainMockRun(
    [
      {
        type: "tool_call",
        name: "AskQuestion",
        status: "running",
        args: { questions: [{ id: "q1", prompt: "Pick", options: [{ id: "a", label: "A" }] }] },
      },
      { type: "assistant", message: { content: [{ type: "text", text: "late text" }] } },
    ],
    ["orchestrate"],
  );
  assert.equal(cancelled, true);
  assert.deepEqual(state.blockedTools, ["AskQuestion"]);
  assert.deepEqual(state.pendingBridged, []);
  assert.equal(text, "");
});

test("drainAgentToolStream forceStopRun on deferred readFile direct call (2.11)", async () => {
  const { cancelled, state } = await drainMockRun(
    [
      {
        type: "tool_call",
        name: "readFile",
        status: "running",
        args: { path: "main.go" },
      },
    ],
    ["orchestrate", "readFile"],
  );
  assert.equal(cancelled, true);
  assert.deepEqual(state.blockedTools, ["readFile"]);
  assert.deepEqual(state.pendingBridged, []);
});
