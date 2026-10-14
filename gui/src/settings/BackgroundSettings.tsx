import { useEffect, useState } from "react";
import { initialClient, serverEndpoint } from "../platform";

export const backgroundSearchText = "General Enable daemon background start keep awake sleep laptop lid";

export function BackgroundSettings() {
  const native = initialClient().surface === "desktop";
  const [running, setRunning] = useState(false);
  const [awake, setAwake] = useState(false);
  const [ready, setReady] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    let disposed = false;
    const controller = new AbortController();
    async function refresh() {
      try {
        const bridge = window.go?.main?.ServerBridge;
        const online = native && bridge?.DaemonRunning
          ? await bridge.DaemonRunning()
          : (await fetch(await serverEndpoint("/health"), { signal: controller.signal })).ok;
        if (disposed) return;
        setRunning(online);
        if (online) {
          const response = await fetch(await serverEndpoint("/__solomon/background"), { signal: controller.signal });
          if (!response.ok) throw new Error("Background settings are unavailable. Update the daemon to use Keep awake.");
          const settings = await response.json() as { keepAwake: boolean; error?: string };
          if (!disposed) { setAwake(settings.keepAwake); setError(settings.error || ""); }
        } else { setAwake(false); }
      } catch (reason) {
        if (!disposed) setError(String(reason instanceof Error ? reason.message : reason));
      } finally { if (!disposed) setReady(true); }
    }
    void refresh();
    return () => { disposed = true; controller.abort(); };
  }, [native]);

  async function changeDaemon() {
    const bridge = window.go?.main?.ServerBridge;
    if (!bridge?.SetDaemonEnabled) { setError("Manual daemon control requires the updated desktop app."); return; }
    if (running && !window.confirm("Stop the daemon? This interrupts running agents and terminals.")) return;
    setBusy(true); setError("");
    try {
      const online = await bridge.SetDaemonEnabled(!running);
      setRunning(online);
      if (online) window.location.reload();
      else setAwake(false);
    } catch (reason) {
      setError(String(reason instanceof Error ? reason.message : reason));
      if (bridge.DaemonRunning) setRunning(await bridge.DaemonRunning());
    } finally { setBusy(false); }
  }

  async function changeAwake() {
    setBusy(true); setError("");
    try {
      const response = await fetch(await serverEndpoint("/__solomon/background"), {
        method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ keepAwake: !awake }),
      });
      const settings = await response.json() as { keepAwake: boolean; error?: string };
      if (!response.ok) throw new Error(settings.error || "Could not change Keep awake.");
      setAwake(settings.keepAwake);
    } catch (reason) { setError(String(reason instanceof Error ? reason.message : reason)); }
    finally { setBusy(false); }
  }

  return <>
    <div className="settings-model-row">
      <div className="settings-model-row-copy">
        <strong id="daemon-label">Enable daemon</strong>
        <small id="daemon-description">{native ? "Start Solomon manually if automatic startup fails. Work continues after closing the app. Turning this off interrupts running work." : "The daemon keeps work running in the background. Use the desktop app to start it manually."}</small>
      </div>
      <button aria-checked={running} aria-labelledby="daemon-label" aria-describedby="daemon-description" className={`settings-model-toggle${running ? " is-enabled" : ""}`} disabled={!native || !ready || busy} onClick={() => void changeDaemon()} role="switch" type="button"><span /></button>
    </div>
    <div className="settings-model-row">
      <div className="settings-model-row-copy">
        <strong id="awake-label">Keep awake</strong>
        <small id="awake-description">Prevent sleep while the daemon runs, even after closing the app. On Linux, also request that closing the lid does not suspend the laptop. Lid behavior depends on system power policies; on macOS, closed-lid operation requires a supported power/display setup.</small>
      </div>
      <button aria-checked={awake} aria-labelledby="awake-label" aria-describedby="awake-description" className={`settings-model-toggle${awake ? " is-enabled" : ""}`} disabled={!running || !ready || busy} onClick={() => void changeAwake()} role="switch" type="button"><span /></button>
    </div>
    {!ready ? <p className="settings-models-empty">Checking daemon…</p> : null}
    {error ? <div className="settings-chat-error" role="alert"><p>{error}</p></div> : null}
  </>;
}
