export type BridgedToolInvocation = {
  name: string;
  args: Record<string, unknown>;
  intent?: string;
};

export type BridgedToolContext = {
  allowedNames: Set<string> | null;
};

export const SOLOMON_MCP_PROVIDER = "solomon";

export const CUSTOM_USER_TOOLS_MCP_PROVIDER = "custom-user-tools";

export function unwrapSolomonMcpCall(
  eventName: string,
  rawArgs: unknown,
): { toolName: string; args: unknown } | null {
  if (eventName !== "mcp") {
    return null;
  }
  if (!rawArgs || typeof rawArgs !== "object") {
    return null;
  }
  const obj = rawArgs as Record<string, unknown>;
  if (obj.providerIdentifier !== SOLOMON_MCP_PROVIDER) {
    return null;
  }
  const toolName = typeof obj.toolName === "string" ? obj.toolName.trim() : "";
  if (!toolName) {
    return null;
  }
  return { toolName, args: obj.args ?? {} };
}

export function unwrapCustomUserToolsMcpCall(
  eventName: string,
  rawArgs: unknown,
): { toolName: string; args: unknown } | null {
  if (eventName !== "mcp") {
    return null;
  }
  if (!rawArgs || typeof rawArgs !== "object") {
    return null;
  }
  const obj = rawArgs as Record<string, unknown>;
  if (obj.providerIdentifier !== CUSTOM_USER_TOOLS_MCP_PROVIDER) {
    return null;
  }
  const toolName = typeof obj.toolName === "string" ? obj.toolName.trim() : "";
  if (!toolName) {
    return null;
  }
  return { toolName, args: obj.args ?? {} };
}
