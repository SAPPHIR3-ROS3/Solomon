import { useEffect, useRef, useState } from "react";
import { TerminalPanelIcon } from "../shell/TerminalPanelToggle";
import { IntegratedShell } from "./IntegratedShell";

function TerminalIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="m7 11 2-2-2-2" /><path d="M11 13h4" /><rect height="18" rx="2" ry="2" width="18" x="3" y="3" /></svg>;
}

function TrashIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M3 6h18" /><path d="M8 6V4h8v2" /><path d="m19 6-1 14H6L5 6" /><path d="M10 11v6M14 11v6" /></svg>;
}

function PlusIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M12 5v14M5 12h14" /></svg>;
}

function SplitIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><rect height="18" rx="2" width="18" x="3" y="3" /><path d="M12 3v18" /></svg>;
}

const MIN_HEIGHT = 120;
const MAX_TERMINAL_PANES = 8;

type TerminalTab = {
  hasRunCommand: boolean;
  id: string;
  isRunning: boolean;
  title: string;
};

type TerminalPane = {
  id: string;
  tabs: TerminalTab[];
  activeTabId: string;
};

type ProjectTerminalSession = {
  nextPaneId: number;
  nextTabId: number;
  panes: TerminalPane[];
};

function createTerminalTab(id: string): TerminalTab {
  return { hasRunCommand: false, id, isRunning: false, title: "Terminal" };
}

function createTerminalPane(id: string, tabId: string): TerminalPane {
  const tab = createTerminalTab(tabId);
  return { id, tabs: [tab], activeTabId: tab.id };
}

function createProjectSession(): ProjectTerminalSession {
  return {
    nextPaneId: 1,
    nextTabId: 1,
    panes: [createTerminalPane("terminal-pane-0", "terminal-tab-0")],
  };
}

function sessionHasArmedTerminal(session: ProjectTerminalSession) {
  return session.panes.some((pane) => pane.tabs.some((tab) => tab.hasRunCommand));
}

function sessionHasRunningTerminal(session: ProjectTerminalSession) {
  return session.panes.some((pane) => pane.tabs.some((tab) => tab.isRunning));
}

type TerminalPanelProps = {
  height: number;
  isOpen: boolean;
  maxHeight: number;
  onClose: () => void;
  onHeightChange: (height: number) => void;
  onProjectArmedChange: (projectId: string, armed: boolean) => void;
  onProjectRunningChange: (projectId: string, running: boolean) => void;
  projectId: string | null;
  workingDirectory?: string;
};

export function TerminalPanel({
  height,
  isOpen,
  maxHeight,
  onClose,
  onHeightChange,
  onProjectArmedChange,
  onProjectRunningChange,
  projectId,
  workingDirectory = "",
}: TerminalPanelProps) {
  const [isResizing, setIsResizing] = useState(false);
  const [sessions, setSessions] = useState<Record<string, ProjectTerminalSession>>({});
  const armedNotifyRef = useRef(onProjectArmedChange);
  const runningNotifyRef = useRef(onProjectRunningChange);
  const knownArmedRef = useRef<Set<string>>(new Set());
  const knownRunningRef = useRef<Set<string>>(new Set());
  armedNotifyRef.current = onProjectArmedChange;
  runningNotifyRef.current = onProjectRunningChange;
  const panelMaxHeight = Math.max(MIN_HEIGHT, maxHeight);

  useEffect(() => {
    if (!projectId || !isOpen) return;
    setSessions((current) => (current[projectId] ? current : { ...current, [projectId]: createProjectSession() }));
  }, [isOpen, projectId]);

  useEffect(() => {
    const syncFlag = (
      previous: Set<string>,
      next: Set<string>,
      notify: (projectId: string, active: boolean) => void,
    ) => {
      for (const id of previous) if (!next.has(id)) notify(id, false);
      for (const id of next) if (!previous.has(id)) notify(id, true);
    };
    const entries = Object.entries(sessions);
    const nextArmed = new Set(entries.flatMap(([id, session]) => (sessionHasArmedTerminal(session) ? [id] : [])));
    const nextRunning = new Set(entries.flatMap(([id, session]) => (sessionHasRunningTerminal(session) ? [id] : [])));
    syncFlag(knownArmedRef.current, nextArmed, armedNotifyRef.current);
    syncFlag(knownRunningRef.current, nextRunning, runningNotifyRef.current);
    knownArmedRef.current = nextArmed;
    knownRunningRef.current = nextRunning;
  }, [sessions]);

  function startResize(event: React.PointerEvent<HTMLButtonElement>) {
    event.preventDefault();
    const startY = event.clientY;
    const startHeight = height;
    setIsResizing(true);

    const onPointerMove = (moveEvent: PointerEvent) => {
      onHeightChange(Math.min(panelMaxHeight, Math.max(MIN_HEIGHT, startHeight + startY - moveEvent.clientY)));
    };
    const stopResize = () => {
      setIsResizing(false);
      window.removeEventListener("pointermove", onPointerMove);
      window.removeEventListener("pointerup", stopResize);
    };

    window.addEventListener("pointermove", onPointerMove);
    window.addEventListener("pointerup", stopResize, { once: true });
  }

  function updateSession(targetProjectId: string, update: (session: ProjectTerminalSession) => ProjectTerminalSession) {
    setSessions((current) => {
      const session = current[targetProjectId];
      if (!session) return current;
      return { ...current, [targetProjectId]: update(session) };
    });
  }

  function updatePane(targetProjectId: string, paneId: string, update: (pane: TerminalPane) => TerminalPane) {
    updateSession(targetProjectId, (session) => ({
      ...session,
      panes: session.panes.map((pane) => (pane.id === paneId ? update(pane) : pane)),
    }));
  }

  function addTerminalTab(targetProjectId: string, paneId: string) {
    updateSession(targetProjectId, (session) => {
      const tabId = `terminal-tab-${session.nextTabId}`;
      const tab = createTerminalTab(tabId);
      return {
        ...session,
        nextTabId: session.nextTabId + 1,
        panes: session.panes.map((pane) => (
          pane.id === paneId ? { ...pane, tabs: [...pane.tabs, tab], activeTabId: tab.id } : pane
        )),
      };
    });
  }

  function addTerminalPane(targetProjectId: string) {
    updateSession(targetProjectId, (session) => {
      if (session.panes.length >= MAX_TERMINAL_PANES) return session;
      const paneId = `terminal-pane-${session.nextPaneId}`;
      const tabId = `terminal-tab-${session.nextTabId}`;
      return {
        ...session,
        nextPaneId: session.nextPaneId + 1,
        nextTabId: session.nextTabId + 1,
        panes: [...session.panes, createTerminalPane(paneId, tabId)],
      };
    });
  }

  function closeTerminalTab(targetProjectId: string, paneId: string, tabId: string) {
    const session = sessions[targetProjectId];
    if (!session) return;
    const pane = session.panes.find((candidate) => candidate.id === paneId);
    if (!pane) return;
    const removesProjectSession = pane.tabs.length === 1 && session.panes.length === 1;

    setSessions((current) => {
      const latest = current[targetProjectId];
      if (!latest) return current;
      const latestPane = latest.panes.find((candidate) => candidate.id === paneId);
      if (!latestPane) return current;
      if (latestPane.tabs.length === 1 && latest.panes.length === 1) {
        const { [targetProjectId]: _removed, ...rest } = current;
        return rest;
      }

      const panes = latest.panes.flatMap((candidate) => {
        if (candidate.id !== paneId) return [candidate];
        const index = candidate.tabs.findIndex((tab) => tab.id === tabId);
        const tabs = candidate.tabs.filter((tab) => tab.id !== tabId);
        if (!tabs.length) return [];
        const activeTabId = candidate.activeTabId === tabId
          ? tabs[Math.min(index, tabs.length - 1)].id
          : candidate.activeTabId;
        return [{ ...candidate, tabs, activeTabId }];
      });
      return { ...current, [targetProjectId]: { ...latest, panes } };
    });

    if (removesProjectSession && targetProjectId === projectId) onClose();
  }

  function markCommandRun(targetProjectId: string, tabId: string) {
    updateSession(targetProjectId, (session) => ({
      ...session,
      panes: session.panes.map((pane) => ({
        ...pane,
        tabs: pane.tabs.map((tab) => (
          tab.id === tabId && !tab.hasRunCommand ? { ...tab, hasRunCommand: true } : tab
        )),
      })),
    }));
  }

  function setTabRunning(targetProjectId: string, tabId: string, isRunning: boolean) {
    updateSession(targetProjectId, (session) => ({
      ...session,
      panes: session.panes.map((pane) => ({
        ...pane,
        tabs: pane.tabs.map((tab) => (tab.id === tabId && tab.isRunning !== isRunning ? { ...tab, isRunning } : tab)),
      })),
    }));
  }

  const sessionEntries = Object.entries(sessions);

  return (
    <section
      aria-label="Terminal panel"
      className={`terminal-panel${isOpen ? "" : " is-hidden"}${isResizing ? " is-resizing" : ""}`}
      style={{ height }}
    >
      <button
        aria-label="Resize terminal panel"
        className="terminal-panel-resize"
        onDoubleClick={() => onHeightChange(Math.min(panelMaxHeight, 240))}
        onPointerDown={startResize}
        title="Drag to resize terminal panel"
        type="button"
      />
      {sessionEntries.map(([sessionProjectId, session]) => {
        const isActiveSession = isOpen && sessionProjectId === projectId;
        const gridColumns = session.panes.map(() => "minmax(120px, 1fr)").join(" ");
        return (
          <div
            aria-hidden={!isActiveSession}
            className={`terminal-panel-stack${isActiveSession ? "" : " is-keepalive"}`}
            key={sessionProjectId}
            style={{ gridTemplateColumns: gridColumns }}
          >
            {session.panes.map((pane, paneIndex) => {
              const isLastPane = paneIndex === session.panes.length - 1;
              return (
                <div className="terminal-panel-group" key={pane.id}>
                  <div className="terminal-panel-chrome">
                    <div className="terminal-tabs-shell">
                      <div className="terminal-tabs-scrollport">
                        <nav aria-label={`Terminal tabs ${paneIndex + 1}`} className="terminal-tabs">
                          {pane.tabs.map((tab) => (
                            <div className={`terminal-tab${tab.id === pane.activeTabId ? " is-active" : ""}`} key={tab.id}>
                              <button
                                className="terminal-tab-trigger"
                                onClick={() => updatePane(sessionProjectId, pane.id, (current) => ({ ...current, activeTabId: tab.id }))}
                                type="button"
                              >
                                <TerminalIcon />
                                <span>{tab.title}</span>
                              </button>
                              <button
                                aria-label={`Close ${tab.title}`}
                                className="terminal-tab-close"
                                onClick={() => closeTerminalTab(sessionProjectId, pane.id, tab.id)}
                                title={`Close ${tab.title}`}
                                type="button"
                              >
                                <TrashIcon />
                              </button>
                            </div>
                          ))}
                        </nav>
                      </div>
                    </div>
                    <div aria-label={`Terminal actions ${paneIndex + 1}`} className="terminal-panel-actions">
                      <button aria-label="New terminal" onClick={() => addTerminalTab(sessionProjectId, pane.id)} title="New terminal" type="button">
                        <PlusIcon />
                      </button>
                      {isLastPane && (
                        <button
                          aria-label="Split terminal"
                          disabled={session.panes.length >= MAX_TERMINAL_PANES}
                          onClick={() => addTerminalPane(sessionProjectId)}
                          title="Split terminal"
                          type="button"
                        >
                          <SplitIcon />
                        </button>
                      )}
                      {isLastPane && (
                        <button
                          aria-label="Close terminal panel"
                          onClick={onClose}
                          title="Close terminal panel"
                          type="button"
                        >
                          <TerminalPanelIcon />
                        </button>
                      )}
                    </div>
                  </div>
                  <div className="terminal-panel-pane">
                    {pane.tabs.map((tab) => (
                      <IntegratedShell
                        key={`${sessionProjectId}:${tab.id}`}
                        onCommandSubmit={() => markCommandRun(sessionProjectId, tab.id)}
                        onRunningChange={(running) => setTabRunning(sessionProjectId, tab.id, running)}
                        tabId={`${sessionProjectId}:${tab.id}`}
                        visible={isActiveSession && tab.id === pane.activeTabId}
                        workingDirectory={isActiveSession ? workingDirectory : ""}
                      />
                    ))}
                  </div>
                </div>
              );
            })}
          </div>
        );
      })}
    </section>
  );
}
