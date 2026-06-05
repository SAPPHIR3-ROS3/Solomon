import { normalizeSolomonToolArgs } from "./legacy-normalize.js";

export type LegacyToolInvocation = {
  name: string;
  args: Record<string, unknown>;
  intent?: string;
};

export type ToolBridgeContext = {
  allowedNames: Set<string> | null;
};

export const SOLOMON_MCP_PROVIDER = "solomon";

export { DEFAULT_SUBAGENT_SYS_PATH } from "./legacy-normalize.js";

export function unwrapSolomonMcpCall(
  eventName: string,
  rawArgs: unknown,
): { toolName: string; args: unknown } | null {
  if (eventName !== "mcp") {
    return null;
  }
  if (!rawArgs || typeof rawArgs !== "object") {
    return null;
  }
  const obj = rawArgs as Record<string, unknown>;
  if (obj.providerIdentifier !== SOLOMON_MCP_PROVIDER) {
    return null;
  }
  const toolName = typeof obj.toolName === "string" ? obj.toolName.trim() : "";
  if (!toolName) {
    return null;
  }
  return { toolName, args: obj.args ?? {} };
}

export function formatLegacyToolCallsBlock(tools: LegacyToolInvocation[]): string {
  const parts: string[] = ["<tool_calls>"];
  for (const t of tools) {
    parts.push(`<tool name="${escapeXmlAttr(t.name)}">`);
    if (t.intent && String(t.intent).trim() !== "") {
      parts.push(`<intent>${escapeXmlText(String(t.intent))}</intent>`);
    }
    parts.push(`<args>${escapeXmlText(JSON.stringify(t.args ?? {}))}</args>`);
    parts.push("</tool>");
  }
  parts.push("</tool_calls>");
  return parts.join("\n");
}

function escapeXmlAttr(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/"/g, "&quot;").replace(/</g, "&lt;");
}

function escapeXmlText(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;");
}

const CURSOR_NATIVE_ALIASES: Record<string, string> = {
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
  search_replace: "editFile",
  Delete: "editFile",
  delete: "editFile",
  find: "find",
  Find: "find",
  Grep: "find",
  grep: "find",
  Glob: "find",
  glob: "find",
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

const SOLOMON_TOOL_NAME_RE = /^[a-zA-Z_][a-zA-Z0-9_-]*$/;

function isAllowedSolomonTool(name: string, ctx: ToolBridgeContext): boolean {
  if (!ctx.allowedNames) {
    return true;
  }
  return ctx.allowedNames.has(name);
}

export function bridgeToolInvocation(
  eventName: string,
  rawArgs: unknown,
  ctx: ToolBridgeContext,
): LegacyToolInvocation | null {
  const trimmed = eventName.trim();
  if (!trimmed) {
    return null;
  }
  const solomonName = CURSOR_NATIVE_ALIASES[trimmed] ?? trimmed;
  if (!SOLOMON_TOOL_NAME_RE.test(solomonName)) {
    return null;
  }
  if (!isAllowedSolomonTool(solomonName, ctx)) {
    return null;
  }
  const args = normalizeSolomonToolArgs(solomonName, trimmed, rawArgs);
  if (!args) {
    return null;
  }
  return invocationWithIntent(solomonName, args);
}

export function collectLegacyTool(
  pending: LegacyToolInvocation[],
  name: string,
  rawArgs: unknown,
  ctx: ToolBridgeContext,
): void {
  const inv = bridgeToolInvocation(name, rawArgs, ctx);
  if (inv) {
    pending.push(inv);
  }
}

export function tryCollectLegacyTool(
  pending: LegacyToolInvocation[],
  name: string,
  rawArgs: unknown,
  ctx: ToolBridgeContext,
): boolean {
  const before = pending.length;
  collectLegacyTool(pending, name, rawArgs, ctx);
  return pending.length > before;
}

function invocationWithIntent(
  solomonName: string,
  args: Record<string, unknown>,
): LegacyToolInvocation {
  const intent =
    typeof args.intent === "string"
      ? args.intent
      : typeof (args as { description?: string }).description === "string"
        ? (args as { description: string }).description
        : undefined;
  if (intent !== undefined) {
    delete args.intent;
    delete (args as { description?: string }).description;
  }
  return { name: solomonName, args, ...(intent ? { intent } : {}) };
}
