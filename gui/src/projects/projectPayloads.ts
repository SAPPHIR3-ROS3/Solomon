import type {
  Project,
  ProjectAtMentionSuggestion,
  ProjectBranches,
  ProjectChatSummary,
  ProjectDirectoryEntry,
  ProjectGitCommit,
  ProjectGitHistory,
  ProjectGitStatus,
  ProjectRemovalInfo,
  ProjectResearch,
  ProjectSidebarData,
  ProjectWorktrees,
} from "./projects";
import { normalizeReasoningEffort } from "./projects";

export function projectSidebarDataFromPayload(payload: unknown): ProjectSidebarData {
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

export function projectRemovalInfoFromPayload(payload: unknown): ProjectRemovalInfo {
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

export function projectDirectoryEntriesFromPayload(payload: unknown): ProjectDirectoryEntry[] {
  if (!Array.isArray(payload)) return [];
  return payload.flatMap((entry) => {
    if (!entry || typeof entry !== "object") return [];
    const name = "name" in entry && typeof entry.name === "string" ? entry.name : "";
    const path = "path" in entry && typeof entry.path === "string" ? entry.path : "";
    if (!name || !path) return [];
    return [{ isDirectory: "isDirectory" in entry && Boolean(entry.isDirectory), name, path }];
  });
}

export function atMentionSuggestionsFromPayload(payload: unknown): ProjectAtMentionSuggestion[] {
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

export function projectResearchFromPayload(payload: unknown): ProjectResearch[] {
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

export function projectBranchesFromPayload(payload: unknown): ProjectBranches {
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

export function projectGitHistoryFromPayload(payload: unknown): ProjectGitHistory {
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

export function projectGitStatusFromPayload(payload: unknown): ProjectGitStatus {
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

export function projectWorktreesFromPayload(payload: unknown): ProjectWorktrees {
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

export function isProject(value: unknown): value is Project {
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
