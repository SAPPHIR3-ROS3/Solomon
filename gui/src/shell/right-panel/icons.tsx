import { folderIcon } from "../../folderIcons";
import { fileIconName } from "../../fileIcons";
export function SearchIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 16 16">
      <circle cx="7" cy="7" r="4.5" />
      <path d="M10.5 10.5 14 14" />
    </svg>
  );
}

export function NewDocumentIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="M6.25 3.5h8.25l5 5v10.25a2 2 0 0 1-2 2H6.25a2.5 2.5 0 0 1-2.5-2.5V6a2.5 2.5 0 0 1 2.5-2.5Z" />
      <path d="M14.5 3.5v3.25a1.75 1.75 0 0 0 1.75 1.75h3.25" />
    </svg>
  );
}

export function GitHistoryIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="M8 5v14" />
      <path d="M8 8h3a4 4 0 0 1 4 4v1" />
      <circle cx="8" cy="5" r="2" />
      <circle cx="8" cy="19" r="2" />
      <circle cx="15" cy="15" r="2" />
    </svg>
  );
}

export function CloudIcon() {
  return (
    <svg aria-hidden="true" className="right-side-panel-history-ref-icon" viewBox="0 0 24 24">
      <path d="M7.25 18.5h9.5a4.25 4.25 0 0 0 .5-8.47A6 6 0 0 0 5.7 8.6a3.9 3.9 0 0 0 1.55 7.9Z" />
    </svg>
  );
}

export function HistoryChevronIcon({ open = true }: { open?: boolean }) {
  return (
    <svg aria-hidden="true" className={open ? "open" : ""} viewBox="0 0 24 24">
      <path d="m9 18 6-6-6-6" />
    </svg>
  );
}

export function ResearchIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="m4.5 12.2 8.7-5.1 3.2 5.4-8.8 5.1z" />
      <path d="m13.2 7.1 2.6-1.5 3.2 5.4-2.6 1.5M5.1 11.9l-1.5.9 1.3 2.2 1.5-.9" />
      <path d="M10.1 16.3 9.2 20M10.1 16.3l3.9 3.7M10.1 16.3 5.8 20" />
    </svg>
  );
}

export function FolderIcon({ name, isOpen }: { name: string; isOpen: boolean }) {
  const { src, badge } = folderIcon(name, isOpen);
  const icon = <img aria-hidden="true" className="right-side-panel-folder-image" src={src} />;
  if (!badge) return <span aria-hidden="true" className="right-side-panel-file-icon">{icon}</span>;
  return <span aria-hidden="true" className="right-side-panel-file-icon">{icon}<img className="right-side-panel-folder-badge" src={`/vscode-icons/${badge}.svg`} /></span>;
}

export function ChatFolderIcon({ isOpen }: { isOpen: boolean }) {
  return (
    <svg aria-hidden="true" className="right-side-panel-file-icon right-side-panel-chat-folder-icon" viewBox="0 0 24 24">
      <path d={isOpen
        ? "m6 14 1.5-2.9A2 2 0 0 1 9.24 10H20a2 2 0 0 1 1.94 2.5l-1.54 6a2 2 0 0 1-1.95 1.5H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h3.9a2 2 0 0 1 1.69.9l.81 1.2a2 2 0 0 0 1.67.9H18a2 2 0 0 1 2 2v2"
        : "M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"}
      />
    </svg>
  );
}

export function FileIcon({ fileName }: { fileName: string }) {
  return <img aria-hidden="true" className="right-side-panel-file-icon" src={`/vscode-icons/${fileIconName(fileName)}.svg`} />;
}
export function IndexMark({ kind }: { kind: "add" | "remove" }) {
  return <svg aria-hidden="true" viewBox="0 0 12 12"><path d={kind === "add" ? "M6 1.5v9M1.5 6h9" : "M1.5 6h9"} /></svg>;
}
