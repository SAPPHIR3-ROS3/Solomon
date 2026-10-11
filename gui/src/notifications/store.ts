import { useSyncExternalStore } from "react";
import { serverEndpoint } from "../platform";

export type Notification = { id: string; createdAt: string; level: string; message: string };
export type Toast = Notification & { expiresAt: number };
const storageKey = "solomon:notification-errors";
let local: Notification[] = [];
try { local = JSON.parse(localStorage.getItem(storageKey) ?? "[]").filter(isNotification); } catch { /* Storage can be unavailable. */ }
let history: Notification[] = local;
let toasts: Toast[] = [];
let snapshot = { history, toasts };
const listeners = new Set<() => void>();
function publish() {
  snapshot = { history, toasts };
  listeners.forEach((listener) => listener());
}
function isNotification(value: unknown): value is Notification {
  if (!value || typeof value !== "object") return false;
  const entry = value as Notification;
  return [entry.id, entry.createdAt, entry.level, entry.message].every((field) => typeof field === "string");
}
function toast(entry: Notification) {
  if (toasts.some((current) => current.message === entry.message)) return;
  toasts = [...toasts, { ...entry, expiresAt: Date.now() + 10_000 }];
}
export function notifyError(message: string) { notifyNotification(message, "error"); }
export function notifyNotification(message: string, level = "system") {
  if (!message.trim()) return;
  if (history.some((entry) => entry.message === message && Date.now() - Date.parse(entry.createdAt) < 10_000)) return;
  const entry = { id: crypto.randomUUID(), createdAt: new Date().toISOString(), level, message };
  local = [entry, ...local];
  try { localStorage.setItem(storageKey, JSON.stringify(local)); } catch { /* Keep the in-memory history. */ }
  history = [entry, ...history];
  toast(entry);
  publish();
}
export function dismissToast(id: string) {
  toasts = toasts.filter((entry) => entry.id !== id);
  publish();
}
export function useNotifications() {
  return useSyncExternalStore((listener) => { listeners.add(listener); return () => { listeners.delete(listener); }; }, () => snapshot);
}

// The first successful fetch loads history silently. Only later arrivals produce toasts.
export function watchNotifications() {
  const controller = new AbortController();
  let seen: Set<string> | null = null;
  let timer: ReturnType<typeof setTimeout>;
  async function poll() {
    try {
      const response = await fetch(await serverEndpoint("/__solomon/notifications"), { signal: controller.signal });
      if (!response.ok) throw new Error("Unable to load notifications");
      const payload: unknown = await response.json();
      if (!Array.isArray(payload)) throw new Error("Invalid notification history");
      const entries = payload.filter(isNotification);
      if (controller.signal.aborted) return;
      if (seen) entries.filter((entry) => !seen!.has(entry.id)).reverse().forEach(toast);
      seen = new Set(entries.map((entry) => entry.id));
      const combined = [...entries, ...local.filter((entry) => !entries.some((remote) => remote.message === entry.message && Math.abs(Date.parse(remote.createdAt) - Date.parse(entry.createdAt)) < 10_000))];
      history = combined.sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt));
      publish();
    } catch { /* Reconnect without generating repeated connection errors. */ }
    if (!controller.signal.aborted) timer = setTimeout(poll, 2000);
  }
  void poll();
  return () => { controller.abort(); clearTimeout(timer); };
}
