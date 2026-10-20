import { useEffect, useState } from "react";
import { AutoSaveSetting, SettingsLoadStatus } from "./AutoSaveSetting";
import { refreshChatPreferences, setAdvancedSetting, useGUIChatPreferencesState } from "./chatPreferences";
import { ReportButton } from "../notifications/Notifications";
import { NewProjectDialog } from "../projects/NewProjectDialog";
import { serverEndpoint } from "../platform";
import "./development-settings.css";

const fields = [
  { key: "server_port", label: "Server port", description: "TCP port used at the next server startup.", min: 1, max: 65535 },
  { key: "compaction_threshold_tokens", label: "Compaction threshold", description: "Conversation tokens before automatic context compaction.", min: 32768, max: 1048576 },
  { key: "subagent_timeout_minutes", label: "Subagent timeout", description: "Maximum duration of a subagent run, in minutes.", min: 1, max: 180 },
  { key: "log_level", label: "Log level", description: "Logging verbosity. Takes effect at the next application startup.", options: ["error", "warning", "info", "debug", "result"] },
  { key: "install_layout", label: "Install layout", description: "split keeps the CLI server and the desktop client separate. standalone is one binary. Updates download this layout, and changing it switches the installation.", options: ["split", "standalone"] },
  { key: "research_max_rounds", label: "Research rounds", description: "Maximum number of rounds in deep research.", min: 1, max: 100 },
  { key: "research_max_urls_per_round", label: "Research URLs per round", description: "Maximum number of URLs explored in each research round.", min: 1, max: 100 },
  { key: "research_max_content_chars", label: "Research content limit", description: "Maximum characters of content collected per research page.", min: 1, max: 1000000 },
  { key: "tool_output.max_bytes", label: "Tool output byte limit", description: "Maximum bytes of a tool result sent to the model.", min: 1, max: 1048576 },
  { key: "tool_output.max_lines", label: "Tool output line limit", description: "Maximum lines of a tool result sent to the model.", min: 1, max: 1048576 },
].map(field => ({ ...field, save: (value: string) => setAdvancedSetting(field.key, ["log_level", "server.dev_directory", "install_layout"].includes(field.key) ? value : Number(value)) }));

export function AdvancedSettings({ query }: { query: string }) {
  const preferences = useGUIChatPreferencesState();
  const matches = fields.filter(field => `Advanced ${field.label} ${field.description} ${field.key}`.toLowerCase().includes(query.trim().toLowerCase()));
  const matchesDev = `Advanced Dev server Enable Vite development mode. server.dev_mode`.toLowerCase().includes(query.trim().toLowerCase());
  const matchesDirectory = `Advanced GUI source directory folder server.dev_directory`.toLowerCase().includes(query.trim().toLowerCase());
  const matchesReport = `Advanced Send report diagnostic logs note`.toLowerCase().includes(query.trim().toLowerCase());
  if (query.trim() && !matches.length && !matchesDev && !matchesDirectory && !matchesReport) return null;
  return <section aria-label="Advanced settings" className="settings-models">
    <header className="settings-models-header"><h1>Advanced</h1></header>
    <div className="settings-models-content">
      {matchesDev || matchesDirectory ? <div className="settings-dev-panel">
        {matchesDev ? <DevServerSetting /> : null}
        {matchesDirectory ? <GUIDirectorySetting /> : null}
      </div> : null}
      {matchesReport ? <ReportButton placement="settings" /> : null}
      {matches.map(({ key, ...field }) => <AutoSaveSetting key={key} id={`advanced-${key.replaceAll(".", "-")}`} {...field} saved={String(preferences.advanced[key] ?? "")} />)}
      <SettingsLoadStatus section="advanced" />
    </div>
  </section>;
}

function DevServerSetting() {
  const preferences = useGUIChatPreferencesState();
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const enabled = preferences.advanced["server.dev_mode"] === true;
  const busy = (!preferences.ready && !preferences.error) || preferences.saving || saving;
  return <div className="settings-model-row settings-dev-header">
    <div className="settings-model-row-copy">
      <div className="settings-dev-heading"><strong id="dev-server-label">Dev server</strong><span className={`settings-dev-state${enabled ? " is-live" : ""}`}>{enabled ? "Live editing" : "Bundled interface"}</span></div>
      <small id="dev-server-description">Customize your interface while chats and terminals keep running.</small>
      {error ? <small className="settings-dev-error" role="alert">{error}</small> : null}
    </div>
    <div className="settings-dev-toggle">
      {saving ? <small className="settings-save-status" aria-live="polite">Preparing interface…</small> : null}
      <button
        aria-busy={busy}
        aria-checked={enabled}
        aria-describedby="dev-server-description"
        aria-labelledby="dev-server-label"
        className={`settings-model-toggle${enabled ? " is-enabled" : ""}`}
        disabled={busy || !preferences.ready}
        onClick={() => {
          setSaving(true);
          setError("");
          void developmentRequest(enabled ? "normal" : "dev", String(preferences.advanced["server.dev_directory"] ?? ""))
            .then(() => refreshChatPreferences()).catch(reason => setError(reason instanceof Error ? reason.message : "Unable to switch interface")).finally(() => setSaving(false));
        }}
        role="switch"
        type="button"
      ><span /></button>
    </div>
  </div>;
}

async function developmentRequest(mode: string, directory: string): Promise<{ directory?: string }> {
  const response = await fetch(await serverEndpoint("/__solomon/development"), {
    method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ mode, directory }),
  });
  const payload = await response.json();
  if (!response.ok) throw new Error(payload.error || "Unable to prepare GUI sources");
  return payload;
}

function GUIDirectorySetting() {
  const preferences = useGUIChatPreferencesState();
  const saved = String(preferences.advanced["server.dev_directory"] ?? "");
  const [directory, setDirectory] = useState(saved);
  const [pickerOpen, setPickerOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  useEffect(() => setDirectory(saved), [saved]);
  async function selectDirectory(path: string) {
    setSaving(true);
    setError("");
    try {
      const result = await developmentRequest("prepare", path);
      if (!result.directory) throw new Error("GUI source directory is missing");
      if (preferences.advanced["server.dev_mode"] === true) {
        await developmentRequest("dev", result.directory);
        await refreshChatPreferences();
      } else {
        await setAdvancedSetting("server.dev_directory", result.directory);
      }
      setDirectory(result.directory);
      setPickerOpen(false);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Unable to prepare GUI sources");
      throw reason;
    } finally { setSaving(false); }
  }
  return <>
    <div className="settings-dev-directory">
      <div className="settings-dev-directory-label">
        <label htmlFor="gui-source-directory"><strong>GUI source directory</strong></label>
      </div>
      <div className="settings-dev-folder-controls">
        <div className="settings-dev-path">
          <FolderIcon />
          <input id="gui-source-directory" aria-label="GUI source directory" aria-describedby="gui-source-directory-help" disabled={saving || !preferences.ready} value={directory} onChange={event => setDirectory(event.target.value)} placeholder="Choose a folder for your interface" />
          <button className="settings-dev-browse" disabled={saving || !preferences.ready} onClick={() => setPickerOpen(true)} type="button">Browse…</button>
        </div>
        <button className="settings-dev-apply" disabled={saving || !directory.trim() || !preferences.ready} onClick={() => void selectDirectory(directory).catch(() => {})} type="button">{saving ? "Preparing…" : "Use folder"}</button>
      </div>
      <p id="gui-source-directory-help" className="settings-dev-hint">Empty folders are set up automatically. Existing GUI sources are preserved.</p>
      {error ? <p className="settings-dev-error" role="alert">{error}</p> : null}
    </div>
    <NewProjectDialog folderOnly initialPath={saved || "~"} title="Select GUI source folder" isOpen={pickerOpen} onClose={() => setPickerOpen(false)} onConfirmLocalFolder={selection => selectDirectory(selection.path)} />
  </>;
}

function FolderIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2Z" /></svg>;
}
