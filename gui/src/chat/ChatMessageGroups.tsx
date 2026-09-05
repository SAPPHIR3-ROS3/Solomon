import { useEffect, useState, type MouseEvent } from "react";
import type { ChatMessage, ChatToolCall } from "./chatTypes";
import type { CheckpointMetadata, IndexedChatMessage } from "./chatViewTypes";
import { ChatImageAttachments, CheckpointLabel, CompactionCard, InterruptedGenerationMarker } from "./ChatMessageParts";
import { SubagentCard, ToolCallCard, subagentDisplayStatus } from "./ChatToolActivity";
import { assistantFooterMessage, groupChatTurns, indexChatMessages, toolCheckpoint } from "./chatMessageUtils";
import { MessageFooter, ReasoningBlock, WorkedForCounter } from "./ChatMessageFooter";
import { CollapseSubchatIcon } from "./ChatIcons";
import { MarkdownContent } from "./MarkdownContent";

type ChatMessageGroupHandlers = {
  onOpenSubagent?: (messageID: string, toolID: string) => void;
  onRequestDelete?: (message: ChatMessage) => void;
  onStopTool?: (messageID: string, toolID: string) => void;
};

export function ChatMessageGroups({ liveWorkedFor, messages, onOpenSubagent, onRequestDelete, onStopTool }: { liveWorkedFor?: number; messages: IndexedChatMessage[] } & ChatMessageGroupHandlers) {
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
              {shouldShowWorkedFor && activeWorkedFor !== undefined ? null : (
                <MessageFooter index={last.index} message={footerMessage} />
              )}
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
  const collapseLabel = isCollapsed ? `Show ${toolCalls.length} tool calls` : "Collapse tool calls";

  function toggleCollapsed() {
    setIsCollapsed((current) => !current);
  }

  return (
    <section aria-label="Tool activity" className={`chat-tool-activity${isCollapsed ? " is-collapsed" : ""}`} onClick={(event) => event.stopPropagation()}>
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


export function SubagentChatPanel({ isLoading = false, messages, onCollapse, tool }: { isLoading?: boolean; messages?: ChatMessage[]; onCollapse: () => void; tool: ChatToolCall }) {
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
