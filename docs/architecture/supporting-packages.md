# Supporting packages

## Purpose

Brief map of smaller `internal/` packages that support the REPL, tools, and UX but are not core orchestration.

## Packages

| Package | Path | Role |
|---------|------|------|
| `search` | `internal/search/` | DuckDuckGo, SearxNG, Google PSE, Brave, Bing backends for `webSearch` |
| `logging` | `internal/logging/` | Level parsing, file rotation under `~/.solomon/logs` |
| `termcolor` | `internal/termcolor/` | Terminal styling via lipgloss/termenv: dark palette, usage line, image tag colorization, `NO_COLOR` / pipe policy |
| `clipboard` | `internal/clipboard/` | Cross-platform image paste for REPL |
| `title` | `internal/title/` | Chat title slug and LLM title refinement |
| `modelsapi` | `internal/modelsapi/` | List models from provider API (picker, `/models`) |
| `logo` | `internal/logo/` | ASCII logo in welcome banner |
| `prompt` (shell) | `prompt/effective_shell_*.go` | Platform-specific shell hints in templates |

## Key entry points

| Symbol | Package | Use |
|--------|---------|-----|
| `search.Engine` implementations | `search` | Called from `web_search.go` |
| `logging.Log`, `Configure` | `logging` | Startup in `main`, tool errors |
| `termcolor.Init`, `WrapUser`, `UsageTokensLine` | `termcolor` | Startup in `main` / `exec`; REPL prompt and footers |
| `clipboard.PasteImage`, `HasImage` | `clipboard` | Ctrl+V image paste in REPL |
| `title.NormalizeSlug`, gen helpers | `title` | Session titles |
| `modelsapi` list helpers | `modelsapi` | `/connect` / wizard model pick |

## Extension points

- New web search engine: add file in `internal/search/`, register in `web_search.go` and config docs.
- Logging: `log_level` in config TOML.
- Terminal colors: env vars and TTY detection in `termcolor.Init` — see [Terminal setup](../user-guide/terminal-setup.md).

## Related code

- [`internal/search/engine.go`](../../internal/search/engine.go)
- [`internal/logging/logging.go`](../../internal/logging/logging.go)
- [`internal/termcolor/init.go`](../../internal/termcolor/init.go)

## See also

- [Configuration](../user-guide/configuration.md)
- [Native tools](native-tools.md)
- [LLM layer](llm-layer.md)
