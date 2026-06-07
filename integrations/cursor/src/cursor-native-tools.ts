import type { SDKMessage } from "@cursor/sdk";
import { unwrapSolomonMcpCall } from "./legacy.js";

export type CursorNativeToolEvent = {
  callId?: string;
  name: string;
  status: "running" | "completed" | "error";
  args?: unknown;
  result?: unknown;
  error?: string;
  displayLine?: string;
};

export function cursorToolEventChunk(
  completionId: string,
  model: string,
  event: CursorNativeToolEvent,
): Record<string, unknown> {
  return {
    id: completionId,
    object: "chat.completion.chunk",
    created: Math.floor(Date.now() / 1000),
    model,
    choices: [{ index: 0, delta: {}, finish_reason: null }],
    solomon_cursor_tool_event: event,
  };
}

function truncate(s: string, max: number): string {
  s = s.trim();
  if (max < 1 || s.length <= max) {
    return s;
  }
  return s.slice(0, max) + "…";
}

function jsonString(v: unknown): string {
  if (typeof v === "string") {
    return v.trim();
  }
  return "";
}

function argMap(args: unknown): Record<string, unknown> {
  if (!args || typeof args !== "object" || Array.isArray(args)) {
    return {};
  }
  return args as Record<string, unknown>;
}

function pickArgPreview(m: Record<string, unknown>): string {
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

function pickResultPreview(result: unknown): string {
  if (result === undefined || result === null) {
    return "";
  }
  if (typeof result === "string") {
    return truncate(result, 200);
  }
  if (typeof result !== "object") {
    return truncate(String(result), 200);
  }
  const m = result as Record<string, unknown>;
  for (const key of ["output", "content", "text", "message", "stdout", "stderr"]) {
    const s = jsonString(m[key]);
    if (s) {
      return truncate(s, 200);
    }
  }
  if (Array.isArray(m.content)) {
    const parts: string[] = [];
    for (const block of m.content) {
      if (!block || typeof block !== "object") {
        continue;
      }
      const b = block as Record<string, unknown>;
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

export function formatUnmappedToolDisplayLine(
  name: string,
  status: CursorNativeToolEvent["status"],
  args?: unknown,
  result?: unknown,
  error?: string,
): string {
  const label = name.trim() || "tool";
  if (status === "error") {
    return (error?.trim() || "failed") + ` (${label})`;
  }
  const preview = pickArgPreview(argMap(args));
  if (status === "running") {
    return preview || "…";
  }
  if (status === "completed") {
    const body = pickResultPreview(result);
    if (body) {
      return body;
    }
    return preview ? `${preview} → done` : "done";
  }
  return preview;
}

export function unmappedToolEvent(
  name: string,
  status: CursorNativeToolEvent["status"],
  args?: unknown,
  result?: unknown,
  error?: string,
  callId?: string,
): CursorNativeToolEvent {
  const ev: CursorNativeToolEvent = {
    name: name.trim() || "tool",
    status,
    displayLine: formatUnmappedToolDisplayLine(name, status, args, result, error),
  };
  if (callId) {
    ev.callId = callId;
  }
  if (args !== undefined) {
    ev.args = args;
  }
  if (result !== undefined) {
    ev.result = result;
  }
  if (error) {
    ev.error = error;
  }
  return ev;
}

function toolCallErrorMessage(event: SDKMessage & { type: "tool_call" }): string | undefined {
  const err = (event as { error?: unknown }).error;
  if (typeof err === "string" && err.trim() !== "") {
    return err;
  }
  if (err && typeof err === "object" && typeof (err as { message?: string }).message === "string") {
    return (err as { message: string }).message;
  }
  return undefined;
}

export function unmappedToolEventFromToolCall(event: SDKMessage & { type: "tool_call" }): CursorNativeToolEvent {
  const mcp = unwrapSolomonMcpCall(event.name, event.args);
  const name = mcp ? `mcp:${mcp.toolName}` : event.name;
  const args = mcp ? mcp.args : event.args;
  const status: CursorNativeToolEvent["status"] =
    event.status === "error" ? "error" : event.status === "completed" ? "completed" : "running";
  return unmappedToolEvent(
    name,
    status,
    args,
    status === "completed" ? event.result : undefined,
    status === "error" ? toolCallErrorMessage(event) : undefined,
    event.call_id,
  );
}
