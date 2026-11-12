import type { Chat, ChatMessage, ChatToolCall } from "./chatTypes";
import type { LiveChat, ChatStreamEvent } from "./chatClient";
import { liveChatFromPayload } from "./chatClient";
import { rateLimitNoticeFromEvent } from "./chatMessageUtils";
import { arrayValue, stringValue, numberValue, messageFromPayload, toolCallFromPayload, normalizeToolResult, subchatStatusFromPayload, eventTimeMilliseconds } from "./payload";

export function applyLiveStreamEvent(chat: LiveChat, event: ChatStreamEvent, imageOrigin = "", replay = false): LiveChat {
  const type = stringValue(event.type);
  if (type === "regenerate_start") {
    let lastUser = chat.messages.length - 1;
    while (lastUser >= 0 && chat.messages[lastUser].role !== "user") lastUser--;
    return { ...chat, messages: chat.messages.slice(0, lastUser + 1), rateLimitNotice: undefined };
  }
  if (type === "chat_title") {
    const title = stringValue(event.title).trim();
    return title ? { ...chat, title } : chat;
  }
  if (type === "chat_snapshot") {
    const snapshot = event.chat;
    if (snapshot) return preserveLiveWorkState(chat, liveChatFromPayload(snapshot, chat.projectID, imageOrigin));
    return chat;
  }
  if (type === "chat_start") {
    const runStartedAt = chat.runStartedAt ?? eventTimeMilliseconds(event);
    if (!replay || !event.chat) {
      // Slash commands are not shown optimistically; the server sends the line when it is a skill request.
      // Only a local optimistic message stands for this request: an earlier user message without a reply is history.
      const user = messageFromPayload(event.user, chat.messages.length, imageOrigin);
      const last = chat.messages[chat.messages.length - 1];
      const shown = !user || isOptimisticUserMessage(last);
      return { ...chat, messages: shown ? chat.messages : [...chat.messages, user], runStartedAt, status: "running" };
    }
    const base = liveChatFromPayload(event.chat, chat.projectID, imageOrigin);
    const user = messageFromPayload(event.user, base.messages.length, imageOrigin);
    return {
      ...base,
      messages: user ? [...base.messages, user] : base.messages,
      runStartedAt,
      status: "running",
    };
  }

  if (type === "assistant_start") {
    const turn = numberValue(event.turn, chat.messages.length);
    const checkpointSeq = numberValue(event.checkpoint_seq, undefined);
    const workStartedAt = eventTimeMilliseconds(event);
    if (checkpointSeq !== undefined && chat.messages.some((message) => message.checkpointSeq === checkpointSeq && message.role === "assistant")) {
      return {
        ...chat,
        messages: chat.messages.map((message) => (
          message.role === "assistant" && message.checkpointSeq === checkpointSeq && message.workStartedAt === undefined
            ? { ...message, workStartedAt }
            : message
        )),
        runStartedAt: chat.runStartedAt ?? workStartedAt,
        status: "running",
      };
    }
    const assistant: ChatMessage = {
      checkpointSeq,
      content: "",
      createdAt: workStartedAt,
      id: `stream-assistant-${turn}-${Date.now()}`,
      reasoning: "",
      role: "assistant",
      toolCalls: [],
      workStartedAt,
    };
    return { ...chat, messages: [...chat.messages, assistant], runStartedAt: chat.runStartedAt ?? workStartedAt, status: "running" };
  }

  if (type === "assistant_delta") {
    const channel = stringValue(event.channel);
    const delta = stringValue(event.delta);
    return updateLastAssistant(chat, (message) => channel === "reasoning"
      ? { ...message, reasoning: `${message.reasoning ?? ""}${delta}` }
      : { ...message, content: `${message.content}${delta}` });
  }

  if (type === "assistant_end") {
    const toolCalls = arrayValue(event.tool_calls).map((tool, index) => toolCallFromPayload(tool, index));
    return updateLastAssistant(chat, (message) => ({
      ...message,
      content: stringValue(event.content) || message.content,
      reasoning: stringValue(event.reasoning) || message.reasoning,
      toolCalls: toolCalls.length ? mergeSubagentCalls(message.toolCalls, toolCalls) : message.toolCalls,
    }));
  }

  if (type === "tool_start") {
    if (chat.messages.at(-1)?.role !== "assistant") {
      chat = { ...chat, messages: [...chat.messages, { id: `stream-tool-${stringValue(event.id) || chat.messages.length}`, role: "assistant", content: "", toolCalls: [] }] };
    }
    const incoming = toolCallFromPayload({
      arguments: event.arguments,
      intent: event.intent,
      checkpointBranch: event.checkpoint_branch ?? event.checkpointBranch,
      checkpointSeq: event.checkpoint_seq ?? event.checkpointSeq,
      id: event.id,
      name: event.name,
      status: "running",
    }, chat.messages.length);
    return updateLastAssistant(chat, (message) => {
      const current = message.toolCalls ?? [];
      const existing = current.findIndex((tool) => tool.id === incoming.id);
      const next = [...current];
      if (existing >= 0) next[existing] = mergeSubagentTool(next[existing], incoming);
      else next.push(incoming);
      return { ...message, toolCalls: next };
    });
  }

  if (type === "subagent_start") {
    const id = stringValue(event.id);
    const subchatID = stringValue(event.subchatId ?? event.subchat_id);
    if (!id || !subchatID) return chat;
    const status = subchatStatusFromPayload(event.status) ?? "running";
    return updateTool(chat, id, (tool) => mergeSubagentTool(tool, {
      ...tool,
      result: { status: tool.result?.status ?? "success", subchatId: subchatID, subchatStatus: status },
    }));
  }

  if (type === "tool_result") {
    const id = stringValue(event.id);
    const result = normalizeToolResult(event.result, stringValue(event.error));
    return updateTool(chat, id, (tool) => mergeSubagentTool(tool, { ...tool, result, status: result.status }, true));
  }

  if (type === "chat_interrupted") {
    return { ...updateLastAssistant(chat, (message) => ({ ...message, status: "interrupted" })), status: "interrupted" };
  }
  if (type === "error") {
    const rateLimitNotice = rateLimitNoticeFromEvent(event);
    const next = rateLimitNotice ? stopOpenToolCalls(chat) : chat;
    return { ...next, rateLimitNotice: rateLimitNotice ?? chat.rateLimitNotice, status: "error" };
  }
  if (type === "run_end") {
    const exitCode = numberValue(event.exit_code ?? event.exitCode, undefined);
    const failed = exitCode !== undefined && exitCode !== 0;
    return { ...chat, rateLimitNotice: failed ? chat.rateLimitNotice : undefined, status: failed ? "error" : "success" };
  }
  return chat;
}

export function preserveLiveWorkState(previous: Chat, next: LiveChat): LiveChat {
  const previousTools = new Map(previous.messages.flatMap((message) => (message.toolCalls ?? []).map((tool) => [tool.id, tool] as const)));
  const archive = retainedArchive(previous, next);
  const seeded = preserveOptimisticMessages(previous, [...archive.history, ...next.messages.map((message) => archive.history.length > 0 && message.historyAvailable ? { ...message, historyAvailable: false } : message)]);
  const presentationIDs = presentationIDsFor(previous.messages, seeded);
  const messages = seeded.map((message, index) => {
    const previousMessage = previous.messages.find((candidate) => candidate.role === message.role && (
      candidate.id === message.id
      || (message.checkpointSeq !== undefined && candidate.checkpointSeq === message.checkpointSeq && candidate.checkpointBranch === message.checkpointBranch)
      || message.toolCalls?.some((tool) => candidate.toolCalls?.some((previousTool) => previousTool.id === tool.id))
    ));
    const tools = mergeSubagentCalls(previousMessage?.toolCalls, message.toolCalls ?? []);
    const presentationID = presentationIDs[index];
    return {
      ...message,
      ...(presentationID ? { presentationID } : {}),
      toolCalls: tools.length ? tools.map((tool) => mergeSubagentTool(previousTools.get(tool.id), tool)) : message.toolCalls,
    };
  });
  const startedAtByCheckpoint = new Map<number, number>();
  for (const message of previous.messages) {
    if (message.role !== "assistant" || message.checkpointSeq === undefined || message.workStartedAt === undefined) continue;
    if (!Number.isFinite(message.checkpointSeq) || !Number.isFinite(message.workStartedAt)) continue;
    startedAtByCheckpoint.set(message.checkpointSeq, message.workStartedAt);
  }
  return {
    ...next,
    knownCompactionIDs: archive.knownCompactionIDs,
    status: next.status ?? (previous.status && previous.status !== "running" ? previous.status : undefined),
    messages: messages.map((message) => {
      if (message.role !== "assistant" || message.workStartedAt !== undefined || message.checkpointSeq === undefined) return message;
      const workStartedAt = startedAtByCheckpoint.get(message.checkpointSeq);
      return workStartedAt === undefined ? message : { ...message, workStartedAt };
    }),
    rateLimitNotice: previous.rateLimitNotice,
    runStartedAt: next.runStartedAt ?? previous.runStartedAt,
  };
}

function stopOpenToolCalls(chat: LiveChat): LiveChat {
  return {
    ...chat,
    messages: chat.messages.map((message) => ({
      ...message,
      toolCalls: message.toolCalls?.map((tool) => tool.status === "running" ? { ...tool, status: "interrupted" } : tool),
    })),
  };
}

// The composer inserts the user's message with a local "user-" id before the server echoes it.
function isOptimisticUserMessage(message: ChatMessage | undefined): boolean {
  return message?.role === "user" && message.id.startsWith("user-");
}

function preserveOptimisticMessages(previous: Chat, nextMessages: ChatMessage[]): ChatMessage[] {
  if (nextMessages.length > 0) return nextMessages;

  const optimisticMessages = previous.messages.filter(isOptimisticUserMessage);
  return optimisticMessages.length ? [...optimisticMessages] : nextMessages;
}

function compactionNumber(id: string): number | undefined {
  const match = /^compaction-(\d+)$/.exec(id);
  return match ? Number(match[1]) : undefined;
}

function retainedArchive(previous: Chat, next: LiveChat): { history: ChatMessage[]; knownCompactionIDs?: string[] } {
  const known = new Set(previous.knownCompactionIDs ?? []);
  for (const message of previous.messages) if (message.kind === "compaction") known.add(message.id);
  const nextIDs = next.messages.filter((message) => message.kind === "compaction").map((message) => message.id);
  let maxKnown = -1;
  for (const id of known) {
    const index = compactionNumber(id);
    if (index !== undefined && index > maxKnown) maxKnown = index;
  }
  const valid = nextIDs.some((id) => known.has(id) || (maxKnown >= 0 && (compactionNumber(id) ?? -1) > maxKnown));
  for (const id of nextIDs) known.add(id);
  return {
    history: valid ? previous.messages.filter((message) => message.id.startsWith("m-archive-") && !next.messages.some((candidate) => candidate.id === message.id)) : [],
    knownCompactionIDs: known.size ? [...known] : undefined,
  };
}

function localUserIdentity(message: ChatMessage): string | undefined {
  if (message.role !== "user" || message.checkpointSeq !== undefined) return undefined;
  const identity = message.presentationID ?? message.id;
  return identity.startsWith("user-") || identity.startsWith("stream-user-") ? identity : undefined;
}

function carriedPresentationID(message: ChatMessage | undefined): string | undefined {
  if (!message) return undefined;
  return message.presentationID ?? (message.checkpointSeq === undefined ? message.id : undefined);
}

function presentationIDsFor(previous: ChatMessage[], next: ChatMessage[]): Array<string | undefined> {
  const claimed = new Set<ChatMessage>();
  const strong = next.map((message) => {
    const match = previous.find((candidate) => !claimed.has(candidate) && candidate.role === message.role && (
      candidate.id === message.id
      || (message.checkpointSeq !== undefined && candidate.checkpointSeq === message.checkpointSeq && candidate.checkpointBranch === message.checkpointBranch)
      || message.toolCalls?.some((tool) => candidate.toolCalls?.some((previousTool) => previousTool.id === tool.id))
    ));
    if (match) claimed.add(match);
    return match;
  });
  const locals = previous.filter((message) => !claimed.has(message) && localUserIdentity(message));
  const used = new Set<ChatMessage>();
  const paired = new Map<number, ChatMessage>();
  const openIndexes = next.flatMap((message, index) => !strong[index] && message.role === "user" ? [index] : []);
  for (const index of openIndexes) {
    const match = locals.find((candidate) => !used.has(candidate) && candidate.content === next[index].content);
    if (!match) continue;
    used.add(match);
    paired.set(index, match);
  }
  const leftoverLocals = locals.filter((candidate) => !used.has(candidate));
  const leftoverIndexes = openIndexes.filter((index) => !paired.has(index));
  if (leftoverLocals.length === 1 && leftoverIndexes.length === 1) paired.set(leftoverIndexes[0], leftoverLocals[0]);
  return next.map((_, index) => carriedPresentationID(strong[index] ?? paired.get(index)));
}

function updateLastAssistant(chat: LiveChat, update: (message: ChatMessage) => ChatMessage): LiveChat {
  for (let index = chat.messages.length - 1; index >= 0; index -= 1) {
    if (chat.messages[index].role !== "assistant") continue;
    const messages = [...chat.messages];
    messages[index] = update(messages[index]);
    return { ...chat, messages };
  }
  return chat;
}


function updateTool(chat: LiveChat, id: string, update: (tool: ChatToolCall) => ChatToolCall): LiveChat {
  if (!id) return chat;
  return {
    ...chat,
    messages: chat.messages.map((message) => message.toolCalls?.some((tool) => tool.id === id)
      ? { ...message, toolCalls: message.toolCalls.map((tool) => tool.id === id ? update(tool) : tool) }
      : message),
  };
}

function mergeSubagentTool(previous: ChatToolCall | undefined, incoming: ChatToolCall, completed = false): ChatToolCall {
  if (incoming.name !== "subagent" || !previous?.result?.subchatId) return incoming;
  if (incoming.result?.subchatId && incoming.result.subchatId !== previous.result.subchatId) return incoming;
  const currentStatus = previous.result.subchatStatus;
  const nextStatus = incoming.result?.subchatStatus;
  const terminal = currentStatus && currentStatus !== "running" && currentStatus !== "queued";
  const preserveFinal = terminal && (!nextStatus || nextStatus === "running" || nextStatus === "queued");
  const failed = completed && incoming.result?.status === "error";
  const subchatStatus = preserveFinal ? currentStatus : nextStatus ?? (failed ? "paused" : currentStatus);
  const result = { ...previous.result, ...incoming.result, subchatStatus };
  const status = subchatStatus === "running" || subchatStatus === "queued" ? "running"
    : subchatStatus === "done" ? "success" : subchatStatus ? "error" : incoming.status;
  return { ...incoming, result, status };
}

function mergeSubagentCalls(previous: ChatToolCall[] | undefined, incoming: ChatToolCall[]): ChatToolCall[] {
  const incomingByID = new Map(incoming.map((tool) => [tool.id, tool]));
  const retained = (previous ?? []).flatMap((tool) => {
    const next = incomingByID.get(tool.id);
    incomingByID.delete(tool.id);
    return next ? [mergeSubagentTool(tool, next)] : tool.name === "subagent" ? [tool] : [];
  });
  return [...retained, ...incomingByID.values()];
}
