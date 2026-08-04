import { useEffect, useMemo, useState } from "react";
import {
  connectProvider,
  fetchModelCatalog,
  saveCurrentModel,
  setModelEnabled,
  type ModelCatalog,
  type ProviderCatalog,
} from "../projects/projects";
import { InputModeIcons, ProviderIcon } from "../home/ModelControl";

type ProviderKind = 1 | 2 | 3 | 4 | 5;

type ModelsPageState = {
  catalog: ModelCatalog;
  error: string;
  loading: boolean;
};

const providerKinds: Array<{ kind: ProviderKind; label: string }> = [
  { kind: 1, label: "ChatGPT Sub (browser sign-in)" },
  { kind: 2, label: "OpenAI Compatible API" },
  { kind: 3, label: "Anthropic Compatible API" },
  { kind: 4, label: "Claude Sub (browser sign-in)" },
  { kind: 5, label: "Cursor API" },
];

const emptyCatalog: ModelCatalog = {
  current: { model: "", provider: "" },
  providers: [],
  recent: [],
};

function catalogWithModelVisibility(catalog: ModelCatalog, provider: string, model: string, enabled: boolean): ModelCatalog {
  return {
    ...catalog,
    providers: catalog.providers.map((group) => {
      if (group.provider !== provider) return group;
      const disabled = group.disabled ?? [];
      return {
        ...group,
        disabled: enabled
          ? disabled.filter((id) => id !== model)
          : disabled.includes(model) ? disabled : [...disabled, model],
      };
    }),
  };
}

export function ModelsPage() {
  const [state, setState] = useState<ModelsPageState>({ catalog: emptyCatalog, error: "", loading: true });
  const [query, setQuery] = useState("");
  const [isAddingProvider, setIsAddingProvider] = useState(false);
  const [isSavingModel, setIsSavingModel] = useState("");
  const [isUpdatingVisibility, setIsUpdatingVisibility] = useState("");

  async function loadCatalog() {
    setState((current) => ({ ...current, error: "", loading: true }));
    try {
      const catalog = await fetchModelCatalog();
      setState({ catalog, error: "", loading: false });
    } catch (error) {
      setState((current) => ({
        ...current,
        error: error instanceof Error ? error.message : "Unable to load models.",
        loading: false,
      }));
    }
  }

  useEffect(() => {
    void loadCatalog();
  }, []);

  useEffect(() => {
    if (!isAddingProvider) return;
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setIsAddingProvider(false);
    };
    document.addEventListener("keydown", closeOnEscape);
    return () => document.removeEventListener("keydown", closeOnEscape);
  }, [isAddingProvider]);

  const visibleProviders = useMemo(() => {
    const needle = query.trim().toLocaleLowerCase();
    return (needle
      ? state.catalog.providers.map((provider) => ({
        ...provider,
        models: provider.models.filter((model) => (provider.provider + " " + model).toLocaleLowerCase().includes(needle)),
      }))
      : state.catalog.providers).filter((provider) => provider.models.length > 0);
  }, [query, state.catalog.providers]);

  const currentKey = state.catalog.current.provider + ":" + state.catalog.current.model;

  async function selectModel(provider: string, model: string) {
    const key = provider + ":" + model;
    if (key === currentKey) return;
    setIsSavingModel(key);
    try {
      const current = await saveCurrentModel(provider, model);
      setState((previous) => ({ ...previous, catalog: { ...previous.catalog, current } }));
    } catch (error) {
      setState((previous) => ({ ...previous, error: error instanceof Error ? error.message : "Unable to save model." }));
    } finally {
      setIsSavingModel("");
    }
  }

  async function toggleModel(provider: string, model: string, enabled: boolean) {
    const key = provider + ":" + model;
    const previousCatalog = state.catalog;
    setIsUpdatingVisibility(key);
    setState((previous) => ({
      ...previous,
      catalog: catalogWithModelVisibility(previous.catalog, provider, model, enabled),
    }));
    try {
      const visibility = await setModelEnabled(provider, model, enabled);
      setState((previous) => ({
        ...previous,
        catalog: catalogWithModelVisibility(previous.catalog, visibility.provider, visibility.model, visibility.enabled),
      }));
    } catch (error) {
      setState((previous) => ({
        ...previous,
        catalog: previousCatalog,
        error: error instanceof Error ? error.message : "Unable to save model visibility.",
      }));
    } finally {
      setIsUpdatingVisibility("");
    }
  }

  async function addProvider(request: { apiKey: string; baseURL: string; kind: ProviderKind; name: string }) {
    setState((current) => ({ ...current, error: "", loading: true }));
    try {
      await connectProvider(request);
      setIsAddingProvider(false);
      await loadCatalog();
    } catch (error) {
      setState((current) => ({
        ...current,
        error: error instanceof Error ? error.message : "Unable to connect provider.",
        loading: false,
      }));
    }
  }

  return (
    <section aria-label="Models" className="settings-models">
      <header className="settings-models-header">
        <h1>Models</h1>
      </header>

      <div className="settings-models-content">
        <section aria-labelledby="settings-add-provider-title" className="settings-models-panel settings-models-add-panel">
          <div className="settings-models-panel-heading">
            <div>
              <h2 id="settings-add-provider-title">Add provider</h2>
              <p>Connect another provider to use its models in Solomon.</p>
            </div>
            <button className="settings-models-add-provider" onClick={() => setIsAddingProvider(true)} type="button">
              <PlusIcon />
              <span>Add provider</span>
            </button>
          </div>
        </section>

        <section aria-labelledby="settings-providers-title" className="settings-models-panel">
          <div className="settings-models-panel-heading">
            <div>
              <h2 id="settings-providers-title">Providers</h2>
              <p>Manage the providers configured in Solomon.</p>
            </div>
          </div>

          <div className="settings-provider-list">
            {state.catalog.providers.map((provider) => <ProviderRow key={provider.provider} provider={provider} />)}
            {!state.catalog.providers.length && !state.loading ? <p className="settings-models-empty">No providers configured.</p> : null}
          </div>
        </section>

        <section aria-labelledby="settings-task-models-title" className="settings-models-panel settings-task-models-panel">
          <div className="settings-models-panel-heading">
            <div>
              <h2 id="settings-task-models-title">Task models</h2>
              <p>Choose the model Solomon uses for tasks.</p>
            </div>
            <button aria-label="Refresh models" className="settings-models-refresh" disabled={state.loading} onClick={() => void loadCatalog()} type="button">
              <RefreshIcon />
            </button>
          </div>

          <label className="settings-models-search">
            <SearchIcon />
            <input aria-label="Search models" onChange={(event) => setQuery(event.target.value)} placeholder="Add or search model" type="search" value={query} />
          </label>

          <div className="settings-model-list">
            {visibleProviders.map((provider) => (
              <section aria-label={provider.provider} className="settings-model-provider-group" key={provider.provider}>
                <h3 className="settings-model-provider-heading">{provider.provider}</h3>
                <div className="settings-model-provider-list">
                  {provider.models.map((model) => {
                    const key = provider.provider + ":" + model;
                    const selected = key === currentKey;
                    const enabled = !(provider.disabled ?? []).includes(model);
                    return (
                      <div className={"settings-model-row" + (selected ? " is-current" : "")} key={key}>
                        <button aria-current={selected ? "true" : undefined} className="settings-model-select" disabled={Boolean(isSavingModel)} onClick={() => void selectModel(provider.provider, model)} type="button">
                          <span className="settings-model-row-copy">
                            <span className="settings-model-row-title">
                              <strong>{model}</strong>
                              <InputModeIcons modes={provider.metadata[model]?.input ?? []} />
                            </span>
                          </span>
                          {isSavingModel === key ? <span className="settings-model-saving">Saving…</span> : selected ? <CheckIcon /> : null}
                        </button>
                        <button
                          aria-checked={enabled}
                          aria-label={`${enabled ? "Hide" : "Show"} ${model} in the chat model selector`}
                          className={`settings-model-toggle${enabled ? " is-enabled" : ""}`}
                          disabled={isUpdatingVisibility === key}
                          onClick={() => void toggleModel(provider.provider, model, !enabled)}
                          role="switch"
                          type="button"
                        >
                          <span aria-hidden="true" />
                        </button>
                      </div>
                    );
                  })}
                </div>
              </section>
            ))}
            {!visibleProviders.length && !state.loading ? <p className="settings-models-empty">No models match your search.</p> : null}
            {state.loading ? <p className="settings-models-empty">Loading models…</p> : null}
          </div>
        </section>

        {state.error ? <p className="settings-models-error" role="alert">{state.error}</p> : null}
      </div>

      {isAddingProvider ? (
        <div className="settings-provider-modal-backdrop" onClick={() => setIsAddingProvider(false)}>
          <section aria-labelledby="settings-provider-modal-title" aria-modal="true" className="settings-provider-modal" onClick={(event) => event.stopPropagation()} role="dialog">
            <header className="settings-provider-modal-header">
              <h2 id="settings-provider-modal-title">Add provider</h2>
              <button aria-label="Close add provider" className="settings-provider-modal-close" onClick={() => setIsAddingProvider(false)} type="button">
                <CloseIcon />
              </button>
            </header>
            <ProviderForm onCancel={() => setIsAddingProvider(false)} onSubmit={addProvider} />
          </section>
        </div>
      ) : null}
    </section>
  );
}

function ProviderForm({ onCancel, onSubmit }: { onCancel: () => void; onSubmit: (request: { apiKey: string; baseURL: string; kind: ProviderKind; name: string }) => void }) {
  const [step, setStep] = useState<1 | 2>(1);
  const [kind, setKind] = useState<ProviderKind>(2);
  const [name, setName] = useState("");
  const [baseURL, setBaseURL] = useState("");
  const [apiKey, setAPIKey] = useState("");
  const needsName = kind === 2 || kind === 3;
  const needsAPIFields = kind === 2 || kind === 3;
  const needsAPIKey = needsAPIFields || kind === 5;

  return (
    <form className="settings-provider-form" onSubmit={(event) => {
      event.preventDefault();
      if (step === 1) {
        setStep(2);
        return;
      }
      onSubmit({ apiKey: apiKey.trim(), baseURL: baseURL.trim(), kind, name: name.trim() });
    }}>
      <p className="settings-provider-step">Step {step} of 2</p>
      {step === 1 ? (
        <>
          <label>
            <span>Provider type</span>
            <select onChange={(event) => setKind(Number(event.target.value) as ProviderKind)} value={kind}>
              {providerKinds.map((provider) => <option key={provider.kind} value={provider.kind}>{provider.label}</option>)}
            </select>
          </label>
          {needsName ? <label><span>Display name</span><input onChange={(event) => setName(event.target.value)} placeholder="My provider" required value={name} /></label> : null}
        </>
      ) : (
        <>
          {needsAPIFields ? <label><span>Base URL</span><input onChange={(event) => setBaseURL(event.target.value)} placeholder="https://api.example.com/v1" required type="url" value={baseURL} /></label> : null}
          {needsAPIKey ? <label><span>API key</span><input onChange={(event) => setAPIKey(event.target.value)} placeholder="Enter API key" required type="password" value={apiKey} /></label> : null}
          {(kind === 1 || kind === 4) ? <p className="settings-provider-form-note">Solomon will open the provider sign-in flow in your browser.</p> : null}
        </>
      )}
      <div className="settings-provider-form-actions">
        {step === 1 ? <button className="settings-provider-cancel" onClick={onCancel} type="button">Cancel</button> : <button className="settings-provider-cancel" onClick={() => setStep(1)} type="button">Back</button>}
        <button className="settings-provider-submit" type="submit">{step === 1 ? "Continue" : "Connect provider"}</button>
      </div>
    </form>
  );
}

function ProviderRow({ provider }: { provider: ProviderCatalog }) {
  return (
    <div className="settings-provider-row">
      <div className="settings-provider-row-main">
        <span className="settings-provider-icon"><ProviderIcon provider={provider.provider} /></span>
        <span className="settings-provider-row-copy">
          <strong>{provider.provider}</strong>
          <small>{provider.models.length} {provider.models.length === 1 ? "model" : "models"}{provider.complete ? "" : " · cached"}</small>
        </span>
      </div>
      <span className="settings-provider-connected">Configured</span>
    </div>
  );
}

function SearchIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><circle cx="11" cy="11" r="6.5" /><path d="m16 16 4 4" /></svg>;
}

function PlusIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M12 5v14M5 12h14" /></svg>;
}

function RefreshIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M20 11a8 8 0 0 0-14.9-4M4 5v4h4M4 13a8 8 0 0 0 14.9 4M20 19v-4h-4" /></svg>;
}

function CheckIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="m5 12 5 5L20 7" /></svg>;
}

function CloseIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="m6 6 12 12M18 6 6 18" /></svg>;
}
