import { setAutoCollapseToolCalls, setStartToolCallsCollapsed, useGUIChatPreferencesState, refreshChatPreferences } from "./chatPreferences";
import "./chat-settings.css";

export function ChatSettings({ query }: { query: string }) {
  const preferences = useGUIChatPreferencesState();
  const autoCollapse = preferences.chat.autoCloseToolCalls;
  const startCollapsed = preferences.chat.startToolCallsCollapsed;
  const settings = [
    {
      id: "auto-close-tool-calls",
      label: "Auto-close tool calls",
      description: "Collapse tool activity when the model finishes its response. You can reopen it at any time.",
      enabled: autoCollapse,
      setEnabled: setAutoCollapseToolCalls,
    },
    {
      id: "start-tool-calls-collapsed",
      label: "Start tool calls collapsed",
      description: "Show new tool calls closed from the start. Turn this off to display them open while the model works.",
      enabled: startCollapsed,
      setEnabled: setStartToolCallsCollapsed,
    },
  ];
  const matches = settings.filter(({ label, description }) => `${label} ${description}`.toLowerCase().includes(query.trim().toLowerCase()));

  return (
    <section aria-label="Chat settings" className="settings-models">
      <header className="settings-models-header"><h1>Chat</h1></header>
      <div className="settings-models-content">
        {matches.map(({ id, label, description, enabled, setEnabled }) => (
          <div className="settings-model-row" key={id}>
            <div className="settings-model-row-copy">
              <strong id={`${id}-label`}>{label}</strong>
              <small id={`${id}-description`}>{description}</small>
            </div>
            <button
              aria-checked={enabled}
              aria-describedby={`${id}-description`}
              aria-labelledby={`${id}-label`}
              className={`settings-model-toggle${enabled ? " is-enabled" : ""}`}
              disabled={!preferences.ready || preferences.saving}
              onClick={() => void setEnabled(!enabled)}
              role="switch"
              type="button"
            ><span /></button>
          </div>
        ))}
        {!preferences.ready && !preferences.error ? <p className="settings-models-empty">Loading chat settings…</p> : null}
        {preferences.error ? (
          <div className="settings-chat-error" role="alert">
            <p>{preferences.error}</p>
            <button className="settings-chat-retry" disabled={preferences.saving} onClick={() => void refreshChatPreferences()} type="button">Retry</button>
          </div>
        ) : null}
        {!matches.length ? <p className="settings-models-empty">No chat settings match this search.</p> : null}
      </div>
    </section>
  );
}
