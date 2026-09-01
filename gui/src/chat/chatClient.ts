import { serverEndpoint } from "../platform";
import type { ComposerImageAttachment } from "./composerTypes";
import type { Chat, ChatImage, ChatMessage, ChatStats, ChatTodoItem, ChatToolCall, ChatToolResult } from "./chatTypes";

export type ChatImageInput = {
  data: string;
  name: string;
};

export type LiveChat = Chat & {
  source: "daemon";
  projectID: string;
  subchatStatus?: ChatToolResult["subchatStatus"];
};

export type ChatStreamEvent = Record<string, unknown>;

export type ChatStreamEventHandler = (event: ChatStreamEvent, replay?: boolean, imageOrigin?: string) => void;

export async function fetchLiveChat(projectID: string, chatID: string, signal?: AbortSignal): Promise<LiveChat> {
  const endpoint = await serverEndpoint(chatPath(projectID, chatID));
  const response = await fetch(endpoint, { cache: "no-store", signal });
  const payload = await readResponsePayload(response, "Unable to load chat");
  return liveChatFromPayload(payload, projectID, assetOrigin(endpoint));
}

export async function createLiveChat(projectID: string, title = "", signal?: AbortSignal): Promise<LiveChat> {
  const endpoint = await serverEndpoint(`${projectChatsPath(projectID)}`);
  const response = await fetch(endpoint, {
    body: JSON.stringify(title.trim() ? { title: title.trim() } : {}),
    headers: { "Content-Type": "application/json" },
    method: "POST",
    signal,
  });
  const payload = await readResponsePayload(response, "Unable to create chat");
  return liveChatFromPayload(payload, projectID, assetOrigin(endpoint));
}

export async function deleteLiveChatMessage(projectID: string, chatID: string, messageID: string): Promise<LiveChat> {
  const endpoint = await serverEndpoint(`${chatPath(projectID, chatID)}/messages/${encodeURIComponent(messageID)}`);
  const response = await fetch(endpoint, { method: "DELETE" });
  const payload = await readResponsePayload(response, "Unable to delete message");
  return liveChatFromPayload(payload, projectID, assetOrigin(endpoint));
}

export async function stopLiveChat(projectID: string, chatID: string): Promise<void> {
  const endpoint = await serverEndpoint(`${chatPath(projectID, chatID)}/stop`);
  const response = await fetch(endpoint, { method: "POST" });
  await readResponsePayload(response, "Unable to stop chat");
}

export async function fetchLiveSubchat(projectID: string, subchatID: string): Promise<LiveChat> {
  const endpoint = await serverEndpoint(`${projectSubchatsPath(projectID)}/${encodeURIComponent(subchatID)}`);
  const response = await fetch(endpoint, { cache: "no-store" });
  const payload = await readResponsePayload(response, "Unable to load subchat");
  return liveChatFromPayload(payload, projectID, assetOrigin(endpoint));
}

export async function controlLiveSubchat(projectID: string, subchatID: string, action: "stop" | "cancel" | "resume"): Promise<void> {
  const endpoint = await serverEndpoint(`${projectSubchatsPath(projectID)}/${encodeURIComponent(subchatID)}`);
  const response = await fetch(endpoint, {
    body: JSON.stringify({ action }),
    headers: { "Content-Type": "application/json" },
    method: "POST",
  });
  await readResponsePayload(response, "Unable to control subchat");
}

export async function streamLiveChatMessage(
  projectID: string,
  chatID: string,
  content: string,
  images: ChatImageInput[],
  signal: AbortSignal,
  onEvent: ChatStreamEventHandler,
): Promise<void> {
  const endpoint = await serverEndpoint(`${chatPath(projectID, chatID)}/messages`);
  const response = await fetch(endpoint, {
    body: JSON.stringify({ content, images }),
    headers: { "Content-Type": "application/json", Accept: "text/event-stream" },
    method: "POST",
    signal,
  });
  if (!response.ok) {
    await readResponsePayload(response, "Unable to send message");
    return;
  }
  await consumeChatStream(response, onEvent, false, assetOrigin(endpoint));
}

export async function streamLiveChatEvents(
  projectID: string,
  chatID: string,
  startingAfter: number,
  signal: AbortSignal,
  onEvent: ChatStreamEventHandler,
): Promise<void> {
  const endpoint = await serverEndpoint(`${chatPath(projectID, chatID)}/events?starting_after=${Math.max(0, Math.floor(startingAfter))}`);
  const response = await fetch(endpoint, {
    headers: { Accept: "text/event-stream" },
    method: "GET",
    signal,
  });
  if (!response.ok) {
    await readResponsePayload(response, "Unable to reconnect chat stream");
    return;
  }
  await consumeChatStream(response, onEvent, true, assetOrigin(endpoint));
}

async function consumeChatStream(
  response: Response,
  onEvent: ChatStreamEventHandler,
  replay: boolean,
  imageOrigin: string,
): Promise<void> {
  if (!response.body) throw new Error("Chat stream has no response body");
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  while (true) {
    const { done, value } = await reader.read();
    buffer += decoder.decode(value ?? new Uint8Array(), { stream: !done });
    const frames = buffer.split(/\r?\n\r?\n/);
    buffer = frames.pop() ?? "";
    for (const frame of frames) {
      const event = parseSSEFrame(frame);
      if (event) onEvent(event, replay, imageOrigin);
    }
    if (done) break;
  }
  const finalEvent = parseSSEFrame(buffer);
  if (finalEvent) onEvent(finalEvent, replay, imageOrigin);
}

export async function snapshotComposerImages(images: ComposerImageAttachment[]): Promise<ChatImage[]> {
  return Promise.all(images.map(async (image) => ({
    name: image.name,
    url: image.blob ? await blobToDataURL(image.blob) : image.url,
  })));
}

export function chatImageInputs(images: ChatImage[] | undefined): ChatImageInput[] {
  return (images ?? []).map((image) => ({ data: image.url, name: image.name }));
}

export function liveChatFromPayload(payload: unknown, projectID: string, imageOrigin = ""): LiveChat {
  const record = objectValue(payload);
  const messages = arrayValue(record.messages).flatMap((message, index) => {
    const parsed = messageFromPayload(message, index, imageOrigin);
    return parsed ? [parsed] : [];
  });
  return {
    createdAt: dateToMilliseconds(stringValue(record.createdAt)),
    id: stringValue(record.id),
    messages,
    modeSwitchTarget: record.mode === "agent" ? undefined : "agent",
    projectID,
    source: "daemon",
    status: chatStatusFromPayload(record.status),
    subchatStatus: subchatStatusFromPayload(record.status),
    title: stringValue(record.title) || "Untitled chat",
    branch: stringValue(record.branch) || undefined,
    worktree: stringValue(record.worktree) || undefined,
    workspaceName: stringValue(record.workspaceName) || undefined,
    workspacePath: stringValue(record.workspacePath) || undefined,
  };
}

export function applyLiveStreamEvent(chat: LiveChat, event: ChatStreamEvent, imageOrigin = "", replay = false): LiveChat {
  const type = stringValue(event.type);
  if (type === "chat_snapshot") {
    const snapshot = event.chat;
    if (snapshot) return preserveLiveWorkState(chat, liveChatFromPayload(snapshot, chat.projectID, imageOrigin));
    return chat;
  }
  if (type === "chat_start") {
    const runStartedAt = chat.runStartedAt ?? eventTimeMilliseconds(event);
    if (!replay || !event.chat) return { ...chat, runStartedAt, status: "running" };
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
      toolCalls: toolCalls.length ? toolCalls : message.toolCalls,
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
      if (existing >= 0) next[existing] = { ...next[existing], ...incoming, status: "running" };
      else next.push(incoming);
      return { ...message, toolCalls: next };
    });
  }

  if (type === "tool_result") {
    const id = stringValue(event.id);
    const result = normalizeToolResult(event.result, stringValue(event.error));
    return updateLastAssistant(chat, (message) => ({
      ...message,
      toolCalls: (message.toolCalls ?? []).map((tool) => tool.id === id
        ? { ...tool, result, status: result.status }
        : tool),
    }));
  }

  if (type === "chat_interrupted") {
    return { ...updateLastAssistant(chat, (message) => ({ ...message, status: "interrupted" })), status: "interrupted" };
  }
  if (type === "error") return { ...chat, status: "error" };
  if (type === "run_end") {
    const exitCode = numberValue(event.exit_code ?? event.exitCode, undefined);
    return { ...chat, status: exitCode !== undefined && exitCode !== 0 ? "error" : "success" };
  }
  return chat;
}

export function preserveLiveWorkState(previous: Chat, next: LiveChat): LiveChat {
  const startedAtByCheckpoint = new Map<number, number>();
  for (const message of previous.messages) {
    if (message.role !== "assistant" || message.checkpointSeq === undefined || message.workStartedAt === undefined) continue;
    if (!Number.isFinite(message.checkpointSeq) || !Number.isFinite(message.workStartedAt)) continue;
    startedAtByCheckpoint.set(message.checkpointSeq, message.workStartedAt);
  }
  return {
    ...next,
    status: next.status ?? (previous.status && previous.status !== "running" ? previous.status : undefined),
    messages: next.messages.map((message) => {
      if (message.role !== "assistant" || message.workStartedAt !== undefined || message.checkpointSeq === undefined) return message;
      const workStartedAt = startedAtByCheckpoint.get(message.checkpointSeq);
      return workStartedAt === undefined ? message : { ...message, workStartedAt };
    }),
    runStartedAt: next.runStartedAt ?? previous.runStartedAt,
  };
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

function messageFromPayload(payload: unknown, index: number, imageOrigin: string): ChatMessage | null {
  const record = objectValue(payload);
  const role = stringValue(record.role);
  if (role !== "user" && role !== "assistant") return null;
  const message: ChatMessage = {
    checkpointBranch: stringValue(record.checkpointBranch) || undefined,
    checkpointSeq: numberValue(record.checkpointSeq, undefined),
    content: stringValue(record.content),
    id: stringValue(record.id) || `m-${index}`,
    images: imagesFromPayload(record.images, imageOrigin),
    role,
  };
  if (record.kind === "compaction") message.kind = "compaction";
  if (typeof record.reasoning === "string") message.reasoning = record.reasoning;
  if (record.stats) message.stats = statsFromPayload(record.stats);
  if (typeof record.status === "string" && record.status === "interrupted") message.status = "interrupted";
  if (typeof record.summary === "string") message.summary = record.summary;
  if (typeof record.thoughtFor === "number") message.thoughtFor = record.thoughtFor;
  if (typeof record.workedFor === "number") message.workedFor = record.workedFor;
  if (Array.isArray(record.toolCalls)) message.toolCalls = record.toolCalls.map((tool, toolIndex) => toolCallFromPayload(tool, toolIndex, imageOrigin));
  if (Array.isArray(record.retainedMessages)) {
    message.retainedMessages = record.retainedMessages.flatMap((retained) => {
      const value = objectValue(retained);
      const retainedRole = stringValue(value.role);
      if (retainedRole !== "user" && retainedRole !== "assistant") return [];
      return [{ content: stringValue(value.content), images: imagesFromPayload(value.images, imageOrigin), role: retainedRole }];
    });
  }
  return message;
}

function toolCallFromPayload(payload: unknown, index: number, _imageOrigin = ""): ChatToolCall {
  const record = objectValue(payload);
  const args = objectValue(record.arguments);
  const result = record.result ? normalizeToolResult(record.result, stringValue(record.error)) : undefined;
  const name = stringValue(record.name) || "tool";
  return {
    checkpointBranch: stringValue(record.checkpointBranch ?? record.checkpoint_branch) || undefined,
    checkpointSeq: numberValue(record.checkpointSeq ?? record.checkpoint_seq, undefined),
    defaultOpen: Boolean(record.defaultOpen),
    delete: Boolean(record.delete),
    full: Boolean(record.full),
    id: stringValue(record.id) || `tool-${index}`,
    input: stringValue(record.input) || firstString(args, ["source", "command", "path", "task", "query", "name", "pattern", "url", "id"]),
    intent: stringValue(record.intent) || stringValue(args.intent),
    mode: stringValue(record.mode) as ChatToolCall["mode"],
    name,
    newString: stringValue(record.newString) || stringValue(args.newString),
    oldString: stringValue(record.oldString) || stringValue(args.oldString),
    parameters: parametersFromPayload(record.parameters),
    renameTo: stringValue(record.renameTo) || stringValue(args.renameTo),
    result,
    status: (stringValue(record.status) || result?.status || "running") as ChatToolCall["status"],
    sync: subagentSyncFromPayload(name, record, args),
  };
}

function normalizeToolResult(payload: unknown, error = ""): ChatToolResult {
  const record = objectValue(payload);
  const recordStatus = stringValue(record.status).toLowerCase();
  const recordError = typeof record.error === "string"
    ? record.error.trim()
    : firstString(objectValue(record.error), ["message", "detail", "error"]);
  const compileError = typeof record.compile_error === "string" ? record.compile_error.trim() : "";
  const explicitError = error.trim() || recordError || compileError || (
    (record.ok === false || recordStatus === "error" || recordStatus === "failed")
      ? firstString(record, ["message", "detail"])
      : ""
  );
  const result: ChatToolResult = {
    status: explicitError || record.ok === false || compileError || recordStatus === "error" || recordStatus === "failed" ? "error" : "success",
  };
  if (explicitError) result.error = explicitError;
  copyNumber(result, "count", record.count);
  copyNumber(result, "completed", record.completed);
  copyNumber(result, "durationMs", record.durationMs ?? record.duration_ms);
  copyNumber(result, "maxRounds", record.maxRounds ?? record.max_rounds);
  copyNumber(result, "round", record.round);
  copyNumber(result, "truncated", record.truncated);
  const jobId = record.jobId ?? record.job_id;
  if (typeof jobId === "string") result.jobId = jobId;
  const sdkCalls = record.sdkCalls ?? record.tool_calls;
  if (typeof sdkCalls === "number" && Number.isFinite(sdkCalls)) result.sdkCalls = sdkCalls;
  else if (Array.isArray(sdkCalls)) result.sdkCalls = sdkCalls.length;
  if (typeof record.output === "string") result.output = record.output;
  if (typeof record.summary === "string") result.summary = record.summary;
  if (typeof record.subchatId === "string") result.subchatId = record.subchatId;
  if (typeof record.subchat_id === "string") result.subchatId = record.subchat_id;
  const subchatStatus = record.subagentStatus ?? record.subchatStatus;
  if (typeof subchatStatus === "string" && ["cancelled", "done", "paused", "queued", "running"].includes(subchatStatus)) {
    result.subchatStatus = subchatStatus as ChatToolResult["subchatStatus"];
  }
  if (typeof record.phase === "string") result.phase = record.phase;
  if (typeof record.title === "string") result.title = record.title;
  const researchStatus = record.researchStatus ?? record.research_status;
  if (typeof researchStatus === "string") result.researchStatus = researchStatus as ChatToolResult["researchStatus"];
  else if (typeof record.status === "string" && ["cancelled", "done", "failed", "paused", "running"].includes(record.status)) result.researchStatus = record.status as ChatToolResult["researchStatus"];
  if (Array.isArray(record.items)) result.items = record.items.filter((item): item is string => typeof item === "string");
  const todoItems = record.todoItems ?? record.todo_items;
  if (Array.isArray(todoItems)) result.todoItems = todoItems.flatMap(todoItemFromPayload);
  if (!result.output && record.reason && typeof record.reason === "string") result.output = record.reason;
  return result;
}

function copyNumber(result: ChatToolResult, key: "completed" | "count" | "durationMs" | "maxRounds" | "round" | "truncated", value: unknown) {
  if (typeof value === "number" && Number.isFinite(value)) (result as Record<string, unknown>)[key] = value;
  if (typeof value === "boolean") (result as Record<string, unknown>)[key] = value;
}

function todoItemFromPayload(payload: unknown): ChatTodoItem[] {
  const record = objectValue(payload);
  return typeof record.text === "string" ? [{ checked: Boolean(record.checked), text: record.text }] : [];
}

function statsFromPayload(payload: unknown): ChatStats {
  const record = objectValue(payload);
  return {
    contextTokens: numberValue(record.contextTokens, 0) ?? 0,
    outputTokensPerSecond: numberValue(record.outputTokensPerSecond, 0) ?? 0,
    promptTokensPerSecond: numberValue(record.promptTokensPerSecond, 0) ?? 0,
    reasoningTokens: numberValue(record.reasoningTokens, 0) ?? 0,
    responseTokens: numberValue(record.responseTokens, 0) ?? 0,
    totalTokens: numberValue(record.totalTokens, 0) ?? 0,
    ttftSeconds: numberValue(record.ttftSeconds, 0) ?? 0,
    userTokens: numberValue(record.userTokens, 0) ?? 0,
  };
}

function imagesFromPayload(payload: unknown, imageOrigin: string): ChatImage[] {
  return arrayValue(payload).flatMap((image) => {
    const record = objectValue(image);
    const rawURL = stringValue(record.url);
    if (!rawURL) return [];
    return [{ name: stringValue(record.name) || "image", url: absoluteAssetURL(rawURL, imageOrigin) }];
  });
}

function parametersFromPayload(payload: unknown): ChatToolCall["parameters"] {
  return arrayValue(payload).flatMap((parameter) => {
    const record = objectValue(parameter);
    const label = stringValue(record.label);
    const value = stringValue(record.value);
    return label ? [{ label, value }] : [];
  });
}

function firstString(record: Record<string, unknown>, keys: string[]): string {
  for (const key of keys) {
    if (typeof record[key] === "string" && record[key]) return record[key] as string;
  }
  return "";
}

function subagentSyncFromPayload(name: string, record: Record<string, unknown>, args: Record<string, unknown>): boolean {
  if (name !== "subagent") return false;
  if (record.sync === true) return true;
  if (!Object.prototype.hasOwnProperty.call(record, "arguments")) return false;
  return !booleanValue(args.run_in_background);
}

function booleanValue(value: unknown): boolean {
  if (typeof value === "boolean") return value;
  if (typeof value !== "string") return false;
  switch (value.trim().toLowerCase()) {
    case "1":
    case "true":
    case "yes":
    case "on":
      return true;
    default:
      return false;
  }
}

function objectValue(value: unknown): Record<string, unknown> {
  return value && typeof value === "object" && !Array.isArray(value) ? value as Record<string, unknown> : {};
}

function arrayValue(value: unknown): unknown[] {
  return Array.isArray(value) ? value : [];
}

function stringValue(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function subchatStatusFromPayload(value: unknown): ChatToolResult["subchatStatus"] | undefined {
  if (typeof value !== "string" || !["cancelled", "done", "paused", "queued", "running"].includes(value)) return undefined;
  return value as ChatToolResult["subchatStatus"];
}

function chatStatusFromPayload(value: unknown): Chat["status"] | undefined {
  if (typeof value !== "string" || !["error", "interrupted", "running", "success"].includes(value)) return undefined;
  return value as Chat["status"];
}

function numberValue(value: unknown, fallback: number | undefined): number | undefined {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

function dateToMilliseconds(value: string): number | undefined {
  if (!value) return undefined;
  const parsed = Date.parse(value);
  return Number.isNaN(parsed) ? undefined : parsed;
}

function eventTimeMilliseconds(event: ChatStreamEvent): number {
  return dateToMilliseconds(stringValue(event.ts)) ?? Date.now();
}

function parseSSEFrame(frame: string): ChatStreamEvent | null {
  const data = frame.split(/\r?\n/)
    .filter((line) => line.startsWith("data:"))
    .map((line) => line.slice(5).trimStart())
    .join("\n");
  if (!data) return null;
  try {
    const value: unknown = JSON.parse(data);
    return value && typeof value === "object" && !Array.isArray(value) ? value as ChatStreamEvent : null;
  } catch {
    return null;
  }
}

async function readResponsePayload(response: Response, fallback: string): Promise<unknown> {
  const text = await response.text();
  let payload: unknown;
  try {
    payload = text ? JSON.parse(text) : undefined;
  } catch {
    payload = undefined;
  }
  if (!response.ok) {
    const rawError = objectValue(payload).error;
    const nestedError = objectValue(rawError);
    const error = typeof rawError === "string"
      ? rawError.trim()
      : firstString(nestedError, ["message", "detail", "error"]);
    const bodyText = text.trim().replace(/\s+/g, " ");
    throw new Error(error || bodyText.slice(0, 500) || `${fallback}: ${response.status}`);
  }
  return payload;
}

function projectChatsPath(projectID: string): string {
  return `/__solomon/projects/${encodeURIComponent(projectID)}/chats`;
}

function projectSubchatsPath(projectID: string): string {
  return `/__solomon/projects/${encodeURIComponent(projectID)}/subchats`;
}

function chatPath(projectID: string, chatID: string): string {
  return `${projectChatsPath(projectID)}/${encodeURIComponent(chatID)}`;
}

function assetOrigin(endpoint: string): string {
  try {
    return new URL(endpoint).origin;
  } catch {
    return "";
  }
}

function absoluteAssetURL(value: string, origin: string): string {
  if (!origin || value.startsWith("data:") || value.startsWith("blob:") || value.startsWith("http://") || value.startsWith("https://")) return value;
  if (value.startsWith("/")) return `${origin}${value}`;
  return value;
}

function blobToDataURL(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.addEventListener("error", () => reject(reader.error ?? new Error("Unable to read image attachment")));
    reader.addEventListener("load", () => typeof reader.result === "string" ? resolve(reader.result) : reject(new Error("Unable to encode image attachment")));
    reader.readAsDataURL(blob);
  });
}
