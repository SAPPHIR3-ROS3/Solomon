import { useEffect, useLayoutEffect, useRef, useState, type RefObject } from "react";
import { readViewState, writeViewState } from "../projects/viewState";

export type FilePosition = { top: number; left: number; anchor?: number; head?: number; scrollAnchor?: number; scrollOffset?: number; page?: number; layout?: string };
const pending = new Map<string, { position: FilePosition; timer: ReturnType<typeof setTimeout> }>();

function flush(path: string) {
  const item = pending.get(path);
  if (!item) return;
  clearTimeout(item.timer);
  pending.delete(path);
  void writeViewState(path, item.position).catch(() => undefined);
}

if (typeof window !== "undefined") {
  const flushAll = () => { for (const path of pending.keys()) flush(path); };
  window.addEventListener("pagehide", flushAll);
  document.addEventListener("visibilitychange", () => {
    if (document.visibilityState === "hidden") flushAll();
  });
}

export function useFilePosition(projectID: string, file: string, view: "source" | "preview", fallback?: FilePosition) {
  const path = `/__solomon/projects/${encodeURIComponent(projectID)}/file-position?path=${encodeURIComponent(file)}&view=${view}`;
  const [loaded, setLoaded] = useState<{ path: string; position: FilePosition | null }>();
  const legacy = useRef(fallback);
  legacy.current = fallback;
  const position = loaded?.path === path ? loaded.position : undefined;

  useEffect(() => {
    let cancelled = false;
    const recent = pending.get(path)?.position;
    flush(path);
    void (recent ? Promise.resolve(recent) : readViewState<FilePosition>(path).catch(() => null)).then((stored) => {
      if (!cancelled) setLoaded({ path, position: stored ?? legacy.current ?? null });
    });
    return () => { cancelled = true; };
  }, [path]);

  function save(next: FilePosition) {
    const previous = pending.get(path);
    if (previous) clearTimeout(previous.timer);
    const position = { ...next, top: Math.max(0, next.top), left: Math.max(0, next.left) };
    pending.set(path, { position, timer: setTimeout(() => flush(path), 250) });
  }

  return { position, save };
}

export function useViewportPosition(
  host: RefObject<HTMLElement | null>,
  position: FilePosition | null | undefined,
  save: (position: FilePosition) => void,
  ready = true,
  page?: number,
  layout?: string,
) {
  const applied = useRef<FilePosition | null | undefined>(undefined);
  useLayoutEffect(() => {
    if (!ready || position === undefined || !host.current || applied.current === position) return;
    applied.current = position;
    if (position) host.current.scrollTo({ top: position.top, left: position.left, behavior: "instant" });
  }, [host, position, ready]);

  function remember() {
    if (position === undefined || applied.current !== position || !host.current) return;
    save({ top: Math.max(0, host.current.scrollTop), left: Math.max(0, host.current.scrollLeft), page, layout });
  }

  return remember;
}
