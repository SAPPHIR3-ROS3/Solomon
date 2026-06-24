import type { SDKCustomTool, SDKCustomToolContext, SDKCustomToolResult } from "@cursor/sdk";
import { openAIToolsToMcpTools } from "./openai-tools.js";
import type { ChatCompletionTool } from "./openai-types.js";

export const SOLOMON_CUSTOM_TOOL_DELEGATE_MSG =
  "Solomon host owns execution — this callback must not run workspace work in Node.";

export function solomonCustomToolPrehookExecute(
  _args: Record<string, unknown>,
  _ctx: SDKCustomToolContext,
): SDKCustomToolResult {
  return {
    content: [{ type: "text", text: SOLOMON_CUSTOM_TOOL_DELEGATE_MSG }],
    isError: true,
  };
}

export function solomonCustomToolsFromOpenAI(
  tools: ChatCompletionTool[] | undefined,
): Record<string, SDKCustomTool> | undefined {
  const defs = openAIToolsToMcpTools(tools);
  if (defs.length === 0) {
    return undefined;
  }
  const out: Record<string, SDKCustomTool> = {};
  for (const def of defs) {
    out[def.name] = {
      description: def.description,
      inputSchema: def.inputSchema,
      execute: solomonCustomToolPrehookExecute,
    };
  }
  return out;
}
