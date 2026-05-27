import type { SDKImage, SDKUserMessage } from "@cursor/sdk";
import type { ChatMessage, ChatToolCall, ContentPart } from "./openai-types.js";
import * as fs from "node:fs";
import * as path from "node:path";
import { fileURLToPath } from "node:url";

export function messageToPromptText(m: ChatMessage): string {
  if (typeof m.content === "string") {
    return m.content;
  }
  if (!Array.isArray(m.content)) {
    return "";
  }
  const texts: string[] = [];
  for (const p of m.content) {
    if (p.type === "text" && p.text) {
      texts.push(p.text);
    }
  }
  return texts.join("\n");
}

export function messageToUserPayload(m: ChatMessage): SDKUserMessage {
  const text = messageToPromptText(m);
  const images = extractImages(m);
  if (images.length > 0) {
    return { text, images };
  }
  return { text };
}

function extractImages(m: ChatMessage): SDKImage[] {
  if (!Array.isArray(m.content)) {
    return [];
  }
  const out: SDKImage[] = [];
  for (const p of m.content as ContentPart[]) {
    if (p.type !== "image_url" || !p.image_url?.url) {
      continue;
    }
    const url = p.image_url.url;
    if (url.startsWith("data:")) {
      const match = /^data:([^;]+);base64,(.+)$/i.exec(url);
      if (match) {
        out.push({ data: match[2], mimeType: match[1] });
      }
      continue;
    }
    if (url.startsWith("file://")) {
      try {
        const filePath = fileURLToPath(url);
        const data = fs.readFileSync(filePath).toString("base64");
        const mime = guessMime(filePath);
        out.push({ data, mimeType: mime });
      } catch {
        out.push({ url });
      }
      continue;
    }
    out.push({ url });
  }
  return out;
}

function guessMime(filePath: string): string {
  const ext = path.extname(filePath).toLowerCase();
  switch (ext) {
    case ".png":
      return "image/png";
    case ".jpg":
    case ".jpeg":
      return "image/jpeg";
    case ".gif":
      return "image/gif";
    case ".webp":
      return "image/webp";
    default:
      return "application/octet-stream";
  }
}

export function roughTokFromString(s: string): number {
  const n = [...s].length;
  if (n <= 0) {
    return 0;
  }
  return Math.floor((n + 2) / 3);
}

export function roughTokFromMessages(messages: ChatMessage[]): number {
  let sum = 0;
  for (const m of messages) {
    sum += roughTokFromString(messageToPromptText(m));
  }
  return sum;
}

const HARNESS_MARKER = "[Harness]";

const HARNESS_CLAUSES: string[] = [
  `${HARNESS_MARKER} Interaction mode: this is not a normal Cursor IDE agent session. You are behind a remote host harness proxy. Cursor built-in tools are unavailable (Read, Write, Edit, Shell, Grep, Glob, rg, SemanticSearch, Task, browser tools, etc.). You cannot access the workspace except through the harness.`,
  `${HARNESS_MARKER} Results: the host executes tools and returns output as [tool result …] lines in later turns. Do not invent or quote file contents unless they appeared in a prior tool result or the user's message.`,
  `${HARNESS_MARKER} Invocation transport: emit exactly one <tool_calls> XML block in the visible assistant response body when you need an action (not in reasoning/thinking). SDK-native tool_use / tool_call events from this stack are bridged when mappable; prefer explicit XML with harness tool names.`,
  `${HARNESS_MARKER} Tool names: use only names listed under ## Available tools in the system message (e.g. readFile, shell, editFile). Map inspection → readFile, terminal commands → shell, file edits → editFile.`,
  `${HARNESS_MARKER} XML shape: <tool_calls><tool name="TOOL"><intent>brief purpose when supported</intent><args>{"key":"value"}</args></tool></tool_calls> — one block per reply that invokes tools; valid JSON in each <args>; optional prose before the block; no text after </tool_calls>.`,
];

export function harnessPreamble(): string {
  return HARNESS_CLAUSES.join("\n\n") + "\n\n";
}

export function withHarnessPreamble(
  prompt: string | SDKUserMessage,
): string | SDKUserMessage {
  const prefix = harnessPreamble();
  if (typeof prompt === "string") {
    return prefix + prompt;
  }
  return { ...prompt, text: prefix + prompt.text };
}

/** @deprecated use withHarnessPreamble */
export const withSolomonHarnessPrefix = withHarnessPreamble;

function escapeXmlAttr(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/"/g, "&quot;").replace(/</g, "&lt;");
}

function formatAssistantToolCalls(toolCalls: ChatToolCall[]): string {
  const parts: string[] = ["<tool_calls>"];
  for (const tc of toolCalls) {
    const name = tc.function?.name?.trim() ?? "";
    if (!name) {
      continue;
    }
    const args = tc.function?.arguments?.trim() || "{}";
    parts.push(`<tool name="${escapeXmlAttr(name)}">`);
    parts.push(`<args>${args}</args>`);
    parts.push("</tool>");
  }
  parts.push("</tool_calls>");
  return parts.join("\n");
}

export function formatChatMessage(m: ChatMessage): string {
  switch (m.role) {
    case "tool":
      return `[tool result ${m.tool_call_id ?? ""}]\n${messageToPromptText(m)}`;
    case "assistant": {
      const text = messageToPromptText(m).trim();
      const tools =
        m.tool_calls && m.tool_calls.length > 0 ? formatAssistantToolCalls(m.tool_calls) : "";
      if (text && tools) {
        return `[assistant]\n${text}\n\n${tools}`;
      }
      if (tools) {
        return `[assistant]\n${tools}`;
      }
      return `[assistant]\n${text}`;
    }
    case "user":
      return `[user]\n${messageToPromptText(m)}`;
    default:
      return `[${m.role}]\n${messageToPromptText(m)}`;
  }
}

/** @deprecated use formatChatMessage */
export const formatDeltaMessage = formatChatMessage;

export function buildPromptFromMessages(messages: ChatMessage[]): string | SDKUserMessage {
  if (messages.length === 1 && messages[0].role === "user") {
    return withHarnessPreamble(messageToUserPayload(messages[0]));
  }
  const lines: string[] = [];
  for (const m of messages) {
    lines.push(formatChatMessage(m));
  }
  return withHarnessPreamble(lines.join("\n\n"));
}
