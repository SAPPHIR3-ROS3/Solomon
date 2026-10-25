import { useEffect, useState } from "react";
import { serverEndpoint } from "../platform";
import { imageMimeType } from "./images";
import "./images.css";

export function ImagePreview({ projectID, path }: { projectID: string; path: string }) {
  const [source, setSource] = useState("");
  const [error, setError] = useState(false);
  const [size, setSize] = useState("");
  useEffect(() => {
    const controller = new AbortController();
    let objectURL = "";
    setSource(""); setError(false); setSize("");
    void (async () => {
      try {
        const endpoint = await serverEndpoint(`/__solomon/projects/${encodeURIComponent(projectID)}/file?path=${encodeURIComponent(path)}`);
        const response = await fetch(endpoint, { signal: controller.signal });
        if (!response.ok) throw new Error("Unable to load image");
        const bytes = await response.arrayBuffer();
        if (controller.signal.aborted) return;
        objectURL = URL.createObjectURL(new Blob([bytes], { type: imageMimeType(path) }));
        setSource(objectURL);
      } catch { if (!controller.signal.aborted) setError(true); }
    })();
    return () => { controller.abort(); if (objectURL) URL.revokeObjectURL(objectURL); };
  }, [projectID, path]);
  return <section className="editor-image-preview" aria-label="Image preview">
    {error ? <p role="alert">Unable to display this image. The file may be damaged or unsupported.</p> : source ? <img src={source} alt={path.split(/[\\/]/).at(-1) ?? path} onError={() => setError(true)} onLoad={(event) => { const image = event.currentTarget; setSize(`${image.naturalWidth} × ${image.naturalHeight}`); }} /> : <p role="status">Loading image…</p>}
    {!error && size ? <span className="editor-image-size">{size}</span> : null}
  </section>;
}
