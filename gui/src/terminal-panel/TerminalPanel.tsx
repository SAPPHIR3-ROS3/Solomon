import { FitAddon } from "@xterm/addon-fit";
import { Terminal } from "@xterm/xterm";
import "@xterm/xterm/css/xterm.css";
import { useEffect, useRef, useState } from "react";
import { terminalSocketUrl } from "./terminalSocket";

const MIN_HEIGHT = 120;
const MAX_TERMINAL_PANES = 8;

type TerminalTab = {
  hasRunCommand: boolean;
  id: string;
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
  return { hasRunCommand: false, id, title: "Terminal" };
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

type TerminalPanelProps = {
  height: number;
  isOpen: boolean;
  maxHeight: number;
  onClose: () => void;
  onHeightChange: (height: number) => void;
  onProjectArmedChange: (projectId: string, armed: boolean) => void;
  projectId: string | null;
};

export function TerminalPanel({
  height,
  isOpen,
  maxHeight,
  onClose,
  onHeightChange,
  onProjectArmedChange,
  projectId,
}: TerminalPanelProps) {
  const [isResizing, setIsResizing] = useState(false);
  const [sessions, setSessions] = useState<Record<string, ProjectTerminalSession>>({});
  const armedNotifyRef = useRef(onProjectArmedChange);
  const knownArmedRef = useRef<Set<string>>(new Set());
  armedNotifyRef.current = onProjectArmedChange;
  const panelMaxHeight = Math.max(MIN_HEIGHT, maxHeight);

  useEffect(() => {
    if (!projectId || !isOpen) return;
    setSessions((current) => (current[projectId] ? current : { ...current, [projectId]: createProjectSession() }));
  }, [isOpen, projectId]);

  useEffect(() => {
    const nextArmed = new Set(
      Object.entries(sessions).flatMap(([id, session]) => (sessionHasArmedTerminal(session) ? [id] : [])),
    );
    const previousArmed = knownArmedRef.current;
    for (const id of previousArmed) {
      if (!nextArmed.has(id)) armedNotifyRef.current(id, false);
    }
    for (const id of nextArmed) {
      if (!previousArmed.has(id)) armedNotifyRef.current(id, true);
    }
    knownArmedRef.current = nextArmed;
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
                    </div>
                  </div>
                  <div className="terminal-panel-pane">
                    {pane.tabs.map((tab) => (
                      <IntegratedShell
                        key={`${sessionProjectId}:${tab.id}`}
                        onCommandSubmit={() => markCommandRun(sessionProjectId, tab.id)}
                        tabId={`${sessionProjectId}:${tab.id}`}
                        visible={isActiveSession && tab.id === pane.activeTabId}
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

function IntegratedShell({
  onCommandSubmit,
  tabId,
  visible,
}: {
  onCommandSubmit: () => void;
  tabId: string;
  visible: boolean;
}) {
  const hostRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<Terminal | null>(null);
  const resizeRef = useRef<(() => void) | null>(null);
  const visibleRef = useRef(visible);
  const commandSubmitRef = useRef(onCommandSubmit);
  commandSubmitRef.current = onCommandSubmit;

  useEffect(() => {
    visibleRef.current = visible;
    if (!visible) return;
    requestAnimationFrame(() => {
      resizeRef.current?.();
      termRef.current?.focus();
    });
  }, [visible]);

  useEffect(() => {
    const host = hostRef.current;
    if (!host) return;

    const term = new Terminal({
      cursorBlink: true,
      fontFamily: '"Geist Mono", ui-monospace, monospace',
      fontSize: 12,
      lineHeight: 1.45,
      scrollback: 5000,
      theme: terminalTheme(),
    });
    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    term.open(host);
    termRef.current = term;

    let disposed = false;
    let retryTimer: number | undefined;
    let socket: WebSocket | undefined;
    let attempts = 0;
    let retryQueued = false;

    const sendResize = () => {
      if (!socket || socket.readyState !== WebSocket.OPEN) return;
      fitAddon.fit();
      socket.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }));
    };
    resizeRef.current = sendResize;

    const connect = () => {
      const nextSocket = new WebSocket(terminalSocketUrl());
      socket = nextSocket;
      nextSocket.binaryType = "arraybuffer";

      const retry = () => {
        if (disposed || socket !== nextSocket || retryQueued) return;
        retryQueued = true;
        if (attempts >= 2) {
          term.write("\r\n[terminal connection failed]\r\n");
          return;
        }
        attempts += 1;
        retryTimer = window.setTimeout(() => {
          retryQueued = false;
          connect();
        }, 250);
      };

      nextSocket.onmessage = (event) => {
        if (typeof event.data === "string") term.write(event.data);
        else term.write(new Uint8Array(event.data as ArrayBuffer));
      };
      nextSocket.onopen = () => {
        attempts = 0;
        requestAnimationFrame(() => {
          if (!visibleRef.current) return;
          sendResize();
          term.focus();
        });
      };
      nextSocket.onerror = retry;
      nextSocket.onclose = retry;
    };
    connect();

    const inputSubscription = term.onData((data) => {
      if (data.includes("\r") || data.includes("\n")) commandSubmitRef.current();
      if (socket?.readyState === WebSocket.OPEN) socket.send(data);
    });
    const observer = new ResizeObserver(() => {
      if (visibleRef.current) sendResize();
    });
    observer.observe(host);

    return () => {
      disposed = true;
      if (retryTimer !== undefined) window.clearTimeout(retryTimer);
      inputSubscription.dispose();
      observer.disconnect();
      termRef.current = null;
      resizeRef.current = null;
      socket?.close();
      term.dispose();
    };
  }, [tabId]);

  return <div aria-hidden={!visible} className={`terminal-panel-host${visible ? " is-visible" : ""}`} ref={hostRef} />;
}

function terminalTheme() {
  const style = getComputedStyle(document.documentElement);
  const color = (name: string, fallback: string) => style.getPropertyValue(name).trim() || fallback;

  return {
    background: color("--color-canvas", "#061c3b"),
    black: color("--color-canvas", "#061c3b"),
    blue: color("--color-accent", "#3b8fd1"),
    brightBlack: color("--color-text-muted", "#9ca3aa"),
    brightBlue: color("--focus-ring", "#86c9f2"),
    brightCyan: color("--focus-ring", "#86c9f2"),
    brightGreen: color("--color-success", "#237a52"),
    brightRed: color("--color-danger", "#a83b3b"),
    brightWhite: color("--color-text", "#e8e5df"),
    brightYellow: color("--color-crown-gold", "#ffc704"),
    cursor: color("--focus-ring", "#86c9f2"),
    cursorAccent: color("--color-canvas", "#061c3b"),
    cyan: color("--focus-ring", "#86c9f2"),
    foreground: color("--color-text", "#e8e5df"),
    green: color("--color-success", "#237a52"),
    red: color("--color-danger", "#a83b3b"),
    selectionBackground: color("--color-surface-raised", "#0d3566"),
    white: color("--color-text", "#e8e5df"),
    yellow: color("--color-crown-gold", "#ffc704"),
  };
}

function TerminalIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="m7 11 2-2-2-2" />
      <path d="M11 13h4" />
      <rect height="18" rx="2" ry="2" width="18" x="3" y="3" />
    </svg>
  );
}

function TrashIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="M3 6h18" />
      <path d="M8 6V4h8v2" />
      <path d="m19 6-1 14H6L5 6" />
      <path d="M10 11v6M14 11v6" />
    </svg>
  );
}

function PlusIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="M12 5v14M5 12h14" />
    </svg>
  );
}

function SplitIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <rect height="18" rx="2" width="18" x="3" y="3" />
      <path d="M12 3v18" />
    </svg>
  );
}
