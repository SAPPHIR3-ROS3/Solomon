import { useEffect, useId, useRef, useState } from "react";
import { fetchProviderQuotas, getCachedProviderQuotas, liveQuotaDetail, type ProviderQuota } from "../projects/models";
import "./provider-quota.css";
import { BankedResetButton } from "./BankedResetButton";

export function ProviderQuotaControl({ provider, open, onOpenChange }: { provider: string; open: boolean; onOpenChange: (open: boolean) => void }) {
  const ref = useRef<HTMLDivElement>(null);
  const id = useId();
  const [quotas, setQuotas] = useState<ProviderQuota[]>(() => getCachedProviderQuotas() ?? []);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [now, setNow] = useState(Date.now);
  const [revision, setRevision] = useState(0);

  useEffect(() => {
    let active = true;
    setLoading(true);
    setError("");
    void fetchProviderQuotas().then((data) => {
      if (active) setQuotas(data);
    }).catch(() => {
      if (active) setError("Unable to refresh quotas.");
    }).finally(() => {
      if (active) setLoading(false);
    });
    return () => { active = false; };
  }, [open, provider, revision]);

  useEffect(() => {
    const timer = window.setInterval(() => setRevision((value) => value + 1), 60 * 1000);
    return () => window.clearInterval(timer);
  }, []);

  useEffect(() => {
    if (!open) return;
    setNow(Date.now());
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    const close = (event: PointerEvent) => {
      if (!ref.current?.contains(event.target as Node)) onOpenChange(false);
    };
    const escape = (event: KeyboardEvent) => {
      if (event.key === "Escape") { onOpenChange(false); ref.current?.querySelector<HTMLButtonElement>("button")?.focus(); }
    };
    document.addEventListener("pointerdown", close);
    document.addEventListener("keydown", escape);
    return () => {
      window.clearInterval(timer);
      document.removeEventListener("pointerdown", close);
      document.removeEventListener("keydown", escape);
    };
  }, [open, onOpenChange]);

  const quota = quotas.find((entry) => entry.provider === provider);
  const previewBars = quota?.bars.filter((bar) => !bar.hidePercent) ?? [];
  const previewTitle = previewBars.map((bar) => `${bar.label}: ${Math.round(bar.percent)}%`).join("; ");
  return <div className="provider-quota-control" ref={ref}>
    <button aria-label="Provider quota" aria-expanded={open} aria-controls={open ? id : undefined} aria-haspopup="dialog" className="notification-bell provider-quota-trigger" title={previewTitle || "Provider quota and credits"} type="button" onMouseEnter={() => setRevision((value) => value + 1)} onClick={() => onOpenChange(!open)}>
      <span aria-hidden="true" className="provider-quota-preview">
        {previewBars.length ? previewBars.map((bar, index) => <span className={`provider-quota-preview-track${bar.percent >= 100 ? " is-limited" : ""}`} key={`${bar.label}:${index}`}><span style={{ width: `${Math.max(0, Math.min(100, bar.percent))}%` }} /></span>) : <span className="provider-quota-preview-track is-unavailable" />}
      </span>
    </button>
    {open ? <section aria-label="Provider quota" aria-busy={loading} className="provider-quota-menu" id={id} role="dialog">
      <header><strong>{provider || "Provider quota"}</strong>{loading ? <span role="status">Refreshing...</span> : null}</header>
      <div className="provider-quota-content">
        {error ? <p role="alert">{error} {quota ? "Showing cached data." : ""}</p> : null}
        {quota?.error ? <p role="alert">{quota.error}</p> : null}
        {quota?.bars.map((bar, index) => <div className="provider-quota-bar" key={`${bar.label}:${index}`}>
          <div className="provider-quota-label"><span>{bar.label}</span>{bar.hidePercent ? null : <strong>{Math.round(bar.percent)}%</strong>}</div>
          {bar.hidePercent ? null : <div aria-label={bar.label} role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(bar.percent)} className={`provider-quota-track${bar.percent >= 100 ? " is-limited" : ""}`}><span style={{ width: `${bar.percent}%` }} /></div>}
          {bar.detail ? <small>{liveQuotaDetail(bar.detail, now)}</small> : null}
          {(bar.availableResets ?? 0) > 0 ? <BankedResetButton key={provider} provider={provider} available={bar.availableResets!} onUpdated={setQuotas} /> : null}
        </div>)}
        {!quota?.bars.length && !quota?.error ? <p>{!provider ? "Select a provider to see its quota." : loading ? "Loading quota and credits..." : "Quota and credits are not available for this provider."}</p> : null}
      </div>
    </section> : null}
  </div>;
}
