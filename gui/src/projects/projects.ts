import { desktopBridge, serverEndpoint } from "../platform";

export type Chat = {
  id: string;
  lastMessageAt: string;
  title: string;
};

export type Project = {
  chats: Chat[];
  id: string;
  name: string;
  path: string;
  chatCount: number;
};

export type ReasoningEffort = "none" | "low" | "medium" | "high";

export type ProjectSidebarData = {
  projects: Project[];
  reasoningEffort: ReasoningEffort;
  userName: string;
};

export async function fetchProjectSidebarData(signal?: AbortSignal): Promise<ProjectSidebarData> {
  const bridge = await desktopBridge();
  if (bridge) return projectSidebarDataFromPayload(await bridge.ProjectSidebarData());
  const response = await fetch(await serverEndpoint("/__solomon/projects"), { signal });
  if (!response.ok) throw new Error(`Unable to load projects: ${response.status}`);
  const payload: unknown = await response.json();
  return projectSidebarDataFromPayload(payload);
}

export async function saveUserName(userName: string): Promise<string> {
  const bridge = await desktopBridge();
  if (bridge) return bridge.SaveUserName(userName);
  const response = await fetch(await serverEndpoint("/__solomon/user-name"), {
    body: JSON.stringify({ userName }),
    headers: { "Content-Type": "application/json" },
    method: "PUT",
  });
  if (!response.ok) throw new Error(`Unable to save user name: ${response.status}`);

  const payload: unknown = await response.json();
  if (!payload || typeof payload !== "object" || !("userName" in payload) || typeof payload.userName !== "string") {
    throw new Error("Unable to save user name: invalid response");
  }
  return payload.userName;
}

export async function saveReasoningEffort(effort: string): Promise<ReasoningEffort> {
  const bridge = await desktopBridge();
  if (bridge?.SaveReasoningEffort) return normalizeReasoningEffort(await bridge.SaveReasoningEffort(effort));
  const response = await fetch(await serverEndpoint("/__solomon/reasoning-effort"), {
    body: JSON.stringify({ reasoningEffort: effort }),
    headers: { "Content-Type": "application/json" },
    method: "PUT",
  });
  if (!response.ok) throw new Error(`Unable to save reasoning effort: ${response.status}`);
  const payload: unknown = await response.json();
  if (!payload || typeof payload !== "object" || !("reasoningEffort" in payload) || typeof payload.reasoningEffort !== "string") {
    throw new Error("Unable to save reasoning effort: invalid response");
  }
  return normalizeReasoningEffort(payload.reasoningEffort);
}

export function normalizeReasoningEffort(value: string): ReasoningEffort {
  const normalized = value.trim().toLowerCase();
  if (normalized === "med") return "medium";
  if (normalized === "none" || normalized === "low" || normalized === "medium" || normalized === "high") return normalized;
  return "none";
}

export type ModelChoice = {
  model: string;
  provider: string;
};

export type ProviderCatalog = {
  complete: boolean;
  models: string[];
  provider: string;
};

export type ModelCatalog = {
  current: ModelChoice;
  providers: ProviderCatalog[];
  recent: ModelChoice[];
};

export async function fetchModelCatalog(signal?: AbortSignal): Promise<ModelCatalog> {
  const bridge = await desktopBridge();
  if (bridge?.ModelCatalog) return modelCatalogFromPayload(await bridge.ModelCatalog());
  const response = await fetch(await serverEndpoint("/__solomon/models"), { signal });
  if (!response.ok) throw new Error(`Unable to load models: ${response.status}`);
  return modelCatalogFromPayload(await response.json());
}

export async function saveCurrentModel(provider: string, model: string): Promise<ModelChoice> {
  const bridge = await desktopBridge();
  if (bridge?.SaveCurrentModel) return modelChoiceFromPayload(await bridge.SaveCurrentModel(provider, model));
  const response = await fetch(await serverEndpoint("/__solomon/current-model"), {
    body: JSON.stringify({ provider, model }),
    headers: { "Content-Type": "application/json" },
    method: "PUT",
  });
  if (!response.ok) throw new Error(`Unable to save model: ${response.status}`);
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
          ? entry.models.filter((model): model is string => typeof model === "string" && Boolean(model.trim()))
          : [];
        return [{ provider: entry.provider, models, complete: "complete" in entry ? Boolean(entry.complete) : false }];
      })
    : [];
  const recent = "recent" in payload && Array.isArray(payload.recent)
    ? payload.recent.map(modelChoiceFromPayload).filter((entry) => entry.provider && entry.model)
    : [];
  return { current, providers, recent };
}

function modelChoiceFromPayload(payload: unknown): ModelChoice {
  if (!payload || typeof payload !== "object") return { provider: "", model: "" };
  return {
    provider: "provider" in payload && typeof payload.provider === "string" ? payload.provider.trim() : "",
    model: "model" in payload && typeof payload.model === "string" ? payload.model.trim() : "",
  };
}

function projectSidebarDataFromPayload(payload: unknown): ProjectSidebarData {
  if (!payload || typeof payload !== "object" || !("projects" in payload) || !Array.isArray(payload.projects)) {
    return { projects: [], reasoningEffort: "none", userName: "" };
  }
  return {
    projects: payload.projects.filter(isProject),
    reasoningEffort: "reasoningEffort" in payload && typeof payload.reasoningEffort === "string"
      ? normalizeReasoningEffort(payload.reasoningEffort)
      : "none",
    userName: "userName" in payload && typeof payload.userName === "string" ? payload.userName : "",
  };
}

function isProject(value: unknown): value is Project {
  return Boolean(
    value
      && typeof value === "object"
      && "chats" in value && Array.isArray(value.chats) && value.chats.every(isChat)
      && "id" in value && typeof value.id === "string"
      && "name" in value && typeof value.name === "string"
      && "path" in value && typeof value.path === "string"
      && "chatCount" in value && typeof value.chatCount === "number",
  );
}

function isChat(value: unknown): value is Chat {
  return Boolean(
    value
      && typeof value === "object"
      && "id" in value && typeof value.id === "string"
      && "lastMessageAt" in value && typeof value.lastMessageAt === "string"
      && "title" in value && typeof value.title === "string",
  );
}
