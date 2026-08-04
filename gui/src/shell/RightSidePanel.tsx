import { type CSSProperties, useEffect, useRef, useState } from "react";
import { fetchProjectDirectoryEntries, fetchProjectResearch, type Project, type ProjectDirectoryEntry, type ProjectResearch } from "../projects/projects";
import { SidePanelResizeHandle } from "./SidePanelResizeHandle";

const EXPLORER_STATE_STORAGE_PREFIX = "solomon.explorer-state.v1";
const TEST_CHATS_EXPLORER_ID = "__test-chats__";
const TEST_CHATS_DIRECTORY_PATH = "__solomon_test_chats__";
const TEST_CHATS_ATTACHMENTS_PATH = `${TEST_CHATS_DIRECTORY_PATH}/attachments`;
const TEST_CHATS_FIXTURES_PATH = `${TEST_CHATS_DIRECTORY_PATH}/fixtures`;
const TEST_CHATS_CONVERSATIONS_PATH = `${TEST_CHATS_FIXTURES_PATH}/conversations`;
const TEST_CHATS_TOOL_RESULTS_PATH = `${TEST_CHATS_FIXTURES_PATH}/tool-results`;
const TEST_CHATS_SNAPSHOTS_PATH = `${TEST_CHATS_DIRECTORY_PATH}/snapshots`;
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
  width: number;
};

export function RightSidePanel({ bottomInset, onOpenResearch, onWidthChange, project, testChatsActive, width }: RightSidePanelProps) {
  const [activeView, setActiveView] = useState<"files" | "research">("files");
  const [entries, setEntries] = useState<Record<string, ProjectDirectoryEntry[]>>({});
  const [expandedDirectories, setExpandedDirectories] = useState<Set<string>>(() => new Set());
  const [error, setError] = useState("");
  const [query, setQuery] = useState("");
  const [scrollShadowOpacity, setScrollShadowOpacity] = useState(0);
  const [bottomScrollShadowOpacity, setBottomScrollShadowOpacity] = useState(0);
  const [research, setResearch] = useState<ProjectResearch[]>([]);
  const [researchError, setResearchError] = useState("");
  const [researchLoading, setResearchLoading] = useState(false);
  const filesRef = useRef<HTMLElement>(null);
  const restoredScrollPositionRef = useRef<{ projectID: string; scrollTop: number } | null>(null);
  const nameFilter = query.trim().toLowerCase();

  useEffect(() => {
    setResearch([]);
    setResearchError("");
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
    setEntries({});
    setError("");
    setQuery("");
    setScrollShadowOpacity(0);
    setBottomScrollShadowOpacity(0);
    const explorerID = testChatsActive ? TEST_CHATS_EXPLORER_ID : project?.id;
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
    if (testChatsActive) {
      setEntries({ "": TEST_CHATS_ENTRIES[TEST_CHATS_DIRECTORY_PATH], ...TEST_CHATS_ENTRIES });
      return;
    }
    if (!project) return;
    let cancelled = false;
    void fetchProjectDirectoryEntries(project.id)
      .then(async (rootEntries) => {
        const restoredEntries: Record<string, ProjectDirectoryEntry[]> = {
          "": rootEntries,
        };
        await Promise.all(restoredDirectories.map(async (path) => {
          try {
            restoredEntries[path] = await fetchProjectDirectoryEntries(project.id, path);
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
  }, [project, testChatsActive]);

  useEffect(() => {
    const files = filesRef.current;
    if (!files || !entries[""]) return;
    const restoredPosition = restoredScrollPositionRef.current;
    const explorerID = testChatsActive ? TEST_CHATS_EXPLORER_ID : project?.id;
    if (!explorerID) return;
    if (!restoredPosition || restoredPosition.projectID !== explorerID) return;
    const frame = window.requestAnimationFrame(() => {
      files.scrollTop = restoredPosition.scrollTop;
      restoredScrollPositionRef.current = null;
    });
    return () => window.cancelAnimationFrame(frame);
  }, [entries, expandedDirectories, project, testChatsActive]);

  useEffect(() => {
    const files = filesRef.current;
    if (!files) return;
    const explorerID = testChatsActive ? TEST_CHATS_EXPLORER_ID : project?.id;
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
  }, [bottomInset, entries, expandedDirectories, nameFilter, project, testChatsActive]);

  function toggleDirectory(entry: ProjectDirectoryEntry) {
    const isExpanded = expandedDirectories.has(entry.path);
    const explorerID = testChatsActive ? TEST_CHATS_EXPLORER_ID : project?.id;
    if (!explorerID) return;
    setExpandedDirectories((current) => {
      const next = new Set(current);
      if (isExpanded) next.delete(entry.path);
      else next.add(entry.path);
      saveExplorerState(explorerID, next, filesRef.current?.scrollTop ?? 0);
      return next;
    });
    if (isExpanded || entries[entry.path]) return;
    if (!project) return;
    void fetchProjectDirectoryEntries(project.id, entry.path)
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
          <button aria-controls="right-side-panel-files" aria-label="Files" aria-selected={activeView === "files"} className={activeView === "files" ? "is-active" : ""} onClick={() => setActiveView("files")} role="tab" title="Files" type="button">
            <NewDocumentIcon />
          </button>
          <button aria-controls="right-side-panel-research" aria-label="Deep research" aria-selected={activeView === "research"} className={activeView === "research" ? "is-active" : ""} onClick={() => setActiveView("research")} role="tab" title="Deep research" type="button">
            <ResearchIcon />
          </button>
        </div>
      </header>
      {activeView === "files" ? <>
        <label className="right-side-panel-search">
          <SearchIcon />
          <input
            aria-label="Filter files"
            disabled={!testChatsActive && !project}
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
          <nav aria-label={testChatsActive ? "Test chats files" : "Project files"} className="right-side-panel-files" id="right-side-panel-files" ref={filesRef} role="tabpanel">
            {error ? <p className="right-side-panel-message" role="status">{error}</p> : null}
            {!error && (testChatsActive || project) && !entries[""] ? <p className="right-side-panel-message">Loading files…</p> : null}
            {!error && !testChatsActive && !project ? <p className="right-side-panel-message">No project open.</p> : null}
            {entries[""]?.length === 0 ? <p className="right-side-panel-message">This folder is empty.</p> : null}
            {nameFilter && entries[""] && !entries[""].some((entry) => entryMatchesFilter(entry, nameFilter, entries)) ? <p className="right-side-panel-message">No files match this search.</p> : null}
            <FileEntries depth={0} entries={entries} expandedDirectories={expandedDirectories} nameFilter={nameFilter} onToggleDirectory={toggleDirectory} parentPath="" />
          </nav>
        </div>
      </> : <section aria-label="Deep research" className="right-side-panel-research" id="right-side-panel-research" role="tabpanel">
        {!project && testChatsActive ? <p className="right-side-panel-message">No deep research in Test chats yet.</p> : null}
        {!project && !testChatsActive ? <p className="right-side-panel-message">Open a project to view its deep research.</p> : null}
        {researchLoading ? <p className="right-side-panel-message">Loading deep research…</p> : null}
        {researchError ? <p className="right-side-panel-message" role="status">{researchError}</p> : null}
        {!researchLoading && !researchError && project && research.length === 0 ? <p className="right-side-panel-message">No deep research in this folder.</p> : null}
        {!researchLoading && !researchError ? research.map((job) => <button className="right-side-panel-research-item" key={job.id} onClick={() => onOpenResearch(job)} type="button">
          <ResearchIcon />
          <div>
            <h2>{job.title}</h2>
            <p>{researchMeta(job)}</p>
          </div>
          <span className={`right-side-panel-research-status is-${job.status || "unknown"}`}>{researchStatusLabel(job.status)}</span>
        </button>) : null}
      </section>}
    </aside>
  );
}

type FileEntriesProps = {
  depth: number;
  entries: Record<string, ProjectDirectoryEntry[]>;
  expandedDirectories: Set<string>;
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

export function FileEntries({ depth, entries, expandedDirectories, iconMode = "all", nameFilter, onOpenFile, onToggleDirectory, parentPath, selectedPath }: FileEntriesProps) {
  return entries[parentPath]?.filter((entry) => entryMatchesFilter(entry, nameFilter, entries)).map((entry) => {
    const hasMatchingChild = Boolean(nameFilter) && entry.isDirectory && (entries[entry.path] ?? []).some((child) => entryMatchesFilter(child, nameFilter, entries));
    const isExpanded = entry.isDirectory && (hasMatchingChild || expandedDirectories.has(entry.path));
    return (
      <div className="right-side-panel-file" key={entry.path}>
        <button
          aria-expanded={entry.isDirectory ? isExpanded : undefined}
          aria-current={!entry.isDirectory && entry.path === selectedPath ? "page" : undefined}
          className={`right-side-panel-file-row${!entry.isDirectory && entry.path === selectedPath ? " is-active" : ""}`}
          data-depth={depth}
          onClick={() => entry.isDirectory ? onToggleDirectory(entry) : onOpenFile?.(entry)}
          title={entry.name}
          type="button"
        >
          {iconMode === "all" ? (entry.isDirectory ? <FolderIcon name={entry.name} isOpen={isExpanded} /> : <FileIcon fileName={entry.name} />) : null}
          {iconMode === "folders-chat" && entry.isDirectory ? <ChatFolderIcon isOpen={isExpanded} /> : null}
          <span>{entry.name}</span>
        </button>
        {isExpanded ? (
          <div className="right-side-panel-file-children">
            {entries[entry.path] ? (
              <FileEntries depth={depth + 1} entries={entries} expandedDirectories={expandedDirectories} iconMode={iconMode} nameFilter={nameFilter} onOpenFile={onOpenFile} onToggleDirectory={onToggleDirectory} parentPath={entry.path} selectedPath={selectedPath} />
            ) : <span className="right-side-panel-loading">Loading…</span>}
          </div>
        ) : null}
      </div>
    );
  }) ?? null;
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
