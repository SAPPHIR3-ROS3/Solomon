export type LegacyToolInvocation = {
  name: string;
  args: Record<string, unknown>;
  intent?: string;
};

export function formatLegacyToolCallsBlock(tools: LegacyToolInvocation[]): string {
  const parts: string[] = ["<tool_calls>"];
  for (const t of tools) {
    parts.push(`<tool name="${escapeXmlAttr(t.name)}">`);
    if (t.intent && String(t.intent).trim() !== "") {
      parts.push(`<intent>${escapeXmlText(String(t.intent))}</intent>`);
    }
    parts.push(`<args>${JSON.stringify(t.args ?? {})}</args>`);
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
};

export function collectLegacyTool(
  pending: LegacyToolInvocation[],
  name: string,
  rawArgs: unknown,
): void {
  const mapped = mapCursorToolToSolomon(name, rawArgs);
  if (mapped) {
    pending.push(mapped);
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

const SOLOMON_TOOL_NAMES = new Set(["readFile", "shell", "editFile"]);

export function mapCursorToolToSolomon(
  name: string,
  rawArgs: unknown,
): LegacyToolInvocation | null {
  const mapped = CURSOR_TO_SOLOMON[name];
  if (!mapped) {
    return null;
  }
  const solomonName = mapped;
  const args = normalizeArgs(solomonName, rawArgs);
  if (!args) {
    return null;
  }
  if (!SOLOMON_TOOL_NAMES.has(solomonName)) {
    return null;
  }
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
  return { name: solomonName, args, intent };
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
      intent: pickString(obj, ["intent", "description", "explanation"]) ?? "cursor tool",
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
    if (!path) {
      return null;
    }
    return {
      path,
      oldString,
      newString,
      intent:
        pickString(obj, ["intent", "description", "explanation"]) ?? "cursor edit",
    };
  }
  return obj;
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
