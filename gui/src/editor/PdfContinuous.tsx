import { memo, useCallback, useEffect, useLayoutEffect, useRef, useState, type RefObject } from "react";
import type { PDFDocumentProxy } from "pdfjs-dist";
import { PDF_TO_CSS, renderPdfSpread, spreadGeometry, type PdfFitMode } from "./pdfRendering";

type Size = { width: number; height: number };
type Navigation = { page: number; revision: number };
type Report = { loading: boolean; zoom?: number; error?: string };

const ContinuousSpread = memo(function ContinuousSpread({ pdf, index, columns, active, natural, size, zoom, fitMode, priority, report }: {
  pdf: PDFDocumentProxy; index: number; columns: number; active: boolean; natural: Size; size: Size;
  zoom: number; fitMode: PdfFitMode; priority: number; report: (index: number, result: Report) => void;
}) {
  const surface = useRef<HTMLDivElement>(null);
  const committed = useRef<{ pdf: PDFDocumentProxy; zoom: number; dpr: number } | null>(null);
  const [knownSize, setKnownSize] = useState<{ widths: number[]; heights: number[] } | null>(null);
  const [error, setError] = useState("");
  const count = Math.min(columns, pdf.numPages - index * columns);
  const dimensions = spreadGeometry(knownSize?.widths ?? Array(count).fill(natural.width), knownSize?.heights ?? Array(count).fill(natural.height), size, zoom, fitMode);

  useEffect(() => {
    if (!active || !surface.current) return;
    setError("");
    const previous = committed.current;
    const dpr = window.devicePixelRatio || 1;
    if (surface.current.querySelector("canvas") && previous?.pdf === pdf
      && previous.dpr === dpr && Math.abs(previous.zoom - dimensions.zoom) < 1e-8) {
      report(index, { loading: false, zoom: previous.zoom });
      return;
    }
    let cancelled = false;
    report(index, { loading: true });
    const numbers = Array.from({ length: count }, (_, offset) => index * columns + offset + 1);
    const render = renderPdfSpread(pdf, numbers, surface.current.ownerDocument, size, zoom, fitMode, 4000000, priority);
    void render.promise.then(result => {
      if (cancelled || !result || !surface.current) return;
      surface.current.replaceChildren(result.element);
      committed.current = { pdf, zoom: result.zoom, dpr };
      surface.current.dataset.zoom = String(result.zoom);
      surface.current.dataset.ready = "true";
      setKnownSize({ widths: result.naturalWidths, heights: result.naturalHeights });
      report(index, { loading: false, zoom: result.zoom });
    }).catch(() => {
      if (!cancelled) {
        setError("Unable to render this PDF page.");
        report(index, { loading: false, error: "Unable to render this PDF page." });
      }
    });
    return () => { cancelled = true; render.cancel(); };
  }, [pdf, index, count, columns, active, size, zoom, fitMode, priority, report]);

  // Offscreen bitmaps are unmounted; only lightweight page-sized slots remain.
  return <div className="editor-pdf-slot" data-spread={index} style={{ width: dimensions.width, height: dimensions.height }}>
    {active ? <div ref={surface} className="editor-pdf-slot-surface" data-ready="false" /> : null}
    {error && active ? <p role="alert">{error}</p> : null}
  </div>;
});

export function PdfContinuous({ pdf, root, columns, direction, size, zoom, fitMode, page, navigation, onPage, onZoom, onLoading, onError }: {
  pdf: PDFDocumentProxy; root: RefObject<HTMLDivElement | null>; columns: number; direction: "vertical" | "horizontal";
  size: Size; zoom: number; fitMode: PdfFitMode; page: number; navigation: Navigation;
  onPage: (page: number) => void; onZoom: (zoom: number) => void; onLoading: (loading: boolean) => void; onError: (error: string) => void;
}) {
  const flow = useRef<HTMLDivElement>(null);
  const [natural, setNatural] = useState<Size | null>(null);
  const [active, setActive] = useState(() => new Set([Math.floor((page - 1) / columns)]));
  const visible = useRef(new Set<number>());
  const current = useRef(Math.floor((page - 1) / columns));
  current.current = Math.floor((page - 1) / columns);
  const count = Math.ceil(pdf.numPages / columns);

  useEffect(() => {
    let cancelled = false;
    void pdf.getPage(1).then(first => {
      if (cancelled) return;
      const viewport = first.getViewport({ scale: PDF_TO_CSS });
      setNatural({ width: viewport.width, height: viewport.height });
    }).catch(() => { if (!cancelled) onError("Unable to read PDF pages."); });
    return () => { cancelled = true; };
  }, [pdf, onError]);

  const report = useCallback((index: number, result: Report) => {
    if (index !== current.current) return;
    onLoading(result.loading);
    onError(result.error ?? "");
    if (result.zoom !== undefined) onZoom(result.zoom);
  }, [onLoading, onError, onZoom]);

  useEffect(() => {
    const host = root.current;
    const content = flow.current;
    if (!host || !content || !natural) return;
    let frame = 0;
    const updateCurrent = () => {
      frame = 0;
      const bounds = host.getBoundingClientRect();
      let best = -1, bestArea = 0;
      for (const index of visible.current) {
        const node = content.children[index] as HTMLElement | undefined;
        if (!node) continue;
        const rect = node.getBoundingClientRect();
        const area = Math.max(0, Math.min(rect.right, bounds.right) - Math.max(rect.left, bounds.left))
          * Math.max(0, Math.min(rect.bottom, bounds.bottom) - Math.max(rect.top, bounds.top));
        if (area > bestArea) { best = index; bestArea = area; }
      }
      const last = content.lastElementChild?.getBoundingClientRect();
      // Measure the last slot rather than subtracting scrollWidth/clientWidth:
      // reserved scrollbar gutters can make that subtraction differ by 10px.
      const atEnd = !!last && (direction === "horizontal"
        ? host.scrollLeft > 0 && last.right <= bounds.right - 19
        : host.scrollTop > 0 && last.bottom <= bounds.bottom - 19);
      if (atEnd) best = count - 1;
      if (best < 0 || best === current.current) return;
      current.current = best;
      onPage(best * columns + 1);
      const rendered = content.children[best]?.querySelector<HTMLElement>(".editor-pdf-slot-surface");
      onLoading(rendered?.dataset.ready !== "true");
      if (rendered?.dataset.zoom) onZoom(Number(rendered.dataset.zoom));
      onError("");
    };
    const schedule = () => { if (!frame) frame = requestAnimationFrame(updateCurrent); };
    const observer = new IntersectionObserver(entries => {
      for (const entry of entries) {
        const index = Number((entry.target as HTMLElement).dataset.spread);
        if (entry.isIntersecting) visible.current.add(index);
        else visible.current.delete(index);
      }
      const nearby = new Set<number>();
      const radius = Math.ceil(2 / columns);
      // Axis changes briefly invalidate intersection records. Keep the current
      // spread and its prefetched neighbors mounted through that transition.
      for (let neighbor = Math.max(0, current.current - radius); neighbor <= Math.min(count - 1, current.current + radius); neighbor++) nearby.add(neighbor);
      for (const index of visible.current) {
        for (let neighbor = Math.max(0, index - radius); neighbor <= Math.min(count - 1, index + radius); neighbor++) nearby.add(neighbor);
      }
      setActive(previous => previous.size === nearby.size && [...previous].every(index => nearby.has(index)) ? previous : nearby);
      schedule();
    }, { root: host, rootMargin: "0px" });
    visible.current.clear();
    for (const child of content.children) observer.observe(child);
    const wheel = (event: WheelEvent) => {
      if (direction !== "horizontal" || event.ctrlKey || event.shiftKey || Math.abs(event.deltaX) >= Math.abs(event.deltaY)) return;
      const unit = event.deltaMode === 1 ? 16 : event.deltaMode === 2 ? host.clientWidth : 1;
      const next = Math.max(0, Math.min(host.scrollWidth - host.clientWidth, host.scrollLeft + event.deltaY * unit));
      if (next !== host.scrollLeft) { event.preventDefault(); host.scrollLeft = next; }
    };
    host.addEventListener("scroll", schedule, { passive: true });
    host.addEventListener("wheel", wheel, { passive: false });
    return () => { observer.disconnect(); host.removeEventListener("scroll", schedule); host.removeEventListener("wheel", wheel); cancelAnimationFrame(frame); };
  }, [root, natural, columns, direction, count, onPage, onZoom, onLoading, onError]);

  useLayoutEffect(() => {
    const host = root.current;
    const index = current.current;
    const target = flow.current?.children[index] as HTMLElement | undefined;
    if (!host || !target) return;
    setActive(previous => new Set(previous).add(index));
    const bounds = host.getBoundingClientRect();
    const rect = target.getBoundingClientRect();
    host.scrollTo({
      top: direction === "vertical" ? host.scrollTop + rect.top - bounds.top - 20 : 0,
      left: direction === "horizontal" ? host.scrollLeft + rect.left - bounds.left - 20 : 0,
      behavior: "instant",
    });
  }, [root, navigation, columns, direction, natural, size, zoom, fitMode]);

  return <div className={`editor-pdf-flow is-${direction}`} ref={flow}>
    {natural && size.width > 0 && size.height > 0 ? Array.from({ length: count }, (_, index) => <ContinuousSpread key={`${columns}:${index}`} pdf={pdf} index={index} columns={columns} active={active.has(index)} natural={natural} size={size} zoom={zoom} fitMode={fitMode} priority={index === current.current ? 0 : 1} report={report} />) : null}
  </div>;
}
