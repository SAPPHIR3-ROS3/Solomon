import { themeZenUML } from "./zenumlTheme";
import { themeTreemap } from "./treemapTheme";
import { themeKanban, themeRadar, themeSankey } from "./chartTheme";

const version = "1";

export function themeDiagram(code: string, original: string) {
  if (original.includes(`data-solomon-diagram-theme="${version}"`)) return original;
  let svg = original;
  if (/^\s*zenuml\b/i.test(code) && !svg.includes('data-solomon-zenuml-theme="2"')) svg = themeZenUML(svg);
  if (/class="[^"]*treemapContainer/.test(svg) && !svg.includes('data-solomon-treemap-theme="1"')) svg = themeTreemap(svg);
  if (/class="radarGraticule"/.test(svg) && !svg.includes('data-solomon-radar-theme="1"')) svg = themeRadar(svg);
  if (/class="links"/.test(svg) && /class="nodes"/.test(svg) && !svg.includes('data-solomon-sankey-theme="2"')) svg = themeSankey(svg);
  if (/class="sections"/.test(svg) && /class="items"/.test(svg) && !svg.includes('data-solomon-kanban-theme="1"')) svg = themeKanban(svg);
  return removeBlack(svg);
}

function removeBlack(svg: string) {
  const palette = getComputedStyle(document.documentElement);
  const color = (name: string, fallback: string) => palette.getPropertyValue(`--color-${name}`).trim() || fallback;
  const surface = color("surface", "#082a54");
  const text = color("text", "#e8e5df");
  const accent = color("accent", "#3b8fd1");
  const canvas = document.createElement("canvas");
  canvas.width = canvas.height = 1;
  const context = canvas.getContext("2d", { willReadFrequently: true });
  const checked = new Map<string, number>();
  const isBlack = (value: string) => {
    if (checked.has(value)) return checked.get(value)!;
    if (!context || !CSS.supports("color", value)) return 0;
    context.clearRect(0, 0, 1, 1);
    context.fillStyle = value;
    context.fillRect(0, 0, 1, 1);
    const [r, g, b, alpha] = context.getImageData(0, 0, 1, 1).data;
    const black = alpha > 0 && Math.max(r, g, b) <= 48 ? alpha / 255 : 0;
    checked.set(value, black);
    return black;
  };
  const replacement = (property: string, target: string) => {
    if (/color$/.test(property) && !/background|border|stop|flood/.test(property)) return text;
    if (/stroke|stop-color/.test(property) || /\b(?:arrow[\w-]*|marker[\w-]*|edge[\w-]*|link[\w-]*|signal[\w-]*|curve[\w-]*|line[\w-]*)\b/i.test(target)) return accent;
    return /text|tspan|label|title|foreignObject/i.test(target) ? text : surface;
  };
  const paint = (value: string, property: string, target: string) => {
    const important = /\s*!important\s*$/i.exec(value)?.[0] || "";
    const clean = value.replace(/\s*!important\s*$/i, "").trim();
    const alpha = isBlack(clean);
    if (!alpha) return value;
    const next = replacement(property, `${target} ${property}`);
    return alpha < 1 ? `color-mix(in srgb, ${next} ${alpha * 100}%, transparent)${important}` : next + important;
  };
  const declarations = (value: string, target: string) => value.replace(/(--[\w-]+|fill|stroke|color|background(?:-color)?|stop-color|flood-color|border-color)\s*:\s*([^;{}]+)/gi,
    (_match, property: string, original: string) => `${property}:${paint(original, property, target)}`);
  const documentSVG = new DOMParser().parseFromString(svg, "image/svg+xml");
  for (const element of documentSVG.querySelectorAll("*")) {
    const owner = element.closest("marker, .arrow-head, .message, .edge, .link");
    const target = `${element.localName} ${element.getAttribute("class") || ""} ${element.parentElement?.getAttribute("class") || ""} ${owner?.localName || ""} ${owner?.getAttribute("class") || ""}`;
    for (const property of ["fill", "stroke", "color", "stop-color", "flood-color"]) {
      const value = element.getAttribute(property);
      if (value) element.setAttribute(property, paint(value, property, target));
    }
    const style = element.getAttribute("style");
    if (style) element.setAttribute("style", declarations(style, target));
    if (element.localName === "style") element.textContent = (element.textContent || "").replace(/([^{}]+)\{([^{}]*)\}/g,
      (_match, selector: string, body: string) => `${selector}{${declarations(body, selector)}}`);
  }
  const root = documentSVG.documentElement;
  root.setAttribute("data-solomon-diagram-theme", version);
  if (!root.hasAttribute("fill")) root.setAttribute("fill", surface);
  root.setAttribute("color", text);
  const defaults = documentSVG.createElementNS("http://www.w3.org/2000/svg", "style");
  defaults.textContent = `text, tspan { fill: ${text}; }`;
  root.insertBefore(defaults, root.firstChild);
  return new XMLSerializer().serializeToString(root);
}
