// src/server.ts
import http from "node:http";

// src/health.ts
import { createHash, createHmac } from "node:crypto";
import { readFileSync, readdirSync } from "node:fs";
import { resolve, join } from "node:path";

// src/tool-policy.ts
var SOLOMON_TOOL_NAME_RE = /^[a-zA-Z_][a-zA-Z0-9_-]*$/;
var CURSOR_NATIVE_ALIASES = {
  read: "readFile",
  Read: "readFile",
  read_file: "readFile",
  ReadFile: "readFile",
  readfile: "readFile",
  shell: "shell",
  Shell: "shell",
  bash: "shell",
  Bash: "shell",
  run_terminal_cmd: "shell",
  terminal: "shell",
  edit: "editFile",
  Edit: "editFile",
  write: "editFile",
  Write: "editFile",
  StrReplace: "editFile",
  strReplace: "editFile",
  str_replace: "editFile",
  search_replace: "editFile",
  Delete: "editFile",
  delete: "editFile",
  find: "find",
  Find: "find",
  Grep: "find",
  grep: "find",
  Glob: "find",
  glob: "find",
  ListDir: "find",
  list_dir: "find",
  listDir: "find",
  ls: "find",
  ripgrep: "find",
  rg: "find",
  SemanticSearch: "find",
  semanticSearch: "find",
  semantic_search: "find",
  Task: "subagent",
  task: "subagent",
  WebFetch: "fetchWeb",
  webFetch: "fetchWeb",
  web_fetch: "fetchWeb",
  Fetch: "fetchWeb",
  fetch: "fetchWeb",
  WebSearch: "webSearch",
  webSearch: "webSearch",
  web_search: "webSearch"
};
var SOLOMON_CANONICAL_TOOLS = /* @__PURE__ */ new Set([
  "readFile",
  "shell",
  "editFile",
  "editPlan",
  "find",
  "listDir",
  "tree",
  "subagent",
  "fetchWeb",
  "webSearch",
  "deepResearch",
  "researchStatus",
  "createPlan",
  "buildPlan",
  "addTodo",
  "todoList",
  "checkTodo",
  "removeTodo",
  "checkPlan",
  "deletePlan",
  "docsRetrieval",
  "readChat",
  "searchTools",
  "loadSkill",
  "searchSkill",
  "orchestrate",
  "switchMode",
  "listSubAgents"
]);
var BLOCKED_MCP_EXTERNAL_LABEL = "mcp:external";
var MISSING_INTENT_BLOCKED_SUFFIX = ":missing_intent";
function missingIntentBlockedLabel(toolName) {
  return `${toolName.trim()}${MISSING_INTENT_BLOCKED_SUFFIX}`;
}
function isMissingIntentBlockedLabel(label) {
  return label.trim().endsWith(MISSING_INTENT_BLOCKED_SUFFIX);
}
function blockedMcpToolLabel(toolName) {
  return `mcp:${toolName}`;
}
var DEFERRED_SOLOMON_TOOL_NAMES = /* @__PURE__ */ new Set([
  "readFile",
  "shell",
  "editFile",
  "find",
  "listDir",
  "fetchWeb",
  "webSearch",
  "createPlan",
  "editPlan",
  "buildPlan"
]);
var CURSOR_HARD_DENY_TOOLS = /* @__PURE__ */ new Set([
  "AskQuestion",
  "ask_question",
  "askQuestion",
  "GenerateImage",
  "generate_image",
  "generateImage",
  "Await",
  "await",
  "ApplyPatch",
  "apply_patch",
  "applyPatch"
]);
function isBrowserCursorTool(name) {
  const trimmed = name.trim();
  if (!trimmed) {
    return false;
  }
  const lower = trimmed.toLowerCase();
  if (lower.startsWith("browser_")) {
    return true;
  }
  if (/^browser[A-Z]/.test(trimmed)) {
    return true;
  }
  if (/^Browser[A-Z]/.test(trimmed)) {
    return true;
  }
  return false;
}
function shouldHardDenyCursorTool(name) {
  const trimmed = name.trim();
  if (!trimmed) {
    return false;
  }
  if (CURSOR_HARD_DENY_TOOLS.has(trimmed)) {
    return true;
  }
  return isBrowserCursorTool(trimmed);
}
var CURSOR_REDIRECT_EXTRA = /* @__PURE__ */ new Set([
  "ReadLints",
  "read_lints",
  "readLints",
  "EditNotebook",
  "edit_notebook",
  "editNotebook",
  "TodoWrite",
  "todo_write",
  "todoWrite",
  "CallMcpTool",
  "call_mcp_tool",
  "callMcpTool",
  "FetchMcpResource",
  "fetch_mcp_resource",
  "fetchMcpResource",
  "ListMcpResources",
  "list_mcp_resources",
  "listMcpResources"
]);
function shouldRedirectCursorTool(name) {
  const trimmed = name.trim();
  if (!trimmed) {
    return false;
  }
  if (CURSOR_REDIRECT_EXTRA.has(trimmed)) {
    return true;
  }
  return Object.prototype.hasOwnProperty.call(CURSOR_NATIVE_ALIASES, trimmed);
}
function isExposedNativePolicyException(name, allowedNames) {
  const trimmed = name.trim();
  if (!allowedNames?.has(trimmed)) {
    return false;
  }
  if (trimmed === "buildPlan") {
    return true;
  }
  return !allowedNames.has("orchestrate") && (trimmed === "fetchWeb" || trimmed === "webSearch");
}
function shouldBlockDeferredSolomonTool(name) {
  return DEFERRED_SOLOMON_TOOL_NAMES.has(name.trim());
}
function shouldStopProxyOnBlockedTool(label) {
  const trimmed = label.trim();
  if (!trimmed) {
    return false;
  }
  if (trimmed === BLOCKED_MCP_EXTERNAL_LABEL) {
    return true;
  }
  if (isMissingIntentBlockedLabel(trimmed)) {
    return true;
  }
  if (shouldHardDenyCursorTool(trimmed)) {
    return true;
  }
  if (trimmed.startsWith("mcp:")) {
    return shouldBlockDeferredSolomonTool(trimmed.slice(4));
  }
  return shouldRedirectCursorTool(trimmed) || shouldBlockDeferredSolomonTool(trimmed);
}
function isHardDenyBlockedLabel(label) {
  const trimmed = label.trim();
  if (trimmed === BLOCKED_MCP_EXTERNAL_LABEL) {
    return true;
  }
  return shouldHardDenyCursorTool(trimmed);
}
function hardDenyCorrectionHint(toolName) {
  const trimmed = toolName.trim();
  if (!trimmed) {
    return null;
  }
  if (trimmed === BLOCKED_MCP_EXTERNAL_LABEL) {
    return "External MCP servers (including Cursor IDE browser) are not available on this host.";
  }
  if (isBrowserCursorTool(trimmed)) {
    return "Cursor IDE browser tools are not available on this host.";
  }
  const key = trimmed.replace(/_/g, "").toLowerCase();
  switch (key) {
    case "askquestion":
      return "Ask the user in plain text instead of AskQuestion.";
    case "generateimage":
      return "Describe the image in text or use an orchestrate workaround; image generation is not available.";
    case "await":
      return "Use synchronous orchestrate or subagent async polling instead of Await.";
    case "applypatch":
      return "Use orchestrate with the sandbox write/replace SDK for edits; unified diff ApplyPatch is not supported.";
    default:
      if (shouldHardDenyCursorTool(trimmed)) {
        return "This Cursor tool is not available on this host.";
      }
      return null;
  }
}
function missingIntentCorrectionHint(toolName) {
  if (!isMissingIntentBlockedLabel(toolName)) {
    return null;
  }
  return "Every Solomon tool call requires a non-empty intent string; include intent in the arguments.";
}
function redirectExtraCorrectionHint(toolName) {
  const key = toolName.replace(/_/g, "").toLowerCase();
  switch (key) {
    case "readlints":
      return "Lint diagnostics: use orchestrate; Cursor ReadLints is not available on this host.";
    case "editnotebook":
      return "Notebook edits: use orchestrate until a dedicated notebook tool ships.";
    case "todowrite":
      return "Plan todos: use orchestrate with addTodo, todoList, checkTodo, or related plan SDK helpers.";
    case "callmcptool":
    case "fetchmcpresource":
    case "listmcpresources":
      return "Cursor MCP wrappers are unavailable; use searchTools for schemas, then orchestrate with sdk.mcp.<tool>(intent, args). Do not claim an MCP action without an actual host tool result.";
    default:
      return null;
  }
}
function redirectCorrectionHint(toolName) {
  const trimmed = toolName.trim();
  if (!trimmed || isHardDenyBlockedLabel(trimmed)) {
    return null;
  }
  if (trimmed.startsWith("mcp:")) {
    const deferred = trimmed.slice(4);
    if (shouldBlockDeferredSolomonTool(deferred)) {
      return `${deferred}: this MCP wrapper is not callable; use searchTools, then orchestrate with sdk.mcp.<tool>(intent, args) \u2014 not a direct native or MCP tool_call.`;
    }
    return null;
  }
  const extra = redirectExtraCorrectionHint(trimmed);
  if (extra) {
    return extra;
  }
  if (!shouldRedirectCursorTool(trimmed)) {
    return null;
  }
  const target = cursorToolRedirectTarget(trimmed);
  switch (target) {
    case "readFile":
      return "Cursor Read is disabled. Call searchTools, then orchestrate with sdk.ReadFile.";
    case "editFile":
      return "Cursor edits are disabled. Call searchTools, then orchestrate with sdk.WriteFile, sdk.ReplaceInFile, or sdk.DeleteFile.";
    case "shell":
      return "Cursor Shell is disabled. Call searchTools, then orchestrate with sdk.Shell (sync only).";
    case "find":
      return "Cursor Grep/Glob are disabled. Call searchTools, then orchestrate with sdk.Glob, sdk.Grep, or sdk.GrepLines.";
    case "subagent":
      return "Nested agent work: emit native subagent via API tool_calls.";
    case "fetchWeb":
      return "HTTP fetch: orchestrate with sdk.FetchWeb.";
    case "webSearch":
      return "Web search: orchestrate with sdk.WebSearch.";
    default:
      return "Call searchTools, then orchestrate with the sandbox SDK.";
  }
}
function chatCorrectionHintForBlockedTool(toolName) {
  const hardDeny = hardDenyCorrectionHint(toolName);
  if (hardDeny) {
    return hardDeny;
  }
  const trimmed = toolName.trim();
  if (trimmed.startsWith("mcp:")) {
    return "MCP actions are not exposed in CHAT mode; use switchMode before any implementation work and do not claim an MCP result without a host tool result.";
  }
  const target = cursorToolRedirectTarget(trimmed);
  if (target === "fetchWeb") {
    return "Use the native fetchWeb tool in CHAT mode.";
  }
  if (target === "webSearch") {
    return "Use the native webSearch tool in CHAT mode.";
  }
  if (shouldRedirectCursorTool(trimmed) || shouldBlockDeferredSolomonTool(trimmed)) {
    return "This workspace or agent tool is unavailable in CHAT mode; call switchMode before implementation.";
  }
  return null;
}
function correctionHintForBlockedTool(toolName, chatSurface = false) {
  const missingIntent = missingIntentCorrectionHint(toolName);
  if (missingIntent) {
    return missingIntent;
  }
  if (chatSurface) {
    return chatCorrectionHintForBlockedTool(toolName);
  }
  return hardDenyCorrectionHint(toolName) ?? redirectCorrectionHint(toolName);
}
function cursorToolRedirectTarget(cursorName) {
  return CURSOR_NATIVE_ALIASES[cursorName];
}
function isSolomonCanonicalTool(name) {
  return SOLOMON_CANONICAL_TOOLS.has(name);
}
function isValidSolomonToolName(name) {
  return SOLOMON_TOOL_NAME_RE.test(name);
}
function resolveBridgedSolomonName(trimmed, allowedNames) {
  const alias = CURSOR_NATIVE_ALIASES[trimmed];
  if (alias) {
    return alias;
  }
  if (allowedNames?.has(trimmed)) {
    return trimmed;
  }
  if (!allowedNames && SOLOMON_CANONICAL_TOOLS.has(trimmed)) {
    return trimmed;
  }
  return null;
}

// src/proxy-observability.ts
var NATIVE_ENTRY_TOOLS = /* @__PURE__ */ new Set([
  "orchestrate",
  "searchTools",
  "subagent",
  "switchMode",
  "searchSkill",
  "loadSkill",
  "docsRetrieval",
  "readChat"
]);
var WORKSPACE_DEFERRED_TOOLS = /* @__PURE__ */ new Set(["readFile", "editFile", "shell", "find"]);
var PROXY_CORRECTION_LOOP_THRESHOLD = 3;
function classifyProxyTool(toolName) {
  const trimmed = toolName.trim();
  if (!trimmed) {
    return "unknown";
  }
  if (NATIVE_ENTRY_TOOLS.has(trimmed)) {
    return "native";
  }
  if (trimmed === BLOCKED_MCP_EXTERNAL_LABEL || shouldHardDenyCursorTool(trimmed)) {
    return "hardDeny";
  }
  if (trimmed.startsWith("mcp:")) {
    const inner = trimmed.slice(4);
    if (NATIVE_ENTRY_TOOLS.has(inner)) {
      return "native";
    }
    if (shouldBlockDeferredSolomonTool(inner)) {
      return "redirect";
    }
    return "unknown";
  }
  if (shouldRedirectCursorTool(trimmed)) {
    return "redirect";
  }
  if (shouldBlockDeferredSolomonTool(trimmed)) {
    return "deferredBlock";
  }
  return "unknown";
}
function emptyClassRecord() {
  return { native: 0, redirect: 0, hardDeny: 0, deferredBlock: 0, unknown: 0 };
}
function correctionClasses(blockedTools) {
  const out = /* @__PURE__ */ new Set();
  for (const name of blockedTools) {
    const cls = classifyProxyTool(name);
    if (cls !== "native" && cls !== "unknown") {
      out.add(cls);
    }
  }
  return [...out];
}
var ProxyObservabilityState = class {
  turns = 0;
  turnsWithCorrection = 0;
  bridgedNativeByTool = {};
  blockedByClass = emptyClassRecord();
  correctionsByClass = emptyClassRecord();
  consecutiveCorrectionsByClass = emptyClassRecord();
  maxConsecutiveCorrectionsByClass = emptyClassRecord();
  deferredDirectBlocked = 0;
  orchestrateBridged = 0;
  workspaceMutationDeferredBlocked = 0;
  snapshot() {
    return {
      turns: this.turns,
      turnsWithCorrection: this.turnsWithCorrection,
      bridgedNativeByTool: { ...this.bridgedNativeByTool },
      blockedByClass: { ...this.blockedByClass },
      correctionsByClass: { ...this.correctionsByClass },
      consecutiveCorrectionsByClass: { ...this.consecutiveCorrectionsByClass },
      maxConsecutiveCorrectionsByClass: { ...this.maxConsecutiveCorrectionsByClass },
      deferredDirectBlocked: this.deferredDirectBlocked,
      orchestrateBridged: this.orchestrateBridged,
      workspaceMutationDeferredBlocked: this.workspaceMutationDeferredBlocked
    };
  }
  recordTurn(obs) {
    this.turns += 1;
    const classes = correctionClasses(obs.blockedTools);
    for (const name of obs.blockedTools) {
      const cls = classifyProxyTool(name);
      this.blockedByClass[cls] += 1;
      if (cls === "deferredBlock") {
        this.deferredDirectBlocked += 1;
        const trimmed = name.trim();
        if (WORKSPACE_DEFERRED_TOOLS.has(trimmed)) {
          this.workspaceMutationDeferredBlocked += 1;
        }
      }
    }
    for (const name of obs.bridgedTools) {
      const trimmed = name.trim();
      if (!trimmed) {
        continue;
      }
      this.bridgedNativeByTool[trimmed] = (this.bridgedNativeByTool[trimmed] ?? 0) + 1;
      if (trimmed === "orchestrate") {
        this.orchestrateBridged += 1;
      }
    }
    const hadCorrection = obs.proxyCorrection && obs.bridgedTools.length === 0;
    if (hadCorrection) {
      this.turnsWithCorrection += 1;
      for (const cls of classes) {
        this.correctionsByClass[cls] += 1;
        const streak = this.consecutiveCorrectionsByClass[cls] + 1;
        this.consecutiveCorrectionsByClass[cls] = streak;
        if (streak > this.maxConsecutiveCorrectionsByClass[cls]) {
          this.maxConsecutiveCorrectionsByClass[cls] = streak;
        }
      }
      for (const cls of Object.keys(this.consecutiveCorrectionsByClass)) {
        if (!classes.includes(cls)) {
          this.consecutiveCorrectionsByClass[cls] = 0;
        }
      }
    } else if (obs.bridgedTools.length > 0 || obs.blockedTools.length === 0) {
      this.consecutiveCorrectionsByClass = emptyClassRecord();
    }
    const snap = this.snapshot();
    emitTurnObservability(obs, snap, classes, hadCorrection);
    return snap;
  }
};
var state = new ProxyObservabilityState();
var logSink = null;
function proxyObservabilityEnabled() {
  const v = process.env.CURSOR_API_PROXY_OBS?.trim().toLowerCase();
  return v === "1" || v === "true" || v === "yes";
}
function shouldEmitLogs() {
  return logSink !== null || proxyObservabilityEnabled();
}
function emitLog(payload) {
  const line = JSON.stringify({ ts: (/* @__PURE__ */ new Date()).toISOString(), ...payload });
  if (logSink) {
    logSink(line);
    return;
  }
  if (proxyObservabilityEnabled()) {
    console.error(line);
  }
}
function emitTurnObservability(obs, counters, classes, hadCorrection) {
  if (!shouldEmitLogs()) {
    return;
  }
  emitLog({
    event: "proxy_turn",
    stream: obs.stream,
    bridged: obs.bridgedTools,
    blocked: obs.blockedTools.map((name) => ({ name, class: classifyProxyTool(name) })),
    proxy_correction: hadCorrection,
    correction_classes: hadCorrection ? classes : [],
    counters: {
      turns: counters.turns,
      turns_with_correction: counters.turnsWithCorrection,
      orchestrate_bridged: counters.orchestrateBridged,
      deferred_direct_blocked: counters.deferredDirectBlocked,
      workspace_mutation_deferred_blocked: counters.workspaceMutationDeferredBlocked,
      corrections_by_class: counters.correctionsByClass,
      consecutive_corrections_by_class: counters.consecutiveCorrectionsByClass
    }
  });
  if (!hadCorrection) {
    return;
  }
  for (const cls of classes) {
    const streak = counters.consecutiveCorrectionsByClass[cls];
    if (streak > PROXY_CORRECTION_LOOP_THRESHOLD) {
      emitLog({
        event: "proxy_correction_loop",
        stream: obs.stream,
        class: cls,
        consecutive: streak,
        threshold: PROXY_CORRECTION_LOOP_THRESHOLD
      });
    }
  }
}
function observeProxyTurn(obs) {
  return state.recordTurn(obs);
}

// src/health.ts
var HEALTH_PROTOCOL = 1;
function runtimeDigest(root) {
  const files = ["dist/index.js", "package.json", "package-lock.json"];
  for (const name of readdirSync(join(root, "dist/prompts")).sort()) {
    files.push(`dist/prompts/${name}`);
  }
  const hash = createHash("sha256");
  for (const name of files.sort()) {
    hash.update(name + "\0");
    hash.update(readFileSync(join(root, name)));
    hash.update("\0");
  }
  return hash.digest("hex");
}
function createHealthResponder(cfg, root) {
  const identity = {
    protocol: HEALTH_PROTOCOL,
    bundle: runtimeDigest(root),
    cwd: resolve(cfg.cwd),
    internalTools: cfg.allowCursorInternalTools,
    observability: proxyObservabilityEnabled()
  };
  return (nonce) => {
    if (!/^[a-f0-9]{64}$/.test(nonce)) {
      return { ok: true, identity };
    }
    const payload = [nonce, String(identity.protocol), identity.bundle, identity.cwd, String(identity.internalTools), String(identity.observability)].join("\n");
    return { ok: true, identity, proof: createHmac("sha256", cfg.apiKey).update(payload).digest("hex") };
  };
}

// src/chat/index.ts
import { Cursor } from "@cursor/sdk";

// src/harness-prompt.ts
import * as fs from "node:fs";
import * as path from "node:path";
import { fileURLToPath } from "node:url";
var cachedClauses;
var cachedToolsClauseTemplate;
var SOLOMON_NATIVE_ENTRY_TOOLS = [
  "searchTools",
  "orchestrate",
  "subagent",
  "listSubAgents",
  "switchMode",
  "searchSkill",
  "loadSkill",
  "docsRetrieval",
  "readChat",
  "fetchWeb",
  "webSearch",
  "deepResearch",
  "researchStatus"
];
function resolvePromptsDir() {
  const here = path.dirname(fileURLToPath(import.meta.url));
  for (const candidate of [path.join(here, "prompts"), path.join(here, "..", "prompts")]) {
    if (fs.existsSync(path.join(candidate, "harness-clauses.txt"))) {
      return candidate;
    }
  }
  throw new Error("cursor harness prompts not found (run: npm run build in integrations/cursor)");
}
function readPromptFile(name) {
  return fs.readFileSync(path.join(resolvePromptsDir(), name), "utf8").trim();
}
function harnessClauses() {
  if (!cachedClauses) {
    cachedClauses = readPromptFile("harness-clauses.txt").split(/\n\n+/).map((s) => s.trim()).filter((s) => s.length > 0);
  }
  return cachedClauses;
}
function harnessToolsClauseTemplate() {
  if (!cachedToolsClauseTemplate) {
    cachedToolsClauseTemplate = readPromptFile("harness-tools-clause.txt");
  }
  return cachedToolsClauseTemplate;
}
function toolNamesFromRequest(tools) {
  if (!tools?.length) {
    return [];
  }
  const names = [];
  const seen = /* @__PURE__ */ new Set();
  for (const t of tools) {
    const n = t.function?.name?.trim();
    if (n && !seen.has(n)) {
      seen.add(n);
      names.push(n);
    }
  }
  return names;
}
function sortToolNamesForHarness(names) {
  const ordered = [];
  const seen = /* @__PURE__ */ new Set();
  for (const native of SOLOMON_NATIVE_ENTRY_TOOLS) {
    if (names.includes(native) && !seen.has(native)) {
      seen.add(native);
      ordered.push(native);
    }
  }
  for (const name of names) {
    if (!seen.has(name)) {
      seen.add(name);
      ordered.push(name);
    }
  }
  return ordered;
}
function harnessToolsClause(tools) {
  const names = sortToolNamesForHarness(toolNamesFromRequest(tools));
  if (names.length === 0) {
    return "";
  }
  return harnessToolsClauseTemplate().replace(/\{\{TOOL_NAMES\}\}/g, names.join(", "));
}
function harnessToolCatalog(tools) {
  if (!tools?.length) {
    return "";
  }
  const lines = ["[Harness] Tool catalog (native entry points first; schemas for XML invocations):"];
  const seen = /* @__PURE__ */ new Set();
  const ordered = [...tools].sort((a, b) => {
    const an = a.function?.name?.trim() ?? "";
    const bn = b.function?.name?.trim() ?? "";
    const ai = SOLOMON_NATIVE_ENTRY_TOOLS.indexOf(an);
    const bi = SOLOMON_NATIVE_ENTRY_TOOLS.indexOf(bn);
    const ar = ai === -1 ? SOLOMON_NATIVE_ENTRY_TOOLS.length : ai;
    const br = bi === -1 ? SOLOMON_NATIVE_ENTRY_TOOLS.length : bi;
    if (ar !== br) {
      return ar - br;
    }
    return an.localeCompare(bn);
  });
  for (const t of ordered) {
    const name = t.function?.name?.trim();
    if (!name || seen.has(name)) {
      continue;
    }
    seen.add(name);
    const desc = t.function?.description?.trim();
    lines.push(desc ? `- ${name}: ${desc}` : `- ${name}`);
  }
  if (lines.length === 1) {
    return "";
  }
  return lines.join("\n");
}
function harnessPreamble(tools) {
  if (!tools?.length) {
    return "";
  }
  const parts = [...harnessClauses()];
  const toolsClause = harnessToolsClause(tools);
  if (toolsClause) {
    parts.push(toolsClause);
  }
  const catalog = harnessToolCatalog(tools);
  if (catalog) {
    parts.push(catalog);
  }
  return parts.join("\n\n") + "\n\n";
}

// src/xml-utils.ts
function escapeXmlAttr(s) {
  return s.replace(/&/g, "&amp;").replace(/"/g, "&quot;").replace(/</g, "&lt;");
}
function escapeXmlText(s) {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;");
}
function escapeXmlTextStrict(s) {
  return escapeXmlText(s).replace(/>/g, "&gt;");
}
function unescapeXML(s) {
  return s.replace(/&quot;/g, '"').replace(/&apos;/g, "'").replace(/&gt;/g, ">").replace(/&lt;/g, "<").replace(/&amp;/g, "&");
}

// src/tool-intent.ts
var TOOL_INTENT_SCHEMA_DESCRIPTION = "Required brief phrase describing why this tool call is being made";
function readToolIntent(raw) {
  let value = raw;
  if (typeof value === "string") {
    try {
      value = JSON.parse(value);
    } catch {
      return null;
    }
  }
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return null;
  }
  const intent = value.intent;
  if (typeof intent !== "string") {
    return null;
  }
  const trimmed = intent.trim();
  return trimmed === "" ? null : trimmed;
}
function schemaWithRequiredToolIntent(schema) {
  const out = { ...schema ?? {} };
  if (typeof out.type !== "string") {
    out.type = "object";
  }
  const properties = {};
  if (out.properties && typeof out.properties === "object" && !Array.isArray(out.properties)) {
    Object.assign(properties, out.properties);
  }
  const existingIntent = properties.intent;
  const intentSchema = existingIntent && typeof existingIntent === "object" && !Array.isArray(existingIntent) ? { ...existingIntent } : {};
  properties.intent = {
    ...intentSchema,
    type: "string",
    minLength: 1,
    description: typeof intentSchema.description === "string" && intentSchema.description.trim() !== "" ? intentSchema.description : TOOL_INTENT_SCHEMA_DESCRIPTION
  };
  out.properties = properties;
  const required = Array.isArray(out.required) ? out.required.filter((item) => typeof item === "string") : [];
  if (!required.includes("intent")) {
    required.push("intent");
  }
  out.required = required;
  return out;
}
function invocationIntent(inv) {
  if (Object.prototype.hasOwnProperty.call(inv, "intent")) {
    return typeof inv.intent === "string" && inv.intent.trim() !== "" ? inv.intent.trim() : null;
  }
  return readToolIntent(inv.args);
}

// src/messages.ts
import * as fs2 from "node:fs";
import * as path2 from "node:path";
import { fileURLToPath as fileURLToPath2 } from "node:url";
function messageToPromptText(m) {
  if (typeof m.content === "string") {
    return m.content;
  }
  if (!Array.isArray(m.content)) {
    return "";
  }
  const texts = [];
  for (const p of m.content) {
    if (p.type === "text" && p.text) {
      texts.push(p.text);
    }
  }
  return texts.join("\n");
}
function messageToUserPayload(m) {
  const text = messageToPromptText(m);
  const images = extractImages(m);
  if (images.length > 0) {
    return { text, images };
  }
  return { text };
}
function extractImages(m) {
  if (!Array.isArray(m.content)) {
    return [];
  }
  const out = [];
  for (const p of m.content) {
    if (p.type !== "image_url" || !p.image_url?.url) {
      continue;
    }
    const url = p.image_url.url;
    if (url.startsWith("data:")) {
      const match = /^data:([^;]+);base64,(.+)$/i.exec(url);
      if (match) {
        out.push({ data: match[2], mimeType: match[1] });
      }
      continue;
    }
    if (url.startsWith("file://")) {
      try {
        const filePath = fileURLToPath2(url);
        const data = fs2.readFileSync(filePath).toString("base64");
        const mime = guessMime(filePath);
        out.push({ data, mimeType: mime });
      } catch {
        out.push({ url });
      }
      continue;
    }
    out.push({ url });
  }
  return out;
}
function guessMime(filePath) {
  const ext = path2.extname(filePath).toLowerCase();
  switch (ext) {
    case ".png":
      return "image/png";
    case ".jpg":
    case ".jpeg":
      return "image/jpeg";
    case ".gif":
      return "image/gif";
    case ".webp":
      return "image/webp";
    default:
      return "application/octet-stream";
  }
}
function roughTokFromString(s) {
  const n = [...s].length;
  if (n <= 0) {
    return 0;
  }
  return Math.floor((n + 2) / 3);
}
function roughTokFromMessages(messages) {
  let sum = 0;
  for (const m of messages) {
    sum += roughTokFromString(messageToPromptText(m));
  }
  return sum;
}
function withHarnessPreamble(prompt, tools) {
  const prefix = harnessPreamble(tools);
  if (typeof prompt === "string") {
    return prefix + prompt;
  }
  return { ...prompt, text: prefix + prompt.text };
}
function sanitizeReflectedText(s) {
  return escapeXmlTextStrict(stripUnsafeControlChars(s)).slice(0, 4096);
}
function stripUnsafeControlChars(s) {
  let out = "";
  for (const ch of s) {
    const code = ch.charCodeAt(0);
    if (code === 9 || code === 10 || code === 13 || code >= 32 && code !== 127) {
      out += ch;
    }
  }
  return out;
}
var DEFAULT_MODEL_ID = "composer-2.5";
var MODEL_ID_RE = /^[a-zA-Z0-9][a-zA-Z0-9._-]*$/;
var TOOL_NAME_RE = /^[a-zA-Z_][a-zA-Z0-9_-]*$/;
function sanitizeModelId(v) {
  let s = (v ?? DEFAULT_MODEL_ID).trim();
  if (s.toLowerCase().startsWith("cursor-")) {
    s = s.slice("cursor-".length).trim();
  }
  if (!s || s.length > 256 || !MODEL_ID_RE.test(s)) {
    return DEFAULT_MODEL_ID;
  }
  return s;
}
function isSafeToolName(name) {
  const s = name.trim();
  return s.length > 0 && s.length <= 128 && TOOL_NAME_RE.test(s);
}
function formatAssistantToolCalls(toolCalls) {
  const parts = ["<tool_calls>"];
  for (const tc of toolCalls) {
    const name = tc.function?.name?.trim() ?? "";
    if (!name) {
      continue;
    }
    const args = tc.function?.arguments?.trim() || "{}";
    parts.push(`<tool name="${escapeXmlAttr(name)}">`);
    parts.push(`<intent>${escapeXmlTextStrict(readToolIntent(args) ?? "")}</intent>`);
    parts.push(`<args>${escapeXmlTextStrict(args)}</args>`);
    parts.push("</tool>");
  }
  parts.push("</tool_calls>");
  return parts.join("\n");
}
function formatChatMessage(m) {
  switch (m.role) {
    case "tool":
      return `[tool result ${m.tool_call_id ?? ""}]
${messageToPromptText(m)}`;
    case "assistant": {
      const text = messageToPromptText(m).trim();
      const tools = m.tool_calls && m.tool_calls.length > 0 ? formatAssistantToolCalls(m.tool_calls) : "";
      if (text && tools) {
        return `[assistant]
${text}

${tools}`;
      }
      if (tools) {
        return `[assistant]
${tools}`;
      }
      return `[assistant]
${text}`;
    }
    case "user":
      return `[user]
${messageToPromptText(m)}`;
    default:
      return `[${m.role}]
${messageToPromptText(m)}`;
  }
}
function buildPromptFromMessages(messages, tools) {
  if (messages.length === 1 && messages[0].role === "user") {
    return withHarnessPreamble(messageToUserPayload(messages[0]), tools);
  }
  const lines = [];
  for (const m of messages) {
    lines.push(formatChatMessage(m));
  }
  return withHarnessPreamble(lines.join("\n\n"), tools);
}

// src/model-selection.ts
function resolveModelSelection(models, id, reasoningEffort, fastMode) {
  const resolvedID = resolveFastModelID(models, resolveKnownModelID(models, id), fastMode);
  let info = models.find((m) => m.id === resolvedID);
  if (!info && !fastMode && isFastModelID(id)) {
    info = models.find((m) => m.id === id);
  }
  let params = info ? modelVariantParams(info, fastMode) : [];
  if (!fastMode && info?.parameters?.length && params.length === 0) {
    params = buildParamsFromDefinitions(info, false);
  }
  const reasoning = normalizeReasoningEffort(reasoningEffort);
  if (reasoning && info) {
    upsertReasoningParam(params, info, reasoning);
  }
  return params.length > 0 ? { id: resolvedID, params } : { id: resolvedID };
}
function resolveKnownModelID(models, id) {
  const raw = id.trim();
  const stripped = raw.toLowerCase().startsWith("cursor-") ? raw.slice("cursor-".length) : raw;
  const ids = models.map((m) => m.id).filter(Boolean);
  const set = new Set(ids);
  if (set.has(raw)) {
    return raw;
  }
  if (set.has(stripped)) {
    return stripped;
  }
  const family = stripped.replace(/-fast$/i, "");
  const matches = ids.filter((candidate) => candidate === family || candidate.startsWith(`${family}.`) || candidate.startsWith(`${family}-`) || family.startsWith("grok-4") && candidate.startsWith("grok-4"));
  if (matches.length === 0) {
    return stripped;
  }
  matches.sort();
  return matches[matches.length - 1];
}
function resolveFastModelID(models, id, fastMode) {
  const ids = new Set(models.map((m) => m.id));
  if (fastMode) {
    if (isFastModelID(id)) {
      return id;
    }
    const fastID = `${id}-fast`;
    return ids.has(fastID) ? fastID : id;
  }
  if (!isFastModelID(id)) {
    return id;
  }
  const baseID = id.replace(/-fast$/i, "");
  return ids.has(baseID) ? baseID : id;
}
function isFastModelID(id) {
  return /-fast$/i.test(id);
}
function modelVariantParams(info, fastMode) {
  const variants = info.variants ?? [];
  if (variants.length > 0) {
    const picked = fastMode ? variants.find((v) => variantFastParam(v) === true) ?? variants.find(isFastVariant) : variants.find((v) => variantFastParam(v) === false) ?? variants.find((v) => variantFastParam(v) !== true && !isFastVariant(v));
    if (picked) {
      return picked.params.map((p) => ({ ...p }));
    }
  }
  return buildParamsFromDefinitions(info, fastMode);
}
function buildParamsFromDefinitions(info, fastMode) {
  const defs = info.parameters ?? [];
  const out = [];
  for (const def of defs) {
    if (isReasoningParam(def)) {
      continue;
    }
    const value = pickParamValue(def, fastMode);
    if (value) {
      out.push({ id: def.id, value });
    }
  }
  return out;
}
function pickParamValue(def, fastMode) {
  const values = def.values ?? [];
  if (values.length === 0) {
    return "";
  }
  if (def.id === "fast") {
    if (fastMode) {
      return values.find((v) => v.value === "true")?.value ?? values[values.length - 1].value;
    }
    return values.find((v) => v.value === "false")?.value ?? values[0].value;
  }
  if (fastMode) {
    const fast = values.find((v) => isFastValue(v.value));
    return fast?.value ?? values[0].value;
  }
  return pickNonFastValue(values) ?? "";
}
function pickNonFastValue(values) {
  const nonFast = values.filter((v) => !isFastValue(v.value));
  if (nonFast.length === 0) {
    return void 0;
  }
  const prefer = ["default", "standard", "normal", "balanced", "medium"];
  for (const p of prefer) {
    const hit = nonFast.find((v) => stringsEqual(v.value, p));
    if (hit) {
      return hit.value;
    }
  }
  return nonFast[0]?.value;
}
function isFastValue(value) {
  const s = value.trim().toLowerCase();
  if (s === "fast" || s.endsWith("-fast")) {
    return true;
  }
  return /(^|[-_.])fast($|[-_.])/.test(s);
}
function variantFastParam(v) {
  const p = v.params.find((x) => x.id === "fast");
  if (!p) {
    return void 0;
  }
  const s = p.value.trim().toLowerCase();
  if (s === "true" || s === "1") {
    return true;
  }
  if (s === "false" || s === "0") {
    return false;
  }
  return void 0;
}
function isFastVariant(v) {
  const fast = variantFastParam(v);
  if (fast !== void 0) {
    return fast;
  }
  const name = v.displayName.toLowerCase();
  return name.includes("fast");
}
function normalizeReasoningEffort(v) {
  const s = (v ?? "").trim().toLowerCase().replaceAll("_", "-").replace(/\s+/g, "-");
  if (!s || s === "none") {
    return "";
  }
  if (s === "med") return "medium";
  if (s === "x-high" || s === "extra-high") return "xhigh";
  return s;
}
function upsertReasoningParam(params, info, effort) {
  const param = (info.parameters ?? []).find((p) => isReasoningParam(p) && p.values.some((v) => stringsEqual(v.value, effort)));
  if (!param) {
    return;
  }
  const existing = params.find((p) => p.id === param.id);
  if (existing) {
    existing.value = effort;
    return;
  }
  params.push({ id: param.id, value: effort });
}
function isReasoningParam(p) {
  const id = p.id.toLowerCase();
  const name = (p.displayName ?? "").toLowerCase();
  return id === "thinking" || id.includes("reason") || id.includes("effort") || name.includes("thinking") || name.includes("reason");
}
function stringsEqual(a, b) {
  return a.toLowerCase() === b.toLowerCase();
}

// src/model-filter.ts
var labOrder = ["composer", "openai", "anthropic", "xai", "kimi", "auto"];
function filterFlagshipModelIDs(ids) {
  if (ids.length === 0) {
    return ["composer-2.5", "auto"];
  }
  const byLab = /* @__PURE__ */ new Map();
  for (const raw of ids) {
    const id = raw.trim();
    if (!id) {
      continue;
    }
    const lab = classifyLab(id);
    if (!lab) {
      continue;
    }
    const bucket = byLab.get(lab) ?? [];
    bucket.push(id);
    byLab.set(lab, bucket);
  }
  const out = [];
  for (const lab of labOrder) {
    const pick = pickLabFlagship(lab, byLab.get(lab) ?? []);
    if (pick) {
      out.push(pick);
    }
  }
  if (out.length === 0) {
    return ["composer-2.5", "auto"];
  }
  return ensureAutoLast(out);
}
function orderModelIDs(ids) {
  if (ids.length === 0) {
    return [];
  }
  const byLab = /* @__PURE__ */ new Map();
  const other = [];
  for (const raw of ids) {
    const id = raw.trim();
    if (!id) {
      continue;
    }
    const lab = classifyLab(id);
    if (!lab) {
      other.push(id);
      continue;
    }
    const bucket = byLab.get(lab) ?? [];
    bucket.push(id);
    byLab.set(lab, bucket);
  }
  const out = [];
  for (const lab of labOrder) {
    out.push(...sortLabModelIDs(lab, byLab.get(lab) ?? []));
  }
  out.push(...other);
  if (out.length === 0) {
    return out;
  }
  if (out.some((id) => id.toLowerCase() === "auto")) {
    return ensureAutoLast(out);
  }
  return out;
}
function sortLabModelIDs(lab, ids) {
  const items = ids.map((id) => ({ id, sc: scoreLabModel(lab, id) }));
  items.sort((a, b) => {
    if (a.sc.ok !== b.sc.ok) {
      return a.sc.ok ? -1 : 1;
    }
    if (!a.sc.ok) {
      return a.id < b.id ? -1 : a.id > b.id ? 1 : 0;
    }
    return flagshipBetter(a.sc, b.sc) ? -1 : flagshipBetter(b.sc, a.sc) ? 1 : 0;
  });
  return items.map((item) => item.id);
}
function ensureAutoLast(out) {
  const filtered = out.filter((id) => id.toLowerCase() !== "auto");
  filtered.push("auto");
  return filtered;
}
function classifyLab(id) {
  const m = id.toLowerCase().trim();
  if (m === "auto") {
    return "auto";
  }
  if (m.startsWith("composer")) {
    return "composer";
  }
  if (m.startsWith("gpt")) {
    return "openai";
  }
  if (m.includes("claude")) {
    return "anthropic";
  }
  if (m.includes("grok")) {
    return "xai";
  }
  if (m.includes("kimi")) {
    return "kimi";
  }
  return null;
}
function pickLabFlagship(lab, ids) {
  if (ids.length === 0) {
    return "";
  }
  if (lab === "auto") {
    return ids.find((id) => id.toLowerCase() === "auto") ?? "";
  }
  let best = "";
  let bestSc = null;
  for (const id of ids) {
    const sc = scoreLabModel(lab, id);
    if (!sc.ok) {
      continue;
    }
    if (!bestSc || flagshipBetter(sc, bestSc)) {
      best = id;
      bestSc = sc;
    }
  }
  return best;
}
function scoreLabModel(lab, id) {
  switch (lab) {
    case "composer":
      return scoreComposer(id);
    case "openai":
      return scoreGPT(id);
    case "anthropic":
      return scoreAnthropic(id);
    case "xai":
      return scoreGrok(id);
    case "kimi":
      return scoreKimi(id);
    default:
      return { ver: [], lineTier: 0, tier: 0, ok: false };
  }
}
function flagshipBetter(a, b) {
  if (a.lineTier !== b.lineTier) {
    return a.lineTier > b.lineTier;
  }
  const c = compareVersionKeys(a.ver, b.ver);
  if (c !== 0) {
    return c > 0;
  }
  return a.tier > b.tier;
}
function scoreComposer(id) {
  const m = id.toLowerCase().trim();
  if (!m.startsWith("composer")) {
    return { ver: [], lineTier: 0, tier: 0, ok: false };
  }
  const rest = m.slice("composer".length).replace(/^-/, "");
  if (!rest) {
    return { ver: [], lineTier: 0, tier: 0, ok: false };
  }
  const parts = rest.split("-");
  const ver = parseVersionSegment(parts[0] ?? "");
  if (ver.length === 0) {
    return { ver: [], lineTier: 0, tier: 0, ok: false };
  }
  return { ver, lineTier: 0, tier: composerVariantTier(parts.slice(1)), ok: true };
}
function scoreGPT(id) {
  const m = id.toLowerCase().trim();
  if (!m.startsWith("gpt")) {
    return { ver: [], lineTier: 0, tier: 0, ok: false };
  }
  for (const p of ["gpt-image", "gpt-realtime", "gpt-audio"]) {
    if (m.startsWith(p)) {
      return { ver: [], lineTier: 0, tier: 0, ok: false };
    }
  }
  const rest = m.slice("gpt-".length);
  const parts = rest.split("-");
  const ver = parseVersionSegment(parts[0] ?? "");
  if (ver.length === 0) {
    return { ver: [], lineTier: 0, tier: 0, ok: false };
  }
  return { ver, lineTier: 0, tier: gptVariantTier(parts.slice(1)), ok: true };
}
function scoreAnthropic(id) {
  const m = id.toLowerCase().trim();
  if (!m.includes("claude")) {
    return { ver: [], lineTier: 0, tier: 0, ok: false };
  }
  let rest = m.replace(/^claude-?/, "");
  const parts = rest.split("-");
  const ver = versionKeyFromParts(parts);
  const lineTier = anthropicModelLineTier(m);
  return { ver, lineTier, tier: anthropicVariantTier(parts), ok: true };
}
function anthropicModelLineTier(m) {
  if (m.includes("opus")) {
    return 100;
  }
  if (m.includes("sonnet")) {
    return 75;
  }
  if (m.includes("haiku")) {
    return 50;
  }
  return 60;
}
function scoreGrok(id) {
  const m = id.toLowerCase().trim();
  if (!m.includes("grok") || m.includes("build")) {
    return { ver: [], lineTier: 0, tier: 0, ok: false };
  }
  let rest = m.replace(/^grok-/, "");
  rest = rest.replace(/^grok/, "");
  let ver = digitsVersionKey(rest);
  if (ver.length === 0) {
    ver = parseVersionSegment(rest.split("-")[0] ?? "");
  }
  if (ver.length === 0) {
    return { ver: [], lineTier: 0, tier: 0, ok: false };
  }
  return { ver, lineTier: 0, tier: grokVariantTier(rest), ok: true };
}
function scoreKimi(id) {
  const m = id.toLowerCase().trim();
  if (!m.includes("kimi")) {
    return { ver: [], lineTier: 0, tier: 0, ok: false };
  }
  const idx = m.indexOf("kimi");
  let rest = m.slice(idx + 4).replace(/^-/, "").replace(/^k/, "");
  let ver = parseVersionSegment(rest);
  if (ver.length === 0) {
    const parts = m.split("-");
    for (let i = 0; i < parts.length; i++) {
      if (parts[i] !== "kimi" || i + 1 >= parts.length) {
        continue;
      }
      ver = parseVersionSegment(parts[i + 1].replace(/^k/, ""));
      break;
    }
  }
  if (ver.length === 0) {
    return { ver: [], lineTier: 0, tier: 0, ok: false };
  }
  return { ver, lineTier: 0, tier: 100, ok: true };
}
function composerVariantTier(suffix) {
  if (suffix.length === 0) {
    return 100;
  }
  const s = suffix.join("-");
  if (s.includes("fast")) {
    return 40;
  }
  if (s.includes("beta")) {
    return 60;
  }
  return 80;
}
function gptVariantTier(suffix) {
  if (suffix.length === 0) {
    return 100;
  }
  const s = suffix.join("-");
  if (s.includes("mini")) {
    return 30;
  }
  if (s.includes("nano")) {
    return 25;
  }
  if (s.includes("codex")) {
    return 50;
  }
  if (s.includes("pro")) {
    return 20;
  }
  if (s.includes("medium")) {
    return 85;
  }
  return 70;
}
function anthropicVariantTier(parts) {
  if (parts.length <= 1) {
    return 80;
  }
  const s = parts.slice(1).join("-");
  if (s.includes("thinking-medium")) {
    return 100;
  }
  if (s.includes("thinking")) {
    return 90;
  }
  return 70;
}
function grokVariantTier(rest) {
  if (rest.includes("low")) {
    return 40;
  }
  if (rest.includes("medium")) {
    return 60;
  }
  if (rest.includes("high")) {
    return 80;
  }
  if (!rest.includes("-")) {
    return 100;
  }
  return 90;
}
function digitsVersionKey(m) {
  const key = [];
  for (let i = 0; i < m.length; ) {
    if (m[i] < "0" || m[i] > "9") {
      i++;
      continue;
    }
    let j = i;
    while (j < m.length && m[j] >= "0" && m[j] <= "9") {
      j++;
    }
    key.push(Number.parseInt(m.slice(i, j), 10));
    i = j;
  }
  return key;
}
function versionKeyFromParts(parts) {
  const key = [];
  for (const p of parts) {
    if (!p) {
      continue;
    }
    if (isAnthropicSuffixPart(p)) {
      break;
    }
    const seg = parseVersionSegment(p);
    if (seg.length === 0) {
      break;
    }
    key.push(...seg);
  }
  return key;
}
function isAnthropicSuffixPart(p) {
  if (p === "thinking" || p === "medium" || p === "fast" || p === "high" || p === "low" || p === "haiku" || p === "sonnet" || p === "opus") {
    return true;
  }
  return p.includes("thinking");
}
function compareVersionKeys(a, b) {
  const n = Math.max(a.length, b.length);
  for (let i = 0; i < n; i++) {
    const xa = i < a.length ? a[i] : 0;
    const xb = i < b.length ? b[i] : 0;
    if (xa !== xb) {
      return xa - xb;
    }
  }
  return 0;
}
function parseVersionSegment(ver) {
  const v = ver.trim();
  if (!v) {
    return [];
  }
  if (v.includes(".")) {
    const key2 = [];
    for (const p of v.split(".")) {
      const { n: n2, rest: rest2 } = parseLeadingDigits(p);
      if (n2 < 0) {
        continue;
      }
      key2.push(n2);
      if (rest2) {
        key2.push(rest2.charCodeAt(0));
      }
    }
    return key2;
  }
  const { n, rest } = parseLeadingDigits(v);
  if (n < 0) {
    return [];
  }
  const key = [n];
  if (rest) {
    key.push(rest.charCodeAt(0));
  }
  return key;
}
function parseLeadingDigits(s) {
  let i = 0;
  while (i < s.length && s[i] >= "0" && s[i] <= "9") {
    i++;
  }
  if (i === 0) {
    return { n: -1, rest: s };
  }
  return { n: Number.parseInt(s.slice(0, i), 10), rest: s.slice(i) };
}

// src/sessions.ts
import { randomBytes } from "node:crypto";
function newCompletionId() {
  return "chatcmpl-" + randomBytes(12).toString("hex");
}

// src/openai-tools.ts
import { randomBytes as randomBytes2 } from "node:crypto";

// src/json-args.ts
function parseJSONObject(raw) {
  try {
    const v = JSON.parse(raw.trim());
    if (v && typeof v === "object" && !Array.isArray(v)) {
      return v;
    }
  } catch {
    return null;
  }
  return null;
}
function parseArgsObject(raw) {
  if (raw === null || raw === void 0) {
    return {};
  }
  if (typeof raw === "string") {
    try {
      return JSON.parse(raw);
    } catch {
      return null;
    }
  }
  if (typeof raw === "object") {
    return { ...raw };
  }
  return null;
}
function normalizeArgsObject(v) {
  if (v === void 0 || v === null) {
    return {};
  }
  if (typeof v === "string") {
    return parseJSONObject(v);
  }
  if (typeof v === "object" && !Array.isArray(v)) {
    return v;
  }
  return null;
}

// src/openai-sse.ts
var JSON_RESPONSE_HEADERS = {
  "Content-Type": "application/json",
  "X-Content-Type-Options": "nosniff"
};
var SSE_RESPONSE_HEADERS = {
  "Content-Type": "text/event-stream",
  "Cache-Control": "no-cache",
  Connection: "keep-alive",
  "X-Content-Type-Options": "nosniff"
};
function sendJsonResponse(res, statusCode, payload) {
  if (res.headersSent || res.writableEnded || res.destroyed) {
    return;
  }
  res.writeHead(statusCode, JSON_RESPONSE_HEADERS);
  res.end(JSON.stringify(payload));
}
function ensureSSEHeaders(res) {
  if (!res.headersSent && !res.writableEnded && !res.destroyed) {
    res.writeHead(200, SSE_RESPONSE_HEADERS);
  }
}
function writeSSE(res, payload) {
  if (res.writableEnded || res.destroyed) {
    return false;
  }
  ensureSSEHeaders(res);
  try {
    res.write(`data: ${JSON.stringify(payload)}

`);
    return true;
  } catch {
    return false;
  }
}
function finishSSE(res) {
  if (res.writableEnded || res.destroyed) {
    return;
  }
  ensureSSEHeaders(res);
  try {
    res.write("data: [DONE]\n\n");
    res.end();
  } catch {
  }
}
function chunkDelta(id, model, delta, finishReason = null) {
  return {
    id,
    object: "chat.completion.chunk",
    created: Math.floor(Date.now() / 1e3),
    model,
    choices: [
      {
        index: 0,
        delta,
        finish_reason: finishReason
      }
    ]
  };
}
function usageChunk(id, model, usage, proxyCorrection) {
  const chunk = {
    id,
    object: "chat.completion.chunk",
    created: Math.floor(Date.now() / 1e3),
    model,
    choices: [{ index: 0, delta: {}, finish_reason: null }],
    usage
  };
  const correction = proxyCorrection?.trim();
  if (correction) {
    chunk.solomon_proxy_correction = correction;
  }
  return chunk;
}

// src/openai-tools.ts
function allowedToolNamesFromRequest(tools, toolChoice) {
  if (!tools?.length) {
    return null;
  }
  if (toolChoice === "none") {
    return /* @__PURE__ */ new Set();
  }
  const names = /* @__PURE__ */ new Set();
  for (const t of tools) {
    const n = t.function?.name?.trim();
    if (n) {
      names.add(n);
    }
  }
  if (names.size === 0) {
    return null;
  }
  if (typeof toolChoice === "object") {
    const chosen = toolChoice.function.name.trim();
    return names.has(chosen) ? /* @__PURE__ */ new Set([chosen]) : /* @__PURE__ */ new Set();
  }
  return names;
}
function requestUsesNativeTools(tools, toolChoice) {
  return (tools?.length ?? 0) > 0 && toolChoice !== "none";
}
function filterInvocations(invs, allowed) {
  const valid = invs.filter(isValidInvocation);
  if (!allowed) {
    return valid;
  }
  if (allowed.size === 0) {
    return [];
  }
  return valid.filter((inv) => allowed.has(inv.name));
}
function limitInvocations(invs, parallelToolCalls) {
  if (parallelToolCalls === false) {
    return invs.slice(0, 1);
  }
  return invs;
}
function newToolCallId() {
  return `call_${randomBytes2(12).toString("hex")}`;
}
function toolArgumentsJSON(inv) {
  const args = { ...inv.args ?? {} };
  args.intent = invocationIntent(inv) ?? "";
  return JSON.stringify(args);
}
function isValidInvocation(inv) {
  if (!invocationIntent(inv)) {
    return false;
  }
  if (inv.name !== "editFile") {
    return true;
  }
  if (inv.args?.delete === true) {
    const path4 = typeof inv.args?.path === "string" ? inv.args.path.trim() : "";
    if (path4 === "") {
      return false;
    }
    const oldString2 = typeof inv.args?.oldString === "string" ? inv.args.oldString : "";
    const newString2 = typeof inv.args?.newString === "string" ? inv.args.newString : "";
    return oldString2 === "" && newString2 === "";
  }
  const oldString = typeof inv.args?.oldString === "string" ? inv.args.oldString : "";
  const newString = typeof inv.args?.newString === "string" ? inv.args.newString : "";
  return oldString !== "" || newString !== "";
}
function openAIToolCallsFromInvocations(invs) {
  const out = [];
  for (const inv of invs) {
    out.push({
      id: newToolCallId(),
      type: "function",
      function: {
        name: inv.name,
        arguments: toolArgumentsJSON(inv)
      }
    });
  }
  return out;
}
function writeSSEToolCalls(res, completionId, model, invs) {
  if (invs.length === 0) {
    return;
  }
  const toolCalls = invs.map((inv, index) => ({
    index,
    id: newToolCallId(),
    type: "function",
    function: {
      name: inv.name,
      arguments: toolArgumentsJSON(inv)
    }
  }));
  writeSSE(res, chunkDelta(completionId, model, { tool_calls: toolCalls }));
}
function openAIToolsToMcpTools(tools) {
  if (!tools?.length) {
    return [];
  }
  const out = [];
  const seen = /* @__PURE__ */ new Set();
  for (const t of tools) {
    const name = t.function?.name?.trim();
    if (!name || seen.has(name)) {
      continue;
    }
    seen.add(name);
    const params = t.function?.parameters;
    const inputSchema = params && typeof params === "object" && !Array.isArray(params) ? { ...params } : { type: "object", properties: {} };
    out.push({
      name,
      description: t.function?.description?.trim() ?? "",
      inputSchema: schemaWithRequiredToolIntent(inputSchema)
    });
  }
  return out;
}
function parseToolInvocationsFromText(text) {
  const invocations = [];
  let content = text;
  content = content.replace(/<tool_calls\b[^>]*>([\s\S]*?)<\/tool_calls>/gi, (_m, inner) => {
    invocations.push(...parseSolomonTools(String(inner)));
    return "";
  });
  content = content.replace(/<tool_call\b[^>]*>([\s\S]*?)<\/tool_call>/gi, (_m, inner) => {
    const inv = parseJSONToolCall(String(inner));
    if (inv) {
      invocations.push(inv);
    }
    return "";
  });
  content = content.replace(/<functioncall\b[^>]*>([\s\S]*?)<\/functioncall>/gi, (_m, inner) => {
    const inv = parseJSONToolCall(String(inner));
    if (inv) {
      invocations.push(inv);
    }
    return "";
  });
  content = stripEmptyToolCodeFences(content);
  return { content: content.trim(), invocations };
}
function stripEmptyToolCodeFences(text) {
  return text.replace(/```(?:xml|tool_calls|tool_call|functioncall)?[ \t]*\r?\n[\s\r\n]*```/gi, "");
}
function parseSolomonTools(inner) {
  const out = [];
  const re = /<tool\b[^>]*\bname\s*=\s*(["'])(.*?)\1[^>]*>([\s\S]*?)<\/tool>/gi;
  for (let m; m = re.exec(inner); ) {
    const name = unescapeXML(m[2]).trim();
    if (!name) {
      continue;
    }
    const body = m[3];
    const argsRaw = tagText(body, "args") ?? "{}";
    const args = parseJSONObject(unescapeXML(argsRaw));
    if (!args) {
      continue;
    }
    const rawIntent = tagText(body, "intent");
    const intent = rawIntent === null ? readToolIntent(args) ?? "" : unescapeXML(rawIntent).trim();
    out.push({
      name,
      args,
      intent
    });
  }
  return out;
}
function parseJSONToolCall(raw) {
  const obj = parseJSONObject(unescapeXML(raw));
  if (!obj) {
    return null;
  }
  const name = pickString(obj, "name") ?? pickString(obj, "tool") ?? pickString(obj, "tool_name") ?? pickString(obj, "function");
  if (!name) {
    return null;
  }
  const argsRaw = obj.arguments ?? obj.args ?? obj.parameters ?? {};
  const args = normalizeArgsObject(argsRaw);
  if (!args) {
    return null;
  }
  const intent = readToolIntent(args) ?? readToolIntent(obj) ?? "";
  if (!readToolIntent(args) && intent) {
    args.intent = intent;
  }
  return { name, args, intent };
}
function tagText(body, tag) {
  const re = new RegExp(`<${tag}\\b[^>]*>([\\s\\S]*?)<\\/${tag}>`, "i");
  const m = re.exec(body);
  return m ? m[1] : null;
}
function pickString(obj, key) {
  const v = obj[key];
  return typeof v === "string" && v.trim() !== "" ? v.trim() : void 0;
}

// src/bridge/context.ts
var SOLOMON_MCP_PROVIDER = "solomon";
var CUSTOM_USER_TOOLS_MCP_PROVIDER = "custom-user-tools";
function unwrapSolomonMcpCall(eventName, rawArgs) {
  if (eventName !== "mcp") {
    return null;
  }
  if (!rawArgs || typeof rawArgs !== "object") {
    return null;
  }
  const obj = rawArgs;
  if (obj.providerIdentifier !== SOLOMON_MCP_PROVIDER) {
    return null;
  }
  const toolName = typeof obj.toolName === "string" ? obj.toolName.trim() : "";
  if (!toolName) {
    return null;
  }
  return { toolName, args: obj.args ?? {} };
}
function unwrapCustomUserToolsMcpCall(eventName, rawArgs) {
  if (eventName !== "mcp") {
    return null;
  }
  if (!rawArgs || typeof rawArgs !== "object") {
    return null;
  }
  const obj = rawArgs;
  if (obj.providerIdentifier !== CUSTOM_USER_TOOLS_MCP_PROVIDER) {
    return null;
  }
  const toolName = typeof obj.toolName === "string" ? obj.toolName.trim() : "";
  if (!toolName) {
    return null;
  }
  return { toolName, args: obj.args ?? {} };
}

// src/bridge/xml.ts
function formatBridgedToolCallsBlock(tools) {
  const parts = ["<tool_calls>"];
  for (const t of tools) {
    parts.push(`<tool name="${escapeXmlAttr(t.name)}">`);
    parts.push(`<intent>${escapeXmlText(invocationIntent(t) ?? "")}</intent>`);
    parts.push(`<args>${escapeXmlText(JSON.stringify(t.args ?? {}))}</args>`);
    parts.push("</tool>");
  }
  parts.push("</tool_calls>");
  return parts.join("\n");
}

// src/legacy-normalize.ts
var DEFAULT_SUBAGENT_SYS_PATH = ".solomon/cursor-task-sys.txt";
function normalizeSolomonToolArgs(solomonName, viaCursorName, raw) {
  if (solomonName === "subagent") {
    return normalizeSubagentArgsFromRaw(raw);
  }
  if (solomonName === "editFile" && isApplyPatchCursorName(viaCursorName)) {
    return normalizeApplyPatchArgs(raw);
  }
  if (solomonName === "editFile" && isDeleteCursorName(viaCursorName)) {
    return normalizeDeleteEditFileArgs(raw);
  }
  if (solomonName === "find" && isListDirCursorName(viaCursorName)) {
    return normalizeListDirArgs(raw);
  }
  if (solomonName === "find" && isSemanticSearchCursorName(viaCursorName)) {
    return normalizeSemanticSearchArgs(raw);
  }
  if (solomonName === "find") {
    return normalizeFindArgsFromRaw(viaCursorName, raw);
  }
  if (solomonName === "fetchWeb") {
    return normalizeFetchWebArgs(raw);
  }
  if (solomonName === "webSearch") {
    return normalizeWebSearchArgs(raw);
  }
  if (solomonName === "readFile" || solomonName === "shell" || solomonName === "editFile") {
    return normalizeArgs(solomonName, raw);
  }
  return parseArgsObject(raw);
}
function normalizeArgs(solomonName, raw) {
  const obj = parseArgsObject(raw);
  if (obj === null) {
    return null;
  }
  if (solomonName === "readFile") {
    const path4 = pickString2(obj, [
      "path",
      "file_path",
      "filePath",
      "target_file",
      "targetFile",
      "file",
      "filename",
      "relative_path",
      "relativePath"
    ]) ?? "";
    if (!path4) {
      return null;
    }
    const out = { path: path4 };
    const start = pickNumber(obj, ["startLine", "start_line", "offset", "line", "start"]);
    const end = pickNumber(obj, ["endLine", "end_line", "end"]);
    const limit = pickNumber(obj, ["limit", "line_count", "lineCount", "num_lines"]);
    if (start !== void 0) {
      out.startLine = start;
    }
    if (end !== void 0) {
      out.endLine = end;
    } else if (start !== void 0 && limit !== void 0 && limit > 0) {
      out.endLine = start + limit - 1;
    }
    return out;
  }
  if (solomonName === "shell") {
    const command = pickString2(obj, ["command", "cmd", "script", "shell_command"]) ?? "";
    if (!command) {
      return null;
    }
    const out = {
      command,
      intent: pickString2(obj, ["intent", "description", "explanation"]) ?? "run command"
    };
    if (typeof obj.timeoutSeconds === "number") {
      out.timeoutSeconds = obj.timeoutSeconds;
    }
    return out;
  }
  if (solomonName === "editFile") {
    const path4 = pickString2(obj, ["path", "file_path", "filePath", "target_file"]) ?? "";
    if (!path4) {
      return null;
    }
    const renameTo = pickString2(obj, ["renameTo", "rename_to", "new_path", "newPath", "destination"]) ?? "";
    if (renameTo) {
      const oldString2 = pickString2(obj, ["oldString", "old_string", "oldText"]) ?? "";
      const newString2 = pickString2(obj, ["newString", "new_string", "newText", "content"]) ?? "";
      if (pickBoolean(obj, ["delete"]) || oldString2 !== "" || newString2 !== "") {
        return null;
      }
      return {
        path: path4,
        renameTo,
        intent: pickString2(obj, ["intent", "description", "explanation"]) ?? "rename file"
      };
    }
    if (pickBoolean(obj, ["delete"])) {
      const oldString2 = pickString2(obj, ["oldString", "old_string", "oldText"]) ?? "";
      const newString2 = pickString2(obj, ["newString", "new_string", "newText", "content"]) ?? "";
      if (oldString2 !== "" || newString2 !== "") {
        return null;
      }
      return {
        path: path4,
        delete: true,
        intent: pickString2(obj, ["intent", "description", "explanation"]) ?? "delete file"
      };
    }
    const oldString = pickString2(obj, ["oldString", "old_string", "oldText", "old"]) ?? "";
    const newString = pickString2(obj, ["newString", "new_string", "newText", "content", "replace", "new"]) ?? "";
    if (oldString === "" && newString === "") {
      return null;
    }
    return {
      path: path4,
      oldString,
      newString,
      intent: pickString2(obj, ["intent", "description", "explanation"]) ?? "edit file"
    };
  }
  return obj;
}
function normalizeFetchWebArgs(raw) {
  const obj = parseArgsObject(raw);
  if (!obj) {
    return null;
  }
  const url = pickString2(obj, ["url", "uri", "target_url", "targetUrl", "href"]) ?? "";
  if (!url) {
    return null;
  }
  const out = { url };
  const timeout = pickNumber(obj, ["timeoutSeconds", "timeout_seconds", "timeout"]);
  if (timeout !== void 0) {
    out.timeoutSeconds = timeout;
  }
  return out;
}
function normalizeWebSearchArgs(raw) {
  const obj = parseArgsObject(raw);
  if (!obj) {
    return null;
  }
  const query = pickString2(obj, ["query", "search_term", "searchQuery", "q"]) ?? "";
  if (!query) {
    return null;
  }
  const out = { query };
  const engine = pickString2(obj, ["engine"]);
  if (engine) {
    out.engine = engine;
  }
  const maxResults = pickNumber(obj, ["maxResults", "max_results", "num_results"]);
  if (maxResults !== void 0) {
    out.maxResults = maxResults;
  }
  const timeout = pickNumber(obj, ["timeoutSeconds", "timeout_seconds", "timeout"]);
  if (timeout !== void 0) {
    out.timeoutSeconds = timeout;
  }
  if (obj.extras && typeof obj.extras === "object") {
    out.extras = obj.extras;
  }
  return out;
}
function isDeleteCursorName(name) {
  return name.trim().toLowerCase() === "delete";
}
function isApplyPatchCursorName(name) {
  const n = name.trim().toLowerCase();
  return n === "applypatch" || n === "apply_patch";
}
function normalizeApplyPatchArgs(raw) {
  const obj = parseArgsObject(raw);
  if (!obj) {
    return null;
  }
  const path4 = pickString2(obj, ["path", "file_path", "filePath", "target_file"]) ?? "";
  if (!path4) {
    return null;
  }
  const oldString = pickString2(obj, ["oldString", "old_string", "oldText", "old"]) ?? "";
  const newString = pickString2(obj, ["newString", "new_string", "newText", "content", "replace", "new"]) ?? "";
  if (oldString !== "" || newString !== "") {
    if (oldString === "" && newString === "") {
      return null;
    }
    return {
      path: path4,
      oldString,
      newString,
      intent: pickString2(obj, ["intent", "description", "explanation"]) ?? "apply patch"
    };
  }
  const patch = pickString2(obj, ["patch", "diff"]) ?? "";
  if (patch.includes("@@")) {
    return null;
  }
  if (patch.trim() !== "") {
    return {
      path: path4,
      oldString: "",
      newString: patch,
      intent: pickString2(obj, ["intent", "description", "explanation"]) ?? "apply patch"
    };
  }
  return null;
}
function isListDirCursorName(name) {
  const n = name.trim().toLowerCase();
  return n === "listdir" || n === "list_dir" || n === "ls";
}
function normalizeListDirArgs(raw) {
  const obj = parseArgsObject(raw);
  if (!obj) {
    return null;
  }
  const dirPath = pickString2(obj, ["path", "target_directory", "targetDirectory", "directory"]) ?? ".";
  const pattern = pickString2(obj, ["pattern", "glob_pattern", "globPattern"]) ?? "**/*";
  const out = { pattern, files: true, path: dirPath };
  const hl = pickNumber(obj, ["headLimit", "head_limit"]);
  if (hl !== void 0) {
    out.headLimit = hl;
  }
  return out;
}
function normalizeDeleteEditFileArgs(raw) {
  const obj = parseArgsObject(raw);
  if (!obj) {
    return null;
  }
  const path4 = pickString2(obj, ["path", "file_path", "filePath", "target_file"]) ?? "";
  if (!path4) {
    return null;
  }
  const oldString = pickString2(obj, ["oldString", "old_string", "oldText"]) ?? "";
  const newString = pickString2(obj, ["newString", "new_string", "newText", "content"]) ?? "";
  if (oldString !== "" || newString !== "") {
    return null;
  }
  return {
    path: path4,
    delete: true,
    intent: pickString2(obj, ["intent", "description", "explanation"]) ?? "delete file"
  };
}
function isSemanticSearchCursorName(name) {
  const n = name.trim().toLowerCase();
  return n === "semanticsearch" || n === "semantic_search";
}
function normalizeSemanticSearchArgs(raw) {
  const obj = parseArgsObject(raw);
  if (!obj) {
    return null;
  }
  const pattern = pickString2(obj, ["query", "search_term", "searchQuery", "pattern", "regex"]) ?? "";
  if (!pattern) {
    return null;
  }
  const out = { pattern, files: false };
  const path4 = pickString2(obj, ["path", "target_directory", "targetDirectory"]);
  if (path4) {
    out.path = path4;
  }
  const dirs = obj.target_directories;
  if (Array.isArray(dirs)) {
    for (const d of dirs) {
      if (typeof d === "string" && d.trim() !== "") {
        out.path = d;
        break;
      }
    }
  }
  const pg = pickString2(obj, ["pathGlob", "glob", "glob_pattern", "globPattern"]);
  if (pg) {
    out.pathGlob = pg;
  }
  const hl = pickNumber(obj, ["headLimit", "head_limit", "num_results"]);
  if (hl !== void 0) {
    out.headLimit = hl;
  }
  return out;
}
function normalizeSubagentArgsFromRaw(raw) {
  const obj = parseArgsObject(raw);
  if (!obj) {
    return null;
  }
  const resume = pickString2(obj, ["resume", "subchatId", "subchat_id"]);
  const task = pickString2(obj, ["task", "prompt", "description", "message", "user_query"]) ?? "";
  if (!task && !resume) {
    return null;
  }
  const sysPromptPath = pickString2(obj, ["sysPromptPath", "sys_prompt_path", "systemPromptPath"]) ?? DEFAULT_SUBAGENT_SYS_PATH;
  const out = { sysPromptPath };
  if (task) {
    out.task = task;
  }
  if (resume) {
    out.resume = resume;
  }
  const bg = pickOptionalBool(obj, ["run_in_background", "runInBackground", "background"]);
  if (bg !== void 0) {
    out.run_in_background = bg;
  }
  const interrupt = pickOptionalBool(obj, ["interrupt"]);
  if (interrupt !== void 0) {
    out.interrupt = interrupt;
  }
  const reasoning = pickString2(obj, ["reasoningEffort", "reasoning_effort"]);
  if (reasoning) {
    out.reasoningEffort = reasoning;
  }
  const intent = pickString2(obj, ["intent", "description"]);
  if (intent && intent !== task) {
    out.intent = intent;
  } else if (task) {
    out.intent = "nested task";
  }
  return out;
}
function normalizeFindArgsFromRaw(cursorName, raw) {
  const obj = parseArgsObject(raw);
  if (!obj) {
    return null;
  }
  return normalizeFindArgs(cursorName, obj);
}
function normalizeFindArgs(cursorName, obj) {
  const n = cursorName.toLowerCase();
  let files = n === "glob";
  if (typeof obj.files === "boolean") {
    files = obj.files;
  }
  const pattern = files ? pickString2(obj, ["pattern", "glob_pattern", "globPattern"]) ?? "" : pickString2(obj, ["pattern", "query", "regex"]) ?? "";
  if (!pattern) {
    return null;
  }
  const out = { pattern, files };
  const path4 = pickString2(obj, ["path", "target_directory", "targetDirectory"]);
  if (path4) {
    out.path = path4;
  }
  if (!files) {
    const pg = pickString2(obj, ["pathGlob", "glob", "glob_pattern", "globPattern"]);
    if (pg) {
      out.pathGlob = pg;
    }
    const om = pickString2(obj, ["outputMode", "output_mode"]);
    if (om) {
      out.outputMode = om;
    }
    if (obj.caseInsensitive === true || obj["-i"] === true) {
      out.caseInsensitive = true;
    }
    const hl = pickNumber(obj, ["headLimit", "head_limit"]);
    if (hl !== void 0) {
      out.headLimit = hl;
    }
  } else {
    const hl = pickNumber(obj, ["headLimit", "head_limit"]);
    if (hl !== void 0) {
      out.headLimit = hl;
    }
  }
  return out;
}
function pickString2(obj, keys) {
  for (const k of keys) {
    const v = obj[k];
    if (typeof v === "string" && v.trim() !== "") {
      return v;
    }
  }
  return void 0;
}
function pickNumber(obj, keys) {
  for (const k of keys) {
    const v = obj[k];
    if (typeof v === "number" && Number.isFinite(v)) {
      return Math.trunc(v);
    }
  }
  return void 0;
}
function pickOptionalBool(obj, keys) {
  for (const k of keys) {
    const v = obj[k];
    if (typeof v === "boolean") {
      return v;
    }
  }
  return void 0;
}
function pickBoolean(obj, keys) {
  for (const k of keys) {
    const v = obj[k];
    if (typeof v === "boolean") {
      return v;
    }
  }
  return false;
}

// src/bridge/invocation.ts
function isAllowedSolomonTool(name, ctx) {
  if (!ctx.allowedNames) {
    return true;
  }
  return ctx.allowedNames.has(name);
}
function mapCursorToolInvocation(eventName, rawArgs, ctx) {
  const trimmed = eventName.trim();
  if (!trimmed) {
    return null;
  }
  const solomonName = resolveBridgedSolomonName(trimmed, ctx.allowedNames);
  if (!solomonName) {
    return null;
  }
  if (!isValidSolomonToolName(solomonName)) {
    return null;
  }
  if (!isAllowedSolomonTool(solomonName, ctx)) {
    return null;
  }
  const intent = readToolIntent(rawArgs);
  if (!intent) {
    return null;
  }
  const args = normalizeSolomonToolArgs(solomonName, trimmed, rawArgs);
  if (!args) {
    return null;
  }
  return invocationWithIntent(solomonName, args, intent);
}
function bridgeToolInvocation(eventName, rawArgs, ctx) {
  const trimmed = eventName.trim();
  if (!trimmed) {
    return null;
  }
  if (shouldHardDenyCursorTool(trimmed)) {
    return null;
  }
  const nativeException = isExposedNativePolicyException(trimmed, ctx.allowedNames);
  if (shouldRedirectCursorTool(trimmed) && !nativeException) {
    return null;
  }
  const mapped = mapCursorToolInvocation(eventName, rawArgs, ctx);
  if (!mapped) {
    return null;
  }
  if (shouldBlockDeferredSolomonTool(mapped.name) && !nativeException) {
    return null;
  }
  return mapped;
}
function collectBridgedTool(pending, name, rawArgs, ctx) {
  const inv = bridgeToolInvocation(name, rawArgs, ctx);
  if (inv) {
    pending.push(inv);
  }
}
function tryCollectBridgedTool(pending, name, rawArgs, ctx) {
  const before = pending.length;
  collectBridgedTool(pending, name, rawArgs, ctx);
  return pending.length > before;
}
function invocationWithIntent(solomonName, args, intent) {
  delete args.intent;
  delete args.description;
  return { name: solomonName, args, intent };
}

// src/chat/helpers/proxy-correction.ts
var ORCHESTRATE_FOOTER = "Cursor built-ins are disabled. Use native tool_calls only: searchTools (discover deferred SDK signatures), orchestrate (run workspace scripts), searchSkill and loadSkill (skills).";
var CHAT_FOOTER = "This is CHAT mode. Use native tool_calls only: docsRetrieval, readChat, webSearch, fetchWeb, deepResearch, researchStatus, or switchMode; switchMode before workspace implementation.";
function isChatSurface(allowedNames) {
  if (!allowedNames || allowedNames.has("orchestrate")) {
    return false;
  }
  return ["fetchWeb", "webSearch", "deepResearch", "researchStatus"].some(
    (name) => allowedNames.has(name)
  );
}
function proxyToolCorrectionMessage(blocked, allowedNames) {
  const unique = [...new Set(blocked.map((n) => n.trim()).filter(Boolean))];
  if (unique.length === 0) {
    return "";
  }
  const chatSurface = isChatSurface(allowedNames);
  const parts = [`Blocked by Solomon proxy: ${unique.join(", ")}.`];
  const hints = [];
  for (const name of unique) {
    const hint = correctionHintForBlockedTool(name, chatSurface);
    if (hint) {
      hints.push(hint);
    }
  }
  if (hints.length > 0) {
    parts.push(hints.join(" "));
  }
  if (unique.some(
    (n) => !isHardDenyBlockedLabel(n) && (shouldRedirectCursorTool(n) || n.startsWith("mcp:"))
  )) {
    parts.push(chatSurface ? CHAT_FOOTER : ORCHESTRATE_FOOTER);
  }
  parts.push("Reply with a corrected invocation or plain text.");
  return parts.join(" ");
}

// src/cursor-native-tools.ts
function cursorToolEventChunk(completionId, model, event) {
  return {
    id: completionId,
    object: "chat.completion.chunk",
    created: Math.floor(Date.now() / 1e3),
    model,
    choices: [{ index: 0, delta: {}, finish_reason: null }],
    solomon_cursor_tool_event: event
  };
}
function truncate(s, max) {
  s = s.trim();
  if (max < 1 || s.length <= max) {
    return s;
  }
  return s.slice(0, max) + "\u2026";
}
function jsonString(v) {
  if (typeof v === "string") {
    return v.trim();
  }
  return "";
}
function argMap(args) {
  if (!args || typeof args !== "object" || Array.isArray(args)) {
    return {};
  }
  return args;
}
function pickArgPreview(m) {
  for (const key of ["path", "command", "query", "pattern", "url", "task", "prompt", "description"]) {
    const s = jsonString(m[key]);
    if (s) {
      return s;
    }
  }
  try {
    return truncate(JSON.stringify(m), 160);
  } catch {
    return "";
  }
}
function pickResultPreview(result) {
  if (result === void 0 || result === null) {
    return "";
  }
  if (typeof result === "string") {
    return truncate(result, 200);
  }
  if (typeof result !== "object") {
    return truncate(String(result), 200);
  }
  const m = result;
  for (const key of ["output", "content", "text", "message", "stdout", "stderr"]) {
    const s = jsonString(m[key]);
    if (s) {
      return truncate(s, 200);
    }
  }
  if (Array.isArray(m.content)) {
    const parts = [];
    for (const block of m.content) {
      if (!block || typeof block !== "object") {
        continue;
      }
      const b = block;
      const s = jsonString(b.text) || jsonString(b.content);
      if (s) {
        parts.push(s);
      }
    }
    if (parts.length > 0) {
      return truncate(parts.join("\n"), 200);
    }
  }
  try {
    return truncate(JSON.stringify(result), 200);
  } catch {
    return "done";
  }
}
function formatUnmappedToolDisplayLine(name, status, args, result, error) {
  const label = name.trim() || "tool";
  if (status === "error") {
    return (error?.trim() || "failed") + ` (${label})`;
  }
  const preview = pickArgPreview(argMap(args));
  if (status === "running") {
    return preview || "\u2026";
  }
  if (status === "completed") {
    const body = pickResultPreview(result);
    if (body) {
      return body;
    }
    return preview ? `${preview} \u2192 done` : "done";
  }
  return preview;
}
function unmappedToolEvent(name, status, args, result, error, callId) {
  const ev = {
    name: name.trim() || "tool",
    status,
    displayLine: formatUnmappedToolDisplayLine(name, status, args, result, error)
  };
  if (callId) {
    ev.callId = callId;
  }
  if (args !== void 0) {
    ev.args = args;
  }
  if (result !== void 0) {
    ev.result = result;
  }
  if (error) {
    ev.error = error;
  }
  return ev;
}
function toolCallErrorMessage(event) {
  const err = event.error;
  if (typeof err === "string" && err.trim() !== "") {
    return err;
  }
  if (err && typeof err === "object" && typeof err.message === "string") {
    return err.message;
  }
  return void 0;
}
function unmappedToolEventFromToolCall(event) {
  const mcp = unwrapSolomonMcpCall(event.name, event.args);
  const name = mcp ? `mcp:${mcp.toolName}` : event.name;
  const args = mcp ? mcp.args : event.args;
  const status = event.status === "error" ? "error" : event.status === "completed" ? "completed" : "running";
  return unmappedToolEvent(
    name,
    status,
    args,
    status === "completed" ? event.result : void 0,
    status === "error" ? toolCallErrorMessage(event) : void 0,
    event.call_id
  );
}

// src/chat/helpers/stream-events.ts
function requiresSolomonIntent(name, bridgeCtx) {
  const trimmed = name.trim();
  return bridgeCtx.allowedNames?.has(trimmed) === true || isSolomonCanonicalTool(trimmed);
}
function wouldBridgeTool(name, rawArgs, bridgeCtx) {
  const solomonMcp = unwrapSolomonMcpCall(name, rawArgs);
  if (solomonMcp) {
    return bridgeToolInvocation(solomonMcp.toolName, solomonMcp.args, bridgeCtx) !== null;
  }
  const customMcp = unwrapCustomUserToolsMcpCall(name, rawArgs);
  if (customMcp) {
    return bridgeToolInvocation(customMcp.toolName, customMcp.args, bridgeCtx) !== null;
  }
  return bridgeToolInvocation(name, rawArgs, bridgeCtx) !== null;
}
function collectBridgedMcpCall(pendingBridged, bridgeCtx, onToolDetected, onBlockedTool, mcp) {
  if (tryCollectBridgedTool(pendingBridged, mcp.toolName, mcp.args, bridgeCtx)) {
    onToolDetected();
    return true;
  }
  const baseLabel = blockedMcpToolLabel(mcp.toolName);
  const label = readToolIntent(mcp.args) ? baseLabel : missingIntentBlockedLabel(baseLabel);
  if (onBlockedTool) {
    onBlockedTool(label);
  }
  if (shouldStopProxyOnBlockedTool(label)) {
    onToolDetected();
  }
  return false;
}
function processStreamEvent(event, allowCursorInternalTools2, onText, onThinking, pendingBridged, onToolDetected, onBlockedTool, bridgeCtx = { allowedNames: null }, onUnmappedToolEvent) {
  const reportBlocked = (name) => {
    if (onBlockedTool) {
      onBlockedTool(name);
    }
    if (shouldStopProxyOnBlockedTool(name)) {
      onToolDetected();
    }
  };
  const handleToolProposal = (name, rawArgs, callId) => {
    if (shouldHardDenyCursorTool(name)) {
      reportBlocked(name);
      return;
    }
    if (shouldRedirectCursorTool(name) && !isExposedNativePolicyException(name, bridgeCtx.allowedNames)) {
      reportBlocked(name);
      return;
    }
    const solomonMcp = unwrapSolomonMcpCall(name, rawArgs);
    if (solomonMcp) {
      collectBridgedMcpCall(pendingBridged, bridgeCtx, onToolDetected, onBlockedTool, solomonMcp);
      return;
    }
    const customMcp = unwrapCustomUserToolsMcpCall(name, rawArgs);
    if (customMcp) {
      collectBridgedMcpCall(pendingBridged, bridgeCtx, onToolDetected, onBlockedTool, customMcp);
      return;
    }
    if (name === "mcp") {
      reportBlocked(BLOCKED_MCP_EXTERNAL_LABEL);
      return;
    }
    if (requiresSolomonIntent(name, bridgeCtx) && !readToolIntent(rawArgs)) {
      reportBlocked(missingIntentBlockedLabel(name));
      return;
    }
    if (tryCollectBridgedTool(pendingBridged, name, rawArgs, bridgeCtx)) {
      onToolDetected();
      return;
    }
    if (!allowCursorInternalTools2) {
      reportBlocked(name);
      return;
    }
    if (onUnmappedToolEvent) {
      onUnmappedToolEvent(unmappedToolEvent(name, "running", rawArgs, void 0, void 0, callId));
    }
  };
  if (event.type === "assistant") {
    let afterTool = false;
    for (const block of event.message.content) {
      if (block.type === "tool_use") {
        afterTool = true;
        handleToolProposal(block.name, block.input, block.id);
        continue;
      }
      if (block.type === "text" && block.text && !afterTool) {
        onText(block.text);
      }
    }
    return;
  }
  if (event.type === "thinking" && event.text) {
    onThinking(event.text);
    return;
  }
  if (event.type === "tool_call") {
    if (wouldBridgeTool(event.name, event.args, bridgeCtx)) {
      if (event.status === "completed" || event.status === "error") {
        return;
      }
      if (event.args !== void 0) {
        handleToolProposal(event.name, event.args, event.call_id);
      }
      return;
    }
    const solomonMcp = unwrapSolomonMcpCall(event.name, event.args);
    const customMcp = unwrapCustomUserToolsMcpCall(event.name, event.args);
    if (event.status === "completed" || event.status === "error") {
      if (shouldHardDenyCursorTool(event.name)) {
        reportBlocked(event.name);
        return;
      }
      if (solomonMcp || customMcp) {
        handleToolProposal(event.name, event.args, event.call_id);
        return;
      }
      if (requiresSolomonIntent(event.name, bridgeCtx) && !readToolIntent(event.args)) {
        reportBlocked(missingIntentBlockedLabel(event.name));
        return;
      }
      if (allowCursorInternalTools2 && onUnmappedToolEvent) {
        onUnmappedToolEvent(unmappedToolEventFromToolCall(event));
      }
      return;
    }
    const enforcePolicy = !allowCursorInternalTools2 || shouldHardDenyCursorTool(event.name) || shouldRedirectCursorTool(event.name) || solomonMcp !== null || customMcp !== null || event.name === "mcp";
    if (enforcePolicy) {
      if (event.args !== void 0) {
        handleToolProposal(event.name, event.args, event.call_id);
      } else {
        reportBlocked(event.name);
      }
      return;
    }
    if (onUnmappedToolEvent) {
      onUnmappedToolEvent(unmappedToolEventFromToolCall(event));
    }
    return;
  }
  if (event.type === "task" && event.text) {
    onThinking(event.text);
  }
}

// src/chat/helpers/usage.ts
function finishReasonForTools(bridgedCount, nativeTools) {
  if (bridgedCount > 0 && nativeTools) {
    return "tool_calls";
  }
  return "stop";
}
function nativeInvocationsFromText(text, turnOpts) {
  const parsed = parseToolInvocationsFromText(text);
  const blockedTools = [];
  const valid = parsed.invocations.filter((inv) => {
    if (shouldHardDenyCursorTool(inv.name)) {
      blockedTools.push(inv.name);
      return false;
    }
    if (shouldBlockDeferredSolomonTool(inv.name) && !isExposedNativePolicyException(inv.name, turnOpts.allowedNames)) {
      blockedTools.push(inv.name);
      return false;
    }
    if (isValidInvocation(inv)) {
      return true;
    }
    blockedTools.push(
      invocationIntent(inv) ? `${inv.name}:invalid` : missingIntentBlockedLabel(inv.name)
    );
    return false;
  });
  return {
    content: parsed.content,
    invocations: limitInvocations(
      filterInvocations(valid, turnOpts.allowedNames),
      turnOpts.parallelToolCalls
    ),
    blockedTools
  };
}
function nextTextChunk(current, incoming) {
  if (!incoming) {
    return "";
  }
  incoming = collapseExactRepeat(incoming);
  if (incoming.startsWith(current)) {
    return incoming.slice(current.length);
  }
  if (current.endsWith(incoming)) {
    return "";
  }
  return incoming;
}
function collapseExactRepeat(text) {
  const n = text.length;
  if (n < 2 || n % 2 !== 0) {
    return text;
  }
  const half = n / 2;
  const left = text.slice(0, half);
  return left === text.slice(half) ? left : text;
}
function buildOpenAIUsage(messages, sdkUsage, textBuf, thinkingBuf) {
  const estPrompt = roughTokFromMessages(messages);
  const estReason = roughTokFromString(thinkingBuf);
  const estResp = roughTokFromString(textBuf);
  let prompt = sdkUsage?.inputTokens ?? 0;
  let completion = sdkUsage?.outputTokens ?? 0;
  const cached = sdkUsage?.cacheReadTokens ?? 0;
  if (prompt <= 0) {
    prompt = estPrompt;
  }
  if (completion <= 0) {
    completion = estReason + estResp;
  }
  let reasoning = estReason;
  if (reasoning > completion) {
    reasoning = completion;
  }
  if (thinkingBuf.length === 0) {
    reasoning = 0;
  }
  const total = prompt + completion;
  const out = {
    prompt_tokens: prompt,
    completion_tokens: completion,
    total_tokens: total > 0 ? total : prompt + completion
  };
  if (cached > 0) {
    out.prompt_tokens_details = { cached_tokens: cached };
  }
  if (reasoning > 0) {
    out.completion_tokens_details = { reasoning_tokens: reasoning };
  }
  return out;
}

// src/chat/turn.ts
function promptToolsFromRequest(req) {
  if (req.tool_choice === "none") {
    return void 0;
  }
  if (typeof req.tool_choice === "object") {
    const chosen = req.tool_choice.function.name;
    return req.tools?.filter((t) => t.function.name === chosen);
  }
  return req.tools;
}
function turnOptsFromRequest(req) {
  return {
    tools: promptToolsFromRequest(req),
    nativeTools: requestUsesNativeTools(req.tools, req.tool_choice),
    allowedNames: allowedToolNamesFromRequest(req.tools, req.tool_choice),
    parallelToolCalls: req.parallel_tool_calls
  };
}
function streamUsageInput(messages, sdkUsage, textBuf, thinkingBuf) {
  return {
    messages,
    sdkUsage,
    textBuf,
    thinkingBuf,
    buildUsage: buildOpenAIUsage
  };
}
function resolveProxyCorrection(blockedTools, bridgedCount, turnOpts) {
  if (blockedTools.length === 0 || bridgedCount > 0) {
    return void 0;
  }
  const msg = proxyToolCorrectionMessage(blockedTools, turnOpts.allowedNames);
  return msg || void 0;
}
function finalizeTurnToolResults(state2, content, turnOpts) {
  const blockedTools = [...state2.blockedTools];
  let mergedContent = content;
  let nativeInvocations = [];
  if (turnOpts.nativeTools) {
    const parsed = nativeInvocationsFromText(content, turnOpts);
    mergedContent = parsed.content;
    nativeInvocations = parsed.invocations;
    blockedTools.push(...parsed.blockedTools);
  }
  const bridged = limitInvocations(
    filterInvocations([...nativeInvocations, ...state2.pendingBridged], turnOpts.allowedNames),
    turnOpts.parallelToolCalls
  );
  return {
    content: mergedContent,
    bridged,
    blockedTools,
    proxyCorrection: resolveProxyCorrection(blockedTools, bridged.length, turnOpts)
  };
}
function emitBufferedReasoning(res, completionId, model, thinkingBuf) {
  if (thinkingBuf) {
    writeSSE(res, chunkDelta(completionId, model, { reasoning_content: thinkingBuf }));
  }
}
function emitBufferedContent(res, completionId, model, content) {
  if (content) {
    writeSSE(res, chunkDelta(completionId, model, { content }));
  }
}

// src/run-control.ts
var runReleaseTimeoutMs = 15e3;
async function forceStopRun(run) {
  if (!run?.supports("cancel")) {
    return;
  }
  try {
    await run.cancel();
  } catch {
  }
}
async function waitRun(run) {
  if (!run) {
    return;
  }
  try {
    await run.wait();
  } catch {
  }
}
async function releaseRun(run, timeoutMs = runReleaseTimeoutMs) {
  if (!run) {
    return true;
  }
  await forceStopRun(run);
  let released = false;
  await Promise.race([
    waitRun(run).then(() => {
      released = true;
    }),
    new Promise((resolve2) => {
      setTimeout(resolve2, timeoutMs);
    })
  ]);
  return released;
}
async function finalizeAgentRun(run) {
  await releaseRun(run);
}
function clientAbortFromRequest(req) {
  return {
    onAborted: (listener) => {
      req.on("aborted", listener);
    },
    offAborted: (listener) => {
      req.off("aborted", listener);
    }
  };
}
function wireClientAbort(clientAbort, res, getRun, onAbort) {
  let fired = false;
  const fire = () => {
    if (fired) {
      return;
    }
    fired = true;
    onAbort();
    void forceStopRun(getRun());
  };
  clientAbort.onAborted(fire);
  const onResClose = () => {
    if (!res.writableFinished) {
      fire();
    }
  };
  res.on("close", onResClose);
  return () => {
    clientAbort.offAborted(fire);
    res.off("close", onResClose);
  };
}
function endOpenAIStream(res, completionId, model, usage, finishReason = "stop", proxyCorrection) {
  if (res.writableEnded || res.destroyed) {
    return;
  }
  writeSSE(res, usageChunk(completionId, model, usage, proxyCorrection));
  writeSSE(res, chunkDelta(completionId, model, {}, finishReason));
  finishSSE(res);
}
function finishStreamWithUsage(res, completionId, model, input, finishReason = "stop", proxyCorrection) {
  endOpenAIStream(
    res,
    completionId,
    model,
    input.buildUsage(input.messages, input.sdkUsage, input.textBuf, input.thinkingBuf),
    finishReason,
    proxyCorrection
  );
}

// src/cursor-agent.ts
import { Agent } from "@cursor/sdk";
import * as fs3 from "node:fs";
import * as path3 from "node:path";

// src/custom-tools.ts
var SOLOMON_CUSTOM_TOOL_DELEGATE_MSG = "Solomon host owns execution \u2014 this callback must not run workspace work in Node.";
function solomonCustomToolPrehookExecute(_args, _ctx) {
  return {
    content: [{ type: "text", text: SOLOMON_CUSTOM_TOOL_DELEGATE_MSG }],
    isError: true
  };
}
function solomonCustomToolsFromOpenAI(tools) {
  const defs = openAIToolsToMcpTools(tools);
  if (defs.length === 0) {
    return void 0;
  }
  const out = {};
  for (const def of defs) {
    out[def.name] = {
      description: def.description,
      inputSchema: def.inputSchema,
      execute: solomonCustomToolPrehookExecute
    };
  }
  return out;
}

// src/cursor-agent.ts
var DEFAULT_SUBAGENT_SYS_PROMPT = [
  "You are a nested Solomon agent running a scoped sub-task behind the remote host harness.",
  'Use searchTools to discover deferred tools and MCP schemas, then orchestrate (package main, import "sdk") for workspace and MCP work. Invoke connected MCP tools as sdk.mcp.<tool>(intent, args); never emit MCP.<server>.<tool> as a direct native call. MCP resources and prompts remain host-managed. Never claim an MCP action without an actual host tool result.',
  "Emit registered Solomon tools (orchestrate, searchTools, subagent, listSubAgents, switchMode, searchSkill, loadSkill, docsRetrieval, readChat) by name \u2014 execution runs on the Solomon host in Go.",
  "Cursor built-ins (Read, StrReplace, Shell, Task, browser_*, ApplyPatch, \u2026) are blocked on this host.",
  "Stay focused on the assigned task and return a concise result."
].join("\n");
function ensureDefaultSubagentSysPrompt(projRoot) {
  const file = path3.join(projRoot, DEFAULT_SUBAGENT_SYS_PATH);
  if (fs3.existsSync(file)) {
    return;
  }
  fs3.mkdirSync(path3.dirname(file), { recursive: true });
  fs3.writeFileSync(file, DEFAULT_SUBAGENT_SYS_PROMPT + "\n", "utf8");
}
async function createAgentWithOptions(cfg, modelSelection, sandbox, tools) {
  ensureDefaultSubagentSysPrompt(cfg.cwd);
  const customTools = cfg.allowCursorInternalTools ? void 0 : solomonCustomToolsFromOpenAI(tools);
  const localBase = {
    cwd: cfg.cwd,
    settingSources: [],
    ...customTools ? { customTools } : {}
  };
  if (cfg.allowCursorInternalTools) {
    return Agent.create({
      apiKey: cfg.apiKey,
      model: modelSelection,
      local: { cwd: cfg.cwd, settingSources: [] }
    });
  }
  return Agent.create({
    apiKey: cfg.apiKey,
    model: modelSelection,
    local: { ...localBase, sandboxOptions: { enabled: sandbox } }
  });
}
async function createAgent(cfg, modelSelection, tools) {
  try {
    return await createAgentWithOptions(cfg, modelSelection, true, tools);
  } catch (err) {
    const msg = err instanceof Error ? err.message.toLowerCase() : String(err).toLowerCase();
    if (!cfg.allowCursorInternalTools && msg.includes("sandbox")) {
      return createAgentWithOptions(cfg, modelSelection, false, tools);
    }
    throw err;
  }
}
async function disposeAgent(agent) {
  if (!agent) {
    return;
  }
  try {
    await agent[Symbol.asyncDispose]();
  } catch {
  }
}
async function sendStateless(cfg, modelSelection, messages, sendOpts, tools) {
  const agent = await createAgent(cfg, modelSelection, tools);
  const prompt = buildPromptFromMessages(messages, tools);
  const customTools = cfg.allowCursorInternalTools ? void 0 : solomonCustomToolsFromOpenAI(tools);
  try {
    const run = await agent.send(prompt, {
      ...sendOpts,
      ...customTools ? { local: { customTools } } : {}
    });
    return { agent, run };
  } catch (err) {
    await disposeAgent(agent);
    throw err;
  }
}

// src/chat/helpers/stream-loop.ts
function createAgentToolStreamState() {
  return { pendingBridged: [], blockedTools: [], toolDetected: false };
}
function shouldForceStopProxyRun(state2) {
  if (state2.toolDetected && state2.pendingBridged.length > 0) {
    return true;
  }
  return state2.blockedTools.some(shouldStopProxyOnBlockedTool);
}
async function drainAgentToolStream(run, allowCursorInternalTools2, bridgeCtx, handlers, state2, options = {}) {
  for await (const event of run.stream()) {
    if (options.shouldStop?.()) {
      break;
    }
    processStreamEvent(
      event,
      allowCursorInternalTools2,
      handlers.onText,
      handlers.onThinking,
      state2.pendingBridged,
      () => {
        state2.toolDetected = true;
      },
      (name) => {
        state2.blockedTools.push(name);
        if (shouldStopProxyOnBlockedTool(name)) {
          state2.toolDetected = true;
        }
      },
      bridgeCtx,
      handlers.onUnmappedToolEvent
    );
    if (shouldForceStopProxyRun(state2)) {
      if (state2.pendingBridged.length > 0) {
        options.onBridgedCollected?.();
      }
      await forceStopRun(run);
      break;
    }
  }
}

// src/chat/stream.ts
async function streamCompletion(cfg, messages, completionId, model, modelSelection, clientAbort, res, turnOpts) {
  let sdkUsage;
  let run;
  let agent;
  let clientAborted = false;
  const sendOpts = {
    model: modelSelection,
    onDelta: async ({ update }) => {
      if (update.type !== "turn-ended" || !update.usage) {
        return;
      }
      sdkUsage = {
        inputTokens: update.usage.inputTokens ?? 0,
        outputTokens: update.usage.outputTokens ?? 0,
        cacheReadTokens: update.usage.cacheReadTokens
      };
    }
  };
  const unwireAbort = wireClientAbort(clientAbort, res, () => run, () => {
    clientAborted = true;
  });
  res.writeHead(200, SSE_RESPONSE_HEADERS);
  res.on("error", () => {
  });
  try {
    try {
      const sent = await sendStateless(cfg, modelSelection, messages, sendOpts, turnOpts.tools);
      agent = sent.agent;
      run = sent.run;
    } catch (err) {
      sendStreamStartError(res, completionId, model, err);
      return;
    }
    if (clientAborted) {
      if (res.headersSent) {
        finishStreamWithUsage(res, completionId, model, {
          messages,
          sdkUsage,
          textBuf: "",
          thinkingBuf: "",
          buildUsage: buildOpenAIUsage
        });
      }
      return;
    }
    let proseBuf = "";
    let thinkingBuf = "";
    let emittedThinkingLen = 0;
    let legacyEmitted = false;
    const streamState = createAgentToolStreamState();
    const emitUnmappedTool = (ev) => {
      writeSSE(res, cursorToolEventChunk(completionId, model, ev));
    };
    try {
      await drainAgentToolStream(
        run,
        cfg.allowCursorInternalTools,
        { allowedNames: turnOpts.allowedNames },
        {
          onText: (t) => {
            if (streamState.toolDetected && streamState.pendingBridged.length > 0 || clientAborted) {
              return;
            }
            proseBuf += nextTextChunk(proseBuf, t);
          },
          onThinking: (t) => {
            if (clientAborted) {
              return;
            }
            thinkingBuf += t;
            writeSSE(res, chunkDelta(completionId, model, { reasoning_content: t }));
            emittedThinkingLen += t.length;
          },
          onUnmappedToolEvent: cfg.allowCursorInternalTools ? emitUnmappedTool : void 0
        },
        streamState,
        {
          shouldStop: () => clientAborted || legacyEmitted,
          onBridgedCollected: () => {
            legacyEmitted = true;
          }
        }
      );
      const { pendingBridged } = streamState;
      const finishStream = (finishReason = "stop", proxyCorrection) => {
        finishStreamWithUsage(
          res,
          completionId,
          model,
          streamUsageInput(messages, sdkUsage, proseBuf, thinkingBuf),
          finishReason,
          proxyCorrection
        );
      };
      if (clientAborted) {
        finishStream();
        return;
      }
      emitBufferedReasoning(res, completionId, model, thinkingBuf.slice(emittedThinkingLen));
      const finalized = finalizeTurnToolResults(streamState, proseBuf, turnOpts);
      observeProxyTurn({
        stream: true,
        bridgedTools: finalized.bridged.map((b) => b.name),
        blockedTools: finalized.blockedTools,
        proxyCorrection: finalized.proxyCorrection !== void 0
      });
      proseBuf = finalized.content;
      emitBufferedContent(res, completionId, model, proseBuf);
      if (finalized.bridged.length > 0 || pendingBridged.length > 0) {
        if (finalized.bridged.length > 0) {
          if (turnOpts.nativeTools) {
            writeSSEToolCalls(res, completionId, model, finalized.bridged);
          } else {
            const block = formatBridgedToolCallsBlock(finalized.bridged);
            writeSSE(res, chunkDelta(completionId, model, { content: block }));
            proseBuf += block;
          }
        }
        finishStream(
          finishReasonForTools(finalized.bridged.length, turnOpts.nativeTools),
          finalized.proxyCorrection
        );
        return;
      }
      if (!legacyEmitted) {
        finishStream("stop", finalized.proxyCorrection);
      }
    } catch (err) {
      if (clientAborted) {
        finishStreamWithUsage(res, completionId, model, {
          messages,
          sdkUsage,
          textBuf: proseBuf,
          thinkingBuf,
          buildUsage: buildOpenAIUsage
        });
        return;
      }
      const msg = sanitizeReflectedText(err instanceof Error ? err.message : String(err));
      proseBuf += `
[error] ${msg}`;
      emitBufferedReasoning(res, completionId, model, thinkingBuf.slice(emittedThinkingLen));
      emitBufferedContent(res, completionId, model, proseBuf);
      finishStreamWithUsage(res, completionId, model, {
        messages,
        sdkUsage,
        textBuf: proseBuf,
        thinkingBuf,
        buildUsage: buildOpenAIUsage
      });
    }
  } finally {
    unwireAbort();
    await finalizeAgentRun(run);
    await disposeAgent(agent);
  }
}
function sendStreamStartError(res, completionId, model, err) {
  const msg = sanitizeReflectedText(err instanceof Error ? err.message : String(err));
  if (!res.headersSent) {
    sendJsonResponse(res, 500, { error: { message: msg, type: "proxy_error" } });
    return;
  }
  writeSSE(res, chunkDelta(completionId, model, { content: `
[error] ${msg}` }));
  writeSSE(res, chunkDelta(completionId, model, {}, "stop"));
  finishSSE(res);
}

// src/chat/nonstream.ts
async function handleNonStream(cfg, messages, completionId, model, modelSelection, clientAbort, res, turnOpts) {
  let sdkUsage;
  let run;
  let agent;
  let clientAborted = false;
  const sendOpts = {
    model: modelSelection,
    onDelta: async ({ update }) => {
      if (update.type !== "turn-ended" || !update.usage) {
        return;
      }
      sdkUsage = {
        inputTokens: update.usage.inputTokens ?? 0,
        outputTokens: update.usage.outputTokens ?? 0,
        cacheReadTokens: update.usage.cacheReadTokens
      };
    }
  };
  const unwireAbort = wireClientAbort(clientAbort, res, () => run, () => {
    clientAborted = true;
  });
  try {
    const sent = await sendStateless(cfg, modelSelection, messages, sendOpts, turnOpts.tools);
    agent = sent.agent;
    run = sent.run;
    let content = "";
    let reasoning = "";
    const streamState = createAgentToolStreamState();
    const nativeToolEvents = [];
    await drainAgentToolStream(
      run,
      cfg.allowCursorInternalTools,
      { allowedNames: turnOpts.allowedNames },
      {
        onText: (t) => {
          content += nextTextChunk(content, t);
        },
        onThinking: (t) => {
          reasoning += t;
        },
        onUnmappedToolEvent: cfg.allowCursorInternalTools ? (ev) => {
          nativeToolEvents.push(ev);
        } : void 0
      },
      streamState,
      { shouldStop: () => clientAborted }
    );
    const { toolDetected } = streamState;
    if (clientAborted) {
      return;
    }
    const finalized = finalizeTurnToolResults(streamState, content, turnOpts);
    observeProxyTurn({
      stream: false,
      bridgedTools: finalized.bridged.map((b) => b.name),
      blockedTools: finalized.blockedTools,
      proxyCorrection: finalized.proxyCorrection !== void 0
    });
    content = finalized.content;
    const toolCalls = openAIToolCallsFromInvocations(finalized.bridged);
    const proxyCorrection = finalized.proxyCorrection;
    if (finalized.bridged.length > 0 && !turnOpts.nativeTools) {
      content = (toolDetected ? "" : content) + formatBridgedToolCallsBlock(finalized.bridged);
    }
    if (res.writableEnded || res.destroyed) {
      return;
    }
    const finishReason = finishReasonForTools(finalized.bridged.length, turnOpts.nativeTools);
    let messageContent = content;
    if (turnOpts.nativeTools && toolCalls.length > 0) {
      messageContent = toolDetected ? content.trim() || null : content || null;
    }
    const message = {
      role: "assistant",
      content: messageContent,
      ...reasoning ? { reasoning_content: reasoning } : {}
    };
    if (turnOpts.nativeTools && toolCalls.length > 0) {
      message.tool_calls = toolCalls;
    }
    const body = {
      id: completionId,
      object: "chat.completion",
      created: Math.floor(Date.now() / 1e3),
      model,
      choices: [
        {
          index: 0,
          message,
          finish_reason: finishReason
        }
      ],
      usage: buildOpenAIUsage(messages, sdkUsage, content, reasoning)
    };
    if (proxyCorrection) {
      body.solomon_proxy_correction = proxyCorrection;
    }
    if (nativeToolEvents.length > 0) {
      body.solomon_cursor_tool_events = nativeToolEvents;
    }
    sendJsonResponse(res, 200, body);
  } finally {
    unwireAbort();
    await finalizeAgentRun(run);
    await disposeAgent(agent);
  }
}

// src/chat/index.ts
var cachedModels;
async function cursorModels(apiKey2) {
  const now = Date.now();
  if (cachedModels?.apiKey === apiKey2 && cachedModels.expiresAt > now) {
    return cachedModels.models;
  }
  const models = await Cursor.models.list({ apiKey: apiKey2 });
  cachedModels = { apiKey: apiKey2, models, expiresAt: now + 6e4 };
  return cachedModels.models;
}
async function cursorModelIDs(apiKey2) {
  const models = await cursorModels(apiKey2);
  const ids = [];
  for (const m of models) {
    if (m.id) {
      ids.push(m.id);
    }
  }
  return ids;
}
async function listModels(apiKey2) {
  return filterFlagshipModelIDs(await cursorModelIDs(apiKey2));
}
async function listAllModels(apiKey2) {
  return orderModelIDs(await cursorModelIDs(apiKey2));
}
async function handleChatCompletions(body, clientAbort, res, cfg) {
  const req = body;
  const model = sanitizeModelId(req.model);
  const messages = req.messages ?? [];
  const stream = req.stream === true;
  const modelSelection = resolveModelSelection(
    await cursorModels(cfg.apiKey),
    model,
    req.reasoning_effort,
    req.solomon_fast_mode ?? true
  );
  const completionId = newCompletionId();
  const turnOpts = turnOptsFromRequest(req);
  if (!stream) {
    await handleNonStream(cfg, messages, completionId, model, modelSelection, clientAbort, res, turnOpts);
    return;
  }
  await streamCompletion(cfg, messages, completionId, model, modelSelection, clientAbort, res, turnOpts);
}

// src/server.ts
var MAX_BODY_BYTES = 8 * 1024 * 1024;
var MAX_MESSAGES = 256;
var MAX_CONTENT_CHARS = 512e3;
var MAX_MODEL_CHARS = 256;
var MAX_TOOL_COUNT = 64;
var MAX_TOOL_NAME_CHARS = 128;
var MAX_IMAGE_URL_CHARS = 8192;
var ALLOWED_ROLES = /* @__PURE__ */ new Set(["user", "assistant", "system", "tool"]);
function createServer(cfg) {
  const health = createHealthResponder(cfg, process.cwd());
  return http.createServer((req, res) => {
    void route(req, res, cfg, health).catch((err) => {
      sendError(res, 500, err instanceof Error ? err.message : String(err));
    });
  });
}
async function route(req, res, cfg, health) {
  const url = new URL(req.url ?? "/", "http://127.0.0.1");
  const path4 = url.pathname.replace(/\/+$/, "") || "/";
  if (req.method === "GET" && (path4 === "/health" || path4 === "/v1/health")) {
    sendJsonResponse(res, 200, health(url.searchParams.get("nonce") ?? ""));
    return;
  }
  if (req.method === "GET" && (path4 === "/v1/models" || path4 === "/models")) {
    const all = url.searchParams.get("all") === "1" || url.searchParams.get("full") === "1";
    const ids = all ? await listAllModels(cfg.apiKey) : await listModels(cfg.apiKey);
    sendJsonResponse(res, 200, {
      object: "list",
      data: ids.map((id) => ({
        id,
        object: "model",
        created: 0,
        owned_by: "cursor"
      }))
    });
    return;
  }
  if (req.method === "POST" && (path4 === "/v1/chat/completions" || path4 === "/chat/completions")) {
    let body;
    try {
      body = await readBody(req);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      if (msg === "body too large") {
        sendError(res, 413, "request body too large");
        return;
      }
      throw err;
    }
    let parsed;
    try {
      parsed = parseChatCompletionRequest(body);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      if (msg === "invalid JSON body") {
        sendError(res, 400, "invalid JSON body");
        return;
      }
      sendError(res, 400, "invalid request body");
      return;
    }
    try {
      await handleChatCompletions(parsed, clientAbortFromRequest(req), res, cfg);
    } catch (err) {
      if (res.headersSent) {
        throw err;
      }
      sendError(res, 500, err instanceof Error ? err.message : String(err));
      return;
    }
    return;
  }
  sendError(res, 404, "not found");
}
function readBody(req) {
  return new Promise((resolve2, reject) => {
    const chunks = [];
    let size = 0;
    req.on("data", (c) => {
      size += c.length;
      if (size > MAX_BODY_BYTES) {
        req.destroy();
        reject(new Error("body too large"));
        return;
      }
      chunks.push(c);
    });
    req.on("end", () => resolve2(Buffer.concat(chunks).toString("utf8")));
    req.on("error", reject);
  });
}
function boundedString(v, max) {
  if (typeof v !== "string") {
    return null;
  }
  const cleaned = stripUnsafeControlChars(v);
  if (cleaned.length > max) {
    return cleaned.slice(0, max);
  }
  return cleaned;
}
function optionalBoundedString(v, max) {
  if (v === void 0 || v === null) {
    return void 0;
  }
  return boundedString(v, max) ?? void 0;
}
function isAllowedImageUrl(url) {
  const t = url.trim();
  const lower = t.toLowerCase();
  if (lower.startsWith("javascript:") || lower.startsWith("vbscript:")) {
    return false;
  }
  if (lower.startsWith("data:")) {
    return /^data:image\/[a-z0-9.+-]+;base64,/i.test(t);
  }
  return lower.startsWith("file://") || lower.startsWith("https://");
}
function sanitizeContentParts(v) {
  if (!Array.isArray(v)) {
    return null;
  }
  const out = [];
  for (const part of v.slice(0, 64)) {
    if (!part || typeof part !== "object") {
      continue;
    }
    const p = part;
    if (p.type === "text") {
      const text = boundedString(p.text, MAX_CONTENT_CHARS);
      if (text !== null) {
        out.push({ type: "text", text });
      }
      continue;
    }
    if (p.type === "image_url") {
      const image = p.image_url;
      if (!image || typeof image !== "object") {
        continue;
      }
      const iu = image;
      const rawUrl = boundedString(iu.url, MAX_IMAGE_URL_CHARS);
      if (!rawUrl || !isAllowedImageUrl(rawUrl)) {
        continue;
      }
      const detail = optionalBoundedString(iu.detail, 32);
      out.push({
        type: "image_url",
        image_url: detail ? { url: rawUrl.trim(), detail } : { url: rawUrl.trim() }
      });
    }
  }
  return out.length > 0 ? out : null;
}
function sanitizeMessageContent(v) {
  if (v === void 0 || v === null) {
    return void 0;
  }
  if (typeof v === "string") {
    return boundedString(v, MAX_CONTENT_CHARS) ?? "";
  }
  return sanitizeContentParts(v) ?? void 0;
}
function sanitizeToolCalls(v) {
  if (!Array.isArray(v)) {
    return void 0;
  }
  const out = [];
  for (const tc of v.slice(0, 32)) {
    if (!tc || typeof tc !== "object") {
      continue;
    }
    const t = tc;
    const fn = t.function;
    if (!fn || typeof fn !== "object") {
      continue;
    }
    const f = fn;
    const name = boundedString(f.name, MAX_TOOL_NAME_CHARS)?.trim();
    if (!name || !isSafeToolName(name)) {
      continue;
    }
    const args = boundedString(f.arguments, MAX_CONTENT_CHARS) ?? "{}";
    if (!readToolIntent(args)) {
      throw new Error("tool call requires a non-empty intent string");
    }
    const id = optionalBoundedString(t.id, 128);
    out.push({
      ...id ? { id } : {},
      type: "function",
      function: { name, arguments: args }
    });
  }
  return out.length > 0 ? out : void 0;
}
function sanitizeJSONObject(v) {
  if (!v || typeof v !== "object" || Array.isArray(v)) {
    return void 0;
  }
  try {
    const s = JSON.stringify(v);
    if (s.length > MAX_CONTENT_CHARS) {
      return void 0;
    }
    return JSON.parse(s);
  } catch {
    return void 0;
  }
}
function sanitizeMessage(v) {
  if (!v || typeof v !== "object" || Array.isArray(v)) {
    return null;
  }
  const m = v;
  const role = boundedString(m.role, 32)?.trim();
  if (!role || !ALLOWED_ROLES.has(role)) {
    return null;
  }
  const content = sanitizeMessageContent(m.content);
  const name = optionalBoundedString(m.name, 128);
  const toolCallId = optionalBoundedString(m.tool_call_id, 128);
  const toolCalls = sanitizeToolCalls(m.tool_calls);
  if (role === "tool" && !toolCallId) {
    return null;
  }
  const out = { role, ...content !== void 0 ? { content } : {} };
  if (name) {
    out.name = name;
  }
  if (toolCallId) {
    out.tool_call_id = toolCallId;
  }
  if (toolCalls) {
    out.tool_calls = toolCalls;
  }
  return out;
}
function sanitizeTools(v) {
  if (!Array.isArray(v)) {
    return void 0;
  }
  const out = [];
  for (const tool of v.slice(0, MAX_TOOL_COUNT)) {
    if (!tool || typeof tool !== "object") {
      continue;
    }
    const t = tool;
    const fn = t.function;
    if (!fn || typeof fn !== "object") {
      continue;
    }
    const f = fn;
    const name = boundedString(f.name, MAX_TOOL_NAME_CHARS)?.trim();
    if (!name || !isSafeToolName(name)) {
      continue;
    }
    const description = optionalBoundedString(f.description, MAX_CONTENT_CHARS);
    const parameters = schemaWithRequiredToolIntent(sanitizeJSONObject(f.parameters));
    const strict = t.strict === true || f.strict === true ? true : void 0;
    out.push({
      type: "function",
      function: {
        name,
        ...description ? { description } : {},
        parameters,
        ...strict ? { strict } : {}
      }
    });
  }
  return out.length > 0 ? out : void 0;
}
function sanitizeToolChoice(v) {
  if (v === void 0 || v === null) {
    return void 0;
  }
  if (v === "none" || v === "auto" || v === "required") {
    return v;
  }
  if (!v || typeof v !== "object" || Array.isArray(v)) {
    return void 0;
  }
  const o = v;
  if (o.type !== "function" || !o.function || typeof o.function !== "object") {
    return void 0;
  }
  const fn = o.function;
  const name = boundedString(fn.name, MAX_TOOL_NAME_CHARS)?.trim();
  if (!name || !isSafeToolName(name)) {
    return void 0;
  }
  return { type: "function", function: { name } };
}
function parseChatCompletionRequest(body) {
  let raw;
  try {
    raw = JSON.parse(body);
  } catch {
    throw new Error("invalid JSON body");
  }
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) {
    throw new Error("invalid request body");
  }
  const o = raw;
  if (!Array.isArray(o.messages)) {
    throw new Error("invalid request body");
  }
  if (o.messages.length === 0 || o.messages.length > MAX_MESSAGES) {
    throw new Error("invalid request body");
  }
  const messages = [];
  for (const m of o.messages) {
    const sm = sanitizeMessage(m);
    if (!sm) {
      throw new Error("invalid request body");
    }
    messages.push(sm);
  }
  const modelRaw = optionalBoundedString(o.model, MAX_MODEL_CHARS);
  const stream = o.stream === void 0 ? void 0 : o.stream === true;
  const reasoningEffort = optionalBoundedString(o.reasoning_effort, 32);
  const solomonFastMode = o.solomon_fast_mode === void 0 ? void 0 : o.solomon_fast_mode !== false;
  const tools = sanitizeTools(o.tools);
  const toolChoice = sanitizeToolChoice(o.tool_choice);
  const parallelToolCalls = o.parallel_tool_calls === void 0 ? void 0 : o.parallel_tool_calls !== false;
  const req = { messages };
  if (modelRaw) {
    req.model = sanitizeModelId(modelRaw);
  }
  if (stream !== void 0) {
    req.stream = stream;
  }
  if (reasoningEffort) {
    req.reasoning_effort = reasoningEffort;
  }
  if (solomonFastMode !== void 0) {
    req.solomon_fast_mode = solomonFastMode;
  }
  if (tools) {
    req.tools = tools;
  }
  if (toolChoice !== void 0) {
    req.tool_choice = toolChoice;
  }
  if (parallelToolCalls !== void 0) {
    req.parallel_tool_calls = parallelToolCalls;
  }
  return req;
}
function sendError(res, code, message) {
  sendJsonResponse(res, code, {
    error: { message: sanitizeReflectedText(message), type: "proxy_error" }
  });
}

// src/index.ts
var apiKey = process.env.CURSOR_API_KEY?.trim();
if (!apiKey) {
  console.error("CURSOR_API_KEY is required");
  process.exit(1);
}
var port = parseInt(process.env.CURSOR_API_PORT ?? "8766", 10);
var cwd = process.env.CURSOR_API_CWD?.trim() || process.cwd();
var allowCursorInternalTools = process.env.CURSOR_API_ALLOW_INTERNAL_TOOLS === "true";
process.on("uncaughtException", (err) => {
  console.error(err);
});
process.on("unhandledRejection", (err) => {
  console.error(err);
});
var server = createServer({ apiKey, cwd, allowCursorInternalTools });
server.listen(port, "127.0.0.1");
