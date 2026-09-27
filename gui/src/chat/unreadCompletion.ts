export type ChatCompletionEvent = {
  chatID: string;
  currentCursor: number;
  eventSequence?: number;
  eventType: string;
  isReplay: boolean;
  selectedChatID: string | null;
};

export function shouldMarkChatCompletionUnread(event: ChatCompletionEvent): boolean {
  if (event.eventType !== "run_end" || event.selectedChatID === event.chatID) return false;
  if (!event.isReplay) return true;
  return event.eventSequence !== undefined
    && Number.isFinite(event.eventSequence)
    && event.eventSequence > event.currentCursor;
}

export function markChatCompletionUnread(current: Set<string>, chatID: string): Set<string> {
  if (!chatID || current.has(chatID)) return current;
  return new Set([...current, chatID]);
}

export function markChatCompletionRead(current: Set<string>, chatID: string): Set<string> {
  if (!current.has(chatID)) return current;
  const next = new Set(current);
  next.delete(chatID);
  return next;
}
