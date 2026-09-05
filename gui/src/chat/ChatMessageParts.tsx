import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import type { ChatImage, ChatMessage } from "./chatTypes";
import { ComposerCrownIcon } from "./ChatComposer";
import type { ActiveSubagent } from "./chatViewTypes";
import { CloseIcon } from "./ChatIcons";
import { MarkdownContent } from "./MarkdownContent";

export function CheckpointLabel({ className, label }: { className?: string; label: string }) {
  return (
    <span aria-label={`Checkpoint ${label}`} className={`chat-checkpoint-label${className ? ` ${className}` : ""}`} title={`Checkpoint ${label}`}>
      {label}
    </span>
  );
}

export function InterruptedGenerationMarker() {
  return (
    <div aria-label="Generation stopped" className="chat-interrupted" role="status">
      <span aria-hidden="true" className="chat-interrupted-line is-left" />
      <span className="chat-interrupted-label">generation stopped</span>
      <span aria-hidden="true" className="chat-interrupted-line is-right" />
    </div>
  );
}

export function ChatImageAttachments({ images }: { images: ChatImage[] }) {
  const [selectedImageIndex, setSelectedImageIndex] = useState<number | null>(null);
  const selectedImage = selectedImageIndex === null ? undefined : images[selectedImageIndex];

  useEffect(() => {
    if (selectedImageIndex === null) return;
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        event.stopPropagation();
        setSelectedImageIndex(null);
        return;
      }
      if (images.length > 1 && (event.key === "ArrowLeft" || event.key === "ArrowRight")) {
        event.preventDefault();
        event.stopPropagation();
        setSelectedImageIndex((current) => {
          if (current === null) return current;
          const offset = event.key === "ArrowLeft" ? -1 : 1;
          return (current + offset + images.length) % images.length;
        });
        return;
      }
      event.preventDefault();
      event.stopPropagation();
    };
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    if (document.activeElement instanceof HTMLElement) document.activeElement.blur();
    document.addEventListener("keydown", closeOnEscape, true);
    return () => {
      document.body.style.overflow = previousOverflow;
      document.removeEventListener("keydown", closeOnEscape, true);
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
      {selectedImage ? createPortal(
        <div
          aria-label={`Image preview: ${selectedImage.name}`}
          aria-modal="true"
          className="composer-image-lightbox chat-message-lightbox"
          onClick={() => setSelectedImageIndex(null)}
          onMouseDown={(event) => event.stopPropagation()}
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
        </div>,
        document.body,
      ) : null}
    </>
  );
}

export function ChatImageArrowIcon({ direction }: { direction: "left" | "right" }) {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d={direction === "left" ? "m14 6-6 6 6 6" : "m10 6 6 6-6 6"} />
    </svg>
  );
}

export function CompactionCard({ message }: { message: ChatMessage }) {
  const retainedMessages = message.retainedMessages ?? [];

  return (
    <section aria-label="Context compaction" className="chat-compaction" data-message-kind="compaction">
      <details className="chat-compaction-card">
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

export function ModeSwitchNotice({ onCancel, progress }: { onCancel: () => void; progress: number }) {
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

export function SubagentActivityIndicator({ isExpanded, onOpenSubagent, onToggle, subagents }: { isExpanded: boolean; onOpenSubagent: (messageID: string, toolID: string) => void; onToggle: () => void; subagents: ActiveSubagent[] }) {
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
