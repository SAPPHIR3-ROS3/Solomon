import { type CSSProperties, useCallback, useEffect, useRef, useState } from "react";
import { detectClient, initialClient } from "./platform";
import { SidePanel } from "./shell/SidePanel";
import { SidePanelToggle } from "./shell/SidePanelToggle";
import { RightSidePanel } from "./shell/RightSidePanel";
import { RightSidePanelToggle } from "./shell/RightSidePanelToggle";
import { TerminalPanelToggle } from "./shell/TerminalPanelToggle";
import { type View, ViewSwitch } from "./shell/ViewSwitch";
import { TerminalPanel } from "./terminal-panel/TerminalPanel";
import { applyTheme, savedTheme } from "./theme";
import { CustomizationPage } from "./customization/CustomizationPage";
import { Welcome } from "./home/Welcome";
import { NewProjectDialog } from "./projects/NewProjectDialog";
import { SettingsPage } from "./settings/SettingsPage";
import { type Project, type ProjectResearch } from "./projects/projects";
import { ResearchReportView } from "./research/ResearchReportView";
import { estimateFakeStats, fakeAssistantReply, type FakeChatMessage } from "./chat-test/fakeChats";
import { createTemporaryWorkspaceChat, createTestChat, getTestChat, updateTestChat, useTestChatStore } from "./chat-test/testChatStore";
import { TestChatTopbar, TestChatView } from "./chat-test/TestChatView";
import type { LocalFolderSelection, TemporaryWorkspace } from "./projects/temporaryWorkspace";

const DEFAULT_TERMINAL_PANEL_HEIGHT = 240;
const MIN_TERMINAL_PANEL_HEIGHT = 120;
const FALLBACK_KEEP_ALIVE_HEIGHT = 96;
const DEFAULT_SIDE_PANEL_WIDTH = 240;
const MIN_SIDE_PANEL_WIDTH = DEFAULT_SIDE_PANEL_WIDTH;
const WELCOME_HORIZONTAL_PADDING = 72;
const MAX_COMPOSER_WIDTH = 960;
const TEXTBOX_SIDE_PANEL_GAP = 36;
const LEFT_SIDE_PANEL_WIDTH_KEY = "solomon.left-side-panel-width";
const RIGHT_SIDE_PANEL_WIDTH_KEY = "solomon.right-side-panel-width";
const TEST_CHAT_STREAM_DELAY_MS = 65;
const LOREM_WORDS = "lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod tempor incididunt ut labore et dolore magna aliqua ut enim ad minim veniam quis nostrud exercitation ullamco laboris nisi aliquip ex ea commodo consequat duis aute irure dolor in reprehenderit voluptate velit esse cillum dolore fugiat nulla pariatur".split(" ");
const EMPTY_MESSAGE_IDS = new Set<string>();

export function App() {
  const [client, setClient] = useState(initialClient);
  const [isSidePanelOpen, setIsSidePanelOpen] = useState(false);
  const [isRightSidePanelOpen, setIsRightSidePanelOpen] = useState(false);
  const [leftSidePanelWidth, setLeftSidePanelWidth] = useState(() => loadPanelWidth(LEFT_SIDE_PANEL_WIDTH_KEY));
  const [rightSidePanelWidth, setRightSidePanelWidth] = useState(() => loadPanelWidth(RIGHT_SIDE_PANEL_WIDTH_KEY));
  const [composerBounds, setComposerBounds] = useState<{ left: number; right: number } | null>(null);
  const [isTerminalPanelOpen, setIsTerminalPanelOpen] = useState(false);
  const [hasOpenedTerminalPanel, setHasOpenedTerminalPanel] = useState(false);
  const [terminalPanelHeight, setTerminalPanelHeight] = useState(DEFAULT_TERMINAL_PANEL_HEIGHT);
  const [armedTerminalProjectIds, setArmedTerminalProjectIds] = useState<string[]>([]);
  const [runningTerminalProjectIds, setRunningTerminalProjectIds] = useState<string[]>([]);
  const [activeView, setActiveView] = useState<View>("agent");
  const [isCustomizationOpen, setIsCustomizationOpen] = useState(false);
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);
  const [isNewProjectDialogOpen, setIsNewProjectDialogOpen] = useState(false);
  const [temporaryWorkspace, setTemporaryWorkspace] = useState<TemporaryWorkspace | null>(null);
  const [activeTemporaryWorkspaceID, setActiveTemporaryWorkspaceID] = useState<string | null>(null);
  const [welcomeKeepAliveHeight, setWelcomeKeepAliveHeight] = useState(0);
  const [selectedWorkspace, setSelectedWorkspace] = useState<Project | null>(null);
  const fakeChats = useTestChatStore();
  const [selectedFakeChatID, setSelectedFakeChatID] = useState<string | null>(null);
  const [isNewTestChatOpen, setIsNewTestChatOpen] = useState(false);
  const [welcomeResetToken, setWelcomeResetToken] = useState(0);
  const [streamingFakeChatIDs, setStreamingFakeChatIDs] = useState<Set<string>>(() => new Set());
  const [pendingFakeChatMessageIDs, setPendingFakeChatMessageIDs] = useState<Map<string, Set<string>>>(() => new Map());
  const streamControllers = useRef(new Map<string, AbortController>());
  const queuedFakeChatMessages = useRef(new Map<string, FakeChatMessage[]>());
  const [selectedResearch, setSelectedResearch] = useState<{ project: Project; research: ProjectResearch } | null>(null);
  const [workspaceFocus, setWorkspaceFocus] = useState<{ project: Project; token: number } | null>(null);
  const [viewportHeight, setViewportHeight] = useState(() => window.innerHeight);
  const [viewportWidth, setViewportWidth] = useState(getViewportContentWidth);
  const [viewportScrollbarWidth, setViewportScrollbarWidth] = useState(getViewportScrollbarWidth);
  const maxTerminalPanelHeight = Math.max(
    MIN_TERMINAL_PANEL_HEIGHT,
    viewportHeight - (welcomeKeepAliveHeight > 0 ? welcomeKeepAliveHeight : FALLBACK_KEEP_ALIVE_HEIGHT),
  );

  useEffect(() => {
    void detectClient().then(setClient);
  }, []);

  useEffect(() => {
    applyTheme(savedTheme());
  }, []);

  useEffect(() => {
    const onResize = () => {
      setViewportHeight(window.innerHeight);
      setViewportWidth(getViewportContentWidth());
      setViewportScrollbarWidth(getViewportScrollbarWidth());
    };
    window.addEventListener("resize", onResize);
    const resizeObserver = new ResizeObserver(onResize);
    resizeObserver.observe(document.documentElement);
    return () => {
      window.removeEventListener("resize", onResize);
      resizeObserver.disconnect();
    };
  }, []);

  useEffect(() => {
    window.localStorage.setItem(LEFT_SIDE_PANEL_WIDTH_KEY, String(leftSidePanelWidth));
  }, [leftSidePanelWidth]);

  useEffect(() => {
    window.localStorage.setItem(RIGHT_SIDE_PANEL_WIDTH_KEY, String(rightSidePanelWidth));
  }, [rightSidePanelWidth]);

  useEffect(() => {
    setTerminalPanelHeight((height) => Math.min(height, maxTerminalPanelHeight));
  }, [maxTerminalPanelHeight]);

  function goHome() {
    setIsNewProjectDialogOpen(false);
    setWelcomeResetToken((current) => current + 1);
    setIsSettingsOpen(false);
    setIsCustomizationOpen(false);
    setActiveView("agent");
    setIsTerminalPanelOpen(false);
    setSelectedFakeChatID(null);
    setIsNewTestChatOpen(false);
    setSelectedResearch(null);
  }

  function openNewProjectDialog() {
    setIsNewProjectDialogOpen(true);
  }

  function closeNewProjectDialog() {
    setIsNewProjectDialogOpen(false);
  }

  function selectLocalFolder(selection: LocalFolderSelection) {
    const nextWorkspace = {
      ...selection,
      id: `temporary-local:${selection.path || "home"}`,
    };
    setTemporaryWorkspace(nextWorkspace);
    setActiveTemporaryWorkspaceID(nextWorkspace.id);
    setIsNewProjectDialogOpen(false);
    setWelcomeResetToken((current) => current + 1);
    setSelectedFakeChatID(null);
    setIsNewTestChatOpen(false);
    setSelectedResearch(null);
    setSelectedWorkspace(null);
    setWorkspaceFocus(null);
  }

  function openTemporaryWorkspace() {
    if (!temporaryWorkspace) return;
    setActiveTemporaryWorkspaceID(temporaryWorkspace.id);
    setWelcomeResetToken((current) => current + 1);
    setActiveView("agent");
    setSelectedFakeChatID(null);
    setIsNewTestChatOpen(false);
    setSelectedResearch(null);
    setSelectedWorkspace(null);
    setWorkspaceFocus(null);
  }

  function selectTemporaryWorkspacePath(worktreePath: string) {
    if (!temporaryWorkspace) return;
    const normalizedPath = normalizeTemporaryWorkspacePath(worktreePath);
    const pathParts = normalizedPath.split("/").filter(Boolean);
    setTemporaryWorkspace((current) => current ? {
      ...current,
      displayPath: normalizedPath ? `~/${normalizedPath}/` : "~/",
      name: pathParts.at(-1) ?? "Home",
      path: normalizedPath,
    } : current);
    setActiveTemporaryWorkspaceID(temporaryWorkspace.id);
    setWelcomeResetToken((current) => current + 1);
  }

  function openSettings() {
    setIsSettingsOpen(true);
    setIsCustomizationOpen(false);
    setIsRightSidePanelOpen(false);
    setIsTerminalPanelOpen(false);
  }

  function toggleCustomization() {
    setIsCustomizationOpen((open) => {
      if (!open) setIsRightSidePanelOpen(false);
      return !open;
    });
  }

  function handleTerminalHeightChange(height: number) {
    setTerminalPanelHeight(Math.min(maxTerminalPanelHeight, Math.max(MIN_TERMINAL_PANEL_HEIGHT, height)));
  }

  function openProjectNewChat(project: Project) {
    setWelcomeResetToken((current) => current + 1);
    setIsCustomizationOpen(false);
    setActiveView("agent");
    setActiveTemporaryWorkspaceID(null);
    setSelectedWorkspace(project);
    setWorkspaceFocus({ project, token: Date.now() });
    setSelectedFakeChatID(null);
    setIsNewTestChatOpen(false);
    setSelectedResearch(null);
  }

  function openProjectTerminal(project: Project) {
    setIsCustomizationOpen(false);
    setActiveView("agent");
    setActiveTemporaryWorkspaceID(null);
    setSelectedWorkspace(project);
    setWorkspaceFocus({ project, token: Date.now() });
    setSelectedFakeChatID(null);
    setIsNewTestChatOpen(false);
    setSelectedResearch(null);
    setHasOpenedTerminalPanel(true);
    setIsTerminalPanelOpen(true);
  }

  function openNewFakeChat() {
    setWelcomeResetToken((current) => current + 1);
    setIsCustomizationOpen(false);
    setActiveView("agent");
    setActiveTemporaryWorkspaceID(null);
    setSelectedFakeChatID(null);
    setIsNewTestChatOpen(true);
    setSelectedResearch(null);
    setSelectedWorkspace(null);
    setWorkspaceFocus(null);
  }

  function sendNewFakeChatMessage(content: string) {
    const chat = createTestChat();
    setIsNewTestChatOpen(false);
    setSelectedFakeChatID(chat.id);
    sendFakeChatMessage(chat.id, { createdAt: Date.now(), id: `user-${Date.now()}`, role: "user", content });
  }

  function sendTemporaryWorkspaceMessage(content: string) {
    if (!temporaryWorkspace) return;
    const chat = createTemporaryWorkspaceChat(temporaryWorkspace.id);
    setIsNewTestChatOpen(false);
    setSelectedFakeChatID(chat.id);
    setSelectedResearch(null);
    setSelectedWorkspace(null);
    setWorkspaceFocus(null);
    sendFakeChatMessage(chat.id, { createdAt: Date.now(), id: `user-${Date.now()}`, role: "user", content });
  }

  function sendFakeChatMessage(chatID: string, message: FakeChatMessage) {
    const chat = getTestChat(chatID);
    if (!chat) return;
    const isStreamingTestChat = chat.isNewTestChat === true;
    if (!isStreamingTestChat) {
      updateTestChat(chatID, (current) => ({
        ...current,
        messages: [...current.messages, message, fakeAssistantReply(message.content)],
      }));
      return;
    }

    updateTestChat(chatID, (current) => ({
      ...current,
      messages: [...current.messages, message],
    }));

    if (streamControllers.current.has(chatID)) {
      markFakeChatMessagePending(chatID, message.id);
      const queue = queuedFakeChatMessages.current.get(chatID) ?? [];
      queue.push(message);
      queuedFakeChatMessages.current.set(chatID, queue);
      return;
    }

    startFakeChatStream(chatID, message);
  }

  function deleteFakeChatMessage(chatID: string, messageID: string) {
    const chat = getTestChat(chatID);
    if (!chat) return;
    const messageIndex = chat.messages.findIndex((message) => message.id === messageID && message.role === "user");
    if (messageIndex === -1) return;

    const assistantResponse = chat.messages[messageIndex + 1]?.role === "assistant" ? chat.messages[messageIndex + 1] : undefined;
    const messageIDs = new Set([messageID, ...(assistantResponse ? [assistantResponse.id] : [])]);
    updateTestChat(chatID, (current) => ({
      ...current,
      messages: current.messages.filter((message) => !messageIDs.has(message.id)),
    }));

    const queuedMessages = queuedFakeChatMessages.current.get(chatID);
    if (queuedMessages?.some((message) => message.id === messageID)) {
      const nextQueue = queuedMessages.filter((message) => message.id !== messageID);
      if (nextQueue.length) queuedFakeChatMessages.current.set(chatID, nextQueue);
      else queuedFakeChatMessages.current.delete(chatID);
      setPendingFakeChatMessageIDs((current) => {
        const previous = current.get(chatID);
        if (!previous?.has(messageID)) return current;
        const next = new Map(current);
        const nextIDs = new Set(previous);
        nextIDs.delete(messageID);
        if (nextIDs.size) next.set(chatID, nextIDs);
        else next.delete(chatID);
        return next;
      });
    }

    const latestAssistantID = [...chat.messages].reverse().find((message) => message.role === "assistant")?.id;
    if (assistantResponse?.id === latestAssistantID) streamControllers.current.get(chatID)?.abort();
  }

  function startFakeChatStream(chatID: string, message: FakeChatMessage) {
    const assistantID = `assistant-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
    const startedAt = Date.now();
    updateTestChat(chatID, (current) => ({
      ...current,
      messages: [...current.messages, { createdAt: Date.now(), id: assistantID, role: "assistant", content: "" }],
    }));

    const words = loremWordsForUserMessage(message.content);
    const controller = new AbortController();
    streamControllers.current.set(chatID, controller);
    setStreamingFakeChatIDs((current) => new Set(current).add(chatID));
    void streamLoremResponse(chatID, assistantID, words, controller, startedAt);
  }

  function stopFakeChatStream(chatID: string) {
    const controller = streamControllers.current.get(chatID);
    if (!controller) return;

    const chat = getTestChat(chatID);
    const assistantID = [...(chat?.messages ?? [])].reverse().find((message) => message.role === "assistant")?.id;
    if (assistantID) {
      updateTestChat(chatID, (current) => ({
        ...current,
        messages: current.messages.map((message) => (
          message.id === assistantID ? { ...message, status: "interrupted" } : message
        )),
      }));
    }

    controller.abort();
  }

  async function streamLoremResponse(chatID: string, assistantID: string, words: string[], controller: AbortController, startedAt: number) {
    try {
      for (let index = 0; index < words.length; index += 1) {
        await waitForStreamDelay(controller.signal);
        if (controller.signal.aborted) return;
        const content = words.slice(0, index + 1).join(" ");
        updateTestChat(chatID, (current) => ({
          ...current,
          messages: current.messages.map((message) => (
            message.id === assistantID ? { ...message, content } : message
          )),
        }));
      }
    } finally {
      if (streamControllers.current.get(chatID) !== controller) return;
      streamControllers.current.delete(chatID);
      updateTestChat(chatID, (current) => ({
        ...current,
        messages: current.messages.map((message) => (
          message.id === assistantID
            ? { ...message, stats: estimateFakeStats(message.content), workedFor: (Date.now() - startedAt) / 1000 }
            : message
        )),
      }));

      const queue = queuedFakeChatMessages.current.get(chatID);
      const nextMessage = queue?.shift();
      if (queue?.length === 0) queuedFakeChatMessages.current.delete(chatID);
      if (nextMessage) {
        markFakeChatMessageComplete(chatID, nextMessage.id);
        startFakeChatStream(chatID, nextMessage);
        return;
      }

      setStreamingFakeChatIDs((current) => {
        const next = new Set(current);
        next.delete(chatID);
        return next;
      });
    }
  }

  function markFakeChatMessagePending(chatID: string, messageID: string) {
    setPendingFakeChatMessageIDs((current) => {
      const next = new Map(current);
      const messageIDs = new Set(next.get(chatID));
      messageIDs.add(messageID);
      next.set(chatID, messageIDs);
      return next;
    });
  }

  function markFakeChatMessageComplete(chatID: string, messageID: string) {
    setPendingFakeChatMessageIDs((current) => {
      const previous = current.get(chatID);
      if (!previous?.has(messageID)) return current;
      const next = new Map(current);
      const messageIDs = new Set(previous);
      messageIDs.delete(messageID);
      if (messageIDs.size) next.set(chatID, messageIDs);
      else next.delete(chatID);
      return next;
    });
  }

  function openFakeChat(chatID: string) {
    setWelcomeResetToken((current) => current + 1);
    setIsCustomizationOpen(false);
    setActiveView("agent");
    const chat = getTestChat(chatID);
    setActiveTemporaryWorkspaceID(chat?.workspaceID ?? null);
    setSelectedFakeChatID(chatID);
    setIsNewTestChatOpen(false);
    setSelectedResearch(null);
    setSelectedWorkspace(null);
    setWorkspaceFocus(null);
  }

  const selectedFakeChat = fakeChats.find((chat) => chat.id === selectedFakeChatID) ?? null;
  const selectedTemporaryWorkspace = selectedFakeChat?.workspaceID === temporaryWorkspace?.id ? temporaryWorkspace : null;
  const isTestChatsExplorerActive = Boolean(selectedFakeChat) || isNewTestChatOpen;
  const workspaceNameOverride = selectedTemporaryWorkspace?.name ?? (isTestChatsExplorerActive ? "Test chats" : null);

  const handleWorkspaceChange = useCallback((project: Project | null) => {
    setSelectedWorkspace(project);
    if (project) setActiveTemporaryWorkspaceID(null);
  }, []);

  const handleComposerBoundsChange = useCallback((bounds: { left: number; right: number }) => {
    setComposerBounds((current) => (
      current?.left === bounds.left && current.right === bounds.right ? current : bounds
    ));
  }, []);

  function handleProjectTerminalArmedChange(projectId: string, armed: boolean) {
    setArmedTerminalProjectIds((current) => {
      const isArmed = current.includes(projectId);
      if (armed === isArmed) return current;
      return armed ? [...current, projectId] : current.filter((id) => id !== projectId);
    });
  }

  function handleProjectTerminalRunningChange(projectId: string, running: boolean) {
    setRunningTerminalProjectIds((current) => {
      const isRunning = current.includes(projectId);
      if (running === isRunning) return current;
      return running ? [...current, projectId] : current.filter((id) => id !== projectId);
    });
  }

  const preferredLeftWidth = isSidePanelOpen ? leftSidePanelWidth : 0;
  const preferredRightWidth = isRightSidePanelOpen && !isCustomizationOpen ? rightSidePanelWidth : 0;
  const composerWidth = Math.min(MAX_COMPOSER_WIDTH, Math.max(0, viewportWidth - WELCOME_HORIZONTAL_PADDING));
  const fallbackMaximumPanelWidth = Math.max(
    0,
    Math.floor((viewportWidth - composerWidth) / 2) - TEXTBOX_SIDE_PANEL_GAP,
  );
  const maximumLeftPanelWidth = !isCustomizationOpen && composerBounds
    ? Math.max(0, Math.floor(composerBounds.left) - TEXTBOX_SIDE_PANEL_GAP)
    : fallbackMaximumPanelWidth;
  const maximumRightPanelWidth = !isCustomizationOpen && composerBounds
    ? Math.max(0, Math.floor(viewportWidth - composerBounds.right) - TEXTBOX_SIDE_PANEL_GAP)
    : fallbackMaximumPanelWidth;
  const renderedLeftPanelWidth = Math.min(preferredLeftWidth, maximumLeftPanelWidth);
  const renderedRightPanelWidth = Math.min(preferredRightWidth, maximumRightPanelWidth);

  function resizeLeftPanel(width: number) {
    const maximum = Math.max(MIN_SIDE_PANEL_WIDTH, maximumLeftPanelWidth);
    setLeftSidePanelWidth(Math.round(Math.min(maximum, Math.max(MIN_SIDE_PANEL_WIDTH, width))));
  }

  function resizeRightPanel(width: number) {
    const maximum = Math.max(MIN_SIDE_PANEL_WIDTH, maximumRightPanelWidth);
    setRightSidePanelWidth(Math.round(Math.min(maximum, Math.max(MIN_SIDE_PANEL_WIDTH, width))));
  }

  return (
    <main
      className="app-shell"
      data-client-os={client.os}
      data-client-surface={client.surface}
      style={{
        "--left-panel-width": `${isSettingsOpen ? Math.min(leftSidePanelWidth, Math.max(0, viewportWidth)) : renderedLeftPanelWidth}px`,
        "--right-panel-width": `${renderedRightPanelWidth}px`,
        "--settings-panel-width": `${Math.min(leftSidePanelWidth, Math.max(0, viewportWidth))}px`,
        "--viewport-scrollbar-width": `${viewportScrollbarWidth}px`,
      } as CSSProperties}
    >
      <div
        aria-hidden="true"
        className={`window-drag-area${isSidePanelOpen || isSettingsOpen ? " is-left-inset" : ""}${isRightSidePanelOpen && !isCustomizationOpen && !isSettingsOpen ? " is-right-inset" : ""}`}
      />
      {!isSettingsOpen ? (
        <SidePanelToggle
          isOpen={isSidePanelOpen}
          onToggle={() => setIsSidePanelOpen((open) => !open)}
        />
      ) : null}
      {!isSettingsOpen && isSidePanelOpen ? (
        <button aria-label="Go to home" className="side-panel-wordmark" onClick={goHome} type="button">
          SOLOMON
        </button>
      ) : null}
      {!isSettingsOpen ? (
        <RightSidePanelToggle
          disabled={isCustomizationOpen}
          isOpen={isRightSidePanelOpen && !isCustomizationOpen}
          onToggle={() => {
            if (isCustomizationOpen) return;
            setIsRightSidePanelOpen((open) => !open);
          }}
        />
      ) : null}
      {!isSettingsOpen && isRightSidePanelOpen && !isCustomizationOpen ? (
        <RightSidePanel
          bottomInset={isTerminalPanelOpen ? terminalPanelHeight : 0}
          onWidthChange={resizeRightPanel}
          onOpenResearch={(research) => {
            if (!selectedWorkspace) return;
            setSelectedFakeChatID(null);
            setIsNewTestChatOpen(false);
            setSelectedResearch({ project: selectedWorkspace, research });
          }}
          project={selectedWorkspace}
          testChatsActive={isTestChatsExplorerActive}
          temporaryWorkspace={activeTemporaryWorkspaceID === temporaryWorkspace?.id ? temporaryWorkspace : null}
          width={renderedRightPanelWidth}
        />
      ) : null}
      {!isSettingsOpen && isSidePanelOpen ? (
        <SidePanel
          armedTerminalProjectIds={armedTerminalProjectIds}
          bottomInset={isTerminalPanelOpen ? terminalPanelHeight : 0}
          fakeChats={fakeChats}
          isCustomizationOpen={isCustomizationOpen}
          onNewProjectChat={openProjectNewChat}
          onNewFakeChat={openNewFakeChat}
          onOpenNewProject={openNewProjectDialog}
          onOpenTemporaryWorkspace={openTemporaryWorkspace}
          onOpenFakeChat={openFakeChat}
          onOpenProjectTerminal={openProjectTerminal}
          onOpenSettings={openSettings}
          onToggleCustomization={toggleCustomization}
          onWidthChange={resizeLeftPanel}
          runningTerminalProjectIds={runningTerminalProjectIds}
          temporaryWorkspace={temporaryWorkspace}
          width={renderedLeftPanelWidth}
        />
      ) : null}
      {!isSettingsOpen && !isCustomizationOpen ? <ViewSwitch activeView={activeView} onChange={setActiveView} /> : null}
      {!isSettingsOpen ? (
        <TerminalPanelToggle
          isOpen={isTerminalPanelOpen}
          onToggle={() => {
            if (!isTerminalPanelOpen) setHasOpenedTerminalPanel(true);
            setIsTerminalPanelOpen((open) => !open);
          }}
        />
      ) : null}
      {!isSettingsOpen && hasOpenedTerminalPanel ? (
        <TerminalPanel
          height={terminalPanelHeight}
          isOpen={isTerminalPanelOpen}
          maxHeight={maxTerminalPanelHeight}
          onClose={() => setIsTerminalPanelOpen(false)}
          onHeightChange={handleTerminalHeightChange}
          onProjectArmedChange={handleProjectTerminalArmedChange}
          onProjectRunningChange={handleProjectTerminalRunningChange}
          projectId={selectedWorkspace?.id ?? (activeTemporaryWorkspaceID === temporaryWorkspace?.id ? temporaryWorkspace.id : "home")}
          workingDirectory={selectedWorkspace?.path ?? (activeTemporaryWorkspaceID === temporaryWorkspace?.id ? temporaryWorkspace.path : "")}
        />
      ) : null}
      {isSettingsOpen ? <SettingsPage onHome={goHome} /> : null}
      {!isSettingsOpen && isCustomizationOpen ? <CustomizationPage /> : null}
      <Welcome
        bottomInset={isTerminalPanelOpen ? terminalPanelHeight : 0}
        isVisible={!isSettingsOpen && !isCustomizationOpen && !selectedFakeChat && !selectedResearch}
        onComposerBoundsChange={handleComposerBoundsChange}
        onKeepAliveHeightChange={setWelcomeKeepAliveHeight}
        onOpenNewProject={openNewProjectDialog}
        onOpenTemporaryWorkspace={openTemporaryWorkspace}
        onTemporaryWorkspacePathChange={selectTemporaryWorkspacePath}
        onSend={isNewTestChatOpen
          ? sendNewFakeChatMessage
          : activeTemporaryWorkspaceID === temporaryWorkspace?.id ? sendTemporaryWorkspaceMessage : undefined}
        onWorkspaceChange={handleWorkspaceChange}
        isTemporaryWorkspaceActive={activeTemporaryWorkspaceID === temporaryWorkspace?.id}
        temporaryWorkspace={temporaryWorkspace}
        resetToken={welcomeResetToken}
        workspaceNameOverride={workspaceNameOverride}
        workspaceFocus={workspaceFocus}
      />
      <NewProjectDialog isOpen={isNewProjectDialogOpen} onConfirmLocalFolder={selectLocalFolder} onClose={closeNewProjectDialog} />
      {!isSettingsOpen && !isCustomizationOpen && selectedFakeChat ? (
        <TestChatTopbar
          breadcrumb={selectedTemporaryWorkspace?.name}
          onOpenFolder={selectedTemporaryWorkspace ? openTemporaryWorkspace : openNewFakeChat}
          title={selectedFakeChat.title}
        />
      ) : null}
      {!isSettingsOpen && !isCustomizationOpen && selectedResearch ? (
        <TestChatTopbar breadcrumb={selectedResearch.project.name} onOpenFolder={() => openProjectNewChat(selectedResearch.project)} title={selectedResearch.research.title} />
      ) : null}
      {!isSettingsOpen && !isCustomizationOpen && selectedFakeChat ? (
        <TestChatView
          bottomInset={isTerminalPanelOpen ? terminalPanelHeight : 0}
          chat={selectedFakeChat}
          isStreaming={streamingFakeChatIDs.has(selectedFakeChat.id)}
          onDeleteMessage={deleteFakeChatMessage}
          onSend={sendFakeChatMessage}
          onStopStreaming={stopFakeChatStream}
          pendingUserMessageIDs={pendingFakeChatMessageIDs.get(selectedFakeChat.id) ?? EMPTY_MESSAGE_IDS}
          workspaceName={selectedTemporaryWorkspace?.name}
          workspacePath={selectedTemporaryWorkspace?.displayPath}
        />
      ) : null}
      {!isSettingsOpen && !isCustomizationOpen && selectedResearch ? (
        <ResearchReportView bottomInset={isTerminalPanelOpen ? terminalPanelHeight : 0} project={selectedResearch.project} research={selectedResearch.research} />
      ) : null}
    </main>
  );
}

function loremWordsForUserMessage(content: string): string[] {
  const userWordCount = content.trim().split(/\s+/).filter(Boolean).length;
  const responseWordCount = userWordCount * 4;
  return Array.from({ length: responseWordCount }, (_, index) => LOREM_WORDS[index % LOREM_WORDS.length]);
}

function waitForStreamDelay(signal: AbortSignal): Promise<void> {
  return new Promise((resolve) => {
    if (signal.aborted) {
      resolve();
      return;
    }
    let timeoutID = 0;
    const stopWaiting = () => {
      window.clearTimeout(timeoutID);
      resolve();
    };
    timeoutID = window.setTimeout(() => {
      signal.removeEventListener("abort", stopWaiting);
      resolve();
    }, TEST_CHAT_STREAM_DELAY_MS);
    signal.addEventListener("abort", stopWaiting, { once: true });
  });
}

function loadPanelWidth(storageKey: string) {
  const storedWidth = Number(window.localStorage.getItem(storageKey));
  if (!Number.isFinite(storedWidth)) return DEFAULT_SIDE_PANEL_WIDTH;
  return Math.max(MIN_SIDE_PANEL_WIDTH, Math.round(storedWidth));
}

function getViewportContentWidth() {
  return document.documentElement.clientWidth || window.innerWidth;
}

function getViewportScrollbarWidth() {
  return Math.max(0, window.innerWidth - getViewportContentWidth());
}

function normalizeTemporaryWorkspacePath(value: string) {
  return value
    .trim()
    .replaceAll("\\", "/")
    .replace(/^~\/?/, "")
    .replace(/^\/+|\/+$/g, "");
}
