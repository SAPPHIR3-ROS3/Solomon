import { notifyNotification } from "../notifications/store";
import { recordProjectChatActivity } from "../projects/chatActivity";
import { beginNavigation, cancelNavigation, failNavigation, markNavigationSelection, measureNavigation, measureNavigationSync, setNavigationCache } from "./navigationPerformance";
import { createChatStreamBatch } from "./streamBatch";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { sendProjectCommand, applyLiveStreamEvent, chatImageInputs, controlLiveSubchat, createLiveChat, deleteLiveChat, deleteLiveChatMessage, fetchLiveChat, fetchLiveSubchat, preserveLiveWorkState, renameLiveChat, snapshotComposerImages, stopLiveChat, streamLiveChatEvents, streamLiveChatMessage, type ChatStreamEventHandler, type LiveChat } from "./chatClient";
import { forgetChatStreamCursor, forgetRememberedActiveChat, getChat, getChatStreamCursor, getUnreadCompletedChatIDs, rememberActiveChat, saveUnreadCompletedChatIDs, removeChat, saveChat, saveChatStreamCursor, updateChat, useChatStore } from "./chatStore";
import { subagentIsActive, subagentStatus } from "./ChatToolActivity";
import type { Chat, ChatMessage } from "./chatTypes";
import { markChatCompletionRead, markChatCompletionUnread, shouldMarkChatCompletionUnread } from "./unreadCompletion";
import type { ComposerImageAttachment, ComposerTerminalClip } from "./composerTypes";
import { fetchProjectBranches, fetchProjectWorktrees, projectWorktreeLabel, PROJECTS_CHANGED_EVENT, type Project } from "../projects/projects";

export type ChatRuntime = {
  chats: Chat[];
  selectedChat: Chat | null;
  selectedChatID: string | null;
  isLoading: boolean;
  error: string;
  streamingChatIDs: ReadonlySet<string>;
  unreadCompletedChatIDs: ReadonlySet<string>;
  queuedMessages: ReadonlyMap<string, readonly ChatMessage[]>;
  clearSelection: () => void;
  openProjectChat: (project: Project, chatID: string, navigationID?: number) => Promise<void>;
  sendNewProjectMessage: (project: Project, content: string, images?: ComposerImageAttachment[], clips?: ComposerTerminalClip[]) => Promise<void>;
  sendMessage: (chatID: string, message: ChatMessage) => void;
  renameChat: (projectID: string, chatID: string, title: string) => Promise<void>;
  deleteChat: (projectID: string, chatID: string) => Promise<void>;
  deleteMessage: (chatID: string, messageID: string) => Promise<void>;
  stopChat: (chatID: string) => void;
  stopTool: (chatID: string, messageID: string, toolID: string) => void;
  loadSubchat: (subchatID: string) => Promise<ChatMessage[]>;
};

export function useChatRuntime(): ChatRuntime {
  const chats = useChatStore();
  const [selectedChatID, setSelectedChatID] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState("");
  const [streamingChatIDs, setStreamingChatIDs] = useState<Set<string>>(() => new Set());
  const [unreadCompletedChatIDs, setUnreadCompletedChatIDs] = useState<Set<string>>(getUnreadCompletedChatIDs);
  const [queuedMessages, setQueuedMessages] = useState<Map<string, ChatMessage[]>>(() => new Map());
  const messageQueues = useRef(new Map<string, ChatMessage[]>());
  const streams = useRef(new Map<string, AbortController>());
  const streamRetryTimers = useRef(new Map<string, number>());
  const streamRetryAttempts = useRef(new Map<string, number>());
  const stopRequested = useRef(new Set<string>());
  const selectedChatIDRef = useRef<string | null>(null);
  const loadController = useRef<AbortController | null>(null);
  const loadRequest = useRef(0);
  const navigationIDRef = useRef<number | undefined>(undefined);
  const createController = useRef<AbortController | null>(null);
  const createRequest = useRef(0);

  useEffect(() => {
    selectedChatIDRef.current = selectedChatID;
  }, [selectedChatID]);

  useEffect(() => {
    saveUnreadCompletedChatIDs(unreadCompletedChatIDs);
  }, [unreadCompletedChatIDs]);

  const selectedChat = useMemo(
    () => chats.find((chat) => chat.id === selectedChatID) ?? null,
    [chats, selectedChatID],
  );
  const selectedSubagentsActive = selectedChat?.messages.some((message) => (
    message.toolCalls?.some((tool) => tool.name === "subagent" && subagentIsActive(subagentStatus(tool)))
  )) ?? false;

  useEffect(() => {
    if (!selectedChat?.projectID || !selectedSubagentsActive) return;
    const projectID = selectedChat.projectID;
    const chatID = selectedChat.id;
    const workspace = {
      workspaceName: selectedChat.workspaceName,
      workspacePath: selectedChat.workspacePath,
      branch: selectedChat.branch,
      worktree: selectedChat.worktree,
    };
    let cancelled = false;
    const refresh = () => {
      void fetchLiveChat(projectID, chatID)
        .then((latest) => {
          if (cancelled) return;
          const current = getChat(chatID);
          const merged = current ? preserveLiveWorkState(current, latest) : latest;
          saveChat({ ...merged, ...workspace });
        })
        .catch(() => undefined);
    };
    const timer = window.setInterval(refresh, 1000);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [selectedChat?.id, selectedChat?.projectID, selectedChat?.workspaceName, selectedChat?.workspacePath, selectedChat?.branch, selectedChat?.worktree, selectedSubagentsActive]);

  const cancelPendingRequests = useCallback(() => {
    cancelNavigation(navigationIDRef.current);
    loadRequest.current += 1;
    loadController.current?.abort();
    loadController.current = null;
    createRequest.current += 1;
    createController.current?.abort();
    createController.current = null;
    setIsLoading(false);
  }, []);

  useEffect(() => () => {
    loadController.current?.abort();
    createController.current?.abort();
    streams.current.forEach((controller) => controller.abort());
    streamRetryTimers.current.forEach((timer) => window.clearTimeout(timer));
    streamRetryTimers.current.clear();
  }, []);

  const clearSelection = useCallback(() => {
    cancelPendingRequests();
    forgetRememberedActiveChat();
    selectedChatIDRef.current = null;
    setSelectedChatID(null);
    setError("");
  }, [cancelPendingRequests]);

  const markStreaming = useCallback((chatID: string, streaming: boolean) => {
    setStreamingChatIDs((current) => {
      const next = new Set(current);
      if (streaming) next.add(chatID);
      else next.delete(chatID);
      return next;
    });
  }, []);

  const publishMessageQueues = useCallback(() => {
    setQueuedMessages(new Map(messageQueues.current));
  }, []);

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
        const latest = await fetchLiveChat(projectID, chat.id);
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
        if (isAbortError(reason)) return;
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

  const sendDaemonMessage = useCallback((chat: Chat, message: ChatMessage) => {
    if (!chat.projectID || streams.current.has(chat.id)) return;
    forgetChatStreamCursor(chat.projectID, chat.id);
    const isCommand = message.content.trimStart().startsWith("/");
    stopRequested.current.delete(chat.id);
    updateChat(chat.id, (current) => ({ ...current, messages: isCommand ? current.messages : [...current.messages, message], rateLimitNotice: undefined, runStartedAt: Date.now(), status: "running" }));
    setError("");
    attachDaemonStream(chat, (signal, onEvent) => streamLiveChatMessage(
      chat.projectID!,
      chat.id,
      message.content,
      chatImageInputs(message.images),
      message.clips ?? [],
      signal,
      onEvent,
    ), (signal, onEvent) => streamLiveChatEvents(chat.projectID!, chat.id, getChatStreamCursor(chat.projectID!, chat.id), signal, onEvent));
  }, [attachDaemonStream]);

  const reconnectDaemonStream = useCallback((chat: Chat) => {
    if (!chat.projectID || streams.current.has(chat.id)) return;
    attachDaemonStream(chat, (signal, onEvent) => streamLiveChatEvents(chat.projectID!, chat.id, getChatStreamCursor(chat.projectID!, chat.id), signal, onEvent));
  }, [attachDaemonStream]);

  useEffect(() => {
    if (!selectedChat?.projectID || selectedChat.status !== "running" || stopRequested.current.has(selectedChat.id)) return;
    const currentStream = streams.current.get(selectedChat.id);
    if (currentStream && !currentStream.signal.aborted) return;
    if (currentStream) streams.current.delete(selectedChat.id);
    reconnectDaemonStream(selectedChat);
  }, [reconnectDaemonStream, selectedChat]);

  const sendNextQueuedMessage = useCallback((chat: Chat) => {
    const queue = messageQueues.current.get(chat.id);
    if (!queue?.length) return;
    const [message, ...remaining] = queue;
    if (remaining.length) messageQueues.current.set(chat.id, remaining);
    else messageQueues.current.delete(chat.id);
    publishMessageQueues();
    sendDaemonMessage(chat, message);
  }, [publishMessageQueues, sendDaemonMessage]);

  const sendMessage = useCallback((chatID: string, message: ChatMessage) => {
    const chat = getChat(chatID);
    if (!chat?.projectID) return;
    recordProjectChatActivity(chat.projectID, chatID);
    const isRunning = streams.current.has(chatID) || chat.status === "running";
    if (isRunning || messageQueues.current.has(chatID)) {
      messageQueues.current.set(chatID, [...(messageQueues.current.get(chatID) ?? []), message]);
      publishMessageQueues();
      if (!isRunning) sendNextQueuedMessage(chat);
      return;
    }
    sendDaemonMessage(chat, message);
  }, [publishMessageQueues, sendDaemonMessage, sendNextQueuedMessage]);

  useEffect(() => {
    for (const chatID of messageQueues.current.keys()) {
      const chat = getChat(chatID);
      // Wait for the stream and its final snapshot before starting the next turn.
      // Failed or interrupted runs leave the queue visible until another send.
      if (!chat?.projectID || streams.current.has(chatID) || (chat.status !== "success" && chat.status !== undefined)) continue;
      sendNextQueuedMessage(chat);
    }
  }, [chats, streamingChatIDs, queuedMessages, sendNextQueuedMessage]);

  const sendNewProjectMessage = useCallback(async (project: Project, content: string, composerImages: ComposerImageAttachment[] = [], composerClips: ComposerTerminalClip[] = []) => {
    if (createController.current) return;
    const controller = new AbortController();
    const requestID = createRequest.current + 1;
    createRequest.current = requestID;
    createController.current = controller;
    setIsLoading(true);
    setError("");
    try {
      if (content.trimStart().startsWith("/")) {
        if (composerImages.length || composerClips.length) throw new Error("Send attachments separately from slash commands");
        if (!await sendProjectCommand(project.id, content, controller.signal)) {
          window.dispatchEvent(new CustomEvent(PROJECTS_CHANGED_EVENT));
          return;
        }
      }
      const images = await snapshotComposerImages(composerImages);
      if (controller.signal.aborted || requestID !== createRequest.current) return;
      const [chat, projectContext] = await Promise.all([
        createLiveChat(project.id, "", controller.signal),
        fetchProjectContext(project.id, controller.signal),
      ]);
      if (controller.signal.aborted || requestID !== createRequest.current) return;
      const contextualChat = {
        ...chat,
        branch: chat.branch ?? projectContext.branch,
        workspaceName: project.name,
        workspacePath: project.path,
        worktree: chat.worktree ?? projectContext.worktree,
      };
      saveChat(contextualChat);
      rememberActiveChat(project.id, contextualChat.id);
      selectedChatIDRef.current = contextualChat.id;
      setSelectedChatID(contextualChat.id);
      window.dispatchEvent(new CustomEvent(PROJECTS_CHANGED_EVENT));
      sendMessage(contextualChat.id, {
        createdAt: Date.now(),
        id: `user-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
        clips: composerClips,
        images,
        role: "user",
        content,
      });
    } catch (reason: unknown) {
      if (controller.signal.aborted || requestID !== createRequest.current) return;
      setError(reason instanceof Error ? reason.message : "Unable to create chat");
      throw reason;
    } finally {
      if (createRequest.current === requestID) {
        createController.current = null;
        setIsLoading(false);
      }
    }
  }, [sendMessage]);

  const openProjectChat = useCallback(async (project: Project, chatID: string, navigationID?: number) => {
    cancelPendingRequests();
    const traceID = navigationID ?? beginNavigation(project.id, chatID);
    navigationIDRef.current = traceID;
    const controller = new AbortController();
    const requestID = loadRequest.current;
    loadController.current = controller;
    const storedChat = getChat(chatID);
    const cachedChat = storedChat?.projectID === project.id ? storedChat : null;
    setNavigationCache(traceID, Boolean(cachedChat));
    selectedChatIDRef.current = cachedChat?.id ?? null;
    setSelectedChatID(cachedChat?.id ?? null);
    setError("");
    setIsLoading(!cachedChat);
    if (cachedChat) {
      markNavigationSelection(traceID);
      rememberActiveChat(project.id, chatID);
      setUnreadCompletedChatIDs((current) => markChatCompletionRead(current, chatID));
      if (cachedChat.status === "running") reconnectDaemonStream(cachedChat);
    }
    // Git metadata must not delay displaying the transcript.
    const projectContextRequest = fetchProjectContext(project.id, controller.signal, traceID);
    try {
      const snapshot = await measureNavigation(traceID, "chat.total", () => fetchLiveChat(project.id, chatID, controller.signal, traceID));
      if (requestID !== loadRequest.current) return;
      const current = getChat(chatID);
      // A stream or user action may have updated the cache during the request.
      // Keep those newer changes rather than replacing them with an older snapshot.
      const chat = current && current !== cachedChat
        ? current
        : current ? preserveLiveWorkState(current, snapshot) : snapshot;
      measureNavigationSync(traceID, "cache.save", () => saveChat({
        ...chat,
        runStartedAt: chat.status === "running" ? chat.runStartedAt ?? Date.now() : undefined,
        branch: chat.branch ?? cachedChat?.branch,
        worktree: chat.worktree ?? cachedChat?.worktree,
        workspaceName: project.name,
        workspacePath: project.path,
      }));
      rememberActiveChat(project.id, chat.id);
      setUnreadCompletedChatIDs((current) => markChatCompletionRead(current, chat.id));
      selectedChatIDRef.current = chat.id;
      setSelectedChatID(chat.id);
      markNavigationSelection(traceID);
      setIsLoading(false);
      const contextualChat = getChat(chat.id);
      if (contextualChat?.status === "running") reconnectDaemonStream(contextualChat);
      const projectContext = await projectContextRequest;
      if (requestID !== loadRequest.current) return;
      updateChat(chat.id, (latest) => ({
        ...latest,
        branch: latest.branch ?? projectContext.branch,
        worktree: latest.worktree ?? projectContext.worktree,
      }));
    } catch (reason: unknown) {
      if (requestID !== loadRequest.current || isAbortError(reason)) return;
      failNavigation(traceID);
      setError(reason instanceof Error ? reason.message : "Unable to load chat");
    } finally {
      if (requestID === loadRequest.current) {
        loadController.current = null;
        setIsLoading(false);
      }
    }
  }, [cancelPendingRequests, reconnectDaemonStream]);

  const deleteMessage = useCallback(async (chatID: string, messageID: string) => {
    const chat = getChat(chatID);
    if (!chat?.projectID) return;
    try {
      const latest = await deleteLiveChatMessage(chat.projectID, chatID, messageID);
      saveChat({ ...latest, workspaceName: chat.workspaceName, workspacePath: chat.workspacePath, branch: chat.branch, worktree: chat.worktree });
      window.dispatchEvent(new CustomEvent(PROJECTS_CHANGED_EVENT));
    } catch (reason: unknown) {
      setError(reason instanceof Error ? reason.message : "Unable to delete message");
    }
  }, []);

  const renameChat = useCallback(async (projectID: string, chatID: string, title: string) => {
    const current = getChat(chatID);
    try {
      const renamed = await renameLiveChat(projectID, chatID, title);
      if (current) {
        saveChat({
          ...renamed,
          workspaceName: current.workspaceName,
          workspacePath: current.workspacePath,
          branch: current.branch,
          worktree: current.worktree,
        });
      }
      setError("");
      window.dispatchEvent(new CustomEvent(PROJECTS_CHANGED_EVENT));
    } catch (reason: unknown) {
      const message = reason instanceof Error ? reason.message : "Unable to rename chat";
      setError(message);
      throw reason instanceof Error ? reason : new Error(message);
    }
  }, []);

  const deleteChat = useCallback(async (projectID: string, chatID: string) => {
    try {
      await deleteLiveChat(projectID, chatID);
      streams.current.get(chatID)?.abort();
      const retryTimer = streamRetryTimers.current.get(chatID);
      if (retryTimer !== undefined) window.clearTimeout(retryTimer);
      streamRetryTimers.current.delete(chatID);
      streamRetryAttempts.current.delete(chatID);
      forgetChatStreamCursor(projectID, chatID);
      setUnreadCompletedChatIDs((current) => markChatCompletionRead(current, chatID));
      messageQueues.current.delete(chatID);
      publishMessageQueues();
      removeChat(chatID);
      if (selectedChatIDRef.current === chatID) {
        cancelPendingRequests();
        forgetRememberedActiveChat();
        setUnreadCompletedChatIDs((current) => markChatCompletionRead(current, chatID));
        selectedChatIDRef.current = null;
        setSelectedChatID(null);
      }
      setError("");
      window.dispatchEvent(new CustomEvent(PROJECTS_CHANGED_EVENT));
    } catch (reason: unknown) {
      const message = reason instanceof Error ? reason.message : "Unable to delete chat";
      setError(message);
      throw reason instanceof Error ? reason : new Error(message);
    }
  }, [cancelPendingRequests, publishMessageQueues]);

  const stopChat = useCallback((chatID: string) => {
    const chat = getChat(chatID);
    if (!chat?.projectID) return;
    stopRequested.current.add(chatID);
    const retryTimer = streamRetryTimers.current.get(chatID);
    if (retryTimer !== undefined) window.clearTimeout(retryTimer);
    streamRetryTimers.current.delete(chatID);
    streamRetryAttempts.current.delete(chatID);
    streams.current.get(chatID)?.abort();
    streams.current.delete(chatID);
    markStreaming(chatID, false);
    updateChat(chatID, (current) => ({ ...current, status: "interrupted" }));
    void stopLiveChat(chat.projectID, chatID).catch((reason: unknown) => setError(reason instanceof Error ? reason.message : "Unable to stop chat"));
  }, [markStreaming]);

	const stopTool = useCallback((chatID: string, messageID: string, toolID: string) => {
		const chat = getChat(chatID);
		const parent = chat?.messages.find((message) => message.id === messageID);
		const tool = parent?.toolCalls?.find((candidate) => candidate.id === toolID);
		if (!chat?.projectID || !tool) return;
		const subchatID = tool.result?.subchatId;
		const stopRequest = subchatID
			? controlLiveSubchat(chat.projectID, subchatID, "stop")
			: stopLiveChat(chat.projectID, chatID);
		void stopRequest
			.then(() => fetchLiveChat(chat.projectID!, chatID))
			.then((latest) => saveChat({ ...latest, workspaceName: chat.workspaceName, workspacePath: chat.workspacePath, branch: chat.branch, worktree: chat.worktree }))
			.catch((reason: unknown) => setError(reason instanceof Error ? reason.message : subchatID ? "Unable to stop subchat" : "Unable to stop chat"));
  }, []);

  const loadSubchat = useCallback(async (subchatID: string): Promise<ChatMessage[]> => {
    const chat = selectedChatID ? getChat(selectedChatID) : null;
    if (!chat?.projectID) return [];
		const subchat = await fetchLiveSubchat(chat.projectID, subchatID);
		if (subchat.subchatStatus) {
			const running = subchat.subchatStatus === "running" || subchat.subchatStatus === "queued";
			const toolStatus = running ? "running" : subchat.subchatStatus === "done" ? "success" : "error";
			updateChat(chat.id, (current) => ({
				...current,
				messages: current.messages.map((message) => ({
					...message,
					toolCalls: message.toolCalls?.map((tool) => tool.result?.subchatId !== subchatID
						? tool
						: { ...tool, result: { ...tool.result, subchatStatus: subchat.subchatStatus }, status: toolStatus }),
				})),
			}));
    }
    return subchat.messages;
  }, [selectedChatID]);

  return {
    chats,
    selectedChat,
    selectedChatID,
    isLoading,
    error,
    streamingChatIDs,
    unreadCompletedChatIDs,
    queuedMessages,
    clearSelection,
    openProjectChat,
    sendNewProjectMessage,
    sendMessage,
    renameChat,
    deleteChat,
    deleteMessage,
    stopChat,
    stopTool,
    loadSubchat,
  };
}

async function fetchProjectContext(projectID: string, signal: AbortSignal, navigationID?: number): Promise<{ branch?: string; worktree?: string }> {
  const [branches, worktrees] = await Promise.allSettled([
    measureNavigation(navigationID, "git.branches", () => fetchProjectBranches(projectID, signal)),
    measureNavigation(navigationID, "git.worktrees", () => fetchProjectWorktrees(projectID, signal)),
  ]);
  const branch = branches.status === "fulfilled" && branches.value.isRepo ? branches.value.current || undefined : undefined;
  if (worktrees.status !== "fulfilled") return { branch };
  const available = worktrees.value.worktrees.filter((worktree) => !worktree.bare);
  const current = available.find((worktree) => worktree.current) ?? available[0];
  if (!current) return { branch };
  return { branch, worktree: available.length === 1 ? "local" : projectWorktreeLabel(current.path) };
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
