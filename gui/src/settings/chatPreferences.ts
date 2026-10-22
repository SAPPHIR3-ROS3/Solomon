import { useSyncExternalStore } from "react";
import { serverEndpoint } from "../platform";

export type FloatingLettersDensity = "low" | "medium" | "high" | "max";

type ChatPreferences = { floatingLettersDensity: FloatingLettersDensity; floatingLetters: boolean; autoCloseToolCalls: boolean; startToolCallsCollapsed: boolean; startReasoningCollapsed: boolean; composerQuotaBars: boolean };
type PreferenceState = { userName: string; responseLanguage: string; advanced: Record<string, string | number | boolean>; chat: ChatPreferences; ready: boolean; saving: boolean; error: string | null };
type GUISettingsPayload = { general?: { userName?: string; responseLanguage?: string }; advanced?: Record<string, string | number | boolean>; chat?: Partial<ChatPreferences> };
type SettingsPatch = { chat?: Partial<ChatPreferences>; general?: { userName?: string; responseLanguage?: string }; advanced?: Record<string, string | number | boolean> };
const initialState: PreferenceState = { userName: "", advanced: {}, responseLanguage: "", chat: { floatingLettersDensity: "medium", floatingLetters: false, autoCloseToolCalls: false, startToolCallsCollapsed: false, startReasoningCollapsed: false, composerQuotaBars: true }, ready: false, saving: false, error: null };
let state = initialState;
let loading: Promise<void> | undefined;
let writes: Promise<void> = Promise.resolve();
const listeners = new Set<() => void>();
const oldKeys = {
  autoCloseToolCalls: "solomon.chat.auto-collapse-tool-calls",
  startToolCallsCollapsed: "solomon.chat.start-tool-calls-collapsed",
};

function publish(next: PreferenceState) {
  state = next;
  for (const listener of listeners) listener();
}

async function requestSettings(patch?: SettingsPatch): Promise<GUISettingsPayload> {
  const controller = new AbortController();
  let timer: number | undefined;
  const timeout = new Promise<never>((_, reject) => {
    timer = window.setTimeout(() => {
      reject(new Error(patch ? "Saving settings timed out. Please try again." : "Loading settings timed out. Please try again."));
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

async function fetchSettings(patch: SettingsPatch | undefined, signal: AbortSignal): Promise<GUISettingsPayload> {
  const response = await fetch(await serverEndpoint("/__solomon/gui-settings"), patch ? {
    method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify(patch), signal,
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
  return { floatingLettersDensity: payload.chat?.floatingLettersDensity === "low" || payload.chat?.floatingLettersDensity === "high" || payload.chat?.floatingLettersDensity === "max" ? payload.chat.floatingLettersDensity : "medium", floatingLetters: payload.chat?.floatingLetters === true, autoCloseToolCalls: payload.chat?.autoCloseToolCalls === true, startToolCallsCollapsed: payload.chat?.startToolCallsCollapsed === true, startReasoningCollapsed: payload.chat?.startReasoningCollapsed === true, composerQuotaBars: payload.chat?.composerQuotaBars !== false };
}

export function refreshChatPreferences(): Promise<void> {
  if (loading) return loading;
  loading = (async () => {
    try {
      let payload = await requestSettings();
      const migration: Partial<ChatPreferences> = {};
      for (const key of Object.keys(oldKeys) as Array<keyof typeof oldKeys>) {
        if (typeof payload.chat?.[key] !== "boolean") {
          try { migration[key] = window.localStorage.getItem(oldKeys[key]) === "true"; }
          catch { migration[key] = false; }
        }
      }
      if (Object.keys(migration).length) payload = await requestSettings({ chat: migration });
      // The config is now authoritative. Browser values are only migration input.
      for (const key of Object.values(oldKeys)) {
        try { window.localStorage.removeItem(key); } catch { /* Storage may be disabled. */ }
      }
      publish({ ...state, userName: payload.general?.userName ?? state.userName, advanced: payload.advanced ?? state.advanced, responseLanguage: payload.general?.responseLanguage ?? state.responseLanguage, chat: resolvedChat(payload), ready: true, error: null });
    } catch (error) {
      publish({ ...state, error: error instanceof Error ? error.message : "Unable to load GUI settings" });
    } finally { loading = undefined; }
  })();
  return loading;
}

function savePreferences(patch: SettingsPatch): Promise<void> {
  publish({ ...state, saving: true, error: null });
  const write = writes.then(async () => {
    if (loading) await loading;
    if (!state.ready) await refreshChatPreferences();
    if (!state.ready) throw new Error(state.error || "GUI settings have not loaded");
    const payload = await requestSettings(patch);
    publish({ ...state, userName: payload.general?.userName ?? state.userName, advanced: payload.advanced ?? state.advanced, responseLanguage: payload.general?.responseLanguage ?? state.responseLanguage, chat: resolvedChat(payload), ready: true, error: null });
  }).catch(error => {
    publish({ ...state, error: error instanceof Error ? error.message : "Unable to save GUI settings" });
    throw error;
  });
  const queued = write.catch(() => {});
  writes = queued;
  return write.finally(() => {
    if (writes === queued) publish({ ...state, saving: false });
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

function setPreference(key: keyof ChatPreferences, enabled: boolean) { return savePreferences({ chat: { [key]: enabled } }); }
export function setUserName(userName: string) { return savePreferences({ general: { userName } }); }
export function setAdvancedSetting(key: string, value: string | number | boolean) { return savePreferences({ advanced: { [key]: value } }); }
export function setResponseLanguage(responseLanguage: string) { return savePreferences({ general: { responseLanguage } }); }
export function setAutoCollapseToolCalls(enabled: boolean) { return setPreference("autoCloseToolCalls", enabled).catch(() => {}); }
export function setStartToolCallsCollapsed(enabled: boolean) { return setPreference("startToolCallsCollapsed", enabled).catch(() => {}); }
export function setStartReasoningCollapsed(enabled: boolean) { return setPreference("startReasoningCollapsed", enabled).catch(() => {}); }
export function setComposerQuotaBars(enabled: boolean) { return setPreference("composerQuotaBars", enabled).catch(() => {}); }
export function useGUIChatPreferencesState() { return useSyncExternalStore(subscribe, () => state, () => initialState); }
export function useAutoCollapseToolCalls() { return useGUIChatPreferencesState().chat.autoCloseToolCalls; }
export function useStartToolCallsCollapsed() { return useGUIChatPreferencesState().chat.startToolCallsCollapsed; }

export function setFloatingLetters(enabled: boolean) { return setPreference("floatingLetters", enabled).catch(() => {}); }

export function setFloatingLettersDensity(density: FloatingLettersDensity) { return savePreferences({ chat: { floatingLettersDensity: density } }).catch(() => {}); }

export function setShowFileChanges(enabled: boolean) { return setAdvancedSetting("show_file_changes", enabled); }
