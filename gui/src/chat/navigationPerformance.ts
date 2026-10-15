// Bounded, local diagnostics. Never record message text, titles, paths or attachments.
const STORAGE_KEY = "solomon.chat-navigation-performance.v1";
const LIMIT = 100;
declare const __SOLOMON_GUI_BUILD_ID__: string;
const buildID = typeof __SOLOMON_GUI_BUILD_ID__ === "undefined" ? "development-or-test" : __SOLOMON_GUI_BUILD_ID__;
export type NavigationSpan = { phase: string; startMs: number; durationMs: number; outcome: "ok" | "error" };
export type NavigationSample = {
  id: number; buildID: string; projectID: string; chatID: string; timestamp: string; timeOrigin: number; startedAt: number;
  cached: boolean; messageCount: number; contentChars: number;
  selectedMs?: number; commitMs?: number; paintOpportunityMs?: number;
  endedAt?: number;
  outcome: "pending" | "ready" | "cancelled" | "error";
  spans: NavigationSpan[]; longTasks: { startMs: number; durationMs: number }[];
  highlighting?: { count: number; totalMs: number; maxMs: number; lastFinishedMs: number };
};
let samples: NavigationSample[] = [];
let sequence = Date.now();
let initialized = false;
let persistenceTimer: ReturnType<typeof setTimeout> | undefined;
let observer: PerformanceObserver | undefined;
let active: number | undefined;
const now = () => performance.now();
const round = (value: number) => Math.round(value * 10) / 10;
function persistLater() {
  if (typeof window === "undefined" || persistenceTimer !== undefined) return;
  persistenceTimer = setTimeout(() => {
    persistenceTimer = undefined;
    try { localStorage.setItem(STORAGE_KEY, JSON.stringify(samples)); } catch { /* In-memory diagnostics still work. */ }
  }, 1000);
}
function find(id?: number) { return id === undefined ? undefined : samples.find(sample => sample.id === id); }
function initialize() {
  if (initialized || typeof window === "undefined") return;
  initialized = true;
  try {
    const saved = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? "[]");
    if (Array.isArray(saved)) samples = saved.filter(sample => typeof sample?.id === "number" && Array.isArray(sample.spans) && Array.isArray(sample.longTasks)).slice(-LIMIT).map(sample => sample.outcome === "pending" ? { ...sample, outcome: "cancelled" } : sample);
  } catch { /* A corrupt or unavailable cache must never prevent navigation. */ }
  try {
    if (PerformanceObserver.supportedEntryTypes.includes("longtask")) {
      observer = new PerformanceObserver(list => {
        for (const entry of list.getEntries()) {
          for (const sample of samples) {
            const end = sample.endedAt ?? sample.startedAt + (sample.paintOpportunityMs ?? (now() - sample.startedAt)) + 500;
            if (sample.timeOrigin !== performance.timeOrigin || entry.startTime + entry.duration < sample.startedAt || entry.startTime > end) continue;
            if (sample.longTasks.length < 100) sample.longTasks.push({ startMs: round(entry.startTime - sample.startedAt), durationMs: round(entry.duration) });
          }
        }
        persistLater();
      });
      observer.observe({ type: "longtask", buffered: true });
    }
  } catch { /* Some WebViews do not implement long-task observation. */ }
  window.solomonPerformance = { report: navigationReport, clear: clearNavigationPerformance, download: downloadNavigationPerformance };
}
export function beginNavigation(projectID: string, chatID: string): number {
  initialize();
  const previous = find(active);
  if (previous && previous.paintOpportunityMs === undefined && previous.outcome === "pending") { previous.outcome = "cancelled"; previous.endedAt = now(); }
  const sample: NavigationSample = {
    id: ++sequence, buildID, projectID, chatID, timestamp: new Date().toISOString(), timeOrigin: performance.timeOrigin, startedAt: now(),
    cached: false, messageCount: 0, contentChars: 0, outcome: "pending", spans: [], longTasks: [],
  };
  active = sample.id;
  samples.push(sample);
  samples = samples.slice(-LIMIT);
  persistLater();
  return sample.id;
}
export function cancelNavigation(id?: number) {
  const sample = find(id);
  if (sample?.outcome === "pending") { sample.outcome = "cancelled"; sample.endedAt = now(); }
  if (active === id) active = undefined;
  persistLater();
}
export function setNavigationCache(id: number, cached: boolean) {
  const sample = find(id);
  if (sample) sample.cached = cached;
}
export function markNavigationSelection(id: number) {
  const sample = find(id);
  if (sample && sample.selectedMs === undefined) sample.selectedMs = round(now() - sample.startedAt);
  persistLater();
}
export function failNavigation(id: number) {
  const sample = find(id);
  if (sample && sample.outcome !== "cancelled") { sample.outcome = "error"; sample.endedAt = now(); }
  persistLater();
}
export function navigationForChat(projectID: string | undefined, chatID: string): number | undefined {
  const sample = find(active);
  return sample && sample.projectID === projectID && sample.chatID === chatID && sample.outcome !== "cancelled" ? sample.id : undefined;
}
export function currentNavigationID() {
  const sample = find(active);
  if (!sample || sample.outcome === "cancelled" || (sample.paintOpportunityMs !== undefined && now() > sample.startedAt + sample.paintOpportunityMs + 500)) return undefined;
  return active;
}
export async function measureNavigationHighlight<T>(id: number | undefined, operation: () => Promise<T>): Promise<T> {
  const start = now();
  try { return await operation(); }
  finally {
    const sample = find(id);
    if (sample) {
      const duration = now() - start;
      const previous = sample.highlighting;
      sample.highlighting = { count: (previous?.count ?? 0) + 1, totalMs: round((previous?.totalMs ?? 0) + duration),
        maxMs: round(Math.max(previous?.maxMs ?? 0, duration)), lastFinishedMs: round(now() - sample.startedAt) };
      persistLater();
    }
  }
}
export function markNavigationCommit(id: number | undefined, messageCount: number, contentChars: number) {
  const sample = find(id);
  if (!sample || sample.commitMs !== undefined || sample.outcome === "cancelled") return () => {};
  sample.commitMs = round(now() - sample.startedAt);
  sample.messageCount = messageCount;
  sample.contentChars = contentChars;
  // Two animation frames give the browser a paint opportunity; this is a proxy,
  // not a claim that pixels have reached the display or syntax highlighting finished.
  let second: number | undefined;
  const first = requestAnimationFrame(() => {
    second = requestAnimationFrame(() => {
      if (active !== id || sample.outcome === "cancelled") return;
      sample.paintOpportunityMs = round(now() - sample.startedAt);
      if (sample.outcome === "pending") sample.outcome = "ready";
      persistLater();
    });
  });
  persistLater();
  return () => {
    cancelAnimationFrame(first);
    if (second !== undefined) cancelAnimationFrame(second);
    // StrictMode can remount effects before the first frame.
    if (sample.paintOpportunityMs === undefined) sample.commitMs = undefined;
  };
}
function finishSpan(id: number | undefined, phase: string, start: number, outcome: NavigationSpan["outcome"]) {
  const sample = find(id);
  if (!sample || sample.spans.length >= 100) return;
  sample.spans.push({ phase, startMs: round(start - sample.startedAt), durationMs: round(now() - start), outcome });
  persistLater();
}
export async function measureNavigation<T>(id: number | undefined, phase: string, operation: () => Promise<T>): Promise<T> {
  const start = now();
  try { const result = await operation(); finishSpan(id, phase, start, "ok"); return result; }
  catch (error) { finishSpan(id, phase, start, "error"); throw error; }
}
export function measureNavigationSync<T>(id: number | undefined, phase: string, operation: () => T): T {
  const start = now();
  try { const result = operation(); finishSpan(id, phase, start, "ok"); return result; }
  catch (error) { finishSpan(id, phase, start, "error"); throw error; }
}
export function navigationReport() {
  initialize();
  return { schemaVersion: 1, buildID, exportedAt: new Date().toISOString(), userAgent: typeof navigator === "undefined" ? "" : navigator.userAgent,
    longTasksSupported: typeof PerformanceObserver !== "undefined" && PerformanceObserver.supportedEntryTypes.includes("longtask"),
    paintMetric: "Two requestAnimationFrame callbacks after layout commit; excludes completion of asynchronous highlighting.",
    samples: JSON.parse(JSON.stringify(samples)) as NavigationSample[] };
}
export function clearNavigationPerformance() {
  samples = []; active = undefined;
  if (persistenceTimer !== undefined) clearTimeout(persistenceTimer);
  persistenceTimer = undefined;
  try { localStorage.removeItem(STORAGE_KEY); } catch { /* Optional storage. */ }
}
export function downloadNavigationPerformance() {
  const url = URL.createObjectURL(new Blob([JSON.stringify(navigationReport(), null, 2)], { type: "application/json" }));
  const anchor = document.createElement("a");
  anchor.href = url; anchor.download = "solomon-chat-performance.json"; anchor.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
export function navigationSummary(values: NavigationSample[]) {
  const durations = values.flatMap(sample => sample.paintOpportunityMs === undefined || sample.outcome !== "ready" ? [] : [sample.paintOpportunityMs]).sort((a, b) => a - b);
  const percentile = (p: number) => durations.length ? durations[Math.ceil(durations.length * p) - 1] : undefined;
  return { count: durations.length, medianMs: percentile(0.5), p95Ms: percentile(0.95), maxMs: durations.at(-1) };
}
declare global {
  interface Window { solomonPerformance: { report: typeof navigationReport; clear: typeof clearNavigationPerformance; download: typeof downloadNavigationPerformance }; }
}
