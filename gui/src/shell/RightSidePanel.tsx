import { type CSSProperties, useEffect, useRef, useState } from "react";
import { fetchProjectDirectoryEntries, type Project, type ProjectDirectoryEntry } from "../projects/projects";
import { SidePanelResizeHandle } from "./SidePanelResizeHandle";

type RightSidePanelProps = {
  bottomInset: number;
  onContentWidthChange: (width: number) => void;
  onWidthChange: (width: number) => void;
  project: Project | null;
  width: number;
};

export function RightSidePanel({ bottomInset, onContentWidthChange, onWidthChange, project, width }: RightSidePanelProps) {
  const [entries, setEntries] = useState<Record<string, ProjectDirectoryEntry[]>>({});
  const [expandedDirectories, setExpandedDirectories] = useState<Set<string>>(() => new Set());
  const [error, setError] = useState("");
  const panelRef = useRef<HTMLElement>(null);
  const filesRef = useRef<HTMLElement>(null);
  const measureRef = useRef<HTMLSpanElement>(null);

  useEffect(() => {
    setEntries({});
    setExpandedDirectories(new Set());
    setError("");
    if (!project) return;
    let cancelled = false;
    void fetchProjectDirectoryEntries(project.id)
      .then((rootEntries) => {
        if (!cancelled) setEntries({ "": rootEntries });
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
    const panel = panelRef.current;
    const measure = measureRef.current;
    if (!files || !panel || !measure) return;

    const updateContentWidth = () => {
      const panelStyle = window.getComputedStyle(panel);
      const filesStyle = window.getComputedStyle(files);
      const sampleRow = files.querySelector<HTMLButtonElement>(".right-side-panel-file-row");
      const sampleIcon = sampleRow?.querySelector("svg");
      const sampleLabel = sampleRow?.querySelector("span");
      if (!sampleRow || !sampleIcon || !sampleLabel) {
        onContentWidthChange(240);
        return;
      }

      const rowStyle = window.getComputedStyle(sampleRow);
      const labelStyle = window.getComputedStyle(sampleLabel);
      const children = files.querySelector(".right-side-panel-file-children");
      const childrenStyle = children ? window.getComputedStyle(children) : null;
      const nestPerLevel = childrenStyle
        ? (Number.parseFloat(childrenStyle.marginLeft) || 0) + (Number.parseFloat(childrenStyle.paddingLeft) || 0)
        : 17;
      const iconWidth = sampleIcon.getBoundingClientRect().width || 15;
      const gap = Number.parseFloat(rowStyle.gap) || 0;
      const rowPad = (Number.parseFloat(rowStyle.paddingLeft) || 0) + (Number.parseFloat(rowStyle.paddingRight) || 0);
      const filesPad = (Number.parseFloat(filesStyle.paddingLeft) || 0) + (Number.parseFloat(filesStyle.paddingRight) || 0);
      const panelBorder = (Number.parseFloat(panelStyle.borderLeftWidth) || 0) + (Number.parseFloat(panelStyle.borderRightWidth) || 0);
      const scrollbarGap = files.offsetWidth - files.clientWidth;
      const scrollbar = files.scrollHeight > files.clientHeight ? Math.max(scrollbarGap, 10) : scrollbarGap;

      measure.style.fontFamily = labelStyle.fontFamily;
      measure.style.fontSize = labelStyle.fontSize;
      measure.style.fontWeight = labelStyle.fontWeight;
      measure.style.fontStyle = labelStyle.fontStyle;
      measure.style.letterSpacing = labelStyle.letterSpacing;
      measure.style.lineHeight = labelStyle.lineHeight;

      let longest = 240;
      for (const row of files.querySelectorAll<HTMLButtonElement>(".right-side-panel-file-row")) {
        const label = row.querySelector("span");
        if (!label) continue;
        const depth = Number(row.dataset.depth) || 0;
        measure.textContent = label.textContent ?? "";
        const labelWidth = Math.max(measure.offsetWidth, label.scrollWidth);
        longest = Math.max(
          longest,
          panelBorder + filesPad + depth * nestPerLevel + rowPad + iconWidth + gap + labelWidth + scrollbar,
        );
      }
      onContentWidthChange(Math.ceil(longest));
    };

    const frame = window.requestAnimationFrame(updateContentWidth);
    void document.fonts?.ready.then(updateContentWidth);
    const resizeObserver = new ResizeObserver(updateContentWidth);
    resizeObserver.observe(files);
    return () => {
      window.cancelAnimationFrame(frame);
      resizeObserver.disconnect();
    };
  }, [entries, expandedDirectories, onContentWidthChange, width]);

  function toggleDirectory(entry: ProjectDirectoryEntry) {
    if (!project) return;
    const isExpanded = expandedDirectories.has(entry.path);
    setExpandedDirectories((current) => {
      const next = new Set(current);
      if (isExpanded) next.delete(entry.path);
      else next.add(entry.path);
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
      ref={panelRef}
      style={{ "--right-side-panel-bottom-inset": `${bottomInset}px` } as CSSProperties}
    >
      <SidePanelResizeHandle onWidthChange={onWidthChange} side="right" width={width} />
      <header className="right-side-panel-head">
        <span>Explorer</span>
        {project ? <span className="right-side-panel-project" title={project.path}>{project.name}</span> : null}
      </header>
      <nav aria-label="Project files" className="right-side-panel-files" ref={filesRef}>
        {error ? <p className="right-side-panel-message" role="status">{error}</p> : null}
        {!error && !project ? <p className="right-side-panel-message">No project open.</p> : null}
        {project && !error && !entries[""] ? <p className="right-side-panel-message">Loading files…</p> : null}
        {entries[""]?.length === 0 ? <p className="right-side-panel-message">This folder is empty.</p> : null}
        <FileEntries depth={0} entries={entries} expandedDirectories={expandedDirectories} onToggleDirectory={toggleDirectory} parentPath="" />
      </nav>
      <span aria-hidden="true" className="right-side-panel-measure" ref={measureRef} />
    </aside>
  );
}

type FileEntriesProps = {
  depth: number;
  entries: Record<string, ProjectDirectoryEntry[]>;
  expandedDirectories: Set<string>;
  onToggleDirectory: (entry: ProjectDirectoryEntry) => void;
  parentPath: string;
};

function FileEntries({ depth, entries, expandedDirectories, onToggleDirectory, parentPath }: FileEntriesProps) {
  return entries[parentPath]?.map((entry) => {
    const isExpanded = entry.isDirectory && expandedDirectories.has(entry.path);
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
          {entry.isDirectory ? <FolderIcon isOpen={isExpanded} /> : <FileIcon />}
          <span>{entry.name}</span>
        </button>
        {isExpanded ? (
          <div className="right-side-panel-file-children">
            {entries[entry.path] ? (
              <FileEntries depth={depth + 1} entries={entries} expandedDirectories={expandedDirectories} onToggleDirectory={onToggleDirectory} parentPath={entry.path} />
            ) : <span className="right-side-panel-loading">Loading…</span>}
          </div>
        ) : null}
      </div>
    );
  }) ?? null;
}

function FolderIcon({ isOpen }: { isOpen: boolean }) {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d={isOpen
        ? "m5 10 1.5-2.5A2 2 0 0 1 8.2 6.5H20a2 2 0 0 1 1.94 2.5l-1.5 7.5a2 2 0 0 1-1.96 1.6H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4l1.5 2h8.5a2 2 0 0 1 2 2v1"
        : "M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-8L10.5 4H4a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2Z"}
      />
    </svg>
  );
}

function FileIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="M6 3h8l4 4v14H6zM14 3v5h5" />
    </svg>
  );
}
