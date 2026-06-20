# Skills and slash

## Purpose

Slash commands for session control, mode, config, and skills; skill registry at global/project/workspace scope; CLI `add`/`remove` and in-REPL skill tools.

## Packages and files

| Area | Path |
|------|------|
| Slash dispatch | `internal/agent/slash/dispatch.go`, `internal/agent/slash_forward.go` |
| Command registry | `internal/agent/commands/builtin_slash.go`, `help.go` |
| Command impls | `internal/agent/commands/*.go` |
| Skills registry | `internal/skills/registry.go`, `add.go`, `remove.go`, `locate.go` |
| Skill tools | `internal/agent/tools/load_skill.go`, `search_skill.go` |
| CLI add/remove | `commands/add.go`, `commands/remove.go` |

## Slash dispatch

| Function | Behavior |
|----------|----------|
| `SlashDispatch` / `slash.Dispatch` | Parse line, forced `/skill:`, builtin registry, dynamic skill slashes |
| `splitSlashArgs` | Quote-aware slash argument split (`slash/dispatch.go`) |
| `commands.Deps` | Context, IO, `ProjHex`, `ProjRoot`, runtime callbacks |

Slash handlers live in `commands` package; the runtime bridge constructs `Deps` from `Runtime` state ([`slash_deps.go`](../../internal/agent/runtime/slash_deps.go)).

## Registry

`/help` prints the authoritative sorted list from [`commands.Registry`](../../internal/agent/commands/help.go).

Common commands: `/agent`, `/chat`, `/resume`, `/new`, `/temp`, `/summarize`, `/connect`, `/models`, `/legacytools`, `/btw`, `/cursortools` (Cursor API configured only), `/skills`, forced `/skill:<name> [request]`, and MCP-related slashes in `mcp_slash.go`.

`/legacytools` persists `[tools].legacy` and `[tools].legacy_force` to `config.toml` (global). `/cursortools` persists `[tools].cursor_internal_tools` and restarts the Cursor sidecar; visibility is gated by `config.CursorAPIConfigured`. Both are implemented in [`thinking.go`](../../internal/agent/commands/thinking.go). User guide: [Usage and commands — `/legacytools`](../user-guide/usage-and-commands.md#legacytools), [`/cursortools`](../user-guide/usage-and-commands.md#cursortools).

`/btw` is a builtin reserved slash name, but unlike normal slash commands it is meant for the active generation window. The idle handler only prints usage; the runtime listener intercepts `/` during streaming and queues a transient no-tools side question. User guide: [Usage and commands — `/btw` side questions](../user-guide/usage-and-commands.md#btw-side-questions).

## Skills registry

| Function | Behavior |
|----------|----------|
| `LoadRegistry` / `SaveRegistry` | `~/.solomon/skills.json` authoritative map |
| `WithRegistryLock` | File lock around registry updates |
| `commands.Add` / `commands.Remove` | CLI and slash-driven install paths |

Scopes (last `/add` argument; default `global`):

| Scope | Path |
|-------|------|
| `global` | `~/.solomon/skills/` |
| `project` | `~/.solomon/projects/<id>/skills/` |
| `local` | `<workspace>/.solomon/skills/` |

See [Data layout](../user-guide/data-layout.md#skills).

**skills.sh URLs** — `/add https://skills.sh/owner/repo/skill [scope]` (and `https://www.skills.sh/...`) are normalized and turned into `npx --yes skills add <repo> --skill <pkg> -y`. The npm `skills` CLI stages files under `~/.agents/skills/`; Solomon then copies into the scope above. Solomon does not auto-append npm `-g`/`--global`; scope is controlled by the Solomon argument, not the npm CLI global flag.

## Install command validation

Skill installation commands coming from `/add npx ...`, `npm exec ...`, or a generated `skills.sh` install line are validated against an allowlist before execution.

Behavior:

- Only the `skills` package with subcommand `add` is accepted.
- Only the repository target plus `--skill`, optional `-g`/`--global`, and `-y`/`--yes` are accepted after `skills add`.
- Shell syntax is rejected instead of passed to `sh -c` / `cmd /c`.
- Execution uses direct argv dispatch (`exec.CommandContext`) after validation, not a general-purpose shell string.

## Forced skill slash

`/skill:<name> [request]` is a special slash path handled before builtin slash dispatch and before dynamic skill slash lookup.

Behavior:

- Resolve `<name>` against installed skills using the same registry ordering as `loadSkill` resolution.
- Support names with spaces without quoting; when names overlap, prefer the longest matching installed skill name.
- Build a structured user payload that embeds the chosen skill body plus the optional trailing request.
- Preserve the visible transcript line as `/skill:<name> ...` while sending the expanded payload to the model via `chatstore.Message.APIContent`.

This path is distinct from dynamic skill slash bindings (such as `/dup` or `/skill-dup`), which still prefill or submit the skill body directly.

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

- [`internal/agent/slash/dispatch.go`](../../internal/agent/slash/dispatch.go)
- [`internal/skills/registry.go`](../../internal/skills/registry.go)

## See also

- [Usage and commands](../user-guide/usage-and-commands.md)
- [Native tools](native-tools.md)
- [Runtime and REPL](runtime-and-repl.md)
