import { nativeImageDropPaths } from "./clipboardImages";

export const NATIVE_IMAGE_DROP_EVENT = "solomon:native-image-drop";
let nativeDropsRegistered = false;

export function usesNativeFileDrops(): boolean {
  return nativeDropsRegistered;
}

// This guard belongs to the window, not a textarea. A miss on the composer
// must never replace Solomon with the browser's standalone image document.
export function watchImageDrops(): () => void {
  const guard = (event: DragEvent) => {
    const data = event.dataTransfer;
    if (!data) return;
    if (Array.from(data.types ?? []).some(type => type === "Files" || type === "text/uri-list") ||
        Array.from(data.items ?? []).some(item => item.kind === "file") ||
        data.files.length || nativeImageDropPaths(data).length) event.preventDefault();
  };
  window.addEventListener("dragover", guard, true);
  window.addEventListener("drop", guard, true);
  const runtime = window.runtime;
  if (runtime?.OnFileDrop) {
    runtime.OnFileDrop((_x, _y, paths) => {
      window.dispatchEvent(new CustomEvent(NATIVE_IMAGE_DROP_EVENT, {detail: paths}));
    }, false);
    nativeDropsRegistered = true;
  }
  return () => {
    window.removeEventListener("dragover", guard, true);
    window.removeEventListener("drop", guard, true);
    if (runtime?.OnFileDrop) {
      runtime.OnFileDropOff?.();
      nativeDropsRegistered = false;
    }
  };
}
