import { useEffect, useState, type RefObject } from "react";
import {
  fetchRolesTable,
  saveRolesTable,
  type CustomizationCatalogItem,
  type CustomizationRule,
  type RolesTableCharacteristic,
  type SubagentScore,
} from "./rules";

type DropIndicator = {
  position: "before" | "after";
  ruleId: number;
};

export function RulesList({
  canReorder,
  draggedRuleId,
  dropIndicator,
  editError,
  editingId,
  editingText,
  editorRef,
  isDeleting,
  isLoading,
  isSaving,
  items,
  menuId,
  menuRef,
  onCancelEdit,
  onDelete,
  onDoubleClickEdit,
  onDragEnd,
  onDragOver,
  onDragStart,
  onDrop,
  onEditingTextChange,
  onMenuToggle,
  onSaveEdit,
  onStartEdit,
  orderError,
  query,
}: {
  canReorder: boolean;
  draggedRuleId: number | null;
  dropIndicator: DropIndicator | null;
  editError: string;
  editingId: number | null;
  editingText: string;
  editorRef: RefObject<HTMLTextAreaElement | null>;
  isDeleting: boolean;
  isLoading: boolean;
  isSaving: boolean;
  items: CustomizationRule[];
  menuId: number | null;
  menuRef: RefObject<HTMLDivElement | null>;
  onCancelEdit: () => void;
  onDelete: (ruleId: number) => void;
  onDoubleClickEdit: (rule: CustomizationRule) => void;
  onDragEnd: () => void;
  onDragOver: (ruleId: number, clientY: number, element: HTMLElement) => void;
  onDragStart: (ruleId: number) => void;
  onDrop: (ruleId: number) => void;
  onEditingTextChange: (value: string) => void;
  onMenuToggle: (ruleId: number | null) => void;
  onSaveEdit: () => void;
  onStartEdit: (rule: CustomizationRule) => void;
  orderError: string;
  query: string;
}) {
  return (
    <div className="customization-rules">
      {items.map((rule) => {
        if (editingId === rule.id) {
          return (
            <div className="customization-rule-editor" key={rule.id}>
              <textarea
                aria-label={`Edit rule ${rule.id}`}
                onChange={(event) => onEditingTextChange(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Escape") onCancelEdit();
                }}
                ref={editorRef}
                value={editingText}
              />
              <div className="customization-rule-editor-actions">
                <button className="customization-rule-cancel" disabled={isSaving} onClick={onCancelEdit} type="button">Cancel</button>
                <button className="customization-rule-save" disabled={isSaving || !editingText.trim()} onClick={onSaveEdit} type="button">Save</button>
              </div>
              {editError ? <p className="customization-rule-error" role="status">{editError}</p> : null}
            </div>
          );
        }

        const dropClass = dropIndicator?.ruleId === rule.id
          ? dropIndicator.position === "before" ? " drop-before" : " drop-after"
          : "";
        return (
          <div
            className={`customization-rule${draggedRuleId === rule.id ? " is-dragging" : ""}${dropClass}`}
            draggable={canReorder}
            key={rule.id}
            onDoubleClick={() => onDoubleClickEdit(rule)}
            onDragEnd={onDragEnd}
            onDragOver={(event) => {
              event.preventDefault();
              event.dataTransfer.dropEffect = "move";
              onDragOver(rule.id, event.clientY, event.currentTarget);
            }}
            onDragStart={(event) => {
              event.dataTransfer.effectAllowed = "move";
              onDragStart(rule.id);
            }}
            onDrop={(event) => {
              event.preventDefault();
              onDrop(rule.id);
            }}
          >
            <button className="customization-rule-main" type="button">
              <span aria-hidden="true" className="customization-rule-icon"><RuleIcon /></span>
              <span>{rule.text}</span>
            </button>
            <div className="customization-rule-more-wrap" onDoubleClick={(event) => event.stopPropagation()} ref={menuId === rule.id ? menuRef : undefined}>
              <button
                aria-expanded={menuId === rule.id}
                aria-haspopup="menu"
                aria-label={`More options for rule ${rule.id}`}
                className="customization-rule-more"
                onClick={(event) => {
                  event.stopPropagation();
                  onMenuToggle(menuId === rule.id ? null : rule.id);
                }}
                type="button"
              >
                <MoreIcon />
              </button>
              {menuId === rule.id ? (
                <div className="customization-rule-menu" role="menu">
                  <button onClick={() => onStartEdit(rule)} role="menuitem" type="button">Edit</button>
                  <button className="is-danger" disabled={isDeleting} onClick={() => onDelete(rule.id)} role="menuitem" type="button">Delete</button>
                </div>
              ) : null}
            </div>
          </div>
        );
      })}
      {!items.length ? (
        <p className="customization-empty">
          {isLoading ? "Loading rules..." : query ? "No rules match this search." : "No global Solomon rules configured yet."}
        </p>
      ) : null}
      {orderError ? <p className="customization-rule-error" role="status">{orderError}</p> : null}
    </div>
  );
}

export function CatalogList({
  emptyLabel,
  isLoading,
  items,
  kind,
  query,
}: {
  emptyLabel: string;
  isLoading: boolean;
  items: CustomizationCatalogItem[];
  kind: string;
  query: string;
}) {
  return (
    <div className="customization-rules">
      {items.map((item) => (
        <div className="customization-rule" key={item.id}>
          <button className="customization-rule-main" type="button">
            <span aria-hidden="true" className="customization-rule-icon"><RuleIcon /></span>
            <span className="customization-catalog-text">
              <CatalogItemHeading item={item} />
              {item.detail ? <span className="customization-catalog-detail">{item.detail}</span> : null}
            </span>
          </button>
          <button aria-label={`More options for ${kind} ${item.title}`} className="customization-rule-more" type="button">
            <MoreIcon />
          </button>
        </div>
      ))}
      {!items.length ? (
        <p className="customization-empty">
          {isLoading ? `Loading ${kind}s...` : query ? `No ${kind}s match this search.` : emptyLabel}
        </p>
      ) : null}
    </div>
  );
}

export function EditableCatalogList({
  editError,
  editingId,
  editingScores,
  editingText,
  emptyLabel,
  isDeleting,
  isLoading,
  isSaving,
  items,
  kind,
  menuId,
  menuRef,
  onCancelEdit,
  onDelete,
  onEditingScoreChange,
  onEditingTextChange,
  onMenuToggle,
  onSaveEdit,
  onStartEdit,
  query,
}: {
  editError: string;
  editingId: string | null;
  editingScores: SubagentScore[];
  editingText: string;
  emptyLabel: string;
  isDeleting: boolean;
  isLoading: boolean;
  isSaving: boolean;
  items: CustomizationCatalogItem[];
  kind: string;
  menuId: string | null;
  menuRef: RefObject<HTMLDivElement | null>;
  onCancelEdit: () => void;
  onDelete: (item: CustomizationCatalogItem) => void;
  onEditingScoreChange: (id: string, value: number) => void;
  onEditingTextChange: (value: string) => void;
  onMenuToggle: (id: string | null) => void;
  onSaveEdit: () => void;
  onStartEdit: (item: CustomizationCatalogItem) => void;
  query: string;
}) {
  return (
    <div className="customization-rules">
      {items.map((item) => {
        if (editingId === item.id) {
          return (
            <div className="customization-rule-editor" key={item.id}>
              <CatalogItemHeading item={item} />
              <textarea
                aria-label={`Edit ${kind} ${item.title}`}
                onChange={(event) => onEditingTextChange(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Escape") onCancelEdit();
                }}
                value={editingText}
              />
              {editingScores.length ? (
                <div className="customization-subagent-scores">
                  {editingScores.map((score) => (
                    <label className="customization-subagent-score" key={score.id}>
                      <span>{score.label}</span>
                      <input
                        disabled={isSaving}
                        inputMode="numeric"
                        max={100}
                        min={0}
                        onChange={(event) => onEditingScoreChange(score.id, Number(event.target.value))}
                        type="number"
                        value={score.value}
                      />
                    </label>
                  ))}
                </div>
              ) : null}
              <div className="customization-rule-editor-actions">
                <button className="customization-rule-cancel" disabled={isSaving} onClick={onCancelEdit} type="button">Cancel</button>
                <button className="customization-rule-save" disabled={isSaving} onClick={onSaveEdit} type="button">Save</button>
              </div>
              {editError ? <p className="customization-rule-error" role="status">{editError}</p> : null}
            </div>
          );
        }

        return (
          <div className="customization-rule" key={item.id} onDoubleClick={() => onStartEdit(item)}>
            <button className="customization-rule-main" type="button">
              <span aria-hidden="true" className="customization-rule-icon"><RuleIcon /></span>
              <span className="customization-catalog-text">
                <CatalogItemHeading item={item} />
                {item.detail ? <span className="customization-catalog-detail">{item.detail}</span> : null}
              </span>
            </button>
            <div className="customization-rule-more-wrap" onDoubleClick={(event) => event.stopPropagation()} ref={menuId === item.id ? menuRef : undefined}>
              <button
                aria-expanded={menuId === item.id}
                aria-haspopup="menu"
                aria-label={`More options for ${kind} ${item.title}`}
                className="customization-rule-more"
                onClick={(event) => {
                  event.stopPropagation();
                  onMenuToggle(menuId === item.id ? null : item.id);
                }}
                type="button"
              >
                <MoreIcon />
              </button>
              {menuId === item.id ? (
                <div className="customization-rule-menu" role="menu">
                  <button onClick={() => onStartEdit(item)} role="menuitem" type="button">Edit</button>
                  <button className="is-danger" disabled={isDeleting} onClick={() => onDelete(item)} role="menuitem" type="button">Delete</button>
                </div>
              ) : null}
            </div>
          </div>
        );
      })}
      {!items.length ? (
        <p className="customization-empty">
          {isLoading ? `Loading ${kind}s...` : query ? `No ${kind}s match this search.` : emptyLabel}
        </p>
      ) : null}
    </div>
  );
}

export function catalogMatches(item: CustomizationCatalogItem, query: string): boolean {
  if (!query) return true;
  return item.title.toLocaleLowerCase().includes(query)
    || item.detail.toLocaleLowerCase().includes(query)
    || (item.badge ?? "").toLocaleLowerCase().includes(query);
}

function CatalogItemHeading({ item }: { item: CustomizationCatalogItem }) {
  const badge = (item.badge ?? "").trim();
  return (
    <span className="customization-catalog-heading">
      <span className="customization-catalog-title">{item.title}</span>
      {badge ? <span className="customization-catalog-badge">{badge}</span> : null}
    </span>
  );
}

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

export function SearchIcon() {
  return (<svg aria-hidden="true" viewBox="0 0 24 24"><circle cx="11" cy="11" r="6.5" /><path d="m16 16 4 4" /></svg>);
}

export function RuleIcon() {
  return (<svg aria-hidden="true" viewBox="0 0 24 24"><path d="M9 6h10M9 12h10M9 18h10M5 6h.01M5 12h.01M5 18h.01" /></svg>);
}

export function MoreIcon() {
  return (<svg aria-hidden="true" viewBox="0 0 24 24"><circle cx="5" cy="12" r="1" /><circle cx="12" cy="12" r="1" /><circle cx="19" cy="12" r="1" /></svg>);
}
