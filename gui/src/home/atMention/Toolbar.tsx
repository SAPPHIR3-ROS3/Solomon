import { useState } from "react";

type LightboxTool = "color" | "cursor" | "eraser" | "sketch" | "text";

const lightboxColors = [
  { label: "Black", value: "#080b11" },
  { label: "Red", value: "#ff4d55" },
  { label: "Yellow", value: "#ffc704" },
  { label: "Green", value: "#31d488" },
  { label: "Cyan", value: "#28c7df" },
  { label: "Purple", value: "#c66cff" },
  { label: "White", value: "#f5f7fb" },
] as const;

const sketchTools = [
  { icon: "sketch", label: "Freehand", value: "freehand" },
  { icon: "pen", label: "Pen", value: "pen" },
  { icon: "ellipse", label: "Oval", value: "ellipse" },
  { icon: "rectangle", label: "Rectangle", value: "rectangle" },
  { icon: "arrow", label: "Arrow", value: "arrow" },
] as const;

const eraserTools = [
  { icon: "eraser", label: "Normal eraser", value: "normal" },
  { icon: "object-eraser", label: "Object eraser", value: "object" },
] as const;

const selectionTools = [
  { icon: "move", label: "Move", value: "move" },
  { icon: "resize", label: "Resize", value: "resize" },
  { icon: "crop", label: "Crop", value: "crop" },
] as const;

type SketchTool = typeof sketchTools[number]["value"];
type EraserTool = typeof eraserTools[number]["value"];
type SelectionTool = typeof selectionTools[number]["value"];

export function useLightboxTools() {
  const [isColorPaletteOpen, setIsColorPaletteOpen] = useState(false);
  const [selectedTool, setSelectedTool] = useState<LightboxTool>("color");
  const [selectedSketchTool, setSelectedSketchTool] = useState<SketchTool>("freehand");
  const [selectedEraserTool, setSelectedEraserTool] = useState<EraserTool>("normal");
  const [selectedSelectionTool, setSelectedSelectionTool] = useState<SelectionTool>("move");
  const [textSize, setTextSize] = useState(16);
  const [selectedToolColor, setSelectedToolColor] = useState<string>(lightboxColors[1].value);
  return { isColorPaletteOpen, setIsColorPaletteOpen, selectedTool, setSelectedTool, selectedSketchTool, setSelectedSketchTool, selectedEraserTool, setSelectedEraserTool, selectedSelectionTool, setSelectedSelectionTool, textSize, setTextSize, selectedToolColor, setSelectedToolColor };
}

type ToolbarProps = ReturnType<typeof useLightboxTools> & { imageCount: number; selectedImageIndex: number };

export function Toolbar({ imageCount, selectedImageIndex, isColorPaletteOpen, setIsColorPaletteOpen, selectedTool, setSelectedTool, selectedSketchTool, setSelectedSketchTool, selectedEraserTool, setSelectedEraserTool, selectedSelectionTool, setSelectedSelectionTool, textSize, setTextSize, selectedToolColor, setSelectedToolColor }: ToolbarProps) {
  return (
          <div aria-label="Image editing tools" className="composer-image-lightbox-tools" onClick={(event) => event.stopPropagation()}>
            {imageCount > 1 ? <div aria-live="polite" className="composer-image-lightbox-count">{selectedImageIndex + 1} / {imageCount}</div> : null}
            <div className="composer-image-lightbox-tool-row">
              <button
                aria-expanded={isColorPaletteOpen}
                aria-label="Color"
                aria-pressed={selectedTool === "color"}
                className={`composer-image-lightbox-tool${selectedTool === "color" ? " is-selected" : ""}`}
                onClick={() => {
                  setSelectedTool("color");
                  setIsColorPaletteOpen((open) => !open);
                }}
                style={{ color: selectedToolColor }}
                title="Color"
                type="button"
              >
                <LightboxToolIcon tool="color" />
              </button>
              <button
                aria-label="Sketch"
                aria-pressed={selectedTool === "sketch"}
                className={`composer-image-lightbox-tool${selectedTool === "sketch" ? " is-selected" : ""}`}
                onClick={() => {
                  setSelectedTool("sketch");
                  setIsColorPaletteOpen(false);
                }}
                title="Sketch"
                type="button"
              >
                <LightboxToolIcon tool="sketch" />
              </button>
              <button
                aria-label="Eraser"
                aria-pressed={selectedTool === "eraser"}
                className={`composer-image-lightbox-tool${selectedTool === "eraser" ? " is-selected" : ""}`}
                onClick={() => {
                  setSelectedTool("eraser");
                  setIsColorPaletteOpen(false);
                }}
                title="Eraser"
                type="button"
              >
                <LightboxToolIcon tool="eraser" />
              </button>
              <button
                aria-label="Text"
                aria-pressed={selectedTool === "text"}
                className={`composer-image-lightbox-tool${selectedTool === "text" ? " is-selected" : ""}`}
                onClick={() => {
                  setSelectedTool("text");
                  setIsColorPaletteOpen(false);
                }}
                title="Text"
                type="button"
              >
                <LightboxToolIcon tool="text" />
              </button>
              <button
                aria-label="Select"
                aria-pressed={selectedTool === "cursor"}
                className={`composer-image-lightbox-tool${selectedTool === "cursor" ? " is-selected" : ""}`}
                onClick={() => {
                  setSelectedTool("cursor");
                  setIsColorPaletteOpen(false);
                }}
                title="Select"
                type="button"
              >
                <LightboxToolIcon tool="cursor" />
              </button>
            </div>
            <div className="composer-image-lightbox-secondary-stack">
            <div
              aria-hidden={!isColorPaletteOpen}
              aria-label="Color palette"
              className={`composer-image-lightbox-color-row${isColorPaletteOpen ? " is-open is-visible" : " is-hidden"}`}
              role="radiogroup"
            >
              {lightboxColors.map((color) => (
                <button
                  aria-checked={selectedToolColor === color.value}
                  aria-label={color.label}
                  className={`composer-image-lightbox-color-swatch${selectedToolColor === color.value ? " is-selected" : ""}`}
                  disabled={!isColorPaletteOpen}
                  key={color.value}
                  onClick={() => { setSelectedToolColor(color.value); setSelectedTool("sketch"); setIsColorPaletteOpen(false); }}
                  role="radio"
                  style={{ backgroundColor: color.value }}
                  title={color.label}
                  type="button"
                />
              ))}
            </div>
            <div aria-hidden={selectedTool !== "sketch" && selectedTool !== "eraser" && selectedTool !== "text" && selectedTool !== "cursor"} aria-label={selectedTool === "eraser" ? "Eraser tools" : selectedTool === "text" ? "Text tools" : selectedTool === "cursor" ? "Selection tools" : "Sketch tools"} className={`composer-image-lightbox-tool-row is-specific-tools${selectedTool === "sketch" || selectedTool === "eraser" || selectedTool === "text" || selectedTool === "cursor" ? " is-visible" : " is-hidden"}`}>
              {selectedTool === "sketch" ? sketchTools.map(({ icon, label, value }) => (
                <button
                  aria-label={label}
                  aria-pressed={selectedSketchTool === value}
                  className={`composer-image-lightbox-tool${selectedSketchTool === value ? " is-selected" : ""}`}
                  key={value}
                  onClick={() => setSelectedSketchTool(value)}
                  title={label}
                  type="button"
                >
                  <LightboxToolIcon tool={icon} />
                </button>
              )) : selectedTool === "eraser" ? eraserTools.map(({ icon, label, value }) => (
                <button
                  aria-label={label}
                  aria-pressed={selectedEraserTool === value}
                  className={`composer-image-lightbox-tool${selectedEraserTool === value ? " is-selected" : ""}`}
                  key={value}
                  onClick={() => setSelectedEraserTool(value)}
                  title={label}
                  type="button"
                >
                  <LightboxToolIcon tool={icon} />
                </button>
              )) : selectedTool === "text" ? (
                <>
                  <button
                    aria-label="Decrease font size"
                    className="composer-image-lightbox-tool composer-image-lightbox-text-step"
                    disabled={textSize <= 8}
                    onClick={() => setTextSize((size) => Math.max(8, size - 2))}
                    title="Decrease font size"
                    type="button"
                  >
                    −
                  </button>
                  <output aria-label={`Font size ${textSize}px`} className="composer-image-lightbox-text-size">{textSize}px</output>
                  <button
                    aria-label="Increase font size"
                    className="composer-image-lightbox-tool composer-image-lightbox-text-step"
                    disabled={textSize >= 72}
                    onClick={() => setTextSize((size) => Math.min(72, size + 2))}
                    title="Increase font size"
                    type="button"
                  >
                    +
                  </button>
                </>
              ) : selectedTool === "cursor" ? selectionTools.map(({ icon, label, value }) => (
                <button
                  aria-label={label}
                  aria-pressed={selectedSelectionTool === value}
                  className={`composer-image-lightbox-tool${selectedSelectionTool === value ? " is-selected" : ""}`}
                  key={value}
                  onClick={() => setSelectedSelectionTool(value)}
                  title={label}
                  type="button"
                >
                  <LightboxToolIcon tool={icon} />
                </button>
              )) : null}
            </div>
            </div>
          </div>
  );
}

function LightboxToolIcon({ tool }: { tool: "arrow" | "color" | "crop" | "cursor" | "ellipse" | "eraser" | "move" | "object-eraser" | "pen" | "rectangle" | "resize" | "sketch" | "text" }) {
  if (tool === "color") {
    return <svg aria-hidden="true" viewBox="0 0 24 24"><circle cx="12" cy="12" fill="currentColor" r="7" /></svg>;
  }
  if (tool === "text") {
    return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M5 5h14M12 5v14" /></svg>;
  }
  if (tool === "eraser") {
    return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="m5 15 8-8a2.8 2.8 0 0 1 4 0l1 1a2.8 2.8 0 0 1 0 4l-3 3H9m-4 0 4 4h6" /></svg>;
  }
  if (tool === "object-eraser") {
    return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="m5 15 8-8a2.8 2.8 0 0 1 4 0l1 1a2.8 2.8 0 0 1 0 4l-3 3H9m-4 0 4 4h6" /><path d="m18 4 .5 1.5L20 6l-1.5.5L18 8l-.5-1.5L16 6l1.5-.5Z" /></svg>;
  }
  if (tool === "move") {
    return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M12 3v18m-3-3 3 3 3-3M12 3 9 6m3-3 3 3M3 12h18m-3-3 3 3-3 3M3 12l3-3M3 12l3 3" /></svg>;
  }
  if (tool === "resize") {
    return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M5 9V5h4M15 5h4v4M19 15v4h-4M9 19H5v-4" /></svg>;
  }
  if (tool === "ellipse") {
    return <svg aria-hidden="true" viewBox="0 0 24 24"><ellipse cx="12" cy="12" rx="8" ry="5.5" /></svg>;
  }
  if (tool === "rectangle") {
    return <svg aria-hidden="true" viewBox="0 0 24 24"><rect height="12" rx="1" width="16" x="4" y="6" /></svg>;
  }
  if (tool === "arrow") {
    return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M5 19 19 5m0 0h-6m6 0v6" /></svg>;
  }
  if (tool === "pen") {
    return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="m6 18 3-3 8-8 2 2-8 8-3 3-3 1 1-3Z" /><path d="m14 7 3 3M6 18l3 1" /></svg>;
  }
  if (tool === "cursor") {
    return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M6 3 18 13l-6 .7 3.4 6.2-2.5 1.4L9.5 15 6 19z" /></svg>;
  }
  if (tool === "crop") {
    return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M8 4H4v4m12-4h4v4M8 20H4v-4m12 4h4v-4M7 7h10v10H7z" /></svg>;
  }
  return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="m5 19 1.4-4.6L16.7 4.1a1.8 1.8 0 0 1 2.5 0l.7.7a1.8 1.8 0 0 1 0 2.5L9.6 17.6 5 19Z" /><path d="m13.8 7.1 3.1 3.1" /></svg>;
}
