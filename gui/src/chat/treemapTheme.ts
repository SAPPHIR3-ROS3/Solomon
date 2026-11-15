export function themeTreemap(svg: string) {
  const palette = getComputedStyle(document.documentElement);
  const color = (name: string, fallback: string) => palette.getPropertyValue(`--color-${name}`).trim() || fallback;
  const surface = color("surface", "#082a54");
  const raised = color("surface-raised", "#0d3566");
  const accent = color("accent", "#3b8fd1");
  const border = color("border", "#174875");
  const text = color("text", "#e8e5df");
  const documentSVG = new DOMParser().parseFromString(svg, "image/svg+xml");
  const style = documentSVG.createElementNS("http://www.w3.org/2000/svg", "style");
  style.setAttribute("data-solomon-treemap-theme", "1");
  style.textContent = `
    .treemapSection > rect:not(.treemapSectionHeader) { fill: ${surface} !important; fill-opacity: 1 !important; stroke: ${border} !important; stroke-opacity: 1 !important; }
    .treemapSectionHeader { fill: ${surface} !important; fill-opacity: 1 !important; }
    .treemapLeaf { fill: ${raised} !important; fill-opacity: 1 !important; stroke: ${accent} !important; }
    .treemapLeafGroup:nth-child(even) .treemapLeaf { fill: ${surface} !important; }
    .treemapSectionLabel, .treemapSectionValue, .treemapLabel, .treemapValue, .treemapTitle { fill: ${text} !important; }
  `;
  documentSVG.documentElement.appendChild(style);
  return new XMLSerializer().serializeToString(documentSVG.documentElement);
}
