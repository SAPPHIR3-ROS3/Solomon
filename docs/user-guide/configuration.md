# Configuration

Solomon stores user settings in `~/.solomon/config.toml`. MCP servers use a separate file (see [MCP integration](../architecture/mcp-integration.md)).

## Main file

Path: `~/.solomon/config.toml`. Schema: [`config.Root`](../../internal/config/config.go).

| Field | Role |
| ----- | ---- |
| `current.provider`, `current.model` | Active backend |
| `providers.<name>` | Named provider blocks (`base_url`, `api_key`, …) |
| `recent_models.<name>` | Recent model ids per provider |
| `user_name` | Shown / used in-session |
| `subagent_timeout_minutes` | Subagent slices (wizard default 20) |
| `reasoning_effort` | Main chat reasoning profile |
| `log_level`, `max_response_tokens` | Verbosity and cap |
| `show_thinking`, `show_usage_stats` | Streams / footer |
| `response_language` | Default reply language |
| `compaction_threshold_tokens` | Auto compaction threshold |
| `web_search_engine` | Default engine for the **`webSearch`** tool (omit for `duckduckgo`) |

You can edit the file directly or manage providers and models in the REPL with `/connect` and `/models`.

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
