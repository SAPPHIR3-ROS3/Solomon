# Troubleshooting

Start with the smallest failing operation. Do not delete your Solomon home or session files as a first recovery step. Paths below use the default `~/.solomon`; when `SOLOMON_HOME` is set, inspect that directory instead.

## Collect diagnostics

```bash
solomon version --binary
solomon server status
solomon server logs
```

`version --binary` identifies the installed CLI independently of daemon health. `solomon version` reports the running daemon version. Record both when investigating an update mismatch. Include your OS, CPU architecture, terminal, installation method, exact command and a minimal reproduction.

General logs are under `~/.solomon/logs/`. The daemon writes `logs/server/server.log`; updates write `logs/update/update.log`. For a running incident, `solomon server logs interactive` follows the daemon log until interrupted. The `/log` command changes the configured log level; restart the application for that change to take effect.

Before sharing logs, redact API keys, OAuth access/refresh tokens, Authorization headers, MCP credentials, signed URLs, private paths and conversation content. Do not attach `config.toml`, `mcp.json`, session JSON or your complete Solomon home without reviewing and sanitizing them.

## Command not found

Reopen the terminal after installation. Check `go env GOBIN` and `go env GOPATH`: Go installs into GOBIN when set, otherwise into the first GOPATH entry's `bin` directory. On Unix use `command -v solomon`; on PowerShell use `Get-Command solomon -All` to detect an older copy earlier on PATH. Follow [Installation and PATH](installation.md#binary-location) rather than reinstalling repeatedly.

## Daemon does not start or the port is occupied

Read `solomon server logs` and `solomon server status` first. The default port is `64000`; `server_port` in the configuration and `SOLOMON_SERVER_PORT` can change it. An explicit environment value takes precedence and is persisted. Identify the process using that port with your OS networking tools before stopping anything. Never kill an unrelated process merely to free the default port.

If Solomon owns stale runtime state, use `solomon server stop` followed by `solomon server start`. These commands reclaim recorded daemon/Vite processes after a crash; do not manually remove `run/server/state.json` while those processes may still be running. Stopping or restarting the daemon interrupts active chats and terminal processes.

For a deliberate port change, stop Solomon, set `SOLOMON_SERVER_PORT` in the shell that starts it, then start the server again. Use the URLs printed by `solomon server status`, not a remembered address. The server has no authentication layer and listens on network interfaces: restrict access with your firewall and trusted-network policy. See [Local server](../architecture/server.md).

## GUI is blank, stale or disconnected

Confirm the daemon is healthy and open its reported loopback URL in a browser. Browser and desktop use the same daemon; a desktop window closing does not stop it. Compare the daemon version with `solomon version --binary` before diagnosing frontend caching.

For source builds, run `make gui-build` before rebuilding the CLI; plain `go build` uses the last staged frontend. Development mode needs a GUI directory containing `package.json` and `src/`, plus working Node/npm dependencies. Use `solomon server start dev /absolute/path/to/Solomon/gui` when the server is stopped. See [Building and releases](../development/building-and-releases.md).

## Provider authentication or model errors

Use `/models` to inspect the selected provider/model and `/connect` to run the appropriate provider setup. Distinguish an invalid credential (401/403), an unavailable model, rate limiting (429), and network/TLS failures. Check the provider URL and account permissions; an OpenAI-compatible endpoint is not interchangeable with an Anthropic Messages endpoint. Subscription providers require their own login flow rather than an arbitrary API key.

If login expired, repeat the provider's setup/login flow. Do not paste tokens into an issue or disable TLS verification. For local providers, confirm the service is running and the configured model exists. See [Configuration](configuration.md).

## MCP server or tools are missing

Use `/mcp` to inspect servers and `/integrations` for integration health. Review the relevant entry in `~/.solomon/mcp.json`, transport, executable path, environment and credentials. A command available in an interactive shell may be unavailable to a detached daemon with a different PATH; prefer an explicit executable path when needed.

Check the server log for connection errors and review tool allow/deny rules before assuming discovery failed. After changing startup environment or MCP configuration, restart Solomon when no work is running. See [MCP integration](../architecture/mcp-integration.md).

## Installation or upgrade failed

Read the original error and `~/.solomon/logs/update/update.log` when an upgrade coordinator ran. Downloads require matching release checksums: do not bypass verification to force an installation. Check network/proxy access, disk space and permissions on the installation directory.

Standalone release installers require Solomon to be closed. Use `solomon upgrade` for an active release installation; source users should use the coordinated `make hot-install` flow. Preparation failures should be resolved before retrying rather than replacing binaries manually.

## Platform-specific checks

- **Windows:** run the documented installer from PowerShell with a process-scoped execution-policy bypass; do not weaken machine-wide policy. Verify WebView2 for desktop use. Extended ANSI colors in readline prompts have a known limitation; see [Terminal setup](terminal-setup.md).
- **Linux:** desktop runtime needs GTK3/WebKitGTK 4.1; native source builds also need development headers and pkg-config. The CLI alone does not need those desktop build headers.
- **macOS:** Homebrew setup may require Xcode Command Line Tools and administrator approval. For REPL image paste use `Ctrl+V`; `Cmd+V` is handled by the terminal and is not a universal image-paste shortcut.

## See also

- [Installation and PATH](installation.md)
- [Configuration](configuration.md)
- [Data layout](data-layout.md)
- [Contributing](../development/contributing.md)
