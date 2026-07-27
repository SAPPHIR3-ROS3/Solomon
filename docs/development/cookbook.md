# Cookbook

Step-by-step recipes for common changes. Each lists files, order, tests, and docs. Testing detail: [Testing](testing.md). Package context: [architecture portal](../architecture/README.md).

## Add a slash command

1. Implement handler in [`internal/agent/commands/`](../../internal/agent/commands/) (new file or existing).
2. Register in [`builtin_slash.go`](../../internal/agent/commands/builtin_slash.go) (`Registry` entry with names + help text).
3. Ensure [`help.go`](../../internal/agent/commands/help.go) lists it (sorted catalogue for `/help`).
4. Wire callbacks on [`commands.Deps`](../../internal/agent/commands/deps.go) if the command needs runtime state — set in [`slash_deps.go`](../../internal/agent/runtime/slash_deps.go).
5. Optional: first-arg completion in [`replcomplete/slash_args.go`](../../internal/agent/runtime/replcomplete/slash_args.go).
6. Test: [`test/slash_dispatch_test.go`](../../test/slash_dispatch_test.go) using [`testDeps`](../../test/helpers_test.go).
7. Docs: [Usage and commands](../user-guide/usage-and-commands.md) if user-facing.

**Do not** parse slash lines outside [`slash/dispatch.go`](../../internal/agent/slash/dispatch.go).

## Add a native tool (plan or build)

1. Implement tool in [`internal/agent/tools/`](../../internal/agent/tools/) (handler func).
2. Register OpenAI schema in [`params.go`](../../internal/agent/tools/params.go) for the right `Mode`(s).
3. Route execution in [`exec.go`](../../internal/agent/tools/exec.go) (mode guards, `switch` on tool name).
4. If the tool needs env/context, extend [`toolenv/env.go`](../../internal/agent/toolenv/env.go), wire fields in [`runtime/exec.go`](../../internal/agent/runtime/exec.go) `toolEnv()`, read from handler via `tools.Env` alias ([`tools/env.go`](../../internal/agent/tools/env.go)).
5. Optional: plan/build dump strings in `dump_*.go` if the tool should appear in system prompt listings.
6. Test: new or extended file in [`test/`](../../test/) with `t.TempDir()` for filesystem tools.
7. Docs: [Native tools](../architecture/native-tools.md), [Plan vs build](../architecture/plan-vs-build.md) if mode-specific.

MCP tools are separate — configure `mcp.json`, not this path ([MCP integration](../architecture/mcp-integration.md)).

## Change the agent turn loop or legacy XML

1. Stream loop: [`internal/agent/runtime/turns.go`](../../internal/agent/runtime/turns.go).
2. Native vs legacy resolution: [`legacy.go`](../../internal/agent/runtime/legacy.go).
3. Terminal tool lines / legacy stream writer: [`tool_print.go`](../../internal/agent/runtime/tool_print.go).
4. XML parse/stream: [`internal/tooling/`](../../internal/tooling/).
5. Test: [`legacy_runtime_test.go`](../../test/legacy_runtime_test.go), [`legacy_tools_test.go`](../../test/legacy_tools_test.go).
6. Docs: [Agent turn pipeline](../architecture/agent-turn-pipeline.md).

Preserve: fail-closed stream integrity ([`llm/stream/completion.go`](../../internal/llm/stream/completion.go)), SIGINT cancel via `WithCancelCause`.

## Modify REPL input or tab completion

1. Keys/buffer: [`repl/editor/read.go`](../../internal/agent/runtime/repl/editor/read.go).
2. Redraw: [`repl/editor/refresh.go`](../../internal/agent/runtime/repl/editor/refresh.go).
3. Loop dispatch: [`repl/loop.go`](../../internal/agent/runtime/repl/loop.go).
4. Completion: [`replcomplete/`](../../internal/agent/runtime/replcomplete/) + wiring [`replcomplete_runtime.go`](../../internal/agent/runtime/replcomplete_runtime.go).
5. Add test exports in [`editor/editorhistory.go`](../../internal/agent/runtime/repl/editor/editorhistory.go) if needed.
6. Test: [`repl_editor_test.go`](../../test/repl_editor_test.go), [`repl_complete_*_test.go`](../../test/).
7. Docs: [Runtime — REPL input](../architecture/runtime-repl.md).

## Add a web search engine

1. Implement [`search.Engine`](../../internal/search/engine.go) in new file under [`internal/search/`](../../internal/search/).
2. Register in [`web_search.go`](../../internal/agent/tools/web_search.go) and config keys.
3. Test: focused test with HTTP stub if the engine calls the network.
4. Docs: [Configuration — web search](../user-guide/configuration.md), [Supporting packages](../architecture/supporting-packages.md).

## Add or change an LLM provider path

1. Provider config: [`internal/config/`](../../internal/config/), wizard [`commands/connect/`](../../internal/agent/commands/connect/).
2. Backend factory: [`internal/llm/factory.go`](../../internal/llm/factory.go).
3. Protocol adapter: `openai_backend.go` or `anthropic_*.go`.
4. OAuth (ChatGPT Sub): [`internal/auth/openai/codex/`](../../internal/auth/openai/codex/), [`providersetup/`](../../internal/providersetup/).
5. Test: `httptest` — see [`api_resilience_*_test.go`](../../test/), [`provider_auth_test.go`](../../test/provider_auth_test.go).
6. Docs: [Configuration](../user-guide/configuration.md), [LLM layer](../architecture/llm-layer.md).

## Cursor sidecar / bridge change

1. Go process/install: [`internal/integrations/cursor/`](../../internal/integrations/cursor/).
2. Node proxy: [`integrations/cursor/src/`](../../integrations/cursor/src/) — especially [`legacy.ts`](../../integrations/cursor/src/legacy.ts), [`chat.ts`](../../integrations/cursor/src/chat.ts).
3. Runtime hooks: [`cursor_sidecar.go`](../../internal/agent/runtime/cursor_sidecar.go), [`cursor_native_display.go`](../../internal/agent/runtime/cursor_native_display.go).
4. Rebuild embed: `go run scripts/cursor_bundler.go build && bundle` (or `make build`).
5. Test: Go [`stream_cursor_tool_test.go`](../../test/stream_cursor_tool_test.go); TS `make cursor-proxy-test`.
6. Docs: [Cursor integration](../architecture/cursor-integration.md) — update tool bridge table if aliases change.

## Docs change checklist

1. Update the architecture or user article for the behavior change.
2. Add or link tests in the article if non-obvious.
3. Run `make check-docs` (markdown links, anchors, cited paths, package index).

Design rationale stays in **architecture** articles; there is no separate ADR folder.

## See also

- [Testing](testing.md)
- [Building and releases](building-and-releases.md)
- [Runtime hub](../architecture/runtime.md)
