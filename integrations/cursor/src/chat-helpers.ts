import type { SDKMessage } from "@cursor/sdk";
import {
  tryCollectLegacyTool,
  unwrapSolomonMcpCall,
  type LegacyToolInvocation,
} from "./legacy.js";
import {
  roughTokFromMessages,
  roughTokFromString,
} from "./messages.js";
import {
  filterInvocations,
  isValidInvocation,
  limitInvocations,
  parseToolInvocationsFromText,
} from "./openai-tools.js";
import type { ChatMessage } from "./openai-types.js";
import type {
  CursorTurnUsage,
  OpenAIFinishReason,
} from "./run-control.js";
import type { OpenAIUsagePayload } from "./openai-sse.js";

export type TurnToolOpts = {
  allowedNames: Set<string> | null;
  parallelToolCalls?: boolean;
};

export function finishReasonForTools(
  bridgedCount: number,
  nativeTools: boolean,
): OpenAIFinishReason {
  if (bridgedCount > 0 && nativeTools) {
    return "tool_calls";
  }
  return "stop";
}

export function nativeInvocationsFromText(text: string, turnOpts: TurnToolOpts): {
  content: string;
  invocations: LegacyToolInvocation[];
} {
  const parsed = parseToolInvocationsFromText(text);
  const invalidCount = parsed.invocations.filter((inv) => !isValidInvocation(inv)).length;
  const content = invalidCount > 0
    ? `${parsed.content}\n[error] Invalid empty editFile tool call blocked by Solomon proxy`.trim()
    : parsed.content;
  return {
    content,
    invocations: limitInvocations(
      filterInvocations(parsed.invocations, turnOpts.allowedNames),
      turnOpts.parallelToolCalls,
    ),
  };
}

export function nextTextChunk(current: string, incoming: string): string {
  if (!incoming) {
    return "";
  }
  incoming = collapseExactRepeat(incoming);
  if (incoming.startsWith(current)) {
    return incoming.slice(current.length);
  }
  if (current.endsWith(incoming)) {
    return "";
  }
  return incoming;
}

export function collapseExactRepeat(text: string): string {
  const n = text.length;
  if (n < 2 || n%2 !== 0) {
    return text;
  }
  const half = n / 2;
  const left = text.slice(0, half);
  return left === text.slice(half) ? left : text;
}

export function buildOpenAIUsage(
  messages: ChatMessage[],
  sdkUsage: CursorTurnUsage | undefined,
  textBuf: string,
  thinkingBuf: string,
): OpenAIUsagePayload {
  const estPrompt = roughTokFromMessages(messages);
  const estReason = roughTokFromString(thinkingBuf);
  const estResp = roughTokFromString(textBuf);
  let prompt = sdkUsage?.inputTokens ?? 0;
  let completion = sdkUsage?.outputTokens ?? 0;
  const cached = sdkUsage?.cacheReadTokens ?? 0;
  if (prompt <= 0) {
    prompt = estPrompt;
  }
  if (completion <= 0) {
    completion = estReason + estResp;
  }
  let reasoning = estReason;
  if (reasoning > completion) {
    reasoning = completion;
  }
  if (thinkingBuf.length === 0) {
    reasoning = 0;
  }
  const total = prompt + completion;
  const out: OpenAIUsagePayload = {
    prompt_tokens: prompt,
    completion_tokens: completion,
    total_tokens: total > 0 ? total : prompt + completion,
  };
  if (cached > 0) {
    out.prompt_tokens_details = { cached_tokens: cached };
  }
  if (reasoning > 0) {
    out.completion_tokens_details = { reasoning_tokens: reasoning };
  }
  return out;
}

export function processStreamEvent(
  event: SDKMessage,
  allowCursorInternalTools: boolean,
  onText: (s: string) => void,
  onThinking: (s: string) => void,
  pendingLegacy: LegacyToolInvocation[],
  onToolDetected: () => void,
): void {
  if (event.type === "assistant") {
    let afterTool = false;
    for (const block of event.message.content) {
      if (block.type === "tool_use") {
        const mcp = unwrapSolomonMcpCall(block.name, block.input);
        if (mcp) {
          if (tryCollectLegacyTool(pendingLegacy, mcp.toolName, mcp.args)) {
            afterTool = true;
            onToolDetected();
          } else {
            onText(blockedCursorToolLine(`mcp:${mcp.toolName}`));
          }
          continue;
        }
        if (!allowCursorInternalTools) {
          if (tryCollectLegacyTool(pendingLegacy, block.name, block.input)) {
            afterTool = true;
            onToolDetected();
          } else {
            onText(blockedCursorToolLine(block.name));
          }
          continue;
        }
        if (tryCollectLegacyTool(pendingLegacy, block.name, block.input)) {
          afterTool = true;
          onToolDetected();
        }
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
    if (event.status === "completed") {
      return;
    }
    const mcp = unwrapSolomonMcpCall(event.name, event.args);
    if (mcp) {
      if (event.status === "error") {
        return;
      }
      if (tryCollectLegacyTool(pendingLegacy, mcp.toolName, mcp.args)) {
        onToolDetected();
      } else {
        onText(blockedCursorToolLine(`mcp:${mcp.toolName}`));
      }
      return;
    }
    if (!allowCursorInternalTools) {
      if (event.args !== undefined && tryCollectLegacyTool(pendingLegacy, event.name, event.args)) {
        onToolDetected();
      } else {
        onText(blockedCursorToolLine(event.name));
      }
      return;
    }
    if (event.status === "error") {
      return;
    }
    if (event.args !== undefined && tryCollectLegacyTool(pendingLegacy, event.name, event.args)) {
      onToolDetected();
    }
  }
}

export function blockedCursorToolLine(name: string): string {
  const safe = name.replace(/[^\w:.-]/g, "").slice(0, 128) || "unknown";
  return `\n[error] Cursor internal tool call blocked by Solomon proxy: ${safe}\n`;
}
