import type { IMarker, Terminal } from "@xterm/xterm";

export function installCommandSelection(term: Terminal, host: HTMLElement) {
  const blocks: { start: IMarker; end?: IMarker }[] = [];
  let prompt: IMarker | undefined;
  let integrated = false;
  let pressed: { x: number; y: number } | undefined;
  const mark = () => term.registerMarker(0);
  const finish = () => {
    const block = blocks.at(-1);
    if (block && !block.end) block.end = mark();
    prompt?.dispose();
    prompt = mark();
  };
  const handlers = [133, 633].map((code) => term.parser.registerOscHandler(code, (data) => {
    const event = data.split(";")[0];
    if (event === "A") { integrated = true; finish(); }
    if (event === "C") begin(true);
    return false;
  }));
  const begin = (execution = false) => {
    if (integrated && !execution && blocks.at(-1) && !blocks.at(-1)?.end) return;
    if (term.buffer.active !== term.buffer.normal) return;
    const last = blocks.at(-1);
    if (last && !last.end) return;
    const start = prompt ?? mark();
    prompt = undefined;
    if (start) blocks.push({ start });
    for (let index = blocks.length - 1; index >= 0; index--) {
      if (blocks[index].start.isDisposed) blocks.splice(index, 1);
    }
  };
  const down = (event: MouseEvent) => {
    pressed = event.button === 0 ? { x: event.clientX, y: event.clientY } : undefined;
  };
  const up = (event: MouseEvent) => {
    const start = pressed;
    pressed = undefined;
    if (!start || event.button !== 0 || event.detail !== 1 || event.shiftKey || event.ctrlKey || event.metaKey || event.altKey) return;
    if (Math.hypot(event.clientX - start.x, event.clientY - start.y) > 4) return;
    if (term.buffer.active !== term.buffer.normal) return;
    const screen = host.querySelector(".xterm-screen");
    if (!screen || !(event.target instanceof Node) || !screen.contains(event.target)) return;
    const rect = screen.getBoundingClientRect();
    if (event.clientY < rect.top || event.clientY >= rect.bottom) return;
    const row = term.buffer.active.viewportY + Math.floor((event.clientY - rect.top) * term.rows / rect.height);
    const block = blocks.find((candidate) => !candidate.start.isDisposed && (!candidate.end || !candidate.end.isDisposed) && row >= candidate.start.line && row < (candidate.end?.line ?? term.buffer.active.baseY + term.buffer.active.cursorY));
    if (block) term.selectLines(block.start.line, (block.end?.line ?? term.buffer.active.baseY + term.buffer.active.cursorY) - 1);
  };
  host.addEventListener("mousedown", down);
  host.addEventListener("mouseup", up);
  return {
    begin,
    finish,
    dispose() {
      host.removeEventListener("mousedown", down);
      host.removeEventListener("mouseup", up);
      handlers.forEach((handler) => handler.dispose());
      blocks.forEach((block) => { block.start.dispose(); block.end?.dispose(); });
      prompt?.dispose();
    },
  };
}
