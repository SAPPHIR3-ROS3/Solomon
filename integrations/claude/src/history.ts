export type ToolCall = {
  id: string;
  name: string;
  arguments: string;
};

export type ChatMessage = {
  role: string;
  content?: unknown;
  tool_calls?: ToolCall[];
  tool_call_id?: string;
};

export type Produced = {
  content: string;
  tools: ToolCall[];
};

export type Action =
  | { type: "start"; system: string; prompt: string }
  | { type: "user"; prompt: string }
  | { type: "tools"; results: Array<{ id: string; output: string }> }
  | { type: "reject"; reason: string };

function text(content: unknown): string | null {
  if (typeof content === "string") return content;
  if (content == null) return "";
  if (!Array.isArray(content)) return null;
  const parts: string[] = [];
  for (const part of content) {
    if (!part || typeof part !== "object") return null;
    const record = part as { type?: string; text?: string };
    if (record.type === "text" && typeof record.text === "string") {
      parts.push(record.text);
      continue;
    }
    return null;
  }
  return parts.join("");
}

function canonical(message: ChatMessage): string {
  return JSON.stringify({
    role: message.role,
    content: text(message.content),
    tool_calls: (message.tool_calls || []).map((call) => ({ id: call.id, name: call.name, arguments: call.arguments })),
    tool_call_id: message.tool_call_id || "",
  });
}

function same(left: ChatMessage[], right: ChatMessage[]): boolean {
  if (left.length !== right.length) return false;
  return left.every((message, index) => canonical(message) === canonical(right[index]));
}

export function nextAction(accepted: ChatMessage[], produced: Produced | null, incoming: ChatMessage[]): Action {
  if (incoming.some((message) => text(message.content) == null)) {
    return { type: "reject", reason: "Claude sidecar only accepts text history; unsupported content is not imported" };
  }
  if (!produced) {
    if (accepted.length === 0) {
      const body = incoming.filter((message) => message.role !== "system");
      const system = incoming.filter((message) => message.role === "system").map((message) => text(message.content) || "").join("\n");
      if (body.length !== 1 || body[0].role !== "user") return { type: "start", system, prompt: transcript(body) };
      return { type: "start", system, prompt: text(body[0].content) || "" };
    }
    if (incoming.length !== accepted.length + 1 || !same(incoming.slice(0, accepted.length), accepted) || incoming[incoming.length - 1].role !== "user") {
      return { type: "reject", reason: "Claude sidecar history diverged from the live official session" };
    }
    return { type: "user", prompt: text(incoming[incoming.length - 1].content) || "" };
  }
  const assistant: ChatMessage = { role: "assistant", content: produced.content, tool_calls: produced.tools };
  if (produced.tools.length === 0) {
    const expected = accepted.concat([assistant]);
    if (incoming.length !== expected.length + 1 || !same(incoming.slice(0, expected.length), expected) || incoming[incoming.length - 1].role !== "user") {
      return { type: "reject", reason: "Claude sidecar history diverged from the live official session" };
    }
    return { type: "user", prompt: text(incoming[incoming.length - 1].content) || "" };
  }
  const results = incoming.slice(accepted.length + 1);
  const expectedResults = produced.tools.map((call, index) => results[index] && results[index].role === "tool" && results[index].tool_call_id === call.id);
  const head = incoming[accepted.length];
  if (!head || canonical(head) !== canonical(assistant) || results.length !== produced.tools.length || expectedResults.some((ok) => !ok)) {
    return { type: "reject", reason: "Claude sidecar history diverged from the live official session" };
  }
  return { type: "tools", results: results.map((message) => ({ id: message.tool_call_id || "", output: text(message.content) || "" })) };
}

// transcript replays a history no live session holds as one prompt, as the Cursor sidecar does
// for every request, so a restarted, edited or re-modelled chat continues instead of failing.
export function transcript(messages: ChatMessage[]): string {
  const turns = messages.map((message) => {
    if (message.role === "tool") return "[tool result " + (message.tool_call_id || "") + "]\n" + text(message.content);
    const calls = (message.tool_calls || []).map((call) => "<tool_call name=\"" + call.name + "\">" + call.arguments + "</tool_call>");
    return ["[" + message.role + "]", text(message.content) || "", ...calls].filter(Boolean).join("\n");
  });
  return "The conversation so far is replayed below as text. Continue from its last message, calling tools only through the tools you are given.\n\n" + turns.join("\n\n");
}

export function normalizeMessages(messages: ChatMessage[]): ChatMessage[] {
  return messages.map((message) => {
    const record = message as ChatMessage & { tool_calls?: Array<ToolCall & { function?: { name?: string; arguments?: string } }> };
    return {
      role: record.role,
      content: record.content,
      tool_call_id: record.tool_call_id,
      tool_calls: (record.tool_calls || []).map((call) => ({ id: call.id, name: call.name || call.function?.name || "", arguments: call.arguments || call.function?.arguments || "{}" })),
    };
  });
}
