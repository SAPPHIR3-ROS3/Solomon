import { useState } from "react";

type SettingsPageProps = {
  onHome: () => void;
};

export function SettingsPage({ onHome }: SettingsPageProps) {
  const [query, setQuery] = useState("");

  return (
    <section aria-label="Settings" className="settings-page">
      <aside aria-label="Settings navigation" className="settings-sidebar">
        <button aria-label="Go to home" className="side-panel-wordmark settings-wordmark" onClick={onHome} type="button">
          SOLOMON
        </button>
        <div aria-hidden="true" className="settings-sidebar-head" />

        <label className="settings-search">
          <SearchIcon />
          <input
            aria-label="Search settings"
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search Settings"
            type="search"
            value={query}
          />
        </label>

        <div className="settings-sidebar-footer">
          <button aria-label="Back to home" className="settings-back" onClick={onHome} type="button">
            <BackIcon />
            <span>Back</span>
          </button>
        </div>
      </aside>

      <main aria-label="Settings content" className="settings-main" />
    </section>
  );
}

function BackIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="M19 12H5M11 6l-6 6 6 6" />
    </svg>
  );
}

function SearchIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <circle cx="11" cy="11" r="6.5" />
      <path d="m16 16 4 4" />
    </svg>
  );
}
