import type { ChatMessage } from "./chatTypes";

const internalCorrections = new Set([
  "Your previous reply contained a malformed tool-invocation block. Preferred shape:\n<tool_calls>\n<tool name=\"TOOL_NAME\">\n<intent>brief purpose</intent>\n<args>{\"key\":\"value\"}</args>\n</tool>\n</tool_calls>\nAlso accepted: <tool_call>{\"name\":\"TOOL_NAME\",\"arguments\":{...}}</tool_call> or <functioncall>{\"name\":\"TOOL_NAME\",\"arguments\":{...}}</functioncall> with valid JSON. Close <tool name=\"...\"> with </tool>, not </tool_call>. Send a corrected block only, or continue without tools if you meant plain text.",
  "Your previous tool call was rejected because every Solomon tool call requires a non-empty string intent. For native API tool_calls, include an intent string in the JSON arguments. For legacy XML, include <intent>brief purpose</intent> or an intent string inside the JSON arguments. Retry the corrected tool call only, or continue without tools if you meant plain text.",
  "Your previous native API tool call was rejected. Every native tool call requires a non-empty string intent. Retry with a native Solomon tool_call whose JSON arguments match the declared schema and include a non-empty intent string. Do not emit XML, markdown pseudo-calls, or plain-text tool narration unless you mean to answer without tools.",
  "Your previous reply did not include valid native API tool_calls. Emit native Solomon tools only (orchestrate, searchTools, subagent, switchMode, searchSkill, loadSkill) via API tool_calls with JSON arguments that match each tool schema, including a non-empty intent string — not <tool_calls> XML or plain-text tool narration. For workspace and MCP work, call searchTools if unsure, then orchestrate (package main, import \"sdk\"), using sdk.mcp.<tool>(intent, args) for connected MCP tools — never emit deferred tools or MCP.* tools as direct native tool_calls. Send a corrected invocation or continue without tools if you meant plain text.",
  "Native API tool_calls are disabled because legacy tools force is ON. Do not use function calling. Emit tool invocations only inside a <tool_calls> XML block as described in the system prompt."
]);

export function isInternalToolCorrection(message: ChatMessage): boolean {
  return message.role === "user" && internalCorrections.has(message.content.trim());
}
