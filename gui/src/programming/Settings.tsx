import { useEffect, useState } from "react";
import { Select } from "../Select";
import { languageRequest, type LanguageSettings } from "./languages";
import "./settings.css";

type Draft = { mode: LanguageSettings["mode"]; command: string; settings: string; initializationOptions: string; completion: boolean; hover: boolean; diagnostics: boolean };
function draftFor(language: LanguageSettings): Draft {
  return { mode: language.mode || "auto", command: JSON.stringify(language.command ?? language.server.command ?? [], null, 2),
    settings: JSON.stringify(language.settings ?? {}, null, 2), initializationOptions: JSON.stringify(language.initializationOptions ?? {}, null, 2),
    completion: language.completion !== false, hover: language.hover !== false, diagnostics: language.diagnostics !== false };
}

export function ProgrammingLanguages({ query }: { query: string }) {
  const [languages, setLanguages] = useState<LanguageSettings[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [selected, setSelected] = useState<string | null>(null);
  async function load() {
    setLoading(true);
    try { setLanguages(await languageRequest()); setError(""); }
    catch (error) { setError(String(error)); }
    finally { setLoading(false); }
  }
  useEffect(() => {
    void load();
    const refresh = () => { void load(); };
    window.addEventListener("solomon:languages-used", refresh);
    window.addEventListener("focus", refresh);
    return () => { window.removeEventListener("solomon:languages-used", refresh); window.removeEventListener("focus", refresh); };
  }, []);
  const search = query.trim().toLowerCase();
  const all = "programming languages language server lsp fallback syntax highlighting".includes(search);
  const visible = all ? languages : languages.filter(language => `${language.name} ${language.id}`.toLowerCase().includes(search));
  if (!all && !visible.length) return null;
  return <section aria-label="Programming Languages" className="settings-models">
    <header className="settings-models-header"><h1>Programming Languages</h1></header>
    <div className="settings-models-content">
      <div className="settings-models-panel-heading"><div><h2>Used languages</h2><p>Languages appear as you open files or view code in Solomon. Installed language servers are detected automatically.</p></div><button type="button" className="settings-chat-retry" disabled={loading} onClick={() => void load()}>Refresh</button></div>
      {error ? <p role="alert" className="settings-chat-error">{error}</p> : null}
      {loading && !languages.length ? <p role="status">Loading languages…</p> : null}
      {!loading && !languages.length && !error ? <p className="settings-models-empty">No programming languages used yet. Open a source file to get started.</p> : null}
      <div className="programming-languages">
        {visible.map(language => <div key={language.id} className="programming-language">
          <button type="button" className="programming-language-heading" aria-expanded={selected === language.id} onClick={() => setSelected(selected === language.id ? null : language.id)}>
            <span><strong>{language.name}</strong><small>{language.id}</small></span>
            <span className={language.server.available ? "programming-server-available" : "programming-server-fallback"}>{language.server.available ? "Language server available" : "Internal fallback"}</span>
          </button>
          {selected === language.id ? <LanguagePreferences key={`${language.id}:${language.lastUsed}`} language={language} onSaved={rows => { setLanguages(rows); window.dispatchEvent(new Event("solomon:language-settings")); }} /> : null}
        </div>)}
      </div>
      <p className="programming-note">Internal syntax highlighting stays enabled with or without an LSP. No language servers are downloaded or installed automatically. Servers run on the machine hosting Solomon.</p>
    </div>
  </section>;
}

function LanguagePreferences({ language, onSaved }: { language: LanguageSettings; onSaved: (rows: LanguageSettings[]) => void }) {
  const [draft, setDraft] = useState(() => draftFor(language));
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const set = <K extends keyof Draft>(key: K, value: Draft[K]) => setDraft(current => ({ ...current, [key]: value }));
  const save = async () => {
    setError(""); setBusy(true);
    try {
      const command: unknown = JSON.parse(draft.command);
      if (!Array.isArray(command) || command.some(argument => typeof argument !== "string")) throw new Error("Command must be a JSON array of strings.");
      const settings = JSON.parse(draft.settings), initializationOptions = JSON.parse(draft.initializationOptions);
      for (const value of [settings, initializationOptions]) if (!value || typeof value !== "object" || Array.isArray(value)) throw new Error("Server settings and initialization options must be JSON objects.");
      onSaved(await languageRequest("PATCH", { ...language, ...draft, command, settings, initializationOptions }));
    } catch (error) { setError(String(error)); }
    finally { setBusy(false); }
  };
  return <div className="programming-language-preferences">
    <p>{language.server.available ? `Detected command: ${JSON.stringify(language.server.command)}` : language.server.reason}</p>
    <div className="programming-language-assistance"><label htmlFor={`language-assistance-${language.id}`}>Language assistance</label><Select id={`language-assistance-${language.id}`} label="Language assistance" value={draft.mode} onChange={value => set("mode", value as Draft["mode"])} options={[
      { value: "auto", label: "Automatic · use an installed server" },
      { value: "custom", label: "Custom language server" },
      { value: "fallback", label: "Internal fallback only" },
    ]} /></div>
    {draft.mode === "custom" ? <label>Command and arguments (JSON array)<textarea aria-label={`${language.name} server command`} value={draft.command} spellCheck={false} onChange={event => set("command", event.target.value)} /></label> : null}
    {draft.mode !== "fallback" ? <>
      <fieldset><legend>LSP features</legend>{(["completion", "hover", "diagnostics"] as const).map(key => <label key={key}><input type="checkbox" checked={draft[key]} onChange={event => set(key, event.target.checked)} />{key === "completion" ? "Code completion" : key === "hover" ? "Hover information" : "Diagnostics"}</label>)}</fieldset>
      <label>Server settings (JSON)<textarea aria-label={`${language.name} server settings`} value={draft.settings} spellCheck={false} onChange={event => set("settings", event.target.value)} /></label>
      <label>Initialization options (JSON)<textarea aria-label={`${language.name} initialization options`} value={draft.initializationOptions} spellCheck={false} onChange={event => set("initializationOptions", event.target.value)} /></label>
    </> : null}
    {error ? <p role="alert" className="settings-chat-error">{error}</p> : null}
    <button type="button" className="settings-chat-retry" disabled={busy} onClick={() => void save()}>{busy ? "Saving…" : "Save language settings"}</button>
  </div>;
}
