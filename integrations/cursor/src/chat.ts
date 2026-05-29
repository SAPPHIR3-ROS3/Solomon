import { Agent, Cursor, type SDKAgent, type SDKMessage } from "@cursor/sdk";
import type { ServerResponse } from "node:http";
import {
  buildPromptFromMessages,
  roughTokFromMessages,
  roughTokFromString,
  sanitizeModelId,
  sanitizeReflectedText,
} from "./messages.js";
import {
  formatLegacyToolCallsBlock,
  tryCollectLegacyTool,
  type LegacyToolInvocation,
} from "./legacy.js";
import type { ChatCompletionRequest, ChatCompletionTool, ChatMessage } from "./openai-types.js";
import {
  allowedToolNamesFromRequest,
  filterInvocations,
  openAIToolCallsFromInvocations,
  requestUsesNativeTools,
  writeSSEToolCalls,
} from "./openai-tools.js";
import {
  chunkDelta,
  finishSSE,
  JSON_RESPONSE_HEADERS,
  SSE_RESPONSE_HEADERS,
  usageChunk,
  writeSSE,
  type OpenAIUsagePayload,
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
  type OpenAIFinishReason,
} from "./run-control.js";
import { newCompletionId } from "./sessions.js";
import type { ModelSelection } from "./model-selection.js";

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

type AgentSendOpts = {
  model: ModelSelection;
  onDelta: (arg: {
    update: { type: string; usage?: { inputTokens?: number; outputTokens?: number; cacheReadTokens?: number } };
  }) => Promise<void>;
};

async function createAgent(cfg: ProxyConfig, modelSelection: ModelSelection): Promise<SDKAgent> {
  return Agent.create({
    apiKey: cfg.apiKey,
    model: modelSelection,
    local: { cwd: cfg.cwd, settingSources: [] },
  });
}

async function disposeAgent(agent: SDKAgent | undefined): Promise<void> {
  if (!agent) {
    return;
  }
  try {
    await agent[Symbol.asyncDispose]();
  } catch {
    /* ignore */
  }
}

type TurnOpts = {
  tools?: ChatCompletionTool[];
  nativeTools: boolean;
  allowedNames: Set<string> | null;
};

function turnOptsFromRequest(req: ChatCompletionRequest): TurnOpts {
  return {
    tools: req.tools,
    nativeTools: requestUsesNativeTools(req.tools),
    allowedNames: allowedToolNamesFromRequest(req.tools),
  };
}

async function sendStateless(
  cfg: ProxyConfig,
  modelSelection: ModelSelection,
  messages: ChatMessage[],
  sendOpts: AgentSendOpts,
  tools?: ChatCompletionTool[],
): Promise<{ agent: SDKAgent; run: AgentRun }> {
  const agent = await createAgent(cfg, modelSelection);
  const prompt = buildPromptFromMessages(messages, tools);
  try {
    const run = await agent.send(prompt, sendOpts);
    return { agent, run };
  } catch (err) {
    await disposeAgent(agent);
    throw err;
  }
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
  const stream = req.stream !== false;
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

function finishReasonForTools(bridgedCount: number, nativeTools: boolean): OpenAIFinishReason {
  if (bridgedCount > 0 && nativeTools) {
    return "tool_calls";
  }
  return "stop";
}

function emitBridgedTools(
  res: ServerResponse,
  completionId: string,
  model: string,
  pending: LegacyToolInvocation[],
  turnOpts: TurnOpts,
): LegacyToolInvocation[] {
  const bridged = filterInvocations(pending, turnOpts.allowedNames);
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
    res.writeHead(200, SSE_RESPONSE_HEADERS);
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
      const finishStream = (finishReason: OpenAIFinishReason = "stop") => {
        finishStreamWithUsage(
          res,
          completionId,
          model,
          {
            messages,
            sdkUsage,
            textBuf: proseBuf,
            thinkingBuf,
            buildUsage: buildOpenAIUsage,
          },
          finishReason,
        );
      };
      if (clientAborted) {
        finishStream();
        return;
      }
      if (pendingLegacy.length > 0) {
        const bridged = emitBridgedTools(res, completionId, model, pendingLegacy, turnOpts);
        if (!turnOpts.nativeTools) {
          proseBuf += formatLegacyToolCallsBlock(bridged);
        }
        finishStream(finishReasonForTools(bridged.length, turnOpts.nativeTools));
        return;
      }
      if (!legacyEmitted) {
        finishStream();
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
      writeSSE(res, chunkDelta(completionId, model, { content: `\n[error] ${msg}` }));
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
    res.writeHead(500, JSON_RESPONSE_HEADERS);
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
    const bridged = filterInvocations(pendingLegacy, turnOpts.allowedNames);
    const toolCalls = openAIToolCallsFromInvocations(bridged);
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
    const body = {
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
    res.writeHead(200, JSON_RESPONSE_HEADERS);
    res.end(JSON.stringify(body));
  } finally {
    unwireAbort();
    await finalizeAgentRun(run);
    await disposeAgent(agent);
  }
}
