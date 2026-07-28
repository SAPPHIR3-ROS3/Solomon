import { useEffect, useMemo, useRef, useState } from "react";
import {
  fetchModelCatalog,
  saveCurrentModel,
  type ModelCatalog,
  type ModelChoice,
} from "../projects/projects";
import "./welcome-model.css";

const recentProvider = "__recent__";

type ModelControlProps = {
  onOpenChange?: (open: boolean) => void;
  open?: boolean;
};

export function ModelControl({ open, onOpenChange }: ModelControlProps) {
  const [internalOpen, setInternalOpen] = useState(false);
  const isOpen = open ?? internalOpen;
  const setOpen = (next: boolean) => {
    onOpenChange?.(next);
    if (open === undefined) setInternalOpen(next);
  };
  const controlRef = useRef<HTMLDivElement>(null);
  const [catalog, setCatalog] = useState<ModelCatalog>({ current: { provider: "", model: "" }, providers: [], recent: [] });
  const [selected, setSelected] = useState<ModelChoice>({ provider: "", model: "" });
  const [activeProvider, setActiveProvider] = useState(recentProvider);
  const [query, setQuery] = useState("");
  const [loadError, setLoadError] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    void fetchModelCatalog(controller.signal)
      .then((data) => {
        setCatalog(data);
        setSelected(data.current.model ? data.current : firstChoice(data));
        setLoadError(false);
      })
      .catch(() => setLoadError(true));
    return () => controller.abort();
  }, []);

  useEffect(() => {
    if (!isOpen) return;
    const close = (event: PointerEvent) => {
      if (!controlRef.current?.contains(event.target as Node)) setOpen(false);
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setOpen(false);
    };
    document.addEventListener("pointerdown", close);
    document.addEventListener("keydown", closeOnEscape);
    return () => {
      document.removeEventListener("pointerdown", close);
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, [isOpen]);

  const recentModels = useMemo(() => {
    const seen = new Set<string>();
    const out: ModelChoice[] = [];
    for (const choice of [selected, ...catalog.recent]) {
      if (!choice.provider || !choice.model) continue;
      const key = `${choice.provider}:${choice.model}`;
      if (seen.has(key)) continue;
      seen.add(key);
      out.push(choice);
      if (out.length >= 10) break;
    }
    return out;
  }, [catalog.recent, selected]);

  const showingRecents = activeProvider === recentProvider;
  const activeGroup = catalog.providers.find((group) => group.provider === activeProvider);
  const activeLabel = showingRecents ? "Recent" : activeProvider;
  const visibleModels = useMemo(() => {
    const source = showingRecents
      ? recentModels
      : (activeGroup?.models ?? []).map((model) => ({ provider: activeProvider, model }));
    const needle = query.trim().toLowerCase();
    if (!needle) return source;
    return source.filter((choice) => choice.model.toLowerCase().includes(needle) || choice.provider.toLowerCase().includes(needle));
  }, [activeGroup, activeProvider, query, recentModels, showingRecents]);

  async function selectModel(choice: ModelChoice) {
    const previous = selected;
    setSelected(choice);
    setOpen(false);
    try {
      setSelected(await saveCurrentModel(choice.provider, choice.model));
      const next = await fetchModelCatalog();
      setCatalog(next);
    } catch {
      setSelected(previous);
    }
  }

  return (
    <div className="welcome-model" ref={controlRef}>
      <button
        aria-expanded={isOpen}
        aria-label="Select model"
        className="welcome-model-trigger"
        onClick={() => {
          const next = !isOpen;
          setOpen(next);
          if (next) {
            setActiveProvider(recentProvider);
            setQuery("");
          }
        }}
        type="button"
      >
        <span>{selected.model || "Select model"}</span>
        <ChevronIcon className={isOpen ? "is-open" : undefined} />
      </button>
      {isOpen ? (
        <div aria-label="Models configured in Solomon Home" className="welcome-model-menu" role="dialog">
          <nav aria-label="Providers" className="welcome-provider-rail">
            <header>{loadError ? "Cached" : "Providers"}</header>
            <button
              aria-pressed={showingRecents}
              className={`welcome-provider-recent${showingRecents ? " is-active" : ""}`}
              onClick={() => {
                setActiveProvider(recentProvider);
                setQuery("");
              }}
              title="Recent"
              type="button"
            >
              <span><HistoryIcon /></span>
              <strong>Recent</strong>
              <small>{recentModels.length}</small>
            </button>
            {catalog.providers.map((group) => (
              <button
                aria-pressed={activeProvider === group.provider}
                className={activeProvider === group.provider ? "is-active" : undefined}
                key={group.provider}
                onClick={() => {
                  setActiveProvider(group.provider);
                  setQuery("");
                }}
                title={group.provider}
                type="button"
              >
                <span><ProviderIcon provider={group.provider} /></span>
                <strong>{group.provider}</strong>
                <small>{group.models.length}</small>
              </button>
            ))}
          </nav>
          <div className="welcome-model-browser">
            <label className="welcome-model-search">
              <SearchIcon />
              <input
                aria-label="Search models"
                onChange={(event) => setQuery(event.target.value)}
                placeholder={`Search ${activeLabel.toLowerCase()}…`}
                value={query}
              />
            </label>
            <header>
              <span>{activeLabel}</span>
              <small>
                {visibleModels.length} {visibleModels.length === 1 ? "model" : "models"}
                {!showingRecents && activeGroup?.complete === false ? " · cached" : ""}
              </small>
            </header>
            <div aria-label={`${activeLabel} models`} className="welcome-model-list" role="listbox">
              {visibleModels.length ? visibleModels.map((choice) => {
                const isSelected = selected.provider === choice.provider && selected.model === choice.model;
                return (
                  <button
                    aria-selected={isSelected}
                    className={isSelected ? "is-selected" : undefined}
                    key={`${choice.provider}:${choice.model}`}
                    onClick={() => void selectModel(choice)}
                    role="option"
                    type="button"
                  >
                    <span>
                      <strong>{choice.model}</strong>
                      <small>{choice.provider}</small>
                    </span>
                    {isSelected ? <CheckIcon /> : null}
                  </button>
                );
              }) : <p>No models match your search.</p>}
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}

function firstChoice(catalog: ModelCatalog): ModelChoice {
  const group = catalog.providers[0];
  if (!group?.models[0]) return { provider: "", model: "" };
  return { provider: group.provider, model: group.models[0] };
}

function ChevronIcon({ className }: { className?: string }) {
  return (
    <svg aria-hidden="true" className={`welcome-chevron${className ? ` ${className}` : ""}`} viewBox="0 0 24 24">
      <path d="m7 10 5 5 5-5" />
    </svg>
  );
}

function HistoryIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8" />
      <path d="M3 3v5h5M12 7v5l4 2" />
    </svg>
  );
}

function SearchIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <circle cx="11" cy="11" r="7" />
      <path d="m20 20-3.5-3.5" />
    </svg>
  );
}

function CheckIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="m5 12 5 5L20 7" />
    </svg>
  );
}

function ProviderIcon({ provider }: { provider: string }) {
  const normalized = provider.toLowerCase();
  if (normalized.includes("chatgpt") || normalized.includes("openai")) {
    return (
      <svg aria-hidden="true" viewBox="0 0 24 24">
        <path d="M22.28 9.82a6 6 0 0 0-.52-4.91 6.05 6.05 0 0 0-6.51-2.9A6.07 6.07 0 0 0 4.98 4.18a5.98 5.98 0 0 0-4 2.9 6.05 6.05 0 0 0 .74 7.1 5.98 5.98 0 0 0 .51 4.91 6.05 6.05 0 0 0 6.51 2.9A5.98 5.98 0 0 0 13.26 24a6.06 6.06 0 0 0 5.77-4.21 5.99 5.99 0 0 0 4-2.9 6.06 6.06 0 0 0-.75-7.07z" fill="currentColor" stroke="none" />
      </svg>
    );
  }
  if (normalized.includes("claude") || normalized.includes("anthropic")) {
    return (
      <svg aria-hidden="true" viewBox="0 0 24 24">
        <path d="M17.3 3.54h-3.67l6.7 16.92H24ZM6.7 3.54 0 20.46h3.74l1.37-3.55h7.01l1.37 3.55h3.74L10.54 3.54Zm-.37 10.22 2.29-5.95 2.29 5.95Z" fill="currentColor" stroke="none" />
      </svg>
    );
  }
  if (normalized.includes("cursor")) {
    return (
      <svg aria-hidden="true" viewBox="0 0 24 24">
        <path d="m4 4 7 16 2.5-6.5L20 11Z" fill="currentColor" stroke="none" />
      </svg>
    );
  }
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01" />
    </svg>
  );
}
