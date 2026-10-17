import { useEffect, useRef, useState } from "react";
import { refreshChatPreferences, useGUIChatPreferencesState } from "./chatPreferences";
import { Select } from "../Select";
import { SettingsInput } from "./SettingsInput";
import "./chat-settings.css";

type Props = {
  id: string;
  label: string;
  description: string;
  saved: string;
  save: (value: string) => Promise<void>;
  min?: number;
  max?: number;
  options?: string[];
  suggestions?: string[];
  allowEmpty?: boolean;
};

export function AutoSaveSetting({ id, label, description, saved, save, min, max, options, suggestions, allowEmpty = false }: Props) {
  const preferences = useGUIChatPreferencesState();
  const [value, setValue] = useState(saved);
  const [failed, setFailed] = useState(false);
  const [saving, setSaving] = useState(false);
  const [retry, setRetry] = useState(0);
  const previous = useRef(saved);
  const attempted = useRef<string | undefined>(undefined);
  const pending = useRef<(() => void) | undefined>(undefined);
  const numeric = min !== undefined;
  const valid = numeric
    ? value.trim() !== "" && Number.isSafeInteger(Number(value)) && Number(value) >= min && Number(value) <= (max ?? Infinity)
    : (allowEmpty || value.trim() !== "") && new TextEncoder().encode(value.trim()).length <= 256 && !/[\r\n\0]/.test(value);
  const canonical = numeric && valid ? String(Number(value)) : value.trim();
  const changed = canonical !== saved;

  useEffect(() => {
    const previousSaved = previous.current;
    setValue(current => current === previousSaved || current.trim() === saved || current.trim() === attempted.current ? saved : current);
    previous.current = saved;
  }, [saved]);

  useEffect(() => {
    pending.current = undefined;
    if (!preferences.ready || !valid || !changed || attempted.current === canonical) return;
    const persist = () => {
      if (attempted.current === canonical) return;
      attempted.current = canonical;
      pending.current = undefined;
      setSaving(true);
      setFailed(false);
      void save(canonical).catch(() => setFailed(true)).finally(() => setSaving(false));
    };
    pending.current = persist;
    // Other fields share a write queue. Keep editing available during a save.
    if (preferences.saving) return;
    const timer = window.setTimeout(persist, 600);
    return () => window.clearTimeout(timer);
  }, [canonical, changed, preferences.ready, preferences.saving, retry, save, valid, value]);

  useEffect(() => () => pending.current?.(), []);

  const changeValue = (next: string) => {
    attempted.current = undefined;
    setFailed(false);
    setValue(next);
  };

  return (
    <div className="settings-model-row settings-language-row">
      <div className="settings-model-row-copy">
        <label htmlFor={id}><strong>{label}</strong></label>
        <small id={`${id}-description`}>{description}</small>
      </div>
      <div className="settings-language-controls">
        {options ? <div className="settings-input">
          <Select id={id} label={label} describedBy={`${id}-description`} invalid={changed && !valid} disabled={!preferences.ready} value={value} options={options.map(option => ({ value: option, label: option }))} onChange={changeValue} onBlur={() => pending.current?.()} />
        </div> : <SettingsInput id={id} label={label} value={value} disabled={!preferences.ready} invalid={changed && !valid} failed={failed} min={min} max={max} suggestions={suggestions} onChange={changeValue} onBlur={() => pending.current?.()} />}
        <small aria-live="polite" className="settings-save-status">
          {saving ? "Saving…" : failed ? "Not saved" : changed ? valid ? "Pending…" : numeric ? `Enter an integer from ${min} to ${max}.` : "Enter a nonempty value (max 256 bytes)." : ""}
        </small>
        {failed ? <button className="settings-chat-retry" onClick={() => { attempted.current = undefined; setRetry(count => count + 1); }} type="button">Retry</button> : null}
      </div>
    </div>
  );
}

export function SettingsLoadStatus({ section }: { section: string }) {
  const preferences = useGUIChatPreferencesState();
  return <>
    {!preferences.ready && !preferences.error ? <p className="settings-models-empty">Loading {section} settings…</p> : null}
    {preferences.error ? <div className="settings-chat-error" role="alert">
      <p>{preferences.error}</p>
      {!preferences.ready ? <button className="settings-chat-retry" disabled={preferences.saving} onClick={() => void refreshChatPreferences()} type="button">Retry</button> : null}
    </div> : null}
  </>;
}
