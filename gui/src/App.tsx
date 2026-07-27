import { useEffect, useState } from "react";
import { detectClient, initialClient } from "./platform";
import { SidePanel } from "./shell/SidePanel";
import { SidePanelToggle } from "./shell/SidePanelToggle";
import { TerminalPanelToggle } from "./shell/TerminalPanelToggle";
import { type View, ViewSwitch } from "./shell/ViewSwitch";
import { TerminalPanel } from "./terminal-panel/TerminalPanel";
import { applyTheme, savedTheme } from "./theme";
import { CustomizationPage } from "./customization/CustomizationPage";

const DEFAULT_TERMINAL_PANEL_HEIGHT = 240;

export function App() {
  const [client, setClient] = useState(initialClient);
  const [isSidePanelOpen, setIsSidePanelOpen] = useState(true);
  const [isTerminalPanelOpen, setIsTerminalPanelOpen] = useState(false);
  const [hasOpenedTerminalPanel, setHasOpenedTerminalPanel] = useState(false);
  const [terminalPanelHeight, setTerminalPanelHeight] = useState(DEFAULT_TERMINAL_PANEL_HEIGHT);
  const [activeView, setActiveView] = useState<View>("agent");
  const [isCustomizationOpen, setIsCustomizationOpen] = useState(false);

  useEffect(() => {
    void detectClient().then(setClient);
  }, []);

  useEffect(() => {
    applyTheme(savedTheme());
  }, []);

  function goHome() {
    setIsCustomizationOpen(false);
    setActiveView("agent");
    setIsTerminalPanelOpen(false);
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
          onClose={() => setIsTerminalPanelOpen(false)}
          onHeightChange={setTerminalPanelHeight}
        />
      ) : null}
      {isCustomizationOpen ? <CustomizationPage /> : null}
      {!isCustomizationOpen ? <section className="bootstrap-screen">
        <div className="bootstrap-message">
          <p>Solomon GUI is ready for development.</p>
          <p className="bootstrap-client-type">
            {client.surface === "desktop" ? client.os : client.surface} client
          </p>
        </div>
      </section> : null}
    </main>
  );
}
