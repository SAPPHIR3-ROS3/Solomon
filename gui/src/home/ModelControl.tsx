import type { ReasoningCapability } from "../projects/reasoning";
import { invalidateModelCatalogCache } from "../projects/models";
import "./logos.css";
import { ProviderIcon } from "./ProviderIcon";
export { ProviderIcon } from "./ProviderIcon";
import { useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import {
  connectProvider,
  fetchModelCatalog,
  MODEL_CATALOG_CHANGED_EVENT,
  getCachedModelCatalog,
  getCachedCurrentModel,
  saveCurrentModel,
  type ModelCatalog,
  type ModelChoice,
} from "../projects/projects";
import type { AddProviderRequest } from "../settings/ModelsPage";
import "./welcome-model.css";
import { modelLabel } from "../projects/modelLabels";

const recentProvider = "__recent__";
const emptyCatalog: ModelCatalog = { current: { provider: "", model: "" }, providers: [], recent: [] };

type ModelControlProps = {
 onReasoningCapabilityChange?: (capability: ReasoningCapability | undefined) => void;
  onFastModeAvailableChange?: (available: boolean) => void;
  onReasoningModeChange?: (mode: ReasoningMode) => void;
  onModelChange?: (choice: ModelChoice, modes?: { fastMode?: boolean; reasoningEffort?: string }) => void;
  onOpenChange?: (open: boolean) => void;
  open?: boolean;
};

export type ReasoningMode = "none" | "toggle" | "levels";

function listHasModel(list: string[] | undefined, model: string): boolean {
  const needle = model.trim().toLowerCase();
  return Boolean(list?.some((id) => id.trim().toLowerCase() === needle));
}

function modelReasoningMode(provider: { thinkingModels?: string[]; thinkingLevelModels?: string[] } | undefined, model: string): ReasoningMode {
  if (!provider || (!Array.isArray(provider.thinkingModels) && !Array.isArray(provider.thinkingLevelModels))) return "levels";
  if (listHasModel(provider.thinkingLevelModels, model)) return "levels";
  if (listHasModel(provider.thinkingModels, model)) return "toggle";
  return "none";
}

function modelSupportsFast(provider: { fastModels?: string[]; supportsFastMode: boolean } | undefined, model: string): boolean {
  if (!provider?.supportsFastMode) return false;
  if (!provider.fastModels) return true;
  const needle = model.trim().toLowerCase();
  return provider.fastModels.some((id) => id.trim().toLowerCase() === needle);
}

function modelMatchesQuery(provider: string, model: string, needle: string): boolean {
  const haystack = (provider + " " + model + " " + modelLabel(model)).toLowerCase();
  if (haystack.includes(needle)) return true;
  const compactNeedle = needle.replace(/[.-]/g, "");
  const compactHaystack = haystack.replace(/[.-]/g, "");
  return compactNeedle.length > 0 && compactHaystack.includes(compactNeedle);
}

export function ModelControl({ onReasoningCapabilityChange, onFastModeAvailableChange, onReasoningModeChange, onModelChange, open, onOpenChange }: ModelControlProps) {
  const [internalOpen, setInternalOpen] = useState(false);
  const isOpen = open ?? internalOpen;
  const setOpen = (next: boolean) => {
    onOpenChange?.(next);
    if (open === undefined) setInternalOpen(next);
  };
  const controlRef = useRef<HTMLDivElement>(null);
  const [catalog, setCatalog] = useState<ModelCatalog>(() => getCachedModelCatalog() ?? emptyCatalog);
  const [selected, setSelected] = useState<ModelChoice>(() => getCachedCurrentModel() ?? getCachedModelCatalog()?.current ?? { provider: "", model: "" });
  const [activeProvider, setActiveProvider] = useState(recentProvider);
  const [query, setQuery] = useState("");
  const [loadError, setLoadError] = useState(false);
  const [catalogLoaded, setCatalogLoaded] = useState(false);
  const [catalogLoading, setCatalogLoading] = useState(false);
  const [isAddingProvider, setIsAddingProvider] = useState(false);
  const [addProviderError, setAddProviderError] = useState("");

  function notifyModelChange(choice: ModelChoice, sourceCatalog = catalog) {
    const normalizedProvider = choice.provider.trim().toLowerCase();
    const provider = sourceCatalog.providers.find((group) => group.provider.trim().toLowerCase() === normalizedProvider);
    onFastModeAvailableChange?.(modelSupportsFast(provider, choice.model));
 onReasoningCapabilityChange?.(provider?.reasoning?.[choice.model]);
    onReasoningModeChange?.(modelReasoningMode(provider, choice.model));
    onModelChange?.(choice);
  }

  useEffect(() => {
    const applyCatalog = (event: Event) => {
      const data = (event as CustomEvent<ModelCatalog>).detail;
      if (!data?.providers) return;
      setCatalog(data);
      setLoadError(false);
      setCatalogLoaded(true);
 notifyModelChange(selected, data);
    };
    window.addEventListener(MODEL_CATALOG_CHANGED_EVENT, applyCatalog);
    return () => window.removeEventListener(MODEL_CATALOG_CHANGED_EVENT, applyCatalog);
  }, [selected.provider, selected.model]);

  useEffect(() => {
    if (catalogLoaded) return;
    let cancelled = false;
    setCatalogLoading(true);
    void fetchModelCatalog()
      .then((data) => {
        if (cancelled) return;
        setCatalog(data);
        const choice = data.current.model ? data.current : firstChoice(data);
        setSelected(choice);
        notifyModelChange(choice, data);
        setLoadError(false);
        setCatalogLoaded(true);
      })
      .catch(() => {
        if (!cancelled) setLoadError(true);
      })
      .finally(() => {
        if (!cancelled) setCatalogLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [catalogLoaded, isOpen]);

  useEffect(() => {
    if (!isOpen) return;
    invalidateModelCatalogCache();
    void fetchModelCatalog().catch(() => setLoadError(true));
  }, [isOpen]);

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

  const visibleRecentModels = useMemo(() => recentModels.filter((choice) => {
    const provider = catalog.providers.find((group) => group.provider === choice.provider);
    if (provider?.disabled.includes(choice.model)) return false;
    if (provider?.complete && !provider.models.includes(choice.model)) return false;
    return true;
  }), [catalog.providers, recentModels]);

  const showingRecents = activeProvider === recentProvider;
  const activeGroup = catalog.providers.find((group) => group.provider === activeProvider);
  const activeLabel = showingRecents ? "Recent" : activeProvider;
  const visibleModels = useMemo(() => {
    const source = showingRecents
      ? visibleRecentModels.map((choice) => ({
          ...choice,
          info: catalog.providers.find((group) => group.provider === choice.provider)?.metadata[choice.model],
        }))
      : (activeGroup?.models ?? []).filter((model) => !activeGroup?.disabled.includes(model)).map((model) => ({ provider: activeProvider, model, info: activeGroup?.metadata[model] }));
    const needle = query.trim().toLowerCase();
    if (!needle) return source;
    return source.filter((choice) => modelMatchesQuery(choice.provider, choice.model, needle));
  }, [activeGroup, activeProvider, catalog.providers, query, showingRecents, visibleRecentModels]);

  const noProviders = catalogLoaded && !catalogLoading && catalog.providers.length === 0;

  async function submitProvider(request: AddProviderRequest) {
    setAddProviderError("");
    try {
      await connectProvider(request);
      const next = await fetchModelCatalog(true);
      setCatalog(next);
      setCatalogLoaded(true);
      setIsAddingProvider(false);
    } catch (error) {
      setAddProviderError(error instanceof Error ? error.message : "Unable to connect provider.");
    }
  }

  async function selectModel(choice: ModelChoice) {
    const previous = selected;
    setSelected(choice);
    setOpen(false);
    try {
      const saved = await saveCurrentModel(choice.provider, choice.model);
      setSelected(saved);
      notifyModelChange(saved);
      onModelChange?.(saved, { fastMode: saved.fastMode, reasoningEffort: saved.reasoningEffort });
      const next = await fetchModelCatalog();
      setCatalog(next);
    } catch {
      setSelected(previous);
      notifyModelChange(previous);
    }
  }

  return (
    <div className="welcome-model" ref={controlRef}>
      <button
        aria-expanded={isOpen}
        aria-label="Select model"
        className="welcome-model-trigger"
        onClick={() => {
          if (noProviders) {
            setAddProviderError("");
            setIsAddingProvider(true);
            return;
          }
          const next = !isOpen;
          setOpen(next);
          if (next) {
            setActiveProvider(recentProvider);
            setQuery("");
          }
        }}
        type="button"
      >
        {selected.provider ? <span className="welcome-model-provider-icon"><ProviderIcon provider={selected.provider} /></span> : null}
        <span>{modelLabel(selected.model) || "Select model"}</span>
        <ChevronIcon className={isOpen ? "is-open" : undefined} />
      </button>
      {isOpen ? (
        <div aria-label="Models configured in Solomon Home" className="welcome-model-menu" role="dialog">
          <nav aria-label="Providers" className="welcome-provider-rail">
            <header>{loadError ? "Cached" : catalogLoading && !catalog.providers.length && !catalog.recent.length ? "Loading…" : "Providers"}</header>
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
              <small>{visibleRecentModels.length}</small>
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
                <small>{group.models.filter((model) => !group.disabled.includes(model)).length}</small>
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
              {catalogLoading && !catalog.providers.length && !catalog.recent.length ? <p>Loading models…</p> : visibleModels.length ? visibleModels.map((choice) => {
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
                    <span className="welcome-model-copy">
                      <span className="welcome-model-title">
                        <strong title={modelLabel(choice.model)}>{modelLabel(choice.model)}</strong>
                        <InputModeIcons modes={choice.info?.input ?? []} />
                      </span>
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
      {isAddingProvider ? createPortal(
        <AddProviderDialog error={addProviderError} onClose={() => setIsAddingProvider(false)} onSubmit={(request) => void submitProvider(request)} />,
        document.body,
      ) : null}
    </div>
  );
}

const inputModeLabels: Record<string, string> = {
  audio: "Audio input",
  image: "Image input",
  pdf: "PDF input",
  text: "Text input",
  video: "Video input",
};

export function InputModeIcons({ modes }: { modes: string[] }) {
  const supported = [...new Set(modes.map((mode) => mode.trim().toLowerCase()))]
    .filter((mode) => mode in inputModeLabels);
  if (!supported.length) return null;
  return (
    <span aria-label={supported.map((mode) => inputModeLabels[mode]).join(", ")} className="welcome-model-inputs" role="group">
      {supported.map((mode) => <InputModeIcon key={mode} mode={mode} />)}
    </span>
  );
}

function InputModeIcon({ mode }: { mode: string }) {
  const label = inputModeLabels[mode];
  if (mode === "text") return <span aria-label={label} className="welcome-model-input-icon is-text" role="img">T</span>;
  if (mode === "pdf") return (
    <svg aria-label={label} className="welcome-model-input-icon" role="img" viewBox="0 0 16 16">
      <path d="M5 3.5h4l2 2v7H5z" /><path d="M9 3.5v2h2M6.5 8h3M6.5 10h3" />
    </svg>
  );
  if (mode === "image") return (
    <svg aria-label={label} className="welcome-model-input-icon" role="img" viewBox="0 0 16 16">
      <circle cx="5.2" cy="5.5" r="1.1" /><path d="m2.5 12 3.4-3 2.2 1.8 2.2-2.2 3.1 3.4" />
    </svg>
  );
  if (mode === "audio") return (
    <svg aria-label={label} className="welcome-model-input-icon" role="img" viewBox="0 0 16 16">
      <path d="M3 6.5h2.2L8 4.3v7.4l-2.8-2.2H3zM10 6.2a2.6 2.6 0 0 1 0 3.6M11.5 4.7a4.7 4.7 0 0 1 0 6.6" />
    </svg>
  );
  return (
    <svg aria-label={label} className="welcome-model-input-icon" role="img" viewBox="0 0 16 16">
      <path d="M3.5 5.3h6v5.4h-6zM9.5 7l2.8-1.5v5L9.5 9z" />
    </svg>
  );
}

function AddProviderDialog({ error, onClose, onSubmit }: { error?: string; onClose: () => void; onSubmit: (request: AddProviderRequest) => void }) {
  const [Dialog, setDialog] = useState<typeof import("../settings/ModelsPage").AddProviderModal | null>(null);
  useEffect(() => {
    let active = true;
    void import("../settings/ModelsPage").then((module) => {
      if (active) setDialog(() => module.AddProviderModal);
    });
    return () => { active = false; };
  }, []);
  if (!Dialog) return null;
  return <Dialog error={error} onClose={onClose} onSubmit={onSubmit} />;
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
