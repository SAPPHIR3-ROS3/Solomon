import {
  correctionHintForBlockedTool,
  isHardDenyBlockedLabel,
  shouldHardDenyCursorTool,
  isChatToolSurface,
  isExposedNativePolicyException,
  shouldBlockDeferredSolomonTool,
  shouldRedirectCursorTool,
} from "../../tool-policy.js";

export function proxyToolCorrectionMessage(
  blocked: string[],
  allowedNames: Set<string> | null,
  surfaceNames: Set<string> | null = allowedNames,
): string {
  const unique = [...new Set(blocked.map((n) => n.trim()).filter(Boolean))];
  if (unique.length === 0) return "";
  const chatSurface = isChatToolSurface(surfaceNames);
  const catalog = allowedNames ?? new Set<string>();
  const parts: string[] = [`Blocked by Solomon proxy: ${unique.join(", ")}.`];
  for (const name of unique) {
    const hint = correctionHintForBlockedTool(name, chatSurface, catalog);
    if (hint) parts.push(hint);
  }
  if (unique.some((n) => !isHardDenyBlockedLabel(n) &&
      (shouldRedirectCursorTool(n) || shouldBlockDeferredSolomonTool(n) || n.startsWith("mcp:")))) {
    const available = [...catalog]
      .filter((n) => !shouldHardDenyCursorTool(n))
      .filter((n) => !shouldBlockDeferredSolomonTool(n) || isExposedNativePolicyException(n, allowedNames, surfaceNames))
      .filter((n) => !shouldRedirectCursorTool(n) || isExposedNativePolicyException(n, allowedNames, surfaceNames))
      .sort();
    parts.push((chatSurface ? "This is CHAT mode. " : "Cursor built-ins are disabled. ") +
      (available.length > 0 ? `Use native tool_calls only from this request: ${available.join(", ")}.`
        : "No native tools are available in this request; continue in plain text."));
  }
  parts.push("Reply with a corrected invocation or plain text.");
  return parts.join(" ");
}
