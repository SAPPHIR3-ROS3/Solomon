import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { externalLinkURL } from "../externalLinks";
import { copyText } from "../chat/chatClipboard";

export function LinkContextMenu() {
  const [target, setTarget] = useState<{ url: string; anchor: HTMLElement; x: number; y: number } | null>(null);
  const [error, setError] = useState("");
  const menuRef = useRef<HTMLDivElement>(null);
  const close = () => { setTarget(null); target?.anchor.focus({ preventScroll: true }); };

  useEffect(() => {
    const show = (event: MouseEvent) => {
      if (event.defaultPrevented || !window.runtime?.BrowserOpenURL) return;
      const url = externalLinkURL(event.target);
      const anchor = event.target instanceof Element ? event.target.closest<HTMLElement>("a[href]") : null;
      if (!url || !anchor) { setTarget(null); return; }
      event.preventDefault();
      const bounds = anchor.getBoundingClientRect();
      setError("");
      setTarget({ url, anchor, x: event.clientX || bounds.left, y: event.clientY || bounds.bottom });
    };
    document.addEventListener("contextmenu", show);
    return () => document.removeEventListener("contextmenu", show);
  }, []);

  useEffect(() => {
    if (!target) return;
    const outside = (event: PointerEvent) => { if (!menuRef.current?.contains(event.target as Node)) setTarget(null); };
    const dismiss = () => setTarget(null);
    document.addEventListener("pointerdown", outside);
    document.addEventListener("scroll", dismiss, true);
    window.addEventListener("resize", dismiss);
    return () => {
      document.removeEventListener("pointerdown", outside);
      document.removeEventListener("scroll", dismiss, true);
      window.removeEventListener("resize", dismiss);
    };
  }, [target]);

  useLayoutEffect(() => {
    const menu = menuRef.current;
    if (!target || !menu) return;
    const bounds = menu.getBoundingClientRect();
    menu.style.left = `${Math.max(8, Math.min(target.x, window.innerWidth - bounds.width - 8))}px`;
    menu.style.top = `${Math.max(8, Math.min(target.y, window.innerHeight - bounds.height - 8))}px`;
    menu.querySelector("button")?.focus();
  }, [target]);

  if (!target) return null;
  return createPortal(<div ref={menuRef} className="project-context-menu" role="menu" aria-label="Link actions" style={{ left: target.x, top: target.y }} onContextMenu={event => event.preventDefault()} onKeyDown={event => {
    if (event.key === "Escape") { event.preventDefault(); event.stopPropagation(); close(); return; }
    const items = Array.from(event.currentTarget.querySelectorAll("button"));
    const index = items.indexOf(document.activeElement as HTMLButtonElement);
    if (["ArrowDown", "ArrowUp", "Home", "End", "Tab"].includes(event.key)) {
      event.preventDefault();
      const next = event.key === "Home" ? 0 : event.key === "End" ? items.length - 1 : (index + (event.key === "ArrowUp" || (event.key === "Tab" && event.shiftKey) ? -1 : 1) + items.length) % items.length;
      items[next]?.focus();
    }
  }}>
    <button role="menuitem" type="button" onClick={() => { window.runtime?.BrowserOpenURL?.(target.url); close(); }}>Open in browser</button>
    <button role="menuitem" type="button" onClick={() => { void copyText(target.url).then(close).catch(() => setError("Unable to copy link address.")); }}>Copy link address</button>
    {error ? <p role="alert">{error}</p> : null}
  </div>, document.body);
}
