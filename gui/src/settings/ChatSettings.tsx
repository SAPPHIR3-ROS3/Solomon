import { setFloatingLettersDensity, setFloatingLetters, setAutoCollapseToolCalls, setStartToolCallsCollapsed, setStartReasoningCollapsed, setComposerQuotaBars, useGUIChatPreferencesState, refreshChatPreferences } from "./chatPreferences";
import "./chat-settings.css";

export function ChatSettings({ query }: { query: string }) {
  const preferences = useGUIChatPreferencesState();
  const autoCollapse = preferences.chat.autoCloseToolCalls;
  const startCollapsed = preferences.chat.startToolCallsCollapsed;
  const settings = [
    {
      id: "start-reasoning-collapsed",
      label: "Start reasoning collapsed",
      description: "Show model reasoning collapsed by default. Click to expand it at any time.",
      enabled: preferences.chat.startReasoningCollapsed,
      setEnabled: setStartReasoningCollapsed,
    },
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
      description: "Collapse the whole tool-call sequence from the start. Each call still opens only its intent until you expand it.",
      enabled: startCollapsed,
      setEnabled: setStartToolCallsCollapsed,
    },
    {
      id: "composer-quota-bars",
      label: "Composer quota bars",
      description: "Show provider quota bars next to the composer. Turn this off to hide them.",
      enabled: preferences.chat.composerQuotaBars,
      setEnabled: setComposerQuotaBars,
    },
    {
      id: "floating-letters",
      label: "Floating letters",
      description: "Let letters drift through the chat and compose the model’s response token by token, even when it arrives in fast bursts. Choose a low, medium, high or max letter density. Respects reduced motion preferences.",
      enabled: preferences.chat.floatingLetters,
      setEnabled: setFloatingLetters,
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
              {id === "floating-letters" && enabled ? (
                <fieldset className="settings-letter-density" disabled={!preferences.ready || preferences.saving || !enabled}>
                  <legend>Letter density</legend>
                  {(["low", "medium", "high", "max"] as const).map(density => (
                    <label key={density}>
                      <input type="radio" name="floating-letters-density" value={density}
                        checked={preferences.chat.floatingLettersDensity === density}
                        onChange={() => void setFloatingLettersDensity(density)} />
                      <span>{density === "low" ? "Low" : density === "medium" ? "Medium" : density === "high" ? "High" : "Max"}</span>
                    </label>
                  ))}
                </fieldset>
              ) : null}
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
