export const DEFAULT_SUBAGENT_SYS_PATH = ".solomon/cursor-task-sys.txt";

export function normalizeSolomonToolArgs(
  solomonName: string,
  viaCursorName: string,
  raw: unknown,
): Record<string, unknown> | null {
  if (solomonName === "subagent") {
    return normalizeSubagentArgsFromRaw(raw);
  }
  if (solomonName === "editFile" && isDeleteCursorName(viaCursorName)) {
    return normalizeDeleteEditFileArgs(raw);
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
    if (!path) {
      return null;
    }
    const renameTo =
      pickString(obj, ["renameTo", "rename_to", "new_path", "newPath", "destination"]) ?? "";
    if (renameTo) {
      const oldString =
        pickString(obj, ["oldString", "old_string", "oldText"]) ?? "";
      const newString =
        pickString(obj, ["newString", "new_string", "newText", "content"]) ?? "";
      if (pickBoolean(obj, ["delete"]) || oldString !== "" || newString !== "") {
        return null;
      }
      return {
        path,
        renameTo,
        intent:
          pickString(obj, ["intent", "description", "explanation"]) ?? "rename file",
      };
    }
    if (pickBoolean(obj, ["delete"])) {
      const oldString =
        pickString(obj, ["oldString", "old_string", "oldText"]) ?? "";
      const newString =
        pickString(obj, ["newString", "new_string", "newText", "content"]) ?? "";
      if (oldString !== "" || newString !== "") {
        return null;
      }
      return {
        path,
        delete: true,
        intent:
          pickString(obj, ["intent", "description", "explanation"]) ?? "delete file",
      };
    }
    const oldString =
      pickString(obj, ["oldString", "old_string", "oldText"]) ?? "";
    const newString =
      pickString(obj, ["newString", "new_string", "newText", "content"]) ?? "";
    if (oldString === "" && newString === "") {
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

function normalizeFetchWebArgs(raw: unknown): Record<string, unknown> | null {
  const obj = parseArgsObject(raw);
  if (!obj) {
    return null;
  }
  const url = pickString(obj, ["url", "uri", "target_url", "targetUrl", "href"]) ?? "";
  if (!url) {
    return null;
  }
  const out: Record<string, unknown> = { url };
  const timeout = pickNumber(obj, ["timeoutSeconds", "timeout_seconds", "timeout"]);
  if (timeout !== undefined) {
    out.timeoutSeconds = timeout;
  }
  return out;
}

function normalizeWebSearchArgs(raw: unknown): Record<string, unknown> | null {
  const obj = parseArgsObject(raw);
  if (!obj) {
    return null;
  }
  const query = pickString(obj, ["query", "search_term", "searchQuery", "q"]) ?? "";
  if (!query) {
    return null;
  }
  const out: Record<string, unknown> = { query };
  const engine = pickString(obj, ["engine"]);
  if (engine) {
    out.engine = engine;
  }
  const maxResults = pickNumber(obj, ["maxResults", "max_results", "num_results"]);
  if (maxResults !== undefined) {
    out.maxResults = maxResults;
  }
  const timeout = pickNumber(obj, ["timeoutSeconds", "timeout_seconds", "timeout"]);
  if (timeout !== undefined) {
    out.timeoutSeconds = timeout;
  }
  if (obj.extras && typeof obj.extras === "object") {
    out.extras = obj.extras;
  }
  return out;
}

function isDeleteCursorName(name: string): boolean {
  return name.trim().toLowerCase() === "delete";
}

function normalizeDeleteEditFileArgs(raw: unknown): Record<string, unknown> | null {
  const obj = parseArgsObject(raw);
  if (!obj) {
    return null;
  }
  const path = pickString(obj, ["path", "file_path", "filePath", "target_file"]) ?? "";
  if (!path) {
    return null;
  }
  const oldString = pickString(obj, ["oldString", "old_string", "oldText"]) ?? "";
  const newString = pickString(obj, ["newString", "new_string", "newText", "content"]) ?? "";
  if (oldString !== "" || newString !== "") {
    return null;
  }
  return {
    path,
    delete: true,
    intent: pickString(obj, ["intent", "description", "explanation"]) ?? "delete file",
  };
}

function isSemanticSearchCursorName(name: string): boolean {
  const n = name.trim().toLowerCase();
  return n === "semanticsearch" || n === "semantic_search";
}

function normalizeSemanticSearchArgs(raw: unknown): Record<string, unknown> | null {
  const obj = parseArgsObject(raw);
  if (!obj) {
    return null;
  }
  const pattern =
    pickString(obj, ["query", "search_term", "searchQuery", "pattern", "regex"]) ?? "";
  if (!pattern) {
    return null;
  }
  const out: Record<string, unknown> = { pattern, files: false };
  const path = pickString(obj, ["path", "target_directory", "targetDirectory"]);
  if (path) {
    out.path = path;
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
  const pg = pickString(obj, ["pathGlob", "glob", "glob_pattern", "globPattern"]);
  if (pg) {
    out.pathGlob = pg;
  }
  const hl = pickNumber(obj, ["headLimit", "head_limit", "num_results"]);
  if (hl !== undefined) {
    out.headLimit = hl;
  }
  return out;
}

function normalizeSubagentArgsFromRaw(raw: unknown): Record<string, unknown> | null {
  const obj = parseArgsObject(raw);
  if (!obj) {
    return null;
  }
  const task =
    pickString(obj, ["task", "prompt", "description", "message", "user_query"]) ?? "";
  if (!task) {
    return null;
  }
  const sysPromptPath =
    pickString(obj, ["sysPromptPath", "sys_prompt_path", "systemPromptPath"]) ??
    DEFAULT_SUBAGENT_SYS_PATH;
  const out: Record<string, unknown> = { sysPromptPath, task };
  const intent = pickString(obj, ["intent", "description"]);
  if (intent && intent !== task) {
    out.intent = intent;
  } else {
    out.intent = "nested task";
  }
  return out;
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

function pickBoolean(obj: Record<string, unknown>, keys: string[]): boolean {
  for (const k of keys) {
    const v = obj[k];
    if (typeof v === "boolean") {
      return v;
    }
  }
  return false;
}
