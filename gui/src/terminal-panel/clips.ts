export type TerminalClip = { end: number; start: number; tag: string; text: string };

const EVENT = "solomon-terminal-clip";

export function terminalClipTag(start: number, end: number) {
  return "[terminal-L" + String(start) + "-L" + String(end) + "]";
}

export function addTerminalClip(start: number, end: number, text: string) {
  const clip: TerminalClip = { end, start, tag: terminalClipTag(start, end), text };
  window.dispatchEvent(new CustomEvent(EVENT, { detail: clip }));
}

export function onTerminalClip(handler: (clip: TerminalClip) => void) {
  const listener = (event: Event) => {
    const detail = (event as CustomEvent<TerminalClip>).detail;
    if (detail?.tag && detail.text) handler(detail);
  };
  window.addEventListener(EVENT, listener);
  return () => window.removeEventListener(EVENT, listener);
}
