import { RangeSet, StateEffect, StateField } from "@codemirror/state";
import { EditorView, GutterMarker, gutter } from "@codemirror/view";
import { useEffect, useState, type RefObject } from "react";
import { serverEndpoint } from "../platform";
import { PROJECT_GIT_STATUS_CHANGED_EVENT, PROJECT_GIT_BRANCH_CHANGED_EVENT } from "../projects/projects";
import { changedLines, type LineChange } from "./lineChanges";

class ChangeMarker extends GutterMarker {
  constructor(readonly kind: LineChange["kind"]) { super(); }
  eq(other: ChangeMarker) { return other.kind === this.kind; }
  toDOM() {
    const element = document.createElement("span");
    element.className = `editor-git-marker is-${this.kind}`;
    element.title = this.kind === "added" ? "Added line" : this.kind === "modified" ? "Modified line" : "Deleted lines";
    element.setAttribute("aria-label", element.title);
    return element;
  }
}
const setMarkers = StateEffect.define<RangeSet<GutterMarker>>();
const markerState = StateField.define<RangeSet<GutterMarker>>({
  create: () => RangeSet.empty,
  update(markers, transaction) {
    markers = markers.map(transaction.changes);
    for (const effect of transaction.effects) if (effect.is(setMarkers)) markers = effect.value;
    return markers;
  },
});
export const gitMarkers = [markerState, gutter({
  class: "editor-git-gutter",
  markers: view => view.state.field(markerState),
  initialSpacer: () => new ChangeMarker("added"),
}), EditorView.baseTheme({
  ".editor-git-gutter": { width: "9px", marginRight: "3px" },
  ".editor-git-gutter .cm-gutterElement": { padding: "0", position: "relative" },
  ".editor-git-marker": { display: "block", width: "3px", height: "100%", minHeight: "1em", marginLeft: "3px" },
  ".editor-git-marker.is-added": { backgroundColor: "#3fb950" },
  ".editor-git-marker.is-modified": { background: "repeating-linear-gradient(135deg, #d7aa45 0 2px, transparent 2px 4px)" },
  ".editor-git-marker.is-deleted": { height: "0", minHeight: "0", width: "0", marginLeft: "1px", borderLeft: "6px solid #f85149", borderTop: "4px solid transparent", borderBottom: "4px solid transparent" },
})];

export function useGitMarkers(viewRef: RefObject<EditorView | null>, projectID: string, path: string, content: string) {
  const [baseline, setBaseline] = useState<{ key: string; content: string } | null>(null);
  const key = JSON.stringify([projectID, path]);
  useEffect(() => {
    let disposed = false;
    let controller: AbortController | undefined;
    async function refresh() {
      controller?.abort();
      const request = new AbortController();
      controller = request;
      try {
        const endpoint = await serverEndpoint(`/__solomon/projects/${encodeURIComponent(projectID)}/file?path=${encodeURIComponent(path)}&revision=HEAD`);
        const response = await fetch(endpoint, { signal: request.signal });
        if (!response.ok) throw new Error("Unable to read Git baseline");
        const data = await response.json();
        if (!disposed && !request.signal.aborted) setBaseline(data.isRepo === true && typeof data.content === "string" ? { key, content: data.content } : null);
      } catch { if (!disposed && !request.signal.aborted) setBaseline(null); }
    }
    void refresh();
    const interval = window.setInterval(() => { if (!document.hidden) void refresh(); }, 5000);
    const changed = (event: Event) => {
      const detail = (event as CustomEvent<{ projectID?: string }>).detail;
      if (!detail?.projectID || detail.projectID === projectID) void refresh();
    };
    window.addEventListener(PROJECT_GIT_STATUS_CHANGED_EVENT, changed);
    window.addEventListener(PROJECT_GIT_BRANCH_CHANGED_EVENT, changed);
    return () => {
      disposed = true; controller?.abort(); window.clearInterval(interval);
      window.removeEventListener(PROJECT_GIT_STATUS_CHANGED_EVENT, changed);
      window.removeEventListener(PROJECT_GIT_BRANCH_CHANGED_EVENT, changed);
    };
  }, [projectID, path, key]);
  useEffect(() => {
    const timer = window.setTimeout(() => {
      const view = viewRef.current;
      if (!view || view.state.doc.toString() !== content) return;
      const changes = baseline?.key === key ? changedLines(baseline.content, content) : [];
      const markers = changes.map(change => new ChangeMarker(change.kind).range(view.state.doc.line(Math.min(change.line, view.state.doc.lines)).from));
      view.dispatch({ effects: setMarkers.of(RangeSet.of(markers, true)) });
    }, 120);
    return () => window.clearTimeout(timer);
  }, [baseline, key, content, viewRef]);
}
