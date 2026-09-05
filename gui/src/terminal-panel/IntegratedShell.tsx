import { FitAddon } from "@xterm/addon-fit";
import { Terminal } from "@xterm/xterm";
import "@xterm/xterm/css/xterm.css";
import { useEffect, useRef, useState } from "react";
import { addTerminalClip } from "./clips";
import { selectedTerminalText, selectionLineRange } from "./selection";
import { terminalSocketUrl } from "./terminalSocket";

export function IntegratedShell({
  onCommandSubmit,
  onRunningChange,
  tabId,
  visible,
  workingDirectory,
}: {
  onCommandSubmit: () => void;
  onRunningChange: (running: boolean) => void;
  tabId: string;
  visible: boolean;
  workingDirectory: string;
}) {
  const hostRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<Terminal | null>(null);
  const resizeRef = useRef<(() => void) | null>(null);
  const visibleRef = useRef(visible);
  const commandSubmitRef = useRef(onCommandSubmit);
  const runningChangeRef = useRef(onRunningChange);
  commandSubmitRef.current = onCommandSubmit;
  runningChangeRef.current = onRunningChange;

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
    if (!host || !visible) return;

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
    const selectionSub = term.onSelectionChange(() => {
      const range = selectionLineRange(term);
      const text = selectedTerminalText(term);
      if (!range || !text.trim()) {
        setSelection(null);
        return;
      }
      setSelection({ end: range.end, start: range.start, text });
    });
    const selectionHost = document.createElement("div");
    selectionHost.className = "terminal-selection-overlay";
    host.append(selectionHost);

    let socket: WebSocket | undefined;
    let retryTimer: number | undefined;
    let attempts = 0;
    let disposed = false;
    const storedSession = loadStoredTerminalSession(tabId);
    const sessionIDRef = { current: storedSession.id };
    const outputSeqRef = { current: storedSession.seq };

    const sendResize = () => {
      if (!socket || socket.readyState !== WebSocket.OPEN) return;
      fitAddon.fit();
      socket.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }));
    };
    resizeRef.current = sendResize;

    const scheduleRetry = () => {
      if (disposed || retryTimer !== undefined) return;
      const delay = Math.min(5000, 250 * (2 ** Math.min(attempts, 4)));
      attempts += 1;
      retryTimer = window.setTimeout(() => {
        retryTimer = undefined;
        void connect();
      }, delay);
    };

    const connect = async () => {
      if (disposed) return;
      let endpoint: string;
      try {
        endpoint = await terminalSocketUrl(workingDirectory, sessionIDRef.current, outputSeqRef.current, term.cols, term.rows);
      } catch {
        scheduleRetry();
        return;
      }
      if (disposed) return;
      const nextSocket = new WebSocket(endpoint);
      socket = nextSocket;
      nextSocket.binaryType = "arraybuffer";

      nextSocket.onmessage = (event) => {
        if (typeof event.data === "string") {
          if (event.data.startsWith("{")) {
            try {
              const message = JSON.parse(event.data) as { data?: string; id?: string; running?: boolean; seq?: number; type?: string };
              if (message.type === "solomon-terminal" && typeof message.id === "string") {
                sessionIDRef.current = message.id;
                saveStoredTerminalSession(tabId, { id: message.id, seq: outputSeqRef.current });
                return;
              }
              if (message.type === "solomon-output" && typeof message.data === "string" && typeof message.seq === "number") {
                if (message.seq <= outputSeqRef.current) return;
                outputSeqRef.current = message.seq;
                saveStoredTerminalSession(tabId, { id: sessionIDRef.current, seq: outputSeqRef.current });
                term.write(decodeTerminalOutput(message.data));
                return;
              }
              if (message.type === "solomon-status" && typeof message.running === "boolean") {
                runningChangeRef.current(message.running);
                return;
              }
              if (message.type === "solomon-exit") {
                runningChangeRef.current(false);
                return;
              }
            } catch {
              // Fall through and write ordinary terminal output.
            }
          }
          term.write(event.data);
          return;
        }
        term.write(new Uint8Array(event.data as ArrayBuffer));
      };
      nextSocket.onopen = () => {
        attempts = 0;
        requestAnimationFrame(() => {
          if (!visibleRef.current) return;
          sendResize();
          term.focus();
        });
      };
      nextSocket.onerror = () => nextSocket.close();
      nextSocket.onclose = () => {
        runningChangeRef.current(false);
        if (!disposed && socket === nextSocket) scheduleRetry();
      };
    };
    void connect();

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
      selectionSub.dispose();
      observer.disconnect();
      termRef.current = null;
      resizeRef.current = null;
      socket?.close();
      term.dispose();
    };
  }, [tabId, visible, workingDirectory]);

  const [selection, setSelection] = useState<{ end: number; start: number; text: string } | null>(null);

  useEffect(() => {
    const term = termRef.current;
    if (!term || !visible) return;
    const host = hostRef.current;
    const sync = () => {
      const range = selectionLineRange(term);
      const text = selectedTerminalText(term);
      if (!range || !text.trim()) {
        setSelection(null);
        return;
      }
      setSelection({ end: range.end, start: range.start, text });
    };
    const sub = term.onSelectionChange(sync);
    host?.addEventListener("mouseup", sync);
    return () => {
      sub.dispose();
      host?.removeEventListener("mouseup", sync);
    };
  }, [tabId, visible, workingDirectory]);

  function addSelectionToChat() {
    if (!selection) return;
    addTerminalClip(selection.start, selection.end, selection.text);
    termRef.current?.clearSelection();
    setSelection(null);
  }

  return (
    <div aria-hidden={!visible} className={`terminal-panel-host${visible ? " is-visible" : ""}`} ref={hostRef}>
      {visible && selection ? (
        <button className="terminal-add-to-chat" onClick={addSelectionToChat} type="button">Add to chat</button>
      ) : null}
    </div>
  );
}

type StoredTerminalSession = {
  id: string;
  seq: number;
};

const TERMINAL_SESSION_STORAGE_PREFIX = "solomon.terminal-session.v1";

function terminalSessionStorageKey(tabId: string) {
  return `${TERMINAL_SESSION_STORAGE_PREFIX}.${tabId}`;
}

function loadStoredTerminalSession(tabId: string): StoredTerminalSession {
  try {
    const value: unknown = JSON.parse(window.localStorage.getItem(terminalSessionStorageKey(tabId)) ?? "null");
    if (!value || typeof value !== "object") return { id: "", seq: 0 };
    const record = value as Partial<StoredTerminalSession>;
    return {
      id: typeof record.id === "string" ? record.id : "",
      seq: typeof record.seq === "number" && Number.isFinite(record.seq) && record.seq > 0 ? Math.floor(record.seq) : 0,
    };
  } catch {
    return { id: "", seq: 0 };
  }
}

function saveStoredTerminalSession(tabId: string, value: StoredTerminalSession) {
  try {
    window.localStorage.setItem(terminalSessionStorageKey(tabId), JSON.stringify(value));
  } catch {
    // Terminal continuity still works for the current client when storage is unavailable.
  }
}

function decodeTerminalOutput(value: string): Uint8Array {
  const binary = window.atob(value);
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) bytes[index] = binary.charCodeAt(index);
  return bytes;
}

function terminalTheme() {
  const style = getComputedStyle(document.documentElement);
  const color = (name: string, fallback: string) => style.getPropertyValue(name).trim() || fallback;
  const canvas = color("--color-canvas", "#061c3b");
  const text = color("--color-text", "#e8e5df");
  const focus = color("--focus-ring", "#86c9f2");
  const success = color("--color-success", "#237a52");
  const danger = color("--color-danger", "#a83b3b");
  const gold = color("--color-crown-gold", "#ffc704");
  return {
    background: canvas, black: canvas, blue: color("--color-accent", "#3b8fd1"), brightBlack: color("--color-text-muted", "#9ca3aa"),
    brightBlue: focus, brightCyan: focus, brightGreen: success, brightRed: danger, brightWhite: text, brightYellow: gold,
    cursor: focus, cursorAccent: canvas, cyan: focus, foreground: text, green: success, red: danger,
    selectionBackground: color("--color-surface-raised", "#0d3566"), white: text, yellow: gold,
  };
}
