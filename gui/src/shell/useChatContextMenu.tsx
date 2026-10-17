import { useEffect, useRef, useState } from "react";

const CHAT_CONTEXT_MENU_HEIGHT = 120;
const CHAT_CONTEXT_MENU_WIDTH = 200;
const PROJECT_CONTEXT_MENU_EDGE_GAP = 8;

type ChatContextMenu = {
  chatID: string;
  projectID: string;
  title: string;
  x: number;
  y: number;
};

type ChatDeletionDialog = {
  chatID: string;
  projectID: string;
  title: string;
};

export type OpenChatContextMenu = (projectID: string, chatID: string, title: string, x: number, y: number) => void;

export function useChatContextMenu({ onRenameChat, onDeleteChat }: {
  onRenameChat: (projectID: string, chatID: string, title: string) => Promise<void>;
  onDeleteChat: (projectID: string, chatID: string) => Promise<void>;
}) {
  const [chatContextMenu, setChatContextMenu] = useState<ChatContextMenu | null>(null);
  const [chatDeletionDialog, setChatDeletionDialog] = useState<ChatDeletionDialog | null>(null);
  const [chatDeletionError, setChatDeletionError] = useState("");
  const [isDeletingChat, setIsDeletingChat] = useState(false);
  const chatDeleteCancelRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!chatContextMenu) return;
    const closeMenu = () => setChatContextMenu(null);
    const closeOnEscape = (event: globalThis.KeyboardEvent) => {
      if (event.key === "Escape") closeMenu();
    };
    window.addEventListener("pointerdown", closeMenu);
    window.addEventListener("resize", closeMenu);
    window.addEventListener("keydown", closeOnEscape);
    return () => {
      window.removeEventListener("pointerdown", closeMenu);
      window.removeEventListener("resize", closeMenu);
      window.removeEventListener("keydown", closeOnEscape);
    };
  }, [chatContextMenu]);

  useEffect(() => {
    if (!chatDeletionDialog) return;

    const closeOnEscape = (event: globalThis.KeyboardEvent) => {
      if (event.key === "Escape" && !isDeletingChat) {
        setChatDeletionDialog(null);
        setChatDeletionError("");
      }
    };
    document.addEventListener("keydown", closeOnEscape);
    const focusTimer = !isDeletingChat ? window.setTimeout(() => chatDeleteCancelRef.current?.focus(), 0) : undefined;
    return () => {
      document.removeEventListener("keydown", closeOnEscape);
      if (focusTimer !== undefined) window.clearTimeout(focusTimer);
    };
  }, [chatDeletionDialog, isDeletingChat]);

  function openChatContextMenu(projectID: string, chatID: string, title: string, x: number, y: number) {
    setChatContextMenu({
      chatID,
      projectID,
      title,
      x: Math.max(PROJECT_CONTEXT_MENU_EDGE_GAP, Math.min(x, window.innerWidth - CHAT_CONTEXT_MENU_WIDTH - PROJECT_CONTEXT_MENU_EDGE_GAP)),
      y: Math.max(PROJECT_CONTEXT_MENU_EDGE_GAP, Math.min(y, window.innerHeight - CHAT_CONTEXT_MENU_HEIGHT - PROJECT_CONTEXT_MENU_EDGE_GAP)),
    });
  }

  async function copyChatID(chatID: string) {
    try {
      await navigator.clipboard.writeText(chatID);
    } catch {
      // The clipboard is unavailable on some non-secure browser previews.
      window.prompt("Copy chat ID", chatID);
    }
  }

  function renameChatFromContextMenu() {
    if (!chatContextMenu) return;
    const menu = chatContextMenu;
    const title = window.prompt("Thread name", menu.title)?.trim();
    setChatContextMenu(null);
    if (!title || title === menu.title) return;
    void onRenameChat(menu.projectID, menu.chatID, title).catch(() => {
      // The main app displays the request error.
    });
  }

  function deleteChatFromContextMenu() {
    if (!chatContextMenu) return;
    const menu = chatContextMenu;
    setChatContextMenu(null);
    setChatDeletionError("");
    setChatDeletionDialog({ chatID: menu.chatID, projectID: menu.projectID, title: menu.title });
  }

  function cancelChatDeletion() {
    if (isDeletingChat) return;
    setChatDeletionDialog(null);
    setChatDeletionError("");
  }

  async function confirmChatDeletion() {
    if (!chatDeletionDialog || isDeletingChat) return;
    const dialog = chatDeletionDialog;
    setIsDeletingChat(true);
    setChatDeletionError("");
    try {
      await onDeleteChat(dialog.projectID, dialog.chatID);
      setChatDeletionDialog(null);
    } catch (reason: unknown) {
      setChatDeletionError(reason instanceof Error ? reason.message : "Unable to delete chat");
    } finally {
      setIsDeletingChat(false);
    }
  }

  return { open: openChatContextMenu, menu: <>
      {chatContextMenu ? (
        <div
          aria-label={`Actions for ${chatContextMenu.title}`}
          className="project-context-menu"
          onPointerDown={(event) => event.stopPropagation()}
          role="menu"
          style={{ left: chatContextMenu.x, top: chatContextMenu.y }}
        >
          <button onClick={() => { void copyChatID(chatContextMenu.chatID); setChatContextMenu(null); }} role="menuitem" type="button">
            Copy chat ID
          </button>
          <button onClick={renameChatFromContextMenu} role="menuitem" type="button">
            Rename thread
          </button>
          <div className="project-context-menu-divider" role="separator" />
          <button className="is-danger" onClick={deleteChatFromContextMenu} role="menuitem" type="button">
            Delete chat
          </button>
        </div>
      ) : null}
      {chatDeletionDialog ? (
        <div
          className="chat-delete-dialog-backdrop"
          onPointerDown={(event) => {
            if (event.target === event.currentTarget) cancelChatDeletion();
          }}
          role="presentation"
        >
          <section
            aria-describedby="chat-sidebar-delete-dialog-description"
            aria-labelledby="chat-sidebar-delete-dialog-title"
            aria-modal="true"
            className="chat-delete-dialog"
            onPointerDown={(event) => event.stopPropagation()}
            role="dialog"
          >
            <div aria-hidden="true" className="chat-delete-dialog-marker">!</div>
            <p className="chat-delete-dialog-eyebrow">Delete confirmation</p>
            <h2 id="chat-sidebar-delete-dialog-title">Delete this chat?</h2>
            <p id="chat-sidebar-delete-dialog-description">
              This permanently removes <strong>{chatDeletionDialog.title}</strong> and all its messages. This action cannot be undone.
            </p>
            {chatDeletionError ? <p className="chat-delete-dialog-error" role="alert">{chatDeletionError}</p> : null}
            <div className="chat-delete-dialog-actions">
              <button disabled={isDeletingChat} onClick={cancelChatDeletion} ref={chatDeleteCancelRef} type="button">Cancel</button>
              <button className="is-danger" disabled={isDeletingChat} onClick={() => void confirmChatDeletion()} type="button">
                {isDeletingChat ? "Deleting…" : "Delete chat"}
              </button>
            </div>
          </section>
        </div>
      ) : null}
  </> };
}
