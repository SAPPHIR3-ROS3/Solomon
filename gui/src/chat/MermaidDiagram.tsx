import { memo, useContext, useEffect, useMemo, useRef, useState } from "react";
import { diagramSize, DiagramUnitContext } from "./diagramSizing";

export const MermaidDiagram = memo(function MermaidDiagram({ svg, pending, error, isStreaming = false }: { svg?: string; pending: boolean; error?: string; isStreaming?: boolean }) {
  const content = useRef<HTMLDivElement>(null);
  const unit = useContext(DiagramUnitContext);
  const size = useMemo(() => svg ? diagramSize(svg, unit) : undefined, [svg, unit]);
  const initialSVG = useRef(svg);
  const immediate = !isStreaming || Boolean(svg && svg === initialSVG.current);
  const [height, setHeight] = useState(160);
  const image = useMemo(() => svg ? `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}` : undefined, [svg]);

  useEffect(() => {
    if (immediate) return;
    const element = content.current;
    if (!element) return;
    const observer = new ResizeObserver(() => {
      setHeight(Math.max(160, element.getBoundingClientRect().height));
    });
    observer.observe(element);
    return () => observer.disconnect();
  }, [immediate]);

  return (
    <div className={`chat-mermaid-preview${immediate ? " chat-mermaid-immediate" : ""}`} style={{ height: immediate ? undefined : height }} aria-busy={pending}>
      <div className="chat-mermaid-content" ref={content}>
        {image ? <img key={svg} src={image} alt="Mermaid diagram" style={size ? { width: size.width, maxHeight: size.height, objectFit: "contain" } : undefined} data-diagram-units={size?.units} /> : null}
        {!svg ? (
          <div className="chat-mermaid-placeholder">
            <span className="chat-mermaid-status" title={error}>
              {pending ? (isStreaming ? "Preparing diagram preview…" : "") : "Preview unavailable. Check the Mermaid source."}
            </span>
          </div>
        ) : null}
      </div>
      {svg && (pending || error) ? (
        <span className="chat-mermaid-update chat-mermaid-status" role="status" title={error}>
          {pending ? "Updating diagram…" : "Preview unavailable for the latest source. Showing the previous diagram."}
        </span>
      ) : null}
    </div>
  );
});
