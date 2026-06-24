import {
  proxyEnabledToolsLabel,
  proxyShellFallbackAllowed,
} from "../../tool-policy.js";

export function proxyToolCorrectionMessage(
  blocked: string[],
  allowedNames: Set<string> | null,
): string {
  const unique = [...new Set(blocked.map((n) => n.trim()).filter(Boolean))];
  if (unique.length === 0) {
    return "";
  }
  const enabled = proxyEnabledToolsLabel(allowedNames);
  const shellAllowed = proxyShellFallbackAllowed(allowedNames);
  const shellFallback = shellAllowed
    ? " Default fallback: call Shell again; the host maps it to shell and runs it on the workspace (include intent). "
    : " ";
  return (
    `Your previous tool call was rejected or not mappable: ${unique.join(", ")}. ` +
    "Use normal Cursor built-in tools (Read, StrReplace, Write, Grep, Glob, Shell, Delete, SemanticSearch, Task, etc.); " +
    "the Solomon host bridge intercepts them and executes on the real workspace when mapped. " +
    `Host-enabled capabilities: ${enabled}. ` +
    shellFallback +
    "Prefer Read for file inspection, StrReplace/Write for edits, and Grep/Glob for search. " +
    "For nested work use Task (mapped to subagent). For web content use fetchWeb or webSearch when available. " +
    "Send a corrected tool call only, or continue without tools if you meant plain text."
  );
}
