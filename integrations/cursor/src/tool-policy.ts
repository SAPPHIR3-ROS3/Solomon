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
  web_search: "webSearch",
};

export const SOLOMON_CANONICAL_TOOLS = new Set([
  "readFile",
  "shell",
  "editFile",
  "editPlan",
  "find",
  "subagent",
  "fetchWeb",
  "webSearch",
  "loadSkill",
  "searchSkill",
  "createPlan",
  "buildPlan",
]);

export const DEFAULT_PROXY_ENABLED_TOOLS =
  "readFile, editFile, find, shell, subagent, fetchWeb, webSearch";

export const BLOCKED_MCP_EXTERNAL_LABEL = "mcp:external";

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
  if (shouldHardDenyCursorTool(trimmed)) {
    return true;
  }
  if (trimmed.startsWith("mcp:")) {
    return shouldBlockDeferredSolomonTool(trimmed.slice(4));
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
      return "MCP work: call searchTools for schemas, then orchestrate with the MCP sandbox SDK.";
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
    if (shouldBlockDeferredSolomonTool(deferred)) {
      return `${deferred}: call searchTools, then orchestrate with the matching sandbox SDK — not a direct native tool_call.`;
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
      return "File reads: call searchTools if unsure, then orchestrate with sdk.ReadFile.";
    case "editFile":
      return "File edits: orchestrate with sdk.WriteFile, sdk.ReplaceInFile, or sdk.DeleteFile.";
    case "shell":
      return "Terminal work: orchestrate with sdk.Shell (sync only).";
    case "find":
      return "Search/listing: orchestrate with sdk.Glob, sdk.Grep, or find SDK helpers.";
    case "subagent":
      return "Nested agent work: emit native subagent via <tool_calls> or tool_calls.";
    case "fetchWeb":
      return "HTTP fetch: orchestrate with sdk.FetchWeb.";
    case "webSearch":
      return "Web search: orchestrate with sdk.WebSearch.";
    default:
      return "Call searchTools, then orchestrate with the sandbox SDK.";
  }
}

export function correctionHintForBlockedTool(toolName: string): string | null {
  return hardDenyCorrectionHint(toolName) ?? redirectCorrectionHint(toolName);
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
