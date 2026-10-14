import { useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { operateProjectFile, type ProjectDirectoryEntry } from "../projects/projects";

export function CreateEntryActions({ projectID, parentPath, onCreated }: { projectID: string; parentPath: string; onCreated: (entry: ProjectDirectoryEntry) => void }) {
  const [kind, setKind] = useState<"file" | "folder" | null>(null);
  const [parent, setParent] = useState("");
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  useLayoutEffect(() => {
    if (error && !busy) inputRef.current?.focus();
  }, [error, busy]);
  function close() { setKind(null); triggerRef.current?.focus(); }
  async function create() {
    if (!kind || busy) return;
    const value = name.trim();
    if (!value || /[\\/]/.test(value) || value === "." || value === "..") { setError("Enter a valid name without path separators."); return; }
    const entry = { name: value, path: [parent, value].filter(Boolean).join("/"), isDirectory: kind === "folder" };
    setBusy(true); setError("");
    try {
      await operateProjectFile(projectID, { action: kind === "file" ? "create-file" : "create-folder", path: entry.path });
      close(); onCreated(entry);
    } catch (reason) { setError(reason instanceof Error ? reason.message : "Unable to create this entry."); }
    finally { setBusy(false); }
  }
  return <>
    <div className="editor-create-actions">
      {(["file", "folder"] as const).map((item) => <button aria-label={`New ${item}`} title={`New ${item}`} key={item} type="button" onClick={(event) => {
        triggerRef.current = event.currentTarget; setParent(parentPath); setName(""); setError(""); setKind(item);
      }}><svg aria-hidden="true" viewBox="0 0 24 24">{item === "file" ? <><path d="M14 3H5v18h7M14 3v6h6M14 3l6 6v3"/><path d="M18 14v8m-4-4h8"/></> : <><path d="M12 20H3V5h7l2 3h9v5"/><path d="M18 14v8m-4-4h8"/></>}</svg></button>)}
    </div>
    {kind ? createPortal(<div className="project-removal-dialog-backdrop" onKeyDown={(event) => {
      if (event.key === "Escape" && !busy) { event.preventDefault(); close(); }
      if (event.key === "Tab") {
        const items = Array.from(event.currentTarget.querySelectorAll<HTMLElement>("button:not(:disabled), input:not(:disabled)"));
        const first = items[0], last = items.at(-1);
        if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
        else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
      }
    }}>
      <form className="project-removal-dialog file-operation-dialog" role="dialog" aria-modal="true" aria-labelledby="create-entry-title" onSubmit={(event) => { event.preventDefault(); void create(); }}>
        <h2 id="create-entry-title">New {kind}</h2>
        {parent ? <p>Create in {parent}</p> : null}
        <label>Name<input ref={inputRef} aria-label={`New ${kind} name`} autoFocus disabled={busy} value={name} onChange={(event) => setName(event.target.value)} /></label>
        {error ? <p className="project-removal-dialog-error" role="alert">{error}</p> : null}
        <div className="project-removal-dialog-actions"><button disabled={busy} type="button" onClick={close}>Cancel</button><button disabled={busy || !name.trim()} type="submit">{busy ? "Creating…" : "Create"}</button></div>
      </form>
    </div>, document.body) : null}
  </>;
}
