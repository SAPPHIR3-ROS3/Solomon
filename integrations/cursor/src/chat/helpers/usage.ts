import type { BridgedToolInvocation } from "../../legacy.js";
import {
  roughTokFromMessages,
  roughTokFromString,
} from "../../messages.js";
import {
  filterInvocations,
  isValidInvocation,
  limitInvocations,
  parseToolInvocationsFromText,
} from "../../openai-tools.js";
import type { ChatMessage } from "../../openai-types.js";
import type { CursorTurnUsage, OpenAIFinishReason } from "../../run-control.js";
import type { OpenAIUsagePayload } from "../../openai-sse.js";

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
  invocations: BridgedToolInvocation[];
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
