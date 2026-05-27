import { Agent, Cursor, type SDKMessage, type SDKUserMessage } from "@cursor/sdk";
import type { IncomingMessage, ServerResponse } from "node:http";
import {
  formatDeltaMessage,
  messageToUserPayload,
  roughTokFromMessages,
  roughTokFromString,
  withHarnessPreamble,
} from "./messages.js";
import {
  formatLegacyToolCallsBlock,
  tryCollectLegacyTool,
  type LegacyToolInvocation,
} from "./legacy.js";
import type { ChatCompletionRequest, ChatMessage } from "./openai-types.js";
import {
  chunkDelta,
  finishSSE,
  usageChunk,
  writeSSE,
  type OpenAIUsagePayload,
} from "./openai-sse.js";
import { filterFlagshipModelIDs, orderModelIDs } from "./model-filter.js";
import { resolveModelSelection, type ModelInfo } from "./model-selection.js";
import {
  type AgentRun,
  forceStopRun,
  finalizeAgentRun,
  finishStreamWithUsage,
  wireClientAbort,
  type CursorTurnUsage,
} from "./run-control.js";
import {
  getSession,
  sessionKey,
  setSession,
  type SessionState,
} from "./sessions.js";

export type ProxyConfig = {
  apiKey: string;
  cwd: string;
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
  httpReq: IncomingMessage,
  res: ServerResponse,
  cfg: ProxyConfig,
): Promise<void> {
  const req = body;
  const model = (req.model ?? "composer-2.5").trim() || "composer-2.5";
  const messages = req.messages ?? [];
  const stream = req.stream !== false;
  const key = sessionKey(messages, cfg.cwd);
  const modelSelection = resolveModelSelection(
    await cursorModels(cfg.apiKey),
    model,
    req.reasoning_effort,
    req.solomon_fast_mode ?? true,
  );
  const modelKey = JSON.stringify(modelSelection);
  let state = getSession(key);
  if (!state || state.modelKey !== modelKey) {
    if (state?.agent) {
      try {
        await state.agent.close();
      } catch {
        /* ignore */
      }
    }
    const agent = await Agent.create({
      apiKey: cfg.apiKey,
      model: modelSelection,
      local: { cwd: cfg.cwd, settingSources: [] },
    });
    state = { agent, syncedMessages: 0, model, modelKey, modelSelection };
    setSession(key, state);
  } else {
    state.modelSelection = modelSelection;
  }
  const delta = messages.slice(state.syncedMessages);
  if (delta.length === 0 && messages.length > 0) {
    const last = messages[messages.length - 1];
    if (last.role === "user") {
      delta.push(last);
    }
  }
  const prompt = withHarnessPreamble(buildPromptFromDelta(delta, messages));
  state.syncedMessages = messages.length;
  const completionId = "chatcmpl-" + key;
  if (!stream) {
    await handleNonStream(key, state, prompt, messages, completionId, model, httpReq, res);
    return;
  }
  await streamCompletion(key, state, prompt, messages, completionId, model, httpReq, res);
}

function buildOpenAIUsage(
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

function emitStreamUsage(
  res: ServerResponse,
  completionId: string,
  model: string,
  messages: ChatMessage[],
  sdkUsage: CursorTurnUsage | undefined,
  textBuf: string,
  thinkingBuf: string,
): void {
  writeSSE(
    res,
    usageChunk(
      completionId,
      model,
      buildOpenAIUsage(messages, sdkUsage, textBuf, thinkingBuf),
    ),
  );
}

function buildPromptFromDelta(
  delta: ChatMessage[],
  all: ChatMessage[],
): string | SDKUserMessage {
  if (delta.length === 1 && delta[0].role === "user") {
    return messageToUserPayload(delta[0]);
  }
  const lines: string[] = [];
  for (const m of delta) {
    lines.push(formatDeltaMessage(m));
  }
  if (lines.length === 0 && all.length > 0) {
    const last = all[all.length - 1];
    if (last.role === "user") {
      return messageToUserPayload(last);
    }
    return formatDeltaMessage(last);
  }
  return lines.join("\n\n");
}

async function streamCompletion(
  sessionKey: string,
  state: SessionState,
  prompt: string | SDKUserMessage,
  messages: ChatMessage[],
  completionId: string,
  model: string,
  httpReq: IncomingMessage,
  res: ServerResponse,
): Promise<void> {
  let sdkUsage: CursorTurnUsage | undefined;
  let run: AgentRun | undefined;
  let clientAborted = false;
  const unwireAbort = wireClientAbort(httpReq, res, () => run, () => {
    clientAborted = true;
  });
  try {
    try {
      run = await state.agent.send(prompt, {
        model: state.modelSelection,
        onDelta: async ({ update }) => {
          if (update.type !== "turn-ended" || !update.usage) {
            return;
          }
          sdkUsage = {
            inputTokens: update.usage.inputTokens ?? 0,
            outputTokens: update.usage.outputTokens ?? 0,
            cacheReadTokens: update.usage.cacheReadTokens,
          };
        },
      });
    } catch (err) {
      sendStreamStartError(res, completionId, model, err);
      return;
    }
    if (clientAborted) {
      if (res.headersSent) {
        finishStreamWithUsage(res, completionId, model, {
          messages,
          sdkUsage,
          textBuf: "",
          thinkingBuf: "",
          buildUsage: buildOpenAIUsage,
        });
      }
      return;
    }
    res.writeHead(200, {
      "Content-Type": "text/event-stream",
      "Cache-Control": "no-cache",
      Connection: "keep-alive",
    });
    res.on("error", () => {});
    let proseBuf = "";
    let thinkingBuf = "";
    let legacyEmitted = false;
    let toolDetected = false;
    const pendingLegacy: LegacyToolInvocation[] = [];
    try {
      for await (const event of run.stream()) {
        if (clientAborted || legacyEmitted) {
          break;
        }
        processStreamEvent(
          event,
          (t) => {
            if (toolDetected || clientAborted) {
              return;
            }
            proseBuf += t;
            writeSSE(res, chunkDelta(completionId, model, { content: t }));
          },
          (t) => {
            if (clientAborted) {
              return;
            }
            thinkingBuf += t;
            writeSSE(res, chunkDelta(completionId, model, { reasoning_content: t }));
          },
          pendingLegacy,
          () => {
            toolDetected = true;
          },
        );
        if (toolDetected && pendingLegacy.length > 0) {
          legacyEmitted = true;
          await forceStopRun(run);
          break;
        }
      }
      const finishStream = () =>
        finishStreamWithUsage(res, completionId, model, {
          messages,
          sdkUsage,
          textBuf: proseBuf,
          thinkingBuf,
          buildUsage: buildOpenAIUsage,
        });
      if (clientAborted) {
        finishStream();
        return;
      }
      if (pendingLegacy.length > 0) {
        const block = formatLegacyToolCallsBlock(pendingLegacy);
        writeSSE(res, chunkDelta(completionId, model, { content: block }));
        proseBuf += block;
        finishStream();
        return;
      }
      if (!legacyEmitted) {
        finishStream();
      }
    } catch (err) {
      const finishStream = () =>
        finishStreamWithUsage(res, completionId, model, {
          messages,
          sdkUsage,
          textBuf: proseBuf,
          thinkingBuf,
          buildUsage: buildOpenAIUsage,
        });
      if (clientAborted) {
        finishStream();
        return;
      }
      const msg = err instanceof Error ? err.message : String(err);
      proseBuf += `\n[error] ${msg}`;
      writeSSE(res, chunkDelta(completionId, model, { content: `\n[error] ${msg}` }));
      finishStream();
    }
  } finally {
    unwireAbort();
    await finalizeAgentRun(sessionKey, run);
  }
}

function sendStreamStartError(
  res: ServerResponse,
  completionId: string,
  model: string,
  err: unknown,
): void {
  const msg = err instanceof Error ? err.message : String(err);
  if (!res.headersSent) {
    res.writeHead(500, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ error: { message: msg, type: "proxy_error" } }));
    return;
  }
  writeSSE(res, chunkDelta(completionId, model, { content: `\n[error] ${msg}` }));
  writeSSE(res, chunkDelta(completionId, model, {}, "stop"));
  finishSSE(res);
}

function processStreamEvent(
  event: SDKMessage,
  onText: (s: string) => void,
  onThinking: (s: string) => void,
  pendingLegacy: LegacyToolInvocation[],
  onToolDetected: () => void,
): void {
  if (event.type === "assistant") {
    let afterTool = false;
    for (const block of event.message.content) {
      if (block.type === "tool_use") {
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
    if (event.status === "completed" || event.status === "error") {
      return;
    }
    if (event.args !== undefined && tryCollectLegacyTool(pendingLegacy, event.name, event.args)) {
      onToolDetected();
    }
  }
}

async function handleNonStream(
  sessionKey: string,
  state: SessionState,
  prompt: string | SDKUserMessage,
  messages: ChatMessage[],
  completionId: string,
  model: string,
  httpReq: IncomingMessage,
  res: ServerResponse,
): Promise<void> {
  let sdkUsage: CursorTurnUsage | undefined;
  let run: AgentRun | undefined;
  let clientAborted = false;
  const unwireAbort = wireClientAbort(httpReq, res, () => run, () => {
    clientAborted = true;
  });
  try {
    run = await state.agent.send(prompt, {
      model: state.modelSelection,
      onDelta: async ({ update }) => {
        if (update.type !== "turn-ended" || !update.usage) {
          return;
        }
        sdkUsage = {
          inputTokens: update.usage.inputTokens ?? 0,
          outputTokens: update.usage.outputTokens ?? 0,
          cacheReadTokens: update.usage.cacheReadTokens,
        };
      },
    });
    let content = "";
    let reasoning = "";
    const pendingLegacy: LegacyToolInvocation[] = [];
    let toolDetected = false;
    for await (const event of run.stream()) {
      if (clientAborted) {
        break;
      }
      if (event.type === "assistant") {
        let afterTool = false;
        for (const block of event.message.content) {
          if (block.type === "tool_use") {
            if (tryCollectLegacyTool(pendingLegacy, block.name, block.input)) {
              afterTool = true;
              toolDetected = true;
            }
            continue;
          }
          if (block.type === "text" && block.text && !afterTool) {
            content += block.text;
          }
        }
        if (toolDetected) {
          await forceStopRun(run);
          break;
        }
      } else if (event.type === "thinking") {
        reasoning += event.text;
      } else if (event.type === "tool_call") {
        if (event.status === "completed" || event.status === "error") {
          continue;
        }
        if (event.args !== undefined && tryCollectLegacyTool(pendingLegacy, event.name, event.args)) {
          toolDetected = true;
          await forceStopRun(run);
          break;
        }
      }
    }
    if (clientAborted) {
      return;
    }
    if (toolDetected) {
      await forceStopRun(run);
    }
    if (pendingLegacy.length > 0) {
      content = (toolDetected ? "" : content) + formatLegacyToolCallsBlock(pendingLegacy);
    }
    if (res.writableEnded || res.destroyed) {
      return;
    }
    const body = {
      id: completionId,
      object: "chat.completion",
      created: Math.floor(Date.now() / 1000),
      model,
      choices: [
        {
          index: 0,
          message: {
            role: "assistant",
            content,
            ...(reasoning ? { reasoning_content: reasoning } : {}),
          },
          finish_reason: "stop",
        },
      ],
      usage: buildOpenAIUsage(messages, sdkUsage, content, reasoning),
    };
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify(body));
  } finally {
    unwireAbort();
    await finalizeAgentRun(sessionKey, run);
  }
}
