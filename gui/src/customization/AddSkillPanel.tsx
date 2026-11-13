import { useState } from "react";
import "./fields.css";
import { serverEndpoint } from "../platform";
import { fetchCustomizationSkills, type CustomizationCatalogItem } from "./rules";

export function AddSkillPanel({ onCancel, onAdded }: {
  onCancel: () => void;
  onAdded: (items: CustomizationCatalogItem[]) => void;
}) {
  const [source, setSource] = useState("");
  const [name, setName] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  async function add() {
    if (saving || !source.trim()) return;
    setSaving(true);
    setError("");
    try {
      const response = await fetch(await serverEndpoint("/__solomon/skills"), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ source, name }),
      });
      if (!response.ok) {
        const payload = await response.json();
        throw new Error(payload.error || "Could not add this skill.");
      }
      onAdded(await fetchCustomizationSkills());
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Could not add this skill.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <form aria-label="Add skill" className="customization-rule-editor" onSubmit={(event) => { event.preventDefault(); void add(); }}>
      <h2>Add skill</h2>
      <p className="customization-catalog-detail">Import a global skill from a Markdown file or skills.sh, using the same installer as /add skill.</p>
      <fieldset className="customization-subagent-fields" disabled={saving}>
        <label>Source<input className="customization-text-input" autoFocus required value={source} onChange={(event) => setSource(event.target.value)} placeholder="/path/to/SKILL.md or https://..." /></label>
        <label>Name (optional if included in the file)<input className="customization-text-input" value={name} onChange={(event) => setName(event.target.value)} /></label>
      </fieldset>
      <div className="customization-rule-editor-actions">
        <button className="customization-rule-cancel" disabled={saving} onClick={onCancel} type="button">Cancel</button>
        <button className="customization-rule-save" disabled={saving || !source.trim()} type="submit">{saving ? "Adding..." : "Add"}</button>
      </div>
      {error ? <p className="customization-rule-error" role="alert">{error}</p> : null}
    </form>
  );
}
