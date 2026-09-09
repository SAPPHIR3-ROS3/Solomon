# Supporting packages

## Purpose

Map of `internal/` packages that support the REPL, tools, auth, and UX but are not the core turn loop. Core orchestration lives in [`internal/agent/runtime/`](../../internal/agent/runtime/) — see [Runtime hub](runtime.md) and [Agent turn pipeline](agent-turn-pipeline.md). Every package path: [Package index](package-index.md).

## Packages

| Package | Path | Role |
|---------|------|------|
| `tooling` | `internal/tooling/` | `Invocation` type; legacy `<tool_calls>` XML parse (`legacy_xml.go`) and stream writer (`legacy_stream.go`); tool name validation |
| `tooloutput` | `internal/tooloutput/` | Truncate oversized tool JSON before the next LLM call; spill full payload to `projects/<id>/temp/` |
| `search` | `internal/search/` | Legacy direct search engines plus the MCP Exa/Parallel router, monthly balance store, and native CloakBrowser fallback for `webSearch` |
| `webfetch` | `internal/webfetch/` | Legacy HTTP fetcher plus MCP Exa/Parallel fetchers, native CloakBrowser fallback, and shared routing metadata for `fetchWeb` |
| `cloak` | `internal/cloak/` | Go-owned lifecycle, JSON-lines process boundary, and DOM snapshot link extraction for the official CloakBrowser wrapper |
| `logging` | `internal/logging/` | Level parsing, file rotation under `~/.solomon/logs` |
| `termcolor` | `internal/termcolor/` | Terminal styling via lipgloss/termenv: dark palette, usage line, image tag colorization, `NO_COLOR` / pipe policy |
| `clipboard` | `internal/clipboard/` | Cross-platform image paste for REPL |
| `title` | `internal/title/` | Chat title slug and LLM title refinement |
| `modelsapi` | `internal/modelsapi/` | List models from provider API (picker, `/models`) |
| `logo` | `internal/logo/` | ASCII logo in welcome banner |
| `atmention` | `internal/atmention/` | `@` tag parsing, shortest unique tags, picker query scoring |
| `instructions` | `internal/instructions/` | Load and cache `AGENTS.md`, `CLAUDE.md`, `GEMINI.md` (global + repo) |
| `auth/openai/codex` | `internal/auth/openai/codex/` | ChatGPT Sub OAuth (PKCE), token refresh, JWT, Codex HTTP middleware |
| `providersetup` | `internal/providersetup/` | Provider-specific setup during `/connect` and onboard wizard |
| `integrations/cursor` | `internal/integrations/cursor/` | Cursor sidecar install dir, health, ensure configured |
| `updater` | `internal/updater/` | GitHub release check, download/install, in-process restart |
| `pathglob`, `gitignore` | `internal/pathglob/`, `internal/gitignore/` | Glob `**` matching and `.gitignore` for `find` |
| `prompt` (shellutils) | `internal/prompt/shellutils/` | Platform-specific shell hints in templates |

## Key entry points

| Symbol | Package | Use |
|--------|---------|-----|
| `tooling.ExtractToolInvocations`, `LegacyStreamWriter` | `tooling` | Legacy XML tool blocks; early stop at `</tool_calls>` |
| `tooloutput.Service`, `applyToolOutput` | `tooloutput` | Truncation and spill from agent turn pipeline |
| `search.Engine` implementations | `search` | Called from `web_search.go`; `internal` resolves to the runtime MCP router |
| `webfetch.Fetcher` implementations | `webfetch` | Called from `fetch_web.go`; `internal` resolves to the runtime MCP fetch router |
| `logging.Log`, `Configure` | `logging` | Startup in `main`, tool errors |
| `termcolor.Init`, `WrapUser`, `UsageTokensLine` | `termcolor` | Startup in `main` / `exec`; REPL prompt and footers |
| `clipboard.PasteImage`, `HasImage` | `clipboard` | Ctrl+V image paste in REPL |
| `title.NormalizeSlug`, gen helpers | `title` | Session titles |
| `modelsapi` list helpers | `modelsapi` | `/connect` / wizard model pick |
| `atmention.ShortTag`, `MatchQuery` | `atmention` | `@` picker and tag expansion |
| `instructions.Loader` | `instructions` | System prompt instruction files |
| `codex.OAuth`, `codex.Refresh` | `auth/openai/codex` | ChatGPT Sub login and token rotation |
| `providersetup.RunProviderSetupByKind` | `providersetup` | `/connect` provider flows |
| `cursor.InstallDir`, `EnsureConfigured` | `integrations/cursor` | Sidecar paths and install |
| `updater.Check`, `updater.Install` | `updater` | `/update`, `/upgrade`, `/autoupdate` |

## Tool output spill

When a tool result exceeds `[tool_output]` limits, `tooloutput.Service` keeps a truncated summary for the model and writes the full body under `~/.solomon/projects/<id>/temp/`. Cross-process cleanup is coordinated via `~/.solomon/temp.txt`. User guide: [Data layout — Tool output spill](../user-guide/data-layout.md#tool-output-spill-temp). Tests: [`test/tooloutput_test.go`](../../test/tooloutput_test.go), [`test/tool_output_integration_test.go`](../../test/tool_output_integration_test.go).

## ChatGPT Sub auth

Providers with ChatGPT subscription OAuth store tokens in `config.toml` today. Flow: browser PKCE via [`oauth.go`](../../internal/auth/openai/codex/oauth.go) → refresh via [`Refresh`](../../internal/auth/openai/codex/oauth.go) → request shaping in [`chatgpt_middleware.go`](../../internal/auth/openai/codex/chatgpt_middleware.go). Wired from `/connect` through `providersetup` and `config`. Tests: [`test/provider_auth_test.go`](../../test/provider_auth_test.go), [`test/codex_chat_request_test.go`](../../test/codex_chat_request_test.go).

## Cursor sidecar (Go side)

Full architecture: [Cursor integration](cursor-integration.md). Install paths and process manager: [`internal/integrations/cursor/`](../../internal/integrations/cursor/). Runtime: [`cursor_sidecar.go`](../../internal/agent/runtime/cursor_sidecar.go). User TOML: [Configuration — Cursor integration](../user-guide/configuration.md#cursor-integration-tool-execution).

## Extension points

- New legacy web search engine: add file in `internal/search/`, register it and update config docs. Exa/Parallel MCP adapters are host-managed factories built from `mcp.json`; CloakBrowser is a native local fallback.
- Logging: `log_level` in config TOML.
- Terminal colors: env vars and TTY detection in `termcolor.Init` — see [Terminal setup](../user-guide/terminal-setup.md).
- Instruction files: extend `instructions/discover.go` and prompt render — see [Project instructions](../user-guide/project-instructions.md).

## Related code

- [`internal/search/engine.go`](../../internal/search/engine.go)
- [`internal/tooloutput/service.go`](../../internal/tooloutput/service.go)
- [`internal/instructions/loader.go`](../../internal/instructions/loader.go)
- [`internal/auth/openai/codex/oauth.go`](../../internal/auth/openai/codex/oauth.go)
- [`internal/updater/check.go`](../../internal/updater/check.go)

## See also

- [Configuration](../user-guide/configuration.md)
- [Native tools](native-tools.md)
- [LLM layer](llm-layer.md)
- [Runtime hub](runtime.md)
