import { useEffect, type RefObject } from "react";
import type { ComposerImageAttachment } from "../../chat/composerTypes";
import { clipboardImageFiles, isImageFileDrag, nativeImageDropPaths, readNativeDroppedImages, supportsNativeImageDrop } from "../clipboardImages";
import { NATIVE_IMAGE_DROP_EVENT, usesNativeFileDrops } from "../imageDrops";
import { notifyError } from "../../notifications/store";

type DropOptions = {
  canAttachImages: boolean;
  shellRef: RefObject<HTMLDivElement | null>;
  latestComposerRef: RefObject<{ attach: (files: File[]) => void; value: string; images: ComposerImageAttachment[] }>;
  pasteRequestRef: RefObject<number>;
  setIsDraggingImages: (dragging: boolean) => void;
};

export function useImageDrop({ canAttachImages, shellRef, latestComposerRef, pasteRequestRef, setIsDraggingImages }: DropOptions) {
  useEffect(() => {
    if (!canAttachImages) return;
    const target: HTMLElement | null = shellRef.current?.closest<HTMLFormElement>("form") ?? shellRef.current;
    if (!target) return;
    let depth = 0;
    let active = true;
    const visible = () => target.isConnected && target.getClientRects().length > 0 &&
      getComputedStyle(target).visibility === "visible" && !target.closest('[aria-hidden="true"], [inert]');
    const imageDrag = (data: DataTransfer) => isImageFileDrag(data) ||
      (supportsNativeImageDrop() && Array.from(data.types ?? []).includes("text/uri-list"));
    const reset = () => { depth = 0; setIsDraggingImages(false); };
    const attachPaths = (paths: string[]) => {
      paths = paths.filter(path => /\.(png|jpe?g|webp|gif)$/i.test(path));
      if (!paths.length || !visible()) return;
      const composer = latestComposerRef.current;
      void readNativeDroppedImages(paths).then((images) => {
        if (active && visible() && composer.value === latestComposerRef.current.value &&
            composer.images === latestComposerRef.current.images) latestComposerRef.current.attach(images);
      }).catch((error: unknown) => {
        if (active) notifyError(`Unable to attach dropped images: ${String(error)}`);
      });
    };
    const nativeDrop = (event: Event) => {
      reset();
      if (!visible()) return;
      pasteRequestRef.current += 1;
      attachPaths((event as CustomEvent<string[]>).detail);
    };
    const enter = (event: DragEvent) => {
      if (!visible() || !event.dataTransfer || !imageDrag(event.dataTransfer)) return;
      event.preventDefault();
      event.stopPropagation();
      depth += 1;
      setIsDraggingImages(true);
    };
    const over = (event: DragEvent) => {
      if (!visible() || !event.dataTransfer || !imageDrag(event.dataTransfer)) return;
      event.preventDefault();
      event.stopPropagation();
      event.dataTransfer.dropEffect = "copy";
    };
    const leave = () => {
      depth = Math.max(0, depth - 1);
      if (!depth) setIsDraggingImages(false);
    };
    const drop = (event: DragEvent) => {
      reset();
      if (!visible()) return;
      const data = event.dataTransfer;
      if (!data) return;
      const paths = supportsNativeImageDrop() ? nativeImageDropPaths(data) : [];
      const hasFiles = Array.from(data.types ?? []).includes("Files") ||
        Array.from(data.items ?? []).some((item) => item.kind === "file") || data.files.length > 0;
      if (!hasFiles && !paths.length) return;
      event.preventDefault();
      if (usesNativeFileDrops() && supportsNativeImageDrop()) return;
      event.stopPropagation();
      pasteRequestRef.current += 1;
      const files = clipboardImageFiles(data);
      if (files.length) latestComposerRef.current.attach(files);
      else if (paths.length) attachPaths(paths);
    };
    window.addEventListener("dragenter", enter, true);
    window.addEventListener("dragover", over, true);
    window.addEventListener("dragleave", leave, true);
    window.addEventListener("dragend", reset, true);
    window.addEventListener("drop", drop, true);
    window.addEventListener(NATIVE_IMAGE_DROP_EVENT, nativeDrop);
    return () => {
      active = false;
      window.removeEventListener("dragenter", enter, true);
      window.removeEventListener("dragover", over, true);
      window.removeEventListener("dragleave", leave, true);
      window.removeEventListener("dragend", reset, true);
      window.removeEventListener("drop", drop, true);
      window.removeEventListener(NATIVE_IMAGE_DROP_EVENT, nativeDrop);
    };
  }, [canAttachImages]);
}
