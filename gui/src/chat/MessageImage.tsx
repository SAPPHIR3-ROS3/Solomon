import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { keepImagePreviewOpen } from "./imagePreview";
import { CloseIcon } from "./ChatIcons";

export function MessageImage({ src, alt = "Image" }: { src?: string; alt?: string }) {
  const [open, setOpen] = useState(false);
  useEffect(() => {
    if (!open) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") { event.preventDefault(); setOpen(false); }
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open]);
  return <>
    <button className="chat-markdown-image" type="button" aria-label={`Enlarge ${alt}`} onClick={(event) => { event.preventDefault(); setOpen(true); }}><img src={src} alt={alt} /></button>
    {open ? createPortal(<div className="composer-image-lightbox chat-message-lightbox" role="dialog" aria-modal="true" aria-label={`Image preview: ${alt}`} onClick={() => setOpen(false)}>
      <button autoFocus className="chat-image-lightbox-close" type="button" aria-label="Close preview" onClick={() => setOpen(false)}><CloseIcon /></button>
      <div className="composer-image-lightbox-stage"><img onClick={keepImagePreviewOpen} className="composer-image-lightbox-image" src={src} alt={alt} /></div>
    </div>, document.body) : null}
  </>;
}
