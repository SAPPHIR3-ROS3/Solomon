import type { PDFDocumentProxy } from "pdfjs-dist";
import { preparePdfPage } from "./pdfCompatibility";
import { pdfPageCache } from "./pdfPageCache";

export type PdfFitMode = "page" | "width" | null;
export const PDF_TO_CSS = 96 / 72;
export const PDF_PAGE_GAP = 16;

export function spreadGeometry(widths: number[], heights: number[], available: { width: number; height: number }, zoom: number, fitMode: PdfFitMode) {
  const gap = (widths.length - 1) * PDF_PAGE_GAP;
  const width = widths.reduce((sum, value) => sum + value, 0);
  const height = Math.max(...heights);
  const fittedWidth = Math.max(1, available.width - gap) / width;
  const effectiveZoom = fitMode === "page" ? Math.min(fittedWidth, available.height / height) : fitMode === "width" ? fittedWidth : zoom;
  return { zoom: effectiveZoom, width: width * effectiveZoom + gap, height: height * effectiveZoom };
}

// Cached immutable bitmaps survive scrolling and view changes. The visible
// surface receives its own copy, so attaching a page never steals another one.
export function renderPdfSpread(pdf: PDFDocumentProxy, numbers: number[], owner: Document, available: { width: number; height: number }, zoom: number, fitMode: PdfFitMode, pixelBudget = 4000000, priority = 0) {
  let cancelled = false;
  const promise = (async () => {
    const pages = await Promise.all(numbers.map(number => pdf.getPage(number)));
    if (cancelled) return null;
    const natural = pages.map(page => page.getViewport({ scale: PDF_TO_CSS }));
    const geometry = spreadGeometry(natural.map(view => view.width), natural.map(view => view.height), available, zoom, fitMode);
    const element = owner.createElement("div");
    element.className = "editor-pdf-spread";
    const bitmaps = await Promise.all(pages.map(async (page, index) => {
      const viewport = page.getViewport({ scale: PDF_TO_CSS * geometry.zoom });
      const ratio = Math.min(window.devicePixelRatio || 1, 2, Math.sqrt(pixelBudget / pages.length / (viewport.width * viewport.height)));
      const width = Math.max(1, Math.floor(viewport.width * ratio));
      const height = Math.max(1, Math.floor(viewport.height * ratio));
      const key = `${numbers[index]}:${width}x${height}:${viewport.width.toFixed(4)}x${viewport.height.toFixed(4)}`;
      return pdfPageCache(pdf).canvas(key, owner, async () => {
        const canvas = owner.createElement("canvas");
        canvas.setAttribute("aria-label", `PDF page ${numbers[index]}`);
        canvas.style.visibility = "visible";
        canvas.width = width; canvas.height = height;
        canvas.style.width = `${viewport.width}px`;
        canvas.style.height = `${viewport.height}px`;
        await preparePdfPage(page);
        const task = page.render({ canvas, viewport, transform: [ratio, 0, 0, ratio, 0, 0] });
        try { await task.promise; return canvas; }
        finally { page.cleanup(); }
      }, priority);
    }));
    if (cancelled) return null;
    element.append(...bitmaps);
    return { element, ...geometry, naturalWidths: natural.map(view => view.width), naturalHeights: natural.map(view => view.height) };
  })();
  // Complete shared work even if its first consumer scrolls away.
  return { promise, cancel() { cancelled = true; } };
}

export function prefetchPdfNeighbors(pdf: PDFDocumentProxy, first: number, columns: number, owner: Document, available: { width: number; height: number }, zoom: number, fitMode: PdfFitMode) {
  const tasks: Array<ReturnType<typeof renderPdfSpread>> = [];
  for (let distance = 1; distance <= Math.ceil(2 / columns); distance++) {
    for (const start of [first + columns * distance, first - columns * distance]) {
      if (start < 1 || start > pdf.numPages) continue;
      const numbers = Array.from({ length: Math.min(columns, pdf.numPages - start + 1) }, (_, index) => start + index);
      const task = renderPdfSpread(pdf, numbers, owner, available, zoom, fitMode, 4000000, 1);
      void task.promise.catch(() => undefined);
      tasks.push(task);
    }
  }
  return () => { for (const task of tasks) task.cancel(); };
}
