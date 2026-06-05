import type { SDKAgent } from "@cursor/sdk";
import type { IncomingMessage, ServerResponse } from "node:http";
import type { ChatMessage } from "./openai-types.js";
import { chunkDelta, finishSSE, usageChunk, writeSSE, type OpenAIUsagePayload } from "./openai-sse.js";

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

export function isStaleAgentError(err: unknown): boolean {
  const msg = err instanceof Error ? err.message : String(err);
  const lower = msg.toLowerCase();
  return (
    lower.includes("agent busy") ||
    lower.includes("active run") ||
    lower.includes("already has an active run") ||
    /agent agent-[0-9a-f-]+/i.test(msg)
  );
}

export async function finalizeAgentRun(run: AgentRun | undefined): Promise<void> {
  await releaseRun(run);
}

export type ClientAbortHandle = {
  onAborted(listener: () => void): void;
  offAborted(listener: () => void): void;
};

export function clientAbortFromRequest(req: IncomingMessage): ClientAbortHandle {
  return {
    onAborted: (listener) => {
      req.on("aborted", listener);
    },
    offAborted: (listener) => {
      req.off("aborted", listener);
    },
  };
}

export function wireClientAbort(
  clientAbort: ClientAbortHandle,
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
  clientAbort.onAborted(fire);
  const onResClose = () => {
    if (!res.writableFinished) {
      fire();
    }
  };
  res.on("close", onResClose);
  return () => {
    clientAbort.offAborted(fire);
    res.off("close", onResClose);
  };
}

export type OpenAIFinishReason = "stop" | "tool_calls" | null;

export function endOpenAIStream(
  res: ServerResponse,
  completionId: string,
  model: string,
  usage: OpenAIUsagePayload,
  finishReason: OpenAIFinishReason = "stop",
  proxyCorrection?: string,
): void {
  if (res.writableEnded || res.destroyed) {
    return;
  }
  writeSSE(res, usageChunk(completionId, model, usage, proxyCorrection));
  writeSSE(res, chunkDelta(completionId, model, {}, finishReason));
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
  finishReason: OpenAIFinishReason = "stop",
  proxyCorrection?: string,
): void {
  endOpenAIStream(
    res,
    completionId,
    model,
    input.buildUsage(input.messages, input.sdkUsage, input.textBuf, input.thinkingBuf),
    finishReason,
    proxyCorrection,
  );
}
