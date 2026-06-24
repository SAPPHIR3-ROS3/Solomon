import type { ChatCompletionRequest, ChatCompletionTool, ChatMessage } from "../openai-types.js";
import {
  allowedToolNamesFromRequest,
  filterInvocations,
  limitInvocations,
  requestUsesNativeTools,
} from "../openai-tools.js";
import { chunkDelta, writeSSE } from "../openai-sse.js";
import type { ServerResponse } from "node:http";
import {
  formatBridgedToolCallsBlock,
  type BridgedToolInvocation,
} from "../legacy.js";
import { writeSSEToolCalls } from "../openai-tools.js";
import { buildOpenAIUsage, proxyToolCorrectionMessage } from "../chat-helpers.js";
import type { CursorTurnUsage, StreamUsageInput } from "../run-control.js";

export type TurnOpts = {
  tools?: ChatCompletionTool[];
  nativeTools: boolean;
  allowedNames: Set<string> | null;
  parallelToolCalls?: boolean;
};

function promptToolsFromRequest(req: ChatCompletionRequest): ChatCompletionTool[] | undefined {
  if (req.tool_choice === "none") {
    return undefined;
  }
  if (typeof req.tool_choice === "object") {
    const chosen = req.tool_choice.function.name;
    return req.tools?.filter((t) => t.function.name === chosen);
  }
  return req.tools;
}

export function turnOptsFromRequest(req: ChatCompletionRequest): TurnOpts {
  return {
    tools: promptToolsFromRequest(req),
    nativeTools: requestUsesNativeTools(req.tools, req.tool_choice),
    allowedNames: allowedToolNamesFromRequest(req.tools, req.tool_choice),
    parallelToolCalls: req.parallel_tool_calls,
  };
}

export function streamUsageInput(
  messages: ChatMessage[],
  sdkUsage: CursorTurnUsage | undefined,
  textBuf: string,
  thinkingBuf: string,
): StreamUsageInput {
  return {
    messages,
    sdkUsage,
    textBuf,
    thinkingBuf,
    buildUsage: buildOpenAIUsage,
  };
}

export function resolveProxyCorrection(
  blockedTools: string[],
  bridgedCount: number,
  turnOpts: TurnOpts,
): string | undefined {
  if (blockedTools.length === 0 || bridgedCount > 0) {
    return undefined;
  }
  const msg = proxyToolCorrectionMessage(blockedTools, turnOpts.allowedNames);
  return msg || undefined;
}

export function emitBridgedTools(
  res: ServerResponse,
  completionId: string,
  model: string,
  pending: BridgedToolInvocation[],
  turnOpts: TurnOpts,
): BridgedToolInvocation[] {
  const bridged = limitInvocations(
    filterInvocations(pending, turnOpts.allowedNames),
    turnOpts.parallelToolCalls,
  );
  if (bridged.length === 0) {
    return bridged;
  }
  if (turnOpts.nativeTools) {
    writeSSEToolCalls(res, completionId, model, bridged);
  } else {
    writeSSE(res, chunkDelta(completionId, model, { content: formatBridgedToolCallsBlock(bridged) }));
  }
  return bridged;
}

export function emitBufferedReasoning(
  res: ServerResponse,
  completionId: string,
  model: string,
  thinkingBuf: string,
): void {
  if (thinkingBuf) {
    writeSSE(res, chunkDelta(completionId, model, { reasoning_content: thinkingBuf }));
  }
}

export function emitBufferedContent(
  res: ServerResponse,
  completionId: string,
  model: string,
  content: string,
): void {
  if (content) {
    writeSSE(res, chunkDelta(completionId, model, { content }));
  }
}
