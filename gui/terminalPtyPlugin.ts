import { execFileSync } from "node:child_process";
import { chmodSync, existsSync } from "node:fs";
import { createRequire } from "node:module";
import { homedir } from "node:os";
import path from "node:path";
import * as pty from "node-pty";
import { WebSocket, WebSocketServer } from "ws";
import type { HttpServer, Plugin } from "vite";

const terminalEndpoint = "/__solomon/terminal";
const statusPollMs = 400;
const requireFromConfig = createRequire(import.meta.url);
const attachedServers = new WeakSet<HttpServer>();

function shellHasForegroundJob(pid: number) {
  if (process.platform === "win32") return false;
  try {
    const output = execFileSync("ps", ["-o", "pgid=,tpgid=", "-p", String(pid)], {
      encoding: "utf8",
      timeout: 500,
    }).trim();
    const [pgid, tpgid] = output.split(/\s+/).filter(Boolean);
    if (!pgid || !tpgid || tpgid === "0" || tpgid === "-1") return false;
    return pgid !== tpgid;
  } catch {
    return false;
  }
}

function ensureNodePtySpawnHelper() {
  if (process.platform !== "darwin") return;

  try {
    const ptyRoot = path.dirname(requireFromConfig.resolve("node-pty/package.json"));
    for (const architecture of ["darwin-arm64", "darwin-x64"]) {
      const helper = path.join(ptyRoot, "prebuilds", architecture, "spawn-helper");
      if (existsSync(helper)) chmodSync(helper, 0o755);
    }
  } catch {
    // node-pty is unavailable; the connection reports the startup error.
  }
}

function integratedShellPath() {
  if (process.env.SHELL?.trim()) return process.env.SHELL.trim();
  return process.platform === "win32" ? "powershell.exe" : "/bin/zsh";
}

function terminalWorkingDirectory(requestedPath: string | null) {
  const value = requestedPath?.trim() ?? "";
  if (!value) return homedir();
  const candidate = path.isAbsolute(value) ? path.resolve(value) : path.resolve(homedir(), value);
  return existsSync(candidate) ? candidate : homedir();
}

function attachTerminalWebSocket(httpServer: HttpServer | null) {
  if (!httpServer || attachedServers.has(httpServer)) return;
  attachedServers.add(httpServer);
  ensureNodePtySpawnHelper();

  const wss = new WebSocketServer({ noServer: true });
  httpServer.on("upgrade", (request, socket, head) => {
    const pathname = new URL(request.url ?? "", "http://127.0.0.1").pathname;
    if (pathname !== terminalEndpoint) return;

    wss.handleUpgrade(request, socket, head, (ws) => {
      const shellPath = integratedShellPath();
      const shellArgs = process.platform === "win32" ? ["-NoLogo"] : ["-i"];
      const requestURL = new URL(request.url ?? "", "http://127.0.0.1");
      let ptyProcess: pty.IPty;

      try {
        ptyProcess = pty.spawn(shellPath, shellArgs, {
          cols: 80,
          cwd: terminalWorkingDirectory(requestURL.searchParams.get("path")),
          env: { ...process.env, COLORTERM: "truecolor", TERM: "xterm-256color" } as Record<string, string>,
          name: "xterm-256color",
          rows: 24,
        });
      } catch (error) {
        ws.send(`\r\n[terminal panel failed to start${error instanceof Error ? `: ${error.message}` : ""}]\r\n`);
        ws.close();
        return;
      }

      let lastRunning: boolean | null = null;
      let inputStatusTimer: ReturnType<typeof setTimeout> | undefined;
      const publishStatus = (running: boolean) => {
        if (lastRunning === running || ws.readyState !== WebSocket.OPEN) return;
        lastRunning = running;
        ws.send(JSON.stringify({ running, type: "solomon-status" }));
      };
      const statusTimer = setInterval(() => {
        publishStatus(shellHasForegroundJob(ptyProcess.pid));
      }, statusPollMs);

      const dispose = () => {
        clearInterval(statusTimer);
        if (inputStatusTimer !== undefined) clearTimeout(inputStatusTimer);
        try {
          ptyProcess.kill();
        } catch {
          // The child process already exited.
        }
      };

      ptyProcess.onData((data) => {
        if (ws.readyState === WebSocket.OPEN) ws.send(data);
      });
      ptyProcess.onExit(() => {
        publishStatus(false);
        if (ws.readyState === WebSocket.OPEN) ws.close();
      });
      ws.on("message", (data) => {
        const text = Buffer.isBuffer(data) ? data.toString("utf8") : String(data);
        if (text.startsWith("{")) {
          try {
            const message = JSON.parse(text) as { cols?: number; rows?: number; type?: string };
            if (message.type === "resize" && typeof message.cols === "number" && typeof message.rows === "number") {
              ptyProcess.resize(Math.max(2, message.cols), Math.max(1, message.rows));
              return;
            }
          } catch {
            // Treat malformed control data as terminal input.
          }
        }
        ptyProcess.write(text);
        if (inputStatusTimer !== undefined) clearTimeout(inputStatusTimer);
        inputStatusTimer = setTimeout(() => publishStatus(shellHasForegroundJob(ptyProcess.pid)), 50);
      });
      ws.on("close", dispose);
      ws.on("error", dispose);
      publishStatus(false);
    });
  });
}

export function terminalPtyPlugin(): Plugin {
  return {
    configurePreviewServer(server) {
      attachTerminalWebSocket(server.httpServer);
    },
    configureServer(server) {
      attachTerminalWebSocket(server.httpServer);
    },
    name: "solomon-terminal-panel",
  };
}
