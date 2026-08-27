import { type CSSProperties, type PointerEvent as ReactPointerEvent, useEffect, useMemo, useRef, useState } from "react";
import { checkoutProjectBranch, fetchHomeDirectoryEntries, fetchProjectBranches, fetchProjectDirectoryEntries, fetchProjectGitHistory, fetchProjectGitStatus, fetchProjectResearch, PROJECT_GIT_BRANCH_CHANGED_EVENT, type Project, type ProjectDirectoryEntry, type ProjectGitHistory, type ProjectGitStatus, type ProjectResearch } from "../projects/projects";
import type { TemporaryWorkspace } from "../projects/temporaryWorkspace";
import { SidePanelResizeHandle } from "./SidePanelResizeHandle";

const EXPLORER_STATE_STORAGE_PREFIX = "solomon.explorer-state.v1";
const TEST_CHATS_EXPLORER_ID = "__test-chats__";
const TEST_CHATS_DIRECTORY_PATH = "__solomon_test_chats__";
const TEST_CHATS_ATTACHMENTS_PATH = `${TEST_CHATS_DIRECTORY_PATH}/attachments`;
const TEST_CHATS_FIXTURES_PATH = `${TEST_CHATS_DIRECTORY_PATH}/fixtures`;
const TEST_CHATS_CONVERSATIONS_PATH = `${TEST_CHATS_FIXTURES_PATH}/conversations`;
const TEST_CHATS_TOOL_RESULTS_PATH = `${TEST_CHATS_FIXTURES_PATH}/tool-results`;
const TEST_CHATS_SNAPSHOTS_PATH = `${TEST_CHATS_DIRECTORY_PATH}/snapshots`;
const EMPTY_GIT_HISTORY: ProjectGitHistory = { commits: [], current: "", isRepo: false };
const EMPTY_GIT_STATUS: ProjectGitStatus = { changes: {}, isRepo: false, staged: {} };
const TEST_CHATS_RESEARCH: ProjectResearch[] = [
  {
    finishedAt: "",
    id: "research-001",
    phase: "reading",
    sourceCount: 12,
    startedAt: "2026-08-27T14:42:00.000Z",
    status: "running",
    title: "Tool call UI patterns",
  },
  {
    finishedAt: "",
    id: "research-002",
    phase: "analyzing",
    sourceCount: 7,
    startedAt: "2026-08-25T09:20:00.000Z",
    status: "paused",
    title: "Async agent workflows",
  },
  {
    finishedAt: "2026-08-26T18:15:00.000Z",
    id: "research-003",
    phase: "writing",
    sourceCount: 24,
    startedAt: "2026-08-26T17:48:00.000Z",
    status: "done",
    title: "Background research UX for Solomon",
  },
  {
    finishedAt: "",
    id: "research-004",
    phase: "error",
    sourceCount: 3,
    startedAt: "2026-08-24T11:05:00.000Z",
    status: "failed",
    title: "Source extraction reliability",
  },
  {
    finishedAt: "",
    id: "research-005",
    phase: "searching",
    sourceCount: 5,
    startedAt: "2026-08-23T16:30:00.000Z",
    status: "cancelled",
    title: "Web research scope",
  },
];
const TEST_CHATS_ENTRIES: Record<string, ProjectDirectoryEntry[]> = {
  [TEST_CHATS_DIRECTORY_PATH]: [
    { isDirectory: true, name: "fixtures", path: TEST_CHATS_FIXTURES_PATH },
    { isDirectory: true, name: "attachments", path: TEST_CHATS_ATTACHMENTS_PATH },
    { isDirectory: true, name: "snapshots", path: TEST_CHATS_SNAPSHOTS_PATH },
    { isDirectory: false, name: "README.md", path: `${TEST_CHATS_DIRECTORY_PATH}/README.md` },
    { isDirectory: false, name: "test.config.json", path: `${TEST_CHATS_DIRECTORY_PATH}/test.config.json` },
  ],
  [TEST_CHATS_FIXTURES_PATH]: [
    { isDirectory: true, name: "conversations", path: TEST_CHATS_CONVERSATIONS_PATH },
    { isDirectory: true, name: "tool-results", path: TEST_CHATS_TOOL_RESULTS_PATH },
    { isDirectory: false, name: "empty-state.json", path: `${TEST_CHATS_FIXTURES_PATH}/empty-state.json` },
  ],
  [TEST_CHATS_CONVERSATIONS_PATH]: [
    { isDirectory: false, name: "assistant-stream.json", path: `${TEST_CHATS_CONVERSATIONS_PATH}/assistant-stream.json` },
    { isDirectory: false, name: "multi-turn.json", path: `${TEST_CHATS_CONVERSATIONS_PATH}/multi-turn.json` },
  ],
  [TEST_CHATS_TOOL_RESULTS_PATH]: [
    { isDirectory: false, name: "filesystem-response.json", path: `${TEST_CHATS_TOOL_RESULTS_PATH}/filesystem-response.json` },
    { isDirectory: false, name: "search-response.json", path: `${TEST_CHATS_TOOL_RESULTS_PATH}/search-response.json` },
  ],
  [TEST_CHATS_ATTACHMENTS_PATH]: [
    { isDirectory: false, name: "architecture-diagram.png", path: `${TEST_CHATS_ATTACHMENTS_PATH}/architecture-diagram.png` },
    { isDirectory: false, name: "release-notes.md", path: `${TEST_CHATS_ATTACHMENTS_PATH}/release-notes.md` },
  ],
  [TEST_CHATS_SNAPSHOTS_PATH]: [
    { isDirectory: false, name: "empty-composer.png", path: `${TEST_CHATS_SNAPSHOTS_PATH}/empty-composer.png` },
    { isDirectory: false, name: "tool-result.png", path: `${TEST_CHATS_SNAPSHOTS_PATH}/tool-result.png` },
  ],
};

export const testChatAtMentionEntries = Object.values(TEST_CHATS_ENTRIES)
  .flat()
  .map((entry) => ({
    isDirectory: entry.isDirectory,
    path: entry.path.replace(`${TEST_CHATS_DIRECTORY_PATH}/`, ""),
  }));

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
  onOpenResearch: (research: ProjectResearch) => void;
  project: Project | null;
  testChatsActive: boolean;
  temporaryWorkspace: TemporaryWorkspace | null;
  width: number;
};

export function RightSidePanel({ bottomInset, onOpenResearch, onWidthChange, project, testChatsActive, temporaryWorkspace, width }: RightSidePanelProps) {
  const [activeView, setActiveView] = useState<"files" | "history" | "research">("files");
  const [entries, setEntries] = useState<Record<string, ProjectDirectoryEntry[]>>({});
  const [expandedDirectories, setExpandedDirectories] = useState<Set<string>>(() => new Set());
  const [error, setError] = useState("");
  const [query, setQuery] = useState("");
  const [scrollShadowOpacity, setScrollShadowOpacity] = useState(0);
  const [bottomScrollShadowOpacity, setBottomScrollShadowOpacity] = useState(0);
  const [research, setResearch] = useState<ProjectResearch[]>([]);
  const [researchError, setResearchError] = useState("");
  const [researchLoading, setResearchLoading] = useState(false);
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
    if (testChatsActive) {
      setResearch(TEST_CHATS_RESEARCH);
      return;
    }
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
  }, [project, testChatsActive]);

  useEffect(() => {
    setGitHistory(EMPTY_GIT_HISTORY);
    setGitHistoryError("");
    setGitHistoryLoading(Boolean(project) && !testChatsActive);
    setGitHistoryProjectID(project && !testChatsActive ? project.id : "");
    setGitStatus(EMPTY_GIT_STATUS);
    setGitStatusError("");
    setGitStatusLoading(Boolean(project) && !testChatsActive);
    if (!project || testChatsActive) return;
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
  }, [project, testChatsActive]);

  useEffect(() => {
    setEntries({});
    setError("");
    setQuery("");
    setScrollShadowOpacity(0);
    setBottomScrollShadowOpacity(0);
    const explorerID = !project && temporaryWorkspace
      ? temporaryWorkspace.id
      : testChatsActive ? TEST_CHATS_EXPLORER_ID : project?.id;
    if (!explorerID) {
      setExpandedDirectories(new Set());
      restoredScrollPositionRef.current = null;
      return;
    }
    const restoredState = loadExplorerState(explorerID);
    const restoredDirectories = [...new Set(restoredState.expandedDirectories.filter((path) => path !== TEST_CHATS_DIRECTORY_PATH))]
      .sort((left, right) => left.split("/").length - right.split("/").length);
    setExpandedDirectories(new Set(restoredDirectories));
    restoredScrollPositionRef.current = { projectID: explorerID, scrollTop: restoredState.scrollTop };
    if (testChatsActive && !temporaryWorkspace && !project) {
      setEntries({ "": TEST_CHATS_ENTRIES[TEST_CHATS_DIRECTORY_PATH], ...TEST_CHATS_ENTRIES });
      return;
    }
    if (!project && !temporaryWorkspace) return;
    let cancelled = false;
    const fetchRootEntries = project
      ? fetchProjectDirectoryEntries(project.id)
      : fetchHomeDirectoryEntries(temporaryWorkspace!.path);
    void fetchRootEntries
      .then(async (rootEntries) => {
        const restoredEntries: Record<string, ProjectDirectoryEntry[]> = {
          "": rootEntries,
        };
        await Promise.all(restoredDirectories.map(async (path) => {
          try {
            restoredEntries[path] = project
              ? await fetchProjectDirectoryEntries(project.id, path)
              : await fetchHomeDirectoryEntries(path);
          } catch {
            // A folder may have been renamed or removed since the state was saved.
          }
        }));
        if (!cancelled) setEntries(restoredEntries);
      })
      .catch(() => {
        if (!cancelled) setError("Could not load the project files.");
      });
    return () => {
      cancelled = true;
    };
  }, [project, temporaryWorkspace, testChatsActive]);

  useEffect(() => {
    const files = filesRef.current;
    if (!files || !entries[""]) return;
    const restoredPosition = restoredScrollPositionRef.current;
    const explorerID = !project && temporaryWorkspace
      ? temporaryWorkspace.id
      : testChatsActive ? TEST_CHATS_EXPLORER_ID : project?.id;
    if (!explorerID) return;
    if (!restoredPosition || restoredPosition.projectID !== explorerID) return;
    const frame = window.requestAnimationFrame(() => {
      files.scrollTop = restoredPosition.scrollTop;
      restoredScrollPositionRef.current = null;
    });
    return () => window.cancelAnimationFrame(frame);
  }, [entries, expandedDirectories, project, temporaryWorkspace, testChatsActive]);

  useEffect(() => {
    const files = filesRef.current;
    if (!files) return;
    const explorerID = !project && temporaryWorkspace
      ? temporaryWorkspace.id
      : testChatsActive ? TEST_CHATS_EXPLORER_ID : project?.id;
    if (!explorerID) return;
    const updateScrollChrome = () => {
      setScrollShadowOpacity(Math.min(1, files.scrollTop / 18));
      const scrollableHeight = files.scrollHeight - files.clientHeight;
      setBottomScrollShadowOpacity(Math.min(1, Math.max(0, scrollableHeight - files.scrollTop) / 18));
      saveExplorerState(explorerID, expandedDirectories, files.scrollTop);
    };
    const resizeObserver = new ResizeObserver(updateScrollChrome);
    resizeObserver.observe(files);
    files.addEventListener("scroll", updateScrollChrome, { passive: true });
    updateScrollChrome();
    return () => {
      resizeObserver.disconnect();
      files.removeEventListener("scroll", updateScrollChrome);
    };
  }, [bottomInset, entries, expandedDirectories, nameFilter, project, temporaryWorkspace, testChatsActive]);

  function toggleDirectory(entry: ProjectDirectoryEntry) {
    const isExpanded = expandedDirectories.has(entry.path);
    const explorerID = !project && temporaryWorkspace
      ? temporaryWorkspace.id
      : testChatsActive ? TEST_CHATS_EXPLORER_ID : project?.id;
    if (!explorerID) return;
    setExpandedDirectories((current) => {
      const next = new Set(current);
      if (isExpanded) next.delete(entry.path);
      else next.add(entry.path);
      saveExplorerState(explorerID, next, filesRef.current?.scrollTop ?? 0);
      return next;
    });
    if (isExpanded || entries[entry.path]) return;
    const fetchChildEntries = !project && temporaryWorkspace
        ? fetchHomeDirectoryEntries(entry.path)
        : project
          ? fetchProjectDirectoryEntries(project.id, entry.path)
          : null;
    if (!fetchChildEntries) return;
    void fetchChildEntries
      .then((childEntries) => setEntries((current) => ({ ...current, [entry.path]: childEntries })))
      .catch(() => setError("Could not load this folder."));
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
        <label className="right-side-panel-search">
          <SearchIcon />
          <input
            aria-label="Filter files"
            disabled={!testChatsActive && !project && !temporaryWorkspace}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Filter files…"
            type="search"
            value={query}
          />
        </label>
        <div
          className="right-side-panel-files-shell"
          style={{
            "--right-side-panel-scroll-shadow-opacity": scrollShadowOpacity,
            "--right-side-panel-bottom-scroll-shadow-opacity": bottomScrollShadowOpacity,
          } as CSSProperties}
        >
          <nav aria-label={!project && temporaryWorkspace ? `${temporaryWorkspace.name} files` : testChatsActive ? "Test chats files" : "Project files"} className="right-side-panel-files" id="right-side-panel-files" ref={filesRef} role="tabpanel">
            {error ? <p className="right-side-panel-message" role="status">{error}</p> : null}
            {!error && (testChatsActive || project || temporaryWorkspace) && !entries[""] ? <p className="right-side-panel-message">Loading files…</p> : null}
            {!error && !testChatsActive && !project && !temporaryWorkspace ? <p className="right-side-panel-message">No project open.</p> : null}
            {entries[""]?.length === 0 ? <p className="right-side-panel-message">This folder is empty.</p> : null}
            {nameFilter && entries[""] && !entries[""].some((entry) => entryMatchesFilter(entry, nameFilter, entries)) ? <p className="right-side-panel-message">No files match this search.</p> : null}
            <FileEntries depth={0} entries={entries} expandedDirectories={expandedDirectories} nameFilter={nameFilter} onToggleDirectory={toggleDirectory} parentPath="" />
          </nav>
        </div>
      </> : visibleView === "history" ? <GitHistoryView error={gitHistoryError} gitStatus={gitStatus} gitStatusError={gitStatusError} gitStatusLoading={gitStatusLoading} history={gitHistory} loading={gitHistoryLoading} project={project} /> : <section aria-label="Deep research" className="right-side-panel-research" id="right-side-panel-research" role="tabpanel">
        {!project && !testChatsActive ? <p className="right-side-panel-message">Open a project to view its deep research.</p> : null}
        {researchLoading ? <p className="right-side-panel-message">Loading deep research…</p> : null}
        {researchError ? <p className="right-side-panel-message" role="status">{researchError}</p> : null}
        {!researchLoading && !researchError && !project && testChatsActive && research.length === 0 ? <p className="right-side-panel-message">No deep research in Test chats yet.</p> : null}
        {!researchLoading && !researchError && project && research.length === 0 ? <p className="right-side-panel-message">No deep research in this folder.</p> : null}
        {!researchLoading && !researchError ? research.map((job) => <button className={`right-side-panel-research-item is-${job.status || "unknown"}`} key={job.id} onClick={() => onOpenResearch(job)} type="button">
          <ResearchIcon />
          <div>
            <h2>{job.title}</h2>
            <p>{researchMeta(job)}</p>
          </div>
          {job.status !== "done" && job.status !== "running" ? <span className={`right-side-panel-research-status is-${job.status || "unknown"}`}>{researchStatusLabel(job.status)}</span> : null}
        </button>) : null}
      </section>}
    </aside>
  );
}

type FileEntriesProps = {
  depth: number;
  entries: Record<string, ProjectDirectoryEntry[]>;
  expandedDirectories: Set<string>;
  collapsedDirectories?: Set<string>;
  fileStatus?: Record<string, string>;
  folderStatus?: Record<string, string>;
  nameFilter: string;
  onOpenFile?: (entry: ProjectDirectoryEntry) => void;
  onToggleDirectory: (entry: ProjectDirectoryEntry) => void;
  parentPath: string;
  selectedPath?: string;
  iconMode?: "all" | "folders-chat" | "none";
};

function entryMatchesFilter(entry: ProjectDirectoryEntry, nameFilter: string, entries: Record<string, ProjectDirectoryEntry[]>): boolean {
  if (!nameFilter) return true;
  if (entry.name.toLowerCase().includes(nameFilter)) return true;
  if (!entry.isDirectory) return false;
  return (entries[entry.path] ?? []).some((child) => entryMatchesFilter(child, nameFilter, entries));
}

export function FileEntries({ collapsedDirectories, depth, entries, expandedDirectories, fileStatus, folderStatus, iconMode = "all", nameFilter, onOpenFile, onToggleDirectory, parentPath, selectedPath }: FileEntriesProps) {
  return entries[parentPath]?.filter((entry) => entryMatchesFilter(entry, nameFilter, entries)).map((entry) => {
    const hasMatchingChild = Boolean(nameFilter) && entry.isDirectory && (entries[entry.path] ?? []).some((child) => entryMatchesFilter(child, nameFilter, entries));
    const isExpanded = entry.isDirectory && (hasMatchingChild || (collapsedDirectories ? !collapsedDirectories.has(entry.path) : expandedDirectories.has(entry.path)));
    const status = entry.isDirectory ? folderStatus?.[entry.path] : fileStatus?.[entry.path];
    return (
      <div className="right-side-panel-file" key={entry.path}>
        <button
          aria-expanded={entry.isDirectory ? isExpanded : undefined}
          aria-current={!entry.isDirectory && entry.path === selectedPath ? "page" : undefined}
          className={`right-side-panel-file-row${!entry.isDirectory && entry.path === selectedPath ? " is-active" : ""}${status ? ` status-${status}` : ""}`}
          data-depth={depth}
          onClick={() => entry.isDirectory ? onToggleDirectory(entry) : onOpenFile?.(entry)}
          title={entry.name}
          type="button"
        >
          {iconMode === "all" ? (entry.isDirectory ? <FolderIcon name={entry.name} isOpen={isExpanded} /> : <FileIcon fileName={entry.name} />) : null}
          {iconMode === "folders-chat" && entry.isDirectory ? <ChatFolderIcon isOpen={isExpanded} /> : null}
          <span>{entry.name}</span>
          {entry.isDirectory && !isExpanded && status ? <i aria-label={`Folder status: ${gitStatusLabel(status)}`} className={`right-side-panel-folder-status status-${status}`} /> : null}
          {!entry.isDirectory && status ? <i aria-label={gitStatusLabel(status)} className={`right-side-panel-file-status status-${status}`}>{status}</i> : null}
        </button>
        {isExpanded ? (
          <div className="right-side-panel-file-children">
            {entries[entry.path] ? (
              <FileEntries collapsedDirectories={collapsedDirectories} depth={depth + 1} entries={entries} expandedDirectories={expandedDirectories} fileStatus={fileStatus} folderStatus={folderStatus} iconMode={iconMode} nameFilter={nameFilter} onOpenFile={onOpenFile} onToggleDirectory={onToggleDirectory} parentPath={entry.path} selectedPath={selectedPath} />
            ) : <span className="right-side-panel-loading">Loading…</span>}
          </div>
        ) : null}
      </div>
    );
  }) ?? null;
}

type GitHistoryViewProps = {
  error: string;
  gitStatus: ProjectGitStatus;
  gitStatusError: string;
  gitStatusLoading: boolean;
  history: ProjectGitHistory;
  loading: boolean;
  project: Project | null;
};

const gitHistoryLaneStep = 12;
const gitHistoryGraphInset = 7;
const gitHistoryContentInset = 6;
const emptyGitExpandedDirectories = new Set<string>();

function GitHistoryView({ error, gitStatus, gitStatusError, gitStatusLoading, history, loading, project }: GitHistoryViewProps) {
  const [branchError, setBranchError] = useState("");
  const [branches, setBranches] = useState<string[]>([]);
  const [branchesError, setBranchesError] = useState("");
  const [branchesLoading, setBranchesLoading] = useState(false);
  const [branchMenuOpen, setBranchMenuOpen] = useState(false);
  const [branchLoading, setBranchLoading] = useState(false);
  const [collapsedGitFolders, setCollapsedGitFolders] = useState<Set<string>>(() => new Set());
  const [stagedChangesCollapsed, setStagedChangesCollapsed] = useState(false);
  const [changesCollapsed, setChangesCollapsed] = useState(false);
  const [graphCollapsed, setGraphCollapsed] = useState(false);
  const [graphPanelHeight, setGraphPanelHeight] = useState(220);
  const [graphResizing, setGraphResizing] = useState(false);
  const [commitMessage, setCommitMessage] = useState("");
  const branchPickerRef = useRef<HTMLDivElement>(null);
  const [graphHistory, setGraphHistory] = useState(history);
  const commits = useMemo(() => gitHistoryLanes(graphHistory.commits), [graphHistory.commits]);
  const graphContentHeight = Math.max(22, commits.length * 22);
  const graphLaneCount = Math.max(1, ...commits.map((commit) => Math.max(commit.laneCount, commit.lane + 1)));
  const graphWidth = Math.max(64, 10 + graphLaneCount * gitHistoryLaneStep);
  const stagedEntries = useMemo(() => gitStatusEntries(gitStatus.staged), [gitStatus.staged]);
  const changedEntries = useMemo(() => gitStatusEntries(gitStatus.changes), [gitStatus.changes]);
  const stagedFolderStatus = useMemo(() => gitStatusFolderStatuses(gitStatus.staged), [gitStatus.staged]);
  const changedFolderStatus = useMemo(() => gitStatusFolderStatuses(gitStatus.changes), [gitStatus.changes]);
  const stagedCount = Object.keys(gitStatus.staged).length;
  const changedCount = Object.keys(gitStatus.changes).length;

  useEffect(() => {
    setGraphHistory(history);
  }, [history]);

  useEffect(() => {
    setBranches([]);
    setBranchesError("");
    setBranchMenuOpen(false);
    if (!project) return;
    let cancelled = false;
    setBranchesLoading(true);
    void fetchProjectBranches(project.id)
      .then((info) => {
        if (!cancelled) setBranches(info.branches);
      })
      .catch(() => {
        if (!cancelled) setBranchesError("Unable to load branches.");
      })
      .finally(() => {
        if (!cancelled) setBranchesLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [project]);

  useEffect(() => {
    if (!branchMenuOpen) return;
    const closeOnOutsidePointer = (event: PointerEvent) => {
      if (event.target instanceof Node && !branchPickerRef.current?.contains(event.target)) setBranchMenuOpen(false);
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setBranchMenuOpen(false);
    };
    document.addEventListener("pointerdown", closeOnOutsidePointer);
    document.addEventListener("keydown", closeOnEscape);
    return () => {
      document.removeEventListener("pointerdown", closeOnOutsidePointer);
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, [branchMenuOpen]);

  function toggleGitFolder(entry: ProjectDirectoryEntry) {
    setCollapsedGitFolders((current) => {
      const next = new Set(current);
      if (next.has(entry.path)) next.delete(entry.path);
      else next.add(entry.path);
      return next;
    });
  }

  function startGraphResize(event: ReactPointerEvent<HTMLButtonElement>) {
    event.preventDefault();
    const panel = event.currentTarget.parentElement?.parentElement;
    if (!panel) return;
    setGraphResizing(true);
    const startY = event.clientY;
    const startHeight = graphPanelHeight;
    const panelHeight = panel.getBoundingClientRect().height;
    const maxHeight = Math.max(140, panelHeight - 135);
    const resize = (moveEvent: PointerEvent) => setGraphPanelHeight(Math.min(maxHeight, Math.max(140, startHeight + startY - moveEvent.clientY)));
    const stop = () => {
      setGraphResizing(false);
      document.removeEventListener("pointermove", resize);
      document.removeEventListener("pointerup", stop);
      document.removeEventListener("pointercancel", stop);
    };
    document.addEventListener("pointermove", resize);
    document.addEventListener("pointerup", stop);
    document.addEventListener("pointercancel", stop);
  }

  async function selectBranch(branch: string) {
    if (!project || branchLoading) return;
    setBranchLoading(true);
    setBranchError("");
    try {
      const nextBranches = await checkoutProjectBranch(project.id, branch);
      setBranches(nextBranches.branches);
      const nextHistory = await fetchProjectGitHistory(project.id);
      setGraphHistory(nextHistory);
      setBranchMenuOpen(false);
      window.dispatchEvent(new CustomEvent(PROJECT_GIT_BRANCH_CHANGED_EVENT, { detail: { projectID: project.id } }));
    } catch {
      setBranchError(`Unable to refresh the graph for ${branch}.`);
    } finally {
      setBranchLoading(false);
    }
  }

  return (
    <section aria-label="Git history" className="right-side-panel-history" id="right-side-panel-history" role="tabpanel" style={{ "--right-side-panel-history-graph-height": `${graphCollapsed ? 29 : graphPanelHeight}px` } as CSSProperties}>
      <div className="right-side-panel-history-notices">
        {loading ? <p className="right-side-panel-message">Loading Git history…</p> : null}
        {gitStatusLoading ? <p className="right-side-panel-message">Loading Git changes…</p> : null}
        {error ? <p className="right-side-panel-message" role="status">{error}</p> : null}
        {gitStatusError ? <p className="right-side-panel-message" role="status">{gitStatusError}</p> : null}
        {branchError ? <p className="right-side-panel-message" role="status">{branchError}</p> : null}
      </div>
      <div aria-label="Create commit" className="right-side-panel-history-commit-form">
        <input aria-label="Commit message" onChange={(event) => setCommitMessage(event.target.value)} placeholder="Commit message" type="text" value={commitMessage} />
        <button disabled={!commitMessage.trim()} title="Commit staged changes" type="button">Commit</button>
      </div>
      <div aria-label="Changed files" className="right-side-panel-history-list">
        <section aria-label="Staged changes" className={`right-side-panel-history-change-section${stagedCount === 0 ? " is-empty" : stagedChangesCollapsed ? " is-collapsed" : ""}`}>
          <header className="right-side-panel-history-section-header">
            {stagedCount ? <button aria-expanded={!stagedChangesCollapsed} onClick={() => setStagedChangesCollapsed((collapsed) => !collapsed)} type="button"><HistoryChevronIcon open={!stagedChangesCollapsed} /><span>Staged Changes</span></button> : <span className="right-side-panel-history-section-label">Staged Changes</span>}
            <small>{stagedCount}</small>
          </header>
          {stagedCount > 0 && !stagedChangesCollapsed ? <div aria-label="Staged files" className="right-side-panel-history-tree">
            <FileEntries collapsedDirectories={collapsedGitFolders} depth={0} entries={stagedEntries} expandedDirectories={emptyGitExpandedDirectories} fileStatus={gitStatus.staged} folderStatus={stagedFolderStatus} nameFilter="" onToggleDirectory={toggleGitFolder} parentPath="" />
          </div> : null}
        </section>
        <section aria-label="Changes" className={`right-side-panel-history-change-section${changedCount === 0 ? " is-empty" : changesCollapsed ? " is-collapsed" : ""}`}>
          <header className="right-side-panel-history-section-header">
            {changedCount ? <button aria-expanded={!changesCollapsed} onClick={() => setChangesCollapsed((collapsed) => !collapsed)} type="button"><HistoryChevronIcon open={!changesCollapsed} /><span>Changes</span></button> : <span className="right-side-panel-history-section-label">Changes</span>}
            <small>{changedCount}</small>
          </header>
          {changedCount > 0 && !changesCollapsed ? <div aria-label="Changed files" className="right-side-panel-history-tree">
            <FileEntries collapsedDirectories={collapsedGitFolders} depth={0} entries={changedEntries} expandedDirectories={emptyGitExpandedDirectories} fileStatus={gitStatus.changes} folderStatus={changedFolderStatus} nameFilter="" onToggleDirectory={toggleGitFolder} parentPath="" />
          </div> : null}
        </section>
      </div>
      <section aria-label="Git graph" className={`right-side-panel-history-graph-panel${graphResizing ? " is-resizing" : ""}`}>
        {graphCollapsed ? null : <button aria-label="Resize Git graph" className="right-side-panel-history-graph-resize" onPointerDown={startGraphResize} title="Drag to resize Git graph" type="button" />}
        <header aria-label="Git graph" className="right-side-panel-history-section-header graph">
          <button aria-expanded={!graphCollapsed} onClick={() => setGraphCollapsed((collapsed) => !collapsed)} type="button"><HistoryChevronIcon open={!graphCollapsed} /><span>Graph</span></button>
          <div className="right-side-panel-history-branch-picker" ref={branchPickerRef}>
            <button aria-expanded={branchMenuOpen} aria-haspopup="listbox" className="right-side-panel-history-branch-trigger" disabled={branchLoading} onClick={() => setBranchMenuOpen((open) => !open)} type="button">
              <span>{graphHistory.current || "Detached HEAD"}</span>
            </button>
            {branchMenuOpen ? <div aria-label="Branches" className="right-side-panel-history-branch-menu" role="listbox">
              {branchesLoading ? <span className="right-side-panel-history-branch-menu-message">Loading branches…</span> : null}
              {!branchesLoading && branchesError ? <span className="right-side-panel-history-branch-menu-message">{branchesError}</span> : null}
              {!branchesLoading && !branchesError && branches.length === 0 ? <span className="right-side-panel-history-branch-menu-message">No local branches.</span> : null}
              {!branchesLoading && !branchesError ? branches.map((branch) => <button aria-selected={branch === history.current} className={`right-side-panel-history-branch-option${branch === history.current ? " is-current" : ""}`} key={branch} onClick={() => void selectBranch(branch)} role="option" type="button">
                <span>{branch}</span>
                {branch === history.current ? <i aria-hidden="true">✓</i> : null}
              </button>) : null}
            </div> : null}
          </div>
        </header>
        <section aria-label="Git graph entries" className="right-side-panel-history-graph">
          {!loading && !error && graphHistory.commits.length === 0 ? <p className="right-side-panel-history-empty">No commits in this repository yet.</p> : null}
          {!graphCollapsed && graphHistory.commits.length > 0 ? <ol style={{ "--right-side-panel-history-graph-width": `${graphWidth}px` } as CSSProperties}>
            <svg aria-hidden="true" className="right-side-panel-history-graph-canvas" viewBox={`0 0 ${graphWidth} ${graphContentHeight}`}>
              {commits.flatMap(({ hash, connections }, rowIndex) => connections.map(({ sourceLane, targetLane, colorLane, sourceRowOffset, targetRowOffset }, connectionIndex) => {
                const sourceX = gitHistoryGraphInset + sourceLane * gitHistoryLaneStep;
                const targetX = gitHistoryGraphInset + targetLane * gitHistoryLaneStep;
                const startY = (rowIndex + sourceRowOffset) * 22 + 11;
                const endY = (rowIndex + targetRowOffset) * 22 + 11;
                return <path d={gitHistoryConnectionPath(sourceX, targetX, startY, endY)} key={`${hash}-${connectionIndex}`} stroke={gitHistoryLaneColors[colorLane % gitHistoryLaneColors.length]} />;
              }))}
              {commits.map(({ hash, lane, colorLane, parents, refs }, rowIndex) => {
                const laneColor = gitHistoryLaneColors[colorLane % gitHistoryLaneColors.length];
                const isCurrent = refs.some((reference) => reference.includes("HEAD ->"));
                const x = gitHistoryGraphInset + lane * gitHistoryLaneStep;
                const y = rowIndex * 22 + 11;
                if (parents.length > 1) return <g key={hash}>
                  <circle cx={x} cy={y} r="5" fill="var(--right-side-panel-history-panel)" stroke={laneColor} strokeWidth="2" />
                  <circle cx={x} cy={y} r="2" fill={laneColor} />
                </g>;
                return <circle cx={x} cy={y} r="4" fill={isCurrent ? "var(--right-side-panel-history-panel)" : laneColor} key={hash} stroke={laneColor} strokeWidth="2" />;
              })}
            </svg>
            {commits.map((commit) => {
              const parsedReferences = commit.refs.map(gitHistoryReference).filter((reference) => !reference.hidden);
              const localBranchNames = new Set(parsedReferences.filter((reference) => reference.kind === "branch").map((reference) => reference.label));
              const references = parsedReferences.map((reference) => ({
                ...reference,
                isAligned: reference.kind === "remote" && localBranchNames.has(reference.label.replace(/^[^/]+\//, "")),
              }));
              return <li key={commit.hash} style={{ "--right-side-panel-history-content-offset": `${gitHistoryContentInset + commit.laneCount * gitHistoryLaneStep}px` } as CSSProperties} title={`${commit.shortHash} · ${commit.author || "Unknown author"} · ${gitCommitDate(commit.authoredAt)}`}>
                <span className="right-side-panel-history-commit">
                  <span className="right-side-panel-history-subject" title={commit.subject}>{commit.subject}</span>
                  <small className="right-side-panel-history-author" title={commit.author || "Unknown author"}>{commit.author || "Unknown author"}</small>
                </span>
                {references.length ? <small aria-label={`References: ${references.map((reference) => reference.label).join(", ")}`} className="right-side-panel-history-refs" title={references.map((reference) => reference.label).join(" · ")}>
                  {references.map((ref, refIndex) => {
                    const className = `right-side-panel-history-ref is-${ref.kind}${ref.isAligned ? " is-aligned" : ""}${ref.isCurrent ? " is-current" : ""}`;
                    if (ref.checkoutBranch && !ref.isCurrent) {
                      return <button aria-label={`Checkout ${ref.label}`} className={className} disabled={branchLoading} key={`${ref.label}-${refIndex}`} onClick={() => void selectBranch(ref.checkoutBranch!)} title={`Checkout ${ref.label}`} type="button">{ref.label}</button>;
                    }
                    return <span aria-label={ref.isAligned ? `${ref.label}, aligned with ${ref.label.replace(/^[^/]+\//, "")}` : undefined} className={className} key={`${ref.label}-${refIndex}`} title={ref.isCurrent ? `Current branch: ${ref.label}` : ref.isAligned ? `${ref.label} · aligned` : ref.label}>{ref.isAligned ? <CloudIcon /> : ref.label}</span>;
                  })}
                </small> : null}
              </li>;
            })}
          </ol> : null}
        </section>
      </section>
    </section>
  );
}

function gitStatusLabel(status: string) {
  if (status === "A" || status === "U") return "added";
  if (status === "D") return "deleted";
  if (status === "R") return "renamed";
  if (status === "C") return "copied";
  if (status === "M") return "modified";
  return "changed";
}

function gitStatusFolderStatuses(fileStatus: Record<string, string>) {
  const folderStatus: Record<string, string> = {};
  const priority: Record<string, number> = { R: 1, C: 1, A: 2, U: 2, M: 3, D: 4 };
  for (const [filePath, status] of Object.entries(fileStatus)) {
    const parts = filePath.split("/");
    parts.pop();
    let folderPath = "";
    for (const part of parts) {
      folderPath = folderPath ? `${folderPath}/${part}` : part;
      if ((priority[status] ?? 0) > (priority[folderStatus[folderPath]] ?? 0)) folderStatus[folderPath] = status;
    }
  }
  return folderStatus;
}

function gitStatusEntries(fileStatus: Record<string, string>) {
  const entries: Record<string, ProjectDirectoryEntry[]> = { "": [] };
  const addEntry = (parentPath: string, entry: ProjectDirectoryEntry) => {
    entries[parentPath] ??= [];
    if (!entries[parentPath].some((candidate) => candidate.path === entry.path)) entries[parentPath].push(entry);
  };
  for (const filePath of Object.keys(fileStatus).sort((left, right) => left.localeCompare(right))) {
    const normalizedPath = filePath.replaceAll("\\", "/").replace(/^\.\//, "");
    const parts = normalizedPath.split("/").filter(Boolean);
    const fileName = parts.pop();
    if (!fileName) continue;
    let parentPath = "";
    for (const part of parts) {
      const directoryPath = parentPath ? `${parentPath}/${part}` : part;
      addEntry(parentPath, { isDirectory: true, name: part, path: directoryPath });
      entries[directoryPath] ??= [];
      parentPath = directoryPath;
    }
    addEntry(parentPath, { isDirectory: false, name: fileName, path: normalizedPath });
  }
  for (const directoryEntries of Object.values(entries)) {
    directoryEntries.sort((left, right) => Number(right.isDirectory) - Number(left.isDirectory) || left.name.localeCompare(right.name));
  }
  return entries;
}

type GitHistoryLaneConnection = {
  colorLane: number;
  sourceLane: number;
  sourceRowOffset: -1 | 0;
  targetLane: number;
  targetRowOffset: 0 | 1;
};

type GitHistoryActiveLane = {
  colorLane: number;
  hash: string;
  skipNextConvergence?: boolean;
};

type GitHistoryReference = {
  checkoutBranch?: string;
  hidden?: boolean;
  isAligned?: boolean;
  isCurrent: boolean;
  kind: "branch" | "head" | "remote" | "tag";
  label: string;
};

type GitHistoryLaneCommit = ProjectGitHistory["commits"][number] & {
  colorLane: number;
  connections: GitHistoryLaneConnection[];
  lane: number;
  laneCount: number;
};

const gitHistoryLaneColors = ["#2991e8", "#f0b429", "#a78bfa", "#62c7a2", "#ee7e9f", "#e37150", "#6f9bf2", "#c58af9"];

function gitHistoryLanes(commits: ProjectGitHistory["commits"]): GitHistoryLaneCommit[] {
  let lanes: GitHistoryActiveLane[] = [];
  let nextColorLane = 0;
  const newLane = (hash: string, colorLane?: number): GitHistoryActiveLane => ({
    colorLane: colorLane ?? nextColorLane++,
    hash,
  });
  const connectionKeys = new Set<string>();

  return commits.map((commit, rowIndex) => {
    let lane = lanes.findIndex((activeLane) => activeLane.hash === commit.hash);
    if (lane < 0) {
      lane = 0;
      lanes.unshift(newLane(commit.hash));
    }
    const lanesBefore = [...lanes];
    const laneCount = lanes.length;
    const currentLane = lanes[lane];
    const parentLanes = commit.parents.map((parent, parentIndex) => lanes.find((activeLane) => activeLane.hash === parent)
      ?? newLane(parent, parentIndex === 0 ? currentLane.colorLane : undefined));
    const nextLanes = [...lanes];
    if (parentLanes.length > 0) {
      // Keep the first parent on the current lane. New side branches go
      // outside the lanes that were already open, so they never take the
      // place of an older inner branch.
      nextLanes.splice(lane, 1, parentLanes[0]);
      nextLanes.push(...parentLanes.slice(1));
    } else {
      nextLanes.splice(lane, 1);
    }
    const activeLanes = nextLanes
      .filter((activeLane) => activeLane.hash !== commit.hash)
      .filter((activeLane, index, allLanes) => allLanes.findIndex((candidate) => candidate.hash === activeLane.hash) === index);
    const connections: GitHistoryLaneConnection[] = [];
    const addConnection = (connection: GitHistoryLaneConnection) => {
      const key = `${connection.sourceLane}:${rowIndex + connection.sourceRowOffset}:${connection.targetLane}:${rowIndex + connection.targetRowOffset}`;
      if (connectionKeys.has(key)) return;
      connectionKeys.add(key);
      connections.push(connection);
    };
    const nextCommitHash = commits[rowIndex + 1]?.hash;
    const nextCommitHasDuplicateLanes = nextCommitHash
      ? activeLanes.filter((activeLane) => activeLane.hash === nextCommitHash).length > 1
      : false;
    const nextCommitLane = nextCommitHash ? activeLanes.findIndex((activeLane) => activeLane.hash === nextCommitHash) : -1;
    lanesBefore.forEach((activeLane, sourceLane) => {
      if (sourceLane === lane) {
        parentLanes.forEach((parentLane, parentIndex) => {
          const targetLane = activeLanes.indexOf(parentLane);
          if (targetLane >= 0) addConnection({ colorLane: parentIndex === 0 ? currentLane.colorLane : parentLane.colorLane, sourceLane, sourceRowOffset: 0, targetLane, targetRowOffset: 1 });
        });
        return;
      }
      if (activeLane.hash === commit.hash) {
        if (activeLane.skipNextConvergence) return;
        addConnection({ colorLane: activeLane.colorLane, sourceLane, sourceRowOffset: -1, targetLane: lane, targetRowOffset: 0 });
        return;
      }
      const targetLane = activeLanes.indexOf(activeLane);
      if (targetLane < 0) return;
      if (nextCommitHasDuplicateLanes && activeLane.hash === nextCommitHash && targetLane !== nextCommitLane) {
        if (sourceLane !== targetLane) {
          activeLane.skipNextConvergence = true;
          addConnection({ colorLane: activeLane.colorLane, sourceLane, sourceRowOffset: 0, targetLane: nextCommitLane, targetRowOffset: 1 });
        }
        return;
      }
      addConnection({ colorLane: activeLane.colorLane, sourceLane, sourceRowOffset: 0, targetLane, targetRowOffset: 1 });
    });
    lanes = activeLanes;
    return { ...commit, colorLane: currentLane.colorLane, connections, lane, laneCount };
  });
}

function gitHistoryConnectionPath(sourceX: number, targetX: number, startY: number, endY: number) {
  if (sourceX === targetX) return `M ${sourceX} ${startY} L ${targetX} ${endY}`;
  if (startY === endY) return `M ${sourceX} ${startY} L ${targetX} ${endY}`;

  const direction = targetX > sourceX ? 1 : -1;
  const bend = Math.min(Math.abs(targetX - sourceX) / 2, Math.abs(endY - startY) / 2, 8);
  return `M ${sourceX} ${startY} C ${sourceX + direction * bend} ${startY + bend}, ${targetX - direction * bend} ${endY - bend}, ${targetX} ${endY}`;
}

function gitHistoryReference(reference: string): GitHistoryReference {
  const normalized = reference.trim();
  const headBranch = normalized.match(/^HEAD -> (.+)$/);
  if (headBranch) {
    return { isCurrent: true, kind: "branch", label: headBranch[1] };
  }
  if (normalized.startsWith("tag: ")) {
    return { isCurrent: false, kind: "tag", label: normalized.slice(5).trim() };
  }
  if (normalized === "HEAD") {
    return { isCurrent: true, kind: "head", label: "HEAD" };
  }
  if (/^(?:origin|upstream|remotes)\//.test(normalized)) {
    if (/^(?:origin|upstream|remotes)\/HEAD(?: -> .+)?$/.test(normalized)) {
      return { hidden: true, isCurrent: false, kind: "remote", label: normalized };
    }
    return { isCurrent: false, kind: "remote", label: normalized };
  }
  return { checkoutBranch: normalized, isCurrent: false, kind: "branch", label: normalized };
}

function SearchIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 16 16">
      <circle cx="7" cy="7" r="4.5" />
      <path d="M10.5 10.5 14 14" />
    </svg>
  );
}

function NewDocumentIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="M6.25 3.5h8.25l5 5v10.25a2 2 0 0 1-2 2H6.25a2.5 2.5 0 0 1-2.5-2.5V6a2.5 2.5 0 0 1 2.5-2.5Z" />
      <path d="M14.5 3.5v3.25a1.75 1.75 0 0 0 1.75 1.75h3.25" />
    </svg>
  );
}

function GitHistoryIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="M8 5v14" />
      <path d="M8 8h3a4 4 0 0 1 4 4v1" />
      <circle cx="8" cy="5" r="2" />
      <circle cx="8" cy="19" r="2" />
      <circle cx="15" cy="15" r="2" />
    </svg>
  );
}

function CloudIcon() {
  return (
    <svg aria-hidden="true" className="right-side-panel-history-ref-icon" viewBox="0 0 24 24">
      <path d="M7.25 18.5h9.5a4.25 4.25 0 0 0 .5-8.47A6 6 0 0 0 5.7 8.6a3.9 3.9 0 0 0 1.55 7.9Z" />
    </svg>
  );
}

function HistoryChevronIcon({ open = true }: { open?: boolean }) {
  return (
    <svg aria-hidden="true" className={open ? "open" : ""} viewBox="0 0 24 24">
      <path d="m9 18 6-6-6-6" />
    </svg>
  );
}

function gitCommitDate(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "Unknown date";
  return new Intl.DateTimeFormat(undefined, { day: "numeric", month: "short", year: "numeric" }).format(date);
}

function ResearchIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="m4.5 12.2 8.7-5.1 3.2 5.4-8.8 5.1z" />
      <path d="m13.2 7.1 2.6-1.5 3.2 5.4-2.6 1.5M5.1 11.9l-1.5.9 1.3 2.2 1.5-.9" />
      <path d="M10.1 16.3 9.2 20M10.1 16.3l3.9 3.7M10.1 16.3 5.8 20" />
    </svg>
  );
}

function researchMeta(job: ProjectResearch) {
  const parts = [job.sourceCount ? `${job.sourceCount} sources` : "No sources"];
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

const specialFolderTypes: Record<string, string> = {
  ".android": "folder_type_android",
  ".anthropic": "folder_type_claude",
  ".cargo": "folder_type_cargo",
  ".claude": "folder_type_claude",
  ".copilot": "folder_type_github",
  ".copilot-chat": "folder_type_github",
  ".cursor": "folder_type_cursor",
  ".cursor-server": "folder_type_cursor",
  ".docker": "folder_type_docker",
  ".flutter": "folder_type_flutter",
  ".flutter-devtools": "folder_type_flutter",
  ".gemini": "folder_type_gemini",
  ".git": "folder_type_git",
  ".gradle": "folder_type_gradle",
  ".github": "folder_type_github",
  ".node": "folder_type_node",
  ".python": "folder_type_python",
  ".solomon": "folder_type_config",
  ".vscode": "folder_type_vscode",
  ".windsurf": "folder_type_windsurf",
  android: "folder_type_android",
  Applications: "folder_type_applications",
  "Applications (Parallels)": "folder_type_applications",
  anthropic: "folder_type_claude",
  components: "folder_type_library",
  docker: "folder_type_docker",
  docs: "folder_type_docs",
  Documents: "folder_type_docs",
  Downloads: "folder_type_downloads",
  flutter: "folder_type_flutter",
  Flutter: "folder_type_flutter",
  internal: "folder_type_src",
  Library: "folder_type_library",
  Movies: "folder_type_video",
  movies: "folder_type_video",
  Music: "folder_type_audio",
  music: "folder_type_audio",
  node_modules: "folder_type_node",
  Photos: "folder_type_images",
  photos: "folder_type_images",
  Pictures: "folder_type_images",
  pictures: "folder_type_images",
  Public: "folder_type_public",
  python: "folder_type_python",
  scripts: "folder_type_tools",
  src: "folder_type_src",
  test: "folder_type_test",
  tests: "folder_type_test",
  tools: "folder_type_tools",
  "ui-prototypes": "folder_type_library",
  Videos: "folder_type_video",
  videos: "folder_type_video",
};

const specialFolderBadges: Record<string, string> = {
  ".bun": "bun",
  ".cocoapods": "cocoapods",
  ".codex": "openai",
  ".conda": "anaconda",
  ".dart-tool": "dart",
  ".dartServer": "dart",
  ".eclipse": "eclipseide",
  ".homebrew": "homebrew",
  ".jupyter": "jupyter",
  ".jupiter": "jupyter",
  ".openai": "openai",
  ".opencode": "opencode",
  ".ollama": "ollama",
  ".npm": "npm",
  ".rustup": "rust",
  ".swiftpm": "swift",
  bun: "bun",
  cocoapods: "cocoapods",
  eclipse: "eclipseide",
  go: "go",
  homebrew: "homebrew",
  ipython: "jupyter",
  jupyter: "jupyter",
  jupiter: "jupyter",
  miniforge3: "anaconda",
  npm: "npm",
  ollama: "ollama",
  openai: "openai",
  opencode: "opencode",
  "oracle-cursor": "cursor",
  rustup: "rust",
  swiftpm: "swift",
};

const standaloneFolderIcons: Record<string, string> = {
  Desktop: "desktop",
};

function FolderIcon({ name, isOpen }: { name: string; isOpen: boolean }) {
  const standaloneIcon = standaloneFolderIcons[name];
  if (standaloneIcon) return <img aria-hidden="true" className="right-side-panel-file-icon" src={`/vscode-icons/${standaloneIcon}.svg`} />;
  const badge = specialFolderBadges[name];
  const folderType = specialFolderTypes[name] ?? "default_folder";
  const icon = <img aria-hidden="true" className="right-side-panel-folder-image" src={`/vscode-icons/${folderType}${isOpen ? "_opened" : ""}.svg`} />;
  if (!badge) return <span aria-hidden="true" className="right-side-panel-file-icon">{icon}</span>;
  return <span aria-hidden="true" className="right-side-panel-file-icon">{icon}<img className="right-side-panel-folder-badge" src={`/vscode-icons/${badge}.svg`} /></span>;
}

function ChatFolderIcon({ isOpen }: { isOpen: boolean }) {
  return (
    <svg aria-hidden="true" className="right-side-panel-file-icon right-side-panel-chat-folder-icon" viewBox="0 0 24 24">
      <path d={isOpen
        ? "m6 14 1.5-2.9A2 2 0 0 1 9.24 10H20a2 2 0 0 1 1.94 2.5l-1.54 6a2 2 0 0 1-1.95 1.5H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h3.9a2 2 0 0 1 1.69.9l.81 1.2a2 2 0 0 0 1.67.9H18a2 2 0 0 1 2 2v2"
        : "M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"}
      />
    </svg>
  );
}

const fileIconsByExtension: Record<string, string> = {
  "7z": "file_type_zip",
  aac: "file_type_audio",
  ai: "file_type_image",
  aif: "file_type_audio",
  aiff: "file_type_audio",
  avif: "file_type_image",
  avi: "file_type_video",
  bmp: "file_type_image",
  bz2: "file_type_zip",
  c: "c",
  cc: "cpp",
  cjs: "javascript",
  coffee: "javascript",
  cpp: "cpp",
  css: "css",
  cs: "csharp",
  csh: "shell",
  csx: "csharp",
  cts: "typescript",
  cxx: "cpp",
  csv: "file_type_excel",
  dart: "dart",
  db: "file_type_db",
  doc: "file_type_word",
  docx: "file_type_word",
  flac: "file_type_audio",
  gif: "file_type_image",
  gz: "file_type_zip",
  h: "c",
  htm: "html",
  html: "html",
  hpp: "cpp",
  hxx: "cpp",
  heic: "file_type_image",
  ico: "file_type_image",
  jpeg: "file_type_image",
  jpg: "file_type_image",
  java: "java",
  jl: "julia",
  js: "javascript",
  jsx: "javascript",
  key: "file_type_powerpoint",
  kt: "kotlin",
  kts: "kotlin",
  lua: "lua",
  m4a: "file_type_audio",
  m4v: "file_type_video",
  mkv: "file_type_video",
  mov: "file_type_video",
  mjs: "javascript",
  mts: "typescript",
  mp3: "file_type_mp3",
  mp4: "file_type_video",
  ods: "file_type_excel",
  odt: "file_type_word",
  ogg: "file_type_audio",
  pdf: "file_type_pdf",
  php: "php",
  phtml: "php",
  png: "file_type_image",
  ppt: "file_type_powerpoint",
  pptx: "file_type_powerpoint",
  psd: "file_type_image",
  py: "python",
  pyi: "python",
  pyw: "python",
  pyx: "python",
  rar: "file_type_zip",
  rake: "ruby",
  rb: "ruby",
  rs: "rust",
  sass: "css",
  scss: "css",
  sql: "file_type_db",
  sqlite: "file_type_sqlite",
  sqlite3: "file_type_sqlite",
  svg: "file_type_image",
  sh: "shell",
  swift: "swift",
  svelte: "svelte",
  ts: "typescript",
  tsx: "typescript",
  vue: "vue",
  xhtml: "html",
  tar: "file_type_zip",
  tiff: "file_type_image",
  txt: "file_type_text",
  wav: "file_type_audio",
  webm: "file_type_video",
  webp: "file_type_image",
  xls: "file_type_excel",
  xlsx: "file_type_excel",
  xml: "file_type_xml",
  xz: "file_type_zip",
  yaml: "file_type_yaml",
  yml: "file_type_yaml",
  zip: "file_type_zip",
};

const fileIconsByName: Record<string, string> = {
  ".gitignore": "git",
  "go.mod": "go_package",
  "go.sum": "go_package",
  gnumakefile: "makefile",
  licence: "license",
  "licence.md": "license",
  "licence.txt": "license",
  license: "license",
  "license.md": "license",
  "license.txt": "license",
  makefile: "makefile",
};

function FileIcon({ fileName }: { fileName: string }) {
  const normalizedFileName = fileName.toLowerCase();
  const extension = normalizedFileName.split(".").at(-1) ?? "";
  const icon = fileIconsByName[normalizedFileName]
    ?? (normalizedFileName.endsWith(".go") ? "go" : undefined)
    ?? (normalizedFileName.endsWith(".json") ? "json" : undefined)
    ?? (normalizedFileName.endsWith(".md") ? "markdown" : undefined)
    ?? (normalizedFileName.endsWith(".env") || normalizedFileName === ".env" ? "env" : undefined)
    ?? fileIconsByExtension[extension]
    ?? "file";
  return <img aria-hidden="true" className="right-side-panel-file-icon" src={`/vscode-icons/${icon}.svg`} />;
}
