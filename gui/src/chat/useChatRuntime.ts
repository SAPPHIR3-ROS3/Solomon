import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { applyLiveStreamEvent, chatImageInputs, controlLiveSubchat, createLiveChat, deleteLiveChatMessage, fetchLiveChat, fetchLiveSubchat, preserveLiveWorkState, snapshotComposerImages, stopLiveChat, streamLiveChatEvents, streamLiveChatMessage, type ChatStreamEventHandler, type LiveChat } from "./chatClient";
import { forgetChatStreamCursor, forgetRememberedActiveChat, getChat, getChatStreamCursor, rememberActiveChat, saveChat, saveChatStreamCursor, updateChat, useChatStore } from "./chatStore";
import type { Chat, ChatMessage } from "./chatTypes";
import type { ComposerImageAttachment } from "./composerTypes";
import { fetchProjectBranches, fetchProjectWorktrees, projectWorktreeLabel, PROJECTS_CHANGED_EVENT, type Project } from "../projects/projects";

export type ChatRuntime = {
  chats: Chat[];
  selectedChat: Chat | null;
  selectedChatID: string | null;
  isLoading: boolean;
  error: string;
  streamingChatIDs: ReadonlySet<string>;
  pendingMessageIDs: ReadonlyMap<string, ReadonlySet<string>>;
  clearSelection: () => void;
  openProjectChat: (project: Project, chatID: string) => Promise<void>;
  sendNewProjectMessage: (project: Project, content: string, images?: ComposerImageAttachment[]) => Promise<void>;
  sendMessage: (chatID: string, message: ChatMessage) => void;
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
  const [pendingMessageIDs, setPendingMessageIDs] = useState<Map<string, Set<string>>>(() => new Map());
  const streams = useRef(new Map<string, AbortController>());
  const streamRetryTimers = useRef(new Map<string, number>());
  const streamRetryAttempts = useRef(new Map<string, number>());
  const loadController = useRef<AbortController | null>(null);
  const loadRequest = useRef(0);
  const createController = useRef<AbortController | null>(null);
  const createRequest = useRef(0);

  const selectedChat = useMemo(
    () => chats.find((chat) => chat.id === selectedChatID) ?? null,
    [chats, selectedChatID],
  );

  const cancelPendingRequests = useCallback(() => {
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

  const markMessagePending = useCallback((chatID: string, messageID: string, pending: boolean) => {
    setPendingMessageIDs((current) => {
      const next = new Map(current);
      const messageIDs = new Set(next.get(chatID));
      if (pending) messageIDs.add(messageID);
      else messageIDs.delete(messageID);
      if (messageIDs.size) next.set(chatID, messageIDs);
      else next.delete(chatID);
      return next;
    });
  }, []);

  const attachDaemonStream = useCallback((
    chat: Chat,
    open: (signal: AbortSignal, onEvent: ChatStreamEventHandler) => Promise<void>,
    reconnectOpen?: (signal: AbortSignal, onEvent: ChatStreamEventHandler) => Promise<void>,
  ) => {
    const projectID = chat.projectID;
    if (!projectID || streams.current.has(chat.id)) return;
    const retryOpen = reconnectOpen ?? open;
    let retryWhenDetached = false;
    let receivedStreamError = false;
    const retry = () => {
      if (streams.current.has(chat.id) || streamRetryTimers.current.has(chat.id)) return;
      const current = getChat(chat.id);
      if (!current?.projectID || current.status !== "running") return;
      const attempts = streamRetryAttempts.current.get(chat.id) ?? 0;
      const delay = Math.min(5000, 300 * (2 ** Math.min(attempts, 4)));
      streamRetryAttempts.current.set(chat.id, attempts + 1);
      const timer = window.setTimeout(() => {
        streamRetryTimers.current.delete(chat.id);
        const latest = getChat(chat.id);
        if (latest?.status === "running") attachDaemonStream(latest, retryOpen, retryOpen);
      }, delay);
      streamRetryTimers.current.set(chat.id, timer);
    };
    const controller = new AbortController();
    streams.current.set(chat.id, controller);
    markStreaming(chat.id, true);
    const onEvent: ChatStreamEventHandler = (event, replay = false, imageOrigin = "") => {
      if (event.type === "assistant_start" || event.type === "error" || event.type === "run_end") {
        setPendingMessageIDs((current) => {
          if (!current.has(chat.id)) return current;
          const next = new Map(current);
          next.delete(chat.id);
          return next;
        });
      }
      if (event.type === "error") {
        receivedStreamError = true;
        setError(streamErrorMessage(event));
      } else if (event.type === "run_end") {
        const exitCode = typeof event.exit_code === "number" && Number.isFinite(event.exit_code) ? event.exit_code : undefined;
        if (exitCode !== undefined && exitCode !== 0 && !receivedStreamError) setError(runEndErrorMessage(event));
      }
      streamRetryAttempts.current.delete(chat.id);
      if (typeof event.seq === "number" && Number.isFinite(event.seq) && event.seq > getChatStreamCursor(projectID, chat.id)) {
        saveChatStreamCursor(projectID, chat.id, event.seq);
      }
      updateChat(chat.id, (current) => applyLiveStreamEvent(current as LiveChat, event, imageOrigin, replay));
      if (event.type === "chat_title") window.dispatchEvent(new CustomEvent(PROJECTS_CHANGED_EVENT));
    };
    void open(controller.signal, onEvent)
      .then(async () => {
        const latest = await fetchLiveChat(projectID, chat.id);
        const current = getChat(chat.id);
        const merged = current ? preserveLiveWorkState(current, latest) : latest;
        saveChat({ ...merged, workspaceName: chat.workspaceName, workspacePath: chat.workspacePath, branch: chat.branch, worktree: chat.worktree });
        window.dispatchEvent(new CustomEvent(PROJECTS_CHANGED_EVENT));
        if (latest.status === "running") retryWhenDetached = true;
        else {
          streamRetryAttempts.current.delete(chat.id);
          forgetChatStreamCursor(projectID, chat.id);
        }
      })
      .catch((reason: unknown) => {
        if (isAbortError(reason)) return;
        setError(reason instanceof Error ? reason.message : "Chat stream failed");
        retryWhenDetached = true;
      })
      .finally(() => {
        if (streams.current.get(chat.id) !== controller) return;
        streams.current.delete(chat.id);
        markStreaming(chat.id, false);
        if (retryWhenDetached && !controller.signal.aborted) retry();
      });
  }, [markStreaming]);

  const sendDaemonMessage = useCallback((chat: Chat, message: ChatMessage) => {
    if (!chat.projectID || streams.current.has(chat.id)) return;
    forgetChatStreamCursor(chat.projectID, chat.id);
    updateChat(chat.id, (current) => ({ ...current, messages: [...current.messages, message], runStartedAt: Date.now(), status: "running" }));
    markMessagePending(chat.id, message.id, true);
    setError("");
    attachDaemonStream(chat, (signal, onEvent) => streamLiveChatMessage(
      chat.projectID!,
      chat.id,
      message.content,
      chatImageInputs(message.images),
      signal,
      onEvent,
    ), (signal, onEvent) => streamLiveChatEvents(chat.projectID!, chat.id, getChatStreamCursor(chat.projectID!, chat.id), signal, onEvent));
  }, [attachDaemonStream, markMessagePending]);

  const reconnectDaemonStream = useCallback((chat: Chat) => {
    if (!chat.projectID || streams.current.has(chat.id)) return;
    attachDaemonStream(chat, (signal, onEvent) => streamLiveChatEvents(chat.projectID!, chat.id, getChatStreamCursor(chat.projectID!, chat.id), signal, onEvent));
  }, [attachDaemonStream]);

  useEffect(() => {
    if (!selectedChat?.projectID || selectedChat.status !== "running") return;
    const currentStream = streams.current.get(selectedChat.id);
    if (currentStream && !currentStream.signal.aborted) return;
    if (currentStream) streams.current.delete(selectedChat.id);
    reconnectDaemonStream(selectedChat);
  }, [reconnectDaemonStream, selectedChat]);

  const sendMessage = useCallback((chatID: string, message: ChatMessage) => {
    const chat = getChat(chatID);
    if (!chat?.projectID) return;
    sendDaemonMessage(chat, message);
  }, [sendDaemonMessage]);

  const sendNewProjectMessage = useCallback(async (project: Project, content: string, composerImages: ComposerImageAttachment[] = []) => {
    if (createController.current) return;
    const controller = new AbortController();
    const requestID = createRequest.current + 1;
    createRequest.current = requestID;
    createController.current = controller;
    setIsLoading(true);
    setError("");
    try {
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
      setSelectedChatID(contextualChat.id);
      window.dispatchEvent(new CustomEvent(PROJECTS_CHANGED_EVENT));
      sendMessage(contextualChat.id, {
        createdAt: Date.now(),
        id: `user-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
        images,
        role: "user",
        content,
      });
    } catch (reason: unknown) {
      if (controller.signal.aborted || requestID !== createRequest.current) return;
      setError(reason instanceof Error ? reason.message : "Unable to create chat");
    } finally {
      if (createRequest.current === requestID) {
        createController.current = null;
        setIsLoading(false);
      }
    }
  }, [sendMessage]);

  const openProjectChat = useCallback(async (project: Project, chatID: string) => {
    cancelPendingRequests();
    const controller = new AbortController();
    const requestID = loadRequest.current;
    loadController.current = controller;
    setSelectedChatID(null);
    setError("");
    setIsLoading(true);
    try {
      const [chat, projectContext] = await Promise.all([
        fetchLiveChat(project.id, chatID, controller.signal),
        fetchProjectContext(project.id, controller.signal),
      ]);
      if (requestID !== loadRequest.current) return;
      saveChat({
        ...chat,
        runStartedAt: chat.status === "running" ? chat.runStartedAt ?? Date.now() : undefined,
        branch: chat.branch ?? projectContext.branch,
        workspaceName: project.name,
        workspacePath: project.path,
        worktree: chat.worktree ?? projectContext.worktree,
      });
      rememberActiveChat(project.id, chat.id);
      setSelectedChatID(chat.id);
      const contextualChat = getChat(chat.id);
      if (contextualChat?.status === "running") reconnectDaemonStream(contextualChat);
    } catch (reason: unknown) {
      if (requestID !== loadRequest.current || isAbortError(reason)) return;
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

  const stopChat = useCallback((chatID: string) => {
    const chat = getChat(chatID);
    if (!chat?.projectID) return;
    void stopLiveChat(chat.projectID, chatID).catch((reason: unknown) => setError(reason instanceof Error ? reason.message : "Unable to stop chat"));
  }, []);

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
    pendingMessageIDs,
    clearSelection,
    openProjectChat,
    sendNewProjectMessage,
    sendMessage,
    deleteMessage,
    stopChat,
    stopTool,
    loadSubchat,
  };
}

async function fetchProjectContext(projectID: string, signal: AbortSignal): Promise<{ branch?: string; worktree?: string }> {
  const [branches, worktrees] = await Promise.allSettled([
    fetchProjectBranches(projectID, signal),
    fetchProjectWorktrees(projectID, signal),
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
