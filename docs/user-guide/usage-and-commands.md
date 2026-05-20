# Usage and commands

## Features

- Interactive readline REPL plus one-shot runs: [`exec`](../../cmd/solomon/main.go), [`temp exec`](../../cmd/solomon/main.go)
- Configuration and state under `~/.solomon`: [`config.toml`](../../internal/paths/paths.go), `mcp.json`, `projects/`, `logs/`, `skills.json`, project-scoped dirs
- First-run wizard if config is missing ([`RunWizardIfNeeded`](../../internal/config/config.go))
- **Working directory ↔ project**: stable id from cwd; chats and skills partitioned per tree ([`project.Resolve`](../../internal/project/project.go))
- **Skills**: `solomon add` / `solomon remove`; `/skills`, `/add`, … in-session (authoritative list: `/help`)
- **MCP clients**: optional `mcp.json`; discovered tools exposed to the model as remote tools

## CLI usage modes

| Mode | Command |
| ---- | ------- |
| Interactive REPL | `solomon` |
| One shot (persisted chat) | `solomon exec <prompt>` |
| Ephemeral session | `solomon temp exec <prompt>` |
| Skill install | `solomon add npx ...` |
| Skill remove | `solomon remove skill <name>` |

Exact usage strings: [`cmd/solomon/main.go`](../../cmd/solomon/main.go).

`exec` and `temp exec` use **shell tokenization**: quotes group words for the shell; they are not smart quotes passed into Solomon.

## Slash commands

In the REPL, type `/help` for the authoritative sorted catalogue ([`commands.Registry`](../../internal/agent/commands/help.go)).

Highlights:

| Command | Role |
| ------- | ---- |
| `/plan` | Planning-only tooling |
| `/build` | Build tools (shell, files, subagent) |
| `/resume`, `/new` | Session switching |
| `/summarize`, `/compact` | Long-context hygiene |
| `/connect` | Add provider and models |

Implementation: [Skills and slash](../architecture/skills-and-slash.md).

## See also

- [Configuration](configuration.md)
- [Data layout](data-layout.md)
- [Runtime and REPL](../architecture/runtime-and-repl.md)
- [Plan vs build](../architecture/plan-vs-build.md)
