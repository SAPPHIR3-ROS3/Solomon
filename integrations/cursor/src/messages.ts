import type { SDKImage, SDKUserMessage } from "@cursor/sdk";
import type { ChatMessage, ContentPart } from "./openai-types.js";
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

const SOLOMON_HARNESS_PREFIX =
  "[Solomon harness] Do not use Cursor built-in tools (Read, Shell, Grep, Write, etc.). " +
  "You cannot access the filesystem from this agent. " +
  "When you need an action, emit exactly one <tool_calls> XML block with Solomon tools only " +
  "(readFile, shell, editFile) in the visible assistant response, not in reasoning or thinking. " +
  "Reasoning may discuss tool format; only response-body XML is parsed and executed. " +
  "Do not summarize or quote file contents unless they appear in a prior [tool result] line from Solomon.\n\n";

export function withSolomonHarnessPrefix(
  prompt: string | SDKUserMessage,
): string | SDKUserMessage {
  if (typeof prompt === "string") {
    return SOLOMON_HARNESS_PREFIX + prompt;
  }
  return { ...prompt, text: SOLOMON_HARNESS_PREFIX + prompt.text };
}

export function formatDeltaMessage(m: ChatMessage): string {
  switch (m.role) {
    case "tool":
      return `[tool result ${m.tool_call_id ?? ""}]\n${messageToPromptText(m)}`;
    case "assistant":
      return `[assistant]\n${messageToPromptText(m)}`;
    case "user":
      return messageToPromptText(m);
    default:
      return `[${m.role}]\n${messageToPromptText(m)}`;
  }
}
