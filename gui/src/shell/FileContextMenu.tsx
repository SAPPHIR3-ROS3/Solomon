import { useEffect, useLayoutEffect, useRef, useState, type KeyboardEvent as ReactKeyboardEvent, type MouseEvent } from "react";
import { createPortal } from "react-dom";
import { fetchProjectAtMentionSuggestions, operateProjectFile, type ProjectDirectoryEntry, type ProjectFileOperation } from "../projects/projects";
import { copyTextFallback } from "../chat/chatClipboard";
import { addFileToChat } from "../chat/fileReferences";

type FileClipboard = { projectID: string; entry: ProjectDirectoryEntry; cut: boolean };
let fileClipboard: FileClipboard | null = null;
type FileDialog = { kind: "rename" | "delete" | "paste"; entry: ProjectDirectoryEntry; clipboard?: FileClipboard };

type Options = {
  projectID?: string;
  allowAddToChat?: boolean;
  rootPath?: string;
  onOpenFile?: (entry: ProjectDirectoryEntry) => void;
  onOpenFileInNewTab?: (entry: ProjectDirectoryEntry) => void;
  onToggleDirectory: (entry: ProjectDirectoryEntry) => void;
};

export function useFileContextMenu({ projectID, allowAddToChat, rootPath, onOpenFile, onOpenFileInNewTab, onToggleDirectory }: Options) {
  const [target, setTarget] = useState<{ entry: ProjectDirectoryEntry; x: number; y: number } | null>(null);
  const [dialog, setDialog] = useState<FileDialog | null>(null);
  const [value, setValue] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const menuRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const close = () => { setTarget(null); triggerRef.current?.focus(); };
  useEffect(() => {
    if (!target) return;
    const outside = (event: PointerEvent) => { if (!menuRef.current?.contains(event.target as Node)) setTarget(null); };
    const escape = (event: KeyboardEvent) => { if (event.key === "Escape") { event.preventDefault(); close(); } };
    const dismiss = () => setTarget(null);
    document.addEventListener("pointerdown", outside);
    document.addEventListener("keydown", escape);
    document.addEventListener("scroll", dismiss, true);
    window.addEventListener("resize", dismiss);
    return () => {
      document.removeEventListener("pointerdown", outside);
      document.removeEventListener("keydown", escape);
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
    menu.querySelector<HTMLButtonElement>("button")?.focus();
  }, [target]);
  function show(entry: ProjectDirectoryEntry, button: HTMLButtonElement, x: number, y: number) {
    triggerRef.current = button; setError(""); setTarget({ entry, x, y });
  }
  async function copy(text: string) {
    try { if (navigator.clipboard?.writeText) { try { await navigator.clipboard.writeText(text); } catch { copyTextFallback(text); } } else copyTextFallback(text); close(); }
    catch { setError("Unable to copy to clipboard."); }
  }
  function beginDialog(kind: FileDialog["kind"], entry: ProjectDirectoryEntry) {
    setError("");
    setTarget(null);
    setDialog({ kind, entry, clipboard: kind === "paste" ? fileClipboard ?? undefined : undefined });
    if (kind === "rename") setValue(entry.name);
    if (kind === "paste" && fileClipboard) {
      const parent = entry.isDirectory ? entry.path : entry.path.split("/").slice(0, -1).join("/");
      const source = fileClipboard.entry;
      const sameParent = parent === source.path.split("/").slice(0, -1).join("/");
      const name = !fileClipboard.cut && sameParent ? source.name.replace(/(\.[^.]*)?$/, " copy$1") : source.name;
      setValue([parent, name].filter(Boolean).join("/"));
    }
  }
  async function submitOperation() {
    if (!dialog || !projectID || busy) return;
    let operation: ProjectFileOperation;
    if (dialog.kind === "delete") operation = { action: "delete", path: dialog.entry.path };
    else if (dialog.kind === "rename") {
      if (!value.trim() || /[\\/]/.test(value) || value === "." || value === "..") { setError("Enter a valid file name."); return; }
      operation = { action: "rename", path: dialog.entry.path, destination: [...dialog.entry.path.split("/").slice(0, -1), value].join("/") };
    } else {
      if (!dialog.clipboard) return;
      operation = { action: dialog.clipboard.cut ? "move" : "copy", path: dialog.clipboard.entry.path, destination: value };
    }
    setBusy(true); setError("");
    try {
      await operateProjectFile(projectID, operation);
      if (dialog.kind === "paste" && dialog.clipboard?.cut) fileClipboard = null;
      setDialog(null); close();
    } catch (reason) { setError(reason instanceof Error ? reason.message : "File operation failed."); }
    finally { setBusy(false); }
  }
  async function attach(entry: ProjectDirectoryEntry) {
    if (!projectID || busy) return;
    setBusy(true); setError("");
    try {
      const suggestions = await fetchProjectAtMentionSuggestions(projectID, entry.path);
      const suggestion = suggestions.find((item) => item.path.replace(/\/$/, "") === entry.path);
      if (!suggestion) throw new Error("This file cannot be added to chat.");
      addFileToChat({ projectID, tag: suggestion.tag }); close();
    } catch (reason) { setError(reason instanceof Error ? reason.message : "Unable to add file to chat."); }
    finally { setBusy(false); }
  }
  function navigate(event: ReactKeyboardEvent<HTMLDivElement>) {
    const items = Array.from(menuRef.current?.querySelectorAll<HTMLButtonElement>("button:not(:disabled)") ?? []);
    const index = items.indexOf(document.activeElement as HTMLButtonElement);
    if (["ArrowDown", "ArrowUp", "Home", "End", "Tab"].includes(event.key)) {
      event.preventDefault();
      const next = event.key === "Home" ? 0 : event.key === "End" ? items.length - 1 : (index + (event.key === "ArrowUp" || (event.key === "Tab" && event.shiftKey) ? -1 : 1) + items.length) % items.length;
      items[next]?.focus();
    }
  }
  const entry = target?.entry;
  const absolutePath = entry && rootPath ? (/^(?:\/|[A-Za-z]:[\\/])/.test(entry.path) ? entry.path : `${rootPath.replace(/[\\/]$/, "")}/${entry.path}`) : undefined;
  return {
    open: (event: MouseEvent<HTMLButtonElement>, entry: ProjectDirectoryEntry) => { event.preventDefault(); event.stopPropagation(); show(entry, event.currentTarget, event.clientX, event.clientY); },
    openWithKeyboard: (event: ReactKeyboardEvent<HTMLButtonElement>, entry: ProjectDirectoryEntry) => {
      if (event.key !== "ContextMenu" && !(event.shiftKey && event.key === "F10")) return;
      event.preventDefault();
      const bounds = event.currentTarget.getBoundingClientRect();
      show(entry, event.currentTarget, bounds.left, bounds.bottom);
    },
    menu: <> {target && entry ? createPortal(<div ref={menuRef} className="project-context-menu" role="menu" aria-label={`Actions for ${entry.name}`} style={{ left: target.x, top: target.y }} onKeyDown={navigate} onContextMenu={(event) => event.preventDefault()}>
      {entry.isDirectory ? <button role="menuitem" type="button" onClick={() => { onToggleDirectory(entry); close(); }}>Expand / collapse folder</button> : <>
        {onOpenFile ? <button role="menuitem" type="button" onClick={() => { onOpenFile(entry); close(); }}>Open in editor</button> : null}
        {onOpenFileInNewTab ? <button role="menuitem" type="button" onClick={() => { onOpenFileInNewTab(entry); close(); }}>Open in new tab</button> : null}
      </>}
      {projectID ? <>
        {allowAddToChat ? <button disabled={busy} role="menuitem" type="button" onClick={() => void attach(entry)}>Add to chat</button> : null}
        <div className="project-context-menu-divider" role="separator" />
        <button role="menuitem" type="button" onClick={() => { fileClipboard = { projectID, entry, cut: true }; close(); }}>Cut</button>
        <button role="menuitem" type="button" onClick={() => { fileClipboard = { projectID, entry, cut: false }; close(); }}>Copy</button>
        <button disabled={!fileClipboard || fileClipboard.projectID !== projectID} role="menuitem" type="button" onClick={() => beginDialog("paste", entry)}>Paste</button>
        <button role="menuitem" type="button" onClick={() => beginDialog("rename", entry)}>Rename</button>
        <button className="is-danger" role="menuitem" type="button" onClick={() => beginDialog("delete", entry)}>Delete</button>
        <div className="project-context-menu-divider" role="separator" />
      </> : null}
      <button role="menuitem" type="button" onClick={() => void copy(entry.name)}>Copy name</button>
      <button role="menuitem" type="button" onClick={() => void copy(entry.path)}>Copy path</button>
      {absolutePath ? <button role="menuitem" type="button" onClick={() => void copy(absolutePath)}>Copy absolute path</button> : null}
      {error ? <p role="alert">{error}</p> : null}
    </div>, document.body) : null}
    {dialog ? createPortal(<div className="project-removal-dialog-backdrop" onKeyDown={(event) => {
      if (event.key === "Escape" && !busy) { setDialog(null); close(); }
      if (event.key === "Tab") {
        const items = Array.from(event.currentTarget.querySelectorAll<HTMLElement>("button:not(:disabled), input:not(:disabled)"));
        const first = items[0], last = items.at(-1);
        if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
        else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
      }
    }}>
      <form className="project-removal-dialog file-operation-dialog" role="dialog" aria-modal="true" aria-labelledby="file-operation-title" onSubmit={(event) => { event.preventDefault(); void submitOperation(); }}>
        <h2 id="file-operation-title">{dialog.kind === "delete" ? "Delete" : dialog.kind === "rename" ? "Rename" : "Paste"} {dialog.kind === "paste" ? dialog.clipboard?.entry.name : dialog.entry.name}</h2>
        {dialog.kind === "delete" ? <p>This will permanently delete {dialog.entry.isDirectory ? "this folder and its contents" : "this file"}.</p> : <label>{dialog.kind === "rename" ? "New name" : "Destination path"}<input autoFocus disabled={busy} value={value} onChange={(event) => setValue(event.target.value)} /></label>}
        {error ? <p className="project-removal-dialog-error" role="alert">{error}</p> : null}
        <div className="project-removal-dialog-actions">
          <button autoFocus={dialog.kind === "delete"} disabled={busy} type="button" onClick={() => { setDialog(null); close(); }}>Cancel</button>
          <button disabled={busy} className={dialog.kind === "delete" ? "is-danger" : ""} type="submit">{busy ? "Working…" : dialog.kind === "delete" ? "Delete" : dialog.kind === "rename" ? "Rename" : "Paste"}</button>
        </div>
      </form>
    </div>, document.body) : null}</>,
  };
}
