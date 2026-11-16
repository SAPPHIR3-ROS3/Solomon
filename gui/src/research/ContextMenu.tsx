import { useEffect, useLayoutEffect, useRef, useState, type MouseEvent, type KeyboardEvent } from "react";
import { createPortal } from "react-dom";
import type { ProjectResearch } from "../projects/projects";
import { serverEndpoint } from "../platform";
import { notifyError } from "../notifications/store";

export const RESEARCH_DELETED_EVENT = "solomon:research-deleted";

type Target = { job: Pick<ProjectResearch, "id" | "title">; projectID: string; anchor: HTMLElement; x: number; y: number };

export function useResearchContextMenu(projectID: string | undefined, onDeleted: (id: string) => void) {
  const [target, setTarget] = useState<Target | null>(null);
  const [deleting, setDeleting] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const close = () => { setTarget(null); target?.anchor.focus({ preventScroll: true }); };

  useEffect(() => { setTarget(null); }, [projectID]);
  useEffect(() => {
    if (!target) return;
    const outside = (event: PointerEvent) => { if (!menuRef.current?.contains(event.target as Node)) setTarget(null); };
    const dismiss = () => setTarget(null);
    document.addEventListener("pointerdown", outside);
    document.addEventListener("scroll", dismiss, true);
    window.addEventListener("resize", dismiss);
    return () => {
      document.removeEventListener("pointerdown", outside);
      document.removeEventListener("scroll", dismiss, true);
      window.removeEventListener("resize", dismiss);
    };
  }, [target]);

  useLayoutEffect(() => {
    const menu = menuRef.current;
    if (!target || !menu) return;
    const bounds = menu.getBoundingClientRect();
    menu.style.left = `${Math.max(8, Math.min(target.x, window.innerWidth - bounds.width - 8))}px`;
    menu.style.top = `${Math.max(8, Math.min(target.y, window.innerHeight - bounds.height - 8))}px`;
    menu.querySelector("button")?.focus();
  }, [target]);

  function open(event: MouseEvent<HTMLElement>, job: Pick<ProjectResearch, "id" | "title">) {
    if (!projectID || deleting) return;
    event.preventDefault();
    event.stopPropagation();
    const bounds = event.currentTarget.getBoundingClientRect();
    setTarget({ job, projectID, anchor: event.currentTarget, x: event.clientX || bounds.left, y: event.clientY || bounds.bottom });
  }

  function openWithKeyboard(event: KeyboardEvent<HTMLElement>, job: Pick<ProjectResearch, "id" | "title">) {
    if (event.key !== "ContextMenu" && !(event.shiftKey && event.key === "F10")) return;
    if (!projectID || deleting) return;
    event.preventDefault();
    const bounds = event.currentTarget.getBoundingClientRect();
    setTarget({ job, projectID, anchor: event.currentTarget, x: bounds.left, y: bounds.bottom });
  }

  async function remove() {
    if (!target || deleting) return;
    const current = target;
    setDeleting(true);
    try {
      const response = await fetch(await serverEndpoint(`/__solomon/projects/${encodeURIComponent(current.projectID)}/research/${encodeURIComponent(current.job.id)}`), { method: "DELETE" });
      if (!response.ok) {
        const payload = await response.json().catch(() => null);
        throw new Error(typeof payload?.error === "string" ? payload.error : "Unable to delete deep research.");
      }
      if (current.projectID === projectID) onDeleted(current.job.id);
      window.dispatchEvent(new CustomEvent(RESEARCH_DELETED_EVENT, { detail: { projectID: current.projectID, id: current.job.id } }));
      setTarget(null);
    } catch (error) {
      notifyError(error instanceof Error ? error.message : "Unable to delete deep research.");
    } finally { setDeleting(false); }
  }

  const menu = target ? createPortal(<div ref={menuRef} className="project-context-menu" role="menu" aria-label="Deep research actions" style={{ left: target.x, top: target.y }} onContextMenu={event => event.preventDefault()} onKeyDown={event => {
    if (event.key === "Escape") { event.preventDefault(); event.stopPropagation(); close(); }
  }}>
    <button role="menuitem" className="is-danger" disabled={deleting} type="button" onClick={() => void remove()}>{deleting ? "Deleting…" : "Delete"}</button>
  </div>, document.body) : null;
  return { open, openWithKeyboard, menu };
}
