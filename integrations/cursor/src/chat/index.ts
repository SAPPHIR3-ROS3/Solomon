import { Cursor } from "@cursor/sdk";
import type { ServerResponse } from "node:http";
import { sanitizeModelId } from "../messages.js";
import type { ChatCompletionRequest } from "../openai-types.js";
import { filterFlagshipModelIDs, orderModelIDs } from "../model-filter.js";
import { resolveModelSelection, type ModelInfo } from "../model-selection.js";
import type { ClientAbortHandle } from "../run-control.js";
import { newCompletionId } from "../sessions.js";
import { turnOptsFromRequest } from "./turn.js";
import { streamCompletion } from "./stream.js";
import { handleNonStream } from "./nonstream.js";

export type ProxyConfig = {
  apiKey: string;
  cwd: string;
  allowCursorInternalTools: boolean;
};

let cachedModels: { apiKey: string; models: ModelInfo[]; expiresAt: number } | undefined;

async function cursorModels(apiKey: string): Promise<ModelInfo[]> {
  const now = Date.now();
  if (cachedModels?.apiKey === apiKey && cachedModels.expiresAt > now) {
    return cachedModels.models;
  }
  const models = await Cursor.models.list({ apiKey });
  cachedModels = { apiKey, models: models as ModelInfo[], expiresAt: now + 60_000 };
  return cachedModels.models;
}

async function cursorModelIDs(apiKey: string): Promise<string[]> {
  const models = await cursorModels(apiKey);
  const ids: string[] = [];
  for (const m of models) {
    if (m.id) {
      ids.push(m.id);
    }
  }
  return ids;
}

export async function listModels(apiKey: string): Promise<string[]> {
  return filterFlagshipModelIDs(await cursorModelIDs(apiKey));
}

export async function listAllModels(apiKey: string): Promise<string[]> {
  return orderModelIDs(await cursorModelIDs(apiKey));
}

export async function handleChatCompletions(
  body: ChatCompletionRequest,
  clientAbort: ClientAbortHandle,
  res: ServerResponse,
  cfg: ProxyConfig,
): Promise<void> {
  const req = body;
  const model = sanitizeModelId(req.model);
  const messages = req.messages ?? [];
  const stream = req.stream === true;
  const modelSelection = resolveModelSelection(
    await cursorModels(cfg.apiKey),
    model,
    req.reasoning_effort,
    req.solomon_fast_mode ?? true,
  );
  const completionId = newCompletionId();
  const turnOpts = turnOptsFromRequest(req);
  if (!stream) {
    await handleNonStream(cfg, messages, completionId, model, modelSelection, clientAbort, res, turnOpts);
    return;
  }
  await streamCompletion(cfg, messages, completionId, model, modelSelection, clientAbort, res, turnOpts);
}
