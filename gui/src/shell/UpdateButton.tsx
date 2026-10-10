import { useEffect, useRef, useState } from "react";
import { serverEndpoint } from "../platform";
import "./update-button.css";

type UpdateStatus = {
  current: string;
  latest?: string;
  phase: "idle" | "checking" | "available" | "downloading" | "ready" | "restarting";
  error?: string;
};

export function UpdateButton() {
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

  if (!status || status.phase === "idle" || status.phase === "checking") return null;
  const busy = pending || status.phase === "downloading" || status.phase === "restarting";
  const label = status.phase === "ready" ? "Restart to update"
    : status.phase === "downloading" ? "Downloading update…"
    : status.phase === "restarting" ? "Restarting to update…"
    : "Download update";
  const message = error || status.error;

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

  return (
    <div className="solomon-update-control">
      <button className={`solomon-update-button${busy ? " is-busy" : ""}`} type="button"
        aria-label={label} disabled={busy} onClick={() => void act()}
        title={`${label}${status.latest ? ` (${status.latest})` : ""}`}>
        <svg aria-hidden="true" viewBox="0 0 24 24">
          <path d="M14 5c2-2 5-2 5-2s0 3-2 5l-5 5-4-4 6-4Z" />
          <path d="m8 9-4 1-1 4 5-1m4 0-1 5 4-1 1-4M6 16c-2 0-3 2-3 5 3 0 5-1 5-3" />
          <circle cx="15" cy="7" r="1" />
        </svg>
      </button>
      {message ? <div className="solomon-update-error" role="alert">{message}</div> : null}
    </div>
  );
}
