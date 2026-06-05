import { randomBytes } from "node:crypto";
import type { ServerResponse } from "node:http";
import type { LegacyToolInvocation } from "./legacy.js";
import { chunkDelta, writeSSE } from "./openai-sse.js";
import type { ChatCompletionTool, ChatToolCall, ToolChoice } from "./openai-types.js";

export function allowedToolNamesFromRequest(
  tools: ChatCompletionTool[] | undefined,
  toolChoice?: ToolChoice,
): Set<string> | null {
  if (!tools?.length) {
    return null;
  }
  if (toolChoice === "none") {
    return new Set();
  }
  const names = new Set<string>();
  for (const t of tools) {
    const n = t.function?.name?.trim();
    if (n) {
      names.add(n);
    }
  }
  if (names.size === 0) {
    return null;
  }
  if (typeof toolChoice === "object") {
    const chosen = toolChoice.function.name.trim();
    return names.has(chosen) ? new Set([chosen]) : new Set();
  }
  return names;
}

export function requestUsesNativeTools(
  tools: ChatCompletionTool[] | undefined,
  toolChoice?: ToolChoice,
): boolean {
  return (tools?.length ?? 0) > 0 && toolChoice !== "none";
}

export function filterInvocations(
  invs: LegacyToolInvocation[],
  allowed: Set<string> | null,
): LegacyToolInvocation[] {
  const valid = invs.filter(isValidInvocation);
  if (!allowed) {
    return valid;
  }
  if (allowed.size === 0) {
    return [];
  }
  return valid.filter((inv) => allowed.has(inv.name));
}

export function limitInvocations(
  invs: LegacyToolInvocation[],
  parallelToolCalls: boolean | undefined,
): LegacyToolInvocation[] {
  if (parallelToolCalls === false) {
    return invs.slice(0, 1);
  }
  return invs;
}

export function newToolCallId(): string {
  return `call_${randomBytes(12).toString("hex")}`;
}

export function toolArgumentsJSON(inv: LegacyToolInvocation): string {
  const args = { ...(inv.args ?? {}) };
  if (inv.intent && String(inv.intent).trim() !== "") {
    args.intent = String(inv.intent).trim();
  }
  return JSON.stringify(args);
}

export function isValidInvocation(inv: LegacyToolInvocation): boolean {
  if (inv.name !== "editFile") {
    return true;
  }
  if (inv.args?.delete === true) {
    const path = typeof inv.args?.path === "string" ? inv.args.path.trim() : "";
    return path !== "";
  }
  const oldString = typeof inv.args?.oldString === "string" ? inv.args.oldString : "";
  const newString = typeof inv.args?.newString === "string" ? inv.args.newString : "";
  return oldString !== "" || newString !== "";
}

export function openAIToolCallsFromInvocations(invs: LegacyToolInvocation[]): ChatToolCall[] {
  const out: ChatToolCall[] = [];
  for (const inv of invs) {
    out.push({
      id: newToolCallId(),
      type: "function",
      function: {
        name: inv.name,
        arguments: toolArgumentsJSON(inv),
      },
    });
  }
  return out;
}

export function writeSSEToolCalls(
  res: ServerResponse,
  completionId: string,
  model: string,
  invs: LegacyToolInvocation[],
): void {
  if (invs.length === 0) {
    return;
  }
  const toolCalls = invs.map((inv, index) => ({
    index,
    id: newToolCallId(),
    type: "function",
    function: {
      name: inv.name,
      arguments: toolArgumentsJSON(inv),
    },
  }));
  writeSSE(res, chunkDelta(completionId, model, { tool_calls: toolCalls }));
}

export type McpToolDefinition = {
  name: string;
  description: string;
  inputSchema: Record<string, unknown>;
};

export function openAIToolsToMcpTools(tools: ChatCompletionTool[] | undefined): McpToolDefinition[] {
  if (!tools?.length) {
    return [];
  }
  const out: McpToolDefinition[] = [];
  const seen = new Set<string>();
  for (const t of tools) {
    const name = t.function?.name?.trim();
    if (!name || seen.has(name)) {
      continue;
    }
    seen.add(name);
    const params = t.function?.parameters;
    const inputSchema =
      params && typeof params === "object" && !Array.isArray(params)
        ? { ...(params as Record<string, unknown>) }
        : { type: "object", properties: {} };
    if (typeof inputSchema.type !== "string") {
      inputSchema.type = "object";
    }
    out.push({
      name,
      description: t.function?.description?.trim() ?? "",
      inputSchema,
    });
  }
  return out;
}

export type ParsedToolText = {
  content: string;
  invocations: LegacyToolInvocation[];
};

export function parseToolInvocationsFromText(text: string): ParsedToolText {
  const invocations: LegacyToolInvocation[] = [];
  let content = text;
  content = content.replace(/<tool_calls\b[^>]*>([\s\S]*?)<\/tool_calls>/gi, (_m, inner) => {
    invocations.push(...parseSolomonTools(String(inner)));
    return "";
  });
  content = content.replace(/<tool_call\b[^>]*>([\s\S]*?)<\/tool_call>/gi, (_m, inner) => {
    const inv = parseJSONToolCall(String(inner));
    if (inv) {
      invocations.push(inv);
    }
    return "";
  });
  content = content.replace(/<functioncall\b[^>]*>([\s\S]*?)<\/functioncall>/gi, (_m, inner) => {
    const inv = parseJSONToolCall(String(inner));
    if (inv) {
      invocations.push(inv);
    }
    return "";
  });
  content = stripEmptyToolCodeFences(content);
  return { content: content.trim(), invocations };
}

function stripEmptyToolCodeFences(text: string): string {
  return text.replace(/```(?:xml|tool_calls|tool_call|functioncall)?[ \t]*\r?\n[\s\r\n]*```/gi, "");
}

function parseSolomonTools(inner: string): LegacyToolInvocation[] {
  const out: LegacyToolInvocation[] = [];
  const re = /<tool\b[^>]*\bname\s*=\s*(["'])(.*?)\1[^>]*>([\s\S]*?)<\/tool>/gi;
  for (let m: RegExpExecArray | null; (m = re.exec(inner)); ) {
    const name = unescapeXML(m[2]).trim();
    if (!name) {
      continue;
    }
    const body = m[3];
    const argsRaw = tagText(body, "args") ?? "{}";
    const args = parseJSONObject(unescapeXML(argsRaw));
    if (!args) {
      continue;
    }
    const intent = tagText(body, "intent");
    out.push({
      name,
      args,
      ...(intent ? { intent: unescapeXML(intent).trim() } : {}),
    });
  }
  return out;
}

function parseJSONToolCall(raw: string): LegacyToolInvocation | null {
  const obj = parseJSONObject(unescapeXML(raw));
  if (!obj) {
    return null;
  }
  const name =
    pickString(obj, "name") ??
    pickString(obj, "tool") ??
    pickString(obj, "tool_name") ??
    pickString(obj, "function");
  if (!name) {
    return null;
  }
  const argsRaw = obj.arguments ?? obj.args ?? obj.parameters ?? {};
  const args = normalizeArgsObject(argsRaw);
  if (!args) {
    return null;
  }
  return { name, args };
}

function tagText(body: string, tag: string): string | null {
  const re = new RegExp(`<${tag}\\b[^>]*>([\\s\\S]*?)<\\/${tag}>`, "i");
  const m = re.exec(body);
  return m ? m[1] : null;
}

function parseJSONObject(raw: string): Record<string, unknown> | null {
  try {
    const v = JSON.parse(raw.trim());
    if (v && typeof v === "object" && !Array.isArray(v)) {
      return v as Record<string, unknown>;
    }
  } catch {
    return null;
  }
  return null;
}

function normalizeArgsObject(v: unknown): Record<string, unknown> | null {
  if (v === undefined || v === null) {
    return {};
  }
  if (typeof v === "string") {
    return parseJSONObject(v);
  }
  if (typeof v === "object" && !Array.isArray(v)) {
    return v as Record<string, unknown>;
  }
  return null;
}

function pickString(obj: Record<string, unknown>, key: string): string | undefined {
  const v = obj[key];
  return typeof v === "string" && v.trim() !== "" ? v.trim() : undefined;
}

function unescapeXML(s: string): string {
  return s
    .replace(/&quot;/g, '"')
    .replace(/&apos;/g, "'")
    .replace(/&gt;/g, ">")
    .replace(/&lt;/g, "<")
    .replace(/&amp;/g, "&");
}
