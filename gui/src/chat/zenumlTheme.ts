export function themeZenUML(svg: string) {
  const palette = getComputedStyle(document.documentElement);
  const color = (name: string, fallback: string) => palette.getPropertyValue(`--color-${name}`).trim() || fallback;
  const surface = color("surface", "#082a54");
  const raised = color("surface-raised", "#0d3566");
  const border = color("border", "#174875");
  const text = color("text", "#e8e5df");
  const muted = color("text-muted", "#9ca3aa");
  const accent = color("accent", "#3b8fd1");
  const documentSVG = new DOMParser().parseFromString(svg, "image/svg+xml");
  documentSVG.querySelectorAll("style[data-solomon-zenuml-theme]").forEach(element => element.remove());
  const style = documentSVG.createElementNS("http://www.w3.org/2000/svg", "style");
  style.setAttribute("data-solomon-zenuml-theme", "2");
  style.textContent = `
    .frame-border-outer { fill: ${border} !important; }
    .frame-border-inner, .frame-header-bg, .group-title-bg { fill: ${surface} !important; }
    .participant-box, .occurrence, .fragment-header, .divider-bg { fill: ${raised} !important; }
    .participant-box, .occurrence, .fragment-border, .group-outline, .divider-bg { stroke: ${accent} !important; }
    .frame-header-line, .fragment-separator { stroke: ${border} !important; }
    .lifeline, .message-line, .return-line, .return-arrow, .divider-line { stroke: ${accent} !important; }
    .arrow-head { fill: ${accent} !important; stroke: ${accent} !important; }
    .arrow-head path, .message path[stroke="#000"] { stroke: ${accent} !important; }
    .arrow-head path[fill="#000"], .message path[fill="#000"] { fill: ${accent} !important; }
    .arrow-open { fill: none !important; }
    .frame-title, .participant-label, .message-label, .fragment-label,
    .fragment-condition, .fragment-section-label, .return-label, .return-icon,
    .divider-label, .group-title-text { fill: ${text} !important; }
    .participant-icon { color: ${text} !important; }
    .participant-icon [fill="currentColor"]:not([stroke]) { stroke: ${accent} !important; }
    .comment-text, .seq-number { fill: ${muted} !important; }
  `;
  documentSVG.documentElement.appendChild(style);
  return new XMLSerializer().serializeToString(documentSVG.documentElement);
}
