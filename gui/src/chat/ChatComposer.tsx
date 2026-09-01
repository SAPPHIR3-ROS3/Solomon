import { useEffect, useRef, useState, type CSSProperties, type Ref } from "react";
import { fetchProjectSidebarData, normalizeReasoningEffort, saveFastMode, saveReasoningEffort, type ReasoningEffort } from "../projects/projects";
import { AtMentionInput } from "../home/AtMentionInput";
import type { ComposerImageAttachment } from "./composerTypes";
import { ModelControl } from "../home/ModelControl";

const reasoningOptions = [
  { value: "none", label: "None" },
  { value: "low", label: "Low" },
  { value: "medium", label: "Medium" },
  { value: "high", label: "High" },
  { value: "xhigh", label: "Extra high" },
  { value: "max", label: "Max" },
] as const;

export type ChatComposerProps = {
  "aria-label": string;
  className?: string;
  formRef?: Ref<HTMLFormElement>;
  initialMode?: "agent" | "chat";
  initialReasoning?: ReasoningEffort;
  initialFastMode?: boolean;
  isSending?: boolean;
  isStreaming?: boolean;
  mode?: "agent" | "chat";
  modeSwitchPending?: boolean;
  openMenu?: ChatComposerMenu;
  onModeChange?: (mode: "agent" | "chat") => void;
  onOpenMenuChange?: (menu: ChatComposerMenu) => void;
  onSend?: (content: string, images: ComposerImageAttachment[]) => void | Promise<void>;
  onStopStreaming?: () => void;
  onFastModeChange?: (enabled: boolean) => void;
  onReasoningChange?: (effort: ReasoningEffort) => void;
  projectID?: string;
  resetKey?: unknown;
};

export type ChatComposerMenu = "model" | "reasoning" | null;

export function ChatComposer({
  "aria-label": ariaLabel,
  className = "welcome-composer",
  formRef,
  initialFastMode = false,
  initialMode = "agent",
  initialReasoning = "none",
  isSending = false,
  isStreaming = false,
  mode: controlledMode,
  modeSwitchPending = false,
  openMenu: controlledOpenMenu,
  onFastModeChange,
  onModeChange,
  onOpenMenuChange,
  onReasoningChange,
  onSend,
  onStopStreaming,
  projectID,
  resetKey,
}: ChatComposerProps) {
  const [draft, setDraft] = useState("");
  const [images, setImages] = useState<ComposerImageAttachment[]>([]);
  const [reasoning, setReasoning] = useState<ReasoningEffort>(initialReasoning);
  const [fastOn, setFastOn] = useState(initialFastMode);
  const [fastModeAvailable, setFastModeAvailable] = useState(false);
  const [selectedProvider, setSelectedProvider] = useState("");
  const fastModeRequestRef = useRef(0);
  const [internalMode, setInternalMode] = useState<"agent" | "chat">(initialMode);
  const [internalOpenMenu, setInternalOpenMenu] = useState<ChatComposerMenu>(null);
  const mode = controlledMode ?? internalMode;
  const openMenu = controlledOpenMenu === undefined ? internalOpenMenu : controlledOpenMenu;

  function setOpenMenu(menu: ChatComposerMenu) {
    if (controlledOpenMenu === undefined) setInternalOpenMenu(menu);
    onOpenMenuChange?.(menu);
  }

  useEffect(() => {
    setReasoning(initialReasoning);
  }, [initialReasoning]);

  useEffect(() => {
    setFastOn(initialFastMode);
  }, [initialFastMode]);

  useEffect(() => {
    setInternalMode(initialMode);
  }, [initialMode]);

  useEffect(() => {
    let cancelled = false;
    void fetchProjectSidebarData()
      .then((data) => {
        if (cancelled) return;
        setReasoning(data.reasoningEffort);
        setFastOn(data.fastMode);
      })
      .catch(() => {
        // The composer remains usable when the daemon is not available yet.
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    setDraft("");
    setImages([]);
    setOpenMenu(null);
  }, [resetKey]);

  function setMode(nextMode: "agent" | "chat") {
    setInternalMode(nextMode);
    onModeChange?.(nextMode);
  }

  function updateFastMode(nextFastOn: boolean) {
    const previous = fastOn;
    const requestID = fastModeRequestRef.current + 1;
    fastModeRequestRef.current = requestID;
    setFastOn(nextFastOn);
    onFastModeChange?.(nextFastOn);
    void saveFastMode(nextFastOn)
      .then((saved) => {
        if (requestID !== fastModeRequestRef.current) return;
        setFastOn(saved);
        onFastModeChange?.(saved);
      })
      .catch(() => {
        if (requestID !== fastModeRequestRef.current) return;
        setFastOn(previous);
        onFastModeChange?.(previous);
      });
  }

  function updateReasoning(nextReasoning: ReasoningEffort) {
    const previous = reasoning;
    setReasoning(nextReasoning);
    onReasoningChange?.(nextReasoning);
    void saveReasoningEffort(nextReasoning)
      .then((saved) => {
        setReasoning(saved);
        onReasoningChange?.(saved);
      })
      .catch(() => {
        setReasoning(previous);
        onReasoningChange?.(previous);
      });
  }

  async function submit() {
    const content = draft.trim();
    if (isSending || modeSwitchPending || (!content && images.length === 0) || !onSend) return;
    await onSend(content, images);
    setDraft("");
    setImages([]);
  }

  return (
    <form
      aria-label={ariaLabel}
      className={className}
      onSubmit={(event) => {
        event.preventDefault();
        void submit();
      }}
      ref={formRef}
    >
      <AtMentionInput
        aria-label={ariaLabel}
        className="welcome-input"
        images={images}
        onChange={setDraft}
        onImagesChange={setImages}
        onKeyDown={(event) => {
          if (event.key === "Enter" && !event.shiftKey) {
            event.preventDefault();
            void submit();
          }
        }}
        placeholder="Ask Solomon anything..."
        projectID={projectID}
        value={draft}
      />
      <div className="welcome-toolbar">
        <div className="welcome-toolbar-left">
          <ModelControl
            onFastModeAvailableChange={(available) => {
              setFastModeAvailable(available);
            }}
            onModelChange={(choice) => {
              setSelectedProvider(choice.provider);
            }}
            onOpenChange={(open) => setOpenMenu(open ? "model" : null)}
            open={openMenu === "model"}
          />
          <span aria-hidden="true" className="welcome-toolbar-sep" />
          <div className="welcome-toolbar-modes">
            <ReasoningControl
              fastAvailable={fastModeAvailable}
              fastOn={fastOn}
              onChange={updateReasoning}
              onFastChange={updateFastMode}
              onOpenChange={(open) => setOpenMenu(open ? "reasoning" : null)}
              open={openMenu === "reasoning"}
              value={reasoning}
            />
            <button
              aria-pressed={mode === "agent"}
              className={`welcome-mode ${mode === "agent" ? "is-agent" : "is-chat"}`}
              disabled={modeSwitchPending}
              onClick={() => setMode(mode === "agent" ? "chat" : "agent")}
              type="button"
            >
              <span aria-hidden="true" className="welcome-mode-icon">
                {mode === "agent" ? <ComposerCrownIcon /> : <ComposerChatIcon />}
              </span>
              <span>{mode === "agent" ? "Agent" : "Chat"}</span>
            </button>
          </div>
        </div>
        {isStreaming ? (
          <button aria-label="Stop streaming" className="welcome-send chat-stop" onClick={onStopStreaming} title="Stop streaming" type="button">
            <ComposerStopIcon />
          </button>
        ) : (
          <button aria-label="Send" className="welcome-send" disabled={isSending || modeSwitchPending || (!draft.trim() && images.length === 0) || !onSend} type="submit">
            <ComposerSendIcon />
          </button>
        )}
      </div>
    </form>
  );
}

function ReasoningControl({ value, onChange, open, onOpenChange, fastAvailable = false, fastOn = false, onFastChange }: {
  value: ReasoningEffort;
  onChange: (value: ReasoningEffort) => void;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  fastAvailable?: boolean;
  fastOn?: boolean;
  onFastChange?: (on: boolean) => void;
}) {
  const [internalOpen, setInternalOpen] = useState(false);
  const isOpen = open ?? internalOpen;
  const controlRef = useRef<HTMLDivElement>(null);
  const selectedIndex = Math.max(0, reasoningOptions.findIndex((option) => option.value === value));
  const selectedLabel = reasoningOptions[selectedIndex]?.label ?? "None";

  function setOpen(next: boolean) {
    onOpenChange?.(next);
    if (open === undefined) setInternalOpen(next);
  }

  useEffect(() => {
    if (!isOpen) return;
    const close = (event: PointerEvent) => {
      if (!controlRef.current?.contains(event.target as Node)) setOpen(false);
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setOpen(false);
    };
    document.addEventListener("pointerdown", close);
    document.addEventListener("keydown", closeOnEscape);
    return () => {
      document.removeEventListener("pointerdown", close);
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, [isOpen]);

  return (
    <div className="welcome-reasoning" ref={controlRef}>
      <button
        aria-expanded={isOpen}
        aria-label={fastOn ? `Change reasoning level, fast mode on, ${selectedLabel}` : "Change reasoning level"}
        className="welcome-reasoning-label"
        onClick={() => setOpen(!isOpen)}
        type="button"
      >
        <strong className="welcome-reasoning-value">
          <span aria-hidden="true" className="welcome-reasoning-value-sizer">
            <span>Extra high</span>
            {fastOn ? <span className="welcome-reasoning-fast-mark"><ComposerBoltIcon /></span> : null}
          </span>
          <span className="welcome-reasoning-value-text">
            <span>{selectedLabel}</span>
            {fastOn ? <span className="welcome-reasoning-fast-mark is-on"><ComposerBoltIcon /></span> : null}
          </span>
        </strong>
        <ComposerChevronIcon className={isOpen ? "is-open" : undefined} />
      </button>
      {isOpen ? (
        <div className={`welcome-reasoning-popover${fastAvailable ? " has-fast" : ""}`}>
          <header>
            <span>Reasoning level</span>
          </header>
          <div className={`welcome-reasoning-row${fastAvailable ? " has-fast" : ""}`}>
            <div className="welcome-reasoning-scale">
              <input
                aria-label="Reasoning level"
                aria-valuetext={selectedLabel}
                max={reasoningOptions.length - 1}
                min={0}
                onChange={(event) => onChange(normalizeReasoningEffort(reasoningOptions[Number(event.target.value)]?.value ?? "none"))}
                step={1}
                style={{ "--reasoning-fill": `${(selectedIndex / (reasoningOptions.length - 1)) * 100}%` } as CSSProperties}
                type="range"
                value={selectedIndex}
              />
              <div aria-hidden="true" className="welcome-reasoning-ticks">
                {reasoningOptions.map((option, index) => <span className={index <= selectedIndex ? "is-reached" : undefined} key={option.value}><i /><small>{option.label}</small></span>)}
              </div>
            </div>
            {fastAvailable ? (
              <div className="welcome-reasoning-fast-slot">
                <button aria-pressed={fastOn} className={`welcome-reasoning-fast${fastOn ? " is-active" : ""}`} onClick={() => onFastChange?.(!fastOn)} type="button">
                  <ComposerBoltIcon />
                  <span>Fast</span>
                </button>
              </div>
            ) : null}
          </div>
        </div>
      ) : null}
    </div>
  );
}

export function ComposerChevronIcon({ className }: { className?: string }) {
  return <svg aria-hidden="true" className={`welcome-chevron${className ? ` ${className}` : ""}`} viewBox="0 0 24 24"><path d="m7 10 5 5 5-5" /></svg>;
}

export function ComposerCrownIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M12 1.8v16.2M9.4 4.4h5.2M4.4 15.8V12c0-1.6 1.2-2.5 2.6-2.5 1.4 0 2.4 1.1 2.6 2.5.3-2 1-3.6 2.4-3.6 1.4 0 2.1 1.6 2.4 3.6.2-1.4 1.2-2.5 2.6-2.5 1.4 0 2.6.9 2.6 2.5v3.8" /><path d="M4 16.2h16l-.6 3.8H4.6z" /></svg>;
}

export function ComposerChatIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M5 6.5A2.5 2.5 0 0 1 7.5 4h11A2.5 2.5 0 0 1 21 6.5v7A2.5 2.5 0 0 1 18.5 16H12l-4 3v-3H7.5A2.5 2.5 0 0 1 5 13.5V6.5Z" /></svg>;
}

function ComposerBoltIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="m13 2-8 12h6l-1 8 8-12h-6z" /></svg>;
}

export function ComposerSendIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="m5 12 14-7-4 14-3-6-7-1Z" /><path d="m12 13 3-3" /></svg>;
}

export function ComposerStopIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><rect height="9" rx="1.5" width="9" x="7.5" y="7.5" /></svg>;
}
