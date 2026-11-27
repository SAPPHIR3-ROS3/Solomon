import { useEffect, useState } from "react";
import { serverEndpoint } from "../platform";
import { SensitiveText } from "../SensitiveText";
import { isSensitiveNetworkAddress } from "./networkAddressPrivacy";
import "./chat-settings.css";

type Link = {
  name: string;
  label: string;
  devices: { ip: string; mac?: string; hostname?: string; allowed?: boolean }[];
};

let cachedLinks: Link[] | undefined;
let pendingLinks: Promise<Link[]> | undefined;

async function fetchLinks(): Promise<Link[]> {
  const response = await fetch(await serverEndpoint("/__solomon/device-links"), {
    cache: "no-store",
  });
  if (!response.ok) {
    throw new Error(response.status === 403
      ? "View device links from the Solomon host."
      : "Could not load device links.");
  }
  if (!response.headers.get("Content-Type")?.toLowerCase().startsWith("application/json")) {
    throw new Error("Update the Solomon server to use device links.");
  }
  const data: unknown = await response.json().catch(() => null);
  if (!Array.isArray(data)) throw new Error("The Solomon server returned an invalid device list.");
  return data;
}

function loadLinks() {
  pendingLinks ??= fetchLinks().then((data) => {
    cachedLinks = data;
    return data;
  }).finally(() => {
    pendingLinks = undefined;
  });
  return pendingLinks;
}

export function prefetchDeviceLinks() {
  if (cachedLinks === undefined) void loadLinks().catch(() => {});
}

export function DeviceLinks() {
  const [links, setLinks] = useState<Link[]>(() => cachedLinks ?? []);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(() => cachedLinks === undefined || pendingLinks !== undefined);
  const [revision, setRevision] = useState(0);
  const [saving, setSaving] = useState(false);
  const visibleLinks = links.map((link) => ({
    ...link,
    devices: link.devices
      .filter((device) => !device.ip.includes(":"))
      .sort((a, b) => Number(Boolean(b.allowed)) - Number(Boolean(a.allowed))),
  })).filter((link) => link.devices.length > 0);

  useEffect(() => {
    if (revision === 0 && cachedLinks && !pendingLinks) {
      setLinks(cachedLinks);
      setLoading(false);
      return;
    }
    let active = true;
    setLoading(true);
    setError("");
    void loadLinks().then((data) => {
      if (active) setLinks(data);
    }).catch((failure: unknown) => {
      if (active) setError(failure instanceof Error ? failure.message : "Could not load device links.");
    }).finally(() => {
      if (active) setLoading(false);
    });
    return () => { active = false; };
  }, [revision]);

  async function setDeviceAccess(name: string, ip: string, allowed: boolean) {
    setSaving(true);
    setError("");
    try {
      const response = await fetch(await serverEndpoint("/__solomon/device-links"), {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ interface: name, ip, allowed }),
      });
      if (response.status !== 204) throw new Error("Could not update device access.");
      cachedLinks = (cachedLinks ?? links).map((link) => link.name === name ? {
        ...link,
        devices: link.devices.map((device) => device.ip === ip ? { ...device, allowed } : device),
      } : link);
      setLinks(cachedLinks);
    } catch {
      setError("Could not update device access.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <section aria-labelledby="device-links-heading" className="settings-device-links">
      <div className="settings-models-panel-heading">
        <div>
          <h2 id="device-links-heading">Links</h2>
          <p>Devices detected on enabled network interfaces. Entries may be offline or lack access to Solomon.</p>
        </div>
        <button className="settings-chat-retry" disabled={loading || saving} onClick={() => setRevision((value) => value + 1)} type="button">
          Refresh
        </button>
      </div>
      {loading ? <p className="settings-models-empty" role="status">Loading links…</p> : null}
      {error ? <p className="settings-models-empty" role="alert">{error}</p> : null}
      {!loading && !error && visibleLinks.length === 0 ? <p className="settings-models-empty">No devices detected.</p> : null}
      {visibleLinks.map((link) => (
        <section aria-label={link.name} className="settings-device-link" key={link.name}>
          <h3>{link.label}{link.label !== link.name ? <small>{link.name}</small> : null}</h3>
          <table>
            <thead><tr><th scope="col">Hostname</th><th scope="col">IP address</th><th scope="col">MAC address</th><th scope="col" className="settings-device-access">Access</th></tr></thead>
            <tbody>
              {link.devices.map((device) => (
                <tr key={device.ip}>
                  <td>{device.hostname?.replace(/\.local\.?$/i, "") || "Unavailable"}</td>
                  <td><SensitiveText label="IPv4 address" sensitive={isSensitiveNetworkAddress(device.ip)} value={device.ip} /></td>
                  <td>{device.mac ? <SensitiveText label="MAC address" value={device.mac} /> : "Unavailable"}</td>
                  <td className="settings-device-access">
                    <button
                      aria-checked={Boolean(device.allowed)}
                      aria-label={`Allow access for ${device.hostname || device.ip}`}
                      className={`settings-model-toggle${device.allowed ? " is-enabled" : ""}`}
                      disabled={loading || saving}
                      onClick={() => void setDeviceAccess(link.name, device.ip, !device.allowed)}
                      role="switch"
                      type="button"
                    ><span /></button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
      ))}
    </section>
  );
}
