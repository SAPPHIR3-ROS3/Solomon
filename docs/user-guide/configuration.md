# Configuration

Solomon stores user settings in `~/.solomon/config.toml`. MCP servers use a separate file (see [MCP integration](../architecture/mcp-integration.md)).

## Main file

Path: `~/.solomon/config.toml`. Schema: [`config.Root`](../../internal/config/config.go).

| Field | Role |
| ----- | ---- |
| `current.provider`, `current.model` | Active backend |
| `providers.<name>` | Named provider blocks (`base_url`, `api_key`, `api_protocol`, …) |
| `recent_models.<name>` | Recent model ids per provider |
| `user_name` | Shown / used in-session |
| `subagent_timeout_minutes` | Subagent slices (wizard default 20) |
| `[api_resilience]` | LLM HTTP retry, backoff, circuit breaker, timeouts (optional; defaults in code) |
| `reasoning_effort` | Main chat reasoning profile |
| `log_level`, `max_response_tokens` | Verbosity and cap |
| `show_thinking`, `show_usage_stats` | Streams / footer |
| `[tools].legacy`, `[tools].legacy_force` | Legacy XML tool calling (global); see below |
| `[tools].cursor_internal_tools` | Cursor sidecar: allow Cursor IDE built-in tools (default **off**); see [Cursor integration](#cursor-integration-tool-execution) |
| `response_language` | Default reply language |
| `compaction_threshold_tokens` | Auto compaction threshold |
| `tool_output.max_bytes`, `tool_output.max_lines` | Tool result truncation before LLM (defaults 65536 / 2048) |
| `web_search_engine` | Default engine for the **`webSearch`** tool (omit for `duckduckgo`) |
| `fast_mode` | Cursor fast mode when the active provider supports it (default on; toggle with `/fast`) |
| `autoupdate` | Auto-install newer releases when `/update` finds one (toggle with `/autoupdate`) |

### `[tools]` (legacy XML tool calling)

Optional text-based tool protocol for models/backends without reliable native function calling. Persisted globally in `config.toml` (also toggled in-session with `/legacytools`).

| Key | Role |
|-----|------|
| `legacy` | When `true`, the model **may** use legacy `<tool_calls>` XML in assistant text (native API tools remain available unless force is on). |
| `legacy_force` | When `true`, native API tool_calls are disabled and the model **must** use legacy XML for tool invocations. Implies `legacy = true`. |

Example:

```toml
[tools]
legacy = true
legacy_force = false
```

Legacy root keys `legacy_tools` / `legacy_tools_force` are still read once for migration, then rewritten under `[tools]` on save.

| Combination | Effect |
|-------------|--------|
| both off | Native API tool_calls only |
| `legacy = true`, `force` off | Native preferred; model may also use `<tool_calls>` XML |
| `legacy_force = true` | Native API tools omitted from requests; model must use XML |

On-screen tool lines show intent on its own line, then `Tool: name …` (same for native and legacy). Syntax or JSON errors trigger an automatic retry turn with a correction message.

Wire format (assistant text):

```xml
<tool_calls>
<tool name="shell">
<intent>Run unit tests</intent>
<args>{"command":"go test ./..."}</args>
</tool>
</tool_calls>
```

Rules: one `<tool_calls>` block per reply that invokes tools; optional prose before the block; no text after `</tool_calls>`; each `<args>` must be valid JSON matching the tool schema. Unknown tool names are rejected like malformed XML.

Architecture: [Agent turn pipeline](../architecture/agent-turn-pipeline.md#legacy-xml-tool-calling), [Native tools](../architecture/native-tools.md).

### Cursor integration (tool execution)

When Solomon uses the optional **Cursor API sidecar** (`/integrations`), inference may run through Cursor’s agent SDK while **tool execution on your project must stay in Solomon** (Go `tools.Exec` on the real project root).

| Key | Default | Role |
|-----|---------|------|
| `cursor_internal_tools` | **`false`** (omit or unset) | When **false**, Cursor built-in tools (`Read`, `Shell`, `Task`, …) are **not** allowed to run against your repo; the sidecar intercepts or blocks them and returns Solomon tool calls instead. When **true**, the sidecar may let Cursor execute its native tools on the project — **avoid unless you explicitly want that**. |

Example (recommended):

```toml
[tools]
cursor_internal_tools = false
```

Sidecar env (set by Solomon when starting the proxy): `CURSOR_API_ALLOW_INTERNAL_TOOLS=true` only when `cursor_internal_tools = true` in config.

#### Fail-closed behavior (`cursor_internal_tools = false`)

With the default, several layers stack:

1. **Deny hooks** — `.solomon-cursor-guard/.cursor/hooks.json` rejects common Cursor tools (`Shell`, `Read`, `Write`, `Edit`, `Grep`, `Glob`, `Delete`, `Task`) with `permission: deny` and `failClosed: true`.
2. **Isolated workspace** — the Cursor agent’s `cwd` is `.solomon-cursor-guard/`, not your project root, so accidental native file/shell ops do not target the repo.
3. **Dummy MCP `solomon`** — exposes tool **schemas** from the current request; `tools/call` does **not** execute on disk (returns an error after a safety timeout). Solomon is the only executor.
4. **Stream proxy** — the sidecar watches Cursor stream events: mappable calls (e.g. `Read` → `readFile`, MCP `solomon` unwrap) become Solomon invocations; the Cursor run is stopped (`forceStopRun`); unmapped or disallowed tools are blocked and reported via `solomon_proxy_correction`.

Solomon then runs `readFile`, `shell`, `editFile`, etc. on the **real** project path.

#### Name mapping vs execution

The sidecar may **translate** Cursor-native names to Solomon names (`Task` → `subagent`, `Delete` → `editFile` with `delete: true`, …). That is **not** Cursor executing your project: it is recognition + handoff so the model’s habit vocabulary still reaches Solomon’s executor. Tools present in the request `tools[]` list are also accepted by **exact Solomon name** without maintaining a fixed map.

#### When `cursor_internal_tools = true`

Hooks and guard workspace are **not** used; the agent may run with your project as `cwd`. Native Cursor tools can execute there. Use only for debugging or deliberate Cursor-native workflows — not for “Solomon owns all tools” setups.

While native tools run, the sidecar streams **`solomon_cursor_tool_event`** chunks (tool name, status, args, result). Solomon prints them in the REPL as `Tool: Read (cursor) …` so you see the same activity you would in Cursor, even though Solomon does not execute those calls.

#### Limits

Hook matchers do not cover every possible Cursor tool name (e.g. some browser or alias names rely on the stream proxy). External MCP servers from Cursor are blocked (`mcp:external`). Guarantees depend on the sidecar version and Cursor SDK behavior; keep `cursor_internal_tools = false` for production Solomon sessions.

Implementation: [`integrations/cursor/`](../integrations/cursor/), [`internal/integrations/cursor/`](../internal/integrations/cursor/). Architecture detail: [Native tools — Cursor sidecar proxy](../architecture/native-tools.md#cursor-sidecar-proxy).

You can edit the file directly, use first-run or `/onboard` (OpenAI or Anthropic Compatible API), or manage providers and models in the REPL with `/connect` and `/models`.

### LLM providers

| Setup | `api_protocol` | Notes |
|-------|----------------|--------|
| `/onboard` or `/connect` → OpenAI Compatible API | `openai` (default) | Any OpenAI Chat Completions-compatible `base_url` |
| `/onboard` or `/connect` → Anthropic Compatible API | `anthropic` | Messages API (`POST …/v1/messages`); curated model list |
| `/connect` → ChatGPT Sub | `openai` | OAuth; Codex middleware |
| `/connect` → Claude Sub | — | Listed in wizard as **coming soon** (not available yet) |
| `/connect` → Cursor API | `openai` | Optional sidecar; see [Cursor integration](#cursor-integration-tool-execution) |

Provider block fields: `base_url`, `api_key`, optional `api_protocol` (`openai` | `anthropic`). Anthropic official base: `https://api.anthropic.com` (normalized on save).

### REPL slash commands and config fields

Many slash commands write back to `config.toml` on save:

| Slash command | Config field | Notes |
|---------------|--------------|-------|
| `/name` | `user_name` | `/name clear` removes |
| `/language` | `response_language` | `/language clear` resets to English |
| `/reasoning` | `reasoning_effort` | Main chat only; subagent reasoning stays off unless extended later |
| `/thinking` | `show_thinking` | Streamed reasoning preview |
| `/stats` | `show_usage_stats` | Token line after assistant turns |
| `/max_response` | `max_response_tokens` | Assistant output cap |
| `/timeout` | `subagent_timeout_minutes` | Range 1–180 |
| `/log` | `log_level` | `error`, `warning`, `info`, `debug`, `result` |
| `/threshold` | `compaction_threshold_tokens` | Auto `/summarize` when prompt tokens exceed limit |
| `/legacytools` | `[tools].legacy`, `[tools].legacy_force` | Global, not per session |
| `/fast` | `fast_mode` | Only when active provider supports Cursor fast mode |
| `/autoupdate` | `autoupdate` | Auto-install on `/update` check |

Commands such as `/configbackup`, `/update`, `/upgrade`, and `/version` do not add new config keys; `/onboard` overwrites wizard-managed provider fields.

### `[api_resilience]` (optional)

Overrides defaults from [`EffectiveAPIResilience`](../../internal/config/api_resilience.go). Omitted keys keep built-in defaults.

| Key | Default | Role |
|-----|---------|------|
| `max_retries` | `3` | Maximum stream/turn attempts per provider host (clamped to 10) |
| `base_delay_ms` | `1000` | Initial backoff before retry |
| `max_delay_ms` | `30000` | Cap on wait between retries |
| `jitter` | `true` | Randomize delay up to half the computed wait |
| `connect_timeout_sec` | `30` | TCP connect and response-header timeout |
| `read_timeout_sec` | `0` (off) | Whole-request timeout for non-stream calls only |
| `circuit_open_sec` | `60` | After exhausting retries, block the host for this duration |

Example:

```toml
[api_resilience]
max_retries = 3
base_delay_ms = 1000
circuit_open_sec = 60
```

## Logs

Directory: `~/.solomon/logs`. Seven-day retention; file-only logging by default ([`cmd/solomon/main.go`](../../cmd/solomon/main.go)).

## Web search (`webSearch`)

The **`webSearch`** tool uses **`web_search_engine`** from `config.toml`. If empty or omitted, **`duckduckgo`** is used. Per-call **`engine`** and **`extras`** override merged config ([`MergeWebSearchExtras`](../../internal/agent/tools/web_search.go)).

| `web_search_engine` | Required `config.toml` | Notes |
|--------------------|-------------------------|--------|
| **`duckduckgo`** (default) | None | HTML results; no API key. |
| **`searxng`** | **`web_search_base_url`** | Your SearxNG instance only; no public pool. Per-call **`extras.baseURL`** overrides. |
| **`googlepse`** | **`web_search_api_key`** + **`web_search_cx`** | [Programmable Search Engine](https://developers.google.com/custom-search/v1/overview). **`maxResults`** capped at **10**. |
| **`brave`** | **`web_search_api_key`** | Brave subscription token. Optional **`extras.apiKey`** per call. |
| **`bing`** | **`web_search_api_key`** | Bing Web Search. Optional **`extras.endpoint`** on the tool call only (default in [`bing.go`](../../internal/search/bing.go)); no TOML mapping for endpoint. |

Example snippets:

```toml
web_search_engine = "duckduckgo"

web_search_engine = "searxng"
web_search_base_url = "https://search.example.net"

web_search_engine = "googlepse"
web_search_api_key = "YOUR_API_KEY"
web_search_cx = "YOUR_SEARCH_ENGINE_ID"

web_search_engine = "brave"
web_search_api_key = "YOUR_BRAVE_SUBSCRIPTION_TOKEN"

web_search_engine = "bing"
web_search_api_key = "YOUR_SUBSCRIPTION_KEY"
```

## MCP configuration file

Path: `~/.solomon/mcp.json`, or the file in `SOLOMON_MCP_CONFIG`. If missing, Solomon starts without MCP servers.

Full schema, JSON example, and runtime behavior: [MCP integration](../architecture/mcp-integration.md).

## See also

- [Usage and commands](usage-and-commands.md)
- [Data layout](data-layout.md)
- [Startup and CLI](../architecture/startup-and-cli.md) — first-run wizard
