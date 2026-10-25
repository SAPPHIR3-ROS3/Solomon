import { useEffect, useState } from "react";
import { fetchRolesTable, saveRolesTable, type RolesTableCharacteristic } from "./rules";

export function useRolesTableEditor(active: boolean) {
  const [isOpen, setIsOpen] = useState(false);
  const [catalog, setCatalog] = useState<RolesTableCharacteristic[]>([]);
  const [draft, setDraft] = useState<string[]>([]);
  const [max, setMax] = useState(5);
  const [isLoading, setIsLoading] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!active) {
      setIsOpen(false);
      setError("");
    }
  }, [active]);

  async function open() {
    if (isLoading || isSaving) return;
    if (isOpen) {
      close();
      return;
    }
    setIsOpen(true);
    setIsLoading(true);
    setError("");
    try {
      const table = await fetchRolesTable();
      setCatalog(table.catalog);
      setDraft(table.characteristics);
      setMax(table.max);
    } catch {
      setError("Could not load the reference table.");
    } finally {
      setIsLoading(false);
    }
  }

  function close() {
    setIsOpen(false);
    setError("");
  }

  function toggle(id: string) {
    setDraft((current) => {
      if (current.includes(id)) return current.filter((entry) => entry !== id);
      if (current.length >= max) return current;
      return [...current, id];
    });
  }

  async function save() {
    if (isSaving) return;
    setIsSaving(true);
    setError("");
    try {
      const table = await saveRolesTable(draft);
      setCatalog(table.catalog);
      setDraft(table.characteristics);
      setMax(table.max);
      setIsOpen(false);
    } catch {
      setError("Could not save the reference table.");
    } finally {
      setIsSaving(false);
    }
  }

  return {
    button: active ? (
      <button
        aria-expanded={isOpen}
        className={`customization-new${isOpen ? " is-active" : ""}`}
        onClick={() => void open()}
        type="button"
      >
        Table
      </button>
    ) : null,
    isBusy: isOpen || isSaving,
    panel: active && isOpen ? (
      <RolesTablePanel
        catalog={catalog}
        error={error}
        isLoading={isLoading}
        isSaving={isSaving}
        max={max}
        onCancel={close}
        onSave={() => void save()}
        onToggle={toggle}
        selected={draft}
      />
    ) : null,
  };
}

function RolesTablePanel({
  catalog,
  error,
  isLoading,
  isSaving,
  max,
  onCancel,
  onSave,
  onToggle,
  selected,
}: {
  catalog: RolesTableCharacteristic[];
  error: string;
  isLoading: boolean;
  isSaving: boolean;
  max: number;
  onCancel: () => void;
  onSave: () => void;
  onToggle: (id: string) => void;
  selected: string[];
}) {
  const active = selected.map((id) => catalog.find((entry) => entry.id === id)).filter((entry): entry is RolesTableCharacteristic => entry !== undefined);
  const inactive = catalog.filter((entry) => !selected.includes(entry.id));
  const renderOption = (entry: RolesTableCharacteristic, isSelected: boolean) => (
    <button
      aria-pressed={isSelected}
      className={`customization-roles-table-option${isSelected ? " is-selected" : ""}`}
      disabled={isLoading || isSaving || (!isSelected && selected.length >= max)}
      key={entry.id}
      onClick={() => onToggle(entry.id)}
      type="button"
    >
      {entry.label}
    </button>
  );
  return (
    <div className="customization-roles-table">
      <p className="customization-roles-table-hint">
        {isLoading ? "Loading reference table..." : `Select 1–${max} characteristics for the reference table (${selected.length}/${max}).`}
      </p>
      <div className="customization-roles-table-options">
        <div className="customization-roles-table-row">{active.map((entry) => renderOption(entry, true))}</div>
        <div className="customization-roles-table-row">{inactive.map((entry) => renderOption(entry, false))}</div>
      </div>
      <div className="customization-rule-editor-actions">
        <button className="customization-rule-cancel" disabled={isSaving} onClick={onCancel} type="button">Cancel</button>
        <button className="customization-rule-save" disabled={isLoading || isSaving || selected.length < 1 || selected.length > max} onClick={onSave} type="button">Save</button>
      </div>
      {error ? <p className="customization-rule-error" role="status">{error}</p> : null}
    </div>
  );
}
