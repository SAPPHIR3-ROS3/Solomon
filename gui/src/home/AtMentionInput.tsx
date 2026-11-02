import { useTextHistory } from "./useTextHistory";
import { overlayTags, insertImageTags, removeImageTag, jumpAcrossImageTag, imageAtCursor, snapSelectionOutsideImageTag, positionImageTags, pickerPositionFor, resizeInput, type ImageTagPosition } from "./atMention/helpers";
import { Toolbar, useLightboxTools } from "./atMention/Toolbar";
import { useImageDrop } from "./atMention/useImageDrop";
import { useEffect, useId, useLayoutEffect, useRef, useState, type CSSProperties, type KeyboardEvent } from "react";
import { createPortal } from "react-dom";
import { ImageLightbox } from "./ImageLightbox";
import { clipboardImageFiles, readNativeClipboardImage } from "./clipboardImages";
import { notifyError } from "../notifications/store";
import type { ComposerImageAttachment, ComposerTerminalClip } from "../chat/composerTypes";
import { fetchProjectAtMentionSuggestions, type ProjectAtMentionSuggestion } from "../projects/projects";
import { fetchProjectSlashCommands, getCachedProjectSlashCommands } from "../projects/projects";
import { suggestionContext, insertSuggestion, type SuggestionContext } from "./composerSuggestions";
import { nativeBindings } from "../platform";

export type { ComposerImageAttachment } from "../chat/composerTypes";

type AtMentionInputProps = {
  "aria-label": string;
  className?: string;
  onChange: (value: string) => void;
  onKeyDown?: (event: KeyboardEvent<HTMLTextAreaElement>) => void;
  placeholder?: string;
  clips?: ComposerTerminalClip[];
  images?: ComposerImageAttachment[];
  onClipsChange?: (clips: ComposerTerminalClip[]) => void;
  onImagesChange?: (images: ComposerImageAttachment[]) => void;
  projectID?: string;
  value: string;
};

type InputSuggestion = ProjectAtMentionSuggestion & { description?: string };
export function AtMentionInput({ "aria-label": ariaLabel, className = "", clips = [], images = [], onChange, onClipsChange, onImagesChange, onKeyDown, placeholder, projectID, value }: AtMentionInputProps) {
  const pickerID = useId();
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const textHistory = useTextHistory({ inputRef, value, images, clips, onChange, onImagesChange, onClipsChange });
  const shellRef = useRef<HTMLDivElement>(null);
  const requestRef = useRef(0);
  const nextImageIDRef = useRef(Math.max(-1, ...images.map(image => image.id)) + 1);
  const imagePickerRef = useRef<HTMLInputElement>(null);
  const recoveredImageIDsRef = useRef(new Set<number>());
  const pasteRequestRef = useRef(0);
  const latestComposerRef = useRef({ attach: (files: File[]) => attachImages(files), value, images });
  latestComposerRef.current = { attach: (files) => attachImages(files), value, images };
  const [suggestions, setSuggestions] = useState<InputSuggestion[]>([]);
  const [selected, setSelected] = useState(0);
  const [pickerPosition, setPickerPosition] = useState<{ left: number; top: number } | null>(null);
  const [imageTagPositions, setImageTagPositions] = useState<ImageTagPosition[]>([]);
  const [selectedImageID, setSelectedImageID] = useState<number | null>(null);
  const [isDraggingImages, setIsDraggingImages] = useState(false);
  const canAttachImages = Boolean(onImagesChange);
  const lightboxTools = useLightboxTools();
  const { isColorPaletteOpen, selectedTool, selectedSketchTool, selectedEraserTool, selectedSelectionTool, textSize, selectedToolColor } = lightboxTools;
  const contextRef = useRef<SuggestionContext | null>(null);
  const selectedImage = images.find((image) => image.id === selectedImageID);
  const selectedImageIndex = images.findIndex((image) => image.id === selectedImageID);

  useImageDrop({ canAttachImages, shellRef, latestComposerRef, pasteRequestRef, setIsDraggingImages });

  useEffect(() => {
    requestRef.current += 1;
    setSuggestions([]);
    setSelected(0);
    contextRef.current = null;
    void fetchProjectSlashCommands(projectID, "").catch(() => {});
  }, [projectID]);

  useLayoutEffect(() => {
    if (!value) {
      requestRef.current += 1;
      contextRef.current = null;
      setSuggestions([]);
    }
  }, [value]);

  useEffect(() => {
    shellRef.current?.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: "nearest" });
  }, [selected]);

  useLayoutEffect(() => {
    const input = inputRef.current;
    if (images.length && input && document.activeElement !== input) input.focus();
    resizeInput(inputRef.current);
    setImageTagPositions(positionImageTags(inputRef.current, shellRef.current, value, overlayTags(images, clips)));
  }, [clips, images, value]);

  useEffect(() => {
    const reposition = () => {
      resizeInput(inputRef.current);
      setImageTagPositions(positionImageTags(inputRef.current, shellRef.current, value, overlayTags(images, clips)));
    };
    let width = inputRef.current?.clientWidth;
    let frame = 0;
    const sizing = new ResizeObserver(() => {
      const nextWidth = inputRef.current?.clientWidth;
      if (nextWidth === width) return;
      width = nextWidth;
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(reposition);
    });
    if (shellRef.current) sizing.observe(shellRef.current);
    document.fonts.addEventListener("loadingdone", reposition);
    return () => {
      sizing.disconnect();
      cancelAnimationFrame(frame);
      document.fonts.removeEventListener("loadingdone", reposition);
    };
  }, [clips, images, value]);

  useEffect(() => {
    if (selectedImageID !== null && !selectedImage) setSelectedImageID(null);
  }, [selectedImage, selectedImageID]);

  useEffect(() => {
    if (selectedImageID === null) return;
    const previousOverflow = document.body.style.overflow;
    const closeOnEscape = (event: globalThis.KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        event.stopPropagation();
        setSelectedImageID(null);
        return;
      }
      if (images.length > 1 && (event.key === "ArrowLeft" || event.key === "ArrowRight")) {
        event.preventDefault();
        event.stopPropagation();
        moveSelectedImage(event.key === "ArrowLeft" ? -1 : 1);
        return;
      }
      event.preventDefault();
      event.stopPropagation();
    };
    document.body.style.overflow = "hidden";
    if (document.activeElement instanceof HTMLElement) document.activeElement.blur();
    document.addEventListener("keydown", closeOnEscape, true);
    return () => {
      document.body.style.overflow = previousOverflow;
      document.removeEventListener("keydown", closeOnEscape, true);
    };
  }, [images, selectedImageID]);

  function updateSuggestions(nextValue: string, cursor: number) {
    const context = suggestionContext(nextValue, cursor);
    const previousContext = contextRef.current;
    contextRef.current = context;
    const request = ++requestRef.current;
    if (!context || (context.kind === "file" && !projectID)) {
      setSuggestions([]);
      setSelected(0);
      setPickerPosition(null);
      return;
    }
    setPickerPosition(pickerPositionFor(inputRef.current, shellRef.current, context.start));
    if (previousContext?.kind !== context.kind || previousContext.query !== context.query) {
      setSuggestions([]);
      setSelected(0);
    }
    if (context.kind === "command") {
      const cached = getCachedProjectSlashCommands(projectID, context.query);
      if (cached) setSuggestions(cached.map((command) => ({ ...command, path: command.tag, isDirectory: false })));
    }
    const loadSuggestions = context.kind === "command"
      ? fetchProjectSlashCommands(projectID, context.query).then((commands) => commands.map((command) => ({ ...command, path: command.tag, isDirectory: false })))
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

  function selectSuggestion(suggestion: InputSuggestion) {
    const input = inputRef.current;
    const context = contextRef.current;
    if (!input || !context) return;
    const insertion = insertSuggestion(value, context, suggestion.tag);
    const nextValue = insertion.value;
    const nextCursor = insertion.cursor;
    onChange(nextValue);
    contextRef.current = null;
    setSuggestions([]);
    setPickerPosition(null);
    requestRef.current += 1;
    requestAnimationFrame(() => {
      input.focus();
      input.setSelectionRange(nextCursor, nextCursor);
    });
  }

  function pasteImages(event: React.ClipboardEvent<HTMLTextAreaElement>) {
    if (!onImagesChange) return;
    const files = clipboardImageFiles(event.clipboardData);
    if (files.length) {
      pasteRequestRef.current += 1;
      event.preventDefault();
      attachImages(files);
    } else if (event.clipboardData.getData("text/plain")) {
      pasteRequestRef.current += 1;
    } else {
      pasteNativeImage();
    }
  }

  function pasteNativeImage() {
    if (!onImagesChange || !nativeBindings()?.ServerBridge?.ClipboardImage) return;
    const request = ++pasteRequestRef.current;
    const composer = latestComposerRef.current;
    void readNativeClipboardImage().then((files) => {
      if (request !== pasteRequestRef.current || !inputRef.current ||
          composer.value !== latestComposerRef.current.value || composer.images !== latestComposerRef.current.images) return;
      latestComposerRef.current.attach(files);
    }).catch((error: unknown) => {
      if (request === pasteRequestRef.current) notifyError(`Unable to paste image: ${String(error)}`);
    });
  }

  function attachImages(files: File[]) {
    if (!files.length || !onImagesChange) return;
    const pasted = files.filter((file) => file.type.startsWith("image/")).map((file) => {
      const id = nextImageIDRef.current++;
      return { blob: file, id, name: file.name || `image-${id}.png`, tag: `[img-${id}]`, url: URL.createObjectURL(file) };
    });
    if (!pasted.length) return;
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
    });
  }

  function removeImage(image: ComposerImageAttachment) {
    const removal = removeImageTag(value, image.tag);
    onChange(removal.value);
    onImagesChange?.(images.filter((candidate) => candidate.id !== image.id));
    requestAnimationFrame(() => {
      inputRef.current?.focus();
      inputRef.current?.setSelectionRange(removal.cursor, removal.cursor);
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

  return (
    <div className={`at-mention-input-shell${isDraggingImages ? " is-image-drop-target" : ""}`} ref={shellRef}>
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
      {onImagesChange ? <>
        <input accept="image/*" aria-label="Choose images" hidden multiple onChange={(event) => {
          attachImages(Array.from(event.currentTarget.files ?? []));
          event.currentTarget.value = "";
        }} ref={imagePickerRef} type="file" />
        <button aria-label="Attach images" className="composer-image-upload" onClick={() => imagePickerRef.current?.click()} type="button">
          <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M12 5v14M5 12h14" /></svg>
        </button>
      </> : null}
      <textarea
        data-text-history="local"
        aria-label={ariaLabel}
        aria-autocomplete="list"
        aria-controls={suggestions.length ? pickerID : undefined}
        aria-activedescendant={suggestions.length ? `${pickerID}-${selected}` : undefined}
        className={`${className} at-mention-input`.trim()}
        onChange={(event) => {
          const nextValue = event.target.value;
          textHistory.record(nextValue, event.currentTarget, (event.nativeEvent as InputEvent).inputType ?? "");
          onChange(nextValue);
          const remainingImages = images.filter((image) => nextValue.includes(image.tag));
          if (remainingImages.length !== images.length) onImagesChange?.(remainingImages);
          const remainingClips = clips.filter((clip) => nextValue.includes(clip.tag));
          if (remainingClips.length !== clips.length) onClipsChange?.(remainingClips);
          updateSuggestions(nextValue, event.target.selectionStart ?? nextValue.length);
        }}
        onClick={(event) => {
          updateSuggestions(value, event.currentTarget.selectionStart ?? value.length);
        }}
        onFocus={(event) => {
          updateSuggestions(value, event.currentTarget.selectionStart ?? value.length);
        }}
        onKeyDown={(event) => {
          if (event.nativeEvent.isComposing || event.nativeEvent.keyCode === 229) return;
          if (textHistory.keyDown(event)) { requestRef.current += 1; setSuggestions([]); contextRef.current = null; return; }
          if ((event.ctrlKey || event.metaKey) && !event.altKey && !event.shiftKey && event.key.toLowerCase() === "v") pasteNativeImage();
          if (suggestions.length) {
            if (event.key === "ArrowDown") { event.preventDefault(); setSelected((current) => Math.min(current + 1, suggestions.length - 1)); return; }
            if (event.key === "ArrowUp") { event.preventDefault(); setSelected((current) => Math.max(current - 1, 0)); return; }
            if (event.key === "Tab" || (event.key === "Enter" && !event.shiftKey)) {
              event.preventDefault();
              selectSuggestion(suggestions[selected]);
              return;
            }
            if (event.key === "Escape") { event.preventDefault(); requestRef.current += 1; setSuggestions([]); contextRef.current = null; return; }
          }
          const input = event.currentTarget;
          if (input.selectionStart === input.selectionEnd) {
            const cursor = input.selectionStart;
            if (event.key === "ArrowLeft") {
              const nextCursor = jumpAcrossImageTag(value, overlayTags(images, clips), cursor, "left");
              if (nextCursor !== null) { event.preventDefault(); input.setSelectionRange(nextCursor, nextCursor); return; }
            }
            if (event.key === "ArrowRight") {
              const nextCursor = jumpAcrossImageTag(value, overlayTags(images, clips), cursor, "right");
              if (nextCursor !== null) { event.preventDefault(); input.setSelectionRange(nextCursor, nextCursor); return; }
            }
            if (event.key === "Backspace") {
              const image = imageAtCursor(value, overlayTags(images, clips), cursor, "left");
              if (image) { removeImageFromKeyboard(image, event); return; }
            }
            if (event.key === "Delete") {
              const image = imageAtCursor(value, overlayTags(images, clips), cursor, "right");
              if (image) { removeImageFromKeyboard(image, event); return; }
            }
          }
          onKeyDown?.(event);
        }}
        onPaste={pasteImages}
        onKeyUp={(event) => {
          if (["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) updateSuggestions(value, event.currentTarget.selectionStart);
        }}
        onScroll={() => {
          setImageTagPositions(positionImageTags(inputRef.current, shellRef.current, value, overlayTags(images, clips)));
        }}
        onSelect={(event) => {
          snapSelectionOutsideImageTag(event.currentTarget, value, overlayTags(images, clips));
        }}
        onBlur={() => { requestRef.current += 1; setSuggestions([]); contextRef.current = null; }}
        placeholder={placeholder}
        ref={inputRef}
        rows={3}
        value={value}
      />
      <div aria-hidden="true" className="composer-image-tag-layer">
        {imageTagPositions.map((position) => {
          const image = images.find((candidate) => candidate.id === position.id);
          return image ? (
            <span className={image.tag.startsWith("[terminal-") ? "composer-image-tag composer-terminal-tag" : "composer-image-tag"} key={image.id} style={{ left: position.left, top: position.top, width: position.width }}>
              {position.text}
            </span>
          ) : null;
        })}
      </div>
      {suggestions.length ? (
        <div id={pickerID} aria-label={contextRef.current?.kind === "command" ? "Command suggestions" : "File suggestions"} className="at-mention-picker" role="listbox" style={pickerPosition ? { left: pickerPosition.left, bottom: pickerPosition.top } satisfies CSSProperties : undefined}>
          {suggestions.map((suggestion, index) => (
            <button
              aria-selected={index === selected}
              id={`${pickerID}-${index}`}
              tabIndex={-1}
              className={`at-mention-picker-item${index === selected ? " is-selected" : ""}`}
              key={suggestion.path}
              onMouseDown={(event) => { event.preventDefault(); selectSuggestion(suggestion); }}
              role="option"
              type="button"
            >
              <span>{suggestion.description ? suggestion.tag : `@${suggestion.path.split("/").at(-1)}`}</span><small>{suggestion.description ?? (suggestion.isDirectory ? `${suggestion.path}/` : suggestion.path)}</small>
            </button>
          ))}
        </div>
      ) : null}
      {selectedImage ? createPortal(
        <ImageLightbox key={selectedImage.id}
          color={selectedToolColor} eraserTool={selectedEraserTool}
          image={selectedImage} images={images} isColorPaletteOpen={isColorPaletteOpen}
          onClose={() => setSelectedImageID(null)} onImagesChange={onImagesChange}
          onRefresh={refreshImageSource} onSelectIndex={moveSelectedImage}
          selectedIndex={selectedImageIndex} selectionTool={selectedSelectionTool}
          sketchTool={selectedSketchTool} textSize={textSize} tool={selectedTool}
        >
          <Toolbar {...lightboxTools} imageCount={images.length} selectedImageIndex={selectedImageIndex} />
        </ImageLightbox>,
        document.body,
      ) : null}
    </div>
  );
}


function RemoveImageIcon() {
  return <svg aria-hidden="true" viewBox="0 0 16 16"><path d="m4 4 8 8M12 4l-8 8" /></svg>;
}
