# Testing

How Solomon tests are organized, which style to use, and shared helpers. Commands and CI: [Building and releases](building-and-releases.md).

## Layout

| Rule | Detail |
|------|--------|
| Location | All tests in top-level [`test/`](../../test/), package name `test` |
| Colocation | Do **not** add `*_test.go` next to `internal/` sources |
| Init | [`test/init_test.go`](../../test/init_test.go) — `TestMain` sets logging for the suite |
| CI | `go vet ./...`, `go test ./... -count=1`, `make check-docs` (doc links, anchors, code paths, package index) ([`release.yml`](../../.github/workflows/release.yml)) |

Run everything:

```bash
go test ./... -count=1
```

Focused:

```bash
go test ./test -run TestSlashDispatch -count=1
go test ./test -run TestLegacy -count=1
go test ./test -run TestRepl -count=1
```

## Test pyramid (Solomon)

| Level | Use when | Typical setup |
|-------|----------|---------------|
| **Pure unit** | Parsers, labels, glob, legacy XML, atmention scoring | Call `internal/` directly; no disk/network |
| **Component** | Slash handlers, editor keys, completion | [`testDeps`](../../test/helpers_test.go), [`*ForTest`](../../internal/agent/runtime/repl/editor/testexport.go) |
| **HTTP stub** | LLM resilience, Anthropic/OpenAI streams, Codex request shape | [`httptest.Server`](../../test/api_resilience_anthropic_test.go), mock OpenAI client |
| **Filesystem** | `editFile`, skills registry, chatstore paths | [`t.TempDir()`](../../test/edit_file_test.go), temp project root |
| **Minimal runtime** | Turn resolution, legacy force, tool routing | `&agentruntime.Runtime{...}` with fields set — no full REPL ([`legacy_runtime_test.go`](../../test/legacy_runtime_test.go)) |

There is **no** project-wide mock framework. Prefer real temp dirs and small structs over heavy interfaces.

## When to mock vs integrate

### Prefer real / local

- File tools (`readFile`, `editFile`, `find`) — real files under `t.TempDir()`
- Slash dispatch — real handler funcs via `testDeps`, capture `bytes.Buffer` on `Out`
- REPL editor — `NewMultilineEditorForTest` with real completion env where needed
- Checkpoint staging — temp session dirs ([`checkpoint_staging_test.go`](../../test/checkpoint_staging_test.go))
- Config load/save — temp `~/.solomon` home when tests override paths

### Prefer stub / fake

- **LLM HTTP** — `httptest` servers; never call live provider APIs in CI
- **Provider client in slash tests** — dummy base URL in `testDeps` (`http://127.0.0.1:9`)
- **`SaveCfg` / MCP connect** — no-op callbacks in `testDeps` unless the test targets persistence
- **Cursor sidecar process** — test Go install paths and stream bridge without starting Node when possible ([`cursor_paths_test.go`](../../test/cursor_paths_test.go))

### Avoid

- Asserting exact ANSI escape sequences — test classification, substrings, or plain `termcolor` helpers
- Network to GitHub/releases in unit tests (updater tests stub HTTP)
- Full interactive REPL driving a real TTY

## Shared helpers

| Helper | File | Purpose |
|--------|------|---------|
| `testDeps(sess)` | [`helpers_test.go`](../../test/helpers_test.go) | Minimal [`commands.Deps`](../../internal/agent/commands/deps.go) for slash tests |
| `NewMultilineEditorForTest` | [`editor/testexport.go`](../../internal/agent/runtime/repl/editor/testexport.go) | Editor buffer, keys, completion without a terminal |
| `NewInputHistoryForTest` | same | Input/shell history |
| `ReplCompleteResetGoCacheForTest` | [`replcomplete/go.go`](../../internal/agent/runtime/replcomplete/go.go) | Reset `go` subcommand cache between tests |
| `testToolOutputService` | [`tooloutput_test.go`](../../test/tooloutput_test.go) | Tool spill limits |

When REPL behavior needs new assertions, add **`ForTest` exports** in `editor/testexport.go` rather than exporting production APIs.

## Conventions

- **Naming:** `TestArea_behavior` or `TestFunction_scenario` — match the file you extend (`TestSlashDispatch_*`, `TestResolveTurnInvocations_*`)
- **Regression:** bug fix should include a test that fails without the fix
- **Table-driven:** use when the file already does; not mandatory everywhere
- **Parallel:** rare; default sequential is fine
- **Cursor bundle:** `make test` / CI run `cursor_bundler` before tests ([`Makefile`](../../Makefile))

## Coverage map

| Area | Test files | Architecture / debug |
|------|------------|----------------------|
| Slash | `slash_dispatch_test.go` | [Skills and slash](../architecture/skills-and-slash.md) |
| Legacy XML | `legacy_tools_test.go`, `legacy_runtime_test.go` | [Agent turn pipeline](../architecture/agent-turn-pipeline.md) |
| REPL / completion | `repl_editor_test.go`, `repl_complete_*.go`, `repl_highlight_test.go` | [Runtime — REPL](../architecture/runtime-repl.md) |
| Checkpoints | `checkpoint_truncate_test.go`, `checkpoint_staging_test.go` | [Checkpoints](../architecture/checkpoints.md) |
| LLM / stream | `stream_integrity_test.go`, `api_resilience_*.go`, `anthropic_*.go` | [LLM layer](../architecture/llm-layer.md) |
| Tools | `edit_file_test.go`, `find_test.go`, `read_file_pagination_test.go` | [Native tools](../architecture/native-tools.md) |
| Tool output | `tooloutput_test.go`, `tool_output_integration_test.go` | [Supporting packages](../architecture/supporting-packages.md) |
| Skills | `skills_test.go`, `skills_search_test.go` | [Skills and slash](../architecture/skills-and-slash.md) |
| MCP | `mcp_config_test.go`, `mcp_adapter_test.go` | [MCP integration](../architecture/mcp-integration.md) |
| Auth / Codex | `provider_auth_test.go`, `codex_*_test.go` | [LLM layer — ChatGPT Sub](../architecture/llm-layer.md) |
| CI events | `cievents_test.go` | [Runtime orchestration](../architecture/runtime-orchestration.md) |
| Cursor | `cursor_paths_test.go`, `stream_cursor_tool_test.go`, `cursor_native_display_test.go` | [Cursor integration](../architecture/cursor-integration.md) |
| Updater | `updater_test.go`, `commands_update_test.go` | [Supporting packages](../architecture/supporting-packages.md) |

## Runtime-specific notes

When testing turn/cancel behavior:

- User stop uses `context.WithCancelCause` and `errUserStopGeneration` ([`turns.go`](../../internal/agent/runtime/turns.go)) — assert on `errors.Is(context.Cause(ctx), ...)` if you add cancel tests
- Stream integrity rejection must **not** persist partial assistant content ([`stream_integrity_test.go`](../../test/stream_integrity_test.go))
- Session mutation should go through runtime helpers / `mutateSession`, not raw races on `Session` in production code

## Node sidecar tests

TypeScript unit tests live under [`integrations/cursor/test/`](../../integrations/cursor/test/). Run:

```bash
make cursor-proxy-test
```

Go tests cover the bridge and install paths; full Cursor SDK runs are not required in CI for most Go changes.

## See also

- [Cookbook](cookbook.md) — what to test when adding features
- [Building and releases](building-and-releases.md)
- [Runtime hub — Debug playbook](../architecture/runtime.md#debug-playbook)
