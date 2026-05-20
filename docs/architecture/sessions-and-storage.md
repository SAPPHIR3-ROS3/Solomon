# Sessions and storage

## Purpose

Persist chat transcripts as JSON per project, resolve storage paths from project id, and support listing, resume, images, and subchat paths.

## Packages and files

| Package / file | Responsibility |
|----------------|----------------|
| `internal/chatstore/chatstore.go` | `Session`, `Message`, read/write, list |
| `internal/chatstore/session_images.go` | Image path repair and storage |
| `internal/chatstore/subchat.go` | Subagent chat paths |
| `internal/chatstore/checkpoint_sync.go` | Git OID fields on session |
| `internal/paths/paths.go` | `SolomonHome`, chats dir, images dir |
| `internal/project/project.go` | `Resolve`, `projectsId.json` |

## Key types

| Type | Fields (high level) |
|------|---------------------|
| `Session` | `ID`, `Title`, `Messages`, checkpoint fields, `ImageFiles`, `ImageSeq`, `LegacyTools`, usage metadata |
| `Message` | `Role`, `Content`, tool call ids, checkpoint stamp fields |

## Key functions

| Function | Behavior |
|----------|----------|
| `WriteSession` / `ReadSession` | Atomic JSON under `projects/<hex>/chats/<id>.json` |
| `NewPlaceholderChatID` | Temporary id until title finalize |
| `ChatIDHex` | Stable id from title slug + timestamp |
| `ListRecent` / `SessionWithLatestUserMessage` | `/resume` helpers |
| `ChatsDir`, `SubchatsDir`, `PlansDir` | Path helpers per project |
| `project.Resolve` | Canonical root + 64-char hex |

## Persistence rules (runtime)

- `Runtime.persistSession` writes only when `sessionFileCreated` and non-empty `Session.ID` and not `EphemeralSession`.
- Ephemeral mode: `solomon temp exec`, or `/temp` on an empty REPL chat (`commands.TempChat` sets `Runtime.EphemeralSession`). Transcript stays in memory; no `WriteSession` until the user starts a normal chat (`/new`, `/resume`, or first persisted message after leaving ephemeral mode).
- User/assistant/tool append paths call persist after mutation (see [Agent turn pipeline](agent-turn-pipeline.md)).

## On-disk layout

See [Data layout](../user-guide/data-layout.md) for the full `~/.solomon` tree diagram.

## Extension points

- New session fields: extend `Session` struct and migration in `FinishSessionLoad` if needed.
- Alternate storage: would replace `WriteSession`/`ReadSession` (not pluggable today).

## Related code

- [`internal/chatstore/chatstore.go`](../../internal/chatstore/chatstore.go)
- [`internal/project/project.go`](../../internal/project/project.go)

## See also

- [Checkpoints](checkpoints.md)
- [Runtime and REPL](runtime-and-repl.md)
- [LLM layer](llm-layer.md)
