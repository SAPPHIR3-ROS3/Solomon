import assert from "node:assert/strict";
import test from "node:test";
import { proxyToolCorrectionMessage, processStreamEvent } from "../src/chat-helpers.js";
import { bridgeToolInvocation, unwrapSolomonMcpCall } from "../src/legacy.js";
import {
  correctionHintForBlockedTool,
  isHardDenyBlockedLabel,
  shouldBlockDeferredSolomonTool,
  shouldHardDenyCursorTool,
  shouldRedirectCursorTool,
  shouldStopProxyOnBlockedTool,
} from "../src/tool-policy.js";

const NATIVE_ALLOW = [
  "orchestrate",
  "searchTools",
  "subagent",
  "switchMode",
  "searchSkill",
  "loadSkill",
] as const;

type PolicyClass = "native" | "redirect" | "hardDeny" | "deferredBlock";

type PolicyMatrixRow = {
  id: string;
  tool: string;
  class: PolicyClass;
  args?: unknown;
  blockedLabel?: string;
  hintIncludes: string;
  orchestrateFooter: boolean;
};

function bridgeCtx(...names: string[]) {
  return { allowedNames: new Set(names) };
}

const defaultArgs: Record<string, unknown> = {
  Read: { path: "main.go" },
  Write: { path: "main.go", contents: "x" },
  StrReplace: { path: "main.go", old_string: "a", new_string: "b" },
  Delete: { path: "main.go" },
  Shell: { command: "go test ./..." },
  Grep: { pattern: "func", path: "." },
  Glob: { glob_pattern: "**/*.go" },
  SemanticSearch: { query: "auth", target_directories: ["internal"] },
  ListDir: { target_directory: "." },
  ReadLints: { paths: ["main.go"] },
  EditNotebook: { target_notebook: "nb.ipynb", cell_idx: 0, is_new_cell: true, new_string: "x" },
  TodoWrite: { todos: [{ id: "1", content: "task", status: "pending" }] },
  Task: { description: "explore", prompt: "find auth" },
  CallMcpTool: { server: "x", toolName: "y", arguments: {} },
  WebFetch: { url: "https://example.com" },
  WebSearch: { search_term: "solomon" },
  AskQuestion: { questions: [{ id: "q1", prompt: "Pick", options: [{ id: "a", label: "A" }] }] },
  GenerateImage: { description: "icon", filename: "icon.png" },
  Await: { task_id: "bg-1" },
  ApplyPatch: { patch: "*** Begin Patch\n*** End Patch" },
  browser_navigate: { url: "https://example.com" },
  orchestrate: { code: "package main" },
  searchTools: { query: "shell" },
  subagent: { task: "explore auth" },
  switchMode: { target_mode_id: "chat" },
  searchSkill: { query: "babysit" },
  loadSkill: { name: "babysit" },
  readFile: { path: "main.go" },
  editFile: { path: "main.go", oldString: "a", newString: "b" },
  shell: { command: "go test" },
  find: { pattern: "main" },
};

const POLICY_MATRIX: PolicyMatrixRow[] = [
  { id: "native-orchestrate", tool: "orchestrate", class: "native", hintIncludes: "", orchestrateFooter: false },
  { id: "native-searchTools", tool: "searchTools", class: "native", hintIncludes: "", orchestrateFooter: false },
  { id: "native-subagent", tool: "subagent", class: "native", hintIncludes: "", orchestrateFooter: false },
  { id: "native-switchMode", tool: "switchMode", class: "native", hintIncludes: "", orchestrateFooter: false },
  { id: "native-searchSkill", tool: "searchSkill", class: "native", hintIncludes: "", orchestrateFooter: false },
  { id: "native-loadSkill", tool: "loadSkill", class: "native", hintIncludes: "", orchestrateFooter: false },
  { id: "redirect-Read", tool: "Read", class: "redirect", hintIncludes: "sdk.ReadFile", orchestrateFooter: true },
  { id: "redirect-Write", tool: "Write", class: "redirect", hintIncludes: "sdk.WriteFile", orchestrateFooter: true },
  { id: "redirect-StrReplace", tool: "StrReplace", class: "redirect", hintIncludes: "sdk.ReplaceInFile", orchestrateFooter: true },
  { id: "redirect-Delete", tool: "Delete", class: "redirect", hintIncludes: "sdk.DeleteFile", orchestrateFooter: true },
  { id: "redirect-Shell", tool: "Shell", class: "redirect", hintIncludes: "sdk.Shell", orchestrateFooter: true },
  { id: "redirect-Grep", tool: "Grep", class: "redirect", hintIncludes: "sdk.Glob", orchestrateFooter: true },
  { id: "redirect-Glob", tool: "Glob", class: "redirect", hintIncludes: "sdk.Glob", orchestrateFooter: true },
  { id: "redirect-SemanticSearch", tool: "SemanticSearch", class: "redirect", hintIncludes: "sdk.Glob", orchestrateFooter: true },
  { id: "redirect-ListDir", tool: "ListDir", class: "redirect", hintIncludes: "sdk.Glob", orchestrateFooter: true },
  { id: "redirect-ReadLints", tool: "ReadLints", class: "redirect", hintIncludes: "orchestrate", orchestrateFooter: true },
  { id: "redirect-EditNotebook", tool: "EditNotebook", class: "redirect", hintIncludes: "orchestrate", orchestrateFooter: true },
  { id: "redirect-TodoWrite", tool: "TodoWrite", class: "redirect", hintIncludes: "addTodo", orchestrateFooter: true },
  { id: "redirect-Task", tool: "Task", class: "redirect", hintIncludes: "native subagent", orchestrateFooter: true },
  { id: "redirect-CallMcpTool", tool: "CallMcpTool", class: "redirect", hintIncludes: "searchTools", orchestrateFooter: true },
  { id: "redirect-WebFetch", tool: "WebFetch", class: "redirect", hintIncludes: "sdk.FetchWeb", orchestrateFooter: true },
  { id: "redirect-WebSearch", tool: "WebSearch", class: "redirect", hintIncludes: "sdk.WebSearch", orchestrateFooter: true },
  { id: "redirect-mcp-deferred", tool: "mcp:editFile", class: "redirect", blockedLabel: "mcp:editFile", hintIncludes: "searchTools", orchestrateFooter: true },
  { id: "deferred-readFile", tool: "readFile", class: "deferredBlock", hintIncludes: "", orchestrateFooter: false },
  { id: "deferred-editFile", tool: "editFile", class: "deferredBlock", hintIncludes: "", orchestrateFooter: false },
  { id: "deferred-shell", tool: "shell", class: "deferredBlock", hintIncludes: "sdk.Shell", orchestrateFooter: true },
  { id: "hardDeny-AskQuestion", tool: "AskQuestion", class: "hardDeny", hintIncludes: "plain text", orchestrateFooter: false },
  { id: "hardDeny-browser", tool: "browser_navigate", class: "hardDeny", hintIncludes: "browser tools", orchestrateFooter: false },
  { id: "hardDeny-mcp-external", tool: "mcp:external", class: "hardDeny", blockedLabel: "mcp:external", hintIncludes: "External MCP", orchestrateFooter: false },
  { id: "hardDeny-GenerateImage", tool: "GenerateImage", class: "hardDeny", hintIncludes: "image generation", orchestrateFooter: false },
  { id: "hardDeny-Await", tool: "Await", class: "hardDeny", hintIncludes: "orchestrate or subagent", orchestrateFooter: false },
  { id: "hardDeny-ApplyPatch", tool: "ApplyPatch", class: "hardDeny", hintIncludes: "ApplyPatch is not supported", orchestrateFooter: false },
];

function argsFor(tool: string): unknown {
  return defaultArgs[tool] ?? {};
}

function assertPolicyDecision(row: PolicyMatrixRow): void {
  const ctx = bridgeCtx(...NATIVE_ALLOW);
  const bridged = bridgeToolInvocation(row.tool, argsFor(row.tool), ctx);
  const label = row.blockedLabel ?? row.tool;

  switch (row.class) {
    case "native":
      assert.notEqual(bridged, null, `${row.id}: expected bridge pass-through`);
      assert.equal(bridged?.name, row.tool, row.id);
      assert.equal(shouldHardDenyCursorTool(row.tool), false, row.id);
      assert.equal(shouldRedirectCursorTool(row.tool), false, row.id);
      assert.equal(shouldBlockDeferredSolomonTool(row.tool), false, row.id);
      return;
    case "redirect":
      assert.equal(bridged, null, `${row.id}: must not bridge`);
      assert.equal(shouldRedirectCursorTool(row.tool) || label.startsWith("mcp:"), true, row.id);
      assert.equal(shouldHardDenyCursorTool(row.tool), false, row.id);
      assert.equal(shouldStopProxyOnBlockedTool(label), true, row.id);
      break;
    case "hardDeny":
      assert.equal(bridged, null, `${row.id}: must not bridge`);
      assert.equal(shouldHardDenyCursorTool(row.tool) || isHardDenyBlockedLabel(label), true, row.id);
      assert.equal(shouldStopProxyOnBlockedTool(label), true, row.id);
      break;
    case "deferredBlock":
      assert.equal(bridged, null, `${row.id}: must not bridge deferred tool`);
      assert.equal(shouldBlockDeferredSolomonTool(row.tool), true, row.id);
      assert.equal(shouldStopProxyOnBlockedTool(row.tool), true, row.id);
      break;
  }
}

function assertCorrectionCopy(row: PolicyMatrixRow): void {
  if (row.class === "native") {
    return;
  }
  const label = row.blockedLabel ?? row.tool;
  const msg = proxyToolCorrectionMessage([label], new Set(NATIVE_ALLOW));
  assert.ok(msg.includes("Blocked by Solomon proxy"), `${row.id}: missing block prefix`);
  if (row.hintIncludes) {
    const hint = correctionHintForBlockedTool(label);
    assert.ok(hint?.includes(row.hintIncludes), `${row.id}: hint ${hint}`);
    assert.ok(msg.includes(row.hintIncludes), `${row.id}: correction ${msg}`);
  }
  const hasFooter = msg.includes("Cursor built-ins are disabled");
  assert.equal(hasFooter, row.orchestrateFooter, `${row.id}: footer mismatch in ${msg}`);
}

function assertStreamEnforcement(row: PolicyMatrixRow): void {
  const pending = [];
  const blocked: string[] = [];
  let detected = false;
  const label = row.blockedLabel ?? row.tool;
  const streamName = row.tool.startsWith("mcp:") ? "mcp" : row.tool;
  const streamArgs = row.tool === "mcp:editFile"
    ? { providerIdentifier: "solomon", toolName: "editFile", args: { path: "x.go" } }
    : row.tool === "mcp:external"
      ? { providerIdentifier: "other", toolName: "readFile", args: { path: "x" } }
      : argsFor(row.tool);

  processStreamEvent(
    {
      type: "tool_call",
      name: streamName,
      status: "running",
      args: streamArgs,
    } as any,
    false,
    () => {},
    () => {},
    pending,
    () => { detected = true; },
    (name) => { blocked.push(name); },
    bridgeCtx(...NATIVE_ALLOW),
  );

  if (row.class === "native") {
    assert.equal(detected, true, row.id);
    assert.equal(pending[0]?.name, row.tool, row.id);
    assert.deepEqual(blocked, [], row.id);
    return;
  }
  assert.equal(detected, true, row.id);
  assert.deepEqual(pending, [], row.id);
  assert.deepEqual(blocked, [label], row.id);
}

test("policy matrix: decision, correction class, no passthrough (2.12)", () => {
  for (const row of POLICY_MATRIX) {
    assertPolicyDecision(row);
    assertCorrectionCopy(row);
  }
});

test("policy matrix: stream enforcement per tool class (2.12)", () => {
  const streamSample: PolicyMatrixRow[] = [
    POLICY_MATRIX.find((r) => r.id === "native-subagent")!,
    POLICY_MATRIX.find((r) => r.id === "redirect-Read")!,
    POLICY_MATRIX.find((r) => r.id === "redirect-Task")!,
    POLICY_MATRIX.find((r) => r.id === "redirect-mcp-deferred")!,
    POLICY_MATRIX.find((r) => r.id === "deferred-readFile")!,
    POLICY_MATRIX.find((r) => r.id === "hardDeny-AskQuestion")!,
    POLICY_MATRIX.find((r) => r.id === "hardDeny-browser")!,
    POLICY_MATRIX.find((r) => r.id === "hardDeny-ApplyPatch")!,
  ];
  for (const row of streamSample) {
    assertStreamEnforcement(row);
  }
});

test("policy matrix: native MCP subagent passes, deferred MCP blocks (2.12)", () => {
  const ctx = bridgeCtx(...NATIVE_ALLOW);
  const subagentMcp = unwrapSolomonMcpCall("mcp", {
    providerIdentifier: "solomon",
    toolName: "subagent",
    args: { task: "explore" },
  });
  assert.ok(subagentMcp);
  const pass = bridgeToolInvocation(subagentMcp!.toolName, subagentMcp!.args, ctx);
  assert.equal(pass?.name, "subagent");
  const editMcp = unwrapSolomonMcpCall("mcp", {
    providerIdentifier: "solomon",
    toolName: "editFile",
    args: { path: "x.go", oldString: "a", newString: "b" },
  });
  assert.ok(editMcp);
  const block = bridgeToolInvocation(editMcp!.toolName, editMcp!.args, ctx);
  assert.equal(block, null);
  const msg = proxyToolCorrectionMessage(["mcp:editFile"], new Set(NATIVE_ALLOW));
  assert.ok(msg.includes("searchTools"));
  assert.ok(msg.includes("Cursor built-ins are disabled"));
});
