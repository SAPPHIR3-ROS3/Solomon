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
import { type Project } from "./projects/projects";

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
  const [leftPanelContentWidth, setLeftPanelContentWidth] = useState(DEFAULT_SIDE_PANEL_WIDTH);
  const [rightPanelContentWidth, setRightPanelContentWidth] = useState(DEFAULT_SIDE_PANEL_WIDTH);
  const [composerBounds, setComposerBounds] = useState<{ left: number; right: number } | null>(null);
  const [isTerminalPanelOpen, setIsTerminalPanelOpen] = useState(false);
  const [hasOpenedTerminalPanel, setHasOpenedTerminalPanel] = useState(false);
  const [terminalPanelHeight, setTerminalPanelHeight] = useState(DEFAULT_TERMINAL_PANEL_HEIGHT);
  const [activeView, setActiveView] = useState<View>("agent");
  const [isCustomizationOpen, setIsCustomizationOpen] = useState(false);
  const [welcomeKeepAliveHeight, setWelcomeKeepAliveHeight] = useState(0);
  const [selectedWorkspace, setSelectedWorkspace] = useState<Project | null>(null);
  const [viewportHeight, setViewportHeight] = useState(() => window.innerHeight);
  const [viewportWidth, setViewportWidth] = useState(() => window.innerWidth);
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
      setViewportWidth(window.innerWidth);
    };
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
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
    setIsCustomizationOpen(false);
    setActiveView("agent");
    setIsTerminalPanelOpen(false);
  }

  function handleTerminalHeightChange(height: number) {
    setTerminalPanelHeight(Math.min(maxTerminalPanelHeight, Math.max(MIN_TERMINAL_PANEL_HEIGHT, height)));
  }

  const preferredLeftWidth = isSidePanelOpen ? leftSidePanelWidth : 0;
  const preferredRightWidth = isRightSidePanelOpen ? rightSidePanelWidth : 0;
  const composerWidth = Math.min(MAX_COMPOSER_WIDTH, Math.max(0, viewportWidth - WELCOME_HORIZONTAL_PADDING));
  const fallbackMaximumPanelWidth = Math.max(
    0,
    Math.floor((viewportWidth - composerWidth) / 2) - TEXTBOX_SIDE_PANEL_GAP,
  );
  const maximumLeftBoundary = composerBounds
    ? Math.max(0, Math.floor(composerBounds.left) - TEXTBOX_SIDE_PANEL_GAP)
    : fallbackMaximumPanelWidth;
  const maximumRightBoundary = composerBounds
    ? Math.max(0, Math.floor(viewportWidth - composerBounds.right) - TEXTBOX_SIDE_PANEL_GAP)
    : fallbackMaximumPanelWidth;
  const maximumLeftPanelWidth = Math.min(maximumLeftBoundary, Math.max(MIN_SIDE_PANEL_WIDTH, leftPanelContentWidth));
  const maximumRightPanelWidth = Math.min(maximumRightBoundary, Math.max(MIN_SIDE_PANEL_WIDTH, rightPanelContentWidth));
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
        "--left-panel-width": `${renderedLeftPanelWidth}px`,
        "--right-panel-width": `${renderedRightPanelWidth}px`,
      } as CSSProperties}
    >
      <div
        aria-hidden="true"
        className={`window-drag-area${isSidePanelOpen ? " is-left-inset" : ""}${isRightSidePanelOpen ? " is-right-inset" : ""}`}
      />
      <SidePanelToggle
        isOpen={isSidePanelOpen}
        onToggle={() => setIsSidePanelOpen((open) => !open)}
      />
      {isSidePanelOpen ? (
        <button aria-label="Go to home" className="side-panel-wordmark" onClick={goHome} type="button">
          SOLOMON
        </button>
      ) : null}
      <RightSidePanelToggle
        isOpen={isRightSidePanelOpen}
        onToggle={() => setIsRightSidePanelOpen((open) => !open)}
      />
      {isRightSidePanelOpen ? (
        <RightSidePanel
          bottomInset={isTerminalPanelOpen ? terminalPanelHeight : 0}
          onContentWidthChange={setRightPanelContentWidth}
          onWidthChange={resizeRightPanel}
          project={selectedWorkspace}
          width={renderedRightPanelWidth}
        />
      ) : null}
      {isSidePanelOpen ? (
        <SidePanel
          bottomInset={isTerminalPanelOpen ? terminalPanelHeight : 0}
          isCustomizationOpen={isCustomizationOpen}
          onContentWidthChange={setLeftPanelContentWidth}
          onToggleCustomization={() => setIsCustomizationOpen((open) => !open)}
          onWidthChange={resizeLeftPanel}
          width={renderedLeftPanelWidth}
        />
      ) : null}
      {!isCustomizationOpen ? <ViewSwitch activeView={activeView} onChange={setActiveView} /> : null}
      <TerminalPanelToggle
        isOpen={isTerminalPanelOpen}
        onToggle={() => {
          if (!isTerminalPanelOpen) setHasOpenedTerminalPanel(true);
          setIsTerminalPanelOpen((open) => !open);
        }}
      />
      {hasOpenedTerminalPanel ? (
        <TerminalPanel
          height={terminalPanelHeight}
          isOpen={isTerminalPanelOpen}
          maxHeight={maxTerminalPanelHeight}
          onClose={() => setIsTerminalPanelOpen(false)}
          onHeightChange={handleTerminalHeightChange}
        />
      ) : null}
      {isCustomizationOpen ? <CustomizationPage /> : null}
      {!isCustomizationOpen ? (
        <Welcome
          bottomInset={isTerminalPanelOpen ? terminalPanelHeight : 0}
          onComposerBoundsChange={(bounds) => setComposerBounds((current) => (
            current?.left === bounds.left && current.right === bounds.right ? current : bounds
          ))}
          onKeepAliveHeightChange={setWelcomeKeepAliveHeight}
          onWorkspaceChange={setSelectedWorkspace}
        />
      ) : null}
    </main>
  );
}

function loadPanelWidth(storageKey: string) {
  const storedWidth = Number(window.localStorage.getItem(storageKey));
  if (!Number.isFinite(storedWidth)) return DEFAULT_SIDE_PANEL_WIDTH;
  return Math.max(MIN_SIDE_PANEL_WIDTH, Math.round(storedWidth));
}
