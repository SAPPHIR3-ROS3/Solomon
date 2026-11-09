import type { Chat, ChatImage, ChatMessage, ChatStats, ChatTodoItem, ChatToolCall, ChatToolResult } from "./chatTypes";
import type { ChatStreamEvent } from "./chatClient";

export function messageFromPayload(payload: unknown, index: number, imageOrigin: string): ChatMessage | null {
  const record = objectValue(payload);
  const role = stringValue(record.role);
  if (role !== "user" && role !== "assistant") return null;
  // The server marks command lines it keeps out of the GUI; a skill request stays visible.
  if (record.guiHidden === true) return null;
  const message: ChatMessage = {
    checkpointBranch: stringValue(record.checkpointBranch) || undefined,
    checkpointSeq: numberValue(record.checkpointSeq, undefined),
    content: stringValue(record.content),
    createdAt: dateToMilliseconds(stringValue(record.createdAt)),
    id: stringValue(record.id) || `m-${index}`,
    images: imagesFromPayload(record.images, imageOrigin),
    role,
  };
  if (role === "assistant" && record.fileChangesRecorded === true) message.fileChanges = [];
  if (role === "assistant" && Array.isArray(record.fileChanges)) {
    message.fileChanges = record.fileChanges.flatMap(item => {
      const change = objectValue(item);
      if (typeof change.path !== "string" || !change.path || typeof change.added !== "number" || typeof change.removed !== "number") return [];
      return [{ path: change.path, added: Math.max(0, change.added), removed: Math.max(0, change.removed), binary: change.binary === true,
        addedLines: Array.isArray(change.addedLines) ? change.addedLines.filter((line): line is string => typeof line === "string") : [],
        removedLines: Array.isArray(change.removedLines) ? change.removedLines.filter((line): line is string => typeof line === "string") : [] }];
    });
  }
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

export function toolCallFromPayload(payload: unknown, index: number, _imageOrigin = ""): ChatToolCall {
  const record = objectValue(payload);
  const args = objectValue(record.arguments);
  const result = record.result ? normalizeToolResult(record.result, stringValue(record.error)) : undefined;
  const name = stringValue(record.name) || "tool";
  return {
    checkpointBranch: stringValue(record.checkpointBranch ?? record.checkpoint_branch) || undefined,
    checkpointSeq: numberValue(record.checkpointSeq ?? record.checkpoint_seq, undefined),
    defaultOpen: typeof record.defaultOpen === "boolean" ? record.defaultOpen : undefined,
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
    model: stringValue(record.model) || stringValue(args.model) || undefined,
    reasoningEffort: stringValue(record.reasoningEffort) || stringValue(args.reasoning_effort) || undefined,
    provider: stringValue(record.provider) || undefined,
    fastMode: typeof record.fastMode === "boolean" ? record.fastMode : undefined,
    sync: subagentSyncFromPayload(name, record, args),
  };
}

export function normalizeToolResult(payload: unknown, error = ""): ChatToolResult {
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
  const subchatStatus = record.subagentStatus ?? record.subchatStatus ?? (result.subchatId ? record.status : undefined);
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
    turnAggregate: record.turnAggregate === true,
    reasoningEffort: stringValue(record.reasoningEffort) || undefined,
    provider: stringValue(record.provider) || undefined,
    model: stringValue(record.model) || undefined,
    contextTokens: numberValue(record.contextTokens, 0) ?? 0,
    cachedPromptTokens: numberValue(record.cachedPromptTokens, -1) ?? -1,
    cacheCreationPromptTokens: numberValue(record.cacheCreationPromptTokens, -1) ?? -1,
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

export function firstString(record: Record<string, unknown>, keys: string[]): string {
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

export function objectValue(value: unknown): Record<string, unknown> {
  return value && typeof value === "object" && !Array.isArray(value) ? value as Record<string, unknown> : {};
}

export function arrayValue(value: unknown): unknown[] {
  return Array.isArray(value) ? value : [];
}

export function stringValue(value: unknown): string {
  return typeof value === "string" ? value : "";
}

export function subchatStatusFromPayload(value: unknown): ChatToolResult["subchatStatus"] | undefined {
  if (typeof value !== "string" || !["cancelled", "done", "paused", "queued", "running"].includes(value)) return undefined;
  return value as ChatToolResult["subchatStatus"];
}

export function chatStatusFromPayload(value: unknown): Chat["status"] | undefined {
  if (typeof value !== "string" || !["error", "interrupted", "running", "success"].includes(value)) return undefined;
  return value as Chat["status"];
}

export function numberValue(value: unknown, fallback: number | undefined): number | undefined {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

export function dateToMilliseconds(value: string): number | undefined {
  if (!value) return undefined;
  const parsed = Date.parse(value);
  return Number.isNaN(parsed) ? undefined : parsed;
}

export function eventTimeMilliseconds(event: ChatStreamEvent): number {
  return dateToMilliseconds(stringValue(event.ts)) ?? Date.now();
}

function absoluteAssetURL(value: string, origin: string): string {
  if (!origin || value.startsWith("data:") || value.startsWith("blob:") || value.startsWith("http://") || value.startsWith("https://")) return value;
  if (value.startsWith("/")) return `${origin}${value}`;
  return value;
}
