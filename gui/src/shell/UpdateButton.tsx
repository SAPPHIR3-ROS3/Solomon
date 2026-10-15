import type { AppUpdate } from "./useAppUpdate";
import "./update-button.css";

export function UpdateButton({ update }: { update: AppUpdate }) {
  const { status, error, busy, act } = update;
  if (!status || status.phase === "idle" || status.phase === "checking") return null;
  const label = status.phase === "ready" ? "Restart to update"
    : status.phase === "downloading" ? "Downloading update…"
    : status.phase === "restarting" ? "Restarting to update…"
    : "Download update";
  const message = error || status.error;

  return (
    <div className="solomon-update-control">
      <button className={`solomon-update-button${busy ? " is-busy" : ""}`} type="button"
        aria-label={label} disabled={busy} onClick={() => void act()}
        title={`${label}${status.latest ? ` (${status.latest})` : ""}${status.notice ? `\n${status.notice}` : ""}`}>
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
