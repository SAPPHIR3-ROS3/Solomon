import { useLayoutEffect, useRef, useState, type RefObject } from "react";
import type { ChatMessage } from "./chatTypes";

const PAGE_SIZE = 60;
const rememberedStarts = new Map<string, number>();

type HistoryWindow = { key: string; start: number; length: number; headID: string; pinned: boolean };

// Keep a turn that begins with an assistant message together with its user message.
export function visibleHistoryStart(messages: { role?: string }[], requested: number) {
  if (messages.length === 0) return 0;
  let start = Math.min(Math.max(0, requested), messages.length - 1);
  while (start > 0 && messages[start]?.role === "assistant") start--;
  return start;
}

export function tailHistoryStart(messages: { role?: string }[]) {
  return visibleHistoryStart(messages, Math.max(0, messages.length - PAGE_SIZE));
}

function readRemembered(chatKey: string) {
  const remembered = rememberedStarts.get(chatKey);
  if (remembered !== undefined) return remembered;
  try {
    const saved = window.sessionStorage.getItem(`solomon.chat-history-start.v1.${chatKey}`);
    if (saved !== null && Number.isInteger(Number(saved)) && Number(saved) >= 0) return Number(saved);
  } catch { /* In-memory restoration still works when storage is disabled. */ }
  return undefined;
}

function initialWindow(chatKey: string, messages: ChatMessage[]): HistoryWindow {
  const remembered = readRemembered(chatKey);
  const tail = tailHistoryStart(messages);
  const pinned = remembered === undefined || remembered >= Math.max(0, messages.length - PAGE_SIZE);
  return { key: chatKey, start: pinned ? tail : visibleHistoryStart(messages, remembered!), length: messages.length, headID: messages[0]?.id ?? "", pinned };
}

// A snapshot can replace a short cache with the full transcript. Stay on the
// recent page instead of painting the older tool calls over the open chat.
export function reconcileHistoryWindow(current: HistoryWindow, messages: { id?: string; role?: string }[]): HistoryWindow {
  const headID = messages[0]?.id ?? "";
  const growth = messages.length - current.length;
  const prepended = growth > 0 && current.headID !== "" && messages[growth]?.id === current.headID;
  const appended = growth > 0 && headID !== "" && headID === current.headID;
  const replaced = growth < 0 || (current.headID === "" && growth > 0) || (current.headID !== "" && !prepended && !appended);
  if (current.pinned && (replaced || prepended || (appended && growth > PAGE_SIZE))) {
    return { ...current, start: tailHistoryStart(messages), length: messages.length, headID };
  }
  const start = visibleHistoryStart(messages, current.start);
  return { ...current, start, length: messages.length, headID };
}

// Render the recent transcript first; prepend history as the reader approaches
// its start. Keep each assistant turn with its preceding user message.
export function useChatHistoryWindow(chatKey: string, messages: ChatMessage[], shellRef: RefObject<HTMLDivElement | null>) {
  const [windowState, setWindowState] = useState(() => initialWindow(chatKey, messages));
  const active = windowState.key === chatKey ? windowState : initialWindow(chatKey, messages);
  const next = reconcileHistoryWindow(active, messages);
  if (next.key !== windowState.key || next.start !== windowState.start || next.length !== windowState.length || next.headID !== windowState.headID || next.pinned !== windowState.pinned) {
    setWindowState(next);
  }
  const start = next.start;
  const pendingAnchor = useRef<{ key: string; height: number; top: number } | null>(null);
  const allowScrollLoad = useRef(false);

  useLayoutEffect(() => {
    allowScrollLoad.current = false;
    rememberedStarts.delete(chatKey);
    rememberedStarts.set(chatKey, start);
    try { window.sessionStorage.setItem(`solomon.chat-history-start.v1.${chatKey}`, String(start)); } catch { /* Retain the in-memory window. */ }
    while (rememberedStarts.size > 100) rememberedStarts.delete(rememberedStarts.keys().next().value!);
    const anchor = pendingAnchor.current;
    const shell = shellRef.current;
    if (anchor?.key === chatKey && shell) shell.scrollTop = anchor.top + shell.scrollHeight - anchor.height;
    pendingAnchor.current = null;
    const frame = requestAnimationFrame(() => { allowScrollLoad.current = true; });
    return () => cancelAnimationFrame(frame);
  }, [chatKey, start, shellRef]);

  function notePrepended(count: number) {
    if (count <= 0) return;
    setWindowState((current) => {
      const base = current.key === chatKey ? current : next;
      return { ...base, key: chatKey, pinned: false, start: base.start + count };
    });
  }

  function loadEarlier() {
    const shell = shellRef.current;
    if (!shell || start === 0 || pendingAnchor.current) return;
    pendingAnchor.current = { key: chatKey, height: shell.scrollHeight, top: shell.scrollTop };
    setWindowState({ ...next, pinned: false, start: Math.max(0, start - PAGE_SIZE) });
  }

  function loadEarlierFromScroll() {
    if (!allowScrollLoad.current) return;
    loadEarlier();
  }

  return { start, loadEarlier, loadEarlierFromScroll, notePrepended };
}
