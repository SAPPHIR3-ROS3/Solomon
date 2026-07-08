# Package index

Canonical map of every Go package under `internal/` and `cmd/`, plus non-Go integration roots. Use this when you need to find where something lives before reading code.

**Tiers**

| Tier | Meaning |
|------|---------|
| Core | Entry, runtime loop, LLM, persistence — start here for turn/REPL bugs |
| Feature | Tools, slash, skills, MCP, checkpoints — behavior you extend |
| Support | Config, paths, UX helpers, search, auth plumbing |
| Integration | Cursor sidecar, updater, provider wizards |

Deep dives stay in linked articles; this file is the single checklist.

## Core

| Path | Role | Article |
|------|------|---------|
| `cmd/solomon/` | Binary entry: CLI flags, wizard, `Runtime` bootstrap | [Startup and CLI](startup-and-cli.md) |
| `internal/server/` | HTTPS daemon: auth, Responses API, SSE, passkey | [Startup and CLI](startup-and-cli.md#solomon-serve) |
| `internal/agent/` | Root agent package: `SlashDispatch` re-export (`slash_forward.go`) | [Skills and slash](skills-and-slash.md) |
| `internal/agent/runtime/` | REPL, turns, session I/O, MCP init, Cursor hooks | [Runtime hub](runtime.md) |
| `internal/agent/runtime/btw/` | Transient `/btw` side questions and output buffering | [Agent turn pipeline](agent-turn-pipeline.md#btw-side-stream) |
| `internal/agent/runtime/btw/input/` | Terminal acquisition for `/btw` listener input | [Agent turn pipeline](agent-turn-pipeline.md#btw-side-stream) |
| `internal/agent/runtime/btw/listener/` | Streaming-time `/btw` trigger listener | [Agent turn pipeline](agent-turn-pipeline.md#btw-side-stream) |
| `internal/agent/runtime/turnloop/` | Agent turn loop: stream, tool exec, compaction, interrupt | [Agent turn pipeline](agent-turn-pipeline.md) |
| `internal/agent/runtime/repl/` | REPL loop, readline wiring | [Runtime — REPL](runtime-repl.md) |
| `internal/agent/runtime/repl/editor/` | Multiline raw-mode editor (keys, render, history, `@` picker) | [Runtime — REPL](runtime-repl.md) |
| `internal/agent/runtime/repl/replhl/` | Input syntax highlighting (`@`, shell lines) | [Runtime — REPL](runtime-repl.md) |
| `internal/agent/runtime/repl/shellhist/` | Shell command history for `!` replay | [Runtime — REPL](runtime-repl.md) |
| `internal/agent/runtime/repl/shelllex/` | Shell line lexer for highlighting/completion | [Runtime — REPL](runtime-repl.md) |
| `internal/agent/runtime/replcomplete/` | Tab completion (slash, `@`, paths) | [Runtime — REPL](runtime-repl.md) |
| `internal/agent/runtime/multiline/` | Bracket/quote-aware multiline terminal modes | [Runtime — REPL](runtime-repl.md) |
| `internal/llm/` | Streaming chat facade, usage, backend factory | [LLM layer](llm-layer.md) |
| `internal/llm/anthropic/` | Anthropic Messages API adapter | [LLM layer](llm-layer.md) |
| `internal/llm/apitype/` | Shared request/response type helpers | [LLM layer](llm-layer.md) |
| `internal/llm/images/` | Image part encoding for multimodal calls | [LLM layer](llm-layer.md) |
| `internal/llm/images/token/` | Image token encoding and `[img-N]` placeholder expansion | [LLM layer](llm-layer.md) |
| `internal/llm/promptparts/` | Prompt token split helpers for usage display | [LLM layer](llm-layer.md) |
| `internal/llm/stream/` | OpenAI completion streaming (`StreamText`, `StreamAssistantTurn`) | [LLM layer](llm-layer.md) |
| `internal/llm/streamio/` | Stream read/write utilities | [LLM layer](llm-layer.md) |
| `internal/llm/transport/` | HTTP transport, retries, stream integrity | [LLM layer](llm-layer.md) |
| `internal/tokcount/` | Tiktoken `o200k_base` prompt estimates (messages, tools, vision) | [LLM layer](llm-layer.md) |
| `internal/chatstore/` | Session JSON read/write, images on disk | [Sessions and storage](sessions-and-storage.md) |
| `internal/project/` | Project hex id, workspace root resolution | [Sessions and storage](sessions-and-storage.md) |
| `internal/paths/` | `~/.solomon` layout helpers | [Sessions and storage](sessions-and-storage.md) |
| `internal/config/` | TOML config load/merge | [Configuration](../user-guide/configuration.md) |
| `internal/prompt/` | System prompt render (`RenderAgent`, `RenderChat`) | [Plan vs build](plan-vs-build.md) |
| `internal/prompt/templates/` | Embedded `.tmpl` defaults (copied to `~/.solomon/prompts/templates/` at runtime) | [Plan vs build](plan-vs-build.md), [Configuration](../user-guide/configuration.md#prompt_templates-system-prompt-templates) |
| `internal/prompt/shell/` | OS-specific shell hints in templates | [Supporting packages](supporting-packages.md) |

## Feature

| Path | Role | Article |
|------|------|---------|
| `internal/agent/tools/` | Native OpenAI tools (plan/build), router `Exec` | [Native tools](native-tools.md) |
| `internal/agent/toolenv/` | `Env` struct — callbacks and deps passed into every tool | [Native tools](native-tools.md#toolenv) |
| `internal/agent/slash/` | Slash line parse and dispatch | [Skills and slash](skills-and-slash.md) |
| `internal/agent/commands/` | Slash command implementations | [Skills and slash](skills-and-slash.md) |
| `internal/agent/commands/connect/` | `/connect` provider wizard | [Startup and CLI](startup-and-cli.md) |
| `internal/agent/cievents/` | JSON/JSONL event schema for `exec --json` | [Runtime — orchestration](runtime-orchestration.md) |
| `internal/tooling/` | `Invocation` type, legacy `<tool_calls>` XML | [Native tools](native-tools.md) |
| `internal/sandbox/compile/` | Go source → WASM for `orchestrate` scripts | [Plan vs build](plan-vs-build.md) |
| `internal/sandbox/host/` | wazero host module, SDK RPC bridge | [Plan vs build](plan-vs-build.md) |
| `internal/sandbox/ipc/` | JSON line protocol for worker ↔ parent | [Plan vs build](plan-vs-build.md) |
| `internal/sandbox/parent/` | Spawn and drive `sandbox-worker` subprocess | [Plan vs build](plan-vs-build.md) |
| `internal/sandbox/run/` | wazero runtime, WASM module cache | [Plan vs build](plan-vs-build.md) |
| `internal/sandbox/sdk/` | Script-facing API (`ReadFile`, `Shell`, …) | [Plan vs build](plan-vs-build.md) |
| `internal/sandbox/worker/` | `solomon sandbox-worker` serve loop | [Plan vs build](plan-vs-build.md) |
| `internal/tooloutput/` | Tool result truncation and spill to `temp/` | [Supporting packages](supporting-packages.md) |
| `internal/tooloutput/process/` | Cross-process temp cleanup coordination | [Supporting packages](supporting-packages.md) |
| `internal/mcp/` | MCP client manager and OpenAI adapter | [MCP integration](mcp-integration.md) |
| `internal/skills/` | Skill registry, install, search | [Skills and slash](skills-and-slash.md) |
| `internal/checkpoint/` | Checkpoint sequences, labels, goto | [Checkpoints](checkpoints.md) |
| `internal/checkpoint/staging/` | Byte snapshots for file restore | [Checkpoints](checkpoints.md) |
| `internal/instructions/` | `AGENTS.md` / fallbacks loader | [Supporting packages](supporting-packages.md) |
| `internal/search/` | Web search backends for `webSearch` | [Supporting packages](supporting-packages.md) |
| `internal/research/` | Research engine: web jobs, parsing, quality checks, LLM integration | [Supporting packages](supporting-packages.md) |
| `internal/research/html/` | HTML rendering templates for research results | [Supporting packages](supporting-packages.md) |
| `internal/roles/` | Subagent role pool from config (`SubagentPool`, `FindSubagent`) | [Native tools](native-tools.md#subagent-roles) |
| `internal/pathglob/` | Glob `**` matching for `find` | [Native tools](native-tools.md) |
| `internal/plan/` | Plan file read, write, sections, todos, status | [Plan vs build](plan-vs-build.md) |
| `internal/gitignore/` | `.gitignore` matcher for `find` | [Native tools](native-tools.md) |
| `internal/atmention/` | `@` file/folder tags and picker scoring | [Runtime — REPL](runtime-repl.md) |
| `internal/docs/` | Embedded docs corpus, BM25 retrieval for `docsRetrieval` and `/docs` | [Supporting packages](supporting-packages.md) |

## Support

| Path | Role | Article |
|------|------|---------|
| `internal/logging/` | File logs under `~/.solomon/logs` | [Supporting packages](supporting-packages.md) |
| `internal/termcolor/` | Lipgloss palette, usage line, `NO_COLOR` | [Supporting packages](supporting-packages.md) |
| `internal/clipboard/` | Cross-platform image paste in REPL | [Supporting packages](supporting-packages.md) |
| `internal/claudecode/` | Claude Code version lookup (GitHub releases, disk cache) for OAuth headers | [LLM layer](llm-layer.md) |
| `internal/title/` | Chat title slug and LLM refinement | [Supporting packages](supporting-packages.md) |
| `internal/modelsapi/` | List models from provider API | [Supporting packages](supporting-packages.md) |
| `internal/logo/` | ASCII banner | [Supporting packages](supporting-packages.md) |
| `internal/auth/anthropic/claude/` | Claude Sub OAuth, token refresh | [LLM layer](llm-layer.md) |
| `internal/auth/openai/codex/` | ChatGPT Sub OAuth, token refresh | [LLM layer](llm-layer.md) |
| `internal/auth/openai/codex/chat/` | Codex chat request shaping | [LLM layer](llm-layer.md) |
| `internal/providersetup/` | Provider onboard during `/connect` | [Supporting packages](supporting-packages.md) |
| `internal/webfetch/` | HTTP client for fetching web content (cookie jar, user agent) | [Supporting packages](supporting-packages.md) |

## Integration

| Path | Role | Article |
|------|------|---------|
| `internal/integrations/cursor/` | Sidecar install paths, process manager, embed | [Cursor integration](cursor-integration.md) |
| `internal/integrations/cursor/agent/` | Cursor Agent SDK runtime wrapper (Go) | [Cursor integration](cursor-integration.md) |
| `internal/updater/` | GitHub release check, install, restart | [Supporting packages](supporting-packages.md) |
| `integrations/cursor/` | Node.js OpenAI-compatible proxy (not Go) | [Cursor integration](cursor-integration.md) |
| `test/` | Integration and unit tests (package `test`) | [Testing](../development/testing.md) |

## `toolenv` — tool execution context

Native tools receive an [`Env`](../../internal/agent/toolenv/env.go) built by [`Runtime.toolEnv()`](../../internal/agent/runtime/exec.go). The type lives in `internal/agent/toolenv/`; `internal/agent/tools/env.go` re-exports it as `tools.Env` so handlers stay in the `tools` package without import cycles.

| Field | Set by runtime | Used for |
|-------|----------------|----------|
| `ProjHex`, `ProjRoot` | session project | Path resolution, spill paths |
| `Cfg` | loaded TOML | Web search merge, tool limits |
| `MCP` | `InitMCP` | `dispatchExternal` MCP calls |
| `RunNested`, `RunNestedWithSystem` | turn loop | `subagent` tool |
| `SetMode`, `CurrentMode` | REPL / tools | Plan vs build guards |
| `CheckpointStageProjAbs`, `CheckpointBeforeProjAbs`, `CheckpointRecordEdit`, `CheckpointCpSeq` | checkpoint hooks | `editFile` staging for `/goto` |
| `ActivateInstructionsFromAbsPath`, `ActivateInstructionsFromShellCommand`, `MergeInstructionBlock` | instructions loader | `readFile` / `shell` activating `AGENTS.md` |

When adding a tool that needs runtime state, extend `toolenv.Env` first, wire fields in `runtime/exec.go` `toolEnv()`, then read them from the handler in `internal/agent/tools/`.

## Alphabetical quick reference

| Path | Tier |
|------|------|
| `cmd/solomon/` | Core |
| `internal/agent/` | Core |
| `internal/agent/cievents/` | Feature |
| `internal/agent/commands/` | Feature |
| `internal/agent/commands/connect/` | Feature |
| `internal/agent/runtime/` | Core |
| `internal/agent/runtime/btw/` | Core |
| `internal/agent/runtime/btw/input/` | Core |
| `internal/agent/runtime/btw/listener/` | Core |
| `internal/agent/runtime/turnloop/` | Core |
| `internal/agent/runtime/multiline/` | Core |
| `internal/agent/runtime/repl/` | Core |
| `internal/agent/runtime/repl/editor/` | Core |
| `internal/agent/runtime/repl/replhl/` | Core |
| `internal/agent/runtime/repl/shellhist/` | Core |
| `internal/agent/runtime/repl/shelllex/` | Core |
| `internal/agent/runtime/replcomplete/` | Core |
| `internal/agent/slash/` | Feature |
| `internal/agent/toolenv/` | Feature |
| `internal/agent/tools/` | Feature |
| `internal/atmention/` | Feature |
| `internal/auth/anthropic/claude/` | Support |
| `internal/auth/openai/codex/` | Support |
| `internal/auth/openai/codex/chat/` | Support |
| `internal/chatstore/` | Core |
| `internal/checkpoint/` | Feature |
| `internal/checkpoint/staging/` | Feature |
| `internal/claudecode/` | Support |
| `internal/clipboard/` | Support |
| `internal/config/` | Core |
| `internal/docs/` | Feature |
| `internal/gitignore/` | Feature |
| `internal/instructions/` | Feature |
| `internal/integrations/cursor/` | Integration |
| `internal/integrations/cursor/agent/` | Integration |
| `internal/llm/` | Core |
| `internal/llm/anthropic/` | Core |
| `internal/llm/apitype/` | Core |
| `internal/llm/images/` | Core |
| `internal/llm/images/token/` | Core |
| `internal/llm/promptparts/` | Core |
| `internal/llm/stream/` | Core |
| `internal/llm/streamio/` | Core |
| `internal/llm/transport/` | Core |
| `internal/logging/` | Support |
| `internal/logo/` | Support |
| `internal/mcp/` | Feature |
| `internal/modelsapi/` | Support |
| `internal/pathglob/` | Feature |
| `internal/plan/` | Feature |
| `internal/paths/` | Core |
| `internal/project/` | Core |
| `internal/prompt/` | Core |
| `internal/prompt/shell/` | Core |
| `internal/providersetup/` | Support |
| `internal/research/` | Feature |
| `internal/research/html/` | Feature |
| `internal/roles/` | Feature |
| `internal/search/` | Feature |
| `internal/server/` | Core |
| `internal/sandbox/compile/` | Feature |
| `internal/sandbox/host/` | Feature |
| `internal/sandbox/ipc/` | Feature |
| `internal/sandbox/parent/` | Feature |
| `internal/sandbox/run/` | Feature |
| `internal/sandbox/sdk/` | Feature |
| `internal/sandbox/worker/` | Feature |
| `internal/skills/` | Feature |
| `internal/termcolor/` | Support |
| `internal/title/` | Support |
| `internal/tokcount/` | Core |
| `internal/webfetch/` | Support |
| `internal/tooling/` | Feature |
| `internal/tooloutput/` | Feature |
| `internal/tooloutput/process/` | Feature |
| `internal/updater/` | Integration |
| `integrations/cursor/` | Integration |
| `test/` | Integration |

## See also

- [Overview](overview.md) — design tenets and dependency graph
- [Supporting packages](supporting-packages.md) — support-package entry points
- [Cookbook](../development/cookbook.md) — change recipes
