import { useEffect, useState, type CSSProperties } from "react";
import type { ChatMessage, ChatToolCall } from "../chatTypes";
import { ChatMessageGroups } from "../ChatMessageGroups";
import { subagentDisplayStatus } from "../ChatToolActivity";
import { subagentLabel } from "./hierarchy";
import { indexChatMessages, liveWorkedForSeconds } from "../chatMessageUtils";
import { CollapseSubchatIcon } from "../ChatIcons";
import { ScrollToBottom } from "../scroll/Bottom";
import { useChatScroll } from "../useChatScroll";

export function SubagentChatPanel({ isLoading = false, messages, onBack, onCollapse, onOpenSubagent, tool }: { isLoading?: boolean; messages?: ChatMessage[]; onBack?: () => void; onCollapse: () => void; onOpenSubagent?: (messageID: string, toolID: string) => void; tool: ChatToolCall }) {
  const status = subagentDisplayStatus(tool);
  const active = status === "running" || status === "queued";
  const [now, setNow] = useState(Date.now);

  useEffect(() => {
    if (!active) return;
    setNow(Date.now());
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, [active]);
  const failure = status !== "success" && tool.result?.status === "error" ? tool.result?.error?.trim() || tool.result?.output?.trim() || tool.result?.summary?.trim() : undefined;
  const normalizeTask = (content: string) => content.replace(/\r\n?/g, "\n").trim();
  let previousUserContent: string | undefined;
  const transcript = (messages ?? []).filter((message) => {
    if (message.role !== "user") {
      previousUserContent = undefined;
      return true;
    }
    const content = normalizeTask(message.content);
    const duplicate = Boolean(content) && content === previousUserContent && !message.images?.length;
    previousUserContent = content;
    return !duplicate;
  });
  const liveWorkedFor = active ? liveWorkedForSeconds({
    id: tool.id,
    title: "",
    messages: transcript.map(message => ({ ...message, workStartedAt: message.workStartedAt ?? message.createdAt })),
  }, now) : undefined;
  const latestIntent = [...transcript].reverse()
    .flatMap((message) => [...(message.toolCalls ?? [])].reverse())
    .find((call) => call.intent?.trim())?.intent?.trim() || tool.intent?.trim() || "Intent missing";
  const lastMessage = transcript.at(-1);
  const scroll = useChatScroll({
    chatID: `subchat:${tool.result?.subchatId ?? tool.id}`,
    contentKey: [isLoading, transcript.length, lastMessage?.content, lastMessage?.reasoning, lastMessage?.toolCalls?.length].join("\u0000"),
  });

  useEffect(() => {
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") (onBack ?? onCollapse)();
    };
    document.addEventListener("keydown", closeOnEscape);
    return () => document.removeEventListener("keydown", closeOnEscape);
  }, [onBack, onCollapse]);

  return (
    <section
      aria-label="Subagent chat"
      aria-modal="true"
      className={`chat-subchat-panel is-${status}`}
      onClick={(event) => event.stopPropagation()}
      style={{ "--subagent-base-depth": (tool.subagentDepth ?? 1) + 1 } as CSSProperties}
      role="dialog"
    >
      <header className="chat-subchat-header">
        <div className="chat-subchat-heading">
          <i aria-hidden="true" className="chat-subchat-status-dot" />
          <span className="chat-subchat-title">{subagentLabel(tool)} chat</span>
        </div>
        {onBack ? <button className="chat-subchat-back" onClick={onBack} type="button">Back to parent subchat</button> : null}
        <button aria-label="Collapse subagent chat" className="chat-subchat-collapse" onClick={onCollapse} title="Collapse subagent chat" type="button">
          <CollapseSubchatIcon />
        </button>
      </header>
      <div className="chat-subchat-body" ref={scroll.messagesShellRef} onScroll={scroll.onMessagesScroll} onWheel={scroll.onMessagesWheel} onPointerDown={scroll.onMessagesPointerDown} onKeyDown={scroll.onMessagesKeyDown} tabIndex={0}>
        <div className="chat-subchat-intent">
          <p>{latestIntent}</p>
        </div>
        {failure ? <pre className="chat-subchat-error" role="alert">{failure}</pre> : null}
        <div className="chat-subchat-transcript">
          {isLoading ? <p className="chat-empty">Loading subchat…</p> : transcript.length ? <ChatMessageGroups onOpenSubagent={onOpenSubagent} isWorking={active} liveWorkedFor={liveWorkedFor} messages={indexChatMessages(transcript)} /> : <p className="chat-empty">No subchat messages yet.</p>}
        </div>
      </div>
      <ScrollToBottom visible={scroll.isAwayFromBottom} onClick={scroll.scrollToBottom} />
    </section>
  );
}
