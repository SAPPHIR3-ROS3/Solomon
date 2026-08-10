import { useEffect, useLayoutEffect, useRef, useState, type CSSProperties, type KeyboardEvent } from "react";
import { createPortal } from "react-dom";
import { fetchAtMentionSuggestions, fetchProjectAtMentionSuggestions, type ProjectAtMentionEntry, type ProjectAtMentionSuggestion } from "../projects/projects";

type AtMentionInputProps = {
  "aria-label": string;
  className?: string;
  onChange: (value: string) => void;
  onKeyDown?: (event: KeyboardEvent<HTMLTextAreaElement>) => void;
  placeholder?: string;
  entries?: ProjectAtMentionEntry[];
  images?: ComposerImageAttachment[];
  onImagesChange?: (images: ComposerImageAttachment[]) => void;
  projectID?: string;
  value: string;
};

export type ComposerImageAttachment = {
  blob?: Blob;
  id: number;
  name: string;
  tag: string;
  url: string;
};

type MentionContext = { start: number; query: string };
type ImageTagPosition = { id: number; left: number; top: number; text: string; width: number };
type CaretPosition = { height: number; left: number; top: number };
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

// The index, matching and shortest unambiguous tag are owned by the Go
// atmention package. This component only supplies the GUI equivalent of the
// terminal picker and its coloured rendering.
export function AtMentionInput({ "aria-label": ariaLabel, className = "", entries, images = [], onChange, onImagesChange, onKeyDown, placeholder, projectID, value }: AtMentionInputProps) {
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const shellRef = useRef<HTMLDivElement>(null);
  const requestRef = useRef(0);
  const nextImageIDRef = useRef(0);
  const previousImagesRef = useRef<ComposerImageAttachment[]>([]);
  const recoveredImageIDsRef = useRef(new Set<number>());
  const [suggestions, setSuggestions] = useState<ProjectAtMentionSuggestion[]>([]);
  const [selected, setSelected] = useState(0);
  const [pickerPosition, setPickerPosition] = useState<{ left: number; top: number } | null>(null);
  const [imageTagPositions, setImageTagPositions] = useState<ImageTagPosition[]>([]);
  const [caretPosition, setCaretPosition] = useState<CaretPosition | null>(null);
  const [selectedImageID, setSelectedImageID] = useState<number | null>(null);
  const [isColorPaletteOpen, setIsColorPaletteOpen] = useState(false);
  const [selectedTool, setSelectedTool] = useState<LightboxTool>("color");
  const [selectedSketchTool, setSelectedSketchTool] = useState<SketchTool>("freehand");
  const [selectedEraserTool, setSelectedEraserTool] = useState<EraserTool>("normal");
  const [selectedSelectionTool, setSelectedSelectionTool] = useState<SelectionTool>("move");
  const [textSize, setTextSize] = useState(16);
  const [selectedToolColor, setSelectedToolColor] = useState<string>(lightboxColors[1].value);
  const contextRef = useRef<MentionContext | null>(null);
  const selectedImage = images.find((image) => image.id === selectedImageID);
  const selectedImageIndex = images.findIndex((image) => image.id === selectedImageID);

  useEffect(() => {
    setSuggestions([]);
    setSelected(0);
    contextRef.current = null;
  }, [entries, projectID]);

  useLayoutEffect(() => {
    const input = inputRef.current;
    if (images.length && input && document.activeElement !== input) input.focus();
    resizeInput(inputRef.current);
    setImageTagPositions(positionImageTags(inputRef.current, shellRef.current, value, images));
    if (document.activeElement === inputRef.current) syncVisualCaret(inputRef.current);
  }, [images, value]);

  useEffect(() => {
    const reposition = () => {
      setImageTagPositions(positionImageTags(inputRef.current, shellRef.current, value, images));
      if (document.activeElement === inputRef.current) syncVisualCaret(inputRef.current);
    };
    window.addEventListener("resize", reposition);
    return () => window.removeEventListener("resize", reposition);
  }, [images, value]);

  useEffect(() => {
    for (const previous of previousImagesRef.current) {
      if (!images.some((image) => image.url === previous.url)) URL.revokeObjectURL(previous.url);
    }
    previousImagesRef.current = images;
  }, [images]);

  useEffect(() => {
    if (selectedImageID !== null && !selectedImage) setSelectedImageID(null);
  }, [selectedImage, selectedImageID]);

  useEffect(() => {
    if (selectedImageID === null) return;
    const previousOverflow = document.body.style.overflow;
    const closeOnEscape = (event: globalThis.KeyboardEvent) => {
      if (event.key === "Escape") setSelectedImageID(null);
      if (images.length < 2) return;
      if (event.key === "ArrowLeft" || event.key === "ArrowRight") {
        event.preventDefault();
        moveSelectedImage(event.key === "ArrowLeft" ? -1 : 1);
      }
    };
    document.body.style.overflow = "hidden";
    document.addEventListener("keydown", closeOnEscape);
    return () => {
      document.body.style.overflow = previousOverflow;
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, [images, selectedImageID]);

  function updateSuggestions(nextValue: string, cursor: number) {
    const context = atMentionContext(nextValue, cursor);
    contextRef.current = context;
    const request = ++requestRef.current;
    if (!context || (!projectID && !entries)) {
      setSuggestions([]);
      setSelected(0);
      setPickerPosition(null);
      return;
    }
    setPickerPosition(pickerPositionFor(inputRef.current, shellRef.current, context.start));
    const loadSuggestions = entries
      ? fetchAtMentionSuggestions(entries, context.query)
      : fetchProjectAtMentionSuggestions(projectID!, context.query);
    void loadSuggestions
      .then((next) => {
        if (request !== requestRef.current) return;
        setSuggestions(next);
        setSelected(0);
      })
      .catch(() => {
        if (request === requestRef.current) setSuggestions([]);
      });
  }

  function selectSuggestion(suggestion: ProjectAtMentionSuggestion) {
    const input = inputRef.current;
    const context = contextRef.current;
    if (!input || !context) return;
    const cursor = input.selectionStart ?? value.length;
    const nextValue = `${value.slice(0, context.start)}${suggestion.tag} ${value.slice(cursor)}`;
    const nextCursor = context.start + suggestion.tag.length + 1;
    onChange(nextValue);
    contextRef.current = null;
    setSuggestions([]);
    setPickerPosition(null);
    requestRef.current += 1;
    requestAnimationFrame(() => {
      input.focus();
      input.setSelectionRange(nextCursor, nextCursor);
      scheduleVisualCaret(input);
    });
  }

  function pasteImages(event: React.ClipboardEvent<HTMLTextAreaElement>) {
    const files = [...event.clipboardData.items]
      .filter((item) => item.kind === "file" && item.type.startsWith("image/"))
      .flatMap((item) => item.getAsFile() ?? []);
    if (!files.length || !onImagesChange) return;
    event.preventDefault();
    const pasted = files.map((file) => {
      const id = nextImageIDRef.current++;
      return { blob: file, id, name: file.name || `image-${id}.png`, tag: `[img-${id}]`, url: URL.createObjectURL(file) };
    });
    const input = inputRef.current;
    const start = input?.selectionStart ?? value.length;
    const end = input?.selectionEnd ?? start;
    const insertion = insertImageTags(value, start, end, pasted.map((image) => image.tag));
    onChange(insertion.value);
    onImagesChange([...images, ...pasted]);
    input?.focus();
    input?.setSelectionRange(insertion.cursor, insertion.cursor);
    requestAnimationFrame(() => {
      input?.focus();
      input?.setSelectionRange(insertion.cursor, insertion.cursor);
      scheduleVisualCaret(input);
    });
  }

  function removeImage(image: ComposerImageAttachment) {
    const removal = removeImageTag(value, image.tag);
    onChange(removal.value);
    onImagesChange?.(images.filter((candidate) => candidate.id !== image.id));
    requestAnimationFrame(() => {
      inputRef.current?.focus();
      inputRef.current?.setSelectionRange(removal.cursor, removal.cursor);
      scheduleVisualCaret(inputRef.current);
    });
  }

  function refreshImageSource(image: ComposerImageAttachment) {
    if (!image.blob || recoveredImageIDsRef.current.has(image.id)) return;
    recoveredImageIDsRef.current.add(image.id);
    const url = URL.createObjectURL(image.blob);
    onImagesChange?.(images.map((candidate) => candidate.id === image.id ? { ...candidate, url } : candidate));
  }

  function moveSelectedImage(offset: number) {
    if (images.length < 2 || selectedImageIndex < 0) return;
    const nextIndex = (selectedImageIndex + offset + images.length) % images.length;
    setSelectedImageID(images[nextIndex].id);
  }

  function removeImageFromKeyboard(image: ComposerImageAttachment, event: KeyboardEvent<HTMLTextAreaElement>) {
    event.preventDefault();
    removeImage(image);
  }

  function syncVisualCaret(input: HTMLTextAreaElement | null) {
    if (!input || document.activeElement !== input || input.selectionStart !== input.selectionEnd) {
      setCaretPosition(null);
      return;
    }
    setCaretPosition(positionCaret(input, shellRef.current, input.value, input.selectionStart));
  }

  function scheduleVisualCaret(input: HTMLTextAreaElement | null) {
    requestAnimationFrame(() => requestAnimationFrame(() => syncVisualCaret(input)));
  }

  return (
    <div className="at-mention-input-shell" ref={shellRef}>
      {images.length ? (
        <div aria-label="Pasted images" className="composer-image-previews">
          {images.map((image) => (
            <figure
              className="composer-image-preview"
              key={image.id}
              onClick={() => setSelectedImageID(image.id)}
              onKeyDown={(event) => {
                if (event.key === "Enter" || event.key === " ") {
                  event.preventDefault();
                  setSelectedImageID(image.id);
                }
              }}
              role="button"
              tabIndex={0}
            >
              <img alt={`Open preview of ${image.name}`} onError={() => refreshImageSource(image)} src={image.url} />
              <button
                aria-label={`Remove ${image.name}`}
                className="composer-image-remove"
                onMouseDown={(event) => {
                  event.preventDefault();
                  removeImage(image);
                }}
                onClick={(event) => event.stopPropagation()}
                title="Remove image"
                type="button"
              >
                <RemoveImageIcon />
              </button>
            </figure>
          ))}
        </div>
      ) : null}
      <textarea
        aria-label={ariaLabel}
        className={`${className} at-mention-input`.trim()}
        onChange={(event) => {
          const nextValue = event.target.value;
          onChange(nextValue);
          const remainingImages = images.filter((image) => nextValue.includes(image.tag));
          if (remainingImages.length !== images.length) onImagesChange?.(remainingImages);
          updateSuggestions(nextValue, event.target.selectionStart ?? nextValue.length);
          scheduleVisualCaret(inputRef.current);
        }}
        onClick={(event) => {
          updateSuggestions(value, event.currentTarget.selectionStart ?? value.length);
          syncVisualCaret(event.currentTarget);
          scheduleVisualCaret(event.currentTarget);
        }}
        onFocus={(event) => {
          syncVisualCaret(event.currentTarget);
          scheduleVisualCaret(event.currentTarget);
        }}
        onMouseUp={(event) => {
          syncVisualCaret(event.currentTarget);
          scheduleVisualCaret(event.currentTarget);
        }}
        onKeyDown={(event) => {
          if (suggestions.length) {
            if (event.key === "ArrowDown") { event.preventDefault(); setSelected((current) => Math.min(current + 1, suggestions.length - 1)); return; }
            if (event.key === "ArrowUp") { event.preventDefault(); setSelected((current) => Math.max(current - 1, 0)); return; }
            if (event.key === "Tab" || event.key === "Enter") {
              event.preventDefault();
              selectSuggestion(suggestions[selected]);
              return;
            }
            if (event.key === "Escape") { event.preventDefault(); setSuggestions([]); return; }
          }
          const input = event.currentTarget;
          if (input.selectionStart === input.selectionEnd) {
            const cursor = input.selectionStart;
            if (event.key === "ArrowLeft") {
              const nextCursor = jumpAcrossImageTag(value, images, cursor, "left");
              if (nextCursor !== null) { event.preventDefault(); input.setSelectionRange(nextCursor, nextCursor); syncVisualCaret(input); return; }
            }
            if (event.key === "ArrowRight") {
              const nextCursor = jumpAcrossImageTag(value, images, cursor, "right");
              if (nextCursor !== null) { event.preventDefault(); input.setSelectionRange(nextCursor, nextCursor); syncVisualCaret(input); return; }
            }
            if (event.key === "Backspace") {
              const image = imageAtCursor(value, images, cursor, "left");
              if (image) { removeImageFromKeyboard(image, event); return; }
            }
            if (event.key === "Delete") {
              const image = imageAtCursor(value, images, cursor, "right");
              if (image) { removeImageFromKeyboard(image, event); return; }
            }
          }
          onKeyDown?.(event);
        }}
        onPaste={pasteImages}
        onScroll={() => {
          setImageTagPositions(positionImageTags(inputRef.current, shellRef.current, value, images));
          syncVisualCaret(inputRef.current);
        }}
        onSelect={(event) => {
          snapSelectionOutsideImageTag(event.currentTarget, value, images);
          scheduleVisualCaret(event.currentTarget);
        }}
        onBlur={() => setCaretPosition(null)}
        placeholder={placeholder}
        ref={inputRef}
        rows={3}
        value={value}
      />
      <div aria-hidden="true" className="composer-image-tag-layer">
        {imageTagPositions.map((position) => {
          const image = images.find((candidate) => candidate.id === position.id);
          return image ? (
            <span className="composer-image-tag" key={image.id} style={{ left: position.left, top: position.top, width: position.width }}>
              {position.text}
            </span>
          ) : null;
        })}
      </div>
      {caretPosition ? (
        <span aria-hidden="true" className="composer-image-caret" style={caretPosition} />
      ) : null}
      {suggestions.length ? (
        <div aria-label="File suggestions" className="at-mention-picker" role="listbox" style={pickerPosition ? { left: pickerPosition.left, top: pickerPosition.top } satisfies CSSProperties : undefined}>
          {suggestions.map((suggestion, index) => (
            <button
              aria-selected={index === selected}
              className={`at-mention-picker-item${index === selected ? " is-selected" : ""}`}
              key={suggestion.path}
              onMouseDown={(event) => { event.preventDefault(); selectSuggestion(suggestion); }}
              role="option"
              type="button"
            >
              <span>{`@${suggestion.path.split("/").at(-1)}`}</span><small>{suggestion.isDirectory ? `${suggestion.path}/` : suggestion.path}</small>
            </button>
          ))}
        </div>
      ) : null}
      {selectedImage ? createPortal(
        <div
          aria-label={`Image preview: ${selectedImage.name}`}
          aria-modal="true"
          className={`composer-image-lightbox${isColorPaletteOpen ? " is-color-open" : ""}`}
          onClick={() => setSelectedImageID(null)}
          role="dialog"
        >
          <div aria-label="Image editing history" className="composer-image-lightbox-actions" onClick={(event) => event.stopPropagation()}>
            <button aria-label="Undo" className="composer-image-lightbox-action" disabled title="Undo" type="button">
              <LightboxHistoryIcon action="undo" />
            </button>
            <button aria-label="Redo" className="composer-image-lightbox-action" disabled title="Redo" type="button">
              <LightboxHistoryIcon action="redo" />
            </button>
            <button aria-label="Reset" className="composer-image-lightbox-action" disabled title="Reset" type="button">
              <LightboxHistoryIcon action="reset" />
            </button>
          </div>
          {images.length > 1 ? (
            <button
              aria-label="Previous image"
              className="composer-image-lightbox-nav is-previous"
              onClick={(event) => { event.stopPropagation(); moveSelectedImage(-1); }}
              type="button"
            >
              <LightboxArrowIcon direction="left" />
            </button>
          ) : null}
          <div className="composer-image-lightbox-stage" onClick={(event) => event.stopPropagation()}>
            <img alt={selectedImage.name} className="composer-image-lightbox-image" onError={() => refreshImageSource(selectedImage)} src={selectedImage.url} />
          </div>
          {images.length > 1 ? (
            <button
              aria-label="Next image"
              className="composer-image-lightbox-nav is-next"
              onClick={(event) => { event.stopPropagation(); moveSelectedImage(1); }}
              type="button"
            >
              <LightboxArrowIcon direction="right" />
            </button>
          ) : null}
          <div aria-label="Image editing tools" className="composer-image-lightbox-tools" onClick={(event) => event.stopPropagation()}>
            {images.length > 1 ? <div aria-live="polite" className="composer-image-lightbox-count">{selectedImageIndex + 1} / {images.length}</div> : null}
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
                  onClick={() => setSelectedToolColor(color.value)}
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
        </div>,
        document.body,
      ) : null}
    </div>
  );
}

function insertImageTags(value: string, start: number, end: number, tags: string[]) {
  const before = value.slice(0, start);
  const after = value.slice(end);
  const prefix = before && /\s$/.test(before) ? "" : " ";
  const suffix = /^\s/.test(after) ? "" : " ";
  const inserted = `${prefix}${tags.join(" ")}${suffix}`;
  return { cursor: start + inserted.length, value: `${before}${inserted}${after}` };
}

function removeImageTag(value: string, tag: string) {
  const bounds = imageTagBounds(value, [{ id: -1, name: "", tag, url: "" }])[0];
  if (!bounds) return { cursor: value.length, value };
  const { start, end } = bounds;
  const nextValue = `${value.slice(0, start)}${value.slice(end)}`;
  return { cursor: start, value: nextValue.trim() ? nextValue : "" };
}

function imageTagBounds(value: string, images: ComposerImageAttachment[]) {
  return images.flatMap((image) => {
    const start = value.indexOf(image.tag);
    if (start < 0) return [];
    const tagEnd = start + image.tag.length;
    return [{
      end: value[tagEnd] === " " ? tagEnd + 1 : tagEnd,
      image,
      start: value[start - 1] === " " ? start - 1 : start,
    }];
  });
}

function jumpAcrossImageTag(value: string, images: ComposerImageAttachment[], cursor: number, direction: "left" | "right") {
  for (const bounds of imageTagBounds(value, images)) {
    if (direction === "left" && cursor > bounds.start && cursor <= bounds.end) return bounds.start;
    if (direction === "right" && cursor >= bounds.start && cursor < bounds.end) return bounds.end;
  }
  return null;
}

function imageAtCursor(value: string, images: ComposerImageAttachment[], cursor: number, direction: "left" | "right") {
  return imageTagBounds(value, images).find((bounds) => (
    direction === "left"
      ? cursor > bounds.start && cursor <= bounds.end
      : cursor >= bounds.start && cursor < bounds.end
  ))?.image;
}

function snapSelectionOutsideImageTag(input: HTMLTextAreaElement, value: string, images: ComposerImageAttachment[]) {
  if (input.selectionStart !== input.selectionEnd) return;
  const cursor = input.selectionStart;
  for (const bounds of imageTagBounds(value, images)) {
    if (cursor <= bounds.start || cursor >= bounds.end) continue;
    const nextCursor = cursor - bounds.start < bounds.end - cursor ? bounds.start : bounds.end;
    input.setSelectionRange(nextCursor, nextCursor);
    return;
  }
}

function positionImageTags(input: HTMLTextAreaElement | null, shell: HTMLDivElement | null, value: string, images: ComposerImageAttachment[]): ImageTagPosition[] {
  if (!input || !shell || !images.length) return [];
  const computed = getComputedStyle(input);
  const mirror = document.createElement("div");
  copyTextareaStyles(computed, mirror);
  const inputRect = input.getBoundingClientRect();
  mirror.style.cssText += `;height:auto;left:${inputRect.left}px;overflow:hidden;position:fixed;top:${inputRect.top}px;visibility:hidden;width:${input.clientWidth}px;`;
  const bounds = imageTagBounds(value, images).sort((left, right) => left.start - right.start);
  const markers: Array<{ id: number; label: HTMLSpanElement; marker: HTMLSpanElement }> = [];
  let cursor = 0;
  for (const bound of bounds) {
    mirror.append(document.createTextNode(value.slice(cursor, bound.start)));
    const marker = document.createElement("span");
    marker.style.whiteSpace = "nowrap";
    if (value[bound.start] === " ") marker.append(document.createTextNode("\u00a0"));
    const label = document.createElement("span");
    label.textContent = bound.image.tag;
    marker.append(label);
    if (value[bound.end - 1] === " ") marker.append(document.createTextNode("\u00a0"));
    mirror.append(marker);
    markers.push({ id: bound.image.id, label, marker });
    cursor = Math.max(cursor, bound.end);
  }
  mirror.append(document.createTextNode(value.slice(cursor) || "\u200b"));
  document.body.append(mirror);
  const shellRect = shell.getBoundingClientRect();
  const positions = markers.map(({ id, label }) => {
    const rect = label.getBoundingClientRect();
    return {
      id,
      left: rect.left - shellRect.left - input.scrollLeft,
      top: rect.top - shellRect.top - input.scrollTop,
      text: label.textContent ?? "",
      width: rect.width,
    };
  });
  mirror.remove();
  return positions;
}

function positionCaret(input: HTMLTextAreaElement | null, shell: HTMLDivElement | null, value: string, position: number): CaretPosition | null {
  if (!input || !shell) return null;
  const computed = getComputedStyle(input);
  const mirror = document.createElement("div");
  copyTextareaStyles(computed, mirror);
  const inputRect = input.getBoundingClientRect();
  mirror.style.cssText += `;height:auto;left:${inputRect.left}px;overflow:hidden;position:fixed;top:${inputRect.top}px;visibility:hidden;width:${input.clientWidth}px;`;
  mirror.textContent = value.slice(0, position);
  const marker = document.createElement("span");
  marker.textContent = "\u200b";
  mirror.append(marker);
  document.body.append(mirror);
  const markerRect = marker.getBoundingClientRect();
  const shellRect = shell.getBoundingClientRect();
  const lineHeight = Number.parseFloat(computed.lineHeight) || 22;
  mirror.remove();
  return {
    height: lineHeight,
    left: markerRect.left - shellRect.left - input.scrollLeft,
    top: markerRect.top - shellRect.top - input.scrollTop,
  };
}

function copyTextareaStyles(computed: CSSStyleDeclaration, target: HTMLElement) {
  const copiedProperties = ["border", "boxSizing", "fontFamily", "fontSize", "fontWeight", "letterSpacing", "lineHeight", "padding", "tabSize", "textTransform", "whiteSpace", "wordBreak", "wordSpacing", "wordWrap"] as const;
  for (const property of copiedProperties) target.style[property] = computed[property];
}

function RemoveImageIcon() {
  return <svg aria-hidden="true" viewBox="0 0 16 16"><path d="m4 4 8 8M12 4l-8 8" /></svg>;
}

function LightboxArrowIcon({ direction }: { direction: "left" | "right" }) {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d={direction === "left" ? "m14.5 5-7 7 7 7" : "m9.5 5 7 7-7 7"} />
    </svg>
  );
}

function LightboxHistoryIcon({ action }: { action: "redo" | "reset" | "undo" }) {
  const path = action === "undo"
    ? "M9 7H4m0 0 3-3M4 7c4.8-4.2 12-.8 12 4.5 0 2.2-1.8 4-4 4H9"
    : action === "redo"
      ? "M15 7h5m0 0-3-3m3 3c-4.8-4.2-12-.8-12 4.5 0 2.2 1.8 4 4 4h3"
      : "M19 8a7 7 0 1 0 1 4m-1-8v4h-4";
  return <svg aria-hidden="true" viewBox="0 0 24 24"><path d={path} /></svg>;
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

function atMentionContext(value: string, cursor: number): MentionContext | null {
  const beforeCursor = value.slice(0, cursor);
  const start = beforeCursor.lastIndexOf("@");
  if (start < 0) return null;
  const query = beforeCursor.slice(start + 1);
  return /\s|@/.test(query) ? null : { query, start };
}

function pickerPositionFor(input: HTMLTextAreaElement | null, shell: HTMLDivElement | null, position: number) {
  if (!input || !shell) return null;
  const computed = getComputedStyle(input);
  const mirror = document.createElement("div");
  copyTextareaStyles(computed, mirror);
  mirror.style.cssText += `;height:auto;left:${input.getBoundingClientRect().left}px;overflow:hidden;position:fixed;top:${input.getBoundingClientRect().top}px;visibility:hidden;width:${input.clientWidth}px;`;
  mirror.textContent = input.value.slice(0, position);
  const marker = document.createElement("span");
  marker.textContent = "\u200b";
  mirror.append(marker);
  document.body.append(mirror);
  const markerRect = marker.getBoundingClientRect();
  const shellRect = shell.getBoundingClientRect();
  mirror.remove();
  return {
    left: Math.max(0, markerRect.left - shellRect.left - input.scrollLeft),
    top: markerRect.bottom - shellRect.top - input.scrollTop + 6,
  };
}

function resizeInput(input: HTMLTextAreaElement | null) {
  if (!input) return;
  const computed = getComputedStyle(input);
  const lineHeight = Number.parseFloat(computed.lineHeight) || 22;
  const verticalPadding = (Number.parseFloat(computed.paddingTop) || 0) + (Number.parseFloat(computed.paddingBottom) || 0);
  const maxHeight = lineHeight * 10 + verticalPadding;
  input.style.height = "auto";
  const nextHeight = Math.min(maxHeight, Math.max(72, input.scrollHeight));
  input.style.height = `${nextHeight}px`;
  input.style.overflowY = input.scrollHeight > maxHeight ? "auto" : "hidden";
}
