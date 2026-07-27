import { readdir, readFile, rename, stat, writeFile } from "node:fs/promises";
import { homedir } from "node:os";
import path from "node:path";
import type { Plugin } from "vite";

const projectsEndpoint = "/__solomon/projects";
const userNameEndpoint = "/__solomon/user-name";

type Project = {
  chats: SidebarChat[];
  id: string;
  name: string;
  path: string;
  chatCount: number;
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
            name: path.basename(projectPath) || projectPath,
            path: projectPath,
            chatCount: chats.length,
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

function attachProjectsEndpoint(server: { middlewares: { use: (route: string, handler: (request: { method?: string }, response: { end: (body: string) => void; setHeader: (name: string, value: string) => void; statusCode: number }, next: () => void) => void) => void } }) {
  server.middlewares.use(projectsEndpoint, (request, response, next) => {
    if (request.method !== "GET") {
      next();
      return;
    }
    const home = solomonHome();
    void Promise.all([readProjects(), readUserName(home)])
      .then(([projects, userName]) => {
        response.statusCode = 200;
        response.setHeader("Content-Type", "application/json; charset=utf-8");
        response.end(JSON.stringify({ projects, userName }));
      })
      .catch(() => {
        response.statusCode = 500;
        response.setHeader("Content-Type", "application/json; charset=utf-8");
        response.end(JSON.stringify({ projects: [], userName: "" }));
      });
  });
}

type UserNameRequest = {
  method?: string;
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

export function projectsPlugin(): Plugin {
  return {
    configurePreviewServer(server) {
      attachProjectsEndpoint(server);
      attachUserNameEndpoint(server);
    },
    configureServer(server) {
      attachProjectsEndpoint(server);
      attachUserNameEndpoint(server);
    },
    name: "solomon-projects",
  };
}
