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
  const downloading = status.phase === "downloading";
  const percent = status.total && status.total > 0
    ? Math.min(100, Math.max(0, (status.downloaded || 0) / status.total * 100)) : undefined;
  const progressLabel = percent === undefined
    ? `${((status.downloaded || 0) / 1048576).toFixed(1)} MB` : `${Math.floor(percent)}%`;

  return (
    <div className="solomon-update-control">
      <button className={`solomon-update-button${busy ? " is-busy" : ""}`} type="button"
        aria-label={label} disabled={busy} onClick={() => void act()}
        title={`${label}${status.latest ? ` (${status.latest})` : ""}${status.notice ? `\n${status.notice}` : ""}`}>
        <span className="solomon-update-icon" aria-hidden="true">
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
      {downloading ? <div className="solomon-update-progress" role="progressbar"
        aria-label="Downloading update" aria-valuemin={0} aria-valuemax={100}
        aria-valuenow={percent} aria-valuetext={progressLabel}>
        <span>{progressLabel}</span>
        <div className="solomon-update-progress-track"><span style={{ width: `${percent ?? 0}%` }} /></div>
      </div> : null}
      {message ? <div className="solomon-update-error" role="alert">{message}</div> : null}
    </div>
  );
}
