import { serverEndpoint } from "../platform";
import {
  atMentionSuggestionsFromPayload,
  isProject,
  projectBranchesFromPayload,
  projectDirectoryEntriesFromPayload,
  projectGitHistoryFromPayload,
  projectGitStatusFromPayload,
  projectRemovalInfoFromPayload,
  projectResearchFromPayload,
  projectSidebarDataFromPayload,
  projectWorktreesFromPayload,
} from "./projectPayloads";

export {
  cacheCurrentModel,
  cacheModelVisibility,
  fetchModelCatalog,
  getCachedCurrentModel,
  getCachedModelCatalog,
  invalidateModelCatalogCache,
  prefetchModelCatalog,
  saveCurrentModel,
  setModelEnabled,
  connectProvider,
} from "./models";
export type { ConnectProviderRequest, ModelCatalog, ModelChoice, ModelInfo, ModelVisibility, ProviderCatalog } from "./models";

export const PROJECT_GIT_BRANCH_CHANGED_EVENT = "solomon:git-branch-changed";
export const PROJECTS_CHANGED_EVENT = "solomon:projects-changed";
const USER_NAME_CACHE_KEY = "solomon:user-name";
const HOME_STATS_CACHE_KEY = "solomon:home-stats";
const PROJECT_SIDEBAR_CACHE_KEY = "solomon:project-sidebar";

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

export function invalidateProjectSidebarDataCache(): void {
  projectSidebarCache = null;
  projectSidebarCacheNeedsRefresh = false;
}

if (typeof window !== "undefined") {
  window.addEventListener(PROJECTS_CHANGED_EVENT, () => invalidateProjectSidebarDataCache());
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
