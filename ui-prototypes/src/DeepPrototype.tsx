import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState, type CSSProperties, type PointerEvent as ReactPointerEvent } from "react";
import { ArrowRight, Bot, Braces, Check, ChevronDown, ChevronRight, Code2, Copy, FileDiff, Folder, FolderOpen, GitBranch, History, Layers3, PanelLeft, Plus, Search, Settings, TerminalSquare, Wrench, X, Zap } from "lucide-react";
import deepAsciiBanner from "../../internal/logo/logo.txt?raw";
import deepAsciiColors from "../../internal/logo/colors.txt?raw";
import { ChangeStats, TextEditor } from "./shared";
import type { MockFile, PrototypeProps } from "./types";

function DeepMessage({ size = 19 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinejoin="round" strokeLinecap="round" aria-hidden="true">
      <path d="M5 6.5A2.5 2.5 0 0 1 7.5 4h11A2.5 2.5 0 0 1 21 6.5v7A2.5 2.5 0 0 1 18.5 16H12l-4 3v-3H7.5A2.5 2.5 0 0 1 5 13.5V6.5Z" />
    </svg>
  );
}

function DeepPuzzle({ size = 19 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="4 7 15 17" fill="none" stroke="currentColor" strokeWidth="1.25" strokeLinejoin="miter" strokeLinecap="butt" aria-hidden="true">
      <path d="M5 22H16V19A2.5 2.5 0 0 1 16 14V12H13A2.5 2.5 0 0 0 8 12H5V22Z" />
    </svg>
  );
}

function DeepCrown({ size = 15 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" strokeLinecap="round" aria-hidden="true">
      <path d="M12 1.8 12 18" />
      <path d="M9.4 4.4 14.6 4.4" />
      <path d="M4.4 15.8 4.4 12C4.4 10.4 5.6 9.5 7 9.5 8.4 9.5 9.4 10.6 9.6 12 9.9 10 10.6 8.4 12 8.4 13.4 8.4 14.1 10 14.4 12 14.6 10.6 15.6 9.5 17 9.5 18.4 9.5 19.6 10.4 19.6 12L19.6 15.8Z" />
      <path d="M4 16.2 20 16.2 19.4 20 4.6 20Z" />
    </svg>
  );
}

type DeepChatMessage = {
  id: number;
  role: "user" | "assistant";
  content: string;
  provider?: string;
  model?: string;
  reasoning?: string;
  toolCalls?: Array<{ name: string; detail: string; status: "done" | "running" }>;
  changedFiles?: Array<{ path: string; additions: number; deletions: number }>;
};

type DeepMockThread = {
  id: string;
  folder: string;
  title: string;
  lastInteraction: string;
  messages: DeepChatMessage[];
};

type DeepModelCatalog = { current: DeepModelChoice; recent: DeepModelChoice[]; providers: DeepModelGroup[] };
type DeepModelCatalogCache = DeepModelCatalog & { cachedAt: number };
type DeepGitGraphEntry = { id: string; parents: string[]; subject: string; references: string[] };
const deepModelCatalogCacheKey = "solomon.ui-prototype.deep-model-catalog.v1";
const deepModelCatalogCacheLifetime = 10 * 60 * 1000;

type DeepWorkspaceFolder = {
  name: string;
  path: string;
  folders: Map<string, DeepWorkspaceFolder>;
  files: string[];
};

function deepWorkspaceTree(files: string[]): DeepWorkspaceFolder {
  const root: DeepWorkspaceFolder = { name: "", path: "", folders: new Map(), files: [] };
  for (const filePath of files) {
    const parts = filePath.split("/");
    const fileName = parts.pop();
    if (!fileName) continue;
    let folder = root;
    for (const part of parts) {
      const path = folder.path ? `${folder.path}/${part}` : part;
      let next = folder.folders.get(part);
      if (!next) {
        next = { name: part, path, folders: new Map(), files: [] };
        folder.folders.set(part, next);
      }
      folder = next;
    }
    folder.files.push(fileName);
  }
  return root;
}

function deepWorkspaceFolderPaths(files: string[]) {
  const folders = new Set<string>();
  for (const filePath of files) {
    const parts = filePath.split("/");
    parts.pop();
    let path = "";
    for (const part of parts) {
      path = path ? `${path}/${part}` : part;
      folders.add(path);
    }
  }
  return folders;
}

function deepWorkspaceFolderStatuses(fileStatus: Record<string, string>) {
  const status: Record<string, string> = {};
  const priority: Record<string, number> = { R: 1, A: 2, U: 2, M: 3 };
  for (const [file, state] of Object.entries(fileStatus)) {
    if (!priority[state]) continue;
    const parts = file.split("/");
    parts.pop();
    let path = "";
    for (const part of parts) {
      path = path ? `${path}/${part}` : part;
      if ((priority[state] ?? 0) > (priority[status[path]] ?? 0)) status[path] = state;
    }
  }
  return status;
}

function deepFolderIcon(name: string, open: boolean, root = false) {
  const type = root ? "default_root_folder" : name === ".github" ? "folder_type_github" : name === ".solomon" ? "folder_type_config" : name === "docs" ? "folder_type_docs" : name === "internal" ? "folder_type_src" : name === "scripts" ? "folder_type_tools" : name === "test" ? "folder_type_test" : name === "ui-prototypes" ? "folder_type_library" : "default_folder";
  return <img className="deep-folder-icon" src={`/vscode-icons/${type}${open ? "_opened" : ""}.svg`} alt="" />;
}

function DeepWorkspaceFileIcon({ fileName }: { fileName: string }) {
  const icon = (/^licen[cs]e(?:\.[^.]+)?$/i.test(fileName) || fileName.endsWith(".svg")) ? "https://raw.githubusercontent.com/vscode-icons/vscode-icons/master/icons/file_type_svg.svg"
    : fileName.endsWith(".go") ? "go.svg"
    : fileName.endsWith(".json") ? "json.svg"
      : fileName.endsWith(".md") ? "markdown.svg"
        : fileName === ".gitignore" ? "git.svg"
          : fileName.endsWith(".env") || fileName === ".env" ? "env.svg"
            : fileName === "Makefile" ? "makefile.svg"
              : "file.svg";
  return <img className="deep-file-icon" src={icon.startsWith("http") ? icon : `/vscode-icons/${icon}`} alt="" />;
}

function DeepWorkspaceTree({ folder, depth = 0, collapsedFolders, toggleFolder, fileStatus, folderStatus, forceExpanded = false, onOpenFile, onOpenFileInNewTab, activeFilePath }: { folder: DeepWorkspaceFolder; depth?: number; collapsedFolders: Set<string>; toggleFolder: (path: string) => void; fileStatus: Record<string, string>; folderStatus: Record<string, string>; forceExpanded?: boolean; onOpenFile?: (path: string) => void; onOpenFileInNewTab?: (path: string) => void; activeFilePath?: string }) {
  const folders = Array.from(folder.folders.values()).sort((left, right) => left.name.localeCompare(right.name));
  const files = [...folder.files].sort((left, right) => left.localeCompare(right));
  return <>
    {folders.map((child) => {
      const collapsed = !forceExpanded && collapsedFolders.has(child.path);
      return <div className="deep-editor-tree-folder" key={child.path}>
        <button className="deep-editor-tree-row deep-editor-tree-folder-row" type="button" aria-expanded={!collapsed} onClick={() => toggleFolder(child.path)} style={{ paddingLeft: 8 + depth * 14 }}><ChevronRight className={collapsed ? "" : "open"} size={12} />{deepFolderIcon(child.name, !collapsed)}<span>{child.name}</span>{collapsed && folderStatus[child.path] ? <i className={`deep-editor-folder-status status-${folderStatus[child.path]}`} /> : null}</button>
        {collapsed ? null : <DeepWorkspaceTree folder={child} depth={depth + 1} collapsedFolders={collapsedFolders} toggleFolder={toggleFolder} fileStatus={fileStatus} folderStatus={folderStatus} forceExpanded={forceExpanded} onOpenFile={onOpenFile} onOpenFileInNewTab={onOpenFileInNewTab} activeFilePath={activeFilePath} />}
      </div>;
    })}
    {files.map((fileName) => {
      const path = folder.path ? `${folder.path}/${fileName}` : fileName;
      const status = fileStatus[path];
      const rowClass = `deep-editor-tree-row deep-editor-tree-file-row ${status ? `status-${status}` : ""}${activeFilePath === path ? " active" : ""}`;
      const rowStyle = { paddingLeft: 8 + depth * 14 };
      const rowContent = <><DeepWorkspaceFileIcon fileName={fileName} /><span className="deep-editor-tree-file-name">{fileName}</span>{status ? <i className={`deep-editor-git-status status-${status}`}>{status}</i> : null}</>;
      return onOpenFile
        ? <button className={rowClass} style={rowStyle} type="button" key={path} title={path} onClick={() => onOpenFile(path)} onDoubleClick={(event) => { event.preventDefault(); onOpenFileInNewTab?.(path); }}>{rowContent}</button>
        : <span className={rowClass} style={rowStyle} key={path} title={path}>{rowContent}</span>;
    })}
  </>;
}

const deepMockThreads: DeepMockThread[] = [
  {
    id: "auth-flow",
    folder: "Solomon / backend",
    title: "Review authentication flow",
    lastInteraction: "2m",
    messages: [
      { id: 101, role: "user", content: "Review the current authentication flow and point out the most important security risks." },
      { id: 102, role: "assistant", content: "The flow is generally clear, but token rotation and expired-session handling deserve attention. I would also make failed login responses indistinguishable and add explicit rate limits around recovery endpoints." },
      { id: 103, role: "user", content: "Turn those findings into a short implementation checklist." },
      { id: 104, role: "assistant", content: "Implemented refresh-token rotation and covered reuse and expiration with integration tests.", toolCalls: [{ name: "Search", detail: "authentication handlers and session storage", status: "done" }, { name: "Edit", detail: "rotate tokens and revoke reused sessions", status: "done" }, { name: "Test", detail: "go test ./internal/auth/...", status: "done" }], changedFiles: [{ path: "internal/auth/session.go", additions: 48, deletions: 12 }, { path: "test/auth_session_test.go", additions: 76, deletions: 3 }] },
    ],
  },
  {
    id: "api-errors",
    folder: "Solomon / backend",
    title: "Normalize API errors",
    lastInteraction: "18m",
    messages: [
      { id: 111, role: "user", content: "Propose a consistent error response for the public API." },
      { id: 112, role: "assistant", content: "Standardized public errors and removed internal provider details from responses.", toolCalls: [{ name: "Read", detail: "public API error responses", status: "done" }, { name: "Edit", detail: "introduce shared error envelope", status: "done" }], changedFiles: [{ path: "internal/server/errors.go", additions: 61, deletions: 29 }, { path: "internal/server/responses.go", additions: 14, deletions: 18 }] },
    ],
  },
  {
    id: "session-cleanup",
    folder: "Solomon / backend",
    title: "Clean up stale sessions",
    lastInteraction: "1h",
    messages: [
      { id: 121, role: "user", content: "How should stale chat sessions be cleaned up without deleting active work?" },
      { id: 122, role: "assistant", content: "Added recoverable cleanup for stale sessions while excluding pinned and locked work.", toolCalls: [{ name: "Tree", detail: "inspect chat persistence layout", status: "done" }, { name: "Edit", detail: "add archive-first cleanup policy", status: "done" }, { name: "Test", detail: "go test ./internal/chatstore/...", status: "done" }], changedFiles: [{ path: "internal/chatstore/cleanup.go", additions: 93, deletions: 0 }, { path: "internal/chatstore/cleanup_test.go", additions: 118, deletions: 0 }] },
    ],
  },
  {
    id: "responses-api-refactor",
    folder: "Solomon / backend",
    title: "Refactor Responses API",
    lastInteraction: "3h",
    messages: [
      { id: 501, role: "user", content: "Refactor the Responses API turn execution so synchronous, streaming, and background requests share the same core path. Preserve current behavior and add tests before changing implementation." },
      { id: 502, role: "assistant", content: "I mapped the three entry points and added characterization coverage for success, cancellation, persistence, and tool failures before touching the execution path.", toolCalls: [{ name: "Tree", detail: "inspect server response and session packages", status: "done" }, { name: "Read", detail: "compare sync, stream, and background turn execution", status: "done" }, { name: "Write", detail: "add characterization tests for existing behavior", status: "done" }, { name: "Test", detail: "go test ./internal/server/...", status: "done" }], changedFiles: [{ path: "internal/server/responses_test.go", additions: 184, deletions: 6 }] },
      { id: 503, role: "user", content: "Good. Keep streaming transport concerns outside the shared executor, especially flushing and event formatting." },
      { id: 504, role: "assistant", content: "Extracted a transport-neutral turn executor. Streaming now adapts internal events to SSE at the boundary, while synchronous and background paths consume the same result stream without depending on HTTP types.", toolCalls: [{ name: "Read", detail: "trace eventBuffer and SSE writer ownership", status: "done" }, { name: "Edit", detail: "extract transport-neutral executeTurn core", status: "done" }, { name: "Edit", detail: "adapt HTTP streaming at response boundary", status: "done" }, { name: "Test", detail: "run server unit and race tests", status: "done" }], changedFiles: [{ path: "internal/server/turn_executor.go", additions: 167, deletions: 0 }, { path: "internal/server/responses.go", additions: 52, deletions: 139 }, { path: "internal/server/events.go", additions: 31, deletions: 12 }] },
      { id: 505, role: "user", content: "What happens if the client disconnects after the assistant output is generated but before the session write completes?" },
      { id: 506, role: "assistant", content: "The request context was cancelling persistence along with delivery. I separated the bounded commit phase from the client connection while retaining cancellation for model generation. Once a completed assistant turn exists, the session write gets a short independent deadline.", toolCalls: [{ name: "Search", detail: "context propagation around session persistence", status: "done" }, { name: "Edit", detail: "add bounded post-generation commit context", status: "done" }, { name: "Write", detail: "cover disconnect between generation and commit", status: "done" }, { name: "Test", detail: "go test -race ./internal/server/...", status: "done" }], changedFiles: [{ path: "internal/server/turn_executor.go", additions: 39, deletions: 8 }, { path: "internal/server/responses_test.go", additions: 71, deletions: 2 }] },
      { id: 507, role: "user", content: "Run the broader suite and check whether the API documentation still matches the response lifecycle." },
      { id: 508, role: "assistant", content: "The broader suite passes. I updated the lifecycle documentation to distinguish generation cancellation from the final bounded commit, and documented that background requests use the same executor without an SSE adapter.", toolCalls: [{ name: "Test", detail: "go test ./...", status: "done" }, { name: "Read", detail: "compare API documentation with refactored lifecycle", status: "done" }, { name: "Edit", detail: "update response lifecycle documentation", status: "done" }], changedFiles: [{ path: "docs/architecture/responses-api.md", additions: 46, deletions: 19 }] },
      { id: 509, role: "user", content: "Summarize the final architecture and any remaining risk." },
      { id: 510, role: "assistant", content: "All three request modes now share one transport-neutral executor. Generation follows the caller context; completed turns receive a bounded persistence window; SSE formatting and flushing remain in the HTTP adapter. The main remaining risk is shutdown coordination for in-flight background commits, which should be handled with a server-level drain mechanism rather than inside the executor." },
    ],
  },
  {
    id: "search-latency",
    folder: "Solomon / search",
    title: "Reduce search latency",
    lastInteraction: "12m",
    messages: [
      { id: 201, role: "user", content: "Search feels slow when the workspace contains many files. Where would you start?" },
      { id: 202, role: "assistant", content: "I would measure directory traversal, content indexing, and result ranking separately. The likely quick wins are caching the file inventory, ignoring generated directories earlier, and streaming the first ranked matches instead of waiting for the complete result set." },
      { id: 203, role: "user", content: "Assume indexing is already cached. Explore the streaming option in more detail." },
      { id: 204, role: "assistant", content: "Implemented stable result batches and cancellation guards for stale queries.", toolCalls: [{ name: "Search", detail: "result ranking and query cancellation", status: "done" }, { name: "Edit", detail: "stream filename, path, then content matches", status: "done" }], changedFiles: [{ path: "internal/search/stream.go", additions: 84, deletions: 21 }, { path: "internal/search/rank.go", additions: 27, deletions: 9 }] },
    ],
  },
  {
    id: "search-filters",
    folder: "Solomon / search",
    title: "Design search filters",
    lastInteraction: "2h",
    messages: [
      { id: 211, role: "user", content: "Which filters would make workspace search more useful without making it complicated?" },
      { id: 212, role: "assistant", content: "Start with file type, path, modified state, and result kind. Present them as optional chips after the user begins typing, and preserve the last selection only within the current search session." },
    ],
  },
  {
    id: "ranking-tests",
    folder: "Solomon / search",
    title: "Test result ranking",
    lastInteraction: "1d",
    messages: [
      { id: 221, role: "user", content: "Create a compact plan for testing search ranking quality." },
      { id: 222, role: "assistant", content: "Created a fixed ranking corpus with reciprocal-rank assertions and a latency budget.", toolCalls: [{ name: "Write", detail: "ranking fixtures and benchmark cases", status: "done" }, { name: "Test", detail: "go test ./internal/search/... -run Ranking", status: "done" }], changedFiles: [{ path: "internal/search/testdata/ranking.json", additions: 142, deletions: 0 }, { path: "internal/search/rank_test.go", additions: 87, deletions: 4 }] },
    ],
  },
  {
    id: "release-notes",
    folder: "Home",
    title: "Draft release notes",
    lastInteraction: "34m",
    messages: [
      { id: 301, role: "user", content: "Draft friendly release notes for the new editor and the redesigned agent chat." },
      { id: 302, role: "assistant", content: "This release brings your code and conversations closer together. The new editor makes navigation calmer and more focused, while the redesigned agent chat gives long-running work more room to breathe. Switching between the two views keeps your context intact." },
      { id: 303, role: "user", content: "Make it more concise and less promotional." },
      { id: 304, role: "assistant", content: "Added a focused project editor and redesigned the agent conversation view. You can switch between editing and chat without losing the current workspace context." },
    ],
  },
  {
    id: "weekly-plan",
    folder: "Home",
    title: "Plan the next iteration",
    lastInteraction: "4h",
    messages: [
      { id: 311, role: "user", content: "Help me prioritize the next iteration: accessibility, search speed, and export." },
      { id: 312, role: "assistant", content: "Created an implementation plan that prioritizes accessibility, then measured search work, followed by export.", toolCalls: [{ name: "CreatePlan", detail: "next-iteration", status: "done" }, { name: "AddTodo", detail: "accessibility audit and keyboard pass", status: "done" }, { name: "AddTodo", detail: "search latency instrumentation", status: "done" }], changedFiles: [{ path: ".solomon/plans/next-iteration.md", additions: 54, deletions: 0 }] },
    ],
  },
  {
    id: "terminal-theme",
    folder: "Solomon / interface",
    title: "Tune terminal theme",
    lastInteraction: "8m",
    messages: [
      { id: 401, role: "user", content: "The terminal palette is too bright at night. Suggest a quieter direction without losing syntax contrast." },
      { id: 402, role: "assistant", content: "Lower the background luminance first, then desaturate secondary text and borders. Keep only two strong accents: amber for active agent states and green for successful code actions. Syntax colors can retain distinct hues at lower saturation if their lightness remains separated." },
      { id: 403, role: "user", content: "What should happen to the ASCII mark behind the conversation?" },
      { id: 404, role: "assistant", content: "Reduced the mark to a quiet background element during active conversations.", toolCalls: [{ name: "Edit", detail: "lower mark opacity and remove active glow", status: "done" }, { name: "Build", detail: "npm run build", status: "done" }], changedFiles: [{ path: "ui-prototypes/src/styles.css", additions: 11, deletions: 7 }] },
    ],
  },
  {
    id: "composer-spacing",
    folder: "Solomon / interface",
    title: "Refine composer spacing",
    lastInteraction: "27m",
    messages: [
      { id: 411, role: "user", content: "The final message feels crowded against the composer. What spacing rule should we use?" },
      { id: 412, role: "assistant", content: "Reserved a larger scroll tail so the final message can stop comfortably above the composer.", toolCalls: [{ name: "Edit", detail: "increase conversation bottom padding", status: "done" }, { name: "Build", detail: "npm run build", status: "done" }], changedFiles: [{ path: "ui-prototypes/src/styles.css", additions: 1, deletions: 1 }] },
    ],
  },
  {
    id: "sidebar-density",
    folder: "Solomon / interface",
    title: "Balance sidebar density",
    lastInteraction: "5h",
    messages: [
      { id: 421, role: "user", content: "Make the thread sidebar easier to scan when several chats share a folder." },
      { id: 422, role: "assistant", content: "Use folder headings as quiet anchors, indent text-only chat rows beneath them, and rely on a restrained active background instead of repeating icons. Truncate long titles but expose the full value on hover." },
    ],
  },
  {
    id: "prototype-five-polish",
    folder: "Solomon / interface",
    title: "Polish prototype five",
    lastInteraction: "2d",
    messages: [
      { id: 601, role: "user", content: "Polish prototype five for long agent sessions. Start by reviewing message width, composer overlap, folder navigation, and the visual weight of execution details." },
      { id: 602, role: "assistant", content: "I reviewed the conversation column and found four related issues: assistant text ignored the inner alignment, the final scroll position was too close to the composer, folder groups lacked hierarchy, and execution details competed with the final answer.", toolCalls: [{ name: "Read", detail: "inspect Deep prototype markup and styles", status: "done" }, { name: "Search", detail: "locate conversation, composer, and sidebar rules", status: "done" }], changedFiles: [] },
      { id: 603, role: "user", content: "Fix alignment and scrolling first. Keep the composer width as the reference." },
      { id: 604, role: "assistant", content: "Aligned the conversation column to the composer and added a matching inner inset for assistant content. The scroll tail now leaves enough room for the final message to rest clearly above the floating composer.", toolCalls: [{ name: "Edit", detail: "align message column and assistant inset", status: "done" }, { name: "Edit", detail: "increase bottom scroll reservation", status: "done" }, { name: "Build", detail: "npm run build", status: "done" }], changedFiles: [{ path: "ui-prototypes/src/styles.css", additions: 8, deletions: 6 }] },
      { id: 605, role: "user", content: "Now make the sidebar useful with several chats grouped by their source folder. Only show the final path segment." },
      { id: 606, role: "assistant", content: "Added grouped mock threads, basename-only folder labels, full-path tooltips, and independent collapse state. Open folders use the open-folder glyph; collapsed groups use the closed variant.", toolCalls: [{ name: "Edit", detail: "add grouped thread data and folder state", status: "done" }, { name: "Edit", detail: "style folder and thread hierarchy", status: "done" }, { name: "Build", detail: "npm run build", status: "done" }], changedFiles: [{ path: "ui-prototypes/src/App.tsx", additions: 126, deletions: 5 }, { path: "ui-prototypes/src/styles.css", additions: 19, deletions: 2 }] },
      { id: 607, role: "user", content: "The agent activity should reflect Solomon's Code Mode rather than a generic list of direct tools." },
      { id: 608, role: "assistant", content: "Reframed agent activity as one orchestrate execution. Deferred operations now appear as an internal sdk trace, followed by the files changed during that run, so the hierarchy matches Solomon's actual execution model.", toolCalls: [{ name: "Read", detail: "verify Code Mode and orchestrate conventions", status: "done" }, { name: "Edit", detail: "replace generic tool UI with Code Mode trace", status: "done" }, { name: "Build", detail: "npm run build", status: "done" }], changedFiles: [{ path: "ui-prototypes/src/App.tsx", additions: 42, deletions: 17 }, { path: "ui-prototypes/src/styles.css", additions: 37, deletions: 14 }] },
      { id: 609, role: "user", content: "Check the full experience at narrow and wide widths, including long file paths and enough messages to scroll." },
      { id: 610, role: "assistant", content: "The layout remains readable at both widths. Long paths truncate without displacing change counts, folder labels retain their tooltip, and the conversation scrolls independently while the composer stays anchored. I also expanded the mock data so the long-session behavior can be explored directly.", toolCalls: [{ name: "Build", detail: "run production build and type checking", status: "done" }, { name: "Search", detail: "audit overflow and truncation rules", status: "done" }], changedFiles: [{ path: "ui-prototypes/src/App.tsx", additions: 94, deletions: 0 }] },
    ],
  },
];

const deepGitGraphFallback = [
  ["refactor: disambiguate duplicated branch names", "main"],
  ["docs: update web UI server status", "SAPPHIR3-ROS3"],
  ["docs: remove oracle from roadmap", "SAPPHIR3-ROS3"],
  ["fix: preserve checkpoint prefixes in wrapped tools", ""],
  ["docs: document subagent lifecycle", "SAPPHIR3-ROS3"],
  ["fix: complete subagent lifecycle controls", "SAPPHIR3-ROS3"],
  ["docs: index benchmark role packages", "SAPPHIR3-ROS3"],
  ["chore: make build version resolution deterministic", ""],
  ["feat: add manual subagent role scoring", "SAPPHIR3-ROS3"],
  ["feat: refresh provider model catalogs and pickers", ""],
  ["Document internal/server in package index for CI", ""],
  ["Add subcommand serve HTTPS daemon with passkey auth", ""],
  ["Document internal/roles in package index for CI", ""],
  ["Add configurable subagent role pool with live progress", ""],
  ["Fix orchestrate module resolution for installed Solomon", ""],
  ["Use GH_TOKEN for updater GitHub API calls and retries", ""],
  ["Drop REPL upgrade smoke and keep CLI-only post-release", ""],
  ["Fix REPL upgrade smoke by pre-filling /upgrade branch", ""],
  ["Fix upgrade smoke install path and macOS Bash edge cases", ""],
  ["Harden Windows upgrade restart and expand release checks", ""],
  ["Run post-release smoke when release succeeds", ""],
  ["Fix upgrade smoke GitHub API auth and retries", ""],
  ["Skip release test suite when CI already passed on origin", ""],
] as const;
const deepGitGraphLaneColors = ["#2991e8", "#f0b429", "#a78bfa", "#62c7a2", "#ee7e9f"];

const deepFolderPathMaxLength = 72;

function deepFolderBaseName(path: string) {
  return path.split(/[\\/]/).map((part) => part.trim()).filter(Boolean).at(-1) ?? path;
}

function deepGitGraphLanes(entries: DeepGitGraphEntry[]) {
  let lanes: string[] = [];
  return entries.map((entry) => {
    let lane = lanes.indexOf(entry.id);
    if (lane < 0) {
      lane = 0;
      lanes.unshift(entry.id);
    }
    const lanesBefore = [...lanes];
    const laneCount = lanes.length;
    lanes.splice(lane, 1, ...entry.parents);
    lanes = lanes.filter((commit, index) => lanes.indexOf(commit) === index);
    const parentLanes = entry.parents.map((parent) => lanes.indexOf(parent)).filter((parentLane) => parentLane >= 0);
    const connections = lanesBefore.flatMap((commit, sourceLane) => {
      const targetLanes = commit === entry.id
        ? parentLanes
        : [lanes.indexOf(commit)].filter((targetLane) => targetLane >= 0);
      return targetLanes.map((targetLane) => ({ sourceLane, targetLane, colorLane: Math.max(sourceLane, targetLane) }));
    });
    return { ...entry, lane, laneCount, connections };
  });
}

function deepGitGraphConnectionPath(sourceX: number, targetX: number, startY: number, endY: number) {
  if (sourceX === targetX) return `M ${sourceX} ${startY} L ${targetX} ${endY}`;
  const bend = Math.min(9, (endY - startY) * .42);
  return `M ${sourceX} ${startY} C ${sourceX} ${startY + bend}, ${targetX} ${endY - bend}, ${targetX} ${endY}`;
}

function deepCodeModeToolName(name: string) {
  return ({
    Search: "sdk.Grep",
    Read: "sdk.ReadFile",
    Edit: "sdk.ReplaceInFile",
    Write: "sdk.WriteFile",
    Tree: "sdk.Tree",
    Test: "sdk.Shell",
    Build: "sdk.Shell",
    CreatePlan: "sdk.CreatePlan",
    AddTodo: "sdk.AddTodo",
  } as Record<string, string>)[name] ?? `sdk.${name}`;
}

function truncateDeepFolderPath(path: string) {
  if (path.length <= deepFolderPathMaxLength) return path;
  const visibleEdgeLength = Math.floor(deepFolderPathMaxLength / 2) - 2;
  return `${path.slice(0, visibleEdgeLength)}...${path.slice(-visibleEdgeLength)}`;
}

function abbreviateDeepHomePath(path: string) {
  const normalized = path.replace(/\\/g, "/");
  const homePrefix = normalized.match(/^(?:\/[Uu]sers|\/[Hh]ome|[A-Za-z]:\/[Uu]sers)\/[^/]+(?=\/|$)/)?.[0];
  return homePrefix ? `~${normalized.slice(homePrefix.length)}` : path;
}

function DeepProjectPath({ path, fullPath }: { path: string; fullPath: string }) {
  const text = useRef<HTMLSpanElement>(null);
  const [visiblePath, setVisiblePath] = useState(path);
  const [copied, setCopied] = useState(false);

  useLayoutEffect(() => {
    const element = text.current;
    if (!element) return;

    const metrics = document.createElement("span");
    const style = getComputedStyle(element);
    Object.assign(metrics.style, {
      position: "absolute",
      visibility: "hidden",
      whiteSpace: "nowrap",
      font: style.font,
      letterSpacing: style.letterSpacing,
    });
    document.body.append(metrics);

    const measure = (value: string) => {
      metrics.textContent = value;
      return metrics.getBoundingClientRect().width;
    };
    const update = () => {
      const availableWidth = element.clientWidth;
      if (measure(path) <= availableWidth) {
        setVisiblePath(path);
        return;
      }
      let index = 0;
      while (index < path.length && measure(`…${path.slice(index)}`) > availableWidth) index += 1;
      setVisiblePath(index < path.length ? `…${path.slice(index)}` : "…");
    };

    const observer = new ResizeObserver(update);
    observer.observe(element);
    update();
    return () => {
      observer.disconnect();
      metrics.remove();
    };
  }, [path]);

  const copyPath = async () => {
    try {
      await navigator.clipboard.writeText(fullPath);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1200);
    } catch {
      setCopied(false);
    }
  };

  return <div className="deep-chrome-project-path" aria-label={`Project path: ${fullPath}`}><span ref={text}>{visiblePath}</span><button type="button" onClick={() => void copyPath()} aria-label="Copy absolute project path" title={copied ? "Copied" : "Copy absolute path"}>{copied ? <Check size={15} /> : <Copy size={15} />}</button></div>;
}

type DeepModelChoice = { provider: string; model: string };
type DeepModelGroup = { provider: string; models: string[]; complete?: boolean };
const deepRecentProvider = "__recent__";
const deepReasoningOptions = [
  { value: "none", label: "None" },
  { value: "low", label: "Low" },
  { value: "medium", label: "Medium" },
  { value: "high", label: "High" },
] as const;

function normalizeDeepReasoning(value: string) {
  const normalized = value.trim().toLowerCase();
  if (normalized === "med") return "medium";
  return deepReasoningOptions.some((option) => option.value === normalized) ? normalized : "high";
}

function DeepOpenAIIcon({ size = 17 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
      <path d="M22.2819 9.8211a5.9847 5.9847 0 0 0-.5157-4.9108 6.0462 6.0462 0 0 0-6.5098-2.9A6.0651 6.0651 0 0 0 4.9807 4.1818a5.9847 5.9847 0 0 0-3.9977 2.9 6.0462 6.0462 0 0 0 .7427 7.0966 5.98 5.98 0 0 0 .511 4.9107 6.051 6.051 0 0 0 6.5146 2.9001A5.9847 5.9847 0 0 0 13.2599 24a6.0557 6.0557 0 0 0 5.7718-4.2058 5.9894 5.9894 0 0 0 3.9977-2.9001 6.0557 6.0557 0 0 0-.7475-7.0729zm-9.022 12.6081a4.4755 4.4755 0 0 1-2.8764-1.0408l.1419-.0804 4.7783-2.7582a.7948.7948 0 0 0 .3927-.6813v-6.7369l2.02 1.1686a.071.071 0 0 1 .038.052v5.5826a4.504 4.504 0 0 1-4.4945 4.4944zm-9.6607-4.1254a4.4708 4.4708 0 0 1-.5346-3.0137l.142.0852 4.783 2.7582a.7712.7712 0 0 0 .7806 0l5.8428-3.3685v2.3324a.0804.0804 0 0 1-.0332.0615L9.74 19.9502a4.4992 4.4992 0 0 1-6.1408-1.6464zM2.3408 7.8956a4.485 4.485 0 0 1 2.3655-1.9728V11.6a.7664.7664 0 0 0 .3879.6765l5.8144 3.3543-2.0201 1.1685a.0757.0757 0 0 1-.071 0l-4.8303-2.7865A4.504 4.504 0 0 1 2.3408 7.872zm16.5963 3.8558L13.1038 8.364 15.1192 7.2a.0757.0757 0 0 1 .071 0l4.8303 2.7913a4.4944 4.4944 0 0 1-.6765 8.1042v-5.6772a.79.79 0 0 0-.407-.667zm2.0107-3.0231-.142-.0852-4.7735-2.7818a.7759.7759 0 0 0-.7854 0L9.409 9.2297V6.8974a.0662.0662 0 0 1 .0284-.0615l4.8303-2.7866a4.4992 4.4992 0 0 1 6.6802 4.66zM8.3065 12.863l-2.02-1.1638a.0804.0804 0 0 1-.038-.0567V6.0742a4.4992 4.4992 0 0 1 7.3757-3.4537l-.142.0805L8.704 5.459a.7948.7948 0 0 0-.3927.6813zm1.0976-2.3654 2.602-1.4998 2.6069 1.4998v2.9994l-2.5974 1.4997-2.6067-1.4997Z" />
    </svg>
  );
}

function DeepAnthropicIcon({ size = 17 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
      <path d="M17.3041 3.541h-3.6718l6.696 16.918H24Zm-10.6082 0L0 20.459h3.7442l1.3693-3.5527h7.0052l1.3693 3.5528h3.7442L10.5363 3.5409Zm-.3712 10.2232 2.2914-5.9456 2.2914 5.9456Z" />
    </svg>
  );
}

function DeepCursorIcon({ size = 17 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 466.73 532.09" fill="currentColor" aria-hidden="true">
      <path d="M457.43 125.94 244.42 2.96c-6.84-3.95-15.28-3.95-22.12 0L9.3 125.94C3.55 129.26 0 135.4 0 142.05v247.99c0 6.65 3.55 12.79 9.3 16.11l213.01 122.98c6.84 3.95 15.28 3.95 22.12 0l213.01-122.98c5.75-3.32 9.3-9.46 9.3-16.11V142.05c0-6.65-3.55-12.79-9.3-16.11h-.01ZM444.05 151.99 238.42 508.15c-1.39 2.4-5.06 1.42-5.06-1.36V273.58c0-4.66-2.49-8.97-6.53-11.31L24.87 145.67c-2.4-1.39-1.42-5.06 1.36-5.06h411.26c5.84 0 9.49 6.33 6.57 11.39h-.01Z" />
    </svg>
  );
}

function DeepProviderIcon({ provider }: { provider: string }) {
  const normalized = provider.toLowerCase();
  if (normalized.includes("chatgpt") || normalized.includes("openai")) return <DeepOpenAIIcon />;
  if (normalized.includes("claude") || normalized.includes("anthropic")) return <DeepAnthropicIcon />;
  if (normalized.includes("cursor")) return <DeepCursorIcon />;
  if (normalized.includes("studio") || normalized.includes("local")) return <TerminalSquare size={16} />;
  if (normalized.includes("router")) return <Layers3 size={16} />;
  return <Braces size={16} />;
}

const deepLoremWords = `Lorem ipsum dolor sit amet, consectetur adipiscing elit. Integer posuere erat a ante venenatis dapibus posuere velit aliquet. Aenean lacinia bibendum nulla sed consectetur. Maecenas faucibus mollis interdum. Donec ullamcorper nulla non metus auctor fringilla. Curabitur blandit tempus porttitor. Praesent commodo cursus magna, vel scelerisque nisl consectetur et. Vestibulum id ligula porta felis euismod semper. Cras mattis consectetur purus sit amet fermentum. Morbi leo risus, porta ac consectetur ac, vestibulum at eros.`.split(/\s+/);
const deepAsciiColorRows = deepAsciiColors.trim().split(/\r?\n/).map((row) => row.trim().split(/\s+/));

function deepTerminalLogoColor(hex: string) {
  const channels = [0, 2, 4].map((offset) => Number.parseInt(hex.slice(offset, offset + 2), 16) / 255);
  const [red, green, blue] = channels;
  const max = Math.max(red, green, blue);
  const min = Math.min(red, green, blue);
  let lightness = (max + min) / 2;
  let hue = 0;
  let saturation = 0;
  if (max !== min) {
    const delta = max - min;
    saturation = delta / (lightness > .5 ? 2 - max - min : max + min);
    if (max === red) hue = ((green - blue) / delta + (blue > green ? 6 : 0)) / 6;
    else if (max === green) hue = ((blue - red) / delta + 2) / 6;
    else hue = ((red - green) / delta + 4) / 6;
  }
  saturation = Math.min(1, saturation * 1.38);
  if (saturation >= .05 && hue >= .065 && hue <= .23) {
    saturation = Math.min(1, saturation * 1.48);
    lightness = Math.min(.94, lightness + (1 - lightness) * .11);
  }
  const hueChannel = (p: number, q: number, t: number) => {
    if (t < 0) t += 1;
    if (t > 1) t -= 1;
    if (t < 1 / 6) return p + (q - p) * 6 * t;
    if (t < 1 / 2) return q;
    if (t < 2 / 3) return p + (q - p) * (2 / 3 - t) * 6;
    return p;
  };
  const q = lightness < .5 ? lightness * (1 + saturation) : lightness + saturation - lightness * saturation;
  const p = 2 * lightness - q;
  const rgb = saturation === 0
    ? [lightness, lightness, lightness]
    : [hueChannel(p, q, hue + 1 / 3), hueChannel(p, q, hue), hueChannel(p, q, hue - 1 / 3)];
  const enhanced = rgb.map((channel) => Math.max(0, Math.min(255, Math.round(128 + 1.14 * (Math.round(channel * 255) - 128)))));
  return `rgb(${enhanced.join(" ")})`;
}

function DeepAsciiBanner() {
  return (
    <pre className="deep-ascii-banner" aria-hidden="true">
      {deepAsciiBanner.trimEnd().split(/\r?\n/).map((line, rowIndex) => (
        <span className="deep-ascii-row" key={rowIndex}>
          {Array.from(line).map((character, columnIndex) => (
            <span className="deep-ascii-cell" style={{ color: deepTerminalLogoColor(deepAsciiColorRows[rowIndex]?.[columnIndex] ?? "ffc704") }} key={columnIndex}>{character}</span>
          ))}
        </span>
      ))}
    </pre>
  );
}

function loremForWordCount(wordCount: number) {
  return Array.from({ length: wordCount }, (_, index) => deepLoremWords[index % deepLoremWords.length]);
}

function deepGitStatusLabel(code: string) {
  if (code === "M") return "modified";
  if (code === "A" || code === "U") return "added";
  if (code === "D") return "deleted";
  if (code === "R") return "renamed";
  return "clean";
}

function deepWorkspaceFileLabel(filePath: string, gitStatus: Record<string, string>) {
  return deepGitStatusLabel(gitStatus[filePath] ?? "");
}

function deepMockFileCache(files: MockFile[]) {
  return Object.fromEntries(files.map((file) => [file.path, file]));
}

export function DeepPrototype(props: PrototypeProps) {
  const { config, mode, setMode, conversation, openFile } = props;
  const [sideCollapsed, setSideCollapsed] = useState(false);
  const [editorSideCollapsed, setEditorSideCollapsed] = useState(false);
  const [editorSideWidth, setEditorSideWidth] = useState(248);
  const [editorSideResizing, setEditorSideResizing] = useState(false);
  const [deepGraphHeight, setDeepGraphHeight] = useState(220);
  const [deepGraphResizing, setDeepGraphResizing] = useState(false);
  const [editorSideView, setEditorSideView] = useState<"files" | "git">("files");
  const [deepEditorTabs, setDeepEditorTabs] = useState(() => conversation.change_paths.slice(0, 3).map((path, index) => ({ id: `editor-tab-${index}`, path })));
  const [deepActiveEditorTabId, setDeepActiveEditorTabId] = useState("editor-tab-0");
  const [deepEditorFileCache, setDeepEditorFileCache] = useState<Record<string, MockFile>>(() => deepMockFileCache(config.files));
  const [deepEditorFileLoading, setDeepEditorFileLoading] = useState<string | null>(null);
  const [deepDraggedEditorTabId, setDeepDraggedEditorTabId] = useState<string | null>(null);
  const [deepWorkspaceFiles, setDeepWorkspaceFiles] = useState<string[]>([]);
  const [deepWorkspaceFileStatus, setDeepWorkspaceFileStatus] = useState<Record<string, string>>({});
  const [deepStagedFileStatus, setDeepStagedFileStatus] = useState<Record<string, string>>({});
  const [deepChangedFileStatus, setDeepChangedFileStatus] = useState<Record<string, string>>({});
  const [deepWorkspaceQuery, setDeepWorkspaceQuery] = useState("");
  const [deepWorkspaceSearchResults, setDeepWorkspaceSearchResults] = useState<string[] | null>(null);
  const [deepCollapsedEditorFolders, setDeepCollapsedEditorFolders] = useState<Set<string>>(() => new Set());
  const [deepCollapsedGitFolders, setDeepCollapsedGitFolders] = useState<Set<string>>(() => new Set());
  const [deepStagedChangesCollapsed, setDeepStagedChangesCollapsed] = useState(false);
  const [deepChangesCollapsed, setDeepChangesCollapsed] = useState(false);
  const [deepGraphCollapsed, setDeepGraphCollapsed] = useState(false);
  const [deepGitGraphEntries, setDeepGitGraphEntries] = useState<DeepGitGraphEntry[]>(() => deepGitGraphFallback.map(([subject, reference], index) => ({ id: `fallback-${index}`, parents: [], subject, references: reference ? [reference] : [] })));
  const [deepCommitMessage, setDeepCommitMessage] = useState("");
  const [userName, setUserName] = useState(config.user_name ?? "");
  const [editingUserName, setEditingUserName] = useState(false);
  const [userNameError, setUserNameError] = useState(false);
  const [deepDraft, setDeepDraft] = useState("");
  const [deepMessages, setDeepMessages] = useState<DeepChatMessage[]>([]);
  const [deepActiveThread, setDeepActiveThread] = useState<string | null>(null);
  const [deepCollapsedFolders, setDeepCollapsedFolders] = useState<Set<string>>(() => new Set());
  const [deepFolderOrder, setDeepFolderOrder] = useState(() => Array.from(new Set(deepMockThreads.map((thread) => thread.folder))));
  const [deepDraggedFolder, setDeepDraggedFolder] = useState<string | null>(null);
  const [deepStream, setDeepStream] = useState<{ messageId: number; words: string[]; revealed: number } | null>(null);
  const [deepModelGroups, setDeepModelGroups] = useState<DeepModelGroup[]>([{ provider: "Mock config", models: [config.session.model], complete: false }]);
  const [deepConfiguredRecents, setDeepConfiguredRecents] = useState<DeepModelChoice[]>([{ provider: "Mock config", model: config.session.model }]);
  const [deepSelectedModel, setDeepSelectedModel] = useState<DeepModelChoice>({ provider: "Mock config", model: config.session.model });
  const [deepModelOpen, setDeepModelOpen] = useState(false);
  const [deepModelsError, setDeepModelsError] = useState(false);
  const [deepActiveProvider, setDeepActiveProvider] = useState(deepRecentProvider);
  const [deepModelQuery, setDeepModelQuery] = useState("");
  const [deepReasoning, setDeepReasoning] = useState(() => normalizeDeepReasoning(config.session.reasoning_effort));
  const [deepReasoningOpen, setDeepReasoningOpen] = useState(false);
  const [deepFastMode, setDeepFastMode] = useState(config.session.fast_mode);
  const [deepMessageMode, setDeepMessageMode] = useState<"agent" | "chat">("agent");
  const [deepFolder, setDeepFolder] = useState("Home");
  const [deepFolderOpen, setDeepFolderOpen] = useState(false);
  const [deepBranch, setDeepBranch] = useState(config.workspace.branch);
  const [deepBranches, setDeepBranches] = useState(() => config.workspace.branch ? [config.workspace.branch] : []);
  const [deepBranchOpen, setDeepBranchOpen] = useState(false);
  const userNameInput = useRef<HTMLInputElement>(null);
  const deepComposerInput = useRef<HTMLTextAreaElement>(null);
  const deepConversationScroll = useRef<HTMLDivElement>(null);
  const deepWelcomeLockup = useRef<HTMLDivElement>(null);
  const deepWelcomeStartRect = useRef<DOMRect | null>(null);
  const deepModelControl = useRef<HTMLDivElement>(null);
  const deepReasoningControl = useRef<HTMLDivElement>(null);
  const deepFolderControl = useRef<HTMLDivElement>(null);
  const deepMessageId = useRef(0);
  const deepEditorTabId = useRef(3);
  const deepEditorFileCacheRef = useRef(deepEditorFileCache);
  const deepEditorTabsRef = useRef<HTMLDivElement>(null);
  const deepEditorTreeClickTimer = useRef<number | undefined>(undefined);
  const savedUserName = useRef(config.user_name ?? "");
  const deepWorkspaceHasLoaded = useRef(false);

  useEffect(() => {
    deepEditorFileCacheRef.current = deepEditorFileCache;
  }, [deepEditorFileCache]);

  const refreshDeepWorkspace = useCallback(async () => {
    try {
      const response = await fetch("/__solomon/workspace-files");
      if (!response.ok) throw new Error(`Unable to read workspace files (${response.status})`);
      const { files, status, staged, changes } = await response.json() as { files: string[]; status?: Record<string, string>; staged?: Record<string, string>; changes?: Record<string, string> };
      const workspaceFiles = Array.isArray(files) ? files.filter((file): file is string => typeof file === "string") : [];
      setDeepWorkspaceFiles(workspaceFiles);
      setDeepWorkspaceFileStatus(status && typeof status === "object" ? status : {});
      setDeepStagedFileStatus(staged && typeof staged === "object" ? staged : {});
      setDeepChangedFileStatus(changes && typeof changes === "object" ? changes : {});
      if (!deepWorkspaceHasLoaded.current) {
        deepWorkspaceHasLoaded.current = true;
        setDeepCollapsedEditorFolders(deepWorkspaceFolderPaths(workspaceFiles));
      }
    } catch {
      if (!deepWorkspaceHasLoaded.current) {
        setDeepWorkspaceFiles([]);
        setDeepWorkspaceFileStatus({});
        setDeepStagedFileStatus({});
        setDeepChangedFileStatus({});
        setDeepCollapsedEditorFolders(new Set());
      }
    }
  }, []);

  useEffect(() => {
    fetch("/__solomon/user-name")
      .then((response) => {
        if (!response.ok) throw new Error(`Unable to read user name (${response.status})`);
        return response.json() as Promise<{ user_name: string }>;
      })
      .then(({ user_name }) => {
        savedUserName.current = user_name;
        setUserName(user_name);
      })
      .catch(() => setUserNameError(true));
  }, []);

  useEffect(() => {
    const query = deepWorkspaceQuery.trim();
    if (!query) {
      setDeepWorkspaceSearchResults(null);
      return;
    }
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      fetch(`/__solomon/workspace-search?q=${encodeURIComponent(query)}`, { signal: controller.signal })
        .then((response) => {
          if (!response.ok) throw new Error(`Unable to search workspace (${response.status})`);
          return response.json() as Promise<{ files: string[] }>;
        })
        .then(({ files }) => setDeepWorkspaceSearchResults(Array.isArray(files) ? files.filter((file): file is string => typeof file === "string") : []))
        .catch((error: unknown) => { if (error instanceof DOMException && error.name === "AbortError") return; setDeepWorkspaceSearchResults([]); });
    }, 180);
    return () => {
      controller.abort();
      window.clearTimeout(timer);
    };
  }, [deepWorkspaceQuery]);

  useEffect(() => {
    const applyCatalog = ({ current, recent, providers }: DeepModelCatalog) => {
      const groups = Array.isArray(providers) ? providers.filter((group) => group.provider && Array.isArray(group.models) && group.models.length) : [];
      if (!groups.length) throw new Error("No configured models");
      setDeepModelGroups(groups);
      setDeepConfiguredRecents(Array.isArray(recent) ? recent.filter((choice) => choice.provider && choice.model) : []);
      const currentExists = groups.some((group) => group.provider === current?.provider && group.models.includes(current?.model));
      const selected = currentExists ? current : { provider: groups[0].provider, model: groups[0].models[0] };
      setDeepSelectedModel(selected);
      setDeepActiveProvider(deepRecentProvider);
      setDeepModelsError(false);
    };
    try {
      const cached = JSON.parse(window.sessionStorage.getItem(deepModelCatalogCacheKey) ?? "null") as DeepModelCatalogCache | null;
      if (cached && Date.now() - cached.cachedAt < deepModelCatalogCacheLifetime) {
        applyCatalog(cached);
        return;
      }
    } catch {
      window.sessionStorage.removeItem(deepModelCatalogCacheKey);
    }
    fetch("/__solomon/models")
      .then((response) => {
        if (!response.ok) throw new Error(`Unable to read models (${response.status})`);
        return response.json() as Promise<DeepModelCatalog>;
      })
      .then((catalog) => {
        applyCatalog(catalog);
        window.sessionStorage.setItem(deepModelCatalogCacheKey, JSON.stringify({ ...catalog, cachedAt: Date.now() }));
      })
      .catch(() => setDeepModelsError(true));
  }, []);

  useEffect(() => {
    fetch("/__solomon/branches")
      .then((response) => {
        if (!response.ok) throw new Error(`Unable to read branches (${response.status})`);
        return response.json() as Promise<{ current: string; branches: string[] }>;
      })
      .then(({ current, branches }) => {
        const actualBranches = Array.isArray(branches) ? branches.filter((branch): branch is string => typeof branch === "string" && Boolean(branch.trim())) : [];
        const ordered = Array.from(new Set(actualBranches.map((branch) => branch.trim())))
          .sort((left, right) => left === "main" ? -1 : right === "main" ? 1 : left.localeCompare(right));
        if (ordered.length) setDeepBranches(ordered);
        if (current && ordered.includes(current)) setDeepBranch(current);
      })
      .catch(() => {
        setDeepBranches(config.workspace.branch ? [config.workspace.branch] : []);
      });
  }, [config.workspace.branch]);

  useEffect(() => {
    void refreshDeepWorkspace();
  }, [refreshDeepWorkspace]);

  useEffect(() => {
    if (mode !== "editor" || editorSideView !== "git") return;
    const refresh = () => void refreshDeepWorkspace();
    const refreshWhenVisible = () => { if (document.visibilityState === "visible") refresh(); };
    refresh();
    const interval = window.setInterval(refresh, 2_000);
    window.addEventListener("focus", refresh);
    document.addEventListener("visibilitychange", refreshWhenVisible);
    return () => {
      window.clearInterval(interval);
      window.removeEventListener("focus", refresh);
      document.removeEventListener("visibilitychange", refreshWhenVisible);
    };
  }, [editorSideView, mode, refreshDeepWorkspace]);

  useEffect(() => {
    fetch("/__solomon/git-history")
      .then((response) => {
        if (!response.ok) throw new Error(`Unable to read Git history (${response.status})`);
        return response.json() as Promise<{ commits?: DeepGitGraphEntry[] }>;
      })
      .then(({ commits }) => {
        if (Array.isArray(commits) && commits.length) setDeepGitGraphEntries(commits.filter((commit) => commit && typeof commit.id === "string" && typeof commit.subject === "string" && Array.isArray(commit.parents) && Array.isArray(commit.references)));
      })
      .catch(() => {});
  }, []);

  useEffect(() => {
    if (editingUserName) userNameInput.current?.select();
  }, [editingUserName]);

  useEffect(() => {
    const keepEditorPanelWithinViewport = () => setEditorSideWidth((width) => Math.min(width, window.innerWidth / 3));
    keepEditorPanelWithinViewport();
    window.addEventListener("resize", keepEditorPanelWithinViewport);
    return () => window.removeEventListener("resize", keepEditorPanelWithinViewport);
  }, []);

  useEffect(() => {
    if (!deepModelOpen) return;
    const closeModelMenu = (event: PointerEvent) => {
      if (!deepModelControl.current?.contains(event.target as Node)) setDeepModelOpen(false);
    };
    const closeModelMenuOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setDeepModelOpen(false);
    };
    document.addEventListener("pointerdown", closeModelMenu);
    document.addEventListener("keydown", closeModelMenuOnEscape);
    return () => {
      document.removeEventListener("pointerdown", closeModelMenu);
      document.removeEventListener("keydown", closeModelMenuOnEscape);
    };
  }, [deepModelOpen]);

  useEffect(() => {
    if (!deepReasoningOpen) return;
    const closeReasoning = (event: PointerEvent) => {
      if (!deepReasoningControl.current?.contains(event.target as Node)) setDeepReasoningOpen(false);
    };
    const closeReasoningOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setDeepReasoningOpen(false);
    };
    document.addEventListener("pointerdown", closeReasoning);
    document.addEventListener("keydown", closeReasoningOnEscape);
    return () => {
      document.removeEventListener("pointerdown", closeReasoning);
      document.removeEventListener("keydown", closeReasoningOnEscape);
    };
  }, [deepReasoningOpen]);

  useEffect(() => {
    if (!deepFolderOpen && !deepBranchOpen) return;
    const closeFolderMenu = (event: PointerEvent) => {
      if (!deepFolderControl.current?.contains(event.target as Node)) {
        setDeepFolderOpen(false);
        setDeepBranchOpen(false);
      }
    };
    const closeFolderMenuOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setDeepFolderOpen(false);
        setDeepBranchOpen(false);
      }
    };
    document.addEventListener("pointerdown", closeFolderMenu);
    document.addEventListener("keydown", closeFolderMenuOnEscape);
    return () => {
      document.removeEventListener("pointerdown", closeFolderMenu);
      document.removeEventListener("keydown", closeFolderMenuOnEscape);
    };
  }, [deepFolderOpen, deepBranchOpen]);

  useEffect(() => {
    if (!deepStream) return;
    const delay = deepStream.revealed === 0 ? 260 : 62;
    const timer = window.setTimeout(() => {
      const revealed = deepStream.revealed + 1;
      const content = deepStream.words.slice(0, revealed).join(" ");
      setDeepMessages((messages) => messages.map((message) => (
        message.id === deepStream.messageId ? { ...message, content } : message
      )));
      setDeepStream(revealed >= deepStream.words.length ? null : { ...deepStream, revealed });
    }, delay);
    return () => window.clearTimeout(timer);
  }, [deepStream]);

  useEffect(() => {
    const scroll = deepConversationScroll.current;
    if (scroll) scroll.scrollTo({ top: scroll.scrollHeight, behavior: deepStream ? "smooth" : "auto" });
  }, [deepMessages, deepStream]);

  const finishUserNameEdit = async () => {
    const next = userName.trim();
    setEditingUserName(false);
    try {
      const response = await fetch("/__solomon/user-name", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ user_name: next }),
      });
      if (!response.ok) throw new Error(`Unable to save user name (${response.status})`);
      savedUserName.current = next;
      setUserName(next);
      setUserNameError(false);
    } catch {
      setUserName(savedUserName.current);
      setUserNameError(true);
    }
  };

  const sendDeepMessage = () => {
    const prompt = deepDraft.trim();
    if (!prompt || deepStream) return;
    deepWelcomeStartRect.current = deepWelcomeLockup.current?.getBoundingClientRect() ?? null;
    const wordCount = prompt.match(/\S+/g)?.length ?? 0;
    const userMessage: DeepChatMessage = { id: ++deepMessageId.current, role: "user", content: prompt };
    const assistantMessage: DeepChatMessage = {
      id: ++deepMessageId.current,
      role: "assistant",
      content: "",
      provider: deepSelectedModel.provider,
      model: deepSelectedModel.model,
      reasoning: deepReasoning,
    };
    setDeepMessages((messages) => [...messages, userMessage, assistantMessage]);
    setDeepStream({ messageId: assistantMessage.id, words: loremForWordCount(wordCount), revealed: 0 });
    setDeepDraft("");
  };

  const startDeepThread = () => {
    setDeepStream(null);
    setDeepMessages([]);
    setDeepActiveThread(null);
    setDeepDraft("");
    window.setTimeout(() => deepComposerInput.current?.focus(), 0);
  };

  const startDeepThreadInFolder = (folder: string) => {
    startDeepThread();
    setDeepFolder(folder);
    setDeepFolderOpen(false);
    setDeepBranchOpen(false);
  };

  const openDeepMockThread = (thread: DeepMockThread) => {
    setDeepStream(null);
    setDeepMessages(thread.messages.map((message) => ({ ...message })));
    setDeepActiveThread(thread.id);
    setDeepFolder(thread.folder);
    setDeepFolderOpen(false);
    setDeepBranchOpen(false);
    deepMessageId.current = Math.max(deepMessageId.current, ...thread.messages.map((message) => message.id));
    window.setTimeout(() => deepComposerInput.current?.focus(), 0);
  };

  const hasDeepConversation = deepMessages.length > 0;
  const deepMockThreadFolders = useMemo(() => {
    const folders = new Map<string, DeepMockThread[]>();
    deepMockThreads.forEach((thread) => folders.set(thread.folder, [...(folders.get(thread.folder) ?? []), thread]));
    const position = new Map(deepFolderOrder.map((folder, index) => [folder, index]));
    return [...folders.entries()].sort(([left], [right]) => (position.get(left) ?? 0) - (position.get(right) ?? 0));
  }, [deepFolderOrder]);

  const moveDeepMockFolder = (targetFolder: string) => {
    if (!deepDraggedFolder || deepDraggedFolder === targetFolder) return;
    setDeepFolderOrder((folders) => {
      const next = folders.filter((folder) => folder !== deepDraggedFolder);
      const targetIndex = next.indexOf(targetFolder);
      next.splice(targetIndex < 0 ? next.length : targetIndex, 0, deepDraggedFolder);
      return next;
    });
  };
  useLayoutEffect(() => {
    const element = deepWelcomeLockup.current;
    const start = deepWelcomeStartRect.current;
    if (!hasDeepConversation || !element || !start) return;
    deepWelcomeStartRect.current = null;
    const end = element.getBoundingClientRect();
    const deltaY = start.top - end.top;
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    element.animate([
      { transform: `translate(-50%, calc(-50% + ${deltaY}px))` },
      { transform: "translate(-50%, -50%)" },
    ], {
      duration: 700,
      easing: "cubic-bezier(.22,.8,.25,1)",
    });
  }, [hasDeepConversation]);
  const deepFolders = useMemo(() => [
    "Home",
    ...Array.from(new Set(config.conversations.map((conversation) => conversation.folder))).filter((folder) => folder !== "Home"),
  ], [config.conversations]);
  const deepRecentModels = useMemo(() => {
    const recent: DeepModelChoice[] = [];
    const seen = new Set<string>();
    const addRecent = (choice: DeepModelChoice) => {
      const key = `${choice.provider}\u0000${choice.model}`;
      if (!choice.provider || !choice.model || seen.has(key)) return;
      seen.add(key);
      recent.push(choice);
    };
    addRecent(deepSelectedModel);
    deepConfiguredRecents.forEach(addRecent);
    return recent.slice(0, 10);
  }, [deepConfiguredRecents, deepSelectedModel]);
  const showingDeepRecents = deepActiveProvider === deepRecentProvider;
  const activeDeepModelGroup = deepModelGroups.find((group) => group.provider === deepActiveProvider) ?? deepModelGroups[0];
  const activeDeepModelLabel = showingDeepRecents ? "Recent" : activeDeepModelGroup.provider;
  const deepModelChoices = showingDeepRecents
    ? deepRecentModels
    : activeDeepModelGroup.models.map((model) => ({ provider: activeDeepModelGroup.provider, model }));
  const normalizedDeepModelQuery = deepModelQuery.trim().toLowerCase();
  const visibleDeepModels = deepModelChoices.filter((choice) => (
    choice.model.toLowerCase().includes(normalizedDeepModelQuery) || choice.provider.toLowerCase().includes(normalizedDeepModelQuery)
  ));
  const deepFolderName = deepFolder.split("/").map((part) => part.trim()).filter(Boolean).at(-1) ?? deepFolder;
  const deepFolderIsRepository = deepFolder === config.workspace.name || deepFolder.startsWith(`${config.workspace.name} /`);
  const deepWorkspaceIsRepository = Boolean(config.workspace.branch);
  const deepVisibleWorkspaceFiles = deepWorkspaceSearchResults ?? deepWorkspaceFiles;
  const deepWorkspaceFileTree = useMemo(() => deepWorkspaceTree(deepVisibleWorkspaceFiles), [deepVisibleWorkspaceFiles]);
  const deepWorkspaceFolderStatus = useMemo(() => deepWorkspaceFolderStatuses(deepWorkspaceFileStatus), [deepWorkspaceFileStatus]);
  const deepStagedFolderStatus = useMemo(() => deepWorkspaceFolderStatuses(deepStagedFileStatus), [deepStagedFileStatus]);
  const deepChangedFolderStatus = useMemo(() => deepWorkspaceFolderStatuses(deepChangedFileStatus), [deepChangedFileStatus]);
  const deepStagedFiles = useMemo(() => Object.entries(deepStagedFileStatus).sort(([left], [right]) => left.localeCompare(right)), [deepStagedFileStatus]);
  const deepChangedFiles = useMemo(() => Object.entries(deepChangedFileStatus).sort(([left], [right]) => left.localeCompare(right)), [deepChangedFileStatus]);
  const deepVisibleGitGraphEntries = useMemo(() => deepGitGraphLanes(deepGitGraphEntries), [deepGitGraphEntries]);
  const deepStagedFileTree = useMemo(() => deepWorkspaceTree(deepStagedFiles.map(([path]) => path)), [deepStagedFiles]);
  const deepChangedFileTree = useMemo(() => deepWorkspaceTree(deepChangedFiles.map(([path]) => path)), [deepChangedFiles]);
  const deepFolderPath = (folder: string) => {
    const root = config.workspace.root.replace(/\/$/, "");
    if (folder === "Home") return "~";
    if (folder === config.workspace.name) return root;
    const parts = folder.split("/").map((part) => part.trim()).filter(Boolean);
    if (parts[0] === config.workspace.name) parts.shift();
    return parts.length ? `${root}/${parts.join("/")}` : root;
  };
  const startEditorPanelResize = (event: ReactPointerEvent<HTMLButtonElement>) => {
    event.preventDefault();
    setEditorSideResizing(true);
    const startX = event.clientX;
    const startWidth = editorSideWidth;
    const maxWidth = window.innerWidth / 3;
    const minWidth = Math.min(190, maxWidth);
    const resize = (moveEvent: PointerEvent) => setEditorSideWidth(Math.min(maxWidth, Math.max(minWidth, startWidth + moveEvent.clientX - startX)));
    const stop = () => {
      setEditorSideResizing(false);
      document.removeEventListener("pointermove", resize);
      document.removeEventListener("pointerup", stop);
      document.removeEventListener("pointercancel", stop);
    };
    document.addEventListener("pointermove", resize);
    document.addEventListener("pointerup", stop);
    document.addEventListener("pointercancel", stop);
  };
  const ensureDeepEditorFile = useCallback(async (path: string) => {
    if (deepEditorFileCacheRef.current[path]) return;
    setDeepEditorFileLoading(path);
    try {
      const response = await fetch(`/__solomon/workspace-file?path=${encodeURIComponent(path)}`);
      if (!response.ok) throw new Error(`Unable to read ${path} (${response.status})`);
      const file = await response.json() as MockFile;
      setDeepEditorFileCache((cache) => ({ ...cache, [path]: { ...file, status: deepWorkspaceFileLabel(path, deepWorkspaceFileStatus) } }));
    } catch {
      return;
    } finally {
      setDeepEditorFileLoading((current) => current === path ? null : current);
    }
  }, [deepWorkspaceFileStatus]);
  const openDeepEditorFile = (path: string) => {
    window.clearTimeout(deepEditorTreeClickTimer.current);
    deepEditorTreeClickTimer.current = window.setTimeout(() => {
      const existing = deepEditorTabs.find((tab) => tab.path === path);
      if (existing) {
        setDeepActiveEditorTabId(existing.id);
        openFile(path);
        void ensureDeepEditorFile(path);
        return;
      }
      if (!deepActiveEditorTabId || !deepEditorTabs.length) {
        deepEditorTabId.current += 1;
        const id = `editor-tab-${deepEditorTabId.current}`;
        setDeepEditorTabs((tabs) => [...tabs, { id, path }]);
        setDeepActiveEditorTabId(id);
        openFile(path);
        void ensureDeepEditorFile(path);
        return;
      }
      setDeepEditorTabs((tabs) => tabs.map((tab) => tab.id === deepActiveEditorTabId ? { ...tab, path } : tab));
      openFile(path);
      void ensureDeepEditorFile(path);
    }, 220);
  };
  const openDeepEditorFileInNewTab = (path: string) => {
    window.clearTimeout(deepEditorTreeClickTimer.current);
    const existing = deepEditorTabs.find((tab) => tab.path === path);
    if (existing) {
      setDeepEditorTabs((tabs) => [existing, ...tabs.filter((tab) => tab.id !== existing.id)]);
      setDeepActiveEditorTabId(existing.id);
      openFile(path);
      void ensureDeepEditorFile(path);
      return;
    }
    deepEditorTabId.current += 1;
    const id = `editor-tab-${deepEditorTabId.current}`;
    setDeepEditorTabs((tabs) => [{ id, path }, ...tabs]);
    setDeepActiveEditorTabId(id);
    openFile(path);
    void ensureDeepEditorFile(path);
  };
  const selectDeepEditorTab = (id: string, path: string) => {
    setDeepActiveEditorTabId(id);
    openFile(path);
    void ensureDeepEditorFile(path);
  };
  const duplicateDeepEditorTab = (path: string) => {
    deepEditorTabId.current += 1;
    const id = `editor-tab-${deepEditorTabId.current}`;
    setDeepEditorTabs((tabs) => [...tabs, { id, path }]);
    setDeepActiveEditorTabId(id);
    openFile(path);
  };
  const closeDeepEditorTab = (id: string) => {
    const index = deepEditorTabs.findIndex((tab) => tab.id === id);
    const next = deepEditorTabs.filter((tab) => tab.id !== id);
    setDeepEditorTabs(next);
    if (id !== deepActiveEditorTabId) return;
    if (!next.length) {
      setDeepActiveEditorTabId("");
      return;
    }
    const nextTab = next[Math.min(index, next.length - 1)];
    setDeepActiveEditorTabId(nextTab.id);
    openFile(nextTab.path);
    void ensureDeepEditorFile(nextTab.path);
  };
  const moveDeepEditorTab = (targetTabId: string) => {
    if (!deepDraggedEditorTabId || deepDraggedEditorTabId === targetTabId) return;
    setDeepEditorTabs((tabs) => {
      const draggedIndex = tabs.findIndex((tab) => tab.id === deepDraggedEditorTabId);
      const targetIndex = tabs.findIndex((tab) => tab.id === targetTabId);
      if (draggedIndex < 0 || targetIndex < 0) return tabs;
      const next = [...tabs];
      const [dragged] = next.splice(draggedIndex, 1);
      next.splice(targetIndex, 0, dragged);
      return next;
    });
  };
  const startDeepGraphResize = (event: ReactPointerEvent<HTMLButtonElement>) => {
    event.preventDefault();
    const panel = event.currentTarget.parentElement?.parentElement;
    if (!panel) return;
    setDeepGraphResizing(true);
    const startY = event.clientY;
    const startHeight = deepGraphHeight;
    const panelHeight = panel.getBoundingClientRect().height;
    const maxHeight = Math.max(140, panelHeight - 135);
    const resize = (moveEvent: PointerEvent) => setDeepGraphHeight(Math.min(maxHeight, Math.max(140, startHeight + startY - moveEvent.clientY)));
    const stop = () => {
      setDeepGraphResizing(false);
      document.removeEventListener("pointermove", resize);
      document.removeEventListener("pointerup", stop);
      document.removeEventListener("pointercancel", stop);
    };
    document.addEventListener("pointermove", resize);
    document.addEventListener("pointerup", stop);
    document.addEventListener("pointercancel", stop);
  };
  const toggleDeepEditorFolder = (path: string) => setDeepCollapsedEditorFolders((collapsed) => {
    const next = new Set(collapsed);
    if (next.has(path)) next.delete(path);
    else next.add(path);
    return next;
  });
  const toggleDeepGitFolder = (path: string) => setDeepCollapsedGitFolders((collapsed) => {
    const next = new Set(collapsed);
    if (next.has(path)) next.delete(path);
    else next.add(path);
    return next;
  });
  const deepEditorRootCollapsed = deepCollapsedEditorFolders.has("__root__");
  const deepWorkspacePath = abbreviateDeepHomePath(config.workspace.root);
  const deepActiveEditorTab = deepEditorTabs.find((tab) => tab.id === deepActiveEditorTabId);
  const resolveDeepEditorFile = useCallback((path: string) => deepEditorFileCache[path], [deepEditorFileCache]);
  const deepActiveFile = deepActiveEditorTab ? resolveDeepEditorFile(deepActiveEditorTab.path) : undefined;
  const deepChromeStyle = { "--deep-side-width": `${mode === "editor" ? editorSideWidth : 248}px` } as CSSProperties;

  useEffect(() => {
    const path = deepActiveEditorTab?.path;
    if (!path || deepEditorFileCacheRef.current[path]) return;
    void ensureDeepEditorFile(path);
  }, [deepActiveEditorTab?.path, ensureDeepEditorFile]);
  useLayoutEffect(() => {
    const nav = deepEditorTabsRef.current;
    if (!nav) return;
    nav.querySelector<HTMLElement>(".deep-editor-tab.active")?.scrollIntoView({ block: "nearest", inline: "nearest" });
  }, [deepActiveEditorTabId, deepEditorTabs.length]);
  return (
    <div className="prototype deep-prototype">
      {mode === "agent" && sideCollapsed ? (
        <button className="deep-side-toggle" aria-label="Expand panel" onClick={() => setSideCollapsed(false)}>
          <PanelLeft size={16} />
        </button>
      ) : null}
      {mode === "editor" && editorSideCollapsed ? (
        <button className="deep-side-toggle" aria-label="Expand editor panel" onClick={() => setEditorSideCollapsed(false)}>
          <PanelLeft size={16} />
        </button>
      ) : null}
      <header className={`deep-chrome ${mode === "editor" ? editorSideCollapsed ? "side-collapsed" : "side-open" : sideCollapsed ? "side-collapsed" : "side-open"}`} style={deepChromeStyle}>
        {mode === "editor" ? <DeepProjectPath path={deepWorkspacePath} fullPath={config.workspace.root} /> : null}
        <div className="deep-chrome-center">
          <div className="mode-control mode-deep">
            <button className={mode === "agent" ? "active" : ""} onClick={() => setMode("agent")}>
              <span className="deep-icon deep-icon-agent"><DeepCrown size={15} /></span><span>Agent</span>
            </button>
            <button className={mode === "editor" ? "active" : ""} onClick={() => setMode("editor")}>
              <span className="deep-icon deep-icon-editor"><Code2 size={14} /></span><span>Editor</span>
            </button>
          </div>
        </div>
      </header>
      {mode === "editor" || sideCollapsed ? null : (
        <aside className="deep-side" aria-label="Side panel">
          <div className="deep-side-head">
            <button className="deep-side-toggle" aria-label="Collapse panel" onClick={() => setSideCollapsed(true)}>
              <PanelLeft size={16} />
            </button>
            <span className="deep-wordmark">SOLOMON</span>
          </div>
          <div className="deep-side-actions">
            <button className="deep-side-action" type="button" onClick={startDeepThread}>
              <span className="deep-side-action-icon" aria-hidden="true"><DeepMessage /></span>
              <span className="deep-side-action-label">New Thread</span>
            </button>
            <button className="deep-side-action" type="button">
              <span className="deep-side-action-icon" aria-hidden="true"><Search size={19} strokeWidth={2} /></span>
              <span className="deep-side-action-label">Search</span>
            </button>
            <button className="deep-side-action" type="button">
              <span className="deep-side-action-icon" aria-hidden="true"><DeepPuzzle /></span>
              <span className="deep-side-action-label">Customization</span>
            </button>
          </div>
          <div className="deep-side-label"><span>threads</span></div>
          <nav className="deep-thread-list" aria-label="Mock threads">
            {deepMockThreadFolders.map(([folder, threads]) => (
              <section className={`deep-thread-folder ${deepDraggedFolder === folder ? "is-dragging" : ""}`} key={folder}>
                <div
                  className="deep-thread-folder-head"
                  draggable
                  onDragStart={(event) => {
                    setDeepDraggedFolder(folder);
                    event.dataTransfer.effectAllowed = "move";
                    event.dataTransfer.setData("text/plain", folder);
                  }}
                  onDragOver={(event) => {
                    event.preventDefault();
                    event.dataTransfer.dropEffect = "move";
                  }}
                  onDrop={(event) => {
                    event.preventDefault();
                    moveDeepMockFolder(folder);
                    setDeepDraggedFolder(null);
                  }}
                  onDragEnd={() => setDeepDraggedFolder(null)}
                >
                  <button
                    className="deep-thread-folder-trigger"
                    type="button"
                    aria-expanded={!deepCollapsedFolders.has(folder)}
                    title={folder}
                    onClick={() => setDeepCollapsedFolders((collapsed) => {
                      const next = new Set(collapsed);
                      if (next.has(folder)) next.delete(folder);
                      else next.add(folder);
                      return next;
                    })}
                  >
                    {deepCollapsedFolders.has(folder) ? <Folder size={13} /> : <FolderOpen size={13} />}
                    <span>{deepFolderBaseName(folder)}</span>
                  </button>
                  <button
                    className="deep-thread-folder-new"
                    type="button"
                    aria-label={`New chat in ${deepFolderBaseName(folder)}`}
                    title={`New chat in ${folder}`}
                    onClick={() => startDeepThreadInFolder(folder)}
                  >
                    <Plus size={16} />
                  </button>
                </div>
                {deepCollapsedFolders.has(folder) ? null : threads.map((thread) => (
                  <button
                    className={deepActiveThread === thread.id ? "active" : ""}
                    type="button"
                    aria-pressed={deepActiveThread === thread.id}
                    key={thread.id}
                    onClick={() => openDeepMockThread(thread)}
                  >
                    <span title={thread.title}>{thread.title}</span>
                    <time title={`Last interaction: ${thread.lastInteraction} ago`}>{thread.lastInteraction}</time>
                  </button>
                ))}
              </section>
            ))}
          </nav>
          <div className="deep-side-user">
            {editingUserName ? (
              <input
                ref={userNameInput}
                value={userName}
                aria-label="User name"
                onChange={(event) => setUserName(event.target.value)}
                onBlur={finishUserNameEdit}
                onKeyDown={(event) => {
                  if (event.key === "Enter") void finishUserNameEdit();
                  if (event.key === "Escape") {
                    setUserName(savedUserName.current);
                    setEditingUserName(false);
                  }
                }}
              />
            ) : (
              <button className={userNameError ? "error" : ""} type="button" title={userNameError ? "Unable to access ~/.solomon/config.toml" : "Double-click to edit"} onDoubleClick={() => setEditingUserName(true)}>
                {userName || "Unnamed user"}
              </button>
            )}
            <button className="deep-user-settings" type="button" aria-label="User settings" title="User settings">
              <Settings size={16} />
            </button>
          </div>
        </aside>
      )}
      {mode === "editor" && !editorSideCollapsed ? (
        <aside className={`deep-editor-side ${editorSideResizing ? "is-resizing" : ""}`} aria-label="Editor side panel" style={{ width: editorSideWidth }}>
          <button className="deep-side-toggle" aria-label="Collapse editor panel" onClick={() => setEditorSideCollapsed(true)}>
            <PanelLeft size={16} />
          </button>
          <span className="deep-editor-wordmark">SOLOMON</span>
          <div className="deep-editor-view-actions" aria-label="Editor side panel view">
            <button type="button" aria-label="Files" title="Files" aria-pressed={editorSideView === "files"} className={editorSideView === "files" ? "active" : ""} onClick={() => setEditorSideView("files")}><Folder size={14} /></button>
            <button type="button" aria-label="Git" title="Git" aria-pressed={editorSideView === "git"} className={editorSideView === "git" ? "active" : ""} onClick={() => setEditorSideView("git")}><GitBranch size={14} /></button>
          </div>
          {editorSideView === "files" ? <>
            <label className="deep-editor-search">
              <Search size={14} />
              <input value={deepWorkspaceQuery} onChange={(event) => setDeepWorkspaceQuery(event.target.value)} placeholder="Search files or folders" aria-label="Search files or folders" />
            </label>
            <div className="deep-editor-folder-row">
              <button className="deep-editor-folder-identity" type="button" aria-expanded={!deepEditorRootCollapsed} onClick={() => toggleDeepEditorFolder("__root__")}><ChevronRight className={deepEditorRootCollapsed ? "" : "open"} size={12} />{deepFolderIcon("Solomon", !deepEditorRootCollapsed, true)}<span>Solomon</span></button>
              {deepWorkspaceIsRepository ? (
                <div className="deep-editor-branch-control">
                  <button
                    className="deep-editor-branch"
                    type="button"
                    aria-label={`Current branch: ${deepBranch}. Select branch`}
                    aria-haspopup="listbox"
                    aria-expanded={deepBranchOpen}
                    onClick={() => setDeepBranchOpen((open) => !open)}
                  >
                    <GitBranch size={14} /><span>{deepBranch}</span><ChevronDown size={11} className={deepBranchOpen ? "open" : ""} />
                  </button>
                  {deepBranchOpen ? <div className="deep-editor-branch-menu" role="listbox" aria-label="Branches">
                    {deepBranches.map((branch) => <button type="button" role="option" aria-selected={branch === deepBranch} key={branch} onClick={() => { setDeepBranch(branch); setDeepBranchOpen(false); }}><GitBranch size={13} /><span>{branch}</span>{branch === deepBranch ? <Check size={12} /> : null}</button>)}
                  </div> : null}
                </div>
              ) : null}
            </div>
            {deepEditorRootCollapsed ? null : <nav className="deep-editor-file-list" aria-label="Files in Solomon">
              <DeepWorkspaceTree folder={deepWorkspaceFileTree} depth={1} collapsedFolders={deepCollapsedEditorFolders} toggleFolder={toggleDeepEditorFolder} fileStatus={deepWorkspaceFileStatus} folderStatus={deepWorkspaceFolderStatus} forceExpanded={Boolean(deepWorkspaceQuery.trim())} onOpenFile={openDeepEditorFile} onOpenFileInNewTab={openDeepEditorFileInNewTab} activeFilePath={deepActiveFile?.path} />
            </nav>}
          </> : <section className={`deep-editor-git-panel ${deepGraphCollapsed ? "graph-collapsed" : ""}`} aria-label="Git changes" style={{ "--deep-graph-height": `${deepGraphCollapsed ? 29 : deepGraphHeight}px` } as CSSProperties}>
            <div className="deep-editor-commit" aria-label="Create commit">
              <input value={deepCommitMessage} onChange={(event) => setDeepCommitMessage(event.target.value)} placeholder="Commit message" aria-label="Commit message" />
              <button type="button" disabled={!deepCommitMessage.trim()} title="Commit staged changes">Commit</button>
            </div>
            <div className="deep-editor-git-list" aria-label="Changed files">
            <header className="deep-editor-git-section-header staged" aria-label="Staged changes">
                <button type="button" aria-expanded={!deepStagedChangesCollapsed} onClick={() => setDeepStagedChangesCollapsed((collapsed) => !collapsed)}>
                  <ChevronRight className={deepStagedChangesCollapsed ? "" : "open"} size={12} /><span>Staged Changes</span>
                </button>
                <small>{deepStagedFiles.length}</small>
            </header>
            {deepStagedChangesCollapsed ? null : <div className="deep-editor-git-tree" aria-label="Staged files"><DeepWorkspaceTree folder={deepStagedFileTree} collapsedFolders={deepCollapsedGitFolders} toggleFolder={toggleDeepGitFolder} fileStatus={deepStagedFileStatus} folderStatus={deepStagedFolderStatus} /></div>}
            <header className="deep-editor-git-section-header changes" aria-label="Changes">
                <button type="button" aria-expanded={!deepChangesCollapsed} onClick={() => setDeepChangesCollapsed((collapsed) => !collapsed)}>
                  <ChevronRight className={deepChangesCollapsed ? "" : "open"} size={12} /><span>Changes</span>
                </button>
                <small>{deepChangedFiles.length}</small>
            </header>
            {deepChangesCollapsed ? null : <div className="deep-editor-git-tree" aria-label="Changed files"><DeepWorkspaceTree folder={deepChangedFileTree} collapsedFolders={deepCollapsedGitFolders} toggleFolder={toggleDeepGitFolder} fileStatus={deepChangedFileStatus} folderStatus={deepChangedFolderStatus} /></div>}
            </div>
            <section className={`deep-editor-git-graph-panel ${deepGraphResizing ? "is-resizing" : ""}`} aria-label="Git graph">
            {deepGraphCollapsed ? null : <button className="deep-editor-graph-resize" type="button" aria-label="Resize Git graph" title="Drag to resize Git graph" onPointerDown={startDeepGraphResize} />}
            <header className="deep-editor-git-section-header graph" aria-label="Git graph">
              <button type="button" aria-expanded={!deepGraphCollapsed} onClick={() => setDeepGraphCollapsed((collapsed) => !collapsed)}>
                <ChevronRight className={deepGraphCollapsed ? "" : "open"} size={12} /><span>Graph</span>
              </button>
            </header>
            <section className="deep-editor-git-graph" aria-label="Git graph entries">
              {deepGraphCollapsed ? null : <ol>
                <svg className="deep-editor-git-graph-canvas" viewBox={`0 0 64 ${Math.max(22, deepVisibleGitGraphEntries.length * 22)}`} aria-hidden="true">
                  {deepVisibleGitGraphEntries.flatMap(({ id, connections }, rowIndex) => connections.map(({ sourceLane, targetLane, colorLane }, connectionIndex) => {
                    const sourceX = 13 + sourceLane * 12;
                    const targetX = 13 + targetLane * 12;
                    const color = deepGitGraphLaneColors[colorLane % deepGitGraphLaneColors.length];
                    const startY = rowIndex * 22 + 11;
                    const endY = (rowIndex + 1) * 22 + 11;
                    return <path d={deepGitGraphConnectionPath(sourceX, targetX, startY, endY)} stroke={color} key={`${id}-${connectionIndex}`} />;
                  }))}
                  {deepVisibleGitGraphEntries.map(({ id, lane, parents, references }, rowIndex) => {
                    const laneColor = deepGitGraphLaneColors[lane % deepGitGraphLaneColors.length];
                    const isCurrent = references.some((reference) => reference.includes("HEAD ->"));
                    const isMerge = parents.length > 1;
                    const x = 13 + lane * 12;
                    const y = rowIndex * 22 + 11;
                    if (isMerge) return <g key={id}>
                      <circle cx={x} cy={y} r="5" fill="var(--panel)" stroke={laneColor} strokeWidth="2" />
                      <circle cx={x} cy={y} r="2" fill={laneColor} />
                    </g>;
                    return <circle cx={x} cy={y} r="4" fill={isCurrent ? "var(--panel)" : laneColor} stroke={laneColor} strokeWidth="2" key={id} />;
                  })}
                </svg>
                {deepVisibleGitGraphEntries.map(({ id, subject, references, laneCount }) => <li key={id} style={{ "--deep-graph-content-offset": `${12 + laneCount * 12}px` } as CSSProperties}>
                  <span title={subject}>{subject}</span>
                  {references.length ? <small className={references.some((reference) => reference.includes("HEAD ->")) ? "branch" : ""}>{references.join(" · ").replace("HEAD -> ", "@ ")}</small> : null}
                </li>)}
              </ol>}
            </section>
            </section>
          </section>}
          <button
            className="deep-editor-resize"
            type="button"
            aria-label="Resize editor panel"
            title="Drag to resize. Double-click to reset."
            onPointerDown={startEditorPanelResize}
            onDoubleClick={() => setEditorSideWidth(248)}
          />
        </aside>
      ) : null}
      <div className="deep-main">
        <main className={`deep-stage ${mode === "agent" ? "is-agent" : "is-editor"}`} aria-label="V5 canvas">
          {mode === "agent" ? (
            <section className={`deep-chat ${hasDeepConversation ? "has-conversation" : "is-empty"}`} aria-label="Conversation">
              <div className="deep-empty-group">
              <div className="deep-welcome-lockup" ref={deepWelcomeLockup}>
                <DeepAsciiBanner />
                <h1><span>Welcome back, </span><strong>{userName || "User"}</strong></h1>
              </div>
              {hasDeepConversation ? (
                <div className="deep-conversation" ref={deepConversationScroll} aria-live="polite">
                  <div className="deep-conversation-inner">
                    {deepMessages.map((message) => (
                      <article className={`deep-turn deep-turn-${message.role}`} key={message.id}>
                        <p>{message.content || <span className="deep-stream-caret" aria-label="Generating response" />}</p>
                        {message.toolCalls?.length ? (
                          <div className="deep-code-mode" aria-label="Code Mode orchestration">
                            <header>
                              <span><Braces size={15} /><strong>Code Mode</strong></span>
                              <small>orchestrate · {message.toolCalls.length} calls</small>
                            </header>
                            <div className="deep-agent-events" aria-label="Orchestration trace">
                              {message.toolCalls.map((tool, index) => (
                                <div className="deep-tool-call" key={`${tool.name}-${index}`}>
                                  <Wrench size={14} />
                                  <strong>{deepCodeModeToolName(tool.name)}</strong>
                                  <span>{tool.detail}</span>
                                  <small className={tool.status}>{tool.status}</small>
                                </div>
                              ))}
                            </div>
                          </div>
                        ) : null}
                        {message.changedFiles?.length ? (
                          <div className="deep-changed-files" aria-label="Changed files">
                            <header><FileDiff size={14} /><span>{message.changedFiles.length} file{message.changedFiles.length === 1 ? "" : "s"} changed</span></header>
                            {message.changedFiles.map((file) => (
                              <div key={file.path}>
                                <span title={file.path}>{file.path}</span>
                                <small><b>+{file.additions}</b><i>-{file.deletions}</i></small>
                              </div>
                            ))}
                          </div>
                        ) : null}
                      </article>
                    ))}
                  </div>
                </div>
              ) : null}
              {!hasDeepConversation ? (
                <div className="deep-folder-control" ref={deepFolderControl}>
                  <button
                    className="deep-folder-trigger"
                    type="button"
                    aria-label={`Current folder: ${deepFolder}. Select folder`}
                    aria-haspopup="listbox"
                    aria-expanded={deepFolderOpen}
                    onClick={() => {
                      setDeepFolderOpen((open) => !open);
                      setDeepBranchOpen(false);
                      setDeepModelOpen(false);
                      setDeepReasoningOpen(false);
                    }}
                  >
                    <Folder size={13} aria-hidden="true" />
                    <span>{deepFolderName}</span>
                    <ChevronDown className={deepFolderOpen ? "open" : ""} size={12} aria-hidden="true" />
                  </button>
                  {deepFolderOpen ? (
                    <div className="deep-folder-menu" role="listbox" aria-label="Folders">
                      {deepFolders.map((folder) => (
                        <button
                          type="button"
                          role="option"
                          aria-selected={folder === deepFolder}
                          className={folder === deepFolder ? "selected" : ""}
                          key={folder}
                          onClick={() => {
                            setDeepFolder(folder);
                            setDeepFolderOpen(false);
                            setDeepBranchOpen(false);
                            deepComposerInput.current?.focus();
                          }}
                        >
                          <Folder size={14} aria-hidden="true" />
                          <span title={deepFolderPath(folder)}>{truncateDeepFolderPath(deepFolderPath(folder))}</span>
                          {folder === deepFolder ? <Check size={13} aria-hidden="true" /> : null}
                        </button>
                      ))}
                    </div>
                  ) : null}
                </div>
              ) : null}
              <div className={`deep-composer-dock ${deepModelOpen || deepReasoningOpen || deepFolderOpen || deepBranchOpen ? "controls-open" : ""}`}>
              <form className={`deep-composer ${deepModelOpen || deepReasoningOpen ? "controls-open" : ""}`} onSubmit={(event) => { event.preventDefault(); sendDeepMessage(); }}>
                <textarea
                  ref={deepComposerInput}
                  value={deepDraft}
                  rows={3}
                  aria-label="Message Solomon"
                  placeholder="Ask Solomon anything…"
                  onChange={(event) => setDeepDraft(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === "Enter" && !event.shiftKey) {
                      event.preventDefault();
                      sendDeepMessage();
                    }
                  }}
                />
                <footer>
                  <div className="deep-model-control" ref={deepModelControl}>
                    <button
                      className="deep-model-trigger"
                      type="button"
                      aria-label="Select model"
                      aria-expanded={deepModelOpen}
                      onClick={() => {
                        setDeepModelOpen((open) => !open);
                        setDeepReasoningOpen(false);
                        setDeepActiveProvider(deepRecentProvider);
                        setDeepModelQuery("");
                      }}
                    >
                      <span>{deepSelectedModel.model}</span>
                      <ChevronDown className={deepModelOpen ? "open" : ""} size={13} />
                    </button>
                    {deepModelOpen ? (
                      <div className="deep-model-menu" role="dialog" aria-label="Models configured in Solomon Home">
                        <nav className="deep-provider-rail" aria-label="Providers">
                          <header>{deepModelsError ? "Mock" : "Providers"}</header>
                          <button
                            type="button"
                            className={`deep-provider-recent ${showingDeepRecents ? "active" : ""}`}
                            aria-pressed={showingDeepRecents}
                            title="Recent"
                            onClick={() => {
                              setDeepActiveProvider(deepRecentProvider);
                              setDeepModelQuery("");
                            }}
                          >
                            <span><History size={16} /></span>
                            <strong>Recent</strong>
                            <small>{deepRecentModels.length}</small>
                          </button>
                          {deepModelGroups.map((group) => (
                            <button
                              type="button"
                              className={deepActiveProvider === group.provider ? "active" : ""}
                              aria-pressed={deepActiveProvider === group.provider}
                              title={group.provider}
                              key={group.provider}
                              onClick={() => {
                                setDeepActiveProvider(group.provider);
                                setDeepModelQuery("");
                              }}
                            >
                              <span><DeepProviderIcon provider={group.provider} /></span>
                              <strong>{group.provider}</strong>
                              <small>{group.models.length}</small>
                            </button>
                          ))}
                        </nav>
                        <div className="deep-model-browser">
                          <label className="deep-model-search">
                            <Search size={15} />
                            <input
                              value={deepModelQuery}
                              aria-label="Search models"
                              placeholder={`Search ${activeDeepModelLabel.toLowerCase()}…`}
                              onChange={(event) => setDeepModelQuery(event.target.value)}
                            />
                          </label>
                          <header>
                            <span>{activeDeepModelLabel}</span>
                            <small>
                              {visibleDeepModels.length} {visibleDeepModels.length === 1 ? "model" : "models"}
                              {!showingDeepRecents && activeDeepModelGroup.complete === false ? " · cached" : ""}
                            </small>
                          </header>
                          <div className="deep-model-list" role="listbox" aria-label={`${activeDeepModelLabel} models`}>
                            {visibleDeepModels.length ? visibleDeepModels.map((choice) => {
                              const selected = deepSelectedModel.provider === choice.provider && deepSelectedModel.model === choice.model;
                              return (
                                <button
                                  type="button"
                                  role="option"
                                  aria-selected={selected}
                                  className={selected ? "selected" : ""}
                                  key={`${choice.provider}:${choice.model}`}
                                  onClick={() => {
                                    setDeepSelectedModel(choice);
                                    setDeepModelOpen(false);
                                    deepComposerInput.current?.focus();
                                  }}
                                >
                                  <span>
                                    <strong>{choice.model}</strong>
                                    <small>{choice.provider}</small>
                                  </span>
                                  {selected ? <Check size={14} /> : null}
                                </button>
                              );
                            }) : <p>No models match your search.</p>}
                          </div>
                        </div>
                      </div>
                    ) : null}
                  </div>
                  <div className="deep-composer-settings">
                    <i />
                    <div className="deep-reasoning-control" ref={deepReasoningControl}>
                      <button
                        className="deep-reasoning-label"
                        type="button"
                        aria-label="Change reasoning level"
                        aria-expanded={deepReasoningOpen}
                        onClick={() => {
                          setDeepReasoningOpen((open) => !open);
                          setDeepModelOpen(false);
                        }}
                      >
                        <span>Reasoning</span>
                        <strong>{deepReasoningOptions.find((option) => option.value === deepReasoning)?.label}</strong>
                        <ChevronDown className={deepReasoningOpen ? "open" : ""} size={11} />
                      </button>
                      {deepReasoningOpen ? (
                        <div className="deep-reasoning-popover">
                          <header><span>Reasoning level</span><strong>{deepReasoningOptions.find((option) => option.value === deepReasoning)?.label}</strong></header>
                          <div className="deep-reasoning-scale">
                            <input
                              type="range"
                              min="0"
                              max={deepReasoningOptions.length - 1}
                              step="1"
                              value={deepReasoningOptions.findIndex((option) => option.value === deepReasoning)}
                              style={{ "--reasoning-fill": `${deepReasoningOptions.findIndex((option) => option.value === deepReasoning) / (deepReasoningOptions.length - 1) * 100}%` } as CSSProperties}
                              aria-label="Reasoning level"
                              aria-valuetext={deepReasoningOptions.find((option) => option.value === deepReasoning)?.label}
                              onChange={(event) => setDeepReasoning(deepReasoningOptions[Number(event.target.value)].value)}
                            />
                            <div className="deep-reasoning-ticks" aria-hidden="true">
                              {deepReasoningOptions.map((option, index) => {
                                const selectedIndex = deepReasoningOptions.findIndex((item) => item.value === deepReasoning);
                                return <span className={index <= selectedIndex ? "reached" : ""} key={option.value}><i /><small>{option.label}</small></span>;
                              })}
                            </div>
                          </div>
                        </div>
                      ) : null}
                    </div>
                    <button
                      className="deep-fast-toggle"
                      type="button"
                      aria-label={`Fast mode ${deepFastMode ? "on" : "off"}`}
                      aria-pressed={deepFastMode}
                      title={`Fast mode: ${deepFastMode ? "on" : "off"}`}
                      onClick={() => setDeepFastMode((enabled) => !enabled)}
                    >
                      <Zap size={12} aria-hidden="true" />
                      <span>Fast</span>
                    </button>
                    <button
                      className={`deep-message-mode is-${deepMessageMode}`}
                      type="button"
                      aria-label={`Message mode: ${deepMessageMode}. Switch to ${deepMessageMode === "agent" ? "chat" : "agent"}`}
                      title={`Switch to ${deepMessageMode === "agent" ? "Chat" : "Agent"}`}
                      onClick={() => setDeepMessageMode((current) => current === "agent" ? "chat" : "agent")}
                    >
                      <span className="deep-message-mode-icon" aria-hidden="true">
                        {deepMessageMode === "agent" ? <Bot /> : <DeepMessage />}
                      </span>
                      <span>{deepMessageMode === "agent" ? "Agent" : "Chat"}</span>
                    </button>
                  </div>
                  <button className="deep-composer-send" type="submit" disabled={!deepDraft.trim() || Boolean(deepStream)} aria-label="Send message">
                    <ArrowRight size={16} />
                  </button>
                </footer>
              </form>
              {deepFolderIsRepository ? (
                <div className="deep-branch-control">
                  <button
                    className="deep-branch-trigger"
                    type="button"
                    aria-label={`Current branch: ${deepBranch}. Select branch`}
                    aria-haspopup="listbox"
                    aria-expanded={deepBranchOpen}
                    onClick={() => {
                      setDeepBranchOpen((open) => !open);
                      setDeepFolderOpen(false);
                      setDeepModelOpen(false);
                      setDeepReasoningOpen(false);
                    }}
                  >
                    <GitBranch size={13} aria-hidden="true" />
                    <span>{deepBranch}</span>
                    <ChevronDown className={deepBranchOpen ? "open" : ""} size={12} aria-hidden="true" />
                  </button>
                  {deepBranchOpen ? (
                    <div className="deep-branch-menu" role="listbox" aria-label="Branches">
                      {deepBranches.map((branch) => (
                        <button
                          type="button"
                          role="option"
                          aria-selected={branch === deepBranch}
                          className={branch === deepBranch ? "selected" : ""}
                          key={branch}
                          onClick={() => {
                            setDeepBranch(branch);
                            setDeepBranchOpen(false);
                            deepComposerInput.current?.focus();
                          }}
                        >
                          <GitBranch size={13} aria-hidden="true" />
                          <span>{branch}</span>
                          {branch === deepBranch ? <Check size={13} aria-hidden="true" /> : null}
                        </button>
                      ))}
                    </div>
                  ) : null}
                </div>
              ) : null}
              </div>
              </div>
            </section>
          ) : !deepActiveEditorTab ? (
            <div className="deep-editor-blank" aria-label="No file open" />
          ) : (
            <section className="deep-editor-workspace" aria-label={`Editing ${deepActiveEditorTab.path}`}>
              <div className="deep-editor-tabs-shell">
                <div className="deep-editor-tabs-scrollport" ref={deepEditorTabsRef}>
                  <nav className="deep-editor-tabs" aria-label="Open files">
                {deepEditorTabs.map((tab) => {
                  const candidate = resolveDeepEditorFile(tab.path);
                  const fileName = tab.path.split("/").at(-1) ?? tab.path;
                  const status = candidate?.status ?? deepWorkspaceFileLabel(tab.path, deepWorkspaceFileStatus);
                  return <div
                    className={`deep-editor-tab status-${status} ${tab.id === deepActiveEditorTabId ? "active" : ""}${deepDraggedEditorTabId === tab.id ? " is-dragging" : ""}`}
                    key={tab.id}
                    onDragOver={(event) => {
                      event.preventDefault();
                      event.dataTransfer.dropEffect = "move";
                    }}
                    onDrop={(event) => {
                      event.preventDefault();
                      moveDeepEditorTab(tab.id);
                      setDeepDraggedEditorTabId(null);
                    }}
                  >
                    <button
                      className="deep-editor-tab-trigger"
                      type="button"
                      draggable
                      onDragStart={(event) => {
                        setDeepDraggedEditorTabId(tab.id);
                        event.dataTransfer.effectAllowed = "move";
                        event.dataTransfer.setData("text/plain", tab.id);
                      }}
                      onDragEnd={() => setDeepDraggedEditorTabId(null)}
                      onClick={() => selectDeepEditorTab(tab.id, tab.path)}
                      onDoubleClick={() => duplicateDeepEditorTab(tab.path)}
                    >
                      <DeepWorkspaceFileIcon fileName={fileName} /><span>{fileName}</span>{status !== "clean" ? <i>{status.charAt(0).toUpperCase()}</i> : null}
                    </button>
                    <button className="deep-editor-tab-close" type="button" aria-label={`Close ${fileName}`} title={`Close ${fileName}`} onClick={() => closeDeepEditorTab(tab.id)}><X size={12} /></button>
                  </div>;
                })}
                  </nav>
                </div>
              </div>
              <div className="deep-editor-breadcrumbs">
                <DeepWorkspaceFileIcon fileName={deepActiveEditorTab.path.split("/").at(-1) ?? deepActiveEditorTab.path} />
                {deepActiveEditorTab.path.split("/").map((part, index, all) => <span key={`${part}-${index}`}>{part}{index < all.length - 1 ? <ChevronRight size={11} /> : null}</span>)}
                {deepActiveFile ? <ChangeStats file={deepActiveFile} /> : null}
              </div>
              <div className="deep-editor-surface">
                {deepActiveFile ? <TextEditor key={deepActiveFile.path} file={deepActiveFile} tone="deep" /> : <div className="deep-editor-loading" aria-live="polite">{deepEditorFileLoading === deepActiveEditorTab.path ? "Loading file…" : "Unable to open file"}</div>}
              </div>
              <footer className="deep-editor-status">
                <span><i />Local draft</span>
                <span>{config.workspace.branch || "No branch"}</span>
                <span>Ln 1, Col 1</span>
                <span>Spaces: 2</span>
                <span>UTF-8</span>
              </footer>
            </section>
          )}
        </main>
      </div>
    </div>
  );
}
