import { randomUUID } from "node:crypto";
import { createSdkMcpServer, query, tool } from "@anthropic-ai/claude-agent-sdk";
import { z } from "zod";
import { nextAction, type ChatMessage, type ToolCall } from "./history.ts";
import type { Completion, ToolDefinition } from "./server.ts";

type Session = {
  query: AsyncIterable<Record<string, any>> & { supportedModels(): Promise<Array<{ value: string }>>; return(value?: unknown): Promise<unknown> };
  input: PushStream<Record<string, unknown>>;
  abort: AbortController;
};

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

function fresh(messages: ChatMessage[]): boolean {
  const body = messages.filter((message) => message.role !== "system");
  return body.length === 1 && body[0].role === "user";
}

export class ClaudeRunner {
  private executable: string;
  private session: Session | null = null;
  private accepted: ChatMessage[] = [];
  private produced: { content: string; tools: ToolCall[] } | null = null;
  private events = new PushStream<{ kind: "sdk"; message: Record<string, any> } | { kind: "tool"; call: ToolCall }>();
  private pending = new Map<string, (output: string) => void>();
  private modelNames: string[] | null = null;
  constructor(executable: string) { this.executable = executable; }
  async models(): Promise<string[]> {
    if (this.modelNames) return this.modelNames;
    const input = new PushStream<Record<string, unknown>>();
    const abort = new AbortController();
    const live = query({ prompt: input as AsyncIterable<any>, options: this.options("", "", abort) }) as Session["query"];
    try {
      const models = await Promise.race([live.supportedModels(), new Promise<never>((_, reject) => setTimeout(() => reject(new Error("Claude model discovery timed out")), 30000))]);
      this.modelNames = models.map((model) => model.value).filter(Boolean);
      return this.modelNames;
    } finally {
      abort.abort();
      input.end();
      await live.return();
    }
  }
  async complete(input: { model: string; messages: ChatMessage[]; tools: ToolDefinition[] }, onDelta: (text: string) => void, signal: AbortSignal): Promise<Completion> {
    let action = nextAction(this.accepted, this.produced, input.messages);
    if (action.type === "reject" && fresh(input.messages)) {
      await this.close();
      action = nextAction(this.accepted, this.produced, input.messages);
    }
    if (action.type === "reject") throw new Error(action.reason);
    if (action.type === "start") await this.start(input.model, action.system, action.prompt, input.tools);
    if (action.type === "user") this.session?.input.push(this.user(action.prompt));
    if (action.type === "tools") {
      for (const result of action.results) {
        const resolve = this.pending.get(result.id);
        if (!resolve) throw new Error("Claude sidecar has no matching live tool call");
        this.pending.delete(result.id);
        resolve(result.output);
      }
    }
    const completion = await this.read(onDelta, signal);
    this.accepted = input.messages;
    this.produced = { content: completion.content, tools: completion.toolCalls };
    return completion;
  }
  private async start(model: string, system: string, prompt: string, tools: ToolDefinition[]) {
    await this.close();
    const abort = new AbortController();
    const input = new PushStream<Record<string, unknown>>();
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
    const live = query({ prompt: input as AsyncIterable<any>, options: { ...this.options(model, system, abort), mcpServers: { solomon: createSdkMcpServer({ name: "solomon", tools: definitions }) }, allowedTools: definitions.map((entry) => "mcp__solomon__" + entry.name) } }) as Session["query"];
    this.session = { query: live, input, abort };
    void this.pump(live, this.events);
    input.push(this.user(prompt));
  }
  private async pump(live: Session["query"], events: PushStream<{ kind: "sdk"; message: Record<string, any> } | { kind: "tool"; call: ToolCall }>) {
    try {
      for await (const message of live) events.push({ kind: "sdk", message: message as Record<string, any> });
    } finally {
      events.end();
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
        if (message.is_error || (message.subtype && message.subtype !== "success")) throw new Error(message.result || "Claude sidecar request failed");
        return { content, toolCalls: [], finishReason: message.subtype === "success" && message.stop_reason === "max_tokens" ? "length" : "stop", usage: { promptTokens: Number(message.usage?.input_tokens || 0), completionTokens: Number(message.usage?.output_tokens || 0) } };
      }
    }
  }
  private options(model: string, system: string, abort: AbortController) {
    return {
      model: model || undefined,
      cwd: process.env.SOLOMON_CLAUDE_CWD || process.cwd(),
      abortController: abort,
      tools: [] as string[],
      skills: [] as string[],
      settingSources: [] as Array<"user" | "project" | "local">,
      permissionMode: "default" as const,
      persistSession: false,
      pathToClaudeCodeExecutable: this.executable || undefined,
      systemPrompt: system ? { type: "preset" as const, preset: "claude_code" as const, append: system } : { type: "preset" as const, preset: "claude_code" as const },
      env: childEnv(),
      disallowedTools: ["Bash", "Read", "Edit", "Write", "Grep", "Glob", "WebFetch", "WebSearch", "Task", "Skill", "NotebookEdit"],
      canUseTool: async (name: string) => name.startsWith("mcp__solomon__") ? { behavior: "allow" as const } : { behavior: "deny" as const, message: "Solomon executes this tool" },
    };
  }
  private user(prompt: string) { return { type: "user", message: { role: "user", content: prompt }, parent_tool_use_id: null }; }
  private async close() {
    const session = this.session;
    this.session = null;
    this.accepted = [];
    this.produced = null;
    for (const resolve of this.pending.values()) resolve("");
    this.pending.clear();
    if (!session) return;
    session.abort.abort();
    session.input.end();
    await session.query.return();
    this.events = new PushStream();
  }
}
