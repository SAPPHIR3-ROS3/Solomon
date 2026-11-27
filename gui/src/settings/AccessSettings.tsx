import { useState } from "react";
import { DeviceLinks } from "./DeviceLinks";
import "./access.css";

const pages = ["Devices", "Environment"] as const;
type AccessPage = typeof pages[number];

export function accessSettingsMatch(query: string) {
  const term = query.trim().toLowerCase();
  return pages.find((page) => page.toLowerCase().includes(term))
    ?? ("access links ip mac".includes(term) ? pages[0] : undefined);
}

export function AccessSettings({ initialPage = "Devices" }: { initialPage?: AccessPage }) {
  const [page, setPage] = useState(initialPage);

  return (
    <section aria-label="Access" className="settings-models">
      <header className="settings-models-header"><h1>Access</h1></header>
      <div className="settings-models-content settings-access-content">
        <nav aria-label="Access pages" className="right-side-panel-view-actions settings-access-navigation">
          {pages.map((label) => (
            <button
              key={label}
              aria-current={page === label ? "page" : undefined}
              className={page === label ? "is-active" : undefined}
              onClick={() => setPage(label)}
              type="button"
            >
              {label}
            </button>
          ))}
        </nav>
        <section aria-label={page}>
          {page === "Environment" ? <p className="settings-models-empty">Coming soon.</p> : <DeviceLinks />}
        </section>
      </div>
    </section>
  );
}
