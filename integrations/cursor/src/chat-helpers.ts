import type { SDKMessage } from "@cursor/sdk";
import {
  type CursorNativeToolEvent,
  unmappedToolEvent,
  unmappedToolEventFromToolCall,
} from "./cursor-native-tools.js";
import {
  bridgeToolInvocation,
  tryCollectLegacyTool,
  unwrapSolomonMcpCall,
  type LegacyToolInvocation,
  type ToolBridgeContext,
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
  blockedTools: string[];
} {
  const parsed = parseToolInvocationsFromText(text);
  const blockedTools: string[] = [];
  const valid = parsed.invocations.filter((inv) => {
    if (isValidInvocation(inv)) {
      return true;
    }
    blockedTools.push(`${inv.name}:invalid`);
    return false;
  });
  return {
    content: parsed.content,
    invocations: limitInvocations(
      filterInvocations(valid, turnOpts.allowedNames),
      turnOpts.parallelToolCalls,
    ),
    blockedTools,
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

export function proxyToolCorrectionMessage(
  blocked: string[],
  allowedNames: Set<string> | null,
): string {
  const unique = [...new Set(blocked.map((n) => n.trim()).filter(Boolean))];
  if (unique.length === 0) {
    return "";
  }
  const enabled =
    allowedNames && allowedNames.size > 0
      ? [...allowedNames].sort().join(", ")
      : "readFile, editFile, find, shell, subagent, fetchWeb, webSearch";
  const shellAllowed = !allowedNames || allowedNames.size === 0 || allowedNames.has("shell");
  const shellFallback = shellAllowed
    ? " Default fallback: call Shell again; the host maps it to shell and runs it on the workspace (include intent). "
    : " ";
  return (
    `Your previous tool call was rejected or not mappable: ${unique.join(", ")}. ` +
    "Use normal Cursor built-in tools (Read, StrReplace, Write, Grep, Glob, Shell, Delete, SemanticSearch, Task, etc.); " +
    "the Solomon host bridge intercepts them and executes on the real workspace when mapped. " +
    `Host-enabled capabilities: ${enabled}. ` +
    shellFallback +
    "Prefer Read for file inspection, StrReplace/Write for edits, and Grep/Glob for search. " +
    "For nested work use Task (mapped to subagent). For web content use fetchWeb or webSearch when available. " +
    "Send a corrected tool call only, or continue without tools if you meant plain text."
  );
}

function wouldBridgeTool(name: string, rawArgs: unknown, bridgeCtx: ToolBridgeContext): boolean {
  const mcp = unwrapSolomonMcpCall(name, rawArgs);
  if (mcp) {
    return bridgeToolInvocation(mcp.toolName, mcp.args, bridgeCtx) !== null;
  }
  return bridgeToolInvocation(name, rawArgs, bridgeCtx) !== null;
}

export function processStreamEvent(
  event: SDKMessage,
  allowCursorInternalTools: boolean,
  onText: (s: string) => void,
  onThinking: (s: string) => void,
  pendingLegacy: LegacyToolInvocation[],
  onToolDetected: () => void,
  onBlockedTool?: (name: string) => void,
  bridgeCtx: ToolBridgeContext = { allowedNames: null },
  onUnmappedToolEvent?: (event: CursorNativeToolEvent) => void,
): void {
  const reportBlocked = (name: string): void => {
    if (onBlockedTool) {
      onBlockedTool(name);
    }
  };
  const handleToolProposal = (name: string, rawArgs: unknown, callId?: string): void => {
    const mcp = unwrapSolomonMcpCall(name, rawArgs);
    if (mcp) {
      if (tryCollectLegacyTool(pendingLegacy, mcp.toolName, mcp.args, bridgeCtx)) {
        onToolDetected();
      } else {
        reportBlocked(`mcp:${mcp.toolName}`);
      }
      return;
    }
    if (name === "mcp") {
      reportBlocked("mcp:external");
      return;
    }
    if (tryCollectLegacyTool(pendingLegacy, name, rawArgs, bridgeCtx)) {
      onToolDetected();
      return;
    }
    if (!allowCursorInternalTools) {
      reportBlocked(name);
      return;
    }
    if (onUnmappedToolEvent) {
      onUnmappedToolEvent(unmappedToolEvent(name, "running", rawArgs, undefined, undefined, callId));
    }
  };
  if (event.type === "assistant") {
    let afterTool = false;
    for (const block of event.message.content) {
      if (block.type === "tool_use") {
        afterTool = true;
        handleToolProposal(block.name, block.input, block.id);
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
    if (wouldBridgeTool(event.name, event.args, bridgeCtx)) {
      if (event.status === "completed" || event.status === "error") {
        return;
      }
      if (event.args !== undefined) {
        handleToolProposal(event.name, event.args, event.call_id);
      }
      return;
    }
    if (!allowCursorInternalTools) {
      if (event.status === "completed") {
        return;
      }
      if (event.args !== undefined) {
        handleToolProposal(event.name, event.args, event.call_id);
      } else {
        reportBlocked(event.name);
      }
      return;
    }
    if (onUnmappedToolEvent) {
      onUnmappedToolEvent(unmappedToolEventFromToolCall(event));
    }
    return;
  }
  if (event.type === "task" && event.text) {
    onThinking(event.text);
  }
}

/** @deprecated inline proxy errors replaced by solomon_proxy_correction */
export function blockedCursorToolLine(name: string): string {
  const safe = name.replace(/[^\w:.-]/g, "").slice(0, 128) || "unknown";
  return `\n[error] Cursor internal tool call blocked by Solomon proxy: ${safe}\n`;
}
