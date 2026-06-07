# Cursor integration

## Purpose

Optional **Cursor API** provider: Solomon talks to a local **Node sidecar** (OpenAI-compatible HTTP), the sidecar drives the **Cursor Agent SDK**, and — by default — **Solomon Go** executes all tools on the real project root.

User setup (TOML, `/connect`, `/integrations`, `/cursortools`): [Configuration — Cursor integration](../user-guide/configuration.md#cursor-integration-tool-execution).

## Mental model

```
Solomon Runtime  --OpenAI HTTP-->  sidecar (:8766/v1/)  --Cursor SDK-->  remote model
       |                                    |
       +-------- tools.Exec (Go) -----------+  (bridge: default mode)
```

- **Sidecar** = thin proxy + stream bridge (`integrations/cursor/`).
- **Go integration** = install bundle, start process, health, `/integrations` (`internal/integrations/cursor/`).
- **Executor** = always Solomon `tools.Exec` on `ProjRoot` when `cursor_internal_tools = false` (default).

## End-to-end flow

```mermaid
sequenceDiagram
  participant RT as Runtime turns
  participant LLM as OpenAI client
  participant Side as Node sidecar
  participant SDK as Cursor Agent SDK
  participant Go as tools.Exec

  RT->>LLM: StreamTurn base_url=127.0.0.1:8766/v1
  LLM->>Side: POST /v1/chat/completions + tools[]
  Side->>SDK: agent run on project cwd
  SDK-->>Side: stream deltas + Cursor native tool events
  alt cursor_internal_tools=false
    Side->>Side: map Read/StrReplace/Grep to Solomon names
    Side->>Side: forceStopRun
    Side-->>LLM: SSE OpenAI tool_calls Solomon names
    LLM-->>RT: native tool_calls
    RT->>Go: execTool on ProjRoot
  else cursor_internal_tools=true
    Side-->>LLM: solomon_cursor_tool_event chunks
    LLM-->>RT: display via cursor_native_display
    Note over SDK,Go: Cursor executes native tools on project cwd
  end
```

Sidecar startup: [`manager.go`](../../internal/integrations/cursor/manager.go) spawns `node dist/index.js` with env from config. Runtime ensures sidecar via [`cursor_sidecar.go`](../../internal/agent/runtime/cursor_sidecar.go) → [`agent/runtime.go`](../../internal/integrations/cursor/agent/runtime.go).

## Operating modes

| `[tools].cursor_internal_tools` | Agent `cwd` | SDK sandbox | Who runs file/shell tools on repo |
|--------------------------------|-------------|-------------|-----------------------------------|
| **`false`** (default, omit) | project root | yes (fallback off if SDK rejects) | **Solomon Go** only |
| **`true`** | project root | no | **Cursor SDK** (native tools) |

Recommended production default: **`false`**. Solomon sets sidecar env `CURSOR_API_ALLOW_INTERNAL_TOOLS=true` only when config is `true`.

Toggle in-session: `/cursortools on|off` (listed in `/help` only when Cursor API is configured). Implementation: [`thinking.go`](../../internal/agent/commands/thinking.go) (`CursorTools`). Inspect status: `/integrations` ([`integrations_slash.go`](../../internal/agent/commands/integrations_slash.go)).

## Transparent bridge (`cursor_internal_tools = false`)

The model uses **normal Cursor built-in tools** (Read, StrReplace, Write, Grep, Glob, Shell, Delete, SemanticSearch, Task, etc.). It does not call Solomon tool names directly.

Layers (see [`cursor-agent.ts`](../../integrations/cursor/src/cursor-agent.ts), [`chat.ts`](../../integrations/cursor/src/chat.ts)):

1. **Native SDK surface** — `Agent.create` with `local.cwd = project root` and Cursor's default tool catalog (no MCP `solomon` stub).
2. **SDK sandbox** — `sandboxOptions.enabled` blocks Cursor from writing to disk when the SDK supports it.
3. **Stream bridge** — map Cursor tool events → Solomon names ([`legacy.ts`](../../integrations/cursor/src/legacy.ts)) → OpenAI `tool_calls` SSE; stop Cursor run (`forceStopRun`).
4. **Reject unmappable tools** — `solomon_proxy_correction` on the SSE stream; suggests **Shell** as default fallback when `shell` is in the host allowlist.

Solomon then runs `readFile`, `shell`, `editFile`, etc. on **`ProjRoot`**. Tool results return on the next turn via the normal OpenAI message history loop.

Harness prompts ([`harness-clauses.txt`](../../integrations/cursor/prompts/harness-clauses.txt), [`harness-tools-clause.txt`](../../integrations/cursor/prompts/harness-tools-clause.txt)) steer the model to use Cursor built-in tools and explain host interception.

## HTTP API (sidecar)

Base URL: `http://127.0.0.1:8766/v1/` (port from [`DefaultPort`](../../internal/integrations/cursor/paths.go), overridable via env).

| Method | Path | Role |
|--------|------|------|
| `GET` | `/health`, `/v1/health` | Liveness (`{ ok: true }`) |
| `GET` | `/v1/models`, `/models` | Model list for picker |
| `GET` | `/v1/models?all=1` | Full model list |
| `POST` | `/v1/chat/completions`, `/chat/completions` | Chat completion proxy |

Implementation: [`server.ts`](../../integrations/cursor/src/server.ts). Request limits: body 8 MiB, 256 messages, 64 tools (see server constants).

### Sidecar environment

Set by Go when starting the process ([`manager.go`](../../internal/integrations/cursor/manager.go)):

| Variable | Role |
|----------|------|
| `CURSOR_API_KEY` | Cursor API key from provider config |
| `CURSOR_API_PORT` | Listen port (default `8766`) |
| `CURSOR_API_CWD` | Project root |
| `CURSOR_API_ALLOW_INTERNAL_TOOLS` | `"true"` only when `cursor_internal_tools = true` |

Optional: `SOLOMON_NODE` — path to `node` binary; `SOLOMON_CURSOR_API_ROOT` — override install dir ([`paths.go`](../../internal/integrations/cursor/paths.go)).

Logs: `~/.solomon/logs/cursor-sidecar.log`.

## Install and lifecycle

| Step | Where |
|------|-------|
| Embed + extract bundle | [`bootstrap.go`](../../internal/integrations/cursor/bootstrap.go), [`embed.go`](../../internal/integrations/cursor/embed.go) |
| Install dir | `~/.solomon/integrations/cursor/` (`dist/index.js`, `node_modules/@cursor/sdk`) |
| First use / missing SDK | `Bootstrap` runs `npm` prod deps |
| Process manager | [`manager.go`](../../internal/integrations/cursor/manager.go) — singleton, health poll, restart on key change |
| Build from source | `npm --prefix integrations/cursor run build` then `go run scripts/cursor_bundler.go bundle` (CI / `make build`) |

Entry: [`integrations/cursor/src/index.ts`](../../integrations/cursor/src/index.ts).

## Go package map

| File | Role |
|------|------|
| [`paths.go`](../../internal/integrations/cursor/paths.go) | Install dir, default base URL, entry script path |
| [`bootstrap.go`](../../internal/integrations/cursor/bootstrap.go) | Extract embedded bundle, npm deps |
| [`manager.go`](../../internal/integrations/cursor/manager.go) | Start/stop sidecar, health, `ProxyStatus` |
| [`sidecar_async.go`](../../internal/integrations/cursor/sidecar_async.go) | Async kick/wait when Cursor provider active |
| [`ensure_configured.go`](../../internal/integrations/cursor/ensure_configured.go) | Wait for sidecar if configured |
| [`agent/runtime.go`](../../internal/integrations/cursor/agent/runtime.go) | `EnsureSidecar` from runtime |
| [`models.go`](../../internal/integrations/cursor/models.go) | Model list via sidecar HTTP |

Runtime display when native tools enabled: [`cursor_native_display.go`](../../internal/agent/runtime/cursor_native_display.go).

## Node package map

| File | Role |
|------|------|
| [`server.ts`](../../integrations/cursor/src/server.ts) | HTTP router, request validation |
| [`chat.ts`](../../integrations/cursor/src/chat.ts) | Chat completions handler, stream loop |
| [`chat-helpers.ts`](../../integrations/cursor/src/chat-helpers.ts) | Stream events, proxy correction text |
| [`cursor-agent.ts`](../../integrations/cursor/src/cursor-agent.ts) | SDK agent create (native surface + sandbox) |
| [`legacy.ts`](../../integrations/cursor/src/legacy.ts) | Cursor name → Solomon name bridge |
| [`legacy-normalize.ts`](../../integrations/cursor/src/legacy-normalize.ts) | Argument normalization per tool |
| [`openai-tools.ts`](../../integrations/cursor/src/openai-tools.ts) | OpenAI `tool_calls` SSE encoding |
| [`openai-sse.ts`](../../integrations/cursor/src/openai-sse.ts) | SSE chunks, `solomon_proxy_correction` |
| [`cursor-native-tools.ts`](../../integrations/cursor/src/cursor-native-tools.ts) | `solomon_cursor_tool_event` chunks |
| [`harness-prompt.ts`](../../integrations/cursor/src/harness-prompt.ts) | Prompt clauses for transparent bridge |
| [`run-control.ts`](../../integrations/cursor/src/run-control.ts) | Abort, usage, `forceStopRun` |

## Tool name bridge

Canonical map in [`legacy.ts`](../../integrations/cursor/src/legacy.ts) (`CURSOR_NATIVE_ALIASES`). Summary:

| Cursor / alias | Solomon tool | Notes |
|----------------|--------------|-------|
| `Read`, `read`, `ReadFile` | `readFile` | |
| `Shell`, `bash`, `run_terminal_cmd` | `shell` | |
| `Edit`, `Write`, `StrReplace`, `str_replace`, `ApplyPatch`* | `editFile` | patch args normalized |
| `Delete`, `delete` | `editFile` | `delete: true` |
| `Grep`, `Glob`, `SemanticSearch`, `ListDir`, `rg` | `find` | semantic = regexp fallback today |
| `Task`, `task` | `subagent` | |
| `WebFetch`, `fetch` | `fetchWeb` | |
| `WebSearch`, `web_search` | `webSearch` | |
| MCP provider `solomon` | unwrap `toolName` | legacy; bridged if still emitted |
| Exact name in request `tools[]` | pass-through | dynamic allowlist |

\*Unified-diff `ApplyPatch` is rejected (unmappable); full-file patch content maps to create/overwrite.

Bridging is **recognition + handoff**, not Cursor executing Solomon tools. Unmapped or disallowed tools → `solomon_proxy_correction` on the SSE stream ([`openai-sse.ts`](../../integrations/cursor/src/openai-sse.ts)), with Shell as default fallback when allowed.

## SSE extensions

| Field | When | Consumer |
|-------|------|----------|
| `solomon_proxy_correction` | Blocked/unmapped Cursor tool | Model retry guidance |
| `solomon_cursor_tool_event` | `cursor_internal_tools = true` | [`cursor_native_display.go`](../../internal/agent/runtime/cursor_native_display.go) — REPL `Tool: Read (cursor) …` |

Native event shape: [`cursor-native-tools.ts`](../../integrations/cursor/src/cursor-native-tools.ts) (`name`, `status`, `args`, `result`, `error`).

## Limits and caveats

- Stream proxy is the primary execution gate; SDK sandbox is a secondary layer when enabled.
- External MCP from Cursor blocked (`mcp:external`).
- Guarantees depend on sidecar + Cursor SDK versions — keep **`cursor_internal_tools = false`** for production.
- Requires **Node.js** when Cursor provider is enabled (install script does not require Node otherwise).
- Manual check after upgrades: Composer with default bridge should use `StrReplace`/`Grep` and complete edits without `/cursortools on`.

## Debug playbook

| Symptom | Start here | Tests |
|---------|------------|-------|
| Sidecar won't start | [`manager.go`](../../internal/integrations/cursor/manager.go), [`bootstrap.go`](../../internal/integrations/cursor/bootstrap.go), `~/.solomon/logs/cursor-sidecar.log` | [`test/cursor_paths_test.go`](../../test/cursor_paths_test.go) |
| `/integrations` health fail | Port 8766, `CURSOR_API_KEY`, firewall | — |
| Cursor tool ran on repo (bridge bug) | [`cursor-agent.ts`](../../integrations/cursor/src/cursor-agent.ts) sandbox, `[tools].cursor_internal_tools` | — |
| Wrong bridged tool name/args | [`legacy.ts`](../../integrations/cursor/src/legacy.ts), [`legacy-normalize.ts`](../../integrations/cursor/src/legacy-normalize.ts) | [`integrations/cursor/test/openai-tools.test.ts`](../../integrations/cursor/test/openai-tools.test.ts) |
| No REPL display for native tools | [`cursor_native_display.go`](../../internal/agent/runtime/cursor_native_display.go) | [`test/cursor_native_display_test.go`](../../test/cursor_native_display_test.go) |
| Model avoids edit/search tools | [`harness-prompt.ts`](../../integrations/cursor/src/harness-prompt.ts), proxy correction text | — |
| Provider config / base URL | [`config`](../../internal/config/config.go), `/connect` Cursor flow | [`test/config_provider_cursor_test.go`](../../test/config_provider_cursor_test.go) |

## See also

- [Configuration — Cursor integration](../user-guide/configuration.md#cursor-integration-tool-execution)
- [Native tools](native-tools.md)
- [Runtime — orchestration](runtime-orchestration.md#cursor-integration-runtime-hooks)
- [MCP integration](mcp-integration.md)
- [`integrations/cursor/`](../../integrations/cursor/)
