import { useState, type ComponentProps } from "react";
import { createPortal } from "react-dom";
import { MarkdownContent } from "../chat/MarkdownContent";
import { CodeEditor } from "./CodeEditor";
import "../chat/chat.css";
import "./document.css";

export const hasDocumentPreview = (path: string) => /\.(md|markdown|mdown|mkd|mmd|mermaid)$/i.test(path);

export function DocumentView({ toolbarHost, ...props }: ComponentProps<typeof CodeEditor> & { toolbarHost?: HTMLElement | null }) {
  const [view, setView] = useState<"preview" | "source">("preview");
  if (!hasDocumentPreview(props.file.path)) return <CodeEditor {...props} />;
  const content = /\.(mmd|mermaid)$/i.test(props.file.path)
    ? `\`\`\`mermaid\n${props.file.content}\n\`\`\``
    : props.file.content;
  return <div className="editor-document">
    {toolbarHost ? createPortal(<div className="chat-mermaid-switch" role="group" aria-label="Document view">
        <button type="button" aria-pressed={view === "preview"} onClick={() => setView("preview")}>Preview</button>
        <button type="button" aria-pressed={view === "source"} onClick={() => setView("source")}>Source</button>
      </div>, toolbarHost) : null}
    {view === "preview" ? <div className="editor-document-preview" aria-label="Document preview"><article className="chat-message is-assistant"><div className="chat-response-content"><MarkdownContent content={content} /></div></article></div> : <CodeEditor {...props} />}
  </div>;
}
