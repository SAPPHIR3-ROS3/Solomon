import { useEffect, useState } from "react";
import { isSensitiveNetworkAddress } from "./networkAddressPrivacy";
import { SensitiveText } from "../SensitiveText";
import { serverEndpoint } from "../platform";
import "./chat-settings.css";
import "./network-settings.css";

type NetworkInterface = { number: number; name: string; type?: string; display_name?: string; tailscale?: { tailnet?: string; dns_name?: string; magic_dns?: string; magic_dns_enabled?: boolean; account?: string; tagged?: boolean }; enabled: boolean; up: boolean; local: boolean; addresses: string[] };
export function NetworkSettings({ query }: { query: string }) {
  const [interfaces, setInterfaces] = useState<NetworkInterface[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  async function load(name?: string, enabled?: boolean) {
    setBusy(true); setError("");
    try {
      const response = await fetch(await serverEndpoint("/__solomon/network-interfaces"), name === undefined
        ? { cache: "no-store" }
        : { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ name, enabled }) });
      if (!response.ok) throw new Error(await response.text());
      setInterfaces(await response.json());
    } catch (error) { setError(String(error)); }
    finally { setBusy(false); }
  }
  useEffect(() => {
    void load();
    const refresh = () => { void load(); };
    window.addEventListener("focus", refresh);
    return () => window.removeEventListener("focus", refresh);
  }, []);
  if (!"network interfaces lan wifi wi-fi vpn tailscale internet".includes(query.trim().toLowerCase())) return null;
  return <section aria-label="Network" className="settings-models">
    <header className="settings-models-header"><h1>Network</h1></header>
    <div className="settings-models-content">
      <div className="settings-models-panel-heading network-heading">
        <div>
          <h2>Network interfaces</h2>
          <p>Choose where Solomon is reachable and which connections it can use.</p>
        </div>
        <button className="settings-chat-retry network-refresh" type="button" disabled={busy} onClick={() => void load()}>
          <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M20 7v5h-5M4 17v-5h5" /><path d="M6.1 7a7 7 0 0 1 11.5-1L20 9M4 15l2.4 3A7 7 0 0 0 17.9 17" /></svg>
          {busy ? "Updating…" : "Refresh"}
        </button>
      </div>
      {error ? <div className="settings-chat-error" role="alert"><p>{error}</p></div> : null}
      {!interfaces.length ? <p className="settings-models-empty" role="status">{busy ? "Loading interfaces…" : error ? "" : "No network interfaces found."}</p> : null}
      <div className="network-interfaces" aria-busy={busy}>
        {interfaces.map(item => {
          const labelID = `network-${item.number}-label`;
          const descriptionID = `network-${item.number}-description`;
          const addresses = item.addresses.filter(address => !address.includes(":"));
          return <div key={item.name} className={`settings-model-row network-interface${item.tailscale?.magic_dns ? " has-children" : ""}`}>
            <div className="settings-model-row-copy">
              <div className="network-interface-title">
                <strong id={labelID}>{item.display_name || item.name}</strong>
                <span className={`network-link-state${item.up ? " is-up" : ""}`}><span aria-hidden="true" />{item.up ? "Link up for Solomon" : "Link down"}</span>
              </div>
              {item.display_name && item.display_name !== item.name ? <code className="network-interface-name">{item.name}</code> : null}
              {item.local ? <small id={descriptionID}>Local access · cannot be disabled</small> : null}
              {item.tailscale ? <dl className="network-tailnet">
                <div><dt>Account</dt><dd><TailnetValue key={item.tailscale.tagged ? "Tagged device · no user account" : item.tailscale.account || "Unavailable"} value={item.tailscale.tagged ? "Tagged device · no user account" : item.tailscale.account || "Unavailable"} label="Account" /></dd></div>
                <div className="network-tailnet-identity"><dt>Tailnet</dt><dd><TailnetValue key={item.tailscale.tailnet || item.tailscale.dns_name || "Unavailable"} value={item.tailscale.tailnet || item.tailscale.dns_name || "Unavailable"} label="Tailnet" /></dd></div>
              </dl> : null}
              <div className="network-addresses">{addresses.length ? addresses.map(address => <code key={address}><SensitiveText value={address} label={item.tailscale || item.type === "tailscale" ? "Tailscale IP address" : "IP address"} sensitive={isSensitiveNetworkAddress(address, Boolean(item.tailscale) || item.type === "tailscale")} /></code>) : <span>No IPv4 address assigned</span>}</div>
            </div>
            <div className={`network-interface-control${item.local ? " is-locked" : ""}`}>
              {item.local ? <span className="network-locked-label">
                <svg aria-hidden="true" viewBox="0 0 24 24"><rect x="5" y="10" width="14" height="11" rx="2" /><path d="M8 10V7a4 4 0 0 1 8 0v3" /></svg>
                Always enabled
              </span> : null}
              <button
                aria-checked={item.enabled}
                aria-busy={!item.local && busy}
                aria-labelledby={labelID}
                aria-describedby={item.local ? descriptionID : undefined}
                className={`settings-model-toggle${item.enabled ? " is-enabled" : ""}`}
                disabled={busy || item.local}
                title={item.local ? "Local access is always enabled" : undefined}
                onClick={() => void load(item.name, !item.enabled)}
                role="switch"
                type="button"
              ><span /></button>
            </div>
            {item.tailscale?.magic_dns ? <div className="network-magic-dns-interface">
              <div className="settings-model-row-copy">
                <strong id={`network-${item.number}-magicdns-label`}>MagicDNS</strong>
                <code className="network-interface-name"><SensitiveText value={item.tailscale.magic_dns} label="MagicDNS name" /></code>
                {!item.enabled ? <small>Enable Tailscale to use MagicDNS</small> : null}
              </div>
              <button
                aria-checked={Boolean(item.tailscale.magic_dns_enabled)}
                aria-busy={busy}
                aria-labelledby={`network-${item.number}-magicdns-label`}
                className={`settings-model-toggle${item.tailscale.magic_dns_enabled ? " is-enabled" : ""}`}
                disabled={busy || !item.enabled}
                onClick={() => void load(`${item.name}:magicdns`, !item.tailscale?.magic_dns_enabled)}
                role="switch"
                type="button"
              ><span /></button>
            </div> : null}
          </div>;
        })}
      </div>
      <p className="network-note">Changes save automatically. Local access stays available. Turning off the interface you’re using may disconnect this page.</p>
      <details className="network-details">
        <summary>Connection details</summary>
        <p>This selection applies to server access and outgoing HTTP connections. Existing streams may stay open until they finish. System DNS and external programs use their own network settings.</p>
        <p>You can also manage these interfaces in the terminal with <code>/interface</code>.</p>
      </details>
    </div>
  </section>;
}

function TailnetValue({ value, label }: { value: string; label: string }) {
  return <SensitiveText value={value} label={`${label.toLowerCase()} email`} sensitive={/[^\s@]+@[^\s@]+/.test(value)} />;
}
