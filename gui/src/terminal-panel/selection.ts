export function selectionLineRange(term: { buffer: { active: { baseY: number } }; getSelectionPosition?: () => { start: { x: number; y: number }; end: { x: number; y: number } } | undefined }) {
  const position = term.getSelectionPosition?.();
  if (!position) return null;
  const offset = term.buffer.active.baseY;
  let start = position.start.y + offset + 1;
  let end = position.end.y + offset + 1;
  if (position.end.x === 0 && end > start) end -= 1;
  if (end < start) { const swap = start; start = end; end = swap; }
  return { end, start };
}

export function selectedTerminalText(term: { getSelection: () => string }) {
  return term.getSelection().replaceAll("\r\n", "\n").replaceAll("\r", "\n");
}
