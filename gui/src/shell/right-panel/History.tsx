import { type CSSProperties, type PointerEvent as ReactPointerEvent, useEffect, useMemo, useRef, useState } from "react";
import { fetchProjectGitHistory, PROJECT_GIT_BRANCH_CHANGED_EVENT, commitProjectChanges, checkoutProjectBranch, fetchProjectBranches, stageProjectChanges, unstageProjectChanges, type Project, type ProjectDirectoryEntry, type ProjectGitHistory, type ProjectGitStatus } from "../../projects/projects";
import { FileEntries } from "./Files";
import { IndexMark, CloudIcon, HistoryChevronIcon } from "./icons";
import { gitHistoryLanes, gitHistoryConnectionPath, gitHistoryReference, gitHistoryLaneColors, gitCommitDate } from "./graph";
import { pathsForGitEntry, gitStatusFolderStatuses, gitStatusEntries } from "./status";
type GitHistoryViewProps = {
  error: string;
  gitStatus: ProjectGitStatus;
  gitStatusError: string;
  gitStatusLoading: boolean;
  history: ProjectGitHistory;
  loading: boolean;
  project: Project | null;
  readOnly?: boolean;
  allowAddToChat?: boolean;
  onOpenFile?: (entry: ProjectDirectoryEntry) => void;
  onOpenFileInNewTab?: (entry: ProjectDirectoryEntry) => void;
};

const gitHistoryLaneStep = 12;
const gitHistoryGraphInset = 7;
const gitHistoryContentInset = 6;
const emptyGitExpandedDirectories = new Set<string>();

export function GitHistoryView({ error, gitStatus, gitStatusError, gitStatusLoading, history, loading, onOpenFile, onOpenFileInNewTab, project, allowAddToChat = false, readOnly = false }: GitHistoryViewProps) {
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
  const [commitError, setCommitError] = useState("");
  const [committing, setCommitting] = useState(false);
  const commitPending = useRef(false);
  const currentProjectID = useRef(project?.id);
  currentProjectID.current = project?.id;

  useEffect(() => {
    setCommitMessage("");
    setCommitError("");
  }, [project?.id]);
  const [stageError, setStageError] = useState("");
  const [staging, setStaging] = useState(false);
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
    if (!project || readOnly) return;
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
  }, [project, readOnly]);

  useEffect(() => {
    if (!branchMenuOpen || !project || readOnly) return;
    let cancelled = false;
    void fetchProjectBranches(project.id)
      .then((info) => { if (!cancelled) setBranches(info.branches); })
      .catch(() => { if (!cancelled) setBranchesError("Unable to load branches."); });
    return () => { cancelled = true; };
  }, [branchMenuOpen, project?.id, readOnly]);

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

  async function stagePaths(paths: string[], action: "stage" | "unstage" = "stage") {
    if (!project || readOnly || staging || commitPending.current || paths.length === 0) return;
    setStaging(true);
    setStageError("");
    try {
      if (action === "unstage") await unstageProjectChanges(project.id, paths);
      else await stageProjectChanges(project.id, paths);
    } catch {
      setStageError(action === "unstage" ? "Unable to unstage the selected changes." : "Unable to stage the selected changes.");
    } finally {
      setStaging(false);
    }
  }

  async function commitChanges() {
    if (!project || readOnly || commitPending.current || staging || branchLoading || gitStatusLoading || !stagedCount || !commitMessage.trim()) return;
    const projectID = project.id;
    commitPending.current = true;
    setCommitting(true);
    setCommitError("");
    try {
      await commitProjectChanges(projectID, commitMessage);
      if (currentProjectID.current !== projectID) return;
      setCommitMessage("");
      try {
        const nextHistory = await fetchProjectGitHistory(projectID);
        if (currentProjectID.current === projectID) setGraphHistory(nextHistory);
      } catch {
        if (currentProjectID.current === projectID) setCommitError("Commit created, but Git history could not be refreshed.");
      }
    } catch (error) {
      if (currentProjectID.current === projectID) setCommitError(error instanceof Error ? error.message : "Unable to commit staged changes.");
    } finally {
      commitPending.current = false;
      setCommitting(false);
    }
  }

  async function selectBranch(branch: string) {
    if (!project || readOnly || branchLoading || commitPending.current) return;
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
        {commitError ? <p className="right-side-panel-message" role="alert">{commitError}</p> : null}
        {stageError ? <p className="right-side-panel-message" role="status">{stageError}</p> : null}
      </div>
      {!readOnly ? <div aria-label="Create commit" className="right-side-panel-history-commit-form">
        <input aria-label="Commit message" disabled={committing} onChange={(event) => setCommitMessage(event.target.value)} placeholder="Commit message" type="text" value={commitMessage} />
        <button disabled={!project || !gitStatus.isRepo || gitStatusLoading || !stagedCount || !commitMessage.trim() || committing || staging || branchLoading} onClick={() => void commitChanges()} title="Commit staged changes" type="button">{committing ? "Committing…" : "Commit"}</button>
      </div> : null}
      <div aria-label="Changed files" className="right-side-panel-history-list">
        <section aria-label="Staged changes" className={`right-side-panel-history-change-section${stagedCount === 0 ? " is-empty" : stagedChangesCollapsed ? " is-collapsed" : ""}`}>
          <header className="right-side-panel-history-section-header">
            {stagedCount ? <button aria-expanded={!stagedChangesCollapsed} onClick={() => setStagedChangesCollapsed((collapsed) => !collapsed)} type="button"><HistoryChevronIcon open={!stagedChangesCollapsed} /><span>Staged Changes</span></button> : <span className="right-side-panel-history-section-label">Staged Changes</span>}
            <span className="right-side-panel-history-section-meta">
              <small>{stagedCount}</small>
              {!readOnly && stagedCount > 0 ? <button aria-label="Unstage all changes" className="right-side-panel-stage-all" disabled={staging || committing} onClick={() => void stagePaths(Object.keys(gitStatus.staged), "unstage")} title="Unstage all changes" type="button"><IndexMark kind="remove" /></button> : null}
            </span>
          </header>
          {stagedCount > 0 && !stagedChangesCollapsed ? <div aria-label="Staged files" className="right-side-panel-history-tree">
            <FileEntries projectID={project?.id} allowAddToChat={allowAddToChat} rootPath={project?.path} collapsedDirectories={collapsedGitFolders} depth={0} entries={stagedEntries} expandedDirectories={emptyGitExpandedDirectories} fileStatus={gitStatus.staged} folderStatus={stagedFolderStatus} nameFilter="" onOpenFile={onOpenFile} onOpenFileInNewTab={onOpenFileInNewTab} onUnstageEntry={readOnly ? undefined : (entry) => void stagePaths(pathsForGitEntry(entry, gitStatus.staged), "unstage")} onToggleDirectory={toggleGitFolder} parentPath="" />
          </div> : null}
        </section>
        <section aria-label="Changes" className={`right-side-panel-history-change-section${changedCount === 0 ? " is-empty" : changesCollapsed ? " is-collapsed" : ""}`}>
          <header className="right-side-panel-history-section-header">
            {changedCount ? <button aria-expanded={!changesCollapsed} onClick={() => setChangesCollapsed((collapsed) => !collapsed)} type="button"><HistoryChevronIcon open={!changesCollapsed} /><span>Changes</span></button> : <span className="right-side-panel-history-section-label">Changes</span>}
            <span className="right-side-panel-history-section-meta">
              <small>{changedCount}</small>
              {!readOnly && changedCount > 0 ? <button aria-label="Stage all changes" className="right-side-panel-stage-all" disabled={staging || committing} onClick={() => void stagePaths(Object.keys(gitStatus.changes))} title="Stage all changes" type="button"><IndexMark kind="add" /></button> : null}
            </span>
          </header>
          {changedCount > 0 && !changesCollapsed ? <div aria-label="Changed files" className="right-side-panel-history-tree">
            <FileEntries projectID={project?.id} allowAddToChat={allowAddToChat} rootPath={project?.path} collapsedDirectories={collapsedGitFolders} depth={0} entries={changedEntries} expandedDirectories={emptyGitExpandedDirectories} fileStatus={gitStatus.changes} folderStatus={changedFolderStatus} nameFilter="" onOpenFile={onOpenFile} onOpenFileInNewTab={onOpenFileInNewTab} onStageEntry={readOnly ? undefined : (entry) => void stagePaths(pathsForGitEntry(entry, gitStatus.changes))} onToggleDirectory={toggleGitFolder} parentPath="" />
          </div> : null}
        </section>
      </div>
      <section aria-label="Git graph" className={`right-side-panel-history-graph-panel${graphResizing ? " is-resizing" : ""}`}>
        {graphCollapsed ? null : <button aria-label="Resize Git graph" className="right-side-panel-history-graph-resize" onPointerDown={startGraphResize} title="Drag to resize Git graph" type="button" />}
        <header aria-label="Git graph" className="right-side-panel-history-section-header graph">
          <button aria-expanded={!graphCollapsed} onClick={() => setGraphCollapsed((collapsed) => !collapsed)} type="button"><HistoryChevronIcon open={!graphCollapsed} /><span>Graph</span></button>
          <div className="right-side-panel-history-branch-picker" ref={branchPickerRef}>
            <button aria-expanded={branchMenuOpen} aria-haspopup="listbox" className="right-side-panel-history-branch-trigger" disabled={readOnly || branchLoading || committing} onClick={() => setBranchMenuOpen((open) => !open)} type="button">
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
                      return <button aria-label={`Checkout ${ref.label}`} className={className} disabled={branchLoading || committing} key={`${ref.label}-${refIndex}`} onClick={() => void selectBranch(ref.checkoutBranch!)} title={`Checkout ${ref.label}`} type="button">{ref.label}</button>;
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
