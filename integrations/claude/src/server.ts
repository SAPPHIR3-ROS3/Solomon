import { createServer, type IncomingMessage, type ServerResponse } from "node:http";
import { timingSafeEqual } from "node:crypto";
import { normalizeMessages, type ChatMessage } from "./history.ts";

export type ToolDefinition = { type?: string; function?: { name?: string; description?: string; parameters?: unknown } };
export type Completion = { content: string; toolCalls: Array<{ id: string; name: string; arguments: string }>; finishReason: string; usage: { promptTokens: number; completionTokens: number } };
export type Runner = {
  models(): Promise<string[]>;
  modelCatalog?(): Promise<Array<{ id: string; supported_reasoning_levels?: string[] }>>;
  complete(input: { model: string; messages: ChatMessage[]; tools: ToolDefinition[]; reasoningEffort?: string }, onDelta: (text: string) => void, signal: AbortSignal): Promise<Completion>;
};

type CompletionRequest = { reasoning_effort?: string; model?: string; messages?: ChatMessage[]; tools?: ToolDefinition[]; stream?: boolean };

function authorized(header: string | undefined, token: string): boolean {
  const value = Buffer.from(header || "");
  const expected = Buffer.from("Bearer " + token);
  return value.length === expected.length && timingSafeEqual(value, expected);
}

function send(res: ServerResponse, status: number, body: unknown) {
  const payload = JSON.stringify(body);
  res.writeHead(status, { "content-type": "application/json", "content-length": Buffer.byteLength(payload) });
  res.end(payload);
}

function writeDelta(res: ServerResponse, model: string, delta: unknown, finish: string | null) {
  res.write("data: " + JSON.stringify({ id: "chatcmpl-claude", object: "chat.completion.chunk", created: Math.floor(Date.now() / 1000), model, choices: [{ index: 0, delta, finish_reason: finish }] }) + "\n\n");
}

async function readBody(req: IncomingMessage): Promise<string> {
  const chunks: Buffer[] = [];
  let size = 0;
  for await (const chunk of req) {
    const buffer = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk);
    size += buffer.length;
    if (size > 2_000_000) throw new Error("request is too large");
    chunks.push(buffer);
  }
  return Buffer.concat(chunks).toString("utf8");
}

// Requests run concurrently: the runner gives each conversation its own session, so a title or
// subagent request never waits behind, or replaces, a chat that is mid tool call.
export function createSidecar(token: string, runner: Runner) {
  return createServer((req, res) => {
    void handle(req, res, token, runner);
  });
}

async function handle(req: IncomingMessage, res: ServerResponse, token: string, runner: Runner) {
  try {
    if (!authorized(req.headers.authorization, token)) return send(res, 401, { error: { message: "unauthorized" } });
    const url = new URL(req.url || "/", "http://127.0.0.1");
    if (req.method === "GET" && url.pathname === "/health") return send(res, 200, { ok: true });
    if (req.method === "GET" && (url.pathname === "/v1/models" || url.pathname === "/models")) {
      const ids = await runner.models();
      return send(res, 200, { object: "list", data: runner.modelCatalog ? await runner.modelCatalog() : ids.map((id) => ({ id, object: "model" })) });
    }
    if (req.method !== "POST" || (url.pathname !== "/v1/chat/completions" && url.pathname !== "/chat/completions")) return send(res, 404, { error: { message: "not found" } });
    const request = JSON.parse(await readBody(req)) as CompletionRequest;
    const controller = new AbortController();
    req.on("aborted", () => controller.abort());
    let streamed = false;
    const completion = await runner.complete({ model: request.model || "", reasoningEffort: request.reasoning_effort, messages: normalizeMessages(request.messages || []), tools: request.tools || [] }, (text) => {
      if (!request.stream || !text) return;
      if (!streamed) {
        res.writeHead(200, { "content-type": "text/event-stream", "cache-control": "no-cache" });
        streamed = true;
      }
      writeDelta(res, request.model || "claude", { content: text }, null);
    }, controller.signal);
    const message = { role: "assistant", content: completion.content, tool_calls: completion.toolCalls.map((call) => ({ id: call.id, type: "function", function: { name: call.name, arguments: call.arguments } })) };
    if (!request.stream) return send(res, 200, { id: "chatcmpl-claude", object: "chat.completion", model: request.model || "claude", choices: [{ index: 0, message, finish_reason: completion.finishReason }], usage: { prompt_tokens: completion.usage.promptTokens, completion_tokens: completion.usage.completionTokens, total_tokens: completion.usage.promptTokens + completion.usage.completionTokens } });
    if (!streamed) res.writeHead(200, { "content-type": "text/event-stream", "cache-control": "no-cache" });
    if (completion.toolCalls.length) writeDelta(res, request.model || "claude", { tool_calls: completion.toolCalls.map((call, index) => ({ index, id: call.id, type: "function", function: { name: call.name, arguments: call.arguments } })) }, null);
    writeDelta(res, request.model || "claude", {}, completion.finishReason);
    res.end("data: [DONE]\n\n");
  } catch (error) {
    const message = error instanceof Error ? error.message : "Claude sidecar request failed";
    const status = message.includes("diverged") || message.includes("cannot import") || message.includes("unsupported content") ? 409 : message.includes("at capacity") ? 503 : 502;
    if (!res.headersSent) send(res, status, { error: { message } });
    else res.end();
  }
}
