# Local server

The `solomon server` process is a user-scoped, detached local service. It is manually started and stopped; it is not tied to a workspace, a shell, or the current working directory.

## Responsibilities

- Serve the local web surface.
- Provide a localhost API boundary for future Solomon and machine interactions.
- Supervise child processes required by the active mode.

The current prototype exposes only `GET /health`. API routes, Solomon worker processes, installed GUI assets, authentication, and the `solomon web` / `solomon desktop` launch commands are not implemented yet.

## Lifecycle

| Command | Behavior |
|---|---|
| `solomon server start` | Start the detached server in normal mode. |
| `solomon server start dev <gui-directory>` | Start development mode with the specified GUI project. The directory must contain `package.json` and `src/`. |
| `solomon server status` | Print the PID, local URLs, mode, version, Vite status, and start time. |
| `solomon server stop` | Request a graceful stop and remove runtime state. |
| `solomon server restart` | Preserve the prior mode and development directory, then restart. |
| `solomon server logs` | Print the recent server log. |
| `solomon server logs interactive` | Continue streaming the server log until interrupted. |

`make install` stops the installed server before replacing the binary.

## Networking and health

The server listens only on `127.0.0.1:8765`; its equivalent browser URL is `http://localhost:8765`. It does not bind to a network interface, so it has no authentication layer at this stage.

`GET /health` returns JSON with `ok`, server PID/version/mode/URLs/start time, Go runtime details, Vite status and development directory when present, plus placeholder statuses for API, GUI, and workers. It is the readiness check used by the CLI.

## Development frontend

In `dev` mode the server starts `npm run dev -- --host 127.0.0.1 --port <free-port>` in the supplied GUI directory. The Vite process stays private on its random loopback port; the Solomon server reverse-proxies it at `http://localhost:8765`, including WebSocket traffic required by hot reload.

The desktop Wails project uses that same stable server URL in development. Both a browser and the desktop WebView therefore consume the same GUI project and the same Vite process. When the server exits it terminates the complete Vite process group, avoiding an orphaned frontend process.

## Runtime files

| Path | Content |
|---|---|
| `~/.solomon/run/server/state.json` | Runtime state used by lifecycle commands and readiness checks. |
| `~/.solomon/logs/server/server.log` | Detached server stdout and stderr. |

## Code map and tests

- [`cmd/solomon/server/`](../../cmd/solomon/server/) owns command parsing, detaching, logs, and lifecycle requests.
- [`internal/server/service.go`](../../internal/server/service.go) owns listening, state, health, Vite startup, proxying, and shutdown.
- [`gui/desktop/wails.json`](../../gui/desktop/wails.json) points Wails development at the stable local server URL.
- [`test/server_runtime_test.go`](../../test/server_runtime_test.go) starts real local server processes with a fake Vite command to verify health, proxying, and child cleanup.

## See also

- [Startup and CLI](startup-and-cli.md)
- [Usage and commands](../user-guide/usage-and-commands.md#local-server)
- [Data layout](../user-guide/data-layout.md#local-server-runtime)
