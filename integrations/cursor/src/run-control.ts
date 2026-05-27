import type { SDKAgent } from "@cursor/sdk";
import type { IncomingMessage, ServerResponse } from "node:http";
import type { ChatMessage } from "./openai-types.js";
import { chunkDelta, finishSSE, usageChunk, writeSSE, type OpenAIUsagePayload } from "./openai-sse.js";
import { resetSessionAgent } from "./sessions.js";

export type AgentRun = Awaited<ReturnType<SDKAgent["send"]>>;

const runReleaseTimeoutMs = 15_000;

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

export async function releaseRun(run: AgentRun | undefined, timeoutMs = runReleaseTimeoutMs): Promise<boolean> {
  if (!run) {
    return true;
  }
  await forceStopRun(run);
  let released = false;
  await Promise.race([
    waitRun(run).then(() => {
      released = true;
    }),
    new Promise<void>((resolve) => {
      setTimeout(resolve, timeoutMs);
    }),
  ]);
  return released;
}

export async function finalizeAgentRun(
  sessionKey: string,
  run: AgentRun | undefined,
): Promise<void> {
  const released = await releaseRun(run);
  if (!released) {
    resetSessionAgent(sessionKey);
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
