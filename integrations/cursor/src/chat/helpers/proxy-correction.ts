import {
  correctionHintForBlockedTool,
  isHardDenyBlockedLabel,
  shouldRedirectCursorTool,
} from "../../tool-policy.js";

const ORCHESTRATE_FOOTER =
  "Cursor built-ins are disabled. Use native tool_calls only: searchTools (discover deferred SDK signatures), orchestrate (run workspace scripts), searchSkill and loadSkill (skills).";

export function proxyToolCorrectionMessage(
  blocked: string[],
  allowedNames: Set<string> | null,
): string {
  void allowedNames;
  const unique = [...new Set(blocked.map((n) => n.trim()).filter(Boolean))];
  if (unique.length === 0) {
    return "";
  }
  const parts: string[] = [`Blocked by Solomon proxy: ${unique.join(", ")}.`];
  const hints: string[] = [];
  for (const name of unique) {
    const hint = correctionHintForBlockedTool(name);
    if (hint) {
      hints.push(hint);
    }
  }
  if (hints.length > 0) {
    parts.push(hints.join(" "));
  }
  if (unique.some((n) => !isHardDenyBlockedLabel(n) && (shouldRedirectCursorTool(n) || n.startsWith("mcp:")))) {
    parts.push(ORCHESTRATE_FOOTER);
  }
  parts.push("Reply with a corrected invocation or plain text.");
  return parts.join(" ");
}
