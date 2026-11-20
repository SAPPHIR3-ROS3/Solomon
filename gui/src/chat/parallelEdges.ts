import { separateBypasses } from "./bypassEdges";

export type Point = { x: number; y: number };
type Axis = keyof Point;
export type Box = { x: number; y: number; width: number; height: number };
export type Route = { path: SVGPathElement; points: Point[] };

const epsilon = 0.05;
const laneGap = 16;

function nodeBoxes(svg: Document) {
  return [...svg.querySelectorAll("g.node[transform]")].flatMap(node => {
    const position = /^translate\((-?[\d.]+)[, ]+(-?[\d.]+)\)$/.exec(node.getAttribute("transform") ?? "");
    const shape = node.querySelector(":scope > rect.label-container");
    if (!position || !shape) return [];
    return [{
      x: Number(position[1]) + Number(shape.getAttribute("x")),
      y: Number(position[2]) + Number(shape.getAttribute("y")),
      width: Number(shape.getAttribute("width")),
      height: Number(shape.getAttribute("height")),
    }];
  });
}

function size(box: Box, axis: Axis) {
  return axis === "x" ? box.width : box.height;
}

function attachment(boxes: Box[], point: Point, axis: Axis, side: number) {
  const cross = axis === "x" ? "y" : "x";
  return boxes.find(box => Math.abs(point[axis] - box[axis] - (side > 0 ? size(box, axis) : 0)) < epsilon &&
    point[cross] >= box[cross] && point[cross] <= box[cross] + size(box, cross));
}

function readRoute(path: SVGPathElement): Route | undefined {
  try {
    const points = JSON.parse(atob(path.getAttribute("data-points") ?? "")) as Point[];
    if (Array.isArray(points) && points.length >= 2 && points.every(point => Number.isFinite(point?.x) && Number.isFinite(point?.y))) {
      return { path, points };
    }
  } catch {
    return undefined;
  }
}

export function separateParallelEdges(svg: Document) {
  const boxes = nodeBoxes(svg);
  if (boxes.length < 2) return;
  const routes = [...svg.querySelectorAll<SVGPathElement>(".edgePaths path[data-points]")].flatMap(path => readRoute(path) ?? []);
  const labels = new Set([...svg.querySelectorAll(".edgeLabel [data-id]")]
    .filter(label => label.textContent?.trim()).map(label => label.getAttribute("data-id")));
  separateBypasses(svg, routes, labels, boxes);

  for (const axis of ["x", "y"] as const) {
    const cross = axis === "x" ? "y" : "x";
    const groups = new Map<string, { source: Box; target: Box; routes: Route[] }>();
    for (const route of routes) {
      const { path, points } = route;
      const start = points[0], next = points[1], end = points.at(-1)!, previous = points.at(-2)!;
      const direction = Math.sign(end[axis] - start[axis]);
      if (!direction || Math.abs(start[cross] - next[cross]) >= epsilon || Math.abs(end[cross] - previous[cross]) >= epsilon) continue;
      const source = attachment(boxes, start, axis, direction);
      const target = attachment(boxes, end, axis, -direction);
      if (!source || !target || source === target) continue;
      const pair = [boxes.indexOf(source), boxes.indexOf(target)].sort((a, b) => a - b).join(",");
      const group = groups.get(pair) ?? { source, target, routes: [] };
      group.routes.push(route);
      groups.set(pair, group);
    }
    for (const { source, target, routes: parallel } of groups.values()) {
      if (parallel.length < 2 || parallel.some(({ path }) => labels.has(path.getAttribute("data-id")) ||
        !path.classList.contains("edge-pattern-solid") || path.classList.contains("edge-thickness-invisible") ||
        path.getAttribute("data-look") === "handDrawn")) continue;

      const low = Math.max(source[cross], target[cross]) + 5;
      const high = Math.min(source[cross] + size(source, cross), target[cross] + size(target, cross)) - 5;
      const first = parallel[0].points;
      const left = Math.min(first[0][axis], first.at(-1)![axis]);
      const right = Math.max(first[0][axis], first.at(-1)![axis]);
      if (high - low < laneGap || right - left < laneGap) continue;

      const siblings = new Set(parallel.map(route => route.path));
      const blocked = (limit: number) =>
        boxes.some(box => box[axis] < right - epsilon && box[axis] + size(box, axis) > left + epsilon &&
          box[cross] < limit && box[cross] + size(box, cross) > low) ||
        routes.some(({ path, points }) => !siblings.has(path) && points.slice(1).some((point, index) => {
          const previous = points[index];
          return Math.min(point[axis], previous[axis]) < right - epsilon && Math.max(point[axis], previous[axis]) > left + epsilon &&
            Math.min(point[cross], previous[cross]) <= limit && Math.max(point[cross], previous[cross]) >= low;
        }));
      if (blocked(high)) continue;

      const middle = (low + high) / 2;
      const gap = Math.min(laneGap, (high - low) / (parallel.length - 1));
      const shoulder = blocked(high + 8) ? high : high + 8;
      parallel.sort((a, b) => a.points[0][cross] + a.points.at(-1)![cross] - b.points[0][cross] - b.points.at(-1)![cross]);
      parallel.forEach((route, index) => {
        const lane = middle + (index - (parallel.length - 1) / 2) * gap;
        const step = parallel.length === 2 && index === 1 && shoulder - lane >= 6 ? shoulder : undefined;
        moveRoute(route, axis, lane, step);
      });
    }
  }
}

function moveRoute(route: Route, axis: Axis, lane: number, step?: number) {
  const { path, points } = route;
  const cross = axis === "x" ? "y" : "x";
  const coordinates = (path.getAttribute("d") ?? "").match(/-?(?:\d+\.?\d*|\.\d+)(?:e[-+]?\d+)?/gi)?.map(Number);
  if (!coordinates || coordinates.length < 4) return;
  const component = axis === "x" ? 0 : 1;
  const start = { ...points[0], [cross]: lane };
  const end = { ...points.at(-1)!, [cross]: lane };
  const drawnStart = { ...start, [axis]: coordinates[component] };
  const drawnEnd = { ...end, [axis]: coordinates[coordinates.length - 2 + component] };
  const span = drawnEnd[axis] - drawnStart[axis];
  const bends: Point[] = [];
  if (step !== undefined && Math.abs(span) >= 22) {
    const near = drawnStart[axis] + span * 0.2;
    const far = drawnEnd[axis] - span * 0.2;
    bends.push({ ...start, [axis]: near }, { ...start, [axis]: near, [cross]: step },
      { ...end, [axis]: far, [cross]: step }, { ...end, [axis]: far });
  }
  const drawn = [drawnStart, ...bends, drawnEnd];
  path.setAttribute("d", drawn.map(({ x, y }, index) => `${index ? "L" : "M"}${x},${y}`).join(""));
  route.points = [start, ...bends, end];
  path.setAttribute("data-points", btoa(JSON.stringify(route.points)));
  const style = path.getAttribute("style");
  if (style) {
    const length = Math.abs(span) + (bends.length && step !== undefined ? 2 * Math.abs(step - lane) : 0);
    path.setAttribute("style", style.replace(/stroke-dasharray:\s*0\s+([\d.]+)\s+[-\d.]+\s+([\d.]+)/,
      (_, before: string, after: string) => `stroke-dasharray: 0 ${before} ${Math.max(0, length - Number(before) - Number(after))} ${after}`));
  }
}
