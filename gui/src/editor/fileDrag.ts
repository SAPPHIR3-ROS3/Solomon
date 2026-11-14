import { importDroppedFiles, importDroppedPaths, isExternalFileDrag } from "./importFiles";
import { NATIVE_FILE_DROP_EVENT, usesNativeFileDrops } from "../home/imageDrops";
import { useEffect, useRef } from "react";
import { operateProjectFile, type ProjectDirectoryEntry } from "../projects/projects";

export function moveDestination(source: ProjectDirectoryEntry, folder: string): string | null {
  const destination = [folder, source.name].filter(Boolean).join("/");
  if (destination === source.path || folder === source.path || folder.startsWith(`${source.path}/`)) return null;
  return destination;
}

export function useFileDrag(projectID: string | undefined, entries: Record<string, ProjectDirectoryEntry[]>, onError: (message: string) => void) {
  const host = useRef<HTMLElement>(null);
  const current = useRef({ entries, onError });
  current.current = { entries, onError };
  useEffect(() => {
    const element = host.current;
    if (!element || !projectID) return;
    let source: ProjectDirectoryEntry | null = null;
    let highlighted: HTMLElement | null = null;
    let ghost: HTMLElement | null = null;
    let busy = false;
    const rowSelector = ".editor-file-list .right-side-panel-file-row";
    function row(target: EventTarget | null) {
      return target instanceof Element ? target.closest<HTMLButtonElement>(rowSelector) : null;
    }
    function entryFor(button: HTMLElement): ProjectDirectoryEntry | undefined {
      const names: string[] = [];
      let node = button.closest(".right-side-panel-file");
      while (node) {
        const item = node.querySelector<HTMLButtonElement>(":scope > .right-side-panel-file-row");
        if (item) names.unshift(item.title);
        node = node.parentElement?.closest(".right-side-panel-file") ?? null;
      }
      const path = names.join("/");
      return Object.values(current.current.entries).flat().find((entry) => entry.path === path);
    }
    function clearHighlight() {
      highlighted?.classList.remove("is-drop-target");
      highlighted = null;
    }
    function cleanup() {
      clearHighlight();
      ghost?.remove();
      ghost = null;
      source = null;
    }
    function prepareRows() {
      element!.querySelectorAll<HTMLButtonElement>(rowSelector).forEach((button) => { button.draggable = true; });
    }
    function targetFolder(event: { target: EventTarget | null }): { node: HTMLElement; path: string } | null {
      const button = row(event.target);
      if (button) {
        const entry = entryFor(button);
        return entry?.isDirectory ? { node: button, path: entry.path } : null;
      }
      const root = event.target instanceof Element ? event.target.closest<HTMLElement>(".editor-root, .editor-file-list") : null;
      return root ? { node: root, path: "" } : null;
    }
    function start(event: DragEvent) {
      const button = row(event.target);
      if (!button || !event.dataTransfer || busy) { if (button) event.preventDefault(); return; }
      source = entryFor(button) ?? null;
      if (!source) { event.preventDefault(); return; }
      event.dataTransfer.effectAllowed = "move";
      event.dataTransfer.setData("application/x-solomon-workspace-entry", source.path);
      ghost = button.cloneNode(true) as HTMLElement;
      ghost.className = "editor-file-drag-ghost";
      ghost.style.width = `${Math.min(button.getBoundingClientRect().width, 280)}px`;
      document.body.appendChild(ghost);
      event.dataTransfer.setDragImage(ghost, 16, 16);
    }
    function over(event: DragEvent) {
      clearHighlight();
      const target = targetFolder(event);
      if (busy || !target || (source ? !moveDestination(source, target.path) : !isExternalFileDrag(event.dataTransfer))) return;
      event.preventDefault();
      if (event.dataTransfer) event.dataTransfer.dropEffect = source ? "move" : "copy";
      highlighted = target.node;
      highlighted.classList.add("is-drop-target");
    }
    function runImport(work: () => Promise<void>) {
      busy = true;
      cleanup();
      void work().catch(error => current.current.onError(error instanceof Error ? error.message : "Unable to import files."))
        .finally(() => { busy = false; });
    }
    function nativeDrop(event: Event) {
      const { x, y, paths } = (event as CustomEvent<{ x: number; y: number; paths: string[] }>).detail;
      const node = document.elementFromPoint(x, y);
      if (!node || !element!.contains(node) || busy || !paths.length) return;
      const target = targetFolder({ target: node });
      if (target) runImport(() => importDroppedPaths(projectID!, target.path, paths));
    }
    function drop(event: DragEvent) {
      const target = targetFolder(event);
      if (!source && isExternalFileDrag(event.dataTransfer)) {
        event.preventDefault();
        if (!target || busy) { cleanup(); return; }
        if (usesNativeFileDrops()) { cleanup(); return; }
        const data = event.dataTransfer!;
        if (Array.from(data.items).some(item => item.webkitGetAsEntry?.()?.isDirectory)) {
          cleanup();
          current.current.onError("Folder import is available in the desktop app. Drop individual files here.");
          return;
        }
        const files = Array.from(data.files);
        runImport(() => importDroppedFiles(projectID!, target.path, files));
        return;
      }
      const destination = source && target ? moveDestination(source, target.path) : null;
      if (!source || !destination || busy) { cleanup(); return; }
      event.preventDefault();
      const path = source.path;
      cleanup();
      busy = true;
      void operateProjectFile(projectID!, { action: "move", path, destination })
        .catch((error) => current.current.onError(error instanceof Error ? error.message : "Unable to move file."))
        .finally(() => { busy = false; });
    }
    function leave(event: DragEvent) {
      if (!(event.relatedTarget instanceof Node) || !highlighted?.contains(event.relatedTarget)) clearHighlight();
    }
    prepareRows();
    const observer = new MutationObserver(prepareRows);
    observer.observe(element, { childList: true, subtree: true });
    element.addEventListener("dragstart", start);
    element.addEventListener("dragover", over);
    element.addEventListener("drop", drop);
    element.addEventListener("dragleave", leave);
    window.addEventListener("dragend", cleanup);
    window.addEventListener(NATIVE_FILE_DROP_EVENT, nativeDrop);
    return () => {
      observer.disconnect();
      element.removeEventListener("dragstart", start);
      element.removeEventListener("dragover", over);
      element.removeEventListener("drop", drop);
      element.removeEventListener("dragleave", leave);
      window.removeEventListener("dragend", cleanup);
      window.removeEventListener(NATIVE_FILE_DROP_EVENT, nativeDrop);
      cleanup();
    };
  }, [projectID]);
  return host;
}
