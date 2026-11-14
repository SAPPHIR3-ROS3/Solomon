import { useEffect, type CSSProperties } from "react";
import type { ChatMessage, ChatToolCall } from "../chatTypes";
import { ChatMessageGroups } from "../ChatMessageGroups";
import { subagentDisplayStatus } from "../ChatToolActivity";
import { subagentLabel } from "./hierarchy";
import { indexChatMessages } from "../chatMessageUtils";
import { CollapseSubchatIcon } from "../ChatIcons";

export function SubagentChatPanel({ isLoading = false, messages, onBack, onCollapse, onOpenSubagent, tool }: { isLoading?: boolean; messages?: ChatMessage[]; onBack?: () => void; onCollapse: () => void; onOpenSubagent?: (messageID: string, toolID: string) => void; tool: ChatToolCall }) {
  const status = subagentDisplayStatus(tool);
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
  const latestIntent = [...transcript].reverse()
    .flatMap((message) => [...(message.toolCalls ?? [])].reverse())
    .find((call) => call.intent?.trim())?.intent?.trim() || tool.intent?.trim() || "Intent missing";

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
      <div className="chat-subchat-body">
        <div className="chat-subchat-intent">
          <p>{latestIntent}</p>
        </div>
        <div className="chat-subchat-transcript">
          {isLoading ? <p className="chat-empty">Loading subchat…</p> : transcript.length ? <ChatMessageGroups onOpenSubagent={onOpenSubagent} isWorking={status === "running" || status === "queued"} messages={indexChatMessages(transcript)} /> : <p className="chat-empty">No subchat messages yet.</p>}
        </div>
      </div>
    </section>
  );
}
