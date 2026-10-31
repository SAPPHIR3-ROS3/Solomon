import type { ReasoningEffort } from "./projects";

export type ReasoningCapability = { known: boolean; levels: ReasoningEffort[]; default?: ReasoningEffort };
export const classicReasoning: ReasoningEffort[] = ["none", "low", "medium", "high", "xhigh", "max"];
const ordered: ReasoningEffort[] = ["none", "minimal", "low", "medium", "high", "xhigh", "max"];
export function reasoningFromPayload(payload: unknown): Record<string, ReasoningCapability> {
 if (!payload || typeof payload !== "object") return {};
 return Object.fromEntries(Object.entries(payload).flatMap(([id, raw]) => {
 if (!raw || typeof raw !== "object") return [];
 const entry = raw as Record<string, unknown>;
 if (typeof entry.known !== "boolean" || !Array.isArray(entry.levels)) return [];
 const levels = ordered.filter((level) => entry.levels instanceof Array && entry.levels.includes(level));
 const defaultLevel = ordered.find((level) => entry.default === level);
 return [[id, { known: entry.known, levels, default: defaultLevel }]];
 }));
}
export function reasoningOptions(capability?: ReasoningCapability): ReasoningEffort[] {
 return capability?.known ? capability.levels : classicReasoning;
}
export function resolveReasoning(value: ReasoningEffort, capability?: ReasoningCapability): ReasoningEffort | undefined {
 const levels = reasoningOptions(capability);
 if (levels.includes(value)) return value;
 if (capability?.default && levels.includes(capability.default)) return capability.default;
 return levels.includes("medium") ? "medium" : levels[0];
}

export function normalizeReasoningEffort(value: string): ReasoningEffort {
  const normalized = value.trim().toLowerCase().replaceAll("_", "-").replace(/\s+/g, "-");
  if (normalized === "med") return "medium";
  if (normalized === "x-high" || normalized === "extra-high") return "xhigh";
  if (normalized === "none" || normalized === "minimal" || normalized === "low" || normalized === "medium" || normalized === "high" || normalized === "xhigh" || normalized === "max") return normalized;
  return "none";
}
