# Session modes: agent and chat

## Purpose

`Runtime.Mode` is either **`agent`** (default) or **`chat`**. It switches the native tool surface and the system prompt template.

## Modes

| Mode | Set by | Native tools (OpenAI) |
|------|--------|------------------------|
| **`agent`** | `/agent`, default (`NewRuntime`) | `searchTools`, `orchestrate`, `subagent`, `listSubAgents`, skills, research, `switchMode`, `docsRetrieval` |
| **`chat`** | `/chat` | `fetchWeb`, `webSearch`, research, `switchMode`, `docsRetrieval` |

Connected MCP tools are deferred in agent mode. Use `searchTools` for their schemas, then invoke them from `orchestrate` as `sdk.mcp.<tool>(intent, args)`. Resources and prompts remain host-managed through the MCP manager API. See [`toolParams`](../../internal/agent/runtime/mcp.go), [`modeAllowed`](../../internal/agent/tools/exec.go).

Deep research follows the same distinction: chat receives `deepResearch` and
`researchStatus` as native tools, while agent mode discovers their deferred
forms and runs them through `orchestrate`. The job itself is shared and
asynchronous across both modes; see [Deep research](research.md).

**Planning** is not a separate mode: `Session.PlanningActive` (set when a plan is created via plan tools) appends native plan tools until cleared.

## Deferred tools (orchestrate)

Filesystem, shell, and most plan tools are **deferred** in agent mode. The model uses **`searchTools`** to discover them and **`orchestrate`** to run Go scripts that call the sandbox SDK (`internal/sandbox/sdk`). **`subagent` is excluded** from the deferred catalog and cannot run inside orchestrate scripts; invoke it as a **native** tool_call.

`AllowDeferredTools` on the tool env (set by orchestrate host) allows deferred handlers without exposing them in the API tool list.

Every invocation entering `tools.Exec` emits lifecycle records through Solomon's internal logging system. Calls made by a native tool, including each SDK call inside `orchestrate`, carry the parent tool and parent call ID, intent, deferred/native scope, duration, and outcome; raw argument payloads are intentionally not logged.

## Legacy XML tools

When `[tools].legacy` is enabled, deferred tool names are also allowed via `<tool_calls>` XML. With `legacy_force`, the deferred tool dump is appended to the agent system prompt and native API tool schemas are omitted.

## Packages and files

| File | Role |
|------|------|
| `internal/agent/tools/params.go` | `NativeToolParams(mode)` |
| `internal/agent/tools/exec.go` | Mode guards; `AllowDeferredTools` for orchestrate host |
| `internal/agent/tools/orchestrate.go` | Compile + worker IPC |
| `internal/agent/tools/search_tools.go` | Deferred catalog search |
| `internal/agent/tools/switch_mode.go` | Mode switch tool |
| `internal/agent/runtime/core.go` | `systemPrompt` → `RenderAgent` / `RenderChat` |
| `internal/agent/runtime/mcp.go` | Plan tools when `PlanningActive` |
| `internal/sandbox/` | SDK, compile, worker, wazero host |
| `internal/prompt/templates/agent.tmpl`, `chat.tmpl` | System prompts |

## See also

- [Native tools](native-tools.md)
- [Checkpoints](checkpoints.md)
- [Agent turn pipeline](agent-turn-pipeline.md)
