import http from "node:http";
import type { IncomingMessage, ServerResponse } from "node:http";
import { handleChatCompletions, listAllModels, listModels, type ProxyConfig } from "./chat.js";
import { sanitizeReflectedText, stripUnsafeControlChars } from "./messages.js";
import type {
  ChatCompletionRequest,
  ChatCompletionTool,
  ChatMessage,
  ChatToolCall,
  ContentPart,
} from "./openai-types.js";

const MAX_BODY_BYTES = 8 * 1024 * 1024;
const MAX_MESSAGES = 256;
const MAX_CONTENT_CHARS = 512_000;
const MAX_MODEL_CHARS = 256;
const MAX_TOOL_COUNT = 64;
const MAX_TOOL_NAME_CHARS = 128;
const MAX_IMAGE_URL_CHARS = 8192;
const ALLOWED_ROLES = new Set(["user", "assistant", "system", "tool"]);

export function createServer(cfg: ProxyConfig): http.Server {
  return http.createServer((req, res) => {
    void route(req, res, cfg).catch((err) => {
      sendError(res, 500, err instanceof Error ? err.message : String(err));
    });
  });
}

async function route(
  req: IncomingMessage,
  res: ServerResponse,
  cfg: ProxyConfig,
): Promise<void> {
  const url = new URL(req.url ?? "/", "http://127.0.0.1");
  const path = url.pathname.replace(/\/+$/, "") || "/";
  if (req.method === "GET" && (path === "/health" || path === "/v1/health")) {
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ ok: true }));
    return;
  }
  if (req.method === "GET" && (path === "/v1/models" || path === "/models")) {
    const all =
      url.searchParams.get("all") === "1" || url.searchParams.get("full") === "1";
    const ids = all ? await listAllModels(cfg.apiKey) : await listModels(cfg.apiKey);
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(
      JSON.stringify({
        object: "list",
        data: ids.map((id) => ({
          id,
          object: "model",
          created: 0,
          owned_by: "cursor",
        })),
      }),
    );
    return;
  }
  if (
    req.method === "POST" &&
    (path === "/v1/chat/completions" || path === "/chat/completions")
  ) {
    let body: string;
    try {
      body = await readBody(req);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      if (msg === "body too large") {
        sendError(res, 413, "request body too large");
        return;
      }
      throw err;
    }
    let parsed: ChatCompletionRequest;
    try {
      parsed = parseChatCompletionRequest(body);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      if (msg === "invalid JSON body") {
        sendError(res, 400, "invalid JSON body");
        return;
      }
      sendError(res, 400, "invalid request body");
      return;
    }
    await handleChatCompletions(parsed, req, res, cfg);
    return;
  }
  sendError(res, 404, "not found");
}

function readBody(req: IncomingMessage): Promise<string> {
  return new Promise((resolve, reject) => {
    const chunks: Buffer[] = [];
    let size = 0;
    req.on("data", (c) => {
      size += c.length;
      if (size > MAX_BODY_BYTES) {
        req.destroy();
        reject(new Error("body too large"));
        return;
      }
      chunks.push(c);
    });
    req.on("end", () => resolve(Buffer.concat(chunks).toString("utf8")));
    req.on("error", reject);
  });
}

function boundedString(v: unknown, max: number): string | null {
  if (typeof v !== "string") {
    return null;
  }
  const cleaned = stripUnsafeControlChars(v);
  if (cleaned.length > max) {
    return cleaned.slice(0, max);
  }
  return cleaned;
}

function optionalBoundedString(v: unknown, max: number): string | undefined {
  if (v === undefined || v === null) {
    return undefined;
  }
  return boundedString(v, max) ?? undefined;
}

function isAllowedImageUrl(url: string): boolean {
  const t = url.trim();
  const lower = t.toLowerCase();
  if (lower.startsWith("javascript:") || lower.startsWith("vbscript:")) {
    return false;
  }
  if (lower.startsWith("data:")) {
    return /^data:image\/[a-z0-9.+-]+;base64,/i.test(t);
  }
  return lower.startsWith("file://") || lower.startsWith("https://");
}

function sanitizeContentParts(v: unknown): ContentPart[] | null {
  if (!Array.isArray(v)) {
    return null;
  }
  const out: ContentPart[] = [];
  for (const part of v.slice(0, 64)) {
    if (!part || typeof part !== "object") {
      continue;
    }
    const p = part as Record<string, unknown>;
    if (p.type === "text") {
      const text = boundedString(p.text, MAX_CONTENT_CHARS);
      if (text !== null) {
        out.push({ type: "text", text });
      }
      continue;
    }
    if (p.type === "image_url") {
      const image = p.image_url;
      if (!image || typeof image !== "object") {
        continue;
      }
      const iu = image as Record<string, unknown>;
      const rawUrl = boundedString(iu.url, MAX_IMAGE_URL_CHARS);
      if (!rawUrl || !isAllowedImageUrl(rawUrl)) {
        continue;
      }
      const detail = optionalBoundedString(iu.detail, 32);
      out.push({
        type: "image_url",
        image_url: detail ? { url: rawUrl.trim(), detail } : { url: rawUrl.trim() },
      });
    }
  }
  return out.length > 0 ? out : null;
}

function sanitizeMessageContent(v: unknown): string | ContentPart[] | undefined {
  if (v === undefined || v === null) {
    return undefined;
  }
  if (typeof v === "string") {
    return boundedString(v, MAX_CONTENT_CHARS) ?? "";
  }
  return sanitizeContentParts(v) ?? undefined;
}

function sanitizeToolCalls(v: unknown): ChatToolCall[] | undefined {
  if (!Array.isArray(v)) {
    return undefined;
  }
  const out: ChatToolCall[] = [];
  for (const tc of v.slice(0, 32)) {
    if (!tc || typeof tc !== "object") {
      continue;
    }
    const t = tc as Record<string, unknown>;
    const fn = t.function;
    if (!fn || typeof fn !== "object") {
      continue;
    }
    const f = fn as Record<string, unknown>;
    const name = boundedString(f.name, MAX_TOOL_NAME_CHARS)?.trim();
    if (!name) {
      continue;
    }
    const args = boundedString(f.arguments, MAX_CONTENT_CHARS) ?? "{}";
    const id = optionalBoundedString(t.id, 128);
    out.push({
      ...(id ? { id } : {}),
      type: "function",
      function: { name, arguments: args },
    });
  }
  return out.length > 0 ? out : undefined;
}

function sanitizeMessage(v: unknown): ChatMessage | null {
  if (!v || typeof v !== "object" || Array.isArray(v)) {
    return null;
  }
  const m = v as Record<string, unknown>;
  const role = boundedString(m.role, 32)?.trim();
  if (!role || !ALLOWED_ROLES.has(role)) {
    return null;
  }
  const content = sanitizeMessageContent(m.content);
  const name = optionalBoundedString(m.name, 128);
  const toolCallId = optionalBoundedString(m.tool_call_id, 128);
  const toolCalls = sanitizeToolCalls(m.tool_calls);
  if (role === "tool" && !toolCallId) {
    return null;
  }
  const out: ChatMessage = { role, ...(content !== undefined ? { content } : {}) };
  if (name) {
    out.name = name;
  }
  if (toolCallId) {
    out.tool_call_id = toolCallId;
  }
  if (toolCalls) {
    out.tool_calls = toolCalls;
  }
  return out;
}

function sanitizeTools(v: unknown): ChatCompletionTool[] | undefined {
  if (!Array.isArray(v)) {
    return undefined;
  }
  const out: ChatCompletionTool[] = [];
  for (const tool of v.slice(0, MAX_TOOL_COUNT)) {
    if (!tool || typeof tool !== "object") {
      continue;
    }
    const t = tool as Record<string, unknown>;
    const fn = t.function;
    if (!fn || typeof fn !== "object") {
      continue;
    }
    const f = fn as Record<string, unknown>;
    const name = boundedString(f.name, MAX_TOOL_NAME_CHARS)?.trim();
    if (!name) {
      continue;
    }
    const description = optionalBoundedString(f.description, MAX_CONTENT_CHARS);
    const strict = t.strict === true || f.strict === true ? true : undefined;
    out.push({
      type: "function",
      function: {
        name,
        ...(description ? { description } : {}),
        ...(strict ? { strict } : {}),
      },
    });
  }
  return out.length > 0 ? out : undefined;
}

function parseChatCompletionRequest(body: string): ChatCompletionRequest {
  let raw: unknown;
  try {
    raw = JSON.parse(body);
  } catch {
    throw new Error("invalid JSON body");
  }
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) {
    throw new Error("invalid request body");
  }
  const o = raw as Record<string, unknown>;
  if (!Array.isArray(o.messages)) {
    throw new Error("invalid request body");
  }
  if (o.messages.length === 0 || o.messages.length > MAX_MESSAGES) {
    throw new Error("invalid request body");
  }
  const messages: ChatMessage[] = [];
  for (const m of o.messages) {
    const sm = sanitizeMessage(m);
    if (!sm) {
      throw new Error("invalid request body");
    }
    messages.push(sm);
  }
  const model = optionalBoundedString(o.model, MAX_MODEL_CHARS);
  const stream = o.stream === undefined ? undefined : o.stream === true;
  const reasoningEffort = optionalBoundedString(o.reasoning_effort, 32);
  const solomonFastMode =
    o.solomon_fast_mode === undefined ? undefined : o.solomon_fast_mode !== false;
  const tools = sanitizeTools(o.tools);
  const req: ChatCompletionRequest = { messages };
  if (model) {
    req.model = model;
  }
  if (stream !== undefined) {
    req.stream = stream;
  }
  if (reasoningEffort) {
    req.reasoning_effort = reasoningEffort;
  }
  if (solomonFastMode !== undefined) {
    req.solomon_fast_mode = solomonFastMode;
  }
  if (tools) {
    req.tools = tools;
  }
  return req;
}

function sendError(res: ServerResponse, code: number, message: string): void {
  res.writeHead(code, { "Content-Type": "application/json" });
  res.end(JSON.stringify({ error: { message: sanitizeReflectedText(message), type: "proxy_error" } }));
}
