import type { SDKAgent } from "@cursor/sdk";
import type { ChatMessage } from "./openai-types.js";

export type SessionState = {
  agent: SDKAgent;
  syncedMessages: number;
  model: string;
};

const sessions = new Map<string, SessionState>();

export function sessionKey(messages: ChatMessage[], cwd: string): string {
  const firstUser = messages.find((m) => m.role === "user");
  const seed =
    cwd +
    "|" +
    (typeof firstUser?.content === "string"
      ? firstUser.content.slice(0, 256)
      : JSON.stringify(firstUser?.content ?? "").slice(0, 256));
  let h = 0;
  for (let i = 0; i < seed.length; i++) {
    h = (Math.imul(31, h) + seed.charCodeAt(i)) | 0;
  }
  return "s" + Math.abs(h).toString(36);
}

export function getSession(key: string): SessionState | undefined {
  return sessions.get(key);
}

export function setSession(key: string, state: SessionState): void {
  sessions.set(key, state);
}

export function deleteSession(key: string): void {
  sessions.delete(key);
}
