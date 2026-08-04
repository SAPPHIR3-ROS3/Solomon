import { type CSSProperties, useEffect, useState } from "react";
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
import { SettingsPage } from "./settings/SettingsPage";
import { type Project, type ProjectResearch } from "./projects/projects";
import { ResearchReportView } from "./research/ResearchReportView";
import { fakeAssistantReply, initialFakeChats, type FakeChat, type FakeChatMessage } from "./chat-test/fakeChats";
import { TestChatTopbar, TestChatView } from "./chat-test/TestChatView";

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

export function App() {
  const [client, setClient] = useState(initialClient);
  const [isSidePanelOpen, setIsSidePanelOpen] = useState(true);
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
  const [welcomeKeepAliveHeight, setWelcomeKeepAliveHeight] = useState(0);
  const [selectedWorkspace, setSelectedWorkspace] = useState<Project | null>(null);
  const [fakeChats, setFakeChats] = useState<FakeChat[]>(initialFakeChats);
  const [selectedFakeChatID, setSelectedFakeChatID] = useState<string | null>(null);
  const [selectedResearch, setSelectedResearch] = useState<{ project: Project; research: ProjectResearch } | null>(null);
  const [newChatFolderName, setNewChatFolderName] = useState<string | null>(null);
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
    setIsSettingsOpen(false);
    setIsCustomizationOpen(false);
    setActiveView("agent");
    setIsTerminalPanelOpen(false);
    setSelectedFakeChatID(null);
    setSelectedResearch(null);
    setNewChatFolderName(null);
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
    setIsCustomizationOpen(false);
    setActiveView("agent");
    setSelectedWorkspace(project);
    setWorkspaceFocus({ project, token: Date.now() });
    setSelectedFakeChatID(null);
    setSelectedResearch(null);
    setNewChatFolderName(null);
  }

  function openProjectTerminal(project: Project) {
    setIsCustomizationOpen(false);
    setActiveView("agent");
    setSelectedWorkspace(project);
    setWorkspaceFocus({ project, token: Date.now() });
    setSelectedFakeChatID(null);
    setSelectedResearch(null);
    setNewChatFolderName(null);
    setHasOpenedTerminalPanel(true);
    setIsTerminalPanelOpen(true);
  }

  function sendFakeChatMessage(chatID: string, message: FakeChatMessage) {
    setFakeChats((current) => current.map((chat) => (
      chat.id === chatID ? { ...chat, messages: [...chat.messages, message, fakeAssistantReply(message.content)] } : chat
    )));
  }

  function openFakeChat(chatID: string) {
    setIsCustomizationOpen(false);
    setActiveView("agent");
    setSelectedFakeChatID(chatID);
    setSelectedResearch(null);
    setSelectedWorkspace(null);
    setWorkspaceFocus(null);
    setNewChatFolderName(null);
  }

  const selectedFakeChat = fakeChats.find((chat) => chat.id === selectedFakeChatID) ?? null;
  const isTestChatsExplorerActive = Boolean(selectedFakeChat) || newChatFolderName === "Test chats";

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
            setNewChatFolderName(null);
            setSelectedResearch({ project: selectedWorkspace, research });
          }}
          project={selectedWorkspace}
          testChatsActive={isTestChatsExplorerActive}
          width={renderedRightPanelWidth}
        />
      ) : null}
      {!isSettingsOpen && isSidePanelOpen ? (
        <SidePanel
          armedTerminalProjectIds={armedTerminalProjectIds}
          bottomInset={isTerminalPanelOpen ? terminalPanelHeight : 0}
          isCustomizationOpen={isCustomizationOpen}
          onNewProjectChat={openProjectNewChat}
          onOpenFakeFolder={() => {
            setIsCustomizationOpen(false);
            setActiveView("agent");
            setSelectedFakeChatID(null);
            setSelectedResearch(null);
            setSelectedWorkspace(null);
            setWorkspaceFocus(null);
            setNewChatFolderName("Test chats");
          }}
          onOpenFakeChat={openFakeChat}
          onOpenProjectTerminal={openProjectTerminal}
          onOpenSettings={openSettings}
          onToggleCustomization={toggleCustomization}
          onWidthChange={resizeLeftPanel}
          runningTerminalProjectIds={runningTerminalProjectIds}
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
          projectId={selectedWorkspace?.id ?? null}
        />
      ) : null}
      {isSettingsOpen ? <SettingsPage onHome={goHome} /> : null}
      {!isSettingsOpen && isCustomizationOpen ? <CustomizationPage /> : null}
      {!isSettingsOpen && !isCustomizationOpen && !selectedFakeChat && !selectedResearch ? (
        <Welcome
          bottomInset={isTerminalPanelOpen ? terminalPanelHeight : 0}
          onComposerBoundsChange={(bounds) => setComposerBounds((current) => (
            current?.left === bounds.left && current.right === bounds.right ? current : bounds
          ))}
          onKeepAliveHeightChange={setWelcomeKeepAliveHeight}
          onWorkspaceChange={setSelectedWorkspace}
          workspaceNameOverride={newChatFolderName}
          workspaceFocus={workspaceFocus}
        />
      ) : null}
      {!isSettingsOpen && !isCustomizationOpen && selectedFakeChat ? (
        <TestChatTopbar
          onOpenFolder={() => {
            setIsCustomizationOpen(false);
            setActiveView("agent");
            setSelectedFakeChatID(null);
            setSelectedWorkspace(null);
            setWorkspaceFocus(null);
            setNewChatFolderName("Test chats");
          }}
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
          onSend={sendFakeChatMessage}
        />
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
