import { ProviderQuotaControl } from "../home/ProviderQuotaControl";
import { useGUIChatPreferencesState } from "../settings/chatPreferences";
import { composerDraftKey } from "../chat/composerDrafts";
import { NotificationControl } from "../notifications/Notifications";
import { useEffect, useLayoutEffect, useRef, useState, type CSSProperties } from "react";
import { AsciiBanner } from "./AsciiBanner";
export { AsciiBanner } from "./AsciiBanner";
import { WorkspaceControl, FolderIcon, ChevronIcon } from "./WorkspaceControl";
import { CatalogCounts, useCatalogCounts } from "./CatalogCounts";
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
import type { ComposerImageAttachment, ComposerTerminalClip } from "../chat/composerTypes";
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
  onSend?: (content: string, images?: ComposerImageAttachment[], clips?: ComposerTerminalClip[]) => void | Promise<void>;
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


export function Welcome({ bottomInset = 0, isSending = false, isTemporaryWorkspaceActive = false, isVisible = true, onComposerBoundsChange, onKeepAliveHeightChange, onOpenNewProject, onOpenTemporaryWorkspace, onTemporaryWorkspacePathChange, onSend, onWorkspaceChange, resetToken = 0, temporaryWorkspace = null, workspaceNameOverride = null, workspaceFocus = null }: WelcomeProps) {
  const { chat: chatPreferences } = useGUIChatPreferencesState();
  const catalogCounts = useCatalogCounts(isVisible);
  const [quotaProvider, setQuotaProvider] = useState("");
  const [userName, setUserName] = useState(() => getCachedUserName() ?? "");
  const [projects, setProjects] = useState<Project[]>(() => getCachedProjectSidebarData()?.projects ?? []);
  const [homeStats, setHomeStats] = useState(() => getCachedHomeStats());
  const [workspaceName, setWorkspaceName] = useState("Home");
  const [selectedProject, setSelectedProject] = useState<Project | null>(null);
  const [version, setVersion] = useState("");
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
  const isVisibleRef = useRef(isVisible);
  keepAliveHandlerRef.current = onKeepAliveHeightChange;
  workspaceFocusRef.current = workspaceFocus;
  workspaceNameOverrideRef.current = workspaceNameOverride;
  temporaryWorkspaceRef.current = temporaryWorkspace;
  temporaryWorkspaceActiveRef.current = isTemporaryWorkspaceActive;
  isVisibleRef.current = isVisible;

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
          // Selection is applied by the effects below from the latest props.
          // Applying it here races a click that has not re-rendered yet and
          // puts the previous project name back over the new branch.
        })
        .catch(() => {
          if (requestController.signal.aborted || currentController !== requestController) return;
          if (getCachedProjectSidebarData()) return;
          setProjects([]);
          setSelectedProject(null);
          if (isVisibleRef.current && !workspaceFocusRef.current) onWorkspaceChange?.(null);
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
    if (!isVisible) return;
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
  }, [isTemporaryWorkspaceActive, isVisible, onWorkspaceChange, projects, temporaryWorkspace, workspaceFocus, workspaceNameOverride]);

  useEffect(() => {
    if (!isVisible || !workspaceFocus) return;
    const focused = projects.find((project) => project.id === workspaceFocus.project.id) ?? workspaceFocus.project;
    setWorkspaceName(focused.name);
    setSelectedProject(focused);
    onWorkspaceChange?.(focused);
  }, [isVisible, onWorkspaceChange, projects, workspaceFocus]);

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
        // Preserve the last daemon version across a temporary interruption.
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
      // Keep the draft mounted when a phone keyboard leaves very little space.
      else next = { banner: false, title: false, folder: false, composer: true, version: false, chatCount: false, tokenCount: false };
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
  }, [catalogCounts.skills, catalogCounts.mcps, bottomInset, homeStats?.chatCount, homeStats?.tokenStats?.total, selectedProject?.chatCount, selectedProject?.tokenStats?.total, userName, version, workspaceNameOverride]);

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
          <CatalogCounts counts={catalogCounts} />
          <div className="welcome-token-count" title="Approximate tokens used in this folder">
            <WelcomeTokenTotal stats={tokenStats} />
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
              onProviderChange={setQuotaProvider}
              aria-label="Ask Solomon"
              initialReasoning="none"
              isSending={isSending}
              onOpenMenuChange={(menu) => setOpenMenu(menu)}
              onSend={onSend}
              openMenu={openMenu === "model" || openMenu === "reasoning" ? openMenu : null}
              projectID={selectedProject?.id}
              draftKey={composerDraftKey(isTemporaryWorkspaceActive ? temporaryWorkspace?.path : (workspaceFocus?.project ?? selectedProject)?.name === "Home" ? undefined : (workspaceFocus?.project ?? selectedProject)?.id)}
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
              {chatPreferences.composerQuotaBars ? <ProviderQuotaControl provider={quotaProvider} open={openMenu === "quota"} onOpenChange={(open) => setOpenMenu(open ? "quota" : null)} /> : null}
              <NotificationControl open={openMenu === "notifications"} onOpenChange={(open) => setOpenMenu(open ? "notifications" : null)} />
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
            <CatalogCounts counts={catalogCounts} />
            <div className="welcome-token-count" title="Approximate tokens used in this folder">
              <WelcomeTokenTotal stats={tokenStats} />
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
    throw new Error("Daemon health response has no version");
  }
  return payload.server.version.trim();
}

function formatVersion(value: string): string {
  const normalized = value.trim().replace(/^v/i, "");
  return normalized ? `v${normalized}` : "—";
}

function formatChatCount(value: number): string {
  return Math.max(0, Math.round(value)).toLocaleString("en-US");
}

function WelcomeTokenTotal({ stats }: { stats: ProjectTokenStats }) {
  const total = stats.total || stats.user + stats.reasoning + stats.response;
  return (
    <span aria-label={`Approximate total tokens: ${total}`} className="welcome-token-breakdown">
      <span className="welcome-token-approximation">~</span>
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
