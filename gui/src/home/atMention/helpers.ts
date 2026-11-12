import type { ComposerImageAttachment, ComposerTerminalClip } from "../../chat/composerTypes";

export type ImageTagPosition = { id: number; left: number; top: number; text: string; width: number };

export function overlayTags(images: ComposerImageAttachment[], clips: ComposerTerminalClip[]): ComposerImageAttachment[] {
  return [
    ...images,
    ...clips.map((clip, index) => ({ id: -1 - index, name: clip.tag, tag: clip.tag, url: "" })),
  ];
}

export function insertImageTags(value: string, start: number, end: number, tags: string[]) {
  const before = value.slice(0, start);
  const after = value.slice(end);
  const prefix = before && /\s$/.test(before) ? "" : " ";
  const suffix = /^\s/.test(after) ? "" : " ";
  const inserted = `${prefix}${tags.join(" ")}${suffix}`;
  return { cursor: start + inserted.length, value: `${before}${inserted}${after}` };
}

export function removeImageTag(value: string, tag: string) {
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

export function jumpAcrossImageTag(value: string, images: ComposerImageAttachment[], cursor: number, direction: "left" | "right") {
  for (const bounds of imageTagBounds(value, images)) {
    if (direction === "left" && cursor > bounds.start && cursor <= bounds.end) return bounds.start;
    if (direction === "right" && cursor >= bounds.start && cursor < bounds.end) return bounds.end;
  }
  return null;
}

export function imageAtCursor(value: string, images: ComposerImageAttachment[], cursor: number, direction: "left" | "right") {
  return imageTagBounds(value, images).find((bounds) => (
    direction === "left"
      ? cursor > bounds.start && cursor <= bounds.end
      : cursor >= bounds.start && cursor < bounds.end
  ))?.image;
}

export function snapSelectionOutsideImageTag(input: HTMLTextAreaElement, value: string, images: ComposerImageAttachment[]) {
  if (input.selectionStart !== input.selectionEnd) return;
  const cursor = input.selectionStart;
  for (const bounds of imageTagBounds(value, images)) {
    if (cursor <= bounds.start || cursor >= bounds.end) continue;
    const nextCursor = cursor - bounds.start < bounds.end - cursor ? bounds.start : bounds.end;
    input.setSelectionRange(nextCursor, nextCursor);
    return;
  }
}

export function positionImageTags(input: HTMLTextAreaElement | null, shell: HTMLDivElement | null, value: string, images: ComposerImageAttachment[]): ImageTagPosition[] {
  if (!input || !shell || !images.length) return [];
  const computed = getComputedStyle(input);
  const mirror = document.createElement("div");
  copyTextareaStyles(computed, mirror);
  const inputRect = input.getBoundingClientRect();
  mirror.style.cssText += `;height:auto;left:${inputRect.left}px;overflow:hidden;position:fixed;top:${inputRect.top}px;visibility:hidden;width:${input.clientWidth}px;`;
  const bounds = images.flatMap(image => {
    const start = value.indexOf(image.tag);
    return start < 0 ? [] : [{ start, end: start + image.tag.length, image }];
  }).sort((left, right) => left.start - right.start);
  const markers: Array<{ id: number; label: HTMLSpanElement; marker: HTMLSpanElement }> = [];
  let cursor = 0;
  for (const bound of bounds) {
    mirror.append(document.createTextNode(value.slice(cursor, bound.start)));
    const marker = document.createElement("span");
    marker.style.whiteSpace = "nowrap";
    const label = document.createElement("span");
    label.textContent = bound.image.tag;
    marker.append(label);
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

function copyTextareaStyles(computed: CSSStyleDeclaration, target: HTMLElement) {
  const copiedProperties = ["border", "boxSizing", "direction", "fontFamily", "fontSize", "fontStyle", "fontWeight", "fontStretch", "fontVariant", "letterSpacing", "lineHeight", "padding", "tabSize", "textIndent", "textTransform", "whiteSpace", "wordBreak", "wordSpacing", "overflowWrap"] as const;
  for (const property of copiedProperties) target.style[property] = computed[property];
}

export function pickerPositionFor(input: HTMLTextAreaElement | null, shell: HTMLDivElement | null, position: number) {
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
    left: Math.max(0, Math.min(markerRect.left - shellRect.left - input.scrollLeft, shellRect.width - 320)),
    top: shellRect.bottom - markerRect.top + input.scrollTop + 6,
  };
}

export function resizeInput(input: HTMLTextAreaElement | null) {
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
