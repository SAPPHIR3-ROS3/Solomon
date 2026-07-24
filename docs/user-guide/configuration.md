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
| `subagent_reasoning_effort` | Default reasoning profile for nested runs; tool calls may override it |
| `log_level`, `max_response_tokens` | Verbosity and cap |
| `show_thinking`, `show_usage_stats` | Streams / footer |
| `[tools].legacy`, `[tools].legacy_force` | Legacy XML tool calling (global); see below |
| `[tools].cursor_internal_tools` | **Deprecated** — always normalized to `false`; `/cursortools off` confirms; see [Cursor integration](#cursor-integration-tool-execution) |
| `response_language` | Default reply language |
| `compaction_threshold_tokens` | Auto compaction threshold |
| `tool_output.max_bytes`, `tool_output.max_lines` | Tool result truncation before LLM (defaults 65536 / 2048) |
| `web_search_engine` | Default engine for the **`webSearch`** tool (omit for `duckduckgo`) |
| `fast_mode` | Cursor fast mode when the active provider supports it (default on; toggle with `/fast`) |
| `autoupdate` | At REPL startup, auto-install a newer release when the GitHub check finds one, then restart in the same terminal (toggle with `/autoupdate`) |
| `doc_search_min_normalized_score` | BM25 minimum for `/docs` and `docsRetrieval` (default `0.05`) |
| `doc_search_full_article_score` | Normalized score threshold for returning a full article on short queries (default `0.9`) |
| `[export].path` | Optional absolute directory for `/export` markdown files (default root `~/.solomon/exported/`) |
| `[[roles.subagent]]` | Optional economical model pool for nested subagents (see below) |
| `[prompt_templates]` | SHA256 per edited system prompt template (see below) |

### `[prompt_templates]` (system prompt templates)

Editable copies of the embedded system prompts live under `~/.solomon/prompts/templates/` (`agent.tmpl`, `chat.tmpl`, `title.tmpl`, `summarize.tmpl`, `summarize_system.tmpl`, `images.tmpl`, `atmention.tmpl`). Solomon copies missing files from the binary on first run. Legacy `plan.tmpl` and `build.tmpl` are removed automatically on startup.

| Situation | SHA compared against |
|-----------|----------------------|
| First edit (no entry in config) | SHA256 of the embedded default in the Solomon binary |
| After you accept a prior edit | SHA256 stored under `[prompt_templates]` in this file |

On **interactive** REPL startup, if a file on disk differs from its `[prompt_templates]` SHA (tampering after a prior accept), Solomon prints `&lt;name&gt; template has been modified` and prompts as above. Drift from a newer binary is handled during `make install` (`solomon templates install`) before files are written.

**Non-interactive** use (pipes, scripts, CI — stdin not a TTY): Solomon exits with an error listing modified templates and the absolute paths to `config.toml` and `prompts/templates/`. Fix by running `solomon` in a terminal to review changes, or update `[prompt_templates]` SHAs to match the files on disk.

Example after accepting a custom `agent.tmpl`:

```toml
[prompt_templates]
agent = "a1b2c3…"
```

Architecture: [Startup and CLI](../architecture/startup-and-cli.md), [Plan vs build](../architecture/plan-vs-build.md). Data layout: [data-layout.md](data-layout.md).

### `[export]` (chat markdown exports)

Optional override for where `/export` writes markdown transcripts. When omitted, Solomon uses `~/.solomon/exported/`.

| Key | Role |
|-----|------|
| `path` | Absolute directory; files land at `{path}/{YYYY-MM-DD}/{basename}.md` |

Example:

```toml
[export]
path = "/Users/me/solomon-exports"
```

`/export` reads this section but does not modify it. Behaviour (basename slug, duplicate suffixes, `/export last` guard, image appendix): [Usage and commands — `/export`](usage-and-commands.md#export-chat-transcript).

### Subagent roles

Optional pool of provider/model pairs for economical nested subagents (`[[roles.subagent]]` in TOML). The primary agent lists entries with the native `listSubAgents` tool, compares `description` and manually assigned `scores`, then passes the chosen `provider` and `model` to `subagent` via `roleProvider` and `roleModel`. Automatic benchmark score refresh and auto-fill are temporarily disabled. When both role fields are omitted, the subagent uses the **session** provider and model.

Configure one to five score characteristics in `[roles.table]`, then assign each score directly under `[roles.subagent.scores]`. Scores must be integers from `0` to `100`; a missing selected score is shown as unclassified and is never filled automatically. `/add subagent` guides both table setup and score entry.

Entries are validated on config load and save: each row requires non-empty `provider` and `model`, a configured `[providers.<name>]`, a live model list from that provider (reachable API; model id must appear in the list), valid manual scores, and a unique provider+model pair. **Requires network and valid provider credentials** when any `[[roles.subagent]]` row is present (including headless `config.Load` / `Save`).

| Key | Role |
|-----|------|
| `provider` | Named provider from `[providers.<name>]` |
| `model` | Model id on that provider |
| `description` | Short hint for the primary agent when choosing from the pool |
| `scores` | User-assigned `0`–`100` values keyed by a characteristic selected in `[roles.table]` |

Example:

```toml
[roles.table]
characteristics = ["reasoning", "cost", "speed"]

[[roles.subagent]]
provider = "openrouter"
model = "qwen-..."
description = "Fast codebase exploration"

[roles.subagent.scores]
reasoning = 75
cost = 90
speed = 85

[[roles.subagent]]
provider = "groq"
model = "llama-..."
description = "Cheap single-file tasks"

[roles.subagent.scores]
reasoning = 65
cost = 95
speed = 92
```

Agent tools: [Native tools — subagent roles](../architecture/native-tools.md#subagent-roles). Feature overview: [Subagent delegation](../features.md#subagent-delegation).

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

When the active provider is **Cursor API**, Solomon starts a local OpenAI-compatible **sidecar** and keeps **all tool execution in Go** on your project root.

| Key | Effective value | Role |
|-----|-----------------|------|
| `cursor_internal_tools` | **`false` always** | Deprecated. Cursor built-in tools (Read, Shell, …) are blocked; Composer uses registered Solomon tools (`orchestrate`, `searchTools`, `searchSkill`, `loadSkill`, …). Config load/save forces `false`; `/cursortools on` is rejected. |

Example (recommended — omit the key or set explicitly):

```toml
[tools]
cursor_internal_tools = false
```

**Setup:** `/connect` → Cursor API (requires Node.js). **Status:** `/integrations` (URL, health, install path). **Fast mode:** `/fast` when the provider supports it. **Confirm native-tools policy:** `/cursortools off` (listed in `/help` only after Cursor API is configured).

Sidecar env (set by Solomon at start): `CURSOR_API_PROXY_OBS=1` (structured proxy JSON in `~/.solomon/logs/cursor-sidecar.log`). `CURSOR_API_ALLOW_INTERNAL_TOOLS` is never set.

Architecture (fail-closed layers, HTTP API, tool bridge, debug): **[Cursor integration](../architecture/cursor-integration.md)**.

Implementation paths: [`integrations/cursor/`](../../integrations/cursor/), [`internal/integrations/cursor/`](../../internal/integrations/cursor/).

You can edit the file directly, use first-run or `/onboard` (OpenAI or Anthropic Compatible API), or manage providers and models in the REPL with `/connect` and `/models`.

### LLM providers

| Setup | `api_protocol` | Notes |
|-------|----------------|--------|
| `/onboard` or `/connect` → OpenAI Compatible API | `openai` (default) | Any OpenAI Chat Completions-compatible `base_url` |
| `/onboard` or `/connect` → Anthropic Compatible API | `anthropic` | Messages API (`POST …/v1/messages`); models loaded from the provider API |
| `/connect` → ChatGPT Sub | `openai` | OAuth; Codex middleware |
| `/connect` → Claude Sub | `anthropic` | OAuth; native Messages API |
| `/connect` → Cursor API | `openai` | Optional sidecar; see [Cursor integration](#cursor-integration-tool-execution) |

Provider block fields: `base_url`, `api_key`, optional `api_protocol` (`openai` | `anthropic`). Anthropic official base: `https://api.anthropic.com` (normalized on save).

### REPL slash commands and config fields

Many slash commands write back to `config.toml` on save:

| Slash command | Config field | Notes |
|---------------|--------------|-------|
| `/name` | `user_name` | `/name clear` removes |
| `/language` | `response_language` | `/language clear` resets to English; custom rules and instruction files may stay in another language — see [Project instructions](project-instructions.md) |
| `/reasoning` | `reasoning_effort` | Main chat default; nested runs can override with `reasoningEffort` or `subagent_reasoning_effort` |
| `/thinking` | `show_thinking` | Streamed reasoning preview |
| `/stats` | `show_usage_stats` | Token line after assistant turns |
| `/max_response` | `max_response_tokens` | Assistant output cap |
| `/timeout` | `subagent_timeout_minutes` | Range 1–180 |
| `/log` | `log_level` | `error`, `warning`, `info`, `debug`, `result` |
| `/threshold` | `compaction_threshold_tokens` | Auto `/summarize` when prompt tokens exceed limit |
| `/legacytools` | `[tools].legacy`, `[tools].legacy_force` | Global, not per session |
| `/cursortools` | Deprecated Cursor native tools flag — always off; `/cursortools off` confirms (Cursor API configured) |
| `/fast` | `fast_mode` | Only when active provider supports Cursor fast mode |
| `/autoupdate` | `autoupdate` | Auto-install at REPL startup when a newer release is available |

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

In **agent** mode, MCP tools are discovered via `searchTools` and invoked through **`orchestrate`** (code mode), not as direct native tool_calls.

Full schema, JSON example, and runtime behavior: [MCP integration](../architecture/mcp-integration.md).

## See also

- [Usage and commands](usage-and-commands.md)
- [Data layout](data-layout.md)
- [Startup and CLI](../architecture/startup-and-cli.md) — first-run wizard
