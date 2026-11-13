import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type { Project } from "../projects/projects";
import type { TemporaryWorkspace } from "../projects/temporaryWorkspace";

export function WorkspaceControl({
  onOpenNewProject,
  workspaceName,
  projects,
  homeProject,
  open,
  onOpenChange,
  onSelect,
  onSelectTemporaryWorkspace,
  temporaryWorkspace,
}: {
  onOpenNewProject?: () => void;
  workspaceName: string;
  projects: Project[];
  homeProject?: Project;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSelect: (project?: Project) => void;
  onSelectTemporaryWorkspace?: () => void;
  temporaryWorkspace?: TemporaryWorkspace | null;
}) {
  const [showAll, setShowAll] = useState(false);
  const [query, setQuery] = useState("");
  const listRef = useRef<HTMLDivElement>(null);
  const searchRef = useRef<HTMLDivElement>(null);
  const normalizedQuery = query.trim().toLowerCase();
  const matches = (name: string, path: string) => !normalizedQuery || name.toLowerCase().includes(normalizedQuery) || path.toLowerCase().includes(normalizedQuery);
  const visibleTemporaryWorkspace = temporaryWorkspace && matches(temporaryWorkspace.name, temporaryWorkspace.path) ? temporaryWorkspace : null;
  const controlRef = useRef<HTMLDivElement>(null);
  const recentProjects = [...projects]
    .sort((a, b) => latestProjectActivity(b) - latestProjectActivity(a))
    .filter((project) => matches(project.name, project.path))
    .slice(0, showAll || normalizedQuery ? undefined : 5);

  useLayoutEffect(() => {
    if (!open) return;
    setQuery("");
    setShowAll(false);
    const list = listRef.current;
    const search = searchRef.current;
    if (list && search) {
      list.scrollTop = search.offsetHeight;
    }
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const close = (event: PointerEvent) => {
      if (!controlRef.current?.contains(event.target as Node)) onOpenChange(false);
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") onOpenChange(false);
    };
    document.addEventListener("pointerdown", close);
    document.addEventListener("keydown", closeOnEscape);
    return () => {
      document.removeEventListener("pointerdown", close);
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, [onOpenChange, open]);

  const selectProject = (project?: Project) => {
    onSelect(project);
    onOpenChange(false);
  };

  return (
    <div className="welcome-workspace-control" ref={controlRef}>
      <button
        aria-expanded={open}
        aria-haspopup="menu"
        className="welcome-workspace"
        onClick={() => onOpenChange(!open)}
        type="button"
      >
        <FolderIcon />
        <span>{workspaceName}</span>
        <ChevronIcon className={open ? "is-open" : undefined} />
      </button>
      {open ? (
        <div aria-label="Project directory" className="welcome-workspace-menu" ref={listRef} role="menu">
          <div className="welcome-workspace-list">
            <div className="welcome-workspace-search" ref={searchRef}>
              <input aria-label="Search projects" placeholder="Search projects…" type="search" value={query} onChange={(event) => { setQuery(event.target.value); if (listRef.current) listRef.current.scrollTop = 0; }} onFocus={() => { if (listRef.current) listRef.current.scrollTop = 0; }} />
            </div>
            <div className="welcome-workspace-recents">
            {visibleTemporaryWorkspace ? (
              <button
                className="welcome-workspace-project is-temporary"
                onClick={() => {
                  onSelectTemporaryWorkspace?.();
                  onOpenChange(false);
                }}
                role="menuitem"
                title={visibleTemporaryWorkspace.displayPath}
                type="button"
              >
                <FolderIcon />
                <span>{visibleTemporaryWorkspace.name}</span>
              </button>
            ) : null}
            {recentProjects.length ? recentProjects.map((project) => (
              <button
                className="welcome-workspace-project"
                key={project.id}
                onClick={() => selectProject(project)}
                role="menuitem"
                title={project.path}
                type="button"
              >
                <FolderIcon />
                <span>{project.name}</span>
              </button>
            )) : <span className="welcome-workspace-empty">{normalizedQuery ? "No matching projects" : "No recent projects"}</span>}
            </div>
          </div>
          {!normalizedQuery && !showAll && projects.length > 5 ? <button
            className="welcome-workspace-show-more"
            onClick={() => setShowAll(true)}
            role="menuitem"
            type="button"
          >
            Show more
          </button> : null}
          <div aria-hidden="true" className="welcome-workspace-divider" role="separator" />
          <div className="welcome-workspace-actions">
            <button aria-current={workspaceName === "Home" ? "page" : undefined} onClick={() => selectProject(homeProject)} role="menuitem" type="button">
              <HomeIcon />
              <span>Home</span>
            </button>
            <button
              onClick={() => {
                onOpenChange(false);
                onOpenNewProject?.();
              }}
              role="menuitem"
              type="button"
            >
              <PlusIcon />
              <span>New Project</span>
            </button>
          </div>
        </div>
      ) : null}
    </div>
  );
}

function latestProjectActivity(project: Project): number {
  const latest = project.chats.reduce((mostRecent, chat) => Math.max(mostRecent, Date.parse(chat.lastMessageAt) || 0), 0);
  return latest;
}

export function FolderIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z" />
    </svg>
  );
}

function HomeIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="m3 10 9-7 9 7v10a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1Z" />
      <path d="M9 21v-7h6v7" />
    </svg>
  );
}

function PlusIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="M12 5v14M5 12h14" />
    </svg>
  );
}

export function ChevronIcon({ className }: { className?: string }) {
  return (
    <svg aria-hidden="true" className={`welcome-chevron${className ? ` ${className}` : ""}`} viewBox="0 0 24 24">
      <path d="m7 10 5 5 5-5" />
    </svg>
  );
}
