import { useEffect, useState, type ReactNode } from "react";
import { CheckIcon, CopyIcon } from "./ChatIcons";
import { copyText } from "./chatClipboard";
import { MermaidDiagram } from "./MermaidDiagram";
import "./mermaid.css";
import { cachedDiagram, cacheDiagram } from "./mermaidCache";
import { chartPalette } from "./chartTheme";
import { themeDiagram } from "./diagramTheme";
import { normalizeMermaidSource } from "./mermaidSource";

let renderer: Promise<typeof import("mermaid")["default"]> | undefined;
let zenUML: Promise<void> | undefined;
let nextID = 0;
const rendering = new Map<string, Promise<string>>();
function loadRenderer() {
  renderer ??= import("mermaid").then(({ default: mermaid }) => {
    const styles = getComputedStyle(document.documentElement);
    const color = (name: string, fallback: string) => styles.getPropertyValue(`--color-${name}`).trim() || fallback;
    const surface = color("surface", "#082a54");
    const raised = color("surface-raised", "#0d3566");
    const text = color("text", "#e8e5df");
    const accent = color("accent", "#3b8fd1");
    const { series } = chartPalette();
    mermaid.initialize({
      startOnLoad: false,
      securityLevel: "strict",
      theme: "base",
      themeVariables: {
        ...Object.fromEntries(Array.from({ length: 12 }, (_, index) => [`cScale${index}`, series[index % series.length]])),
        darkMode: true,
        background: surface,
        primaryColor: raised,
        primaryTextColor: text,
        primaryBorderColor: accent,
        secondaryColor: surface,
        secondaryTextColor: text,
        secondaryBorderColor: accent,
        tertiaryColor: color("canvas", "#061c3b"),
        tertiaryTextColor: text,
        tertiaryBorderColor: color("border", "#174875"),
        lineColor: accent,
        textColor: text,
        mainBkg: raised,
        nodeBorder: accent,
        clusterBkg: surface,
        clusterBorder: color("border", "#174875"),
        edgeLabelBackground: surface,
        actorBkg: raised,
        actorBorder: accent,
        actorTextColor: text,
        actorLineColor: accent,
        signalColor: text,
        signalTextColor: text,
        labelBoxBkgColor: surface,
        labelBoxBorderColor: accent,
        labelTextColor: text,
        loopTextColor: text,
        noteBkgColor: raised,
        noteTextColor: text,
        noteBorderColor: accent,
      },
      htmlLabels: false,
      elk: { mergeEdges: false, straightenEdges: false, lineHops: "gap" },
      sankey: { linkColor: "target" },
      suppressErrorRendering: true,
      maxTextSize: 50_000,
    });
    return mermaid;
  }).catch(error => {
    renderer = undefined;
    throw error;
  });
  return renderer;
}

export function MermaidPreview({ code, source, isStreaming = false }: { code: string; source: ReactNode; isStreaming?: boolean }) {
  const [view, setView] = useState<"graph" | "source">("graph");
  const [copied, setCopied] = useState<string>();
  const [result, setResult] = useState<{ code: string; svg?: string; error?: string } | undefined>(() => {
    const svg = cachedDiagram(code);
    return svg ? { code, svg } : undefined;
  });
  const [lastSVG, setLastSVG] = useState<string | undefined>(() => cachedDiagram(code));

  useEffect(() => {
    const cached = cachedDiagram(code);
    if (cached) {
      setResult(previous => previous?.code === code && previous.svg === cached ? previous : { code, svg: cached });
      setLastSVG(cached);
      return;
    }
    let cancelled = false;
    const timer = window.setTimeout(() => {
      void (async () => {
        try {
          const ready = cachedDiagram(code);
          let task = ready ? Promise.resolve(ready) : rendering.get(code);
          if (!task) {
            task = (async () => {
              const mermaid = await loadRenderer();
              if (/^\s*zenuml\b/i.test(code)) {
                zenUML ??= import("@mermaid-js/mermaid-zenuml").then(async ({ default: diagram }) => {
                  await mermaid.registerExternalDiagrams([diagram]);
                }).catch(error => {
                  zenUML = undefined;
                  throw error;
                });
                await zenUML;
              }
              const rendered = await mermaid.render(`solomon-mermaid-${++nextID}`, normalizeMermaidSource(code));
              const svg = themeDiagram(code, rendered.svg);
              cacheDiagram(code, svg);
              return svg;
            })().finally(() => rendering.delete(code));
            rendering.set(code, task);
          }
          const svg = await task;
          if (!cancelled) {
            setResult({ code, svg });
            setLastSVG(svg);
          }
        } catch (error) {
          if (!cancelled) setResult({ code, error: error instanceof Error ? error.message : String(error) });
        }
      })();
    }, isStreaming ? 300 : 0);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [code, isStreaming]);

  const current = result?.code === code ? result : undefined;
  const value = view === "graph" ? current?.svg : code;
  const label = view === "graph" ? "Copy SVG" : "Copy text";

  async function copy() {
    if (!value) return;
    try {
      await copyText(value);
      setCopied(value);
      window.setTimeout(() => setCopied(undefined), 1400);
    } catch {
      setCopied(undefined);
    }
  }

  return (
    <div className="chat-code-block">
      <div className="chat-code-toolbar">
        <div className="chat-code-language">Mermaid</div>
        <div className="chat-mermaid-switch" role="group" aria-label="Mermaid view">
          <button type="button" aria-pressed={view === "graph"} onClick={() => setView("graph")}>Graph</button>
          <button type="button" aria-pressed={view === "source"} onClick={() => setView("source")}>Source</button>
        </div>
        <button className="chat-copy-code" type="button" aria-label={label} title={copied === value && value ? "Copied" : label} disabled={!value} onClick={() => void copy()}>
          {copied === value && value ? <CheckIcon /> : <CopyIcon />}
        </button>
      </div>
      {view === "source" ? source : (
        <MermaidDiagram svg={lastSVG} pending={!current} error={current?.error} isStreaming={isStreaming} />
      )}
    </div>
  );
}
