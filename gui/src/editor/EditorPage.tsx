import { startSideResize } from "./resize";
import { CodeEditor } from "./CodeEditor";
import { ImagePreview } from "./ImagePreview";
import { isImageFile } from "./images";
import { fileIconName } from "../fileIcons";
import { type CSSProperties, useEffect, useMemo, useRef, useState } from "react";
import { copyTextFallback } from "../chat/chatClipboard";
import { AsciiBanner } from "../home/Welcome";
import { FileEntries, GitHistoryIcon, GitHistoryView, NewDocumentIcon, SearchIcon } from "../shell/RightSidePanel";
import { SidePanelToggle } from "../shell/SidePanelToggle";
import { CreateEntryActions } from "./CreateEntryActions";
import { checkoutProjectBranch, fetchProjectBranches, fetchProjectDirectoryEntries, fetchProjectFile, fetchProjectGitHistory, fetchProjectGitStatus, PROJECT_GIT_BRANCH_CHANGED_EVENT, PROJECT_GIT_STATUS_CHANGED_EVENT, sameProjectDirectoryEntries, sameProjectGitHistory, sameProjectGitStatus, type Project, type ProjectDirectoryEntry, type ProjectGitHistory, type ProjectGitStatus, PROJECT_FILES_CHANGED_EVENT, type ProjectFilesChanged } from "../projects/projects";

import { loadEditorSession, storeEditorSession, saveEditorFile, waitForEditorSaves, type OpenFile, type EditorSession } from "./editorSession";
const EMPTY_GIT: ProjectGitStatus = { changes: {}, isRepo: false, staged: {} };
const EMPTY_HISTORY: ProjectGitHistory = { commits: [], current: "", isRepo: false };

export function EditorPage({ bottomInset, onHome, project, initialFile, onInitialFileOpened }: { onInitialFileOpened?: () => void; initialFile?: ProjectDirectoryEntry | null; bottomInset: number; onHome: () => void; project: Project | null }) {
  const [restoredSession] = useState(() => loadEditorSession(project?.id));
  const [sideView, setSideView] = useState<"files" | "git">("files");
  const [sideCollapsed, setSideCollapsed] = useState(false);
  const [sideWidth, setSideWidth] = useState(248);
  const [sideResizing, setSideResizing] = useState(false);
  const [entries, setEntries] = useState<ProjectDirectoryEntry[]>([]);
  const [children, setChildren] = useState<Record<string, ProjectDirectoryEntry[]>>({});
  const childrenRef = useRef(children);
  childrenRef.current = children;
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set(restoredSession.expanded));
  const expandedRef = useRef(expanded);
  expandedRef.current = expanded;
  const [rootCollapsed, setRootCollapsed] = useState(restoredSession.rootCollapsed);
  const [selectedFolderPath, setSelectedFolderPath] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [files, setFiles] = useState<OpenFile[]>(restoredSession.files);
  const [activeTabId, setActiveTabId] = useState(restoredSession.activeTabId);
  const activePath = files.find((file) => file.id === activeTabId)?.path ?? "";
  const nextTabId = useRef(Math.max(-1, ...restoredSession.files.map((file) => Number(file.id.replace("editor-tab-", "")) || 0)) + 1);
  const pendingFileReads = useRef(new Map<string, Promise<string>>());
  const draggedTab = useRef<string | null>(null);
  const [git, setGit] = useState<ProjectGitStatus>(EMPTY_GIT);
  const [history, setHistory] = useState<ProjectGitHistory>(EMPTY_HISTORY);
  const [branch, setBranch] = useState("");
  const [branchOptions, setBranchOptions] = useState<string[]>([]);
  const [branchMenuOpen, setBranchMenuOpen] = useState(false);
  const branchControlRef = useRef<HTMLDivElement>(null);
  const [branchLoading, setBranchLoading] = useState(false);
  const [message, setMessage] = useState("");
  const [cursor, setCursor] = useState({ line: 1, column: 1 });
  const openFileTimer = useRef<number | undefined>(undefined);

  const viewsRef = useRef(restoredSession.views ?? {});
  const sessionRef = useRef<EditorSession>(restoredSession);
  sessionRef.current = { files, activeTabId, expanded: [...expanded], rootCollapsed, views: viewsRef.current };

  useEffect(() => {
    if (project) storeEditorSession(project.id, sessionRef.current);
  }, [project?.id, files, activeTabId, expanded, rootCollapsed]);

  useEffect(() => {
    if (!project) return;
    let cancelled = false;
    void Promise.all([fetchProjectDirectoryEntries(project.id), fetchProjectGitStatus(project.id), fetchProjectBranches(project.id), fetchProjectGitHistory(project.id)])
      .then(([nextEntries, status, branches, nextHistory]) => {
        if (cancelled) return;
        setEntries(nextEntries); setGit(status); setBranch(branches.current); setBranchOptions(branches.branches); setHistory(nextHistory);
      })
      .catch(() => { if (!cancelled) setMessage("Unable to read this folder."); });
    void Promise.all(restoredSession.expanded.map(async (path) => {
      try { return [path, await fetchProjectDirectoryEntries(project.id, path)] as const; }
      catch { return [path, []] as const; }
    })).then((loaded) => { if (!cancelled) setChildren(Object.fromEntries(loaded)); });
    void waitForEditorSaves(project.id).then(async () => {
      const loaded = new Map(await Promise.all(restoredSession.files.map(async (file) => {
        try { return [file.id, isImageFile(file.path) ? "" : await fetchProjectFile(project.id, file.path)] as const; }
        catch { return [file.id, null] as const; }
      })));
      if (cancelled) return;
      setFiles((current) => current.flatMap((file) => {
        if (!loaded.has(file.id)) return [file];
        const content = loaded.get(file.id);
        if (content === null || content === undefined) return file.content !== file.saved ? [file] : [];
        return [{ ...file, content: file.content !== file.saved ? file.content : content, saved: content }];
      }));
    });
    return () => { cancelled = true; };
  }, [project?.id]);

  useEffect(() => {
    if (!project) return;
    const flush = () => {
      storeEditorSession(project.id, sessionRef.current);
      for (const file of new Map(sessionRef.current.files.map((file) => [file.path, file])).values()) {
        if (file.content !== file.saved) void saveEditorFile(project.id, file.path, file.content).catch(() => undefined);
      }
    };
    const onVisibilityChange = () => { if (document.visibilityState === "hidden") flush(); };
    window.addEventListener("pagehide", flush);
    document.addEventListener("visibilitychange", onVisibilityChange);
    return () => {
      window.removeEventListener("pagehide", flush);
      document.removeEventListener("visibilitychange", onVisibilityChange);
      flush();
    };
  }, [project?.id]);

  useEffect(() => {
    if (!project || !files.some((file) => file.content !== file.saved)) return;
    const timer = window.setTimeout(() => {
      for (const file of new Map(sessionRef.current.files.map((file) => [file.path, file])).values()) {
        if (file.content !== file.saved) void saveFile(file.path, file.content);
      }
    }, 500);
    return () => window.clearTimeout(timer);
  }, [project?.id, files]);

  useEffect(() => () => window.clearTimeout(openFileTimer.current), []);

  useEffect(() => {
    if (!branchMenuOpen) return;
    const closeOnOutsideClick = (event: PointerEvent) => {
      if (event.target instanceof Node && !branchControlRef.current?.contains(event.target)) setBranchMenuOpen(false);
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      setBranchMenuOpen(false);
      branchControlRef.current?.querySelector("button")?.focus();
    };
    window.addEventListener("pointerdown", closeOnOutsideClick);
    window.addEventListener("keydown", closeOnEscape);
    return () => {
      window.removeEventListener("pointerdown", closeOnOutsideClick);
      window.removeEventListener("keydown", closeOnEscape);
    };
  }, [branchMenuOpen]);


  useEffect(() => {
    setSelectedFolderPath(null);
  }, [project?.id, activeTabId]);

  useEffect(() => {
    if (!project) return;
    const refreshBranchState = () => {
      void Promise.all([fetchProjectBranches(project.id), fetchProjectGitStatus(project.id), fetchProjectGitHistory(project.id)])
        .then(([nextBranches, nextStatus, nextHistory]) => {
          setBranch(nextBranches.current);
          setBranchOptions(nextBranches.branches);
          setGit(nextStatus);
          setHistory(nextHistory);
        })
        .catch(() => undefined);
    };
    window.addEventListener(PROJECT_GIT_BRANCH_CHANGED_EVENT, refreshBranchState);
    return () => window.removeEventListener(PROJECT_GIT_BRANCH_CHANGED_EVENT, refreshBranchState);
  }, [project?.id]);

  useEffect(() => {
    if (!project) return;
    let cancelled = false;
    const refresh = (event: Event) => {
      const change = (event as CustomEvent<ProjectFilesChanged>).detail;
      if (change.projectID !== project.id) return;
      const affected = (path: string) => path === change.path || path.startsWith(`${change.path}/`);
      setSelectedFolderPath((path) => {
        if (path === null || !affected(path)) return path;
        if (change.action === "delete") return null;
        if ((change.action === "rename" || change.action === "move") && change.destination) return change.destination + path.slice(change.path.length);
        return path;
      });
      if (change.action === "delete") {
        setFiles((current) => current.filter((file) => !affected(file.path)));
      } else if ((change.action === "rename" || change.action === "move") && change.destination) {
        setFiles((current) => current.map((file) => affected(file.path) ? { ...file, path: change.destination + file.path.slice(change.path.length) } : file));
      }
      void Promise.all([fetchProjectDirectoryEntries(project.id), fetchProjectGitStatus(project.id)]).then(([nextEntries, status]) => {
        if (!cancelled) { setEntries(nextEntries); setGit(status); }
      }).catch(() => { if (!cancelled) setMessage("Unable to refresh files."); });
      void Promise.all(Object.keys(childrenRef.current).map(async (path) => {
        const nextPath = affected(path) && change.destination && change.action !== "copy" ? change.destination + path.slice(change.path.length) : path;
        try { return [nextPath, await fetchProjectDirectoryEntries(project.id, nextPath)] as const; }
        catch { return [nextPath, []] as const; }
      })).then((loaded) => { if (!cancelled) setChildren(Object.fromEntries(loaded)); });
      setExpanded((current) => new Set([...current].filter((path) => !(change.action === "delete" && affected(path))).map((path) => affected(path) && change.destination && change.action !== "copy" ? change.destination + path.slice(change.path.length) : path)));
    };
    const refreshGitStatus = (event: Event) => {
      const detail = (event as CustomEvent<{ projectID?: string }>).detail;
      if (detail?.projectID && detail.projectID !== project.id) return;
      void fetchProjectGitStatus(project.id).then((status) => { if (!cancelled) setGit(status); }).catch(() => undefined);
    };
    window.addEventListener(PROJECT_FILES_CHANGED_EVENT, refresh);
    window.addEventListener(PROJECT_GIT_STATUS_CHANGED_EVENT, refreshGitStatus);
    return () => { cancelled = true; window.removeEventListener(PROJECT_FILES_CHANGED_EVENT, refresh); window.removeEventListener(PROJECT_GIT_STATUS_CHANGED_EVENT, refreshGitStatus); };
  }, [project?.id]);

  useEffect(() => {
    if (!project) return;
    let cancelled = false;
    let timer: number | undefined;
    let inFlight = false;
    const refresh = async () => {
      if (inFlight) return;
      inFlight = true;
      const paths = ["", ...expandedRef.current];
      const [listings, status, branches] = await Promise.all([
        Promise.all(paths.map(async (path) => {
          try { return [path, await fetchProjectDirectoryEntries(project.id, path)] as const; }
          catch { return null; }
        })),
        fetchProjectGitStatus(project.id).catch(() => null),
        fetchProjectBranches(project.id).catch(() => null),
      ]);
      if (!cancelled) {
        const root = listings.find((listing) => listing?.[0] === "");
        if (root) setEntries((current) => sameProjectDirectoryEntries(current, root[1]) ? current : root[1]);
        setChildren((current) => {
          let changed = false;
          const next = { ...current };
          for (const listing of listings) {
            if (!listing || listing[0] === "") continue;
            const [path, nextEntries] = listing;
            if (sameProjectDirectoryEntries(current[path], nextEntries)) continue;
            next[path] = nextEntries;
            changed = true;
          }
          return changed ? next : current;
        });
        if (status) setGit((current) => sameProjectGitStatus(current, status) ? current : status);
        if (branches) {
          setBranch((current) => current === branches.current ? current : branches.current);
          setBranchOptions((current) => current.length === branches.branches.length && current.every((value, index) => value === branches.branches[index]) ? current : branches.branches);
        }
      }
      inFlight = false;
      if (!cancelled) timer = window.setTimeout(() => void refresh(), 1200);
    };
    timer = window.setTimeout(() => void refresh(), 1200);
    return () => {
      cancelled = true;
      if (timer !== undefined) window.clearTimeout(timer);
    };
  }, [project?.id]);

  useEffect(() => {
    if (!project || sideView !== "git") return;
    let cancelled = false;
    let timer: number | undefined;
    const refresh = async () => {
      try {
        const nextHistory = await fetchProjectGitHistory(project.id);
        if (!cancelled) setHistory((current) => sameProjectGitHistory(current, nextHistory) ? current : nextHistory);
      } catch {
        // Keep the current history visible when the daemon is temporarily unavailable.
      }
      if (!cancelled) timer = window.setTimeout(() => void refresh(), 3500);
    };
    void refresh();
    return () => {
      cancelled = true;
      if (timer !== undefined) window.clearTimeout(timer);
    };
  }, [project?.id, sideView]);

  useEffect(() => {
    if (activeTabId && !files.some((file) => file.id === activeTabId)) setActiveTabId(files[0]?.id ?? "");
  }, [files, activeTabId]);

  async function toggleFolder(entry: ProjectDirectoryEntry) {
    window.clearTimeout(openFileTimer.current);
    setSelectedFolderPath(entry.path);
    if (expanded.has(entry.path)) { setExpanded((current) => without(current, entry.path)); return; }
    if (!children[entry.path] && project) {
      try { const loaded = await fetchProjectDirectoryEntries(project.id, entry.path); setChildren((current) => ({ ...current, [entry.path]: loaded })); }
      catch { setMessage(`Unable to open ${entry.name}.`); return; }
    }
    setExpanded((current) => new Set(current).add(entry.path));
  }

  async function revealCreatedEntry(entry: ProjectDirectoryEntry) {
    if (!project) return;
    setRootCollapsed(false);
    const parent = entry.path.split("/").slice(0, -1).join("/");
    try {
      const loaded = await fetchProjectDirectoryEntries(project.id, parent);
      if (parent) {
        setChildren((current) => ({ ...current, [parent]: loaded }));
        setExpanded((current) => new Set(current).add(parent));
      } else setEntries(loaded);
      if (entry.isDirectory) setSelectedFolderPath(entry.path);
      else { setSelectedFolderPath(null); await openFile(entry, "new"); }
    } catch { setMessage(`Created ${entry.name}, but could not refresh the explorer.`); }
  }

  async function openFile(entry: ProjectDirectoryEntry, mode: "replace" | "new" = "replace", isCurrent: () => boolean = () => true) {
    if (!project) return;
    const existing = files.find((file) => file.path === entry.path);
    if (existing) {
      setFiles((current) => mode === "new" ? [...current.filter((file) => file.id !== existing.id), existing] : current);
      setActiveTabId(existing.id);
      if (window.matchMedia("(max-width: 720px), (pointer: coarse) and (max-width: 1024px)").matches) setSideCollapsed(true);
      return;
    }
    setMessage("Opening file…");
    try {
      const requestKey = `${project.id}:${entry.path}`;
      let request = pendingFileReads.current.get(requestKey);
      if (!request) {
        request = (isImageFile(entry.path) ? Promise.resolve("") : fetchProjectFile(project.id, entry.path)).finally(() => pendingFileReads.current.delete(requestKey));
        pendingFileReads.current.set(requestKey, request);
      }
      const content = await request;
      if (!isCurrent()) return;
      const replacing = mode === "replace" ? sessionRef.current.files.find((file) => file.id === sessionRef.current.activeTabId) : undefined;
      if (replacing && replacing.content !== replacing.saved && !await saveFile(replacing.path, replacing.content)) return;
      if (replacing && sessionRef.current.files.find((file) => file.id === replacing.id)?.content !== replacing.content) return;
      const nextFile = { id: `editor-tab-${nextTabId.current++}`, path: entry.path, content, saved: content };
      setFiles((current) => {
        if (mode === "new" || !replacing) return [...current, nextFile];
        const activeIndex = current.findIndex((file) => file.id === replacing.id);
        if (activeIndex < 0) return [...current, nextFile];
        return current.map((file, index) => index === activeIndex ? nextFile : file);
      });
      setActiveTabId(nextFile.id);
      if (window.matchMedia("(max-width: 720px), (pointer: coarse) and (max-width: 1024px)").matches) setSideCollapsed(true);
      setMessage("");
    }
    catch { if (isCurrent()) setMessage(`Unable to open ${entry.path}.`); }
  }

  useEffect(() => {
    let cancelled = false;
    if (initialFile && project) void openFile(initialFile, "new", () => !cancelled).then(() => { if (!cancelled) onInitialFileOpened?.(); });
    return () => { cancelled = true; };
  }, [project?.id, initialFile]);

  function scheduleOpenFile(entry: ProjectDirectoryEntry) {
    setSelectedFolderPath(null);
    window.clearTimeout(openFileTimer.current);
    openFileTimer.current = window.setTimeout(() => void openFile(entry), 220);
  }

  function openFileInNewTab(entry: ProjectDirectoryEntry) {
    setSelectedFolderPath(null);
    window.clearTimeout(openFileTimer.current);
    void openFile(entry, "new");
  }

  async function closeFile(id: string) {
    const closing = files.find((file) => file.id === id);
    if (closing && closing.content !== closing.saved && !await saveFile(closing.path, closing.content)) return;
    if (closing && sessionRef.current.files.find((file) => file.id === id)?.content !== closing.content) return;
    const current = sessionRef.current.files;
    const index = current.findIndex((file) => file.id === id);
    const remaining = current.filter((file) => file.id !== id);
    setFiles(remaining);
    if (sessionRef.current.activeTabId === id) setActiveTabId(remaining[Math.min(index, remaining.length - 1)]?.id ?? "");
  }

  function duplicateFile(file: OpenFile) {
    const duplicate = { ...file, id: `editor-tab-${nextTabId.current++}` };
    setFiles((current) => {
      const index = current.findIndex((item) => item.id === file.id);
      return [...current.slice(0, index + 1), duplicate, ...current.slice(index + 1)];
    });
    setActiveTabId(duplicate.id);
  }

  function moveFile(targetId: string) {
    const sourceId = draggedTab.current;
    if (!sourceId || sourceId === targetId) return;
    setFiles((current) => {
      const source = current.find((file) => file.id === sourceId);
      if (!source) return current;
      const targetIndex = current.findIndex((file) => file.id === targetId);
      const next = current.filter((file) => file.id !== sourceId);
      next.splice(targetIndex, 0, source);
      return next;
    });
    draggedTab.current = null;
  }

  async function selectBranch(nextBranch: string) {
    if (!project || branchLoading || nextBranch === branch) return;
    setBranchLoading(true);
    setMessage("Switching branch…");
    try {
      const dirty = files.filter((file) => file.content !== file.saved);
      await Promise.all(dirty.map((file) => saveEditorFile(project.id, file.path, file.content)));
      if (dirty.some((file) => sessionRef.current.files.find((current) => current.id === file.id)?.content !== file.content)) throw new Error("Files changed while saving");
      setFiles((current) => current.map((file) => {
        const saved = dirty.find((savedFile) => savedFile.path === file.path);
        return saved ? { ...file, saved: saved.content } : file;
      }));
      const nextBranches = await checkoutProjectBranch(project.id, nextBranch);
      const [nextEntries, nextStatus, nextHistory] = await Promise.all([
        fetchProjectDirectoryEntries(project.id),
        fetchProjectGitStatus(project.id),
        fetchProjectGitHistory(project.id),
      ]);
      setBranch(nextBranches.current);
      setBranchOptions(nextBranches.branches);
      setEntries(nextEntries);
      setChildren({});
      setExpanded(new Set());
      setGit(nextStatus);
      setHistory(nextHistory);
      setBranchMenuOpen(false);
      setMessage("");
    } catch {
      setMessage(`Unable to switch to ${nextBranch}.`);
    } finally {
      setBranchLoading(false);
    }
  }

  async function saveFile(path: string, content: string): Promise<boolean> {
    if (!project) return false;
    setMessage("Saving…");
    try {
      await saveEditorFile(project.id, path, content);
      setFiles((current) => current.map((file) => file.path === path ? { ...file, saved: content } : file));
      setMessage("");
      return true;
    } catch {
      setMessage("Autosave failed. Your changes are kept in this editor; try saving again.");
      return false;
    }
  }

  const active = files.find((file) => file.id === activeTabId);
  const fileStatus = useMemo(() => ({ ...git.changes, ...git.staged }), [git]);
  const entryMap = useMemo(() => ({ "": entries, ...children }), [entries, children]);
  if (!project) return <>
    <SidePanelToggle isOpen={!sideCollapsed} onToggle={() => setSideCollapsed((current) => !current)} />
    <section className={`editor-page editor-empty${sideCollapsed ? " side-collapsed" : ""}`} style={{ bottom: bottomInset }}><div><AsciiBanner /><strong>Open a folder to use the editor</strong></div></section>
  </>;
  const workspaceName = displayWorkspaceName(project.name);
  const editorLayoutStyle = { "--editor-side-width": `${sideWidth}px` } as CSSProperties;

  return <>
    <SidePanelToggle isOpen={!sideCollapsed} onToggle={() => setSideCollapsed((current) => !current)} />
    {!sideCollapsed ? <button aria-label="Go to home" className="editor-top-wordmark" onClick={onHome} style={editorLayoutStyle} type="button">SOLOMON</button> : null}
    <div className={`editor-top-path${sideCollapsed ? " is-collapsed" : ""}`} style={editorLayoutStyle} title={project.path}><span>{abbreviateHomePath(project.path)}</span><button aria-label="Copy project path" title="Copy project path" onClick={() => { if (navigator.clipboard?.writeText) void navigator.clipboard.writeText(project.path).catch(() => copyTextFallback(project.path)); else copyTextFallback(project.path); }}><CopyIcon /></button></div>
    <section className={`editor-page${sideCollapsed ? " side-collapsed" : ""}`} style={{ ...editorLayoutStyle, bottom: bottomInset }}>
    <aside className={`editor-sidebar${sideResizing ? " is-resizing" : ""}`} id="side-panel">
      <header aria-label="Explorer views" className="right-side-panel-head editor-side-panel-head">
        <div aria-label="Explorer view" className="right-side-panel-view-actions" role="tablist">
          <button aria-controls="editor-files" aria-label="Files" aria-selected={sideView === "files"} className={sideView === "files" ? "is-active" : ""} onClick={() => setSideView("files")} role="tab" title="Files" type="button"><NewDocumentIcon /></button>
          <button aria-controls="editor-history" aria-label="Git history" aria-selected={sideView === "git"} className={sideView === "git" ? "is-active" : ""} onClick={() => setSideView("git")} role="tab" title="Git history" type="button"><GitHistoryIcon /></button>
        </div>
      </header>
      {sideView === "files" ? <>
        <label className="right-side-panel-search editor-side-panel-search">
          <SearchIcon />
          <input aria-label="Search files or folders" placeholder="Search files or folders" type="search" value={query} onChange={(event) => setQuery(event.target.value)} />
        </label>
        <header className="editor-root editor-folder-row">
          <button className="editor-root-identity" aria-expanded={!rootCollapsed} aria-label={`${workspaceName} folder`} onClick={() => setRootCollapsed((collapsed) => !collapsed)} title={workspaceName} type="button"><Chevron open={!rootCollapsed} /><strong>{workspaceName}</strong></button>
          <CreateEntryActions key={project.id} projectID={project.id} parentPath={selectedFolderPath ?? activePath.split("/").slice(0, -1).join("/")} onCreated={(entry) => void revealCreatedEntry(entry)} />
        </header>
        <div className="right-side-panel-files-shell editor-side-panel-files-shell">
          <nav aria-label={`${workspaceName} files`} className="right-side-panel-files editor-file-list" id="editor-files" role="tabpanel">
            {!rootCollapsed ? <FileEntries projectID={project.id} rootPath={project.path} depth={0} entries={entryMap} expandedDirectories={expanded} fileStatus={fileStatus} nameFilter={query.trim().toLowerCase()} onOpenFile={scheduleOpenFile} onOpenFileInNewTab={openFileInNewTab} onToggleDirectory={toggleFolder} parentPath="" selectedPath={selectedFolderPath ?? activePath} /> : null}
          </nav>
        </div>
      </> : <div className="editor-git-host"><GitHistoryView error="" gitStatus={git} gitStatusError="" gitStatusLoading={false} history={history} loading={false} onOpenFile={scheduleOpenFile} onOpenFileInNewTab={openFileInNewTab} project={project} /></div>}
      <button aria-label="Resize editor side panel" className="editor-resize" onDoubleClick={() => setSideWidth(248)} onPointerDown={(event) => startSideResize(event, sideWidth, setSideWidth, setSideResizing)} title="Drag to resize" type="button" />
    </aside>
    <main className="editor-workbench">
      {files.length ? <div className="editor-tabs-shell"><nav className="editor-tabs">{files.map((file) => { const state=statusLabel(fileStatus[file.path]); return <div className={`editor-tab status-${state}${file.id === activeTabId ? " active" : ""}`} key={file.id} onDragOver={(event) => { if (draggedTab.current) { event.preventDefault(); event.dataTransfer.dropEffect = "move"; } }} onDrop={(event) => { event.preventDefault(); moveFile(file.id); }}><button className="editor-tab-trigger" draggable title="Drag to reorder; double-click to duplicate" onDragStart={(event) => { draggedTab.current = file.id; event.dataTransfer.effectAllowed = "move"; event.dataTransfer.setData("application/x-solomon-editor-tab", file.id); }} onDragEnd={() => { draggedTab.current = null; }} onDoubleClick={() => duplicateFile(file)} onClick={() => setActiveTabId(file.id)}><FileIcon fileName={baseName(file.path)} /><span>{baseName(file.path)}</span>{file.content !== file.saved ? <i>M</i> : state !== "clean" ? <i>{state[0].toUpperCase()}</i> : null}</button><button className="editor-tab-close" aria-label={`Close ${baseName(file.path)}`} onClick={() => closeFile(file.id)}>×</button></div>; })}</nav></div> : null}
      {active ? <><div className="editor-breadcrumb"><FileIcon fileName={baseName(active.path)} />{active.path.split("/").map((part, index) => <span key={`${part}-${index}`}>{part}</span>)}<button aria-label="Save file" className="editor-save" disabled={isImageFile(active.path) || active.content === active.saved} onClick={() => void saveFile(active.path, active.content)} type="button">Save</button></div>{isImageFile(active.path) ? <ImagePreview key={`${project.id}:${active.path}`} projectID={project.id} path={active.path} /> : <CodeEditor key={active.id} viewState={viewsRef.current[active.id]} onViewState={(state) => { viewsRef.current[active.id] = state; if (project) storeEditorSession(project.id, sessionRef.current); }} file={active} onCursor={setCursor} onChange={(content) => setFiles((current) => current.map((file) => file.path === active.path ? { ...file, content } : file))} onSave={saveFile} />}</> : <div className="editor-welcome"><AsciiBanner /><strong>{workspaceName}</strong><span>Choose a file from the explorer</span></div>}
      {message ? <div className="editor-save-message">{message}</div> : null}
    </main>
    <footer className="editor-status">
      {git.isRepo ? <div className="editor-branch-control" ref={branchControlRef}>
        <button aria-expanded={branchMenuOpen} aria-haspopup="listbox" aria-controls="editor-branch-menu" aria-label={`Current branch: ${branch || "HEAD"}. Select branch`} className="editor-branch" disabled={branchLoading} onClick={() => setBranchMenuOpen((open) => !open)} type="button"><GitBranchIcon /><span>{branch || "HEAD"}</span><Chevron open={branchMenuOpen} /></button>
        {branchMenuOpen ? <div id="editor-branch-menu" aria-label="Branches" className="editor-branch-menu" role="listbox">
          {branchOptions.length ? branchOptions.map((option) => <button aria-selected={option === branch} key={option} onClick={() => void selectBranch(option)} role="option" type="button"><GitBranchIcon /><span>{option}</span>{option === branch ? <CheckIcon /> : null}</button>) : <span>No local branches.</span>}
        </div> : null}
      </div> : <span className="editor-status-branch">No repository</span>}
      <span className="editor-status-spacer"/>{active && isImageFile(active.path) ? <span>Image preview · Read only</span> : <><span>Ln {cursor.line}, Col {cursor.column}</span><span>Spaces: 2</span><span>UTF-8</span></>}
    </footer>
  </section></>;
}


const without=(set:Set<string>,value:string)=>{const next=new Set(set);next.delete(value);return next}; const baseName=(path:string)=>path.split(/[\\/]/).at(-1)??path; const statusLabel=(code="")=>code==="M"?"modified":code==="A"||code==="U"?"added":code==="D"?"deleted":code==="R"?"renamed":"clean";
function displayWorkspaceName(name:string){return name && name === name.toUpperCase() ? `${name[0]}${name.slice(1).toLowerCase()}` : name;}
function abbreviateHomePath(path:string){const normalized=path.replace(/\\/g,"/");const home=normalized.match(/^\/(?:Users|home)\/[^/]+(?=\/|$)/)?.[0];return home ? `~${normalized.slice(home.length)}` : path;}
function FileIcon({ fileName }: { fileName: string }) {
  return <img className="workspace-file-icon" src={`/vscode-icons/${fileIconName(fileName)}.svg`} alt="" />;
}
function GitBranchIcon(){return <svg viewBox="0 0 24 24"><circle cx="6" cy="5" r="2"/><circle cx="18" cy="7" r="2"/><circle cx="6" cy="19" r="2"/><path d="M6 7v10M8 11h5a5 5 0 005-4"/></svg>} function CheckIcon(){return <svg viewBox="0 0 24 24"><path d="m5 12 4 4L19 6"/></svg>} function CopyIcon(){return <svg viewBox="0 0 24 24"><rect x="9" y="9" width="11" height="11" rx="2"/><path d="M6 15H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v1"/></svg>} function Chevron({open}:{open:boolean}){return <svg className={open?"open":""} viewBox="0 0 24 24"><path d="m9 5 7 7-7 7"/></svg>}
