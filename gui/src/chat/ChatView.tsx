import { type CSSProperties, useEffect, useRef, useState } from "react";
import type { Chat, ChatMessage, ChatToolCall } from "./chatTypes";
import { ChatComposer } from "./ChatComposer";
import type { ComposerImageAttachment, ComposerTerminalClip } from "./composerTypes";
import { snapshotComposerImages } from "./chatClient";
import { useChatScroll } from "./useChatScroll";
import { ChatImageAttachments, ModeSwitchNotice, SubagentActivityIndicator } from "./ChatMessageParts";
import { ChatMessageGroups, SubagentChatPanel } from "./ChatMessageGroups";
import { indexChatMessages, liveWorkedForSeconds } from "./chatMessageUtils";
import { subagentIsActive, subagentStatus } from "./ChatToolActivity";
import { BranchIcon, CloseIcon, FolderIcon, WorktreeIcon } from "./ChatIcons";
import { MarkdownContent } from "./MarkdownContent";
import type { ActiveSubagent } from "./chatViewTypes";
import "./chat.css";
import "./chat-timeline.css";
import "./chat-compaction.css";

const MODE_SWITCH_DURATION_MS = 5000;
const WORKED_FOR_TICK_MS = 250;
export { ChatTopbar } from "./ChatTopbar";

type OpenSubagentRequest = {
  chatID: string;
  status?: string;
  subchatID: string;
  task?: string;
  title?: string;
  toolID?: string;
};

type ChatViewProps = {
  bottomInset?: number;
  chat: Chat;
  isStreaming?: boolean;
  onDeleteMessage: (chatID: string, messageID: string) => void;
  onOpenedSubagentRequest?: () => void;
  onSend: (chatID: string, message: ChatMessage) => void;
  onStopTool: (chatID: string, messageID: string, toolID: string) => void;
  onStopStreaming: (chatID: string) => void;
  openSubagentRequest?: OpenSubagentRequest | null;
  loadSubchat?: (subchatID: string) => Promise<ChatMessage[]>;
  pendingUserMessageIDs?: ReadonlySet<string>;
  branch?: string;
  worktree?: string;
  workspaceName?: string;
  workspacePath?: string;
};

export function ChatView({ bottomInset = 0, branch, chat, isStreaming = false, loadSubchat, onDeleteMessage, onOpenedSubagentRequest, onSend, onStopTool, onStopStreaming, openSubagentRequest, pendingUserMessageIDs = new Set(), worktree, workspaceName, workspacePath }: ChatViewProps) {
  const [deleteTarget, setDeleteTarget] = useState<ChatMessage | null>(null);
  const [composerMode, setComposerMode] = useState<"agent" | "chat">(chat.modeSwitchTarget ? "chat" : "agent");
  const [isModeSwitchPending, setIsModeSwitchPending] = useState(Boolean(chat.modeSwitchTarget));
  const [modeSwitchProgress, setModeSwitchProgress] = useState(0);
  const [openSubagent, setOpenSubagent] = useState<{ messageID: string; toolID: string } | null>(null);
  const [forcedSubagent, setForcedSubagent] = useState<OpenSubagentRequest | null>(null);
  const [subchatMessages, setSubchatMessages] = useState<ChatMessage[] | null>(null);
  const [isSubchatLoading, setIsSubchatLoading] = useState(false);
  const [isSubagentIndicatorExpanded, setIsSubagentIndicatorExpanded] = useState(false);
  const [clockNow, setClockNow] = useState(() => Date.now());
  const viewRef = useRef<HTMLElement>(null);
  const composerDockRef = useRef<HTMLDivElement>(null);
  const composerRef = useRef<HTMLFormElement>(null);
  const deleteCancelRef = useRef<HTMLButtonElement>(null);
  const lastMessageContent = chat.messages.at(-1)?.content ?? "";
  const lastMessageReasoning = chat.messages.at(-1)?.reasoning ?? "";
  const lastMessageStatus = chat.messages.at(-1)?.status ?? "";
  const lastMessageThoughtFor = chat.messages.at(-1)?.thoughtFor ?? null;
  const lastMessageWorkedFor = chat.messages.at(-1)?.workedFor ?? null;
  const pendingMessageKey = [...pendingUserMessageIDs].join("-");
  const isChatWorking = isStreaming || chat.status === "running";
  const scrollContentKey = [
    chat.messages.length,
    lastMessageContent,
    lastMessageReasoning,
    lastMessageStatus,
    lastMessageThoughtFor,
    lastMessageWorkedFor,
    pendingMessageKey,
  ].join("\u0000");
  const indexedMessages = indexChatMessages(chat.messages);
  const pendingMessages = indexedMessages.filter(({ message }) => pendingUserMessageIDs.has(message.id));
  const visibleMessages = indexedMessages.filter(({ message }) => !pendingUserMessageIDs.has(message.id));
  const liveWorkedFor = isChatWorking ? liveWorkedForSeconds(chat, clockNow) : undefined;
  const { messagesRef, messagesShellRef, onMessagesKeyDown, onMessagesPointerDown, onMessagesScroll, onMessagesWheel } = useChatScroll({
    bottomInset,
    chatID: chat.id,
    composerDockRef,
    composerRef,
    contentKey: scrollContentKey,
    viewRef,
  });
  const activeSubagents: ActiveSubagent[] = chat.messages.flatMap((message) => (
    (message.toolCalls ?? [])
      .filter((tool) => tool.name === "subagent" && !tool.sync && subagentIsActive(subagentStatus(tool)))
      .map((tool) => ({ messageID: message.id, tool }))
  ));
  const matchedOpenTool = openSubagent
    ? chat.messages.find((message) => message.id === openSubagent.messageID)?.toolCalls?.find((tool) => tool.id === openSubagent.toolID)
    : undefined;
  const openSubagentTool = matchedOpenTool ?? (forcedSubagent ? syntheticSubagentTool(forcedSubagent) : undefined);
  const openSubchatID = openSubagentTool?.result?.subchatId;
  const openSubagentState = openSubagentTool ? subagentStatus(openSubagentTool) : "";

  useEffect(() => {
    setOpenSubagent(null);
    setForcedSubagent(null);
    setIsSubagentIndicatorExpanded(false);
  }, [chat.id]);

  useEffect(() => {
    if (!openSubagentRequest?.subchatID || openSubagentRequest.chatID !== chat.id) return;
    const found = findSubagentTool(chat.messages, openSubagentRequest);
    if (found) {
      setOpenSubagent(found);
      setForcedSubagent(null);
    } else {
      setForcedSubagent(openSubagentRequest);
    }
    onOpenedSubagentRequest?.();
  }, [chat.id, openSubagentRequest]);

  useEffect(() => {
    if (!isChatWorking) return;
    const tick = () => setClockNow(Date.now());
    tick();
    const timer = window.setInterval(tick, WORKED_FOR_TICK_MS);
    return () => window.clearInterval(timer);
  }, [chat.id, isChatWorking]);

  useEffect(() => {
    if (!openSubagentTool || !openSubchatID || !loadSubchat) {
      setSubchatMessages(null);
      setIsSubchatLoading(false);
      return;
    }
    let cancelled = false;
    let refreshInFlight = false;
    let isInitialLoad = true;
    setSubchatMessages(null);
    setIsSubchatLoading(true);
    const refresh = async () => {
      if (cancelled || refreshInFlight) return;
      refreshInFlight = true;
      try {
        const messages = await loadSubchat(openSubchatID);
        if (!cancelled) setSubchatMessages(messages);
      } catch {
        if (!cancelled && isInitialLoad) setSubchatMessages([]);
      } finally {
        refreshInFlight = false;
        if (!cancelled && isInitialLoad) {
          isInitialLoad = false;
          setIsSubchatLoading(false);
        }
      }
    };
    void refresh();
    const pollID = subagentIsActive(openSubagentState) ? window.setInterval(() => void refresh(), 1000) : undefined;
    return () => {
      cancelled = true;
      if (pollID !== undefined) window.clearInterval(pollID);
    };
  }, [loadSubchat, openSubagentState, openSubchatID]);

  useEffect(() => {
    if (activeSubagents.length === 0) setIsSubagentIndicatorExpanded(false);
  }, [activeSubagents.length]);

  useEffect(() => {
    if (chat.modeSwitchTarget !== "agent") {
      setComposerMode("agent");
      setIsModeSwitchPending(false);
      setModeSwitchProgress(0);
      return;
    }

    setComposerMode("chat");
    setIsModeSwitchPending(true);
    setModeSwitchProgress(0);
    const startedAt = performance.now();
    const progressTimer = window.setInterval(() => {
      setModeSwitchProgress(Math.min(1, (performance.now() - startedAt) / MODE_SWITCH_DURATION_MS));
    }, 50);
    const completeTimer = window.setTimeout(() => {
      setModeSwitchProgress(1);
      setComposerMode("agent");
      setIsModeSwitchPending(false);
    }, MODE_SWITCH_DURATION_MS);

    return () => {
      window.clearInterval(progressTimer);
      window.clearTimeout(completeTimer);
    };
  }, [chat.id, chat.modeSwitchTarget]);

  useEffect(() => {
    if (!deleteTarget) return;
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setDeleteTarget(null);
    };
    document.addEventListener("keydown", closeOnEscape);
    const focusTimer = window.setTimeout(() => deleteCancelRef.current?.focus(), 0);
    return () => {
      document.removeEventListener("keydown", closeOnEscape);
      window.clearTimeout(focusTimer);
    };
  }, [deleteTarget]);

  async function sendFromComposer(content: string, composerImages: ComposerImageAttachment[], composerClips: ComposerTerminalClip[] = []) {
    const messageImages = await snapshotComposerImages(composerImages);
    onSend(chat.id, {
      content,
      createdAt: Date.now(),
      clips: composerClips,
      id: `user-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
      images: messageImages,
      role: "user",
    });
  }

  function cancelModeSwitch() {
    setIsModeSwitchPending(false);
    setModeSwitchProgress(0);
    setComposerMode("chat");
  }

  return (
    <section aria-label={`Chat: ${chat.title}`} className="chat-view" ref={viewRef} style={{ bottom: Math.max(0, bottomInset) } as CSSProperties}>
      <div aria-live="polite" className="chat-messages-shell" onKeyDown={onMessagesKeyDown} onPointerDown={onMessagesPointerDown} onScroll={onMessagesScroll} onWheel={onMessagesWheel} ref={messagesShellRef} tabIndex={0}>
        <div className="chat-messages" ref={messagesRef}>
          {chat.messages.length ? (
            <ChatMessageGroups
              liveWorkedFor={liveWorkedFor}
              messages={visibleMessages}
              onOpenSubagent={(messageID, toolID) => setOpenSubagent({ messageID, toolID })}
              onRequestDelete={setDeleteTarget}
              onStopTool={(messageID, toolID) => onStopTool(chat.id, messageID, toolID)}
            />
          ) : <p className="chat-empty">This chat is ready for the first message.</p>}
        </div>
      </div>
		{openSubagentTool && openSubchatID ? <SubagentChatPanel isLoading={isSubchatLoading} messages={subchatMessages ?? undefined} onCollapse={() => { setOpenSubagent(null); setForcedSubagent(null); }} tool={openSubagentTool} /> : null}
      <div className="welcome-composer-dock chat-composer-dock" ref={composerDockRef}>
        {pendingMessages.length ? (
          <div className="chat-pending-messages">
            {pendingMessages.map(({ message }) => (
              <div className="chat-pending-turn chat-turn is-user" key={message.id}>
              <article className="chat-message chat-pending-message is-user">
                {message.images?.length ? <ChatImageAttachments images={message.images} /> : null}
                <MarkdownContent content={message.content} />
              </article>
            </div>
            ))}
          </div>
        ) : null}
        {activeSubagents.length ? (
          <SubagentActivityIndicator
            isExpanded={isSubagentIndicatorExpanded}
            onOpenSubagent={(messageID, toolID) => setOpenSubagent({ messageID, toolID })}
            onToggle={() => setIsSubagentIndicatorExpanded((current) => !current)}
            subagents={activeSubagents}
          />
        ) : null}
        {isModeSwitchPending ? <ModeSwitchNotice progress={modeSwitchProgress} onCancel={cancelModeSwitch} /> : null}
        <div aria-hidden="true" className="chat-composer-background" />
        <div className="chat-composer">
          <ChatComposer
            aria-label="Message"
            formRef={composerRef}
            initialMode={composerMode}
            mode={composerMode}
            modeSwitchPending={isModeSwitchPending}
            onModeChange={(nextMode) => {
              if (!isModeSwitchPending) setComposerMode(nextMode);
            }}
            onSend={sendFromComposer}
            onStopStreaming={() => onStopStreaming(chat.id)}
            projectID={chat.projectID}
            resetKey={chat.id}
            isStreaming={isStreaming}
          />
        </div>
        <div aria-label="Read-only Git context" className="welcome-git-controls">
          {workspaceName ? <span className="chat-readonly-control is-workspace" title={workspacePath}><FolderIcon />{workspaceName}</span> : null}
          <span className="chat-readonly-control"><BranchIcon />{branch ?? chat.branch ?? "main"}</span>
          <span className="chat-readonly-control"><WorktreeIcon />{worktree ?? chat.worktree ?? "Worktree"}</span>
        </div>
      </div>
      {deleteTarget ? (
        <div
          className="chat-delete-dialog-backdrop"
          onPointerDown={(event) => {
            if (event.target === event.currentTarget) setDeleteTarget(null);
          }}
          role="presentation"
        >
          <section
            aria-describedby="chat-delete-dialog-description"
            aria-labelledby="chat-delete-dialog-title"
            aria-modal="true"
            className="chat-delete-dialog"
            onPointerDown={(event) => event.stopPropagation()}
            role="dialog"
          >
            <div aria-hidden="true" className="chat-delete-dialog-marker"><CloseIcon /></div>
            <p className="chat-delete-dialog-eyebrow">Delete confirmation</p>
            <h2 id="chat-delete-dialog-title">Delete this message?</h2>
            <p id="chat-delete-dialog-description">The assistant's reply will also be deleted. This action cannot be undone.</p>
            <div className="chat-delete-dialog-actions">
              <button ref={deleteCancelRef} onClick={() => setDeleteTarget(null)} type="button">Cancel</button>
              <button
                className="is-danger"
                onClick={() => {
                  onDeleteMessage(chat.id, deleteTarget.id);
                  setDeleteTarget(null);
                }}
                type="button"
              >
                Delete message
              </button>
            </div>
          </section>
        </div>
      ) : null}
    </section>
  );
}

function findSubagentTool(messages: ChatMessage[], request: OpenSubagentRequest): { messageID: string; toolID: string } | null {
  for (const message of messages) {
    const tool = message.toolCalls?.find((candidate) => candidate.result?.subchatId === request.subchatID || (request.toolID && candidate.id === request.toolID));
    if (tool) return { messageID: message.id, toolID: tool.id };
  }
  return null;
}

function syntheticSubagentTool(request: OpenSubagentRequest): ChatToolCall {
  const running = request.status === "running" || request.status === "queued";
  return {
    id: request.toolID || request.subchatID,
    input: request.task || request.title,
    name: "subagent",
    result: {
      subchatId: request.subchatID,
      subchatStatus: request.status === "cancelled" || request.status === "done" || request.status === "paused" || request.status === "queued" || request.status === "running"
        ? request.status
        : "running",
      status: running ? "success" : request.status === "done" ? "success" : "error",
    },
    status: running ? "running" : request.status === "done" ? "success" : "error",
  };
}
