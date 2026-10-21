import { CheckIcon, CopyIcon } from "../chat/ChatIcons";
import { copyTextFallback } from "../chat/chatClipboard";
import { serverEndpoint } from "../platform";
import { useEffect, useRef, useState } from "react";
import { dismissToast, groupNotifications, useNotifications, type Notification, type NotificationGroup, type Toast } from "./store";
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
function RelatedErrors({ entry }: { entry: NotificationGroup }) {
  if (!entry.children.length) return null;
  return <details className="notification-related"><summary>{entry.children.length} related {entry.children.length === 1 ? "error" : "errors"}</summary><div>{entry.children.map(child => <article className="notification-child is-error" key={child.id}><NotificationBody entry={child} /></article>)}</div></details>;
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
    {open ? <section aria-label="Notification history" className="notification-menu" role="dialog"><header><span>Notifications</span><span className="notification-header-actions"><span>{history.length}</span><ReportButton placement="menu" /></span></header><div className="notification-history" tabIndex={0}>{history.length ? groupNotifications(history).map((entry) => <article className={`notification-entry is-${entry.level}`} key={entry.id}><NotificationBody entry={entry} /><RelatedErrors entry={entry} /></article>) : <p className="notification-empty">No notifications yet.</p>}</div></section> : null}
  </div>;
}
function NotificationToast({ entry }: { entry: Toast }) {
  useEffect(() => {
    const timer = setTimeout(() => dismissToast(entry.id), Math.max(0, entry.expiresAt - Date.now()));
    return () => clearTimeout(timer);
  }, [entry.id, entry.expiresAt]);
  return <article className={`notification-toast is-${entry.level}`} role={entry.level === "error" ? "alert" : "status"}><NotificationBody entry={entry} /><RelatedErrors entry={entry} /><button aria-label="Dismiss notification" className="notification-dismiss" onClick={() => dismissToast(entry.id)} type="button">×</button><div aria-hidden="true" className="notification-progress" /></article>;
}
export function ReportButton({ placement }: { placement: "menu" | "settings" }) {
  const [open, setOpen] = useState(false);
  const [note, setNote] = useState("");
  const [sending, setSending] = useState(false);
  const [error, setError] = useState("");
  async function sendReport() {
    setSending(true);
    setError("");
    try {
      const response = await fetch(await serverEndpoint("/__solomon/report"), {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ note }),
      });
      const payload = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(payload.error || "Unable to send report");
      setNote("");
      setOpen(false);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Unable to send report");
    } finally {
      setSending(false);
    }
  }
  const button = <button className={placement === "menu" ? "notification-report" : "settings-dev-apply"} disabled={sending} onClick={() => { setError(""); setOpen(true); }} type="button">Send report</button>;
  return <>
    {placement === "settings" ? <div className="settings-model-row settings-dev-header"><div className="settings-model-row-copy"><strong>Send report</strong><small>Send the latest error, warning, and info logs. A note is optional.</small>{error && !open ? <small className="settings-dev-error" role="alert">{error}</small> : null}</div>{button}</div> : button}
    {open ? <div className="report-backdrop" role="presentation"><section aria-labelledby="report-title" className="report-dialog" role="dialog"><h2 id="report-title">Send report</h2><p>Recent error, warning, and info logs are anonymized and sent. The note is optional.</p><textarea aria-label="Optional note" maxLength={500} onChange={event => setNote(event.target.value)} placeholder="Optional note" rows={4} value={note} /><div className="report-actions">{error ? <span role="alert">{error}</span> : <span />}<button disabled={sending} onClick={() => { if (!sending) setOpen(false); }} type="button">Cancel</button><button disabled={sending} onClick={() => void sendReport()} type="button">{sending ? "Sending…" : "Send"}</button></div></section></div> : null}
  </>;
}
export function NotificationToasts() {
  const { toasts } = useNotifications();
  return <div className="notification-toasts">{toasts.map((entry) => <NotificationToast entry={entry} key={entry.id} />)}</div>;
}
