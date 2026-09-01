/** Transcript model returned by the real daemon chat API. */
export type ChatSource = "daemon";

export type ChatImage = {
  name: string;
  url: string;
};

export type ChatStats = {
  contextTokens: number;
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
  subchatStatus?: "cancelled" | "done" | "paused" | "queued" | "running";
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
};

export type ChatRetainedMessage = {
  content: string;
  images?: ChatImage[];
  role: "assistant" | "user";
};

export type ChatMessage = {
  checkpointBranch?: string;
  checkpointSeq?: number;
  createdAt?: number;
  id: string;
  images?: ChatImage[];
  kind?: "compaction";
  role: "assistant" | "user";
  reasoning?: string;
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
  status?: "error" | "interrupted" | "running" | "success";
  title: string;
  workspaceID?: string;
  workspaceName?: string;
  workspacePath?: string;
  branch?: string;
  worktree?: string;
};
