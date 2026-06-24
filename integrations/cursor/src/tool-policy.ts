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
  ApplyPatch: "editFile",
  apply_patch: "editFile",
  applyPatch: "editFile",
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
