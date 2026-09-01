# Local server

The `solomon server` process is a user-scoped, detached local service. It is manually started and stopped; it is not tied to a workspace, a shell, or the current working directory.

## Responsibilities

- Serve the local web surface.
- Provide a localhost API boundary for the GUI and machine interactions.
- Own the GUI-facing project, chat, image and streaming endpoints.
- Supervise child processes required by the active mode, including daemon-owned
  PTY sessions.

In addition to `GET /health`, the server exposes the project, chat, customization,
model and terminal APIs below. A chat run constructs the same
`internal/agent/runtime.Runtime` used by `solomon exec --json`; the GUI receives
its machine events over an SSE stream and reads the persisted session for the
final snapshot. The run belongs to the daemon, not to an individual HTTP
connection: a browser reload only closes its SSE subscription, and a later
`/events` subscription replays the buffered events. Only the explicit stop
endpoint or daemon shutdown cancels the run. Distinct chats can run concurrently
across projects.

## GUI chat API

| Route | Purpose |
|---|---|
| `GET /__solomon/projects` | List registered projects, chat summaries and project stats. |
| `POST /__solomon/projects` | Register a local folder as a project. |
| `POST /__solomon/projects/<project>/chats` | Create an empty persisted chat. |
| `GET /__solomon/projects/<project>/chats/<chat>` | Open a persisted chat and map its transcript for the GUI. |
| `POST /__solomon/projects/<project>/chats/<chat>/messages` | Run a prompt and stream runtime events as SSE. |
| `GET /__solomon/projects/<project>/chats/<chat>/events?starting_after=N` | Attach to an active run and replay events after sequence `N`; returns the persisted snapshot when no run is active. |
| `DELETE /__solomon/projects/<project>/chats/<chat>/messages/<message>` | Delete a user turn and its following assistant/tool records. |
| `POST /__solomon/projects/<project>/chats/<chat>/stop` | Interrupt the active chat run. |
| `GET /__solomon/projects/<project>/chats/<chat>/images/<seq>` | Serve a validated persisted chat image. |
| `GET/POST /__solomon/projects/<project>/subchats/<subchat>` | Read or control a persisted subagent chat. |
| `GET /__solomon/projects/<project>/files?path=...` | List one directory level inside a registered project. |
| `GET /__solomon/projects/<project>/{branches,history,status,worktrees}` | Read project Git state. |
| `POST /__solomon/projects/<project>/checkout` | Checkout an existing project branch. |
| `GET /__solomon/projects/<project>/research` | List persisted research jobs. |
| `GET /__solomon/home-directories?path=...` | List one directory level under the user home. |

Customization, model and terminal routes are also daemon-owned:

| Route | Purpose |
|---|---|
| `/__solomon/rules`, `/__solomon/skills`, `/__solomon/mcps`, `/__solomon/subagents` | Read global customization catalogs. |
| `/__solomon/rules/*`, `/__solomon/subagents/*`, `/__solomon/roles-table`, `/__solomon/promptTemplate*` | Mutate customization and prompt-template state. |
| `GET /__solomon/models` | Read configured providers, models and visibility. |
| `PUT /__solomon/current-model`, `PUT /__solomon/model-visibility`, `POST /__solomon/connect-provider` | Change model/provider configuration. |
| `GET /__solomon/terminal?path=...` (WebSocket upgrade) | Attach to a daemon-owned shell PTY with replay and resize. Add `mode=tui` for the Solomon REPL. |

The browser and Wails GUI use these same daemon routes. The Vite middleware
contains a development fallback for opening the frontend directly, but it is not
the ownership boundary for a server-backed client; the Wails bridge is used only
to discover the daemon URL during desktop development.

## Lifecycle

| Command | Behavior |
|---|---|
| `solomon server start` | Start the detached server in normal mode. |
| `solomon server start dev <gui-directory>` | Start development mode with the specified GUI project. The directory must contain `package.json` and `src/`. |
| `solomon server status` | Print the PID, local and network URLs, mode, version, Vite status, and start time. |
| `solomon server stop` | POST `/_solomon/stop`, wait for shutdown, then remove runtime state. If health fails or the process does not exit in time, the CLI force-stops the recorded PID and clears stale `state.json`. |
| `solomon server restart` | Preserve the prior mode and development directory, then restart. |
| `solomon server logs` | Print the recent server log. |
| `solomon server logs interactive` | Continue streaming the server log until interrupted. |

`make install` stops any running local server (via `go run ./cmd/solomon server stop`) before replacing the binary.

`solomon server status` reports `stopped` when `state.json` is missing or `/health` fails. After a crash or interrupted shutdown, leftover state is cleared on the next successful `stop` (unreachable host or unhealthy process).

## Networking and health

The server listens on the TCP port configured by `SOLOMON_SERVER_PORT` when it is set, and selects a free port otherwise. It listens on all IPv4 interfaces by default. `solomon server start` and `solomon server status` print the loopback URLs plus every discovered non-loopback IPv4 address. Addresses in the local network, or on another active IPv4 interface, are labelled `local`; Tailscale addresses in `100.64.0.0/10` are labelled `tailscale`. The same list is stored in `state.json` and returned by `GET /health`.

The server currently has no authentication layer. Binding to network interfaces
therefore makes the GUI, API and PTY reachable by other devices that can access
the host; use the host firewall and Tailscale ACLs to limit access to trusted
clients. Browser API requests and terminal upgrades reject unrelated origins,
but this is not a substitute for authentication on an untrusted network.

`GET /health` returns JSON with `ok`, server PID/version/mode/URLs, the discovered `addresses` list, start time, Go runtime details, Vite status and development directory when present, plus the existing API, GUI and worker readiness fields. It is the readiness check used by the CLI.

## Development frontend

In `dev` mode the server selects a free loopback port, then starts `npm run dev -- --host 127.0.0.1 --port <free-port>` in the supplied GUI directory. The Vite process stays private on its own random loopback port; the Solomon server reverse-proxies it at the URL advertised in runtime state, including WebSocket traffic required by hot reload.

The desktop development launcher reads the running server state, verifies its health endpoint, and passes its current local URL to Wails. Both a browser and the desktop WebView therefore consume the same GUI project, the same Vite process and the same daemon APIs even if the server port changes. When the server exits it terminates the complete Vite process group, avoiding an orphaned frontend process.

The terminal panel deliberately closes only its WebSocket subscription when a
tab becomes hidden or the GUI reloads. The daemon keeps the shell or TUI process
alive, retains a bounded output buffer, and accepts a later connection with the
session id and `after` cursor. A daemon restart invalidates those in-memory
sessions; the client then starts a fresh session instead of retrying a dead id.

## Runtime files

| Path | Content |
|---|---|
| `~/.solomon/run/server/state.json` | Runtime state used by lifecycle commands and readiness checks. |
| `~/.solomon/logs/server/server.log` | Detached server stdout and stderr. |

## Code map and tests

- [`cmd/solomon/server/`](../../cmd/solomon/server/) owns command parsing, detaching, logs, and lifecycle requests (including stale-state reclaim on `stop`).
- [`internal/server/service.go`](../../internal/server/service.go) owns listening, state, health, Vite startup, proxying, and shutdown.
- [`internal/server/chat_api.go`](../../internal/server/chat_api.go) owns daemon chat runs, SSE replay and persisted snapshots.
- [`internal/server/project_api.go`](../../internal/server/project_api.go), [`customization_api.go`](../../internal/server/customization_api.go) and [`model_api.go`](../../internal/server/model_api.go) own the non-chat GUI surfaces.
- [`internal/server/terminal.go`](../../internal/server/terminal.go) and [`terminal_api.go`](../../internal/server/terminal_api.go) own PTY sessions, output replay, resize and WebSocket attachment.
- [`internal/server/process_windows.go`](../../internal/server/process_windows.go) / [`process_unix.go`](../../internal/server/process_unix.go) own child-process teardown and `ForceStopPID`.
- [`scripts/desktop_dev.go`](../../scripts/desktop_dev.go) reads the server state and starts Wails with its current local server URL.
- [`test/server_runtime_test.go`](../../test/server_runtime_test.go) starts real local server processes with a fake Vite command to verify health, proxying, stop, and child cleanup.

## See also

- [Startup and CLI](startup-and-cli.md)
- [Usage and commands](../user-guide/usage-and-commands.md#local-server)
- [Data layout](../user-guide/data-layout.md#local-server-runtime)
