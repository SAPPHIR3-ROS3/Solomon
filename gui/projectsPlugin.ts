import { mkdir, readdir, readFile, realpath, rename, rm, stat, writeFile } from "node:fs/promises";
import { createHash } from "node:crypto";
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
const chatAPIPath = projectsEndpoint;
const chatProxyHeader = "x-solomon-chat-proxy";
const homeDirectoryEntriesEndpoint = "/__solomon/home-directories";
const homeDirectoryBranchesEndpoint = "/__solomon/home-git-branches";
const homeDirectoryWorktreesEndpoint = "/__solomon/home-git-worktrees";
const homeDirectoryCheckoutEndpoint = "/__solomon/home-git-checkout";
const userNameEndpoint = "/__solomon/user-name";
const reasoningEffortEndpoint = "/__solomon/reasoning-effort";
const fastModeEndpoint = "/__solomon/fast-mode";
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

function normalizeReasoningEffort(value: string): string {
  const normalized = value.trim().toLowerCase().replaceAll("_", "-").replace(/\s+/g, "-");
  if (normalized === "med") return "medium";
  if (normalized === "x-high" || normalized === "extra-high") return "xhigh";
  return reasoningEfforts.has(normalized) ? normalized : "";
}

function rootConfigParts(source: string) {
  const firstTable = source.search(/^\s*\[/m);
  const boundary = firstTable === -1 ? source.length : firstTable;
  return { root: source.slice(0, boundary), tables: source.slice(boundary) };
}

function rootValuePattern(key: string): RegExp {
  const escapedKey = key.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  return new RegExp(`^(\\s*${escapedKey}\\s*=\\s*)((?:"(?:[^"\\\\]|\\\\.)*")|(?:'(?:[^']|'')*')|[^\\s#]+)(\\s*(?:#.*)?)$`, "m");
}

function parseQuotedString(value: string): string {
  const trimmed = value.trim();
  if (trimmed.startsWith('"') && trimmed.endsWith('"')) {
    try {
      return JSON.parse(trimmed) as string;
    } catch {
      return trimmed.slice(1, -1);
    }
  }
  if (trimmed.startsWith("'") && trimmed.endsWith("'")) {
    return trimmed.slice(1, -1).replaceAll("''", "'");
  }
  return trimmed;
}

function parseRootValue(raw: string): string {
  return parseQuotedString(raw).trim();
}

async function readRootValue(home: string, key: string): Promise<string | undefined> {
  try {
    const source = await readFile(path.join(home, "config.toml"), "utf8");
    const match = rootConfigParts(source).root.match(rootValuePattern(key));
    return match ? parseRootValue(match[1]) : undefined;
  } catch {
    return undefined;
  }
}

async function writeRootValue(home: string, key: string, value: string | boolean): Promise<void> {
  const configPath = path.join(home, "config.toml");
  const source = await readFile(configPath, "utf8");
  const { root, tables } = rootConfigParts(source);
  const serialized = typeof value === "boolean" ? String(value) : JSON.stringify(value);
  const pattern = rootValuePattern(key);
  const nextRoot = pattern.test(root)
    ? root.replace(pattern, (_match, prefix: string, _oldValue: string, suffix: string) => `${prefix}${serialized}${suffix}`)
    : `${key} = ${serialized}\n${root.startsWith("\n") ? "" : "\n"}${root}`;
  const temporaryPath = `${configPath}.gui.tmp`;
  await writeFile(temporaryPath, `${nextRoot}${tables}`, { encoding: "utf8", mode: 0o600 });
  await rename(temporaryPath, configPath);
}

async function readUserName(home: string): Promise<string> {
  return (await readRootValue(home, "user_name")) ?? "";
}

async function writeUserName(home: string, userName: string): Promise<void> {
  await writeRootValue(home, "user_name", userName);
}

async function readReasoningEffort(home: string): Promise<string> {
  return normalizeReasoningEffort((await readRootValue(home, "reasoning_effort")) ?? "") || "none";
}

async function writeReasoningEffort(home: string, effort: string): Promise<void> {
  await writeRootValue(home, "reasoning_effort", effort);
}

async function readFastMode(home: string): Promise<boolean> {
  return (await readRootValue(home, "fast_mode")) !== "false";
}

async function writeFastMode(home: string, enabled: boolean): Promise<void> {
  await writeRootValue(home, "fast_mode", enabled);
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

async function registerProject(rawPath: string): Promise<Project> {
  const homeDirectory = path.resolve(homedir());
  const trimmedPath = rawPath.trim();
  const projectPath = !trimmedPath || trimmedPath === "~" || trimmedPath === "~/"
    ? homeDirectory
    : path.resolve(trimmedPath.startsWith("~/")
      ? path.join(homeDirectory, trimmedPath.slice(2))
      : path.isAbsolute(trimmedPath) ? trimmedPath : path.join(homeDirectory, trimmedPath));
  if (!(await stat(projectPath)).isDirectory()) throw new Error("Project path is not a directory");
  const canonicalProjectPath = await realpath(projectPath);

  const home = solomonHome();
  const mapPath = path.join(home, "projectsId.json");
  let projectMap: Record<string, unknown> = {};
  try {
    const payload: unknown = JSON.parse(await readFile(mapPath, "utf8"));
    if (payload && typeof payload === "object" && !Array.isArray(payload)) projectMap = payload as Record<string, unknown>;
  } catch {
    // The first project creates the map.
  }
  const mappedProjectID = projectMap[canonicalProjectPath];
  const projectID = typeof mappedProjectID === "string" && mappedProjectID.length === 64
    ? mappedProjectID
    : createHash("sha256").update(canonicalProjectPath).digest("hex");
  projectMap[canonicalProjectPath] = projectID;
  await mkdir(path.join(home, "projects", projectID, "chats", "subchats"), { recursive: true, mode: 0o700 });
  await mkdir(path.join(home, "projects", projectID, "chats", "images"), { recursive: true, mode: 0o700 });
  await mkdir(path.join(home, "projects", projectID, "plans"), { recursive: true, mode: 0o700 });
  await mkdir(path.join(home, "projects", projectID, "research"), { recursive: true, mode: 0o700 });
  await mkdir(path.join(home, "projects", projectID, "skills"), { recursive: true, mode: 0o700 });
  await mkdir(path.dirname(mapPath), { recursive: true, mode: 0o700 });
  const temporaryMapPath = `${mapPath}.gui.tmp`;
  await writeFile(temporaryMapPath, `${JSON.stringify(projectMap, null, 2)}\n`, { encoding: "utf8", mode: 0o600 });
  await rename(temporaryMapPath, mapPath);

  const project = (await readProjects()).find((candidate) => candidate.id === projectID);
  if (!project) throw new Error("Unable to read the registered project");
  return project;
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

function attachProjectsEndpoint(server: { middlewares: { use: (route: string, handler: (request: UserNameRequest, response: UserNameResponse, next: () => void) => void) => void } }) {
  server.middlewares.use(projectsEndpoint, (request, response, next) => {
    // Connect mounts middleware on a prefix and strips that prefix from the
    // request URL. Nested chat endpoints share the same prefix, so only the
    // exact project collection route belongs to this fallback.
    const route = request.url?.split("?")[0] ?? "";
    if (route !== "" && route !== "/") {
      next();
      return;
    }
    if (request.method === "POST") {
      void readJsonBody(request)
        .then(async (payload) => {
          if (!payload || typeof payload !== "object" || !("path" in payload) || typeof payload.path !== "string") {
            respondWithJson(response, 400, { error: "path must be a string" });
            return;
          }
          respondWithJson(response, 201, { project: await registerProject(payload.path) });
        })
        .catch((error: unknown) => respondWithJson(response, 400, { error: error instanceof Error ? error.message : "Unable to create project" }));
      return;
    }
    if (request.method !== "GET") {
      next();
      return;
    }
    const home = solomonHome();
    void Promise.all([readProjects(), readUserName(home), readReasoningEffort(home), readFastMode(home)])
      .then(([projects, userName, reasoningEffort, fastMode]) => {
        response.statusCode = 200;
        response.setHeader("Content-Type", "application/json; charset=utf-8");
        response.end(JSON.stringify({ fastMode, projects, userName, reasoningEffort }));
      })
      .catch(() => {
        response.statusCode = 500;
        response.setHeader("Content-Type", "application/json; charset=utf-8");
        response.end(JSON.stringify({ fastMode: true, projects: [], userName: "", reasoningEffort: "none" }));
      });
  });
}

function attachChatAPIProxy(server: { middlewares: { use: (route: string, handler: (request: UserNameRequest, response: UserNameResponse, next: () => void) => void) => void } }) {
  server.middlewares.use(projectActionEndpoint, (request, response, next) => {
    const rawURL = request.url ?? "";
    const queryIndex = rawURL.indexOf("?");
    const rawPath = queryIndex < 0 ? rawURL : rawURL.slice(0, queryIndex);
    const query = queryIndex < 0 ? "" : rawURL.slice(queryIndex + 1);
    const normalizedPath = rawPath.startsWith("/") ? rawPath : `/${rawPath}`;
    const relativePath = normalizedPath.startsWith(`${chatAPIPath}/`)
      ? normalizedPath.slice(chatAPIPath.length)
      : normalizedPath;
    if (!/^\/[a-f0-9]{64}\/(?:chats|subchats|at-mentions)(?:\/|$)/.test(relativePath)) {
      next();
      return;
    }

    void proxyChatAPIRequest(request, response, `${chatAPIPath}${relativePath}${query ? `?${query}` : ""}`)
      .catch((error: unknown) => respondWithJson(response, 502, { error: error instanceof Error ? error.message : "Solomon daemon is unavailable" }));
  });
}

async function proxyChatAPIRequest(request: UserNameRequest, response: UserNameResponse, targetPath: string): Promise<void> {
  if (requestHeader(request, chatProxyHeader) === "1") {
    respondWithJson(response, 502, { error: "Solomon daemon chat API is unavailable; restart the daemon" });
    return;
  }
  const daemonURL = await readDaemonURL();
  if (!daemonURL) {
    respondWithJson(response, 502, { error: "Solomon daemon is unavailable" });
    return;
  }

  const method = (request.method ?? "GET").toUpperCase();
  const body = method === "GET" || method === "HEAD" ? undefined : await readRequestBody(request, 32 << 20);
  const headers: Record<string, string> = {};
  const contentType = requestHeader(request, "content-type");
  const accept = requestHeader(request, "accept");
  if (contentType) headers["content-type"] = contentType;
  if (accept) headers.accept = accept;
  headers[chatProxyHeader] = "1";

  let upstream: Response;
  const requestController = new AbortController();
  let timedOut = false;
  const timeoutID = setTimeout(() => {
    timedOut = true;
    requestController.abort();
  }, 15_000);
  try {
    upstream = await fetch(`${daemonURL}${targetPath}`, {
      body: body?.length ? Buffer.from(body) : undefined,
      headers,
      method,
      signal: requestController.signal,
    });
  } catch {
    respondWithJson(response, timedOut ? 504 : 502, { error: timedOut ? "Solomon daemon did not respond" : "Solomon daemon is unavailable" });
    return;
  } finally {
    clearTimeout(timeoutID);
  }

  response.statusCode = upstream.status;
  upstream.headers.forEach((value, name) => {
    if (name !== "connection" && name !== "content-length" && name !== "transfer-encoding") response.setHeader(name, value);
  });
  if (!upstream.body || !response.write) {
    response.end(await upstream.text());
    return;
  }

  const reader = upstream.body.getReader();
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      response.write(value);
    }
  } finally {
    response.end("");
  }
}

async function readDaemonURL(): Promise<string> {
  try {
    const payload: unknown = JSON.parse(await readFile(path.join(solomonHome(), "run", "server", "state.json"), "utf8"));
    if (!payload || typeof payload !== "object" || Array.isArray(payload)) return "";
    const url = (payload as Record<string, unknown>).url;
    return typeof url === "string" ? url.trim().replace(/\/$/, "") : "";
  } catch {
    return "";
  }
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
  headers?: Record<string, string | string[] | undefined>;
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
  write?: (chunk: Uint8Array) => boolean;
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

function readRequestBody(request: UserNameRequest, maxBytes: number): Promise<Uint8Array> {
  return new Promise((resolve, reject) => {
    const chunks: Uint8Array[] = [];
    let size = 0;
    let settled = false;
    const fail = (error: unknown) => {
      if (settled) return;
      settled = true;
      reject(error);
    };
    request.on("data", (chunk) => {
      if (settled) return;
      const bytes = typeof chunk === "string" ? Buffer.from(chunk) : chunk;
      size += bytes.byteLength;
      if (size > maxBytes) {
        fail(new Error("Request body is too large"));
        return;
      }
      chunks.push(bytes);
    });
    request.once("error", () => fail(new Error("Request body read failed")));
    request.once("end", () => {
      if (settled) return;
      settled = true;
      const body = new Uint8Array(size);
      let offset = 0;
      for (const chunk of chunks) {
        body.set(chunk, offset);
        offset += chunk.byteLength;
      }
      resolve(body);
    });
  });
}

function requestHeader(request: UserNameRequest, name: string): string | undefined {
  const value = request.headers?.[name];
  return Array.isArray(value) ? value.join(", ") : value;
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

function attachFastModeEndpoint(server: { middlewares: { use: (route: string, handler: (request: UserNameRequest, response: UserNameResponse, next: () => void) => void) => void } }) {
  server.middlewares.use(fastModeEndpoint, (request, response, next) => {
    if (request.method !== "PUT") {
      next();
      return;
    }
    void readJsonBody(request)
      .then(async (payload) => {
        if (!payload || typeof payload !== "object" || !("fastMode" in payload) || typeof payload.fastMode !== "boolean") {
          respondWithJson(response, 400, { error: "fastMode must be a boolean" });
          return;
        }
        await writeFastMode(solomonHome(), payload.fastMode);
        respondWithJson(response, 200, { fastMode: payload.fastMode });
      })
      .catch(() => respondWithJson(response, 500, { error: "Unable to save fast mode" }));
  });
}

export function projectsPlugin(): Plugin {
  return {
    configurePreviewServer(server) {
      attachProjectActionEndpoint(server);
      attachChatAPIProxy(server);
      attachProjectsEndpoint(server);
      attachHomeDirectoryEntriesEndpoint(server);
      attachHomeGitEndpoints(server);
      attachUserNameEndpoint(server);
      attachReasoningEffortEndpoint(server);
      attachFastModeEndpoint(server);
    },
    configureServer(server) {
      attachProjectActionEndpoint(server);
      attachChatAPIProxy(server);
      attachProjectsEndpoint(server);
      attachHomeDirectoryEntriesEndpoint(server);
      attachHomeGitEndpoints(server);
      attachUserNameEndpoint(server);
      attachReasoningEffortEndpoint(server);
      attachFastModeEndpoint(server);
    },
    name: "solomon-projects",
  };
}
