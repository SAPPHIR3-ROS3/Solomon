import { useEffect, useRef, useState } from "react";
import { identifyLanguage, languageRequest, type LanguageSettings } from "./languages";
import "./status.css";

function languageLabel(status: string) {
  return status === "Internal fallback" ? "Plain text" : status.split(" · ")[0];
}

export function LanguageStatus({ path, firstLine, status }: { path: string; firstLine: string; status: string }) {
  const root = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  const [language, setLanguage] = useState<LanguageSettings | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  useEffect(() => { setOpen(false); setLanguage(null); setError(""); }, [path]);
  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    setBusy(true); setError("");
    void identifyLanguage(path, firstLine).then(async identity => {
      if (!identity) return null;
      const rows = await languageRequest<LanguageSettings[]>();
      return rows.find(row => row.id === identity.id) ?? null;
    }).then(row => { if (!cancelled) setLanguage(row); }).catch(error => { if (!cancelled) setError(String(error)); }).finally(() => { if (!cancelled) setBusy(false); });
    const dismiss = (event: PointerEvent) => { if (!root.current?.contains(event.target as Node)) setOpen(false); };
    const escape = (event: KeyboardEvent) => { if (event.key === "Escape") { setOpen(false); root.current?.querySelector<HTMLButtonElement>(".editor-language-status")?.focus(); } };
    document.addEventListener("pointerdown", dismiss);
    document.addEventListener("keydown", escape);
    return () => { cancelled = true; document.removeEventListener("pointerdown", dismiss); document.removeEventListener("keydown", escape); };
  }, [open, path, firstLine, status]);
  const choose = async (mode: LanguageSettings["mode"], command = language?.command ?? []) => {
    if (!language) { setOpen(false); return; }
    setBusy(true); setError("");
    try {
      await languageRequest("PATCH", { ...language, mode, command });
      setOpen(false);
      window.dispatchEvent(new Event("solomon:language-settings"));
      root.current?.querySelector<HTMLButtonElement>(".editor-language-status")?.focus();
    } catch (error) { setError(String(error)); }
    finally { setBusy(false); }
  };
  const servers = (language?.servers ?? (language?.server.available ? [language.server] : [])).filter(server => server.available && server.command?.length);
  const sameCommand = (command: string[] | null) => JSON.stringify(command) === JSON.stringify(language?.server.command);
  const item = (label: string, selected: boolean, action: () => void, title?: string) => <button key={title ?? label} type="button" role="option" aria-selected={selected} disabled={busy} onClick={action} title={title}><span>{label}</span></button>;
  return <div className="editor-branch-control editor-language-control" ref={root} onKeyDown={event => {
    if (!["ArrowDown", "ArrowUp", "Home", "End"].includes(event.key)) return;
    event.preventDefault();
    if (!open) { setOpen(true); return; }
    const options = [...root.current?.querySelectorAll<HTMLButtonElement>('[role="option"]:not(:disabled)') ?? []];
    const current = options.indexOf(document.activeElement as HTMLButtonElement);
    const next = event.key === "Home" ? 0 : event.key === "End" ? options.length - 1 : (current + (event.key === "ArrowUp" ? -1 : 1) + options.length) % options.length;
    options[next]?.focus();
  }}>
    <button type="button" className="editor-branch editor-language-status" aria-label={`Language server: ${languageLabel(status)}. Select LSP`} aria-haspopup="listbox" aria-expanded={open} title={status} onClick={() => setOpen(value => !value)}><span>{languageLabel(status)}</span></button>
    {open ? <div className="editor-branch-menu editor-language-menu" role="listbox" aria-label="Language servers">
      {language ? <>
        {servers.length ? item("Automatic", language.mode === "auto", () => void choose("auto")) : null}
        {servers.map(server => item(server.command?.[0].split(/[\\/]/).at(-1) ?? "LSP", language.mode === "custom" && sameCommand(server.command), () => void choose("custom", server.command ?? []), JSON.stringify(server.command)))}
        {item("Internal", !servers.length || language.mode === "fallback", () => void choose("fallback"))}
      </> : item("Internal", true, () => void choose("fallback"))}
      {error ? <span className="editor-language-error" role="alert" title={error}>Unable to change LSP</span> : null}
    </div> : null}
  </div>;
}
