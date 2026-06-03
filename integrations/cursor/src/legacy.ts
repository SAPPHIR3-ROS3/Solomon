export type LegacyToolInvocation = {
  name: string;
  args: Record<string, unknown>;
  intent?: string;
};

export const SOLOMON_MCP_PROVIDER = "solomon";

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

const CURSOR_TO_SOLOMON: Record<string, string> = {
  read: "readFile",
  Read: "readFile",
  readFile: "readFile",
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
  editFile: "editFile",
  StrReplace: "editFile",
  strReplace: "editFile",
  search_replace: "editFile",
  find: "find",
  Find: "find",
  Grep: "find",
  grep: "find",
  Glob: "find",
  glob: "find",
  ripgrep: "find",
  rg: "find",
};

export function collectLegacyTool(
  pending: LegacyToolInvocation[],
  name: string,
  rawArgs: unknown,
): void {
  const mapped = mapCursorToolToSolomon(name, rawArgs);
  if (mapped) {
    pending.push(mapped);
    return;
  }
  const direct = directSolomonToolInvocation(name, rawArgs);
  if (direct) {
    pending.push(direct);
  }
}

export function tryCollectLegacyTool(
  pending: LegacyToolInvocation[],
  name: string,
  rawArgs: unknown,
): boolean {
  const before = pending.length;
  collectLegacyTool(pending, name, rawArgs);
  return pending.length > before;
}

const SOLOMON_TOOL_NAME_RE = /^[a-zA-Z_][a-zA-Z0-9_-]*$/;

export function mapCursorToolToSolomon(
  name: string,
  rawArgs: unknown,
): LegacyToolInvocation | null {
  const mapped = CURSOR_TO_SOLOMON[name];
  if (!mapped) {
    return null;
  }
  const solomonName = mapped;
  const args =
    solomonName === "find"
      ? normalizeFindArgsFromRaw(name, rawArgs)
      : normalizeArgs(solomonName, rawArgs);
  if (!args) {
    return null;
  }
  return invocationWithIntent(solomonName, args);
}

function directSolomonToolInvocation(
  name: string,
  rawArgs: unknown,
): LegacyToolInvocation | null {
  const trimmed = name.trim();
  if (!trimmed || !SOLOMON_TOOL_NAME_RE.test(trimmed)) {
    return null;
  }
  if (CURSOR_TO_SOLOMON[trimmed] !== undefined) {
    return null;
  }
  const obj = parseArgsObject(rawArgs);
  if (obj === null) {
    return null;
  }
  return invocationWithIntent(trimmed, obj);
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

function normalizeArgs(
  solomonName: string,
  raw: unknown,
): Record<string, unknown> | null {
  if (raw === null || raw === undefined) {
    return {};
  }
  let obj: Record<string, unknown>;
  if (typeof raw === "string") {
    try {
      obj = JSON.parse(raw) as Record<string, unknown>;
    } catch {
      return null;
    }
  } else if (typeof raw === "object") {
    obj = { ...(raw as Record<string, unknown>) };
  } else {
    return null;
  }
  if (solomonName === "readFile") {
    const path =
      pickString(obj, [
        "path",
        "file_path",
        "filePath",
        "target_file",
        "targetFile",
        "file",
        "filename",
        "relative_path",
        "relativePath",
      ]) ?? "";
    if (!path) {
      return null;
    }
    const out: Record<string, unknown> = { path };
    const start = pickNumber(obj, ["startLine", "start_line", "offset", "line", "start"]);
    const end = pickNumber(obj, ["endLine", "end_line", "end"]);
    const limit = pickNumber(obj, ["limit", "line_count", "lineCount", "num_lines"]);
    if (start !== undefined) {
      out.startLine = start;
    }
    if (end !== undefined) {
      out.endLine = end;
    } else if (start !== undefined && limit !== undefined && limit > 0) {
      out.endLine = start + limit - 1;
    }
    return out;
  }
  if (solomonName === "shell") {
    const command =
      pickString(obj, ["command", "cmd", "script", "shell_command"]) ?? "";
    if (!command) {
      return null;
    }
    const out: Record<string, unknown> = {
      command,
      intent: pickString(obj, ["intent", "description", "explanation"]) ?? "run command",
    };
    if (typeof obj.timeoutSeconds === "number") {
      out.timeoutSeconds = obj.timeoutSeconds;
    }
    return out;
  }
  if (solomonName === "editFile") {
    const path = pickString(obj, ["path", "file_path", "filePath", "target_file"]) ?? "";
    const oldString =
      pickString(obj, ["oldString", "old_string", "oldText"]) ?? "";
    const newString =
      pickString(obj, ["newString", "new_string", "newText", "content"]) ?? "";
    if (!path || (oldString === "" && newString === "")) {
      return null;
    }
    return {
      path,
      oldString,
      newString,
      intent:
        pickString(obj, ["intent", "description", "explanation"]) ?? "edit file",
    };
  }
  return obj;
}

function normalizeFindArgsFromRaw(
  cursorName: string,
  raw: unknown,
): Record<string, unknown> | null {
  const obj = parseArgsObject(raw);
  if (!obj) {
    return null;
  }
  return normalizeFindArgs(cursorName, obj);
}

function parseArgsObject(raw: unknown): Record<string, unknown> | null {
  if (raw === null || raw === undefined) {
    return {};
  }
  if (typeof raw === "string") {
    try {
      return JSON.parse(raw) as Record<string, unknown>;
    } catch {
      return null;
    }
  }
  if (typeof raw === "object") {
    return { ...(raw as Record<string, unknown>) };
  }
  return null;
}

function normalizeFindArgs(cursorName: string, obj: Record<string, unknown>): Record<string, unknown> | null {
  const n = cursorName.toLowerCase();
  let files = n === "glob";
  if (typeof obj.files === "boolean") {
    files = obj.files;
  }
  const pattern = files
    ? pickString(obj, ["pattern", "glob_pattern", "globPattern"]) ?? ""
    : pickString(obj, ["pattern", "query", "regex"]) ?? "";
  if (!pattern) {
    return null;
  }
  const out: Record<string, unknown> = { pattern, files };
  const path = pickString(obj, ["path", "target_directory", "targetDirectory"]);
  if (path) {
    out.path = path;
  }
  if (!files) {
    const pg = pickString(obj, ["pathGlob", "glob", "glob_pattern", "globPattern"]);
    if (pg) {
      out.pathGlob = pg;
    }
    const om = pickString(obj, ["outputMode", "output_mode"]);
    if (om) {
      out.outputMode = om;
    }
    if (obj.caseInsensitive === true || obj["-i"] === true) {
      out.caseInsensitive = true;
    }
    const hl = pickNumber(obj, ["headLimit", "head_limit"]);
    if (hl !== undefined) {
      out.headLimit = hl;
    }
  } else {
    const hl = pickNumber(obj, ["headLimit", "head_limit"]);
    if (hl !== undefined) {
      out.headLimit = hl;
    }
  }
  return out;
}

function pickString(obj: Record<string, unknown>, keys: string[]): string | undefined {
  for (const k of keys) {
    const v = obj[k];
    if (typeof v === "string" && v.trim() !== "") {
      return v;
    }
  }
  return undefined;
}

function pickNumber(obj: Record<string, unknown>, keys: string[]): number | undefined {
  for (const k of keys) {
    const v = obj[k];
    if (typeof v === "number" && Number.isFinite(v)) {
      return Math.trunc(v);
    }
  }
  return undefined;
}
