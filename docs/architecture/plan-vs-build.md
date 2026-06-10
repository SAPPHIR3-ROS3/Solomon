# Session modes: agent, chat, plan, build

## Purpose

`Runtime.Mode` switches the native tool surface and the system prompt template.

## Current modes

| Mode | Set by | Native tools (OpenAI) |
|------|--------|------------------------|
| **`agent`** | `/agent`, default (`NewRuntime`) | `searchTools`, `orchestrate`, `switchMode` |
| **`chat`** | `/chat` | `docsRetrieval`, `fetchWeb`, `webSearch`, `switchMode` |
| `plan` | `/plan` (deprecated → agent) | Legacy: `createPlan`, `editPlan`, `buildPlan`, `docsRetrieval` |
| `build` | `/build` (deprecated → agent) | Legacy: `shell`, `readFile`, `editFile`, `find`, `subagent`, skills, web, `docsRetrieval` |

MCP tools append in **agent** mode when connected ([`toolParams`](../../internal/agent/runtime/mcp.go)).

## Code mode (orchestrate)

Deferred tools (shell, readFile, editFile, plan tools, …) are **not** exposed directly in agent mode. The model uses **`searchTools`** to discover them and **`orchestrate`** to run Go scripts that call the sandbox SDK (`internal/sandbox/sdk`). Scripts compile to WASM (`GOOS=wasip1`) and execute in a long-lived **`solomon sandbox-worker`** subprocess.

Internal tool calls from orchestrate scripts use the same checkpoint **`cp_seq`** as the parent `orchestrate` invocation (rollback via `/goto` restores all script edits atomically).

## Packages and files

| File | Role |
|------|------|
| `internal/agent/tools/params.go` | `NativeToolParams(mode)` |
| `internal/agent/tools/exec.go` | Mode guards; `AllowDeferredTools` for orchestrate host |
| `internal/agent/tools/orchestrate.go` | Compile + worker IPC |
| `internal/agent/tools/search_tools.go` | Deferred catalog search |
| `internal/agent/tools/switch_mode.go` | Mode switch tool |
| `internal/agent/runtime/core.go` | `systemPrompt` → `RenderAgent` / `RenderChat` / legacy |
| `internal/agent/runtime/switch_mode.go` | Countdown UX (5s, Ctrl+C cancel) |
| `internal/sandbox/` | SDK, compile, worker, wazero host |
| `internal/prompt/templates/agent.tmpl`, `chat.tmpl` | System prompts |

## Legacy plan / build

`/plan` and `/build` print a migration message and switch to **agent**. Existing sessions or tests may still use `plan` / `build` mode strings; `NativeToolParams` keeps legacy tool surfaces for compatibility.

## Key functions

| Function | Behavior |
|----------|----------|
| `NativeToolParams` | Returns tool schema slice for mode |
| `BuildAgentToolDump` / `BuildChatToolDump` | Prompt tool listings |
| `tools.Exec` | Rejects tools outside mode unless `AllowDeferredTools` |
| `switchMode` | Countdown then `Runtime.Mode` + next-turn system prompt |

## See also

- [Native tools](native-tools.md)
- [Checkpoints](checkpoints.md)
- [Agent turn pipeline](agent-turn-pipeline.md)
