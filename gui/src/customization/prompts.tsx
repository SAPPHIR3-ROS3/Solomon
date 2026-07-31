import { type CSSProperties, useEffect, useRef, useState } from "react";
import {
  fetchCustomizationPromptTemplate,
  fetchCustomizationPromptTemplates,
  resetCustomizationPromptTemplate,
  sameCustomizationCatalog,
  updateCustomizationPromptTemplate,
  type CustomizationCatalogItem,
  type PromptTemplate,
} from "./rules";
import { catalogMatches } from "./catalog";

const FILTERS_FADE_DISTANCE = 12;

export function SystemPromptsPanel({ query }: { query: string }) {
  const [items, setItems] = useState<CustomizationCatalogItem[]>([]);
  const [selectedId, setSelectedId] = useState<string>("");
  const [template, setTemplate] = useState<PromptTemplate | null>(null);
  const [isLoadingList, setIsLoadingList] = useState(true);
  const [isLoadingContent, setIsLoadingContent] = useState(false);
  const [isEditing, setIsEditing] = useState(false);
  const [draft, setDraft] = useState("");
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState("");
  const [leftFade, setLeftFade] = useState(0);
  const [rightFade, setRightFade] = useState(0);
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const editorRef = useRef<HTMLTextAreaElement | null>(null);
  const normalizedQuery = query.trim().toLocaleLowerCase();
  const visibleItems = items.filter((item) => catalogMatches(item, normalizedQuery));
  const activeId = visibleItems.some((item) => item.id === selectedId) ? selectedId : (visibleItems[0]?.id ?? "");

  useEffect(() => {
    const controller = new AbortController();
    void fetchCustomizationPromptTemplates(controller.signal)
      .then((next) => {
        setItems(next);
        setSelectedId((current) => current || next[0]?.id || "");
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted || (error instanceof DOMException && error.name === "AbortError")) return;
        setItems([]);
      })
      .finally(() => {
        if (!controller.signal.aborted) setIsLoadingList(false);
      });
    return () => controller.abort();
  }, []);

  useEffect(() => {
    if (!activeId) {
      setTemplate(null);
      return;
    }
    const controller = new AbortController();
    setIsLoadingContent(true);
    setError("");
    setIsEditing(false);
    void fetchCustomizationPromptTemplate(activeId, controller.signal)
      .then((next) => {
        setTemplate(next);
        setDraft(next.content);
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted || (error instanceof DOMException && error.name === "AbortError")) return;
        setTemplate(null);
        setError("Could not load this system prompt.");
      })
      .finally(() => {
        if (!controller.signal.aborted) setIsLoadingContent(false);
      });
    return () => controller.abort();
  }, [activeId]);

  useEffect(() => {
    if (activeId && selectedId !== activeId) setSelectedId(activeId);
  }, [activeId, selectedId]);

  useEffect(() => {
    const scrollport = scrollRef.current;
    if (!scrollport) return;
    const updateFades = () => {
      const scrollableWidth = scrollport.scrollWidth - scrollport.clientWidth;
      setLeftFade(Math.min(1, scrollport.scrollLeft / FILTERS_FADE_DISTANCE));
      setRightFade(Math.min(1, Math.max(0, scrollableWidth - scrollport.scrollLeft) / FILTERS_FADE_DISTANCE));
    };
    const resizeObserver = new ResizeObserver(updateFades);
    resizeObserver.observe(scrollport);
    if (scrollport.firstElementChild) resizeObserver.observe(scrollport.firstElementChild);
    scrollport.addEventListener("scroll", updateFades, { passive: true });
    updateFades();
    return () => {
      resizeObserver.disconnect();
      scrollport.removeEventListener("scroll", updateFades);
    };
  }, [visibleItems.length]);

  useEffect(() => {
    if (!isEditing) return;
    editorRef.current?.focus();
  }, [isEditing]);

  async function refreshList() {
    const next = await fetchCustomizationPromptTemplates();
    setItems((current) => (sameCustomizationCatalog(current, next) ? current : next));
  }

  async function save() {
    if (!template || isSaving) return;
    setIsSaving(true);
    setError("");
    try {
      const next = await updateCustomizationPromptTemplate(template.id, draft);
      setTemplate(next);
      setDraft(next.content);
      setIsEditing(false);
      await refreshList();
    } catch {
      setError("Could not save this system prompt.");
    } finally {
      setIsSaving(false);
    }
  }

  async function resetToDefault() {
    if (!template || isSaving || isEditing) return;
    setIsSaving(true);
    setError("");
    try {
      const next = await resetCustomizationPromptTemplate(template.id);
      setTemplate(next);
      setDraft(next.content);
      await refreshList();
    } catch {
      setError("Could not reset this system prompt.");
    } finally {
      setIsSaving(false);
    }
  }

  function cancelEdit() {
    setDraft(template?.content ?? "");
    setIsEditing(false);
    setError("");
  }

  return (
    <>
      <div className="customization-list-head">
        <div className="customization-list-head-title">
          <h1>
            System Prompts
            <span className="customization-list-count">{items.length}</span>
          </h1>
        </div>
        {template && !isLoadingContent ? (
          isEditing ? (
            <div className="customization-rule-editor-actions">
              <button className="customization-rule-cancel" disabled={isSaving} onClick={cancelEdit} type="button">Cancel</button>
              <button className="customization-rule-save" disabled={isSaving || draft === template.content} onClick={() => void save()} type="button">Save</button>
            </div>
          ) : (
            <div className="customization-rule-editor-actions">
              <button className="customization-rule-cancel" disabled={isSaving || !template.modified} onClick={() => void resetToDefault()} type="button">Reset to default</button>
              <button className="customization-new" disabled={isSaving} onClick={() => setIsEditing(true)} type="button">Edit</button>
            </div>
          )
        ) : null}
      </div>

      <div
        className="customization-filters-shell customization-prompt-picker-shell"
        style={{ "--customization-filters-left-fade": leftFade, "--customization-filters-right-fade": rightFade } as CSSProperties}
      >
        <div className="customization-filters-scrollport" ref={scrollRef}>
          <nav aria-label="System prompt templates" className="customization-filters" role="tablist">
            {visibleItems.map((item) => (
              <button
                aria-selected={item.id === activeId}
                className={`customization-filter${item.id === activeId ? " is-active" : ""}`}
                disabled={isEditing || isSaving}
                key={item.id}
                onClick={(event) => {
                  setSelectedId(item.id);
                  event.currentTarget.scrollIntoView({ behavior: "smooth", block: "nearest", inline: "nearest" });
                }}
                role="tab"
                type="button"
              >
                {item.title}
                {item.badge ? <span className="customization-catalog-badge">{item.badge}</span> : null}
              </button>
            ))}
          </nav>
        </div>
      </div>

      {isLoadingList || isLoadingContent ? (
        <p className="customization-empty">Loading system prompts...</p>
      ) : !visibleItems.length ? (
        <p className="customization-empty">{query ? "No system prompts match this search." : "No system prompt templates found."}</p>
      ) : isEditing ? (
        <div className="customization-prompt-editor">
          <textarea
            aria-label={`Edit ${template?.title ?? "system prompt"}`}
            onChange={(event) => setDraft(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Escape") cancelEdit();
            }}
            ref={editorRef}
            value={draft}
          />
        </div>
      ) : (
        <pre className="customization-prompt-content">{template?.content ?? ""}</pre>
      )}
      {error ? <p className="customization-rule-error" role="status">{error}</p> : null}
    </>
  );
}
