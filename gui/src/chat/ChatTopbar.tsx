import { useLayoutEffect, useRef, useState } from "react";
import { FolderIcon } from "./ChatIcons";

export function ChatTopbar({ breadcrumb, onOpenFolder, title }: { breadcrumb?: string; onOpenFolder: () => void; title: string }) {
  const topbarRef = useRef<HTMLDivElement>(null);
  const contextRef = useRef<HTMLButtonElement>(null);
  const folderName = breadcrumb ?? "Project";
  const [showContext, setShowContext] = useState(true);
  const [visibleTitle, setVisibleTitle] = useState(title);

  useLayoutEffect(() => {
    const topbar = topbarRef.current;
    const context = contextRef.current;
    if (!topbar || !context) return;
    const measure = () => {
      // Research reports always keep the folder breadcrumb actionable; the
      // report title can shrink and ellipsize when the available width is tight.
      if (breadcrumb) {
        setShowContext(true);
        setVisibleTitle(title);
        return;
      }
      const available = topbar.clientWidth;
      const contextWidth = context.getBoundingClientRect().width;
      const fullTitleWidth = measureTopbarText(title);
      if (contextWidth + 10 + fullTitleWidth <= available) {
        setShowContext(true);
        setVisibleTitle(title);
        return;
      }
      setShowContext(false);
      setVisibleTitle(title);
    };
    const observer = new ResizeObserver(measure);
    observer.observe(topbar);
    measure();
    return () => observer.disconnect();
  }, [breadcrumb, title]);

  return (
    <div aria-label={`${folderName} / ${title}`} className="chat-topbar" ref={topbarRef}>
      <button aria-label={`Back to new chat in ${folderName}`} className={`chat-topbar-context${showContext ? "" : " is-hidden"}`} onClick={onOpenFolder} ref={contextRef} type="button">
        <FolderIcon />
        <span className="chat-topbar-folder">{folderName}</span>
        <span aria-hidden="true" className="chat-topbar-slash">/</span>
      </button>
      <span className="chat-topbar-title" title={title}>{visibleTitle}</span>
    </div>
  );
}

function measureTopbarText(value: string) {
  if (typeof document === "undefined") return value.length * 10;
  const canvas = document.createElement("canvas");
  const context = canvas.getContext("2d");
  if (!context) return value.length * 10;
  context.font = '650 20px "Geist", ui-sans-serif, system-ui, sans-serif';
  return context.measureText(value).width;
}
