# Building and releases

## Development checks

From the repository root:

```bash
go vet ./...
go test ./... -count=1
go build ./cmd/solomon
```

Same checks as [.github/workflows/release.yml](../../.github/workflows/release.yml), including `make check-docs`:

- `check_doc_paths.go` — markdown links between docs, `#` anchors, and cited code paths
- `check_package_index.go` — every Go package under `internal/` and `cmd/` listed in [Package index](../architecture/package-index.md)

Strategy, helpers, and when to mock: **[Testing](testing.md)**. Feature recipes: **[Cookbook](cookbook.md)**.

## Tests (quick reference)

All tests live in [`test/`](../../test/), package `test`. Full guide: [Testing](testing.md).

```bash
go test ./... -count=1
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

- [Project README](../../README.md) — install and requirements
- [Testing](testing.md)
- [Startup and CLI](../architecture/startup-and-cli.md)
