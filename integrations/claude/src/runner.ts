import { randomUUID } from "node:crypto";
import { createSdkMcpServer, query, tool } from "@anthropic-ai/claude-agent-sdk";
import { z } from "zod";
import { nextAction, type Action, type ChatMessage, type ToolCall } from "./history.ts";
import type { Completion, ToolDefinition } from "./server.ts";

type Query = AsyncIterable<Record<string, any>> & { supportedModels(): Promise<Array<{ value: string; resolvedModel?: string; supportsEffort?: boolean; supportedEffortLevels?: string[] }>>; return(value?: unknown): Promise<unknown> };
type Event = { kind: "sdk"; message: Record<string, any> } | { kind: "tool"; call: ToolCall };

const maxSessions = 8;
const idleMs = 30 * 60 * 1000;
const admitTimeoutMs = 120 * 1000;

class PushStream<T> {
  private items: T[] = [];
  private waiters: Array<(result: IteratorResult<T>) => void> = [];
  private ended = false;
  push(value: T) {
    if (this.ended) return;
    const waiter = this.waiters.shift();
    if (waiter) waiter({ value, done: false });
    else this.items.push(value);
  }
  end() {
    this.ended = true;
    for (const waiter of this.waiters) waiter({ value: undefined as T, done: true });
    this.waiters = [];
  }
  next(): Promise<IteratorResult<T>> {
    const value = this.items.shift();
    if (value !== undefined) return Promise.resolve({ value, done: false });
    if (this.ended) return Promise.resolve({ value: undefined as T, done: true });
    return new Promise((resolve) => this.waiters.push(resolve));
  }
  [Symbol.asyncIterator]() { return this; }
  drainTools() {
    const calls: Array<{ kind: "tool"; call: { id: string; name: string; arguments: string } }> = [];
    const rest: Array<{ kind: "sdk"; message: Record<string, any> } | { kind: "tool"; call: { id: string; name: string; arguments: string } }> = [];
    for (const item of this.items) {
      if (item.kind === "tool") calls.push(item);
      else rest.push(item);
    }
    this.items = rest;
    return calls;
  }
}

function childEnv(): Record<string, string | undefined> {
  for (const key of ["ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_BASE_URL"]) {
    if (process.env[key]) throw new Error("Unset " + key + " before using the Claude subscription sidecar");
  }
  const env: Record<string, string | undefined> = { ...process.env, CLAUDE_AGENT_SDK_CLIENT_APP: "solomon-sidecar" };
  delete env.SOLOMON_CLAUDE_TOKEN;
  return env;
}

function schemaFor(parameters: unknown) {
  const record = parameters && typeof parameters === "object" ? parameters as { properties?: Record<string, { type?: string }>; required?: string[] } : {};
  const shape: Record<string, z.ZodType> = {};
  for (const [key, value] of Object.entries(record.properties || {})) {
    let field: z.ZodType = value?.type === "string" ? z.string() : value?.type === "number" || value?.type === "integer" ? z.number() : value?.type === "boolean" ? z.boolean() : z.unknown();
    if (!(record.required || []).includes(key)) field = field.optional();
    shape[key] = field;
  }
  return z.object(shape).passthrough();
}

function options(executable: string, model: string, system: string, abort: AbortController, effort?: string) {
  return {
    model: model || undefined,
    effort: ["low", "medium", "high", "xhigh", "max"].includes(effort ?? "") ? effort as "low" | "medium" | "high" | "xhigh" | "max" : undefined,
    cwd: process.env.SOLOMON_CLAUDE_CWD || process.cwd(),
    abortController: abort,
    tools: [] as string[],
    skills: [] as string[],
    settingSources: [] as Array<"user" | "project" | "local">,
    permissionMode: "default" as const,
    persistSession: false,
    pathToClaudeCodeExecutable: executable || undefined,
    systemPrompt: system ? { type: "preset" as const, preset: "claude_code" as const, append: system } : { type: "preset" as const, preset: "claude_code" as const },
    env: childEnv(),
    disallowedTools: ["Bash", "Read", "Edit", "Write", "Grep", "Glob", "WebFetch", "WebSearch", "Task", "Skill", "NotebookEdit"],
    canUseTool: async (name: string) => name.startsWith("mcp__solomon__") ? { behavior: "allow" as const } : { behavior: "deny" as const, message: "Solomon executes this tool" },
  };
}

function user(prompt: string) { return { type: "user", message: { role: "user", content: prompt }, parent_tool_use_id: null }; }

// LiveSession is one official SDK conversation. Like the Cursor sidecar's per-request agents,
// sessions share no state, so title generation or a subagent never disturbs a chat that is
// waiting on Solomon's tool results.
export type PoolSession = {
  readonly model: string;
  readonly effort?: string;
  accepted: ChatMessage[];
  produced: { content: string; tools: ToolCall[] } | null;
  busy: boolean;
  used: number;
  readonly awaitingTools: boolean;
  run(action: Action, tools: ToolDefinition[], onDelta: (text: string) => void, signal: AbortSignal): Promise<Completion>;
  close(): Promise<void>;
};

class LiveSession implements PoolSession {
  readonly model: string;
  readonly effort?: string;
  accepted: ChatMessage[] = [];
  produced: { content: string; tools: ToolCall[] } | null = null;
  busy = false;
  used = Date.now();
  private executable: string;
  private live: Query | null = null;
  private input = new PushStream<Record<string, unknown>>();
  private abort = new AbortController();
  private events = new PushStream<Event>();
  private pending = new Map<string, (output: string) => void>();
  constructor(executable: string, model: string, effort?: string) {
    this.effort = effort;
    this.executable = executable;
    this.model = model;
  }
  // awaitingTools is true while SDK tool callbacks wait for Solomon's results; closing the
  // session then would answer them with empty output and lose the round.
  get awaitingTools() { return this.pending.size > 0; }
  async run(action: Action, tools: ToolDefinition[], onDelta: (text: string) => void, signal: AbortSignal): Promise<Completion> {
    if (action.type === "reject") throw new Error(action.reason);
    if (action.type === "start") this.start(action.system, action.prompt, tools);
    if (action.type === "user") this.input.push(user(action.prompt));
    if (action.type === "tools") {
      for (const result of action.results) {
        const resolve = this.pending.get(result.id);
        if (!resolve) throw new Error("Claude sidecar has no matching live tool call");
        this.pending.delete(result.id);
        resolve(result.output);
      }
    }
    return this.read(onDelta, signal);
  }
  private start(system: string, prompt: string, tools: ToolDefinition[]) {
    const definitions = tools.flatMap((entry) => {
      const name = entry.function?.name || "";
      if (!name) return [];
      return [tool(name, entry.function?.description || name, schemaFor(entry.function?.parameters), async (args) => {
        const id = randomUUID();
        const output = await new Promise<string>((resolve) => {
          this.pending.set(id, resolve);
          this.events.push({ kind: "tool", call: { id, name, arguments: JSON.stringify(args || {}) } });
        });
        return { content: [{ type: "text" as const, text: output }] };
      })];
    });
    const live = query({ prompt: this.input as AsyncIterable<any>, options: { ...options(this.executable, this.model, system, this.abort, this.effort), mcpServers: { solomon: createSdkMcpServer({ name: "solomon", tools: definitions }) }, allowedTools: definitions.map((entry) => "mcp__solomon__" + entry.name) } }) as Query;
    this.live = live;
    void this.pump(live);
    this.input.push(user(prompt));
  }
  private async pump(live: Query) {
    try {
      for await (const message of live) this.events.push({ kind: "sdk", message: message as Record<string, any> });
    } finally {
      this.events.end();
    }
  }
  private async read(onDelta: (text: string) => void, signal: AbortSignal): Promise<Completion> {
    let content = "";
    const calls: ToolCall[] = [];
    while (true) {
      if (signal.aborted) throw new Error("Claude sidecar request cancelled");
      const event = await this.events.next();
      if (event.done) throw new Error("Claude sidecar session ended before a response");
      if (event.value.kind === "tool") {
        calls.push(event.value.call);
        await new Promise((resolve) => setTimeout(resolve, 100));
        for (const extra of this.events.drainTools()) calls.push(extra.call);
        return { content, toolCalls: calls, finishReason: "tool_calls", usage: { promptTokens: 0, completionTokens: 0 } };
      }
      const message = event.value.message;
      if (message.type === "assistant") {
        for (const block of message.message?.content || []) if (block.type === "text" && block.text) { content += block.text; onDelta(block.text); }
      }
      if (message.type === "result") {
        if (message.is_error || (message.subtype && message.subtype !== "success")) {
          const reason = String(message.result || "Claude sidecar request failed");
          throw new Error(/not logged in/i.test(reason) ? "Claude Sub is not logged in: run /connect and choose Claude Sub to sign in with the Claude CLI" : reason);
        }
        return { content, toolCalls: [], finishReason: message.subtype === "success" && message.stop_reason === "max_tokens" ? "length" : "stop", usage: { promptTokens: Number(message.usage?.input_tokens || 0), completionTokens: Number(message.usage?.output_tokens || 0) } };
      }
    }
  }
  async close() {
    for (const resolve of this.pending.values()) resolve("");
    this.pending.clear();
    this.abort.abort();
    this.input.end();
    if (this.live) await this.live.return().catch(() => undefined);
  }
}

export type RunnerOptions = {
  createSession?: (model: string, effort?: string) => PoolSession;
  admitTimeoutMs?: number;
};

export class ClaudeRunner {
  private executable: string;
  private sessions: PoolSession[] = [];
  private modelNames: string[] | null = null;
 private modelEntries: Array<{ id: string; supported_reasoning_levels?: string[] }> = [];
  private createSession: (model: string, effort?: string) => PoolSession;
  private admitTimeoutMs: number;
  private waiters: Array<() => void> = [];
  constructor(executable: string, opts: RunnerOptions = {}) {
    this.executable = executable;
    this.createSession = opts.createSession || ((model, effort) => new LiveSession(executable, model, effort));
    this.admitTimeoutMs = opts.admitTimeoutMs ?? admitTimeoutMs;
  }
  async models(): Promise<string[]> {
    if (this.modelNames) return this.modelNames;
    const input = new PushStream<Record<string, unknown>>();
    const abort = new AbortController();
    const live = query({ prompt: input as AsyncIterable<any>, options: options(this.executable, "", "", abort) }) as Query;
    try {
      const models = await Promise.race([live.supportedModels(), new Promise<never>((_, reject) => setTimeout(() => reject(new Error("Claude model discovery timed out")), 30000))]);
      // The newest models are listed only as aliases ("opus", "sonnet"); their concrete IDs live in resolvedModel.
      this.modelEntries = models.map((model) => ({ id: model.resolvedModel || model.value, supported_reasoning_levels: model.supportedEffortLevels ?? (model.supportsEffort === false ? [] : undefined) }));
      this.modelNames = [...new Set(models.map((model) => model.resolvedModel || model.value).filter(Boolean))];
      return this.modelNames;
    } finally {
      abort.abort();
      input.end();
      await live.return();
    }
  }
  async modelCatalog() {
    await this.models();
    return this.modelEntries;
  }
  async complete(input: { model: string; messages: ChatMessage[]; tools: ToolDefinition[]; reasoningEffort?: string }, onDelta: (text: string) => void, signal: AbortSignal): Promise<Completion> {
    const deadline = Date.now() + this.admitTimeoutMs;
    let claim = this.claim(input);
    while (!claim) {
      if (signal.aborted) throw new Error("Claude sidecar request cancelled");
      if (Date.now() >= deadline) throw new Error("Claude sidecar is at capacity: " + maxSessions + " conversations are running or waiting on tool results");
      await this.changed(deadline, signal);
      claim = this.claim(input);
    }
    const { session, action } = claim;
    try {
      const completion = await session.run(action, input.tools, onDelta, signal);
      session.accepted = input.messages;
      session.produced = { content: completion.content, tools: completion.toolCalls };
      return completion;
    } catch (error) {
      await this.drop(session);
      throw error;
    } finally {
      session.busy = false;
      session.used = Date.now();
      this.notify();
    }
  }
  async close() {
    await Promise.all(this.sessions.map((session) => this.drop(session)));
  }
  // claim matches and reserves a session synchronously, so concurrent requests never share one
  // and the pool never exceeds maxSessions. It reuses the session whose history the request
  // extends before evicting anything, and only evicts sessions that are neither running nor
  // waiting on tool results. It returns null when the request has to wait for a free slot.
  private claim(input: { model: string; messages: ChatMessage[]; reasoningEffort?: string }): { session: PoolSession; action: Action } | null {
    const now = Date.now();
    for (const session of this.sessions.filter((entry) => !entry.busy && !entry.awaitingTools && now - entry.used > idleMs)) void this.drop(session);
    for (const session of this.sessions) {
      if (session.busy || session.model !== input.model || session.effort !== input.reasoningEffort) continue;
      const action = nextAction(session.accepted, session.produced, input.messages);
      if (action.type !== "reject") {
        session.busy = true;
        return { session, action };
      }
    }
    const action = nextAction([], null, input.messages);
    if (action.type === "reject") throw new Error(action.reason);
    if (this.sessions.length >= maxSessions) {
      const victim = this.sessions.filter((entry) => !entry.busy && !entry.awaitingTools).sort((left, right) => left.used - right.used)[0];
      if (!victim) return null;
      void this.drop(victim);
    }
    const session = this.createSession(input.model, input.reasoningEffort);
    session.busy = true;
    this.sessions.push(session);
    return { session, action };
  }
  // Resolves on a pool change, the deadline, or cancellation, whichever comes first.
  private changed(deadline: number, signal: AbortSignal) {
    return new Promise<void>((resolve) => {
      const wake = () => {
        clearTimeout(timer);
        signal.removeEventListener("abort", wake);
        this.waiters = this.waiters.filter((waiter) => waiter !== wake);
        resolve();
      };
      const timer = setTimeout(wake, Math.max(0, deadline - Date.now()));
      signal.addEventListener("abort", wake, { once: true });
      this.waiters.push(wake);
    });
  }
  private notify() {
    const waiters = this.waiters;
    this.waiters = [];
    for (const wake of waiters) wake();
  }
  // drop removes the session synchronously, so callers can free its slot without awaiting the CLI.
  private async drop(session: PoolSession) {
    if (!this.sessions.includes(session)) return;
    this.sessions = this.sessions.filter((entry) => entry !== session);
    this.notify();
    await session.close();
  }
}
