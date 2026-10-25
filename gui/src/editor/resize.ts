import type { PointerEvent as ReactPointerEvent } from "react";

export function startSideResize(event: ReactPointerEvent<HTMLButtonElement>, sideWidth: number, setSideWidth: (width: number) => void, setSideResizing: (resizing: boolean) => void) {
    event.preventDefault();
    setSideResizing(true);
    const startX = event.clientX;
    const startWidth = sideWidth;
    const maxWidth = window.innerWidth / 3;
    const minWidth = Math.min(190, maxWidth);
    const resize = (moveEvent: PointerEvent) => {
      setSideWidth(Math.min(maxWidth, Math.max(minWidth, startWidth + moveEvent.clientX - startX)));
    };
    const stop = () => {
      setSideResizing(false);
      window.removeEventListener("pointermove", resize);
      window.removeEventListener("pointerup", stop);
      window.removeEventListener("pointercancel", stop);
    };
    window.addEventListener("pointermove", resize);
    window.addEventListener("pointerup", stop);
    window.addEventListener("pointercancel", stop);
  }

