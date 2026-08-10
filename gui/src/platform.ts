import { Environment } from "../wailsjs/runtime/runtime";

export type ClientSurface = "web" | "desktop";
export type ClientOS = "macos" | "windows" | "linux" | "unknown";

export interface ClientInfo {
  surface: ClientSurface;
  os: ClientOS;
}

type WailsEnvironment = {
  Environment?: () => Promise<unknown>;
};

type WailsServerBridge = {
  URL: () => Promise<string>;
};

type WailsDesktopBridge = {
  ModelCatalog: () => Promise<unknown>;
  ConnectProvider?: (request: { APIKey: string; BaseURL: string; Kind: number; Name: string }) => Promise<unknown>;
  SetModelEnabled?: (provider: string, model: string, enabled: boolean) => Promise<unknown>;
  ProjectSidebarData: () => Promise<unknown>;
  ProjectDirectoryEntries?: (projectID: string, directoryPath: string) => Promise<unknown>;
  ProjectAtMentionSuggestions?: (projectID: string, query: string) => Promise<unknown>;
  AtMentionSuggestions?: (entries: Array<{ isDirectory: boolean; path: string }>, query: string) => Promise<unknown>;
  ProjectResearch?: (projectID: string) => Promise<unknown>;
  ProjectResearchReport?: (projectID: string, researchID: string) => Promise<string>;
  ProjectBranches?: (projectID: string) => Promise<unknown>;
  ProjectGitHistory?: (projectID: string) => Promise<unknown>;
  ProjectWorktrees?: (projectID: string) => Promise<unknown>;
  CheckoutProjectBranch?: (projectID: string, branch: string) => Promise<unknown>;
  ProjectRemovalInfo?: (projectID: string) => Promise<unknown>;
  RemoveProjectFromDisk?: (projectID: string) => Promise<void>;
  RemoveProjectFromSidebar?: (projectID: string) => Promise<void>;
  SaveCurrentModel: (provider: string, model: string) => Promise<unknown>;
  SaveReasoningEffort?: (effort: string) => Promise<string>;
  SaveUserName: (userName: string) => Promise<string>;
  CustomizationRules?: () => Promise<unknown>;
  ReorderCustomizationRules?: (ruleIDs: number[]) => Promise<unknown>;
  UpdateCustomizationRule?: (ruleID: number, text: string) => Promise<unknown>;
  DeleteCustomizationRule?: (ruleID: number) => Promise<unknown>;
  CustomizationSkills?: () => Promise<unknown>;
  CustomizationMcps?: () => Promise<unknown>;
  CustomizationSubagents?: () => Promise<unknown>;
  CustomizationPromptTemplates?: () => Promise<unknown>;
  CustomizationPromptTemplate?: (id: string) => Promise<unknown>;
  UpdateCustomizationPromptTemplate?: (id: string, content: string) => Promise<unknown>;
  ResetCustomizationPromptTemplate?: (id: string) => Promise<unknown>;
  UpdateCustomizationSubagent?: (id: string, detail: string, scores: Array<{ id: string; value: number }>) => Promise<unknown>;
  DeleteCustomizationSubagent?: (id: string) => Promise<unknown>;
  RolesTable?: () => Promise<unknown>;
  SaveRolesTable?: (characteristics: string[]) => Promise<unknown>;
};

declare global {
  interface Window {
    runtime?: WailsEnvironment;
    wails?: unknown;
    go?: {
      main?: {
        DesktopBridge?: WailsDesktopBridge;
        ServerBridge?: WailsServerBridge;
      };
    };
  }
}

export async function desktopBridge(): Promise<WailsDesktopBridge | undefined> {
  // Wails injects its bindings immediately after the document starts loading.
  // Do not use runtime.Environment as the guard here: on a cold WebView load it
  // can arrive after React has already requested sidebar data.
  if (window.location.hostname !== "wails.localhost" && !window.go?.main) return undefined;
  for (let attempt = 0; attempt < 20; attempt += 1) {
    const bridge = window.go?.main?.DesktopBridge;
    if (bridge) return bridge;
    await new Promise((resolve) => window.setTimeout(resolve, 25));
  }
  return undefined;
}

export function initialClient(): ClientInfo {
  return {
    surface: isWailsDesktop() ? "desktop" : "web",
    os: osFromPlatform(navigator.userAgent),
  };
}

export async function detectClient(): Promise<ClientInfo> {
  const client = initialClient();
  if (client.surface !== "desktop") {
    return client;
  }

  try {
    const environment = await Environment();
    return { ...client, os: osFromPlatform(environment.platform) };
  } catch {
    return client;
  }
}

export async function serverEndpoint(path: string): Promise<string> {
  if (!isWailsDesktop()) return path;

  const serverURL = await getDesktopServerURL();
  if (serverURL) return `${serverURL.replace(/\/$/, "")}${path}`;

  if (window.location.hostname !== "wails.localhost") return path;
  const protocol = window.location.protocol === "https:" ? "https:" : "http:";
  const port = window.location.port ? `:${window.location.port}` : "";
  return `${protocol}//127.0.0.1${port}${path}`;
}

function getDesktopServerURL(): Promise<string> {
  return new Promise((resolve) => {
    let attempt = 0;
    const resolveURL = () => {
      const bridge = window.go?.main?.ServerBridge;
      if (!bridge && attempt++ < 10) {
        window.setTimeout(resolveURL, 25);
        return;
      }
      if (!bridge) {
        resolve("");
        return;
      }
      void bridge.URL().then((url) => resolve(url.trim())).catch(() => resolve(""));
    };
    resolveURL();
  });
}

function isWailsDesktop(): boolean {
  return typeof window.runtime?.Environment === "function";
}

function osFromPlatform(platform: string): ClientOS {
  const normalized = platform.toLowerCase();
  if (normalized.includes("darwin") || normalized.includes("mac")) {
    return "macos";
  }
  if (normalized.includes("win")) {
    return "windows";
  }
  if (normalized.includes("linux")) {
    return "linux";
  }
  return "unknown";
}
