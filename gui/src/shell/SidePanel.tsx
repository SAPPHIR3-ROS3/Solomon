import { type CSSProperties, type FormEvent, type KeyboardEvent, type MouseEvent, type PointerEvent as ReactPointerEvent, useEffect, useRef, useState } from "react";
import { fetchProjectRemovalInfo, fetchProjectSidebarData, type Project, type ProjectRemovalInfo, removeProjectFromDisk, removeProjectFromSidebar, saveUserName } from "../projects/projects";
import { initialFakeChats } from "../chat-test/fakeChats";
import { SidePanelResizeHandle } from "./SidePanelResizeHandle";

const INITIAL_CHAT_LIMIT = 5;
const MIN_SCROLL_THUMB_HEIGHT = 28;
const PROJECT_CONTEXT_MENU_HEIGHT = 158;
const PROJECT_CONTEXT_MENU_WIDTH = 200;
const PROJECT_CONTEXT_MENU_EDGE_GAP = 8;

type ProjectContextMenu = {
  project: Project;
  x: number;
  y: number;
};

type ProjectRemovalDialog = {
  project: Project;
  removeData: boolean;
};

type SidePanelProps = {
  armedTerminalProjectIds: string[];
  bottomInset: number;
  isCustomizationOpen: boolean;
  onNewProjectChat: (project: Project) => void;
  onOpenFakeFolder: () => void;
  onOpenFakeChat: (chatID: string) => void;
  onOpenProjectTerminal: (project: Project) => void;
  onToggleCustomization: () => void;
  onWidthChange: (width: number) => void;
  runningTerminalProjectIds: string[];
  width: number;
};

export function SidePanel({
  armedTerminalProjectIds,
  bottomInset,
  isCustomizationOpen,
  onNewProjectChat,
  onOpenFakeFolder,
  onOpenFakeChat,
  onOpenProjectTerminal,
  onToggleCustomization,
  onWidthChange,
  runningTerminalProjectIds,
  width,
}: SidePanelProps) {
  const [projects, setProjects] = useState<Project[]>([]);
  const [projectContextMenu, setProjectContextMenu] = useState<ProjectContextMenu | null>(null);
  const [projectRemovalDialog, setProjectRemovalDialog] = useState<ProjectRemovalDialog | null>(null);
  const [projectRemovalError, setProjectRemovalError] = useState("");
  const [projectRemovalInfo, setProjectRemovalInfo] = useState<ProjectRemovalInfo | null>(null);
  const [isRemovingProject, setIsRemovingProject] = useState(false);
  const [openProjectIds, setOpenProjectIds] = useState<Set<string>>(() => new Set());
  const [visibleChatCounts, setVisibleChatCounts] = useState<Map<string, number>>(() => new Map());
  const [projectScrollThumb, setProjectScrollThumb] = useState<ProjectScrollThumb>({ height: 0, isVisible: false, top: 0 });
  const [projectScrollShadowOpacity, setProjectScrollShadowOpacity] = useState(0);
  const [projectBottomScrollShadowOpacity, setProjectBottomScrollShadowOpacity] = useState(0);
  const [userName, setUserName] = useState("");
  const [userNameDraft, setUserNameDraft] = useState("");
  const [isEditingUserName, setIsEditingUserName] = useState(false);
  const [isSavingUserName, setIsSavingUserName] = useState(false);
  const [userNameError, setUserNameError] = useState("");
  const userNameInputRef = useRef<HTMLInputElement>(null);
  const projectsListRef = useRef<HTMLElement>(null);

  useEffect(() => {
    const controller = new AbortController();
    void fetchProjectSidebarData(controller.signal)
      .then((data) => {
        setProjects(data.projects);
        setOpenProjectIds(new Set(data.projects.map((project) => project.id)));
        setUserName(data.userName);
      })
      .catch(() => {
        setProjects([]);
        setUserName("");
      });
    return () => controller.abort();
  }, []);

  useEffect(() => {
    if (!isEditingUserName) return;
    userNameInputRef.current?.focus();
    userNameInputRef.current?.select();
  }, [isEditingUserName]);

  useEffect(() => {
    const list = projectsListRef.current;
    if (!list) return;

    const updateScrollThumb = () => {
      setProjectScrollShadowOpacity(Math.min(1, list.scrollTop / 18));
      const scrollableHeight = list.scrollHeight - list.clientHeight;
      setProjectBottomScrollShadowOpacity(Math.min(1, Math.max(0, scrollableHeight - list.scrollTop) / 18));
      const isVisible = scrollableHeight > 0;
      const height = isVisible
        ? Math.max(MIN_SCROLL_THUMB_HEIGHT, (list.clientHeight * list.clientHeight) / list.scrollHeight)
        : 0;
      const top = isVisible
        ? (list.scrollTop / scrollableHeight) * (list.clientHeight - height)
        : 0;
      const nextThumb = { height, isVisible, top };
      setProjectScrollThumb((currentThumb) => (
        currentThumb.height === nextThumb.height
          && currentThumb.isVisible === nextThumb.isVisible
          && currentThumb.top === nextThumb.top
          ? currentThumb
          : nextThumb
      ));
    };

    const resizeObserver = new ResizeObserver(updateScrollThumb);
    resizeObserver.observe(list);
    list.addEventListener("scroll", updateScrollThumb, { passive: true });
    updateScrollThumb();
    return () => {
      resizeObserver.disconnect();
      list.removeEventListener("scroll", updateScrollThumb);
    };
  }, [bottomInset, openProjectIds, projects, visibleChatCounts]);

  useEffect(() => {
    if (!projectContextMenu) return;

    const closeMenu = () => setProjectContextMenu(null);
    const closeOnEscape = (event: globalThis.KeyboardEvent) => {
      if (event.key === "Escape") closeMenu();
    };
    window.addEventListener("pointerdown", closeMenu);
    window.addEventListener("resize", closeMenu);
    window.addEventListener("keydown", closeOnEscape);
    return () => {
      window.removeEventListener("pointerdown", closeMenu);
      window.removeEventListener("resize", closeMenu);
      window.removeEventListener("keydown", closeOnEscape);
    };
  }, [projectContextMenu]);

  function openProjectContextMenu(project: Project, x: number, y: number) {
    setProjectContextMenu({
      project,
      x: Math.max(PROJECT_CONTEXT_MENU_EDGE_GAP, Math.min(x, window.innerWidth - PROJECT_CONTEXT_MENU_WIDTH - PROJECT_CONTEXT_MENU_EDGE_GAP)),
      y: Math.max(PROJECT_CONTEXT_MENU_EDGE_GAP, Math.min(y, window.innerHeight - PROJECT_CONTEXT_MENU_HEIGHT - PROJECT_CONTEXT_MENU_EDGE_GAP)),
    });
  }

  function handleProjectContextMenu(event: MouseEvent<HTMLElement>, project: Project) {
    event.preventDefault();
    openProjectContextMenu(project, event.clientX, event.clientY);
  }

  function handleProjectContextMenuKey(event: KeyboardEvent<HTMLButtonElement>, project: Project) {
    if (event.key !== "ContextMenu" && !(event.shiftKey && event.key === "F10")) return;
    event.preventDefault();
    const rect = event.currentTarget.getBoundingClientRect();
    openProjectContextMenu(project, rect.left + 12, rect.bottom + 4);
  }

  function renameProject(project: Project) {
    const name = window.prompt("Project name", project.name)?.trim();
    if (!name || name === project.name) return;
    setProjects((currentProjects) => currentProjects.map((currentProject) => (
      currentProject.id === project.id ? { ...currentProject, name } : currentProject
    )));
  }

  async function copyProjectPath(path: string) {
    try {
      await navigator.clipboard.writeText(path);
    } catch {
      // The clipboard is unavailable on some non-secure browser previews.
      window.prompt("Copy absolute path", path);
    }
  }

  function removeProjectFromList(project: Project) {
    setProjects((currentProjects) => currentProjects.filter((currentProject) => currentProject.id !== project.id));
    setOpenProjectIds((currentProjectIds) => {
      const nextProjectIds = new Set(currentProjectIds);
      nextProjectIds.delete(project.id);
      return nextProjectIds;
    });
  }

  function openProjectRemovalDialog(project: Project, removeData: boolean) {
    setProjectRemovalError("");
    setProjectRemovalInfo({ dataPath: "", dataSizeBytes: -1, projectPath: project.path, projectSizeBytes: -1 });
    setProjectRemovalDialog({ project, removeData });
    void fetchProjectRemovalInfo(project.id)
      .then(setProjectRemovalInfo)
      .catch(() => setProjectRemovalError("Could not read the project folder details."));
  }

  async function confirmProjectRemoval() {
    if (!projectRemovalDialog || !projectRemovalInfo || isRemovingProject) return;
    const { project, removeData } = projectRemovalDialog;
    setIsRemovingProject(true);
    setProjectRemovalError("");
    try {
      if (removeData) await removeProjectFromDisk(project.id);
      else await removeProjectFromSidebar(project.id);
      removeProjectFromList(project);
      setProjectRemovalDialog(null);
    } catch {
      setProjectRemovalError(`Could not remove “${project.name}”. Try again.`);
    } finally {
      setIsRemovingProject(false);
    }
  }

  function beginUserNameEdit() {
    setUserNameDraft(userName);
    setUserNameError("");
    setIsEditingUserName(true);
  }

  function cancelUserNameEdit() {
    if (isSavingUserName) return;
    setIsEditingUserName(false);
    setUserNameDraft(userName);
    setUserNameError("");
  }

  async function submitUserNameEdit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsSavingUserName(true);
    setUserNameError("");
    try {
      const savedUserName = await saveUserName(userNameDraft.trim());
      setUserName(savedUserName);
      setIsEditingUserName(false);
    } catch {
      setUserNameError("Could not save the name. Try again.");
    } finally {
      setIsSavingUserName(false);
    }
  }

  function startProjectScrollThumbDrag(event: ReactPointerEvent<HTMLDivElement>) {
    const list = projectsListRef.current;
    if (!list) return;
    event.preventDefault();
    const startClientY = event.clientY;
    const startScrollTop = list.scrollTop;
    const thumbTrackHeight = list.clientHeight - projectScrollThumb.height;
    const scrollableHeight = list.scrollHeight - list.clientHeight;

    const onPointerMove = (moveEvent: PointerEvent) => {
      if (thumbTrackHeight <= 0 || scrollableHeight <= 0) return;
      list.scrollTop = startScrollTop + ((moveEvent.clientY - startClientY) / thumbTrackHeight) * scrollableHeight;
    };
    const stopDrag = () => {
      window.removeEventListener("pointermove", onPointerMove);
      window.removeEventListener("pointerup", stopDrag);
    };

    window.addEventListener("pointermove", onPointerMove);
    window.addEventListener("pointerup", stopDrag, { once: true });
  }

  return (
    <aside
      aria-label="Side panel"
      className="side-panel"
      id="side-panel"
      style={{ "--side-panel-bottom-inset": `${bottomInset}px` } as CSSProperties}
    >
      <SidePanelResizeHandle onWidthChange={onWidthChange} side="left" width={width} />
      <div className="side-panel-head" />
      <div className="side-panel-actions">
        <button className="side-panel-action" type="button">
          <span aria-hidden="true" className="side-panel-action-icon">
            <NewProjectIcon />
          </span>
          <span className="side-panel-action-label">New Project</span>
        </button>
        <button className="side-panel-action" type="button">
          <span aria-hidden="true" className="side-panel-action-icon">
            <SearchIcon />
          </span>
          <span className="side-panel-action-label">Search</span>
        </button>
        <button
          aria-pressed={isCustomizationOpen}
          className={`side-panel-action${isCustomizationOpen ? " is-active" : ""}`}
          onClick={onToggleCustomization}
          type="button"
        >
          <span aria-hidden="true" className="side-panel-action-icon">
            <CustomizationIcon />
          </span>
          <span className="side-panel-action-label">Customization</span>
        </button>
      </div>
      <div
        className="side-panel-section-label"
        style={{ "--side-panel-scroll-shadow-opacity": projectScrollShadowOpacity } as CSSProperties}
      >
        Projects
      </div>
      <div className="side-panel-projects-shell">
      <nav aria-label="Projects" className="side-panel-projects" ref={projectsListRef}>
        <section className="side-panel-project side-panel-test-folder">
          <div className="side-panel-project-head">
            <button className="side-panel-project-trigger" aria-label="Apri nuova chat nella cartella Test chats" onClick={onOpenFakeFolder} type="button">
              <FolderIcon isOpen />
              <span>Test chats</span>
            </button>
          </div>
          <div className="side-panel-project-children">
            {initialFakeChats.map((chat) => (
              <button className="side-panel-chat" key={chat.id} onClick={() => onOpenFakeChat(chat.id)} title={chat.title} type="button">
                <span>{chat.title}</span>
                <time>test</time>
              </button>
            ))}
          </div>
        </section>
        {projects.map((project) => {
          const isProjectOpen = openProjectIds.has(project.id);
          const visibleChatCount = visibleChatCounts.get(project.id) ?? INITIAL_CHAT_LIMIT;
          const visibleChats = project.chats.slice(0, visibleChatCount);
          const remainingChatCount = project.chats.length - visibleChats.length;

          return (
            <section className="side-panel-project" key={project.id} onContextMenu={(event) => handleProjectContextMenu(event, project)}>
              <div className="side-panel-project-head">
                <button
                  aria-expanded={isProjectOpen}
                  className="side-panel-project-trigger"
                  onClick={() => setOpenProjectIds((projectIds) => {
                    const nextProjectIds = new Set(projectIds);
                    if (isProjectOpen) nextProjectIds.delete(project.id);
                    else nextProjectIds.add(project.id);
                    return nextProjectIds;
                  })}
                  onKeyDown={(event) => handleProjectContextMenuKey(event, project)}
                  title={project.path}
                  type="button"
                >
                  <FolderIcon isOpen={isProjectOpen} />
                  <span>{project.name}</span>
                </button>
                {armedTerminalProjectIds.includes(project.id) ? (
                  <button
                    aria-label={`Open terminal for ${project.name}`}
                    className={`side-panel-project-terminal${runningTerminalProjectIds.includes(project.id) ? " is-running" : ""}`}
                    onClick={() => onOpenProjectTerminal(project)}
                    title={`Open terminal for ${project.name}`}
                    type="button"
                  >
                    <ProjectTerminalIcon />
                  </button>
                ) : null}
                <button
                  aria-label={`New chat in ${project.name}`}
                  className="side-panel-project-new"
                  onClick={() => onNewProjectChat(project)}
                  title={`New chat in ${project.name}`}
                  type="button"
                >
                  <PlusIcon />
                </button>
              </div>
              {isProjectOpen ? (
                <div className="side-panel-project-children">
                  {visibleChats.map((chat) => (
                    <button className="side-panel-chat" key={chat.id} title={chat.title} type="button">
                      <span>{chat.title}</span>
                      <time dateTime={chat.lastMessageAt} title={`Last interaction: ${chat.lastMessageAt}`}>
                        {formatRelativeTime(chat.lastMessageAt)}
                      </time>
                    </button>
                  ))}
                  {remainingChatCount > 0 ? (
                    <button
                      className="side-panel-show-more"
                      onClick={() => setVisibleChatCounts((chatCounts) => {
                        const nextChatCounts = new Map(chatCounts);
                        nextChatCounts.set(project.id, Math.min(project.chats.length, visibleChats.length + INITIAL_CHAT_LIMIT));
                        return nextChatCounts;
                      })}
                      type="button"
                    >
                      Show more
                    </button>
                  ) : null}
                </div>
              ) : null}
            </section>
          );
        })}
      </nav>
        <div aria-hidden="true" className="side-panel-project-scrollbar">
          {projectScrollThumb.isVisible ? (
            <div
              className="side-panel-project-scrollbar-thumb"
              onPointerDown={startProjectScrollThumbDrag}
              style={{ height: projectScrollThumb.height, transform: `translateY(${projectScrollThumb.top}px)` }}
            />
          ) : null}
        </div>
      </div>
      <div
        className={`side-panel-user${isEditingUserName ? " is-editing" : ""}`}
        style={{ "--side-panel-bottom-scroll-shadow-opacity": projectBottomScrollShadowOpacity } as CSSProperties}
      >
        {isEditingUserName ? (
          <form className="side-panel-user-edit" onSubmit={submitUserNameEdit}>
            <input
              aria-label="User name"
              autoComplete="name"
              className="side-panel-user-name-input"
              disabled={isSavingUserName}
              onChange={(event) => setUserNameDraft(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Escape") cancelUserNameEdit();
              }}
              ref={userNameInputRef}
              value={userNameDraft}
            />
            <button
              aria-label="Cancel editing user name"
              className="side-panel-user-cancel"
              disabled={isSavingUserName}
              onClick={cancelUserNameEdit}
              title="Cancel"
              type="button"
            >
              <CloseIcon />
            </button>
          </form>
        ) : (
          <>
            <button className="side-panel-user-name" onDoubleClick={beginUserNameEdit} title="Double-click to edit" type="button">
              {userName || "Unnamed user"}
            </button>
            <button aria-label="User settings" className="side-panel-user-settings" title="User settings" type="button">
              <SettingsIcon />
            </button>
          </>
        )}
        {userNameError ? <span className="side-panel-user-error" role="status">{userNameError}</span> : null}
      </div>
      {projectContextMenu ? (
        <div
          aria-label={`Actions for ${projectContextMenu.project.name}`}
          className="project-context-menu"
          onPointerDown={(event) => event.stopPropagation()}
          role="menu"
          style={{ left: projectContextMenu.x, top: projectContextMenu.y }}
        >
          <button onClick={() => { renameProject(projectContextMenu.project); setProjectContextMenu(null); }} role="menuitem" type="button">
            Rename
          </button>
          <button onClick={() => { void copyProjectPath(projectContextMenu.project.path); setProjectContextMenu(null); }} role="menuitem" type="button">
            Copy absolute path
          </button>
          <div className="project-context-menu-divider" role="separator" />
          <button onClick={() => { openProjectRemovalDialog(projectContextMenu.project, false); setProjectContextMenu(null); }} role="menuitem" type="button">
            Remove from sidebar
          </button>
          <button className="is-danger" onClick={() => { openProjectRemovalDialog(projectContextMenu.project, true); setProjectContextMenu(null); }} role="menuitem" type="button">
            Remove from disk
          </button>
        </div>
      ) : null}
      {projectRemovalDialog ? (
        <div className="project-removal-dialog-backdrop" role="presentation">
          <section aria-describedby="project-removal-dialog-description" aria-labelledby="project-removal-dialog-title" aria-modal="true" className="project-removal-dialog" role="dialog">
            <div className="project-removal-dialog-marker" aria-hidden="true">!</div>
            <div>
              <p className="project-removal-dialog-eyebrow">{projectRemovalDialog.removeData ? "Permanent action" : "Sidebar only"}</p>
              <h2 id="project-removal-dialog-title">{projectRemovalDialog.removeData ? "Remove project from disk?" : "Remove project from sidebar?"}</h2>
              <p id="project-removal-dialog-description">
                {projectRemovalDialog.removeData
                  ? <>This permanently removes <strong>{projectRemovalDialog.project.name}</strong>, its project folder, and all Solomon chats.</>
                  : <>This removes <strong>{projectRemovalDialog.project.name}</strong> from the sidebar. Its project folder and chats stay on disk.</>}
              </p>
            </div>
            {projectRemovalError ? <p className="project-removal-dialog-error" role="alert">{projectRemovalError}</p> : null}
            {projectRemovalInfo ? (
              <dl className="project-removal-dialog-details">
                <div>
                  <dt>Project folder</dt>
                  <dd title={projectRemovalInfo.projectPath}>{projectRemovalInfo.projectPath}</dd>
                  <small>{formatFileSize(projectRemovalInfo.projectSizeBytes)}</small>
                </div>
                {projectRemovalDialog.removeData && projectRemovalInfo.dataPath ? (
                  <div>
                    <dt>Solomon data &amp; chats</dt>
                    <dd title={projectRemovalInfo.dataPath}>{projectRemovalInfo.dataPath}</dd>
                    <small>{formatFileSize(projectRemovalInfo.dataSizeBytes)}</small>
                  </div>
                ) : null}
              </dl>
            ) : <p className="project-removal-dialog-loading">Reading folder details…</p>}
            <div className="project-removal-dialog-actions">
              <button disabled={isRemovingProject} onClick={() => setProjectRemovalDialog(null)} type="button">Cancel</button>
              <button className="is-danger" disabled={isRemovingProject || !projectRemovalInfo} onClick={() => void confirmProjectRemoval()} type="button">
                {isRemovingProject ? "Removing…" : projectRemovalDialog.removeData ? "Remove from disk" : "Remove from sidebar"}
              </button>
            </div>
          </section>
        </div>
      ) : null}
    </aside>
  );
}

function formatRelativeTime(dateTime: string) {
  const timestamp = Date.parse(dateTime);
  if (Number.isNaN(timestamp)) return "";

  const minutes = Math.max(0, Math.floor((Date.now() - timestamp) / 60_000));
  if (minutes < 1) return "now";
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h`;
  const days = Math.floor(hours / 24);
  return `${days}d`;
}

function formatFileSize(bytes: number) {
  if (bytes < 0) return "Size unavailable";
  if (bytes < 1024) return `${bytes} B`;
  const units = ["KB", "MB", "GB", "TB"];
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)) - 1, units.length - 1);
  return `${(bytes / (1024 ** (index + 1))).toFixed(bytes >= 1024 ** (index + 2) ? 1 : 0)} ${units[index]}`;
}

function FolderIcon({ isOpen }: { isOpen: boolean }) {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d={isOpen
        ? "m6 14 1.5-2.9A2 2 0 0 1 9.24 10H20a2 2 0 0 1 1.94 2.5l-1.54 6a2 2 0 0 1-1.95 1.5H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h3.9a2 2 0 0 1 1.69.9l.81 1.2a2 2 0 0 0 1.67.9H18a2 2 0 0 1 2 2v2"
        : "M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"}
      />
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

function ProjectTerminalIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="m7 11 2-2-2-2" />
      <path d="M11 13h4" />
      <rect height="18" rx="2" ry="2" width="18" x="3" y="3" />
    </svg>
  );
}

function SettingsIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <circle cx="12" cy="12" r="3" />
      <path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z" />
    </svg>
  );
}

function CloseIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="m6 6 12 12M18 6 6 18" />
    </svg>
  );
}

function NewProjectIcon() {
  return (
    <svg viewBox="0 0 24 24">
      <path d="M5 6.5A2.5 2.5 0 0 1 7.5 4h11A2.5 2.5 0 0 1 21 6.5v7A2.5 2.5 0 0 1 18.5 16H12l-4 3v-3H7.5A2.5 2.5 0 0 1 5 13.5V6.5Z" />
    </svg>
  );
}

function SearchIcon() {
  return (
    <svg viewBox="0 0 24 24">
      <circle cx="11" cy="11" r="6.5" />
      <path d="m16 16 4 4" />
    </svg>
  );
}

function CustomizationIcon() {
  return (
    <svg viewBox="4 7 15 17">
      <path d="M5 22H16V19A2.5 2.5 0 0 1 16 14V12H13A2.5 2.5 0 0 0 8 12H5V22Z" />
    </svg>
  );
}
type ProjectScrollThumb = {
  height: number;
  isVisible: boolean;
  top: number;
};
