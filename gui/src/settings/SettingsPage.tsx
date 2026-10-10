import { useEffect, useState } from "react";
import { prefetchProviderQuotas } from "../projects/projects";
import { DocsViewer } from "./DocsViewer";
import { ModelsPage } from "./ModelsPage";
import { ChatSettings } from "./ChatSettings";

type SettingsPageProps = {
  onHome: () => void;
};

type SettingsSection = "" | "chat" | "models" | "docs";

const locationEvent = "solomon-settings-location";

function normalizedPathname(pathname = window.location.pathname) {
  return pathname.replace(/\/+$/, "") || "/";
}

export function isSettingsPath(pathname = window.location.pathname) {
  return normalizedPathname(pathname) === "/settings";
}

export function subscribeSettingsLocation(onChange: () => void) {
  window.addEventListener("popstate", onChange);
  window.addEventListener(locationEvent, onChange);
  return () => {
    window.removeEventListener("popstate", onChange);
    window.removeEventListener(locationEvent, onChange);
  };
}

function publishLocation() {
  window.dispatchEvent(new Event(locationEvent));
}

export function showSettings() {
  if (!isSettingsPath()) window.history.pushState(null, "", "/settings");
  publishLocation();
}

export function hideSettings() {
  if (!isSettingsPath()) return;
  window.history.pushState(null, "", "/");
  publishLocation();
}

export function SettingsPage({ onHome }: SettingsPageProps) {
  const [query, setQuery] = useState("");
  const [section, setSection] = useState<SettingsSection>("");
  const isSearching = query.trim().length > 0;

  useEffect(() => {
    prefetchProviderQuotas();
  }, []);

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

        <nav aria-label="Settings sections" className="settings-navigation">
          <button
            aria-current={section === "chat" ? "page" : undefined}
            className="settings-section-link"
            onClick={() => setSection("chat")}
            type="button"
          >
            <ChatIcon />
            <span>Chat</span>
          </button>
          <button
            aria-current={section === "models" ? "page" : undefined}
            className="settings-section-link"
            onClick={() => setSection("models")}
            type="button"
          >
            <ModelsIcon />
            <span>Models</span>
          </button>
          <button
            aria-current={section === "docs" ? "page" : undefined}
            className={`settings-docs-link${section === "docs" ? " is-active" : ""}`}
            onClick={() => setSection("docs")}
            type="button"
          >
            <DocsIcon />
            <span>Docs</span>
          </button>
        </nav>

        <div className="settings-sidebar-footer">
          <button aria-label="Back to home" className="settings-back" onClick={onHome} type="button">
            <BackIcon />
            <span>Back</span>
          </button>
        </div>
      </aside>

      <main aria-label="Settings content" className="settings-main">
        {isSearching || section === "chat" ? <ChatSettings query={query} /> : null}
        {!isSearching && section === "models" ? <ModelsPage /> : null}
        {!isSearching && section === "docs" ? <DocsViewer /> : null}
      </main>
    </section>
  );
}

function ChatIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="M7 4h10a4 4 0 0 1 4 4v6a4 4 0 0 1-4 4H9l-5 3v-4a4 4 0 0 1-1-3V8a4 4 0 0 1 4-4Z" />
      <path d="M7 9h10M7 13h6" />
    </svg>
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

function DocsIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20" />
      <path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2Z" />
    </svg>
  );
}

function ModelsIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="M4 6h16M4 12h16M4 18h16" />
      <circle cx="9" cy="6" r="2" />
      <circle cx="15" cy="12" r="2" />
      <circle cx="11" cy="18" r="2" />
    </svg>
  );
}
