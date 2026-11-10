import { useEffect, useRef, useState, type RefObject } from "react";
import { Compartment } from "@codemirror/state";
import { EditorView, ViewPlugin } from "@codemirror/view";
import { serverEndpoint } from "../platform";
import { LanguageConnection } from "./connection";
import { completionExtension, hoverExtension, languageDiagnostics, languageFeatureTheme } from "./editorFeatures";
import { identifyLanguage, recordLanguage } from "./languages";

export const languageServerCompartment = new Compartment();

export function useLanguageServer(viewRef: RefObject<EditorView | null>, projectID: string, fileID: string, path: string, saved: string) {
  const [status, setStatus] = useState("Internal fallback");
  const [revision, setRevision] = useState(0);
  const save = useRef<(() => void) | null>(null);
  useEffect(() => {
    const changed = () => setRevision(value => value + 1);
    window.addEventListener("solomon:language-settings", changed);
    return () => window.removeEventListener("solomon:language-settings", changed);
  }, []);
  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    let disposed = false;
    let connection: LanguageConnection | undefined;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let label = "";
    setStatus("Internal fallback");
    const fallback = (reason: string) => {
      if (disposed) return;
      setStatus(`${label ? label + " · " : ""}Internal fallback · ${reason}`);
      if (viewRef.current === view) view.dispatch({ effects: languageServerCompartment.reconfigure([]) });
    };
    void (async () => {
      const language = await identifyLanguage(path, view.state.doc.line(1).text);
      if (!language || disposed) return;
      label = language.name;
      const rows = await recordLanguage(language);
      if (disposed) return;
      const settings = rows.find(row => row.id === language.id);
      if (!settings?.server.available) { fallback(settings?.server.reason ?? "No language server detected"); return; }
      setStatus(`${label} · Connecting to language server…`);
      const query = new URLSearchParams({ project: projectID, path, language: language.id });
      const documentResponse = await fetch(await serverEndpoint(`/__solomon/language-document?${query}`));
      if (!documentResponse.ok) throw new Error(await documentResponse.text());
      const { rootUri, uri } = await documentResponse.json();
      if (disposed) return;
      const url = new URL(await serverEndpoint(`/__solomon/language-server?${query}`), window.location.href);
      url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
      connection = new LanguageConnection(new WebSocket(url), settings, uri);
      connection.onClose = fallback;
      let version = 1;
      connection.onDiagnostics = (diagnostics, diagnosticVersion) => {
        if (disposed || settings.diagnostics === false || diagnosticVersion !== undefined && diagnosticVersion !== version) return;
        view.dispatch({ effects: languageDiagnostics.of(diagnostics) });
      };
      const capabilities = await connection.open(rootUri);
      if (disposed) { connection.close(); return; }
      const client = connection;
      let previous = view.state.doc.toString();
      const sync = typeof capabilities.textDocumentSync === "number" ? capabilities.textDocumentSync : capabilities.textDocumentSync?.change ?? 0;
      const flush = () => {
        if (timer !== undefined) { clearTimeout(timer); timer = undefined; }
        const text = view.state.doc.toString();
        if (text === previous || !sync || disposed) return;
        const lines = previous.split("\n");
        client.notify("textDocument/didChange", {
          textDocument: { uri, version: ++version },
          contentChanges: [sync === 2 ? { range: { start: { line: 0, character: 0 }, end: { line: lines.length - 1, character: lines.at(-1)!.length } }, text } : { text }],
        });
        previous = text;
      };
      const extensions = [...languageFeatureTheme, ViewPlugin.fromClass(class {
        update(update: import("@codemirror/view").ViewUpdate) {
          if (!update.docChanged) return;
          if (timer !== undefined) clearTimeout(timer);
          timer = setTimeout(flush, 120);
        }
      })];
      if (capabilities.completionProvider && settings.completion !== false) extensions.push(completionExtension(client, uri, flush, capabilities.completionProvider.triggerCharacters ?? []));
      if (capabilities.hoverProvider && settings.hover !== false) extensions.push(hoverExtension(client, uri, flush));
      view.dispatch({ effects: languageServerCompartment.reconfigure(extensions) });
      client.notify("textDocument/didOpen", { textDocument: { uri, languageId: language.id, version, text: previous } });
      const saveOptions = typeof capabilities.textDocumentSync === "object" ? capabilities.textDocumentSync.save : false;
      save.current = saveOptions ? () => {
        flush();
        client.notify("textDocument/didSave", { textDocument: { uri }, ...(typeof saveOptions === "object" && saveOptions.includeText ? { text: view.state.doc.toString() } : {}) });
      } : null;
      setStatus(`${label} · LSP: ${settings.server.command?.[0].split(/[\\/]/).at(-1) ?? "connected"}`);
    })().catch(error => { fallback(error instanceof Error ? error.message : "Language server unavailable"); connection?.close(); });
    return () => {
      disposed = true; save.current = null;
      if (timer !== undefined) clearTimeout(timer);
      connection?.close();
      if (viewRef.current === view) view.dispatch({ effects: languageServerCompartment.reconfigure([]) });
    };
  }, [projectID, fileID, path, revision, viewRef]);
  useEffect(() => { save.current?.(); }, [saved]);
  return status;
}
