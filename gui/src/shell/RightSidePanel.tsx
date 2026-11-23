import { loadFolderState, saveFolderState } from "../projects/folders";
import { fileBrowserStatuses } from "./right-panel/status";
import { FileBrowser } from "./right-panel/Browser";
import { useResearchContextMenu } from "../research/ContextMenu";
import { FileEntries, entryMatchesFilter } from "./right-panel/Files";
import { GitHistoryView } from "./right-panel/History";
import { NewDocumentIcon, GitHistoryIcon, ResearchIcon } from "./right-panel/icons";
export { FileEntries } from "./right-panel/Files";
export { GitHistoryView } from "./right-panel/History";
export { SearchIcon, NewDocumentIcon, GitHistoryIcon } from "./right-panel/icons";
import { type CSSProperties, useEffect, useRef, useState } from "react";
import { fetchHomeDirectoryEntries, fetchProjectDirectoryEntries, fetchProjectGitHistory, fetchProjectGitStatus, fetchProjectResearch, PROJECT_GIT_BRANCH_CHANGED_EVENT, PROJECT_GIT_STATUS_CHANGED_EVENT, sameProjectDirectoryEntries, sameProjectGitHistory, sameProjectGitStatus, type Project, type ProjectDirectoryEntry, type ProjectGitHistory, type ProjectGitStatus, type ProjectResearch, PROJECT_FILES_CHANGED_EVENT, type ProjectFilesChanged } from "../projects/projects";
import type { TemporaryWorkspace } from "../projects/temporaryWorkspace";
import { SidePanelResizeHandle } from "./SidePanelResizeHandle";

const EXPLORER_STATE_STORAGE_PREFIX = "solomon.explorer-state.v1";
const EMPTY_GIT_HISTORY: ProjectGitHistory = { commits: [], current: "", isRepo: false };
const EMPTY_GIT_STATUS: ProjectGitStatus = { changes: {}, isRepo: false, staged: {} };
type ExplorerState = {
  expandedDirectories: string[];
  scrollTop: number;
};

function explorerStateStorageKey(projectID: string) {
  return `${EXPLORER_STATE_STORAGE_PREFIX}.${projectID}`;
}

function loadExplorerState(projectID: string): ExplorerState {
  try {
    const value: unknown = JSON.parse(window.localStorage.getItem(explorerStateStorageKey(projectID)) ?? "null");
    if (!value || typeof value !== "object") return { expandedDirectories: [], scrollTop: 0 };
    const { expandedDirectories, scrollTop } = value as Partial<ExplorerState>;
    return {
      expandedDirectories: Array.isArray(expandedDirectories) ? expandedDirectories.filter((path): path is string => typeof path === "string") : [],
      scrollTop: typeof scrollTop === "number" && Number.isFinite(scrollTop) && scrollTop > 0 ? scrollTop : 0,
    };
  } catch {
    return { expandedDirectories: [], scrollTop: 0 };
  }
}

function saveExplorerState(projectID: string, expandedDirectories: Set<string>, scrollTop: number) {
  try {
    window.localStorage.setItem(explorerStateStorageKey(projectID), JSON.stringify({
      expandedDirectories: [...expandedDirectories],
      scrollTop: Math.max(0, scrollTop),
    } satisfies ExplorerState));
  } catch {
    // The app remains fully usable when browser storage is unavailable.
  }
}

type RightSidePanelProps = {
  bottomInset: number;
  onWidthChange: (width: number) => void;
  onOpenFile?: (entry: ProjectDirectoryEntry) => void;
  onOpenResearch: (research: ProjectResearch) => void;
  project: Project | null;
  temporaryWorkspace: TemporaryWorkspace | null;
  width: number;
};

export function RightSidePanel({ bottomInset, onOpenResearch, onOpenFile, onWidthChange, project, temporaryWorkspace, width }: RightSidePanelProps) {
  const [activeView, setActiveView] = useState<"files" | "history" | "research">("files");
  const [entries, setEntries] = useState<Record<string, ProjectDirectoryEntry[]>>({});
  const entriesRef = useRef(entries);
  entriesRef.current = entries;
  const [expandedDirectories, setExpandedDirectories] = useState<Set<string>>(() => new Set());
  const expandedDirectoriesRef = useRef(expandedDirectories);
  const loadedExplorerRef = useRef<string | null>(null);
  const [error, setError] = useState("");
  const [query, setQuery] = useState("");
  const [scrollShadowOpacity, setScrollShadowOpacity] = useState(0);
  const [bottomScrollShadowOpacity, setBottomScrollShadowOpacity] = useState(0);
  const [research, setResearch] = useState<ProjectResearch[]>([]);
  const [researchError, setResearchError] = useState("");
  const [researchLoading, setResearchLoading] = useState(false);
  const researchMenu = useResearchContextMenu(project?.id, id => setResearch(current => current.filter(job => job.id !== id)));
  const [gitHistory, setGitHistory] = useState<ProjectGitHistory>(EMPTY_GIT_HISTORY);
  const [gitHistoryError, setGitHistoryError] = useState("");
  const [gitHistoryLoading, setGitHistoryLoading] = useState(false);
  const [gitHistoryProjectID, setGitHistoryProjectID] = useState("");
  const [gitStatus, setGitStatus] = useState<ProjectGitStatus>(EMPTY_GIT_STATUS);
  const [gitStatusError, setGitStatusError] = useState("");
  const [gitStatusLoading, setGitStatusLoading] = useState(false);
  const filesRef = useRef<HTMLElement>(null);
  const restoredScrollPositionRef = useRef<{ projectID: string; scrollTop: number } | null>(null);
  const nameFilter = query.trim().toLowerCase();
  const isGitHistoryAvailable = Boolean(project && gitHistoryProjectID === project.id && gitHistory.isRepo);
  const visibleView = activeView === "history" && !isGitHistoryAvailable ? "files" : activeView;

  useEffect(() => {
    setResearch([]);
    setResearchError("");
    setResearchLoading(false);
    setResearchLoading(Boolean(project));
    if (!project) return;
    let cancelled = false;
    void fetchProjectResearch(project.id)
      .then((jobs) => {
        if (!cancelled) setResearch(jobs);
      })
      .catch(() => {
        if (!cancelled) setResearchError("Could not load deep research for this folder.");
      })
      .finally(() => {
        if (!cancelled) setResearchLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [project]);

  useEffect(() => {
    setGitHistory(EMPTY_GIT_HISTORY);
    setGitHistoryError("");
    setGitHistoryLoading(Boolean(project));
    setGitHistoryProjectID(project?.id ?? "");
    setGitStatus(EMPTY_GIT_STATUS);
    setGitStatusError("");
    setGitStatusLoading(Boolean(project));
    if (!project) return;
    let cancelled = false;
    void fetchProjectGitHistory(project.id)
      .then((history) => {
        if (!cancelled) setGitHistory(history);
      })
      .catch(() => {
        if (!cancelled) setGitHistoryError("Could not load Git history for this folder.");
      })
      .finally(() => {
        if (!cancelled) setGitHistoryLoading(false);
      });
    void fetchProjectGitStatus(project.id)
      .then((status) => {
        if (!cancelled) setGitStatus(status);
      })
      .catch(() => {
        if (!cancelled) setGitStatusError("Could not load Git changes for this folder.");
      })
      .finally(() => {
        if (!cancelled) setGitStatusLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [project]);

  useEffect(() => {
    setEntries({});
    loadedExplorerRef.current = null;
    expandedDirectoriesRef.current = new Set();
    setExpandedDirectories(expandedDirectoriesRef.current);
    setError("");
    setQuery("");
    setScrollShadowOpacity(0);
    setBottomScrollShadowOpacity(0);
    const explorerID = project?.id ?? temporaryWorkspace?.id;
    if (!explorerID) {
      restoredScrollPositionRef.current = null;
      return;
    }
    const restoredState = loadExplorerState(explorerID);
    restoredScrollPositionRef.current = { projectID: explorerID, scrollTop: restoredState.scrollTop };
    let cancelled = false;
    const fallback = { expanded: restoredState.expandedDirectories, rootCollapsed: false };
    const folderState = project ? loadFolderState(project.id, "chat-files", fallback) : Promise.resolve(fallback);
    const readDirectory = (path: string) => project
      ? fetchProjectDirectoryEntries(project.id, path)
      : fetchHomeDirectoryEntries(path || temporaryWorkspace!.path);
    void Promise.all([folderState, readDirectory("")])
      .then(async ([state, rootEntries]) => {
        const restoredEntries: Record<string, ProjectDirectoryEntry[]> = { "": rootEntries };
        const restoredDirectories = new Set(state.expanded);
        await Promise.all([...restoredDirectories].map(async (path) => {
          try { restoredEntries[path] = await readDirectory(path); }
          catch { return; }
        }));
        if (cancelled) return;
        loadedExplorerRef.current = explorerID;
        expandedDirectoriesRef.current = restoredDirectories;
        setExpandedDirectories(restoredDirectories);
        setEntries(restoredEntries);
      })
      .catch(() => {
        if (!cancelled) setError("Could not load the project files.");
      });
    return () => {
      cancelled = true;
    };
  }, [project, temporaryWorkspace]);

  useEffect(() => {
    const files = filesRef.current;
    if (!files || !entries[""]) return;
    const restoredPosition = restoredScrollPositionRef.current;
    const explorerID = project?.id ?? temporaryWorkspace?.id;
    if (!explorerID) return;
    if (!restoredPosition || restoredPosition.projectID !== explorerID) return;
    const frame = window.requestAnimationFrame(() => {
      files.scrollTop = restoredPosition.scrollTop;
      restoredScrollPositionRef.current = null;
    });
    return () => window.cancelAnimationFrame(frame);
  }, [entries, expandedDirectories, project, temporaryWorkspace]);

  useEffect(() => {
    const files = filesRef.current;
    if (!files) return;
    const explorerID = project?.id ?? temporaryWorkspace?.id;
    if (!explorerID) return;
    const updateScrollChrome = () => {
      setScrollShadowOpacity(Math.min(1, files.scrollTop / 18));
      const scrollableHeight = files.scrollHeight - files.clientHeight;
      setBottomScrollShadowOpacity(Math.min(1, Math.max(0, scrollableHeight - files.scrollTop) / 18));
      if (loadedExplorerRef.current === explorerID) saveExplorerState(explorerID, expandedDirectoriesRef.current, files.scrollTop);
    };
    const resizeObserver = new ResizeObserver(updateScrollChrome);
    resizeObserver.observe(files);
    files.addEventListener("scroll", updateScrollChrome, { passive: true });
    updateScrollChrome();
    return () => {
      resizeObserver.disconnect();
      files.removeEventListener("scroll", updateScrollChrome);
    };
  }, [bottomInset, entries, expandedDirectories, nameFilter, project, temporaryWorkspace]);

  useEffect(() => {
    if (!project) return;
    let cancelled = false;
    const refresh = (event: Event) => {
      const change = (event as CustomEvent<ProjectFilesChanged>).detail;
      if (change.projectID !== project.id) return;
      const affected = (path: string) => path === change.path || path.startsWith(`${change.path}/`);
      const updatedPath = (path: string) => affected(path) && change.destination && change.action !== "copy" ? change.destination + path.slice(change.path.length) : path;
      if (loadedExplorerRef.current !== project.id) return;
      const current = expandedDirectoriesRef.current;
      const next = new Set([...current].filter((path) => !(change.action === "delete" && affected(path))).map(updatedPath));
      if (next.size !== current.size || [...next].some((path) => !current.has(path))) updateExpandedDirectories(project.id, next);
      void Promise.all(Object.keys(entriesRef.current).map(async (path) => {
        const nextPath = updatedPath(path);
        try { return [nextPath, await fetchProjectDirectoryEntries(project.id, nextPath)] as const; }
        catch { return [nextPath, []] as const; }
      })).then((loaded) => { if (!cancelled) { setEntries(Object.fromEntries(loaded)); setError(""); } });
      void fetchProjectGitStatus(project.id).then((status) => { if (!cancelled) setGitStatus(status); }).catch(() => undefined);
    };
    const refreshGitStatus = (event: Event) => {
      const detail = (event as CustomEvent<{ projectID?: string }>).detail;
      if (detail?.projectID && detail.projectID !== project.id) return;
      void fetchProjectGitStatus(project.id).then((status) => { if (!cancelled) setGitStatus(status); }).catch(() => undefined);
    };
    window.addEventListener(PROJECT_FILES_CHANGED_EVENT, refresh);
    window.addEventListener(PROJECT_GIT_STATUS_CHANGED_EVENT, refreshGitStatus);
    return () => { cancelled = true; window.removeEventListener(PROJECT_FILES_CHANGED_EVENT, refresh); window.removeEventListener(PROJECT_GIT_STATUS_CHANGED_EVENT, refreshGitStatus); };
  }, [project?.id]);

  useEffect(() => {
    if (!project && !temporaryWorkspace) return;
    let cancelled = false;
    let timer: number | undefined;
    let inFlight = false;
    const readDirectory = (path: string) => project
      ? fetchProjectDirectoryEntries(project.id, path)
      : fetchHomeDirectoryEntries(path || temporaryWorkspace!.path);
    const refresh = async () => {
      if (inFlight) return;
      inFlight = true;
      const paths = [...new Set(["", ...Object.keys(entriesRef.current).filter((path) => expandedDirectoriesRef.current.has(path))])];
      const [listings, status] = await Promise.all([
        Promise.all(paths.map(async (path) => {
          try { return [path, await readDirectory(path)] as const; }
          catch { return null; }
        })),
        project ? fetchProjectGitStatus(project.id).catch(() => null) : Promise.resolve(null),
      ]);
      if (!cancelled) {
        setEntries((current) => {
          let changed = false;
          const next = { ...current };
          for (const listing of listings) {
            if (!listing) continue;
            const [path, nextEntries] = listing;
            if (sameProjectDirectoryEntries(current[path], nextEntries)) continue;
            next[path] = nextEntries;
            changed = true;
          }
          return changed ? next : current;
        });
        if (status) setGitStatus((current) => sameProjectGitStatus(current, status) ? current : status);
      }
      inFlight = false;
      if (!cancelled) timer = window.setTimeout(() => void refresh(), 1200);
    };
    timer = window.setTimeout(() => void refresh(), 1200);
    return () => {
      cancelled = true;
      if (timer !== undefined) window.clearTimeout(timer);
    };
  }, [project?.id, temporaryWorkspace?.id]);

  useEffect(() => {
    if (!project || activeView !== "history") return;
    let cancelled = false;
    let timer: number | undefined;
    const refresh = async () => {
      try {
        const nextHistory = await fetchProjectGitHistory(project.id);
        if (!cancelled) setGitHistory((current) => sameProjectGitHistory(current, nextHistory) ? current : nextHistory);
      } catch {
        // Keep the last successful graph visible while the daemon is unavailable.
      }
      if (!cancelled) timer = window.setTimeout(() => void refresh(), 3500);
    };
    timer = window.setTimeout(() => void refresh(), 3500);
    return () => {
      cancelled = true;
      if (timer !== undefined) window.clearTimeout(timer);
    };
  }, [project?.id, activeView]);

  function updateExpandedDirectories(explorerID: string, next: Set<string>) {
    expandedDirectoriesRef.current = next;
    setExpandedDirectories(next);
    saveExplorerState(explorerID, next, filesRef.current?.scrollTop ?? 0);
    if (project) void saveFolderState(project.id, "chat-files", { expanded: [...next], rootCollapsed: false }).catch(() => setError("Could not save folder state."));
  }

  function toggleDirectory(entry: ProjectDirectoryEntry) {
    const explorerID = project?.id ?? temporaryWorkspace?.id;
    if (!explorerID || loadedExplorerRef.current !== explorerID) return;
    const next = new Set(expandedDirectoriesRef.current);
    const isExpanded = next.delete(entry.path);
    if (!isExpanded) next.add(entry.path);
    updateExpandedDirectories(explorerID, next);
    if (isExpanded || entries[entry.path]) return;
    const fetchChildEntries = !project && temporaryWorkspace
        ? fetchHomeDirectoryEntries(entry.path)
        : project
          ? fetchProjectDirectoryEntries(project.id, entry.path)
          : null;
    if (!fetchChildEntries) return;
    void fetchChildEntries
      .then((childEntries) => {
        if (loadedExplorerRef.current === explorerID) setEntries((current) => ({ ...current, [entry.path]: childEntries }));
      })
      .catch(() => { if (loadedExplorerRef.current === explorerID) setError("Could not load this folder."); });
  }

  return (
    <aside
      aria-label="Right side panel"
      className="right-side-panel"
      id="right-side-panel"
      style={{ "--right-side-panel-bottom-inset": `${bottomInset}px` } as CSSProperties}
    >
      <SidePanelResizeHandle onWidthChange={onWidthChange} side="right" width={width} />
      <header aria-label="Explorer views" className="right-side-panel-head">
        <div aria-label="Explorer view" className="right-side-panel-view-actions" role="tablist">
          <button aria-controls="right-side-panel-files" aria-label="Files" aria-selected={visibleView === "files"} className={visibleView === "files" ? "is-active" : ""} onClick={() => setActiveView("files")} role="tab" title="Files" type="button">
            <NewDocumentIcon />
          </button>
          {isGitHistoryAvailable ? <button aria-controls="right-side-panel-history" aria-label="Git history" aria-selected={visibleView === "history"} className={visibleView === "history" ? "is-active" : ""} onClick={() => setActiveView("history")} role="tab" title="Git history" type="button">
            <GitHistoryIcon />
          </button> : null}
          <button aria-controls="right-side-panel-research" aria-label="Deep research" aria-selected={visibleView === "research"} className={visibleView === "research" ? "is-active" : ""} onClick={() => setActiveView("research")} role="tab" title="Deep research" type="button">
            <ResearchIcon />
          </button>
        </div>
      </header>
      {visibleView === "files" ? <>
        <FileBrowser query={query} onQueryChange={setQuery} disabled={!project && !temporaryWorkspace} label={!project && temporaryWorkspace ? `${temporaryWorkspace.name} files` : "Project files"} id="right-side-panel-files" filesRef={filesRef} style={{
          "--right-side-panel-scroll-shadow-opacity": scrollShadowOpacity,
          "--right-side-panel-bottom-scroll-shadow-opacity": bottomScrollShadowOpacity,
        } as CSSProperties}>
            {error ? <p className="right-side-panel-message" role="status">{error}</p> : null}
            {!error && (project || temporaryWorkspace) && !entries[""] ? <p className="right-side-panel-message">Loading files…</p> : null}
            {!error && !project && !temporaryWorkspace ? <p className="right-side-panel-message">No project open.</p> : null}
            {entries[""]?.length === 0 ? <p className="right-side-panel-message">This folder is empty.</p> : null}
            {nameFilter && entries[""] && !entries[""].some((entry) => entryMatchesFilter(entry, nameFilter, entries)) ? <p className="right-side-panel-message">No files match this search.</p> : null}
            <FileEntries projectID={project?.id} allowAddToChat rootPath={project?.path ?? temporaryWorkspace?.path} onOpenFile={project ? onOpenFile : undefined} fileStatus={fileBrowserStatuses(gitStatus.changes, gitStatus.staged)} depth={0} entries={entries} expandedDirectories={expandedDirectories} nameFilter={nameFilter} onToggleDirectory={toggleDirectory} parentPath="" />
        </FileBrowser>
      </> : visibleView === "history" ? <GitHistoryView allowAddToChat onOpenFile={onOpenFile} error={gitHistoryError} gitStatus={gitStatus} gitStatusError={gitStatusError} gitStatusLoading={gitStatusLoading} history={gitHistory} loading={gitHistoryLoading} project={project} /> : <section aria-label="Deep research" className="right-side-panel-research" id="right-side-panel-research" role="tabpanel">
        {!project ? <p className="right-side-panel-message">Open a project to view its deep research.</p> : null}
        {researchLoading ? <p className="right-side-panel-message">Loading deep research…</p> : null}
        {researchError ? <p className="right-side-panel-message" role="status">{researchError}</p> : null}
        {!researchLoading && !researchError && project && research.length === 0 ? <p className="right-side-panel-message">No deep research in this folder.</p> : null}
        {!researchLoading && !researchError ? research.map((job) => <button className={`right-side-panel-research-item is-${job.status || "unknown"}`} key={job.id} onClick={() => onOpenResearch(job)} onContextMenu={event => researchMenu.open(event, job)} onKeyDown={event => researchMenu.openWithKeyboard(event, job)} type="button">
          <ResearchIcon />
          <div>
            <h2>{job.title}</h2>
            <p>{researchMeta(job)}</p>
          </div>
          {job.status !== "done" && job.status !== "running" ? <span className={`right-side-panel-research-status is-${job.status || "unknown"}`}>{researchStatusLabel(job.status)}</span> : null}
        </button>) : null}
      </section>}
      {researchMenu.menu}
    </aside>
  );
}

function researchMeta(job: ProjectResearch) {
  const parts: string[] = [];
  if (job.status === "running") {
    if (typeof job.round === "number") {
      const maxRounds = typeof job.maxRounds === "number" && job.maxRounds > 0 ? `/${job.maxRounds}` : "";
      parts.push(`round ${job.round}${maxRounds}`);
    }
    if (job.phase) parts.push(job.phase);
  }
  parts.push(job.sourceCount ? `${job.sourceCount} sources` : "No sources");
  const date = job.finishedAt || job.startedAt;
  if (date) {
    const parsedDate = new Date(date);
    if (!Number.isNaN(parsedDate.getTime())) {
      parts.push(new Intl.DateTimeFormat(undefined, { day: "numeric", month: "short" }).format(parsedDate));
    }
  }
  return parts.join(" · ");
}

function researchStatusLabel(status: string) {
  if (status === "done") return "Done";
  if (status === "running") return "Running";
  if (status === "paused") return "Paused";
  if (status === "failed") return "Failed";
  if (status === "cancelled") return "Cancelled";
  return "Unknown";
}
