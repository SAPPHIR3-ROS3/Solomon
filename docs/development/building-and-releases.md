# Building and releases

## Development checks

From the repository root:

```bash
go vet ./...
go test ./... -count=1
CGO_ENABLED=1 go test ./... -count=1 -race  # Linux CI parity
go build ./cmd/solomon
```

Same checks as [.github/workflows/release.yml](../../.github/workflows/release.yml), including `make check-docs`. The Linux matrix job enables CGO and the race detector; macOS and Windows run the non-race suite:

```bash
CGO_ENABLED=1 go test ./... -count=1 -race
```

The workflow also runs:

- `check_doc_paths.go` — markdown links between docs, `#` anchors, and cited code paths
- `check_package_index.go` — every Go package under `internal/` and `cmd/` listed in [Package index](../architecture/package-index.md)

Strategy, helpers, and when to mock: **[Testing](testing.md)**. Feature recipes: **[Cookbook](cookbook.md)**.

## Tests (quick reference)

All tests live in [`test/`](../../test/), package `test`. Full guide: [Testing](testing.md).

```bash
go test ./... -count=1
CGO_ENABLED=1 go test ./... -count=1 -race  # Linux CI parity
go test ./test -run TestSlashDispatch -count=1
```

### Coverage map (summary)

| Area | Example test files |
|------|-------------------|
| Slash dispatch | `slash_dispatch_test.go` |
| Legacy XML tools | `legacy_tools_test.go`, `legacy_runtime_test.go` |
| REPL editor / completion | `repl_editor_test.go`, `repl_complete_test.go`, `repl_complete_path_test.go` |
| Checkpoints | `checkpoint_truncate_test.go`, `checkpoint_staging_test.go` |
| LLM / stream | `stream_integrity_test.go`, `api_resilience_test.go`, `anthropic_test.go` |
| Tools | `edit_file_test.go`, `find_test.go`, `tooloutput_test.go` |
| Skills | `skills_test.go`, `skills_search_test.go` |
| MCP | `mcp_config_test.go`, `mcp_adapter_test.go` |
| Deep research / web surfaces | `web_surfaces_test.go`, `research_store_test.go` |
| Auth / Codex | `provider_auth_test.go`, `codex_chat_request_test.go` |
| CI events | `cievents_test.go` |
| Cursor integration | `cursor_paths_test.go`, `stream_cursor_tool_test.go` — [Cursor integration](../architecture/cursor-integration.md#debug-playbook) |
| Updater | `updater_test.go`, `commands_update_test.go` |

Prefer regression tests in `test/` when fixing REPL, turn pipeline, or tool behavior.

## Local build via Makefile

```bash
make build
```

Produces `solomon` (Unix/macOS) or `solomon.exe` (Windows). `CGO_ENABLED=0` per [Makefile](../../Makefile).
Builds from a non-tagged or dirty checkout carry a `<base>-dev-<commit>` version
and are compared against the latest release commit before startup auto-update.
A release version can be supplied
explicitly with `VERSION=vYYYY.MDD.N make build` when needed.

Makefile and release builds also embed the Git source-tree hash captured before
compilation and version stamping. The update check displays the release tag only
when both the full current commit and this input tree match the release's source
commit. Uncommitted source changes retain `-dirty`; a newer commit retains `-dev`.
Generated outputs ignored by Git do not change this source fingerprint. The
release workflow creates a tag on the source commit, so that commit is the
pre-release baseline; its parent would omit the last actual code change.
Executable checksums still verify downloads, but do not determine development
status. Builds made directly with `go build` lack this source snapshot and cannot
be promoted to a release by the source-tree comparison.

The running daemon is the version authority for the desktop app, TUI and
`solomon version`. Use `solomon version --binary` only to inspect the installed
binary independently of the daemon. If no daemon is available, the normal
version command reports that fact instead of substituting a binary version.

On Linux, Windows and macOS, `solomon upgrade` and TUI updates use a detached coordinator. It
downloads and verifies the release before interrupting work, closes the native
Solomon clients, stops the daemon and its PTYs/workers, replaces the binary, and
starts the daemon with the release's bundled production UI. Desktop windows are
reopened and attached TUIs re-enter through the installed CLI. The desktop shell
loads UI assets and API data from the daemon, so it does not retain a separate
frontend version. A failed installation restores the previous binary and restarts
the runtime. Progress and errors are written to `~/.solomon/logs/update/update.log`
(under `SOLOMON_HOME` when configured). Native clients register a graceful shutdown handshake on all three platforms.
Windows runs the coordinator from a separate executable copy to release installation
locks, and reopens TUI transports in a new console. Unix TUI transports resume in
their existing terminal. CI runs the runtime handoff tests on all three platforms.

## Application icon

The shared icon is generated from the terminal and GUI Braille art in
[`internal/logo/logo.txt`](../../internal/logo/logo.txt) and its color map in
[`internal/logo/colors.txt`](../../internal/logo/colors.txt). The Go renderer is
[`internal/logo/icon.go`](../../internal/logo/icon.go). After editing either
source file, run:

```bash
go generate ./internal/logo
```

This updates the root `icon.svg` and `icon.png`, plus the desktop app assets.
The GUI's Vite configuration runs the generator before serving or building, so
the favicon and Wails platform icons stay in sync. Treat generated icon files as
outputs; edit the two source maps instead.

## Install from module path

Module path: `github.com/SAPPHIR3-ROS3/Solomon/v2026`

```bash
go install github.com/SAPPHIR3-ROS3/Solomon/v2026/cmd/solomon@latest
go install github.com/SAPPHIR3-ROS3/Solomon/v2026/cmd/solomon@v2026.527.2
```

Release tags use calendar semver `vYYYY.MDD.N` (month×100+day in the middle component, e.g. `v2026.527.2` = 2026, May 27, revision 2).

## Yearly module path updates

When the calendar year advances, create a new major module path (e.g. `.../Solomon/v2027`) and update imports. Older paths remain installable via their existing tags (`@v2026.527.2` on `/v2026`, etc.).

## Releases

### CI and calendar tags

Push and pull requests run vet, test, and build ([release.yml](../../.github/workflows/release.yml)).

**Actions → Release → Run workflow** creates tag `vYYYY.MDD.N`, GitHub release assets, and a GitHub release.

Prebuilt binaries are attached per platform; the install scripts download those assets by default.

## See also

- [Installation and PATH](../user-guide/installation.md)
- [Overview](../architecture/overview.md)
- [Testing](testing.md)
- [Startup and CLI](../architecture/startup-and-cli.md)

## Native desktop client (Linux)

The desktop client embeds the production React UI. It connects to the same
Solomon daemon used by the web client and starts that daemon when needed.
Closing the window leaves daemon-owned chats and terminals running.

Install the GTK 3 and WebKitGTK 4.1 development packages first. On Debian:

```bash
sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev
make desktop-install
```

This installs `solomon` and `solomon-desktop` in `BIN_DIR` (the Go binary
directory by default), plus the Solomon icon and launcher under
`${XDG_DATA_HOME:-~/.local/share}`. Open **Solomon** from **Show Applications**.
`desktop-install` preserves a running daemon, including its development mode.
The desktop gateway adapts native WebView requests for older running daemons.

On Linux, `make hot-install` updates the CLI, web frontend, integrations and
native desktop client, including its icon and application-menu entry, then
restarts the daemon in its previous mode. It builds the native client before
stopping the daemon; missing GTK/WebKit development packages abort the command
while the existing daemon keeps running. Use `make desktop-install` for a desktop
update that preserves the running daemon.

For separate builds:

```bash
make gui-build       # compile and stage the shared embedded frontend
make desktop-build   # build gui/desktop/build/bin/solomon-desktop
```

`make build` and `make install` also compile the web frontend. Direct `go build`
uses the last staged frontend; run `make gui-build` first to refresh it.
`SOLOMON_BINARY` can point the desktop client at a CLI outside its own directory.
