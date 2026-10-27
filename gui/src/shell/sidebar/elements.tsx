import type { KeyboardEvent, MouseEvent } from "react";

export function formatRelativeTime(dateTime: string) {
  const timestamp = Date.parse(dateTime);
  if (Number.isNaN(timestamp)) return "";

  const minutes = Math.max(0, Math.floor((Date.now() - timestamp) / 60_000));
  if (minutes < 1) return "now";
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h`;
  const days = Math.floor(hours / 24);
  return `${days}d`;
}

type ChatListButtonProps = {
  chatID: string;
  dateTime?: string;
  isWorking?: boolean;
  isUnreadCompleted?: boolean;
  onClick: () => void;
  onContextMenu: (event: MouseEvent<HTMLButtonElement>) => void;
  onContextMenuKey: (event: KeyboardEvent<HTMLButtonElement>) => void;
  timeLabel: string;
  timeTitle?: string;
  title: string;
};

export function ChatListButton({ chatID, dateTime, isWorking = false, isUnreadCompleted = false, onClick, onContextMenu, onContextMenuKey, timeLabel, timeTitle, title }: ChatListButtonProps) {
  const showUnreadCompleted = isUnreadCompleted && !isWorking;
  return (
    <button aria-busy={isWorking || undefined} aria-label={showUnreadCompleted ? `${title}, completed, unread` : undefined} className={`side-panel-chat${isWorking ? " is-working" : showUnreadCompleted ? " is-unread-completed" : ""}`} key={chatID} onClick={onClick} onContextMenu={onContextMenu} onKeyDown={onContextMenuKey} title={title} type="button">
      {isWorking || showUnreadCompleted ? <span aria-hidden="true" className={`side-panel-chat-working-dot${showUnreadCompleted ? " is-completed" : ""}`} /> : null}
      <span>{title}</span>
      <time dateTime={dateTime} title={timeTitle}>{timeLabel}</time>
    </button>
  );
}

export function formatFileSize(bytes: number) {
  if (bytes < 0) return "Size unavailable";
  if (bytes < 1024) return `${bytes} B`;
  const units = ["KB", "MB", "GB", "TB"];
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)) - 1, units.length - 1);
  return `${(bytes / (1024 ** (index + 1))).toFixed(bytes >= 1024 ** (index + 2) ? 1 : 0)} ${units[index]}`;
}

export function FolderIcon({ isOpen }: { isOpen: boolean }) {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d={isOpen
        ? "m6 14 1.5-2.9A2 2 0 0 1 9.24 10H20a2 2 0 0 1 1.94 2.5l-1.54 6a2 2 0 0 1-1.95 1.5H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h3.9a2 2 0 0 1 1.69.9l.81 1.2a2 2 0 0 0 1.67.9H18a2 2 0 0 1 2 2v2"
        : "M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"}
      />
    </svg>
  );
}

export function PlusIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="M12 5v14M5 12h14" />
    </svg>
  );
}

export function ProjectTerminalIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="m7 11 2-2-2-2" />
      <path d="M11 13h4" />
      <rect height="18" rx="2" ry="2" width="18" x="3" y="3" />
    </svg>
  );
}

export function SettingsIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <circle cx="12" cy="12" r="3" />
      <path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z" />
    </svg>
  );
}

export function CloseIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="m6 6 12 12M18 6 6 18" />
    </svg>
  );
}

export function ActiveAgentsIcon() {
  return (
    <svg viewBox="0 0 24 24">
      <circle cx="12" cy="5" r="2.2" />
      <circle cx="6.5" cy="18" r="2.2" />
      <circle cx="17.5" cy="18" r="2.2" />
      <path d="M12 7.3v3.2M12 10.5 7.6 16.2M12 10.5l4.4 5.7" />
    </svg>
  );
}

export function SearchIcon() {
  return (
    <svg viewBox="0 0 24 24">
      <circle cx="11" cy="11" r="6.5" />
      <path d="m16 16 4 4" />
    </svg>
  );
}

export function CustomizationIcon() {
  return (
    <svg viewBox="4 7 15 17">
      <path d="M5 22H16V19A2.5 2.5 0 0 1 16 14V12H13A2.5 2.5 0 0 0 8 12H5V22Z" />
    </svg>
  );
}
type ProjectScrollThumb = {
  height: number;
  isVisible: boolean;
  top: number;
};
