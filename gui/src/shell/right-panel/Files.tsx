import { useMemo } from "react";
import { WORKSPACE_FILE_REFERENCE } from "../../chat/fileReferences";
import type { ProjectDirectoryEntry } from "../../projects/projects";
import { useFileContextMenu } from "../FileContextMenu";
import { FolderIcon, ChatFolderIcon, FileIcon, IndexMark } from "./icons";
import { gitStatusFolderStatuses, gitStatusLabel } from "./status";
type FileEntriesProps = {
  projectID?: string;
  allowAddToChat?: boolean;
  rootPath?: string;
  depth: number;
  entries: Record<string, ProjectDirectoryEntry[]>;
  expandedDirectories: Set<string>;
  collapsedDirectories?: Set<string>;
  fileStatus?: Record<string, string>;
  folderStatus?: Record<string, string>;
  nameFilter: string;
  onOpenFile?: (entry: ProjectDirectoryEntry) => void;
  onOpenFileInNewTab?: (entry: ProjectDirectoryEntry) => void;
  onStageEntry?: (entry: ProjectDirectoryEntry) => void;
  onUnstageEntry?: (entry: ProjectDirectoryEntry) => void;
  onToggleDirectory: (entry: ProjectDirectoryEntry) => void;
  parentPath: string;
  selectedPath?: string;
  iconMode?: "all" | "folders-chat" | "none";
};

export function entryMatchesFilter(entry: ProjectDirectoryEntry, nameFilter: string, entries: Record<string, ProjectDirectoryEntry[]>): boolean {
  if (!nameFilter) return true;
  if (entry.name.toLowerCase().includes(nameFilter)) return true;
  if (!entry.isDirectory) return false;
  return (entries[entry.path] ?? []).some((child) => entryMatchesFilter(child, nameFilter, entries));
}

export function FileEntries({ projectID, allowAddToChat, rootPath, collapsedDirectories, depth, entries, expandedDirectories, fileStatus, folderStatus, iconMode = "all", nameFilter, onOpenFile, onOpenFileInNewTab, onStageEntry, onUnstageEntry, onToggleDirectory, parentPath, selectedPath }: FileEntriesProps) {
  const directoryStatus = useMemo(() => folderStatus ?? gitStatusFolderStatuses(fileStatus ?? {}), [folderStatus, fileStatus]);
  const contextMenu = useFileContextMenu({ projectID, allowAddToChat, rootPath, onOpenFile, onOpenFileInNewTab, onToggleDirectory, canRestore: onStageEntry ? (entry) => !entry.isDirectory && !!fileStatus?.[entry.path] && fileStatus[entry.path] !== "U" : undefined });
  return <>{entries[parentPath]?.filter((entry) => entryMatchesFilter(entry, nameFilter, entries)).map((entry) => {
    const hasMatchingChild = Boolean(nameFilter) && entry.isDirectory && (entries[entry.path] ?? []).some((child) => entryMatchesFilter(child, nameFilter, entries));
    const isExpanded = entry.isDirectory && (hasMatchingChild || (collapsedDirectories ? !collapsedDirectories.has(entry.path) : expandedDirectories.has(entry.path)));
    const status = entry.isDirectory ? directoryStatus[entry.path] : fileStatus?.[entry.path];
    return (
      <div className="right-side-panel-file" key={entry.path}>
        <button
          aria-expanded={entry.isDirectory ? isExpanded : undefined}
          aria-current={entry.path === selectedPath ? "true" : undefined}
          className={`right-side-panel-file-row${entry.path === selectedPath ? " is-active" : ""}${status ? ` status-${status}` : ""}`}
          data-depth={depth}
          draggable={Boolean(projectID)}
          onDragStart={(event) => {
            if (!projectID) return;
            event.dataTransfer.effectAllowed = "copyMove";
            event.dataTransfer.setData(WORKSPACE_FILE_REFERENCE, JSON.stringify({ projectID, path: entry.path }));
          }}
          onClick={() => entry.isDirectory ? onToggleDirectory(entry) : onOpenFile?.(entry)}
          onContextMenu={(event) => contextMenu.open(event, entry)}
          onKeyDown={(event) => contextMenu.openWithKeyboard(event, entry)}
          onDoubleClick={() => entry.isDirectory ? undefined : onOpenFileInNewTab?.(entry)}
          title={entry.name}
          type="button"
        >
          {iconMode === "all" ? (entry.isDirectory ? <FolderIcon name={entry.name} isOpen={isExpanded} /> : <FileIcon fileName={entry.name} />) : null}
          {iconMode === "folders-chat" && entry.isDirectory ? <ChatFolderIcon isOpen={isExpanded} /> : null}
          <span>{entry.name}</span>
          {entry.isDirectory && !isExpanded && status ? <i aria-label={`Folder status: ${gitStatusLabel(status)}`} className={`right-side-panel-folder-status status-${status}`} /> : null}
          {!entry.isDirectory && status ? <i aria-label={gitStatusLabel(status)} className={`right-side-panel-file-status status-${status}`}>{status}</i> : null}
          {onStageEntry ? <span aria-label={`Stage ${entry.name}`} className="right-side-panel-stage" role="button" tabIndex={0} title="Stage changes" onClick={(event) => { event.stopPropagation(); onStageEntry(entry); }} onKeyDown={(event) => { if (event.key === "Enter" || event.key === " ") { event.preventDefault(); event.stopPropagation(); onStageEntry(entry); } }}><IndexMark kind="add" /></span> : null}
          {onUnstageEntry ? <span aria-label={`Unstage ${entry.name}`} className="right-side-panel-stage" role="button" tabIndex={0} title="Unstage changes" onClick={(event) => { event.stopPropagation(); onUnstageEntry(entry); }} onKeyDown={(event) => { if (event.key === "Enter" || event.key === " ") { event.preventDefault(); event.stopPropagation(); onUnstageEntry(entry); } }}><IndexMark kind="remove" /></span> : null}
        </button>
        {isExpanded ? (
          <div className="right-side-panel-file-children">
            {entries[entry.path] ? (
              <FileEntries projectID={projectID} allowAddToChat={allowAddToChat} rootPath={rootPath} collapsedDirectories={collapsedDirectories} depth={depth + 1} entries={entries} expandedDirectories={expandedDirectories} fileStatus={fileStatus} folderStatus={directoryStatus} iconMode={iconMode} nameFilter={nameFilter} onOpenFile={onOpenFile} onOpenFileInNewTab={onOpenFileInNewTab} onStageEntry={onStageEntry} onUnstageEntry={onUnstageEntry} onToggleDirectory={onToggleDirectory} parentPath={entry.path} selectedPath={selectedPath} />
            ) : <span className="right-side-panel-loading">Loading…</span>}
          </div>
        ) : null}
      </div>
    );
  }) ?? null}{contextMenu.menu}</>;
}
