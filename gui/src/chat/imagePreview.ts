import type { MouseEvent } from "react";

// The element fills the stage; only the centered, contained bitmap absorbs clicks.
export function keepImagePreviewOpen(event: MouseEvent<HTMLImageElement>) {
  const image = event.currentTarget;
  if (!image.naturalWidth || !image.naturalHeight) return;
  const bounds = image.getBoundingClientRect();
  const scale = Math.min(bounds.width / image.naturalWidth, bounds.height / image.naturalHeight);
  const width = image.naturalWidth * scale;
  const height = image.naturalHeight * scale;
  const left = bounds.left + (bounds.width - width) / 2;
  const top = bounds.top + (bounds.height - height) / 2;
  if (event.clientX >= left && event.clientX <= left + width && event.clientY >= top && event.clientY <= top + height) {
    event.stopPropagation();
  }
}
