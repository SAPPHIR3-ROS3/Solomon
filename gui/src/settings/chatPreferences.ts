import { useSyncExternalStore } from "react";
import { serverEndpoint } from "../platform";

type ChatPreferences = { autoCloseToolCalls: boolean; startToolCallsCollapsed: boolean };
type PreferenceState = { chat: ChatPreferences; ready: boolean; saving: boolean; error: string | null };
type GUISettingsPayload = { chat?: Partial<ChatPreferences> };
const initialState: PreferenceState = { chat: { autoCloseToolCalls: false, startToolCallsCollapsed: false }, ready: false, saving: false, error: null };
let state = initialState;
let loading: Promise<void> | undefined;
let writes: Promise<void> = Promise.resolve();
const listeners = new Set<() => void>();
const oldKeys: Record<keyof ChatPreferences, string> = {
  autoCloseToolCalls: "solomon.chat.auto-collapse-tool-calls",
  startToolCallsCollapsed: "solomon.chat.start-tool-calls-collapsed",
};

function publish(next: PreferenceState) {
  state = next;
  for (const listener of listeners) listener();
}

async function requestSettings(patch?: Partial<ChatPreferences>): Promise<GUISettingsPayload> {
  const controller = new AbortController();
  let timer: number | undefined;
  const timeout = new Promise<never>((_, reject) => {
    timer = window.setTimeout(() => {
      reject(new Error(patch ? "Saving chat settings timed out. Please try again." : "Loading chat settings timed out. Please try again."));
      controller.abort();
    }, 10000);
  });
  try {
    return await Promise.race([fetchSettings(patch, controller.signal), timeout]);
  } catch (error) {
    if (error instanceof TypeError) throw new Error("Unable to connect to Solomon. Please try again.");
    throw error;
  } finally {
    window.clearTimeout(timer);
  }
}

async function fetchSettings(patch: Partial<ChatPreferences> | undefined, signal: AbortSignal): Promise<GUISettingsPayload> {
  const response = await fetch(await serverEndpoint("/__solomon/gui-settings"), patch ? {
    method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ chat: patch }), signal,
  } : { cache: "no-store", signal });
  if (!response.headers.get("Content-Type")?.includes("application/json")) {
    throw new Error("GUI settings are unavailable. Update Solomon and try again.");
  }
  const payload = await response.json();
  if (!response.ok) throw new Error(typeof payload.error === "string" ? payload.error : "Unable to access GUI settings");
  if (!payload.chat || typeof payload.chat !== "object") throw new Error("Invalid GUI settings response");
  return payload;
}

function resolvedChat(payload: GUISettingsPayload): ChatPreferences {
  return { autoCloseToolCalls: payload.chat?.autoCloseToolCalls === true, startToolCallsCollapsed: payload.chat?.startToolCallsCollapsed === true };
}

export function refreshChatPreferences(): Promise<void> {
  if (loading) return loading;
  loading = (async () => {
    try {
      let payload = await requestSettings();
      const migration: Partial<ChatPreferences> = {};
      for (const key of Object.keys(oldKeys) as Array<keyof ChatPreferences>) {
        if (typeof payload.chat?.[key] !== "boolean") {
          try { migration[key] = window.localStorage.getItem(oldKeys[key]) === "true"; }
          catch { migration[key] = false; }
        }
      }
      if (Object.keys(migration).length) payload = await requestSettings(migration);
      // The config is now authoritative. Browser values are only migration input.
      for (const key of Object.values(oldKeys)) {
        try { window.localStorage.removeItem(key); } catch { /* Storage may be disabled. */ }
      }
      publish({ ...state, chat: resolvedChat(payload), ready: true, error: null });
    } catch (error) {
      publish({ ...state, error: error instanceof Error ? error.message : "Unable to load GUI settings" });
    } finally { loading = undefined; }
  })();
  return loading;
}

function setPreference(key: keyof ChatPreferences, enabled: boolean): Promise<void> {
  publish({ ...state, saving: true, error: null });
  const write = writes.then(async () => {
    if (loading) await loading;
    if (!state.ready) await refreshChatPreferences();
    if (!state.ready) throw new Error(state.error || "GUI settings have not loaded");
    const payload = await requestSettings({ [key]: enabled });
    publish({ ...state, chat: resolvedChat(payload), ready: true, error: null });
  }).catch(error => {
    publish({ ...state, error: error instanceof Error ? error.message : "Unable to save GUI settings" });
  });
  writes = write;
  return write.finally(() => {
    if (writes === write) publish({ ...state, saving: false });
  });
}

function subscribe(onChange: () => void) {
  const first = listeners.size === 0;
  listeners.add(onChange);
  if (first) {
    window.addEventListener("focus", onFocus);
    void refreshChatPreferences();
  }
  return () => {
    listeners.delete(onChange);
    if (!listeners.size) window.removeEventListener("focus", onFocus);
  };
}

function onFocus() { if (!state.saving) void refreshChatPreferences(); }

export function setAutoCollapseToolCalls(enabled: boolean) { return setPreference("autoCloseToolCalls", enabled); }
export function setStartToolCallsCollapsed(enabled: boolean) { return setPreference("startToolCallsCollapsed", enabled); }
export function useGUIChatPreferencesState() { return useSyncExternalStore(subscribe, () => state, () => initialState); }
export function useAutoCollapseToolCalls() { return useGUIChatPreferencesState().chat.autoCloseToolCalls; }
export function useStartToolCallsCollapsed() { return useGUIChatPreferencesState().chat.startToolCallsCollapsed; }
