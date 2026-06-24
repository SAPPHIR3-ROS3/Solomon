import type { SDKImage, SDKUserMessage } from "@cursor/sdk";
import type { ChatCompletionTool, ChatMessage, ChatToolCall, ContentPart } from "./openai-types.js";
import { harnessPreamble } from "./harness-prompt.js";
import { escapeXmlAttr, escapeXmlTextStrict } from "./xml-utils.js";
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

export function withHarnessPreamble(
  prompt: string | SDKUserMessage,
  tools?: ChatCompletionTool[],
): string | SDKUserMessage {
  const prefix = harnessPreamble(tools);
  if (typeof prompt === "string") {
    return prefix + prompt;
  }
  return { ...prompt, text: prefix + prompt.text };
}

export function sanitizeReflectedText(s: string): string {
  return escapeXmlTextStrict(stripUnsafeControlChars(s)).slice(0, 4096);
}

export function stripUnsafeControlChars(s: string): string {
  let out = "";
  for (const ch of s) {
    const code = ch.charCodeAt(0);
    if (code === 9 || code === 10 || code === 13 || (code >= 32 && code !== 127)) {
      out += ch;
    }
  }
  return out;
}

const DEFAULT_MODEL_ID = "composer-2.5";
const MODEL_ID_RE = /^[a-zA-Z0-9][a-zA-Z0-9._-]*$/;
const TOOL_NAME_RE = /^[a-zA-Z_][a-zA-Z0-9_-]*$/;

export function sanitizeModelId(v: string | undefined): string {
  const s = (v ?? DEFAULT_MODEL_ID).trim();
  if (!s || s.length > 256 || !MODEL_ID_RE.test(s)) {
    return DEFAULT_MODEL_ID;
  }
  return s;
}

export function isSafeToolName(name: string): boolean {
  const s = name.trim();
  return s.length > 0 && s.length <= 128 && TOOL_NAME_RE.test(s);
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
    parts.push(`<args>${escapeXmlTextStrict(args)}</args>`);
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

export function buildPromptFromMessages(
  messages: ChatMessage[],
  tools?: ChatCompletionTool[],
): string | SDKUserMessage {
  if (messages.length === 1 && messages[0].role === "user") {
    return withHarnessPreamble(messageToUserPayload(messages[0]), tools);
  }
  const lines: string[] = [];
  for (const m of messages) {
    lines.push(formatChatMessage(m));
  }
  return withHarnessPreamble(lines.join("\n\n"), tools);
}
