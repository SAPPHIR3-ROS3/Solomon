import type { ChatStreamEvent, ChatStreamEventHandler } from "./chatClient";

// Limit text/reasoning updates to 20 per second. Flush before structural events
// so snapshots, tool calls and completion always see all preceding text.
export type BufferedChatStreamEvent = { event: ChatStreamEvent; replay: boolean; imageOrigin: string };

export function createChatStreamBatch(onEvents: (events: BufferedChatStreamEvent[]) => void) {
  let pending: BufferedChatStreamEvent[] = [];
  let timer: ReturnType<typeof setTimeout> | undefined;

  const flush = () => {
    if (timer !== undefined) clearTimeout(timer);
    timer = undefined;
    const batch = pending;
    pending = [];
    if (batch.length) onEvents(batch);
  };

  const push: ChatStreamEventHandler = (event, replay = false, imageOrigin = "") => {
    if (event.type !== "assistant_delta") {
      flush();
      onEvents([{ event, replay, imageOrigin }]);
      return;
    }
    const last = pending.at(-1);
    if (last && last.event.channel === event.channel && last.replay === replay && last.imageOrigin === imageOrigin) {
      last.event = { ...event, delta: `${last.event.delta ?? ""}${event.delta ?? ""}` };
    } else {
      pending.push({ event: { ...event }, replay, imageOrigin });
    }
    if (timer === undefined) timer = setTimeout(flush, 50);
  };

  return { push, flush };
}
