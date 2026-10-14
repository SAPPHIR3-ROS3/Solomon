import { AutoSaveSetting, SettingsLoadStatus } from "./AutoSaveSetting";
import { setResponseLanguage, setUserName, useGUIChatPreferencesState } from "./chatPreferences";
import { BackgroundSettings, backgroundSearchText } from "./BackgroundSettings";

export function GeneralSettings({ query }: { query: string }) {
  const preferences = useGUIChatPreferencesState();
  const fields = [
    { id: "user-name", label: "User name", description: "The name the model uses to address you.", saved: preferences.userName, save: setUserName },
    { id: "response-language", label: "Response language", description: "Choose the language the model uses for its responses.", saved: preferences.responseLanguage, save: setResponseLanguage, suggestions: ["English", "Italian", "French", "German", "Spanish", "Portuguese", "Japanese", "Chinese", "Korean", "Arabic", "Hindi", "Russian"] },
  ];
  const matches = fields.filter(field => `General ${field.label} ${field.description}`.toLowerCase().includes(query.trim().toLowerCase()));
  const showBackground = backgroundSearchText.toLowerCase().includes(query.trim().toLowerCase());
  if (query.trim() && !matches.length && !showBackground) return null;
  return <section aria-label="General settings" className="settings-models">
    <header className="settings-models-header"><h1>General</h1></header>
    <div className="settings-models-content">
      {matches.map(field => <AutoSaveSetting key={field.id} {...field} />)}
      {showBackground ? <BackgroundSettings /> : null}
      <SettingsLoadStatus section="general" />
    </div>
  </section>;
}
