import { normalizeSolomonToolArgs } from "../legacy-normalize.js";
import {
  isValidSolomonToolName,
  resolveBridgedSolomonName,
  shouldBlockDeferredSolomonTool,
  shouldHardDenyCursorTool,
  shouldRedirectCursorTool,
} from "../tool-policy.js";
import type { BridgedToolContext, BridgedToolInvocation } from "./context.js";

function isAllowedSolomonTool(name: string, ctx: BridgedToolContext): boolean {
  if (!ctx.allowedNames) {
    return true;
  }
  return ctx.allowedNames.has(name);
}

export function mapCursorToolInvocation(
  eventName: string,
  rawArgs: unknown,
  ctx: BridgedToolContext,
): BridgedToolInvocation | null {
  const trimmed = eventName.trim();
  if (!trimmed) {
    return null;
  }
  const solomonName = resolveBridgedSolomonName(trimmed, ctx.allowedNames);
  if (!solomonName) {
    return null;
  }
  if (!isValidSolomonToolName(solomonName)) {
    return null;
  }
  if (!isAllowedSolomonTool(solomonName, ctx)) {
    return null;
  }
  const args = normalizeSolomonToolArgs(solomonName, trimmed, rawArgs);
  if (!args) {
    return null;
  }
  return invocationWithIntent(solomonName, args);
}

export function bridgeToolInvocation(
  eventName: string,
  rawArgs: unknown,
  ctx: BridgedToolContext,
): BridgedToolInvocation | null {
  const trimmed = eventName.trim();
  if (!trimmed) {
    return null;
  }
  if (shouldHardDenyCursorTool(trimmed)) {
    return null;
  }
  if (shouldRedirectCursorTool(trimmed)) {
    return null;
  }
  const mapped = mapCursorToolInvocation(eventName, rawArgs, ctx);
  if (!mapped) {
    return null;
  }
  if (shouldBlockDeferredSolomonTool(mapped.name)) {
    return null;
  }
  return mapped;
}

export function collectBridgedTool(
  pending: BridgedToolInvocation[],
  name: string,
  rawArgs: unknown,
  ctx: BridgedToolContext,
): void {
  const inv = bridgeToolInvocation(name, rawArgs, ctx);
  if (inv) {
    pending.push(inv);
  }
}

export function tryCollectBridgedTool(
  pending: BridgedToolInvocation[],
  name: string,
  rawArgs: unknown,
  ctx: BridgedToolContext,
): boolean {
  const before = pending.length;
  collectBridgedTool(pending, name, rawArgs, ctx);
  return pending.length > before;
}

function invocationWithIntent(
  solomonName: string,
  args: Record<string, unknown>,
): BridgedToolInvocation {
  const intent =
    typeof args.intent === "string"
      ? args.intent
      : typeof (args as { description?: string }).description === "string"
        ? (args as { description: string }).description
        : undefined;
  if (intent !== undefined) {
    delete args.intent;
    delete (args as { description?: string }).description;
  }
  return { name: solomonName, args, ...(intent ? { intent } : {}) };
}
