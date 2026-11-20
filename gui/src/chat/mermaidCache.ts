import { themeDiagram } from "./diagramTheme";

const storageKey = "solomon.mermaid.svg.v6";
const diagrams = new Map<string, string>();
let loaded = false;
let chars = 0;
let saveTimer: number | undefined;

function save() {
  saveTimer = undefined;
  let size = 0;
  const entries = [...diagrams].reverse().filter(([code, svg]) => {
    size += code.length + svg.length;
    return size <= 2_000_000;
  }).reverse();
  try {
    window.localStorage.setItem(storageKey, JSON.stringify(entries));
  } catch {
    return;
  }
}

function trim() {
  while (diagrams.size > 512 || chars > 16_000_000) {
    const oldest = diagrams.keys().next().value!;
    chars -= oldest.length + diagrams.get(oldest)!.length;
    diagrams.delete(oldest);
  }
}

export function cachedDiagram(code: string) {
  if (!loaded && typeof window !== "undefined") {
    loaded = true;
    window.addEventListener("pagehide", save);
    try {
      window.localStorage.removeItem("solomon.mermaid.svg.v1");
      window.localStorage.removeItem("solomon.mermaid.svg.v2");
      window.localStorage.removeItem("solomon.mermaid.svg.v3");
      window.localStorage.removeItem("solomon.mermaid.svg.v4");
      window.localStorage.removeItem("solomon.mermaid.svg.v5");
      const entries: unknown = JSON.parse(window.localStorage.getItem(storageKey) || "[]");
      if (Array.isArray(entries)) for (const entry of entries) {
        if (!Array.isArray(entry) || entry.length !== 2 || !entry.every(value => typeof value === "string")) continue;
        const [source, originalSVG] = entry as [string, string];
        const svg = themeDiagram(source, originalSVG);
        if (diagrams.has(source)) continue;
        diagrams.set(source, svg);
        chars += source.length + svg.length;
      }
      trim();
    } catch {
      loaded = true;
    }
  }
  return diagrams.get(code);
}

export function cacheDiagram(code: string, svg: string) {
  if (diagrams.has(code) || code.length + svg.length > 16_000_000) return;
  diagrams.set(code, svg);
  chars += code.length + svg.length;
  trim();
  if (saveTimer !== undefined) window.clearTimeout(saveTimer);
  saveTimer = window.setTimeout(save, 250);
}
