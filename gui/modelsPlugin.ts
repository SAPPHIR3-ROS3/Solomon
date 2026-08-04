import { execFile, spawn } from "node:child_process";
import { readFile, rename, writeFile } from "node:fs/promises";
import { homedir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import type { Plugin } from "vite";

const modelsEndpoint = "/__solomon/models";
const currentModelEndpoint = "/__solomon/current-model";
const modelVisibilityEndpoint = "/__solomon/model-visibility";
const connectProviderEndpoint = "/__solomon/connect-provider";
const skippedProviders = new Set(["Claude Sub"]);
const recentModelCap = 64;
const guiRoot = fileURLToPath(new URL(".", import.meta.url));
const repositoryRoot = fileURLToPath(new URL("..", import.meta.url));
const catalogHelper = path.join(guiRoot, "desktop", "model_catalog.go");
const providerSetupHelper = path.join(guiRoot, "desktop", "provider_setup.go");

type ModelChoice = { provider: string; model: string };
type ProviderCatalog = { provider: string; models: string[]; disabled: string[]; complete: boolean };
type ModelCatalog = { current: ModelChoice; recent: ModelChoice[]; providers: ProviderCatalog[] };
type ModelVisibility = { enabled: boolean; model: string; provider: string };

let modelCatalogCache: { createdAt: number; catalog: ModelCatalog } | undefined;
let modelCatalogInFlight: Promise<ModelCatalog> | undefined;

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

function solomonHome() {
  return process.env.SOLOMON_HOME?.trim() || path.join(homedir(), ".solomon");
}

function configPath(home: string) {
  return path.join(home, "config.toml");
}

function respond(response: JsonResponse, statusCode: number, body: object) {
  response.statusCode = statusCode;
  response.setHeader("Content-Type", "application/json; charset=utf-8");
  response.end(JSON.stringify(body));
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

function unquoteTomlKey(raw: string): string {
  const value = raw.trim();
  if ((value.startsWith("'") && value.endsWith("'")) || (value.startsWith('"') && value.endsWith('"'))) {
    return value.slice(1, -1);
  }
  return value;
}

function quoteTomlKey(value: string): string {
  if (/^[A-Za-z0-9_-]+$/.test(value)) return value;
  return `'${value.replaceAll("'", "''")}'`;
}

function parseQuotedString(raw: string): string {
  const value = raw.trim();
  if (value.startsWith("'")) return value.slice(1, -1).replaceAll("''", "'");
  if (value.startsWith('"')) {
    try {
      return String(JSON.parse(value));
    } catch {
      return value.slice(1, -1);
    }
  }
  return value;
}

function sectionBody(source: string, header: RegExp): string {
  const match = source.match(header);
  if (!match || match.index === undefined) return "";
  const start = match.index + match[0].length;
  const rest = source.slice(start);
  const next = rest.search(/^\s*\[/m);
  return next === -1 ? rest : rest.slice(0, next);
}

function listProviderNames(source: string): string[] {
  const names: string[] = [];
  const seen = new Set<string>();
  for (const match of source.matchAll(/^\s*\[providers\.(.+?)\]\s*$/gm)) {
    const name = unquoteTomlKey(match[1]);
    if (!name || skippedProviders.has(name) || seen.has(name)) continue;
    seen.add(name);
    names.push(name);
  }
  return names;
}

function parseRecentModels(source: string): Map<string, string[]> {
  const body = sectionBody(source, /^\s*\[recent_models\]\s*$/m);
  const recent = new Map<string, string[]>();
  for (const line of body.split(/\r?\n/)) {
    const match = line.match(/^\s*(?:"([^"]+)"|'((?:[^']|'')*)'|([A-Za-z0-9_-]+))\s*=\s*\[(.*)\]\s*(?:#.*)?$/);
    if (!match) continue;
    const provider = (match[1] || (match[2] ? match[2].replaceAll("''", "'") : "") || match[3] || "").trim();
    if (!provider || skippedProviders.has(provider)) continue;
    const models = Array.from(match[4].matchAll(/"([^"]+)"|'((?:[^']|'')*)'/g), (entry) => (entry[1] || entry[2]?.replaceAll("''", "'") || "").trim()).filter(Boolean);
    if (models.length) recent.set(provider, models);
  }
  return recent;
}

function parseHiddenModels(source: string): Map<string, string[]> {
  const body = sectionBody(source, /^\s*\[hidden_models\]\s*$/m);
  const hidden = new Map<string, string[]>();
  for (const line of body.split(/\r?\n/)) {
    const match = line.match(/^\s*(?:"([^"]+)"|'((?:[^']|'')*)'|([A-Za-z0-9_-]+))\s*=\s*\[(.*)\]\s*(?:#.*)?$/);
    if (!match) continue;
    const provider = (match[1] || (match[2] ? match[2].replaceAll("''", "'") : "") || match[3] || "").trim();
    if (!provider || skippedProviders.has(provider)) continue;
    const models = Array.from(match[4].matchAll(/"([^"]+)"|'((?:[^']|'')*)'/g), (entry) => (entry[1] || entry[2]?.replaceAll("''", "'") || "").trim()).filter(Boolean);
    if (models.length) hidden.set(provider, models);
  }
  return hidden;
}

function parseCurrent(source: string): ModelChoice {
  const body = sectionBody(source, /^\s*\[current\]\s*$/m);
  const provider = body.match(/^\s*provider\s*=\s*((?:"(?:[^"\\]|\\.)*")|(?:'(?:[^']|'')*')|[^\s#]+)\s*(?:#.*)?$/m);
  const model = body.match(/^\s*model\s*=\s*((?:"(?:[^"\\]|\\.)*")|(?:'(?:[^']|'')*')|[^\s#]+)\s*(?:#.*)?$/m);
  return {
    provider: provider ? parseQuotedString(provider[1]).trim() : "",
    model: model ? parseQuotedString(model[1]).trim() : "",
  };
}

function uniqueModels(ids: string[], first = ""): string[] {
  const out: string[] = [];
  const seen = new Set<string>();
  for (const id of [first, ...ids]) {
    const model = id.trim();
    if (!model || seen.has(model)) continue;
    seen.add(model);
    out.push(model);
  }
  return out;
}

async function readRecentModelCatalog(home: string): Promise<ModelCatalog> {
  const source = await readFile(configPath(home), "utf8");
  const current = parseCurrent(source);
  const recentMap = parseRecentModels(source);
  const hiddenMap = parseHiddenModels(source);
  const providerNames = listProviderNames(source);
  for (const name of recentMap.keys()) {
    if (!providerNames.includes(name) && !skippedProviders.has(name)) providerNames.push(name);
  }
  if (current.provider && !providerNames.includes(current.provider) && !skippedProviders.has(current.provider)) {
    providerNames.unshift(current.provider);
  }
  const providers: ProviderCatalog[] = providerNames.map((provider) => {
    const models = uniqueModels(recentMap.get(provider) ?? [], provider === current.provider ? current.model : "");
    return { provider, models, disabled: hiddenMap.get(provider) ?? [], complete: false };
  });
  const recent: ModelChoice[] = [];
  const prefer = current.provider;
  const ordered = prefer ? [prefer, ...providerNames.filter((name) => name !== prefer)] : providerNames;
  for (const provider of ordered) {
    for (const model of recentMap.get(provider) ?? []) {
      recent.push({ provider, model });
    }
  }
  return { current, recent, providers };
}

function runDesktopModelCatalog(): Promise<ModelCatalog> {
  return new Promise((resolve, reject) => {
    execFile(
      "go",
      ["run", catalogHelper],
      { cwd: repositoryRoot, env: process.env, timeout: 70_000, maxBuffer: 4 * 1024 * 1024 },
      (error, stdout, stderr) => {
        if (error) {
          reject(new Error(stderr?.trim() || error.message));
          return;
        }
        try {
          resolve(JSON.parse(stdout.trim()) as ModelCatalog);
        } catch (parseError) {
          reject(parseError);
        }
      },
    );
  });
}

async function readModelCatalog(home: string): Promise<ModelCatalog> {
  if (modelCatalogCache && Date.now() - modelCatalogCache.createdAt < 60_000) return modelCatalogCache.catalog;
  if (modelCatalogInFlight) return modelCatalogInFlight;
  modelCatalogInFlight = runDesktopModelCatalog()
    .then((catalog) => {
      modelCatalogCache = { createdAt: Date.now(), catalog };
      return catalog;
    })
    .catch(async () => readRecentModelCatalog(home))
    .finally(() => {
      modelCatalogInFlight = undefined;
    });
  return modelCatalogInFlight;
}

function replaceSectionField(section: string, key: "provider" | "model", value: string): string {
  const serialized = JSON.stringify(value);
  const pattern = new RegExp(`^(\\s*${key}\\s*=\\s*)((?:"(?:[^"\\\\]|\\\\.)*")|(?:'(?:[^']|'')*')|[^\\s#]+)(\\s*(?:#.*)?)$`, "m");
  if (pattern.test(section)) return section.replace(pattern, (_match, prefix: string, _old: string, suffix: string) => `${prefix}${serialized}${suffix}`);
  const trimmed = section.replace(/^\s*\n/, "");
  return `${key} = ${serialized}\n${trimmed}`;
}

function upsertCurrentSection(source: string, provider: string, model: string): string {
  const match = source.match(/^\s*\[current\]\s*$/m);
  if (!match || match.index === undefined) {
    return `${source.trimEnd()}\n\n[current]\nprovider = ${JSON.stringify(provider)}\nmodel = ${JSON.stringify(model)}\n`;
  }
  const headerEnd = match.index + match[0].length;
  const rest = source.slice(headerEnd);
  const next = rest.search(/^\s*\[/m);
  const body = next === -1 ? rest : rest.slice(0, next);
  const after = next === -1 ? "" : rest.slice(next);
  const nextBody = replaceSectionField(replaceSectionField(body, "provider", provider), "model", model);
  return `${source.slice(0, headerEnd)}${nextBody.startsWith("\n") ? nextBody : `\n${nextBody}`}${after}`;
}

function formatRecentArray(models: string[]): string {
  return `[${models.map((model) => JSON.stringify(model)).join(", ")}]`;
}

function upsertRecentModels(source: string, provider: string, model: string): string {
  const match = source.match(/^\s*\[recent_models\]\s*$/m);
  const key = quoteTomlKey(provider);
  const linePattern = new RegExp(`^(\\s*(?:${key.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")})\\s*=\\s*)\\[(.*)\\](\\s*(?:#.*)?)$`, "m");
  if (!match || match.index === undefined) {
    const block = `\n[recent_models]\n${key} = ${formatRecentArray([model])}\n`;
    return `${source.trimEnd()}\n${block}`;
  }
  const headerEnd = match.index + match[0].length;
  const rest = source.slice(headerEnd);
  const next = rest.search(/^\s*\[/m);
  const body = next === -1 ? rest : rest.slice(0, next);
  const after = next === -1 ? "" : rest.slice(next);
  let nextBody = body;
  if (linePattern.test(body)) {
    nextBody = body.replace(linePattern, (_full, prefix: string, rawItems: string, suffix: string) => {
      const existing = Array.from(rawItems.matchAll(/"([^"]+)"|'((?:[^']|'')*)'/g), (entry) => (entry[1] || entry[2]?.replaceAll("''", "'") || "").trim()).filter(Boolean);
      return `${prefix}${formatRecentArray(uniqueModels(existing, model).slice(0, recentModelCap))}${suffix}`;
    });
  } else {
    const insertion = `${key} = ${formatRecentArray([model])}\n`;
    nextBody = body.endsWith("\n") || body.length === 0 ? `${body}${insertion}` : `${body}\n${insertion}`;
  }
  return `${source.slice(0, headerEnd)}${nextBody.startsWith("\n") ? nextBody : `\n${nextBody.replace(/^\n/, "")}`}${after}`;
}

function upsertHiddenModels(source: string, provider: string, model: string, enabled: boolean): string {
  const hidden = parseHiddenModels(source);
  const current = hidden.get(provider) ?? [];
  const models = enabled
    ? current.filter((id) => id !== model)
    : uniqueModels(current, model);
  const key = quoteTomlKey(provider);
  const match = source.match(/^\s*\[hidden_models\]\s*$/m);

  if (!match || match.index === undefined) {
    if (models.length === 0) return source;
    return `${source.trimEnd()}\n\n[hidden_models]\n${key} = ${formatRecentArray(models)}\n`;
  }

  const headerEnd = match.index + match[0].length;
  const rest = source.slice(headerEnd);
  const next = rest.search(/^\s*\[/m);
  const body = next === -1 ? rest : rest.slice(0, next);
  const after = next === -1 ? "" : rest.slice(next);
  const escapedKey = key.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const linePattern = new RegExp(`^(\\s*${escapedKey}\\s*=\\s*)\\[(.*)\\](\\s*(?:#.*)?)$`, "m");
  let nextBody = body;

  if (linePattern.test(body)) {
    nextBody = models.length === 0
      ? body.replace(linePattern, "")
      : body.replace(linePattern, (_full, prefix: string, _items: string, suffix: string) => `${prefix}${formatRecentArray(models)}${suffix}`);
  } else if (models.length > 0) {
    const insertion = `${key} = ${formatRecentArray(models)}\n`;
    nextBody = body.endsWith("\n") || body.length === 0 ? `${body}${insertion}` : `${body}\n${insertion}`;
  }

  return `${source.slice(0, headerEnd)}${nextBody.startsWith("\n") ? nextBody : `\n${nextBody}`}${after}`;
}

async function writeModelVisibility(home: string, provider: string, model: string, enabled: boolean): Promise<ModelVisibility> {
  const filePath = configPath(home);
  const source = await readFile(filePath, "utf8");
  const providers = listProviderNames(source);
  if (!providers.includes(provider) && !parseRecentModels(source).has(provider)) {
    throw new Error(`unknown provider ${provider}`);
  }
  const next = upsertHiddenModels(source, provider, model, enabled);
  if (next !== source) {
    const temporaryPath = `${filePath}.gui.tmp`;
    await writeFile(temporaryPath, next, { encoding: "utf8", mode: 0o600 });
    await rename(temporaryPath, filePath);
  }
  return { enabled, model, provider };
}

async function writeCurrentModel(home: string, provider: string, model: string): Promise<ModelChoice> {
  const filePath = configPath(home);
  const source = await readFile(filePath, "utf8");
  const providers = listProviderNames(source);
  if (!providers.includes(provider) && !parseRecentModels(source).has(provider)) {
    throw new Error(`unknown provider ${provider}`);
  }
  const current = parseCurrent(source);
  let next = upsertCurrentSection(source, provider, model);
  if (current.provider !== provider || current.model !== model) {
    next = upsertRecentModels(next, provider, model);
  }
  const temporaryPath = `${filePath}.gui.tmp`;
  await writeFile(temporaryPath, next, { encoding: "utf8", mode: 0o600 });
  await rename(temporaryPath, filePath);
  modelCatalogCache = undefined;
  return { provider, model };
}

function attachModelsEndpoint(server: { middlewares: { use: (route: string, handler: (request: JsonRequest, response: JsonResponse, next: () => void) => void) => void } }) {
  server.middlewares.use(modelsEndpoint, (request, response, next) => {
    if (request.method !== "GET") {
      next();
      return;
    }
    void readModelCatalog(solomonHome())
      .then((catalog) => respond(response, 200, catalog))
      .catch(() => respond(response, 500, { current: { provider: "", model: "" }, recent: [], providers: [] }));
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
        respond(response, 200, await writeCurrentModel(solomonHome(), provider, model));
      })
      .catch((error: unknown) => respond(response, 500, { error: error instanceof Error ? error.message : "Unable to save model" }));
  });
}

function attachModelVisibilityEndpoint(server: { middlewares: { use: (route: string, handler: (request: JsonRequest, response: JsonResponse, next: () => void) => void) => void } }) {
  server.middlewares.use(modelVisibilityEndpoint, (request, response, next) => {
    if (request.method !== "PUT") {
      next();
      return;
    }
    void readJsonBody(request)
      .then((payload) => {
        if (!payload || typeof payload !== "object" || !("enabled" in payload) || !("model" in payload) || !("provider" in payload)) {
          throw new Error("provider, model and enabled are required");
        }
        const enabled = typeof payload.enabled === "boolean" ? payload.enabled : undefined;
        const model = typeof payload.model === "string" ? payload.model.trim() : "";
        const provider = typeof payload.provider === "string" ? payload.provider.trim() : "";
        if (enabled === undefined || !model || !provider) throw new Error("provider, model and enabled are required");
        return writeModelVisibility(solomonHome(), provider, model, enabled);
      })
      .then((result) => {
        modelCatalogCache = undefined;
        respond(response, 200, result);
      })
      .catch((error: unknown) => respond(response, 500, { error: error instanceof Error ? error.message : "Unable to save model visibility" }));
  });
}

function attachConnectProviderEndpoint(server: { middlewares: { use: (route: string, handler: (request: JsonRequest, response: JsonResponse, next: () => void) => void) => void } }) {
  server.middlewares.use(connectProviderEndpoint, (request, response, next) => {
    if (request.method !== "POST") {
      next();
      return;
    }
    void readJsonBody(request)
      .then((payload) => new Promise<string>((resolve, reject) => {
        const child = spawn("go", ["run", providerSetupHelper], {
          cwd: repositoryRoot,
          env: process.env,
          stdio: ["pipe", "pipe", "pipe"],
        });
        let stdout = "";
        let stderr = "";
        child.stdout.on("data", (chunk) => { stdout += String(chunk); });
        child.stderr.on("data", (chunk) => { stderr += String(chunk); });
        child.once("error", reject);
        child.once("close", (code) => {
          if (code === 0) resolve(stdout.trim());
          else reject(new Error(stderr.trim() || "provider setup failed"));
        });
        child.stdin.end(JSON.stringify(payload));
      }))
      .then((result) => {
        modelCatalogCache = undefined;
        respond(response, 200, JSON.parse(result) as object);
      })
      .catch((error: unknown) => respond(response, 500, { error: error instanceof Error ? error.message : "Unable to connect provider" }));
  });
}

export function modelsPlugin(): Plugin {
  return {
    configurePreviewServer(server) {
      attachModelsEndpoint(server);
      attachCurrentModelEndpoint(server);
      attachModelVisibilityEndpoint(server);
      attachConnectProviderEndpoint(server);
    },
    configureServer(server) {
      attachModelsEndpoint(server);
      attachCurrentModelEndpoint(server);
      attachModelVisibilityEndpoint(server);
      attachConnectProviderEndpoint(server);
    },
    name: "solomon-models",
  };
}
