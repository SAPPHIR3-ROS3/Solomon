import { useEffect, useRef, useState } from "react";
import type { ChatMessage, ChatStats } from "./chatTypes";
import { CheckIcon, CloseIcon, CopyIcon, InfoIcon } from "./ChatIcons";
import { copyText } from "./chatClipboard";

export function MessageFooter({ index, message, onRequestDelete, onRegenerate }: { index: number; message: ChatMessage; onRequestDelete?: () => void; onRegenerate?: () => void }) {
  const [copied, setCopied] = useState(false);
  const [isStatsOpen, setIsStatsOpen] = useState(false);
  const statsRef = useRef<HTMLDivElement>(null);
  const createdAt = message.createdAt;
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
      await copyText(message.content);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1400);
    } catch {
      // Leave the button available so the user can retry.
    }
  }

  return (
    <footer className="chat-message-footer">
      {onRequestDelete ? (
        <button aria-label="Delete message" className="chat-delete-message" onClick={onRequestDelete} title="Delete message" type="button">
          <CloseIcon />
        </button>
      ) : null}
      {onRegenerate ? <button aria-label="Resend message" className="chat-copy-message" onClick={onRegenerate} title="Resend message" type="button"><svg aria-hidden="true" viewBox="0 0 24 24"><path d="M20 7v5h-5M20 12a8 8 0 1 0-2 5M20 7l-3-3" /></svg></button> : null}
      <button
        aria-label={copied ? "Message copied" : "Copy message"}
        className="chat-copy-message"
        onClick={() => void copyMessage()}
        title={copied ? "Message copied" : "Copy message"}
        type="button"
      >
        {copied ? <CheckIcon /> : <CopyIcon />}
      </button>
      {stats ? (
        <div className="chat-stats-control" ref={statsRef}>
          <button
            aria-controls={`message-stats-${message.id}`}
            aria-expanded={isStatsOpen}
            aria-label={isStatsOpen ? "Hide last request statistics" : "Show last request statistics"}
            className="chat-stats-trigger"
            onClick={(event) => {
              event.stopPropagation();
              setIsStatsOpen((current) => !current);
            }}
            title={isStatsOpen ? "Hide last request statistics" : "Show last request statistics"}
            type="button"
          >
            <InfoIcon />
          </button>
          {isStatsOpen ? <MessageStatsPopover id={`message-stats-${message.id}`} stats={stats} workedFor={message.workedFor} /> : null}
        </div>
      ) : null}
      {createdAt !== undefined && Number.isFinite(createdAt) ? <time dateTime={new Date(createdAt).toISOString()}>{formatMessageTime(createdAt)}</time> : null}
    </footer>
  );
}

export function WorkedForCounter({ isLive, seconds }: { isLive: boolean; seconds: number }) {
  const label = `worked for ${formatWorkedDuration(seconds)}`;

  return (
    <span aria-label={isLive ? label : undefined} aria-live={isLive ? "polite" : undefined} className={`chat-worked-for${isLive ? " is-live" : ""}`}>
      {isLive ? (
        <span aria-hidden="true" className="chat-worked-for-wave">
          {label}
        </span>
      ) : label}
    </span>
  );
}

function MessageStatsPopover({ id, stats, workedFor }: { id: string; stats: ChatStats; workedFor?: number }) {
  const rows = [
    ["provider", stats.provider || "—"],
    ["model", stats.model || "—"],
    ["reasoning effort", stats.reasoningEffort || "—"],
    ["context", formatStatsTokenCount(stats.contextTokens)],
    ["cached input", stats.cachedPromptTokens == null || stats.cachedPromptTokens < 0 ? "—" : formatStatsTokenCount(stats.cachedPromptTokens)],
    ["cache write", stats.cacheCreationPromptTokens == null || stats.cacheCreationPromptTokens < 0 ? "—" : formatStatsTokenCount(stats.cacheCreationPromptTokens)],
    ["reasoning", formatStatsTokenCount(stats.reasoningTokens)],
    ["response", formatStatsTokenCount(stats.responseTokens)],
    ["total", formatStatsTokenCount(stats.totalTokens)],
    ["output t/s", `${formatStatsDecimal(stats.outputTokensPerSecond)} t/s`],
    ["first token", `${formatStatsDecimal(stats.ttftSeconds)}s`],
    ["input t/s", `${formatStatsDecimal(stats.promptTokensPerSecond)} t/s`],
    ["turn duration", workedFor === undefined ? "—" : formatWorkedDuration(workedFor)],
  ] as const;

  return (
    <div aria-label="Last request statistics" className="chat-stats-popover" id={id} role="dialog">
      <div className="chat-stats-title">Last request statistics</div>
      <dl>
        {rows.map(([label, value]) => (
          <div className={`chat-stats-row${label === "total" ? " is-total" : ""}`} key={label}>
            <dt>{label}</dt>
            <dd>{value}</dd>
          </div>
        ))}
      </dl>
      <div className="chat-stats-note">All token, cache and speed counters refer to the last request. Cache counters are included in context. Turn duration covers the whole turn.</div>
    </div>
  );
}

export function ReasoningBlock({ isCollapsed, isLive = false, message, onToggle, suppressBriefThought = false, suppressThoughtFor = false }: { isCollapsed: boolean; isLive?: boolean; message: ChatMessage; onToggle: () => void; suppressBriefThought?: boolean; suppressThoughtFor?: boolean }) {
  const reasoning = message.reasoning?.trim();
  const thoughtFor = message.thoughtFor ?? message.stats?.ttftSeconds;
  const hasOutput = Boolean(reasoning || message.content.trim() || message.toolCalls?.length);
  const awaitingThought = isLive && thoughtFor === undefined;
  const liveThoughtFor = useLiveThoughtDuration(awaitingThought && hasOutput, message.workStartedAt ?? message.createdAt);
  if (!hasOutput) return null;
  if (!reasoning) {
    if (awaitingThought) return <ReasoningSummaryBlock isLive seconds={liveThoughtFor} />;
    if (suppressBriefThought) return null;
    return <BriefThought />;
  }

  const displayedThoughtFor = suppressThoughtFor ? undefined : thoughtFor ?? (awaitingThought ? liveThoughtFor : undefined);

  return (
    <div
      aria-expanded={!isCollapsed}
      aria-label="Model reasoning"
      className={`chat-reasoning${isCollapsed ? " is-collapsed" : ""}${isCollapsed && isLive ? " is-live" : ""}`}
      onClick={(event) => {
        event.stopPropagation();
        if (window.getSelection()?.toString()) return;
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
      {(displayedThoughtFor !== undefined && (displayedThoughtFor > 0 || awaitingThought)) || isCollapsed ? (
        <div className="chat-thought-for"><span className="chat-reasoning-wave">{displayedThoughtFor !== undefined && (displayedThoughtFor > 0 || awaitingThought) ? awaitingThought ? `thought for ${formatWorkedDuration(displayedThoughtFor)}` : formatThoughtDuration(displayedThoughtFor) : isLive ? "thinking" : "thought briefly"}</span></div>
      ) : null}
    </div>
  );
}

function useLiveThoughtDuration(isThinking: boolean, startedAt?: number) {
  const [fallbackStartedAt] = useState(Date.now);
  const start = startedAt ?? fallbackStartedAt;
  const [now, setNow] = useState(Date.now);
  useEffect(() => {
    if (!isThinking) return;
    setNow(Date.now());
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [isThinking, start]);
  return Math.max(0, (now - start) / 1000);
}

export function LiveThoughtSummary({ startedAt }: { startedAt?: number }) {
  const seconds = useLiveThoughtDuration(true, startedAt);
  return <ReasoningSummaryBlock isLive seconds={seconds} />;
}

export function BriefThought() {
  return <div className="chat-reasoning chat-reasoning-brief is-collapsed"><div className="chat-thought-for">thought briefly</div></div>;
}

export function ReasoningSummaryBlock({ isLive = false, seconds }: { isLive?: boolean; seconds: number }) {
  if (!Number.isFinite(seconds) || seconds < 0 || (!isLive && seconds === 0)) return null;

  return (
    <div aria-label="Model reasoning" className={`chat-reasoning chat-reasoning-summary${isLive ? " is-live" : ""}`}>
      <div className="chat-thought-for"><span className="chat-reasoning-wave">{isLive ? `thought for ${formatWorkedDuration(seconds)}` : formatThoughtDuration(seconds)}</span></div>
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
