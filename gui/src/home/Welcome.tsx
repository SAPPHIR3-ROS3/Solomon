import { useEffect, useLayoutEffect, useRef, useState, type CSSProperties } from "react";
import asciiBanner from "../../../internal/logo/logo.txt?raw";
import asciiColors from "../../../internal/logo/colors.txt?raw";
import { serverEndpoint } from "../platform";
import {
  cacheHomeStats,
  cacheUserName,
  fetchProjectSidebarData,
  getCachedProjectSidebarData,
  getCachedHomeStats,
  getCachedUserName,
  PROJECTS_CHANGED_EVENT,
  type Project,
  type ProjectTokenStats,
} from "../projects/projects";
import type { TemporaryWorkspace } from "../projects/temporaryWorkspace";
import { BranchControl, WorktreeControl } from "./BranchControl";
import { ChatComposer, ComposerCrownIcon, ComposerSendIcon, type ChatComposerMenu } from "../chat/ChatComposer";
import type { ComposerImageAttachment } from "../chat/composerTypes";
import "./welcome.css";
import "./welcome-reasoning.css";

const emptyProjectTokenStats: ProjectTokenStats = { user: 0, reasoning: 0, response: 0, total: 0 };

type WelcomeProps = {
  bottomInset?: number;
  isVisible?: boolean;
  onComposerBoundsChange?: (bounds: { left: number; right: number }) => void;
  onKeepAliveHeightChange?: (height: number) => void;
  onOpenNewProject?: () => void;
  onOpenTemporaryWorkspace?: () => void;
  onTemporaryWorkspacePathChange?: (path: string) => void;
  onSend?: (content: string, images?: ComposerImageAttachment[]) => void;
  onWorkspaceChange?: (project: Project | null) => void;
  isSending?: boolean;
  isTemporaryWorkspaceActive?: boolean;
  resetToken?: number;
  temporaryWorkspace?: TemporaryWorkspace | null;
  workspaceNameOverride?: string | null;
  workspaceFocus?: { project: Project; token: number } | null;
};

type Visibility = {
  banner: boolean;
  title: boolean;
  folder: boolean;
  composer: boolean;
  version: boolean;
  chatCount: boolean;
  tokenCount: boolean;
};

const asciiColorRows = asciiColors.trim().split(/\r?\n/).map((row) => row.trim().split(/\s+/));

export function Welcome({ bottomInset = 0, isSending = false, isTemporaryWorkspaceActive = false, isVisible = true, onComposerBoundsChange, onKeepAliveHeightChange, onOpenNewProject, onOpenTemporaryWorkspace, onTemporaryWorkspacePathChange, onSend, onWorkspaceChange, resetToken = 0, temporaryWorkspace = null, workspaceNameOverride = null, workspaceFocus = null }: WelcomeProps) {
  const [userName, setUserName] = useState(() => getCachedUserName() ?? "");
  const [projects, setProjects] = useState<Project[]>(() => getCachedProjectSidebarData()?.projects ?? []);
  const [homeStats, setHomeStats] = useState(() => getCachedHomeStats());
  const [workspaceName, setWorkspaceName] = useState("Home");
  const [selectedProject, setSelectedProject] = useState<Project | null>(null);
  const [version, setVersion] = useState("dev");
  const [openMenu, setOpenMenu] = useState<"workspace" | "branch" | "worktree" | Exclude<ChatComposerMenu, null> | null>(null);
  const [visibility, setVisibility] = useState<Visibility>({ banner: true, title: true, folder: true, composer: true, version: true, chatCount: true, tokenCount: true });
  const screenRef = useRef<HTMLElement>(null);
  const measureBannerRef = useRef<HTMLDivElement>(null);
  const measureTitleRef = useRef<HTMLHeadingElement>(null);
  const measureFolderRef = useRef<HTMLDivElement>(null);
  const measureComposerRef = useRef<HTMLDivElement>(null);
  const measureMetaRef = useRef<HTMLDivElement>(null);
  const composerRef = useRef<HTMLDivElement>(null);
  const keepAliveHandlerRef = useRef(onKeepAliveHeightChange);
  const workspaceFocusRef = useRef(workspaceFocus);
  const workspaceNameOverrideRef = useRef(workspaceNameOverride);
  const temporaryWorkspaceRef = useRef(temporaryWorkspace);
  const temporaryWorkspaceActiveRef = useRef(isTemporaryWorkspaceActive);
  keepAliveHandlerRef.current = onKeepAliveHeightChange;
  workspaceFocusRef.current = workspaceFocus;
  workspaceNameOverrideRef.current = workspaceNameOverride;
  temporaryWorkspaceRef.current = temporaryWorkspace;
  temporaryWorkspaceActiveRef.current = isTemporaryWorkspaceActive;

  useLayoutEffect(() => {
    const composer = composerRef.current;
    if (!composer) return;
    const update = () => {
      const { left, right } = composer.getBoundingClientRect();
      onComposerBoundsChange?.({ left, right });
    };
    const observer = new ResizeObserver(update);
    observer.observe(composer);
    window.addEventListener("resize", update);
    update();
    return () => {
      observer.disconnect();
      window.removeEventListener("resize", update);
    };
  }, [onComposerBoundsChange, visibility.composer]);

  useEffect(() => {
    let currentController: AbortController | null = null;
    const loadProjects = () => {
      currentController?.abort();
      const requestController = new AbortController();
      currentController = requestController;
      void fetchProjectSidebarData(requestController.signal)
        .then((data) => {
          if (requestController.signal.aborted || currentController !== requestController) return;
          const nextUserName = data.userName.trim();
          cacheUserName(nextUserName);
          setUserName(nextUserName);
          setProjects(data.projects);
          const home = data.projects.find((project) => project.name === "Home") ?? null;
          setHomeStats(home ? { chatCount: home.chatCount, tokenStats: home.tokenStats ?? emptyProjectTokenStats } : null);
          if (home) cacheHomeStats(home);
          const focus = workspaceFocusRef.current;
          if (focus) {
            const focused = data.projects.find((project) => project.id === focus.project.id) ?? focus.project;
            setWorkspaceName(focused.name);
            setSelectedProject(focused);
            onWorkspaceChange?.(focused);
            return;
          }
          if (workspaceNameOverrideRef.current) {
            setWorkspaceName(workspaceNameOverrideRef.current);
            setSelectedProject(null);
            onWorkspaceChange?.(null);
            return;
          }
          if (temporaryWorkspaceActiveRef.current && temporaryWorkspaceRef.current) {
            setWorkspaceName(temporaryWorkspaceRef.current.name);
            setSelectedProject(null);
            onWorkspaceChange?.(null);
            return;
          }
          setWorkspaceName(home?.name ?? "Home");
          setSelectedProject(home);
          onWorkspaceChange?.(home);
        })
        .catch(() => {
          if (requestController.signal.aborted || currentController !== requestController) return;
          if (getCachedProjectSidebarData()) return;
          setProjects([]);
          setSelectedProject(null);
          if (!workspaceFocusRef.current) onWorkspaceChange?.(null);
        });
    };
    loadProjects();
    window.addEventListener(PROJECTS_CHANGED_EVENT, loadProjects);
    return () => {
      currentController?.abort();
      window.removeEventListener(PROJECTS_CHANGED_EVENT, loadProjects);
    };
  }, [onWorkspaceChange]);

  useEffect(() => {
    if (workspaceNameOverride) {
      setWorkspaceName(workspaceNameOverride);
      setSelectedProject(null);
      return;
    }
    if (workspaceFocus) return;
    if (isTemporaryWorkspaceActive && temporaryWorkspace) {
      setWorkspaceName(temporaryWorkspace.name);
      setSelectedProject(null);
      return;
    }
    const home = projects.find((project) => project.name === "Home") ?? null;
    setWorkspaceName(home?.name ?? "Home");
    setSelectedProject(home);
    onWorkspaceChange?.(home);
  }, [isTemporaryWorkspaceActive, onWorkspaceChange, projects, temporaryWorkspace, workspaceFocus, workspaceNameOverride]);

  useEffect(() => {
    if (!workspaceFocus) return;
    setWorkspaceName(workspaceFocus.project.name);
    setSelectedProject(workspaceFocus.project);
    onWorkspaceChange?.(workspaceFocus.project);
  }, [onWorkspaceChange, workspaceFocus]);

  useEffect(() => {
    setOpenMenu(null);
  }, [resetToken]);

  useEffect(() => {
    if (!isVisible) setOpenMenu(null);
  }, [isVisible]);

  useEffect(() => {
    if (!isVisible) return;
    const controller = new AbortController();
    void loadServerVersion(controller.signal)
      .then((nextVersion) => {
        if (!controller.signal.aborted) setVersion(nextVersion);
      })
      .catch(() => {
        if (!controller.signal.aborted) setVersion("dev");
      });
    return () => controller.abort();
  }, [isVisible]);

  useLayoutEffect(() => {
    const screen = screenRef.current;
    if (!screen) return;

    const update = () => {
      const styles = getComputedStyle(screen);
      const padTop = parseFloat(styles.paddingTop);
      const padBottom = parseFloat(styles.paddingBottom);
      const available = Math.max(0, screen.clientHeight - padTop - padBottom);
      const bannerH = measureBannerRef.current?.offsetHeight ?? 0;
      const titleH = measureTitleRef.current?.offsetHeight ?? 0;
      const folderH = measureFolderRef.current?.offsetHeight ?? 0;
      const composerH = measureComposerRef.current?.offsetHeight ?? 0;
      const metaH = measureMetaRef.current?.offsetHeight ?? 0;
      const withAll = bannerH + folderH + composerH + metaH;
      const withTitle = titleH + 10 + folderH + composerH + metaH;
      const withFolder = folderH + composerH + metaH;
      const withComposerAndMeta = composerH + metaH;
      let next: Visibility;
      if (available >= withAll) next = { banner: true, title: true, folder: true, composer: true, version: true, chatCount: true, tokenCount: true };
      else if (available >= withTitle) next = { banner: false, title: true, folder: true, composer: true, version: true, chatCount: true, tokenCount: true };
      else if (available >= withFolder) next = { banner: false, title: false, folder: true, composer: true, version: true, chatCount: true, tokenCount: true };
      else if (available >= withComposerAndMeta) next = { banner: false, title: false, folder: false, composer: true, version: true, chatCount: true, tokenCount: true };
      else if (available >= composerH) next = { banner: false, title: false, folder: false, composer: true, version: false, chatCount: false, tokenCount: false };
      else next = { banner: false, title: false, folder: false, composer: false, version: false, chatCount: false, tokenCount: false };
      keepAliveHandlerRef.current?.(padTop + folderH + composerH + (next.version || next.chatCount || next.tokenCount ? metaH : 0) + padBottom);
      setVisibility((current) => (
        current.banner === next.banner && current.title === next.title && current.folder === next.folder && current.composer === next.composer && current.version === next.version && current.chatCount === next.chatCount && current.tokenCount === next.tokenCount
          ? current
          : next
      ));
    };

    const observer = new ResizeObserver(update);
    observer.observe(screen);
    if (measureComposerRef.current) observer.observe(measureComposerRef.current);
    update();
    return () => observer.disconnect();
  }, [bottomInset, homeStats?.chatCount, homeStats?.tokenStats?.total, selectedProject?.chatCount, selectedProject?.tokenStats?.total, userName, version, workspaceNameOverride]);

  const displayName = userName || "User";
  const homeStatsFallback = !workspaceNameOverride && (!selectedProject || selectedProject.name === "Home") ? homeStats : null;
  const displayChatCount = selectedProject?.chatCount ?? homeStatsFallback?.chatCount ?? 0;
  const tokenStats = selectedProject?.tokenStats ?? homeStatsFallback?.tokenStats ?? emptyProjectTokenStats;
  return (
    <section
      aria-label="Home"
      aria-hidden={!isVisible}
      className={`welcome-screen${isVisible ? "" : " is-hidden"}`}
      ref={screenRef}
      style={{ bottom: Math.max(0, bottomInset) }}
    >
      <div aria-hidden="true" className="welcome-measure">
        <div ref={measureBannerRef}><AsciiBanner /></div>
        <h1 className="welcome-title is-solo" ref={measureTitleRef}>
          <span>Welcome back, </span>
          <strong>{displayName}</strong>
        </h1>
        <div className="welcome-folder-row" ref={measureFolderRef}>
          <button className="welcome-workspace" tabIndex={-1} type="button">
            <FolderIcon />
            <span>{workspaceName}</span>
            <ChevronIcon />
          </button>
        </div>
        <div className="welcome-composer-dock" ref={measureComposerRef}>
          <div className="welcome-composer">
            <textarea aria-hidden="true" className="welcome-input" readOnly rows={3} tabIndex={-1} value="" />
            <div className="welcome-toolbar">
              <div className="welcome-toolbar-left">
                <button className="welcome-model-trigger" tabIndex={-1} type="button"><span>Select model</span><ChevronIcon /></button>
                <span aria-hidden="true" className="welcome-toolbar-sep" />
                <button className="welcome-reasoning-label" tabIndex={-1} type="button"><strong>None</strong><ChevronIcon /></button>
                <button className="welcome-mode is-agent" tabIndex={-1} type="button"><span className="welcome-mode-icon"><ComposerCrownIcon /></span><span>Agent</span></button>
              </div>
              <button className="welcome-send" tabIndex={-1} type="button"><ComposerSendIcon /></button>
            </div>
          </div>
        </div>
        <div className="welcome-meta-row" ref={measureMetaRef}>
          <div className="welcome-version">
            <strong>{formatVersion(version)}</strong>
          </div>
          <div className="welcome-chat-count">
            <strong>{formatChatCount(displayChatCount)}</strong> {displayChatCount === 1 ? "chat" : "chats"}
          </div>
          <div className="welcome-token-count" title="Approximate tokens used in this folder">
            <WelcomeTokenBreakdown stats={tokenStats} />
          </div>
        </div>
      </div>

      <div className="welcome-stage">
        {visibility.banner || visibility.title ? (
          <div className="welcome-lockup">
            {visibility.banner ? <AsciiBanner /> : null}
            {visibility.title ? (
              <h1 className={`welcome-title${visibility.banner ? "" : " is-solo"}`}>
                <span>Welcome back, </span>
                <strong>{displayName}</strong>
              </h1>
            ) : null}
          </div>
        ) : null}

        {visibility.folder ? (
          <div
            className={`welcome-folder-row${openMenu === "workspace" ? " is-workspace-open" : ""}${openMenu === "model" ? " is-concealed" : ""}`}
          >
            {workspaceNameOverride ? (
              <button aria-label={`Cartella ${workspaceNameOverride}`} className="welcome-workspace" type="button">
                <FolderIcon />
                <span>{workspaceNameOverride}</span>
              </button>
            ) : <WorkspaceControl
              onOpenNewProject={onOpenNewProject}
              onOpenChange={(open) => setOpenMenu(open ? "workspace" : null)}
              onSelect={(project) => {
                setWorkspaceName(project?.name ?? "Home");
                setSelectedProject(project ?? null);
                onWorkspaceChange?.(project ?? null);
              }}
              onSelectTemporaryWorkspace={() => {
                onOpenTemporaryWorkspace?.();
              }}
              homeProject={projects.find((project) => project.name === "Home")}
              open={openMenu === "workspace"}
              projects={projects}
              temporaryWorkspace={temporaryWorkspace}
              workspaceName={workspaceName}
            />}
          </div>
        ) : null}

        {visibility.composer ? (
          <div className="welcome-composer-dock" ref={composerRef}>
            <ChatComposer
              aria-label="Ask Solomon"
              initialReasoning="none"
              isSending={isSending}
              onOpenMenuChange={(menu) => setOpenMenu(menu)}
              onSend={onSend}
              openMenu={openMenu === "model" || openMenu === "reasoning" ? openMenu : null}
              projectID={selectedProject?.id}
              resetKey={resetToken}
            />
            <div className="welcome-git-controls">
              <BranchControl
                directoryPath={isTemporaryWorkspaceActive ? temporaryWorkspace?.path : undefined}
                onOpenChange={(open) => setOpenMenu(open ? "branch" : null)}
                open={openMenu === "branch"}
                project={selectedProject}
              />
              <WorktreeControl
                directoryPath={isTemporaryWorkspaceActive ? temporaryWorkspace?.path : undefined}
                onOpenChange={(open) => setOpenMenu(open ? "worktree" : null)}
                onSelect={(worktree) => {
                  if (isTemporaryWorkspaceActive) {
                    onTemporaryWorkspacePathChange?.(worktree.path);
                    return;
                  }
                  const matched = projects.find((entry) => projectPathsMatch(entry.path, worktree.path));
                  if (!matched) return;
                  setWorkspaceName(matched.name);
                  setSelectedProject(matched);
                  onWorkspaceChange?.(matched);
                }}
                open={openMenu === "worktree"}
                project={selectedProject}
              />
            </div>
          </div>
        ) : null}
        {visibility.version || visibility.chatCount || visibility.tokenCount ? (
          <div className="welcome-meta-row">
            <div className="welcome-version">
              <strong>{formatVersion(version)}</strong>
            </div>
            <div className="welcome-chat-count">
              <strong>{formatChatCount(displayChatCount)}</strong> {displayChatCount === 1 ? "chat" : "chats"}
            </div>
            <div className="welcome-token-count" title="Approximate tokens used in this folder">
              <WelcomeTokenBreakdown stats={tokenStats} />
            </div>
          </div>
        ) : null}
      </div>
    </section>
  );
}

async function loadServerVersion(signal: AbortSignal): Promise<string> {
  const response = await fetch(await serverEndpoint("/health"), { cache: "no-store", signal });
  if (!response.ok) throw new Error(`Unable to load server version: ${response.status}`);
  const payload: unknown = await response.json();
  if (!payload || typeof payload !== "object" || !("server" in payload) || !payload.server || typeof payload.server !== "object" || !("version" in payload.server) || typeof payload.server.version !== "string") {
    return "dev";
  }
  return payload.server.version.trim() || "dev";
}

function formatVersion(value: string): string {
  const normalized = value.trim().replace(/^v/i, "");
  return `v${normalized || "dev"}`;
}

function formatChatCount(value: number): string {
  return Math.max(0, Math.round(value)).toLocaleString("en-US");
}

function WelcomeTokenBreakdown({ stats }: { stats: ProjectTokenStats }) {
  const total = stats.total || stats.user + stats.reasoning + stats.response;
  return (
    <span aria-label={`Approximate tokens: ${stats.user} user, ${stats.reasoning} reasoning, ${stats.response} response, ${total} total`} className="welcome-token-breakdown">
      <span className="welcome-token-approximation">~</span>
      <span className="welcome-token-part is-user">{formatTokenValue(stats.user)}</span>
      <i className="welcome-token-separator">+</i>
      <span className="welcome-token-part is-reasoning">{formatTokenValue(stats.reasoning)}</span>
      <i className="welcome-token-separator">+</i>
      <span className="welcome-token-part is-response">{formatTokenValue(stats.response)}</span>
      <i className="welcome-token-separator">=</i>
      <span className="welcome-token-part is-total">{formatTokenValue(total)}</span>
      <span className="welcome-token-label">tokens</span>
    </span>
  );
}

function formatTokenValue(value: number): string {
  return Math.max(0, Math.round(value)).toLocaleString("en-US");
}

function projectPathsMatch(left: string, right: string): boolean {
  const normalize = (value: string) => value.replace(/\\/g, "/").replace(/\/+$/, "").toLowerCase();
  return normalize(left) === normalize(right);
}

function WorkspaceControl({
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
  const controlRef = useRef<HTMLDivElement>(null);
  const recentProjects = [...projects]
    .sort((a, b) => latestProjectActivity(b) - latestProjectActivity(a))
    .slice(0, showAll ? undefined : 5);

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
        <div aria-label="Project directory" className="welcome-workspace-menu" role="menu">
          <div className="welcome-workspace-recents">
            {temporaryWorkspace ? (
              <button
                className="welcome-workspace-project is-temporary"
                onClick={() => {
                  onSelectTemporaryWorkspace?.();
                  onOpenChange(false);
                }}
                role="menuitem"
                title={temporaryWorkspace.displayPath}
                type="button"
              >
                <FolderIcon />
                <span>{temporaryWorkspace.name}</span>
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
            )) : <span className="welcome-workspace-empty">No recent projects</span>}
          </div>
          <button
            className="welcome-workspace-show-more"
            onClick={() => setShowAll(true)}
            role="menuitem"
            type="button"
          >
            Show more
          </button>
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

function AsciiBanner() {
  const lines = asciiBanner.trimEnd().split(/\r?\n/);
  return (
    <pre className="welcome-ascii-banner" aria-hidden="true">
      {lines.map((line, rowIndex) => {
        const colors = asciiColorRows[rowIndex] ?? [];
        const width = Math.max(line.length, colors.length);
        const padded = line.padEnd(width, " ");
        return (
          <span className="welcome-ascii-row" key={rowIndex}>
            {Array.from(padded).map((character, columnIndex) => (
              <span
                className="welcome-ascii-cell"
                key={columnIndex}
                style={{ color: logoColor(colors[columnIndex] ?? "000000") }}
              >
                {character === " " ? "\u00a0" : character}
              </span>
            ))}
          </span>
        );
      })}
    </pre>
  );
}

function logoColor(hex: string) {
  const channels = [0, 2, 4].map((offset) => Number.parseInt(hex.slice(offset, offset + 2), 16) / 255);
  const [red, green, blue] = channels;
  const max = Math.max(red, green, blue);
  const min = Math.min(red, green, blue);
  let lightness = (max + min) / 2;
  let hue = 0;
  let saturation = 0;
  if (max !== min) {
    const delta = max - min;
    saturation = delta / (lightness > 0.5 ? 2 - max - min : max + min);
    if (max === red) hue = ((green - blue) / delta + (blue > green ? 6 : 0)) / 6;
    else if (max === green) hue = ((blue - red) / delta + 2) / 6;
    else hue = ((red - green) / delta + 4) / 6;
  }
  saturation = Math.min(1, saturation * 1.38);
  if (saturation >= 0.05 && hue >= 0.065 && hue <= 0.23) {
    saturation = Math.min(1, saturation * 1.48);
    lightness = Math.min(0.94, lightness + (1 - lightness) * 0.11);
  }
  const hueChannel = (p: number, q: number, t: number) => {
    if (t < 0) t += 1;
    if (t > 1) t -= 1;
    if (t < 1 / 6) return p + (q - p) * 6 * t;
    if (t < 1 / 2) return q;
    if (t < 2 / 3) return p + (q - p) * (2 / 3 - t) * 6;
    return p;
  };
  const q = lightness < 0.5 ? lightness * (1 + saturation) : lightness + saturation - lightness * saturation;
  const p = 2 * lightness - q;
  const rgb = saturation === 0
    ? [lightness, lightness, lightness]
    : [hueChannel(p, q, hue + 1 / 3), hueChannel(p, q, hue), hueChannel(p, q, hue - 1 / 3)];
  const enhanced = rgb.map((channel) => Math.max(0, Math.min(255, Math.round(128 + 1.14 * (Math.round(channel * 255) - 128)))));
  return `rgb(${enhanced.join(" ")})`;
}

function FolderIcon() {
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

function ChevronIcon({ className }: { className?: string }) {
  return (
    <svg aria-hidden="true" className={`welcome-chevron${className ? ` ${className}` : ""}`} viewBox="0 0 24 24">
      <path d="m7 10 5 5 5-5" />
    </svg>
  );
}
