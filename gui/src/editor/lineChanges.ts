export type LineChange = { line: number; kind: "added" | "modified" | "deleted" };

function edits(before: string[], after: string[]): string[] {
  const n = before.length, m = after.length;
  if (!n) return after.map(() => "add");
  if (!m) return before.map(() => "remove");
  const frontier = new Map<number, number>([[1, 0]]);
  const trace: Map<number, number>[] = [];
  for (let depth = 0; depth <= Math.min(n + m, 1000); depth++) {
    trace.push(new Map(frontier));
    for (let diagonal = -depth; diagonal <= depth; diagonal += 2) {
      let x = diagonal === -depth || (diagonal !== depth && (frontier.get(diagonal - 1) ?? -1) < (frontier.get(diagonal + 1) ?? -1))
        ? frontier.get(diagonal + 1) ?? 0 : (frontier.get(diagonal - 1) ?? 0) + 1;
      let y = x - diagonal;
      while (x < n && y < m && before[x] === after[y]) { x++; y++; }
      frontier.set(diagonal, x);
      if (x < n || y < m) continue;
      const result: string[] = [];
      x = n; y = m;
      for (let step = depth; step >= 0; step--) {
        const previous = trace[step];
        const k = x - y;
        const previousK = k === -step || (k !== step && (previous.get(k - 1) ?? -1) < (previous.get(k + 1) ?? -1)) ? k + 1 : k - 1;
        const previousX = previous.get(previousK) ?? 0;
        const previousY = previousX - previousK;
        while (x > previousX && y > previousY) { result.push("equal"); x--; y--; }
        if (step > 0) result.push(x === previousX ? "add" : "remove");
        x = previousX; y = previousY;
      }
      return result.reverse();
    }
  }
  return [...before.map(() => "remove"), ...after.map(() => "add")];
}

export function changedLines(original: string, current: string): LineChange[] {
  if (original === current) return [];
  const lines = (text: string) => text === "" ? [] : text.replaceAll("\r\n", "\n").split("\n");
  const before = lines(original), after = lines(current);
  let prefix = 0, suffix = 0;
  while (prefix < before.length && prefix < after.length && before[prefix] === after[prefix]) prefix++;
  while (suffix < before.length - prefix && suffix < after.length - prefix && before[before.length - suffix - 1] === after[after.length - suffix - 1]) suffix++;
  const operations = edits(before.slice(prefix, before.length - suffix), after.slice(prefix, after.length - suffix));
  const changes: LineChange[] = [];
  let line = prefix + 1, added = 0, removed = 0;
  function flush() {
    for (let index = 0; index < added; index++) changes.push({ line: line + index, kind: removed ? "modified" : "added" });
    if (removed && !added) changes.push({ line: Math.max(1, Math.min(line, after.length)), kind: "deleted" });
    line += added; added = 0; removed = 0;
  }
  for (const operation of operations) {
    if (operation === "add") added++;
    else if (operation === "remove") removed++;
    else { flush(); line++; }
  }
  flush();
  return changes;
}
