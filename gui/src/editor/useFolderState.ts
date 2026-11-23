import { useEffect, useRef, useState } from "react";
import { saveFolderState } from "../projects/folders";
import type { EditorSession } from "./editorSession";

export function useFolderState(projectID: string | undefined, session: EditorSession, onError: (error: string) => void) {
  const [expanded, setExpanded] = useState(() => new Set(session.expanded));
  const [rootCollapsed, setRootCollapsed] = useState(session.rootCollapsed);
  const [foldersReady, setFoldersReady] = useState(false);
  const baseline = useRef("");

  function restoreFolders(state: Pick<EditorSession, "expanded" | "rootCollapsed">) {
    baseline.current = JSON.stringify({ expanded: [...new Set(state.expanded)], rootCollapsed: state.rootCollapsed });
    setExpanded(new Set(state.expanded));
    setRootCollapsed(state.rootCollapsed);
    setFoldersReady(true);
  }

  useEffect(() => {
    if (!projectID || !foldersReady) return;
    const state = { expanded: [...expanded], rootCollapsed };
    const snapshot = JSON.stringify(state);
    if (baseline.current === snapshot) return;
    baseline.current = snapshot;
    void saveFolderState(projectID, "editor", state).catch(() => onError("Could not save folder state."));
  }, [projectID, expanded, rootCollapsed, foldersReady, onError]);

  return { expanded, setExpanded, rootCollapsed, setRootCollapsed, foldersReady, restoreFolders };
}
