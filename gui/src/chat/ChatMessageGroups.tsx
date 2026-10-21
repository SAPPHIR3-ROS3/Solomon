import { memo, useEffect, useRef, useState, type MouseEvent } from "react";
import type { ChatMessage, ChatToolCall } from "./chatTypes";
import type { CheckpointMetadata, IndexedChatMessage } from "./chatViewTypes";
import { ChatImageAttachments, CheckpointLabel, CompactionCard, InterruptedGenerationMarker } from "./ChatMessageParts";
import { SubagentCard, ToolCallCard, subagentDisplayStatus } from "./ChatToolActivity";
import { assistantFooterMessage, groupChatTurns, indexChatMessages, toolCheckpoint } from "./chatMessageUtils";
import { BriefThought, MessageFooter, ReasoningBlock, ReasoningSummaryBlock, WorkedForCounter } from "./ChatMessageFooter";
import { CollapseSubchatIcon } from "./ChatIcons";
import { MarkdownContent } from "./MarkdownContent";
import { useStartToolCallsCollapsed, useGUIChatPreferencesState } from "../settings/chatPreferences";

type ChatMessageGroupHandlers = {
  onOpenSubagent?: (messageID: string, toolID: string) => void;
  onRequestDelete?: (message: ChatMessage) => void;
  onStopTool?: (messageID: string, toolID: string) => void;
};

export function ChatMessageGroups({ liveWorkedFor, isWorking = liveWorkedFor !== undefined, messages, onOpenSubagent, onRequestDelete, onStopTool }: { isWorking?: boolean; liveWorkedFor?: number; messages: IndexedChatMessage[] } & ChatMessageGroupHandlers) {
  const preferences = useGUIChatPreferencesState();
  const preferencesReady = preferences.ready;
  const autoCollapseToolCalls = preferences.chat.autoCloseToolCalls;
  const startToolCallsCollapsed = preferences.chat.startToolCallsCollapsed;
  const groups = groupChatTurns(messages);


  return (
    <>
      {groups.map((entries, groupIndex) => {
        const first = entries[0];
        if (first.message.kind === "compaction") return <CompactionCard key={first.message.id} message={first.message} />;

        if (first.message.role === "assistant") {
          const activeWorkedFor = isWorking && groupIndex === groups.length - 1 ? liveWorkedFor : undefined;
          return (
            <AssistantTurn
              preferencesReady={preferencesReady}
              autoCollapseToolCalls={autoCollapseToolCalls}
              startToolCallsCollapsed={startToolCallsCollapsed}
              activeWorkedFor={activeWorkedFor}
              isWorking={isWorking && groupIndex === groups.length - 1}
              entries={entries}
              key={first.message.id}
              onOpenSubagent={onOpenSubagent}
              onStopTool={onStopTool}
            />
          );
        }
        return <UserTurn entry={first} key={first.message.id} onOpenSubagent={onOpenSubagent} onRequestDelete={onRequestDelete} onStopTool={onStopTool} />;
      })}
    </>
  );
}

const UserTurn = memo(function UserTurn({ entry, ...handlers }: { entry: IndexedChatMessage } & ChatMessageGroupHandlers) {
  const actions = messageActions(entry, handlers);
  return <ChatMessageTurn checkpoint={entry.checkpoint} index={entry.index} message={entry.message} onOpenSubagent={actions.onOpenSubagent} onRequestDelete={actions.onRequestDelete} onStopTool={actions.onStopTool} />;
});

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

const AssistantTurn = memo(function AssistantTurn({
  preferencesReady,
  autoCollapseToolCalls,
  startToolCallsCollapsed,
  activeWorkedFor,
  entries,
  isWorking,
  onOpenSubagent,
  onStopTool,
}: {
  preferencesReady: boolean;
  autoCollapseToolCalls: boolean;
  startToolCallsCollapsed: boolean;
  activeWorkedFor?: number;
  entries: IndexedChatMessage[];
  isWorking: boolean;
  onOpenSubagent?: ChatMessageGroupHandlers["onOpenSubagent"];
  onStopTool?: ChatMessageGroupHandlers["onStopTool"];
}) {
  const footerMessage = assistantFooterMessage(entries);
  const hasAnyToolCalls = entries.some(({ message }) => Boolean(message.toolCalls?.length));
  const lastToolEntryIndex = entries.reduce((lastIndex, entry, index) => (
    entry.message.toolCalls?.length ? index : lastIndex
  ), -1);
  const responseIndex = hasAnyToolCalls ? findFinalResponseIndex(entries, lastToolEntryIndex) : -1;
  const responseEntry = responseIndex >= 0 ? entries[responseIndex] : undefined;
  const rawActivityEntries = responseEntry ? entries.filter((_, index) => index !== responseIndex) : entries;
  const toolCalls = rawActivityEntries.flatMap(({ message }) => message.toolCalls ?? []);
  const hasToolCalls = toolCalls.length > 0;
  const latestMessage = entries.at(-1)?.message;
  const activeGenerationMessageID = isWorking && (latestMessage?.toolCalls?.length ?? 0) === 0 ? latestMessage?.id : undefined;
  const activityEntries = withActivityCheckpoints(rawActivityEntries, toolCalls);
  const [collapsedOverride, setCollapsedOverride] = useState<boolean | undefined>(undefined);
  const wasWorking = useRef(isWorking);
  useEffect(() => {
    if (!preferencesReady) return;
    if (autoCollapseToolCalls && !isWorking) setCollapsedOverride(true);
    else if (isWorking && !wasWorking.current) setCollapsedOverride(undefined);
    wasWorking.current = isWorking;
  }, [autoCollapseToolCalls, isWorking, preferencesReady]);
  const isToolActivityCollapsed = collapsedOverride ?? (startToolCallsCollapsed || (preferencesReady && autoCollapseToolCalls && !isWorking));
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
        {(!hasToolCalls || !isToolActivityCollapsed) ? activityEntries.map((entry, entryIndex) => {
          const actions = messageActions(entry, { onOpenSubagent, onStopTool });
          const toolActivity = hasToolCalls ? {
            isCollapsed: isToolActivityCollapsed,
            onToggleCollapsed: toggleToolActivity,
          } : undefined;
          return (
            <AssistantMessageBlock
              checkpoint={entry.checkpoint}
              isWorking={entry.message.id === activeGenerationMessageID}
              key={entry.message.id}
              message={entry.message}
              onOpenSubagent={actions.onOpenSubagent}
              onStopTool={actions.onStopTool}
              suppressBriefThought={isRepeatedBriefThought(activityEntries, entryIndex)}
              toolActivity={toolActivity}
            />
          );
        }) : null}
        {hasToolCalls ? (
          <ToolActivityCollapseControl
            isCollapsed={isToolActivityCollapsed}
            isWorking={activeGenerationMessageID !== undefined}
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
          isWorking={responseEntry.message.id === activeGenerationMessageID}
          message={responseEntry.message}
          onOpenSubagent={responseActions?.onOpenSubagent}
          onStopTool={responseActions?.onStopTool}
        />
      ) : null}
      <div className="chat-assistant-footer">
        {(activeWorkedFor !== undefined || footerMessage.workedFor !== undefined) ? (
          <WorkedForCounter isLive={activeWorkedFor !== undefined} seconds={activeWorkedFor ?? footerMessage.workedFor!} />
        ) : null}
        {activeWorkedFor !== undefined ? null : (
          <MessageFooter index={(responseEntry ?? activityEntries[activityEntries.length - 1]).index} message={footerMessage} />
        )}
      </div>
    </div>
  );
}, (previous, next) => previous.preferencesReady === next.preferencesReady
  && previous.autoCollapseToolCalls === next.autoCollapseToolCalls
  && previous.startToolCallsCollapsed === next.startToolCallsCollapsed
  && previous.activeWorkedFor === next.activeWorkedFor
  && previous.isWorking === next.isWorking
  && previous.onOpenSubagent === next.onOpenSubagent
  && previous.onStopTool === next.onStopTool
  && previous.entries.length === next.entries.length
  && previous.entries.every((entry, index) => entry === next.entries[index]));

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

function isEmptyBriefThought(message: ChatMessage) {
  if (message.reasoning?.trim()) return false;
  const thoughtFor = message.thoughtFor ?? message.stats?.ttftSeconds;
  return Boolean(message.content.trim() || message.toolCalls?.length || (thoughtFor !== undefined && thoughtFor > 0));
}

function isRepeatedBriefThought(entries: IndexedChatMessage[], index: number) {
  const message = entries[index]?.message;
  if (!message || !isEmptyBriefThought(message)) return false;
  const previous = entries[index - 1]?.message;
  return previous !== undefined && isEmptyBriefThought(previous);
}

function AssistantMessageBlock({ checkpoint, isWorking = false, message, onOpenSubagent, onStopTool, suppressBriefThought = false, toolActivity }: { checkpoint?: CheckpointMetadata; isWorking?: boolean; message: ChatMessage; onOpenSubagent?: (tool: ChatToolCall) => void; onStopTool?: (toolID: string) => void; suppressBriefThought?: boolean; toolActivity?: ToolActivityControl }) {
  return (
    <div className={message.toolCalls?.length ? "chat-assistant-segment has-tool-activity" : "chat-assistant-segment"}>
      <ChatMessageBody checkpoint={checkpoint} isWorking={isWorking} message={message} onOpenSubagent={onOpenSubagent} onStopTool={onStopTool} suppressBriefThought={suppressBriefThought} toolActivity={toolActivity} />
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

function ChatMessageBody({ checkpoint, isWorking = false, message, onOpenSubagent, onStopTool, suppressBriefThought = false, toolActivity }: { checkpoint?: CheckpointMetadata; isWorking?: boolean; message: ChatMessage; onOpenSubagent?: (tool: ChatToolCall) => void; onStopTool?: (toolID: string) => void; suppressBriefThought?: boolean; toolActivity?: ToolActivityControl }) {
  const preferences = useGUIChatPreferencesState();
  const [reasoningCollapsedOverride, setReasoningCollapsedOverride] = useState<boolean | undefined>(undefined);
  const isReasoningCollapsed = reasoningCollapsedOverride ?? preferences.chat.startReasoningCollapsed;
  const toggleReasoning = () => setReasoningCollapsedOverride(!isReasoningCollapsed);
  const canCollapseReasoning = message.role === "assistant" && Boolean(message.reasoning?.trim());

  function handleMessageClick(event: MouseEvent<HTMLElement>) {
    if (!canCollapseReasoning) return;
    if (window.getSelection()?.toString()) return;
    if (event.target instanceof HTMLElement && event.target.closest("a, button, input, textarea, select, summary")) return;
    toggleReasoning();
  }

  return (
    <article className={"chat-message is-" + message.role} onClick={canCollapseReasoning ? handleMessageClick : undefined}>
      {checkpoint && !(message.role === "assistant" && message.toolCalls?.length) ? <CheckpointLabel label={checkpoint.label} /> : null}
      {message.images?.length ? <ChatImageAttachments images={message.images} /> : null}
      {message.role === "assistant" && !toolActivity?.isCollapsed ? <ReasoningBlock isLive={isWorking} isCollapsed={isReasoningCollapsed} message={message} onToggle={toggleReasoning} suppressBriefThought={suppressBriefThought} /> : null}
      {message.toolCalls?.length ? (
        <ToolActivity
          isCollapsed={toolActivity?.isCollapsed}
          onOpenSubagent={onOpenSubagent}
          onStopTool={onStopTool}
          onToggleCollapsed={toolActivity?.onToggleCollapsed}
          toolCalls={message.toolCalls}
        />
      ) : null}
      <div className="chat-response-content" data-message-id={message.id}><MarkdownContent content={message.content} /></div>
    </article>
  );
}

function ToolActivity({ isCollapsed: controlledIsCollapsed, onOpenSubagent, onStopTool, onToggleCollapsed, toolCalls }: { isCollapsed?: boolean; onOpenSubagent?: (tool: ChatToolCall) => void; onStopTool?: (toolID: string) => void; onToggleCollapsed?: () => void; toolCalls: ChatToolCall[] }) {
  const startToolCallsCollapsed = useStartToolCallsCollapsed();
  const [localIsCollapsed, setLocalIsCollapsed] = useState<boolean | undefined>(undefined);
  const isControlled = onToggleCollapsed !== undefined;
  const isCollapsed = controlledIsCollapsed ?? localIsCollapsed ?? startToolCallsCollapsed;
  const collapseLabel = toolActivityCollapseLabel(isCollapsed, toolCalls.length);

  function toggleCollapsed() {
    if (onToggleCollapsed) {
      onToggleCollapsed();
      return;
    }
    setLocalIsCollapsed(!isCollapsed);
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
    .filter(({ message }) => Boolean(message.reasoning?.trim()))
    .map(({ message }) => message.thoughtFor ?? message.stats?.ttftSeconds)
    .filter((value): value is number => value !== undefined && Number.isFinite(value) && value > 0);
  if (durations.length === 0) return undefined;
  return durations.reduce((total, value) => total + value, 0);
}

function ToolActivityCollapseControl({ entries, isCollapsed, isWorking, onToggleCollapsed, thoughtFor, toolCalls }: { entries: IndexedChatMessage[]; isCollapsed: boolean; isWorking: boolean; onToggleCollapsed: () => void; thoughtFor?: number; toolCalls: ChatToolCall[] }) {
  const collapseLabel = toolActivityCollapseLabel(isCollapsed, toolCalls.length);
  const hasReasoning = entries.some(({ message }) => Boolean(message.reasoning?.trim()));

  return (
    <section aria-label="Tool activity" className={"chat-tool-activity chat-tool-activity-controls" + (isCollapsed ? " is-collapsed" : "")}>
      {isCollapsed ? <CollapsedActivityCheckpoints entries={entries} toolCalls={toolCalls} /> : null}
      {isCollapsed && (thoughtFor !== undefined || !hasReasoning) ? (
        <div className="chat-tool-collapsed-summary">
          {thoughtFor !== undefined ? <ReasoningSummaryBlock isLive={isWorking} seconds={thoughtFor} /> : <BriefThought />}
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
  const normalizeTask = (content: string) => content.replace(/\r\n?/g, "\n").trim();
  const task = normalizeTask(tool.input ?? "");
  const taskInTranscript = !isLoading && Boolean(task) && transcript.some((message) => (
    message.role === "user" && normalizeTask(message.content) === task
  ));

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
        {!taskInTranscript ? <div className="chat-subchat-task">
          <span className="chat-subchat-task-label">Task</span>
          <p>{tool.input ?? "No task provided."}</p>
        </div> : null}
        <div className="chat-subchat-transcript">
          {isLoading ? <p className="chat-empty">Loading subchat…</p> : transcript.length ? <ChatMessageGroups isWorking={status === "running" || status === "queued"} messages={indexChatMessages(transcript)} /> : <p className="chat-empty">No subchat messages yet.</p>}
        </div>
      </div>
    </section>
  );
}
