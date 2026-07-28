import { execFile } from "node:child_process";
import { readFile } from "node:fs/promises";
import { homedir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import type { Plugin } from "vite";

const modelsEndpoint = "/__solomon/models";
const currentModelEndpoint = "/__solomon/current-model";
const repositoryRoot = fileURLToPath(new URL("..", import.meta.url));
const skippedProviders = new Set(["Claude Sub"]);

let modelCatalogCache: { createdAt: number; json: string } | undefined;
let modelCatalogInFlight: Promise<string> | undefined;

function solomonHome() {
  return process.env.SOLOMON_HOME?.trim() || path.join(homedir(), ".solomon");
}

function runGoScript(scriptName: string, args: string[] = []): Promise<string> {
  return new Promise((resolve, reject) => {
    execFile(
      "go",
      ["run", path.join(repositoryRoot, "scripts", scriptName), ...args],
      { cwd: repositoryRoot, env: process.env, timeout: 70_000, maxBuffer: 2 * 1024 * 1024 },
      (error, stdout, stderr) => {
        if (error) {
          reject(new Error(stderr?.trim() || error.message));
          return;
        }
        resolve(stdout.trim());
      },
    );
  });
}

async function fullModelCatalogJSON(): Promise<string> {
  if (modelCatalogCache && Date.now() - modelCatalogCache.createdAt < 60_000) return modelCatalogCache.json;
  if (modelCatalogInFlight) return modelCatalogInFlight;
  modelCatalogInFlight = runGoScript("ui_model_catalog.go")
    .then((json) => {
      JSON.parse(json);
      modelCatalogCache = { createdAt: Date.now(), json };
      return json;
    })
    .finally(() => {
      modelCatalogInFlight = undefined;
    });
  return modelCatalogInFlight;
}

async function recentFallbackCatalog(): Promise<object> {
  const source = await readFile(path.join(solomonHome(), "config.toml"), "utf8");
  const currentProvider = source.match(/^\s*\[current\][\s\S]*?^\s*provider\s*=\s*"([^"]+)"/m)?.[1]?.trim() ?? "";
  const currentModel = source.match(/^\s*\[current\][\s\S]*?^\s*model\s*=\s*"([^"]+)"/m)?.[1]?.trim() ?? "";
  const recentBlock = source.match(/^\s*\[recent_models\]\s*\n([\s\S]*?)(?=^\s*\[|\s*$)/m)?.[1] ?? "";
  const providers: Array<{ provider: string; models: string[]; complete: boolean }> = [];
  const recent: Array<{ provider: string; model: string }> = [];
  for (const line of recentBlock.split(/\r?\n/)) {
    const match = line.match(/^\s*(?:"([^"]+)"|([A-Za-z0-9 _-]+))\s*=\s*\[(.*)\]\s*$/);
    if (!match) continue;
    const provider = (match[1] || match[2] || "").trim();
    if (!provider || skippedProviders.has(provider)) continue;
    const models = Array.from(match[3].matchAll(/"([^"]+)"/g), (entry) => entry[1]).filter(Boolean);
    if (!models.length) continue;
    providers.push({ provider, models, complete: false });
    for (const model of models) recent.push({ provider, model });
  }
  if (currentProvider && currentModel && !providers.some((entry) => entry.provider === currentProvider)) {
    providers.unshift({ provider: currentProvider, models: [currentModel], complete: false });
  }
  return { current: { provider: currentProvider, model: currentModel }, recent, providers };
}

type JsonRequest = {
  method?: string;
  on: (event: "data", listener: (chunk: string | Uint8Array) => void) => void;
  once(event: "error", listener: () => void): void;
  once(event: "end", listener: () => void): void;
};

type JsonResponse = {
  end: (body: string) => void;
  setHeader: (name: string, value: string) => void;
  statusCode: number;
};

function respond(response: JsonResponse, statusCode: number, body: object | string) {
  response.statusCode = statusCode;
  response.setHeader("Content-Type", "application/json; charset=utf-8");
  response.end(typeof body === "string" ? body : JSON.stringify(body));
}

function readJsonBody(request: JsonRequest): Promise<unknown> {
  return new Promise((resolve, reject) => {
    let body = "";
    request.on("data", (chunk) => {
      body += String(chunk);
      if (body.length > 4096) reject(new Error("Request body is too large"));
    });
    request.once("error", reject);
    request.once("end", () => {
      try {
        resolve(JSON.parse(body));
      } catch {
        reject(new Error("Invalid JSON"));
      }
    });
  });
}

function attachModelsEndpoint(server: { middlewares: { use: (route: string, handler: (request: JsonRequest, response: JsonResponse, next: () => void) => void) => void } }) {
  server.middlewares.use(modelsEndpoint, (request, response, next) => {
    if (request.method !== "GET") {
      next();
      return;
    }
    void fullModelCatalogJSON()
      .then((json) => respond(response, 200, json))
      .catch(async () => {
        try {
          respond(response, 200, await recentFallbackCatalog());
        } catch {
          respond(response, 500, { current: { provider: "", model: "" }, recent: [], providers: [] });
        }
      });
  });
}

function attachCurrentModelEndpoint(server: { middlewares: { use: (route: string, handler: (request: JsonRequest, response: JsonResponse, next: () => void) => void) => void } }) {
  server.middlewares.use(currentModelEndpoint, (request, response, next) => {
    if (request.method !== "PUT") {
      next();
      return;
    }
    void readJsonBody(request)
      .then(async (payload) => {
        if (!payload || typeof payload !== "object" || !("provider" in payload) || !("model" in payload)) {
          respond(response, 400, { error: "provider and model are required" });
          return;
        }
        const provider = typeof payload.provider === "string" ? payload.provider.trim() : "";
        const model = typeof payload.model === "string" ? payload.model.trim() : "";
        if (!provider || !model) {
          respond(response, 400, { error: "provider and model are required" });
          return;
        }
        const json = await runGoScript("ui_set_current_model.go", [provider, model]);
        modelCatalogCache = undefined;
        respond(response, 200, JSON.parse(json));
      })
      .catch((error: unknown) => respond(response, 500, { error: error instanceof Error ? error.message : "Unable to save model" }));
  });
}

export function modelsPlugin(): Plugin {
  return {
    configurePreviewServer(server) {
      attachModelsEndpoint(server);
      attachCurrentModelEndpoint(server);
    },
    configureServer(server) {
      attachModelsEndpoint(server);
      attachCurrentModelEndpoint(server);
    },
    name: "solomon-models",
  };
}
