import { useEffect, useState } from "react";
import { CatalogList } from "./catalog";
import { serverEndpoint } from "../platform";
import { fetchCustomizationSkills, type CustomizationCatalogItem } from "./rules";

export function SkillsList({ items, isLoading, query, onChanged, onBusy }: {
  items: CustomizationCatalogItem[];
  isLoading: boolean;
  query: string;
  onChanged: (items: CustomizationCatalogItem[]) => void;
  onBusy: (busy: boolean) => void;
}) {
  const [editing, setEditing] = useState<CustomizationCatalogItem | null>(null);
  const [content, setContent] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  useEffect(() => {
    onBusy(pending || editing !== null);
    return () => onBusy(false);
  }, [pending, editing, onBusy]);

  async function request(item: CustomizationCatalogItem, method: string, body?: object) {
    const response = await fetch(await serverEndpoint(`/__solomon/skills?id=${encodeURIComponent(item.id)}`), {
      method,
      headers: { "Content-Type": "application/json" },
      body: body ? JSON.stringify(body) : undefined,
      cache: "no-store",
    });
    const payload = await response.json();
    if (!response.ok) throw new Error(payload.error || "Could not update this skill.");
    return payload;
  }

  async function edit(item: CustomizationCatalogItem) {
    if (pending) return;
    setPending(true);
    setError("");
    try {
      const payload = await request(item, "GET");
      if (typeof payload.content !== "string") {
        throw new Error("The daemon did not return the skill Markdown. Update the daemon to use skill editing.");
      }
      setContent(payload.content);
      setEditing(item);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Could not load this skill.");
    } finally { setPending(false); }
  }

  async function mutate(item: CustomizationCatalogItem, method: "PATCH" | "DELETE") {
    if (pending) return;
    setPending(true);
    setError("");
    try {
      await request(item, method, method === "PATCH" ? { content } : undefined);
      onChanged(await fetchCustomizationSkills());
      setEditing(null);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Could not update this skill.");
    } finally { setPending(false); }
  }

  return <>
    {editing ? <form className="customization-rule-editor" aria-label={`Edit skill ${editing.title}`} onSubmit={(event) => { event.preventDefault(); void mutate(editing, "PATCH"); }}>
      <h2>Edit {editing.title}</h2>
      <textarea aria-label="Skill Markdown" autoFocus disabled={pending} value={content} onChange={(event) => setContent(event.target.value)} />
      <div className="customization-rule-editor-actions">
        <button className="customization-rule-cancel" disabled={pending} onClick={() => { setEditing(null); setError(""); }} type="button">Cancel</button>
        <button className="customization-rule-save" disabled={pending || !content.trim()} type="submit">{pending ? "Saving..." : "Save"}</button>
      </div>
    </form> : <CatalogList emptyLabel="No global Solomon skills configured yet." isLoading={isLoading} items={items} kind="skill" query={query} isSaving={pending} onEdit={(item) => void edit(item)} onRemove={(item) => void mutate(item, "DELETE")} />}
    {error ? <p className="customization-rule-error" role="alert">{error}</p> : null}
  </>;
}
