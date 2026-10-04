import { serverEndpoint } from "./platform";

// A temporary outage does not change the baseline. A healthy new daemon does.
export function daemonRestartTracker(): (payload: unknown) => boolean {
  let generation = "";
  return (payload) => {
    const health = payload as { ok?: boolean; server?: { pid?: number; started_at?: string; version?: string } } | null;
    if (!health?.ok || typeof health.server?.pid !== "number" || typeof health.server.started_at !== "string" || typeof health.server.version !== "string") return false;
    const next = `${health.server.pid}:${health.server.started_at}:${health.server.version}`;
    const changed = generation !== "" && next !== generation;
    generation = next;
    return changed;
  };
}

export function watchDaemonLifecycle(onRestart: () => void): () => void {
  const restarted = daemonRestartTracker();
  let stopped = false;
  let timer: number | undefined;
  let request: AbortController | undefined;
  const check = async () => {
    request = new AbortController();
    const timeout = window.setTimeout(() => request?.abort(), 4000);
    try {
      const response = await fetch(await serverEndpoint("/health"), { cache: "no-store", signal: request.signal });
      if (response.ok && restarted(await response.json()) && !stopped) {
        stopped = true;
        onRestart();
      }
    } catch {
      // Wait for the coordinator to finish before reloading the frontend.
    } finally {
      window.clearTimeout(timeout);
      if (!stopped) timer = window.setTimeout(() => { void check(); }, 2000);
    }
  };
  void check();
  return () => {
    stopped = true;
    window.clearTimeout(timer);
    request?.abort();
  };
}
