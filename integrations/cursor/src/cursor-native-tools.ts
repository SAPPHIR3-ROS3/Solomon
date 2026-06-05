import type { SDKMessage } from "@cursor/sdk";
import { unwrapSolomonMcpCall } from "./legacy.js";

export type CursorNativeToolEvent = {
  callId?: string;
  name: string;
  status: "running" | "completed" | "error";
  args?: unknown;
  result?: unknown;
  error?: string;
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

function emitNativeTool(
  emit: (event: CursorNativeToolEvent) => void,
  event: CursorNativeToolEvent,
): void {
  if (event.callId === undefined) {
    delete event.callId;
  }
  emit(event);
}

function mcpDisplayName(eventName: string, rawArgs: unknown): string {
  const mcp = unwrapSolomonMcpCall(eventName, rawArgs);
  if (mcp) {
    return `mcp:${mcp.toolName}`;
  }
  if (eventName === "mcp") {
    return "mcp:external";
  }
  return eventName.trim();
}

export function processNativeCursorStreamEvent(
  event: SDKMessage,
  onText: (s: string) => void,
  onThinking: (s: string) => void,
  emitNativeToolEvent: (event: CursorNativeToolEvent) => void,
): void {
  if (event.type === "assistant") {
    let afterTool = false;
    for (const block of event.message.content) {
      if (block.type === "tool_use") {
        afterTool = true;
        const mcp = unwrapSolomonMcpCall(block.name, block.input);
        emitNativeTool(emitNativeToolEvent, {
          callId: block.id,
          name: mcp ? `mcp:${mcp.toolName}` : block.name,
          status: "running",
          args: mcp ? mcp.args : block.input,
        });
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
    const name = mcpDisplayName(event.name, event.args);
    const base: CursorNativeToolEvent = {
      callId: event.call_id,
      name,
      status: event.status === "error" ? "error" : event.status === "completed" ? "completed" : "running",
    };
    if (event.args !== undefined) {
      const mcp = unwrapSolomonMcpCall(event.name, event.args);
      base.args = mcp ? mcp.args : event.args;
    }
    if (event.status === "completed" && event.result !== undefined) {
      base.result = event.result;
    }
    if (event.status === "error") {
      const err = (event as { error?: unknown }).error;
      if (typeof err === "string" && err.trim() !== "") {
        base.error = err;
      } else if (err && typeof err === "object" && typeof (err as { message?: string }).message === "string") {
        base.error = (err as { message: string }).message;
      }
    }
    emitNativeTool(emitNativeToolEvent, base);
    return;
  }
  if (event.type === "task" && event.text) {
    onThinking(event.text);
  }
}
