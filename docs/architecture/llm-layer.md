# LLM layer

## Purpose

Translate `chatstore` messages into provider API requests, stream assistant output to the terminal, collect usage, handle reasoning streams, and enforce stream integrity where applicable.

## Provider backends

| Protocol | Implementation | API surface |
|----------|----------------|-------------|
| `openai` (default) | `OpenAIBackend` | OpenAI Chat Completions (`openai-go`) |
| `anthropic` | `AnthropicBackend` | Anthropic Messages API (HTTP + SSE) |

Runtime holds `CompletionBackend` (`NewCompletionBackend` in [`internal/llm/factory.go`](../../internal/llm/factory.go)). OpenAI-compatible and ChatGPT Sub providers use `openai`; Anthropic Compatible API providers use `anthropic`.

## Packages and files

| Package / file | Responsibility |
|----------------|----------------|
| `internal/llm/backend.go` | `CompletionBackend`, `TurnRequest`, `ToolDef` |
| `internal/llm/openai_backend.go` | OpenAI adapter |
| `internal/llm/anthropic_*.go` | Anthropic mapper, stream, usage |
| `internal/llm/stream.go` | Shared types, OpenAI stream helpers |
| `internal/llm/params.go` | `MessageParams`, images `[img-N]`, token display |
| `internal/llm/reasoning.go` | `MessagesForAPI` (reasoning only on last assistant) |
| `internal/llm/httpresilience.go` | Error classification, backoff, circuit breaker, HTTP client |
| `internal/llm/resilient_backend.go` | `ResilientBackend` decorator (retry full turn) |

Runtime wraps every `CompletionBackend` from `NewCompletionBackend` in `ResilientBackend`. OpenAI SDK retries are disabled (`WithMaxRetries(0)`); Solomon handles retries at turn level.

## API resilience

- **Retry:** Configurable attempts (default 3) for retryable HTTP codes (429, 5xx, 408) and transient network errors. Full stream turn is retried; the REPL prints an explicit line via `StreamOpts.OnRetry` before each wait.
- **Backoff:** Exponential delay from `base_delay_ms` to `max_delay_ms`, optional jitter, honors `Retry-After` when present.
- **Circuit breaker:** After all retries fail for a request, the provider host (`base_url` host) is opened for `circuit_open_sec` (default 60s). The next turn still probes the API (half-open); a successful turn clears the open state immediately. Failed probes while open keep the circuit tripped until expiry or success.
- **Timeouts:** `connect_timeout_sec` on dial and response headers; `read_timeout_sec` optional for non-stream calls (`CompleteText`, `ListModels`). Streams keep body read unlimited.
- **Not retried:** `ErrStreamAccumulatorRejected`, 401/403/404/422, and other permanent errors.

Tests: [`test/api_resilience_test.go`](../../test/api_resilience_test.go), [`test/resilient_backend_test.go`](../../test/resilient_backend_test.go), [`test/api_resilience_anthropic_test.go`](../../test/api_resilience_anthropic_test.go), [`test/api_resilience_openai_test.go`](../../test/api_resilience_openai_test.go).

## Key functions

| Function | Behavior |
|----------|----------|
| `CompletionBackend.StreamTurn` | Stream assistant turn (content, tools, usage) |
| `MessageParams` | Map session messages to OpenAI chat params |
| `buildAnthropicMessages` | Map session messages to Anthropic message blocks |
| `MessagesForAPI` | Strip `ReasoningText` from all but the last assistant message |
| `AggregateConsecutiveTurnUsage` | Footer stats across tool sub-turns |
| `ErrStreamAccumulatorRejected` | OpenAI stream chunk rejected — turn aborted |

## Reasoning and thinking

- **Display:** `ReasoningText` in session; `show_thinking` / `reasoning_effort` (OpenAI path).
- **API history:** only the **last** assistant message may include reasoning toward the model (`MessagesForAPI`).
- **Anthropic extended thinking:** disabled in v1 (no `thinking` blocks in requests).
- **Compaction:** `/summarize` transcript and retained tail omit reasoning text.

## Usage stats

`UsageStats` is provider-agnostic. Anthropic maps `input_tokens`, `output_tokens`, `cache_read_input_tokens`, and `cache_creation_input_tokens` (creation stored but not shown in the REPL footer v1).

## Images

User messages may contain `[img-N]` placeholders. OpenAI uses `image_url` data URIs; Anthropic uses `image` blocks with base64 `source` (PNG/JPEG/GIF).

## Stream integrity (OpenAI)

`ChatCompletionAccumulator.AddChunk` must accept every chunk in a single completion stream. On rejection, Solomon aborts the turn without persisting partial results.

Tests: [`test/stream_integrity_test.go`](../../test/stream_integrity_test.go).

## Flow

```mermaid
sequenceDiagram
  participant RT as Runtime
  participant RB as ResilientBackend
  participant BE as OpenAI_or_Anthropic
  participant API as Provider_API

  RT->>RB: StreamTurn TurnRequest
  RB->>BE: StreamTurn
  BE->>API: stream request
  loop chunks
    API-->>BE: delta
    BE-->>RT: print reasoning/content
  end
  BE-->>RB: AssistantTurnResult
  RB-->>RT: AssistantTurnResult
```

## See also

- [Agent turn pipeline](agent-turn-pipeline.md)
- [Sessions and storage](sessions-and-storage.md)
- [Configuration](../user-guide/configuration.md)
