import { type CSSProperties, useEffect, useRef, useState } from "react";
import { fetchProjectDirectoryEntries, type Project, type ProjectDirectoryEntry } from "../projects/projects";
import { SidePanelResizeHandle } from "./SidePanelResizeHandle";

const EXPLORER_STATE_STORAGE_PREFIX = "solomon.explorer-state.v1";

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
  project: Project | null;
  width: number;
};

export function RightSidePanel({ bottomInset, onWidthChange, project, width }: RightSidePanelProps) {
  const [entries, setEntries] = useState<Record<string, ProjectDirectoryEntry[]>>({});
  const [expandedDirectories, setExpandedDirectories] = useState<Set<string>>(() => new Set());
  const [error, setError] = useState("");
  const [query, setQuery] = useState("");
  const filesRef = useRef<HTMLElement>(null);
  const restoredScrollPositionRef = useRef<{ projectID: string; scrollTop: number } | null>(null);
  const nameFilter = query.trim().toLowerCase();

  useEffect(() => {
    setEntries({});
    setError("");
    setQuery("");
    if (!project) return;
    const restoredState = loadExplorerState(project.id);
    const restoredDirectories = [...new Set(restoredState.expandedDirectories)].sort((left, right) => left.split("/").length - right.split("/").length);
    setExpandedDirectories(new Set(restoredDirectories));
    restoredScrollPositionRef.current = { projectID: project.id, scrollTop: restoredState.scrollTop };
    let cancelled = false;
    void fetchProjectDirectoryEntries(project.id)
      .then(async (rootEntries) => {
        const restoredEntries: Record<string, ProjectDirectoryEntry[]> = { "": rootEntries };
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
  }, [project]);

  useEffect(() => {
    const files = filesRef.current;
    if (!files || !project || !entries[""]) return;
    const restoredPosition = restoredScrollPositionRef.current;
    if (!restoredPosition || restoredPosition.projectID !== project.id) return;
    const frame = window.requestAnimationFrame(() => {
      files.scrollTop = restoredPosition.scrollTop;
      restoredScrollPositionRef.current = null;
    });
    return () => window.cancelAnimationFrame(frame);
  }, [entries, expandedDirectories, project]);

  useEffect(() => {
    const files = filesRef.current;
    if (!files || !project) return;
    const saveScrollPosition = () => saveExplorerState(project.id, expandedDirectories, files.scrollTop);
    files.addEventListener("scroll", saveScrollPosition, { passive: true });
    return () => files.removeEventListener("scroll", saveScrollPosition);
  }, [expandedDirectories, project]);

  function toggleDirectory(entry: ProjectDirectoryEntry) {
    if (!project) return;
    const isExpanded = expandedDirectories.has(entry.path);
    setExpandedDirectories((current) => {
      const next = new Set(current);
      if (isExpanded) next.delete(entry.path);
      else next.add(entry.path);
      saveExplorerState(project.id, next, filesRef.current?.scrollTop ?? 0);
      return next;
    });
    if (isExpanded || entries[entry.path]) return;
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
      <header className="right-side-panel-head">
        <span>Explorer</span>
        {project ? <span className="right-side-panel-project" title={project.path}>{project.name}</span> : null}
      </header>
      <label className="right-side-panel-search">
        <SearchIcon />
        <input
          aria-label="Filter files"
          disabled={!project}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="Filter files…"
          type="search"
          value={query}
        />
      </label>
      <nav aria-label="Project files" className="right-side-panel-files" ref={filesRef}>
        {error ? <p className="right-side-panel-message" role="status">{error}</p> : null}
        {!error && !project ? <p className="right-side-panel-message">No project open.</p> : null}
        {project && !error && !entries[""] ? <p className="right-side-panel-message">Loading files…</p> : null}
        {entries[""]?.length === 0 ? <p className="right-side-panel-message">This folder is empty.</p> : null}
        {nameFilter && entries[""] && !entries[""].some((entry) => entryMatchesFilter(entry, nameFilter, entries)) ? (
          <p className="right-side-panel-message">No files match this search.</p>
        ) : null}
        <FileEntries depth={0} entries={entries} expandedDirectories={expandedDirectories} nameFilter={nameFilter} onToggleDirectory={toggleDirectory} parentPath="" />
      </nav>
    </aside>
  );
}

type FileEntriesProps = {
  depth: number;
  entries: Record<string, ProjectDirectoryEntry[]>;
  expandedDirectories: Set<string>;
  nameFilter: string;
  onToggleDirectory: (entry: ProjectDirectoryEntry) => void;
  parentPath: string;
};

function entryMatchesFilter(entry: ProjectDirectoryEntry, nameFilter: string, entries: Record<string, ProjectDirectoryEntry[]>): boolean {
  if (!nameFilter) return true;
  if (entry.name.toLowerCase().includes(nameFilter)) return true;
  if (!entry.isDirectory) return false;
  return (entries[entry.path] ?? []).some((child) => entryMatchesFilter(child, nameFilter, entries));
}

function FileEntries({ depth, entries, expandedDirectories, nameFilter, onToggleDirectory, parentPath }: FileEntriesProps) {
  return entries[parentPath]?.filter((entry) => entryMatchesFilter(entry, nameFilter, entries)).map((entry) => {
    const hasMatchingChild = Boolean(nameFilter) && entry.isDirectory && (entries[entry.path] ?? []).some((child) => entryMatchesFilter(child, nameFilter, entries));
    const isExpanded = entry.isDirectory && (hasMatchingChild || expandedDirectories.has(entry.path));
    return (
      <div className="right-side-panel-file" key={entry.path}>
        <button
          aria-expanded={entry.isDirectory ? isExpanded : undefined}
          className="right-side-panel-file-row"
          data-depth={depth}
          onClick={() => entry.isDirectory && onToggleDirectory(entry)}
          title={entry.name}
          type="button"
        >
          {entry.isDirectory ? <FolderIcon name={entry.name} isOpen={isExpanded} /> : <FileIcon fileName={entry.name} />}
          <span>{entry.name}</span>
        </button>
        {isExpanded ? (
          <div className="right-side-panel-file-children">
            {entries[entry.path] ? (
              <FileEntries depth={depth + 1} entries={entries} expandedDirectories={expandedDirectories} nameFilter={nameFilter} onToggleDirectory={onToggleDirectory} parentPath={entry.path} />
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
