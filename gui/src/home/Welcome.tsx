import { useEffect, useLayoutEffect, useRef, useState, type CSSProperties } from "react";
import asciiBanner from "../../../internal/logo/logo.txt?raw";
import asciiColors from "../../../internal/logo/colors.txt?raw";
import { serverEndpoint } from "../platform";
import {
  fetchProjectSidebarData,
  normalizeReasoningEffort,
  saveReasoningEffort,
  type Project,
  type ProjectTokenStats,
  type ReasoningEffort,
} from "../projects/projects";
import { ModelControl } from "./ModelControl";
import { BranchControl, WorktreeControl } from "./BranchControl";
import { AtMentionInput, type ComposerImageAttachment } from "./AtMentionInput";
import { testChatAtMentionEntries } from "../shell/RightSidePanel";
import "./welcome.css";
import "./welcome-reasoning.css";

const reasoningOptions = [
  { value: "none", label: "None" },
  { value: "low", label: "Low" },
  { value: "medium", label: "Medium" },
  { value: "high", label: "High" },
] as const;

const emptyProjectTokenStats: ProjectTokenStats = { user: 0, reasoning: 0, response: 0, total: 0 };

type WelcomeProps = {
  bottomInset?: number;
  isVisible?: boolean;
  onComposerBoundsChange?: (bounds: { left: number; right: number }) => void;
  onKeepAliveHeightChange?: (height: number) => void;
  onSend?: (content: string) => void;
  onWorkspaceChange?: (project: Project | null) => void;
  resetToken?: number;
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

export function Welcome({ bottomInset = 0, isVisible = true, onComposerBoundsChange, onKeepAliveHeightChange, onSend, onWorkspaceChange, resetToken = 0, workspaceNameOverride = null, workspaceFocus = null }: WelcomeProps) {
  const [userName, setUserName] = useState("");
  const [reasoning, setReasoning] = useState<ReasoningEffort>("none");
  const [projects, setProjects] = useState<Project[]>([]);
  const [workspaceName, setWorkspaceName] = useState("Home");
  const [selectedProject, setSelectedProject] = useState<Project | null>(null);
  const [selectedProvider, setSelectedProvider] = useState("");
  const [draft, setDraft] = useState("");
  const [images, setImages] = useState<ComposerImageAttachment[]>([]);
  const [fastOn, setFastOn] = useState(false);
  const [agentOn, setAgentOn] = useState(true);
  const [version, setVersion] = useState("dev");
  const [openMenu, setOpenMenu] = useState<"workspace" | "model" | "reasoning" | "branch" | "worktree" | null>(null);
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
  keepAliveHandlerRef.current = onKeepAliveHeightChange;
  workspaceFocusRef.current = workspaceFocus;
  workspaceNameOverrideRef.current = workspaceNameOverride;

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
    const controller = new AbortController();
    void fetchProjectSidebarData(controller.signal)
      .then((data) => {
        setUserName(data.userName.trim());
        setReasoning(data.reasoningEffort);
        setProjects(data.projects);
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
        const home = data.projects.find((project) => project.name === "Home") ?? null;
        setSelectedProject(home);
        onWorkspaceChange?.(home);
      })
      .catch(() => {
        setUserName("");
        setReasoning("none");
        setProjects([]);
        setSelectedProject(null);
        if (!workspaceFocusRef.current) onWorkspaceChange?.(null);
      });
    return () => controller.abort();
  }, [onWorkspaceChange]);

  useEffect(() => {
    if (workspaceNameOverride) {
      setWorkspaceName(workspaceNameOverride);
      setSelectedProject(null);
      return;
    }
    if (workspaceFocus) return;
    const home = projects.find((project) => project.name === "Home") ?? null;
    setWorkspaceName(home?.name ?? "Home");
    setSelectedProject(home);
    onWorkspaceChange?.(home);
  }, [onWorkspaceChange, projects, workspaceFocus, workspaceNameOverride]);

  useEffect(() => {
    if (!workspaceFocus) return;
    setWorkspaceName(workspaceFocus.project.name);
    setSelectedProject(workspaceFocus.project);
    onWorkspaceChange?.(workspaceFocus.project);
  }, [onWorkspaceChange, workspaceFocus]);

  useEffect(() => {
    setDraft("");
    setImages([]);
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
  }, [bottomInset, selectedProject?.chatCount, selectedProject?.tokenStats?.total, selectedProvider, userName, version]);

  const displayName = userName || "User";
  const tokenStats = selectedProject?.tokenStats ?? emptyProjectTokenStats;
  const fastAvailable = fastModeAvailableFor(selectedProvider);

  function send() {
    const content = draft.trim();
    if (!content || !onSend) return;
    onSend(content);
    setDraft("");
    setImages([]);
  }

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
            <textarea aria-hidden="true" className="welcome-input" readOnly rows={3} tabIndex={-1} value={draft} />
            <div className="welcome-toolbar">
              <div className="welcome-toolbar-left">
                <button className="welcome-model-trigger" tabIndex={-1} type="button"><span>Select model</span><ChevronIcon /></button>
                <span aria-hidden="true" className="welcome-toolbar-sep" />
                <button className="welcome-reasoning-label" tabIndex={-1} type="button"><strong>None</strong><ChevronIcon /></button>
                <button className="welcome-mode is-agent" tabIndex={-1} type="button"><span className="welcome-mode-icon"><BotIcon /></span><span>Agent</span></button>
              </div>
              <button className="welcome-send" tabIndex={-1} type="button"><SendIcon /></button>
            </div>
          </div>
        </div>
        <div className="welcome-meta-row" ref={measureMetaRef}>
          <div className="welcome-version">
            <strong>{formatVersion(version)}</strong>
          </div>
          <div className="welcome-chat-count">
            <strong>{formatChatCount(selectedProject?.chatCount ?? 0)}</strong> {selectedProject?.chatCount === 1 ? "chat" : "chats"}
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
            aria-hidden={openMenu === "model"}
            className={`welcome-folder-row${openMenu === "model" ? " is-concealed" : ""}`}
          >
            {workspaceNameOverride ? (
              <button aria-label={`Cartella ${workspaceNameOverride}`} className="welcome-workspace" type="button">
                <FolderIcon />
                <span>{workspaceNameOverride}</span>
              </button>
            ) : <WorkspaceControl
              onOpenChange={(open) => setOpenMenu(open ? "workspace" : null)}
              onSelect={(project) => {
                setWorkspaceName(project?.name ?? "Home");
                setSelectedProject(project ?? null);
                onWorkspaceChange?.(project ?? null);
              }}
              homeProject={projects.find((project) => project.name === "Home")}
              open={openMenu === "workspace"}
              projects={projects}
              workspaceName={workspaceName}
            />}
          </div>
        ) : null}

        {visibility.composer ? (
          <div className="welcome-composer-dock" ref={composerRef}>
            <form
              className="welcome-composer"
              onSubmit={(event) => {
                event.preventDefault();
                send();
              }}
            >
              <AtMentionInput
                aria-label="Ask Solomon"
                className="welcome-input"
                entries={workspaceNameOverride === "Test chats" ? testChatAtMentionEntries : undefined}
                images={images}
                onChange={setDraft}
                onImagesChange={setImages}
                onKeyDown={(event) => {
                  if (event.key === "Enter" && !event.shiftKey) {
                    event.preventDefault();
                    send();
                  }
                }}
                placeholder="Ask Solomon anything..."
                projectID={selectedProject?.id}
                value={draft}
              />
              <div className="welcome-toolbar">
                <div className="welcome-toolbar-left">
                  <ModelControl
                    onModelChange={(choice) => {
                      setSelectedProvider(choice.provider);
                      if (!fastModeAvailableFor(choice.provider)) setFastOn(false);
                    }}
                    onOpenChange={(open) => setOpenMenu(open ? "model" : null)}
                    open={openMenu === "model"}
                  />
                  <span aria-hidden="true" className="welcome-toolbar-sep" />
                  <div className="welcome-toolbar-modes">
                    <ReasoningControl
                      fastAvailable={fastAvailable}
                      fastOn={fastOn}
                      onChange={(value) => {
                        const previous = reasoning;
                        setReasoning(value);
                        void saveReasoningEffort(value).then(setReasoning).catch(() => setReasoning(previous));
                      }}
                      onFastChange={setFastOn}
                      onOpenChange={(open) => setOpenMenu(open ? "reasoning" : null)}
                      open={openMenu === "reasoning"}
                      value={reasoning}
                    />
                    <button
                      aria-pressed={agentOn}
                      className={`welcome-mode ${agentOn ? "is-agent" : "is-chat"}`}
                      onClick={() => setAgentOn((value) => !value)}
                      type="button"
                    >
                      <span aria-hidden="true" className="welcome-mode-icon">
                        {agentOn ? <CrownIcon /> : <ChatIcon />}
                      </span>
                      <span>{agentOn ? "Agent" : "Chat"}</span>
                    </button>
                  </div>
                </div>
                <button aria-label="Send" className="welcome-send" disabled={Boolean(onSend) && !draft.trim() && images.length === 0} type="submit">
                  <SendIcon />
                </button>
              </div>
            </form>
            <div className="welcome-git-controls">
              <BranchControl
                onOpenChange={(open) => setOpenMenu(open ? "branch" : null)}
                open={openMenu === "branch"}
                project={selectedProject}
              />
              <WorktreeControl
                onOpenChange={(open) => setOpenMenu(open ? "worktree" : null)}
                onSelect={(worktree) => {
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
              <strong>{formatChatCount(selectedProject?.chatCount ?? 0)}</strong> {selectedProject?.chatCount === 1 ? "chat" : "chats"}
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

function fastModeAvailableFor(provider: string): boolean {
  const normalized = provider.trim().toLowerCase();
  return normalized === "chatgpt sub"
    || normalized === "claude sub"
    || normalized.includes("anthropic")
    || normalized.includes("cursor");
}

function WorkspaceControl({
  workspaceName,
  projects,
  homeProject,
  open,
  onOpenChange,
  onSelect,
}: {
  workspaceName: string;
  projects: Project[];
  homeProject?: Project;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSelect: (project?: Project) => void;
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
            <button onClick={() => onOpenChange(false)} role="menuitem" type="button">
              <OpenProjectIcon />
              <span>Open Project</span>
            </button>
            <button onClick={() => onOpenChange(false)} role="menuitem" type="button">
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

function ReasoningControl({
  value,
  onChange,
  open,
  onOpenChange,
  fastAvailable = false,
  fastOn = false,
  onFastChange,
}: {
  value: ReasoningEffort;
  onChange: (value: ReasoningEffort) => void;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  fastAvailable?: boolean;
  fastOn?: boolean;
  onFastChange?: (on: boolean) => void;
}) {
  const [internalOpen, setInternalOpen] = useState(false);
  const isOpen = open ?? internalOpen;
  const setOpen = (next: boolean) => {
    onOpenChange?.(next);
    if (open === undefined) setInternalOpen(next);
  };
  const controlRef = useRef<HTMLDivElement>(null);
  const selectedIndex = Math.max(0, reasoningOptions.findIndex((option) => option.value === value));
  const selectedLabel = reasoningOptions[selectedIndex]?.label ?? "None";

  useEffect(() => {
    if (!isOpen) return;
    const close = (event: PointerEvent) => {
      if (!controlRef.current?.contains(event.target as Node)) setOpen(false);
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setOpen(false);
    };
    document.addEventListener("pointerdown", close);
    document.addEventListener("keydown", closeOnEscape);
    return () => {
      document.removeEventListener("pointerdown", close);
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, [isOpen]);

  return (
    <div className="welcome-reasoning" ref={controlRef}>
      <button
        aria-expanded={isOpen}
        aria-label={fastOn ? `Change reasoning level, fast mode on, ${selectedLabel}` : "Change reasoning level"}
        className="welcome-reasoning-label"
        onClick={() => setOpen(!isOpen)}
        type="button"
      >
        <strong className="welcome-reasoning-value">
          <span aria-hidden="true" className="welcome-reasoning-value-sizer">
            <span>Medium</span>
            {fastAvailable ? <span className="welcome-reasoning-fast-mark"><BoltIcon /></span> : null}
          </span>
          <span className="welcome-reasoning-value-text">
            <span>{selectedLabel}</span>
            {fastAvailable ? (
              <span aria-hidden={!fastOn} className={`welcome-reasoning-fast-mark${fastOn ? " is-on" : ""}`}>
                <BoltIcon />
              </span>
            ) : null}
          </span>
        </strong>
        <ChevronIcon className={isOpen ? "is-open" : undefined} />
      </button>
      {isOpen ? (
        <div className={`welcome-reasoning-popover${fastAvailable ? " has-fast" : ""}`}>
          <header>
            <span>Reasoning level</span>
            <strong>{selectedLabel}</strong>
          </header>
          <div className={`welcome-reasoning-row${fastAvailable ? " has-fast" : ""}`}>
            <div className="welcome-reasoning-scale">
              <input
                aria-label="Reasoning level"
                aria-valuetext={selectedLabel}
                max={reasoningOptions.length - 1}
                min={0}
                onChange={(event) => onChange(normalizeReasoningEffort(reasoningOptions[Number(event.target.value)]?.value ?? "none"))}
                step={1}
                style={{ "--reasoning-fill": `${(selectedIndex / (reasoningOptions.length - 1)) * 100}%` } as CSSProperties}
                type="range"
                value={selectedIndex}
              />
              <div aria-hidden="true" className="welcome-reasoning-ticks">
                {reasoningOptions.map((option, index) => (
                  <span className={index <= selectedIndex ? "is-reached" : undefined} key={option.value}>
                    <i />
                    <small>{option.label}</small>
                  </span>
                ))}
              </div>
            </div>
            {fastAvailable ? (
              <div className="welcome-reasoning-fast-slot">
                <button
                  aria-pressed={fastOn}
                  className={`welcome-reasoning-fast${fastOn ? " is-active" : ""}`}
                  onClick={() => onFastChange?.(!fastOn)}
                  type="button"
                >
                  <BoltIcon />
                  <span>Fast</span>
                </button>
              </div>
            ) : null}
          </div>
        </div>
      ) : null}
    </div>
  );
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

function OpenProjectIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2Z" />
      <path d="m13 11 3 3-3 3M16 14H9" />
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

function BoltIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="m13 2-8 12h6l-1 8 8-12h-6z" />
    </svg>
  );
}

function CrownIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="M12 1.8v16.2M9.4 4.4h5.2M4.4 15.8V12c0-1.6 1.2-2.5 2.6-2.5 1.4 0 2.4 1.1 2.6 2.5.3-2 1-3.6 2.4-3.6 1.4 0 2.1 1.6 2.4 3.6.2-1.4 1.2-2.5 2.6-2.5 1.4 0 2.6.9 2.6 2.5v3.8" />
      <path d="M4 16.2h16l-.6 3.8H4.6z" />
    </svg>
  );
}

function BotIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="M12 8V4H8" />
      <rect height="12" rx="2" width="16" x="4" y="8" />
      <path d="M2 14h2M20 14h2M15 13v2M9 13v2" />
    </svg>
  );
}

function ChatIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="M5 6.5A2.5 2.5 0 0 1 7.5 4h11A2.5 2.5 0 0 1 21 6.5v7A2.5 2.5 0 0 1 18.5 16H12l-4 3v-3H7.5A2.5 2.5 0 0 1 5 13.5V6.5Z" />
    </svg>
  );
}

function SendIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="M12 19V5M7 10l5-5 5 5" />
    </svg>
  );
}
