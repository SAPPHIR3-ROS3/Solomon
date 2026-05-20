# Startup and CLI

## Purpose

Documents how the `solomon` binary boots, branches on subcommands, and constructs the interactive runtime.

## Packages and files

| Package / file | Responsibility |
|----------------|----------------|
| `cmd/solomon/main.go` | Entry: logging, CLI branches, readline, `Runtime` lifecycle |
| `internal/paths` | `SolomonHome()` → `~/.solomon` |
| `internal/config` | Load/save TOML, wizard, provider resolve, model pick |
| `internal/project` | `Resolve(wd)` → canonical root + 64-char hex id |
| `internal/logging` | File logs under `~/.solomon/logs` |
| `internal/chatstore` | Empty or loaded `Session` passed into runtime |
| `internal/agent/runtime` | `NewRuntime`, `InitMCP`, `Run`, `RunPromptOnce` |

## Key functions

| Function | File | Behavior |
|----------|------|----------|
| `main` | `cmd/solomon/main.go` | Init logging; handle `add`/`remove`; else wizard + REPL path |
| `paths.SolomonHome` | `internal/paths/paths.go` | User data root |
| `config.RunWizardIfNeeded` | `internal/config/config.go` | First-run interactive setup |
| `config.ResolveProvider` | `internal/config/config.go` | Active provider from `current.*` |
| `project.Resolve` | `internal/project/project.go` | Map cwd → `(root, hex)` |
| `agentruntime.NewRuntime` | `runtime/core.go` | OpenAI client, default `Mode: "build"` |
| `Runtime.InitMCP` | `runtime/mcp.go` | Start MCP manager from config |
| `Runtime.Run` | `runtime/repl.go` | Interactive loop |
| `Runtime.RunPromptOnce` | `runtime/core.go` | Single user message + turns |

## Startup flow

```mermaid
flowchart LR
  start[Run solomon]
  cfg{config exists}
  wizard[wizard]
  load[Load config]
  proj[Resolve cwd]
  mode{CLI mode}
  repl[REPL]
  execOnce[exec]
  start --> cfg
  cfg -->|no| wizard --> load
  cfg -->|yes| load
  load --> proj --> mode
  mode -->|default| repl
  mode -->|exec args| execOnce
```

## CLI branches (early exit)

Before the wizard, `main` handles:

- `solomon add ...` → `commands.Add` with `project.Resolve` deps
- `solomon remove skill <name>` → `commands.Remove`

After runtime construction:

- `solomon temp exec <prompt>` — sets `EphemeralSession`, no long-term chat file
- REPL `/temp` — same persistence rules when the current chat has no messages ([`commands.TempChat`](../../internal/agent/commands/resume.go))
- `solomon exec <prompt>` — one shot with normal session persistence rules
- default — `Runtime.Run`

## Session construction at boot

`main` allocates an empty `chatstore.Session` (placeholder checkpoint fields, empty messages). The REPL or `/resume` loads or assigns ids; see [Sessions and storage](sessions-and-storage.md).

## Extension points

- New global CLI subcommands: add branch in `main` before wizard (mirror `add`/`remove` pattern with `commands.Deps`).
- Boot-time defaults: `NewRuntime` and `config.Root` fields.

## Related code

- [`cmd/solomon/main.go`](../../cmd/solomon/main.go)
- [`internal/config/config.go`](../../internal/config/config.go)
- [`internal/project/project.go`](../../internal/project/project.go)

## See also

- [Runtime and REPL](runtime-and-repl.md)
- [Configuration](../user-guide/configuration.md)
- [Building and releases](../development/building-and-releases.md)
