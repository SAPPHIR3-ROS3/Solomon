# Sessions and storage

## Purpose

Persist chat transcripts as JSON per project, resolve storage paths from project id, and support listing, resume, images, subchat paths, and background-subagent lifecycle state.

## Packages and files

| Package / file | Responsibility |
|----------------|----------------|
| `internal/chatstore/store.go` | `Session`, `Message`, read/write, list |
| `internal/chatstore/images.go` | Image path repair and storage |
| `internal/chatstore/subchat.go` | Subagent chat paths and image helpers |
| `internal/chatstore/subsession.go` | Subagent records, statuses, active registry, and atomic persistence |
| `internal/chatstore/checkpoint_sync.go` | Git OID fields on session |
| `internal/paths/paths.go` | `SolomonHome`, chats, subagents, and image dirs |
| `internal/project/project.go` | `Resolve`, `projectsId.json` |

## Key types

| Type | Fields (high level) |
|------|---------------------|
| `Session` | `ID`, `Title`, `Messages`, checkpoint fields, `ImageFiles`, `ImageSeq`, usage metadata |
| `Message` | `Role`, `Content`, tool call ids, checkpoint stamp fields |
| `SubSession` | Stable subchat ID, title, messages, parent/tool-call linkage, status, role, reasoning, pending nested spawns |
| `ActiveSubagentsFile` | Process-level registry of background subagents currently running or queued |

## Key functions

| Function | Behavior |
|----------|----------|
| `WriteSession` / `ReadSession` | Atomic JSON under `projects/<hex>/chats/<id>.json` |
| `NewPlaceholderChatID` | Temporary id until title finalize |
| `ChatIDHex` | Stable id from title slug + timestamp |
| `ListRecent` / `SessionWithLatestUserMessage` | `/resume` and `/export last` helpers |
| `ChatsDir`, `SubchatsDir`, `PlansDir`, `TempDir` | Path helpers per project |
| `SubagentsDir`, `ScheduledSubagentPath`, `ActiveSubagentsPath` | Global scheduled-subagent records and active-run registry |
| `project.Resolve` | Canonical root + 64-char hex |

## Persistence rules (runtime)

- `Runtime.persistSession` writes only when `sessionFileCreated` and non-empty `Session.ID` and not `EphemeralSession`.
- Ephemeral mode: `solomon temp exec`, or `/temp` on an empty REPL chat (`commands.TempChat` sets `Runtime.EphemeralSession`). Transcript stays in memory; no `WriteSession` until the user starts a normal chat (`/new`, `/resume`, or first persisted message after leaving ephemeral mode). `/export current` can still archive an ephemeral transcript to markdown under `~/.solomon/exported/` (or `[export].path`).
- Legacy tool settings (`[tools].legacy`, `legacy_force`) are global in `config.toml`, not per-session. Deprecated `legacy_tools` fields in old session JSON are ignored.
- User/assistant/tool append paths call persist after mutation (see [Agent turn pipeline](agent-turn-pipeline.md)).

## Subagent persistence and lifecycle

Every non-ephemeral nested run has a stable ID derived from its parent chat, tool call, and spawn time. Its `SubSession` is written atomically as JSON before the nested stream starts and updated as messages, tool results, usage metadata, and status change.

Project-origin subagents are stored under:

```text
~/.solomon/projects/<project-id>/chats/subchats/<subchat-id>.json
```

Scheduled-origin subagents use the global directory:

```text
~/.solomon/subagents/<subchat-id>.json
```

While a background run is live, `~/.solomon/subagents/activeSubagents.json` records its ID, origin, status, session path, project, and spawn time. The in-memory registry owns cancellation and completion handles; the file is coordination and recovery metadata, not a second transcript.

Subsession statuses are:

| Status | Meaning |
|--------|---------|
| `running` | The nested stream is active or has been resumed in the background |
| `queued` | Work is waiting to be consumed by the runtime |
| `paused` | The run stopped because of timeout, cancellation of the context, or a recoverable error; it can be resumed |
| `done` | The nested stream completed without further tool calls |
| `cancelled` | The user explicitly cancelled the run; its transcript remains available |

`/subagent stop` cancels the live context and persists `paused`; `/subagent cancel` does the same with `cancelled`; `/subagent resume` starts a background continuation. The native tool can also resume with a new task. `interrupt: true` requires `resume` and cancels the active resumed context before appending the new task. Nested subagent requests made while the parent session is unavailable are stored as pending spawn metadata and consumed when a suitable parent session becomes available.

Ephemeral parent sessions do not create persistent subagent records. On process startup, stale `running` entries in the active-run registry are reconciled as paused coordination state; the transcript JSON remains the source of message and status data.

## On-disk layout

See [Data layout](../user-guide/data-layout.md) for the full `~/.solomon` tree diagram.

## Extension points

- New session fields: extend `Session` struct and migration in `FinishSessionLoad` if needed.
- Alternate storage: would replace `WriteSession`/`ReadSession` (not pluggable today).

## Related code

- [`internal/chatstore/store.go`](../../internal/chatstore/store.go)
- [`internal/chatstore/subsession.go`](../../internal/chatstore/subsession.go)
- [`internal/agent/runtime/subagent_registry.go`](../../internal/agent/runtime/subagent_registry.go)
- [`internal/project/project.go`](../../internal/project/project.go)

## See also

- [Checkpoints](checkpoints.md)
- [Runtime and REPL](runtime-and-repl.md)
- [LLM layer](llm-layer.md)
