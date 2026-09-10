import { useEffect, useState, type MouseEvent } from "react";
import type { ChatMessage, ChatToolCall } from "./chatTypes";
import type { CheckpointMetadata, IndexedChatMessage } from "./chatViewTypes";
import { ChatImageAttachments, CheckpointLabel, CompactionCard, InterruptedGenerationMarker } from "./ChatMessageParts";
import { SubagentCard, ToolCallCard, subagentDisplayStatus } from "./ChatToolActivity";
import { assistantFooterMessage, groupChatTurns, indexChatMessages, toolCheckpoint } from "./chatMessageUtils";
import { MessageFooter, ReasoningBlock, ReasoningSummaryBlock, WorkedForCounter } from "./ChatMessageFooter";
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
          const footerMessage = assistantFooterMessage(entries);
          const activeWorkedFor = liveWorkedFor;
          const shouldShowWorkedFor = groupIndex === lastAssistantGroupIndex;
          return (
            <AssistantTurn
              activeWorkedFor={activeWorkedFor}
              entries={entries}
              footerMessage={footerMessage}
              key={first.message.id}
              onOpenSubagent={onOpenSubagent}
              onStopTool={onStopTool}
              shouldShowWorkedFor={shouldShowWorkedFor}
            />
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

type ToolActivityControl = {
  isCollapsed: boolean;
  onToggleCollapsed: () => void;
};

function AssistantTurn({
  activeWorkedFor,
  entries,
  footerMessage,
  onOpenSubagent,
  onStopTool,
  shouldShowWorkedFor,
}: {
  activeWorkedFor?: number;
  entries: IndexedChatMessage[];
  footerMessage: ChatMessage;
  onOpenSubagent?: ChatMessageGroupHandlers["onOpenSubagent"];
  onStopTool?: ChatMessageGroupHandlers["onStopTool"];
  shouldShowWorkedFor: boolean;
}) {
  const hasAnyToolCalls = entries.some(({ message }) => Boolean(message.toolCalls?.length));
  const lastToolEntryIndex = entries.reduce((lastIndex, entry, index) => (
    entry.message.toolCalls?.length ? index : lastIndex
  ), -1);
  const responseIndex = hasAnyToolCalls ? findFinalResponseIndex(entries, lastToolEntryIndex) : -1;
  const responseEntry = responseIndex >= 0 ? entries[responseIndex] : undefined;
  const rawActivityEntries = responseEntry ? entries.filter((_, index) => index !== responseIndex) : entries;
  const toolCalls = rawActivityEntries.flatMap(({ message }) => message.toolCalls ?? []);
  const hasToolCalls = toolCalls.length > 0;
  const activityEntries = withActivityCheckpoints(rawActivityEntries, toolCalls);
  const [collapsedOverride, setCollapsedOverride] = useState<boolean | undefined>(undefined);
  const isToolActivityCollapsed = collapsedOverride ?? !toolCalls.some((tool) => tool.name === "orchestrate");
  const timelineClassName = [
    "chat-assistant-timeline",
    hasToolCalls ? "has-tool-activity" : "",
    hasToolCalls && isToolActivityCollapsed ? "is-tool-activity-collapsed" : "",
  ].filter(Boolean).join(" ");
  const responseActions = responseEntry ? messageActions(responseEntry, { onOpenSubagent, onStopTool }) : undefined;

  function toggleToolActivity() {
    setCollapsedOverride(!isToolActivityCollapsed);
  }

  return (
    <div className="chat-turn is-assistant">
      <div className={timelineClassName}>
        {(!hasToolCalls || !isToolActivityCollapsed) ? activityEntries.map((entry) => {
          const actions = messageActions(entry, { onOpenSubagent, onStopTool });
          const toolActivity = hasToolCalls ? {
            isCollapsed: isToolActivityCollapsed,
            onToggleCollapsed: toggleToolActivity,
          } : undefined;
          return (
            <AssistantMessageBlock
              checkpoint={entry.checkpoint}
              key={entry.message.id}
              message={entry.message}
              onOpenSubagent={actions.onOpenSubagent}
              onStopTool={actions.onStopTool}
              toolActivity={toolActivity}
            />
          );
        }) : null}
        {hasToolCalls ? (
          <ToolActivityCollapseControl
            isCollapsed={isToolActivityCollapsed}
            onToggleCollapsed={toggleToolActivity}
            entries={activityEntries}
            thoughtFor={isToolActivityCollapsed ? aggregateThoughtFor(activityEntries) : undefined}
            toolCalls={toolCalls}
          />
        ) : null}
      </div>
      {responseEntry ? (
        <AssistantMessageBlock
          checkpoint={responseEntry.checkpoint}
          message={responseEntry.message}
          onOpenSubagent={responseActions?.onOpenSubagent}
          onStopTool={responseActions?.onStopTool}
        />
      ) : null}
      {shouldShowWorkedFor && activeWorkedFor !== undefined ? null : (
        <MessageFooter index={(responseEntry ?? activityEntries[activityEntries.length - 1]).index} message={footerMessage} />
      )}
      {shouldShowWorkedFor && (activeWorkedFor !== undefined || footerMessage.workedFor !== undefined) ? (
        <WorkedForCounter isLive={activeWorkedFor !== undefined} seconds={activeWorkedFor ?? footerMessage.workedFor!} />
      ) : null}
    </div>
  );
}

function findFinalResponseIndex(entries: IndexedChatMessage[], lastToolEntryIndex: number) {
  for (let index = entries.length - 1; index > lastToolEntryIndex; index -= 1) {
    const message = entries[index].message;
    if (!message.toolCalls?.length && (message.content.trim() || message.images?.length)) return index;
  }
  return -1;
}

function withActivityCheckpoints(entries: IndexedChatMessage[], toolCalls: ChatToolCall[]): IndexedChatMessage[] {
  const toolCheckpointKeys = new Set(
    toolCalls
      .map(toolCheckpoint)
      .filter((checkpoint): checkpoint is CheckpointMetadata => checkpoint !== undefined)
      .map(checkpointKey),
  );
  const usedCheckpointKeys = new Set<string>();
  const allSequences = [
    ...entries.flatMap(({ checkpoint }) => checkpoint ? [checkpoint.sequence] : []),
    ...toolCalls.map(toolCheckpoint).filter((checkpoint): checkpoint is CheckpointMetadata => checkpoint !== undefined).map((checkpoint) => checkpoint.sequence),
  ];
  let nextSequence = allSequences.length ? Math.max(...allSequences) : -1;
  let fallbackBranch = entries.find(({ checkpoint }) => checkpoint)?.checkpoint?.branch ?? "";

  return entries.map((entry) => {
    const checkpoint = entry.checkpoint;
    if (checkpoint?.branch !== undefined) fallbackBranch = checkpoint.branch;
    const key = checkpoint ? checkpointKey(checkpoint) : "";
    const needsGeneratedCheckpoint = !checkpoint || (
      !entry.message.toolCalls?.length && (toolCheckpointKeys.has(key) || usedCheckpointKeys.has(key))
    );
    if (!needsGeneratedCheckpoint) {
      usedCheckpointKeys.add(key);
      return entry;
    }

    nextSequence += 1;
    const generated = {
      branch: checkpoint?.branch ?? fallbackBranch,
      label: activityCheckpointLabel(nextSequence, checkpoint?.branch ?? fallbackBranch),
      sequence: nextSequence,
    };
    usedCheckpointKeys.add(checkpointKey(generated));
    return { ...entry, checkpoint: generated };
  });
}

function checkpointKey(checkpoint: CheckpointMetadata) {
  return checkpoint.sequence + ":" + checkpoint.branch;
}

function activityCheckpointLabel(sequence: number, branch: string) {
  return "[#" + String(sequence).padStart(3, "0") + branch + "]";
}

function AssistantMessageBlock({ checkpoint, message, onOpenSubagent, onStopTool, toolActivity }: { checkpoint?: CheckpointMetadata; message: ChatMessage; onOpenSubagent?: (tool: ChatToolCall) => void; onStopTool?: (toolID: string) => void; toolActivity?: ToolActivityControl }) {
  return (
    <div className={message.toolCalls?.length ? "chat-assistant-segment has-tool-activity" : "chat-assistant-segment"}>
      <ChatMessageBody checkpoint={checkpoint} message={message} onOpenSubagent={onOpenSubagent} onStopTool={onStopTool} toolActivity={toolActivity} />
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

function ChatMessageBody({ checkpoint, message, onOpenSubagent, onStopTool, toolActivity }: { checkpoint?: CheckpointMetadata; message: ChatMessage; onOpenSubagent?: (tool: ChatToolCall) => void; onStopTool?: (toolID: string) => void; toolActivity?: ToolActivityControl }) {
  const [isReasoningCollapsed, setIsReasoningCollapsed] = useState(false);
  const thoughtFor = message.thoughtFor ?? message.stats?.ttftSeconds;
  const canCollapseReasoning = message.role === "assistant" && Boolean(message.reasoning || (thoughtFor !== undefined && thoughtFor > 0));

  function handleMessageClick(event: MouseEvent<HTMLElement>) {
    if (!canCollapseReasoning) return;
    if (event.target instanceof HTMLElement && event.target.closest("a, button, input, textarea, select, summary")) return;
    setIsReasoningCollapsed((current) => !current);
  }

  return (
    <article className={"chat-message is-" + message.role} onClick={canCollapseReasoning ? handleMessageClick : undefined}>
      {checkpoint && !(message.role === "assistant" && message.toolCalls?.length) ? <CheckpointLabel label={checkpoint.label} /> : null}
      {message.images?.length ? <ChatImageAttachments images={message.images} /> : null}
      {canCollapseReasoning && !toolActivity?.isCollapsed ? <ReasoningBlock isCollapsed={isReasoningCollapsed} message={message} onToggle={() => setIsReasoningCollapsed((current) => !current)} /> : null}
      {message.toolCalls?.length ? (
        <ToolActivity
          isCollapsed={toolActivity?.isCollapsed}
          onOpenSubagent={onOpenSubagent}
          onStopTool={onStopTool}
          onToggleCollapsed={toolActivity?.onToggleCollapsed}
          toolCalls={message.toolCalls}
        />
      ) : null}
      <MarkdownContent content={message.content} />
    </article>
  );
}

function ToolActivity({ isCollapsed: controlledIsCollapsed, onOpenSubagent, onStopTool, onToggleCollapsed, toolCalls }: { isCollapsed?: boolean; onOpenSubagent?: (tool: ChatToolCall) => void; onStopTool?: (toolID: string) => void; onToggleCollapsed?: () => void; toolCalls: ChatToolCall[] }) {
  const [localIsCollapsed, setLocalIsCollapsed] = useState(() => !toolCalls.some((tool) => tool.name === "orchestrate"));
  const isControlled = onToggleCollapsed !== undefined;
  const isCollapsed = controlledIsCollapsed ?? localIsCollapsed;
  const collapseLabel = toolActivityCollapseLabel(isCollapsed, toolCalls.length);

  function toggleCollapsed() {
    if (onToggleCollapsed) {
      onToggleCollapsed();
      return;
    }
    setLocalIsCollapsed((current) => !current);
  }

  if (isControlled && isCollapsed) return null;

  return (
    <section aria-label="Tool activity" className={"chat-tool-activity" + (isCollapsed ? " is-collapsed" : "")} onClick={(event) => event.stopPropagation()}>
      {isCollapsed ? <CollapsedToolCheckpoints toolCalls={toolCalls} /> : null}
      {!isCollapsed ? toolCalls.map((tool) => (
        tool.name === "subagent"
          ? <SubagentCard key={tool.id} onOpenSubagent={onOpenSubagent} onStopTool={onStopTool} tool={tool} />
          : <ToolCallCard key={tool.id} tool={tool} />
      )) : null}
      {!isControlled ? (
        <button aria-expanded={!isCollapsed} aria-label={collapseLabel} className="chat-tool-collapse-all" onClick={toggleCollapsed} type="button">
          {collapseLabel}
        </button>
      ) : null}
    </section>
  );
}

function aggregateThoughtFor(entries: IndexedChatMessage[]): number | undefined {
  const durations = entries
    .map(({ message }) => message.thoughtFor ?? message.stats?.ttftSeconds)
    .filter((value): value is number => value !== undefined && Number.isFinite(value) && value > 0);
  if (durations.length === 0) return undefined;
  return durations.reduce((total, value) => total + value, 0);
}

function ToolActivityCollapseControl({ entries, isCollapsed, onToggleCollapsed, thoughtFor, toolCalls }: { entries: IndexedChatMessage[]; isCollapsed: boolean; onToggleCollapsed: () => void; thoughtFor?: number; toolCalls: ChatToolCall[] }) {
  const collapseLabel = toolActivityCollapseLabel(isCollapsed, toolCalls.length);

  return (
    <section aria-label="Tool activity" className={"chat-tool-activity chat-tool-activity-controls" + (isCollapsed ? " is-collapsed" : "")}>
      {isCollapsed ? <CollapsedActivityCheckpoints entries={entries} toolCalls={toolCalls} /> : null}
      {isCollapsed && thoughtFor !== undefined ? (
        <div className="chat-tool-collapsed-summary">
          <ReasoningSummaryBlock seconds={thoughtFor} />
        </div>
      ) : null}
      <button aria-expanded={!isCollapsed} aria-label={collapseLabel} className="chat-tool-collapse-all" onClick={onToggleCollapsed} type="button">
        {collapseLabel}
      </button>
    </section>
  );
}

function toolActivityCollapseLabel(isCollapsed: boolean, count: number) {
  return isCollapsed ? "Show " + count + " tool calls" : "Collapse tool calls";
}

function CollapsedActivityCheckpoints({ entries, toolCalls }: { entries: IndexedChatMessage[]; toolCalls: ChatToolCall[] }) {
  const checkpoints = [
    ...entries.flatMap(({ checkpoint }) => checkpoint ? [checkpoint] : []),
    ...toolCalls.map(toolCheckpoint).filter((checkpoint): checkpoint is CheckpointMetadata => checkpoint !== undefined),
  ];
  return <CollapsedCheckpointRail checkpoints={checkpoints} />;
}

function CollapsedToolCheckpoints({ toolCalls }: { toolCalls: ChatToolCall[] }) {
  return <CollapsedCheckpointRail checkpoints={toolCalls.map(toolCheckpoint).filter((checkpoint): checkpoint is CheckpointMetadata => checkpoint !== undefined)} />;
}

function CollapsedCheckpointRail({ checkpoints }: { checkpoints: CheckpointMetadata[] }) {
  const uniqueCheckpoints = checkpoints
    .filter((checkpoint, index, all) => all.findIndex((candidate) => candidate.sequence === checkpoint.sequence && candidate.branch === checkpoint.branch) === index)
    .sort((left, right) => left.sequence - right.sequence);
  if (uniqueCheckpoints.length === 0) return null;

  const first = uniqueCheckpoints[0];
  const last = uniqueCheckpoints[uniqueCheckpoints.length - 1];
  const hasHiddenCheckpoints = uniqueCheckpoints.length > 2;

  return (
    <div aria-label="Activity checkpoints" className={"chat-tool-checkpoints-collapsed" + (uniqueCheckpoints.length === 1 ? " is-single" : "")}>
      <CheckpointLabel label={first.label} />
      {hasHiddenCheckpoints ? <span aria-hidden="true" className="chat-tool-checkpoints-ellipsis">...</span> : null}
      {uniqueCheckpoints.length > 1 ? <CheckpointLabel label={last.label} /> : null}
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
