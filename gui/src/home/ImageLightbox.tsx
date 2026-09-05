import { useEffect, useRef, useState } from "react";
import type { ComposerImageAttachment } from "../chat/composerTypes";
import { canvasPoint, cropStrokes, drawStroke, moveStroke, scaleStroke, strokeContains, type Point, type Stroke, type StrokeKind } from "./imageEdit";

export type LightboxTool = "color" | "cursor" | "eraser" | "sketch" | "text";
export type SketchTool = "arrow" | "ellipse" | "freehand" | "pen" | "rectangle";
export type EraserTool = "normal" | "object";
export type SelectionTool = "crop" | "move" | "resize";

type Props = {
  color: string;
  eraserTool: EraserTool;
  image: ComposerImageAttachment;
  images: ComposerImageAttachment[];
  isColorPaletteOpen: boolean;
  onClose: () => void;
  onImagesChange?: (images: ComposerImageAttachment[]) => void;
  onRefresh: (image: ComposerImageAttachment) => void;
  onSelectIndex: (offset: number) => void;
  selectedIndex: number;
  selectionTool: SelectionTool;
  setColor: (color: string) => void;
  setEraserTool: (tool: EraserTool) => void;
  setIsColorPaletteOpen: (open: boolean | ((value: boolean) => boolean)) => void;
  setSelectionTool: (tool: SelectionTool) => void;
  setSketchTool: (tool: SketchTool) => void;
  setTextSize: (size: number | ((value: number) => number)) => void;
  setTool: (tool: LightboxTool) => void;
  sketchTool: SketchTool;
  textSize: number;
  tool: LightboxTool;
};

type History = { crop?: { left: number; top: number; width: number; height: number }; strokes: Stroke[] };

export function ImageLightbox(props: Props) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const imageRef = useRef<HTMLImageElement>(null);
  const [history, setHistory] = useState<History[]>([{ strokes: [] }]);
  const [historyIndex, setHistoryIndex] = useState(0);
  const [draft, setDraft] = useState<Stroke | null>(null);
  const [selectedStroke, setSelectedStroke] = useState<number | null>(null);
  const dragRef = useRef<{ kind: string; last: Point; origin: Point; start: History } | null>(null);
  const current = history[historyIndex];

  useEffect(() => {
    setHistory([{ strokes: [] }]);
    setHistoryIndex(0);
    setDraft(null);
    setSelectedStroke(null);
  }, [props.image.id, props.image.url]);

  useEffect(() => {
    const canvas = canvasRef.current;
    const image = imageRef.current;
    if (!canvas || !image || !image.naturalWidth) return;
    canvas.width = image.naturalWidth;
    canvas.height = image.naturalHeight;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    for (const stroke of current.strokes) drawStroke(ctx, stroke);
    if (draft) drawStroke(ctx, draft, true);
  }, [current, draft, props.image.url]);

  function pointFromEvent(event: React.PointerEvent<HTMLCanvasElement>): Point | null {
    const canvas = canvasRef.current;
    if (!canvas) return null;
    return canvasPoint(canvas.getBoundingClientRect(), event.clientX, event.clientY, canvas.width, canvas.height);
  }

  function commit(next: History) {
    setHistory((items) => [...items.slice(0, historyIndex + 1), next]);
    setHistoryIndex((index) => index + 1);
    setDraft(null);
  }

  function persist() {
    const canvas = canvasRef.current;
    const image = imageRef.current;
    if (!canvas || !image || !props.onImagesChange) return;
    const exportCanvas = document.createElement("canvas");
    const crop = current.crop;
    exportCanvas.width = crop?.width ?? image.naturalWidth;
    exportCanvas.height = crop?.height ?? image.naturalHeight;
    const ctx = exportCanvas.getContext("2d");
    if (!ctx) return;
    ctx.drawImage(image, crop?.left ?? 0, crop?.top ?? 0, exportCanvas.width, exportCanvas.height, 0, 0, exportCanvas.width, exportCanvas.height);
    ctx.drawImage(canvas, 0, 0);
    exportCanvas.toBlob((blob) => {
      if (!blob) return;
      const url = URL.createObjectURL(blob);
      props.onImagesChange?.(props.images.map((candidate) => candidate.id === props.image.id ? { ...candidate, blob, url } : candidate));
    }, "image/png");
  }

  function startStroke(kind: StrokeKind, point: Point) {
    setDraft({ color: props.color, kind, points: [point], size: kind === "text" ? props.textSize : kind === "pen" ? 3 : 8 });
  }

  function onPointerDown(event: React.PointerEvent<HTMLCanvasElement>) {
    event.preventDefault();
    event.stopPropagation();
    const point = pointFromEvent(event);
    if (!point) return;
    event.currentTarget.setPointerCapture(event.pointerId);
    if (props.tool === "sketch") {
      startStroke(props.sketchTool, point);
      return;
    }
    if (props.tool === "eraser") {
      if (props.eraserTool === "object") {
        const index = [...current.strokes].reverse().findIndex((stroke) => strokeContains(stroke, point));
        if (index >= 0) commit({ ...current, strokes: current.strokes.filter((_, item) => item !== current.strokes.length - 1 - index) });
        return;
      }
      startStroke("eraser", point);
      return;
    }
    if (props.tool === "text") {
      const text = window.prompt("Text", "");
      if (text) commit({ ...current, strokes: [...current.strokes, { color: props.color, kind: "text", points: [point], size: props.textSize, text }] });
      return;
    }
    if (props.tool === "cursor") {
      const index = [...current.strokes].reverse().findIndex((stroke) => strokeContains(stroke, point));
      const selected = index >= 0 ? current.strokes.length - 1 - index : null;
      setSelectedStroke(selected);
      dragRef.current = { kind: props.selectionTool, last: point, origin: point, start: current };
    }
  }

  function onPointerMove(event: React.PointerEvent<HTMLCanvasElement>) {
    const point = pointFromEvent(event);
    if (!point) return;
    if (draft) {
      setDraft({ ...draft, points: draft.kind === "freehand" || draft.kind === "eraser" || draft.kind === "pen" ? [...draft.points, point] : [draft.points[0], point] });
      return;
    }
    const drag = dragRef.current;
    if (!drag || selectedStroke === null) return;
    if (drag.kind === "move") {
      const dx = point.x - drag.last.x;
      const dy = point.y - drag.last.y;
      setHistory((items) => items.map((item, index) => index === historyIndex ? { ...item, strokes: item.strokes.map((stroke, strokeIndex) => strokeIndex === selectedStroke ? moveStroke(stroke, dx, dy) : stroke) } : item));
      drag.last = point;
    }
    if (drag.kind === "resize") {
      const origin = drag.start.strokes[selectedStroke].points[0];
      const startDist = Math.hypot(drag.origin.x - origin.x, drag.origin.y - origin.y) || 1;
      const nextDist = Math.hypot(point.x - origin.x, point.y - origin.y);
      setHistory((items) => items.map((item, index) => index === historyIndex ? { ...item, strokes: item.strokes.map((stroke, strokeIndex) => strokeIndex === selectedStroke ? scaleStroke(drag.start.strokes[selectedStroke], origin, nextDist / startDist) : stroke) } : item));
    }
    if (drag.kind === "crop") setDraft({ color: "#ffc704", kind: "rectangle", points: [drag.origin, point], size: 2 });
  }

  function onPointerUp() {
    if (draft && (draft.kind === "arrow" || draft.kind === "ellipse" || draft.kind === "eraser" || draft.kind === "freehand" || draft.kind === "pen" || draft.kind === "rectangle")) {
      if (props.tool === "cursor" && props.selectionTool === "crop" && draft.points.length > 1) {
        const cropped = cropStrokes(current.strokes, draft.points[0], draft.points[1]);
        commit({ crop: cropped.box, strokes: cropped.strokes });
      } else {
        commit({ ...current, strokes: [...current.strokes, draft] });
      }
    }
    dragRef.current = null;
  }

  function undo() { if (historyIndex > 0) setHistoryIndex(historyIndex - 1); }
  function redo() { if (historyIndex < history.length - 1) setHistoryIndex(historyIndex + 1); }
  function reset() { setHistory([{ strokes: [] }]); setHistoryIndex(0); setDraft(null); }

  return (
    <div aria-label={"Image preview: " + props.image.name} aria-modal="true" className={"composer-image-lightbox" + (props.isColorPaletteOpen ? " is-color-open" : "")} onClick={props.onClose} onMouseDown={(event) => event.stopPropagation()} role="dialog">
      <div aria-label="Image editing history" className="composer-image-lightbox-actions" onClick={(event) => event.stopPropagation()}>
        <button aria-label="Undo" className="composer-image-lightbox-action" disabled={historyIndex === 0} onClick={undo} title="Undo" type="button"><HistoryIcon action="undo" /></button>
        <button aria-label="Redo" className="composer-image-lightbox-action" disabled={historyIndex === history.length - 1} onClick={redo} title="Redo" type="button"><HistoryIcon action="redo" /></button>
        <button aria-label="Reset" className="composer-image-lightbox-action" disabled={historyIndex === 0 && current.strokes.length === 0} onClick={reset} title="Reset" type="button"><HistoryIcon action="reset" /></button>
        <button aria-label="Apply edits" className="composer-image-lightbox-action" onClick={(event) => { event.stopPropagation(); persist(); }} title="Apply" type="button"><HistoryIcon action="reset" /></button>
      </div>
      {props.images.length > 1 ? <button aria-label="Previous image" className="composer-image-lightbox-nav is-previous" onClick={(event) => { event.stopPropagation(); props.onSelectIndex(-1); }} type="button"><ArrowIcon direction="left" /></button> : null}
      <div className="composer-image-lightbox-stage" onClick={(event) => event.stopPropagation()}>
        <img alt={props.image.name} className="composer-image-lightbox-image" onError={() => props.onRefresh(props.image)} onLoad={() => { const canvas = canvasRef.current; const image = imageRef.current; if (canvas && image) { canvas.width = image.naturalWidth; canvas.height = image.naturalHeight; } }} ref={imageRef} src={props.image.url} />
        <canvas className="composer-image-lightbox-canvas" onPointerDown={onPointerDown} onPointerMove={onPointerMove} onPointerUp={onPointerUp} ref={canvasRef} />
      </div>
      {props.images.length > 1 ? <button aria-label="Next image" className="composer-image-lightbox-nav is-next" onClick={(event) => { event.stopPropagation(); props.onSelectIndex(1); }} type="button"><ArrowIcon direction="right" /></button> : null}
      <div aria-label="Image editing tools" className="composer-image-lightbox-tools" onClick={(event) => event.stopPropagation()}>
        {props.images.length > 1 ? <div aria-live="polite" className="composer-image-lightbox-count">{props.selectedIndex + 1} / {props.images.length}</div> : null}
        <div className="composer-image-lightbox-tool-row">
          <button aria-expanded={props.isColorPaletteOpen} aria-label="Color" aria-pressed={props.tool === "color"} className={"composer-image-lightbox-tool" + (props.tool === "color" ? " is-selected" : "")} onClick={() => { props.setTool("color"); props.setIsColorPaletteOpen((open) => !open); }} style={{ color: props.color }} title="Color" type="button"><ToolIcon tool="color" /></button>
          <button aria-label="Sketch" aria-pressed={props.tool === "sketch"} className={"composer-image-lightbox-tool" + (props.tool === "sketch" ? " is-selected" : "")} onClick={() => { props.setTool("sketch"); props.setIsColorPaletteOpen(false); }} title="Sketch" type="button"><ToolIcon tool="sketch" /></button>
          <button aria-label="Eraser" aria-pressed={props.tool === "eraser"} className={"composer-image-lightbox-tool" + (props.tool === "eraser" ? " is-selected" : "")} onClick={() => { props.setTool("eraser"); props.setIsColorPaletteOpen(false); }} title="Eraser" type="button"><ToolIcon tool="eraser" /></button>
          <button aria-label="Text" aria-pressed={props.tool === "text"} className={"composer-image-lightbox-tool" + (props.tool === "text" ? " is-selected" : "")} onClick={() => { props.setTool("text"); props.setIsColorPaletteOpen(false); }} title="Text" type="button"><ToolIcon tool="text" /></button>
          <button aria-label="Select" aria-pressed={props.tool === "cursor"} className={"composer-image-lightbox-tool" + (props.tool === "cursor" ? " is-selected" : "")} onClick={() => { props.setTool("cursor"); props.setIsColorPaletteOpen(false); }} title="Select" type="button"><ToolIcon tool="cursor" /></button>
        </div>
      </div>
    </div>
  );
}

function ArrowIcon({ direction }: { direction: "left" | "right" }) {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><path d={direction === "left" ? "m14.5 5-7 7 7 7" : "m9.5 5 7 7-7 7"} /></svg>;
}

function HistoryIcon({ action }: { action: "redo" | "reset" | "undo" }) {
  const path = action === "undo" ? "M9 7H4m0 0 3-3M4 7c4.8-4.2 12-.8 12 4.5 0 2.2-1.8 4-4 4H9" : action === "redo" ? "M15 7h5m0 0-3-3m3 3c-4.8-4.2-12-.8-12 4.5 0 2.2 1.8 4 4 4h3" : "M19 8a7 7 0 1 0 1 4m-1-8v4h-4";
  return <svg aria-hidden="true" viewBox="0 0 24 24"><path d={path} /></svg>;
}

function ToolIcon({ tool }: { tool: "color" | "cursor" | "eraser" | "sketch" | "text" }) {
  if (tool === "color") return <svg aria-hidden="true" viewBox="0 0 24 24"><circle cx="12" cy="12" fill="currentColor" r="7" /></svg>;
  if (tool === "text") return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M5 5h14M12 5v14" /></svg>;
  if (tool === "eraser") return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="m5 15 8-8a2.8 2.8 0 0 1 4 0l1 1a2.8 2.8 0 0 1 0 4l-3 3H9m-4 0 4 4h6" /></svg>;
  if (tool === "cursor") return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M6 3 18 13l-6 .7 3.4 6.2-2.5 1.4L9.5 15 6 19z" /></svg>;
  return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="m5 19 1.4-4.6L16.7 4.1a1.8 1.8 0 0 1 2.5 0l.7.7a1.8 1.8 0 0 1 0 2.5L9.6 17.6 5 19Z" /></svg>;
}
