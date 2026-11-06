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
      toolCalls: toolCalls.length ? toolCalls.map((tool) => mergeSubagentTool(message.toolCalls?.find((previous) => previous.id === tool.id), tool)) : message.toolCalls,
    }));
  }

  if (type === "tool_start") {
    const incoming = toolCallFromPayload({
      arguments: event.arguments,
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
  const messages = preserveOptimisticMessages(previous, next.messages).map((message) => ({
    ...message,
    toolCalls: message.toolCalls?.map((tool) => mergeSubagentTool(previousTools.get(tool.id), tool)),
  }));
  const startedAtByCheckpoint = new Map<number, number>();
  for (const message of previous.messages) {
    if (message.role !== "assistant" || message.checkpointSeq === undefined || message.workStartedAt === undefined) continue;
    if (!Number.isFinite(message.checkpointSeq) || !Number.isFinite(message.workStartedAt)) continue;
    startedAtByCheckpoint.set(message.checkpointSeq, message.workStartedAt);
  }
  return {
    ...next,
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
