import { copyFile, lstat, mkdir, readdir, readFile, realpath, rename, rm, stat, writeFile } from "node:fs/promises";
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
} from "../projectBranches.ts";

import { reasoningEfforts, solomonHome } from "./shared.ts";

export type Project = {
  chats: SidebarChat[];
  id: string;
  name: string;
  path: string;
  chatCount: number;
  tokenStats: ProjectTokenStats;
};

export type ProjectTokenStats = {
  user: number;
  reasoning: number;
  response: number;
  total: number;
};

export type SidebarChat = Omit<Chat, "lastActivity">;

export type Chat = {
  id: string;
  lastMessageAt: string;
  lastActivity: number;
  title: string;
};

export type ProjectWithActivity = Project & { lastActivity: number };

export function projectDisplayName(projectPath: string): string {
  const resolvedHome = path.resolve(homedir());
  const resolvedProject = path.resolve(projectPath);
  if (resolvedProject === resolvedHome) return "Home";
  return path.basename(projectPath) || projectPath;
}

export function normalizeReasoningEffort(value: string): string {
  const normalized = value.trim().toLowerCase().replaceAll("_", "-").replace(/\s+/g, "-");
  if (normalized === "med") return "medium";
  if (normalized === "x-high" || normalized === "extra-high") return "xhigh";
  return reasoningEfforts.has(normalized) ? normalized : "";
}

export function rootConfigParts(source: string) {
  const firstTable = source.search(/^\s*\[/m);
  const boundary = firstTable === -1 ? source.length : firstTable;
  return { root: source.slice(0, boundary), tables: source.slice(boundary) };
}

export function rootValuePattern(key: string): RegExp {
  const escapedKey = key.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  return new RegExp(`^(\\s*${escapedKey}\\s*=\\s*)((?:"(?:[^"\\\\]|\\\\.)*")|(?:'(?:[^']|'')*')|[^\\s#]+)(\\s*(?:#.*)?)$`, "m");
}

export function parseQuotedString(value: string): string {
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

export function parseRootValue(raw: string): string {
  return parseQuotedString(raw).trim();
}

export async function readRootValue(home: string, key: string): Promise<string | undefined> {
  try {
    const source = await readFile(path.join(home, "config.toml"), "utf8");
    const match = rootConfigParts(source).root.match(rootValuePattern(key));
    return match ? parseRootValue(match[1]) : undefined;
  } catch {
    return undefined;
  }
}

export async function writeRootValue(home: string, key: string, value: string | boolean): Promise<void> {
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

export async function readUserName(home: string): Promise<string> {
  return (await readRootValue(home, "user_name")) ?? "";
}

export async function writeUserName(home: string, userName: string): Promise<void> {
  await writeRootValue(home, "user_name", userName);
}

export async function readReasoningEffort(home: string): Promise<string> {
  return normalizeReasoningEffort((await readRootValue(home, "reasoning_effort")) ?? "") || "none";
}

export async function writeReasoningEffort(home: string, effort: string): Promise<void> {
  await writeRootValue(home, "reasoning_effort", effort);
}

export async function readFastMode(home: string): Promise<boolean> {
  return (await readRootValue(home, "fast_mode")) !== "false";
}

export async function writeFastMode(home: string, enabled: boolean): Promise<void> {
  await writeRootValue(home, "fast_mode", enabled);
}

export async function readProjects(): Promise<Project[]> {
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

export async function registerProject(rawPath: string): Promise<Project> {
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

export async function readChat(chatDirectory: string, fileName: string): Promise<Chat | null> {
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

export async function readProjectTokenStats(projectDirectory: string): Promise<ProjectTokenStats> {
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

export function approximateSessionTokenStats(payload: unknown): ProjectTokenStats {
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

export function emptyProjectTokenStats(): ProjectTokenStats {
  return { user: 0, reasoning: 0, response: 0, total: 0 };
}

export function withTokenTotal(stats: ProjectTokenStats): ProjectTokenStats {
  return { ...stats, total: stats.user + stats.reasoning + stats.response };
}

export function approximateTextTokens(value: string): number {
  return Math.ceil(value.length / 4);
}

export function nonNegativeNumber(value: unknown): number {
  return typeof value === "number" && Number.isFinite(value) ? Math.max(0, Math.round(value)) : 0;
}

