# Claude sidecar: blocked capability investigation

## Status

Solomon starts this directory as a local OpenAI-compatible sidecar for the Claude Sub provider. Chat completion uses the official Agent SDK and the installed Claude CLI login. Like the Cursor sidecar, requests run concurrently and share no state: each conversation gets its own live SDK session (at most 8, closed after 30 idle minutes; a request that finds every session running or waiting on tool results waits up to two minutes for a slot, then fails with 503, and such sessions are never evicted to make room), and every request continues the session whose exact history it extends, so background title or subagent requests never disturb a chat waiting on tool results. History that no live session holds (a restarted sidecar, an edited or branched chat, a model switch) starts a new session with the transcript replayed as text in the first prompt, never through undocumented transcript records.

## Verified dependency

Installed the approved `@anthropic-ai/claude-agent-sdk` at exactly `0.3.310`, with a local package-lock. npm installed its declared peer and optional transitive packages; no additional direct dependencies were added. The package reports Claude Code `2.1.310`. Its official Linux x64 bundled executable reports `2.1.310 (Claude Code)`. The SDK module imports successfully. No authenticated inference was performed.

## Public-interface evidence

See `node_modules/@anthropic-ai/claude-agent-sdk/sdk.d.ts`:

- `query`: accepts `string | AsyncIterable<SDKUserMessage>`, not an arbitrary assistant/user conversation.
- `SDKUserMessage`: a user envelope; its documentation specifies an Anthropic user message. `message` uses the broader Messages API `MessageParam` type, but that alone does not document assistant-history injection into a user envelope. `shouldQuery: false` accumulates user input and merges it into the next querying user message; it is not an assistant-history import API.
- `Options.resume`: loads an existing Claude session. It does not import arbitrary OpenAI history, edits, truncations, or branches.
- `SessionStore.load`: requires entries deep-equal to previously appended records.
- `SessionStoreEntry`: explicitly describes the concrete transcript union as CLI-internal, not part of the SDK API; adapters must treat records as pass-through blobs. Synthesizing these records would rely on unsupported internal structures.
- `Options.tools: []`: disables built-in tools. `PreToolUse` exposes tool name, input and tool-use ID and supports permission denial. `createSdkMcpServer` supports SDK-hosted custom tools. These capabilities can intercept new calls, but do not resolve arbitrary history restoration.
- `Query.close()` terminates the CLI and cleans up pending requests and MCP transports.

An MCP handler could remain pending while Solomon approves and executes its tool, then return the real result to a live SDK session. That requires a separately designed stateful continuation protocol and exact session-prefix validation. It is not a general replacement for stateless OpenAI conversation replay, especially after a parent restart or history edit. This alternative was not implemented or validated.

## CLI and cleanup

No existing `claude` executable was found on PATH during investigation. No existing installation or Claude settings were changed. The approved SDK installation supplied its official platform CLI under this directory's `node_modules`. It is an installed artifact, not automatically removed on SDK query completion. No global CLI installer was run and no CLI identity was spoofed.

A future Go parent must install the locked package in a parent-owned temporary runtime directory, use any existing CLI via `pathToClaudeCodeExecutable` without modifying it, otherwise use the SDK's bundled CLI, terminate all queries before removing the owned temporary directory, and clean stale owned directories after crashes. It must never remove an existing CLI installation or user authentication directory. Cleanup management is not implemented here.

## Protocol and launch environment

There is no launch command, launch environment contract, listening port, or implemented `/health`, `/v1/models`, or `/v1/chat/completions` protocol. Streaming, nonstreaming, token authentication, request limits, cancellation and tool-call delivery are therefore not claimed as implemented. Provider credentials must remain separate from any future localhost bearer token. No credentials were printed by this investigation.

## Verification

Run `npm ci` followed by `npm test` in this directory using Node 22.18 or newer with TypeScript stripping. Tests use only Node built-ins and the approved SDK. They check the installed SDK version, exported entrypoints and the documented history/safety contracts; they are not inference, HTTP or end-to-end Solomon-loop tests. `npm ci` installs the same approved dependency graph and may download the official platform CLI.
