import type { SDKAgent } from "@cursor/sdk";
import type { IncomingMessage, ServerResponse } from "node:http";
import type { ChatMessage } from "./openai-types.js";
import { chunkDelta, finishSSE, usageChunk, writeSSE, type OpenAIUsagePayload } from "./openai-sse.js";

export type AgentRun = Awaited<ReturnType<SDKAgent["send"]>>;

export async function forceStopRun(run: AgentRun | undefined): Promise<void> {
  if (!run?.supports("cancel")) {
    return;
  }
  try {
    await run.cancel();
  } catch {
    /* ignore */
  }
}

export async function waitRun(run: AgentRun | undefined): Promise<void> {
  if (!run) {
    return;
  }
  try {
    await run.wait();
  } catch {
    /* ignore */
  }
}

export function wireClientAbort(
  httpReq: IncomingMessage,
  res: ServerResponse,
  getRun: () => AgentRun | undefined,
  onAbort: () => void,
): () => void {
  let fired = false;
  const fire = () => {
    if (fired) {
      return;
    }
    fired = true;
    onAbort();
    void forceStopRun(getRun());
  };
  httpReq.on("aborted", fire);
  const onResClose = () => {
    if (!res.writableFinished) {
      fire();
    }
  };
  res.on("close", onResClose);
  return () => {
    httpReq.off("aborted", fire);
    res.off("close", onResClose);
  };
}

export function endOpenAIStream(
  res: ServerResponse,
  completionId: string,
  model: string,
  usage: OpenAIUsagePayload,
): void {
  if (res.writableEnded || res.destroyed) {
    return;
  }
  writeSSE(res, usageChunk(completionId, model, usage));
  writeSSE(res, chunkDelta(completionId, model, {}, "stop"));
  finishSSE(res);
}

export type CursorTurnUsage = {
  inputTokens: number;
  outputTokens: number;
  cacheReadTokens?: number;
};

export type StreamUsageInput = {
  messages: ChatMessage[];
  sdkUsage: CursorTurnUsage | undefined;
  textBuf: string;
  thinkingBuf: string;
  buildUsage: (
    messages: ChatMessage[],
    sdkUsage: CursorTurnUsage | undefined,
    textBuf: string,
    thinkingBuf: string,
  ) => OpenAIUsagePayload;
};

export function finishStreamWithUsage(
  res: ServerResponse,
  completionId: string,
  model: string,
  input: StreamUsageInput,
): void {
  endOpenAIStream(
    res,
    completionId,
    model,
    input.buildUsage(input.messages, input.sdkUsage, input.textBuf, input.thinkingBuf),
  );
}
