import { RemovalDialog } from "./sidebar/RemovalDialog";
import { useProjectScroll } from "./sidebar/useProjectScroll";
import { Actions } from "./sidebar/Actions";
import { ChatListButton, formatRelativeTime, FolderIcon, PlusIcon, ProjectTerminalIcon, SettingsIcon, CloseIcon } from "./sidebar/elements";
export { ChatListButton } from "./sidebar/elements";
import type { OpenChatContextMenu } from "./useChatContextMenu";
import { type CSSProperties, type FormEvent, type KeyboardEvent, type MouseEvent, useEffect, useRef, useState } from "react";
import { cacheUserName, fetchProjectRemovalInfo, fetchProjectSidebarData, getCachedProjectSidebarData, getCachedUserName, PROJECTS_CHANGED_EVENT, type Project, type ProjectRemovalInfo, removeProjectFromDisk, removeProjectFromSidebar, saveUserName } from "../projects/projects";
import { applyProjectChatActivity, PROJECT_CHAT_ACTIVITY_EVENT } from "../projects/chatActivity";
import type { TemporaryWorkspace } from "../projects/temporaryWorkspace";
import { SidePanelResizeHandle } from "./SidePanelResizeHandle";
import { fetchActiveAgentChatIDs } from "./ActiveAgentsPage";
import type { AppUpdate } from "./useAppUpdate";
import { UpdateButton } from "./UpdateButton";

const INITIAL_CHAT_LIMIT = 5;
const PROJECT_CONTEXT_MENU_HEIGHT = 158;
const PROJECT_CONTEXT_MENU_WIDTH = 200;
const PROJECT_CONTEXT_MENU_EDGE_GAP = 8;
const ACTIVE_AGENT_POLL_MS = 1000;

type ProjectContextMenu = { project: Project; x: number; y: number };

type ProjectRemovalDialog = { project: Project; removeData: boolean };

type SidePanelProps = {
  appUpdate: AppUpdate;
  armedTerminalProjectIds: string[];
  bottomInset: number;
  isActiveAgentsOpen: boolean;
  isCustomizationOpen: boolean;
  onNewProjectChat: (project: Project) => void;
  onOpenNewProject: () => void;
  onOpenTemporaryWorkspace: () => void;
  onOpenProjectChat: (project: Project, chatID: string) => void;
  onOpenProjectTerminal: (project: Project) => void;
  onOpenChatContextMenu: OpenChatContextMenu;
  onOpenSettings: () => void;
  onToggleActiveAgents: () => void;
  onToggleCustomization: () => void;
  onWidthChange: (width: number) => void;
  runningTerminalProjectIds: string[];
  streamingChatIDs?: ReadonlySet<string>;
  unreadCompletedChatIDs?: ReadonlySet<string>;
  temporaryWorkspace: TemporaryWorkspace | null;
  width: number;
};

export function SidePanel({
  appUpdate,
  armedTerminalProjectIds,
  bottomInset,
  isActiveAgentsOpen,
  isCustomizationOpen,
  onNewProjectChat,
  onOpenNewProject,
  onOpenTemporaryWorkspace,
  onOpenProjectChat,
  onOpenProjectTerminal,
  onOpenChatContextMenu,
  onOpenSettings,
  onToggleActiveAgents,
  onToggleCustomization,
  onWidthChange,
  runningTerminalProjectIds,
  streamingChatIDs,
  unreadCompletedChatIDs,
  temporaryWorkspace,
  width,
}: SidePanelProps) {
  const [projects, setProjects] = useState<Project[]>(() => applyProjectChatActivity(getCachedProjectSidebarData()?.projects ?? []));
  const [activeAgentChatIDs, setActiveAgentChatIDs] = useState<ReadonlySet<string>>(() => new Set());
  const [projectContextMenu, setProjectContextMenu] = useState<ProjectContextMenu | null>(null);
  const [projectRemovalDialog, setProjectRemovalDialog] = useState<ProjectRemovalDialog | null>(null);
  const [projectRemovalError, setProjectRemovalError] = useState("");
  const [projectRemovalInfo, setProjectRemovalInfo] = useState<ProjectRemovalInfo | null>(null);
  const [isRemovingProject, setIsRemovingProject] = useState(false);
  const [isTemporaryWorkspaceOpen, setIsTemporaryWorkspaceOpen] = useState(true);
  const [openProjectIds, setOpenProjectIds] = useState<Set<string>>(() => new Set(getCachedProjectSidebarData()?.projects.map((project) => project.id) ?? []));
  const [visibleChatCounts, setVisibleChatCounts] = useState<Map<string, number>>(() => new Map());
  const { projectsListRef, projectScrollThumb, projectScrollShadowOpacity, projectBottomScrollShadowOpacity, startProjectScrollThumbDrag } = useProjectScroll({ bottomInset, openProjectIds, projects, temporaryWorkspace, visibleChatCounts });
  const [userName, setUserName] = useState(() => getCachedUserName() ?? "");
  const [userNameDraft, setUserNameDraft] = useState("");
  const [isEditingUserName, setIsEditingUserName] = useState(false);
  const [isSavingUserName, setIsSavingUserName] = useState(false);
  const [userNameError, setUserNameError] = useState("");
  const userNameInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    let cancelled = false;
    let requestInFlight = false;
    const controller = new AbortController();
    const loadActiveAgentChatIDs = async () => {
      if (requestInFlight) return;
      requestInFlight = true;
      try {
        const next = await fetchActiveAgentChatIDs(controller.signal);
        if (!cancelled) setActiveAgentChatIDs(next);
      } catch {
        // Preserve the last confirmed active state during a transient daemon outage.
      } finally {
        requestInFlight = false;
      }
    };
    void loadActiveAgentChatIDs();
    const timer = window.setInterval(() => void loadActiveAgentChatIDs(), ACTIVE_AGENT_POLL_MS);
    return () => {
      cancelled = true;
      controller.abort();
      window.clearInterval(timer);
    };
  }, []);

  useEffect(() => {
    let currentController: AbortController | null = null;
    const loadProjects = () => {
      currentController?.abort();
      const requestController = new AbortController();
      currentController = requestController;
      void fetchProjectSidebarData(requestController.signal)
        .then((data) => {
          if (requestController.signal.aborted || currentController !== requestController) return;
          setProjects(applyProjectChatActivity(data.projects));
          setOpenProjectIds(new Set(data.projects.map((project) => project.id)));
          const nextUserName = data.userName.trim();
          cacheUserName(nextUserName);
          setUserName(nextUserName);
        })
        .catch(() => {
          if (!requestController.signal.aborted && currentController === requestController && !getCachedProjectSidebarData()) setProjects([]);
        });
    };
    loadProjects();
    const updateActivity = () => setProjects((current) => applyProjectChatActivity(current));
    window.addEventListener(PROJECT_CHAT_ACTIVITY_EVENT, updateActivity);
    window.addEventListener(PROJECTS_CHANGED_EVENT, loadProjects);
    return () => {
      currentController?.abort();
      window.removeEventListener(PROJECT_CHAT_ACTIVITY_EVENT, updateActivity);
      window.removeEventListener(PROJECTS_CHANGED_EVENT, loadProjects);
    };
  }, []);

  useEffect(() => {
    if (!isEditingUserName) return;
    userNameInputRef.current?.focus();
    userNameInputRef.current?.select();
  }, [isEditingUserName]);

  useEffect(() => {
    if (temporaryWorkspace) setIsTemporaryWorkspaceOpen(true);
  }, [temporaryWorkspace?.id]);

  useEffect(() => {
    if (!projectContextMenu) return;

    const closeMenu = () => {
      setProjectContextMenu(null);
    };
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

  function handleChatContextMenu(event: MouseEvent<HTMLButtonElement>, projectID: string, chatID: string, title: string) {
    event.preventDefault();
    event.stopPropagation();
    setProjectContextMenu(null);
    onOpenChatContextMenu(projectID, chatID, title, event.clientX, event.clientY);
  }

  function handleChatContextMenuKey(event: KeyboardEvent<HTMLButtonElement>, projectID: string, chatID: string, title: string) {
    if (event.key !== "ContextMenu" && !(event.shiftKey && event.key === "F10")) return;
    event.preventDefault();
    const rect = event.currentTarget.getBoundingClientRect();
    setProjectContextMenu(null);
    onOpenChatContextMenu(projectID, chatID, title, rect.left + 12, rect.bottom + 4);
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
      window.dispatchEvent(new CustomEvent(PROJECTS_CHANGED_EVENT));
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
      const nextUserName = savedUserName.trim();
      cacheUserName(nextUserName);
      setUserName(nextUserName);
      setIsEditingUserName(false);
    } catch {
      setUserNameError("Could not save the name. Try again.");
    } finally {
      setIsSavingUserName(false);
    }
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
      <Actions isActiveAgentsOpen={isActiveAgentsOpen} isCustomizationOpen={isCustomizationOpen} onToggleActiveAgents={onToggleActiveAgents} onToggleCustomization={onToggleCustomization} />
      <div
        className="side-panel-section-label"
        style={{ "--side-panel-scroll-shadow-opacity": projectScrollShadowOpacity } as CSSProperties}
      >
        <span>Projects</span>
        <button
          aria-label="New project"
          className="side-panel-section-new-project"
          onClick={onOpenNewProject}
          title="New project"
          type="button"
        >
          <PlusIcon />
        </button>
      </div>
      <div className="side-panel-projects-shell">
      <nav aria-label="Projects" className="side-panel-projects" ref={projectsListRef}>
        {temporaryWorkspace ? (
          <section className="side-panel-project side-panel-temporary-project">
            <div className="side-panel-project-head">
              <button
                aria-expanded={isTemporaryWorkspaceOpen}
                className="side-panel-project-trigger"
                onClick={() => {
                  setIsTemporaryWorkspaceOpen((isOpen) => !isOpen);
                  onOpenTemporaryWorkspace();
                }}
                title={temporaryWorkspace.displayPath}
                type="button"
              >
                <FolderIcon isOpen={isTemporaryWorkspaceOpen} />
                <span>{temporaryWorkspace.name}</span>
              </button>
            </div>
          </section>
        ) : null}
        {projects.map((project) => {
          const isProjectOpen = openProjectIds.has(project.id);
          const visibleChatCount = visibleChatCounts.get(project.id) ?? INITIAL_CHAT_LIMIT;
          const visibleChats = [...project.chats].sort((left, right) => (Date.parse(right.lastMessageAt) || 0) - (Date.parse(left.lastMessageAt) || 0)).slice(0, visibleChatCount);
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
                    <ChatListButton
                      chatID={chat.id}
                      dateTime={chat.lastMessageAt}
                      isWorking={streamingChatIDs?.has(chat.id) || activeAgentChatIDs.has(chat.id)}
                      isUnreadCompleted={unreadCompletedChatIDs?.has(chat.id)}
                      key={chat.id}
                      onClick={() => onOpenProjectChat(project, chat.id)}
                      onContextMenu={(event) => handleChatContextMenu(event, project.id, chat.id, chat.title)}
                      onContextMenuKey={(event) => handleChatContextMenuKey(event, project.id, chat.id, chat.title)}
                      timeLabel={formatRelativeTime(chat.lastMessageAt)}
                      timeTitle={`Last interaction: ${chat.lastMessageAt}`}
                      title={chat.title}
                    />
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
            <UpdateButton update={appUpdate} />
            <button aria-label="User settings" className="side-panel-user-settings" onClick={onOpenSettings} title="User settings" type="button">
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
      {projectRemovalDialog ? <RemovalDialog projectRemovalDialog={projectRemovalDialog} projectRemovalError={projectRemovalError} projectRemovalInfo={projectRemovalInfo} isRemovingProject={isRemovingProject} onCancel={() => setProjectRemovalDialog(null)} onConfirm={() => void confirmProjectRemoval()} /> : null}
    </aside>
  );
}
