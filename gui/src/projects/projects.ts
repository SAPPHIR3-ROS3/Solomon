import { desktopBridge, serverEndpoint } from "../platform";

export const PROJECT_GIT_BRANCH_CHANGED_EVENT = "solomon:git-branch-changed";
const USER_NAME_CACHE_KEY = "solomon:user-name";
const HOME_STATS_CACHE_KEY = "solomon:home-stats";

export type Chat = {
  id: string;
  lastMessageAt: string;
  title: string;
};

export type ProjectTokenStats = {
  user: number;
  reasoning: number;
  response: number;
  total: number;
};

export type Project = {
  chats: Chat[];
  id: string;
  name: string;
  path: string;
  chatCount: number;
  tokenStats?: ProjectTokenStats;
};

export type CachedHomeStats = {
  chatCount: number;
  tokenStats: ProjectTokenStats;
};

export type ReasoningEffort = "none" | "low" | "medium" | "high";

export type ProjectSidebarData = {
  projects: Project[];
  reasoningEffort: ReasoningEffort;
  userName: string;
};

export type ProjectRemovalInfo = {
  dataPath: string;
  dataSizeBytes: number;
  projectPath: string;
  projectSizeBytes: number;
};

export type ProjectDirectoryEntry = {
  isDirectory: boolean;
  name: string;
  path: string;
};

export type ProjectAtMentionSuggestion = {
  isDirectory: boolean;
  path: string;
  tag: string;
};

export type ProjectAtMentionEntry = Pick<ProjectAtMentionSuggestion, "isDirectory" | "path">;

export type ProjectResearch = {
  finishedAt: string;
  id: string;
  phase: string;
  sourceCount: number;
  startedAt: string;
  status: string;
  title: string;
};

export type ProjectBranches = {
  branches: string[];
  current: string;
  isRepo: boolean;
};

export type ProjectGitCommit = {
  author: string;
  authoredAt: string;
  hash: string;
  parents: string[];
  refs: string[];
  shortHash: string;
  subject: string;
};

export type ProjectGitHistory = {
  commits: ProjectGitCommit[];
  current: string;
  isRepo: boolean;
};

export type ProjectGitStatus = {
  changes: Record<string, string>;
  isRepo: boolean;
  staged: Record<string, string>;
};

export type ProjectWorktree = {
  bare: boolean;
  branch: string;
  current: boolean;
  path: string;
};

export type ProjectWorktrees = {
  worktrees: ProjectWorktree[];
};

export async function fetchProjectDirectoryEntries(projectID: string, directoryPath = ""): Promise<ProjectDirectoryEntry[]> {
  const bridge = await desktopBridge();
  if (bridge?.ProjectDirectoryEntries) {
    return projectDirectoryEntriesFromPayload(await bridge.ProjectDirectoryEntries(projectID, directoryPath));
  }
  const response = await fetch(await serverEndpoint(
    `/__solomon/projects/${encodeURIComponent(projectID)}/files?path=${encodeURIComponent(directoryPath)}`,
  ));
  if (!response.ok) throw new Error(`Unable to read project files: ${response.status}`);
  return projectDirectoryEntriesFromPayload(await response.json());
}

export async function fetchHomeDirectoryEntries(directoryPath = "", signal?: AbortSignal): Promise<ProjectDirectoryEntry[]> {
  const bridge = await desktopBridge();
  if (bridge?.HomeDirectoryEntries) {
    return projectDirectoryEntriesFromPayload(await bridge.HomeDirectoryEntries(directoryPath));
  }
  const response = await fetch(await serverEndpoint(
    `/__solomon/home-directories?path=${encodeURIComponent(directoryPath)}`,
  ), { signal });
  if (!response.ok) throw new Error(`Unable to read home directories: ${response.status}`);
  return projectDirectoryEntriesFromPayload(await response.json());
}

export async function fetchProjectAtMentionSuggestions(projectID: string, query: string): Promise<ProjectAtMentionSuggestion[]> {
  const bridge = await desktopBridge();
  if (!bridge?.ProjectAtMentionSuggestions) return [];
  return atMentionSuggestionsFromPayload(await bridge.ProjectAtMentionSuggestions(projectID, query));
}

export async function fetchAtMentionSuggestions(entries: ProjectAtMentionEntry[], query: string): Promise<ProjectAtMentionSuggestion[]> {
  const bridge = await desktopBridge();
  // Test chats are also used by the browser-only preview, where no Wails Go
  // bridge exists. Keep that fixture usable there; desktop always delegates to
  // the canonical atmention implementation above.
  if (!bridge?.AtMentionSuggestions) return virtualAtMentionSuggestions(entries, query);
  return atMentionSuggestionsFromPayload(await bridge.AtMentionSuggestions(entries, query));
}

export async function fetchProjectResearch(projectID: string): Promise<ProjectResearch[]> {
  const bridge = await desktopBridge();
  if (bridge?.ProjectResearch) return projectResearchFromPayload(await bridge.ProjectResearch(projectID));
  const response = await fetch(await serverEndpoint(`/__solomon/projects/${encodeURIComponent(projectID)}/research`));
  if (!response.ok) throw new Error(`Unable to read project research: ${response.status}`);
  return projectResearchFromPayload(await response.json());
}

export async function fetchProjectResearchReport(projectID: string, researchID: string): Promise<string> {
  const bridge = await desktopBridge();
  if (bridge?.ProjectResearchReport) return bridge.ProjectResearchReport(projectID, researchID);
  const response = await fetch(await serverEndpoint(`/__solomon/projects/${encodeURIComponent(projectID)}/research/${encodeURIComponent(researchID)}/report`));
  if (!response.ok) throw new Error(`Unable to read research report: ${response.status}`);
  return response.text();
}

export async function fetchProjectBranches(projectID: string, signal?: AbortSignal): Promise<ProjectBranches> {
  const bridge = await desktopBridge();
  if (bridge?.ProjectBranches) {
    return projectBranchesFromPayload(await bridge.ProjectBranches(projectID));
  }
  const response = await fetch(await serverEndpoint(`/__solomon/projects/${encodeURIComponent(projectID)}/branches`), { signal });
  if (!response.ok) throw new Error(`Unable to read project branches: ${response.status}`);
  return projectBranchesFromPayload(await response.json());
}

export async function fetchProjectGitHistory(projectID: string, signal?: AbortSignal): Promise<ProjectGitHistory> {
  const bridge = await desktopBridge();
  if (bridge?.ProjectGitHistory) {
    return projectGitHistoryFromPayload(await bridge.ProjectGitHistory(projectID));
  }
  const response = await fetch(await serverEndpoint(`/__solomon/projects/${encodeURIComponent(projectID)}/history`), { signal });
  if (!response.ok) throw new Error(`Unable to read project Git history: ${response.status}`);
  return projectGitHistoryFromPayload(await response.json());
}

export async function fetchProjectGitStatus(projectID: string, signal?: AbortSignal): Promise<ProjectGitStatus> {
  const bridge = await desktopBridge();
  if (bridge?.ProjectGitStatus) {
    return projectGitStatusFromPayload(await bridge.ProjectGitStatus(projectID));
  }
  const response = await fetch(await serverEndpoint(`/__solomon/projects/${encodeURIComponent(projectID)}/status`), { signal });
  if (!response.ok) throw new Error(`Unable to read project Git status: ${response.status}`);
  return projectGitStatusFromPayload(await response.json());
}

export async function fetchProjectWorktrees(projectID: string, signal?: AbortSignal): Promise<ProjectWorktrees> {
  const bridge = await desktopBridge();
  if (bridge?.ProjectWorktrees) {
    return projectWorktreesFromPayload(await bridge.ProjectWorktrees(projectID));
  }
  const response = await fetch(await serverEndpoint(`/__solomon/projects/${encodeURIComponent(projectID)}/worktrees`), { signal });
  if (!response.ok) throw new Error(`Unable to read project worktrees: ${response.status}`);
  return projectWorktreesFromPayload(await response.json());
}

export async function checkoutProjectBranch(projectID: string, branch: string): Promise<ProjectBranches> {
  const bridge = await desktopBridge();
  if (bridge?.CheckoutProjectBranch) {
    return projectBranchesFromPayload(await bridge.CheckoutProjectBranch(projectID, branch));
  }
  const response = await fetch(await serverEndpoint(`/__solomon/projects/${encodeURIComponent(projectID)}/checkout`), {
    body: JSON.stringify({ branch }),
    headers: { "Content-Type": "application/json" },
    method: "POST",
  });
  if (!response.ok) throw new Error(`Unable to checkout branch: ${response.status}`);
  return projectBranchesFromPayload(await response.json());
}

export async function fetchProjectSidebarData(signal?: AbortSignal): Promise<ProjectSidebarData> {
  const bridge = await desktopBridge();
  if (bridge) return projectSidebarDataFromPayload(await bridge.ProjectSidebarData());
  const response = await fetch(await serverEndpoint("/__solomon/projects"), { signal });
  if (!response.ok) throw new Error(`Unable to load projects: ${response.status}`);
  const payload: unknown = await response.json();
  return projectSidebarDataFromPayload(payload);
}

export function getCachedUserName(): string | null {
  try {
    return typeof window === "undefined" ? null : window.localStorage.getItem(USER_NAME_CACHE_KEY);
  } catch {
    return null;
  }
}

export function cacheUserName(userName: string): void {
  try {
    if (typeof window !== "undefined") window.localStorage.setItem(USER_NAME_CACHE_KEY, userName);
  } catch {
    // Local storage can be unavailable in private or restricted browser contexts.
  }
}

export function getCachedHomeStats(): CachedHomeStats | null {
  try {
    if (typeof window === "undefined") return null;
    const raw = window.localStorage.getItem(HOME_STATS_CACHE_KEY);
    if (!raw) return null;

    const payload: unknown = JSON.parse(raw);
    if (!payload || typeof payload !== "object") return null;
    const record = payload as Record<string, unknown>;
    const tokenStats = record.tokenStats;
    if (!tokenStats || typeof tokenStats !== "object") return null;
    const tokenRecord = tokenStats as Record<string, unknown>;
    const isCount = (value: unknown): value is number => typeof value === "number" && Number.isFinite(value) && value >= 0;
    if (!isCount(record.chatCount) || !isCount(tokenRecord.user) || !isCount(tokenRecord.reasoning) || !isCount(tokenRecord.response) || !isCount(tokenRecord.total)) {
      return null;
    }

    return {
      chatCount: record.chatCount,
      tokenStats: {
        reasoning: tokenRecord.reasoning,
        response: tokenRecord.response,
        total: tokenRecord.total,
        user: tokenRecord.user,
      },
    };
  } catch {
    return null;
  }
}

export function cacheHomeStats(project: Project): void {
  try {
    if (typeof window === "undefined") return;
    window.localStorage.setItem(HOME_STATS_CACHE_KEY, JSON.stringify({
      chatCount: project.chatCount,
      tokenStats: project.tokenStats ?? { reasoning: 0, response: 0, total: 0, user: 0 },
    }));
  } catch {
    // Local storage can be unavailable in private or restricted browser contexts.
  }
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

export async function removeProjectFromSidebar(projectID: string): Promise<void> {
  const bridge = await desktopBridge();
  if (bridge?.RemoveProjectFromSidebar) {
    await bridge.RemoveProjectFromSidebar(projectID);
    return;
  }
  await removeProject(projectID, false);
}

export async function removeProjectFromDisk(projectID: string): Promise<void> {
  const bridge = await desktopBridge();
  if (bridge?.RemoveProjectFromDisk) {
    await bridge.RemoveProjectFromDisk(projectID);
    return;
  }
  await removeProject(projectID, true);
}

export async function fetchProjectRemovalInfo(projectID: string): Promise<ProjectRemovalInfo> {
  const bridge = await desktopBridge();
  if (bridge?.ProjectRemovalInfo) return projectRemovalInfoFromPayload(await bridge.ProjectRemovalInfo(projectID));
  const response = await fetch(await serverEndpoint(`/__solomon/projects/${encodeURIComponent(projectID)}/removal-info`));
  if (!response.ok) throw new Error(`Unable to read project details: ${response.status}`);
  return projectRemovalInfoFromPayload(await response.json());
}

async function removeProject(projectID: string, removeData: boolean): Promise<void> {
  const response = await fetch(await serverEndpoint(`/__solomon/projects/${encodeURIComponent(projectID)}${removeData ? "/disk" : ""}`), {
    method: "DELETE",
  });
  if (!response.ok) throw new Error(`Unable to remove project: ${response.status}`);
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

const modelVisibilityStorageKey = "solomon.model-visibility";

function modelVisibilityKey(provider: string, model: string) {
  return `${provider}\u0000${model}`;
}

function readLocalModelVisibility(): Record<string, boolean> {
  try {
    const parsed: unknown = JSON.parse(window.localStorage.getItem(modelVisibilityStorageKey) ?? "{}");
    if (!parsed || typeof parsed !== "object") return {};
    return Object.fromEntries(Object.entries(parsed).filter(([, value]) => typeof value === "boolean")) as Record<string, boolean>;
  } catch {
    return {};
  }
}

function writeLocalModelVisibility(provider: string, model: string, enabled: boolean) {
  try {
    const values = readLocalModelVisibility();
    values[modelVisibilityKey(provider, model)] = enabled;
    window.localStorage.setItem(modelVisibilityStorageKey, JSON.stringify(values));
  } catch {
    // Local storage is only a compatibility fallback; the config remains the source of truth.
  }
}

function clearLocalModelVisibility(provider: string, model: string) {
  try {
    const values = readLocalModelVisibility();
    delete values[modelVisibilityKey(provider, model)];
    window.localStorage.setItem(modelVisibilityStorageKey, JSON.stringify(values));
  } catch {
    // Ignore unavailable local storage after a successful server save.
  }
}

function applyLocalModelVisibility(providers: ProviderCatalog[]): ProviderCatalog[] {
  const overrides = readLocalModelVisibility();
  if (!Object.keys(overrides).length) return providers;
  return providers.map((provider) => {
    const disabled = new Set(provider.disabled);
    for (const model of provider.models) {
      const override = overrides[modelVisibilityKey(provider.provider, model)];
      if (override === undefined) continue;
      if (override) disabled.delete(model);
      else disabled.add(model);
    }
    return { ...provider, disabled: [...disabled] };
  });
}

export async function fetchModelCatalog(signal?: AbortSignal): Promise<ModelCatalog> {
  const bridge = await desktopBridge();
  if (bridge) {
    if (!bridge.ModelCatalog) throw new Error("Unable to load models: desktop bridge missing ModelCatalog");
    return modelCatalogFromPayload(await bridge.ModelCatalog());
  }
  const response = await fetch("/__solomon/models", { signal });
  if (!response.ok) throw new Error(`Unable to load models: ${response.status}`);
  return modelCatalogFromPayload(await response.json());
}

export async function saveCurrentModel(provider: string, model: string): Promise<ModelChoice> {
  const bridge = await desktopBridge();
  if (bridge) {
    if (!bridge.SaveCurrentModel) throw new Error("Unable to save model: desktop bridge missing SaveCurrentModel");
    return modelChoiceFromPayload(await bridge.SaveCurrentModel(provider, model));
  }
  const response = await fetch("/__solomon/current-model", {
    body: JSON.stringify({ provider, model }),
    headers: { "Content-Type": "application/json" },
    method: "PUT",
  });
  if (!response.ok) {
    const detail = await response.text().catch(() => "");
    throw new Error(detail || `Unable to save model: ${response.status}`);
  }
  return modelChoiceFromPayload(await response.json());
}

export async function setModelEnabled(provider: string, model: string, enabled: boolean): Promise<ModelVisibility> {
  const bridge = await desktopBridge();
  if (bridge?.SetModelEnabled) {
    const result = modelVisibilityFromPayload(await bridge.SetModelEnabled(provider, model, enabled));
    if (result.provider !== provider || result.model !== model || result.enabled !== enabled) {
      throw new Error("Unable to verify model visibility update");
    }
    clearLocalModelVisibility(provider, model);
    return result;
  }
  let response: Response;
  try {
    response = await fetch(await serverEndpoint("/__solomon/model-visibility"), {
      body: JSON.stringify({ enabled, model, provider }),
      headers: { "Content-Type": "application/json" },
      method: "PUT",
    });
  } catch {
    writeLocalModelVisibility(provider, model, enabled);
    return { enabled, model, provider };
  }
  if (!response.ok) {
    if (response.status === 404 || response.status === 405) {
      writeLocalModelVisibility(provider, model, enabled);
      return { enabled, model, provider };
    }
    const payload = await response.json().catch(() => null);
    const detail = payload && typeof payload === "object" && "error" in payload && typeof payload.error === "string" ? payload.error : "Unable to save model visibility";
    throw new Error(detail);
  }
  const result = modelVisibilityFromPayload(await response.json());
  if (result.provider !== provider || result.model !== model || result.enabled !== enabled) {
    throw new Error("Unable to verify model visibility update");
  }
  clearLocalModelVisibility(provider, model);
  return result;
}

export async function connectProvider(request: ConnectProviderRequest): Promise<ModelChoice> {
  const bridge = await desktopBridge();
  if (bridge?.ConnectProvider) {
    return modelChoiceFromPayload(await bridge.ConnectProvider({
      APIKey: request.apiKey,
      BaseURL: request.baseURL,
      Kind: request.kind,
      Name: request.name,
    }));
  }
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
        return [{ provider: entry.provider, models, metadata, complete: "complete" in entry ? Boolean(entry.complete) : false, disabled }];
      })
    : [];
  const recent = "recent" in payload && Array.isArray(payload.recent)
    ? payload.recent.map(modelChoiceFromPayload).filter((entry) => entry.provider && entry.model)
    : [];
  return { current, providers: applyLocalModelVisibility(providers), recent };
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

function projectRemovalInfoFromPayload(payload: unknown): ProjectRemovalInfo {
  if (!payload || typeof payload !== "object") throw new Error("Unable to read project details");
  const record = payload as Record<string, unknown>;
  const requiredString = (key: string) => typeof record[key] === "string" ? record[key] : "";
  const requiredNumber = (key: string) => typeof record[key] === "number" ? record[key] : -1;
  const projectPath = requiredString("projectPath");
  const dataPath = requiredString("dataPath");
  const projectSizeBytes = requiredNumber("projectSizeBytes");
  const dataSizeBytes = requiredNumber("dataSizeBytes");
  if (!projectPath || !dataPath || projectSizeBytes < 0 || dataSizeBytes < 0) throw new Error("Unable to read project details");
  return { dataPath, dataSizeBytes, projectPath, projectSizeBytes };
}

function projectDirectoryEntriesFromPayload(payload: unknown): ProjectDirectoryEntry[] {
  if (!Array.isArray(payload)) return [];
  return payload.flatMap((entry) => {
    if (!entry || typeof entry !== "object") return [];
    const name = "name" in entry && typeof entry.name === "string" ? entry.name : "";
    const path = "path" in entry && typeof entry.path === "string" ? entry.path : "";
    if (!name || !path) return [];
    return [{ isDirectory: "isDirectory" in entry && Boolean(entry.isDirectory), name, path }];
  });
}

function atMentionSuggestionsFromPayload(payload: unknown): ProjectAtMentionSuggestion[] {
  if (!Array.isArray(payload)) return [];
  return payload.flatMap((entry) => {
    if (!entry || typeof entry !== "object") return [];
    const record = entry as Record<string, unknown>;
    const path = typeof record.path === "string" ? record.path : "";
    const tag = typeof record.tag === "string" ? record.tag : "";
    if (!path || !tag) return [];
    return [{ isDirectory: Boolean(record.isDirectory), path, tag }];
  });
}

function virtualAtMentionSuggestions(entries: ProjectAtMentionEntry[], query: string): ProjectAtMentionSuggestion[] {
  const normalizedQuery = normalizeMentionPath(query);
  const matches = normalizedQuery
    ? entries.flatMap((entry) => {
        const score = virtualMentionScore(normalizedQuery, entry.path);
        return score === undefined ? [] : [{ entry, score }];
      }).sort((left, right) => left.score - right.score || normalizeMentionPath(left.entry.path).localeCompare(normalizeMentionPath(right.entry.path)))
    : entries.map((entry) => ({ entry, score: 0 })).sort((left, right) => normalizeMentionPath(left.entry.path).localeCompare(normalizeMentionPath(right.entry.path)));
  return matches.slice(0, 10).map(({ entry }) => ({
    ...entry,
    tag: `@${virtualShortTag(entry.path, entries)}`,
  }));
}

function virtualMentionScore(query: string, path: string): number | undefined {
  const normalizedPath = normalizeMentionPath(path);
  const base = normalizedPath.split("/").at(-1) ?? normalizedPath;
  if (base.startsWith(query)) return 0;
  if (normalizedPath.split("/").some((part) => part.startsWith(query))) return 1;
  if (query.length >= 3 && base.includes(query)) return 2;
  if (query.length >= 3 && normalizedPath.includes(query)) return 3;
  return undefined;
}

function virtualShortTag(path: string, entries: ProjectAtMentionEntry[]): string {
  const normalizedPath = normalizeMentionPath(path);
  const parts = normalizedPath.split("/");
  for (let index = 0; index < parts.length; index += 1) {
    const suffix = parts.slice(index).join("/");
    const count = entries.filter((entry) => {
      const candidate = normalizeMentionPath(entry.path);
      return candidate === suffix || candidate.endsWith(`/${suffix}`);
    }).length;
    if (count === 1) return suffix;
  }
  return normalizedPath;
}

function normalizeMentionPath(path: string): string {
  return path.trim().replaceAll("\\", "/").replace(/^\.\//, "");
}

function projectResearchFromPayload(payload: unknown): ProjectResearch[] {
  if (!Array.isArray(payload)) return [];
  return payload.flatMap((entry) => {
    if (!entry || typeof entry !== "object") return [];
    const record = entry as Record<string, unknown>;
    const title = typeof record.title === "string" ? record.title.trim() : "";
    if (!title) return [];
    const stats = record.stats && typeof record.stats === "object" ? record.stats as Record<string, unknown> : {};
    return [{
      finishedAt: typeof record.finished_at === "string" ? record.finished_at : "",
      id: typeof record.id === "string" ? record.id : title,
      phase: typeof record.phase === "string" ? record.phase : "",
      sourceCount: typeof stats.urls === "number" ? stats.urls : 0,
      startedAt: typeof record.started_at === "string" ? record.started_at : "",
      status: typeof record.status === "string" ? record.status : "",
      title,
    }];
  }).sort((left, right) => Date.parse(right.finishedAt || right.startedAt) - Date.parse(left.finishedAt || left.startedAt));
}

function projectBranchesFromPayload(payload: unknown): ProjectBranches {
  if (!payload || typeof payload !== "object") {
    return { branches: [], current: "", isRepo: false };
  }
  const current = "current" in payload && typeof payload.current === "string" ? payload.current.trim() : "";
  const isRepo = "isRepo" in payload && Boolean(payload.isRepo);
  const branches = "branches" in payload && Array.isArray(payload.branches)
    ? payload.branches.filter((branch): branch is string => typeof branch === "string" && Boolean(branch.trim())).map((branch) => branch.trim())
    : [];
  return { branches, current, isRepo };
}

function projectGitHistoryFromPayload(payload: unknown): ProjectGitHistory {
  if (!payload || typeof payload !== "object") {
    return { commits: [], current: "", isRepo: false };
  }
  const current = "current" in payload && typeof payload.current === "string" ? payload.current.trim() : "";
  const isRepo = "isRepo" in payload && Boolean(payload.isRepo);
  const commits = "commits" in payload && Array.isArray(payload.commits)
    ? payload.commits.flatMap((entry): ProjectGitCommit[] => {
      if (!entry || typeof entry !== "object") return [];
      const record = entry as Record<string, unknown>;
      const hash = typeof record.hash === "string" ? record.hash.trim() : "";
      const shortHash = typeof record.shortHash === "string" ? record.shortHash.trim() : "";
      const subject = typeof record.subject === "string" ? record.subject.trim() : "";
      if (!hash || !shortHash || !subject) return [];
      const stringArray = (value: unknown) => Array.isArray(value)
        ? value.filter((item): item is string => typeof item === "string" && Boolean(item.trim())).map((item) => item.trim())
        : [];
      return [{
        author: typeof record.author === "string" ? record.author.trim() : "Unknown author",
        authoredAt: typeof record.authoredAt === "string" ? record.authoredAt : "",
        hash,
        parents: stringArray(record.parents),
        refs: stringArray(record.refs),
        shortHash,
        subject,
      }];
    })
    : [];
  return { commits, current, isRepo };
}

function projectGitStatusFromPayload(payload: unknown): ProjectGitStatus {
  if (!payload || typeof payload !== "object") {
    return { changes: {}, isRepo: false, staged: {} };
  }
  const readStatusMap = (value: unknown) => {
    if (!value || typeof value !== "object" || Array.isArray(value)) return {};
    return Object.entries(value).reduce<Record<string, string>>((result, [filePath, status]) => {
      if (typeof filePath === "string" && filePath && typeof status === "string" && status) result[filePath] = status[0] ?? status;
      return result;
    }, {});
  };
  return {
    changes: readStatusMap("changes" in payload ? payload.changes : null),
    isRepo: "isRepo" in payload && Boolean(payload.isRepo),
    staged: readStatusMap("staged" in payload ? payload.staged : null),
  };
}

function projectWorktreesFromPayload(payload: unknown): ProjectWorktrees {
  if (!payload || typeof payload !== "object" || !("worktrees" in payload) || !Array.isArray(payload.worktrees)) {
    return { worktrees: [] };
  }
  return {
    worktrees: payload.worktrees.flatMap((entry) => {
      if (!entry || typeof entry !== "object") return [];
      const path = "path" in entry && typeof entry.path === "string" ? entry.path.trim() : "";
      if (!path) return [];
      return [{
        bare: "bare" in entry && Boolean(entry.bare),
        branch: "branch" in entry && typeof entry.branch === "string" ? entry.branch.trim() : "",
        current: "current" in entry && Boolean(entry.current),
        path,
      }];
    }),
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
