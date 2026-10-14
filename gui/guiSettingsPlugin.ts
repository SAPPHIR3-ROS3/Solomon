import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";
import type { Plugin, ViteDevServer, PreviewServer } from "vite";

const repositoryRoot = fileURLToPath(new URL("..", import.meta.url));
const helper = fileURLToPath(new URL("./desktop/gui_settings.go", import.meta.url));

export type GUISettings = {
  general: { userName: string; responseLanguage: string };
  advanced: Record<string, string | number | boolean>;
  chat: { autoCloseToolCalls?: boolean; startToolCallsCollapsed?: boolean; startReasoningCollapsed?: boolean };
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
        let payload: { chat?: object; general?: { userName?: string; responseLanguage?: string }; advanced?: Record<string, unknown> };
        try {
          if (body.length > 4096) throw new Error("Request body is too large");
          payload = JSON.parse(body);
          if (!payload || typeof payload !== "object" || Array.isArray(payload) || (!payload.chat && !payload.general && !payload.advanced)) throw new Error("Preferences are required");
          if (payload.chat !== undefined) {
            if (!payload.chat || typeof payload.chat !== "object" || Array.isArray(payload.chat)) throw new Error("Invalid chat preferences");
            const entries = Object.entries(payload.chat);
            if (!entries.length || entries.some(([key, value]) => !["autoCloseToolCalls", "startToolCallsCollapsed", "startReasoningCollapsed"].includes(key) || typeof value !== "boolean")) throw new Error("Invalid chat preference");
          }
          if (payload.general !== undefined) {
            if (!payload.general || typeof payload.general !== "object" || Array.isArray(payload.general)) throw new Error("Invalid general preferences");
            const entries = Object.entries(payload.general);
            if (!entries.length || entries.some(([key, value]) => !["userName", "responseLanguage"].includes(key) || typeof value !== "string" || !value.trim() || Buffer.byteLength(value.trim()) > 256 || /[\r\n\0]/.test(value.trim()))) throw new Error("Invalid general preference");
          }
          if (payload.advanced !== undefined && (!payload.advanced || typeof payload.advanced !== "object" || Array.isArray(payload.advanced) || !Object.keys(payload.advanced).length)) throw new Error("Invalid advanced preferences");
        } catch (error) { respond(400, { error: error instanceof Error ? error.message : "Invalid JSON" }); return; }
        void runGUISettings({ action: "patch", ...payload }).then(settings => respond(200, settings)).catch(error => respond(500, { error: error.message }));
      });
    } else { respond(405, { error: "Method not allowed" }); }
  });
}

export function guiSettingsPlugin(): Plugin {
  return { name: "solomon-gui-settings", configureServer: attach, configurePreviewServer: attach };
}
