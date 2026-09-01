import { serverEndpoint } from "../platform";

export const PROJECT_GIT_BRANCH_CHANGED_EVENT = "solomon:git-branch-changed";
export const PROJECTS_CHANGED_EVENT = "solomon:projects-changed";
const USER_NAME_CACHE_KEY = "solomon:user-name";
const HOME_STATS_CACHE_KEY = "solomon:home-stats";
const PROJECT_SIDEBAR_CACHE_KEY = "solomon:project-sidebar";
const CURRENT_MODEL_CACHE_KEY = "solomon:current-model";
const MODEL_CATALOG_CACHE_KEY = "solomon:model-catalog";

export type ProjectChatSummary = {
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
  chats: ProjectChatSummary[];
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

export type ReasoningEffort = "none" | "low" | "medium" | "high" | "xhigh" | "max";

export type ProjectSidebarData = {
  fastMode: boolean;
  projects: Project[];
  reasoningEffort: ReasoningEffort;
  userName: string;
};

let projectSidebarCache: ProjectSidebarData | null = readProjectSidebarCache();
let projectSidebarCacheNeedsRefresh = Boolean(projectSidebarCache);
let projectSidebarRequest: Promise<ProjectSidebarData> | null = null;
let projectSidebarPrefetchStarted = false;
let modelCatalogCache: ModelCatalog | null = readModelCatalogCache();
let modelCatalogCacheNeedsRefresh = Boolean(modelCatalogCache);
let modelCatalogRequest: Promise<ModelCatalog> | null = null;
let modelCatalogPrefetchStarted = false;

export function invalidateProjectSidebarDataCache(): void {
  projectSidebarCache = null;
  projectSidebarCacheNeedsRefresh = false;
}

if (typeof window !== "undefined") {
  window.addEventListener(PROJECTS_CHANGED_EVENT, () => invalidateProjectSidebarDataCache());
}

export function invalidateModelCatalogCache(): void {
  modelCatalogCache = null;
  modelCatalogCacheNeedsRefresh = false;
}

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

export type ProjectResearch = {
  finishedAt: string;
  id: string;
  maxRounds?: number;
  phase: string;
  round?: number;
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

export function projectWorktreeLabel(worktreePath: string): string {
  const normalized = worktreePath.replace(/\\/g, "/").replace(/\/+$/, "");
  const parts = normalized.split("/");
  return parts.at(-1) || worktreePath;
}

export async function fetchProjectDirectoryEntries(projectID: string, directoryPath = ""): Promise<ProjectDirectoryEntry[]> {
  const response = await fetch(await serverEndpoint(
    `/__solomon/projects/${encodeURIComponent(projectID)}/files?path=${encodeURIComponent(directoryPath)}`,
  ));
  if (!response.ok) throw new Error(`Unable to read project files: ${response.status}`);
  return projectDirectoryEntriesFromPayload(await response.json());
}

export async function fetchHomeDirectoryEntries(directoryPath = "", signal?: AbortSignal): Promise<ProjectDirectoryEntry[]> {
  const response = await fetch(await serverEndpoint(
    `/__solomon/home-directories?path=${encodeURIComponent(directoryPath)}`,
  ), { signal });
  if (!response.ok) throw new Error(`Unable to read home directories: ${response.status}`);
  return projectDirectoryEntriesFromPayload(await response.json());
}

export async function fetchProjectAtMentionSuggestions(projectID: string, query: string): Promise<ProjectAtMentionSuggestion[]> {
  const response = await fetch(await serverEndpoint(
    `/__solomon/projects/${encodeURIComponent(projectID)}/at-mentions?query=${encodeURIComponent(query)}`,
  ));
  if (!response.ok) throw new Error(`Unable to read project mentions: ${response.status}`);
  return atMentionSuggestionsFromPayload(await response.json());
}

export async function fetchProjectResearch(projectID: string): Promise<ProjectResearch[]> {
  const response = await fetch(await serverEndpoint(`/__solomon/projects/${encodeURIComponent(projectID)}/research`));
  if (!response.ok) throw new Error(`Unable to read project research: ${response.status}`);
  return projectResearchFromPayload(await response.json());
}

export async function fetchProjectResearchReport(projectID: string, researchID: string): Promise<string> {
  const response = await fetch(await serverEndpoint(`/__solomon/projects/${encodeURIComponent(projectID)}/research/${encodeURIComponent(researchID)}/report`));
  if (!response.ok) throw new Error(`Unable to read research report: ${response.status}`);
  return response.text();
}

export async function fetchProjectBranches(projectID: string, signal?: AbortSignal): Promise<ProjectBranches> {
  const response = await fetch(await serverEndpoint(`/__solomon/projects/${encodeURIComponent(projectID)}/branches`), { signal });
  if (!response.ok) throw new Error(`Unable to read project branches: ${response.status}`);
  return projectBranchesFromPayload(await response.json());
}

export async function fetchHomeDirectoryBranches(directoryPath: string, signal?: AbortSignal): Promise<ProjectBranches> {
  const response = await fetch(await serverEndpoint(
    `/__solomon/home-git-branches?path=${encodeURIComponent(directoryPath)}`,
  ), { signal });
  if (!response.ok) throw new Error(`Unable to read home directory branches: ${response.status}`);
  return projectBranchesFromPayload(await response.json());
}

export async function fetchProjectGitHistory(projectID: string, signal?: AbortSignal): Promise<ProjectGitHistory> {
  const response = await fetch(await serverEndpoint(`/__solomon/projects/${encodeURIComponent(projectID)}/history`), { signal });
  if (!response.ok) throw new Error(`Unable to read project Git history: ${response.status}`);
  return projectGitHistoryFromPayload(await response.json());
}

export async function fetchProjectGitStatus(projectID: string, signal?: AbortSignal): Promise<ProjectGitStatus> {
  const response = await fetch(await serverEndpoint(`/__solomon/projects/${encodeURIComponent(projectID)}/status`), { signal });
  if (!response.ok) throw new Error(`Unable to read project Git status: ${response.status}`);
  return projectGitStatusFromPayload(await response.json());
}

export async function fetchProjectWorktrees(projectID: string, signal?: AbortSignal): Promise<ProjectWorktrees> {
  const response = await fetch(await serverEndpoint(`/__solomon/projects/${encodeURIComponent(projectID)}/worktrees`), { signal });
  if (!response.ok) throw new Error(`Unable to read project worktrees: ${response.status}`);
  return projectWorktreesFromPayload(await response.json());
}

export async function fetchHomeDirectoryWorktrees(directoryPath: string, signal?: AbortSignal): Promise<ProjectWorktrees> {
  const response = await fetch(await serverEndpoint(
    `/__solomon/home-git-worktrees?path=${encodeURIComponent(directoryPath)}`,
  ), { signal });
  if (!response.ok) throw new Error(`Unable to read home directory worktrees: ${response.status}`);
  return projectWorktreesFromPayload(await response.json());
}

export async function checkoutProjectBranch(projectID: string, branch: string): Promise<ProjectBranches> {
  const response = await fetch(await serverEndpoint(`/__solomon/projects/${encodeURIComponent(projectID)}/checkout`), {
    body: JSON.stringify({ branch }),
    headers: { "Content-Type": "application/json" },
    method: "POST",
  });
  if (!response.ok) throw new Error(`Unable to checkout branch: ${response.status}`);
  return projectBranchesFromPayload(await response.json());
}

export async function checkoutHomeDirectoryBranch(directoryPath: string, branch: string): Promise<ProjectBranches> {
  const response = await fetch(await serverEndpoint(
    `/__solomon/home-git-checkout?path=${encodeURIComponent(directoryPath)}`,
  ), {
    body: JSON.stringify({ branch }),
    headers: { "Content-Type": "application/json" },
    method: "POST",
  });
  if (!response.ok) throw new Error(`Unable to checkout home directory branch: ${response.status}`);
  return projectBranchesFromPayload(await response.json());
}

export function getCachedProjectSidebarData(): ProjectSidebarData | null {
  return projectSidebarCache ?? readProjectSidebarCache();
}

export function prefetchProjectSidebarData(): void {
  if (projectSidebarPrefetchStarted) return;
  projectSidebarPrefetchStarted = true;
  void startProjectSidebarRequest().catch(() => {
    // The first request is opportunistic; consumers retry when they need data.
  });
}

export async function fetchProjectSidebarData(_signal?: AbortSignal): Promise<ProjectSidebarData> {
  // The shared request deliberately outlives an individual component's
  // AbortController. Consumers still ignore the result after unmounting, but
  // closing and reopening a panel does not start another disk/network scan.
  if (projectSidebarRequest) return projectSidebarRequest;
  if (projectSidebarCache && !projectSidebarCacheNeedsRefresh) return projectSidebarCache;
  return startProjectSidebarRequest();
}

function startProjectSidebarRequest(): Promise<ProjectSidebarData> {
  if (projectSidebarRequest) return projectSidebarRequest;
  const stale = projectSidebarCache;
  projectSidebarRequest = (async () => {
    const response = await fetch(await serverEndpoint("/__solomon/projects"));
    if (!response.ok) throw new Error(`Unable to load projects: ${response.status}`);
    const payload: unknown = await response.json();
    return projectSidebarDataFromPayload(payload);
  })()
    .then((data) => {
      projectSidebarCache = data;
      projectSidebarCacheNeedsRefresh = false;
      cacheProjectSidebarData(data);
      return data;
    })
    .catch((reason: unknown) => {
      if (stale) return stale;
      throw reason;
    })
    .finally(() => {
      projectSidebarRequest = null;
    });
  return projectSidebarRequest;
}

export async function createProjectFromFolder(path: string): Promise<Project> {
  const response = await fetch(await serverEndpoint("/__solomon/projects"), {
    body: JSON.stringify({ path }),
    headers: { "Content-Type": "application/json" },
    method: "POST",
  });
  if (!response.ok) {
    let message = `Unable to create project: ${response.status}`;
    try {
      const payload: unknown = await response.json();
      if (payload && typeof payload === "object" && "error" in payload && typeof payload.error === "string") message = payload.error;
    } catch {
      // Keep the status-based message when the server did not return JSON.
    }
    throw new Error(message);
  }
  const payload: unknown = await response.json();
  if (!payload || typeof payload !== "object" || !("project" in payload) || !isProject(payload.project)) {
    throw new Error("Unable to read the created project");
  }
  window.dispatchEvent(new CustomEvent(PROJECTS_CHANGED_EVENT));
  invalidateProjectSidebarDataCache();
  return payload.project;
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

export async function saveUserName(userName: string): Promise<string> {
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
  await removeProject(projectID, false);
}

export async function removeProjectFromDisk(projectID: string): Promise<void> {
  await removeProject(projectID, true);
}

export async function fetchProjectRemovalInfo(projectID: string): Promise<ProjectRemovalInfo> {
  const response = await fetch(await serverEndpoint(`/__solomon/projects/${encodeURIComponent(projectID)}/removal-info`));
  if (!response.ok) throw new Error(`Unable to read project details: ${response.status}`);
  return projectRemovalInfoFromPayload(await response.json());
}

async function removeProject(projectID: string, removeData: boolean): Promise<void> {
  const response = await fetch(await serverEndpoint(`/__solomon/projects/${encodeURIComponent(projectID)}${removeData ? "/disk" : ""}`), {
    method: "DELETE",
  });
  if (!response.ok) throw new Error(`Unable to remove project: ${response.status}`);
  invalidateProjectSidebarDataCache();
}

export async function saveReasoningEffort(effort: string): Promise<ReasoningEffort> {
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
  const saved = normalizeReasoningEffort(payload.reasoningEffort);
  invalidateProjectSidebarDataCache();
  return saved;
}

export async function saveFastMode(enabled: boolean): Promise<boolean> {
  const response = await fetch(await serverEndpoint("/__solomon/fast-mode"), {
    body: JSON.stringify({ fastMode: enabled }),
    headers: { "Content-Type": "application/json" },
    method: "PUT",
  });
  if (!response.ok) throw new Error(`Unable to save fast mode: ${response.status}`);
  const payload: unknown = await response.json();
  if (!payload || typeof payload !== "object" || !("fastMode" in payload) || typeof payload.fastMode !== "boolean") {
    throw new Error("Unable to save fast mode: invalid response");
  }
  invalidateProjectSidebarDataCache();
  return payload.fastMode;
}

export function normalizeReasoningEffort(value: string): ReasoningEffort {
  const normalized = value.trim().toLowerCase().replaceAll("_", "-").replace(/\s+/g, "-");
  if (normalized === "med") return "medium";
  if (normalized === "x-high" || normalized === "extra-high") return "xhigh";
  if (normalized === "none" || normalized === "low" || normalized === "medium" || normalized === "high" || normalized === "xhigh" || normalized === "max") return normalized;
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

function cacheProjectSidebarData(data: ProjectSidebarData): void {
  try {
    if (typeof window !== "undefined") window.localStorage.setItem(PROJECT_SIDEBAR_CACHE_KEY, JSON.stringify(data));
  } catch {
    // The cache is only an optimization; storage quotas and restrictions are safe to ignore.
  }
}

function readProjectSidebarCache(): ProjectSidebarData | null {
  try {
    if (typeof window === "undefined") return null;
    const raw = window.localStorage.getItem(PROJECT_SIDEBAR_CACHE_KEY);
    if (!raw) return null;
    const payload: unknown = JSON.parse(raw);
    if (!payload || typeof payload !== "object" || !("projects" in payload) || !Array.isArray(payload.projects)) return null;
    return projectSidebarDataFromPayload(payload);
  } catch {
    return null;
  }
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

function projectSidebarDataFromPayload(payload: unknown): ProjectSidebarData {
  if (!payload || typeof payload !== "object" || !("projects" in payload) || !Array.isArray(payload.projects)) {
    return { fastMode: true, projects: [], reasoningEffort: "none", userName: "" };
  }
  return {
    fastMode: "fastMode" in payload && typeof payload.fastMode === "boolean" ? payload.fastMode : true,
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
      maxRounds: typeof record.max_rounds === "number" ? record.max_rounds : undefined,
      phase: typeof record.phase === "string" ? record.phase : "",
      round: typeof record.round === "number" ? record.round : undefined,
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

function isChat(value: unknown): value is ProjectChatSummary {
  return Boolean(
    value
      && typeof value === "object"
      && "id" in value && typeof value.id === "string"
      && "lastMessageAt" in value && typeof value.lastMessageAt === "string"
      && "title" in value && typeof value.title === "string",
  );
}
