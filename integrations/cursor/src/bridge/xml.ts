import type { BridgedToolInvocation } from "./context.js";
import { escapeXmlAttr, escapeXmlText } from "../xml-utils.js";
import { invocationIntent } from "../tool-intent.js";

export function formatBridgedToolCallsBlock(tools: BridgedToolInvocation[]): string {
  const parts: string[] = ["<tool_calls>"];
  for (const t of tools) {
    parts.push(`<tool name="${escapeXmlAttr(t.name)}">`);
    parts.push(`<intent>${escapeXmlText(invocationIntent(t) ?? "")}</intent>`);
    parts.push(`<args>${escapeXmlText(JSON.stringify(t.args ?? {}))}</args>`);
    parts.push("</tool>");
  }
  parts.push("</tool_calls>");
  return parts.join("\n");
}
