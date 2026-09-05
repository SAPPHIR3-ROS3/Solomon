import { useEffect, useRef, useState } from "react";
import type { ChatMessage, ChatStats } from "./chatTypes";
import { CheckIcon, CloseIcon, CopyIcon, InfoIcon } from "./ChatIcons";
import { copyTextFallback } from "./chatClipboard";

export function MessageFooter({ index, message, onRequestDelete }: { index: number; message: ChatMessage; onRequestDelete?: () => void }) {
  const [copied, setCopied] = useState(false);
  const [isStatsOpen, setIsStatsOpen] = useState(false);
  const [legacyCreatedAt] = useState(() => Date.now());
  const statsRef = useRef<HTMLDivElement>(null);
  const createdAt = message.createdAt ?? legacyCreatedAt;
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

export function WorkedForCounter({ isLive, seconds }: { isLive: boolean; seconds: number }) {
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

export function ReasoningBlock({ isCollapsed, message, onToggle }: { isCollapsed: boolean; message: ChatMessage; onToggle: () => void }) {
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
