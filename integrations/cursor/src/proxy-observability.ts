import {
  BLOCKED_MCP_EXTERNAL_LABEL,
  shouldBlockDeferredSolomonTool,
  shouldHardDenyCursorTool,
  shouldRedirectCursorTool,
} from "./tool-policy.js";

const NATIVE_ENTRY_TOOLS = new Set([
  "orchestrate",
  "searchTools",
  "subagent",
  "switchMode",
  "searchSkill",
  "loadSkill",
  "docsRetrieval",
]);

const WORKSPACE_DEFERRED_TOOLS = new Set(["readFile", "editFile", "shell", "find"]);

export const PROXY_CORRECTION_LOOP_THRESHOLD = 3;

export type ProxyToolPolicyClass = "native" | "redirect" | "hardDeny" | "deferredBlock" | "unknown";

export function classifyProxyTool(toolName: string): ProxyToolPolicyClass {
  const trimmed = toolName.trim();
  if (!trimmed) {
    return "unknown";
  }
  if (NATIVE_ENTRY_TOOLS.has(trimmed)) {
    return "native";
  }
  if (trimmed === BLOCKED_MCP_EXTERNAL_LABEL || shouldHardDenyCursorTool(trimmed)) {
    return "hardDeny";
  }
  if (trimmed.startsWith("mcp:")) {
    const inner = trimmed.slice(4);
    if (NATIVE_ENTRY_TOOLS.has(inner)) {
      return "native";
    }
    if (shouldBlockDeferredSolomonTool(inner)) {
      return "redirect";
    }
    return "unknown";
  }
  if (shouldRedirectCursorTool(trimmed)) {
    return "redirect";
  }
  if (shouldBlockDeferredSolomonTool(trimmed)) {
    return "deferredBlock";
  }
  return "unknown";
}

export type ProxyTurnObservation = {
  stream: boolean;
  bridgedTools: string[];
  blockedTools: string[];
  proxyCorrection: boolean;
};

export type ProxyObservabilitySnapshot = {
  turns: number;
  turnsWithCorrection: number;
  bridgedNativeByTool: Record<string, number>;
  blockedByClass: Record<ProxyToolPolicyClass, number>;
  correctionsByClass: Record<ProxyToolPolicyClass, number>;
  consecutiveCorrectionsByClass: Record<ProxyToolPolicyClass, number>;
  maxConsecutiveCorrectionsByClass: Record<ProxyToolPolicyClass, number>;
  deferredDirectBlocked: number;
  orchestrateBridged: number;
  workspaceMutationDeferredBlocked: number;
};

function emptyClassRecord(): Record<ProxyToolPolicyClass, number> {
  return { native: 0, redirect: 0, hardDeny: 0, deferredBlock: 0, unknown: 0 };
}

function correctionClasses(blockedTools: string[]): ProxyToolPolicyClass[] {
  const out = new Set<ProxyToolPolicyClass>();
  for (const name of blockedTools) {
    const cls = classifyProxyTool(name);
    if (cls !== "native" && cls !== "unknown") {
      out.add(cls);
    }
  }
  return [...out];
}

class ProxyObservabilityState {
  turns = 0;
  turnsWithCorrection = 0;
  bridgedNativeByTool: Record<string, number> = {};
  blockedByClass = emptyClassRecord();
  correctionsByClass = emptyClassRecord();
  consecutiveCorrectionsByClass = emptyClassRecord();
  maxConsecutiveCorrectionsByClass = emptyClassRecord();
  deferredDirectBlocked = 0;
  orchestrateBridged = 0;
  workspaceMutationDeferredBlocked = 0;

  snapshot(): ProxyObservabilitySnapshot {
    return {
      turns: this.turns,
      turnsWithCorrection: this.turnsWithCorrection,
      bridgedNativeByTool: { ...this.bridgedNativeByTool },
      blockedByClass: { ...this.blockedByClass },
      correctionsByClass: { ...this.correctionsByClass },
      consecutiveCorrectionsByClass: { ...this.consecutiveCorrectionsByClass },
      maxConsecutiveCorrectionsByClass: { ...this.maxConsecutiveCorrectionsByClass },
      deferredDirectBlocked: this.deferredDirectBlocked,
      orchestrateBridged: this.orchestrateBridged,
      workspaceMutationDeferredBlocked: this.workspaceMutationDeferredBlocked,
    };
  }

  recordTurn(obs: ProxyTurnObservation): ProxyObservabilitySnapshot {
    this.turns += 1;
    const classes = correctionClasses(obs.blockedTools);
    for (const name of obs.blockedTools) {
      const cls = classifyProxyTool(name);
      this.blockedByClass[cls] += 1;
      if (cls === "deferredBlock") {
        this.deferredDirectBlocked += 1;
        const trimmed = name.trim();
        if (WORKSPACE_DEFERRED_TOOLS.has(trimmed)) {
          this.workspaceMutationDeferredBlocked += 1;
        }
      }
    }
    for (const name of obs.bridgedTools) {
      const trimmed = name.trim();
      if (!trimmed) {
        continue;
      }
      this.bridgedNativeByTool[trimmed] = (this.bridgedNativeByTool[trimmed] ?? 0) + 1;
      if (trimmed === "orchestrate") {
        this.orchestrateBridged += 1;
      }
    }

    const hadCorrection = obs.proxyCorrection && obs.bridgedTools.length === 0;
    if (hadCorrection) {
      this.turnsWithCorrection += 1;
      for (const cls of classes) {
        this.correctionsByClass[cls] += 1;
        const streak = this.consecutiveCorrectionsByClass[cls] + 1;
        this.consecutiveCorrectionsByClass[cls] = streak;
        if (streak > this.maxConsecutiveCorrectionsByClass[cls]) {
          this.maxConsecutiveCorrectionsByClass[cls] = streak;
        }
      }
      for (const cls of Object.keys(this.consecutiveCorrectionsByClass) as ProxyToolPolicyClass[]) {
        if (!classes.includes(cls)) {
          this.consecutiveCorrectionsByClass[cls] = 0;
        }
      }
    } else if (obs.bridgedTools.length > 0 || obs.blockedTools.length === 0) {
      this.consecutiveCorrectionsByClass = emptyClassRecord();
    }

    const snap = this.snapshot();
    emitTurnObservability(obs, snap, classes, hadCorrection);
    return snap;
  }
}

let state = new ProxyObservabilityState();
let logSink: ((line: string) => void) | null = null;

export function proxyObservabilityEnabled(): boolean {
  const v = process.env.CURSOR_API_PROXY_OBS?.trim().toLowerCase();
  return v === "1" || v === "true" || v === "yes";
}

function shouldEmitLogs(): boolean {
  return logSink !== null || proxyObservabilityEnabled();
}

function emitLog(payload: Record<string, unknown>): void {
  const line = JSON.stringify({ ts: new Date().toISOString(), ...payload });
  if (logSink) {
    logSink(line);
    return;
  }
  if (proxyObservabilityEnabled()) {
    console.error(line);
  }
}

function emitTurnObservability(
  obs: ProxyTurnObservation,
  counters: ProxyObservabilitySnapshot,
  classes: ProxyToolPolicyClass[],
  hadCorrection: boolean,
): void {
  if (!shouldEmitLogs()) {
    return;
  }
  emitLog({
    event: "proxy_turn",
    stream: obs.stream,
    bridged: obs.bridgedTools,
    blocked: obs.blockedTools.map((name) => ({ name, class: classifyProxyTool(name) })),
    proxy_correction: hadCorrection,
    correction_classes: hadCorrection ? classes : [],
    counters: {
      turns: counters.turns,
      turns_with_correction: counters.turnsWithCorrection,
      orchestrate_bridged: counters.orchestrateBridged,
      deferred_direct_blocked: counters.deferredDirectBlocked,
      workspace_mutation_deferred_blocked: counters.workspaceMutationDeferredBlocked,
      corrections_by_class: counters.correctionsByClass,
      consecutive_corrections_by_class: counters.consecutiveCorrectionsByClass,
    },
  });
  if (!hadCorrection) {
    return;
  }
  for (const cls of classes) {
    const streak = counters.consecutiveCorrectionsByClass[cls];
    if (streak > PROXY_CORRECTION_LOOP_THRESHOLD) {
      emitLog({
        event: "proxy_correction_loop",
        stream: obs.stream,
        class: cls,
        consecutive: streak,
        threshold: PROXY_CORRECTION_LOOP_THRESHOLD,
      });
    }
  }
}

export function observeProxyTurn(obs: ProxyTurnObservation): ProxyObservabilitySnapshot {
  return state.recordTurn(obs);
}

export function getProxyObservabilitySnapshot(): ProxyObservabilitySnapshot {
  return state.snapshot();
}

export function setProxyObservabilitySinkForTest(sink: ((line: string) => void) | null): void {
  logSink = sink;
}

export function resetProxyObservabilityForTest(): void {
  state = new ProxyObservabilityState();
  logSink = null;
}
