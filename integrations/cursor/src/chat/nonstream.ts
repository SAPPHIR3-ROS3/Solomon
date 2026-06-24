import { type SDKAgent } from "@cursor/sdk";
import type { ServerResponse } from "node:http";
import { formatBridgedToolCallsBlock } from "../legacy.js";
import type { ChatMessage } from "../openai-types.js";
import {
  buildOpenAIUsage,
  finishReasonForTools,
  nextTextChunk,
} from "../chat-helpers.js";
import { createAgentToolStreamState, drainAgentToolStream } from "../chat/helpers/stream-loop.js";
import type { CursorNativeToolEvent } from "../cursor-native-tools.js";
import { openAIToolCallsFromInvocations } from "../openai-tools.js";
import { sendJsonResponse } from "../openai-sse.js";
import type { ModelSelection } from "../model-selection.js";
import {
  type AgentRun,
  type ClientAbortHandle,
  finalizeAgentRun,
  wireClientAbort,
  type CursorTurnUsage,
} from "../run-control.js";
import { disposeAgent, sendStateless, type AgentSendOpts } from "../cursor-agent.js";
import { observeProxyTurn } from "../proxy-observability.js";
import type { ProxyConfig } from "./index.js";
import { finalizeTurnToolResults, type TurnOpts } from "./turn.js";

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
    const streamState = createAgentToolStreamState();
    const nativeToolEvents: CursorNativeToolEvent[] = [];
    await drainAgentToolStream(
      run,
      cfg.allowCursorInternalTools,
      { allowedNames: turnOpts.allowedNames },
      {
        onText: (t) => { content += nextTextChunk(content, t); },
        onThinking: (t) => { reasoning += t; },
        onUnmappedToolEvent: cfg.allowCursorInternalTools
          ? (ev) => { nativeToolEvents.push(ev); }
          : undefined,
      },
      streamState,
      { shouldStop: () => clientAborted },
    );
    const { toolDetected } = streamState;
    if (clientAborted) {
      return;
    }
    const finalized = finalizeTurnToolResults(streamState, content, turnOpts);
    observeProxyTurn({
      stream: false,
      bridgedTools: finalized.bridged.map((b) => b.name),
      blockedTools: finalized.blockedTools,
      proxyCorrection: finalized.proxyCorrection !== undefined,
    });
    content = finalized.content;
    const toolCalls = openAIToolCallsFromInvocations(finalized.bridged);
    const proxyCorrection = finalized.proxyCorrection;
    if (finalized.bridged.length > 0 && !turnOpts.nativeTools) {
      content = (toolDetected ? "" : content) + formatBridgedToolCallsBlock(finalized.bridged);
    }
    if (res.writableEnded || res.destroyed) {
      return;
    }
    const finishReason = finishReasonForTools(finalized.bridged.length, turnOpts.nativeTools);
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
