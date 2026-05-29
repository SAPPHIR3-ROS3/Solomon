import type { ServerResponse } from "node:http";

export const JSON_RESPONSE_HEADERS: Record<string, string> = {
  "Content-Type": "application/json",
  "X-Content-Type-Options": "nosniff",
};

export const SSE_RESPONSE_HEADERS: Record<string, string> = {
  "Content-Type": "text/event-stream",
  "Cache-Control": "no-cache",
  Connection: "keep-alive",
  "X-Content-Type-Options": "nosniff",
};

export function writeSSE(res: ServerResponse, payload: unknown): boolean {
  if (res.writableEnded || res.destroyed) {
    return false;
  }
  try {
    res.write(`data: ${JSON.stringify(payload)}\n\n`);
    return true;
  } catch {
    return false;
  }
}

export function finishSSE(res: ServerResponse): void {
  if (res.writableEnded || res.destroyed) {
    return;
  }
  try {
    res.write("data: [DONE]\n\n");
    res.end();
  } catch {
    /* ignore */
  }
}

export function chunkDelta(
  id: string,
  model: string,
  delta: Record<string, unknown>,
  finishReason: string | null = null,
): Record<string, unknown> {
  return {
    id,
    object: "chat.completion.chunk",
    created: Math.floor(Date.now() / 1000),
    model,
    choices: [
      {
        index: 0,
        delta,
        finish_reason: finishReason,
      },
    ],
  };
}

export type OpenAIUsagePayload = {
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  prompt_tokens_details?: { cached_tokens: number };
  completion_tokens_details?: { reasoning_tokens: number };
};

export function usageChunk(
  id: string,
  model: string,
  usage: OpenAIUsagePayload,
): Record<string, unknown> {
  return {
    id,
    object: "chat.completion.chunk",
    created: Math.floor(Date.now() / 1000),
    model,
    choices: [{ index: 0, delta: {}, finish_reason: null }],
    usage,
  };
}
