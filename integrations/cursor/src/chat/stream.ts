import { type SDKAgent } from "@cursor/sdk";
import type { ServerResponse } from "node:http";
import { sanitizeReflectedText } from "../messages.js";
import { formatLegacyToolCallsBlock, type LegacyToolInvocation } from "../legacy.js";
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
  forceStopRun,
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
  nativeInvocationsFromText,
  nextTextChunk,
  processStreamEvent,
} from "../chat-helpers.js";
import { cursorToolEventChunk, type CursorNativeToolEvent } from "../cursor-native-tools.js";
import type { ProxyConfig } from "./index.js";
import {
  emitBridgedTools,
  emitBufferedContent,
  emitBufferedReasoning,
  resolveProxyCorrection,
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
