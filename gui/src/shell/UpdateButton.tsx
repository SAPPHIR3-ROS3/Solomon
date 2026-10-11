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
        <span className="solomon-update-icon" aria-hidden="true">
          {busy ? <svg className="solomon-update-spinner" viewBox="0 0 32 32">
            <circle className="solomon-update-track" cx="16" cy="16" r="14" />
            <circle className="solomon-update-arc" cx="16" cy="16" r="14" />
          </svg> : null}
          {status.phase === "ready" || status.phase === "restarting" ? <>
            <svg viewBox="0 0 24 24">
              <path d="M3 11a9 9 0 1 1 2.6 6.4M3 4v7h7" />
            </svg>
            {!busy ? <span className="solomon-update-check">
              <svg viewBox="0 0 24 24"><path d="m5 12 4 4L19 6" /></svg>
            </span> : null}
          </> : <>
            <svg viewBox="0 0 24 24">
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3" />
            </svg>
            {!busy ? <span className="solomon-update-dot" /> : null}
          </>}
        </span>
      </button>
      {message ? <div className="solomon-update-error" role="alert">{message}</div> : null}
    </div>
  );
}
