import { type SDKAgent } from "@cursor/sdk";
import type { ServerResponse } from "node:http";
import { sanitizeReflectedText } from "../messages.js";
import { formatBridgedToolCallsBlock } from "../legacy.js";
import type { ChatMessage } from "../openai-types.js";
import { writeSSEToolCalls } from "../openai-tools.js";
import {
  chunkDelta,
  finishSSE,
  sendJsonResponse,
  SSE_RESPONSE_HEADERS,
  writeSSE,
} from "../openai-sse.js";
import type { ModelSelection } from "../model-selection.js";
import {
  type AgentRun,
  type ClientAbortHandle,
  finalizeAgentRun,
  finishStreamWithUsage,
  wireClientAbort,
  type CursorTurnUsage,
  type OpenAIFinishReason,
} from "../run-control.js";
import { disposeAgent, sendStateless, type AgentSendOpts } from "../cursor-agent.js";
import {
  buildOpenAIUsage,
  finishReasonForTools,
  nextTextChunk,
} from "../chat-helpers.js";
import { createAgentToolStreamState, drainAgentToolStream } from "../chat/helpers/stream-loop.js";
import { cursorToolEventChunk, type CursorNativeToolEvent } from "../cursor-native-tools.js";
import { observeProxyTurn } from "../proxy-observability.js";
import type { ProxyConfig } from "./index.js";
import {
  emitBufferedContent,
  emitBufferedReasoning,
  finalizeTurnToolResults,
  streamUsageInput,
  type TurnOpts,
} from "./turn.js";

export async function streamCompletion(
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
    const streamState = createAgentToolStreamState();
    const emitUnmappedTool = (ev: CursorNativeToolEvent): void => {
      writeSSE(res, cursorToolEventChunk(completionId, model, ev));
    };
    try {
      await drainAgentToolStream(
        run,
        cfg.allowCursorInternalTools,
        { allowedNames: turnOpts.allowedNames },
        {
          onText: (t) => {
            if ((streamState.toolDetected && streamState.pendingBridged.length > 0) || clientAborted) {
              return;
            }
            proseBuf += nextTextChunk(proseBuf, t);
          },
          onThinking: (t) => {
            if (clientAborted) {
              return;
            }
            thinkingBuf += t;
            writeSSE(res, chunkDelta(completionId, model, { reasoning_content: t }));
            emittedThinkingLen += t.length;
          },
          onUnmappedToolEvent: cfg.allowCursorInternalTools ? emitUnmappedTool : undefined,
        },
        streamState,
        {
          shouldStop: () => clientAborted || legacyEmitted,
          onBridgedCollected: () => {
            legacyEmitted = true;
          },
        },
      );
      const { pendingBridged } = streamState;
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
      const finalized = finalizeTurnToolResults(streamState, proseBuf, turnOpts);
      observeProxyTurn({
        stream: true,
        bridgedTools: finalized.bridged.map((b) => b.name),
        blockedTools: finalized.blockedTools,
        proxyCorrection: finalized.proxyCorrection !== undefined,
      });
      proseBuf = finalized.content;
      emitBufferedContent(res, completionId, model, proseBuf);
      if (finalized.bridged.length > 0 || pendingBridged.length > 0) {
        if (finalized.bridged.length > 0) {
          if (turnOpts.nativeTools) {
            writeSSEToolCalls(res, completionId, model, finalized.bridged);
          } else {
            const block = formatBridgedToolCallsBlock(finalized.bridged);
            writeSSE(res, chunkDelta(completionId, model, { content: block }));
            proseBuf += block;
          }
        }
        finishStream(
          finishReasonForTools(finalized.bridged.length, turnOpts.nativeTools),
          finalized.proxyCorrection,
        );
        return;
      }
      if (!legacyEmitted) {
        finishStream("stop", finalized.proxyCorrection);
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
