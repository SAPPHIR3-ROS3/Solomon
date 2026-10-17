import { useRef, useState } from "react";
import { consumeProviderReset, fetchProviderQuotas, type ProviderQuota } from "../projects/models";
import "./banked-reset.css";

export function BankedResetButton({ provider, available, onUpdated }: { provider: string; available: number; onUpdated: (quotas: ProviderQuota[]) => void }) {
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [failed, setFailed] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const trigger = useRef<HTMLButtonElement>(null);
  const pending = useRef(false);
  const key = useRef<string | null>(null);
  async function useReset() {
    if (!confirming || pending.current || available < 1) return;
    pending.current = true;
    setConfirming(false);
    setBusy(true);
    setMessage("");
    setFailed(false);
    key.current ??= crypto.randomUUID();
    try {
      const code = await consumeProviderReset(provider, key.current);
      key.current = null;
      setMessage(code === "no_credit" ? "No resets available." : code === "nothing_to_reset" ? "No usage limit needs resetting." : "Reset applied.");
      try { onUpdated(await fetchProviderQuotas()); }
      catch { setMessage(current => current + " Unable to refresh quotas. Reopen this panel to refresh."); }
    } catch (error) {
      setFailed(true);
      setMessage(error instanceof Error ? error.message : "Unable to use reset.");
    } finally { pending.current = false; setBusy(false); }
  }
  function cancel() {
    setConfirming(false);
    trigger.current?.focus();
  }
  return <div className="provider-banked-reset">
    <button ref={trigger} className="provider-banked-reset-button" aria-busy={busy} aria-expanded={confirming} disabled={busy || available < 1} onClick={() => setConfirming(true)} type="button">{busy ? "Applying reset…" : "Use available reset"}</button>
    {confirming ? <div className="provider-banked-reset-confirm" role="group" aria-label="Confirm use of banked reset" onKeyDown={event => { if (event.key === "Escape") { event.stopPropagation(); cancel(); } }}>
      <p>Use one banked reset for {provider}? This will consume one available reset.</p>
      <div className="provider-banked-reset-actions">
        <button autoFocus className="provider-banked-reset-button" onClick={cancel} type="button">Cancel</button>
        <button className="provider-banked-reset-button" disabled={busy || available < 1} onClick={() => void useReset()} type="button">Confirm reset</button>
      </div>
    </div> : null}
    {message ? <p role={failed ? "alert" : "status"}>{message}</p> : null}
  </div>;
}
