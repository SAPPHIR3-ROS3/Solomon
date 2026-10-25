import { useEffect, useRef, useState } from "react";
import { serverEndpoint } from "../platform";

type UpdateStatus = {
  current: string;
  downloaded?: number;
  total?: number;
  latest?: string;
  phase: "idle" | "checking" | "available" | "downloading" | "ready" | "restarting";
  error?: string;
  notice?: string;
};

// Bound both the response and its body: a connection left open during restart
// must not stop all subsequent status checks.
async function updateRequest(init: RequestInit = {}, signal?: AbortSignal) {
  const controller = new AbortController();
  const abort = () => controller.abort();
  signal?.addEventListener("abort", abort, { once: true });
  if (signal?.aborted) controller.abort();
  const timeout = window.setTimeout(abort, 10_000);
  try {
    return await new Promise<UpdateStatus>((resolve, reject) => {
      const onAbort = () => reject(new Error("Update request aborted."));
      controller.signal.addEventListener("abort", onAbort, { once: true });
      if (controller.signal.aborted) onAbort();
      const request = async () => {
        const endpoint = await serverEndpoint("/__solomon/update");
        if (controller.signal.aborted) throw new Error("Update request aborted.");
        const response = await fetch(endpoint, {
          ...init, cache: "no-store", signal: controller.signal,
        });
        const next: UpdateStatus = await response.json();
        if (!response.ok) throw new Error(next.error || "Unable to update Solomon. Try again.");
        return next;
      };
      void request().then(resolve, reject).finally(() => {
        controller.signal.removeEventListener("abort", onAbort);
      });
    });
  } catch (cause) {
    if (controller.signal.aborted && !signal?.aborted) {
      throw new Error("Update request timed out. Try again.");
    }
    throw cause;
  } finally {
    window.clearTimeout(timeout);
    signal?.removeEventListener("abort", abort);
  }
}

export function useAppUpdate() {
  const [status, setStatus] = useState<UpdateStatus | null>(null);
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);
  const actionInFlight = useRef(false);
  const actionVersion = useRef(0);

  useEffect(() => {
    const controller = new AbortController();
    let timer: number | undefined;
    const poll = async () => {
      let delay = 30_000;
      const version = actionVersion.current;
      try {
        const next = await updateRequest({}, controller.signal);
        if (controller.signal.aborted) return;
        if (!actionInFlight.current && version === actionVersion.current) {
          setStatus(next);
          setError("");
        }
        if (next.phase === "checking" || next.phase === "downloading" || next.phase === "restarting") delay = 2000;
      } catch {
        // Keep the update visible while the daemon restarts or reconnects.
        delay = 2000;
      } finally {
        if (!controller.signal.aborted) timer = window.setTimeout(() => void poll(), delay);
      }
    };
    if (status?.phase) {
      const active = status.phase === "checking" || status.phase === "downloading" || status.phase === "restarting";
      timer = window.setTimeout(() => void poll(), active ? 2000 : 30_000);
    } else {
      void poll();
    }
    return () => { controller.abort(); window.clearTimeout(timer); };
  }, [status?.phase]);

  const busy = pending || status?.phase === "downloading" || status?.phase === "restarting";

  async function act() {
    if (!status || busy || actionInFlight.current) return;
    actionInFlight.current = true;
    actionVersion.current += 1;
    setPending(true);
    setError("");
    try {
      const next = await updateRequest({
        method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ action: status.phase === "ready" ? "install" : "download" }),
      });
      setStatus(next);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Unable to update Solomon. Try again.");
    } finally {
      actionInFlight.current = false;
      setPending(false);
    }
  }

  return { status, error, busy, act };
}

export type AppUpdate = ReturnType<typeof useAppUpdate>;
