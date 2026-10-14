import { useState } from "react";
import { AutoSaveSetting, SettingsLoadStatus } from "./AutoSaveSetting";
import { setAdvancedSetting, useGUIChatPreferencesState } from "./chatPreferences";

const fields = [
  { key: "server_port", label: "Server port", description: "TCP port used at the next server startup.", min: 1, max: 65535 },
  { key: "server.dev_directory", label: "GUI source directory", description: "Absolute path to the GUI project containing package.json and src. Used by the dev server at its next startup.", allowEmpty: true },
  { key: "compaction_threshold_tokens", label: "Compaction threshold", description: "Conversation tokens before automatic context compaction.", min: 32768, max: 1048576 },
  { key: "subagent_timeout_minutes", label: "Subagent timeout", description: "Maximum duration of a subagent run, in minutes.", min: 1, max: 180 },
  { key: "log_level", label: "Log level", description: "Logging verbosity. Takes effect at the next application startup.", options: ["error", "warning", "info", "debug", "result"] },
  { key: "research_max_rounds", label: "Research rounds", description: "Maximum number of rounds in deep research.", min: 1, max: 100 },
  { key: "research_max_urls_per_round", label: "Research URLs per round", description: "Maximum number of URLs explored in each research round.", min: 1, max: 100 },
  { key: "research_max_content_chars", label: "Research content limit", description: "Maximum characters of content collected per research page.", min: 1, max: 1000000 },
  { key: "tool_output.max_bytes", label: "Tool output byte limit", description: "Maximum bytes of a tool result sent to the model.", min: 1, max: 1048576 },
  { key: "tool_output.max_lines", label: "Tool output line limit", description: "Maximum lines of a tool result sent to the model.", min: 1, max: 1048576 },
].map(field => ({ ...field, save: (value: string) => setAdvancedSetting(field.key, ["log_level", "server.dev_directory"].includes(field.key) ? value : Number(value)) }));

export function AdvancedSettings({ query }: { query: string }) {
  const preferences = useGUIChatPreferencesState();
  const matches = fields.filter(field => `Advanced ${field.label} ${field.description} ${field.key}`.toLowerCase().includes(query.trim().toLowerCase()));
  const matchesDev = `Advanced Dev server Enable Vite development mode at the next server startup. server.dev_mode`.toLowerCase().includes(query.trim().toLowerCase());
  if (query.trim() && !matches.length && !matchesDev) return null;
  return <section aria-label="Advanced settings" className="settings-models">
    <header className="settings-models-header"><h1>Advanced</h1></header>
    <div className="settings-models-content">
      {matchesDev ? <DevServerSetting /> : null}
      {matches.map(({ key, ...field }) => <AutoSaveSetting key={key} id={`advanced-${key.replaceAll(".", "-")}`} {...field} saved={String(preferences.advanced[key] ?? "")} />)}
      <SettingsLoadStatus section="advanced" />
    </div>
  </section>;
}

function DevServerSetting() {
  const preferences = useGUIChatPreferencesState();
  const [saving, setSaving] = useState(false);
  const enabled = preferences.advanced["server.dev_mode"] === true;
  const hasDirectory = typeof preferences.advanced["server.dev_directory"] === "string" && preferences.advanced["server.dev_directory"].trim().length > 0;
  return <div className="settings-model-row settings-language-row">
    <div className="settings-model-row-copy">
      <strong id="dev-server-label">Dev server</strong>
      <small id="dev-server-description">Enable Vite development mode at the next server startup. Configure a GUI source directory first.</small>
    </div>
    <div className="settings-language-controls">
      {saving ? <small className="settings-save-status" aria-live="polite">Saving…</small> : null}
      <button
        aria-checked={enabled}
        aria-describedby="dev-server-description"
        aria-labelledby="dev-server-label"
        className={`settings-model-toggle${enabled ? " is-enabled" : ""}`}
        disabled={!preferences.ready || preferences.saving || !hasDirectory}
        onClick={() => {
          setSaving(true);
          void setAdvancedSetting("server.dev_mode", !enabled).catch(() => {}).finally(() => setSaving(false));
        }}
        role="switch"
        type="button"
      ><span /></button>
    </div>
  </div>;
}
