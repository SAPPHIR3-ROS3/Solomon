# UI-PRD — Solomon GUI (Deep v5)

Product requirements for the production Solomon interface and feature parity with the `ui-prototypes` Deep (v5) direction.

**Status:** Draft — decisions from `/grill-me` session (2026-07-22).

### Terminology

| Term | Meaning |
|------|---------|
| **Top bar** | Horizontal chrome at the top of the UI: panel toggle buttons (side / editor side), **SOLOMON** label, Agent ↔ Editor switch, workspace path label (Editor view), terminal panel toggle, notifications, and related controls. Product term; in v5 markup this spans the side-panel head row and `deep-chrome`. |
| **System terminal** | The OS shell emulator (Ghostty, Terminal.app, iTerm, Windows Terminal, …) where `solomon` REPL runs. This is Solomon’s native surface. |
| **Terminal panel** | The integrated xterm area at the bottom of the **Editor** view (resizable, multi-pane). |
| **Terminal tab** | One shell session inside the terminal panel (tab bar per pane). |
| **GUI** | The React interface for chat, editor, git, and related UI — **not** the system terminal and **not** a replacement for the REPL. |
| **GUI bundle** | The mutable, production React build in `~/.solomon/gui/`. It will be shared by the browser and the Wails WebView once a new host design is approved. |

Do not use “terminal” alone in specs or tasks when the meaning is ambiguous; prefer **system terminal**, **terminal panel**, or **terminal tab**.
Use **top bar** for the upper chrome; do not call it “header” or “toolbar” in specs unless referring to a specific sub-element.

---

## 1. Executive Summary

**Problem Statement:** Solomon’s agent runtime is built for the **system terminal** (REPL). There is currently no approved GUI host or service architecture. Prototyping (five directions) is complete; Deep v5 is the chosen product surface.

**Proposed Solution:** Build the Deep v5 GUI as a shared frontend under `gui/src/`, with Wails code isolated under `gui/desktop/`. The GUI host, service process, API contract, installation flow, and browser delivery are intentionally undecided and will be designed from scratch.

**User-updatable GUI:** The intended active bundle location is `~/.solomon/gui/`. The embed, seed, reset, validation, and serving rules are deliberately deferred with the new host design.

**Current implementation focus (2026-07-23):** The daemon, its API contract, static serving, embed/seed mechanism, and Wails lifecycle are deferred for redesign. The active work is exclusively the production UI source under `gui/src/`: extract and organize Deep v5 while it continues to run against local mock data. UI code must depend on a narrow client interface, not on provisional `/v1/*` daemon routes.

**Success Criteria:**

| KPI | Target |
|-----|--------|
| Install → usable UI | User completes first chat within **5 minutes** after `make install` on a fresh config (tri-OS). |
| Prototype parity | **100%** of Deep v5 panels and interactions listed in §2.3 behave correctly against real backends (not mock TOML / Lorem stream). |
| Daemon availability | App launch succeeds when daemon is down: auto-start or clear recovery within **3 s**. |
| Turn latency (UI) | First SSE byte visible within **500 ms** of submit on local daemon (excluding LLM time). |
| Workspace isolation | Switching workspace never leaks chats, files, or git state across project ids. |

---

## 2. User Experience & Functionality

### 2.1 User Personas

| Persona | Need |
|---------|------|
| **Solo developer (primary)** | Click icon → pick repo → chat + edit + terminal panel without opening a system terminal first. |
| **Power user** | REPL in system terminal remains available; same sessions visible in UI and REPL. |
| **Future remote user** | Passkey/bootstrap auth (deferred; loopback only for MVP). |

### 2.2 User Stories

- **US-1:** As a user, I want to open Solomon from a desktop icon so I do not need a system terminal to start the product.
- **US-2:** As a user, I want to complete provider/model setup in the app when config is empty so I never see `config not set up; run solomon and use /onboard`.
- **US-3:** As a user, I want to switch between Agent and Editor views without losing workspace context.
- **US-4:** As a user, I want to send a message and see streaming assistant output, reasoning, tool calls, and changed files like Deep v5.
- **US-5:** As a user, I want to browse, open, edit, and save project files with git status in the sidebar.
- **US-6:** As a user, I want a **terminal panel** with multiple **terminal tabs** and panes in the Editor view.
- **US-7:** As a user, I want to pick model, reasoning effort, and fast mode from the composer controls.
- **US-8:** As a user, I want conversations grouped by folder with drag-reorder, matching Deep v5 thread sidebar.
- **US-9:** As a user, I want a system-tray/menu-bar icon showing daemon status (running / error / updating).

### 2.3 Acceptance Criteria — Deep v5 Parity Checklist

Each item is **Done** when wired to the global daemon (not Vite mock middleware).

**Shell & navigation**

- [ ] **Top bar**: panel toggles, SOLOMON label, Agent ↔ Editor switch, path label (Editor), terminal panel toggle; layout matches v5.
- [ ] Welcome stage: ASCII banner, centered composer → bottom dock after first message.
- [ ] Resizable side panels (thread sidebar, editor file/git panel) with double-click reset.

**Chat / Agent**

- [ ] Thread list grouped by folder; collapse; drag folder order; show last path segment only.
- [ ] Create/select conversation; persist under `~/.solomon/projects/<hex>/`.
- [ ] Composer: model picker (current + recent + per-provider catalog), reasoning effort, fast mode, agent/chat mode.
- [ ] Submit message → SSE stream: assistant text, reasoning blocks, tool call timeline, turn changed-files list.
- [ ] Cancel in-flight turn (`POST /v1/responses/{id}/cancel`).
- [ ] Usage/token footer when turn completes (match REPL footer fields where available).

**Editor**

- [ ] File tree from workspace scan; git status badges (staged / modified / untracked).
- [ ] Workspace filename search (debounced).
- [ ] Multi-tab editor; drag reorder; open in new tab (double-click tree).
- [ ] CodeMirror: Go + generic syntax; dark Deep theme.
- [ ] Read/write file content with autosave debounce (900 ms edit → save); path jail under active `ProjRoot`.
- [ ] Git panel: staged list, unstaged list, collapsible sections, commit message + **Commit** action.
- [ ] Git graph (topo-order, lane layout) from `git log`.
- [ ] Branch list + checkout from UI (updates workspace state).

**Terminal panel** (Editor view only; not the system terminal / REPL)

- [ ] Bottom **terminal panel**; resizable height; open/close toggle.
- [ ] Multi-pane (up to 8); split; drag **terminal tabs** between panes.
- [ ] Each terminal tab: xterm.js over WebSocket PTY; cwd = active workspace root; resize messages.

**Settings & misc**

- [ ] Edit `user_name` in config (GET/PUT).
- [ ] Notifications panel (daemon events: turn complete, errors, updates) — minimum: turn error + daemon lifecycle.
- [ ] Project path display with copy-to-clipboard.
- [ ] Folder picker for thread organization (persist per conversation metadata).

### 2.4 Non-Goals (MVP)

- Remote / tailnet access with passkey login (auth endpoints exist; UI enforcement deferred on loopback).
- Replacing the **system terminal** REPL for shell-first workflows.
- Other prototype directions (Current, Atlas, Pulse, Quiet) in production build.
- LSP, semantic search, MemPalace, vault migration.
- Git push/pull UI, PR integration, merge conflict editor beyond status display.
- Native macOS `Cmd+V` image paste (see `TODO.md`).
- Embedded `ui-prototypes` gallery / mock-config.toml in production app.

---

## 3. AI System Requirements

### 3.1 Tool Requirements

The UI does **not** duplicate the agent runtime. It consumes existing server turn execution:

| Capability | Source |
|------------|--------|
| All backend capabilities | Deferred: the GUI currently uses its typed mock client. No HTTP API or service contract is approved. |

### 3.2 Evaluation Strategy

| Test | Pass criteria |
|------|---------------|
| **Parity walkthrough** | Scripted checklist §2.3 — all boxes checked on macOS, Linux, Windows. |
| **Stream integrity** | Disconnect mid-turn → session not corrupted; UI shows recoverable state (match `test/stream_integrity_test.go` philosophy). |
| **Workspace switch** | 10 rapid switches between two repos → correct chats and file trees every time. |
| **File jail** | UI cannot read/write outside active `ProjRoot` (403). |
| **Terminal panel isolation** | PTY cwd defaults to workspace root; user can `cd` elsewhere (same trust model as REPL `!`). |

---

## 4. Technical Specifications

### 4.1 Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│  Solomon.app / Solomon.exe / Solomon.AppImage  (Wails)      │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  WebView → daemon-served Deep v5 React GUI bundle      │  │
│  └───────────────┬───────────────────────┬───────────────┘  │
│  Terminal panel: xterm ──WS──► node-pty (UI layer, not Go)  │
│  System tray · workspace picker · daemon lifecycle            │
└──────────────────┼──────────────────────────────────────────┘
                   │ HTTPS localhost (TLS self-signed)
                   ▼
┌─────────────────────────────────────────────────────────────┐
│  solomon daemon  (global, user service)                     │
│  · Host/service architecture to be redesigned from zero      │
│  · Workspace registry + active workspace routing            │
│  · API facade; delegates turns to existing Solomon runtime  │
│  · Responses API + workspace / editor / git HTTP APIs       │
└──────────────────────────────┬──────────────────────────────┘
                               │
         ┌─────────────────────┼─────────────────────┐
         ▼                     ▼                     ▼
 ~/.solomon/config  ~/.solomon/gui/  projects/<hex>/chats  ProjRoot files/git

System terminal (separate):  user runs `solomon` REPL in Ghostty, iTerm, etc.
```

**Data flow (chat turn):**

1. App sets active workspace id (header or `/v1/workspaces/{id}/activate`).
2. UI `POST /v1/responses` with `conversation` + `input`.
3. Daemon loads session, acquires flock, delegates to the existing Solomon turn runtime, and streams SSE to the client.
4. UI renders events into Deep message components; on `completed`, refresh git/file status.

### 4.2 Integration Points

**Existing (keep):**

- `GET/POST /v1/conversations`, `GET /v1/conversations/{id}`
- `POST/GET /v1/responses`, cancel, stream replay
- `GET /v1/health`

**New daemon routes** (port from `ui-prototypes/vite.config.ts` middleware):

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/v1/workspaces` | List registered roots + metadata |
| POST | `/v1/workspaces` | Register workspace (path → hex) |
| POST | `/v1/workspaces/{id}/activate` | Set active workspace for connection |
| GET | `/v1/config/user-name` | Read `user_name` |
| PUT | `/v1/config/user-name` | Write `user_name` |
| GET | `/v1/models/catalog` | Current, recent, providers (from `ui_model_catalog`) |
| PUT | `/v1/session/model` | Switch provider/model for active workspace |
| GET | `/v1/workspace/files` | Tree listing + git status maps |
| GET | `/v1/workspace/search?q=` | Filename filter |
| GET | `/v1/workspace/file?path=` | Read file |
| PUT | `/v1/workspace/file` | Write file (path jail) |
| GET | `/v1/git/branches` | Current + branch list |
| POST | `/v1/git/checkout` | Checkout branch |
| GET | `/v1/git/history` | Graph entries |
| POST | `/v1/git/commit` | Commit staged (message body) |
| GET | `/v1/onboard/status` | `NeedsOnboard` + missing fields |
| POST | `/v1/onboard/*` | Web wizard steps (mirror `/connect`, `/models`) |

**Source and runtime layout (proposed):**

| Path | Role |
|------|------|
| `gui/` | Root of the GUI project: shared frontend tooling and package metadata. |
| `gui/src/` | Source for the production Deep GUI (extracted from `ui-prototypes/src/Deep*`, `shared.tsx`, styles); its build is embedded in `solomon` as the default bundle. |
| `gui/desktop/` | Wails desktop adapter only: tray, daemon lifecycle, and desktop-only PTY bridge. It must not duplicate React UI code. |
| `~/.solomon/gui/` | Intended active GUI bundle location; hosting and installation behavior are deferred. |

**Install artifacts (tri-OS, same release):**

| OS | Daemon | App | Tray |
|----|--------|-----|------|
| macOS | `launchd` user agent | `Solomon.app` | Menu bar extra |
| Linux | `systemd` user unit | `.desktop` + AppImage or binary | StatusNotifierItem |
| Windows | User Windows Service or scheduled task | Start Menu shortcut | Notification area |

### 4.3 Security & Privacy

| Topic | MVP policy |
|-------|------------|
| **Loopback auth** | Requests from `127.0.0.1` / `::1` skip Bearer; configurable off later. |
| **Path jail** | All file/git APIs resolve under active `ProjRoot`; reject `..` and absolute escape (reuse patterns from `replcomplete/path.go`). |
| **GUI bundle integrity** | Deferred with the new host design; the intended user-owned location is `~/.solomon/gui/`. |
| **Terminal panel PTY** | Lives in the **UI layer** (node-pty), not in `solomon daemon`. Spawn shell with cwd = workspace root; full user power (same as REPL `!`). |
| **SSRF** | Not in UI scope; no change to `fetchWeb` — note risk if loopback auth disabled on non-local bind. |

---

## 5. Risks & Roadmap

### 5.1 Phased Rollout

All phases ship toward **full §2.3 parity**; phasing is build order, not scope cuts. Only **P0 — UI foundation** is currently in scope. The daemon- and desktop-dependent phases are intentionally deferred until their architecture is reconsidered.

| Phase | Deliverable | Exit criterion |
|-------|-------------|----------------|
| **P0 — UI foundation (active)** | Extract and organize Deep v5 in `gui/src/`; preserve its current interactive mock behavior behind a client interface | `gui/` runs in Vite and visually/functionally matches Deep v5 without importing `ui-prototypes` |
| **P1 — Daemon architecture (deferred)** | Reconsider daemon scope, UI serving, GUI bundle embed/updates, workspace registry, auth, and desktop lifecycle | Approved daemon design and API contract |
| **P2 — API integration (deferred)** | Implement the approved daemon contract; replace the mock client without reshaping UI components | `gui/src/` works against the approved real backend |
| **P3 — Chat** | Conversations API + SSE wired to Deep chat UI; model/reasoning/fast controls persist to session | End-to-end agent turn with tool events |
| **P4 — Editor + git** | File R/W, autosave, git panel, graph, commit, branch checkout | Edit file on disk; commit from UI |
| **P5 — Terminal panel** | Port lab PTY service (node-pty + WebSocket); wire to Wails/desktop UI | Multi-pane terminal panel matches v5 behavior |
| **P6 — Onboarding** | In-app wizard when `NeedsOnboard` | Fresh install never requires REPL |
| **P7 — Install** | `make install` registers daemon + app + tray on macOS, Linux, Windows | Click icon → app → chat on clean machine |

**v1.1:** Enable passkey auth for non-loopback; optional browser client via `--static-dir`.

**v2.0:** Remote access (Tailscale pairing per mock roadmap); notification push; workspace sync.

### 5.2 Technical Risks

| Risk | Mitigation |
|------|------------|
| **CGO conflict** — main binary uses `CGO_ENABLED=0`; Wails may need CGO | Separate `solomon` (CGO=0) and `solomon-desktop` (CGO=1 if required by Wails); daemon stays CGO=0 |
| **DeepPrototype.tsx > 500 LoC** | Split into `gui/src/agent/`, `gui/src/editor/`, `gui/src/terminal-panel/` during migration |
| **Global daemon + one turn** | Single turn lock globally; queue or 409 with UI message (match current server) |
| **GNOME systray** | Document extension requirement; test KDE + GNOME in parity walkthrough |
| **Windows terminal panel** | Early spike in P5 (node-pty + ConPTY); fallback message if spawn fails |
| **Feature creep from mock data** | Parity checklist §2.3 is the contract; mock threads deleted, not ported |

---

## 6. Task Backlog

Tasks are ordered by phase. Estimate **T-shirt size** only (S/M/L/XL).

### P0 — UI foundation (active)

| ID | Task | Size |
|----|------|------|
| T-001 | Create `gui/` project root with standalone frontend tooling and `gui/src/` from Deep styles, assets, and dependencies | M |
| T-002 | Split `DeepPrototype.tsx` into bounded `gui/src/app/`, `agent/`, `editor/`, `terminal-panel/`, and `shared/` modules | XL |
| T-003 | Define a typed UI client interface and a mock implementation backed by fixtures; no daemon route names leak into components | M |
| T-004 | Move `mock-config.toml` and mock stream behavior into explicit UI development fixtures | M |
| T-005 | Keep visual and interaction parity with `/v5` through a repeatable local walkthrough | M |

### P1 — Daemon architecture (deferred)

No daemon implementation tasks are active. Revisit its lifecycle, process model, API, static GUI serving, embedded fallback, and Wails relationship before creating `internal/daemon/` or `cmd/solomon-desktop/`.

### P2 — API integration (deferred)

The earlier route-by-route backlog (`/v1/workspaces`, files, models, git, and user name) is retired as a provisional design. Define replacement tasks only after P1 approves the daemon contract. The UI client interface from P0 is the compatibility boundary.

### P3 — Chat

| ID | Task | Size |
|----|------|------|
| T-030 | Map conversation list API to Deep folder-grouped sidebar | M |
| T-031 | SSE client: parse turn events → Deep message model (tools, reasoning, files) | L |
| T-032 | Wire composer controls to session config (model, reasoning, fast, agent/chat) | M |
| T-033 | Turn cancel + reconnect (`stream=true&starting_after=N`) | M |
| T-034 | Usage footer component from SSE completion metadata | S |

### P4 — Editor + git

| ID | Task | Size |
|----|------|------|
| T-040 | Wire file tree + search to daemon workspace APIs | M |
| T-041 | CodeMirror autosave → `PUT /v1/workspace/file` | M |
| T-042 | Git staged/changes panels; live refresh on focus + interval | M |
| T-043 | Git graph rendering from `/v1/git/history` | S |
| T-044 | Commit button → `POST /v1/git/commit`; branch checkout UI | M |

### P5 — Terminal panel

| ID | Task | Size |
|----|------|------|
| T-050 | Extract lab PTY service from `ui-prototypes/vite.config.ts`; run under Wails/desktop (node-pty, `/__solomon/terminal` or renamed path) | L |
| T-051 | Wire Deep terminal panel / tabs to UI-layer WebSocket (not daemon) | M |
| T-052 | Tri-OS terminal panel QA (zsh, bash, PowerShell) | M |

### P6 — Onboarding

| ID | Task | Size |
|----|------|------|
| T-060 | `GET /v1/onboard/status` + blocking overlay in app when incomplete | M |
| T-061 | Web wizard: provider connect (OAuth/API key) — reuse `connect` package logic | XL |
| T-062 | Model pick step; write config; resume main UI | M |

### P7 — Install (tri-OS)

| ID | Task | Size |
|----|------|------|
| T-070 | macOS: `launchd` plist template + `Solomon.app` bundle in `make install` | L |
| T-071 | Linux: systemd user unit + `.desktop` entry | M |
| T-072 | Windows: service/task registration + Start Menu shortcut | L |
| T-073 | Install docs update (`docs/user-guide/installation.md`) | S |
| T-074 | Parity walkthrough script/checklist for tri-OS manual QA | M |

### Dependencies requiring explicit approval

| Dependency | Used for | Status |
|------------|----------|--------|
| **Wails v2** | Desktop shell, tray, WebView | **Approved** (2026-07-22) |
| **node-pty** | Terminal panel PTY in UI layer (already in `ui-prototypes`) | Existing; no new dep |

---

## Appendix A — Grilling decisions log

| Decision | Choice |
|----------|--------|
| UI direction | Deep v5 only |
| MVP scope | ≥ prototype feature parity |
| Daemon scope | **Deferred for redesign**; earlier global-user-level proposal is not implementation authority |
| Client | Wails app (not browser-first) |
| Auth (MVP) | Open on loopback |
| Platforms | Tri-OS from first installable milestone |
| REPL | Remains in **system terminal**; not primary onboarding path |
| Terminal naming | **Terminal panel** / **terminal tab** in UI; not “terminal” alone |
| Top bar | Horizontal top chrome (panels, wordmark, switch, path, terminal panel toggle, …) |
| Terminal panel backend | UI layer (node-pty), **not** `solomon daemon` |

## Appendix B — Prototype → production mapping

| Prototype source | Production |
|------------------|------------|
| `ui-prototypes/src/DeepPrototype.tsx` | `gui/src/` (split modules) |
| `gui/` build embedded in `solomon` | Deferred installation design decision |
| `~/.solomon/gui/` | Intended active GUI-bundle location; user-owned mutable files |
| `ui-prototypes/vite.config.ts` middleware (workspace, git, models, …) | UI mock-client behavior for now; later mapping is deferred with daemon design |
| `ui-prototypes/vite.config.ts` terminal plugin | UI-layer PTY service (Wails/desktop); **not** daemon |
| `ui-prototypes/src/shared.tsx` (`TextEditor`, etc.) | `gui/src/shared/` |
| `scripts/ui_model_catalog.go` | Deferred backend integration decision |
| Previous HTTP service | Removed; the replacement design starts from zero |
