import "./provider-actions.css";
import { ProviderIcon } from "../home/ModelControl";
import { BankedResetButton } from "../home/BankedResetButton";
import { liveQuotaDetail, type ProviderCatalog, type ProviderQuota } from "../projects/projects";
export function ProviderRow({
  onRemove,
  onRelogin,
  provider,
  quota,
  quotaNow,
  reloginBusy,
  onQuotaUpdated,
}: {
  onRemove: () => void;
  onRelogin: () => void;
  provider: ProviderCatalog;
  quota?: ProviderQuota;
  quotaNow: number;
  reloginBusy: boolean;
  onQuotaUpdated: (quotas: ProviderQuota[]) => void;
}) {
  const bars = quota?.bars ?? [];
  const hasQuota = bars.length > 0 || Boolean(quota?.error);
  const canRelogin = Boolean(quota?.canRelogin) && Boolean(reloginKind(provider.provider));
  return (
    <section className="settings-provider-section">
      <div className="settings-provider-row">
        <div className="settings-provider-summary">
          <span className="settings-provider-row-main">
            <span className="settings-provider-icon"><ProviderIcon provider={provider.provider} /></span>
            <span className="settings-provider-row-copy">
              <strong>{provider.provider}</strong>
              <small>{provider.models.length} {provider.models.length === 1 ? "model" : "models"}{provider.complete ? "" : " · cached"}</small>
            </span>
          </span>
        </div>
        <button aria-label={`Remove provider ${provider.provider}`} className="settings-provider-relogin settings-provider-remove" type="button" onClick={onRemove}>Remove provider</button>
      </div>
      {hasQuota ? (
        <div className="settings-provider-quota">
          {quota?.error ? (
            <div className="settings-provider-quota-error-row">
              <p className="settings-provider-quota-error" role="alert">{quota.error}</p>
              {canRelogin ? (
                <button className="settings-provider-relogin" disabled={reloginBusy} onClick={onRelogin} type="button">
                  {reloginBusy ? "Signing in…" : "Sign in again"}
                </button>
              ) : null}
            </div>
          ) : null}
          {bars.map((bar) => (
            <div className="settings-provider-quota-bar" key={bar.label}>
              <div className="settings-provider-quota-label">
                <span>{bar.label}{bar.detail ? ` · ${liveQuotaDetail(bar.detail, quotaNow)}` : ""}</span>
                {bar.hidePercent ? null : <span>{Math.round(bar.percent)}%</span>}
              </div>
              {bar.hidePercent ? null : (
                <div aria-hidden="true" className={`settings-provider-quota-track${bar.percent >= 100 ? " is-limited" : ""}`}>
                  <span style={{ width: `${Math.round(bar.percent)}%` }} />
                </div>
              )}
              {(bar.availableResets ?? 0) > 0 ? <BankedResetButton provider={provider.provider} available={bar.availableResets!} onUpdated={onQuotaUpdated} /> : null}
            </div>
          ))}
        </div>
      ) : null}
    </section>
  );
}


import { reloginKind } from "./ModelsPage";
