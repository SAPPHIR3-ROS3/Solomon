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

declare global {
  interface Window {
    runtime?: WailsEnvironment;
    wails?: unknown;
  }
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

function isWailsDesktop(): boolean {
  return typeof window.runtime?.Environment === "function" && window.wails !== undefined;
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
