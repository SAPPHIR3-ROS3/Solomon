import { useAgentPages } from "./shell/useAgentPages";
import { watchImageDrops } from "./home/imageDrops";
import { normalizeTemporaryWorkspacePath } from "./projects/temporaryPath";
import { useTerminalActivity } from "./terminal-panel/useTerminalActivity";
import { useLayout } from "./shell/useLayout";
import { NotificationToasts } from "./notifications/Notifications";
import { notifyError, watchNotifications } from "./notifications/store";
import { type CSSProperties, useCallback, useEffect, useRef, useState } from "react";
import { detectClient, initialClient } from "./platform";
import { ActiveAgentsPage, prefetchActiveAgents, type ActiveAgentNode } from "./shell/ActiveAgentsPage";
import { useAppUpdate } from "./shell/useAppUpdate";
import { useChatContextMenu } from "./shell/useChatContextMenu";
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
import { hideSettings, isSettingsPath, SettingsPage, showSettings, subscribeSettingsLocation } from "./settings/SettingsPage";
import { createProjectFromFolder, resolveMessageProject, fetchProjectSidebarData, prefetchModelCatalog, prefetchProjectSidebarData, type Project, type ProjectResearch } from "./projects/projects";
import { ResearchReportView } from "./research/ResearchReportView";
import { useChatRuntime } from "./chat/useChatRuntime";
import { beginNavigation } from "./chat/navigationPerformance";
import { ChatTopbar, ChatView } from "./chat/ChatView";
import type { LocalFolderSelection, TemporaryWorkspace } from "./projects/temporaryWorkspace";
import { EditorPage } from "./editor/EditorPage";
import { watchDaemonLifecycle } from "./daemonLifecycle";
import { watchExternalLinks } from "./externalLinks";
import { LinkContextMenu } from "./shell/LinkContextMenu";

const DEFAULT_TERMINAL_PANEL_HEIGHT = 240;
const MIN_TERMINAL_PANEL_HEIGHT = 120;
const FALLBACK_KEEP_ALIVE_HEIGHT = 96;
export function App() {
  const appUpdate = useAppUpdate();
  const { armedTerminalProjectIds, runningTerminalProjectIds, handleProjectTerminalArmedChange, handleProjectTerminalRunningChange } = useTerminalActivity();
  useEffect(watchExternalLinks, []);
  useEffect(watchImageDrops, []);
  useEffect(() => watchDaemonLifecycle(() => window.location.reload()), []);
  const [client, setClient] = useState(initialClient);
  const [isSidePanelOpen, setIsSidePanelOpen] = useState(false);
  const [isRightSidePanelOpen, setIsRightSidePanelOpen] = useState(false);
  const [isTerminalPanelOpen, setIsTerminalPanelOpen] = useState(false);
  const [hasOpenedTerminalPanel, setHasOpenedTerminalPanel] = useState(false);
  const [terminalPanelHeight, setTerminalPanelHeight] = useState(DEFAULT_TERMINAL_PANEL_HEIGHT);
  const [editorFile, setEditorFile] = useState<{ projectID: string; entry: import("./projects/projects").ProjectDirectoryEntry } | null>(null);
  const [activeView, setActiveView] = useState<View>("agent");
  const { isActiveAgentsOpen, isCustomizationOpen, setIsActiveAgentsOpen, setIsCustomizationOpen } = useAgentPages();
  const [openSubagentRequest, setOpenSubagentRequest] = useState<{ chatID: string; subchatID: string; task?: string; title?: string; toolID?: string; status?: string } | null>(null);
  const [isSettingsOpen, setIsSettingsOpen] = useState(isSettingsPath);
  const [isNewProjectDialogOpen, setIsNewProjectDialogOpen] = useState(false);
  const [temporaryWorkspace, setTemporaryWorkspace] = useState<TemporaryWorkspace | null>(null);
  const [activeTemporaryWorkspaceID, setActiveTemporaryWorkspaceID] = useState<string | null>(null);
  const [welcomeKeepAliveHeight, setWelcomeKeepAliveHeight] = useState(0);
  const [selectedWorkspace, setSelectedWorkspace] = useState<Project | null>(null);
  const resolvingMessageProject = useRef(false);
  const [isResolvingMessageProject, setIsResolvingMessageProject] = useState(false);
  const [welcomeResetToken, setWelcomeResetToken] = useState(0);
  const [selectedResearch, setSelectedResearch] = useState<{ project: Project; research: ProjectResearch } | null>(null);
  const [workspaceFocus, setWorkspaceFocus] = useState<{ project: Project; token: number } | null>(null);
  const { leftSidePanelWidth, rightSidePanelWidth, viewportHeight, viewportTop, viewportWidth, viewportScrollbarWidth, handleComposerBoundsChange, maximumLeftPanelWidth, maximumRightPanelWidth, leftPanelOverlay, rightPanelOverlay, renderedLeftPanelWidth, rightPanelVisible, renderedRightPanelWidth, panelOverlayOpen, resizeLeftPanel, resizeRightPanel } = useLayout({ isSidePanelOpen, isRightSidePanelOpen, isCustomizationOpen, isSettingsOpen, activeView });
  const maxTerminalPanelHeight = Math.max(
    MIN_TERMINAL_PANEL_HEIGHT,
    viewportHeight - (welcomeKeepAliveHeight > 0 ? welcomeKeepAliveHeight : FALLBACK_KEEP_ALIVE_HEIGHT),
  );
  const chatRuntime = useChatRuntime();
  const chatContextMenu = useChatContextMenu({ onRenameChat: chatRuntime.renameChat, onDeleteChat: chatRuntime.deleteChat });
  const {
    selectedChat,
    isLoading: isChatLoading,
    error: chatError,
    streamingChatIDs,
    queuedMessages,
  } = chatRuntime;

  useEffect(() => {
    void detectClient().then(setClient);
  }, []);

  useEffect(() => subscribeSettingsLocation(() => setIsSettingsOpen(isSettingsPath())), []);

  useEffect(() => {
    applyTheme(savedTheme());
  }, []);

  useEffect(() => {
    prefetchProjectSidebarData();
    prefetchModelCatalog();
    prefetchActiveAgents();
  }, []);

  useEffect(() => {
    setTerminalPanelHeight((height) => Math.min(height, maxTerminalPanelHeight));
  }, [maxTerminalPanelHeight]);

  const closeNavigationPanels = useCallback(() => {
    if (!panelOverlayOpen && !window.matchMedia("(max-width: 720px), (pointer: coarse) and (max-width: 1024px)").matches) return;
    setIsSidePanelOpen(false);
    setIsRightSidePanelOpen(false);
  }, [panelOverlayOpen]);

  function goHome() {
    closeNavigationPanels();
    chatRuntime.clearSelection();
    setIsNewProjectDialogOpen(false);
    setWelcomeResetToken((current) => current + 1);
    hideSettings();
    setIsSettingsOpen(false);
    setIsActiveAgentsOpen(false);
    setIsCustomizationOpen(false);
    setOpenSubagentRequest(null);
    setActiveView("agent");
    setIsTerminalPanelOpen(false);
    setSelectedResearch(null);
    setSelectedWorkspace(null);
    setWorkspaceFocus(null);
  }

  function openNewProjectDialog() {
    closeNavigationPanels();
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
    closeNavigationPanels();
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
    closeNavigationPanels();
    showSettings();
    setIsSettingsOpen(true);
    setIsActiveAgentsOpen(false);
    setIsCustomizationOpen(false);
    setIsRightSidePanelOpen(false);
    setIsTerminalPanelOpen(false);
  }

  function toggleActiveAgents() {
    closeNavigationPanels();
    if (!isActiveAgentsOpen) setIsRightSidePanelOpen(false);
    setIsActiveAgentsOpen(!isActiveAgentsOpen);
  }

  function toggleCustomization() {
    closeNavigationPanels();
    if (!isCustomizationOpen) setIsRightSidePanelOpen(false);
    setIsCustomizationOpen(!isCustomizationOpen);
  }

  function handleTerminalHeightChange(height: number) {
    setTerminalPanelHeight(Math.min(maxTerminalPanelHeight, Math.max(MIN_TERMINAL_PANEL_HEIGHT, height)));
  }

  function openProjectNewChat(project: Project) {
    closeNavigationPanels();
    chatRuntime.clearSelection();
    setWelcomeResetToken((current) => current + 1);
    setIsActiveAgentsOpen(false);
    setIsCustomizationOpen(false);
    setOpenSubagentRequest(null);
    setActiveView("agent");
    setActiveTemporaryWorkspaceID(null);
    setSelectedWorkspace(project);
    setWorkspaceFocus({ project, token: Date.now() });
    setSelectedResearch(null);
  }

  function openProjectTerminal(project: Project) {
    closeNavigationPanels();
    chatRuntime.clearSelection();
    setIsActiveAgentsOpen(false);
    setIsCustomizationOpen(false);
    setOpenSubagentRequest(null);
    setActiveView("agent");
    setActiveTemporaryWorkspaceID(null);
    setSelectedWorkspace(project);
    setWorkspaceFocus({ project, token: Date.now() });
    setSelectedResearch(null);
    setHasOpenedTerminalPanel(true);
    setIsTerminalPanelOpen(true);
  }

  async function openActiveAgent(projectID: string, projectName: string, node: ActiveAgentNode) {
    const sidebar = await fetchProjectSidebarData();
    const project = sidebar.projects.find((candidate) => candidate.id === projectID);
    const chatID = node.kind === "chat" ? node.id : (node.rootChatID || node.parentChatID || "");
    if (!project || !chatID) return;
    setOpenSubagentRequest(node.kind === "subagent"
      ? { chatID, subchatID: node.id, status: node.status, task: node.task, title: node.title, toolID: node.parentToolCallID }
      : null);
    await openProjectChat({ ...project, name: projectName }, chatID);
  }

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

  useEffect(() => watchNotifications(), []);
  useEffect(() => { if (chatError) notifyError(chatError); }, [chatError]);

  const openProjectChat = useCallback(async (project: Project, chatID: string) => {
    setIsActiveAgentsOpen(false);
    setIsCustomizationOpen(false);
    setActiveView("agent");
    const isDifferentChat = selectedChat?.id !== chatID || selectedChat?.projectID !== project.id;
    if (!isDifferentChat) {
      setIsSidePanelOpen(false);
      if (panelOverlayOpen) setIsRightSidePanelOpen(false);
      closeNavigationPanels();
      return;
    }
    if (panelOverlayOpen) {
      setIsSidePanelOpen(false);
      setIsRightSidePanelOpen(false);
    }
    closeNavigationPanels();
    const navigationID = beginNavigation(project.id, chatID);
    setWelcomeResetToken((current) => current + 1);
    setActiveTemporaryWorkspaceID(null);
    setSelectedWorkspace(project);
    setWorkspaceFocus({ project, token: Date.now() });
    setSelectedResearch(null);
    await chatRuntime.openProjectChat(project, chatID, navigationID);
  }, [chatRuntime.openProjectChat, closeNavigationPanels, panelOverlayOpen, selectedChat?.id, selectedChat?.projectID]);

  useEffect(() => {
    if (!panelOverlayOpen) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      setIsSidePanelOpen(false);
      setIsRightSidePanelOpen(false);
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [panelOverlayOpen]);

  return (
    <main
      className="app-shell"
      data-active-view={activeView}
      data-compact-height={viewportHeight < 560}
      data-left-panel-overlay={leftPanelOverlay}
      data-right-panel-overlay={rightPanelOverlay}
      data-client-os={client.os}
      data-client-surface={client.surface}
      style={{
        "--visual-viewport-height": `${viewportHeight}px`,
        "--visual-viewport-top": `${viewportTop}px`,
        "--left-panel-width": `${isSettingsOpen ? Math.min(leftSidePanelWidth, Math.max(0, viewportWidth)) : renderedLeftPanelWidth}px`,
        "--right-panel-width": `${renderedRightPanelWidth}px`,
        "--settings-panel-width": `${Math.min(leftSidePanelWidth, Math.max(0, viewportWidth))}px`,
        "--viewport-scrollbar-width": `${viewportScrollbarWidth}px`,
      } as CSSProperties}
    >
      {panelOverlayOpen ? (
        <button
          aria-label="Close side panels"
          className="side-panel-backdrop"
          onClick={() => { setIsSidePanelOpen(false); setIsRightSidePanelOpen(false); }}
          type="button"
        />
      ) : null}
      <div
        aria-hidden="true"
        className={`window-drag-area${isSidePanelOpen || isSettingsOpen ? " is-left-inset" : ""}${isRightSidePanelOpen && !isCustomizationOpen && !isSettingsOpen ? " is-right-inset" : ""}`}
      />
      {!isSettingsOpen && activeView !== "editor" ? (
        <SidePanelToggle
          isOpen={isSidePanelOpen}
          onToggle={() => {
            if (leftPanelOverlay) setIsRightSidePanelOpen(false);
            setIsSidePanelOpen((open) => !open);
          }}
        />
      ) : null}
      {!isSettingsOpen && activeView !== "editor" && isSidePanelOpen ? (
        <button aria-label="Go to home" className="side-panel-wordmark" onClick={goHome} type="button">
          SOLOMON
        </button>
      ) : null}
      {!isSettingsOpen && activeView !== "editor" ? (
        <RightSidePanelToggle
          disabled={isCustomizationOpen}
          isOpen={rightPanelVisible}
          onToggle={() => {
            if (isCustomizationOpen) return;
            if (rightPanelOverlay) setIsSidePanelOpen(false);
            setIsRightSidePanelOpen((open) => !open);
          }}
        />
      ) : null}
      {!isSettingsOpen && activeView !== "editor" && rightPanelVisible ? (
        <RightSidePanel
          bottomInset={isTerminalPanelOpen ? terminalPanelHeight : 0}
          onOpenFile={(entry) => { if (!selectedWorkspace) return; closeNavigationPanels(); setEditorFile({ projectID: selectedWorkspace.id, entry }); setActiveView("editor"); }}
          onWidthChange={resizeRightPanel}
          onOpenResearch={(research) => {
            if (!selectedWorkspace) return;
            closeNavigationPanels();
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
      {!isSettingsOpen && activeView !== "editor" && isSidePanelOpen ? (
        <SidePanel
          appUpdate={appUpdate}
          armedTerminalProjectIds={armedTerminalProjectIds}
          bottomInset={isTerminalPanelOpen ? terminalPanelHeight : 0}
          isActiveAgentsOpen={isActiveAgentsOpen}
          isCustomizationOpen={isCustomizationOpen}
          onNewProjectChat={openProjectNewChat}
          onOpenNewProject={openNewProjectDialog}
          onOpenTemporaryWorkspace={openTemporaryWorkspace}
          onOpenProjectChat={openProjectChat}
          onOpenProjectTerminal={openProjectTerminal}
          onOpenChatContextMenu={chatContextMenu.open}
          onOpenSettings={openSettings}
          onToggleActiveAgents={toggleActiveAgents}
          onToggleCustomization={toggleCustomization}
          onWidthChange={resizeLeftPanel}
          runningTerminalProjectIds={runningTerminalProjectIds}
          streamingChatIDs={streamingChatIDs}
          unreadCompletedChatIDs={chatRuntime.unreadCompletedChatIDs}
          temporaryWorkspace={temporaryWorkspace}
          width={renderedLeftPanelWidth}
        />
      ) : null}
      {!isSettingsOpen && !isCustomizationOpen && !isActiveAgentsOpen ? <ViewSwitch activeView={activeView} onChange={setActiveView} /> : null}
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
      {!isSettingsOpen && !isCustomizationOpen && !isActiveAgentsOpen && activeView === "editor" ? (
        <EditorPage key={selectedWorkspace?.id ?? "no-project"} onInitialFileOpened={() => setEditorFile(null)} initialFile={editorFile?.projectID === selectedWorkspace?.id ? editorFile?.entry : null} bottomInset={isTerminalPanelOpen ? terminalPanelHeight : 0} onHome={goHome} project={selectedWorkspace} />
      ) : null}
      {isSettingsOpen ? <SettingsPage onHome={goHome} /> : null}
      {!isSettingsOpen && isActiveAgentsOpen ? (
        <ActiveAgentsPage onOpenAgent={openActiveAgent} />
      ) : null}
      {!isSettingsOpen && isCustomizationOpen ? <CustomizationPage /> : null}
      <Welcome
        bottomInset={isTerminalPanelOpen ? terminalPanelHeight : 0}
        isVisible={!isSettingsOpen && !isCustomizationOpen && !isActiveAgentsOpen && activeView === "agent" && !selectedChat && !selectedResearch}
        onComposerBoundsChange={handleComposerBoundsChange}
        onKeepAliveHeightChange={setWelcomeKeepAliveHeight}
        onOpenNewProject={openNewProjectDialog}
        onOpenTemporaryWorkspace={openTemporaryWorkspace}
        onTemporaryWorkspacePathChange={selectTemporaryWorkspacePath}
        isSending={isChatLoading || isResolvingMessageProject}
        onSend={workspaceNameOverride ? undefined : async (content, images, clips) => {
          if (resolvingMessageProject.current) throw new Error("A message is already being sent");
          resolvingMessageProject.current = true;
          setIsResolvingMessageProject(true);
          try {
            const project = await resolveMessageProject(selectedWorkspace);
            await chatRuntime.sendNewProjectMessage(project, content, images, clips);
            handleWorkspaceChange(project);
          } finally {
            resolvingMessageProject.current = false;
            setIsResolvingMessageProject(false);
          }
        }}
        onWorkspaceChange={handleWorkspaceChange}
        isTemporaryWorkspaceActive={activeTemporaryWorkspaceID === temporaryWorkspace?.id}
        temporaryWorkspace={temporaryWorkspace}
        resetToken={welcomeResetToken}
        workspaceNameOverride={workspaceNameOverride}
        workspaceFocus={workspaceFocus}
      />
      <NewProjectDialog isOpen={isNewProjectDialogOpen} onConfirmLocalFolder={selectLocalFolder} onClose={closeNewProjectDialog} />
      {!isSettingsOpen && !isCustomizationOpen && !isActiveAgentsOpen && activeView === "agent" && selectedChat ? (
        <ChatTopbar
          breadcrumb={selectedChat.workspaceName ?? selectedWorkspace?.name}
          onOpenFolder={() => {
            if (selectedWorkspace) openProjectNewChat(selectedWorkspace);
          }}
          onTitleContextMenu={selectedChat.projectID ? (x, y) => chatContextMenu.open(selectedChat.projectID!, selectedChat.id, selectedChat.title, x, y) : undefined}
          title={selectedChat.title}
        />
      ) : null}
      {!isSettingsOpen && !isCustomizationOpen && !isActiveAgentsOpen && activeView === "agent" && selectedResearch ? (
        <ChatTopbar
          breadcrumb={selectedResearch.project.name}
          onOpenFolder={() => openProjectNewChat(selectedResearch.project)}
          title={selectedResearch.research.title}
        />
      ) : null}
      {!isSettingsOpen && !isCustomizationOpen && !isActiveAgentsOpen && activeView === "agent" && selectedChat ? (
        <ChatView
          key={JSON.stringify([selectedChat.projectID, selectedChat.id])}
          bottomInset={isTerminalPanelOpen ? terminalPanelHeight : 0}
          chat={selectedChat}
          isStreaming={streamingChatIDs.has(selectedChat.id)}
          loadSubchat={chatRuntime.loadSubchat}
          onDeleteMessage={chatRuntime.deleteMessage}
          onOpenedSubagentRequest={() => setOpenSubagentRequest(null)}
          onSend={chatRuntime.sendMessage}
          onStopTool={chatRuntime.stopTool}
          onStopStreaming={chatRuntime.stopChat}
          openSubagentRequest={openSubagentRequest}
          queuedMessages={queuedMessages.get(selectedChat.id)}
          branch={selectedChat.branch}
          worktree={selectedChat.worktree}
          workspaceName={selectedChat.workspaceName ?? selectedWorkspace?.name}
          workspacePath={selectedChat.workspacePath ?? selectedWorkspace?.path}
        />
      ) : null}
      <NotificationToasts />
      <LinkContextMenu />
      {chatContextMenu.menu}
      {!isSettingsOpen && !isCustomizationOpen && !isActiveAgentsOpen && activeView === "agent" && selectedResearch ? (
        <ResearchReportView bottomInset={isTerminalPanelOpen ? terminalPanelHeight : 0} project={selectedResearch.project} research={selectedResearch.research} />
      ) : null}
    </main>
  );
}
