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
import { createProjectFromFolder, fetchProjectSidebarData, prefetchModelCatalog, prefetchProjectSidebarData, type Project, type ProjectResearch } from "./projects/projects";
import { ResearchReportView } from "./research/ResearchReportView";
import { useChatRuntime } from "./chat/useChatRuntime";
import { forgetRememberedActiveChat, getRememberedActiveChat } from "./chat/chatStore";
import { ChatTopbar, ChatView } from "./chat/ChatView";
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
  const [welcomeResetToken, setWelcomeResetToken] = useState(0);
  const [selectedResearch, setSelectedResearch] = useState<{ project: Project; research: ProjectResearch } | null>(null);
  const [workspaceFocus, setWorkspaceFocus] = useState<{ project: Project; token: number } | null>(null);
  const [viewportHeight, setViewportHeight] = useState(() => window.innerHeight);
  const [viewportWidth, setViewportWidth] = useState(getViewportContentWidth);
  const [viewportScrollbarWidth, setViewportScrollbarWidth] = useState(getViewportScrollbarWidth);
  const restoreAttemptedRef = useRef(false);
  const maxTerminalPanelHeight = Math.max(
    MIN_TERMINAL_PANEL_HEIGHT,
    viewportHeight - (welcomeKeepAliveHeight > 0 ? welcomeKeepAliveHeight : FALLBACK_KEEP_ALIVE_HEIGHT),
  );
  const chatRuntime = useChatRuntime();
  const {
    selectedChat,
    isLoading: isChatLoading,
    error: chatError,
    streamingChatIDs,
    pendingMessageIDs,
  } = chatRuntime;

  useEffect(() => {
    void detectClient().then(setClient);
  }, []);

  useEffect(() => {
    applyTheme(savedTheme());
  }, []);

  useEffect(() => {
    prefetchProjectSidebarData();
    prefetchModelCatalog();
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
    chatRuntime.clearSelection();
    setIsNewProjectDialogOpen(false);
    setWelcomeResetToken((current) => current + 1);
    setIsSettingsOpen(false);
    setIsCustomizationOpen(false);
    setActiveView("agent");
    setIsTerminalPanelOpen(false);
    setSelectedResearch(null);
    setSelectedWorkspace(null);
    setWorkspaceFocus(null);
  }

  function openNewProjectDialog() {
    setIsNewProjectDialogOpen(true);
  }

  function closeNewProjectDialog() {
    setIsNewProjectDialogOpen(false);
  }

  async function selectLocalFolder(selection: LocalFolderSelection) {
    const project = await createProjectFromFolder(selection.path);
    setTemporaryWorkspace(null);
    setActiveTemporaryWorkspaceID(null);
    setIsNewProjectDialogOpen(false);
    openProjectNewChat(project);
  }

  function openTemporaryWorkspace() {
    if (!temporaryWorkspace) return;
    chatRuntime.clearSelection();
    setActiveTemporaryWorkspaceID(temporaryWorkspace.id);
    setWelcomeResetToken((current) => current + 1);
    setActiveView("agent");
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
    chatRuntime.clearSelection();
    setWelcomeResetToken((current) => current + 1);
    setIsCustomizationOpen(false);
    setActiveView("agent");
    setActiveTemporaryWorkspaceID(null);
    setSelectedWorkspace(project);
    setWorkspaceFocus({ project, token: Date.now() });
    setSelectedResearch(null);
  }

  function openProjectTerminal(project: Project) {
    chatRuntime.clearSelection();
    setIsCustomizationOpen(false);
    setActiveView("agent");
    setActiveTemporaryWorkspaceID(null);
    setSelectedWorkspace(project);
    setWorkspaceFocus({ project, token: Date.now() });
    setSelectedResearch(null);
    setHasOpenedTerminalPanel(true);
    setIsTerminalPanelOpen(true);
  }

  const openProjectChat = useCallback(async (project: Project, chatID: string) => {
    setWelcomeResetToken((current) => current + 1);
    setIsCustomizationOpen(false);
    setActiveView("agent");
    setActiveTemporaryWorkspaceID(null);
    setSelectedWorkspace(project);
    setWorkspaceFocus({ project, token: Date.now() });
    setSelectedResearch(null);
    await chatRuntime.openProjectChat(project, chatID);
  }, [chatRuntime.openProjectChat]);

  useEffect(() => {
    if (restoreAttemptedRef.current) return;
    restoreAttemptedRef.current = true;
    const remembered = getRememberedActiveChat();
    if (!remembered) return;
    void fetchProjectSidebarData()
      .then(async (sidebar) => {
        const project = sidebar.projects.find((candidate) => candidate.id === remembered.projectID);
        const chat = project?.chats.find((candidate) => candidate.id === remembered.chatID);
        if (!project || !chat) {
          forgetRememberedActiveChat();
          return;
        }
        await openProjectChat(project, chat.id);
      })
      .catch(() => {
        // A temporary daemon outage should not prevent the normal home view.
      });
  }, [openProjectChat]);

  const selectedTemporaryWorkspace = activeTemporaryWorkspaceID === temporaryWorkspace?.id ? temporaryWorkspace : null;
  const workspaceNameOverride = selectedTemporaryWorkspace?.name ?? null;

  const handleWorkspaceChange = useCallback((project: Project | null) => {
    setSelectedWorkspace(project);
    if (project) setActiveTemporaryWorkspaceID(null);
    setWorkspaceFocus((current) => {
      if (!project) return current ? null : current;
      return current?.project.id === project.id ? current : { project, token: Date.now() };
    });
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
            chatRuntime.clearSelection();
            setSelectedResearch({
              project: selectedWorkspace,
              research,
            });
          }}
          project={selectedWorkspace}
          temporaryWorkspace={activeTemporaryWorkspaceID === temporaryWorkspace?.id ? temporaryWorkspace : null}
          width={renderedRightPanelWidth}
        />
      ) : null}
      {!isSettingsOpen && isSidePanelOpen ? (
        <SidePanel
          armedTerminalProjectIds={armedTerminalProjectIds}
          bottomInset={isTerminalPanelOpen ? terminalPanelHeight : 0}
          isCustomizationOpen={isCustomizationOpen}
          onNewProjectChat={openProjectNewChat}
          onOpenNewProject={openNewProjectDialog}
          onOpenTemporaryWorkspace={openTemporaryWorkspace}
          onOpenProjectChat={openProjectChat}
          onOpenProjectTerminal={openProjectTerminal}
          onOpenSettings={openSettings}
          onToggleCustomization={toggleCustomization}
          onWidthChange={resizeLeftPanel}
          runningTerminalProjectIds={runningTerminalProjectIds}
          streamingChatIDs={streamingChatIDs}
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
        isVisible={!isSettingsOpen && !isCustomizationOpen && !selectedChat && !selectedResearch}
        onComposerBoundsChange={handleComposerBoundsChange}
        onKeepAliveHeightChange={setWelcomeKeepAliveHeight}
        onOpenNewProject={openNewProjectDialog}
        onOpenTemporaryWorkspace={openTemporaryWorkspace}
        onTemporaryWorkspacePathChange={selectTemporaryWorkspacePath}
        isSending={isChatLoading}
        onSend={selectedWorkspace ? (content, images) => chatRuntime.sendNewProjectMessage(selectedWorkspace, content, images) : undefined}
        onWorkspaceChange={handleWorkspaceChange}
        isTemporaryWorkspaceActive={activeTemporaryWorkspaceID === temporaryWorkspace?.id}
        temporaryWorkspace={temporaryWorkspace}
        resetToken={welcomeResetToken}
        workspaceNameOverride={workspaceNameOverride}
        workspaceFocus={workspaceFocus}
      />
      <NewProjectDialog isOpen={isNewProjectDialogOpen} onConfirmLocalFolder={selectLocalFolder} onClose={closeNewProjectDialog} />
      {!isSettingsOpen && !isCustomizationOpen && selectedChat ? (
        <ChatTopbar
          breadcrumb={selectedChat.workspaceName ?? selectedWorkspace?.name}
          onOpenFolder={() => {
            if (selectedWorkspace) openProjectNewChat(selectedWorkspace);
          }}
          title={selectedChat.title}
        />
      ) : null}
      {!isSettingsOpen && !isCustomizationOpen && selectedResearch ? (
        <ChatTopbar
          breadcrumb={selectedResearch.project.name}
          onOpenFolder={() => openProjectNewChat(selectedResearch.project)}
          title={selectedResearch.research.title}
        />
      ) : null}
      {!isSettingsOpen && !isCustomizationOpen && selectedChat ? (
        <ChatView
          bottomInset={isTerminalPanelOpen ? terminalPanelHeight : 0}
          chat={selectedChat}
          isStreaming={streamingChatIDs.has(selectedChat.id)}
          loadSubchat={chatRuntime.loadSubchat}
          onDeleteMessage={chatRuntime.deleteMessage}
          onSend={chatRuntime.sendMessage}
          onStopTool={chatRuntime.stopTool}
          onStopStreaming={chatRuntime.stopChat}
          pendingUserMessageIDs={pendingMessageIDs.get(selectedChat.id) ?? EMPTY_MESSAGE_IDS}
          branch={selectedChat.branch}
          worktree={selectedChat.worktree}
          workspaceName={selectedChat.workspaceName ?? selectedWorkspace?.name}
          workspacePath={selectedChat.workspacePath ?? selectedWorkspace?.path}
        />
      ) : null}
      {isChatLoading ? <div aria-live="polite" className="app-chat-loading">Loading chat…</div> : null}
      {chatError ? (
        <div aria-live="assertive" className="app-chat-error" role="alert">
          <strong className="app-chat-error-label">Error</strong>
          <span>{chatError}</span>
        </div>
      ) : null}
      {!isSettingsOpen && !isCustomizationOpen && selectedResearch ? (
        <ResearchReportView bottomInset={isTerminalPanelOpen ? terminalPanelHeight : 0} project={selectedResearch.project} research={selectedResearch.research} />
      ) : null}
    </main>
  );
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
