# Skills and slash

## Purpose

Slash commands for session control, mode, config, and skills; skill registry at global/project/workspace scope; CLI `add`/`remove` and in-REPL skill tools.

## Packages and files

| Area | Path |
|------|------|
| Slash dispatch | `internal/agent/slash.go` |
| Command registry | `internal/agent/commands/builtin_slash.go`, `help.go` |
| Command impls | `internal/agent/commands/*.go` |
| Skills registry | `internal/skills/registry.go`, `add.go`, `remove.go`, `locate.go` |
| Skill tools | `internal/agent/tools/load_skill.go`, `search_skill.go` |
| CLI add/remove | `commands/add.go`, `commands/remove.go` |

## Slash dispatch

| Function | Behavior |
|----------|----------|
| `SlashDispatch` | Parse line, build name, lookup handler in registry |
| `splitSlashArgs` | Shell-like quoting for slash args |
| `commands.Deps` | Context, IO, `ProjHex`, `ProjRoot`, runtime callbacks |

Slash handlers live in `commands` package; the runtime bridge constructs `Deps` from `Runtime` state (`slash_bridge.go`).

## Registry

`/help` prints the authoritative sorted list from [`commands.Registry`](../../internal/agent/commands/help.go).

Common commands: `/plan`, `/build`, `/resume`, `/new`, `/summarize`, `/connect`, `/models`, `/skills`, MCP-related slashes in `mcp_slash.go`.

## Skills registry

| Function | Behavior |
|----------|----------|
| `LoadRegistry` / `SaveRegistry` | `~/.solomon/skills.json` authoritative map |
| `WithRegistryLock` | File lock around registry updates |
| `commands.Add` / `commands.Remove` | CLI and slash-driven install paths |

Scopes: global, project (`projects/<id>/skills/`), local workspace (`.solomon/skills/`). See [Data layout](../user-guide/data-layout.md).

## Flow

```mermaid
flowchart LR
  line["/command args"]
  dispatch[SlashDispatch]
  reg[commands.Registry]
  handler[command func]
  line --> dispatch --> reg --> handler
```

## Extension points

- Add slash: implement handler, register in `builtin_slash.go` and `help.go`.
- Add skill source: extend `skills/add.go` install paths.

## Related code

- [`internal/agent/slash.go`](../../internal/agent/slash.go)
- [`internal/skills/registry.go`](../../internal/skills/registry.go)

## See also

- [Usage and commands](../user-guide/usage-and-commands.md)
- [Native tools](native-tools.md)
- [Runtime and REPL](runtime-and-repl.md)
