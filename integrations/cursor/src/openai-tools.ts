import { randomBytes } from "node:crypto";
import type { ServerResponse } from "node:http";
import type { LegacyToolInvocation } from "./legacy.js";
import { chunkDelta, writeSSE } from "./openai-sse.js";
import type { ChatCompletionTool, ChatToolCall } from "./openai-types.js";

export function allowedToolNamesFromRequest(tools: ChatCompletionTool[] | undefined): Set<string> | null {
  if (!tools?.length) {
    return null;
  }
  const names = new Set<string>();
  for (const t of tools) {
    const n = t.function?.name?.trim();
    if (n) {
      names.add(n);
    }
  }
  if (names.size === 0) {
    return null;
  }
  return names;
}

export function requestUsesNativeTools(tools: ChatCompletionTool[] | undefined): boolean {
  return (tools?.length ?? 0) > 0;
}

export function filterInvocations(
  invs: LegacyToolInvocation[],
  allowed: Set<string> | null,
): LegacyToolInvocation[] {
  if (!allowed || allowed.size === 0) {
    return invs;
  }
  return invs.filter((inv) => allowed.has(inv.name));
}

export function newToolCallId(): string {
  return `call_${randomBytes(12).toString("hex")}`;
}

export function openAIToolCallsFromInvocations(invs: LegacyToolInvocation[]): ChatToolCall[] {
  const out: ChatToolCall[] = [];
  for (const inv of invs) {
    out.push({
      id: newToolCallId(),
      type: "function",
      function: {
        name: inv.name,
        arguments: JSON.stringify(inv.args ?? {}),
      },
    });
  }
  return out;
}

export function writeSSEToolCalls(
  res: ServerResponse,
  completionId: string,
  model: string,
  invs: LegacyToolInvocation[],
): void {
  if (invs.length === 0) {
    return;
  }
  const toolCalls = invs.map((inv, index) => ({
    index,
    id: newToolCallId(),
    type: "function",
    function: {
      name: inv.name,
      arguments: JSON.stringify(inv.args ?? {}),
    },
  }));
  writeSSE(res, chunkDelta(completionId, model, { tool_calls: toolCalls }));
}

export function harnessToolsClause(tools: ChatCompletionTool[] | undefined): string {
  if (!tools?.length) {
    return "";
  }
  const names: string[] = [];
  for (const t of tools) {
    const n = t.function?.name?.trim();
    if (n) {
      names.push(n);
    }
  }
  if (names.length === 0) {
    return "";
  }
  return (
    `[Harness] OpenAI function tools are enabled for this request. Use only these tool names via API tool_calls: ${names.join(", ")}. ` +
    "Do not invoke Cursor built-in tools (Read, Shell, Grep, etc.). Do not emit <tool_calls> XML when native tools are active."
  );
}
