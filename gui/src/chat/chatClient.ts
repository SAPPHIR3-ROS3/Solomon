import { chatRequest, contextualChatError } from "./chatRequest";
import { notifyNotification } from "../notifications/store";
import { measureNavigation, measureNavigationSync } from "./navigationPerformance";
import { initialClient, serverEndpoint } from "../platform";
import type { ComposerImageAttachment } from "./composerTypes";
export { applyLiveStreamEvent, preserveLiveWorkState } from "./events";
import { arrayValue, objectValue, stringValue, firstString, messageFromPayload, dateToMilliseconds, chatStatusFromPayload, subchatStatusFromPayload } from "./payload";
import type { Chat, ChatImage, ChatToolResult } from "./chatTypes";

export type ChatImageInput = {
  data: string;
  name: string;
};

export type ChatClipInput = {
  end: number;
  start: number;
  tag: string;
  text: string;
};

export type LiveChat = Chat & {
  source: "daemon";
  projectID: string;
  subchatStatus?: ChatToolResult["subchatStatus"];
};

export type ChatStreamEvent = Record<string, unknown>;

export type ChatStreamEventHandler = (event: ChatStreamEvent, replay?: boolean, imageOrigin?: string) => void;

export async function fetchChatHistory(projectID: string, chatID: string): Promise<Chat["messages"]> {
  const endpoint = await serverEndpoint(`${chatPath(projectID, chatID)}/history`);
  const response = await chatRequest("Unable to load earlier messages", endpoint, { cache: "no-store" });
  const payload = await readResponsePayload(response, "Unable to load earlier messages");
  const origin = assetOrigin(endpoint);
  return arrayValue(objectValue(payload).messages).flatMap((message, index) => {
    const parsed = messageFromPayload(message, index, origin);
    return parsed ? [parsed] : [];
  });
}

export async function fetchLiveChat(projectID: string, chatID: string, signal?: AbortSignal, navigationID?: number): Promise<LiveChat> {
  const endpoint = await measureNavigation(navigationID, "chat.endpoint", () => serverEndpoint(chatPath(projectID, chatID)));
  const response = await measureNavigation(navigationID, "chat.headers", () => chatRequest("Unable to load chat", endpoint, { cache: "no-store", signal }));
  const payload = await readResponsePayload(response, "Unable to load chat", navigationID);
  return measureNavigationSync(navigationID, "chat.normalize", () => liveChatFromPayload(payload, projectID, assetOrigin(endpoint)));
}

export async function createLiveChat(projectID: string, title = "", signal?: AbortSignal): Promise<LiveChat> {
  const endpoint = await serverEndpoint(`${projectChatsPath(projectID)}`);
  const response = await chatRequest("Unable to create chat", endpoint, {
    body: JSON.stringify(title.trim() ? { title: title.trim() } : {}),
    headers: { "Content-Type": "application/json" },
    method: "POST",
    signal,
  });
  const payload = await readResponsePayload(response, "Unable to create chat");
  return liveChatFromPayload(payload, projectID, assetOrigin(endpoint));
}

// Resolves true when the command starts an agent turn and belongs in a new chat.
export async function sendProjectCommand(projectID: string, content: string, signal?: AbortSignal): Promise<boolean> {
  const response = await chatRequest("Unable to run command", await serverEndpoint(projectID ? `${projectChatsPath(projectID).replace(/\/chats$/, "")}/commands` : "/__solomon/commands"), {
    body: JSON.stringify({ content, surface: initialClient().surface }),
    headers: { "Content-Type": "application/json" },
    method: "POST",
    signal,
  });
  const payload = objectValue(await readResponsePayload(response, "Unable to run command"));
  if (Array.isArray(payload.notifications)) {
    for (const item of payload.notifications) {
      const notification = objectValue(item);
      if (typeof notification.message === "string") notifyNotification(notification.message, stringValue(notification.level) || "system");
    }
  }
  return payload.open_chat === true;
}

export async function renameLiveChat(projectID: string, chatID: string, title: string): Promise<LiveChat> {
  const endpoint = await serverEndpoint(chatPath(projectID, chatID));
  const response = await chatRequest("Unable to rename chat", endpoint, {
    body: JSON.stringify({ title }),
    headers: { "Content-Type": "application/json" },
    method: "PATCH",
  });
  const payload = await readResponsePayload(response, "Unable to rename chat");
  return liveChatFromPayload(payload, projectID, assetOrigin(endpoint));
}

export async function deleteLiveChat(projectID: string, chatID: string): Promise<void> {
  const endpoint = await serverEndpoint(chatPath(projectID, chatID));
  await readResponsePayload(await chatRequest("Unable to delete chat", endpoint, { method: "DELETE" }), "Unable to delete chat");
}

export async function deleteLiveChatMessage(projectID: string, chatID: string, messageID: string): Promise<LiveChat> {
  const endpoint = await serverEndpoint(`${chatPath(projectID, chatID)}/messages/${encodeURIComponent(messageID)}`);
  const response = await chatRequest("Unable to delete message", endpoint, { method: "DELETE" });
  const payload = await readResponsePayload(response, "Unable to delete message");
  return liveChatFromPayload(payload, projectID, assetOrigin(endpoint));
}

export async function stopLiveChat(projectID: string, chatID: string): Promise<void> {
  const endpoint = await serverEndpoint(`${chatPath(projectID, chatID)}/stop`);
  const response = await chatRequest("Unable to stop chat", endpoint, { method: "POST" });
  await readResponsePayload(response, "Unable to stop chat");
}

export async function fetchLiveSubchat(projectID: string, subchatID: string): Promise<LiveChat> {
  const endpoint = await serverEndpoint(`${projectSubchatsPath(projectID)}/${encodeURIComponent(subchatID)}`);
  const response = await chatRequest("Unable to load subchat", endpoint, { cache: "no-store" });
  const payload = await readResponsePayload(response, "Unable to load subchat");
  return liveChatFromPayload(payload, projectID, assetOrigin(endpoint));
}

export async function controlLiveSubchat(projectID: string, subchatID: string, action: "stop" | "cancel" | "resume"): Promise<void> {
  const endpoint = await serverEndpoint(`${projectSubchatsPath(projectID)}/${encodeURIComponent(subchatID)}`);
  const response = await chatRequest("Unable to control subchat", endpoint, {
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
  clips: ChatClipInput[],
  signal: AbortSignal,
  onEvent: ChatStreamEventHandler,
): Promise<void> {
  const endpoint = await serverEndpoint(`${chatPath(projectID, chatID)}/messages`);
  const response = await chatRequest("Unable to send message", endpoint, {
    body: JSON.stringify({ content, images, clips, surface: initialClient().surface }),
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
  const response = await chatRequest("Unable to reconnect chat stream", endpoint, {
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
    const { done, value } = await reader.read().catch((cause: unknown) => {
      throw contextualChatError(cause, "Chat event stream interrupted", response.url);
    });
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
    runStartedAt: dateToMilliseconds(stringValue(record.runStartedAt)),
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

async function readResponsePayload(response: Response, fallback: string, navigationID?: number): Promise<unknown> {
  const text = await measureNavigation(navigationID, "chat.body", () => response.text().catch((cause: unknown) => { throw contextualChatError(cause, `${fallback}: response body interrupted`, response.url); }));
  let payload: unknown;
  try {
    payload = measureNavigationSync(navigationID, "chat.json", () => text ? JSON.parse(text) : undefined);
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
    throw new Error(`${fallback} (HTTP ${response.status}${response.statusText ? ` ${response.statusText}` : ""}): ${error || bodyText.slice(0, 500) || "Server returned no error details"}`);
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

function blobToDataURL(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.addEventListener("error", () => reject(reader.error ?? new Error("Unable to read image attachment")));
    reader.addEventListener("load", () => typeof reader.result === "string" ? resolve(reader.result) : reject(new Error("Unable to encode image attachment")));
    reader.readAsDataURL(blob);
  });
}
