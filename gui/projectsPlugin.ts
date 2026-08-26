import { readdir, readFile, rename, rm, stat, writeFile } from "node:fs/promises";
import type { Dirent } from "node:fs";
import { homedir } from "node:os";
import path from "node:path";
import type { Plugin } from "vite";
import {
  checkoutHomeDirectoryBranch,
  checkoutProjectBranch,
  homeDirectoryBranches,
  homeDirectoryWorktrees,
  projectBranches,
  projectGitHistory,
  projectGitStatus,
  projectWorktrees,
} from "./projectBranches";

const projectsEndpoint = "/__solomon/projects";
const homeDirectoryEntriesEndpoint = "/__solomon/home-directories";
const homeDirectoryBranchesEndpoint = "/__solomon/home-git-branches";
const homeDirectoryWorktreesEndpoint = "/__solomon/home-git-worktrees";
const homeDirectoryCheckoutEndpoint = "/__solomon/home-git-checkout";
const userNameEndpoint = "/__solomon/user-name";
const reasoningEffortEndpoint = "/__solomon/reasoning-effort";
const projectActionEndpoint = "/__solomon/projects/";
const reasoningEfforts = new Set(["none", "low", "medium", "high", "xhigh", "max"]);

type Project = {
  chats: SidebarChat[];
  id: string;
  name: string;
  path: string;
  chatCount: number;
  tokenStats: ProjectTokenStats;
};

type ProjectTokenStats = {
  user: number;
  reasoning: number;
  response: number;
  total: number;
};

type SidebarChat = Omit<Chat, "lastActivity">;

type Chat = {
  id: string;
  lastMessageAt: string;
  lastActivity: number;
  title: string;
};

type ProjectWithActivity = Project & { lastActivity: number };

function solomonHome() {
  return process.env.SOLOMON_HOME?.trim() || path.join(homedir(), ".solomon");
}

function projectDisplayName(projectPath: string): string {
  const resolvedHome = path.resolve(homedir());
  const resolvedProject = path.resolve(projectPath);
  if (resolvedProject === resolvedHome) return "Home";
  return path.basename(projectPath) || projectPath;
}

async function readUserName(home: string): Promise<string> {
  let config: string;
  try {
    config = await readFile(path.join(home, "config.toml"), "utf8");
  } catch {
    return "";
  }

  const firstTable = config.search(/^\s*\[/m);
  const rootConfig = config.slice(0, firstTable === -1 ? config.length : firstTable);
  const match = rootConfig.match(/^\s*user_name\s*=\s*((?:"(?:[^"\\]|\\.)*")|(?:'(?:[^']|'')*'))\s*(?:#.*)?$/m);
  if (!match) return "";
  const rawValue = match[1];
  if (rawValue.startsWith("'")) {
    return rawValue.slice(1, -1).replaceAll("''", "'").trim();
  }
  try {
    return String(JSON.parse(rawValue)).trim();
  } catch {
    return "";
  }
}

async function writeUserName(home: string, userName: string): Promise<void> {
  const configPath = path.join(home, "config.toml");
  const config = await readFile(configPath, "utf8");
  const firstTable = config.search(/^\s*\[/m);
  const boundary = firstTable === -1 ? config.length : firstTable;
  const rootConfig = config.slice(0, boundary);
  const tableConfig = config.slice(boundary);
  const serializedUserName = JSON.stringify(userName);
  const userNameLine = /^(\s*user_name\s*=\s*)((?:"(?:[^"\\]|\\.)*")|(?:'(?:[^']|'')*'))(\s*(?:#.*)?)$/m;
  const nextRootConfig = userNameLine.test(rootConfig)
    ? rootConfig.replace(userNameLine, (_match, prefix: string, _value: string, suffix: string) => `${prefix}${serializedUserName}${suffix}`)
    : `user_name = ${serializedUserName}\n${rootConfig.startsWith("\n") ? "" : "\n"}${rootConfig}`;
  const temporaryPath = `${configPath}.gui.tmp`;
  await writeFile(temporaryPath, `${nextRootConfig}${tableConfig}`, { encoding: "utf8", mode: 0o600 });
  await rename(temporaryPath, configPath);
}

function normalizeReasoningEffort(value: string): string {
  const normalized = value.trim().toLowerCase().replaceAll("_", "-").replace(/\s+/g, "-");
  if (normalized === "med") return "medium";
  if (normalized === "x-high" || normalized === "extra-high") return "xhigh";
  return reasoningEfforts.has(normalized) ? normalized : "";
}

async function readReasoningEffort(home: string): Promise<string> {
  let config: string;
  try {
    config = await readFile(path.join(home, "config.toml"), "utf8");
  } catch {
    return "none";
  }
  const firstTable = config.search(/^\s*\[/m);
  const rootConfig = config.slice(0, firstTable === -1 ? config.length : firstTable);
  const match = rootConfig.match(/^\s*reasoning_effort\s*=\s*((?:"(?:[^"\\]|\\.)*")|(?:'(?:[^']|'')*')|[A-Za-z]+)\s*(?:#.*)?$/m);
  if (!match) return "none";
  const rawValue = match[1];
  let parsed = rawValue;
  if (rawValue.startsWith("'")) parsed = rawValue.slice(1, -1).replaceAll("''", "'");
  else if (rawValue.startsWith('"')) {
    try {
      parsed = String(JSON.parse(rawValue));
    } catch {
      return "none";
    }
  }
  return normalizeReasoningEffort(parsed) || "none";
}

async function writeReasoningEffort(home: string, effort: string): Promise<void> {
  const configPath = path.join(home, "config.toml");
  const config = await readFile(configPath, "utf8");
  const firstTable = config.search(/^\s*\[/m);
  const boundary = firstTable === -1 ? config.length : firstTable;
  const rootConfig = config.slice(0, boundary);
  const tableConfig = config.slice(boundary);
  const serialized = JSON.stringify(effort);
  const effortLine = /^(\s*reasoning_effort\s*=\s*)((?:"(?:[^"\\]|\\.)*")|(?:'(?:[^']|'')*')|[A-Za-z]+)(\s*(?:#.*)?)$/m;
  const nextRootConfig = effortLine.test(rootConfig)
    ? rootConfig.replace(effortLine, (_match, prefix: string, _value: string, suffix: string) => `${prefix}${serialized}${suffix}`)
    : `reasoning_effort = ${serialized}\n${rootConfig.startsWith("\n") ? "" : "\n"}${rootConfig}`;
  const temporaryPath = `${configPath}.gui.tmp`;
  await writeFile(temporaryPath, `${nextRootConfig}${tableConfig}`, { encoding: "utf8", mode: 0o600 });
  await rename(temporaryPath, configPath);
}

async function readProjects(): Promise<Project[]> {
  const home = solomonHome();
  let projectMap: unknown;

  try {
    projectMap = JSON.parse(await readFile(path.join(home, "projectsId.json"), "utf8"));
  } catch {
    return [];
  }
  if (!projectMap || typeof projectMap !== "object" || Array.isArray(projectMap)) return [];

  const folders = await Promise.all(
    Object.entries(projectMap).flatMap(([projectPath, projectID]) => {
      if (typeof projectID !== "string" || !projectID) return [];
      return [
        (async (): Promise<ProjectWithActivity> => {
          let chats: Chat[] = [];
          let lastActivity = 0;
          const projectDirectory = path.join(home, "projects", projectID);
          try {
            const chatDirectory = path.join(projectDirectory, "chats");
            const files = (await readdir(chatDirectory)).filter((file) => file.endsWith(".json"));
            chats = (await Promise.all(files.map((file) => readChat(chatDirectory, file))))
              .filter((chat): chat is Chat => chat !== null)
              .sort((left, right) => right.lastActivity - left.lastActivity);
            lastActivity = Math.max(0, ...chats.map((chat) => chat.lastActivity));
          } catch {
            // A registered project can legitimately have no chat directory yet.
            try {
              lastActivity = (await stat(projectDirectory)).mtimeMs;
            } catch {
              // The project record can outlive a removed project directory.
            }
          }
          return {
            id: projectID,
            name: projectDisplayName(projectPath),
            path: projectPath,
            chatCount: chats.length,
            tokenStats: await readProjectTokenStats(projectDirectory),
            chats: chats.map(({ lastActivity: _, ...chat }) => chat),
            lastActivity,
          };
        })(),
      ];
    }),
  );

  return folders
    .sort((left, right) => right.lastActivity - left.lastActivity || left.name.localeCompare(right.name))
    .map(({ lastActivity: _, ...folder }) => folder);
}

async function readChat(chatDirectory: string, fileName: string): Promise<Chat | null> {
  const chatPath = path.join(chatDirectory, fileName);
  try {
    const [source, fileInfo] = await Promise.all([readFile(chatPath, "utf8"), stat(chatPath)]);
    const payload: unknown = JSON.parse(source);
    const record = payload && typeof payload === "object" ? payload as Record<string, unknown> : {};
    const id = typeof record.id === "string" && record.id ? record.id : path.basename(fileName, ".json");
    const title = typeof record.title === "string" && record.title.trim() ? record.title.trim() : "Untitled chat";
    const storedLastMessageAt = typeof record.last_message_at === "string" && record.last_message_at
      ? record.last_message_at
      : fileInfo.mtime.toISOString();
    const parsedLastMessageAt = Date.parse(storedLastMessageAt);
    const lastActivity = Number.isNaN(parsedLastMessageAt) || parsedLastMessageAt <= 0
      ? fileInfo.mtimeMs
      : parsedLastMessageAt;
    return {
      id,
      lastActivity,
      lastMessageAt: lastActivity === fileInfo.mtimeMs ? fileInfo.mtime.toISOString() : storedLastMessageAt,
      title,
    };
  } catch {
    return null;
  }
}

async function readProjectTokenStats(projectDirectory: string): Promise<ProjectTokenStats> {
  try {
    const payload: unknown = JSON.parse(await readFile(path.join(projectDirectory, "welcome_stats.json"), "utf8"));
    if (payload && typeof payload === "object" && "chats" in payload && payload.chats && typeof payload.chats === "object" && !Array.isArray(payload.chats)) {
      const stats = Object.values(payload.chats).reduce((result, chat) => {
        if (!chat || typeof chat !== "object") return result;
        const values = chat as Record<string, unknown>;
        result.user += nonNegativeNumber(values.user_sum);
        result.reasoning += nonNegativeNumber(values.reason_sum);
        result.response += nonNegativeNumber(values.resp_sum);
        return result;
      }, emptyProjectTokenStats());
      return withTokenTotal(stats);
    }
  } catch {
    // Older projects may not have a materialized stats file yet.
  }

  let files: string[];
  try {
    files = (await readdir(path.join(projectDirectory, "chats"))).filter((file) => file.endsWith(".json"));
  } catch {
    return emptyProjectTokenStats();
  }
  const estimates = await Promise.all(files.map(async (file) => {
    try {
      return approximateSessionTokenStats(JSON.parse(await readFile(path.join(projectDirectory, "chats", file), "utf8")));
    } catch {
      return emptyProjectTokenStats();
    }
  }));
  return withTokenTotal(estimates.reduce((total, estimate) => ({
    user: total.user + estimate.user,
    reasoning: total.reasoning + estimate.reasoning,
    response: total.response + estimate.response,
    total: 0,
  }), emptyProjectTokenStats()));
}

function approximateSessionTokenStats(payload: unknown): ProjectTokenStats {
  if (!payload || typeof payload !== "object" || !("messages" in payload) || !Array.isArray(payload.messages)) return emptyProjectTokenStats();
  const messages = payload.messages.filter((message): message is Record<string, unknown> => Boolean(message && typeof message === "object"));
  const hasStoredUsage = messages.some((message) => (
    message.role === "assistant"
      && (nonNegativeNumber(message.turn_total_tokens) > 0
        || nonNegativeNumber(message.user_prompt_tokens) > 0
        || nonNegativeNumber(message.reasoning_tokens) > 0
        || nonNegativeNumber(message.response_tokens) > 0)
  ));
  const stats = messages.reduce<ProjectTokenStats>((result, record) => {
    const role = typeof record.role === "string" ? record.role : "";
    const storedTotal = nonNegativeNumber(record.turn_total_tokens);
    const storedUser = nonNegativeNumber(record.user_prompt_tokens);
    const storedReasoning = nonNegativeNumber(record.reasoning_tokens);
    const storedResponse = nonNegativeNumber(record.response_tokens);
    const storedParts = storedUser + storedReasoning + storedResponse;
    if (role === "assistant" && (storedTotal > 0 || storedParts > 0)) {
      result.user += storedUser;
      result.reasoning += storedReasoning;
      result.response += storedResponse || (storedParts === 0 ? storedTotal : 0);
      return result;
    }
    if (role === "user" && hasStoredUsage) return result;
    const reasoningText = typeof record.reasoning_text === "string" ? record.reasoning_text : "";
    const responseText = [record.content, record.api_content]
      .filter((value): value is string => typeof value === "string")
      .join(" ");
    if (role === "user") result.user += approximateTextTokens(responseText);
    else if (reasoningText) {
      result.reasoning += approximateTextTokens(reasoningText);
      result.response += approximateTextTokens(responseText);
    } else {
      result.response += approximateTextTokens(responseText);
    }
    return result;
  }, emptyProjectTokenStats());
  return withTokenTotal(stats);
}

function emptyProjectTokenStats(): ProjectTokenStats {
  return { user: 0, reasoning: 0, response: 0, total: 0 };
}

function withTokenTotal(stats: ProjectTokenStats): ProjectTokenStats {
  return { ...stats, total: stats.user + stats.reasoning + stats.response };
}

function approximateTextTokens(value: string): number {
  return Math.ceil(value.length / 4);
}

function nonNegativeNumber(value: unknown): number {
  return typeof value === "number" && Number.isFinite(value) ? Math.max(0, Math.round(value)) : 0;
}

function attachProjectsEndpoint(server: { middlewares: { use: (route: string, handler: (request: { method?: string }, response: { end: (body: string) => void; setHeader: (name: string, value: string) => void; statusCode: number }, next: () => void) => void) => void } }) {
  server.middlewares.use(projectsEndpoint, (request, response, next) => {
    if (request.method !== "GET") {
      next();
      return;
    }
    const home = solomonHome();
    void Promise.all([readProjects(), readUserName(home), readReasoningEffort(home)])
      .then(([projects, userName, reasoningEffort]) => {
        response.statusCode = 200;
        response.setHeader("Content-Type", "application/json; charset=utf-8");
        response.end(JSON.stringify({ projects, userName, reasoningEffort }));
      })
      .catch(() => {
        response.statusCode = 500;
        response.setHeader("Content-Type", "application/json; charset=utf-8");
        response.end(JSON.stringify({ projects: [], userName: "", reasoningEffort: "none" }));
      });
  });
}

function attachHomeDirectoryEntriesEndpoint(server: { middlewares: { use: (route: string, handler: (request: UserNameRequest, response: UserNameResponse, next: () => void) => void) => void } }) {
  server.middlewares.use(homeDirectoryEntriesEndpoint, (request, response, next) => {
    if (request.method !== "GET") {
      next();
      return;
    }
    const directoryPath = new URL(request.url ?? "", "http://solomon.local").searchParams.get("path") ?? "";
    void homeDirectoryEntries(directoryPath)
      .then((entries) => respondWithJson(response, 200, entries))
      .catch(() => respondWithJson(response, 500, { error: "Unable to read home directories" }));
  });
}

function attachHomeGitEndpoints(server: { middlewares: { use: (route: string, handler: (request: UserNameRequest, response: UserNameResponse, next: () => void) => void) => void } }) {
  server.middlewares.use(homeDirectoryBranchesEndpoint, (request, response, next) => {
    if (request.method !== "GET") {
      next();
      return;
    }
    const directoryPath = new URL(request.url ?? "", "http://solomon.local").searchParams.get("path") ?? "";
    void homeDirectoryBranches(directoryPath)
      .then((info) => respondWithJson(response, 200, info))
      .catch(() => respondWithJson(response, 500, { error: "Unable to read home directory branches" }));
  });

  server.middlewares.use(homeDirectoryWorktreesEndpoint, (request, response, next) => {
    if (request.method !== "GET") {
      next();
      return;
    }
    const directoryPath = new URL(request.url ?? "", "http://solomon.local").searchParams.get("path") ?? "";
    void homeDirectoryWorktrees(directoryPath)
      .then((info) => respondWithJson(response, 200, info))
      .catch(() => respondWithJson(response, 500, { error: "Unable to read home directory worktrees" }));
  });

  server.middlewares.use(homeDirectoryCheckoutEndpoint, (request, response, next) => {
    if (request.method !== "POST") {
      next();
      return;
    }
    const directoryPath = new URL(request.url ?? "", "http://solomon.local").searchParams.get("path") ?? "";
    void readJsonBody(request)
      .then(async (payload) => {
        if (!payload || typeof payload !== "object" || !("branch" in payload) || typeof payload.branch !== "string") {
          respondWithJson(response, 400, { error: "branch must be a string" });
          return;
        }
        respondWithJson(response, 200, await checkoutHomeDirectoryBranch(directoryPath, payload.branch));
      })
      .catch(() => respondWithJson(response, 500, { error: "Unable to checkout home directory branch" }));
  });
}

async function removeProject(projectID: string, removeData: boolean) {
  if (!/^[a-f0-9]{64}$/.test(projectID)) throw new Error("Invalid project ID");
  const home = solomonHome();
  const mapPath = path.join(home, "projectsId.json");
  const rawMap: unknown = JSON.parse(await readFile(mapPath, "utf8"));
  if (!rawMap || typeof rawMap !== "object" || Array.isArray(rawMap)) throw new Error("Invalid projects map");

  const projectMap = rawMap as Record<string, unknown>;
  const registeredPaths = Object.entries(projectMap)
    .filter(([, registeredID]) => registeredID === projectID)
    .map(([projectPath]) => projectPath);
  if (!registeredPaths.length) throw new Error("Project is not registered");

  if (removeData) {
    const resolvedHomeDirectory = path.resolve(homedir());
    for (const projectPath of registeredPaths) {
      const resolvedProjectPath = path.resolve(projectPath);
      if (resolvedProjectPath === path.parse(resolvedProjectPath).root || resolvedProjectPath === resolvedHomeDirectory) {
        throw new Error("Refusing to remove a critical directory");
      }
      await rm(resolvedProjectPath, { force: true, recursive: true });
    }
    await rm(path.join(home, "projects", projectID), { force: true, recursive: true });
  }
  for (const projectPath of registeredPaths) delete projectMap[projectPath];
  const temporaryMapPath = `${mapPath}.gui.tmp`;
  await writeFile(temporaryMapPath, `${JSON.stringify(projectMap, null, 2)}\n`, { encoding: "utf8", mode: 0o600 });
  await rename(temporaryMapPath, mapPath);
}

async function directorySize(directory: string): Promise<number> {
  let entries: Dirent<string>[];
  try {
    entries = await readdir(directory, { withFileTypes: true });
  } catch (error: unknown) {
    return 0;
  }
  const sizes = await Promise.all(entries.map(async (entry) => {
    try {
      const entryPath = path.join(directory, entry.name);
      if (entry.isSymbolicLink()) return 0;
      if (entry.isDirectory()) return directorySize(entryPath);
      return (await stat(entryPath)).size;
    } catch {
      return 0;
    }
  }));
  return sizes.reduce((total, size) => total + size, 0);
}

async function projectRemovalInfo(projectID: string) {
  if (!/^[a-f0-9]{64}$/.test(projectID)) throw new Error("Invalid project ID");
  const home = solomonHome();
  const rawMap: unknown = JSON.parse(await readFile(path.join(home, "projectsId.json"), "utf8"));
  if (!rawMap || typeof rawMap !== "object" || Array.isArray(rawMap)) throw new Error("Invalid projects map");
  const projectPath = Object.entries(rawMap).find(([, registeredID]) => registeredID === projectID)?.[0];
  if (!projectPath) throw new Error("Project is not registered");
  const absoluteProjectPath = path.resolve(projectPath);
  const dataPath = path.join(home, "projects", projectID);
  const [projectSizeBytes, dataSizeBytes] = await Promise.all([directorySize(absoluteProjectPath), directorySize(dataPath)]);
  return { projectPath: absoluteProjectPath, projectSizeBytes, dataPath, dataSizeBytes };
}

async function projectDirectoryEntries(projectID: string, directoryPath: string) {
  if (!/^[a-f0-9]{64}$/.test(projectID)) throw new Error("Invalid project ID");
  const rawMap: unknown = JSON.parse(await readFile(path.join(solomonHome(), "projectsId.json"), "utf8"));
  if (!rawMap || typeof rawMap !== "object" || Array.isArray(rawMap)) throw new Error("Invalid projects map");
  const projectPath = Object.entries(rawMap).find(([, registeredID]) => registeredID === projectID)?.[0];
  if (!projectPath) throw new Error("Project is not registered");

  const root = path.resolve(projectPath);
  const target = path.resolve(root, directoryPath);
  const relativeTarget = path.relative(root, target);
  if (path.isAbsolute(relativeTarget) || relativeTarget === ".." || relativeTarget.startsWith(`..${path.sep}`)) {
    throw new Error("Invalid project directory");
  }

  const entries = await readdir(target, { withFileTypes: true });
  return entries
    .map((entry) => ({
      isDirectory: entry.isDirectory(),
      name: entry.name,
      path: relativeTarget ? path.join(relativeTarget, entry.name) : entry.name,
    }))
    .sort((left, right) => Number(right.isDirectory) - Number(left.isDirectory) || left.name.localeCompare(right.name));
}

async function homeDirectoryEntries(directoryPath: string) {
  const root = path.resolve(homedir());
  const target = path.resolve(root, directoryPath);
  const relativeTarget = path.relative(root, target);
  if (path.isAbsolute(relativeTarget) || relativeTarget === ".." || relativeTarget.startsWith(`..${path.sep}`)) {
    throw new Error("Invalid home directory");
  }

  const entries = await readdir(target, { withFileTypes: true });
  return entries
    .map((entry) => ({
      isDirectory: entry.isDirectory(),
      name: entry.name,
      path: relativeTarget ? path.join(relativeTarget, entry.name) : entry.name,
    }))
    .sort((left, right) => Number(right.isDirectory) - Number(left.isDirectory) || left.name.localeCompare(right.name));
}

async function projectResearch(projectID: string) {
  if (!/^[a-f0-9]{64}$/.test(projectID)) throw new Error("Invalid project ID");
  const home = solomonHome();
  const rawMap: unknown = JSON.parse(await readFile(path.join(home, "projectsId.json"), "utf8"));
  if (!rawMap || typeof rawMap !== "object" || Array.isArray(rawMap)) throw new Error("Invalid projects map");
  if (!Object.values(rawMap).includes(projectID)) throw new Error("Project is not registered");
  let files: string[];
  try {
    files = (await readdir(path.join(home, "projects", projectID, "research"))).filter((file) => file.endsWith(".json"));
  } catch {
    return [];
  }
  const jobs = await Promise.all(files.map(async (file) => {
    try {
      const payload: unknown = JSON.parse(await readFile(path.join(home, "projects", projectID, "research", file), "utf8"));
      return payload && typeof payload === "object" && !Array.isArray(payload) ? payload : null;
    } catch {
      return null;
    }
  }));
  return jobs.filter((job): job is Record<string, unknown> => job !== null)
    .sort((left, right) => String(right.finished_at ?? right.started_at ?? "").localeCompare(String(left.finished_at ?? left.started_at ?? "")));
}

async function projectResearchReport(projectID: string, researchID: string) {
  const jobs = await projectResearch(projectID);
  const job = jobs.find((entry) => entry.id === researchID);
  if (!job || typeof job.slug !== "string" || !/^[a-z0-9][a-z0-9-]*$/i.test(job.slug)) throw new Error("Research report not found");
  return readFile(path.join(solomonHome(), "projects", projectID, "research", `${job.slug}.html`), "utf8");
}

function attachProjectActionEndpoint(server: { middlewares: { use: (route: string, handler: (request: UserNameRequest, response: UserNameResponse, next: () => void) => void) => void } }) {
  server.middlewares.use(projectActionEndpoint, (request, response, next) => {
    const route = request.url?.split("?")[0] ?? "";
    const reportMatch = route.match(/^\/?([a-f0-9]{64})\/research\/([^/]+)\/report\/?$/);
    if (request.method === "GET" && reportMatch) {
      void projectResearchReport(reportMatch[1], decodeURIComponent(reportMatch[2]))
        .then((report) => { response.statusCode = 200; response.setHeader("Content-Type", "text/html; charset=utf-8"); response.end(report); })
        .catch(() => { response.statusCode = 404; response.end("Research report not found"); });
      return;
    }
    const match = route.match(/^\/?([a-f0-9]{64})(?:\/(disk|removal-info|files|research|history|status|branches|checkout|worktrees))?\/?$/);
    if (request.method === "GET" && match?.[2] === "files") {
      const directoryPath = new URL(request.url ?? "", "http://solomon.local").searchParams.get("path") ?? "";
      void projectDirectoryEntries(match[1], directoryPath)
        .then((entries) => respondWithJson(response, 200, entries))
        .catch(() => respondWithJson(response, 500, { error: "Unable to read project files" }));
      return;
    }
    if (request.method === "GET" && match?.[2] === "research") {
      void projectResearch(match[1])
        .then((jobs) => respondWithJson(response, 200, jobs))
        .catch(() => respondWithJson(response, 500, { error: "Unable to read project research" }));
      return;
    }
    if (request.method === "GET" && match?.[2] === "history") {
      void projectGitHistory(match[1])
        .then((history) => respondWithJson(response, 200, history))
        .catch(() => respondWithJson(response, 500, { error: "Unable to read project Git history" }));
      return;
    }
    if (request.method === "GET" && match?.[2] === "status") {
      void projectGitStatus(match[1])
        .then((status) => respondWithJson(response, 200, status))
        .catch(() => respondWithJson(response, 500, { error: "Unable to read project Git status" }));
      return;
    }
    if (request.method === "GET" && match?.[2] === "branches") {
      void projectBranches(match[1])
        .then((info) => respondWithJson(response, 200, info))
        .catch(() => respondWithJson(response, 500, { error: "Unable to read project branches" }));
      return;
    }
    if (request.method === "GET" && match?.[2] === "worktrees") {
      void projectWorktrees(match[1])
        .then((info) => respondWithJson(response, 200, info))
        .catch(() => respondWithJson(response, 500, { error: "Unable to read project worktrees" }));
      return;
    }
    if (request.method === "POST" && match?.[2] === "checkout") {
      void readJsonBody(request)
        .then(async (payload) => {
          if (!payload || typeof payload !== "object" || !("branch" in payload) || typeof payload.branch !== "string") {
            respondWithJson(response, 400, { error: "branch must be a string" });
            return;
          }
          respondWithJson(response, 200, await checkoutProjectBranch(match[1], payload.branch));
        })
        .catch(() => respondWithJson(response, 500, { error: "Unable to checkout branch" }));
      return;
    }
    if (request.method === "GET" && match?.[2] === "removal-info") {
      void projectRemovalInfo(match[1])
        .then((info) => respondWithJson(response, 200, info))
        .catch(() => respondWithJson(response, 500, { error: "Unable to read project details" }));
      return;
    }
    if (request.method !== "DELETE") {
      next();
      return;
    }
    if (!match || match[2] === "removal-info" || match[2] === "files" || match[2] === "research" || match[2] === "history" || match[2] === "status" || match[2] === "branches" || match[2] === "checkout" || match[2] === "worktrees") {
      respondWithJson(response, 400, { error: "Invalid project ID" });
      return;
    }
    void removeProject(match[1], match[2] === "disk")
      .then(() => respondWithJson(response, 200, {}))
      .catch(() => respondWithJson(response, 500, { error: "Unable to remove project" }));
  });
}

type UserNameRequest = {
  method?: string;
  url?: string;
  on: (event: "data", listener: (chunk: string | Uint8Array) => void) => void;
  once(event: "error", listener: () => void): void;
  once(event: "end", listener: () => void): void;
};

type UserNameResponse = {
  end: (body: string) => void;
  setHeader: (name: string, value: string) => void;
  statusCode: number;
};

function respondWithJson(response: UserNameResponse, statusCode: number, body: object) {
  response.statusCode = statusCode;
  response.setHeader("Content-Type", "application/json; charset=utf-8");
  response.end(JSON.stringify(body));
}

function readJsonBody(request: UserNameRequest): Promise<unknown> {
  return new Promise((resolve, reject) => {
    let body = "";
    request.on("data", (chunk) => {
      body += String(chunk);
      if (body.length > 2048) reject(new Error("Request body is too large"));
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

function attachUserNameEndpoint(server: { middlewares: { use: (route: string, handler: (request: UserNameRequest, response: UserNameResponse, next: () => void) => void) => void } }) {
  server.middlewares.use(userNameEndpoint, (request, response, next) => {
    if (request.method !== "PUT") {
      next();
      return;
    }
    void readJsonBody(request)
      .then(async (payload) => {
        if (!payload || typeof payload !== "object" || !("userName" in payload) || typeof payload.userName !== "string") {
          respondWithJson(response, 400, { error: "userName must be a string" });
          return;
        }
        const userName = payload.userName.trim();
        if (userName.length > 120) {
          respondWithJson(response, 400, { error: "userName is too long" });
          return;
        }
        await writeUserName(solomonHome(), userName);
        respondWithJson(response, 200, { userName });
      })
      .catch(() => respondWithJson(response, 500, { error: "Unable to save user name" }));
  });
}

function attachReasoningEffortEndpoint(server: { middlewares: { use: (route: string, handler: (request: UserNameRequest, response: UserNameResponse, next: () => void) => void) => void } }) {
  server.middlewares.use(reasoningEffortEndpoint, (request, response, next) => {
    if (request.method !== "PUT") {
      next();
      return;
    }
    void readJsonBody(request)
      .then(async (payload) => {
        if (!payload || typeof payload !== "object" || !("reasoningEffort" in payload) || typeof payload.reasoningEffort !== "string") {
          respondWithJson(response, 400, { error: "reasoningEffort must be a string" });
          return;
        }
        const reasoningEffort = normalizeReasoningEffort(payload.reasoningEffort);
        if (!reasoningEffort) {
          respondWithJson(response, 400, { error: "reasoning must be none, low, med, medium, high, xhigh (extra high), or max" });
          return;
        }
        await writeReasoningEffort(solomonHome(), reasoningEffort);
        respondWithJson(response, 200, { reasoningEffort });
      })
      .catch(() => respondWithJson(response, 500, { error: "Unable to save reasoning effort" }));
  });
}

export function projectsPlugin(): Plugin {
  return {
    configurePreviewServer(server) {
      attachProjectActionEndpoint(server);
      attachProjectsEndpoint(server);
      attachHomeDirectoryEntriesEndpoint(server);
      attachHomeGitEndpoints(server);
      attachUserNameEndpoint(server);
      attachReasoningEffortEndpoint(server);
    },
    configureServer(server) {
      attachProjectActionEndpoint(server);
      attachProjectsEndpoint(server);
      attachHomeDirectoryEntriesEndpoint(server);
      attachHomeGitEndpoints(server);
      attachUserNameEndpoint(server);
      attachReasoningEffortEndpoint(server);
    },
    name: "solomon-projects",
  };
}
