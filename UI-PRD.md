# UI-PRD — Solomon GUI (Deep v5)

Product requirements and implementation baseline for the production Solomon interface.

**Status:** Active requirements; architecture reconciled with repository code on **2026-10-29**. Supersedes the July 2026 mock-only foundation plan. Implementation evidence is distinct from release acceptance on macOS, Linux, and Windows.

## 1. Scope and terminology

The shared React frontend lives in `gui/src/`. Browser and Wails desktop clients use the Go daemon in `internal/server/`, which delegates agent execution to the existing runtime. The system terminal REPL remains supported.

| Term | Meaning |
|------|---------|
| Top bar | Upper chrome: panel toggles, SOLOMON label, Agent ↔ Editor switch, workspace path, terminal panel toggle, notifications, and related controls. |
| System terminal | OS shell emulator where the Solomon REPL runs. |
| Terminal panel | Integrated xterm area with multiple panes and terminal tabs. |
| Terminal tab | A shell session inside the terminal panel. |
| GUI bundle | Production React build embedded from `gui/assets/frontend`; shared through the daemon by browser and desktop. There is no active mutable `~/.solomon/gui/` bundle contract. |
| Project / workspace | Registered filesystem root addressed by project id; chat and file requests carry their project scope explicitly. |
| Tray | Persistent native status icon in the system notification area or macOS menu bar; distinct from a launcher, taskbar, or Dock icon. |

Use these terms where “terminal” or “header” would be ambiguous.

**Scope decision (2026-10-29): folder drag-reordering is removed entirely.** It is not an MVP requirement, acceptance criterion, or backlog task. Project grouping and collapse remain. This does not remove editor-tab reordering or moving terminal tabs between panes.

Other retained requirements include in-app onboarding, workspace-wide filename search, Git commit, tray status, and tri-OS acceptance. Updating this document does not silently waive those gaps.

## 2. User experience and acceptance

### 2.1 User stories

- **US-1:** Launch from a desktop icon, choose a project, and use chat, editor, and terminal panel without first opening a system terminal.
- **US-2:** Complete provider/model setup in the app on a fresh config without being sent to REPL `/onboard`.
- **US-3:** Switch Agent ↔ Editor without losing workspace context.
- **US-4:** See streaming assistant text, reasoning, tool activity, changed files, and turn statistics; stop or reconnect a turn.
- **US-5:** Browse, search, edit, and save project files with Git status, history, branch checkout, and staged-file commits.
- **US-6:** Use multiple terminal tabs and panes scoped to the chosen workspace.
- **US-7:** Choose model, reasoning effort, fast mode, and agent/chat mode. Model, reasoning, and fast preferences currently persist globally; agent/chat mode belongs to the conversation.
- **US-8:** Browse conversations grouped by project/folder, collapse groups, and see short project names.
- **US-9:** See daemon running/error/updating status in a native tray/menu-bar icon.

### 2.2 Acceptance matrix

**Present** means code is connected to a real implementation, not that tri-OS product acceptance has passed. **Partial** means a requirement has an identified gap. **Pending** means the required behavior is not implemented. Visual parity, reliability, and platform verification remain separate gates.

| Requirement | Code status | Remaining acceptance work |
|-------------|-------------|---------------------------|
| Top bar, panel toggles, Agent ↔ Editor, workspace path | Present | Deep v5 visual walkthrough on three OSes. |
| ASCII welcome and composer transition into chat | Present | Visual and interaction walkthrough. |
| Resizable side panels with double-click reset | Partial | Editor reset exists; shared shell resize handles lack double-click reset. |
| Conversation groups, collapse, short project names | Present | No folder reordering required. |
| Create/select conversations; project-scoped disk persistence | Present | Repeated-switch and REPL interoperability acceptance. |
| Current/recent/provider model catalog, reasoning, fast, agent/chat | Present | Verify persisted preferences and mode transitions end to end. This baseline records shared preferences rather than claiming per-session isolation. |
| SSE text, reasoning, tools, changed files | Present | Real-provider stream integrity and recovery acceptance. |
| Stop turn and reconnect with event cursor | Present | Validate disconnect/reconnect without corrupting sessions. |
| Usage/token footer | Present | Statistics are exposed through a footer popover; verify available REPL fields. |
| File tree and staged/modified/untracked badges | Present | Workspace-switch isolation acceptance. |
| Debounced workspace-wide filename search | Partial | Current filter searches loaded entries only; unloaded directories must be searchable. |
| Multiple editor tabs, drag reorder, double-click opening | Present | Interaction walkthrough. |
| CodeMirror Go and generic syntax, Deep theme | Present | Syntax and visual acceptance. |
| Read/write files, autosave, workspace path confinement | Present | Current debounce is 500 ms; verify saves and traversal/symlink rejection. |
| Git staged/unstaged lists, collapse, stage/unstage | Present | Refresh and failure handling acceptance. |
| Commit staged changes with a message | Present | UI and project-scoped daemon/development APIs implemented; staged-only behavior and Git failures tested. See §4. |
| Git graph with topo-order and lanes | Present | Merge/branch topology acceptance. |
| Branch list and checkout | Present | Refresh file, branch, and Git state after checkout. |
| Resizable terminal panel with open/close control | Present | Desktop interaction acceptance. |
| Up to eight terminal panes, splits, tab moves | Present | Continuity and resize acceptance. |
| xterm over WebSocket to real PTY; workspace cwd and resize | Present | zsh/bash/PowerShell acceptance on three OSes. |
| Persist user name | Present | Read through sidebar data; write through user-name API. |
| Notifications for errors, completion, updates/lifecycle | Partial | Notification feed and lifecycle watcher exist; verify required event coverage and recovery UX. |
| Project path display and clipboard copy | Present | Clipboard behavior across clients/platforms. |
| Folder picker with per-conversation organization metadata | Partial | Project folder selection exists; separate per-conversation organization metadata is not established. |
| In-app fresh-config onboarding | Partial | Provider connection/model settings exist; guided first-run flow is missing and backend can still request `/onboard`. |
| Native tray/menu-bar daemon status | Pending | No tray implementation found. See §4. |
| Desktop installation and OS registration | Present | Installer paths exist; clean-install tri-OS validation remains. |

### 2.3 Success criteria and release gates

| Criterion | Target | Current evidence |
|-----------|--------|------------------|
| Fresh install → first chat | Within 5 minutes on each supported OS | Not measured; onboarding gap remains. |
| Feature coverage | All retained acceptance requirements work against real backends | Incomplete; see matrix. |
| Daemon unavailable at app launch | Auto-start or clear recovery within 3 seconds | Auto-start exists; desktop startup uses a 15-second timeout, which does not prove the target. |
| UI turn latency | First SSE byte visible within 500 ms, excluding LLM time | Not measured in this audit. |
| Workspace isolation | No chat/file/Git leakage during 10 rapid switches | Must be validated against the real daemon. |
| Path confinement | Reject file operations outside project root, including symlink escapes | Checks exist; current invalid-file-path response is HTTP 400. |
| Terminal isolation | Initial cwd is workspace root; shell may subsequently `cd` elsewhere | Real PTY implementation exists; tri-OS QA remains. |

**Implementation progress estimate: approximately 81%** of retained functional scope, following the folder-order scope removal, architecture reconciliation, and staged-file commit implementation (previous estimate: approximately 78%). This is an engineering estimate, not a checkbox ratio or release-readiness score. Outstanding functional work and platform validation still count; documentation changes do not implement features.

**Verification snapshot (2026-10-29):** `tsc --noEmit` passed. `npm test` ran 173 tests: 172 passed, one failed in `gui/tests/imageTags.test.mjs:88` (unclosed backtick text unexpectedly rendered as a code block). Some GUI tests use Vite middleware/fixtures and cannot certify real-daemon or tri-OS acceptance. This snapshot is from the audit immediately preceding this documentation revision.

### 2.4 Non-goals

- Folder drag-reordering.
- Replacing the system terminal REPL.
- LSP, semantic search, MemPalace, vault migration.
- Git push/pull UI, PR integration, and a merge-conflict editor beyond status display.
- Treating current LAN/Tailscale reachability as an authenticated remote-access product. Passkey/bootstrap authentication remains future work.

## 3. Current architecture

### 3.1 Hosts, bundle, and lifecycle

- **Shared frontend:** React/TypeScript under `gui/src/`, organized into `shell/`, `home/`, `chat/`, `projects/`, `editor/`, `terminal-panel/`, `settings/`, and other feature modules. A synthetic `app/agent/shared` directory split is not a completion requirement.
- **Go daemon:** `internal/server/` owns project/chat/model/file/Git APIs, SSE streams, terminal sessions, health, notifications, updates, and production asset serving.
- **Production assets:** Vite output is packaged into `gui/assets/frontend` and embedded by `gui/assets.go`. The daemon serves this bundle with SPA fallback; missing static assets return 404.
- **Browser:** consumes the daemon's frontend and APIs directly. Browser delivery already exists and is not a future `--static-dir` milestone.
- **Wails desktop:** `gui/desktop/app/` calls `EnsureRunning` through the CLI, uses `DesktopFrontend()` for assets, and exposes a loopback API/WebSocket proxy through `ServerBridge.URL()`. Frontend requests normally reach the daemon through that proxy. If daemon discovery/asset forwarding fails, desktop frontend serving can fall back to embedded assets; this does not make backend operations available offline.
- **Lifecycle:** one daemon owns the Solomon home lock; discovery/state is in `<SolomonHome>/run/server/state.json`. It is started/reused on demand rather than requiring launchd/systemd/Windows-service registration in this architecture.
- **Development:** Vite and development middleware remain useful for fixtures and tests; they are not the production backend or proof of full integration.

### 3.2 Runtime and persistence

Chat requests identify both project and chat in their URL. Creating a message starts server-side execution and an SSE response; event replay uses `starting_after`, and stop targets the chat. The UI maps events into text, reasoning, tool, changed-file, and statistics components. The backend supports concurrent chats across projects; a single global turn lock is not the architectural contract.

Project roots resolve through the project registry. Conversations live under `<SolomonHome>/projects/<hex>/chats`; configuration and project mapping live under the same Solomon home (`~/.solomon` by default). File and Git operations target the project in the request, rather than a mutable global active-workspace endpoint.

Model/reasoning/fast preferences are saved to shared configuration. They must not be documented as isolated per-workspace settings. Runtime chat metadata records the actual model/usage for display.

### 3.3 Terminal panel

`IntegratedShell.tsx` connects xterm to `/__solomon/terminal` over WebSocket. The Go server manages PTY processes, output/reconnection, input, and resize. Unix uses `github.com/creack/pty`; Windows uses a native pseudoconsole implementation. The initial cwd is the workspace root.

The earlier node-pty/UI-only PTY requirement is superseded. Wails is a host and proxy, not the owner of a separate desktop-only terminal backend.

### 3.4 Current GUI API surface

Paths below describe implemented GUI routes. They replace the provisional `/v1/*` GUI integration plan; this does not remove unrelated APIs used by other clients. `{p}` is a project id and `{c}` a chat id.

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/health` | Health and daemon identity/state. |
| GET / POST | `/__solomon/projects` | Sidebar/project data and project registration. |
| GET / POST | `/__solomon/projects/{p}/chats` | List/create chats. |
| GET / PATCH / DELETE | `/__solomon/projects/{p}/chats/{c}` | Load/rename/delete chat. |
| POST | `/__solomon/projects/{p}/chats/{c}/messages` | Submit message and consume SSE. |
| GET | `/__solomon/projects/{p}/chats/{c}/events?starting_after=N` | Replay/reconnect events. |
| POST | `/__solomon/projects/{p}/chats/{c}/stop` | Stop active chat execution. |
| GET | `/__solomon/projects/{p}/files` | Directory entries. |
| GET / PUT | `/__solomon/projects/{p}/file?path=...` | Read/write file contents. |
| POST | `/__solomon/projects/{p}/file-operation` | Create/rename/move/copy/delete entries. |
| GET | `/__solomon/projects/{p}/status`, `/history`, `/branches`, `/worktrees` | Git state and navigation data (each suffix is relative to the project prefix). |
| POST | `/__solomon/projects/{p}/stage`, `/unstage`, `/restore`, `/checkout` | Git operations (each suffix is relative to the project prefix). |
| POST | `/__solomon/projects/{p}/commit` | Commit staged changes with a message; returns `{ "committed": true }` on success. |
| GET | `/__solomon/models` | Current/recent/provider model catalog. |
| PUT | `/__solomon/current-model`, `/__solomon/reasoning-effort`, `/__solomon/fast-mode` | Shared model preferences. |
| POST | `/__solomon/connect-provider` | Provider setup. |
| PUT | `/__solomon/user-name` | User name; current value comes with sidebar data. |
| GET | `/__solomon/notifications` | Notifications feed. |
| WebSocket | `/__solomon/terminal` | Integrated PTY sessions. |

Additional existing surfaces include GUI settings, customization, active agents, network interfaces, provider quotas, research, development mode, and updates. There is currently no workspace-wide filename-search route or dedicated onboarding-status/wizard contract in this list; those remain implementation work.

### 3.5 Network and security baseline

| Topic | Current behavior / requirement |
|-------|--------------------------------|
| Transport | HTTP and WebSocket; no self-signed HTTPS requirement in the current host architecture. |
| Bind and access | Daemon default bind is `0.0.0.0:<configured port>`, not loopback-only. Access is filtered by interface/MagicDNS policy in `internal/networkpolicy` and `allowNetworkRequest`. Desktop proxy binds exclusively to loopback. |
| Authentication | Do not describe interface policy or origin checks as user authentication. The current GUI path does not implement the former proposed Bearer/passkey contract. Authenticated remote access remains separate work. |
| Origin handling | Desktop proxy validates native origins; terminal WebSocket validates origins; server configures allowed-origin response headers. |
| File confinement | Resolve project roots and requested paths; reject traversal and symlink escape. Current invalid-file-path response is 400. |
| Bundle integrity | Assets travel with the installed binary/build; installation/update flow replaces the shipped bundle. No mutable per-user GUI seed/reset contract. |
| PTY trust | Shells run with the user's privileges, initially in the project root; navigation outside the project in the shell is allowed. |

### 3.6 Installation

`make install` prepares GUI/native desktop artifacts and uses the hot-install coordinator. Desktop installation/registration is implemented in `internal/updater/`: Linux application-menu `.desktop` entry, macOS app registration, and Windows registration script. Existing registration is distinct from tray support. Validate fresh install, upgrades, and launch behavior on each OS before claiming production acceptance. Do not require launchd/systemd/service installation merely to conform to the retired host proposal.

## 4. Outstanding feature contracts

### 4.1 Git Commit

**Implemented:** the Git form calls `POST /__solomon/projects/{p}/commit` with a message. The daemon and Vite development fallback run `git commit -m` in the registered project root, without staging files or supplying pathspecs. The UI blocks empty messages, empty indexes, and duplicate submissions while pending; it also blocks index/branch actions in that view while committing.

On success the message is cleared, Git status refresh is dispatched, and history is reloaded. Refresh errors are distinguished from commit errors so a successful commit is not presented as a failed mutation. On Git failure the UI retains the message and displays Git output, including identity and hook errors. Unstaged and untracked file content remains untouched by the implementation. No implicit stage-all, push, amend, or identity reconfiguration is performed; normal Git hook/signing behavior remains in force.

**Verification:** real-daemon HTTP tests cover staged-only commits (including files with both staged and unstaged edits), literal/multiline messages, empty message/index rejection, initial commit, missing identity, and hook rejection. Browser interaction tests against the development API cover successful commit/history refresh, pending controls, and preserving draft/index on failure. Tri-OS release validation remains separate.

### 4.2 Tray / menu bar

**Current state:** Wails window, launcher icons, desktop registration, health polling, and app update UI exist. No native tray/status-item implementation was found. Tray remains a requirement; it has not been removed by the folder-order scope change.

**Required behavior:** display a native icon in the macOS menu bar, Windows notification area, or Linux status notifier area, reflecting daemon running/error/updating state. Health and update state should come from the existing daemon/lifecycle mechanisms. Verify unavailable-daemon recovery and platform integration. Menu actions and close-to-tray behavior require a product decision before they become acceptance requirements; neither is silently implied by a status icon.

### 4.3 Other remaining work

- Search filenames across the entire workspace, including directories never expanded; debounce requests/filtering.
- Finish guided in-app provider/model onboarding and resume the first chat without a REPL instruction.
- Add double-click reset to shared shell panel resize handles.
- Establish per-conversation folder metadata if retaining the separate organization picker; project selection alone does not fulfill that requirement.
- Verify lifecycle/error/completion/update notification coverage and recovery UX.
- Fix the recorded Markdown test failure and complete real-daemon, visual, and tri-OS acceptance.

## 5. Backlog and validation

Historical phase labels are retained for orientation; implementation is no longer restricted to P0. Sizes are T-shirt estimates, not completion percentages.

| ID / phase | Remaining task | Size |
|------------|----------------|------|
| T-005 / P0 | Repeatable Deep v5 walkthrough; shell double-click reset | M |
| P1–P2 | Maintain current Go daemon/client contract; no host redesign prerequisite | Ongoing |
| T-030 / P3 | Resolve per-conversation organization metadata; preserve project grouping | M |
| T-031–034 / P3 | Validate real-provider SSE, replay, stop, usage, and preference behavior | M |
| T-040 / P4 | Implement debounced whole-workspace filename search | M |
| T-041–043 / P4 | Verify 500 ms autosave, confinement, Git refresh, and graph topology | M |
| T-044 / P4 | Implemented and tested: project-scoped staged-file commit and UI action; retain tri-OS acceptance | M |
| T-050–052 / P5 | Validate existing Go PTY/WebSocket continuity and tri-OS shells | M |
| T-060–062 / P6 | Complete guided provider/model onboarding and fresh-config recovery | L |
| T-070–074 / P7 | Validate existing installer/OS registration on clean systems; update walkthrough evidence | L |
| T-075 / P7 | Implement native tray/menu-bar daemon status | L |
| T-076 / Quality | Fix failing Markdown regression test; verify notification coverage | M |

Validation must cover stream interruption/replay, ten rapid project switches, file traversal/symlink rejection, staged-only commits and Git failures, terminal reconnect/resize, fresh onboarding, and daemon-down startup. UI fixtures complement these checks; they do not replace real backend execution.

### Risks

- Large UI components and duplicated development middleware can drift from production APIs.
- Concurrent chats require per-chat/project isolation; do not introduce a global single-turn assumption.
- Native tray availability differs across Linux desktops; document support and validate without manipulating the user's graphical session.
- Windows pseudoconsole lifecycle and desktop WebSocket proxying need platform testing.
- Network interface selection is not authentication; keep remote authentication requirements explicit.
- A high implementation estimate does not establish latency, reliability, visual parity, or tri-OS release readiness.

## Appendix A — Decision record

| Decision | Current baseline |
|----------|------------------|
| UI direction | Deep v5; shared React implementation. |
| Backend | Existing Go daemon in `internal/server/`, real GUI APIs and runtime integration. |
| Hosts | Browser plus Wails desktop with loopback proxy and daemon-provided assets. |
| Bundle | Embedded production build; no active `~/.solomon/gui/` hosting plan. |
| Project routing | Explicit project/chat ids in requests; no global activate-workspace API requirement. |
| Model preferences | Shared config for model/reasoning/fast; actual turn metadata retained. |
| Concurrent turns | Backend supports concurrent chats across projects. |
| PTY | Go daemon; Unix PTY and Windows pseudoconsole; xterm over WebSocket. |
| Network | HTTP; configurable daemon port, all-interface IPv4 default bind, interface/MagicDNS policy; desktop proxy loopback-only. |
| Platforms | macOS, Linux, Windows; clean-install acceptance still required. |
| Folder ordering | Removed from scope on 2026-10-29; no backlog or acceptance work. |
| Commit and tray | Commit implemented and tested; native tray remains outstanding. See §4. |
| Original decisions | July 2026 mock-only, deferred-host, `/v1/*` GUI proposal, node-pty, and mandatory OS-service assumptions are superseded by this baseline. |
