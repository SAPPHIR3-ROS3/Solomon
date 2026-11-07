import { useEffect, type RefObject } from "react";
import { fetchLiveChat, preserveLiveWorkState } from "./chatClient";
import { getChat, saveChat } from "./chatStore";
import type { Chat } from "./chatTypes";

export function useSubagentRefresh(chat: Chat | null, active: boolean, deleting: RefObject<Set<string>>) {
  useEffect(() => {
    if (!chat?.projectID || !active) return;
    const projectID = chat.projectID;
    const chatID = chat.id;
    const workspace = {
      workspaceName: chat.workspaceName,
      workspacePath: chat.workspacePath,
      branch: chat.branch,
      worktree: chat.worktree,
    };
    return startRefresh(async (signal) => {
      const latest = await fetchLiveChat(projectID, chatID, signal);
      if (signal.aborted || deleting.current.has(chatID)) return;
      const current = getChat(chatID);
      saveChat({ ...(current ? preserveLiveWorkState(current, latest) : latest), ...workspace });
    });
  }, [chat?.id, chat?.projectID, chat?.workspaceName, chat?.workspacePath, chat?.branch, chat?.worktree, active, deleting]);
}

export function startRefresh(task: (signal: AbortSignal) => Promise<void>) {
  const controller = new AbortController();
  let timer: number;
  const refresh = async () => {
    try {
      await task(controller.signal);
    } catch {
    } finally {
      if (!controller.signal.aborted) timer = window.setTimeout(refresh, 1000);
    }
  };
  timer = window.setTimeout(refresh, 1000);
  return () => {
    controller.abort();
    window.clearTimeout(timer);
  };
}
