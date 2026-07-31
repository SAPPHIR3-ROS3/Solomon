import { type CSSProperties, useEffect, useRef, useState } from "react";
import { CatalogList, EditableCatalogList, RulesList, SearchIcon, catalogMatches, useRolesTableEditor } from "./catalog";
import { SystemPromptsPanel } from "./prompts";
import {
  deleteCustomizationRule,
  deleteCustomizationSubagent,
  fetchCustomizationMcps,
  fetchCustomizationRules,
  fetchCustomizationSkills,
  fetchCustomizationSubagents,
  reorderCustomizationRules,
  sameCustomizationCatalog,
  sameCustomizationRules,
  updateCustomizationRule,
  updateCustomizationSubagent,
  type CustomizationCatalogItem,
  type CustomizationRule,
  type SubagentScore,
} from "./rules";

const filters = ["System Prompts", "Rules", "Global AGENTS.md", "MCPs", "Skills", "Subagents"] as const;
const CATALOG_POLL_MS = 1000;
const FILTERS_FADE_DISTANCE = 12;
type DropIndicator = {
  position: "before" | "after";
  ruleId: number;
};

export function CustomizationPage() {
  const [activeFilter, setActiveFilter] = useState<(typeof filters)[number]>("System Prompts");
  const [query, setQuery] = useState("");
  const [rules, setRules] = useState<CustomizationRule[]>([]);
  const [skills, setSkills] = useState<CustomizationCatalogItem[]>([]);
  const [mcps, setMcps] = useState<CustomizationCatalogItem[]>([]);
  const [subagents, setSubagents] = useState<CustomizationCatalogItem[]>([]);
  const [isLoadingRules, setIsLoadingRules] = useState(true);
  const [isLoadingCatalog, setIsLoadingCatalog] = useState(true);
  const [isSavingRuleOrder, setIsSavingRuleOrder] = useState(false);
  const [isSavingRuleText, setIsSavingRuleText] = useState(false);
  const [editingRuleId, setEditingRuleId] = useState<number | null>(null);
  const [editingRuleText, setEditingRuleText] = useState("");
  const [draggedRuleId, setDraggedRuleId] = useState<number | null>(null);
  const [dropIndicator, setDropIndicator] = useState<DropIndicator | null>(null);
  const [ruleOrderError, setRuleOrderError] = useState("");
  const [ruleEditError, setRuleEditError] = useState("");
  const [menuRuleId, setMenuRuleId] = useState<number | null>(null);
  const [isDeletingRule, setIsDeletingRule] = useState(false);
  const [editingSubagentId, setEditingSubagentId] = useState<string | null>(null);
  const [editingSubagentText, setEditingSubagentText] = useState("");
  const [editingSubagentScores, setEditingSubagentScores] = useState<SubagentScore[]>([]);
  const [menuSubagentId, setMenuSubagentId] = useState<string | null>(null);
  const [isSavingSubagent, setIsSavingSubagent] = useState(false);
  const [isDeletingSubagent, setIsDeletingSubagent] = useState(false);
  const [subagentEditError, setSubagentEditError] = useState("");
  const interactionLock = useRef(false);
  const editorRef = useRef<HTMLTextAreaElement | null>(null);
  const menuRef = useRef<HTMLDivElement | null>(null);
  const filtersScrollRef = useRef<HTMLDivElement | null>(null);
  const [filtersLeftFade, setFiltersLeftFade] = useState(0);
  const [filtersRightFade, setFiltersRightFade] = useState(0);
  const rolesTable = useRolesTableEditor(activeFilter === "Subagents");
  const normalizedQuery = query.trim().toLocaleLowerCase();
  const visibleRules = rules.filter((rule) => rule.text.toLocaleLowerCase().includes(normalizedQuery));
  const visibleSkills = skills.filter((item) => catalogMatches(item, normalizedQuery));
  const visibleMcps = mcps.filter((item) => catalogMatches(item, normalizedQuery));
  const visibleSubagents = subagents.filter((item) => catalogMatches(item, normalizedQuery));
  const canReorderRules = !isSavingRuleOrder && !query.trim() && editingRuleId === null && !isDeletingRule;
  interactionLock.current = draggedRuleId !== null || isSavingRuleOrder || editingRuleId !== null || isSavingRuleText || isDeletingRule || editingSubagentId !== null || isSavingSubagent || isDeletingSubagent || rolesTable.isBusy;

  useEffect(() => {
    const scrollport = filtersScrollRef.current;
    if (!scrollport) return;
    const updateFades = () => {
      const scrollableWidth = scrollport.scrollWidth - scrollport.clientWidth;
      setFiltersLeftFade(Math.min(1, scrollport.scrollLeft / FILTERS_FADE_DISTANCE));
      setFiltersRightFade(Math.min(1, Math.max(0, scrollableWidth - scrollport.scrollLeft) / FILTERS_FADE_DISTANCE));
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
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    void fetchCustomizationRules(controller.signal)
      .then(setRules)
      .catch((error: unknown) => {
        if (controller.signal.aborted || (error instanceof DOMException && error.name === "AbortError")) return;
        setRules([]);
      })
      .finally(() => {
        if (!controller.signal.aborted) setIsLoadingRules(false);
      });
    void Promise.all([
      fetchCustomizationSkills(controller.signal),
      fetchCustomizationMcps(controller.signal),
      fetchCustomizationSubagents(controller.signal),
    ])
      .then(([nextSkills, nextMcps, nextSubagents]) => {
        setSkills(nextSkills);
        setMcps(nextMcps);
        setSubagents(nextSubagents);
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted || (error instanceof DOMException && error.name === "AbortError")) return;
        setSkills([]);
        setMcps([]);
        setSubagents([]);
      })
      .finally(() => {
        if (!controller.signal.aborted) setIsLoadingCatalog(false);
      });
    return () => controller.abort();
  }, []);

  useEffect(() => {
    if (activeFilter !== "Rules" && activeFilter !== "Skills" && activeFilter !== "MCPs" && activeFilter !== "Subagents") return;
    let cancelled = false;
    const refresh = async () => {
      if (interactionLock.current) return;
      try {
        if (activeFilter === "Rules") {
          const nextRules = await fetchCustomizationRules();
          if (cancelled || interactionLock.current) return;
          setRules((current) => (sameCustomizationRules(current, nextRules) ? current : nextRules));
          return;
        }
        if (activeFilter === "Skills") {
          const nextSkills = await fetchCustomizationSkills();
          if (cancelled || interactionLock.current) return;
          setSkills((current) => (sameCustomizationCatalog(current, nextSkills) ? current : nextSkills));
          return;
        }
        if (activeFilter === "MCPs") {
          const nextMcps = await fetchCustomizationMcps();
          if (cancelled || interactionLock.current) return;
          setMcps((current) => (sameCustomizationCatalog(current, nextMcps) ? current : nextMcps));
          return;
        }
        const nextSubagents = await fetchCustomizationSubagents();
        if (cancelled || interactionLock.current) return;
        setSubagents((current) => (sameCustomizationCatalog(current, nextSubagents) ? current : nextSubagents));
      } catch {
        // Keep the last successful list while another client is mid-write.
      }
    };
    const timer = window.setInterval(() => {
      void refresh();
    }, CATALOG_POLL_MS);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [activeFilter]);

  useEffect(() => {
    if (editingRuleId === null) return;
    editorRef.current?.focus();
    const value = editorRef.current?.value ?? "";
    editorRef.current?.setSelectionRange(value.length, value.length);
  }, [editingRuleId]);

  useEffect(() => {
    if (menuRuleId === null && menuSubagentId === null) return;
    const onPointerDown = (event: PointerEvent) => {
      if (menuRef.current?.contains(event.target as Node)) return;
      setMenuRuleId(null);
      setMenuSubagentId(null);
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setMenuRuleId(null);
        setMenuSubagentId(null);
      }
    };
    window.addEventListener("pointerdown", onPointerDown);
    window.addEventListener("keydown", onKeyDown);
    return () => {
      window.removeEventListener("pointerdown", onPointerDown);
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [menuRuleId, menuSubagentId]);

  async function moveRule(draggedId: number, targetId: number, position: "before" | "after") {
    if (draggedId === targetId || isSavingRuleOrder || editingRuleId !== null) return;
    const originalRules = rules;
    const fromIndex = rules.findIndex((rule) => rule.id === draggedId);
    const targetIndex = rules.findIndex((rule) => rule.id === targetId);
    if (fromIndex < 0 || targetIndex < 0) return;

    let insertIndex = position === "after" ? targetIndex + 1 : targetIndex;
    const nextRules = [...rules];
    const [draggedRule] = nextRules.splice(fromIndex, 1);
    if (fromIndex < insertIndex) insertIndex -= 1;
    nextRules.splice(insertIndex, 0, draggedRule);
    if (sameCustomizationRules(originalRules, nextRules)) return;

    setRules(nextRules);
    setIsSavingRuleOrder(true);
    setRuleOrderError("");
    try {
      setRules(await reorderCustomizationRules(nextRules.map((rule) => rule.id)));
    } catch {
      setRules(originalRules);
      setRuleOrderError("Could not save the new rule order.");
    } finally {
      setIsSavingRuleOrder(false);
    }
  }

  function startEditingRule(rule: CustomizationRule) {
    setMenuRuleId(null);
    setEditingRuleId(rule.id);
    setEditingRuleText(rule.text);
    setRuleEditError("");
    setDraggedRuleId(null);
    setDropIndicator(null);
  }

  function cancelEditingRule() {
    setEditingRuleId(null);
    setEditingRuleText("");
    setRuleEditError("");
  }

  async function saveEditingRule() {
    if (editingRuleId === null || isSavingRuleText) return;
    setIsSavingRuleText(true);
    setRuleEditError("");
    try {
      setRules(await updateCustomizationRule(editingRuleId, editingRuleText));
      cancelEditingRule();
    } catch {
      setRuleEditError("Could not save this rule.");
    } finally {
      setIsSavingRuleText(false);
    }
  }

  async function deleteRule(ruleId: number) {
    if (isDeletingRule) return;
    setMenuRuleId(null);
    setIsDeletingRule(true);
    setRuleOrderError("");
    try {
      if (editingRuleId === ruleId) cancelEditingRule();
      setRules(await deleteCustomizationRule(ruleId));
    } catch {
      setRuleOrderError("Could not delete this rule.");
    } finally {
      setIsDeletingRule(false);
    }
  }

  function startEditingSubagent(item: CustomizationCatalogItem) {
    setMenuSubagentId(null);
    setEditingSubagentId(item.id);
    setEditingSubagentText(item.detail);
    setEditingSubagentScores((item.scores ?? []).map((score) => ({ ...score })));
    setSubagentEditError("");
  }

  function cancelEditingSubagent() {
    setEditingSubagentId(null);
    setEditingSubagentText("");
    setEditingSubagentScores([]);
    setSubagentEditError("");
  }

  async function saveEditingSubagent() {
    if (editingSubagentId === null || isSavingSubagent) return;
    setIsSavingSubagent(true);
    setSubagentEditError("");
    try {
      setSubagents(await updateCustomizationSubagent(
        editingSubagentId,
        editingSubagentText,
        editingSubagentScores.map((score) => ({ id: score.id, value: score.value })),
      ));
      cancelEditingSubagent();
    } catch {
      setSubagentEditError("Could not save this subagent.");
    } finally {
      setIsSavingSubagent(false);
    }
  }

  async function deleteSubagent(item: CustomizationCatalogItem) {
    if (isDeletingSubagent) return;
    setMenuSubagentId(null);
    setIsDeletingSubagent(true);
    setSubagentEditError("");
    try {
      if (editingSubagentId === item.id) cancelEditingSubagent();
      setSubagents(await deleteCustomizationSubagent(item.id));
    } catch {
      setSubagentEditError("Could not delete this subagent.");
    } finally {
      setIsDeletingSubagent(false);
    }
  }

  function clearDragState() {
    setDraggedRuleId(null);
    setDropIndicator(null);
  }

  function updateDropIndicator(ruleId: number, clientY: number, element: HTMLElement) {
    if (draggedRuleId === null || draggedRuleId === ruleId) {
      setDropIndicator(null);
      return;
    }
    const bounds = element.getBoundingClientRect();
    const position = clientY < bounds.top + bounds.height / 2 ? "before" : "after";
    setDropIndicator((current) => (current?.ruleId === ruleId && current.position === position ? current : { ruleId, position }));
  }

  const searchPlaceholder = ({
    "Global AGENTS.md": "Search...",
    MCPs: "Search MCPs...",
    Rules: "Search rules...",
    Skills: "Search skills...",
    "System Prompts": "Search system prompts...",
    Subagents: "Search subagents...",
  } as const)[activeFilter];

  const activeCount = ({
    "Global AGENTS.md": null,
    MCPs: mcps.length,
    Rules: rules.length,
    Skills: skills.length,
    "System Prompts": null,
    Subagents: subagents.length,
  } as const)[activeFilter];

  return (
    <section aria-label="Customization" className="customization-screen">
      <div className="customization-content">
        <div className="customization-toolbar">
          <label className="customization-search">
            <SearchIcon />
            <input
              onChange={(event) => setQuery(event.target.value)}
              placeholder={searchPlaceholder}
              type="search"
              value={query}
            />
          </label>
        </div>

        <div
          className="customization-filters-shell"
          style={{ "--customization-filters-left-fade": filtersLeftFade, "--customization-filters-right-fade": filtersRightFade } as CSSProperties}
        >
          <div className="customization-filters-scrollport" ref={filtersScrollRef}>
            <nav aria-label="Customization sections" className="customization-filters" role="tablist">
              {filters.map((filter) => (
                <button
                  aria-selected={activeFilter === filter}
                  className={`customization-filter${activeFilter === filter ? " is-active" : ""}`}
                  key={filter}
                  onClick={(event) => {
                    setActiveFilter(filter);
                    event.currentTarget.scrollIntoView({ behavior: "smooth", block: "nearest", inline: "nearest" });
                  }}
                  role="tab"
                  type="button"
                >
                  {filter}
                </button>
              ))}
            </nav>
          </div>
        </div>

        {activeFilter !== "System Prompts" ? (
          <div className="customization-list-head">
            <div className="customization-list-head-title">
              <h1>
                {activeFilter}
                {activeCount !== null ? <span className="customization-list-count">{activeCount}</span> : null}
              </h1>
              {rolesTable.button}
            </div>
            {activeFilter === "Rules" ? <button className="customization-new" type="button">+ New</button> : null}
          </div>
        ) : null}
        {rolesTable.panel}
        {activeFilter === "System Prompts" ? <SystemPromptsPanel query={query} /> : null}
        {activeFilter === "Rules" ? (
          <RulesList
            canReorder={canReorderRules}
            draggedRuleId={draggedRuleId}
            dropIndicator={dropIndicator}
            editError={ruleEditError}
            editingId={editingRuleId}
            editingText={editingRuleText}
            editorRef={editorRef}
            isDeleting={isDeletingRule}
            isLoading={isLoadingRules}
            isSaving={isSavingRuleText}
            items={visibleRules}
            menuId={menuRuleId}
            menuRef={menuRef}
            onCancelEdit={cancelEditingRule}
            onDelete={(ruleId) => void deleteRule(ruleId)}
            onDoubleClickEdit={startEditingRule}
            onDragEnd={clearDragState}
            onDragOver={updateDropIndicator}
            onDragStart={(ruleId) => {
              setDraggedRuleId(ruleId);
              setDropIndicator(null);
            }}
            onDrop={(ruleId) => {
              const position = dropIndicator?.ruleId === ruleId ? dropIndicator.position : "before";
              if (draggedRuleId !== null) void moveRule(draggedRuleId, ruleId, position);
              clearDragState();
            }}
            onEditingTextChange={setEditingRuleText}
            onMenuToggle={setMenuRuleId}
            onSaveEdit={() => void saveEditingRule()}
            onStartEdit={startEditingRule}
            orderError={ruleOrderError}
            query={query}
          />
        ) : null}
        {activeFilter === "Skills" ? (
          <CatalogList
            emptyLabel="No global Solomon skills configured yet."
            isLoading={isLoadingCatalog}
            items={visibleSkills}
            kind="skill"
            query={query}
          />
        ) : null}
        {activeFilter === "MCPs" ? (
          <CatalogList
            emptyLabel="No MCP servers configured yet."
            isLoading={isLoadingCatalog}
            items={visibleMcps}
            kind="MCP"
            query={query}
          />
        ) : null}
        {activeFilter === "Subagents" ? (
          <EditableCatalogList
            editError={subagentEditError}
            editingId={editingSubagentId}
            editingScores={editingSubagentScores}
            editingText={editingSubagentText}
            emptyLabel="No subagent roles configured yet."
            isDeleting={isDeletingSubagent}
            isLoading={isLoadingCatalog}
            isSaving={isSavingSubagent}
            items={visibleSubagents}
            kind="subagent"
            menuId={menuSubagentId}
            menuRef={menuRef}
            onCancelEdit={cancelEditingSubagent}
            onDelete={(item) => void deleteSubagent(item)}
            onEditingScoreChange={(id, value) => {
              setEditingSubagentScores((current) => current.map((score) => (score.id === id ? { ...score, value } : score)));
            }}
            onEditingTextChange={setEditingSubagentText}
            onMenuToggle={setMenuSubagentId}
            onSaveEdit={() => void saveEditingSubagent()}
            onStartEdit={startEditingSubagent}
            query={query}
          />
        ) : null}
        {activeFilter === "Global AGENTS.md" ? (
          <p className="customization-empty">No {activeFilter.toLocaleLowerCase()} configured yet.</p>
        ) : null}
      </div>
    </section>
  );
}
