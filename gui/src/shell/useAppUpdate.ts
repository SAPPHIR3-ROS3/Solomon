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
        const response = await fetch(await serverEndpoint("/__solomon/update"), { cache: "no-store", signal: controller.signal });
        if (!response.ok) return;
        const next: UpdateStatus = await response.json();
        if (controller.signal.aborted) return;
        if (!actionInFlight.current && version === actionVersion.current) setStatus(next);
        if (next.phase === "checking" || next.phase === "downloading" || next.phase === "restarting") delay = 2000;
      } catch {
        // Keep the update visible while the daemon restarts or reconnects.
        delay = 2000;
      } finally {
        if (!controller.signal.aborted) timer = window.setTimeout(() => void poll(), delay);
      }
    };
    void poll();
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
      const response = await fetch(await serverEndpoint("/__solomon/update"), {
        method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ action: status.phase === "ready" ? "install" : "download" }),
      });
      const next = await response.json();
      if (!response.ok) throw new Error(next.error || "Unable to update Solomon. Try again.");
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
