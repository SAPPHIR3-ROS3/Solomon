import { chmodSync, existsSync } from "node:fs";
import { createRequire } from "node:module";
import { homedir } from "node:os";
import path from "node:path";
import * as pty from "node-pty";
import { WebSocket, WebSocketServer } from "ws";
import type { HttpServer, Plugin } from "vite";

const terminalEndpoint = "/__solomon/terminal";
const requireFromConfig = createRequire(import.meta.url);
const attachedServers = new WeakSet<HttpServer>();

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
      let ptyProcess: pty.IPty;

      try {
        ptyProcess = pty.spawn(shellPath, shellArgs, {
          cols: 80,
          cwd: homedir(),
          env: { ...process.env, COLORTERM: "truecolor", TERM: "xterm-256color" } as Record<string, string>,
          name: "xterm-256color",
          rows: 24,
        });
      } catch (error) {
        ws.send(`\r\n[terminal panel failed to start${error instanceof Error ? `: ${error.message}` : ""}]\r\n`);
        ws.close();
        return;
      }

      const dispose = () => {
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
      });
      ws.on("close", dispose);
      ws.on("error", dispose);
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
