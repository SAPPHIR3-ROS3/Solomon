import http from "node:http";
import type { IncomingMessage, ServerResponse } from "node:http";
import { handleChatCompletions, listAllModels, listModels, type ProxyConfig } from "./chat.js";
import type { ChatCompletionRequest } from "./openai-types.js";

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
    const body = await readBody(req);
    let parsed: ChatCompletionRequest;
    try {
      parsed = JSON.parse(body) as ChatCompletionRequest;
    } catch {
      sendError(res, 400, "invalid JSON body");
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
    req.on("data", (c) => chunks.push(c));
    req.on("end", () => resolve(Buffer.concat(chunks).toString("utf8")));
    req.on("error", reject);
  });
}

function sendError(res: ServerResponse, code: number, message: string): void {
  res.writeHead(code, { "Content-Type": "application/json" });
  res.end(JSON.stringify({ error: { message, type: "proxy_error" } }));
}
