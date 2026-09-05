export type Point = { x: number; y: number };
export type StrokeKind = "arrow" | "ellipse" | "eraser" | "freehand" | "pen" | "rectangle" | "text";
export type Stroke = { color: string; kind: StrokeKind; points: Point[]; size: number; text?: string };

export function canvasPoint(rect: { left: number; top: number; width: number; height: number }, clientX: number, clientY: number, width: number, height: number): Point {
  const x = ((clientX - rect.left) / Math.max(rect.width, 1)) * width;
  const y = ((clientY - rect.top) / Math.max(rect.height, 1)) * height;
  return { x: clamp(x, 0, width), y: clamp(y, 0, height) };
}

export function strokeBounds(stroke: Stroke) {
  const xs = stroke.points.map((point) => point.x);
  const ys = stroke.points.map((point) => point.y);
  const pad = stroke.size + (stroke.kind === "text" ? stroke.size * 4 : 0);
  return {
    left: Math.min(...xs) - pad,
    top: Math.min(...ys) - pad,
    right: Math.max(...xs) + pad + (stroke.text ? stroke.text.length * stroke.size * 0.6 : 0),
    bottom: Math.max(...ys) + pad,
  };
}

export function strokeContains(stroke: Stroke, point: Point) {
  const bounds = strokeBounds(stroke);
  return point.x >= bounds.left && point.x <= bounds.right && point.y >= bounds.top && point.y <= bounds.bottom;
}

export function moveStroke(stroke: Stroke, dx: number, dy: number): Stroke {
  return { ...stroke, points: stroke.points.map((point) => ({ x: point.x + dx, y: point.y + dy })) };
}

export function scaleStroke(stroke: Stroke, origin: Point, factor: number): Stroke {
  const next = Math.max(0.2, factor);
  return {
    ...stroke,
    size: Math.max(4, stroke.size * next),
    points: stroke.points.map((point) => ({
      x: origin.x + (point.x - origin.x) * next,
      y: origin.y + (point.y - origin.y) * next,
    })),
  };
}

export function cropStrokes(strokes: Stroke[], start: Point, end: Point) {
  const left = Math.min(start.x, end.x);
  const top = Math.min(start.y, end.y);
  const right = Math.max(start.x, end.x);
  const bottom = Math.max(start.y, end.y);
  return {
    box: { left, top, width: Math.max(1, right - left), height: Math.max(1, bottom - top) },
    strokes: strokes.map((stroke) => moveStroke(stroke, -left, -top)),
  };
}

export function drawStroke(ctx: CanvasRenderingContext2D, stroke: Stroke, preview = false) {
  if (!stroke.points.length) return;
  ctx.save();
  ctx.lineCap = "round";
  ctx.lineJoin = "round";
  ctx.strokeStyle = stroke.color;
  ctx.fillStyle = stroke.color;
  ctx.lineWidth = stroke.size;
  ctx.globalCompositeOperation = stroke.kind === "eraser" ? "destination-out" : "source-over";
  const first = stroke.points[0];
  const last = stroke.points[stroke.points.length - 1];
  if (stroke.kind === "text" && stroke.text) {
    ctx.font = `600 ${stroke.size}px Geist, ui-sans-serif, sans-serif`;
    ctx.fillText(stroke.text, first.x, first.y);
    ctx.restore();
    return;
  }
  ctx.beginPath();
  if (stroke.kind === "ellipse") {
    ctx.ellipse((first.x + last.x) / 2, (first.y + last.y) / 2, Math.abs(last.x - first.x) / 2, Math.abs(last.y - first.y) / 2, 0, 0, Math.PI * 2);
    ctx.stroke();
  } else if (stroke.kind === "rectangle") {
    ctx.strokeRect(first.x, first.y, last.x - first.x, last.y - first.y);
  } else if (stroke.kind === "arrow") {
    ctx.moveTo(first.x, first.y);
    ctx.lineTo(last.x, last.y);
    ctx.stroke();
    const angle = Math.atan2(last.y - first.y, last.x - first.x);
    ctx.beginPath();
    ctx.moveTo(last.x, last.y);
    ctx.lineTo(last.x - 16 * Math.cos(angle - 0.4), last.y - 16 * Math.sin(angle - 0.4));
    ctx.lineTo(last.x - 16 * Math.cos(angle + 0.4), last.y - 16 * Math.sin(angle + 0.4));
    ctx.closePath();
    ctx.fill();
  } else {
    ctx.moveTo(first.x, first.y);
    for (const point of stroke.points.slice(1)) ctx.lineTo(point.x, point.y);
    if (preview && stroke.kind === "pen" && stroke.points.length === 1) ctx.lineTo(last.x, last.y);
    ctx.stroke();
  }
  ctx.restore();
}

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value));
}
