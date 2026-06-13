import { type SDKAgent } from "@cursor/sdk";
import type { ServerResponse } from "node:http";
import { formatLegacyToolCallsBlock, type LegacyToolInvocation } from "../legacy.js";
import type { ChatMessage } from "../openai-types.js";
import {
  filterInvocations,
  limitInvocations,
  openAIToolCallsFromInvocations,
} from "../openai-tools.js";
import { sendJsonResponse } from "../openai-sse.js";
import type { ModelSelection } from "../model-selection.js";
import {
  type AgentRun,
  type ClientAbortHandle,
  forceStopRun,
  finalizeAgentRun,
  wireClientAbort,
  type CursorTurnUsage,
} from "../run-control.js";
import { disposeAgent, sendStateless, type AgentSendOpts } from "../cursor-agent.js";
import {
  buildOpenAIUsage,
  finishReasonForTools,
  nativeInvocationsFromText,
  nextTextChunk,
  processStreamEvent,
} from "../chat-helpers.js";
import type { CursorNativeToolEvent } from "../cursor-native-tools.js";
import type { ProxyConfig } from "./index.js";
import { resolveProxyCorrection, type TurnOpts } from "./turn.js";

export async function handleNonStream(
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
