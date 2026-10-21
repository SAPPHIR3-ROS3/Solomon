import { useEffect, useRef, useState, type ReactNode } from "react";
import type { ComposerImageAttachment } from "../chat/composerTypes";
import { canvasPoint, cropStrokes, drawStroke, moveStroke, scaleStroke, strokeContains, type Point, type Stroke, type StrokeKind, type ImageEditHistory } from "./imageEdit";

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
  children: ReactNode;
  sketchTool: SketchTool;
  textSize: number;
  tool: LightboxTool;
};

type History = ImageEditHistory;

export function ImageLightbox(props: Props) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const imageRef = useRef<HTMLImageElement>(null);
  const [history, setHistory] = useState<History[]>(props.image.editState?.history ?? [{ strokes: [] }]);
  const [historyIndex, setHistoryIndex] = useState(props.image.editState?.index ?? 0);
  const [draft, setDraft] = useState<Stroke | null>(null);
  const [imageLoaded, setImageLoaded] = useState(0);
  const [selectedStroke, setSelectedStroke] = useState<number | null>(null);
  const dragRef = useRef<{ kind: string; last: Point; origin: Point; start: History } | null>(null);
  const current = history[historyIndex];

  const [source] = useState(() => props.image.editState?.source ?? props.image.blob);
  const [sourceURL, setSourceURL] = useState(props.image.url);
  const latestProps = useRef(props);
  latestProps.current = props;
  const savedState = useRef(current);
  const saveSequence = useRef(0);

  useEffect(() => {
    if (!source) return;
    const url = URL.createObjectURL(source);
    setSourceURL(url);
    return () => URL.revokeObjectURL(url);
  }, [source]);

  useEffect(() => {
    const canvas = canvasRef.current;
    const image = imageRef.current;
    if (!canvas || !image || !image.naturalWidth) return;
    canvas.width = current.crop?.width ?? image.naturalWidth;
    canvas.height = current.crop?.height ?? image.naturalHeight;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    ctx.drawImage(image, current.crop?.left ?? 0, current.crop?.top ?? 0, canvas.width, canvas.height, 0, 0, canvas.width, canvas.height);
    const overlay = document.createElement("canvas");
    overlay.width = canvas.width;
    overlay.height = canvas.height;
    const overlayContext = overlay.getContext("2d");
    if (!overlayContext) return;
    for (const stroke of current.strokes) drawStroke(overlayContext, stroke);
    if (draft) drawStroke(overlayContext, draft, true);
    ctx.drawImage(overlay, 0, 0);
  }, [current, draft, imageLoaded, sourceURL]);

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

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas || !imageLoaded || draft || dragRef.current || savedState.current === current) return;
    savedState.current = current;
    const sequence = ++saveSequence.current;
    // Snapshot the finished edit; later pointer previews must never enter the saved image.
    const snapshot = document.createElement("canvas");
    snapshot.width = canvas.width;
    snapshot.height = canvas.height;
    snapshot.getContext("2d")?.drawImage(canvas, 0, 0);
    snapshot.toBlob((blob) => {
      if (!blob || sequence !== saveSequence.current) return;
      const latest = latestProps.current;
      if (!latest.onImagesChange || !latest.images.some((image) => image.id === props.image.id)) return;
      const url = URL.createObjectURL(blob);
      latest.onImagesChange(latest.images.map((image) => image.id === props.image.id ? {
        ...image, blob, url,
        editState: source ? { source, history, index: historyIndex } : undefined,
      } : image));
    }, "image/png");
  }, [current, draft, history, historyIndex, imageLoaded, source, props.image.id]);

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
    if (draft && !dragRef.current) {
      setDraft({ ...draft, points: draft.kind === "freehand" || draft.kind === "eraser" || draft.kind === "pen" ? [...draft.points, point] : [draft.points[0], point] });
      return;
    }
    const drag = dragRef.current;
    if (!drag) return;
    if (drag.kind === "crop") {
      setDraft({ color: "#ffc704", kind: "rectangle", points: [drag.origin, point], size: 2 });
      return;
    }
    if (selectedStroke === null) return;
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

  }

  function onPointerUp() {
    if (draft && (draft.kind === "arrow" || draft.kind === "ellipse" || draft.kind === "eraser" || draft.kind === "freehand" || draft.kind === "pen" || draft.kind === "rectangle")) {
      if (props.tool === "cursor" && props.selectionTool === "crop" && draft.points.length > 1) {
        const cropped = cropStrokes(current.strokes, draft.points[0], draft.points[1]);
        if (cropped.box.width > 1 && cropped.box.height > 1) commit({
          crop: { ...cropped.box, left: (current.crop?.left ?? 0) + cropped.box.left, top: (current.crop?.top ?? 0) + cropped.box.top },
          strokes: cropped.strokes,
        });
        else setDraft(null);
      } else {
        commit({ ...current, strokes: [...current.strokes, draft] });
      }
    }
    const drag = dragRef.current;
    if (drag && drag.kind !== "crop" && current !== drag.start) {
      setHistory((items) => [...items.slice(0, historyIndex), drag.start, current]);
      setHistoryIndex(historyIndex + 1);
    }
    dragRef.current = null;
  }

  function undo() { if (historyIndex > 0) setHistoryIndex(historyIndex - 1); }
  function redo() { if (historyIndex < history.length - 1) setHistoryIndex(historyIndex + 1); }
  function reset() { commit({ strokes: [] }); setSelectedStroke(null); }

  return (
    <div aria-label={"Image preview: " + props.image.name} aria-modal="true" className={"composer-image-lightbox" + (props.isColorPaletteOpen ? " is-color-open" : "")} onClick={props.onClose} onMouseDown={(event) => event.stopPropagation()} role="dialog">
      <div aria-label="Image editing history" className="composer-image-lightbox-actions" onClick={(event) => event.stopPropagation()}>
        <button aria-label="Undo" className="composer-image-lightbox-action" disabled={historyIndex === 0} onClick={undo} title="Undo" type="button"><HistoryIcon action="undo" /></button>
        <button aria-label="Redo" className="composer-image-lightbox-action" disabled={historyIndex === history.length - 1} onClick={redo} title="Redo" type="button"><HistoryIcon action="redo" /></button>
        <button aria-label="Reset" className="composer-image-lightbox-action" disabled={historyIndex === 0 && current.strokes.length === 0} onClick={reset} title="Reset" type="button"><HistoryIcon action="reset" /></button>
      </div>
      {props.images.length > 1 ? <button aria-label="Previous image" className="composer-image-lightbox-nav is-previous" onClick={(event) => { event.stopPropagation(); props.onSelectIndex(-1); }} type="button"><ArrowIcon direction="left" /></button> : null}
      <div className="composer-image-lightbox-stage composer-image-editor-stage" onClick={(event) => event.stopPropagation()}>
        <img alt={props.image.name} className="composer-image-lightbox-image" onError={() => props.onRefresh(props.image)} onLoad={() => setImageLoaded((value) => value + 1)} ref={imageRef} src={sourceURL} />
        <canvas aria-label="Image editing canvas" className="composer-image-lightbox-canvas" onPointerDown={onPointerDown} onPointerMove={onPointerMove} onPointerUp={onPointerUp} onPointerCancel={() => { setDraft(null); const drag = dragRef.current; if (drag) setHistory((items) => items.map((item, index) => index === historyIndex ? drag.start : item)); dragRef.current = null; }} ref={canvasRef} />
      </div>
      {props.images.length > 1 ? <button aria-label="Next image" className="composer-image-lightbox-nav is-next" onClick={(event) => { event.stopPropagation(); props.onSelectIndex(1); }} type="button"><ArrowIcon direction="right" /></button> : null}
      {props.children}
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
