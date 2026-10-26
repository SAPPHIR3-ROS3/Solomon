import { useEffect, useRef, useState } from "react";
import { getDocument, GlobalWorkerOptions, type PDFDocumentProxy } from "pdfjs-dist";
import workerURL from "pdfjs-dist/build/pdf.worker.min.mjs?url";
import { serverEndpoint } from "../platform";
import "./pdf.css";
import { renderPdfSpread, prefetchPdfNeighbors, type PdfFitMode } from "./pdfRendering";
import { pdfImageOptions } from "./pdfCompatibility";
import { disposePdfPageCache } from "./pdfPageCache";
import { PdfContinuous } from "./PdfContinuous";

GlobalWorkerOptions.workerSrc = workerURL;

export function PdfPreview({ projectID, path }: { projectID: string; path: string }) {
  const [document, setDocument] = useState<PDFDocumentProxy | null>(null);
  const [page, setPage] = useState(1);
  const [zoom, setZoom] = useState(1);
  const [twoPages, setTwoPages] = useState(false);
  const [continuous, setContinuous] = useState(false);
  const [direction, setDirection] = useState<"vertical" | "horizontal">("vertical");
  const [navigation, setNavigation] = useState({ page: 1, revision: 0 });
  const columns = twoPages ? 2 : 1;
  const firstPage = Math.floor((page - 1) / columns) * columns + 1;
  function goToPage(next: number) {
    const target = Math.max(1, Math.min(document?.numPages ?? 1, next));
    setPage(target);
    setNavigation(current => ({ page: target, revision: current.revision + 1 }));
  }
  function updateLayout() {
    setNavigation(current => ({ page, revision: current.revision + 1 }));
  }
  const [fitMode, setFitMode] = useState<PdfFitMode>(null);
  const [fittedZoom, setFittedZoom] = useState(1);
  const displayedZoom = fitMode ? fittedZoom : zoom;
  const [zoomDraft, setZoomDraft] = useState<string | null>(null);
  function applyZoom(value: number) {
    const next = Math.max(0.25, Math.min(3, value));
    if (next !== zoom || fitMode !== null) setLoading(true);
    setZoom(next);
    setFitMode(null);
    setZoomDraft(null);
  }
  function commitZoom() {
    if (zoomDraft === null) return;
    const value = zoomDraft.trim().replace(/%$/, "").trim().replace(",", ".");
    if (/^\d+(?:\.\d+)?$/.test(value) && Number.isFinite(Number(value))) applyZoom(Number(value) / 100);
    else setZoomDraft(null);
  }
  function changeZoom(delta: number) {
    applyZoom(displayedZoom + delta);
  }
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [showLoading, setShowLoading] = useState(false);
  useEffect(() => {
    if (!loading) { setShowLoading(false); return; }
    const timer = window.setTimeout(() => setShowLoading(true), 180);
    return () => window.clearTimeout(timer);
  }, [loading]);
  const surface = useRef<HTMLDivElement>(null);
  const host = useRef<HTMLDivElement>(null);
  const [size, setSize] = useState({ width: 0, height: 0 });

  useEffect(() => {
    let cancelled = false;
    let task: ReturnType<typeof getDocument> | undefined;
    let loaded: PDFDocumentProxy | undefined;
    void serverEndpoint(`/__solomon/projects/${encodeURIComponent(projectID)}/file?path=${encodeURIComponent(path)}`).then((url) => {
      if (cancelled) return;
      task = getDocument({ url, disableStream: true, disableAutoFetch: true, rangeChunkSize: 65536, ...pdfImageOptions });
      return task.promise.then((pdf) => { loaded = pdf; if (!cancelled) setDocument(pdf); });
    }).catch((reason: unknown) => { if (!cancelled) { setError(reason instanceof Error ? `Unable to display this PDF: ${reason.message}` : "Unable to display this PDF."); setLoading(false); } });
    return () => { cancelled = true; if (loaded) disposePdfPageCache(loaded); void task?.destroy(); };
  }, [projectID, path]);

  useEffect(() => {
    const observer = new ResizeObserver(([entry]) => {
      const width = Math.max(1, Math.floor(entry.contentRect.width));
      const height = Math.max(1, Math.floor(entry.contentRect.height));
      setSize((current) => current.width === width && current.height === height ? current : { width, height });
    });
    if (host.current) observer.observe(host.current);
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    if (!document || !size.width || !size.height || continuous || !surface.current) return;
    let cancelled = false;
    setLoading(true); setError("");
    const numbers = Array.from({ length: Math.min(columns, document.numPages - firstPage + 1) }, (_, index) => firstPage + index);
    const owner = surface.current.ownerDocument;
    let stopPrefetch: (() => void) | undefined;
    const render = renderPdfSpread(document, numbers, owner, size, zoom, fitMode);
    void render.promise.then(result => {
      if (cancelled || !result || !surface.current) return;
      surface.current.replaceChildren(result.element);
      if (fitMode) setFittedZoom(result.zoom);
      setLoading(false);
      stopPrefetch = prefetchPdfNeighbors(document, firstPage, columns, owner, size, zoom, fitMode);
    }).catch((reason: unknown) => { if (!cancelled) { setError(reason instanceof Error ? `Unable to render this PDF page: ${reason.message}` : "Unable to render this PDF page."); setLoading(false); } });
    return () => { cancelled = true; render.cancel(); stopPrefetch?.(); };
  }, [document, firstPage, columns, size, zoom, fitMode, continuous]);

  return <section className="editor-pdf-preview" aria-label="PDF preview">
    <div className="editor-pdf-toolbar">
      <button disabled={!document || firstPage === 1} onClick={() => goToPage(firstPage - columns)} title="Previous page" aria-label="Previous page"><svg viewBox="0 0 24 24"><path d="m14 6-6 6 6 6" /></svg></button>
      <label>Page <input aria-label="PDF page" type="number" min={1} max={document?.numPages ?? 1} value={page} onChange={(event) => { const next = Number(event.target.value); if (document && next >= 1 && next <= document.numPages) goToPage(Math.floor(next)); }} /> / {document?.numPages ?? "…"}</label>
      <button disabled={!document || firstPage + columns > document.numPages} onClick={() => goToPage(firstPage + columns)} title="Next page" aria-label="Next page"><svg viewBox="0 0 24 24"><path d="m10 6 6 6-6 6" /></svg></button>
      <span className="editor-pdf-divider" aria-hidden="true" />
      <button disabled={displayedZoom <= 0.25} onClick={() => changeZoom(-0.25)} title="Zoom out" aria-label="Zoom out"><svg viewBox="0 0 24 24"><path d="M5 12h14" /></svg></button>
      <input className="editor-pdf-zoom" aria-label="Zoom percentage" title="Zoom percentage (25–300%)" type="text" inputMode="decimal" disabled={!document} value={zoomDraft ?? `${Math.round(displayedZoom * 10000) / 100}%`} onFocus={(event) => event.currentTarget.select()} onChange={(event) => setZoomDraft(event.target.value)} onBlur={commitZoom} onKeyDown={(event) => {
        if (event.key === "Enter") { event.preventDefault(); commitZoom(); event.currentTarget.blur(); }
        if (event.key === "Escape") { event.preventDefault(); setZoomDraft(null); }
      }} />
      <button disabled={displayedZoom >= 3} onClick={() => changeZoom(0.25)} title="Zoom in" aria-label="Zoom in"><svg viewBox="0 0 24 24"><path d="M5 12h14M12 5v14" /></svg></button>
      <span className="editor-pdf-divider" aria-hidden="true" />
      <button className="editor-pdf-fit" disabled={!document} onClick={() => { setLoading(true); setFitMode((current) => current === "page" ? "width" : "page"); if (continuous) updateLayout(); else host.current?.scrollTo(0, 0); }} title={fitMode === "page" ? "Fit to width" : "Fit to page"} aria-label={fitMode === "page" ? "Fit to width" : "Fit to page"}>
        {fitMode === "page"
          ? <svg viewBox="0 0 24 24"><path d="M3 4v16M21 4v16M8 8l-4 4 4 4M16 8l4 4-4 4M4 12h16" /></svg>
          : <svg viewBox="0 0 24 24"><path d="M8 3H4v4M16 3h4v4M4 17v4h4M20 17v4h-4" /><rect x="8" y="6" width="8" height="12" rx="1" /></svg>}
      </button>
      <span className="editor-pdf-divider" aria-hidden="true" />
      <button disabled={!document} aria-label="Two-page view" aria-pressed={twoPages} title="Two-page view" onClick={() => { setLoading(true); updateLayout(); setTwoPages(current => !current); }}>
        <svg viewBox="0 0 24 24"><rect x="2" y="4" width="9" height="16" rx="1" /><rect x="13" y="4" width="9" height="16" rx="1" /></svg>
      </button>
      <button disabled={!document} aria-label="Continuous scrolling" aria-pressed={continuous} title="Continuous scrolling" onClick={() => { setLoading(true); updateLayout(); setContinuous(current => !current); }}>
        <svg viewBox="0 0 24 24"><path d="M5 1h14v5H5zM5 10h14v5H5zM5 19h14v4H5z" /></svg>
      </button>
        <button disabled={!continuous} aria-label="Vertical scrolling" aria-pressed={continuous && direction === "vertical"} title="Vertical scrolling" onClick={() => { updateLayout(); setDirection("vertical"); }}><svg viewBox="0 0 24 24"><path d="M12 3v18M7 8l5-5 5 5M7 16l5 5 5-5" /></svg></button>
        <button disabled={!continuous} aria-label="Horizontal scrolling" aria-pressed={continuous && direction === "horizontal"} title="Horizontal scrolling" onClick={() => { updateLayout(); setDirection("horizontal"); }}><svg viewBox="0 0 24 24"><path d="M3 12h18M8 7l-5 5 5 5M16 7l5 5-5 5" /></svg></button>
    </div>
    <div className={`editor-pdf-pages${continuous ? ` is-continuous is-${direction}` : ""}`} ref={host} aria-busy={loading}>
      {showLoading && !error ? <p className="editor-pdf-notice" role="status">Loading PDF…</p> : null}
      {error ? <p className="editor-pdf-notice" role="alert">{error}</p> : null}
      {continuous && document ? <PdfContinuous pdf={document} root={host} columns={columns} direction={direction} size={size} zoom={zoom} fitMode={fitMode} page={page} navigation={navigation} onPage={setPage} onZoom={setFittedZoom} onLoading={setLoading} onError={setError} /> : <div className="editor-pdf-surface" ref={surface} />}
    </div>
  </section>;
}
