import { FolderIcon } from "./ChatIcons";

export function ChatTopbar({ breadcrumb, onOpenFolder, onTitleContextMenu, title }: { breadcrumb?: string; onOpenFolder: () => void; onTitleContextMenu?: (x: number, y: number) => void; title: string }) {
  const folderName = breadcrumb ?? "Project";

  return (
    <div aria-label={`${folderName} / ${title}`} className="chat-topbar">
      <button aria-label={`Back to new chat in ${folderName}`} className="chat-topbar-context" onClick={onOpenFolder} type="button">
        <FolderIcon />
        <span className="chat-topbar-folder">{folderName}</span>
        <span aria-hidden="true" className="chat-topbar-slash">/</span>
      </button>
      <span className="chat-topbar-title" title={title} tabIndex={onTitleContextMenu ? 0 : undefined} aria-haspopup={onTitleContextMenu ? "menu" : undefined}
        onContextMenu={onTitleContextMenu ? (event) => {
          event.preventDefault();
          event.stopPropagation();
          onTitleContextMenu(event.clientX, event.clientY);
        } : undefined}
        onKeyDown={onTitleContextMenu ? (event) => {
          if (event.key !== "ContextMenu" && !(event.shiftKey && event.key === "F10")) return;
          event.preventDefault();
          const rect = event.currentTarget.getBoundingClientRect();
          onTitleContextMenu(rect.left + 12, rect.bottom + 4);
        } : undefined}>{title}</span>
    </div>
  );
}
