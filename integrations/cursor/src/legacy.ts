export type { BridgedToolContext, BridgedToolInvocation } from "./bridge/context.js";
export { SOLOMON_MCP_PROVIDER, CUSTOM_USER_TOOLS_MCP_PROVIDER, unwrapSolomonMcpCall, unwrapCustomUserToolsMcpCall } from "./bridge/context.js";
export { formatBridgedToolCallsBlock } from "./bridge/xml.js";
export {
  bridgeToolInvocation,
  collectBridgedTool,
  mapCursorToolInvocation,
  tryCollectBridgedTool,
} from "./bridge/invocation.js";
export { DEFAULT_SUBAGENT_SYS_PATH } from "./legacy-normalize.js";
