import { createContext, useLayoutEffect, useState, type RefObject } from "react";

export const DiagramUnitContext = createContext(600);

export function useDiagramUnit(view: RefObject<HTMLElement | null>, composer: RefObject<HTMLElement | null>) {
  const [unit, setUnit] = useState(600);
  useLayoutEffect(() => {
    const container = view.current;
    const input = composer.current;
    if (!container || !input) return;
    const measure = () => {
      const top = container.getBoundingClientRect().top + parseFloat(getComputedStyle(container).paddingTop || "0");
      const height = Math.max(200, Math.floor(input.getBoundingClientRect().top - top - 24));
      setUnit(previous => previous === height ? previous : height);
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(container);
    observer.observe(input);
    const dock = input.closest(".chat-composer-dock");
    if (dock) observer.observe(dock);
    window.addEventListener("resize", measure);
    return () => {
      observer.disconnect();
      window.removeEventListener("resize", measure);
    };
  }, [view, composer]);
  return unit;
}

export function diagramSize(svg: string, unit: number) {
  const root = new DOMParser().parseFromString(svg, "image/svg+xml").documentElement;
  const box = root.getAttribute("viewBox")?.trim().split(/[\s,]+/).map(Number);
  const width = box?.[2] || parseFloat(root.getAttribute("width") || "0");
  const height = box?.[3] || parseFloat(root.getAttribute("height") || "0");
  if (!(width > 0 && height > 0)) return undefined;
  const overhead = 120;
  const units = Math.max(1, Math.ceil((height * .75 + overhead) / unit));
  const scale = Math.min(1, Math.max(1, units * unit - overhead) / height);
  return { width: Math.round(width * scale), height: Math.round(height * scale), units };
}
