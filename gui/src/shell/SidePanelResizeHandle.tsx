import { type PointerEvent as ReactPointerEvent } from "react";

type SidePanelResizeHandleProps = {
  onWidthChange: (width: number) => void;
  side: "left" | "right";
  width: number;
};

export function SidePanelResizeHandle({ onWidthChange, side, width }: SidePanelResizeHandleProps) {
  function startResize(event: ReactPointerEvent<HTMLButtonElement>) {
    event.preventDefault();
    const startX = event.clientX;

    const onPointerMove = (moveEvent: PointerEvent) => {
      const delta = side === "left" ? moveEvent.clientX - startX : startX - moveEvent.clientX;
      onWidthChange(width + delta);
    };
    const stopResize = () => {
      window.removeEventListener("pointermove", onPointerMove);
      window.removeEventListener("pointerup", stopResize);
    };

    window.addEventListener("pointermove", onPointerMove);
    window.addEventListener("pointerup", stopResize, { once: true });
  }

  return (
    <button
      aria-label={`Resize ${side} side panel`}
      className={`side-panel-resize side-panel-resize-${side}`}
      onPointerDown={startResize}
      title="Drag to resize"
      type="button"
    />
  );
}
