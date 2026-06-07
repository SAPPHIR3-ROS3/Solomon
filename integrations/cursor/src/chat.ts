import { Cursor, type SDKAgent } from "@cursor/sdk";
import type { ServerResponse } from "node:http";
import {
  sanitizeModelId,
  sanitizeReflectedText,
} from "./messages.js";
import {
  formatLegacyToolCallsBlock,
  type LegacyToolInvocation,
} from "./legacy.js";
import type { ChatCompletionRequest, ChatCompletionTool, ChatMessage } from "./openai-types.js";
import {
  allowedToolNamesFromRequest,
  filterInvocations,
  limitInvocations,
  openAIToolCallsFromInvocations,
  requestUsesNativeTools,
  writeSSEToolCalls,
} from "./openai-tools.js";
import {
  chunkDelta,
  finishSSE,
  sendJsonResponse,
  SSE_RESPONSE_HEADERS,
  writeSSE,
} from "./openai-sse.js";
import { filterFlagshipModelIDs, orderModelIDs } from "./model-filter.js";
import { resolveModelSelection, type ModelInfo } from "./model-selection.js";
import {
  type AgentRun,
  type ClientAbortHandle,
  forceStopRun,
  finalizeAgentRun,
  finishStreamWithUsage,
  wireClientAbort,
  type CursorTurnUsage,
  type StreamUsageInput,
  type OpenAIFinishReason,
} from "./run-control.js";
import { newCompletionId } from "./sessions.js";
import type { ModelSelection } from "./model-selection.js";
import { disposeAgent, sendStateless, type AgentSendOpts } from "./cursor-agent.js";
import { buildOpenAIUsage, finishReasonForTools, nativeInvocationsFromText, nextTextChunk, processStreamEvent, proxyToolCorrectionMessage } from "./chat-helpers.js";
import { cursorToolEventChunk, type CursorNativeToolEvent } from "./cursor-native-tools.js";

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

type TurnOpts = {
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

function turnOptsFromRequest(req: ChatCompletionRequest): TurnOpts {
  return {
    tools: promptToolsFromRequest(req),
    nativeTools: requestUsesNativeTools(req.tools, req.tool_choice),
    allowedNames: allowedToolNamesFromRequest(req.tools, req.tool_choice),
    parallelToolCalls: req.parallel_tool_calls,
  };
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

function streamUsageInput(
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

function resolveProxyCorrection(
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

function emitBridgedTools(
  res: ServerResponse,
  completionId: string,
  model: string,
  pending: LegacyToolInvocation[],
  turnOpts: TurnOpts,
): LegacyToolInvocation[] {
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
    writeSSE(res, chunkDelta(completionId, model, { content: formatLegacyToolCallsBlock(bridged) }));
  }
  return bridged;
}

function emitBufferedReasoning(
  res: ServerResponse,
  completionId: string,
  model: string,
  thinkingBuf: string,
): void {
  if (thinkingBuf) {
    writeSSE(res, chunkDelta(completionId, model, { reasoning_content: thinkingBuf }));
  }
}

function emitBufferedContent(
  res: ServerResponse,
  completionId: string,
  model: string,
  content: string,
): void {
  if (content) {
    writeSSE(res, chunkDelta(completionId, model, { content }));
  }
}

async function streamCompletion(
  cfg: ProxyConfig,
  messages: ChatMessage[],
  completionId: string,
  model: string,
  modelSelection: ModelSelection,
  clientAbort: ClientAbortHandle,
  res: ServerResponse,
  turnOpts: TurnOpts,
): Promise<void> {
  let sdkUsage: CursorTurnUsage | undefined;
  let run: AgentRun | undefined;
  let agent: SDKAgent | undefined;
  let clientAborted = false;
  const sendOpts: AgentSendOpts = {
    model: modelSelection,
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
  };
  const unwireAbort = wireClientAbort(clientAbort, res, () => run, () => {
    clientAborted = true;
  });
  res.writeHead(200, SSE_RESPONSE_HEADERS);
  res.on("error", () => {});
  try {
    try {
      const sent = await sendStateless(cfg, modelSelection, messages, sendOpts, turnOpts.tools);
      agent = sent.agent;
      run = sent.run;
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
    let proseBuf = "";
    let thinkingBuf = "";
    let emittedThinkingLen = 0;
    let legacyEmitted = false;
    let toolDetected = false;
    const pendingLegacy: LegacyToolInvocation[] = [];
    const blockedTools: string[] = [];
    const recordBlocked = (name: string): void => {
      blockedTools.push(name);
    };
    const emitUnmappedTool = (ev: CursorNativeToolEvent): void => {
      writeSSE(res, cursorToolEventChunk(completionId, model, ev));
    };
    try {
      for await (const event of run.stream()) {
        if (clientAborted || legacyEmitted) {
          break;
        }
        processStreamEvent(
          event,
          cfg.allowCursorInternalTools,
          (t) => {
            if ((toolDetected && pendingLegacy.length > 0) || clientAborted) {
              return;
            }
            proseBuf += nextTextChunk(proseBuf, t);
          },
          (t) => {
            if (clientAborted) {
              return;
            }
            thinkingBuf += t;
            writeSSE(res, chunkDelta(completionId, model, { reasoning_content: t }));
            emittedThinkingLen += t.length;
          },
          pendingLegacy,
          () => {
            toolDetected = true;
          },
          recordBlocked,
          { allowedNames: turnOpts.allowedNames },
          cfg.allowCursorInternalTools ? emitUnmappedTool : undefined,
        );
        if (toolDetected && pendingLegacy.length > 0) {
          legacyEmitted = true;
          await forceStopRun(run);
          break;
        }
      }
      const finishStream = (finishReason: OpenAIFinishReason = "stop", proxyCorrection?: string) => {
        finishStreamWithUsage(
          res,
          completionId,
          model,
          streamUsageInput(messages, sdkUsage, proseBuf, thinkingBuf),
          finishReason,
          proxyCorrection,
        );
      };
      if (clientAborted) {
        finishStream();
        return;
      }
      emitBufferedReasoning(res, completionId, model, thinkingBuf.slice(emittedThinkingLen));
      let nativeInvocations: LegacyToolInvocation[] = [];
      if (turnOpts.nativeTools) {
        const parsed = nativeInvocationsFromText(proseBuf, turnOpts);
        proseBuf = parsed.content;
        nativeInvocations = parsed.invocations;
        blockedTools.push(...parsed.blockedTools);
      }
      emitBufferedContent(res, completionId, model, proseBuf);
      if (turnOpts.nativeTools && nativeInvocations.length > 0) {
        writeSSEToolCalls(res, completionId, model, nativeInvocations);
        finishStream("tool_calls", resolveProxyCorrection(blockedTools, nativeInvocations.length, turnOpts));
        return;
      }
      if (pendingLegacy.length > 0) {
        const bridged = emitBridgedTools(res, completionId, model, pendingLegacy, turnOpts);
        if (!turnOpts.nativeTools) {
          proseBuf += formatLegacyToolCallsBlock(bridged);
        }
        finishStream(
          finishReasonForTools(bridged.length, turnOpts.nativeTools),
          resolveProxyCorrection(blockedTools, bridged.length, turnOpts),
        );
        return;
      }
      if (!legacyEmitted) {
        finishStream("stop", resolveProxyCorrection(blockedTools, 0, turnOpts));
      }
    } catch (err) {
      if (clientAborted) {
        finishStreamWithUsage(res, completionId, model, {
          messages,
          sdkUsage,
          textBuf: proseBuf,
          thinkingBuf,
          buildUsage: buildOpenAIUsage,
        });
        return;
      }
      const msg = sanitizeReflectedText(err instanceof Error ? err.message : String(err));
      proseBuf += `\n[error] ${msg}`;
      emitBufferedReasoning(res, completionId, model, thinkingBuf.slice(emittedThinkingLen));
      emitBufferedContent(res, completionId, model, proseBuf);
      finishStreamWithUsage(res, completionId, model, {
        messages,
        sdkUsage,
        textBuf: proseBuf,
        thinkingBuf,
        buildUsage: buildOpenAIUsage,
      });
    }
  } finally {
    unwireAbort();
    await finalizeAgentRun(run);
    await disposeAgent(agent);
  }
}

function sendStreamStartError(
  res: ServerResponse,
  completionId: string,
  model: string,
  err: unknown,
): void {
  const msg = sanitizeReflectedText(err instanceof Error ? err.message : String(err));
  if (!res.headersSent) {
    sendJsonResponse(res, 500, { error: { message: msg, type: "proxy_error" } });
    return;
  }
  writeSSE(res, chunkDelta(completionId, model, { content: `\n[error] ${msg}` }));
  writeSSE(res, chunkDelta(completionId, model, {}, "stop"));
  finishSSE(res);
}

async function handleNonStream(
  cfg: ProxyConfig,
  messages: ChatMessage[],
  completionId: string,
  model: string,
  modelSelection: ModelSelection,
  clientAbort: ClientAbortHandle,
  res: ServerResponse,
  turnOpts: TurnOpts,
): Promise<void> {
  let sdkUsage: CursorTurnUsage | undefined;
  let run: AgentRun | undefined;
  let agent: SDKAgent | undefined;
  let clientAborted = false;
  const sendOpts: AgentSendOpts = {
    model: modelSelection,
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
  };
  const unwireAbort = wireClientAbort(clientAbort, res, () => run, () => {
    clientAborted = true;
  });
  try {
    const sent = await sendStateless(cfg, modelSelection, messages, sendOpts, turnOpts.tools);
    agent = sent.agent;
    run = sent.run;
    let content = "";
    let reasoning = "";
    const pendingLegacy: LegacyToolInvocation[] = [];
    const blockedTools: string[] = [];
    const nativeToolEvents: CursorNativeToolEvent[] = [];
    let toolDetected = false;
    for await (const event of run.stream()) {
      if (clientAborted) {
        break;
      }
      processStreamEvent(
        event,
        cfg.allowCursorInternalTools,
        (t) => { content += nextTextChunk(content, t); },
        (t) => { reasoning += t; },
        pendingLegacy,
        () => { toolDetected = true; },
        (name) => { blockedTools.push(name); },
        { allowedNames: turnOpts.allowedNames },
        cfg.allowCursorInternalTools
          ? (ev) => { nativeToolEvents.push(ev); }
          : undefined,
      );
      if (toolDetected && pendingLegacy.length > 0) {
        await forceStopRun(run);
        break;
      }
    }
    if (clientAborted) {
      return;
    }
    if (toolDetected && pendingLegacy.length > 0) {
      await forceStopRun(run);
    }
    const parsedNative = turnOpts.nativeTools
      ? nativeInvocationsFromText(content, turnOpts)
      : { content, invocations: [], blockedTools: [] as string[] };
    if (turnOpts.nativeTools) {
      content = parsedNative.content;
      blockedTools.push(...parsedNative.blockedTools);
    }
    const bridged = limitInvocations(
      filterInvocations([...parsedNative.invocations, ...pendingLegacy], turnOpts.allowedNames),
      turnOpts.parallelToolCalls,
    );
    const toolCalls = openAIToolCallsFromInvocations(bridged);
    const proxyCorrection = resolveProxyCorrection(blockedTools, bridged.length, turnOpts);
    if (bridged.length > 0 && !turnOpts.nativeTools) {
      content = (toolDetected ? "" : content) + formatLegacyToolCallsBlock(bridged);
    }
    if (res.writableEnded || res.destroyed) {
      return;
    }
    const finishReason = finishReasonForTools(bridged.length, turnOpts.nativeTools);
    let messageContent: string | null = content;
    if (turnOpts.nativeTools && toolCalls.length > 0) {
      messageContent = toolDetected ? content.trim() || null : content || null;
    }
    const message: Record<string, unknown> = {
      role: "assistant",
      content: messageContent,
      ...(reasoning ? { reasoning_content: reasoning } : {}),
    };
    if (turnOpts.nativeTools && toolCalls.length > 0) {
      message.tool_calls = toolCalls;
    }
    const body: Record<string, unknown> = {
      id: completionId,
      object: "chat.completion",
      created: Math.floor(Date.now() / 1000),
      model,
      choices: [
        {
          index: 0,
          message,
          finish_reason: finishReason,
        },
      ],
      usage: buildOpenAIUsage(messages, sdkUsage, content, reasoning),
    };
    if (proxyCorrection) {
      body.solomon_proxy_correction = proxyCorrection;
    }
    if (nativeToolEvents.length > 0) {
      body.solomon_cursor_tool_events = nativeToolEvents;
    }
    sendJsonResponse(res, 200, body);
  } finally {
    unwireAbort();
    await finalizeAgentRun(run);
    await disposeAgent(agent);
  }
}
