import type { PDFDocumentProxy } from "pdfjs-dist";

type Entry = { canvas?: HTMLCanvasElement; blob?: Blob; stored: boolean; width: number; height: number; cssWidth: string; cssHeight: string; label: string };
const HOT_BYTES = 48 * 1024 * 1024;
const caches = new WeakMap<PDFDocumentProxy, PdfPageCache>();
let database: Promise<IDBDatabase | null> | undefined;

function openDatabase() {
  return database ??= new Promise<IDBDatabase | null>(resolve => {
    try {
      const request = indexedDB.open("solomon-pdf-bitmaps", 1);
      request.onupgradeneeded = () => request.result.createObjectStore("pages");
      request.onsuccess = () => resolve(request.result);
      request.onerror = request.onblocked = () => resolve(null);
    } catch { resolve(null); }
  });
}

async function diskPage(key: string, blob?: Blob): Promise<Blob | null> {
  const db = await openDatabase();
  if (!db) return null;
  return new Promise((resolve, reject) => {
    const tx = db.transaction("pages", blob ? "readwrite" : "readonly");
    const request = blob ? tx.objectStore("pages").put(blob, key) : tx.objectStore("pages").get(key);
    let result: Blob | null = null;
    request.onsuccess = () => { result = blob ?? request.result ?? null; };
    tx.oncomplete = () => resolve(result);
    tx.onerror = tx.onabort = () => reject(tx.error);
  });
}

class PdfPageCache {
  private prefix = `${Date.now()}-${Math.random()}:`;
  private entries = new Map<string, Entry>();
  private pending = new Map<string, Promise<Entry>>();
  private writes = new Set<Promise<void>>();
  private disposed = false;
  private running = 0;
  private queue: Array<{ key: string; priority: number; run: () => void }> = [];

  private schedule<T>(key: string, create: () => Promise<T>, priority: number): Promise<T> {
    return new Promise<T>((resolve, reject) => {
      this.queue.push({ key, priority, run: () => {
        this.running++;
        void Promise.resolve().then(() => {
          if (this.disposed) throw new Error("PDF closed");
          return create();
        }).then(resolve, reject).finally(() => { this.running--; this.drain(); });
      } });
      this.queue.sort((a, b) => a.priority - b.priority);
      this.drain();
    });
  }

  private drain() {
    while (this.running < 2 && this.queue.length) this.queue.shift()!.run();
  }

  private trim() {
    let bytes = [...this.entries.values()].reduce((sum, entry) => sum + (entry.canvas ? entry.width * entry.height * 4 : 0), 0);
    for (const entry of this.entries.values()) {
      if (bytes <= HOT_BYTES) break;
      if (!entry.canvas || (!entry.stored && !entry.blob)) continue;
      bytes -= entry.width * entry.height * 4;
      entry.canvas.width = entry.canvas.height = 0;
      entry.canvas = undefined;
    }
  }

  private persist(key: string, entry: Entry) {
    const write = (async () => {
      const canvas = entry.canvas;
      if (!canvas) return;
      const blob = await new Promise<Blob | null>(resolve => canvas.toBlob(resolve, "image/png"));
      if (!blob || this.disposed) return;
      entry.blob = blob;
      try {
        if (await diskPage(this.prefix + key, blob)) { entry.stored = true; entry.blob = undefined; }
      } catch { /* Retain the compressed bitmap when browser storage is unavailable. */ }
      if (!this.disposed) this.trim();
    })().catch(() => { /* Keep the original bitmap if encoding fails. */ });
    this.writes.add(write);
    void write.finally(() => this.writes.delete(write));
  }

  async canvas(key: string, owner: Document, create: () => Promise<HTMLCanvasElement>, priority = 0): Promise<HTMLCanvasElement> {
    if (this.disposed) throw new Error("PDF closed");
    let entry = this.entries.get(key);
    if (!entry) {
      let request = this.pending.get(key);
      if (!request) {
        request = this.schedule(key, create, priority).then(canvas => {
          if (this.disposed) throw new Error("PDF closed");
          const value: Entry = { canvas, stored: false, width: canvas.width, height: canvas.height, cssWidth: canvas.style.width, cssHeight: canvas.style.height, label: canvas.getAttribute("aria-label") ?? "" };
          this.entries.set(key, value);
          this.persist(key, value);
          return value;
        }).finally(() => this.pending.delete(key));
        this.pending.set(key, request);
      }
      const queued = this.queue.find(job => job.key === key);
      if (queued && priority < queued.priority) {
        queued.priority = priority;
        this.queue.sort((a, b) => a.priority - b.priority);
      }
      entry = await request;
    }
    // Refresh LRU order without evicting the compressed on-disk page.
    this.entries.delete(key);
    this.entries.set(key, entry);
    let source: CanvasImageSource | undefined = entry.canvas;
    let image: HTMLImageElement | undefined;
    let objectURL = "";
    if (!source) {
      const blob = entry.blob ?? await diskPage(this.prefix + key);
      if (!blob) { this.entries.delete(key); return this.canvas(key, owner, create, priority); }
      image = owner.createElement("img");
      objectURL = URL.createObjectURL(blob);
      await new Promise<void>((resolve, reject) => { image!.onload = () => resolve(); image!.onerror = () => reject(new Error("Unable to decode cached PDF page")); image!.src = objectURL; });
      source = image;
    }
    try {
      const canvas = owner.createElement("canvas");
      canvas.width = entry.width; canvas.height = entry.height;
      canvas.style.width = entry.cssWidth; canvas.style.height = entry.cssHeight;
      canvas.style.visibility = "visible";
      canvas.setAttribute("aria-label", entry.label);
      const context = canvas.getContext("2d", { alpha: false });
      if (!context) throw new Error("Canvas unavailable");
      context.drawImage(source, 0, 0);
      if (!entry.canvas && !this.disposed) {
        const master = owner.createElement("canvas");
        master.width = entry.width; master.height = entry.height;
        master.getContext("2d", { alpha: false })?.drawImage(canvas, 0, 0);
        entry.canvas = master;
        this.trim();
      }
      return canvas;
    } finally { if (objectURL) URL.revokeObjectURL(objectURL); }
  }

  dispose() {
    this.disposed = true;
    for (const entry of this.entries.values()) if (entry.canvas) entry.canvas.width = entry.canvas.height = 0;
    this.entries.clear();
    void Promise.allSettled([...this.writes]).then(async () => {
      const db = await openDatabase();
      if (!db) return;
      const request = db.transaction("pages", "readwrite").objectStore("pages").openCursor(IDBKeyRange.bound(this.prefix, this.prefix + "\uffff"));
      request.onsuccess = () => { const cursor = request.result; if (cursor) { cursor.delete(); cursor.continue(); } };
    }).catch(() => undefined);
  }
}

export function pdfPageCache(pdf: PDFDocumentProxy) {
  let cache = caches.get(pdf);
  if (!cache) { cache = new PdfPageCache(); caches.set(pdf, cache); }
  return cache;
}

export function disposePdfPageCache(pdf: PDFDocumentProxy) {
  caches.get(pdf)?.dispose();
  caches.delete(pdf);
}
