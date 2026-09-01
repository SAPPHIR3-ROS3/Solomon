import { isValidElement, type CSSProperties, type MouseEvent, type ReactNode, useEffect, useLayoutEffect, useRef, useState } from "react";
import { go } from "@codemirror/lang-go";
import { HighlightStyle, syntaxHighlighting } from "@codemirror/language";
import { EditorState } from "@codemirror/state";
import { EditorView as CodeMirrorView, lineNumbers } from "@codemirror/view";
import { tags } from "@lezer/highlight";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import type { Chat, ChatImage, ChatMessage, ChatStats, ChatToolCall, ChatToolResult } from "./chatTypes";
import { ChatComposer, ComposerCrownIcon } from "./ChatComposer";
import type { ComposerImageAttachment } from "./composerTypes";
import { snapshotComposerImages } from "./chatClient";
import "./chat.css";

const MESSAGE_START_TIME = new Date("2026-01-01T09:00:00").getTime();
const MODE_SWITCH_DURATION_MS = 5000;
const PULSE_DURATION_MS = 1150;
const MISSING_TOOL_INTENT_LABEL = "Intent missing";
const WORKED_FOR_TICK_MS = 250;

const codeModeHighlightStyle = HighlightStyle.define([
  { tag: [tags.keyword, tags.controlKeyword, tags.modifier, tags.operatorKeyword], color: "#77ddd1" },
  { tag: [tags.typeName, tags.className, tags.namespace, tags.definition(tags.typeName)], color: "#7ee0d5" },
  { tag: [tags.function(tags.variableName), tags.labelName], color: "#e8aa76" },
  { tag: [tags.string, tags.special(tags.string)], color: "#e79be1" },
  { tag: [tags.number, tags.bool, tags.null], color: "#e7c15f" },
  { tag: tags.comment, color: "#7f8981" },
  { tag: [tags.propertyName, tags.variableName], color: "#e1e4db" },
  { tag: [tags.operator, tags.punctuation], color: "#eed85b" },
]);

function synchronizedPulseDelay() {
  return `${-(performance.now() % PULSE_DURATION_MS)}ms`;
}

function displayToolIntent(tool: ChatToolCall) {
  return tool.intent?.trim() || MISSING_TOOL_INTENT_LABEL;
}

function toolIntentClassName(tool: ChatToolCall) {
  return `chat-tool-intent${tool.intent?.trim() ? "" : " is-missing"}`;
}

type CheckpointMetadata = {
  branch: string;
  label: string;
  sequence: number;
};

type IndexedChatMessage = {
  checkpoint?: CheckpointMetadata;
  index: number;
  message: ChatMessage;
  toolMessageIDs: ReadonlyMap<string, string>;
};

type ActiveSubagent = {
  messageID: string;
  tool: ChatToolCall;
};

type ChatViewProps = {
  bottomInset?: number;
  chat: Chat;
  isStreaming?: boolean;
  onDeleteMessage: (chatID: string, messageID: string) => void;
  onSend: (chatID: string, message: ChatMessage) => void;
  onStopTool: (chatID: string, messageID: string, toolID: string) => void;
  onStopStreaming: (chatID: string) => void;
  loadSubchat?: (subchatID: string) => Promise<ChatMessage[]>;
  pendingUserMessageIDs?: ReadonlySet<string>;
  branch?: string;
  worktree?: string;
  workspaceName?: string;
  workspacePath?: string;
};

export function ChatView({ bottomInset = 0, branch, chat, isStreaming = false, loadSubchat, onDeleteMessage, onSend, onStopTool, onStopStreaming, pendingUserMessageIDs = new Set(), worktree, workspaceName, workspacePath }: ChatViewProps) {
  const [deleteTarget, setDeleteTarget] = useState<ChatMessage | null>(null);
  const [composerMode, setComposerMode] = useState<"agent" | "chat">(chat.modeSwitchTarget ? "chat" : "agent");
  const [isModeSwitchPending, setIsModeSwitchPending] = useState(Boolean(chat.modeSwitchTarget));
  const [modeSwitchProgress, setModeSwitchProgress] = useState(0);
  const [openSubagent, setOpenSubagent] = useState<{ messageID: string; toolID: string } | null>(null);
  const [subchatMessages, setSubchatMessages] = useState<ChatMessage[] | null>(null);
  const [isSubchatLoading, setIsSubchatLoading] = useState(false);
  const [isSubagentIndicatorExpanded, setIsSubagentIndicatorExpanded] = useState(false);
  const [clockNow, setClockNow] = useState(() => Date.now());
  const viewRef = useRef<HTMLElement>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
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
  const [pulseDelay] = useState(() => synchronizedPulseDelay());
  const indexedMessages = indexChatMessages(chat.messages);
  const pendingMessages = indexedMessages.filter(({ message }) => pendingUserMessageIDs.has(message.id));
  const visibleMessages = indexedMessages.filter(({ message }) => !pendingUserMessageIDs.has(message.id));
  const liveWorkedFor = isChatWorking ? liveWorkedForSeconds(chat, clockNow) : undefined;
  const activeSubagents: ActiveSubagent[] = chat.messages.flatMap((message) => (
    (message.toolCalls ?? [])
      .filter((tool) => tool.name === "subagent" && !tool.sync && subagentIsActive(subagentStatus(tool)))
      .map((tool) => ({ messageID: message.id, tool }))
  ));
  const openSubagentTool = openSubagent
    ? chat.messages.find((message) => message.id === openSubagent.messageID)?.toolCalls?.find((tool) => tool.id === openSubagent.toolID)
    : undefined;
  const openSubchatID = openSubagentTool?.result?.subchatId;
  const openSubagentState = openSubagentTool ? subagentStatus(openSubagentTool) : "";

  useEffect(() => {
    setOpenSubagent(null);
    setIsSubagentIndicatorExpanded(false);
  }, [chat.id]);

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
    messagesEndRef.current?.scrollIntoView({ block: "end" });
  }, [chat.id, chat.messages.length, lastMessageContent, lastMessageReasoning, lastMessageStatus, lastMessageThoughtFor, lastMessageWorkedFor, pendingMessageKey]);

  useLayoutEffect(() => {
    const view = viewRef.current;
    const dock = composerDockRef.current;
    const composer = composerRef.current;
    if (!view || !dock || !composer) return;
    const measureDock = () => {
      const dockRect = dock.getBoundingClientRect();
      const composerRect = composer.getBoundingClientRect();
      view.style.setProperty("--chat-composer-dock-height", `${Math.ceil(dockRect.height)}px`);
      view.style.setProperty("--chat-composer-surface-top", `${Math.max(0, composerRect.top - dockRect.top)}px`);
    };
    const observer = new ResizeObserver(measureDock);
    observer.observe(dock);
    observer.observe(composer);
    measureDock();
    return () => observer.disconnect();
  }, [activeSubagents.length, isModeSwitchPending, isSubagentIndicatorExpanded, pendingMessageKey]);

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

  async function sendFromComposer(content: string, composerImages: ComposerImageAttachment[]) {
    const messageImages = await snapshotComposerImages(composerImages);
    onSend(chat.id, {
      content,
      createdAt: Date.now(),
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
    <section aria-label={`Chat: ${chat.title}`} className="chat-view" ref={viewRef} style={{ "--chat-pulse-delay": pulseDelay, bottom: Math.max(0, bottomInset) } as CSSProperties}>
      <div aria-live="polite" className="chat-messages-shell">
        <div className="chat-messages">
          {chat.messages.length ? (
            <ChatMessageGroups
              liveWorkedFor={liveWorkedFor}
              messages={visibleMessages}
              onOpenSubagent={(messageID, toolID) => setOpenSubagent({ messageID, toolID })}
              onRequestDelete={setDeleteTarget}
              onStopTool={(messageID, toolID) => onStopTool(chat.id, messageID, toolID)}
            />
          ) : <p className="chat-empty">This chat is ready for the first message.</p>}
          <div ref={messagesEndRef} />
        </div>
      </div>
		{openSubagentTool && openSubchatID ? <SubagentChatPanel isLoading={isSubchatLoading} messages={subchatMessages ?? undefined} onCollapse={() => setOpenSubagent(null)} tool={openSubagentTool} /> : null}
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

function CheckpointLabel({ className, label }: { className?: string; label: string }) {
  return (
    <span aria-label={`Checkpoint ${label}`} className={`chat-checkpoint-label${className ? ` ${className}` : ""}`} title={`Checkpoint ${label}`}>
      {label}
    </span>
  );
}

function InterruptedGenerationMarker() {
  return (
    <div aria-label="Generation stopped" className="chat-interrupted" role="status">
      <span aria-hidden="true" className="chat-interrupted-line is-left" />
      <span className="chat-interrupted-label">generation stopped</span>
      <span aria-hidden="true" className="chat-interrupted-line is-right" />
    </div>
  );
}

function ChatImageAttachments({ images }: { images: ChatImage[] }) {
  const [selectedImageIndex, setSelectedImageIndex] = useState<number | null>(null);
  const selectedImage = selectedImageIndex === null ? undefined : images[selectedImageIndex];

  useEffect(() => {
    if (selectedImageIndex === null) return;
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setSelectedImageIndex(null);
      if (images.length < 2) return;
      if (event.key === "ArrowLeft" || event.key === "ArrowRight") {
        event.preventDefault();
        setSelectedImageIndex((current) => {
          if (current === null) return current;
          const offset = event.key === "ArrowLeft" ? -1 : 1;
          return (current + offset + images.length) % images.length;
        });
      }
    };
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    document.addEventListener("keydown", closeOnEscape);
    return () => {
      document.body.style.overflow = previousOverflow;
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, [images.length, selectedImageIndex]);

  useEffect(() => {
    if (selectedImageIndex !== null && selectedImageIndex >= images.length) setSelectedImageIndex(null);
  }, [images.length, selectedImageIndex]);

  function moveSelectedImage(offset: number) {
    setSelectedImageIndex((current) => current === null ? current : (current + offset + images.length) % images.length);
  }

  return (
    <>
      <div aria-label="Attached images" className="composer-image-previews chat-message-images">
        {images.map((image, index) => (
          <figure
            className="composer-image-preview"
            key={`${image.name}-${index}`}
            onClick={() => setSelectedImageIndex(index)}
            onKeyDown={(event) => {
              if (event.key === "Enter" || event.key === " ") {
                event.preventDefault();
                setSelectedImageIndex(index);
              }
            }}
            role="button"
            tabIndex={0}
          >
            <img alt={`Open preview of ${image.name}`} src={image.url} />
          </figure>
        ))}
      </div>
      {selectedImage ? (
        <div
          aria-label={`Image preview: ${selectedImage.name}`}
          aria-modal="true"
          className="composer-image-lightbox chat-message-lightbox"
          onClick={() => setSelectedImageIndex(null)}
          role="dialog"
        >
          <button aria-label="Close preview" className="chat-image-lightbox-close" onClick={() => setSelectedImageIndex(null)} type="button"><CloseIcon /></button>
          {images.length > 1 ? (
            <button aria-label="Previous image" className="composer-image-lightbox-nav is-previous" onClick={(event) => { event.stopPropagation(); moveSelectedImage(-1); }} type="button">
              <ChatImageArrowIcon direction="left" />
            </button>
          ) : null}
          <div className="composer-image-lightbox-stage" onClick={(event) => event.stopPropagation()}>
            <img alt={selectedImage.name} className="composer-image-lightbox-image" src={selectedImage.url} />
          </div>
          {images.length > 1 ? (
            <button aria-label="Next image" className="composer-image-lightbox-nav is-next" onClick={(event) => { event.stopPropagation(); moveSelectedImage(1); }} type="button">
              <ChatImageArrowIcon direction="right" />
            </button>
          ) : null}
          {images.length > 1 ? <div className="composer-image-lightbox-count chat-image-lightbox-count">{selectedImageIndex! + 1} / {images.length}</div> : null}
        </div>
      ) : null}
    </>
  );
}

function ChatImageArrowIcon({ direction }: { direction: "left" | "right" }) {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d={direction === "left" ? "m14 6-6 6 6 6" : "m10 6 6 6-6 6"} />
    </svg>
  );
}

function CompactionCard({ message }: { message: ChatMessage }) {
  const retainedMessages = message.retainedMessages ?? [];

  return (
    <section aria-label="Context compaction" className="chat-compaction" data-message-kind="compaction">
      <details>
        <summary className="chat-compaction-summary">
          <span className="chat-compaction-title">Context compacted</span>
          <svg aria-hidden="true" className="chat-compaction-chevron" viewBox="0 0 24 24">
            <path d="m7 10 5 5 5-5" />
          </svg>
        </summary>
        <div className="chat-compaction-body">
          <details className="chat-compaction-section" open>
            <summary className="chat-compaction-section-summary">
              <span className="chat-compaction-eyebrow">Summary</span>
              <svg aria-hidden="true" className="chat-compaction-section-chevron" viewBox="0 0 24 24">
                <path d="m7 10 5 5 5-5" />
              </svg>
            </summary>
            <div className="chat-compaction-section-body">
              <div className="chat-compaction-markdown">
                <MarkdownContent content={message.summary ?? ""} />
              </div>
            </div>
          </details>
          <details className="chat-compaction-section" open>
            <summary className="chat-compaction-section-summary">
              <span className="chat-compaction-eyebrow">Recent messages</span>
              <svg aria-hidden="true" className="chat-compaction-section-chevron" viewBox="0 0 24 24">
                <path d="m7 10 5 5 5-5" />
              </svg>
            </summary>
            <div className="chat-compaction-section-body">
              <div className="chat-retained-messages">
                {retainedMessages.map((retainedMessage, index) => (
                  <div className={`chat-retained-message is-${retainedMessage.role}`} key={`${retainedMessage.role}-${index}`}>
                    <div className="chat-retained-content">
                      {retainedMessage.images?.length ? <ChatImageAttachments images={retainedMessage.images} /> : null}
                      <MarkdownContent content={retainedMessage.content} />
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </details>
        </div>
      </details>
    </section>
  );
}

function ModeSwitchNotice({ onCancel, progress }: { onCancel: () => void; progress: number }) {
  const percentage = Math.round(Math.max(0, Math.min(1, progress)) * 100);

  return (
    <section aria-label="Switching to Agent mode" className="chat-mode-switch" aria-live="polite">
      <div className="chat-mode-switch-header">
        <div className="chat-mode-switch-copy">
          <span aria-hidden="true" className="chat-mode-switch-icon"><ComposerCrownIcon /></span>
          <span>Switching to Agent mode</span>
        </div>
        <button className="chat-mode-switch-cancel" onClick={onCancel} type="button">Stay in Chat</button>
      </div>
      <div aria-label={`${percentage}% complete`} aria-valuemax={100} aria-valuemin={0} aria-valuenow={percentage} className="chat-mode-switch-progress" role="progressbar">
        <span style={{ transform: `scaleX(${progress})` }} />
      </div>
  </section>
  );
}

function SubagentActivityIndicator({ isExpanded, onOpenSubagent, onToggle, subagents }: { isExpanded: boolean; onOpenSubagent: (messageID: string, toolID: string) => void; onToggle: () => void; subagents: ActiveSubagent[] }) {
  const count = subagents.length;

  return (
    <section aria-label={`${count} subagents working`} className={`chat-subagent-indicator${isExpanded ? " is-expanded" : ""}`}>
      <button aria-expanded={isExpanded} className="chat-subagent-indicator-toggle" onClick={onToggle} type="button">
        <i aria-hidden="true" className="chat-subagent-indicator-dot" />
        <span className="chat-subagent-indicator-label">
          <span className="chat-subagent-indicator-count">{count}</span>
          <span>subagents working</span>
        </span>
        <svg aria-hidden="true" className="chat-subagent-indicator-chevron" viewBox="0 0 24 24">
          <path d="m7 10 5 5 5-5" />
        </svg>
      </button>
      {isExpanded ? (
        <div className="chat-subagent-indicator-list">
          {subagents.map(({ messageID, tool }) => (
            <button
              aria-label={`Open subagent chat: ${tool.input ?? "Untitled subagent"}`}
              className="chat-subagent-indicator-item"
              key={tool.id}
              onClick={() => onOpenSubagent(messageID, tool.id)}
              type="button"
            >
              <i aria-hidden="true" className="chat-subagent-indicator-item-dot" />
              <span>{tool.input ?? "Untitled subagent"}</span>
            </button>
          ))}
        </div>
      ) : null}
    </section>
  );
}

type ChatMessageGroupHandlers = {
  onOpenSubagent?: (messageID: string, toolID: string) => void;
  onRequestDelete?: (message: ChatMessage) => void;
  onStopTool?: (messageID: string, toolID: string) => void;
};

function ChatMessageGroups({ liveWorkedFor, messages, onOpenSubagent, onRequestDelete, onStopTool }: { liveWorkedFor?: number; messages: IndexedChatMessage[] } & ChatMessageGroupHandlers) {
  const groups = groupChatTurns(messages);
  const lastAssistantGroupIndex = groups.reduce((lastIndex, entries, groupIndex) => (
    entries[0]?.message.role === "assistant" ? groupIndex : lastIndex
  ), -1);

  return (
    <>
      {groups.map((entries, groupIndex) => {
        const first = entries[0];
        if (first.message.kind === "compaction") return <CompactionCard key={first.message.id} message={first.message} />;

        if (first.message.role === "assistant") {
          const last = entries[entries.length - 1];
          const footerMessage = assistantFooterMessage(entries);
          const activeWorkedFor = liveWorkedFor;
          const shouldShowWorkedFor = groupIndex === lastAssistantGroupIndex;
          return (
            <div className="chat-turn is-assistant" key={first.message.id}>
              {entries.map((entry) => {
                const actions = messageActions(entry, { onOpenSubagent, onStopTool });
                return <AssistantMessageBlock checkpoint={entry.checkpoint} key={entry.message.id} message={entry.message} onOpenSubagent={actions.onOpenSubagent} onStopTool={actions.onStopTool} />;
              })}
              <MessageFooter index={last.index} message={footerMessage} />
              {shouldShowWorkedFor && (activeWorkedFor !== undefined || footerMessage.workedFor !== undefined) ? (
                <WorkedForCounter isLive={activeWorkedFor !== undefined} seconds={activeWorkedFor ?? footerMessage.workedFor!} />
              ) : null}
            </div>
          );
        }

        const actions = messageActions(first, { onOpenSubagent, onRequestDelete, onStopTool });
        return <ChatMessageTurn checkpoint={first.checkpoint} index={first.index} key={first.message.id} message={first.message} onOpenSubagent={actions.onOpenSubagent} onRequestDelete={actions.onRequestDelete} onStopTool={actions.onStopTool} />;
      })}
    </>
  );
}

function messageActions(entry: IndexedChatMessage, handlers: ChatMessageGroupHandlers) {
  const { message, toolMessageIDs } = entry;
  return {
    onOpenSubagent: handlers.onOpenSubagent
      ? (tool: ChatToolCall) => handlers.onOpenSubagent?.(toolMessageIDs.get(tool.id) ?? message.id, tool.id)
      : undefined,
    onRequestDelete: handlers.onRequestDelete && message.role === "user"
      ? () => handlers.onRequestDelete?.(message)
      : undefined,
    onStopTool: handlers.onStopTool
      ? (toolID: string) => handlers.onStopTool?.(toolMessageIDs.get(toolID) ?? message.id, toolID)
      : undefined,
  };
}

function AssistantMessageBlock({ checkpoint, message, onOpenSubagent, onStopTool }: { checkpoint?: CheckpointMetadata; message: ChatMessage; onOpenSubagent?: (tool: ChatToolCall) => void; onStopTool?: (toolID: string) => void }) {
  return (
    <div className="chat-assistant-segment">
      <ChatMessageBody checkpoint={checkpoint} message={message} onOpenSubagent={onOpenSubagent} onStopTool={onStopTool} />
      {message.status === "interrupted" ? <InterruptedGenerationMarker /> : null}
    </div>
  );
}

function ChatMessageTurn({ checkpoint, index, message, onOpenSubagent, onRequestDelete, onStopTool }: { checkpoint?: CheckpointMetadata; index: number; message: ChatMessage; onOpenSubagent?: (tool: ChatToolCall) => void; onRequestDelete?: () => void; onStopTool?: (toolID: string) => void }) {
  return (
    <div className={`chat-turn is-${message.role}`} data-checkpoint={checkpoint?.label}>
      <ChatMessageBody checkpoint={checkpoint} message={message} onOpenSubagent={onOpenSubagent} onStopTool={onStopTool} />
      {message.status === "interrupted" ? <InterruptedGenerationMarker /> : null}
      <MessageFooter index={index} message={message} onRequestDelete={onRequestDelete} />
    </div>
  );
}

function ChatMessageBody({ checkpoint, message, onOpenSubagent, onStopTool }: { checkpoint?: CheckpointMetadata; message: ChatMessage; onOpenSubagent?: (tool: ChatToolCall) => void; onStopTool?: (toolID: string) => void }) {
  const [isReasoningCollapsed, setIsReasoningCollapsed] = useState(false);
  const thoughtFor = message.thoughtFor ?? message.stats?.ttftSeconds;
  const canCollapseReasoning = message.role === "assistant" && Boolean(message.reasoning || (thoughtFor !== undefined && thoughtFor > 0));

  function handleMessageClick(event: MouseEvent<HTMLElement>) {
    if (!canCollapseReasoning) return;
    if (event.target instanceof HTMLElement && event.target.closest("a, button, input, textarea, select, summary")) return;
    setIsReasoningCollapsed((current) => !current);
  }

  return (
    <article className={`chat-message is-${message.role}`} onClick={canCollapseReasoning ? handleMessageClick : undefined}>
      {checkpoint && !(message.role === "assistant" && message.toolCalls?.length) ? <CheckpointLabel label={checkpoint.label} /> : null}
      {message.images?.length ? <ChatImageAttachments images={message.images} /> : null}
      {canCollapseReasoning ? <ReasoningBlock isCollapsed={isReasoningCollapsed} message={message} onToggle={() => setIsReasoningCollapsed((current) => !current)} /> : null}
      {message.toolCalls?.length ? <ToolActivity onOpenSubagent={onOpenSubagent} onStopTool={onStopTool} toolCalls={message.toolCalls} /> : null}
      <MarkdownContent content={message.content} />
    </article>
  );
}

function ToolActivity({ onOpenSubagent, onStopTool, toolCalls }: { onOpenSubagent?: (tool: ChatToolCall) => void; onStopTool?: (toolID: string) => void; toolCalls: ChatToolCall[] }) {
  const [isCollapsed, setIsCollapsed] = useState(() => !toolCalls.some((tool) => tool.name === "orchestrate"));
  const activityRef = useRef<HTMLElement>(null);
  const shouldAnchorOnCollapseRef = useRef(false);
  const collapseLabel = isCollapsed ? `Show ${toolCalls.length} tool calls` : "Collapse tool calls";

  useLayoutEffect(() => {
    if (!isCollapsed || !shouldAnchorOnCollapseRef.current) return;
    shouldAnchorOnCollapseRef.current = false;
    activityRef.current?.scrollIntoView({ block: "start", behavior: "auto" });
  }, [isCollapsed]);

  function toggleCollapsed() {
    if (!isCollapsed) shouldAnchorOnCollapseRef.current = true;
    setIsCollapsed((current) => !current);
  }

  return (
    <section aria-label="Tool activity" className={`chat-tool-activity${isCollapsed ? " is-collapsed" : ""}`} onClick={(event) => event.stopPropagation()} ref={activityRef}>
      {!isCollapsed ? toolCalls.map((tool) => (
        tool.name === "subagent"
          ? <SubagentCard key={tool.id} onOpenSubagent={onOpenSubagent} onStopTool={onStopTool} tool={tool} />
          : <ToolCallCard key={tool.id} tool={tool} />
      )) : null}
      {isCollapsed ? (
        <CollapsedToolCheckpoints toolCalls={toolCalls} />
      ) : null}
      <button aria-expanded={!isCollapsed} aria-label={collapseLabel} className="chat-tool-collapse-all" onClick={toggleCollapsed} type="button">
        {collapseLabel}
      </button>
    </section>
  );
}

function CollapsedToolCheckpoints({ toolCalls }: { toolCalls: ChatToolCall[] }) {
  const checkpoints = toolCalls
    .map(toolCheckpoint)
    .filter((checkpoint): checkpoint is CheckpointMetadata => checkpoint !== undefined);
  if (checkpoints.length === 0) return null;

  const first = checkpoints[0];
  const last = checkpoints[checkpoints.length - 1];
  const hasHiddenCheckpoints = checkpoints.length > 2;

  return (
    <div aria-label="Tool call checkpoints" className={`chat-tool-checkpoints-collapsed${checkpoints.length === 1 ? " is-single" : ""}`}>
      <CheckpointLabel label={first.label} />
      {hasHiddenCheckpoints ? <span aria-hidden="true" className="chat-tool-checkpoints-ellipsis">...</span> : null}
      {checkpoints.length > 1 ? <CheckpointLabel label={last.label} /> : null}
    </div>
  );
}

function ToolCallCard({ tool }: { tool: ChatToolCall }) {
  if (tool.name === "orchestrate") {
    return <OrchestrateToolCard tool={tool} />;
  }

  const status = tool.status ?? tool.result?.status ?? "running";
  const checkpoint = toolCheckpoint(tool);
  const isFind = tool.name === "find";
  const isCreatePlan = tool.name === "createPlan";
  const isEditPlan = tool.name === "editPlan";
  const isBuildPlan = tool.name === "buildPlan";
  const isAddTodo = tool.name === "addTodo";
  const isTodoList = tool.name === "todoList";
  const isCheckTodo = tool.name === "checkTodo";
  const isRemoveTodo = tool.name === "removeTodo";
  const isCheckPlan = tool.name === "checkPlan";
  const isDeletePlan = tool.name === "deletePlan";
  const isFetchWeb = tool.name === "fetchWeb";
  const isWebSearch = tool.name === "webSearch";
  const isSearchSkill = tool.name === "searchSkill";
  const isSearchTools = tool.name === "searchTools";
  const isListSubAgents = tool.name === "listSubAgents";
  const isResearchStatus = tool.name === "researchStatus";
  const isRename = tool.name === "editFile" && Boolean(tool.renameTo);
  const isDelete = tool.name === "editFile" && Boolean(tool.delete);
  const isInlineTool = tool.name === "shell" || tool.name === "readFile" || isFind || tool.name === "listDir" || tool.name === "tree" || tool.name === "editFile" || tool.name === "loadSkill" || isSearchSkill || isSearchTools || isListSubAgents || isResearchStatus || tool.name === "deepResearch" || tool.name === "docsRetrieval" || isCreatePlan || isEditPlan || isBuildPlan || isAddTodo || isTodoList || isCheckTodo || isRemoveTodo || isCheckPlan || isDeletePlan || isFetchWeb || isWebSearch;
  const isDangerousArgument = isDelete || isRemoveTodo || isDeletePlan;
  const inlineCommand = isListSubAgents
    ? ""
    : tool.name === "editFile" && tool.renameTo
    ? `${tool.input ?? ""} → ${tool.renameTo}`
    : tool.input;
  const toolParameters = isFind || isCreatePlan || isAddTodo || isFetchWeb || isWebSearch
    ? tool.parameters ?? (isFind && tool.input ? [{ label: "pattern", value: tool.input }] : [])
    : [];
  const suppressResult = status === "success" && (
    isListSubAgents ||
    tool.name === "readFile" ||
    (tool.name === "editFile" && status === "success") ||
    (tool.name === "loadSkill" && status === "success") ||
    (tool.name === "docsRetrieval" && status === "success") ||
    (isCreatePlan && status === "success") ||
    (isEditPlan && status === "success") ||
    (isBuildPlan && status === "success") ||
    (isAddTodo && status === "success") ||
    (isCheckTodo && status === "success") ||
    (isRemoveTodo && status === "success") ||
    (isCheckPlan && status === "success") ||
    (isDeletePlan && status === "success") ||
    (isFetchWeb && status === "success")
  );
  const [isOpen, setIsOpen] = useState(() => tool.defaultOpen ?? false);

  return (
    <div className={`chat-tool-card is-${status}`} data-checkpoint={checkpoint?.label}>
      {checkpoint ? <CheckpointLabel className="chat-tool-checkpoint-label" label={checkpoint.label} /> : null}
      <details aria-busy={status === "running"} onToggle={(event) => setIsOpen(event.currentTarget.open)} open={isOpen}>
        <summary className="chat-tool-summary">
          <i aria-hidden="true" className="chat-tool-status-dot" />
          <HammerIcon />
          <span className={toolIntentClassName(tool)} title={displayToolIntent(tool)}>{displayToolIntent(tool)}</span>
          <svg aria-hidden="true" className="chat-tool-chevron" viewBox="0 0 24 24">
            <path d="m7 10 5 5 5-5" />
          </svg>
        </summary>
        <div className="chat-tool-body">
          <div className="chat-tool-execution">
            <div className="chat-tool-name-row">
              {isInlineTool ? (
                <>
                  <strong className="chat-tool-name chat-tool-label">Tool:</strong>
                  <strong className="chat-tool-name">{tool.name}</strong>
                  {isFind ? <span className="chat-tool-command">{tool.mode ?? "text"}</span> : inlineCommand ? <span className={`chat-tool-command${isDangerousArgument ? " is-delete" : ""}`}>{inlineCommand}</span> : null}
                </>
              ) : (
                <strong className="chat-tool-name">{tool.name}</strong>
              )}
            </div>
            {tool.input && !isInlineTool ? (
              <div className="chat-tool-input">
                <span aria-hidden="true" className="chat-tool-prompt">$</span>
                <pre><code>{tool.input}</code></pre>
              </div>
            ) : null}
            {isCheckPlan && tool.full ? (
              <div className="chat-tool-parameters">
                <div className="chat-tool-parameter">
                  <span className="chat-tool-parameter-value">full</span>
                </div>
              </div>
            ) : null}
            {(isFind || isCreatePlan || isAddTodo || isFetchWeb || isWebSearch) && toolParameters.length ? (
              <div className="chat-tool-parameters">
                {toolParameters.map((parameter) => (
                  <div className={`chat-tool-parameter${isAddTodo && parameter.label === "todo" ? " is-todo" : ""}`} key={`${parameter.label}-${parameter.value}`}>
                    {isAddTodo && parameter.label === "todo" ? (
                      <>
                        <span aria-hidden="true" className="chat-tool-todo-checkbox" />
                        <span className="chat-tool-parameter-value">{parameter.value}</span>
                      </>
                    ) : (
                      <>
                        <span className="chat-tool-parameter-label">{parameter.label}:</span>
                        <span className="chat-tool-parameter-value"> {parameter.value}</span>
                      </>
                    )}
                  </div>
                ))}
              </div>
            ) : null}
            {(tool.name === "editFile" || isEditPlan) && !isRename && !isDelete && (tool.oldString !== undefined || tool.newString !== undefined) ? (
              <EditFileDiffCard newString={tool.newString} oldString={tool.oldString} />
            ) : null}
            {tool.result ? (
              suppressResult ? null : <ToolResultCard result={tool.result} toolName={tool.name} />
            ) : null}
          </div>
        </div>
      </details>
    </div>
  );
}

function OrchestrateToolCard({ tool }: { tool: ChatToolCall }) {
  const status = tool.status ?? tool.result?.status ?? "running";
  const checkpoint = toolCheckpoint(tool);
  const [isOpen, setIsOpen] = useState(() => tool.defaultOpen ?? false);
  const [isSourceOpen, setIsSourceOpen] = useState(false);
  const result = tool.result;
  const sdkCalls = result?.sdkCalls;
  const duration = formatOrchestrateDuration(result?.durationMs);
  const source = tool.input?.replace(/\r\n?/g, "\n") ?? "";
  const sourcePanelID = `${tool.id}-source`;

  return (
    <div className={`chat-tool-card chat-code-mode-card is-${status}`} data-checkpoint={checkpoint?.label}>
      {checkpoint ? <CheckpointLabel className="chat-tool-checkpoint-label" label={checkpoint.label} /> : null}
      <details aria-busy={status === "running"} onToggle={(event) => setIsOpen(event.currentTarget.open)} open={isOpen}>
        <summary className="chat-tool-summary chat-code-mode-summary">
          <i aria-hidden="true" className="chat-tool-status-dot" />
          <HammerIcon />
          <span className="chat-code-mode-label">CODE</span>
          <span className={toolIntentClassName(tool)} title={displayToolIntent(tool)}>{displayToolIntent(tool)}</span>
          <svg aria-hidden="true" className="chat-tool-chevron" viewBox="0 0 24 24">
            <path d="m7 10 5 5 5-5" />
          </svg>
        </summary>
        <div className="chat-tool-body chat-code-mode-body">
          <div className="chat-code-mode-row chat-code-mode-tool-row">
            <span className="chat-code-mode-row-label">Tool:</span>
            <strong className="chat-code-mode-row-value">orchestrate</strong>
          </div>
          <div className="chat-code-mode-row chat-code-mode-sdk-row">
            <span className="chat-code-mode-row-label">SDK calls</span>
            <span className="chat-code-mode-row-value">{sdkCalls ?? "—"}</span>
            {duration ? <span className="chat-code-mode-row-meta">{duration}</span> : null}
          </div>
          <button
            aria-controls={sourcePanelID}
            aria-expanded={isSourceOpen}
            className="chat-code-mode-source-toggle"
            disabled={!source}
            onClick={() => setIsSourceOpen((current) => !current)}
            type="button"
          >
            {isSourceOpen ? "Hide source" : "Show source"}
            <svg aria-hidden="true" className="chat-code-mode-source-chevron" viewBox="0 0 24 24">
              <path d="m7 10 5 5 5-5" />
            </svg>
          </button>
          {isSourceOpen ? (
            <div aria-label="Orchestration source" className="chat-code-mode-source" id={sourcePanelID}>
              <CodeModeSource source={source} />
            </div>
          ) : null}
          {result ? <OrchestrateResult result={result} status={status} /> : (
            <div className="chat-code-mode-pending">Waiting for the daemon result…</div>
          )}
        </div>
      </details>
    </div>
  );
}

function CodeModeSource({ source }: { source: string }) {
  const hostRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const parent = hostRef.current;
    if (!parent) return;

    const view = new CodeMirrorView({
      parent,
      state: EditorState.create({
        doc: source,
        extensions: [
          lineNumbers(),
          EditorState.readOnly.of(true),
          CodeMirrorView.editable.of(false),
          syntaxHighlighting(codeModeHighlightStyle, { fallback: true }),
          go(),
          CodeMirrorView.theme(
            {
              "&": { background: "transparent", color: "#dce6e9", height: "100%" },
              ".cm-scroller": { fontFamily: '\"Geist Mono\", \"SFMono-Regular\", \"Cascadia Code\", ui-monospace, monospace', overflow: "auto" },
              ".cm-content": { caretColor: "transparent", padding: "10px 0 11px" },
              ".cm-line": { padding: "0 12px 0 8px" },
              ".cm-gutters": { background: "transparent", border: "none", color: "#52616a", minWidth: "48px" },
              ".cm-gutterElement": { padding: "0 9px 0 8px" },
              ".cm-activeLine, .cm-activeLineGutter": { background: "transparent" },
              "&.cm-focused": { outline: "none" },
              ".cm-selectionBackground, ::selection": { background: "transparent !important" },
            },
            { dark: true },
          ),
        ],
      }),
    });

    return () => view.destroy();
  }, [source]);

  return <div className="chat-code-mode-source-editor"><div ref={hostRef} /></div>;
}

function OrchestrateResult({ result, status }: { result: ChatToolResult; status: NonNullable<ChatToolCall["status"]> }) {
  const output = (result.output ?? "").replace(/\r\n?/g, "\n").trim();
  const error = result.error?.trim();
  const body = error || output || result.summary || (status === "success" ? "Script completed without printed output." : "No result output.");

  return (
    <div className={`chat-code-mode-result is-${result.status}`}>
      <div className="chat-code-mode-result-heading">
        <span className="chat-code-mode-result-label">Result</span>
        {result.truncated ? <span className="chat-code-mode-result-truncated">truncated</span> : null}
      </div>
      <pre className={error ? "is-error" : undefined}>{body}</pre>
    </div>
  );
}

function formatOrchestrateDuration(durationMs?: number) {
  if (durationMs === undefined || !Number.isFinite(durationMs) || durationMs < 0) return "";
  const seconds = durationMs / 1000;
  return `${seconds.toFixed(seconds >= 10 ? 1 : 2).replace(/0+$/, "").replace(/\.$/, "")}s`;
}

function SubagentCard({ onOpenSubagent, onStopTool, tool }: { onOpenSubagent?: (tool: ChatToolCall) => void; onStopTool?: (toolID: string) => void; tool: ChatToolCall }) {
	const status = subagentStatus(tool);
	const displayStatus = subagentDisplayStatus(tool);
	const checkpoint = toolCheckpoint(tool);
	const canOpen = Boolean(tool.result?.subchatId);

  return (
    <section
      aria-label="Open subagent chat"
      className={`chat-subagent-card is-${displayStatus}`}
      data-checkpoint={checkpoint?.label}
		onClick={() => {
			if (canOpen) onOpenSubagent?.(tool);
		}}
		onKeyDown={(event) => {
			if (!canOpen || event.target !== event.currentTarget || (event.key !== "Enter" && event.key !== " ")) return;
			event.preventDefault();
			onOpenSubagent?.(tool);
		}}
		role={canOpen ? "button" : undefined}
		tabIndex={canOpen ? 0 : -1}
    >
      {checkpoint ? <CheckpointLabel className="chat-tool-checkpoint-label" label={checkpoint.label} /> : null}
      <div className="chat-subagent-content">
        <div className="chat-subagent-heading">
          <i aria-hidden="true" className="chat-subagent-status-dot" />
          <span className="chat-subagent-title">Subagent</span>
          {tool.sync ? <span className="chat-subagent-mode">sync</span> : null}
          {subagentIsActive(status) ? (
            <button
              aria-label="Stop subagent"
              className="chat-subagent-stop"
              onClick={(event) => {
                event.stopPropagation();
                onStopTool?.(tool.id);
              }}
              title="Stop subagent"
              type="button"
            >
              Stop
            </button>
          ) : null}
        </div>
        {tool.input ? <p className="chat-subagent-task">{tool.input}</p> : null}
      </div>
    </section>
  );
}

function SubagentChatPanel({ isLoading = false, messages, onCollapse, tool }: { isLoading?: boolean; messages?: ChatMessage[]; onCollapse: () => void; tool: ChatToolCall }) {
  const status = subagentDisplayStatus(tool);
  const transcript = messages ?? [];

  useEffect(() => {
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") onCollapse();
    };
    document.addEventListener("keydown", closeOnEscape);
    return () => document.removeEventListener("keydown", closeOnEscape);
  }, [onCollapse]);

  return (
    <section
      aria-label="Subagent chat"
      aria-modal="true"
      className={`chat-subchat-panel is-${status}`}
      onClick={(event) => event.stopPropagation()}
      role="dialog"
    >
      <header className="chat-subchat-header">
        <div className="chat-subchat-heading">
          <i aria-hidden="true" className="chat-subchat-status-dot" />
          <span className="chat-subchat-title">Subagent chat</span>
        </div>
        <button aria-label="Collapse subagent chat" className="chat-subchat-collapse" onClick={onCollapse} title="Collapse subagent chat" type="button">
          <CollapseSubchatIcon />
        </button>
      </header>
      <div className="chat-subchat-body">
        <div className="chat-subchat-task">
          <span className="chat-subchat-task-label">Task</span>
          <p>{tool.input ?? "No task provided."}</p>
        </div>
        <div className="chat-subchat-transcript">
          {isLoading ? <p className="chat-empty">Loading subchat…</p> : transcript.length ? <ChatMessageGroups messages={indexChatMessages(transcript)} /> : <p className="chat-empty">No subchat messages yet.</p>}
        </div>
      </div>
    </section>
  );
}

function subagentStatus(tool: ChatToolCall): string {
  return tool.result?.subchatStatus ?? tool.status ?? tool.result?.status ?? "running";
}

function subagentDisplayStatus(tool: ChatToolCall): string {
	const status = subagentStatus(tool);
	if (status === "done") return "success";
	if (status === "cancelled" || status === "failed" || status === "interrupted" || status === "paused") return "error";
	return status;
}

function subagentIsActive(status: string): boolean {
  return status === "running" || status === "queued";
}

function EditFileDiffCard({ newString, oldString }: { newString?: string; oldString?: string }) {
  const [isExpanded, setIsExpanded] = useState(false);
  const oldLines = splitEditFileLines(oldString);
  const newLines = splitEditFileLines(newString);
  const canExpand = oldLines.length > 1 || newLines.length > 1;
  const visibleOldLines = isExpanded ? oldLines : oldLines.slice(0, 1);
  const visibleNewLines = isExpanded ? newLines : newLines.slice(0, 1);
  const toggleExpanded = () => {
    if (canExpand) setIsExpanded((current) => !current);
  };

  return (
    <div className={`chat-tool-edit-diff${isExpanded ? " is-expanded" : ""}`}>
      {visibleOldLines.map((line, index) => (
        <button
          aria-expanded={canExpand ? isExpanded : undefined}
          aria-label={canExpand ? (isExpanded ? "Collapse removed lines" : "Show all removed lines") : undefined}
          className="chat-tool-edit-line is-old"
          key={`old-${index}`}
          onClick={toggleExpanded}
          type="button"
        >
          {line || " "}
        </button>
      ))}
      {visibleNewLines.map((line, index) => (
        <button
          aria-expanded={canExpand ? isExpanded : undefined}
          aria-label={canExpand ? (isExpanded ? "Collapse added lines" : "Show all added lines") : undefined}
          className="chat-tool-edit-line is-new"
          key={`new-${index}`}
          onClick={toggleExpanded}
          type="button"
        >
          {line || " "}
        </button>
      ))}
    </div>
  );
}

function splitEditFileLines(value?: string): string[] {
  const normalized = (value ?? "").replace(/\r\n?/g, "\n");
  if (!normalized) return [];
  const lines = normalized.split("\n");
  if (lines.at(-1) === "") lines.pop();
  return lines;
}

function ToolResultCard({ result, toolName }: { result: ChatToolResult; toolName: string }) {
  if (result.status === "error") return <ToolErrorResultCard result={result} />;
  if (toolName === "find") {
    return <CountedToolResultCard collapseLabel="find results" plural="matches" result={result} singular="match" />;
  }
  if (toolName === "searchSkill") {
    return <CountedToolResultCard collapseLabel="skill matches" plural="matches" result={result} singular="match" />;
  }
  if (toolName === "searchTools") {
    return <CountedToolResultCard collapseLabel="tool matches" plural="matches" result={result} singular="match" />;
  }
  if (toolName === "listDir") {
    return <CountedToolResultCard collapseLabel="directory entries" plural="entries" result={result} singular="entry" />;
  }
  if (toolName === "deepResearch") {
    return <DeepResearchResultCard result={result} />;
  }
  if (toolName === "todoList") {
    return <TodoListResultCard result={result} />;
  }
  if (toolName === "researchStatus") {
    return <ResearchStatusResultCard result={result} />;
  }
  return <GenericToolResultCard result={result} />;
}

function ToolErrorResultCard({ result }: { result: ChatToolResult }) {
  const message = result.error?.trim() || result.output?.trim() || result.summary?.trim() || "The tool returned an error.";

  return (
    <div aria-label={`Error: ${message}`} className="chat-tool-result chat-tool-error-result is-error" role="alert">
      <span className="chat-tool-result-prefix">Error:</span>
      <span className="chat-tool-result-text">{message}</span>
    </div>
  );
}

function DeepResearchResultCard({ result }: { result: ChatToolResult }) {
  const status = result.researchStatus ?? "unknown";

  return (
    <div className={`chat-tool-result chat-research-status-result is-${result.status}`}>
      <span className="chat-research-status-prefix">Result:</span>
      <span className={`chat-research-status-value is-${status}`}>{status}</span>
      {result.jobId ? (
        <div className="chat-research-status-parameter">
          <span className="chat-research-status-label">jobId:</span>
          <span className="chat-research-status-value">{result.jobId}</span>
        </div>
      ) : null}
      {result.title ? (
        <div className="chat-research-status-parameter">
          <span className="chat-research-status-label">title:</span>
          <span className="chat-research-status-value">{result.title}</span>
        </div>
      ) : null}
    </div>
  );
}

function ResearchStatusResultCard({ result }: { result: ChatToolResult }) {
  const status = result.researchStatus ?? "unknown";

  return (
    <div className={`chat-tool-result chat-research-status-result is-${result.status}`}>
      <span className="chat-research-status-prefix">Result:</span>
      <span className={`chat-research-status-value is-${status}`}>{status}</span>
      {result.phase ? (
        <div className="chat-research-status-parameter">
          <span className="chat-research-status-label">phase:</span>
          <span className="chat-research-status-value">{result.phase}</span>
        </div>
      ) : null}
      {typeof result.round === "number" && typeof result.maxRounds === "number" ? (
        <div className="chat-research-status-parameter">
          <span className="chat-research-status-label">round:</span>
          <span className="chat-research-status-value">{result.round}/{result.maxRounds}</span>
        </div>
      ) : null}
    </div>
  );
}

function GenericToolResultCard({ result }: { result: ChatToolResult }) {
  const [isExpanded, setIsExpanded] = useState(false);
  const output = (result.output ?? "").replace(/\r\n?/g, "\n");
  const firstLine = result.summary ?? (output.split("\n")[0] || "—");
  const displayedOutput = isExpanded ? output || "—" : firstLine;
  const toggleLabel = isExpanded ? "Collapse result" : "Show full result";

  return (
    <div className={`chat-tool-result is-${result.status}${isExpanded ? " is-expanded" : ""}`}>
      <button aria-expanded={isExpanded} aria-label={toggleLabel} className="chat-tool-result-toggle" onClick={() => setIsExpanded((current) => !current)} type="button">
        <span className="chat-tool-result-prefix">Result:</span>
        <span className="chat-tool-result-text">{displayedOutput}</span>
      </button>
    </div>
  );
}

function CountedToolResultCard({
  collapseLabel,
  plural,
  result,
  singular,
}: {
  collapseLabel: string;
  plural: string;
  result: ChatToolResult;
  singular: string;
}) {
  const [isExpanded, setIsExpanded] = useState(false);
  const outputItems = (result.output ?? "").replace(/\r\n?/g, "\n").split("\n").filter(Boolean);
  const items = result.items?.length ? result.items : outputItems;
  const count = typeof result.count === "number" ? result.count : items.length;

  if (!isExpanded) {
    return (
      <div className={`chat-tool-result is-${result.status}`}>
        <button aria-expanded={false} aria-label={`Show ${collapseLabel}`} className="chat-tool-result-toggle" onClick={() => setIsExpanded(true)} type="button">
          <span className="chat-tool-result-prefix">Result:</span>
          <span className="chat-tool-result-text">{count} {count === 1 ? singular : plural}</span>
        </button>
      </div>
    );
  }

  return (
    <div className={`chat-tool-result chat-tool-result-list is-${result.status} is-expanded`}>
      <div className="chat-tool-result-list-label">Result:</div>
      <div className="chat-tool-result-items">
        {items.map((item) => (
          <button aria-label={`Collapse ${collapseLabel}`} className="chat-tool-result-item" key={item} onClick={() => setIsExpanded(false)} type="button">
            {item}
          </button>
        ))}
      </div>
    </div>
  );
}

function TodoListResultCard({ result }: { result: ChatToolResult }) {
  const [isExpanded, setIsExpanded] = useState(false);
  const items = result.todoItems?.length ? result.todoItems : parseTodoItems(result.output);
  const orderedItems = [...items.filter((item) => item.checked), ...items.filter((item) => !item.checked)];
  const count = typeof result.count === "number" ? result.count : items.length;
  const completed = typeof result.completed === "number" ? result.completed : items.filter((item) => item.checked).length;
  const summary = `${count} ${count === 1 ? "todo" : "todos"}, ${completed} completed`;

  if (!isExpanded) {
    return (
      <div className={`chat-tool-result is-${result.status}`}>
        <button aria-expanded={false} aria-label="Show todo list" className="chat-tool-result-toggle" onClick={() => setIsExpanded(true)} type="button">
          <span className="chat-tool-result-prefix">Result:</span>
          <span className="chat-tool-result-text">{summary}</span>
        </button>
      </div>
    );
  }

  return (
    <div className={`chat-tool-result chat-tool-result-list is-${result.status} is-expanded`}>
      <div className="chat-tool-result-list-label">Result:</div>
      <div className="chat-tool-result-items">
        {orderedItems.map((item) => (
          <button aria-label={`Collapse todo list: ${item.text}`} className="chat-tool-result-todo-item" key={`${item.checked}-${item.text}`} onClick={() => setIsExpanded(false)} type="button">
            <span aria-hidden="true" className={`chat-tool-result-todo-checkbox${item.checked ? " is-checked" : ""}`} />
            <span>{item.text}</span>
          </button>
        ))}
      </div>
    </div>
  );
}

function parseTodoItems(output?: string) {
  return (output ?? "")
    .replace(/\r\n?/g, "\n")
    .split("\n")
    .map((line) => line.match(/^\s*-\s*\[([ xX])\]\s*(.+?)\s*$/))
    .filter((match): match is RegExpMatchArray => Boolean(match))
    .map((match) => ({ checked: match[1].toLowerCase() === "x", text: match[2] }));
}

function indexChatMessages(messages: ChatMessage[]): IndexedChatMessage[] {
  let fallbackSequence = -1;
  let fallbackBranch = "";

  return messages.map((message, index) => {
    const toolMessageIDs = new Map<string, string>();
    if (message.role === "assistant") {
      for (const tool of message.toolCalls ?? []) toolMessageIDs.set(tool.id, message.id);
    }

    if (message.kind === "compaction") {
      return { index, message, toolMessageIDs };
    }

    const hasExplicitSequence = typeof message.checkpointSeq === "number" && Number.isFinite(message.checkpointSeq);
    if (hasExplicitSequence) {
      fallbackSequence = Math.max(fallbackSequence, Math.max(0, Math.floor(message.checkpointSeq!)));
      if (message.checkpointBranch !== undefined) fallbackBranch = message.checkpointBranch;
    } else if (message.role === "user" || fallbackSequence < 0) {
      fallbackSequence += 1;
      fallbackBranch = "";
    }

    const sequence = hasExplicitSequence ? Math.max(0, Math.floor(message.checkpointSeq!)) : Math.max(0, fallbackSequence);
    const branch = message.checkpointBranch ?? fallbackBranch;
    const label = formatCheckpointLabel(sequence, branch);
    const displayMessage = addToolCheckpoints(message, sequence, branch, fallbackSequence);
    const toolCheckpointSequences = displayMessage.toolCalls?.flatMap((tool) => (
      typeof tool.checkpointSeq === "number" && Number.isFinite(tool.checkpointSeq) ? [Math.floor(tool.checkpointSeq)] : []
    )) ?? [];
    if (toolCheckpointSequences.length) fallbackSequence = Math.max(fallbackSequence, ...toolCheckpointSequences);

    return {
      checkpoint: { branch, label, sequence },
      index,
      message: displayMessage,
      toolMessageIDs,
    };
  });
}

function toolCheckpoint(tool: ChatToolCall): CheckpointMetadata | undefined {
  if (typeof tool.checkpointSeq !== "number" || !Number.isFinite(tool.checkpointSeq)) return undefined;
  const sequence = Math.max(0, Math.floor(tool.checkpointSeq));
  const branch = tool.checkpointBranch ?? "";
  return { branch, label: formatCheckpointLabel(sequence, branch), sequence };
}

function addToolCheckpoints(message: ChatMessage, sequence: number, branch: string, lastUsedSequence: number): ChatMessage {
  if (!message.toolCalls?.length) return message;

  let fallbackSequence = Math.max(sequence + 1, lastUsedSequence + 1);
  const toolCalls = message.toolCalls.map((tool) => {
    if (toolCheckpoint(tool)) {
      fallbackSequence = Math.max(fallbackSequence, Math.floor(tool.checkpointSeq!) + 1);
      return tool;
    }
    const checkpointSeq = fallbackSequence;
    fallbackSequence += 1;
    return { ...tool, checkpointBranch: branch, checkpointSeq };
  });

  return { ...message, toolCalls };
}

function groupChatTurns(messages: IndexedChatMessage[]): IndexedChatMessage[][] {
  const groups: IndexedChatMessage[][] = [];

  for (let index = 0; index < messages.length; index += 1) {
    const entry = messages[index];
    if (entry.message.kind === "compaction" || entry.message.role !== "assistant") {
      groups.push([entry]);
      continue;
    }

    const assistantMessages = [entry];
    while (index + 1 < messages.length && messages[index + 1].message.role === "assistant" && messages[index + 1].message.kind !== "compaction") {
      index += 1;
      assistantMessages.push(messages[index]);
    }
    groups.push(assistantMessages);
  }

  return groups;
}

function liveWorkedForSeconds(chat: Chat, now: number): number | undefined {
  const groups = groupChatTurns(indexChatMessages(chat.messages));
  const lastAssistantGroupIndex = groups.reduce((lastIndex, entries, groupIndex) => (
    entries[0]?.message.role === "assistant" ? groupIndex : lastIndex
  ), -1);
  if (lastAssistantGroupIndex < 0) return undefined;

  const lastAssistantGroup = groups[lastAssistantGroupIndex];
  const startTimes = lastAssistantGroup
    .map(({ message }) => message.workStartedAt)
    .filter((value): value is number => value !== undefined && Number.isFinite(value));
  const assistantStartedAt = startTimes.length ? Math.min(...startTimes) : undefined;
  const hasContentAfterAssistant = lastAssistantGroupIndex < groups.length - 1;
  const startedAt = hasContentAfterAssistant
    ? chat.runStartedAt ?? assistantStartedAt
    : assistantStartedAt ?? chat.runStartedAt;
  if (startedAt === undefined || !Number.isFinite(startedAt)) return undefined;
  return Math.max(0, (now - startedAt) / 1000);
}

function assistantFooterMessage(entries: IndexedChatMessage[]): ChatMessage {
  const messages = entries.map((entry) => entry.message);
  if (messages.length === 1) return messages[0];

  const first = messages[0];
  const last = messages[messages.length - 1];
  return {
    ...last,
    content: messages.map((message) => message.content.trim()).filter(Boolean).join("\n\n"),
    id: `assistant-turn-footer-${first.id}`,
    images: messages.flatMap((message) => message.images ?? []),
    reasoning: undefined,
    stats: aggregateAssistantStats(messages),
    toolCalls: undefined,
    workedFor: sumDefined(messages.map((message) => message.workedFor)),
  };
}

function aggregateAssistantStats(messages: ChatMessage[]): ChatStats | undefined {
  const stats = messages.flatMap((message) => message.stats ? [message.stats] : []);
  if (stats.length === 0) return undefined;
  if (stats.length === 1) return stats[0];

  const last = stats[stats.length - 1];
  const reasoningTokens = stats.reduce((total, current) => total + current.reasoningTokens, 0);
  const responseTokens = stats.reduce((total, current) => total + current.responseTokens, 0);
  const contextTokens = lastNonZero(stats.map((current) => current.contextTokens));
  const userTokens = lastNonZero(stats.map((current) => current.userTokens));
  const outputTokensPerSecond = average(stats.map((current) => current.outputTokensPerSecond));
  const promptTokensPerSecond = average(stats.map((current) => current.promptTokensPerSecond));
  const totalTokens = contextTokens + userTokens + reasoningTokens + responseTokens;

  return {
    contextTokens,
    outputTokensPerSecond,
    promptTokensPerSecond,
    reasoningTokens,
    responseTokens,
    totalTokens,
    ttftSeconds: stats[0].ttftSeconds,
    userTokens,
  };
}

function sumDefined(values: Array<number | undefined>) {
  const defined = values.filter((value): value is number => value !== undefined && Number.isFinite(value));
  return defined.length ? defined.reduce((total, value) => total + value, 0) : undefined;
}

function lastNonZero(values: number[]) {
  return [...values].reverse().find((value) => value > 0) ?? 0;
}

function average(values: number[]) {
  if (values.length === 0) return 0;
  return values.reduce((total, value) => total + value, 0) / values.length;
}

function formatCheckpointLabel(sequence: number, branch: string) {
  return `[#${String(sequence).padStart(3, "0")}${branch}]`;
}

function MessageFooter({ index, message, onRequestDelete }: { index: number; message: ChatMessage; onRequestDelete?: () => void }) {
  const [copied, setCopied] = useState(false);
  const [isStatsOpen, setIsStatsOpen] = useState(false);
  const statsRef = useRef<HTMLDivElement>(null);
  const createdAt = message.createdAt ?? MESSAGE_START_TIME + index * 60_000;
  const stats = message.role === "assistant" ? message.stats : undefined;

  useEffect(() => {
    if (!isStatsOpen) return;

    const closeOnPointerDown = (event: PointerEvent) => {
      if (statsRef.current && event.target instanceof Node && statsRef.current.contains(event.target)) return;
      setIsStatsOpen(false);
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setIsStatsOpen(false);
    };

    document.addEventListener("pointerdown", closeOnPointerDown);
    document.addEventListener("keydown", closeOnEscape);
    return () => {
      document.removeEventListener("pointerdown", closeOnPointerDown);
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, [isStatsOpen]);

  async function copyMessage() {
    try {
      if (navigator.clipboard?.writeText) await navigator.clipboard.writeText(message.content);
      else copyTextFallback(message.content);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1400);
    } catch {
      try {
        copyTextFallback(message.content);
        setCopied(true);
        window.setTimeout(() => setCopied(false), 1400);
      } catch {
        // Clipboard access can be unavailable in a restricted webview.
      }
    }
  }

  return (
    <footer className="chat-message-footer">
      <time dateTime={new Date(createdAt).toISOString()}>{formatMessageTime(createdAt)}</time>
      {stats ? (
        <div className="chat-stats-control" ref={statsRef}>
          <button
            aria-controls={`message-stats-${message.id}`}
            aria-expanded={isStatsOpen}
            aria-label={isStatsOpen ? "Hide turn statistics" : "Show turn statistics"}
            className="chat-stats-trigger"
            onClick={(event) => {
              event.stopPropagation();
              setIsStatsOpen((current) => !current);
            }}
            title={isStatsOpen ? "Hide turn statistics" : "Show turn statistics"}
            type="button"
          >
            <InfoIcon />
          </button>
          {isStatsOpen ? <MessageStatsPopover id={`message-stats-${message.id}`} stats={stats} workedFor={message.workedFor} /> : null}
        </div>
      ) : null}
      <button
        aria-label={copied ? "Message copied" : "Copy message"}
        className="chat-copy-message"
        onClick={() => void copyMessage()}
        title={copied ? "Message copied" : "Copy message"}
        type="button"
      >
        {copied ? <CheckIcon /> : <CopyIcon />}
      </button>
      {onRequestDelete ? (
        <button aria-label="Delete message" className="chat-delete-message" onClick={onRequestDelete} title="Delete message" type="button">
          <CloseIcon />
        </button>
      ) : null}
    </footer>
  );
}

function WorkedForCounter({ isLive, seconds }: { isLive: boolean; seconds: number }) {
  return (
    <span aria-live={isLive ? "polite" : undefined} className={`chat-worked-for${isLive ? " is-live" : ""}`}>
      worked for {formatWorkedDuration(seconds)}
    </span>
  );
}

function MessageStatsPopover({ id, stats, workedFor }: { id: string; stats: ChatStats; workedFor?: number }) {
  const rows = [
    ["context", formatStatsTokenCount(stats.contextTokens)],
    ["user", formatStatsTokenCount(stats.userTokens)],
    ["reasoning", formatStatsTokenCount(stats.reasoningTokens)],
    ["response", formatStatsTokenCount(stats.responseTokens)],
    ["total", formatStatsTokenCount(stats.totalTokens)],
    ["t/s", `${formatStatsDecimal(stats.outputTokensPerSecond)} t/s`],
    ["ttft", `${formatStatsDecimal(stats.ttftSeconds)}s`],
    ["pp", `${formatStatsDecimal(stats.promptTokensPerSecond)} t/s`],
    ["worked for", workedFor === undefined ? "—" : formatWorkedDuration(workedFor)],
  ] as const;

  return (
    <div aria-label="Turn statistics" className="chat-stats-popover" id={id} role="dialog">
      <div className="chat-stats-title">Turn statistics</div>
      <dl>
        {rows.map(([label, value]) => (
          <div className={`chat-stats-row${label === "total" ? " is-total" : ""}`} key={label}>
            <dt>{label}</dt>
            <dd>{value}</dd>
          </div>
        ))}
      </dl>
    </div>
  );
}

function ReasoningBlock({ isCollapsed, message, onToggle }: { isCollapsed: boolean; message: ChatMessage; onToggle: () => void }) {
  const reasoning = message.reasoning?.trim();
  const thoughtFor = message.thoughtFor ?? message.stats?.ttftSeconds;

  return (
    <div
      aria-expanded={!isCollapsed}
      aria-label="Model reasoning"
      className={`chat-reasoning${isCollapsed ? " is-collapsed" : ""}`}
      onClick={(event) => {
        event.stopPropagation();
        onToggle();
      }}
      onKeyDown={(event) => {
        if (event.key !== "Enter" && event.key !== " ") return;
        event.preventDefault();
        event.stopPropagation();
        onToggle();
      }}
      role="button"
      tabIndex={0}
      title="Click to collapse or expand reasoning"
    >
      {!isCollapsed && reasoning ? <div className="chat-reasoning-copy">{reasoning}</div> : null}
      {thoughtFor !== undefined && thoughtFor > 0 ? <div className="chat-thought-for">{formatThoughtDuration(thoughtFor)}</div> : null}
    </div>
  );
}

function formatMessageTime(timestamp: number) {
  return new Intl.DateTimeFormat("it-IT", { hour: "2-digit", minute: "2-digit" }).format(timestamp);
}

function formatWorkedDuration(seconds: number) {
  if (!Number.isFinite(seconds) || seconds <= 0) return "0s";
  const totalSeconds = Math.round(seconds);
  const hours = Math.floor(totalSeconds / 3600);
  const remaining = totalSeconds % 3600;
  const minutes = Math.floor(remaining / 60);
  const remainingSeconds = remaining % 60;
  return `${hours ? `${hours}h` : ""}${minutes || hours ? `${minutes}m` : ""}${remainingSeconds}s`;
}

function formatThoughtDuration(seconds: number) {
  if (Number.isFinite(seconds) && seconds > 0 && seconds < 1) return "thought briefly";
  return `thought for ${formatWorkedDuration(seconds)}`;
}

function formatStatsTokenCount(value: number) {
  if (!Number.isFinite(value)) return "—";
  return Math.max(0, Math.round(value)).toLocaleString("it-IT");
}

function formatStatsDecimal(value: number) {
  if (!Number.isFinite(value)) return "—";
  return value.toFixed(2).replace(/\.?(0+)$/, "");
}

function MarkdownContent({ content }: { content: string }) {
  return (
    <Markdown
      components={{
        a: ({ children, href, ...props }) => (
          <a {...props} className={`${props.className ?? ""}${isFileLink(href) ? " chat-file-link" : ""}`.trim()} href={href} rel="noreferrer" target="_blank">{children}</a>
        ),
        input: (props) => <input {...props} aria-label="Completed activity" className="chat-checkbox" disabled />,
        pre: ({ children }) => <CodeBlock>{children}</CodeBlock>,
        table: ({ children, ...props }) => (
          <div className="chat-table-wrap">
            <table {...props}>{children}</table>
          </div>
        ),
      }}
      remarkPlugins={[remarkGfm]}
    >
      {content}
    </Markdown>
  );
}

function CodeBlock({ children }: { children?: ReactNode }) {
  const [copied, setCopied] = useState(false);
  const code = textFromNode(children);
  const codeLines = code.replace(/\r\n/g, "\n").replace(/\n$/, "").split("\n");
  const language = codeLanguageFromNode(children);

  async function copyCode() {
    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(code);
      } else {
        copyTextFallback(code);
      }
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1400);
    } catch {
      try {
        copyTextFallback(code);
        setCopied(true);
        window.setTimeout(() => setCopied(false), 1400);
      } catch {
        // Clipboard access can be unavailable in a restricted webview.
      }
    }
  }

  return (
    <div className="chat-code-block">
      <div className="chat-code-toolbar">
        <div className="chat-code-language">{language ?? "Code"}</div>
        <button
          aria-label={copied ? "Code copied" : "Copy code"}
          className="chat-copy-code"
          onClick={() => void copyCode()}
          title={copied ? "Code copied" : "Copy code"}
          type="button"
        >
          {copied ? <CheckIcon /> : <CopyIcon />}
        </button>
      </div>
      <pre>
        <code className="chat-code-lines">
          {codeLines.map((line, index) => (
            <span className="chat-code-line" key={index}>
              <span aria-hidden="true" className="chat-line-number">{index + 1}</span>
              <span className="chat-line-content">{line || " "}</span>
            </span>
          ))}
        </code>
      </pre>
    </div>
  );
}

function copyTextFallback(value: string) {
  const textarea = document.createElement("textarea");
  textarea.value = value;
  textarea.setAttribute("readonly", "");
  textarea.style.position = "fixed";
  textarea.style.opacity = "0";
  document.body.appendChild(textarea);
  try {
    textarea.select();
    if (!document.execCommand("copy")) throw new Error("Copy command failed");
  } finally {
    textarea.remove();
  }
}

function textFromNode(node: ReactNode): string {
  if (typeof node === "string" || typeof node === "number") return String(node);
  if (Array.isArray(node)) return node.map(textFromNode).join("");
  if (isValidElement(node)) {
    return textFromNode((node.props as { children?: ReactNode }).children);
  }
  return "";
}

function codeLanguageFromNode(node: ReactNode): string | undefined {
  if (Array.isArray(node)) {
    for (const child of node) {
      const language = codeLanguageFromNode(child);
      if (language) return language;
    }
    return undefined;
  }
  if (!isValidElement(node)) return undefined;

  const props = node.props as { className?: string; children?: ReactNode };
  const match = props.className?.match(/(?:^|\s)language-([^\s]+)/);
  if (match) return formatCodeLanguage(match[1]);
  return codeLanguageFromNode(props.children);
}

function formatCodeLanguage(language: string) {
  const normalized = language.toLowerCase();
  const labels: Record<string, string> = {
    bash: "Shell",
    css: "CSS",
    go: "Go",
    html: "HTML",
    javascript: "JavaScript",
    js: "JavaScript",
    json: "JSON",
    jsx: "JSX",
    markdown: "Markdown",
    md: "Markdown",
    plaintext: "Text",
    sh: "Shell",
    shell: "Shell",
    sql: "SQL",
    text: "Text",
    ts: "TypeScript",
    tsx: "TSX",
    typescript: "TypeScript",
    yaml: "YAML",
    yml: "YAML",
  };
  return labels[normalized] ?? language;
}

function isFileLink(href?: string) {
  if (!href || href.startsWith("#") || /^[a-z][a-z\d+.-]*:/i.test(href)) return false;
  return href.startsWith("./") || href.startsWith("../") || href.startsWith("/") || /\.[a-z\d]{1,8}(?:[?#].*)?$/i.test(href);
}

export function ChatTopbar({ breadcrumb, onOpenFolder, title }: { breadcrumb?: string; onOpenFolder: () => void; title: string }) {
  const topbarRef = useRef<HTMLDivElement>(null);
  const contextRef = useRef<HTMLButtonElement>(null);
  const folderName = breadcrumb ?? "Project";
  const [showContext, setShowContext] = useState(true);
  const [visibleTitle, setVisibleTitle] = useState(title);

  useLayoutEffect(() => {
    const topbar = topbarRef.current;
    const context = contextRef.current;
    if (!topbar || !context) return;
    const measure = () => {
      // Research reports always keep the folder breadcrumb actionable; the
      // report title can shrink and ellipsize when the available width is tight.
      if (breadcrumb) {
        setShowContext(true);
        setVisibleTitle(title);
        return;
      }
      const available = topbar.clientWidth;
      const contextWidth = context.getBoundingClientRect().width;
      const fullTitleWidth = measureTopbarText(title);
      if (contextWidth + 10 + fullTitleWidth <= available) {
        setShowContext(true);
        setVisibleTitle(title);
        return;
      }
      setShowContext(false);
      setVisibleTitle(title);
    };
    const observer = new ResizeObserver(measure);
    observer.observe(topbar);
    measure();
    return () => observer.disconnect();
  }, [breadcrumb, title]);

  return (
    <div aria-label={`${folderName} / ${title}`} className="chat-topbar" ref={topbarRef}>
      <button aria-label={`Back to new chat in ${folderName}`} className={`chat-topbar-context${showContext ? "" : " is-hidden"}`} onClick={onOpenFolder} ref={contextRef} type="button">
        <FolderIcon />
        <span className="chat-topbar-folder">{folderName}</span>
        <span aria-hidden="true" className="chat-topbar-slash">/</span>
      </button>
      <span className="chat-topbar-title" title={title}>{visibleTitle}</span>
    </div>
  );
}

function measureTopbarText(value: string) {
  if (typeof document === "undefined") return value.length * 10;
  const canvas = document.createElement("canvas");
  const context = canvas.getContext("2d");
  if (!context) return value.length * 10;
  context.font = '650 20px "Geist", ui-sans-serif, system-ui, sans-serif';
  return context.measureText(value).width;
}


function InfoIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><circle cx="12" cy="12" r="8.5" /><path d="M12 10.5v5M12 7.5h.01" /></svg>;
}

function CollapseSubchatIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M9 4H4v5M15 20h5v-5M4 8l5-5M20 16l-5 5" /></svg>;
}

function BranchIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><circle cx="6" cy="6" r="2.5" /><circle cx="6" cy="18" r="2.5" /><circle cx="18" cy="6" r="2.5" /><path d="M8.5 6h4A5.5 5.5 0 0 1 18 11.5V14" /><path d="M6 8.5v7" /></svg>;
}

function WorktreeIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M4 5h6l2 2h8v12H4z" /><path d="M4 9h16" /></svg>;
}

function CopyIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><rect height="11" rx="1.5" width="11" x="9" y="9" /><path d="M15 9V6.5A2.5 2.5 0 0 0 12.5 4H6a2 2 0 0 0-2 2v6.5A2.5 2.5 0 0 0 6.5 15H9" /></svg>;
}

function CheckIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="m5 12 4 4L19 6" /></svg>;
}

function CloseIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="m6 6 12 12M18 6 6 18" /></svg>;
}

function HammerIcon() {
  return (
    <svg aria-hidden="true" className="chat-tool-hammer-icon" viewBox="0 0 80.4375 96.03515625">
      <g fillRule="nonzero" transform="scale(1,-1) translate(0,-96.03515625)">
        <path d="M 12.58984375,18.025390625 L 8.03515625,22.55859375 Q 6.251953125,24.36328125 6.3701171875,26.3291015625 Q 6.48828125,28.294921875 8.59375,30.12109375 L 38.84375,56.67578125 L 46.814453125,48.705078125 L 20.15234375,18.583984375 Q 18.3046875,16.5 16.349609375,16.3603515625 Q 14.39453125,16.220703125 12.58984375,18.025390625 Z M 61.703125,41.59375 L 59.640625,43.61328125 Q 58.78125,44.4296875 58.5986328125,45.095703125 Q 58.416015625,45.76171875 58.48046875,46.771484375 L 58.716796875,49.62890625 L 56.482421875,51.884765625 L 52.20703125,51.025390625 Q 50.896484375,50.767578125 50.0048828125,51.00390625 Q 49.11328125,51.240234375 48.296875,52.056640625 L 42.044921875,58.30859375 Q 40.94921875,59.42578125 40.6484375,60.6826171875 Q 40.34765625,61.939453125 41.013671875,63.59375 L 43.18359375,69.05078125 Q 40.1328125,70.984375 36.6201171875,71.0166015625 Q 33.107421875,71.048828125 29.08984375,69.953125 Q 28.359375,69.73828125 27.671875,69.953125 Q 26.984375,70.16796875 26.51171875,70.640625 Q 25.931640625,71.306640625 25.888671875,72.2841796875 Q 25.845703125,73.26171875 26.791015625,74.185546875 Q 29.111328125,76.505859375 32.0546875,77.81640625 Q 34.998046875,79.126953125 38.2314453125,79.470703125 Q 41.46484375,79.814453125 44.6982421875,79.234375 Q 47.931640625,78.654296875 50.853515625,77.2041015625 Q 53.775390625,75.75390625 56.07421875,73.4765625 L 61.810546875,67.783203125 Q 63.25,66.365234375 63.744140625,64.96875 Q 64.23828125,63.572265625 63.916015625,62.154296875 L 63.03515625,58.39453125 L 65.291015625,56.16015625 L 68.169921875,56.4609375 Q 68.814453125,56.50390625 69.2978515625,56.439453125 Q 69.78125,56.375 70.2431640625,56.1064453125 Q 70.705078125,55.837890625 71.263671875,55.279296875 L 73.34765625,53.216796875 Q 74.12109375,52.443359375 74.1533203125,51.5517578125 Q 74.185546875,50.66015625 73.43359375,49.88671875 L 65.033203125,41.529296875 Q 64.259765625,40.755859375 63.3896484375,40.7880859375 Q 62.51953125,40.8203125 61.703125,41.59375 Z" />
      </g>
    </svg>
  );
}

function FolderIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M3 6a2 2 0 0 1 2-2h5l2 2h7a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2Z" /></svg>;
}
