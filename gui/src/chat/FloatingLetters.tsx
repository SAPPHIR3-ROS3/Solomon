import { useEffect, useId, useLayoutEffect, useRef, type RefObject } from "react";
import "./floating-letters.css";
import { reconcileFloatingText } from "./floatingLetterDiff";
import type { FloatingLettersDensity } from "../settings/chatPreferences";

type Props = {
  content: string;
  density?: FloatingLettersDensity;
  messageID?: string;
  responseKey?: string;
  working: boolean;
  viewRef: RefObject<HTMLElement | null>;
};
type Particle = { x: number; y: number; phase: number; glyph: string };
type Flight = { messageID: string; offset: number; range: Range; glyph: string; x: number; y: number; targetX: number; targetY: number; visible: boolean; start: number; font: string; color: string };
type HighlightAPI = {
  Highlight?: new (...ranges: Range[]) => { add(range: Range): void; delete(range: Range): void; clear(): void };
  CSS: typeof CSS & { highlights?: Map<string, { add(range: Range): void; delete(range: Range): void; clear(): void }> };
};
const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ";
const MAX_FLIGHTS = 100;
const DURATION = 460;

// This layer only paints decoration. The original Markdown remains selectable,
// available to assistive technology, and authoritative throughout streaming.
export function FloatingLetters({ content, density = "medium", messageID, responseKey = messageID, working, viewRef }: Props) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const latest = useRef({ messageID, responseKey, working });
  const scanRef = useRef<() => void>(() => {});
  const highlightName = `floating-letters-${useId().replace(/[^a-zA-Z0-9-]/g, "")}`;
  useLayoutEffect(() => {
    latest.current = { messageID, responseKey, working };
    scanRef.current();
  }, [content, messageID, responseKey, working]);

  useEffect(() => {
    const canvas = canvasRef.current;
    const view = viewRef.current;
    const shell = view?.querySelector<HTMLElement>(".chat-messages-shell");
    const context = canvas?.getContext("2d");
    if (!canvas || !view || !shell || !context) return;
    const dock = view.querySelector<HTMLElement>(".chat-composer-dock");
    const motion = window.matchMedia("(prefers-reduced-motion: reduce)");
    const api = window as unknown as HighlightAPI;
    const highlight = api.Highlight && api.CSS.highlights ? new api.Highlight() : undefined;
    if (highlight) api.CSS.highlights!.set(highlightName, highlight);
    let width = 0, height = 0, frame = 0, previousFrame = 0;
    let seedComposerSides = true;
    const renderedResponses = new Map<string, string>();
    let initialized = false;
    let flights: Flight[] = [];
    let geometryDirty = true;
    let canvasBounds = canvas.getBoundingClientRect();
    const cardSelector = ".chat-message.is-user, .chat-tool-card, .chat-subagent-card, .welcome-composer";
    const visibleCards = new Set<HTMLElement>();
    const watchedCards = new Set<HTMLElement>();
    let masksDirty = true;
    let masks: { x: number; y: number; width: number; height: number; radii: number[] }[] = [];
    const cardSizing = new ResizeObserver(invalidateGeometry);
    const visibility = new IntersectionObserver(entries => {
      for (const entry of entries) {
        const element = entry.target as HTMLElement;
        if (entry.isIntersecting) {
          visibleCards.add(element);
          cardSizing.observe(element);
        } else {
          visibleCards.delete(element);
          cardSizing.unobserve(element);
        }
      }
      masksDirty = true;
    }, { root: view });
    function watchCards(node: Node, added: boolean) {
      if (!(node instanceof HTMLElement)) return;
      const cards = [...(node.matches(cardSelector) ? [node] : []), ...node.querySelectorAll<HTMLElement>(cardSelector)];
      for (const card of cards) {
        if (added) {
          if (!watchedCards.has(card)) { watchedCards.add(card); visibility.observe(card); }
        } else {
          watchedCards.delete(card);
          visibleCards.delete(card);
          visibility.unobserve(card);
          cardSizing.unobserve(card);
        }
      }
    }
    function invalidateGeometry() { masksDirty = true; geometryDirty = true; }
    const particles: Particle[] = Array.from({ length: { low: 16, medium: 32, high: 64, max: 128 }[density] }, () => ({
      x: Math.random(), y: Math.random(), phase: Math.random() * Math.PI * 2,
      glyph: alphabet[Math.floor(Math.random() * alphabet.length)],
    }));
    let ambientColor = getComputedStyle(view).getPropertyValue("--color-text-muted").trim() || "#888";

    function resize() {
      invalidateGeometry();
      const rect = shell!.getBoundingClientRect();
      const parent = view!.getBoundingClientRect();
      width = rect.width;
      height = Math.max(0, rect.height);
      canvas!.style.left = `${rect.left - parent.left}px`;
      canvas!.style.top = `${rect.top - parent.top}px`;
      canvas!.style.width = `${width}px`;
      canvas!.style.height = `${height}px`;
      const ratio = Math.min(window.devicePixelRatio || 1, 2);
      const pixelWidth = Math.ceil(width * ratio), pixelHeight = Math.ceil(height * ratio);
      if (canvas!.width !== pixelWidth || canvas!.height !== pixelHeight) {
        canvas!.width = pixelWidth;
        canvas!.height = pixelHeight;
      }
      context!.setTransform(ratio, 0, 0, ratio, 0, 0);
      // Give both newly available side bands an initial letter, even at low
      // density; the remaining particles keep their usual free distribution.
      const surface = dock?.querySelector<HTMLElement>(".welcome-composer")?.getBoundingClientRect();
      if (seedComposerSides && surface && width && height) {
        const left = Math.max(rect.left, parent.left);
        const right = Math.min(rect.right, parent.right);
        const y = Math.max(0, Math.min(0.98, (surface.top + surface.height / 2 - rect.top) / height));
        if (surface.left - left >= 24) Object.assign(particles[0], { x: ((left + surface.left) / 2 - rect.left) / width, y });
        if (right - surface.right >= 24) Object.assign(particles[1], { x: ((surface.right + right) / 2 - rect.left) / width, y });
        seedComposerSides = false;
      }
    }

    function scan() {
      geometryDirty = true;
      const { messageID: domID, responseKey: id, working: active } = latest.current;
      if (!domID || !id) return;
      const response = shell!.querySelector<HTMLElement>(`.chat-response-content[data-message-id="${CSS.escape(domID)}"]`);
      if (!response) return;
      // Ignore controls, line numbers and duplicate math accessibility markup.
      const walker = response ? document.createTreeWalker(response, NodeFilter.SHOW_TEXT, {
        acceptNode: node => node.parentElement?.closest("button, .chat-code-toolbar, .chat-line-number, .katex, .chat-image-tag")
          ? NodeFilter.FILTER_REJECT : NodeFilter.FILTER_ACCEPT,
      }) : undefined;
      const nodes: Text[] = [];
      while (walker?.nextNode()) nodes.push(walker.currentNode as Text);
      const text = nodes.map(node => node.data).join("");
      const known = renderedResponses.has(id);
      const previousText = renderedResponses.get(id) ?? "";
      const animate = initialized && (active || known) && !motion.matches && !document.hidden;
      renderedResponses.delete(id);
      renderedResponses.set(id, text);
      while (renderedResponses.size > 128) renderedResponses.delete(renderedResponses.keys().next().value!);
      if (!animate) { flights = []; highlight?.clear(); return; }
      const changes = reconcileFloatingText(previousText, text);
      const additions = new Set(changes.addedOffsets);
      const offset = changes.addedOffsets[0] ?? text.length;
      // Appending a token can replace Markdown text nodes. Rebind in-flight
      // glyphs to their rendered offsets instead of making them land early.
      const rangeAt = (offset: number, length: number) => {
        let position = 0;
        for (const node of nodes) {
          if (offset >= position && offset + length <= position + node.length) {
            const range = document.createRange();
            range.setStart(node, offset - position);
            range.setEnd(node, offset - position + length);
            return range;
          }
          position += node.length;
        }
      };
      const now = performance.now();
      const glyphStyles = new Map<HTMLElement, { font: string; color: string }>();
      const glyphStyle = (element: HTMLElement) => {
        let cached = glyphStyles.get(element);
        if (!cached) {
          const style = getComputedStyle(element);
          cached = { font: style.font || `${style.fontStyle} ${style.fontWeight} ${style.fontSize} ${style.fontFamily}`, color: style.color };
          glyphStyles.set(element, cached);
        }
        return cached;
      };
      flights = flights.filter(flight => {
        if (now >= flight.start + DURATION) return false;
        if (flight.messageID !== id) return flight.range.startContainer.isConnected;
        const offset = changes.retainedOffset(flight.offset);
        if (offset === undefined) return false;
        const range = rangeAt(offset, flight.glyph.length);
        if (!range) return false;
        flight.offset = offset;
        flight.range = range;
        Object.assign(flight, glyphStyle(range.startContainer.parentElement!));
        return true;
      });
      // React replaces text nodes when a delimiter turns plain text into a
      // strong/em/code element. Rebind the registry in this layout effect,
      // before paint: waiting for the next canvas frame briefly reveals the
      // whole block and then hides it again, making its flight look restarted.
      highlight?.clear();
      for (const flight of flights) highlight?.add(flight.range);
      const bounds = canvas!.getBoundingClientRect();
      let position = 0;
      let launched = 0;
      for (const node of nodes) {
        const first = Math.max(0, offset - position);
        for (let index = first; index < node.length && flights.length < MAX_FLIGHTS;) {
          const glyph = String.fromCodePoint(node.data.codePointAt(index)!);
          const end = index + glyph.length;
          if (glyph.trim() && additions.has(position + index)) {
            const range = document.createRange();
            range.setStart(node, index);
            range.setEnd(node, end);
            const rect = range.getBoundingClientRect();
            if (rect.width && rect.bottom > bounds.top && rect.top < bounds.bottom && rect.right > bounds.left && rect.left < bounds.right) {
              const source = particles[Math.floor(Math.random() * particles.length)];
              const style = glyphStyle(node.parentElement!);
              flights.push({ messageID: id, offset: position + index, range, glyph, x: source.x * width, y: source.y * height,
                targetX: rect.left - bounds.left, targetY: rect.top - bounds.top, visible: true,
                start: now + launched++ * 9, font: style.font, color: style.color });
              source.glyph = glyph;
              highlight?.add(range);
            }
          }
          index = end;
        }
        position += node.length;
      }
    }
    scanRef.current = scan;
    resize();
    scan(); // Seed existing history without replaying it.
    initialized = true;
    watchCards(view, true);
    const observer = new MutationObserver(records => {
      invalidateGeometry();
      for (const record of records) {
        record.removedNodes.forEach(node => watchCards(node, false));
        record.addedNodes.forEach(node => watchCards(node, true));
      }
    });
    // Text changes are already delivered through React props. Watching them
    // here would scan the transcript again for every syntax-highlight update.
    observer.observe(view, { childList: true, subtree: true });
    shell.addEventListener("scroll", invalidateGeometry, { passive: true });
    const sizing = new ResizeObserver(resize);
    sizing.observe(shell);
    sizing.observe(view);
    if (dock) sizing.observe(dock);
    const transcriptSizing = new ResizeObserver(invalidateGeometry);
    const transcript = shell.querySelector(".chat-messages");
    if (transcript) transcriptSizing.observe(transcript);
    document.fonts.addEventListener("loadingdone", invalidateGeometry);

    function draw(now: number) {
      frame = 0;
      if (motion.matches || document.hidden) return;
      // Measure before painting, only after a token, scroll or layout change.
      // Reading every Range and rebuilding highlights every frame makes WebKit
      // repeatedly lay out/repaint the transcript during otherwise pure motion.
      if (geometryDirty || masksDirty) canvasBounds = canvas!.getBoundingClientRect();
      const bounds = canvasBounds;
      if (geometryDirty) {
        geometryDirty = false;
        for (const flight of flights) {
          if (!flight.range.startContainer.isConnected || flight.range.toString() !== flight.glyph) { flight.visible = false; continue; }
          const rect = flight.range.getBoundingClientRect();
          flight.targetX = rect.left - bounds.left;
          flight.targetY = rect.top - bounds.top;
          flight.visible = Boolean(rect.width && rect.bottom > bounds.top && rect.top < bounds.bottom && rect.right > bounds.left && rect.left < bounds.right);
        }
      }
      const delta = Math.min(now - (previousFrame || now), 50) / 1000;
      previousFrame = now;
      context!.clearRect(0, 0, width, height);
      context!.textBaseline = "top";
      context!.font = '15px "Geist Mono", monospace';
      context!.fillStyle = ambientColor;
      context!.globalAlpha = 0.16;
      for (const particle of particles) {
        particle.x = (particle.x + Math.cos(now / 4000 + particle.phase) * delta * 0.012 + 1) % 1;
        particle.y = (particle.y + Math.sin(now / 5000 + particle.phase) * delta * 0.016 + 1) % 1;
        context!.fillText(particle.glyph, particle.x * width, particle.y * height);
      }
      flights = flights.filter(flight => {
        const progress = Math.max(0, (now - flight.start) / DURATION);
        if (progress >= 1 || !flight.visible || !flight.range.startContainer.isConnected) {
          highlight?.delete(flight.range);
          return false;
        }
        const eased = 1 - (1 - progress) ** 3;
        const x = flight.x + (flight.targetX - flight.x) * eased;
        const y = flight.y + (flight.targetY - flight.y) * eased - Math.sin(progress * Math.PI) * 35;
        context!.font = flight.font;
        context!.fillStyle = flight.color;
        context!.globalAlpha = 0.35 + eased * 0.65;
        context!.fillText(flight.glyph, x, y);
        return true;
      });
      // Cache geometry between layout changes, and only measure visible cards.
      // Walking all historical tool cards every frame stalls long transcripts.
      if (masksDirty) {
        masksDirty = false;
        masks = [];
        for (const element of visibleCards) {
          const rect = element.getBoundingClientRect();
          if (!rect.width || !rect.height || rect.bottom <= bounds.top || rect.top >= bounds.bottom || rect.right <= bounds.left || rect.left >= bounds.right) continue;
          const style = getComputedStyle(element);
          masks.push({ x: rect.left - bounds.left, y: rect.top - bounds.top, width: rect.width, height: rect.height,
            radii: [style.borderTopLeftRadius, style.borderTopRightRadius, style.borderBottomRightRadius, style.borderBottomLeftRadius].map(value => parseFloat(value) || 0) });
        }
      }
      context!.save();
      context!.globalAlpha = 1;
      context!.globalCompositeOperation = "destination-out";
      context!.fillStyle = "#000";
      for (const mask of masks) {
        context!.beginPath();
        context!.roundRect(mask.x, mask.y, mask.width, mask.height, mask.radii);
        context!.fill();
      }
      context!.restore();
      context!.globalAlpha = 1;
      frame = requestAnimationFrame(draw);
    }
    function restart() {
      cancelAnimationFrame(frame);
      flights = [];
      highlight?.clear();
      context!.clearRect(0, 0, width, height);
      previousFrame = 0;
      ambientColor = getComputedStyle(view!).getPropertyValue("--color-text-muted").trim() || "#888";
      if (!motion.matches && !document.hidden) frame = requestAnimationFrame(draw);
    }
    motion.addEventListener("change", restart);
    document.addEventListener("visibilitychange", restart);
    restart();
    return () => {
      scanRef.current = () => {};
      cancelAnimationFrame(frame);
      observer.disconnect();
      visibility.disconnect();
      cardSizing.disconnect();
      shell.removeEventListener("scroll", invalidateGeometry);
      sizing.disconnect();
      transcriptSizing.disconnect();
      document.fonts.removeEventListener("loadingdone", invalidateGeometry);
      motion.removeEventListener("change", restart);
      document.removeEventListener("visibilitychange", restart);
      api.CSS.highlights?.delete(highlightName);
    };
  }, [density, highlightName, viewRef]);

  return <><style>{`::highlight(${highlightName}) { color: transparent; }`}</style><canvas aria-hidden="true" className="chat-floating-letters" data-density={density} ref={canvasRef} /></>;
}
