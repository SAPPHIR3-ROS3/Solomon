import type { SDKMessage } from "@cursor/sdk";
import {
  type CursorNativeToolEvent,
  unmappedToolEvent,
  unmappedToolEventFromToolCall,
} from "../../cursor-native-tools.js";
import {
  bridgeToolInvocation,
  tryCollectBridgedTool,
  unwrapCustomUserToolsMcpCall,
  unwrapSolomonMcpCall,
  type BridgedToolInvocation,
  type BridgedToolContext,
} from "../../legacy.js";
import {
  BLOCKED_MCP_EXTERNAL_LABEL,
  blockedMcpToolLabel,
  shouldHardDenyCursorTool,
  shouldRedirectCursorTool,
  shouldStopProxyOnBlockedTool,
} from "../../tool-policy.js";

function wouldBridgeTool(name: string, rawArgs: unknown, bridgeCtx: BridgedToolContext): boolean {
  const solomonMcp = unwrapSolomonMcpCall(name, rawArgs);
  if (solomonMcp) {
    return bridgeToolInvocation(solomonMcp.toolName, solomonMcp.args, bridgeCtx) !== null;
  }
  const customMcp = unwrapCustomUserToolsMcpCall(name, rawArgs);
  if (customMcp) {
    return bridgeToolInvocation(customMcp.toolName, customMcp.args, bridgeCtx) !== null;
  }
  return bridgeToolInvocation(name, rawArgs, bridgeCtx) !== null;
}

function collectBridgedMcpCall(
  pendingBridged: BridgedToolInvocation[],
  bridgeCtx: BridgedToolContext,
  onToolDetected: () => void,
  onBlockedTool: ((name: string) => void) | undefined,
  mcp: { toolName: string; args: unknown },
): boolean {
  if (tryCollectBridgedTool(pendingBridged, mcp.toolName, mcp.args, bridgeCtx)) {
    onToolDetected();
    return true;
  }
  const label = blockedMcpToolLabel(mcp.toolName);
  if (onBlockedTool) {
    onBlockedTool(label);
  }
  if (shouldStopProxyOnBlockedTool(label)) {
    onToolDetected();
  }
  return false;
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
    if (shouldStopProxyOnBlockedTool(name)) {
      onToolDetected();
    }
  };
  const handleToolProposal = (name: string, rawArgs: unknown, callId?: string): void => {
    if (shouldHardDenyCursorTool(name)) {
      reportBlocked(name);
      return;
    }
    if (shouldRedirectCursorTool(name)) {
      reportBlocked(name);
      return;
    }
    const solomonMcp = unwrapSolomonMcpCall(name, rawArgs);
    if (solomonMcp) {
      collectBridgedMcpCall(pendingBridged, bridgeCtx, onToolDetected, onBlockedTool, solomonMcp);
      return;
    }
    const customMcp = unwrapCustomUserToolsMcpCall(name, rawArgs);
    if (customMcp) {
      collectBridgedMcpCall(pendingBridged, bridgeCtx, onToolDetected, onBlockedTool, customMcp);
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
    if (event.status === "completed" || event.status === "error") {
      if (shouldHardDenyCursorTool(event.name)) {
        reportBlocked(event.name);
        return;
      }
      if (allowCursorInternalTools && onUnmappedToolEvent) {
        onUnmappedToolEvent(unmappedToolEventFromToolCall(event));
      }
      return;
    }
    const mcp = unwrapSolomonMcpCall(event.name, event.args);
    const enforcePolicy =
      !allowCursorInternalTools ||
      shouldHardDenyCursorTool(event.name) ||
      shouldRedirectCursorTool(event.name) ||
      mcp !== null ||
      event.name === "mcp";
    if (enforcePolicy) {
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
