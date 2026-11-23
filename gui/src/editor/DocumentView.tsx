import { useRef, useState, type ComponentProps } from "react";
import { createPortal } from "react-dom";
import { MarkdownContent } from "../chat/MarkdownContent";
import { CodeEditor } from "./CodeEditor";
import { useFilePosition, useViewportPosition } from "./filePosition";
import "../chat/chat.css";
import "./document.css";

export const hasDocumentPreview = (path: string) => /\.(md|markdown|mdown|mkd|mmd|mermaid)$/i.test(path);

export function DocumentView({ toolbarHost, ...props }: ComponentProps<typeof CodeEditor> & { toolbarHost?: HTMLElement | null }) {
  const [view, setView] = useState<"preview" | "source">("preview");
  if (!hasDocumentPreview(props.file.path)) return <RememberedCodeEditor {...props} key={props.file.path} />;
  const content = /\.(mmd|mermaid)$/i.test(props.file.path)
    ? `\`\`\`mermaid\n${props.file.content}\n\`\`\``
    : props.file.content;
  return <div className="editor-document">
    {toolbarHost ? createPortal(<div className="chat-mermaid-switch" role="group" aria-label="Document view">
        <button type="button" aria-pressed={view === "preview"} onClick={() => setView("preview")}>Preview</button>
        <button type="button" aria-pressed={view === "source"} onClick={() => setView("source")}>Source</button>
      </div>, toolbarHost) : null}
    {view === "preview" ? <DocumentPreview key={props.file.path} projectID={props.projectID} path={props.file.path} content={content} /> : <RememberedCodeEditor {...props} key={props.file.path} />}
  </div>;
}

function RememberedCodeEditor(props: ComponentProps<typeof CodeEditor>) {
  const { position, save } = useFilePosition(props.projectID, props.file.path, "source", props.viewState);
  if (position === undefined) return null;
  const viewState = position ? { ...position, anchor: position.anchor ?? 0, head: position.head ?? 0 } : undefined;
  return <CodeEditor {...props} viewState={viewState} onViewState={(next) => {
    props.onViewState(next);
    save(next);
  }} />;
}

function DocumentPreview({ projectID, path, content }: { projectID: string; path: string; content: string }) {
  const host = useRef<HTMLDivElement>(null);
  const { position, save } = useFilePosition(projectID, path, "preview");
  const remember = useViewportPosition(host, position, save);
  return <div ref={host} onScroll={remember} className="editor-document-preview" aria-label="Document preview"><article className="chat-message is-assistant"><div className="chat-response-content"><MarkdownContent content={content} /></div></article></div>;
}
