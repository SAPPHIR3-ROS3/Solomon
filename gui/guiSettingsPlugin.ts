import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";
import type { Plugin, ViteDevServer, PreviewServer } from "vite";

const repositoryRoot = fileURLToPath(new URL("..", import.meta.url));
const helper = fileURLToPath(new URL("./desktop/gui_settings.go", import.meta.url));

export type GUISettings = {
  chat: { autoCloseToolCalls?: boolean; startToolCallsCollapsed?: boolean };
  models: { hiddenModels: Record<string, string[]> };
  docs: Record<string, never>;
};

export function runGUISettings(request: object, home?: string): Promise<GUISettings> {
  return new Promise((resolve, reject) => {
    const child = spawn("go", ["run", helper], {
      cwd: repositoryRoot,
      env: { ...process.env, ...(home ? { SOLOMON_HOME: home } : {}) },
      stdio: ["pipe", "pipe", "pipe"],
    });
    let stdout = "";
    let stderr = "";
    child.stdout.on("data", chunk => { stdout += chunk; });
    child.stderr.on("data", chunk => { stderr += chunk; });
    child.once("error", reject);
    child.once("close", code => {
      if (code !== 0) { reject(new Error(stderr.trim() || "Unable to access GUI settings")); return; }
      try { resolve(JSON.parse(stdout) as GUISettings); } catch (error) { reject(error); }
    });
    child.stdin.end(JSON.stringify(request));
  });
}

function attach(server: ViteDevServer | PreviewServer) {
  server.middlewares.use("/__solomon/gui-settings", (request, response) => {
    response.setHeader("Content-Type", "application/json");
    response.setHeader("Cache-Control", "no-store");
    const respond = (status: number, body: object) => { response.statusCode = status; response.end(JSON.stringify(body)); };
    if (request.method === "GET") {
      void runGUISettings({ action: "read" }).then(settings => respond(200, settings)).catch(error => respond(500, { error: error.message }));
    } else if (request.method === "PATCH") {
      let body = "";
      request.on("data", chunk => { body += String(chunk); });
      request.on("end", () => {
        let payload: { chat?: object };
        try {
          if (body.length > 4096) throw new Error("Request body is too large");
          payload = JSON.parse(body);
          if (!payload?.chat || typeof payload.chat !== "object" || Array.isArray(payload.chat)) throw new Error("Chat preferences are required");
          const entries = Object.entries(payload.chat);
          if (!entries.length || entries.some(([key, value]) => !["autoCloseToolCalls", "startToolCallsCollapsed"].includes(key) || typeof value !== "boolean")) throw new Error("Invalid chat preference");
        } catch (error) { respond(400, { error: error instanceof Error ? error.message : "Invalid JSON" }); return; }
        void runGUISettings({ action: "patch", chat: payload.chat }).then(settings => respond(200, settings)).catch(error => respond(500, { error: error.message }));
      });
    } else { respond(405, { error: "Method not allowed" }); }
  });
}

export function guiSettingsPlugin(): Plugin {
  return { name: "solomon-gui-settings", configureServer: attach, configurePreviewServer: attach };
}
