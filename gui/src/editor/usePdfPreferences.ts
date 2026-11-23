import { useEffect, useRef, useState } from "react";
import { readViewState, writeViewState } from "../projects/viewState";
import type { PdfFitMode } from "./pdfRendering";

type PdfPreferences = {
  continuous: boolean;
  direction: "vertical" | "horizontal";
  twoPages: boolean;
  zoom: number;
  fitMode: PdfFitMode;
};

const defaults: PdfPreferences = {
  continuous: false,
  direction: "vertical",
  twoPages: false,
  zoom: 1,
  fitMode: null,
};

export function usePdfPreferences(projectID: string) {
  const path = `/__solomon/projects/${encodeURIComponent(projectID)}/pdf-preferences`;
  const [preferences, setPreferences] = useState(defaults);
  const current = useRef(defaults);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    let cancelled = false;
    setReady(false);
    void readViewState<PdfPreferences>(path).catch(() => null).then((stored) => {
      if (cancelled) return;
      current.current = stored?.direction ? stored : defaults;
      setPreferences(current.current);
      setReady(true);
    });
    return () => { cancelled = true; };
  }, [path]);

  function updatePreferences(change: Partial<PdfPreferences>) {
    if (!ready) return;
    const next = { ...current.current, ...change };
    current.current = next;
    setPreferences(next);
    void writeViewState(path, next).catch(() => undefined);
  }

  return { preferences, ready, updatePreferences };
}
