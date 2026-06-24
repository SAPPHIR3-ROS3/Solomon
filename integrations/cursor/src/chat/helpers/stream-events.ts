import type { SDKMessage } from "@cursor/sdk";
import {
  type CursorNativeToolEvent,
  unmappedToolEvent,
  unmappedToolEventFromToolCall,
} from "../../cursor-native-tools.js";
import {
  bridgeToolInvocation,
  tryCollectBridgedTool,
  unwrapSolomonMcpCall,
  type BridgedToolInvocation,
  type BridgedToolContext,
} from "../../legacy.js";
import {
  BLOCKED_MCP_EXTERNAL_LABEL,
  blockedMcpToolLabel,
} from "../../tool-policy.js";

function wouldBridgeTool(name: string, rawArgs: unknown, bridgeCtx: BridgedToolContext): boolean {
  const mcp = unwrapSolomonMcpCall(name, rawArgs);
  if (mcp) {
    return bridgeToolInvocation(mcp.toolName, mcp.args, bridgeCtx) !== null;
  }
  return bridgeToolInvocation(name, rawArgs, bridgeCtx) !== null;
}

export function processStreamEvent(
  event: SDKMessage,
  allowCursorInternalTools: boolean,
  onText: (s: string) => void,
  onThinking: (s: string) => void,
  pendingBridged: BridgedToolInvocation[],
  onToolDetected: () => void,
  onBlockedTool?: (name: string) => void,
  bridgeCtx: BridgedToolContext = { allowedNames: null },
  onUnmappedToolEvent?: (event: CursorNativeToolEvent) => void,
): void {
  const reportBlocked = (name: string): void => {
    if (onBlockedTool) {
      onBlockedTool(name);
    }
  };
  const handleToolProposal = (name: string, rawArgs: unknown, callId?: string): void => {
    const mcp = unwrapSolomonMcpCall(name, rawArgs);
    if (mcp) {
      if (tryCollectBridgedTool(pendingBridged, mcp.toolName, mcp.args, bridgeCtx)) {
        onToolDetected();
      } else {
        reportBlocked(blockedMcpToolLabel(mcp.toolName));
      }
      return;
    }
    if (name === "mcp") {
      reportBlocked(BLOCKED_MCP_EXTERNAL_LABEL);
      return;
    }
    if (tryCollectBridgedTool(pendingBridged, name, rawArgs, bridgeCtx)) {
      onToolDetected();
      return;
    }
    if (!allowCursorInternalTools) {
      reportBlocked(name);
      return;
    }
    if (onUnmappedToolEvent) {
      onUnmappedToolEvent(unmappedToolEvent(name, "running", rawArgs, undefined, undefined, callId));
    }
  };
  if (event.type === "assistant") {
    let afterTool = false;
    for (const block of event.message.content) {
      if (block.type === "tool_use") {
        afterTool = true;
        handleToolProposal(block.name, block.input, block.id);
        continue;
      }
      if (block.type === "text" && block.text && !afterTool) {
        onText(block.text);
      }
    }
    return;
  }
  if (event.type === "thinking" && event.text) {
    onThinking(event.text);
    return;
  }
  if (event.type === "tool_call") {
    if (wouldBridgeTool(event.name, event.args, bridgeCtx)) {
      if (event.status === "completed" || event.status === "error") {
        return;
      }
      if (event.args !== undefined) {
        handleToolProposal(event.name, event.args, event.call_id);
      }
      return;
    }
    if (!allowCursorInternalTools) {
      if (event.status === "completed") {
        return;
      }
      if (event.args !== undefined) {
        handleToolProposal(event.name, event.args, event.call_id);
      } else {
        reportBlocked(event.name);
      }
      return;
    }
    if (onUnmappedToolEvent) {
      onUnmappedToolEvent(unmappedToolEventFromToolCall(event));
    }
    return;
  }
  if (event.type === "task" && event.text) {
    onThinking(event.text);
  }
}
