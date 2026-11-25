import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type { ChatMessage } from "./chatTypes";
import { chatPresentationKey } from "./chatPresentationKey";

const TOKEN_INTERVAL_MS = 60;
const segmenter = new Intl.Segmenter(undefined, { granularity: "grapheme" });

// Provider deltas may already contain many tokens. Presentation uses small
// Unicode-safe tokens, independently of network chunk sizes and frame delays.
export function nextVisualTokenEnd(source: string, start: number): number {
  let end = start;
  let count = 0;
  for (const { segment } of segmenter.segment(source.slice(start))) {
    end += segment.length;
    if (++count >= 4 || (count > 1 && /\s/u.test(segment))) break;
  }
  return end;
}

type Response = { source: string; shown: string };

export function usePacedChatMessages(key: string, messages: ChatMessage[], enabled: boolean) {
  const [, redraw] = useState(0);
  const [session] = useState(() => ({
    key, enabled, messages,
    responses: new Map<string, Response>(messages.filter(message => message.role === "assistant").map(message => [chatPresentationKey(message), { source: message.content, shown: message.content }])),
    frame: 0, nextAt: 0, activeID: undefined as string | undefined,
  }));
  const state = useRef(session);

  function cancel() {
    cancelAnimationFrame(state.current.frame);
    state.current.frame = 0;
  }
  function revealAll() {
    cancel();
    for (const response of state.current.responses.values()) response.shown = response.source;
    redraw(value => value + 1);
  }
  function step(now: number) {
    const current = state.current;
    current.frame = 0;
    if (document.hidden) { revealAll(); return; }
    const next = current.messages.find(message => {
      const response = current.responses.get(chatPresentationKey(message));
      return response && response.shown.length < response.source.length;
    });
    if (!next) return;
    if (now >= current.nextAt) {
      current.activeID = chatPresentationKey(next);
      const response = current.responses.get(current.activeID)!;
      response.shown = response.source.slice(0, nextVisualTokenEnd(response.source, response.shown.length));
      // Never catch up by revealing a line after a slow frame or a fast burst.
      current.nextAt = now + TOKEN_INTERVAL_MS;
      redraw(value => value + 1);
    }
    current.frame = requestAnimationFrame(step);
  }

  useLayoutEffect(() => {
    const current = state.current;
    const reset = key !== current.key || enabled !== current.enabled || !enabled || document.hidden;
    const ids = new Set<string>();
    let changed = false;
    if (reset) { cancel(); current.activeID = undefined; current.responses.clear(); }
    for (const message of messages) {
      if (message.role !== "assistant") continue;
      const identity = chatPresentationKey(message);
      ids.add(identity);
      const previous = current.responses.get(identity);
      if (!previous) {
        current.responses.set(identity, { source: message.content, shown: reset ? message.content : "" });
        changed = true;
      } else if (previous.source !== message.content) {
        if (!message.content.startsWith(previous.shown)) {
          const offset = Math.min(previous.shown.length, message.content.length);
          const boundary = segmenter.segment(message.content).containing(offset)?.index ?? offset;
          previous.shown = message.content.slice(0, boundary);
        }
        previous.source = message.content;
        changed = true;
      }
    }
    for (const id of current.responses.keys()) if (!ids.has(id)) current.responses.delete(id);
    Object.assign(current, { key, enabled, messages });
    if (changed && !reset) redraw(value => value + 1);
    if (enabled && !document.hidden && !current.frame && [...current.responses.values()].some(response => response.shown !== response.source)) {
      current.nextAt = performance.now() + TOKEN_INTERVAL_MS;
      current.frame = requestAnimationFrame(step);
    }
  }, [key, messages, enabled]);

  useEffect(() => {
    const onVisibility = () => { if (document.hidden) revealAll(); };
    document.addEventListener("visibilitychange", onVisibility);
    return () => {
      cancel();
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, []);

  if (!enabled || typeof document === "undefined" || document.hidden || state.current.key !== key || !state.current.enabled) return { messages, animationMessageID: undefined };
  const displayed = messages.map(message => {
    if (message.role !== "assistant" || message.kind === "compaction") return message;
    const shown = state.current.responses.get(chatPresentationKey(message))?.shown ?? "";
    return shown === message.content ? message : { ...message, content: shown };
  });
  return { messages: displayed, animationMessageID: messages.find(message => chatPresentationKey(message) === state.current.activeID)?.id };
}
