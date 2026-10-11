import { CheckIcon, CopyIcon } from "../chat/ChatIcons";
import { copyTextFallback } from "../chat/chatClipboard";
import { useEffect, useRef, useState } from "react";
import { dismissToast, useNotifications, type Notification, type Toast } from "./store";
import "./notifications.css";

function BellIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9M10 21h4" /></svg>;
}
function NotificationBody({ entry }: { entry: Notification }) {
  const [copyState, setCopyState] = useState<"idle" | "copied" | "failed">("idle");
  useEffect(() => {
    if (copyState === "idle") return;
    const timer = setTimeout(() => setCopyState("idle"), 1400);
    return () => clearTimeout(timer);
  }, [copyState]);
  async function copyNotification() {
    try {
      if (navigator.clipboard?.writeText) {
        try { await navigator.clipboard.writeText(entry.message); }
        catch { copyTextFallback(entry.message); }
      } else copyTextFallback(entry.message);
      setCopyState("copied");
    } catch { setCopyState("failed"); }
  }
  const copyLabel = copyState === "copied" ? "Notification copied" : copyState === "failed" ? "Unable to copy notification, try again" : "Copy notification text";
  return <><div className="notification-meta"><strong>{entry.level === "error" ? "Error" : entry.level === "warning" ? "Warning" : "System"}</strong><time dateTime={entry.createdAt}>{new Date(entry.createdAt).toLocaleString()}</time><button aria-label={copyLabel} className="notification-copy" onClick={() => void copyNotification()} title={copyLabel} type="button">{copyState === "copied" ? <CheckIcon /> : <CopyIcon />}</button></div><p>{entry.message}</p></>;
}
export function NotificationControl({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const { history } = useNotifications();
  const ref = useRef<HTMLDivElement>(null);
  const [readCount, setReadCount] = useState(history.length);
  useEffect(() => {
    if (!open) return;
    setReadCount(history.length);
    function outside(event: PointerEvent) { if (!ref.current?.contains(event.target as Node)) onOpenChange(false); }
    function escape(event: KeyboardEvent) { if (event.key === "Escape") { onOpenChange(false); ref.current?.querySelector("button")?.focus(); } }
    document.addEventListener("pointerdown", outside);
    document.addEventListener("keydown", escape);
    return () => { document.removeEventListener("pointerdown", outside); document.removeEventListener("keydown", escape); };
  }, [open, history.length, onOpenChange]);
  return <div className="notification-control" ref={ref}>
    <button aria-label="Notifications" aria-expanded={open} aria-haspopup="dialog" className="notification-bell" title="Notifications" type="button" onClick={() => onOpenChange(!open)}><BellIcon />{history.length > readCount ? <span className="notification-unread" /> : null}</button>
    {open ? <section aria-label="Notification history" className="notification-menu" role="dialog"><header>Notifications <span>{history.length}</span></header><div className="notification-history" tabIndex={0}>{history.length ? history.map((entry) => <article className={`notification-entry is-${entry.level}`} key={entry.id}><NotificationBody entry={entry} /></article>) : <p className="notification-empty">No notifications yet.</p>}</div></section> : null}
  </div>;
}
function NotificationToast({ entry }: { entry: Toast }) {
  useEffect(() => {
    const timer = setTimeout(() => dismissToast(entry.id), Math.max(0, entry.expiresAt - Date.now()));
    return () => clearTimeout(timer);
  }, [entry.id, entry.expiresAt]);
  return <article className={`notification-toast is-${entry.level}`} role={entry.level === "error" ? "alert" : "status"}><NotificationBody entry={entry} /><button aria-label="Dismiss notification" className="notification-dismiss" onClick={() => dismissToast(entry.id)} type="button">×</button><div aria-hidden="true" className="notification-progress" /></article>;
}
export function NotificationToasts() {
  const { toasts } = useNotifications();
  return <div className="notification-toasts">{toasts.map((entry) => <NotificationToast entry={entry} key={entry.id} />)}</div>;
}
