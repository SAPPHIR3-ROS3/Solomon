import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { promises as fs, chmodSync, existsSync } from "node:fs";
import { homedir } from "node:os";
import { execFile } from "node:child_process";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import path from "node:path";
import type { IncomingMessage, ServerResponse } from "node:http";
import type { Server as HttpServer } from "node:http";
import { parse } from "smol-toml";
import { WebSocketServer } from "ws";
import * as pty from "node-pty";

const userNameEndpoint = "/__solomon/user-name";
const modelsEndpoint = "/__solomon/models";
const branchesEndpoint = "/__solomon/branches";
const workspaceFilesEndpoint = "/__solomon/workspace-files";
const workspaceFileEndpoint = "/__solomon/workspace-file";
const workspaceSearchEndpoint = "/__solomon/workspace-search";
const gitHistoryEndpoint = "/__solomon/git-history";
const terminalEndpoint = "/__solomon/terminal";
const requireFromConfig = createRequire(import.meta.url);

function ensureNodePtySpawnHelper() {
  if (process.platform !== "darwin") return;
  try {
    const ptyRoot = path.dirname(requireFromConfig.resolve("node-pty/package.json"));
    for (const arch of ["darwin-arm64", "darwin-x64"]) {
      const helper = path.join(ptyRoot, "prebuilds", arch, "spawn-helper");
      if (existsSync(helper)) chmodSync(helper, 0o755);
    }
  } catch {
    /* node-pty unavailable */
  }
}

ensureNodePtySpawnHelper();
const temporarilySkippedProviders = new Set(["Claude Sub"]);
const skippedWorkspaceDirectories = new Set([".git", "node_modules", "dist"]);
const repositoryRoot = fileURLToPath(new URL("../", import.meta.url));
let modelCatalogCache: { createdAt: number; json: string } | undefined;
let modelCatalogInFlight: Promise<string> | undefined;

function configPath() {
  const solomonHome = process.env.SOLOMON_HOME?.trim() || path.join(homedir(), ".solomon");
  return path.join(solomonHome, "config.toml");
}

function topLevelUserName(source: string) {
  const firstTable = source.search(/^\s*\[/m);
  const root = firstTable === -1 ? source : source.slice(0, firstTable);
  return root.match(/^\s*user_name\s*=\s*(["'])(.*?)\1\s*(?:#.*)?$/m)?.[2] ?? "";
}

function setTopLevelUserName(source: string, userName: string) {
  const firstTable = source.search(/^\s*\[/m);
  const boundary = firstTable === -1 ? source.length : firstTable;
  const root = source.slice(0, boundary);
  const rest = source.slice(boundary);
  const declaration = `user_name = ${JSON.stringify(userName)}`;
  const pattern = /^\s*user_name\s*=.*$/m;
  if (pattern.test(root)) return root.replace(pattern, declaration) + rest;
  return `${declaration}\n${source.startsWith("\n") ? "" : "\n"}${source}`;
}

async function readBody(request: IncomingMessage) {
  const chunks: Buffer[] = [];
  for await (const chunk of request) chunks.push(Buffer.from(chunk));
  return Buffer.concat(chunks).toString("utf8");
}

async function handleUserName(request: IncomingMessage, response: ServerResponse) {
  response.setHeader("Content-Type", "application/json; charset=utf-8");
  try {
    const target = configPath();
    const source = await fs.readFile(target, "utf8");
    if (request.method === "GET") {
      response.end(JSON.stringify({ user_name: topLevelUserName(source) }));
      return;
    }
    if (request.method === "PUT") {
      const body = JSON.parse(await readBody(request)) as { user_name?: unknown };
      if (typeof body.user_name !== "string") {
        response.statusCode = 400;
        response.end(JSON.stringify({ error: "user_name must be a string" }));
        return;
      }
      const userName = body.user_name.trim();
      const temporary = `${target}.ui-prototype.tmp`;
      await fs.writeFile(temporary, setTopLevelUserName(source, userName), { mode: 0o600 });
      await fs.rename(temporary, target);
      response.end(JSON.stringify({ user_name: userName }));
      return;
    }
    response.statusCode = 405;
    response.end(JSON.stringify({ error: "method not allowed" }));
  } catch (error) {
    response.statusCode = 500;
    response.end(JSON.stringify({ error: error instanceof Error ? error.message : String(error) }));
  }
}

function recordValue(value: unknown): Record<string, unknown> {
  return value && typeof value === "object" && !Array.isArray(value) ? value as Record<string, unknown> : {};
}

function modelCatalog(source: string) {
  const document = recordValue(parse(source));
  const currentConfig = recordValue(document.current);
  const current = {
    provider: typeof currentConfig.provider === "string" ? currentConfig.provider.trim() : "",
    model: typeof currentConfig.model === "string" ? currentConfig.model.trim() : "",
  };
  const recentModels = recordValue(document.recent_models);
  const providers = Object.entries(recentModels).flatMap(([provider, value]) => {
    if (temporarilySkippedProviders.has(provider)) return [];
    if (!Array.isArray(value)) return [];
    const models = Array.from(new Set(value.filter((model): model is string => typeof model === "string").map((model) => model.trim()).filter(Boolean)));
    return models.length ? [{ provider, models, complete: false }] : [];
  });
  const recent = providers.flatMap((group) => group.models.map((model) => ({ provider: group.provider, model })));
  let currentProvider = providers.find((entry) => entry.provider === current.provider);
  if (!currentProvider && current.provider && current.model) {
    currentProvider = { provider: current.provider, models: [current.model], complete: false };
    providers.unshift(currentProvider);
  } else if (currentProvider && current.model && !currentProvider.models.includes(current.model)) {
    currentProvider.models.unshift(current.model);
  }
  if (currentProvider) {
    const index = providers.indexOf(currentProvider);
    if (index > 0) providers.unshift(...providers.splice(index, 1));
  }
  return { current, recent, providers };
}

function fullModelCatalogJSON() {
  if (modelCatalogCache && Date.now() - modelCatalogCache.createdAt < 60_000) return Promise.resolve(modelCatalogCache.json);
  if (modelCatalogInFlight) return modelCatalogInFlight;
  modelCatalogInFlight = new Promise<string>((resolve, reject) => {
    execFile(
      "go",
      ["run", path.join(repositoryRoot, "scripts", "ui_model_catalog.go")],
      { cwd: repositoryRoot, env: process.env, timeout: 70_000, maxBuffer: 2 * 1024 * 1024 },
      (error, stdout) => {
        if (error) {
          reject(error);
          return;
        }
        try {
          JSON.parse(stdout);
          resolve(stdout.trim());
        } catch (parseError) {
          reject(parseError);
        }
      },
    );
  }).then((json) => {
    modelCatalogCache = { createdAt: Date.now(), json };
    return json;
  }).finally(() => {
    modelCatalogInFlight = undefined;
  });
  return modelCatalogInFlight;
}

async function handleModels(request: IncomingMessage, response: ServerResponse) {
  response.setHeader("Content-Type", "application/json; charset=utf-8");
  if (request.method !== "GET") {
    response.statusCode = 405;
    response.end(JSON.stringify({ error: "method not allowed" }));
    return;
  }
  try {
    response.end(await fullModelCatalogJSON());
  } catch (error) {
    try {
      const source = await fs.readFile(configPath(), "utf8");
      response.setHeader("X-Solomon-Models-Source", "recent-fallback");
      response.end(JSON.stringify(modelCatalog(source)));
    } catch {
      response.statusCode = 500;
      response.end(JSON.stringify({ error: error instanceof Error ? error.message : String(error) }));
    }
  }
}

function runGit(args: string[], trim = true) {
  return new Promise<string>((resolve, reject) => {
    execFile("git", args, { cwd: repositoryRoot, timeout: 10_000, maxBuffer: 1024 * 1024 }, (error, stdout) => {
      if (error) reject(error);
      else resolve(trim ? stdout.trim() : stdout);
    });
  });
}

async function handleBranches(request: IncomingMessage, response: ServerResponse) {
  response.setHeader("Content-Type", "application/json; charset=utf-8");
  if (request.method !== "GET") {
    response.statusCode = 405;
    response.end(JSON.stringify({ error: "method not allowed" }));
    return;
  }
  try {
    const [current, branchOutput] = await Promise.all([
      runGit(["symbolic-ref", "--quiet", "--short", "HEAD"]).catch(() => ""),
      runGit(["for-each-ref", "--format=%(refname:short)", "refs/heads"]),
    ]);
    const branches = Array.from(new Set(branchOutput.split(/\r?\n/).map((branch) => branch.trim()).filter(Boolean)))
      .sort((left, right) => left === "main" ? -1 : right === "main" ? 1 : left.localeCompare(right));
    response.end(JSON.stringify({ current, branches }));
  } catch (error) {
    response.statusCode = 500;
    response.end(JSON.stringify({ error: error instanceof Error ? error.message : String(error) }));
  }
}

async function workspaceFiles(directory = repositoryRoot, relative = ""): Promise<string[]> {
  const entries = await fs.readdir(directory, { withFileTypes: true });
  const files: string[] = [];
  for (const entry of entries.sort((left, right) => left.name.localeCompare(right.name))) {
    const nextRelative = relative ? `${relative}/${entry.name}` : entry.name;
    if (entry.isDirectory()) {
      if (!skippedWorkspaceDirectories.has(entry.name)) files.push(...await workspaceFiles(path.join(directory, entry.name), nextRelative));
    } else if (entry.isFile()) {
      files.push(nextRelative);
    }
  }
  return files;
}

function gitFileState(code: string) {
  if (code === "?" || code === "A" || code === "C") return "A";
  if (code === "D") return "D";
  if (code === "R") return "R";
  if (code === "M") return "M";
  return "";
}

function gitNameStatus(output: string) {
  const changes: Record<string, string> = {};
  const entries = output.split("\0");
  for (let index = 0; index < entries.length - 1;) {
    const nameStatus = entries[index++];
    if (!nameStatus) continue;
    const state = gitFileState(nameStatus[0]);
    const firstPath = entries[index++];
    if (!state || !firstPath) continue;
    // For renames and copies, --name-status -z supplies the old path first.
    const file = (state === "R" || nameStatus[0] === "C") ? entries[index++] : firstPath;
    if (file) changes[file] = state;
  }
  return changes;
}

function workspaceGitStatus(stagedOutput: string, changesOutput: string, untrackedOutput: string) {
  const status: Record<string, string> = {};
  const staged = gitNameStatus(stagedOutput);
  const changes = gitNameStatus(changesOutput);
  for (const file of untrackedOutput.split("\0")) if (file) changes[file] = "U";
  Object.assign(status, staged, changes);
  return { status, staged, changes };
}

async function handleWorkspaceFiles(request: IncomingMessage, response: ServerResponse) {
  response.setHeader("Content-Type", "application/json; charset=utf-8");
  if (request.method !== "GET") {
    response.statusCode = 405;
    response.end(JSON.stringify({ error: "method not allowed" }));
    return;
  }
  try {
    const [files, stagedOutput, changesOutput, untrackedOutput] = await Promise.all([
      workspaceFiles(),
      runGit(["diff", "--cached", "--name-status", "-z"], false).catch(() => ""),
      runGit(["diff", "--name-status", "-z"], false).catch(() => ""),
      runGit(["ls-files", "--others", "--exclude-standard", "-z"], false).catch(() => ""),
    ]);
    response.end(JSON.stringify({ files, ...workspaceGitStatus(stagedOutput, changesOutput, untrackedOutput) }));
  } catch (error) {
    response.statusCode = 500;
    response.end(JSON.stringify({ error: error instanceof Error ? error.message : String(error) }));
  }
}

function workspaceFileKind(filePath: string) {
  const ext = path.extname(filePath).slice(1).toLowerCase();
  if (ext === "go") return "go";
  return ext || "text";
}

async function handleWorkspaceFile(request: IncomingMessage, response: ServerResponse) {
  response.setHeader("Content-Type", "application/json; charset=utf-8");
  if (request.method !== "GET") {
    response.statusCode = 405;
    response.end(JSON.stringify({ error: "method not allowed" }));
    return;
  }
  const relative = new URL(request.url ?? "", "http://localhost").searchParams.get("path")?.trim();
  if (!relative) {
    response.statusCode = 400;
    response.end(JSON.stringify({ error: "path is required" }));
    return;
  }
  const normalized = path.normalize(relative).replace(/^(\.\.(\/|\\|$))+/, "");
  const workspaceRoot = path.resolve(repositoryRoot);
  const absolute = path.resolve(workspaceRoot, normalized);
  const relativeToRoot = path.relative(workspaceRoot, absolute);
  if (relativeToRoot.startsWith("..") || path.isAbsolute(relativeToRoot)) {
    response.statusCode = 403;
    response.end(JSON.stringify({ error: "path outside workspace" }));
    return;
  }
  try {
    const content = await fs.readFile(absolute, "utf8");
    response.end(JSON.stringify({
      path: normalized.replace(/\\/g, "/"),
      kind: workspaceFileKind(normalized),
      status: "clean",
      additions: 0,
      deletions: 0,
      content,
    }));
  } catch (error) {
    response.statusCode = 404;
    response.end(JSON.stringify({ error: error instanceof Error ? error.message : String(error) }));
  }
}

async function handleWorkspaceSearch(request: IncomingMessage, response: ServerResponse) {
  response.setHeader("Content-Type", "application/json; charset=utf-8");
  if (request.method !== "GET") {
    response.statusCode = 405;
    response.end(JSON.stringify({ error: "method not allowed" }));
    return;
  }
  const query = new URL(request.url ?? "", "http://localhost").searchParams.get("q")?.trim().toLocaleLowerCase() ?? "";
  if (!query) {
    response.end(JSON.stringify({ files: await workspaceFiles() }));
    return;
  }
  try {
    const files = await workspaceFiles();
    response.end(JSON.stringify({ files: files.filter((relative) => relative.toLocaleLowerCase().includes(query)).slice(0, 300) }));
  } catch (error) {
    response.statusCode = 500;
    response.end(JSON.stringify({ error: error instanceof Error ? error.message : String(error) }));
  }
}

async function handleGitHistory(request: IncomingMessage, response: ServerResponse) {
  response.setHeader("Content-Type", "application/json; charset=utf-8");
  if (request.method !== "GET") {
    response.statusCode = 405;
    response.end(JSON.stringify({ error: "method not allowed" }));
    return;
  }
  try {
    const output = await runGit(["log", "--branches", "--remotes", "--topo-order", "--max-count=100", "--pretty=format:%H%x1f%P%x1f%s%x1f%D%x1e"], false);
    const commits = output.split("\x1e").flatMap((entry) => {
      const [id, parents = "", subject, decorations = ""] = entry.trim().split("\x1f");
      if (!id || !subject) return [];
      const references = decorations.split(",").map((value) => value.trim()).filter(Boolean);
      return [{ id, parents: parents.split(" ").filter(Boolean), subject, references }];
    });
    response.end(JSON.stringify({ commits }));
  } catch (error) {
    response.statusCode = 500;
    response.end(JSON.stringify({ error: error instanceof Error ? error.message : String(error) }));
  }
}

function integratedShellPath() {
  if (process.env.SHELL?.trim()) return process.env.SHELL.trim();
  return process.platform === "win32" ? "powershell.exe" : "/bin/zsh";
}

function resolveTerminalCwd(request: IncomingMessage) {
  const cwdParam = new URL(request.url ?? "", "http://127.0.0.1").searchParams.get("cwd")?.trim();
  if (cwdParam && path.isAbsolute(cwdParam)) return cwdParam;
  return repositoryRoot;
}

function attachTerminalWebSocket(httpServer: HttpServer | null | undefined) {
  if (!httpServer) return;
  const wss = new WebSocketServer({ noServer: true });
  httpServer.on("upgrade", (request, socket, head) => {
    const pathname = new URL(request.url ?? "", "http://127.0.0.1").pathname;
    if (pathname !== terminalEndpoint) return;
    wss.handleUpgrade(request, socket, head, (ws) => {
      const cwd = resolveTerminalCwd(request);
      const shellPath = integratedShellPath();
      const shellArgs = process.platform === "win32" ? ["-NoLogo"] : ["-i"];
      let ptyProcess: pty.IPty;
      try {
        ptyProcess = pty.spawn(shellPath, shellArgs, {
          name: "xterm-256color",
          cwd,
          cols: 80,
          rows: 24,
          env: { ...process.env, TERM: "xterm-256color", COLORTERM: "truecolor" } as Record<string, string>,
        });
      } catch (error) {
        ws.send(`\r\n[terminal failed to start${error instanceof Error ? `: ${error.message}` : ""}]\r\n`);
        ws.close();
        return;
      }
      const dispose = () => {
        try {
          ptyProcess.kill();
        } catch {
          /* already dead */
        }
      };
      ptyProcess.onData((data) => {
        if (ws.readyState === ws.OPEN) ws.send(data);
      });
      ptyProcess.onExit(() => {
        if (ws.readyState === ws.OPEN) ws.close();
      });
      ws.on("message", (data) => {
        const text = Buffer.isBuffer(data) ? data.toString("utf8") : String(data);
        if (text.startsWith("{")) {
          try {
            const message = JSON.parse(text) as { type?: string; cols?: number; rows?: number };
            if (message.type === "resize" && typeof message.cols === "number" && typeof message.rows === "number") {
              ptyProcess.resize(Math.max(2, message.cols), Math.max(1, message.rows));
              return;
            }
          } catch {
            /* fall through as terminal input */
          }
        }
        ptyProcess.write(text);
      });
      ws.on("close", dispose);
      ws.on("error", dispose);
    });
  });
}

function terminalPlugin() {
  return {
    name: "solomon-terminal",
    configureServer(server: { httpServer: HttpServer | null }) {
      attachTerminalWebSocket(server.httpServer);
    },
    configurePreviewServer(server: { httpServer: HttpServer | null }) {
      attachTerminalWebSocket(server.httpServer);
    },
  };
}

function originalConfigPlugin() {
  const install = (middlewares: { use: (path: string, handler: typeof handleUserName) => void }) => {
    middlewares.use(userNameEndpoint, handleUserName);
    middlewares.use(modelsEndpoint, handleModels);
    middlewares.use(branchesEndpoint, handleBranches);
    middlewares.use(workspaceFilesEndpoint, handleWorkspaceFiles);
    middlewares.use(workspaceFileEndpoint, handleWorkspaceFile);
    middlewares.use(workspaceSearchEndpoint, handleWorkspaceSearch);
    middlewares.use(gitHistoryEndpoint, handleGitHistory);
  };
  return {
    name: "solomon-original-config",
    configureServer(server: { middlewares: Parameters<typeof install>[0] }) { install(server.middlewares); },
    configurePreviewServer(server: { middlewares: Parameters<typeof install>[0] }) { install(server.middlewares); },
  };
}

export default defineConfig({
  plugins: [react(), tailwindcss(), originalConfigPlugin(), terminalPlugin()],
  server: {
    host: "127.0.0.1",
    port: 4173,
    strictPort: true,
  },
  preview: {
    host: "127.0.0.1",
    port: 4173,
    strictPort: true,
  },
});
