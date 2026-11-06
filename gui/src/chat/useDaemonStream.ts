import { useCallback, type MutableRefObject, type Dispatch, type SetStateAction } from "react";
import { notifyNotification } from "../notifications/store";
import { createChatStreamBatch } from "./streamBatch";
import { applyLiveStreamEvent, fetchLiveChat, preserveLiveWorkState, type LiveChat, type ChatStreamEventHandler } from "./chatClient";
import { getChat, getChatStreamCursor, saveChat, updateChat, saveChatStreamCursor, forgetChatStreamCursor } from "./chatStore";
import { shouldMarkChatCompletionUnread, markChatCompletionUnread } from "./unreadCompletion";
import { PROJECTS_CHANGED_EVENT } from "../projects/projects";
import type { Chat } from "./chatTypes";

type Ref<T> = MutableRefObject<T>;
type StreamState = {
 streams: Ref<Map<string, AbortController>>;
 streamRetryTimers: Ref<Map<string, number>>;
 streamRetryAttempts: Ref<Map<string, number>>;
 stopRequested: Ref<Set<string>>;
 selectedChatIDRef: Ref<string | null>;
 markStreaming: (id: string, active: boolean) => void;
 setError: Dispatch<SetStateAction<string>>;
 setUnreadCompletedChatIDs: Dispatch<SetStateAction<Set<string>>>;
};

export function useDaemonStream({ streams, streamRetryTimers, streamRetryAttempts, stopRequested, selectedChatIDRef, markStreaming, setError, setUnreadCompletedChatIDs }: StreamState) {
  const attachDaemonStream = useCallback((
    chat: Chat,
    open: (signal: AbortSignal, onEvent: ChatStreamEventHandler) => Promise<void>,
    reconnectOpen?: (signal: AbortSignal, onEvent: ChatStreamEventHandler) => Promise<void>,
  ) => {
    const projectID = chat.projectID;
    if (!projectID || streams.current.has(chat.id) || stopRequested.current.has(chat.id)) return;
    const retryOpen = reconnectOpen ?? open;
    let retryWhenDetached = false;
    let receivedStreamError = false;
    const retry = () => {
      if (streams.current.has(chat.id) || streamRetryTimers.current.has(chat.id)) return;
      const current = getChat(chat.id);
      if (!current?.projectID || current.status !== "running" || stopRequested.current.has(chat.id)) return;
      const attempts = streamRetryAttempts.current.get(chat.id) ?? 0;
      const delay = Math.min(5000, 300 * (2 ** Math.min(attempts, 4)));
      streamRetryAttempts.current.set(chat.id, attempts + 1);
      const timer = window.setTimeout(() => {
        streamRetryTimers.current.delete(chat.id);
        const latest = getChat(chat.id);
        if (latest?.status === "running" && !stopRequested.current.has(chat.id)) attachDaemonStream(latest, retryOpen, retryOpen);
      }, delay);
      streamRetryTimers.current.set(chat.id, timer);
    };
    const controller = new AbortController();
    streams.current.set(chat.id, controller);
    markStreaming(chat.id, true);
    let currentCursor = getChatStreamCursor(projectID, chat.id);
    const batch = createChatStreamBatch((events) => {
      if (controller.signal.aborted || streams.current.get(chat.id) !== controller) return;
      const previousCursor = currentCursor;
      for (const { event, replay } of events) {
        const eventSequence = typeof event.seq === "number" && Number.isFinite(event.seq) ? event.seq : undefined;
        if (shouldMarkChatCompletionUnread({
          chatID: chat.id,
          currentCursor,
          eventSequence,
          eventType: typeof event.type === "string" ? event.type : "",
          isReplay: replay,
          selectedChatID: selectedChatIDRef.current,
        })) {
          setUnreadCompletedChatIDs((current) => markChatCompletionUnread(current, chat.id));
        }
        if (event.type === "notification" && !replay && typeof event.message === "string") {
          notifyNotification(event.message, typeof event.level === "string" ? event.level : "system");
        }
        if (event.type === "error") {
          receivedStreamError = true;
          setError(streamErrorMessage(event));
        } else if (event.type === "run_end") {
          const exitCode = typeof event.exit_code === "number" && Number.isFinite(event.exit_code) ? event.exit_code : undefined;
          if (exitCode !== undefined && exitCode !== 0 && !receivedStreamError) setError(runEndErrorMessage(event));
        }
        streamRetryAttempts.current.delete(chat.id);
        if (eventSequence !== undefined && eventSequence > currentCursor) {
          currentCursor = eventSequence;
        }
        if (event.type === "chat_title") window.dispatchEvent(new CustomEvent(PROJECTS_CHANGED_EVENT));
      }
      updateChat(chat.id, (current) => events.reduce((next, { event, imageOrigin, replay }) => (
        applyLiveStreamEvent(next, event, imageOrigin, replay)
      ), current as LiveChat));
      if (currentCursor > previousCursor) saveChatStreamCursor(projectID, chat.id, currentCursor);
    });
    void open(controller.signal, batch.push)
      .then(async () => {
        batch.flush();
        if (controller.signal.aborted) return;
        const latest = await fetchLiveChat(projectID, chat.id);
        if (controller.signal.aborted) return;
        const current = getChat(chat.id);
        const merged = current ? preserveLiveWorkState(current, latest) : latest;
        saveChat({ ...merged, workspaceName: chat.workspaceName, workspacePath: chat.workspacePath, branch: chat.branch, worktree: chat.worktree });
        window.dispatchEvent(new CustomEvent(PROJECTS_CHANGED_EVENT));
        if (latest.status === "running" && !stopRequested.current.has(chat.id)) retryWhenDetached = true;
        else {
          streamRetryAttempts.current.delete(chat.id);
          forgetChatStreamCursor(projectID, chat.id);
        }
      })
      .catch((reason: unknown) => {
        batch.flush();
        if (controller.signal.aborted || isAbortError(reason)) return;
        setError(reason instanceof Error ? reason.message : "Chat stream failed");
        retryWhenDetached = true;
      })
      .finally(() => {
        batch.flush();
        if (streams.current.get(chat.id) !== controller) return;
        streams.current.delete(chat.id);
        markStreaming(chat.id, false);
        if (retryWhenDetached && !controller.signal.aborted) retry();
      });
  }, [markStreaming]);

  return attachDaemonStream;
}

function streamErrorMessage(event: Record<string, unknown>): string {
  for (const key of ["detail", "error", "message"]) {
    const value = typeof event[key] === "string" ? event[key].trim() : "";
    if (value && !isGenericErrorText(value)) return value;
  }
  const reason = typeof event.exit_reason === "string" ? event.exit_reason.trim().replace(/[_-]+/g, " ") : "";
  return reason && !isGenericErrorText(reason) ? `Chat run failed: ${reason}.` : "Chat run failed.";
}

function runEndErrorMessage(event: Record<string, unknown>): string {
  const reason = typeof event.exit_reason === "string" ? event.exit_reason.trim().replace(/[_-]+/g, " ") : "";
  const code = typeof event.exit_code === "number" && Number.isFinite(event.exit_code) ? ` (code ${event.exit_code})` : "";
  return reason && !isGenericErrorText(reason) ? `Chat run failed: ${reason}${code}.` : `Chat run failed${code}.`;
}

function isGenericErrorText(value: string): boolean {
  return ["error", "failed", "failure", "run failed", "chat run failed"].includes(value.toLowerCase());
}

function isAbortError(reason: unknown): boolean {
  return reason instanceof DOMException && reason.name === "AbortError";
}
