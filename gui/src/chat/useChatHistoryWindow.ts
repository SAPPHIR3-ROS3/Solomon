import { useLayoutEffect, useRef, useState, type RefObject } from "react";
import type { ChatMessage } from "./chatTypes";

const PAGE_SIZE = 60;
const rememberedStarts = new Map<string, number>();

// Render the recent transcript first; prepend history as the reader approaches
// its start. Keep each assistant turn with its preceding user message.
export function useChatHistoryWindow(chatKey: string, messages: ChatMessage[], shellRef: RefObject<HTMLDivElement | null>) {
  const initialStart = () => {
    const remembered = rememberedStarts.get(chatKey);
    if (remembered !== undefined) return remembered;
    try {
      const saved = window.sessionStorage.getItem(`solomon.chat-history-start.v1.${chatKey}`);
      if (saved !== null && Number.isInteger(Number(saved)) && Number(saved) >= 0) return Number(saved);
    } catch { /* In-memory restoration still works when storage is disabled. */ }
    return Math.max(0, messages.length - PAGE_SIZE);
  };
  const [windowState, setWindowState] = useState(() => ({ key: chatKey, start: initialStart() }));
  let start = Math.min(windowState.key === chatKey ? windowState.start : initialStart(), Math.max(0, messages.length - 1));
  while (start > 0 && messages[start]?.role === "assistant") start--;
  const pendingAnchor = useRef<{ key: string; height: number; top: number } | null>(null);

  useLayoutEffect(() => {
    rememberedStarts.delete(chatKey);
    rememberedStarts.set(chatKey, start);
    try { window.sessionStorage.setItem(`solomon.chat-history-start.v1.${chatKey}`, String(start)); } catch { /* Retain the in-memory window. */ }
    while (rememberedStarts.size > 100) rememberedStarts.delete(rememberedStarts.keys().next().value!);
    const anchor = pendingAnchor.current;
    const shell = shellRef.current;
    if (anchor?.key === chatKey && shell) shell.scrollTop = anchor.top + shell.scrollHeight - anchor.height;
    pendingAnchor.current = null;
  }, [chatKey, start, shellRef]);

  function loadEarlier() {
    const shell = shellRef.current;
    if (!shell || start === 0 || pendingAnchor.current) return;
    pendingAnchor.current = { key: chatKey, height: shell.scrollHeight, top: shell.scrollTop };
    setWindowState({ key: chatKey, start: Math.max(0, start - PAGE_SIZE) });
  }

  return { start, loadEarlier };
}
