import { type CSSProperties, type FormEvent, type PointerEvent as ReactPointerEvent, useEffect, useRef, useState } from "react";
import { fetchProjectSidebarData, type Project, saveUserName } from "../projects/projects";

const INITIAL_CHAT_LIMIT = 5;
const MIN_SCROLL_THUMB_HEIGHT = 28;

type ProjectScrollThumb = {
  height: number;
  isVisible: boolean;
  top: number;
};

type SidePanelProps = {
  bottomInset: number;
  isCustomizationOpen: boolean;
  onToggleCustomization: () => void;
};

export function SidePanel({ bottomInset, isCustomizationOpen, onToggleCustomization }: SidePanelProps) {
  const [projects, setProjects] = useState<Project[]>([]);
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
        {projects.map((project) => {
          const isProjectOpen = openProjectIds.has(project.id);
          const visibleChatCount = visibleChatCounts.get(project.id) ?? INITIAL_CHAT_LIMIT;
          const visibleChats = project.chats.slice(0, visibleChatCount);
          const remainingChatCount = project.chats.length - visibleChats.length;

          return (
            <section className="side-panel-project" key={project.id}>
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
                  title={project.path}
                  type="button"
                >
                  <FolderIcon isOpen={isProjectOpen} />
                  <span>{project.name}</span>
                </button>
                <button aria-label={`New project in ${project.name}`} className="side-panel-project-new" title={`New project in ${project.name}`} type="button">
                  <PlusIcon />
                </button>
              </div>
              {isProjectOpen ? visibleChats.map((chat) => (
                <button className="side-panel-chat" key={chat.id} title={chat.title} type="button">
                  <span>{chat.title}</span>
                  <time dateTime={chat.lastMessageAt} title={`Last interaction: ${chat.lastMessageAt}`}>
                    {formatRelativeTime(chat.lastMessageAt)}
                  </time>
                </button>
              )) : null}
              {isProjectOpen && remainingChatCount > 0 ? (
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
