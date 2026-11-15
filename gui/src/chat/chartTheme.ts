export function chartPalette() {
  const palette = getComputedStyle(document.documentElement);
  const color = (name: string, fallback: string) => palette.getPropertyValue(`--color-${name}`).trim() || fallback;
  return {
    series: [color("accent", "#3b8fd1"), color("crown-gold", "#ffc704"), color("text", "#e8e5df"), color("success", "#237a52"), color("danger", "#a83b3b"), color("text-muted", "#9ca3aa")],
    border: color("border", "#174875"),
    text: color("text", "#e8e5df"),
  };
}

export function themeRadar(svg: string) {
  const { series, border, text } = chartPalette();
  const documentSVG = new DOMParser().parseFromString(svg, "image/svg+xml");
  const style = documentSVG.createElementNS("http://www.w3.org/2000/svg", "style");
  style.setAttribute("data-solomon-radar-theme", "1");
  style.textContent = `
    .radarGraticule { fill: none !important; stroke: ${border} !important; stroke-opacity: 1 !important; }
    .radarAxisLine { stroke: ${border} !important; }
    .radarAxisLabel, .radarTitle, .radarLegendText { fill: ${text} !important; color: ${text} !important; }
    ${Array.from({ length: 12 }, (_, index) => `
      .radarCurve-${index}, .radarLegendBox-${index} {
        fill: ${series[index % series.length]} !important;
        stroke: ${series[index % series.length]} !important;
        fill-opacity: .18 !important;
        stroke-width: 2 !important;
      }
      .radarLegendBox-${index} { fill-opacity: .8 !important; }
    `).join("\n")}
  `;
  documentSVG.documentElement.appendChild(style);
  return new XMLSerializer().serializeToString(documentSVG.documentElement);
}

export function themeSankey(svg: string) {
  const { series, text } = chartPalette();
  const documentSVG = new DOMParser().parseFromString(svg, "image/svg+xml");
  const colors = new Map<string, string>();
  documentSVG.querySelectorAll(".nodes .node rect").forEach((node, index) => {
    const previous = node.getAttribute("fill");
    const next = series[index % series.length];
    if (previous) colors.set(previous, next);
    node.setAttribute("fill", next);
  });
  documentSVG.querySelectorAll(".links path").forEach(path => {
    const stroke = path.getAttribute("stroke") || "";
    const gradientID = stroke.match(/#([^)'"\s]+)/)?.[1];
    const gradient = gradientID ? documentSVG.getElementById(gradientID) : null;
    const target = gradient?.querySelector("stop:last-child")?.getAttribute("stop-color") || stroke;
    path.setAttribute("stroke", colors.get(target) || target);
  });
  documentSVG.querySelectorAll(".links linearGradient, style[data-solomon-sankey-theme]").forEach(element => element.remove());
  const style = documentSVG.createElementNS("http://www.w3.org/2000/svg", "style");
  style.setAttribute("data-solomon-sankey-theme", "2");
  style.textContent = `
    .links .link { mix-blend-mode: normal !important; }
    .links { stroke-opacity: 1 !important; }
    .node-labels text { fill: ${text} !important; }
  `;
  documentSVG.documentElement.appendChild(style);
  return new XMLSerializer().serializeToString(documentSVG.documentElement);
}

export function themeKanban(svg: string) {
  const palette = getComputedStyle(document.documentElement);
  const color = (name: string, fallback: string) => palette.getPropertyValue(`--color-${name}`).trim() || fallback;
  const surface = color("surface", "#082a54");
  const raised = color("surface-raised", "#0d3566");
  const border = color("border", "#174875");
  const accent = color("accent", "#3b8fd1");
  const text = color("text", "#e8e5df");
  const documentSVG = new DOMParser().parseFromString(svg, "image/svg+xml");
  const style = documentSVG.createElementNS("http://www.w3.org/2000/svg", "style");
  style.setAttribute("data-solomon-kanban-theme", "1");
  style.textContent = `
    .sections rect { fill: ${surface} !important; stroke: ${border} !important; stroke-width: 1 !important; }
    .items rect { fill: ${raised} !important; stroke: ${accent} !important; stroke-width: 1 !important; }
    .sections text, .items text { fill: ${text} !important; }
    .sections .label, .items .label { color: ${text} !important; }
  `;
  documentSVG.documentElement.appendChild(style);
  return new XMLSerializer().serializeToString(documentSVG.documentElement);
}
