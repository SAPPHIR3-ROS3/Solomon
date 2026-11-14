import { useEffect, useState, type ReactNode } from "react";
import { CheckIcon, CopyIcon } from "./ChatIcons";
import { copyText } from "./chatClipboard";
import "./mermaid.css";

let renderer: Promise<typeof import("mermaid")["default"]> | undefined;
let nextID = 0;

function loadRenderer() {
  renderer ??= import("mermaid").then(({ default: mermaid }) => {
    const styles = getComputedStyle(document.documentElement);
    const color = (name: string, fallback: string) => styles.getPropertyValue(`--color-${name}`).trim() || fallback;
    const surface = color("surface", "#082a54");
    const raised = color("surface-raised", "#0d3566");
    const text = color("text", "#e8e5df");
    const accent = color("accent", "#3b8fd1");
    mermaid.initialize({
      startOnLoad: false,
      securityLevel: "strict",
      theme: "base",
      themeVariables: {
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

export function MermaidPreview({ code, source }: { code: string; source: ReactNode }) {
  const [view, setView] = useState<"graph" | "source">("graph");
  const [copied, setCopied] = useState<string>();
  const [result, setResult] = useState<{ code: string; svg?: string; error?: string }>();

  useEffect(() => {
    let cancelled = false;
    const timer = window.setTimeout(() => {
      void (async () => {
        try {
          const mermaid = await loadRenderer();
          if (cancelled) return;
          const { svg } = await mermaid.render(`solomon-mermaid-${++nextID}`, code);
          if (!cancelled) setResult({ code, svg });
        } catch (error) {
          if (!cancelled) setResult({ code, error: error instanceof Error ? error.message : String(error) });
        }
      })();
    }, 300);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [code]);

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
        <div className="chat-mermaid-preview" aria-live="polite">
          {current?.svg ? (
            <img src={`data:image/svg+xml;charset=utf-8,${encodeURIComponent(current.svg)}`} alt="Mermaid diagram" />
          ) : (
            <span className="chat-mermaid-status" title={current?.error}>
              {current ? "Preview unavailable. Check the Mermaid source." : "Preparing diagram preview…"}
            </span>
          )}
        </div>
      )}
    </div>
  );
}
