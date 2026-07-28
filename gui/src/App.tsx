import { useEffect, useState } from "react";
import { detectClient, initialClient } from "./platform";
import { SidePanel } from "./shell/SidePanel";
import { SidePanelToggle } from "./shell/SidePanelToggle";
import { TerminalPanelToggle } from "./shell/TerminalPanelToggle";
import { type View, ViewSwitch } from "./shell/ViewSwitch";
import { TerminalPanel } from "./terminal-panel/TerminalPanel";
import { applyTheme, savedTheme } from "./theme";
import { CustomizationPage } from "./customization/CustomizationPage";
import { Welcome } from "./home/Welcome";

const DEFAULT_TERMINAL_PANEL_HEIGHT = 240;
const MIN_TERMINAL_PANEL_HEIGHT = 120;
const FALLBACK_KEEP_ALIVE_HEIGHT = 96;

export function App() {
  const [client, setClient] = useState(initialClient);
  const [isSidePanelOpen, setIsSidePanelOpen] = useState(true);
  const [isTerminalPanelOpen, setIsTerminalPanelOpen] = useState(false);
  const [hasOpenedTerminalPanel, setHasOpenedTerminalPanel] = useState(false);
  const [terminalPanelHeight, setTerminalPanelHeight] = useState(DEFAULT_TERMINAL_PANEL_HEIGHT);
  const [activeView, setActiveView] = useState<View>("agent");
  const [isCustomizationOpen, setIsCustomizationOpen] = useState(false);
  const [welcomeKeepAliveHeight, setWelcomeKeepAliveHeight] = useState(0);
  const [viewportHeight, setViewportHeight] = useState(() => window.innerHeight);
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
    const onResize = () => setViewportHeight(window.innerHeight);
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, []);

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

  return (
    <main
      className="app-shell"
      data-client-os={client.os}
      data-client-surface={client.surface}
    >
      <div aria-hidden="true" className={`window-drag-area${isSidePanelOpen ? " is-inset" : ""}`} />
      <SidePanelToggle
        isOpen={isSidePanelOpen}
        onToggle={() => setIsSidePanelOpen((open) => !open)}
      />
      {isSidePanelOpen ? (
        <button aria-label="Go to home" className="side-panel-wordmark" onClick={goHome} type="button">
          SOLOMON
        </button>
      ) : null}
      {isSidePanelOpen ? (
        <SidePanel
          bottomInset={isTerminalPanelOpen ? terminalPanelHeight : 0}
          isCustomizationOpen={isCustomizationOpen}
          onToggleCustomization={() => setIsCustomizationOpen((open) => !open)}
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
          onKeepAliveHeightChange={setWelcomeKeepAliveHeight}
        />
      ) : null}
    </main>
  );
}
