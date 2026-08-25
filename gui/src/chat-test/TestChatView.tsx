import { isValidElement, type MouseEvent, type ReactNode, useEffect, useLayoutEffect, useRef, useState } from "react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import asciiBanner from "../../../internal/logo/logo.txt?raw";
import asciiColors from "../../../internal/logo/colors.txt?raw";
import type { FakeChat, FakeChatImage, FakeChatMessage, FakeChatStats, FakeChatToolCall, FakeChatToolResult } from "./fakeChats";
import { AtMentionInput, type ComposerImageAttachment } from "../home/AtMentionInput";
import { testChatAtMentionEntries } from "../shell/RightSidePanel";
import "./test-chat.css";

const asciiColorRows = asciiColors.trim().split(/\r?\n/).map((row) => row.trim().split(/\s+/));
const FIXTURE_MESSAGE_START_TIME = new Date("2026-01-01T09:00:00").getTime();

type CheckpointMetadata = {
  branch: string;
  label: string;
  sequence: number;
};

type IndexedChatMessage = {
  checkpoint?: CheckpointMetadata;
  index: number;
  message: FakeChatMessage;
};

type TestChatViewProps = {
  bottomInset?: number;
  chat: FakeChat;
  isStreaming?: boolean;
  onDeleteMessage: (chatID: string, messageID: string) => void;
  onSend: (chatID: string, message: FakeChatMessage) => void;
  onStopStreaming: (chatID: string) => void;
  pendingUserMessageIDs?: ReadonlySet<string>;
  workspaceName?: string;
  workspacePath?: string;
};

export function TestChatView({ bottomInset = 0, chat, isStreaming = false, onDeleteMessage, onSend, onStopStreaming, pendingUserMessageIDs = new Set(), workspaceName, workspacePath }: TestChatViewProps) {
  const [draft, setDraft] = useState("");
  const [images, setImages] = useState<ComposerImageAttachment[]>([]);
  const [deleteTarget, setDeleteTarget] = useState<FakeChatMessage | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const deleteCancelRef = useRef<HTMLButtonElement>(null);
  const lastMessageContent = chat.messages.at(-1)?.content ?? "";
  const lastMessageReasoning = chat.messages.at(-1)?.reasoning ?? "";
  const lastMessageStatus = chat.messages.at(-1)?.status ?? "";
  const lastMessageThoughtFor = chat.messages.at(-1)?.thoughtFor ?? null;
  const lastMessageWorkedFor = chat.messages.at(-1)?.workedFor ?? null;
  const pendingMessageKey = [...pendingUserMessageIDs].join("-");
  const indexedMessages = indexChatMessages(chat.messages);
  const pendingMessages = indexedMessages.filter(({ message }) => pendingUserMessageIDs.has(message.id));
  const visibleMessages = indexedMessages.filter(({ message }) => !pendingUserMessageIDs.has(message.id));

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ block: "end" });
  }, [chat.id, chat.messages.length, lastMessageContent, lastMessageReasoning, lastMessageStatus, lastMessageThoughtFor, lastMessageWorkedFor, pendingMessageKey]);

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

  const send = async () => {
    const content = draft.trim();
    if (!content && images.length === 0) return;
    const messageImages = await Promise.all(images.map(snapshotComposerImage));
    onSend(chat.id, { createdAt: Date.now(), id: `user-${Date.now()}`, images: messageImages, role: "user", content });
    setDraft("");
    setImages([]);
  };

  return (
    <section aria-label={`Test chat: ${chat.title}`} className="test-chat-view" style={{ bottom: Math.max(0, bottomInset) }}>
      <AsciiCrown />
      <div aria-live="polite" className="test-chat-messages-shell">
        <div className="test-chat-messages">
          {chat.messages.length ? visibleMessages.map(({ checkpoint, index, message }) => (
            message.kind === "compaction" ? <CompactionCard key={message.id} message={message} /> : (
              <ChatMessageTurn
                checkpoint={checkpoint}
                index={index}
                key={message.id}
                message={message}
                onRequestDelete={message.role === "user" ? () => setDeleteTarget(message) : undefined}
              />
            )
          )) : <p className="test-chat-empty">This chat is ready for the first message.</p>}
          <div ref={messagesEndRef} />
        </div>
      </div>
      <div className="welcome-composer-dock test-chat-composer-dock">
        {pendingMessages.length ? (
          <div className="test-chat-pending-messages">
            {pendingMessages.map(({ message }) => (
              <div className="test-chat-pending-turn test-chat-turn is-user" key={message.id}>
              <article className="test-chat-message test-chat-pending-message is-user">
                {message.images?.length ? <ChatImageAttachments images={message.images} /> : null}
                <MarkdownContent content={message.content} />
              </article>
            </div>
            ))}
          </div>
        ) : null}
        <form className="test-chat-composer" onSubmit={(event) => { event.preventDefault(); void send(); }}>
          <div className="welcome-composer">
          <AtMentionInput
            aria-label="Test message"
            className="welcome-input"
            entries={testChatAtMentionEntries}
            images={images}
            onChange={setDraft}
            onImagesChange={setImages}
            onKeyDown={(event) => {
              if (event.key === "Enter" && !event.shiftKey) {
                event.preventDefault();
                void send();
              }
            }}
            placeholder="Ask Solomon anything..."
            value={draft}
          />
          <div className="welcome-toolbar">
            <div className="welcome-toolbar-left">
              <button className="welcome-menu" type="button">Select model <ChevronIcon /></button>
              <span aria-hidden="true" className="welcome-toolbar-sep" />
              <button className="welcome-menu" type="button">None <ChevronIcon /></button>
              <button className="welcome-mode is-agent" type="button">
                <span aria-hidden="true" className="welcome-mode-icon"><CrownIcon /></span>
                <span>Agent</span>
              </button>
            </div>
            {isStreaming ? (
              <button aria-label="Stop streaming" className="welcome-send test-chat-stop" onClick={() => onStopStreaming(chat.id)} title="Stop streaming" type="button"><StopIcon /></button>
            ) : <button aria-label="Send" className="welcome-send" disabled={!draft.trim() && images.length === 0} type="submit"><SendIcon /></button>}
          </div>
          </div>
        </form>
        <div aria-label="Read-only Git context" className="welcome-git-controls">
          {workspaceName ? <span className="test-chat-readonly-control is-workspace" title={workspacePath}><FolderIcon />{workspaceName}</span> : null}
          <span className="test-chat-readonly-control"><BranchIcon />main</span>
          <span className="test-chat-readonly-control"><WorktreeIcon />{chat.worktree ?? "Worktree"}</span>
        </div>
      </div>
      {deleteTarget ? (
        <div
          className="test-chat-delete-dialog-backdrop"
          onPointerDown={(event) => {
            if (event.target === event.currentTarget) setDeleteTarget(null);
          }}
          role="presentation"
        >
          <section
            aria-describedby="test-chat-delete-dialog-description"
            aria-labelledby="test-chat-delete-dialog-title"
            aria-modal="true"
            className="test-chat-delete-dialog"
            onPointerDown={(event) => event.stopPropagation()}
            role="dialog"
          >
            <div aria-hidden="true" className="test-chat-delete-dialog-marker"><CloseIcon /></div>
            <p className="test-chat-delete-dialog-eyebrow">Delete confirmation</p>
            <h2 id="test-chat-delete-dialog-title">Delete this message?</h2>
            <p id="test-chat-delete-dialog-description">The assistant's reply will also be deleted. This action cannot be undone.</p>
            <div className="test-chat-delete-dialog-actions">
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

function CheckpointLabel({ label }: { label: string }) {
  return (
    <span aria-label={`Checkpoint ${label}`} className="test-chat-checkpoint-label" title={`Checkpoint ${label}`}>
      {label}
    </span>
  );
}

function InterruptedGenerationMarker() {
  return (
    <div aria-label="Generation stopped" className="test-chat-interrupted" role="status">
      <span aria-hidden="true" className="test-chat-interrupted-line is-left" />
      <span className="test-chat-interrupted-label">generation stopped</span>
      <span aria-hidden="true" className="test-chat-interrupted-line is-right" />
    </div>
  );
}

async function snapshotComposerImage(image: ComposerImageAttachment): Promise<FakeChatImage> {
  if (!image.blob) return { name: image.name, url: image.url };
  return { name: image.name, url: await blobToDataUrl(image.blob) };
}

function blobToDataUrl(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.addEventListener("error", () => reject(reader.error ?? new Error("Unable to read image attachment")));
    reader.addEventListener("load", () => {
      if (typeof reader.result === "string") resolve(reader.result);
      else reject(new Error("Unable to encode image attachment"));
    });
    reader.readAsDataURL(blob);
  });
}

function ChatImageAttachments({ images }: { images: FakeChatImage[] }) {
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
      <div aria-label="Attached images" className="composer-image-previews test-chat-message-images">
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
          className="composer-image-lightbox test-chat-message-lightbox"
          onClick={() => setSelectedImageIndex(null)}
          role="dialog"
        >
          <button aria-label="Close preview" className="test-chat-image-lightbox-close" onClick={() => setSelectedImageIndex(null)} type="button"><CloseIcon /></button>
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
          {images.length > 1 ? <div className="composer-image-lightbox-count test-chat-image-lightbox-count">{selectedImageIndex! + 1} / {images.length}</div> : null}
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

function CompactionCard({ message }: { message: FakeChatMessage }) {
  const retainedMessages = message.retainedMessages ?? [];

  return (
    <section aria-label="Context compaction" className="test-chat-compaction" data-message-kind="compaction">
      <details>
        <summary className="test-chat-compaction-summary">
          <span className="test-chat-compaction-title">Context compacted</span>
          <svg aria-hidden="true" className="test-chat-compaction-chevron" viewBox="0 0 24 24">
            <path d="m7 10 5 5 5-5" />
          </svg>
        </summary>
        <div className="test-chat-compaction-body">
          <details className="test-chat-compaction-section" open>
            <summary className="test-chat-compaction-section-summary">
              <span className="test-chat-compaction-eyebrow">Summary</span>
              <svg aria-hidden="true" className="test-chat-compaction-section-chevron" viewBox="0 0 24 24">
                <path d="m7 10 5 5 5-5" />
              </svg>
            </summary>
            <div className="test-chat-compaction-section-body">
              <div className="test-chat-compaction-markdown">
                <MarkdownContent content={message.summary ?? ""} />
              </div>
            </div>
          </details>
          <details className="test-chat-compaction-section" open>
            <summary className="test-chat-compaction-section-summary">
              <span className="test-chat-compaction-eyebrow">Recent messages</span>
              <svg aria-hidden="true" className="test-chat-compaction-section-chevron" viewBox="0 0 24 24">
                <path d="m7 10 5 5 5-5" />
              </svg>
            </summary>
            <div className="test-chat-compaction-section-body">
              <div className="test-chat-retained-messages">
                {retainedMessages.map((retainedMessage, index) => (
                  <div className={`test-chat-retained-message is-${retainedMessage.role}`} key={`${retainedMessage.role}-${index}`}>
                    <div className="test-chat-retained-content">
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

function ChatMessageTurn({ checkpoint, index, message, onRequestDelete }: { checkpoint?: CheckpointMetadata; index: number; message: FakeChatMessage; onRequestDelete?: () => void }) {
  const [isReasoningCollapsed, setIsReasoningCollapsed] = useState(true);
  const canCollapseReasoning = message.role === "assistant" && Boolean(message.reasoning || message.thoughtFor !== undefined);

  function handleMessageClick(event: MouseEvent<HTMLElement>) {
    if (!canCollapseReasoning) return;
    if (event.target instanceof HTMLElement && event.target.closest("a, button, input, textarea, select, summary")) return;
    setIsReasoningCollapsed((current) => !current);
  }

  return (
    <div className={`test-chat-turn is-${message.role}`} data-checkpoint={checkpoint?.label}>
      <article className={`test-chat-message is-${message.role}`} onClick={canCollapseReasoning ? handleMessageClick : undefined}>
        {checkpoint ? <CheckpointLabel label={checkpoint.label} /> : null}
        {message.images?.length ? <ChatImageAttachments images={message.images} /> : null}
        {canCollapseReasoning ? <ReasoningBlock isCollapsed={isReasoningCollapsed} message={message} onToggle={() => setIsReasoningCollapsed((current) => !current)} /> : null}
        {message.toolCalls?.length ? <ToolActivity checkpoint={checkpoint} toolCalls={message.toolCalls} /> : null}
        <MarkdownContent content={message.content} />
      </article>
      {message.status === "interrupted" ? <InterruptedGenerationMarker /> : null}
      <MessageFooter index={index} message={message} onRequestDelete={onRequestDelete} />
    </div>
  );
}

function ToolActivity({ checkpoint, toolCalls }: { checkpoint?: CheckpointMetadata; toolCalls: FakeChatToolCall[] }) {
  const [isCollapsed, setIsCollapsed] = useState(false);
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
    <section aria-label="Tool activity" className={`test-chat-tool-activity${isCollapsed ? " is-collapsed" : ""}`} onClick={(event) => event.stopPropagation()} ref={activityRef}>
      {!isCollapsed ? toolCalls.map((tool, index) => (
        <ToolCallCard key={tool.id} checkpoint={resolveToolCheckpoint(tool, checkpoint, index)} tool={tool} />
      )) : null}
      <button aria-expanded={!isCollapsed} aria-label={collapseLabel} className="test-chat-tool-collapse-all" onClick={toggleCollapsed} type="button">
        {collapseLabel}
      </button>
    </section>
  );
}

function ToolCallCard({ checkpoint, tool }: { checkpoint: CheckpointMetadata; tool: FakeChatToolCall }) {
  const status = tool.status ?? tool.result?.status ?? "running";
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
  const isRename = tool.name === "editFile" && Boolean(tool.renameTo);
  const isDelete = tool.name === "editFile" && Boolean(tool.delete);
  const isInlineTool = tool.name === "shell" || tool.name === "readFile" || isFind || tool.name === "listDir" || tool.name === "tree" || tool.name === "editFile" || tool.name === "loadSkill" || tool.name === "docsRetrieval" || isCreatePlan || isEditPlan || isBuildPlan || isAddTodo || isTodoList || isCheckTodo || isRemoveTodo || isCheckPlan || isDeletePlan || isFetchWeb || isWebSearch;
  const isDangerousArgument = isDelete || isRemoveTodo || isDeletePlan;
  const inlineCommand = tool.name === "editFile" && tool.renameTo
    ? `${tool.input ?? ""} → ${tool.renameTo}`
    : tool.input;
  const toolParameters = isFind || isCreatePlan || isAddTodo || isFetchWeb || isWebSearch
    ? tool.parameters ?? (isFind && tool.input ? [{ label: "pattern", value: tool.input }] : [])
    : [];
  const [isOpen, setIsOpen] = useState(false);

  return (
    <div className={`test-chat-tool-card is-${status}`} data-checkpoint={checkpoint.label}>
      <CheckpointLabel label={checkpoint.label} />
      <details aria-busy={status === "running"} onToggle={(event) => setIsOpen(event.currentTarget.open)} open={isOpen}>
        <summary className="test-chat-tool-summary">
          <i aria-hidden="true" className="test-chat-tool-status-dot" />
          <HammerIcon />
          {tool.intent ? <span className="test-chat-tool-intent" title={tool.intent}>{tool.intent}</span> : null}
          <svg aria-hidden="true" className="test-chat-tool-chevron" viewBox="0 0 24 24">
            <path d="m7 10 5 5 5-5" />
          </svg>
        </summary>
        <div className="test-chat-tool-body">
          <div className="test-chat-tool-execution">
            <div className="test-chat-tool-name-row">
              {isInlineTool ? (
                <>
                  <strong className="test-chat-tool-name test-chat-tool-label">Tool:</strong>
                  <strong className="test-chat-tool-name">{tool.name}</strong>
                  {isFind ? <span className="test-chat-tool-command">{tool.mode ?? "text"}</span> : inlineCommand ? <span className={`test-chat-tool-command${isDangerousArgument ? " is-delete" : ""}`}>{inlineCommand}</span> : null}
                </>
              ) : (
                <strong className="test-chat-tool-name">{tool.name}</strong>
              )}
            </div>
            {tool.input && !isInlineTool ? (
              <div className="test-chat-tool-input">
                <span aria-hidden="true" className="test-chat-tool-prompt">$</span>
                <pre><code>{tool.input}</code></pre>
              </div>
            ) : null}
            {isCheckPlan && tool.full ? (
              <div className="test-chat-tool-parameters">
                <div className="test-chat-tool-parameter">
                  <span className="test-chat-tool-parameter-value">full</span>
                </div>
              </div>
            ) : null}
            {(isFind || isCreatePlan || isAddTodo || isFetchWeb || isWebSearch) && toolParameters.length ? (
              <div className="test-chat-tool-parameters">
                {toolParameters.map((parameter) => (
                  <div className={`test-chat-tool-parameter${isAddTodo && parameter.label === "todo" ? " is-todo" : ""}`} key={`${parameter.label}-${parameter.value}`}>
                    {isAddTodo && parameter.label === "todo" ? (
                      <>
                        <span aria-hidden="true" className="test-chat-tool-todo-checkbox" />
                        <span className="test-chat-tool-parameter-value">{parameter.value}</span>
                      </>
                    ) : (
                      <>
                        <span className="test-chat-tool-parameter-label">{parameter.label}:</span>
                        <span className="test-chat-tool-parameter-value"> {parameter.value}</span>
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
              tool.name === "readFile" || (tool.name === "editFile" && status === "success") || (tool.name === "loadSkill" && status === "success") || (tool.name === "docsRetrieval" && status === "success") || (isCreatePlan && status === "success") || (isEditPlan && status === "success") || (isBuildPlan && status === "success") || (isAddTodo && status === "success") || (isCheckTodo && status === "success") || (isRemoveTodo && status === "success") || (isCheckPlan && status === "success") || (isDeletePlan && status === "success") || (isFetchWeb && status === "success") ? null : <ToolResultCard result={tool.result} toolName={tool.name} />
            ) : null}
          </div>
        </div>
      </details>
    </div>
  );
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
    <div className={`test-chat-tool-edit-diff${isExpanded ? " is-expanded" : ""}`}>
      {visibleOldLines.map((line, index) => (
        <button
          aria-expanded={canExpand ? isExpanded : undefined}
          aria-label={canExpand ? (isExpanded ? "Collapse removed lines" : "Show all removed lines") : undefined}
          className="test-chat-tool-edit-line is-old"
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
          className="test-chat-tool-edit-line is-new"
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

function resolveToolCheckpoint(tool: FakeChatToolCall, parentCheckpoint: CheckpointMetadata | undefined, index: number): CheckpointMetadata {
  const hasExplicitSequence = typeof tool.checkpointSeq === "number" && Number.isFinite(tool.checkpointSeq);
  const sequence = hasExplicitSequence
    ? Math.max(0, Math.floor(tool.checkpointSeq!))
    : Math.max(0, (parentCheckpoint?.sequence ?? 0) + index + 1);
  const branch = tool.checkpointBranch ?? parentCheckpoint?.branch ?? "";

  return { branch, label: formatCheckpointLabel(sequence, branch), sequence };
}

function ToolResultCard({ result, toolName }: { result: FakeChatToolResult; toolName: string }) {
  if (toolName === "find") {
    return <CountedToolResultCard collapseLabel="find results" plural="matches" result={result} singular="match" />;
  }
  if (toolName === "listDir") {
    return <CountedToolResultCard collapseLabel="directory entries" plural="entries" result={result} singular="entry" />;
  }
  if (toolName === "todoList") {
    return <TodoListResultCard result={result} />;
  }
  return <GenericToolResultCard result={result} />;
}

function GenericToolResultCard({ result }: { result: FakeChatToolResult }) {
  const [isExpanded, setIsExpanded] = useState(false);
  const output = (result.output ?? "").replace(/\r\n?/g, "\n");
  const firstLine = result.summary ?? (output.split("\n")[0] || "—");
  const displayedOutput = isExpanded ? output || "—" : firstLine;
  const toggleLabel = isExpanded ? "Collapse result" : "Show full result";

  return (
    <div className={`test-chat-tool-result is-${result.status}${isExpanded ? " is-expanded" : ""}`}>
      <button aria-expanded={isExpanded} aria-label={toggleLabel} className="test-chat-tool-result-toggle" onClick={() => setIsExpanded((current) => !current)} type="button">
        <span className="test-chat-tool-result-prefix">Result:</span>
        <span className="test-chat-tool-result-text">{displayedOutput}</span>
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
  result: FakeChatToolResult;
  singular: string;
}) {
  const [isExpanded, setIsExpanded] = useState(false);
  const outputItems = (result.output ?? "").replace(/\r\n?/g, "\n").split("\n").filter(Boolean);
  const items = result.items?.length ? result.items : outputItems;
  const count = typeof result.count === "number" ? result.count : items.length;

  if (!isExpanded) {
    return (
      <div className={`test-chat-tool-result is-${result.status}`}>
        <button aria-expanded={false} aria-label={`Show ${collapseLabel}`} className="test-chat-tool-result-toggle" onClick={() => setIsExpanded(true)} type="button">
          <span className="test-chat-tool-result-prefix">Result:</span>
          <span className="test-chat-tool-result-text">{count} {count === 1 ? singular : plural}</span>
        </button>
      </div>
    );
  }

  return (
    <div className={`test-chat-tool-result test-chat-tool-result-list is-${result.status} is-expanded`}>
      <div className="test-chat-tool-result-list-label">Result:</div>
      <div className="test-chat-tool-result-items">
        {items.map((item) => (
          <button aria-label={`Collapse ${collapseLabel}`} className="test-chat-tool-result-item" key={item} onClick={() => setIsExpanded(false)} type="button">
            {item}
          </button>
        ))}
      </div>
    </div>
  );
}

function TodoListResultCard({ result }: { result: FakeChatToolResult }) {
  const [isExpanded, setIsExpanded] = useState(false);
  const items = result.todoItems?.length ? result.todoItems : parseTodoItems(result.output);
  const orderedItems = [...items.filter((item) => item.checked), ...items.filter((item) => !item.checked)];
  const count = typeof result.count === "number" ? result.count : items.length;
  const completed = typeof result.completed === "number" ? result.completed : items.filter((item) => item.checked).length;
  const summary = `${count} ${count === 1 ? "todo" : "todos"}, ${completed} completed`;

  if (!isExpanded) {
    return (
      <div className={`test-chat-tool-result is-${result.status}`}>
        <button aria-expanded={false} aria-label="Show todo list" className="test-chat-tool-result-toggle" onClick={() => setIsExpanded(true)} type="button">
          <span className="test-chat-tool-result-prefix">Result:</span>
          <span className="test-chat-tool-result-text">{summary}</span>
        </button>
      </div>
    );
  }

  return (
    <div className={`test-chat-tool-result test-chat-tool-result-list is-${result.status} is-expanded`}>
      <div className="test-chat-tool-result-list-label">Result:</div>
      <div className="test-chat-tool-result-items">
        {orderedItems.map((item) => (
          <button aria-label={`Collapse todo list: ${item.text}`} className="test-chat-tool-result-todo-item" key={`${item.checked}-${item.text}`} onClick={() => setIsExpanded(false)} type="button">
            <span aria-hidden="true" className={`test-chat-tool-result-todo-checkbox${item.checked ? " is-checked" : ""}`} />
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

function indexChatMessages(messages: FakeChatMessage[]): IndexedChatMessage[] {
  let fallbackSequence = -1;
  let fallbackBranch = "";

  return messages.map((message, index) => {
    if (message.kind === "compaction") {
      return { index, message };
    }

    const hasExplicitSequence = typeof message.checkpointSeq === "number" && Number.isFinite(message.checkpointSeq);
    if (hasExplicitSequence) {
      fallbackSequence = Math.max(0, Math.floor(message.checkpointSeq!));
      fallbackBranch = message.checkpointBranch ?? "";
    } else if (message.role === "user" || fallbackSequence < 0) {
      fallbackSequence += 1;
      fallbackBranch = "";
    }

    const sequence = hasExplicitSequence ? Math.max(0, Math.floor(message.checkpointSeq!)) : Math.max(0, fallbackSequence);
    const branch = message.checkpointBranch ?? fallbackBranch;
    if (message.role === "assistant" && message.toolCalls?.length) {
      fallbackSequence = Math.max(fallbackSequence, sequence + message.toolCalls.length);
    }
    const label = formatCheckpointLabel(sequence, branch);

    return {
      checkpoint: { branch, label, sequence },
      index,
      message,
    };
  });
}

function formatCheckpointLabel(sequence: number, branch: string) {
  return `[#${String(sequence).padStart(3, "0")}${branch}]`;
}

function MessageFooter({ index, message, onRequestDelete }: { index: number; message: FakeChatMessage; onRequestDelete?: () => void }) {
  const [copied, setCopied] = useState(false);
  const [isStatsOpen, setIsStatsOpen] = useState(false);
  const statsRef = useRef<HTMLDivElement>(null);
  const createdAt = message.createdAt ?? FIXTURE_MESSAGE_START_TIME + index * 60_000;
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
    <footer className="test-chat-message-footer">
      <time dateTime={new Date(createdAt).toISOString()}>{formatMessageTime(createdAt)}</time>
      {stats ? (
        <div className="test-chat-stats-control" ref={statsRef}>
          <button
            aria-controls={`message-stats-${message.id}`}
            aria-expanded={isStatsOpen}
            aria-label={isStatsOpen ? "Hide turn statistics" : "Show turn statistics"}
            className="test-chat-stats-trigger"
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
        className="test-chat-copy-message"
        onClick={() => void copyMessage()}
        title={copied ? "Message copied" : "Copy message"}
        type="button"
      >
        {copied ? <CheckIcon /> : <CopyIcon />}
      </button>
      {message.workedFor !== undefined ? <span className="test-chat-worked-for">worked for {formatWorkedDuration(message.workedFor)}</span> : null}
      {onRequestDelete ? (
        <button aria-label="Delete message" className="test-chat-delete-message" onClick={onRequestDelete} title="Delete message" type="button">
          <CloseIcon />
        </button>
      ) : null}
    </footer>
  );
}

function MessageStatsPopover({ id, stats, workedFor }: { id: string; stats: FakeChatStats; workedFor?: number }) {
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
    <div aria-label="Turn statistics" className="test-chat-stats-popover" id={id} role="dialog">
      <div className="test-chat-stats-title">Turn statistics</div>
      <dl>
        {rows.map(([label, value]) => (
          <div className={`test-chat-stats-row${label === "total" ? " is-total" : ""}`} key={label}>
            <dt>{label}</dt>
            <dd>{value}</dd>
          </div>
        ))}
      </dl>
    </div>
  );
}

function ReasoningBlock({ isCollapsed, message, onToggle }: { isCollapsed: boolean; message: FakeChatMessage; onToggle: () => void }) {
  const reasoning = message.reasoning?.trim();

  return (
    <div
      aria-expanded={!isCollapsed}
      aria-label="Model reasoning"
      className={`test-chat-reasoning${isCollapsed ? " is-collapsed" : ""}`}
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
      {!isCollapsed && reasoning ? <div className="test-chat-reasoning-copy">{reasoning}</div> : null}
      {message.thoughtFor !== undefined ? <div className="test-chat-thought-for">thought for {formatWorkedDuration(message.thoughtFor)}</div> : null}
    </div>
  );
}

function formatMessageTime(timestamp: number) {
  return new Intl.DateTimeFormat("it-IT", { hour: "2-digit", minute: "2-digit" }).format(timestamp);
}

function formatWorkedDuration(seconds: number) {
  if (!Number.isFinite(seconds) || seconds <= 0) return "0s";
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds - hours * 3600) / 60);
  const remainingSeconds = Number((seconds - hours * 3600 - minutes * 60).toFixed(3));
  return `${hours ? `${hours}h` : ""}${minutes || hours ? `${minutes}m` : ""}${remainingSeconds}s`;
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
          <a {...props} className={`${props.className ?? ""}${isFileLink(href) ? " test-chat-file-link" : ""}`.trim()} href={href} rel="noreferrer" target="_blank">{children}</a>
        ),
        input: (props) => <input {...props} aria-label="Completed activity" className="test-chat-checkbox" disabled />,
        pre: ({ children }) => <CodeBlock>{children}</CodeBlock>,
        table: ({ children, ...props }) => (
          <div className="test-chat-table-wrap">
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
    <div className="test-chat-code-block">
      <div className="test-chat-code-toolbar">
        <div className="test-chat-code-language">{language ?? "Code"}</div>
        <button
          aria-label={copied ? "Code copied" : "Copy code"}
          className="test-chat-copy-code"
          onClick={() => void copyCode()}
          title={copied ? "Code copied" : "Copy code"}
          type="button"
        >
          {copied ? <CheckIcon /> : <CopyIcon />}
        </button>
      </div>
      <pre>
        <code className="test-chat-code-lines">
          {codeLines.map((line, index) => (
            <span className="test-chat-code-line" key={index}>
              <span aria-hidden="true" className="test-chat-line-number">{index + 1}</span>
              <span className="test-chat-line-content">{line || " "}</span>
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

export function TestChatTopbar({ breadcrumb, onOpenFolder, title }: { breadcrumb?: string; onOpenFolder: () => void; title: string }) {
  const topbarRef = useRef<HTMLDivElement>(null);
  const contextRef = useRef<HTMLButtonElement>(null);
  const folderName = breadcrumb ?? "Test chats";
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
    <div aria-label={`${folderName} / ${title}`} className="test-chat-topbar" ref={topbarRef}>
      <button aria-label={`Back to new chat in ${folderName}`} className={`test-chat-topbar-context${showContext ? "" : " is-hidden"}`} onClick={onOpenFolder} ref={contextRef} type="button">
        <FolderIcon />
        <span className="test-chat-topbar-folder">{folderName}</span>
        <span aria-hidden="true" className="test-chat-topbar-slash">/</span>
      </button>
      <span className="test-chat-topbar-title" title={title}>{visibleTitle}</span>
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


function AsciiCrown() {
  return (
    <pre aria-hidden="true" className="test-chat-crown">
      {asciiBanner.trimEnd().split(/\r?\n/).map((line, rowIndex) => (
        <span className="test-chat-crown-row" key={rowIndex}>
          {Array.from(line).map((character, columnIndex) => (
            <span
              className="test-chat-crown-cell"
              key={columnIndex}
              style={{ color: `#${asciiColorRows[rowIndex]?.[columnIndex] ?? "ffc704"}` }}
            >
              {character}
            </span>
          ))}
        </span>
      ))}
    </pre>
  );
}

function ChevronIcon() {
  return <svg aria-hidden="true" className="welcome-chevron" viewBox="0 0 24 24"><path d="m7 10 5 5 5-5" /></svg>;
}

function CrownIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="M12 1.8v16.2M9.4 4.4h5.2M4.4 15.8V12c0-1.6 1.2-2.5 2.6-2.5 1.4 0 2.4 1.1 2.6 2.5.3-2 1-3.6 2.4-3.6 1.4 0 2.1 1.6 2.4 3.6.2-1.4 1.2-2.5 2.6-2.5 1.4 0 2.6.9 2.6 2.5v3.8" />
      <path d="M4 16.2h16l-.6 3.8H4.6z" />
    </svg>
  );
}

function SendIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="m5 12 14-7-4 14-3-6-7-1Z" /><path d="m12 13 3-3" /></svg>;
}

function InfoIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><circle cx="12" cy="12" r="8.5" /><path d="M12 10.5v5M12 7.5h.01" /></svg>;
}

function StopIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><rect height="9" rx="1.5" width="9" x="7.5" y="7.5" /></svg>;
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
    <svg aria-hidden="true" className="test-chat-tool-hammer-icon" viewBox="0 0 80.4375 96.03515625">
      <g fillRule="nonzero" transform="scale(1,-1) translate(0,-96.03515625)">
        <path d="M 12.58984375,18.025390625 L 8.03515625,22.55859375 Q 6.251953125,24.36328125 6.3701171875,26.3291015625 Q 6.48828125,28.294921875 8.59375,30.12109375 L 38.84375,56.67578125 L 46.814453125,48.705078125 L 20.15234375,18.583984375 Q 18.3046875,16.5 16.349609375,16.3603515625 Q 14.39453125,16.220703125 12.58984375,18.025390625 Z M 61.703125,41.59375 L 59.640625,43.61328125 Q 58.78125,44.4296875 58.5986328125,45.095703125 Q 58.416015625,45.76171875 58.48046875,46.771484375 L 58.716796875,49.62890625 L 56.482421875,51.884765625 L 52.20703125,51.025390625 Q 50.896484375,50.767578125 50.0048828125,51.00390625 Q 49.11328125,51.240234375 48.296875,52.056640625 L 42.044921875,58.30859375 Q 40.94921875,59.42578125 40.6484375,60.6826171875 Q 40.34765625,61.939453125 41.013671875,63.59375 L 43.18359375,69.05078125 Q 40.1328125,70.984375 36.6201171875,71.0166015625 Q 33.107421875,71.048828125 29.08984375,69.953125 Q 28.359375,69.73828125 27.671875,69.953125 Q 26.984375,70.16796875 26.51171875,70.640625 Q 25.931640625,71.306640625 25.888671875,72.2841796875 Q 25.845703125,73.26171875 26.791015625,74.185546875 Q 29.111328125,76.505859375 32.0546875,77.81640625 Q 34.998046875,79.126953125 38.2314453125,79.470703125 Q 41.46484375,79.814453125 44.6982421875,79.234375 Q 47.931640625,78.654296875 50.853515625,77.2041015625 Q 53.775390625,75.75390625 56.07421875,73.4765625 L 61.810546875,67.783203125 Q 63.25,66.365234375 63.744140625,64.96875 Q 64.23828125,63.572265625 63.916015625,62.154296875 L 63.03515625,58.39453125 L 65.291015625,56.16015625 L 68.169921875,56.4609375 Q 68.814453125,56.50390625 69.2978515625,56.439453125 Q 69.78125,56.375 70.2431640625,56.1064453125 Q 70.705078125,55.837890625 71.263671875,55.279296875 L 73.34765625,53.216796875 Q 74.12109375,52.443359375 74.1533203125,51.5517578125 Q 74.185546875,50.66015625 73.43359375,49.88671875 L 65.033203125,41.529296875 Q 64.259765625,40.755859375 63.3896484375,40.7880859375 Q 62.51953125,40.8203125 61.703125,41.59375 Z" />
      </g>
    </svg>
  );
}

function FolderIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M3 6a2 2 0 0 1 2-2h5l2 2h7a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2Z" /></svg>;
}
