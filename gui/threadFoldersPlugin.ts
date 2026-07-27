import { readdir, readFile, stat } from "node:fs/promises";
import { homedir } from "node:os";
import path from "node:path";
import type { Plugin } from "vite";

const threadFoldersEndpoint = "/__solomon/thread-folders";

type ThreadFolder = {
  id: string;
  name: string;
  path: string;
  threadCount: number;
};

type ThreadFolderWithActivity = ThreadFolder & { lastActivity: number };

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

  const match = config.match(/^\s*user_name\s*=\s*((?:"(?:[^"\\]|\\.)*")|(?:'(?:[^']|'')*'))\s*(?:#.*)?$/m);
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

async function readThreadFolders(): Promise<ThreadFolder[]> {
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
        (async (): Promise<ThreadFolderWithActivity> => {
          let threadCount = 0;
          let lastActivity = 0;
          const projectDirectory = path.join(home, "projects", projectID);
          try {
            const chatDirectory = path.join(projectDirectory, "chats");
            const files = (await readdir(chatDirectory)).filter((file) => file.endsWith(".json"));
            threadCount = files.length;
            const modifiedTimes = await Promise.all(
              files.map(async (file) => (await stat(path.join(chatDirectory, file))).mtimeMs),
            );
            lastActivity = Math.max(0, ...modifiedTimes);
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
            threadCount,
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

function attachThreadFoldersEndpoint(server: { middlewares: { use: (route: string, handler: (request: { method?: string }, response: { end: (body: string) => void; setHeader: (name: string, value: string) => void; statusCode: number }, next: () => void) => void) => void } }) {
  server.middlewares.use(threadFoldersEndpoint, (request, response, next) => {
    if (request.method !== "GET") {
      next();
      return;
    }
    const home = solomonHome();
    void Promise.all([readThreadFolders(), readUserName(home)])
      .then(([folders, userName]) => {
        response.statusCode = 200;
        response.setHeader("Content-Type", "application/json; charset=utf-8");
        response.end(JSON.stringify({ folders, userName }));
      })
      .catch(() => {
        response.statusCode = 500;
        response.setHeader("Content-Type", "application/json; charset=utf-8");
        response.end(JSON.stringify({ folders: [], userName: "" }));
      });
  });
}

export function threadFoldersPlugin(): Plugin {
  return {
    configurePreviewServer(server) {
      attachThreadFoldersEndpoint(server);
    },
    configureServer(server) {
      attachThreadFoldersEndpoint(server);
    },
    name: "solomon-thread-folders",
  };
}
