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
| `response_language` | Default reply language |
| `compaction_threshold_tokens` | Auto compaction threshold |
| `tool_output.max_bytes`, `tool_output.max_lines` | Tool result truncation before LLM (defaults 65536 / 2048) |
| `web_search_engine` | Default engine for the **`webSearch`** tool (omit for `duckduckgo`) |

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

You can edit the file directly, use first-run or `/onboard` (OpenAI or Anthropic Compatible API), or manage providers and models in the REPL with `/connect` and `/models`.

### LLM providers

| Setup | `api_protocol` | Notes |
|-------|----------------|--------|
| `/onboard` or `/connect` → OpenAI Compatible API | `openai` (default) | Any OpenAI Chat Completions-compatible `base_url` |
| `/onboard` or `/connect` → Anthropic Compatible API | `anthropic` | Messages API (`POST …/v1/messages`); curated model list |
| `/connect` → ChatGPT Sub | `openai` | OAuth; Codex middleware |

Provider block fields: `base_url`, `api_key`, optional `api_protocol` (`openai` | `anthropic`). Anthropic official base: `https://api.anthropic.com` (normalized on save).

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
