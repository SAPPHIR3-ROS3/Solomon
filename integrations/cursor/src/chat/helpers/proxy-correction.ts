import {
  correctionHintForBlockedTool,
  isHardDenyBlockedLabel,
  shouldBlockDeferredSolomonTool,
  shouldRedirectCursorTool,
} from "../../tool-policy.js";

const ORCHESTRATE_FOOTER =
  "Cursor built-ins are disabled. Use native tool_calls only: searchTools (discover deferred SDK signatures), orchestrate (run workspace scripts), searchSkill and loadSkill (skills).";
const CHAT_FOOTER =
  "This is CHAT mode. Use native tool_calls only: docsRetrieval, webSearch, fetchWeb, deepResearch, researchStatus, or switchMode; switchMode before workspace implementation.";

function isChatSurface(allowedNames: Set<string> | null): boolean {
  if (!allowedNames || allowedNames.has("orchestrate")) {
    return false;
  }
  return ["fetchWeb", "webSearch", "deepResearch", "researchStatus"].some((name) =>
    allowedNames.has(name),
  );
}

export function proxyToolCorrectionMessage(
  blocked: string[],
  allowedNames: Set<string> | null,
): string {
  const unique = [...new Set(blocked.map((n) => n.trim()).filter(Boolean))];
  if (unique.length === 0) {
    return "";
  }
  const chatSurface = isChatSurface(allowedNames);
  const parts: string[] = [`Blocked by Solomon proxy: ${unique.join(", ")}.`];
  const hints: string[] = [];
  for (const name of unique) {
    const hint = correctionHintForBlockedTool(name, chatSurface);
    if (hint) {
      hints.push(hint);
    }
  }
  if (hints.length > 0) {
    parts.push(hints.join(" "));
  }
  if (
    unique.some(
      (n) =>
        !isHardDenyBlockedLabel(n) &&
        (shouldRedirectCursorTool(n) || shouldBlockDeferredSolomonTool(n) || n.startsWith("mcp:")),
    )
  ) {
    parts.push(chatSurface ? CHAT_FOOTER : ORCHESTRATE_FOOTER);
  }
  parts.push("Reply with a corrected invocation or plain text.");
  return parts.join(" ");
}
