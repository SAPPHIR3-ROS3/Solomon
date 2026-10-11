import { useEffect, useState } from "react";
import { ScoreInput } from "./ScoreInput";
import { Select } from "../Select";
import { fetchModelCatalog, type ProviderCatalog } from "../projects/models";
import { createCustomizationSubagent, fetchRolesTable, type CustomizationCatalogItem, type SubagentScore } from "./rules";

export function NewSubagentPanel({ onCancel, onCreated }: {
  onCancel: () => void;
  onCreated: (items: CustomizationCatalogItem[]) => void;
}) {
  const [providers, setProviders] = useState<ProviderCatalog[]>([]);
  const [provider, setProvider] = useState("");
  const [model, setModel] = useState("");
  const [detail, setDetail] = useState("");
  const [scores, setScores] = useState<SubagentScore[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    void Promise.all([fetchModelCatalog(), fetchRolesTable()]).then(([catalog, table]) => {
      if (cancelled) return;
      setProviders(catalog.providers);
      setScores(table.characteristics.map((id) => ({ id, label: table.catalog.find((entry) => entry.id === id)?.label ?? id, value: 0 })));
      if (!table.characteristics.length) setError("Configure and save the reference table using Table before adding a subagent.");
      else if (!catalog.providers.some((entry) => entry.models.length)) setError("Connect a provider with available models in Settings before adding a subagent.");
    }).catch(() => {
      if (!cancelled) setError("Could not load models and the reference table. Close this form and try again.");
    }).finally(() => {
      if (!cancelled) setLoading(false);
    });
    return () => { cancelled = true; };
  }, []);

  async function save() {
    if (saving || loading || !provider || !model || !scores.length) return;
    setSaving(true);
    setError("");
    try {
      onCreated(await createCustomizationSubagent(provider, model, detail, scores.map(({ id, value }) => ({ id, value }))));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Could not create this subagent.");
    } finally {
      setSaving(false);
    }
  }

  const models = providers.find((entry) => entry.provider === provider)?.models ?? [];
  return (
    <form aria-label="New subagent" className="customization-rule-editor" onSubmit={(event) => { event.preventDefault(); void save(); }}>
      <h2>New subagent</h2>
      {loading ? <p className="customization-empty">Loading models and reference table...</p> : null}
      <fieldset className="customization-subagent-fields" disabled={loading || saving}>
        <label>Provider
          <Select label="Provider" disabled={loading || saving} value={provider} placeholder="Select a provider" onChange={(value) => { setProvider(value); setModel(""); }} options={providers.filter((entry) => entry.models.length).map((entry) => ({ value: entry.provider, label: entry.provider }))} />
        </label>
        <label>Model
          <Select label="Model" disabled={loading || saving || !provider} value={model} placeholder="Select a model" onChange={setModel} options={models.map((id) => ({ value: id, label: id }))} />
        </label>
        <label>Description (optional)<textarea value={detail} onChange={(event) => setDetail(event.target.value)} /></label>
        <div className="customization-subagent-scores">
          {scores.map((score) => <label className="customization-subagent-score" key={score.id}>
            <span>{score.label}</span>
            <ScoreInput required value={score.value} onChange={(value) => setScores((current) => current.map((entry) => entry.id === score.id ? { ...entry, value } : entry))} />
          </label>)}
        </div>
      </fieldset>
      <div className="customization-rule-editor-actions">
        <button className="customization-rule-cancel" disabled={saving} onClick={onCancel} type="button">Cancel</button>
        <button className="customization-rule-save" disabled={loading || saving || !provider || !model || !scores.length} type="submit">{saving ? "Creating..." : "Create"}</button>
      </div>
      {error ? <p className="customization-rule-error" role="alert">{error}</p> : null}
    </form>
  );
}
