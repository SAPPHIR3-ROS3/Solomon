import { forceStopRun, type AgentRun } from "../../run-control.js";
import type { CursorNativeToolEvent } from "../../cursor-native-tools.js";
import type { BridgedToolContext, BridgedToolInvocation } from "../../legacy.js";
import { shouldStopProxyOnBlockedTool } from "../../tool-policy.js";
import { processStreamEvent } from "./stream-events.js";

export type AgentToolStreamState = {
  pendingBridged: BridgedToolInvocation[];
  blockedTools: string[];
  toolDetected: boolean;
};

export type AgentToolStreamHandlers = {
  onText: (t: string) => void;
  onThinking: (t: string) => void;
  onUnmappedToolEvent?: (event: CursorNativeToolEvent) => void;
};

export type AgentToolStreamOptions = {
  shouldStop?: () => boolean;
  onBridgedCollected?: () => void;
};

export function createAgentToolStreamState(): AgentToolStreamState {
  return { pendingBridged: [], blockedTools: [], toolDetected: false };
}

export function shouldForceStopProxyRun(state: AgentToolStreamState): boolean {
  if (state.toolDetected && state.pendingBridged.length > 0) {
    return true;
  }
  return state.blockedTools.some(shouldStopProxyOnBlockedTool);
}

export async function drainAgentToolStream(
  run: AgentRun,
  allowCursorInternalTools: boolean,
  bridgeCtx: BridgedToolContext,
  handlers: AgentToolStreamHandlers,
  state: AgentToolStreamState,
  options: AgentToolStreamOptions = {},
): Promise<void> {
  for await (const event of run.stream()) {
    if (options.shouldStop?.()) {
      break;
    }
    processStreamEvent(
      event,
      allowCursorInternalTools,
      handlers.onText,
      handlers.onThinking,
      state.pendingBridged,
      () => {
        state.toolDetected = true;
      },
      (name) => {
        state.blockedTools.push(name);
        if (shouldStopProxyOnBlockedTool(name)) {
          state.toolDetected = true;
        }
      },
      bridgeCtx,
      handlers.onUnmappedToolEvent,
    );
    if (shouldForceStopProxyRun(state)) {
      if (state.pendingBridged.length > 0) {
        options.onBridgedCollected?.();
      }
      await forceStopRun(run);
      break;
    }
  }
}
