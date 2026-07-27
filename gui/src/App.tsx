import { useEffect, useState } from "react";
import { detectClient, initialClient } from "./platform";
import { SidePanel } from "./shell/SidePanel";
import { SidePanelToggle } from "./shell/SidePanelToggle";
import { TerminalPanelToggle } from "./shell/TerminalPanelToggle";
import { type View, ViewSwitch } from "./shell/ViewSwitch";
import { TerminalPanel } from "./terminal-panel/TerminalPanel";
import { applyTheme, savedTheme } from "./theme";

export function App() {
  const [client, setClient] = useState(initialClient);
  const [isSidePanelOpen, setIsSidePanelOpen] = useState(true);
  const [isTerminalPanelOpen, setIsTerminalPanelOpen] = useState(false);
  const [hasOpenedTerminalPanel, setHasOpenedTerminalPanel] = useState(false);
  const [activeView, setActiveView] = useState<View>("agent");

  useEffect(() => {
    void detectClient().then(setClient);
  }, []);

  useEffect(() => {
    applyTheme(savedTheme());
  }, []);

  return (
    <main
      className="app-shell"
      data-client-os={client.os}
      data-client-surface={client.surface}
    >
      <div aria-hidden="true" className="window-drag-area" />
      <SidePanelToggle
        isOpen={isSidePanelOpen}
        onToggle={() => setIsSidePanelOpen((open) => !open)}
      />
      {isSidePanelOpen ? <SidePanel /> : null}
      <ViewSwitch activeView={activeView} onChange={setActiveView} />
      <TerminalPanelToggle
        isOpen={isTerminalPanelOpen}
        onToggle={() => {
          if (!isTerminalPanelOpen) setHasOpenedTerminalPanel(true);
          setIsTerminalPanelOpen((open) => !open);
        }}
      />
      {hasOpenedTerminalPanel ? (
        <TerminalPanel isOpen={isTerminalPanelOpen} onClose={() => setIsTerminalPanelOpen(false)} />
      ) : null}
      <section className="bootstrap-screen">
        <div className="bootstrap-message">
          <p>Solomon GUI is ready for development.</p>
          <p className="bootstrap-client-type">
            {client.surface === "desktop" ? client.os : client.surface} client
          </p>
        </div>
      </section>
    </main>
  );
}
