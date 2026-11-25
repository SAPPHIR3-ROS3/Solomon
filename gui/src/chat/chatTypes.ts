/** Transcript model returned by the real daemon chat API. */
export type ChatSource = "daemon";

export type ChatImage = {
  name: string;
  url: string;
};

export type ChatStats = {
  turnAggregate?: boolean;
  reasoningEffort?: string;
  provider?: string;
  model?: string;
  contextTokens: number;
  contextEstimated?: boolean;
  /** -1 (or absent) means the provider did not report this counter. */
  cachedPromptTokens?: number;
  cacheCreationPromptTokens?: number;
  outputTokensPerSecond: number;
  promptTokensPerSecond: number;
  reasoningTokens: number;
  responseTokens: number;
  totalTokens: number;
  ttftSeconds: number;
  userTokens: number;
};

export type ChatTodoItem = {
  checked: boolean;
  text: string;
};

export type ChatToolResult = {
  count?: number;
  completed?: number;
  durationMs?: number;
  error?: string;
  items?: string[];
  jobId?: string;
  maxRounds?: number;
  output?: string;
  phase?: string;
  researchStatus?: "cancelled" | "done" | "failed" | "paused" | "running";
  round?: number;
  sdkCalls?: number;
  subchatStatus?: "cancelled" | "done" | "failed" | "interrupted" | "paused" | "queued" | "running";
  subchatId?: string;
  summary?: string;
  status: "error" | "success";
  title?: string;
  todoItems?: ChatTodoItem[];
  truncated?: boolean;
};

export type ChatToolParameter = {
  label: string;
  value: string;
};

export type ChatToolCall = {
  subagentDepth?: number;
  parentSubchatId?: string;
  checkpointBranch?: string;
  checkpointSeq?: number;
  defaultOpen?: boolean;
  delete?: boolean;
  full?: boolean;
  id: string;
  input?: string;
  intent?: string;
  mode?: "files" | "text";
  name: string;
  newString?: string;
  oldString?: string;
  parameters?: ChatToolParameter[];
  result?: ChatToolResult;
  renameTo?: string;
  status?: "error" | "interrupted" | "running" | "success";
  sync?: boolean;
  provider?: string;
  fastMode?: boolean;
  model?: string;
  reasoningEffort?: string;
};

export type ChatRetainedMessage = {
  content: string;
  images?: ChatImage[];
  role: "assistant" | "user";
};

export type ChatClip = {
  end: number;
  start: number;
  tag: string;
  text: string;
};

export type ChatFileChange = {
  path: string;
  added: number;
  removed: number;
  binary?: boolean;
  addedLines?: string[];
  removedLines?: string[];
};

export type ChatMessage = {
  fileChanges?: ChatFileChange[];
  checkpointBranch?: string;
  checkpointSeq?: number;
  clips?: ChatClip[];
  createdAt?: number;
  historyAvailable?: boolean;
  id: string;
  presentationID?: string;
  images?: ChatImage[];
  kind?: "compaction";
  role: "assistant" | "user";
  reasoning?: string;
  reasoningStartedAt?: number;
  stats?: ChatStats;
  status?: "interrupted";
  thoughtFor?: number;
  toolCalls?: ChatToolCall[];
  workedFor?: number;
  /** Browser-only timestamp used while the active agent turn is still running. */
  workStartedAt?: number;
  content: string;
  retainedMessages?: ChatRetainedMessage[];
  summary?: string;
};

export type RateLimitResumeSubagent = {
  subchatID: string;
  provider?: string;
  model?: string;
  reasoningEffort?: string;
  fastMode?: boolean;
};

export type RateLimitResumeTarget = {
  provider?: string;
  model?: string;
  reasoningEffort?: string;
  fastMode?: boolean;
  subagents?: RateLimitResumeSubagent[];
};

export type Chat = {
  /** Transport metadata kept on cached API responses. */
  source?: ChatSource;
  createdAt?: number;
  id: string;
  messages: ChatMessage[];
  modeSwitchTarget?: "agent";
  projectID?: string;
  /** Browser-only fallback for runs restored while the stream is already active. */
  runStartedAt?: number;
  knownCompactionIDs?: string[];
  rateLimitNotice?: string;
  rateLimitResetAt?: number;
  rateLimitScheduledAt?: number;
  rateLimitResume?: RateLimitResumeTarget;
  status?: "error" | "interrupted" | "running" | "success";
  title: string;
  workspaceID?: string;
  workspaceName?: string;
  workspacePath?: string;
  branch?: string;
  worktree?: string;
};
