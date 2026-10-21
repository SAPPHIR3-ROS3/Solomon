import type { Chat, ChatMessage, ChatStats, ChatToolCall } from "./chatTypes";
import type { CheckpointMetadata, IndexedChatMessage } from "./chatViewTypes";

// Preserve historical entry identities while streaming changes only the tail.
// The checkpoint context is part of the cache key because prepending/deleting
// messages can change fallback sequence numbers without changing the message.
const indexedEntries = new WeakMap<ChatMessage, { index: number; startSequence: number; startBranch: string; endSequence: number; endBranch: string; entry: IndexedChatMessage }>();

export function indexChatMessages(messages: ChatMessage[]): IndexedChatMessage[] {
  let fallbackSequence = -1;
  let fallbackBranch = "";

  return messages.map((message, index) => {
    const startSequence = fallbackSequence, startBranch = fallbackBranch;
    const cached = indexedEntries.get(message);
    if (cached && cached.index === index && cached.startSequence === startSequence && cached.startBranch === startBranch) {
      fallbackSequence = cached.endSequence;
      fallbackBranch = cached.endBranch;
      return cached.entry;
    }
    const retain = (entry: IndexedChatMessage) => {
      indexedEntries.set(message, { index, startSequence, startBranch, endSequence: fallbackSequence, endBranch: fallbackBranch, entry });
      return entry;
    };
    const toolMessageIDs = new Map<string, string>();
    if (message.role === "assistant") {
      for (const tool of message.toolCalls ?? []) toolMessageIDs.set(tool.id, message.id);
    }

    if (message.kind === "compaction") {
      return retain({ index, message, toolMessageIDs });
    }

    const hasExplicitSequence = typeof message.checkpointSeq === "number" && Number.isFinite(message.checkpointSeq);
    if (hasExplicitSequence) {
      fallbackSequence = Math.max(fallbackSequence, Math.max(0, Math.floor(message.checkpointSeq!)));
      if (message.checkpointBranch !== undefined) fallbackBranch = message.checkpointBranch;
    } else if (message.role === "user" || fallbackSequence < 0) {
      fallbackSequence += 1;
      fallbackBranch = "";
    }

    const sequence = hasExplicitSequence ? Math.max(0, Math.floor(message.checkpointSeq!)) : Math.max(0, fallbackSequence);
    const branch = message.checkpointBranch ?? fallbackBranch;
    const label = formatCheckpointLabel(sequence, branch);
    const displayMessage = addToolCheckpoints(message, sequence, branch, fallbackSequence);
    const toolCheckpointSequences = displayMessage.toolCalls?.flatMap((tool) => (
      typeof tool.checkpointSeq === "number" && Number.isFinite(tool.checkpointSeq) ? [Math.floor(tool.checkpointSeq)] : []
    )) ?? [];
    if (toolCheckpointSequences.length) fallbackSequence = Math.max(fallbackSequence, ...toolCheckpointSequences);

    return retain({
      checkpoint: { branch, label, sequence },
      index,
      message: displayMessage,
      toolMessageIDs,
    });
  });
}

export function toolCheckpoint(tool: ChatToolCall): CheckpointMetadata | undefined {
  if (typeof tool.checkpointSeq !== "number" || !Number.isFinite(tool.checkpointSeq)) return undefined;
  const sequence = Math.max(0, Math.floor(tool.checkpointSeq));
  const branch = tool.checkpointBranch ?? "";
  return { branch, label: formatCheckpointLabel(sequence, branch), sequence };
}

function addToolCheckpoints(message: ChatMessage, sequence: number, branch: string, lastUsedSequence: number): ChatMessage {
  if (!message.toolCalls?.length) return message;

  let fallbackSequence = Math.max(sequence + 1, lastUsedSequence + 1);
  const toolCalls = message.toolCalls.map((tool) => {
    if (toolCheckpoint(tool)) {
      fallbackSequence = Math.max(fallbackSequence, Math.floor(tool.checkpointSeq!) + 1);
      return tool;
    }
    const checkpointSeq = fallbackSequence;
    fallbackSequence += 1;
    return { ...tool, checkpointBranch: branch, checkpointSeq };
  });

  return { ...message, toolCalls };
}

export function groupChatTurns(messages: IndexedChatMessage[]): IndexedChatMessage[][] {
  const groups: IndexedChatMessage[][] = [];

  for (let index = 0; index < messages.length; index += 1) {
    const entry = messages[index];
    if (entry.message.kind === "compaction" || entry.message.role !== "assistant") {
      groups.push([entry]);
      continue;
    }

    const assistantMessages = [entry];
    while (index + 1 < messages.length && messages[index + 1].message.role === "assistant" && messages[index + 1].message.kind !== "compaction") {
      index += 1;
      assistantMessages.push(messages[index]);
    }
    groups.push(assistantMessages);
  }

  return groups;
}

export function liveWorkedForSeconds(chat: Chat, now: number): number | undefined {
  const groups = groupChatTurns(indexChatMessages(chat.messages));
  const lastAssistantGroupIndex = groups.reduce((lastIndex, entries, groupIndex) => (
    entries[0]?.message.role === "assistant" ? groupIndex : lastIndex
  ), -1);
  // A new user turn can be pending before assistant_start arrives. Its timer
  // must not be attached to the preceding, already completed assistant turn.
  if (lastAssistantGroupIndex < 0 || lastAssistantGroupIndex !== groups.length - 1) return undefined;

  const lastAssistantGroup = groups[lastAssistantGroupIndex];
  const startTimes = lastAssistantGroup
    .map(({ message }) => message.workStartedAt)
    .filter((value): value is number => value !== undefined && Number.isFinite(value));
  const assistantStartedAt = startTimes.length ? Math.min(...startTimes) : undefined;
  const startedAt = assistantStartedAt ?? chat.runStartedAt;
  if (startedAt === undefined || !Number.isFinite(startedAt)) return undefined;
  return Math.max(0, (now - startedAt) / 1000);
}

export function assistantFooterMessage(entries: IndexedChatMessage[]): ChatMessage {
  const messages = entries.map((entry) => entry.message);
  if (messages.length === 1) return messages[0];

  const first = messages[0];
  const last = messages[messages.length - 1];
  return {
    ...last,
    content: messages.map((message) => message.content.trim()).filter(Boolean).join("\n\n"),
    id: `assistant-turn-footer-${first.id}`,
    images: messages.flatMap((message) => message.images ?? []),
    reasoning: undefined,
    stats: aggregateAssistantStats(messages),
    toolCalls: undefined,
    workedFor: sumDefined(messages.map((message) => message.workedFor)),
  };
}

function aggregateAssistantStats(messages: ChatMessage[]): ChatStats | undefined {
  const stats = messages.flatMap((message) => message.stats ? [message.stats] : []);
  if (stats.length === 0) return undefined;
  if (stats.length === 1) return stats[0];

  const turnAggregate = [...stats].reverse().find((current) => current.turnAggregate);
  const reasoningTokens = turnAggregate?.reasoningTokens ?? stats.reduce((total, current) => total + current.reasoningTokens, 0);
  const responseTokens = turnAggregate?.responseTokens ?? stats.reduce((total, current) => total + current.responseTokens, 0);
  const contextTokens = turnAggregate?.contextTokens ?? stats.reduce((total, current) => total + current.contextTokens, 0);
  const userTokens = turnAggregate?.userTokens ?? stats.reduce((total, current) => total + current.userTokens, 0);
  const cachedPromptTokens = turnAggregate?.cachedPromptTokens ?? stats.reduce((total, current) => total + (current.cachedPromptTokens ?? 0), 0);
  const cacheCreationPromptTokens = turnAggregate?.cacheCreationPromptTokens ?? stats.reduce((total, current) => total + (current.cacheCreationPromptTokens ?? 0), 0);
  const outputTokensPerSecond = turnAggregate?.outputTokensPerSecond ?? average(stats.map((current) => current.outputTokensPerSecond));
  const promptTokensPerSecond = turnAggregate?.promptTokensPerSecond ?? average(stats.map((current) => current.promptTokensPerSecond));
  const totalTokens = turnAggregate?.totalTokens ?? stats.reduce((total, current) => total + current.totalTokens, 0);

  return {
    reasoningEffort: [...new Set(stats.map((current) => current.reasoningEffort).filter(Boolean))].join(", ") || undefined,
    provider: [...new Set(stats.map((current) => current.provider).filter(Boolean))].join(", ") || undefined,
    model: [...new Set(stats.map((current) => current.model).filter(Boolean))].join(", ") || undefined,
    contextTokens,
    cachedPromptTokens,
    cacheCreationPromptTokens,
    outputTokensPerSecond,
    promptTokensPerSecond,
    reasoningTokens,
    responseTokens,
    totalTokens,
    ttftSeconds: turnAggregate?.ttftSeconds ?? stats[0].ttftSeconds,
    userTokens,
  };
}

function sumDefined(values: Array<number | undefined>) {
  const defined = values.filter((value): value is number => value !== undefined && Number.isFinite(value));
  return defined.length ? defined.reduce((total, value) => total + value, 0) : undefined;
}

function average(values: number[]) {
  if (values.length === 0) return 0;
  return values.reduce((total, value) => total + value, 0) / values.length;
}

function formatCheckpointLabel(sequence: number, branch: string) {
  return `[#${String(sequence).padStart(3, "0")}${branch}]`;
}

export type ChatBannerError = {
  hint?: string;
  message?: string;
  plan?: string;
  resetLabel?: string;
  source?: string;
  title: string;
};

export function rateLimitNoticeFromEvent(event: Record<string, unknown>): string | undefined {
  const raw = ["detail", "error", "message"]
    .map((key) => typeof event[key] === "string" ? event[key].trim() : "")
    .filter(Boolean)
    .join("\n");
  if (!raw || parseChatBannerError(raw).title !== "Usage limit reached") return undefined;
  return raw;
}

export function parseChatBannerError(raw: string): ChatBannerError {
  let text = raw.trim();
  let source: string | undefined;
  for (const prefix of ["ChatGPT Sub", "Claude Sub"]) {
    if (text.startsWith(`${prefix}:`) || text.startsWith(`${prefix} `)) {
      source = prefix;
      const colon = text.indexOf(":");
      text = colon >= 0 ? text.slice(colon + 1).trim() : text.slice(prefix.length).trim();
      break;
    }
  }
  const jsonStart = text.indexOf("{");
  if (jsonStart >= 0) {
    try {
      const parsed = JSON.parse(text.slice(jsonStart)) as Record<string, unknown>;
      const nested = parsed.error;
      const body = nested && typeof nested === "object" && !Array.isArray(nested) ? nested as Record<string, unknown> : parsed;
      if (typeof body.type === "string" || typeof body.message === "string" || typeof body.detail === "string") {
        return bannerFromAPIBody(body, source);
      }
    } catch {
      /* keep the original text */
    }
  }
  if (/usage limit/i.test(text) || /rate[\s-]?limit/i.test(text) || /\b429\b/.test(text)) {
    const plan = text.match(/\(([^)\n]+) plan\)/i)?.[1] ?? text.match(/^plan:\s*(.+)$/im)?.[1];
    const reset = text.match(/^reset:\s*(.+)$/im)?.[1] ?? text.match(/resets?\s+(.+)$/im)?.[1];
    const message = (text.match(/^message:\s*(.+)$/im)?.[1] ?? text.replace(/^summary:\s*.+$/im, "").replace(/\s*\([^)]+ plan\)/i, "").replace(/;\s*resets?\s+.+$/im, "").replace(/^(type|plan|reset|hint|HTTP|attempts):\s*.+$/gim, "").trim()) || "The usage limit has been reached";
    return {
      title: "Usage limit reached",
      source,
      message,
      plan: plan ? plan.charAt(0).toUpperCase() + plan.slice(1) : undefined,
      resetLabel: reset ? `Resets ${reset}` : undefined,
      hint: "Wait for the reset, or pick another model.",
    };
  }
  return { title: "Error", source, message: text };
}

function bannerFromAPIBody(body: Record<string, unknown>, source?: string): ChatBannerError {
  const type = typeof body.type === "string" ? body.type : "";
  const message = (typeof body.message === "string" ? body.message : typeof body.detail === "string" ? body.detail : "").trim();
  const plan = typeof body.plan_type === "string" ? body.plan_type.trim() : "";
  const resetsAt = typeof body.resets_at === "number" ? body.resets_at : 0;
  const resetsIn = typeof body.resets_in_seconds === "number" ? body.resets_in_seconds : 0;
  const limit = type === "usage_limit_reached" || type === "rate_limit_error";
  return {
    title: errorBannerTitle(type),
    source,
    message: message || undefined,
    plan: plan ? plan.charAt(0).toUpperCase() + plan.slice(1) : undefined,
    resetLabel: formatErrorResetLabel(resetsAt, resetsIn),
    hint: limit ? "Wait for the reset, or pick another model." : undefined,
  };
}

function errorBannerTitle(type: string): string {
  switch (type) {
    case "usage_limit_reached":
    case "rate_limit_error":
      return "Usage limit reached";
    case "authentication_error":
      return "Sign-in failed";
    case "permission_error":
      return "Permission denied";
    case "overloaded_error":
      return "Provider overloaded";
    case "not_found_error":
      return "Not found";
    case "invalid_request_error":
      return "Invalid request";
    default:
      return "Request failed";
  }
}

function formatErrorResetLabel(resetsAt: number, resetsInSeconds: number): string | undefined {
  const when = resetsAt > 0 ? new Date(resetsAt * 1000) : undefined;
  let seconds = resetsInSeconds;
  if (seconds <= 0 && when) seconds = Math.max(0, Math.round((when.getTime() - Date.now()) / 1000));
  const wait = formatResetWait(seconds);
  if (when && wait) return `Resets ${when.toLocaleString()} · ${wait}`;
  if (when) return `Resets ${when.toLocaleString()}`;
  return wait ? `Resets ${wait}` : undefined;
}

function formatResetWait(seconds: number): string | undefined {
  if (seconds <= 0) return undefined;
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (hours > 0 && minutes > 0) return `in ${hours}h ${minutes}m`;
  if (hours > 0) return `in ${hours}h`;
  if (minutes > 0) return `in ${minutes}m`;
  return `in ${seconds}s`;
}
