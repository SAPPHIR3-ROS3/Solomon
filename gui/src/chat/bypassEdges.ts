import type { Box, Point, Route } from "./parallelEdges";

const laneSpacing = 24;
const tolerance = 0.05;

type Bypass = { route: Route; first: number; last: number; left: number; right: number; y: number };

function findBypass(route: Route): Bypass | undefined {
  const { points } = route;
  for (let first = 1; first < points.length - 2; first++) {
    const start = points[first];
    const previous = points[first - 1];
    if (Math.abs(start.x - previous.x) > tolerance || start.y <= previous.y + tolerance) continue;
    let last = first;
    while (last + 1 < points.length && Math.abs(points[last + 1].y - start.y) < tolerance) last++;
    if (last === first || last === points.length - 1) continue;
    const end = points[last];
    const next = points[last + 1];
    if (Math.abs(end.x - next.x) > tolerance || next.y >= end.y - tolerance || Math.abs(end.x - start.x) < laneSpacing) continue;
    return { route, first, last, left: Math.min(start.x, end.x), right: Math.max(start.x, end.x), y: start.y };
  }
}

export function separateBypasses(svg: Document, routes: Route[], labels: Set<string | null>, nodes: Box[]) {
  const root = svg.documentElement;
  const box = root.getAttribute("viewBox")?.split(/[\s,]+/).map(Number);
  if (box?.length !== 4 || box.some(value => !Number.isFinite(value))) return;
  const bypasses = routes.flatMap(route => findBypass(route) ?? []).sort((a, b) => b.left - a.left);
  const placed: Bypass[] = [];
  let bottom = box[1] + box[3];
  for (const bypass of bypasses) {
    const { route, first, last } = bypass;
    if (labels.has(route.path.getAttribute("data-id")) || !route.path.classList.contains("edge-pattern-solid") ||
      route.path.classList.contains("edge-thickness-invisible") || route.path.getAttribute("data-look") === "handDrawn") {
      placed.push(bypass);
      continue;
    }
    const neighbors = placed.filter(other => bypass.left < other.right && bypass.right > other.left)
      .sort((a, b) => a.y - b.y);
    const y = neighbors.reduce((lane, other) =>
      Math.abs(lane - other.y) < laneSpacing ? other.y + laneSpacing : lane, bypass.y);
    const blocked = nodes.some(node => node.x < bypass.right && node.x + node.width > bypass.left &&
      node.y <= y && node.y + node.height >= bypass.y);
    if (y > bypass.y && !blocked) {
      const points = route.points.map((point, index) => index >= first && index <= last ? { x: point.x, y } : point);
      if (redraw(route, points, routes)) {
        bypass.y = y;
        bottom = Math.max(bottom, y + 8);
      }
    }
    placed.push(bypass);
  }
  if (bottom > box[1] + box[3]) {
    root.setAttribute("viewBox", [box[0], box[1], box[2], bottom - box[1]].join(" "));
  }
}

function redraw(route: Route, points: Point[], routes: Route[]) {
  const { path } = route;
  const coordinates = (path.getAttribute("d") ?? "").match(/-?(?:\d+\.?\d*|\.\d+)(?:e[-+]?\d+)?/gi)?.map(Number);
  if (!coordinates || coordinates.length < 4) return false;
  const drawn = [...points];
  drawn[0] = { x: coordinates[0], y: coordinates[1] };
  drawn[drawn.length - 1] = { x: coordinates.at(-2)!, y: coordinates.at(-1)! };
  const commands = [`M${drawn[0].x},${drawn[0].y}`];
  let length = 0;
  for (let index = 1; index < drawn.length; index++) {
    const start = drawn[index - 1];
    const end = drawn[index];
    length += Math.hypot(end.x - start.x, end.y - start.y);
    if (Math.abs(end.x - start.x) < tolerance) {
      const direction = Math.sign(end.y - start.y);
      const gaps = new Set<number>();
      for (const other of routes) {
        if (other === route) continue;
        for (let i = 1; i < other.points.length; i++) {
          const a = other.points[i - 1], b = other.points[i];
          if (Math.abs(a.y - b.y) < tolerance && Math.min(a.x, b.x) < start.x && start.x < Math.max(a.x, b.x) &&
            (a.y - start.y) * direction > 6 && (end.y - a.y) * direction > 6) gaps.add(a.y);
        }
      }
      let lastGap = start.y;
      for (const y of [...gaps].sort((a, b) => direction * (a - b))) {
        if ((y - lastGap) * direction < 12) continue;
        commands.push(`L${start.x},${y - 5 * direction}M${start.x},${y + 5 * direction}`);
        length -= 10;
        lastGap = y;
      }
    }
    commands.push(`L${end.x},${end.y}`);
  }
  path.setAttribute("d", commands.join(""));
  route.points = points;
  path.setAttribute("data-points", btoa(JSON.stringify(points)));
  const style = path.getAttribute("style");
  if (style) {
    path.setAttribute("style", style.replace(/stroke-dasharray:\s*0\s+([\d.]+)\s+[-\d.]+\s+([\d.]+)/,
      (_, before: string, after: string) => `stroke-dasharray: 0 ${before} ${Math.max(0, length - Number(before) - Number(after))} ${after}`));
  }
  return true;
}
