import type { ReasoningEffort } from "./projects";

export function normalizeReasoningEffort(value: string): ReasoningEffort {
  const normalized = value.trim().toLowerCase().replaceAll("_", "-").replace(/\s+/g, "-");
  if (normalized === "med") return "medium";
  if (normalized === "x-high" || normalized === "extra-high") return "xhigh";
  if (normalized === "none" || normalized === "low" || normalized === "medium" || normalized === "high" || normalized === "xhigh" || normalized === "max") return normalized;
  return "none";
}
