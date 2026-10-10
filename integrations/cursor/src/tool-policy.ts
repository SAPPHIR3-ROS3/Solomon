export const SOLOMON_TOOL_NAME_RE = /^[a-zA-Z_][a-zA-Z0-9_-]*$/;

export const CURSOR_NATIVE_ALIASES: Record<string, string> = {
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
  ListDir: "listDir",
  list_dir: "listDir",
  listDir: "listDir",
  LS: "listDir",
  ls: "listDir",
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
  web_search: "webSearch",
};

export const SOLOMON_CANONICAL_TOOLS = new Set([
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
  "listSubAgents",
]);

export const DEFAULT_PROXY_ENABLED_TOOLS =
  "readFile, editFile, find, shell, subagent, fetchWeb, webSearch";

export const BLOCKED_MCP_EXTERNAL_LABEL = "mcp:external";
export const MISSING_INTENT_BLOCKED_SUFFIX = ":missing_intent";

export function missingIntentBlockedLabel(toolName: string): string {
  return `${toolName.trim()}${MISSING_INTENT_BLOCKED_SUFFIX}`;
}

export function isMissingIntentBlockedLabel(label: string): boolean {
  return label.trim().endsWith(MISSING_INTENT_BLOCKED_SUFFIX);
}

export function blockedMcpToolLabel(toolName: string): string {
  return `mcp:${toolName}`;
}

export const DEFERRED_SOLOMON_TOOL_NAMES = new Set([
  "readFile",
  "shell",
  "editFile",
  "find",
  "listDir",
  "fetchWeb",
  "webSearch",
  "createPlan",
  "editPlan",
  "buildPlan",
]);

export const CURSOR_HARD_DENY_TOOLS = new Set([
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
  "applyPatch",
]);

export function isBrowserCursorTool(name: string): boolean {
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

export function shouldHardDenyCursorTool(name: string): boolean {
  const trimmed = name.trim();
  if (!trimmed) {
    return false;
  }
  if (CURSOR_HARD_DENY_TOOLS.has(trimmed)) {
    return true;
  }
  return isBrowserCursorTool(trimmed);
}

export const CURSOR_REDIRECT_EXTRA = new Set([
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
  "listMcpResources",
]);

export function shouldRedirectCursorTool(name: string): boolean {
  const trimmed = name.trim();
  if (!trimmed) {
    return false;
  }
  if (CURSOR_REDIRECT_EXTRA.has(trimmed)) {
    return true;
  }
  return Object.prototype.hasOwnProperty.call(CURSOR_NATIVE_ALIASES, trimmed);
}

export function isExposedNativePolicyException(
  name: string,
  allowedNames: Set<string> | null,
  surfaceNames: Set<string> | null = allowedNames,
): boolean {
  const trimmed = name.trim();
  if (!allowedNames?.has(trimmed)) {
    return false;
  }
  if (trimmed === "buildPlan") {
    return true;
  }
  return isChatToolSurface(surfaceNames) &&
    (trimmed === "fetchWeb" || trimmed === "webSearch");
}

export function shouldBlockDeferredSolomonTool(name: string): boolean {
  return DEFERRED_SOLOMON_TOOL_NAMES.has(name.trim());
}

export function shouldStopProxyOnBlockedTool(label: string): boolean {
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
    const tool = trimmed.slice(4);
    return shouldBlockDeferredSolomonTool(tool) || shouldRedirectCursorTool(tool) || shouldHardDenyCursorTool(tool);
  }
  return shouldRedirectCursorTool(trimmed) || shouldBlockDeferredSolomonTool(trimmed);
}

export function isHardDenyBlockedLabel(label: string): boolean {
  const trimmed = label.trim();
  if (trimmed === BLOCKED_MCP_EXTERNAL_LABEL) {
    return true;
  }
  return shouldHardDenyCursorTool(trimmed);
}

export function hardDenyCorrectionHint(toolName: string): string | null {
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

export function missingIntentCorrectionHint(toolName: string): string | null {
  if (!isMissingIntentBlockedLabel(toolName)) {
    return null;
  }
  return "Every Solomon tool call requires a non-empty intent string; include intent in the arguments.";
}

function redirectExtraCorrectionHint(toolName: string): string | null {
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

export function redirectCorrectionHint(toolName: string): string | null {
  const trimmed = toolName.trim();
  if (!trimmed || isHardDenyBlockedLabel(trimmed)) {
    return null;
  }
  if (trimmed.startsWith("mcp:")) {
    const deferred = trimmed.slice(4);
    if (cursorToolRedirectTarget(deferred) === "listDir") {
      return "Cursor directory wrappers are disabled. Call searchTools, then orchestrate with sdk.ListDir.";
    }
    if (shouldBlockDeferredSolomonTool(deferred)) {
      return `${deferred}: this MCP wrapper is not callable; use searchTools, then orchestrate with sdk.mcp.<tool>(intent, args) — not a direct native or MCP tool_call.`;
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
    case "listDir":
      return "Cursor directory listing is disabled. Call searchTools, then orchestrate with sdk.ListDir.";
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

export function isChatToolSurface(names: Set<string> | null): boolean {
  if (!names || ["orchestrate", "searchTools", "subagent", "listSubAgents", "searchSkill", "loadSkill"].some((n) => names.has(n))) {
    return false;
  }
  return ["docsRetrieval", "readChat", "fetchWeb", "webSearch", "deepResearch", "researchStatus", "switchMode"].some((n) => names.has(n));
}

function chatCorrectionHintForBlockedTool(toolName: string, allowedNames: Set<string> | null): string | null {
  const trimmed = toolName.trim();
  const key = trimmed.replace(/_/g, "").toLowerCase();
  const switchHint = allowedNames?.has("switchMode")
    ? "Call native switchMode before workspace implementation."
    : "Workspace implementation is unavailable in this request; continue in plain text.";
  // Agent recovery for these hard-denied tools is not available in chat.
  if (key === "generateimage") {
    return "Image generation is unavailable; describe the image in plain text.";
  }
  if (key === "await") {
    return "Background workspace tasks are unavailable in CHAT mode. " + switchHint;
  }
  if (key === "applypatch") {
    return "Unified diff ApplyPatch is unsupported. " + switchHint;
  }
  const hardDeny = hardDenyCorrectionHint(trimmed);
  if (hardDeny) {
    return hardDeny;
  }
  if (trimmed.startsWith("mcp:")) {
    return "MCP actions are not exposed in CHAT mode; do not claim an MCP result without a host tool result. " + switchHint;
  }
  const target = cursorToolRedirectTarget(trimmed);
  if (target === "fetchWeb" || target === "webSearch") {
    return allowedNames?.has(target)
      ? `Use the native ${target} tool in CHAT mode.`
      : "The requested web capability is not exposed in this request; continue in plain text.";
  }
  if (shouldRedirectCursorTool(trimmed) || shouldBlockDeferredSolomonTool(trimmed)) {
    return "This workspace or agent tool is unavailable in CHAT mode. " + switchHint;
  }
  return null;
}

export function correctionHintForBlockedTool(
  toolName: string,
  chatSurface = false,
  allowedNames: Set<string> | null = null,
): string | null {
  const missingIntent = missingIntentCorrectionHint(toolName);
  if (missingIntent) {
    return missingIntent;
  }
  if (chatSurface) {
    return chatCorrectionHintForBlockedTool(toolName, allowedNames);
  }
  // A forced/restricted agent catalog may lack its normal execution entry point.
  if (allowedNames && !allowedNames.has("orchestrate")) {
    const key = toolName.trim().replace(/_/g, "").toLowerCase();
    if (key === "generateimage") return "Image generation is unavailable; describe the image in plain text.";
    if (key === "await" || key === "applypatch") return "This workspace operation is unavailable in this request; continue in plain text.";
    if (shouldRedirectCursorTool(toolName) || shouldBlockDeferredSolomonTool(toolName) || toolName.startsWith("mcp:")) {
      return hardDenyCorrectionHint(toolName) ?? "This tool is unavailable in this request; use only exposed native tools or continue in plain text.";
    }
  }
  if (allowedNames && cursorToolRedirectTarget(toolName.trim()) === "subagent" && !allowedNames.has("subagent")) {
    return "Nested agent execution is not exposed in this request; continue in plain text.";
  }
  if (allowedNames?.has("orchestrate") && !allowedNames.has("subagent") && toolName.trim().replace(/_/g, "").toLowerCase() === "await") {
    return "Use synchronous native orchestrate instead of Await.";
  }
  const hint = hardDenyCorrectionHint(toolName) ?? redirectCorrectionHint(toolName);
  if (hint && allowedNames && !allowedNames.has("searchTools")) {
    return hint.replace("Call searchTools, then orchestrate", "Call native orchestrate")
      .replace("use searchTools, then orchestrate", "use native orchestrate")
      .replace("use searchTools for schemas, then orchestrate", "use native orchestrate");
  }
  return hint;
}

export function cursorToolRedirectTarget(cursorName: string): string | undefined {
  return CURSOR_NATIVE_ALIASES[cursorName];
}

export function isSolomonCanonicalTool(name: string): boolean {
  return SOLOMON_CANONICAL_TOOLS.has(name);
}

export function isValidSolomonToolName(name: string): boolean {
  return SOLOMON_TOOL_NAME_RE.test(name);
}

export function resolveBridgedSolomonName(
  trimmed: string,
  allowedNames: Set<string> | null,
): string | null {
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

export function proxyEnabledToolsLabel(allowedNames: Set<string> | null): string {
  if (allowedNames && allowedNames.size > 0) {
    return [...allowedNames].sort().join(", ");
  }
  return DEFAULT_PROXY_ENABLED_TOOLS;
}

export function proxyShellFallbackAllowed(allowedNames: Set<string> | null): boolean {
  return !allowedNames || allowedNames.size === 0 || allowedNames.has("shell");
}
