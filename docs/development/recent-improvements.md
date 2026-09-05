# Recent workspace and runtime improvements

This page summarizes the user-visible behavior introduced across the desktop GUI, server runtime, model management, integrations, and developer tooling.

## Chat and images

- The chat view is split into focused renderers for messages, tool activity, tool results, footers, subagent panels, and scrolling behavior.
- Composer image attachments are preserved when a message is sent and restored when a chat is reopened.
- Visible `[img-N]` references are rendered consistently in Markdown while code blocks, links, and malformed tags remain unchanged.
- The image lightbox supports drawing, shapes, text, erasing, moving, resizing, and cropping before an attachment is sent.
- Compact Go snippets returned by tools can be formatted by the server for display without changing invalid source.

## Active agents

The side panel includes an Active Agents view backed by the server's active-agent tree. Root chats and nested subagents are grouped by project, expose their current status, and can be opened from the tree. The view polls while open so completed and interrupted work is reflected without reloading the application.

## Integrated terminal

Terminal sessions are managed by the extracted integrated-shell component and persist per project. Selected terminal output can be captured as a clip and referenced from the composer. Clip references use the same safe Markdown decoration path as image references.

## Models and providers

- Model catalog loading is shared consistently across server and desktop clients.
- Providers can be connected from Settings and the catalog can be refreshed without replacing unrelated configuration changes.
- Current model and reasoning-effort updates use targeted configuration writes.
- Model visibility is stored independently in `model-visibility.json`; legacy `hidden_models` values are migrated on the first visibility update.
- Cursor flagship selection handles additional Grok version and reasoning-tier forms.

See [Configuration](../user-guide/configuration.md) for persistence details.

## ChatGPT Sub compatibility

Solomon resolves the stable `@openai/codex` client version from the npm registry, caches it under `SOLOMON_HOME`, and falls back safely when the registry is unavailable. Upstream Codex errors are decoded into actionable messages, including unsupported-model responses.

## Runtime reliability

- Stream completion preserves backend metadata and normalizes oversized inline reasoning whitespace.
- Automatic compaction can complete ephemerally without corrupting the turn loop.
- Recent chats use the last user-message time, keeping project ordering and statistics current.
- `@` expansion rejects binary files, and nested `.gitignore` matching handles path boundaries correctly.
- Server status reports the active GUI source in development mode and clears stale runtime state when the recorded process is unhealthy.

See [Server architecture](../architecture/server.md) and [Usage and commands](../user-guide/usage-and-commands.md) for operational details.

## Validation

Run the repository-wide suite with:

    make test
    make check-docs

The suite builds and tests the Cursor integration, tests the UI prototypes, runs all Go tests, and validates documentation links and package indexes.
