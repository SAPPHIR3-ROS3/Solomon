import { serverEndpoint } from "../platform";

const CURRENT_MODEL_CACHE_KEY = "solomon:current-model";
const MODEL_CATALOG_CACHE_KEY = "solomon:model-catalog";

let modelCatalogCache: ModelCatalog | null = readModelCatalogCache();
let modelCatalogCacheNeedsRefresh = Boolean(modelCatalogCache);
let modelCatalogRequest: Promise<ModelCatalog> | null = null;
let modelCatalogPrefetchStarted = false;

export function invalidateModelCatalogCache(): void {
  modelCatalogCache = null;
  modelCatalogCacheNeedsRefresh = false;
}

export function getCachedCurrentModel(): ModelChoice | null {
  try {
    if (typeof window === "undefined") return null;
    const raw = window.localStorage.getItem(CURRENT_MODEL_CACHE_KEY);
    if (!raw) return null;
    const choice = modelChoiceFromPayload(JSON.parse(raw));
    return choice.provider && choice.model ? choice : null;
  } catch {
    return null;
  }
}

export function cacheCurrentModel(choice: ModelChoice): void {
  try {
    if (typeof window === "undefined" || !choice.provider || !choice.model) return;
    window.localStorage.setItem(CURRENT_MODEL_CACHE_KEY, JSON.stringify({
      model: choice.model,
      provider: choice.provider,
    }));
  } catch {
    // Local storage can be unavailable in private or restricted browser contexts.
  }
}

export function getCachedModelCatalog(): ModelCatalog | null {
  return modelCatalogCache ?? readModelCatalogCache();
}

export function prefetchModelCatalog(): void {
  if (modelCatalogPrefetchStarted) return;
  modelCatalogPrefetchStarted = true;
  void startModelCatalogRequest().catch(() => {
    // The first request is opportunistic; the model selector reports errors if
    // it still has no cached catalog when the user opens it.
  });
}


export type ModelChoice = {
	info?: ModelInfo;
  model: string;
  provider: string;
};

export type ModelInfo = {
  context: number;
  input: string[];
  output: number;
};

export type ProviderCatalog = {
  complete: boolean;
  disabled: string[];
  metadata: Record<string, ModelInfo>;
  models: string[];
  provider: string;
  supportsFastMode: boolean;
};

export type ModelCatalog = {
  current: ModelChoice;
  providers: ProviderCatalog[];
  recent: ModelChoice[];
};

export type ConnectProviderRequest = {
  apiKey: string;
  baseURL: string;
  kind: number;
  name: string;
};

export type ModelVisibility = {
  enabled: boolean;
  model: string;
  provider: string;
};

export async function fetchModelCatalog(): Promise<ModelCatalog> {
  if (modelCatalogRequest) return modelCatalogRequest;
  if (modelCatalogCache && !modelCatalogCacheNeedsRefresh) return modelCatalogCache;
  return startModelCatalogRequest();
}

function startModelCatalogRequest(): Promise<ModelCatalog> {
  if (modelCatalogRequest) return modelCatalogRequest;
  const stale = modelCatalogCache;
  modelCatalogRequest = (async () => {
    const response = await fetch(await serverEndpoint("/__solomon/models"));
    if (!response.ok) throw new Error(`Unable to load models: ${response.status}`);
    return modelCatalogFromPayload(await response.json());
  })()
    .then((catalog) => {
      modelCatalogCache = catalog;
      modelCatalogCacheNeedsRefresh = false;
      cacheModelCatalog(catalog);
      if (catalog.current.provider && catalog.current.model) cacheCurrentModel(catalog.current);
      return catalog;
    })
    .catch((reason: unknown) => {
      if (stale) return stale;
      throw reason;
    })
    .finally(() => {
      modelCatalogRequest = null;
    });
  return modelCatalogRequest;
}

export async function saveCurrentModel(provider: string, model: string): Promise<ModelChoice> {
  const response = await fetch(await serverEndpoint("/__solomon/current-model"), {
    body: JSON.stringify({ provider, model }),
    headers: { "Content-Type": "application/json" },
    method: "PUT",
  });
  if (!response.ok) {
    const detail = await response.text().catch(() => "");
    throw new Error(detail || `Unable to save model: ${response.status}`);
  }
  const saved = modelChoiceFromPayload(await response.json());
  invalidateModelCatalogCache();
  cacheCurrentModel(saved);
  return saved;
}

export function cacheModelVisibility(provider: string, model: string, enabled: boolean): void {
  if (!modelCatalogCache) return;
  modelCatalogCache = {
    ...modelCatalogCache,
    providers: modelCatalogCache.providers.map((group) => {
      if (group.provider !== provider) return group;
      const disabled = group.disabled ?? [];
      return {
        ...group,
        disabled: enabled
          ? disabled.filter((id) => id !== model)
          : disabled.includes(model) ? disabled : [...disabled, model],
      };
    }),
  };
  cacheModelCatalog(modelCatalogCache);
}

export async function setModelEnabled(provider: string, model: string, enabled: boolean): Promise<ModelVisibility> {
  const response = await fetch(await serverEndpoint("/__solomon/model-visibility"), {
    body: JSON.stringify({ enabled, model, provider }),
    headers: { "Content-Type": "application/json" },
    method: "PUT",
  });
  if (!response.ok) {
    const payload = await response.json().catch(() => null);
    const detail = payload && typeof payload === "object" && "error" in payload && typeof payload.error === "string" ? payload.error : "Unable to save model visibility";
    throw new Error(detail);
  }
  const result = modelVisibilityFromPayload(await response.json());
  if (result.provider !== provider || result.model !== model || result.enabled !== enabled) {
    throw new Error("Unable to verify model visibility update");
  }
  return result;
}

export async function connectProvider(request: ConnectProviderRequest): Promise<ModelChoice> {
  const response = await fetch(await serverEndpoint("/__solomon/connect-provider"), {
    body: JSON.stringify(request),
    headers: { "Content-Type": "application/json" },
    method: "POST",
  });
  if (!response.ok) {
    const payload = await response.json().catch(() => null);
    const detail = payload && typeof payload === "object" && "error" in payload && typeof payload.error === "string" ? payload.error : "Unable to connect provider";
    throw new Error(detail);
  }
  return modelChoiceFromPayload(await response.json());
}

function modelCatalogFromPayload(payload: unknown): ModelCatalog {
  if (!payload || typeof payload !== "object") {
    return { current: { provider: "", model: "" }, providers: [], recent: [] };
  }
  const current = "current" in payload ? modelChoiceFromPayload(payload.current) : { provider: "", model: "" };
  const providers = "providers" in payload && Array.isArray(payload.providers)
    ? payload.providers.flatMap((entry) => {
        if (!entry || typeof entry !== "object" || !("provider" in entry) || typeof entry.provider !== "string") return [];
        const models = "models" in entry && Array.isArray(entry.models)
          ? entry.models.filter((model: unknown): model is string => typeof model === "string" && Boolean(model.trim()))
          : [];
        const metadata = "metadata" in entry && entry.metadata && typeof entry.metadata === "object"
          ? Object.fromEntries(Object.entries(entry.metadata).flatMap(([id, info]) => {
              const parsed = modelInfoFromPayload(info);
              return parsed ? [[id, parsed]] : [];
            }))
          : {};
        const disabled = "disabled" in entry && Array.isArray(entry.disabled)
          ? entry.disabled.filter((model: unknown): model is string => typeof model === "string" && Boolean(model.trim()))
          : [];
        return [{
          provider: entry.provider,
          models,
          metadata,
          complete: "complete" in entry ? Boolean(entry.complete) : false,
          disabled,
          supportsFastMode: "supportsFastMode" in entry && entry.supportsFastMode === true,
        }];
      })
    : [];
  const recent = "recent" in payload && Array.isArray(payload.recent)
    ? payload.recent.map(modelChoiceFromPayload).filter((entry) => entry.provider && entry.model)
    : [];
  return { current, providers, recent };
}

function modelInfoFromPayload(payload: unknown): ModelInfo | undefined {
  if (!payload || typeof payload !== "object") return undefined;
  const context = "context" in payload && typeof payload.context === "number" ? payload.context : 0;
  const output = "output" in payload && typeof payload.output === "number" ? payload.output : 0;
  const input = "input" in payload && Array.isArray(payload.input)
    ? payload.input.filter((mode): mode is string => typeof mode === "string" && Boolean(mode.trim()))
    : [];
  if (!context && !output && !input.length) return undefined;
  return { context, input, output };
}

function modelChoiceFromPayload(payload: unknown): ModelChoice {
  if (!payload || typeof payload !== "object") return { provider: "", model: "" };
  return {
    provider: "provider" in payload && typeof payload.provider === "string" ? payload.provider.trim() : "",
    model: "model" in payload && typeof payload.model === "string" ? payload.model.trim() : "",
  };
}

function modelVisibilityFromPayload(payload: unknown): ModelVisibility {
  if (!payload || typeof payload !== "object") return { enabled: false, model: "", provider: "" };
  const record = payload as Record<string, unknown>;
  return {
    enabled: typeof record.enabled === "boolean" ? record.enabled : typeof record.Enabled === "boolean" ? record.Enabled : false,
    model: typeof record.model === "string" ? record.model.trim() : typeof record.Model === "string" ? record.Model.trim() : "",
    provider: typeof record.provider === "string" ? record.provider.trim() : typeof record.Provider === "string" ? record.Provider.trim() : "",
  };
}


function cacheModelCatalog(catalog: ModelCatalog): void {
  try {
    if (typeof window !== "undefined") window.localStorage.setItem(MODEL_CATALOG_CACHE_KEY, JSON.stringify(catalog));
  } catch {
    // The cache is only an optimization; storage quotas and restrictions are safe to ignore.
  }
}

function readModelCatalogCache(): ModelCatalog | null {
  try {
    if (typeof window === "undefined") return null;
    const raw = window.localStorage.getItem(MODEL_CATALOG_CACHE_KEY);
    if (!raw) return null;
    const payload: unknown = JSON.parse(raw);
    if (!payload || typeof payload !== "object" || !("providers" in payload) || !Array.isArray(payload.providers)) return null;
    return modelCatalogFromPayload(payload);
  } catch {
    return null;
  }
}
